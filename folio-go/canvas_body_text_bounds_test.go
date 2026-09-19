package folio8

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"

	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// bodyTextDocument builds a one-text-element document carrying value, plus a
// SECOND text element with short text, so every test here can also assert
// D-7.4.2's central promise: degrading one element leaves every other
// component in the document projecting normally.
func bodyTextDocument(t *testing.T, value string, style string) *Template {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	source := fmt.Sprintf(`{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},`+
		`"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[`+
		`{"id":"e1","type":"text","x":0,"y":0,"width":500,"height":20,"value":%s,"style":%s},`+
		`{"id":"e2","type":"text","x":0,"y":600,"width":200,"height":20,"value":"neighbour","style":{"fontFamily":"body","fontSize":12}}`+
		`]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":3}`, encoded, style)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse body-text document: %v", err)
	}
	return tpl
}

func paintOf(t *testing.T, projection designer.CanvasProjection, id string) *designer.CanvasTextPaint {
	t.Helper()
	for _, component := range projection.Components {
		if component.ID == id {
			return component.TextPaint
		}
	}
	t.Fatalf("component %s is not in the projection", id)
	return nil
}

// TestCanvasBodyTextDegradesPastTheLineBoundInsteadOfAbortingIsThe DW-25 fix
// stated as behaviour rather than as a constant: the projection that used to
// return an error — blanking the WHOLE canvas for a document that renders to
// a perfectly good PDF — now paints a prefix and says so.
func TestCanvasBodyTextDegradesPastTheLineBoundInsteadOfAborting(t *testing.T) {
	// Mandatory breaks set the line count directly, so this is the cheapest
	// document that reaches the bound (DW-25's own reachability note).
	value := strings.TrimSuffix(strings.Repeat("clause\n", maxCanvasBodyTextLines+40), "\n")
	tpl := bodyTextDocument(t, value, `{"fontFamily":"body","fontSize":12}`)
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatalf("projection aborted instead of degrading: %v", err)
	}
	paint := paintOf(t, projection, "e1")
	if paint == nil || !paint.Truncated {
		t.Fatalf("over-long element was not flagged as truncated: %#v", paint)
	}
	if len(paint.Lines) != maxCanvasBodyTextLines {
		t.Fatalf("painted %d lines, want the first %d", len(paint.Lines), maxCanvasBodyTextLines)
	}
	if len(paint.Lines[0].Fragments) == 0 || paint.Lines[0].Fragments[0].Text != "clause" {
		t.Fatalf("the painted prefix is not the value's own first line: %#v", paint.Lines[0])
	}
	assertWithinBrowserFragmentBounds(t, paint, maxCanvasBodyTextLines)
	// Every OTHER component projects normally. This is the half that makes
	// it a degradation rather than a differently-shaped abort.
	neighbour := paintOf(t, projection, "e2")
	if neighbour == nil || neighbour.Truncated || len(neighbour.Lines) != 1 {
		t.Fatalf("neighbouring component did not project normally: %#v", neighbour)
	}
	// The document is untouched: truncation lives on the paint side only
	// (D-7.4.2 §1), and element.Value is what the properties panel saves.
	if got := tpl.doc.Bands.Content.Elements[0].Value.Value; got != value {
		t.Fatalf("projection shortened the element's VALUE (%d bytes, want %d)", len(got), len(value))
	}
}

// TestCanvasTruncatedPaintIsDistinguishableFromAnEmptyOne is D-7.4.2 §2: the
// all-clear must not wear the face of could-not-look. Before this story a
// 400-line element and an element with no text both projected `Lines: []`.
func TestCanvasTruncatedPaintIsDistinguishableFromAnEmptyOne(t *testing.T) {
	long := strings.TrimSuffix(strings.Repeat("clause\n", maxCanvasBodyTextLines+2), "\n")
	tpl := bodyTextDocument(t, long, `{"fontFamily":"body","fontSize":12}`)
	// e2 is emptied in place, so both dispositions come from ONE projection.
	tpl.doc.Bands.Content.Elements[1].Value.Value = ""
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	truncated := paintOf(t, projection, "e1")
	empty := paintOf(t, projection, "e2")
	if truncated == nil || empty == nil {
		t.Fatalf("missing paint: truncated=%#v empty=%#v", truncated, empty)
	}
	if !truncated.Truncated || len(truncated.Lines) == 0 {
		t.Fatalf("truncated element must paint a prefix AND carry the flag: %#v", truncated.Truncated)
	}
	if empty.Truncated || len(empty.Lines) != 0 {
		t.Fatalf("empty element must be unflagged and unpainted: %#v", empty)
	}
}

// TestCanvasFragmentBudgetRefusesACumulativeOverrunOfPerLineLegalLines is the
// per-line/cumulative ASYMMETRY, asserted explicitly. Go's
// maxCanvasTextFragments counts fragments PER LINE; engine-protocol.ts counts
// them CUMULATIVELY across the component and drops the WHOLE engine response
// when it overruns. So a projection whose every line is legal per-line, but
// whose total is not, must be degraded here — or the canvas blanks in the
// browser with no error anyone can attribute.
//
// This exercises the rule the projection itself calls, rather than a document
// that reaches it, for a measured reason: packLines is superlinear in a
// value's break-opportunity count (measured at HEAD, one justified element:
// 1.2 s at 4,000 word opportunities, 9.8 s at 8,000), and a justified
// component's cumulative fragment count is ~ its word count, so a fixture
// reaching 65,536 would take minutes of wall clock in the ordinary suite. The
// projection-level half is asserted alongside every other paint test below:
// no projection may emit more than the browser accepts.
func TestCanvasFragmentBudgetRefusesACumulativeOverrunOfPerLineLegalLines(t *testing.T) {
	budget := canvasFragmentBudget{}
	lines := 0
	// Every line here carries exactly maxCanvasTextFragments fragments —
	// the largest a line may legally carry. None of them is refused for
	// its own size; the sequence stops only on the cumulative bound.
	for budget.admits(maxCanvasTextFragments) {
		budget.take(maxCanvasTextFragments)
		lines++
	}
	if want := maxCanvasBodyTextFragments / maxCanvasTextFragments; lines != want {
		t.Fatalf("admitted %d per-line-legal lines, want %d before the cumulative bound bit", lines, want)
	}
	if budget.used > maxCanvasBodyTextFragments {
		t.Fatalf("emitted %d cumulative fragments; the browser rejects past %d", budget.used, maxCanvasBodyTextFragments)
	}
	if lines <= 1 {
		t.Fatal("presence precondition: the budget must admit more than one line, or it is bounding the wrong quantity")
	}
	// And the per-line guard still refuses on its own, on a fresh budget
	// with the whole cumulative allowance untouched. The two bounds are
	// independent; neither implies the other.
	fresh := canvasFragmentBudget{}
	if fresh.admits(maxCanvasTextFragments + 1) {
		t.Fatal("a line past the PER-LINE guard was admitted because the cumulative budget was empty")
	}
	if !fresh.admits(maxCanvasTextFragments) {
		t.Fatal("a line exactly at the per-line guard was refused")
	}
}

// TestCanvasBodyTextDegradesPastThePerLineFragmentGuard drives the same
// refusal through a REAL projection: an element with no declared width packs
// its whole value onto one line, so a value of more than
// maxCanvasTextFragments words puts a single line past the per-line guard.
// That used to abort the projection outright.
func TestCanvasBodyTextDegradesPastThePerLineFragmentGuard(t *testing.T) {
	// Wide enough that ONE wrapped line holds more than maxCanvasTextFragments
	// word-pieces, and long enough to wrap — a justified element's LAST line
	// is drawn at its natural edge, so the over-full line must not be the
	// last one.
	value := strings.TrimSpace(strings.Repeat("a ", 3*maxCanvasTextFragments))
	source := fmt.Sprintf(`{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},`+
		`"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[`+
		`{"id":"e1","type":"text","x":0,"y":0,"width":6000,"height":20,"value":%q,"style":{"fontFamily":"body","fontSize":12,"align":"justify"}},`+
		`{"id":"e2","type":"text","x":0,"y":600,"width":200,"height":20,"value":"neighbour","style":{"fontFamily":"body","fontSize":12}}`+
		`]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":3}`, value)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatalf("projection aborted instead of degrading: %v", err)
	}
	paint := paintOf(t, projection, "e1")
	if paint == nil || !paint.Truncated {
		t.Fatalf("a line past the per-line fragment guard was not flagged as truncated: %#v", paint)
	}
	// MEASURED: this path paints NO lines at all, and the count is asserted
	// rather than left to fall out of a bounds check that an empty slice
	// satisfies for free. The reason it is zero is structural: the element
	// declares a width wide enough that packLines puts the whole value on its
	// FIRST line, that line already carries more than maxCanvasTextFragments
	// word-pieces, and the guard is a WHOLE-LINE guard — so there is no line
	// of the value that fits, and the prefix that does fit is empty.
	//
	// An empty paint is exactly what `Truncated` exists to disambiguate
	// (D-7.4.2 §2): without the flag this element and one with no text at all
	// would be indistinguishable on the wire, and the canvas would show the
	// all-clear for a paint that could not look. Whether the projection ought
	// instead to paint a PARTIAL first line is a production-behaviour
	// question, deliberately not answered here.
	assertWithinBrowserFragmentBounds(t, paint, 0)
	neighbour := paintOf(t, projection, "e2")
	if neighbour == nil || neighbour.Truncated || len(neighbour.Lines) != 1 {
		t.Fatalf("neighbouring component did not project normally: %#v", neighbour)
	}
}

// assertWithinBrowserFragmentBounds is the projection-level half of the
// asymmetry: whatever a paint carries, it must be something
// engine-protocol.ts will admit — per line AND cumulatively.
//
// wantLines IS THE ANTI-VACUITY GUARD, and it is required rather than
// optional. Every bound below is trivially satisfied by an EMPTY Lines slice,
// so this helper once certified a paint that painted nothing at all as
// "within the browser's bounds" — the assertion passed and said nothing. A
// caller must now state the line count it expects, which turns an empty paint
// into a declared outcome instead of an accident wearing a pass's face.
func assertWithinBrowserFragmentBounds(t *testing.T, paint *designer.CanvasTextPaint, wantLines int) {
	t.Helper()
	if len(paint.Lines) != wantLines {
		t.Fatalf("paint carries %d lines, want %d — this assertion is only meaningful over the line count its caller declared", len(paint.Lines), wantLines)
	}
	cumulative := 0
	for i, line := range paint.Lines {
		if len(line.Fragments) > maxCanvasTextFragments {
			t.Fatalf("line %d carries %d fragments, past the per-line guard", i, len(line.Fragments))
		}
		cumulative += len(line.Fragments)
	}
	if cumulative > maxCanvasBodyTextFragments {
		t.Fatalf("emitted %d cumulative fragments; the browser rejects past %d", cumulative, maxCanvasBodyTextFragments)
	}
}

// TestPaginationIsIndependentOfCanvasPaintTruncation is D-7.4.2 §5. Story
// 7.5's projection reports how many windows the content column occupies; if a
// truncated PAINT could shorten that, Story 7.6 would draw the wrong number
// of sheets and the canvas would lie about pagination. The independence holds
// today by construction — nothing in Go reads CanvasTextPaint — and this
// pins it.
func TestPaginationIsIndependentOfCanvasPaintTruncation(t *testing.T) {
	long := strings.TrimSuffix(strings.Repeat("clause\n", maxCanvasBodyTextLines+400), "\n")
	tpl := bodyTextDocument(t, long, `{"fontFamily":"body","fontSize":12}`)
	data := emptyBindValue(t)

	pages := func() int {
		t.Helper()
		bands, err := documentBands(tpl)
		if err != nil {
			t.Fatalf("documentBands: %v", err)
		}
		runs, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, data, data, testFontSet(), newFontCache(), contentBandResolver, nil)
		if err != nil {
			t.Fatalf("collectBandTextRuns: %v", err)
		}
		plan, err := layout.Paginate(mustPageGeometry(t, tpl), contentColumnItems(runs, nil, nil, nil, nil))
		if err != nil {
			t.Fatalf("layout.Paginate: %v", err)
		}
		return len(plan.Pages)
	}

	before := pages()
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	paint := paintOf(t, projection, "e1")
	if paint == nil || !paint.Truncated {
		t.Fatalf("presence precondition: this fixture must TRUNCATE for the test to say anything: %#v", paint)
	}
	after := pages()
	if before != after {
		t.Fatalf("page count moved across a paint projection: %d then %d", before, after)
	}
	// And the count is the FULL text's, demonstrably more than the painted
	// prefix could account for: a page count derived from paint.lines.length
	// would be visibly wrong here, not coincidentally right.
	if before <= len(paint.Lines)/48 {
		t.Fatalf("page count %d is not larger than the truncated paint's %d lines would imply", before, len(paint.Lines))
	}
}

// TestCanvasIdentifierBoundsStillRefuseAtFiveHundredAndTwelve records the
// SPLIT, not just the raise: the eight identifier, colour and expression
// sites keep maxCanvasPropertyString and keep aborting. Epic 7 makes none of
// them newly reachable, so that residue is recorded rather than fixed
// (D-7.4.2 §6) — and this test is where "recorded" is executable.
//
// ALL EIGHT, and STILL EIGHT AFTER STORY 14.9 — deliberately, and the reason
// is the sharpest thing in this comment. 14.9 added two sites bounded by
// maxCanvasPropertyString (a table COLUMN's `label` and its own `bind`) and
// they belong in NEITHER list, because they do not REFUSE: they CLIP. A
// document with a 600-byte column label loads and prints a real PDF —
// `decodeColumn` caps neither field — so refusing it would blank the designer's
// canvas until reload for a template that prints correctly. A probe asserting
// refusal for those two would now be asserting a defect. Their coverage was not
// deleted, it MOVED: the clipping assertion is at the foot of this function,
// and the behavioural half is in canvas_table_column_projection_test.go. This is
// a THIRD category on the same constant, and the category is what DW-104's
// grep-derived remedy will have to carry.
//
// Each by its OWN refusal message. Probing a subset would
// leave the unprobed sites free to move without anything going red, which is
// the opposite of recording them; and asserting only that Canvas returned
// SOME error would let any unrelated parse or projection failure keep this
// test green while the bound it names had gone. The eight are the sites
// `grep -n maxCanvasPropertyString folio-go/page_setup.go` reports, minus the
// constant's own declaration and its doc comment. The eighth is Story 8.1's
// chain ENTRY: it was a site this list did not cover from the day it was
// added, which is the drift a count in a comment exists to catch.
//
// ⚠ CORRECTED IN PLACE AT STORY 8.4e's CLOSE, 2026-09-01. THE SENTENCE ABOVE
// IS NOW FALSE AND IS KEPT SO THE DRIFT IS READABLE. That grep reports NINE
// enforcement sites, not eight: Story 8.4e added `page_setup.go:1547`, the
// bound on a text fragment's shipped FACE NAME, and did not add a probe here.
// The list below is still eight, and eight is still the right number — but by
// EXCEPTION now, not by exhaustion, and an exception nothing writes down is
// how this comment was wrong twice before.
//
// THE NINTH IS UNREACHABLE BY A DOCUMENT, WHICH IS WHY IT IS NOT PROBED HERE.
// `Canvas` runs `canvasFontChains` -> `projectFontChainEntry`, which refuses
// the same string over the same bound at `:645`, and it returns before
// `CanvasWithTextPaint` ever calls `addCanvasTextPaint`. For a shipped face
// `fragment.face` IS that chain entry's `Face`, so an over-long name has
// already failed the projection by the time `:1547` runs. Every probe in the
// table below drives a real document through `Canvas`; there is no document
// that reaches the ninth site, so there is no probe to write. MEASURED, twice
// and independently: deleting `:1547` outright leaves this suite at exactly
// 1815 pass / 2 fail / 5 skip, byte for byte what it is unmutated.
//
// AND THE HAND-LIST ITSELF IS THE STANDING DEFECT — DW-104. The count lives
// only in this prose, so nothing goes red when a site is added; the remedy is
// to derive the site list from `page_setup.go` by grep and assert this table
// covers it minus a NAMED exception list. A count is lossy; a set difference
// is not. Deferred rather than done at 8.4e's close because a new guard needs
// its own mutation proof.
func TestCanvasIdentifierBoundsStillRefuseAtFiveHundredAndTwelve(t *testing.T) {
	long := strings.Repeat("x", maxCanvasPropertyString+1)
	if len(long) >= maxCanvasBodyText {
		t.Fatal("fixture precondition: the identifier fixture must be far below the body-text bound")
	}
	// Three of the eight are not reachable by parsing a document that names
	// them: `style.fontFamily` must name a declared chain, a `bind` belongs
	// to a table extension, and an over-long chain NAME would be refused by
	// the style site first if any element referenced it. They are reached by
	// setting the field on the parsed document, which is what the projection
	// itself sees — the bound under test is the PROJECTION's, not the
	// loader's, and probing it needs a document that gets that far.
	for _, probe := range []struct {
		name     string
		document func(t *testing.T) *Template
		message  string
	}{
		{
			name: "visibleIf",
			document: func(t *testing.T) *Template {
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
				tpl.doc.Bands.Content.Elements[0].VisibleIf = template.Presence[string]{Set: true, Value: long}
				return tpl
			},
			message: "folio8: visibleIf exceeds the 512-byte editor/projection limit",
		},
		{
			name: "table.bind",
			document: func(t *testing.T) *Template {
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
				element := &tpl.doc.Bands.Content.Elements[0]
				element.Type = template.ElementTable
				element.Width = template.Presence[geom.Length]{}
				element.Table = template.Presence[template.TableExt]{Set: true, Value: template.TableExt{Bind: long}}
				return tpl
			},
			message: "folio8: component table bind exceeds the projection bound",
		},
		{
			name: "style.fontFamily",
			document: func(t *testing.T) *Template {
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
				tpl.doc.Bands.Content.Elements[0].Style.Value.FontFamily = template.Presence[string]{Set: true, Value: long}
				return tpl
			},
			message: "folio8: component fontFamily exceeds the projection bound",
		},
		{
			name: "color",
			document: func(t *testing.T) *Template {
				// Written past the loader, which refuses a non-#RRGGBB colour.
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
				tpl.doc.Bands.Content.Elements[0].Style.Value.Color = template.Presence[string]{Set: true, Value: long}
				return tpl
			},
			message: "folio8: component color exceeds the projection bound",
		},
		{
			name: "background",
			document: func(t *testing.T) *Template {
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
				tpl.doc.Bands.Content.Elements[0].Style.Value.Background = template.Presence[string]{Set: true, Value: long}
				return tpl
			},
			message: "folio8: component background exceeds the projection bound",
		},
		{
			name: "border.color",
			document: func(t *testing.T) *Template {
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12,"border":{"width":1}}`)
				tpl.doc.Bands.Content.Elements[0].Style.Value.Border.Value.Color = template.Presence[string]{Set: true, Value: long}
				return tpl
			},
			message: "folio8: component borderColor exceeds the projection bound",
		},
		{
			// The chain NAME itself, in the document's `fonts` map — the one
			// site that is not on a component at all. No element references
			// it, so the component sites pass and canvasFontChains is
			// genuinely the refusing site.
			name: "fonts chain name",
			document: func(t *testing.T) *Template {
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
				tpl.doc.Fonts[long] = []template.FontChainEntry{template.FaceEntry("Roboto-Regular")}
				return tpl
			},
			message: "folio8: font family name exceeds the projection bound",
		},
		{
			// The chain ENTRY — the site the projection acquired when Story
			// 8.1 put the chains themselves on the wire, and one nothing else
			// in this table reaches: a face name is not a component property,
			// and the chain-NAME row above is refused before the entry walk is
			// entered at all. Measured before this row existed: the guard
			// could be deleted with every Go test still green.
			name: "fonts chain entry",
			document: func(t *testing.T) *Template {
				tpl := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
				tpl.doc.Fonts["body"] = []template.FontChainEntry{template.FaceEntry(long)}
				return tpl
			},
			message: "folio8: font chain entry exceeds the projection bound",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			_, err := canvas(probe.document(t))
			if err == nil {
				t.Fatalf("%s over 512 reached the projection; the identifier bound must still refuse", probe.name)
			}
			if err.Error() != probe.message {
				t.Fatalf("%s was refused for the wrong reason:\n got %q\nwant %q", probe.name, err.Error(), probe.message)
			}
		})
	}
	// And the body-text site does NOT refuse at 512 any more — the split is
	// only real if BOTH halves moved, so both are asserted here.
	//
	// Half one, the element VALUE: Canvas alone reaches it, and its failure
	// is still an abort, so `err == nil` is the whole claim.
	tpl := bodyTextDocument(t, strings.Repeat("y", maxCanvasPropertyString+1), `{"fontFamily":"body","fontSize":12}`)
	if _, err := canvas(tpl); err != nil {
		t.Fatalf("a 513-byte clause is still refused: %v", err)
	}
	// Half two, the FRAGMENT text — the site DW-25 never named. It needs its
	// own assertion because its failure mode CHANGED with this story: it no
	// longer aborts, it DEGRADES. Repointing this one site back at
	// maxCanvasPropertyString would therefore raise no error at all; it
	// would quietly paint a truncation notice over a clause well inside the
	// documented bound, and an `err == nil` check would stay green through
	// it. Measured with that revert applied: Truncated=true, len(Lines)==0.
	//
	// 513 unbroken bytes is ONE fragment by construction — a run of 'y' has
	// no break opportunity inside it — so this fixture puts the fragment
	// site, and only the fragment site, under the bound.
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatalf("CanvasWithTextPaint on a 513-byte clause: %v", err)
	}
	paint := paintOf(t, projection, "e1")
	if paint == nil {
		t.Fatal("the 513-byte clause projected no paint at all")
	}
	if paint.Truncated {
		t.Fatal("a 513-byte clause was TRUNCATED: the fragment-text site is back on the identifier bound")
	}
	if len(paint.Lines) != 1 || len(paint.Lines[0].Fragments) != 1 {
		t.Fatalf("fixture precondition: want one line of one fragment, got %d line(s)", len(paint.Lines))
	}
	if got := len(paint.Lines[0].Fragments[0].Text); got != maxCanvasPropertyString+1 {
		t.Fatalf("the fragment painted %d bytes, want the whole %d", got, maxCanvasPropertyString+1)
	}
	// AND STORY 14.9's TWO SITES DO NOT REFUSE AT ALL — the third category this
	// function's comment names. They are here, beside the eight that do, so the
	// contrast is readable in one place: over the same bound, on the same
	// constant, these two CLIP and keep their column. Deleting the clip and
	// restoring an abort reddens this.
	table := bodyTextDocument(t, "short", `{"fontFamily":"body","fontSize":12}`)
	element := &table.doc.Bands.Content.Elements[0]
	element.Type = template.ElementTable
	element.Width = template.Presence[geom.Length]{}
	element.Table = template.Presence[template.TableExt]{Set: true, Value: template.TableExt{
		Bind:    "rows[]",
		Columns: []template.Column{{ID: "e9", Label: long, Width: 80000, Bind: long}},
	}}
	clipped, err := canvas(table)
	if err != nil {
		t.Fatalf("an over-bound column label or bind was REFUSED; both must clip, because the document that carries them loads and prints: %v", err)
	}
	projected := canvasTableColumnsOf(t, clipped, "e1")
	if len(projected) != 1 {
		t.Fatalf("the over-bound column was dropped rather than clipped: %#v", projected)
	}
	if len(projected[0].Label) != maxCanvasPropertyString || len(projected[0].Bind) != maxCanvasPropertyString {
		t.Fatalf("the over-bound label/bind projected %d/%d bytes, want both clipped to %d", len(projected[0].Label), len(projected[0].Bind), maxCanvasPropertyString)
	}
}

// TestCanvasTextPaintAlwaysMarshalsBothBooleanDispositions is the one thing
// standing between an innocent-looking `,omitempty` and a designer with no
// canvas and no error to attribute it to.
//
// engine-protocol.ts's isTextPaint requires `typeof value.truncated ===
// 'boolean'` behind an EXACT-key hasOnly, so a `truncated` key that
// disappeared whenever it is false would make parseInbound drop every real
// snapshot. Every neighbouring optional field on this wire carries
// `,omitempty` — which is exactly why adding it here would read as tidying up
// — and both `go test ./...` and `npm test` would stay green while the
// product stopped working. Nothing else in Go asserts the MARSHALLED form.
func TestCanvasTextPaintAlwaysMarshalsBothBooleanDispositions(t *testing.T) {
	encoded, err := json.Marshal(designer.CanvasTextPaint{})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"overflow"`, `"truncated"`} {
		if !strings.Contains(string(encoded), key) {
			t.Fatalf("a default CanvasTextPaint marshalled as %s, with no %s key: the browser drops every snapshot whose paint omits it", encoded, key)
		}
	}
	// And on a REAL, untruncated projection, which is the case an omitempty
	// tag would actually silence.
	projection, err := canvasWithTextPaint(bodyTextDocument(t, "clause", `{"fontFamily":"body","fontSize":12}`), testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	paint := paintOf(t, projection, "e1")
	if paint == nil || paint.Overflow || paint.Truncated {
		t.Fatalf("fixture precondition: this paint must carry BOTH dispositions false: %#v", paint)
	}
	painted, err := json.Marshal(paint)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"overflow":false`, `"truncated":false`} {
		if !strings.Contains(string(painted), want) {
			t.Fatalf("an ordinary paint marshalled as %s, without %s", painted, want)
		}
	}
}

// TestCanvasBodyTextPastTheChannelCeilingIsStillRefused is the matrix's
// "value past the channel ceiling" row: the ONE body-text site that stayed a
// hard refusal while every other body-text bound became a degradation. That
// refusal is deliberate, and it lives on the VALUE rather than on the paint
// because a value may never be truncated, only its paint (D-7.4.2 §1) —
// element.Value is what the properties panel edits and SAVES, so shortening
// it here would write the cut into the author's own document. It is a
// channel-representability backstop at megabyte scale, recorded rather than
// fixed because Epic 7's input cannot reach it.
//
// The assertion pins the BOUNDARY rather than merely "big things fail":
// exactly maxCanvasBodyText bytes must get past this check, one byte more
// must not. canvasComponents runs before any shaping, so Canvas — which
// never paints — reaches the check directly, and the fixture is a single
// whitespace-free run so nothing downstream can spend packLines time on it.
func TestCanvasBodyTextPastTheChannelCeilingIsStillRefused(t *testing.T) {
	atTheCeiling := strings.Repeat("x", maxCanvasBodyText)
	if _, err := canvas(bodyTextDocument(t, atTheCeiling, `{"fontFamily":"body","fontSize":12}`)); err != nil {
		t.Fatalf("a value of exactly %d bytes was refused: %v", maxCanvasBodyText, err)
	}
	if _, err := canvas(bodyTextDocument(t, atTheCeiling+"x", `{"fontFamily":"body","fontSize":12}`)); err == nil {
		t.Fatalf("a value of %d bytes reached the projection; the channel backstop must refuse it", maxCanvasBodyText+1)
	}
}

// TestCanvasBodyTextAtTheLineBoundCarriesEveryLine is the matrix's "long
// clause, under the new bounds" row, and the UNTRUNCATED side of the cliff:
// the tests above assert what happens past maxCanvasBodyTextLines, this one
// asserts that a value landing exactly ON it keeps every line and is not
// flagged. A bound that truncated at its own value would make the epic's
// forty-page target unreachable by one line.
func TestCanvasBodyTextAtTheLineBoundCarriesEveryLine(t *testing.T) {
	// Mandatory breaks again set the line count directly, as the
	// neighbouring tests do — the cheap way to land exactly on the bound.
	// The last line is spelled differently so it can be identified.
	value := strings.Repeat("clause\n", maxCanvasBodyTextLines-1) + "final"
	tpl := bodyTextDocument(t, value, `{"fontFamily":"body","fontSize":12}`)
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatalf("a document exactly at the line bound was refused a projection: %v", err)
	}
	paint := paintOf(t, projection, "e1")
	if paint == nil {
		t.Fatal("the element at the line bound has no paint at all")
	}
	if paint.Truncated {
		t.Fatal("a value exactly at the line bound was flagged as truncated")
	}
	if len(paint.Lines) != maxCanvasBodyTextLines {
		t.Fatalf("painted %d lines, want all %d", len(paint.Lines), maxCanvasBodyTextLines)
	}
	last := paint.Lines[len(paint.Lines)-1]
	if len(last.Fragments) == 0 || last.Fragments[0].Text != "final" {
		t.Fatalf("the last painted line is not the value's own last line: %#v", last)
	}
	assertWithinBrowserFragmentBounds(t, paint, maxCanvasBodyTextLines)
}

// TestCanvasFontChainEntryCountIsBoundedOnALoadedDocument is the projection's
// per-chain entry-count guard, measured where it actually bites: a document
// LOADED FROM BYTES that already declares a chain deeper than the projection
// will carry. decodeFonts validates nothing about a chain's length, so such a
// .folio parses; only the projection refuses it.
//
// It is deliberately not the command path's test.
// TestFontChainEntryCountIsBoundedAtTheCommand drives addFontChain, a
// different function with its own literals, and it left this one unexercised —
// measured, the two projection guards (the entry count here and the entry
// LENGTH in the table above) could both be deleted with every Go test in the
// module still green. What they prevent is not a visible error: an over-long
// chain on the wire makes the browser's isCanvas return false, which discards
// the WHOLE snapshot and blanks the canvas with nothing to attribute it to.
func TestCanvasFontChainEntryCountIsBoundedOnALoadedDocument(t *testing.T) {
	chain := func(count int) *Template {
		t.Helper()
		faces := make([]string, count)
		for i := range faces {
			faces[i] = fmt.Sprintf("%q", fmt.Sprintf("face-%d", i))
		}
		source := strings.Replace(fontChainDocJSON, `"body": ["Noto Sans", "Noto Sans Thai"]`, `"body": [`+strings.Join(faces, ",")+`]`, 1)
		tpl, err := ParseTemplate([]byte(source))
		if err != nil {
			t.Fatalf("the loader refused a %d-entry chain; decodeFonts validates nothing about a chain's length and this test needs it not to start: %v", count, err)
		}
		return tpl
	}
	if _, err := canvas(chain(maxCanvasFontChainEntries)); err != nil {
		t.Fatalf("a chain at the bound was refused a projection: %v", err)
	}
	_, err := canvas(chain(maxCanvasFontChainEntries + 1))
	if err == nil {
		t.Fatal("a chain past the bound reached the projection; the browser would drop the whole snapshot and blank the canvas with no attributable error")
	}
	if err.Error() != "folio8: font chain declares more entries than the projection bound" {
		t.Fatalf("the over-deep chain was refused for the wrong reason: %q", err.Error())
	}
}
