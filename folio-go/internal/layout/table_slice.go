package layout

import "github.com/panitw/folio8/folio-go/internal/geom"

// SliceRequest is carried by a table's own items when the table draws a frame,
// interior rules or a floor (SPEC-table-rules): it asks Paginate to report
// that table's per-page slices, and to floor each one at MinHeight.
//
// The zero value asks for nothing, which is every item of every document that
// declares none of those keys — Paginate's answer for them is unchanged.
type SliceRequest struct {
	Present   bool
	MinHeight geom.Length // 0: no floor
}

// TableSlice is one table's extent on one page, in PAGE space (after the
// page's Shift, the table's own row displacement and any floor push): what a
// caller draws that page's frame and column rules around.
type TableSlice struct {
	ElementID string

	// Top is the top of the slice's header. On the table's own first page
	// that is its header item's top; on a continuation page it is the TOP of
	// the repeated header drawn there (owner-ruled: the repeated header lies
	// inside the frame, so every page matches the first). Where the repeat
	// was suppressed, it is the first row's top.
	Top geom.Length

	// Bottom is max(content bottom, min(Top + MinHeight, the window's
	// content bottom)) — the floor never lifts the content, never passes
	// the window.
	Bottom geom.Length

	// Continuation is true when the table's own header is not on this page.
	Continuation bool
}

// ElementPush is the extra downward displacement one element receives on one
// page because a floored table above it on that page ends lower than its rows
// (SPEC-table-rules §3: "the floor is reserved during pagination, so an element
// following the table on that page starts below the floored bottom").
type ElementPush struct {
	ElementID string
	Amount    geom.Length
}

type span struct{ left, right geom.Length }

type pushKey struct {
	element string
	page    int
}

// tableSlicer holds Paginate's per-table slice bookkeeping, apart from the
// sweep so the sweep changes by a handful of calls.
type tableSlicer struct {
	items         []ColumnItem
	contentBottom geom.Length // page space: contentTop + height

	tables     []string // sliced tables, first-appearance order
	floors     map[string]geom.Length
	colBottom  map[string]geom.Length // floored tables: latest item Bottom
	elementTop map[string]geom.Length
	extent     map[string]span // elements with a known horizontal extent

	placed   []bool
	pageOf   []int
	lastPage map[string]int
	extMemo  map[string]geom.Length
	pushMemo map[pushKey]geom.Length
	shift    func(page int) geom.Length
	reserved func(table string, page int) geom.Length
}

func newTableSlicer(items []ColumnItem, contentBottom geom.Length) *tableSlicer {
	s := &tableSlicer{
		items:         items,
		contentBottom: contentBottom,
		floors:        map[string]geom.Length{},
		colBottom:     map[string]geom.Length{},
		elementTop:    map[string]geom.Length{},
		extent:        map[string]span{},
		placed:        make([]bool, len(items)),
		pageOf:        make([]int, len(items)),
		lastPage:      map[string]int{},
		extMemo:       map[string]geom.Length{},
		pushMemo:      map[pushKey]geom.Length{},
	}
	seen := map[string]bool{}
	for _, it := range items {
		if top, ok := s.elementTop[it.ElementID]; !ok || it.Top < top {
			s.elementTop[it.ElementID] = it.Top
		}
		if it.HasExtent {
			e, ok := s.extent[it.ElementID]
			if !ok || it.Left < e.left {
				e.left = it.Left
			}
			if !ok || it.Right > e.right {
				e.right = it.Right
			}
			s.extent[it.ElementID] = e
		}
		if !it.Slice.Present {
			continue
		}
		if !seen[it.ElementID] {
			seen[it.ElementID] = true
			s.tables = append(s.tables, it.ElementID)
		}
		if it.Slice.MinHeight > 0 {
			s.floors[it.ElementID] = it.Slice.MinHeight
			if b, ok := s.colBottom[it.ElementID]; !ok || it.Bottom > b {
				s.colBottom[it.ElementID] = it.Bottom
			}
		}
	}
	return s
}

// place records that item idx landed on page.
func (s *tableSlicer) place(idx, page int) {
	s.placed[idx] = true
	s.pageOf[idx] = page
	if s.items[idx].Slice.Present {
		if p, ok := s.lastPage[s.items[idx].ElementID]; !ok || page > p {
			s.lastPage[s.items[idx].ElementID] = page
		}
	}
}

// push is the displacement element receives on page: the sum of the floor
// extensions of every floored table that ENDS on that page and lies wholly
// above the element. Zero, without allocation, for a document with no floor.
//
// Every such table is completely placed before the element's first item is
// visited — its items all have a Top strictly above the element's top — so
// the answer is final the first time it is asked.
func (s *tableSlicer) push(element string, page int) geom.Length {
	if len(s.floors) == 0 {
		return 0
	}
	key := pushKey{element, page}
	if v, ok := s.pushMemo[key]; ok {
		return v
	}
	// IN PROGRESS before the loop (review item 6): push -> extension ->
	// slice -> push can otherwise recurse without end when two floored
	// tables each lie below the other (zero-height tables at one y).
	s.pushMemo[key] = 0
	top, ok := s.elementTop[element]
	var total geom.Length
	if ok {
		for _, table := range s.tables {
			if table == element {
				continue
			}
			if _, floored := s.floors[table]; !floored {
				continue
			}
			if lp, placed := s.lastPage[table]; !placed || lp != page || top < s.colBottom[table] {
				continue
			}
			if !s.overlaps(element, table) {
				continue
			}
			total += s.extension(table, page)
		}
	}
	s.pushMemo[key] = total
	return total
}

// overlaps reports whether element's horizontal extent meets table's — a
// growing table moves what lies below it, never a sibling beside it (review
// item 4). An element or table with no known extent is treated as overlapping.
func (s *tableSlicer) overlaps(element, table string) bool {
	e, eok := s.extent[element]
	t, tok := s.extent[table]
	if !eok || !tok {
		return true
	}
	return e.left < t.right && e.right >= t.left
}

// extension is how far table's floor carries its last slice below that
// slice's content.
func (s *tableSlicer) extension(table string, page int) geom.Length {
	if v, ok := s.extMemo[table]; ok {
		return v
	}
	s.extMemo[table] = 0 // in progress: see push
	sl, raw, ok := s.slice(table, page)
	ext := geom.Length(0)
	if ok {
		ext = sl.Bottom - raw
	}
	s.extMemo[table] = ext
	return ext
}

// slice computes table's slice on page, and also its unfloored bottom.
func (s *tableSlicer) slice(table string, page int) (TableSlice, geom.Length, bool) {
	out := TableSlice{ElementID: table, Continuation: true}
	found := false
	push := s.push(table, page)
	shift := s.shift(page)
	for i, it := range s.items {
		if !s.placed[i] || s.pageOf[i] != page || it.ElementID != table || !it.Slice.Present {
			continue
		}
		disp := geom.Length(0)
		if it.Group.Present && it.Group.Key.IsHeader {
			out.Continuation = false
		} else {
			disp = s.reserved(table, page)
		}
		top := it.Top - shift + disp + push
		bottom := it.Bottom - shift + disp + push
		if !found || top < out.Top {
			out.Top = top
		}
		if !found || bottom > out.Bottom {
			out.Bottom = bottom
		}
		found = true
	}
	if !found {
		return TableSlice{}, 0, false
	}
	// A continuation page's repeated header sits directly above the rows,
	// displaced by exactly its own reserved height — so the slice begins at
	// the header's top, and the floor's `t` is that same top.
	if out.Continuation {
		out.Top -= s.reserved(table, page)
	}
	// A clipped group's rects are cut at the content bottom (RectClip), so
	// the slice is too.
	if out.Bottom > s.contentBottom {
		out.Bottom = s.contentBottom
	}
	raw := out.Bottom
	if floor := s.floors[table]; floor > 0 {
		floored := out.Top + floor
		if floored > s.contentBottom {
			floored = s.contentBottom
		}
		if floored > out.Bottom {
			out.Bottom = floored
		}
	}
	return out, raw, true
}

// finish writes TableSlices and ElementPush onto every page. Both stay nil on
// every page of a document that requests no slice.
func (s *tableSlicer) finish(pages []PageAssignment) {
	if len(s.tables) == 0 {
		return
	}
	for p := range pages {
		for _, table := range s.tables {
			if sl, _, ok := s.slice(table, p); ok {
				pages[p].TableSlices = append(pages[p].TableSlices, sl)
			}
		}
		if len(s.floors) == 0 {
			continue
		}
		seen := map[string]bool{}
		for i, it := range s.items {
			if !s.placed[i] || s.pageOf[i] != p || seen[it.ElementID] {
				continue
			}
			seen[it.ElementID] = true
			if amount := s.push(it.ElementID, p); amount > 0 {
				pages[p].ElementPush = append(pages[p].ElementPush, ElementPush{ElementID: it.ElementID, Amount: amount})
			}
		}
	}
}
