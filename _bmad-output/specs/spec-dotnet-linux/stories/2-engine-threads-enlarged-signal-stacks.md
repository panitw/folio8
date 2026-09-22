---
title: 'Route every ABI crossing through engine threads with enlarged signal stacks'
type: 'feature'
created: '2026-09-22'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '86e7e5a8a5094849bdc3831c3c5c8f94dd33f3cf'
context:
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/sigaltstack-findings.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** DW-396. The CLR installs a fixed-size alternate signal stack on
every thread it touches — 16 KiB amd64, 24 KiB arm64 — and Go, under cgo,
*adopts* the caller's altstack rather than installing its own 32 KiB
(`minitSignalStack`). So a Go `SA_ONSTACK` handler entered from a .NET
thread-pool thread runs in less room than Go sizes for itself, the kernel turns
the overflow into SIGSEGV, and the CLR reports `Internal CLR error
(0x80131506)`. It is nondeterministic, it has no consumer-side mitigation, and
it is why `linux-x64` and `linux-arm64` were withdrawn from folio8 1.1.0.

**Approach:** Stop crossing the ABI on arbitrary CLR threads. Every crossing
runs on a small pool of long-lived engine threads the binding owns, and each of
those enlarges its own alternate signal stack before its first crossing.

## Boundaries & Constraints

**Always:**
- **Enlarging the signal stack is the fix; owning the thread is what makes it possible.** A pool of ordinary threads reproduces the defect exactly.
- **Set the altstack before the thread's first crossing** — Go's `minitSignalStack` adopts whatever it finds at `needm` time — and **never free that memory while the thread can still take a signal**. Engine threads therefore live for the process's lifetime.
- **Every crossing goes through the pool**, so a caller may still render on one thread and dispose on another with no new restriction.
- **One path on every platform** (owner ruling). Windows uses the same pool. Only the `sigaltstack` step is POSIX-conditional — a no-op where there are no POSIX signals — never the call path itself.
- **Move `Native.Invoke` as a whole unit.** Moving only the inner delegate would leave the copy-and-free tail on the calling thread, straddling `folio8_render` and `folio8_free` across a thread boundary.
- **Re-entrancy must not deadlock.** `Invoke` already calls `EnsureAbi`, which itself crosses; work submitted from a thread that is already an engine thread has to run inline.
- **`ExceptionDispatchInfo` is the rethrow mechanism** across the hop — already used at `Native.cs:229/248/258`, available at the floor, and it preserves the original stack.
- **The `net46` floor is unchanged**: `netstandard2.0`, LangVersion 7.3, `TreatWarningsAsErrors`, and **no `PackageReference`, ever**. `Folio8.Net46Compile` globs the shipped sources and must stay green.

**Never:**
- No new public API. `DocsTests.TheGuideNamesEveryPublicItemInCodeInBothTwins` requires a docs twin for every public name; this story adds none, so it touches no docs.
- `System.Threading.ApartmentState` / `Thread.SetApartmentState` are absent from `netstandard2.0` (they exist in net46) — using them passes `Net46Compile` and fails the main build.
- No corpus hash moves, and no rendering logic enters C#. This changes *where* the ABI is crossed from, never *what* crosses it.
- Does not re-add the Linux RIDs or touch `PackagingTests`' ban — story 6 owns that, after the soak.
- Do not pursue a raw `pthread_create` thread as the fix. **Measured, story 1:** a raw pthread reads the same 24576 once the runtime attaches it. Nothing reachable from managed code escapes the CLR's altstack; enlarging it is what works.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Render from a `ThreadPool` worker | Any supported platform | Work runs on an engine thread, never the caller's | N/A |
| Render from a `Task` continuation | Sync-context-free continuation | Same | N/A |
| Result disposed on a different thread than it was rendered on | Cross-thread `folio8_free` | Succeeds; free also crosses on an engine thread | N/A |
| Re-entrant crossing | `Invoke` → `EnsureAbi` → `folio8_abi_version`, already on an engine thread | Runs inline, no deadlock, no second queue hop | N/A |
| Native call throws | `FolioRenderException`, `InvalidOperationException`, `FolioNativeLoadException` | Rethrown on the caller's thread with the original stack intact | `ExceptionDispatchInfo` |
| Concurrency exceeds pool size | More caller threads than engine threads | Callers queue and complete; no deadlock, no unbounded thread growth | N/A |
| `sigaltstack` enlargement fails | Non-zero return on an engine thread | Thread startup fails loudly, naming errno — never a thread that silently renders on the CLR's small stack | Fail, do not degrade |
| Windows | No POSIX signals | Pool used identically; no libc P/Invoke attempted | N/A |

</frozen-after-approval>

## Code Map

- `folio-dotnet/src/Folio8/Native.cs:210` `Invoke(Call)` -- the funnel for five result-frame exports **and** `folio8_free` (:255). The P/Invoke runs inside a closure built at each call site (`Native.Call`, :201). Move this whole method's body onto an engine thread.
- `folio-dotnet/src/Folio8/Native.cs:125` `EnsureAbi` -- crosses via `folio8_abi_version` (:147) under `AbiGate` (:116). Called *from* `Invoke` (:212), so it is the re-entrancy case.
- `folio-dotnet/src/Folio8/Folio8.cs:177` `OutstandingNativeAllocations` -- calls `folio8_allocation_count` **directly, bypassing the funnel entirely**. Must be routed too.
- Call sites that build the closure: `Folio8.cs:44` (version), `:77` (render), `:160` (validate); `Template.cs:44` (parse), `:75` (parameter references). These need no change if `Invoke` is moved as a unit.
- `folio-dotnet/src/Folio8/NativeLibraryLoader.cs:73` `Ensure` -- Windows-only by design (`IsWindows()`, :85); off Windows the loader does nothing and `dlopen` happens implicitly at the first `DllImport`, which will now be on an engine thread. Correct as-is.
- `folio-dotnet/src/Folio8/Native.cs:116-117` and `NativeLibraryLoader.cs:65-66` -- `_abiChecked` / `_loaded` are plain `bool`, not `volatile`, read unsynchronized on the fast path. Pre-existing; note it, and do not make it worse.
- `folio-dotnet/test/Folio8.Tests/AssemblyInfo.cs:8` -- `DisableTestParallelization = true`, because `FreeDisciplineTests` measures the engine's global allocation table. A new concurrency test must create its own threads rather than relying on xunit parallelism.
- `folio-dotnet/test/Folio8.Tests/SurfaceTests.cs:33-38` -- regex bans over `src/Folio8/**/*.cs`; threading types are **not** banned. `:64-68` bans `\bLayout\w*\(`, `\bxref\b`, `%PDF` over the **whole file text including comments** — so avoid those words in new commentary.
- `folio-dotnet/build/signal-stack-probe/Program.cs` -- story 1's diagnostic. Its `StackT` layout, the `sigaltstack` P/Invoke signature and the LP64 guard are a working reference. **Do not share code with it**; re-declare inside the floor.

## Tasks & Acceptance

**Execution:**
- [x] `folio-dotnet/src/Folio8/EngineThreads.cs` -- new internal type: a lazily-started, fixed set of long-lived threads sized to `Environment.ProcessorCount`, a work queue, submit-and-wait with `ExceptionDispatchInfo` rethrow, and inline execution when the caller is already an engine thread.
- [x] `folio-dotnet/src/Folio8/EngineThreads.cs` -- each thread enlarges its own alternate signal stack on POSIX before serving any work, by `sigaltstack()` through `DllImport("libc")`, with memory that is never freed; a failure fails thread startup loudly. No-op on Windows.
- [x] `folio-dotnet/src/Folio8/Native.cs` -- route `Invoke` (:210) and `EnsureAbi` (:125) through the pool, moving `Invoke`'s body as a unit so the call, the copy and the free stay on one thread.
- [x] `folio-dotnet/src/Folio8/Folio8.cs:177` -- route `OutstandingNativeAllocations` through the pool; it is the one crossing with no funnel.
- [x] `folio-dotnet/test/Folio8.Tests/EngineThreadTests.cs` -- assert the mechanism, not the absence of a crash: from a `ThreadPool` worker and a `Task` continuation, the thread that crosses is a binding-owned engine thread **and** its altstack is the enlarged size (read it with the same `sigaltstack(NULL,&old)` call the probe uses); plus re-entrancy, cross-thread free, and more caller threads than engine threads.

**Acceptance Criteria:**
- Given a render invoked from a `ThreadPool` worker on Linux, when the crossing happens, then the executing thread is an engine thread whose alternate signal stack is the enlarged size, and no `.NET TP Worker` ever enters the engine.
- Given a result rendered on one thread and freed on another, when it is freed, then it succeeds and the engine's allocation count returns to its prior value.
- Given a native error, when it surfaces on the caller's thread, then it is the same exception type with its original stack preserved.
- Given more concurrent callers than engine threads, when all render, then every call completes and no deadlock occurs.
- Given the corpus, when it is rendered after this change on every CI target, then every SHA-256 matches the committed hashes.
- Given `dotnet build Folio8.Net46Compile.csproj`, when it runs, then it succeeds with zero warnings.

## Implementation Notes

- **`EngineThreads` is a new internal type in the global namespace**, beside
  `Native` and `NativeLibraryLoader`. `Run(Action)` is the one primitive;
  `Run<Func<T>>` wraps it. The queue is a `Queue<WorkItem>` under a monitor
  and each caller waits on its own item's gate, so a caller is woken by the
  thread that served it and not by every completion.
- **`stack_t`'s field order is not the same on Linux and Darwin**, and this
  cost a failing first run. Linux/glibc is `ss_sp; ss_flags; <4 bytes pad>;
  ss_size`; Darwin is `ss_sp; ss_size; ss_flags`. The Linux layout on macOS
  passes a size of zero where the kernel wants one and returns `ENOMEM` —
  measured, errno 12, on the host dev loop. The binding therefore asks
  `uname()` for the kernel's name and picks the layout, and then **reads the
  installed size back** with `sigaltstack(NULL, &old)` and refuses to serve
  unless it is the size that was asked for. A successful return is not a
  larger stack, and the read-back is what makes a wrong layout loud instead
  of silent. `build/signal-stack-probe` is Linux-only and has only the one
  layout; sharing its declaration would have shipped the bug.
- **The enlarged size is 1 MiB** (`EngineThreads.SignalStackBytes`), the size
  the DW-396 spike measured working. Fixed, not computed: `SIGSTKSZ` is a
  compile-time constant before glibc 2.34 and a `sysconf()` value after, so
  no single queried number is right across the supported range.
- **`Startup` is a separate rendezvous object, and `SignalStacks` has its own
  gate.** `Start()` holds `StartGate` while it waits for every thread to
  report, so a thread reaching for `StartGate` on its way to reporting would
  deadlock the pool at creation. Found by reading, not by running.
- **`_isEngineThread` is set only after the enlargement succeeds.** Setting it
  first would let a nested submission run inline on a thread still on the
  runtime's 16 KiB, which is the defect itself.
- **Threads are background threads.** They live for the process's lifetime and
  must not keep it alive; the altstack memory is never returned, by design.
- `Native.EnsureAbi`'s unsynchronised `_abiChecked` read is untouched and
  noted in place, per the Code Map.
- `EngineThreadTests` carries more than the five cases the story lists, and
  each extra one closes a gap the five leave open:
  `EveryEngineThreadCarriesTheEnlargedSignalStack` holds every engine thread
  inside a crossing at once through a `Barrier`, so the reading covers the
  whole pool rather than whichever thread happened to pick the work up;
  `TheAllocationCountCrossesOnAnEngineThreadToo` asserts the hop itself —
  without it, deleting the routing of `OutstandingNativeAllocations` leaves
  the suite green, because the returned integer is identical either way; and
  `AThreadThatCannotEnlargeItsSignalStackDisablesTheLibrary` drives the
  matrix's `sigaltstack`-failure row through an internal seam, covering the
  message, its stickiness, and that nothing is served afterwards.
- **`SurfaceTests` now holds the rule as a standing one.**
  `EveryNativeEntryPointIsReachedOnlyThroughTheEngineThreads` scans
  `src/Folio8/**/*.cs` with comments and string literals stripped and refuses
  any `folio8_*` reference that is not inside a `Native.Invoke(` or
  `EngineThreads.Run(` statement, bar the two sites inside the pooled path
  itself. A future direct call site added in good faith reddens there rather
  than crashing a consumer's host.
- The test suite was itself a source of the defect: seven calls in
  `FreeDisciplineTests` and `NativeNamingTests` entered the engine from the
  xunit thread. They now go through `EngineThreads.Run`.


## Spec Change Log

## Review Triage Log

Three layers, all launched before any result was read. Verdicts are mine; reviewer
severities were disregarded as the workflow requires.

| # | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|
| 1 | On 32-bit POSIX `EnlargeSignalStack` returns silently and the thread serves crossings anyway (edge-case) | **high** | patch | Read at the guard: declining to guess the LP64 layout is right, but the thread then set `_isEngineThread` and served. Directly contradicts the frozen matrix row "never a thread that silently renders on the CLR's small stack", and the failure mode is DW-396 unchanged. Now refuses. |
| 2 | `OutstandingNativeAllocations`' routing is unobservable — revert it and the whole suite passes (verification-gap, pre-verified) | **high** | patch | Confirmed by mutation: reverting `Folio8.cs:179` to a direct P/Invoke left every test green, because all call sites assert only the returned integer. This is the one crossing the story added *because* it bypassed the funnel. After the fix the same mutation fails three tests. |
| 3 | Seven test-side calls still cross on the xunit thread (verification-gap) | **high** | patch | `FreeDisciplineTests` 60/64/65/78/85/86 and `NativeNamingTests:77` called `Native.folio8_*` directly. On Linux those enter the engine on a CLR thread with the small altstack — the suite meant to be the evidence DW-396 is closed contained the defect's trigger. All seven routed. |
| 4 | No standing guard against a future unfunneled crossing (blind-hunter, verification-gap) | **high** | patch | Verified absent: `SurfaceTests`' existing regex sets say nothing about P/Invoke placement. One direct call in a later change would reintroduce DW-396 with CI green. New guard verified negatively — it reddens naming the file and the statement. |
| 5 | `IsDarwin()` defaults to the Linux layout when `uname()` fails, and every non-Darwin POSIX kernel gets it though the BSDs use Darwin's field order (blind-hunter, edge-case) | **medium** | patch | Real: the read-back check meant a BSD bricked rather than misbehaved, but an unidentifiable kernel silently picked a layout — the "plausible-looking wrong answer" the file's own 32-bit comment refuses. Now names the kernel and refuses anything but Linux/Darwin. |
| 6 | `Start()`'s `_started` fast path sits inside `lock (StartGate)` (blind-hunter) | **medium** | patch | Confirmed by reading `Start()`. I initially overstated this: the critical section is nanoseconds against a millisecond render, so it is contention, not a throughput cap. Still trivially avoidable, and the pool exists to preserve concurrency. Now a volatile read before the lock. |
| 7 | The `sigaltstack`-failure row of the matrix is untested and there is no seam to induce it (blind-hunter) | **medium** | patch | True, and it is the path that permanently disables the library. Now covered: message shape, stickiness across a different entry point, and that nothing is served afterwards. |
| 8 | `AResultRenderedOnOneThreadIsFreedFromAnother` asserts nothing about render/free placement (blind-hunter, edge-case) | **medium** | patch | Verified: it compared the test thread's id with an engine thread's, which can never be equal. The real property — `Invoke` moves as a unit so render and free are always on one engine thread — is now what is asserted, with the caller-side split documented as unreachable (no public dispose; the token never escapes the frame). |
| 9 | A dequeued item's caller can wait forever if `Serve` throws outside the inner try (edge-case) | **medium** | patch | Real; `Run` has no timeout, so the caller parks indefinitely. Gate pulse moved into a `finally`, `Pulse` to `PulseAll`. |
| 10 | `Environment.ProcessorCount == 1` in a cpu-limited container can deadlock a non-inline nested submission (edge-case) | **medium** | patch | Real, and the inline re-entrancy test would hang to the 45-minute job timeout rather than fail. Pool floored at 2. |
| 11 | Two `Barrier.SignalAndWait` return values discarded (blind-hunter, edge-case) | **low** | patch | A genuine pool deadlock would be swallowed and resurface as a confusing count assertion. Direct correction. |
| 12 | `InvokeHere` stack-trace assertion is brittle under Release inlining (blind-hunter, edge-case, verification-gap) | **low** | patch | Verification runs `-c Release`; the JIT may inline. `NoInlining` applied. |
| 13 | The altstack is heap memory with no guard page; `DllImport("libc")` is unreachable on musl (blind-hunter, edge-case) | **low** | patch | Both real and both were undocumented decisions. Judged correct on the merits and written down where the code makes them: a guard page costs two more libc surfaces across the floor for frames nowhere near 1 MiB, and no `linux-musl` RID is packed so Alpine still meets an install-time absence. Behaviour unchanged, reasoning recorded. |
| 14 | Story record: U+2011 in a test name, and a test count the file contradicts (blind-hunter) | **low** | patch | Confirmed; the name neither matched the code nor grepped. Corrected. |
| 15 | `EnsureAbi`'s hop can never take the queue path — its only caller is already an engine thread (verification-gap, other) | **low** | rejected | True but harmless: the hop is the cheap inline branch, and removing it would make the invariant depend on `InvokeHere` staying the sole caller. Keeping it is the safer expression of "every crossing goes through the pool". |
| 16 | A partial startup failure leaks threads holding 1 MiB each (blind-hunter, edge-case) | **low** | rejected | The threads are `IsBackground`, so the process still exits, and the failure path is now loud and sticky, so the pool is not reused. The fix would add reaping machinery for a case that already ends in a permanently-refusing binding. |
| 17 | The queue is unbounded and `Run` cannot time out (blind-hunter, edge-case) | **low** | rejected | A render has no natural deadline, so a timeout would convert a slow document into a spurious failure — a worse defect than the one it guards. Finding 9 removes the reachable way a caller parks forever. |
| 18 | Process shutdown can kill an engine thread mid-crossing, leaving a foreground caller never woken (edge-case) | **maybe-false** | defer | Could not settle it from the diff: the threads are background, so the runtime tears them down at exit, but whether a foreground caller mid-`Run` is observably hung during shutdown depends on runtime teardown ordering I did not test. What would settle it: a test that renders on a foreground thread while the process exits. |
| 19 | The enlarged-stack assertions never run against the shipped Linux natives (verification-gap, other) | **medium** | defer | Verified: `folio-dotnet-linux` runs no `dotnet test`, so the only Linux execution is `folio-dotnet-host` against a runner-glibc build. Correct as a finding, but story 3 owns the CI legs and story 6 the shipped RIDs. Filed rather than pulled forward. |
| 20 | Nothing in Verification runs on Linux, so the story cannot show it meets its own Linux acceptance criterion (blind-hunter) | **medium** | rejected | Correct about the spec text, refuted in fact: this story was verified on Linux arm64 in a container against a real `linux-arm64` native built in the pinned image — 161/161, five consecutive runs, and the mechanism assertions among them. Verification section updated to record that command. |

**Mutation evidence recorded here because it is the story's real acceptance.** With the
enlargement removed, on Linux arm64: `Expected: 1048576, Actual: 24576` — the suite
measures the DW-396 condition directly rather than waiting for a crash. With
`OutstandingNativeAllocations` un-routed: three tests fail, one naming the cause.


## Design Notes

The pool is justified twice. cgo runs a `c-shared` export **on the calling
thread** rather than handing it to Go's scheduler, so a single marshalling
thread would serialise every render in the process onto one core. It is also the
only clean holder of the fix: a bounded set of long-lived threads whose signal
stacks the binding sizes once at creation.

One consequence to accept deliberately: a caller with far more threads than
cores previously had that many concurrent Go renders and will now queue at the
pool. Rendering is CPU-bound, so this is throughput-neutral-to-better, but it is
an observable change under high concurrency. Story 4 measures it.

## Verification

**Commands:**
- `dotnet build folio-dotnet/test/Folio8.Net46Compile/Folio8.Net46Compile.csproj -c Release` -- expected: succeeds, 0 warnings
- `dotnet test folio-dotnet/test/Folio8.Tests -c Release` -- expected: all pass, including the new engine-thread tests and the unchanged golden corpus hashes
- `folio-dotnet/build/build-native.sh host && dotnet test folio-dotnet/test/Folio8.Tests -c Release` -- expected: same, against the host native
- **On Linux, against a real shipped-shape native** -- `folio-dotnet/build/build-native.sh linux-arm64`, copy it to `build/native/host/libfolio8_native.so`, then `docker run --platform linux/arm64 -v "$PWD:/repo" -w /repo mcr.microsoft.com/dotnet/sdk:10.0 dotnet test folio-dotnet/test/Folio8.Tests -c Release` -- expected: all pass, the engine-thread mechanism assertions among them. This is what makes the Linux acceptance criterion checkable before story 3 wires CI.
