package folio8

// Story 4.6 (FR25, FR41 · AD-14, AD-24, AD-13, AD-5, AD-1): a table row
// taller than the whole content window is CLIPPED to a page of its own and
// reported as a Warning alongside PDF bytes, instead of killing the render.
//
// This file holds the story's own assertions. The four assertions it
// INVERTS live where they were written (table_pagination_test.go,
// table_footer_test.go, internal/layout/paginate_group_test.go), each with
// a comment naming the story that placed it and this one — see the story's
// blast-radius table.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
)

// --- The two fixtures this story could not be written without --------------

// overTallRowDoc is the DISCRIMINATING fixture D-4.5.5 requires and the
// story's Task 6 names, and it is deliberately NOT tooTallRowDoc().
//
// tooTallRowDoc()'s over-tall row is ONE physical line 272,400mp tall
// against a 110,000mp window: clipping it keeps ZERO lines, so against that
// fixture alone "clip to the window" and "drop all the row's text" render
// IDENTICALLY and a wrong clip boundary is invisible. That document is kept
// here as the DEGENERATE case (AC1), never as the only case.
//
// THE FIXTURE'S PAINT MOVED AT SPEC-table-rules, and the over-tall row moved
// with it. The chrome used to be a table `style.background`/`style.border`,
// which §1 now gives to the table's OWN BOX — a single rect spanning the whole
// table, which paints no cell and is a different pagination question entirely.
// A data cell's only remaining fill is `altRowBackground`, which applies to ODD
// zero-based collection indexes, so the over-tall row is at index 11 rather
// than 12 and the repeated header paints from `headerStyle.background`. Nothing
// about what these tests assert changed: the row is still over-tall, still
// alone on its page, and still truncated at the content bottom.
//
// This one is an ordinary 8pt table in a narrow column. One record's cell
// carries a wall of text that wraps to fourteen lines, which is what makes
// the row too tall — a height DERIVED FROM DATA the author never saw, which
// is exactly the asymmetry D-4.6.2 rests on. Measured at this commit:
//
//	content window   10,000 .. 120,000mp   (110,000mp tall)
//	header row       10,000 ..  20,000mp
//	data rows 0..10  10,896mp each, 20,000 .. 139,856mp
//	the over-tall row (index 11)  139,856 .. 292,400mp  (152,544mp)
//	  its lines      14 of them, 10,896mp each, at 139,856 + 10,896k
//	trailing rows 12..14  292,400 .. 325,088mp
//
// D-000.80 part (a), and it is load-bearing here: the twelve rows BEFORE
// the over-tall one exist so that "at the top of a fresh page" means page
// N>0. Page 0 is always a fresh page, so a fixture whose over-tall row
// comes first satisfies AC2 accidentally. The three rows AFTER it exist so
// that "the clip does not reflow the rest of the document" has a successor
// to be true of.
func overTallRowDoc() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "headerStyle": {"background": "#DDDDDD"},
        "altRowBackground": "#DDDDDD",
        "columns": [
          {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"},
          {"id": "e3", "label": "B", "width": 60, "bind": "{{row.b}}"}
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

// overTallRowData binds `before` ordinary records, then ONE record whose
// first cell holds `words` short words (the wall of text that makes the row
// too tall), then `after` ordinary records.
func overTallRowData(before, words, after int) string {
	type record struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	items := make([]record, 0, before+1+after)
	for i := 0; i < before; i++ {
		items = append(items, record{A: fmt.Sprintf("N%d", i), B: fmt.Sprintf("n%d", i)})
	}
	var wall strings.Builder
	for w := 0; w < words; w++ {
		fmt.Fprintf(&wall, "W%02d ", w)
	}
	items = append(items, record{A: wall.String(), B: "tall"})
	for i := 0; i < after; i++ {
		items = append(items, record{A: fmt.Sprintf("T%d", i), B: fmt.Sprintf("t%d", i)})
	}
	b, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		panic(err) // test-fixture construction only; unreachable through Render
	}
	return string(b)
}

// The measured constants of overTallRowDoc()/overTallRowData(11, 40, 3),
// named once so every assertion below reads against the same document
// rather than against a number retyped four times.
const (
	clipFixtureBefore = 11
	clipFixtureWords  = 40
	clipFixtureAfter  = 3

	// The over-tall row's index within the bound collection — the number
	// AC4 requires the diagnostic to name.
	clipFixtureRowIndex = clipFixtureBefore

	clipFixtureContentHeight geom.Length = 110_000
	clipFixtureRowTop        geom.Length = 139_856
	clipFixtureRowBottom     geom.Length = 292_400
	clipFixtureRowHeight     geom.Length = clipFixtureRowBottom - clipFixtureRowTop // 152,544mp

	// Origins(g).Content for this fixture: the top edge of the content
	// band, in the page model's own printable-corner space.
	clipFixtureContentTop geom.Length = 10_000

	// The table's own declared headerHeight ("headerHeight": 10 → 10pt),
	// which is also the height its REPEAT reserves on every continuation
	// page — the clipped row's page included (D-4.6.4).
	clipFixtureHeaderHeight geom.Length = 10_000

	// The cut: the content bottom of the one page the over-tall row got.
	// It is the window's own bottom LESS the repeated header's height,
	// because D-4.6.4 composes FR26's repeat with the clip rather than
	// letting the clip short-circuit it: the header is drawn at the top of
	// the band and the row is clipped into what is left below it.
	clipFixtureCutAt geom.Length = clipFixtureRowTop + clipFixtureContentHeight - clipFixtureHeaderHeight // 239,856mp
)

func overTallRowFixtureData() string {
	return overTallRowData(clipFixtureBefore, clipFixtureWords, clipFixtureAfter)
}

// --- AC1: the render completes and returns bytes ---------------------------

// TestOverTallRowRendersBytesRatherThanFailing is AC1, and it is the
// assertion that brings the code to the spine. ARCHITECTURE-SPINE.md's
// AD-14 says, verbatim, that "Rows and author-declared keep-together
// groups too tall for one content window (FR25, FR51), and clipped
// content (FR44), are `Warning`s returned alongside PDF bytes, never
// silent and never fatal" — a sentence Story 7.9 widened from "Over-tall
// rows (FR25)" to describe the keep-together clipping Story 7.7 had
// already shipped. It says TOO TALL of both, because an ordinary group
// that merely slides warns about nothing. Story 7.10 then APPENDED a
// discriminator clause to that same bullet rather than rewording the
// sentence quoted above, so the quote stays verbatim: a keep-together
// group is lenient only IN AGGREGATE, and one whose own template element
// is by itself too tall is refused with a located Error (D-7.10.1). Every
// subject of THIS test is a table ROW, which the clause leaves exactly
// where Story 4.6 put it. Measured at
// 45cf812, both documents below returned Result.Bytes
// of length 0 and a *RenderError carrying CONTENT_UNLAYOUTABLE — HEAD was
// in violation, and this test is the record that it no longer is.
//
// D-000.80 part (a): nothing here was accidentally true. The zero-byte
// error was re-measured at this story's own start, through this same public
// entry point, before a line of the clip was written.
//
// AC1 DECLARED TWO OBSERVABLES — (i) the error is nil; (ii) the bytes are
// non-empty and structurally valid — and said a clip that dropped the
// group's every ref would keep (i) green and redden (ii) alone.
//
// MEASURED AT THE FINISHER'S BASELINE, IT DOES NOT. Clipping to zero
// height, so the over-tall group contributes nothing to any page, leaves
// this test GREEN in both its subtests: the render still returns a nil
// error, still returns bytes, and the page tree still resolves, because a
// document that destroyed one row is still a well-formed document. Other
// tests redden (AC3's kept-line set, D-4.6.4's), but AC1's own does not.
//
// So observable (ii) has no witness that can fail while (i) passes, and
// AC1 is carried as a Class A compound-observable entry in the story's
// ledger rather than as two. Stated here rather than left as a claim the
// next reader would take on trust — the declared count is what the
// programme measures against, and shrinking it quietly is the failure mode
// D-000.85 exists to catch.
func TestOverTallRowRendersBytesRatherThanFailing(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		data string
	}{
		{
			// The DEGENERATE case, kept: a single 200pt line, taller
			// than the window all by itself, so the clip keeps none of
			// its lines. It must still produce a document.
			name: "single-line row (the degenerate case)",
			doc:  tooTallRowDoc(),
			data: multiRowTableData(1, -1),
		},
		{
			name: "multi-line row after a full page (the discriminating case)",
			doc:  overTallRowDoc(),
			data: overTallRowFixtureData(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(tc.doc))
			if err != nil {
				t.Fatalf("ParseTemplate: %v", err)
			}
			res, rerr := Render(tpl, Data(tc.data), nil, testShippedFontSet())

			// (i) never fatal.
			if rerr != nil {
				t.Fatalf("Render returned %T: %v\n\nAD-14: an over-tall ROW is a Warning returned alongside PDF bytes, never fatal. At 45cf812 this returned a *RenderError with CONTENT_UNLAYOUTABLE and zero bytes; that was the spine violation this story exists to close.", rerr, rerr)
			}

			// (ii) and it produced a real document, not an empty one.
			if len(res.Bytes) == 0 {
				t.Fatal("Render returned a nil error and ZERO bytes — 'alongside PDF bytes' means there are bytes")
			}
			if !AssertPDFPageTreeResolves(t, res.Bytes, tc.name) {
				t.Error("the clipped document's page tree does not resolve — a document that exists but cannot be opened is not a document")
			}
		})
	}
}

// --- AC2: alone on a fresh page, and the neighbours do not move ------------

// rowPartition renders the fixture through the real collectors and returns
// {page -> the table rows on it}, read off each rect's OWN identity fields
// (isHeaderRow/rowIndex) rather than re-derived from geometry or order.
// "H" is the table's own header row; a data row is its index.
func rowPartition(t *testing.T, plan layout.Pagination, tableRects []tableRectSource) map[int][]string {
	t.Helper()
	out := map[int][]string{}
	for pageIdx, pa := range plan.Pages {
		for _, ref := range pa.ContentRects {
			r := tableRects[ref]
			switch {
			case r.isHeaderRow:
				out[pageIdx] = append(out[pageIdx], "H")
			case r.isFooterRow:
				out[pageIdx] = append(out[pageIdx], "F")
			default:
				out[pageIdx] = append(out[pageIdx], fmt.Sprintf("%d", r.rowIndex))
			}
		}
	}
	return out
}

// TestClippedRowLandsAloneOnAFreshPageAndItsNeighboursDoNotMove is AC2.
//
// The criterion is the SET {page -> row indices} (D-000.68 / D-000.83), not
// a page count: a wrong arrangement produces the same count, so
// `len(plan.Pages) == 4` would be satisfied by the rows landing anywhere.
//
// TWO OBSERVABLES: (i) the over-tall row's page carries no other row;
// (ii) the rows before AND after it land exactly where they did — the clip
// does not reflow the document around itself (AD-24: siblings never move).
func TestClippedRowLandsAloneOnAFreshPageAndItsNeighboursDoNotMove(t *testing.T) {
	plan, _, tableRects := paginateContentTableForTest(t, overTallRowDoc(), overTallRowFixtureData())
	got := rowPartition(t, plan, tableRects)

	// Hand-derived from the fixture's own measured extents (see
	// overTallRowDoc's table) against the four-rule window model:
	// page 0 takes the header and every row whose bottom is <= 120,000;
	// page 1 slides to row 9's top and takes rows 9..10 under FR26's
	// header reservation; row 11 is over-tall and takes page 2 ALONE;
	// rows 12..14 resume on page 3.
	want := map[int][]string{
		0: {"H", "0", "1", "2", "3", "4", "5", "6", "7", "8"},
		1: {"9", "10"},
		2: {"11"},
		3: {"12", "13", "14"},
	}

	// (i) THE OVER-TALL ROW IS ALONE. Asserted on its own so it can
	// redden on its own: deleting the clip branch's fresh-page step
	// leaves row 12 sharing page 1 with rows 9..11.
	const clippedPage = 2
	if len(got[clippedPage]) != 1 || got[clippedPage][0] != fmt.Sprintf("%d", clipFixtureRowIndex) {
		t.Errorf("the over-tall row's page carries %v; want exactly [%d] — an over-tall row is placed ALONE on a fresh page, not clipped in place wherever the window had reached",
			got[clippedPage], clipFixtureRowIndex)
	}

	// ... and "fresh page" means a LATER page than the one holding the
	// rows before it, which is the part a fixture whose over-tall row
	// came first would satisfy for free (D-000.80 part (a)).
	if clippedPage == 0 {
		t.Fatal("fixture defect: the over-tall row landed on page 0, where 'a fresh page' is accidentally true")
	}

	// (ii) EVERY OTHER ROW IS WHERE IT WAS — predecessors and
	// successors alike, pinned BY VALUE.
	if len(got) != len(want) {
		t.Fatalf("the document paginated into %d page(s) carrying rows; the hand-derived partition has %d.\ngot  %v\nwant %v", len(got), len(want), got, want)
	}
	for page, wantRows := range want {
		gotRows := got[page]
		if strings.Join(gotRows, ",") != strings.Join(wantRows, ",") {
			t.Errorf("page %d carries rows %v; want %v", page, gotRows, wantRows)
		}
	}
}

// TestClippingOneRowPushesNothingOffItsPage is AC2's SECOND observable,
// as its own test so it can stay green while the first reddens (D-000.85:
// an AC producing two observables owes two proofs, and a boolean standing
// in for a set is what that ruling exists to stop).
//
// The property is AD-24's: the clip must not reflow the document around
// itself. Stated as the harm rather than as a page number — every item on
// every page is drawn INSIDE that page's content band. A clip that
// advanced the window past the group's untruncated height would leave the
// rows after it drawn below the bottom of the paper, which no assertion
// about page INDICES can see (the row is still "on page 2"; it is simply
// off the sheet).
//
// This formulation is a REPLACEMENT for the one the story named, and the
// reason is recorded rather than substituted quietly (D-000.79): the
// story's "the preceding rows still land where they did" is a claim about
// page indices, and losing the fresh page shifts every later page index by
// one — so it cannot stay green under AC2's own first deletion, and the
// two observables would not be separated by it.
//
// THE CLAIM THAT USED TO STAND HERE — "This one can, and does" — WAS
// FALSE, and it was the reviewer who measured it, not this file. Removing
// the fresh-page step reddens this test as well as its sibling, so the
// replacement separated the two observables no better than the
// formulation it replaced. Recorded here rather than quietly corrected,
// because a screen outcome that measurement contradicts is worse than an
// admitted gap: it is a data point in D-000.85's series pointing the wrong
// way.
//
// WHERE IT STANDS NOW, all measured at the finisher's own baseline:
//
//   - This test (AC2's observable ii) IS separably witnessed. Setting the
//     clip's header reservation to zero — `reserved = hh` → `reserved = 0`,
//     so the repeat is still drawn and the row is still displaced beneath
//     it, but the cut is not narrowed to match — keeps one extra line and
//     draws it past the bottom of the sheet. This test reddens; its
//     sibling stays GREEN. That mutation exists only because D-4.6.4
//     introduced the reservation, so the reviewer's finding was correct
//     about the code as it stood.
//
//   - The SIBLING's observable (i), "the over-tall row is alone on its
//     page", is NOT separably witnessed. Both mutations tried — deleting
//     the fresh-page step, and hand-placing the preceding row onto the
//     clipped page — redden this test too, because either one changes the
//     page's Shift and puts content outside the band. It is carried as a
//     Class A compound-observable entry in the story's ledger rather than
//     counted as a witness.
//
// This test is also what makes the reservation observable at all, and that
// is not incidental: it reads each item's DRAWN extent, which means the
// page Shift AND the per-table RowDisplacement. Reading only the Shift —
// as it did at review — under-reports every row of a repeating table by
// the header's height, which is exactly the direction that hides content
// pushed off the bottom.
func TestClippingOneRowPushesNothingOffItsPage(t *testing.T) {
	plan, contentRuns, tableRects := paginateContentTableForTest(t, overTallRowDoc(), overTallRowFixtureData())

	const contentTop = clipFixtureContentTop
	bandBottom := contentTop + clipFixtureContentHeight

	checked := 0
	for pageIdx, pa := range plan.Pages {
		for _, ref := range pa.ContentRects {
			r := tableRects[ref]
			// The DRAWN extent, and it is drawn with BOTH transforms
			// render.go applies: the page's own window shift, and the
			// per-table RowDisplacement that FR26's repeated header
			// pushes that table's rows down by. Reading only the shift
			// under-reports every row of a repeating table by the
			// header's height — which is precisely the direction that
			// hides content pushed off the BOTTOM of the sheet, and the
			// clipped row's page carries a repeat too (D-4.6.4).
			disp := rowDisplacementFor(pa.RowDisplacement, r.elementID)
			top, bottom := r.top-pa.Shift+disp, r.bottom-pa.Shift+disp
			// The clipped row's own chrome is truncated, so read the
			// bound the clip imposed rather than the untruncated
			// rectangle it was built with.
			if b, ok := rectClipBottomFor(pa.ClippedRects, ref); ok && r.bottom > b {
				bottom = b - pa.Shift + disp
			}
			checked++
			if top < contentTop || bottom > bandBottom {
				t.Errorf("page %d: table row rect %d is drawn at %d..%dmp, outside the page's content band %d..%dmp.\nA clip that advances the window past the group's untruncated height — or that repeats a header without clipping the row into the space BELOW it — pushes content off the sheet, and a page-INDEX assertion cannot see that, because the row is still 'on' the page.",
					pageIdx, ref, top, bottom, contentTop, bandBottom)
			}
		}
		for _, ref := range pa.ContentRuns {
			r := contentRuns[ref]
			disp := geom.Length(0)
			if r.isTableRowLine || r.isFooterLine {
				disp = rowDisplacementFor(pa.RowDisplacement, r.elementID)
			}
			top, bottom := r.itemTop-pa.Shift+disp, r.itemBottom-pa.Shift+disp
			checked++
			if top < contentTop || bottom > bandBottom {
				t.Errorf("page %d: run %d (line %d of element %s) is drawn at %d..%dmp, outside the page's content band %d..%dmp",
					pageIdx, ref, r.lineIndex, r.elementID, top, bottom, contentTop, bandBottom)
			}
		}
	}

	// VACUITY GUARD (D-000.9): a walk that entered nothing is trivially
	// inside every band, and reports exactly the all-clear a healthy one
	// gives. 60 placed items were measured here at Story 4.6.
	//
	// The floor is deliberately well BELOW that measurement rather than
	// equal to it. This test's subject is WHERE things are drawn, not HOW
	// MANY are drawn: a change that legitimately drops more lines (the
	// clip's own boundary moving, which is AC3's subject and has AC3's
	// own assertions) would trip an exact-count floor and turn this guard
	// into a second, weaker copy of AC3. Guarding vacuity is what it is
	// for; conservation is asserted where conservation is the claim.
	const emptyWalkFloor = 30
	if checked < emptyWalkFloor {
		t.Fatalf("vacuity guard: only %d placed item(s) were checked, below the %d floor (60 were measured at Story 4.6) — an empty or truncated walk is trivially inside every content band", checked, emptyWalkFloor)
	}
}

// --- D-4.6.4: the clipped row's page keeps its column headers -------------

// headerRepeatsOn returns the number of repeated headers page p carries for
// elementID — read off the plan's own HeaderRepeats channel, which is where
// FR26's repeats travel and is NOT the ContentRects channel rowPartition
// reads. That separation is exactly why the defect D-4.6.4 closes was
// invisible to every assertion this story shipped at review.
func headerRepeatsOn(plan layout.Pagination, p int, elementID string) int {
	n := 0
	for _, rep := range plan.Pages[p].HeaderRepeats {
		if rep.ElementID == elementID {
			n++
		}
	}
	return n
}

// TestClippedRowsPageStillCarriesItsRepeatedHeader is D-4.6.4, and it is
// the assertion whose absence let this story ship a page that silently lost
// its column headers.
//
// As written at review, the clip branch `continue`d before Story 4.4's
// entire DECISION-2/DECISION-3 block, so the clipped row's page got no
// repeat AND no suppression record: measured on this very fixture,
// headerRepeats was 1 on pages 1 and 3 and 0 on page 2, with plan.Suppressed
// empty for the whole document.
//
// The ruling is that the header REPEATS. 4.4's suppression has a stated
// TRIGGER — reserving the header leaves no room for a row — and that
// trigger was never met here: there IS a row on the page. Applying 4.4's
// remedy where 4.4's condition never fired is the defect; recording it
// would only have documented a suppression that should not be happening.
// And the substance agrees: of every page in the document, the one carrying
// a row that has already been cut is the one that can least afford to lose
// the labels saying what its surviving cells mean.
//
// So the two rules COMPOSE — repeat the header, then clip the row into what
// is left below it — with 4.4's own arm (c) as the floor when the
// reservation would leave room for not even one line.
//
// TWO OBSERVABLES, and they are separable (D-000.85):
// (i) the repeat is DRAWN on the clipped page, above the row and inside the
// band; (ii) the cut is NARROWED by the header's height, which is what makes
// this a composition rather than a header drawn on top of the row.
// A third, (iii), covers the floor arm: when no line survives the
// reservation, the repeat is suppressed and RECORDED through 4.4's path.
func TestClippedRowsPageStillCarriesItsRepeatedHeader(t *testing.T) {
	plan, contentRuns, _ := paginateContentTableForTest(t, overTallRowDoc(), overTallRowFixtureData())

	const clippedPage = 2
	const contentTop = clipFixtureContentTop

	// ANTI-VACUITY, and D-000.80 part (a)'s accidental cause named and
	// removed: "page 2 has a repeat" would be worth nothing if every page
	// had one unconditionally. Page 0 is the table's OWN header page, and
	// DECISION-1 says a header is never repeated above itself — so the
	// repeat is a DECISION per page, not a blanket, and page 2's is a
	// decision that went the right way rather than a constant.
	if got := headerRepeatsOn(plan, 0, "e1"); got != 0 {
		t.Fatalf("page 0 carries %d repeated header(s); the table's OWN header page must carry none (DECISION-1) — if every page gets a repeat unconditionally, this test's subject is a constant and proves nothing", got)
	}
	// ... and the ordinary continuation pages DO repeat, which is the
	// baseline page 2 was measured against and found short of.
	for _, p := range []int{1, 3} {
		if got := headerRepeatsOn(plan, p, "e1"); got != 1 {
			t.Fatalf("page %d carries %d repeated header(s); want 1 — this is an ordinary continuation page and FR26 repeats there", p, got)
		}
	}

	// (i) THE CLIPPED PAGE REPEATS TOO. Its own named subtest, so
	// deleting only the HeaderRepeats append reddens here and nowhere
	// else.
	t.Run("the clipped row's page repeats the header", func(t *testing.T) {
		if got := headerRepeatsOn(plan, clippedPage, "e1"); got != 1 {
			t.Fatalf("page %d — the page carrying the CLIPPED row — carries %d repeated header(s); want 1.\nAt review this was 0 while pages 1 and 3 had theirs, and plan.Suppressed was empty: the reader got an unlabelled, truncated table row and no warning at all. A clipped row is already degraded; stripping its column headers makes what survived uninterpretable.", clippedPage, got)
		}
	})

	// (ii) AND THE ROW WAS CLIPPED INTO THE SPACE BELOW IT, not under it.
	// This is the half that makes the two rules COMPOSE: a repeat drawn
	// over the row's own first line would satisfy (i) and still be wrong.
	// Asserted as the drawn geometry rather than as the bound alone, so
	// it reads as the harm (overlap) rather than as arithmetic.
	t.Run("the row is clipped into the space below the repeat, never under it", func(t *testing.T) {
		pa := plan.Pages[clippedPage]
		disp := rowDisplacementFor(pa.RowDisplacement, "e1")
		if disp != clipFixtureHeaderHeight {
			t.Errorf("the clipped page displaces the table's rows by %dmp; want the repeated header's own height, %dmp — without the displacement the repeat is drawn ON TOP of the row's first line", disp, clipFixtureHeaderHeight)
		}

		// The repeat occupies contentTop .. contentTop+headerHeight; every
		// kept line of the clipped row must begin at or below that.
		repeatBottom := contentTop + clipFixtureHeaderHeight
		clippedKey := layout.ItemGroupKey{ElementID: "e1", Index: clipFixtureRowIndex}
		kept := 0
		for _, ref := range pa.ContentRuns {
			r := contentRuns[ref]
			if r.lineRowGroup().Key != clippedKey {
				continue
			}
			kept++
			if drawnTop := r.itemTop - pa.Shift + disp; drawnTop < repeatBottom {
				t.Errorf("line %d of the clipped row is drawn at %dmp, above the repeated header's bottom edge at %dmp — the repeat and the row overlap", r.lineIndex, drawnTop, repeatBottom)
			}
		}
		if kept == 0 {
			t.Fatal("vacuity guard: no line of the clipped row reached the clipped page, so 'below the repeat' is trivially true — this arm needs a row with surviving lines")
		}
	})

	// (iii) NOTHING WAS SILENTLY SUPPRESSED. With the repeat honoured
	// there is no suppression to record, so this is now a TRUE statement
	// about the document rather than the empty slice that was covering a
	// silent loss at review.
	t.Run("no suppression is recorded, because none happened", func(t *testing.T) {
		if len(plan.Suppressed) != 0 {
			t.Errorf("plan.Suppressed = %+v; want empty — every page of this document that should repeat the header does repeat it", plan.Suppressed)
		}
	})
}

// TestClippedRowSuppressesTheRepeatOnlyWhenItBuysNothingAndRecordsIt is
// D-4.6.4's FLOOR arm, and the reason the composition terminates.
//
// tooTallRowDoc()'s single row is ONE physical line 272,400mp tall against
// a 110,000mp window. No line of it survives the cut with or without the
// header's reservation — so repeating the header would cost 10,000mp of a
// row that is already being destroyed and buy the reader nothing. Story
// 4.4's DECISION-2 arm (c) then fires ON ITS OWN TERMS ("even alone, this
// row does not fit under the reservation"), and is RECORDED through the
// very TableHeaderSuppressed channel 4.4 built for it.
//
// This is the discriminating pair for D-4.6.4: two real documents, one per
// arm, differing in exactly the property the branch keys on.
func TestClippedRowSuppressesTheRepeatOnlyWhenItBuysNothingAndRecordsIt(t *testing.T) {
	plan, _, _ := paginateContentTableForTest(t, tooTallRowDoc(), multiRowTableData(1, -1))

	if len(plan.Clipped) != 1 {
		t.Fatalf("plan.Clipped = %+v; want exactly one clipped group", plan.Clipped)
	}
	clippedPage := plan.Clipped[0].Page

	if got := headerRepeatsOn(plan, clippedPage, "e1"); got != 0 {
		t.Errorf("page %d repeats the header %d time(s); want 0 — not one line of this row survives the reservation, so the repeat costs height and buys the reader nothing", clippedPage, got)
	}

	// THE RECORD, and it is the load-bearing half. "No repeat here" is
	// accidentally true of any table that never repeats; "no repeat here,
	// and here is the record saying so" is not. Measured before this fix:
	// plan.Suppressed was EMPTY for this document.
	if len(plan.Suppressed) != 1 {
		t.Fatalf("plan.Suppressed = %+v; want exactly one record. AD-14 and folio-format.md both say nothing is silent in either direction — a page that loses its header repeat says so", plan.Suppressed)
	}
	s := plan.Suppressed[0]
	if s.ElementID != "e1" || s.Page != clippedPage {
		t.Errorf("the suppression record names element %q on page %d; want %q on page %d (the clipped row's own page)", s.ElementID, s.Page, "e1", clippedPage)
	}
	if s.HeaderHeight != 10_000 {
		t.Errorf("the record's HeaderHeight is %dmp; want the table's declared headerHeight, 10,000mp — the number the author needs to act on it", s.HeaderHeight)
	}

	// ... and it reaches the reader, on the bytes channel, as 4.4's own
	// code rather than as a second spelling of the clip's.
	tpl, err := ParseTemplate([]byte(tooTallRowDoc()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, rerr := Render(tpl, Data(multiRowTableData(1, -1)), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render returned %T: %v", rerr, rerr)
	}
	suppressions := 0
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableHeaderRepeatSuppressed {
			suppressions++
		}
	}
	if suppressions != 1 {
		t.Errorf("Result.Diagnostics carries %d %s warning(s); want exactly 1. All: %+v", suppressions, DiagCodeTableHeaderRepeatSuppressed, res.Diagnostics)
	}
}

// --- AC3: whole lines only, and the chrome is truncated -------------------

// TestClippedRowKeepsTheLinesThatFitAndTruncatesItsChrome is AC3, written
// against the multi-line fixture for the reason D-4.5.5 gives: on a
// single-line row, "clip to the window" and "drop the row's text entirely"
// are the same rendering, so the boundary is untestable there.
//
// THREE OBSERVABLES: (i) the kept-line set; (ii) the dropped-line set,
// including the one that STRADDLES the boundary; (iii) the chrome rect's
// truncated bottom.
func TestClippedRowKeepsTheLinesThatFitAndTruncatesItsChrome(t *testing.T) {
	plan, contentRuns, tableRects := paginateContentTableForTest(t, overTallRowDoc(), overTallRowFixtureData())

	clippedKey := layout.ItemGroupKey{ElementID: "e1", Index: clipFixtureRowIndex}

	// Every line of the over-tall row, by its own lineIndex, with the
	// extent the vertical model gave it — read off the runs themselves,
	// never re-derived.
	var all []clippedLine
	seen := map[int]bool{}
	for _, r := range contentRuns {
		if r.lineRowGroup().Key != clippedKey || seen[r.lineIndex] {
			continue
		}
		seen[r.lineIndex] = true
		all = append(all, clippedLine{r.lineIndex, r.itemTop, r.itemBottom})
	}
	if len(all) < 3 {
		t.Fatalf("fixture defect (D-4.5.5): the over-tall row has %d line(s). A clip boundary is only observable on a row with lines on BOTH sides of it; with fewer, 'clip to the window' and 'drop the row's text' render identically", len(all))
	}

	// The boundary line, NAMED explicitly as the story requires: the one
	// whose extent straddles the cut. Its presence is what makes the
	// kept/dropped split a proper non-empty partition of a non-empty set.
	straddler := -1
	for _, l := range all {
		if l.top < clipFixtureCutAt && l.bottom > clipFixtureCutAt {
			straddler = l.index
		}
	}
	if straddler < 0 {
		t.Fatalf("fixture defect: no line of the over-tall row straddles the cut at %dmp, so 'never a straddle' is vacuous here", clipFixtureCutAt)
	}
	// 21 since SPEC-table-rules moved the over-tall row from collection
	// index 12 to 11 (see overTallRowDoc): every line index above it drops
	// by the one row's worth of lines, and the cut with it.
	if straddler != 21 {
		t.Errorf("the straddling line is line %d; the fixture's measured boundary line is 21 (extent 237,920..248,816mp against a cut at %dmp)", straddler, clipFixtureCutAt)
	}

	// The kept and dropped sets, hand-derived from the rule "a line is
	// present iff its whole extent lies within the page's window".
	wantKept, wantDropped := map[int]bool{}, map[int]bool{}
	for _, l := range all {
		if l.bottom <= clipFixtureCutAt {
			wantKept[l.index] = true
		} else {
			wantDropped[l.index] = true
		}
	}
	if len(wantKept) == 0 || len(wantDropped) == 0 {
		t.Fatalf("fixture defect: the clip keeps %d line(s) and drops %d — AC3 needs both sides non-empty", len(wantKept), len(wantDropped))
	}

	// What actually reached a page, anywhere in the document.
	gotPlaced := map[int]bool{}
	for _, pa := range plan.Pages {
		for _, ref := range pa.ContentRuns {
			r := contentRuns[ref]
			if r.lineRowGroup().Key == clippedKey {
				gotPlaced[r.lineIndex] = true
			}
		}
	}

	// (i) THE KEPT SET. Each observable is its OWN named subtest, so a
	// deletion screen's answer is a SET of test names rather than one
	// boolean standing in for three (D-000.85).
	t.Run("kept lines", func(t *testing.T) {
		for idx := range wantKept {
			if !gotPlaced[idx] {
				t.Errorf("line %d of the over-tall row (bottom %dmp, within the cut at %dmp) is absent; every line that FITS the window must be drawn",
					idx, lineBottom(all, idx), clipFixtureCutAt)
			}
		}
	})

	// (ii) THE DROPPED SET, the straddler included.
	t.Run("dropped lines including the straddler", func(t *testing.T) {
		for idx := range wantDropped {
			if gotPlaced[idx] {
				t.Errorf("line %d of the over-tall row (bottom %dmp, past the cut at %dmp) was drawn; a line that crosses or lies beyond the content bottom is dropped WHOLE — AD-24 forbids drawing half of one",
					idx, lineBottom(all, idx), clipFixtureCutAt)
			}
		}
		if gotPlaced[straddler] {
			t.Errorf("the straddling line %d was drawn — this is the never-a-straddle case itself, not an ordinary drop", straddler)
		}

	})

	// (iii) THE CHROME. The row's rectangle is TRUNCATED at the content
	// bottom rather than drawn to its untruncated height. It is a
	// coordinate bound, applied by the caller against the rect's own
	// geometry (AD-5: no PDF clip path, and none needed).
	t.Run("chrome truncated at the content bottom", func(t *testing.T) {
		var chromeRef layout.RectRef = -1
		for i, r := range tableRects {
			if r.chromeRowGroup().Key == clippedKey {
				chromeRef = layout.RectRef(i)
			}
		}
		if chromeRef < 0 {
			t.Fatal("fixture defect: the over-tall row has no chrome rect")
		}
		var gotBound geom.Length = -1
		for _, pa := range plan.Pages {
			if b, ok := rectClipBottomFor(pa.ClippedRects, chromeRef); ok {
				gotBound = b
			}
		}
		if gotBound != clipFixtureCutAt {
			t.Errorf("the over-tall row's chrome rect is bounded at %dmp; want the page's content bottom, %dmp (its own untruncated bottom is %dmp — drawing it there would run the row's rectangle off the bottom of the page)",
				gotBound, clipFixtureCutAt, clipFixtureRowBottom)
		}
	})
}

// TestClippedRowsChromeIsTruncatedInTheRenderedDocument is AC3's
// observable (iii) asserted WHERE THE BYTES ARE PRODUCED, and it exists
// because the version of AC3(iii) this story shipped at review could not
// see its own subject.
//
// Two independent causes, both measured by the reviewer and both closed
// here — and BOTH had to be, or the assertion re-inerts:
//
//  1. WRONG REF SPACE. The sibling assertion above reads
//     layout.Pagination.ClippedRects against itemsForTest's SYNTHETIC
//     RectRef space, which gives one ref per tableRectSource. Production
//     flattens `ts.rects...` into pdfRects, so one source yields N refs —
//     N being the column count. This test reads the PAGE MODEL, which is
//     that flattened space, and pins the count so the two spaces cannot
//     silently diverge again.
//
//  2. NOTHING WAS PAINTED. Neither fixture declared a cell fill or a
//     border, so a truncated rectangle emitted no marks at all and the
//     rendered PDF was BYTE-IDENTICAL with and without the truncation
//     (62,482 bytes, same hash). That is D-4.5.5 exactly: a behaviour is
//     only tested by an input that would render differently under a wrong
//     implementation. overTallRowDoc() now declares a background AND a
//     border, so the row's rectangle is a thing a reader can see, and the
//     precondition is asserted here rather than assumed.
//
// The deletion this closes is the render-side application itself
// (render.go's `if bottom, clip := rectClipBottomFor(...)`), which at
// review produced ZERO new failures across the whole suite.
func TestClippedRowsChromeIsTruncatedInTheRenderedDocument(t *testing.T) {
	tpl, err := ParseTemplate([]byte(overTallRowDoc()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, overTallRowFixtureData()), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}

	const clippedPage = 2
	if len(pages) <= clippedPage {
		t.Fatalf("the document has %d page(s); the clipped row is on page %d", len(pages), clippedPage)
	}
	bandBottom := clipFixtureContentTop + clipFixtureContentHeight

	// PRECONDITION, and it is cause (2) asserted rather than assumed: a
	// rectangle that paints nothing cannot witness its own truncation.
	t.Run("the fixture's cells actually paint", func(t *testing.T) {
		painted := 0
		for _, r := range pages[clippedPage].Rects {
			if r.HasFill || r.HasStroke {
				painted++
			}
		}
		if painted != len(pages[clippedPage].Rects) || painted == 0 {
			t.Fatalf("%d of %d rects on the clipped page paint anything (fill or stroke); want all of them, and more than none.\nWithout a visible cell chrome a truncated rectangle emits no marks and the rendered document is byte-identical whether the truncation happens or not — which is exactly how this assertion came to be unobservable (D-4.5.5).", painted, len(pages[clippedPage].Rects))
		}
	})

	// THE FLATTENED SPACE, pinned. Two header-repeat cells and two
	// clipped-row cells, because the table has TWO COLUMNS: one
	// tableRectSource yields N page-model rects, not one. An assertion
	// written against itemsForTest's one-ref-per-source space cannot see
	// this and would pass against either.
	t.Run("production flattens one row into one rect per column", func(t *testing.T) {
		if got := len(pages[clippedPage].Rects); got != 4 {
			t.Errorf("the clipped page carries %d page-model rect(s); want 4 — two cells of the repeated header plus two cells of the clipped row, the table having two columns.\nThis is the FLATTENED pdfRects space; itemsForTest's synthetic space would give 2.", got)
		}
	})

	// (iii) THE TRUNCATION, in the space the serializer reads.
	t.Run("the clipped row's rectangle stops at the content bottom", func(t *testing.T) {
		wantTop := clipFixtureContentTop + clipFixtureHeaderHeight
		wantH := clipFixtureContentHeight - clipFixtureHeaderHeight
		found := 0
		for i, r := range pages[clippedPage].Rects {
			if r.Y != wantTop {
				continue // the repeated header's own cells, at the band top
			}
			found++
			if r.H != wantH {
				t.Errorf("rect %d of the clipped page is %dmp tall; want %dmp — the content height less the repeated header's reservation. Its UNTRUNCATED height is %dmp, which would draw the row's rectangle to %dmp, past the content bottom at %dmp and past the bottom of the paper.",
					i, r.H, wantH, clipFixtureRowHeight, r.Y+clipFixtureRowHeight, bandBottom)
			}
			if r.Y+r.H != bandBottom {
				t.Errorf("rect %d of the clipped page ends at %dmp; want the content bottom, %dmp", i, r.Y+r.H, bandBottom)
			}
		}
		if found != 2 {
			t.Fatalf("found %d clipped-row rect(s) at Y=%dmp on the clipped page; want 2 (one per column)", found, wantTop)
		}
	})

	// ... and NOTHING anywhere in the document is drawn below the band.
	// Stated over every page so a clip that merely moved the harm to
	// another page cannot pass.
	t.Run("no rectangle in the document is drawn below the content bottom", func(t *testing.T) {
		checked := 0
		for p, page := range pages {
			for i, r := range page.Rects {
				checked++
				if r.Y+r.H > bandBottom {
					t.Errorf("page %d rect %d is drawn to %dmp, past the content bottom at %dmp — a table row's rectangle running off the bottom of the sheet is the harm this assertion names", p, i, r.Y+r.H, bandBottom)
				}
			}
		}
		if checked < 30 {
			t.Fatalf("vacuity guard: only %d rect(s) were examined — an empty walk is trivially inside every band", checked)
		}
	})

	// AND IN THE BYTES. The page model is what internal/pdf serializes
	// verbatim, but the reviewer's finding was stated in bytes ("the
	// rendered document is byte-identical"), so it is answered in bytes.
	// Both directions, because presence alone would pass on a document
	// that also drew the untruncated rectangle somewhere.
	//
	// The two literals are the measured spelling of the fill operator
	// appendRectContentStream emits: "x y w h re f" in POINTS. 100 is the
	// truncated height (100,000mp); 152.544 is the untruncated one, and it
	// must appear nowhere in the document at all.
	t.Run("the truncated rectangle reaches the content stream", func(t *testing.T) {
		res, rerr := Render(tpl, Data(overTallRowFixtureData()), nil, testShippedFontSet())
		if rerr != nil {
			t.Fatalf("Render returned %T: %v", rerr, rerr)
		}
		doc := string(res.Bytes)
		for _, want := range []string{"10 20 60 100 re f", "70 20 60 100 re f"} {
			if !strings.Contains(doc, want) {
				t.Errorf("the content stream does not contain %q — the clipped row's cell fill, truncated to the content bottom", want)
			}
		}
		if strings.Contains(doc, "152.544") {
			t.Error("the content stream contains \"152.544\" — the clipped row's UNTRUNCATED height reached the document, so the row's rectangle is drawn past the bottom of the page")
		}
	})
}

// clippedLine is one physical line of the over-tall row, with the extent
// the vertical model gave it — read off the runs, never re-derived.
type clippedLine struct {
	index       int
	top, bottom geom.Length
}

// lineBottom returns line idx's own bottom, for a failure message that
// says WHICH side of the cut the line was on rather than only that it was
// on the wrong one.
func lineBottom(all []clippedLine, idx int) geom.Length {
	for _, l := range all {
		if l.index == idx {
			return l.bottom
		}
	}
	return -1
}

// --- AC4: a located Warning, on the bytes channel -------------------------

// TestClippedRowReportsALocatedWarningNamingTheRow is AC4.
//
// THREE OBSERVABLES: (i) a Warning with this code exists at all;
// (ii) it carries the ROW INDEX — data HEAD's *layout.OverflowError does
// not have at all, so this is new information and not a relabelling;
// (iii) it travels on Result.Diagnostics as a Warning, never wrapped in a
// *RenderError (AD-14, D-3.6.3 arm A).
func TestClippedRowReportsALocatedWarningNamingTheRow(t *testing.T) {
	tpl, err := ParseTemplate([]byte(overTallRowDoc()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, rerr := Render(tpl, Data(overTallRowFixtureData()), nil, testShippedFontSet())

	// Each observable is its OWN named subtest, so a deletion screen's
	// answer is a SET of names rather than one boolean for three
	// (D-000.85).

	// (iii) THE CHANNEL. A Warning on the bytes channel, not an error.
	t.Run("warning channel, not an error", func(t *testing.T) {
		if rerr != nil {
			t.Fatalf("Render returned %T: %v — AD-14 puts this diagnostic on Result.Diagnostics beside the bytes, never in a *RenderError", rerr, rerr)
		}
	})

	var found []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			found = append(found, d)
		}
	}

	// (i) PRESENCE, and EXACTLY ONE: one clipped group, one Warning.
	t.Run("a warning with this code exists, exactly once", func(t *testing.T) {
		if len(found) != 1 {
			t.Fatalf("Result.Diagnostics carries %d %s diagnostic(s); want exactly 1 (the document clips exactly one row). All diagnostics: %+v",
				len(found), DiagCodeTableRowClippedHeight, res.Diagnostics)
		}
		if found[0].Severity != SeverityWarning {
			t.Errorf("Severity = %v, want %v — the document rendered", found[0].Severity, SeverityWarning)
		}
		if found[0].ElementID != "e1" {
			t.Errorf("ElementID = %q, want %q (the table)", found[0].ElementID, "e1")
		}
	})

	// (ii) THE ROW INDEX, the epic's explicit requirement and the
	// observable most likely to ride along unwitnessed. Asserted
	// SEPARATELY from (i) so that formatting the message from the
	// element id alone reddens here and nowhere else.
	t.Run("the message names the row index", func(t *testing.T) {
		if len(found) != 1 {
			t.Skipf("no single %s diagnostic to read a row index from — see the sibling subtest", DiagCodeTableRowClippedHeight)
		}
		d := found[0]
		wantRow := fmt.Sprintf("row %d of the bound collection", clipFixtureRowIndex)
		if !strings.Contains(d.Message, wantRow) {
			t.Errorf("the message does not name the row: it must contain %q.\ngot: %s", wantRow, d.Message)
		}
		// ... with the two heights the author needs to act on it
		// (D-000.37, "executable by a human").
		for _, want := range []string{
			millipointsForDiag(clipFixtureRowHeight),
			millipointsForDiag(clipFixtureContentHeight),
		} {
			if !strings.Contains(d.Message, want) {
				t.Errorf("the message does not contain %q — a reader cannot act on 'too tall' without the row's height and the height it was measured against.\ngot: %s", want, d.Message)
			}
		}
	})
}

// --- AC5: the footer-alone-too-tall case, and the sentinel ----------------

// TestClippedGroupDiagnosticNamesTheRoleNeverTheSentinel is AC5's second
// observable, and it is asserted against the production message builder for
// all three group roles at once rather than against three documents.
//
// footerGroupIndex is -1: a WIRE VALUE, chosen so no real data row could
// collide with it. A message that printed it verbatim would put "row -1" in
// front of a human, which is the defect this test exists to prevent.
func TestClippedGroupDiagnosticNamesTheRoleNeverTheSentinel(t *testing.T) {
	for _, tc := range []struct {
		name    string
		key     layout.ItemGroupKey
		wantSub string
	}{
		{"data row", layout.ItemGroupKey{ElementID: "e1", Index: 7}, "row 7 of the bound collection"},
		{"header row", layout.ItemGroupKey{ElementID: "e1", IsHeader: true}, "the header row"},
		{"footer row", layout.ItemGroupKey{ElementID: "e1", Index: footerGroupIndex}, "the footer row"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := clippedRowDiagnostic(layout.TableRowClipped{
				ElementID: "e1", Key: tc.key,
				ItemHeight: 200_000, ContentHeight: 110_000, Page: 3,
			})
			if d.Code != DiagCodeTableRowClippedHeight {
				t.Errorf("Code = %q, want %q — all three group roles share one code (D-4.6.3: same thing happened to the document, same remedy)", d.Code, DiagCodeTableRowClippedHeight)
			}
			if d.Severity != SeverityWarning {
				t.Errorf("Severity = %v, want %v", d.Severity, SeverityWarning)
			}
			if !strings.Contains(d.Message, tc.wantSub) {
				t.Errorf("the message does not name the group as %q.\ngot: %s", tc.wantSub, d.Message)
			}
			if strings.Contains(d.Message, fmt.Sprintf("row %d", footerGroupIndex)) {
				t.Errorf("the message prints the footer's %d sentinel verbatim — that is a wire value reaching a reader.\ngot: %s", footerGroupIndex, d.Message)
			}
		})
	}
}

// --- AC6: termination, bounded -------------------------------------------

// TestOverTallGroupPaginationTerminatesWithinABound is AC6, following
// Story 4.5's select/time.After precedent. A hang is not a red — it is a
// stuck test whose failure arrives as a whole-package timeout with no name
// on it. The bound is what makes "it returns" an observable at all.
func TestOverTallGroupPaginationTerminatesWithinABound(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		data string
	}{
		{"single-line over-tall row", tooTallRowDoc(), multiRowTableData(1, -1)},
		{"multi-line over-tall row between ordinary ones", overTallRowDoc(), overTallRowFixtureData()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(tc.doc))
			if err != nil {
				t.Fatalf("ParseTemplate: %v", err)
			}
			type outcome struct {
				n   int
				err error
			}
			done := make(chan outcome, 1)
			go func() {
				res, rerr := Render(tpl, Data(tc.data), nil, testShippedFontSet())
				done <- outcome{len(res.Bytes), rerr}
			}()
			select {
			case got := <-done:
				if got.err != nil || got.n == 0 {
					t.Fatalf("Render returned (%d bytes, %v); the bound was met but the answer is wrong", got.n, got.err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Render did not return within 10s on an over-tall row — the clip must place the group and advance, never revisit it")
			}
		})
	}
}

// --- D-4.6.2's required tripwire -----------------------------------------

// TestAPresentItemGroupIsATableRowOrAKeepTogetherGroup is the tripwire
// D-4.6.2 requires, and it guards the one soft spot in Story 4.6's whole
// design.
//
// IT WAS RENAMED BY STORY 7.7, BECAUSE THE PROPERTY ITS OLD NAME ASSERTED
// IS NOW FALSE ON PURPOSE. It used to be
// TestAPresentItemGroupIsAlwaysATableRow, and it named the only two lawful
// ways out: "give the new grouping its own placement rule, or take the
// decision to widen the clip deliberately and update D-4.6.2." Story 7.7
// took the second, the engineering lead recorded the amendment in
// folio-mvp-decision-log.md's D-4.6.2 entry (AMENDED 2026-08-31), and this
// test now states the WIDENED invariant rather than the old one. The
// tripwire fired exactly as designed; it was not evaded.
//
// The clip is keyed on layout.ItemGroup.Present, because Kind cannot tell a
// table row from a plain line (measured: at 45cf812 an over-tall table row
// and an over-tall plain text element were byte-identical at the public
// API — both 0 bytes, both CONTENT_UNLAYOUTABLE, both Kind "line"). That
// key is CORRECT only while every present ItemGroup names something whose
// clipping D-4.6.2 has actually ruled on. Nothing in the type system stops
// a future story tagging some further element with a present ItemGroup, and
// the day one does, that element becomes silently CLIPPABLE instead of
// erroring, reversing D-2.6.1 without anyone deciding to.
//
// THE INVARIANT, AS AMENDED: in package folio8's own non-test sources, a
// present ItemGroup is a TABLE ROW or an AUTHOR-DECLARED KEEP-TOGETHER
// GROUP, and nothing else. It is constructed ONLY inside the three
// derivations named below — tableRectSource.chromeRowGroup and
// textRunSource.lineRowGroup, whose every arm is a table row (a header row,
// a data row, or the footer row), and keepTogetherIndex.keepTogetherGroup,
// whose single arm is an element carrying the author's own `keepTogether`
// tag (FR51). The build says so the day a FOURTH one appears.
//
// THE TWO POPULATIONS ARE NO LONGER CLIPPABLE ON THE SAME TERMS, and
// saying they were is what let a real defect through (Story 7.10, D-7.10.1
// / D-7.10.2). Leniency follows AUTHORSHIP, and pushed to its own
// conclusion that separates them rather than joining them: a table row's
// height is derived from DATA the author may never have seen and cannot
// fix, so it is clipped whatever its shape; a keep-together group exists
// because someone TYPED a tag, and a tag is never data-driven, so a group
// holding a template element that is by itself too tall for any window is
// REFUSED — the author declared an atomic block that fits nowhere and can
// dissolve it. Only the genuinely aggregate keep-together group — two or
// more elements, each fitting, the union not — is still clipped. The
// discriminator is carried as layout.ItemGroup.AuthorDeclared, which is
// set in exactly one of the three derivations below.
//
// SO THE SCAN GUARDS THAT FIELD TOO, on an allowlist of its own with ONE
// member. Guarding Present alone left the reversal this story fixed
// reachable from either side: a fourth derivation that sets Present WITHOUT
// setting AuthorDeclared gets a grouping clipped whatever its shape — Story
// 4.6's leniency, which is justified only by a row's height coming from data
// nobody can audit, handed to something the author typed — and one that sets
// AuthorDeclared makes its elements individually refusable. Neither shows up
// in a diff as anything more than a struct field, and both reverse a ruling.
func TestAPresentItemGroupIsATableRowOrAKeepTogetherGroup(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}

	// The three derivations that ARE the grouping — named as a closed
	// set, because that closure is the property. Two are the table-row
	// grouping (Story 4.3); the third is Story 7.7's author-declared
	// keep-together group, added here under D-4.6.2's 2026-08-31
	// amendment and not by quietly widening a whitelist.
	rowGroupDerivations := map[string]bool{
		"chromeRowGroup":    true,
		"lineRowGroup":      true,
		"keepTogetherGroup": true,
	}

	// AND THE NARROWER SET FOR THE DISCRIMINATOR ITSELF (Story 7.10).
	// AuthorDeclared is not a second spelling of Present: it is the field
	// that decides whether an over-tall group is CLIPPED or REFUSED, so it
	// has an allowlist of its own with exactly ONE member. The asymmetry
	// is the property. A future derivation that sets Present alone gets a
	// group that is clipped whatever its shape — Story 4.6's answer, given
	// to something that may be nothing like a table row — and one that
	// sets AuthorDeclared makes its elements individually refusable. Both
	// directions reverse a ruling (D-7.10.1 / D-7.10.2) and neither is
	// visible in a diff that only adds a struct literal, which is why the
	// build says so rather than a comment.
	authorDeclaredDerivations := map[string]bool{
		"keepTogetherGroup": true,
	}

	fset := token.NewFileSet()
	constructions := 0
	authorDeclarations := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(root, name)
		file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		for _, d := range file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				// SPELLING 2 (this story's reviewer, Finding 7):
				// `g.Present = true` on an already-declared value. It
				// never passes through a composite literal at all, so
				// the literal walk below cannot see it, and neither
				// does `go vet`. Flagged by the FIELD being written
				// rather than by the type of what it is written on:
				// resolving the receiver's type would need go/types,
				// and the honest trade is to OVER-approximate — any
				// `.Present = …` in this package's non-test sources is
				// reported, including one on some future unrelated
				// type. Over-approximating costs a comment on a false
				// positive; under-approximating loses the property
				// silently, which is the whole failure mode D-4.6.2
				// exists to prevent. Measured at Story 4.6: package
				// folio8's non-test sources contain no such assignment
				// at all, so the over-approximation costs nothing today.
				if as, ok := n.(*ast.AssignStmt); ok {
					for _, lhs := range as.Lhs {
						sel, ok := lhs.(*ast.SelectorExpr)
						if !ok {
							continue
						}
						switch sel.Sel.Name {
						case "Present":
							constructions++
							if !rowGroupDerivations[fd.Name.Name] {
								t.Errorf("%s: %s ASSIGNS to a .Present field.\n\n"+
									"This is the second spelling of the property D-4.6.2's tripwire guards, and it evades a composite-literal scan entirely — as does `layout.ItemGroup{Present: ok}` with a non-literal value. See this test's own doc comment for why 'grouped' meaning 'table row' is load-bearing.\n\n"+
									"If this is a legitimate new grouping, give it its own placement rule or widen the clip deliberately and update D-4.6.2. If it is a .Present on an unrelated type, this guard is deliberately over-approximating — say so here and add the function to %v only if it really is a row-group derivation.",
									fset.Position(sel.Pos()), fd.Name.Name, sortedNames(rowGroupDerivations))
							}
						case "AuthorDeclared":
							authorDeclarations++
							if !authorDeclaredDerivations[fd.Name.Name] {
								t.Errorf("%s: %s ASSIGNS to an .AuthorDeclared field.\n\n"+
									"AuthorDeclared decides whether an over-tall group is CLIPPED or REFUSED (Story 7.10, D-7.10.1/D-7.10.2). Setting it makes the group's own template elements individually refusable; leaving it unset on a NEW grouping makes that grouping clippable like a table row, which is Story 4.6's leniency handed to something whose height the author CAN fix.\n\n"+
									"Only %v may set it. If this really is a new author-declared grouping, say so in D-7.10.2 and add the function here; if it is an .AuthorDeclared on an unrelated type, this guard is deliberately over-approximating — say so at the site.",
									fset.Position(sel.Pos()), fd.Name.Name, sortedNames(authorDeclaredDerivations))
							}
						}
					}
				}

				lit, ok := n.(*ast.CompositeLit)
				if !ok || !isLayoutItemGroupType(lit.Type) {
					return true
				}
				// The two fields are checked INDEPENDENTLY, because a
				// literal can set either without the other and each one
				// carries its own ruling. In particular a literal that
				// sets AuthorDeclared but leaves Present alone would
				// have slipped past a scan gated on Present.
				if setsFieldNotFalse(lit, "AuthorDeclared") {
					authorDeclarations++
					if !authorDeclaredDerivations[fd.Name.Name] {
						t.Errorf("%s: %s constructs a layout.ItemGroup with AuthorDeclared set to something other than the literal false.\n\n"+
							"That field is Story 7.10's discriminator: it says the grouping is the AUTHOR's own declaration, so a template element of it that is by itself taller than a content window is REFUSED rather than clipped (D-7.10.1). Only %v may set it — a fourth derivation that does is claiming the author typed something, and a fourth that omits it is claiming the height came from data nobody can audit.\n\n"+
							"Whichever is intended, it is a decision to record in D-7.10.2, not a field to copy.",
							fset.Position(lit.Pos()), fd.Name.Name, sortedNames(authorDeclaredDerivations))
					}
				}
				if !setsFieldNotFalse(lit, "Present") {
					return true
				}
				constructions++
				if !rowGroupDerivations[fd.Name.Name] {
					t.Errorf("%s: %s constructs a layout.ItemGroup with Present set to something other than the literal false.\n\n"+
						"Story 4.6 clips an over-tall GROUP instead of erroring, and refuses an over-tall UNGROUPED item — the distinction D-4.6.2 rests on is that a table ROW's height comes from DATA the author never saw, while a font size and an image box are things the author typed (AD-13). That reasoning holds only while 'grouped' means 'table row'.\n\n"+
						"A present ItemGroup built anywhere but %v breaks it, and breaks it SILENTLY: the new element stops erroring and starts being truncated, reversing D-2.6.1 without anyone deciding to. Either give the new grouping its own placement rule, or take the decision to widen the clip deliberately and update D-4.6.2.",
						fset.Position(lit.Pos()), fd.Name.Name, sortedNames(rowGroupDerivations))
				}
				return true
			})
		}
	}

	// VACUITY GUARD (D-000.9), reading the walk's OWN count: a scan that
	// entered no files finds no violation and reports exactly the
	// all-clear a healthy one does.
	//
	// The floor sits well BELOW the six constructions measured at Story
	// 4.6 (three arms in each of the two derivations), and the reason is
	// the same one emptyWalkFloor states 300 lines above — a
	// methodology this file had applied to one of its two floors and not
	// the other (this story's reviewer, Finding 8).
	//
	// This test's subject is WHERE a present ItemGroup is constructed,
	// not HOW MANY there are. Merging any two of the arms is a
	// legitimate refactor that changes nothing about the property, and a
	// floor pinned to the measurement would meet it with a false red
	// wearing a vacuity message. The floor is ONE ARM PER DERIVATION —
	// the least that can be there while every derivation still exists —
	// so Story 7.7 raises it from two to three along with the third
	// derivation. The UPPER direction, a construction somewhere new, is
	// genuinely guarded by the t.Errorf above, which is where this
	// test's teeth actually are.
	const derivationFloor = 3
	if constructions < derivationFloor {
		t.Fatalf("vacuity guard: the scan found only %d present-ItemGroup construction(s) in package folio8's non-test sources; at least %d must exist while all three grouping derivations do (seven were measured at Story 7.7: three arms in each row-group derivation and one in keepTogetherGroup). A truncated walk is trivially clean",
			constructions, derivationFloor)
	}

	// THE SAME FLOOR FOR THE DISCRIMINATOR, on the same methodology: the
	// least that can be there while the one derivation that sets
	// AuthorDeclared still exists. Without it, DELETING the discriminator
	// — the exact regression D-7.10.1 is about, and one that would clip
	// every author-declared group again — leaves this scan reporting the
	// same all-clear a healthy one does.
	const authorDeclaredFloor = 1
	if authorDeclarations < authorDeclaredFloor {
		t.Fatalf("vacuity guard: the scan found %d AuthorDeclared construction(s) in package folio8's non-test sources; at least %d must exist while keepTogetherGroup does. Zero means either a truncated walk or a deleted discriminator, and the second silently returns every author-declared group to Story 4.6's clip (D-7.10.1)",
			authorDeclarations, authorDeclaredFloor)
	}
}

// isLayoutItemGroupType reports whether a composite literal's type is
// written as layout.ItemGroup — the only spelling package folio8 uses.
func isLayoutItemGroupType(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ItemGroup" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "layout"
}

// setsFieldNotFalse reports whether the literal sets `field` to anything
// that is not the literal `false`. Story 7.10 generalised it from a
// Present-only check to a named field, because AuthorDeclared needs exactly
// the same reading and for exactly the same reasons.
//
// A literal that leaves the field unset is the zero value — "not grouped",
// or "not the author's declaration" — and is not a construction of one at
// all, so it is skipped. So is an explicit `false`, which says the same
// thing louder.
//
// EVERYTHING ELSE COUNTS, and that is deliberate (Story 4.6's reviewer,
// Finding 7). The original required the value to be the identifier `true`
// exactly, so `layout.ItemGroup{Present: ok}` with a bool variable — or a
// call, or a field read — passed silently: an AST whitelist of one
// spelling rather than the property. A value the scan cannot evaluate is
// a value that MIGHT be true, and a maybe-grouped item is exactly the
// thing D-4.6.2 needs a human to look at.
func setsFieldNotFalse(lit *ast.CompositeLit, field string) bool {
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != field {
			continue
		}
		if val, ok := kv.Value.(*ast.Ident); ok && val.Name == "false" {
			return false
		}
		return true
	}
	return false
}

func sortedNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Small closed set; a simple insertion sort keeps this readable and
	// avoids pulling a package in for three strings.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
