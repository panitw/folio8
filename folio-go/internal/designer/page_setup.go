package designer

// GridIncrement is the fixed six-point grid used by the designer projection.
// SnapNearest's documented midpoint rule is away from zero.
const GridIncrement int64 = 6000

// MaxCanvasMillipoints keeps every document value emitted to the JSON/JS paint
// boundary within Number.MAX_SAFE_INTEGER. The page-setup command has the same
// bound, so a successful command can never strand the worker with an
// unrepresentable projection.
const MaxCanvasMillipoints int64 = 9007199254740991

// CanvasTextFragment is a shaped, positioned paint fragment. It is not a
// document text node: x is the engine-owned, band-relative paint origin.
type CanvasTextFragment struct {
	Text string `json:"text"`
	X    int64  `json:"x"`
	// AssetKey names the document's OWN asset the engine resolved this
	// fragment's face to, and is empty — and omitted from the wire — for
	// every fragment drawn with a face the caller shipped.
	//
	// WHY THE ASSET KEY AND NOT THE FACE NAME (Story 8.4a). The engine's
	// name for a carried face is embeddedFaceName(assetKey), and
	// embedded_face.go states that a caller spelling that prefix is
	// writing the derivation a second time. Putting the minted name on
	// the wire would force the browser to either strip the prefix — the
	// second spelling, in a second language, that no Go test can pin — or
	// use an engine-internal namespace as a CSS family. The KEY is what
	// the existing `asset` operation already takes as its payload and
	// what canvasFontChainEntryWireKeys already carries, so the browser
	// derives its own CSS family from it (D-8.4.1: from the ASSET KEY,
	// never from font.family) and needs no other rule.
	//
	// WHY PER FRAGMENT AND NEVER PER COMPONENT. faceSegment.face is a
	// scalar and positionSegments emits at most one run per segment
	// without ever merging adjacent runs, so a fragment is exactly one
	// face BY CONSTRUCTION. A component is not: a mixed-script element
	// draws Latin through one chain entry and Thai through another, and
	// attributing at the component would hand one of them the other's
	// glyphs.
	//
	// AD-17 IS UNTOUCHED BY IT. This is attribution, not measurement: X
	// is still the engine's own paint origin and the browser still
	// computes no metric, no advance and no line break.
	AssetKey string `json:"assetKey,omitempty"`
	// Face names the SHIPPED face the engine resolved this fragment to —
	// the caller's own FontSet key, verbatim — and is empty, and omitted
	// from the wire, for every fragment drawn with a face the document
	// carries. It is AssetKey's mutually exclusive twin: exactly one of
	// the two is set on every emitted fragment, which is the same
	// discriminated pair CanvasFontChainEntry already puts on the wire
	// (Face xor AssetKey) one level up.
	//
	// WHY THE FontSet NAME AND NOT SOMETHING DERIVED (Story 8.4e,
	// D-8.4.14). "A carried face's browser family derives from the
	// engine's identity for it (the asset key); a shipped face's from the
	// engine's identity for it (the FontSet name). One rule for one
	// question." The browser declares an @font-face under each of those
	// three names already (Story 8.4b), so the name IS the CSS family and
	// there is nothing to map. The two alternatives were rejected BY NAME
	// there: renaming the generated families (the design system's own
	// typeface is not the engine's to rename) and a face-name -> family
	// mapping table (a second authority maintained in lockstep with
	// fonts.Shipped()). Nothing derived, mapped or re-spelled goes here,
	// and never a chain entry's `family` or `style` — those are DISPLAY
	// identity (AD-8, D-8.4.1), not how a face is found.
	//
	// IT CANNOT CARRY DOCUMENT TEXT. resolveRuneFace returns an element of
	// chainFaceNames(chain) and a chain entry whose face is absent from the
	// supplied FontSet is skipped, so a face can only be attributed here
	// once the engine actually loaded and measured with it: the value comes
	// from the caller's FontSet keys, not from arbitrary document input.
	// The browser still bounds and shape-checks it, because a guard's job
	// is to hold when this side is wrong.
	//
	// WHY PER FRAGMENT AND NEVER PER COMPONENT. The same construction that
	// settles it for AssetKey: faceSegment.face is a scalar and
	// positionSegments emits at most one run per segment without ever
	// merging adjacent runs, so a fragment is exactly one face. A component
	// is not — a document whose chain is ["Noto Sans Thai"] draws its Latin
	// through that same Thai face, and the three shipped faces' cmaps
	// overlap (339 / 529 / 230 code points pairwise, all three covering `A`
	// and `5`), so a component-level answer would hand a run the wrong
	// face's advances while painting the right glyphs.
	//
	// AD-17 IS UNTOUCHED BY IT, exactly as for AssetKey: this is
	// attribution, not measurement. X is still the engine's paint origin
	// and the browser computes no metric, no advance and no line break.
	Face string `json:"face,omitempty"`
}

// CanvasTextLine is one pre-broken engine line. All coordinates are
// band-relative, top-left/Y-down millipoints. Advance is retained so the
// browser never derives a following line's origin from CSS metrics.
type CanvasTextLine struct {
	Top       int64                `json:"top"`
	Baseline  int64                `json:"baseline"`
	Advance   int64                `json:"advance"`
	Width     int64                `json:"width"`
	Fragments []CanvasTextFragment `json:"fragments"`
}

// CanvasTextPaint is the closed browser paint plan for one text component.
// It deliberately carries no CSS, browser metric, or document-schema input.
type CanvasTextPaint struct {
	Overflow bool `json:"overflow"`
	// Truncated says this paint is a PREFIX of the element's text: the value
	// is intact in the document and renders whole to PDF, but the projection
	// stopped at a painting bound (D-7.4.2 §2).
	//
	// It exists because without it a degraded element and an EMPTY element
	// are indistinguishable — both used to project `Lines: []`, the all-clear
	// wearing the face of could-not-look. It is a projection disposition, not
	// a document validity rule: no diag.Diagnostic, no registry entry, and
	// the render path has no such cap.
	Truncated bool             `json:"truncated"`
	Lines     []CanvasTextLine `json:"lines"`
}

type CanvasBand struct {
	Name   string `json:"name"`
	X      int64  `json:"x"`
	Y      int64  `json:"y"`
	Width  int64  `json:"width"`
	Height int64  `json:"height"`
}

type CanvasComponent struct {
	Authored  *CanvasAuthoredProperties `json:"authored,omitempty"`
	ID        string                    `json:"id"`
	Type      string                    `json:"type"`
	Band      string                    `json:"band"`
	X         int64                     `json:"x"`
	Y         int64                     `json:"y"`
	Width     int64                     `json:"width"`
	Height    int64                     `json:"height"`
	Resizable bool                      `json:"resizable"`
	// The following explicitly named optional values are the minimum committed
	// property-panel projection. This is not a generic style or document bag.
	Value *string `json:"value,omitempty"`
	// Binding is a bounded, Go-derived paint label for a direct text binding.
	// It is not a general expression/template projection and cannot be used to
	// reconstruct canonical document bytes in the browser.
	Binding    *string `json:"binding,omitempty"`
	VisibleIf  *string `json:"visibleIf,omitempty"`
	FontFamily *string `json:"fontFamily,omitempty"`
	FontSize   *int64  `json:"fontSize,omitempty"`
	// LineSpacing is style.lineSpacing in THOUSANDTHS, the unit the format
	// and the property command both carry it in (template.LineSpacingUnit).
	// It is dimensionless — a ratio applied to the vertical model's Advance —
	// so it is not a geom.Length and is never treated as one.
	LineSpacing   *int64            `json:"lineSpacing,omitempty"`
	Bold          *bool             `json:"bold,omitempty"`
	Italic        *bool             `json:"italic,omitempty"`
	Align         *string           `json:"align,omitempty"`
	Valign        *string           `json:"valign,omitempty"`
	Background    *string           `json:"background,omitempty"`
	Color         *string           `json:"color,omitempty"`
	BorderWidth   *int64            `json:"borderWidth,omitempty"`
	BorderColor   *string           `json:"borderColor,omitempty"`
	BorderEdges   []string          `json:"borderEdges,omitempty"`
	TableBind     *string           `json:"tableBind,omitempty"`
	PaddingTop    *int64            `json:"paddingTop,omitempty"`
	PaddingRight  *int64            `json:"paddingRight,omitempty"`
	PaddingBottom *int64            `json:"paddingBottom,omitempty"`
	PaddingLeft   *int64            `json:"paddingLeft,omitempty"`
	TextPaint     *CanvasTextPaint  `json:"textPaint,omitempty"`
	Image         *CanvasImagePaint `json:"image,omitempty"`
	// ImageUnavailable is a small, bounded discriminant set ONLY when this
	// is an image element and Image is absent (Finding 9, review of
	// 2026-08-29): "missing" when the element's own asset key is not in
	// the document's assets map, or "undecodable" when the key resolves
	// but the bytes fail to decode or the media type is one this library
	// version cannot render. D-5.13.2's "one Go-side signal drives both"
	// governed the media-type case only; collapsing a dangling asset
	// reference into that same undecodable text was a defect — the media
	// type there is fine, the asset is simply gone. This does not widen
	// the projection's authority: it is still Go stating which of two
	// bounded, enumerated reasons applies, never bytes or a path.
	ImageUnavailable *string `json:"imageUnavailable,omitempty"`
	// Barcode is a barcode component's Go-computed bars (barcode_element.go).
	// BarcodeUnavailable is set instead, to "unencodable" or "doesNotFit", when
	// a barcode with a value cannot be painted.
	Barcode            *CanvasBarcodePaint `json:"barcode,omitempty"`
	BarcodeUnavailable *string             `json:"barcodeUnavailable,omitempty"`
	// QRCode is a qrcode component's Go-computed module runs
	// (barcode_element.go). QRCodeUnavailable is set instead, to "tooLong" or
	// "doesNotFit", when a qrcode with a value cannot be painted.
	QRCode            *CanvasQRCodePaint `json:"qrcode,omitempty"`
	QRCodeUnavailable *string            `json:"qrcodeUnavailable,omitempty"`
	// Columns is Story 14.9's per-column paint data for a TABLE, and it is
	// absent — never an empty array — for a table that declares none, and for
	// every non-table component. See CanvasTableColumn below.
	Columns []CanvasTableColumn `json:"columns,omitempty"`
	// BelowSectionBreak is spec-section-break CAP-6's section membership: set,
	// for every CONTENT component, only when the document declares a break —
	// true when the element is declared at or below it (it moves with the
	// section), false when above. Absent on every other component and on every
	// component of a document without a break, so such a document projects
	// exactly as before. The designer derives membership from nothing else.
	BelowSectionBreak *bool `json:"belowSectionBreak,omitempty"`
	// Page is SPEC-multi-pages' designed page a CONTENT component belongs to,
	// 0-based and always present, so the designer homes and echoes it only on
	// its own page's sheets. It is 0 for every page header and page footer
	// component, which belong to no one page, and for every component of a
	// one-page document.
	Page int `json:"page"`
}

// CanvasTableColumn is Story 14.9's read-only, paint-only projection of ONE
// table column: what the canvas needs to draw the table it will print, and
// nothing more.
//
// IT IS NOT TableColumnProjection, AND THE DIFFERENCE IS THE GATE. That type
// (table_columns_projection.go) serves the table EDITOR, is requested per table
// by element id, and its producer VALIDATES — it hard-errors on `width <= 0`,
// on more than 128 columns, and on a bind that fails `rootCollectionPath`. The
// canvas tolerates all three today, and a canvas-projection error blanks the
// WHOLE designer, so reusing that function or its gate would newly kill
// documents that currently paint. The derivations are copied; the gate is not.
//
// TWO RESOLVED ALIGNMENTS, BECAUSE THE CANVAS DRAWS TWO ROWS (Story 14.9 / R2).
// The engine resolves a header cell's alignment and a data cell's through
// DIFFERENT cascades — resolveHeaderStyle takes `headerStyle.align` then
// `style.align`, resolveBodyStyle takes `style.align` alone — and a column's
// own `align` wins over either. One value used for both rows would be wrong on
// every table whose `headerStyle.align` differs from its `style.align`, which
// Story 14.8 made authorable from the shipped UI. Both are obtained by CALLING
// those two functions plus the shared columnAlign, never by mirroring them:
// a mirrored cascade drifts, and the failure mode is a canvas that lies about
// print while every test passes.
//
// Width is MILLIPOINTS, unconverted — the wire unit, as everywhere else on this
// projection — and is projected verbatim, including zero and negative values,
// which load and paint today.
//
// Label and Bind are REQUIRED KEYS WHOSE VALUES MAY BE EMPTY. internal/template
// hand-decodes `columns` and both keys are mandatory there, so `""` means
// "declared empty", not "absent": an empty label is a header cell the renderer
// builds and then skips the glyphs for, and an empty bind is a column nobody
// has pointed at data yet. Neither is `omitempty`, because the browser's guard
// is hasExactKeys per column and a dropped key fails it as hard as a surplus
// one.
type CanvasTableColumn struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// LabelLines — SPEC-table-rules §4: the header label as the ENGINE lays
	// it out, one entry per line, line feeds removed. The canvas paints
	// these and never lets the browser wrap. Always present ([] for an
	// empty label). With a font set (CanvasWithTextPaint) they are the
	// packed lines the PDF prints; without one, the label split at its
	// line feeds.
	LabelLines []string `json:"labelLines"`
	Width      int64    `json:"width"`
	// HeaderAlign is resolveHeaderStyle's fallback with the column's own
	// `align` applied over it; CellAlign is resolveBodyStyle's, the same way.
	HeaderAlign string `json:"headerAlign"`
	CellAlign   string `json:"cellAlign"`
	Bind        string `json:"bind"`
}

// CanvasImagePaint is Story 5.13's read-only, paint-only projection of one
// placed image element's Go-owned display data: the declared media type,
// the asset's content-addressed key, VALIDATED intrinsic pixel dimensions,
// and the fit-and-centre draw rectangle already computed for the PDF
// (resolveImagePlacement), in BAND-RELATIVE millipoints (matching this
// component's own X/Y, D-5.13.2's "Frame" clause). It carries no asset
// BYTES (AD-17: a paint-only projection must not carry anything that
// reconstructs canonical bytes or the assets map): AssetKey is only a
// LOOKUP TOKEN for the separate, explicit, per-key bytes request
// (AssetBytes/wasm.Engine.AssetBytes) the canvas uses to obtain what it
// paints — a key alone cannot reconstruct the assets map or canonical
// bytes any more than a table id (already sent on every projection) can.
// The inspector abbreviates this same key for DISPLAY (a formatting choice
// over a value Go already supplied, same as it formats millipoints as
// "12.5pt"); Go does not truncate it on the wire, because the canvas needs
// the real key to ask for bytes.
//
// The whole field is present only when the referenced asset decodes
// successfully through the recognised-image path — D-5.13.2's "Absence,
// not zero": DecodedImage.Width()/Height() are reachable only through
// decodeRecognisedImage, so a legally-loaded asset of an unrecognised media
// type (or one whose bytes fail to decode) has no dimensions and no
// computable rectangle. Rather than carry two independently-absent signals
// (a known media type but a missing rectangle), the ENTIRE paint is absent
// together — ONE Go-side signal drives both AC2's inspector failure text
// and AC3's canvas placeholder, never two.
type CanvasImagePaint struct {
	MediaType  string `json:"mediaType"`
	AssetKey   string `json:"assetKey"`
	Width      int64  `json:"width"`
	Height     int64  `json:"height"`
	DrawX      int64  `json:"drawX"`
	DrawY      int64  `json:"drawY"`
	DrawWidth  int64  `json:"drawWidth"`
	DrawHeight int64  `json:"drawHeight"`
}

type CanvasProjection struct {
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
	// Locale and UTCOffset are the DOCUMENT's two declared formatting
	// authorities (Story 12.2), projected so the panel can show what the
	// engine holds instead of a default of its own. Both are top-level
	// document fields, both are REQUIRED at load, and neither is derived
	// here: Locale is one of AD-12's four tags and UTCOffset is the
	// loader's ±HH:MM string, carried verbatim.
	//
	// NEITHER CARRIES omitempty, and that is a protocol requirement rather
	// than a style: TestCanvasProjectionWireKeysAreTheRecordedSet marshals
	// the ZERO CanvasProjection as well as a real one and demands the same
	// key set from both, because a key that appears only sometimes is a key
	// the browser's guard rejects only sometimes.
	Locale        string            `json:"locale"`
	UTCOffset     string            `json:"utcOffset"`
	Orientation   string            `json:"orientation"`
	Preset        string            `json:"preset"`
	MarginTop     int64             `json:"marginTop"`
	MarginRight   int64             `json:"marginRight"`
	MarginBottom  int64             `json:"marginBottom"`
	MarginLeft    int64             `json:"marginLeft"`
	GridIncrement int64             `json:"gridIncrement"`
	CommandWidth  int64             `json:"commandWidth"`
	CommandHeight int64             `json:"commandHeight"`
	Bands         []CanvasBand      `json:"bands"`
	Components    []CanvasComponent `json:"components"`
	// FontFamilies is the closed set style.fontFamily may name in THIS
	// document: the declared, non-empty font chains, by name, sorted so the
	// projection is deterministic. It exists so the designer can offer the
	// author exactly the families the engine will accept (knownFontFamily),
	// instead of a free text field whose every rejection is a round trip. It
	// is still names only — the faces live in FontChains below, and the font
	// BYTES are projected by nothing.
	FontFamilies []string `json:"fontFamilies"`
	// FontChains is the same set, WITH the ordered faces behind each name —
	// entry for entry, in the document's own authored order. It is exactly the
	// chains FontFamilies names, in the same positions, so the two can never
	// disagree about which chains a document declares; it exists so a chain
	// editor re-projects the engine's answer instead of modelling the fonts
	// map a second time in the browser.
	FontChains []CanvasFontChain `json:"fontChains"`
	// DefaultFontSize is the size the producer draws a text element at when
	// its style carries no fontSize, in millipoints. It is projected rather
	// than restated in the browser for the ordinary reason: it is the
	// engine's number, and a second copy of it in the designer would be a
	// second authority on what an unset size means.
	DefaultFontSize int64 `json:"defaultFontSize"`
	// DefaultLineSpacing is the leading ratio the producer measures a text
	// element with when its style carries no lineSpacing, in THOUSANDTHS —
	// `defaultLineSpacing` in render.go, which is template.LineSpacingUnit
	// and nothing else. It is dimensionless: 1000 here is a ratio of 1.0,
	// not a length, and it travels beside DefaultFontSize for the same
	// reason that one does. Story 17.3: the designer used to spell this
	// number itself, as a hard-coded `'1'` in the inspector's line-spacing
	// field, which was a SECOND authority on a number this engine owns —
	// the two could disagree and neither would know.
	DefaultLineSpacing int64 `json:"defaultLineSpacing"`
	// ContentWindowHeight is ONE page's worth of content column, in
	// millipoints: internal/layout's ContentHeight, which is the single
	// function permitted to derive it (AD-13). It is the same number
	// bands[1].height carries — the content band rectangle IS one window —
	// and it is named separately because the two stop being interchangeable
	// the moment the designer draws a second sheet: the band is where page
	// one's content sits, the window is the distance between sheets.
	//
	// It is projected rather than recomputed in the browser because a second
	// spelling of it would be a second authority on where a page ends, and
	// the divergence would be invisible: the canvas and the engine would draw
	// different pages and still agree on the bytes.
	ContentWindowHeight int64 `json:"contentWindowHeight"`
	// ContentWindowCount is how many of those windows the content column
	// occupies, from internal/layout's Paginate — the ONE function that
	// decides how many pages a column has. It is never `ceil(lowestBottom /
	// ContentWindowHeight)`, a spelling paginate.go forbids by name: the
	// window advances to the first item that did not fit, so an element
	// declared ten windows below the text starts the NEXT window rather than
	// generating nine empty ones.
	//
	// WHAT THIS NUMBER IS A NUMBER ABOUT. It describes the column AS THE
	// CANVAS CURRENTLY PAINTS IT, not the document that will render. The
	// canvas has no data, so a bound table contributes only its header
	// height (projectedSize) and every row it will grow is absent: for any
	// document with a bound table in the content band this count is a FLOOR,
	// never a prediction. The finished document may run longer; it can never
	// run shorter.
	//
	// It is never derived from CanvasTextPaint. The paint truncates — at a
	// line budget and at a fragment budget — and a count that read a paint's
	// line list would shorten with it, so the canvas would draw the wrong
	// number of sheets for exactly the documents long enough to need them.
	// The extents fed to Paginate come from the FULL shaped line list and the
	// vertical model, which truncation never touches.
	//
	// Always at least 1: a column with nothing in it is one page, not zero.
	ContentWindowCount int64 `json:"contentWindowCount"`
	// ContentWindowOrigins is where each of those windows BEGINS, in the
	// content column's own band-relative frame — the same frame
	// CanvasComponent.Y is already in for a content component. origins[0] is
	// 0, because window one starts at the top of the column and internal/
	// layout guarantees that unconditionally; every later entry is the
	// column offset the engine slid that window to. There is exactly one
	// entry per window, so len(ContentWindowOrigins) == ContentWindowCount.
	//
	// They come from internal/layout's own PageAssignment.Shift — the value
	// Paginate had already computed while deciding the count — and are NEVER
	// `index * ContentWindowHeight`. That closed form is the spelling
	// paginate.go forbids by name for the count, and origins expose it more
	// sharply than the count does: the window advances to the TOP OF THE
	// FIRST ITEM THAT DID NOT FIT, never by a fixed height, so three
	// elements a round 728pt apart begin at 0, 728000 and 1456000 where the
	// closed form answers 0, 727890 and 1455780 — adrift by 110 millipoints
	// per window — and a column with a declared ten-window gap begins two
	// windows where the closed form answers eleven.
	//
	// The window HEIGHT does not vary: window i spans
	// [origins[i], origins[i]+ContentWindowHeight). Only the tops slide.
	//
	// It is ALWAYS non-empty. A nil slice marshals to JSON null, the browser
	// protocol rejects it, and rejecting one field discards the whole
	// snapshot — which blanks the canvas with nothing to attribute the blank
	// to.
	ContentWindowOrigins []int64 `json:"contentWindowOrigins"`
	// ContentWindowPages is SPEC-multi-pages CAP-8's engine half: the designed
	// page each window belongs to, one entry per window, so
	// len(ContentWindowPages) == ContentWindowCount. Windows are grouped by
	// page in page order, and ContentWindowOrigins are PAGE-LOCAL: each
	// page's first window has origin 0 and its later windows rise strictly
	// from there, in that page's own band-relative frame. A one-page document
	// projects all zeros. Always non-empty, for ContentWindowOrigins' reason.
	ContentWindowPages []int `json:"contentWindowPages"`
	// PageBreaks is each designed page's Page Break setting, one entry per
	// page in page order, so the Page Setup checkbox shows the engine's value.
	// Page 1's does not apply and is always true. Always non-empty, for
	// ContentWindowOrigins' reason.
	PageBreaks []bool `json:"pageBreaks"`
	// ContentWindowCountIsExact states, as a value rather than only in the
	// comment above, whether ContentWindowCount can be TRUSTED as the number
	// of pages this content column occupies. The ENGINE reports it, because
	// only the engine knows every cause; the designer states the consequence
	// in words and never decides for itself that, say, a table means more
	// pages — that would be a second authority on a question this flag
	// answers exactly.
	//
	// ITS ZERO VALUE IS THE SAFE CLAIM, and that is why it is spelled this
	// way round rather than as `…IsApproximate`. `false` reads "do not trust
	// this count", so a projection path that forgets to set it degrades to
	// the HONEST claim. The inverse field would have had a forgotten set
	// CLAIM EXACTNESS — which is precisely the defect that produced this
	// field's rename, rebuilt into its default. A hazard indicator must not
	// fail toward the quiet variant.
	//
	// It is false when any of these is so:
	//
	//	(a) a content-band table carries a non-empty binding, so the column
	//	    being counted holds that table's header and none of the rows its
	//	    data will grow;
	//	(b) Paginate could not place the column at all — a component taller
	//	    than one window — and the count degraded to the documented one,
	//	    or the pagination produced an origin sequence the browser
	//	    protocol would refuse;
	//	(c) a content-band text element contributed no extents because it
	//	    could not be shaped, so its lines are absent from the column
	//	    the count measures. TWO conditions reach it, and this used to
	//	    name only the first: the element's font chain would not
	//	    RESOLVE at all (no chain chosen, or none this build can read),
	//	    or the chain resolved and a face it names would not PARSE —
	//	    which since D-8.4.12 degrades the element rather than aborting
	//	    the projection, and so became a cause of an inexact count
	//	    instead of a cause of no count at all. A stale enumeration
	//	    reads as EXCLUDING the case it has not caught up with, which
	//	    is why the second is spelled here rather than left implied by
	//	    the first;
	//	(d) a content-band element's VISIBILITY DEPENDS ON DATA — it carries
	//	    a visibleIf, which this file only projects as a string and which
	//	    nothing on the canvas path evaluates, because evaluating it needs
	//	    the data the canvas has never been given. The canvas places the
	//	    element and the render may omit it, and AD-24 makes a hidden
	//	    element absent WITH NO GAP, so the column is simply shorter.
	//	    UNDISCLOSED SINCE STORY 7.5 shipped the count: it applies to an
	//	    UNGROUPED visibleIf element exactly as much as to a grouped one.
	//	    Story 7.9's grouping work is how it was found, not what caused it.
	//
	// GROUPING IS NOT AMONG THEM, and never becomes one. keepTogetherTags
	// takes the *Template and nothing else, so an author-declared
	// keep-together group is a pure template property the canvas holds every
	// input for: being wrong about it is a defect to fix, never a shortfall
	// to disclose. parse_bands.go's refusal of keepTogether on a table is
	// what keeps that true, by stopping a group inheriting (a)'s data
	// dependency.
	//
	// DIRECTION WAS DELIBERATELY DROPPED, and this sentence is here because
	// without it a future reader restores the floor claim mistaking a choice
	// for lost fidelity. The causes do not agree on a direction — (a) and (c)
	// make the canvas count too LOW (a floor), while (d) makes it too HIGH (a
	// ceiling), and a document carrying both is wrong in either direction —
	// so no single direction is honest for the general case, and the field
	// this replaced was named `ContentWindowCountIsFloor` and set true on
	// ceiling causes. Direction also informs no decision: a floor means there
	// may be more sheets than drawn and a ceiling fewer, and neither is a
	// safe side to act on. It belongs WITH THE CAUSES — a cause knows its own
	// direction — so if this projection ever carries the cause set, direction
	// can be derived there without this flag re-acquiring a claim. The
	// projection carries only the boolean today.
	ContentWindowCountIsExact bool `json:"contentWindowCountIsExact"`
	// SectionBreak is spec-section-break CAP-6's break offset, in the content
	// column's band-relative millipoints — the same frame a content component's
	// Y is in. ABSENT when the document declares no break, which is why this
	// one key carries omitempty: a document without a break projects exactly
	// the key set it always did. The canvas window count is untouched by it and
	// keeps plain pagination; the canvas draws the section only where it is
	// declared.
	SectionBreak *int64 `json:"sectionBreak,omitempty"`
	// SectionBreakAnchor is spec-section-break CAP-7's Anchor setting. It is
	// PRESENT, and false, only when the document declares a break and that
	// break is unanchored; absent otherwise, so an anchored break and a
	// document without one project exactly as before.
	SectionBreakAnchor *bool `json:"sectionBreakAnchor,omitempty"`
	// SectionBreaks is SPEC-multi-pages CAP-6's per-page break: on a projection
	// with more than one designed page, one entry per page in page order, the
	// page's break offset (in its own column's band-relative millipoints) or
	// null. SectionBreak and SectionBreakAnchor are then absent. ABSENT on a
	// one-page projection, which keeps exactly the key set it always had.
	SectionBreaks []*int64 `json:"sectionBreaks,omitempty"`
	// SectionBreakAnchors is SectionBreaks' Anchor, one entry per page: false
	// only where that page's break is unanchored, true otherwise (anchored, or
	// no break). Present exactly when SectionBreaks is.
	SectionBreakAnchors []bool `json:"sectionBreakAnchors,omitempty"`
}

// CanvasFontChain is one declared font chain AS THE DESIGNER SEES IT: the name
// style.fontFamily may carry, and the ordered face names behind it. Story 8.1
// adds the second half. FontFamilies' own doc comment used to say the
// projection was "names only — never the chains", and that was exactly the
// limitation a chain editor could not be built on: a moved or removed entry
// changes nothing the browser can observe, so the panel would have to model
// the fonts map itself rather than re-project it.
type CanvasFontChain struct {
	Name string `json:"name"`
	// Entries carries the ordered entries, and since Story 8.3 each is
	// an OBJECT rather than a string: an entry may name a face the
	// renderer is given or a face the document itself carries, and the
	// browser must be able to tell which without inspecting the value.
	Entries []CanvasFontChainEntry `json:"entries"`
}

// CanvasFontChainEntry is one chain entry AS THE DESIGNER SEES IT.
//
// THE SHAPE IS DISCRIMINATED, AND THE DISCRIMINANT IS PROJECTED RATHER
// THAN INFERRED. Exactly one of Face and AssetKey is non-empty. The
// browser is forbidden from deriving which kind an entry is — no key
// detection, no parsing, no length heuristic on a 64-character string —
// so the engine states it, and the designer's guard asserts it.
//
// Family and Style are EMPTY for a named face (its name is the whole
// identity the document gives it) and non-empty for an embedded one.
// They come from the asset's own `font` record, read HERE — the browser
// may display what this projection carries and derive nothing from it,
// which is why the family falls back below rather than being left for
// the panel to patch up.
//
// WHAT MECHANICALLY ENFORCES THAT, STATED NARROWLY. Nothing tests "the
// panel holds no rule" as such, and claiming otherwise was this
// comment's own defect (review finding 5).
// canvas-authority-contract.test.ts walks every production source file
// under folio-designer/src — FontChainEditor.tsx among them, by
// directory walk rather than by name — and fails if any of them restates
// the ENGINE'S REFUSAL VOCABULARY. That is one rule, not all of them.
// The rest of this paragraph is an engineering rule the reviewer of a
// browser change enforces, and the Go-side half of the contract — that
// the engine really emits the shape the browser's guard requires — is
// pinned by canvas_font_chain_entry_test.go.
//
// Family is NEVER EMPTY for an embedded entry. When the asset declares
// no `font.family`, the ASSET KEY is projected as the family — the
// engine chooses what the panel shows, so the browser never has to
// decide what to do with an empty name. Showing a 64-character digest is
// the honest answer for a document that named its own face nothing;
// inventing a name here would be the engine guessing.
//
// All SEVEN keys are ALWAYS emitted (no omitempty, deliberately). The
// browser checks this object with an exact-key guard, so a key that
// appears only for some entries is a key that rejects the whole snapshot
// for some documents — and the symptom is a blank canvas.
//
// STORY 11.3 / DW-239: Bold, Italic and BoldItalic are the entry's own
// DECLARED style variants, projected so the panel can READ BACK what the
// document declares. They exist for one question the panel could not
// otherwise answer — does this chain declare a bold cut at all? — and
// the epic's rule is that an absent cut is STATED, never shown as a
// reachable on-state.
//
// THEY ARE COPIED VERBATIM, NEVER CONSTRUCTED (D-11.2.1 / D-11.2.2).
// Nothing here appends " Bold", parses a face name, reads a name table
// or sniffs OS/2. "" is absence, exactly as it is on FontChainEntry.
//
// A SIBLING'S NAMESPACE MATCHES ITS ENTRY'S DISCRIMINANT (AD-8). On a
// `face` entry these are FontSet face names; on an `asset` entry they
// are `assets` keys. The projection never crosses the two, because it
// reads them off the entry that already carries the discriminant.
//
// THIS IS NOT THE RESOLVER. Which face a PAINTED fragment ends up in is
// CanvasTextFragment.Face, decided by shapeSegments against coverage.
// A chain entry's declared variant is what the DOCUMENT says, and the
// two are different facts: a declared bold that does not cover a rune is
// not the face that rune is drawn in.
type CanvasFontChainEntry struct {
	Face     string `json:"face"`
	AssetKey string `json:"assetKey"`
	Family   string `json:"family"`
	Style    string `json:"style"`

	Bold       string `json:"bold"`
	Italic     string `json:"italic"`
	BoldItalic string `json:"boldItalic"`
}
