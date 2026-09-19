---
id: SPEC-shipped-font-tiers
companions:
  - ./weight-analysis.md
  - ../../../docs/rendering-library.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Tier the shipped font set so the Chinese/Japanese face is opt-in

> **Status: DEFERRED** by owner decision. The work is postponed, not rejected.
> The measurements and reasoning below stand; only the sequencing changed —
> see *Why* for the route that replaced the original pre-tag deadline.

## Why

`fonts.Shipped()` embeds 14 MB of typefaces, and **10 MB of that — 71% — is a single face, Noto Sans SC**, which only matters to documents that render Chinese or Japanese. Measured, ~27,500 of its 31,036 glyphs are CJK ideographs, so the weight is irreducible without choosing which characters to drop — the tier boundary is the whole face. See [weight-analysis.md](./weight-analysis.md). Every Go caller pays it, and under [SPEC-client-libraries](../spec-client-libraries/SPEC.md) every npm and NuGet install would pay it too. That is the **opportunity**: a 71% cut to the default dependency weight that costs nothing to anyone who is not rendering Chinese or Japanese, and costs one explicit dependency to anyone who is.

This originally carried a deadline — land before `folio-go/v1.0.0` or wait for a major version — because removing a face from `fonts.Shipped()` after the tag is a breaking change. That deadline **no longer applies**: the owner deferred the work and accepted that `Shipped()` keeps all eleven faces for the life of v1. The route afterwards is **additive and semver-safe**: add a `fonts/cjk` sub-package and a `fonts.ShippedCore()` beside an **unchanged** `Shipped()`, so existing callers are untouched and no `/v2` is needed.

A second route needs no engine change at all: the client libraries construct their own `FontSet` and need not call `Shipped()`, so binding-side tiering can capture the same install saving independently. It was declined for now in favour of matching the engine's face list exactly.

## Capabilities

- **CAP-1**
  - **intent:** A caller who renders no Chinese can opt into a materially smaller dependency without editing a template.
  - **success:** A build using the core constructor carries ~4.2 MB of faces instead of ~14 MB, and every golden-corpus fixture that does not use CJK renders byte-identically with no source change.

- **CAP-2**
  - **intent:** A caller who does render Chinese or Japanese adds one explicit dependency and gets exactly what they get today.
  - **success:** With the opt-in package present, every CJK fixture renders to the committed expected SHA-256 — unchanged from before the split.

- **CAP-3**
  - **intent:** A document naming a face that is not installed fails in a way that names the fix, instead of rendering wrong.
  - **success:** Rendering a CJK-naming template without the opt-in package produces a diagnostic naming both the missing face and the package that provides it — never a silent substitution, a blank glyph run, or tofu. This covers a missing **face**; missing **coverage** within a present face is a different condition — see the open question.

- **CAP-4**
  - **intent:** The designer keeps offering every face it offers today, unaffected by how the library is packaged.
  - **success:** The designer's face list and rendering are unchanged before and after the split, and its build still resolves every face it advertises.

## Constraints

- **`Shipped()` itself may not change.** After v1.0.0 its contents are fixed for the life of v1, so the retiering is additive only: a new `fonts/cjk` sub-package and a new `fonts.ShippedCore()`, with `Shipped()` returning all eleven faces exactly as it does today.
- **Byte-identity is not negotiable.** Non-CJK fixtures must render identically after the retiering, and CJK fixtures identically with the opt-in package added. A moved hash is a defect until someone proves it was intended.
- **Nothing that renders today may stop rendering.** This is a packaging change, not a capability reduction — every face remains obtainable.
- **Noto Sans Thai stays in core.** It costs 124 KB (0.9%) and carries a first-class concern: the Thai bill-payment barcode fixture, the `thai_words.trie` dictionary embedded in the engine, and Thai line-breaking.
- **Noto Sans (Latin) stays in core** despite overlapping Roboto. [fonts.go](../../../folio-go/fonts/fonts.go) keeps the three original Noto names for every document that already names them, `body` chains from before Story 16.8 included; removing it breaks authored templates.
- **The starter template and the core tier must agree.** `starter_template_test.go` intersects the face names the starter declares with the `Shipped()` keys — a core tier that drops a face the starter names reds that test.
- **The tier is named for Chinese *and Japanese*.** Measured, Japanese depends on this same face — kana and kanji are all present — so "CJK" understates who the opt-in package is for. Wherever the tier is named, the caveat travels with it: Japanese and Traditional Chinese render with **Simplified Chinese regional glyph variants**, so coverage is complete but regional typographic correctness is not.
- **Korean is not covered by the shipped set at all** (Hangul Jamo 0/256, Hangul Syllables 0/11,172) and this work must not appear to change that. A pre-existing gap, recorded where the coverage evidence lives.
- **No fetch from a third party.** Unchanged from the engine's determinism commitments: faces arrive as an embedded package or from the application's own origin, never from a font CDN.

## Non-goals

- **Removing any face from the project.** Noto Sans SC continues to ship; only where it ships from changes.
- **Changing the fallback-chain mechanism, face naming, or the `.folio` `fonts` syntax.** Templates are untouched.
- **Retiering the Story 8.5 catalogue faces.** Those already fetch on demand and are a separate mechanism.
- **Adding Korean coverage.** Out of scope here; naming the gap is not a commitment to close it.
- **Subsetting or re-deriving any face.** The committed faces stay byte-for-byte what `tools/fontgen` produced; a different fontTools run produces a different font and therefore a different PDF.
- **A font-management or font-upload feature.** Out of scope entirely.

## Success signal

`go get` of the engine, and `npm install` / `dotnet add package` of the client libraries, pull ~4.2 MB of faces instead of 14 MB — and the golden corpus proves, hash for hash, that nothing rendered differently as a result. A team rendering Chinese adds one dependency, named for them by the diagnostic they hit the first time they forget it.

## Assumptions

- The ~4.2 MB core figure is the measured sum of Roboto (4 cuts), Noto Sans (4 cuts) and Noto Sans Thai (2 cuts); see [weight-analysis.md](./weight-analysis.md).
- Whenever this is picked up, the measured weights still hold; they were taken against `folio-go/fonts/` at the time of deferral and no font work has landed since.

## Open Questions

- What are the additive names — `fonts/cjk` as a sub-package, `fonts.ShippedCore()`, `fonts.ShippedCJK()`, or some combination? Additive names can be introduced at any point in v1, so this is no longer time-critical.
- Is engine-side retiering still the right route at all, given binding-side tiering achieves the same install saving with no engine change and no version implications?
- Korean renders as tofu today with no diagnostic at all. Should missing **coverage** — the face is present but has no glyph for the character — raise a diagnostic of its own, distinct from CAP-3's missing **face**? It may belong in a separate spec about coverage reporting rather than here.
- Does the designer keep loading Noto Sans SC as it does today, or does it become a fetch-on-demand catalogue face like the Story 8.4d/8.5 faces?
- Do any golden-corpus fixtures render CJK? If so the corpus job needs the opt-in package as an explicit CI dependency, not an incidental one.
