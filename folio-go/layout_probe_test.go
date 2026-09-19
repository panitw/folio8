package folio8

import (
	"testing"

	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// elementLayout is one text element's Story 2.4 layout, recovered for
// assertion purposes.
//
// IT IS BUILT FROM THE SAME FUNCTIONS collectTextRuns USES — shapeSegments,
// atomicSpansFor, text.Opportunities, packLines — deliberately, because
// what these tests need to assert is per-element structure (which runes
// on which line, which spans were atomic) that the produced PDF flattens
// into a list of positioned runs. The artifact-level properties that CAN
// be read off the PDF are asserted off the PDF instead, in
// TestWrappedTextSemanticAcceptance: baseline count, embedded faces,
// beginbfchar sizes.
//
// So this is a probe, not a second implementation, and the split is
// stated rather than left for a reader to infer: anything assertable on
// the produced bytes is asserted there.
type elementLayout struct {
	id        string
	text      string
	box       geom.Length
	fontSize  geom.Length
	fullWidth geom.Length
	atomic    []text.Span
	ops       []text.Opportunity
	lines     []wrappedLine
}

// elementLayouts lays out every text element of tpl against data, in
// document order.
func elementLayouts(t *testing.T, tpl *Template, dataJSON string) []elementLayout {
	t.Helper()

	d, err := bind.DecodeData([]byte(dataJSON))
	if err != nil {
		t.Fatalf("decode data: %v", err)
	}
	p, err := bind.DecodeParams([]byte(`{}`))
	if err != nil {
		t.Fatalf("decode params: %v", err)
	}

	fs := testShippedFontSet()
	cache := newFontCache()
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}

	var out []elementLayout
	for _, b := range bands {
		for _, el := range b.band.Elements {
			if el.Type != template.ElementText {
				continue
			}
			if !el.Value.Set || el.Value.Null || el.Value.Value == "" {
				continue
			}
			boundText, subs, _, berr := bind.BindTextSpans(el.Value.Value, d, p, testFormatContext(), string(el.ID))
			if berr != nil {
				t.Fatalf("bind %s: %v", el.ID, berr)
			}
			if boundText == "" {
				continue
			}
			chain, styled, cerr := fontChain(tpl, el)
			if cerr != nil {
				t.Fatalf("fontChain %s: %v", el.ID, cerr)
			}
			fontSize := defaultFontSizePt
			if el.Style.Set && !el.Style.Null && el.Style.Value.FontSize.Set && !el.Style.Value.FontSize.Null {
				fontSize = el.Style.Value.FontSize.Value
			}
			segs, _, serr := shapeSegments(string(el.ID), chain, styled, boundText, fs, cache, breaksAreConsumed)
			if serr != nil {
				t.Fatalf("shapeSegments %s: %v", el.ID, serr)
			}
			n := len([]rune(boundText))
			atomic := atomicSpansFor(tpl.doc.UnbreakableValues, subs)
			ops := text.Opportunities(text.Dictionary(), boundText, atomic)
			box := geom.Length(0)
			if el.Width.Set && !el.Width.Null {
				box = el.Width.Value
			}
			out = append(out, elementLayout{
				id:        string(el.ID),
				text:      boundText,
				box:       box,
				fontSize:  fontSize,
				fullWidth: measureRuneRange(segs, 0, n, fontSize),
				atomic:    atomic,
				ops:       ops,
				lines:     packLines(segs, ops, n, fontSize, box),
			})
		}
	}
	return out
}
