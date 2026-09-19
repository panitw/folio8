package folio8

// Story 4.7, AC1-AC5: everything about the Customer Account Statement
// that is machine-checkable, asserted OFF THE PRODUCED PDF and written
// and passing BEFORE any digest was frozen (D-000.22's machine-checkable
// half is never deferrable; the ordering is Task 7's).
//
// D-000.21, and it is why this file is longer than a hash comparison:
// every assertion below reads the ARTIFACT — the page tree, the page
// content streams, and the characters recovered from the drawn glyphs
// through each face's OWN /ToUnicode CMap. Nothing here reads the input
// document, and nothing here asserts "a substitution occurred".
//
// ONE OBSERVABLE PER TEST FUNCTION, deliberately. D-000.85's deletion
// screen requires each observable to be separably witnessable: the
// story names a mutation per observable and names the NEIGHBOURING
// GREENS as the discrimination, and that report is only possible if a
// mutation can redden one function while leaving its siblings green.
// Folding two observables into one test would make that report
// unwriteable.
//
// A NOTE ON THE INSTRUMENT, because it is a correction rather than a
// choice. This story's task list says to reuse splitPageContentStreams,
// mpParseToUnicode and mpExtractRuns. splitPageContentStreams is reused
// unchanged. The other two are NOT USABLE HERE, and the reason is
// measured, not stylistic: mpParseToUnicode merges EVERY face's
// beginbfchar section into ONE cid -> text map, so on a document with
// more than one embedded face the CID spaces collide and the recovered
// text is garbage. Measured on this fixture before the substitution:
// "Customer: Ada Lovelace" came back as "ไustomerะบdaบศovelace". Every
// prior caller is single-face, which is why the defect has never shown.
// The replacements are NOT newly derived either — toUnicodeForResources
// and parseContentStreamRuns are the existing per-resource instruments
// shaped_fixture_test.go already uses, composed here rather than
// re-implemented.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// --- geometry pinned by hand from the template ------------------------
//
// Derived from statementTemplateJSON, not read back out of a render:
// A4 portrait 841.89pt tall, 36pt margins, a 54pt pageHeader band and a
// 34pt pageFooter band leave a 681.89pt content band; the table sits
// 54pt down it behind a 28pt header row.
const (
	// statementLineHeightMP is one 8pt Noto Sans line box, in
	// millipoints.
	statementLineHeightMP int64 = 12880
	// statementRowPaddingMP is the table style's 8pt top plus 8pt
	// bottom padding.
	statementRowPaddingMP int64 = 16000
	// statementSingleRowAdvanceMP and statementWrappedRowAdvanceMP are
	// the baseline-to-baseline advance across a one-line and a two-line
	// row.
	statementSingleRowAdvanceMP  int64 = statementLineHeightMP + statementRowPaddingMP
	statementWrappedRowAdvanceMP int64 = 2*statementLineHeightMP + statementRowPaddingMP

	// statementRowsOnFirstPage and statementRowsOnContinuationPage are
	// MEASURED (by rendering every collection length from 1 to 400 and
	// recording where the page count steps), and stated here as
	// independent literals so the partition assertion has something to
	// compare against that is not itself a render.
	//
	// Page 1 PLACES 19 rows and not 20 because the second row WRAPS to
	// two lines: it is a discriminating row, present in all four
	// documents, and it costs one line of the first page's budget.
	//
	// NINETEEN IS THE ROWS-PLACED FIGURE, NOT THE BAND EDGE, and the two
	// were confused (this story's review, Finding 5). The largest
	// collection that still fits in P pages is 18 + 22*(P-1), one lower,
	// because the sum footer aggregate reserves a row slot on the FINAL
	// page — so the bands are 1..18 / 85..106 / 415..436 / 1075..1096
	// while the partition read off the artifact is [19 22 22 ... k].
	// statement_fixture_test.go's header records both, measured at each
	// band edge; neither number is derivable from the other without the
	// aggregate's slot, which is why recording only one of them made the
	// two files read as a contradiction.
	statementRowsOnFirstPage        = 19
	statementRowsOnContinuationPage = 22
)

// column origins, in millipoints: the left margin plus the running sum
// of the preceding column widths, plus the 3pt left padding.
const (
	statementDateColumnXMP        int64 = 39000
	statementReferenceColumnXMP   int64 = 99000
	statementDescriptionColumnXMP int64 = 165000
	statementNoteColumnXMP        int64 = 335000
	statementAmountColumnRightMP  int64 = 447000 // 36 + 60 + 66 + 170 + 34 + 84 - 3
	// statementNoteColumnWidthMP is the Note column's DECLARED width
	// (34pt), so the Amount cell's left bound can be derived from the
	// template's own geometry rather than eyeballed.
	statementNoteColumnWidthMP int64 = 34000
)

// statementColumnLabels is the header-label SET, in column order, as the
// template declares it. Restated here as an independent literal.
var statementColumnLabels = []string{"Date", "Reference", "Description", "Note", "Amount"}

const statementConfidentialityText = "Confidential - for the named account holder only. Generated "

// statementDrawnRun is one BT/ET block of one page: which face resource
// drew it, where, the text it recovers to through that face's own
// /ToUnicode CMap, and the TJ adjustments it carried.
type statementDrawnRun struct {
	Resource      string
	Text          string
	XMP, YMP      int64
	CIDs          []uint16
	Adjustments   []int64
	UsedTJ        bool
	NonZeroAdjust bool
}

// renderStatement renders one of the four documents through the public
// Render path, exactly as the fixture's own capture function does.
func renderStatement(t *testing.T, f statementFixture) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(statementTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate(statementTemplateJSON): %v", err)
	}
	res, rerr := Render(tpl, Data(statementDataJSON(f.rows)), Params(statementParamsJSON), testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render(%s): %v", f.slug, rerr)
	}
	// AD-14: a diagnostic travels alongside the bytes and is asserted.
	// A golden that silently carries an unasserted Warning is a
	// recording of a document nobody read.
	if len(res.Diagnostics) != 0 {
		t.Fatalf("%s: Render produced %d diagnostic(s), want none: %+v", f.slug, len(res.Diagnostics), res.Diagnostics)
	}
	if len(res.Bytes) == 0 {
		t.Fatalf("%s: Render produced no bytes", f.slug)
	}
	return res.Bytes
}

// statementPageRuns decodes every page's text runs through the
// PER-RESOURCE /ToUnicode CMaps.
func statementPageRuns(t *testing.T, b []byte) [][]statementDrawnRun {
	t.Helper()
	cmaps := toUnicodeForResources(t, b)
	streams := splitPageContentStreams(t, b)
	out := make([][]statementDrawnRun, len(streams))
	for i, s := range streams {
		for _, r := range parseContentStreamRuns(t, []byte(s)) {
			cmap, ok := cmaps[r.Resource]
			if !ok {
				t.Fatalf("page %d: a run selects font resource %q, which resolves to no /ToUnicode CMap", i+1, r.Resource)
			}
			var sb strings.Builder
			for _, cid := range r.CIDs {
				txt, known := cmap[cid]
				if !known {
					t.Fatalf("page %d: resource %q emits CID %d, which its own /ToUnicode CMap does not map — the text is unextractable", i+1, r.Resource, cid)
				}
				sb.WriteString(txt)
			}
			out[i] = append(out[i], statementDrawnRun{
				Resource: r.Resource, Text: sb.String(),
				XMP: r.OriginXMilli, YMP: r.OriginYMilli,
				CIDs: r.CIDs, Adjustments: r.Adjustments,
				UsedTJ: r.UsedTJ, NonZeroAdjust: r.NonZeroAdjust,
			})
		}
		if len(out[i]) == 0 {
			t.Fatalf("presence precondition: page %d carries no text run at all", i+1)
		}
	}
	return out
}

// statementPageTexts flattens one page's runs to their recovered text.
func statementPageTexts(runs []statementDrawnRun) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.Text
	}
	return out
}

func statementContains(runs []statementDrawnRun, want string) bool {
	for _, r := range runs {
		if r.Text == want {
			return true
		}
	}
	return false
}

var statementRefRE = regexp.MustCompile(`^TXN-(\d{4})$`)

// statementRowIndicesOnPage returns the transaction ordinals whose
// Reference cell is drawn on this page, in emission order — the
// {page -> row indices} set D-000.68/D-000.83 requires instead of a
// page count alone, read off the artifact.
func statementRowIndicesOnPage(t *testing.T, runs []statementDrawnRun) []int {
	t.Helper()
	var out []int
	for _, r := range runs {
		if r.XMP != statementReferenceColumnXMP {
			continue
		}
		m := statementRefRE.FindStringSubmatch(r.Text)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("unparseable reference %q", r.Text)
		}
		out = append(out, n)
	}
	return out
}

// statementExpectedPartition is the {page -> row indices} partition the
// measured row capacities imply, computed WITHOUT rendering anything.
func statementExpectedPartition(n int) [][]int {
	var out [][]int
	next := 1
	for next <= n {
		cap := statementRowsOnContinuationPage
		if len(out) == 0 {
			cap = statementRowsOnFirstPage
		}
		var page []int
		for i := 0; i < cap && next <= n; i++ {
			page = append(page, next)
			next++
		}
		out = append(out, page)
	}
	if len(out) == 0 {
		out = append(out, nil)
	}
	return out
}

// -----------------------------------------------------------------------
// AC1, observable (i): each document's produced page count equals its
// declared count, counted off the produced artifact — and the
// {page -> row indices} partition matches, because two different wrong
// arrangements produce the same count (D-000.68/D-000.83).
// -----------------------------------------------------------------------

func TestStatementDocumentsRenderTheirDeclaredPageCount(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			b := renderStatement(t, f)

			// Counted three independent ways: the /Type /Page object
			// definitions, the resolved /Kids array, and the number of
			// page content streams reached through each page object's
			// own /Contents reference.
			defs := bytes.Count(b, []byte("/Type /Page "))
			if defs != f.pages {
				t.Errorf("%s: the produced document carries %d /Type /Page object definitions, want %d", f.slug, defs, f.pages)
			}
			streams := splitPageContentStreams(t, b)
			if len(streams) != f.pages {
				t.Fatalf("%s: the page tree resolves to %d page(s), want %d", f.slug, len(streams), f.pages)
			}

			pages := statementPageRuns(t, b)
			var got [][]int
			for _, runs := range pages {
				got = append(got, statementRowIndicesOnPage(t, runs))
			}
			want := statementExpectedPartition(f.rows)
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("%s: the {page -> row indices} partition read off the artifact is\n  %v\nbut the measured row capacities (%d on page 1, %d thereafter) imply\n  %v",
					f.slug, got, statementRowsOnFirstPage, statementRowsOnContinuationPage, want)
			}
			var placed int
			for _, p := range got {
				placed += len(p)
			}
			if placed != f.rows {
				t.Errorf("%s: %d of the collection's %d rows reached a page", f.slug, placed, f.rows)
			}
			t.Logf("%s: %d pages, %d rows placed, partition sizes %v", f.slug, len(streams), placed, func() []int {
				var s []int
				for _, p := range got {
					s = append(s, len(p))
				}
				return s
			}())
		})
	}
}

// -----------------------------------------------------------------------
// AC1, observable (ii): the page count FOLLOWS THE LENGTH OF THE BOUND
// COLLECTION — removing rows removes pages.
//
// This observable exists because "renders at N pages" is ACCIDENTALLY
// TRUE by construction for any geometrically-built document, which is
// exactly how fixtures/page-count-* works and exactly why its green
// says nothing about a table (D-000.86 part (a)). The accidental cause
// is never introduced here — the fixture is data-driven from the start
// — and this test is the standing demonstration that observable (i)
// alone is not evidence.
//
// It is a RELATIVE property on purpose: it survives a uniform shift in
// rows-per-page, which is what makes it the discrimination for
// observable (i)'s deletion rather than a second copy of it.
// -----------------------------------------------------------------------

func TestStatementPageCountFollowsTheBoundCollectionLength(t *testing.T) {
	tpl, err := ParseTemplate([]byte(statementTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pagesFor := func(n int) int {
		res, rerr := Render(tpl, Data(statementDataJSON(n)), Params(statementParamsJSON), testShippedFontSet())
		if rerr != nil {
			t.Fatalf("Render(n=%d): %v", n, rerr)
		}
		return len(splitPageContentStreams(t, res.Bytes))
	}

	// Strictly increasing across the four declared lengths, and each
	// step is caused by rows and nothing else — the template, the params
	// document and the geometry are identical in every call.
	prevPages, prevRows := 0, 0
	for _, f := range statementFixtures {
		got := pagesFor(f.rows)
		if got <= prevPages {
			t.Errorf("collection length %d produced %d page(s); length %d produced %d — a longer collection must not produce fewer or equal pages", f.rows, got, prevRows, prevPages)
		}
		prevPages, prevRows = got, f.rows
	}

	// And the property in the direction the AC states it: REMOVING rows
	// removes pages.
	//
	// THE ROW COUNT REMOVED IS MEASURED FROM THE RENDER, not taken from
	// statementRowsOnContinuationPage, and that is deliberate. This
	// observable's whole job is to be a RELATIVE property that SURVIVES
	// a uniform shift in rows-per-page — that survival is what makes it
	// the discrimination for observable (i)'s deletion rather than a
	// second copy of it. Pinning the literal 22 here destroyed exactly
	// that: measured by the deletion screen, changing the paginator's
	// content window reddened this test as well as (i), so the two
	// observables were not separably witnessable. Deriving the capacity
	// from the artifact restores the separation.
	// The number of rows removed is MEASURED FROM THE ARTIFACT — the
	// rows the last two pages actually carried — and the assertion is
	// that the page count STRICTLY DECREASES, not that it drops by an
	// exact amount.
	//
	// Two pages rather than one, and "strictly fewer" rather than
	// "exactly one fewer", because the footer aggregate's own
	// orphan-avoidance (Story 4.5) can hold the last page open for the
	// total alone: measured, removing only the last page's 10 rows from
	// statement-5 left the document at 5 pages, with the final page
	// carrying the aggregate and no rows. That is CORRECT behaviour, and
	// an assertion that called it a failure would be pinning the
	// aggregate's placement rule inside an observable about collection
	// length.
	lastTwoPageRows := func(n int) int {
		res, rerr := Render(tpl, Data(statementDataJSON(n)), Params(statementParamsJSON), testShippedFontSet())
		if rerr != nil {
			t.Fatalf("Render(n=%d): %v", n, rerr)
		}
		pages := statementPageRuns(t, res.Bytes)
		total := 0
		for _, runs := range pages[max(0, len(pages)-2):] {
			total += len(statementRowIndicesOnPage(t, runs))
		}
		return total
	}
	for _, f := range statementFixtures {
		if f.pages < 2 {
			continue
		}
		before := pagesFor(f.rows)
		removed := lastTwoPageRows(f.rows)
		if removed <= 0 {
			t.Fatalf("%s: the last two pages carry no rows, so removing them could not remove a page", f.slug)
		}
		got := pagesFor(f.rows - removed)
		if got >= before {
			t.Errorf("%s: removing the %d row(s) the last two pages carried (%d -> %d) produced %d page(s), which is not FEWER than %d — the page count does not follow the collection length",
				f.slug, removed, f.rows, f.rows-removed, got, before)
		}
		t.Logf("%s: %d rows -> %d pages; %d rows -> %d pages (removed the %d rows the last two pages carried)", f.slug, f.rows, before, f.rows-removed, got, removed)
	}
}

// -----------------------------------------------------------------------
// AC2, observable (i): the five column header labels appear on EVERY
// page.
//
// D-000.86 part (a), and it is why statement-1 is excluded by name:
// on a one-page document "on every page" is satisfied by one page, so
// including it would let the assertion pass on a renderer that had
// stopped repeating the header entirely. The loop runs over the three
// MULTI-PAGE documents; statement-1's vacuity is recorded below rather
// than hidden.
// -----------------------------------------------------------------------

func TestStatementColumnHeaderLabelsAppearOnEveryPage(t *testing.T) {
	multiPage := 0
	for _, f := range statementFixtures {
		if f.pages < 2 {
			t.Logf("%s is EXCLUDED from this assertion: it has one page, so \"the header appears on every page\" is accidentally true of it and its green would say nothing (D-000.86 part (a))", f.slug)
			continue
		}
		multiPage++
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			if len(pages) < 2 {
				t.Fatalf("presence precondition: %s resolved to %d page(s) — this assertion needs a continuation page to mean anything", f.slug, len(pages))
			}
			for p, runs := range pages {
				for _, label := range statementColumnLabels {
					if !statementContains(runs, label) {
						t.Errorf("%s page %d does not draw the column header label %q — the table header does not repeat on continuation pages", f.slug, p+1, label)
					}
				}
			}
		})
	}
	if multiPage == 0 {
		t.Fatal("vacuity guard: no multi-page statement document was examined, so nothing about header repetition was asserted")
	}
	t.Logf("header-repeat witness: %d multi-page document(s) examined, all five labels %v present on every page of each", multiPage, statementColumnLabels)
}

// -----------------------------------------------------------------------
// AC2, observable (ii): the FIRST and LAST transaction rows' cell text
// appears with the values the data declares, formatted by the declared
// locale ("en" -> "#,##0.00" groups thousands).
// -----------------------------------------------------------------------

func TestStatementFirstAndLastRowsRenderTheDeclaredValues(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			rows := statementRows(f.rows)

			assertRow := func(label string, runs []statementDrawnRun, r statementRow) {
				// The Reference cell locates the row; every other cell
				// of that row shares its baseline.
				var y int64
				var found bool
				for _, run := range runs {
					if run.XMP == statementReferenceColumnXMP && run.Text == r.Ref {
						y, found = run.YMP, true
						break
					}
				}
				if !found {
					t.Fatalf("%s row: no Reference cell drawing %q was found on its page", label, r.Ref)
					return
				}
				// Cells are located by the column's own x-range, read
				// off the template's declared widths.
				cell := func(lo, hi int64) string {
					var sb strings.Builder
					for _, run := range runs {
						if run.YMP == y && run.XMP >= lo && run.XMP < hi {
							sb.WriteString(run.Text)
						}
					}
					return sb.String()
				}
				if got := cell(statementDateColumnXMP, statementReferenceColumnXMP); got != r.Date {
					t.Errorf("%s row Date cell drew %q, want %q", label, got, r.Date)
				}
				// The Description cell may be split across face runs —
				// that is the point of the three-script row — so it is
				// compared as the CONCATENATION of the runs the cell's
				// x-range carries on this baseline.
				//
				// THE INVARIANT THIS ASSERTION DEPENDS ON, asserted
				// rather than special-cased. This story's review, Finding
				// 14: a branch here rewrote the expectation for the
				// WRAPPING row, and assertRow is only ever called with
				// rows[0] (always TXN-0001) and rows[len(rows)-1] — the
				// wrapping row is TXN-0002 and can never be either, so
				// the branch was dead and read as though a first-or-last
				// row might wrap. What the assertion actually needs is
				// that the row it is given does NOT wrap, and that is now
				// stated: a future reordering of
				// statementDiscriminatingRows fails here, loudly, instead
				// of silently comparing a first line against a whole
				// cell.
				if r.Description == statementWrappedLine1+" "+statementWrappedLine2 {
					t.Fatalf("%s row (%s) is the WRAPPING row. This assertion compares the whole declared Description against one baseline's runs and cannot express a two-line cell; AC3(ii)'s own test owns the wrapping row. Reordering statementDiscriminatingRows so the wrapping row is first or last needs this assertion rewritten, not special-cased.", label, r.Ref)
				}
				desc := cell(statementDescriptionColumnXMP, statementNoteColumnXMP)
				if got := desc; got != r.Description {
					t.Errorf("%s row Description cell drew %q, want %q", label, got, r.Description)
				}
				wantAmount := statementFormatMinor(statementAmountMinor(r.Amount))
				// The Amount cell's x-range begins at the NOTE COLUMN'S
				// DECLARED RIGHT EDGE (34pt wide, so +34000 mp), not at an
				// eyeballed offset. This story's review, Finding 16: the
				// bound was +31000, which sat 3,000 mp INSIDE the Note
				// column's tail. It was harmless only because Note cells
				// are left-aligned and their runs begin at the column
				// origin; changing that column to right- or centre-aligned
				// would have folded Note text into the Amount comparison.
				amount := cell(statementNoteColumnXMP+statementNoteColumnWidthMP, statementAmountColumnRightMP+1)
				if amount != wantAmount {
					t.Errorf("%s row Amount cell drew %q, want %q (locale \"en\", format \"#,##0.00\")", label, amount, wantAmount)
				}
			}

			assertRow("first", pages[0], rows[0])
			assertRow("last", pages[len(pages)-1], rows[len(rows)-1])
			t.Logf("%s: first row %s, last row %s", f.slug, rows[0].Ref, rows[len(rows)-1].Ref)
		})
	}
}

// -----------------------------------------------------------------------
// AC2, observable (iii): the sum footer appears EXACTLY ONCE, on the
// LAST page only, and equals a total computed independently of the
// renderer — by exact integer arithmetic on the decimal literals
// (AD-23: a float check would inherit the bug it exists to catch).
//
// D-000.86 part (a): the occurrence count is taken over EVERY page, and
// the same assertion is then re-run with page 1 EXCLUDED, because an
// extraction that only ever read page 1 would satisfy "exactly once"
// while saying nothing.
// -----------------------------------------------------------------------

func TestStatementSumFooterAppearsExactlyOnceOnTheLastPage(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			want := statementFormatMinor(statementExpectedSumMinor(f.rows))

			// Anchoring (D-000.68): the total must not coincide with any
			// data cell's own rendered text, or "found it" proves nothing.
			for _, r := range statementRows(f.rows) {
				if statementFormatMinor(statementAmountMinor(r.Amount)) == want {
					t.Fatalf("anchoring: the footer total %q equals row %s's own amount — this assertion could not tell the aggregate from a data cell", want, r.Ref)
				}
			}

			count, onPages := 0, []int{}
			for p, runs := range pages {
				for _, run := range runs {
					if run.Text == want {
						count++
						onPages = append(onPages, p+1)
					}
				}
			}
			if count != 1 {
				t.Fatalf("%s: the footer total %q is drawn %d time(s), on page(s) %v — want exactly once", f.slug, want, count, onPages)
			}
			if onPages[0] != len(pages) {
				t.Errorf("%s: the footer total %q is drawn on page %d, want the LAST page (%d)", f.slug, want, onPages[0], len(pages))
			}
			if run := pages[len(pages)-1]; !statementContains(run, want) {
				t.Errorf("%s: the last page does not draw the footer total %q", f.slug, want)
			}

			// The same count, taken with page 1 excluded.
			if len(pages) > 1 {
				tail := 0
				for _, runs := range pages[1:] {
					for _, run := range runs {
						if run.Text == want {
							tail++
						}
					}
				}
				if tail != 1 {
					t.Errorf("%s: with page 1 excluded the footer total %q occurs %d time(s), want 1 — the extraction is not reading every page", f.slug, want, tail)
				}
			}
			t.Logf("%s: footer total %q drawn exactly once, on page %d of %d", f.slug, want, onPages[0], len(pages))
		})
	}
}

// -----------------------------------------------------------------------
// AC2, observable (iv): the page footer's THREE components — the
// confidentiality wording, the params-supplied date and "Page X of Y" —
// appear on EVERY page, with X and Y correct.
// -----------------------------------------------------------------------

func TestStatementPageFooterCarriesItsThreeComponentsOnEveryPage(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			wantConfidential := statementConfidentialityText + statementGeneratedDate + "."
			for p, runs := range pages {
				if !statementContains(runs, wantConfidential) {
					t.Errorf("%s page %d does not draw the page-footer line %q", f.slug, p+1, wantConfidential)
				}
				wantPageNo := "Page " + itoaForTest(int64(p+1)) + " of " + itoaForTest(int64(len(pages)))
				if !statementContains(runs, wantPageNo) {
					t.Errorf("%s page %d does not draw %q anywhere in its content stream; it drew %q", f.slug, p+1, wantPageNo, statementPageTexts(runs))
				}
			}
			t.Logf("%s: confidentiality wording, supplied date %s and Page X of %d present on all %d page(s)", f.slug, statementGeneratedDate, len(pages), len(pages))
		})
	}
}

// -----------------------------------------------------------------------
// AC3, observable (i): at least one table ROW draws glyphs from all
// three shipped faces under ONE fallback stack.
//
// D-4.5.5: measured at this story's baseline, NO test and NO fixture in
// the repository put all three shipped scripts inside a table — the
// table suites use a single-face fontFamily per subtest. This is new
// coverage with nothing to inherit.
// -----------------------------------------------------------------------

func TestStatementRowDrawsAllThreeShippedFaces(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			// Row 1 is the three-script row, and it is on page 1 in
			// every document because the four data documents share a
			// prefix.
			var y int64 = -1
			for _, run := range pages[0] {
				if run.XMP == statementReferenceColumnXMP && run.Text == "TXN-0001" {
					y = run.YMP
				}
			}
			if y < 0 {
				t.Fatal("presence precondition: the three-script row (TXN-0001) is not on page 1")
			}
			// Restricted to the DESCRIPTION COLUMN'S OWN X-RANGE, not to
			// the row's baseline. This story's review, Finding 8: the
			// faces map was accumulated over every run sharing row 1's
			// baseline, across all five columns, so the assertion
			// implemented "three faces in one ROW" while the claim
			// recorded here, in statement_fixture_test.go and in all four
			// READMEs is "three faces in ONE CELL under ONE fallback
			// stack". A future fixture edit that moved the Thai into the
			// Note column would have kept this test green while
			// falsifying the sentence — and one cell is what makes this
			// genuinely new coverage, since the table suites use a
			// single-face fontFamily per subtest.
			faces := map[string]string{}
			for _, run := range pages[0] {
				if run.YMP != y || run.XMP < statementDescriptionColumnXMP || run.XMP >= statementNoteColumnXMP {
					continue
				}
				if strings.TrimSpace(run.Text) == "" {
					continue
				}
				faces[run.Resource] = faces[run.Resource] + run.Text
			}
			for _, want := range []string{"NotoSans", "NotoSansThai", "NotoSansSC"} {
				if _, ok := faces[want]; !ok {
					t.Errorf("%s: the three-script row's DESCRIPTION CELL draws no glyph from face resource %q — it drew from %v", f.slug, want, faces)
				}
			}
			t.Logf("%s: three-script row's Description cell draws from %d faces: %v", f.slug, len(faces), faces)
		})
	}
}

// -----------------------------------------------------------------------
// AC3, observable (ii): at least one cell occupies MORE THAN ONE LINE
// and its row's height grew accordingly.
//
// D-4.5.5: a cell one physical line longer than its column would not
// discriminate a wrong break. The wrap point here falls BETWEEN TWO
// WORDS and is named: "...retainer for the" / "regional office". A
// renderer that broke one character early or late fails on the text,
// not merely on the line count.
// -----------------------------------------------------------------------

const (
	statementWrappedLine1 = "Quarterly maintenance retainer for the"
	statementWrappedLine2 = "regional office"
)

func TestStatementDescriptionCellWrapsAndGrowsItsRow(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			rowY := map[string]int64{}
			for _, run := range pages[0] {
				if run.XMP == statementReferenceColumnXMP {
					rowY[run.Text] = run.YMP
				}
			}
			y1, ok1 := rowY["TXN-0001"]
			y2, ok2 := rowY["TXN-0002"]
			y3, ok3 := rowY["TXN-0003"]
			if !ok1 || !ok2 || !ok3 {
				t.Fatal("presence precondition: rows 1-3 are not all on page 1")
			}

			var lines []string
			for _, run := range pages[0] {
				if run.XMP == statementDescriptionColumnXMP && (run.YMP == y2 || run.YMP == y2-statementLineHeightMP) {
					lines = append(lines, run.Text)
				}
			}
			if len(lines) != 2 {
				t.Fatalf("%s: the wrapping Description cell occupies %d line(s) %q, want exactly 2", f.slug, len(lines), lines)
			}
			if lines[0] != statementWrappedLine1 || lines[1] != statementWrappedLine2 {
				t.Errorf("%s: the wrapping Description cell broke as %q / %q, want %q / %q — the wrap point must fall between \"the\" and \"regional\"",
					f.slug, lines[0], lines[1], statementWrappedLine1, statementWrappedLine2)
			}

			// The row GREW: the advance across the wrapped row is one
			// line box taller than the advance across a single-line row.
			single := y1 - y2
			wrapped := y2 - y3
			if single != statementSingleRowAdvanceMP {
				t.Errorf("%s: the single-line row advance is %d mp, want %d (8pt line box %d + %d padding)", f.slug, single, statementSingleRowAdvanceMP, statementLineHeightMP, statementRowPaddingMP)
			}
			if wrapped != statementWrappedRowAdvanceMP {
				t.Errorf("%s: the WRAPPED row advance is %d mp, want %d — the row height did not grow with its tallest cell", f.slug, wrapped, statementWrappedRowAdvanceMP)
			}
			t.Logf("%s: wrap point %q | %q; single-row advance %d mp, wrapped-row advance %d mp", f.slug, lines[0], lines[1], single, wrapped)
		})
	}
}

// -----------------------------------------------------------------------
// AC3, observable (iii): the logo image is embedded ONCE and referenced
// from EVERY page that shows it.
//
// D-000.86 part (a), MEASURED AND DECLARED RATHER THAN IMPLIED. "An
// image XObject is present" is accidentally true of any document with
// an image — fixtures/image-embed already proves it. Before writing
// this assertion a throwaway two-page document carrying a header image
// was rendered at HEAD (df8cbcc) and its image XObject DEFINITIONS were
// counted: the answer was ONE, referenced by object number from both
// pages' /Resources dictionaries, with one `Do` operator per page. So
// this AC PINS AN EXISTING PROPERTY AT A NEW SCALE (50 pages, 50
// references, one definition) — it does not create one, and saying so
// is the honest form.
// -----------------------------------------------------------------------

func TestStatementLogoIsDefinedOnceAndReferencedFromEveryPage(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			b := renderStatement(t, f)
			if defs := bytes.Count(b, []byte("/Subtype /Image")); defs != 1 {
				t.Errorf("%s: the produced document carries %d image XObject definition(s), want exactly 1 — the logo is embedded once and shared", f.slug, defs)
			}
			// The page count is measured, not asserted, here: AC1(i)
			// owns "the document has N pages", and coupling this
			// observable to that literal made the paginator mutation
			// redden both (measured by the deletion screen). What this
			// observable owns is ONE DEFINITION, MANY REFERENCES —
			// relative to however many pages there are.
			streams := splitPageContentStreams(t, b)
			if len(streams) == 0 {
				t.Fatalf("%s: resolved no pages at all", f.slug)
			}
			want := "/" + statementLogoAssetKey + " Do"
			for p, s := range streams {
				if n := strings.Count(s, want); n != 1 {
					t.Errorf("%s page %d draws the logo XObject %d time(s), want exactly 1", f.slug, p+1, n)
				}
			}
			// And every page's own resource dictionary names it, or the
			// `Do` above would be an unresolvable name.
			resources := bytes.Count(b, []byte("/XObject << /"+statementLogoAssetKey))
			if resources != len(streams) {
				t.Errorf("%s: %d page object(s) name the logo in their /Resources /XObject dictionary, want %d — one per page of this render", f.slug, resources, len(streams))
			}
			t.Logf("%s: 1 image XObject definition, %d page(s), %d page references", f.slug, len(streams), resources)
		})
	}
}

// -----------------------------------------------------------------------
// AC4: the generated date arrives through `params` and is never read
// from a clock.
//
// The clock half is discharged by an existing STATIC guard and is
// declared as such rather than counted as an observable:
// lint/internal/rules/forbiddenimports.go bans "time" under internal/
// and in _test.go files, red-proofed by story34_forbiddenimports_test.go.
//
// D-000.86 part (a) is built into the fixture, not asserted about it:
// the data document carries no date field beyond transaction dates and
// the statement period, all of which fall in 2026-07, so the params
// value 2026-08-27 appears NOWHERE ELSE. That is checked below before
// anything is concluded from finding it.
// -----------------------------------------------------------------------

func TestStatementGeneratedDateResolvesFromParams(t *testing.T) {
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			if strings.Contains(statementDataJSON(f.rows), statementGeneratedDate) {
				t.Fatalf("%s: the DATA document contains %q, so finding that string in the render would not prove it came from params", f.slug, statementGeneratedDate)
			}

			tpl, err := ParseTemplate([]byte(statementTemplateJSON))
			if err != nil {
				t.Fatalf("ParseTemplate: %v", err)
			}
			render := func(params string) [][]statementDrawnRun {
				res, rerr := Render(tpl, Data(statementDataJSON(f.rows)), Params(params), testShippedFontSet())
				if rerr != nil {
					t.Fatalf("Render: %v", rerr)
				}
				return statementPageRuns(t, res.Bytes)
			}

			const otherDate = "2031-01-09"
			otherParams := `{
  "generatedDate": "` + otherDate + `"
}
`
			base := render(statementParamsJSON)
			swapped := render(otherParams)

			if len(base) != len(swapped) {
				t.Fatalf("%s: swapping the params value changed the page count (%d -> %d)", f.slug, len(base), len(swapped))
			}
			// The subject is the RUN THAT CARRIES THE DATE, located by
			// the date itself. It is deliberately NOT the whole
			// confidentiality sentence: measured by the deletion screen,
			// asserting the full literal here made AC2(iv)'s own
			// mutation (dropping the confidentiality wording) redden
			// this test too, so the two observables were not separably
			// witnessable. AC2(iv) owns the wording; this observable
			// owns the date's PROVENANCE.
			carries := func(runs []statementDrawnRun, date string) int {
				n := 0
				for _, r := range runs {
					if strings.Contains(r.Text, date) {
						n++
					}
				}
				return n
			}
			for p := range base {
				if carries(base[p], statementGeneratedDate) != 1 {
					t.Errorf("%s page %d draws the params date %q %d time(s), want exactly 1", f.slug, p+1, statementGeneratedDate, carries(base[p], statementGeneratedDate))
				}
				if carries(swapped[p], otherDate) != 1 {
					t.Errorf("%s page %d does not draw the SWAPPED params date %q exactly once — the date does not resolve from params", f.slug, p+1, otherDate)
				}
				if carries(swapped[p], statementGeneratedDate) != 0 {
					t.Errorf("%s page %d still draws the ORIGINAL params date %q after the swap", f.slug, p+1, statementGeneratedDate)
				}
				// ...and NOTHING ELSE changed: every other run is
				// identical, in order.
				a, bb := statementPageTexts(base[p]), statementPageTexts(swapped[p])
				if len(a) != len(bb) {
					t.Fatalf("%s page %d: the swap changed the run count (%d -> %d)", f.slug, p+1, len(a), len(bb))
				}
				diffs := 0
				for i := range a {
					if a[i] != bb[i] {
						diffs++
						if !strings.Contains(a[i], statementGeneratedDate) || !strings.Contains(bb[i], otherDate) {
							t.Errorf("%s page %d: swapping the params date changed run %d from %q to %q — it must change the date text and nothing else", f.slug, p+1, i, a[i], bb[i])
						}
					}
				}
				if diffs != 1 {
					t.Errorf("%s page %d: swapping the params date changed %d run(s), want exactly 1", f.slug, p+1, diffs)
				}
			}
			t.Logf("%s: the rendered date's provenance is params (%s -> %s), across %d page(s)", f.slug, statementGeneratedDate, otherDate, len(base))
		})
	}
}

// -----------------------------------------------------------------------
// AC5, observable (i): the Thai cell's line breaks match the FROZEN
// expected-break fixture.
//
// This is a COMPARISON against fixtures/expected-breaks/expected_breaks.json
// — S4, the frozen authority Thai break correctness is judged against
// for the life of the project — and NOT a restatement of the label in
// Go. The engineering lead ruled reuse over minting on exactly that
// ground: minting new strings would create a SECOND signed Thai-break
// corpus, and two signed authorities over the same rules can disagree.
//
// D-4.5.5 / D-000.86 part (a), and this AC is VACUOUS without it: a Thai
// cell that never wraps compares zero breaks and passes. The subject is
// thai-001, "ประเทศไทย", whose label carries an interior seam at rune 6,
// and the Note column's 34pt width (28pt usable) was chosen so the whole
// string CANNOT fit on one line while its first labelled word CAN. The
// column width is the knob; the string set is frozen.
//
// thai-007, thai-008 and thai-009 are RECORDED DIVERGENCES in
// break-signoff.json — the human label and the engine deliberately
// disagree — and are deliberately not used.
// -----------------------------------------------------------------------

const statementThaiBreakItemID = "thai-001"

type frozenBreakItem struct {
	ID             string   `json:"id"`
	Text           string   `json:"text"`
	Words          []string `json:"words"`
	ExpectedBreaks []int    `json:"expectedBreaks"`
	DeclaredAtomic bool     `json:"declaredAtomic"`
}

func statementFrozenBreakItem(t *testing.T, id string) frozenBreakItem {
	t.Helper()
	path := filepath.Join(repoRootFromTest(t), "fixtures", "expected-breaks", "expected_breaks.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Items []frozenBreakItem `json:"items"`
	}
	if uerr := json.Unmarshal(raw, &doc); uerr != nil {
		t.Fatalf("%s is not valid JSON: %v", path, uerr)
	}
	if len(doc.Items) == 0 {
		t.Fatal("vacuity guard: the frozen break fixture carries no items")
	}
	for _, it := range doc.Items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("the frozen break fixture carries no item %q", id)
	return frozenBreakItem{}
}

func TestStatementThaiCellBreaksAtTheFrozenSeam(t *testing.T) {
	item := statementFrozenBreakItem(t, statementThaiBreakItemID)
	if len(item.ExpectedBreaks) == 0 {
		t.Fatalf("%s carries no interior seam in the frozen fixture, so this comparison would be a zero-break no-op — that is a FINDING, not a licence to mint a new string", item.ID)
	}
	if len(item.Words) < 2 {
		t.Fatalf("%s is labelled as a single word in the frozen fixture", item.ID)
	}

	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			// Locate the cell by its ROW, not by guessing at text.
			var lines []string
			var y int64 = -1
			for _, run := range pages[0] {
				if run.XMP == statementReferenceColumnXMP && run.Text == "TXN-0003" {
					y = run.YMP
				}
			}
			if y < 0 {
				t.Fatal("presence precondition: the Thai row (TXN-0003) is not on page 1")
			}
			for _, run := range pages[0] {
				if run.XMP == statementNoteColumnXMP && (run.YMP == y || run.YMP == y-statementLineHeightMP) {
					lines = append(lines, run.Text)
				}
			}
			if len(lines) < 2 {
				t.Fatalf("%s: the Thai cell %q occupies %d line(s) %q — a cell that never wraps compares ZERO breaks and this assertion would pass vacuously", f.slug, item.Text, len(lines), lines)
			}
			if strings.Join(lines, "") != item.Text {
				t.Fatalf("%s: the Thai cell's lines %q do not reassemble to %q", f.slug, lines, item.Text)
			}
			if fmt.Sprint(lines) != fmt.Sprint(item.Words) {
				t.Errorf("%s: the Thai cell broke as %q; fixtures/expected-breaks/expected_breaks.json labels %s as %q",
					f.slug, lines, item.ID, item.Words)
			}
			// ...and the break POSITION, in rune indices, is the frozen
			// arithmetic — checked independently of the word split.
			var got []int
			acc := 0
			for _, w := range lines[:len(lines)-1] {
				acc += len([]rune(w))
				got = append(got, acc)
			}
			if fmt.Sprint(got) != fmt.Sprint(item.ExpectedBreaks) {
				t.Errorf("%s: the Thai cell's break positions are %v, the frozen fixture records %v for %s", f.slug, got, item.ExpectedBreaks, item.ID)
			}
			t.Logf("%s: %s %q broke as %q at rune positions %v, matching the frozen label", f.slug, item.ID, item.Text, lines, got)
		})
	}
}

// -----------------------------------------------------------------------
// AC5, observable (ii): a Thai mark's position differs from its
// unpositioned default INSIDE A TABLE CELL — GPOS reached the page.
//
// D-4.5.5: a mark whose positioned offset happens to equal its default
// proves nothing, so the subject is a cluster whose GPOS offset is
// non-zero and independently declared: shaping_expectations_test.go's
// row for "ฟั" records a GPOS x-offset of +21 font units on the mark
// glyph. The assertion reads the TJ adjustment that offset becomes in
// the produced content stream — off the artifact, inside a table cell.
// -----------------------------------------------------------------------

const statementGPOSCluster = "ฟั"

func statementDeclaredGPOSXOffset(t *testing.T, face, text string) int16 {
	t.Helper()
	for _, row := range shapedExpectations {
		if row.Face != face || row.Text != text {
			continue
		}
		for _, g := range row.Glyphs {
			if g.XOffset != 0 {
				return g.XOffset
			}
		}
		t.Fatalf("shapedExpectations' row for %q/%q declares no non-zero GPOS x-offset, so it cannot discriminate a positioned mark from an unpositioned one", face, text)
	}
	t.Fatalf("shapedExpectations carries no row for %q/%q", face, text)
	return 0
}

func TestStatementThaiMarkIsPositionedByGPOSInsideACell(t *testing.T) {
	wantOffset := int64(statementDeclaredGPOSXOffset(t, "Noto Sans Thai", statementGPOSCluster))
	if wantOffset == 0 {
		t.Fatal("the declared GPOS x-offset is zero, so this assertion could not tell a positioned mark from an unpositioned one")
	}

	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			pages := statementPageRuns(t, renderStatement(t, f))
			var y int64 = -1
			for _, run := range pages[0] {
				if run.XMP == statementReferenceColumnXMP && run.Text == "TXN-0004" {
					y = run.YMP
				}
			}
			if y < 0 {
				t.Fatal("presence precondition: the GPOS row (TXN-0004) is not on page 1")
			}
			var found *statementDrawnRun
			for i, run := range pages[0] {
				if run.XMP >= statementNoteColumnXMP && run.XMP < statementAmountColumnRightMP &&
					run.Resource == "NotoSansThai" && run.Text == statementGPOSCluster {
					found = &pages[0][i]
					break
				}
			}
			if found == nil {
				t.Fatalf("presence precondition: no %q run was drawn inside the Note column — this assertion exercises nothing", statementGPOSCluster)
			}
			if !found.UsedTJ || !found.NonZeroAdjust {
				t.Fatalf("%s: the %q run inside a table cell emits %v with UsedTJ=%v — a mark whose position equalled its unpositioned default would emit a bare Tj and prove nothing about GPOS",
					f.slug, statementGPOSCluster, found.Adjustments, found.UsedTJ)
			}
			// THE SIGN IS THE POINT, and it was not checked as first
			// written. This story's review, Finding 9: the assertion took
			// the ABSOLUTE VALUE of every adjustment and accepted the run
			// if any magnitude matched, so a renderer that applied the
			// offset in the WRONG DIRECTION would emit [21 -21] instead of
			// [-21 21] and pass unchanged — and on a FIRST recording there
			// is no prior golden to catch that, because the golden records
			// whatever the engine did.
			//
			// The convention, verbatim from internal/pdf/textdoc.go's own
			// doc comment on the emitter:
			//
			//	before glyph i:  -XOffset_i
			//	after  glyph i:  XOffset_i + (W_i - XAdvance_i)
			//
			// A TJ number is SUBTRACTED from the horizontal displacement,
			// so the "before" term of -XOffset shifts the glyph RIGHT by
			// XOffset and the "after" term restores the pen for the runs
			// that follow. The first non-zero adjustment in a run whose
			// only positioned glyph is this mark is therefore exactly
			// -XOffset, and its sign is what says the mark went right
			// rather than left.
			firstNonZero := int64(0)
			firstIdx := -1
			for i, a := range found.Adjustments {
				if a != 0 {
					firstNonZero, firstIdx = a, i
					break
				}
			}
			if firstIdx < 0 {
				t.Fatalf("%s: the %q run's TJ adjustments %v are all zero, so no GPOS offset reached the page at all", f.slug, statementGPOSCluster, found.Adjustments)
			}
			if firstNonZero != -wantOffset {
				t.Errorf("%s: the %q run's first non-zero TJ adjustment is %d at index %d; shapedExpectations declares a GPOS x-offset of %+d for this cluster, which internal/pdf/textdoc.go emits as a BEFORE term of %d.\n\n"+
					"The SIGN is the assertion. A term of %d would move the mark the opposite way, and a magnitude-only comparison would accept it — on a first recording there is no earlier golden that could tell the two apart.",
					f.slug, statementGPOSCluster, firstNonZero, firstIdx, wantOffset, -wantOffset, wantOffset)
			}
			// ...and the offset is COMPENSATED, so the pen is where the
			// following runs expect it: the adjustments over the cluster
			// sum to the net advance correction, which for this mark is
			// zero.
			var sum int64
			for _, a := range found.Adjustments {
				sum += a
			}
			if sum != 0 {
				t.Errorf("%s: the %q run's TJ adjustments %v sum to %d, want 0 — a positioned mark whose offset is not compensated displaces every glyph after it",
					f.slug, statementGPOSCluster, found.Adjustments, sum)
			}
			t.Logf("%s: %q drawn at x=%d inside the Note column with TJ adjustments %v; declared GPOS x-offset %d reached the page", f.slug, statementGPOSCluster, found.XMP, found.Adjustments, wantOffset)
		})
	}
}

// -----------------------------------------------------------------------
// AD-23, and the property that makes every other money assertion in this
// file worth making: THE FIXTURE'S VALUE CLASS CAN EXPRESS A
// DECIMAL-ARITHMETIC DEFECT.
//
// WHY THIS TEST EXISTS, stated as the history rather than as a principle,
// because the principle was already written down and the fixture still
// failed it. As first recorded, every amount in all four documents was an
// exact multiple of 0.25. Every such value is EXACTLY REPRESENTABLE in
// binary64, so all four goldens would have been BYTE-IDENTICAL under a
// renderer whose money path went through binary floating point, and
// TestStatementSumFooterAppearsExactlyOnceOnTheLastPage — whose check
// value is computed by exact integer arithmetic precisely so that it does
// not inherit the bug it exists to catch — would still have passed. The
// check was exact and the fixture was not, which is D-4.5.5 exactly: a
// behaviour is only tested by an input that would render differently
// under a wrong implementation.
//
// WHY THE WRONG IMPLEMENTATION IS NOT WRITTEN HERE, and this is a real
// architectural fact rather than a convenience. This project forbids
// binary floating point under folio-go/ THREE times over, and every one
// of the three covers _test.go files with no exemption:
//
//   - TestNoFloat64UnderModule (internal/arch_test.go) bans the
//     identifiers float64 and float32 across the whole module;
//   - the no-bigfloat-type lint rule bans math/big.Float and math/big.Rat
//     by RESOLVED TYPE IDENTITY, so the obvious way to model a binary
//     float exactly is banned too;
//   - forbidden-imports bans the packages a float path would reach for.
//
// Both were MEASURED, not assumed: a first draft of this test wrote the
// naive path literally and reddened TestNoFloat64UnderModule, naming this
// file; a second draft modelled binary64 with math/big.Float at 53 bits
// and reddened TestBigFloatTypeTestScopeInventory, naming eleven
// expressions in this file. Spelling around either (`total := 0.0`,
// `strconv.ParseFloat(lit, 64)`) would have passed while doing exactly
// the forbidden thing — D-000.15's warning about the workaround that
// erodes a guard fastest — and adding a sanctioned exception to a guard
// that has never had one, in order to make a test convenient, is a
// worse trade than the one taken here.
//
// SO THE PROPERTY IS ASSERTED EXACTLY, AND THE CONSEQUENCE IS PINNED.
//
// The exact half is an integer identity, and it is the STRONGER of the
// two statements. A decimal amount of m hundredths is exactly
// representable in binary64 IF AND ONLY IF 25 divides m: m/100 = m/(4*25),
// so the reduced denominator is a power of two exactly when the factor 25
// cancels, and every quotient m/25 in this fixture is far below 2^53.
// "Every amount is a multiple of 0.25" and "every amount is exactly
// representable in binary floating point" are therefore THE SAME
// STATEMENT about these numbers — no float is needed to check it, and no
// float can check it better.
//
// The consequence half — what a binary money path would actually DRAW —
// is pinned as test-owned literals, measured out of tree by the procedure
// recorded in this story's Delivery Log and cross-checked by two
// independent implementations (CPython's float64, and math/big.Float at
// 53 bits with ToNearestEven). The pin cannot go quietly stale: each row
// pins the EXACT total too, so any edit to the amounts reddens this test
// and sends the reader back to re-measure both numbers together.
// -----------------------------------------------------------------------

// statementMoneyDiscrimination is the measured consequence of this
// fixture's value class, pinned.
//
// floatRendered is what the canonical naive money path draws: parse each
// amount with strconv.ParseFloat, accumulate in a float64, and scale to
// minor units with int64(total*100). The truncation is the whole defect —
// it is what turns a binary64 total that sits a few parts in 10^12 below
// 602,408.68 into 602,408.67.
//
// divergentCells is how many of that document's Amount cells the same
// path would draw differently ONE AT A TIME, which is the divergence a
// human reading the statement would actually see.
var statementMoneyDiscrimination = []struct {
	slug           string
	exactMinor     int64
	exactRendered  string
	floatRendered  string
	divergentCells int
}{
	{"statement-1", 185200, "1,852.00", "1,851.99", 2},
	{"statement-5", 734839, "7,348.39", "7,348.38", 5},
	{"statement-20", 9680182, "96,801.82", "96,801.81", 34},
	{"statement-50", 60240868, "602,408.68", "602,408.67", 78},
}

func TestStatementMoneyColumnDiscriminatesFloatArithmetic(t *testing.T) {
	// (1) THE EXACT PROPERTY, checked by integer arithmetic: not one
	// amount in any of the four documents is a multiple of 0.25, and by
	// the identity above that is precisely "not one amount is exactly
	// representable in binary64". This is the limb a future edit is most
	// likely to undo by accident, so it is asserted over EVERY amount of
	// the largest document — which contains every other document's
	// amounts as a prefix.
	all := statementRows(statementFixtures[len(statementFixtures)-1].rows)
	if len(all) == 0 {
		t.Fatal("vacuity guard: the collection is empty, so nothing about its value class is asserted")
	}
	representable := 0
	for _, r := range all {
		if statementAmountMinor(r.Amount)%25 == 0 {
			representable++
			if representable <= 3 {
				t.Errorf("amount %q (row %s) is an exact multiple of 0.25, so it IS exactly representable in binary64 and cannot distinguish an exact-decimal money path from a binary one", r.Amount, r.Ref)
			}
		}
	}
	if representable > 3 {
		t.Errorf("...and %d of this document's %d amounts are exact multiples of 0.25", representable, len(all))
	}
	if representable == 0 {
		t.Logf("value-class witness: all %d amounts are non-dyadic (no multiple of 0.25), so NONE is exactly representable in binary64", len(all))
	}

	// (2) THE CONTROL, and it is what makes (1) mean anything: the value
	// class this family USED to carry — the discriminating rows' original
	// amounts and an all-.25 filler — was ENTIRELY representable. Without
	// this, "no amount is a multiple of 0.25" would be a claim about an
	// arithmetic test rather than about a change that was made.
	oldClass := []string{"1250.00", "480.25", "75.00", "12.50", "33.75"}
	for i := 6; i <= statementFixtures[len(statementFixtures)-1].rows; i++ {
		oldClass = append(oldClass, fmt.Sprintf("%d.25", 10+i))
	}
	for _, lit := range oldClass {
		if statementAmountMinor(lit)%25 != 0 {
			t.Fatalf("control: %q is not a multiple of 0.25, but the value class this family was FIRST recorded with was entirely quarter-integral — the control does not reproduce the state it is meant to describe", lit)
		}
	}
	t.Logf("control: all %d amounts of the ORIGINAL value class are multiples of 0.25 and therefore exactly representable in binary64 — which is precisely why the first recording of this family could not express the defect (the story's code review, Finding 1)", len(oldClass))

	// (3) THE CONSEQUENCE, pinned. The exact half is recomputed from the
	// fixture on every run, so the pin cannot outlive the numbers it
	// describes; the binary half is a measured literal, because this
	// module may not contain a binary float to recompute it with.
	if len(statementMoneyDiscrimination) != len(statementFixtures) {
		t.Fatalf("statementMoneyDiscrimination pins %d document(s) and statementFixtures declares %d", len(statementMoneyDiscrimination), len(statementFixtures))
	}
	for i, pin := range statementMoneyDiscrimination {
		f := statementFixtures[i]
		t.Run(pin.slug, func(t *testing.T) {
			if pin.slug != f.slug {
				t.Fatalf("pin %d names %q and statementFixtures names %q", i, pin.slug, f.slug)
			}
			gotMinor := statementExpectedSumMinor(f.rows)
			gotRendered := statementFormatMinor(gotMinor)
			if gotMinor != pin.exactMinor || gotRendered != pin.exactRendered {
				t.Fatalf("%s: the EXACT total is now %d minor units (%s), but this pin records %d (%s).\n\n"+
					"The amounts have been edited. Both halves of this pin must be RE-MEASURED together: re-derive the binary64 total by parsing every amount with strconv.ParseFloat, accumulating in a float64 and scaling with int64(total*100), OUTSIDE this module (nothing under folio-go/ may contain a binary float — AD-23), and confirm it still DIFFERS from the exact total. If it no longer differs, the money column has stopped discriminating and the fixture must be fixed, not the pin.",
					f.slug, gotMinor, gotRendered, pin.exactMinor, pin.exactRendered)
			}
			if pin.floatRendered == pin.exactRendered {
				t.Fatalf("%s: the pinned binary64 total %q is EQUAL to the exact total, so this document's %d amounts cannot tell an exact money path from a binary one — AC2(iii) would pass on a renderer carrying the defect AD-23 exists to prevent",
					f.slug, pin.floatRendered, f.rows)
			}
			if pin.divergentCells <= 0 {
				t.Errorf("%s: the pin records %d individually-divergent Amount cells; at least one is needed for the divergence to be visible to a reader cell by cell", f.slug, pin.divergentCells)
			}
			t.Logf("%s: exact total %s; a binary64 money path would draw %s, and would draw %d of its %d Amount cells differently",
				f.slug, pin.exactRendered, pin.floatRendered, pin.divergentCells, f.rows)
		})
	}
}

// statementAmountMinor parses one of the fixture's exact decimal amount
// literals into minor units, by integer arithmetic.
func statementAmountMinor(lit string) int64 {
	whole, frac, _ := strings.Cut(lit, ".")
	var w, f int64
	for _, c := range whole {
		w = w*10 + int64(c-'0')
	}
	for len(frac) < 2 {
		frac += "0"
	}
	for _, c := range frac[:2] {
		f = f*10 + int64(c-'0')
	}
	return w*100 + f
}
