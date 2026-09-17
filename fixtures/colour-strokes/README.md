# `fixtures/colour-strokes/` — text colour and coloured strokes

The golden for **DW-147**, landed before the `folio8-go/v1.0.0` tag (SPEC-client-libraries story 3).
Until it, no committed fixture declared a text `color` or a stroke colour, so the four-target
byte-identity check had never rendered coloured ink or coloured strokes.

| File | What it is |
|---|---|
| `input.folio` | A canonical one-page A4 document declaring format version `3.1`. Every colour differs from `#000000`. **e1**, a heading, has `style.color` `#1B2A4A` on a filled `style.background` `#FFF4D6`, and a `style.border` in `#C81E1E` whose `edges` are `bottom` and `left` only. **e2** is a line of text in `#6A1B9A`. **e3**, a `rect`, is stroked `#2E7D32`; **e4**, a `line`, is stroked `#1565C0` on its top edge. **e5** is a proportional table: cell ink `#37474F`, a frame border in `#00838F`, a `headerStyle` with ink `#FFFFFF` on `#1B2A4A`, `rules` between columns and rows in `#8E24AA`, and `altRowBackground` `#E3F2FD`. |
| `data.json` | Six **synthetic** table rows, each naming an item, one of the page's colours and a quantity |
| `expected.pdf` | The render, sha256 `3e88b304a1660bea2e8d8958a35037ded436e2bea37fae9d8f0d2f6309b60d8c` |

## What the tests prove

- `TestColourStrokesGoldenFixture` pins `input.folio` and `data.json` to the Go constants in
  `colour_strokes_template.go`, checks that `input.folio` saves back byte-for-byte, and pins the
  render's sha256 to `expected.json` and `expected.pdf`.
- `TestColourStrokesSemanticAcceptance` checks the page model: the heading box is filled and stroked
  on its bottom and left edges only, each run carries its declared ink, the rect, the line and the
  table frame are each stroked once in their colour, every cell of the second, fourth and sixth rows carries the alternate fill, and the
  rules are drawn. It also reads the PDF: every declared fill and ink is set by an `rg` operator,
  every stroke by an `RG` operator, and no colour operator sets black.
- `TestColourStrokesRendersIdenticallyInAFreshProcess` renders it again in a fresh process.
- The four-target hash matrix renders it on darwin/arm64, linux/amd64, linux/arm64 and js/wasm.
- `TestColourStrokesSemanticSignOffIsRecorded` (matrix tag) is the human sign-off gate. It fails
  until the owner has examined `expected.pdf` and a `signoff.json` bound to this digest exists.

## Manual check (the sign-off)

Open `expected.pdf`. The heading must be dark navy on a pale cream band, with a red stroke along the
band's bottom and left edges and none on its top or right. The note under it must be purple. The box
must be outlined in green and the line must be blue. The table must have a navy header row with white
labels, dark grey cell text, purple lines between columns and rows, a teal outer frame, and a pale
blue fill on the second, fourth and sixth rows.
