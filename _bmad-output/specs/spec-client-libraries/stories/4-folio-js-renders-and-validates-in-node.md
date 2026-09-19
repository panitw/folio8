---
title: 'folio-js renders and validates in Node'
type: 'feature'
created: '2026-09-17'
status: 'done'
baseline_commit: '699da8d88d3d05cfa67a93e59ff849468c24c4a6'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-client-libraries/api-surface.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Node code can't reach the folio8 engine without shelling out to the CLI. The only wasm build, `folio-go/wasm/cmd/engine`, is the designer's stateful Load/Apply loop. It has 8 MiB input caps and fonts compiled in, so it is not a render API.

**Approach:**
- Add a stateless, render-oriented js/wasm entry to `folio-go` whose font set is passed in by the caller.
- Add a new top-level `folio-js/` package: a promise-based TypeScript API over that entry, matching `api-surface.md` exactly (CAP-1, CAP-2).
- Packaging and the embedded `fonts.shipped()` belong to story 5.

## Boundaries & Constraints

**Always:**
- **API** (ESM, TypeScript declarations), all returning Promises:
  - `parseTemplate(bytes: Uint8Array | string)`
  - `loadTemplate(path)`
  - `render(tpl, data, params, fonts)` resolving to `{ bytes: Uint8Array, diagnostics }`
  - `renderTo(writable, tpl, data, params, fonts)` resolving to `diagnostics`
  - `validate(bytes, data, params, fonts)`
  - `parameterReferences(tpl)`
  - `version`, the engine's `folio8.Version`
- **Types:**
  - `Template` is opaque. It holds the parsed canonical bytes and exposes no editing.
  - `Data` and `Params` take `Uint8Array | string | object`; an object is sent as `JSON.stringify`. `params` may be `undefined`, which maps to Go's nil.
  - `fonts` is a required `Map<string, Uint8Array>`. A missing or wrong-typed argument throws a `TypeError` before the engine is reached.
- **Diagnostics:** `{ severity: 'warning' | 'error', code, elementId, dataPath, message }`, in Go's order, with Go's exact strings. `diagnostics` is always an array, never null.
- **Errors:**
  - A Go `*RenderError` throws `FolioRenderError`, which extends `Error` and carries `.diagnostic` with the same fields.
  - Any other Go error throws a plain `Error` with Go's message.
  - A throw never carries a result, and a `render`/`renderTo` result never holds an `error` diagnostic.
  - `validate` mirrors Go's `Validate` exactly: it resolves to Go's diagnostic slice verbatim and rejects whenever Go returns an `error`. Go reports a predicted error — an absent data path, say — as a returned `*RenderError`, not as an entry in that slice (owner ruling, 2026-09-18).
- **`renderTo`** renders completely first. Only on success does it write the bytes once to the `Writable`, honouring the write callback and backpressure. It resolves after the write completes and **never ends or destroys the stream**, matching Go's `RenderTo`. On failure nothing is written.
- **Byte identity:** a render through folio-js yields the same SHA-256 as the committed `expected.json` for the same inputs. Nothing in `folio-js` lays out, shapes or emits PDF.
- **Determinism:** no clock, environment, network or filesystem input, except inside `loadTemplate`, which reads the one path it is given.
- **Wasm entry:**
  - The entry is `package main` under `folio-go/wasm/cmd/render/` (`js && wasm`). It calls only the public `folio8` API and does **not** import `fonts`.
  - No size caps beyond what wasm memory allows.
  - Bytes cross the boundary with `js.CopyBytesToGo` / `js.CopyBytesToJS`, not base64.
  - It is built with the go.mod toolchain as `GOOS=js GOARCH=wasm go build -buildvcs=false`.
- **Glue:** `wasm_exec.js` is copied from that toolchain's `GOROOT` (`lib/wasm`, falling back to `misc/wasm`). The wasm instance loads lazily once and is shared by every call.
- **Node:** `engines` is `>=22.12` (ESM-only, loadable through `require(esm)`). CI runs Node 24.16.0, the designer's pin.

**Never:**
- Touch the designer engine, the public Go API, or any golden.
- Compile `fonts.Shipped()` into the render wasm.
- Add a runtime dependency to `folio-js`.
- Publish to npm, embed fonts, or write docs (stories 5 and 9).
- Offer a browser build.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Golden render | `colour-strokes` template, its data, shipped fonts | `sha256(bytes)` equals `expected.json` | N/A |
| Warning render | a fixture Go renders with a warning | bytes plus that warning; code, severity and message equal Go's | N/A |
| Render error | template binds a path absent from data | rejects with `FolioRenderError`; `.diagnostic.code` is Go's code | nothing resolved |
| Malformed template | `parseTemplate('{')` | rejects with `FolioRenderError` `TEMPLATE_MALFORMED` | N/A |
| Validate | clean and warning inputs | `[]` or Go's warnings | an absent data path rejects with `FolioRenderError` `BINDING_PATH_ABSENT`, and a malformed template with `TEMPLATE_MALFORMED` — as Go's `Validate` returns them |
| renderTo success | `PassThrough` | the collected bytes equal `render`'s; the stream is still writable afterwards | N/A |
| renderTo failure | failing render | nothing written; rejects | stream untouched |
| Bad arguments | fonts omitted, or data a number | `TypeError` | engine not called |
| Parameter references | template using `params.documentDate` | `['documentDate']`, same as Go | N/A |

</frozen-after-approval>

## Code Map

- `folio-go/render_entry.go:160,246`: `Render` and `RenderTo`. RenderTo renders fully and then writes once. `validate.go:52`, `folio8.go:54,80`, `parameter_references.go:22`.
- `folio-go/diagnostic.go:75,391,461`: `Severity.String()` is `Warning`/`Error`, so lowercase it. `Diagnostic` has no JSON tags. `render_error.go:47`: `RenderError{Diagnostic; Err}`.
- `folio-go/wasm/cmd/engine/main.go:22-96`: the precedent for the `js.Global().Set` host object and `select {}`. Reuse the diagnostic JSON shape at :62-68 and the severity mapping at :358-370. Don't reuse its request type, its caps or `internal/wasm`.
- `folio-go/public_surface_census_test.go:219-262`: a `package main` anywhere passes. A non-main helper package would fail, so keep the entry one `main` package.
- `folio-designer/scripts/build-wasm.mjs:19-23,38` and `wasm-vcs-stamp.mjs:60-64`: the build command and the `wasm_exec.js` lookup. Copy the approach into `folio-js/scripts/build-wasm.mjs`, whose output is git-ignored.
- `folio-designer/src/engine.worker.ts:41-47`: `new Go()`, instantiate, `go.run` without await, then read the global. Under Node, instantiate from `fs.readFile` bytes.
- `folio-go/fonts/fonts.go:159-172`: face name → embedded file. Tests build the same `Map` by reading `folio-go/fonts/<dir>/*.ttf` with that name table.
- `fixtures/*/expected.json`, `input.folio`, `data.json`, `params.json`: the golden inputs. Pick fixtures whose Go golden test renders with exactly `fonts.Shipped()`.
- `folio-designer/package.json` and `tsconfig*.json`: TypeScript 5.9.3, vitest 4.1.11, oxlint 1.79.0 and `@types/node` 24.13.3 as the dev toolchain precedent.
- `.github/workflows/ci.yml:309-340`: the Node job shape (`setup-node` 24.16.0 and the go.mod toolchain); nothing picks up `folio-js/` today.

## Tasks & Acceptance

**Execution:**
- [ ] `folio-go/wasm/cmd/render/main.go` -- a stateless js/wasm host exposing parse, render, validate and parameterReferences over copied bytes and a font map; return a JSON envelope (`ok`, `pdf` length or bytes via CopyBytesToJS, `diagnostics`, or `error{diagnostic|message}`) -- the render-oriented engine entry
- [ ] `folio-js/package.json`, `tsconfig.json`, `.gitignore`, `scripts/build-wasm.mjs` -- ESM package `folio-js`, `build` = build-wasm + tsc emit to `dist/` with declarations, `test` = vitest, `lint` = oxlint -- the package skeleton
- [ ] `folio-js/src/{index,engine,template,errors,types}.ts` -- lazy shared instance, argument checks, conversion, the Promise API and `FolioRenderError` -- CAP-1 and CAP-2
- [ ] `folio-js/test/*.test.ts` -- cover every I/O matrix row. Golden hashes for at least three fixtures, including `colour-strokes` and one Thai-shaping fixture. Warning parity against a Go-generated expectation for the same inputs, recorded in a test-data file produced by a small Go test or `cmd/folio8 validate`. -- proves byte and diagnostic parity
- [ ] `.github/workflows/ci.yml` -- a `folio-js` job: setup-go 1.26.0, setup-node 24.16.0, `npm ci`, build, lint, test -- keeps it green on every commit

**Acceptance Criteria:**
- Given a clean checkout with Go 1.26.0 and Node 24, when `cd folio-js && npm ci && npm run build && npm test` runs, then every test passes and the golden hashes match.
- Given the repo, when `cd folio-go && go vet ./... && go test ./...` runs and `GOOS=js GOARCH=wasm go vet ./wasm/cmd/render` runs, then both are green and every golden hash is unchanged.
- Given `folio-js/src`, when searched, then it contains no layout, shaping or PDF logic, and `package.json` has no `dependencies`.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|---|
| 1 | edge | `renderTo` removes the stream's error listener on the write callback, so a failing write with no caller listener crashes the process | high | patch | Reproduced on Node 24.16.0. Listener now stays until the error is delivered; a test covers it. |
| 2 | blind, edge | No handler recovers a Go panic, so the shared engine dies for the rest of the process | high | patch | `bytesArg` on a non-typed-array panics and exits the program. Handlers now recover; `engine.ts` reboots when `go.run` settles. |
| 3 | edge | `ArrayBuffer`, `DataView`, other typed arrays, `Map` and `Set` were JSON-stringified to `{}` | medium | patch | Silent wrong data. Now `TypeError` before the engine; `null` params accepted as Go nil. |
| 4 | verification-gap | Nothing enforces that the render entry imports neither `fonts` nor `internal/` | medium | patch | Pre-verified: no test reads those imports; a `fonts.Shipped()` fallback would pass every test. Added `imports_test.go`. |
| 5 | blind | The test font table is copied by hand and can drift from `fonts.Shipped()` | medium | patch | Drift would silently change the font set behind the hash claim. `go-parity.json` now records the face names and lengths; the JS tests assert equality. |
| 6 | blind | Test files were never type-checked | low | patch | `tsconfig.json` covered `src` only. Added `tsconfig.test.json` to the lint script. |
| 7 | blind, verification-gap | `main.go` header comment describes fields the code does not have | low | patch | Said `pdf`/`template`; code returns `bytes`. Corrected. |
| 8 | blind | `go-parity.json`'s `folio8Version` unchecked; two case names missing from the covered list | low | patch | Both now asserted. |
| 9 | blind | `optional()` swallowed every read error, not just a missing file | low | patch | Only `ENOENT` is treated as absent now. |
| 10 | blind | The `loadTemplate` test leaked its temp directory | low | patch | Removed after the test. |
| 11 | blind | The CI job lacked the js/wasm `go vet` acceptance check and a timeout | low | patch | Both added. |
| 12 | blind | A render blocks the event loop, undocumented | low | patch | Documented on the module and on `render`. |
| 13 | blind | The frozen validate row contradicts Go and the implementation | medium | intent_gap | Owner ruled 2026-09-18: follow Go. The frozen row and the errors rule were corrected; no code change. |
| 14 | blind, edge | Two globals (`Go`, `Folio8RenderHost`) could collide with another Go wasm module | low | reject | One engine per process in this package; boot now clears a stale host. Namespacing the Go class is not possible from `wasm_exec.js`. |
| 15 | blind | `version` is hand-maintained in `src/version.ts` | low | reject | A test pins it to the engine's value, so drift fails CI at once. |
| 16 | blind | No test covers `validate` resolving an error-severity diagnostic | false | reject | Go never returns one that way (finding 13); the case does not exist. |
| 17 | edge | Invalid UTF-8 in a message would be replaced by `json.Marshal` | low | reject | Messages are engine-authored ASCII prose; the recorded parity data pins the text. |
| 18 | edge | `build-wasm.mjs` deletes the previous wasm before building | low | reject | The build is reproducible from source in seconds; a temp-dir dance adds moving parts. |
| 19 | edge | A retry after a partial boot could leave an orphaned Go instance | low | reject | Boot failures throw before `go.run` in practice, and finding 2's reset covers the live case. |
| 20 | edge | Render results rely on Go never emitting an error-severity diagnostic | false | reject | That is Go's documented contract (AD-14); re-asserting it in the binding would duplicate the rule. |
| 21 | blind, verification-gap | `package-lock.json` missing from the diff | false | reject | I excluded it from the review diff; it is on disk and is committed. |

## Implementation Notes

- **Validate with an absent path rejects; it does not resolve.** Go's `Validate` returns a `*RenderError` (`BINDING_PATH_ABSENT`) for an absent data path and never puts an `error`-severity entry in its slice. folio-js follows Go verbatim (the Always rule), so the I/O matrix's "resolved rather than thrown" wording for that input does not hold; `go-parity.json` records Go's rejection and the test asserts it.
- `version` is a synchronous `const` string (`src/version.ts`); a test holds it equal to the wasm host's `folio8.Version`.
- Go parity expectations live in `folio-js/test/data/go-parity.json`, generated and drift-checked by `folio-go/wasm/cmd/render/parity_test.go` (`FOLIO8_UPDATE_JS_PARITY=1` rewrites it).
- The package is marked `private: true` so it cannot be published before story 5.

## Design Notes

**The template keeps its bytes and re-parses on each call.** A Go-side handle registry would need explicit disposal: `FinalizationRegistry` gives no timing guarantee, so a long-running server would leak templates. Parsing is cheap next to rendering and deterministic, so re-parsing the canonical bytes yields identical output. `parseTemplate` still parses eagerly, so a malformed template rejects at parse, as in Go.

**Fonts are copied into wasm memory on every call** (about 14 MB when the full set is passed). The copy costs milliseconds against a render and keeps the entry stateless. A font-set cache is a later optimisation, not a contract.

## Verification

**Commands:**
- `cd folio-go && go vet ./... && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: green
- `cd folio-go && GOOS=js GOARCH=wasm go vet ./wasm/cmd/render` -- expected: clean
- `cd folio-js && npm ci && npm run build && npm run lint && npm test` -- expected: green, golden hashes equal
