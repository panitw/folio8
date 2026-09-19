// Package pagemodel is AD-5's page model: the finished description of a
// laid-out page that every renderer consumes and no renderer defines.
//
// AD-5, verbatim: "The page model knows nothing about PDF." Its types
// name only geometry, glyph runs (font identity + glyph ids +
// positions) and images. That is what keeps a PNG, SVG or HTML renderer
// possible later: `internal/layout` produces a value of this package's
// types and never learns what a PDF object is, and `internal/pdf`
// consumes one and never learns what a band is.
//
// Two properties this package holds, and which internal/arch's
// TestPageModelNamesNoPDFConcept asserts rather than describes:
//
//   - It imports NO first-party package other than internal/geom. The
//     stage-rank guard (lint's `stage-rank` rule) makes that structural:
//     pagemodel is rank 1 and may import only rank 0.
//   - No identifier in it names a PDF object reference, a resource
//     dictionary, a content-stream operator, or a font PROGRAM. A face
//     NAME and a glyph id are page-model concepts; an embedded,
//     subsetted font program is not — that is internal/pdf's
//     EmbeddedFace, and it stays there. So does ImageXObject, which
//     names a PDF construct in its own name.
//
// Coordinates. Every coordinate here is PAGE-ABSOLUTE and in
// geom.Length millipoints (AD-2's one fixed-point unit), on a TOP-LEFT
// origin with Y increasing DOWNWARD — the same sense X/Y read in the
// `.folio` document model. Band placement has ALREADY been resolved by
// internal/layout before a value of these types exists: AD-24 says
// "bands are placed on the page by internal/layout alone", so nothing
// downstream of this package may add a band origin, and nothing here
// carries one.
package pagemodel

import "github.com/panitw/folio8/folio-go/internal/geom"

// TextRun is one run of shaped text on one page, at a PAGE-ABSOLUTE
// origin.
//
// X/Y are the offset from the page's top-left PRINTABLE corner (inside
// the margins) to the run's top-left corner, Y increasing DOWNWARD.
// This is PERMANENT, not provisional. Under AD-24 band placement
// belongs to internal/layout alone, so making these band-relative would
// move band placement into whichever renderer consumed them — violating
// AD-24 outright. internal/layout resolves the band origin and emits
// the sum (layout.PlaceInBand); what reaches a renderer is already
// absolute.
//
// Converting this top-down Y into a bottom-up renderer's own space is
// that renderer's business and happens in exactly one function there
// (for PDF: internal/pdf's flipY, AD-24's one-and-only inverter,
// ratified by D-1.8.10).
type TextRun struct {
	// Face is the face's IDENTITY — the FontSet key the document
	// resolved this run's runes to. It is a name, not a font program:
	// nothing here carries font bytes.
	Face string

	// Glyphs is the run's shaped glyph sequence, in drawing order.
	// Positions are already scaled to the 1000-unit em by
	// geom.ScaleRound at the one site that knows the face's unitsPerEm.
	Glyphs []ShapedGlyph

	// SourceText is the run's original text, carried for diagnostics
	// only. Nothing downstream derives a glyph, an identifier or a
	// width from it.
	SourceText string

	X, Y     geom.Length
	FontSize geom.Length

	// HasColor / Color are Story 10.1's ink: the fill colour this run's
	// glyphs are painted in. Absent (HasColor false) means the renderer
	// emits no colour operator at all and the run takes the PDF's own
	// initial fill, black — the state every document produced before the
	// field existed, which is what keeps them byte-identical. Spelled as
	// a bool plus a value, never a *Color, for the reason Rect's own
	// HasFill/HasStroke are: a pointer invites a float and an alias.
	HasColor bool
	Color    Color

	// BaselineOffset is the distance DOWNWARD from this run's top-left
	// corner (X/Y above) to its BASELINE — the ruled vertical model's
	// first span: max(hhea ascent) over the element's DECLARED font
	// chain, scaled to FontSize (D-2.4.2 as amended).
	//
	// WHY IT IS CARRIED HERE RATHER THAN DERIVED BY A RENDERER. It is a
	// LAYOUT quantity, and under AD-5 the page model is the finished
	// description of a laid-out page: a renderer that computed its own
	// baseline would be deciding placement. internal/layout and package
	// folio8 resolve it from the chain before a value of this type
	// exists, exactly as they already resolve the band origin into X/Y.
	//
	// WHY IT IS NOT DERIVABLE FROM Face. It is a function of the
	// DECLARED CHAIN, not of the face this particular run resolved to,
	// so every run of one element carries the SAME value even when the
	// runs sit on different faces and different lines. A renderer
	// deriving it from its own resolved face would make it
	// content-dependent — adding one CJK character would reflow the
	// element — which is what AD-24's "boxes are absolute, and nothing
	// negotiates" rules out.
	//
	// WHY IT IS NOT FontSize. It used to be, by omission rather than by
	// choice: internal/pdf placed the baseline FontSize below the top,
	// using the point size as a proxy for an ascent it has no
	// relationship to (DW-15, fixed by Story 2.5a). The two agree only
	// by accident. Measured on the shipped faces, max(hhea ascent) is
	// 1069/1061/1160 per em for the three Noto faces and 928 for the
	// Roboto test face — so the old proxy erred DOWNWARD for the first
	// three and UPWARD for the fourth. It has no consistent direction,
	// and nothing may assert one (D-000.45).
	BaselineOffset geom.Length

	// PageSlots is empty for every run except one carrying at least one
	// of Story 2.7's {{page}} constructs. AD-4, verbatim: "a late-bound
	// slot whose box is already measured" (D-2.7.1: the box is FIXED
	// here, in pass one) "resolved between the passes by substituting
	// pre-measured glyphs" (D-2.7.1: substitution is glyph selection,
	// never a re-shape; D-2.7.2: the digits are right-aligned within
	// the reservation, no padding).
	//
	// A SLICE, not a single value (this story's review, Blocker 1): a
	// template author may write more than one {{page}} occurrence in
	// one element ("Page {{page}} of {{pages}} / {{page}}"), and
	// because the whole construct is ASCII it can land in a single
	// face segment on a single line — one TextRun carrying two
	// reservations. A scalar field here silently overwrote the first
	// with the second (the run drew the reservation's filler `0` on
	// every page, unnoticed, since nothing errored). Entries are
	// disjoint, non-overlapping glyph ranges within Glyphs, in
	// ascending GlyphLo order.
	//
	// It names no PDF concept: a reservation width in DIGITS, a
	// per-digit advance and a face's own pre-shaped digit identifiers
	// are page-model quantities exactly as CID already is on
	// ShapedGlyph above.
	PageSlots []PageNumberSlot

	// ClipToBox, ClipX, ClipWidth are Story 2.8's FR44 clip (D-2.8.1):
	// when ClipToBox is true, a renderer restricts this run's drawing to
	// the HORIZONTAL span [ClipX, ClipX+ClipWidth) — the element's own
	// declared box left edge and width, PAGE-ABSOLUTE exactly like X
	// above — because the element's widest laid-out line exceeds that
	// declared width.
	//
	// There is no vertical counterpart, and this is not an omission: a
	// text element's declared HEIGHT is not a clip bound (D-2.8.1 — no
	// mechanism lets an author satisfy one, and no layout stage ever
	// reads it). A renderer honouring ClipToBox restricts only the
	// horizontal axis and must clip vertically to the page itself (or
	// not at all), never to anything derived from an element's declared
	// height.
	//
	// Set uniformly across every run of one overflowing element (package
	// folio8's collectBandTextRuns): a multi-line or multi-face-segment
	// element clips the same box on every one of its runs.
	ClipToBox bool
	ClipX     geom.Length
	ClipWidth geom.Length
}

// PageNumberSlot is Story 2.7's late-bound page-number slot (D-2.7.1,
// D-2.7.2). It marks a GlyphLo:GlyphHi range within the carrying
// TextRun's Glyphs as a DigitsY-wide reservation for the document's
// current page number, and carries everything substitution needs
// WITHOUT re-shaping: the ten digits' CIDs and their (tabular, equal)
// advance, both measured once in pass one.
//
// Substitution (the between-passes step, package folio8's
// paginateDocument) replaces the reserved range with DECIMAL(pageNum)'s
// CIDs, RIGHT-ALIGNED within the DigitsY-wide reservation: a page whose
// own digit count is less than DigitsY leaves the slack BEFORE the
// digits, expressed as a positioning adjustment on the first drawn
// digit — never a pad glyph (none exists across the shipped face set,
// measured at this story's creation) and never zero-padding (AD-14: a
// coercion of a bound value, which this project never performs
// silently).
type PageNumberSlot struct {
	// GlyphLo, GlyphHi delimit the reservation within the carrying
	// run's Glyphs. GlyphHi - GlyphLo == DigitsY always (D-2.7.2: a
	// slot of digits(Y) digit-advances is exactly sufficient and never
	// over-wide, since X <= Y always).
	GlyphLo, GlyphHi int

	// DigitsY is digits(Y), the slot's fixed reservation width — the
	// number of decimal digits in the document's total page count.
	DigitsY int

	// DigitAdvance is the per-digit XAdvance, 1000-unit em, identical
	// for every digit 0-9 in a tabular-figure face (measured at this
	// story's creation: 572/572/555 across the three shipped faces).
	DigitAdvance int64

	// DigitCID[d] is the pre-shaped CID for decimal digit d (0-9) on
	// this run's Face — allocated once, in pass one, alongside every
	// other CID this document uses. Substitution never shapes; it only
	// selects among these ten.
	DigitCID [10]uint16
}

// ShapedGlyph is one glyph of a shaped run: its identifier within the
// run's face, plus its position, in the 1000-unit em.
//
// CID is the identifier the document's font plan allocated for this
// glyph. This package neither allocates it nor knows how any renderer
// spells it out.
//
// KNOWN GAP — DW-16. It is NOT always a glyph id, and this comment used
// to say that it was. The allocator gives it two kinds of value: the
// subset glyph id for a glyph's first source text (AD-5's "glyph ids",
// legitimately), and a SYNTHETIC identifier — an index past the end of
// the subset — for the same glyph carrying a different source text. The
// second kind exists only because PDF's /ToUnicode CMap maps one
// identifier to one text, and the table that resolves it back to a glyph
// is deliberately NOT carried here. So a non-PDF renderer cannot resolve
// Glyphs today, and cannot tell the two kinds apart at the type.
//
// This is recorded, not fixed: it is Story 2.3's allocation, which Story
// 2.5 merely relocated into this package and thereby exposed. Read DW-16
// before adding a second producer of this field — that is what makes it
// expensive to fix.
type ShapedGlyph struct {
	CID      uint16
	XAdvance int64
	XOffset  int64
	YOffset  int64
}

// ImagePlacement is one image element's resolved, DRAWN placement on a
// page: the fit-and-centre computation's RESULT, never its inputs.
//
// X, Y are the top-left corner of the DRAWN box (already centred within
// the element's declared box), page-absolute in the same Y-down
// convention as TextRun.X/Y.
type ImagePlacement struct {
	// AssetKey names WHICH image this places — the document's own asset
	// key, which is what internal/template produced and what
	// internal/layout passes through. A renderer maps it onto whatever
	// it calls a resource; that mapping is the renderer's, not the page
	// model's (this field was `ResourceName` while these types lived in
	// internal/pdf, which named a PDF resource dictionary from inside
	// the page model — exactly the identifier class AD-5 excludes).
	AssetKey              string
	X, Y                  geom.Length
	DrawWidth, DrawHeight geom.Length
}

// Color is a page-model colour: three channels, each 0..255, exactly the
// `.folio` format's own `#RRGGBB` representation (folio-format.md). It
// names no PDF concept — never a `rg`/`RG` operand, never a decimal, never
// a float (AD-1/AD-23). Converting a channel into the PDF's 0..1 decimal
// operand is internal/pdf's job, at the one site that emits it
// (geom.ScaleRound(channel, 1000, 255), Story 4.1's AC5).
type Color struct {
	R, G, B uint8
}

// RectEdges selects which of a Rect's four sides are stroked. It is
// meaningless when the carrying Rect has HasStroke == false.
type RectEdges struct {
	Top, Right, Bottom, Left bool
}

// Rect is the page model's first vector primitive (AD-5, AD-13, Story
// 4.1): an axis-aligned, filled and/or stroked rectangle, PAGE-ABSOLUTE,
// top-left origin, Y increasing DOWNWARD — the same convention as
// TextRun.X/Y and ImagePlacement.X/Y above.
//
// Its fields name only geometry, an edge set, a stroke width and colour
// channels — no PDF operator, no resource name, no object number (AD-5;
// TestPageModelNamesNoPDFConcept). HasFill/HasStroke are plain booleans
// rather than a nil-pointer "absent" convention because AD-1 already
// forbids a float anywhere under internal/, and a *Color would invite one
// (a caller reaching for a zero-value sentinel) for no benefit: the two
// bits carry the same information a pointer would, without ever being
// nil-dereferenced.
type Rect struct {
	X, Y, W, H geom.Length

	HasFill bool
	Fill    Color

	HasStroke   bool
	Stroke      Color
	StrokeWidth geom.Length
	Edges       RectEdges
}

// Page is one finished page: its content, page-absolute, plus the page
// geometry a renderer needs to place that content in its own space.
//
// MarginTop/MarginLeft are here because TextRun.X/Y are offsets from
// the printable corner rather than from the paper corner; a renderer
// adds them to reach paper coordinates. Width/Height are the paper's.
type Page struct {
	Runs                  []TextRun
	Images                []ImagePlacement
	Rects                 []Rect
	Width, Height         geom.Length
	MarginTop, MarginLeft geom.Length
}
