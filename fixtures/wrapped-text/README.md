# `fixtures/wrapped-text/` — line breaking in all three scripts (Story 2.4, S4)

The golden for **line breaking and measurement**. Every earlier text fixture *fits* its boxes — all
ten of their text elements were measured, the widest at 26% of its declared width — so none of them
can tell a build that wraps from one that does not. This is the only fixture whose elements are
measured to **overflow** their boxes, and therefore the only one whose bytes move if wrapping breaks
— and, as of Story 2.8, the only one whose bytes move because FR44's clip finally exists to paint
something differently over that overflow (see "Story 2.8" below).

## What each element is for

| Element | Script | Box | Full text measures | Lines | What it proves |
|---|---|---|---|---|---|
| `e1` | Latin | 150 pt | 331,683 mp | 3 | breaks after a whitespace run, and the run is **consumed** — drawn on neither line |
| `e2` | Thai | 150 pt | 192,830 mp | 2 | breaks come from the embedded dictionary, under AD-25's absolutes |
| `e3` | CJK | 150 pt | 198,000 mp | 2 | breaks between adjacent Han characters |
| `e4` | Thai, bound | **20 pt** | 46,585 mp | 2 | **the declared unbreakable value** |

All four boxes are narrower than the text they hold. That is measured, and asserted as a precondition
before any wrapping assertion runs — "it wrapped" must not be satisfiable by an input that fitted.

## `e4` is the discriminating element — read this before changing its width

`e4` binds `customer.name`, which the document declares in `unbreakableValues`. Its value is
`ศรีสุข` — D-2.1.6's worked case, a surname that is character-for-character the two common
dictionary words `ศรี` and `สุข`.

Its box is **deliberately narrower (20,000 mp) than the value itself measures (24,585 mp)**. That is
what makes the declaration load-bearing rather than decorative:

- **declared** (as shipped): `ผู้รับ` / `ศรีสุข` — the surname stays whole and, as of Story 2.8, is
  **clipped at the box's left/right edges** (FR44, D-2.8.1): every glyph is still drawn — never
  reflowed, never dropped — but a PDF clip path (`W n`) restricts what is visible to the box's
  20,000 mp width, and a `Diagnostic` (`TEXT_CLIPPED_WIDTH`, naming `e4`) is returned alongside the
  bytes. Before Story 2.8 the surname painted past the box's edge with no clip and no diagnostic;
  that is the AD-25 prescription this story finishes.
- **undeclared** (the same template with the list removed, nothing else changed): `ผู้รับ` / `ศรี` /
  `สุข` — the surname **splits**.

`TestWrappedTextDeclarationIsLoadBearing` asserts **both**. Widening this box breaks the test on
purpose: at any width ≥ 24,585 mp the two polarities produce the same layout and the fixture stops
proving anything about the declaration.

## Rules

- **Hand-check and re-record are different things.** This fixture's *break placement* is certified by
  `fixtures/expected-breaks/`, not here — one human judgment, at the layer where it is legible
  (D-2.4.3, DN-4). There is deliberately **no third sign-off** for this document.
- **A hash change here is a defect until proven otherwise** (AD-21/AD-22). Do not regenerate to make a
  test pass. Every semantic property — line counts, per-line widths against the box, no boundary
  inside a Thai cluster, no boundary inside a declared span, the embedded faces, the `/ToUnicode`
  section sizes — is asserted at recording and none of it is deferrable (D-000.22, D-2.3.5).
- **The vertical model is welded in.** Both spans come from the *declared* font stack, each axis
  maximised **independently** (D-2.4.2 as **amended**, Story 2.5a):

  | span | rule | this fixture, at 11 pt |
  |---|---|---|
  | top → first baseline | `max(A)` | `1160/1000 em` (Noto Sans SC) → **12,760 mp** |
  | baseline → baseline | `max(A) + max(D) + max(gap)` | `1160 + 450 + 0 = 1610` → **17,710 mp** |

  The superseded form — `max(ascent - descent + lineGap)`, the largest *single* face — gave 1511/em
  and 16,621 mp, **99 units per em short**, because it assumed one face supplies both axes when Noto
  Sans Thai wins the descent (450) and Noto Sans SC the ascent (1160). Changing that rule re-records
  this fixture and every one recorded after it.

  **This is the ONLY fixture in the repository on which the baseline-to-baseline span is observable
  at all** — it is the only one with a multi-line element on a multi-face stack, and for a
  single-face stack the amended and superseded forms are identical. Measured by mutation: reinstating
  the superseded rule reddens this fixture and nothing else.
- `input.folio` is kept byte-identical to `folio-go/wrapped_text_template.go`'s
  `wrappedTextTemplateJSON` by hand, as `font-text` and `multi-script-fallback` already are.

## Cross-target status

Registered in `matrixDocuments` **and** in `.github/workflows/matrix.yml`, and — unlike `shaped-text`
— its four legs were **actually run in its own story**, because D-000.4 names 2.4 as a per-story
matrix override ("line breaking feeds every measurement"). Story 2.8 re-ran all four legs against the
NEW bytes below (`go test -tags=matrix -run TestCrossTargetByteIdentity .`, `TestCrossTargetByteIdentity`
PASS) — this is a **re-recording of an existing registered document**, not a new registration, so the
obligation is "the four legs agree on the new bytes before the digest is trusted" (D-000.54's native-leg
obligation attaches only to a *newly* registered document and is NOT owed here; this is the other case).
All four targets agree:

```
darwin/arm64   07c38cf765a39d86376c1a3c78bfb6f0a96f089f19792c9bfeeaa1dc754269d6  72790 bytes
linux/amd64    07c38cf765a39d86376c1a3c78bfb6f0a96f089f19792c9bfeeaa1dc754269d6  72790 bytes
linux/arm64    07c38cf765a39d86376c1a3c78bfb6f0a96f089f19792c9bfeeaa1dc754269d6  72790 bytes
js/wasm        07c38cf765a39d86376c1a3c78bfb6f0a96f089f19792c9bfeeaa1dc754269d6  72790 bytes
```

## Story 2.8 — FR44's clip re-recorded this golden

`e4`'s overflowing surname is now clipped (see "`e4` is the discriminating element" above): the PDF
content stream wraps `e4`'s two lines in `q / 36 0 20 841.89 re / W n / ... / Q` — a clip restricted to
the box's HORIZONTAL span alone (`36` = left margin + `e4`'s `x`; `20` = `e4`'s declared width). The
clip rectangle's vertical extent is the whole page (`0` to `841.89`, the A4 height in points) — never
anything derived from `e4`'s declared `height` (D-2.8.1: a text element's declared height is not a clip
bound). This is the **only** change: `e1`/`e2`/`e3` and every other committed golden are byte-identical
before and after this story (measured, `TestGoldenDigestAgreesAtEveryDeclaredSite`).

**This is a deliberate re-recording, not a regression** (AD-21/AD-22, D-000.44): there is no
byte-neutral way to stop painting something FR44 says must be clipped. See Story
`2-8-clip-overflowing-content-and-say-so.md`'s "Golden movement" section for the full argument.

## Provenance — external structural validation (D-000.53)

No golden artifact is accepted — first recording **or re-recording** (D-000.44) — until a reader this
project did not write parses it and resolves it into the semantic objects it claims to contain.

| | |
|---|---|
| reader | `qpdf` **12.4.0** |
| invocation | `qpdf --check fixtures/wrapped-text/expected.pdf` |
| result | exit **0** — `No syntax or stream encoding errors found` |
| invocation | `qpdf --show-npages fixtures/wrapped-text/expected.pdf` |
| result | **1** page(s), matching the declared `/Count` |
| validated at | Story 2.8's development commit (baseline `278520b`; finisher records the landing commit) |

**Re-validated for Story 2.8's re-recording** (D-000.44: a re-recording is a recording, so this step is
owed again — it is not carried forward from the `50ad6c8` row below).

Prior validation (Story 2.6, retroactive, predates D-000.53): settled at `50ad6c8`, recorded so that
row ends settled rather than carried (D-000.29).

The external reader is the **acceptance instrument, at recording time only** — run off-leg on the
recording machine, hand-checked, output pasted here. It is never a runtime or CI dependency (AD-25,
`TestModuleGraphAllowlist`), and it is deliberately **not** gated to "the legs that have qpdf": a check
that runs on some legs and not others reproduces D-000.9's failure — an "all clear" indistinguishable
from "I could not look" — one level up, at the leg. The standing every-leg regression guard is the
in-repo checker `folio-go/golden_structural_validity_test.go`, which is hermetic and covers all four
targets including `js-wasm`.
