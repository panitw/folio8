package folio8

import (
	"bytes"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/geom"
)

func TestCanvasTextPaintUsesPackedProductionLineGeometryWithoutMutation(t *testing.T) {
	tpl := parseFontTestTemplate(t)
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Components) == 0 || projection.Components[0].TextPaint == nil {
		t.Fatalf("missing text paint projection: %#v", projection.Components)
	}
	paint := projection.Components[0].TextPaint
	if len(paint.Lines) != 1 || paint.Lines[0].Width <= 0 || paint.Lines[0].Baseline < paint.Lines[0].Top || len(paint.Lines[0].Fragments) == 0 {
		t.Fatalf("invalid text paint line: %#v", paint.Lines)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("text paint mutated canonical template: %v", err)
	}
}

func TestCanvasTextPaintCarriesEngineOverflowInsteadOfRebreaking(t *testing.T) {
	tpl, err := ParseTemplate([]byte(`{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":1,"height":20,"value":"unbreakable","style":{"fontFamily":"body","fontSize":12}}]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":2}`))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	paint := projection.Components[0].TextPaint
	if paint == nil || !paint.Overflow || len(paint.Lines) != 1 || paint.Lines[0].Width <= projection.Components[0].Width {
		t.Fatalf("overflow paint = %#v", paint)
	}
}

// TestCanvasTextPaintKeepsAPlaceholderOnOneLine: the canvas paints the
// expression source, and a call's argument spaces must not wrap it onto a
// second line below its box — the PDF prints the value, not the source.
func TestCanvasTextPaintKeepsAPlaceholderOnOneLine(t *testing.T) {
	const tplJSON = `{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":120,"height":30,"value":"{{formatNumber(amountDue, \"#,##0.00\")}}","style":{"fontFamily":"body","fontSize":22}}]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":2}`
	paintOf := func(src string) *designer.CanvasTextPaint {
		t.Helper()
		tpl, err := ParseTemplate([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		projection, err := canvasWithTextPaint(tpl, testFontSet())
		if err != nil {
			t.Fatal(err)
		}
		return projection.Components[0].TextPaint
	}
	paint := paintOf(tplJSON)
	if paint == nil || len(paint.Lines) != 1 || !paint.Overflow {
		t.Fatalf("placeholder paint = %#v, want one overflowing line", paint)
	}
	// THE NEGATIVE CONTROL: the same characters without the delimiters are
	// plain text, and plain text still wraps at its spaces.
	control := paintOf(strings.Replace(tplJSON, `{{formatNumber(amountDue, \"#,##0.00\")}}`, `formatNumber(amountDue, \"#,##0.00\")`, 1))
	if control == nil || len(control.Lines) < 2 {
		t.Fatalf("control paint = %#v, want the literal to wrap", control)
	}
}

func TestCanvasTextPaintExactlyMatchesTheShippingRunPath(t *testing.T) {
	tpl := parseFontTestTemplate(t)
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	data := emptyBindValue(t)
	runs, err := collectTextRuns(tpl, data, data, testFontSet(), newFontCache())
	if err != nil {
		t.Fatalf("shipping run collection: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("presence precondition: shipping path produced no text runs")
	}
	byElement := make(map[string][]textRunSource)
	for _, run := range runs {
		byElement[run.elementID] = append(byElement[run.elementID], run)
	}
	bandY := make(map[string]int64)
	for _, band := range projection.Bands {
		bandY[band.Name] = band.Y
	}
	shippingFonts := newFontCache()
	for _, component := range projection.Components {
		if component.Type != "text" || component.TextPaint == nil {
			continue
		}
		paint := component.TextPaint
		wantRuns := byElement[component.ID]
		if len(wantRuns) == 0 {
			t.Fatalf("component %s has paint but no shipping runs", component.ID)
		}
		for lineIndex, line := range paint.Lines {
			var lineRuns []textRunSource
			for _, run := range wantRuns {
				if run.lineIndex == lineIndex {
					lineRuns = append(lineRuns, run)
				}
			}
			if len(lineRuns) != len(line.Fragments) {
				t.Fatalf("component %s line %d fragments = %d, want shipping run count %d", component.ID, lineIndex, len(line.Fragments), len(lineRuns))
			}
			var shippingWidth geom.Length
			for fragmentIndex, run := range lineRuns {
				runWidth, err := shippingRunWidth(run, testFontSet(), shippingFonts)
				if err != nil {
					t.Fatal(err)
				}
				shippingWidth += runWidth
				bandOrigin := bandY[component.Band] - projection.MarginTop
				if line.Top != int64(run.y)-bandOrigin || line.Baseline != int64(run.y+run.baselineOffset)-bandOrigin {
					t.Errorf("component %s line %d origin/baseline diverges from shipping run: %#v versus y=%d baseline=%d", component.ID, lineIndex, line, run.y, run.y+run.baselineOffset)
				}
				fragment := line.Fragments[fragmentIndex]
				if fragment.Text != run.text || fragment.X != int64(run.x) {
					t.Errorf("component %s line %d fragment %d = %#v, want shipping run text=%q x=%d", component.ID, lineIndex, fragmentIndex, fragment, run.text, run.x)
				}
			}
			if line.Width != int64(shippingWidth) {
				t.Errorf("component %s line %d width = %d, want shipping shaped width %d", component.ID, lineIndex, line.Width, shippingWidth)
			}
		}
	}
}

func shippingRunWidth(run textRunSource, fs FontSet, cache *fontCache) (geom.Length, error) {
	face, err := cache.get(run.face, fs)
	if err != nil {
		return 0, err
	}
	var advance1000 geom.Length
	for _, glyph := range run.glyphs {
		advance1000 += geom.ScaleRound(geom.Length(glyph.XAdvance), 1000, int64(face.UnitsPerEm()))
	}
	return geom.ScaleRound(advance1000, int64(run.fontSize), 1000), nil
}

func TestCanvasTextPaintRejectsDerivedCoordinatesOutsideTheJSRange(t *testing.T) {
	if _, err := canvasLineTop(geom.Length(designer.MaxCanvasMillipoints), 1, 1); err == nil {
		t.Fatal("overflowing later-line origin was accepted")
	}
	if _, err := canvasDerivedSum(geom.Length(designer.MaxCanvasMillipoints), 1); err == nil {
		t.Fatal("overflowing baseline was accepted")
	}
}

// TestCanvasTextPaintHonoursATypedBreak is the canvas half of D-7.1.3's
// "every caller" — the caller the intent contract names alongside the
// text element and the two table-cell paths, and the one Epic 5's
// canvas/PDF agreement invariant binds: the designer must see the
// breaks the PDF will take, because the browser never measures text and
// never computes a break itself.
//
// The subject is deliberately an element with NO declared width. Before
// Story 7.1 that path returned exactly one line unconditionally, so it
// is the canvas's own instance of packLines' maxWidth <= 0 hazard, and
// a test using a width-bearing element would not reach it.
func TestCanvasTextPaintHonoursATypedBreak(t *testing.T) {
	const tplJSON = `{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":0,"height":60,"value":"Clause 1.\nClause 2.","style":{"fontFamily":"body","fontSize":12}}]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":2}`
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Components) == 0 || projection.Components[0].TextPaint == nil {
		t.Fatalf("missing text paint projection: %#v", projection.Components)
	}
	// PRESENCE PRECONDITION: width 0, which is how "no declared box"
	// reaches packLines (render.go and page_setup.go both gate boxWidth
	// on Width.Set && !Width.Null, and an absent width is always 0).
	// That is the path which used to return exactly one line whatever
	// the value contained.
	if w := projection.Components[0].Width; w != 0 {
		t.Fatalf("precondition: the component reports width %d — this test must exercise the NO-declared-box path", w)
	}

	paint := projection.Components[0].TextPaint
	if len(paint.Lines) != 2 {
		t.Fatalf("the canvas projected %d line(s) for a value carrying one typed break, want 2 — the designer must see the break the PDF will take (Epic 5's canvas/PDF agreement)", len(paint.Lines))
	}
	for i, ln := range paint.Lines {
		if len(ln.Fragments) == 0 {
			t.Errorf("projected line %d carries no fragments", i)
		}
		if ln.Width <= 0 {
			t.Errorf("projected line %d has width %d, want a positive measured width", i, ln.Width)
		}
	}
	if got := paint.Lines[1].Top - paint.Lines[0].Top; got != paint.Lines[0].Advance {
		t.Errorf("the two projected lines are %d mp apart, want the engine's own Advance of %d — the browser must never derive the second line's origin itself", got, paint.Lines[0].Advance)
	}
	if paint.Overflow {
		t.Error("the projection reports overflow for an element with no declared width — there is no bound to overflow")
	}

	// THE NEGATIVE CONTROL. The same value with the line feed replaced
	// by a space, same geometry: one line. Without it, "2 lines" could
	// not be told from a canvas that breaks on something else.
	control, err := ParseTemplate([]byte(strings.Replace(tplJSON, `Clause 1.\nClause 2.`, `Clause 1. Clause 2.`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	cp, err := canvasWithTextPaint(control, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(cp.Components[0].TextPaint.Lines); n != 1 {
		t.Fatalf("the control (a space in place of the line feed) projected %d line(s), want 1 — so the two lines above are the typed break and nothing else", n)
	}
}
