# DW-396 spike: what signal stack does a .NET thread actually get?

Run 2026-09-22, to settle the question that gated this spec's design: **does a
thread the binding creates get a larger alternate signal stack than a CLR
thread-pool thread?**

**No. Every thread gets the same one, and it is smaller than Go needs.**

The probe that produced this is preserved at [evidence/Program.cs](./evidence/Program.cs)
and [evidence/probe.csproj](./evidence/probe.csproj). It reads
`sigaltstack(NULL, &old)` on each thread kind rather than waiting for an
overflow — a deterministic reading in seconds, instead of a soak.

## Measurements

Every row is one `sigaltstack()` reading. All containers, arm64 executing
**natively** on Apple Silicon.

| Host | `sysconf(_SC_SIGSTKSZ)` | main | `.NET TP Worker` | `new Thread()` | raw `pthread`, attached |
| --- | --- | --- | --- | --- | --- |
| arm64 · Debian 12 · glibc 2.36 · .NET 8 | 20480 | **24576** | **24576** | **24576** | **24576** |
| arm64 · Debian 12 · glibc 2.36 · .NET 9 | 20480 | **24576** | **24576** | **24576** | **24576** |
| arm64 · Ubuntu 20.04 · glibc 2.31 · .NET 6 | **−1** (pre-2.34) | **24576** | **24576** | — | — |
| amd64 · Debian 12 · .NET 8 — ⚠ **emulated** | 8192 | **16384** | **16384** | **16384** | — |
| amd64 · Debian 12 · .NET 8 — ✅ **real silicon** (WSL2, `GenuineIntel`, 2026-09-23) | 8192 | **16384** | **16384** | **16384** | **16384** |

`new Thread(…, 16 * 1024 * 1024)` reads 24576 as well: **the managed stack size
is irrelevant** — it sizes the ordinary stack, not the signal stack.

Four things follow, and each rules something out:

1. **It is a constant, not a computed value.** On glibc 2.31 `sysconf(_SC_SIGSTKSZ)` does not exist and returns −1, and the CLR still installs exactly 24576. It does not track `SIGSTKSZ`, and it did not change across .NET 6, 8 and 9.
2. **Thread origin does not matter.** A raw `pthread` created through `pthread_create`, outside the CLR entirely, reads 24576 once the runtime has attached it. There is no thread you can reach from managed code that escapes this.
3. **The architectures differ, and amd64 is the tighter one:** 16 KiB against arm64's 24 KiB.
4. **Both are below what Go expects — and *above* what glibc advises.** On the real-amd64 leg `_SC_MINSIGSTKSZ` is 1776 and `_SC_SIGSTKSZ` is 8192, so the CLR's 16384 is **twice glibc's recommendation** and nine times its floor. The CLR is not undersizing its altstack by any standard glibc sets; Go is sizing its handlers at 4× that recommendation and then adopting someone else's stack without checking it. The defect is on the Go side of the boundary, and any runtime following glibc's advice would hit it.

## Why that is the defect

Go allocates 32 KiB for its own signal stacks — `runtime/os_linux.go`, at the
version this repository pins (`toolchain go1.26.0`):

```go
func mpreinit(mp *m) {
	mp.gsignal = malg(32 * 1024) // Linux wants >= 2K
	mp.gsignal.m = mp
}
```

But under cgo it does not install that stack on a thread that already has one.
`runtime/signal_unix.go`:

```go
func minitSignalStack() {
	mp := getg().m
	var st stackt
	sigaltstack(nil, &st)
	if st.ss_flags&_SS_DISABLE != 0 || !iscgo {
		signalstack(&mp.gsignal.stack)   // install Go's own 32 KiB
		mp.newSigstack = true
	} else {
		setGsignalStack(&st, &mp.goSigStack)   // ADOPT what is already there
		mp.newSigstack = false
	}
}
```

The CLR always installs one, so the `else` branch is always taken, and **every
Go `SA_ONSTACK` handler entered from .NET runs in the CLR's 16 or 24 KiB rather
than Go's 32.** When the frame does not fit, the kernel converts the overflow
into SIGSEGV and the CLR reports `Internal CLR error (0x80131506)`.

### This explains the asymmetry DW-396 could not

DW-396 recorded "both natives, linux/arm64 (containers, native execution): 0
crashes, many runs" beside a crashing amd64, and had no account of why. The
account is **16 KiB versus 24 KiB** against the same handler: arm64 has half
again as much headroom for the same frame. It was never clear — it had a wider
margin.

⚠ **Under this spec's own rule, arm64 is therefore not cleared**, and the amd64
row above is emulated and indicative only. The CLR's altstack size is a
compile-time constant inside the amd64 CoreCLR binary — which is the real
binary, even under translation — so the 16384 should hold; but `sysconf`'s 8192
is the emulator's answer, not real silicon's. On a real amd64 with AVX-512 the
kernel's signal frame is much larger, which is exactly what drove glibc 2.34 to
make `SIGSTKSZ` a runtime value in the first place. **Confirm 16384 on the WSL2
box** — it is a ten-second run of the same probe.

### And it refines the glibc reading

DW-396 concluded the build's glibc moves *how often* the defect fires, not
whether. That stands, and the mechanism is now sharper: **the receiving buffer
is a CLR constant and does not move at all.** What varies is how much goes into
it — the kernel's signal frame, whose size follows the CPU's register state,
plus whatever the handler itself uses. So machine-to-machine variation is
expected and a clean run on one box says nothing about another.

## The fix, verified

Enlarging the alternate signal stack explicitly works, and the runtime stays
healthy afterwards:

```
  before                     size=24576 (24 KiB)
  sigaltstack(set 1024 KiB) -> 0
  after                      size=1048576 (1024 KiB)
  NullReferenceException still caught normally after the swap.
  Full blocking GC completed on the swapped thread.
```

The CLR installed that altstack for its own SIGSEGV handling (null checks, GC
write barriers, stack-overflow detection). This **replaces the memory with more
of it** rather than removing it, so the CLR's handler is not deprived — it is
given more room too. The two post-swap checks above exercise exactly those
paths. Not exhaustive, and worth re-running under the full suite.

Two consequences for the design:

- **The altstack memory must outlive the thread**, and must not be freed while the thread can still take a signal. That means **long-lived threads**, allocated once at creation — which is an argument for the pool independent of the throughput one.
- **It must be set before the thread's first crossing**, because `minitSignalStack` adopts whatever it finds at `needm` time.

## What this does to the design

**The pool survives, and is now justified twice over.** It was chosen for
throughput (cgo runs a `c-shared` export on the calling thread, so one thread
would serialise every render onto one core). It is now also the only clean way
to hold the fix: a bounded, long-lived set of threads whose signal stacks the
binding controls and can size once at creation.

What changes is that the pool is **not sufficient on its own**. A pool of
ordinary `new Thread()` workers would have reproduced the defect exactly.
The load-bearing step is the explicit `sigaltstack()` enlargement on each pool
thread; the pool is what makes that step possible and permanent.

## Open after this spike

- ~~Confirm `16384` on real amd64 hardware, and capture `sysconf(_SC_SIGSTKSZ)` there~~ — **✅ done 2026-09-23** on the WSL2 box, probe-stamped NATIVE. 16384 confirmed on all six thread kinds. `_SC_SIGSTKSZ` is **8192**, so it does *not* exceed 16384 and the conditional above resolves the opposite way to the one it anticipated: the CLR sits above glibc's recommendation, not below its minimum. See DW-396's 2026-09-23 subsection.
- Decide the enlarged size. 1 MiB was used here arbitrarily and worked; the cost is per pool thread and paid once.
- Confirm Go adopts the enlarged stack in practice, not only by source reading, by entering the real native from an enlarged pool thread on amd64 and soaking it.
- `DllImport("libc")` for `sigaltstack` must be reachable from the `netstandard2.0`/`net46` floor. It is plain P/Invoke so it should be, but `pthread_create` was **not** resolvable from `libc` on glibc 2.31 (it lived in `libpthread` until the 2.34 merge) — a reminder that libc entry points are not uniformly available across the floor this package supports.
