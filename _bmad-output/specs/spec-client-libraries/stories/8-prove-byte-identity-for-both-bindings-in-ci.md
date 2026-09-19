---
title: 'Prove byte-identity for both bindings in CI'
type: 'feature'
created: '2026-09-18'
status: 'done'
baseline_commit: 'eb473bb4768caab96793ffc211a4eaeb5faddf3c'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-client-libraries/packaging-matrix.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Each binding checks five hand-picked fixtures on one runtime. The corpus has 38 directories, folio-js ships to every Node platform and folio-dotnet to two architectures on two target families, so byte-identity is asserted far more widely than it is tested (CAP-5).

**Approach:** Derive one machine-readable corpus manifest from Go, run **every** renderable fixture through both bindings on every supported runtime and platform, and fail on any hash or diagnostic that differs from what Go produces.

## Boundaries & Constraints

**Always:**
- **The manifest is generated from Go, never hand-written.** A host-only Go test walks `fixtures/`, renders each fixture from its own files (`input.folio`, `data.json`, `params.json` when present) with `fonts.Shipped()`, and records for each: the slug, whether data and params files are used, and the diagnostic sequence Go produces.
- **Every fixture directory is classified — included or excluded with a stated reason.** The generator fails if a directory is neither, so a new fixture cannot be silently uncovered. Known exclusions today, by cause:
  - needs a face outside `fonts.Shipped()` (`font-text` wants `Roboto-Regular`);
  - data exists only as a Go literal (`alignment-rounding`, `line-spacing`, `mandatory-break`, `wrapped-text`);
  - no template or no golden (`minimal-rect`, `hidden-image`, `page-count-*`, `expected-breaks`, `thai-break-corpus`).
- **The manifest carries no golden digest.** Each binding reads the hash from that fixture's own `expected.json` at test time, so `expected.json` stays the single source and no new digest site is declared.
- **The generator verifies itself:** rendering with `fonts.Shipped()` must reproduce each included fixture's committed hash, or the fixture is not included and the failure is named. It refuses to write a manifest with fewer than the count it last recorded.
- **Both bindings run the whole included corpus**, comparing SHA-256 against `expected.json` and the diagnostic sequence — code, severity and message, in order — against the manifest. A fixture in the manifest that a binding skips is a failure, not an omission.
- **Platform and runtime axes:**
  - folio-js: Linux, macOS and Windows, each on the oldest supported Node LTS and on the pinned Node version.
  - folio-dotnet: modern .NET and .NET Framework 4.8, each 64-bit and 32-bit, plus the existing Linux host leg.

  Every leg runs the full corpus. The axes are expressed as a job matrix, not copied jobs.
- **A moved hash is a defect until proven otherwise** (AD-21/AD-22): no leg may regenerate, skip or tolerate a mismatch, and no test may quietly narrow the fixture list.
- **The existing hand-picked golden tests are replaced** by the corpus-driven ones, not left beside them.
- Go's own four-target matrix, the corpus itself and every golden stay untouched.

**Never:**
- Write or update an `expected.pdf`, `expected.json` or any digest.
- Add a fixture, or change what any fixture renders.
- Let a binding's corpus run depend on a Go toolchain at test time — the manifest is committed.
- Publish anything, or change the public surface of either binding.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Full corpus, each binding | every included fixture | each render's SHA-256 equals its `expected.json`, and diagnostics equal the manifest | any mismatch fails, naming fixture, expected and actual |
| Every leg | each OS, Node version, framework and bitness | the same hashes everywhere | a leg that renders nothing fails rather than passing empty |
| New fixture added | a directory with a golden but no manifest entry | the generator fails naming it | it cannot be ignored |
| Fixture excluded | a directory in the excluded list | the reason is recorded and the binding skips it | an excluded fixture with no reason fails |
| Manifest drift | Go's diagnostics change for a fixture | the generator's check fails until the manifest is regenerated deliberately | regeneration is an explicit, opt-in command |
| Shrinking corpus | the generator would write fewer fixtures than recorded | it refuses to write | the count is part of the manifest |

</frozen-after-approval>

## Code Map

- `fixtures/`: **37 directories** (the Intent's "38" was a miscount, corrected here rather than inside the frozen block), of which **25** render from files plus `fonts.Shipped()` — `page-count-20` has both a template and a golden and is included, so the Intent's `page-count-*` exclusion covers 1, 5 and 50 only; `statement-1/5/20/50` are the only ones with `params.json`; `image-embed` and `component-asset-import` render with a nil font set in Go and must be checked to yield the same bytes with the shipped set before inclusion.
- `folio-go/wasm/cmd/render/parity_test.go:33-97,222-236`: the generator precedent — case structs, the `FOLIO8_UPDATE_JS_PARITY=1` regeneration gate, and the drift failure. The corpus generator belongs beside it, writing its own file.
- `folio-js/test/data/go-parity.json`: shape to mirror (`comment`, `folio8Version`, `shippedFaces`, `cases`). The corpus manifest is a sibling; the .NET tests already read this directory.
- `folio-go/byte_neutrality_test.go:84-121`: `goldenDigestRecord` and its declared-site kinds. A file carrying a golden digest must be declared there — which is why the manifest carries none.
- `folio-go/testfont_embed_test.go:138`: `testShippedFontSet()` is byte-equal to `fonts.Shipped()`. `folio-go/render_test.go:511-513`: the test-only `Roboto-Regular` that keeps `font-text` out.
- `folio-js/test/golden.test.ts:12` and `helpers.ts:8-36`: the five-fixture list and the shipped-font loader to generalise.
- `folio-dotnet/test/Folio8.Tests/GoldenTests.cs:21-25` and `Repo.cs:88-101`: the same five, and the font-set builder.
- `.github/workflows/ci.yml:370` (`folio-js`, ubuntu, Node 24.16.0), `:516` (`folio-dotnet`, windows-2022, 64- and 32-bit legs), `:743` (`folio-dotnet-host`, ubuntu). Neither workflow uses `strategy: matrix` anywhere yet.
- `.github/workflows/matrix.yml:243-300`: the compare job's explicit slug list — the model for a corpus list that cannot silently shrink.
- Cost: the whole Go golden subset renders in about one second, so per-leg corpus time will be dominated by process and call overhead, not by any fixture.

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/wasm/cmd/render/corpus_test.go` -- the generator: classify every fixture directory, render each included one with `fonts.Shipped()`, verify its committed hash, record diagnostics, refuse to shrink, and fail on drift unless regeneration is requested -- one manifest, derived not authored
- [x] `folio-js/test/data/go-corpus.json` -- the generated manifest -- the shared conformance input
- [x] `folio-js/test/golden.test.ts` (+ helpers) -- drive the whole manifest, compare each hash against `expected.json` and each diagnostic sequence -- CAP-5 for folio-js
- [x] `folio-dotnet/test/Folio8.Tests/GoldenTests.cs` (+ `Repo.cs`) -- the same, driven by the same manifest -- CAP-5 for folio-dotnet
- [x] `.github/workflows/ci.yml` -- a job matrix for folio-js over OS × Node version; the corpus running in every existing folio-dotnet leg -- every supported runtime renders the corpus
- [x] `folio-dotnet/test/consumers/run-consumers.ps1` or the Windows job -- confirm the corpus legs cover 64- and 32-bit on both families without duplicating the consumer suite -- no gap between what ships and what is proved

**Acceptance Criteria:**
- Given the repository, when the generator runs, then every `fixtures/` directory is either included in the manifest or excluded with a reason, and the included count matches the manifest's own record.
- Given either binding's test suite, when it runs, then it renders every included fixture and compares against `expected.json` and the manifest, failing on the first difference.
- Given CI, when a commit lands, then the corpus has been rendered on Linux, macOS and Windows for folio-js, and on modern .NET and .NET Framework 4.8 at both bitnesses for folio-dotnet.
- Given a deliberately altered expected hash or diagnostic, when the suites run, then they fail naming the fixture.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|---|
| 1 | verification-gap, edge | The `dotnet-install.ps1` guard reads `$LASTEXITCODE`, which a PowerShell script never sets, so it would throw on a successful install | high | patch | A reviewer fetched and read the script: the 10.0.0 path invokes no native command. Guard dropped, `Stop` preference set, install asserted by path, fetch retried. |
| 2 | edge | `execFileSync` on `npm.cmd` throws EINVAL on modern Node, which would red all three Windows legs | high | patch | npm now runs as `node npm-cli.js`; the misleading comment about git is corrected. |
| 3 | blind | A moved hash was bucketed with unclassified directories, and the message offered exclusion as the remedy | high | patch | That advised deleting coverage in response to a byte-identity regression. Mismatches now have their own bucket citing AD-21/AD-22 and forbidding exclusion. |
| 4 | blind, edge | The shrink guard fatalled before the regeneration branch, so a legitimate removal could never be regenerated | medium | patch | Added `FOLIO8_CORPUS_MAY_SHRINK=1`, named in the message; red-proved 25 → 24 → restored. |
| 5 | edge | An exclusion was never re-tested, so it outlived its cause | medium | patch | Excluded fixtures are rendered too; one that now reproduces its golden fails as stale. |
| 6 | blind, verification-gap | Nothing asserted the manifest was current | medium | patch | `go-parity.json`'s version was checked but the corpus manifest's was not. Both bindings now assert it. |
| 7 | edge | The "renders nothing fails" guard compared the manifest to itself | medium | patch | Both suites now count renders actually performed and compare that to the recorded count. |
| 8 | edge | The removed "no diagnostics" assertions let a regenerated manifest bake in a warning | medium | patch | Every recorded sequence is asserted empty, so introducing one is deliberate. |
| 9 | blind, edge | Renaming the job orphaned the `folio-js` required check | medium | patch | A fan-in job keeps the name and fails unless the matrix succeeded. |
| 10 | blind | Every leg rebuilt its own wasm, so the matrix proved six local engines agree | medium | patch | Each leg records the engine's digest; the fan-in asserts six digests and one distinct value. |
| 11 | blind, edge | The lint step's `if:` duplicated matrix literals and would silently stop running | low | patch | Moved into `matrix.include`. |
| 12 | blind, edge | A single unretried network fetch gated a required job | low | patch | Three attempts with a timeout. |
| 13 | blind | `optional()` swallowed read errors and treated an empty file as present | low | patch | `os.IsNotExist` distinguished; dot- and underscore-directories skipped. |
| 14 | edge | A missing or unparseable manifest silently disabled the count guard during regeneration | low | patch | Now a failure unless the shrink gate is set. |
| 15 | edge | An unknown severity string deserialised as Warning | low | patch | Rejected explicitly. |
| 16 | verification-gap | `engines.node` was untied to the Node floor CI renders on | low | patch | A test pins the declared floor to the matrix's lowest version. |
| 17 | edge, blind | The recorded data/params flags were never checked against the tree | low | patch | Both suites assert them, naming the fixture. |
| 18 | blind | `Repo`'s corpus accessors reparsed the manifest, and the hash assertion hid the diff | low | patch | One lazy parse; `Assert.Equal`; `expected.json` read once per case. |
| 19 | blind, edge | The story document said 38 directories, five exclusions, and excluded all `page-count-*` | low | patch | Corrected outside the frozen block: 37 directories, 25 included, 12 excluded, `page-count-20` included. |
| 20 | blind | The negative acceptance criterion had no verification step | low | patch | Added as a recorded negative check; the implementer ran and reverted each mutation. |
| 21 | edge | The x86 staging step leaves an x86 native in two output trees for the rest of the job | low | reject | No later step in the job runs 64-bit; the consumer suite installs the package instead. Noted for whoever appends a step. |

## Implementation Notes

**The corpus is 25 fixtures, not 26, and page-count-20 is in it.** The intent
block's known-exclusion list globs `page-count-*` under "no template or no
golden", but page-count-20 carries both and reproduces its committed hash from
its own files with `fonts.Shipped()`. The stated CAUSE is what classifies, so
page-count-1/5/50 are excluded ("no expected.json") and page-count-20 is
included. Every other directory landed exactly where the intent predicted: 25
included, 12 excluded, 37 directories in all. (`fixtures/` also holds two loose
FILES — `statement-signoff.json` and a `.DS_Store` — which are records about
fixtures rather than fixtures; the generator classifies directories only.)

**Every included fixture renders clean.** All 25 produce an empty diagnostic
sequence today. The manifest still records the sequence per fixture and both
bindings still compare it, so the first fixture to start warning moves the
manifest rather than passing silently.

**Two regeneration gates, not one.** `FOLIO8_UPDATE_JS_CORPUS=1` regenerates
the manifest; a regeneration that would *shrink* the corpus additionally needs
`FOLIO8_CORPUS_MAY_SHRINK=1`. Without the second gate a legitimate removal
could never be recorded, and the only way past the refusal would have been to
widen `corpusExclusions` — which shrinks coverage further. The generator keeps
three failure buckets apart for the same reason: an unclassified directory
wants a decision, a moved hash wants an investigation (AD-21/AD-22, and its
message says never to exclude or re-record), and an exclusion whose cause has
gone away wants deleting. Excluded fixtures are re-rendered every run, so a
stale exclusion is named rather than trusted forever.

**Both 32-bit corners are now real legs.** The modern-.NET x86 corner had no
host at all: `actions/setup-dotnet` installs only a 64-bit runtime, which is
why `x86.runsettings` said "net48 only, in practice". ci.yml now fetches the
x86 runtime from Microsoft's own `dotnet-install.ps1` and runs the corpus
through `--framework net10.0 --settings x86.runsettings`, so the axes are
{net48, modern} x {64-bit, 32-bit} in fact rather than by inference.

**The consumer suite was left alone, deliberately.** It asks whether the
PACKAGE resolves and loads across seven process shapes; the corpus legs ask
whether the BYTES match across 25 fixtures and four runtimes. Running the
corpus inside the consumer projects would multiply minutes without widening
either claim, and the ci.yml comment records that division.

**One portability fix outside the corpus.** `folio-js/test/package.test.ts`
called `execFileSync('npm', ...)`, which is ENOENT on Windows because
`execFileSync` does not consult PATHEXT. The folio-js matrix now runs that
suite on Windows, so the executable name is resolved per platform. The
`.gitattributes` `* text=auto eol=lf` rule already protects the fixtures from a
CRLF checkout, so no byte-identity hazard came with the new platforms.

## Design Notes

**Why the manifest omits digests.** `TestGoldenDigestAgreesAtEveryDeclaredSite` treats a golden digest appearing in an undeclared file as a defect. Reading each hash from `expected.json` at test time keeps that rule intact and leaves one source of truth for every hash.

**Why exclusions are data, not silence.** The twelve directories the bindings cannot render today are not failures, but an unrecorded gap would be. Each carries its reason, and a directory that is neither included nor excluded fails the generator — the same shape as the matrix workflow's explicit slug list.

## Verification

**Commands:**
- `cd folio-go && go test -count=1 ./wasm/cmd/render/...` -- expected: green, and the manifest matches the tree
- `cd folio-js && npm run build && npm test` -- expected: the whole corpus renders and matches
- `cd folio-dotnet && ./build/build-native.sh host && dotnet test -c Release` -- expected: the same
- `cd folio-go && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: green, no golden moved
- CI -- expected: every folio-js matrix leg and every folio-dotnet leg renders the full corpus

**Negative check (the acceptance criterion that no green command covers):**
- Alter one fixture's `expected.json` hash, or inject a diagnostic into the manifest, and confirm each binding fails naming that fixture; then revert. Record the observed failure text in the Implementation Notes.

**Manual checks:**
- Windows and macOS legs for folio-js, and the .NET Framework legs, run only in CI; a green run is the evidence.
