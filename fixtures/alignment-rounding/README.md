# `fixtures/alignment-rounding/` — the branches that halve a slack (Story 7.3, closing DW-24)

This fixture exists to close a hole, not to show a feature.

Measured across every `fixtures/*/input.folio` at this story's baseline: **16 `"align": "left"`,
8 `"align": "right"`, zero `"center"`, and zero `valign` of any value.** So of the alignment
feature's branches, the golden corpus exercised the two that *cannot* round — `left` returns zero and
`right` returns the slack unchanged — and none of the ones that halve a slack with
`geom.ScaleRound(slack, 1, 2)`. Every recorded byte in the repository was compatible with a build
that broke the half-to-even tie differently on a different target, which is exactly what AD-2/AD-3
exist to prevent and what the four-target matrix exists to catch. Seven stories in Epic 7 and 8 bind
themselves to "the corpus hashes identically"; for the rounded branches that criterion was
unfalsifiable.

## One document, every site the re-derived enumeration returns

A centred *text element* does not cover a table's cells: those round in different code, in
`table_render.go`. The enumeration is re-derived by grep at the closing revision and recorded in
DW-24's closing note, never read off the entry's hand-list — which had rotted twice.

| Element | What it declares | Site it reaches |
|---|---|---|
| `e1` | `align: "center"` | `text_alignment.go`'s `textAlignOffset` |
| `e2` | `valign: "middle"`, `height: 40.001` | `text_alignment.go`'s `textValignOffset` |
| `e3` | `valign: "bottom"` | unrounded — but equally undeclared by the corpus until now |
| `e4` header | column `align: "center"`, `headerStyle.valign: "middle"`, `headerHeight: 24.001` | the table **header** cell's horizontal *and* vertical rounds |
| `e4` body | the same centred column | the table **body** cell's horizontal round |
| `e4` footer | the same column's `count` footer | the table **footer** cell's horizontal round |
| `e4` rows | table `style.valign: "middle"` | the integer **line-slot** split — a row's spare line slots, not a millipoint round |

## Every slack is odd, and that is the whole mechanism

`geom.ScaleRound(slack, 1, 2)` and a truncating `slack / 2` **agree on every even slack.** A centred
fixture whose boxes happened to leave even slack would satisfy DW-24's literal text while detecting
none of the mutations its own falsifier runs, and nothing else in the repository would notice.

Every slack here is therefore odd — so the exact-half tie is genuinely taken — and, further, every
one of them is `≡ 3 (mod 4)`, so the tie rounds **up** to even and the result differs from truncation.
Two boxes are declared in thousandths of a point (`height: 40.001`, `headerHeight: 24.001`) and the
centred column is `60.003` pt wide precisely to make that true. `TestAlignmentRoundingSlacksAreOdd`
asserts it rather than leaving it to luck: an even slack silently avoids the tie the fixture exists
to take.

| Site | Slack | Halves to |
|---|---|---|
| `e1` centred, 200 pt box less the shaped `Centred` | 158,783 mp | 79,392 (truncation gives 79,391) |
| `e2` middle, 40.001 pt box less a 14,982 mp block | 25,019 mp | 12,510 |
| header `Qty` centred in 60.003 pt | 41,831 mp | 20,916 |
| header middle, 24.001 pt less a 14,982 mp block | 9,019 mp | 4,510 |
| body `3` / `7` centred | 53,711 mp | 26,856 |
| body `12` centred | 47,419 mp | 23,710 |
| footer count `3` centred | 53,711 mp | 26,856 |

The two qty values shape to different widths on purpose, so the centred body cell rounds *two*
different odd slacks rather than the same one three times.

## The integer line-slot split

The first clause wraps to **four** lines in its 140 pt column while its qty cell is one line, so that
row has three spare line slots. Under `valign: "middle"` a body cell takes slot `3 / 2 = 1` — neither
the first nor the last, which is what makes the assertion discriminating. It is an integer *line
count*, not a `geom.ScaleRound`, and therefore not a cross-target float hazard; it is here because it
is a `middle`-only branch with zero golden coverage, which is the absence DW-24 exists to record.

## It declares `1.0`, and keeps declaring it

This document uses no 1.1 key and no 2.0 value, so 1.0 is the lowest version its content requires.
Beside `fixtures/line-spacing/`'s 1.1 and `fixtures/justified-text/`'s 2.0, it is the corpus's
witness that the format's three live versions coexist and that neither the library's ceiling nor a
sibling fixture's version leaks into a document requiring neither.

## Files

| File | What it is |
|---|---|
| `input.folio` | the template, kept byte-identical to `folio-go/alignmentRoundingTemplateJSON` by hand |
| `expected.json` | the recorded digest and the toolchain it was recorded under |
| `expected.pdf` | the recorded artifact |

Recorded digest: `986400a1c8bb1ff84d868bb8df70479c5e7e7a2ad5e867634efb810a47327087`.
