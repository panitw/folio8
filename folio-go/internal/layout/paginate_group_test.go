package layout

// Story 4.3: DECISION-1's grouping mechanism, asserted directly against
// layout.ColumnItem/ItemGroup — the same declarative-table discipline
// paginate_test.go already uses (D-000.45): every expectation is a
// HAND-DERIVED INTEGER, and the arithmetic is written out beside it.
//
// D2 (the story's own measurement) found that row-wholeness ALREADY holds
// at 903bf8f, by accident: a data row's chrome rect happens to span the
// row's full extent, and PHASE B's rects-before-text append order happens
// to make the chrome win ties. Every test below is built to be BLIND to
// that accident — several deliberately use the ORDER D2 measured as
// SPLITTING the row (lines before the chrome rect) — so a green result
// here is evidence of the Group mechanism, not of D2's accident.

import (
	"errors"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// rowGroupItems builds one "row"'s ColumnItems: one Rect chrome item
// spanning [top, top+h) and n line items packed inside it with no gaps,
// all sharing the SAME ItemGroupKey — the same shape table_render.go
// produces for one data row, but built here without any font/text
// machinery so every number is hand-derivable.
func rowGroupItems(key ItemGroupKey, top, h geom.Length, n int, base int) (rect ColumnItem, lines []ColumnItem) {
	group := ItemGroup{Present: true, Key: key}
	rect = ColumnItem{
		ElementID: "table",
		Top:       top, Bottom: top + h,
		Rects: []RectRef{RectRef(base)},
		Group: group,
	}
	adv := h / geom.Length(int64(n))
	for i := range n {
		lt := top + geom.Length(int64(i))*adv
		lines = append(lines, ColumnItem{
			ElementID: "table",
			Top:       lt, Bottom: lt + adv,
			Runs:  []TextRunRef{TextRunRef(base*10 + i)},
			Group: group,
		})
	}
	return
}

// TestPaginateRowGroupMovesWholeRegardlessOfAppendOrder is AC1's core
// mechanism proof. Two rows of 60,000mp each (3 lines of 20,000mp
// apiece), window 100,000: row 0 fits window 0 entirely; row 1's chrome
// spans 78,000..138,000, which does NOT fit (window 0 ends at 118,000),
// so row 1 — chrome AND every one of its lines — must start window 1.
//
// Run BOTH append orders (lines-before-rect, matching D2's PHASE-A order
// that SPLIT a row at 903bf8f, and rect-before-lines, PHASE B's order):
// the Group field must produce the SAME partition either way, because
// the mechanism does not depend on which content kind the caller happens
// to append first.
func TestPaginateRowGroupMovesWholeRegardlessOfAppendOrder(t *testing.T) {
	g := testGeometry()
	key0 := ItemGroupKey{ElementID: "e1", Index: 0}
	key1 := ItemGroupKey{ElementID: "e1", Index: 1}
	rect0, lines0 := rowGroupItems(key0, testContentTop, 60000, 3, 0)
	rect1, lines1 := rowGroupItems(key1, testContentTop+60000, 60000, 3, 1)

	for _, order := range []struct {
		name  string
		items []ColumnItem
	}{
		{"lines before rects (D2's splitting order)", concatItems(lines0, []ColumnItem{rect0}, lines1, []ColumnItem{rect1})},
		{"rects before lines (D2's accidental order)", concatItems([]ColumnItem{rect0}, lines0, []ColumnItem{rect1}, lines1)},
	} {
		t.Run(order.name, func(t *testing.T) {
			plan, err := Paginate(g, order.items)
			if err != nil {
				t.Fatalf("Paginate: %v", err)
			}
			if len(plan.Pages) != 2 {
				t.Fatalf("presence precondition: this fixture must straddle exactly one boundary; got %d pages, want 2", len(plan.Pages))
			}
			// Row 0's chrome (a Rect item) and all 3 of its lines must be
			// on page 0; row 1's on page 1. Anchored on which RunRef/
			// RectRef values land on which page (D-000.68: identity, not
			// a count).
			wantPage0Runs := map[TextRunRef]bool{0: true, 1: true, 2: true}
			wantPage1Runs := map[TextRunRef]bool{10: true, 11: true, 12: true}
			if len(plan.Pages[0].ContentRects) != 1 || plan.Pages[0].ContentRects[0] != 0 {
				t.Errorf("page 0's rects = %v, want [0] (row 0's chrome)", plan.Pages[0].ContentRects)
			}
			if len(plan.Pages[1].ContentRects) != 1 || plan.Pages[1].ContentRects[0] != 1 {
				t.Errorf("page 1's rects = %v, want [1] (row 1's chrome)", plan.Pages[1].ContentRects)
			}
			for _, ref := range plan.Pages[0].ContentRuns {
				if !wantPage0Runs[ref] {
					t.Errorf("page 0 carries run %d, which belongs to row 1 — row 0 and row 1 are mixed on one page", ref)
				}
			}
			for _, ref := range plan.Pages[1].ContentRuns {
				if !wantPage1Runs[ref] {
					t.Errorf("page 1 carries run %d, which belongs to row 0 — row 0 and row 1 are mixed on one page", ref)
				}
			}
			if len(plan.Pages[0].ContentRuns) != 3 || len(plan.Pages[1].ContentRuns) != 3 {
				t.Errorf("got %d/%d runs on page 0/1, want 3/3 — every one of a row's lines must land with its chrome", len(plan.Pages[0].ContentRuns), len(plan.Pages[1].ContentRuns))
			}
		})
	}
}

// concatItems is a small helper so the table above reads as a sequence
// of named groups rather than a hand-flattened slice.
func concatItems(groups ...[]ColumnItem) []ColumnItem {
	var out []ColumnItem
	for _, gr := range groups {
		out = append(out, gr...)
	}
	return out
}

// TestPaginateGroupPartitionPinnedByValue is AC2's D-000.33 discipline
// applied to groups: the partition is pinned BY VALUE, never by a
// conservation law (satisfied by the degenerate paginator that puts
// everything on page 0). Six rows of 30,000mp each (3 lines of
// 10,000mp), window 100,000 — the exact geometry D2's own probe used,
// reproduced here as a permanent, asserted test rather than a scratch
// file. Built in LINES-BEFORE-RECT order (D2's splitting order).
//
//	row 0:  18,000.. 48,000
//	row 1:  48,000.. 78,000
//	row 2:  78,000..108,000  (window 0 ends 118,000: fits)
//	row 3: 108,000..138,000  (does NOT fit -> starts window 1 at 108,000)
//	row 4: 138,000..168,000  (window 1 = [108,000..208,000]: fits)
//	row 5: 168,000..198,000  (fits, window 1 ends 208,000)
func TestPaginateGroupPartitionPinnedByValue(t *testing.T) {
	g := testGeometry()
	var items []ColumnItem
	for r := range 6 {
		key := ItemGroupKey{ElementID: "e1", Index: r}
		rect, lines := rowGroupItems(key, testContentTop+geom.Length(int64(r))*30000, 30000, 3, r)
		items = append(items, lines...)
		items = append(items, rect)
	}

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != 2 {
		t.Fatalf("Paginate produced %d pages; the declared partition has 2", len(plan.Pages))
	}

	wantRowPage := []int{0, 0, 0, 1, 1, 1}
	gotRowPage := make([]int, 6)
	for i := range gotRowPage {
		gotRowPage[i] = -1
	}
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRects {
			gotRowPage[int(ref)] = p
		}
	}
	for r, want := range wantRowPage {
		if gotRowPage[r] != want {
			t.Errorf("row %d (chrome RectRef %d) landed on page %d; the declared partition puts it on page %d", r, r, gotRowPage[r], want)
		}
	}
	// Every row's 3 lines must land on the SAME page as that row's own
	// chrome (D-000.68: set equality by identity, not a count).
	for p, pg := range plan.Pages {
		runRows := map[int]bool{}
		for _, ref := range pg.ContentRuns {
			runRows[int(ref)/10] = true
		}
		rectRows := map[int]bool{}
		for _, ref := range pg.ContentRects {
			rectRows[int(ref)] = true
		}
		if len(runRows) != len(rectRows) {
			t.Fatalf("page %d: %d distinct rows among line items, %d among chrome items — sets must be equal", p, len(runRows), len(rectRows))
		}
		for row := range rectRows {
			if !runRows[row] {
				t.Errorf("page %d: row %d's chrome is present but none of its lines are", p, row)
			}
		}
	}
}

// TestPaginateGroupEmissionStaysInAuthoredOrder is AC2's ORDER clause,
// witnessed independently of TestPaginateEmitsContentInAuthoredOrder
// (which uses no grouping at all): a group changes WHICH PAGE an item
// lands on, and it must change NOTHING about emission order. Row "late"
// is declared FIRST (authored index 0) but sits BELOW row "early"
// (declared second, authored index 1) — the sweep visits "early" first
// (smaller Top). Both rows fit on one page, so this isolates emission
// order from the page-assignment question entirely.
func TestPaginateGroupEmissionStaysInAuthoredOrder(t *testing.T) {
	g := testGeometry()
	lateKey := ItemGroupKey{ElementID: "e1", Index: 1}
	earlyKey := ItemGroupKey{ElementID: "e1", Index: 0}
	lateRect, lateLines := rowGroupItems(lateKey, testContentTop+20000, 10000, 1, 1)
	earlyRect, earlyLines := rowGroupItems(earlyKey, testContentTop, 10000, 1, 0)

	// Authored order: "late" (spatially below) declared FIRST.
	items := concatItems([]ColumnItem{lateRect}, lateLines, []ColumnItem{earlyRect}, earlyLines)

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != 1 {
		t.Fatalf("presence precondition: both rows fit one window; got %d pages", len(plan.Pages))
	}
	wantRects := []RectRef{1, 0} // authored order, NOT column/sweep order (which visits "early" first)
	got := plan.Pages[0].ContentRects
	if len(got) != 2 || got[0] != wantRects[0] || got[1] != wantRects[1] {
		t.Errorf("chrome emitted in order %v; the authored order is %v — grouping must not leak sweep (column) order into emission", got, wantRects)
	}
	wantRuns := []TextRunRef{10, 0} // late row's single line (base=1 -> ref 10), then early row's (base=0 -> ref 0)
	gotRuns := plan.Pages[0].ContentRuns
	if len(gotRuns) != 2 || gotRuns[0] != wantRuns[0] || gotRuns[1] != wantRuns[1] {
		t.Errorf("lines emitted in order %v; the authored order is %v", gotRuns, wantRuns)
	}
}

// TestPaginateGroupToleratesAnInterveningItemAtTheSameTop is Finding 1
// (this story's finisher review), reproduced as a regression at the
// `layout` level: R7's ORIGINAL premise — "a group's members are
// contiguous in column order by construction" — is FALSE for real
// templates. An ordinary, legal element (a caption, a second table, a
// text note) that merely SHARES a group member's own Top sorts BETWEEN
// two of that group's members under the stable sort's tie-breaking rule,
// which preserves INPUT order for equal Tops — exactly what this fixture
// constructs by declaring the group's two members FIRST and LAST with an
// unrelated, ungrouped item in between, all three sharing one Top. This
// must paginate successfully, not return the "group ... is not
// contiguous" internal error the pre-fix code raised on input the schema
// accepts (a table beside any element at the same y, measured against a
// 903bf8f worktree to still render there — the fix removes the
// dependence on contiguity instead of asserting a premise that does not
// hold).
func TestPaginateGroupToleratesAnInterveningItemAtTheSameTop(t *testing.T) {
	g := testGeometry()
	key := ItemGroupKey{ElementID: "e1", IsHeader: true}
	group := ItemGroup{Present: true, Key: key}
	items := []ColumnItem{
		{ElementID: "e1", Top: testContentTop, Bottom: testContentTop + 10000, Rects: []RectRef{0}, Group: group},
		{ElementID: "e2", Top: testContentTop, Bottom: testContentTop + 8000, Runs: []TextRunRef{99}}, // interloper: unrelated, ungrouped, same Top
		{ElementID: "e1", Top: testContentTop, Bottom: testContentTop + 10000, Runs: []TextRunRef{0, 1}, Group: group},
	}
	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate returned an error for a legal document where an unrelated item merely shares a group member's own Top: %v — this is Finding 1's regression", err)
	}
	if len(plan.Pages) != 1 {
		t.Fatalf("got %d pages, want 1 (every item fits window 0)", len(plan.Pages))
	}
	if len(plan.Pages[0].ContentRects) != 1 || plan.Pages[0].ContentRects[0] != 0 {
		t.Errorf("page 0's rects = %v, want [0] (the group's own chrome)", plan.Pages[0].ContentRects)
	}
	wantRuns := map[TextRunRef]bool{0: true, 1: true, 99: true}
	if len(plan.Pages[0].ContentRuns) != len(wantRuns) {
		t.Fatalf("page 0's runs = %v, want all of %v", plan.Pages[0].ContentRuns, wantRuns)
	}
	for _, ref := range plan.Pages[0].ContentRuns {
		if !wantRuns[ref] {
			t.Errorf("unexpected run %d on page 0", ref)
		}
	}
}

// TestPaginateGroupSurvivesAnInterveningPageAdvance is the mechanism-level
// discriminator behind Finding 1's fix: it is not enough to stop erroring
// on a non-contiguous group — the group must still land on ONE page even
// when an UNRELATED, ungrouped item interposed between two of its members
// forces the sweep's own "current page" to advance before the group's
// second member is visited.
//
// The group's chrome (Top 18,000) and its second member, "line" (Top
// 100,000), have a UNION extent of 18,000..104,000 (height 86,000), which
// fits window 0 = [18,000..118,000) — so the group is decided, at the
// FIRST member the sweep visits (chrome), to land on page 0. The
// interloper's OWN extent (68,000..168,000) does not fit window 0 either
// (its bottom, 168,000, exceeds 118,000), so processing it alone advances
// the sweep to page 1 with a new window starting at 68,000 — BEFORE "line"
// is ever visited.
//
// A fix that merely deletes the contiguity error but still re-tests every
// group member against whatever window is CURRENT when it is visited gets
// this wrong: "line"'s own union extent (18,000..104,000) fits the
// ADVANCED window (68,000..168,000) too, so it would be assigned the
// CURRENT page (1), splitting the group from its chrome (page 0) — which
// the R6 belt-and-suspenders check below would then have to reject as
// "unreachable". The correct fix resolves a group's page ONCE, at its
// first-visited member, and every later member copies that page directly
// — so "line" must land on page 0 with its chrome, and the interloper
// alone occupies page 1.
func TestPaginateGroupSurvivesAnInterveningPageAdvance(t *testing.T) {
	g := testGeometry()
	key := ItemGroupKey{ElementID: "e5", Index: 0}
	group := ItemGroup{Present: true, Key: key}
	chrome := ColumnItem{ElementID: "e5", Top: testContentTop, Bottom: testContentTop + 10000, Rects: []RectRef{0}, Group: group}
	line := ColumnItem{ElementID: "e5", Top: testContentTop + 82000, Bottom: testContentTop + 86000, Runs: []TextRunRef{0}, Group: group}
	interloper := ColumnItem{ElementID: "e6", Top: testContentTop + 50000, Bottom: testContentTop + 150000, Runs: []TextRunRef{99}}
	items := []ColumnItem{chrome, interloper, line}

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != 2 {
		t.Fatalf("got %d pages, want 2 (the interloper alone forces one advance)", len(plan.Pages))
	}
	if len(plan.Pages[0].ContentRects) != 1 || plan.Pages[0].ContentRects[0] != 0 {
		t.Errorf("page 0's rects = %v, want [0] (the group's chrome)", plan.Pages[0].ContentRects)
	}
	if len(plan.Pages[0].ContentRuns) != 1 || plan.Pages[0].ContentRuns[0] != 0 {
		t.Errorf("page 0's runs = %v, want [0] (the group's own second member — it must stay with its chrome despite the intervening item advancing the sweep's current page)", plan.Pages[0].ContentRuns)
	}
	if len(plan.Pages[1].ContentRuns) != 1 || plan.Pages[1].ContentRuns[0] != 99 {
		t.Errorf("page 1's runs = %v, want [99] (the interloper, alone, on its own page)", plan.Pages[1].ContentRuns)
	}
}

// TestPaginateGroupTallerThanWindowIsClippedRatherThanFatal is Story
// 4.3's AC4 residual case, INVERTED ON PURPOSE by Story 4.6.
//
// Story 4.3 placed this test asserting a located *OverflowError with Kind
// "table" for a group taller than the window, and said in its own comment
// that "Story 4.6 owns clipping this case to a fresh page". This is that
// story. AD-14 rules the case never fatal — "Rows and author-declared
// keep-together groups too tall for one content window (FR25, FR51), and
// clipped content (FR44), are Warnings returned alongside PDF bytes, never
// silent and never fatal" — so the group is now placed alone on a fresh
// page and cut off at that page's content bottom, and Paginate returns no
// error at all.
//
// THE GROUP HERE IS ENGINE-CREATED (ItemGroup.AuthorDeclared is false, the
// zero value, which is what a table row carries), and that is what keeps
// this test on Story 4.6's answer after Story 7.10 narrowed the exception.
// The narrowing is scoped by WHO CREATED THE GROUPING: an AUTHOR-declared
// group holding a template element taller than any window is refused
// instead (D-7.10.1, D-7.10.2). Were the narrowing unscoped it would fire
// here — this group's chrome rect spans the whole 150,000 extent, so "some
// member is individually over-tall" is true of it, exactly as it is true
// of every over-tall table row.
//
// The inversion is planned, not a regression. What did NOT invert is the
// quantity: ItemHeight is still the GROUP's UNION height, not any one
// member's own, which is the part of Story 4.3's assertion that was about
// grouping rather than about erroring.
func TestPaginateGroupTallerThanWindowIsClippedRatherThanFatal(t *testing.T) {
	g := testGeometry()
	key := ItemGroupKey{ElementID: "e9", Index: 0}
	group := ItemGroup{Present: true, Key: key}
	// Group union height: 150,000, which exceeds the 100,000 window.
	// The chrome rect spans the whole group; the line member ends at
	// +30,000, well inside the window, so the two land on opposite
	// sides of the clip and the kept/dropped split is a real one.
	items := []ColumnItem{
		{ElementID: "e9", Top: testContentTop, Bottom: testContentTop + 150000, Rects: []RectRef{0}, Group: group},
		{ElementID: "e9", Top: testContentTop, Bottom: testContentTop + 30000, Runs: []TextRunRef{0}, Group: group},
	}
	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate returned %T: %v — AD-14 makes an over-tall GROUP a clip with a Warning, never an error. An UNGROUPED over-tall item still errors; see TestPaginateOverflowingItemReturnsLocatedError", err, err)
	}

	if len(plan.Clipped) != 1 {
		t.Fatalf("plan.Clipped = %+v; want exactly one record for the one over-tall group", plan.Clipped)
	}
	c := plan.Clipped[0]
	if c.Key != key {
		t.Errorf("the clip record names group %+v; want %+v — the record carries the group's identity VERBATIM so a caller reads the row index off it rather than re-deriving one", c.Key, key)
	}
	if c.ItemHeight != 150000 {
		t.Errorf("clip.ItemHeight = %d, want the GROUP's union height, 150,000 (not any one member's own height)", c.ItemHeight)
	}
	if c.ContentHeight != ContentHeight(g) {
		t.Errorf("clip.ContentHeight = %d, want the content window's own height %d", c.ContentHeight, ContentHeight(g))
	}

	// The cut, asserted as the two halves it actually is: the chrome
	// rect is TRUNCATED (present, bounded at the content bottom) and
	// the line that fits is KEPT. Nothing straddles.
	pa := plan.Pages[c.Page]
	wantBound := testContentTop + ContentHeight(g)
	if len(pa.ClippedRects) != 1 || pa.ClippedRects[0] != (RectClip{Ref: 0, Bottom: wantBound}) {
		t.Errorf("page %d's ClippedRects = %+v; want [{Ref:0 Bottom:%d}] — the row's chrome is truncated at the content bottom, not drawn to its untruncated height", c.Page, pa.ClippedRects, wantBound)
	}
	if len(pa.ContentRuns) != 1 || pa.ContentRuns[0] != 0 {
		t.Errorf("page %d's ContentRuns = %v; want [0] — the group's one line ends inside the window and is kept whole", c.Page, pa.ContentRuns)
	}
}

// TestPaginateUngroupedItemsAreUnaffectedByGrouping is R2: an item with
// the zero-value ItemGroup (Present == false) — every item built before
// this story, and every non-table item this story's own callers still
// build — must place EXACTLY as it did before the Group field existed.
// Reruns TestPaginatePinsTheLineToPagePartitionByValue's own fixture
// with the zero Group value made explicit, so a change to the zero
// value's meaning would redden here first.
//
// Finding 5 (this story's finisher review): the partition is pinned BY
// VALUE, the same D-000.33 discipline TestPaginateGroupPartitionPinnedByValue
// uses — a bare `len(plan.Pages) != 3` conservation check cannot see two
// lines swapping pages while the page COUNT stays 3, which is exactly the
// failure this guard exists to catch.
func TestPaginateUngroupedItemsAreUnaffectedByGrouping(t *testing.T) {
	g := testGeometry()
	items := stackedLines(21, 10000)
	for i := range items {
		items[i].Group = ItemGroup{} // explicit, though already the default
	}

	// Identical hand-derived partition to
	// TestPaginatePinsTheLineToPagePartitionByValue (same fixture: 21 lines
	// of 10,000mp from the content top, window 100,000):
	//
	//	window 0 = [18,000 .. 118,000]  <- lines 0..9
	//	window 1 = [118,000 .. 218,000] <- lines 10..19
	//	window 2 starts at line 20's own top
	wantPageOfLine := []int{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // lines 0-9
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // lines 10-19
		2, // line 20
	}
	wantShift := []geom.Length{0, 100000, 200000}

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(plan.Pages) != len(wantShift) {
		t.Fatalf("Paginate produced %d pages; the declared partition has %d", len(plan.Pages), len(wantShift))
	}

	gotPageOfLine := make([]int, len(items))
	for i := range gotPageOfLine {
		gotPageOfLine[i] = -1
	}
	for p, page := range plan.Pages {
		if page.Shift != wantShift[p] {
			t.Errorf("page %d's window shift is %d; the declared value is %d", p, page.Shift, wantShift[p])
		}
		for _, ref := range page.ContentRuns {
			gotPageOfLine[int(ref)] = p
		}
	}
	for i, want := range wantPageOfLine {
		if gotPageOfLine[i] != want {
			t.Errorf("line %d landed on page %d; the declared (ungrouped, unchanged-from-before-this-story) partition puts it on page %d", i, gotPageOfLine[i], want)
		}
	}
}

// TestPaginateHeaderGroupMovesWholeEvenWhenChromeAndLabelExtentsDiffer is
// Finding 2's remedy (this story's finisher review, Blocker): the ONLY
// folio8-level witness for AC5 used a header whose chrome and column
// labels are ALWAYS EXACTLY tied on extent (Story 4.1's single-line-header
// construction, §9.4's own measurement), so deleting the header's
// grouping there left the WHOLE SUITE GREEN — any implementation, grouped
// or not, places two items with equal extent on the same page. That is a
// fact about the fixture, not about the mechanism.
//
// This test builds an `ItemGroupKey{IsHeader: true}` group whose two
// members have DIFFERENT extents on purpose — chrome and label alike
// share Story 4.1's own header shape (one ElementID, IsHeader true,
// Index meaningless), but here they DISAGREE about whether they fit
// window 0, which is precisely the situation Story 4.1's construction
// never produces and therefore never witnesses. It is the `IsHeader`
// key branch itself under test, independent of whether the SHIPPED
// header can ever reach this state (it cannot — see finding 9.4/Finding
// 2's own record).
//
//   - filler: 18,000..28,000, ungrouped — establishes that page 0
//     already carries something, so the header's own placement decision
//     is a real advance-or-not choice, not the "no page is ever empty"
//     free ride the very first item of a document gets.
//   - label: 110,000..115,000 (height 5,000) — ALONE, fits window 0
//     (window 0 ends 118,000).
//   - chrome: 110,000..140,000 (height 30,000) — ALONE, does NOT fit
//     window 0.
//
// GROUPED (this test's positive case): the union extent (110,000..140,000)
// decides for BOTH, so both move to page 1 together.
//
// UNGROUPED (the same two items with Group cleared — the fixture's own
// non-vacuity check, proving the two extents really do disagree): label
// fits window 0 on its own and stays on page 0; chrome does not and
// starts page 1 — a genuine split, which is what proves grouping is
// LOAD-BEARING for this exact key branch rather than free by
// construction.
func TestPaginateHeaderGroupMovesWholeEvenWhenChromeAndLabelExtentsDiffer(t *testing.T) {
	g := testGeometry()
	key := ItemGroupKey{ElementID: "e1", IsHeader: true}
	group := ItemGroup{Present: true, Key: key}
	filler := ColumnItem{ElementID: "filler", Top: testContentTop, Bottom: testContentTop + 10000, Runs: []TextRunRef{7}}
	chrome := ColumnItem{ElementID: "e1", Top: testContentTop + 92000, Bottom: testContentTop + 122000, Rects: []RectRef{0}}
	label := ColumnItem{ElementID: "e1", Top: testContentTop + 92000, Bottom: testContentTop + 97000, Runs: []TextRunRef{0}}

	t.Run("grouped: chrome and label move together", func(t *testing.T) {
		grouped := chrome
		grouped.Group = group
		groupedLabel := label
		groupedLabel.Group = group
		items := []ColumnItem{filler, groupedLabel, grouped}

		plan, err := Paginate(g, items)
		if err != nil {
			t.Fatalf("Paginate: %v", err)
		}
		if len(plan.Pages) != 2 {
			t.Fatalf("got %d pages, want 2 (filler alone on page 0, the header group forced to page 1)", len(plan.Pages))
		}
		if len(plan.Pages[0].ContentRuns) != 1 || plan.Pages[0].ContentRuns[0] != 7 {
			t.Fatalf("page 0's runs = %v, want [7] (only the filler)", plan.Pages[0].ContentRuns)
		}
		if len(plan.Pages[1].ContentRects) != 1 || plan.Pages[1].ContentRects[0] != 0 {
			t.Errorf("page 1's rects = %v, want [0] (the header chrome)", plan.Pages[1].ContentRects)
		}
		if len(plan.Pages[1].ContentRuns) != 1 || plan.Pages[1].ContentRuns[0] != 0 {
			t.Errorf("page 1's runs = %v, want [0] (the header label — it must move WITH the chrome even though its own, smaller extent would fit page 0 alone)", plan.Pages[1].ContentRuns)
		}
	})

	t.Run("ungrouped: chrome and label split (non-vacuity check)", func(t *testing.T) {
		// label visited BEFORE chrome (both tie on Top; the stable sort
		// preserves this declared order) so its own, smaller extent is
		// tested against window 0 while window 0 is still the CURRENT
		// window — exactly the ordering that makes this fixture capable
		// of splitting them when ungrouped, which is what this sub-test
		// exists to confirm.
		items := []ColumnItem{filler, label, chrome} // Group left zero-value on both
		plan, err := Paginate(g, items)
		if err != nil {
			t.Fatalf("Paginate: %v", err)
		}
		if len(plan.Pages) != 2 {
			t.Fatalf("got %d pages, want 2", len(plan.Pages))
		}
		if len(plan.Pages[0].ContentRuns) != 2 {
			t.Fatalf("page 0's runs = %v, want the filler AND the label (2 runs) — the label's own extent fits window 0 alone", plan.Pages[0].ContentRuns)
		}
		if len(plan.Pages[1].ContentRects) != 1 || plan.Pages[1].ContentRects[0] != 0 {
			t.Fatalf("page 1's rects = %v, want [0] (the chrome, which does NOT fit window 0 alone) — if this fixture does not split ungrouped, it cannot witness the grouped case above either", plan.Pages[1].ContentRects)
		}
	})
}

// TestPaginateOverTallHeaderGroupClipsAndIsRecordedAsTheHeader closes the
// third arm of D-4.6.3 (this story's reviewer, Finding 9).
//
// D-4.6.3's headline is "ONE CODE FOR ALL THREE GROUP ROLES", and that
// merging — rather than splitting — is the ruling's novelty. At review the
// data-row arm had four end-to-end tests and the footer arm had
// TestFooterAloneTooTallForTheWindowIsClippedRatherThanFatal, but the
// HEADER arm had nothing that ran through Paginate at all: its only
// witness called the message builder directly with a synthetic record,
// which cannot see a pagination-level regression.
//
// It also pins the one thing the header arm does DIFFERENTLY, and it is
// the reason D-4.6.4's repeat block is guarded on !IsHeader: a table's
// header is never repeated above ITSELF (DECISION-1), so the clipped
// header's own page carries no repeat — while the next page, which now has
// a header far too tall to reserve, falls to DECISION-2 arm (c) and is
// recorded there.
func TestPaginateOverTallHeaderGroupClipsAndIsRecordedAsTheHeader(t *testing.T) {
	g := testGeometry()
	hdrKey := ItemGroupKey{ElementID: "e9", IsHeader: true}
	rowKey := ItemGroupKey{ElementID: "e9", Index: 0}
	hdr := ItemGroup{Present: true, Key: hdrKey}

	// The header group's union height is 150,000 against a 100,000
	// window, so it fits no window at all. Its line member ends well
	// inside the window, so the kept/dropped split is a real one.
	items := []ColumnItem{
		{ElementID: "e9", Top: testContentTop, Bottom: testContentTop + 150000, Rects: []RectRef{0}, Group: hdr},
		{ElementID: "e9", Top: testContentTop, Bottom: testContentTop + 30000, Runs: []TextRunRef{0}, Group: hdr},
		{ElementID: "e9", Top: testContentTop + 150000, Bottom: testContentTop + 160000, Rects: []RectRef{1},
			Group: ItemGroup{Present: true, Key: rowKey}},
	}

	plan, err := Paginate(g, items)
	if err != nil {
		t.Fatalf("Paginate returned %T: %v — an over-tall HEADER group is a table group like any other: AD-14 makes it a clip, never an error", err, err)
	}

	if len(plan.Clipped) != 1 {
		t.Fatalf("plan.Clipped = %+v; want exactly one record — the header group is the only over-tall group here", plan.Clipped)
	}
	c := plan.Clipped[0]

	// THE ROLE, which is the whole point: the record identifies the
	// group as the HEADER, so the caller's message builder renders "the
	// header row" rather than a row index.
	if !c.Key.IsHeader {
		t.Errorf("the clip record's Key is %+v; want IsHeader true — D-4.6.3 gives all three group roles ONE code and carries the ROLE in the record, so a caller can name it without re-deriving it", c.Key)
	}
	if c.Key != hdrKey {
		t.Errorf("the clip record names group %+v; want %+v", c.Key, hdrKey)
	}
	if c.ItemHeight != 150000 {
		t.Errorf("clip.ItemHeight = %d, want the GROUP's union height, 150,000", c.ItemHeight)
	}

	// A HEADER IS NEVER REPEATED ABOVE ITSELF (DECISION-1), so the clip
	// on this page reserves nothing and cuts at the full window bottom.
	if got := len(plan.Pages[c.Page].HeaderRepeats); got != 0 {
		t.Errorf("the clipped header's own page carries %d repeated header(s); want 0 — a table's header is not repeated above itself", got)
	}
	wantBound := testContentTop + ContentHeight(g)
	if len(plan.Pages[c.Page].ClippedRects) != 1 || plan.Pages[c.Page].ClippedRects[0] != (RectClip{Ref: 0, Bottom: wantBound}) {
		t.Errorf("page %d's ClippedRects = %+v; want [{Ref:0 Bottom:%d}] — with no repeat to reserve for, the cut is the window's own bottom",
			c.Page, plan.Pages[c.Page].ClippedRects, wantBound)
	}

	// AND THE NEXT PAGE'S REPEAT IS SUPPRESSED AND RECORDED. The header
	// is 150,000 tall; reserving it on the data row's page leaves nothing
	// at all. That is DECISION-2 arm (c) on its own terms — never silent.
	if len(plan.Suppressed) != 1 {
		t.Fatalf("plan.Suppressed = %+v; want exactly one record — the data row's page cannot reserve a 150,000mp header inside a 100,000mp window, and AD-14 says nothing is silent", plan.Suppressed)
	}
	if s := plan.Suppressed[0]; s.ElementID != "e9" || s.HeaderHeight != 150000 {
		t.Errorf("the suppression record is %+v; want element e9 with HeaderHeight 150,000", s)
	}
}

// --- Story 7.10's discriminator, measured IN THIS PACKAGE ----------------
//
// ItemGroup.AuthorDeclared, overTallGroupMember and overflowKind are all
// internal/layout's own, and until this table every one of them was exercised
// ONLY end-to-end from package folio8 — through ParseTemplate, the shaper, the
// binder and the whole render path. That is a real measurement of the shipped
// behaviour and it is not a substitute for one here: an end-to-end arm cannot
// state the geometry it depends on, so it cannot say which quantity the
// discriminator actually compared, and any change to shaping or to element
// ordering moves it for reasons that have nothing to do with the rule.
//
// These rows are this package's declarative-table discipline (D-000.45):
// every extent is a HAND-DERIVED INTEGER against the 100,000 window
// testGeometry() defines, so what is being compared is written down.

// TestPaginateAuthorDeclaredGroupIsRefusedOnlyForAnOverTallElement is
// D-7.10.1 and D-7.10.2 stated at the deciding site: WHO created the
// grouping, and WHAT inside it is over-tall.
func TestPaginateAuthorDeclaredGroupIsRefusedOnlyForAnOverTallElement(t *testing.T) {
	g := testGeometry()
	key := ItemGroupKey{ElementID: "group", Index: 0}
	author := ItemGroup{Present: true, AuthorDeclared: true, Key: key}
	engine := ItemGroup{Present: true, Key: key}

	// line builds one ColumnItem spanning [top, top+h) carrying one run —
	// the shape a shaped text line reaches Paginate as.
	line := func(id string, top, h geom.Length, ref int, grp ItemGroup) ColumnItem {
		return ColumnItem{ElementID: id, Top: top, Bottom: top + h, Runs: []TextRunRef{TextRunRef(ref)}, Group: grp}
	}
	// stack builds n items of h each, packed with no gaps, ALL BELONGING TO
	// ONE ELEMENT — the decomposition package folio8 performs on a multi-line
	// text element, and the reason the member unit cannot be the ColumnItem.
	stack := func(id string, top, h geom.Length, n int, grp ItemGroup) []ColumnItem {
		var out []ColumnItem
		for i := range n {
			out = append(out, line(id, top+geom.Length(int64(i))*h, h, i, grp))
		}
		return out
	}

	for _, row := range []struct {
		name  string
		items []ColumnItem

		wantFatal     bool
		wantElementID string // when fatal
		wantKind      string // when fatal
		wantExtent    geom.Length
	}{
		{
			// e1 alone is 120,000 against the 100,000 window; e2 is
			// 15,000 and fits. The group's union is 145,000, so the
			// clip branch is entered either way — what differs is what
			// happens inside it.
			name: "author-declared, one member individually over-tall",
			items: []ColumnItem{
				line("e1", testContentTop, 120000, 0, author),
				line("e2", testContentTop+130000, 15000, 1, author),
			},
			wantFatal:     true,
			wantElementID: "e1",
			wantKind:      "line",
			wantExtent:    120000,
		},
		{
			// Both members are 15,000 and each fits; only their union,
			// 125,000, does not. Story 4.6's answer, untouched
			// (D-7.10.4).
			name: "author-declared, over-tall only in aggregate",
			items: []ColumnItem{
				line("e1", testContentTop, 15000, 0, author),
				line("e2", testContentTop+110000, 15000, 1, author),
			},
			wantExtent: 125000,
		},
		{
			// THE ELEMENT-UNIT MEASUREMENT. Eight items of 15,000, every
			// one of them fitting the window on its own, all belonging
			// to ONE element whose union is 120,000. Read the
			// discriminator in ITEMS and this group is "aggregate-only"
			// and is clipped forever, silently destroying a tagged
			// paragraph (DW-50). Read in ELEMENTS it is refused, and the
			// extent reported is the element's own union.
			name:          "author-declared, one element whose ITEMS each fit but whose union does not",
			items:         stack("e1", testContentTop, 15000, 8, author),
			wantFatal:     true,
			wantElementID: "e1",
			wantKind:      "line",
			wantExtent:    120000,
		},
		{
			// THE SCOPE, as a control. Identical geometry to the first
			// row, with the grouping ENGINE-created — a table row. It
			// must still be clipped, and it must be clipped for the
			// reason the narrowing was scoped: a data row's chrome rect
			// spans the row's whole extent, so "some member is
			// individually over-tall" is ALWAYS true of an over-tall
			// row. An unscoped test reverses Story 4.6 wholesale.
			name: "engine-created, one member individually over-tall",
			items: []ColumnItem{
				line("e1", testContentTop, 120000, 0, engine),
				line("e2", testContentTop+130000, 15000, 1, engine),
			},
			wantExtent: 145000,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			plan, err := Paginate(g, row.items)

			if !row.wantFatal {
				if err != nil {
					t.Fatalf("Paginate returned %T: %v — this group must keep Story 4.6's clip-and-warn", err, err)
				}
				if len(plan.Clipped) != 1 {
					t.Fatalf("plan.Clipped = %+v; want exactly one record", plan.Clipped)
				}
				if got := plan.Clipped[0].ItemHeight; got != row.wantExtent {
					t.Errorf("clip.ItemHeight = %d, want the GROUP's union %d", got, row.wantExtent)
				}
				if got := plan.Clipped[0].ContentHeight; got != testContentHeight {
					t.Errorf("clip.ContentHeight = %d, want the window's own %d", got, testContentHeight)
				}
				return
			}

			if err == nil {
				t.Fatalf("Paginate accepted a group holding an individually over-tall element; it must be REFUSED, naming that element. plan = %+v", plan)
			}
			var overflow *OverflowError
			if !errors.As(err, &overflow) {
				t.Fatalf("Paginate returned %v (%T); the refusal must be an *OverflowError so a caller can tell it from a programming error", err, err)
			}
			if overflow.ElementID != row.wantElementID {
				t.Errorf("the refusal names %q, want %q — an UNLOCATED refusal is what D-1.8.1 exists to prevent", overflow.ElementID, row.wantElementID)
			}
			if overflow.Kind != row.wantKind {
				t.Errorf("Kind = %q, want %q", overflow.Kind, row.wantKind)
			}
			if overflow.ItemHeight != row.wantExtent {
				t.Errorf("ItemHeight = %d, want the ELEMENT's own union extent %d — not the group's, and not one item's", overflow.ItemHeight, row.wantExtent)
			}
			if overflow.ContentHeight != testContentHeight {
				t.Errorf("ContentHeight = %d, want the window's own %d", overflow.ContentHeight, testContentHeight)
			}
		})
	}
}

// TestOverflowKindNamesWhatTheItemIs is D-7.10.5's derivation, asserted
// directly rather than only through a rendered document.
//
// The rect arm is the one that needed it: a rectangle in this column is one
// of two entirely different things and only its GROUPING tells them apart —
// a table row's cell chrome, which the ENGINE groups, or an element BOX,
// which nothing groups and which an author-declared tag does not turn into a
// table. Before Story 7.10 both were called a "table", so an author who had
// declared a too-tall rect was told their document contained an over-tall
// table it does not contain (DW-47).
func TestOverflowKindNamesWhatTheItemIs(t *testing.T) {
	key := ItemGroupKey{ElementID: "group", Index: 0}
	for _, row := range []struct {
		name string
		item ColumnItem
		want string
	}{
		{"an image", ColumnItem{ElementID: "e1", Images: []ImageRef{0}}, "image"},
		{"a shaped line", ColumnItem{ElementID: "e1", Runs: []TextRunRef{0}}, "line"},
		{
			"a rect the ENGINE grouped — a table row's cell chrome",
			ColumnItem{ElementID: "e1", Rects: []RectRef{0}, Group: ItemGroup{Present: true, Key: key}},
			"table",
		},
		{
			"an UNGROUPED rect — an element box",
			ColumnItem{ElementID: "e1", Rects: []RectRef{0}},
			"box",
		},
		{
			// The arm that must read AuthorDeclared and not Present
			// alone: a tagged element box is still a box, and the
			// document it comes from may contain no table at all.
			"a rect in an AUTHOR-DECLARED group — still an element box",
			ColumnItem{ElementID: "e1", Rects: []RectRef{0}, Group: ItemGroup{Present: true, AuthorDeclared: true, Key: key}},
			"box",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := overflowKind(row.item); got != row.want {
				t.Errorf("overflowKind = %q, want %q — the word reaches a template author verbatim in OverflowError.Error, so a wrong one sends them looking for a declaration they never wrote", got, row.want)
			}
		})
	}
}
