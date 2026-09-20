package folio8

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/pdf"
	"github.com/panitw/folio8/folio-go/internal/template"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// A4 page dimensions in millipoints, matching internal/pdf's Story 1.1
// fixture constants (595.276pt x 841.89pt) — package folio8 does not
// import those unexported constants; it restates the one named size
// this story resolves. A custom page size is read directly from the
// template instead.
const (
	pageWidthA4  geom.Length = 595276
	pageHeightA4 geom.Length = 841890
)

// defaultFontSizePt is the provisional default text size (12pt) used
// when a text element's style does not specify one (AC28: provisional,
// pending real style inheritance/defaults, which are not this story's
// concern).
const defaultFontSizePt geom.Length = 12000

// defaultLineSpacing is the neutral leading ratio: an element whose
// style declares no `lineSpacing` is measured with exactly
// template.LineSpacingUnit thousandths, so ScaleRound's quotient is the
// ruled advance unchanged and every document written before Story 7.2
// renders to the same bytes it did.
const defaultLineSpacing int64 = template.LineSpacingUnit

// styleLineSpacing extracts an element's leading ratio in thousandths,
// beside — and in the same shape as — the fontSize extraction every
// construction site already performs. Absent, null, or no style block at
// all all mean the neutral ratio.
//
// It is a function rather than four inlined copies because Story 7.2
// reaches FOUR construction sites (text element, table header labels,
// table body-and-footer, canvas projection) and D-7.1.3's "every caller,
// no carve-out" is a property that four hand-written copies cannot keep.
func styleLineSpacing(st template.Presence[template.Style]) int64 {
	if st.Set && !st.Null && st.Value.LineSpacing.Set && !st.Value.LineSpacing.Null {
		return st.Value.LineSpacing.Value
	}
	return defaultLineSpacing
}

// pageDimensions resolves a Document's page geometry (band composition
// needs page height and margins; see pageGeometryOf, which is the only
// caller that reaches internal/layout with them). An unrecognised named size is a
// located error, not a silent A4 substitution (Finding 17, QA review:
// the previous version fell back to A4 for ANY named size other than
// "A4" — a document declaring "size": "Letter" rendered as A4 with no
// error, silent wrong output on a valid-looking input, in a module
// whose entire premise is predictable bytes). Note that
// internal/template's closedPageSizeNames (Story 1.4) already accepts
// "Letter" as a load-time-valid value — this story only ever built A4
// dimensions (this story's scope is the font-embedding mechanism, not a
// named-page-size table), so "Letter" is a legally loadable document
// this version of Render cannot yet produce bytes for; failing loudly
// here is more honest than a silent A4 substitution, and adding real
// Letter dimensions is out of this story's scope.
func pageDimensions(doc *Template) (width, height geom.Length, err error) {
	switch {
	case doc.doc.Page.SizeIsName && doc.doc.Page.SizeName == "A4":
		width, height = pageWidthA4, pageHeightA4
	case !doc.doc.Page.SizeIsName:
		width, height = doc.doc.Page.SizeCustom.Width, doc.doc.Page.SizeCustom.Height
	default:
		return 0, 0, fmt.Errorf(
			"folio8: Render: page.size names %q, which this version does not implement dimensions "+
				"for (only \"A4\" or a custom width/height)", doc.doc.Page.SizeName,
		)
	}
	if doc.doc.Page.Orientation == "landscape" {
		width, height = height, width
	}
	return width, height, nil
}

// textRunSource is one text element found while walking the document's
// bands, together with the resolved face name it needs and the SHAPED
// answer for its text.
//
// glyphs/clusterTexts are populated by positionSegments, in the same
// pass that computes x, and that co-location is the point rather than a
// convenience: x is derived from the very glyphs carried alongside it
// (positionSegments' cursor sums faceSegment.advance1000 over the same
// slice it hands to the run), so a run cannot be drawn from one shaping
// answer and positioned from another. Story 2.3's finisher, Blocker 1 — the previous arrangement
// shaped in renderDocument and summed raw `hmtx` advances here, which
// drew kerned text at unkerned origins.
type textRunSource struct {
	face     string
	text     string
	x, y     geom.Length
	fontSize geom.Length

	// baselineOffset is top-of-line -> baseline for this run: the ruled
	// model's first span, max(hhea ascent) over the DECLARED chain,
	// scaled to fontSize (D-2.4.2 as amended). It is a LAYOUT quantity
	// and it is resolved here, in package folio8, because AD-5 keeps
	// placement decisions out of every renderer: internal/pdf must not
	// re-derive it from faces[run.Face].
	//
	// It is identical for every run of one element — every face segment
	// and every line — because it is a function of the chain and the
	// size and NOT of what was drawn. Deriving it per-run from the
	// resolved face would make it content-dependent, which AD-24 rules
	// out for exactly the reason it rules it out for the advance:
	// adding one CJK character would reflow the element.
	baselineOffset geom.Length

	// glyphs is the shaper's answer for text, in FONT UNITS and drawing
	// order. clusterTexts is its per-glyph /ToUnicode source text
	// (text.ClusterTexts), parallel to glyphs.
	glyphs       []text.ShapedGlyph
	clusterTexts []string

	// --- Story 2.6: which band, and which atomic column item ---
	//
	// band is the index into documentBands' authored order: 0 pageHeader,
	// 1 content, 2 pageFooter. Only the CONTENT band paginates; the other
	// two are repeated verbatim on every page (AC3), which is what makes
	// page 34 as complete as page 1.
	band int

	// elementID names the element this run came from, for the located
	// overflow diagnostic ONLY (layout.OverflowError). No geometry is
	// derived from it.
	elementID string

	// lineIndex is the run's line within its element. Together with
	// elementID it identifies the atomic COLUMN ITEM this run belongs to:
	// one line's runs are contiguous in the slice and share both values,
	// so grouping is a scan for a change of key rather than a map.
	lineIndex int

	// itemTop / itemBottom are the LINE's vertical extent, page-absolute:
	// `baseline − max(ascent)` .. `baseline + max(descent)` over the
	// element's DECLARED chain (D-2.4.2 as amended). Carried rather than
	// re-derived downstream — a second derivation of this number is
	// precisely what that amendment exists to prevent.
	itemTop, itemBottom geom.Length

	// --- Story 2.7: AD-4's late-bound page-number slot ---
	//
	// pageSlots mirrors pagemodel.PageNumberSlot's GlyphLo/GlyphHi/
	// DigitsY, in the FONT-UNIT glyph slice this run carries before
	// buildShapedPDFRuns converts it (renderDocument attaches the
	// pagemodel.PageNumberSlot values themselves, once CIDs and
	// 1000-em advances exist). Populated only by positionSegments, when
	// it is handed a non-empty slots argument — every other call site
	// leaves this nil, so this is additive and changes no existing run.
	//
	// A SLICE (this story's review, Blocker 1): a run may carry more
	// than one {{page}} occurrence — see pagemodel.TextRun.PageSlots'
	// doc comment for why a scalar field here was a silent mis-render.
	pageSlots []textRunPageSlot

	// --- Story 2.8: FR44's clip, D-2.8.1 ---
	//
	// clipToBox marks that this run belongs to a text element whose
	// widest packed line exceeds its declared WIDTH (detectWidthOverflow,
	// below) — never its declared height, which D-2.8.1 rules is not a
	// clip bound at all. clipX/clipWidth are that element's declared box
	// left edge and width, PAGE-ABSOLUTE exactly like x above.
	// internal/pdf uses them to wrap this run's drawing operators in a
	// PDF clip path restricted to the box's HORIZONTAL extent only — the
	// vertical clip bound it uses is the full page, never anything
	// derived from an element's declared height (AC3).
	clipToBox bool

	// hasColor/color — Story 10.1's ink, resolved from the element's
	// style.color once and stamped on every run the element produces,
	// for the same reason clipToBox is: colour is a property of the
	// ELEMENT, never of one line or one face segment within it.
	hasColor         bool
	color            pagemodel.Color
	clipX, clipWidth geom.Length

	// --- Story 4.2: DECISION-2's row identity, for Story 4.3 ---
	//
	// isTableRowLine/rowIndex mirror tableRectSource's own fields (see
	// its doc comment, table_render.go) — one bound-collection row's
	// identity, carried on every physical LINE this row's cells
	// produce, so 4.3 can group a wrapped row's several line items
	// WITHOUT reconstructing membership from elementID/extent/order.
	// false/unset for every run this story does not itself mint (every
	// header label, every ordinary text element) — unchanged from
	// before this story.
	isTableRowLine bool
	rowIndex       int

	// isHeaderLabel — Story 4.3: mirrors tableRectSource.isHeaderRow,
	// carried on a table's column-label runs so the header's chrome and
	// its labels form ONE group (AC5), the same mechanism a data row's
	// chrome and lines use. Never true alongside isTableRowLine.
	isHeaderLabel bool

	// isFooterLine — Story 4.5: mirrors tableRectSource.isFooterRow,
	// carried on a table's footer VALUE runs so the footer's chrome and
	// its value text form one group (AC1/AC5), the same mechanism a
	// data row's chrome and lines use, or the header's chrome and
	// labels. Never true alongside isTableRowLine or isHeaderLabel. A
	// DISTINCT row-type tag from isTableRowLine on purpose: a future
	// story keying alternating-row shading off isTableRowLine/rowIndex
	// (Story 4.8, epics.md: "the alternation follows row index in the
	// collection") never sees the footer as a row, regardless of
	// whichever layout.ItemGroup.Key the footer is carrying for
	// pagination purposes at any given moment (see chromeRowGroup's own
	// note on Index -1).
	isFooterLine bool
}

// lineRowGroup derives this run's layout.ItemGroup — Story 4.3's grouping
// identity — by DIRECT FIELD LOOKUP from isTableRowLine/isHeaderLabel/
// rowIndex, never by reconstruction (D-4.2.2, R3). Mirrors
// tableRectSource.chromeRowGroup exactly, so a table's rect and line items
// compute the SAME Key for the same row from each type's own fields.
func (r textRunSource) lineRowGroup() layout.ItemGroup {
	switch {
	case r.isHeaderLabel:
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: r.elementID, IsHeader: true}}
	case r.isFooterLine:
		// Story 4.5: Index -1 is a sentinel no real data row ever carries
		// (rowIndex ranges 0..N-1) — it names the footer's OWN group,
		// distinct from the header's (IsHeader) and from every data
		// row's (Index>=0). paginateWithFooterOrphanFix (table_footer.go)
		// may temporarily redirect this Key to a preceding row's own Key
		// for ONE layout.Paginate call, when (and only when) the orphan
		// rule requires it — that redirection is a pagination-time
		// grouping decision only, made outside this package's row-type
		// tags (isFooterLine itself never changes), so it cannot leak
		// into anything that keys off row identity instead of group
		// membership (see isFooterLine's own doc comment).
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: r.elementID, Index: footerGroupIndex}}
	case r.isTableRowLine:
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: r.elementID, Index: r.rowIndex}}
	default:
		return layout.ItemGroup{}
	}
}

// textRunPageSlot is one {{page}} reservation's glyph range within the
// carrying textRunSource — positionSegments' own coordinates (local to
// this run, before buildShapedPDFRuns reindexes into the document's
// final CID space). Mirrors pagemodel.PageNumberSlot's GlyphLo/GlyphHi/
// DigitsY exactly; the CID table and per-digit advance are attached
// later, in renderDocument, once pass one has allocated them.
type textRunPageSlot struct {
	glyphLo, glyphHi int
	digitsY          int
}

// bandWithOrigin pairs one of the document's three bands with the
// PAGE-ABSOLUTE vertical offset at which its own element-relative Y=0
// sits — the placement both collectTextRuns and collectImageRuns need,
// factored once so the two element kinds agree on where a band starts.
//
// The origin itself is NOT computed here. Under AD-24 "bands are placed
// on the page by internal/layout alone", so this file resolves the page
// SETUP and hands it to internal/layout, which answers with the three
// origins. Package folio8 computes no band origin
// (TestNoBandOriginArithmeticInPackageFolio8, internal/bandcomposition_arch_test.go).
type bandWithOrigin struct {
	band   template.Band
	origin geom.Length
}

// pageGeometryOf reads the document's page setup into internal/layout's
// closed input struct. Every geometric input band composition is allowed
// to see passes through here, and nothing else does: an element, or a
// measurement of one, has no route into PageGeometry, which is AD-24's
// "nothing negotiates" holding structurally rather than by convention.
//
// A band's height key that is absent or explicitly null reads as zero —
// the same treatment the pre-2.5 code gave it, unchanged.
func pageGeometryOf(doc *Template) (layout.PageGeometry, error) {
	width, height, err := pageDimensions(doc)
	if err != nil {
		return layout.PageGeometry{}, err
	}
	g := layout.PageGeometry{
		Width:        width,
		Height:       height,
		MarginTop:    doc.doc.Page.Margin.Top,
		MarginBottom: doc.doc.Page.Margin.Bottom,
		MarginLeft:   doc.doc.Page.Margin.Left,
		MarginRight:  doc.doc.Page.Margin.Right,
	}
	if doc.doc.Bands.PageHeader.Height.Set && !doc.doc.Bands.PageHeader.Height.Null {
		g.PageHeaderHeight = doc.doc.Bands.PageHeader.Height.Value
	}
	if doc.doc.Bands.PageFooter.Height.Set && !doc.doc.Bands.PageFooter.Height.Null {
		g.PageFooterHeight = doc.doc.Bands.PageFooter.Height.Value
	}
	return g, nil
}

// documentBands returns the three bands with their origins, in authored
// (header, content, footer) order, asking internal/layout for the
// origins rather than deriving them.
func documentBands(doc *Template) ([]bandWithOrigin, error) {
	g, err := pageGeometryOf(doc)
	if err != nil {
		return nil, err
	}
	origins := layout.Origins(g)
	// SPEC-multi-pages: every designed page's elements travel as the one
	// content band, in page order, so every collector reads them unchanged;
	// pagination splits them back by page (paginateContentPages).
	content := *firstContentBand(doc)
	if doc.doc.PageCount() > 1 {
		content = template.Band{Elements: contentElements(doc)}
	}
	return []bandWithOrigin{
		{doc.doc.Bands.PageHeader, origins.PageHeader},
		{content, origins.Content},
		{doc.doc.Bands.PageFooter, origins.PageFooter},
	}, nil
}

// resolvedRowAlias is source AC2: a repeating region's row-scope alias
// is the author's declared "as", or the literal "row" when the region
// omits it (AD-11). TableExt.As itself stays Presence-absent in the
// parsed document — parse_bands.go and model.go are unchanged by this
// story, so the round-trip fixed point roundtrip_test.go pins is
// undisturbed — the default is applied HERE, at resolution time, never
// at load.
func resolvedRowAlias(as template.Presence[string]) string {
	if as.Set {
		return as.Value
	}
	return "row"
}

// checkTableBindings is source AC5, plus D-3.1.1's ruling on the
// creator's flagged OD-1, both checked EARLY in renderDocument's
// prologue (finding 8) — after documentBands, before collectImageRuns
// — so a not-a-list binding or a colliding row alias fails before any
// font work, and its error ordering is plain document order across all
// three bands. Bands in documentBands order, elements in declaration
// order; the FIRST offending table element is the one reported
// (deterministic, never a map, D-1.3.5).
//
// The alias check is declaration-level and DATA-FREE (D-3.1.1, same
// category as D-2.6.5/D-2.7.3): a region declaring "as": "params",
// "as": "page" or "as": "pages" is a located template error naming the
// element — "params" because AD-11 forbids shadowing it, "page"/
// "pages" because AD-4 forbids that namespace forever and an alias
// spelling them would create one through the side door.
//
// The collection-bind check needs data: one trailing "[]" is stripped
// from bind if present, and the remainder is resolved as a bare dotted
// path against the DATA ROOT ONLY — never params, never a row, since
// bind is a root-relative collection path (AD-11). An absent path, an
// explicit null, or any non-array value are all errors naming the
// bind as authored and the element id; an empty array is not an error
// (Story 4.2 owns what an empty collection renders as).
func checkTableBindings(bands []bandWithOrigin, data bind.Value) error {
	for _, b := range bands {
		for _, el := range b.band.Elements {
			if el.Type != template.ElementTable || !el.Table.Set {
				continue
			}
			tbl := el.Table.Value

			alias := resolvedRowAlias(tbl.As)
			if alias == "params" || alias == "page" || alias == "pages" {
				return fmt.Errorf(
					"folio8: Render: element %s: table's row alias %q collides with a reserved name — "+
						"\"params\" can be shadowed by nothing (AD-11) and \"page\"/\"pages\" never acquire "+
						"a namespace (AD-4)",
					el.ID, alias,
				)
			}

			segments := tableCollectionSegments(tbl.Bind)
			val, presence := data.Lookup(segments)
			switch presence {
			case bind.Absent:
				return fmt.Errorf("folio8: Render: element %s: table bind %q is absent from the report data", el.ID, tbl.Bind)
			case bind.Null:
				return fmt.Errorf("folio8: Render: element %s: table bind %q is null, not an array", el.ID, tbl.Bind)
			case bind.Present:
				if val.Kind != bind.KindArray {
					return fmt.Errorf("folio8: Render: element %s: table bind %q resolved to a %s, not an array", el.ID, tbl.Bind, val.Kind)
				}
			}
		}
	}
	return nil
}

// imageRunSource is one image element found while walking the
// document's bands: its declared BOX (before fit/centre), and the asset
// key it references.
type imageRunSource struct {
	elementID  string
	assetKey   string
	x, y       geom.Length
	boxW, boxH geom.Length

	// band is documentBands' authored index: 0 pageHeader, 1 content,
	// 2 pageFooter. Story 2.6 paginates the content band alone.
	band int
}

// imageDrawsItsAsset is THE ONE READING of "this image element has a file
// to draw": a PRESENT, non-null `asset`. An image whose asset is null is a
// box the author has placed and not yet filled, and collectImageRuns below
// returns no run for it — so it reaches no content-column item by that
// route, exactly as an undeclared box reaches none by element_box.go's.
//
// It is a predicate rather than an inlined test because page_setup.go's
// canvas window count must ask the same question, and a second spelling of
// it there would be the drift element_box.go's elementDeclaresBox was
// extracted to stop.
func imageDrawsItsAsset(el template.Element) bool {
	return el.Asset.Set && !el.Asset.Null
}

// collectImageRuns walks every band in authored order and returns one
// imageRunSource per image element (AD-24, source AC3/AC4). It does not
// decode or validate the referenced asset — that is
// resolveImagePlacement's job (renderDocument), called once the union
// of images the whole document uses is known, mirroring collectTextRuns/
// resolveFace's split.
//
// collectImageRuns walks EVERY image element, visible or not (Story
// 3.5, R2/AC7): a missing width/height box or a missing asset field is
// a located error regardless of visibility, and — every image run this
// returns still goes through buildPageModel's later, deduplicated
// asset-resolution pass (asset key existence, decode) BEFORE this
// story's visibility verdicts are consulted at all, because that pass
// is keyed by DISTINCT asset key across the whole imageRuns slice, not
// by element. Filtering a hidden image out HERE, before that pass
// runs, would mean a hidden image naming an asset key absent from the
// document's assets map loads and renders successfully instead of
// erroring (AC7 subject (b)) — exactly the render.go:601-616 trap
// (Story 2.5, QA Finding 5, Major) applied to images instead of text.
// Visibility is instead consulted at PAGE-MODEL CONSTRUCTION —
// contentColumnItems and paginateDocument (below) — strictly after
// every validation an image element undergoes, never here.
func collectImageRuns(doc *Template) ([]imageRunSource, error) {
	bands, err := documentBands(doc)
	if err != nil {
		return nil, err
	}

	var runs []imageRunSource
	for bandIndex, b := range bands {
		for _, el := range b.band.Elements {
			if el.Type != template.ElementImage {
				continue
			}
			if !el.Width.Set || !el.Height.Set {
				return nil, fmt.Errorf("folio8: Render: element %s: image element has no declared width/height box", el.ID)
			}
			if !el.Asset.Set {
				// M-2: parse_bands.go already makes a missing asset on
				// an image element a load error, so this is
				// unreachable for any successfully parsed Document —
				// handled rather than assumed.
				return nil, fmt.Errorf("folio8: Render: element %s: image element has no asset", el.ID)
			}
			if !imageDrawsItsAsset(el) {
				// A placed but unfilled image box: the author has chosen
				// no file yet. There is nothing to draw and nothing has
				// gone wrong, so the run is simply absent and the render
				// completes without a diagnostic. Only the designer shows
				// the empty box, as canvas chrome that never prints.
				continue
			}
			runs = append(runs, imageRunSource{
				elementID: string(el.ID),
				assetKey:  el.Asset.Value,
				x:         el.X,
				y:         layout.PlaceInBand(b.origin, el.Y),
				boxW:      el.Width.Value,
				boxH:      el.Height.Value,
				band:      bandIndex,
			})
		}
	}
	return runs, nil
}

// resolveImagePlacement computes AC13/AC14's fit-and-centre geometry for
// one image run: the binding axis is chosen by CROSS-MULTIPLICATION
// (bw*H vs bh*W — exact integer comparison, no division, no float), then
// exactly one geom.ScaleRound call computes the free axis, and the
// centring offsets are a SECOND ScaleRound call each
// (ScaleRound(box-drawn, 1, 2)) rather than "/2" — D-1.8.4: "(bw-dw)/2
// truncates when the difference is odd... route it through the same
// function, so round-half-to-even applies and the program keeps exactly
// one rounding mode in exactly one function."
func resolveImagePlacement(run imageRunSource, img template.DecodedImage) (drawX, drawY, drawW, drawH geom.Length) {
	bw, bh := run.boxW, run.boxH
	w, h := int64(img.Width()), int64(img.Height())

	// AC13: compare bw*H against bh*W, exact integer, no division.
	if bw*geom.Length(h) <= bh*geom.Length(w) {
		// width binds
		drawW = bw
		drawH = geom.ScaleRound(bw, h, w)
	} else {
		// height binds
		drawH = bh
		drawW = geom.ScaleRound(bh, w, h)
	}

	offsetX := geom.ScaleRound(bw-drawW, 1, 2)
	offsetY := geom.ScaleRound(bh-drawH, 1, 2)

	drawX = run.x + offsetX
	drawY = run.y + offsetY
	return drawX, drawY, drawW, drawH
}

// collectTextRuns walks every band (content, pageHeader, pageFooter) in
// authored order and returns one textRunSource per non-empty text
// element, resolving each element's face via its style's fontFamily
// against the document's font fallback chains and the caller's FontSet
// — the first chain entry present in fs wins. AC9's "union of glyphs
// the document uses" is exactly the set of runes across everything this
// function returns.
//
// Each text element's authored value is first resolved against data
// via internal/bind.BindText (AC15-AC21, D-1.6.5): this is the ONLY
// site that calls BindText, so AC20's field-scope fence — bind text
// interpolation applies to text-element `value` only, never
// table.bind/columns[].bind — holds by construction rather than by a
// document-wide scan that would also visit those other fields (M-4:
// the canonical golden fixture's `columns[].bind` contains an
// expression-shaped `{{formatNumber(...)}}` that is deliberately not
// this story's business).
func collectTextRuns(doc *Template, data, params bind.Value, fs FontSet, cache *fontCache) ([]textRunSource, error) {
	// documentBands asks internal/layout where each band's own
	// (element-relative) Y=0 sits on the page: pageHeader at the top,
	// content directly below it, pageFooter starting exactly where the
	// content band ends. Every element's page Y below is
	// layout.PlaceInBand(origin, el.Y) — a TRANSLATION, never an
	// inversion (D-2.0.4, AD-24).
	bands, err := documentBands(doc)
	if err != nil {
		return nil, err
	}

	var runs []textRunSource
	for bandIndex := range bands {
		// Diagnostics discarded here: collectTextRuns is the pre-2.7
		// legacy, all-bands walker kept alive only for a handful of
		// composition tests (collect_text_runs_composition_test.go,
		// shaped_fixture_test.go) that assert on run geometry, not on
		// overflow. renderDocument (below) is the one path that
		// actually surfaces Story 2.8's Diagnostics through Render.
		// visible=nil: this legacy walker never computes visibility
		// (isVisible's nil-map case treats every element as visible),
		// byte-identical to its own pre-3.5 behaviour.
		bandRuns, _, _, berr := collectBandTextRuns(doc, bands, bandIndex, data, params, fs, cache, passthroughResolver, nil)
		if berr != nil {
			return nil, berr
		}
		runs = append(runs, bandRuns...)
	}
	return runs, nil
}

// elementTokenResolver decides what a text element's bound text — ALREADY
// through bind.BindTextSpans, so the only "{{…}}" tokens still literal
// are {{page}} and {{pages}} (internal/bind/text.go:45-50's reservation)
// — becomes before shaping. It is the seam Story 2.7 uses to give the
// content band and the two repeated bands different answers to "what
// does {{page}} mean here" without duplicating collectBandTextRuns'
// shaping/packing/positioning body (D-000.42).
type elementTokenResolver func(elementID, boundText string, subs []bind.Substitution) (resolvedText string, resolvedSubs []bind.Substitution, slots []pageSlotSpan, err error)

// passthroughResolver is collectTextRuns' resolver: {{page}}/{{pages}}
// pass through exactly as bind.BindTextSpans left them, unchanged. This
// is BYTE-FOR-BYTE today's pre-Story-2.7 behaviour — collectTextRuns
// itself never learns a page count and never needs to.
func passthroughResolver(_, boundText string, subs []bind.Substitution) (string, []bind.Substitution, []pageSlotSpan, error) {
	return boundText, subs, nil, nil
}

// contentBandResolver is D-2.7.3's fence: {{page}}/{{pages}} resolve
// ONLY in the page-header and page-footer bands. In the content band
// the construct is a located template error naming the element — not a
// silent literal — because content-band Y depends on content-band
// layout, which depends on this construct's width: a fixed point AD-24
// forbids negotiating.
func contentBandResolver(elementID, boundText string, subs []bind.Substitution) (string, []bind.Substitution, []pageSlotSpan, error) {
	if name, found := firstReservedPageToken(boundText, subs); found {
		return "", nil, nil, fmt.Errorf(
			"folio8: Render: element %s: {{%s}} resolves only in the page header and page footer bands "+
				"(D-2.7.3) — this element is in the content band, where the page count the construct "+
				"needs depends on this band's own layout, which AD-24 does not permit resolving by "+
				"negotiation",
			elementID, name,
		)
	}
	return boundText, subs, nil, nil
}

// headerFooterResolver is D-2.7.2's reservation, built once pageCount
// (Y) is known: {{pages}} becomes Y's exact digits (no reservation — Y
// is the same value on every page) and {{page}} becomes a
// digits(Y)-wide filler reservation, reported as a pageSlotSpan for
// positionSegments to mark.
func headerFooterResolver(pageCount int) elementTokenResolver {
	return func(elementID, boundText string, subs []bind.Substitution) (string, []bind.Substitution, []pageSlotSpan, error) {
		if _, found := firstReservedPageToken(boundText, subs); !found {
			return boundText, subs, nil, nil
		}
		resolved, slots, repl := resolvePageTokens(boundText, pageCount, subs)
		return resolved, shiftSubstitutions(subs, repl), slots, nil
	}
}

// diagnosticFromCaveat turns one internal/expr.Caveat (Story 3.3,
// DECISION-5) into a Diagnostic: expr may not import folio8 (the rank
// is backwards), so expr reports the raw condition and THIS function —
// the module root, where Diagnostic itself is declared — is what
// mints a code for it, following DiagCodeTextClippedWidth's own
// precedent (D-2.8.1: a code is minted where the condition first ships,
// not where internal/diag happens to exist yet).
func diagnosticFromCaveat(elementID string, c expr.Caveat) Diagnostic {
	switch c.Kind {
	case expr.CaveatEmptyAverage:
		return Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeEmptyAverage,
			ElementID: elementID,
			DataPath:  c.Path,
			Message: fmt.Sprintf(
				"element %s: avg(%s) has no operand — the collection is present and empty — so it resolves to empty rather than aborting the render (Story 4.2 requires an empty-collection table to render successfully)",
				elementID, c.Path,
			),
		}
	default:
		// Unreachable given expr.CaveatKind's own closed set (caveat.go)
		// — kept as a located, honest Diagnostic rather than a panic
		// (AD-14: never a panic), naming the unhandled kind so a future
		// caveat added there without a matching arm here fails loudly.
		//
		// Story 3.6, R12/D-3.6.7: this arm previously returned a
		// Diagnostic with an EMPTY Code — a construction AD-14 forbids
		// (every Diagnostic carries "a stable string code from a
		// closed registry"), and one that a caller could not tell
		// apart from a real, handled caveat except by a blank field
		// nobody notices. This is NOT R7's criterion being relaxed for
		// an internal condition: the arm already produces a Diagnostic
		// that is ALREADY RETURNED to a caller, so the choice is
		// between a coded one and a codeless one, never between a
		// coded one and a plain error. Giving it
		// DiagCodeInternalUnhandledCaveat makes an unmapped caveat
		// LOUD rather than blank, while the arm itself is retained
		// (AD-14: never a panic).
		return Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeInternalUnhandledCaveat,
			ElementID: elementID,
			DataPath:  c.Path,
			Message:   fmt.Sprintf("element %s: internal: unhandled expr.Caveat kind %v", elementID, c.Kind),
		}
	}
}

// collectBandTextRuns is collectTextRuns' body, generalised over ONE
// band and one elementTokenResolver — the single implementation
// collectTextRuns (legacy, all bands, pass-through) and Story 2.7's
// two-phase pipeline (content-only, then header/footer-once-Y-is-known)
// both drive, so the shaping/packing/positioning sequence exists in
// exactly one place (D-000.42).
//
// It returns, alongside the band's runs, one pendingPageSlot per
// {{page}} occurrence resolved in this call — indices LOCAL to the
// returned runs slice; the caller shifts them once the final combined
// run order is known.
// visible carries Story 3.5's pre-computed per-element visibility
// verdicts (nil for collectTextRuns' legacy, pre-3.5 callers — isVisible
// treats a nil map as "everything visible"). R2/AC7: every validation
// call in this loop (bind.BindTextSpans' path resolution, fontChain,
// shapeSegments' coverage resolution) runs UNCONDITIONALLY, whether the
// element is visible or not — render.go:601-616 (Story 2.5, QA Finding
// 5, Major) is the shipped precedent for what happens when a skip is
// placed before validation instead of after it: "the SAME broken
// template passing or failing depending on which report it was
// handed." Only the OUTPUT this function produces for a hidden element
// — its Diagnostics (AC8) and its textRunSource entries (AC1/AC2, R3)
// — is suppressed, and only after every validation call above it in
// this same loop iteration has already run and succeeded.
func collectBandTextRuns(
	doc *Template,
	bands []bandWithOrigin,
	bandIndex int,
	data, params bind.Value,
	fs FontSet,
	cache *fontCache,
	resolve elementTokenResolver,
	visible visibilityVerdicts,
) ([]textRunSource, []pendingPageSlot, []Diagnostic, error) {
	b := bands[bandIndex]
	// fc (Story 3.4, R1) is the document's formatting context —
	// declared locale plus fixed UTC offset — constructed once here,
	// at the render entry point that already holds *Template, and
	// threaded down through bind.BindTextSpans/Resolve to
	// expr.Eval. Never sourced from the host (AD-1's "no host
	// locale…").
	fc := expr.NewFormatContext(doc.doc.Locale, doc.doc.UTCOffset)
	var runs []textRunSource
	var pending []pendingPageSlot
	// diags accumulates in ELEMENT DECLARATION ORDER within this one
	// band — the `for _, el := range b.band.Elements` loop below walks
	// the authored `.folio` document in order, never a map, so this
	// slice is already the order D-2.8.6's Result.Diagnostics doc
	// comment requires WITHIN one band. renderDocument concatenates
	// this band's diags after the header band's and before the footer
	// band's to get full document order across all three.
	var diags []Diagnostic

	for _, el := range b.band.Elements {
		if el.Type != template.ElementText {
			continue
		}
		if !el.Value.Set || el.Value.Null || el.Value.Value == "" {
			continue
		}
		// Story 3.5, AC8: elVisible decides ONLY whether this
		// element's OWN diagnostics/output are emitted below — it
		// gates no validation call in this loop (R2/AC7).
		elVisible := isVisible(visible, el.ID)

		boundText, subs, caveats, berr := bind.BindTextSpans(el.Value.Value, data, params, fc, string(el.ID))
		if berr != nil {
			// Story 3.6, AC4/AC8, R9: FR41's "unresolvable binding"
			// mode — internal/bind/text.go's lookupBound reports an
			// absent path (AD-14's own "an absent path is an Error
			// carrying the path"); this is the one site R9 names for
			// this mode.
			return nil, nil, nil, expressionRuntimeError(string(el.ID), "value", fmt.Errorf("folio8: Render: %w", berr))
		}
		// Story 3.3/DECISION-5: a bind-stage Caveat (today, only
		// avg()-on-empty) becomes a Diagnostic HERE, before this
		// element's own layout-stage clip Warning (below) — D-2.8.6's
		// ordering guarantee applies pipeline-stage-first WITHIN one
		// element, and this loop already walks elements in band/
		// declaration order, so appending here in caveat order (the
		// order Resolve encountered them) keeps the whole diags slice
		// in the one required order without any separate sort.
		//
		// AC8: a hidden element emits NO diagnostic of its own,
		// including a caveat-derived one — gated here, never by
		// skipping the Caveat-producing evaluation above.
		if elVisible {
			for _, c := range caveats {
				diags = append(diags, diagnosticFromCaveat(string(el.ID), c))
			}
		}

		resolvedText, resolvedSubs, slots, rerr := resolve(string(el.ID), boundText, subs)
		if rerr != nil {
			return nil, nil, nil, rerr
		}
		boundText, subs = resolvedText, resolvedSubs

		// QA Finding 5 (this story's review, Major): the fontFamily
		// chain must be validated BEFORE the AC9 empty-text
		// short-circuit below, not after it. The previous ordering
		// let boundText == "" skip font-chain validation entirely,
		// so an element with an unresolvable style.fontFamily chain
		// (Story 1.5 AC2/AC4's located error) rendered successfully
		// whenever its bound value happened to be null or "" — the
		// SAME broken template passing or failing depending on
		// which report it was handed. AC9 only requires that a null
		// binding "renders as empty, and is not an error"; it does
		// not license skipping the element's own validation.
		chain, styledChain, err := fontChain(doc, el)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, err)
		}
		// The faces this element is actually DRAWN and MEASURED with:
		// the styled list coalesced onto the base one, so shaping,
		// leading and the digit table all read the same faces. Coverage
		// below still walks `chain` — see chainFaceNames.
		metricsChain := metricsFaceNames(chain, styledChain, fs, cache)
		// Story 10.1: the element's ink, resolved ONCE per element and
		// validated at render, through the module's one hex parser, so a
		// malformed value is a located render error naming the element and
		// the field.
		//
		// HOISTED HERE, beside fontChain, for fontChain's OWN reason and
		// not by analogy to it: this validation used to sit ~120 lines
		// below, under four `continue`s — the AC9 empty-text
		// short-circuit, the hidden-element skip, and the two before them
		// — so `style.color: "red"` was a located error when the element
		// had a visible value and NO diagnostic at all when the value was
		// "", when it bound to null, or when the element was hidden. That
		// is precisely the defect the comment above this call was written
		// to describe: the SAME broken template passing or failing
		// depending on which report it was handed. Nothing may go below
		// that line, and this had.
		//
		// The move is a MOVE: elementInk takes el.Style and el.ID only, so
		// it depends on nothing computed in between and there is exactly
		// one call, here.
		ink, hasInk, inkErr := elementInk(el.Style, string(el.ID), "style.color")
		if inkErr != nil {
			return nil, nil, nil, inkErr
		}
		// SCOPED TO THIS ELEMENT'S CHAIN, shadowing the parameter for the
		// rest of the iteration so nothing below can consult the unscoped
		// one by accident. It is the same cache — same maps, same parsed
		// faces — carrying the chain name a located error needs.
		cache := cache.forChain(el.Style.Value.FontFamily.Value)
		if boundText == "" {
			// AC9: a placeholder resolving to explicit JSON null
			// renders as empty — nothing left to draw for this run
			// — but the element's fontFamily chain still validated
			// above. There is no text, so there is nothing to check
			// coverage against (Story 2.2, AC4).
			continue
		}
		fontSize := defaultFontSizePt
		if el.Style.Set && !el.Style.Null && el.Style.Value.FontSize.Set && !el.Style.Value.FontSize.Null {
			fontSize = el.Style.Value.FontSize.Value
		}
		// Story 2.2, AC4: COVERAGE-based resolution, per rune,
		// across the chain — never "first chain member present in
		// fs" (the pre-Story-2.2 reading). May split one element
		// into several face segments, one per contiguous run of
		// runes sharing the same resolved face. Shaped ONCE here;
		// every line below is a SLICE of these glyphs, never a
		// re-shape of a shorter string (Story 2.4, AC10).
		segs, glyphDiags, serr := shapeSegments(string(el.ID), chain, styledChain, boundText, fs, cache, breaksAreConsumed)
		if serr != nil {
			return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, serr)
		}
		if !elVisible {
			// AD-24: absent from the page model entirely. Every
			// validation above this line has already run and
			// succeeded for THIS element (bind.BindTextSpans' path
			// resolution, fontChain, shapeSegments' coverage
			// resolution) — R2/AC7 — so a hidden element with a
			// broken font chain still fails the render exactly as a
			// visible one would; an uncoverable rune (Story 3.6:
			// no longer a failure, a Warning) is likewise still
			// DETECTED here, but its Diagnostic is discarded rather
			// than reported, matching AC8's rule for the bind-stage
			// caveat diagnostics above: a hidden element emits NO
			// diagnostic of its own. Everything below this line only
			// computes OUTPUT (packed lines, the clip-width
			// Diagnostic, positioned glyphs) for drawing, which a
			// hidden element never needs and must never produce
			// (AC1/AC2/AC8, R3: no run, no gap-filling substitute, no
			// diagnostic).
			continue
		}
		// Story 3.6, AC4/AC8: a missing-glyph Warning is appended in
		// the SAME position a caveat-derived Diagnostic would be —
		// this element's own diagnostics, in the order this element's
		// pipeline stages produced them (bind-stage caveats above,
		// then shaping-stage missing-glyph warnings here) — keeping
		// the whole diags slice in D-2.8.6's required document order
		// without a separate sort.
		diags = append(diags, glyphDiags...)
		totalRunes := len([]rune(boundText))

		// Story 2.4: where may this element break, and what may it
		// not break inside? The atomic spans are the document's
		// declared unbreakableValues matched against the rune spans
		// bind.BindTextSpans reported — handed to internal/text as a
		// PARAMETER (D-000.16), never through an import.
		atomic := atomicSpansFor(doc.doc.UnbreakableValues, subs)
		ops := text.Opportunities(text.Dictionary(), boundText, atomic)

		boxWidth := geom.Length(0)
		if el.Width.Set && !el.Width.Null {
			boxWidth = el.Width.Value
		}
		lines := packLines(segs, ops, totalRunes, fontSize, boxWidth)

		// Story 2.8, AC1/D-2.8.1: does this element's widest packed
		// line exceed its declared WIDTH? Computed from the SAME
		// wrappedLine.width the packer already measured — never a
		// second measurement of the text — and against boxWidth alone,
		// never el.Height (D-2.8.1: a text element's declared height is
		// not a clip bound and this function never reads el.Height).
		overflow, overflows := detectWidthOverflow(string(el.ID), lines, boxWidth)
		if overflows {
			diags = append(diags, Diagnostic{
				Severity:  SeverityWarning,
				Code:      DiagCodeTextClippedWidth,
				ElementID: overflow.elementID,
				Message:   widthClipMessage("element", "declared", overflow),
			})
		}

		// ONE vertical model for the element, from its DECLARED
		// chain (D-2.4.2 as amended): computed once, outside the
		// loop, from ONE walk of the chain, because every span of it
		// is a function of the chain and the size and of nothing on
		// any individual line.
		//
		// UNCONDITIONAL, where the superseded code computed the
		// advance only when len(lines) > 1. The first-baseline
		// offset is needed by EVERY element with at least one line,
		// so this call now runs for single-line elements too — which
		// widens the set of inputs that can reach verticalModel's two
		// error paths. That widening is measured rather than assumed:
		// see TestVerticalModelErrorPathsAreUnreachableThroughRender.
		vm, serr := chainVerticalModel(metricsChain, fontSize, styleLineSpacing(el.Style), fs, cache)
		if serr != nil {
			return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, serr)
		}

		// The element's committed alignment, applied here and nowhere else
		// in this loop: valign moves the whole block once, align moves each
		// line inside the declared width. Both distribute slack only
		// (text_alignment.go), so an element that fits exactly, one with no
		// alignment, and one that overflows all draw exactly where they drew
		// before this rule existed.
		align, valign := elementAlignment(el)
		boxHeight := geom.Length(0)
		if el.Height.Set && !el.Height.Null {
			boxHeight = el.Height.Value
		}
		elementY := layout.PlaceInBand(b.origin, el.Y) + textValignOffset(valign, boxHeight, textBlockHeight(len(lines), vm))
		startPending := len(pending)
		for i, ln := range lines {
			lineY := elementY + geom.Length(int64(i))*vm.Advance
			lineX := el.X + textAlignOffset(align, boxWidth, ln.width)
			// Story 7.3 / FR47. A justified line is drawn as several
			// rune ranges at several x positions — a piece is exactly
			// positionSegments' existing contract, so this is more
			// calls to the same function rather than a second
			// placement primitive, and `slots` reaches every one of
			// them. justifiedLinePieces returns nil for every ragged
			// case and for every element that is not justified at all,
			// so the unjustified path below is the pre-7.3 call
			// unchanged — which is what keeps the corpus byte-
			// identical (page_setup.go carries the identical branch).
			var placed []textRunSource
			if pieces := justifiedLinePieces(align, ln, i, len(lines), segs, ops, fontSize, boxWidth); pieces != nil {
				for _, piece := range pieces {
					pieceRuns, pieceErr := positionSegments(segs, piece.from, piece.to, el.X+piece.offset, lineY, fontSize, vm.FirstBaseline, slots)
					if pieceErr != nil {
						return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, pieceErr)
					}
					placed = append(placed, pieceRuns...)
				}
			} else {
				var poserr error
				placed, poserr = positionSegments(segs, ln.from, ln.to, lineX, lineY, fontSize, vm.FirstBaseline, slots)
				if poserr != nil {
					return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, poserr)
				}
			}
			// Story 2.6: the LINE's extent, computed here from the
			// vertical model that is already in hand — `lineY` IS
			// `baseline − max(ascent)` because positionSegments
			// places the baseline at lineY + vm.FirstBaseline, and
			// vm.FirstBaseline IS max(ascent) scaled. The bottom
			// adds max(descent). Same numbers, no re-derivation.
			for j := range placed {
				placed[j].band = bandIndex
				placed[j].elementID = string(el.ID)
				placed[j].lineIndex = i
				placed[j].itemTop = lineY
				placed[j].itemBottom = lineY + vm.FirstBaseline + vm.LastDescent
				// Story 2.8, AC6: every run of an overflowing element
				// carries the SAME clip box (the element's declared
				// left edge and width) — clipping is a property of the
				// ELEMENT, not of any one line or face segment within
				// it, so a multi-line or multi-face-segment overflow
				// clips uniformly across all its runs.
				if overflows {
					placed[j].clipToBox = true
					placed[j].clipX = el.X
					placed[j].clipWidth = boxWidth
				}
				if hasInk {
					placed[j].hasColor = true
					placed[j].color = ink
				}
				// Story 2.7 review, Blocker 1: ONE pendingPageSlot per
				// {{page}} occurrence this run carries, not one per
				// run — a run may carry more than one.
				for _, ps := range placed[j].pageSlots {
					pending = append(pending, pendingPageSlot{
						runIndex: len(runs) + j,
						glyphLo:  ps.glyphLo,
						glyphHi:  ps.glyphHi,
						digitsY:  ps.digitsY,
					})
				}
			}
			runs = append(runs, placed...)
		}

		// Story 2.7: this element used a {{page}} slot, so its face's
		// ten digits need CIDs allocated even though only ONE filler
		// digit was actually shaped above (finding 3, story creation:
		// digit identity never affects width, but substitution needs
		// EVERY digit's CID, since any of 0-9 may be a page's own).
		if len(slots) > 0 && len(pending) > startPending {
			dt, dterr := digitTableRun(chain, styledChain, fontSize, fs, cache)
			if dterr != nil {
				return nil, nil, nil, dterr
			}
			dtIndex := len(runs)
			runs = append(runs, dt)
			for k := startPending; k < len(pending); k++ {
				pending[k].digitTableIndex = dtIndex
			}
		}
	}
	return runs, pending, diags, nil
}

// widthOverflow is Story 2.8, AC1's per-element horizontal overflow
// record: the element id, the declared bound and the measured extent
// (the widest packed line's width). The axis is always "width" —
// D-2.8.1 fences the vertical axis out entirely, so there is no axis
// field to carry. detectWidthOverflow is the SINGLE place this record
// is computed, driving both the emitted Diagnostic (AC1/AC7) and the
// clip decision (AC6): an element that does not overflow never reaches
// either.
type widthOverflow struct {
	elementID     string
	declaredWidth geom.Length
	measuredWidth geom.Length
}

// detectWidthOverflow reports whether lines' widest member exceeds
// boxWidth (D-2.8.1: the declared WIDTH is FR44's only clip bound; a
// text element's declared HEIGHT is read by nothing here, exactly as
// finding 1 measured it is read by nothing in the pre-2.8 renderer). A
// zero-or-negative boxWidth means "no declared width" — packLines'
// own convention (wrap.go: "el.Width.Set && !el.Width.Null" gates
// boxWidth above; an absent width is always boxWidth == 0) — and never
// overflows: there is no bound to measure against.
func detectWidthOverflow(elementID string, lines []wrappedLine, boxWidth geom.Length) (widthOverflow, bool) {
	if boxWidth <= 0 {
		return widthOverflow{}, false
	}
	widest := geom.Length(0)
	for _, ln := range lines {
		if ln.width > widest {
			widest = ln.width
		}
	}
	if widest <= boxWidth {
		return widthOverflow{}, false
	}
	return widthOverflow{elementID: elementID, declaredWidth: boxWidth, measuredWidth: widest}, true
}

// tableCollectionSegments splits a table's `bind` field (a bare
// collection path plus its trailing "[]", e.g. "transactions[]") into
// the dotted segments bind.Value.Lookup expects. Story 4.2 review
// Finding 18: checkTableBindings (above, which validates the
// collection BEFORE render) and collectBandTableRuns (table_render.go,
// which reads it during render) each reproduced this exact two-line
// parse independently; they are now the SAME function, so the two can
// never silently drift into resolving different collections for the
// same bind string.
func tableCollectionSegments(bind string) []string {
	collection := strings.TrimSuffix(bind, "[]")
	if collection == "" {
		return nil
	}
	return strings.Split(collection, ".")
}

// widthClipMessage builds FR44's clip Warning message, shared by BOTH
// sites that reuse detectWidthOverflow (collectBandTextRuns's own text
// element, and table_render.go's data cell — D-000.65: no new
// diagnostic code, and now no independently-drifting message text
// either).
//
// noun is "element" for a text element's own declared width, or
// "column" for a table cell (Story 4.2 review Finding 15: the table
// site previously hand-copied this string and still said "element"
// even though overflow.elementID there names a COLUMN — AC4's own
// grounds are that columns[].id exists precisely so a diagnostic can
// name a column). widthLabel is "declared" (a text element's own box
// width IS its declared width) or "content" (a column's clip bound is
// its declared width MINUS padding — a genuinely different quantity,
// not merely a re-spelling, which is why this parameter exists rather
// than being folded into noun).
func widthClipMessage(noun, widthLabel string, overflow widthOverflow) string {
	return fmt.Sprintf(
		"%s %s: the widest laid-out line is %s wide, exceeding the %s's %s "+
			"width of %s; the overflowing content is clipped at the %s's left/right edges, "+
			"never reflowed and never dropped (FR44)",
		noun, overflow.elementID, millipoints(overflow.measuredWidth), noun, widthLabel, millipoints(overflow.declaredWidth), noun,
	)
}

// millipoints spells a geom.Length for a HUMAN-READABLE Diagnostic
// message (Story 2.8). Not an output-format emitter: nothing in a PDF,
// a page model or a golden passes through here, so AD-3's "one number
// emitter" (internal/pdf's numbers.go) is untouched — this mirrors
// internal/layout/paginate.go's own millipoints, kept package-local
// exactly as that one is, because the property it serves (a readable
// diagnostic) is local to each package that needs one.
func millipoints(v geom.Length) string {
	return strconv.FormatInt(int64(v), 10) + "mp"
}

// lookupFontChain is THE one site that turns a style.fontFamily NAME
// into the document's entries, and the one owner of the error when the
// name resolves to nothing.
//
// IT WAS EXTRACTED BECAUSE IT HAD ALREADY DRIFTED INTO TWO (Story 8.4,
// Task 3). fontChain below and collectBandTableRuns (table_render.go)
// each held their own copy of the Fonts.Chain call and their own copy of
// its message, the second under a comment that said so verbatim —
// "Mirrors fontChain's own error, verbatim in shape (render.go)". A
// message maintained in two places is a message that will be maintained
// in one.
//
// WHAT IS NOT SHARED, deliberately: the "this element has no
// style.fontFamily at all" check. Its two spellings say genuinely
// different things — a text element "has text but no style.fontFamily",
// a table column "has a column label but no style.fontFamily (nor
// headerStyle.fontFamily)" — and collapsing them would tell a table
// author about a field their element does not have.
//
// The error is returned BARE, without the "folio8: Render: element %s:"
// prefix, because both callers already apply their own (fontChain's
// caller in collectBandTextRuns, and collectBandTableRuns inline).
func lookupFontChain(doc *Template, chainName string) ([]template.FontChainEntry, error) {
	chain, ok := doc.doc.Fonts.Chain(chainName)
	if !ok {
		return nil, fmt.Errorf("style.fontFamily %q names a chain with no entries in the document's fonts map", chainName)
	}
	return chain, nil
}

// fontStyleOf is THE (bold, italic) -> template.FontStyle mapping, and
// the only place the two booleans are turned into the closed set the
// chain answers. Two callers hold the pair — a text element's own style,
// and a table header's cascaded one — and neither may spell the mapping
// itself.
func fontStyleOf(bold, italic bool) template.FontStyle {
	switch {
	case bold && italic:
		return template.FontStyleBoldItalic
	case bold:
		return template.FontStyleBold
	case italic:
		return template.FontStyleItalic
	}
	return template.FontStyleRegular
}

// styleFontStyle reads the requested weight and slope off one resolved
// Style. OFF IS ABSENT-OR-FALSE AND ON IS NOTHING ELSE: a Presence that
// is unset, explicitly null, or set to false all mean "not this", which
// is the same three-state reading every other style field here uses.
func styleFontStyle(st template.Style) template.FontStyle {
	return fontStyleOf(
		st.Bold.Set && !st.Bold.Null && st.Bold.Value,
		st.Italic.Set && !st.Italic.Null && st.Italic.Value,
	)
}

// elementFontStyle is styleFontStyle over an element's optional style
// block: an element with no style at all requests no variant.
func elementFontStyle(el template.Element) template.FontStyle {
	if !el.Style.Set || el.Style.Null {
		return template.FontStyleRegular
	}
	return styleFontStyle(el.Style.Value)
}

// fontChain resolves one text element's style.fontFamily to its ordered
// fallback chain of face names (AD-8's Rule; AC3), together with the
// STYLED list its own style.bold/style.italic select (FR57).
//
// It does not touch coverage — resolveRuneFace does that, per rune,
// against the BASE list, and only against the base list. See
// chainFaceNames for why the two are parallel rather than substituted.
func fontChain(doc *Template, el template.Element) (base, styled []string, err error) {
	if !el.Style.Set || el.Style.Null || !el.Style.Value.FontFamily.Set || el.Style.Value.FontFamily.Null {
		return nil, nil, fmt.Errorf("has text but no style.fontFamily to resolve a font from")
	}
	chain, err := lookupFontChain(doc, el.Style.Value.FontFamily.Value)
	if err != nil {
		return nil, nil, err
	}
	base, styled = chainFaceNames(chain, elementFontStyle(el))
	return base, styled, nil
}

// metricsFaceNames is the list chainLineMetrics walks: EVERY FACE THAT
// MAY ACTUALLY DRAW a rune of this element, in a fixed order.
//
// It is a PROJECTION of chainFaceNames' two slices, never a third
// derivation of either. A styled list of nil means no variant was
// requested at all, and the base list is already the answer.
//
// ⚠ IT ADDS THE VARIANT, IT DOES NOT REPLACE THE BASE — AND THAT IS A
// DEFECT THIS FUNCTION SHIPPED TWICE BEFORE GETTING RIGHT. It began as a
// coalesce: `out[i] = styled[i]`. Two conditions break that, and both
// end the same way — the leading derived from a face the glyphs did not
// come from:
//
//   - THE VARIANT IS NOT SUPPLIED. faceCovers opens with cache.declares,
//     so shaping skips it and draws the base face; an uncoalesced name
//     left chainLineMetrics with no present face at all, and a
//     single-entry chain ABORTED the render on a document the format
//     calls valid.
//   - THE VARIANT IS SUPPLIED BUT DOES NOT COVER THE RUNE. Coverage
//     chose the entry on its BASE face, so the variant is not guaranteed
//     to carry the glyph; shaping falls back to the base (AC3's absence
//     arm) while a replaced list showed chainLineMetrics only the
//     variant. No error, just wrong leading, silently.
//
// Both faces can draw, so both belong. That is not a widening of the
// vertical model's rule but the plain reading of it: chainLineMetrics
// has always walked the DECLARED chain — every member, used or not —
// and a styled entry declares two faces rather than one.
//
// The order is base-then-variant per entry, stated because the model is
// a max() that cannot observe it: an order left to chance is one a later
// reader cannot rely on.
// PRECONDITION: styled is nil, or len(styled) == len(base) with the same
// order — chainFaceNames is the only producer of the pair and builds
// them together, so this holds by construction. It is STATED rather than
// guarded: a runtime check here would be a guard for a case no
// acceptance criterion demonstrates, and TestTheTwoChainSlicesAreAligned
// pins the producer instead.
func metricsFaceNames(base, styled []string, fs FontSet, cache *fontCache) []string {
	if styled == nil {
		return base
	}
	out := make([]string, 0, 2*len(base))
	for i := range base {
		out = append(out, base[i])
		if styled[i] != "" && styled[i] != base[i] && cache.declares(styled[i], fs) {
			out = append(out, styled[i])
		}
	}
	return out
}

// chainFaceNames is THE one boundary between the document's chain and
// the render path's face-name list.
//
// WHAT IT DOES, SINCE STORY 8.4: it maps EVERY entry to a face name,
// preserving the document's authored order. An entry naming a face the
// FontSet supplies contributes that name; an entry naming a face the
// DOCUMENT carries contributes the reserved name embeddedFaceName
// derives from its ASSET KEY (embedded_face.go — and the asset key, never
// font.family, is what AD-8/D-8.4.1 make the resolver). Nothing is
// dropped any more.
//
// UNTIL STORY 8.4 AN EMBEDDED ENTRY WAS DROPPED HERE, and the comment
// that stood in this place said so. That was the honest interim state:
// the format could express a face the renderer could not draw. It is no
// longer the state, and the change is visible at exactly one line —
// which is the whole reason this boundary exists as a function.
//
// THE NAME IS ALL THIS FUNCTION KNOWS. It resolves no bytes and reads no
// asset: whether the minted name can actually supply a face is the
// fontCache's question, asked lazily at the point of use, so an entry
// naming a non-font asset that nothing ever draws from costs a render
// nothing and errors nowhere (Story 8.4's I/O matrix, "never drawn").
//
// Both consequences are pinned by test rather than left to this comment
// — chain_face_names_test.go, in THIS package, which is the only package
// that can call Render. (An earlier version of this comment cited
// internal/template's fonts_embedded_test.go, which is `package template`
// and structurally cannot reach Render at all; the citation was wrong and
// the tests it claimed did not exist.)
//
// IT IS CONVERTED HERE, AT ONE BOUNDARY, rather than by widening the
// consumers to the richer type. That comment used to name FOUR of them —
// resolveRuneFace/chainLineMetrics/shapeSegments/formatFontChain — and
// the four were neither the population nor the risk set (D-8.2.3: a hole
// in one arm of an enumeration is evidence about the enumeration).
// MEASURED at 15ca0dd: `chain []string` is a parameter of TEN non-test
// functions, and the SIX that also take (FontSet, *fontCache) are the
// real seam:
//
//	resolveRuneFace (render.go)        FontSet + fontCache   named before
//	shapeSegments (render.go)          FontSet + fontCache   named before
//	digitTableRun (page_number.go)     FontSet + fontCache   NOT named
//	chainLineMetrics (wrap.go)         FontSet + fontCache   named before
//	chainVerticalModel (wrap.go)       FontSet + fontCache   NOT named
//	lineAdvance (wrap.go)              FontSet + fontCache   NOT named
//	formatFontChain (render.go)        *fontCache            named before
//	missingGlyphMessage (render.go)    *fontCache            NOT named
//	verticalModel (wrap.go)            neither               NOT named
//	scaleAdvanceByLineSpacing (wrap.go) neither              NOT named
//
// ⚠ THE SPLIT IS 6/2/2, NOT 6/4, AND THIS TABLE WAS STALE. Story 8.4
// gave formatFontChain and missingGlyphMessage a *fontCache of their own
// (so an embedded entry could be spelled by display name), which moved
// them out of the "neither" column this table used to put them in. Only
// verticalModel and scaleAdvanceByLineSpacing take neither now.
// Re-measured at 3ad4ede.
//
// None of the ten can reach a *Template, so none can reach Assets. The
// two that take neither consume the chain for vertical arithmetic only —
// they need the names and never the bytes — and that asymmetry is why
// Story 8.4 put the name -> bytes view behind the fontCache the six
// already hold, instead of widening six signatures into six answer
// sites. See embedded_face.go for the choice and the rejected
// alternative.
//
// ---------------------------------------------------------------------
// STORY 11.2: TWO ALIGNED SLICES, AND WHY IT IS NOT ONE SUBSTITUTED ONE.
//
// base is what it always was: one name per entry, authored order. styled
// is the SAME LENGTH AND THE SAME ORDER, holding the face the entry
// declares for want — "" where it declares none. It is nil, entirely,
// when want is FontStyleRegular: nothing is restyled, so there is
// nothing to carry, and every caller's pre-11.2 behaviour is the nil
// path unchanged.
//
// COVERAGE WALKS base AND ONLY base. If a bold name replaced its base
// name BEFORE coverage ran, a variant whose cmap is narrower than its
// base would push a rune to the NEXT ENTRY — silently changing the
// TYPEFACE in order to keep the WEIGHT, which is precisely the
// substitution AD-8 and D-B forbid by name. The declared variant is
// applied WITHIN the entry coverage already chose, and the index is what
// carries the correspondence between the two slices.
//
// A VARIANT IS READ, NEVER CONSTRUCTED. There is no `entry.Face + " Bold"`
// here and there must never be one: FR57 says the mapping is "resolved
// per rune through the DECLARED chain", so an entry that declares no
// variant has none, however suggestively the supplied FontSet happens to
// be named. TestABareEntryNeverConstructsAVariantName is the thing that
// keeps this true.
//
// A variant's NAMESPACE follows its entry's: an embedded entry's variant
// is an assets key and is minted into the reserved namespace exactly as
// the entry's own key is; a face entry's variant is a FontSet name and
// passes through verbatim. The parser has already refused the
// cross-namespace case, so nothing here has to decide it.
func chainFaceNames(chain []template.FontChainEntry, want template.FontStyle) (base, styled []string) {
	base = make([]string, 0, len(chain))
	if want != template.FontStyleRegular {
		styled = make([]string, 0, len(chain))
	}
	for _, entry := range chain {
		name := entry.Face
		if entry.Embedded() {
			name = embeddedFaceName(entry.AssetKey)
		}
		base = append(base, name)
		if want == template.FontStyleRegular {
			continue
		}
		variant := entry.Variant(want)
		if variant != "" && entry.Embedded() {
			variant = embeddedFaceName(variant)
		}
		styled = append(styled, variant)
	}
	return base, styled
}

// fontCache parses a face's bytes into a *fontset.Font at most once per
// distinct face name, across the whole render (coverage checks touch
// every candidate face in a chain, not just the one ultimately used,
// so without this a long document would re-parse the same face
// repeatedly). Looked up and written only by key — NEVER ranged
// (ScanMapRange, D-2.2.3's whole-module scan).
//
// SINCE STORY 8.4 IT IS ALSO THE ONE ANSWER SITE for "where do this
// face's bytes come from". Two sources, in a fixed PRECEDENCE: a name in
// the reserved embedded namespace resolves from the DOCUMENT's own
// assets and the supplied FontSet is not consulted for it at all; every
// other name resolves from the FontSet. That order is what makes a
// collision harmless in both directions — a caller cannot shadow a
// document's carried face, and a document cannot capture a caller's
// (embedded_face.go states the rule; TestEmbeddedFaceWinsOverAColliding-
// FontSetKey pins it).
type fontCache struct {
	byName map[string]*fontset.Font
	// embedded is the per-render name -> asset view. Empty for a cache
	// built without a document, which is exactly the pre-8.4 behaviour.
	embedded embeddedFaceIndex
	// failedEmbedded memoizes an embedded face's DECODE FAILURE, so a
	// document whose chain names a non-font asset does not re-base64 and
	// re-sniff ~47 KB once per element that consults the chain. Only
	// embedded failures are cached: the FontSet arm's error is a map miss
	// and costs nothing to recompute.
	//
	// KEYED BY FACE NAME **AND CHAIN NAME**, because the error's printed
	// address is per chain (embeddedFaceSource.siteIn): the same
	// unreadable asset named by two chains is one failure with two
	// addresses, and one memo entry would hand the second chain's element
	// the first chain's coordinates. It matters more since the canvas
	// degrades rather than aborting — a projection walks every element of
	// a broken document instead of stopping at the first.
	failedEmbedded map[string]error
	// chainName is the DOCUMENT CHAIN this view of the cache is being
	// consulted through, or "" for an unscoped one. See forChain.
	chainName string
	// substitution memoizes the pool and the per-rune answer derived from
	// it. A POINTER, because forChain hands out struct COPIES that share
	// the maps by reference: a plain field would be rebuilt once per
	// chain view, which is once per table column.
	substitution *substitutionMemo
	// fallback is the caller's resolution mode, and it RIDES THE CACHE
	// rather than shapeSegments' signature. The cache already knows
	// which faces the renderer holds — its two arms in get, embedded
	// then supplied, ARE the substitution pool's two arms and their
	// order — and it already reaches every site that resolves a rune to
	// a face, so nothing else had to widen. The ZERO VALUE is
	// FaceFallbackStrict, which is what newFontCache (test fixtures)
	// and every pre-selector caller get.
	fallback FaceFallback
}

// forChain returns a view of this cache scoped to one document chain, and
// it is how the chain's NAME reaches a located error without widening a
// single one of the ten chain consumers.
//
// WHY IT IS NEEDED. The face name a carried face resolves under is derived
// from the ASSET KEY ALONE (AD-8/D-8.4.1), so one asset named by two chains
// is ONE face name — while the address an author must be sent to is per
// chain. Nothing downstream of chainFaceNames carries a chain name: they
// carry a []string. Without this, the error names whichever chain sorted
// first, which may be a chain the failing element does not draw through.
//
// WHY IT IS A VIEW AND NOT A SECOND CACHE. All three maps are shared by
// reference, so a face parsed through one chain is not re-parsed through
// another and the whole memoization argument for this type survives intact.
// Only the printed address differs.
//
// The three production sites that know a chain's name call it:
// collectBandTextRuns (render.go), collectBandTableRuns (table_render.go)
// and addCanvasTextPaint (page_setup.go) — the same three that call
// fontChain/lookupFontChain. A caller with no chain name uses the cache
// unscoped and gets the deterministic first-occurrence address.
func (c *fontCache) forChain(chainName string) *fontCache {
	if c == nil || c.chainName == chainName {
		return c
	}
	scoped := *c
	scoped.chainName = chainName
	return &scoped
}

// failedEmbeddedKey is the memo key, spelled once.
func failedEmbeddedKey(name string, site template.FontChainSite) string {
	return name + "\x00" + site.ChainName
}

// newFontCache builds a cache with NO document behind it: every name
// resolves from the supplied FontSet, which is the pre-Story-8.4
// behaviour and is all a test fixture over shipped faces needs.
//
// PRODUCTION CODE HOLDING A *Template MUST USE newDocumentFontCache.
// There are exactly two such sites (predictDocument here, and
// addCanvasTextPaint in page_setup.go) and they must agree, or the canvas
// measures with a different set of faces than the page prints with —
// which is AD-17's whole subject. TWO tests pin it, and each covers what
// the other cannot: TestOnlyTheTwoDocumentAwareSitesBuildAFontCache
// (font_cache_sites_test.go) scans this package's non-test sources and
// fails on a THIRD site or a missing one, and
// TestCanvasMeasuresWithTheEmbeddedFace fails if the canvas's own site
// loses the document.
func newFontCache() *fontCache {
	return &fontCache{
		byName:         map[string]*fontset.Font{},
		embedded:       embeddedFaceIndex{},
		failedEmbedded: map[string]error{},
		substitution:   newSubstitutionMemo(),
	}
}

// newDocumentFontCache is newFontCache plus the document's own carried
// faces (Story 8.4) and the caller's FaceFallback selector.
//
// THE SELECTOR IS A CONSTRUCTOR PARAMETER, NOT A FIELD SET AFTERWARDS,
// so a site that builds a cache cannot forget to state which mode it is
// in — the two production sites answer the question in opposite ways
// (predictDocument forwards the caller's; addCanvasTextPaint pins
// FaceFallbackStrict and says why), and a default would have made the
// canvas's answer invisible.
func newDocumentFontCache(t *Template, fallback FaceFallback) *fontCache {
	return &fontCache{
		byName:         map[string]*fontset.Font{},
		embedded:       newEmbeddedFaceIndex(t),
		failedEmbedded: map[string]error{},
		substitution:   newSubstitutionMemo(),
		fallback:       fallback,
	}
}

// isEmbedded reports whether name is one this cache resolves from the
// document rather than from a FontSet. It is the discriminant, spelled
// once.
func (c *fontCache) isEmbedded(name string) bool {
	_, ok := c.embedded.source(name)
	return ok
}

// carriedAssetKey answers, for a face name this cache RESOLVED, which of
// the document's own assets it came from — the inverse of the mint at
// embedded_face.go, and the accessor Story 8.4a's canvas projection uses
// to attribute a painted fragment to a carried face.
//
// THE INDEX IS ASKED FIRST, AND THAT ORDER IS THE POINT. A name's shape
// is not evidence: a caller's FontSet may legally file an entry under any
// string at all, and a shipped face that merely LOOKS minted must not be
// reported as one the document carries. The index is the resolution
// namespace (AD-8: the asset key decides), so membership in it is the
// question, and the prefix is only read afterwards — in the one file that
// writes it.
//
// The two agree by construction: newEmbeddedFaceIndex keys the index by
// embeddedFaceName(entry.AssetKey) and records that same AssetKey, so the
// key read back off the name is the key the entry named. Asserted rather
// than assumed by TestACarriedFragmentIsAttributedToItsAssetKey.
func (c *fontCache) carriedAssetKey(name string) (string, bool) {
	if c == nil || !c.isEmbedded(name) {
		return "", false
	}
	return embeddedFaceAssetKey(name)
}

// declares reports whether name COULD supply a face at all — the
// tolerance predicate a chain walk applies before consulting an entry.
// It replaced the open-coded `_, ok := fs[name]` at resolveRuneFace and
// chainLineMetrics, which could not see an embedded name and so silently
// skipped every carried face.
//
// It answers from DECLARATIONS ONLY and decodes nothing: an embedded name
// declares a face whether or not its bytes turn out to be readable. That
// is deliberate — the readability question is asked at the point of use,
// by get, and answering it here would make it fire for a document that
// never draws with the entry.
func (c *fontCache) declares(name string, fs FontSet) bool {
	if c.isEmbedded(name) {
		return true
	}
	_, ok := fs[name]
	return ok
}

// THE D-8.4.12 STAMP LIVES IN internal/template, NOT HERE, and the reason
// is worth recording because it moved during implementation.
//
// It was first written as a package-local error type declared beside this
// cache — the obvious home, next to the door that mints it. That reddened
// TestFolio8MethodNamesAreInjective (render_arch_test.go): a second
// root-file receiver type declaring Error and Unwrap makes
// buildFolio8CallGraph's name-keyed method map lossy, and that guard's own
// remedies — rename the methods, or take DW-20 — are respectively
// impossible for `error` and a separate story. So the CARRIER moved; the
// DOOR did not. The attribution is still minted at exactly one site, in
// get's embedded arm below, and package folio8's root files gain no method
// at all. Attribution is a format fact, so template.CarriedFaceError also
// sits beside the document-attribution error it generalises, which is a
// better home than this one on the merits and not only on the guard's.

// get parses and caches the face on first use. A face NAMED in a chain
// but ABSENT from fs is reported here, once, the first time that chain
// entry is actually consulted — not a document-wide upfront validation
// pass, matching this package's existing "validate at the point of use"
// shape (resolveFace's prior behaviour). Story 8.4's embedded arm keeps
// exactly that shape: the document's asset is base64-decoded, checked
// against the recognised-font set and parsed HERE, the first time
// something must actually draw with it.
//
// THE EMBEDDED ARM IS CHECKED FIRST, and that ordering is the precedence
// rule the type's doc comment states.
func (c *fontCache) get(name string, fs FontSet) (*fontset.Font, error) {
	if f, ok := c.byName[name]; ok {
		return f, nil
	}
	if src, ok := c.embedded.source(name); ok {
		// The chain this cache is SCOPED TO decides the address in the
		// error, not whichever chain naming this asset sorted first. An
		// unscoped cache gets the deterministic first-occurrence answer.
		site := src.siteIn(c.chainName)
		key := failedEmbeddedKey(name, site)
		if err, failed := c.failedEmbedded[key]; failed {
			return nil, err
		}
		f, err := c.parseEmbedded(name, src, site)
		if err != nil {
			// THE STAMP, applied to every error this arm produces
			// (D-8.4.12). It goes on the MEMOIZED value and not only
			// on the value returned here, or the second element to
			// consult the same broken carried face would be handed an
			// unstamped answer and abort the projection the first one
			// was allowed to survive.
			stamped := &template.CarriedFaceError{Err: err}
			c.failedEmbedded[key] = stamped
			return nil, stamped
		}
		c.byName[name] = f
		return f, nil
	}
	data, ok := fs[name]
	if !ok {
		return nil, fmt.Errorf("face %q is not present in the supplied FontSet", name)
	}
	f, err := fontset.New(name, data)
	if err != nil {
		// UNSTAMPED, DELIBERATELY (D-8.4.12 guardrail 6). This face is
		// the CALLER's, and a caller's unreadable face is not a
		// document property: there is no edit an author can make on the
		// canvas that repairs it, so it keeps aborting the projection.
		// The two arms of this one function are the whole attributability
		// axis, which is why the stamp lives here and not at a consumer.
		return nil, err
	}
	c.byName[name] = f
	return f, nil
}

// parseEmbedded is the font analogue of predictDocument's image loop
// (below): decode the asset's bytes, refuse a media type this build
// cannot read, and hand the bytes to internal/fontset — which takes
// bytes and does not care where they came from. No network, no host font,
// no path on disk is read on this path, which is AC2's whole claim.
func (c *fontCache) parseEmbedded(name string, src embeddedFaceSource, site template.FontChainSite) (*fontset.Font, error) {
	raw, err := src.decodeAt(site)
	if err != nil {
		return nil, err
	}
	f, err := fontset.New(name, raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", site, err)
	}
	return f, nil
}

// metricsFace is chainLineMetrics' door onto the cache, and it exists
// because the vertical-model walk and coverage resolution ask DIFFERENT
// questions of the same chain.
//
// chainLineMetrics walks EVERY entry of a chain to derive the line
// height, tolerating a member the caller did not supply on the stated
// ground that such a member "cannot appear in the element, so it does not
// constrain the vertical model". An embedded entry whose asset this build
// cannot read as a font cannot appear in the element either, for exactly
// the same reason — so it is tolerated on exactly the same ground, and
// contributes no metrics.
//
// THE DECODE IS ATTEMPTED HERE, AND ONLY ITS FAILURE IS TOLERATED. An
// embedded name always declares (fontCache.declares), so this walk calls
// get, which base64-decodes the asset and parses it. A READABLE carried
// face is therefore decoded on the metrics walk and DOES constrain the
// vertical model, which is what makes the canvas and the page agree on
// line advance. What is swallowed is the error, and nothing else.
//
// IT IS NOT TOLERATED AT COVERAGE RESOLUTION (resolveRuneFace), which is
// where the renderer is actually asked to DRAW with the entry and refuses,
// located. The two answers are consistent rather than contradictory: if a
// render completes at all, coverage never reached the entry, and its
// absence from the vertical model is then exactly right — nothing it
// could have constrained was drawn with it.
//
// A FontSet face that fails to PARSE is still a hard error here, unchanged.
// Only the carried arm is tolerated, and only for a resolution failure.
//
// D-8.4.12 RECONCILIATION: ONE CONDITION USED TO HAVE THREE DISPOSITIONS.
// "The document carries a face whose bytes will not parse" was answered
// three different ways — silently tolerated here, aborted at the canvas's
// shapeSegments arm, refused located on the render path. It now has two,
// and the two are the two ANSWERS OF ONE QUESTION rather than two rulings:
// does the element actually DRAW with that entry?
//
//	drawn with     -> resolveRuneFace fails, the canvas degrades that one
//	                  element (page_setup.go) and Render refuses, located
//	not drawn with -> this walk skips it and it constrains no metrics
//
// The two are reached on DISJOINT conditions — resolveRuneFace stops at the
// first covering entry, so an entry this walk tolerates is one no rune
// needed — which is the same reason D-8.4.12 forbids pre-resolving a whole
// chain to obtain the attribution. The surviving asymmetry is between
// SURFACES, not within one: the canvas degrades where the page refuses,
// because a wrong page must not print and a document must still open.
//
// THE TOLERANCE IS SPELLED AS THE STAMP, not as a second name predicate.
// It read `if c.isEmbedded(name)` — a consumer re-deciding attribution from
// the face's NAME, which is exactly the shape D-8.4.12 ruled against at the
// canvas gate. Exactly equivalent today (an embedded name takes get's
// embedded arm and every error out of that arm is stamped), and equivalent
// by construction rather than by coincidence afterwards: there is now ONE
// spelling of "this fault is the document's" in this package.
//
// THE COUPLING THAT CREATES, STATED. Reading the stamp rather than the name
// means this tolerance would widen if the stamp were ever applied to get's
// FontSet arm as well — a caller's unreadable face would stop constraining
// the vertical model. That cannot arrive quietly: stamping the FontSet arm
// reddens TestCanvasStillAbortsOnAHostFontSetFaceThatWillNotParse, which is
// D-8.4.12 guardrail 6's assertion and is red-proved against exactly that
// mutation.
func (c *fontCache) metricsFace(name string, fs FontSet) (*fontset.Font, bool, error) {
	if !c.declares(name, fs) {
		return nil, false, nil
	}
	f, err := c.get(name, fs)
	if err != nil {
		var carried *template.CarriedFaceError
		if errors.As(err, &carried) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return f, true, nil
}

// resolveRuneFace is AC4's COVERAGE-based resolution: walk chain in
// order and return the first face name that is BOTH present in fs AND
// whose cmap actually contains a glyph for r (never a proxy such as
// "the locale is ja" or "the face is not the preferred one for this
// script" — D-2.2-D4). A chain member absent from fs is skipped, not an
// error by itself.
//
// Story 3.6 (divergence 6, OPEN-1 ruled): "no face in chain covers r"
// is NO LONGER an error here — a rune uncovered by every face in its
// element's declared chain is FR41's fifth mode, ruled a WARNING
// (AD-8, EXPERIENCE.md:216, UX-DR22, Story 5.12's first AC), not an
// aborting failure. found reports whether coverage was located; when
// it is false and err is nil, the caller (shapeSegments) is the one
// that turns the absence into a Diagnostic and OMITS the rune (OPEN-1's
// ruling: no glyph, no advance — never `.notdef`, never a substituted
// replacement glyph). err is reserved for a genuine hard failure
// (today, only cache.get's face-parse error) that still aborts the
// render — that is a different condition from "no coverage" and must
// not be folded into it.
//
// ⚠ IT RETURNS THE ENTRY'S INDEX, NOT ITS NAME, SINCE STORY 11.2. The
// index is what ties the base chain to the parallel STYLED chain
// chainFaceNames produces beside it: the caller needs to know WHICH
// ENTRY covered the rune, so it can apply that entry's own declared
// variant and no other entry's. A name answers a different question, and
// a name is one index away anyway.
//
// ⚠ THE COMMENT ABOVE BELONGS TO resolveRuneFace, WHICH IS BELOW
// faceCovers — godoc attaches a comment to the declaration that FOLLOWS
// it, so this block is deliberately not adjacent to a func line. Read it
// with resolveRuneFace; faceCovers has its own comment.

// faceCovers is the single "does this face draw this rune" test, shared
// by resolveRuneFace's walk of the base chain and by shapeSegments'
// styled arm — so a declared variant is checked for coverage in exactly
// the way a base face is, and the FontSet tolerance is asked once rather
// than spelled twice.
//
// ⚠ IT ANSWERS IN TWO BOOLEANS, NOT ONE (spec-deferred-offline-cache
// CAP-7). covers is "this face draws r". absent is "the caller never
// supplied this face at all", and it is the reason covers is false
// whenever it is true — the two used to collapse into a single false,
// and a caller could no longer tell a face that was supplied and has no
// glyph from one it was never given. What each caller does with absent
// is its own rule: resolveRuneFace collects it, shapeSegments' styled
// arm discards it.
func faceCovers(name string, r rune, fs FontSet, cache *fontCache) (covers, absent bool, err error) {
	if !cache.declares(name, fs) {
		// ABSENT IS REPORTED ALONGSIDE THE MISS, NOT FOLDED INTO IT
		// (spec-deferred-offline-cache CAP-7). Until this story the two
		// collapsed into one `false`, and the caller could no longer tell
		// "this face was supplied and has no glyph" from "this face was
		// never supplied at all" — the first is a document fact the
		// engine may act on, the second is a supply fact it may not
		// guess about. cache.declares already computed the bit; this
		// return stops throwing it away.
		return false, true, nil
	}
	// Story 8.4: THIS is "something must actually draw from that
	// entry". An embedded entry whose asset is not a font this build
	// can read fails HERE, located, rather than at load (D-1.8.1 as
	// amended keeps load accepting it) and rather than never
	// (DW-83). An entry the chain never reaches for any rune is
	// never decoded and never complains.
	f, err := cache.get(name, fs)
	if err != nil {
		return false, false, err
	}
	return f.HasGlyph(r), false, nil
}

// resolveRuneFace: see the long comment above faceCovers, which is this
// function's — AC4's coverage walk, returning the INDEX of the first
// entry whose face draws r.
func resolveRuneFace(chain []string, r rune, fs FontSet, cache *fontCache) (index int, found bool, absent []string, err error) {
	// absentNames collects EVERY chain member the FontSet does not
	// supply, in chain order, and is returned only when coverage was NOT
	// located. A member that is absent while a LATER member draws the
	// rune is still skipped silently — the chain did its job, and
	// reporting the gap would fail a document that works (the format's
	// standing tolerance of an absent member, unchanged since AD-8).
	//
	// ⚠ EVERY ONE OF THEM, NOT THE FIRST. The refusal's whole purpose
	// is to say which face to supply, and the engine cannot know which of
	// several absent faces would have covered the rune — that is the
	// same ignorance the refusal exists to report. Naming only the first
	// sends an author to supply a face that may have nothing to do with
	// the script in question, and to be refused again.
	var absentNames []string
	for i, name := range chain {
		covers, gone, cerr := faceCovers(name, r, fs, cache)
		if cerr != nil {
			return 0, false, nil, cerr
		}
		if covers {
			return i, true, nil, nil
		}
		if gone {
			absentNames = append(absentNames, name)
		}
	}
	return 0, false, absentNames, nil
}

// substitutionMemo is the per-render answer to "what can this renderer
// paint with", computed once.
//
// WHY IT IS MEMOIZED AT ALL. Both consumers are inside loops: the pool
// is consulted once per UNCOVERED RUNE OCCURRENCE inside shapeSegments,
// and once per shaped element — which, for a table, is once per column
// PER ROW — from chainLineMetrics. Rebuilding and re-sorting it there
// makes a five-hundred-row table over a substituting chain pay the sort
// and the coverage walk five hundred times for an answer that cannot
// have changed.
//
// IT IS A PURE FUNCTION OF (embedded, fs), AND BOTH ARE FIXED FOR A
// RENDER. The embedded index is built at cache construction and never
// written again; the FontSet is the caller's map, which the engine only
// ever reads. So one answer per cache is one answer per render, and the
// memo cannot go stale — there is no mutation for it to miss.
type substitutionMemo struct {
	pool  []string
	built bool
	// painted is the per-rune answer: the face substituteFace chose, or
	// "" when the renderer holds nothing that draws the rune. A map,
	// because it is only ever LOOKED UP BY KEY and never ranged — AD-1
	// forbids map ITERATION where order can reach an output, and nothing
	// here iterates.
	painted map[rune]string
}

func newSubstitutionMemo() *substitutionMemo {
	return &substitutionMemo{painted: map[rune]string{}}
}

// substitutionPool is D2's candidate list: every face this renderer was
// actually GIVEN, in the one order the whole capability's determinism
// rests on.
//
// THE POOL IS WHAT THE RENDERER HOLDS, NEVER "THE SHIPPED SET". Package
// folio8 deliberately never imports folio-go/fonts — reversing that
// one-directional import would drag ~14.8 MB of faces into every
// consumer — so the ENGINE OWNS NO FACES and can only paint with what
// the caller handed it, plus what the document carries. A host passing
// fonts.Shipped() gets the eleven; a host passing nothing gets nothing
// to substitute from, and the render refuses exactly as it always did.
// This is what keeps AD-8's "pure lookup against the supplied FontSet,
// never a host font query" intact through a capability that looks, from
// the outside, like a font fallback.
//
// THE ORDER IS EMBEDDED FIRST, THEN SUPPLIED, EACH BY FACE NAME (D2).
// The two arms are fontCache.get's own two arms, in get's own
// precedence, reused rather than re-invented: an embedded face was
// chosen deliberately by the author of THIS document and is likelier to
// match their intent than a face the host happened to have lying about.
//
// ⚠ THE EMBEDDED ARM SORTS BY THE FACE NAME A PERSON READS, NOT BY THE
// MINTED ONE. A carried face's resolution name is "asset:" plus the
// asset key's 64 hex characters (AD-8/D-8.4.1: the key decides), so
// sorting those would order embedded candidates BY CONTENT HASH — which
// is deterministic but is not what D2 says, is not what any doc comment
// in three languages promises, and is not predictable to an author
// looking at their own document. The sort key is displayName, with the
// minted name as the tie-break so two carried faces declaring the same
// family still have one fixed order.
//
// A SLICE, BUILT BY SORTING, NEVER A RANGE OVER A MAP (AD-1): this
// order reaches the OUTPUT BYTES, so map iteration here would make two
// renders of one document on one machine disagree.
//
// Computed ONCE per render and memoized — see substitutionMemo.
func (c *fontCache) substitutionPool(fs FontSet) []string {
	if c.substitution.built {
		return c.substitution.pool
	}
	embedded := slices.Sorted(maps.Keys(c.embedded))
	slices.SortStableFunc(embedded, func(a, b string) int {
		as, _ := c.embedded.source(a)
		bs, _ := c.embedded.source(b)
		if d := strings.Compare(as.displayName(), bs.displayName()); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	c.substitution.pool = append(embedded, slices.Sorted(maps.Keys(fs))...)
	c.substitution.built = true
	return c.substitution.pool
}

// substituteFace is the coverage-resolved answer to "what else could
// draw this rune": the FIRST face in substitutionPool's order that
// carries a glyph for r, or ("", false) when the renderer holds nothing
// that does.
//
// PER RUNE, NEVER PER ELEMENT AND NEVER PER DOCUMENT. A fixed
// substitute — or the chain's own first entry — cannot be relied on to
// draw the script it replaced, and a fallback that renders tofu has
// substituted nothing. A Thai document whose brand face is missing must
// come out IN THAI against a pool that holds a Thai face.
//
// PER RUNE, AND ANSWERED ONCE PER RUNE. The walk is memoized on the
// cache, so the thousandth 'a' in a table costs a map lookup rather
// than a second walk of the pool.
//
// AN UNPARSEABLE CANDIDATE IS SKIPPED, NOT RAISED ON. The caller's own
// broken face is not this rune's fault, and the rune's real outcome —
// painted, or refused for want of any candidate — must not turn on a
// face the document never named. The error is not swallowed for good:
// anything that actually DRAWS with that face still fails through
// fontCache.get at the point of use, located.
func (c *fontCache) substituteFace(r rune, fs FontSet) (string, bool) {
	if name, memoized := c.substitution.painted[r]; memoized {
		return name, name != ""
	}
	painted := ""
	for _, name := range c.substitutionPool(fs) {
		f, err := c.get(name, fs)
		if err != nil {
			continue
		}
		if f.HasGlyph(r) {
			painted = name
			break
		}
	}
	c.substitution.painted[r] = painted
	return painted, painted != ""
}

// formatFontChain renders chain as AD-8's Rule names it for a human
// reader — "[Noto Sans, Noto Sans Thai]" — so a missing-glyph
// Diagnostic's message tells its reader not just what is wrong but
// what was actually searched (D-000.37: an actionable diagnostic names
// the chain, not only the rune).
//
// IT IS THE MESSAGE PATH, AND SINCE STORY 8.4 THAT IS A DISTINCTION
// WORTH MAKING. A carried face's render-path name is "asset:" plus the
// asset key's 64 hex characters, because AD-8/D-8.4.1 make the asset key
// the resolver. Printed verbatim in a diagnostic it reads as though the
// author mistyped a font name. So an embedded entry is spelled HERE by
// the display identity the asset itself carries
// (embeddedFaceSource.displayName) — and nowhere else: the reserved name
// is untouched everywhere it resolves a face, keys the fontCache, names a
// pagemodel.TextRun.Face or reaches a PDF resource dictionary.
//
// cache may be nil, and then every name is printed verbatim: a caller
// with no cache has no document behind the chain either, so there is no
// embedded entry in it to spell.
func formatFontChain(chain []string, cache *fontCache) string {
	out := make([]string, len(chain))
	for i, name := range chain {
		out[i] = faceDisplayName(name, cache)
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// faceDisplayName spells ONE face name the way a person reads it, and it
// is the rule formatFontChain has always applied, lifted out so a
// diagnostic naming a single face applies the identical rule rather than
// a second copy of it (Story 11.2's AC3 Warning names one face, not a
// chain).
//
// A carried face's render-path name is "asset:" plus 64 hex characters;
// printed verbatim in a diagnostic it reads as though the author
// mistyped a font name. cache may be nil, and then every name is printed
// verbatim — a caller with no cache has no document behind the chain
// either, so there is no embedded entry in it to spell.
func faceDisplayName(name string, cache *fontCache) string {
	if cache == nil {
		return name
	}
	if src, ok := cache.embedded.source(name); ok {
		return src.displayName()
	}
	return name
}

// missingGlyphMessage is the one construction site for FR41's fifth
// mode's Diagnostic message (Story 3.6, OPEN-1's ruling): the element
// id, the rune as BOTH its U+XXXX form and its literal character, and
// the exact chain that was searched — naming the chain is what turns
// "something is wrong" into "here is what to fix" (D-000.37).
func missingGlyphMessage(elementID string, r rune, chain []string, cache *fontCache) string {
	return fmt.Sprintf(
		"no face in chain %s covers %U (%c) in element %s — the rune is omitted from the rendered output (no glyph, no advance); "+
			"it is not substituted or drawn as a blank box (AD-8)",
		formatFontChain(chain, cache), r, r, elementID,
	)
}

// fontFamilyDataPath is the data path every font-chain refusal is
// located at: the style field fontChain resolves the chain from. It is a
// constant rather than a literal at each site so the two refusals this
// story codes — shapeSegments' and the vertical model's — cannot drift
// to two spellings of one field.
const fontFamilyDataPath = "style.fontFamily"

// faceAbsentMessage is the one construction site for
// spec-deferred-offline-cache CAP-7's refusal message.
//
// IT NAMES THE ABSENT FACE, WHERE missingGlyphMessage NAMES THE CHAIN,
// and the difference is the whole reason the two are separate. A chain
// every one of whose faces was supplied tells its reader to edit the
// chain; a chain with a face the CALLER never supplied tells its reader
// to supply that face, and only naming it says which.
//
// The first clause is deliberately the wording fontCache.get already
// uses for the same fact ("face %q is not present in the supplied
// FontSet") rather than a second phrasing of it — an author who hits the
// two conditions reads one sentence, not two that must be recognised as
// the same.
func faceAbsentMessage(elementID string, r rune, faces []string, chain []string, cache *fontCache) string {
	where := "element " + elementID
	if elementID == "" {
		where = "the document"
	}
	// EVERY ABSENT FACE IS NAMED, because the engine cannot know which of
	// them would have covered the rune. The singular reading is kept
	// byte-identical to fontCache.get's sentence for the common case of
	// one.
	quoted := make([]string, len(faces))
	for i, f := range faces {
		quoted[i] = fmt.Sprintf("%q", f)
	}
	subject := "face " + quoted[0] + " is"
	if len(quoted) > 1 {
		subject = "faces " + strings.Join(quoted, ", ") + " are"
	}
	return fmt.Sprintf(
		"%s not present in the supplied FontSet, and no present face in chain %s covers %U (%c) in %s — "+
			"the render is refused rather than omitting the rune, because whether an absent face would have covered it cannot be known here (AD-8)",
		subject, formatFontChain(chain, cache), r, r, where,
	)
}

// coalesceFaceDiags appends src to dst, dropping any per-(element,
// distinct rune) FACE Warning already recorded in seen and recording
// the ones it keeps.
//
// IT CARRIES TWO CODES: AC3's style-variant fallback
// (TEXT_STYLE_FACE_UNDECLARED) and this story's substitution
// (TEXT_FACE_SUBSTITUTED). Both state the same rule — one Warning per
// (element, distinct rune) — and both are safe through one memo because
// the identity compared is the WHOLE Diagnostic: two Diagnostics
// carrying different codes are never equal, so neither code can silence
// the other. It was named coalesceStyleFaceDiags while it carried one.
//
// WHY IT EXISTS: shapeSegments coalesces to one Diagnostic per (element,
// distinct rune) WITHIN ONE CALL, which is the whole story for a text
// element — it is shaped once. A TABLE shapes a column ONCE PER ROW, so
// a five-hundred-row table would report the same column's same rune five
// hundred times, and the rule the spec states is per (element, distinct
// rune), not per (element, rune, row). This is where the memo outlives
// the call.
//
// The identity compared is the WHOLE Diagnostic, not a key derived from
// it: the message is a pure function of (element, rune, face), so two
// equal Diagnostics ARE the same (element, distinct rune) and there is
// no key to get wrong. A SLICE with a linear scan, never a map — AD-1
// forbids map iteration where order can reach an output, and the
// population is the distinct fallback runes of one table's columns.
//
// ⚠ IT IS SCOPED TO THIS STORY'S CODE, AND THAT ASYMMETRY IS DELIBERATE
// AND REPORTED. TEXT_MISSING_GLYPH has the identical per-row duplication
// (measured: five rows of one uncovered rune produce five Warnings), and
// it is SHIPPED behaviour that predates this story. Widening this to
// cover it would change a shipped diagnostic's output, which is the
// adjacent-bug fix this story's scope fence rules out; it is registered
// instead.
func coalesceFaceDiags(dst, src []Diagnostic, seen *[]Diagnostic) []Diagnostic {
	for _, d := range src {
		if d.Code == DiagCodeTextStyleFaceUndeclared || d.Code == DiagCodeTextFaceSubstituted {
			already := false
			for _, s := range *seen {
				if s == d {
					already = true
					break
				}
			}
			if already {
				continue
			}
			*seen = append(*seen, d)
		}
		dst = append(dst, d)
	}
	return dst
}

// styleFaceUndeclaredMessage is the one construction site for Story
// 11.2's AC3 Warning: the element, the rune (as U+XXXX and as itself),
// and THE FACE THE RUNE WAS ACTUALLY DRAWN IN.
//
// It names one FACE where missingGlyphMessage names the whole chain, and
// the difference is the whole reason this is a separate code rather than
// a reuse of TEXT_MISSING_GLYPH: that one means the rune was DROPPED,
// this one means the rune was drawn at the WRONG WEIGHT. Naming the face
// is what tells the author which chain entry to give a variant to.
//
// It says the entry's own base face was used and that no bold or oblique
// was synthesized, because "nothing was drawn in the weight you asked
// for" is only actionable when the reader can see what WAS drawn and
// knows the engine did not invent a substitute (I-2).
func styleFaceUndeclaredMessage(elementID string, r rune, face string, cache *fontCache) string {
	return fmt.Sprintf(
		"the font chain entry covering %U (%c) in element %s declares no face for the requested weight and slope, "+
			"so the rune is drawn in that entry's own base face %s — no bold or oblique is synthesized, and no other entry "+
			"in the chain is substituted for it (FR57, AD-8)",
		r, r, elementID, faceDisplayName(face, cache),
	)
}

// faceSubstitutedMessage is the one construction site for
// DiagCodeTextFaceSubstituted's message, and it is shaped after
// styleFaceUndeclaredMessage above rather than invented: the element,
// the rune as BOTH its U+XXXX form and its literal character, and the
// face — one face, named, not the chain.
//
// IT NAMES FOUR THINGS BECAUSE THE SPEC REQUIRES FOUR: the element, the
// rune, THE FACE REQUESTED and THE FACE PAINTED. Naming only the
// painted face would tell a reader their page is wrong without telling
// them what to go and supply; naming only the requested one would not
// say what came out instead.
//
// THE REQUESTED FACES ARE THE ABSENT CHAIN MEMBERS, ALL OF THEM, for
// exactly faceAbsentMessage's reason: the engine cannot know which of
// several absent faces would have covered the rune, and naming one
// sends the author to supply a face that may have nothing to do with
// the script in question.
func faceSubstitutedMessage(elementID string, r rune, requested []string, painted string, chain []string, cache *fontCache) string {
	where := "element " + elementID
	if elementID == "" {
		where = "the document"
	}
	quoted := make([]string, len(requested))
	for i, f := range requested {
		quoted[i] = fmt.Sprintf("%q", f)
	}
	subject := "face " + quoted[0] + " is"
	if len(quoted) > 1 {
		subject = "faces " + strings.Join(quoted, ", ") + " are"
	}
	return fmt.Sprintf(
		"%s not present in the supplied FontSet, and no present face in chain %s covers %U (%c) in %s — "+
			"the rune is painted in %s instead, which is a face this renderer was given, because the caller asked for "+
			"FaceFallbackSubstitute; supply the named face to render the document as it was authored",
		subject, formatFontChain(chain, cache), r, r, where, faceDisplayName(painted, cache),
	)
}

// lineBreakHandling tells shapeSegments what its CALLER will do with a
// line feed in the text it is about to shape, and it exists because the
// answer decides whether an uncoverable U+000A is a defect worth
// reporting or the ordinary working of FR46.
//
// It is a parameter rather than a rule shapeSegments could work out for
// itself: this function sees a rune and a font chain, and nothing about
// whether the segments it returns are on their way to packLines.
type lineBreakHandling uint8

const (
	// breaksAreDrawn: the caller positions the shaped runes directly and
	// never packs them, so a line feed reaches the page as a rune no
	// face covers — dropped, silently, exactly the condition FR41's
	// fifth mode exists to report. Today one production caller is in
	// this class: a table COLUMN LABEL (table_render.go), which shapes
	// its text and hands the whole rune range to positionSegments.
	breaksAreDrawn lineBreakHandling = iota

	// breaksAreConsumed: the caller hands these segments to packLines,
	// which takes every mandatory break the text carries (Story 7.1), so
	// a line feed is absent from the drawn output BY DESIGN rather than
	// for want of a glyph.
	breaksAreConsumed
)

// shapeSegments performs Story 2.2's per-rune coverage resolution and
// Story 2.3's per-face-segment shaping, ONCE, and returns the result
// without positioning it.
//
// It is separated from positioning so that Story 2.4's line breaker
// measures and slices the SAME shaped glyphs that are ultimately drawn.
// Shaping once and slicing is not an optimisation — it is the
// correctness property: re-shaping a line's shorter text can
// legitimately produce different glyphs at the new boundary, and a
// second derivation of the same quantity is exactly what Story 2.3's
// Blocker 1 removed.
//
// Story 3.6 (divergence 6, OPEN-1 ruled): a rune covered by no face in
// chain is no longer an error. It is FR41's fifth mode — a Warning,
// never fatal — collected here in the SAME shape BindTextSpans already
// uses for its own non-error condition (a third return, never an
// error): elementID is now required, so the Diagnostic can carry AD-10's
// element id alongside the rune and the chain that was searched
// (D-000.37). Per OPEN-1's ruling, the render OMITS the rune entirely
// — no glyph, no advance, never `.notdef` and never a substituted
// replacement — but its slot in the ELEMENT-GLOBAL rune index space is
// preserved as an empty (zero-glyph) faceSegment, so runeStart/runeEnd
// bookkeeping downstream (packLines, measureRuneRange,
// positionSegments — all of which count rune positions against the
// ORIGINAL elementText, via totalRunes) never silently renumbers a
// later rune's position because an earlier one was dropped.
//
// STORY 11.2 ADDED THE styled SLICE, AND IT IS NOT A SECOND CHAIN. It is
// chainFaceNames' parallel list: same length, same order, "" where the
// entry declares no face for the requested weight and slope, and nil
// entirely when no variant was requested at all. COVERAGE STILL WALKS
// chain — never styled — and the variant is then applied WITHIN the
// entry coverage chose. Substituting into chain would let a narrow
// variant push a rune to the next entry and change its typeface to keep
// its weight, which is the substitution AD-8 forbids.
//
// THE ABSENCE ARM IS AC3's, AND IT IS EMITTED HERE FOR ONE REASON: this
// is the innermost function that holds an ELEMENT ID, so the Warning
// costs no plumbing, and a hidden element's diagnostics are already
// discarded upstream. Two conditions reach it and both are "absence":
// the entry declares no variant, and the declared variant does not cover
// this rune. Neither invents a second fallback policy — both draw the
// entry's OWN base face.
//
// PRECONDITION: styled is nil, or len(styled) == len(chain) with the
// same order, so styled[i] is entry i's declared variant. chainFaceNames
// is the pair's only producer and builds them together; the alignment is
// STATED rather than guarded, and pinned at the producer by
// TestTheTwoChainSlicesAreAligned.
func shapeSegments(elementID string, chain, styled []string, elementText string, fs FontSet, cache *fontCache, breaks lineBreakHandling) ([]faceSegment, []Diagnostic, error) {
	type segment struct {
		face    string
		runes   []rune
		missing bool // true: no face in chain covers these rune(s) (OPEN-1)
	}
	var segments []segment
	var diags []Diagnostic
	// seenMissingRunes is D-3.7.3's engine-side coalescing (OVERRULING
	// the creator's presentation-layer recommendation, AC7): one
	// Diagnostic per (element, distinct rune), never one per
	// occurrence. A SLICE with a linear scan, deliberately never a map
	// — AD-1 forbids map iteration where order can reach an output, and
	// D-2.8.6 made the diagnostics slice's order a determinism
	// guarantee. The population is the distinct uncovered runes in ONE
	// element's text, which is tiny, so the linear scan costs nothing
	// that matters. First-occurrence position determines order.
	var seenMissingRunes []rune
	// seenStyleFallbackRunes is AC3's coalescing, in the identical shape
	// and for the identical reason — a separate slice because the two
	// conditions are different Diagnostics about different runes, and one
	// shared slice would let a dropped rune silence a mis-weighted one.
	var seenStyleFallbackRunes []rune
	// seenSubstitutedRunes is this story's coalescing, a third slice for
	// the third condition and for the identical reason the second one is
	// separate: a substituted rune and a mis-weighted one are different
	// Diagnostics about different facts, and one shared slice would let
	// either silence the other.
	var seenSubstitutedRunes []rune
	for _, r := range elementText {
		index, found, absentFaces, err := resolveRuneFace(chain, r, fs, cache)
		if err != nil {
			return nil, nil, err
		}
		var face string
		if found {
			face = chain[index]
			if styled != nil {
				variant := styled[index]
				if variant != "" {
					// THE VARIANT MAY NOT COVER THE RUNE. Coverage chose
					// this entry on its BASE face, so the declared
					// variant is not guaranteed to carry the glyph.
					// Treat that as absence too: the entry's own base
					// face, and the same Warning — one policy on every
					// path, rather than a second fallback invented for
					// this case.
					// THE ABSENCE OF A STYLED VARIANT IS NOT CAP-7's
					// REFUSAL, and the discarded second return is that
					// rule written down. The entry that COVERS the rune
					// is present and draws it, at the wrong weight —
					// Story 11.2's condition, which has its own Warning.
					// Refusing here would fail every document that asks
					// for bold from a chain declaring no bold face.
					covers, _, cerr := faceCovers(variant, r, fs, cache)
					if cerr != nil {
						return nil, nil, cerr
					}
					if covers {
						face = variant
					} else {
						variant = ""
					}
				}
				if variant == "" {
					alreadySeen := false
					for _, sr := range seenStyleFallbackRunes {
						if sr == r {
							alreadySeen = true
							break
						}
					}
					if !alreadySeen {
						seenStyleFallbackRunes = append(seenStyleFallbackRunes, r)
						diags = append(diags, Diagnostic{
							Severity:  SeverityWarning,
							Code:      DiagCodeTextStyleFaceUndeclared,
							ElementID: elementID,
							Message:   styleFaceUndeclaredMessage(elementID, r, face, cache),
						})
					}
				}
			}
		}
		if !found {
			// A LINE FEED IS NOT A COVERAGE FAILURE — ON A CALLER
			// THAT CONSUMES IT (Story 7.1). No face covers U+000A and
			// none is expected to; where the caller hands these
			// segments to packLines, the breaker takes it as a
			// mandatory break, so it is absent from the drawn output
			// by design rather than for want of a glyph. FR41's fifth
			// mode reports a rune the document asked to be DRAWN and
			// the chain could not draw; reporting one the engine
			// deliberately consumed would say "your font chain is
			// incomplete" about a character no font has ever carried,
			// and would fire on every document holding a paragraph
			// break.
			//
			// THE PREMISE IS THE CALLER'S, NOT THIS FUNCTION'S, WHICH
			// IS WHY IT ARRIVES AS A PARAMETER. A table column label
			// is shaped here and positioned directly, never packed, so
			// a line feed in a LABEL really is dropped — and that path
			// keeps its Warning, which is the only signal it has.
			//
			// SCOPED TO U+000A, AND ONLY THE DIAGNOSTIC. The
			// segmentation below is untouched on every caller, so the
			// rune still claims its slot in the element-global rune
			// index space — which is exactly what keeps internal/text's
			// break positions meaningful. And a lone carriage return
			// gains no meaning in this story (it stays an ordinary
			// optional whitespace break), so nothing about it changes.
			if r != '\n' || breaks == breaksAreDrawn {
				// THE FORK (spec-deferred-offline-cache CAP-7), stated
				// once:
				//
				//   uncovered + a chain member was ABSENT -> refuse,
				//                                            naming it
				//   uncovered + every member PRESENT      -> the
				//                                            Warning,
				//                                            unchanged
				//
				// The second arm is FR41's fifth mode and keeps every
				// byte of its behaviour: the rune is dropped, the render
				// completes, a PDF ships. The first arm cannot be
				// folded into it, because the engine cannot ask whether
				// the face it was never given would have drawn the rune
				// — so it refuses instead of guessing that it would not.
				//
				// IT SITS INSIDE THE NEWLINE GUARD DELIBERATELY. A line
				// feed a caller consumes is absent from the output by
				// design, not for want of a glyph, so an absent chain
				// member must not turn a paragraph break into a refusal.
				//
				// AND NEVER ON A CONTROL CHARACTER. No font has ever
				// carried U+0009 or its neighbours, so the refusal's own
				// justification — that the absent face MIGHT have covered
				// this rune — is false for them: they are absent from the
				// drawn output by design, exactly as U+000A is. The
				// WARNING's rule is untouched, control character or not,
				// which is why this is a second condition here rather
				// than a widening of the guard above.
				if len(absentFaces) > 0 && !unicode.IsControl(r) {
					// THE LENIENT ARM, AND ITS GUARD IS THE
					// REFUSAL'S — THE SAME `if`, not a second
					// condition that resembles it. It fires exactly
					// where TEXT_FACE_ABSENT would have fired and
					// nowhere else, which is what keeps
					// TEXT_MISSING_GLYPH — a chain every member of
					// which WAS supplied — untouched under either
					// selector: that condition never reaches this
					// block at all.
					//
					// THE THREE OUTCOMES ARE ONE RULE. Strict
					// refuses. Lenient with a covering candidate
					// paints and warns. Lenient with NO candidate
					// falls through to the same refusal strict
					// takes, because a renderer holding nothing
					// that draws the rune has nothing to
					// substitute — the selector asks for a
					// substitute, not for the rune to be dropped.
					if cache.fallback == FaceFallbackSubstitute {
						if painted, ok := cache.substituteFace(r, fs); ok {
							alreadySeen := false
							for _, sr := range seenSubstitutedRunes {
								if sr == r {
									alreadySeen = true
									break
								}
							}
							if !alreadySeen {
								seenSubstitutedRunes = append(seenSubstitutedRunes, r)
								diags = append(diags, Diagnostic{
									Severity:  SeverityWarning,
									Code:      DiagCodeTextFaceSubstituted,
									ElementID: elementID,
									Message:   faceSubstitutedMessage(elementID, r, absentFaces, painted, chain, cache),
								})
							}
							// AND THE REQUESTED WEIGHT IS GONE
							// TOO, SO IT GOES ON THE RECORD.
							// styled != nil means this element
							// asked for bold, italic or both, and
							// the styled list is indexed by CHAIN
							// ENTRY — a pool face is not a chain
							// entry, so there is no declared
							// variant for it and none may be
							// inferred (FR57: a face name is never
							// guessed at). That is exactly Story
							// 11.2's absence condition one level
							// out, so it reuses that condition's
							// own Warning rather than inventing a
							// second one: the rune is drawn in the
							// substitute's OWN base face, at the
							// wrong weight, and a reader who saw
							// only the substitution Warning would
							// be told the typeface changed and not
							// that the weight was dropped.
							if styled != nil {
								alreadyStyled := false
								for _, sr := range seenStyleFallbackRunes {
									if sr == r {
										alreadyStyled = true
										break
									}
								}
								if !alreadyStyled {
									seenStyleFallbackRunes = append(seenStyleFallbackRunes, r)
									diags = append(diags, Diagnostic{
										Severity:  SeverityWarning,
										Code:      DiagCodeTextStyleFaceUndeclared,
										ElementID: elementID,
										Message:   styleFaceUndeclaredMessage(elementID, r, painted, cache),
									})
								}
							}
							// THE SEGMENT IS AN ORDINARY ONE. A
							// substituted rune is DRAWN, so it
							// joins the run of whatever face draws
							// it exactly as a covered rune does —
							// there is no third segment kind and
							// nothing downstream learns that this
							// face came from the pool rather than
							// from the chain.
							if n := len(segments); n > 0 && !segments[n-1].missing && segments[n-1].face == painted {
								segments[n-1].runes = append(segments[n-1].runes, r)
							} else {
								segments = append(segments, segment{face: painted, runes: []rune{r}})
							}
							continue
						}
					}
					return nil, nil, newRenderError(
						DiagCodeTextFaceAbsent, elementID, fontFamilyDataPath,
						errors.New(faceAbsentMessage(elementID, r, absentFaces, chain, cache)),
					)
				}
				alreadySeen := false
				for _, sr := range seenMissingRunes {
					if sr == r {
						alreadySeen = true
						break
					}
				}
				if !alreadySeen {
					seenMissingRunes = append(seenMissingRunes, r)
					diags = append(diags, Diagnostic{
						Severity:  SeverityWarning,
						Code:      DiagCodeTextMissingGlyph,
						ElementID: elementID,
						Message:   missingGlyphMessage(elementID, r, chain, cache),
					})
				}
			}
			if n := len(segments); n > 0 && segments[n-1].missing {
				segments[n-1].runes = append(segments[n-1].runes, r)
				continue
			}
			segments = append(segments, segment{missing: true, runes: []rune{r}})
			continue
		}
		if n := len(segments); n > 0 && !segments[n-1].missing && segments[n-1].face == face {
			segments[n-1].runes = append(segments[n-1].runes, r)
			continue
		}
		segments = append(segments, segment{face: face, runes: []rune{r}})
	}

	out := make([]faceSegment, 0, len(segments))
	runeStart := 0
	for _, seg := range segments {
		if seg.missing {
			// OPEN-1: no glyph, no advance. An empty faceSegment still
			// claims this rune's slot in the element-global index
			// space (runeStart/runeEnd), so it is skipped, not erased,
			// by every downstream consumer — glyphRangeForRunes
			// naturally returns an empty range for it (no glyphs to
			// find), so measureRuneRange contributes zero width and
			// positionSegments emits no run.
			out = append(out, faceSegment{
				runeStart: runeStart,
				runeEnd:   runeStart + len(seg.runes),
			})
			runeStart += len(seg.runes)
			continue
		}

		f, err := cache.get(seg.face, fs)
		if err != nil {
			return nil, nil, err
		}

		// AC1/AC8: one buffer per FACE-SEGMENT, never per element and
		// never per document — GuessSegmentProperties derives script and
		// direction from the buffer's own contents, so a mixed-script
		// string would let one script's rules govern another's runes.
		// The segmentation above is Story 2.2's, unchanged.
		segText := string(seg.runes)
		glyphs, serr := f.Shaper().Shape(segText)
		if serr != nil {
			return nil, nil, serr
		}
		texts, terr := text.ClusterTexts(segText, glyphs)
		if terr != nil {
			return nil, nil, fmt.Errorf("face %q: %w", seg.face, terr)
		}

		out = append(out, faceSegment{
			face:         seg.face,
			segText:      segText,
			runeStart:    runeStart,
			runeEnd:      runeStart + len(seg.runes),
			glyphs:       glyphs,
			clusterTexts: texts,
			unitsPerEm:   int64(f.UnitsPerEm()),
		})
		runeStart += len(seg.runes)
	}
	return out, diags, nil
}

// positionSegments turns the element-global rune range [from, to) of a
// shaped element into drawable runs, laying them out left to right from
// x at baseline y.
//
// Positioning: only the FIRST sub-run keeps x; each later sub-run's x is
// the previous one's x plus that sub-run's SHAPED total advance, scaled
// by fontSize. The cursor is advanced by measureRuneRange over exactly
// the glyphs the run carries — the same function the line breaker
// decides with — so a run's drawn width and its contribution to the
// cursor cannot disagree.
// slots is Story 2.7's addition: element-global rune spans of a
// {{page}} reservation within [from,to). Empty for every caller but the
// page-header/page-footer collection path, in which case this function
// is byte-for-byte what it was before this story — no extra allocation,
// no extra comparison beyond the nil check on the outer loop.
//
// A run's pageSlots is a SLICE (this story's review, Blocker 1): more
// than one {{page}} occurrence can land in one face segment on one
// line — "Page {{page}} of {{pages}} / {{page}}" is entirely ASCII, so
// it is a single run, and each matching slot is APPENDED, never
// overwritten.
//
// A slot that would straddle two face segments or two lines cannot be
// expressed as one contiguous glyph range and is therefore a located
// error naming the run's rune range, not a panic — this story's review,
// Finding 10: everywhere else on this path a structural impossibility
// is a located error (D-2.6.5's precedent; digitTableRun and
// buildPageNumberSlot in page_number.go both return one), and a public
// entry point (Render) must not let an internal panic cross it
// uncaught. Unreachable through the shipped set, where the construct is
// entirely ASCII (finding 5, story creation), but checked rather than
// assumed.
func positionSegments(segs []faceSegment, from, to int, x, y, fontSize, baselineOffset geom.Length, slots []pageSlotSpan) ([]textRunSource, error) {
	runs := make([]textRunSource, 0, len(segs))
	cursor := x
	for _, s := range segs {
		if to <= s.runeStart || from >= s.runeEnd {
			continue
		}
		elemLo := maxInt(from, s.runeStart)
		elemHi := minInt(to, s.runeEnd)
		lo, hi := s.glyphRangeForRunes(elemLo, elemHi)
		if hi <= lo {
			continue
		}
		runeLo := elemLo - s.runeStart
		runeHi := elemHi - s.runeStart
		run := textRunSource{
			face:           s.face,
			text:           string([]rune(s.segText)[runeLo:runeHi]),
			x:              cursor,
			y:              y,
			fontSize:       fontSize,
			baselineOffset: baselineOffset,
			glyphs:         s.glyphs[lo:hi],
			clusterTexts:   s.clusterTexts[lo:hi],
		}
		for _, sl := range slots {
			if sl.to <= elemLo || sl.from >= elemHi {
				continue // no overlap with this segment's contribution to this line
			}
			if sl.from < elemLo || sl.to > elemHi {
				return nil, fmt.Errorf(
					"folio8: Render: internal error: a {{page}} reservation [%d,%d) straddles a "+
						"face-segment, line or justified-piece boundary at [%d,%d) — Story 2.7 requires "+
						"the construct to resolve to one face segment on one line, and Story 7.3 draws a "+
						"justified line as several pieces positioned separately, so a piece boundary is a "+
						"run boundary too",
					sl.from, sl.to, elemLo, elemHi,
				)
			}
			slotLo, slotHi := s.glyphRangeForRunes(sl.from, sl.to)
			run.pageSlots = append(run.pageSlots, textRunPageSlot{
				glyphLo: slotLo - lo,
				glyphHi: slotHi - lo,
				digitsY: sl.digitsY,
			})
		}
		runs = append(runs, run)
		cursor += geom.ScaleRound(geom.Length(s.advance1000(lo, hi)), int64(fontSize), 1000)
	}
	return runs, nil
}

// renderDocument is Render's implementation once t is known non-nil
// (AC14b). It resolves every text element's face, subsets each distinct
// face EXACTLY ONCE over the union of runes the whole document uses
// (AC9), and hands the result to internal/pdf.SerializeTextDocument.
//
// ERROR ORDERING ON A MULTI-DEFECT DOCUMENT (this story's review,
// Finding 9). This story's two-phase restructure — geometry, then
// images, then content-band text (D-2.7.3's fence), then
// layout.Paginate (D-2.6.5's OverflowError), then header/footer text
// (D-2.7.2's reservation) — changed WHICH located error a document with
// MORE THAN ONE simultaneous defect reports, in four ways relative to
// the pre-2.7 single-pass collection order. This is NOT a contractual
// promise: no AC and no golden fixes an ordering among unrelated,
// simultaneous defects, and none of the eight pre-existing goldens
// (single-outcome inputs) can observe it. It is recorded here, as this
// story's review asked, so a future restructure changes it
// KNOWINGLY rather than as an unnoticed by-product of where a phase
// boundary happens to fall — not so a caller can depend on it.
func renderDocument(t *Template, data, params bind.Value, fs FontSet, fallback ...FaceFallback) ([]byte, []Diagnostic, error) {
	date, derr := resolveDocumentDate(params)
	if derr != nil {
		return nil, nil, derr
	}
	pages, embedded, pdfImages, diags, err := buildPageModel(t, data, params, fs, fallback...)
	if err != nil {
		return nil, nil, err
	}
	b, serr := pdf.SerializeTextDocument(pages, embedded, pdfImages, date)
	if serr != nil {
		return nil, nil, serr
	}
	return b, diags, nil
}

// resolveDocumentDate is D-3.7.2's reserved params key, "documentDate":
// an RFC 3339 timestamp string that, when present, sets BOTH
// /CreationDate and /ModDate to the same value (R4: it rides into
// internal/pdf as a plain VALUE, never as an import — internal/pdf
// never parses a date string or imports internal/expr). Absent from
// params (or params carrying no "documentDate" key at all — the
// ordinary case for every caller before this story and for every
// caller after it that never supplies one) returns (nil, nil): no
// /Info dictionary is emitted at all (AC11), not a defaulted or
// present-but-empty one.
//
// A present value that is not a string, or a string that is not a
// valid RFC 3339 timestamp, is a located Error (AC10) — the concrete
// case D-3.7.1's four-argument Validate exists to catch before
// production, consistent with Story 3.4's formatDate rule for the same
// malformed-timestamp shape.
func resolveDocumentDate(params bind.Value) (*pdf.DocumentDate, error) {
	if params.Kind != bind.KindObject {
		return nil, nil
	}
	v, ok := params.Obj[documentDateParamKey]
	if !ok {
		return nil, nil
	}
	if v.Kind != bind.KindString {
		return nil, newRenderError(
			DiagCodeDocumentDateInvalid, "", "",
			fmt.Errorf("folio8: params.%s must be an RFC 3339 timestamp string, got %s", documentDateParamKey, v.Kind),
		)
	}
	civil, err := expr.ParseRFC3339(v.Str)
	if err != nil {
		return nil, newRenderError(
			DiagCodeDocumentDateInvalid, "", "",
			fmt.Errorf("folio8: params.%s: %w", documentDateParamKey, err),
		)
	}
	return &pdf.DocumentDate{
		Year: civil.Year, Month: civil.Month, Day: civil.Day,
		Hour: civil.Hour, Minute: civil.Minute, Second: civil.Second,
		OffsetMinutes: civil.OffsetMinutes,
	}, nil
}

// documentDateParamKey is D-3.7.2's reserved top-level params key.
// "reportDate" is deliberately NOT used: Story 6.3's AC already spends
// it as the author's own worked example. This spelling is public
// contract, frozen at folio-go/v1.0.0 alongside the API signatures
// (AD-22) — the params namespace now has one reserved name in it.
const documentDateParamKey = "documentDate"

// buildPageModel is renderDocument's body up to, but not including, PDF
// serialization — a one-line wrapper over predictDocument (below),
// which is the SAME derivation renderDocument uses, not a second one
// (D-000.42). Split into its own name at Story 3.7 (D-3.7.1, AC1) so
// that folio8.Validate can call predictDocument directly and this
// package's own AST guard (TestValidateNeverReachesRenderOrInternalPDF,
// render_arch_test.go) can assert, by name, that Validate's call graph
// never reaches buildPageModel OR renderDocument OR internal/pdf: the
// three ways this module's render pipeline can ever produce document
// bytes.
func buildPageModel(t *Template, data, params bind.Value, fs FontSet, fallback ...FaceFallback) ([]pagemodel.Page, map[string]pdf.EmbeddedFace, map[string]pdf.ImageXObject, []Diagnostic, error) {
	return predictDocument(t, data, params, fs, fallback...)
}

// predictDocument is buildPageModel's actual body (AC1, Story 3.5's
// original split point, renamed at Story 3.7): it derives everything a
// render needs UP TO AND INCLUDING page composition — collecting bands,
// checking table bindings, resolving visibility, collecting and shaping
// text and image runs, subsetting fonts, decoding and validating image
// assets, and paginating — every step of Render's pipeline that can
// produce a located error or Diagnostic. It makes ZERO calls into
// internal/pdf (only pdf.EmbeddedFace/pdf.ImageXObject struct literals,
// package-local data shapes, never a call through the pdf import
// alias) — folio8.Validate (D-3.7.1) calls this function directly,
// discarding the page model/embedded-face/image-XObject values and
// keeping only diags and err, which is exactly what makes "Validate
// predicts Render" (D-3.7.1) true by construction rather than by two
// independently-maintained implementations agreeing by coincidence
// (D-000.42).
func predictDocument(t *Template, data, params bind.Value, fs FontSet, fallback ...FaceFallback) ([]pagemodel.Page, map[string]pdf.EmbeddedFace, map[string]pdf.ImageXObject, []Diagnostic, error) {
	// THE VARIADIC BECOMES A VALUE HERE, THROUGH THE SAME DOOR THE
	// PUBLIC PATH USES. An internal seam that took fallback[len-1] and
	// validated nothing would silently clamp what Render refuses, and
	// the next internal caller would diverge from the public contract
	// without anything saying so.
	mode, ferr := resolveFaceFallback(fallback)
	if ferr != nil {
		return nil, nil, nil, nil, ferr
	}
	// cache is shared between collection (coverage checks, AC4) and
	// embedding (subsetting) below, so a face is ever parsed at most
	// once per render regardless of how many chain members or runes
	// consult it. Only ever looked up by key, never ranged.
	//
	// It is built FROM THE DOCUMENT (Story 8.4) so that a chain entry
	// naming a face the document CARRIES resolves to that face's own
	// bytes. This is the site that makes AC4's Validate half true by
	// construction: folio8.Validate calls predictDocument directly, so
	// every check reached from here is a check Validate reaches too —
	// there is no second rule system to keep in step.
	cache := newDocumentFontCache(t, mode)

	bands, bandsErr := documentBands(t)
	if bandsErr != nil {
		return nil, nil, nil, nil, bandsErr
	}
	// Story 3.1, AC5 / D-3.1.1: checked here, before any font work
	// (finding 8) — a not-a-list collection bind or a colliding row
	// alias is a located error, never a plausible-looking document
	// with the table silently missing.
	if terr := checkTableBindings(bands, data); terr != nil {
		return nil, nil, nil, nil, terr
	}
	geometry, gerr := pageGeometryOf(t)
	if gerr != nil {
		return nil, nil, nil, nil, gerr
	}

	// Story 3.5 (R1/AC9): every element's visibility verdict, decided
	// ONCE, before any collection pass, from the data/params scope
	// alone — see computeVisibility's own doc comment (render_visibility.go)
	// for why this must happen here and not per-band or per-phase.
	fc := expr.NewFormatContext(t.doc.Locale, t.doc.UTCOffset)
	visible, conditionDiags, verr := computeVisibility(bands, data, params, fc)
	if verr != nil {
		return nil, nil, nil, nil, verr
	}

	imageRuns, ierr := collectImageRuns(t)
	if ierr != nil {
		return nil, nil, nil, nil, ierr
	}

	// Story 4.1: table header runs/rects, one call per band, BEFORE
	// PHASE A — a table's column labels are plain strings (never
	// {{page}}/{{pages}}-bearing), so collection needs no resolver and
	// no pageCount, and can run once, ahead of the phase split PHASE
	// A/B exists for. Order: header, content, footer — documentBands'
	// own authored order — matching how PHASE B below appends its own
	// three bands' text.
	headerTableRuns, headerTableRects, headerTableDiags, htterr := collectBandTableRuns(t, bands, pageHeaderBandIndex, data, params, fc, fs, cache, visible)
	if htterr != nil {
		return nil, nil, nil, nil, htterr
	}
	contentTableRuns, contentTableRects, contentTableDiags, ctterr := collectBandTableRuns(t, bands, contentBandIndex, data, params, fc, fs, cache, visible)
	if ctterr != nil {
		return nil, nil, nil, nil, ctterr
	}
	footerTableRuns, footerTableRects, footerTableDiags, ftterr := collectBandTableRuns(t, bands, pageFooterBandIndex, data, params, fc, fs, cache, visible)
	if ftterr != nil {
		return nil, nil, nil, nil, ftterr
	}
	// Story 9.1: element boxes FIRST, then the tables' own chrome. Both
	// populations travel in one slice because both are rect groups with a
	// band and an extent, and everything downstream — contentColumnItems'
	// page-count pass, paginateDocument's placement, the page assembler's
	// header/footer repetition — already reads exactly that. Element boxes
	// lead so a box painted behind a table sits UNDER that table's cell
	// chrome; within each population the order is documentBands' band
	// order and then declaration order, which is the emitted byte order.
	// An element declaring neither background nor border contributes
	// nothing here, which is what leaves the corpus byte-identical.
	elementBoxes, eberr := collectElementBoxRects(bands, visible)
	if eberr != nil {
		return nil, nil, nil, nil, eberr
	}
	// spec-barcode-qr-elements: the code elements' rects (barcode bars and
	// qrcode module runs) follow element boxes and precede table chrome. A
	// document with no barcode or qrcode contributes nothing here.
	barcodeRects, barcodeDiags, bcerr := collectBarcodeRects(t, bands, data, params, visible)
	if bcerr != nil {
		return nil, nil, nil, nil, bcerr
	}
	var tableRects []tableRectSource
	tableRects = append(tableRects, elementBoxes...)
	tableRects = append(tableRects, barcodeRects...)
	tableRects = append(tableRects, headerTableRects...)
	tableRects = append(tableRects, contentTableRects...)
	tableRects = append(tableRects, footerTableRects...)

	// Story 2.7, PHASE A: the content band ALONE, under D-2.7.3's fence
	// (contentBandResolver errors on {{page}}/{{pages}} rather than
	// passing them through). This is the ONLY input layout.Paginate
	// needs — internal/layout/band.go's ContentHeight takes page
	// geometry alone (finding 2, story creation) — so it is legal to
	// learn Y here, BEFORE the page-header/page-footer text exists,
	// without becoming pass two's job: this is still pass one, just
	// reordered within it.
	contentRuns, _, contentDiags, cterr := collectBandTextRuns(t, bands, contentBandIndex, data, params, fs, cache, contentBandResolver, visible)
	if cterr != nil {
		return nil, nil, nil, nil, cterr
	}
	// Table label runs are appended AFTER this band's own text runs
	// (this story's own, stated D-2.8.6 deviation — see the Delivery
	// Log's "D-2.8.6 deviation: table diagnostics after text
	// diagnostics within a band" entry — from strict
	// element-declaration order for the rare case a table's label
	// carries a missing-glyph Warning and a text element in the SAME
	// band also does; the ACTUAL page-model content is unaffected,
	// only Result.Diagnostics' relative order between the two kinds).
	contentRuns = append(contentRuns, contentTableRuns...)
	// Story 7.7 (FR51): the document's own keep-together declarations,
	// read ONCE here and handed to BOTH pagination passes — PHASE A
	// below and paginateDocument's PHASE B — because a grouping seen by
	// only one of them would make the page COUNT disagree with the
	// render.
	keepTogether := keepTogetherTags(t)
	// spec-section-break: both passes paginate through the same
	// section-aware wrapper, so a page added for the section is counted by
	// {{pages}} exactly as it is rendered.
	// SPEC-multi-pages: each designed page is paginated on its own and the
	// output pages summed, in both passes.
	contentPages := contentPagesOf(t, geometry)
	contentItems := contentColumnItems(contentRuns, imageRuns, tableRects, visible, keepTogether)
	contentPlan, _, plerr := paginateContentPages(geometry, contentItems, contentPages)
	if plerr != nil {
		return nil, nil, nil, nil, wrapOverflowError(plerr)
	}
	pageCount := len(contentPlan.Pages)

	// PHASE B: the two repeated bands, now that D-2.7.2's reservation
	// (digits(Y)) is computable. Collected in documentBands' authored
	// order — header, then footer — and combined below in that same
	// order relative to content (header, content, footer) so a
	// document using NO page slot produces the identical run sequence,
	// and therefore the identical CID allocation order
	// (buildShapedPDFRuns is order-sensitive by design, AC7/D-2.3-Q1),
	// this story's own change produced for every pre-existing document.
	headerRuns, headerPending, headerDiags, herr := collectBandTextRuns(t, bands, pageHeaderBandIndex, data, params, fs, cache, headerFooterResolver(pageCount), visible)
	if herr != nil {
		return nil, nil, nil, nil, herr
	}
	headerRuns = append(headerRuns, headerTableRuns...)
	footerRuns, footerPending, footerDiags, ferr := collectBandTextRuns(t, bands, pageFooterBandIndex, data, params, fs, cache, headerFooterResolver(pageCount), visible)
	if ferr != nil {
		return nil, nil, nil, nil, ferr
	}
	footerRuns = append(footerRuns, footerTableRuns...)

	// Story 2.8, D-2.8.6: Result.Diagnostics is DOCUMENT ORDER — band
	// order, then element declaration order within a band — never map
	// order (there is no map here) and never collection order (PHASE A
	// collects content BEFORE header/footer above, but band order is
	// header, content, footer, so this concatenation is NOT the order
	// the three collectBandTextRuns calls above happened to run in).
	// Each *Diags slice is already in element-declaration order within
	// its own band (collectBandTextRuns' own doc comment on `diags`).
	var diags []Diagnostic
	textWarnings := [][]Diagnostic{headerDiags, contentDiags, footerDiags}
	tableWarnings := [][]Diagnostic{headerTableDiags, contentTableDiags, footerTableDiags}
	for i, band := range bands {
		diags = append(diags, mergeConditionDiagnostics(band.band.Elements, conditionDiags, textWarnings[i])...)
		// Keep the existing table-warning exception after this band's text warnings.
		diags = append(diags, tableWarnings[i]...)
		diags = append(diags, barcodeDiags[i]...)
	}
	// spec-section-break: a keepTogether group split by the break, on every
	// render. Empty for a document without a break.
	diags = append(diags, sectionBreakSplitDiagnostics(t)...)

	headerOffset := 0
	contentOffset := len(headerRuns)
	footerOffset := contentOffset + len(contentRuns)

	runs := make([]textRunSource, 0, len(headerRuns)+len(contentRuns)+len(footerRuns))
	runs = append(runs, headerRuns...)
	runs = append(runs, contentRuns...)
	runs = append(runs, footerRuns...)

	var pending []pendingPageSlot
	for _, p := range headerPending {
		p.runIndex += headerOffset
		p.digitTableIndex += headerOffset
		pending = append(pending, p)
	}
	for _, p := range footerPending {
		p.runIndex += footerOffset
		p.digitTableIndex += footerOffset
		pending = append(pending, p)
	}

	// Story 2.3, AC1/AC8: each face-segment is shaped ONCE, with its own
	// buffer, by shapeSegments; positionSegments then derives the
	// segment cursor from those same shaped glyphs. So there is exactly
	// ONE shaping answer per segment and the drawn glyphs and the next
	// segment's origin are derived from it. Re-shaping here would
	// reintroduce the second derivation Blocker 1 was; these two slices
	// are views onto what shapeSegments already produced, built by
	// ranging a SLICE (D-1.3.5).
	shapedRuns := make([][]text.ShapedGlyph, len(runs))
	clusterTexts := make([][]string, len(runs))
	for i, r := range runs {
		shapedRuns[i] = r.glyphs
		clusterTexts[i] = r.clusterTexts
	}

	// Union of SHAPED GLYPH IDS per face, across the WHOLE document
	// (AC9, AC5) — built by ranging `runs` (a slice), never a map.
	//
	// This is the change AC5 is about: the subset input is the set of
	// glyphs the renderer actually draws, not the set of runes the
	// author typed. They are measurably different — shaping "office"
	// draws the `ffi` ligature, which no rune maps to — and building
	// the subset from the runes would leave the drawn glyph
	// unaddressable (D-1.5.8, one level up).
	glyphsByFace := map[string][]uint16{}
	for i, r := range runs {
		for _, g := range shapedRuns[i] {
			glyphsByFace[r.face] = append(glyphsByFace[r.face], g.GlyphID)
		}
	}

	faceNames := slices.Sorted(maps.Keys(glyphsByFace)) // ScanMapRange-compliant: sorted, deterministic object order.

	subsets := make(map[string]*fontset.Subset, len(faceNames))
	embedded := make(map[string]pdf.EmbeddedFace, len(faceNames))
	for _, name := range faceNames {
		font, ferr := cache.get(name, fs)
		if ferr != nil {
			if cache.isEmbedded(name) {
				// A face the DOCUMENT carries is never "missing from the
				// FontSet" — it was never looked for there. Its own error
				// already names the chain, the entry and the asset key.
				// (Unreachable in practice: a name only reaches this loop
				// by having already been parsed to shape a glyph.)
				return nil, nil, nil, nil, fmt.Errorf("folio8: Render: %w", ferr)
			}
			return nil, nil, nil, nil, fmt.Errorf("folio8: Render: face %q was resolved from a fallback chain but is missing from the FontSet: %w", name, ferr)
		}
		// ONE subsetting call per font per document (AC9), over the
		// union of shaped glyph ids collected above.
		sub, serr := font.Subset(glyphsByFace[name])
		if serr != nil {
			return nil, nil, nil, nil, fmt.Errorf("folio8: Render: %w", serr)
		}
		subsets[name] = sub
		metrics := font.Metrics()
		created, modified := font.HeadTimes()
		embedded[name] = pdf.EmbeddedFace{
			Name: name,
			// The face's OWN identity, read off the supplied font
			// program's `name` table — not this map key (ISO 32000-1
			// Table 117; see internal/pdf's baseFont comment).
			PostScriptName: font.PostScriptName(),
			Program:        sub.Program,
			Tag:            sub.Tag,
			NumGlyphs:      sub.NumGlyphs,
			WidthForGlyph:  sub.WidthForGlyph,
			Ascent:         metrics.Ascent,
			Descent:        metrics.Descent,
			CapHeight:      metrics.CapHeight,
			BBoxXMin:       metrics.BBoxXMin,
			BBoxYMin:       metrics.BBoxYMin,
			BBoxXMax:       metrics.BBoxXMax,
			BBoxYMax:       metrics.BBoxYMax,
			HeadCreated:    created,
			HeadModified:   modified,
		}
	}

	// AC9's "one XObject per asset per document" (the same shape as
	// fonts' "one subset per font per document"): dedup by asset key —
	// decode each DISTINCT referenced asset exactly once, regardless of
	// how many elements place it.
	pdfImages := make(map[string]pdf.ImageXObject, len(imageRuns))
	decodedByKey := make(map[string]template.DecodedImage, len(imageRuns))
	assetKeys := make([]string, 0, len(imageRuns))
	// firstElementIDByAssetKey carries FIRST (in imageRuns' authored
	// order) referencing element id alongside each distinct asset key,
	// so a render-time error can name the element that caused it (D-1.8.1
	// amended's binding verdict-table clause: "Located, naming element
	// id, asset key and media type" — Finding 4, Story 1.8 review: this
	// was previously hard-coded to "", producing a visible hole in the
	// error message instead of the element id).
	firstElementIDByAssetKey := make(map[string]string, len(imageRuns))
	seenAssetKey := map[string]bool{}
	for _, r := range imageRuns {
		if seenAssetKey[r.assetKey] {
			continue
		}
		seenAssetKey[r.assetKey] = true
		assetKeys = append(assetKeys, r.assetKey)
		firstElementIDByAssetKey[r.assetKey] = r.elementID
	}
	slices.Sort(assetKeys)
	// visibleAssetKeys (Story 3.5 finisher review, Blocker 1): the set of
	// asset keys reached by at least one VISIBLE image run. assetKeys
	// itself stays derived from EVERY image run, hidden or not — every
	// asset is still resolved and validated below (asset presence,
	// DecodeAssetBytes, DecodeImageForRender), unconditionally, because
	// AC7(b) requires a hidden image's broken asset to still error
	// exactly as it would while visible (mutation M2 reddens if this
	// validation is skipped for a hidden run). What must NOT happen
	// unconditionally is EMBEDDING: an asset reached only by hidden runs
	// must not enter pdfImages, or the PDF still carries the /XObject for
	// an element AC1/AC2 say contributes zero entries to the page model.
	visibleAssetKeys := make(map[string]bool, len(imageRuns))
	for _, r := range imageRuns {
		if isVisible(visible, template.ElementID(r.elementID)) {
			visibleAssetKeys[r.assetKey] = true
		}
	}
	for _, key := range assetKeys {
		asset, ok := t.doc.Assets[key]
		if !ok {
			// Story 3.5 finisher review, Finding 7 (Minor): names the
			// element too, not only the asset key — firstElementIDByAssetKey
			// is already populated by the loop above, and AC7(b) itself
			// requires this error "unchanged in text and in LOCATION"
			// whether the referencing element is visible or hidden.
			return nil, nil, nil, nil, fmt.Errorf("folio8: Render: element %s: an image element references asset %q, which is not present in the document's assets map", firstElementIDByAssetKey[key], key)
		}
		raw, derr := template.DecodeAssetBytes(asset)
		if derr != nil {
			return nil, nil, nil, nil, fmt.Errorf("folio8: Render: asset %q: %w", key, derr)
		}
		img, derr := template.DecodeImageForRender(asset.MediaType, raw, key, firstElementIDByAssetKey[key])
		if derr != nil {
			return nil, nil, nil, nil, fmt.Errorf("folio8: Render: %w", derr)
		}
		decodedByKey[key] = img
		if !visibleAssetKeys[key] {
			// Validated above like every other asset; not embedded,
			// because nothing visible ever references it.
			continue
		}
		pdfImages[key] = pdf.ImageXObject{
			Width:            img.Width(),
			Height:           img.Height(),
			ColorSpace:       img.ColorSpace,
			BitsPerComponent: img.BitsPerComponent,
			Filter:           img.Filter,
			HasDecodeParms:   img.HasDecodeParms,
			PredictorColors:  img.PredictorColors,
			PredictorBPC:     img.PredictorBPC,
			PredictorColumns: img.PredictorColumns,
			Stream:           img.Stream,
		}
	}

	pdfPlacements := make([]pagemodel.ImagePlacement, len(imageRuns))
	for i, r := range imageRuns {
		img := decodedByKey[r.assetKey]
		drawX, drawY, drawW, drawH := resolveImagePlacement(r, img)
		pdfPlacements[i] = pagemodel.ImagePlacement{
			AssetKey:   r.assetKey,
			X:          drawX,
			Y:          drawY,
			DrawWidth:  drawW,
			DrawHeight: drawH,
		}
	}

	pdfRuns, cerr := buildShapedPDFRuns(runs, shapedRuns, clusterTexts, subsets, embedded, cache, fs)
	if cerr != nil {
		return nil, nil, nil, nil, cerr
	}

	// Story 2.7, AC2's between-passes attachment point: buildShapedPDFRuns
	// has just allocated CIDs for every glyph in the document, INCLUDING
	// each page-slot's digit table, so this is the first point at which
	// buildPageNumberSlot can read the ten pre-shaped, pre-CID'd digits a
	// {{page}} occurrence will select among. Nothing here shapes
	// anything — it reads what pass one already measured.
	//
	// APPENDED, not assigned (this story's review, Blocker 1): `pending`
	// carries one entry per {{page}} OCCURRENCE, and more than one can
	// share a runIndex — a run's PageSlots is the ordered collection of
	// every reservation it carries, not its last one.
	for _, ps := range pending {
		slot, serr := buildPageNumberSlot(pdfRuns[ps.runIndex].Face, pdfRuns[ps.digitTableIndex], ps)
		if serr != nil {
			return nil, nil, nil, nil, serr
		}
		pdfRuns[ps.runIndex].PageSlots = append(pdfRuns[ps.runIndex].PageSlots, *slot)
	}

	// internal/layout produces the page model (AD-5); package folio8 hands
	// it to a renderer and does nothing else with it.
	//
	// Story 2.6: this used to be `[]pagemodel.Page{layout.ComposePage(...)}`
	// — a ONE-ELEMENT SLICE, which was the entire defect. Everything
	// upstream already produced page-absolute content and everything
	// downstream already handled N pages: pdf.SerializeTextDocument
	// reserves a page/content object pair per page and writes len(pages)
	// into /Count. Only the middle produced one. Content taller than the
	// content band was still DRAWN — below the bottom edge of the sheet,
	// with no error and no warning.
	pages, repeatDiags, perr := paginateDocument(geometry, runs, imageRuns, tableRects, pdfRuns, pdfPlacements, visible, keepTogether, contentPages)
	if perr != nil {
		return nil, nil, nil, nil, perr
	}
	diags = append(diags, repeatDiags...)

	return pages, embedded, pdfImages, diags, nil
}

// keepTogetherKeyPrefix namespaces every keep-together group's
// layout.ItemGroupKey.ElementID, and it is a CORRECTNESS device rather
// than a naming convention (Story 7.7, D-7.7 Ruling C).
//
// validateElementID (internal/template/ids.go) admits only `^e[0-9a-z]+$`
// with no leading zero and a decoded counter >= 1, enforced document-wide
// at parse time. A key whose ElementID contains a character outside
// [0-9a-z] — here, the ':' — is therefore PROVABLY never equal to any
// real element's id, and that single fact is what makes every
// table-shaped path in internal/layout unreachable for a keep-together
// group at once: headerExtent cannot match, so the sweep's `table` stays
// "" and ceilingFor is unnarrowed; no HeaderRepeats, no RowDisplacement
// and no TableHeaderSuppressed can be produced for it; headerContentOf
// cannot match; and footerOrphanTargetsFrom builds no target for it, so
// applyFooterMerge can never re-key it.
//
// THE FOUR SITES THAT ASSUME A KEY MEANS A TABLE, enumerated because
// D-7.7.1 requires the audit to be a list rather than a reassurance
// (line numbers as measured at this story's baseline):
//
//	paginate.go:833   `if !it.Group.Key.IsHeader` — the clip branch's
//	                  header-repeat arm. Excluded by IsHeader being
//	                  false AND by headerExtent missing, below.
//	paginate.go:839   `tbl := it.Group.Key.ElementID` — reads the key as
//	                  A TABLE'S ID. Namespaced, so headerExtent(tbl)
//	                  cannot hit and the arm is never entered.
//	paginate.go:949-950  `headerPageOf[...]` — written only under
//	                  `it.Group.Key.IsHeader`, which is false here.
//	paginate.go:956-964  Gate B, `headerExtent(Key.ElementID)` — the
//	                  gate that decides whether `table` is set at all.
//	                  It cannot hit, so `table` stays "", ceilingFor is
//	                  unnarrowed and the whole FR26 reservation block is
//	                  unreachable.
//
// And paginate.go's clip-branch comment — "Keyed on Group.Present, and
// NOT on the item's kind" — is the decision Story 7.7 put under load: it
// became true of a second population, deliberately (D-4.6.2 as amended
// 2026-08-31), and table_row_clip_test.go's tripwire is what holds the
// line at those two. Story 7.10 then split that key: the clip is keyed on
// Group.AuthorDeclared as well, so a keep-together group over-tall in ONE
// OF ITS OWN ELEMENTS is refused rather than clipped (D-7.10.1). The one
// bit internal/layout learns is the group's PROVENANCE; it still learns
// nothing about this prefix, which is what the paragraphs above are for.
//
// TestKeepTogetherGroupKeyIsNotAValidElementID asserts the grammar
// rejects it, rather than asserting the convention in prose — the day
// someone "tidies" this prefix, every one of those paths reopens
// silently, and TestKeepTogetherReachesNoTablePath asserts the
// consequence behaviourally.
const keepTogetherKeyPrefix = "keepTogether:"

// keepTogetherGroupIndex is the layout.ItemGroupKey.Index every
// keep-together group carries. It is distinct from footerGroupIndex (-1)
// and from every data row's index (>= 0), so the clipped-row
// diagnostic's role switch can tell the three apart without needing the
// prefix — though it checks the prefix too, because THAT is the
// namespace fact the rest of the design rests on.
const keepTogetherGroupIndex = -2

// keepTogetherIndex maps a content-band element's id to the
// author-declared keep-together tag it carries — Story 7.7's FR51
// declaration, read off the document ONCE and then consulted by LOOKUP
// only.
//
// It is never RANGED (D-1.3.5 / R5): a map range would reach the order
// in which items are built, and therefore the emitted byte order. It is
// built by walking the content band's own element slice, which is the
// authored order.
type keepTogetherIndex map[string]keepTogetherMember

// keepTogetherMember is one tagged element's entry: its tag, and whether it
// is the below-line side of a group the section break splits
// (spec-section-break). The two sides of a split group carry the same tag but
// different group keys, so each side is kept together on its own.
type keepTogetherMember struct {
	tag        string
	belowBreak bool
}

// keepTogetherBelowBreakGroupIndex is the layout.ItemGroupKey.Index of the
// below-line side of a keepTogether group the section break splits. Distinct
// from keepTogetherGroupIndex, so the two sides never share a key whatever
// the tag, while keepTogetherTagOf still reads the one tag off either.
const keepTogetherBelowBreakGroupIndex = -3

// keepTogetherTags reads the document's keep-together declarations.
// Elements outside the content band, and tables, cannot carry the key at
// all (parse_bands.go refuses both at load), so this walks the content
// band alone.
//
// spec-section-break: a group with members on both sides of the section
// break is split at the line HERE, so pass A, pass B and the canvas all see
// the same two groups. A group wholly on one side keeps its one key.
func keepTogetherTags(t *Template) keepTogetherIndex {
	if t == nil || t.doc == nil {
		return nil
	}
	// SPEC-multi-pages CAP-6: each designed page's group is split by that
	// page's own break, and by no other page's.
	var idx keepTogetherIndex
	for page, band := range t.doc.ContentBands() {
		split, _ := pageSectionBreakSplitTags(t, page)
		offset, _ := declaredSectionBreak(t, page)
		for _, el := range band.Elements {
			if !el.KeepTogether.Set || el.KeepTogether.Null || el.KeepTogether.Value == "" {
				continue
			}
			if idx == nil {
				idx = keepTogetherIndex{}
			}
			member := keepTogetherMember{tag: el.KeepTogether.Value}
			if el.Y >= offset && slices.Contains(split, member.tag) {
				member.belowBreak = true
			}
			idx[string(el.ID)] = member
		}
	}
	return idx
}

// keepTogetherGroup is Story 7.7's grouping derivation — the THIRD, and
// the only non-table one, of package folio8's present-ItemGroup
// constructions (see TestAPresentItemGroupIsATableRowOrAKeepTogetherGroup,
// table_row_clip_test.go).
//
// Two elements sharing one tag produce one EQUAL Key, and equality of Key
// is the whole of internal/layout's definition of a group
// (paginate.go's ItemGroup.Key doc). The paginator requires no
// contiguity — R7's contiguity premise is recorded there as measured
// FALSE and removed — so a group of loose, non-adjacent signature
// elements needs no new mechanism and no key extension.
//
// An untagged element gets the ZERO ItemGroup, which is "not grouped" and
// is exactly what every item carried before this story. That is what
// makes a document declaring no tag byte-identical.
//
// AuthorDeclared IS THE ONE THING THAT SEPARATES THIS DERIVATION FROM THE
// OTHER TWO (Story 7.10, D-7.10.2). It is set here, and only here, because
// this is the only grouping in package folio8 that exists because a person
// typed something: a table row's grouping is the engine's own, built from
// data the author may never have seen. internal/layout reads that one bit
// and refuses an over-tall element of an author-declared group instead of
// clipping it — never the tag itself, which stays this package's word
// (see keepTogetherKeyPrefix's doc comment).
func (idx keepTogetherIndex) keepTogetherGroup(elementID string) layout.ItemGroup {
	member, ok := idx[elementID]
	if !ok {
		return layout.ItemGroup{}
	}
	index := keepTogetherGroupIndex
	if member.belowBreak {
		index = keepTogetherBelowBreakGroupIndex
	}
	return layout.ItemGroup{Present: true, AuthorDeclared: true, Key: layout.ItemGroupKey{
		ElementID: keepTogetherKeyPrefix + member.tag,
		IsHeader:  false,
		Index:     index,
	}}
}

// orKeepTogether substitutes the keep-together group ONLY where the
// item's existing group is not Present.
//
// Filling only the ungrouped case is what preserves byte-identity: a
// table's rows already carry a row key and keep it untouched, and an
// untagged element's zero group stays the zero group. One item belongs
// to at most one group, which is also why parse_bands.go refuses the tag
// on a table rather than trying to honour both.
func (idx keepTogetherIndex) orKeepTogether(g layout.ItemGroup, elementID string) layout.ItemGroup {
	if g.Present {
		return g
	}
	return idx.keepTogetherGroup(elementID)
}

// keepTogetherTagOf returns the author's own tag for a keep-together
// group key, and whether the key names one at all. Read from the key's
// namespace prefix — the same fact Ruling C rests on — never from the
// Index sentinel alone.
func keepTogetherTagOf(key layout.ItemGroupKey) (string, bool) {
	if key.IsHeader || !strings.HasPrefix(key.ElementID, keepTogetherKeyPrefix) {
		return "", false
	}
	return strings.TrimPrefix(key.ElementID, keepTogetherKeyPrefix), true
}

// contentBandIndex is documentBands' authored index for the content band —
// the one band Story 2.6 paginates. The page header and page footer are
// repeated verbatim on every page instead.
const (
	pageHeaderBandIndex = 0
	contentBandIndex    = 1
	pageFooterBandIndex = 2
)

// paginateDocument turns one document's finished, page-absolute content into
// N pages, by asking internal/layout where the window boundaries fall.
//
// AD-4 IS THE POINT OF THIS FUNCTION'S SHAPE. Everything it receives is
// already laid out: pdfRuns and pdfPlacements carry final page-absolute
// coordinates, and this function only decides WHICH PAGE each belongs to and
// subtracts that page's window shift. It measures nothing, breaks nothing and
// re-positions nothing relative to anything else — pass one already did all
// of it. The page model that leaves here is finished, which is what lets
// internal/pdf lay nothing out (internal/passtwo_arch_test.go).
//
// THE PER-PAGE ORDER IS load-bearing and is NOT incidental: page header runs,
// then that page's content runs in AUTHORED order, then page footer runs —
// exactly the order documentBands walks the three bands, and therefore
// exactly the order the pre-2.6 code produced. That is what makes a document
// which fits on one page emit the SAME BYTES as before this story, which
// every one of the six existing goldens depends on.
// visible (Story 3.5, R3/AC7) filters ONLY imageRuns here, for the same
// reason contentColumnItems does (page_number.go's own doc comment):
// runs (text) has already had every hidden element's runs excluded
// upstream, strictly after that element's own validation ran, while
// imageRuns stays deliberately unfiltered until this, the FINAL
// page-model construction step, so every image element's asset
// resolution already ran unconditionally before visible is consulted.
func paginateDocument(
	geometry layout.PageGeometry,
	runs []textRunSource,
	imageRuns []imageRunSource,
	tableRects []tableRectSource,
	pdfRuns []pagemodel.TextRun,
	pdfPlacements []pagemodel.ImagePlacement,
	visible visibilityVerdicts,
	keepTogether keepTogetherIndex,
	contentPages contentPagesSplit,
) ([]pagemodel.Page, []Diagnostic, error) {
	// The two repeated bands, and the content column's atomic items.
	var header, footer layout.BandContent
	var items []layout.ColumnItem

	// Story 4.1: flatten every table's header-row rects into ONE slice
	// this function owns (pdfRects, below) — the same "index into a
	// caller-owned slice" shape imageRuns/pdfPlacements already use for
	// images, so layout.RectRef needs no new machinery. Each
	// tableRectSource contributes a CONTIGUOUS span; collectBandTableRuns
	// already filtered to visible tables with >=1 column, so no
	// isVisible check is repeated here (mirrors this file's own images
	// loop below, which DOES re-check — imageRuns is unfiltered by
	// design, see its own doc comment; tableRects is not).
	// rectIsDataRow, parallel to pdfRects (Story 4.4): whether the RectRef
	// at that index belongs to a DATA ROW's chrome (never the header's
	// own) — read by DIRECT FIELD LOOKUP from the SAME tableRectSource
	// this loop already walks, never reconstructed from ElementID/extent/
	// order (D-4.2.2). Consulted below to apply a page's RowDisplacement
	// (FR26) to exactly the rows it names, and to nothing else.
	var pdfRects []pagemodel.Rect
	var rectIsDataRow []bool
	var rectElementID []string
	// rectSource, parallel to pdfRects (SPEC-table-rules): the index in
	// tableRects each rect came from, so a page's frame can find its
	// table's rects and their row regions by direct lookup.
	var rectSource []int
	for srcIdx, ts := range tableRects {
		lo := len(pdfRects)
		pdfRects = append(pdfRects, ts.rects...)
		for range ts.rects {
			rectSource = append(rectSource, srcIdx)
			// Story 4.5: a footer row's chrome gets the SAME per-table
			// row displacement a data row's chrome gets (AC6) — it is
			// one more row of this table for FR26's purposes.
			rectIsDataRow = append(rectIsDataRow, ts.isDataRow || ts.isFooterRow)
			rectElementID = append(rectElementID, ts.elementID)
		}
		refs := make([]layout.RectRef, 0, len(ts.rects))
		for k := lo; k < len(pdfRects); k++ {
			refs = append(refs, layout.RectRef(k))
		}
		switch ts.band {
		case pageHeaderBandIndex:
			header.Rects = append(header.Rects, refs...)
		case pageFooterBandIndex:
			footer.Rects = append(footer.Rects, refs...)
		default:
			items = append(items, layout.ColumnItem{
				ElementID: ts.elementID,
				Top:       ts.top,
				Bottom:    ts.bottom,
				Rects:     refs,
				// Story 4.3, AC1/AC5, DECISION-1: the row's grouping
				// identity, by direct field lookup (R3) — never
				// reconstructed from ElementID/extent/order. Story 7.7
				// substitutes a keep-together group ONLY where that
				// derivation returns the zero (ungrouped) value, which
				// is what leaves every table row untouched.
				Group: keepTogether.orKeepTogether(ts.chromeRowGroup(), ts.elementID),
				// SPEC-table-rules: report this table's per-page slices
				// (and floor them) when it draws a frame, rules or a
				// floor. The zero value for every other table.
				Slice: ts.frame.sliceRequest(),
			})
			items[len(items)-1].Left, items[len(items)-1].Right, items[len(items)-1].HasExtent = rectSourceExtent(ts)
		}
	}

	// SPEC-table-rules: a table in the page header or page footer is not
	// paginated, so its one slice is the whole table, floored — built once
	// and drawn on every page with the band's other rects.
	bandSlices := map[int][]frameSlice{}
	for _, ts := range tableRects {
		if ts.frame == nil || (ts.band != pageHeaderBandIndex && ts.band != pageFooterBandIndex) {
			continue
		}
		list := bandSlices[ts.band]
		if n := len(list); n > 0 && list[n-1].elementID == ts.elementID {
			if ts.top < list[n-1].top {
				list[n-1].top = ts.top
			}
			if ts.bottom > list[n-1].bottom {
				list[n-1].bottom = ts.bottom
			}
		} else {
			list = append(list, frameSlice{elementID: ts.elementID, top: ts.top, bottom: ts.bottom})
		}
		// The floor is capped at the band's own bottom — the band-table
		// form of "the floor never passes the window" (review item 3).
		bandBottom := layout.Origins(geometry).Content
		if ts.band == pageFooterBandIndex {
			bandBottom = layout.Origins(geometry).PageFooter + geometry.PageFooterHeight
		}
		floored := list[len(list)-1].top + ts.frame.minHeight
		if floored > bandBottom {
			floored = bandBottom
		}
		if floored > list[len(list)-1].bottom {
			list[len(list)-1].bottom = floored
		}
		bandSlices[ts.band] = list
	}

	// Text. One line's runs are CONTIGUOUS in `runs` and share
	// (band, elementID, lineIndex) — positionSegments emits them together —
	// so the grouping below is a scan for a change of key, never a map
	// (D-1.3.5 / ScanMapRange: a map range would make the item order, and
	// therefore the emitted byte order, non-deterministic).
	for i := 0; i < len(runs); i++ {
		switch runs[i].band {
		case pageHeaderBandIndex:
			header.Runs = append(header.Runs, layout.TextRunRef(i))
			continue
		case pageFooterBandIndex:
			footer.Runs = append(footer.Runs, layout.TextRunRef(i))
			continue
		case digitTableBandIndex:
			// Story 2.7: exists only to carry a face's ten digit CIDs
			// through buildShapedPDFRuns (see digitTableRun) — never
			// drawn, never assigned to a page, never a content-band
			// item. Explicitly skipped, by name, rather than falling
			// into the "unrecognised band" internal error below: that
			// error guards documentBands' three-band ENUMERATION, and
			// this band is deliberately outside it.
			continue
		}
		// Content band: gather this whole line.
		//
		// This ASSUMES runs[i].band == contentBandIndex, since the switch
		// above already `continue`d past header and footer. That is true
		// today because documentBands (see documentBands, this file)
		// enumerates exactly three bands — but the assumption used to be
		// implicit: if it ever stopped holding (a fourth band), the loop
		// below matches ZERO runs at j (its condition fails at j==i), so
		// item.Runs stays empty, i = j-1 restores i, and the outer i++
		// puts i right back where it started — an infinite loop appending
		// an empty ColumnItem every iteration until OOM. Story 2.6
		// finisher, Finding 10: made an explicit internal error instead of
		// relying on an unasserted enumeration invariant in another
		// function.
		if runs[i].band != contentBandIndex {
			return nil, nil, fmt.Errorf("folio8: internal error: paginateDocument: run %d has band %d, which is neither the page-header, page-footer, nor content band — documentBands' three-band enumeration invariant no longer holds", i, runs[i].band)
		}
		j := i
		item := layout.ColumnItem{
			ElementID: runs[i].elementID,
			Top:       runs[i].itemTop,
			Bottom:    runs[i].itemBottom,
			// Story 4.3, AC1/AC5, DECISION-1: see the rects loop above —
			// same identity, same direct-lookup rule, for a row's line
			// items. Story 7.7's substitution applies here too, and only
			// to a line that is not already a row's.
			Group: keepTogether.orKeepTogether(runs[i].lineRowGroup(), runs[i].elementID),
		}
		for j < len(runs) &&
			runs[j].band == contentBandIndex &&
			runs[j].elementID == runs[i].elementID &&
			runs[j].lineIndex == runs[i].lineIndex {
			item.Runs = append(item.Runs, layout.TextRunRef(j))
			extendItem(&item, runs[j].x, runs[j].x)
			j++
		}
		items = append(items, item)
		i = j - 1
	}

	// Images. Each is its own atomic item, and its extent is its DECLARED
	// BOX (r.y .. r.y+boxH), not the drawn box centred inside it: AD-24
	// already scaled the image to fit the box, so "does it fit on a page" is
	// a question about the BOX the template declared (D-2.6.1, rule 4).
	for i, r := range imageRuns {
		if !isVisible(visible, template.ElementID(r.elementID)) {
			// AD-24/R3: absent from the page model entirely. r's own
			// validation (width/height/asset presence, and the
			// deduplicated asset-resolution pass keyed by asset key,
			// buildPageModel) already ran unconditionally regardless
			// of this verdict — this is strictly the last step, page-
			// model construction, and it never skips one.
			continue
		}
		switch r.band {
		case pageHeaderBandIndex:
			header.Images = append(header.Images, layout.ImageRef(i))
		case pageFooterBandIndex:
			footer.Images = append(footer.Images, layout.ImageRef(i))
		default:
			items = append(items, layout.ColumnItem{
				ElementID: r.elementID,
				Top:       r.y,
				Bottom:    r.y + r.boxH,
				Images:    []layout.ImageRef{layout.ImageRef(i)},
				// Story 7.7: an image carried no group at all before
				// this story, and still carries none unless its element
				// is tagged — a signature's ruled line and its scanned
				// mark belong to the same block as its name.
				Group: keepTogether.keepTogetherGroup(r.elementID),
			})
			extendItem(&items[len(items)-1], r.x, r.x+r.boxW)
		}
	}

	plan, footerOrphanDiags, err := paginateContentPages(geometry, items, contentPages)
	if err != nil {
		return nil, nil, wrapOverflowError(err)
	}

	// Story 4.4, DECISION-2: one Warning per (table, page) suppression,
	// built straight from Paginate's OWN decision — never a second,
	// independent re-run of the fit arithmetic (D-4.2.2). The message
	// names the table, the page, the row's own height, the space it fit
	// inside WITHOUT the reservation, and the three levers a template
	// author actually has (D-000.37): reduce the header's declared
	// height, reduce the row's height (font size/padding), or increase
	// the page's content height (smaller margins, or a smaller
	// page-header/page-footer).
	var repeatDiags []Diagnostic
	repeatDiags = append(repeatDiags, footerOrphanDiags...)
	for _, s := range plan.Suppressed {
		repeatDiags = append(repeatDiags, Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeTableHeaderRepeatSuppressed,
			ElementID: s.ElementID,
			Message: fmt.Sprintf(
				"folio8: Render: element %s: the repeated header could not be drawn on page %d — the next row is %s tall and only %s is available on that page without the header's own reservation (the table's own headerHeight is %s), so the header repeat is suppressed on this page only (FR26). Reduce the table's headerHeight, reduce this row's height (font size or cell padding), or increase the page's content height (smaller margins, or a smaller page-header/page-footer)",
				s.ElementID, s.Page+1, millipointsForDiag(s.RowHeight), millipointsForDiag(s.Available), millipointsForDiag(s.HeaderHeight)),
		})
	}

	// Story 4.6 (FR25/AD-14/D-4.6.3): one Warning per clipped group,
	// built straight from Paginate's OWN decision — never a second,
	// independent re-run of the fit arithmetic (D-4.2.2).
	for _, c := range plan.Clipped {
		repeatDiags = append(repeatDiags, clippedRowDiagnostic(c))
	}

	pages := make([]pagemodel.Page, 0, len(plan.Pages))
	for pageIdx, assigned := range plan.Pages {
		// Story 2.7, AC2's between-passes step: pageNum is THIS page's
		// own number, 1-based. resolvePageRunForPage is a no-op for
		// every run but the ones carrying a PageSlots entry (empty for
		// every document that declares no {{page}} construct, which is
		// what keeps every pre-2.7 golden byte-identical: len(PageSlots)
		// == 0 returns run unchanged, verbatim, exactly as this loop
		// already copied it).
		pageNum := pageIdx + 1
		pageRuns := make([]pagemodel.TextRun, 0, len(header.Runs)+len(assigned.ContentRuns)+len(footer.Runs))
		for _, ref := range header.Runs {
			pageRuns = append(pageRuns, resolvePageRunForPage(pdfRuns[ref], pageNum))
		}
		// Story 4.4, FR26/DECISION-3: this page's repeated table headers,
		// drawn before that page's own content — the SAME Rects/Runs the
		// table's own header carries (R6: no new glyphs, no second
		// producer), repositioned by the repeat's OWN Shift, a quantity
		// separate from assigned.Shift (DECISION-3: the page's Shift is
		// untouched and continues to govern everything else on the page).
		for _, rep := range assigned.HeaderRepeats {
			for _, ref := range rep.Runs {
				run := pdfRuns[ref]
				run.Y -= rep.Shift
				pageRuns = append(pageRuns, run)
			}
		}
		for _, ref := range assigned.ContentRuns {
			// The window shift, and it is the ONLY transformation
			// pagination applies. Every content item on one page shares it,
			// so no item can be displaced relative to another — the column
			// itself is never mutated.
			run := pdfRuns[ref]
			// spec-section-break: a section element on the page it
			// shares with the above-line content has its own shift.
			run.Y -= plan.outputShiftFor(pageIdx, runs[ref].elementID)
			// Story 4.4: a repeating table's OWN rows are displaced
			// further down, beyond Shift, to make room for the repeat
			// above them — scoped to that table's ElementID alone
			// (DECISION-3), never to any other element on this page.
			// Story 4.5: the footer's own value lines get the same
			// displacement its chrome does (AC6), for the same reason.
			if runs[ref].isTableRowLine || runs[ref].isFooterLine {
				run.Y += rowDisplacementFor(assigned.RowDisplacement, runs[ref].elementID)
			}
			// SPEC-table-rules §3: a floored table above this element on
			// this page pushes it below the floored bottom.
			run.Y += elementPushFor(assigned.ElementPush, runs[ref].elementID)
			pageRuns = append(pageRuns, run)
		}
		for _, ref := range footer.Runs {
			pageRuns = append(pageRuns, resolvePageRunForPage(pdfRuns[ref], pageNum))
		}

		pageImages := make([]pagemodel.ImagePlacement, 0, len(header.Images)+len(assigned.ContentImages)+len(footer.Images))
		for _, ref := range header.Images {
			pageImages = append(pageImages, pdfPlacements[ref])
		}
		for _, ref := range assigned.ContentImages {
			img := pdfPlacements[ref]
			img.Y -= plan.outputShiftFor(pageIdx, imageRuns[ref].elementID)
			img.Y += elementPushFor(assigned.ElementPush, imageRuns[ref].elementID)
			pageImages = append(pageImages, img)
		}
		for _, ref := range footer.Images {
			pageImages = append(pageImages, pdfPlacements[ref])
		}

		// Every rect of this page, with the table source it came from, so
		// SPEC-table-rules' frames can be drawn per slice once the page's
		// rects are placed (applyTableFrames). A page carrying no slice
		// comes out of applyTableFrames exactly as it went in.
		headerEntries := make([]framedRect, 0, len(header.Rects))
		for _, ref := range header.Rects {
			headerEntries = append(headerEntries, framedRect{rect: pdfRects[ref], src: rectSource[ref]})
		}
		pageRects := applyTableFrames(headerEntries, tableRects, bandSlices[pageHeaderBandIndex])
		contentEntries := make([]framedRect, 0, len(assigned.ContentRects))
		for _, rep := range assigned.HeaderRepeats {
			for _, ref := range rep.Rects {
				r := pdfRects[ref]
				r.Y -= rep.Shift
				contentEntries = append(contentEntries, framedRect{rect: r, src: rectSource[ref], repeat: true})
			}
		}
		for _, ref := range assigned.ContentRects {
			r := pdfRects[ref]
			// Story 4.6: this rect belongs to a group taller than the
			// page, so its bottom edge is cut off at the content
			// bottom. Applied in COLUMN space, BEFORE the shift, because
			// layout.RectClip.Bottom is a column coordinate exactly like
			// a ColumnItem's own Top/Bottom — and applied as
			// min(rect's own bottom, the bound), so a group of several
			// rects with different extents is correct with one number
			// and a rect that already ends above the cut is untouched.
			// No PDF clip path is involved: truncating a rectangle is a
			// change to a rectangle (AD-5).
			if bottom, clip := rectClipBottomFor(assigned.ClippedRects, ref); clip && r.Y+r.H > bottom {
				r.H = bottom - r.Y
			}
			r.Y -= plan.outputShiftFor(pageIdx, rectElementID[ref])
			if rectIsDataRow[ref] {
				r.Y += rowDisplacementFor(assigned.RowDisplacement, rectElementID[ref])
			}
			r.Y += elementPushFor(assigned.ElementPush, rectElementID[ref])
			contentEntries = append(contentEntries, framedRect{rect: r, src: rectSource[ref]})
		}
		contentSlices := make([]frameSlice, 0, len(assigned.TableSlices))
		for _, sl := range assigned.TableSlices {
			contentSlices = append(contentSlices, frameSlice{elementID: sl.ElementID, top: sl.Top, bottom: sl.Bottom})
		}
		pageRects = append(pageRects, applyTableFrames(contentEntries, tableRects, contentSlices)...)
		footerEntries := make([]framedRect, 0, len(footer.Rects))
		for _, ref := range footer.Rects {
			footerEntries = append(footerEntries, framedRect{rect: pdfRects[ref], src: rectSource[ref]})
		}
		pageRects = append(pageRects, applyTableFrames(footerEntries, tableRects, bandSlices[pageFooterBandIndex])...)

		pages = append(pages, layout.ComposePage(geometry, pageRuns, pageImages, pageRects))
	}
	return pages, repeatDiags, nil
}

// extendItem widens a column item's horizontal extent to cover left..right
// (SPEC-table-rules review item 4: the floor push is for what lies BELOW a
// table, never beside it, and pagination needs the x extent to tell).
func extendItem(item *layout.ColumnItem, left, right geom.Length) {
	if !item.HasExtent || left < item.Left {
		item.Left = left
	}
	if !item.HasExtent || right > item.Right {
		item.Right = right
	}
	item.HasExtent = true
}

// rectSourceExtent is a rect source's horizontal extent.
func rectSourceExtent(ts tableRectSource) (left, right geom.Length, ok bool) {
	var item layout.ColumnItem
	for _, r := range ts.rects {
		extendItem(&item, r.X, r.X+r.W)
	}
	return item.Left, item.Right, item.HasExtent
}

// elementPushFor returns the SPEC-table-rules §3 floor push a page's
// ElementPush gives elementID — a single slice walk, empty on every page of
// every document with no floored table.
func elementPushFor(list []layout.ElementPush, elementID string) geom.Length {
	for _, p := range list {
		if p.ElementID == elementID {
			return p.Amount
		}
	}
	return 0
}

// rectClipBottomFor returns the column-space bottom bound (Story 4.6) a
// page's ClippedRects imposes on ref — found by a single SLICE WALK
// (never a map range, R5), for the same reason rowDisplacementFor is one:
// the list is empty on every page of every document with no over-tall
// group, and holds one clipped group's rects otherwise.
func rectClipBottomFor(list []layout.RectClip, ref layout.RectRef) (geom.Length, bool) {
	for _, c := range list {
		if c.Ref == ref {
			return c.Bottom, true
		}
	}
	return 0, false
}

// clippedRowDiagnostic turns ONE of Paginate's clip decisions into the
// located Warning AD-14 requires (Story 4.6, AC4/AC5). It is a named
// function rather than an inline literal so the ROLE rendering — the one
// thing that must never leak a wire value at an author — is assertable
// directly for all three group roles, including the footer's Index -1
// sentinel, without needing a document that produces each.
//
// The message names the table, the row (BY ROLE, never by the sentinel),
// the row's own height, the content height it was measured against, and
// the three levers a template author actually has (D-000.37, "executable
// by a human") — the same three the sibling TABLE_HEADER_REPEAT_SUPPRESSED
// and TABLE_FOOTER_ORPHAN_SUPPRESSED messages name.
func clippedRowDiagnostic(c layout.TableRowClipped) Diagnostic {
	// The row index the epic requires named — read straight off the
	// group's own Key (D-4.2.2: never re-derived from extent or order).
	// footerGroupIndex is -1, a WIRE VALUE: a message that printed it
	// verbatim would put "row -1" in front of a human.
	//
	// STORY 7.7 ADDS THE FOURTH ARM, and it is not optional. A
	// keep-together group is not a row of anything: without this arm an
	// author's signature block is announced as "row 0 of the bound
	// collection" with a remedy about cell padding, which names neither
	// the thing that was clipped nor an action its author could take.
	// It is tested FIRST because the namespace prefix is the fact the
	// whole design rests on, and because such a key's Index sentinel
	// (-2) is deliberately neither the footer's (-1) nor a data row's.
	var row, remedy string
	tag, isKeepTogether := keepTogetherTagOf(c.Key)
	switch {
	case isKeepTogether:
		row = fmt.Sprintf("the keep-together group %q", tag)
		remedy = "Remove a member from this group, stop declaring the group so its elements paginate individually,"
	case c.Key.IsHeader:
		row = "the header row"
	case c.Key.Index == footerGroupIndex:
		row = "the footer row"
	default:
		row = fmt.Sprintf("row %d of the bound collection", c.Key.Index)
	}
	if remedy == "" {
		remedy = "Reduce this row's height (font size or cell padding), shorten the data in it,"
	}
	return Diagnostic{
		Severity:  SeverityWarning,
		Code:      DiagCodeTableRowClippedHeight,
		ElementID: c.ElementID,
		Message: fmt.Sprintf(
			"folio8: Render: element %s: %s is %s tall, which is taller than the whole %s content window, so it fits on no page — it was placed alone on page %d and CLIPPED at that page's content bottom, and the content past the bottom is absent from this document (FR25). %s or increase the page's content height (smaller margins, or a smaller page-header/page-footer)",
			c.ElementID, row, millipointsForDiag(c.ItemHeight), millipointsForDiag(c.ContentHeight), c.Page+1, remedy),
	}
}

// rowDisplacementFor returns the extra downward displacement (Story 4.4)
// a page's RowDisplacement reserves for elementID — 0 if none — found by a
// single SLICE WALK (never a map range, R5): the list is small (one entry
// per repeating table on that page) and its own order is not itself
// meaningful, so a linear scan is the simplest correct read.
func rowDisplacementFor(list []layout.TableRowDisplacement, elementID string) geom.Length {
	for _, d := range list {
		if d.ElementID == elementID {
			return d.Amount
		}
	}
	return 0
}

// millipointsForDiag spells a geom.Length for a HUMAN-READABLE diagnostic
// message (Story 4.4, D-000.37) — mirrors internal/layout's own
// millipoints helper (that one is unexported and this package may not
// import internal/layout for a formatting helper alone); not an
// output-format emitter, so AD-3's "one number emitter" (internal/pdf's
// numbers.go) is untouched.
func millipointsForDiag(v geom.Length) string {
	return strconv.FormatInt(int64(v), 10) + "mp"
}

// cidKey identifies one allocated CID: a subset glyph together with the
// text it extracts as. Two entries sharing a glyph but differing in text
// are two CIDs pointing at one glyph (Story 2.3, D-2.3-Q1 as ruled) —
// see pdf.EmbeddedFace.ExtraCIDs for the measured case that forces it.
type cidKey struct {
	glyph uint16
	text  string
}

// buildShapedPDFRuns turns the document's shaped runs into the CID-level
// runs internal/pdf emits, and — as the same pass, because the two
// cannot be computed independently — allocates each face's CID space and
// its /ToUnicode entries.
//
// Three things happen here and each is an acceptance criterion:
//
//   - AC4: every CID a content stream will emit originates in a shaped
//     glyph run. There is no other route: this is the only function that
//     produces a pagemodel.ShapedGlyph, and the only input it reads is the
//     shaper's output mapped through the subset plan. The old
//     rune -> GlyphForRune -> CID path does not exist any more, so the
//     property is structural rather than asserted by a denylist.
//   - AC6/AD-2: every position is scaled from FONT UNITS to the PDF's
//     1000-unit em exactly once, here, through geom.ScaleRound — this
//     module's one scaling function with its one documented rounding
//     mode. internal/pdf receives numbers already in the output's unit
//     and scales nothing.
//   - AC7/D-2.3-Q1: CIDs are allocated per (subset glyph, cluster text)
//     pair, in first-encounter order over runs in document order — a
//     deterministic order derived by ranging SLICES only (D-1.3.5).
//
// AD-23 holds trivially here: ot.GlyphPos is int16 throughout and
// geom.Length is int64, so nothing on this path is or becomes a float.
// Advances come from the shaper (which includes GPOS kerning), never
// from ot.Face.HorizontalAdvance (which returns float32 and omits it).
func buildShapedPDFRuns(
	runs []textRunSource,
	shaped [][]text.ShapedGlyph,
	clusterTexts [][]string,
	subsets map[string]*fontset.Subset,
	embedded map[string]pdf.EmbeddedFace,
	cache *fontCache,
	fs FontSet,
) ([]pagemodel.TextRun, error) {
	type faceCIDs struct {
		byKey       map[cidKey]uint16
		baseClaimed map[uint16]bool
		extras      []uint16
		entries     []pdf.CIDText
	}
	alloc := map[string]*faceCIDs{}

	pdfRuns := make([]pagemodel.TextRun, len(runs))
	for i, r := range runs {
		sub, ok := subsets[r.face]
		if !ok {
			return nil, fmt.Errorf("folio8: Render: face %q has shaped runs but no subset", r.face)
		}
		font, ferr := cache.get(r.face, fs)
		if ferr != nil {
			return nil, fmt.Errorf("folio8: Render: face %q: %w", r.face, ferr)
		}
		upem := int64(font.UnitsPerEm())

		state, seen := alloc[r.face]
		if !seen {
			state = &faceCIDs{byKey: map[cidKey]uint16{}, baseClaimed: map[uint16]bool{}}
			alloc[r.face] = state
		}

		glyphs := make([]pagemodel.ShapedGlyph, 0, len(shaped[i]))
		for gi, g := range shaped[i] {
			newGID, retained := sub.GlyphForSource[g.GlyphID]
			if !retained {
				// AC5: a shaped glyph the plan did not retain is a
				// located error naming the face and the glyph id, never
				// a silent .notdef.
				return nil, fmt.Errorf(
					"folio8: Render: face %q: shaped glyph id %d was not retained by the subset plan",
					r.face, g.GlyphID,
				)
			}

			key := cidKey{glyph: newGID, text: clusterTexts[i][gi]}
			cid, allocated := state.byKey[key]
			if !allocated {
				switch {
				case !state.baseClaimed[newGID]:
					// The BASE block: CID == subset glyph id, exactly
					// as every folio8 PDF worked before this story.
					cid = newGID
					state.baseClaimed[newGID] = true
				default:
					// This glyph already carries a different text at its
					// base CID, so its second meaning needs a second CID
					// pointing at the same glyph.
					//
					// Identity-H's CID is TWO BYTES. Past 65535 the
					// conversion below wraps silently and the extra CID
					// collides with the base block, producing both a
					// wrong glyph and a wrong /ToUnicode entry — a silent
					// wrap where every other limit in this codebase is a
					// located error (Story 2.3 finisher, Finding 12).
					// Unreachable in practice (it needs a subset near the
					// 65535-glyph ceiling PLUS context-distinct CIDs on
					// top of it), which is exactly why it would never be
					// noticed if it did happen.
					next := sub.NumGlyphs + len(state.extras)
					if next > 0xFFFF {
						return nil, fmt.Errorf(
							"folio8: Render: face %q: CID space exhausted — the subset has %d glyphs and this "+
								"document needs %d additional CIDs for glyphs carrying more than one source "+
								"text, which exceeds Identity-H's two-byte CID ceiling of 65535",
							r.face, sub.NumGlyphs, len(state.extras)+1,
						)
					}
					cid = uint16(next)
					state.extras = append(state.extras, newGID)
				}
				state.byKey[key] = cid
				state.entries = append(state.entries, pdf.CIDText{CID: cid, Text: key.text})
			}

			glyphs = append(glyphs, pagemodel.ShapedGlyph{
				CID:      cid,
				XAdvance: int64(geom.ScaleRound(geom.Length(int64(g.XAdvance)), 1000, upem)),
				XOffset:  int64(geom.ScaleRound(geom.Length(int64(g.XOffset)), 1000, upem)),
				YOffset:  int64(geom.ScaleRound(geom.Length(int64(g.YOffset)), 1000, upem)),
			})
		}

		pdfRuns[i] = pagemodel.TextRun{
			Face:           r.face,
			Glyphs:         glyphs,
			SourceText:     r.text,
			X:              r.x,
			Y:              r.y,
			FontSize:       r.fontSize,
			BaselineOffset: r.baselineOffset,
			HasColor:       r.hasColor,
			Color:          r.color,
			ClipToBox:      r.clipToBox,
			ClipX:          r.clipX,
			ClipWidth:      r.clipWidth,
		}
	}

	// Write each face's allocated CID space back into its EmbeddedFace.
	// Ranges `runs` (a slice) to reach the face names, never `alloc`
	// (a map) — D-1.3.5, and the reason the entries end up in a
	// deterministic order at all.
	written := map[string]bool{}
	for _, r := range runs {
		if written[r.face] {
			continue
		}
		written[r.face] = true
		state := alloc[r.face]
		face := embedded[r.face]
		face.ExtraCIDs = state.extras
		entries := slices.Clone(state.entries)
		// Ascending CID order — the order buildToUnicodeCMap emits and
		// the order the pre-2.3 CMap already used, so a document that
		// needs no extra CIDs produces byte-identical /ToUnicode.
		slices.SortFunc(entries, func(a, b pdf.CIDText) int { return int(a.CID) - int(b.CID) })
		face.ToUnicode = entries
		embedded[r.face] = face
	}

	return pdfRuns, nil
}

// mergeConditionDiagnostics inserts conditions in declaration order, before
// their own element's body warnings, without reordering existing body warnings.
func mergeConditionDiagnostics(elements []template.Element, conditions, body []Diagnostic) []Diagnostic {
	rank := make(map[string]int, len(elements))
	for i, element := range elements {
		rank[string(element.ID)] = i
	}
	var pending []Diagnostic
	for _, condition := range conditions {
		if _, belongs := rank[condition.ElementID]; belongs {
			pending = append(pending, condition)
		}
	}
	var out []Diagnostic
	next := 0
	for _, warning := range body {
		if position, belongs := rank[warning.ElementID]; belongs {
			for next < len(pending) && rank[pending[next].ElementID] <= position {
				out = append(out, pending[next])
				next++
			}
		}
		out = append(out, warning)
	}
	return append(out, pending[next:]...)
}
