package folio8

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/pagemodel"
)

// tablePagesForTest renders straight to the page model so a test can
// assert on pagemodel.Page.Rects/Runs directly (AC1's own literal
// anchor) without decoding serialized PDF bytes. AC3's other half — a
// byte-level assertion over the actual content stream — lives in
// internal/pdf/rectdoc_test.go, this story's byte-level anchor for the
// operators these pages' Rects ultimately produce.
func tablePagesForTest(t *testing.T, tplJSON, dataJSON string) []pagemodel.Page {
	t.Helper()
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := mustDecodeData(t, dataJSON)
	params := mustDecodeParams(t)
	pages, _, _, _, err := buildPageModel(tpl, data, params, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	return pages
}

// tableHeaderDoc builds a minimal one-table `.folio` document. style is
// injected verbatim (may be "", meaning no "style" key at all — R6,
// amended: at least fontFamily must still be set for a non-empty
// label, so callers pass one).
func tableHeaderDoc(styleJSON, columnsJSON string) string {
	styleField := ""
	if styleJSON != "" {
		styleField = `, "style": ` + styleJSON
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 20,
        "columns": ` + columnsJSON + styleField + `}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"latin": ["Noto Sans"], "thai": ["Noto Sans Thai"], "cjk": ["Noto Sans SC"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// tableHeaderDocFull is tableHeaderDoc's superset (finisher fix, Story
// 4.1 review Blockers 1 and 2): it also injects a "headerStyle" block
// and lets the caller declare headerHeight, since a valign assertion
// needs a header box tall enough for "top"/"middle"/"bottom" to
// produce visibly different label positions. styleJSON/headerStyleJSON
// may each be "", meaning that key is omitted entirely.
func tableHeaderDocFull(styleJSON, headerStyleJSON, columnsJSON string, headerHeight int) string {
	styleField := ""
	if styleJSON != "" {
		styleField = `, "style": ` + styleJSON
	}
	headerStyleField := ""
	if headerStyleJSON != "" {
		headerStyleField = `, "headerStyle": ` + headerStyleJSON
	}
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": %d,
        "columns": %s%s%s}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"latin": ["Noto Sans"], "thai": ["Noto Sans Thai"], "cjk": ["Noto Sans SC"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`, headerHeight, columnsJSON, styleField, headerStyleField)
}

const twoColumnsNoAlign = `[
  {"id": "e2", "label": "Date", "width": 100, "bind": "{{row.a}}"},
  {"id": "e3", "label": "Amount", "width": 150, "bind": "{{row.b}}"}
]`

// TestTableHeaderNoStyleExceptFontFamilyRendersDocumentedDefaults is
// R6, restated: padding 0, no border, transparent background, align
// left, valign top — and the render succeeds (not an error, not a
// panic, AD-14).
func TestTableHeaderNoStyleExceptFontFamilyRendersDocumentedDefaults(t *testing.T) {
	doc := tableHeaderDoc(`{"fontFamily": "latin", "fontSize": 9}`, twoColumnsNoAlign)
	pages := tablePagesForTest(t, doc, `{"items": []}`)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	rects := pages[0].Rects
	if len(rects) != 2 {
		t.Fatalf("got %d rects, want 2 (one per column cell)", len(rects))
	}
	for i, r := range rects {
		if r.HasFill {
			t.Errorf("rect %d: HasFill = true, want false (no background declared)", i)
		}
		if r.HasStroke {
			t.Errorf("rect %d: HasStroke = true, want false (no border declared)", i)
		}
	}
	if rects[0].X != 0 || rects[0].W != 100000 {
		t.Errorf("rect 0: X=%d W=%d, want X=0 W=100000", rects[0].X, rects[0].W)
	}
	if rects[1].X != 100000 || rects[1].W != 150000 {
		t.Errorf("rect 1: X=%d W=%d, want X=100000 W=150000", rects[1].X, rects[1].W)
	}
	// Y is band-relative-to-page-absolute: this fixture's pageHeader
	// band is 20pt tall, so the content band's own origin is 20000, and
	// the table's own Y=0 lands at exactly that (PlaceInBand — a
	// translation, never an inversion).
	if rects[0].Y != 20000 || rects[0].H != 20000 {
		t.Errorf("rect 0: Y=%d H=%d, want Y=20000 H=20000", rects[0].Y, rects[0].H)
	}
	if len(pages[0].Runs) == 0 {
		t.Fatal("expected label glyph runs, got none")
	}
	// Left/top defaults: the first run's X equals its column's own X
	// (no padding, "left" align) and Y equals the table's own top (no
	// padding, "top" valign) — before the ruled first-baseline offset,
	// which BaselineOffset (not Y) carries (Story 2.5a's model).
	if pages[0].Runs[0].X != 0 {
		t.Errorf("first run X = %d, want 0 (left align, zero padding)", pages[0].Runs[0].X)
	}
	if pages[0].Runs[0].Y != 20000 {
		t.Errorf("first run Y = %d, want 20000 (top valign, zero padding, content band origin)", pages[0].Runs[0].Y)
	}
}

// TestTableHeaderBorderEdgesSubset is AC3's edges assertion: naming a
// strict subset strokes exactly those edges.
//
// Mutation run (recorded in the Delivery Log): hardcode all four edges
// regardless of style.border.edges — reds, asserted here by checking
// Right/Left are explicitly false.
func TestTableHeaderBorderEdgesSubset(t *testing.T) {
	// THE BORDER MOVED TO headerStyle, and that is SPEC-table-rules §1
	// rather than a test convenience: a table's own `style.border` now
	// paints the table's BOX, and `headerStyle.border` is the only
	// declaration left that puts chrome on a header cell. The assertion
	// below — a named subset strokes exactly those edges — is unchanged.
	doc := tableHeaderDocFull(
		`{"fontFamily": "latin"}`,
		`{"border": {"edges": ["bottom", "top"], "color": "#112233", "width": 1}}`,
		twoColumnsNoAlign, 20)
	pages := tablePagesForTest(t, doc, `{"items": []}`)
	for i, r := range pages[0].Rects {
		if !r.HasStroke {
			t.Fatalf("rect %d: HasStroke = false, want true", i)
		}
		if !r.Edges.Top || !r.Edges.Bottom {
			t.Errorf("rect %d: Edges = %+v, want Top and Bottom set", i, r.Edges)
		}
		if r.Edges.Left || r.Edges.Right {
			t.Errorf("rect %d: Edges = %+v, want Left and Right UNSET (edges is a strict subset)", i, r.Edges)
		}
		if r.Stroke != (pagemodel.Color{R: 0x11, G: 0x22, B: 0x33}) {
			t.Errorf("rect %d: Stroke = %+v, want #112233", i, r.Stroke)
		}
		if r.StrokeWidth != 1000 {
			t.Errorf("rect %d: StrokeWidth = %d, want 1000 (1pt)", i, r.StrokeWidth)
		}
	}
}

// TestTableHeaderBackgroundFill is AC3's background assertion.
func TestTableHeaderBackgroundFill(t *testing.T) {
	// headerStyle, for the reason TestTableHeaderBorderEdgesSubset states.
	doc := tableHeaderDocFull(`{"fontFamily": "latin"}`, `{"background": "#00FF00"}`, twoColumnsNoAlign, 20)
	pages := tablePagesForTest(t, doc, `{"items": []}`)
	for i, r := range pages[0].Rects {
		if !r.HasFill {
			t.Fatalf("rect %d: HasFill = false, want true", i)
		}
		if r.Fill != (pagemodel.Color{R: 0, G: 255, B: 0}) {
			t.Errorf("rect %d: Fill = %+v, want #00FF00", i, r.Fill)
		}
		if r.HasStroke {
			t.Errorf("rect %d: HasStroke = true, want false (no border declared)", i)
		}
	}
}

// TestTableHeaderPaddingInsetsLabel is AC3's padding assertion:
// style.padding.left shifts the label's X by exactly that amount.
//
// Mutation run (recorded in the Delivery Log): drop the padding inset
// (use cg.X instead of cg.X+padLeft) — reds, the two renders' first-run
// X becomes equal.
func TestTableHeaderPaddingInsetsLabel(t *testing.T) {
	noPadding := tablePagesForTest(t, tableHeaderDoc(`{"fontFamily": "latin"}`, twoColumnsNoAlign), `{"items": []}`)
	withPadding := tablePagesForTest(t, tableHeaderDoc(`{"fontFamily": "latin", "padding": {"left": 20}}`, twoColumnsNoAlign), `{"items": []}`)

	if len(noPadding[0].Runs) == 0 || len(withPadding[0].Runs) == 0 {
		t.Fatal("expected label runs in both renders")
	}
	got := withPadding[0].Runs[0].X - noPadding[0].Runs[0].X
	if want := int64(20000); int64(got) != want {
		t.Errorf("padding.left=20 shifted the label X by %d, want %d", got, want)
	}
}

// TestColumnAlignWinsOverStyleAlign is AC4: the first column's OWN
// align ("left") wins over style.align ("right"); the second column,
// with no align of its own, falls back to style.align.
//
// Mutation run (recorded in the Delivery Log): make style.align win
// unconditionally — reds on column 0 (its label would shift right).
// A second mutation, ignoring style.align entirely (column align the
// only source) — reds on column 1 (its label would stay flush left).
func TestColumnAlignWinsOverStyleAlign(t *testing.T) {
	cols := `[
  {"id": "e2", "label": "A", "width": 100, "align": "left", "bind": "{{row.a}}"},
  {"id": "e3", "label": "B", "width": 100, "bind": "{{row.b}}"}
]`
	doc := tableHeaderDoc(`{"fontFamily": "latin", "align": "right"}`, cols)
	// Story 4.2 review Finding 12: a NON-EMPTY collection, so this
	// precedence property is asserted over the header AND the data
	// row, never the header alone. Every run in each column bucket
	// (not merely the first one encountered) must satisfy the
	// precedence, so a data cell falling back to the WRONG source
	// cannot hide behind the header cell's own correct answer.
	pages := tablePagesForTest(t, doc, `{"items": [{"a":"1","b":"2"}]}`)

	// Column 0's cell is [0,100000); flush-left means its label X == 0.
	// Column 1's cell is [100000,200000); flush-right means its label X
	// is somewhere strictly greater than 100000 (its own cell's left
	// edge) — a right-aligned short label never starts at the cell's
	// own left edge.
	var col0Runs, col1Runs int
	for i := range pages[0].Runs {
		r := &pages[0].Runs[i]
		x := int64(r.X)
		switch {
		case x < 100000:
			col0Runs++
			if x != 0 {
				t.Errorf("column 0 (own align=left) run %d X = %d, want 0", i, x)
			}
		default:
			col1Runs++
			if x <= 100000 {
				t.Errorf("column 1 (falls back to style.align=right) run %d X = %d, want > 100000 (right-aligned)", i, x)
			}
		}
	}
	if col0Runs == 0 || col1Runs == 0 {
		t.Fatalf("expected runs in both column cells, got %d total runs", len(pages[0].Runs))
	}
	if col0Runs != 2 || col1Runs != 2 {
		t.Fatalf("expected 2 runs per column (1 header + 1 data row), got column0=%d column1=%d", col0Runs, col1Runs)
	}
}

// TestHeaderStyleBackgroundWinsOverStyle is the finisher's fix for
// Story 4.1 review Blocker 1 (Finding 2): headerStyle — this story's
// owner-ruled scope addition — was inert to every test in the tree.
// When a table declares BOTH style.background and a DIFFERENT
// headerStyle.background, the header cell must carry headerStyle's
// colour, never style's.
//
// Discriminating mutation (the reviewer's own mutation H, re-run
// below and recorded in the story): replace resolveHeaderStyle's
// hasHeader detection with `hasHeader := false` so headerStyle is
// ignored entirely at render — reds, because the rect would then
// carry style's RED background instead of headerStyle's GREEN one.
func TestHeaderStyleBackgroundWinsOverStyle(t *testing.T) {
	doc := tableHeaderDocFull(
		`{"fontFamily": "latin", "background": "#FF0000"}`,
		`{"background": "#00FF00"}`,
		twoColumnsNoAlign, 20)
	pages := tablePagesForTest(t, doc, `{"items": []}`)
	// SPEC-table-rules §1 sharpened what "wins" means here. The two
	// declarations no longer COMPETE for the header cell at all: the
	// header cell takes headerStyle's green, and style's red paints the
	// table's own frame — its fill the FIRST rect, under the two header
	// cells' fills. What
	// the test still guards is the thing it was written for: a header
	// cell never carries the table's own style.background.
	rects := pages[0].Rects
	if len(rects) != 3 {
		t.Fatalf("got %d rects, want 3 (2 header cells + the table's own box)", len(rects))
	}
	for i, r := range rects[1:] {
		if !r.HasFill {
			t.Fatalf("header rect %d: HasFill = false, want true", i)
		}
		if r.Fill != (pagemodel.Color{R: 0, G: 255, B: 0}) {
			t.Errorf("rect %d: Fill = %+v, want #00FF00 (a header cell takes headerStyle.background, never style.background=#FF0000)", i, r.Fill)
		}
	}
	if box := rects[0]; !box.HasFill || box.Fill != (pagemodel.Color{R: 255, G: 0, B: 0}) {
		t.Errorf("the table's own box: HasFill=%v Fill=%+v, want a #FF0000 fill — style.background paints the box", box.HasFill, box.Fill)
	}
}

// TestHeaderStyleCascadesPerField is Blocker 1's second half: the
// cascade is PER FIELD, not "headerStyle present means style is
// ignored entirely". A table whose headerStyle sets ONLY border and
// whose style sets ONLY padding must render with headerStyle's border
// AND style's padding — the header falls through to style field by
// field, exactly as resolveHeaderStyle's doc comment claims.
func TestHeaderStyleCascadesPerField(t *testing.T) {
	withHeaderStyle := tablePagesForTest(t, tableHeaderDocFull(
		`{"fontFamily": "latin", "padding": {"left": 20}}`,
		`{"border": {"edges": ["bottom"], "color": "#112233", "width": 1}}`,
		twoColumnsNoAlign, 20), `{"items": []}`)
	withoutPadding := tablePagesForTest(t, tableHeaderDocFull(
		`{"fontFamily": "latin"}`,
		`{"border": {"edges": ["bottom"], "color": "#112233", "width": 1}}`,
		twoColumnsNoAlign, 20), `{"items": []}`)

	// headerStyle's border must be drawn (headerStyle sets it, style
	// does not).
	for i, r := range withHeaderStyle[0].Rects {
		if !r.HasStroke {
			t.Fatalf("rect %d: HasStroke = false, want true (headerStyle.border)", i)
		}
		if r.Stroke != (pagemodel.Color{R: 0x11, G: 0x22, B: 0x33}) {
			t.Errorf("rect %d: Stroke = %+v, want #112233 (from headerStyle.border, not a table default)", i, r.Stroke)
		}
	}
	// style's padding must ALSO apply — falling through per field,
	// rather than headerStyle's presence blanking style entirely.
	if len(withHeaderStyle[0].Runs) == 0 || len(withoutPadding[0].Runs) == 0 {
		t.Fatal("expected label runs in both renders")
	}
	got := withHeaderStyle[0].Runs[0].X - withoutPadding[0].Runs[0].X
	if want := int64(20000); int64(got) != want {
		t.Errorf("style.padding.left=20 shifted the label X by %d, want %d — headerStyle setting ONLY border must not suppress style's padding (per-field cascade)", got, want)
	}
}

// TestColumnAlignWinsOverHeaderStyleAlign extends AC4's precedence one
// level (the owner ruling's own words: "columns[].align still wins
// over both"): a column's OWN align wins even when headerStyle (not
// just style) sets a conflicting align.
func TestColumnAlignWinsOverHeaderStyleAlign(t *testing.T) {
	cols := `[
  {"id": "e2", "label": "A", "width": 100, "align": "left", "bind": "{{row.a}}"},
  {"id": "e3", "label": "B", "width": 100, "bind": "{{row.b}}"}
]`
	doc := tableHeaderDocFull(`{"fontFamily": "latin"}`, `{"align": "right"}`, cols, 20)
	// Story 4.2 review Finding 12: a NON-EMPTY collection. This also
	// doubles as an AC5 witness: headerStyle.align governs the HEADER
	// row's column-1 fallback only — a data cell's fallback comes from
	// `style.align` alone (absent here, so it defaults to "left") and
	// must NOT inherit headerStyle's "right", per D-000.76/AC5. Runs
	// are emitted header-first then data-row (collectBandTableRuns's
	// own order), so Runs[0:2] are the header's and Runs[2:4] the data
	// row's.
	pages := tablePagesForTest(t, doc, `{"items": [{"a":"1","b":"2"}]}`)
	if len(pages[0].Runs) != 4 {
		t.Fatalf("got %d runs, want 4 (2 header + 2 data row)", len(pages[0].Runs))
	}
	headerCol0X, headerCol1X := int64(pages[0].Runs[0].X), int64(pages[0].Runs[1].X)
	dataCol0X, dataCol1X := int64(pages[0].Runs[2].X), int64(pages[0].Runs[3].X)

	if headerCol0X != 0 {
		t.Errorf("header column 0 (own align=left) label X = %d, want 0 — must win over headerStyle.align=right", headerCol0X)
	}
	if headerCol1X <= 100000 {
		t.Errorf("header column 1 (falls back to headerStyle.align=right) label X = %d, want > 100000 (right-aligned)", headerCol1X)
	}
	if dataCol0X != 0 {
		t.Errorf("data column 0 (own align=left) X = %d, want 0 — column align still wins for data cells", dataCol0X)
	}
	if dataCol1X != 100000 {
		t.Errorf("data column 1 X = %d, want 100000 (style.align default \"left\", NEVER headerStyle.align=right — AC5/D-000.76)", dataCol1X)
	}
}

// TestTableHeaderValignPlacement is the finisher's fix for Story 4.1
// review Blocker 2 (Finding 3): style.valign reached no assertion
// anywhere in the tree, and the byte-identity assertion in AC7 was
// vacuous for that field. This renders the SAME table three times,
// varying only style.valign, with a header box (headerHeight=100pt)
// large relative to a 9pt label's own line height, and asserts the
// three renders place the label at three DISTINCT, strictly ordered Y
// positions: top < middle < bottom.
//
// "top" is asserted against a test-owned literal (20000 — the content
// band's own origin in this fixture, zero padding: the same literal
// TestTableHeaderNoStyleExceptFontFamilyRendersDocumentedDefaults
// already pins for the documented default). "middle" and "bottom" are
// asserted RELATIONALLY against "top" and each other (D-000.68: the
// text block's own height in thousandths is a font-metrics fact this
// test does not hardcode, so it anchors to strict ordering plus a
// symmetry bound instead of duplicating resolveHeaderStyle's own
// arithmetic) — both anchors the code under test cannot move by
// re-deriving its own answer.
//
// Discriminating mutation (the reviewer's own mutation G, re-run below
// and recorded in the story): delete the valign cascade so
// resolveHeaderStyle's r.valign is permanently "top" — reds, because
// "middle" and "bottom" then collapse onto the SAME Y as "top".
func TestTableHeaderValignPlacement(t *testing.T) {
	const headerHeight = 100 // pt — large relative to a 9pt label's line height
	build := func(valign string) geom.Length {
		doc := tableHeaderDocFull(
			fmt.Sprintf(`{"fontFamily": "latin", "fontSize": 9, "valign": %q}`, valign),
			"", twoColumnsNoAlign, headerHeight)
		pages := tablePagesForTest(t, doc, `{"items": []}`)
		if len(pages[0].Runs) == 0 {
			t.Fatalf("valign=%s: expected label runs, got none", valign)
		}
		return pages[0].Runs[0].Y
	}
	top := build("top")
	middle := build("middle")
	bottom := build("bottom")

	if top != 20000 {
		t.Errorf("valign=top: Y = %d, want 20000 (content band origin, zero padding — the documented default's own literal)", top)
	}
	if !(top < middle && middle < bottom) {
		t.Fatalf("valign placements are not strictly ordered top < middle < bottom: got top=%d middle=%d bottom=%d", top, middle, bottom)
	}
	// Symmetry bound: "middle" must sit within 1 thousandth-unit of the
	// midpoint between "top" and "bottom" (round-half-to-even's own
	// +/-1 tolerance) — this is what distinguishes a genuine midpoint
	// placement from some other value that merely happens to fall
	// between the two.
	midpoint := (int64(top) + int64(bottom)) / 2
	if d := int64(middle) - midpoint; d < -1 || d > 1 {
		t.Errorf("valign=middle: Y = %d, want within 1 of the top/bottom midpoint %d", middle, midpoint)
	}
}

// TestAMalformedTableColourIsALoadError: a malformed table colour is a located
// LOAD error, even on a table with no rows to draw.
func TestAMalformedTableColourIsALoadError(t *testing.T) {
	doc := tableHeaderDoc(`{"fontFamily": "latin", "background": "not-a-colour"}`, twoColumnsNoAlign)
	requireColourLoadError(t, doc, "e1", "style.background")
}

// TestTableHeaderNoFontFamilyIsLocatedError is R6's own negative half:
// a table with a non-empty label and no resolvable style.fontFamily
// (nor headerStyle.fontFamily) fails the render — the SAME failure
// mode a text element with the same omission already has.
func TestTableHeaderNoFontFamilyIsLocatedError(t *testing.T) {
	doc := tableHeaderDoc(``, twoColumnsNoAlign)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	_, rerr := Render(tpl, Data(`{"items": []}`), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("expected a render error: a table with no style at all and non-empty labels needs a resolvable font")
	}
}

// TestTableHeaderVisibleIfFalseIsAbsentFromPageModel is AC8's
// behavioural half: a hidden table contributes no header row, no
// borders, no background — absent from the page model entirely.
//
// Mutation run (recorded in the Delivery Log): render the table
// unconditionally, ignoring its visibility verdict — reds (Rects/Runs
// would be non-empty).
func TestTableHeaderVisibleIfFalseIsAbsentFromPageModel(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 20,
        "visibleIf": "customer.hasItems",
        "style": {"fontFamily": "latin", "background": "#FF0000"},
        "columns": ` + twoColumnsNoAlign + `}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	visible := tablePagesForTest(t, doc, `{"customer": {"hasItems": true}, "items": []}`)
	if len(visible[0].Rects) == 0 || len(visible[0].Runs) == 0 {
		t.Fatal("control: a visible table must produce rects and runs")
	}
	hidden := tablePagesForTest(t, doc, `{"customer": {"hasItems": false}, "items": []}`)
	if len(hidden[0].Rects) != 0 {
		t.Errorf("hidden table produced %d rects, want 0", len(hidden[0].Rects))
	}
	if len(hidden[0].Runs) != 0 {
		t.Errorf("hidden table produced %d runs, want 0", len(hidden[0].Runs))
	}
}

// TestTableRendersOneRowPerCollectionElementInDataOrder is AC1,
// rewriting TestTableRendersZeroDataRows (4.1's own doc comment marked
// it "expected to be REWRITTEN by Story 4.2" — this is that rewrite,
// kept rather than deleted per this story's own obligation list).
//
// Anchor (D-000.68): a five-element collection whose cell values are
// five DISTINCT, test-owned strings; expected run count =
// columns * (1 header + 5 rows) since every cell here is single-line;
// expected rect count = columns * (1 header row + 5 data rows), one
// rect per column per row (DECISION-1, ruled: data rows get cell
// chrome too); and the ORDERED list of SourceText values compared
// against the test's own literal slice — data order is asserted by
// comparing the SEQUENCE, never a set, since a set assertion cannot
// see a reversal.
//
// Vacuity fence: the collection is non-empty by construction (five
// distinct values), so "in data order" is a positive claim, not an
// empty-collection accident (no AC in this story may be satisfied by
// asserting on an empty collection).
func TestTableRendersOneRowPerCollectionElementInDataOrder(t *testing.T) {
	doc := tableHeaderDoc(`{"fontFamily": "latin"}`, twoColumnsNoAlign)
	pages := tablePagesForTest(t, doc, `{"items": [
		{"a":"r0a","b":"r0b"}, {"a":"r1a","b":"r1b"}, {"a":"r2a","b":"r2b"},
		{"a":"r3a","b":"r3b"}, {"a":"r4a","b":"r4b"}
	]}`)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	const columns = 2
	const rows = 5
	if got, want := len(pages[0].Rects), columns*(1+rows); got != want {
		t.Errorf("got %d rects, want %d (%d columns * (1 header row + %d data rows))", got, want, columns, rows)
	}
	if got, want := len(pages[0].Runs), columns*(1+rows); got != want {
		t.Fatalf("got %d runs, want %d (%d columns * (1 header row + %d data rows), one line each)", got, want, columns, rows)
	}

	// Data order: every run's SourceText, in emission order, must be
	// exactly the header labels followed by the rows' own values IN THE
	// COLLECTION'S OWN ORDER — a reversed row loop would still produce
	// the same SET of strings, which is why this is a sequence
	// comparison, never a set.
	want := []string{"Date", "Amount"}
	for i := 0; i < rows; i++ {
		want = append(want, fmt.Sprintf("r%da", i), fmt.Sprintf("r%db", i))
	}
	// len(pages[0].Runs) == len(want) is already guaranteed by the
	// run-count Fatalf above (both equal columns*(1+rows) by
	// construction), so it is not re-checked here (Story 4.2 review
	// Finding 19: the duplicate check could never fire).
	for i, r := range pages[0].Runs {
		if r.SourceText != want[i] {
			t.Errorf("run %d: SourceText = %q, want %q (data order)", i, r.SourceText, want[i])
		}
	}
}

// TestTableStyleFieldsAreNotDataDriven is AC10 (originally AC7 Part
// A), for the fields this story wires on DATA CELLS too
// (border/padding/background/align/valign): two report-data documents
// differing ONLY in a field the template does not bind (exactly the
// shape a conditional-formatting implementation would key on) must
// produce byte-identical output.
//
// Story 4.2 correction (D4, this story's own creation record): the
// ORIGINAL version of this test rendered "items": [] — an empty
// collection — so it compared two HEADER-ONLY documents forever, even
// after this story ships row output, while claiming a property about
// a table WITH rows. That is 4.1 review's Blockers 1/2 shape exactly.
// AC10 requires a NON-EMPTY collection, identical in both renders,
// with cell binds resolving REAL row fields (row.a/row.b) and
// "overdue" still unbound — so this test now actually exercises the
// row style cascade (resolveBodyStyle, buildCellRect) it claims to
// cover, not merely the header's.
func TestTableStyleFieldsAreNotDataDriven(t *testing.T) {
	doc := tableHeaderDoc(`{"fontFamily": "latin", "border": {"edges": ["bottom"], "color": "#112233", "width": 1},
		"padding": {"left": 5, "top": 2}, "background": "#EFEFEF", "align": "right", "valign": "middle"}`, twoColumnsNoAlign)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	const items = `[{"a":"x1","b":"y1"},{"a":"x2","b":"y2"},{"a":"x3","b":"y3"}]`
	resTrue, err := Render(tpl, Data(`{"items": `+items+`, "overdue": true}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render(overdue=true): %v", err)
	}
	resFalse, err := Render(tpl, Data(`{"items": `+items+`, "overdue": false}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render(overdue=false): %v", err)
	}
	if string(resTrue.Bytes) != string(resFalse.Bytes) {
		t.Fatal("table style output differs between two datasets differing only in an unbound field — style must never be data-driven (AC10)")
	}
}

// TestColumnGeometryNeverNegotiatesAgainstLabelContent is AC2's
// end-to-end proof: two templates identical in every byte except their
// MIDDLE column's label string — narrow vs a long Latin label, a long
// Thai label with no spaces, and a long CJK label (all three shaping
// paths, AC2's own requirement) — must produce IDENTICAL column
// x-origins, widths and the table's total width (summed over its
// rects — TableGeometry itself is not reachable from the page model).
// The wide-label render's header text overflows its (padded) cell box
// — asserted via ClipToBox — and NEVER widens the column, and never
// shifts a LATER column's origin (AC2's own "never negotiated"
// property has no way to show up on a single-column fixture: with
// three columns, a widened middle column would visibly push column
// 2's origin).
//
// Finisher fix (Story 4.1 review Finding 8, Minor): the ORIGINAL
// version of this test (a) used a single column, so no x-origin
// PROPAGATION was exercised — the one column's X is el.X and cannot
// move under any defect; (b) never asserted the table's total width;
// (c) used the WIDE render's own output as the NARROW render's
// "expected" value (and vice versa) rather than a literal the test
// owns, which is AC2's own stated anchor requirement. All three are
// fixed below: three columns, TEST-OWNED literal expected {X,W} pairs
// for each (read from neither render), and a literal expected total.
//
// Anchor: the three columns' widths (60/60/60) and the expected X/W
// pairs and total they imply are literals this test owns — the SAME
// literals for both the narrow and the wide render, since AC2's claim
// is that content can NEVER move them (D-000.68).
//
// Control (D-000.9): narrow and wide renders must differ in their
// glyph output — asserted via run count/SourceText — or the geometry
// equality above would be vacuous (both could have rendered nothing).
// threeColumnTableDoc is tableHeaderDoc's shape with "nextId" raised to
// 5 (AD-10/AC37: nextId must exceed the highest element id present),
// since this test's three columns claim ids up to "e4".
func threeColumnTableDoc(styleJSON, columnsJSON string) string {
	styleField := ""
	if styleJSON != "" {
		styleField = `, "style": ` + styleJSON
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 20,
        "columns": ` + columnsJSON + styleField + `}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"latin": ["Noto Sans"], "thai": ["Noto Sans Thai"], "cjk": ["Noto Sans SC"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

func TestColumnGeometryNeverNegotiatesAgainstLabelContent(t *testing.T) {
	cases := []struct {
		name   string
		narrow string
		wide   string
	}{
		{"latin", "N", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{"thai", "น", "กขคงจฉชซฌญฎฏฐฑฒณดตถทธนบปผฝพฟภมยรลวศษสหฬอฮกขคงจฉชซฌญฎฏฐฑฒณดตถทธนบปผฝพฟภมยรลวศษสหฬอฮ"},
		{"cjk", "日", "日本語漢字書体文書作成印刷組版技術情報処理装置画面表示解像度改善対応方法検討委員会報告書提出期限厳守"},
	}
	// TEST-OWNED literal expected geometry for the three 60pt columns —
	// identical for both the narrow and wide render, since AC2 claims
	// content can never move it.
	type wantRect struct{ x, w int64 }
	want := []wantRect{{0, 60000}, {60000, 60000}, {120000, 60000}}
	const wantTotal = int64(180000)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fontFamily := c.name
			cols := func(middleLabel string) string {
				return `[
  {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"},
  {"id": "e3", "label": "` + middleLabel + `", "width": 60, "bind": "{{row.b}}"},
  {"id": "e4", "label": "B", "width": 60, "bind": "{{row.c}}"}
]`
			}
			narrowDoc := threeColumnTableDoc(`{"fontFamily": "`+fontFamily+`"}`, cols(c.narrow))
			wideDoc := threeColumnTableDoc(`{"fontFamily": "`+fontFamily+`"}`, cols(c.wide))

			// Story 4.2 review Finding 12: a NON-EMPTY collection,
			// IDENTICAL in both the narrow and wide render, so this
			// test — named in AC2 as its own model — exercises the
			// property over a table WITH a data row, not the header
			// alone. The row's own values are short and identical in
			// both renders, so they contribute no wrapping of their
			// own; only the header LABEL varies.
			const items = `{"items": [{"a":"1","b":"2","c":"3"}]}`
			narrowPages := tablePagesForTest(t, narrowDoc, items)
			widePages := tablePagesForTest(t, wideDoc, items)

			for _, pages := range []struct {
				label string
				pages []pagemodel.Page
			}{{"narrow", narrowPages}, {"wide", widePages}} {
				if len(pages.pages[0].Rects) != 6 {
					t.Fatalf("%s/%s: expected exactly 6 rects (3 header + 3 data row), got %d", c.name, pages.label, len(pages.pages[0].Rects))
				}
				// Both the header's rect group and the data row's rect
				// group must show the SAME test-owned {X,W} geometry —
				// a widened column would move both.
				for group := 0; group < 2; group++ {
					var total int64
					for i := 0; i < 3; i++ {
						r := pages.pages[0].Rects[group*3+i]
						if int64(r.X) != want[i].x || int64(r.W) != want[i].w {
							t.Errorf("%s/%s: group %d column %d geometry = {X:%d,W:%d}, want {X:%d,W:%d} — content must never move it", c.name, pages.label, group, i, r.X, r.W, want[i].x, want[i].w)
						}
						total += int64(r.W)
					}
					if total != wantTotal {
						t.Errorf("%s/%s: group %d table total width (summed rects) = %d, want %d", c.name, pages.label, group, total, wantTotal)
					}
				}
			}

			// Control: the two renders must actually differ in glyph
			// output (more glyphs for the wide label), or the equality
			// above proves nothing.
			if len(widePages[0].Runs) == 0 || len(narrowPages[0].Runs) == 0 {
				t.Fatalf("%s: expected runs in both renders", c.name)
			}
			totalWideGlyphs, totalNarrowGlyphs := 0, 0
			for _, r := range widePages[0].Runs {
				totalWideGlyphs += len(r.Glyphs)
			}
			for _, r := range narrowPages[0].Runs {
				totalNarrowGlyphs += len(r.Glyphs)
			}
			if totalWideGlyphs <= totalNarrowGlyphs {
				t.Fatalf("%s: control failed — wide label produced %d glyphs, narrow produced %d; the two renders must differ", c.name, totalWideGlyphs, totalNarrowGlyphs)
			}

			// The wide label NEVER WIDENS THE COLUMN — that is the
			// assertion above, and it is untouched by SPEC-table-rules.
			// What the label does INSIDE the column changed: it WRAPS
			// (§4, through the body cell's packer) instead of being
			// clipped in silence. So a clip is no longer required, and
			// where it still happens — a run with no break opportunity
			// narrow enough, which is the latin and thai cases here —
			// it must be bounded by the column's own width AND reported,
			// which is the half that was missing before.
			wideTpl, terr := ParseTemplate([]byte(wideDoc))
			if terr != nil {
				t.Fatalf("%s: ParseTemplate(wideDoc): %v", c.name, terr)
			}
			wideRes, rerr := Render(wideTpl, Data(items), nil, testShippedFontSet())
			if rerr != nil {
				t.Fatalf("%s: Render(wideDoc): %v", c.name, rerr)
			}
			wideDiags := wideRes.Diagnostics

			clipped, warned := false, 0
			for _, r := range widePages[0].Runs {
				if r.ClipToBox {
					clipped = true
					if r.ClipWidth != 60000 {
						t.Errorf("%s: ClipWidth = %d, want 60000 (the column's own width)", c.name, r.ClipWidth)
					}
				}
			}
			for _, d := range wideDiags {
				if d.Code == DiagCodeTextClippedWidth {
					warned++
				}
			}
			if clipped && warned == 0 {
				t.Errorf("%s: the wide label was clipped and NO TEXT_CLIPPED_WIDTH warning was emitted — the silent header clip is what SPEC-table-rules retired", c.name)
			}
			if !clipped && warned != 0 {
				t.Errorf("%s: nothing was clipped but %d TEXT_CLIPPED_WIDTH warning(s) were emitted", c.name, warned)
			}
			t.Logf("%s: wide label clipped=%v, warnings=%d", c.name, clipped, warned)
		})
	}
}

// TestTableStyleFieldsAreNotDataDrivenControl is D-000.9's vacuity
// control for the test above: it proves the byte-comparison mechanism
// itself is sensitive at all — changing an ACTUAL style field (never
// data) between two renders of the SAME template structure must
// produce DIFFERENT bytes. Without this, a byte-identical assertion
// could pass merely because nothing in the pipeline ever varies.
//
// Finisher fix (Story 4.1 review Finding 9, Minor): the ORIGINAL
// version of this control varied ONLY border.color, proving the
// byte-comparison is sensitive to exactly one of AC7's five fields.
// The other four (padding/background/align/valign) were independently
// observed elsewhere (TestTableHeaderPaddingInsetsLabel etc.), but
// TestTableStyleFieldsAreNotDataDriven's OWN control did not cover
// them — so a regression that made ONE of those four fields silently
// stop reaching output (as valign's did, Blocker 2) would not have
// been caught by this control even though it would have made that
// field's slice of the byte-identity assertion vacuous. This is now a
// table over all five fields: each varies ONE field between two
// otherwise-identical renders and asserts the bytes differ.
func TestTableStyleFieldsAreNotDataDrivenControl(t *testing.T) {
	cases := []struct {
		name string
		a, b string // style JSON, differing in exactly one field
	}{
		{"border", `{"fontFamily": "latin", "border": {"edges": ["bottom"], "color": "#FF0000", "width": 1}}`,
			`{"fontFamily": "latin", "border": {"edges": ["bottom"], "color": "#00FF00", "width": 1}}`},
		{"padding", `{"fontFamily": "latin", "padding": {"left": 5}}`,
			`{"fontFamily": "latin", "padding": {"left": 25}}`},
		{"background", `{"fontFamily": "latin", "background": "#FF0000"}`,
			`{"fontFamily": "latin", "background": "#00FF00"}`},
		{"align", `{"fontFamily": "latin", "align": "left"}`,
			`{"fontFamily": "latin", "align": "right"}`},
		{"valign", `{"fontFamily": "latin", "valign": "top"}`,
			`{"fontFamily": "latin", "valign": "bottom"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			docA := tableHeaderDoc(c.a, twoColumnsNoAlign)
			docB := tableHeaderDoc(c.b, twoColumnsNoAlign)
			tplA, err := ParseTemplate([]byte(docA))
			if err != nil {
				t.Fatalf("ParseTemplate(a): %v", err)
			}
			tplB, err := ParseTemplate([]byte(docB))
			if err != nil {
				t.Fatalf("ParseTemplate(b): %v", err)
			}
			const items = `[{"a":"x1","b":"y1"},{"a":"x2","b":"y2"}]`
			resA, err := Render(tplA, Data(`{"items": `+items+`}`), nil, testShippedFontSet())
			if err != nil {
				t.Fatalf("Render(a): %v", err)
			}
			resB, err := Render(tplB, Data(`{"items": `+items+`}`), nil, testShippedFontSet())
			if err != nil {
				t.Fatalf("Render(b): %v", err)
			}
			if string(resA.Bytes) == string(resB.Bytes) {
				t.Fatalf("control failed: two templates differing only in %s produced IDENTICAL bytes — the comparison mechanism cannot distinguish this field", c.name)
			}
		})
	}
}

// TestTableInPageHeaderRepeatsIdenticallyAcrossPages is the finisher's
// fix for Story 4.1 review Finding 12 (Minor): table rects in the
// page-header/page-footer bands were new, entirely uncovered code
// (render.go's `case pageHeaderBandIndex`/`case pageFooterBandIndex`
// arms, and page_number.go's parallel `contentColumnItems` filter) —
// no test placed a `table` element in either band, so a defect routing
// those rects through the CONTENT path instead (subject to pagination
// Shift, and appearing on only ONE page) went undetected.
//
// This renders a two-page document (a content text element long
// enough to overflow one page, the same technique
// multi_page_composition_test.go uses) with a small table in the
// pageHeader band, and asserts the SAME rect geometry — X, Y, W, H —
// appears on BOTH pages, with no Shift applied (a page-header/footer
// item is placed identically on every page; only CONTENT items slide
// by the pagination window's Shift).
//
// Discriminating mutation (the reviewer's own mutation I, re-run and
// recorded in the story): route header/footer-band table rects into
// `items` instead of `header.Rects`/`footer.Rects` — reds, because the
// header table would then appear on at most one page (assigned to
// whichever page its Y happens to fall on) instead of both.
//
// MUST NOT BE "FIXED" (Story 4.3, "things the schema and the record
// could not resolve" note 3). A table declared in the pageHeader/
// pageFooter band is repeated VERBATIM on every page and never
// paginates — that is BandContent's own contract (D-2.6.1/AD-24), and
// its rows sit entirely outside Story 4.3's grouping. A future story
// that makes a header/footer table "paginate like a content table"
// would have to change this test on purpose; it must never go red as a
// side effect of touching row-atomicity code.
func TestTableInPageHeaderRepeatsIdenticallyAcrossPages(t *testing.T) {
	const sentence = "The quick brown fox jumps over the lazy dog. "
	// 25 repetitions: multi_page_composition_test.go's own measured
	// boundaries put this comfortably inside the two-page window (20
	// renders one page's worth plus a little; the next page-count
	// boundary is 30) — the SAME geometry/font as that file, reused
	// rather than re-derived, so this test does not need its own
	// page-count arithmetic.
	value := ""
	for i := 0; i < 25; i++ {
		value += sentence
	}
	doc := fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e2", "type": "text", "x": 0, "y": 0, "width": 480, "height": 700, "value": %q, "style": {"fontFamily": "body", "fontSize": 24}}
    ]},
    "pageFooter": {"elements": [], "height": 24},
    "pageHeader": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "headerHeight": 15,
        "style": {"fontFamily": "body", "background": "#00FF00"},
        "columns": [{"id": "e3", "label": "H", "width": 60, "bind": "{{row.n}}"}]}
    ], "height": 18}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`, value)
	pages := tablePagesForTest(t, doc, `{"rows": []}`)
	if len(pages) < 2 {
		t.Fatalf("presence precondition: got %d page(s), want at least 2 (this test needs a second page to prove the header repeats)", len(pages))
	}
	// Two rects: the header row's single column cell, and the table's own
	// BOX (SPEC-table-rules §1 — the `style.background` declared below
	// paints it, and no longer the cell). BOTH must repeat identically,
	// which is a strictly stronger statement of the same property.
	for i, p := range pages[:2] {
		if len(p.Rects) != 2 {
			t.Fatalf("page %d: got %d rects, want 2 (the pageHeader table's single column cell and the table's own box)", i, len(p.Rects))
		}
	}
	for i := range pages[0].Rects {
		r0, r1 := pages[0].Rects[i], pages[1].Rects[i]
		if r0 != r1 {
			t.Errorf("pageHeader table rect %d differs between page 1 and page 2: page1=%+v page2=%+v — a page-header item must repeat IDENTICALLY (no pagination Shift applies to it)", i, r0, r1)
		}
	}
	if !pages[0].Rects[0].HasFill {
		t.Fatal("presence precondition: the table's own box must carry HasFill (style.background is declared)")
	}
}

// mandatoryBreakTableDoc is one document holding the SAME bound value
// twice: once in a text element and once in a table cell, in boxes of
// the same declared width. It is the subject of D-7.1.3 — one packer,
// one mandatory-break rule, every caller.
func mandatoryBreakTableDoc() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 180, "height": 40, "value": "{{note}}", "style": {"fontFamily": "latin", "fontSize": 8}},
      {"id": "e2", "type": "table", "x": 0, "y": 60, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e3", "label": "A", "width": 180, "bind": "{{row.a}}"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// tableCellPhysicalLines counts the DISTINCT physical lines a data row
// occupies, read off the runs the table renderer actually produced —
// never off cellResult, which is internal to the function under test.
func tableCellPhysicalLines(t *testing.T, tplJSON, dataJSON string, rowIndex int) int {
	t.Helper()
	_, contentRuns, _ := paginateContentTableForTest(t, tplJSON, dataJSON)
	seen := map[int]bool{}
	for _, r := range contentRuns {
		if r.isTableRowLine && r.rowIndex == rowIndex {
			seen[r.lineIndex] = true
		}
	}
	return len(seen)
}

// TestTableCellAndTextElementBreakTheSameValueIdentically is D-7.1.3:
// the change lands in the SHARED packer, so a line feed in a table
// cell's bound data breaks that cell exactly as it breaks a text
// element's. table_render.go's own comment already declares it is "the
// SAME packer text elements use"; this asserts the consequence.
//
// The text element's count is pinned to a LITERAL, and the cell's is
// compared against the text element's, so the two live computations
// cannot drift together into agreeing on a wrong answer.
//
// THE NEGATIVE CONTROL IS THE MEASUREMENT. The same value with the line
// feed replaced by a space must be ONE line on BOTH sides, in the same
// boxes: without it, "2 lines" could not be told from a value that was
// simply too wide for its box, and the assertion would hold for a packer
// that ignores mandatory breaks entirely.
func TestTableCellAndTextElementBreakTheSameValueIdentically(t *testing.T) {
	const broken = "first\nsecond word"
	const unbroken = "first second word"

	tpl, err := ParseTemplate([]byte(mandatoryBreakTableDoc()))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dataFor := func(v string) string {
		return fmt.Sprintf(`{"note": %q, "items": [{"a": %q}]}`, v, v)
	}

	// NEGATIVE CONTROL FIRST: with a space in place of the line feed,
	// the value FITS both boxes and both sides are one line. That is
	// what makes the broken case's 2 a consequence of the break rather
	// than of the width.
	controlText := elementLayoutByID(t, elementLayouts(t, tpl, dataFor(unbroken)), "e1")
	if len(controlText.lines) != 1 {
		t.Fatalf("presence precondition: %q occupies %d line(s) in a %d mp box — it must FIT, or the broken case below proves nothing about the break", unbroken, len(controlText.lines), controlText.box)
	}
	if got := tableCellPhysicalLines(t, mandatoryBreakTableDoc(), dataFor(unbroken), 0); got != 1 {
		t.Fatalf("presence precondition: the table cell holding %q occupies %d physical line(s), want 1", unbroken, got)
	}

	// THE SUBJECT.
	brokenText := elementLayoutByID(t, elementLayouts(t, tpl, dataFor(broken)), "e1")
	const wantLines = 2 // stated as a literal, ahead of both measurements
	if len(brokenText.lines) != wantLines {
		t.Fatalf("text element e1 holding %q occupies %d line(s), want %d", broken, len(brokenText.lines), wantLines)
	}
	cellLines := tableCellPhysicalLines(t, mandatoryBreakTableDoc(), dataFor(broken), 0)
	if cellLines != len(brokenText.lines) {
		t.Errorf("the table cell holding %q occupies %d physical line(s) but the text element holding the same value occupies %d — one packer, one mandatory-break rule, every caller (D-7.1.3)", broken, cellLines, len(brokenText.lines))
	}
	t.Logf("D-7.1.3: %q -> %d lines in a text element and %d physical lines in a table cell; the control %q -> 1 and 1", broken, len(brokenText.lines), cellLines, unbroken)
}

// multiRowTableDataWithBreak builds `rows` bound rows in which breakRow
// carries a TYPED LINE FEED rather than a value too long for its column
// — so the row occupies several physical lines for the reason Story 7.1
// introduces, and row atomicity is re-asserted against that cause.
func multiRowTableDataWithBreak(t *testing.T, rows, breakRow int) string {
	t.Helper()
	items := make([]tableRowJSON, rows)
	for i := range items {
		marker := fmt.Sprintf("R%dW-", i)
		a := marker + "x"
		if i == breakRow {
			a = marker + "Alpha\n" + marker + "Bravo\n" + marker + "Charlie"
		}
		items[i] = tableRowJSON{A: a, B: marker + "b"}
	}
	b, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		t.Fatalf("marshal table data: %v", err)
	}
	return string(b)
}

// TestDataRowWithATypedBreakIsNeverSplitAcrossPages re-asserts, rather
// than assumes, the property D-7.1.3 warned about: a line feed in bound
// data changes a cell's line count, hence its row height, hence where
// the table breaks — and that must NEVER become a way to split a row
// across a page boundary. The property is held by code this story does
// not touch, which is exactly when it breaks.
func TestDataRowWithATypedBreakIsNeverSplitAcrossPages(t *testing.T) {
	const rows = 20
	const breakRow = 5
	plan, contentRuns, _ := paginateContentTableForTest(t, multiRowTableDoc(false), multiRowTableDataWithBreak(t, rows, breakRow))

	if len(plan.Pages) < 2 {
		t.Fatalf("presence precondition: the fixture must paginate to >= 2 pages, got %d — a row cannot be split across a boundary that does not exist", len(plan.Pages))
	}
	brokenLines := map[int]bool{}
	for _, r := range contentRuns {
		if r.isTableRowLine && r.rowIndex == breakRow {
			brokenLines[r.lineIndex] = true
		}
	}
	if len(brokenLines) < 3 {
		t.Fatalf("presence precondition: row %d carries two typed breaks and must occupy >= 3 physical lines, got %d — the break is not reaching the table packer at all", breakRow, len(brokenLines))
	}

	linePages := map[int]map[int]bool{}
	checked := 0
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRuns {
			r := contentRuns[ref]
			if !r.isTableRowLine {
				continue
			}
			if linePages[r.rowIndex] == nil {
				linePages[r.rowIndex] = map[int]bool{}
			}
			linePages[r.rowIndex][p] = true
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("presence precondition: no table-row-line runs were examined")
	}
	for row, pages := range linePages {
		if len(pages) != 1 {
			t.Errorf("row %d's physical lines are spread across %d distinct pages — a row's lines must all land on ONE page, whatever made the row tall", row, len(pages))
		}
	}
	if len(linePages[breakRow]) != 1 {
		t.Errorf("the row carrying the typed breaks occupies %d pages", len(linePages[breakRow]))
	}
	t.Logf("D-7.1.3: row %d occupies %d physical lines from typed breaks alone, across %d pages, and every one of the %d examined row-line runs stayed on its row's single page", breakRow, len(brokenLines), len(linePages[breakRow]), checked)
}

// TestFooterCellTextCannotCarryATypedBreak discharges the intent
// contract's "both table-cell paths" for the FOOTER path — by
// construction, because a test that fed it a line feed cannot be
// written.
//
// A footer cell's text is never bound data. It is always an aggregate
// (`sum`, `count`, `avg`) rendered through `footerFormat`, whose pattern
// is a CLOSED grammar of '#', '0', ',' and one '.'. There is no route by
// which a U+000A reaches that cell, so the footer path's mandatory-break
// behaviour is vacuously correct today — and this test is what makes
// that a checked claim rather than an assumption.
//
// IT IS ALSO THE TRIPWIRE. If the pattern grammar is ever widened to
// admit literal text, this test reddens and says so: the footer cell
// path would then need the same `\n` coverage the body path carries.
func TestFooterCellTextCannotCarryATypedBreak(t *testing.T) {
	doc := func(footerFormat string) string {
		return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 180, "bind": "{{formatNumber(row.a, \"#,##0\")}}", "footer": "sum", "footerOf": "items.a", "footerFormat": %q}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`, footerFormat)
	}
	const dataJSON = `{"items":[{"a":1},{"a":2}]}`

	// PRESENCE PRECONDITION: the well-formed pattern renders, so the
	// refusal below is about the line feed and not about the document.
	ok, err := ParseTemplate([]byte(doc("#,##0")))
	if err != nil {
		t.Fatalf("precondition: the control template must parse, got %v", err)
	}
	if _, err := Render(ok, Data(dataJSON), nil, testShippedFontSet()); err != nil {
		t.Fatalf("precondition: the control template must render, got %v", err)
	}

	// THE SUBJECT: the only field on the footer path that carries author
	// text at all, given a line feed.
	tpl, err := ParseTemplate([]byte(doc("Total\n#,##0")))
	if err == nil {
		_, err = Render(tpl, Data(dataJSON), nil, testShippedFontSet())
	}
	if err == nil {
		t.Fatal("a footerFormat carrying a line feed was accepted — the footer cell path can now receive a U+000A, and it needs the same mandatory-break coverage the body path carries (D-7.1.3, \"both table-cell paths\")")
	}
	if !containsSubstring(err.Error(), "closed number-pattern grammar") {
		t.Errorf("the refusal was %q; expected it to come from the closed number-pattern grammar, which is what makes a footer cell line-feed-free BY CONSTRUCTION rather than by luck", err.Error())
	}
	t.Logf("footer cell path: a line feed is unreachable by construction — %v", err)
}

// TestTrailingBreakInACellGrowsTheRowByOneAdvance is the row-height half
// of D-7.1.3, and it is the case the interior-break tests above cannot
// reach: a TRAILING line feed adds an empty line that draws NO RUN, so
// every run-counting assertion is blind to it — while the row still gets
// one full Advance taller. That height is what moves a page boundary.
func TestTrailingBreakInACellGrowsTheRowByOneAdvance(t *testing.T) {
	dataFor := func(v string) string {
		return fmt.Sprintf(`{"note": %q, "items": [{"a": %q}]}`, v, v)
	}
	rowExtent := func(dataJSON string) (top, bottom geom.Length, runLines int) {
		t.Helper()
		_, contentRuns, tableRects := paginateContentTableForTest(t, mandatoryBreakTableDoc(), dataJSON)
		found := false
		for _, r := range tableRects {
			if r.isDataRow && r.rowIndex == 0 {
				top, bottom, found = r.top, r.bottom, true
			}
		}
		if !found {
			t.Fatal("presence precondition: the table produced no data-row chrome rect for row 0")
		}
		seen := map[int]bool{}
		for _, r := range contentRuns {
			if r.isTableRowLine && r.rowIndex == 0 {
				seen[r.lineIndex] = true
			}
		}
		return top, bottom, len(seen)
	}

	plainTop, plainBottom, plainRuns := rowExtent(dataFor("abc"))
	brokenTop, brokenBottom, brokenRuns := rowExtent(dataFor("abc\n"))

	plainH := plainBottom - plainTop
	brokenH := brokenBottom - brokenTop
	if plainH <= 0 {
		t.Fatalf("presence precondition: the unbroken row has height %d", plainH)
	}

	// THE BLINDNESS THIS TEST EXISTS FOR, stated rather than implied:
	// the trailing empty line draws nothing, so both documents emit the
	// SAME number of physical line runs. A run-counting assertion cannot
	// see the difference at all.
	if brokenRuns != plainRuns {
		t.Errorf("the trailing-break row emitted %d line run(s) against the plain row's %d — an EMPTY line must draw nothing", brokenRuns, plainRuns)
	}

	vm, verr := chainVerticalModel([]string{"Noto Sans"}, geom.Length(8000), defaultLineSpacing, testShippedFontSet(), newFontCache())
	if verr != nil {
		t.Fatalf("chainVerticalModel: %v", verr)
	}
	if got := brokenH - plainH; got != vm.Advance {
		t.Errorf("a cell value of %q makes its row %d mp taller than %q does, want exactly one Advance of %d — a trailing break adds a real line box, and that height is what moves a page boundary", "abc\n", got, "abc", vm.Advance)
	}
	t.Logf("D-7.1.3 row height: %q -> %d mp, %q -> %d mp (+%d = one Advance), both emitting %d line run(s)", "abc", plainH, "abc\n", brokenH, brokenH-plainH, plainRuns)
}

// TestLineFeedInAColumnLabelBreaksTheLine is the SPEC-table-rules
// reversal of Story 7.1's TestLineFeedInAColumnLabelStillWarns, and the
// reversal is the whole point of that change.
//
// A column label used to be shaped by shapeSegments and handed straight
// to positionSegments — the one production caller that never packed — so
// a line feed in a label really WAS dropped: no glyph, no advance, two
// words run together on one baseline, and a TEXT_MISSING_GLYPH Warning
// as the only signal that anything had happened. SPEC-table-rules §4
// routes a label through the SAME packer a data cell uses, so the break
// is now TAKEN: two lines, and no Warning, because nothing was dropped.
//
// The old test's closing assertion — "the label stays on ONE baseline" —
// is inverted here for the same reason. That was never a property worth
// keeping; it was the defect, written down.
func TestLineFeedInAColumnLabelBreaksTheLine(t *testing.T) {
	doc := func(label string) string {
		return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": %q, "width": 180, "bind": "{{row.a}}"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`, label)
	}
	const dataJSON = `{"items":[{"a":"x"}]}`

	countMissingGlyph := func(tplJSON string) (int, []Diagnostic) {
		t.Helper()
		tpl, err := ParseTemplate([]byte(tplJSON))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		res, err := Render(tpl, Data(dataJSON), nil, testShippedFontSet())
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		n := 0
		for _, d := range res.Diagnostics {
			if d.Code == DiagCodeTextMissingGlyph {
				n++
				if d.ElementID != "e2" {
					t.Errorf("the missing-glyph Warning names %q, want the column e2", d.ElementID)
				}
				if d.Severity != SeverityWarning {
					t.Errorf("the missing-glyph diagnostic is %q, want a Warning", d.Severity)
				}
			}
		}
		return n, res.Diagnostics
	}

	// NEGATIVE CONTROL: an ordinary label warns about nothing, so the
	// Warning below is the line feed and not the document.
	if n, diags := countMissingGlyph(doc("Alpha Bravo")); n != 0 {
		t.Fatalf("presence precondition: an ordinary column label produced %d missing-glyph Warning(s): %+v", n, diags)
	}

	// THE SUBJECT: a label carrying a line feed. The break is taken, so
	// no rune is dropped and there is nothing to warn about.
	n, diags := countMissingGlyph(doc("Alpha\nBravo"))
	if n != 0 {
		t.Fatalf("a column label carrying a line feed produced %d missing-glyph Warning(s), want 0 — the label is packed now, so the break is TAKEN rather than handed to the shaper as a rune to draw (diagnostics: %+v)", n, diags)
	}

	// ...and it occupies TWO baselines.
	_, contentRuns, _ := paginateContentTableForTest(t, doc("Alpha\nBravo"), dataJSON)
	baselines := map[geom.Length]bool{}
	labelRuns := 0
	for _, r := range contentRuns {
		if r.isTableRowLine {
			continue
		}
		if containsSubstring(r.text, "Alpha") || containsSubstring(r.text, "Bravo") {
			// The run's own y, not itemTop: every line of ONE header row
			// shares the header's extent by construction (it is one
			// group that moves whole), so itemTop cannot distinguish
			// them and an assertion made on it would be vacuous.
			baselines[r.y] = true
			labelRuns++
		}
	}
	if labelRuns == 0 {
		t.Fatal("presence precondition: no run carrying the column label was found, so the baseline assertion below is vacuous")
	}
	if len(baselines) != 2 {
		t.Errorf("the column label occupies %d baselines, want 2 — the line feed must start a new line", len(baselines))
	}
	t.Logf("label path: %d run(s) on %d baseline(s), %d missing-glyph Warning — the break is taken and nothing is dropped", labelRuns, len(baselines), n)
}

// justifyCascadeColumns declares no column-level `align` at all, so
// every cell — header, body and footer — takes the table's cascaded
// alignFallback: headerStyle.align for the header, style.align for the
// rest.
const justifyCascadeColumns = `[
  {"id": "e2", "label": "Date", "width": 140, "bind": "{{row.a}}"},
  {"id": "e3", "label": "Amount", "width": 160, "bind": "{{row.b}}", "footer": "count"}
]`

// TestATableWhoseCascadedAlignIsJustifyIsRefusedAtLoad is Story 7.3's
// TestTableCellsCascadedJustifyIsDrawnAtTheStartEdge, INVERTED at Story
// 7.8. It is not deleted, and the assertion it carried is not dropped —
// it MOVED DOWN A LAYER, because its subject no longer exists.
//
// What it used to pin: `justify` was admitted at style.align and
// headerStyle.align — the contract REQUIRED such a document to load and
// to be raised to 2.0 — and those two are exactly what a table cell
// without its own `columns[].align` falls back to. So a cascaded
// `justify` really did reach the header, body and footer align
// switches, none of which has a justify arm, and every cell drew at the
// start edge. The test asserted byte-identity against `left`, and it was
// correct against the contract as written.
//
// What DW-29 recorded about that: the author paid a MAJOR version bump
// — a document unreadable to every 1.x reader — for a render identical
// to `left`, with no diagnostic anywhere. Story 7.8 refuses the value at
// load instead, keyed on the CONSUMER: a table's style.align and
// headerStyle.align validate against the same three values its
// columns[].align does, because all three meet in one alignFallback.
//
// So the byte-identity claim is now UNREACHABLE — no such document can
// exist — and this test asserts the refusal that makes it unreachable,
// through the same real load path (ParseTemplate over
// tableHeaderDocFull, which injects the style at BOTH attachment
// points).
//
// The `center`-vs-`left` leg is PRESERVED, and it is still doing the
// same job: it proves the cascade genuinely reaches the cell align
// switches for the values that still load. Without it, "justify is
// refused" would be consistent with a table that ignores its cascaded
// align entirely.
func TestATableWhoseCascadedAlignIsJustifyIsRefusedAtLoad(t *testing.T) {
	const data = `{"items": [{"a": "Jan", "b": "7"}, {"a": "Feb", "b": "12"}]}`
	docFor := func(align string) []byte {
		style := `{"fontFamily": "latin", "fontSize": 9, "align": "` + align + `"}`
		return []byte(tableHeaderDocFull(style, style, justifyCascadeColumns, 20))
	}
	render := func(align string) []byte {
		t.Helper()
		tpl, err := ParseTemplate(docFor(align))
		if err != nil {
			t.Fatalf("ParseTemplate(align %q): %v", align, err)
		}
		res, err := Render(tpl, Data(data), nil, testShippedFontSet())
		if err != nil {
			t.Fatalf("Render(align %q): %v", align, err)
		}
		return res.Bytes
	}

	// THE INVERSION. The document tableHeaderDocFull builds carries the
	// same style at the table's own `style` and at its `headerStyle`, so
	// whichever the loader reaches first must refuse it, located.
	_, err := ParseTemplate(docFor("justify"))
	if err == nil {
		t.Fatal("a table whose cascaded align is \"justify\" must be REFUSED at load — it would otherwise raise the document to 2.0 and then render identically to \"left\" (DW-29)")
	}
	var renderErr *RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("the refusal must reach the caller as a *RenderError: %T %v", err, err)
	}
	if renderErr.Diagnostic.Code != DiagCodeTemplateFieldInvalid {
		t.Errorf("Code = %q, want %q — an uncoded load error is flattened to \"The template could not be processed\" at the WASM boundary and never reaches the author", renderErr.Diagnostic.Code, DiagCodeTemplateFieldInvalid)
	}
	if renderErr.Diagnostic.ElementID != "e1" {
		t.Errorf("ElementID = %q, want %q — the refusal must name the element", renderErr.Diagnostic.ElementID, "e1")
	}
	msg := err.Error()
	for _, want := range []string{"e1", "align", "left, center, right"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal must contain %q, got: %s", want, msg)
		}
	}
	if strings.Contains(msg, "right, justify") {
		t.Errorf("a table's refusal must not name justify among the legal values, got: %s", msg)
	}

	// THE NON-VACUITY LEG, preserved verbatim in intent: the values that
	// still load must still reach the cell align switches, or "justify
	// is refused" would be true of a table that ignores its align.
	left, centred := render("left"), render("center")
	if sha256Hex(centred) == sha256Hex(left) {
		t.Fatal("the centred table renders byte-identically to the left-aligned one, so the cascade is not reaching the cell align switches at all and the refusal above proves nothing about a table's alignment")
	}
	t.Logf("justify refused at load (%v); centred still differs from left (%s vs %s)", renderErr.Diagnostic.Code, sha256Hex(centred)[:12], sha256Hex(left)[:12])
}
