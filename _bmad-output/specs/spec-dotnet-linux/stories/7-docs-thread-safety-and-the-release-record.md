---
title: 'Documentation, the thread-safety contract, and the release record'
type: 'feature'
created: '2026-09-23'
status: 'done'
route: 'dispatch'
review_loop_iteration: 1
baseline_commit: '0ffb312'
context:
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/withdrawal-surface.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** CAP-8 and CAP-9. The package's own documentation still tells a
reader folio-dotnet is Windows-only, the engine's C ABI still documents a
thread-safety contract written before the binding stopped crossing on arbitrary
threads, and DW-396 is still an open entry describing a defect that now has a
fix. Every one of those is a claim the code has outgrown.

**Approach:** Retire the Windows-only claims, state what is actually true about
concurrency on both sides of the ABI, and bring DW-396 up to date — **without
closing it**, because CAP-5's soak has not run.

## Boundaries & Constraints

**Always:**
- **⚠ THE PUBLISHED SITE DEPLOYS FROM `main` ON EVERY GREEN PUSH.** `deploy.yml` runs on a successful CI workflow for `main`, and the designer precaches `docs/*.html`. A page that says "Linux is supported" goes live while the published package is Windows-only. **Version-scope every new platform claim** — name the release that carries it — so a reader on 1.1.0 and a reader on 1.2.0 are both told the truth. This repository has already had a docs page reach production describing behaviour the shipped library did not have.
- **`docs/*.md` is the source of truth and `docs/*.html` is the published page, and they must agree.** `DocsTests` asserts the twins are token-identical and that every public name appears in both.
- **DW-396 stays OPEN.** Record the mechanism, the fix, the measurements and every leg run, and state precisely what closes it: CAP-5's two hardware legs. An entry closed on a fix without its evidence is the 1.1.0 mistake with the sign flipped.
- **Re-examine the engine's thread-safety claim in the same change that changes what the binding does.** `folio-go/cshared/README.md` says calls are safe from several threads. Say what is true now, including for a caller who is not folio-dotnet.
- **State the binding's concurrency behaviour where a .NET caller reads it**: whether renders run in parallel, what bounds them, and that no caller-side threading code is required.

**Never:**
- Does not close DW-396, and does not describe the soak as done.
- Does not publish, tag, or bump the package version. Nothing here is a release.
- Does not weaken or remove a guard from stories 3 or 6; where a guard asserts on this text, the guard moves with it.
- Does not add public API. `DocsTests` requires a twin entry for every public name, and this story adds none.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Reader on 1.1.0 visits the live page | Docs deployed from `main` before 1.2.0 | Reads that Linux arrives in 1.2.0 and that 1.1.0 is Windows-only — no false promise | N/A |
| Reader on 1.2.0 | After release | Reads the four RIDs, the glibc floor, the musl exclusion, macOS and `win-arm64` absences | N/A |
| Alpine reader | musl host | Told it is unsupported, why, and that the failure is an install-time absence | N/A |
| Reader asking whether renders run in parallel | Concurrency section | Told renders run in parallel, what bounds them, and that no caller-side threading is needed | N/A |
| A Go caller of the C ABI | `cshared/README.md` | Told what is actually true about calling from several threads, including the signal-stack caveat that applies to any runtime with its own handlers | N/A |
| `DocsTests` after the edit | Twins compared | Pass — `.md` and `.html` agree token for token | Test failure |

</frozen-after-approval>

## Code Map

- `docs/folio-dotnet.md:33-37` "Supported platforms" and `docs/folio-dotnet.html:370` -- the Windows-only claim in both twins. `.md` is source, `.html` is what folio8.report serves.
- `folio-dotnet/test/Folio8.Tests/DocsTests.cs:251` `TheTwinsPublishTheSameTextPunctuationIncluded`, and `:296` which requires every public name in both twins' code spans -- the guards that move with this text.
- `folio-dotnet/src/Folio8/Folio8.csproj` `<Description>` -- "on Windows x86 and x64", the NuGet listing text. Not yet guarded by any test; story 6 left it here deliberately.
- `folio-go/cshared/README.md:185` -- "Calls are safe to make from several threads; the allocation table is mutex-guarded." CAP-9's subject.
- `folio-dotnet/src/Folio8/NativeLibraryLoader.cs` class remark -- "Off Windows this does nothing at all… the macOS and Linux host libraries are a development aid". Behaviour is right; the wording is now wrong.
- `RELEASING.md:197` and the package-contents section at ~311, plus the prerequisites table marking Docker as "only for the Linux natives, which 1.1.0 does not ship" -- the release record. Story 3 corrected its CI description and story 6 its pack block; this is the rest.
- `_bmad-output/implementation-artifacts/deferred-work.md` DW-396 -- add a dated subsection; leave status OPEN.
- `_bmad-output/specs/spec-dotnet-linux/withdrawal-surface.md` -- the inventory; mark the rows this story closes and leave visible anything it cannot.
- `.github/workflows/deploy.yml` -- why the version-scoping constraint exists. Read it before writing a platform claim.

## Tasks & Acceptance

**Execution:**
- [x] `docs/folio-dotnet.md` + `docs/folio-dotnet.html` -- replace the Windows-only section with the four RIDs **scoped to the release that carries them**, the glibc floor, the musl exclusion, and the macOS / `win-arm64` absences; add the concurrency statement CAP-9 requires. Both twins, token-identical.
- [x] `folio-go/cshared/README.md` -- re-examine the thread-safety claim and state what is true, including what a caller whose runtime installs its own signal handlers must do.
- [x] `folio-dotnet/src/Folio8/Folio8.csproj` `<Description>` and `folio-dotnet/src/Folio8/NativeLibraryLoader.cs` -- bring both in line with what is true.
- [x] `RELEASING.md` -- retire the remaining Windows-only prose and the withdrawal section; state that packing Linux requires the soak assertion and why.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- a dated DW-396 subsection: mechanism, fix, measurements, legs run and legs PENDING, and exactly what closes it. **Status stays OPEN.**
- [x] `_bmad-output/specs/spec-dotnet-linux/withdrawal-surface.md` -- close the rows this story closes; leave the rest visible.

**Acceptance Criteria:**
- Given the live documentation page before 1.2.0 ships, when a 1.1.0 user reads it, then no sentence tells them Linux works in the version they have.
- Given `dotnet test folio-dotnet/test/Folio8.Tests`, when it runs, then `DocsTests` passes with the twins in agreement.
- Given a grep for "Windows only" and "Why there is no Linux build yet" across tracked files, when it runs, then only historical records match — no live claim.
- Given DW-396 after this story, when a reader opens it, then it carries the mechanism and the fix, names its PENDING legs, states what closes it, and is still OPEN.
- Given `folio-go/cshared/README.md`, when a Go or C caller reads the threading section, then it states the signal-stack consideration rather than an unqualified safety claim.

## Implementation Notes

**Version-scoping landed as a table, not a sentence.** `docs/folio-dotnet.md`'s
"Supported platforms" now opens with "Linux support arrives in 1.2.0. Version
1.1.0, the release on NuGet today, carries the two Windows natives and nothing
else", followed by a two-row table (`1.1.0 and earlier` / `1.2.0 and newer`)
and the line "Everything below about Linux describes 1.2.0 and newer" — so the
glibc floor, the musl exclusion and the macOS / `win-arm64` paragraphs are all
scoped by that one sentence rather than each carrying its own hedge. It is true
before the release and after it, and needs no edit at tag time.

**The `.html` twin was hand-mirrored and checked by a local reimplementation of
`DocsTests.ReaderTokens`** before `dotnet test` was run — 5244 tokens, identical.
Nothing was authored outside `twin:begin`/`twin:end`, no new `h3` was added, so
the page's TOC (which lives outside the compared region) needed no change.

**`folio-go/cshared/README.md`: the claim was narrowed, not reaffirmed and not
deleted.** The bullet under *Ownership and lifetime* now says calls are safe
from several threads *as far as this library's own state goes*, and points at a
new *Threads, and the alternate signal stack* section. That section states what
is true for **any** caller: nothing serialises, nothing is pinned, `folio8_free`
need not run on the producing thread — and then the caveat, which is about
signals rather than data. A caller whose runtime installs its own alternate
signal stack must size it before the first crossing (Go reads it once at
`needm`) and keep the memory for the life of the thread. An ordinary C or Go
caller needs none of it, because Go installs its own 32 KiB when it finds
nothing to adopt; the trap is specifically a managed runtime with a small fixed
one, and the CLR's 16 KiB / 24 KiB is named as the measured instance.

**The csproj `<Description>` is not version-scoped but IS platform-scoped.** A
NuGet listing's description is frozen at publish beside the version it
describes, so it needs no version hedge — but `-p:FolioPackPlatforms=windows`
is a supported pack mode, and a Windows-only nupkg advertising `linux-x64` and
`linux-arm64` would freeze a false claim into the listing with nothing able to
redden for it. So there are two `<Description>` elements conditioned on the
same property the `FolioNative` items are; both resolutions were checked with
`dotnet msbuild -getProperty:Description`. The docs page is the one that had to
carry both versions at once, because it deploys from `main`.

**`RELEASING.md` keeps the v1.1.0 notes bullet as history** — it is a per-tag
record — and reworded so it reads that way; a new `v1.2.0` block states the two
restored RIDs, the concurrency behaviour change (every crossing now on a
`ProcessorCount`-sized pool, on Windows too), where CAP-10's numbers live, and
that the release is not packable until CAP-5's legs are run. The
package-contents section lists all four runtimes and then explains the soak
gate; the Docker prerequisites row and *Before publishing* step 1 now build and
verify the Linux pair instead of saying "Linux: nothing".

**DW-396 stays OPEN, and the new subsection says so four separate times.** It
carries the mechanism, the four load-bearing properties of the shipped fix, a
table of every claim measured and where it is written down (each marked for what
it does *not* establish), the two PENDING hardware legs, and a closing statement
that a mechanism plus a fix plus a green CI leg is the same evidence that
produced both of this entry's wrong readings.

**No guard was weakened.** `PackagingTests`'s soak-assertion sweep exempts
`RELEASING.md` already, and its pack enumerator matches an *invocation*
(`dotnet pack <path>Folio8.csproj`), not the prose mention added here. The
`InlineData("Windows only")` / `InlineData("Why there is no Linux build yet")`
absence pins on `folio-dotnet/README.md` are untouched.

## Spec Change Log

## Review Triage Log

Three review layers over the implementation; twelve findings dispatched, all
twelve fixed. Two of them were the same failure this whole epic exists to
prevent, reappearing in the epic's own closing record.

**The two that mattered most — a measurement published without its provenance.**
`folio-go/cshared/README.md` had promoted "the CLR installs 16 KiB on amd64 and
24 KiB on arm64 — **this is measured, not theoretical**" to the document that
external C and Go callers read, who would size their own alternate stacks from
it. This repository's own record says the amd64 figure was taken **under
Rosetta** and is *not admissible* as evidence about real amd64, and the arm64
figure came out of a Docker Desktop hypervisor and is pending a re-take on a
host. The same sentence that warned against reasoning from an emulated tally
was itself reasoning from one. Fixed by leading with **"Read your own size; do
not copy ours"**, stating each figure with the host it came from, saying what
the pair *does* establish (the shape — a managed runtime's fixed altstack below
the 32 KiB Go sizes for itself), and telling a caller to take its own
`sigaltstack(NULL, &old)` reading. Alongside it, DW-396's new subsection had
called the mechanism confirmed "on both architectures" three paragraphs above
its own table marking both rows PENDING; it now says what was read and on what
kind of host, and that neither figure is a real-hardware figure yet.

**Four claim-accuracy fixes that had already reached the published page.**
"meets a clean absence at install time" was not what happens — `dotnet add
package` and `dotnet restore` both succeed on Alpine, and the failure appears at
the first crossing; the phrase came from an internal note and had been promoted
verbatim. The `FolioNativeLoadException` promise survived the rewrite and now
sat directly beneath the new absence paragraphs, while `NativeLibraryLoader`
probes only on Windows — so the readers that section was rewritten for were the
ones it misled; it is scoped to Windows now, with the bare `DllNotFoundException`
named for everyone else. Two consumer-visible consequences of the pool were
documented nowhere a .NET reader looks — the latched `InvalidOperationException`
and the megabyte per logical processor — and the concurrency section gave only
the upper bound, omitting CAP-10's 8.2% drop at concurrency 1 on darwin/arm64,
which is the ordinary shape of the service the docs invoke two paragraphs later.

**One that would have frozen a false claim where nothing can redden for it.**
`<Description>` claimed Linux unconditionally while `-p:FolioPackPlatforms=windows`
is a supported path and currently the only passable one, so a Windows-only nupkg
would have advertised two RIDs it does not carry, permanently, in the NuGet
listing. Now two elements conditioned on the same property the `FolioNative`
items use; both resolutions verified with `dotnet msbuild -getProperty:Description`.

**The rest.** `RELEASING.md` never stated the literal `FolioAssertLinuxSoak`
compares against, so a release author could not pack without tripping the error
to learn it — it is quoted now, with the comparison's ordinality called out; and
its CAP-10 bullet carries leg A's table, its provenance line and the 3.63×
scaling figure instead of telling a future author to go and fetch them, which
also discharges the deferred entry that named this story as owner. The cshared
threading section was silent on Darwin, the one POSIX audience whose `stack_t`
field order differs and whose mistake compiles cleanly and returns `ENOMEM`.
`folio-dotnet/README.md`, which ships inside the package, stated the four
natives flatly while the page was version-scoped, and carried no concurrency
statement; it now matches the page on both. The version table's `1.1.0 and
earlier` row left a reader on a 1.1.x hotfix with no row describing what they
installed.

**Verified after the patch set, not taken on report:** 171 tests pass
(`DocsTests` twins, `PackagingTests`' README pins, the soak-assertion sweep and
the pack enumerator), `go test ./cshared/...` passes, a repository-wide grep for
"Windows only" and "Why there is no Linux build yet" matches only the absence
pins that assert against them and dated records, and DW-396 is still **OPEN**.

## Design Notes

Version-scoping is the whole trick in the documentation half. The page cannot
simply flip, because it is served from `main` and the package is not. Saying
"1.2.0 adds Linux; 1.1.0 is Windows-only" is true before the release and true
after it, and needs no second edit at tag time.

## Verification

**Commands:**
- `dotnet test folio-dotnet/test/Folio8.Tests -c Release` -- expected: all pass, `DocsTests` included
- `cd folio-go && go test ./...` -- expected: pass, the cshared docs guards included
- `grep -rn "Windows only" --include=*.md --include=*.html --include=*.cs --include=*.csproj .` -- expected: only historical records
