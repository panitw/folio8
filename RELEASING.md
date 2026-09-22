# Releasing folio8

This document is the procedure for cutting a folio8 release. It exists because
**a release with no written procedure is not a release** — the same rule
D-000.58 applies to gate procedures, one level up.

It covers the Go engine module, `folio-go`, and the two client libraries
published as `folio8` — the npm package built in `folio-js/` and the NuGet
package built in `folio-dotnet/`.
The designer's own version and force-upgrade policy are at the end.

**Released version:** `folio-go/v1.1.0`

`TestVersionAgreesWithReleasingDoc` (`folio-go/version_test.go`) reads the line
above and fails unless `folio8.Version` equals it, so the stamp in the code and
the release this document names cannot drift apart.

## `folio-go/v1.1.0`

**A MINOR, not a major.** Everything added since `v1.0.0` is additive — no
`v1.0.0` identifier moved, was renamed or was removed — so the frozen surface
D-1.1.c fixes at the tag is intact and **no `/v2` import path is needed**. The
census (`publicSurfacePins`) grew and never shrank; that is the machine-checked
form of this claim.

**Inside the release:**

- **package `fontdir`** — a host-side font-directory loader (`fontdir.Set`,
  `fontdir.Skipped`). Its own package so that a consumer who wants it does not
  also take the `fonts` package's ~14.8 MB of embedded faces.
- **`FaceFallback`** (`FaceFallbackStrict`, `FaceFallbackSubstitute`) — an
  optional selector on `Render`, `RenderTo` and `Validate`. Absent is
  `Strict`, which is what every call written against `v1.0.0` asks for, so
  existing callers are unaffected.
- **`DiagCodeTextFaceAbsent`** and **`DiagCodeTextFaceSubstituted`**.
- **Format `4.2`** — the `authorAcknowledged` key on a `font` record a chain
  names. A MINOR: `SupportedMajor` is still 4, and `5.0` stays unopened for
  spec-loop-section. See `internal/template/version.go` for why the MAJOR this
  briefly shipped as was reversed.
- **A behaviour change in text layout** — whitespace *after* a mandatory line
  break is now drawn as the author's indent instead of being consumed with the
  break (GitHub issue #2, superseding D-7.1.6). Whitespace *before* the break
  is still consumed, and an optional whitespace break is unchanged.
  **This can move the rendered geometry of an existing document**: any value
  carrying whitespace after a `\n` now indents that line rather than starting
  it flush. No corpus fixture hash moved — `go-corpus.json` and
  `go-parity.json` changed only in their `folio8Version` string — so no golden
  in this repository exercised the old behaviour, but an integrator's own
  template may.

**Not in it:** the `loop` element and `$.` root paths (spec-loop-section,
unbuilt), which are what `5.0` is reserved for.

## `folio-go/v1.0.0`

**Cut after SPEC-client-libraries stories 1 and 3, before the folio-js and
folio-dotnet bindings are built** (owner decision, 2026-09-16, recorded in
`_bmad-output/specs/spec-client-libraries/SPEC.md`). This supersedes the
earlier "first tag after Epic 6" trigger (D-000.78): Epics 7–14, 16 and 17 and
Epic 15's completed stories all landed after that decision without amending it, and the bindings need a stable version to
build against.

The tag is a `v1` release, not a `v0` pre-release, because it commits to semver. **Cutting it is
irreversible**: D-1.1.c fixes the public API at it, and any later breaking
change needs a `/v2` import path that every caller edits.

**Inside the release:** everything on `main` at the release commit — Epics 1–14,
16 and 17, Epic 15's completed stories (15.1, 15.2a, 15.2b), and the client-library
prerequisites: the designer surface moved behind `internal/` (story 1), a signed
colour-and-strokes golden with both unused style codes retired (story 3), and
this procedure (story 2). **v1.0.0 freezes render and validate only.**

**Released from the tag gate (owner rulings):** 8.4d (size budget) and 8.4k
(licence exception) are designer-release and `lint` work, not Go-tag
prerequisites; DW-230 stays open against Story 15.2. **DW-68 is ruled: v1.0.0
ships the clip** — an aggregate-only over-tall keep-together group keeps
rendering clipped with `TABLE_ROW_CLIPPED_HEIGHT`, and making it fatal is now a
`/v2` choice. **Gated the tag, and met:** DW-147 (the `fixtures/colour-strokes/`
golden, owner sign-off in its `signoff.json`) and D-7.8.2 (`STYLE_COLOR_INVALID`
and `STYLE_LINE_SPACING_INVALID` retired).

## Release checklist

Each precondition below was re-verified on the release tree on 2026-09-17.

### 1. The third-party licence manifest ships with the release

**Obligation:** the committed manifest at **`lint/MANIFEST.md`** is attached to
the release as an artifact.

AD-26's substance already ships and is guarded continuously: every module in
the resolved graph carries a resolved licence, an unresolvable one fails the
build, and `TestManifestUpToDate` (`lint/internal/manifest/manifest_test.go`)
fails if the committed file drifts from what the generator produces. What this
line adds is **publication**: `gh release create` below attaches the file.

Regenerate with `cd lint && go run ./cmd/genmanifest` (from inside `lint/`,
which has its own `go.mod` — not from the repo root).

*Discharges DW-3, retired at Epic 4 planning.* **Re-verified:**
`TestManifestUpToDate` passes.

### 2. The public API surface is deliberate

**Obligation:** the exported surface of packages `folio8`, `fonts` and
`fontdir` is reviewed as a whole before it freezes, because D-1.1.c fixes it
at the tag.

**Measured for v1.0.0: 60 items** — package `folio8` has 7 funcs, 8 types,
32 consts (27 `DiagCode*`, `SeverityWarning`, `SeverityError`, `Version`,
`LocaleTableVersion`, `MaxParameterReferenceNameLength`), 3 methods and
9 struct fields; package `fonts` has `Shipped`. The surface was reviewed as a
whole at story 1's spec checkpoint, which cut it from 287 items.

**Measured for v1.1.0: 70 items** — package `folio8` grew to 7 funcs, 9 types
(`FaceFallback`), 36 consts (29 `DiagCode*` including `DiagCodeTextFaceAbsent`
and `DiagCodeTextFaceSubstituted`, plus `FaceFallbackStrict` and
`FaceFallbackSubstitute`), 3 methods and 9 struct fields; package `fonts` still
has `Shipped`; **package `fontdir` is new** with `Set`, `Skipped`,
`Skipped.String` and two fields. **Ten additions, zero removals** — which is
what makes this a MINOR and leaves the v1 import path intact.

**The live trigger is `TestPublicSurfaceMatchesTheFrozenV1Census`**
(`folio-go/public_surface_census_test.go`). It pins every exported identifier by
package, kind and name — not signatures, types or constant values — fails
naming an addition `UNEXPECTED` and a removal `GONE`, fails if a scan finds
nothing, and fails if any importable package other than `folio8` and `fonts`
appears under `folio-go/`. This line is what a release reads, not
what fires.

*Discharges DW-4's surface re-measure.*

### 3. The call-graph walker is precise, or its precondition still holds

**Obligation:** `buildFolio8CallGraph` (`folio-go/render_arch_test.go`) resolves
methods by name alone. Before a tag, either replace it with a `go/types`
version in `lint`, or confirm its precondition — that no two receiver types in
package `folio8` declare the same method name — still holds.

*Backstop for DW-20. The live trigger is the pinned injectivity assertion
beside the walker itself; it fires at the commit that creates the collision,
which is years earlier than anyone reads this file.* **Re-verified:**
`TestFolio8MethodNamesAreInjective` passes, and an independent `go/ast` scan of
package `folio8`'s non-test sources found 41 methods under 41 distinct names.

## Cutting a release

### Version stamping

`folio8.Version` (`folio-go/version.go`) is the release's version without the
tag prefix: tag `folio-go/v1.1.0` ↔ `Version = "1.1.0"`. Bump it, and the
**Released version** line at the top of this document, in the release commit
itself. Nothing else holds a copy: `TestVersionAgreesWithReleasingDoc` fails if
`Version` and that line disagree, and `TestReleasingDocNamesTheGuardedManifest`
(`lint`) reads the tag from the same line.

`Version` reaches no PDF byte — only the designer's preview identity and the
wasm `RenderResult` — so bumping it moves no golden hash. Each fixture's
`expected.json` keeps the `folio8GoVersion` that produced it; those records are
provenance and are **not** rewritten on a bump.

### Changelog

**GitHub release notes per tag.** There is no changelog file in the
repository. The notes for each tag state what changed since the previous
release, every breaking change an integrator will hit, and the matrix run
below.

For `v1.0.0` the notes carry the breaking changes since `main` that
integrators on `@main` pseudo-versions would hit:

- the designer surface (`Canvas`, `ApplyComponentCommand`, the projection
  types and the `folio-go/wasm` package) left the public API;
- `STYLE_COLOR_INVALID` and `STYLE_LINE_SPACING_INVALID` were retired into
  `TEMPLATE_FIELD_INVALID`;
- an invalid colour is now refused when the template loads;
- a render that needs a face the supplied `FontSet` does not carry is now
  refused, located, under the new `TEXT_FACE_ABSENT` code. A caller passing
  `fonts.Shipped()` wholesale cannot reach it; a `folio-js` or `folio-dotnet`
  caller supplying a **partial** font set previously got a
  `TEXT_MISSING_GLYPH` warning and a PDF with those characters silently
  missing, and now gets an error and no PDF. `TEXT_MISSING_GLYPH` keeps its
  narrower meaning: every declared face was supplied and none covers the
  character. There is no opt-in flag and no per-language leniency.

For `v1.1.0` there is **no breaking API change** — the notes carry the
additions and the one behaviour change:

- **new** package `fontdir` (`Set`, `Skipped`), for loading a font set from a
  directory on the host;
- **new** `FaceFallback` with `FaceFallbackStrict` / `FaceFallbackSubstitute`,
  an optional trailing argument to `Render`, `RenderTo` and `Validate` in all
  three libraries. **Omitting it is `Strict`**, which is exactly what a
  `v1.0.0` call does today, so no existing call changes meaning;
- **new** diagnostics `TEXT_FACE_ABSENT`'s companion
  `DiagCodeTextFaceSubstituted`, raised only under `Substitute`;
- **folio-dotnet shipped Windows-only, and that is a fact about `v1.1.0`
  rather than about this package.** Linux support was built for this release
  and withdrawn before it, over DW-396. Nothing about the Windows package
  changed. It is restored for `v1.2.0` — see the next list, and the
  package-contents section.
- **format `4.2`** — a document whose `fonts` chain names an asset carrying
  `authorAcknowledged: true` declares it. A `4.0`/`4.1` reader loads such a
  document and then refuses it on the licence terms the key exists to excuse;
  it never mis-renders one. `SupportedMajor` is unchanged at 4.
- ⚠ **a text-layout behaviour change an integrator can see** (GitHub issue
  #2): whitespace **after** a mandatory line break is now drawn as an indent
  rather than consumed with the break. `"a \n b"` was two lines `a` / `b` and
  is now `a` / ` b`. Whitespace **before** the break is still consumed, and
  optional (wrapped) whitespace breaks are unchanged. A template that relies
  on leading spaces after a `\n` renders differently — this is the fix for
  indents silently vanishing, so the new output is the intended one. No golden
  in this repository moved.

For `v1.2.0`, **not yet released and not yet packable** — the notes will carry
one addition and one behaviour change a .NET integrator can observe:

- **new** the `linux-x64` and `linux-arm64` runtime identifiers, restored
  after their withdrawal from `v1.1.0`. glibc 2.28 or newer; no
  `linux-musl-x64` ships and `linux-musl-x64` inherits nothing from
  `linux-x64`, so an Alpine consumer's restore succeeds and the first call into the engine then fails to load a native, rather than a render going wrong. No
  public API is added and a caller's code is identical on either platform.
- **a concurrency behaviour change, on every platform including Windows.**
  Every crossing of the C ABI — render, validate and the free of a result
  buffer — now runs on a pool of binding-owned threads sized to
  `Environment.ProcessorCount` (minimum two), rather than on the caller's
  thread. Calls from more threads than that now queue rather than all
  entering the engine at once. The API is unchanged and no caller writes
  threading code, but a consumer measuring concurrency will see the bound.
  The reason is DW-396: those threads are the only place the binding can
  enlarge the alternate signal stack the Go runtime adopts.
- **the throughput measurements** (CAP-10). Rendering
  `fixtures/multi-page-statement` from a fixed number of caller threads, before
  is `86e7e5a` (the direct-`DllImport` binding) and after is the engine-thread
  binding. **Leg A — `linux-arm64`, Debian 12 container, 10 CPU, .NET 8.0.31,
  the shipped `linux-arm64` native, executing natively on an Apple Silicon Mac;
  three alternating runs per leg, 5 s per level. One developer machine, so
  indicative rather than controlled** — the spread columns are what that is
  worth:

  | Concurrency | before r/s | spread | after r/s | spread | change |
  |---|---|---|---|---|---|
  | 1 | 40.80 | ±3.1 % | 43.14 | ±1.8 % | +5.7 % |
  | 2 | 74.17 | ±1.9 % | 76.15 | ±2.4 % | +2.7 % |
  | 4 | 116.26 | ±1.2 % | 121.04 | ±0.6 % | +4.1 % |
  | 10 (`ProcessorCount`) | 158.59 | ±1.1 % | 156.66 | ±3.8 % | −1.2 % |

  **Scaling, after: 3.63×** from one caller to ten — the pool buys
  parallelism rather than flattening at one core. Every change above is at or
  inside the legs' own run-to-run spread, so **no regression is measurable on
  Linux**.

  ⚠ **What these numbers do not cover, and the notes must say so.**
  **Windows x64 is UNMEASURED** — CAP-10 names it explicitly, and no Windows
  leg was taken; the wrapper is POSIX-only. **Real amd64 is unmeasured too**:
  the `linux-amd64` leg ran under Rosetta and no comparison completed there,
  both bindings dying identically. And a **low-concurrency latency tail** was
  measured on macOS — 8.2 % fewer renders a second at one caller, median
  unchanged, p95 22.33 → 41.78 ms, gone by four callers — which is filed in
  `deferred-work.md` rather than dismissed, because the handoff responsible is
  the same code on every platform. Full record, with every log behind every
  figure: `_bmad-output/implementation-artifacts/dotnet-linux-throughput.md`.
- ⚠ **DW-396 is still OPEN as this is written.** `v1.2.0` cannot be packed at
  all until CAP-5's two hardware soak legs are run and recorded — the pack
  refuses without the assertion. See the package-contents section below.

### The cross-target hash matrix

A release commit is tagged only after both `ci.yml` (Build, vet, and
guardrails — the full test suites and the designer e2e) and `matrix.yml`
(Cross-target byte identity) are **green on that exact commit**. The matrix
run is recorded in the release notes as its URL and the commit SHA it ran on.
It cannot be recorded in the repository, because the run only exists after the
commit does.

#### v1.1.0: an exception was prepared for DW-208, and proved unnecessary

Kept as a record of a judgement, not as a standing allowance. **No exception
was used to cut v1.1.0** — `ci.yml` and `matrix.yml` were both green on the
tagged commit `d176ed3`, so the rule above was met on its own terms.

**What happened.** `folio-designer-e2e`'s `browser-native-roundtrip`
(**[[DW-208]]**, open since 2026-09-05 at HIGH) fails intermittently — four
failures in the first five release candidates, passing on the sixth and
seventh. With the gate unsatisfiable by re-running honestly, a dated
single-release exception was drafted and approved. The next run then came back
green and it was never invoked.

**Why it was drafted rather than the job quarantined.** `folio-go-known-red`'s
pattern does not fit an intermittent failure: it works by *asserting* the red
and inverting the exit code, which is impossible for a test that passes half
the time. Quarantining these would have needed `continue-on-error` — forbidden
everywhere else in `ci.yml`, for the reason stated there — and DW-208 itself
says skipping the test "converts an unverified guarantee into an unstated one"
and does not discharge it.

**What the exception would have rested on, had it been needed.** Not the luck
of a green run: `browser-native-roundtrip` had already PASSED on `f8bdffe`,
and `git diff f8bdffe..d176ed3` touches only `ci.yml`, this file,
`deferred-work.md`, `folio-dotnet/README.md`, `Folio8.csproj` and
`PackagingTests.cs`. Every source under `folio-go/`, `folio-js/src/` and
`folio-designer/src/` was byte-identical between the commit where it passed
and the commit tagged — so the browser-and-native agreement on a
human-authored document was measured on these exact sources.

**For the next release:** this is not precedent to lean on. If DW-208 is still
red then, the judgement is made again, on that release's own evidence. What
would remove the question entirely is discharging DW-208 — naming and fixing
the blocking action in preview admission, or measuring the timeout as
environmental and saying which environments it does not reproduce in.

### The commands

Run from `folio-js/`, on `main`, with the release commit at `HEAD`.
**Publishing to npm is irreversible for anyone who installs it; do not run
this without the owner's explicit go-ahead.**

```sh
V=$(node -p "require('./package.json').version")   # the version being published; never typed twice
npm whoami                            # the publishing account, confirmed before anything is sent
npm pack                              # prepack rebuilds and re-checks; inspect the tarball if in doubt
npm publish --dry-run                 # last look at exactly what would be sent

# TAG FIRST, so a published version always maps back to a commit. The tag is
# directory-prefixed (AD-22), like the engine's, because the package lives in
# folio-js/ — the tag names the DIRECTORY, not the published package id, so it
# stays `folio-js/v…` even though the package publishes as `folio8`. Push it
# before publishing: an unpublished tag is cheap to live with, an
# unattributable npm version is not.
git tag -a "folio-js/v$V" -m "folio-js v$V" && git push origin "folio-js/v$V"

npm publish                           # the irreversible step (access comes from publishConfig)
npm view "folio8@$V" dist.tarball     # confirm the registry serves it
```

A published version is never unpublished or overwritten, and a pushed tag is
never moved or deleted; a bad release is fixed forward with a new patch
version, and `npm deprecate` marks the bad one.

## Publishing `folio8` to NuGet

The NuGet package `folio8`, built in `folio-dotnet/`, is a separate release
line from `folio-go` and from the npm package, published by hand. **`dotnet nuget push` is never run by a script,
an MSBuild target or a CI job**: no workflow in this repository holds a NuGet
API key, and none should. It is the owner's command, typed at the owner's
terminal, on the owner's explicit go-ahead. `PackagingTests` greps every
tracked script, workflow, project file and build script for the literal
`nuget push` — this document excepted, since it is where the command is
written down — and reddens if one acquires it.

### What the package promises

The packed `folio8.<version>.nupkg` is **self-contained**:

```
lib/netstandard2.0/Folio8.dll          the one managed assembly, faces embedded
runtimes/win-x64/native/folio8_native.dll
runtimes/win-x86/native/folio8_native.dll
runtimes/linux-x64/native/libfolio8_native.so
runtimes/linux-arm64/native/libfolio8_native.so
build/folio8.targets                   the .NET Framework delivery
buildTransitive/folio8.targets
README.md, LICENSE
third-party-notices/fonts/**           each face's OFL text and notice
```

**The two Linux entries are what `1.1.0` does not have.** `1.1.0` shipped the
two `win-*` runtimes and nothing else; the Linux pair was built, verified and
packed during its preparation and then taken out before release, because
entering the engine from a CLR thread-pool thread overflows that thread's
`sigaltstack` and kills the process (**DW-396**). The pinned build image does
not fix that — it was the first reading and it was wrong; the glibc a native
is built against moves how OFTEN the defect fires, not whether. The fix is on
the .NET side: every crossing now runs on a binding-owned thread whose
alternate signal stack was enlarged before its first crossing.

⚠ **AND THAT IS WHY A LINUX PACK REFUSES UNTIL A PERSON ASSERTS THE SOAK.**
`Folio8.csproj` declares both Linux `FolioNative` items, and its
`FolioAssertLinuxSoak` target then **refuses the pack** unless this exact
sentence is typed on the command line:

```
-p:FolioLinuxSoakEvidence="CAP-5 soaked on real amd64 and real arm64"
```

The comparison is **ordinal** — the wording and the casing must match
character for character, and a differently-cased or reworded sentence is
refused. It must also arrive as a command-line property and **not** be
exported into the environment, which the gate refuses separately, because the
point is that a person states it at the moment they pack. The assertion is a **claim about
CAP-5's two hardware legs**: a reproduction of the pre-fix crash and then a
100-iteration soak, on **real amd64** and on **real arm64** separately, with
no emulation and no Rosetta, both recorded in
`_bmad-output/implementation-artifacts/dotnet-linux-soak.md`. Both legs are
PENDING as this is written, so **`1.2.0` is not packable today**, and typing
the assertion anyway would repeat the false clear DW-396 already records being
made twice. A Windows-only pack — `-p:FolioPackPlatforms=windows`, which the
.NET Framework consumer harness uses — drops both Linux items and needs no
assertion, because it ships no Linux native.

`PackagingTests` holds all four RIDs in the packed layout, refuses a musl one,
and enumerates every tracked `dotnet pack` of this project to check each one
either asserts the soak or packs Windows-only. `ci.yml`'s `folio-dotnet-linux`
job builds both ELF natives in the pinned image, verifies their machine type
and glibc floor, and runs the corpus suite against each on its own
architecture — which proves byte identity on Linux and **does not** clear
DW-396.

It declares **no dependencies**, so `dotnet add package folio8` on a
machine with no Go and no C compiler produces a project that renders — from
.NET Framework 4.6 through modern .NET, in a 64-bit or a 32-bit process
(CAP-7). The native asset is `folio8_native.dll`, **never** `folio8.dll`:
NTFS is case-insensitive and the managed assembly is `Folio8.dll`, so the two
names are one file.

**The pack step is what keeps that true.** `Folio8.csproj` runs an inline
`FolioPackageCheck` task before the nuspec is generated, and it refuses the
pack — naming exactly what is wrong — if either native is missing, if a
native's PE header says it was built for the other architecture, if a face is
missing, if a face's byte length has drifted from the `shippedFaces` record
the Go engine wrote into `folio-js/test/data/go-parity.json`, or if the set is
not the eleven faces `fonts.Shipped()` returns. A publish therefore cannot
ship a half package.

### Version and engine stamp

The NuGet package's `Version` is its own; it is not tied to `folio-go`'s. What
ties them is `Folio8.csproj`'s **`FolioEngineVersion`**, written into the
assembly as the `folio8EngineVersion` metadata attribute — the .NET spelling
of the npm package's `package.json` field. `PackagingTests` fails if it disagrees
with what the loaded native library reports or with `go-parity.json`. Bump it
in the release commit when the package is rebuilt against a newer engine tag.

**What the natives are actually built from is the working tree.**
`build-native.{sh,ps1}` compile `folio-go/cshared/cmd/folio8` out of this
repository, not out of a fetched module version — so "built against
`folio-go/v1.1.0`" is a statement about the COMMIT the release is cut from,
and it holds only because the release commit is the tagged one. Cut the
package from the commit the engine tag points at, or from a descendant whose
engine sources are unchanged; nothing in the build enforces it for you.

⚠ **AND THE FOUR NATIVES CAN NOW DISAGREE WITH EACH OTHER.** The Windows pair
is built on Windows and the Linux pair in a container, at different times and
possibly from different working trees — and `FolioPackageCheck` verifies each
native's *architecture*, never its *provenance*, so a pack with stale natives
of one platform succeeds silently. This is not hypothetical: during the 1.1.0
preparation a `dotnet pack` produced a package whose Linux natives were at
`HEAD` and whose Windows natives were three days old, predating a rendering
change — an internally inconsistent package that no guard in this repository
refused.

**So: delete `folio-dotnet/build/native/` before a release pack, and rebuild
all four from the release commit.** `PackagingTests` catches the specific case
where the loaded native's reported engine version disagrees with
`FolioEngineVersion`, but only for the native it can load — the host's — so it
cannot speak for the other three.

### The build machine

Everything below runs anywhere, but **step 1 does not**: `build-native.ps1`
shells out to a Go toolchain and to a mingw-w64 gcc **per architecture**, and a
machine that has never cut a release has none of them. What the CI runner
carries preinstalled, an owner's laptop has to be given.

| Needed | Why |
| --- | --- |
| Go | `go build -buildmode=c-shared` builds the engine |
| mingw-w64 gcc, `x86_64` | cgo's C compiler for `win-x64` |
| mingw-w64 gcc, `i686` | a SEPARATE toolchain, for `win-x86` |
| Docker | the **Linux** natives, and the only supported way to build them — see below |

**The Linux pair ships from `1.2.0` onward, and the publish procedure below
builds it.** It needs Docker and nothing else — not a Go toolchain, not a
cross-compiler. `build-native.sh linux-x64 linux-arm64` builds them inside a
**digest-pinned AlmaLinux 8 image**, and the image is the point: a cgo library
records the glibc symbol versions of the machine that built it, so building on
a modern host silently raises the consumer's minimum glibc. The same sources
built on Debian bookworm demand `GLIBC_2.34` and will not load on RHEL 8 or
Ubuntu 20.04; built in the pinned image they demand no more than `GLIBC_2.17`.
Never build a shipped Linux native outside that image, and never substitute
the `host` target for it.

`linux-arm64` builds under emulation on an x86 machine (and `linux-x64` under
emulation on Apple Silicon); Docker Desktop supplies it, and CI uses
`docker/setup-qemu-action`. It is slow and correct.

The Go version does not matter beyond the module's floor. The script pins
`GOTOOLCHAIN = 'go1.26.0'`, so whichever Go is installed fetches and builds with
that exact toolchain — which is the point, and why a newer local Go is not the
AD-22 drift hazard it looks like.

```powershell
winget install --id GoLang.Go
winget install --id MSYS2.MSYS2
```

Take MSYS2's default `C:\msys64`: that is where `Resolve-Cc`'s candidate list
looks, and installing it elsewhere means editing the script.

```powershell
# Twice on purpose: the first pass can update pacman itself and end the
# transaction before the toolchains are reached.
C:\msys64\usr\bin\pacman.exe -Syuu --noconfirm
C:\msys64\usr\bin\pacman.exe -Syuu --noconfirm
C:\msys64\usr\bin\pacman.exe -S --noconfirm --needed mingw-w64-x86_64-gcc mingw-w64-i686-gcc
```

**If every mirror fails with `error adding trust anchors from file:
/usr/ssl/certs/ca-bundle.crt`, the network is not the problem.** That file is
MSYS2's trust store, and a `-Syuu` that updated `ca-certificates` and ended
mid-transaction can leave it ZERO BYTES — which fails every TLS handshake
identically, on every mirror, while plain HTTP downloads in the same run
succeed and make it look like a flaky remote. It is rebuilt offline, from
material already on disk:

```powershell
C:\msys64\usr\bin\bash.exe -lc update-ca-trust
```

Then open a NEW terminal. The Go installer edits PATH, and a shell started
before it reports `go` missing while every other shell builds fine.

**`win-x86` is not optional.** CAP-7 promises a 32-bit process, and
`FolioPackageCheck` refuses the pack when a native is missing or its PE header
names the wrong architecture — so a machine carrying only the 64-bit toolchain
cannot produce a publishable package, only a later and less legible failure.
MSYS2 has been phasing out its 32-bit environment since December 2023 and
treats MINGW32 as legacy; `mingw-w64-i686-gcc` is still published. If that ever
stops, the script's second `win-x86` candidate is `C:\mingw32\bin\gcc.exe`, and
a standalone i686 mingw-w64 build unpacked there resolves with no code change.

### Before publishing

0. **`rm -rf folio-dotnet/build/native/`.** All four natives are rebuilt from
   the release commit, and nothing in the pack checks that they came from the
   same one — see the warning above. Starting from empty is what makes the
   claim true.
1. The natives are built from the release commit:
   - **Windows**, on Windows, with the pinned toolchain:
     `folio-dotnet\build\build-native.ps1 win-x64 win-x86`. On a machine that
     has not built them before, satisfy *The build machine* above first — the
     script needs Go and a mingw-w64 gcc for each architecture.
   - **Linux**, anywhere Docker runs:
     `folio-dotnet/build/build-native.sh linux-x64 linux-arm64`, which builds
     both inside the digest-pinned AlmaLinux 8 image, then
     `folio-dotnet/build/verify-linux-natives.sh` to check each one's ELF
     machine and its glibc floor. Never build a shipped Linux native outside
     that image, and never substitute the `host` target for it.

   All four natives must be present, or the pack refuses and names what is
   missing — and for the two Linux ones the pack **also** refuses without the
   soak assertion (see *What the package promises*). To cut a Windows-only
   package deliberately, pass `-p:FolioPackPlatforms=windows` instead of
   omitting the natives.
2. `dotnet test folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj -c Release`
   is green, and so is the 32-bit leg. `ci.yml`'s `folio-dotnet` job runs both
   plus the consumer suite — the pack, the install into all three process
   shapes on both target families, the corpus hash, and each forced CAP-11
   failure — so nothing below re-checks the package's *behaviour* by hand.
   `ci.yml`'s `folio-dotnet-linux` job builds both ELF natives in the pinned
   image, asserts their machine type and glibc floor, and then runs the suite
   against each of them on its own architecture — so the corpus hashes are
   proved on the shipped Linux bytes too. That is byte identity, **not** a
   DW-396 clearance: the crash class is cleared by the soak, not by a green
   CI leg, and nothing this job produces is packed.
3. `ci.yml` is green on that exact commit.

### The commands

Run from the repository root, on `main`, with the release commit at `HEAD`.
**Publishing to NuGet is irreversible for anyone who installs it; do not run
this without the owner's explicit go-ahead.**

**POWERSHELL, NOT `sh`, AND THAT IS NOT A STYLE CHOICE.** Step 1 above can
only run on Windows — `build-native.ps1` needs a mingw-w64 gcc per
architecture — so every command here runs on a Windows machine. This block was
written in `sh` until v1.1.0, with `ls`, `basename`, `sed` and `unzip`, and
cost the owner time mid-release when `unzip` was not found. The shell now
matches the machine the procedure requires.

```powershell
# The version is READ from the packed file, never retyped: Folio8.csproj's
# <Version> is the one place it is declared, and the consumer suite takes it
# from the packed file name for the same reason.
# THE SOAK ASSERTION IS TYPED HERE, BY YOU, AND IT IS A CLAIM. Packing the two
# Linux RIDs refuses without it (Folio8.csproj, FolioAssertLinuxSoak): CAP-5's
# two hardware legs -- real amd64 and real arm64, no emulation, Rosetta
# included -- must both be recorded in
# _bmad-output/implementation-artifacts/dotnet-linux-soak.md before this line
# is true. It is not a flag and it must not be exported into the environment;
# the gate refuses an assertion that arrives that way, because the point is
# that a person types it at the moment they pack.
dotnet pack folio-dotnet\src\Folio8\Folio8.csproj -c Release -o .\artifacts\nupkg `
  -p:FolioLinuxSoakEvidence="CAP-5 soaked on real amd64 and real arm64"
$pkg = (Get-ChildItem .\artifacts\nupkg\folio8.*.nupkg).FullName
$v   = [IO.Path]::GetFileNameWithoutExtension($pkg) -replace '^folio8\.', ''

# LAST LOOK AT EXACTLY WHAT WOULD BE SENT. Expect FOUR runtimes/ entries — two
# win-* and two linux-* — and this listing is where a half-restored package
# would be caught before it ships. It read the other way through 1.1.0, when
# the Linux RIDs were withdrawn (DW-396); they are restored, and the soak
# assertion above is what now stands between them and a consumer.
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead($pkg)
try {
  $zip.Entries | ForEach-Object { "{0,12}  {1}" -f $_.Length, $_.FullName }
  $rids  = @($zip.Entries.FullName -like 'runtimes/*')
  $linux = @($zip.Entries.FullName -like 'runtimes/linux*')
  $musl  = @($zip.Entries.FullName -like 'runtimes/*musl*')
  if ($rids.Count -ne 4 -or $linux.Count -ne 2 -or $musl.Count -ne 0) {
    throw "expected four runtimes entries — win-x64, win-x86, linux-x64, linux-arm64 — and no musl one; got: $($rids -join ', ')"
  }
} finally { $zip.Dispose() }

# TAG FIRST, so a published version always maps back to a commit. The tag is
# directory-prefixed (AD-22), like the engine's and the npm package's, because
# the package lives in folio-dotnet/ — the tag names the DIRECTORY, not the
# published package id, so it stays `folio-dotnet/v…` even though the package
# publishes as `folio8`. Push it before publishing: an unpublished tag is
# cheap to live with, an unattributable NuGet version is not.
git tag -a "folio-dotnet/v$v" -m "folio-dotnet v$v"; git push origin "folio-dotnet/v$v"

# The irreversible step. The API key is the owner's: read it into the session
# for this one command and clear it after, and never put it into a file in
# this repository.
$env:NUGET_API_KEY = Read-Host "NuGet API key"
dotnet nuget push $pkg --source https://api.nuget.org/v3/index.json --api-key $env:NUGET_API_KEY
Remove-Item Env:\NUGET_API_KEY
```

**Then wait for it to be queryable before calling it done.** nuget.org
validates and indexes a new package after the push returns, so
`https://api.nuget.org/v3-flatcontainer/folio8/index.json` keeps listing only
the previous versions for a few minutes. A package still in flight shows as
**Validating** at <https://www.nuget.org/account/Packages>; a rejected one
shows **Failed validation** and sends mail. **`dotnet nuget push` returning
success is not the same as the package being available**, and v1.1.0's push
was reported done while the index still showed 1.0.1.

A published version is never unlisted-and-reused or overwritten, and a pushed
tag is never moved or deleted; a bad release is fixed forward with a new patch
version, and the bad one is **unlisted** on nuget.org.


## Choosing the designer version, and forcing an upgrade

`folio-designer/package.json`'s `version` is the number an open tab compares
itself against, and its **MAJOR is the entire force-upgrade policy**:

| Bump | What an open tab does |
| --- | --- |
| patch / minor (`1.0.0` → `1.2.3`) | Offers a dismissible "Update available". The running release stays usable; "Later" ends the asking for that tab. |
| **major** (`1.9.9` → `2.0.0`) | Blocks with "Update required". No dismissal, no Escape, no Later. |

Nothing else promotes a release to mandatory. The release `id` is a content
hash, so it changes on every deploy and answers "are these the same bytes",
which is the wrong question for "must this author stop what they are doing".
Forcing is therefore an **authored act with a diff**: bump the major, and no
build step can do it for you.

**A forced upgrade never discards a document.** Activation reloads the tab, so
a blocked author with unsaved changes is offered a save and no upgrade button
at all; the upgrade appears once the work is safe. Before publishing a major,
be satisfied that stopping every open tab is worth it — a wrong major cannot be
recalled from tabs that already took it.

**Tabs notice within about 15 minutes**, and immediately on refocusing the tab
or regaining network. A tab left open across a deploy no longer waits for the
browser's own ~24h service-worker check.

## Usage measurement in the deployed designer

The designer reports page loads and four fixed action names — open template,
export, font import, preview — through Google Tag Manager, container
`GTM-NQZRC9V4`. The reversal of the old no-telemetry posture, and the bound it
was allowed under, are recorded at `ARCHITECTURE-SPINE.md` **AD-27**; the
decisions are in `_bmad-output/implementation-artifacts/ga-decision-log.md`.

**It is a BUILD-time input, not a runtime one.** Vite substitutes
`VITE_GA_CONTAINER_ID` into the bundle, so the value has to be present when
`vite build` runs inside the image's build stage. A variable added to the
running service arrives after the bytes are already fixed and changes nothing.

**Where the operator sets it:** as an ordinary service variable on the
`folio-designer` service, `VITE_GA_CONTAINER_ID=GTM-NQZRC9V4`, in each
environment that should report (`production`, `staging`, or both — they are
separate). Railway has NO separate "build variable" type: service variables are
exposed to a Dockerfile build only for the names the Dockerfile itself declares
as `ARG`, which is why the `ARG`/`ENV` pair exists in the build stage and why
adding the variable alone is not enough — the two go together.

A REBUILD IS REQUIRED, not a redeploy. The value is substituted into the bundle
by Vite, so redeploying the existing image cannot pick up a new value; Railway
rebuilds on a variable change, but if a deploy is restarted or rolled back to an
older image it carries whatever that image was built with.

**Unset is a supported, silent state, and it is the default.** With no value —
a local `docker build`, `npm run dev`, Vitest, Playwright — no script is
loaded, `window.dataLayer` is never defined and no event is pushed. A
malformed id is treated as unset rather than injected. To confirm a build went
out configured, grep the deployed bundle:

```bash
grep -rc googletagmanager folio-designer/dist/assets/*.js
```

Zero on every file means the build was unconfigured.
