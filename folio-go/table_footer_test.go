package folio8

// Story 4.5: footer aggregates render, cover the whole collection, route
// through Story 3.3's one aggregate evaluation, and never orphan.
//
// Anchoring (D-000.68, the story's own "once, for all ten"): footerFixtureData
// is built so the rendered sum ("13,500.00"), count ("9") and avg ("45.00")
// are pairwise distinct and none equals any data cell's own rendered text —
// verified once, in TestFooterAnchorValuesAreDistinctFromDataCells, rather
// than re-argued in every other test here.

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// footerFixtureDoc is a three-column table over the SAME page/margins/
// header-footer-band geometry as table_pagination_test.go's
// multiRowTableDoc (200x150 page, 10pt margins, 10pt header/footer bands,
// 10pt table headerHeight, Noto Sans 8pt body) — chosen so this story's
// row-height arithmetic (~10,896mp/row, one line, default padding) is the
// SAME measured quantity that story's fixtures already exercise, not a
// second, independently-derived geometry.
//
// Column e2 (A) declares no footer (AC1's "carries no value" clause).
// Column e3 (B) declares footer:"sum" with an EXPLICIT footerOf/
// footerFormat (AC2's first arrival). Column e4 (C) declares footer:"avg"
// with NEITHER — both derived from its shape-2 bind (AC2's second
// arrival, D-1.4.1). alignRight, when true, aligns column e3 right
// (AC1's "honouring that column's align").
func footerFixtureDoc(footerOnA string, alignRight bool) string {
	aFooter := ""
	if footerOnA != "" {
		aFooter = fmt.Sprintf(`, "footer": %q`, footerOnA)
	}
	align := ""
	if alignRight {
		align = `, "align": "right"`
	}
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"%s},
          {"id": "e3", "label": "B", "width": 60, "bind": "{{formatNumber(row.b, \"#,##0.00\")}}", "footer": "sum", "footerOf": "items.b", "footerFormat": "#,##0.00"%s},
          {"id": "e4", "label": "C", "width": 60, "bind": "{{formatNumber(row.c, \"#,##0.00\")}}", "footer": "avg"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`, aFooter, align)
}

// footerFixtureDocWithSibling is footerFixtureDoc plus an UNRELATED text
// element ("e5") positioned so it lands on the SAME continuation page the
// footer does — AC6's negative witness. It belongs to no table, so Story
// 4.4's per-table RowDisplacement must not touch it: its drawn y must be
// exactly `declared y - page Shift`, with no displacement added.
func footerFixtureDocWithSibling() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 55, "bind": "{{row.a}}", "footer": "count"},
          {"id": "e3", "label": "B", "width": 55, "bind": "{{formatNumber(row.b, \"#,##0.00\")}}", "footer": "sum", "footerOf": "items.b", "footerFormat": "#,##0.00"},
          {"id": "e4", "label": "C", "width": 55, "bind": "{{formatNumber(row.c, \"#,##0.00\")}}", "footer": "avg"}
        ]},
      {"id": "e5", "type": "text", "x": 170, "y": 100, "width": 20, "height": 8, "value": "SIB", "style": {"fontFamily": "latin", "fontSize": 6}}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// footerFixtureDocCount declares footer:"count" on column A alongside
// sum/avg on B/C — this story's own three-kind AC1 fixture. No explicit
// align (AC1's default, "left").
func footerFixtureDocCount() string { return footerFixtureDoc("count", false) }

// footerFixtureDocRightAlignedSum aligns the sum column right, so AC1's
// "honouring that column's align" clause has a witness distinguishable
// from the default.
func footerFixtureDocRightAlignedSum() string { return footerFixtureDoc("", true) }

// footerFixtureData builds `rows` bound rows, as a JSON literal built
// from integer arithmetic alone (AD-1/AD-23: no binary float anywhere
// under this module, test code included — TestNoFloat64UnderModule scans
// it). B is 1100, 1200, ..., so sum(B) is 13,500.00 for rows=9 —
// comma-bearing under "#,##0.00" (AC2's anchor: formatted and
// unformatted spellings differ by more than whitespace). C alternates
// 1/100 so avg(C) is never one of its own operands (an arithmetic
// progression's mean IS its median member, which this deliberately is
// not).
func footerFixtureData(rows int) string {
	var b []byte
	b = append(b, `{"items":[`...)
	for i := 0; i < rows; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		c := "1"
		if i%2 == 1 {
			c = "100"
		}
		b = append(b, fmt.Sprintf(`{"a":"R%d","b":%d.00,"c":%s.00}`, i, 1000+(i+1)*100, c)...)
	}
	b = append(b, `]}`...)
	return string(b)
}

// footerFixtureExpected names, for `rows` built by footerFixtureData, the
// three rendered aggregate strings this file's tests anchor on. Computed
// once, by hand, rather than re-derived per test (D-000.68).
func footerFixtureExpectedSum(rows int) string {
	total := 0
	for i := 0; i < rows; i++ {
		total += 1000 + (i+1)*100
	}
	return commaFormat(total)
}

func footerFixtureExpectedAvg(rows int) (string, bool) {
	if rows == 0 {
		return "", false
	}
	sum := 0
	for i := 0; i < rows; i++ {
		if i%2 == 0 {
			sum += 1
		} else {
			sum += 100
		}
	}
	// avg is Story 3.3's own scale/rounding; this file's fixtures only
	// ever use rows counts whose avg terminates cleanly at 2 decimals
	// (9, 18, 20 — checked by TestFooterAnchorValuesAreDistinctFromDataCells).
	whole := sum / rows
	frac := (sum % rows) * 100 / rows
	return fmt.Sprintf("%d.%02d", whole, frac), true
}

func commaFormat(n int) string {
	s := fmt.Sprintf("%d", n)
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(c)
	}
	return out + ".00"
}

// TestFooterAnchorValuesAreDistinctFromDataCells is this file's "once,
// for all ten" anchoring proof (D-000.68): for the row counts this file
// actually uses, sum/count/avg are pairwise distinct and none equals any
// data cell's own rendered text.
func TestFooterAnchorValuesAreDistinctFromDataCells(t *testing.T) {
	for _, rows := range []int{6, 9, 20} {
		sum := footerFixtureExpectedSum(rows)
		count := fmt.Sprintf("%d", rows)
		avg, ok := footerFixtureExpectedAvg(rows)
		if !ok {
			t.Fatalf("rows=%d: avg precondition failed", rows)
		}
		if sum == count || sum == avg || count == avg {
			t.Fatalf("rows=%d: aggregates are not pairwise distinct: sum=%q count=%q avg=%q", rows, sum, count, avg)
		}
		for i := 0; i < rows; i++ {
			b := fmt.Sprintf("%s", commaFormat(1000+(i+1)*100))
			c := "1.00"
			if i%2 == 1 {
				c = "100.00"
			}
			a := fmt.Sprintf("R%d", i)
			for _, v := range []string{sum, count, avg} {
				if v == b || v == c || v == a {
					t.Fatalf("rows=%d row=%d: aggregate %q collides with a data cell (a=%q b=%q c=%q)", rows, i, v, a, b, c)
				}
			}
		}
	}
}

// renderFooterFixture renders docJSON/data through the public Render()
// entry point (AC8's own layer) and returns the result.
func renderFooterFixture(t *testing.T, docJSON, dataJSON string) Result {
	t.Helper()
	tpl, err := ParseTemplate([]byte(docJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data(dataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return res
}

// pageTextsOf extracts every drawn run's reconstructed text, per page, from
// rendered PDF bytes — table_header_repeat_test.go's own ToUnicode-backed
// extractor (mpParseToUnicode/splitPageContentStreams/mpExtractRuns),
// reused rather than re-implemented.
func pageTextsOf(t *testing.T, pdfBytes []byte) [][]string {
	t.Helper()
	streams := splitPageContentStreams(t, pdfBytes)
	cmap := mpParseToUnicode(t, pdfBytes)
	out := make([][]string, len(streams))
	for i, s := range streams {
		for _, run := range mpExtractRuns(t, s, cmap) {
			out[i] = append(out[i], run.text)
		}
	}
	return out
}

func pageContains(pages [][]string, page int, want string) bool {
	if page < 0 || page >= len(pages) {
		return false
	}
	for _, s := range pages[page] {
		if s == want {
			return true
		}
	}
	return false
}

func anyPageContains(pages [][]string, want string) (page int, ok bool) {
	for p, texts := range pages {
		for _, s := range texts {
			if s == want {
				return p, true
			}
		}
	}
	return -1, false
}

// --- AC1: sum, count and avg render per the column's configuration ---

func TestFooterAggregatesRenderPerColumnConfiguration(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(9))
	pages := pageTextsOf(t, res.Bytes)

	for _, want := range []string{"9", "13,500.00", "45.00"} {
		if _, ok := anyPageContains(pages, want); !ok {
			t.Errorf("no page carries the footer value %q", want)
		}
	}
}

// TestFooterHonoursColumnAlign is AC1's "honouring that column's align"
// clause: the sum column's footer VALUE run sits at a different X when
// the column declares align:"right" than when it does not — asserted by
// VALUE (the run's own drawn X, via its Tm), never by a bookkeeping flag.
func TestFooterHonoursColumnAlign(t *testing.T) {
	leftX := footerRunX(t, footerFixtureDocCount(), "13,500.00")
	rightX := footerRunX(t, footerFixtureDocRightAlignedSum(), "13,500.00")
	if leftX == rightX {
		t.Fatalf("sum footer's drawn X is %v under both left (default) and right align — column align is not being honoured", leftX)
	}
	if rightX <= leftX {
		t.Errorf("right-aligned footer value's X (%v) is not greater than the left-aligned one's (%v)", rightX, leftX)
	}
}

// footerRunX returns the named run's drawn X, as a scaled integer (micro-
// points: decimal string "12.345" -> 12345) rather than a float (AD-1/
// AD-23: no binary float anywhere under this module — TestNoFloat64UnderModule
// scans test code too).
func footerRunX(t *testing.T, docJSON, want string) int64 {
	t.Helper()
	res := renderFooterFixture(t, docJSON, footerFixtureData(9))
	streams := splitPageContentStreams(t, res.Bytes)
	cmap := mpParseToUnicode(t, res.Bytes)
	for _, s := range streams {
		for _, run := range mpExtractRuns(t, s, cmap) {
			if run.text == want {
				fields := strings.Fields(run.tm)
				if len(fields) < 6 {
					t.Fatalf("unexpected Tm shape %q", run.tm)
				}
				return decimalStringToMicros(t, fields[4])
			}
		}
	}
	t.Fatalf("run %q not found in rendered output", want)
	return 0
}

// decimalStringToMicros parses a plain decimal string ("12.345", "-3",
// "7.5") into an int64 count of 1/1,000,000ths — exact integer
// arithmetic, never strconv.ParseFloat.
func decimalStringToMicros(t *testing.T, s string) int64 {
	t.Helper()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, fracPart, _ := strings.Cut(s, ".")
	for len(fracPart) < 6 {
		fracPart += "0"
	}
	fracPart = fracPart[:6]
	i, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		t.Fatalf("decimalStringToMicros: %q: %v", s, err)
	}
	f, err := strconv.ParseInt(fracPart, 10, 64)
	if err != nil {
		t.Fatalf("decimalStringToMicros: %q: %v", s, err)
	}
	v := i*1_000_000 + f
	if neg {
		v = -v
	}
	return v
}

// --- AC2: footerFormat is applied, through the one existing formatter ---

func TestFooterFormatAppliesThroughExistingFormatter(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(9))
	pages := pageTextsOf(t, res.Bytes)

	// Both arrivals: e3 (sum) declares footerFormat EXPLICITLY;
	// e4 (avg) derives it from its shape-2 bind. Both must be FORMATTED
	// ("13,500.00", "45.00"), never the bare, unformatted spelling
	// ("13500", "45").
	if _, ok := anyPageContains(pages, "13,500.00"); !ok {
		t.Error("explicit footerFormat: formatted sum \"13,500.00\" not found")
	}
	if _, ok := anyPageContains(pages, "13500"); ok {
		t.Error("explicit footerFormat: unformatted sum \"13500\" was drawn instead")
	}
	if _, ok := anyPageContains(pages, "45.00"); !ok {
		t.Error("derived footerFormat: formatted avg \"45.00\" not found")
	}
}

// --- AC3: the aggregate covers the whole collection ---

// TestFooterAggregateCoversWholeCollectionNotJustItsOwnPage is AC3, and
// its non-vacuity mechanism is an ASSERTION rather than a narrated
// intention (this story's review, Major 5: the previous version's
// mandated precondition was an `if` block whose body held only a
// comment, and its one surviving check — "the page defined as the page
// containing 13,500.00 contains 13,500.00" — was implied by its own
// setup).
//
// The AC's real demand is that a per-page implementation cannot coincide
// with a whole-collection one. That is a statement about WHICH CELL
// carries WHICH VALUE, so it is asserted positionally, in the composed
// page model, rather than as "the string appears somewhere on the page":
// the footer's own page carries exactly one data row (row 8, whose own
// B cell renders "1,900.00" — the whole of what a per-page sum would
// produce), so
//
//	at row 8's Y      "1,900.00" is PRESENT   (the data cell)
//	at the footer's Y "13,500.00" is PRESENT  (the whole-collection sum)
//	at the footer's Y "1,900.00" is ABSENT    (a per-page sum would put it here)
//
// The first of those three is the presence precondition that keeps the
// third from being satisfied by "1,900.00" never being drawn at all.
func TestFooterAggregateCoversWholeCollectionNotJustItsOwnPage(t *testing.T) {
	tpl, err := ParseTemplate([]byte(footerFixtureDocCount()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, footerFixtureData(9)), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("presence precondition: fixture must paginate to >= 2 pages, got %d", len(pages))
	}
	p1 := pages[len(pages)-1]

	// Row 8's own Y is the Y of the run carrying its own "R8" marker; the
	// footer's Y is the next distinct run Y BELOW it — the footer row
	// sits directly under the page's last data row. Both are read from
	// the page model, and NEITHER is located by the value under test:
	// locating the footer row by "the run that says 13,500.00" would make
	// the whole assertion circular, and would make a per-page
	// implementation (which draws "1,900.00" there instead) look like a
	// footer that vanished rather than a footer that computed the wrong
	// thing.
	var rowY int64
	haveRowY := false
	for _, r := range p1.Runs {
		if r.SourceText == "R8" {
			rowY, haveRowY = int64(r.Y), true
		}
	}
	if !haveRowY {
		t.Fatal("presence precondition: row 8's own marker \"R8\" is not on the footer's page, so the page carries no data row to compare against")
	}
	var footerY int64
	haveFooterY := false
	for _, r := range p1.Runs {
		if y := int64(r.Y); y > rowY && (!haveFooterY || y < footerY) {
			footerY, haveFooterY = y, true
		}
	}
	if !haveFooterY {
		t.Fatal("presence precondition: nothing is drawn below row 8 on its own page — there is no footer row to inspect")
	}

	perPageAnswer := "1,900.00" // sum over the footer page's rows alone: row 8's B, and nothing else
	atRow, atFooter, wholeAtFooter := false, false, false
	for _, r := range p1.Runs {
		y := int64(r.Y)
		if r.SourceText == "13,500.00" && y == footerY {
			wholeAtFooter = true
		}
		if r.SourceText != perPageAnswer {
			continue
		}
		switch y {
		case rowY:
			atRow = true
		case footerY:
			atFooter = true
		}
	}
	if !wholeAtFooter {
		t.Errorf("the footer's own row does not carry the whole-collection sum \"13,500.00\"")
	}
	if !atRow {
		t.Fatalf("presence precondition: %q (what a per-page aggregate would produce) is not drawn as row 8's own data cell — "+
			"its absence from the footer's row below would then prove nothing", perPageAnswer)
	}
	if atFooter {
		t.Errorf("the footer's own row carries %q — the aggregate covered only the rows placed on the footer's own page, not the whole collection", perPageAnswer)
	}
}

// --- AC4: the footer uses the SAME aggregate evaluation as {{sum(...)}} ---

// TestFooterRoutesThroughTheSameAggregateEvaluationAsAnOrdinaryExpression
// is AC4/DW-7's BEHAVIOURAL half: an author-written
// {{sum(items.b)}}/{{avg(items.c)}}/{{count(items)}} expression,
// evaluated independently over the SAME data, must produce the IDENTICAL
// string the footer renders.
//
// IT RUNS OVER MORE THAN ONE DATASET, AND THAT IS THE POINT (this
// story's review, Blocker 2). The previous version compared against one
// fixture, so any rival that agreed on that one fixture — including a
// literal `return "13,500.00"` with no evaluation behind it at all, the
// exact mutation the review ran — passed untouched. A footer value is a
// FUNCTION OF THE DATA, and a function is not witnessed by one point:
// the loop below re-renders at two row counts whose aggregates all
// differ, and asserts up front that they differ, so a constant cannot
// satisfy both legs and a wrong-but-plausible constant cannot satisfy
// either silently.
//
// Its STRUCTURAL half is TestFooterCellExpressionNamesTheSharedAggregateFunctions
// below: this test proves the footer's answer tracks the data, that one
// proves the answer is reached by naming the shared aggregate rather
// than by a second implementation that happens to agree.
func TestFooterRoutesThroughTheSameAggregateEvaluationAsAnOrdinaryExpression(t *testing.T) {
	rowCounts := []int{9, 6}

	// Anti-constant precondition, asserted rather than assumed: the two
	// datasets really do produce different aggregates, so "agrees with
	// both" is a statement a constant cannot make.
	if a, b := footerFixtureExpectedSum(rowCounts[0]), footerFixtureExpectedSum(rowCounts[1]); a == b {
		t.Fatalf("precondition: both row counts produce the same sum %q — a constant would satisfy this test", a)
	}

	for _, rows := range rowCounts {
		data := mustDecodeData(t, footerFixtureData(rows))
		params := mustDecodeParams(t)
		fc := testFormatContext()
		scope := bind.NewScope(data, params)

		sumText, _, _, err := bind.Resolve(`{{formatNumber(sum(items.b), "#,##0.00")}}`, scope, fc, "test")
		if err != nil {
			t.Fatalf("rows=%d: ordinary sum expression: %v", rows, err)
		}
		countText, _, _, err := bind.Resolve(`{{formatNumber(count(items), "0")}}`, scope, fc, "test")
		if err != nil {
			t.Fatalf("rows=%d: ordinary count expression: %v", rows, err)
		}
		avgText, _, _, err := bind.Resolve(`{{formatNumber(avg(items.c), "#,##0.00")}}`, scope, fc, "test")
		if err != nil {
			t.Fatalf("rows=%d: ordinary avg expression: %v", rows, err)
		}

		res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(rows))
		pages := pageTextsOf(t, res.Bytes)
		for _, want := range []string{sumText, countText, avgText} {
			if _, ok := anyPageContains(pages, want); !ok {
				t.Errorf("rows=%d: footer does not carry %q, which an ordinary {{ }} expression over the identical data produces", rows, want)
			}
		}
	}
}

// TestFooterCellExpressionNamesTheSharedAggregateFunctions is AC4/DW-7's
// STRUCTURAL half — the footer-side analogue of internal/expr's own
// TestSumRoutesThroughSumDecimals / TestAvgRoutesThroughAvgDecimals
// (routing_arch_test.go), which cover the {{ }} half and, as the review
// established, cover ONLY the {{ }} half: the mutation the developer
// recorded for AC4 reddened nothing but Story 3.1a's module-wide reducer
// inventory, so DW-7 was retired against an instrument belonging to
// another story.
//
// What it pins: the text the footer hands to bind.Resolve is a
// formatNumber() call whose OPERAND is a call to the shared sum/count/avg
// — by AST, so whitespace and pattern choice cannot make it pass or fail
// — over the column's own resolved source path. Everything downstream of
// that (expr's closed function table, evalSum/evalAvg's routing through
// SumDecimals/AvgDecimals, and internal/expr's own routing guard) is then
// already witnessed. A footer that computed its own answer — a literal,
// an inline accumulator, a second reducer — produces some other text
// here and reddens, whether or not its answer happens to agree with the
// real one on this fixture's data.
func TestFooterCellExpressionNamesTheSharedAggregateFunctions(t *testing.T) {
	tpl, err := ParseTemplate([]byte(footerFixtureDocCount()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	tbl := footerFixtureTableOf(t, tpl)
	data := mustDecodeData(t, footerFixtureData(9))
	scope := bind.NewScope(data, mustDecodeParams(t))
	fc := testFormatContext()

	want := map[string]struct{ fn, path string }{
		"e2": {"count", "items"},
		"e3": {"sum", "items.b"},
		"e4": {"avg", "items.c"},
	}
	seen := 0
	for _, col := range tbl.Columns {
		w, ok := want[string(col.ID)]
		if !ok {
			continue
		}
		seen++
		text, terr := footerCellExprText(tpl, tbl, col, false, scope, fc)
		if terr != nil {
			t.Fatalf("column %s: footerCellExprText: %v", col.ID, terr)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(text, "{{"), "}}")
		if inner == text {
			t.Fatalf("column %s: footer expression %q is not a {{ }} interpolation at all", col.ID, text)
		}
		parsed, perr := expr.Parse(inner)
		if perr != nil {
			t.Fatalf("column %s: footer expression %q does not parse: %v", col.ID, text, perr)
		}
		outer, ok := parsed.(*expr.CallExpr)
		if !ok || outer.Name != "formatNumber" {
			t.Fatalf("column %s: footer expression %q is not a formatNumber() call (got %T)", col.ID, text, parsed)
		}
		agg, ok := outer.Args[0].(*expr.CallExpr)
		if !ok {
			t.Fatalf("column %s: formatNumber's operand is %T, not a call to an aggregate — the footer's value is not being computed by the shared evaluation", col.ID, outer.Args[0])
		}
		if agg.Name != w.fn {
			t.Errorf("column %s: footer routes through %s(), want %s()", col.ID, agg.Name, w.fn)
		}
		operand, ok := agg.Args[0].(*expr.PathExpr)
		if !ok {
			t.Fatalf("column %s: %s()'s operand is %T, not a data path", col.ID, agg.Name, agg.Args[0])
		}
		if got := strings.Join(operand.Segments, "."); got != w.path {
			t.Errorf("column %s: %s() is applied to %q, want %q", col.ID, agg.Name, got, w.path)
		}
	}
	if seen != len(want) {
		t.Fatalf("presence precondition: matched %d of %d footer columns — the fixture no longer carries all three aggregate kinds", seen, len(want))
	}
}

// footerFixtureTableOf returns the content band's one table extension.
func footerFixtureTableOf(t *testing.T, tpl *Template) template.TableExt {
	t.Helper()
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	for _, el := range bands[contentBandIndex].band.Elements {
		if el.Type == template.ElementTable && el.Table.Set {
			return el.Table.Value
		}
	}
	t.Fatal("the content band carries no table element")
	return template.TableExt{}
}

// --- AC5: orphan tie ---

func TestFooterOrphanTieMovesWithImmediatelyPrecedingRow(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(9))
	pages := pageTextsOf(t, res.Bytes)
	if len(pages) != 2 {
		t.Fatalf("presence precondition: 9-row fixture must paginate to exactly 2 pages, got %d", len(pages))
	}
	// Row 8's own marker cell ("R8") and the footer's sum must be on the
	// SAME (second) page, and row 8 must be ABSENT from page 0.
	if pageContains(pages, 0, "R8") {
		t.Error("row 8 (the immediately preceding row) is on page 0, not moved with the footer")
	}
	if !pageContains(pages, 1, "R8") {
		t.Error("row 8 is not on page 1 (the footer's own page) — the orphan tie did not fire")
	}
	if !pageContains(pages, 1, "13,500.00") {
		t.Fatal("the footer's own sum value is not on page 1")
	}
	// Rows 0..7 stay on page 0 (the tie moves the MINIMUM: one row, not
	// the whole table).
	for i := 0; i < 8; i++ {
		if !pageContains(pages, 0, fmt.Sprintf("R%d", i)) {
			t.Errorf("row %d expected on page 0, not found", i)
		}
	}
}

// TestFooterOrphanTiePresencePrecondition is AC5's own required
// precondition: the row that ties with the footer must fit its ORIGINAL
// page with room to spare — i.e. it would not have moved on its own.
// Checked at the layout level directly, against layout.Paginate's own
// (un-fixed) verdict for the SAME items, before the orphan-tie helper
// runs at all.
func TestFooterOrphanTiePresencePrecondition(t *testing.T) {
	tpl, err := ParseTemplate([]byte(footerFixtureDocCount()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	data := mustDecodeData(t, footerFixtureData(9))
	params := mustDecodeParams(t)
	geometry, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	fc := testFormatContext()
	contentRuns, tableRects, _, err := collectBandTableRuns(tpl, bands, contentBandIndex, data, params, fc, testShippedFontSet(), newFontCache(), nil)
	if err != nil {
		t.Fatalf("collectBandTableRuns: %v", err)
	}
	textRuns, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, data, params, testShippedFontSet(), newFontCache(), contentBandResolver, nil)
	if err != nil {
		t.Fatalf("collectBandTextRuns: %v", err)
	}
	contentRuns = append(textRuns, contentRuns...)
	items := itemsForTest(contentRuns, tableRects)

	// The UN-FIXED verdict: without the orphan tie, does row 8 already
	// land on page 0 with room to spare?
	naive, err := layout.Paginate(geometry, items)
	if err != nil {
		t.Fatalf("layout.Paginate: %v", err)
	}
	if len(naive.Pages) != 2 {
		t.Fatalf("presence precondition: naive (un-fixed) pagination must still be 2 pages, got %d — D-000.68's own measured invariant", len(naive.Pages))
	}
	// Row 8's own chrome rect is the last one this table emits before the
	// footer; find it and confirm it is on page 0 there (i.e. the naive
	// placement does NOT already orphan the footer onto row 8's page —
	// it puts the footer ALONE on page 1, per Finding 4).
	row8Page := -1
	for i := range items {
		if items[i].Group.Present && items[i].Group.Key.Index == 8 && !items[i].Group.Key.IsHeader {
			for p, pg := range naive.Pages {
				for _, r := range pg.ContentRects {
					if len(items[i].Rects) > 0 && r == items[i].Rects[0] {
						row8Page = p
					}
				}
			}
		}
	}
	if row8Page != 0 {
		t.Fatalf("presence precondition failed: row 8 lands on naive page %d, want 0 (it must fit its original page unaided)", row8Page)
	}
}

// --- AC6: the footer travels with its own table's displacement, and with
// nothing else's ---

// TestFooterTravelsWithItsOwnTablesDisplacementAndNothingElses is AC6.
// Every assertion is a POSITION BY VALUE — where something actually
// landed in the composed page model — never that a bookkeeping field was
// populated (Story 4.4's Blocker 1 shape). It carries BOTH halves in one
// test, as AC6 requires: the positive (the footer receives the same
// per-table displacement its table's rows do, so nothing overlaps) and
// the negative (an unrelated sibling on the same page does not move at
// all).
//
// Measured geometry for footerFixtureDocWithSibling at 9 rows, page 1
// (the continuation page, where Story 4.4's header repeat is honoured
// with a 10,000mp reservation):
//
//	repeated header   Y = 10,000  H = 10,000   -> occupies 10,000..20,000
//	row 8             Y = 20,000  H = 10,896   -> occupies 20,000..30,896
//	footer            Y = 30,896  H = 10,896   -> occupies 30,896..41,792
//	sibling "SIB"     Y = 12,832               -> declared 110,000 - Shift 97,168
//
// Without the footer's share of the displacement the footer would sit at
// 20,896 and OVERLAP row 8 — which is what the positive mutation
// produces, observed as a position rather than as a missing flag.
func TestFooterTravelsWithItsOwnTablesDisplacementAndNothingElses(t *testing.T) {
	tpl, err := ParseTemplate([]byte(footerFixtureDocWithSibling()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, footerFixtureData(9)), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("presence precondition: expected 2 pages, got %d", len(pages))
	}
	p1 := pages[1]

	runY := func(text string) (geomY int64, ok bool) {
		for _, r := range p1.Runs {
			if r.SourceText == text {
				return int64(r.Y), true
			}
		}
		return 0, false
	}

	// Presence precondition: page 1 really is a continuation page with the
	// header repeated on it — otherwise there is no displacement to test.
	hdrY, ok := runY("A")
	if !ok {
		t.Fatal("presence precondition: page 1 carries no repeated header label \"A\" — FR26's repeat is not active, so AC6 has nothing to measure")
	}
	if hdrY != 10_000 {
		t.Errorf("repeated header label Y = %d, want 10000", hdrY)
	}

	rowY, ok := runY("R8")
	if !ok {
		t.Fatal("presence precondition: page 1 carries no row-8 cell \"R8\"")
	}
	if rowY != 20_000 {
		t.Errorf("row 8's cell Y = %d, want 20000 (displaced by the table's own 10,000mp reservation)", rowY)
	}

	// POSITIVE HALF: the footer's own value runs receive the SAME
	// displacement, so they land BELOW row 8 rather than on top of it.
	for _, want := range []struct {
		text string
		y    int64
	}{{"9", 30_896}, {"13,500.00", 30_896}, {"45.00", 30_896}} {
		got, ok := runY(want.text)
		if !ok {
			t.Errorf("page 1 carries no footer value run %q", want.text)
			continue
		}
		if got != want.y {
			t.Errorf("footer value %q Y = %d, want %d — the footer did not receive its table's own per-page row displacement", want.text, got, want.y)
		}
	}

	// ...and nothing overlaps: the repeated header sits entirely above
	// row 8, and row 8 entirely above the footer. Asserted on the CHROME
	// rects' own extents, by value.
	var hdrBottom, rowTop, rowBottom, footerTop int64
	for _, r := range p1.Rects {
		top, bottom := int64(r.Y), int64(r.Y)+int64(r.H)
		switch top {
		case 10_000:
			hdrBottom = bottom
		case 20_000:
			rowTop, rowBottom = top, bottom
		case 30_896:
			footerTop = top
		}
	}
	if hdrBottom == 0 || rowTop == 0 || footerTop == 0 {
		t.Fatalf("presence precondition: page 1's three chrome bands were not all found (hdrBottom=%d rowTop=%d footerTop=%d)", hdrBottom, rowTop, footerTop)
	}
	if hdrBottom > rowTop {
		t.Errorf("the repeated header (bottom %d) overlaps row 8 (top %d)", hdrBottom, rowTop)
	}
	if rowBottom > footerTop {
		t.Errorf("row 8 (bottom %d) overlaps the footer (top %d)", rowBottom, footerTop)
	}

	// NEGATIVE HALF: an unrelated sibling on the SAME page, belonging to
	// no table, moved by the page's own Shift alone — never by the
	// table's per-table displacement (DECISION-3 of Story 4.4: the
	// displacement channel is per-table, never page-wide).
	sibY, ok := runY("SIB")
	if !ok {
		t.Fatal("presence precondition: page 1 carries no unrelated sibling run \"SIB\" — the negative witness is missing")
	}
	if sibY != 12_832 {
		t.Errorf("unrelated sibling Y = %d, want 12832 (declared 110000 minus the page's own Shift 97168, and NOTHING else) — the table's per-table displacement leaked onto an unrelated element", sibY)
	}
}

// --- AC7: both pagination passes agree ---

func TestBothPaginationPassesAgreeOnFooterPartition(t *testing.T) {
	tpl, err := ParseTemplate([]byte(footerFixtureDocCount()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	data := mustDecodeData(t, footerFixtureData(9))
	params := mustDecodeParams(t)
	geometry, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	fc := testFormatContext()
	visible, _, err := computeVisibility(bands, data, params, fc)
	if err != nil {
		t.Fatalf("computeVisibility: %v", err)
	}
	contentTableRuns, contentTableRects, _, err := collectBandTableRuns(tpl, bands, contentBandIndex, data, params, fc, testShippedFontSet(), newFontCache(), visible)
	if err != nil {
		t.Fatalf("collectBandTableRuns: %v", err)
	}
	contentRuns, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, data, params, testShippedFontSet(), newFontCache(), contentBandResolver, visible)
	if err != nil {
		t.Fatalf("collectBandTextRuns: %v", err)
	}
	contentRuns = append(contentRuns, contentTableRuns...)

	// PHASE A, exactly as predictDocument builds it.
	phaseAItems := contentColumnItems(contentRuns, nil, contentTableRects, visible, nil)
	phaseAPlan, _, err := paginateWithFooterOrphanFix(geometry, phaseAItems, footerOrphanTargetsFrom(phaseAItems))
	if err != nil {
		t.Fatalf("PHASE A paginate: %v", err)
	}

	// PHASE B, via itemsForTest's own transcription of paginateDocument's
	// wiring (table_pagination_test.go).
	phaseBItems := itemsForTest(contentRuns, contentTableRects)
	phaseBPlan, _, err := paginateWithFooterOrphanFix(geometry, phaseBItems, footerOrphanTargetsFrom(phaseBItems))
	if err != nil {
		t.Fatalf("PHASE B paginate: %v", err)
	}

	// The WHOLE partition, not just the footer's entry (this story's
	// review, Major 6). AC7's own text is "the per-page partition — data
	// rows AND the footer's page — is identical between the two", and
	// D-4.3.2's named defect is "two passes silently disagreeing": a
	// disagreement confined to the data rows is the larger half of the
	// partition and the half whose two builders append in DIFFERENT
	// ORDERS, so a guard that compares only the footer's entry is
	// satisfied by exactly the defect it exists to catch.
	//
	// Compared as a MAP over every group key either pass carries — the
	// header's, every data row's, and the footer's — in both directions,
	// so a key present in one pass and missing from the other is a
	// failure rather than an unnoticed gap.
	partitionA := groupPagePartition(phaseAPlan, phaseAItems)
	partitionB := groupPagePartition(phaseBPlan, phaseBItems)

	// Presence preconditions, taken against the FINAL pass (PHASE B),
	// which is the pass that decides what actually exists: the partition
	// really covers the whole table — one header, nine data rows and the
	// footer — so "the two maps agree" is not satisfied by two nearly-
	// empty maps. Asserted against B rather than A on purpose: the defect
	// this test exists to catch lives in A, so a precondition asserted
	// against A would abort the comparison instead of reporting it.
	const wantGroups = 1 + 9 + 1
	if len(partitionB) != wantGroups {
		t.Fatalf("presence precondition: PHASE B's partition covers %d groups, want %d (header + 9 rows + footer)", len(partitionB), wantGroups)
	}
	if _, ok := partitionB[layout.ItemGroupKey{ElementID: "e1", Index: footerGroupIndex}]; !ok {
		t.Fatal("presence precondition: PHASE B's partition carries no footer group at all")
	}
	if len(phaseBPlan.Pages) != 2 {
		t.Fatalf("presence precondition: fixture must paginate to 2 pages (the orphan rule must have changed the partition), got %d", len(phaseBPlan.Pages))
	}

	for key, pageA := range partitionA {
		pageB, ok := partitionB[key]
		if !ok {
			t.Errorf("group %+v is placed by PHASE A (page %d) and carried by PHASE B not at all", key, pageA)
			continue
		}
		if pageA != pageB {
			t.Errorf("the two passes disagree on group %+v: PHASE A=%d PHASE B=%d", key, pageA, pageB)
		}
	}
	for key := range partitionB {
		if _, ok := partitionA[key]; !ok {
			t.Errorf("group %+v is carried by PHASE B and by PHASE A not at all", key)
		}
	}

	// The page count is checked LAST and as an Errorf, never as the
	// leading Fatalf it used to be: D-000.68's "a count is a lossy set"
	// applies to this guard's own report, and AC7's named deletion
	// requires the failure to land ON THE PARTITION rather than merely on
	// a count that happened to move with it.
	if len(phaseAPlan.Pages) != len(phaseBPlan.Pages) {
		t.Errorf("page counts disagree: PHASE A=%d PHASE B=%d", len(phaseAPlan.Pages), len(phaseBPlan.Pages))
	}
}

// groupPagePartition is the per-page partition of plan, keyed by group:
// every group any item in `items` carries, mapped to the page that
// group's items landed on. Built from the plan's OWN page assignments,
// never re-derived from geometry.
func groupPagePartition(plan layout.Pagination, items []layout.ColumnItem) map[layout.ItemGroupKey]int {
	rectPage, runPage := refPageIndexes(plan)
	out := map[layout.ItemGroupKey]int{}
	for i := range items {
		g := items[i].Group
		if !g.Present {
			continue
		}
		if _, done := out[g.Key]; done {
			continue
		}
		if p, ok := pageOfGroup(items, g.Key, rectPage, runPage); ok {
			out[g.Key] = p
		}
	}
	return out
}

func refPageIndexes(plan layout.Pagination) (map[layout.RectRef]int, map[layout.TextRunRef]int) {
	rectPage := map[layout.RectRef]int{}
	runPage := map[layout.TextRunRef]int{}
	for p, pg := range plan.Pages {
		for _, r := range pg.ContentRects {
			rectPage[r] = p
		}
		for _, r := range pg.ContentRuns {
			runPage[r] = p
		}
	}
	return rectPage, runPage
}

// --- AC8: it holds through the public Render(), and this AC's LAYER is
// pinned: this assertion may not later be narrowed to a layout-level
// check (Story 4.3's own live regression — a table-beside-same-y element
// became unrenderable and escaped review because no test called
// Render() — is the grounds this AC exists on). ---

func TestFooterHoldsThroughPublicRender(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(9))
	pages := pageTextsOf(t, res.Bytes)
	if len(pages) != 2 {
		t.Fatalf("presence precondition: expected 2 pages, got %d", len(pages))
	}
	// The formatted sum lands on the page the footer moved to (page 1,
	// AC5), and on NO OTHER page.
	for p := range pages {
		has := pageContains(pages, p, "13,500.00")
		want := p == 1
		if has != want {
			t.Errorf("page %d carries the footer's sum %v, want %v", p, has, want)
		}
	}
	if got := countPageObjects(res.Bytes); got != len(pages) {
		t.Errorf("/Type /Page object count is %d, want %d", got, len(pages))
	}
	if got := readDeclaredCount(t, res.Bytes); got != len(pages) {
		t.Errorf("/Count is %d, want %d", got, len(pages))
	}
}

// --- AC9: empty collection ---

// AC9 HAS THREE OBSERVABLES, AND THEY GET THREE WITNESSES (this story's
// review, Blocker 4). The AC asserts a property across three reducers —
// sum's zero, count's zero, avg's Warning — and the first version of it
// carried ONE genuine provenance witness (avg's Caveat DataPath) while
// sum and count were asserted by rendered VALUE alone. A fallthrough that
// returned "0.00"/"0" without evaluating anything left avg on the real
// path, so the one witness kept certifying a column whose zero was never
// in doubt: the review's own mutation replaced both non-avg zeros with
// literals and the whole suite stayed green. D-4.5.3's demand — "a test
// that would FAIL if the zero arrived by fallthrough" — is a demand per
// reducer, not per AC, so the three live in three separately-named tests
// below and each has its own deletion-proof recorded in the Delivery Log.
//
// TestFooterOverEmptyCollectionRendersAndTerminates keeps AC9's OTHER
// clauses (the render succeeds, returns bytes, terminates, and the two
// zeros are NOT harmonised with each other — D-3.1a.2).
func TestFooterOverEmptyCollectionRendersAndTerminates(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(0))
	if len(res.Bytes) == 0 {
		t.Fatal("Render returned zero bytes for an empty collection")
	}
	pages := pageTextsOf(t, res.Bytes)
	if len(pages) != 1 {
		t.Fatalf("an empty-collection document should be one page, got %d", len(pages))
	}
	if !pageContains(pages, 0, "0") {
		t.Error("count footer over an empty collection did not render \"0\"")
	}
	if !pageContains(pages, 0, "0.00") {
		t.Error("sum footer over an empty collection did not render \"0.00\"")
	}
	// D-3.1a.2, by value: the two zeros are DELIBERATELY not harmonised —
	// count's is at its own scale, sum's at SumDecimals'. A developer who
	// harmonises them has re-decided a scale at a call site.
	if pageContains(pages, 0, "0") == pageContains(pages, 0, "0.00") && !pageContains(pages, 0, "0") {
		t.Error("neither empty-collection zero rendered at all")
	}
}

// footerEmptyCollectionExprText returns the expression text the footer
// synthesises for column colID over an EMPTY bound collection — the
// artifact a "zero by fallthrough" has to falsify in order to produce a
// plausible-looking zero without evaluating anything.
func footerEmptyCollectionExprText(t *testing.T, colID string) (string, bind.Scope, expr.FormatContext) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(footerFixtureDocCount()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	tbl := footerFixtureTableOf(t, tpl)
	scope := bind.NewScope(mustDecodeData(t, footerFixtureData(0)), mustDecodeParams(t))
	fc := testFormatContext()
	for _, col := range tbl.Columns {
		if string(col.ID) != colID {
			continue
		}
		text, terr := footerCellExprText(tpl, tbl, col, true, scope, fc)
		if terr != nil {
			t.Fatalf("column %s: footerCellExprText: %v", colID, terr)
		}
		return text, scope, fc
	}
	t.Fatalf("column %s is not in the fixture", colID)
	return "", bind.Scope{}, expr.FormatContext{}
}

// assertFooterZeroHasProvenance is the shared shape of AC9's sum and
// count witnesses: the expression the footer synthesises over an empty
// collection must NAME the shared aggregate (structural — a hardcoded
// zero cannot), and the value actually drawn must equal what resolving
// that same expression independently produces (behavioural — a hardcode
// placed downstream of the synthesis cannot).
func assertFooterZeroHasProvenance(t *testing.T, colID, wantFn, wantPath, wantDrawn string) {
	t.Helper()
	text, scope, fc := footerEmptyCollectionExprText(t, colID)

	inner := strings.TrimSuffix(strings.TrimPrefix(text, "{{"), "}}")
	if inner == text {
		t.Fatalf("column %s: empty-collection footer expression %q is not a {{ }} interpolation — the zero is not coming from an evaluation at all", colID, text)
	}
	parsed, perr := expr.Parse(inner)
	if perr != nil {
		t.Fatalf("column %s: empty-collection footer expression %q does not parse: %v", colID, text, perr)
	}
	outer, ok := parsed.(*expr.CallExpr)
	if !ok || outer.Name != "formatNumber" {
		t.Fatalf("column %s: empty-collection footer expression %q is not a formatNumber() call (got %T)", colID, text, parsed)
	}
	agg, ok := outer.Args[0].(*expr.CallExpr)
	if !ok {
		t.Fatalf("column %s: the empty-collection zero is not produced by an aggregate call — formatNumber's operand is %T", colID, outer.Args[0])
	}
	if agg.Name != wantFn {
		t.Errorf("column %s: empty-collection zero routes through %s(), want %s()", colID, agg.Name, wantFn)
	}
	path, ok := agg.Args[0].(*expr.PathExpr)
	if !ok {
		t.Fatalf("column %s: %s()'s operand is %T, not a data path", colID, agg.Name, agg.Args[0])
	}
	if got := strings.Join(path.Segments, "."); got != wantPath {
		t.Errorf("column %s: %s() is applied to %q, want %q", colID, agg.Name, got, wantPath)
	}

	resolved, _, _, rerr := bind.Resolve(text, scope, fc, colID)
	if rerr != nil {
		t.Fatalf("column %s: resolving the footer's own expression independently: %v", colID, rerr)
	}
	if resolved != wantDrawn {
		t.Errorf("column %s: the footer's own expression resolves to %q, want %q", colID, resolved, wantDrawn)
	}
	res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(0))
	pages := pageTextsOf(t, res.Bytes)
	if !pageContains(pages, 0, resolved) {
		t.Errorf("column %s: the rendered document does not carry %q, which the footer's own expression resolves to", colID, resolved)
	}
}

// TestFooterEmptyCollectionSumZeroComesFromTheRealEvaluation is AC9's
// sum observable, with provenance (D-4.5.3).
func TestFooterEmptyCollectionSumZeroComesFromTheRealEvaluation(t *testing.T) {
	assertFooterZeroHasProvenance(t, "e3", "sum", "items.b", "0.00")
}

// TestFooterEmptyCollectionCountZeroComesFromTheRealEvaluation is AC9's
// count observable, with provenance (D-4.5.3). Its zero renders at a
// DIFFERENT scale from sum's, deliberately (D-3.1a.2).
func TestFooterEmptyCollectionCountZeroComesFromTheRealEvaluation(t *testing.T) {
	assertFooterZeroHasProvenance(t, "e2", "count", "items", "0")
}

// TestFooterEmptyCollectionAvgWarningComesFromTheRealEvaluation is AC9's
// avg observable: the EXISTING DiagCodeEmptyAverage Warning, an empty
// cell, never an Error and never a new code — and the Warning's own
// DataPath, which only an implementation that actually evaluated the
// column's derived footerOf can know.
func TestFooterEmptyCollectionAvgWarningComesFromTheRealEvaluation(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocCount(), footerFixtureData(0))
	foundWarning := false
	for _, d := range res.Diagnostics {
		if d.Code != DiagCodeEmptyAverage || d.Severity != SeverityWarning {
			continue
		}
		foundWarning = true
		if d.DataPath != "items.c" {
			t.Errorf("empty-average warning's DataPath is %q, want \"items.c\" (the column's real derived footerOf) — a hardcoded fallthrough would not know this path", d.DataPath)
		}
	}
	if !foundWarning {
		t.Error("no DiagCodeEmptyAverage Warning was produced for the avg footer over an empty collection")
	}
}

// --- AC10: byte-neutrality ---

func TestFooterlessTableCarriesNoFooterRow(t *testing.T) {
	// multiRowTableDoc (table_pagination_test.go) declares NO footer on
	// either column — AC10's own fixture. If Story 4.5's new code path
	// were reachable for it, hasFooter would be true and a
	// tableRectSource with isFooterRow would appear; it must not.
	tpl, err := ParseTemplate([]byte(multiRowTableDoc(false)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	data := mustDecodeData(t, multiRowTableData(5, -1))
	params := mustDecodeParams(t)
	fc := testFormatContext()
	_, tableRects, _, err := collectBandTableRuns(tpl, bands, contentBandIndex, data, params, fc, testShippedFontSet(), newFontCache(), nil)
	if err != nil {
		t.Fatalf("collectBandTableRuns: %v", err)
	}
	for _, ts := range tableRects {
		if ts.isFooterRow {
			t.Fatal("a footer-less table produced a tableRectSource with isFooterRow set")
		}
	}
	if targets := footerOrphanTargetsFrom(itemsForTest(nil, tableRects)); len(targets) != 0 {
		t.Fatalf("a footer-less table produced %d footerOrphanTarget(s), want 0", len(targets))
	}

	// Re-render evidence (not a hash of committed files): two renders of
	// the identical footer-less document/data are byte-identical.
	res1, err := Render(tpl, Data(multiRowTableData(5, -1)), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	res2, err := Render(tpl, Data(multiRowTableData(5, -1)), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if len(res1.Bytes) != len(res2.Bytes) {
		t.Fatalf("byte length differs across renders of the identical footer-less document: %d vs %d", len(res1.Bytes), len(res2.Bytes))
	}
	for i := range res1.Bytes {
		if res1.Bytes[i] != res2.Bytes[i] {
			t.Fatalf("byte %d differs across renders of the identical footer-less document", i)
		}
	}
}

// --- Heavy test (D-000.4, R12): a real body, gated by FOLIO8_HEAVY=1, never
// a build tag — DW-21's own pattern (table_header_repeat_test.go). Appended
// to DW-21's recorded command in the same change (deferred-work.md and this
// file's heavyTestGateEnvVar doc comment, table_header_repeat_test.go). NOT
// run as part of the routine gate. ---

// TestFooterOrphanTieHoldsAcrossHundredsOfPagesWithByteStability is a
// HEAVY integration test: a 500-row footer table through the public
// Render(), confirming the footer's sum/count/avg appear exactly once,
// on the LAST page only, that the produced bytes are stable across two
// renders, and that rendering completes in bounded time.
func TestFooterOrphanTieHoldsAcrossHundredsOfPagesWithByteStability(t *testing.T) {
	if os.Getenv(heavyTestGateEnvVar) != "1" {
		t.Skipf("D-000.4: heavy/integration suite, run only at the Epic 4 boundary gate (set %s=1) — see table_header_repeat_test.go's heavyTestGateEnvVar doc comment for the exact command", heavyTestGateEnvVar)
	}
	const rows = 500
	tpl, err := ParseTemplate([]byte(footerFixtureDocCount()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := Data(footerFixtureData(rows))

	done := make(chan struct {
		res Result
		err error
	}, 1)
	go func() {
		res, rerr := Render(tpl, data, nil, testShippedFontSet())
		done <- struct {
			res Result
			err error
		}{res, rerr}
	}()
	const bound = 60 * time.Second
	var res Result
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Render: %v", r.err)
		}
		res = r.res
	case <-time.After(bound):
		t.Fatalf("Render did not return within %s for a %d-row table — bounded-time requirement", bound, rows)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("presence precondition: Render returned zero bytes")
	}

	res2, err := Render(tpl, data, nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if len(res.Bytes) != len(res2.Bytes) {
		t.Fatalf("byte length differs across two renders of the identical document: %d vs %d", len(res.Bytes), len(res2.Bytes))
	}
	for i := range res.Bytes {
		if res.Bytes[i] != res2.Bytes[i] {
			t.Fatalf("byte %d differs across two renders of the identical document", i)
		}
	}

	pages := pageTextsOf(t, res.Bytes)
	if len(pages) < 50 {
		t.Fatalf("presence precondition: a %d-row table must paginate to dozens of pages, got %d", rows, len(pages))
	}
	want := footerFixtureExpectedSum(rows)
	seen := 0
	for p := range pages {
		if pageContains(pages, p, want) {
			seen++
			if p != len(pages)-1 {
				t.Errorf("footer sum %q found on page %d, not on the last page (%d)", want, p, len(pages)-1)
			}
		}
	}
	if seen != 1 {
		t.Errorf("footer sum %q found on %d pages, want exactly 1", want, seen)
	}

	// The ORPHAN TIE this test is NAMED for (this story's review, Nit 11:
	// the body asserted byte stability, page count and the sum's single
	// appearance, but never that the last data row accompanied the
	// footer — the one thing the name claims). Asserted the way AC5's own
	// test does: by the moved row's IDENTITY, not by "some row is there".
	last := len(pages) - 1
	lastRow := fmt.Sprintf("R%d", rows-1)
	if !pageContains(pages, last, lastRow) {
		t.Errorf("the last data row %q is not on the final page alongside the footer — the orphan tie this test is named for did not hold at scale", lastRow)
	}
}

// --- Fence: DECISION-1's "never error" carve-out is footer-specific ---

// TestOverTallSingleRowStillOverflows is the fence this story's DECISION-1
// re-derivation requires: an over-tall SINGLE data row (no footer
// involved — Story 4.6's own subject) must still produce
// layout.OverflowError exactly as it does at HEAD. This story's
// never-error carve-out (paginateWithFooterOrphanFix) must never widen to
// swallow this case. Under this story's shape (the probe-then-merge
// helper never runs a second Paginate call for a table with no footer,
// and reaches its OverflowError-catching branch ONLY from its own merge
// attempt), this assertion is TRUE BY CONSTRUCTION rather than by a
// runtime bypass condition — written anyway, as the story's own rulings
// require, so a future change to this file that widens the catch reddens
// here rather than being discovered by Story 4.6.
// TestFooterAloneTooTallForTheWindowIsClippedRatherThanFatal is Story
// 4.5's own deferral, DISCHARGED HERE by Story 4.6 — the story 4.5 named.
//
// D-4.5.2 ruled that "'place it alone' assumes the footer FITS alone",
// and gave two acceptable answers when it does not: route it to FR44's
// clip, or declare it Story 4.6's subject. Story 4.5 took the second and
// guaranteed only TERMINATION, via the existing layout.OverflowError.
// Story 4.6 owns the answer, and it is the first of the two: AD-14 makes
// an over-tall table group a Warning beside the bytes, never fatal, and
// a footer row is a row in FR25's own wording. The deferral is closed;
// the test is rewritten rather than deleted so the inversion reads as
// planned.
//
// TWO OBSERVABLES, kept apart: (i) the footer CLIPS rather than erroring;
// (ii) the clip record identifies it as the FOOTER — not as data row
// index -1, which is footerGroupIndex's wire sentinel leaking at a
// reader. The message half of (ii) is asserted directly against the
// production message builder in
// TestClippedGroupDiagnosticNamesTheRoleNeverTheSentinel.
//
// AND THE ORPHAN TIE STANDS DOWN. A clipped footer is alone on a fresh
// page BY DESIGN, not orphaned, so paginateWithFooterOrphanFix must not
// merge it into the preceding row: the merged group is taller still, it
// would clip too, and the Warning would then name the PRECEDING ROW's
// index instead of the footer. That is asserted here as part of (ii).
func TestFooterAloneTooTallForTheWindowIsClippedRatherThanFatal(t *testing.T) {
	g := layout.PageGeometry{
		Width: 200_000, Height: 150_000,
		MarginTop: 10_000, MarginBottom: 10_000, MarginLeft: 10_000, MarginRight: 10_000,
		PageHeaderHeight: 10_000, PageFooterHeight: 10_000,
	}
	// contentHeight = 150,000 - 10,000*2 - 10,000*2 = 110,000mp. A footer
	// group taller than that (chrome rect + one line, both 200,000mp
	// tall) fits no window at all.
	footerKey := layout.ItemGroupKey{ElementID: "e1", Index: footerGroupIndex}
	items := []layout.ColumnItem{
		{ElementID: "row0", Top: 0, Bottom: 10_000, Rects: []layout.RectRef{0}, Group: layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: "e1", Index: 0}}},
		{ElementID: "e1", Top: 10_000, Bottom: 210_000, Rects: []layout.RectRef{1}, Group: layout.ItemGroup{Present: true, Key: footerKey}},
		{ElementID: "e1", Top: 10_000, Bottom: 210_000, Runs: []layout.TextRunRef{0}, Group: layout.ItemGroup{Present: true, Key: footerKey}},
	}
	targets := []footerOrphanTarget{{
		elementID:    "e1",
		footerKey:    footerKey,
		precedingKey: layout.ItemGroupKey{ElementID: "e1", Index: 0},
	}}

	// The bound is kept from Story 4.5's own test: a hang here is not a
	// red, it is a stuck package, and the select is what turns "it
	// returns" into an observable at all.
	type outcome struct {
		plan  layout.Pagination
		diags []Diagnostic
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		p, d, err := paginateWithFooterOrphanFix(g, items, targets)
		done <- outcome{p, d, err}
	}()
	select {
	case r := <-done:
		// (i) it clips.
		if r.err != nil {
			t.Fatalf("paginateWithFooterOrphanFix returned %T: %v — D-4.5.2's deferral is discharged: an over-tall footer group is CLIPPED (AD-14, never fatal), not an error", r.err, r.err)
		}
		if len(r.plan.Clipped) != 1 {
			t.Fatalf("plan.Clipped = %+v; want exactly one record for the one over-tall group", r.plan.Clipped)
		}
		c := r.plan.Clipped[0]

		// (ii) and the record says FOOTER, not "row -1".
		if c.Key != footerKey {
			t.Errorf("the clip record names group %+v; want the FOOTER's own key %+v.\nA record naming the preceding data row means the orphan tie merged a clipped footer into it — a clipped group is alone on its page by design, not orphaned, and merging it makes the Warning name the wrong row.", c.Key, footerKey)
		}
		if c.ItemHeight != 200_000 || c.ContentHeight != 110_000 {
			t.Errorf("clip reports %dmp against %dmp; want the footer group's own 200,000mp against the 110,000mp window", c.ItemHeight, c.ContentHeight)
		}
		// No orphan Warning: the footer was not orphaned, it was cut.
		for _, d := range r.diags {
			if d.Code == DiagCodeTableFooterOrphanSuppressed {
				t.Errorf("a %s Warning was emitted for a CLIPPED footer: %s\nThe orphan rule is not what failed here — the footer fits no page at all.", d.Code, d.Message)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("paginateWithFooterOrphanFix did not return within 5s — D-4.5.2 requires termination, never a hang")
	}
}

func TestOverTallSingleRowStillOverflows(t *testing.T) {
	items := []layout.ColumnItem{
		{ElementID: "e9", Top: 0, Bottom: 999_999_000, Runs: []layout.TextRunRef{0}},
	}
	g := layout.PageGeometry{
		Width: 200_000, Height: 150_000,
		MarginTop: 10_000, MarginBottom: 10_000, MarginLeft: 10_000, MarginRight: 10_000,
		PageHeaderHeight: 10_000, PageFooterHeight: 10_000,
	}
	_, _, err := paginateWithFooterOrphanFix(g, items, nil)
	if err == nil {
		t.Fatal("expected an OverflowError for a single item taller than the content window; got nil")
	}
	if _, ok := err.(*layout.OverflowError); !ok {
		t.Fatalf("expected *layout.OverflowError, got %T: %v", err, err)
	}
}

// --- D-000.67 part 2 sweep: the out-of-collection footerOf load error is
// CODED (Story 3.6 absorption gap, closed here) ---

// TestOutOfCollectionFooterOfCarriesTheUnresolvedCode pins the site the
// story's "Things the schema and the record could not resolve" note 2
// named: D-1.4.1 says TABLE_FOOTER_SOURCE_UNRESOLVED covers an
// "underivable OR out-of-collection source", but parse_bands.go's prefix
// check returned a PLAIN newLoadError while the two FORBIDDEN checks
// beside it were coded at Story 3.6. Swept in here under D-000.67 part 2
// (sweep count reported in the Delivery Log: 61 load-error sites examined
// in parse_bands.go, 3 now coded, 4 footer-related sites correctly left
// uncoded as malformed-template/type failures).
func TestOutOfCollectionFooterOfCarriesTheUnresolvedCode(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "B", "width": 60, "bind": "{{row.b}}", "footer": "sum", "footerOf": "elsewhere.b"}
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
`
	_, err := ParseTemplate([]byte(doc))
	if err == nil {
		t.Fatal("presence precondition: an out-of-collection footerOf must be a load error")
	}
	var re *RenderError
	if !errors.As(err, &re) {
		t.Fatalf("expected a *RenderError, got %T: %v", err, err)
	}
	if re.Diagnostic.Code != DiagCodeTableFooterSourceUnresolved {
		t.Errorf("out-of-collection footerOf carries code %q, want %q — D-1.4.1's UNRESOLVED covers the out-of-collection arm too",
			re.Diagnostic.Code, DiagCodeTableFooterSourceUnresolved)
	}
}

// --- Blocker 1 (this story's review): the "absent and underived"
// footerFormat path, and the fixture gap that hid it ---

// footerFixtureDocUnderivedFormat is the fixture the corpus did not have.
//
// WHY IT EXISTS, AND WHY ANOTHER ASSERTION AGAINST THE EXISTING CORPUS
// WOULD NOT DO. A default is only tested by an input that would render
// DIFFERENTLY under a wrong default. Before this fixture, the only column
// in the whole file reaching D-1.4.1's "absent and underived" arm was
// footerFixtureDocCount's `count` column — and a count is INTEGRAL, so
// the lossy default that shipped ("0", zero fraction digits) and a
// correct one produce the identical string. The wrong state and the right
// state were indistinguishable on the only data class that exercised the
// path, which is why a true total of 30.85 could render "31" with the
// whole suite green.
//
// Every column here reaches that arm — each declares a footer over a
// NUMERIC source while its own cell bind is shape 1 over a STRING field
// ({{row.a}}), so no footerFormat is explicit and none is derivable —
// and the data is deliberately NON-INTEGRAL.
func footerFixtureDocUnderivedFormat() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}", "footer": "count"},
          {"id": "e3", "label": "B", "width": 60, "bind": "{{row.a}}", "footer": "sum", "footerOf": "items.b"},
          {"id": "e4", "label": "C", "width": 60, "bind": "{{row.a}}", "footer": "avg", "footerOf": "items.b"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// footerFixtureDataNonIntegral is two rows whose B values carry real
// fraction digits and whose exact total (30.85) is NOT what either a
// zero-fraction-digit default ("31") or a fixed maximum-precision default
// ("30.8500") produces. The cells themselves render the A field, so no
// data cell can collide with a footer value (D-000.68).
func footerFixtureDataNonIntegral() string {
	return `{"items":[{"a":"R0","b":10.55},{"a":"R1","b":20.30}]}`
}

// TestFooterWithNoFooterFormatRendersUnformattedNotRounded is D-1.4.1's
// "absent and underived, the footer renders UNFORMATTED" arm, asserted on
// the one data class that can tell a correct default from a lossy one.
//
// The three expected spellings are pinned BY VALUE, and each rules out a
// different wrong default:
//
//	sum   "30.85"    not "31"      (the zero-fraction-digit default that shipped)
//	                 not "30.8500" (a fixed maximum-precision default)
//	                 not "30.85" with a grouping separator anywhere (unformatted means ungrouped)
//	count "2"        not "2.0000"  (an integral value keeps its own scale)
//	avg   "15.4250"  the pattern grammar's own maxPatternFractionDigits ceiling,
//	                 which avg reaches by construction (its scale is max operand
//	                 scale + avgExtraScale) — pinned so a change to that ceiling
//	                 is visible here rather than silent
func TestFooterWithNoFooterFormatRendersUnformattedNotRounded(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocUnderivedFormat(), footerFixtureDataNonIntegral())
	pages := pageTextsOf(t, res.Bytes)
	if len(pages) != 1 {
		t.Fatalf("presence precondition: the fixture must render as one page, got %d", len(pages))
	}

	// Presence precondition that SURVIVES D-000.80 part (a): the table
	// really rendered, witnessed by its own column labels (header chrome,
	// which part (a)'s deletion of DATA-row cell text leaves in place).
	// The separate claim that no data cell can collide with a footer
	// value is TestFooterUnderivedFixtureDataCellsCannotCollideWithFooterValues
	// below — deliberately a different test, because that one asserts the
	// data cells EXIST and so cannot itself survive part (a).
	for _, label := range []string{"A", "B", "C"} {
		if !pageContains(pages, 0, label) {
			t.Fatalf("presence precondition: column label %q is not drawn — the fixture is not rendering a table at all", label)
		}
	}

	if !pageContains(pages, 0, "30.85") {
		t.Errorf("a sum footer with NO footerFormat rendered %q; want the exact total \"30.85\" — D-1.4.1: absent and underived, the footer renders UNFORMATTED", pages[0])
	}
	for _, wrong := range []string{"31", "30.8500", "30.9"} {
		if pageContains(pages, 0, wrong) {
			t.Errorf("a sum footer with NO footerFormat rendered %q — the exact total is 30.85 and the value was altered at the display boundary", wrong)
		}
	}
	if !pageContains(pages, 0, "2") {
		t.Error("a count footer with NO footerFormat did not render \"2\"")
	}
	if pageContains(pages, 0, "2.0000") {
		t.Error("a count footer rendered \"2.0000\" — an integral aggregate must keep its own scale, not the widest the grammar allows")
	}
	if !pageContains(pages, 0, "15.4250") {
		t.Errorf("an avg footer with NO footerFormat rendered %q; want \"15.4250\" (avg's own scale, clamped to the pattern grammar's maxPatternFractionDigits ceiling)", pages[0])
	}
}

// TestFooterUnderivedFixtureDataCellsCannotCollideWithFooterValues is the
// underived-format fixture's own D-000.68 anchoring proof: every data
// cell it draws is a STRING field ("R0", "R1"), so no data cell can ever
// carry one of the footer's numeric values and
// TestFooterWithNoFooterFormatRendersUnformattedNotRounded cannot be
// observing a data cell by accident.
//
// D-000.80 part (a) is NOT claimed for THIS test, and the reason is
// stated rather than skipped: part (a) removes data-row cell text, and
// this test's whole subject is that the data cells are drawn and are
// disjoint from the footer's values. It is the anchoring, not the AC's
// witness; the witness above is the test that must survive part (a),
// and does.
func TestFooterUnderivedFixtureDataCellsCannotCollideWithFooterValues(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocUnderivedFormat(), footerFixtureDataNonIntegral())
	pages := pageTextsOf(t, res.Bytes)
	footerValues := []string{"30.85", "2", "15.4250"}
	for _, cell := range []string{"R0", "R1"} {
		if !pageContains(pages, 0, cell) {
			t.Fatalf("data cell %q is not drawn — the fixture is not exercising a real table", cell)
		}
		for _, v := range footerValues {
			if cell == v {
				t.Errorf("data cell %q is identical to footer value %q — the fixture cannot anchor anything", cell, v)
			}
		}
	}
}

// TestUnformattedPatternTracksTheValuesOwnScale is expr.UnformattedPattern's
// own unit witness: the pattern is a FUNCTION OF THE VALUE, never a fixed
// literal, and it groups nothing and pads nothing.
func TestUnformattedPatternTracksTheValuesOwnScale(t *testing.T) {
	for _, c := range []struct {
		coefficient int64
		exponent    int
		want        string
	}{
		{3085, -2, "0.00"},    // 30.85 — the review's own probe value
		{31, 0, "0"},          // an integral value keeps zero fraction digits
		{1500, -3, "0.000"},   // 1.500 — a declared trailing zero is part of the value (AD-23)
		{15425, -6, "0.0000"}, /* 0.015425 — beyond the ceiling, clamped */
		{5, 2, "0"},           // a positive exponent is still zero fraction digits
	} {
		if got := expr.UnformattedPattern(expr.Decimal{Coefficient: c.coefficient, Exponent: c.exponent}); got != c.want {
			t.Errorf("UnformattedPattern({%d, %d}) = %q, want %q", c.coefficient, c.exponent, got, c.want)
		}
		for _, forbidden := range []string{",", "#"} {
			if strings.Contains(expr.UnformattedPattern(expr.Decimal{Coefficient: c.coefficient, Exponent: c.exponent}), forbidden) {
				t.Errorf("UnformattedPattern({%d, %d}) contains %q — \"unformatted\" means no grouping and no padding", c.coefficient, c.exponent, forbidden)
			}
		}
	}
}

// --- Blocker 3 (this story's review): DECISION-2(b)'s carve-out, and the
// minted TABLE_FOOTER_ORPHAN_SUPPRESSED, get a behavioural witness ---

// footerFixtureDocUnsatisfiableTie is the fixture DECISION-2(b) actually
// describes and the story shipped without: a table whose footer row and
// its immediately preceding data row EACH fit the content window alone,
// but not together. Measured, not hand-derived:
//
//	content window = 61 - 10 - 10 - 10 - 10 = 21pt = 21,000mp
//	header row     = 10,000mp (declared headerHeight)
//	data row 0     = 10,896mp -> occupies 10,000..20,896, fits page 0 (21,000)
//	footer row     = 10,896mp -> would occupy 20,896..31,792, does NOT fit
//	merged group   = row 0 + footer = 21,792mp > 21,000mp -> unsatisfiable
//
// Column B's cell binds row.c while the footer sums items.b, so the
// footer's own value can never collide with a data cell's (D-000.68).
func footerFixtureDocUnsatisfiableTie() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}", "footer": "count"},
          {"id": "e3", "label": "B", "width": 60, "bind": "{{formatNumber(row.c, \"#,##0.00\")}}", "footer": "sum", "footerOf": "items.b", "footerFormat": "#,##0.00"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 61}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

func footerFixtureDataUnsatisfiableTie() string {
	return `{"items":[{"a":"R0","b":8100.00,"c":7.00}]}`
}

// TestUnsatisfiableFooterTieStillRendersWithTheFooterPlacedAlone is
// DECISION-2(b)'s LAYOUT observable: when the orphan rule cannot be
// honoured, the render must SUCCEED and place the footer alone rather
// than error. Before this test the whole carve-out could be deleted
// outright with the suite staying green (this story's review, Blocker 3).
//
// Its sibling TestUnsatisfiableFooterTieRecordsTheOrphanSuppressedWarning
// carries the DIAGNOSTIC observable. They are two tests, not two
// assertions in one, because the AC produces two observables and a single
// deletion screen returns one boolean for both — deleting the carve-out
// reddens on the layout half and lets the diagnostic half ride along
// unwitnessed, which is exactly how the minted code shipped with no test
// of any kind.
func TestUnsatisfiableFooterTieStillRendersWithTheFooterPlacedAlone(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocUnsatisfiableTie(), footerFixtureDataUnsatisfiableTie())
	pages := pageTextsOf(t, res.Bytes)
	if len(pages) != 2 {
		t.Fatalf("presence precondition: the unsatisfiable-tie fixture must paginate to 2 pages, got %d", len(pages))
	}
	// Presence precondition: the tie really was attempted and really was
	// unsatisfiable — the footer is on a later page than its only data
	// row, which is the orphan the rule exists to prevent and which this
	// geometry makes impossible to prevent.
	if !pageContains(pages, 0, "R0") {
		t.Fatal("presence precondition: row 0 is not on page 0, so the fixture is not the geometry this test describes")
	}
	if pageContains(pages, 1, "R0") {
		t.Fatal("presence precondition: row 0 came across with the footer, so the tie was satisfiable after all and this fixture measures nothing")
	}
	if !pageContains(pages, 1, "8,100.00") {
		t.Errorf("the footer's own sum is not on page 1: page 1 carries %q — the footer was not placed at all", pages[1])
	}
	if !pageContains(pages, 1, "1") {
		t.Errorf("the footer's own count is not on page 1: page 1 carries %q", pages[1])
	}
}

// TestUnsatisfiableFooterTieRecordsTheOrphanSuppressedWarning is
// DECISION-2(b)'s DIAGNOSTIC observable and D-4.5.1's minted code's only
// witness: the suppression is RECORDED, never silent. Deleting the
// Diagnostic construction while leaving the suppression itself in place
// reddens here and nowhere else.
func TestUnsatisfiableFooterTieRecordsTheOrphanSuppressedWarning(t *testing.T) {
	res := renderFooterFixture(t, footerFixtureDocUnsatisfiableTie(), footerFixtureDataUnsatisfiableTie())
	found := 0
	for _, d := range res.Diagnostics {
		if d.Code != DiagCodeTableFooterOrphanSuppressed {
			continue
		}
		found++
		if d.Severity != SeverityWarning {
			t.Errorf("TABLE_FOOTER_ORPHAN_SUPPRESSED has severity %q, want a Warning — AD-14 forbids an Error here", d.Severity)
		}
		if d.ElementID != "e1" {
			t.Errorf("TABLE_FOOTER_ORPHAN_SUPPRESSED names element %q, want \"e1\" (the table whose tie could not be honoured)", d.ElementID)
		}
		if !strings.Contains(d.Message, "placed alone") {
			t.Errorf("the Warning's message does not say what happened to the footer: %q", d.Message)
		}
	}
	if found != 1 {
		t.Errorf("Result.Diagnostics carries %d TABLE_FOOTER_ORPHAN_SUPPRESSED Warning(s), want exactly 1", found)
	}
}

// TestUnsatisfiableTieForOneTableDoesNotAbandonAnothersTie is this
// story's review, Finding 9: `toMerge` may hold candidates for several
// tables, and the first shape merged them all into ONE Paginate call, so
// a single pathological table's OverflowError discarded the whole merged
// plan — silently orphaning every OTHER table's footer, and emitting a
// Warning naming each of them as the cause.
//
// Built at the layout level, with two tables in one column, because the
// property is about which candidate is reverted and which Warning is
// emitted — not about glyphs. Table "bad" cannot tie (its row plus its
// footer exceed the 110,000mp window); table "ok" can, easily.
func TestUnsatisfiableTieForOneTableDoesNotAbandonAnothersTie(t *testing.T) {
	g := layout.PageGeometry{
		Width: 200_000, Height: 150_000,
		MarginTop: 10_000, MarginBottom: 10_000, MarginLeft: 10_000, MarginRight: 10_000,
		PageHeaderHeight: 10_000, PageFooterHeight: 10_000,
	} // contentHeight = 110,000mp
	group := func(id string, index int) layout.ItemGroup {
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: id, Index: index}}
	}
	// Extents are page-ABSOLUTE, so the first window runs from
	// contentTop (10,000mp) to contentTop+contentHeight (120,000mp) —
	// measured with a throwaway probe rather than hand-derived, because
	// the sweep slides its window WITHIN a page and only advances the
	// page when the current one already carries something.
	//
	// Both tables are orphaned by pass 1 (verified below as a presence
	// precondition — each one's footer lands on a later page than its
	// own row 0), so both are merge candidates:
	//
	//	bad row 0    10,000..65,000    page 0
	//	bad footer   65,000..122,000   page 1  -> bad is orphaned
	//	ok  row 0   122,000..170,000   page 1
	//	ok  footer  170,000..178,000   page 2  -> ok is orphaned
	//
	// ok's merged group spans 122,000..178,000 = 56,000mp and fits the
	// 110,000mp window comfortably. bad's spans 10,000..122,000 =
	// 112,000mp and fits no window at all, though each of its two
	// members fits alone — so exactly one of the two ties is
	// unsatisfiable, which is the discrimination this test needs.
	items := []layout.ColumnItem{
		{ElementID: "bad", Top: 10_000, Bottom: 65_000, Rects: []layout.RectRef{0}, Group: group("bad", 0)},
		{ElementID: "bad", Top: 65_000, Bottom: 122_000, Rects: []layout.RectRef{1}, Group: group("bad", footerGroupIndex)},
		{ElementID: "ok", Top: 122_000, Bottom: 170_000, Rects: []layout.RectRef{2}, Group: group("ok", 0)},
		{ElementID: "ok", Top: 170_000, Bottom: 178_000, Rects: []layout.RectRef{3}, Group: group("ok", footerGroupIndex)},
	}
	targets := footerOrphanTargetsFrom(items)
	if len(targets) != 2 {
		t.Fatalf("presence precondition: expected 2 orphan-tie candidates (one per table), got %d", len(targets))
	}

	// Presence precondition: pass 1 really orphans BOTH footers, so
	// "only one Warning" below is a statement about scoping and not an
	// artifact of only one table ever being a candidate.
	naive, nerr := layout.Paginate(g, items)
	if nerr != nil {
		t.Fatalf("un-fixed layout.Paginate: %v", nerr)
	}
	nRect, nRun := refPageIndexes(naive)
	for _, id := range []string{"bad", "ok"} {
		rowPage, ok1 := pageOfGroup(items, layout.ItemGroupKey{ElementID: id, Index: 0}, nRect, nRun)
		footerPage, ok2 := pageOfGroup(items, layout.ItemGroupKey{ElementID: id, Index: footerGroupIndex}, nRect, nRun)
		if !ok1 || !ok2 {
			t.Fatalf("presence precondition: table %q was not fully placed by the un-fixed pagination", id)
		}
		if rowPage == footerPage {
			t.Fatalf("presence precondition: table %q's footer is NOT orphaned by the un-fixed pagination (row and footer both on page %d), so it is not a merge candidate at all", id, rowPage)
		}
	}

	plan, diags, err := paginateWithFooterOrphanFix(g, items, targets)
	if err != nil {
		t.Fatalf("paginateWithFooterOrphanFix: %v — DECISION-2(b) rules this never errors", err)
	}

	// The Warning names ONLY the table that could not fit.
	if len(diags) != 1 {
		t.Fatalf("got %d suppression Warning(s), want exactly 1 (only \"bad\" is unsatisfiable): %+v", len(diags), diags)
	}
	if diags[0].ElementID != "bad" {
		t.Errorf("the suppression Warning names %q, want \"bad\" — a table whose tie succeeded was reported as the cause", diags[0].ElementID)
	}

	// ...and "ok" keeps its tie: its footer lands on the same page as its
	// own row 0.
	rectPage, runPage := refPageIndexes(plan)
	okRow, ok1 := pageOfGroup(items, layout.ItemGroupKey{ElementID: "ok", Index: 0}, rectPage, runPage)
	okFooter, ok2 := pageOfGroup(items, layout.ItemGroupKey{ElementID: "ok", Index: footerGroupIndex}, rectPage, runPage)
	if !ok1 || !ok2 {
		t.Fatal("presence precondition: table \"ok\"'s row or footer was not placed at all")
	}
	if okRow != okFooter {
		t.Errorf("table \"ok\"'s footer landed on page %d and its row 0 on page %d — its tie was abandoned because ANOTHER table's tie was unsatisfiable", okFooter, okRow)
	}
}

// TestFooterMergeRewritesOnlyFooterGroupKeys is D-4.5.4's fence, aimed at
// the property it is actually about (this story's review, Finding 10).
//
// TestOverTallSingleRowStillOverflows below fences PASS 1's error
// passthrough: its fixture returns before any merge is attempted, so its
// own doc comment concedes the assertion is true BY CONSTRUCTION. The
// carve-out's SCOPE — "nothing but a merged footer group can produce the
// OverflowError this file catches" — rests on a different fact: that the
// merge rewrites footer group keys and nothing else. That is a property
// of applyFooterMerge, it is executable, and it is asserted here: every
// key the merge changes must have been a footer key, and every other
// item must come through untouched.
func TestFooterMergeRewritesOnlyFooterGroupKeys(t *testing.T) {
	group := func(id string, index int, header bool) layout.ItemGroup {
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: id, Index: index, IsHeader: header}}
	}
	items := []layout.ColumnItem{
		{ElementID: "e1", Group: group("e1", 0, true)},
		{ElementID: "e1", Group: group("e1", 0, false)},
		{ElementID: "e1", Group: group("e1", 1, false)},
		{ElementID: "e1", Group: group("e1", footerGroupIndex, false)},
		{ElementID: "e9"}, // ungrouped sibling
	}
	targets := []footerOrphanTarget{{
		elementID:    "e1",
		footerKey:    layout.ItemGroupKey{ElementID: "e1", Index: footerGroupIndex},
		precedingKey: layout.ItemGroupKey{ElementID: "e1", Index: 1},
	}}

	merged := applyFooterMerge(items, targets)
	if len(merged) != len(items) {
		t.Fatalf("applyFooterMerge returned %d items for %d inputs", len(merged), len(items))
	}
	changed := 0
	for i := range items {
		if merged[i].Group.Key == items[i].Group.Key {
			continue
		}
		changed++
		if items[i].Group.Key.Index != footerGroupIndex || items[i].Group.Key.IsHeader {
			t.Errorf("applyFooterMerge rewrote a NON-footer group key %+v -> %+v — the never-error carve-out is no longer keyed on footer-ness",
				items[i].Group.Key, merged[i].Group.Key)
		}
		if merged[i].Group.Key != targets[0].precedingKey {
			t.Errorf("the footer's key was rewritten to %+v, want the preceding row's %+v", merged[i].Group.Key, targets[0].precedingKey)
		}
	}
	if changed != 1 {
		t.Errorf("applyFooterMerge changed %d group key(s), want exactly 1 (the footer's)", changed)
	}
	// The input is never mutated: the caller still has pass 1's own items.
	if items[3].Group.Key.Index != footerGroupIndex {
		t.Error("applyFooterMerge mutated its input slice — pass 1's placement would be read through rewritten keys")
	}
}
