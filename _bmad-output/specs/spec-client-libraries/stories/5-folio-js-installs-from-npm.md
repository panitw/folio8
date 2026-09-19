---
title: 'folio-js installs from npm with no build step'
type: 'feature'
created: '2026-09-18'
status: 'done'
baseline_commit: '1b5d312dd49026c9b9b72e0973090fa1e7024880'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-client-libraries/packaging-matrix.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `folio-js` is `private: true`, ships no fonts and builds its `.wasm` from the sibling Go module. An installer would get a package that cannot render: the API demands a font set and nothing in the package provides one.

**Approach:** Embed the full `fonts.Shipped()` set, add `shipped()`, and make the tarball self-contained — prebuilt wasm, fonts and licences, no install scripts and no dependencies (CAP-6). Prove it by installing the packed tarball offline and rendering from it.

## Boundaries & Constraints

**Always:**
- **Fonts:** the complete set, eleven faces, about 14 MB, byte-identical to `fonts.Shipped()`. Copied from `folio-go/fonts/` by the build, not committed to `folio-js/`, and git-ignored in the working tree.
- **`shipped()`** is exported from the subpath `folio-js/fonts`. It returns a `Promise<Map<string, Uint8Array>>` whose keys and bytes equal `fonts.Shipped()`, reading the packaged files. It caches after the first call and stays an explicit argument — nothing becomes ambient.
- **Self-contained tarball:** prebuilt `.wasm`, `wasm_exec.js`, `dist/`, fonts, `README.md`, `LICENSE` and every font licence and notice. No `dependencies`, no `postinstall`, no `prepare`, no compiler, no network use after install.
- **A publish cannot ship an incomplete tarball.** `prepack` rebuilds wasm, fonts and `dist/`, and packing fails if any is missing or if a face's bytes differ from the Go source.
- **Package metadata:** name `folio-js`, version `1.0.0`, `private` removed, MIT, `files` allowlisting exactly what ships, `exports` for `.` and `./fonts` with types, and `repository`, `homepage`, `keywords` and `engines`. A field records the engine version the wasm was built from.
- **README:** the documented first-PDF snippet, which must be the one the offline test runs. The full guide is story 9.
- **Third-party licences ship and are named.** The font licences and notices are copied beside the faces, and the README says which faces ship under which licence.
- **Determinism and parity are unchanged:** the same wasm as story 4, no rendering logic in JS, and the golden hashes stay identical.

**Never:**
- Publish to npm, or run `npm publish` in any script or CI job. Publishing is the owner's, after the done checkpoint.
- Slim, subset or re-tier the font set, or fetch a face over the network.
- Commit font bytes or the `.wasm` into `folio-js/`.
- Add a runtime dependency, a browser build, or a bundler entry point.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Offline install | `npm pack`, then install the tarball into a temp project with the network refused and scripts ignored | install succeeds; the README snippet renders a fixture whose SHA-256 equals its `expected.json` | N/A |
| Shipped fonts | `await shipped()` | 11 entries; names and byte lengths equal `go-parity.json`'s `shippedFaces` | N/A |
| Repeat call | `shipped()` twice | the same cached Map, one read of each file | N/A |
| Tarball contents | `npm pack --dry-run` | contains `dist/`, `wasm/`, 11 faces, each font licence and notice, `LICENSE`, `README.md`, and no test, source or lockfile | N/A |
| Incomplete pack | fonts or wasm missing | `prepack` fails naming what is missing | packing stops |
| Drifted face | a packaged face differs from `folio-go/fonts/` | the build fails naming the face | N/A |
| Install hygiene | the packed `package.json` | no `dependencies`, no install scripts | N/A |

</frozen-after-approval>

## Code Map

- `folio-js/package.json`: `private: true` and `version: 0.0.0` today; `files` is `["dist","wasm"]`; `exports` has `.` only; devDependencies pin TypeScript 5.9.3, vitest 4.1.11, oxlint 1.79.0.
- `folio-js/scripts/build-wasm.mjs`: builds `wasm/folio8-render.wasm` (12.4 MB) and copies `wasm_exec.js`. Extend it, or add a sibling script, to copy the faces.
- `folio-go/fonts/`: eleven face directories, each holding its `.ttf` plus `LICENSE-OFL.txt` and `NOTICE.md`. `fonts.go:159-172` is the name → file table; `fonts.go:20` records the total, 14,782,604 bytes. `NotoSansSC-Regular.ttf` alone is 10.1 MB.
- `folio-js/test/data/go-parity.json`: already carries `shippedFaces` (name plus byte length) written by `folio-go/wasm/cmd/render/parity_test.go`. It is the drift check for both the copy and `shipped()`.
- `folio-js/test/helpers.ts`: builds the same map from the Go tree for tests; after this story the tests should use `shipped()` and keep `helpers.ts` only if something still needs the Go-side path.
- `folio-js/src/index.ts`, `engine.ts`: the API and the lazy engine; `src/version.ts` holds the engine version string.
- `folio-js/.gitignore`: `node_modules/`, `dist/`, `wasm/`. Add the fonts directory.
- `.github/workflows/ci.yml`: the `folio-js` job runs npm ci, build, lint and test — the offline-install test must fit inside it.
- `RELEASING.md`: the precedent for a written publish procedure, and where the npm steps belong.

## Tasks & Acceptance

**Execution:**
- [x] `folio-js/scripts/build-fonts.mjs` (or an extension of `build-wasm.mjs`) -- copy the eleven faces and their licences from `folio-go/fonts/`, verifying each against `go-parity.json`'s `shippedFaces` -- the embedded set cannot drift
- [x] `folio-js/src/fonts.ts` -- `shipped()`, cached, resolving packaged paths from `import.meta.url` so it works from `node_modules` -- CAP-6's missing half
- [x] `folio-js/package.json` -- drop `private`, version `1.0.0`, add the `./fonts` export, `files`, metadata, the engine-version field, and `prepack` -- a publishable, self-contained package
- [x] `folio-js/README.md`, `folio-js/LICENSE` -- the first-PDF snippet and the licence set -- an installer can start, and the font licences travel with the bytes
- [x] `folio-js/test/package.test.ts` -- `npm pack` into a temp dir, install it with the network refused and scripts ignored, run the README snippet in a child process, compare the hash; plus the tarball-contents and install-hygiene rows -- proves CAP-6 rather than asserting it
- [x] `folio-js/test/fonts.test.ts` and the existing suites -- `shipped()` parity and caching; switch the golden tests to `shipped()` -- the tests use what installers use
- [x] `RELEASING.md` -- an npm section: `npm publish` runs only on the owner's go-ahead, after the tarball test, with the engine version recorded -- publishing has a written procedure

**Acceptance Criteria:**
- Given a clean checkout, when `cd folio-js && npm ci && npm run build && npm run lint && npm test` runs, then everything passes including the offline install.
- Given the packed tarball, when a project installs it with no Go, no C compiler and no network, then the README snippet writes a PDF matching the committed hash.
- Given `folio-js/`, when the repository is searched, then no `.ttf` and no `.wasm` is tracked by git.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|---|
| 1 | verification-gap | `prepack`, the publish gate, is exercised by no test or CI step | medium | patch | Pre-verified: deleting the script kept every test green. The hygiene test now asserts its contents. |
| 2 | verification-gap, blind | `shipped()`'s failure reset is never executed | medium | patch | A retained rejected promise would poison the cache for the process. A mocked first-read failure now covers it. |
| 3 | edge, verification-gap | `shipped()` returned the same mutable Map to every caller | medium | patch | One caller deleting a face corrupted the set process-wide. Each call now gets its own Map over the cached bytes. |
| 4 | blind, edge | Unguarded `JSON.parse` in `package-check.mjs` and `fonts.ts` | medium | patch | A corrupt manifest gave a raw SyntaxError instead of the refusal the script exists to print. Both now name the problem. |
| 5 | blind, edge | `build-fonts.mjs` left a half-copied `fonts/` when a source file was missing | medium | patch | Verified by moving a NOTICE.md. Now a named problem, and the directory is removed in a `finally`. |
| 6 | edge | The build compared sizes only, so a same-size byte drift passed | medium | patch | The matrix row requires the build to fail. It now compares sha256 against the Go source. |
| 7 | blind, edge | The manifest's `byteLength` was written but never checked | low | patch | Now validated against the packaged file's size. |
| 8 | blind | `requiredFiles` restated twice in the test | low | patch | Three copies had to agree; the test now imports the list. |
| 9 | blind, edge | `.gitignore` patterns unanchored | low | patch | `fonts/` would have matched any nested directory. All three anchored. |
| 10 | blind | `setup.ts` claimed one read per run | low | patch | `setupFiles` runs per test file. Comments corrected. |
| 11 | edge | A trailing `--root` with no value silently used the real tree | low | patch | Now exits 2 naming the flag. |
| 12 | blind | RELEASING.md contradicted itself and hardcoded 1.0.0 | low | patch | Step 1 reworded; the version is read from package.json. |
| 13 | blind | No published npm version maps back to a commit | medium | patch | A directory-prefixed `folio-js/v<version>` tag is now pushed before publishing, per AD-22. |
| 14 | blind | `publishConfig`, `sideEffects` and the ESM-only note missing | low | patch | All three added. |
| 15 | blind | The story's verification command used `git status`, which passes vacuously | low | patch | The spec's Verification section now uses `git ls-files`. |
| 16 | blind | No `folio-js/CHANGELOG.md` | low | reject | The changelog policy is GitHub release notes per tag (story 2); a second changelog would be a competing record. |
| 17 | blind | npm provenance not enabled | low | reject | Provenance needs a CI publish with a registry token, and the spec forbids publishing from CI. |
| 18 | verification-gap, blind | The README's licence table restates the face table | low | reject | Static documentation text; the shipped licences themselves are copied and checked by the build. |
| 19 | edge | The suite packs with `--ignore-scripts`, so `prepack` never runs over the real tree | low | reject | A second 12 MB wasm rebuild inside the suite for a path `npm pack --dry-run` covers in verification; finding 1 closes the wiring gap. |

## Implementation Notes

- **The face table lives once, in `folio-js/scripts/faces.mjs`.** `build-fonts.mjs` copies from it and writes `fonts/manifest.json`; `src/fonts.ts` reads only the manifest, so the shipped package never restates the name -> file mapping and cannot disagree with what was copied.
- **Two checks, at two moments.** `build-fonts.mjs` checks every face's SIZE against `go-parity.json`'s `shippedFaces` as it copies and deletes the output directory if any differs. `scripts/package-check.mjs` — run by `prepack`, after the build — re-reads the tree about to be packed and checks every face's BYTES (sha256) against `folio-go/fonts/`, plus the presence of everything the `files` allowlist promises. It takes `--root`/`--go-fonts`/`--parity`, which is how `package.test.ts` exercises its refusals against deliberately broken trees in milliseconds instead of rebuilding a 13 MB wasm.
- **The tests use the packaged fonts, not the Go tree.** `test/setup.ts` awaits `shipped()` once before any suite and `helpers.ts` hands the resulting Map to the many render call sites that are not in async position (`arguments.test.ts` passes thunks). `helpers.ts` keeps `repoRoot`/`repoFile`/`sha256` because the fixtures still come from the repository.
- **`package.test.ts` packs with `--ignore-scripts`.** `npm test` runs after `npm run build`, so the tree is already what `prepack` would produce; skipping it keeps the suite from rebuilding the wasm with the Go toolchain a second time. `prepack`'s own gate is covered directly, against broken trees.
- **The README snippet is extracted, not transcribed.** The test pulls the first ```js block out of `README.md` and runs it verbatim in the installed project, so the documented snippet and the tested snippet cannot drift.
- **`fixtures/alternating-rows` is the offline fixture**: small, data-driven, no params, and already in `golden.test.ts`'s list of fixtures the Go golden renders with exactly `fonts.Shipped()`.
- **`folio8EngineVersion`** in `package.json` records the engine the wasm was built from; `package.test.ts` holds it equal to `src/version.ts`.
- **Measured tarball:** 53 files, 11.7 MB packed, 27.9 MB unpacked — 13.0 MB wasm, 14.0 MB fonts.

## Design Notes

**Why the fonts are copied at build time rather than committed.** The bytes already live in `folio-go/fonts/` and are embedded in the engine; a second tracked copy would be 14 MB of duplicate history that can silently diverge. Copying at build keeps one source of truth, and the `shippedFaces` check makes divergence a build failure rather than a rendering difference.

**Size.** The tarball carries about 12.4 MB of wasm and 14 MB of fonts. That is the cost of the decided "embed the full set" constraint, and `NotoSansSC-Regular.ttf` is 10 MB of it. Slimming stays available additively (`SPEC-shipped-font-tiers`) and is not this story's business.

## Verification

**Commands:**
- `cd folio-js && npm ci && npm run build && npm run lint && npm test` -- expected: green, offline install included
- `cd folio-js && npm pack --dry-run` -- expected: lists dist, wasm, the 11 faces and the licences; no sources or tests
- `cd folio-go && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: green, no golden moved
- `git ls-files folio-js | grep -E '\.(ttf|wasm)$'` -- expected: no output
