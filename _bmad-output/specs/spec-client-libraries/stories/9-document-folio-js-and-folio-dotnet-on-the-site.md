---
title: 'Document folio-js and folio-dotnet on the site'
type: 'feature'
created: '2026-09-18'
status: 'done'
baseline_commit: '2deba47dd093e22f0c1b37fc352bfc298957ee1e'
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
- [x] `docs/folio-js.md` + `docs/folio-js.html` -- installation, first PDF, warnings and errors, full API reference -- CAP-8
- [x] `docs/folio-dotnet.md` + `docs/folio-dotnet.html` -- the same, plus target frameworks, both architectures and unsupported-platform behaviour -- CAP-9
- [x] `docs/rendering-library.html`, `docs/folio-format.html`, `docs/expression-reference.html` -- add both pages to every sidebar and document bar -- one navigable set
- [x] `folio8-designer/scripts/build-wasm.mjs`, `vite.config.ts`, `scripts/verify-offline-release.mjs`, `e2e/documentation-link.spec.ts` -- register both pages -- CAP-10
- [x] `folio-js/test/docs.test.ts` -- twins agree, every public entry point and `shipped()` named, the first-PDF snippet equals the tested README snippet, with a floor -- the page cannot drift from the library
- [x] `folio-dotnet/test/Folio8.Tests/DocsTests.cs` -- the same against the public managed surface -- likewise
- [x] `folio8-designer` -- run the production build and `verify:offline` -- both pages precached and under budget

**Acceptance Criteria:**
- Given `docs/folio-js.*` and `docs/folio-dotnet.*`, when each language's docs test runs, then every public item of that library appears in both twins and the twins agree.
- Given a production designer build, when `verify:offline` runs, then both pages are precached, immutable, linked and free of remote hosts, and the asset count is under the warning threshold.
- Given either new page, when it is searched for `http://` or `https://` outside code samples, then nothing is found.
- Given the repository, when the Go guide's tests run, then they are unaffected.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|---|
| 1 | verification-gap | Nothing pinned the new guides' reachability: deleting both links from the guide kept every check green while the pages became unreachable offline | high | patch | Pre-verified — only `folio-format-` was asserted. The e2e now requires a `folio-js-` and a `folio-dotnet-` target by name and navigates to each. |
| 2 | blind | Neither guide mentioned the OFL licence or gave the face-name keys a caller needs | medium | patch | The Go guide carries both; the new pages invited callers to build a font set without saying what to call the faces. Both added. |
| 3 | edge, blind | The new pages' sidebars were about 23 entries short of every other page | medium | patch | All five sidebars are now generated from one ordered list; every page carries the same 83 entries. |
| 4 | blind | Only the HTML twins were cross-linked, leaving the Markdown set one-way | medium | patch | All three existing guides and the repository README now name both new pages. |
| 5 | blind | folio-js's three install rules had no headings or anchors, unlike folio-dotnet's | low | patch | Now sections with ids, visible in every sidebar, including what a reader below the Node floor sees. |
| 6 | blind | folio-dotnet never said whether a render may be called concurrently | medium | patch | Answered from the ABI's own contract: calls are safe from several threads, the allocation table is mutex-guarded. |
| 7 | blind | "the two libraries carry the same names" was false | low | patch | Reworded to the same surface in the same shape, each spelled its language's way, with examples. |
| 8 | blind | No worked params example in either guide | low | patch | Both build a params object with `documentDate`, pass it, and use parameter discovery. |
| 9 | blind | folio-dotnet's FontSet listed twelve members in prose while every neighbour got signatures | low | patch | Given the same signature block, constructors included. |
| 10 | blind, verification-gap | The .NET surface check matched names anywhere in the prose | medium | patch | `Count`, `Add`, `Error` and friends passed on ordinary sentences. Names must now appear in a code span or signature block; red-proved. |
| 11 | edge | The .NET override skip exempted members this library declares | low | patch | Skips only when the base definition is outside the assembly. |
| 12 | edge | folio-js's entry points were a literal pair, and star or default exports would escape the scan | low | patch | Derived from `package.json` exports; unresolvable forms now throw. |
| 13 | edge, verification-gap | The twin comparison ignored punctuation, and prose outside the markers could drift | medium | patch | Tokens now include punctuation, and both tests assert nothing but chrome sits outside the twin region. |
| 14 | verification-gap | The .NET first-PDF claim said the consumer tests run that program, which nothing compiled | medium | patch | Softened to what is true: the consumer programs make exactly these calls, differing only in taking the fixture from the command line. |
| 15 | blind | Both pages shipped the generic documentation title | low | patch | Each has its own title, outside the twin region. |
| 16 | blind | The story recorded a point-in-time asset count | low | reject | The count is enforced by `verify:offline` against the warning threshold; the note is context, not a check. |

**Round 1 — 15 findings, all accepted and fixed.**

| Finding | Fix |
| --- | --- |
| The e2e pinned only a `folio-format-` destination, so deleting both new links from the guide's chrome stayed green while the two pages became unreachable offline | The test now requires a `folio-js-` and a `folio-dotnet-` target by name and visits each, asserting its title; the stale test name and messages were rewritten |
| Neither guide gave the shipped faces' licence, said that embedding a face redistributes it, or listed the face-name keys | Both gained the key table, a build-your-own example, and the OFL-1.1 / redistribution note, cross-referring the Go guide's `fonts` section |
| The new pages' sidebars carried abridged copies of the other pages' section lists | All five sidebars are now generated from one list of verbatim groups; every page carries the same 83 entries |
| Only the HTML twins were cross-linked | `rendering-library.md`, `folio-format.md` and `expression-reference.md` (and their HTML twins) now link both new guides; the Go guide's companion sentence names four references and says the same engine renders from Node and .NET |
| folio-js's three load-bearing install rules were unanchored run-in paragraphs | They are `###` sections with ids, in every sidebar, and the floor section now says what a reader below 22.12 sees |
| folio-dotnet never answered whether the API is thread-safe | A "Calling from several threads" section states it, from `folio8-go/cshared/README.md` and the mutex-guarded allocation table in `cshared/cmd/folio8/main.go` |
| "the two libraries carry the same names" was false | Reworded to the same surface in the same shape, each spelled its language's way, with the three renamings shown |
| No worked params example | Both guides gained a "Passing params" section building and passing one, with `documentDate` and `parameterReferences` |
| `FontSet` was twelve members of prose while its neighbours got signatures | It now carries the same signature block, constructors included |
| The C# surface check matched whole words over the whole page, so `Count`, `Add`, `Error` passed on ordinary prose | It now matches only inside code spans and signature blocks, with its own vacuity floor |
| The override skip exempted members this library declares | It skips only when the base definition's declaring type is outside this assembly |
| The JS entry-point list was a literal pair | Entry points are derived from `package.json`'s `exports`, and a star or default export the scan cannot resolve fails the test |
| The twin comparison ignored punctuation | Both comparisons now tokenise words AND single punctuation marks, with Markdown's own syntax removed fence-aware; each also asserts that nothing but page chrome lives outside the markers |
| folio-dotnet claimed the first-PDF block is what the consumer tests run | Softened to what is true: the consumer programs make exactly these calls, differing only in the fixture argument and where the font set is held |
| Both new pages shipped the generic `<title>` | Each has its own, outside the twin region |
| The repository README listed three guides | It lists five |


## Implementation Notes

**The twins are compared over a marked region, token by token — punctuation included.** Both `.md` and `.html`
carry `<!-- twin:begin -->` / `<!-- twin:end -->` markers; outside them sit the parts that
exist in one form only (the page's sidebar, document bar and source note; the Markdown's
closing note). Inside, each twin is reduced to its `[A-Za-z0-9]+` word sequence — HTML tags
and entities become spaces, Markdown code-fence info strings and link targets are dropped —
and the two streams must be identical. Markdown's own syntax — fences and their info
strings, table pipes and rules, heading hashes, list bullets, emphasis, code ticks and link
targets — is removed fence-aware, so a code sample's own backticks, pipes and dashes survive
as content. That is robust to where a `<span class="tk-…">` falls inside a code sample
(`folio8.<span>Render</span>` and `folio8.Render` tokenise the same) while failing on a
paragraph, a bracket or a generic edited in one twin only. Each suite also asserts that
nothing but page chrome lives outside the markers, so prose cannot be authored where the
comparison does not reach. The same normaliser is written twice, once in
`folio-js/test/docs.test.ts` and once in `folio-dotnet/test/Folio8.Tests/DocsTests.cs`.

**Surface scans.** folio-js's entry points come from `package.json`'s `exports` rather than a
literal list, and a star or default re-export the scanner cannot name fails the test instead of
being skipped; type-only exports (`Diagnostic`, `Severity`, `Template`, …) are part of the
contract and leave no runtime trace, so the scan reads the sources and is cross-checked against
the entry points' real runtime exports. It finds 16 names and floors at 16. folio-dotnet's is
reflection over the shipped assembly: exported types plus the members each type *introduces*
(constructors share their type's name; property accessors, conversion operators and the indexer
have no name a guide can print; an override whose base definition is declared outside this
assembly — `ToString`, `Equals(object)`, `GetHashCode`, `GetObjectData` — is the framework's
contract, not this library's). It finds 44 names and floors at 40, and each must appear in a
**code span or signature block**, not merely somewhere on the page: `Count`, `Add`, `Error` and
`Message` are ordinary English.

**First-PDF snippets are the READMEs'.** `docs/folio-js.md` carries folio-js README's first
```js block verbatim — the one `test/package.test.ts` packs, installs offline and runs against
a corpus fixture — and `docs/folio-js.html`'s `your-first-pdf` sample decodes to exactly that
text. The same holds for folio-dotnet's first ```csharp block, the shape its consumer suite runs.

**Sidebars are generated from one list.** `docs/*.html` sidebars are rebuilt from a single
ordered set of `toc-group` blocks, so no page can carry an abridged copy of another page's
section list; all five carry the same 83 entries.

**Registration.** `documentationPages` in `build-wasm.mjs` gained `['js', 'folio-js']` and
`['dotnet', 'folio-dotnet']`; the pages share one group digest, so all five stems were
re-fingerprinted, and `documentation-assets.ts` now exports five URLs. `vite.config.ts`'s
verbatim-name regex, `verify-offline-release.mjs`'s stem list and `e2e/documentation-link.spec.ts`'s
stem list and fingerprint pattern each gained both stems. `App.tsx` and its tests are untouched:
the designer's one documentation link still opens the Go guide, whose document bar now lists
all five pages.

**Budget.** The release carries 79 cache assets against `warnCacheAssets` 82 and
`maximumCacheAssets` 90; `verify:offline` prints no approach warning.


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
