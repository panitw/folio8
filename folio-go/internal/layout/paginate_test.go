package layout

// Story 2.6's pagination, asserted against DECLARATIVE TABLES (D-000.45).
//
// Every expectation here is a HAND-DERIVED INTEGER, and the arithmetic that
// produced it is written out beside it. Nothing is computed from the code
// under test, and no assertion is phrased as a DIRECTION: "more content
// yields more pages" is monotone and is satisfied by a paginator that
// returns 2, 5 or 900 pages for the same input.
//
// D-000.33 is why the line->page PARTITION is pinned by value rather than by
// a conservation law. "Every line appears on exactly one page" holds for ANY
// monotone boundary function, including the degenerate one that puts
// everything on page 0 — which is the pre-2.6 behaviour, i.e. THE DEFECT.
// So the boundary INDEX is asserted, never the sum.

import (
	"errors"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// The test geometry, chosen so every expectation below is a round number a
// reader can check by hand rather than by running the code.
//
//	pageHeight        200,000
//	marginTop          30,000
//	marginBottom       42,000
//	pageHeaderHeight   18,000
//	pageFooterHeight   10,000
//
// ContentHeight = 200000 − 30000 − 42000 − 18000 − 10000 = 100,000
// Content band origin = pageHeaderHeight             =  18,000
//
// The five geometric inputs are PAIRWISE DISTINCT (30000, 42000, 18000,
// 10000, 200000), so a SUBSTITUTION of one for another — the error class
// Story 2.5 measured as invisible in every pre-2.5 fixture — cannot survive
// here either.
func testGeometry() PageGeometry {
	return PageGeometry{
		Width:            150000,
		Height:           200000,
		MarginTop:        30000,
		MarginBottom:     42000,
		MarginLeft:       36000,
		MarginRight:      24000,
		PageHeaderHeight: 18000,
		PageFooterHeight: 10000,
	}
}

const (
	testContentHeight geom.Length = 100000
	testContentTop    geom.Length = 18000
)

// stackedLines builds n lines stacked from the content band's top, each
// `height` tall with the SAME value as its advance — the Noto Sans case,
// where hhea lineGap is 0 and so advance == ascent+descent exactly.
func stackedLines(n int, height geom.Length) []ColumnItem {
	items := make([]ColumnItem, 0, n)
	for i := range n {
		top := testContentTop + geom.Length(int64(i))*height
		items = append(items, ColumnItem{
			ElementID: "e1",
			Top:       top,
			Bottom:    top + height,
			Runs:      []TextRunRef{TextRunRef(i)},
		})
	}
	return items
}

// TestPaginatePageCountFromADeclaredTable is AC2.
//
// PRESENCE PRECONDITION: every row but the zero-content one asserts the
// input actually carries items, so "the page count is 1" cannot pass by
// paginating nothing.
func TestPaginatePageCountFromADeclaredTable(t *testing.T) {
	g := testGeometry()
	if got := ContentHeight(g); got != testContentHeight {
		t.Fatalf("presence precondition: the test geometry's content height is %d, not the %d every expectation below is derived from", got, testContentHeight)
	}
	if got := Origins(g).Content; got != testContentTop {
		t.Fatalf("presence precondition: the test geometry's content origin is %d, not %d", got, testContentTop)
	}

	table := []struct {
		name      string
		items     []ColumnItem
		wantPages int
		why       string
	}{
		{
			name:      "a document with no content at all is ONE page",
			items:     nil,
			wantPages: 1,
			why:       "a page header and page footer with an empty content band is a legitimate document; a zero-page PDF is not",
		},
		{
			name:      "content that fits the window EXACTLY is one page",
			items:     stackedLines(10, 10000),
			wantPages: 1,
			why:       "10 lines x 10,000 = 100,000, and the last line's bottom is 18,000+100,000 = 118,000, which is exactly the window's bottom edge (18,000+100,000). Fits ENTIRELY, so no window advance.",
		},
		{
			name:      "content ONE LINE over the window is two pages",
			items:     stackedLines(11, 10000),
			wantPages: 2,
			why:       "line 10 spans 118,000..128,000; the window ends at 118,000, so it does not fit ENTIRELY and starts window 1",
		},
		{
			name:      "content one MILLIPOINT over the window is two pages",
			items:     append(stackedLines(10, 10000), ColumnItem{ElementID: "e2", Top: 118000, Bottom: 118001, Runs: []TextRunRef{99}}),
			wantPages: 2,
			why:       "the 11th item's bottom is 118,001, one millipoint past the window's 118,000 edge. One millipoint is enough: the rule is 'contains it ENTIRELY', not 'mostly'.",
		},
		{
			name:      "content spanning three windows is three pages",
			items:     stackedLines(21, 10000),
			wantPages: 3,
			why:       "lines 0-9 fill window 0; line 10 starts window 1 at 118,000 and lines 10-19 fill it (last bottom 218,000 = 118,000+100,000); line 20 starts window 2",
		},
		{
			// Story 2.6 finisher, Finding 13: EVERY row above that expects
			// 2 pages places the offending item's TOP exactly ON the
			// window edge (118,000) or past it — which the REJECTED model
			// ("a line belongs to whichever page contains its TOP") also
			// puts on page 1, so those rows cannot tell the ruled model
			// from the rejected one. This row's item has its TOP strictly
			// INSIDE window 0 (108,001 < 118,000) and its BOTTOM strictly
			// OUTSIDE it (118,001 > 118,000): the rejected "contains its
			// top" model would say this item fits window 0 entirely
			// (wantPages 1) and is wrong; the ruled "contains it ENTIRELY"
			// model correctly starts window 1, discriminating the two in
			// the declared table itself rather than only in
			// TestPaginateKeepsTheLeadingAcrossAWindowBoundary, which
			// pins the discrimination but is not part of AC2's table.
			name:      "an item whose TOP is inside the window but whose BOTTOM is not is two pages",
			items:     append(stackedLines(10, 10000), ColumnItem{ElementID: "e2", Top: 108001, Bottom: 118001, Runs: []TextRunRef{99}}),
			wantPages: 2,
			why:       "the 11th item spans 108,001..118,001: its TOP (108,001) is inside window 0 ([18,000..118,000]) but its BOTTOM (118,001) is not, so it does not fit ENTIRELY and starts window 1 — a model that placed it by its TOP alone would keep it on page 0",
		},
	}

	for _, row := range table {
		t.Run(row.name, func(t *testing.T) {
			if row.wantPages > 1 && len(row.items) == 0 {
				t.Fatal("presence precondition: a row expecting more than one page declares no items")
			}
			plan, err := Paginate(g, row.items)
			if err != nil {
				t.Fatalf("Paginate: %v", err)
			}
			if len(plan.Pages) != row.wantPages {
				t.Errorf("Paginate produced %d pages; the declared value is %d.\nwhy: %s",
					len(plan.Pages), row.wantPages, row.why)
			}
		})
	}
}

// TestPaginatePinsTheLineToPagePartitionByValue is AC4.
//
// This is the assertion D-000.33 requires and the one the pre-2.6 code would
// FAIL: it put every line on page 0, which satisfies every conservation law
// anyone could write about the partition.
func TestPaginatePinsTheLineToPagePartitionByValue(t *testing.T) {
	g := testGeometry()

	// 21 lines of 10,000 from the content top. Hand-derived:
	//
	//	window 0 = [18,000 .. 118,000]  <- lines 0..9   (line 9 ends at 118,000)
	//	line 10 spans 118,000..128,000, does NOT fit -> starts window 1
	//	window 1 = [118,000 .. 218,000] <- lines 10..19 (line 19 ends at 218,000)
	//	line 20 spans 218,000..228,000, does NOT fit -> starts window 2
	items := stackedLines(21, 10000)
	wantPageOfLine := []int{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // lines 0-9
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // lines 10-19
		2, // line 20
	}
	wantShift := []geom.Length{0, 100000, 200000} // windowStart − contentTop

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != len(wantShift) {
		t.Fatalf("Paginate produced %d pages; the declared partition has %d", len(plan.Pages), len(wantShift))
	}

	// The observed partition, read back off the plan.
	gotPageOfLine := make([]int, len(items))
	for i := range gotPageOfLine {
		gotPageOfLine[i] = -1
	}
	for p, page := range plan.Pages {
		// AC4's SECOND half: every page in the partition is NON-EMPTY. A
		// conservation law is satisfied by a degenerate partition; this is
		// what rules the degenerate one out.
		if len(page.ContentRuns) == 0 && len(page.ContentImages) == 0 {
			t.Errorf("page %d carries no content at all — the window advances to the first UNPLACED item, so no page can be empty", p)
		}
		for _, ref := range page.ContentRuns {
			gotPageOfLine[int(ref)] = p
		}
		if page.Shift != wantShift[p] {
			t.Errorf("page %d's window shift is %d; the declared value is %d", p, page.Shift, wantShift[p])
		}
	}

	for i, want := range wantPageOfLine {
		if gotPageOfLine[i] != want {
			t.Errorf("line %d landed on page %d; the declared partition puts it on page %d",
				i, gotPageOfLine[i], want)
		}
	}
}

// TestPaginateKeepsTheLeadingAcrossAWindowBoundary is the property that
// eliminated the two rejected models, asserted so neither can be
// reintroduced without a red test.
//
// The FIXED-GRID-WITH-PER-ITEM-BUMP model puts the straddling line flush at
// the next window's top and leaves the following line where it was, which
// makes the gap between them SMALLER than one advance — the two lines
// OVERLAP. That is a worse defect than the one this story fixes, and it is
// why the window slides instead.
func TestPaginateKeepsTheLeadingAcrossAWindowBoundary(t *testing.T) {
	g := testGeometry()

	// Lines of 30,000, advance 30,000, window 100,000:
	//	line 0: 18,000.. 48,000   fits
	//	line 1: 48,000.. 78,000   fits
	//	line 2: 78,000..108,000   fits (window ends 118,000)
	//	line 3: 108,000..138,000  does NOT fit -> starts window 1 at 108,000
	//	line 4: 138,000..168,000  fits window 1 = [108,000..208,000]
	const advance geom.Length = 30000
	items := stackedLines(5, advance)

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != 2 {
		t.Fatalf("presence precondition: this fixture must straddle exactly one boundary; got %d pages, want 2", len(plan.Pages))
	}

	// Page-relative Y of a line is its column Top minus the page's shift.
	shift := plan.Pages[1].Shift
	if shift != 90000 {
		t.Errorf("window 1's shift is %d; hand-derived it is 108,000−18,000 = 90,000", shift)
	}
	line3Y := items[3].Top - shift
	line4Y := items[4].Top - shift
	if line3Y != testContentTop {
		t.Errorf("the straddling line 3 renders at page-relative Y %d; it must render at the content band's top, %d", line3Y, testContentTop)
	}
	if gap := line4Y - line3Y; gap != advance {
		t.Errorf("the gap between line 3 and line 4 across the page boundary is %d; it must still be exactly one advance, %d.\n"+
			"A smaller gap means the two lines OVERLAP — the defect the fixed-grid-with-bump model produces.", gap, advance)
	}
}

// TestPaginateNeverProducesAnEmptyPage is the property the ruling named
// explicitly: windows advance to the first UNPLACED item, not by a fixed
// content height.
//
// This is exactly what a `ceil(lowestBottom / contentHeight)` page count
// would get wrong — and that spelling is the one a reader is most likely to
// reintroduce, because it is the arithmetic DN-1's option (A) originally
// stated before it was withdrawn.
func TestPaginateNeverProducesAnEmptyPage(t *testing.T) {
	g := testGeometry()

	// One short line at the top, and one element declared FIFTY WINDOWS
	// below it with nothing in between.
	items := []ColumnItem{
		{ElementID: "e1", Top: testContentTop, Bottom: testContentTop + 10000, Runs: []TextRunRef{0}},
		{ElementID: "e2", Top: testContentTop + 5000000, Bottom: testContentTop + 5010000, Runs: []TextRunRef{1}},
	}

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}

	// ceil(5,010,000 / 100,000) = 51. The ruled model gives 2.
	if len(plan.Pages) != 2 {
		t.Errorf("Paginate produced %d pages for two items 5,000,000mp apart; the declared value is 2.\n"+
			"A count of 51 means `ceil(lowestBottom / contentHeight)` has been reintroduced: the window advances "+
			"to the first UNPLACED item, so a far-below element starts the next window rather than generating "+
			"49 blank pages before it.", len(plan.Pages))
	}
	for p, page := range plan.Pages {
		if len(page.ContentRuns) == 0 && len(page.ContentImages) == 0 {
			t.Errorf("page %d is empty", p)
		}
	}
}

// TestPaginateReportsAnItemThatFitsNoWindow is FR44's located diagnostic —
// the ONLY residual case once the window can slide, because an item shorter
// than the window always fits one.
//
// ONE OVERFLOW RULE, TWO SUBJECTS: a line taller than the window and an image
// whose declared box is taller than the window get the same answer, so the
// implementation is never left to split, loop or panic.
func TestPaginateReportsAnItemThatFitsNoWindow(t *testing.T) {
	g := testGeometry()

	for _, row := range []struct {
		name     string
		item     ColumnItem
		wantKind string
	}{
		{
			name:     "a line taller than the content window",
			item:     ColumnItem{ElementID: "e7", Top: testContentTop, Bottom: testContentTop + 100001, Runs: []TextRunRef{0}},
			wantKind: "line",
		},
		{
			name:     "an image whose DECLARED BOX is taller than the content window",
			item:     ColumnItem{ElementID: "e8", Top: testContentTop, Bottom: testContentTop + 250000, Images: []ImageRef{0}},
			wantKind: "image",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			// Presence precondition: the item really is taller than the
			// window, or this asserts nothing.
			if h := row.item.Bottom - row.item.Top; h <= testContentHeight {
				t.Fatalf("presence precondition: the item is %d tall, which FITS the %d window — the overflow case is not exercised", h, testContentHeight)
			}

			_, err := Paginate(g, []ColumnItem{row.item})
			if err == nil {
				t.Fatal("Paginate accepted an item that fits in no window. It must be a located diagnostic: never a straddle, never a silent clip.")
			}
			var overflow *OverflowError
			if !errors.As(err, &overflow) {
				t.Fatalf("Paginate returned %T; FR44's diagnostic must be a *OverflowError so a caller can tell it from an I/O failure", err)
			}
			if overflow.ElementID != row.item.ElementID {
				t.Errorf("the diagnostic names element %q; the offending element is %q — an unlocated overflow message is what D-1.8.1 exists to prevent", overflow.ElementID, row.item.ElementID)
			}
			if overflow.Kind != row.wantKind {
				t.Errorf("the diagnostic calls the item a %q; it is a %q", overflow.Kind, row.wantKind)
			}
		})
	}
}

// TestPaginateEmitsContentInAuthoredOrder is what makes a one-page document
// byte-identical to its pre-2.6 self.
//
// The sweep visits items in COLUMN order, which is not necessarily authored
// order — an element declared second may sit above one declared first.
// Emission order reaches output bytes (content-stream operator order), so it
// must be the authored order regardless of what the sweep did.
func TestPaginateEmitsContentInAuthoredOrder(t *testing.T) {
	g := testGeometry()

	// Authored second-then-first: e2 is declared FIRST but sits BELOW e1.
	items := []ColumnItem{
		{ElementID: "e2", Top: testContentTop + 50000, Bottom: testContentTop + 60000, Runs: []TextRunRef{0}},
		{ElementID: "e1", Top: testContentTop, Bottom: testContentTop + 10000, Runs: []TextRunRef{1}},
	}

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != 1 {
		t.Fatalf("presence precondition: both items fit one window; got %d pages", len(plan.Pages))
	}
	got := plan.Pages[0].ContentRuns
	want := []TextRunRef{0, 1} // authored order, NOT column order (which is 1, 0)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("content emitted in order %v; the authored order is %v.\n"+
			"The sweep visits in COLUMN order (1, 0 here); emission must not inherit it, or every "+
			"pre-2.6 golden moves.", got, want)
	}
}

// TestPaginateRejectsAnItemThatIsNeitherOneLineNorOneImage is D-2.6.5's
// guardrail on the Kind classifier.
//
// OverflowError.Kind is derived from which slice is populated and — since
// Story 7.10's rect arm — from Group.AuthorDeclared, which tells a table
// row's cell chrome ("table") from an element BOX ("box"). Either way the
// derivation is only sound if an item never carries MORE THAN ONE slice — a
// claim about the CALLER, and a claim about a caller is the kind that stops
// being true quietly. This asserts the check rather than the claim.
func TestPaginateRejectsAnItemThatIsNeitherOneLineNorOneImage(t *testing.T) {
	g := testGeometry()

	for _, row := range []struct {
		name      string
		item      ColumnItem
		wantEmpty bool
	}{
		{
			name: "an item carrying BOTH runs and an image",
			item: ColumnItem{
				ElementID: "e9", Top: testContentTop, Bottom: testContentTop + 1000,
				Runs: []TextRunRef{0}, Images: []ImageRef{0},
			},
		},
		{
			name:      "an item carrying NEITHER",
			item:      ColumnItem{ElementID: "e10", Top: testContentTop, Bottom: testContentTop + 1000},
			wantEmpty: true,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, err := Paginate(g, []ColumnItem{row.item})
			if err == nil {
				t.Fatal("Paginate accepted a malformed column item; the Kind classifier would mislabel its diagnostic")
			}
			var mixed *MixedItemError
			if !errors.As(err, &mixed) {
				t.Fatalf("Paginate returned %T; it must be a *MixedItemError so a caller cannot confuse a build defect with a document that does not fit", err)
			}
			if mixed.ElementID != row.item.ElementID {
				t.Errorf("the diagnostic names element %q; the offending element is %q", mixed.ElementID, row.item.ElementID)
			}
			if mixed.Empty != row.wantEmpty {
				t.Errorf("the diagnostic reports Empty=%v; want %v — \"both\" and \"neither\" have different causes and must not share a message", mixed.Empty, row.wantEmpty)
			}
		})
	}
}

// TestPaginateShiftIsZeroOnPageZeroEvenForContentPositionedLow is the
// finisher's check of Finding 14 (DISMISSED, not fixed — see the story's
// finding resolutions).
//
// The review claimed PageAssignment.Shift's docblock overstates the
// guarantee: that window 0 begins at "the top of the FIRST ITEM", so a
// single element positioned LOW in the content band (its declared Top well
// below the band's own top) would get a NON-ZERO page-0 shift, pulling it
// up to the band's top. Measured against this file's own ruled model (rule
// 1 in the package doc, unchanged): window 0 begins at the CONTENT BAND'S
// OWN TOP, unconditionally — only window N+1 begins at the first item that
// did not fit in window N. This test is the discriminating case: an item
// declared 80,000mp below the content band's top, comfortably inside one
// window. If the finding's claim were true, Shift would be 80,000 and the
// item would render pulled to the band's top; the ruled model says Shift is
// 0 and the item renders exactly where it was declared.
func TestPaginateShiftIsZeroOnPageZeroEvenForContentPositionedLow(t *testing.T) {
	g := testGeometry()

	const positionedTop = testContentTop + 80000 // well below the band's own top, still inside window 0
	item := ColumnItem{
		ElementID: "e1",
		Top:       positionedTop,
		Bottom:    positionedTop + 1000,
		Runs:      []TextRunRef{0},
	}

	plan, err := Paginate(g, []ColumnItem{item})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != 1 {
		t.Fatalf("a single low-positioned item that fits one window should produce ONE page, got %d", len(plan.Pages))
	}
	if plan.Pages[0].Shift != 0 {
		t.Errorf("page 0's shift is %d for an item positioned at %d (80,000mp below the content band's "+
			"top, %d); the ruled model says window 0 begins at the BAND'S top unconditionally, so the "+
			"declared value is 0 — a non-zero shift here would mean the item was pulled to the band's "+
			"top, which is the behaviour Finding 14 (incorrectly) predicted",
			plan.Pages[0].Shift, positionedTop, testContentTop)
	}
}
