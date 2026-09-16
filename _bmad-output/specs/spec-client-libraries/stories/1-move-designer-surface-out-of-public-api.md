---
title: 'Move the designer surface out of folio8-go public API'
type: 'refactor'
created: '2026-09-17'
status: 'done'
baseline_commit: 'cfb7a552a13963452efe5ba774747b0cbd2fff3a'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `folio8-go` exports 313 items across `folio8`, `fonts` and `wasm`, and nearly all of them serve the designer. The tag `folio8-go/v1.0.0` (story 2) would place that designer surface under semver, so a later rename or removal would need `/v2`.

**Approach:** Leave the designer implementation in package `folio8`, unexport it, and move its data types, constants and `ComponentCommandError` into a new `internal/designer` package. Root exposes the designer functions to in-module callers through function variables it assigns in `init()`. Move package `wasm` to `internal/wasm`. Afterwards the public surface is render and validate only, about 59 items, and no render output changes.

## Boundaries & Constraints

**Always:**
- **What stays public.** Package `folio8` keeps exactly:
  - funcs `LoadTemplate`, `ParseTemplate`, `Render`, `RenderTo`, `Validate`, `ParameterReferences`, `SerializeTemplate`
  - types `Template`, `Data`, `Params`, `FontSet`, `Result`, `Diagnostic`, `Severity`, `RenderError`, with their current methods
  - consts: every `DiagCode*` and `Severity*`, plus `Version`, `LocaleTableVersion`, `MaxParameterReferenceNameLength`

  Package `fonts` keeps `Shipped`.
- **What leaves public.** Funcs `Canvas`, `CanvasWithTextPaint`, `ApplyComponentCommand`, `ApplyPageSetupCommand`, `PreviewComponentMove`, `TableColumns`, `SnapToGrid`, `PreviewIdentity`, `AssetBytes`, `StandInData`; all 20 designer types; consts `MaxCanvasMillipoints`, `GridIncrement`, `MaxStandInDataBytes`, `MaxStandInDataPaths`, `StandInInstant`; and all of package `wasm`.
- **Render-neutral.** Every golden-corpus hash is identical before and after. The designer's wire JSON is byte-identical: field names and tags do not change.
- **The designer keeps working.** `npm run build:wasm`, the designer unit suite and the e2e compile all pass.
- **Designer documentation is deleted from the public guide** (owner decision). Remove from `docs/rendering-library.md` and its `.html` twin every section documenting the moved API: `ComponentCommandError`; the designer parts of "Template and asset helpers" (`AssetBytes`, `StandInData`, `PreviewIdentity`, `TableColumns`); "Authoring and canvas"; "Authoring command catalog"; "wasm integration". Also update the intro and line 9 so `folio8-go/wasm` is no longer listed as public. `ParameterReferences`, `Version` and `LocaleTableVersion` stay documented. The two files must stay in step. Nothing is moved to contributor docs.
- **The wasm clock comes from outside `internal/`.** `internal/wasm` may not import `time`, so the main package under `folio8-go/wasm/cmd/engine` supplies the render-elapsed clock. Tests use a fixed fake.

**Never:**
- Weaken, exempt or skip a lint rule to admit the move. If moved code breaks an internal-only rule (float64, map-range, forbidden imports) and cannot be fixed without changing behaviour, stop and ask.
- Export new accessors on `Template`, or break the opacity check in `testdata/templateopaque`.
- Change any rendering, validation, diagnostic or command behaviour.
- Add the pinned surface census, which is story 2's work, or cut a tag.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Outside caller | module consumer references `folio8.Canvas` or imports `folio8-go/wasm` | compile error: undefined, or internal package not allowed | N/A |
| In-module caller | `internal/wasm` calls a designer function through `internal/designer` | same projection or command result as today, same JSON bytes | a component failure still matches `*designer.ComponentCommandError` via `errors.As` |
| Bridge not assigned | a package imports `internal/designer` without importing `folio8` | caught by a test | a test asserts every bridge variable is set once `folio8` is imported |
| Handle of the wrong type | a bridge function receives something other than `*folio8.Template` | a clear panic naming the expected type | programmer error; never reachable from wasm |

</frozen-after-approval>

## Code Map

- `folio8-go/folio8.go:33`: `Template` has only the unexported fields `doc` and `derivedFooters`, and it stays opaque.
- `folio8-go/page_setup.go`: effectively all designer code. Consts and `Canvas*` types occupy lines 1–246; also `SnapToGrid`, `Canvas` (:949), `CanvasWithTextPaint` (:1092), `ApplyPageSetupCommand` (:2468). Its calls to render helpers stay within root.
- `folio8-go/component_commands.go`: `ComponentCommandError` (:26) and its constructor (:34), which must move with the type, then `ApplyComponentCommand` (:218) and about 35 unexported handlers.
- `folio8-go/{group_movement,pages_command,table_columns_projection,stand_in_data,asset_bytes,preview_identity,canvas_authored_properties}.go`: purely designer code.
- `folio8-go/section_break.go` and `barcode_element.go`: mixed files. `ParseTemplate` and `Render` depend on their non-designer halves, so they stay in root. `barcode_element.go:183-340` defines the Canvas barcode/QR types, which move.
- `folio8-go/table_min_height.go:84`: kept code that calls `componentFailure`, which will construct `designer.ComponentCommandError`.
- Designer types reference only builtins, each other and `geom.Length` (rank 0), never a root type, so they can live in `internal/designer`.
- `folio8-go/wasm/engine.go`: imports `time` at :23 and brackets `Render` with `time.Now` at :255–257. Uses about 16 designer references. It moves to `internal/wasm`.
- `folio8-go/wasm/cmd/engine/main.go`: `package main`, `//go:build js && wasm`. It stays at this path, so the designer build path `./wasm/cmd/engine` in `folio8-designer/scripts/wasm-vcs-stamp.mjs:64` does not change. Update its imports (:44, :266).
- `lint/internal/rules/stagerank.go:57-81`: an unranked `internal/` directory is a finding. Add `designer` (1, since it imports only geom) and `wasm` (strictly above every internal package it imports; use 9 if unsure). Forbidden-imports, map-range and float64 rules then apply to both.
- `folio8-go/docs_examples_test.go:400,415`: change the package list to `{".", "fonts"}` and set the `total < 250` floor to the new measured count. Keep the guard.
- `folio8-designer/src/engine-bounds-mirror.test.ts:74`: points at `folio8-go/wasm/engine.go`, which moves.
- `folio8-go/component_commands_test.go:2220,3034`: read `wasm/cmd/engine/main.go` by path, which does not change.
- Tests using moved symbols: 42 root test files. 39 are `package folio8` and switch to the unexported names and `designer.` types. `formatlocale_command_test.go`, `pick_declares_cuts_ext_test.go` and `starter_template_test.go` are `package folio8_test`: call through `internal/designer`. Tests under `wasm/` move with the package.
- `README.md:29`: points at `folio8-go/wasm/`.
- `font_cache_sites_test.go:97-116`, `render_arch_test.go` and `testdata/templateopaque`: each scans root or pins root call sites. Designer code stays in root, so all should pass unchanged. Confirm they do.

## Tasks & Acceptance

**Execution:**
- [x] `folio8-go/internal/designer/` -- create the package: the 20 types moved verbatim (tags unchanged), the 5 consts, a `ComponentCommandError` constructor, and typed function variables whose template parameter is `any` (the opaque `*folio8.Template`) -- the only way for code outside root to reach root's unexported designer code without exporting `Template` internals
- [x] `folio8-go/*.go` (designer files, `barcode_element.go`, `table_min_height.go`, new `designer_bridge.go`) -- rename the 10 funcs to unexported, repoint type and const references at `designer.`, and assign every bridge variable in `init()` -- removes the surface while keeping behaviour
- [x] `folio8-go/wasm/*` -> `folio8-go/internal/wasm/` -- move the package, call designer functions through the bridge, and take a `func() int64` millisecond clock in place of `time` -- keeps it lint-clean under `internal/`
- [x] `folio8-go/wasm/cmd/engine/main.go` -- import `internal/wasm` and `internal/designer`, supply the clock from `time` -- the only shell allowed to read a clock
- [x] `lint/internal/rules/stagerank.go` -- rank `designer` and `wasm` -- unranked packages fail lint
- [x] root `_test.go` files, `docs_examples_test.go`, `engine-bounds-mirror.test.ts`, `README.md` -- update per Code Map; add a test asserting every bridge variable is set -- keeps the guards meaningful
- [x] `docs/rendering-library.md` and `docs/rendering-library.html` -- delete the designer sections listed in Boundaries (~882–897 intro, line 9, ~1135–1343 designer parts, ~1344–1988) in both files together -- the published guide documents only the public API

**Acceptance Criteria:**
- Given the moved tree, when a census of exported identifiers runs over package `folio8` (non-test files) and package `fonts`, then it lists exactly the "What stays public" set and nothing else.
- Given a fresh module that `require`s this checkout, when it references any moved symbol or imports `folio8-go/wasm`, then compilation fails.
- Given the full test suite with `-count=1`, then every golden-corpus hash and every existing guard passes. A lowered census floor is not a failure.

## Implementation Notes

- **Bridge as specified.** `internal/designer/bridge.go` declares 10 function variables with a template parameter typed `any`, and `folio8-go/designer_bridge.go` assigns them in `init()`. A foreign handle panics naming `*folio8.Template`. A nil `*Template` passes through, so each function keeps its own nil refusal, as the public functions did before.
- **`ComponentCommandError` moved with its only constructor**, `designer.NewComponentCommandError`, because it embeds an unexported `error`. `componentFailure` stays in root and calls it.
- **`internal/wasm` reads no clock.** `NewEngine(clock func() int64)`. `wasm/cmd/engine/main.go` stays at its path, supplies a monotonic clock from `time`, and keeps the designer build path `./wasm/cmd/engine` unchanged. Engine tests use a fake clock that advances exactly 7 ms per reading.
- **Stage ranks: `designer` 1, `wasm` 9.** The lint compares its rank table to the ladder in `_bmad-output/planning-artifacts/architecture/architecture-folio-2026-08-23/ARCHITECTURE-SPINE.md`, so both rows were added there too. Lint was confirmed to cover `internal/wasm`: a temporary `import "time"` there failed it.
- **Docs census:** the package list is now `{".", "fonts"}` and the floor is 60, the new measured count. The guard remains.
- **Guide deletion reached two more pages:** the sidebar navigation in `docs/folio-format.html` and `docs/expression-reference.html` linked to the deleted sections and was updated. Every remaining link from the designer or docs into the guide resolves; 23 anchors were checked.
- **Test adjustments that were not pure renames:**
  - `engine-bounds-mirror.test.ts`: three regexes read Go signatures that now say `designer.CanvasBand` and `designer.CanvasProjection{}`.
  - `TestStandInGeneratorReadsNoClock`: its sanity anchor moved from `const StandInInstant` to `func standInData(`.
  - `component_commands_test.go`: a local variable `canvas` became `initial`, so it no longer shadows the renamed function.
- **Added during the matrix audit:** matrix row 1 (an outside caller) had no committed test. `public_surface_consumer_test.go` builds separate consumer modules. It starts with a positive control, where the kept API must build, then checks that `folio8.Canvas`, `folio8.ApplyComponentCommand`, `folio8.CanvasProjection`, `folio8-go/wasm`, `internal/wasm` and `internal/designer` each fail to build, naming the cause. A mutation check that re-exported `Canvas` made the test fail as intended.
- **Not run:** `wasm/cmd/engine/main_test.go` builds only for js/wasm. It was vetted for that target, but its tests were not executed, as before this story. Some prose comments in designer TypeScript still name `wasm/engine.go` at its old path; they are not load-bearing and were left alone.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| B1 | blind | Stage-rank lint cannot see `internal/wasm` importing the module root | false | Any `internal/` stage the root imports cannot import root (Go rejects the cycle). The only `internal/` package that imports root is `internal/wasm`, which root never imports: the intended shape. | rejected |
| B2 | blind | Bridge `tpl any` gives up compile-time type safety | low | Real, but it is the approved design (Design Notes). The one in-module caller always passes `*folio8.Template`. The fix would change the spec's mechanism and add complexity. | rejected |
| B3 | blind | A package importing `internal/designer` without `folio8` would call a nil bridge function | low | Unreachable today: every production caller (`internal/wasm`, `wasm/cmd/engine`) imports `folio8`. Matrix row 3 asks for exactly the test that exists. A guard would add branches. | rejected |
| B4 | blind | Consumer test probes 3 of ~20 removed exports | low | True, but a complete pinned surface is out of scope by frozen intent ("Never: add the pinned surface census, which is story 2's work"). The census acceptance criterion was verified separately: exactly the kept set. | rejected (out of scope) |
| B5 | blind | Census floor lowered from 250 to 60 without justification | false | The frozen spec says to set the floor to the new measured count and keep the guard. 60 is that measured count, and `TestDocsGuideNamesEveryExportedIdentifier` passes against it. | rejected |
| B6 | blind | Designer and worker-protocol reference deleted, not moved | false | Owner decision in frozen Boundaries: "Designer documentation is deleted from the public guide … Nothing is moved to contributor docs." | rejected |
| B7 | blind | Stale references after the move (`wasm/engine.go` paths, `folio8.AssetBytes`, `wasm.Engine.Apply`) | low | Path and `folio8.AssetBytes` mentions verified stale (`internal/wasm/engine.go:276`, designer TS and e2e comments). The `wasm.Engine.Apply` part is false: `internal/wasm` is still `package wasm`. Fix is a direct correction. | patch (P1) |
| B8 | blind | No migration note for a breaking removal; README calls an installable `main` "Not public API" | low | No release and no versioned importers exist (the module is untagged), so a migration note has no reader. A `main` package exports nothing importable. A note would be new content, not a correction. | rejected |
| B9 | blind | `NewEngine(nil)` panics; `elapsedClock` untested; fake clock's fixed step couples tests | false | Every caller passes a non-nil clock; a loud failure on an unreachable input is correct. The real clock is covered end to end by `e2e/preview-evidence-rail.spec.ts:119`. An exact-step assertion fails loudly if the number of clock readings changes. | rejected |
| B10 | blind | Spine edited in a historical file; bridge doc overstates the surface; `ComponentCommandError{}` literal has a nil error | false | The lint reads its rank ladder from that `ARCHITECTURE-SPINE.md`, so the edit is required. The doc comment mirrors the frozen intent's own "render and validate" wording. No caller builds the literal, and a zero-value literal was equally possible before the change. | rejected |
| B11 | blind | Consumer test brittle (hard-coded `go 1.25.0`, `GOPROXY=off`, exact error wording) | low | Same pattern as the existing green `TestDocsGuideProgramsRunAgainstTheWorkingTree`. The toolchain is pinned (AD-22), and a reworded message fails loudly with a cause. Making it looser adds parsing. | rejected |
| E1 | edge | `NewEngine` with a nil clock crashes on the first `Render` | false | As B9: no caller passes nil. | rejected |
| E2 | edge | `elapsedMs` off by up to 1 ms because two truncated millisecond readings are subtracted | low | Verified. `RenderResult`'s comment (`internal/wasm/engine.go:49`) promises a render under a millisecond reports `0`, and the baseline's `time.Since(...).Milliseconds()` guaranteed it. Now a render that crosses a millisecond boundary reports 1. Fix is a direct correction of the clock's unit. | patch (P2) |
| E3 | edge | Injected clock going backwards yields a negative `elapsedMs` | false | The only real clock is `time.Since(origin)`, which uses the monotonic reading and cannot go backwards. | rejected |
| E4 | edge | Importing `internal/designer` without `folio8` makes bridge calls panic | low | Same root cause and verdict as B3. | rejected |
| E5 | edge | `ComponentCommandError` literal leaves the embedded error nil, so `Error()` panics | false | As B10: no caller builds the literal, and it was equally possible before the change. | rejected |
| E6 | edge | Consumer test's control fails on a clean module cache | false | `go test` builds the package, filling the module cache, before the test runs. The identical `GOPROXY=off` harness in `docs_examples_test.go:454` already passes in CI without `-short`. | rejected |
| E7 | edge | `internal/wasm/engine.go:276` comment names removed `folio8.AssetBytes` | low | Verified stale. | patch (P1) |
| E8 | edge | Old `wasm/engine.go` path persists in comments and a test title | low | Verified: `page_setup.go:320,849`, `component_commands.go:3611`, two root test comments, and 13 designer TS/e2e sites including the test title at `engine-bounds-mirror.test.ts:859`. Several also cite stale `:240-246` line numbers. | patch (P1) |
| V0 | verification-gap | No verification gaps found | false | Not a defect; the layer reports coverage confirmed for the clock, the bridge and the public API. | rejected |
| V1 | verification-gap | `internal/wasm/engine.go:276` comment names `folio8.AssetBytes` | low | Same as E7. | patch (P1) |
| V2 | verification-gap | `designer.SnapToGrid` bridge variable has no caller | low | Verified: 0 non-test callers, and nothing outside root called exported `SnapToGrid` at the baseline. Dead surface in the bridge. Fix is a direct deletion (variable, assignment, test entry). | patch (P3) |
| V3 | verification-gap | Designer TS comments still cite `folio8-go/wasm/engine.go` | low | Same as E8. | patch (P1) |

## Design Notes

A bridge instead of moving the code: the designer functions read and write `Template`'s unexported state in about 150 places and share about 50 unexported render helpers. `ParseTemplate` and `Render` in turn depend on `section_break.go`, `barcode_element.go` and `componentFailure`. Moving the code out of root would either force `Template` internals public or split shared helpers. Moving all of root behind `internal/` would make ~53 files and 124 tests subject to lint rules they were never written for. Only the data types move, because they need nothing from root.

```go
// internal/designer
var Canvas func(tpl any) (CanvasProjection, error) // tpl is *folio8.Template; assigned by folio8's init
// folio8/designer_bridge.go
func init() { designer.Canvas = func(tpl any) (designer.CanvasProjection, error) { return canvas(tpl.(*Template)) } }
```

## Verification

**Commands:**
- `cd folio8-go && go build ./... && go vet ./...` -- expected: clean
- `cd folio8-go && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: all pass, golden hashes unchanged
- `cd folio8-go && go test -count=1 -tags=matrix -skip '^TestCorpusMeetsP6ExerciseFloors$|^TestShippedFacesReproduceFromUpstream$|^TestCrossTargetByteIdentity$|^TestFMAProbeDiverges$' ./...` -- expected: all pass
- `cd lint && go test -count=1 ./...` -- expected: all pass, no new findings
- `cd folio8-designer && npm run build:wasm && npx vitest run && npm run test:e2e:compile` -- expected: all pass
