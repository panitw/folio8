# Fixture: multi-script-fallback

Story 2.2's fourth matrix document (AC8, D-1.8.6: **added**, replacing neither `font-text/` nor
`image-embed/`) — the first fixture that renders folio8's actual **shipped** face set
(`github.com/panitw/folio8/folio-go/fonts`.Shipped()) through the public `Render` API, and the
first that genuinely exercises AD-8's ordered fallback **chain**: one text element whose value
mixes three scripts against a three-member chain, `["Noto Sans", "Noto Sans Thai", "Noto Sans
SC"]`.

## Why this fixture exists

`fixtures/font-text/` (Story 1.5) proves font embedding works, with one face. It says nothing
about **coverage-based, per-rune fallback across a chain** (AC4), or about the **shipped faces being
the static, Regular instances they claim to be** (AC7, AD-7 — PDF 1.7 cannot express a variable
font). Both are new hazards this story introduces, and D-000.4's override rules the full four-target
matrix must cover them in-story, not at the Epic 2 gate.

## Contents

- `input.folio` — byte-identical to `folio-go/render_test.go`'s `multiScriptTestTemplateJSON`
  constant (verified, same shape as `font-text/`'s own `TestRenderMatchesFontTextGoldenFixture`
  drift check).
- `expected.json` — the normative record: SHA-256 of the rendered bytes, `folio8GoVersion`,
  `goToolchain` — recorded ONLY after `TestCrossTargetByteIdentity` confirms all four targets
  (`darwin/arm64`, `linux/amd64`, `linux/arm64`, `js/wasm`) agree (D-2.2-D5's reasoning for why
  `font-text` was not switched applies here from day one: a fixture whose golden was recorded from
  one machine would silently retire the four-target coverage this story exists to add).

## The document

- `"Ada ก 汉"` — Latin "Ada" (resolves to **Noto Sans**, the chain's first member), Thai "ก"
  (Noto Sans has no glyph for it; resolves to **Noto Sans Thai**), and a Han ideograph "汉"
  (resolves to **Noto Sans SC**). All three shipped faces are exercised by ONE element, proving the
  chain is walked **per rune**, not "whichever chain member is present wins for the whole element"
  (the pre-Story-2.2 behaviour).

## Feature guard (AC8, V6)

`requireInstancedShippedFaces` (`folio-go/matrix_test.go`) runs on **every captured leg, before any
byte comparison**. It extracts all three embedded `FontFile2` programs and asserts, each behind a
presence precondition:

- no `fvar`, `gvar` or `avar` table — each program is genuinely **static**, not merely "a
  FontFile2", which `requireFontFile2` alone would accept from a variable font passed straight
  through;
- `glyf` present and `CFF2` absent — NFR7's outline-format choice;
- **`OS/2.usWeightClass == 400`** — read off the embedded program;
- `/BaseFont` matching `^[A-Z]{6}\+<PostScript name>$` — one subset tag, one `+`, and the face's
  own name (ISO 32000-1 §9.6.4 and Table 117).

**The weight assertion is why this fixture had to be re-recorded.** The first recording embedded
Simplified Chinese at `usWeightClass = 100` — **Thin** — because `NotoSansSC-VF`'s `wght` axis
defaults to 100 and the render seam pinned every axis to its default. The old guard checked only
that the variation tables were gone, and the Thin program satisfied that perfectly: it *was*
genuinely instanced. Four targets then agreed on it byte-for-byte, and each was right about the
question it had been asked. None was asked whether the value meant what its name implied
(D-000.21, D-000.22).

Story 2.2 now ships **static, Regular-only** faces derived ahead of the build
(`tools/fontgen/instance_faces.py`, D-2.2.4), and `fontset.New` **rejects** a caller-supplied
variable face outright rather than choosing an instance on the caller's behalf.

## Per-(face, pinned-instance) goldens (AC7)

The story ships exactly three pairs — one Regular instance per shipped face (Bold is out of scope:
the package exposes no way to request a non-default instance, so a Bold face would be selectable by
nothing; D-2.2.1's standing condition, now **DW-12**, means the story that adds one inherits this
obligation). Each pair's embedded program digest is pinned in `folio-go/fixture_test.go`
(`wantProgramSHA256`), and the count is asserted against the shipped set's own cardinality rather
than a literal — the earlier `!= 3` compared a slice literal's length to a constant six lines above
it, and was red-proved open. This sits alongside — not instead of — this fixture's own whole-document SHA-256 above: the whole-document
hash proves the DOCUMENT is stable; the three per-face digests prove each PAIR individually is,
which is what AC7's "not one sample" language asks for.

## If this fixture's hash test goes red

Same rule as every other fixture (AD-21, AD-22): a hash change here is a defect until proven to be
an intended, versioned change. Never regenerate this fixture just to make a failing test pass;
investigate first, and re-record deliberately as part of a documented breaking change — recording
the new hash ONLY after all four targets agree.

## Provenance — external structural validation (D-000.53)

No golden artifact is accepted — first recording **or re-recording** (D-000.44) — until a reader this
project did not write parses it and resolves it into the semantic objects it claims to contain.

| | |
|---|---|
| reader | `qpdf` **12.4.0** |
| invocation | `qpdf --check fixtures/multi-script-fallback/expected.pdf` |
| result | exit **0** — `No syntax or stream encoding errors found` |
| invocation | `qpdf --show-npages fixtures/multi-script-fallback/expected.pdf` |
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
