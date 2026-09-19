# Fixture: font-text

This fixture is Story 1.5's golden record — the first PDF `folio-go` produces with real,
embedded, subsetted text: an A4 page whose content band renders `"Hello, World!"` and whose page
footer renders `"Page footer 0123456789"`, both in a subset of the committed Latin test face
(`folio-go/testdata/fonts/Roboto-Regular.ttf`, AC26), embedded as a `Type0`/`Identity-H` composite
font with a `FontFile2` stream and a `ToUnicode` CMap (AC5).

It sits **beside** `fixtures/minimal-rect/`, not in place of it (AC14a, D-1.5.9):
`minimal-rect/` is retained unchanged and reclassified as pinning `internal/pdf`'s **fontless**
emission path; this fixture is the first covering the **font-embedding** path through the public
`Render` API.

## Contents

- `input.folio` — the `.folio` document rendered to produce this fixture. `folio-go/fixture_test.go`
  renders the `fontTestTemplateJSON` constant directly, not this file — exactly as `minimal-rect/`'s
  fixture test renders `internal/pdf.Serialize()` directly rather than reading a file — but
  `TestRenderMatchesFontTextGoldenFixture` now asserts this file is byte-identical to that constant
  before rendering (Finding 14, this story's QA review), so the two can no longer drift apart
  silently.
- `expected.json` — the normative record: SHA-256 of the rendered bytes, `folio8GoVersion`, and the
  exact Go toolchain version that produced the hash (Story 1.2 AC16, D-1.2.2: `sha256` is always a
  JSON string of exactly 64 lower-case hex characters, never a per-target map).
- `expected.pdf` — the recorded bytes, kept for human diffing only. **The hash in `expected.json`
  is normative, not this file.**

## Font input

Rendered with `FontSet{"Roboto-Regular": <bytes of folio-go/testdata/fonts/Roboto-Regular.ttf>}`.
The font bytes are NOT copied into this directory a second time — AD-26 governs one committed copy
per redistributed asset, and `folio-go/testdata/fonts/` (with its own `LICENSE-Roboto.txt` and
`NOTICE.md`) is that copy (AC25).

## Measured, at record time

- `head.created` (source face) = 3304067374, `head.modified` = 3573633780 (F-5; embedded subset
  copies these verbatim — AC11a).
- Independent-reader validation: `qpdf --check` reports no syntax/stream errors; PyMuPDF's
  `page.get_text()` on the rendered PDF returns exactly
  `"Hello, World!\nPage footer 0123456789\n"` — confirming the `ToUnicode` CMap and glyph mapping
  are correct, not merely well-formed (AC13).

## If this fixture's hash test goes red

Same rule as `minimal-rect/` (AD-21, AD-22): a hash change here is a defect until proven to be an
intended, versioned change (e.g. a `textshape` version bump, which AD-22 already classes as a
subsetting-affecting change with a moving subset tag as its correct signal — D-1.5.8). Never
regenerate this fixture just to make a failing test pass; investigate first, and re-record
deliberately as part of a documented breaking change.

## Re-recorded at Story 2.2 (AD-22 versioned change)

Story 2.2 (`D-2.2.2 (superseded)`, AC6/AC6a) re-derives the six-letter subset tag: it now hashes
the embedded font program's own returned bytes (`subset.Subset()`'s output, in full) instead of a
hash of the sorted glyph-id set alone — the previous form collided across two pinned instances of
one variable face (B6), which this story's shipped Noto faces make a live hazard for the first
time. `(*Font).Subset` is the one shared function every embedded face goes through, so this
fixture's Roboto embedding moves too, even though nothing about Roboto or this document changed.

**The delta was measured, not assumed, before re-recording**: both `expected.pdf` revisions are
**22299 bytes**; exactly **77 bytes differ, in 7 contiguous runs**; `startxref 21957` is identical.
Three of the runs are the six-letter subset tag at its three appearances (`/BaseFont` ×2 and
`/FontDescriptor /FontName`) — `OETEKT` → `HXRYNT`. The remaining four runs are the two duplicated
`/ID` array entries, which move as a *consequence*: `computeID` derives `/ID` as a SHA-256 prefix
over the serialized body up to that point (AD-7), and the tag is upstream of it. **The `FontFile2`
stream itself — the embedded, subsetted Roboto program — is byte-identical**; nothing about the
outlines, widths, or glyph mapping changed. Old hash: `dcd453a1f593c1135c44b52d732683d19419c24743344d3843826fccbafacbdf`.
New hash: recorded in `expected.json` after Story 2.2's in-story four-target matrix run confirmed
all four targets agree (D-2.2-D5's first reason for not switching `font-text`'s face is exactly
this four-target verification — re-recording from one machine would have silently retired it).

`fixture_test.go` additionally now pins the embedded `FontFile2` stream's own SHA-256 as a
permanent constant, independent of the whole-document hash — the assertion AC8's now-corrected
"bytes unchanged" clause needed to become, so a future change that moves the *program* stays
distinguishable from one that only moves the *tag*.

The "Measured, at record time" facts above (`head.created`/`head.modified`, the PyMuPDF text
extraction) did not need re-measurement: the program bytes did not move, only its tag and the
consequently-derived `/ID`.

---

## Re-recorded at Story 2.3 (AD-22 versioned change) — and this fixture is now an OBSERVING fixture

Story 2.3 put the typeface's own OpenType layout rules in charge of the render. This fixture's
bytes moved as a result, and the move **retires a regression rather than accepting one**.

### What changed, and why it is a fix

Story 2.3's F3 recorded this fixture as *blind to shaping*. It is not, and the way that
measurement misled is worth keeping on the page: **F3 measured the string `"Hello"` against a
shipped Noto face**, while this fixture renders `"Hello, World!"` and `"Page footer 0123456789"`
through `folio-go/testdata/fonts/Roboto-Regular.ttf` at `unitsPerEm` 2048 — a different string
*and* a different face. Roboto's `GPOS` kerns two pairs in that text, so the **previous bytes
recorded UNKERNED output**: two `Tj` operators, no `TJ`, 22,299 bytes. The new bytes are 22,310,
with two `TJ` arrays and zero `Tj`.

| run | glyph | `hmtx` advance | shaped advance | delta (font units) | TJ adjustment (1000-em) |
|---|---|---|---|---|---|
| `Hello, World!` | `W`, gid 59, index 7 | 1817 | **1786** | −31 | **15** |
| `Page footer 0123456789` | `P`, gid 52, index 0 | 1292 | **1281** | −11 | **6** |

Scaling is `geom.ScaleRound` to the PDF's 1000-unit em: `887 − 872 = 15` and `631 − 625 = 6`.

### Provenance: cross-checked against the reference implementation before re-recording

The kerning was confirmed against **HarfBuzz**, not against a second port of the same lineage — a
**one-time offline reference run, hand-checked, never a build, test or runtime dependency**
(AD-25; the same precedent Story 1.1 set with `qpdf --check`). The module graph is unchanged and
`TestModuleGraphAllowlist` is untouched.

**Tool version:** `hb-shape (HarfBuzz) 14.2.0`

**Exact invocations, verbatim, from the repository root:**

```
hb-shape --no-glyph-names folio-go/testdata/fonts/Roboto-Regular.ttf "Hello, World!"
[44=0+1460|73=1+1085|80=2+497|80=3+497|83=4+1168|16=5+402|4=6+507|59=7+1786|83=8+1168|86=9+693|80=10+497|72=11+1155|5=12+527]

hb-shape --no-glyph-names folio-go/testdata/fonts/Roboto-Regular.ttf "Page footer 0123456789"
[52=0+1281|69=1+1114|75=2+1149|73=3+1085|4=4+507|74=5+711|83=6+1168|83=7+1168|88=8+669|73=9+1085|86=10+693|4=11+507|20=12+1150|21=13+1150|22=14+1150|23=15+1150|24=16+1150|25=17+1150|26=18+1150|27=19+1150|28=20+1150|29=21+1150]
```

HarfBuzz reports `1786` for `W` and `1281` for `P` — the kerned advances — agreeing with
`textshape` value for value.

### This fixture has moved lists

`fixtures/font-text/` is no longer blind to shaping. It **observes** it, and its guard says so in
values rather than only in a digest: `TestRenderMatchesFontTextGoldenFixture` asserts the operator
is `TJ` (never a bare `Tj`) **and asserts the adjustments `15` and `6` by value**.

That is deliberate, and it is the part to preserve if this fixture is ever touched again. A change
that silently loses kerning produces a **hash** mismatch, and the cheapest available response to a
hash mismatch is to re-record it. Naming the expected adjustments makes re-recording a kerning
regression impossible without **deleting an assertion** — which is visible in a diff, where a
changed digest is not (D-000.22).

The fixtures that genuinely **cannot** observe Story 2.3 are `minimal-rect/` (fontless),
`image-embed/` (no text) and `multi-script-fallback/` (measured: `"Ada ก 汉"` shapes to itself on
all three shipped faces). All three are byte-identical across this story, and
`TestShapedTextFixtureObservabilityRedProof` asserts `multi-script-fallback/`'s blindness directly
— it is the red-proof that gives `fixtures/shaped-text/`'s observability claim its meaning.

### If this hash goes red

Unchanged from above: under AD-21/AD-22 it is a **defect until proven to be an intended, versioned
change**. Do not regenerate the fixture to make a test pass.

## Provenance — external structural validation (D-000.53)

No golden artifact is accepted — first recording **or re-recording** (D-000.44) — until a reader this
project did not write parses it and resolves it into the semantic objects it claims to contain.

| | |
|---|---|
| reader | `qpdf` **12.4.0** |
| invocation | `qpdf --check fixtures/font-text/expected.pdf` |
| result | exit **0** — `No syntax or stream encoding errors found` |
| invocation | `qpdf --show-npages fixtures/font-text/expected.pdf` |
| result | **1** page(s), matching the declared `/Count` |
| validated at | `50ad6c8` (Story 2.6) |

**Settled**, validated unchanged at `50ad6c8`. This artifact predates D-000.53 and was validated retroactively; it is recorded here so the row ends settled rather than carried (D-000.29).

The external reader is the **acceptance instrument, at recording time only** — run off-leg on the
recording machine, hand-checked, output pasted here. It is never a runtime or CI dependency (AD-25,
`TestModuleGraphAllowlist`), and it is deliberately **not** gated to "the legs that have qpdf": a check
that runs on some legs and not others reproduces D-000.9's failure — an "all clear" indistinguishable
from "I could not look" — one level up, at the leg. The standing every-leg regression guard is the
in-repo checker `folio-go/golden_structural_validity_test.go`, which is hermetic and covers all four
targets including `js-wasm`.
