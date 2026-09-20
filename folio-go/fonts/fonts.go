// Package fonts holds folio-go's shipped production faces — Story 2.2's
// three (Noto Sans, Noto Sans Thai and Noto Sans SC), Story 16.8's fourth
// (Roboto) and Story 11.1's seven weighted and sloped cuts (Noto Sans
// Bold / Italic / Bold Italic, Noto Sans Thai Bold, Roboto Bold / Italic
// / Bold Italic) — each embedded from its own subdirectory under
// folio-go/fonts/, alongside its OFL-1.1 licence text and NOTICE (AD-26,
// AC25). It is declared at the module root, NOT under internal/: AD-8's
// "no package under internal/ embeds font data" binds internal/ packages
// to never carry shipped font DATA — it says nothing about a leaf,
// non-internal package whose entire purpose is to hold that data
// (spine's own scope fence, AC26).
//
// fonts imports package folio8 (also module root, folio-go/*.go) for its
// FontSet type — a one-directional import (fonts -> folio8) that package
// folio8 never reverses (folio8 -> fonts): confirmed by inspection of
// folio-go/*.go, none of which imports this package. AC9's blockquote
// makes that direction load-bearing: "folio8 must NOT import
// folio8/fonts... two-package import is load-bearing" — reversing it
// would force every consumer of package folio8 to pull in ~14.8 MB of
// embedded font data whether or not it uses the shipped set (11,645,836
// raw bytes before Story 11.1, 14,782,604 after — measured, not
// estimated) — go:embed
// stores RAW bytes, so that is the figure that lands in every binary,
// not the compressed download budget NFR7 is written in.
package fonts

import (
	_ "embed"

	folio8 "github.com/panitw/folio8/folio-go"
)

// The three Story 2.2 faces are STATIC, Regular-only instances derived
// from their upstream variable builds ahead of the build, by
// tools/fontgen/instance_faces.py (D-2.2.4). The .ttf files are
// COMMITTED, not generated at build time, on purpose: generating them
// here would make the shipped font a function of the build environment
// — a different fontTools produces a different font, which produces a
// different PDF — which is AD-22's drift class reintroduced at the
// asset layer. Each face's NOTICE.md records the exact invocation, both
// sha256s and the pinned toolchain, so the derivation can be replayed;
// folio-go/fontgen_matrix_test.go (//go:build matrix) proves it still
// reproduces.

//go:embed notosans/NotoSans-Regular.ttf
var notoSans []byte

//go:embed notosansthai/NotoSansThai-Regular.ttf
var notoSansThai []byte

// NOTO SANS SC IS NOT EMBEDDED HERE. It lives in its own build-tagged
// pair beside this file — notosanssc.go (`//go:build !nocjkface`) and
// notosanssc_absent.go (`//go:build nocjkface`) — because a go:embed
// directive is package-scope and no linker dead-code elimination can
// drop the 10,595,932 bytes it pulls in: a build that must not carry
// them must not COMPILE them, which is what a build constraint on the
// embed's own file buys and what a filter at the call site cannot
// (spec-deferred-offline-cache, CAP-6). The pair follows the
// cgo/!cgo precedent at folio-go/cshared/cmd/folio8/.
//
// THE UNTAGGED BUILD IS UNCHANGED, and that is the whole contract:
// Shipped() returns the same eleven faces it always has, keyed the same
// way, to every consumer — folio-js, folio-dotnet, the CJK golden
// fixtures and the eleven-face documentation included. Only the
// designer's own engine wasm is compiled with `-tags nocjkface`, and
// it receives the face from its host instead (internal/wasm's injected
// FontSet).

// Roboto is Story 16.8's fourth shipped face, and it is NOT a derivation:
// unlike the three Noto faces above, the upstream release publishes a
// static TTF directly, so this file is copied byte-for-byte from the
// designer's own catalogue face (`folio-designer/public/fonts/roboto/
// Roboto-Regular.ttf`, `font-catalogue.json`'s `"roboto"` entry) rather
// than derived by tools/fontgen/instance_faces.py — its own NOTICE.md
// records that explicitly, and fontgen_matrix_test.go's UPSTREAM list is
// unchanged by this addition for that reason.
//
// THERE IS EXACTLY ONE ROBOTO (Story 16.8's own boundary): this file's
// sha256 must always equal the designer catalogue's, and
// TestShippedRobotoMatchesDesignerCatalogue in fonts_test.go makes that a
// machine-checked property. folio-go/testdata/fonts/Roboto-Regular.ttf is
// a DIFFERENT cut used only as an Apache-2.0 licence-signature fixture in
// internal/fontset — it is never embedded here.
//
//go:embed roboto/Roboto-Regular.ttf
var roboto []byte

// ---------------------------------------------------------------------
// Story 11.1's seven weighted and sloped cuts.
// ---------------------------------------------------------------------
//
// Before this story every face here was upright Regular, so Style.Bold
// and Style.Italic had nothing to resolve to and a bold heading printed
// in book weight. These seven give them something. NOTHING IN THIS
// PACKAGE RESOLVES A WEIGHT: 11.2 resolves and 11.3 paints; this file
// only ships the faces and keys them.
//
// EACH CUT IS A FACE OF ITS OWN UNDER ITS OWN KEY (D-B). The keys below
// are chosen to be READABLE, and nothing may derive a family from one:
// a `strings.TrimSuffix(key, " Bold")` anywhere would reinstate the
// naming-convention weight carrier D-B explicitly foreclosed, and would
// silently reinterpret a caller's already-legal key. The
// machine-readable family is the face's own sfnt name ID 1.
// `shippedFaceSpecs.Family` (folio-go/shipped_faces_test.go) is the TEST
// that asserts name ID 1 against the binary — it is not a symbol
// production code can import, and naming it as one sent an earlier draft
// of this comment chasing an unreachable authority.
//
// FOUR ARE DERIVED AND THREE ARE COPIED, exactly as above. The Noto cuts
// are instanced from their upstream variable builds by
// tools/fontgen/instance_faces.py and committed as output; the three
// Roboto cuts come byte-for-byte out of the same upstream static release
// the Regular does. accounting_test.go's
// TestEveryShippedFaceIsAccountedForByExactlyOneRoute holds each cut to
// exactly one of those two routes.
//
// NOTO SANS SC IS DELIBERATELY ABSENT FROM THIS LIST (D-A), and its
// absence is a ruling rather than an oversight: its Regular alone is
// 10,595,932 bytes, so three instances of it would take the offline
// payload from ~11 MB to ~45 MB. A family with no face at a requested
// weight is therefore a permanent, shipped condition 11.2 must state
// rather than a corner case.

//go:embed notosans-bold/NotoSans-Bold.ttf
var notoSansBold []byte

//go:embed notosans-italic/NotoSans-Italic.ttf
var notoSansItalic []byte

//go:embed notosans-bolditalic/NotoSans-BoldItalic.ttf
var notoSansBoldItalic []byte

//go:embed notosansthai-bold/NotoSansThai-Bold.ttf
var notoSansThaiBold []byte

// Noto Sans Thai gains BOLD ONLY, and the count is measured rather than
// assumed: folio-designer/font-index.json records the family's styles as
// 100–900 with no italic variants at all, and upstream publishes none.
// Seven is the whole realizable set, not eight or nine.

//go:embed roboto-bold/Roboto-Bold.ttf
var robotoBold []byte

//go:embed roboto-italic/Roboto-Italic.ttf
var robotoItalic []byte

//go:embed roboto-bolditalic/Roboto-BoldItalic.ttf
var robotoBoldItalic []byte

// Shipped returns folio-go's shipped font set — the three Story 2.2 Noto
// faces, Story 16.8's Roboto and Story 11.1's seven weighted and sloped
// cuts — keyed by the exact face names a `.folio` document's `fonts`
// fallback chains reference. A new document's starter template
// (folio-designer/public/templates/starter.folio) names its default chain
// `"Roboto"` over the same three families, and since Story 11.3 it
// declares the CUTS as well: Roboto as an object carrying `bold`,
// `italic` and `boldItalic`, Noto Sans Thai carrying `bold` alone (there
// is no upstream italic — see the ruling above), and Noto Sans SC as a
// bare string, because it has no cut at all (D-A) and a variant-free
// object canonicalises straight back to a string. The three original
// Noto names remain shipped unchanged for every document that names
// them, `body` chains from before Story 16.8 included.
//
// THE STARTER IS NOT DOCUMENTED HERE TWICE. folio-go's own
// starter_template_test.go intersects every face name that file declares
// with the keys below, so this paragraph going stale is caught by a test
// rather than by a reader. One expression, no arguments (AC9) — callers wire the shipped
// set into a render with `fonts.Shipped()`, never a package-level
// variable a caller could mutate out from under another caller.
//
// NEVER PARSE ONE OF THESE KEYS (D-B). They are readable strings for a
// human writing a chain, not an encoding: the family lives in the face's
// own name table and is asserted there.
func Shipped() folio8.FontSet {
	set := folio8.FontSet{
		"Noto Sans":             notoSans,
		"Noto Sans Bold":        notoSansBold,
		"Noto Sans Italic":      notoSansItalic,
		"Noto Sans Bold Italic": notoSansBoldItalic,
		"Noto Sans Thai":        notoSansThai,
		"Noto Sans Thai Bold":   notoSansThaiBold,
		"Roboto":                roboto,
		"Roboto Bold":           robotoBold,
		"Roboto Italic":         robotoItalic,
		"Roboto Bold Italic":    robotoBoldItalic,
	}
	// THE BUILD-TAGGED FACES ARE MERGED IN, NOT LISTED ABOVE. Under the
	// default build this adds "Noto Sans SC" and the set is the same
	// eleven it has always been; under `nocjkface` it adds nothing and
	// the set is ten. The merge is a loop over a two-file pair rather
	// than a second Shipped() variant so that everything else about this
	// function — its keys, its doc, its one-expression call shape — has
	// exactly one declaration.
	for key, face := range buildTaggedFaces() {
		set[key] = face
	}
	return set
}
