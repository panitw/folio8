---
title: 'Corpus byte-identity on Linux in CI, on both architectures'
type: 'feature'
created: '2026-09-23'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '7f6a936'
context:
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `folio-dotnet-linux` builds both shipped Linux natives, checks their
architecture and their glibc floor, stages the `linux-x64` one as the test
native — and then stops, because running the suite meant entering the engine on
CLR thread-pool threads. Story 2 removed that reason. Until the step comes back,
nothing renders a single document through the binding on Linux in CI, and the
`linux-arm64` native is built and machine-checked but never executed at all.

**Approach:** Restore the `dotnet test` step in that job, and add an arm64 leg so
both shipped natives are exercised rather than only inspected.

## Boundaries & Constraints

**Always:**
- **The suite runs against the SHIPPED native, not a host build.** The job already stages `linux-x64` from the pinned image into `build/native/host/` for exactly this reason; the arm64 leg must do the equivalent with `linux-arm64`.
- **Replace the withdrawal comment, do not delete it.** `ci.yml`'s block at the end of that job explains why the step was removed. Rewrite it to say what is true now and what changed — the history is why the step is trusted this time.
- **Emulation is acceptable in this story, and only in this story.** Rendering is deterministic CPU work, so a SHA-256 is a SHA-256 whatever executed it. That exemption does not extend to story 5's soak, and this job's greenness must not be described anywhere as clearing DW-396.
- **Keep the glibc-floor and ELF-machine checks ahead of the tests.** They are provenance checks on the artefact; a suite that passes against a native from the wrong image proves the wrong thing.
- **No `continue-on-error`**, consistent with every other job in this file.

**Never:**
- Does not re-add the Linux RIDs to the package or touch `PackagingTests`' ban — story 6, after the soak.
- Does not add a soak, a repeat count, or any timing-based assertion. A run of passes is not evidence and must not be presented as any.
- Does not change `build-native.sh`, `verify-linux-natives.sh`, or the pinned image.
- Does not weaken or skip any existing assertion to make a leg pass.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| x64 leg | Shipped `linux-x64` native staged as the test native | Whole suite runs; every corpus SHA-256 matches the committed hash | Job fails on any mismatch |
| arm64 leg | Shipped `linux-arm64` native staged likewise | Same suite, same hashes — byte identity is architecture-independent | Job fails on any mismatch |
| Native missing or wrong machine | `verify-linux-natives.sh` fails | Job fails **before** any test runs | Fail fast |
| Engine-thread assertions under emulation | `EngineThreadTests` reads the altstack on an emulated host | Must still pass; if the enlargement cannot be observed under emulation, the leg fails rather than skipping | Fail, do not skip |

</frozen-after-approval>

## Code Map

- `.github/workflows/ci.yml:668` `folio-dotnet-linux` -- `runs-on: ubuntu-24.04`, 45-minute timeout. Steps today: checkout, setup-dotnet, `docker/setup-qemu-action@v3` (arm64), `build-native.sh linux-x64 linux-arm64`, `verify-linux-natives.sh`, stage `linux-x64` into `build/native/host/`.
- `.github/workflows/ci.yml:709-723` -- the withdrawal comment that ends the job. This is what gets replaced by the restored step.
- `.github/workflows/ci.yml:1031` `folio-dotnet-host` -- already runs `dotnet test` on ubuntu against a **host** build (runner glibc). Different purpose; leave it alone, but the new step must not duplicate it pointlessly — this one tests the shipped artefact.
- `folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj:15` -- non-Windows target framework list is `net10.0` alone.
- `env.DOTNET_VERSION` at `ci.yml:45` -- `10.0.400`.
- The arm64 runner question: GitHub-hosted `ubuntu-24.04-arm` exists and executes natively; the job already has qemu configured for the build. **Prefer a native arm64 runner if available to this repository** — it is faster and the measurement is honest; fall back to qemu under the emulation exemption above, and say in a comment which was chosen and why.
- Verified locally before this story: the suite passes on linux/arm64 against a real `linux-arm64` native from the pinned image (161/161, five consecutive runs), so the arm64 leg is expected to be green rather than exploratory.

## Tasks & Acceptance

**Execution:**
- [x] `.github/workflows/ci.yml` -- restore `dotnet test folio-dotnet/test/Folio8.Tests -c Release` to `folio-dotnet-linux` after the staging step, replacing the withdrawal comment with what is now true.
- [x] `.github/workflows/ci.yml` -- add the arm64 leg: stage the shipped `linux-arm64` native and run the same suite against it, natively if an arm64 runner is available, otherwise under emulation with the choice explained in a comment.
- [x] `.github/workflows/ci.yml` -- make sure a failing leg names which architecture and which native it ran against, so a red job is diagnosable without opening the log.

**Acceptance Criteria:**
- Given the `folio-dotnet-linux` job, when it runs, then the suite executes twice — once per shipped native — and every corpus SHA-256 matches its committed hash on both.
- Given a native that fails the glibc-floor or ELF-machine check, when the job runs, then it fails before any test executes.
- Given a corpus hash that differs on either architecture, when the job runs, then the job fails naming the architecture.
- Given the job passes, when a reader looks at the workflow, then nothing in it claims or implies that DW-396 is thereby closed.

## Implementation Notes

The matrix job is `folio-dotnet-linux-matrix` and a fan-in job keeps the name
`folio-dotnet-linux` (`needs:` + `if: always()` + `test "<result>" = "success"`),
in the idiom `folio-js` already uses at ci.yml:496 — splitting the job would
otherwise have renamed its check and left any branch protection, and
`RELEASING.md`'s release gate, naming a check that no longer exists.

The legs are `fail-fast: false`, one per shipped native, each on real silicon:

| leg | runner | native executed | native built under qemu |
|---|---|---|---|
| `linux-x64` | `ubuntu-24.04` | `linux-x64` | `linux-arm64` |
| `linux-arm64` | `ubuntu-24.04-arm` | `linux-arm64` | `linux-x64` |

The native arm64 runner was chosen over qemu because `ubuntu-24.04-arm` is
already in use by `matrix.yml`'s `render-linux-arm64` job, so availability to
this repository is demonstrated rather than assumed. The emulation exemption is
therefore unused; the comment in `ci.yml` records qemu as the fallback if that
runner class ever goes away. The stated reason is speed and the honesty of
running the shipped aarch64 native on an aarch64 CPU — NOT the engine-thread
assertions: `EngineThreads.SignalStackBytes` is a fixed 1 MiB constant and
`EngineThreadTests` asserts equality with it, so that reading is
architecture-independent and identical under emulation. Each leg still builds **both** natives — it must,
because `verify-linux-natives.sh` checks the pair and is not being changed —
but only stages and executes its own.

Order is unchanged and load-bearing: build → `verify-linux-natives.sh` →
stage this leg's RID into `build/native/host/` → `dotnet test`. A native that
fails the ELF-machine or glibc-floor check fails the job before any test runs.

The staging step ASSERTS rather than prints: the staged ELF's machine must be
this RID's, and the runner's own `uname -m` must match it too, so a leg that
staged the other architecture's native — or ran on the wrong runner — fails
instead of reporting byte identity for bytes it never executed.

Diagnosability: the leg name carries the RID, and one `if: failure()` step
names the leg, the architecture and the PHASE — build, provenance, staging or
suite — since a `build-native.sh` failure under qemu and a
`verify-linux-natives.sh` rejection are now also architecture-ambiguous. The
suite message deliberately states what failed and where to look, not why: a
corpus difference and a dead test host both arrive there and the step cannot
tell them apart.

The suite runs with `--blame-crash --blame-hang --blame-hang-timeout 15m`,
which no other test step in this file needs: this is the one job whose
expected failure mode is the host dying from a signal handler, and a dead host
writes no `.trx` at all. The whole `TestResults/` directory is uploaded per leg
(guarded on the suite having run), so a crash leaves a dump and a `Sequence.xml`
rather than an empty red step.

`timeout-minutes` went 45 -> 60, with the reasoning in the file: each leg now
also restores, builds and runs the suite, and each builds the other
architecture's native under qemu. Measured datapoint: the mirror build
(`linux-x64` on an arm64 host) took 1m03s under Rosetta, and qemu on a runner
is slower than Rosetta.

The withdrawal comment was rewritten, not deleted: it now carries the history
(154/154 and 0-in-11 were true and were not evidence), what changed (story 2's
binding-owned engine threads with enlarged signal stacks, asserted by
`EngineThreadTests` on the crossing thread), and an explicit statement that a
green run here does **not** close DW-396 and that the Linux RIDs stay unpacked
until the soak.

Three records that described the withdrawn state were corrected, minimally and
without expanding into story 7's release-record rewrite: `RELEASING.md` step 2
("deliberately does NOT run the suite against them"), its withdrawal block
(~line 311) and its build list (~line 458); and the DW entry filed by story 2
("folio-dotnet-linux runs no `dotnet test`"), which now keeps its history and
records that the CI half of the gap is closed while the packing half remains
story 6's. No other withdrawal-surface site was touched: no RID re-added, no
`PackagingTests` ban weakened.

## Spec Change Log

## Review Triage Log

Three layers, all launched before any result was read. Verdicts mine.

| # | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|
| 1 | The matrix renamed the job's checks, so the check `folio-dotnet-linux` ceased to exist and no fan-in kept the name (all three layers) | **high** | patch | Verified against `ci.yml:496-514`, where this file already solved the identical problem for `folio-js` and its comment names the failure: "a required gate that can never go green, or worse, one quietly dropped". `RELEASING.md`'s release gate leans on this job by name. Fan-in added in the same idiom. |
| 2 | The comment justified the native arm64 runner on a reading the tests do not take (edge-case, verification-gap) | **medium** | patch | Confirmed: `EngineThreads.SignalStackBytes` is a fixed `1024*1024` and `EngineThreadTests` asserts equality with it, so the altstack reading is architecture-independent and identical under qemu. The runner choice stands on speed and on running the shipped aarch64 native on an aarch64 CPU; the stated reason was what was wrong. In a repository with DW-148 open on comments that claim things they do not control, this is not cosmetic. |
| 3 | The staging step echoed `uname -m`, `file` and `sha256sum` but compared none of them (blind-hunter, edge-case, verification-gap) | **medium** | patch | True — a human-only check, while the comment beside it says a leg staging the other architecture's native "would be worse than no leg at all". Now `readelf -h` must report the leg's machine and `uname -m` must match, or the step fails naming which. |
| 4 | `timeout-minutes: 45` inherited unchanged although each leg's work roughly doubled (blind-hunter, edge-case) | **medium** | patch | Real. I measured the mirror case: the emulated `linux-x64` build on an arm64 host took 1m03s under Rosetta, and qemu on the runner is slower than Rosetta. Raised to 60 with that datapoint recorded, so the number is a decision. |
| 5 | A native crash writes no `.trx`, so the upload guard and the annotation both misreport the one failure mode this job exists to catch (edge-case) | **medium** | patch | Correct and specific to this job. Suite now runs with crash/hang blame and uploads the whole results directory, so a crashed host leaves a dump. The annotation no longer asserts a cause — its "Neither is a DW-396 signal" claim was false for a crash-class failure. |
| 6 | The `::error::` annotation fired only on the suite step, leaving build and provenance failures unannotated (blind-hunter) | **low** | patch | True, and "which leg, which architecture" is a question this change newly created. Now one `if: failure()` step emits a phase-specific annotation. |
| 7 | `foreign: arm64` / `foreign: amd64` used two vocabularies, and the upload condition was a long spelling of "the step ran" (blind-hunter, edge-case) | **low** | patch | Both confirmed; direct corrections. |
| 8 | The story-2 deferred-work entry became false with this change (verification-gap) | **medium** | patch | Verified: it said "folio-dotnet-linux runs no `dotnet test`", and it is the entry that explicitly says it was "recorded so the gap is not mistaken for coverage in the meantime". Updated to record what now runs and what remains story 6's, keeping the original wording as history. |
| 9 | Two further `RELEASING.md` descriptions of this job were left stale (blind-hunter, verification-gap) | **low** | patch | Confirmed at ~311 and ~458. Brought in line minimally; story 7 owns the wider rewrite. |
| 10 | The frozen I/O matrix row about emulation describes a scenario the implementation eliminated by choosing native silicon (blind-hunter) | **low** | rejected | Accurate observation, but the row is inside `<frozen-after-approval>` and the triage rules forbid a fix whose change is to edit this build's spec. The row was written as a conditional — if emulation were used, it must not skip — and emulation was not used. Recorded here so the reader is not left checking an untested requirement. |
| 11 | Neither leg has a CI run behind it; `actionlint` proves syntax, not that the job works (blind-hunter, verification-gap) | **medium** | defer | True and unfixable from here: this session has no amd64 host and cannot push. Mitigated rather than closed — the arm64 leg was reproduced locally against the shipped native with the leg's exact flags, both natives were built and passed `verify-linux-natives.sh` in a container, and the emulated-build cost was measured. The x64 leg and the arm64 runner's qemu-amd64 build remain unrun until CI runs them. |

**One item for the owner that no patch can reach:** if branch protection on this
repository requires the check `folio-dotnet-linux`, the fan-in job added here is
what keeps that name reporting. Branch-protection settings are not in the
repository and cannot be verified from a session.


## Design Notes

The x64 and arm64 legs are the same suite against different bytes, and that is
the point: byte identity is the property, and it is architecture-independent by
design. A leg that passed only because it ran the other architecture's native
would be worse than no leg, which is why the staging step and the machine check
are both upstream of the tests.

## Verification

**Commands:**
- `actionlint .github/workflows/ci.yml` (or `gh workflow view`) -- expected: the workflow parses
- Locally reproducible equivalent of the arm64 leg: `folio-dotnet/build/build-native.sh linux-arm64`, copy to `build/native/host/libfolio8_native.so`, then `docker run --platform linux/arm64 -v "$PWD:/repo" -w /repo mcr.microsoft.com/dotnet/sdk:10.0 dotnet test folio-dotnet/test/Folio8.Tests -c Release` -- expected: all pass

**Results (2026-09-23):**
- `actionlint .github/workflows/ci.yml` -- the only finding is the pre-existing SC2086 info at line 521, identical on `HEAD`; nothing new in this job, and `ubuntu-24.04-arm` is accepted as a runner label.
- arm64 leg reproduced locally on Apple Silicon (native arm64, not emulated), shipped `linux-arm64` native staged as the test native: **161/161 passed, 0 failed, 0 skipped** — re-run with the leg's exact `--blame-crash --blame-hang --blame-hang-timeout 15m` and trx flags, same result, `.trx` written to the results directory.
- The x64 leg could not be reproduced locally (no amd64 host available); it is the same steps as before the withdrawal, with the `dotnet test` step restored.
