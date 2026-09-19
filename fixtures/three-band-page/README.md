# fixtures/three-band-page/ — band composition, all three bands populated (Story 2.5)

A folio8 report is three stacked strips of the page: a page header at the top, a page footer at the
bottom, and the content region between them. This is the document that makes a mistake in that
composition **visible in the bytes**.

## Why it exists — and it is a measurement, not a preference

At Story 2.5's creation each band's origin was moved by one line in turn and the whole suite was run:

| band origin moved | detected by |
|---|---|
| **page header** | **ZERO tests** |
| content | 5 golden tests |
| page footer | 1 test |

**The cause was the subject, not the assertions.** Enumerated over all six recorded fixtures:

| fixture | `pageHeader` elements | `content` elements | `pageFooter` elements |
|---|---|---|---|
| `minimal-rect` | *(no `input.folio`)* | — | — |
| `font-text` | **0** | 1 | 1 |
| `image-embed` | **0** | 1 | 0 |
| `multi-script-fallback` | **0** | 1 | 0 |
| `shaped-text` | **0** | 7 | 0 |
| `wrapped-text` | **0** | 4 | 0 |

Six goldens, twelve content elements, one footer element, and **not one page-header element
anywhere**. All six also share one page setup — margins 36 on every edge, both band heights 20 — so
`marginTop == marginBottom` and `pageHeader.height == pageFooter.height`, and **any swap among the
four geometric inputs produces identical bytes**.

A correct assertion over a subject that cannot express the defect is still vacuous.

## What this document's content CAN express

| element | band | band-relative `y` | size | value | what it makes observable |
|---|---|---|---|---|---|
| `e4` | `pageHeader` | 4 | 9 | `HEADER BAND ONLY` | the page-header band origin — the one nothing could see before |
| `e1` | `content` | 0 | 12 | `CONTENT BAND FIRST ELEMENT` | the content band origin at the band's own zero |
| `e2` | `content` | 120 | 12 | `CONTENT BAND SECOND ELEMENT` | a **non-zero** band-relative `y`, so "band origin" and "element y" cannot be conflated |
| `e3` | `pageFooter` | 6 | 8 | `FOOTER BAND ONLY` | the page-footer band origin, which depends on the derived content height |

Its four geometric inputs are **pairwise distinct**:

```
page.margin.top      30 pt    ->  30000 mp
page.margin.bottom   42 pt    ->  42000 mp
pageHeader.height    18 pt    ->  18000 mp
pageFooter.height    24 pt    ->  24000 mp
```

No two are equal, so **no substitution among them is invisible**. Each band's string is distinct, so
a band mix-up shows up in the **rendered text** and not only in a coordinate.

## The partition, hand-derived

```
pageHeader band origin  = 0
content    band origin  = pageHeader.height                                = 18000 mp
content    band height  = 841890 - 30000 - 42000 - 18000 - 24000           = 727890 mp
pageFooter band origin  = 841890 - 30000 - 42000 - 24000                   = 745890 mp
                        ( identically: 18000 + 727890 )
```

These four numbers are asserted as **four independent literals**, never as a sum-to-whole.
D-000.33: an additivity law is satisfied trivially by a degenerate partition, and D-000.36 measured
that the obvious remedy — a non-emptiness precondition — stays green too, because additivity
survives **any monotone boundary function**. The three origins are additionally asserted **strictly
increasing**, so a collapsed band fails rather than passing quietly.

Each element's emitted content-stream Y is asserted against a hand-computed literal
(`pdfY = 841890 - 30000 - runY - baselineOffset`): `798.269`, `781.062`, `661.062`, `51.448`.

**The fourth term is the ruled vertical model's first span, not the point size** (Story 2.5a; DW-15
fixed). It is `max(hhea ascent)` over the element's declared stack, scaled — for this document's
`["Noto Sans"]` stack, `1069/1000 em`, giving `9621`, `12828` and `8552` mp at 9, 12 and 8 pt. Three
sizes give **three distinct offsets**, which is what keeps the "four distinct placements" assertion
non-vacuous after the change.

## What this document CANNOT express

Stated so nobody reads more into it than is there:

- **Nothing about pagination.** One page. Repeating headers and footers are Story 2.6.
- **Nothing about Thai, or any script judgment.** It is deliberately all-Latin and **creates no
  human sign-off obligation**. The Thai reading judgment is D-2.3.5's and is bound to
  `fixtures/shaped-text/`; the Thai break judgment is D-2.4.3's and is bound to the break vector.
- **Nothing about shaping, breaking or inter-baseline spacing.** Every element fits its box on one
  line, so no *second* baseline exists here and the baseline-to-baseline span is unobservable; those
  are Story 2.4's fixtures. This document *does* pin the **first**-baseline span, at three sizes.
- **Nothing about overflow.** Content taller than its band still overflows visibly. Clipping and
  overflow diagnostics are Story 2.8.
- **Nothing about images, bindings or a second face.** One face, four literal strings, no assets.
- **Nothing about a `marginTop`/`marginBottom` swap written inside the content-height derivation
  itself.** Measured: it stays green on every assertion and on the digest, because the two margins
  enter that derivation only as a **sum** and reordering a subtraction changes no number. The
  substitution that *is* a defect — feeding `margin.bottom` into the `MarginTop` role where the page
  setup is read — reddens on the input precondition, on all four content-stream Y literals, and on
  the digest.

## Rules

- `input.folio` is kept **byte-identical to `folio-go/threeBandPageTemplateJSON`**
  (`folio-go/three_band_page_template.go`) **by hand**; `TestThreeBandPageGoldenFixture` asserts it
  before asserting anything else.
- **A hash change here is a defect until proven otherwise** (AD-21/AD-22). Hand-check the change;
  **do not regenerate the fixture to make a test pass.** An intended change is a versioned event
  with a ruling behind it, never a developer's judgment.
- The semantic acceptance assertions (`TestThreeBandPageSemanticAcceptance`) were written and run
  **before** the digest was frozen (D-000.22, D-2.3.5): a hash over bytes nobody has read is Story
  1.1's "two empty files are byte-identical".

## Cross-target status

Registered in **both** `matrixDocuments` (`folio-go/matrix_test.go`) and
`.github/workflows/matrix.yml`'s `docs="…"` line, with four `hash.<target>.three-band-page.txt`
upload paths — `TestMatrixDocumentSlugsAreRegisteredInCI` pins the two lists together.

**Registered; legs deferred to the Epic 2 gate.** The four-target legs have **not** run for this
document. D-000.4's override criterion was applied and Story 2.5's per-story override was
**declined**: an override is warranted when a story introduces a new *source of cross-target
divergence* — float arithmetic, a vendor call, a compressor, a new dependency — not merely because
it records a new golden. Story 2.5 is integer band arithmetic on `geom.Length`.

Two independent process invocations on `darwin/arm64` do produce byte-identical output
(`TestThreeBandPageIsByteIdenticalAcrossTwoProcesses`). That is a determinism witness on **one**
target and is not four-target agreement; nothing here claims it is.

```
recorded on   darwin/arm64, go1.26.0
sha256        746efcbcfb5be30a06caaaefae25e3eaba1962c3fa47a74da10af6d0885372bf
bytes         54452
```

## Provenance — external structural validation (D-000.53)

No golden artifact is accepted — first recording **or re-recording** (D-000.44) — until a reader this
project did not write parses it and resolves it into the semantic objects it claims to contain.

| | |
|---|---|
| reader | `qpdf` **12.4.0** |
| invocation | `qpdf --check fixtures/three-band-page/expected.pdf` |
| result | exit **0** — `No syntax or stream encoding errors found` |
| invocation | `qpdf --show-npages fixtures/three-band-page/expected.pdf` |
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
