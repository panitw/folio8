---
title: 'Document folio-js and folio-dotnet on the site'
type: 'feature'
created: '2026-09-18'
status: 'draft'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-client-libraries/api-surface.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Both bindings ship with only a package README. The documentation site teaches the Go library alone, so a Node or .NET developer has no installation guide, no first PDF, no error reference and no API reference (CAP-8, CAP-9), and nothing is readable offline in the designer (CAP-10).

**Approach:** Author two guides — `docs/folio-js.md`/`.html` and `docs/folio-dotnet.md`/`.html` — in the existing guides' shape, register them for offline precaching, link them from every page, and add tests that hold each page to its language's real API surface.

## Boundaries & Constraints

**Always:**
- **Each guide covers, in this order:** installation; a complete first PDF; warnings and errors; and a full API reference naming every row of `api-surface.md` for that language.
  - folio-js: promise-based usage, `shipped()`, the Node version floor, ESM and `require()` from CommonJS.
  - folio-dotnet: the supported target frameworks explicitly including .NET Framework 4.6, the Windows-only native platforms with both architectures, what happens on an unsupported platform, and the named load-failure exception.
- **`.md` is the source of truth and `.html` is the published page**, and the two must agree in reader-visible text, as the existing guides do.
- **The `.html` pages follow the existing skeleton exactly:** no document wrapper, inline `<style>` only, system fonts only, the sidebar table of contents, the document bar, per-section ids, the same code-sample markup and token classes, and the same inline table-of-contents script. **No external stylesheet, script, font or image, and no remote host of any kind.**
- **Every page links to every other page.** The three existing guides gain the two new entries in their sidebar and document bar, so the set stays navigable from wherever a reader lands.
- **Offline registration is complete:** the pages are added to the designer's documentation list, the filename-preservation rule, the offline verifier's page list and the end-to-end link check, so both are precached, immutable and reachable with no network.
- **Tests hold each page to its language:**
  - every public item of `folio-js`'s entry points and of `folio-dotnet`'s public surface appears in that language's guide, in both twins, with a floor that cannot pass on an empty scan;
  - the two twins agree in reader-visible text;
  - the guide's first-PDF snippet is the one the package's tested README snippet already runs, so the published code is executed rather than illustrated.
- **The Go guide and its tests are untouched**, and no new file is added under `docs/examples/`, which the Go guide's verbatim-embedding test owns.
- **The release budget holds:** the added pages keep the precached asset count below the warning threshold, and `verify:offline` passes with both pages in the manifest.

**Never:**
- Change either binding's public API, the engine, or any golden.
- Introduce a build step, generator or template for the guides — they are authored, like the existing three.
- Add a remote font, stylesheet, script or image to any page.
- Add `docs/` to the source font-host scan, which deliberately excludes it.
- Publish anything, or change the designer's own documentation entry point beyond the links the guides carry.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| A Node developer reads the page | installation through first PDF | the snippet is the tested README snippet and produces a correct PDF | N/A |
| A .NET Framework 4.6 developer | the page's supported-frameworks section | states 4.6 explicitly, both architectures, and the unsupported-platform behaviour | N/A |
| API completeness | a public item missing from a guide | that language's docs test fails naming the item | a scan finding nothing fails rather than passing |
| Twins disagree | a paragraph edited in one twin only | the agreement test fails | N/A |
| Offline reading | a production build, network off | both pages are precached, immutable and reachable from the guide | `verify:offline` fails if either is missing or unhashed |
| Remote asset | a page references any remote host | the offline verifier fails | N/A |
| Cross-page links | a link to a page not registered | the link check fails | N/A |

</frozen-after-approval>

## Code Map

- `docs/rendering-library.html`: the skeleton to copy — charset/viewport/title, one inline `<style>` with the design tokens and the "system fonts only" note, `.shell` → `aside.sidebar` (`.sidebar-head`, `nav.toc` with one `.toc-group[data-current]` plus a group per sibling page), `main > .col` with `nav.docbar` listing every page, `article#top` with `.article-head`, one `section` per `##`, and the closing inline table-of-contents script. Samples are `<div class="sample"><pre data-lang="…">` with `tk-kw tk-fn tk-type tk-str tk-num tk-key tk-punc tk-com` spans; API tables are plain tables.
- `docs/rendering-library.md`: house style and scope — second person, bold lead-ins, `#`/`##`/`###`/`####`, a `## Warnings and errors` section (:375-417) with a field table, the warning-versus-error split and a worked error example, and a diagnostic-code table (:1128). About 1,200 lines and 8,300 words; the new pages need not match that length.
- `folio8-go/docs_examples_test.go:254,271,283,291,397,419`: `guideTwins` is the hardcoded Go pair, `guideText`/`guideWords` strip tags for comparison, the identifier test parses `folio8` and `fonts` with `go/doc` and floors at 58. **Do not extend this file** — it owns the Go guide and `docs/examples/`. The new tests live in each binding's own suite and follow its shape.
- `folio8-designer/scripts/build-wasm.mjs:486-509`: `documentationPages` (key, stem) is the list to extend; a missing file throws; pages share one group digest, so adding pages re-fingerprints all of them; the emitted `documentation-assets.ts` exports one URL per key.
- `folio8-designer/vite.config.ts:28`: the filename regex that keeps hashed page names verbatim — extend it or the new pages get double-hashed and their links break.
- `folio8-designer/scripts/verify-offline-release.mjs:82,117-119,152-163`: the stem list, the exactly-once immutable asset check, the cross-page link check, and the font-host scan over shipped page bytes.
- `folio8-designer/e2e/documentation-link.spec.ts:24,126`: the stems and fingerprint shape the browser test asserts.
- `folio8-designer/src/release-payload.ts:41,48,67`: `minimumCacheAssets` 10, `warnCacheAssets` 82, `maximumCacheAssets` 90; the current build carries 77 assets, so two pages bring it to 79.
- `folio8-designer/src/App.tsx:3662,3678` and `src/App.test.tsx:1215-1240`: the single documentation link and the test asserting exactly one. Leaving that alone keeps the new pages reachable through the document bar without touching the designer's UI tests.
- `folio-js/README.md` and `folio-dotnet/README.md`: the first-PDF snippets; folio-js's is extracted and executed by `folio-js/test/package.test.ts`, and folio-dotnet's is the consumer program's shape.
- `_bmad-output/specs/spec-client-libraries/api-surface.md`: the row-by-row contract each API reference must cover.

## Tasks & Acceptance

**Execution:**
- [ ] `docs/folio-js.md` + `docs/folio-js.html` -- installation, first PDF, warnings and errors, full API reference -- CAP-8
- [ ] `docs/folio-dotnet.md` + `docs/folio-dotnet.html` -- the same, plus target frameworks, both architectures and unsupported-platform behaviour -- CAP-9
- [ ] `docs/rendering-library.html`, `docs/folio-format.html`, `docs/expression-reference.html` -- add both pages to every sidebar and document bar -- one navigable set
- [ ] `folio8-designer/scripts/build-wasm.mjs`, `vite.config.ts`, `scripts/verify-offline-release.mjs`, `e2e/documentation-link.spec.ts` -- register both pages -- CAP-10
- [ ] `folio-js/test/docs.test.ts` -- twins agree, every public entry point and `shipped()` named, the first-PDF snippet equals the tested README snippet, with a floor -- the page cannot drift from the library
- [ ] `folio-dotnet/test/Folio8.Tests/DocsTests.cs` -- the same against the public managed surface -- likewise
- [ ] `folio8-designer` -- run the production build and `verify:offline` -- both pages precached and under budget

**Acceptance Criteria:**
- Given `docs/folio-js.*` and `docs/folio-dotnet.*`, when each language's docs test runs, then every public item of that library appears in both twins and the twins agree.
- Given a production designer build, when `verify:offline` runs, then both pages are precached, immutable, linked and free of remote hosts, and the asset count is under the warning threshold.
- Given either new page, when it is searched for `http://` or `https://` outside code samples, then nothing is found.
- Given the repository, when the Go guide's tests run, then they are unaffected.

## Spec Change Log

## Review Triage Log

## Implementation Notes

## Design Notes

**Why the designer's own documentation button stays as it is.** It opens the rendering-library guide, whose document bar lists every page, so both new guides are reachable offline without touching `App.tsx` or the designer test that pins exactly one link. Adding entry points is a designer decision, not a documentation one.

**Why the first-PDF snippets are shared with the READMEs.** folio-js's README snippet is already extracted and executed against a packed, offline-installed tarball. Pointing the guide at the same text makes the published code tested rather than illustrative, at no extra machinery.

## Verification

**Commands:**
- `cd folio-js && npm test` -- expected: green, including the new docs test
- `cd folio-dotnet && dotnet test -c Release` -- expected: green, including the new docs test
- `cd folio8-designer && npm run build && npm run verify:offline` -- expected: both pages precached and immutable, asset count under the warning threshold
- `cd folio8-designer && npx playwright test e2e/documentation-link.spec.ts` -- expected: green
- `cd folio8-go && go test -count=1 -run 'Docs' ./...` -- expected: the Go guide's tests unaffected
- `grep -nE "https?://" docs/folio-js.html docs/folio-dotnet.html` -- expected: matches only inside code samples

**Manual checks:**
- Read both pages end to end for the four required sections and for agreement with `api-surface.md`.
