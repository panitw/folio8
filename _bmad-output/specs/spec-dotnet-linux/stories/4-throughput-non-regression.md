---
title: 'Throughput non-regression across the new boundary'
type: 'feature'
created: '2026-09-23'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '3005db9'
context:
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** CAP-10. Every render now takes a queue hop onto an engine thread
instead of crossing on the caller's own thread. The owner chose a pool over a
single thread specifically so throughput would survive, which makes "it
survived" an acceptance criterion rather than a curiosity — and nothing measures
it. Two distinct risks are unmeasured: that Windows, which took this change
without needing it, pays for it; and that Linux flattens at one core, which
would mean the pool is not actually buying the parallelism it was chosen for.

**Approach:** A small benchmark harness that renders a corpus document at
increasing concurrency and reports throughput, run against the binding **before**
and **after** the engine-thread change so the comparison is measured rather than
argued.

## Boundaries & Constraints

**Always:**
- **The baseline is real code, not a remembered number.** Compare against the binding as it was before the engine threads landed — a worktree at that commit is the honest way; a bypass switch inside the shipped binding is not, because it would add surface that exists only for a benchmark.
- **Report the shape, not one number.** Throughput at 1, 2, 4 and `ProcessorCount` concurrent callers, so "scales with cores" and "flattens at one" are distinguishable.
- **Say what the numbers are worth.** Record the host, its architecture, whether it was translated, and that a benchmark on a developer machine is indicative — the same provenance discipline story 1 established for measurements.
- **A benchmark is not a gate.** It reports; a human reads it. No CI job, no pass/fail threshold baked into a test.
- **`src/Folio8` stays free of `PackageReference` and stays on the floor.** A harness project outside it may take dependencies, but prefer none — the existing corpus fixtures and a stopwatch are enough.

**Never:**
- No change to `src/Folio8` at all. If the benchmark needs a seam, the benchmark is wrong.
- No tuning, no pool-size knob, no attempt to beat 1.1.0 — CAP-10 is non-regression, and SPEC.md's non-goals put optimisation out of scope.
- No claim that a benchmark result says anything about DW-396 or about crash-freedom.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Single caller | Concurrency 1 | Throughput reported for before and after; the hop cost is visible here in isolation | N/A |
| Concurrency = `ProcessorCount` | Many callers | After-throughput scales materially above the single-caller figure, rather than flattening | N/A |
| Concurrency above pool size | More callers than engine threads | Completes; throughput does not collapse | N/A |
| Translated host | Rosetta or qemu | Reported with its provenance and marked indicative | N/A |
| Baseline worktree unavailable | The before-commit cannot be built | Fails saying so, rather than reporting an after-only number as a comparison | Fail loudly |

</frozen-after-approval>

## Code Map

- `folio-dotnet/src/Folio8/EngineThreads.cs` -- the boundary under measurement. `SignalStackBytes`, the pool sized to `Environment.ProcessorCount` (floored at 2), inline execution when already on an engine thread.
- `7f6a936` -- the engine-thread commit. Its parent is the baseline: the binding with the direct-`DllImport` path.
- `folio-dotnet/build/signal-stack-probe/` -- the shape to copy for a developer-run measurement tool: console project under `folio-dotnet/build/`, a shell wrapper beside it, house-style rationale header, provenance reported rather than assumed. Not shared code — a second small tool.
- `folio-dotnet/test/Folio8.Tests/Repo.cs` -- how the test project locates the repo root and the corpus manifest; useful for finding a fixture to render.
- `fixtures/` -- the corpus. Pick one representative document and say which; a benchmark whose input is unnamed is not reproducible.
- `folio-dotnet/build/native/host/libfolio8_native.so` -- where the binding looks for the native on non-Windows; the benchmark needs one staged, same as the tests.

## Tasks & Acceptance

**Execution:**
- [x] `folio-dotnet/build/render-throughput/` -- new console harness: renders a named corpus fixture at concurrency 1, 2, 4 and `ProcessorCount`, reporting renders/second and per-render latency, with host provenance in the header.
- [x] `folio-dotnet/build/measure-throughput.sh` -- wrapper that builds the binding at the baseline commit in a git worktree and at `HEAD`, runs the harness against each, and prints them side by side. House style: rationale header, literal usage block, argument validation before any work, failures naming their remedy.
- [x] `_bmad-output/implementation-artifacts/` -- record the measured before/after table with its host provenance, so the release notes have a source. Do not put it in `sigaltstack-findings.md`, which is `bmad-spec`-owned.

**Acceptance Criteria:**
- Given the wrapper on a Linux host, when it runs, then it prints a before/after throughput table at four concurrency levels with the host and its provenance named.
- Given the after-binding at concurrency `ProcessorCount`, when compared with concurrency 1, then throughput is materially higher — demonstrating the pool buys parallelism rather than serialising.
- Given the before/after comparison, when read, then any regression is visible as a number rather than a claim.
- Given a host where the baseline worktree cannot be built, when the wrapper runs, then it fails saying so rather than presenting an after-only figure as a comparison.

## Implementation Notes

**The before/after mechanism is an MSBuild property, not a switch in the binding.**
`render-throughput.csproj` takes `FolioBindingProject`, and its `ProjectReference`
points at whatever that names. `measure-throughput.sh` copies the harness sources
into a per-leg directory and builds them twice — once against
`folio-dotnet/src/Folio8/Folio8.csproj` in this tree, once against the same file
inside a `git worktree` at `7f6a936^`. `src/Folio8` is not touched by this story at
all, and there is no seam in it for a benchmark to use. A per-leg copy rather than
one in-place build is deliberate: two builds sharing an `obj/` could hand one leg the
other's `Folio8.dll`, which is precisely the failure this story exists to detect —
two measurements that look comparable and are not.

**The engine is held fixed while the binding varies.** Both legs stage the **same**
native library file, chosen once by the wrapper and passed in as `FolioNativeFile`.
Letting each leg stage its own worktree's native would vary two things at once. On
Linux the wrapper prefers the **shipped** RID native (`build/native/linux-x64` or
`linux-arm64`) over `build/native/host/`, because measuring what the package actually
carries is the point.

**The harness verifies before it times.** The fixture is rendered once and its
SHA-256 checked against the committed `expected.json` before any timed loop starts.
A build that renders the wrong bytes quickly would otherwise be reported as a
throughput win. `4b0367f5…` matched on every leg.

**Callers are dedicated threads, not the CLR thread pool.** The level has to *be* the
number asked for; the pool grows on its own schedule and would make "four concurrent
callers" mean whatever it decided. Every worker is started and parked on a gate
before any of them renders, so thread creation is outside the timed window.

**`/proc/cpuinfo`'s `vendor_id` is the translation signal, and that is story 1's
measured finding rather than a guess repeated here.** The first cut of
`DescribeTranslation` said "unknowable from inside" on Linux. Story 1 had already
established that inside a Docker Desktop `linux/amd64` container on Apple Silicon,
`uname` says `x86_64`, `/run/rosetta` is absent and `binfmt_misc` is empty — but
`vendor_id` reads `VirtualApple`. The harness now claims **only** that one conclusive
reading and points at `probe-signal-stack.sh` for the full four-signal derivation; a
second, weaker copy of that derivation inside a benchmark could only disagree with
the first. Confirmed working: leg C stamped
`YES — /proc/cpuinfo vendor_id is 'VirtualApple'`.

**The wrapper stamps what the harness cannot see.** A process inside a
`linux/<arch>` container reads the emulated architecture from `uname`. Outside it,
both halves are known, so `measure-throughput.sh` computes the verdict — `native on
this host's silicon` or `TRANSLATED (linux/amd64 on a arm64 host)` — and passes it in
as `--host-context`.

**`safe.directory` is passed as environment, not written as config.** A bind-mounted
repository presents the host's ownership, which git inside the container reads as
"dubious ownership" and refuses — and the refusal surfaced as
`cannot resolve 7f6a936^`, which reads like a shallow clone and sends the reader
somewhere useless. `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_0` scope the exception to the
one container and the one path; `git config --global` would leave it behind.

**Two baseline failure modes, two different responses.** A baseline that will not
**build** ends the run: there is no comparison to be had, and an after-only number is
not one. A baseline that builds and then **dies mid-run** is a result worth printing
— on Linux that is the shape of DW-396 itself — so the after leg still runs, but what
comes out is headed `AFTER ONLY — THIS IS NOT A COMPARISON` and the script exits
non-zero. Both paths were exercised (see Verification).

**`.gitignore` needed no change.** After a full Release build,
`git status --untracked-files=all` lists only `Program.cs` and the `.csproj` under
`folio-dotnet/build/render-throughput/`; `bin/` and `obj/` are already matched, and
the sources are re-included by the existing `!/folio-dotnet/build/` negation — the
same arrangement story 1 confirmed for `signal-stack-probe`.

**Measurements are recorded in
`_bmad-output/implementation-artifacts/dotnet-linux-throughput.md`**, not in
`sigaltstack-findings.md`, which is `bmad-spec`-owned.

### Corrections made after review

**Correctness is now checked under concurrency, not only once.** The single-threaded
digest check before the timed loops said nothing about N callers crossing at once —
which is the failure this change could plausibly introduce. Every caller at every
level now hashes its first render, inside the warmup so no digest arithmetic lands
in a timed window, and the loop is `while (pending || …)` so it holds even at
`--warmup 0`.

**A leg that dies is classified before it is narrated.** Exit 1 or 2 is the harness
*refusing* — a bad argument, a missing fixture, a digest mismatch — and is reported
as that. Only `>= 128` is a signal death, and only SIGSEGV, SIGABRT or SIGBUS is
narrated as DW-396's shape; any other signal says so and attributes nothing. The
first cut sent every non-build failure down the "check dmesg for an overflowed
sigaltstack" path, which is the exact misattribution this epic exists to prevent —
and the amd64 leg then produced signal 34, proving it would have fired.

**The wrapper validates by value, not by spelling.** `THROUGHPUT_SECONDS=0` and
`THROUGHPUT_LEVELS=1,,2` passed a digits-and-dots shape check and were rejected by
the harness minutes later, after a worktree checkout and two builds — where the
failure read as "the binding did not complete the run (exit 2)".

**Legs alternate and repeat.** `THROUGHPUT_REPEATS` runs before, after, before,
after…, and the comparison carries a spread column. Without it, differences of a few
percent on a host with several percent of drift were being attributed to the binding.

**The after leg is described by what it is.** It links the working tree, so the
wrapper checks `git status -- folio-dotnet/src/Folio8` and either says "HEAD <sha>
(clean)" or warns loudly that the numbers are a working tree's and must not be
recorded against a commit.

**Logs survive the run.** `THROUGHPUT_LOG_DIR` keeps every leg and build log; the
recorded runs used it, and the logs are committed under
`_bmad-output/implementation-artifacts/evidence/dotnet-linux-throughput/`. The root
`.gitignore` excludes `*.log`, so that directory carries a scoped `!*.log` negation —
these logs are the record, not build noise.

**The comparison prints median and p95, and the union of levels.** The mean alone hid
this harness's most load-bearing reading: a leg whose median was unchanged and whose
p95 had doubled. A level present in one leg and not the other is now printed as a gap
rather than dropped.

**A build failure on the AFTER leg gets its own message** — "a compile failure is not
a measurement" — rather than the baseline leg's "that is a result". Both legs are now
built before either is run.

**The pool annotation is a property of one leg, and says it can drift.**
`--pool none` suppresses it for the baseline, which has no pool at all. The constant
is a restatement of `EngineThreads.Start()`'s sizing rule that nothing links to it;
the comment in `WriteTable` says so.

**The level set says when it collapses.** The default 1, 2, 4, `ProcessorCount`
deduplicates to three levels on a 2- or 4-core host; the table now names that rather
than silently printing fewer rows than the acceptance criterion asks for.

## Spec Change Log

## Review Triage Log

Three layers, launched together. Verdicts mine.

| # | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|
| 1 | Every non-build baseline failure is narrated as DW-396, including a digest mismatch or a bad argument (all three layers) | **high** | patch | Confirmed by reading `run_leg`: only a build failure returns the 90 sentinel, so exits 1 and 2 reached the "check `dmesg` for an overflowed sigaltstack line" branch. This is the misattribution the whole epic exists to prevent, inside the epic's own tooling. **It then fired for real**: the amd64 leg produced signal 34, which the old code would have called an overflowed sigaltstack. Now only SIGSEGV/SIGABRT/SIGBUS gets that narrative, and 1/2 is reported as the harness refusing. |
| 2 | The validate-before-any-work block accepts values its own messages call illegal (blind-hunter, edge-case, verification-gap) | **medium** | patch | Reproduced: `THROUGHPUT_SECONDS=0` and `THROUGHPUT_LEVELS=0`/`1,,2` passed the shell guards and were rejected minutes later by the harness, after a worktree checkout and two builds — and the failure then read as "the CURRENT binding did not complete the run", pointing at the binding. Validation is now by value, not spelling. |
| 3 | Correctness is checked once, single-threaded, in a story about concurrency (blind-hunter) | **medium** | patch | Real: inside the timed loops only `Bytes.Length != 0` was asserted, so a binding that corrupts under concurrent crossings would have been counted as throughput. Every caller at every level now hashes its first render, inside warmup so no digest work enters a timed window. |
| 4 | Legs never interleaved, single number per cell, on a host the record says has several percent spread (blind-hunter) | **medium** | patch | Correct, and conclusions were already being drawn from those numbers. `THROUGHPUT_REPEATS` now alternates the legs and the table carries a spread column. This changed the reading: the concurrency-10 figure moved to −1.2 % against ±3.8 % spread, i.e. noise, where a single run had said +2.7 %. |
| 5 | The after leg links the working tree while the output is labelled HEAD (edge-case) | **medium** | patch | Real: uncommitted binding edits would be recorded as HEAD. The wrapper now checks and either names the sha or warns loudly. |
| 6 | The macOS single-caller deficit was closed in prose, and the scaling claim was an artifact (blind-hunter) | **medium** | patch | Both verified. "4.14x vs 3.69x" divides each leg by its own concurrency-1 figure, so a depressed denominator reads as improved scaling — the same figure that section called a regression. Corrected, with the error named; the macOS tail is now filed rather than dismissed. |
| 7 | Raw logs deleted on exit, so the record cannot be reproduced (blind-hunter) | **medium** | patch | True, and the artifact was presented as the source for release notes while its tables were retyped from a vanished terminal. Logs are now kept and committed under `evidence/`. |
| 8 | Side-by-side table drops median and p95; levels present in before but absent from after vanish silently (blind-hunter, edge-case) | **medium** | patch | Both confirmed. The dropped columns are what the record's most load-bearing reading rests on. Union of levels now iterated, gaps named. |
| 9 | The `*` pool annotation is printed on the baseline leg, which has no pool (blind-hunter, verification-gap) | **low** | patch | Asserted something untrue about the build being measured, and was an unlinked restatement of `EngineThreads.Start()`'s rule. Baseline now says it has no engine pool. |
| 10 | Default level set collapses below four levels on small hosts, silently (edge-case) | **low** | patch | Verified with `DOTNET_PROCESSOR_COUNT=2`. Now disclosed in the output. |
| 11 | A failed AFTER build is printed as "a result, not a benchmark failure" (blind-hunter) | **low** | patch | Language meant for the baseline leg; a compile failure is not a measurement. Mirrored. |
| 12 | Nothing builds or runs `render-throughput`; no task owns CAP-10's release-notes step (blind-hunter, verification-gap) | **medium** | defer | Both verified absent. Story 1 filed the same gap for `signal-stack-probe`; this adds a second such project plus a long wrapper. Filed, along with the release-notes obligation SPEC.md's CAP-10 actually names. |
| 13 | Worker `Start()` throwing mid-loop parks already-started threads forever; `--levels` has no ceiling; `expected.json` digest parse can rescan from byte 0 (edge-case) | **low** | rejected | All three real but confined to a developer-run tool with hand-typed arguments, and each fix adds a branch for a case nobody meets. Rejected under the low-severity rule, recorded so the reasoning is on record. |
| 14 | The timed window includes stragglers, overstating elapsed time as concurrency rises (blind-hunter) | **low** | rejected | Real and correctly reasoned (~0.5 % at c=1, ~5 % at c=40). It biases both legs identically, so the before/after comparison — the only thing this story claims — is unaffected. It would matter for an absolute figure, which the record does not publish. |

**What the review changed about the conclusions, not just the code.** The amd64
leg was re-run three times and **all three died** — before (signal 34), after
(SIGABRT), before (SIGSEGV). Both bindings, same host, same native. The earlier
−11.5 % amd64 figure is **withdrawn**, and no amd64 number is reported.


## Design Notes

The single-caller figure is the one that isolates the hop cost, and the
`ProcessorCount` figure is the one that answers the question the owner actually
asked when choosing a pool over a single thread. Reporting only an aggregate
would hide whichever of the two went wrong.

## Verification

**Commands:**
- `folio-dotnet/build/measure-throughput.sh` -- expected: a before/after table at four concurrency levels, with provenance
- The same inside a `linux/arm64` container with a shipped native staged, so the figures come from the platform this epic is about

**What was run, and what came back** (full tables, provenance and the committed
leg logs behind every number in
`_bmad-output/implementation-artifacts/dotnet-linux-throughput.md` and
`_bmad-output/implementation-artifacts/evidence/dotnet-linux-throughput/`):

Every recorded run is `THROUGHPUT_REPEATS=3` with `THROUGHPUT_LOG_DIR` set, so each
cell has a spread beside it and a log file behind it.

| Run | Result |
|---|---|
| `measure-throughput.sh arm64` — linux/arm64 container, **native silicon**, shipped `linux-arm64` native | Before/after at 1, 2, 4, 10 over three alternating runs per leg: +5.7 %, +2.7 %, +4.1 %, −1.2 %, every one at or inside the legs' own ±0.6–3.8 % spread. **Scaling after: 3.63×**. Median and p95 track each other on both legs — no tail. |
| `THROUGHPUT_LEVELS=1,2,4,10,40 measure-throughput.sh arm64` (n=1) | Completes at four times the pool size; 164 r/s against 160 r/s at concurrency 10, latency 62 ms → 240 ms. Queueing, not collapse. n=1, so the −3.5 % against before is inside drift and is not treated as a finding. |
| `measure-throughput.sh` — darwin/arm64 host, .NET 10 | −8.2 % at concurrency 1, −8.5 % at 2, neutral at 4 and 10 — outside both legs' spread, so real. But the **median** render is unchanged (19.99 vs 20.35 ms) and **p95 nearly doubles** (22.33 → 41.78 ms): a handoff wakeup tail, not a slower render, and absent on Linux. **Filed** in `deferred-work.md` rather than closed in prose. |
| `measure-throughput.sh amd64` — **Rosetta-translated** | **No comparison could be taken.** Three attempts, each killed by `Stack overflow.`: before (signal 34), after (SIGABRT), before (SIGSEGV). Both bindings die on the same host with the same native, so nothing is attributed to the engine-thread change, and the first pass's −11.5 % is **withdrawn**. |
| `THROUGHPUT_SECONDS=0`, `=0.0`, `THROUGHPUT_LEVELS=0`, `=1,,2`, `=1,0,2`, `THROUGHPUT_REPEATS=0`, `THROUGHPUT_WARMUP=x` | Each rejected by value, not by spelling, before any worktree or build. |
| `THROUGHPUT_FIXTURE=<an empty fixture dir>` | The before leg's harness exits 2; the wrapper reports "refused by the harness itself", prints no DW-396 narrative and no AFTER-ONLY block, and exits 1. |
| `THROUGHPUT_ENGINE_COMMIT=deadbeef` / `=05e4ccd` | Unresolvable baseline → refuses naming the shallow-clone remedy. Unbuildable baseline → exits 1 without running the after leg. Matrix row *Baseline worktree unavailable*. |
| `describe_exit` over 1, 2, 134, 139, 3 | `refused by the harness itself` / `killed by signal 6` / `killed by signal 11` / `exit 3`. |
| Level union and gap note, exercised against crafted logs | A level present in one leg only is printed with `-` in the other column and named under the table rather than dropped. |
| `--pool none` on the before leg | Its table prints "This binding has no engine pool"; the oversubscription annotation appears on the after leg only. |
| `--warmup 0` with per-caller verification | 3 renders hashed at levels 1,2 — one per caller per level — proving the verify pass does not depend on there being a warmup window. |

After every run `git worktree list` shows only the main tree and `git status --porcelain`
is unchanged apart from this story's own new files.

**Not covered — the open half of CAP-10, both now filed in `deferred-work.md`.** No Windows
leg was taken; CAP-10 names Windows x64 explicitly, and the harness is portable while the
wrapper and its worktree drive are POSIX. Real amd64 silicon is likewise uncovered — leg C
is translated and could not complete a comparison — and is best taken alongside CAP-5's
real-hardware soak. Also filed: nothing builds or runs `render-throughput` in CI though it
`ProjectReference`s `src/Folio8`; and no task owns copying these numbers into the 1.2.0
release notes, which is what CAP-10's success criterion actually asks for.
