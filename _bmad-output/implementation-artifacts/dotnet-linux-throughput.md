# CAP-10: render throughput across the engine-thread boundary

| | |
|---|---|
| **Spec** | `_bmad-output/specs/spec-dotnet-linux/SPEC.md` — **CAP-10** |
| **Story** | `_bmad-output/specs/spec-dotnet-linux/stories/4-throughput-non-regression.md` |
| **Tool** | `folio-dotnet/build/measure-throughput.sh` + `folio-dotnet/build/render-throughput/` |
| **Before** | `86e7e5a` — the parent of `7f6a936`, the commit that moved every crossing onto binding-owned engine threads |
| **After** | `HEAD 3005db9`, with `folio-dotnet/src/Folio8` clean — the wrapper checks and stamps this per run |
| **Evidence** | `evidence/dotnet-linux-throughput/` — every leg log and build log behind every table below |
| **Taken** | 2026-09-22/23 |

> **This is a measurement record, not a gate.** No CI job runs the harness, it bakes in no
> threshold, and nothing here is asserted by a test. CAP-10 is a **non-regression** claim:
> the question is whether throughput survived the change, not whether it beat it.
>
> **Nothing here says anything about DW-396 or about crash-freedom.** A benchmark that
> completes is not a soak. CAP-5 owns that claim, on real hardware, with the pre-fix build
> reproduced first.
>
> **Every table below is read out of a committed log file**, named per section. No number
> here was retyped from a terminal.

---

## What was compared, and how

The harness renders `fixtures/multi-page-statement` — a two-page design over 130 synthetic
transactions, flowing to four output pages with a page header and footer on each — from a
fixed number of dedicated caller threads, for a fixed window, and counts completed renders.

**Correctness is checked concurrently, not just once.** One single-threaded render is
hashed against the fixture's committed `expected.json` before anything is timed; then, in
*every level's warmup*, **every caller** hashes its first render and compares. So a binding
that rendered correctly alone and corrupted under concurrent crossings — the failure mode
this change could plausibly introduce — cannot be counted here as throughput. `4b0367f5…`
matched on every render on every leg.

**The baseline is real code.** `measure-throughput.sh` creates a `git worktree` at `86e7e5a`
and builds the *same* harness sources against **that tree's** `Folio8.csproj`. There is no
bypass switch inside `src/Folio8` — the shipped binding was not touched by this story at
all. Both legs stage the **same** native library file, so the engine is held fixed while the
binding varies.

**The legs alternate, run by run, three times each.** Running all of before and then all of
after would let host drift over the intervening minutes land entirely on one column. The
`spread` column is half the observed range as a percentage of the mean; it is how a reader
separates a difference from drift, and it is the reason single-run figures from the first
pass of this work are not reproduced here.

**Concurrency levels are 1, 2, 4 and `ProcessorCount`.** The single-caller figure isolates
the queue-hop cost; the `ProcessorCount` figure answers the question the owner actually asked
when choosing a pool over a single thread.

### Reading the scaling figures

`3.63×` for the after leg means *after at concurrency 10 over after at concurrency 1*. Each
leg divides by **its own** concurrency-1 throughput. **The two legs' multiples are therefore
not comparable with each other**: a leg that is slower at concurrency 1 scores a *higher*
multiple for the same absolute throughput. Scaling answers "does the pool buy parallelism";
the `change` column answers "did this cost anything". Conflating them is how the first pass
of this record reported "improved scaling" on macOS when the numerator was flat and the
denominator had fallen.

---

## Provenance — read this before quoting any number below

**All three legs ran on one developer machine: an Apple Silicon Mac, 10 logical processors,
macOS 26.6.2.** That is **indicative, not authoritative**: it is not a controlled
environment, it has other work on it, and the spread columns below show what that is worth.

| Leg | Host as measured | Runtime | Native staged | Translated? |
|---|---|---|---|---|
| **A — linux/arm64** | Debian 12 container, arm64, 10 CPU | .NET 8.0.31 | `build/native/linux-arm64/libfolio8_native.so` | **No** — arm64 executes natively on this host |
| **B — darwin/arm64** | macOS 26.6.2, arm64, 10 CPU | .NET 10.0.11 | `build/native/host/libfolio8_native.dylib` | **No** — `sysctl.proc_translated = 0` |
| **C — linux/amd64** | Debian 12 container, x64, 10 CPU | .NET 8.0.31 | `build/native/linux-x64/libfolio8_native.so` | **YES — Rosetta.** `/proc/cpuinfo` `vendor_id` reads `VirtualApple` |

**There is no Windows leg here.** CAP-10 names Windows x64 explicitly. See *What this does
not establish*.

---

## Leg A — linux/arm64, native silicon, 3 runs per leg, 5 s per level

The platform this epic is about, running natively.
Source: `evidence/dotnet-linux-throughput/linux-arm64/`.

| Concurrency | before r/s | spread | after r/s | spread | change |
|---|---|---|---|---|---|
| 1 | 40.80 | ±3.1 % | 43.14 | ±1.8 % | **+5.7 %** |
| 2 | 74.17 | ±1.9 % | 76.15 | ±2.4 % | **+2.7 %** |
| 4 | 116.26 | ±1.2 % | 121.04 | ±0.6 % | **+4.1 %** |
| 10 (`ProcessorCount`) | 158.59 | ±1.1 % | 156.66 | ±3.8 % | **−1.2 %** |

| Concurrency | before median ms | after median ms | before p95 ms | after p95 ms |
|---|---|---|---|---|
| 1 | 22.71 | 21.88 | 32.52 | 33.23 |
| 2 | 25.10 | 24.39 | 36.30 | 37.07 |
| 4 | 33.37 | 31.46 | 46.01 | 44.31 |
| 10 | 61.54 | 61.01 | 95.44 | 97.47 |

- **Scaling, after: 3.63×** from concurrency 1 to 10. The pool buys parallelism; it does not
  flatten at one core. (Before: 3.89× — see *Reading the scaling figures*; the two multiples
  are not a comparison, the change column is.)
- **No measurable regression at any level.** Every change is within, or barely outside, the
  legs' own spread.
- **No latency tail.** Median and p95 track each other across both legs.

### Above the pool size

From the earlier single-run pass, kept because no 3-repeat run of it has been taken:
`THROUGHPUT_LEVELS=1,2,4,10,40` on the same leg gave **164.15 r/s after against 170.14 r/s
before at 40 callers** (−3.5 %, n=1), against 160.60 after at concurrency 10. Throughput
**does not collapse** when callers outnumber engine threads four to one; what grows is
per-render latency (62 ms → 240 ms), which is queueing. **n=1, so the −3.5 % is inside this
host's drift and is not a finding.**

## Leg B — darwin/arm64, the host itself, 3 runs per leg

Source: `evidence/dotnet-linux-throughput/darwin-arm64/`.

| Concurrency | before r/s | spread | after r/s | spread | change |
|---|---|---|---|---|---|
| 1 | 48.50 | ±0.9 % | 44.50 | ±1.6 % | **−8.2 %** |
| 2 | 83.95 | ±2.2 % | 76.84 | ±3.8 % | **−8.5 %** |
| 4 | 121.14 | ±0.6 % | 124.78 | ±1.4 % | +3.0 % |
| 10 | 183.87 | ±2.3 % | 181.68 | ±2.7 % | −1.2 % |

| Concurrency | before median ms | after median ms | before p95 ms | after p95 ms |
|---|---|---|---|---|
| 1 | 20.35 | 19.99 | **22.33** | **41.78** |
| 2 | 22.21 | 21.84 | 34.81 | 46.91 |
| 4 | 28.31 | 26.88 | 53.38 | 52.91 |
| 10 | 49.46 | 50.31 | 93.54 | 90.46 |

**The low-concurrency deficit is real and it is a tail, not a slower render.** −8.2 % at
concurrency 1 sits well outside both legs' spread (±0.9 % and ±1.6 %), so it is not drift.
But the **median render is unchanged** — 19.99 ms after against 20.35 ms before — while
**p95 has nearly doubled**, 22.33 ms to 41.78 ms. The same shape appears at concurrency 2 and
is gone by concurrency 4. That is a handoff whose wakeup sometimes waits, not a render that
got slower.

Leg A shows no such tail at the same levels on the same hardware, so this is specific to
macOS, which the package does not ship for. **It is filed rather than dismissed** — see
DW entry *"the engine-thread handoff has a wakeup tail at low concurrency"* in
`deferred-work.md`. The reason to track it is not macOS: it is that the mechanism is the
`Monitor.Wait`/`Pulse` handoff in `EngineThreads`, which is the same code on every platform,
and a scheduler that behaves this way on a Linux host would cost a low-concurrency consumer
the same 8 %.

## Leg C — linux/amd64 under Rosetta: NO COMPARISON COULD BE TAKEN

Source: `evidence/dotnet-linux-throughput/linux-amd64-rosetta/`.

**Three attempts at a 3-repeat comparison; none completed.** In each attempt one leg or the
other printed `Stack overflow.` and was killed:

| Attempt | Which leg died | Signal |
|---|---|---|
| 1 | **before** (pre-fix binding) | 34 (exit 162) |
| 2 | **after** (current binding) | 6, SIGABRT (exit 134) |
| 3 | **before** (pre-fix binding) | 11, SIGSEGV (exit 139) |

**Both bindings die, on the same host, with the same native.** That is why nothing is
attributed to the engine-thread change: the pre-fix binding, which has no pool and no
enlarged signal stack, aborts identically. A translated host is also the least interpretable
place for a stack-related failure, which is precisely why SPEC.md excludes translated hosts
from CAP-5's soak evidence. The related open question — the engine threads' inherited
managed stack size — is already filed in `deferred-work.md` and is to be measured on real
hardware during CAP-5.

**No amd64 throughput figure is reported.** The first pass of this record quoted a −11.5 %
at concurrency 10 from a single unrepeated run on this leg; on the evidence above that
number is withdrawn, not restated.

---

## What this establishes

1. **On Linux the pool buys parallelism.** 3.63× on ten cores, natively, against 1.00× for a
   single marshalling thread. The owner's reason for choosing a pool over one thread holds up.
2. **On Linux there is no measurable throughput regression at any concurrency level.** Every
   change is at or inside the legs' own run-to-run spread over three alternating runs.
3. **Throughput does not collapse under oversubscription** — 40 callers against 10 engine
   threads, n=1, held within 3.5 % of the before-binding.
4. **Concurrent renders are byte-correct**, verified per caller per level rather than once.
5. **Any regression is now a number with a spread beside it**, re-runnable by one command,
   with the logs behind it committed.

## What this does NOT establish

- **Windows.** CAP-10 names "no material regression against 1.1.0's direct-`DllImport` path"
  on **Windows x64**, and no Windows leg was taken. Filed in `deferred-work.md`.
- **Real amd64 silicon.** Leg C is translated and could not complete a comparison at all.
- **Anything about crash-freedom or DW-396.** A benchmark is not a soak; CAP-5 owns that.
- **That these numbers reach a reader.** CAP-10's success criterion says the numbers are
  "recorded in the release notes"; this file is the source, and no task in story 4 owns the
  copy into `RELEASING.md` or the 1.2.0 notes. Filed in `deferred-work.md`.

---

## Re-running this

```sh
# Anything whose numbers get written down: repeat, and keep the logs.
THROUGHPUT_REPEATS=3 THROUGHPUT_LOG_DIR=./throughput-logs \
  folio-dotnet/build/measure-throughput.sh arm64

folio-dotnet/build/measure-throughput.sh            # this host
folio-dotnet/build/measure-throughput.sh amd64      # linux/amd64 in a container
THROUGHPUT_LEVELS=1,2,4,10,40 folio-dotnet/build/measure-throughput.sh arm64
```

It needs a native library staged (`folio-dotnet/build/build-native.sh`) and a full clone —
the baseline half is a `git worktree` at `7f6a936^`, and on a shallow clone the wrapper
refuses to run rather than present an after-only figure as a comparison. It also refuses,
without attributing anything to DW-396, when a leg is *refused by the harness* (exit 1 or 2)
rather than killed by a signal.
