---
id: SPEC-client-libraries
companions:
  - ./api-surface.md
  - ./packaging-matrix.md
  - ../../../docs/rendering-library.md
  - ../../../docs/folio-format.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# folio8 Client Libraries — folio-js and folio-dotnet

## Why

folio8 is **a library, not a service**, and today that library exists in exactly one language. An application that wants a PDF must either be written in Go, shell out to the `folio8` CLI, or rebuild the engine itself — so the JSON-first, deterministic rendering folio8 exists to provide is unreachable from the two runtimes most line-of-business reporting actually lives in: Node services and .NET applications. This is an **opportunity to capture**: the engine, the format and the golden corpus are already done and proven, and what stands between them and a much larger set of callers is binding work, not rendering work.

The .NET floor is what makes this urgent rather than merely nice. Reporting workloads of the kind folio8 targets — statements, invoices, regulatory documents — disproportionately run on long-lived .NET Framework applications that cannot be moved to modern .NET. A binding that reaches **.NET Framework 4.6** reaches those applications where they are; one that requires .NET Core does not, and they stay on whatever they replaced JasperReports with.

## Capabilities

- **CAP-1**
  - **intent:** A Node application can turn a `.folio` template plus JSON data and params into PDF bytes in-process.
  - **success:** A Node script renders `fixtures/` corpus documents and writes a PDF whose SHA-256 equals the committed expected hash for each.

- **CAP-2**
  - **intent:** A Node application can validate a template against data and params and read the engine's diagnostics without rendering.
  - **success:** Validating a fixture known to produce a warning returns a diagnostic with the same code, severity and message text the Go `Result` carries for the same inputs; a clean fixture returns none.

- **CAP-3**
  - **intent:** A .NET application can render the same way, from .NET Framework 4.6 through modern .NET, on the supported platforms.
  - **success:** One test project multi-targeting `net46` and a current .NET renders the corpus on each target and matches the committed hashes; both targets consume the same managed assembly.

- **CAP-4**
  - **intent:** A .NET application can validate a template and read the same diagnostics.
  - **success:** For every corpus fixture, the diagnostic sequence from .NET equals the sequence from Go, compared by code, severity and message.

- **CAP-5**
  - **intent:** Both libraries are continuously proved to render byte-identically to `folio8-go`, rather than asserted to.
  - **success:** A CI job renders the whole corpus through each binding on every supported runtime and platform and fails on any hash that differs from the committed expected PDF — the same standard [matrix.yml](../../../.github/workflows/matrix.yml) already holds the Go targets to.

- **CAP-6**
  - **intent:** A Node developer can install folio-js and reach a first PDF without a build step, a toolchain, or a compiler on their machine.
  - **success:** `npm install` followed by the documented first-PDF snippet produces a correct PDF on a machine with no Go, no C compiler and no network access after install.

- **CAP-7**
  - **intent:** A .NET developer can install folio-dotnet and have the right native binary load for whichever process bitness they end up in, without writing configuration or probing paths.
  - **success:** A `PackageReference` and the documented snippet produce a correct PDF from an **AnyCPU** project on 64-bit Windows, an AnyCPU project forced 32-bit, and an explicitly x86 project — on both `net46` and a modern .NET target, with no caller-authored load logic.

- **CAP-11**
  - **intent:** When the native library cannot be loaded, a developer is told what went wrong and what to change, rather than being handed a platform stack trace.
  - **success:** Forcing each failure mode — wrong bitness, missing RID asset, blocked P/Invoke — throws a folio8 exception naming the detected process bitness, the RID and filename sought, and the likely cause.

- **CAP-8**
  - **intent:** The documentation site teaches folio-js end to end — installation, a complete first PDF, errors and warnings, and a full API reference.
  - **success:** A developer who has not seen this repository renders a correct PDF from the page alone, and every public item in [api-surface.md](./api-surface.md) appears in its reference section.

- **CAP-9**
  - **intent:** The documentation site teaches folio-dotnet the same way, including the .NET Framework 4.6 path explicitly rather than by implication.
  - **success:** As CAP-8, and the page states the supported target frameworks, the platforms with native binaries, and what happens on an unsupported one.

- **CAP-10**
  - **intent:** Both new documentation pages are readable offline in the designer exactly as the existing three guides are.
  - **success:** After a production build, both pages are precached release assets reachable offline, and `verify:offline` passes with them in the manifest.

## Constraints

- **Byte-identity with `folio8-go` is the acceptance bar, not a goal.** For the same template, data, params and font set on a given build toolchain, both libraries must emit the bytes the golden corpus records. This is what rules out reimplementing the engine in TypeScript or C#.
- **One engine, two delivery mechanisms.** folio-js runs the Go engine compiled to wasm; folio-dotnet calls a `c-shared` build of the same engine over a C ABI. Neither library may contain rendering logic of its own — layout, shaping, pagination and PDF emission stay in `folio8-go`.
- **.NET Framework 4.6 is the floor, and it bans the modern toolkit.** No wasm runtime targets it; it predates `Span<T>`, `System.Text.Json` and `DllImportResolver`. The binding is therefore plain `DllImport` marshalling, and the managed assembly targets `netstandard2.0` to span 4.6 through modern .NET from one build.
- **The determinism commitments carry over unchanged.** No clock, no locale, no network, no filesystem beyond calls the caller makes, no ambient environment input. `SOURCE_DATE_EPOCH` stays a CLI-only convenience; library callers pass `documentDate` as an ordinary param.
- **`docs/*.md` is the source of truth and `docs/*.html` is the published page.** Both must be authored for each new page, and they must agree.
- **A new documentation page costs release budget.** Each `.html` page must be registered in [build-wasm.mjs](../../../folio8-designer/scripts/build-wasm.mjs), must use system fonts only — the `forbidden-font-hosts` scan fails the build on a remote font host — and spends a cache-asset slot against `maximumCacheAssets = 90`.
- **folio-dotnet is Windows-only, and ships both `win-x86` and `win-x64`.** .NET Framework projects default to AnyCPU, which runs as a 64-bit process on 64-bit Windows and cannot load a 32-bit DLL; shipping one architecture alone would make `BadImageFormatException` the normal first experience. Resolution is by **process bitness at load time**, because .NET Framework 4.6 has no `DllImportResolver` — RID-based package layout and probing are the only mechanisms available.
- **Both libraries embed the full shipped font set (~14 MB), mirroring `fonts.Shipped()` exactly.** This diverges from the Go `folio8`/`folio8/fonts` split, which exists so Go callers opt into that weight: here it is paid on every install to keep the call site one step and the face list identical to the engine's. Fonts remain an explicit argument at the API level; embedding changes how the bytes arrive, not whether the caller names them.
- **`fonts.Shipped()` can never shrink after the tag.** Under the v1.0.0 semver commitment its contents are fixed for the life of v1, so any future slimming must be additive — a `fonts/cjk` sub-package and a `fonts.ShippedCore()` beside an unchanged `Shipped()`. Accepted deliberately; the retiering is specified in [SPEC-shipped-font-tiers](../spec-shipped-font-tiers/SPEC.md), currently deferred.
- **The prerequisite chain: move the designer surface out of the public API → land a signed-off colour fixture and retire the two unused style codes → cut `folio8-go/v1.0.0` → build the bindings.** The tag is `v1.0.0`, not the `v0.1.0` that RELEASING.md and D-1.1.c name (owner decision): it commits to semver, so any later breaking change needs a `/v2` import path every caller edits. Both documents need rewording. Cutting it carries RELEASING.md's checklist and is irreversible: D-1.1.c fixes the public API at it. It replaces backlog Story 15.3, now deprecated, and inherits DW-4's surface re-measure and the engineering-lead checkpoint. Changelog policy is GitHub release notes per tag.
- **What does and does not gate the tag (owner rulings).** Before the tag:
  - **DW-147.** No fixture declares a colour today, so the byte-identity claim has never covered `style.color` or element box strokes. A golden fixture declaring both lands with a recorded human sign-off.
  - **D-7.8.2.** `STYLE_COLOR_INVALID` and `STYLE_LINE_SPACING_INVALID` are both retired, because the audit found no consumer branches on either and removing a public code is free only before the tag (AD-14).

  Released from the gate: 8.4d and 8.4k, which are designer-release and `lint` work, and DW-230, which stays open on Story 15.2. **DW-68 is ruled: v1.0.0 ships the clip.** An over-tall aggregate-only keep-together group keeps rendering clipped with a warning.
- **v1.0.0 freezes render and validate only — about 59 items.** Before the tag, the canvas/designer surface of package `folio8` and the whole `folio8-go/wasm` package move behind `internal/`. What stays public: `LoadTemplate`, `ParseTemplate`, `Render`, `RenderTo`, `Validate`, `ParameterReferences`, `SerializeTemplate`; the types `Template`, `Data`, `Params`, `FontSet`, `Result`, `Diagnostic`, `Severity`, `RenderError`; the `DiagCode*` and `Severity*` constants, `Version`, `LocaleTableVersion`, `MaxParameterReferenceNameLength`; and `fonts.Shipped`. The move must not change a single corpus hash.
- **folio-js is promise-based over the `js/wasm` target.** It reuses the designer's proven build and Go's `wasm_exec.js` shim rather than adding a second wasm target. Rendering is pure CPU work, so a synchronous API over `wasip1/wasm` was defensible and was declined: async keeps a large render offloadable to a worker thread, so it cannot block a Node server's event loop. `renderTo` takes a Node `Writable`, mirroring Go's `RenderTo(w io.Writer, …)`.
- **Diagnostics are a shared contract, not a per-language convenience.** Codes, severities and message text match the Go `Result` across all three languages, because callers port between them.

## Non-goals

- **The canvas and designer command API.** `Canvas`, `CanvasWithTextPaint`, `ApplyComponentCommand`, `ApplyPageSetupCommand`, `PreviewComponentMove`, `TableColumns` and the projection types become `internal/` to `folio8-go` and serve the designer alone. These libraries render and validate; they do not edit templates.
- **Reimplementing the engine in either language.** No TypeScript or C# renderer, not even a partial one for a fast path.
- **A folio8 service, server, or HTTP API.** folio8 remains a library.
- **Database connectivity.** Unchanged from the Go engine: the caller prepares the JSON.
- **A browser or bundler build of folio-js.** Node is the target. This is a scope choice, not a technical limit — folio8-designer already runs this same wasm engine in a browser worker — so it can be revisited without re-establishing feasibility.
- **Non-Windows platforms for folio-dotnet.** No Linux, macOS or ARM native binaries in this spec. folio-js, being wasm, is unaffected and runs wherever Node does.
- **Authoring `.folio` templates from either library.** `SerializeTemplate` and the editing surface are out.

## Success signal

A developer on a .NET Framework 4.6 application that has never been able to use folio8 adds a NuGet package, pastes ten lines from the documentation page, and gets a PDF whose bytes are indistinguishable from what the Go engine, the CLI and the designer's preview produce for the same template and data. The same holds for a Node developer and `npm install`. The corpus job in CI proves it for every supported runtime and platform on every commit, so "indistinguishable" is a tested property rather than a claim.

## Assumptions

- The managed assembly targets `netstandard2.0` — the widest TFM both .NET Framework 4.6 and modern .NET consume from a single build. Not confirmed by the owner.
- Package identities are npm `folio-js` and NuGet `folio-dotnet`, taken from the names in the request. Registry availability is unverified.
- Both libraries expose diagnostics carrying the same codes, severities and messages as the Go `Result`, on the assumption that callers port between languages.
- folio-js is Node-only for this spec; the browser case is deferred, not ruled out.
- `folio8-go/wasm` is the **designer** engine — a `Load`/`Apply` command loop — so folio-js needs its own render-oriented wasm entry point rather than reusing that shell.

## Release preconditions — checked

| RELEASING.md precondition | Result | Evidence |
| --- | --- | --- |
| 1. Licence manifest ships | **Ready** | `TestManifestUpToDate` passes. Attaching `lint/MANIFEST.md` to the release happens when the tag is cut. |
| 2. Public surface reviewed as a whole | **Not met — resolved by story 1** | 287 items in `folio8` today; story 1 cuts the frozen surface to about 59, which story 1's spec checkpoint reviews as a whole. |
| 3. Call-graph walker precise | **Holds** | The pinned injectivity test passes, and an independent `go/ast` scan found 41 methods under 41 distinct names. Re-check after the move, since it relocates methods. |

**RELEASING.md's protection for #2 does not exist as written.** It names "the pinned surface census, which reddens when anything is added or removed"; no such test exists. The closest, `TestDocsGuideNamesEveryExportedIdentifier`, checks documentation and floors the count at 250 — a floor the move will break, since the surface drops far below it. Story 2 adds the real pinned census before tagging.
