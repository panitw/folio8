---
title: 'Land the signal-stack probe as a repo diagnostic, and confirm the amd64 constant'
type: 'feature'
created: '2026-09-22'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '666019fc4d4fb8bd9b80bd3f85a384240dcf6439'
context:
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/sigaltstack-findings.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** DW-396's mechanism was settled by a throwaway probe in a scratch
directory — the CLR installs a fixed-size alternate signal stack on every thread
it touches (24 KiB arm64, 16 KiB amd64) while Go sizes its own at 32 KiB and,
under cgo, adopts whatever it finds. That reading underpins the whole
`SPEC-dotnet-linux` design and lives nowhere a person can re-run it. The amd64
figure is worse than unpreserved: it was taken under emulation.

**Approach:** Land the probe as a supported diagnostic beside the other
native-layer checks in `folio-dotnet/build/`, with a one-command invocation so
the amd64 leg can be run on hardware this session cannot reach. It reports and
asserts nothing — the judgement stays with the reader.

## Boundaries & Constraints

**Always:**
- A **diagnostic, not a test**: it prints measurements and exits, and is wired into no CI job or suite in this story.
- **Not** bound by the package's `netstandard2.0` / `net46` / LangVersion 7.3 floor — developer tooling, modern C# is fine.
- House style of `folio-dotnet/build/*.sh`: rationale header, literal usage block, script-relative path discovery, arguments validated before any work, failures to stderr prefixed with the tool's name and naming the remedy.
- Report the CLR's installed size **and** `sysconf(_SC_SIGSTKSZ)` — the finding is that the two disagree and the CLR's is a constant.
- Measurements go under **DW-396 in `_bmad-output/implementation-artifacts/deferred-work.md`**, a dated subsection matching the existing ones. That is the repository's evidence ledger and DW-396 is still OPEN; `sigaltstack-findings.md` is `bmad-spec`-owned and is not edited from here.
- **Every row states its provenance, and the probe determines it rather than trusting the operator.** It reports whether the host is executing natively or under translation (Rosetta or qemu — `/run/rosetta` and `/proc/sys/fs/binfmt_misc` are the available signals) so a translated row can never be mistaken for a real-silicon one. This is the discipline DW-396 was burned by twice.

**Decisions (owner, 2026-09-22):**
- Story 1 **completes now**. Legs runnable on this machine are recorded, including the **amd64 leg under Rosetta** — written into DW-396 with its translated provenance attached and explicitly marked non-admissible as soak evidence.
- The **real-amd64 and native-arm64 confirmation runs are the owner's, happening 2026-09-23** on the WSL2 box. DW-396's new subsection carries a **PENDING** row naming exactly what is still outstanding, so it is visible rather than forgotten.
- Stories 2–6 are not blocked on that number: the fix is arch-independent, so the measurement sharpens the record without changing the design.

**Never:**
- No `LICENSE` file in the new directory — `lint/internal/licence/licencecensus_test.go` pins every committed `LICENSE*` basename and an unpinned one fails the lint job.
- No tracked binaries; no string `nuget push` in any new file (`PackagingTests.NothingInTheRepositoryPushesToNuGet` scans every tracked file).
- Nothing under `folio-dotnet/src/Folio8/` — `SurfaceTests` bans modern constructs there and `DocsTests` requires a docs twin for every public name in that assembly.
- Does not modify the binding, the natives, the package, or any guard. Story 2 declares its own `sigaltstack` P/Invoke inside the package floor rather than sharing code with this tool.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Ordinary run | Linux, glibc ≥ 2.34 | One line per thread kind with `ss_size` in bytes and KiB, plus both `sysconf` values | N/A |
| Pre-2.34 glibc | `sysconf(_SC_SIGSTKSZ)` returns −1 | Reported as unavailable with the reason, not as a size of −1 | N/A |
| `pthread_create` absent from `libc` | glibc < 2.34, where it lived in `libpthread` | The raw-pthread row is skipped with a named reason; every other row still reported | Caught, not fatal |
| No altstack installed | `ss_flags` carries `SS_DISABLE`, or `ss_sp` is null | Reported as `NONE INSTALLED` rather than a misleading size | N/A |
| `sigaltstack` call fails | Non-zero return | Row reports the errno; the run continues and exits non-zero at the end | Accumulated, not bailed on |
| Non-Linux host | macOS or Windows | Refuses with one line saying the measurement is Linux-only | Exit non-zero |

</frozen-after-approval>


## Code Map

- `_bmad-output/specs/spec-dotnet-linux/evidence/Program.cs` -- the working probe. Correct as measured; needs the edge-case handling in the matrix above, which it does not have (it has no non-Linux guard, no `SS_DISABLE` reporting, and `pthread_create` throws rather than degrading).
- `folio-dotnet/build/verify-linux-natives.sh` -- the style exemplar: header shape, `set -euo pipefail` placement (line 25), script-relative `here=` discovery (line 27), `failed=0` accumulation rather than bailing (lines 99-102), stderr messages naming the remedy.
- `folio-dotnet/build/build-native.sh` -- second exemplar; validates its whole argument list before doing any work (lines 95-107) with a comment saying why.
- `.gitignore` lines 173-181 -- the comment says `folio-dotnet/build/` "holds the two BUILD SCRIPTS"; sources in a new subdirectory are already un-ignored by the `!/folio-dotnet/build/` negation (verified with `git check-ignore`) and `bin/`/`obj/` are caught at any depth, so **no functional change is needed** — only the comment is now inaccurate.
- `_bmad-output/implementation-artifacts/deferred-work.md` -- DW-396, still OPEN, with two existing dated subsections whose shape the new entry matches.
- `folio-dotnet/Folio8.slnx` -- registration is **optional**; `test/consumers/*` are already deliberately outside it. Do not add this project.

## Tasks & Acceptance

**Execution:**
- [x] `folio-dotnet/build/signal-stack-probe/signal-stack-probe.csproj` -- new console project, modern TFM, no `LICENSE`, no package references -- the diagnostic needs nothing from the package.
- [x] `folio-dotnet/build/signal-stack-probe/Program.cs` -- port `evidence/Program.cs` and harden it against every row of the I/O matrix; carry the house-style rationale header.
- [x] `folio-dotnet/build/probe-signal-stack.sh` -- wrapper giving the one documented invocation, including running a chosen architecture in a container so the owner's amd64 leg is a single command; house style throughout.
- [x] `.gitignore` -- correct the lines 173-181 comment so it describes what `folio-dotnet/build/` now holds. Comment accuracy only; the rules are already right.
- [x] `folio-dotnet/build/signal-stack-probe/Program.cs` -- add a `--self-check` mode that drives the row-formatting logic with synthetic `stack_t` values and a synthetic failure, printing each case with its expected rendering and exiting non-zero on any mismatch -- the `SS_DISABLE` and `sigaltstack`-fails rows of the matrix are defensive branches no real host reaches on demand, and an unexercised branch is an unverified one.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- append a dated subsection under DW-396 recording what the probe measures, every leg actually run, and the provenance of each.

**Acceptance Criteria:**
- Given `--self-check`, when it is run on any host, then every synthetic case prints its expected rendering, the `SS_DISABLE` case renders `NONE INSTALLED`, the failure case reports its errno, and the exit status is zero.
- Given a Linux host, when the wrapper is run with no arguments, then every thread kind is reported with its `ss_size` and both `sysconf` values, and the exit status is zero.
- Given an unknown argument, when the wrapper is run, then it names the valid values on stderr and exits non-zero **before** starting any container or build.
- Given a macOS or Windows host, when the probe binary is run directly, then it refuses in one line rather than reporting a misleading measurement.
- Given the repository after this story, when `go test ./...` runs in `lint/`, then the licence census passes — no new `LICENSE*` file was added.
- Given the repository after this story, when `git status` is run following a build of the new project, then no build output appears as untracked.

## Implementation Notes

**`uname` is not a provenance signal on its own, and finding that out changed the
probe.** The first cut derived provenance from `uname`'s machine against
`ProcessArchitecture`, plus `/run/rosetta` and `binfmt_misc`. Run against a Docker
Desktop `linux/amd64` container on Apple Silicon it reported **NATIVE** — which is
precisely the mistake this story exists to make impossible. Measured inside that
container: `uname` says `x86_64`, `/run/rosetta` is absent, and
`/proc/sys/fs/binfmt_misc` is empty. The signal that survives translation is
`/proc/cpuinfo`'s `vendor_id`: `VirtualApple` there, against `GenuineIntel` /
`AuthenticAMD` on real amd64. The probe now reads all four signals, prints each one
raw, and derives TRANSLATED / SUSPECT / NATIVE / UNKNOWN from them — `SUSPECT` for
evidence that is suggestive but not conclusive (an unrecognised x86 `vendor_id`, or
a `binfmt_misc` interpreter registered for the probe's *own* architecture, which a
cross-build host legitimately has for the *other* one).

**`--self-check` covers every matrix row whose branch a real host cannot be made to
take.** Three renderings were split out of their callers to make that possible:
`FormatRow` (the `SS_DISABLE`, null-`ss_sp` and `sigaltstack`-failed rows),
`FormatSysconf` / `FormatSysconfUnavailable` (both *unavailable* answers) and
`FormatSkip` (the `pthread_create`-not-in-libc degradation); review added a fourth,
`DeriveProvenance`, the verdict derivation itself. **Nineteen cases in total**, each printing its
expected rendering beside the actual one, and any mismatch exits non-zero — verified
by tampering with one expectation in a scratch copy: the case printed `FAIL` and the
process exited 1.

**The raw-pthread skip is a rendering, not a reason left implicit.** `FormatSkip`
carries the provenance tag, the label and a **named** reason, so "this host cannot
take that measurement" never reads as "this host said nothing". Its pre-2.34 reason
string is a named constant shared by the live row and its self-check case, so the two
cannot drift. Provoking the real branch would need a glibc 2.31 host with a .NET 8
SDK, which no published image pairs — hence the synthetic case.

**The SDK image is a tag, not a digest**, unlike `build-native.sh`'s AlmaLinux pins.
There the image's glibc becomes a permanent property of a shipped artifact; here
nothing ships, and the probe prints the runtime version it actually ran on, so the
provenance travels with the measurement. `PROBE_SDK_IMAGE` overrides it.

**The container leg copies the source in rather than building in place** — the
project directory is mounted read-only at `/src` and copied to `/work`. A container
build writing `obj/` and `bin/` into the repository would collide with a host build
of the same project for a different architecture, and the resulting failure reads as
a NuGet asset problem rather than as what happened.

**`.gitignore` took the comment fix only.** `git check-ignore` confirms
`folio-dotnet/build/signal-stack-probe/obj/` matches `obj/` (line 170) and `bin/`
matches the Go section's `bin/` (line 40), while the sources are re-included by the
existing `!/folio-dotnet/build/` negation. After a full Release build,
`git status --untracked-files=all` lists only `Program.cs`, the `.csproj` and the
wrapper.

## Spec Change Log

## Review Triage Log

Three layers (blind-hunter, edge-case-hunter, verification-gap), all launched before any
result was read. Every finding below carries my verdict and the evidence for it —
severities the reviewers assigned were disregarded, as the workflow requires.

| # | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|
| 1 | `DetermineProvenance` is the only logic `--self-check` does not cover — all ten cases fed a literal provenance string to a formatter (all three layers) | **high** | patch | Verified by mutation: replacing the `VirtualApple` test with `false` left self-check green at 10/10, exit 0. After the fix the same mutation gives `1 of 18 ... did not render as expected`, exit 1. This is the function whose first cut called a Rosetta container NATIVE. |
| 2 | An unmapped-but-present `uname` machine falls through to NATIVE, claiming agreement on a comparison never made (edge-case) | **high** | patch | Read at the verdict cascade: `unsure` was set only for a *null* machine or unreadable cpuinfo, so `s390x` skipped the mismatch check and reached the `else` branch. Contradicts `MachineToArchitecture`'s own comment, "an unknown pairing must not read as agreement". |
| 3 | `KnownX86Vendors` stores `"  Shanghai  "` / `"VIA VIA VIA "` with CPUID padding a trimmed reading can never match (blind-hunter, verification-gap) | **medium** | patch | Confirmed by reading both sites: `CpuinfoField` returned `.Trim()`ed values; the other six entries are 12 chars unpadded. Genuine Zhaoxin/VIA silicon would have been stamped SUSPECT. |
| 4 | Unreadable `/proc/cpuinfo` is conflated with a missing `vendor_id`, yielding TRANSLATED with the false reason "the CPU underneath is not x86 at all" (all three layers) | **medium** | patch | `CpuinfoField` swallowed `IOException`/`UnauthorizedAccessException` and returned null, the same value as a missing key. That reason text is transcribed into the DW-396 ledger, so a masked procfs would have put a false statement in the evidence record. |
| 5 | `StackT` hard-codes the LP64 4-byte pad while `MachineToArchitecture`/`QemuNamesForSelf` contemplate i386/armv7 (blind-hunter, edge-case) | **medium** | patch | On ILP32 `stack_t` is 12 bytes with no padding, so `ss_size` would be read from the wrong offset and the set path would write past it. Unlikely on this repo's targets, but the failure is silent wrong numbers — the one outcome a measurement tool must not have. |
| 6 | `--help --bogus` exits 0: `--help` acted on before the argument list is validated (edge-case) | **medium** | patch | Reproduced directly: exit 0 before, exit 2 after. Contradicts the validate-every-argument-before-any-work rule the rest of the tool follows and the spec states. |
| 7 | The `sysconf` `EntryPointNotFoundException` path formats its own string inline instead of via `FormatSysconf` (blind-hunter) | **low** | patch | Same formatter/live-path drift the shared `PthreadNotInLibc` constant was introduced to prevent, one method away. Fix was a direct correction, so the low-severity rejection rule does not apply. |
| 8 | An exception in `ForeignThread` escapes `UnmanagedCallersOnly` and aborts the process; a non-zero `pthread_join` leaves the row silently absent (edge-case) | **low** | patch | Both real on inspection. The silent-absence half is precisely the case `FormatSkip` exists to prevent, so leaving it was internally inconsistent. |
| 9 | `--property:WarningLevel=0` suppresses warnings on a project nothing else compiles (blind-hunter, verification-gap) | **medium** | patch | Confirmed in the wrapper. Not wiring the project into CI is a deliberate frozen-block decision; suppressing warnings is not required by it and is a separate choice. Clean rebuild now reports 0 warnings. |
| 10 | Wrapper: opaque failure when docker is installed but the daemon is unreachable (edge-case) | **low** | patch | Every other failure in this script names its remedy; this one emitted a raw docker error. Direct correction. |
| 11 | Wrapper: `cp /src/*.csproj /src/*.cs /work/` is a flat glob, so a file added in a subdirectory is silently dropped and the container legs build a different program than the host leg (blind-hunter, verification-gap) | **low** | patch | Real; the comment documented the assumption without enforcing it. Now aborts naming the dropped file — verified by planting `sub/Extra.cs`. |
| 12 | Wrapper has no `--help`/`-h` although the binary it fronts does and the house style calls for a usage block (blind-hunter) | **low** | patch | Confirmed; direct addition. |
| 13 | DW-396's PENDING row conflates `_SC_SIGSTKSZ` with a minimum (blind-hunter) | **medium** | patch | Correct and substantive: `_SC_SIGSTKSZ` is glibc's *recommended* size, `_SC_MINSIGSTKSZ` its floor — 8192 vs 1348 on the amd64 leg. Which one 16384 falls below is the actual question the pending run answers. The same imprecision in `SPEC.md` was mine and I corrected it as spec author. |
| 14 | "It reports and asserts nothing" sits beside a run that exits non-zero on a failed reading (edge-case) | **low** | patch | The two statements did disagree. Narrowed to "asserts nothing about what the numbers should be". |
| 15 | `uname` buffer could overflow if `_UTSNAME_LENGTH` exceeded 65 (edge-case) | **false** | rejected | Linux fixes `_UTSNAME_LENGTH` at 65 in both glibc and musl; the buffer is 8x65 = 520 against six fields of 65 = 390. The probe refuses every non-Linux host before `uname` is reached, so the trigger condition cannot occur. |
| 16 | `Marshal.AllocHGlobal(1 MiB)` OOM sits outside the surrounding try (edge-case) | **low** | rejected | Real but not met in everyday use — a 1 MiB allocation on a host running a .NET SDK — and the fix adds a branch and a failure path for it. Rejected under the low-severity rule (unlikely to be met, fix adds complexity rather than correcting). |
| 17 | On musl, sysconf ordinals 249/250 are not these names, so another limit could print as `_SC_SIGSTKSZ` (edge-case) | **low** | rejected | Alpine/musl is explicitly unsupported by this package (SPEC.md constraint), the values are printed raw beside their names, and the fix adds libc-detection branching. Rejected on the same rule. Noted here so the reasoning is on the record rather than absent. |
| 18 | The probe transcribes rather than derives the libc version the DW-396 entry depends on (blind-hunter) | **low** | defer | Genuine gap in the derive-don't-transcribe principle the rest of the provenance work follows, and met on every run rather than rarely. Fix adds a `gnu_get_libc_version` P/Invoke with its own degradation path — more than a direct correction, so it is filed rather than patched. |
| 19 | Nothing in the repository compiles or runs this project (blind-hunter, verification-gap) | **medium** | defer | Verified: absent from `Folio8.slnx`, from every `ci.yml` dotnet step, and from `lint`'s Go-only walk. But the frozen block states "wired into no CI job or suite in this story" — the intent itself excludes it, which is the only ground on which scope may reject a finding. Filed with its cheap close (one `--self-check` step, OS-independent by construction). |
| 20 | Three wrapper paths unrun: the no-argument Linux host path, the `uname -s` gate, the docker-missing branch (blind-hunter) | **low** | defer | True — the container legs invoke `dotnet run` inside the image, so the wrapper's own host path is never exercised. Filed rather than patched: closing it means restructuring a leg to call the wrapper recursively, which is not a direct correction. |
| 21 | The superseded probe copy remains at `evidence/Program.cs` (blind-hunter, edge-case) | **low** | resolved by spec author | Real drift hazard: two probes answering the same question. Not this story's to fix — `evidence/` is `bmad-spec`-owned. Resolved outside the build loop with `evidence/README.md`, which marks it the preserved spike record, forbids running or developing it, and points at the supported tool. |

**Not counted as findings:** the reviewers' confirmations that the `.gitignore` hunk is
comment-only (independently re-verified with `git check-ignore`), that the six thread
kinds and shared `PthreadNotInLibc` constant check out, and that no negation rule was
lost. Recorded because a review that only lists problems hides what it actually checked.


## Design Notes

The probe reads rather than asserts, and that is the point: a diagnostic that
failed a build would turn "the CLR's altstack is 16 KiB here" into policy, when
what the reader needs is the number and its provenance. Story 2 owns the
assertion. Provenance is per-row because the spike's central error was reading
one clean tally as evidence — an emulated row and a real-silicon row must not
print identically.

## Verification

**Commands:**
- `cd folio-dotnet/build && ./probe-signal-stack.sh` -- expected: every thread-kind row printed, exit 0
- `cd folio-dotnet/build && ./probe-signal-stack.sh nonsense` -- expected: valid values on stderr, non-zero exit, nothing built
- `cd lint && go test ./...` -- expected: pass, licence census included
- `git status --short` -- expected: only the intended source files, no `bin/` or `obj/`

**Run 2026-09-22 (macOS arm64 host, Docker Desktop, .NET SDK 10.0.400):**

| Command | Result |
|---|---|
| `./probe-signal-stack.sh self-check` | **19 of 19** cases rendered as expected, **exit 0** |
| `./probe-signal-stack.sh arm64` | all six thread kinds **24576**, `sysconf` 20480, provenance **NATIVE**, **exit 0** |
| `./probe-signal-stack.sh amd64` | all six thread kinds **16384**, `sysconf` 8192, provenance **TRANSLATED — Rosetta**, **exit 0** |
| `./probe-signal-stack.sh nonsense` | names the valid targets on stderr, **exit 1**, no container started and nothing built |
| `./probe-signal-stack.sh` (macOS) | one line: the measurement is Linux-only, **exit 1** |
| `dotnet run` directly on macOS | one line refusal naming the OS, exit 2 |
| `cd lint && go test -count=1 ./...` | pass, licence census included |
| `git status --short --untracked-files=all` | only `Program.cs`, `signal-stack-probe.csproj`, `probe-signal-stack.sh` |

A tampered scratch copy of the probe (one self-check expectation altered) printed
`FAIL` for that case and exited **1**, confirming the mode fails rather than reports.

`./probe-signal-stack.sh` with no argument on a **Linux** host is unrun here — this
machine is macOS, and the no-argument path is the same `dotnet run` the two container
legs execute.
