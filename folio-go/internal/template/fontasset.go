package template

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

// This file is image.go's shape, for fonts (Story 8.3). It is a
// deliberate MIRROR and not a generalisation of it: the two share a
// media-type key and nothing else — an image decode yields pixel
// dimensions a layout consults, a font decode yields nothing this
// library reads at load, and folding them into one "asset decoder"
// would put a pixel-dimension trust boundary on a code path that has no
// pixels.
//
// Rendering FROM an embedded face is Story 8.4, and it is the render
// path (folio8.fontCache, render.go) that calls DecodeFontForRender
// below. Nothing HERE parses a face for glyphs, shapes with one, or
// subsets one: this file still stops at "can this build read these
// bytes at all".

// FontChainSite is WHERE in a document a font asset was asked for, and
// it exists because Story 8.4 gave the capability error two more
// coordinates than its image twin needs.
//
// An image asset is placed by an ELEMENT and that is the whole address.
// A font asset is named by an ENTRY of a named fallback CHAIN, and the
// chain may be shared by many elements — so "asset K is not a font this
// build reads" is only actionable when the reader is told which chain
// and which position in it to go and edit. The four fields are carried
// as one value rather than as four positional parameters because they
// are one address, and because a six-argument decode call is how two of
// them come to be passed in the wrong order.
//
// ElementID is OPTIONAL and is empty on the render path, deliberately.
// render.go's own caller wraps every error out of shapeSegments with
// "folio8: Render: element %s:" naming the element that actually needed
// the face — a better locator than the first element that happened to
// reference the chain, and printing both would name an element twice in
// one sentence. A caller that does NOT supply its own located prefix
// (internal/template's own tests, a future non-render consumer) fills
// this in and gets it in the message.
type FontChainSite struct {
	AssetKey   string
	ElementID  string
	ChainName  string
	EntryIndex int
}

// String is the site as a message fragment, with each coordinate omitted
// when it is not known rather than printed as a hole (Story 1.8's
// Finding 4, applied before it can recur here).
func (s FontChainSite) String() string {
	out := ""
	if s.ElementID != "" {
		out += "element " + s.ElementID + ": "
	}
	out += "asset " + s.AssetKey
	if s.ChainName != "" {
		out += ": font chain " + strconv.Quote(s.ChainName) + " entry " + strconv.Itoa(s.EntryIndex)
	}
	return out
}

// CarriedFaceError marks a fault that arose while resolving a face THE
// DOCUMENT ITSELF CARRIES. It is D-8.4.12's discriminator, and it is a
// STAMP rather than a message: it adds no text of its own and delegates
// Error() to what it wraps.
//
// WHAT IT DECIDES. The canvas projection must degrade the one element and
// never abort (D-7.4.2) — but only for a fault the author can actually
// REPAIR ON THE CANVAS. D-8.4.12 rules that the dividing line is
// POSITIONAL, not a list of error types: a fault arising while resolving a
// face the element's own chain names is attributable to document content
// and degrades that element WHATEVER ERROR TYPE CARRIES IT; a fault
// arising after every face the element needs has resolved still aborts.
//
// WHY A STAMP AND NOT AN ALLOWLIST. The gate it replaces tested the error
// against ONE type — UnsupportedFontMediaTypeError, immediately below —
// at the CONSUMER, and D-8.4.12 found the defect that shape guarantees:
// the population an error-type gate excludes contains document
// properties, and every FUTURE face-resolution failure type rejoins the
// abort silently, with nothing observing it. checkSfnt in this file is
// the standing proof — it bounds-checks the header and every
// table-directory record and never reads a table's CONTENTS, so a
// structurally valid sfnt over unreadable contents LOADS, and then
// aborted the whole canvas projection at fontset.New. An enumeration
// written at a consumer cannot be closed; a stamp applied at the single
// DOOR can, because the door is the only way in.
//
// WHY IT LIVES HERE AND IS CONSTRUCTED ELSEWHERE. Attribution is a FORMAT
// fact — "this face is something the document carries" — so it belongs
// beside the format's other document-attribution error, which it
// generalises and which is one of the things it now wraps. The fault it
// marks, however, is only observable at a render surface: it is minted at
// exactly one site, fontCache.get's embedded arm in package folio8, which
// is the one place a face resolves from the DOCUMENT's own assets rather
// than from a caller's FontSet. Every error out of that arm is stamped —
// not a chosen few — so the set is closed by construction.
//
// WHAT IT DELIBERATELY DOES NOT COVER (D-8.4.12 guardrail 6 — the case
// that DISCRIMINATES the two axes). A face the CALLER's FontSet supplies
// that will not parse fails in get's other arm and is NOT stamped: it is
// the host application's face, not the document's, and no edit an author
// makes on the canvas repairs it. An error-type gate sweeps that case in;
// the attributability axis correctly leaves it out.
//
// IT ADDS NO TEXT, which is what keeps every located string the render
// path already prints byte-identical, and Unwrap keeps every existing
// errors.As/errors.Is target reachable through it — including the
// media-type error the old allowlist named.
type CarriedFaceError struct {
	Err error
}

func (e *CarriedFaceError) Error() string { return e.Err.Error() }

func (e *CarriedFaceError) Unwrap() error { return e.Err }

// UnsupportedFontMediaTypeError is the font analogue of
// UnsupportedMediaTypeError (image.go): the document is VALID — a font
// asset's mediaType is an open set, D-1.4.12/D-1.8.1 as amended — but
// THIS version of the library cannot render this container. Like its
// image twin it is never raised by the loader; only by a render-surface
// caller that actually needs the face.
//
// Story 8.4 widened it from (AssetKey, ElementID, MediaType) to carry
// the whole FontChainSite. The widening is ADDITIVE and the message
// text is safe to change under AD-14 because every consumer matches on
// the TYPE, never on the text (measured: errors.As is the only match in
// the module).
type UnsupportedFontMediaTypeError struct {
	Site      FontChainSite
	MediaType string
}

func (e *UnsupportedFontMediaTypeError) Error() string {
	return fmt.Sprintf(
		"template: %s: this version cannot render font media type %q "+
			"(the document is valid — mediaType is an open set, D-1.4.12; this is a library "+
			"capability limit, not a format error)",
		e.Site, e.MediaType,
	)
}

// decodeRecognisedFont is decodeRecognisedImage's twin, and its doc
// comment carries image.go:98-104's statement forward in substance
// because the statement is what keeps this set out of closedsets.go.
//
// THE SET THIS FUNCTION SWITCHES ON IS A LIBRARY CAPABILITY SET, NOT A
// CLOSED SET OF THE FORMAT. It answers "can THIS build read these
// bytes", which is free to grow release to release; a closed set in
// closedsets.go answers "is this document legal", which under D-1.4.12
// can only be extended by a permanent MAJOR version bump. Putting a
// font media type there would make every new font container — WOFF2
// today, whatever follows it tomorrow — a breaking change to the
// format. D-1.8.1's own amendment note predicted this recurrence "later
// for font formats"; this is that later.
//
// THE POPULATION THE SET WAS MEASURED AGAINST: every mediaType string
// appearing anywhere under fixtures/ at f51dd5e is `image/png` or
// `image/jpeg` — zero font media types ship today — and the faces this
// module itself ships (folio-go/fonts/*/*.ttf, three files) are all
// sfnt-with-glyf, whose wrappers are `font/ttf` and `font/otf` per
// RFC 8081. So the recognised set is exactly those two. `font/woff`,
// `font/woff2` and `font/collection` are deliberately OUTSIDE it: this
// build cannot decompress or de-collection them, they are NOT refused
// at load, and a document carrying one is valid.
//
// Both recognised types describe a SINGLE FACE, which is what lets
// checkSfnt refuse a collection outright. If `font/collection` ever
// joins this set, that refusal has to become conditional on the declared
// type — the two are a pair, and neither moves alone.
//
// recognised reports whether the switch knew the type at all; err is
// non-nil only when it DID and the bytes are not that format.
func decodeRecognisedFont(mediaType string, data []byte) (recognised bool, err error) {
	switch mediaType {
	case "font/ttf", "font/otf":
		return true, checkSfnt(data)
	default:
		return false, nil
	}
}

// DecodeFontForRender is DecodeImageForRender's twin: the ONE named
// predicate that turns an unrecognised media type into an error, and it
// is never called from the load path. Story 8.4 gave it its call site on
// the render path (folio8.fontCache.get, render.go); it is declared here,
// with the set it guards, so that set has exactly one exported door
// rather than acquiring one per consumer.
//
// It answers "can this build read these bytes as a single face", and
// nothing more — building the glyph source from them is internal/fontset's
// job, at the call site, after this returns nil.
func DecodeFontForRender(mediaType string, data []byte, site FontChainSite) error {
	recognised, err := decodeRecognisedFont(mediaType, data)
	if !recognised {
		return &UnsupportedFontMediaTypeError{Site: site, MediaType: mediaType}
	}
	if err != nil {
		return newLoadError("assets."+site.AssetKey, site.ElementID, mediaType, err.Error())
	}
	return nil
}

// sfntVersions are the version tags a SINGLE-FACE sfnt may open with
// (OpenType spec, Table Directory): 0x00010000 for TrueType outlines,
// 'true' for the legacy Apple spelling of the same, and 'OTTO' for CFF
// outlines.
//
// 'ttcf' — a COLLECTION — is deliberately NOT here, and it is called out
// separately below rather than silently omitted. A collection is a
// structurally valid sfnt file and a perfectly legal thing to embed; it
// is simply not what `font/ttf` or `font/otf` describe (RFC 8081 gives a
// collection its own type, `font/collection`), and those two are the
// only media types that reach this function.
var sfntVersions = map[uint32]bool{
	0x00010000: true,
	0x74727565: true, // 'true'
	0x4F54544F: true, // 'OTTO'
}

// sfntCollectionVersion is 'ttcf', the tag a TrueType/OpenType Collection
// opens with. Named rather than spelled inline, because the refusal it
// drives is about a MISMATCH and the message has to be able to say which
// side of it was wrong.
const sfntCollectionVersion = 0x74746366

// checkSfnt is the STRUCTURAL check a recognised font media type gets at
// load — the exact counterpart of image.go's byte sniff, and bounded by
// the same principle: it establishes that the file is not lying about
// what it is, and nothing more. It does not build a glyph source, does
// not read a cmap and does not validate a single table's contents; all
// of that belongs to the shaper, at render — internal/fontset, reached
// from folio8.fontCache.get since Story 8.4.
//
// What it checks: the file opens with a SINGLE-FACE sfnt version tag,
// declares at least one table, carries a whole table directory, and
// every directory entry's (offset, length) lies inside the file. That
// last one is the part worth having — a truncated face passes a
// version-tag-only sniff and then reads out of bounds somewhere far away
// from the loader.
//
// A COLLECTION ('ttcf') is refused here, and the reason is at that
// branch: `mediaType` is author-declared, so a collection labelled
// `font/ttf` really does reach this function.
func checkSfnt(data []byte) error {
	const (
		headerSize = 12
		recordSize = 16
	)
	if len(data) < headerSize {
		return fmt.Errorf("declared font bytes are %d long — too short to carry an sfnt table directory", len(data))
	}
	version := binary.BigEndian.Uint32(data[0:4])
	// The COLLECTION check runs FIRST, before the single-face tag lookup,
	// so a collection is diagnosed as a collection rather than as "not an
	// sfnt at all". It IS an sfnt; it is the wrong one for this media
	// type, and an author told the truth about their bytes and the wrong
	// thing about their label needs to be told which.
	if version == sfntCollectionVersion {
		// A COLLECTION UNDER A SINGLE-FACE MEDIA TYPE IS THE FILE LYING
		// ABOUT ITSELF, which is exactly what this check exists to
		// refuse. `mediaType` is AUTHOR-DECLARED, so `font/ttf` over TTC
		// bytes is recognised, reaches here, and would otherwise load
		// clean — while folio-format.md promises a recognised type whose
		// bytes are not that format is a load error.
		//
		// This branch used to return nil on the stated ground that "no
		// recognised media type reaches here with it anyway". That was
		// an unmeasured negative and it was false: nothing stops an
		// author declaring font/ttf over a collection, and nothing
		// inspects the bytes before this point.
		//
		// It is refused rather than validated because this library
		// cannot draw with a collection either — `font/collection` is
		// not in decodeRecognisedFont's set, so a document that really
		// means to carry one declares that type and loads clean and
		// unread, on D-1.8.1's unrecognised-type path. Refusing here
		// costs such a document nothing and catches the mislabelled one.
		return fmt.Errorf("declared font bytes are a font COLLECTION ('ttcf'), which is not what this media type describes — a collection has its own media type, font/collection (RFC 8081), and this build does not draw with one")
	}
	if !sfntVersions[version] {
		return fmt.Errorf("declared font bytes do not open with a single-face sfnt version tag (got 0x%08x; expected 0x00010000, 'true' or 'OTTO')", version)
	}
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if numTables == 0 {
		return fmt.Errorf("declared font bytes declare zero tables — an sfnt with no table directory cannot be a face")
	}
	if len(data) < headerSize+numTables*recordSize {
		return fmt.Errorf("declared font bytes declare %d tables but are only %d long — the table directory is truncated", numTables, len(data))
	}
	for i := range numTables {
		record := headerSize + i*recordSize
		offset := binary.BigEndian.Uint32(data[record+8 : record+12])
		length := binary.BigEndian.Uint32(data[record+12 : record+16])
		end := uint64(offset) + uint64(length)
		if end > uint64(len(data)) {
			return fmt.Errorf("declared font bytes are truncated: table %d spans bytes %d..%d of a %d-byte file", i, offset, end, len(data))
		}
	}
	return nil
}
