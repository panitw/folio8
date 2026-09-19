# `fixtures/keep-together/` — a signature block that will not be severed (Story 7.7, FR51)

This fixture exists to **discriminate**, not to demonstrate.

A contract ends with a signature block: a name, a ruled line, a date. Those few pieces belong
together. Before this story the paginator's only unit of atomicity was the individual line, image or
table row, so a page boundary falling inside that block printed the name at the foot of one page and
the date at the head of the next. FR51 lets a template author say *these pieces travel together*.

The document here is authored so that its signature block lands **astride the first content
window's ceiling**. That is the whole point: the same document with its three `keepTogether` tags
and without them paginates **differently**, and the pair is what makes the feature falsifiable.

## The arithmetic

Band-relative, because an element's `y` is relative to its band's top-left corner (AD-24). A4 is
841.890 pt tall, margins are 36 top and bottom, and the page header and page footer each declare 20,
so the content window is

```
841.890 − 36 − 36 − 20 − 20 = 729.890 pt
```

and a content-band item is in window one exactly while its **bottom** is at or above 729.890. At
font size 11 over the `["Noto Sans"]` chain one line occupies one line advance —
(1069 + 293 + 0) / 1000 × 11 = **14.982 pt**, from that face's hhea ascent 1069, descent −293 and
lineGap 0 (the same 1362/1000 em Story 2.5a records for this chain).

Every extent below is **measured from the built column items**, band-relative, not computed by hand,
and each one reconciles with that per-line figure — `e1`'s 37 lines are 37 × 14.982 = 554.334, and
each single-line element's extent is its declared `y` plus 14.982.

| element | what it is | extent | window one? |
|---|---|---|---|
| `e1` | the body text, 37 lines | 0.000 … 554.334 | in — and nowhere near the boundary |
| `e2` | "Signed for the Company" | 706.000 … 720.982 | **in** |
| `e3` | the ruled line (a `rect`, height 1) | 734.000 … 735.000 | **out** |
| `e4` | "Date: 31 August 2026" | 740.000 … 754.982 | **out** |

**Untagged, that is a severed signature block** — the name prints at the foot of page 1, the rule and
the date at the head of page 2. **Tagged**, the union extent is 706.000 … 754.982: 48.982 pt, far
short of a whole window, so the group is not over-tall; it does not fit window one, so the window
slides to the group's *earliest* top and all three members move to page 2, each at its own declared
position. The body text does not move. No gap is invented and no page is left empty.

This table is the **single copy** of the fixture's arithmetic. `folio-go/keep_together_template.go`
used to restate it beside the template const, and the two drifted apart — and away from the
fixture — so that doc comment now points here instead of repeating the numbers.

## The twin

`folio-go/keep_together_template.go` ships this document as `keepTogetherTemplateJSON`,
byte-identical to `input.folio`, **and a second const identical except that the three tags are
absent**. `TestKeepTogetherTwinDiffersOnlyByTheTags` asserts mechanically that the pair differs in
exactly that one respect, so "the two renders differ" cannot quietly become evidence about something
else. Every assertion in `keep_together_fixture_test.go` is about the *difference* between the two,
because "the block is on page 2" is a fact many unrelated implementations produce and "on page 2
here and split there" is not.

## What the canvas does with it

The canvas **previews grouping** (Story 7.9). It tags its own column items from the same
`keepTogetherTags` index the render path uses — grouping takes the template and nothing else, so the
canvas holds every input it needs — and its projected window **count** and window **origins**
therefore equal the render path's for this document. On the pair above that difference is visible:
window two begins at the group's earliest top, **706.000 pt** (706000 millipoints, which is what the
projection carries and what the tests assert), and not at **734.000 pt**, where the untagged twin's
ruled line falls out of window one.

`folio-go/canvas_window_count_test.go`'s
`TestCanvasWindowsAgreeWithTheRenderPathForAGroupedDocument` asserts that equality against a real
pagination of this very document, rather than against a flag.

Grouping is **not** among the causes that make `contentWindowCountIsExact` false, and it never
becomes one. Three of those causes are things the canvas **genuinely cannot know without the data**:
a content-band table with a non-empty binding, a text element whose font chain would not resolve, and
an element whose visibility depends on data. The fourth is not about data at all — a pagination that
degraded is a shortfall the canvas **observed in its own answer**, and reports rather than hides. A
declared group is neither kind: `keepTogetherTags` takes the template and nothing else, so getting
grouping wrong is a defect to fix rather than a shortfall to disclose. What keeps that true for
*this* fixture is `parse_bands.go`'s refusal of `keepTogether` on a **table**, which stops a group
inheriting a table's data dependency.

One thing the canvas does **not** place, and neither does the printed document: a `rect` or a `line`
declaring neither a background nor a border. `e3` here declares `"background": "#000000"`, so it is
a real member of the group on both sides. An unstyled one prints nothing, contributes no column item
on the render path, and — since Story 7.9 — contributes none on the canvas either.

## What it does not cover

Authoring a group **in the designer**. Epic 7 makes a group *declarable in the file*, which is all
FR51 asks; no inspector control views, sets or clears a tag. That is a stated scope boundary. Its
one consequence the engine does handle: duplicating a tagged component drops the tag, so a copy
never joins a group the author has no way to see or remove.

## The recorded artifact

`expected.pdf` — 82,825 bytes, sha256
`6ed495b4c22d7473d82c536c40dce8ca6f2a2fa4bf38efff44b2207929137640` — is the golden. It is recorded
in `expected.json` beside it and in `folio-go/byte_neutrality_test.go`'s `goldenDigestRecord`, whose
completeness half fails on any *undeclared* occurrence of that digest, which is why this README's
mention of it is a declared site rather than a stray quotation.

Under AD-21/AD-22 a mismatch here is a **defect** until it is proven to be an intended, versioned
change. Do not regenerate the fixture to make a test pass.
