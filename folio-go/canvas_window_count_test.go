package folio8

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/layout"
)

func parseWindowCountTemplate(t *testing.T, source string) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse window-count fixture: %v", err)
	}
	return tpl
}

func projectWithPaint(t *testing.T, tpl *Template) designer.CanvasProjection {
	t.Helper()
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatalf("CanvasWithTextPaint: %v", err)
	}
	return projection
}

// renderPathWindows is the RENDER path's own answer, built the way
// paginateDocument's page-count pass builds it: document bands, the band's
// shaped text runs, the element boxes those bands declare, contentColumnItems,
// layout.Paginate. It is a different route to the same integer, and Story
// 7.5's projection must agree with it rather than assume it does — the
// projection reads the CANVAS paint plan's extents, which is a second consumer
// of one shaping, not a second shaping.
//
// IT RETURNS THE ORIGINS TOO (Story 7.9). Story 7.6 shipped
// ContentWindowOrigins with no render-path oracle anywhere to compare them
// against, so "where each window begins" was asserted only against hardcoded
// numbers on two ungrouped fixtures. PageAssignment.Shift is the render's own
// answer to that question and it costs nothing to return.
//
// ⚠ THE FIFTH ARGUMENT IS WHY STORY 7.9 EXISTS. It used to be `nil`, so this
// oracle disabled keep-together grouping on the RENDER side — and
// TestCanvasWindowCountAgreesWithTheRenderPathOracle then compared an
// ungrouped canvas against an ungrouped render and found them in perfect
// agreement. Both sides were wrong in the same way. It now passes the
// document's REAL keepTogetherTags, which is what makes a grouping divergence
// visible at all. Do not put `nil` back.
func renderPathWindows(t *testing.T, tpl *Template, fs FontSet) (int, []int64) {
	t.Helper()
	data := emptyBindValue(t)
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	runs, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, data, data, fs, newFontCache(), contentBandResolver, nil)
	if err != nil {
		t.Fatalf("collectBandTextRuns: %v", err)
	}
	// Story 9.1's element boxes are content-column items on the render path
	// too, and a signature's ruled line IS one — an oracle that dropped them
	// could not see a non-text group member move.
	boxes, err := collectElementBoxRects(bands, nil)
	if err != nil {
		t.Fatalf("collectElementBoxRects: %v", err)
	}
	plan, err := layout.Paginate(mustPageGeometry(t, tpl), contentColumnItems(runs, nil, boxes, nil, keepTogetherTags(tpl)))
	if err != nil {
		t.Fatalf("layout.Paginate: %v", err)
	}
	origins := make([]int64, 0, len(plan.Pages))
	for _, page := range plan.Pages {
		origins = append(origins, int64(page.Shift))
	}
	return len(plan.Pages), origins
}

// TestCanvasReportsTheWindowHeightAndTheWindowCount is AC4's discriminating
// assertion, and the whole reason canvas_window_count_template.go exists.
//
// The gap fixture declares one line of text at the top of the column and a
// rect TEN WINDOWS BELOW IT. Two is the answer: the window advances to the
// top of the first item that did not fit, so an element declared far below
// the text STARTS the next window rather than generating blank pages before
// it. `ceil(lowestBottom / contentWindowHeight)` answers eleven, so this test
// is red under the one spelling internal/layout/paginate.go forbids by name —
// which the page-count-N fixtures, spaced exactly one window apart, cannot
// say, because there the two routes agree at every N.
func TestCanvasReportsTheWindowHeightAndTheWindowCount(t *testing.T) {
	gap := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountGapTemplateJSON))
	// The window height is ContentHeight's number, and the content band
	// rectangle is one window, so the two must be the same integer: a
	// divergence would have the canvas and the engine drawing different
	// pages while agreeing on every byte.
	if gap.ContentWindowHeight != 727890 {
		t.Fatalf("contentWindowHeight = %d, want 727890", gap.ContentWindowHeight)
	}
	if band := gap.Bands[1]; band.Name != "content" || band.Height != gap.ContentWindowHeight {
		t.Fatalf("content band %q height %d, want it equal to the window height %d", band.Name, band.Height, gap.ContentWindowHeight)
	}
	if gap.ContentWindowCount != 2 {
		closedForm := (int64(7280000+20000) + gap.ContentWindowHeight - 1) / gap.ContentWindowHeight
		t.Fatalf("contentWindowCount = %d, want 2 (the forbidden closed form answers %d here)", gap.ContentWindowCount, closedForm)
	}
	// The negative control: one window apart, both routes answer three. It
	// is here so the test above cannot pass by always answering two, and so
	// the record says why the forbidden spelling survived this long.
	control := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON))
	if control.ContentWindowCount != 3 {
		t.Fatalf("control contentWindowCount = %d, want 3", control.ContentWindowCount)
	}
	if control.ContentWindowHeight != gap.ContentWindowHeight {
		t.Fatalf("the two fixtures share a geometry but report windows of %d and %d", control.ContentWindowHeight, gap.ContentWindowHeight)
	}
}

// TestCanvasWindowCountIsIndependentOfPaintTruncation re-asserts D-7.4.2 §5
// FROM THE PROJECTION SIDE. canvas_body_text_bounds_test.go's
// TestPaginationIsIndependentOfCanvasPaintTruncation guards the RENDER path's
// oracle and stays exactly as it is; this one guards the number Story 7.5
// actually ships, which is computed by a different route.
//
// The discrimination is the second half: a document containing only the
// PAINTED PREFIX occupies visibly fewer windows, so a count that had read the
// paint's line list would be wrong here by a wide margin rather than
// coincidentally right.
func TestCanvasWindowCountIsIndependentOfPaintTruncation(t *testing.T) {
	line := "clause"
	long := strings.TrimSuffix(strings.Repeat(line+"\n", maxCanvasBodyTextLines+400), "\n")
	tpl := bodyTextDocument(t, long, `{"fontFamily":"body","fontSize":12}`)
	projection := projectWithPaint(t, tpl)
	paint := paintOf(t, projection, "e1")
	if paint == nil || !paint.Truncated {
		t.Fatalf("presence precondition: this fixture must TRUNCATE for the test to say anything: %#v", paint)
	}
	if want, _ := renderPathWindows(t, tpl, testFontSet()); projection.ContentWindowCount != int64(want) {
		t.Fatalf("the canvas counts %d windows and the render path %d for the same document", projection.ContentWindowCount, want)
	}
	// What the paint alone would have implied.
	prefix := strings.TrimSuffix(strings.Repeat(line+"\n", len(paint.Lines)), "\n")
	painted := projectWithPaint(t, bodyTextDocument(t, prefix, `{"fontFamily":"body","fontSize":12}`))
	if painted.ContentWindowCount >= projection.ContentWindowCount {
		t.Fatalf("the painted prefix occupies %d windows and the whole document %d; the two must differ for this test to discriminate", painted.ContentWindowCount, projection.ContentWindowCount)
	}
}

// TestCanvasWindowCountAgreesWithTheRenderPathOracle measures the agreement
// the story must not assume. The projection builds its ColumnItems from the
// canvas paint plan's extents; the render path builds its own from shaped
// runs carrying glyphs, faces and CIDs. Same shaping, same vertical model,
// two builders — so equal counts are a property to check, not a given.
func TestCanvasWindowCountAgreesWithTheRenderPathOracle(t *testing.T) {
	// The one-window case is the control with its two later elements removed,
	// so the pair differs in exactly the thing under test.
	oneWindow := canvasWindowCountControlTemplateJSON
	for _, drop := range []string{
		`,
        {"id": "e2", "type": "text", "x": 0, "y": 728, "width": 200, "height": 20, "value": "Window two", "style": {"fontFamily": "body", "fontSize": 12}}`,
		`,
        {"id": "e3", "type": "text", "x": 0, "y": 1456, "width": 200, "height": 20, "value": "Window three", "style": {"fontFamily": "body", "fontSize": 12}}`,
	} {
		if !strings.Contains(oneWindow, drop) {
			t.Fatal("the control fixture moved; this test's edit no longer applies to it")
		}
		oneWindow = strings.Replace(oneWindow, drop, "", 1)
	}
	for name, source := range map[string]string{
		"one window":    oneWindow,
		"three windows": canvasWindowCountControlTemplateJSON,
	} {
		tpl := parseWindowCountTemplate(t, source)
		projection := projectWithPaint(t, tpl)
		if want, wantOrigins := renderPathWindows(t, tpl, testFontSet()); projection.ContentWindowCount != int64(want) ||
			!reflect.DeepEqual(projection.ContentWindowOrigins, wantOrigins) {
			t.Fatalf("%s: canvas counts %d at %v, render path counts %d at %v", name, projection.ContentWindowCount, projection.ContentWindowOrigins, want, wantOrigins)
		}
	}
}

// TestCanvasGroupedTwinDiffersOnlyByTheTags is the precondition the two tests
// below rest on, on fixtures/keep-together/'s own pattern: if the pair ever
// drifted into differing for a second reason, "the counts differ" would stop
// being evidence about grouping.
func TestCanvasGroupedTwinDiffersOnlyByTheTags(t *testing.T) {
	const tag = `"keepTogether": "signature", `
	if n := strings.Count(canvasWindowCountGroupedTemplateJSON, tag); n != 2 {
		t.Fatalf("the grouped template must carry exactly two tags, found %d — the fixture is a two-member group", n)
	}
	if strings.Contains(canvasWindowCountGroupedUngroupedTemplateJSON, "keepTogether") {
		t.Fatal("the untagged twin must carry no keepTogether tag at all")
	}
	if stripped := strings.ReplaceAll(canvasWindowCountGroupedTemplateJSON, tag, ""); stripped != canvasWindowCountGroupedUngroupedTemplateJSON {
		t.Fatalf("the twin is not the grouped template minus its tags — the pair differs for some SECOND reason, so it no longer discriminates:\n%s", stripped)
	}
	// The group has one TEXT member and one NON-TEXT member, which is what
	// makes the equality below reach BOTH of the canvas's column-item arms.
	// Tagging only one arm leaves the other member loose, and this fixture is
	// authored so that being loose changes the answer.
	if !strings.Contains(canvasWindowCountGroupedTemplateJSON, `"type": "text", "x": 0, "y": 700, "width": 240, "height": 20, "keepTogether": "signature"`) ||
		!strings.Contains(canvasWindowCountGroupedTemplateJSON, `"type": "rect", "x": 0, "y": 740, "width": 240, "height": 20, "keepTogether": "signature"`) {
		t.Fatal("the group must span a text member and a non-text member; without both, tagging one canvas arm would satisfy this file")
	}

	// THE CONDITIONAL TWIN, held to the same mechanical equality. It is the
	// grouped template with `visibleIf` added to its rect member and nothing
	// else, and the test that uses it (TestConditionalVisibilityIsWhyTheCount
	// IsNotAlwaysExact) reads its inexactness as evidence about CONDITIONAL
	// VISIBILITY specifically. If the pair ever drifted into differing for a
	// second reason — a bound table, an unresolvable font chain — that
	// document would still be inexact and the test would still be green while
	// measuring something else entirely.
	const conditional = `"keepTogether": "signature", "visibleIf": "showRule", "style"`
	if n := strings.Count(canvasWindowCountConditionalTemplateJSON, conditional); n != 1 {
		t.Fatalf("the conditional template must carry exactly one visibleIf, on the group's rect member, found %d", n)
	}
	if plain := strings.Replace(canvasWindowCountConditionalTemplateJSON, conditional, `"keepTogether": "signature", "style"`, 1); plain != canvasWindowCountGroupedTemplateJSON {
		t.Fatalf("the conditional template is not the grouped one plus a visibleIf — the pair differs for some SECOND reason, so its inexactness is no longer evidence about conditional visibility:\n%s", plain)
	}
}

// assertCanvasAgreesWithTheRenderPath is Story 7.9's spine, and it is stated
// as a helper because its ABSENCE is what let the defect ship: Story 7.7
// taught the render path to keep a declared group whole and did not teach the
// canvas the same thing, and no test anywhere compared the two on a grouped
// document.
//
// It asserts the EQUALITY — count and origins, against a real pagination of
// the same template through the render path's own builders — rather than
// asserting the flag. A flag is a claim about a count; it says nothing about
// whether the count is right.
func assertCanvasAgreesWithTheRenderPath(t *testing.T, name, source string, fs FontSet, wantCount int64, wantOrigins []int64) designer.CanvasProjection {
	t.Helper()
	tpl := parseWindowCountTemplate(t, source)
	projection, err := canvasWithTextPaint(tpl, fs)
	if err != nil {
		t.Fatalf("%s: CanvasWithTextPaint: %v", name, err)
	}
	assertWindowOriginsAreWellFormed(t, name, projection)
	count, origins := renderPathWindows(t, tpl, fs)
	if projection.ContentWindowCount != int64(count) {
		t.Errorf("%s: the canvas counts %d windows and the render path %d for the same document", name, projection.ContentWindowCount, count)
	}
	if !reflect.DeepEqual(projection.ContentWindowOrigins, origins) {
		t.Errorf("%s: the canvas begins its windows at %v and the render path at %v for the same document", name, projection.ContentWindowOrigins, origins)
	}
	// The measured values, so a future change that moved BOTH sides together
	// — the one way an equality assertion can go quiet — still reddens here.
	if projection.ContentWindowCount != wantCount || !reflect.DeepEqual(projection.ContentWindowOrigins, wantOrigins) {
		t.Errorf("%s: canvas count %d origins %v, want %d and %v", name, projection.ContentWindowCount, projection.ContentWindowOrigins, wantCount, wantOrigins)
	}
	return projection
}

// TestCanvasWindowsAgreeWithTheRenderPathForAGroupedDocument is Story 7.9's
// acceptance, and it restores Story 7.6's AC2 — the boundary is marked where
// the engine will ACTUALLY break — for a document that declares a group.
//
// The COUNT arm is canvasWindowCountGroupedTemplateJSON, authored so the tags
// change how many windows the column occupies (3 against the twin's 2). The
// ORIGINS arm is the shipped fixtures/keep-together/ document, where the tags
// change only WHERE window two begins — 706000, the group's earliest top,
// rather than 734000, the top of its overflowing ruled line. Both arms are
// checked against a real pagination and against their twins, because "the
// canvas answers 3" is a fact an unrelated implementation could produce and
// "3 here and 2 there, for two documents differing in two tags" is not.
func TestCanvasWindowsAgreeWithTheRenderPathForAGroupedDocument(t *testing.T) {
	// The two arms are SUBTESTS so neither can mask the other. A wrong count
	// and a wrong origin are different failures — there is a floor flag on
	// the count and none at all on the origins — and a fix that closed only
	// one of them must be visibly red on the other rather than unreached.
	t.Run("count", func(t *testing.T) {
		grouped := assertCanvasAgreesWithTheRenderPath(t, "grouped", canvasWindowCountGroupedTemplateJSON, testFontSet(), 3, []int64{0, 700000, 1440000})
		untagged := assertCanvasAgreesWithTheRenderPath(t, "untagged twin", canvasWindowCountGroupedUngroupedTemplateJSON, testFontSet(), 2, []int64{0, 740000})
		if grouped.ContentWindowCount == untagged.ContentWindowCount {
			t.Errorf("both twins occupy %d windows — the tags changed nothing here, so this arm proves nothing about grouping", grouped.ContentWindowCount)
		}
		// The exact shape of the pre-Story-7.9 defect, named so a regression
		// is recognisable rather than merely red: the canvas used to answer
		// the UNTAGGED numbers for the TAGGED document.
		if reflect.DeepEqual(grouped.ContentWindowOrigins, untagged.ContentWindowOrigins) {
			t.Errorf("the tagged document begins its windows at %v, exactly where the untagged twin does — this is the defect Story 7.9 exists to fix", grouped.ContentWindowOrigins)
		}
	})

	// The origins arm, on the shipped fixture. Its count is 2 either way, so
	// nothing but the origins can discriminate it — which is precisely why
	// Story 7.6's origins needed an oracle of their own.
	t.Run("origins", func(t *testing.T) {
		fixture := assertCanvasAgreesWithTheRenderPath(t, "keep-together fixture", keepTogetherTemplateJSON, testShippedFontSet(), 2, []int64{0, 706000})
		twin := assertCanvasAgreesWithTheRenderPath(t, "keep-together twin", keepTogetherUngroupedTemplateJSON, testShippedFontSet(), 2, []int64{0, 734000})
		if fixture.ContentWindowCount != twin.ContentWindowCount {
			t.Errorf("the shipped pair must agree on the COUNT (%d against %d); this arm is about the ORIGINS", fixture.ContentWindowCount, twin.ContentWindowCount)
		}
		if reflect.DeepEqual(fixture.ContentWindowOrigins, twin.ContentWindowOrigins) {
			t.Errorf("the shipped pair begins its windows identically at %v — a count-only fix would pass this file while the sheet boundary stayed drawn in the wrong place", fixture.ContentWindowOrigins)
		}
	})
}

// renderPathPages is the SHIPPING render, not an oracle: buildPageModel, the
// same function Render calls, on empty data. Where renderPathWindows rebuilds
// the page-count pass out of its parts — and can therefore be wrong in the
// same way as the thing it measures — this counts the pages the document
// actually produces. Story 7.9's four-row matrix is asserted against THIS.
func renderPathPages(t *testing.T, tpl *Template, fs FontSet, dataJSON string) int {
	t.Helper()
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, dataJSON), mustDecodeParams(t), fs)
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	return len(pages)
}

// TestCanvasCountsOnlyWhatTheRenderActuallyPlaces is Story 7.9's SECOND
// defect, and the one the story itself created: the canvas contributed a
// column item for every non-text content component, styled or not, while the
// render path places a rect or a line only where it declares a background or
// a border (element_box.go). Ungrouped, that asymmetry moved an ORIGIN and no
// count, so nothing caught it. Tagged, it moved the COUNT — the group slid for
// a member the printed document has nothing on — and the projection reported
// that count as exact.
//
// ALL FOUR ROWS ARE ASSERTED, not only the one that regressed. The two
// untagged rows were correct before this story and changing WHAT THE CANVAS
// PLACES can move them, so a fix verified only against its own row could trade
// one wrong count for another a second time.
//
//	untagged + styled     canvas 2   render 2
//	untagged + unstyled   canvas 2   render 2
//	tagged   + styled     canvas 3   render 3   (Story 7.9's subject)
//	tagged   + unstyled   canvas 2   render 2   (this test's subject)
func TestCanvasCountsOnlyWhatTheRenderActuallyPlaces(t *testing.T) {
	for _, row := range []struct {
		name      string
		source    string
		wantCount int64
	}{
		{"untagged + styled", canvasWindowCountGroupedUngroupedTemplateJSON, 2},
		{"untagged + unstyled", canvasWindowCountGroupedUngroupedUnstyledTemplateJSON, 2},
		{"tagged + styled", canvasWindowCountGroupedTemplateJSON, 3},
		{"tagged + unstyled", canvasWindowCountGroupedUnstyledTemplateJSON, 2},
		// The kinds the first version of canvasElementIsPlaced exempted
		// UNCONDITIONALLY, and the clause of the box rule it dropped. Each
		// row's second element sits two windows down, so counting one the
		// render path does not place is worth a whole sheet.
		{"image with no file chosen", canvasWindowCountUnfilledImageTemplateJSON, 1},
		{"styled rect with no height", canvasWindowCountFlatRectTemplateJSON, 1},
		{"styled rect with a height", canvasWindowCountTallRectTemplateJSON, 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			tpl := parseWindowCountTemplate(t, row.source)
			projection := projectWithPaint(t, tpl)
			assertWindowOriginsAreWellFormed(t, row.name, projection)
			// Against the SHIPPING render, which is what the sheet count on
			// screen is a claim about.
			if pages := renderPathPages(t, tpl, testFontSet(), `{}`); projection.ContentWindowCount != int64(pages) {
				t.Errorf("the canvas draws %d sheets and the document prints %d pages", projection.ContentWindowCount, pages)
			}
			// And the measured value, so a change that moved BOTH sides
			// together still reddens.
			if projection.ContentWindowCount != row.wantCount {
				t.Errorf("canvas count %d, want %d", projection.ContentWindowCount, row.wantCount)
			}
			// The origins too, from the render path's own builders — the
			// count agreeing while the sheet boundaries sit somewhere else is
			// exactly the half Story 7.6 shipped with no oracle at all.
			if _, origins := renderPathWindows(t, tpl, testFontSet()); !reflect.DeepEqual(projection.ContentWindowOrigins, origins) {
				t.Errorf("the canvas begins its windows at %v and the render path at %v", projection.ContentWindowOrigins, origins)
			}
			// Every row here is a document with no registered cause, so the
			// projection must also CLAIM the count — a right answer reported
			// as untrustworthy is a different failure, not a pass.
			if !projection.ContentWindowCountIsExact {
				t.Errorf("contentWindowCountIsExact is false for a document carrying no registered cause")
			}
		})
	}
	// The discrimination, stated rather than left implicit: the tags change
	// the count for the STYLED pair and must not for the unstyled one. A fix
	// that simply stopped tagging the canvas would satisfy the unstyled rows
	// and fail here.
	styled := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountGroupedTemplateJSON))
	unstyled := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountGroupedUnstyledTemplateJSON))
	if styled.ContentWindowCount == unstyled.ContentWindowCount {
		t.Fatalf("the styled and unstyled grouped documents both occupy %d windows; this matrix discriminates nothing", styled.ContentWindowCount)
	}
	// And the twin pair really does differ in exactly the style declaration.
	if stripped := strings.Replace(canvasWindowCountGroupedTemplateJSON, `, "style": {"background": "#000000"}}`, "}", 1); stripped != canvasWindowCountGroupedUnstyledTemplateJSON {
		t.Fatalf("the unstyled twin is not the grouped template minus its ruled line's style — the pair differs for some SECOND reason:\n%s", stripped)
	}
	if stripped := strings.Replace(canvasWindowCountGroupedUngroupedTemplateJSON, `, "style": {"background": "#000000"}}`, "}", 1); stripped != canvasWindowCountGroupedUngroupedUnstyledTemplateJSON {
		t.Fatalf("the untagged unstyled twin is not its own template minus the style:\n%s", stripped)
	}
	// THE IMAGE DISCRIMINATOR, and it cannot be a row above. renderPathWindows
	// passes `nil` image runs, so the origins oracle every row is checked
	// against has no opinion about an image at all; only the SHIPPING render
	// can judge this pair. Without it, "the canvas answers 1 for an image with
	// no file chosen" would be satisfied by a canvas that never counted an
	// image at all.
	filledTpl := parseWindowCountTemplate(t, canvasWindowCountFilledImageTemplateJSON)
	filled := projectWithPaint(t, filledTpl)
	assertWindowOriginsAreWellFormed(t, "image with a file chosen", filled)
	if pages := renderPathPages(t, filledTpl, testFontSet(), `{}`); filled.ContentWindowCount != int64(pages) || pages != 2 {
		t.Errorf("image with a file chosen: the canvas draws %d sheets and the document prints %d pages, want 2 and 2", filled.ContentWindowCount, pages)
	}
	unfilled := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountUnfilledImageTemplateJSON))
	if filled.ContentWindowCount == unfilled.ContentWindowCount {
		t.Fatalf("an image with a file and an image without one both occupy %d windows; this pair discriminates nothing", filled.ContentWindowCount)
	}
	// And the two pairs added here differ in exactly the substring that
	// decides placement, the same mechanical check the styled pair gets.
	const assetKey = `"5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"`
	if n := strings.Count(canvasWindowCountUnfilledImageTemplateJSON, `"asset": null`); n != 1 {
		t.Fatalf("the unfilled image template must carry exactly one null asset, found %d", n)
	}
	if chosen := strings.Replace(canvasWindowCountUnfilledImageTemplateJSON, `"asset": null`, `"asset": `+assetKey, 1); chosen != canvasWindowCountFilledImageTemplateJSON {
		t.Fatalf("the filled image twin is not the unfilled one with a file chosen — the pair differs for some SECOND reason:\n%s", chosen)
	}
	if n := strings.Count(canvasWindowCountFlatRectTemplateJSON, `"height": 0,`); n != 1 {
		t.Fatalf("the flat rect template must declare exactly one zero height, found %d", n)
	}
	if raised := strings.Replace(canvasWindowCountFlatRectTemplateJSON, `"height": 0,`, `"height": 20,`, 1); raised != canvasWindowCountTallRectTemplateJSON {
		t.Fatalf("the tall rect twin is not the flat one given a height — the pair differs for some SECOND reason:\n%s", raised)
	}
}

// TestCanvasCountsOnlyTheTablesTheRenderDraws is the same defect one kind
// over. canvasElementIsPlaced's first version returned true for every table
// unconditionally, on the claim that a table is "always placed"; it is not.
// internal/template/parse_bands.go imposes no minimum column count, so
// `"columns": []` loads, and collectBandTableRuns skips such a table outright
// — nothing to lay out or draw — so it reaches no content-column item and the
// printed document is one page shorter than the canvas drew.
//
// IT IS A TEST OF ITS OWN rather than a row of the matrix above, for two
// reasons that are both properties of tables and not of this pair. A table
// needs a BINDING to render at all, and a content-band table with a non-empty
// binding is registered cause (a), so neither document here can claim an
// exact count — while every row of the matrix asserts exactness. And
// renderPathWindows collects no table rects, so the origins oracle those rows
// are checked against has no opinion about a table either. What is left is
// the strongest oracle available anyway: the SHIPPING render, buildPageModel.
func TestCanvasCountsOnlyTheTablesTheRenderDraws(t *testing.T) {
	// Empty rather than absent: an absent binding is a render error, and this
	// pair is about the COLUMNS, so the data must not be what discriminates.
	const data = `{"transactions": []}`
	for _, row := range []struct {
		name      string
		source    string
		wantCount int64
	}{
		{"table with no columns", canvasWindowCountColumnlessTableTemplateJSON, 1},
		{"table with one column", canvasWindowCountOneColumnTableTemplateJSON, 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			tpl := parseWindowCountTemplate(t, row.source)
			projection := projectWithPaint(t, tpl)
			assertWindowOriginsAreWellFormed(t, row.name, projection)
			if pages := renderPathPages(t, tpl, testFontSet(), data); projection.ContentWindowCount != int64(pages) {
				t.Errorf("the canvas draws %d sheets and the document prints %d pages", projection.ContentWindowCount, pages)
			}
			if projection.ContentWindowCount != row.wantCount {
				t.Errorf("canvas count %d, want %d", projection.ContentWindowCount, row.wantCount)
			}
			// Cause (a) applies to both, and it is asserted so that "the
			// canvas agrees with the render here" is never read as a claim
			// that a bound table's count can be trusted: with rows in the
			// data the render runs longer and the canvas cannot know it.
			if projection.ContentWindowCountIsExact {
				t.Errorf("contentWindowCountIsExact is true for a document whose content band carries a bound table")
			}
		})
	}
	// The discrimination, stated rather than left implicit.
	columnless := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountColumnlessTableTemplateJSON))
	oneColumn := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountOneColumnTableTemplateJSON))
	if columnless.ContentWindowCount == oneColumn.ContentWindowCount {
		t.Fatalf("a table with no columns and a table with one both occupy %d windows; this pair discriminates nothing", columnless.ContentWindowCount)
	}
	const column = `{"id": "e3", "label": "Date", "width": 80, "align": "left", "bind": "{{row.date}}"}`
	if n := strings.Count(canvasWindowCountColumnlessTableTemplateJSON, `"columns": []`); n != 1 {
		t.Fatalf("the columnless table template must declare exactly one empty column list, found %d", n)
	}
	if filled := strings.Replace(canvasWindowCountColumnlessTableTemplateJSON, `"columns": []`, `"columns": [`+column+`]`, 1); filled != canvasWindowCountOneColumnTableTemplateJSON {
		t.Fatalf("the one-column twin is not the columnless template given a column — the pair differs for some SECOND reason:\n%s", filled)
	}
}

// TestGroupingIsNotARegisteredCauseOfInexactness is D-7.7.6's other half,
// asserted rather than left to the enumeration in page_setup.go's doc comment.
//
// keepTogetherTags takes the *Template and nothing else — no data, no params,
// no FontSet — so grouping is a pure template property and the canvas holds
// every input it needs to be RIGHT about it. The registered causes are each
// things the canvas genuinely CANNOT know; registering a knowable case among
// them would park a defect inside the mechanism that exists to be honest about
// shortfalls.
func TestGroupingIsNotARegisteredCauseOfInexactness(t *testing.T) {
	for _, grouped := range []struct{ name, source string }{
		{"grouped count fixture", canvasWindowCountGroupedTemplateJSON},
		{"grouped shipped fixture", keepTogetherTemplateJSON},
	} {
		fs := testFontSet()
		if grouped.source == keepTogetherTemplateJSON {
			fs = testShippedFontSet()
		}
		tpl := parseWindowCountTemplate(t, grouped.source)
		projection, err := canvasWithTextPaint(tpl, fs)
		if err != nil {
			t.Fatalf("%s: %v", grouped.name, err)
		}
		if !projection.ContentWindowCountIsExact {
			t.Errorf("%s: contentWindowCountIsExact is false for a well-formed grouped document — grouping is knowable canvas-side and must never become a registered cause", grouped.name)
		}
	}
	// The tag cannot reach a TABLE, which is what stops a group inheriting a
	// table's data dependency — the one grouping case that would genuinely be
	// unknowable canvas-side. internal/template's
	// TestKeepTogetherIsNotValidOnATable owns the refusal; this states, at the
	// seam that depends on it, WHY the sentence above is true.
	if _, err := ParseTemplate([]byte(strings.Replace(
		canvasWindowCountBoundTableTemplateJSON,
		`"type": "table", "x": 0, "y": 0,`,
		`"type": "table", "x": 0, "y": 0, "keepTogether": "signature",`, 1))); err == nil {
		t.Fatal("a table carrying keepTogether was accepted; a group that could contain a bound table would be a grouping case the canvas cannot know")
	}
}

// TestCanvasWindowCountDegradesRatherThanFailingTheProjection is Story 7.5's
// Ruling C. Lifting the content band's cap makes a component taller than one
// window authorable, so layout.Paginate's OverflowError becomes reachable
// from the canvas for the first time. Turning it into a projection failure
// would make a canvas bound into a document validity rule and would blank the
// canvas with no attributable error; reporting ONE window is Paginate's own
// answer for a column it cannot place.
func TestCanvasWindowCountDegradesRatherThanFailingTheProjection(t *testing.T) {
	tpl := parseWindowCountTemplate(t, canvasWindowCountOversizedTemplateJSON)
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatalf("an over-tall content component failed the projection: %v", err)
	}
	if projection.ContentWindowCount != 1 {
		t.Fatalf("over-tall contentWindowCount = %d, want the degraded floor of 1", projection.ContentWindowCount)
	}
	if len(projection.Components) != 1 || projection.Components[0].Height != 900000 {
		t.Fatalf("the component itself must survive the degradation: %#v", projection.Components)
	}
	// The precondition, stated rather than trusted: the geometry really is
	// past one window, so the degradation branch really is the one taken.
	if geomHeight := projection.Components[0].Height; geomHeight <= projection.ContentWindowHeight {
		t.Fatalf("component height %d does not exceed one window %d", geomHeight, projection.ContentWindowHeight)
	}
}

// TestCanvasWindowCountIsAFloorForABoundTable records, rather than lets Story
// 7.6 discover by drawing one sheet for a fifty-page statement, that the
// count describes THE COLUMN AS THE CANVAS PAINTS IT.
//
// projectedSize returns a table's header height and no rows, because the
// canvas has never been given the data. The behaviour is pre-existing (Epic 6
// shipped it) and irreparable inside this story — reusing the render/bind
// machinery is Story 13.4, in another epic — so what 7.5 owes is that the
// number says what it is a number about, in the projection's own comment and
// here.
func TestCanvasWindowCountIsAFloorForABoundTable(t *testing.T) {
	tpl := parseWindowCountTemplate(t, canvasWindowCountBoundTableTemplateJSON)
	projection := projectWithPaint(t, tpl)
	if projection.ContentWindowCount != 1 {
		t.Fatalf("bound-table contentWindowCount = %d, want 1", projection.ContentWindowCount)
	}
	if len(projection.Components) != 1 {
		t.Fatalf("want one projected component, got %d", len(projection.Components))
	}
	// The floor's mechanism, asserted so a future change to projectedSize
	// cannot quietly turn this count into a prediction: the projected table
	// is exactly its header, with no row contributing a millipoint.
	if height := projection.Components[0].Height; height != 16000 {
		t.Fatalf("projected table height = %d, want the header height 16000 and no rows", height)
	}
	if projection.Components[0].Height >= projection.ContentWindowHeight {
		t.Fatal("the fixture must fit one window for the floor to be the interesting fact about it")
	}
}

// TestCanvasReportsOneWindowForAnEmptyColumn pins internal/layout's own rule
// at the projection seam: a document with no content items is ONE page, not
// zero. A zero here would reach Story 7.6 as a canvas with no sheets on it.
func TestCanvasReportsOneWindowForAnEmptyColumn(t *testing.T) {
	tpl := componentTemplate(t)
	bare, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	// Canvas has no FontSet and therefore cannot shape; it reports the
	// documented floor of one window and never reaches the browser, because
	// every projection that does is a CanvasWithTextPaint.
	if bare.ContentWindowCount != 1 {
		t.Fatalf("Canvas contentWindowCount = %d, want the documented floor of 1", bare.ContentWindowCount)
	}
	if bare.ContentWindowHeight != bare.Bands[1].Height {
		t.Fatalf("Canvas window height %d does not match its own content band %d", bare.ContentWindowHeight, bare.Bands[1].Height)
	}
	empty := parseWindowCountTemplate(t, fmt.Sprintf(`{"assets":{},"bands":{"content":{"elements":[]},"pageFooter":{"elements":[],"height":24},"pageHeader":{"elements":[],"height":18}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","nextId":1,"page":{"margin":{"bottom":42,"left":36,"right":54,"top":30},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"%s"}`, "1.0"))
	if projection := projectWithPaint(t, empty); projection.ContentWindowCount != 1 {
		t.Fatalf("empty-column contentWindowCount = %d, want 1", projection.ContentWindowCount)
	}
}

// assertWindowOriginsAreWellFormed states the three properties the browser
// protocol independently re-checks, so a fixture that satisfied the exact
// values below by accident — a stale slice, a duplicated shift — still fails
// here. A projection that fails any of them is not merely wrong: it is
// rejected by parseInbound, which discards the WHOLE snapshot and blanks the
// canvas with no attributable error.
func assertWindowOriginsAreWellFormed(t *testing.T, name string, projection designer.CanvasProjection) {
	t.Helper()
	origins := projection.ContentWindowOrigins
	if origins == nil {
		t.Fatalf("%s: contentWindowOrigins is nil, which marshals to JSON null and blanks the canvas", name)
	}
	if int64(len(origins)) != projection.ContentWindowCount {
		t.Fatalf("%s: %d origins for %d windows; there must be exactly one origin per window", name, len(origins), projection.ContentWindowCount)
	}
	if len(origins) == 0 || origins[0] != 0 {
		t.Fatalf("%s: origins = %v, want a first window beginning at column offset 0", name, origins)
	}
	for i := 1; i < len(origins); i++ {
		if origins[i] <= origins[i-1] {
			t.Fatalf("%s: origins = %v are not strictly increasing at index %d", name, origins, i)
		}
	}
}

// TestCanvasProjectsWhereEachWindowBegins is Story 7.6's AC2, and it
// red-proves the forbidden closed form a SECOND time — more sharply than the
// count can.
//
// The count only distinguishes `index * contentWindowHeight` from the engine's
// answer where the spacing is uneven: on the CONTROL fixture, elements a round
// 728pt apart, both routes answer three. The ORIGINS distinguish it there too,
// because the window height is 727890 and the elements sit at 728000 — so the
// closed form answers [0, 727890, 1455780] where the engine answers
// [0, 728000, 1456000], adrift by 110 millipoints per window. That is small
// enough to survive a casual eye on screen and large enough to assert exactly.
func TestCanvasProjectsWhereEachWindowBegins(t *testing.T) {
	gap := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountGapTemplateJSON))
	assertWindowOriginsAreWellFormed(t, "gap", gap)
	// The gap fixture's second element is declared TEN windows below the
	// text, and the window advances to the top of the first item that did not
	// fit — so window two begins at that element's own Y, not nine windows
	// earlier and not at a multiple of anything.
	if got := gap.ContentWindowOrigins; len(got) != 2 || got[0] != 0 || got[1] != 7280000 {
		t.Fatalf("gap contentWindowOrigins = %v, want [0 7280000]", got)
	}

	control := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON))
	assertWindowOriginsAreWellFormed(t, "control", control)
	closedForm := []int64{0, control.ContentWindowHeight, 2 * control.ContentWindowHeight}
	if got := control.ContentWindowOrigins; len(got) != 3 || got[0] != 0 || got[1] != 728000 || got[2] != 1456000 {
		t.Fatalf("control contentWindowOrigins = %v, want [0 728000 1456000] (the forbidden closed form answers %v)", got, closedForm)
	}
	// Stated rather than left implicit: the two answers really do differ on
	// this fixture, so the assertion above is a discrimination and not a
	// coincidence.
	if closedForm[1] != 727890 || closedForm[2] != 1455780 {
		t.Fatalf("the closed form on this geometry is %v; this test's red proof assumed [0 727890 1455780]", closedForm)
	}
}

// TestCanvasSaysWhenTheWindowCountIsExact pins the flag to its documented
// causes and, just as importantly, to its ABSENCE where none of them holds. A
// flag stuck at either value would satisfy half of this test and say nothing,
// which is why both halves are here: a document that SHOULD be exact reddens
// if the field is forced false, and a document carrying a cause reddens if it
// is forced true. Inverting a boolean flips every call site at once and the
// corpus cannot catch a backwards one, because most documents are exact.
func TestCanvasSaysWhenTheWindowCountIsExact(t *testing.T) {
	for _, exact := range []struct {
		name   string
		source string
	}{
		{"gap", canvasWindowCountGapTemplateJSON},
		{"control", canvasWindowCountControlTemplateJSON},
		// Story 7.9: a document DECLARING a keep-together group. Its count
		// moved when the canvas learned about grouping, and it is still
		// EXACT — grouping is a pure template property, so it is a defect
		// to get wrong and never a cause to disclose.
		{"grouped", canvasWindowCountGroupedTemplateJSON},
		// The SAME unshapeable text as case (c) below, in the page header
		// instead of the content band. The flag is a statement about the
		// content column, and this column is counted exactly — so the
		// degradation site's `band.name == bandContent` guard is what keeps
		// this false. Deleting that guard left the whole Go suite green.
		{"unshaped header", canvasWindowCountUnshapedHeaderTemplateJSON},
	} {
		projection := projectWithPaint(t, parseWindowCountTemplate(t, exact.source))
		if !projection.ContentWindowCountIsExact {
			t.Fatalf("%s: contentWindowCountIsExact is false for a column with no table, no degradation, no unshaped text and no conditional element", exact.name)
		}
	}

	// (a) A bound table: the canvas has the header and none of the rows.
	table := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountBoundTableTemplateJSON))
	assertWindowOriginsAreWellFormed(t, "bound table", table)
	if table.ContentWindowCount != 1 || len(table.ContentWindowOrigins) != 1 || table.ContentWindowCountIsExact {
		t.Fatalf("bound table: count %d, origins %v, exact %v; want 1, [0] and false", table.ContentWindowCount, table.ContentWindowOrigins, table.ContentWindowCountIsExact)
	}

	// (b) The Ruling C degradation: a component taller than one window.
	oversized := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountOversizedTemplateJSON))
	assertWindowOriginsAreWellFormed(t, "oversized", oversized)
	if oversized.ContentWindowCount != 1 || len(oversized.ContentWindowOrigins) != 1 || oversized.ContentWindowCountIsExact {
		t.Fatalf("oversized: count %d, origins %v, exact %v; want 1, [0] and false", oversized.ContentWindowCount, oversized.ContentWindowOrigins, oversized.ContentWindowCountIsExact)
	}

	// (c) A content text element whose chain would not resolve contributes no
	// extents, so the column is counted short. The discriminating half is the
	// precondition: the element really is present and really painted nothing.
	unshaped := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountUnshapedTextTemplateJSON))
	assertWindowOriginsAreWellFormed(t, "unshaped text", unshaped)
	if paint := paintOf(t, unshaped, "e1"); paint == nil || len(paint.Lines) != 0 {
		t.Fatalf("precondition: e1 must degrade to an empty paint for this case to say anything: %#v", paint)
	}
	if paint := paintOf(t, unshaped, "e2"); paint == nil || len(paint.Lines) == 0 {
		t.Fatalf("precondition: e2 must shape normally, so the column is counted and merely counted SHORT: %#v", paint)
	}
	if unshaped.ContentWindowCountIsExact {
		t.Fatal("unshaped text: contentWindowCountIsExact is true, but one content element's lines are missing from the column that was counted")
	}

	// (d) A content element whose VISIBILITY DEPENDS ON DATA. The canvas
	// projects visibleIf as a string and evaluates nothing, so it places an
	// element the render may omit.
	conditional := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountConditionalTemplateJSON))
	assertWindowOriginsAreWellFormed(t, "conditional", conditional)
	if conditional.ContentWindowCountIsExact {
		t.Fatal("conditional visibility: contentWindowCountIsExact is true, but whether one content element is placed at all is a property of data the canvas has never been given")
	}
}

// TestConditionalVisibilityIsWhyTheCountIsNotAlwaysExact measures cause (d)
// rather than asserting it, and measures it in the direction that forced the
// field's rename.
//
// The canvas has NO data, so it places the conditional element and answers 3.
// The real render answers 3 when the condition holds and 2 when it does not —
// AD-24 makes a hidden element absent WITH NO GAP, so the group never slides.
// That is canvas >= render: a CEILING. The field this replaced was called
// ContentWindowCountIsFloor and would have been set true here, which is a
// second confidently-wrong disclosure — the exact failure this story exists to
// stop, one layer down.
//
// IT IS NOT ABOUT GROUPING. The tag is on the fixture because it makes the
// divergence loud; an ungrouped element carrying visibleIf diverges the same
// way, and has since Story 7.5 shipped the count. The untagged control below
// says so.
func TestConditionalVisibilityIsWhyTheCountIsNotAlwaysExact(t *testing.T) {
	tpl := parseWindowCountTemplate(t, canvasWindowCountConditionalTemplateJSON)
	projection := projectWithPaint(t, tpl)
	shown := renderPathPages(t, tpl, testFontSet(), `{"showRule": true}`)
	hidden := renderPathPages(t, tpl, testFontSet(), `{"showRule": false}`)
	if shown == hidden {
		t.Fatalf("both data cases print %d pages; this fixture measures nothing about conditional visibility", shown)
	}
	if projection.ContentWindowCount != int64(shown) {
		t.Errorf("the canvas draws %d sheets; with the element visible the document prints %d pages", projection.ContentWindowCount, shown)
	}
	// The DIRECTION, stated as the measurement it is: the canvas is never
	// short here, it is long. A floor claim on this document would be false.
	if projection.ContentWindowCount < int64(hidden) {
		t.Errorf("canvas %d is below the hidden render's %d; this cause is a CEILING and the assertion above assumed it", projection.ContentWindowCount, hidden)
	}
	if projection.ContentWindowCountIsExact {
		t.Error("the canvas reports this count exact while the same document prints two different lengths")
	}
	// The control: the SAME visibleIf with no tag anywhere. The cause predates
	// grouping and must be registered without it.
	untagged := strings.ReplaceAll(canvasWindowCountConditionalTemplateJSON, `"keepTogether": "signature", `, "")
	if strings.Contains(untagged, "keepTogether") {
		t.Fatal("the untagged control still carries a tag")
	}
	if projectWithPaint(t, parseWindowCountTemplate(t, untagged)).ContentWindowCountIsExact {
		t.Error("an UNGROUPED element carrying visibleIf reports an exact count; grouping is how this cause was found, not what causes it")
	}
}

// TestCanvasOriginsForAnEmptyColumnAndForTheShapelessEntryPoint covers the two
// ends of the range: a column with nothing in it, and the entry point that
// cannot shape at all.
func TestCanvasOriginsForAnEmptyColumnAndForTheShapelessEntryPoint(t *testing.T) {
	empty := projectWithPaint(t, parseWindowCountTemplate(t, `{"assets":{},"bands":{"content":{"elements":[]},"pageFooter":{"elements":[],"height":24},"pageHeader":{"elements":[],"height":18}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","nextId":1,"page":{"margin":{"bottom":42,"left":36,"right":54,"top":30},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"1.0"}`))
	assertWindowOriginsAreWellFormed(t, "empty column", empty)
	if len(empty.ContentWindowOrigins) != 1 || !empty.ContentWindowCountIsExact {
		t.Fatalf("empty column: origins %v, exact %v; want [0] and true — nothing about an empty column is unknowable", empty.ContentWindowOrigins, empty.ContentWindowCountIsExact)
	}

	// Canvas has no FontSet, cannot shape, and says so in both fields: one
	// window beginning at zero, DECLARED a floor. It never reaches the
	// browser, but the struct is shared and its values must be honest.
	bare, err := canvas(componentTemplate(t))
	if err != nil {
		t.Fatal(err)
	}
	assertWindowOriginsAreWellFormed(t, "Canvas", bare)
	if len(bare.ContentWindowOrigins) != 1 || bare.ContentWindowCountIsExact {
		t.Fatalf("Canvas: origins %v, exact %v; want [0] and false — the shapeless entry point cannot count and must not claim it can", bare.ContentWindowOrigins, bare.ContentWindowCountIsExact)
	}
}

// sheetOf answers which drawn sheet a column offset lands on, the way the
// designer's own model does: the last window that begins at or above it.
func sheetOf(origins []int64, y int64) int {
	sheet := 0
	for i := 1; i < len(origins); i++ {
		if origins[i] <= y {
			sheet = i
		}
	}
	return sheet
}

// TestAComponentAuthoredWindowsDownTheColumnLandsOnItsOwnSheet closes Story
// 7.6's loop END TO END rather than asserting that a conditional changed: a
// component is CREATED windows below the top of the column through the same
// band-aware command the designer's later-sheet placement sends, the template
// is serialized to its canonical bytes and parsed back, and the projection
// taken from those bytes is asked which sheet the component is on. Then it is
// MOVED further down and asked again.
//
// It is deliberately built from the command surface and the canonical bytes,
// not from a fixture literal: what the story claims is that an author can put
// something on sheet three and have it stay there.
func TestAComponentAuthoredWindowsDownTheColumnLandsOnItsOwnSheet(t *testing.T) {
	// The control fixture is the base because its font chain is one
	// testFontSet supplies: this test is about where a component LANDS, and a
	// document the projection cannot shape would never reach the question.
	tpl := parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON)
	bands := projectedBands(t, tpl)
	content := bands["content"]
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	deep := content.Height*2 + 5000
	created, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":`+pointLiteral(deep)+`,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatalf("createComponent two windows down the column was refused: %v", err)
	}
	component := newProjectedComponent(t, before, created)

	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := ParseTemplate(canonical)
	if err != nil {
		t.Fatalf("the canonical bytes did not parse back: %v", err)
	}
	projection := projectWithPaint(t, reloaded)
	assertWindowOriginsAreWellFormed(t, "authored deep", projection)
	if !projection.ContentWindowCountIsExact {
		t.Fatalf("this document has no table, no degradation, no unshaped text and no conditional element; exact = %v", projection.ContentWindowCountIsExact)
	}
	sheet := sheetOf(projection.ContentWindowOrigins, component.Y)
	if sheet == 0 {
		t.Fatalf("a component at column offset %d landed on sheet one of %d; origins %v", component.Y, projection.ContentWindowCount, projection.ContentWindowOrigins)
	}
	// The window it landed in really does contain it — the sheet the canvas
	// draws it on shows it, rather than merely being the last one that starts
	// above it.
	origin := projection.ContentWindowOrigins[sheet]
	if component.Y < origin || component.Y >= origin+projection.ContentWindowHeight {
		t.Fatalf("component y %d is not inside window %d, which spans [%d, %d)", component.Y, sheet, origin, origin+projection.ContentWindowHeight)
	}

	// AND FURTHER DOWN, through the ordinary opaque move the drag commits —
	// a COLUMN coordinate, not a pin to a sheet.
	deeper := content.Height*5 + 5000
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+component.ID+`","x":0,"y":`+pointLiteral(deeper)+`,"snap":false}`)); err != nil {
		t.Fatalf("a move five windows down the column was refused: %v", err)
	}
	movedBytes, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	movedTemplate, err := ParseTemplate(movedBytes)
	if err != nil {
		t.Fatal(err)
	}
	moved := projectWithPaint(t, movedTemplate)
	assertWindowOriginsAreWellFormed(t, "moved deeper", moved)
	if landed := sheetOf(moved.ContentWindowOrigins, deeper); landed <= sheet {
		t.Fatalf("moving from column offset %d to %d left the component on sheet %d, no later than sheet %d; origins %v", component.Y, deeper, landed, sheet, moved.ContentWindowOrigins)
	}
	if reloadedComponent(t, tpl, component.ID).Y != deeper {
		t.Fatalf("the canonical bytes did not carry the column coordinate %d", deeper)
	}
}

// TestAStyledTextBoxCountsTheSameWindowsAsTheRenderPath is E9-3(b): the
// canvas gives a content-band text element its DECLARED BOX as a second
// column item, beside its shaped lines, exactly as the render path does.
//
// Measured before the repair: RENDER pages=2 | CANVAS windows=1. The
// oracle here is renderPathWindows — documentBands, collectElementBoxRects,
// contentColumnItems, layout.Paginate — a genuinely different route to the
// number, and it checks the ORIGINS too, so a count that agreed by
// coincidence over a different partition would still fail.
func TestAStyledTextBoxCountsTheSameWindowsAsTheRenderPath(t *testing.T) {
	tpl := parseWindowCountTemplate(t, canvasWindowCountStyledTextBoxTemplateJSON)
	projection := projectWithPaint(t, tpl)
	want, wantOrigins := renderPathWindows(t, tpl, testFontSet())
	if want != 2 {
		t.Fatalf("the fixture no longer spans two windows on the render path (got %d) — it proves nothing in this shape", want)
	}
	if projection.ContentWindowCount != int64(want) || !reflect.DeepEqual(projection.ContentWindowOrigins, wantOrigins) {
		t.Fatalf("canvas counts %d at %v, render path counts %d at %v — a styled text element's declared box must occupy the canvas column exactly where it occupies the printed one",
			projection.ContentWindowCount, projection.ContentWindowOrigins, want, wantOrigins)
	}
}

// TestAStyledTextBoxDoesNotChangeTheExactnessFlag is the INVERSE of the
// assertion this test once made, and it is the invariant that survived
// E9-3: declaring a box must not change WHAT THE CANVAS CAN KNOW, and
// therefore must not change the flag. Both fixtures report exact = true.
//
// A fifth cause did once clear the flag here. It was removed with (b) in
// place, and the argument is a correctness one rather than a tidiness one.
// The flag's semantic is "can the canvas know", not "is the canvas being
// careful": its four remaining causes are each a case of genuine
// unknowability — no data for a bound table, an unresolvable chain, a
// data-dependent visibility verdict, a degraded pagination. Once the canvas
// mirrors the render path's box item, a styled text box is not such a case;
// a declared box is declared geometry, fully knowable with no data. Clearing
// the flag over a count that is provably right is a false statement with a
// conservative sign, and this finding existed because the flag made a false
// statement. Fixing a false `true` by installing a false `false` is a
// polarity swap, not a repair.
//
// What guards the COUNT is
// TestAStyledTextBoxCountsTheSameWindowsAsTheRenderPath, which reds if the
// mirrored box item is reverted. What this test guards is the flag's
// meaning: nothing may clear it for a knowable box.
func TestAStyledTextBoxDoesNotChangeTheExactnessFlag(t *testing.T) {
	styled := parseWindowCountTemplate(t, canvasWindowCountStyledTextBoxTemplateJSON)
	if !projectWithPaint(t, styled).ContentWindowCountIsExact {
		t.Error("a content-band text element declaring a box cleared ContentWindowCountIsExact — a declared box is knowable with no data, so it cannot be a cause of inexactness")
	}
	// The control: the same template with the box declaration removed. It
	// differs in exactly the thing under test, so "the flag is unchanged"
	// is evidence about the box and not about the fixture.
	unstyled := strings.Replace(canvasWindowCountStyledTextBoxTemplateJSON, `, "background": "#eeeeee"`, "", 1)
	if unstyled == canvasWindowCountStyledTextBoxTemplateJSON {
		t.Fatal("the fixture moved; this test's edit no longer applies to it")
	}
	if !projectWithPaint(t, parseWindowCountTemplate(t, unstyled)).ContentWindowCountIsExact {
		t.Error("the unstyled control did not report exact either — the fixture proves nothing about the box")
	}
}
