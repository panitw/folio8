package pdf

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// TestAppendColorChannels is Story 4.1, AC5's test-owned expectation
// table: every literal here is written out, never computed by calling
// the function under test (D-000.68).
//
// The mutation this test is DESIGNED to redden under (recorded, run, and
// restored in the story's Delivery Log): change ScaleRound's rounding to
// truncation at this call site — #808080 (128) must emit "0.502", not
// the truncated "0.501".
func TestAppendColorChannels(t *testing.T) {
	cases := []struct {
		name    string
		channel uint8
		want    string
	}{
		{"#000000", 0x00, "0"},
		{"#FFFFFF", 0xFF, "1"},
		{"#808080", 0x80, "0.502"},
		{"#010101", 0x01, "0.004"},
		{"#7F7F7F", 0x7F, "0.498"},
	}
	for _, c := range cases {
		got := string(appendColorChannels(nil, pagemodel.Color{R: c.channel, G: c.channel, B: c.channel}))
		want := c.want + " " + c.want + " " + c.want
		if got != want {
			t.Errorf("appendColorChannels(%s) = %q, want %q", c.name, got, want)
		}
	}
}

// AC5's build-time half, CORRECTED (finisher fix, Story 4.1 review
// Finding 4 / Major): the deleted TestAppendColorChannelsNoFloat
// claimed introducing `float64(c.R)/255.0` at appendColorChannels'
// call site "does not compile under this module's float ban" — that
// claim is FALSE. Measured: `go build ./...` SUCCEEDS with
// `geom.Length(float64(c.R)/255.0*1000)` substituted at
// rectdoc.go:106 (Go's type system permits a float64 expression
// converted to an integer type; a syntactic float-literal ban is not
// what enforces AD-1/AD-23 here). The actual instrument that catches
// it is a DIFFERENT MODULE's cross-module, type-resolved scan:
// `lint`'s TestFloatTypedProductionScan (from `lint/`,
// `go test ./... -run TestFloatTypedProductionScan`) reports
// "internal/pdf/rectdoc.go:106:38: this value expression has
// floating-point type float64 (resolved by go/types, not spelled in
// the source)", and TestFloatTypedTestScopeInventory reds on the
// site-count mismatch (got 6, want 5) — both re-run and confirmed
// during this fix. The property IS protected (lint is inside the
// three-module gate, D-000.64); only the RECORD naming "the compiler"
// as the anchor was wrong, and the two deleted rows of
// TestAppendColorChannelsNoFloat asserted nothing about floats at all
// (they duplicated TestAppendColorChannels' own #000000/#FFFFFF rows
// above and stayed green under the truncation mutation that reddens
// the real test) — a green line that read as "no float, proven"
// while proving nothing (Finding 11). No replacement test is added
// here: `lint`'s two named tests already are the correct,
// cross-module anchor, and a duplicate the-compiler-anchored test in
// this file would misrepresent it a second time.

// TestAppendRectContentStreamFill is AC3/AC6's byte-level assertion for a
// filled, unstroked rect: exactly one "re f" path, bracketed in q/Q, with
// no RG/w emitted (there is no stroke).
func TestAppendRectContentStreamFill(t *testing.T) {
	page := pagemodel.Page{
		Width: 600000, Height: 800000,
		Rects: []pagemodel.Rect{
			{X: 10000, Y: 20000, W: 100000, H: 30000, HasFill: true, Fill: pagemodel.Color{R: 255, G: 0, B: 0}},
		},
	}
	got := string(appendRectContentStream(nil, page))
	if !strings.Contains(got, "1 0 0 rg\n") {
		t.Errorf("missing fill colour operator, got %q", got)
	}
	if !strings.Contains(got, " re f\n") {
		t.Errorf("missing fill path, got %q", got)
	}
	if strings.Contains(got, "RG") || strings.Contains(got, " w\n") {
		t.Errorf("unstroked rect must emit no stroke operator, got %q", got)
	}
	if !strings.HasPrefix(got, "q\n") || !strings.HasSuffix(got, "Q\n") {
		t.Errorf("fill must be bracketed in q/Q, got %q", got)
	}
}

// TestAppendRectContentStreamStrokeEdgesSubset asserts AC3's edges
// mutation directly: style.border.edges naming a strict subset strokes
// EXACTLY those edges — here top and bottom only, never left or right.
// Discriminating mutation (run, recorded, restored): hardcode all four
// edges regardless of r.Edges → this test reddens, counting exactly two
// "m "/"l " subpaths where four would appear otherwise.
func TestAppendRectContentStreamStrokeEdgesSubset(t *testing.T) {
	page := pagemodel.Page{
		Width: 600000, Height: 800000,
		Rects: []pagemodel.Rect{
			{
				X: 0, Y: 0, W: 100000, H: 50000,
				HasStroke: true, Stroke: pagemodel.Color{R: 0, G: 0, B: 0}, StrokeWidth: 1000,
				Edges: pagemodel.RectEdges{Top: true, Bottom: true},
			},
		},
	}
	got := string(appendRectContentStream(nil, page))
	if n := strings.Count(got, "m "); n != 2 {
		t.Fatalf("edges subset {top,bottom} must draw exactly 2 subpaths, got %d in %q", n, got)
	}
	if !strings.Contains(got, "RG\n") {
		t.Errorf("missing stroke colour operator, got %q", got)
	}
	if !strings.Contains(got, " w\n") {
		t.Errorf("missing line width operator, got %q", got)
	}
	if strings.Contains(got, "rg\n") {
		t.Errorf("no fill was set; must not emit a fill colour operator, got %q", got)
	}
}

// TestAppendRectContentStreamNoStrokeWhenNoEdges: HasStroke true but every
// edge false emits NOTHING for the stroke half — no stray q/Q, no RG, no
// "w" left in the graphics state for the next primitive.
func TestAppendRectContentStreamNoStrokeWhenNoEdges(t *testing.T) {
	page := pagemodel.Page{
		Width: 600000, Height: 800000,
		Rects: []pagemodel.Rect{
			{X: 0, Y: 0, W: 100000, H: 50000, HasStroke: true, Stroke: pagemodel.Color{R: 1, G: 2, B: 3}, StrokeWidth: 1000},
		},
	}
	got := string(appendRectContentStream(nil, page))
	if got != "" {
		t.Errorf("a Rect with HasStroke but no edges set must draw nothing, got %q", got)
	}
}

// TestAppendRectContentStreamAbsentDrawsNothing (R6): a Rect with neither
// HasFill nor HasStroke draws zero bytes.
func TestAppendRectContentStreamAbsentDrawsNothing(t *testing.T) {
	page := pagemodel.Page{
		Width: 600000, Height: 800000,
		Rects: []pagemodel.Rect{{X: 0, Y: 0, W: 100000, H: 50000}},
	}
	got := string(appendRectContentStream(nil, page))
	if got != "" {
		t.Errorf("an absent-style rect must draw nothing, got %q", got)
	}
}

// TestAppendRectContentStreamEmptyPageProducesEmptyStream confirms every
// pre-4.1 document (page.Rects == nil) appends zero bytes — the property
// that keeps every existing golden byte-identical.
func TestAppendRectContentStreamEmptyPageProducesEmptyStream(t *testing.T) {
	got := string(appendRectContentStream(nil, pagemodel.Page{}))
	if got != "" {
		t.Errorf("nil Rects must append nothing, got %q", got)
	}
}

// TestAppendRectContentStreamEdgeOperandsArePostfix pins the ORDER of a
// stroked edge's operands, which no test read until Story 9.1: a PDF
// content stream is postfix, so an edge is "<x1> <y1> m <x2> <y2> l".
// Story 4.1 shipped it prefix ("m <x1> <y1> l <x2> <y2>"), which a reader
// parses as an m with no operands followed by every later pair binding one
// operator late — four edges came out as a chain of diagonals between
// opposite corners, the "crossed box" a rectangle element made visible.
// The pre-existing edge tests count "m " occurrences, so both spellings
// satisfied them equally; this one fails on the prefix spelling.
func TestAppendRectContentStreamEdgeOperandsArePostfix(t *testing.T) {
	page := pagemodel.Page{
		Width: 600000, Height: 800000,
		Rects: []pagemodel.Rect{{
			X: 0, Y: 0, W: 100000, H: 50000,
			HasStroke: true, Stroke: pagemodel.Color{}, StrokeWidth: 1000,
			Edges: pagemodel.RectEdges{Top: true},
		}},
	}
	got := string(appendRectContentStream(nil, page))
	if !strings.Contains(got, "0 800 m 100 800 l\n") {
		t.Errorf("stroked edge = %q, want the subpath spelled postfix: %q", got, "0 800 m 100 800 l")
	}
	// The prefix spelling is not merely absent by accident: an "m" that
	// leads its own operands is the defect itself.
	if strings.Contains(got, "m 0 800") {
		t.Errorf("stroked edge is spelled prefix (operator before its operands), got %q", got)
	}
}
