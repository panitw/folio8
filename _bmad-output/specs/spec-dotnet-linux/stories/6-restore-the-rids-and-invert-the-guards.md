---
title: 'Restore the two Linux RIDs and invert the withdrawal guards'
type: 'feature'
created: '2026-09-23'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '0e852b9'
context:
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/withdrawal-surface.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** CAP-7, CAP-4, CAP-6. `linux-x64` and `linux-arm64` were built,
verified and withdrawn from the package before 1.1.0, and a set of guards was
put in place to keep them out — a `PackagingTests` assertion that no `linux` RID
is packed, and README text telling an installer why there is no Linux build.
Story 2 removed the reason those guards exist. Until they are inverted, the
package cannot carry Linux however well the binding works.

**Approach:** Put the two `FolioNative` items back, and turn every guard that
enforced their absence into one that enforces their presence — so a
half-restored package still cannot ship. Add the one guard the withdrawal never
needed: a pack-time gate, because CAP-5's soak has **not** run.

## Boundaries & Constraints

**Always:**
- **Invert guards, never delete them.** A guard that asserted absence becomes one that asserts presence. Deleting it removes what would catch a half-restored package.
- **⚠ THE SOAK HAS NOT RUN, AND THIS STORY DOES NOT PRETEND OTHERWISE.** CAP-5's two hardware legs are still PENDING. Restoring the RIDs makes the repository ready; it must not make an unsoaked package easy to produce. Packing with a Linux RID requires a deliberate, explicit opt-in that names the evidence it is asserting, and refuses without it.
- **The comment beside the withdrawn items becomes the record of why they were once out.** It is the most valuable prose in the file: it is what stops this being re-litigated from memory.
- **Keep the existing provenance machinery exactly as it is** — `build-native.sh`'s linux targets, `verify-linux-natives.sh`, the ELF arm of `FolioPackageCheck` with its `ET_DYN` and machine-table checks, and the pinned AlmaLinux 8 image. They survived the withdrawal deliberately and are not this story's to touch.
- **The glibc 2.28 ceiling is a provenance check, not a portability warning.** A higher floor means the native did not come out of the pinned image, whatever the build log said.

**Never:**
- No `linux-musl` RID. Go's `-buildmode=c-shared` emits initial-exec TLS relocations musl's loader refuses under `dlopen`, and building *with* musl does not fix it. An Alpine consumer must keep meeting a clean install-time absence, never a first-render crash.
- Does not publish, tag, or change the package version. Nothing here is a release.
- Does not touch `docs/`, `folio-dotnet/README.md` or `RELEASING.md`'s wider prose — story 7 owns the record, except where a guard this story inverts asserts on that text.
- Does not modify `src/Folio8`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Pack without the opt-in | Linux RIDs present, no evidence asserted | **Refuses**, naming CAP-5, the soak record and what to run | Fail the pack |
| Pack with the opt-in | A human asserts the soak passed | Packs all four RIDs in the `runtimes/<rid>/native/` layout | N/A |
| A Linux native missing | `build/native/linux-arm64/` empty | `FolioPackageCheck` refuses naming the RID and the build command | Fail the pack |
| A Linux native of the wrong machine | arm64 build in the x64 folder | Refused on the ELF `e_machine` table, before any package exists | Fail the pack |
| A Linux native from the wrong image | glibc floor above 2.28 | `verify-linux-natives.sh` fails — provenance, not portability | Fail the job |
| A RID removed again later | Someone drops a `FolioNative` item | `PackagingTests` reddens naming which RID is missing | Test failure |
| Alpine consumer | musl host | No native resolves; install-time absence, never a runtime crash | Documented absence |

</frozen-after-approval>

## Code Map

- `folio-dotnet/src/Folio8/Folio8.csproj` -- the two commented-out `FolioNative` items with the withdrawal comment above them, and `FolioPackageCheck` whose ELF arm and `linux-x64` = `0x003E` / `linux-arm64` = `0x00B7` machine table are already in place. The pack gate belongs here, beside the check that already refuses packs.
- `folio-dotnet/test/Folio8.Tests/PackagingTests.cs:62-85` `BothNativesArePackedIntoTheRidLayout` -- `Assert.DoesNotContain(PackedRids(), rid => rid.StartsWith("linux"))` is the guard to invert. It reads the `<Rid>` elements rather than the file text, deliberately, because the comment beside the items names the Linux RIDs in order to rule them out.
- `folio-dotnet/test/Folio8.Tests/PackagingTests.cs:~113` -- every packed RID must have an arm in `FolioPackageCheck`; the two lists are held equal rather than checked by eye. Restoring RIDs must keep that true.
- `folio-dotnet/test/Folio8.Tests/PackagingTests.cs:223` `TheReadmeTellsAnInstallerWhatTheyCannotGuess` -- pins `"Windows only"` and `"Why there is no Linux build yet"` in `folio-dotnet/README.md`. Inverting it requires the README text to change with it; that is the one place this story touches the record.
- `RELEASING.md:498` -- the manual `dotnet pack` command. `dotnet nuget push` is never run by a script, so publishing is already a deliberate act; the gate makes packing one too.
- `_bmad-output/specs/spec-dotnet-linux/withdrawal-surface.md` -- the inventory. Work from it rather than grepping.

## Tasks & Acceptance

**Execution:**
- [x] `folio-dotnet/src/Folio8/Folio8.csproj` -- restore the `linux-x64` and `linux-arm64` `FolioNative` items; rewrite the withdrawal comment as the record of why they were once out and what changed.
- [x] `folio-dotnet/src/Folio8/Folio8.csproj` -- add the pack gate: packing a Linux RID without an explicit soak assertion fails, naming CAP-5, `soak.sh` and the soak record. Passing the assertion is a deliberate human act at release time.
- [x] `folio-dotnet/test/Folio8.Tests/PackagingTests.cs` -- invert the RID ban to assert both Linux RIDs are packed; keep the packed-RIDs/check-arms equality; assert the pack gate exists and that it refuses by default.
- [x] `folio-dotnet/test/Folio8.Tests/PackagingTests.cs` + `folio-dotnet/README.md` -- replace the `"Windows only"` / `"Why there is no Linux build yet"` pins with pins on what is true: the four RIDs, the glibc floor, the musl exclusion.
- [x] `_bmad-output/specs/spec-dotnet-linux/withdrawal-surface.md` -- mark the rows this story closes.

**Acceptance Criteria:**
- Given a `dotnet pack` with no soak assertion, when it runs, then it fails naming CAP-5 and what to run, and produces no package.
- Given a `dotnet pack` with the assertion and all four natives present, when it runs, then the package contains `runtimes/{win-x64,win-x86,linux-x64,linux-arm64}/native/`.
- Given a `FolioNative` item removed, when the tests run, then `PackagingTests` reddens naming the RID.
- Given a packed RID with no arm in `FolioPackageCheck`, when the tests run, then they redden — an unverified native must not be shippable.
- Given the README after this story, when `PackagingTests` runs, then it pins what is true and no longer pins the Windows-only claim.
- Given the whole suite, when it runs on macOS and on Linux, then it passes.

## Implementation Notes

**What landed.**

- `folio-dotnet/src/Folio8/Folio8.csproj` — the two `FolioNative` items are live
  (`linux-x64` / `linux-arm64`, both `libfolio8_native.so`, the names
  `build-native.sh` and `verify-linux-natives.sh` already use). The withdrawal
  comment was rewritten rather than deleted: it now records the mechanism, the
  wrong first reading (the pinned image), what changed (every crossing on a
  binding-owned engine thread with an enlarged signal stack), and that the
  restoration is gated on the soak.
- `folio-dotnet/src/Folio8/Folio8.csproj` — new `FolioAssertLinuxSoak` target,
  `BeforeTargets="GenerateNuspec"` beside `FolioCheckPackageContents`. It arms
  itself off the packed RIDs (`%(Rid)` starting `linux`), not a hand-written
  list, so removing the Linux items disarms it and adding a third Linux RID
  arms it automatically. The opt-in is **the claim, typed out** —
  `-p:FolioLinuxSoakEvidence="CAP-5 soaked on real amd64 and real arm64"` —
  not a boolean, because a boolean is a flag and a sentence is an assertion.
  The refusal names CAP-5, both `soak.sh` legs, the PENDING record at
  `_bmad-output/implementation-artifacts/dotnet-linux-soak.md`, and that a
  translated host is refused by the runner itself.
- `PackagingTests` — `BothNativesArePackedIntoTheRidLayout` became
  `AllFourNativesArePackedIntoTheRidLayout` and holds `PackedRids()` **equal**
  to the four RIDs (not merely containing them), so both a dropped item and a
  stray added one redden. `NoMuslRidIsPacked` is the one prohibition that
  stays a prohibition. `EveryPackedRidHasAnArchitectureArmInThePackCheck` now
  covers the two Linux arms for free, which is why it was left untouched.
- `PackagingTests` — five gate tests (see the review triage below for how each
  got its shape): `ThePackGateNamesWhatItIsWaitingFor`,
  `NoTrackedFileSuppliesTheSoakAssertion`,
  `EveryTrackedPackOfThisProjectSatisfiesTheGate`,
  `ADotnetPackWithNoAssertionRefusesAndWritesNothing` (a real `dotnet pack`,
  ~500 ms) and `ThePackGateOpensOnlyForTheAssertionItDocuments` (four legs
  through the target: bare, miscased, Windows-only, correct — ~880 ms).
- `folio-dotnet/README.md` — the "Supported platforms" section is now a
  four-RID table plus the glibc 2.28 floor (stated with the base images it
  implies) and the musl exclusion **with its reason**. "How the right native
  is chosen" gained the Linux answer: the host resolves it from the RID graph;
  the bitness dance is .NET Framework's, and is labelled as such. The intro's
  "both Windows native libraries" became "all four native libraries".
  `TheReadmeNoLongerClaimsTheWithdrawal` pins the *absence* of "Windows only"
  and "Why there is no Linux build yet", because inverting the positive pins
  alone would not catch a README that says both things.

**Why the negative README test exists.** The task said "replace the pins". A
straight replacement leaves nothing stopping the old claim reappearing further
down the file — the four RIDs and "Windows only" can coexist in one document,
and that document is worse than either. The prohibition is cheap and it is the
half that actually enforces the inversion.

**Verified.**

- `dotnet pack … -c Release -o /tmp/nupkg` → exit 1, the refusal text, **no
  package written**.
- The same with `-p:FolioLinuxSoakEvidence="CAP-5 soaked on real amd64 and
  real arm64"` → the gate passes and both Linux natives clear
  `FolioPackageCheck`'s ELF/`ET_DYN`/`e_machine` arms; it then stops on the
  two **Windows** natives, which cannot be built on macOS (`build-native.ps1`
  needs mingw-w64 per architecture — RELEASING.md already requires a Windows
  machine for the release pack). To see the layout itself, the pack was re-run
  with `-p:FolioNativeDir=` pointed at a scratch tree holding the real Linux
  natives and two minimal PE stubs: the resulting `.nupkg` carries exactly
  `runtimes/{win-x64,win-x86,linux-x64,linux-arm64}/native/`. The scratch tree
  and the stub package were deleted; nothing of it is tracked.
- Mutation check: deleting the `linux-arm64` `FolioNative` item reddens
  `AllFourNativesArePackedIntoTheRidLayout` naming `linux-arm64`. Restored.
- `dotnet test folio-dotnet/test/Folio8.Tests -c Release` → 168 passed, 0
  failed (macOS arm64).
- `dotnet build folio-dotnet/test/Folio8.Net46Compile -c Release` → 0 warnings.

**Two sites the gate broke, and this story therefore fixed** — both are pack
invocations, which is the one thing a pack gate is guaranteed to touch:

- `folio-dotnet/test/consumers/run-consumers.ps1` packs on a `windows-2022`
  runner that has no `libfolio8_native.so`. It now passes
  `-p:FolioPackPlatforms=windows`, a new mode that drops the two Linux
  `FolioNative` items — disarming the gate the honest way (nothing Linux in
  the package) rather than by asserting evidence the harness has not got.
  Adding the evidence property there instead would have made the harness an
  offender under the sweep, which is the trap worth recording.
- `RELEASING.md`'s pack block now types the assertion and expects four
  `runtimes/` entries. The boundaries reserve RELEASING.md for story 7 *except
  where a guard this story inverts asserts on that text*, and the new gate
  asserts on exactly that command.

**Deliberately not touched — story 7's record (CAP-8).**

- `Folio8.csproj`'s `<Description>` still says "on Windows x86 and x64". It is
  a "claim that must stop being true" on the withdrawal surface, no guard here
  asserts on it, and the boundaries give the record to story 7.
- `RELEASING.md`'s wider prose (~197 and ~311: "folio-dotnet stays
  Windows-only", "NO LINUX RID SHIPS", the Docker prerequisites row),
  `docs/folio-dotnet.md` / `.html`, `NativeLibraryLoader`'s class remark, and
  DW-396's status.

**Risk.** Two tests spawn `dotnet` from inside the suite — one a real
`dotnet pack`, one four MSBuild target evaluations. They were kept because a
gate verified only by substring is a gate nobody has watched fire, and this one
guards an unsoaked package. Both drain stdout and stderr asynchronously, are
bounded by a timeout with a kill, and clear `FolioLinuxSoakEvidence` out of the
child environment so a developer who has it exported still sees the truth. The
pack writes to a temp directory and never touches `bin/`.

## Spec Change Log

## Review Triage Log

Three layers, launched together. Verdicts mine. This round was the most
consequential of the epic: two findings would have reddened CI, and my own
verification missed both because it ran macOS/net10.0 only.

| # | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|
| 1 | `ProcessStartInfo.ArgumentList` does not exist on .NET Framework, and the test project targets `net10.0;net48` on Windows (blind-hunter) | **high** | patch | Verified both: the TFM condition at `Folio8.Tests.csproj:14` and the `ArgumentList` calls. A **compile error** on the Windows leg, taking down the whole job rather than one test. Replaced with the string form the sibling helper already uses. |
| 2 | `run-consumers.ps1:89` packs with no assertion and is invoked unconditionally by the Windows job (verification-gap, pre-verified) | **high** | patch | Confirmed at the call site and at `ci.yml`, in the step whose own comment calls it the only evidence .NET Framework consumers work at all. The trap the reviewer spotted is real: adding the assertion to that script would make it an offender under the new sweep. Resolved with a Windows-only pack mode that conditions the Linux items off, so the gate disarms naturally rather than being bypassed. |
| 3 | The expected claim lived in an overridable property, so `-p:FolioLinuxSoakAssertion=` with no evidence made both sides empty and the gate opened (edge-case) | **high** | patch | Real, and it defeats the story's central mechanism. The property is gone; the condition compares against a literal with `String.CompareOrdinal`, closing the override hole and MSBuild's case-insensitive `!=` at once. |
| 4 | MSBuild reads environment variables as properties, so an exported evidence variable opens the gate where the tracked-file sweep cannot see it (edge-case) | **high** | patch | Correct. A second `Error` now refuses an assertion arriving from the environment. |
| 5 | The sweep allowlisted ten extensions while claiming no tracked file may set the assertion (all three layers) | **high** | patch | This repository has a tracked `Makefile` and `Dockerfile`, neither examined. Demonstrated: a `pack:` rule in the real Makefile satisfied the gate with the test green — the repository opening the gate on the packer's behalf. Filter inverted to every tracked text file minus a named, existence-asserted exemption set. |
| 6 | The gate test invoked the target by name, so the `BeforeTargets` wiring was pinned by substring only (verification-gap) | **medium** | patch | Real: an SDK change reworking the pack graph would leave both gate tests green while `dotnet pack` produced an unsoaked package. A real `dotnet pack` test now asserts non-zero, `CAP-5` in the output, and no `.nupkg`. |
| 7 | `RunMsBuild` could deadlock on the stderr buffer and waited without a timeout; it also inherited the parent environment (blind-hunter, edge-case, verification-gap) | **medium** | patch | All three real. Both streams now drained asynchronously, a timeout with a kill, and the variable cleared for the bare leg. |
| 8 | The csproj exemption was `EndsWith("Folio8.csproj")`, and the element check missed the attributed form (edge-case) | **medium** | patch | Both confirmed; exact-path match and a pattern now. |
| 9 | Nothing pinned that the paths the refusal names still exist (blind-hunter) | **medium** | patch | Only their substrings were asserted, so moving either would leave the gate instructing a packer to run something absent. Both now existence-checked. |
| 10 | `ci.yml`'s comment still said the RIDs remain unpacked and nothing here changes that (verification-gap) | **medium** | patch | Now false as written. Corrected: the RIDs are declared and the pack gate is what holds them back. |
| 11 | README: the glibc claim is false for `-alpine` images; macOS lost its alternative and `win-arm64` is unmentioned though it inherits no assets (blind-hunter, edge-case) | **medium** | patch | All three verified. Qualified, and macOS and `win-arm64` each got a line the way musl has one. |
| 12 | `withdrawal-surface.md` — the inventory successors are told to work from — omitted the two sites this story broke and named a test that no longer exists (blind-hunter) | **medium** | patch | Confirmed. Rows added for `RELEASING.md`'s pack block and `run-consumers.ps1`, plus the `ci.yml` comment; name corrected. |
| 13 | `NoMuslRidIsPacked` is vacuous once the four-RID set is held equal (blind-hunter, verification-gap) | **low** | rejected | Correct that it cannot redden independently. Kept anyway: it names the one prohibition that stays a prohibition, and a test that documents an invariant it shares with a stronger neighbour costs nothing. Recorded so its vacuity is on the record rather than mistaken for coverage. |
| 14 | `RELEASING.md` now contains the exact assertion string, so a packer copies rather than composes it (implied by finding 5's reasoning) | **low** | rejected | Accepted deliberately. That file is the documented exception for `nuget push` for the same reason: a release procedure has to state the command. The human act is typing the claim while reading the paragraph that says what it asserts, not recalling it from memory. |
| 15 | No Linux consumer ever resolves the packed RID layout — both Linux CI legs hand-stage the native, and the only pack-install-render proof is Windows-only (verification-gap) | **medium** | defer | Verified: `run-consumers.ps1` is `windows-2022`-only and its required-entries list names only the two `win-*` paths. So "the package works on Linux" is proved for the binding and not for the packaging. Closing it needs a job-sized Linux consumer leg, which sequences with CAP-5's soak rather than bolting onto this story. |

**Residual risk I could not close here:** the net48 compile leg is Windows-only
and unreachable from this session. I scanned the new test code for
.NET-Core-only APIs and found none (`System.Text.Json` is already
package-referenced for that TFM), and I attempted a local net48 build, which
fails on restore plumbing rather than on the code. The Windows job compiles it
first on the owner's side.


## Design Notes

The pack gate is an addition the withdrawal never needed, and it is here for one
reason: the evidence that is supposed to authorise this story does not exist
yet. Story 5's two hardware legs are PENDING. Restoring the RIDs without a gate
would leave `dotnet pack` one command from producing exactly the package 1.1.0
withdrew — and the reason it was withdrawn would be a comment rather than a
refusal. A gate that names CAP-5 at the moment someone packs is the difference
between a decision and an oversight.

## Verification

**Commands:**
- `dotnet pack folio-dotnet/src/Folio8/Folio8.csproj -c Release -o /tmp/nupkg` -- expected: **refuses**, naming CAP-5
- the same with the soak assertion -- expected: packs; then list the archive and confirm four `runtimes/<rid>/native/` entries
- `dotnet test folio-dotnet/test/Folio8.Tests -c Release` -- expected: all pass
- `dotnet build folio-dotnet/test/Folio8.Net46Compile -c Release` -- expected: 0 warnings
