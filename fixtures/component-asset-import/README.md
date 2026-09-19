# Fixture: component-asset-import

Story 5.13's golden record. Unlike `fixtures/image-embed/`, which pins the passthrough
image-**rendering** path (a `.folio` document that already names an image asset), this fixture
pins the **asset-authoring command** (AD-9, D-5.13.1): `setComponentAsset`'s digest-as-key
insertion, 76-column canonical base64 wrapping, sorted-key serialization, insert-if-absent
dedup, and repoint-with-orphan-collection, run through the real `ApplyComponentCommand` /
`SerializeTemplate` path — not hand-authored to resemble its output.

It sits beside `fixtures/image-embed/`, replacing neither: `image-embed/` remains the
passthrough-render baseline (a document that already carries its image asset); this fixture is
the first to pin the **command that produces** a `.folio` document's asset bytes in the first
place.

## How this fixture was generated

**Re-recorded after initial delivery** (Finding 7, review of 2026-08-29): the base document below
now carries a SECOND image element/asset the command never touches. The first version of this
fixture started from `fixtures/image-embed/`'s bare single-image base document, so the command's
own orphan-collection left exactly ONE surviving asset — and a one-entry map has no ordering to
get wrong, which made the sorted-key claim below unfalsifiable (D-5.13.7's own falsification
criterion: "assertion (b) never failing across a real change to ... key derivation would mean it
is not actually pinning the command's output"). Confirmed empirically: even WITH a second asset,
reversing the ITERATION order the command happens to build map entries in front of Go's
`writeObject` serializer is a genuine no-op (that function re-sorts unconditionally, by design);
the property this fixture actually protects is that `writeAssets` keeps delegating to that shared,
sorted `writeObject` path at all, rather than a future edit writing the assets object's fields
directly and skipping it. Verified live: disabling `writeObject`'s sort entirely reddens
`TestComponentAssetImportCommandReproducesTheFixtureInput` (part (b) below) with two assets
present; it stayed green under the same mutation with only one.

1. Start from a base document with TWO `image` elements: `e1` (a 100pt x 60pt box, `image-embed/`'s
   own 3x2 8-bit RGB PNG asset, key `5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078`)
   and `e2` (a 40pt x 40pt box, a real, decodable 4x4 JPEG — `render_image_test.go`'s own
   `jpeg4x4Bytes()` fixture — key `a3beda078fd65550fb477583f62a56b17fcb89a881b22606c1790cede7f9640a`).
   `e2` is never referenced by the command below; it exists purely so a second, different asset
   key survives alongside `e1`'s new one.
2. Apply one real `setComponentAsset` command against element `e1` ONLY, replacing its asset with a
   different real, valid, decodable 1x1 grayscale PNG (`internal/template/fixtures_test.go`'s own
   fixture picture; 81 bytes decoded) declared as `image/png`.
3. Serialize the resulting template canonically (`SerializeTemplate`) — that is `input.folio`,
   captured verbatim, not retyped. The command's own logic (D-5.13.3) repoints element `e1` to
   the new asset's key and, because nothing else in the document referenced the prior asset,
   collects it as an orphan and drops it. `e2`'s asset is untouched throughout — neither repointed
   nor collected — so `input.folio` carries exactly TWO assets: the new one at key
   `541581d3ab4d47c46ce5bcfbe86f9e9369f425b41df11decff572d259fa22c65` and `e2`'s untouched one at
   `a3beda078fd65550fb477583f62a56b17fcb89a881b22606c1790cede7f9640a`, serialized in sorted-key
   order (`5...` before `a...`).
4. Render that document through the public `Render` path (exactly as every other fixture was
   recorded) to produce `expected.pdf`, and record its SHA-256 in `expected.json`.

## Contents

- `input.folio` — the captured canonical output of step 3 above, byte-identical to the
  `componentAssetImportTemplateJSON` constant in `folio-go/render_test.go`.
- `expected.json` — the normative record: SHA-256 of the rendered bytes, `folio8GoVersion`, and the
  exact Go toolchain version that produced the hash (AC16, D-1.2.2). `goToolchain` matches every
  other fixture's recorded value (`assertFixturesShareToolchain`, `folio-go/matrix_test.go`).
- `expected.pdf` — the recorded bytes, kept for human diffing only. **The hash in `expected.json`
  is normative, not this file.**

## What this fixture proves that image-embed does not

`TestRenderMatchesComponentAssetImportGoldenFixture` (`folio-go/fixture_test.go`) asserts two
things, not one:

1. **(a)** the render of `input.folio` matches `expected.json`'s recorded hash, in the same shape
   as every other golden fixture (`TestRenderMatchesGoldenFixture`'s pattern) — with the same
   image-XObject vacuity guard `image-embed/` carries, so a render that silently dropped the
   embedded image would fail here before any hash comparison runs.
2. **(b)** re-running the real `setComponentAsset` command — against the same starting document
   and the same source image bytes used to generate this fixture — reproduces `input.folio`
   byte-for-byte.

(b) is what makes this a Story 5.13 fixture rather than a second `image-embed`: the committed
bytes are the pinned literal, and the command is the live computation being pinned against it. It
red-proofs cleanly — change the base64 wrap width or the digest-as-key derivation, and the
committed bytes disagree with the command's live output. (Nit, review of 2026-08-29: mutating the
digest-as-key derivation specifically reddens through `ApplyComponentCommand`'s own reparse hitting
`decodeAssets`'s pre-existing digest-match enforcement in `parse.go` — a real catch, but a
mechanism Story 1.8 already shipped, not new coverage this fixture adds.) With TWO assets now
surviving the command (Finding 7, above), it also catches a regression that stops `writeAssets`
from serializing through the shared, sorted `writeObject` path — verified live by disabling that
sort entirely and watching this assertion fail.

## Matrix registration

Registered in `matrixDocuments` (`folio-go/matrix_test.go`, `//go:build matrix`) alongside the
other fixtures, so the four-target hash matrix picks this fixture up automatically the next time
it runs (D-5.13.5: this story writes the registration but does not run the matrix itself).
