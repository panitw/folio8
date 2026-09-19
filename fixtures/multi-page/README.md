# `multi-page` — the repository's first document with more than one page

**Story 2.6** (FR30 · NFR4 · AD-4). Golden digest
`66ce0ee477fa1ce5e42d51bcc87d859bcddafb3d2bb2ca6ade3e35d3f895869b`.

## Why this fixture exists

Before Story 2.6, folio8 drew exactly one sheet of paper no matter how much there was to draw.
Content taller than the space between the running header and the running footer was still
**drawn** — simply past the bottom edge of the sheet, where no printer and no viewer will ever
show it. Nothing warned and nothing errored, and the file was perfectly well-formed.

Measured on a document of this shape at the story's baseline: **1 `/Type /Page` object,
`/Count 1`, and 72 of 122 text placements at a NEGATIVE PDF Y — the most negative at
−1163.874 pt on an 841.89 pt sheet.** Roughly three-fifths of the text was invisible ink.

**No existing fixture could express that.** Measured across all seven pre-2.6 fixtures by
laying each one out with the production functions and comparing its lowest content bottom
against its derived content height:

| property | fixtures having it |
|---|---|
| more than one page of content | **0 of 7** |
| a **populated** `pageHeader` band | 1 of 7 (`three-band-page`) |
| a **populated** `pageFooter` band | 2 of 7 (`three-band-page`, `font-text`) |

The tightest of them, `wrapped-text`, uses **37.7%** of one content band, and none is within
454 pt of the page boundary. An assertion that overflowing content produces a second page is
**vacuous** over every one of them — and vacuous *invisibly*, because the assertion itself
reads as sound.

## What this document's content CAN express

- **It paginates.** Its single content element wraps to **29 lines** at 24 pt on the Noto Sans
  chain, against a content window that holds **22**. Lines 0–21 fall on page 1 and lines 22–28
  on page 2, so the element's lines **straddle a page boundary** — the case D-2.6.1's *"no line
  is ever split"* invariant is about, and the case no other fixture contains.
- **Both running bands are populated, with distinct literal strings**
  (`HEADER REPEATED ON EVERY PAGE` / `FOOTER REPEATED ON EVERY PAGE`), so a band mix-up shows
  up in the rendered **text** and not only in a coordinate — a human reading this PDF can see
  it.
- **The six geometric inputs are pairwise distinct**: `margin.top` 30, `margin.bottom` 42,
  `margin.left` 36, `margin.right` 54, `pageHeader.height` 18, `pageFooter.height` 24. No two
  are equal, so no **substitution** among them and no **swap** of a pair survives. (Story 2.5
  measured that four mutually-equal inputs made a moved page-header band invisible to *every*
  test in the repository.)
- **It contains a ligature.** The prose includes *first*, *file* and *fit*, so the shaper draws
  the `fi` ligature: one glyph whose `/ToUnicode` source text is two runes. That exercises the
  glyph-vs-rune distinction the subset is built on. The fixture was **not** rewritten to avoid
  it — distorting the subject so an existing test helper could read it would be fitting the
  subject to the instrument.

## What it CANNOT express — read this before assuming coverage

- **It is all-Latin by construction**, so it creates **no reading judgment and no human sign-off
  obligation** (`three-band-page`'s precedent). The Thai obligations belong to D-2.3.5 and
  D-2.4.3 and bind to `fixtures/shaped-text/` and `fixtures/expected-breaks/` respectively.
  **Nothing here touches either.**
- **Single-face, no image, no binding, no fallback.** It says nothing about shaping, coverage
  resolution or subsetting beyond the one ligature above.
- **No element is taller than the content window**, so it does **not** exercise the
  fits-nowhere overflow diagnostic. That case is covered in
  `folio-go/internal/layout/paginate_test.go` and `folio-go/render_overflow_test.go`, where it
  can be stated as an error assertion rather than as bytes.
- **Exactly one content element**, so it says nothing about how two absolutely-positioned
  siblings interleave across a window boundary. `internal/layout/paginate_test.go` covers that.
- **No `{{page}}` or `{{pages}}`.** Those are **reserved for Story 2.7** (D-1.6.5, AC18). Their
  pass-through is asserted on a separate in-test document precisely so that this golden is not
  re-recorded when 2.7 implements them.
- **It is not a streaming test.** NFR4 is explicit that FR39's writer API is an ergonomic shape,
  not a constant-memory guarantee.

## The pagination model it demonstrates

Ruled by **D-2.6.1** as amended. The content band is a **window onto one unbounded content
column**; a longer document is more windows, never rearranged furniture.

- Window 0 begins at the content band's top. **Window N+1 begins at the top of the first item
  that did not fit in window N.**
- The atom is the **line**, and that is **forced, not chosen**: a single text element can wrap
  past a page, so element granularity would make an element taller than a page unplaceable.
- **No line is ever split.** A line is placed on the first page whose window contains it
  entirely, measured from `baseline − max(ascent)` to `baseline + max(descent)`. Whitespace at
  the foot of a page is **correct typesetting**, not a defect — a bank statement cannot ship a
  half-line.
- **Images are atomic**, and their extent is the **declared box**.
- The column is **never mutated**, so no element is ever displaced relative to another, and a
  paragraph's leading survives a page break exactly (AD-24: *"siblings never move, because
  nothing in a band ever reflows"*).

Hand-derived for this document: content height `841890 − 30000 − 42000 − 18000 − 24000 =
727890`; a line at 24 pt is `32688` (advance) tall; `(i+1) × 32688 ≤ 727890` holds for `i ≤ 21`,
giving **22 lines on page 1 and 7 on page 2**.

## Files

| file | what it is |
|---|---|
| `input.folio` | the document. Kept **byte-identical** to `multiPageTemplateJSON` in `folio-go/multi_page_template.go`, by hand. |
| `expected.pdf` | the golden, recorded by Story 2.6. |
| `expected.json` | the golden's digest and the toolchain that produced it. |

There is **no generator** for this directory. `expected.pdf` is re-recorded only by a
deliberate, ruled act under AD-22 — *"a hash change is an intended, versioned event"* — and a
re-recording is a recording (D-000.44): it owes the same semantic acceptance step as the first.

## Provenance — external structural validation (D-000.53)

No golden artifact is accepted — first recording **or re-recording** (D-000.44) — until a reader this
project did not write parses it and resolves it into the semantic objects it claims to contain.

| | |
|---|---|
| reader | `qpdf` **12.4.0** |
| invocation | `qpdf --check fixtures/multi-page/expected.pdf` |
| result | exit **0** — `No syntax or stream encoding errors found` |
| invocation | `qpdf --show-npages fixtures/multi-page/expected.pdf` |
| result | **2** page(s), matching the declared `/Count` |
| validated at | `50ad6c8` (Story 2.6) |

**Recorded, then found BROKEN, then fixed and re-recorded.** The first recording emitted `/Kids [8 0 R10 0 R]` — no separator between the two indirect references — so `qpdf --check` reported `ERROR: file does not contain any pages` (exit 2) and a viewer showed *"page 0 of 2"*. The page objects were correct and `/Count 2` was correct; the **array pointing at them** was unparseable. Fixed in `internal/pdf`'s `appendRefArray`, which emits the array from the slice and cannot omit the separator, and re-recorded. **This is the finding that produced D-000.53.**

The external reader is the **acceptance instrument, at recording time only** — run off-leg on the
recording machine, hand-checked, output pasted here. It is never a runtime or CI dependency (AD-25,
`TestModuleGraphAllowlist`), and it is deliberately **not** gated to "the legs that have qpdf": a check
that runs on some legs and not others reproduces D-000.9's failure — an "all clear" indistinguishable
from "I could not look" — one level up, at the leg. The standing every-leg regression guard is the
in-repo checker `folio-go/golden_structural_validity_test.go`, which is hermetic and covers all four
targets including `js-wasm`.
