---
title: 'Bank Statement, Legal Contract and Electricity Bill examples'
type: 'feature'
created: '2026-09-15'
status: 'done'
route: 'dispatch'
baseline_commit: '72f27e50f13bfe79c5b276c0614747c4cdfa90ff'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-startup-templates/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-startup-templates/example-templates.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 1 ships only the Invoice; the startup dialog needs the other three examples from `example-templates.md`, each showing the engine features its document type needs.

**Approach:** Author `bank-statement`, `legal-contract` and `electricity-bill` as `.folio` + `.sample.json` in `folio-designer/public/templates/examples/`, append their ids to `exampleIds` in dialog order, and let story 1's pipeline render-gate, thumbnail and bundle them.

## Boundaries & Constraints

**Always:**
- Each example meets its row in `example-templates.md`, renders against its sample with zero diagnostics under `-strict`, is canonical, uses only the starter's shipped font chain (declare bold faces in the chain before using `style.bold`), and declares the lowest `version` its features need.
- Fictional English data, no real organization; account, meter and reference numbers obviously fake; neutral two-decimal currency via `formatNumber`; dates RFC 3339 via `formatDate`.
- `exampleIds` order: `invoice`, `bank-statement`, `legal-contract`, `electricity-bill`.
- **Bank Statement:** `transactions[]` table spanning ≥2 pages with its header repeating; `altRowBackground`; `Page {{page}} of {{pages}}` in the page footer; opening/closing balances; a static legend below a content-band `sectionBreak`. Debit/credit keys present on every row, `null` when empty, bound as `{{txn.debit != null ? formatNumber(txn.debit, "#,##0.00") : ""}}`.
- **Legal Contract:** renders ≥2 pages. Clauses are a `clauses[]` table (a text element cannot push later content): a narrow number column from data `no` and a wide column `{{clause.heading}}\n{{clause.body}}`; 8–12 clauses. Parties block bound from `partyA`/`partyB`; document reference and `Page {{page}} of {{pages}}` in the page footer; the two signatory blocks sit below a `sectionBreak` and share one `keepTogether` tag.
- **Electricity Bill:** one page; charges table with `footer: "sum"` on the amount column; 6-month `history[]` table (no chart element); overdue notice with `visibleIf: "overdue"` and `overdue: true` in the sample so Preview shows it; payment `barcode` (Code 128) with an ASCII-only value, sized so bars are not narrower than 0.25 mm; any table below another table sits below a `sectionBreak` or at a position clear of the upper table's fixed row count.

**Never:**
- No changes to `build-examples.mjs` beyond the `exampleIds` array; no pipeline, verifier or asset-bound changes.
- No UI or startup changes (stories 3–4).
- No embedded font assets, no chart workaround drawn with shapes, no Thai text.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| All four examples build | Four ids listed, eight source files present | `build:wasm` emits template, sample and thumbnail per id; `exampleAssets` lists four in order | N/A |
| Statement paginates | `bank-statement` sample | Render has ≥2 pages, zero diagnostics | Go test fails naming the example and the page count |
| Contract paginates, signatures intact | `legal-contract` sample | ≥2 pages, zero diagnostics; both signatory blocks on the same page | Go test fails naming the example |
| Overdue notice | `electricity-bill` sample with `overdue: true` | Notice drawn on page 1; one page; zero diagnostics | Go test fails naming the example |

</frozen-after-approval>

## Code Map

- `folio-designer/scripts/build-examples.mjs:31` -- `exampleIds`; the only line to change in this script.
- `folio-designer/public/templates/examples/invoice.folio` -- story 1's example: font chain, canonical layout, table/footer/qrcode shapes to follow.
- `folio-go/designer_examples_test.go` -- canonical + zero-diagnostic check per `*.folio`; extend with per-example page-count expectations.
- `folio-designer/src/example-assets.test.ts:6` -- pins `['invoice']`; update to the four ids.
- `folio-designer/scripts/build-examples.test.mjs:106` -- directory ↔ `exampleIds` check; must pass unchanged.
- `fixtures/section-break-statement/input.folio:8-29` -- paginating table, `sectionBreak`, `Page {{page}} of {{pages}}`.
- `fixtures/alternating-rows/input.folio` -- `altRowBackground`.
- `fixtures/keep-together/input.folio:6-9` -- `keepTogether` tag (not allowed on tables).
- `docs/examples/formula-visibility.folio:7` -- bare-path `visibleIf`.
- `fixtures/barcode-thai-bill-payment/input.folio:6` -- `barcode` value and 400×50 pt sizing.
- `docs/folio-format.md:113-119,561,611-663` -- version triggers, text clipping, table fields (`footerOf` needed when a footer column's bind is a conditional).

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/public/templates/examples/bank-statement.folio`, `bank-statement.sample.json` -- author per Boundaries -- statement example.
- [x] `folio-designer/public/templates/examples/legal-contract.folio`, `legal-contract.sample.json` -- author per Boundaries -- contract example.
- [x] `folio-designer/public/templates/examples/electricity-bill.folio`, `electricity-bill.sample.json` -- author per Boundaries -- bill example.
- [x] `folio-designer/scripts/build-examples.mjs` -- append the three ids in dialog order -- ship them.
- [x] `folio-go/designer_examples_test.go` -- assert page counts (`bank-statement` ≥2, `legal-contract` ≥2, `electricity-bill` 1, `invoice` 1) and, for `legal-contract`, that both signatory names land on the same page -- verifies pagination is actually exercised.
- [x] `folio-designer/src/example-assets.test.ts` -- expect the four ids in order -- generated module contract.

**Acceptance Criteria:**
- Given `npm run build`, when it finishes, then it exits 0, `verify:offline` passes, and the manifest carries 12 example assets.
- Given each built thumbnail, when opened, then it shows that example's first page and is visibly not blank.
- Given each example opened with its sample in the Designer's Preview, when rendered, then no diagnostic is shown.

## Implementation Notes

- Rendered pages: bank-statement 2 (48 transactions; unanchored section break so the legend follows the table on page 2), legal-contract 2 (10 clauses; signatures under an unanchored break, all members tagged `signatures`), electricity-bill 1, all `-strict` clean. All three declare `4.1` (section break).
- Signatories are bound as `partyA.signatory{name, role}` / `partyB.signatory{name, role}` rather than a top-level `signatories[]`: the expression language has no index syntax, so a collection cannot feed two fixed signature blocks.
- The Go page-text check uses `statementPageRuns` (per-resource ToUnicode); `pageTextsOf` refuses multi-face documents.
- Step-03 matrix audit: the "Overdue notice" row had no covering assertion (only page count and zero diagnostics). Added `"electricity-bill": {{"PAYMENT OVERDUE"}}` to `designerExampleSamePage`; confirmed it fails with `overdue: false` and passes as committed, both with `go test -count=1`.
- Review patches (triage rows 1, 4, 9), all in `folio-go/designer_examples_test.go`: `bank-statement` and `legal-contract` pinned to exactly 2 pages; the contract same-page group widened to "Signed by the authorised representatives" plus both names; `requireTextsOnOnePage` now counts occurrences across all pages (any total other than 1 fails) and names the text that set the reference page. Removing every `keepTogether` still passes: the unanchored section break already moves the signature block as one unit, so the tag is redundant in this layout.
- Post-patch verification: `go test -count=1 ./...` only the pre-existing `TestCorpusMeetsP6ExerciseFloors`; `npm run build` 0 incl. `verify:offline` (77 assets, 12 example); `npx vitest run` 1804/1804; `typecheck` 0; `lint` 0.
- Caveat: `go test` caches this test's result without tracking the example files (they live outside the Go module), so after editing an example run `go test -count=1 -run DesignerExample .` locally; CI runs uncached.
- `example-templates.md` lists `signatories[]{name, role}` for the contract; the implemented shape (`partyA.signatory` / `partyB.signatory`) is the companion's to update, since no index syntax can feed two fixed signature blocks.
- Thumbnails inspected: statement (header, shading, Page 1 of 2), contract (parties, clauses 1–6), bill (overdue notice, readings, charges sum 104.95, history, amount due 199.25, barcode). Sample arithmetic checked: statement running balance ends at the closing balance; bill units × rate, charges + arrears = amount due.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | blind, verification-gap | Signature same-page check cannot detect a missing or broken `keepTogether` | false | The claimed harm (a split signature block shipping green) does not occur: with all 9 `keepTogether` tags removed the block still lands whole, because everything below the unanchored `sectionBreak` moves as one rigid block (`go test -count=1`, file restored byte-identical). The group was still widened to "Signed by the authorised representatives" as a harmless tightening | patched (tightening only) |
| 2 | blind | Repeating table header on the statement's page 2 is untested | low | True; header repeat is engine behaviour covered by fixtures, and asserting it needs a new multi-page text assertion | rejected |
| 3 | blind | `Page X of Y` and the contract reference in the footer are untested | low | True; page-number slots are engine-tested; fix adds new assertions for unlikely regressions | rejected |
| 4 | blind | Page-count ranges `{2, 3}` are looser than the 2 pages rendered | low | A legend or signature block orphaned onto a third page would pass; pinning `{2, 2}` is a direct correction | patch |
| 5 | blind, edge | Overdue notice only tested with `overdue: true` | low | True; the hidden branch was checked by hand with `-count=1`; a committed variant needs a second data file | rejected |
| 6 | blind, edge | Hidden overdue notice leaves an empty band above the readings table | low | True; the engine never pulls content up, so the gap is inherent; fix is a layout redesign | rejected |
| 7 | blind | Amount due (199.25) vs charges total (104.95) has no on-page breakdown | low | With the sample's `overdue: true` the notice states the 94.30 arrears; a breakdown block is new content | rejected |
| 8 | blind, edge | Readings table fits above the charges label only for 2 rows | low | Spec permits a position "clear of the upper table's fixed row count"; the sample has 2 registers | rejected |
| 9 | blind, edge | `requireTextsOnOnePage` misses a duplicate on one page and can name the wrong text in its message | low | Only cross-page duplicates are caught; the mismatch message always cites `texts[0]`; direct correction | patch |
| 10 | blind | Runs joined with `\n` could split a name across runs | maybe-false | Would need an engine change that splits one line's runs; if true it fails loudly, not silently | rejected |
| 11 | blind | `example-templates.md` disagrees with the shipped data shapes | low | True (`signatories[]` vs `partyA.signatory`, unlisted fields); the companion is spec-owned, not this build's code | follow-up: spec update after this build |
| 12 | blind | `example-assets.test.ts` does not check all 12 URLs are distinct | false | URLs are content-addressed per `<id>` stem from one generated module; two ids cannot yield the same filename | rejected |
| 13 | blind | Clause headings do not stand out from clause text | low | One style per column is an accepted limit in Design Notes; changing it means changing the spec's approach | rejected |
| 14 | edge | Empty same-page group passes silently | false | `designerExampleSamePage` is a static literal with no empty groups | rejected |
| 15 | edge | A stale page-count entry for a removed example stops checking silently | low | `build-examples.test.mjs` already fails when the directory and `exampleIds` diverge; unlikely | rejected |
| 16 | edge | A transaction missing the `debit`/`credit` key | false | An absent key is a `BINDING_PATH_ABSENT` render error, so the build fails loudly; the spec requires both keys | rejected |

## Design Notes

Why clauses are a table: a text element paginates but never pushes the elements after it (AD-24), so a long body would overlap the signature block; a table row grows and moves whole to the next page, and the section break carries the signatures below wherever the table ends.

Bold clause headings are not possible inside one table cell (one style per column); numbering and heading live in data and share the body's style.

## Verification

**Commands:**
- `cd folio-go && go test ./...` -- expected: pass except the pre-existing `internal/text` `TestCorpusMeetsP6ExerciseFloors`.
- `cd folio-designer && npm run build` -- expected: exits 0 including `verify:offline`.
- `cd folio-designer && npx vitest run` -- expected: pass.
- `cd folio-designer && npm run typecheck && npm run lint` -- expected: pass.

**Manual checks (if no CLI):**
- Open the four emitted thumbnails and confirm each is the right document's page 1.
