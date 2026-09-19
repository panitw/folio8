package folio8

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// This file is Story 8.4's SEAM: the one place the document's own font
// assets are turned into something the render path's face-name world can
// consume, and the one place an embedded entry's bytes are decoded.
//
// THE CHOICE, AND THE ALTERNATIVE THAT WAS REJECTED (Task 1 states both
// so a later reader can weigh the trade rather than rediscover it).
//
// The problem: SIX non-test functions take (chain []string, fs FontSet,
// cache *fontCache) — resolveRuneFace, shapeSegments, digitTableRun,
// chainLineMetrics, chainVerticalModel, lineAdvance — and FOUR more take
// the chain for messages and vertical arithmetic only (formatFontChain,
// missingGlyphMessage, verticalModel, scaleAdvanceByLineSpacing). NONE of
// the ten can reach a *Template, so none can reach Assets.
//
// REJECTED: widening the six to carry the document (or a richer chain
// element type). That spreads "what is an embedded entry" across six
// answer sites, which is precisely what chainFaceNames' own doc comment
// says the single boundary exists to prevent — and it leaves the other
// four taking a different chain type from their siblings for no reason
// they can state.
//
// CHOSEN: materialise a per-render NAME -> BYTES view upstream of the
// []string narrowing. chainFaceNames keeps emitting []string; an embedded
// entry simply emits a RESERVED face name (embeddedFaceName below)
// instead of being dropped, and the fontCache — which every one of the
// six already holds — is the single site that knows such a name resolves
// from the document's assets rather than from the caller's FontSet. All
// ten signatures are untouched; there is exactly one answer site; and the
// decode stays LAZY, at the point of use, which is what keeps "a chain
// entry naming a non-font asset that nothing draws from renders clean"
// true rather than aspirational.
//
// THE COST, STATED: the view must be built at BOTH fontCache construction
// sites (predictDocument, render.go; addCanvasTextPaint, page_setup.go),
// or the canvas silently keeps the pre-8.4 behaviour. Both use
// newDocumentFontCache below, and a THIRD production site calling
// newFontCache() with a document in hand is caught by
// TestOnlyTheTwoDocumentAwareSitesBuildAFontCache (font_cache_sites_test.go),
// which scans this package's own non-test sources for the call. That guard
// exists because the claim used to be made for
// TestCanvasMeasuresWithTheEmbeddedFace, which cannot support it: that test
// exercises addCanvasTextPaint alone, so a NEW third call site would ship
// green past it.

// embeddedFaceNamePrefix is the reserved head of every face name this
// package mints for a face the DOCUMENT carries.
//
// WHY A DERIVED NAME AT ALL. fontCache, pagemodel.TextRun.Face and
// pdf.EmbeddedFace.Name are all documented as keying on "the caller's
// FontSet key", so an embedded face needs a key in that same namespace or
// it cannot be cached, cannot be named on a run, and cannot be embedded.
//
// WHY IT IS DERIVED FROM THE ASSET KEY AND NEVER FROM font.family
// (D-8.4.1). AD-8: an embedded face and a shipped face that share a
// family name must never substitute for one another — the ASSET KEY
// decides. font.family/font.style are DISPLAY identity. Deriving this
// name from font.family would let a document's "Inter" and the caller's
// "Inter" collide in exactly the registry AD-8 keeps them out of.
//
// THE PRECEDENCE RULE, so a collision cannot be exploited either way: a
// name in this namespace is resolved from the DOCUMENT'S OWN ASSETS,
// always, and the supplied FontSet is not consulted for it even if the
// caller happens to have filed an entry under the same string
// (fontCache.get checks the embedded index FIRST). A caller therefore
// cannot shadow a document's carried face, and a document cannot capture
// a caller's.
//
// The prefix is letters and a colon: internal/pdf's pdfNameEscape keeps
// [A-Za-z0-9_-] and drops the rest, so this reaches a PDF resource
// dictionary as "asset" + the 64 hex characters of the key — distinct per
// asset, and unmistakable in a produced file.
const embeddedFaceNamePrefix = "asset:"

// embeddedFaceName is the ONE construction site for that name. A caller
// that spells the prefix itself is writing the derivation a second time.
func embeddedFaceName(assetKey string) string { return embeddedFaceNamePrefix + assetKey }

// embeddedFaceAssetKey is that derivation READ, and it lives here because
// reading it and writing it are one decision: the prefix is spelled in
// this file and nowhere else, in this language or any other (Story 8.4a
// carries the ASSET KEY to the browser precisely so no TypeScript has to
// spell it a second time). It reports false for a name that is not in
// this namespace at all, which is every shipped FontSet face.
//
// IT IS NOT AN AUTHORITY ON ITS OWN. A name's SHAPE says only that it
// could have come from this mint; whether a document actually carries the
// face behind it is the embeddedFaceIndex's answer, and the precedence
// rule above is stated in terms of that index. fontCache.carriedAssetKey
// (render.go) is therefore the caller-facing accessor: it asks the index
// first and only then reads the name.
//
// A MISS RETURNS THE EMPTY STRING, NEVER THE NAME. strings.CutPrefix
// hands back the WHOLE INPUT alongside false when the prefix is absent,
// so an accessor that forwarded its two results verbatim would put a face
// NAME wherever a caller ignored the bool. Story 8.4a's projection carries
// this value onto the wire, where the browser admits a fragment's assetKey
// only as 64 lowercase hex characters and rejects the whole projection
// otherwise — which terminates the worker and kills the session. The
// emptying is done HERE, once, rather than trusted to every call site:
// "not in this namespace" is an ABSENCE, and the value that says so must
// be the empty one. Pinned by TestANonMintedFaceNameYieldsNoAssetKey.
func embeddedFaceAssetKey(name string) (string, bool) {
	key, ok := strings.CutPrefix(name, embeddedFaceNamePrefix)
	if !ok {
		return "", false
	}
	return key, true
}

// embeddedFaceSource is one carried face's resolution record: the asset
// to decode, and the addresses to put in the error if it cannot be.
type embeddedFaceSource struct {
	assetKey string
	// asset is the document's own asset record, captured at index time so
	// the decode needs no *Template. present is false when the chain names
	// a key the assets map does not hold — structurally unreachable today
	// (decodeFontChainEntry refuses an absent key at load) and reported
	// rather than panicked on, exactly as the image loop reports the same
	// condition.
	asset   template.Asset
	present bool
	// sites are EVERY place this asset key is named by a chain entry, in
	// the order newEmbeddedFaceIndex visits them (chains in sorted name
	// order, entries in the document's authored order). ElementID is left
	// EMPTY on every one of them on purpose — see template.FontChainSite's
	// own doc comment.
	//
	// THERE IS A LIST RATHER THAN ONE SITE BECAUSE THE FACE NAME IS
	// DERIVED FROM THE ASSET KEY ALONE (AD-8/D-8.4.1: the key decides, and
	// it must, or a document's face and a caller's could collide). So one
	// asset key is one face name however many chains name it — while the
	// ADDRESS the error prints is per chain. Keeping only the first
	// occurrence made the error name a chain the failing element may not
	// draw through at all, which sends the author to the wrong line of
	// their own document.
	sites []template.FontChainSite
}

// siteIn is the address to print for a failure discovered while drawing
// through chainName: the entry of THAT chain, so the coordinates match
// the element's own chain rather than whichever chain sorted first.
//
// The chain name reaches this through the CACHE rather than through the
// ten chain consumers' signatures: fontCache.forChain returns a view of
// the cache scoped to one document chain, and the three sites that resolve
// a chain name (collectBandTextRuns and collectBandTableRuns in render.go
// and table_render.go, addCanvasTextPaint in page_setup.go) hand the scoped
// view down. Nothing between them had to widen.
//
// It falls back to the first recorded site when the chain is not known —
// a caller with no document chain in hand (the PDF producer's post-shaping
// loops, a test passing an ad-hoc chain) — which keeps the pre-existing
// deterministic answer for everything that stays global.
func (s embeddedFaceSource) siteIn(chainName string) template.FontChainSite {
	if chainName != "" {
		for _, site := range s.sites {
			if site.ChainName == chainName {
				return site
			}
		}
	}
	if len(s.sites) == 0 {
		return template.FontChainSite{AssetKey: s.assetKey}
	}
	return s.sites[0]
}

// displayName is how a MESSAGE spells this face for a person, and it is
// the only place an embedded entry is spelled as anything other than the
// reserved name that resolves it.
//
// WHY IT EXISTS. The reserved name is "asset:" + 64 hex characters,
// because the asset key is what AD-8 makes the resolver. Printed in a
// missing-glyph diagnostic it reads as though the author mistyped a font
// name, and the digest is not something they can act on — while D-000.37
// makes the diagnostic's job "here is what to fix".
//
// WHY font.family IS THE RIGHT SOURCE **HERE** AND ONLY HERE. D-8.4.1
// calls font.family DISPLAY IDENTITY — "what a chain editor shows a
// person" — and forbids it from resolving or substituting a face. This is
// display, and nothing on this path reaches the fontCache, a
// pagemodel.TextRun.Face or a PDF resource dictionary.
//
// A carried face with no font.family (the record is optional, and so is
// every key in it) is spelled by a SHORT key prefix instead: something to
// tell two unnamed carried faces apart, without putting a 64-character
// digest in a sentence a person reads.
func (s embeddedFaceSource) displayName() string {
	if family := s.familyName(); family != "" {
		return "embedded " + strconv.Quote(family)
	}
	key := s.assetKey
	if len(key) > 8 {
		key = key[:8] + "…"
	}
	return "embedded font asset " + key
}

// familyName is the asset's declared font.family, or "" when the record,
// the key or the value is absent. Presence all the way down: `"font":
// null` and `"family": null` are legal spellings and neither is a name.
func (s embeddedFaceSource) familyName() string {
	if !s.asset.Font.Set || s.asset.Font.Null {
		return ""
	}
	family := s.asset.Font.Value.Family
	if !family.Set || family.Null {
		return ""
	}
	return family.Value
}

// embeddedFaceIndex maps a minted face name to the asset behind it. It is
// the RESOLUTION namespace, keyed by the asset key alone (AD-8), which is
// what reaches the fontCache, pagemodel.TextRun.Face and the PDF resource
// dictionary. It is LOOKED UP BY KEY ONLY and never ranged (ScanMapRange,
// D-2.2.3): its only iteration is at construction, over a SORTED
// chain-name slice.
type embeddedFaceIndex map[string]embeddedFaceSource

// source is the by-name lookup, spelled once.
func (x embeddedFaceIndex) source(name string) (embeddedFaceSource, bool) {
	src, ok := x[name]
	return src, ok
}

// newEmbeddedFaceIndex builds the per-render view from the document's
// own fonts map. It DECODES NOTHING — it costs one map walk and no
// base64, so building it unconditionally at both cache sites is free.
//
// DETERMINISM. One asset key may appear in several chains, and the face
// name is derived from the asset key ALONE (AD-8: the key decides), so
// exactly one RESOLUTION record survives per key. Its ADDRESSES do not
// collapse that way: every occurrence is recorded, and the one printed is
// chosen by the chain being drawn (embeddedFaceSource.siteIn). Chains are
// visited in SORTED NAME order and entries in the document's authored
// order, so both the fallback address and the sequence-collision tie-break
// are independent of map order.
//
// ⚠ SINCE STORY 11.2 THE SIBLINGS ARE A THIRD AXIS, and they are visited
// in the closed variant set's own FIXED ORDER — that is what
// FontChainEntry.EmbeddedAssetKeys returns, discriminant first. This
// walk used to read `entry.AssetKey` alone, which was the SECOND MINT
// SITE for the reserved namespace and would have FAILED on a variant
// key: the variant's minted name would be absent from the index,
// cache.declares would answer false, faceCovers would report no
// coverage, and the render would fall back to the entry's base face.
//
// ⚠ NOT SILENTLY — an earlier draft of this comment said so and it was
// WRONG. That fall-back is AC3's absence arm, so it emits a
// TEXT_STYLE_FACE_UNDECLARED Warning naming the element, the rune and
// the base face. The defect is real and worse than a missing diagnostic
// in one respect: the document DOES carry the bold face, states its
// terms and names it correctly, and the page would come out at the wrong
// weight while a Warning told the author their chain declared no bold.
// The face is derived from the entry now, not re-walked here.
func newEmbeddedFaceIndex(t *Template) embeddedFaceIndex {
	index := embeddedFaceIndex{}
	if t == nil || t.doc == nil {
		return index
	}
	chainNames := slices.Sorted(maps.Keys(t.doc.Fonts)) // sorted: deterministic addresses.
	for _, chainName := range chainNames {
		for i, entry := range t.doc.Fonts[chainName] {
			for _, assetKey := range entry.EmbeddedAssetKeys() {
				name := embeddedFaceName(assetKey)
				src, seen := index[name]
				if !seen {
					asset, present := t.doc.Assets[assetKey]
					src = embeddedFaceSource{assetKey: assetKey, asset: asset, present: present}
				}
				src.sites = append(src.sites, template.FontChainSite{
					AssetKey:   assetKey,
					ChainName:  chainName,
					EntryIndex: i,
				})
				index[name] = src
			}
		}
	}
	return index
}

// decodeAt is the font analogue of predictDocument's image loop
// (render.go): DecodeAssetBytes -> template.DecodeFontForRender, in that
// order, with prose location and NO diagnostic code — D-7.8.1, and the
// image precedent (render.go's `fmt.Errorf("folio8: Render: %w", derr)`)
// takes the same shape for the same reason: no consumer branches on it.
//
// THE ERRORS ARE RETURNED BARE, without a "folio8: Render:" prefix,
// because every caller on this path already applies its own located
// wrapper — collectBandTextRuns wraps everything out of shapeSegments
// with "folio8: Render: element %s:", naming the element that actually
// needed the face. That is why FontChainSite leaves ElementID empty
// here: prefixing again would name the element twice in one sentence.
//
// The SITE IS A PARAMETER rather than a field, because one asset key can
// be named by several chains and the address to print is the one the
// failing element draws through — see embeddedFaceSource.sites.
//
// It is called from fontCache.parseEmbedded and nowhere else, so a face
// is decoded at most once per render even though this function memoizes
// nothing itself.
func (s embeddedFaceSource) decodeAt(site template.FontChainSite) ([]byte, error) {
	if !s.present {
		return nil, fmt.Errorf("%s: names an asset that is not present in the document's assets map", site)
	}
	raw, derr := template.DecodeAssetBytes(s.asset)
	if derr != nil {
		return nil, fmt.Errorf("asset %q: %w", s.assetKey, derr)
	}
	if derr := template.DecodeFontForRender(s.asset.MediaType, raw, site); derr != nil {
		return nil, derr
	}
	return raw, nil
}
