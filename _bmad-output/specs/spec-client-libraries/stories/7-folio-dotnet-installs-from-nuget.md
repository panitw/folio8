---
title: 'folio-dotnet installs from NuGet and loads the right binary'
type: 'feature'
created: '2026-09-18'
status: 'in-review'
baseline_commit: 'ea5d53018455537417b394bbc168262f42ff396b'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-client-libraries/packaging-matrix.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `folio-dotnet` exists only as source. Nothing packs it, nothing ships fonts, and the native library is found only because the test project copies it. A consumer has no package, no faces, and on .NET Framework no mechanism to pick the right architecture.

**Approach:** Pack one NuGet package carrying the managed assembly, both Windows natives and the embedded font set; resolve the native by process bitness at load time (CAP-7); and fail with a named, actionable exception when it cannot load (CAP-11).

## Boundaries & Constraints

**Always:**
- **The native asset is `folio8_native.dll`.** `packaging-matrix.md` still says `folio8.dll`; that name collides with the managed `Folio8.dll` on case-insensitive Windows and was renamed in story 6. The package layout is otherwise as that document specifies:
  - `lib/netstandard2.0/Folio8.dll`
  - `runtimes/win-x86/native/folio8_native.dll`
  - `runtimes/win-x64/native/folio8_native.dll`
- **.NET Framework gets its own delivery.** `runtimes/` is a modern-.NET mechanism, so the package also ships MSBuild targets that place both architectures beside a .NET Framework consumer's output, in per-architecture subdirectories. The consumer authors nothing.
- **Resolution is by process bitness at load time**, before the first P/Invoke: the loader picks the `x86` or `x64` asset from `IntPtr.Size` and loads it by full path, so the later `DllImport` binds to the already-loaded module. No `DllImportResolver`, no `NativeLibrary`, nothing newer than .NET Framework 4.6. It must work unchanged from an AnyCPU project on 64-bit Windows, an AnyCPU project forced 32-bit, and an explicitly x86 project, on both target families.
- **Failure is named and actionable (CAP-11).** A dedicated folio8 exception states the detected process bitness, the RID and filename sought, every path probed, and the likely cause — bitness mismatch, missing RID asset, or P/Invoke blocked by the host. There is no degraded mode and no silent fallback to the other architecture.
- **Fonts ship inside the package**, the complete eleven-face `fonts.Shipped()` set, byte-identical to the engine's, copied from `folio8-go/fonts/` at build time and never committed under `folio-dotnet/`. `Fonts.Shipped()` returns them as a `FontSet`, caching after the first call, and stays an explicit argument at every call site.
- **Package metadata:** id `folio-dotnet`, version `1.0.0`, MIT, README and licence in the package, repository and project metadata, a field recording the engine version, and no `PackageReference` dependencies.
- **The licence census records the new licence file**, as `folio-js/LICENSE` is recorded, so the AD-26 gate stays satisfied.
- **Packing cannot ship an incomplete package:** the pack step fails if either native is missing, if a native has the wrong architecture, if a face is missing or has drifted, or if the font set is not the eleven faces.
- **Proof runs in CI on Windows:** the package is packed, installed into consumer projects covering AnyCPU 64-bit, AnyCPU forced 32-bit and explicit x86 across .NET Framework and modern .NET, and each renders a fixture whose SHA-256 matches its `expected.json`. Each CAP-11 failure mode is forced and the exception's text asserted.
- **`RELEASING.md`** gains a NuGet section mirroring the npm one: tag `folio-dotnet/v<version>`, publish only on the owner's go-ahead, never from CI.
- Byte identity, diagnostics and determinism are unchanged from story 6.

**Never:**
- Publish to NuGet, or run `dotnet nuget push` in any script or CI job.
- Commit font bytes or native binaries.
- Slim or re-tier the font set, or fetch a face at runtime.
- Probe the other architecture after a bitness mismatch, or continue without fonts.
- Add a runtime package dependency, or an API beyond `Fonts.Shipped()` and the load-failure exception.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| AnyCPU on 64-bit Windows | consumer installs the package, calls render | the x64 native loads; SHA-256 matches `expected.json` | N/A |
| AnyCPU forced 32-bit | same project, `Prefer32Bit` | the x86 native loads; same hash | N/A |
| Explicit x86 | `PlatformTarget=x86` | the x86 native loads; same hash | N/A |
| Both target families | .NET Framework and modern .NET consumers | both consume `lib/netstandard2.0`, same results | N/A |
| Missing RID asset | the architecture's native deleted after install | the folio8 load exception naming bitness, RID, filename and probed paths | no other architecture is tried |
| Wrong bitness | the x86 asset replaced by the x64 file | the same exception, naming a bitness mismatch as the likely cause | the underlying error is the inner exception |
| Blocked P/Invoke | loading refused by the host | the same exception, naming that cause | N/A |
| Shipped fonts | `Fonts.Shipped()` | 11 faces, names and byte lengths equal to the engine's | a missing resource throws a named error |
| Repeat call | `Fonts.Shipped()` twice | one read, and no caller can mutate another's set | N/A |
| Incomplete pack | a native or a face missing at pack time | packing fails naming what is missing | nothing is produced |

</frozen-after-approval>

## Code Map

- `folio-dotnet/src/Folio8/Native.cs:59`: `internal const string Library = "folio8_native"` and the seven `DllImport`s with `CallingConvention.Cdecl`; `EnsureAbi()` is the first call and already reports the loaded module's path. The bitness loader hooks in ahead of it.
- `folio-dotnet/src/Folio8/Folio8.csproj`: `netstandard2.0`, `LangVersion 7.3`, empty `RootNamespace`, no package references. Packing metadata and the font resources belong here.
- `folio-dotnet/build/build-native.{sh,ps1}`: produce `build/native/{host,win-x64,win-x86}/`. The pack step consumes those outputs.
- `folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj`: `FolioNativeRid` selects which native is staged, plus the MSBuild error when it is missing — the precedent for consumer-side wiring.
- `folio-dotnet/test/Folio8.Net46Compile/`: the 4.6 API-surface check; the new loader must keep compiling there.
- `folio-js/scripts/faces.mjs`, `build-fonts.mjs`, `src/fonts.ts`: the face table, the drift check against `folio-js/test/data/go-parity.json`'s `shippedFaces`, and the caching contract `Fonts.Shipped()` should mirror.
- `folio8-go/fonts/fonts.go:159-172`: the name → file table; eleven faces, 14,782,604 bytes total.
- `.github/workflows/ci.yml`: the `folio-dotnet` Windows job (compiler probes, export dump, 64- and 32-bit legs, trx uploads) and `folio-dotnet-host` on ubuntu. Consumer tests extend the Windows job.
- `RELEASING.md:172+`: the npm section, as the shape for the NuGet one.
- `lint/internal/licence/licencecensus_test.go:103`: where `folio-js/LICENSE` is pinned; `folio-dotnet/LICENSE` needs the same row or CI's `lint` job fails.
- Local tooling: `dotnet pack` works on macOS, and mingw-w64 now builds both Windows natives locally, so the package can be built and inspected here. Running a .NET Framework consumer remains CI-only.

## Tasks & Acceptance

**Execution:**
- [x] `folio-dotnet/src/Folio8/NativeLibraryLoader.cs` (+ the exception type) -- bitness selection, probe order, explicit load by full path, and the CAP-11 exception -- CAP-7 and CAP-11
- [x] `folio-dotnet/src/Folio8/Fonts.cs` + a build step copying the faces -- `Fonts.Shipped()` over embedded resources, verified against `shippedFaces` -- the package can render out of the box
- [x] `folio-dotnet/src/Folio8/Folio8.csproj` + `build/folio-dotnet.targets` -- pack metadata, the `runtimes/` layout, the .NET Framework delivery, and pack-time completeness checks -- one package, both families
- [x] `folio-dotnet/README.md`, `folio-dotnet/LICENSE`, `lint/internal/licence/licencecensus_test.go` -- first-PDF snippet, supported frameworks and platforms, and the census row -- an installer can start, and AD-26 stays satisfied
- [x] `folio-dotnet/test/Folio8.Tests/` -- loader, fonts and exception tests that run on any host -- the parts that need no Windows are proved everywhere
- [x] `folio-dotnet/test/consumers/` + `.github/workflows/ci.yml` -- pack, install into the three process shapes on both families, render and hash; force each failure mode -- CAP-7 and CAP-11 proved, not asserted
- [x] `RELEASING.md` -- the NuGet section -- publishing has a written procedure

**Acceptance Criteria:**
- Given a clean checkout on this host, when the natives are built and `dotnet test` and `dotnet pack` run, then all tests pass and the package contains both natives, the eleven faces and the managed assembly.
- Given the CI Windows job, when it runs, then every consumer project renders the fixture to its committed hash, and each forced failure throws the folio8 exception naming bitness, RID, filename and probed paths.
- Given the repository, when searched, then no `.ttf`, `.dll` or `.nupkg` is tracked by git.

## Spec Change Log

## Review Triage Log

**Round 1 — all findings accepted and fixed.**

- `run-consumers.ps1` rewritten: every leg is now a `dotnet publish` (the header said "publish" while the code ran `build`) with its **own `BaseIntermediateOutputPath` and `BaseOutputPath`** — with a shared obj/ MSBuild skipped CoreCompile and the 32-bit legs would have run the 64-bit executable, making the bitness proof vacuous. Seven legs now, three process shapes on each family: the modern consumer gained `Prefer32Bit` and `PlatformTarget=x86` legs (each paired with `-r win-x86 --self-contained`, since modern .NET takes its apphost bitness from the RID and no 32-bit runtime is installed on the runner). The dead per-leg `Rid` key now drives the forced-failure assertions, and the wrong-bitness mode is forced in **both directions** on both families, with the source DLL's absence failing that leg instead of aborting the run. Added: `buildTransitive/folio-dotnet.targets`, 11 `LICENSE-OFL.txt` and 11 `NOTICE.md` counted in the package, the packed `.nuspec` asserted to declare no dependency, the `ENGINE` line checked against `go-parity.json`, and the `ZipFile` handle disposed.
- `ci.yml`: the consumer step now tees to `folio-dotnet-consumers.log`, writes the package's entry listing, and uploads both with `if: always()` — it was the one job called the only evidence for .NET Framework and it left nothing behind when it red.
- `FolioPackageCheck`: the go-parity regexp is anchored on the `shippedFaces` array and matches `name` and `byteLength` independently, so a reformat can no longer turn the gate into "0 faces recorded".
- `README.md`: the first-PDF snippet is a class with an explicit `Main` at C# 7.3 — top-level statements cannot compile on the 4.6 floor the page documents — and the false "this is the snippet the consumer tests run" claim is now the true one.
- `Fonts.Shipped()` costs ~14.8 MB per call; the XML doc and the README now tell callers to hoist it to a field. `Program.cs` records why the consumer deliberately does not.
- `RELEASING.md`: the API key is no longer described as prompt-only while the command reads an env var; the `PackagingTests` claim matches what the test does; and the "built against the v1.0.0 tag" claim is corrected to what is true — the build scripts compile the working tree, so it is a statement about the release commit.
- `PackagingTests`: the colliding-name guard looks for `/folio8.dll` (the old `native/folio8.dll` could never have matched), the push scan covers .mjs/.js/.cmd/.bat/.py/Makefile and excepts RELEASING.md explicitly, and the version literal is replaced by a shape assertion.
- The package version is now declared once, in `Folio8.csproj`; the consumers take it from the packed `.nupkg` name via `-p:FolioDotnetVersion` and error if it is unset.
- `FolioNativeLoadException` gained the conventional `(string)` and `(string, Exception)` overloads; `.gitignore` gained `*.nupkg` and RELEASING.md packs into `artifacts/nupkg`.


## Implementation Notes

**The loader.** `NativeLibraryLoader.Ensure()` runs as the first statement
inside `Native.EnsureAbi()`'s lock, so it is ahead of every `DllImport` on
every entry point. It is a no-op off Windows (`Environment.OSVersion.Platform`
— `RuntimeInformation` arrived in 4.7.1 and is below the floor), and on
Windows it loads the chosen file with `LoadLibraryEx(...,
LOAD_WITH_ALTERED_SEARCH_PATH)`. Probe order per directory:
`runtimes/<rid>/native/`, then `folio8-native/<rid>/`, then flat beside the
assembly; directories are the AppDomain base, the assembly's own location and
`<base>/bin`. **The first candidate that exists is the one that is loaded** —
if it will not load, that is the failure, not a reason to keep hunting, which
is how "no silent fallback to the other architecture" is enforced by
construction rather than by a rule.

`Resolve()` takes its probe directories, pointer size, existence test and load
call as parameters. That seam is what makes all three CAP-11 modes reachable
from a unit test on any host — a bitness mismatch and a blocked P/Invoke
cannot be produced in the process running the suite — while the consumer
projects force the two that *can* be produced for real, against a real
installed package.

**Fonts.** The eleven faces are `EmbeddedResource`s read straight out of
`folio8-go/fonts/` at build time, with explicit `LogicalName`s (`RootNamespace`
is deliberately empty). Nothing is copied into `folio-dotnet/` at all, so
"never committed" holds by construction. `Fonts.Shipped()` reads the resources
once per process and returns a fresh `FontSet` each call; `FontSet.Add` copies,
so the isolation the story asks for is real rather than documented.

**Packing.** `FolioPackageCheck` is an inline `RoslynCodeTaskFactory` task in
`Folio8.csproj`, run `BeforeTargets="GenerateNuspec"`. It reads each native's
**PE machine field** rather than trusting its directory name, and each face's
byte length against `folio-js/test/data/go-parity.json`'s `shippedFaces`.
Verified live on this host: with `win-x86/folio8_native.dll` removed the pack
fails naming it and the script to run; with the x64 file copied over it, the
pack fails with `has PE machine 0x8664, but win-x86 needs 0x014C`.

**Engine version.** Recorded as the `folio8EngineVersion` assembly-metadata
attribute from `Folio8.csproj`'s `FolioEngineVersion` — the .NET spelling of
folio-js's `package.json` field. `PackagingTests` ties it to both the loaded
native's `Folio8.Version` and `go-parity.json`.

**The .NET Framework delivery.** `build/folio-dotnet.targets` is packed to both
`build/` and `buildTransitive/`, scoped to `'$(TargetFrameworkIdentifier)' ==
'.NETFramework'`, and links both RIDs' natives to `folio8-native/<rid>/` with
`CopyToOutputDirectory`. Modern .NET is left to `runtimes/` — **measured on
this host**: a RID-agnostic `dotnet build` of the modern consumer against the
packed package put both `runtimes/win-x64/native/folio8_native.dll` and
`runtimes/win-x86/native/folio8_native.dll` in the output, which is probe
candidate 1.

**Local evidence beyond the stated commands.** The modern consumer was built
against the packed package on this macOS host and run with the host native
staged: it printed `FACES 11` and the `colour-strokes` hash from
`expected.json`. That exercises the package's managed assembly, the embedded
faces and the README's snippet end to end from an *installed* package — the
only part it cannot exercise is the Windows native selection itself.

## Verification — what was run here

- `dotnet test folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj -c Release` — **112 passed, 0 failed** (was 97 before this story).
- `dotnet build folio-dotnet/test/Folio8.Net46Compile/Folio8.Net46Compile.csproj -c Release` — green; the loader, the exception and `Fonts` all fit inside the 4.6 surface.
- `dotnet pack folio-dotnet/src/Folio8/Folio8.csproj -c Release` — one 42 MB `.nupkg` containing `lib/netstandard2.0/Folio8.dll` (14.8 MB, faces embedded), both `runtimes/<rid>/native/folio8_native.dll`, `build/` and `buildTransitive/` targets, `README.md`, `LICENSE` and 22 font licence/notice files.
- Both pack-time refusals forced and observed (missing native, wrong architecture).
- `cd lint && go test -count=1 ./...` — green, census included.
- `cd folio8-go && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` — green, no golden moved.

## Outstanding

- **The CI Windows job has not run.** Per the story's own manual-checks note,
  that job is the only evidence for the .NET Framework consumers, the 32-bit
  process shapes and the forced CAP-11 modes against a real package. It must be
  green before this story is done.
- `run-consumers.ps1` could not be syntax-checked on this host (no `pwsh`).
- The third CAP-11 mode, a host that blocks P/Invoke, is proved only by the
  injected-seam unit test; no consumer project can produce it.

## Design Notes

**Why an explicit load rather than probing paths per call.** `DllImport` resolves by module name once; loading the chosen file by full path first means every later import binds to it, which is the only mechanism available on .NET Framework 4.6 and works identically on modern .NET. It also makes the failure a single, explainable point rather than seven.

**Why fonts are embedded in the managed assembly.** It is the one asset that must reach both target families without MSBuild help — `runtimes/` and content files behave differently on .NET Framework, while an embedded resource is just there. The weight is the decided "embed the full set" constraint.

## Verification

**Commands:**
- `cd folio-dotnet && ./build/build-native.sh host win-x64 win-x86 && dotnet test -c Release` -- expected: green
- `cd folio-dotnet && dotnet pack -c Release` -- expected: one `.nupkg` with both natives, eleven faces, README and licence
- `cd lint && go test -count=1 ./...` -- expected: green, including the licence census
- `cd folio8-go && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: green, no golden moved
- CI Windows job -- expected: every consumer shape renders to the committed hash; each forced failure names its cause

**Manual checks:**
- .NET Framework consumers cannot run on this machine. The CI job is the only evidence and must be green before the story is done.
