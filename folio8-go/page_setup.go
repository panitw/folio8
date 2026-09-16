package folio8

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/expr"
	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/layout"
	"github.com/panitw/folio8/folio8-go/internal/template"
	"github.com/panitw/folio8/folio8-go/internal/text"
)

// maxCanvasPropertyString bounds an IDENTIFIER, a COLOUR or an EXPRESSION —
// a font-family name, `color`, `background`, `border.color`, `visibleIf` and
// a table's `bind`. SEVEN sites that ABORT, all legitimately short: Epic 7
// makes none of them newly reachable, so that residue is recorded rather than
// fixed (DW-25).
//
// ⚠ STORY 14.9 ADDED TWO SITES THAT DO NOT ABORT, AND THAT IS THE WHOLE POINT
// OF THEM. A table COLUMN's `label` and its own `bind` are bounded by this same
// constant and are CLIPPED to it (clipCanvasPropertyString), never refused and
// never dropped. `decodeColumn` imposes no length cap on either field, so a
// document with a 600-byte column label LOADS and PRINTS today — measured: it
// renders a real 64,123-byte PDF — and aborting the projection for it would
// terminate the worker with no respawn, blanking the designer for a template
// that prints correctly. A document the engine prints is a document the canvas
// draws. Clipping is a display concern on a surface that is display-only paint
// by [R1], and the column is never omitted: a dropped column would put the
// chip's `5 columns` above four drawn headers, which is being systematically
// wrong about STRUCTURE — precisely the harm AD-17 exists to prevent.
//
// ⚠ THE COUNTS ABOVE ARE PROSE AND THEREFORE LOSSY, and this comment has been
// wrong twice before. DW-104 holds the remedy (derive the site list from this
// file by grep and assert the probe table covers it minus a NAMED exception
// list). Story 14.9 covered its OWN two sites — as clipping assertions, since
// they do not refuse — and deliberately did not close the wider gap.
//
// It used to bound document body text as well, which is the two-jobs
// conflation D-7.4.2 §3 ruled must be SPLIT rather than raised: 512 bytes is
// ~80 English words, or ~170 Thai/CJK characters at three bytes each, which
// is less than one numbered contract clause. Body text now has its own
// derived bounds below; this constant governs identifiers only.
const maxCanvasPropertyString = 512

// maxCanvasTextFragments is the PER-LINE fragment guard, unchanged at 512 and
// deliberately a different quantity from the cumulative per-element budget
// below. It bounds one degenerate line; the browser mirrors only the
// cumulative count, so the two are not each other's mirror and the tie
// assertion does not pair them.
//
// Its sibling `maxCanvasTextLines = 256` is GONE, not renamed: 256 lines is
// about five pages at 11pt — maxCanvasBodyTextLines' own derivation below
// establishes 48 lines to an A4 page at that size — an order of magnitude
// short of the epic's own
// forty-page target, and D-7.4.2 rejected raising it in place — the cliff
// was the defect, its position was not.
const maxCanvasTextFragments = 512

// maxCanvasBodyText is a CHANNEL-REPRESENTABILITY BACKSTOP, not a paint
// bound, and it is the ONE body-text site that still refuses rather than
// degrades. Degradation lives on the paint side alone (D-7.4.2 §1): a
// component's Value is what the properties panel edits and SAVES, so
// shortening it would write the truncation into the author's document.
//
// Criterion: it must not be able to bind before the paint bounds do. The
// largest document the paint bounds admit is maxCanvasBodyTextLines lines of
// ~90 characters, and the worst case is three bytes per character (Thai/CJK,
// which NFR3 makes first-class): 1920 × 90 × 3 = 518 400 bytes. The next
// power of two above that is 524 288 (512 KiB); this constant is the one
// ABOVE it, and that extra doubling is deliberate rather than arithmetic —
// the ~90-character line is an ESTIMATE of a line's width in characters, not
// a bound on it, so the backstop is set a full power of two clear of the
// estimate. Epic 7's own input cannot reach either figure. Recorded, not
// fixed — following D-7.2.3's precedent for a stated sanity ceiling.
const maxCanvasBodyText = 1048576

// maxCanvasBodyTextLines is Epic 7's own forty-page target, measured:
//
//	40 pages × ⌊729890 mp content-band height ÷ 14982 mp advance⌋ = 40 × 48
//
// where 729890 mp is ContentHeight for a canonical A4 page (841890 mp tall,
// internal/layout/band.go) with 36pt margins and 20pt header and footer, and
// 14982 mp is the measured Advance of the shipped ["Noto Sans"] chain at
// 11pt. (At 12pt the advance is 16344 mp and a page holds 44 lines, so 11pt
// is the admitting figure of the two.) Past this the element paints its
// first N lines and sets Truncated — it never aborts the projection.
const maxCanvasBodyTextLines = 1920

// maxCanvasBodyTextFragments is the CUMULATIVE per-element fragment budget,
// mirroring the browser validator's own cumulative count (Go's per-line
// maxCanvasTextFragments bounds a different quantity, and the Go side must
// not emit what the browser will reject).
//
// Criterion: the same forty-page document, justified at full A4 content
// width, where Story 7.3 makes a justified line project one fragment per
// word-piece.
//
// MEASURED, at the closing revision of Story 7.4 and with the value cap
// lifted, through CanvasWithTextPaint itself: justified English contract
// prose at 11pt in the shipped ["Noto Sans"] chain — the same face and size
// maxCanvasBodyTextLines is derived from — across 523.276 pt of A4 content
// width gives 18.05 fragments per line over 101 lines (1 823 fragments for
// 1 824 words). 1920 × 18.05 = 34 656, and the next power of two above that
// is 65 536.
//
// 65 536 also clears a SHORT-WORD worst case measured the same way — "the cat
// sat on a mat" prose packs 30.86 fragments per line, and 1920 × 30.86 =
// 59 251 — so the forty-page criterion holds for text denser than a
// contract's, not only for the corpus it was measured on. (The earlier 16.72
// figure was a thirteen-line sample, where a justified block's short last
// line still moves the average; the earlier 19.35 was the Roboto-Regular TEST
// face rather than the shipped chain. Both are superseded by the figure
// above, which deferred-work.md and epic-7-8-decision-log.md now also carry.)
//
// The geometry-free law behind it: a justified component's cumulative
// fragment count ≈ the value's WORD COUNT, at any column width.
const maxCanvasBodyTextFragments = 65536

// snapToGrid is the reusable core-command seam for Story 5.7 placement.
// It uses the fixed six-point grid and half-away-from-zero rule; callers pass
// millipoints and never browser pixels.
func snapToGrid(proposed geom.Length) (geom.Length, bool) {
	return proposed.SnapNearest(geom.Length(designer.GridIncrement))
}

// imageUnavailableMissing / imageUnavailableUndecodable are
// ImageUnavailable's only two values (Finding 9). Kept as named constants,
// not inline literals, so the Go producer and any future consumer cannot
// drift on spelling.
const (
	imageUnavailableMissing     = "missing"
	imageUnavailableUndecodable = "undecodable"
)

const maxCanvasBindingString = 256

// maxCanvasFontFamilies bounds the projected name list the way every other
// list in this projection is bounded. A document declaring more chains than
// this is refused a projection with a stated reason, never silently cut.
const maxCanvasFontFamilies = 256

// canvasFontChains is the projection of the document's declared chains, in
// sorted key order: every chain template.Fonts.Chain accepts — declared AND
// non-empty — and no other. It ASKS Fonts.Chain that question rather than
// re-implementing it, which is what makes the sentence above a description of
// the code instead of a second copy of it. The comment this replaced named a
// different function as the authority ("exactly the names knownFontFamily
// accepts") while spelling the test out again three lines later; naming a
// caller rather than the rule is exactly how that drift started.
func canvasFontChains(t *Template) ([]designer.CanvasFontChain, error) {
	chains := make([]designer.CanvasFontChain, 0, len(t.doc.Fonts))
	// slices.Sorted(maps.Keys(...)) is the module's one way to walk a map:
	// map order is not an order, and this list is projected output.
	for _, name := range slices.Sorted(maps.Keys(t.doc.Fonts)) {
		entries, ok := t.doc.Fonts.Chain(name)
		if !ok {
			continue
		}
		if len(name) > maxCanvasPropertyString {
			return nil, fmt.Errorf("folio8: font family name exceeds the projection bound")
		}
		if len(entries) > maxCanvasFontChainEntries {
			return nil, fmt.Errorf("folio8: font chain declares more entries than the projection bound")
		}
		projected := make([]designer.CanvasFontChainEntry, 0, len(entries))
		for _, entry := range entries {
			p, perr := projectFontChainEntry(t, entry)
			if perr != nil {
				return nil, perr
			}
			projected = append(projected, p)
		}
		chains = append(chains, designer.CanvasFontChain{Name: name, Entries: projected})
	}
	if len(chains) > maxCanvasFontFamilies {
		return nil, fmt.Errorf("folio8: document declares more font families than the projection bound")
	}
	return chains, nil
}

// projectFontChainEntry projects ONE entry, and applies
// maxCanvasPropertyString to EVERY string it puts on the wire — the face
// name, the asset key, the family, the style and, since Story 11.3, the
// three declared style variants alike. A bound applied to four of seven
// fields is a bound on nothing: the projection is refused with a stated
// reason rather than silently cut, which is the rule every other list in
// this projection already follows.
//
// The family and style are read from the asset's `font` record. An
// explicit `null` there is treated as absence for DISPLAY purposes —
// the file keeps the distinction (Presence round-trips it), but a panel
// has nothing to draw for a null, and it is not the browser's job to
// decide that.
func projectFontChainEntry(t *Template, entry template.FontChainEntry) (designer.CanvasFontChainEntry, error) {
	var out designer.CanvasFontChainEntry
	if entry.Embedded() {
		out.AssetKey = entry.AssetKey
		out.Family = entry.AssetKey
		if asset, ok := t.doc.Assets[entry.AssetKey]; ok && asset.Font.Set && !asset.Font.Null {
			record := asset.Font.Value
			if record.Family.Set && !record.Family.Null && record.Family.Value != "" {
				out.Family = record.Family.Value
			}
			if record.Style.Set && !record.Style.Null {
				out.Style = record.Style.Value
			}
		}
	} else {
		out.Face = entry.Face
	}
	// THE DECLARED VARIANTS, VERBATIM, FROM THE ENTRY'S OWN FIELDS.
	// Read through FontChainEntry.Variant so the closed set is the
	// model's one table (fontChainVariants) rather than a second list
	// here; "" comes back for a variant the entry does not declare, and
	// "" is exactly how this projection spells absence.
	out.Bold = entry.Variant(template.FontStyleBold)
	out.Italic = entry.Variant(template.FontStyleItalic)
	out.BoldItalic = entry.Variant(template.FontStyleBoldItalic)
	for _, s := range []string{out.Face, out.AssetKey, out.Family, out.Style, out.Bold, out.Italic, out.BoldItalic} {
		if len(s) > maxCanvasPropertyString {
			return designer.CanvasFontChainEntry{}, fmt.Errorf("folio8: font chain entry exceeds the projection bound")
		}
	}
	return out, nil
}

// canvasFontFamilyNames is FontFamilies, derived from FontChains rather than
// walked a second time: FontChains[i].Name == FontFamilies[i] then holds BY
// CONSTRUCTION, which is what lets the browser cross-check the two lists
// against each other and lets the single Fonts.Chain authority govern both.
func canvasFontFamilyNames(chains []designer.CanvasFontChain) []string {
	names := make([]string, 0, len(chains))
	for _, chain := range chains {
		names = append(names, chain.Name)
	}
	return names
}

// canvasPageGeometry is THE one layout.PageGeometry the canvas builds, and
// every canvas consumer of a page-geometry quantity reads it: the content
// band rectangle, the projected window height and the window count all come
// from this single struct, so they cannot diverge from one another.
//
// It is deliberately NOT render.go's pageGeometryOf. That one routes through
// pageDimensions, which hard-errors on "Letter" by design — failing loudly is
// more honest than a silent A4 substitution when a PDF is about to be
// produced. canvasDimensions supports Letter, and a Letter document projects
// a canvas today, so routing the canvas through the render path's spelling
// would break a projection that works.
func canvasPageGeometry(t *Template) (layout.PageGeometry, error) {
	w, h, err := canvasDimensions(t)
	if err != nil {
		return layout.PageGeometry{}, err
	}
	m := t.doc.Page.Margin
	return layout.PageGeometry{
		Width:            w,
		Height:           h,
		MarginTop:        m.Top,
		MarginBottom:     m.Bottom,
		MarginLeft:       m.Left,
		MarginRight:      m.Right,
		PageHeaderHeight: t.doc.Bands.PageHeader.Height.Value,
		PageFooterHeight: t.doc.Bands.PageFooter.Height.Value,
	}, nil
}

// bandsLeaveContentWindow is THE content-window invariant, in ONE place, and
// it is asked by TWO callers with two different audiences (Story 12.1, Q3).
//
// Canvas asks it while LOADING: a document whose two capping bands eat the
// whole printable column cannot be laid out at all, and its refusal is the
// bare sentence it always was. setBandHeight asks it while AUTHORING, of a
// CANDIDATE height the author just typed, and owes a located message naming
// that height — a different sentence about the same arithmetic.
//
// The arithmetic is therefore written down once. Two callers phrasing their
// own refusals over one predicate is the shape Q3's ruling required; two
// spellings of `header >= innerH-footer` is the shape it forbade, because the
// version that drifts is always the one the author never reaches.
//
// The content region must be STRICTLY positive: header+footer == innerH is
// refused, because a content band of zero height is a document with nowhere
// for content to be, not a document with a very short one.
//
// THE BOUND IT ACCEPTS UP TO IS PART OF THE PREDICATE, not a number a caller
// re-derives to put in a sentence. setBandHeight has to TELL the author which
// numbers are left, and a message that computed `innerH-other` for itself
// would be a second spelling of this arithmetic living inside the one function
// whose job was to call it — the drift would show up only as a refusal quoting
// a bound the check does not hold. So the ceiling is named here and the
// predicate is written in terms of it; bandContentWindowCeiling is the LARGEST
// height this returns true for, and nothing else may say what that is.
func bandsLeaveContentWindow(header, footer, innerH geom.Length) bool {
	return header >= 0 && footer >= 0 && header <= bandContentWindowCeiling(footer, innerH)
}

// bandContentWindowCeiling is the tallest a capping band may be beside a
// sibling of `other` in a printable column of `innerH`: one millipoint short
// of the whole column, because the content region must be strictly positive
// and a geom.Length is an integer count of millipoints.
func bandContentWindowCeiling(other, innerH geom.Length) geom.Length {
	return innerH - other - 1
}

// canvas returns immutable paint geometry. It intentionally exposes neither
// template fields nor elements, canonical bytes, or browser measurements.
//
// ContentWindowCount is a documented ONE window here, declared NOT EXACT.
// Counting windows needs shaped lines, which needs a FontSet this entry point
// does not receive — and it does not need to: every projection that reaches
// the browser is a CanvasWithTextPaint (folio8-go/internal/wasm/engine.go's three seams),
// because every mutating command's own Canvas(t) is discarded and recomputed
// there with fonts. One window is what a column with nothing placeable in it
// occupies anyway; a silent zero would be a page count no document has. The
// number is not a claim of any kind — neither a floor nor a ceiling — which
// is exactly what the flag below says.
//
// ContentWindowOrigins and ContentWindowCountIsExact are the SAME admission,
// spelled in the two fields that carry it: one window beginning at column
// offset zero, declared NOT EXACT. ⚠ THE SENSE OF THAT FIELD IS INVERTED
// FROM THE ONE IT REPLACED, and this literal is the site where a mechanical
// rename would have converted a documented shortfall into a claim of
// exactness — the flag reads `false` here for the same reason it used to
// read `true`. The struct is shared with the entry point that can shape, so
// these values never reach the browser — but a shared struct's values must
// be honest wherever they are set, and a `nil` origins slice would marshal to
// a JSON null the protocol rejects.
func canvas(t *Template) (designer.CanvasProjection, error) {
	if t == nil {
		return designer.CanvasProjection{}, errNilTemplate
	}
	g, err := canvasPageGeometry(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	w, h := g.Width, g.Height
	m := t.doc.Page.Margin
	header, footer := g.PageHeaderHeight, g.PageFooterHeight
	if w <= 0 || h <= 0 || m.Left < 0 || m.Right < 0 || m.Top < 0 || m.Bottom < 0 || m.Left >= w-m.Right || m.Top >= h-m.Bottom {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page setup leaves no positive content region")
	}
	for _, v := range []geom.Length{w, h, m.Top, m.Right, m.Bottom, m.Left, header, footer} {
		if v < 0 || v > geom.Length(designer.MaxCanvasMillipoints) {
			return designer.CanvasProjection{}, fmt.Errorf("folio8: page setup exceeds the JavaScript-safe geometry bound")
		}
	}
	innerW, innerH := w-m.Left-m.Right, h-m.Top-m.Bottom
	if !bandsLeaveContentWindow(header, footer, innerH) {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page setup leaves no positive content region")
	}
	preset := "custom"
	if t.doc.Page.SizeIsName {
		preset = t.doc.Page.SizeName
	}
	commandW, commandH := w, h
	if !t.doc.Page.SizeIsName {
		commandW, commandH = t.doc.Page.SizeCustom.Width, t.doc.Page.SizeCustom.Height
	}
	// AD-13: the content band's height is derived by ONE function, in
	// internal/layout. The inline `innerH - header - footer` that used to
	// stand here was arithmetically identical and still a second spelling of
	// a derived quantity — and the projection now REPORTS this number as the
	// page-height window, which is what the designer draws sheet boundaries
	// from, so a divergence would show up as the canvas and the engine
	// drawing different pages while agreeing on every byte.
	window := layout.ContentHeight(g)
	bands := []designer.CanvasBand{
		{Name: bandPageHeader, X: int64(m.Left), Y: int64(m.Top), Width: int64(innerW), Height: int64(header)},
		{Name: bandContent, X: int64(m.Left), Y: int64(m.Top + header), Width: int64(innerW), Height: int64(window)},
		{Name: bandPageFooter, X: int64(m.Left), Y: int64(h - m.Bottom - footer), Width: int64(innerW), Height: int64(footer)},
	}
	components, err := canvasComponents(t, bands)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	chains, err := canvasFontChains(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	pageBreaks := make([]bool, t.doc.PageCount())
	pageBreaks[0] = true
	if len(pageBreaks) > 1 {
		for page := 1; page < len(pageBreaks); page++ {
			pageBreaks[page] = t.doc.Pages[page].PageBreak
		}
		pageOf := contentPageIndex(t)
		for index := range components {
			if components[index].Band == bandContent {
				components[index].Page = pageOf[components[index].ID]
			}
		}
	}
	// spec-section-break CAP-6, per designed page (SPEC-multi-pages CAP-6). A
	// one-page projection keeps sectionBreak and sectionBreakAnchor exactly; a
	// multi-page projection carries one entry per page in sectionBreaks and
	// sectionBreakAnchors instead. belowSectionBreak is set exactly on the
	// content components whose own page has a break.
	var sectionBreak *int64
	var sectionBreakAnchor *bool
	var sectionBreaks []*int64
	var sectionBreakAnchors []bool
	pageCount := t.doc.PageCount()
	offsets := make([]*int64, pageCount)
	for page := range offsets {
		if offset, ok := declaredSectionBreak(t, page); ok {
			value := int64(offset)
			offsets[page] = &value
		}
	}
	if pageCount == 1 {
		sectionBreak = offsets[0]
		if sectionBreak != nil && !sectionBreakAnchored(t, 0) {
			unanchored := false
			sectionBreakAnchor = &unanchored
		}
	} else {
		sectionBreaks = offsets
		sectionBreakAnchors = make([]bool, pageCount)
		for page := range sectionBreakAnchors {
			sectionBreakAnchors[page] = offsets[page] == nil || sectionBreakAnchored(t, page)
		}
	}
	var pageOf map[string]int
	for index := range components {
		if components[index].Band != bandContent {
			continue
		}
		page := 0
		if pageCount > 1 {
			if pageOf == nil {
				pageOf = contentPageIndex(t)
			}
			page = pageOf[components[index].ID]
		}
		if offsets[page] == nil {
			continue
		}
		below := components[index].Y >= *offsets[page]
		components[index].BelowSectionBreak = &below
	}
	return designer.CanvasProjection{SectionBreak: sectionBreak, SectionBreakAnchor: sectionBreakAnchor, SectionBreaks: sectionBreaks, SectionBreakAnchors: sectionBreakAnchors, Width: int64(w), Height: int64(h), Locale: t.doc.Locale, UTCOffset: t.doc.UTCOffset, Orientation: t.doc.Page.Orientation, Preset: preset, MarginTop: int64(m.Top), MarginRight: int64(m.Right), MarginBottom: int64(m.Bottom), MarginLeft: int64(m.Left), GridIncrement: designer.GridIncrement, CommandWidth: int64(commandW), CommandHeight: int64(commandH), Bands: bands, Components: components, FontFamilies: canvasFontFamilyNames(chains), FontChains: chains, DefaultFontSize: int64(defaultFontSizePt), DefaultLineSpacing: defaultLineSpacing, ContentWindowHeight: int64(window), ContentWindowCount: int64(t.doc.PageCount()), ContentWindowOrigins: canvasOnePerPageOrigins(t.doc.PageCount()), ContentWindowPages: canvasOnePerPagePages(t.doc.PageCount()), PageBreaks: pageBreaks, ContentWindowCountIsExact: false}, nil
}

// canvasOnePerPageOrigins and canvasOnePerPagePages are the window sequence
// of a projection that has not paginated: one window per designed page, each
// beginning at its page's top. For a one-page document, [0] and [0].
func canvasOnePerPageOrigins(pages int) []int64 {
	return make([]int64, pages)
}

func canvasOnePerPagePages(pages int) []int {
	out := make([]int, pages)
	for i := range out {
		out[i] = i
	}
	return out
}

// canvasWindowPage is window index's designed page, 0 when the projection
// carries no page for it.
func canvasWindowPage(projection designer.CanvasProjection, index int) int {
	if index < 0 || index >= len(projection.ContentWindowPages) {
		return 0
	}
	return projection.ContentWindowPages[index]
}

// canvasWithTextPaint returns Canvas geometry augmented with a read-only,
// production-parity text paint plan. It is session output only: it never
// mutates the template or its canonical serialization.
func canvasWithTextPaint(t *Template, fs FontSet) (designer.CanvasProjection, error) {
	projection, err := canvas(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	// One shaping, two consumers. addCanvasTextPaint shapes the content
	// band's text once; the paint plan is one consumer of the extents that
	// shaping produced and the window count is the other. A second shaping
	// pass would be a second derivation of the same numbers, which is the
	// thing internal/layout's ColumnItem doc forbids.
	column := canvasColumnExtents{Items: make([]layout.ColumnItem, 0)}
	if err := addCanvasTextPaint(t, &projection, fs, &column); err != nil {
		return designer.CanvasProjection{}, err
	}
	if err := addCanvasImagePaint(t, &projection); err != nil {
		return designer.CanvasProjection{}, err
	}
	if err := addCanvasBarcodePaint(t, &projection); err != nil {
		return designer.CanvasProjection{}, err
	}
	if err := addCanvasWindowCount(t, &projection, column); err != nil {
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

// canvasColumnExtents is what addCanvasTextPaint hands addCanvasWindowCount:
// the content column's per-line extents, and the ONE thing about them the
// count cannot see for itself.
//
// Items is the same slice this function used to pass on its own. FontChain-
// Degraded is the addition: a content-band text element the canvas cannot
// shape — its font chain would not RESOLVE, or (since D-8.4.12) a face that
// chain names would not PARSE — is skipped with an empty paint a few lines
// into addCanvasTextPaint, and the extents it would have contributed are
// simply absent from Items. The count that measures Items is therefore a
// FLOOR for that document, and nothing downstream of Items could tell — the missing
// lines look exactly like an element that had nothing to say. Carrying the
// fact beside the extents is what keeps the flag an ENGINE fact rather than
// a browser rule about what an empty paint might mean.
type canvasColumnExtents struct {
	Items             []layout.ColumnItem
	FontChainDegraded bool
}

// canvasWindowOrigins reads the window tops out of the very Pagination the
// count is taken from: PageAssignment.Shift IS window i's band-relative
// column offset, already computed by the one function permitted to decide
// where a page begins. Nothing is derived, and in particular nothing is
// multiplied — `index * ContentWindowHeight` is the closed form
// internal/layout/paginate.go forbids by name, and it is wrong here by 110
// millipoints per window on a column of round 728pt spacing and by nine whole
// windows on a column with a declared gap.
//
// It STATES the three properties the browser protocol independently requires
// — a non-empty sequence, a first origin of zero, strictly increasing — and
// refuses rather than returning a sequence that would fail them. A refused
// sequence degrades exactly as a refused pagination does; a sequence the
// protocol rejects would discard the whole snapshot and blank the canvas with
// nothing to attribute the blank to.
func canvasWindowOrigins(plan layout.Pagination) ([]int64, bool) {
	if len(plan.Pages) == 0 {
		return nil, false
	}
	origins := make([]int64, 0, len(plan.Pages))
	for i, page := range plan.Pages {
		shift := page.Shift
		if shift < 0 || shift > geom.Length(designer.MaxCanvasMillipoints) {
			return nil, false
		}
		if i == 0 && shift != 0 {
			return nil, false
		}
		if i > 0 && int64(shift) <= origins[i-1] {
			return nil, false
		}
		origins = append(origins, int64(shift))
	}
	return origins, true
}

// canvasContentBandHasBoundTable is cause (a) of ContentWindowCountIsExact's
// register, and the one that reads as a FLOOR: a table in the content band
// with a non-empty binding. projectedSize gives
// such a table its header height and not one row, because the canvas has
// never been given the data — so the column being counted is one header tall
// however many hundred rows the finished document runs to.
func canvasContentBandHasBoundTable(t *Template) bool {
	for _, element := range contentElements(t) {
		if element.Type == template.ElementTable && element.Table.Set && !element.Table.Null && element.Table.Value.Bind != "" {
			return true
		}
	}
	return false
}

// canvasElementIsPlaced answers, for a NON-TEXT content element, whether the
// render path would contribute a content-column item for it — the question
// addCanvasWindowCount's own arm must answer the same way, or the canvas
// counts a column the document does not have.
//
// Three routes reach the render path's contentColumnItems, and the kinds
// divide by which ones they can take (page_number.go). NO KIND IS PLACED
// UNCONDITIONALLY — each route has a condition, every one of them is a pure
// property of the template, and this predicate is the conjunction the canvas
// can therefore evaluate with no data at all:
//
//	table — its header rect, from collectBandTableRuns, and never through
//	        element_box.go, which excludes a table by name. Placed while it
//	        declares at least one COLUMN: `"columns": []` parses, and a
//	        table with no columns has nothing to lay out or draw.
//	image — its image run, from collectImageRuns, which is placed while the
//	        element's `asset` is present and non-null; createComponent gives
//	        every newly dropped image a NULL asset, so an unfilled box is
//	        the designer's ordinary state and not an exotic one. An image
//	        may ALSO declare a box, and a styled one reaches a column item
//	        that way even with no file chosen — so the two routes are OR-ed
//	        rather than ranked.
//	rect,
//	line  — element_box.go's rect source, and NOTHING ELSE. So these two are
//	        placed exactly where they declare a box.
//
// Each condition is the render path's own predicate, called rather than
// restated: one authority per route, two callers each, the same shape as
// keepTogetherTags. elementDeclaresBox carries BOTH halves of the box rule —
// the style declaration and the positive rectangle — so a styled element of
// zero height is placed by neither side.
//
// WHAT THIS DOES NOT ANSWER, deliberately: whether the element is VISIBLE.
// The render path consults its visibility verdicts before asking any of
// these questions, and the canvas has no data to resolve a visibleIf with —
// which is why conditional visibility is a registered cause of inexactness
// rather than a clause here.
func canvasElementIsPlaced(element template.Element) bool {
	switch element.Type {
	case template.ElementTable:
		return tableDrawsColumns(element)
	case template.ElementImage:
		return imageDrawsItsAsset(element) || elementDeclaresBox(element)
	case template.ElementBarcode, template.ElementQRCode:
		return canvasBarcodeIsPlaced(element)
	default:
		return elementDeclaresBox(element)
	}
}

// canvasContentBandHasConditionalVisibility is the cause named for what it
// IS — an element whose visibility depends on DATA — rather than for the
// story that found it.
//
// page_setup.go only PROJECTS VisibleIf, as a string; nothing on this path
// evaluates it, because evaluating it needs the data the canvas has never
// been given. So the canvas places the element and the render may omit it,
// and AD-24 makes a hidden element ABSENT WITH NO GAP — no sibling moves up
// into the space, the column is simply shorter, and the two counts differ.
//
// UNDISCLOSED SINCE STORY 7.5, which shipped the count, and 7.6, which
// shipped the flag. It applies to an UNGROUPED element carrying visibleIf
// exactly as much as to a grouped one: grouping did not create this cause, it
// is only how it was found, because a conditional member makes a group's
// whole slide conditional and that is loud enough to measure.
//
// Content band only, like every other cause here: this flag is a claim about
// the content column, and a conditional element in a repeated band changes
// what that band draws, never how many windows the column occupies.
//
// PRESENT AND NON-NULL IS THE WHOLE TEST. It carried a third clause,
// `Value != ""`, which was unreachable — the loader refuses an empty
// expression outright ("visibleIf \"\" is not a valid expression: empty
// expression") — and unreachable in the unsafe direction: had a document
// ever carried one, that clause would have made this flag claim exactness
// for an element whose placement the canvas cannot decide. A hazard test
// does not need an escape hatch for a document the loader cannot produce.
func canvasContentBandHasConditionalVisibility(t *Template) bool {
	for _, element := range contentElements(t) {
		if element.VisibleIf.Set && !element.VisibleIf.Null {
			return true
		}
	}
	return false
}

// addCanvasWindowCount is the THIRD paint producer, beside addCanvasTextPaint
// and addCanvasImagePaint: it reports how many page-height windows the
// content column occupies, WHERE EACH ONE BEGINS, and whether that number can
// be trusted as the printed document's page count.
//
// It calls internal/layout's Paginate — the one function that decides how
// many pages a column has — and never a second pagination of its own. What it
// supplies is only the extents: the per-line tops and advances the paint plan
// already computed (textItems), plus one item per non-text content component
// from the box canvasComponents already projects. Paginate's signature takes
// PageGeometry and ColumnItems, no data, no bindings and no template, because
// receiving caller-derived extents is exactly what it is for.
//
// A NON-TEXT COMPONENT CONTRIBUTES EXACTLY WHERE THE RENDER PATH PLACES ONE
// — canvasElementIsPlaced above is that whole rule, calling each route's own
// predicate rather than restating it — and the sentence here used to say the
// opposite ("every content component contributes, styled or not"). It was
// wrong, and Story 7.9 is where it started to matter: an unstyled rect
// reaches no column item on the render path, so a canvas that counted it drew
// a window the printed document does not have. That was invisible while the
// two answers were both ungrouped and merely differed on origins; the moment
// a declared group made the canvas's partition matter, it became a count that
// was confidently wrong.
//
// ⚠ THE SENTENCE IS QUALIFIED FOR A REASON, AND THE QUALIFIER IS TEXT. It
// holds for the kinds this arm carries and NOT in the other direction for
// text, which is why "non-text" is not decoration. Text contributes one item
// per SHAPED LINE and never its box, so a text element that shapes no lines
// at all — an unset, null or empty value, or a font chain that cannot be
// resolved — contributes nothing here, exactly as the render path treats a
// value that binds to empty. But collectElementBoxRects accepts TEXT as well
// (element_box.go's four eligible kinds are text, image, rect and line), so a
// content-band text element declaring a background or a border ALSO
// contributes a full-declared-box column item on the render path — and the
// canvas MIRRORS that item below, from elementDeclaresBox, the same single
// predicate the render path uses. Before it did, a styled text element at y
// 700 with a declared height of 200 and a one-line value gave
// `RENDER pages=2 | CANVAS windows=1 exact=true`: a count that was wrong and
// a flag that denied it.
//
// THAT DIVERGENCE IS CLOSED, and closing it is why this arm carries NO
// exactness cause for a styled text box. A cause clears the flag to say the
// canvas CANNOT KNOW; each of the four below is exactly that — no data for a
// bound table, an unresolvable font chain, a data-dependent visibility
// verdict, a pagination that degraded. A declared box is none of them: it is
// declared geometry, fully knowable with no data at all. A fifth cause once
// stood here, and clearing the flag over a count that is now provably right
// made it a false statement with a conservative sign — the same defect as the
// false `true` it replaced, in the other polarity, rather than a repair of
// it. What guards the count instead is
// TestAStyledTextBoxCountsTheSameWindowsAsTheRenderPath, which reaches the
// number by the render path's own route and reds if the mirrored item below
// is reverted.
func addCanvasWindowCount(t *Template, projection *designer.CanvasProjection, column canvasColumnExtents) error {
	g, err := canvasPageGeometry(t)
	if err != nil {
		return err
	}
	// THREE of the flag's causes are known before Paginate runs — a bound
	// table, a degraded font chain, and an element whose visibility
	// depends on data; the fourth is the degradation branch below. Each
	// is a case where the canvas genuinely cannot know, which is the
	// admission bar for a cause here. They are OR-ed
	// rather than ranked because the flag reports that the count cannot be
	// trusted, not which cause made it so. ⚠ The SENSE is inverted from the
	// field this replaced: `exact` is true only when NONE of them applies.
	exact := !(column.FontChainDegraded ||
		canvasContentBandHasBoundTable(t) ||
		canvasContentBandHasBoundBarcode(t) ||
		canvasContentBandHasConditionalVisibility(t))
	// Story 7.9 (FR51): the same index addCanvasTextPaint tagged its line
	// items with, from the same one authority. Grouping is a pure property
	// of the Template — keepTogetherTags takes nothing else — so the canvas
	// already holds every input it needs to be RIGHT about it, and being
	// wrong about it is a defect rather than a cause to register beside the
	// causes above.
	keepTogether := keepTogetherTags(t)
	items := make([]layout.ColumnItem, 0, len(column.Items)+len(contentElements(t)))
	items = append(items, column.Items...)
	for _, element := range contentElements(t) {
		if element.Type == template.ElementText {
			// Text contributes one item PER SHAPED LINE, never its box: a
			// paragraph splits between windows at a line, which is what
			// makes the count a slide rather than a division.
			//
			// EXCEPT when it also declares a box, which the render path
			// places as its OWN full-height column item beside the line
			// items (element_box.go accepts text among its four eligible
			// kinds). Mirroring that item here is not a second reading of
			// anything: it is elementDeclaresBox, the same single
			// predicate, feeding the same one Paginate.
			if !elementDeclaresBox(element) {
				continue
			}
		} else if !canvasElementIsPlaced(element) {
			continue
		}
		_, height := projectedSize(element)
		items = append(items, layout.ColumnItem{
			ElementID: string(element.ID),
			Top:       element.Y,
			Bottom:    element.Y + height,
			// The dummy-ref idiom page_number.go already sanctions:
			// Paginate's exclusivity pre-pass requires exactly one of
			// Runs/Images/Rects to be non-empty, and this Pagination is
			// discarded except for len(Pages), so the value is never read
			// back.
			Rects: []layout.RectRef{0},
			// Story 7.9 (FR51): a rect, line, image or table box joins its
			// author-declared group here, so a signature's ruled line rides
			// with the name above it in the canvas's column exactly as it
			// does in the render's. This arm carries every non-text content
			// kind, so tagging only the text arm would group the column in
			// halves and the count would still diverge.
			Group: keepTogether.keepTogetherGroup(string(element.ID)),
		})
	}
	// ONE translation, in one place. Every extent above is band-relative,
	// exactly as the author declared it and as CanvasComponent carries it;
	// Paginate reads the printable frame, whose content origin is the page
	// header's height. MarginTop is deliberately NOT added — Origins measures
	// downward from the printable top edge, inside the margin, while
	// CanvasBand.Y is paper-absolute.
	origin := layout.Origins(g).Content
	for i := range items {
		items[i].Top += origin
		items[i].Bottom += origin
	}
	// SPEC-multi-pages CAP-8: each designed page is its own column, paginated
	// on its own, with page-local origins. A one-page document's items are
	// paginated exactly as before.
	pageCount := t.doc.PageCount()
	degrade := func() {
		// A pagination failure DEGRADES THE COUNT; it never fails the
		// projection. The reachable case is a content component taller than
		// one window, which this story newly makes authorable — turning the
		// render path's overflow into a canvas refusal would make a
		// canvas bound into a document validity rule. One window per page is
		// Paginate's own answer for a column it cannot place, and it is the
		// same shape as this file's other degradations: dispose of the
		// number, keep the canvas.
		//
		// The origins degrade with the count they describe — one window
		// beginning at the top of each page's column — and the flag says the
		// number is NOT EXACT, because a column Paginate could not place is
		// emphatically not a prediction of the document's length. A sequence
		// that would not survive the browser's own validation degrades the
		// same way: discarding the number is cheaper than discarding the
		// snapshot.
		projection.ContentWindowCount = int64(pageCount)
		projection.ContentWindowOrigins = canvasOnePerPageOrigins(pageCount)
		projection.ContentWindowPages = canvasOnePerPagePages(pageCount)
		projection.ContentWindowCountIsExact = false
	}
	split := contentPagesSplit{breaks: make([]sectionBreakSplit, pageCount)}
	if pageCount > 1 {
		split.pageOf = contentPageIndex(t)
	}
	origins := make([]int64, 0, pageCount)
	pages := make([]int, 0, pageCount)
	for page, pageItems := range split.partition(items) {
		plan, err := layout.Paginate(g, pageItems)
		if err != nil {
			degrade()
			return nil
		}
		pageOrigins, ok := canvasWindowOrigins(plan)
		if !ok {
			degrade()
			return nil
		}
		origins = append(origins, pageOrigins...)
		for range pageOrigins {
			pages = append(pages, page)
		}
	}
	projection.ContentWindowCount = int64(len(origins))
	projection.ContentWindowOrigins = origins
	projection.ContentWindowPages = pages
	projection.ContentWindowCountIsExact = exact
	return nil
}

// addCanvasImagePaint is addCanvasTextPaint's sibling for image components
// (D-5.13.2's "Producer" clause): a paint PRODUCER invoked from
// CanvasWithTextPaint, never computed inside setComponentAsset or any other
// command — every mutating command's own Canvas(t) is discarded and
// recomputed by folio8-go/internal/wasm/engine.go, so the paint must be derivable from
// template state alone, exactly like text paint.
//
// It builds each run in the BAND frame (element.X/Y untranslated by band
// origin, matching this component's own X/Y) and calls the same
// resolveImagePlacement collectImageRuns/renderDocument use for the PDF —
// never a second fit computation. canvas_image_paint_test.go asserts that
// this band-relative rectangle is exactly a translation of the page-absolute
// one collectImageRuns/resolveImagePlacement produce, rather than assuming
// it from the two call sites merely sharing a function.
//
// A missing asset key or a decode failure (unrecognised media type or
// malformed bytes) leaves this component's Image field absent and does NOT
// fail the whole projection — Render (render.go) is the located, fatal
// diagnostic for a genuinely broken document; this paint-only projection
// must stay paintable (AC3: "not a crash") even for a document a save
// cannot yet produce a clean render from.
func addCanvasImagePaint(t *Template, projection *designer.CanvasProjection) error {
	components := make(map[string]*designer.CanvasComponent, len(projection.Components))
	for i := range projection.Components {
		component := &projection.Components[i]
		components[component.ID] = component
	}
	for _, band := range []struct {
		name     string
		elements []template.Element
	}{
		{bandPageHeader, t.doc.Bands.PageHeader.Elements},
		{bandContent, contentElements(t)},
		{bandPageFooter, t.doc.Bands.PageFooter.Elements},
	} {
		for _, element := range band.elements {
			if element.Type != template.ElementImage {
				continue
			}
			component := components[string(element.ID)]
			if component == nil || component.Band != band.name {
				return fmt.Errorf("folio8: canvas image component %q is missing from geometry projection", element.ID)
			}
			if !element.Width.Set || !element.Height.Set || !element.Asset.Set {
				// Load-time validation (parse_bands.go) already makes these
				// required for a successfully parsed document — handled
				// rather than assumed, never reached in practice.
				continue
			}
			if element.Asset.Null {
				// A placed but unfilled box. Neither Image nor
				// ImageUnavailable is set: there is nothing to paint and
				// nothing has gone wrong, and that pairing is what tells the
				// designer to draw its empty placeholder instead of one of
				// the two failure texts.
				continue
			}
			assetKey := element.Asset.Value
			asset, ok := t.doc.Assets[assetKey]
			if !ok {
				missing := imageUnavailableMissing
				component.ImageUnavailable = &missing
				continue
			}
			raw, err := template.DecodeAssetBytes(asset)
			if err != nil {
				undecodable := imageUnavailableUndecodable
				component.ImageUnavailable = &undecodable
				continue
			}
			img, err := template.DecodeImageForRender(asset.MediaType, raw, assetKey, string(element.ID))
			if err != nil {
				undecodable := imageUnavailableUndecodable
				component.ImageUnavailable = &undecodable
				continue
			}
			run := imageRunSource{elementID: string(element.ID), assetKey: assetKey, x: element.X, y: element.Y, boxW: element.Width.Value, boxH: element.Height.Value}
			drawX, drawY, drawW, drawH := resolveImagePlacement(run, img)
			width, err := canvasDerived("image intrinsic width", geom.Length(img.Width()))
			if err != nil {
				return fmt.Errorf("folio8: canvas image element %s: %w", element.ID, err)
			}
			height, err := canvasDerived("image intrinsic height", geom.Length(img.Height()))
			if err != nil {
				return fmt.Errorf("folio8: canvas image element %s: %w", element.ID, err)
			}
			dx, err := canvasDerived("image draw x", drawX)
			if err != nil {
				return fmt.Errorf("folio8: canvas image element %s: %w", element.ID, err)
			}
			dy, err := canvasDerived("image draw y", drawY)
			if err != nil {
				return fmt.Errorf("folio8: canvas image element %s: %w", element.ID, err)
			}
			dw, err := canvasDerived("image draw width", drawW)
			if err != nil {
				return fmt.Errorf("folio8: canvas image element %s: %w", element.ID, err)
			}
			dh, err := canvasDerived("image draw height", drawH)
			if err != nil {
				return fmt.Errorf("folio8: canvas image element %s: %w", element.ID, err)
			}
			component.Image = &designer.CanvasImagePaint{
				MediaType:  asset.MediaType,
				AssetKey:   assetKey,
				Width:      int64(width),
				Height:     int64(height),
				DrawX:      int64(dx),
				DrawY:      int64(dy),
				DrawWidth:  int64(dw),
				DrawHeight: int64(dh),
			}
		}
	}
	return nil
}

func addCanvasTextPaint(t *Template, projection *designer.CanvasProjection, fs FontSet, column *canvasColumnExtents) error {
	components := make(map[string]*designer.CanvasComponent, len(projection.Components))
	for i := range projection.Components {
		component := &projection.Components[i]
		components[component.ID] = component
	}
	// Story 7.9: the document's OWN keep-together declarations, read
	// through the render path's single authority (keepTogetherTags,
	// render.go) rather than re-read here. It takes the *Template and
	// nothing else — no data, no params, no FontSet — which is precisely
	// why grouping is knowable canvas-side and is a DEFECT to omit rather
	// than a shortfall to disclose. See addCanvasWindowCount's own use of
	// it for the non-text arm; the two arms must tag from the same index
	// or the column is grouped in halves.
	keepTogether := keepTogetherTags(t)
	// FROM THE DOCUMENT (Story 8.4), and this is the second of the two
	// sites that must be — predictDocument (render.go) is the other. The
	// canvas consumes the IDENTICAL advance the renderer does (AD-17), so
	// a canvas cache that could not see the document's carried faces would
	// measure a document the PDF does not print.
	cache := newDocumentFontCache(t)
	// degrade disposes of ONE element and carries on, and it is spelled
	// once because two different conditions reach it (D-7.4.2: DEGRADE
	// THIS ELEMENT, NEVER ABORT THE PROJECTION). See both call sites
	// below for what each one is.
	degrade := func(band string, element template.Element, component *designer.CanvasComponent) {
		// AND THE WINDOW COUNT LOSES THIS ELEMENT'S EXTENTS. Nothing
		// downstream of the column could tell — an element with nothing
		// to say and an element that could not be shaped contribute the
		// identical nothing — so the fact is recorded here, where it is
		// known, and reported as the honesty flag. Only an element that
		// actually HAS a value loses anything: an unset or empty one
		// would have contributed no lines with a perfectly good chain.
		if band == bandContent && element.Value.Set && !element.Value.Null && element.Value.Value != "" {
			column.FontChainDegraded = true
		}
		component.TextPaint = &designer.CanvasTextPaint{Lines: []designer.CanvasTextLine{}}
	}
	for _, band := range []struct {
		name     string
		elements []template.Element
	}{
		{bandPageHeader, t.doc.Bands.PageHeader.Elements},
		{bandContent, contentElements(t)},
		{bandPageFooter, t.doc.Bands.PageFooter.Elements},
	} {
		for _, element := range band.elements {
			if element.Type != template.ElementText {
				continue
			}
			component := components[string(element.ID)]
			if component == nil || component.Band != band.name {
				return fmt.Errorf("folio8: canvas text component %q is missing from geometry projection", element.ID)
			}
			chain, styledChain, err := fontChain(t, element)
			if err != nil {
				// Existing designer documents can be structurally valid while
				// incomplete for production rendering (for example, a text
				// component without a chosen font chain). They remain loadable;
				// there is simply no honest measured paint to display yet.
				degrade(band.name, element, component)
				continue
			}
			// SCOPED TO THIS ELEMENT'S CHAIN (fontCache.forChain), shadowing
			// the shared cache for the rest of the iteration: the canvas is
			// the surface an author repairs a chain on, so a message about a
			// chain entry must name the chain their element draws through.
			cache := cache.forChain(element.Style.Value.FontFamily.Value)
			paint := &designer.CanvasTextPaint{Lines: []designer.CanvasTextLine{}}
			if !element.Value.Set || element.Value.Null || element.Value.Value == "" {
				component.TextPaint = paint
				continue
			}
			fontSize := defaultFontSizePt
			if element.Style.Set && !element.Style.Null && element.Style.Value.FontSize.Set && !element.Style.Value.FontSize.Null {
				fontSize = element.Style.Value.FontSize.Value
			}
			// The canvas reads the SAME engine measurement the PDF does
			// (AC6 / the Story 5.9 invariant), so it consumes the same
			// lists: coverage on the base chain, shaping on the styled
			// one, leading over every face that may draw. Its diagnostics
			// channel is discarded here, exactly as every other Warning
			// on this path is.
			metricsChain := metricsFaceNames(chain, styledChain, fs, cache)
			segs, _, err := shapeSegments(string(element.ID), chain, styledChain, element.Value.Value, fs, cache, breaksAreConsumed)
			if err != nil {
				// A CHAIN ENTRY THIS BUILD CANNOT DRAW WITH IS A DOCUMENT
				// THE FORMAT CALLS VALID, and it degrades exactly as an
				// unresolvable chain does above (D-7.4.2, stated at the
				// truncation arm below in this same function).
				//
				// Story 8.4 made this reachable from document CONTENT: a
				// chain entry naming a non-font asset loads (D-1.8.1 as
				// amended) and is refused at coverage resolution, which is
				// right for Render — the page would be wrong — and wrong
				// here. The designer is the ONE surface on which an author
				// can repair that entry, and a projection that returns an
				// error opens no document at all, so the defect would lock
				// the author out of its own repair.
				//
				// THE GATE IS POSITIONAL, NOT AN ENUMERATED ALLOWLIST
				// (D-8.4.12). It used to test err against ONE type,
				// *template.UnsupportedFontMediaTypeError, with an
				// errors.As allowlist justified HERE, in this comment, by
				// the claim that everything it excluded was "a genuine
				// internal shaping fault … not a document property, has no
				// author repair".
				// That claim was true about internal faults and FALSE
				// about the set an error-type gate actually excludes:
				// checkSfnt validates the table directory and never a
				// table's contents, so a carried face that is a
				// structurally valid sfnt over unreadable contents LOADS
				// and then aborted this whole projection at fontset.New —
				// a document property, with an author repair, closing the
				// surface the repair happens on. Same axis error as
				// D-7.3.1: the mechanism named was narrower than the
				// invariant meant.
				//
				// So the question asked is WHERE the fault arose, not what
				// type carries it. template.CarriedFaceError is stamped on
				// EVERY error out of fontCache.get's embedded arm — the
				// single door that resolves a face THIS DOCUMENT carries —
				// so a future face-resolution failure type joins this
				// degrade automatically instead of silently rejoining the
				// abort.
				//
				// AND THE ABORT BELOW IS REAL, not a leftover. A fault
				// arising AFTER every face this element needs has resolved
				// keeps aborting, and so does a face the CALLER's FontSet
				// supplies that will not parse — the host application's
				// face, which no edit on this canvas repairs (guardrail 6;
				// TestCanvasStillAbortsOnAHostFontSetFaceThatWillNotParse
				// is the assertion the retained half went without).
				var carried *template.CarriedFaceError
				if !errors.As(err, &carried) {
					return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
				}
				degrade(band.name, element, component)
				continue
			}
			ops := text.Opportunities(text.Dictionary(), element.Value.Value, placeholderSpans(element.Value.Value))
			boxWidth := geom.Length(0)
			if element.Width.Set && !element.Width.Null {
				boxWidth = element.Width.Value
			}
			lines := packLines(segs, ops, len([]rune(element.Value.Value)), fontSize, boxWidth)
			// D-7.4.2: DEGRADE THIS ELEMENT, NEVER ABORT THE PROJECTION.
			// This used to `return` an error, and that error was the
			// function's own — one over-long clause blanked the whole
			// canvas for a document that renders to a perfectly good PDF.
			// The shape reused here is the fontChain path a few lines
			// above: dispose of the one element and carry on. What is new
			// is that the element keeps its first N lines and SAYS it was
			// cut, rather than presenting an empty paint.
			//
			// The lines dropped here are dropped from the PAINT ONLY.
			// element.Value is untouched, is what the properties panel
			// saves, and renders whole.
			painted := lines
			if len(painted) > maxCanvasBodyTextLines {
				painted = painted[:maxCanvasBodyTextLines]
				paint.Truncated = true
			}
			// Overflow and the vertical origin below are still derived from
			// the FULL line list, so the prefix paints at exactly the
			// coordinates it occupies in the whole block: truncation must
			// not silently move the text the author can still see.
			_, paint.Overflow = detectWidthOverflow(string(element.ID), lines, boxWidth)
			// AC6 / the Story 5.9 invariant: the canvas consumes the
			// IDENTICAL advance the renderer does, ratio included — the
			// browser never measures text and never adjudicates what the
			// engine measured.
			vm, err := chainVerticalModel(metricsChain, fontSize, styleLineSpacing(element.Style), fs, cache)
			if err != nil {
				return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
			}
			// The same slack-only alignment rule the PDF producer applies
			// (text_alignment.go), from the same committed style: the canvas
			// has to show what prints. Both offsets are non-negative, so every
			// projected coordinate stays inside the band-relative, JS-safe
			// bound canvasLineTop and canvasDerived check.
			align, valign := elementAlignment(element)
			boxHeight := geom.Length(0)
			if element.Height.Set && !element.Height.Null {
				boxHeight = element.Height.Value
			}
			originY := element.Y + textValignOffset(valign, boxHeight, textBlockHeight(len(lines), vm))
			// THE WINDOW COUNT'S EXTENTS, taken here and nowhere else.
			//
			// It iterates `lines` — the FULL, untruncated list — and never
			// `painted`, `budget`, `oversized` or the placed runs, so the
			// count is identical whether this element paints every line, a
			// truncated prefix, or (the first-line-too-tall path) none at
			// all. That independence is the whole point: a canvas that
			// stopped drawing at line 1920 must not also stop counting
			// pages there.
			//
			// The extent is the render path's, term for term rather than
			// re-derived: `top` here is exactly the `lineY` render.go
			// places a line at, and the bottom adds the same
			// FirstBaseline + LastDescent from the same vertical model.
			if band.name == bandContent {
				for i := range lines {
					top, err := canvasLineTop(originY, i, vm.Advance)
					if err != nil {
						return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
					}
					column.Items = append(column.Items, layout.ColumnItem{
						ElementID: string(element.ID),
						Top:       top,
						Bottom:    top + vm.FirstBaseline + vm.LastDescent,
						Runs:      []layout.TextRunRef{0},
						// Story 7.9 (FR51): the SAME group the render
						// path's own line items carry
						// (contentColumnItems / paginateDocument). A
						// canvas line has no prior group — only a
						// table's row items do, and a table's cells are
						// not text elements — so the group is taken
						// directly rather than through orKeepTogether,
						// exactly as the render path's image arm does.
						// An untagged element gets the ZERO ItemGroup,
						// which is what it carried before this story and
						// is what keeps an ungrouped document identical.
						Group: keepTogether.keepTogetherGroup(string(element.ID)),
					})
				}
			}
			budget := canvasFragmentBudget{}
			for i, line := range painted {
				top, err := canvasLineTop(originY, i, vm.Advance)
				if err != nil {
					return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
				}
				lineX := element.X + textAlignOffset(align, boxWidth, line.width)
				// THE IDENTICAL BRANCH render.go's line loop carries,
				// from the identical shared rule (Story 7.3): the
				// canvas shows the word positions the PDF prints, and
				// it gets them by consuming engine-computed offsets —
				// never by asking the browser to justify, which
				// canvas-authority-contract.test.ts bans across every
				// production, unit and e2e source.
				var placed []textRunSource
				if pieces := justifiedLinePieces(align, line, i, len(lines), segs, ops, fontSize, boxWidth); pieces != nil {
					for _, piece := range pieces {
						pieceRuns, pieceErr := positionSegments(segs, piece.from, piece.to, element.X+piece.offset, top, fontSize, vm.FirstBaseline, nil)
						if pieceErr != nil {
							return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, pieceErr)
						}
						placed = append(placed, pieceRuns...)
					}
				} else {
					var perr error
					placed, perr = positionSegments(segs, line.from, line.to, lineX, top, fontSize, vm.FirstBaseline, nil)
					if perr != nil {
						return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, perr)
					}
				}
				baseline, err := canvasDerivedSum(top, vm.FirstBaseline)
				if err != nil {
					return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
				}
				advance, err := canvasDerived("line advance", vm.Advance)
				if err != nil {
					return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
				}
				width, err := canvasDerived("line width", line.width)
				if err != nil {
					return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
				}
				paintLine := designer.CanvasTextLine{Top: int64(top), Baseline: int64(baseline), Advance: int64(advance), Width: int64(width), Fragments: []designer.CanvasTextFragment{}}
				// A fragment's text is BODY TEXT, not an identifier: this
				// site is the second of the two maxCanvasPropertyString
				// conflations DW-25 undercounted, and a value that got past
				// the value cap used to abort here instead.
				oversized := false
				for _, fragment := range placed {
					if len(fragment.text) > maxCanvasBodyText {
						oversized = true
						break
					}
					x, err := canvasDerived("fragment x", fragment.x)
					if err != nil {
						return fmt.Errorf("folio8: canvas text element %s: %w", element.ID, err)
					}
					// The attribution the projection used to discard.
					// cache is the CHAIN-SCOPED cache this element draws
					// through, so the answer is the one the element's own
					// chain resolved; carried is "" for every shipped face,
					// which omitempty turns into the precise, self-describing
					// absence the browser reads as "this fragment is a
					// shipped face".
					//
					// The bool is discardable because the MISS IS ALREADY
					// EMPTY: embeddedFaceAssetKey returns "" rather than the
					// name it was handed (TestANonMintedFaceNameYieldsNoAssetKey).
					// That matters here specifically — this value goes on the
					// wire, and the browser admits a fragment's assetKey only
					// as 64 lowercase hex characters, failing the whole
					// projection otherwise.
					carried, _ := cache.carriedAssetKey(fragment.face)
					// AND THE OTHER HALF OF THE SAME ANSWER (Story
					// 8.4e). A face this chain resolved that the
					// document does NOT carry is a shipped one, and
					// fragment.face is then the caller's FontSet key
					// verbatim — chainFaceNames mints a name only for
					// an embedded entry and copies entry.Face for every
					// other, and fontCache.get can resolve nothing that
					// is not in one of those two namespaces. So the
					// engine's identity for this face travels beside the
					// engine's identity for a carried one, and exactly
					// one of the two is ever set.
					//
					// THE BOUND IS THE ONE projectFontChainEntry ALREADY
					// APPLIES to a chain entry's face, and it is applied
					// the same way: refused with a stated reason, never
					// silently emptied. Emptying would put a fragment on
					// the wire carrying NEITHER identity, which the
					// browser reads as "shipped, unattributed" — a
					// quieter lie than a refusal.
					//
					// ⚠ IT WAS UNREACHABLE UNTIL STORY 11.2, AND IT IS
					// NOT ANY MORE. This paragraph used to read "AND IT
					// IS UNREACHABLE TODAY", on this premise:
					// projectFontChainEntry refuses first — Canvas builds
					// the font chains BEFORE CanvasWithTextPaint calls
					// addCanvasTextPaint, and it refuses any chain entry
					// whose Face exceeds this same bound — and "for a
					// shipped face fragment.face IS that entry's Face".
					//
					// THAT PREMISE NO LONGER HOLDS. fragment.face can now
					// be a STYLE VARIANT's name (chainFaceNames' styled
					// list, Story 11.2), and projectFontChainEntry bounds
					// an entry's `face` and `asset` and NOT its variant
					// siblings — projecting a variant into
					// CanvasFontChainEntry is Story 11.3's call, fenced
					// out of 11.2 by its Ask First. So an over-long
					// VARIANT name reaches this line without having
					// failed the projection, and this check is the thing
					// that catches it: a live guard, no longer defence in
					// depth. The bound is applied the way
					// projectFontChainEntry applies it — refused with a
					// stated reason, never silently emptied.
					//
					// The old paragraph's measurement (deleting the check
					// reddened no test) was true when it was taken and is
					// left recorded here rather than deleted, because it
					// is what a later reader would otherwise re-derive
					// and mis-trust. It is stale, not wrong-then.
					shipped := ""
					if carried == "" {
						if len(fragment.face) > maxCanvasPropertyString {
							return fmt.Errorf("folio8: canvas text element %s: font face name exceeds the projection bound", element.ID)
						}
						shipped = fragment.face
					}
					paintLine.Fragments = append(paintLine.Fragments, designer.CanvasTextFragment{Text: fragment.text, X: int64(x), AssetKey: carried, Face: shipped})
				}
				// Painting stops at the last WHOLE line that fits. A half
				// line would be a worse lie than a short one: the author
				// would read a sentence the document does not contain.
				if oversized || !budget.admits(len(paintLine.Fragments)) {
					paint.Truncated = true
					break
				}
				budget.take(len(paintLine.Fragments))
				paint.Lines = append(paint.Lines, paintLine)
			}
			component.TextPaint = paint
		}
	}
	// SPEC-table-rules §4: the table header labels, through the SAME
	// document font cache as the text above — the canvas and the PDF path
	// must agree on which faces exist (AD-17).
	addCanvasTableLabelLines(t, projection, fs, cache)
	return nil
}

// canvasFragmentBudget is the two fragment bounds a painted line must satisfy,
// held in one place because they bound DIFFERENT QUANTITIES and are easy to
// mistake for one another.
//
//   - maxCanvasTextFragments is PER LINE. It is Go's own long-standing guard
//     on one degenerate line and has no counterpart in the browser.
//   - maxCanvasBodyTextFragments is CUMULATIVE across the whole element. It
//     exists to mirror engine-protocol.ts's `fragments` counter, which is
//     declared once per component and never reset — so a projection whose
//     every line is per-line legal can still be refused there, and a refusal
//     there drops the ENTIRE engine response with no attributable error.
//
// The asymmetry is why the engine has to carry the cumulative count itself:
// satisfying the per-line guard says nothing about the browser's bound.
type canvasFragmentBudget struct{ used int }

// admits reports whether a line carrying count fragments can still be
// painted. Both bounds must hold; neither implies the other.
func (b *canvasFragmentBudget) admits(count int) bool {
	return count <= maxCanvasTextFragments && b.used+count <= maxCanvasBodyTextFragments
}

func (b *canvasFragmentBudget) take(count int) { b.used += count }

func canvasDerived(name string, value geom.Length) (geom.Length, error) {
	if value < 0 || value > geom.Length(designer.MaxCanvasMillipoints) {
		return 0, fmt.Errorf("%s exceeds the JavaScript-safe projection bound", name)
	}
	return value, nil
}

func canvasDerivedSum(left, right geom.Length) (geom.Length, error) {
	if left < 0 || right < 0 || left > geom.Length(designer.MaxCanvasMillipoints)-right {
		return 0, fmt.Errorf("derived canvas coordinate exceeds the JavaScript-safe projection bound")
	}
	return left + right, nil
}

func canvasLineTop(elementY geom.Length, index int, advance geom.Length) (geom.Length, error) {
	if index < 0 || advance < 0 || elementY < 0 || advance > 0 && geom.Length(index) > (geom.Length(designer.MaxCanvasMillipoints)-elementY)/advance {
		return 0, fmt.Errorf("derived canvas line origin exceeds the JavaScript-safe projection bound")
	}
	return canvasDerivedSum(elementY, geom.Length(index)*advance)
}

func canvasComponents(t *Template, bands []designer.CanvasBand) ([]designer.CanvasComponent, error) {
	out := make([]designer.CanvasComponent, 0)
	for _, projected := range bands {
		var elements []template.Element
		switch projected.Name {
		case bandPageHeader:
			elements = t.doc.Bands.PageHeader.Elements
		case bandContent:
			elements = contentElements(t)
		case bandPageFooter:
			elements = t.doc.Bands.PageFooter.Elements
		}
		for _, element := range elements {
			width, height := projectedSize(element)
			for _, value := range []geom.Length{element.X, element.Y, width, height} {
				if value > geom.Length(designer.MaxCanvasMillipoints) {
					return nil, fmt.Errorf("folio8: component exceeds the JavaScript-safe geometry bound")
				}
			}
			component := designer.CanvasComponent{ID: string(element.ID), Type: string(element.Type), Band: projected.Name, X: int64(element.X), Y: int64(element.Y), Width: int64(width), Height: int64(height), Resizable: element.Type != template.ElementTable}
			if element.Type == template.ElementText && element.Value.Set && !element.Value.Null {
				// BODY TEXT, so the body-text backstop — not the identifier
				// bound this used to share. At 512 bytes this returned nil
				// for the ENTIRE component list, and because the property
				// command re-projects inside its own transaction
				// (component_commands.go's updateComponentProperties) it did
				// not merely blank the canvas: it REJECTED the author's edit
				// at about eighty English words, or a hundred and seventy
				// Thai characters. The refusal stays — a value may not be
				// truncated, only its paint (D-7.4.2 §1) — but it is now a
				// megabyte-scale channel backstop Epic 7's input cannot
				// reach, not a clause-length cap.
				if len(element.Value.Value) > maxCanvasBodyText {
					return nil, fmt.Errorf("folio8: component value exceeds the projection bound")
				}
				component.Value = stringPointer(element.Value.Value)
				if binding := directCanvasBinding(element.Value.Value); binding != "" {
					component.Binding = stringPointer(binding)
				}
			}
			if isCodeElement(element.Type) && element.Value.Set && !element.Value.Null {
				// The designer's single-line field shows control characters
				// as escapes; the command layer decodes them back.
				escaped := encodeBarcodeEscapes(element.Value.Value)
				if len(escaped) > maxCanvasBodyText {
					return nil, fmt.Errorf("folio8: component value exceeds the projection bound")
				}
				component.Value = stringPointer(escaped)
				if binding := directCanvasBinding(element.Value.Value); binding != "" {
					component.Binding = stringPointer(binding)
				}
			}
			if element.VisibleIf.Set && !element.VisibleIf.Null {
				if len(element.VisibleIf.Value) > maxCanvasPropertyString {
					return nil, componentFailure(string(element.ID), "component.visibleIf", fmt.Sprintf("visibleIf exceeds the %d-byte editor/projection limit", maxCanvasPropertyString))
				}
				component.VisibleIf = stringPointer(element.VisibleIf.Value)
			}
			if element.Type == template.ElementTable && element.Table.Set && !element.Table.Null {
				if len(element.Table.Value.Bind) > maxCanvasPropertyString {
					return nil, fmt.Errorf("folio8: component table bind exceeds the projection bound")
				}
				component.TableBind = stringPointer(element.Table.Value.Bind)
				var err error
				component.Columns, err = canvasTableColumns(element)
				if err != nil {
					return nil, err
				}
			}
			if element.Style.Set && !element.Style.Null {
				if err := applyCanvasStyle(&component, element.Type, element.Style.Value); err != nil {
					return nil, err
				}
			}
			authored, err := canvasAuthoredProperties(element)
			if err != nil {
				return nil, err
			}
			component.Authored = authored
			out = append(out, component)
		}
	}
	return out, nil
}

// canvasTableColumns is Story 14.9's per-column projection: the canvas draws
// the table it will print, so it needs each column's label, its declared width
// and the alignment the engine will actually use for EACH of the two rows the
// canvas paints.
//
// THE ALIGNMENTS COME FROM THE RENDERER'S OWN FUNCTIONS, CALLED. resolveHeader-
// Style and resolveBodyStyle are pure, need no fonts and no data, and live in
// the same package (table_render.go); columnAlign is the last step both rows
// share. Nothing here re-implements a cascade — 14.8's Part 4 requirement, and
// R2's binding guardrail: a projection that mirrors the cascade drifts, and the
// failure mode is a canvas that lies about print while every test passes.
//
// THE TWO BOUNDED STRINGS CLIP, THEY DO NOT ABORT, AND THE COLUMN IS NEVER
// DROPPED. This is the one place on the canvas projection where an over-bound
// string is not a refusal, and the reason is that these two are the only ones
// a LEGAL, PRINTING document can exceed: `decodeColumn` caps neither `label`
// nor `bind`, so `worked-example.json` with a 600-byte column label parses,
// renders a real 64,123-byte PDF, and — with an abort here — blanked the
// designer's canvas until reload. Measured on both sides, at HEAD without this
// story and with it. A document the engine prints is a document the canvas
// draws; that is the matrix's own answer for a zero or negative column width,
// two rows away, and it is the same principle.
//
// ⚠ CLIP, NEVER OMIT — and the difference is not stylistic. A column dropped
// from the projection makes the canvas systematically wrong about the
// document's STRUCTURE: the chip's `5 columns` would sit above four drawn
// headers. Being systematically wrong about structure is precisely the harm
// AD-17 names, so omitting converts a display problem into a structural lie.
// Clipping keeps every column, keeps the projection bounded, and lands on a
// surface that is display-only paint by [R1] — where `.canvas-display-paint`
// already ellipsises what does not fit.
//
// ⚠ IT VALIDATES NOTHING ELSE, AND THAT IS DELIBERATE. `width: 0`, a NEGATIVE
// width, an empty label, an empty bind and `columns: []` all load, project and
// paint today; TableColumns' validating gate refuses the first three, and
// reusing it here would blank the whole designer for a document that currently
// draws. Absence, not an empty slice, for a table with no columns: `omitempty`
// drops the key, and the browser's guard admits its absence.
//
// Proportional allocation can refuse a structural edit that leaves a column
// with zero width. Preserve its located engine diagnostic at the Canvas seam.
func canvasTableColumns(element template.Element) ([]designer.CanvasTableColumn, error) {
	declared := element.Table.Value.Columns
	if len(declared) == 0 {
		return nil, nil
	}
	header := resolveHeaderStyle(element)
	body := resolveBodyStyle(element)
	widths, err := template.TableColumnWidths(element)
	if err != nil {
		return nil, wrapTableWidthError(err)
	}
	columns := make([]designer.CanvasTableColumn, 0, len(declared))
	for i, column := range declared {
		columns = append(columns, designer.CanvasTableColumn{
			ID:          string(column.ID),
			Label:       clipCanvasPropertyString(column.Label),
			LabelLines:  canvasLabelLinesAtBreaks(column.Label),
			Width:       int64(widths[i]),
			HeaderAlign: columnHeaderAlign(header.alignFallback, column),
			CellAlign:   columnAlign(body.alignFallback, column),
			Bind:        clipCanvasPropertyString(column.Bind),
		})
	}
	return columns, nil
}

// clipCanvasPropertyString cuts value to at most maxCanvasPropertyString BYTES,
// ON A RUNE BOUNDARY.
//
// ⚠ THE RUNE BOUNDARY IS LOAD-BEARING, NOT TIDINESS. A byte-cut through the
// middle of a multibyte rune emits invalid UTF-8, encoding/json would then
// replace it with U+FFFD or fail the envelope, and a display concern would
// become the fatal outcome the clipping exists to remove. Thai and CJK are
// three bytes to the character, so this is the ordinary case for this
// codebase's own corpus, not an exotic one.
//
// ⚠ AND THE CLIPPED VALUE IS ALWAYS ACCEPTABLE TO THE BROWSER GUARD, which
// bounds `column.label.length` — UTF-16 code units — against the same 512
// while this bounds BYTES. The two units disagree, but only in one direction:
// a BMP rune is 1 UTF-16 unit and 1–3 UTF-8 bytes, a supplementary rune is 2
// units and 4 bytes, so UTF-16 length is never greater than byte length. A
// value inside the byte bound is therefore inside the guard's bound too, and
// the test for that computes both rather than reasoning about it.
// maxCanvasHeaderLabelLines bounds CanvasTableColumn.LabelLines; the
// browser's guard admits no more.
// It is 256 because a label is bounded at 256 code points, so it can pack to
// at most 256 lines; the canvas never drops one.
const maxCanvasHeaderLabelLines = 256

// maxCanvasHeaderLabelLineBytes bounds one LabelLines entry. 1024 bytes holds
// the longest line a 256-code-point label can produce (256 Thai characters are
// 768 bytes), so the canvas never cuts a label line.
// engine-protocol.ts mirrors both bounds (TestCanvasLabelLineBoundsMatchTheDesignerGuard).
const maxCanvasHeaderLabelLineBytes = 1024

// canvasLabelLinesAtBreaks is the font-less projection of a label: its
// mandatory breaks only.
func canvasLabelLinesAtBreaks(label string) []string {
	out := []string{}
	if label == "" {
		return out
	}
	for _, line := range strings.Split(label, "\n") {
		if len(out) == maxCanvasHeaderLabelLines {
			break
		}
		out = append(out, clipCanvasStringTo(strings.TrimRight(line, "\r"), maxCanvasHeaderLabelLineBytes))
	}
	return out
}

// canvasLabelLines turns packed lines into their text, line feeds removed.
func canvasLabelLines(label string, lines []wrappedLine) []string {
	runes := []rune(label)
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if len(out) == maxCanvasHeaderLabelLines {
			break
		}
		out = append(out, clipCanvasStringTo(strings.TrimRight(string(runes[ln.from:ln.to]), "\r\n"), maxCanvasHeaderLabelLineBytes))
	}
	return out
}

// addCanvasTableLabelLines replaces every table column's font-less
// LabelLines with the lines the rendered header packs, through the same
// shaper and the same packer (packHeaderLabelLines). A table whose header
// font does not resolve keeps the line-feed split, as a text element whose
// chain does not resolve degrades rather than failing the canvas.
func addCanvasTableLabelLines(t *Template, projection *designer.CanvasProjection, fs FontSet, cache *fontCache) {
	components := make(map[string]*designer.CanvasComponent, len(projection.Components))
	for i := range projection.Components {
		components[projection.Components[i].ID] = &projection.Components[i]
	}
	for _, elements := range [][]template.Element{t.doc.Bands.PageHeader.Elements, contentElements(t), t.doc.Bands.PageFooter.Elements} {
		for _, el := range elements {
			if el.Type != template.ElementTable || !el.Table.Set || el.Table.Null {
				continue
			}
			component := components[string(el.ID)]
			if component == nil || len(component.Columns) != len(el.Table.Value.Columns) {
				continue
			}
			hs := resolveHeaderStyle(el)
			if !hs.hasFontFamily {
				continue
			}
			entries, err := lookupFontChain(t, hs.fontFamily)
			if err != nil {
				continue
			}
			chain, styledChain := chainFaceNames(entries, fontStyleOf(hs.bold, hs.italic))
			headerCache := cache.forChain(hs.fontFamily)
			widths, err := template.TableColumnWidths(el)
			if err != nil {
				continue
			}
			_, padRight, _, padLeft := paddingEdges(hs.padding)
			for i, col := range el.Table.Value.Columns {
				if col.Label == "" {
					continue
				}
				segs, _, serr := shapeSegments(string(col.ID), chain, styledChain, col.Label, fs, headerCache, breaksAreConsumed)
				if serr != nil {
					continue
				}
				lines := packHeaderLabelLines(segs, col.Label, hs.fontSize, widths[i]-padLeft-padRight)
				component.Columns[i].LabelLines = canvasLabelLines(col.Label, lines)
			}
		}
	}
}

func clipCanvasPropertyString(value string) string {
	return clipCanvasStringTo(value, maxCanvasPropertyString)
}

// clipCanvasStringTo cuts value to at most limit bytes, at a rune boundary.
func clipCanvasStringTo(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}

func directCanvasBinding(value string) string {
	literal, placeholders, trailing, err := expr.ScanPlaceholders(value)
	if err != nil || len(literal) != 1 || literal[0] != "" || len(placeholders) != 1 || trailing != "" || placeholders[0].Reserved {
		return ""
	}
	parsed, err := expr.Parse(placeholders[0].Inner)
	if err != nil {
		return ""
	}
	path, ok := parsed.(*expr.PathExpr)
	if !ok || path.Raw == "" || len(path.Raw) > maxCanvasBindingString {
		return ""
	}
	return path.Raw
}

func stringPointer(value string) *string     { return &value }
func boolPointer(value bool) *bool           { return &value }
func int64Pointer(value int64) *int64        { return &value }
func lengthPointer(value geom.Length) *int64 { rendered := int64(value); return &rendered }
func canvasPropertyLength(name string, value geom.Length) (*int64, error) {
	if value < 0 || value > geom.Length(designer.MaxCanvasMillipoints) {
		return nil, fmt.Errorf("folio8: component %s exceeds the projection bound", name)
	}
	return lengthPointer(value), nil
}

// canvasPropertyColor is the projection's ONE reading of "this is a
// colour the designer may paint": the existing length bound, plus the
// module's one hex parser — the SAME parseHexColor that
// buildCellRectWithBackgroundField and elementInk call on the render
// path.
//
// It exists because the projection admitted what Render refuses. The
// three colour arms below bounded the string's LENGTH and never its
// SHAPE, so `"red"`, `""`, `"rgba(1,2,3,.5)"` and `"var(--x)"` all
// projected verbatim and reached the canvas's `--text-ink`, while the
// same document's Render produced a located STYLE_COLOR_INVALID. The
// designer painted what the engine would not print.
//
// It REFUSES rather than silently dropping the field: dropping would
// trade a loud divergence for a quiet one, and the refusal keeps the
// existing `exceeds the projection bound` shape's contract — the
// projection either carries a value both sides agree on, or it fails.
//
// The command path already refuses to SET a malformed colour
// (component_commands.go), so this closes the last door into the
// projection rather than inventing a policy.
func canvasPropertyColor(name, value string) (*string, error) {
	if len(value) > maxCanvasPropertyString {
		return nil, fmt.Errorf("folio8: component %s exceeds the projection bound", name)
	}
	if _, ok := parseHexColor(value); !ok {
		return nil, fmt.Errorf("folio8: component %s %q is not a #RRGGBB colour", name, value)
	}
	return stringPointer(value), nil
}

func applyCanvasStyle(component *designer.CanvasComponent, elementType template.ElementType, style template.Style) error {
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.FontFamily.Set && !style.FontFamily.Null {
		if len(style.FontFamily.Value) > maxCanvasPropertyString {
			return fmt.Errorf("folio8: component fontFamily exceeds the projection bound")
		}
		component.FontFamily = stringPointer(style.FontFamily.Value)
	}
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.FontSize.Set && !style.FontSize.Null {
		value, err := canvasPropertyLength("fontSize", style.FontSize.Value)
		if err != nil {
			return err
		}
		component.FontSize = value
	}
	// style.lineSpacing, projected for the first time (Story 7.4). Its range
	// is already settled at load and on the property-command path by the one
	// validator both call (template.DecodeLineSpacing, D-7.2.3), so this is a
	// projection of a committed value and not a second opinion on it.
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.LineSpacing.Set && !style.LineSpacing.Null {
		component.LineSpacing = int64Pointer(style.LineSpacing.Value)
	}
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.Bold.Set && !style.Bold.Null {
		component.Bold = boolPointer(style.Bold.Value)
	}
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.Italic.Set && !style.Italic.Null {
		component.Italic = boolPointer(style.Italic.Value)
	}
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.Align.Set && !style.Align.Null {
		component.Align = stringPointer(style.Align.Value)
	}
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.Valign.Set && !style.Valign.Null {
		component.Valign = stringPointer(style.Valign.Value)
	}
	if (elementType == template.ElementText || elementType == template.ElementTable) && style.Color.Set && !style.Color.Null {
		value, err := canvasPropertyColor("color", style.Color.Value)
		if err != nil {
			return err
		}
		component.Color = value
	}
	if style.Background.Set && !style.Background.Null {
		value, err := canvasPropertyColor("background", style.Background.Value)
		if err != nil {
			return err
		}
		component.Background = value
	}
	// A BORDER THAT PAINTS NO INK PROJECTS NO BORDER FIELDS AT ALL —
	// element_box.go's borderPaints, its second call site, so the placer
	// and the projection cannot disagree about what a border is.
	//
	// `style.border: {"edges": []}` used to project BorderWidth and
	// BorderColor while BorderEdges — an EMPTY slice under
	// `json:",omitempty"` — was dropped from the wire entirely, so
	// App.tsx's `component.borderEdges ?? boxEdges` fell back to all four
	// and the canvas painted a full border for a document the PDF prints
	// none of. Emitting no border field at all makes App.tsx's `bordered`
	// evaluate false, which is the same answer the printed page gives.
	//
	// ⚠ THE OBVIOUS FIX — dropping `omitempty` — IS WRONG IN BOTH
	// DIRECTIONS and must not be applied: a nil slice would then marshal
	// to JSON `null` on EVERY component, `null ?? boxEdges` re-fires the
	// full border for every one of them, and the protocol note says the
	// validator may reject the null outright.
	if borderPaints(style.Border) {
		border := style.Border.Value
		if border.Width.Set && !border.Width.Null {
			value, err := canvasPropertyLength("borderWidth", border.Width.Value)
			if err != nil {
				return err
			}
			component.BorderWidth = value
		}
		if border.Color.Set && !border.Color.Null {
			value, err := canvasPropertyColor("borderColor", border.Color.Value)
			if err != nil {
				return err
			}
			component.BorderColor = value
		}
		if border.Edges.Set && !border.Edges.Null {
			component.BorderEdges = append([]string(nil), border.Edges.Value...)
		}
	}
	if style.Padding.Set && !style.Padding.Null {
		padding := style.Padding.Value
		if padding.Top.Set && !padding.Top.Null {
			value, err := canvasPropertyLength("paddingTop", padding.Top.Value)
			if err != nil {
				return err
			}
			component.PaddingTop = value
		}
		if padding.Right.Set && !padding.Right.Null {
			value, err := canvasPropertyLength("paddingRight", padding.Right.Value)
			if err != nil {
				return err
			}
			component.PaddingRight = value
		}
		if padding.Bottom.Set && !padding.Bottom.Null {
			value, err := canvasPropertyLength("paddingBottom", padding.Bottom.Value)
			if err != nil {
				return err
			}
			component.PaddingBottom = value
		}
		if padding.Left.Set && !padding.Left.Null {
			value, err := canvasPropertyLength("paddingLeft", padding.Left.Value)
			if err != nil {
				return err
			}
			component.PaddingLeft = value
		}
	}
	return nil
}

func canvasDimensions(t *Template) (geom.Length, geom.Length, error) {
	var width, height geom.Length
	switch {
	case t.doc.Page.SizeIsName && t.doc.Page.SizeName == "A4":
		width, height = 595276, 841890
	case t.doc.Page.SizeIsName && t.doc.Page.SizeName == "Letter":
		width, height = 612000, 792000
	case !t.doc.Page.SizeIsName:
		width, height = t.doc.Page.SizeCustom.Width, t.doc.Page.SizeCustom.Height
	default:
		return 0, 0, fmt.Errorf("folio8: unsupported page size")
	}
	if t.doc.Page.Orientation == "landscape" {
		width, height = height, width
	}
	return width, height, nil
}

// applyPageSetupCommand decodes the one versioned, Go-defined opaque command.
// Numeric input stays a JSON literal until exact millipoint conversion; it is
// never decoded through float64.
func applyPageSetupCommand(t *Template, command []byte) (designer.CanvasProjection, error) {
	if t == nil {
		return designer.CanvasProjection{}, errNilTemplate
	}
	// The SAME scan the component door runs, and it has to be: the property
	// this guards is a property of the command CHANNEL, and a guard over one of
	// two exported doors passes while the property is false. Both of this
	// door's own gates are duplicate-blind for the same reason the component
	// door's are — len(raw) != 7 counts a map that has already deduped, the
	// nested len(margins) != 4 counts another one, and the version gate reads
	// the last "version" key.
	if err := refuseDuplicateCommandKeys(command, pageSetupCommandPath); err != nil {
		return designer.CanvasProjection{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(command))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil || dec.More() {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page setup command is malformed")
	}
	if len(raw) != 7 || !equalString(raw["kind"], "pageSetup") || !equalNumber(raw["version"], "1") {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: unknown page setup command")
	}
	preset, orientation := stringField(raw, "preset"), stringField(raw, "orientation")
	if orientation != "portrait" && orientation != "landscape" {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page.orientation must be portrait or landscape")
	}
	if preset != "A4" && preset != "Letter" && preset != "custom" {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page.size must be A4, Letter, or custom")
	}
	var width, height geom.Length
	if preset == "custom" {
		var err error
		width, err = lengthField(raw, "width")
		if err != nil {
			return designer.CanvasProjection{}, fmt.Errorf("folio8: page.width: %w", err)
		}
		height, err = lengthField(raw, "height")
		if err != nil {
			return designer.CanvasProjection{}, fmt.Errorf("folio8: page.height: %w", err)
		}
	}
	marginRaw, ok := raw["margin"]
	if !ok {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page.margin is required")
	}
	var margins map[string]json.RawMessage
	if json.Unmarshal(marginRaw, &margins) != nil || len(margins) != 4 {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page.margin must contain top, right, bottom, left")
	}
	readMargin := func(name string) (geom.Length, error) {
		v, err := lengthField(margins, name)
		if err != nil {
			return 0, fmt.Errorf("folio8: page.margin.%s: %w", name, err)
		}
		if v < 0 {
			return 0, fmt.Errorf("folio8: page.margin.%s must not be negative", name)
		}
		return v, nil
	}
	top, err := readMargin("top")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	right, err := readMargin("right")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	bottom, err := readMargin("bottom")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	left, err := readMargin("left")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if preset == "A4" {
		width, height = 595276, 841890
	} else if preset == "Letter" {
		width, height = 612000, 792000
	}
	if width <= 0 || height <= 0 {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: page.size width and height must be positive")
	}
	if preset == "custom" && (width <= 0 || height <= 0) {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: custom page size is required")
	}
	before, err := SerializeTemplate(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	page := &t.doc.Page
	page.Orientation = orientation
	page.Margin = template.Margin{Top: top, Right: right, Bottom: bottom, Left: left}
	if preset == "custom" {
		page.SizeIsName = false
		page.SizeName = ""
		page.SizeCustom = template.PageSize{Width: width, Height: height}
	} else {
		page.SizeIsName = true
		page.SizeName = preset
		page.SizeCustom = template.PageSize{}
	}
	// SPEC-table-rules review item 1: a page size, margin or orientation
	// that shrinks the content window must not strand a table's minHeight.
	if err := refuseStrandedFloor(t, "table.minHeight"); err != nil {
		restorePage(t, before)
		return designer.CanvasProjection{}, err
	}
	// spec-section-break: nor the section break.
	if err := refuseSectionBreakBeyondContent(t); err != nil {
		restorePage(t, before)
		return designer.CanvasProjection{}, err
	}
	projection, err := canvas(t)
	if err != nil {
		restorePage(t, before)
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

func restorePage(t *Template, canonical []byte) {
	restored, err := ParseTemplate(canonical)
	if err == nil {
		t.doc = restored.doc
		t.derivedFooters = restored.derivedFooters
	}
}
func equalString(raw json.RawMessage, want string) bool {
	var got string
	return json.Unmarshal(raw, &got) == nil && got == want
}
func equalNumber(raw json.RawMessage, want string) bool { return string(raw) == want }
func stringField(raw map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(raw[key], &value)
	return value
}
func lengthField(raw map[string]json.RawMessage, key string) (geom.Length, error) {
	v, ok := raw[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	literal := string(v)
	if strings.ContainsAny(literal, "eE") {
		return 0, fmt.Errorf("%s must be a decimal with at most three places", key)
	}
	if dot := strings.IndexByte(literal, '.'); dot >= 0 && len(literal)-dot-1 > 3 {
		return 0, fmt.Errorf("%s must have at most three decimal places", key)
	}
	var n json.Number
	if json.Unmarshal(v, &n) != nil {
		return 0, fmt.Errorf("%s must be a finite number", key)
	}
	value, err := parseMillipoints(literal, key)
	if err != nil {
		return 0, err
	}
	if value > geom.Length(designer.MaxCanvasMillipoints) || value < -geom.Length(designer.MaxCanvasMillipoints) {
		return 0, fmt.Errorf("%s exceeds the JavaScript-safe geometry bound", key)
	}
	return value, nil
}
func parseMillipoints(literal, key string) (geom.Length, error) {
	negative := strings.HasPrefix(literal, "-")
	if negative {
		literal = literal[1:]
	}
	parts := strings.Split(literal, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	whole := int64(0)
	for _, c := range parts[0] {
		// STORY 15.2a SPLIT THIS BRANCH, and the split is two branches rather
		// than a changed string because the condition mixed two causes: a
		// character that is not a digit, and a whole part that is genuinely
		// about to overflow. Both reported OVERFLOW, so the `null` a designer
		// draft emits for an unparseable number came back as "width overflows
		// millipoints" — refused, so nothing was written, but with a cause the
		// author could not act on. The overflow detector below is untouched.
		//
		// The wording is the fraction loop's own, seventeen lines down, which
		// has always called a non-digit exactly this. The two loops disagreeing
		// about the identical condition WAS the defect; importing a third
		// vocabulary would have left them disagreeing.
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%s must be a number", key)
		}
		if whole > (1<<63-1)/10 {
			return 0, fmt.Errorf("%s overflows millipoints", key)
		}
		whole = whole*10 + int64(c-'0')
	}
	frac := int64(0)
	if len(parts) == 2 {
		for _, c := range parts[1] {
			if c < '0' || c > '9' {
				return 0, fmt.Errorf("%s must be a number", key)
			}
			frac = frac*10 + int64(c-'0')
		}
		for len(parts[1]) < 3 {
			parts[1] += "0"
			frac *= 10
		}
	}
	if whole > (1<<63-1)/1000 || (whole == (1<<63-1)/1000 && frac > (1<<63-1)%1000) {
		return 0, fmt.Errorf("%s overflows millipoints", key)
	}
	value := whole*1000 + frac
	if negative {
		value = -value
	}
	return geom.Length(value), nil
}
