package folio8

import (
	"errors"
	"sort"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// TestWrappedCellGrowsTheRowAndNeverTheColumn is AC2's end-to-end
// proof, modelled on TestColumnGeometryNeverNegotiatesAgainstLabelContent
// (4.1's own anchor pattern for exactly this reason): three 60pt
// columns, so a widened middle column would visibly push the third
// column's origin. The middle column's DATA CELL carries a short value
// in one render and a long one in the other, in all three shaping
// paths AC2 requires (long Latin, long space-less Thai, long CJK).
//
// Anchor (D-000.68): the three columns' {X,W} pairs and the table's
// total width are TEST-OWNED literals, identical for both renders,
// read from neither render (4.1 Finding 8's own fix, reused verbatim).
func TestWrappedCellGrowsTheRowAndNeverTheColumn(t *testing.T) {
	cases := []struct {
		name   string
		narrow string
		wide   string
	}{
		{"latin", "N", "Alpha Bravo Charlie Delta Echo Foxtrot Golf Hotel India Juliet Kilo Lima Mike November"},
		{"thai", "น", "ณัฐวุฒิ เกิด กรุงเทพ ประเทศไทย สวัสดี ครับ ผม ชื่อ นาย ทดสอบ ระบบ งาน"},
		{"cjk", "日", "日本語漢字書体文書作成印刷組版技術情報処理装置画面表示解像度改善対応方法検討委員会報告書提出期限厳守"},
	}
	type wantRect struct{ x, w int64 }
	want := []wantRect{{0, 60000}, {60000, 60000}, {120000, 60000}}
	const wantTotal = int64(180000)

	cols := `[
  {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"},
  {"id": "e3", "label": "M", "width": 60, "bind": "{{row.b}}"},
  {"id": "e4", "label": "B", "width": 60, "bind": "{{row.c}}"}
]`

	assertRowGeometry := func(t *testing.T, name, label string, rects []pagemodel.Rect) {
		t.Helper()
		if len(rects) != 6 {
			t.Fatalf("%s/%s: expected 6 rects (3 header + 3 data row), got %d", name, label, len(rects))
		}
		for group := 0; group < 2; group++ {
			var total int64
			for i := 0; i < 3; i++ {
				r := rects[group*3+i]
				if int64(r.X) != want[i].x || int64(r.W) != want[i].w {
					t.Errorf("%s/%s: group %d column %d geometry = {X:%d,W:%d}, want {X:%d,W:%d}", name, label, group, i, r.X, r.W, want[i].x, want[i].w)
				}
				total += int64(r.W)
			}
			if total != wantTotal {
				t.Errorf("%s/%s: group %d total width = %d, want %d", name, label, group, total, wantTotal)
			}
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := threeColumnTableDoc(`{"fontFamily": "`+c.name+`"}`, cols)
			mkData := func(middle string) string {
				return `{"items": [{"a":"A0","b":"` + middle + `","c":"C0"}]}`
			}
			narrowPages := tablePagesForTest(t, doc, mkData(c.narrow))
			widePages := tablePagesForTest(t, doc, mkData(c.wide))

			if len(narrowPages) != 1 || len(widePages) != 1 {
				t.Fatalf("%s: expected exactly 1 page in both renders (R8 — this story's tables always fit one page), got narrow=%d wide=%d", c.name, len(narrowPages), len(widePages))
			}
			t.Logf("%s: page count observed = 1 in both renders (AC7's own requirement)", c.name)

			assertRowGeometry(t, c.name, "narrow", narrowPages[0].Rects)
			assertRowGeometry(t, c.name, "wide", widePages[0].Rects)

			// Control (D-000.9): the two renders must differ in glyph
			// output, or the geometry equality proves nothing.
			totalWide, totalNarrow := 0, 0
			for _, r := range widePages[0].Runs {
				totalWide += len(r.Glyphs)
			}
			for _, r := range narrowPages[0].Runs {
				totalNarrow += len(r.Glyphs)
			}
			if totalWide <= totalNarrow {
				t.Fatalf("%s: control failed — wide value produced %d glyphs, narrow produced %d", c.name, totalWide, totalNarrow)
			}

			// The row must be STRICTLY taller in the wide render — and by
			// EXACTLY (linesWide-1)*vm.Advance, never merely ">".
			narrowRowTop, narrowRowBottom := dataRowExtent(t, narrowPages[0].Rects)
			wideRowTop, wideRowBottom := dataRowExtent(t, widePages[0].Rects)
			narrowHeight := int64(narrowRowBottom - narrowRowTop)
			wideHeight := int64(wideRowBottom - wideRowTop)
			if wideHeight <= narrowHeight {
				t.Fatalf("%s: wide row height %d, want strictly greater than narrow row height %d", c.name, wideHeight, narrowHeight)
			}

			// linesWide is OBSERVED from the wide render's own distinct
			// line Y positions among the data row's runs — an
			// independent measurement of "how many lines the wide cell
			// wrapped to", never read back from the row-height formula
			// under test.
			linesWide := distinctRowLineCount(widePages[0].Runs, wideRowTop)
			if linesWide < 2 {
				t.Fatalf("%s: wide value did not wrap at all (observed %d line(s)) — this test's control precondition failed", c.name, linesWide)
			}
			vm, err := chainVerticalModel([]string{shippedFaceFor(c.name)}, defaultFontSizePt, defaultLineSpacing, testShippedFontSet(), newFontCache())
			if err != nil {
				t.Fatalf("chainVerticalModel: %v", err)
			}
			wantDelta := int64(linesWide-1) * int64(vm.Advance)
			gotDelta := wideHeight - narrowHeight
			if gotDelta != wantDelta {
				t.Errorf("%s: row height delta = %d, want EXACTLY %d ((%d-1)*%d, production vm.Advance) — not merely greater", c.name, gotDelta, wantDelta, linesWide, vm.Advance)
			}
		})
	}
}

// shippedFaceFor maps this file's fontFamily key to the shipped face
// name testShippedFontSet() actually registers it under.
func shippedFaceFor(fontFamily string) string {
	switch fontFamily {
	case "thai":
		return "Noto Sans Thai"
	case "cjk":
		return "Noto Sans SC"
	default:
		return "Noto Sans"
	}
}

// dataRowExtent returns the data row's rect group's Y..Y+H extent —
// rects[3] is the first data-row rect (3 header rects precede it) in
// every 3-column fixture this file builds.
func dataRowExtent(t *testing.T, rects []pagemodel.Rect) (top, bottom geom.Length) {
	t.Helper()
	if len(rects) < 6 {
		t.Fatalf("expected at least 6 rects (3 header + 3 data row), got %d", len(rects))
	}
	r := rects[3]
	return r.Y, r.Y + r.H
}

// distinctRowLineCount counts the distinct Y positions among runs
// whose Y is at or below rowTopFloor — i.e. within the data row, never
// the header — an INDEPENDENT observation of how many physical lines
// the row occupies, derived from where positionSegments actually
// placed glyphs, not from the row-height arithmetic under test.
func distinctRowLineCount(runs []pagemodel.TextRun, rowTopFloor geom.Length) int {
	seen := map[geom.Length]bool{}
	for _, r := range runs {
		if r.Y >= rowTopFloor {
			seen[r.Y] = true
		}
	}
	return len(seen)
}

// TestUnbreakableCellContentIsClippedNotWidened is AC3: a cell whose
// content contains no break opportunity narrow enough for its column
// is clipped at the column's content box, with the EXISTING
// DiagCodeTextClippedWidth (D-2.8.1's precedent, D-000.65: no new
// code minted) — never widened, and never moving any column's origin.
func TestUnbreakableCellContentIsClippedNotWidened(t *testing.T) {
	cols := `[
  {"id": "e2", "label": "A", "width": 100, "bind": "{{row.a}}"},
  {"id": "e3", "label": "Long", "width": 30, "bind": "{{row.b}}"}
]`
	doc := tableHeaderDoc(`{"fontFamily": "latin"}`, cols)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := mustDecodeData(t, `{"items": [{"a":"x","b":"Supercalifragilisticexpialidocious"}]}`)
	params := mustDecodeParams(t)
	pages, _, _, diags, err := buildPageModel(tpl, data, params, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	t.Logf("page count observed = 1 (AC7)")

	// Exactly one Warning, naming the COLUMN id (AC4's own grounds).
	var found []Diagnostic
	for _, d := range diags {
		if d.Code == DiagCodeTextClippedWidth {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("got %d DiagCodeTextClippedWidth diagnostics, want exactly 1: %+v", len(found), found)
	}
	if found[0].ElementID != "e3" {
		t.Errorf("diagnostic ElementID = %q, want %q (the COLUMN id, not the table's)", found[0].ElementID, "e3")
	}
	if found[0].Severity != SeverityWarning {
		t.Errorf("Severity = %v, want SeverityWarning", found[0].Severity)
	}

	// ClipToBox set, ClipWidth == the column's OWN content width
	// (test-owned literal: 30pt column, zero padding, so content width
	// == 30000 millipoints).
	var clipped int
	for _, r := range pages[0].Runs {
		if r.ClipToBox {
			clipped++
			if int64(r.ClipWidth) != 30000 {
				t.Errorf("ClipWidth = %d, want 30000 (the column's own content width)", r.ClipWidth)
			}
		}
	}
	if clipped == 0 {
		t.Fatal("expected at least one clipped run in the over-wide column's data cell")
	}

	// Column geometry (X/W) is unchanged by the overflow — same literal
	// pairs as the header's own columns: {0,100000} and {100000,30000}.
	if len(pages[0].Rects) != 4 { // 2 header + 2 data-row cells
		t.Fatalf("got %d rects, want 4 (2 header + 2 data row)", len(pages[0].Rects))
	}
	wantX := []int64{0, 100000, 0, 100000}
	wantW := []int64{100000, 30000, 100000, 30000}
	for i, r := range pages[0].Rects {
		if int64(r.X) != wantX[i] || int64(r.W) != wantW[i] {
			t.Errorf("rect %d geometry = {X:%d,W:%d}, want {X:%d,W:%d}", i, r.X, r.W, wantX[i], wantW[i])
		}
	}
}

// TestDataRowCarriesNoChromeFromTheTablesOwnStyle is the
// SPEC-table-rules §1 reversal of Story 4.2's TestDataRowBorderIsDrawn.
//
// D-4.2.1 ruled that data cells get cell chrome cascaded from the
// table's own `style`, and that ruling is what made a 1pt table border
// print a full grid whose interior lines were each stroked twice and
// whose frame grew heavier as rows arrived. SPEC-table-rules overturns
// it: `style.border` paints the table's own BOX, once, and NOTHING
// strokes a data cell. The interior lines are `table.rules`, addressed
// by boundary — see TestRulesDrawOneLinePerInteriorBoundary.
//
// The data-row rects still EXIST: they carry the row's pagination
// identity, and D-2.6.5 forbids an item that occupies space from being
// empty. They simply paint nothing.
func TestDataRowCarriesNoChromeFromTheTablesOwnStyle(t *testing.T) {
	doc := tableHeaderDoc(`{"fontFamily": "latin", "border": {"color": "#112233", "width": 2}}`, twoColumnsNoAlign)
	pages := tablePagesForTest(t, doc, `{"items": [{"a":"1","b":"2"}]}`)
	if len(pages[0].Rects) != 5 {
		t.Fatalf("got %d rects, want 5 (2 header + 2 data row + the table's own box)", len(pages[0].Rects))
	}
	for i, r := range pages[0].Rects[:4] {
		if r.HasStroke {
			t.Errorf("cell rect %d strokes; no cell may carry the element's own style.border (SPEC-table-rules §1)", i)
		}
		if r.HasFill {
			t.Errorf("cell rect %d fills; no cell may carry the element's own style.background", i)
		}
	}
	box := pages[0].Rects[4]
	if !box.HasStroke {
		t.Fatalf("the table's own box does not stroke — style.border must paint it")
	}
	if box.Stroke != (pagemodel.Color{R: 0x11, G: 0x22, B: 0x33}) {
		t.Errorf("box Stroke = %+v, want #112233", box.Stroke)
	}
	if box.StrokeWidth != 2000 {
		t.Errorf("box StrokeWidth = %d, want 2000 (2pt)", box.StrokeWidth)
	}
	if !box.Edges.Top || !box.Edges.Right || !box.Edges.Bottom || !box.Edges.Left {
		t.Errorf("box Edges = %+v, want all four (no \"edges\" declared)", box.Edges)
	}
	// THE FRAME IS THE TABLE'S EXTENT, not a row's: it spans the header
	// plus the one data row, and it is the only rect that does.
	if box.Y != pages[0].Rects[0].Y {
		t.Errorf("box Y = %d, want the header's own top %d", box.Y, pages[0].Rects[0].Y)
	}
	lastRow := pages[0].Rects[3]
	if want := lastRow.Y + lastRow.H - box.Y; box.H != want {
		t.Errorf("box H = %d, want %d (header top to the last row's bottom)", box.H, want)
	}
}

// TestDataCellPaddingShiftsRowHeightAndContentOrigin is Story 4.2
// review Blocker 2: data-cell padding IS wired into the row-height
// formula (R3) and every cell's content X/width, but nothing asserted
// it, so zeroing all four padding edges left the full gate green.
//
// This is a DIFFERENTIAL test (D-000.68/D-000.9): two renders of the
// SAME template, varying only style.padding, so the delta is a
// test-owned literal exact amount rather than an absolute value this
// test would have to re-derive font metrics to state.
//
// Red-proof (recorded in the Delivery Log as M3, reproducing the
// reviewer's own mutation): zero padTopB/padRightB/padBottomB/padLeftB
// immediately after they are resolved — both deltas below collapse to
// 0 and this test reddens; TestTableStyleFieldsAreNotDataDriven (which
// compares two renders that are BOTH padded identically) stays green,
// confirming that test alone cannot see this defect.
func TestDataCellPaddingShiftsRowHeightAndContentOrigin(t *testing.T) {
	cols := `[{"id": "e2", "label": "A", "width": 100, "bind": "{{row.a}}"}]`
	noPadDoc := tableHeaderDoc(`{"fontFamily": "latin"}`, cols)
	paddedDoc := tableHeaderDoc(`{"fontFamily": "latin", "padding": {"left": 20, "top": 5, "bottom": 7}}`, cols)

	noPadPages := tablePagesForTest(t, noPadDoc, `{"items": [{"a":"x"}]}`)
	paddedPages := tablePagesForTest(t, paddedDoc, `{"items": [{"a":"x"}]}`)

	// Each render is 1 header rect + 1 data-row rect, and 1 header run
	// + 1 data run (single column, single line, single row).
	for _, p := range []struct {
		name  string
		pages []pagemodel.Page
	}{{"no-padding", noPadPages}, {"padded", paddedPages}} {
		if len(p.pages[0].Rects) != 2 {
			t.Fatalf("%s: got %d rects, want 2 (1 header + 1 data row)", p.name, len(p.pages[0].Rects))
		}
		if len(p.pages[0].Runs) != 2 {
			t.Fatalf("%s: got %d runs, want 2 (1 header label + 1 data value)", p.name, len(p.pages[0].Runs))
		}
	}

	// Row height: padded row's rect H must exceed the unpadded row's by
	// EXACTLY padding.top + padding.bottom = (5+7)pt = 12000 millipoints
	// (R3's formula adds padTopB/padBottomB directly; content is a
	// single line in both renders, so vm's own contribution is
	// identical and cancels out of the delta).
	gotDelta := int64(paddedPages[0].Rects[1].H) - int64(noPadPages[0].Rects[1].H)
	if wantDelta := int64(12000); gotDelta != wantDelta {
		t.Errorf("data row H delta = %d, want %d (padding.top+padding.bottom, exactly)", gotDelta, wantDelta)
	}

	// Content X: the padded render's data run must sit EXACTLY
	// padding.left = 20pt = 20000 millipoints to the right of the
	// unpadded render's.
	gotShift := int64(paddedPages[0].Runs[1].X) - int64(noPadPages[0].Runs[1].X)
	if wantShift := int64(20000); gotShift != wantShift {
		t.Errorf("data cell run X delta = %d, want %d (padding.left, exactly)", gotShift, wantShift)
	}
}

// TestCellBindingsResolveInRowScope is AC4: a cell's bind resolves
// against a scope whose row root is the current element under the
// table's alias — the alias is the declared "as", or "row" when
// absent — an unqualified path still resolves from the DOCUMENT ROOT
// (never the row), and "params." still resolves to the parameters,
// shadowed by nothing (AD-11).
//
// Anchor (D-000.68): the document root's "a" and the row's "a" are
// DELIBERATELY DIFFERENT test-owned strings, so "unqualified resolves
// from the root" is falsifiable — a test where they coincide proves
// nothing.
func TestCellBindingsResolveInRowScope(t *testing.T) {
	cols := `[
  {"id": "e2", "label": "Row", "width": 80, "bind": "{{row.a}}"},
  {"id": "e3", "label": "Root", "width": 80, "bind": "{{a}}"},
  {"id": "e4", "label": "Param", "width": 80, "bind": "{{params.p}}"}
]`
	doc := threeColumnTableDoc(`{"fontFamily": "latin"}`, cols)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := mustDecodeData(t, `{"a": "ROOT-VALUE", "items": [{"a": "ROW-VALUE"}]}`)
	params, perr := decodeParams(Params(`{"p": "PARAM-VALUE"}`))
	if perr != nil {
		t.Fatalf("decodeParams: %v", perr)
	}
	pages, _, _, _, err := buildPageModel(tpl, data, params, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	t.Logf("page count observed = 1 (AC7)")

	// Story 4.2 review Finding 16: scanning for "did this string appear
	// ANYWHERE" cannot detect a column swap — if row/root/params were
	// transposed across e2/e3/e4, all three sentinels would still be
	// present. So each sentinel's run X is also checked against its
	// OWN column's declared span: e2 is [0,80000), e3 is
	// [80000,160000), e4 is [160000,240000) (all three columns are
	// 80pt wide, left-aligned by default, zero padding, so a
	// correctly-attributed run's X equals its column's own X exactly).
	var rowText, rootText, paramText string
	var rowX, rootX, paramX *int64
	for _, r := range pages[0].Runs {
		x := int64(r.X)
		switch r.SourceText {
		case "ROW-VALUE":
			rowText = r.SourceText
			rowX = &x
		case "ROOT-VALUE":
			rootText = r.SourceText
			rootX = &x
		case "PARAM-VALUE":
			paramText = r.SourceText
			paramX = &x
		}
	}
	if rowText != "ROW-VALUE" {
		t.Errorf("{{row.a}} did not resolve to the row's own value, got runs %v", sourceTexts(pages[0].Runs))
	}
	if rootText != "ROOT-VALUE" {
		t.Errorf("{{a}} did not resolve to the DOCUMENT ROOT's value (must not be shadowed by the row), got runs %v", sourceTexts(pages[0].Runs))
	}
	if paramText != "PARAM-VALUE" {
		t.Errorf("{{params.p}} did not resolve to the supplied parameter, got runs %v", sourceTexts(pages[0].Runs))
	}
	if rowX == nil || rootX == nil || paramX == nil {
		t.Fatalf("expected all three sentinel runs to be present")
	}
	if *rowX != 0 {
		t.Errorf("{{row.a}} (column e2) landed at X=%d, want 0 — a value in the wrong column would not be caught by presence alone", *rowX)
	}
	if *rootX != 80000 {
		t.Errorf("{{a}} (column e3) landed at X=%d, want 80000 — a value in the wrong column would not be caught by presence alone", *rootX)
	}
	if *paramX != 160000 {
		t.Errorf("{{params.p}} (column e4) landed at X=%d, want 160000 — a value in the wrong column would not be caught by presence alone", *paramX)
	}
}

// TestCellBindingResolvesUnderDeclaredAlias is AC4's declared-alias
// counterpart: {{transaction.a}} under a table declaring
// "as": "transaction".
func TestCellBindingResolvesUnderDeclaredAlias(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 20,
        "as": "transaction", "style": {"fontFamily": "latin"},
        "columns": [{"id": "e2", "label": "A", "width": 80, "bind": "{{transaction.a}}"}]}
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
	pages := tablePagesForTest(t, doc, `{"items": [{"a": "ALIASED-VALUE"}]}`)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	found := false
	for _, r := range pages[0].Runs {
		if r.SourceText == "ALIASED-VALUE" {
			found = true
		}
	}
	if !found {
		t.Errorf("{{transaction.a}} under a declared alias did not resolve, got runs %v", sourceTexts(pages[0].Runs))
	}
}

func sourceTexts(runs []pagemodel.TextRun) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.SourceText
	}
	return out
}

// TestCellBindingAbsentNullWrongKind is AC4's AD-14 triple, asserted
// individually: absent path -> Error; explicit JSON null -> empty
// cell, not an error; wrong kind -> Error, never coerced.
//
// Story 4.2 review Finding 5: the original explicit_null and wrong_kind
// subtests asserted only "didn't error" / "errored", which cannot
// discriminate the behaviour AD-14 actually names. A SECOND, always-
// bound column ("b") is added so explicit_null can assert the null
// cell contributes NO run while its sibling column's run is still
// present (a differential — "no runs at all" alone could also mean the
// whole render broke), and wrong_kind now asserts the SAME rigour as
// the absent subtest: a *RenderError, its diagnostic code, and the
// COLUMN id.
func TestCellBindingAbsentNullWrongKind(t *testing.T) {
	cols := `[
  {"id": "e2", "label": "A", "width": 80, "bind": "{{row.a}}"},
  {"id": "e3", "label": "B", "width": 80, "bind": "{{row.b}}"}
]`
	doc := tableHeaderDoc(`{"fontFamily": "latin"}`, cols)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	t.Run("absent", func(t *testing.T) {
		_, rerr := Render(tpl, Data(`{"items": [{"b":"y"}]}`), nil, testShippedFontSet())
		if rerr == nil {
			t.Fatal("an absent bound path must be a located Error (AD-14)")
		}
		var re *RenderError
		if !errors.As(rerr, &re) {
			t.Fatalf("expected a *RenderError, got %T: %v", rerr, rerr)
		}
		if re.Diagnostic.Code != DiagCodeBindingPathAbsent {
			t.Errorf("Code = %q, want %q", re.Diagnostic.Code, DiagCodeBindingPathAbsent)
		}
		if re.Diagnostic.ElementID != "e2" {
			t.Errorf("ElementID = %q, want the COLUMN id %q", re.Diagnostic.ElementID, "e2")
		}
	})

	t.Run("explicit_null", func(t *testing.T) {
		pages, _, _, _, rerr := buildPageModel(tpl, mustDecodeData(t, `{"items": [{"a": null, "b": "SIBLING-VALUE"}]}`), mustDecodeParams(t), testShippedFontSet())
		if rerr != nil {
			t.Fatalf("an explicit JSON null must render as an empty cell, not an error (AD-14): %v", rerr)
		}
		if len(pages) != 1 {
			t.Fatalf("got %d pages, want 1", len(pages))
		}
		// Positive, differential proof the null cell contributes NO
		// run: the header's two labels plus the data row's SIBLING
		// column alone is exactly 3 runs, and the sibling's own run
		// must be present (a regression that rendered the literal
		// "null", "<nil>", or fell back to the column label would move
		// this count or this text).
		if len(pages[0].Runs) != 3 {
			t.Fatalf("got %d runs, want 3 (2 header labels + 1 data run from the SIBLING column only — the null cell contributes none)", len(pages[0].Runs))
		}
		found := false
		for _, r := range pages[0].Runs {
			if r.SourceText == "SIBLING-VALUE" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected the sibling column's own run, got runs %v", sourceTexts(pages[0].Runs))
		}
	})

	t.Run("wrong_kind", func(t *testing.T) {
		_, rerr := Render(tpl, Data(`{"items": [{"a": {"nested": true}, "b": "y"}]}`), nil, testShippedFontSet())
		if rerr == nil {
			t.Fatal("a wrong-kind bound value (an object) must be a located Error, never coerced (AD-14)")
		}
		var re *RenderError
		if !errors.As(rerr, &re) {
			t.Fatalf("expected a *RenderError, got %T: %v", rerr, rerr)
		}
		if re.Diagnostic.Code != DiagCodeExpressionInvalid {
			t.Errorf("Code = %q, want %q (wrong-kind formula values are distinct from missing paths)", re.Diagnostic.Code, DiagCodeExpressionInvalid)
		}
		if re.Diagnostic.ElementID != "e2" {
			t.Errorf("ElementID = %q, want the COLUMN id %q", re.Diagnostic.ElementID, "e2")
		}
	})
}

// TestDataCellsDoNotInheritHeaderStyle is AC5, guarding against 4.1's
// Blocker 1 shape recurring at the header/body boundary this story
// creates: a table declaring headerStyle.fontFamily and NO
// style.fontFamily renders its header successfully and fails its DATA
// CELLS — the SAME message the existing font-resolution failure
// produces (fontChain, reused verbatim), never a third spelling.
func TestDataCellsDoNotInheritHeaderStyle(t *testing.T) {
	doc := tableHeaderDocFull(``, `{"fontFamily": "latin"}`, twoColumnsNoAlign, 20)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	// (i) headerStyle.fontFamily-only + non-empty collection -> data
	// cells error.
	_, rerr := Render(tpl, Data(`{"items": [{"a":"1","b":"2"}]}`), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("a table with headerStyle.fontFamily but no style.fontFamily must fail on its DATA CELLS once rows exist")
	}
	t.Logf("AC5(i) error: %v", rerr)

	// The SAME message shape fontChain's own failure produces for a
	// text element with no style.fontFamily (D-000.65: no third
	// spelling) — asserted by substring, since fontChain is reused
	// verbatim rather than re-implemented.
	wantSubstring := "has text but no style.fontFamily to resolve a font from"
	if !containsSubstring(rerr.Error(), wantSubstring) {
		t.Errorf("error %q does not contain fontChain's own message %q — AC5 requires reusing the SAME font-resolution failure, not a new spelling", rerr.Error(), wantSubstring)
	}

	// The SAME table with an EMPTY collection must still succeed — AC6
	// (empty collection is fine even style-less on the body side).
	_, rerr2 := Render(tpl, Data(`{"items": []}`), nil, testShippedFontSet())
	if rerr2 != nil {
		t.Fatalf("an EMPTY collection must not require a resolvable body font (AC6): %v", rerr2)
	}

	// (ii) headerStyle sets a DIFFERENT background/align from style:
	// header cells use headerStyle's, data cells use style's.
	cols := `[
  {"id": "e2", "label": "A", "width": 100, "bind": "{{row.a}}"},
  {"id": "e3", "label": "B", "width": 100, "bind": "{{row.b}}"}
]`
	doc2 := tableHeaderDocFull(
		`{"fontFamily": "latin", "background": "#0000FF", "align": "left"}`,
		`{"background": "#FF0000", "align": "right"}`,
		cols, 20)
	tpl2, err := ParseTemplate([]byte(doc2))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := mustDecodeData(t, `{"items": [{"a":"x","b":"y"}]}`)
	params := mustDecodeParams(t)
	pages, _, _, _, err := buildPageModel(tpl2, data, params, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	// SPEC-table-rules §1: style.background paints the table's own BOX
	// (its fill the FIRST rect), so the two data cells are unfilled and the
	// header's own fill still comes from headerStyle. The property this
	// test exists for is unchanged — a data cell never takes
	// headerStyle's — and it is now joined by its converse.
	if len(pages[0].Rects) != 5 {
		t.Fatalf("got %d rects, want 5 (2 header + 2 data row + the table's own box)", len(pages[0].Rects))
	}
	headerRects := pages[0].Rects[1:3]
	dataRects := pages[0].Rects[3:5]
	for i, r := range headerRects {
		if r.Fill != (pagemodel.Color{R: 0xFF, G: 0, B: 0}) {
			t.Errorf("header rect %d Fill = %+v, want #FF0000 (headerStyle's)", i, r.Fill)
		}
	}
	for i, r := range dataRects {
		if r.HasFill {
			t.Errorf("data rect %d fills with %+v; no cell carries the element's own style.background any more", i, r.Fill)
		}
	}
	if box := pages[0].Rects[0]; box.Fill != (pagemodel.Color{R: 0, G: 0, B: 0xFF}) {
		t.Errorf("the table's own box Fill = %+v, want #0000FF (style's, never headerStyle's)", box.Fill)
	}

	// Alignment: header is right-aligned (headerStyle.align), data is
	// left-aligned (style.align) — a right-aligned short value's run X
	// sits strictly right of a left-aligned one's for the SAME column.
	// Column 0 spans [0,100000).
	var headerCol0X, dataCol0X *int64
	for _, r := range pages[0].Runs {
		x := int64(r.X)
		if x >= 100000 {
			continue // column 1 only
		}
		if r.Y < dataRects[0].Y {
			if headerCol0X == nil {
				headerCol0X = &x
			}
		} else {
			if dataCol0X == nil {
				dataCol0X = &x
			}
		}
	}
	if headerCol0X == nil || dataCol0X == nil {
		t.Fatalf("expected runs in both the header and the data row's column 0")
	}
	if *dataCol0X != 0 {
		t.Errorf("data cell column 0 (style.align=left) X = %d, want 0", *dataCol0X)
	}
	if *headerCol0X <= 0 {
		t.Errorf("header cell column 0 (headerStyle.align=right) X = %d, want > 0 (right-aligned)", *headerCol0X)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// TestDataCellValignDistributesRowSlack is Story 4.2 review Finding 4
// (Major): resolvedBodyStyle.valign was resolved and NEVER READ, while
// AC5's own mechanism text, table_render.go's own comment, and
// folio-format.md all claimed it cascades to data cells exactly as it
// does to the header. This wires it: a SHORT cell sitting in a row
// whose OTHER cell wraps to several lines has real vertical slack —
// "top" (the only behaviour before this fix) leaves it at the row's
// first line, "bottom" moves it to the row's last line, and "middle"
// splits the difference.
//
// Anchor (D-000.68): this test does not hardcode vm.Advance or any
// font metric. It reads the WRAPPING column's own observed per-line Y
// positions (which valign never moves — that column has zero slack,
// since it IS the row's tallest cell) as its reference frame, then
// asserts the SHORT column's Y against specific entries in that same
// observed list — an anchor the code under test cannot move by
// re-deriving its own answer.
func TestDataCellValignDistributesRowSlack(t *testing.T) {
	cols := `[
  {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"},
  {"id": "e3", "label": "B", "width": 150, "bind": "{{row.b}}"}
]`
	longValue := "Alpha Bravo Charlie Delta Echo Foxtrot Golf Hotel India Juliet Kilo Lima Mike November Oscar Papa Quebec Romeo Sierra Tango"

	render := func(valign string) []textRunSource {
		doc := tableHeaderDoc(`{"fontFamily": "latin", "valign": "`+valign+`"}`, cols)
		tpl, err := ParseTemplate([]byte(doc))
		if err != nil {
			t.Fatalf("valign=%s: ParseTemplate: %v", valign, err)
		}
		bands, err := documentBands(tpl)
		if err != nil {
			t.Fatalf("valign=%s: documentBands: %v", valign, err)
		}
		data := mustDecodeData(t, `{"items": [{"a":"`+longValue+`","b":"Short"}]}`)
		params := mustDecodeParams(t)
		runs, _, _, err := collectBandTableRuns(tpl, bands, contentBandIndex, data, params, testFormatContext(), testShippedFontSet(), newFontCache(), nil)
		if err != nil {
			t.Fatalf("valign=%s: collectBandTableRuns: %v", valign, err)
		}
		return runs
	}

	// wrappingColumnLineTops returns the SORTED, DISTINCT itemTop
	// values among column A's ("Alpha", "Bravo", ...) row-line runs —
	// column A is the row's tallest cell (zero slack, offset always 0
	// per table_render.go), so this list is the row's own per-line Y
	// positions, identical across every valign (the control below
	// confirms this).
	wrappingColumnLineTops := func(runs []textRunSource) []int64 {
		seen := map[int64]bool{}
		for _, r := range runs {
			if !r.isTableRowLine || r.text == "Short" {
				continue
			}
			seen[int64(r.itemTop)] = true
		}
		tops := make([]int64, 0, len(seen))
		for top := range seen {
			tops = append(tops, top)
		}
		sort.Slice(tops, func(i, j int) bool { return tops[i] < tops[j] })
		return tops
	}
	shortColumnLineTop := func(runs []textRunSource) int64 {
		var found []int64
		for _, r := range runs {
			if r.isTableRowLine && r.text == "Short" {
				found = append(found, int64(r.itemTop))
			}
		}
		if len(found) != 1 {
			t.Fatalf("expected exactly one row-line run for the SHORT column, got %d", len(found))
		}
		return found[0]
	}

	topRuns := render("top")
	middleRuns := render("middle")
	bottomRuns := render("bottom")

	topTops := wrappingColumnLineTops(topRuns)
	middleTops := wrappingColumnLineTops(middleRuns)
	bottomTops := wrappingColumnLineTops(bottomRuns)

	if len(topTops) < 3 {
		t.Fatalf("presence precondition: column A wrapped to only %d line(s), want >= 3 for this test to distinguish top/middle/bottom", len(topTops))
	}
	// Control (D-000.9): valign must NOT move the row's own per-line
	// geometry — only the short cell's placement within it.
	for i := range topTops {
		if topTops[i] != middleTops[i] || topTops[i] != bottomTops[i] {
			t.Fatalf("control failed: column A's own line %d Y differs across valigns (top=%d, middle=%d, bottom=%d) — valign must move only the SHORT cell, never the row's own geometry", i, topTops[i], middleTops[i], bottomTops[i])
		}
	}

	slack := len(topTops) - 1 // the short cell has exactly 1 line
	wantTop := topTops[0]
	wantMiddle := topTops[slack/2]
	wantBottom := topTops[slack]
	if wantTop == wantMiddle || wantMiddle == wantBottom {
		t.Fatalf("presence precondition: slack=%d does not produce three DISTINCT positions (top=%d, middle=%d, bottom=%d)", slack, wantTop, wantMiddle, wantBottom)
	}

	if got := shortColumnLineTop(topRuns); got != wantTop {
		t.Errorf("valign=top: short cell itemTop = %d, want %d (the row's FIRST line)", got, wantTop)
	}
	if got := shortColumnLineTop(middleRuns); got != wantMiddle {
		t.Errorf("valign=middle: short cell itemTop = %d, want %d (slack/2 lines down)", got, wantMiddle)
	}
	if got := shortColumnLineTop(bottomRuns); got != wantBottom {
		t.Errorf("valign=bottom: short cell itemTop = %d, want %d (the row's LAST line)", got, wantBottom)
	}
}

// TestEmptyCollectionRendersHeaderOnly is AC6, strengthened per this
// story's own creation record (the pre-existing
// TestRenderTableBindEmptyArrayIsNotAnError proves only "does not
// fail" — vacuous as AC6's own instrument): all three halves are
// asserted POSITIVELY — the header is PRESENT (rect count ==
// len(columns), not merely "no error"), run count == len(columns) with
// each SourceText a column LABEL, and Render succeeds with no
// diagnostic.
func TestEmptyCollectionRendersHeaderOnly(t *testing.T) {
	doc := tableHeaderDoc(`{"fontFamily": "latin"}`, twoColumnsNoAlign)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := mustDecodeData(t, `{"items": []}`)
	params := mustDecodeParams(t)
	pages, _, _, diags, err := buildPageModel(tpl, data, params, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if len(pages[0].Rects) != 2 {
		t.Errorf("got %d rects, want 2 (== len(columns): the header IS present)", len(pages[0].Rects))
	}
	if len(pages[0].Runs) != 2 {
		t.Fatalf("got %d runs, want 2 (== len(columns))", len(pages[0].Runs))
	}
	wantLabels := map[string]bool{"Date": true, "Amount": true}
	for _, r := range pages[0].Runs {
		if !wantLabels[r.SourceText] {
			t.Errorf("run SourceText = %q, want a column label", r.SourceText)
		}
	}
	if len(diags) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(diags))
	}

	// Red-proof (a): emit one row for an empty collection -> counts
	// redden. Simulated here by rendering the SAME template against a
	// ONE-element collection and confirming the counts DO move (the
	// discriminating direction: this is what a `<=` mutation would
	// have left passing).
	oneItemData := mustDecodeData(t, `{"items": [{"a":"1","b":"2"}]}`)
	onePages, _, _, _, err := buildPageModel(tpl, oneItemData, params, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel(one item): %v", err)
	}
	if len(onePages[0].Rects) == len(pages[0].Rects) {
		t.Error("presence precondition: a one-item collection must produce MORE rects than an empty one, or the counts assertion above cannot distinguish them")
	}
	if len(onePages[0].Runs) == len(pages[0].Runs) {
		t.Error("presence precondition: a one-item collection must produce MORE runs than an empty one, or the counts assertion above cannot distinguish them")
	}
}

// TestDataRowIdentityIsConsistentAndDistinct is DECISION-2 (owner
// ruling): row membership must be recoverable from Paginate's input
// without inference. This story carries it as isDataRow/rowIndex on
// tableRectSource and isTableRowLine/rowIndex on textRunSource — this
// test proves the identity itself is correct: items from ONE row carry
// the SAME identity, items from DIFFERENT rows carry DIFFERENT
// identities, and — the property this ruling exists for — a WRAPPED
// row's several line items ALL AGREE with the row whose DATA they
// actually render, not merely with each other.
//
// Story 4.2 review Blocker 3: the original version of this test
// asserted only a COUNT of distinct rowIndex values, that maxLines>=2,
// and global lineIndex uniqueness. None of those can see a wrapped
// row's continuation lines being mis-attributed to a DIFFERENT,
// EXISTING row (mutation M2b: `placed[j].rowIndex = rowIdx + 1` for
// li>0) — the count of distinct indices stays 3 either way. This
// version embeds a PER-ROW MARKER in every row's own cell content
// ("R0W-", "R1W-", "R2W-" — every word of row 0's long value carries
// its marker, so every wrapped continuation line's SourceText still
// does too) and asserts, for every table-row line run, that (a) the
// marker actually present in its OWN SourceText names the row it
// claims via rowIndex, and (b) that rowIndex matches the tableRectSource
// group whose vertical extent physically contains the line — content
// identity and geometric identity must agree.
//
// Story 4.3 is this identity's one named consumer ("a row moves whole
// to the next page") — this test asserts NOTHING about pagination
// (AC7's fence).
func TestDataRowIdentityIsConsistentAndDistinct(t *testing.T) {
	tpl, err := ParseTemplate([]byte(tableHeaderDoc(`{"fontFamily": "latin"}`, twoColumnsNoAlign)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	data := mustDecodeData(t, `{"items": [
		{"a":"R0W-Alpha R0W-Bravo R0W-Charlie R0W-Delta R0W-Echo R0W-Foxtrot R0W-Golf R0W-Hotel R0W-India R0W-Juliet R0W-Kilo R0W-Lima R0W-Mike","b":"R0W-b"},
		{"a":"R1W-only","b":"R1W-b"},
		{"a":"R2W-only","b":"R2W-b"}
	]}`)
	params := mustDecodeParams(t)
	fc := testFormatContext()
	// Story 4.2 review Finding 21: ONE call, bound to both variables —
	// the original two identical calls (one discarding runs, one
	// discarding rects) served no purpose a single call did not.
	runs, rects, _, err := collectBandTableRuns(tpl, bands, contentBandIndex, data, params, fc, testShippedFontSet(), newFontCache(), nil)
	if err != nil {
		t.Fatalf("collectBandTableRuns: %v", err)
	}

	var dataRowRects []tableRectSource
	for _, r := range rects {
		if r.isDataRow {
			dataRowRects = append(dataRowRects, r)
		}
	}
	if len(dataRowRects) != 3 {
		t.Fatalf("got %d data-row rect groups, want 3 (one per row)", len(dataRowRects))
	}
	seen := map[int]bool{}
	for i, r := range dataRowRects {
		if r.rowIndex != i {
			t.Errorf("data-row rect group %d carries rowIndex %d, want %d", i, r.rowIndex, i)
		}
		if seen[r.rowIndex] {
			t.Errorf("rowIndex %d repeated across rect groups — rows must carry DISTINCT identities", r.rowIndex)
		}
		seen[r.rowIndex] = true
	}

	markers := []string{"R0W-", "R1W-", "R2W-"}
	rowLineCount := map[int]map[int]bool{} // rowIndex -> set of lineIndex
	contentChecked := 0
	for _, r := range runs {
		if !r.isTableRowLine {
			continue
		}
		if rowLineCount[r.rowIndex] == nil {
			rowLineCount[r.rowIndex] = map[int]bool{}
		}
		rowLineCount[r.rowIndex][r.lineIndex] = true

		// (a) content identity: the marker THIS RUN'S OWN TEXT carries
		// must name the row it claims via rowIndex — this is what
		// catches a continuation line attributed to the WRONG EXISTING
		// row, which a count of distinct indices cannot (M2b).
		wantRow := -1
		for m, marker := range markers {
			if containsSubstring(r.text, marker) {
				wantRow = m
				break
			}
		}
		if wantRow == -1 {
			t.Fatalf("row-line run %q carries none of the row markers %v — test fixture defect", r.text, markers)
		}
		if r.rowIndex != wantRow {
			t.Errorf("run %q carries marker for row %d but rowIndex = %d — a wrapped row's continuation line disagrees with the row whose DATA it renders", r.text, wantRow, r.rowIndex)
		}
		contentChecked++

		// (b) geometric identity: the SAME rowIndex must name the
		// tableRectSource whose vertical extent physically contains
		// this line.
		geomRow := -1
		for _, rr := range dataRowRects {
			if r.itemTop >= rr.top && r.itemBottom <= rr.bottom {
				geomRow = rr.rowIndex
				break
			}
		}
		if geomRow == -1 {
			t.Errorf("run %q (itemTop=%d, itemBottom=%d) falls inside no data-row rect group's extent", r.text, r.itemTop, r.itemBottom)
		} else if geomRow != r.rowIndex {
			t.Errorf("run %q: rowIndex = %d but its own line's extent falls inside rect group %d's span — content and geometric identity disagree", r.text, r.rowIndex, geomRow)
		}
	}
	if contentChecked == 0 {
		t.Fatal("presence precondition: no table-row-line runs were examined")
	}
	if len(rowLineCount) != 3 {
		t.Fatalf("got row-line runs for %d distinct rowIndex values, want 3", len(rowLineCount))
	}
	// SOME row must wrap to at least 2 physical lines — deliberately
	// not assumed to be row 0 by POSITION, only by data content (the
	// long value): this test's row-loop order is independent of which
	// collection element is "the long one", so the check is keyed off
	// the observed line counts themselves, not a positional assumption
	// (this is what keeps this test from reddening under AC1's own
	// red-proof (b), a reversed row loop, for a reason that has
	// nothing to do with row identity).
	maxLines := 0
	for _, lines := range rowLineCount {
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	if maxLines < 2 {
		t.Errorf("no row wrapped to at least 2 physical lines (max observed %d) — the long value must produce a wrapped row for this test to exercise anything", maxLines)
	}
	for row, lines := range rowLineCount {
		if len(lines) == 0 {
			t.Errorf("row %d reported zero lines", row)
		}
	}
	// No lineIndex is shared between two different rows (global
	// uniqueness within the element, D2's own requirement so
	// (elementID,lineIndex) grouping never merges two physical lines
	// from different rows into one ColumnItem).
	lineOwner := map[int]int{}
	for row, lines := range rowLineCount {
		for li := range lines {
			if owner, ok := lineOwner[li]; ok {
				t.Errorf("lineIndex %d claimed by both row %d and row %d — must be globally unique per element", li, owner, row)
			}
			lineOwner[li] = row
		}
	}
}

// TestWithinBandTableDiagnosticsFollowAllTextDiagnostics is AC8,
// pinning D-2.8.6's CURRENT within-band ordering as a NAMED,
// deliberate pin — not an endorsement (engineering lead's ruling on
// DECISION-3): render.go assembles headerDiags, headerTableDiags,
// contentDiags, contentTableDiags, footerDiags, footerTableDiags, so
// within one band every TEXT element's diagnostics precede every
// TABLE's, regardless of declaration order — even though
// render.go's own D-2.8.6 comment states the rule as "declaration
// order within a band". THIS TEST RECORDS CURRENT BEHAVIOUR. IT DOES
// NOT ENDORSE IT. Whether the assembly should be interleaved by
// declaration order (matching the stated rule) or D-2.8.6 relaxed to
// match the code is an open question for the engineering lead — this
// story pins the ordering so a future change to it is a visible,
// deliberate diff against this test rather than a silent behaviour
// change (DECISION-3).
func TestWithinBandTableDiagnosticsFollowAllTextDiagnostics(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 20,
        "style": {"fontFamily": "latin"},
        "columns": [{"id": "e2", "label": "Long", "width": 30, "bind": "{{row.a}}"}]},
      {"id": "e3", "type": "text", "x": 0, "y": 40, "width": 20, "height": 14, "value": "Supercalifragilisticexpialidocious", "style": {"fontFamily": "latin"}}
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
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := mustDecodeData(t, `{"items": [{"a":"Supercalifragilisticexpialidocious"}]}`)
	params := mustDecodeParams(t)
	_, _, _, diags, err := buildPageModel(tpl, data, params, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	var clipDiags []Diagnostic
	for _, d := range diags {
		if d.Code == DiagCodeTextClippedWidth {
			clipDiags = append(clipDiags, d)
		}
	}
	if len(clipDiags) != 2 {
		t.Fatalf("got %d DiagCodeTextClippedWidth diagnostics, want 2 (one from the table's column, one from the text element)", len(clipDiags))
	}
	// e3 (the TEXT element, declared SECOND) must come FIRST — this is
	// the pinned deviation from "declaration order within a band".
	if clipDiags[0].ElementID != "e3" {
		t.Errorf("diags[0].ElementID = %q, want %q — CURRENT (pinned, not endorsed) behaviour puts every text diagnostic before every table diagnostic within a band, regardless of declaration order", clipDiags[0].ElementID, "e3")
	}
	if clipDiags[1].ElementID != "e2" {
		t.Errorf("diags[1].ElementID = %q, want %q (the table's COLUMN id)", clipDiags[1].ElementID, "e2")
	}
}
