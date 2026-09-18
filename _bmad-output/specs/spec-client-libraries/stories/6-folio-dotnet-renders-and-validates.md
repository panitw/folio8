---
title: 'folio-dotnet renders and validates, .NET Framework 4.6 through modern .NET'
type: 'feature'
created: '2026-09-18'
status: 'done'
baseline_commit: 'b9bba9bfb8263396886bdd36886ecb14f8d4731b'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-client-libraries/api-surface.md'
  - '{project-root}/_bmad-output/specs/spec-client-libraries/packaging-matrix.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** .NET applications cannot reach the engine at all, and the .NET Framework 4.6 floor rules out every wasm runtime. Nothing in the repo builds the engine as a native library or speaks its C ABI.

**Approach:**
- Add a `c-shared` build of `folio8-go` behind a small, explicit C ABI.
- Add a `netstandard2.0` managed library, `folio-dotnet`, that binds it with plain `DllImport` and exposes render and validate with Go's diagnostics (CAP-3, CAP-4).
- NuGet packaging and bitness-aware loading are story 7; the full-corpus matrix is story 8.

## Boundaries & Constraints

**Always:**
- **C ABI**, C identifiers only, no structs across the boundary:
  - Inputs are pointer plus length byte buffers. Fonts arrive as one buffer holding a length-prefixed sequence of name/bytes pairs, in caller order.
  - Every call returns a status int plus out-parameters for a result buffer and its length.
  - The engine owns what it allocates and hands back a token the caller frees with one `folio8_free`; a double free or an unknown token is refused, not undefined.
  - Nothing is stored between calls; no globals beyond the allocation table.
  - Exports carry the `folio8_` prefix and never leak Go types, callbacks or the Go runtime.
- **Managed library** targets `netstandard2.0` and uses nothing newer: no `Span<T>`, `System.Text.Json`, `NativeLibrary`, `DllImportResolver` or nullable reference types.
- **Managed surface** (`api-surface.md`): `Template.Parse(byte[])`, `Template.Load(string)`, `Template.ParameterReferences()`, `Folio8.Render(...)` returning `RenderResult`, `Folio8.RenderTo(Stream, ...)`, `Folio8.Validate(byte[], ...)`; `Data` and `Params` wrapping bytes or a string, `FontSet` as `IDictionary<string, byte[]>`, `Diagnostic`, `Severity`, `RenderResult`, `FolioRenderException`.
  - Synchronous, matching Go.
  - `Data`/`Params` take bytes or a string only — no serialiser dependency.
  - `Template` is immutable and holds its canonical bytes; the native side keeps no handle.
  - Fonts are a required argument; a null or empty set is an `ArgumentException`.
- **Diagnostics and errors mirror Go exactly:** the same codes, severities and message text, in Go's order.
  - A warning lands in `RenderResult.Diagnostics`, which is never null.
  - An error throws `FolioRenderException` carrying its `Diagnostic`.
  - `Validate` returns Go's slice and throws where Go returns an error.
  - `RenderTo` renders fully, then writes once to the `Stream`, and never closes or disposes it.
- **Byte identity:** a render through folio-dotnet matches the committed `expected.json` hash for the same inputs. No rendering logic in C#.
- **Determinism:** no clock, environment, network or ambient input. Only `Template.Load` touches disk.
- **Platforms:**
  - CI builds `win-x86` and `win-x64` DLLs with `-buildmode=c-shared` and a mingw-w64 toolchain per architecture.
  - The build script also builds a host library (a macOS `.dylib`) so the ABI and the binding can be developed and tested off Windows. That artifact is a development aid, not something shipped.
- **Test projects:**
  - The library test project runs on the modern .NET target on the developer's host and on Windows in CI, plus `net48` on Windows, which consumes the same `netstandard2.0` assembly.
  - A compile-only project builds against **.NET Framework 4.6 reference assemblies**, so the floor is proved by the compiler rather than asserted.
  - Golden hashes and diagnostic parity are checked against the same fixtures and recorded Go output folio-js uses.
- **CI:** a new Windows job builds both DLLs, runs the managed tests on `net48` and modern .NET, and builds the 4.6 compile check. The existing host CI stays green without Windows.

**Never:**
- Change the public Go API, the golden corpus or folio-js.
- Ship or commit native binaries, or publish to NuGet.
- Do RID layout, bitness probing or the load-failure exception — story 7.
- Embed fonts in the managed assembly or the native library — story 7.
- Add a third-party runtime dependency to the managed library.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Golden render | a fixture template, its data, the shipped faces read from the Go tree | SHA-256 equals `expected.json` | N/A |
| Warning render | a fixture Go renders with a warning | bytes plus that warning, equal to Go's recorded output | N/A |
| Render error | binding path absent from data | `FolioRenderException` with Go's code, element id and message | nothing returned |
| Validate | clean, warning and failing inputs | Go's diagnostics, or a throw where Go returns an error | N/A |
| RenderTo | a `MemoryStream` | the same bytes as `Render`; the stream stays open | on failure nothing is written |
| Bad arguments | null template, null or empty fonts, a `Data` holding invalid UTF-8 | `ArgumentNullException` / `ArgumentException` before the native call | N/A |
| Free discipline | every call in a loop over the corpus | no growth in native allocations after the loop | a second free of one token is refused, not a crash |
| Empty result | a template producing no diagnostics | `Diagnostics` is empty, not null | N/A |

</frozen-after-approval>

## Code Map

- `folio8-go/wasm/cmd/render/main.go`: the closest precedent for a shell over the public API — the request shapes, the diagnostic JSON (`severity`, `code`, `elementId`, `dataPath`, `message` with severity lowercased), the error envelope, and the panic-recovery wrapper. Copy the shapes; the transport differs.
- `folio8-go/render_entry.go:160,246`, `validate.go:52`, `folio8.go:54,80`, `parameter_references.go:22`: the Go entry points. `diagnostic.go:75,391,461` and `render_error.go:47` for the value types.
- `folio8-go/public_surface_census_test.go:219-262`: any `package main` passes, so the c-shared entry belongs at `folio8-go/cshared/cmd/folio8/` (or similar) as `package main` with `//export`ed functions. A non-main helper would have to live under `internal/`.
- `folio8-go/wasm/cmd/render/imports_test.go` and `parity_test.go`: the precedents for an import guard and for recording Go's output as test data. `folio-js/test/data/go-parity.json` already holds the recorded cases and the `shippedFaces` table — reuse it rather than recording a second copy.
- `folio-js/scripts/faces.mjs` and `build-fonts.mjs`: how the shipped faces are located in `folio8-go/fonts/`. The .NET tests need the same table to build a `FontSet` from the Go tree.
- `folio-js/src/index.ts`: the argument checking, the error split and the `renderTo` discipline, as a behavioural reference for the same contract in C#.
- `.github/workflows/ci.yml`: job shapes, the Go toolchain pin (1.26.0) and the `folio-js` job added last story. No Windows runner is used anywhere yet, and no job sets `CGO_ENABLED=1`.
- `.github/workflows/matrix.yml`: how a per-target job records and compares hashes — the model story 8 will extend to this binding.
- Local tooling: Go 1.26.0 and .NET SDK 10.0.400 are present; mingw-w64 is not, and neither is Windows. Windows artifacts and the `net48`/4.6 checks are therefore CI-only.

## Tasks & Acceptance

**Execution:**
- [x] `folio8-go/cshared/cmd/folio8/main.go` + an import guard test -- the `//export`ed C ABI over the public API, with the allocation table, panic recovery and status codes -- the engine as a native library
- [x] `folio8-go/cshared/README.md` or header comment -- the ABI contract: each function, its parameters, the status codes, the ownership rule -- the binding and any future caller share one written contract
- [x] `folio-dotnet/src/Folio8/*.cs` + `Folio8.csproj` -- the `netstandard2.0` managed surface and its `DllImport` layer -- CAP-3 and CAP-4
- [x] `folio-dotnet/build/build-native.ps1` and `.sh` -- build `win-x86`, `win-x64` and the host library from one script -- reproducible native builds
- [x] `folio-dotnet/test/Folio8.Tests/` -- every I/O matrix row, golden hashes and parity against `folio-js/test/data/go-parity.json` -- proves equality with Go rather than asserting it
- [x] `folio-dotnet/test/Folio8.Net46Compile/` -- a compile-only project against .NET Framework 4.6 reference assemblies -- the floor is checked by the compiler
- [x] `.github/workflows/ci.yml` -- a Windows job: both DLLs, the tests on `net48` and modern .NET, and the 4.6 compile check -- Windows stays proved on every commit

**Acceptance Criteria:**
- Given a host with Go and the .NET SDK, when the native build script and `dotnet test` run, then every test passes against the host library.
- Given the CI Windows job, when it runs, then both DLLs build, the tests pass on `net48` and modern .NET, and the 4.6 compile check succeeds.
- Given `folio-dotnet/src`, when searched, then it contains no layout, shaping or PDF logic, no `Span<T>`, `System.Text.Json`, `NativeLibrary` or `DllImportResolver`, and no third-party package reference.
- Given the Go module, when `go build ./...` and the suites run on the host, then they are unaffected by the new cgo package, and every golden hash is unchanged.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|---|
| 1 | verification-gap, blind, edge | The win-x86 DLL is built and existence-checked but never loaded | high | patch | Pre-verified: dropping `CallingConvention.Cdecl` kept every test and the Windows job green. A 32-bit `net48` leg under `x86.runsettings` now runs the suite. |
| 2 | verification-gap, blind | `FOLIO8_ERROR_ARGUMENT` and `FOLIO8_ERROR_PANIC` are published contract nothing executes | high | patch | Pre-verified: removing the guards or the recover reddened nothing. `abi_test.go` now drives both, plus the allocation table. |
| 3 | verification-gap | `build-native.sh` and the whole host path run nowhere in CI | medium | patch | Renaming its output would break every developer with CI green. New `folio-dotnet-host` job on ubuntu. |
| 4 | blind | `DOTNET_VERSION: "10.0.x"` floats while its comment claims a pin | low | patch | Pinned to 10.0.400. |
| 5 | blind | `Data(string)`/`Params(string)` silently replaced invalid UTF-8 | medium | patch | The implicit conversion made the lossy path the default while the byte[] path threw. Both now use a strict encoder. |
| 6 | blind | `FontSet` stored caller face arrays by reference | medium | patch | A post-`Add` mutation changed rendered bytes in the library whose bar is byte identity. Faces are copied now. |
| 7 | blind, edge | The frame decoder trusted signed lengths and an unverified payload | medium | patch | A corrupt frame threw BCL exceptions or passed a short payload as complete. Every length is bounds-checked. |
| 8 | blind, edge | A throw from `finally`'s `folio8_free` replaced the original exception | medium | patch | `ExceptionDispatchInfo` preserves it. |
| 9 | edge | A status/kind mismatch produced a `FolioRenderException` with a null Diagnostic | medium | patch | Now a named malformed-frame error. |
| 10 | edge | A mismatched native library decodes garbage silently | medium | patch | Added `folio8_abi_version` plus a one-time check and a README rule. |
| 11 | edge | An unset or future `Severity` crossed the ABI as Warning | medium | patch | Now an explicit encoding failure. |
| 12 | blind, edge, verification-gap | A duplicate face name last-wins; the README promised an order the engine discards | medium | patch | Duplicates are refused; the ordering language is corrected. |
| 13 | edge | A result frame over 2 GiB overflowed the int32 length | low | patch | Refused with a named message. |
| 14 | blind, edge | Two stale names in `main.go`'s header comment | low | patch | Corrected. |
| 15 | blind | `RootNamespace` was the one namespace the design forbids | low | patch | Emptied with the reason. |
| 16 | blind, edge | `build-native.ps1` used PowerShell 6+ `Join-Path` and mis-detected Windows arm64 | low | patch | Nested calls; arm64 refused rather than built as amd64. |
| 17 | blind | The build scripts mishandled `all`, validated targets late, and kept stale artifacts | low | patch | A failed build could leave yesterday's engine under test. Fixed in both. |
| 18 | blind, edge | A missing native library gave a bare `DllNotFoundException` | low | patch | An MSBuild error now names the script and path. |
| 19 | edge | `ParityTests` wrapped a null data case into `ArgumentNullException` | low | patch | Passed through. |
| 20 | edge | `FontSet.CopyTo` threw the wrong exception after a partial copy | low | patch | ICollection contract exceptions first. |
| 21 | edge | Empty vs absent `Params` could differ across the ABI | low | patch | Go treats them identically (`decodeParams`); documented, and the redundant helper dropped. |
| 22 | blind | The job uploaded no test report, unlike its neighbours | low | patch | trx loggers with `if-no-files-found: error`. |
| 23 | blind | `Folio8.Version` is public but absent from the frozen member list | medium | intent_gap | Raised with the owner: folio-js ships `version`, so parity argues for keeping it. Code unchanged pending that ruling. |
| 24 | blind | `folio-dotnet` has no README or LICENSE | low | reject | Packaging is story 7, which adds both; `folio-js` got them at its packaging story too. |
| 25 | blind | `Folio8.slnx` is committed but never built in CI | low | reject | CI builds the three projects directly, which is the stronger check; the solution file is an IDE convenience. |
| 26 | blind | No Go round-trip test of the frame encoder | low | reject | `abi_test.go` now exercises the encoder end to end through the exports, which covers the same ground. |
| 27 | edge | `Frame.Read`'s hand-written truncation messages were partly unreachable | low | patch | Folded into finding 7's bounds checks. |
| 28 | CI (Windows) | cgo failed with no diagnostic: MSYS2's gcc was selected by full path with its own bin directory off PATH, so it died at DLL load | high | patch | Windows-only, found by the first CI run. The x64 build now prefers the image's standalone mingw, each candidate must compile a trivial file, and a failure dumps `go env` and an `-x` retry. |
| 29 | CI (Windows) | Every managed test failed with EntryPointNotFound: the native `folio8.dll` and the managed `Folio8.dll` are one file on case-insensitive NTFS | high | patch | Proved by building both DLLs locally: all 8 exports present on both architectures, and writing the two names into one directory leaves one file. The native library is now `folio8_native`, with three cross-platform tests pinning the distinction and a loaded-module path in the error. |

## Implementation Notes

**The public types sit in the GLOBAL namespace, not in a `Folio8` namespace.** `api-surface.md` pins the call site as `Folio8.Render(...)` and `Template.Parse(...)`, and a namespace named `Folio8` makes that spelling unreachable: C# resolves the simple name `Folio8` to the namespace and never reaches a class of the same name inside it. Measured, not assumed — a throwaway project with `namespace Folio8 { public static class Folio8 }` fails with `CS0234: the type or namespace name 'Render' does not exist in the namespace 'Folio8'`, with or without a `using`. The same rule bit the test project, whose original `Folio8.Tests` namespace shadowed the class from the inside; it is `Folio8Tests` now. Nine public types is a small enough surface to carry in the global namespace, and the frozen contract is what decided it.

**The ABI frame is binary, not JSON.** `wasm/cmd/render`'s envelope is JSON because JavaScript parses JSON for free. .NET Framework 4.6 does not: no `System.Text.Json`, and no third-party dependency is allowed in the managed assembly, so a JSON transport would have forced a hand-rolled JSON parser — escape handling included, and the `BINDING_PATH_ABSENT` message contains embedded quotes — into the shipped library. The frame carries the SAME SHAPES (severity, code, elementId, dataPath, message; a payload; a reference list) in a length-prefixed binary form that `BinaryReader`/`BinaryWriter` handle exactly, in about forty lines. `folio8-go/cshared/README.md` is the written contract; `SurfaceTests.TheAbiContractIsDocumented` keeps every export and status code named in it.

**`CallingConvention.Cdecl` is load-bearing.** cgo exports are cdecl on every platform; .NET's `DllImport` default is `Winapi`, which is stdcall on Windows. The mismatch is invisible on x64, which has one convention, and corrupts the stack on x86 — the floor this library exists to reach. It could only have been found on Windows, so it is set deliberately everywhere rather than discovered there.

**A `//go:build !cgo` stub sits beside the c-shared entry.** `matrix.yml` sets `CGO_ENABLED=0` on every leg. Without the stub the directory holds no buildable Go files under that setting and the toolchain fails the package outright rather than skipping it. `imports_test.go` pins both build constraints so the pair cannot drift apart.

**The 4.6 compile project compiles the SOURCES, not a reference to the assembly.** NuGet's compatibility table starts `netstandard2.0` support at `net461`, so a `ProjectReference` from `net46` is refused before a line is type-checked — that would test the packaging rule, not the floor. Compiling `src/Folio8/**/*.cs` against `Microsoft.NETFramework.ReferenceAssemblies.net46` asks the actual question ("do these sources fit inside 4.6's API surface") and answers it on any operating system, so the check runs on the host as well as in CI. **This leaves a real packaging question for story 7:** a NuGet package whose `lib/` folder is `netstandard2.0` is installable from `net461` upward, not `net46`. Reaching literal 4.6 through the package will need a `lib/net46/` asset — the same sources, built for `net46`, from the same repository — or an explicit owner decision that 4.6.1 is the shipped floor. Nothing here forecloses either; the sources are proved 4.6-clean, which is the prerequisite for both.

**A non-diagnostic engine error throws `InvalidOperationException`, not `FolioRenderException`.** Go returns a plain `error` for these (malformed JSON report data, for instance) and folio-js throws a plain `Error` rather than a `FolioRenderError`. Adding a second folio8 exception type would have grown the frozen surface; mapping them onto the BCL type keeps the split identical to the other two languages. `ParityTests` asserts the distinction in both directions, from Go's own recorded outcomes.

**`folio8_abi_version()` is new published contract.** A managed assembly can load a native library from a different build — nothing in the file system stops it — and would then decode a frame grammar that had moved under it and report the result as data. The export allocates nothing and cannot fail, so the binding calls it once before its first real call and refuses on mismatch with a named error. It is `1` today; bump it for any change a caller must be recompiled for.

**The 32-bit CI leg is the one that executes `CallingConvention.Cdecl`.** x64 has a single calling convention, so an x64-only suite passes with the attribute deleted from every `DllImport`. The Windows job therefore swaps in the `win-x86` DLL and re-runs the `net48` target in a 32-bit test host (`x86.runsettings`), which is why the test assembly stays AnyCPU. `net48` only: `setup-dotnet` installs no 32-bit modern .NET runtime, and `net48` consumes the same `netstandard2.0` assembly, so the ABI is proved on x86 either way.

**The native library is `folio8_native`, not `folio8` — and this contradicts a filename in `packaging-matrix.md`.** The managed assembly is `Folio8.dll`; NTFS and the default macOS file system are case-insensitive, so `folio8.dll` IS `Folio8.dll`. Staged into one directory they are one file and the later copy wins. On Windows CI every managed test failed with `EntryPointNotFoundException` on the first P/Invoke while `objdump -p` showed both DLLs exporting all eight symbols undecorated — the loader had resolved the managed assembly. macOS could never have caught it: the native file there is `libfolio8*.dylib`, a different name.

This is not confined to the test harness. `packaging-matrix.md` specifies the RID layout as `runtimes/win-x64/native/folio8.dll`, and on modern .NET that asset is copied into the consumer's output directory **beside `lib/netstandard2.0/Folio8.dll`** — the same collision, for every consumer, on every install. So the name is fixed here rather than worked around in the test project, and `NativeNamingTests` fails on any platform if the two names ever converge again. **`packaging-matrix.md`'s `folio8.dll` needs an owner decision in story 7**: either it becomes `folio8_native.dll` there too, or the managed assembly is renamed. Nothing in this story forecloses either choice.

**Free discipline is measured, not asserted.** The ABI exports `folio8_allocation_count()`, so `FreeDisciplineTests` takes the count, runs the corpus loop, and takes it again — including a loop of FAILING calls, which allocate a buffer too. A second free of one token returns `FOLIO8_ERROR_UNKNOWN_FREE`; that is exercised at the ABI directly, because the managed surface offers no way to free a token twice, which is the point. The test assembly disables xUnit parallelism for this reason: a render in another collection holding a buffer would turn a real guarantee into a flaky measurement.

## Design Notes

**Why a host library exists at all.** Every Windows artifact and the `net48`/4.6 checks run in CI, which is a slow loop for ABI work. A `.dylib` built from the same source gives the binding a real native library to exercise locally, so only the platform-specific half waits for CI. It is never packaged.

**Why the template keeps its bytes.** The same reason as folio-js: a native handle would need deterministic disposal across a boundary that .NET Framework's finalisers do not promise. Re-parsing canonical bytes is cheap and yields identical output.

## Verification

**Commands:**
- `cd folio8-go && go build ./... && go vet ./... && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: green, no golden moved
- `cd folio-dotnet && ./build/build-native.sh && dotnet test` -- expected: green against the host library
- `cd folio-dotnet && dotnet build src/Folio8/Folio8.csproj` -- expected: `netstandard2.0` output with no package references
- CI Windows job -- expected: both DLLs build; `net48` and modern .NET tests pass; the 4.6 compile check succeeds

**Manual checks:**
- Windows behaviour cannot be run on this machine. The CI job is the only evidence, and it must be green before the story is called done.
