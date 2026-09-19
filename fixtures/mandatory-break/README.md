# `fixtures/mandatory-break/` — a break the author typed (Story 7.1, FR46)

The golden for **mandatory line breaks**. Measured at this story's baseline, **not one committed
fixture's text or bound data contained a line feed** — so no recorded byte in this repository could
tell a build that honours a typed break from one that silently eats it. AC6 ("every existing golden
hashes identically") is only falsifiable alongside a document that *does* carry one, and this is it.

## What each element is for

| Element | Value | Box | Full text measures | Lines | What it proves |
|---|---|---|---|---|---|
| `e1` | `"Yours\nsincerely"` (literal) | 200 pt | 74,305 mp | 2 | the break is taken **regardless of remaining width** |
| `e2` | `"Clause 1.\n\nClause 2."` (literal) | 200 pt | 92,664 mp | 3 | a **paragraph gap**: the middle line is empty and occupies one full `Advance` |
| `e3` | `{{customer.note}}` = `"first\nsecond word"` (**bound**, declared) | **40 pt** | 86,625 mp | 2 | **a typed break survives a declared-unbreakable value** |

## `e1` and `e2` FIT their boxes — do not "fix" that

This is the exact opposite of `fixtures/wrapped-text/`'s premise, where every element had to be
*wider* than its box or "it wrapped" was vacuous. Here a value too wide for its box would break for
want of room and would say nothing about a break the author typed.

`packLines` short-circuits on "does everything that is left fit?", bypassing the opportunity list
entirely. A fixture whose text overflowed would never reach that branch — and that branch is exactly
where FR46's "regardless of how much width remained" is won or lost. Widening the text, or narrowing
these boxes, silently turns both elements into ordinary wrapping cases.

## `e3` is the discriminating element, and it discriminates in **two** directions

`e3` binds `customer.note`, which the document declares in `unbreakableValues`. Its value carries
**both** a line feed and a space, inside **one** atomic span:

- **the line feed must survive.** Flip the kind test at `internal/text/opportunity.go`'s atomic-span
  filter site and the break is suppressed with everything else: `e3` becomes **one** line.
- **the space must not.** Remove the declaration instead and the packer breaks there too: `e3`
  becomes **three** lines (`first` / `second` / `word` — `"second word"` measures 66,220 mp against a
  40,000 mp box, `"second"` alone 36,971).

Either assertion alone cannot tell *"the exemption works"* from *"span suppression broke"*.
`TestMandatoryBreakDeclarationIsLoadBearing` asserts both, and the two failures name different
things.

**The line feed arrives through DATA, and that is load-bearing.** `bind.Substitution`'s rune span
brackets only what a placeholder substituted, so a line feed typed into an element's own *literal*
text is never inside an atomic span at all — a fixture that put it there would pass vacuously
(D-7.1.1). `e1` and `e2` do carry literal line feeds; they make no claim about spans.

## The empty line, and how it is observable at all

An empty line draws nothing, so `e2`'s three lines produce only **two** baselines in the artifact.
What the produced bytes carry is the *interval*: `e2`'s two drawn baselines sit **two** advances
apart where `e1`'s and `e3`'s sit one. That gap is the only artifact-level evidence that a line
nobody drew still occupies its full `Advance`, and it is what `TestMandatoryBreakSemanticAcceptance`
asserts.

## The vertical model, hand-derived

Chain `["Noto Sans"]` at 11 pt; A4, margins 36 pt, `pageHeader` height 20 pt → content origin 20,000.

| span | rule | this fixture |
|---|---|---|
| top → first baseline | `max(A)` | `1069/1000 em` → **11,759 mp** |
| baseline → baseline | `max(A) + max(D) + max(gap)` | `1069 + 293 + 0 = 1362` → **14,982 mp** |
| last baseline → bottom | `max(D)` | `293/1000 em` → **3,223 mp** |

`pdfY` of an element's first baseline is `841890 − 36000 − (20000 + y) − 11759 = 774131 − y`, so the
drawn baselines are `774131, 759149` (`e1`), `714131, 684167` (`e2` — **two** advances), and
`634131, 619149` (`e3`). Every one of those numbers is asserted as a literal.

## Rules

- **A hash change here is a defect until proven otherwise** (AD-21/AD-22). Do not regenerate to make
  a test pass.
- **`e3`'s overflow is the EXISTING clip-and-warn path**, not a new one: `TEXT_CLIPPED_WIDTH`, a
  `Warning` beside the bytes, never fatal, never reflowed (FR44, AC11). No diagnostic code was minted
  for this story.
- **`TEXT_MISSING_GLYPH` does not fire here, and that green is not glyph coverage.** No face covers
  U+000A and none is expected to — from Story 7.1 the breaker consumes it. This is the first document
  under `TestCorpusFixturesProduceNoMissingGlyphWarnings` to pass for a reason unrelated to coverage.
- `input.folio` is kept byte-identical to `folio-go/mandatoryBreakTemplateJSON` by hand, the same way
  `font-text`, `multi-script-fallback` and `wrapped-text` are.

## Recorded

```
sha256  7cf743deb8b9c6c300f31acd304b49de625def36a5b7d3e5e73d815336141f1d
bytes   56,681
go      go1.26.0
```
