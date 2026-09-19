# `fixtures/line-spacing/` — the space between a paragraph's lines (Story 7.2)

The golden for **`style.lineSpacing`**. Measured at this story's baseline, **no committed fixture
declared a line spacing at all** — the key did not exist — so no recorded byte in this repository
could tell a build that honours an author's leading from one that silently ignores it. The story's
byte-neutrality guard ("every existing golden hashes identically") is only falsifiable alongside a
document that *does* set one, and this is it.

**It is also the first committed document declaring `"version": "1.1"`.** That is not decoration.
Under D-1.4.13 a document declares the lowest version its own *content* requires; this one requires
1.1 because it sets `lineSpacing`. A fixture declaring 1.0 while using a 1.1 key would be the exact
misdeclaration D-7.2.1 exists to end.

## What each element is for

| Element | `lineSpacing` | Lines | Advance | What it proves |
|---|---|---|---|---|
| `e1` | **absent** | 2 | **14,982 mp** (the ruled advance) | THE CONTROL — a build that scaled the wrong thing, or scaled everything, disagrees with it first |
| `e2` | `1.5` | 2 | **22,473 mp** | `Advance` scales — and `e2`'s FIRST baseline sits exactly where `e1`'s rule puts it |
| `e3` | `0.6` | 3 | **8,989 mp** | **tight leading**: the advance is *below* the 11,759 mp first-baseline offset, so the line boxes overlap |
| `e4` | `1.5` on `style`, `0.6` on `headerStyle` | header 1, rows 2 and 1 | body **22,473 mp** | the cascade reaches a table — **every caller, no carve-out** (D-7.1.3) |

## `e1` is the control, and it is what makes the rest mean something

Every other element's advance is stated as a literal, but a literal alone cannot tell "the ratio was
applied" from "the whole model moved". `e1` declares no spacing, so its two baselines must stay
exactly one *ruled* advance apart — the number this repository has recorded since Story 2.4. Remove
it and a build that multiplied every element's advance by 1.5 would still satisfy every remaining
assertion.

## `e3` is the geometry the canvas used to refuse

`lineSpacing: 0.6` gives an advance of **8,989 mp** against a first-baseline offset of **11,759 mp**.
One line's baseline therefore sits *below* the next line's top: the line boxes overlap. **That is
what tight leading is**, and it is what the page draws. `folio-designer`'s `isTextPaint` carried a
clause forbidding exactly this shape (`paint.baseline > paint.top + paint.advance`), which failed one
line, then `isCanvas`, then `isSnapshot`, and blanked the **whole** projection. D-7.2.2 deleted it:
the browser was refusing the engine's own honest measurement, which is AD-17 inverted rather than
enforced. Three lines rather than two, so the overlap is interior and not merely terminal.

## `e4`'s header sets a different ratio, and observes nothing — deliberately

`headerStyle.lineSpacing` is `0.6` while `style.lineSpacing` is `1.5`, and the header row's
placement is **unchanged** by either: a header label is one line, and that construction site reads
`FirstBaseline` and `LastDescent` only, never `Advance`. It is here because the *cascade* must reach
it — `headerStyle` wins over `style`, and `headerStyle` inherits `fontFamily` from `style` in the
same resolution — and because a key silently dropped on one of the four construction sites is
exactly what "every caller, no carve-out" forbids. The body rows are where the ratio becomes visible:
the first clause's bound value carries a line feed (Story 7.1), so that row is two lines and its
height is a function of the scaled advance.

## The vertical model, hand-derived

Chain `["Noto Sans"]` at 11 pt; A4, margins 36 pt, `pageHeader` height 20 pt → content origin 20,000.

| span | rule | this fixture |
|---|---|---|
| top → first baseline | `max(A)` | `1069/1000 em` → **11,759 mp** — **NOT scaled by `lineSpacing`** |
| baseline → baseline | `(max(A) + max(D) + max(gap)) × lineSpacing` | `1362 × 11 = 14,982` → ×1.5 = **22,473**, ×0.6 = **8,989** |
| last baseline → bottom | `max(D)` | `293/1000 em` → **3,223 mp** — **NOT scaled by `lineSpacing`** |

`pdfY` of an element's first baseline is `841890 − 36000 − (20000 + y) − 11759 = 774131 − y`, so the
drawn baselines are `774131, 759149` (`e1`), `714131, 691658` (`e2`), `634131, 625142, 616153`
(`e3`), `534131` (`e4`'s header), `514131, 491658` (row 1, two lines) and `476676` (row 2). Every one
of those numbers is asserted as a literal.

**`FirstBaseline` and `LastDescent` are untouched, and that is the point.** `lineSpacing` scales the
`Advance` and nothing else, so a multi-line element's top edge does not move and no neighbour
appears to jump — the two-model split Story 2.5a exists to establish (D-2.5a / DW-15). `e2`'s and
`e3`'s first baselines sit at `774131 − y` exactly as `e1`'s does; only the intervals below them
differ.

## Rules

- **A hash change here is a defect until proven otherwise** (AD-21/AD-22). Do not regenerate to make
  a test pass.
- **This document produces no diagnostics at all.** Nothing overflows its box; `lineSpacing` never
  warns — an out-of-range value is a *load* error carrying `TEMPLATE_FIELD_INVALID`, so it never
  reaches a render.
- **Do not add this fixture to `baselineAcceptanceFixtures`.** That list is Story 2.5a's record of
  the five goldens *that story* re-recorded, and is hard-pinned to exactly five.
- `input.folio` is kept byte-identical to `folio-go/lineSpacingTemplateJSON` by hand, the same way
  `font-text`, `multi-script-fallback`, `wrapped-text` and `mandatory-break` are.

## Recorded

```
sha256  de2121156d8c58e93a0c8b6032f338f4c24886145488aad248bc775fc83ee290
bytes   57,770
go      go1.26.0
```
