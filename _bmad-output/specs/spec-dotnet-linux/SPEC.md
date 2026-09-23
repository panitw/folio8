---
id: SPEC-dotnet-linux
companions:
  - ./sigaltstack-findings.md
  - ./withdrawal-surface.md
  - ../../implementation-artifacts/deferred-work.md
  - ../spec-client-libraries/SPEC.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# folio-dotnet on Linux — closing DW-396 and restoring the two withdrawn RIDs

## Why

This is **a pain to solve, and the work is already most of the way done.** `linux-x64` and `linux-arm64` were built, verified and packed during folio-dotnet 1.1.0's preparation, then **withdrawn from the package before release**: entering the engine from a CLR thread-pool thread overflows that thread's alternate signal stack and kills the host process (DW-396). The build tooling, the pinned-image glibc discipline, `verify-linux-natives.sh`, the ELF arm of the pack check and the `folio-dotnet-linux` CI job all remain on `main` — the RIDs were withdrawn, not the work.

What that costs is the majority of the .NET reporting workloads folio8 was built to reach. The `net46` floor exists for long-lived Windows applications, but everything written since runs modern .NET in a Linux container, and for those callers folio-dotnet does not exist: `dotnet add package folio8` on Linux resolves no native at all and the first render throws. The documented alternative — use folio-js — tells a C# team to adopt Node.

DW-396 is no longer "nothing explains it". The mechanism is identified, the blast radius is known, and the remaining work is a design change at the ABI boundary rather than an investigation.

## Capabilities

- **CAP-1**
  - **intent:** A .NET application running on Linux x64 or arm64 can render a template and data to PDF bytes in-process, from an ordinary `PackageReference`, with no caller-authored load logic and no configuration.
  - **success:** On a clean `mcr.microsoft.com/dotnet` container on each architecture, with no Go and no C compiler present, `dotnet add package folio8` plus the documented first-PDF snippet produces a correct PDF.

- **CAP-2**
  - **intent:** The folio8 engine is only ever entered from a binding-owned thread whose alternate signal stack is large enough for the Go runtime's handlers — on **every** crossing, `folio8_free` included.
  - **success:** A test enters every public entry point from a `ThreadPool` worker and from a `Task` continuation while the process is under deliberate stack-depth and GC pressure, and the kernel's `overflowed sigaltstack` message is absent from `dmesg` for every run. Stated as a property of the design, not inferred from a clean run: an instrumented assertion reads `sigaltstack(NULL, &old)` on the thread that actually crosses and asserts both that it is a binding-owned thread and that its signal stack is the enlarged size — the same reading the spike used.

- **CAP-10**
  - **intent:** Routing calls through the engine-thread boundary does not meaningfully cost a caller throughput, on Linux or on Windows.
  - **success:** A concurrent render benchmark on Windows x64 shows no material regression against 1.1.0's direct-`DllImport` path, and Linux scales with cores rather than flattening at one — measured, with the numbers recorded in the release notes.

- **CAP-3**
  - **intent:** Byte-identity with `folio-go` holds on both Linux RIDs exactly as it does on the Windows ones.
  - **success:** A CI leg per architecture renders the whole `fixtures/` corpus through the binding and matches every committed expected SHA-256, and the diagnostic sequence per fixture equals Go's by code, severity and message.

- **CAP-4**
  - **intent:** Each shipped Linux native is provably the architecture its RID claims and carries the glibc floor the pinned build image produces, so no consumer's minimum glibc is raised in silence.
  - **success:** `verify-linux-natives.sh` passes on both RIDs in CI — ELF machine matched per RID, no referenced glibc symbol above **2.28** — and `FolioPackageCheck` refuses the pack if either is missing, is not `ET_DYN`, or has the other architecture's `e_machine`.

- **CAP-5**
  - **intent:** The fix is shown to hold under repetition long enough to have caught the defect that was withdrawn, on hardware rather than emulation.
  - **success:** A soak of the render-heavy suite completes clean over a run count materially beyond the 11 that previously produced a false clear — **separately on real amd64 and on real arm64**, neither standing in for the other — with the crashing pre-fix build reproduced in the same harness first to prove the harness can see the defect. No emulated or translated host counts, Rosetta included.

- **CAP-6**
  - **intent:** An Alpine/musl consumer meets a clear install-time absence rather than a crash on first render.
  - **success:** On a musl image, no native resolves from the package and the documented `FolioNativeLoadException` (or the host's own load failure) names the unsupported platform; no `linux-musl-*` RID is packed.

- **CAP-7**
  - **intent:** The guards that enforced the withdrawal now enforce the restoration, so a half-restored package cannot ship.
  - **success:** Every site in [withdrawal-surface.md](./withdrawal-surface.md) under "Guards that prohibit" asserts presence instead of absence, and each reddens when its RID, its CI step, or its README claim is removed again.

- **CAP-8**
  - **intent:** A developer choosing folio8 can read the supported platforms and get the truth, and the release record says what changed and why it was ever otherwise.
  - **success:** `docs/folio-dotnet.md` and its `.html` agree and state the four RIDs, the glibc floor and the musl exclusion; `folio-dotnet/README.md`, the csproj `<Description>` and `RELEASING.md` carry no Windows-only claim; DW-396 is closed with the mechanism, the fix and the soak evidence recorded.

- **CAP-9**
  - **intent:** The engine's own documented thread-safety contract matches what the binding now does, in the same change that changes it.
  - **success:** `folio-go/cshared/README.md`'s "safe to call from several threads" claim is re-examined and either reaffirmed with the sigaltstack caveat named, or narrowed; `docs/folio-dotnet.md` states the binding's concurrency behaviour — whether calls serialise, and what a caller may assume about parallel renders.

## Constraints

- **The mechanism, now measured end to end, is what the design must defeat.** The CLR installs a **fixed-size** alternate signal stack on every thread it touches — **16 KiB on amd64, 24 KiB on arm64** — and Go, under cgo, **adopts an existing altstack rather than installing its own 32 KiB** (`minitSignalStack`). So every Go `SA_ONSTACK` handler entered from .NET runs in a stack smaller than Go sizes for itself; when the frame does not fit, the kernel converts the overflow into SIGSEGV and the CLR reports `Internal CLR error (0x80131506)`. Windows is immune because it has no POSIX signals. Measurements, source references and the probe are in [sigaltstack-findings.md](./sigaltstack-findings.md).
- **Creating the thread is not the fix; sizing its signal stack is** (spike, 2026-09-22). `new Thread()` at any managed stack size, and even a raw `pthread` created outside the CLR, all receive the same undersized altstack once the runtime attaches them. The binding must **explicitly enlarge** it with `sigaltstack()` on each engine thread, **before that thread's first crossing** — `minitSignalStack` adopts whatever it finds at `needm` time — and the memory must outlive the thread, never freed while it can still take a signal.
- **A run of passes is not evidence, and this is a ruling, not a caution.** The AlmaLinux-8-built native went 0-crashes-in-11 on CI and was read as unaffected; staged into a WSL2 soak, the same shipped native overflowed and took two fatal signals. glibc 2.34 turned `SIGSTKSZ` into a `sysconf()`-backed runtime value, so the build's glibc moves **how often** the defect fires, not whether it exists. No acceptance criterion in this spec may be satisfied by absence of failures alone.
- **One call path on every platform** (owner ruling). Windows crosses the same marshalling boundary as Linux. Two paths behind a conditional would leave the POSIX one unexercised by every Windows CI leg, which is how the Linux path rots between releases. The cost is accepted: Windows, today 0 crashes in 20 measured runs, takes a change it does not need.
- **A pool of engine threads, not one** (owner ruling, and the spike supports it from a second direction). cgo runs a `c-shared` export **on the calling thread** rather than handing it to Go's scheduler, so a single marshalling thread would serialise every render in the process onto one core. The pool is sized to available parallelism rather than a fixed small constant, and its threads are reused — a fresh thread pays cgo's attach/detach cost on every crossing. It is also the only clean holder of the fix above: a bounded set of **long-lived** threads whose signal stacks the binding sizes once at creation. The pool alone is not sufficient — a pool of ordinary threads would reproduce the defect exactly.
- **Every crossing goes through the boundary, `folio8_free` included.** Freeing a result token is itself an ABI call, so a caller who renders on one thread and disposes on another crosses it too. Routing all of them means callers see **no new restriction**; the alternative — documenting "dispose on the thread you rendered on" — is rejected as a contract.
- **The signal stack is a generous fixed size, not a computed one.** `SIGSTKSZ` is a compile-time constant before glibc 2.34 and a `sysconf()`-backed runtime value after, because modern register save areas (AVX-512) are large — so no single queried value is right on both. A few hundred KB per pool thread costs nothing against that ambiguity.
- **Emulation is not soak evidence, and that includes Rosetta** (owner ruling). Running `linux-x64` under Rosetta on Apple Silicon is sanctioned for the development loop and for corpus byte-identity — rendering is deterministic CPU work and the hashes match. It is **not** admissible for CAP-5: the defect is a handler frame overflowing an alternate stack whose size follows the CPU's register save area, and under Rosetta that is the **arm64 host's**, with translated signal delivery on top. That is the exact subsystem under test, and it is the qemu trap DW-396 fell into once already. An arm64 Mac does run Linux arm64 **natively** in a VM or container, which supplies one of the two real-hardware legs.
- **The `net46` floor is unchanged and unnegotiable.** `netstandard2.0`, `LangVersion 7.3`, and **no `PackageReference`, ever**. The marshalling thread is built from what .NET Framework 4.6 has — no `Span<T>`, no `System.Text.Json`, no `NativeLibrary`, no `DllImportResolver`, no nullable reference types. `test/Folio8.Net46Compile` proves the floor a second way and must keep passing.
- **No rendering logic moves into C#, and no corpus hash moves.** This change alters *where the ABI is crossed from*, never *what crosses it*. A moved hash means the change did something it was not permitted to do.
- **The pinned build image is the provenance mechanism, not a convenience.** A cgo library records the glibc symbol versions of the machine that built it. The AlmaLinux 8 image, pinned **by digest**, is what holds the consumer's floor at 2.28; `build-native.sh host` remains a development aid and is never packaged, on Linux or anywhere else.
- **`linux-musl-x64` is not shipped, deliberately.** Go's `-buildmode=c-shared` emits initial-exec TLS relocations that musl's loader refuses under `dlopen` — which is exactly how P/Invoke loads this library — and building *with* musl does not fix it, measured on x86-64 and arm64 alike. Shipping the RID would convert a clean install-time absence into a runtime crash.
- **The thread-safety claim is re-examined in the same change, not after it.** `folio-go/cshared/README.md` states calls are safe from several threads; if the binding now serialises or pins them, the claim and the .NET documentation must say what is actually true before the package ships.
- **The release line is 1.2.0, a minor, not a 1.1.x patch.** New RIDs and a reworked call boundary with its own concurrency consequences are not patch-shaped, and both `RELEASING.md` and DW-396 already name 1.2.0 as where this lands.

## Non-goals

- **macOS natives.** `osx-x64` and `osx-arm64` are not shipped. macOS has the same POSIX signal exposure and the fix would cover it, but the build leg, the CI leg and the min-version verification are a platform's worth of work this spec does not take. `build-native.sh host` stays a development aid.
- **Alpine / musl support.** Ruled out on a measured property of Go's build mode, above. Not a deferral awaiting effort.
- **Linux on .NET Framework.** `net46` is Windows-only by definition; `folio-dotnet.targets`' staging path is untouched.
- **A new folio-dotnet API surface.** No new public types, methods or options. A caller's code is identical on Linux and on Windows.
- **Changing what crosses the ABI.** The eight `folio8_*` entry points, the result-frame encoding, the allocation-token discipline and `folio8_abi_version` are unchanged. The engine is not rebuilt, re-tuned or upgraded as part of this.
- **Making `folio-dotnet-linux` a general correctness gate for the engine.** It gates this package on Linux; `matrix.yml` remains where cross-target byte identity is established.
- **Tuning the boundary beyond non-regression.** CAP-10 requires throughput not to fall off; making it *faster* than 1.1.0, or exposing pool size as a knob, is out.

## Success signal

A team running a .NET service in a Linux container adds `folio8` as a `PackageReference`, deploys on amd64 and on arm64, and renders statements under production concurrency for a sustained period with no process crash and no caller-authored load or threading code — and the PDF bytes are indistinguishable from what the Go engine, the `folio8` CLI and the designer's preview produce for the same template and data. DW-396 is closed with a named mechanism, a design that makes it unreachable, and a soak that reproduced the old defect before showing the new build clean.

## Assumptions

- The marshalling boundary is a change local to `folio-dotnet`'s managed code; `folio-go/cshared` needs documentation work (CAP-9) but no code change. Supported by the spike — the fix is on the .NET side of the boundary — but not yet proved against the real native.
- ~~A newer Go runtime has not already changed this behaviour.~~ **Checked and closed by the spike.** Go 1.26.0, the version this repository pins, still adopts the caller's altstack under cgo. There is no toolchain bump that makes this go away.
- Both Linux RIDs keep the same package-size and install-story shape as the Windows pair; no separate runtime package or opt-in RID selection is introduced.
- The soak harness of CAP-5 can be run on owner-accessible real hardware: the arm64 leg on a Linux VM or container on the owner's Apple Silicon Mac, which executes natively; the amd64 leg on the WSL2 box that first reproduced the defect, or a cloud amd64 runner. Translated hosts — qemu, Rosetta — are excluded by constraint, not by preference.

## Open Questions

- ~~**Confirm `16384` on real amd64, and capture both `sysconf(_SC_SIGSTKSZ)` and `_SC_MINSIGSTKSZ` there.**~~ **RESOLVED 2026-09-23** on the WSL2 box, probe-stamped NATIVE (`GenuineIntel`). 16384 confirmed on all six thread kinds. The answer is **neither** of the two the question anticipated: `_SC_MINSIGSTKSZ` is 1776 and `_SC_SIGSTKSZ` is 8192, so the CLR installs **twice glibc's recommendation**. It is not undersizing anything — Go sizes its handlers at 4× that recommendation and then adopts an adopted stack without checking it, so the discrepancy is on **Go's** side of the boundary. Not a .NET defect. The consequence for this spec is that CAP-6's warning to non-.NET callers covers runtimes that are doing nothing wrong, and must not present the CLR as the anomaly.
- **How large is the enlarged stack?** 1 MiB was used in the spike arbitrarily and worked. The cost is per pool thread, paid once.
- **Does `DllImport("libc")` reach `sigaltstack` across the whole supported floor?** Plain P/Invoke, so it should — but the spike found `pthread_create` **not** resolvable from `libc` on glibc 2.31 (it lived in `libpthread` until the 2.34 merge), so libc entry points are not uniformly available across the range this package supports.
- **Where does the boundary live — `Native.cs`, or a new type?** Placement only; the constraints settle the behaviour.
- **Does the engine pool interact badly with a caller's own concurrency limits?** A consumer already sizing its work against `ProcessorCount` now has a second pool of the same size behind the binding. Whether that needs a cap, or is simply documented, is open.
