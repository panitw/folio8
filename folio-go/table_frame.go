package folio8

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// tableFrame is SPEC-table-rules' perimeter/interior split for ONE table:
// the frame its own `style.border`/`style.background` paint (§1), the
// interior lines `table.rules` draws (§2), and the per-page floor
// `table.minHeight` puts under each slice (§3).
//
// IT IS NOT A PAGINATION ITEM, and that is the whole lesson of the first
// attempt. A frame's bottom is its slice's bottom, and a slice's extent is
// known only once pages are assigned — so the frame is described here,
// before pagination, and DRAWN per page slice afterwards (applyTableFrames),
// from the slices layout.Paginate reports. Built before, the only item that
// could carry it is one as tall as the whole table, which pagination
// correctly treats as a group that fits no page.
type tableFrame struct {
	// box carries the frame's X, W, fill and stroke. Y and H are set per
	// slice. Built through buildCellRectWithBackgroundField — the same
	// builder every table chrome rect and every element box uses — so a
	// colour is refused with the same code and a border resolves the same
	// width, colour and edges it does everywhere else.
	box pagemodel.Rect

	// columnRules are the X of every INTERIOR column boundary: never the
	// table's own left or right edge, which belong to the frame.
	columnRules []geom.Length
	ruleRows    bool
	rule        pagemodel.Rect // stroke, colour and width; geometry per line

	// headerStrokesBottom: `headerStyle.border` already strokes the header's
	// bottom edge, so a `rows` rule skips that boundary — the header wins.
	headerStrokesBottom bool

	// minHeight is the per-slice floor; 0 is none.
	minHeight geom.Length
}

// buildTableFrame returns nil when the table declares no frame paint, no
// rules and no floor — every document written before these keys existed,
// which is what keeps them byte-identical: a nil frame makes no slice
// request, so pagination and the page loop are untouched.
func buildTableFrame(el template.Element, tbl template.TableExt, hs resolvedHeaderStyle, geometry layout.TableGeometry) (*tableFrame, error) {
	if len(geometry.Columns) == 0 {
		return nil, nil
	}
	var style template.Style
	if el.Style.Set && !el.Style.Null {
		style = el.Style.Value
	}
	hasBackground := style.Background.Set && !style.Background.Null
	// borderPaints is element_box.go's reading of "this border puts ink on
	// the page", so a table cannot disagree with an element box about it.
	hasBorder := borderPaints(style.Border)
	hasRules := tbl.Rules.Set && !tbl.Rules.Null
	hasFloor := tbl.MinHeight.Set && !tbl.MinHeight.Null && tbl.MinHeight.Value > 0
	if !hasBackground && !hasBorder && !hasRules && !hasFloor {
		return nil, nil
	}

	first := geometry.Columns[0]
	last := geometry.Columns[len(geometry.Columns)-1]
	x := first.X
	w := last.X + last.Width - x

	box, err := buildCellRectWithBackgroundField(string(el.ID), x, 0, w, 0,
		hasBackground, style.Background.Value, "style.background", hasBorder, style.Border.Value)
	if err != nil {
		return nil, err
	}
	f := &tableFrame{
		box: box,
		// The SAME paints predicate the frame uses (borderPaints), so a
		// header border that puts no ink on the page suppresses no rule.
		headerStrokesBottom: borderPaints(template.Presence[template.Border]{Set: hs.hasBorder, Value: hs.border}) && resolvedBorderEdges(hs.border).Bottom,
	}
	if hasFloor {
		f.minHeight = tbl.MinHeight.Value
	}

	if hasRules {
		rules := tbl.Rules.Value
		// A rules colour is refused at LOAD when it is not #RRGGBB, like
		// every other colour in the format, so this decode cannot fail
		// for a loaded template: the error arm is an unreachable guard.
		asBorder := template.Border{Width: rules.Width, Color: rules.Color}
		colorHex := resolvedBorderColor(asBorder)
		stroke, ok := parseHexColor(colorHex)
		if !ok {
			return nil, fmt.Errorf("folio8: Render: element %s: rules.color %q is not a #RRGGBB colour, which the loader refuses (unreachable)", el.ID, colorHex)
		}
		f.rule = pagemodel.Rect{HasStroke: true, Stroke: stroke, StrokeWidth: resolvedBorderWidth(asBorder)}
		if rules.Between.Set && !rules.Between.Null {
			for _, b := range rules.Between.Value {
				switch b {
				case "columns":
					for i := 1; i < len(geometry.Columns); i++ {
						f.columnRules = append(f.columnRules, geometry.Columns[i].X)
					}
				case "rows":
					f.ruleRows = true
				}
			}
		}
	}
	return f, nil
}

// sliceRequest is what one of this table's pagination items asks of
// layout.Paginate: report my per-page slices, and floor them.
func (f *tableFrame) sliceRequest() layout.SliceRequest {
	if f == nil {
		return layout.SliceRequest{}
	}
	return layout.SliceRequest{Present: true, MinHeight: f.minHeight}
}

// maxVerticalMetrics is the member-wise maximum of two line models — the
// header's line geometry across every column, never one column's.
func maxVerticalMetrics(a, b verticalMetrics) verticalMetrics {
	out := a
	if b.FirstBaseline > out.FirstBaseline {
		out.FirstBaseline = b.FirstBaseline
	}
	if b.Advance > out.Advance {
		out.Advance = b.Advance
	}
	if b.LastDescent > out.LastDescent {
		out.LastDescent = b.LastDescent
	}
	return out
}

// framedRect is one rect on a page being assembled, with the table source
// it came from (src < 0: not a table's) and whether it is a repeated header.
type framedRect struct {
	rect   pagemodel.Rect
	src    int
	repeat bool
}

// frameSlice is one table's extent on one page, in PAGE space.
type frameSlice struct {
	elementID   string
	top, bottom geom.Length
}

// applyTableFrames draws each slice's frame and interior rules into one
// page's rects and returns the finished list.
//
// PLACEMENT. A frame's fill goes immediately BEFORE the table's first rect on
// the page, so row and header fills paint over it; its stroke and the rules
// go immediately AFTER the table's last rect, so they paint over row fills.
// Rects paint before any text (internal/pdf), so all of it is under the text.
//
// THE PERIMETER BELONGS TO THE FRAME. Where the frame strokes an edge, any
// header or cell edge lying on that same line is cleared, so the table's
// outer edges are stroked once.
func applyTableFrames(entries []framedRect, sources []tableRectSource, slices []frameSlice) []pagemodel.Rect {
	if len(slices) == 0 {
		out := make([]pagemodel.Rect, len(entries))
		for i, e := range entries {
			out[i] = e.rect
		}
		return out
	}
	before := map[int][]pagemodel.Rect{}
	after := map[int][]pagemodel.Rect{}
	for _, sl := range slices {
		first, last := -1, -1
		var frame *tableFrame
		type region struct {
			top      geom.Length
			isHeader bool
		}
		var regions []region
		lastSrc := -1
		for i, e := range entries {
			// A repeated header is part of the slice (owner-ruled: it lies
			// inside the frame), so it counts for the fill's placement and
			// as the slice's first row region.
			if e.src < 0 || sources[e.src].elementID != sl.elementID || sources[e.src].frame == nil {
				continue
			}
			frame = sources[e.src].frame
			if first < 0 {
				first = i
			}
			last = i
			if e.src != lastSrc {
				regions = append(regions, region{top: e.rect.Y, isHeader: sources[e.src].isHeaderRow})
				lastSrc = e.src
			}
		}
		if frame == nil {
			continue
		}
		box := frame.box
		box.Y, box.H = sl.top, sl.bottom-sl.top

		if box.HasFill {
			fill := box
			fill.HasStroke, fill.Stroke, fill.StrokeWidth, fill.Edges = false, pagemodel.Color{}, 0, pagemodel.RectEdges{}
			before[first] = append(before[first], fill)
		}
		var tail []pagemodel.Rect
		if box.HasStroke {
			stroke := box
			stroke.HasFill, stroke.Fill = false, pagemodel.Color{}
			tail = append(tail, stroke)
			for i := range entries {
				e := &entries[i]
				if e.src < 0 || sources[e.src].elementID != sl.elementID || !e.rect.HasStroke {
					continue
				}
				suppressPerimeterEdges(&e.rect, stroke)
			}
		}
		for _, x := range frame.columnRules {
			r := frame.rule
			r.X, r.Y, r.W, r.H = x, sl.top, 0, sl.bottom-sl.top
			r.Edges = pagemodel.RectEdges{Left: true}
			tail = append(tail, r)
		}
		if frame.ruleRows {
			for i := 1; i < len(regions); i++ {
				if regions[i-1].isHeader && frame.headerStrokesBottom {
					continue
				}
				y := regions[i].top
				if y <= sl.top || y >= sl.bottom {
					continue
				}
				r := frame.rule
				r.X, r.Y, r.W, r.H = box.X, y, box.W, 0
				r.Edges = pagemodel.RectEdges{Top: true}
				tail = append(tail, r)
			}
		}
		after[last] = append(after[last], tail...)
	}
	out := make([]pagemodel.Rect, 0, len(entries)+len(before)+len(after))
	for i, e := range entries {
		out = append(out, before[i]...)
		out = append(out, e.rect)
		out = append(out, after[i]...)
	}
	return out
}

// suppressPerimeterEdges clears every edge of r that lies on a line the
// frame itself strokes, within the frame's extent.
func suppressPerimeterEdges(r *pagemodel.Rect, frame pagemodel.Rect) {
	frameRight, frameBottom := frame.X+frame.W, frame.Y+frame.H
	withinX := r.X >= frame.X && r.X+r.W <= frameRight
	withinY := r.Y >= frame.Y && r.Y+r.H <= frameBottom
	if withinX && frame.Edges.Top && r.Edges.Top && r.Y == frame.Y {
		r.Edges.Top = false
	}
	if withinX && frame.Edges.Bottom && r.Edges.Bottom && r.Y+r.H == frameBottom {
		r.Edges.Bottom = false
	}
	if withinY && frame.Edges.Left && r.Edges.Left && r.X == frame.X {
		r.Edges.Left = false
	}
	if withinY && frame.Edges.Right && r.Edges.Right && r.X+r.W == frameRight {
		r.Edges.Right = false
	}
}
