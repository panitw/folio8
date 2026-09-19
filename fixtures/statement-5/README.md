# `statement-5` — the Customer Account Statement at 5 pages

**Story 4.7** (the C4 gate · S1, S2, S3, S4 · AD-21, AD-22, AD-1, AD-5, AD-14, AD-23). Golden digest
`70dce051495cf68daa71fe8185aa2467acfd82d10fb195439a4d71bcf41944d0`.

95 bound transactions, 5 pages, 127363 bytes.

## RE-RECORDED 2026-08-30 by Story 15.1 — one `Tm` x-coordinate, and the sign-off it invalidated

The previous digest was `7f67b317c0a1925a404f8435bd4736b85e831a213f5a69fc2a2934a742ff950f` at 127,343 bytes.
This one is 127,363 bytes: **+20**, which is **+4 bytes per page** on 5 pages, the same
per-page delta all four statement goldens took.

**What moved, measured rather than reasoned about** (D-15.1.1; full evidence and commands in
`_bmad-output/implementation-artifacts/evidence/15-1/attribution.md`). Commit `791ed00` created
`folio-go/text_alignment.go` and wired `style.align` into the emitter for the first time. The page
footer `e4` declares `"align": "right"`, and until that commit the engine parsed, validated,
round-tripped and displayed that request and then drew the text at the **left** edge of its box
anyway. Resolving both PDFs with `splitPageContentStreams` and diffing them page by page shows
exactly one differing line per page, out of hundreds:

```
recorded: 1 0 0 1 436 53.88 Tm
produced: 1 0 0 1 522.474 53.88 Tm
```

The glyph string on the next line is byte-identical — the same text, moved. `436` is 3 characters
and the new value is 7: **+4 bytes, once per page**, on the one element this document repeats on
every page. An operator census over the page streams (`re f S rg m l Tm Tj TJ q Q w`) is identical
before and after, which rules out the element-box paint and the text-ink bracket mechanically:
neither added or removed an operator, and a coordinate is the only thing that changes byte length
without changing the operator sequence.

`36 + 400 + 123 = 559` is the footer box's right edge in absolute page coordinates, and the new
position is exactly that edge minus the packed line width. The old position was exactly the box's
left edge. **The new rendering is the correct one and the old one was the defect** — the owner's
ruling, D-R7.6.

**The human sign-off does not transfer, and it was not edited.** D-4.7.1 invalidates
`fixtures/statement-signoff.json` **in whole across all four documents** the moment any one digest
moves. Story 15.1 left that file untouched and halted; a person re-reads the four rendered
documents and writes the record. Until they do,
`TestGoldenDigestAgreesAtEveryDeclaredSite` and the matrix-gated
`TestStatementSemanticSignOffIsRecorded` are red, and that red is the process working.

The independent-reader acceptance below was **re-run on these bytes**, same reader and same version,
with identical output.

## Why this fixture exists

### The coverage history, recorded here so a future reader at a gate does not have to re-derive it

Measured at this story's baseline (`df8cbcc`), with `grep -l '"table"' fixtures/*/input.folio`:
**no committed golden contained a table** before this story. Not one of the nine recorded
`expected.pdf` files in this repository carried a single table element, so no recorded byte in
the corpus could tell a correct table from a broken one.

The sharper measurement is the one the story immediately before this made. **Story 4.6's
unconditional-clip mutation — a live behaviour change to table row clipping — reddened NO GOLDEN
AT ALL**, while reddening the table behaviour suite. The golden corpus was, at that moment,
structurally incapable of expressing a table defect. This family of four is what closes that.

### And what recording them does NOT prove

This story creates these references and then compares against the very files it just produced.
That limb is **circular on this first pass** and proves nothing until the bytes move. Byte
identity across `darwin/arm64`, `linux/amd64`, `linux/arm64` and `js/wasm` proves **DETERMINISM,
not correctness** — this project's own words, from the multi-page incident: *"a deterministically
wrong file is byte-identical to itself."*

The one non-circular check is a person. See *Human semantic acceptance* below.

### Two independent producers, and neither replaces the other

The four statement goldens do **not** supersede the table behaviour suite (D-4.7.0's obligation
2). They are **two independent producers of the same answers**: the golden pins bytes and cannot
say whether they are right; the behaviour suite asserts properties and cannot say whether the
bytes moved. That pairing is the construction that has reliably worked all programme. The suite
is six named files — `table_render_test.go`, `table_render_row_test.go`,
`table_pagination_test.go`, `table_header_repeat_test.go`, `table_footer_test.go`,
`table_row_clip_test.go`, 90 top-level tests between them — guarded by NAME, never by a count
(`table_behaviour_suite_test.go`, D-2.5.1). Do not retire, thin, or fold that suite into these
goldens.

## What this document's content can express

- **The smallest statement with continuation pages.** It is the first document in the
  repository able to express a table header that fails to repeat, or a footer aggregate emitted
  somewhere other than the last page. On a one-page document neither defect is visible.
- **Four page boundaries**, at rows 19/20, 41/42, 63/64 and 85/86 — the places where
  row-atomic pagination can be wrong in a way no interior page shows.
- **CJK subsetting at volume**: 41 distinct CJK glyphs, against `multi-script-fallback`'s one.

Shared by all four documents, because the four data documents differ **only** in the length of
the bound collection:

- **A page count that is a CONSEQUENCE OF THE DATA**, not a geometric construction. Contrast
  `fixtures/page-count-{1,5,20,50}`, whose page counts are placement arithmetic — N elements
  one content-window apart — and are therefore accidentally true at any N and say nothing
  whatever about a table.
- **A repeated table header** (five columns), a **sum footer** on the last page only, and
  **row-atomic pagination**.
- **Latin, Thai and CJK inside ONE table row under ONE fallback stack.** Measured at this
  story's baseline: no test and no fixture in the repository put all three shipped scripts
  inside a table — the table suites use a single-face `fontFamily` per subtest.
- **A wrapping cell whose wrap point falls BETWEEN TWO WORDS** (`...retainer for the` /
  `regional office`), so a break one word early or late is visible in the text, not merely in a
  line count.
- **A Thai break asserted against the frozen S4 corpus.** `fixtures/expected-breaks/expected_breaks.json`
  item `thai-001`, `ประเทศไทย`, labelled `["ประเทศ", "ไทย"]` with an interior seam at rune 6.
  The Note column's 34pt width (28pt usable after padding) was chosen so the whole string
  cannot fit on one line while its first labelled word can — **the column width is the knob;
  the string set is frozen.** Thai strings are REUSED rather than minted because
  `expected_breaks.json` is S4, the authority Thai break correctness is judged against for the
  life of the project, and minting would create a second signed corpus that could disagree with
  it.
- **A lowered/stacked Thai mark positioned by GPOS inside a table cell** — `ปั ฟั ที่ ป้ำ`,
  verbatim from `fixtures/shaped-text/input.folio`. The `ฟั` cluster's declared GPOS x-offset of
  +21 font units reaches the page as a TJ adjustment.
- **A generated date supplied through `params`, never a clock.** The value `2026-08-27` occurs
  nowhere else in the fixture — every transaction date and the statement period fall in
  2026-07 — so finding it in the render can only mean it came from `params`.
- **A logo image embedded ONCE and referenced from every page.**
- **A money column that DISCRIMINATES exact-decimal arithmetic from binary floating point.**
  Every amount's cent part is chosen so the value is **not** exactly representable in float64:
  none is a multiple of 0.25. As FIRST recorded this family used cent parts drawn from
  `{0, 25, 50, 75}` — all exactly representable — so all four goldens would have been
  **byte-identical** under a renderer whose money path went through float64, and the sum
  footer's assertion would still have passed (the story's code review, Finding 1). Measured on
  the values now committed, the canonical naive money path (parse to `float64`, accumulate,
  scale with `int64(total*100)`) renders a **different total** in all four documents and a
  different Amount cell for dozens of individual rows.
  `TestStatementMoneyColumnDiscriminatesFloatArithmetic` holds that permanently: it checks the
  value class by exact integer arithmetic (a decimal of *m* hundredths is exactly representable
  in binary64 **iff** 25 divides *m*, so "no multiple of 0.25" and "not representable" are the
  same statement), keeps the old quarter-integral class as the control that does **not**
  discriminate, and pins the totals a binary money path would draw. The wrong implementation is
  deliberately **not written** anywhere under `folio-go/` — three separate guards forbid binary
  floating point there, `_test.go` files included, and all three were measured firing during
  this story's finisher pass. This is the column AD-23 exists for.

## What it cannot express

- **It does not cross the page-9-to-page-10 digit boundary.** Every page number in this
  document is one digit; `statement-20` and `statement-50` carry that case.
- It cannot express a pagination drift that only accumulates over tens of pages.
- **The CJK content is NOT asserted.** The Chinese descriptions are read by the human sign-off
  and are counted as glyphs in the `/ToUnicode` subset, but no assertion in this repository
  checks what they say or how they break. They are deliberately NOT drawn from
  `fixtures/expected-breaks/` — that file is S4, the frozen authority, and putting one of its
  labelled items into an unasserted cell would create a second apparent authority over the same
  rules.
- **Only ONE of the two Thai cells carries an asserted break.** `thai-001` (`ประเทศไทย`) is
  compared against the frozen label; `thai-002` (`เก็บเงิน`) is not, and cannot be at any usable
  column width — measured: `thai-002` breaks only below a content width of ~23,400 mp, and
  `thai-001`'s first labelled word `ประเทศ` needs 23,704 mp, so every width that breaks the one
  clips the other. The `เก็บเงิน` cell is here for the human reader, not for a machine check.
- **A float64 money path that FORMATS BY ROUNDING is still invisible here**, and no fixture at
  this magnitude could make it visible: accumulating ~1,000 amounts of this size in a `float64`
  drifts by about 1e-9, and hiding a rendered cent needs half a cent. The money column
  discriminates the scale-to-minor-units spelling of the defect, which is the canonical one; it
  does not discriminate every conceivable one.

## Independent-reader acceptance (D-000.53)

No golden this story records is accepted until a reader this project did not write resolves it
into the objects it claims to contain.

**Reader**: `qpdf` **12.4.0** (`/opt/homebrew/bin/qpdf`).

**Invocation and output, verbatim:**

```
$ qpdf --check fixtures/statement-5/expected.pdf
checking fixtures/statement-5/expected.pdf
PDF Version: 1.7
File is not encrypted
File is not linearized
No syntax or stream encoding errors found; the file may still contain
errors that qpdf cannot detect

$ qpdf --show-npages fixtures/statement-5/expected.pdf
5
```

`qpdf --check` resolves the file's cross-reference table and object graph independently of
folio8's own writer and reports no structural defect; `qpdf --show-npages` resolves the page tree
and reports **5**, matching this document's declared page count exactly.

## Human semantic acceptance (D-000.22 / D-2.3.5) — PENDING, and tracked by a failing test

The machine-checkable half of D-000.22 is **complete and not deferred**: page count and the
`{page -> row indices}` partition, header repetition on every page, the footer aggregate's value
and its page, the params-supplied date's provenance, the frozen Thai break, GPOS inside a cell,
the logo's single definition, structural validity, and cross-target byte identity are all
asserted at recording (`statement_semantics_test.go`, `matrix_test.go`).

The irreducibly-human half — *does this statement READ correctly to a person* — is outstanding,
and is tracked by a **failing, matrix-gated test**, not by this paragraph:
`TestStatementSemanticSignOffIsRecorded` in `folio-go/statement_signoff_matrix_test.go`. It is
red until `fixtures/statement-signoff.json` names a reader, a date, what they examined, and
**all four** of this family's digests.

**One record, invalidated in whole.** The record covers all four documents, and if **any one**
digest moves the **whole** record is invalidated. Over-invalidating costs a re-read;
under-invalidating would let three attestations survive a systemic change.

## Matrix registration

Registered in `matrixDocuments` (`folio-go/matrix_test.go`), in
`.github/workflows/matrix.yml`'s `docs=` list and its four per-target upload paths, and in
`declaredEpic2GateObligations` (`folio-go/byte_neutrality_test.go`).

**The four legs are RUN IN THIS STORY**, not deferred to the Epic 4 boundary gate. D-000.4 names
4.7 explicitly as a per-story matrix override (`matrix_test.go`'s own comments list the
overrides as *"1.2, 1.5, 1.8, 2.4 and 4.7"*), and the reason here is structural rather than
cadence: **Story 4.7 IS the C4 gate, so deferring its own matrix to the gate would be the gate
certifying itself.**
