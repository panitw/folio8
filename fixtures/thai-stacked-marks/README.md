# `fixtures/thai-stacked-marks/` — a Thai mark the shaper lifts off the baseline (Story 8.0)

The golden for **a glyph the shaper gives a non-zero `YOffset`**, expressed in the PDF with the
text-rise operator `Ts`.

It is the corpus's **twenty-second** document and the **first** that could ever hold such a glyph.
Until this story `internal/pdf`'s `appendShapedRun` refused one outright — a hard `Render` error and
**zero bytes** — so a fixture containing one would have had nothing to record. That is why the first
twenty-one goldens go without one, and it is why "ordinary Thai does not render" was found by the
owner pasting a real contract into the shipped designer rather than by a test (DW-28, HIGH).

## The trigger is a non-zero `YOffset`, **not** mark stacking

`ที่`, `ป้ำ` and `ปั` each stack two marks over one base and have rendered since Story 2.3, because
Noto Sans Thai resolves those pairs with a **GSUB lowered-form substitution at zero offset**. Only
`ั` followed by a tone mark is resolved by a **GPOS y-displacement**, and only a displacement had
nowhere to go. `ที่` appears in `fixtures/shaped-text`, in all four `statement-*` fixtures and in
`fixtures/justified-thai` — so a reader who takes *"two stacked marks"* as the predicate will read
those goldens as contradicting this one. They do not.

## What each element is for

| Element | Value | What it proves |
|---|---|---|
| `e1` | the owner's contractor-liability clause, verbatim | the subject: five lines of ordinary Thai legal prose whose runs carry **three distinct text rises** |
| `e2` | `สัญญา` ("contract") | THE CONTROL — same script, same chain, same 12 pt size, same 400 pt box, and **not one glyph with a vertical offset**. Its run emits **no `Ts` at all**: byte-for-byte what this renderer emitted before Story 8.0. |

`e1` is not constructed to trip anything. It is the clause the owner pasted into the shipped
designer, which the canvas drew correctly and the PDF stage then refused with
`Render failure · ENGINE_REJECTED`. Ordinary Thai legal prose contains `ทั้งสิ้น`, and `ครั้ง`,
`ทั้งนี้` and `ตั้งแต่` elsewhere.

## The three rises, measured

`Ts` sets the **text rise**, a persistent text-state parameter in text space, y-up — the same
convention and the same sign the shaper's `YOffset` already carries, which is why nothing negates it
anywhere between the shaper and the operand. The rise is

    geom.ScaleRound(run.FontSize, g.YOffset, 1000)

in integer millipoints, round-half-to-even, with no float intermediate (AD-23) — which is what makes
all four AD-21 targets agree on it.

| `YOffset` (thousandths of an em) | Font size | Rise emitted | Where |
|---|---|---|---|
| −2 | 12 pt | `-0.024 Ts` | line 0 |
| −57 | 12 pt | `-0.684 Ts` | line 4 |
| −59 | 12 pt | `-0.708 Ts` | line 4 |

The last two land on the **same CID**, one after the other. A build that cached the rise per glyph
id, or that hoisted one rise to the whole run, would place the second mark wrongly and this fixture
would say so.

## The rule the bytes pin

`Ts` is text state, not a per-glyph parameter, so a run is **partitioned into maximal contiguous
segments of equal rise**, walked in index order, and each segment gets its own show-text operator.
The rise in effect is `0` on entry to every run; a segment whose rise differs emits `<rise> Ts`
first; and after the last segment a non-zero rise is restored to `0 Ts`.

The restore is **not optional and not provided by anything else**. `buildTextContentStream` brackets
a run in `q`/`Q` only when it is clipped or coloured, and text rise **survives `ET`** regardless — so
a run that walked away with the rise still set would displace whatever drew next. Every `Ts` in this
file is asserted to be restored before its own `ET`.

`e1`'s fourth line is where the whole rule is visible in one run:

```
[…<0011>] TJ
-0.684 Ts
[-30<0015>] TJ
0 Ts
[27<0019005a>-7<0027>] TJ
-0.708 Ts
<0015> Tj
0 Ts
[7<001c>] TJ
```

Five segments; the adjustment term that sits *before* a glyph leads the segment that glyph opens
(`Ts` moves no pen, so a term crossing a segment boundary changes nothing about where the glyphs
land); and the segment at rise `-0.708` has no non-zero term of its own, so it takes the `Tj` fast
path exactly as a whole zero-offset run does.

## Why the corpus's other twenty-one goldens are untouched

The `Ts` path is entered **only** when a glyph's `YOffset != 0`. A run whose every glyph carries
zero offset is **one segment at rise zero**: no `Ts`, no restore, and the exact bytes the renderer
emitted before this story. That is asserted rather than assumed, in three places — `e2` here,
`TestZeroOffsetRunIsByteIdenticalToItsRisenTwin` (which deletes the rise operators from a positive
stream and demands byte-equality with the negative one), and
`TestNoPreStory80GoldenCarriesATextRise`, which scans every other committed `expected.pdf` for the
substring ` Ts` (content streams are uncompressed, so the scan is valid).

## The refusal that is left

The fail-closed branch **narrowed**; it did not vanish. With `Ts` in place all four `ShapedGlyph`
fields are expressible, and the one condition left is the **rounding boundary**: an offset the
shaper really asked for whose rise scales to zero. `fontSize` has no positivity floor at parse, so a
stacked-mark document at `fontSize: 0.008` reaches the emitter with
`ScaleRound(8, -57, 1000) == 0`. Emitting `0 Ts` there would drop the offset silently — the healthy
output and the broken output would be the same bytes — so it still refuses, with zero bytes and a
message pinned verbatim by `folio-go/thai_mark_stacking_test.go`. At any `fontSize` of 1 pt or more,
`|rise| >= |YOffset|` millipoints and is never zero.

## Files

| File | What it is |
|---|---|
| `input.folio` | the template, kept byte-identical to `folio-go/thaiStackedMarksTemplateJSON` by hand |
| `expected.json` | the recorded digest and the toolchain it was recorded under |
| `expected.pdf` | the recorded artifact |

The data is the empty object: what this document witnesses is a property of the shaper's output for
a literal string, and bound data would only add a way for it to move for reasons that are not its
subject.

## Not signed off

**There is no human reading sign-off for this document's mark placement, and no agent may write
one.** `fixtures/shaped-text/thai-signoff.json` (D-2.3.5) attests GSUB lowered-form placement at zero
offset — a different mechanism, and not evidence about a rise. What is attested here is that the
marks are placed at **the offset the shaper computed**; whether that reads correctly to a Thai reader
is a judgment only the owner can commission.
