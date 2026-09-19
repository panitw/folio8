package pdf

import (
	"maps"
	"slices"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// EmbeddedFace is one font's resolved, subsetted embedding, ready to be
// written into a PDF (AC5). internal/pdf never touches a vendor font
// type: everything here is plain data that already crossed
// internal/fontset's boundary (AC17a) — this package's job is only to
// spell it out as PDF objects, per AD-3's routing rules.
type EmbeddedFace struct {
	Name           string // PDF resource name (the caller's FontSet key)
	PostScriptName string // the embedded program's OWN name — see baseFont below
	Program        []byte // FontFile2 payload (a standalone TrueType font)
	Tag            string // AC6's six-letter subset tag
	// NumGlyphs is the subset's glyph count, and separately the SIZE OF
	// THE BASE CID BLOCK — CIDs 0..NumGlyphs-1. The two readings coincide
	// by construction rather than by law: nothing keeps the CID space
	// equal to the glyph count once ExtraCIDs is non-empty, and the total
	// CID count is NumGlyphs+len(ExtraCIDs). Read this field as "where
	// the base block ends" wherever a CID is being resolved, and go
	// through glyphForCID rather than assuming the identity (Story 2.3
	// finisher, Finding 10).
	NumGlyphs     int
	WidthForGlyph map[uint16]int64

	// ExtraCIDs is Story 2.3's CID-space extension (D-2.3-Q1, as ruled).
	//
	// CIDs 0..NumGlyphs-1 are the BASE block and map to the subset glyph
	// of the same number — the identity arrangement every folio8 PDF used
	// before this story. ExtraCIDs[i] is the subset glyph that CID
	// NumGlyphs+i points at, so two CIDs may address ONE glyph.
	//
	// This exists because one glyph legitimately needs two different
	// /ToUnicode answers in one document, and one CID cannot carry both.
	// Measured: shaping "น้ำ" (3 runes) yields 4 glyphs whose last is the
	// same glyph a standalone "า" yields. Under D-2.3-Q1(a) the cluster's
	// text sits on the cluster's FIRST glyph and the rest map to the
	// empty string — so that shared glyph must map to "" inside "น้ำ" and
	// to "า" when it stands alone. Mapping it to "" loses every
	// standalone "า"; mapping it to U+0E32 makes "น้ำ" extract as
	// NIKHAHIT + SARA AA, which U+0E33 has NO canonical decomposition to
	// and which NFC never recomposes — so a reader searching the PDF for
	// "น้ำ" finds nothing.
	//
	// /CIDToGIDMap is ours to define, so the fix costs CID space and
	// nothing else: no glyph is duplicated in the embedded program. When
	// ExtraCIDs is empty the map is emitted as /Identity and the bytes
	// are exactly what they were before this story.
	ExtraCIDs []uint16

	// ToUnicode is the /ToUnicode CMap's content, as one record per CID
	// that the content stream actually emits, in ASCENDING CID order.
	// Text may be empty, which is emitted as the empty UTF-16BE string
	// <> — that is a real entry carrying real information ("this glyph
	// contributes no text of its own"), not a missing one.
	ToUnicode []CIDText

	Ascent, Descent, CapHeight             int64
	BBoxXMin, BBoxYMin, BBoxXMax, BBoxYMax int64
	HeadCreated, HeadModified              int64 // AC11a: copied verbatim into the embedded head table
}

// CIDText is one /ToUnicode entry: the text a CID extracts as.
type CIDText struct {
	CID  uint16
	Text string
}

// ImageXObject is one distinct embedded image asset, ready to be
// written as a PDF XObject stream (AC9, AC10): the Stream bytes are
// ALREADY COMPRESSED (the source file's own JPEG or PNG IDAT bytes,
// passed through unchanged — D-1.8.1) and are never decoded or
// re-encoded by this package. Embedded exactly once per document
// regardless of how many elements place it (AC8's dedup-by-key
// definition, mirrored here the same way EmbeddedFace is embedded once
// per document). Stream/Filter here are exactly what
// internal/template.DecodedImage produced (AC10a: see that type's own
// Stream field for the full explanation of why this is never
// re-compressed — carried risk R4, D-1.8.1, AD-22).
type ImageXObject struct {
	Width, Height    int64 // intrinsic pixel dimensions (/Width, /Height — appendInt, D-1.1.b)
	ColorSpace       string
	BitsPerComponent int64
	Filter           string // "DCTDecode" or "FlateDecode", no leading slash
	HasDecodeParms   bool
	PredictorColors  int64
	PredictorBPC     int64
	PredictorColumns int64
	Stream           []byte
}

// SerializeTextDocument writes pages as a classic (uncompressed) PDF 1.7
// document, embedding every distinct face named in faces EXACTLY ONCE
// regardless of how many pages or runs reference it (AC9: "one subset
// per font per document... never per page, never per element" — this
// function is the "never per page" half; the "one subsetting call"
// half is internal/fontset.Font.Subset's caller's job, upstream of
// here). It follows Story 1.1/1.4's determinism rules: every geometric
// number goes through appendLength, every count/offset/object number
// through appendInt/appendIntPadded (AD-3), and /ID is derived from the
// document's own content (AD-7) — no compression.
//
// date is D-3.7.2's /Info dictionary: nil when no documentDate was
// supplied by any route, in which case NO /Info object is emitted at
// all (AC11 — absent, never a present-but-empty dictionary, and never
// a defaulted date) and neither /CreationDate nor /ModDate appears
// anywhere in the produced bytes. When date is non-nil, ONE /Info
// object is emitted carrying BOTH /CreationDate and /ModDate set to
// the SAME value (D-3.7.2: "one value, two keys") and referenced from
// the trailer's /Info entry. date arrives already parsed and validated
// (R4: "the date value must ride into internal/pdf as a VALUE, never
// as an import") — this package never parses an RFC 3339 string and
// never imports internal/expr or "time".
func SerializeTextDocument(pages []pagemodel.Page, faces map[string]EmbeddedFace, images map[string]ImageXObject, date *DocumentDate) ([]byte, error) {
	b := newBuilder()

	catalogID := b.reserve()
	pagesID := b.reserve()
	var infoID int64
	if date != nil {
		infoID = b.reserve()
	}

	// Images are emitted in SORTED resource-name order (same
	// ScanMapRange-compliant idiom as faces below): deterministic
	// object numbers and resource-dict key order (AC18a: asset keys are
	// 64 lowercase hex characters, a strict subset of pdfNameEscape's
	// kept set, so distinct keys cannot collide as resource names —
	// asserted directly in TestAssetKeyEscapeIsIdentity).
	imageNames := slices.Sorted(maps.Keys(images))
	imageIDs := make(map[string]int64, len(imageNames))
	for _, name := range imageNames {
		imageIDs[name] = b.reserve()
	}

	// Faces are emitted in SORTED name order (ScanMapRange-compliant:
	// the escape hatch idiom, not a raw map range) — this reaches
	// output bytes (object numbers, the resource dictionary's key
	// order), same reasoning as AC6/AC7's tag derivation.
	faceNames := slices.Sorted(maps.Keys(faces))

	// Finding 23 (QA review): pdfNameEscape drops every character
	// outside [A-Za-z0-9_-], so two distinct face names can reduce to
	// the same escaped resource name (e.g. "Roboto Regular" and
	// "RobotoRegular" both become "RobotoRegular"). Detect that
	// collision across the WHOLE document's face set up front and fail
	// loudly, rather than silently rendering one face's runs in
	// another's glyphs.
	seenResourceNames := make(map[string]string, len(faceNames))
	for _, name := range faceNames {
		escaped := pdfNameEscape(name)
		if other, collided := seenResourceNames[escaped]; collided {
			return nil, &resourceNameCollisionError{a: other, b: name, resourceName: escaped}
		}
		seenResourceNames[escaped] = name
	}

	type faceObjIDs struct {
		type0, cidFont, descriptor, fontFile, toUnicode, cidToGID int64
	}
	faceIDs := make(map[string]faceObjIDs, len(faceNames))
	for _, name := range faceNames {
		ids := faceObjIDs{
			type0:      b.reserve(),
			cidFont:    b.reserve(),
			descriptor: b.reserve(),
			fontFile:   b.reserve(),
			toUnicode:  b.reserve(),
		}
		// A /CIDToGIDMap STREAM object is reserved only for a face that
		// actually needs one (Story 2.3). Reserving one unconditionally
		// would shift every later object number and move all three
		// pre-2.3 golden hashes for a stream nothing references.
		if len(faces[name].ExtraCIDs) > 0 {
			ids.cidToGID = b.reserve()
		}
		faceIDs[name] = ids
	}

	pageIDs := make([]int64, len(pages))
	contentIDs := make([]int64, len(pages))
	for i := range pages {
		pageIDs[i] = b.reserve()
		contentIDs[i] = b.reserve()
	}

	// --- Catalog ---
	b.begin(catalogID)
	b.write([]byte("<< /Type /Catalog /Pages "))
	b.writeRef(pagesID)
	b.write([]byte(" >>"))
	b.end()

	// --- Pages (parent) ---
	b.begin(pagesID)
	b.write([]byte("<< /Type /Pages /Kids "))
	// The array is emitted from the SLICE, by appendRefArray, which cannot
	// omit the separator between two references. The hand-rolled loop this
	// replaced emitted "[8 0 R10 0 R]" for a two-page document: a tokenizer
	// reads "R10" as one unknown token, so neither kid resolved and the page
	// tree was empty behind a correct-looking /Count.
	b.writeRefArray(pageIDs)
	b.write([]byte(" /Count "))
	b.writeInt(int64(len(pages)))
	b.write([]byte(" >>"))
	b.end()

	// --- Info (D-3.7.2, AC8-AC11) — emitted ONLY when date != nil ---
	if date != nil {
		b.begin(infoID)
		b.write([]byte("<< /CreationDate "))
		b.write(appendPDFDate(nil, *date))
		b.write([]byte(" /ModDate "))
		b.write(appendPDFDate(nil, *date))
		b.write([]byte(" >>"))
		b.end()
	}

	// --- Per-page objects ---
	for i, page := range pages {
		// Story 4.1: rects are drawn FIRST — a cell's background and
		// border sit behind its label — so the content stream's
		// prefix, not its suffix, carries them. Every pre-4.1 document
		// has page.Rects == nil, so this appends zero bytes and every
		// existing golden stays byte-identical.
		content := appendRectContentStream(nil, page)
		textContent, cerr := buildTextContentStream(page, faces)
		if cerr != nil {
			return nil, cerr
		}
		content = append(content, textContent...)
		content, cerr = appendImageContentStream(content, page, imageIDs)
		if cerr != nil {
			return nil, cerr
		}

		b.begin(pageIDs[i])
		b.write([]byte("<< /Type /Page /Parent "))
		b.writeRef(pagesID)
		b.write([]byte(" /MediaBox [0 0 "))
		b.writeLength(page.Width)
		b.write([]byte(" "))
		b.writeLength(page.Height)
		b.write([]byte("] /Resources << /Font <<"))
		for _, name := range faceNames {
			b.write([]byte(" /"))
			b.write([]byte(pdfNameEscape(name)))
			b.write([]byte(" "))
			b.writeRef(faceIDs[name].type0)
		}
		b.write([]byte(" >>"))
		if len(imageNames) > 0 {
			b.write([]byte(" /XObject <<"))
			for _, name := range imageNames {
				b.write([]byte(" /"))
				b.write([]byte(pdfNameEscape(name)))
				b.write([]byte(" "))
				b.writeRef(imageIDs[name])
			}
			b.write([]byte(" >>"))
		}
		b.write([]byte(" >> /Contents "))
		b.writeRef(contentIDs[i])
		b.write([]byte(" >>"))
		b.end()

		b.begin(contentIDs[i])
		b.write([]byte("<< /Length "))
		b.writeInt(int64(len(content)))
		b.write([]byte(" >>\nstream\n"))
		b.write(content)
		b.write([]byte("endstream"))
		b.end()
	}

	// --- Per-face font objects ---
	for _, name := range faceNames {
		face := faces[name]
		ids := faceIDs[name]
		// ISO 32000-1 Table 117 (CIDFontType2): /BaseFont "shall be the
		// value of the CIDFontName entry in the CIDFont program",
		// prefixed with the six-letter subset tag (§9.6.4). Before
		// Story 2.2 folio8 spelled this from the FontSet KEY — the name
		// the caller happened to file the face under — so the declared
		// name and the embedded program disagreed (`NotoSansSC` vs
		// `NotoSansSC-Regular`). PDF/A validators flag that, and it is
		// the name a viewer falls back to when the embedded program
		// fails to load.
		//
		// The two halves are INDEPENDENT, which is what keeps the tag
		// derivation non-circular (AC6): the tag hashes the final
		// program bytes, while PostScriptName is read off the supplied
		// face's own `name` table. Neither is an input to the other, and
		// the embedded program carries no `name` table for a tag to be
		// written into (PDF §9.9.2 sanctions the reduced table set for
		// an embedded CIDFontType2, since /CIDToGIDMap does the
		// mapping). The semantic acceptance assertion pins the composed
		// result to ^[A-Z]{6}\+<name6>$ per face, which catches both a
		// stale name and a double-applied tag.
		//
		// D-2.3a.2 (binding): when the supplied program declares no name
		// record at all, /BaseFont still carries the FontSet key — but
		// the substitution is NAMED IN THE PDF rather than performed
		// silently.
		//
		// Why the key still goes there. /BaseFont is Required by Table
		// 117 and must be a legal name, so something has to be written,
		// and the FontSet key is genuinely true of the face folio8
		// loaded. Rejecting a nameless program is not available either:
		// Story 2.2 deliberately tolerates one (fontset.readPostScriptName
		// returns "", which is observably absent, and that is a ruled
		// disposition).
		//
		// Why silence is not available. D-2.2.6 bought /BaseFont
		// specifically to make an invisible property visible — a
		// Thin-named program shows as NotoSansSC-Thin, which is how the
		// Thin defect would have been legible by reading the PDF. Under
		// a silent fallback a NAMELESS program is indistinguishable from
		// a correctly-named one, so a reader cannot tell whether
		// /BaseFont reflects the embedded program or reflects us —
		// which defeats the property the entry was bought for. AD-8's
		// "not a silent substitution" governs, and it costs one
		// diagnostic.
		//
		// Why the diagnostic is a PDF comment and not a code comment. A
		// code comment documents the behaviour for a maintainer; it does
		// nothing for whoever reads the PDF, and that reader is the
		// audience D-2.2.6 was written for. A `%` comment between two
		// indirect objects is legal PDF (ISO 32000-1 §7.2.4), is skipped
		// as whitespace by every conforming reader, is greppable in the
		// bytes, and is written BEFORE b.begin so the object's recorded
		// xref offset is the offset of "N 0 obj", unchanged.
		//
		// It is byte-neutral for every face that has a name record, and
		// every face folio8 ships or tests with has one — so no fixture
		// digest moves.
		psName := face.PostScriptName
		substituted := psName == ""
		if substituted {
			psName = name
		}
		baseFont := face.Tag + "+" + pdfNameEscape(psName)

		if substituted {
			b.write([]byte("% folio8: /BaseFont "))
			b.write([]byte(baseFont))
			b.write([]byte(" names the FontSet key, NOT the embedded program: this CIDFont program declares no `name` record, so it has no CIDFontName of its own (ISO 32000-1 Table 117).\n"))
		}

		toUnicodeCMap, uerr := buildToUnicodeCMap(face)
		if uerr != nil {
			return nil, uerr
		}

		b.begin(ids.type0)
		b.write([]byte("<< /Type /Font /Subtype /Type0 /BaseFont /"))
		b.write([]byte(baseFont))
		b.write([]byte(" /Encoding /Identity-H /DescendantFonts "))
		// Routed through writeRefArray, not hand-rolled "[" + writeRef +
		// "]", per Story 2.6 finisher's Finding 3: /DescendantFonts is a
		// second ref array in this module, live in most goldens, and was
		// the other hand-rolled site the "no other ref array exists yet"
		// premise was measurably false about. Byte-identical for this
		// always-one-element array (leading separator design).
		b.writeRefArray([]int64{ids.cidFont})
		b.write([]byte(" /ToUnicode "))
		b.writeRef(ids.toUnicode)
		b.write([]byte(" >>"))
		b.end()

		b.begin(ids.cidFont)
		b.write([]byte("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /"))
		b.write([]byte(baseFont))
		b.write([]byte(" /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >>"))
		b.write([]byte(" /FontDescriptor "))
		b.writeRef(ids.descriptor)
		b.write([]byte(" /DW "))
		b.writeInt(1000)
		b.write([]byte(" /W ["))
		b.writeInt(0)
		b.write([]byte(" ["))
		// /W is indexed by CID, not by glyph. For the BASE block those
		// coincide; each extra CID repeats the width of the glyph it
		// points at, because it IS that glyph (Story 2.3).
		//
		// Both loops take the `, ok` presence check that appendShapedRun
		// already takes, and for the same reason (Story 2.3 finisher,
		// Finding 12): a missing width read off a Go map yields 0, and a
		// zero-width glyph is output that LOOKS healthy — the "broken
		// output and healthy output are the same bytes" shape this file's
		// posture is fail-closed against everywhere else. WidthForGlyph
		// is dense over 0..NumGlyphs-1 today, so neither branch is
		// reachable; the point is that the file has one posture rather
		// than two.
		for cid := 0; cid < face.NumGlyphs; cid++ {
			if cid > 0 {
				b.write([]byte(" "))
			}
			w, ok := face.WidthForGlyph[uint16(cid)]
			if !ok {
				return nil, &missingGlyphError{face: face.Name, cid: uint16(cid)}
			}
			b.writeInt(w)
		}
		for _, gid := range face.ExtraCIDs {
			b.write([]byte(" "))
			w, ok := face.WidthForGlyph[gid]
			if !ok {
				return nil, &missingGlyphError{face: face.Name, cid: gid}
			}
			b.writeInt(w)
		}
		b.write([]byte("]] /CIDToGIDMap "))
		if len(face.ExtraCIDs) == 0 {
			// Identity: exactly the bytes every folio8 PDF carried before
			// Story 2.3, emitted whenever no glyph needed a second CID.
			b.write([]byte("/Identity >>"))
		} else {
			b.writeRef(ids.cidToGID)
			b.write([]byte(" >>"))
		}
		b.end()

		if len(face.ExtraCIDs) > 0 {
			cidToGIDMap := buildCIDToGIDMap(face)
			b.begin(ids.cidToGID)
			b.write([]byte("<< /Length "))
			b.writeInt(int64(len(cidToGIDMap)))
			b.write([]byte(" >>\nstream\n"))
			b.write(cidToGIDMap)
			b.write([]byte("endstream"))
			b.end()
		}

		b.begin(ids.descriptor)
		b.write([]byte("<< /Type /FontDescriptor /FontName /"))
		b.write([]byte(baseFont))
		b.write([]byte(" /Flags "))
		b.writeInt(4) // bit 3 (Symbolic) — this is a CID-keyed subset, not one of the 14 standard fonts.
		b.write([]byte(" /FontBBox ["))
		b.writeInt(face.BBoxXMin)
		b.write([]byte(" "))
		b.writeInt(face.BBoxYMin)
		b.write([]byte(" "))
		b.writeInt(face.BBoxXMax)
		b.write([]byte(" "))
		b.writeInt(face.BBoxYMax)
		b.write([]byte("] /ItalicAngle "))
		// Finding 22 (QA review): route through writeInt rather than a
		// bare "0" literal — D-1.1.b names /ItalicAngle explicitly as a
		// "convert to thousandths, decimal route" value. It is a
		// constant 0 today (no italic faces are tested), but the shape
		// is already correct for the day the value stops being constant.
		b.writeInt(0)
		b.write([]byte(" /Ascent "))
		b.writeInt(face.Ascent)
		b.write([]byte(" /Descent "))
		b.writeInt(face.Descent)
		b.write([]byte(" /CapHeight "))
		b.writeInt(face.CapHeight)
		b.write([]byte(" /StemV "))
		b.writeInt(80)
		b.write([]byte(" /FontFile2 "))
		b.writeRef(ids.fontFile)
		b.write([]byte(" >>"))
		b.end()

		b.begin(ids.fontFile)
		b.write([]byte("<< /Length "))
		b.writeInt(int64(len(face.Program)))
		b.write([]byte(" /Length1 "))
		b.writeInt(int64(len(face.Program)))
		b.write([]byte(" >>\nstream\n"))
		b.write(face.Program)
		b.write([]byte("endstream"))
		b.end()

		b.begin(ids.toUnicode)
		b.write([]byte("<< /Length "))
		b.writeInt(int64(len(toUnicodeCMap)))
		b.write([]byte(" >>\nstream\n"))
		b.write(toUnicodeCMap)
		b.write([]byte("endstream"))
		b.end()
	}

	// --- Per-image XObject objects ---
	for _, name := range imageNames {
		img := images[name]
		b.begin(imageIDs[name])
		b.write([]byte("<< /Type /XObject /Subtype /Image /Width "))
		b.writeInt(img.Width)
		b.write([]byte(" /Height "))
		b.writeInt(img.Height)
		b.write([]byte(" /ColorSpace /"))
		b.write([]byte(img.ColorSpace))
		b.write([]byte(" /BitsPerComponent "))
		b.writeInt(img.BitsPerComponent)
		b.write([]byte(" /Filter /"))
		b.write([]byte(img.Filter))
		if img.HasDecodeParms {
			b.write([]byte(" /DecodeParms << /Predictor 15 /Colors "))
			b.writeInt(img.PredictorColors)
			b.write([]byte(" /BitsPerComponent "))
			b.writeInt(img.PredictorBPC)
			b.write([]byte(" /Columns "))
			b.writeInt(img.PredictorColumns)
			b.write([]byte(" >>"))
		}
		b.write([]byte(" /Length "))
		b.writeInt(int64(len(img.Stream)))
		b.write([]byte(" >>\nstream\n"))
		b.write(img.Stream)
		b.write([]byte("endstream"))
		b.end()
	}

	return b.finish(infoID), nil
}

// pdfNameEscape returns name with characters outside a conservative safe
// set (ASCII letters, digits, hyphen, underscore) replaced with nothing
// — enough for the face names and tags this story constructs; PDF name
// escaping (#xx) is not needed for the identifiers this story produces.
func pdfNameEscape(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "F"
	}
	return string(out)
}

// bfcharSectionCap is DW-14's cap: ISO 32000-1 §9.10.3 (the ToUnicode
// CMap clause, which incorporates the Adobe CMap and CIDFont
// specifications' operator limits) REQUIRES that a single
// `beginbfchar`/`endbfchar` section carry no more than 100 entries —
// this is not implementation-defined guidance, and several
// third-party PDF readers are known to mis-parse a section that
// exceeds it. Story 4.2 is the first story to hand buildToUnicodeCMap
// a face whose ToUnicode table can exceed this at all in principle
// (D1's measurement: the largest section shipped anywhere in this
// repository is 45, on a two-page document).
//
// This is a NAMED constant precisely so the corpus-wide witness
// (package folio8's TestNoRealToUnicodeSectionExceedsTheCap) and the
// direct chunking witness (internal/pdf/tounicode_chunk_test.go) can
// each state their own expectation as an INDEPENDENT literal — never
// derived from this constant — so a wrong value here cannot pass
// either test by construction (D-000.9).
const bfcharSectionCap = 100

// buildToUnicodeCMap builds the ToUnicode CMap stream — one bfchar
// entry per CID the content stream actually emits, mapping it back to
// the text it extracts as. This is what an independent reader uses to
// get correct text out of the rendered PDF (AC7), and what this story's
// round-trip assertion checks.
//
// Story 2.3 changed where the entries come from, and only that. Before
// shaping, the map was the inverse of rune -> subset glyph, which was
// well defined because every glyph came from exactly one rune. Shaping
// breaks that assumption in both directions — "office" draws four
// glyphs for six runes, "น้ำ" draws four glyphs for three — so the
// entries are now supplied by the caller (EmbeddedFace.ToUnicode),
// which is the only place that knows each glyph's cluster. A ligature
// or a substituted mark form has no rune of its own and would have got
// NO entry at all under the old derivation: the text would silently
// stop being selectable, which nobody notices until they try to copy it.
//
// Entries arrive in ascending CID order and are emitted in that order.
// An entry whose Text is empty is emitted as the empty UTF-16BE string
// <> — the non-first glyphs of a merged cluster carry no text of their
// own, and saying so explicitly is what makes "น้ำ" extract once
// instead of four times.
//
// Basic Multilingual Plane only (runes <= 0xFFFF); a supplementary-plane
// rune would need a UTF-16 surrogate pair, which no shipped face's
// coverage reaches in this story's fixtures — and which is reported as a
// located error rather than silently truncated.
//
// DW-14 (Story 4.2): entries are emitted in consecutive
// bfcharSectionCap-sized sections rather than one unbounded section —
// see that constant's own comment for why.

func buildToUnicodeCMap(face EmbeddedFace) ([]byte, error) {
	var c []byte
	c = append(c, "/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n"...)
	c = append(c, "/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n"...)
	c = append(c, "/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n"...)
	c = append(c, "1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n"...)

	// DW-14: chunked into consecutive sections of at most
	// bfcharSectionCap entries each, in the SAME ascending-CID order
	// face.ToUnicode already carries, every entry present exactly once.
	// Byte-identical to the pre-chunking single-section form for any
	// face whose ToUnicode table is at or under the cap (measured:
	// 965/0/1, unchanged, at cap 100 — see the story's Delivery Log).
	//
	// A face with ZERO entries still emits exactly one (empty) section
	// — the pre-chunking behaviour, preserved byte-for-byte — rather
	// than none at all: `start < len(...)` alone would skip the loop
	// body entirely for an empty ToUnicode table.
	sections := (len(face.ToUnicode) + bfcharSectionCap - 1) / bfcharSectionCap
	if sections == 0 {
		sections = 1
	}
	for i := 0; i < sections; i++ {
		start := i * bfcharSectionCap
		end := start + bfcharSectionCap
		if end > len(face.ToUnicode) {
			end = len(face.ToUnicode)
		}
		section := face.ToUnicode[start:end]
		c = appendInt(c, int64(len(section)))
		c = append(c, " beginbfchar\n"...)
		for _, e := range section {
			c = append(c, '<')
			c = appendHex4(c, e.CID)
			c = append(c, "> <"...)
			for _, r := range e.Text {
				if r > 0xFFFF {
					return nil, &supplementaryPlaneError{face: face.Name, r: r}
				}
				c = appendHex4(c, uint16(r))
			}
			c = append(c, ">\n"...)
		}
		c = append(c, "endbfchar\n"...)
	}
	c = append(c, "endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n"...)
	return c, nil
}

// buildCIDToGIDMap emits the /CIDToGIDMap stream: two big-endian bytes
// per CID, giving the subset glyph that CID addresses. CIDs
// 0..NumGlyphs-1 are the identity block; the extras follow, each
// pointing at a glyph the base block already names (Story 2.3 — see
// EmbeddedFace.ExtraCIDs for why two CIDs may share one glyph).
//
// D-1.1.b's routing rule applies here exactly as it does to a hex CID
// string: these are a byte encoding, not numbers, so they do not go
// through numbers.go's integer emitters.
func buildCIDToGIDMap(face EmbeddedFace) []byte {
	out := make([]byte, 0, 2*(face.NumGlyphs+len(face.ExtraCIDs)))
	for gid := 0; gid < face.NumGlyphs; gid++ {
		out = append(out, byte(uint16(gid)>>8), byte(uint16(gid)))
	}
	for _, gid := range face.ExtraCIDs {
		out = append(out, byte(gid>>8), byte(gid))
	}
	return out
}

type supplementaryPlaneError struct {
	face string
	r    rune
}

// Error names the offending rune as U+XXXX rather than embedding the
// character itself. The whole subject of this error is a rune the
// pipeline cannot encode, so it may equally be unrenderable in the
// terminal or log that reads the message — and every other located error
// in this codebase names its subject numerically (Story 2.3 finisher,
// Finding 6).
//
// The formatting is hand-rolled byte arithmetic rather than fmt.Sprintf
// or strconv, because emit_source_test.go's scanNumericFormatting
// forbids EVERY fmt formatting call and the strconv Format*/Itoa/Append*
// family anywhere under internal/pdf except numbers.go, with no
// diagnostic exemption (D-1.1.b; D-1.3.2 kept the no-exemption choice
// deliberately). Reaching for fmt here would have traded a nit for a
// guard violation.
func (e *supplementaryPlaneError) Error() string {
	return "internal/pdf: face " + e.face + ": /ToUnicode cannot yet express rune " +
		codepointLabel(e.r) + " above the Basic Multilingual Plane (a UTF-16 surrogate pair is not implemented)"
}

// codepointLabel renders r in the standard Unicode notation, U+ followed
// by at least four uppercase hex digits (more for a codepoint above
// U+FFFF, which is precisely the case this label is used for). Pure byte
// arithmetic, on the same basis as appendHex4 below.
func codepointLabel(r rune) string {
	const hexDigits = "0123456789ABCDEF"
	v := uint32(r)
	// Build most-significant digit first into a fixed buffer; a rune is
	// at most U+10FFFF, so six digits always suffice.
	var buf [6]byte
	n := 0
	for shift := 20; shift >= 0; shift -= 4 {
		d := byte(v>>uint(shift)) & 0x0F
		if n == 0 && d == 0 && shift > 12 {
			// Leading zero above the four-digit minimum: skip it.
			continue
		}
		buf[n] = hexDigits[d]
		n++
	}
	return "U+" + string(buf[:n])
}

// appendHex4 encodes v as a 4-hex-digit big-endian byte pair. D-1.1.b,
// verbatim, on why this is not routed through numbers.go's appendInt:
// "Glyph ids under Identity-H take neither route — they are a
// big-endian hex pair inside a string literal, i.e. a byte encoding
// like /ID, not a number. Same for the six-letter subset tag. This
// must be said in a code comment or a later agent will 'unify' it."
func appendHex4(dst []byte, v uint16) []byte {
	return append(dst,
		hexDigits[(v>>12)&0xf], hexDigits[(v>>8)&0xf],
		hexDigits[(v>>4)&0xf], hexDigits[v&0xf],
	)
}

// buildTextContentStream renders one page's text runs as a content
// stream: for each run, select the resource font, position with an
// absolute text matrix (Tm — avoids accumulating relative Td offsets
// across runs), and show its text as a hex string of 2-byte CIDs
// (Identity-H).
//
// CID == subset glyph id for the BASE block only; extras resolve through
// glyphForCID. The comment used to state the identity unconditionally,
// which is precisely the assumption ExtraCIDs falsified — sitting three
// lines from glyphForCID, which exists BECAUSE it is no longer true
// (Story 2.3 finisher, Finding 10). D-2.3.2 closes with "anything that
// assumes CID<->GID is 1:1, or that inverts the map, must be checked":
// the code was checked and this comment was not, and this file already
// quotes D-1.1.b's warning that an invariant "must be said in a code
// comment or a later agent will 'unify' it". A comment asserting the
// FALSE version of an invariant is that same hazard pointed the other
// way.
//
// Origin (PERMANENT, not provisional): run.X/run.Y are PAGE-ABSOLUTE
// offsets from the page's top-left printable corner, Y increasing
// DOWNWARD, as pagemodel.TextRun documents. They are NOT band-relative
// and must never become so — under AD-24 bands are placed on the page by
// internal/layout alone, so a band-relative TextRun would move band
// placement into this package and violate AD-24 outright. internal/layout
// has already added the band origin before a page model exists.
//
// Converting that top-down Y into PDF's bottom-up user space is this
// package's own business and happens in exactly one function, flipY
// (AD-24's one-and-only inverter, ratified by D-1.8.10):
//
//	pdfX = marginLeft + run.X
//	pdfY = flipY(pageHeight, marginTop, run.Y, run.BaselineOffset)
//
// The fourth term places the baseline that far below the run's top, and
// it is a CARRIED LAYOUT QUANTITY — pagemodel.TextRun.BaselineOffset,
// resolved by package folio8 from the element's DECLARED font chain
// before this package sees the page.
//
// IT IS NO LONGER run.FontSize, AND MUST NEVER BECOME SO AGAIN. That
// was DW-15: the point size used as a proxy for an ascent it has no
// relationship to, so the first baseline and D-2.4.2's inter-baseline
// advance answered one geometric question from two unrelated sources
// and agreed only by accident. Story 2.5a fixed it, re-recording five
// goldens as one attributable movement.
//
// NOR MAY IT BE RE-DERIVED HERE FROM faces[run.Face]. Two reasons, and
// both are structural rather than stylistic:
//
//   - AD-5. Placement is decided before a renderer sees it. A renderer
//     computing a baseline is a renderer deciding layout.
//   - AD-24. The offset is a function of the element's DECLARED chain,
//     not of the face a particular run resolved to. Deriving it per-run
//     from the resolved face would make it content-dependent — adding
//     one CJK character would reflow the element.
//
// run.FontSize is still used, one line below, as the Tf operand. That
// use is the actual font size in the content stream and is correct; it
// is a DIFFERENT job that happens to have had the same value.
func buildTextContentStream(page pagemodel.Page, faces map[string]EmbeddedFace) ([]byte, error) {
	var c []byte
	for _, run := range page.Runs {
		if len(run.Glyphs) == 0 {
			continue
		}
		face, ok := faces[run.Face]
		if !ok {
			return nil, &missingFaceError{face: run.Face}
		}

		pdfX := page.MarginLeft + run.X
		pdfY := flipY(page.Height, page.MarginTop, run.Y, run.BaselineOffset)

		body, berr := appendShapedRun(nil, run, face)
		if berr != nil {
			return nil, berr
		}

		// Story 2.8, FR44/D-2.8.1: ClipToBox wraps this run's BT..ET
		// pair in a "q ... re W n ... Q" clip, restricted to the
		// element's declared box on the HORIZONTAL axis alone. The
		// clip rectangle's vertical extent is the WHOLE PAGE (0 to
		// page.Height, in this content stream's own bottom-up user
		// space) — never anything derived from the element's own
		// declared height, which D-2.8.1 rules is not a clip bound.
		// "q"/"Q" bracket the clip so it never leaks into the next
		// run's drawing.
		clipped := run.ClipToBox
		if clipped {
			clipX := page.MarginLeft + run.ClipX
			c = append(c, "q\n"...)
			c = appendLength(c, clipX)
			c = append(c, " 0 "...)
			c = appendLength(c, run.ClipWidth)
			c = append(c, ' ')
			c = appendLength(c, page.Height)
			c = append(c, " re\nW n\n"...)
		}

		// Story 10.1: the run's own ink. Bracketed in q/Q — the same
		// discipline rectdoc.go's fill and stroke halves use — so a
		// coloured run never leaves its colour in the graphics state for
		// whatever draws next. A run with no colour emits nothing here,
		// so a document that declares none is byte-identical to the one
		// this renderer produced before the field existed.
		coloured := run.HasColor
		if coloured {
			c = append(c, "q\n"...)
			c = appendColorChannels(c, run.Color)
			c = append(c, " rg\n"...)
		}

		c = append(c, "BT\n/"...)
		c = append(c, pdfNameEscape(run.Face)...)
		c = append(c, ' ')
		c = appendLength(c, run.FontSize)
		c = append(c, " Tf\n1 0 0 1 "...)
		c = appendLength(c, pdfX)
		c = append(c, ' ')
		c = appendLength(c, pdfY)
		c = append(c, " Tm\n"...)
		c = append(c, body...)
		c = append(c, "ET\n"...)

		if coloured {
			c = append(c, "Q\n"...)
		}
		if clipped {
			c = append(c, "Q\n"...)
		}
	}
	return c, nil
}

// appendShapedRun writes one shaped run's show-text operators (AC6).
//
// A glyph carries three numbers the plain Tj operator cannot express:
// an x-offset (GPOS mark positioning), an x-advance that may differ from
// the glyph's own /W width (GPOS kerning), and a y-offset. The first two
// become integer adjustments inside a TJ array; the third becomes PDF's
// text-rise operator Ts, which is inside AD-6's pinned profile.
//
// The arithmetic, per glyph i, with W_i the glyph's /W width:
//
//	before glyph i:  -XOffset_i
//	after  glyph i:  XOffset_i + (W_i - XAdvance_i)
//
// A TJ number moves the pen LEFT by number/1000 of the font size, which
// is why the signs read backwards from the offsets they express: the
// "before" term shifts the glyph RIGHT by XOffset, and the "after" term
// undoes that shift and then corrects the advance the glyph itself
// consumed. Adjacent terms are summed and a zero term is omitted, so a
// run whose every adjustment computes to zero emits
//
//	<hex...> Tj
//
// — EXACTLY the bytes folio8 emitted before Story 2.3. That is not an
// optimisation: it is what keeps the three pre-2.3 golden fixtures
// byte-identical (measured: none of their text is shape-observable), and
// it is the condition under which AC6 and AC14 are both true at once.
//
// THE VERTICAL OFFSET (Story 8.0). Ts sets the text rise, a persistent
// text-state parameter in text space, y-up — the same convention and the
// same sign the shaper's YOffset already carries (render.go's producer
// never negates it). The rise is
//
//	geom.ScaleRound(run.FontSize, g.YOffset, 1000)
//
// — millipoints, integer, round-half-to-even, no float intermediate
// (AD-23), which is what makes all four AD-21 targets agree on it.
//
// Because Ts is text state rather than a per-glyph parameter, the run is
// PARTITIONED into maximal contiguous segments of equal rise, walked in
// index order, and each segment gets its own show-text operator. The
// rise in effect is 0 on entry to every run; a segment whose rise
// differs from the rise in effect emits "<rise> Ts" first, and after the
// last segment a non-zero rise is restored to "0 Ts". The restore is
// NOT optional and NOT provided by anything else: buildTextContentStream
// brackets a run in q/Q only when it is clipped or coloured, and text
// rise survives ET regardless, so a run that left the rise set would
// displace whatever drew next.
//
// The adjustment term sitting BEFORE glyph i leads the segment glyph i
// opens; the trailing term belongs to the final segment. Ts moves no
// pen, so moving a term across a segment boundary changes nothing about
// where the glyphs land.
//
// A run whose every glyph carries YOffset 0 is ONE segment with rise 0:
// no Ts operator is emitted, no restore is emitted, and the bytes are
// exactly the bytes this function produced before Story 8.0. That is the
// whole of this story's byte-identity claim and it is asserted, not
// assumed — TestZeroOffsetRunIsByteIdenticalToItsRisenTwin.
//
// The fail-closed branch NARROWS rather than vanishes: see
// verticalOffsetError.
//
// Every number here reaches the output through numbers.go's emitters
// (AD-3: "no number reaches an output byte by any other route") —
// appendInt for the TJ adjustments, appendLength for the rise. The CIDs
// do not, and must not: D-1.1.b, verbatim — "Glyph ids under Identity-H
// take neither route — they are a big-endian hex pair inside a string
// literal, i.e. a byte encoding like /ID, not a number. Same for the
// six-letter subset tag. This must be said in a code comment or a later
// agent will 'unify' it."
func appendShapedRun(dst []byte, run pagemodel.TextRun, face EmbeddedFace) ([]byte, error) {
	// adjustments[i] is the combined term that sits BEFORE glyph i;
	// adjustments[len] is the trailing term after the last glyph, which
	// is emitted so the run's total advance is exactly the shaped
	// advance rather than the sum of the /W widths.
	adjustments := make([]int64, len(run.Glyphs)+1)
	// rises[i] is glyph i's text rise in millipoints. Zero for the
	// overwhelming majority of glyphs, and a whole run of zeroes is the
	// case that must emit today's bytes.
	rises := make([]geom.Length, len(run.Glyphs))
	for i, g := range run.Glyphs {
		if g.YOffset != 0 {
			rise := geom.ScaleRound(run.FontSize, g.YOffset, 1000)
			if rise == 0 {
				// FAIL CLOSED, on the ONE condition left after Ts.
				//
				// The branch that stood here refused EVERY non-zero
				// YOffset, and the comment above it said the refusal was
				// unreachable through the render path. Both were wrong,
				// and both were load-bearing. The unreachability claim
				// was measured over Story 2.3's OWN SAMPLES and reported
				// over THE SHIPPED SET — two different populations — and
				// ordinary Thai (ทั้งสิ้น, ครั้ง, ทั้งนี้, ตั้งแต่) reaches
				// it on the shipped Noto Sans Thai, which is why a large
				// class of ordinary Thai legal prose did not render at
				// all (DW-28). The refusal is now the fix's own
				// boundary rather than the feature's ceiling.
				//
				// What is left to catch: the offset is real but the rise
				// ROUNDS AWAY to zero. Emitting "0 Ts" there would drop
				// the offset silently, and the healthy output and the
				// broken output would be the same bytes — the posture
				// this project keeps getting burned by.
				//
				// IT IS REACHABLE, and the population that claim is
				// measured over is: documents parsed by the shipped
				// parser and rendered by the shipped renderer. fontSize
				// has no positivity floor at parse (parse.go's
				// decodePoints), so a stacked-mark document at
				// fontSize 0.008 loads and arrives here with
				// ScaleRound(8, -57, 1000) == 0. It is red-proved
				// through a real document by thai_mark_stacking_test.go
				// and directly by textdoc_test.go. At any fontSize of
				// 1pt or more, |rise| >= |YOffset| millipoints and is
				// never zero, so no ordinary document reaches it.
				return nil, &verticalOffsetError{face: face.Name, cid: g.CID, offset: g.YOffset, fontSize: run.FontSize}
			}
			rises[i] = rise
		}
		width, ok := face.WidthForGlyph[glyphForCID(face, g.CID)]
		if !ok {
			return nil, &missingGlyphError{face: face.Name, cid: g.CID}
		}
		adjustments[i] += -g.XOffset
		adjustments[i+1] += g.XOffset + (width - g.XAdvance)
	}

	if len(run.Glyphs) == 0 {
		// Degenerate, and kept here so the segment walk below never has
		// to reason about an empty partition: no glyphs means no rise
		// and the same empty show-text operator this function emitted
		// before Story 8.0. buildTextContentStream skips a run this
		// shape before it ever calls here.
		return appendShowText(dst, nil, adjustments), nil
	}

	// The rise in effect. Zero on entry to every run, and restored to
	// zero before the caller's ET.
	var current geom.Length
	for start := 0; start < len(run.Glyphs); {
		end := start + 1
		for end < len(run.Glyphs) && rises[end] == rises[start] {
			end++
		}

		if rises[start] != current {
			dst = appendLength(dst, rises[start])
			dst = append(dst, " Ts\n"...)
			current = rises[start]
		}

		// terms[j] is the adjustment before this segment's glyph j.
		// terms[len] is the trailing term, which exists ONLY for the
		// final segment: for any other segment adjustments[end] is the
		// term that leads the NEXT segment, and emitting it here as well
		// would double it.
		terms := make([]int64, end-start+1)
		copy(terms, adjustments[start:end])
		if end == len(run.Glyphs) {
			terms[end-start] = adjustments[end]
		}
		dst = appendShowText(dst, run.Glyphs[start:end], terms)
		start = end
	}
	if current != 0 {
		// The restore goes through appendLength like every other number
		// this function emits (AD-3: "no number reaches an output byte
		// by any other route"). appendLength(0) is exactly "0" — one
		// digit, no sign, no fractional part — so this is byte-identical
		// to the literal it replaces, and the invariant stated in the
		// doc comment above is now literally true of every number here
		// rather than true of all but one.
		dst = appendLength(dst, 0)
		dst = append(dst, " Ts\n"...)
	}
	return dst, nil
}

// appendShowText writes ONE show-text operator for one segment of a run:
// a bare "<hex…> Tj" when every one of that segment's terms is zero, and
// a "[…] TJ" array otherwise. terms has one more entry than glyphs — the
// leading term of each glyph, then the segment's trailing term.
//
// This is Story 2.3's emitter, moved out of appendShapedRun's body
// unchanged so that a run split by text rise emits each segment by the
// identical rule. A one-segment run therefore emits the identical bytes.
func appendShowText(dst []byte, glyphs []pagemodel.ShapedGlyph, terms []int64) []byte {
	anyAdjustment := false
	for _, a := range terms {
		if a != 0 {
			anyAdjustment = true
			break
		}
	}

	if !anyAdjustment {
		dst = append(dst, '<')
		for _, g := range glyphs {
			dst = appendHexCID(dst, g.CID)
		}
		dst = append(dst, "> Tj\n"...)
		return dst
	}

	dst = append(dst, '[')
	open := false
	for i, g := range glyphs {
		if terms[i] != 0 {
			if open {
				dst = append(dst, '>')
				open = false
			}
			dst = appendInt(dst, terms[i])
		}
		if !open {
			dst = append(dst, '<')
			open = true
		}
		dst = appendHexCID(dst, g.CID)
	}
	if open {
		dst = append(dst, '>')
	}
	if trailing := terms[len(glyphs)]; trailing != 0 {
		dst = appendInt(dst, trailing)
	}
	dst = append(dst, "] TJ\n"...)
	return dst
}

// glyphForCID resolves a CID back to the subset glyph it addresses:
// the identity for the base block, and ExtraCIDs for anything above it
// (Story 2.3). A CID beyond both is returned unchanged and then fails
// the WidthForGlyph presence check at the call site, which is a located
// error rather than a silently wrong width.
func glyphForCID(face EmbeddedFace, cid uint16) uint16 {
	if int(cid) < face.NumGlyphs {
		return cid
	}
	idx := int(cid) - face.NumGlyphs
	if idx < len(face.ExtraCIDs) {
		return face.ExtraCIDs[idx]
	}
	return cid
}

type resourceNameCollisionError struct{ a, b, resourceName string }

func (e *resourceNameCollisionError) Error() string {
	return "internal/pdf: face names " + e.a + " and " + e.b +
		" both escape to the PDF resource name " + e.resourceName + " (Finding 23, QA review)"
}

type missingFaceError struct{ face string }

func (e *missingFaceError) Error() string {
	return "internal/pdf: text run names face " + e.face + ", not present in the document's face set"
}

type missingGlyphError struct {
	face string
	cid  uint16
}

func (e *missingGlyphError) Error() string {
	return "internal/pdf: face " + e.face + ": CID " + itoa(int64(e.cid)) +
		" appears in a text run but has no width in the embedded face — document assembly and " +
		"content-stream assembly disagree about what this document draws"
}

// verticalOffsetError is the fail-closed branch AC6 opened and Story 8.0
// NARROWED: a shaped glyph whose non-zero YOffset scales to a text rise
// of ZERO at this run's font size.
//
// It is no longer "a TJ array cannot express this". Ts expresses it, and
// the emitter uses Ts. What survives is the rounding boundary: an offset
// the shaper really asked for, scaled by a font size small enough that
// geom.ScaleRound returns 0. Emitting "0 Ts" there would silently place
// the mark on the baseline, and the reason clause below — kept verbatim
// from the branch this narrows, because it is still exactly why refusing
// beats degrading — says what that would cost.
//
// fontSize is carried so the message can state the two numbers that
// produced the zero rather than only the offset; it is in millipoints,
// the unit run.FontSize is stated in.
type verticalOffsetError struct {
	face     string
	cid      uint16
	offset   int64
	fontSize geom.Length
}

func (e *verticalOffsetError) Error() string {
	return "internal/pdf: face " + e.face + ": CID " + itoa(int64(e.cid)) +
		" carries a non-zero vertical offset (" + itoa(e.offset) + " thousandths of an em) that scales to " +
		"a text rise of zero at this run's font size (" + itoa(int64(e.fontSize)) + " millipoints). " +
		"Emitting the glyph without its offset would place it wrongly with no observable difference in " +
		"the output bytes, so this fails rather than degrades."
}

// itoa formats an integer for an error message without importing
// strconv into this package's non-test code path beyond what AD-3
// already routes; numbers.go's emitters are for OUTPUT BYTES, and an
// error string is not one.
func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [24]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

const hexDigits = "0123456789abcdef"

// appendHexCID appends one CID as four hex digits (a 2-byte big-endian
// value under Identity-H).
//
// D-1.1.b, verbatim, on why this is not routed through numbers.go's
// appendInt: "Glyph ids under Identity-H take neither route — they are
// a big-endian hex pair inside a string literal, i.e. a byte encoding
// like /ID, not a number. Same for the six-letter subset tag. This must
// be said in a code comment or a later agent will 'unify' it."
func appendHexCID(dst []byte, cid uint16) []byte {
	b := [2]byte{byte(cid >> 8), byte(cid)}
	for _, x := range b {
		dst = append(dst, hexDigits[x>>4], hexDigits[x&0x0f])
	}
	return dst
}
