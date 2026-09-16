package folio8

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

type groupMember struct {
	band         designer.CanvasBand
	element      *template.Element
	windowOrigin geom.Length
	windowHeight geom.Length
	// page is the designed page holding a content element; 0 for the page
	// header and footer, which belong to no page.
	page int
}

// Project once per operation; looking up every member through findComponent
// would repeat pagination and text projection for each selected ID.
func groupMemberIndex(t *Template, fonts ...FontSet) (map[string]groupMember, error) {
	var projection designer.CanvasProjection
	var err error
	if len(fonts) > 0 {
		projection, err = canvasWithTextPaint(t, fonts[0])
	} else {
		projection, err = canvas(t)
	}
	if err != nil {
		return nil, err
	}
	bands := map[string][]*template.Band{bandPageHeader: {&t.doc.Bands.PageHeader}, bandContent: t.doc.ContentBands(), bandPageFooter: {&t.doc.Bands.PageFooter}}
	members := make(map[string]groupMember, len(projection.Components))
	for _, band := range projection.Bands {
		for page, source := range bands[band.Name] {
			for index := range source.Elements {
				element := &source.Elements[index]
				origin := geom.Length(0)
				windowHeight := geom.Length(projection.ContentWindowHeight)
				if band.Name == bandContent {
					// SPEC-multi-pages: origins are page-local, so only this
					// element's own page's windows are its candidates.
					for index, candidate := range projection.ContentWindowOrigins {
						if canvasWindowPage(projection, index) != page {
							continue
						}
						if geom.Length(candidate) > element.Y {
							break
						}
						origin = geom.Length(candidate)
						windowHeight = geom.Length(projection.ContentWindowHeight)
						if index+1 < len(projection.ContentWindowOrigins) && canvasWindowPage(projection, index+1) == page {
							windowHeight = min(windowHeight, geom.Length(projection.ContentWindowOrigins[index+1])-origin)
						}
					}
				}
				members[string(element.ID)] = groupMember{band: band, element: element, windowOrigin: origin, windowHeight: windowHeight, page: page}
			}
		}
	}
	return members, nil
}

// previewComponentMove accepts exactly the atomic movement command vocabulary.
// It is read-only and shares its solver with the public mutation door.
// Supply the same FontSet used by CanvasWithTextPaint when constraining to
// displayed windows; without fonts the canvas only has its fallback window.
func previewComponentMove(t *Template, command []byte, fonts ...FontSet) (designer.ComponentMove, error) {
	if t == nil {
		return designer.ComponentMove{}, errNilTemplate
	}
	if err := refuseDuplicateCommandKeys(command, componentCommandPath); err != nil {
		return designer.ComponentMove{}, err
	}
	var raw map[string]json.RawMessage
	d := json.NewDecoder(bytes.NewReader(command))
	var trailing any
	if d.Decode(&raw) != nil || d.Decode(&trailing) != io.EOF {
		return designer.ComponentMove{}, fmt.Errorf("folio8: group move is malformed")
	}
	solved, err := solveComponentMove(t, raw, fonts...)
	return solved.move, err
}

// solvedMove is a validated group move: the ids, the accepted translation and,
// for SPEC-multi-pages story 3, the page the members move to when that is not
// the page they are on.
type solvedMove struct {
	ids       []string
	move      designer.ComponentMove
	crossPage bool
	target    int
}

func solveComponentMove(t *Template, raw map[string]json.RawMessage, fonts ...FontSet) (solvedMove, error) {
	fail := func(id, path, message string) (solvedMove, error) {
		return solvedMove{}, componentFailure(id, path, message)
	}
	fields := 8
	if _, present := raw["page"]; present {
		fields++
	}
	constrain := false
	if value, present := raw["constrainToWindow"]; present {
		fields++
		if bytes.Equal(value, []byte("null")) || json.Unmarshal(value, &constrain) != nil {
			return fail("", "component.constrainToWindow", "constrainToWindow must be a boolean")
		}
	}
	if componentFields(raw, fields) != nil || !equalNumber(raw["version"], "1") || string(raw["kind"]) != `"moveComponents"` {
		return fail("", "component.move", "group move has unknown or missing fields")
	}
	var revision *uint64
	if json.Unmarshal(raw["expectedRevision"], &revision) != nil || revision == nil || *revision > uint64(designer.MaxCanvasMillipoints) {
		return fail("", "component.move", "group move requires a safe revision")
	}
	var ids []string
	if json.Unmarshal(raw["ids"], &ids) != nil || len(ids) == 0 {
		return fail("", "component.ids", "group move requires component ids")
	}
	ref, err := commandString(raw, "referenceId")
	if err != nil || !slices.Contains(ids, ref) {
		return fail(ref, "component.referenceId", "the movement reference must belong to the selection")
	}
	snap, err := commandBool(raw, "snap")
	if err != nil || bytes.Equal(raw["snap"], []byte("null")) {
		return fail("", "component.snap", "snap must be a boolean")
	}
	dx, err := componentLength(raw, "dx", false)
	if err != nil {
		return fail("", "component.dx", err.Error())
	}
	dy, err := componentLength(raw, "dy", false)
	if err != nil {
		return fail("", "component.dy", err.Error())
	}
	target, hasPage, err := optionalPageField(t, raw)
	if err != nil {
		return solvedMove{}, err
	}
	bound := geom.Length(designer.MaxCanvasMillipoints)
	minX, maxX, minY, maxY := -bound, bound, -bound, bound
	var refX, refY geom.Length
	seen := make(map[string]bool, len(ids))
	members, err := groupMemberIndex(t, fonts...)
	if err != nil {
		return solvedMove{}, err
	}
	// SPEC-multi-pages story 3 (D-3.1): with a target page, every member must be
	// a content element on one page. When that page is the target, the move is
	// today's; otherwise the members move to the target page together.
	crossPage := false
	if hasPage {
		source := -1
		for _, id := range ids {
			member, found := members[id]
			if !found {
				return fail(id, "component.id", "component was not found")
			}
			if member.band.Name != bandContent {
				return solvedMove{}, contentOnlyPage(member.band.Name, id)
			}
			if source >= 0 && member.page != source {
				return fail(id, pagesPath, "the selection is on more than one page, so it cannot move to another page")
			}
			source = member.page
		}
		crossPage = source != target
	}
	for _, id := range ids {
		if seen[id] {
			return fail(id, "component.ids", "group move ids must be unique")
		}
		seen[id] = true
		member, found := members[id]
		if !found {
			return fail(id, "component.id", "component was not found")
		}
		band, element := member.band, member.element
		width, height := projectedSize(*element)
		if err := containComponent(band, element.X, element.Y, width, height); err != nil {
			return fail(id, "component.geometry", err.Error())
		}
		// Bound both origins and derived far edges before adding any delta.
		if element.X > bound-width || element.Y > bound-height {
			return fail(id, "component.geometry", "component exceeds safe geometry")
		}
		minX = max(minX, -element.X)
		maxX = min(maxX, geom.Length(band.Width)-width-element.X)
		minY = max(minY, -element.Y)
		maxY = min(maxY, bound-height-element.Y)
		if slices.Contains(bandsCappingVertically, band.Name) {
			maxY = min(maxY, geom.Length(band.Height)-height-element.Y)
		}
		if constrain && band.Name == bandContent && !crossPage {
			// Preserve an existing overflow rather than normalizing authored
			// geometry. Every range contains zero, including oversized/orphan
			// components, so a group always has a feasible common displacement.
			low := min(member.windowOrigin-element.Y, 0)
			high := max(member.windowOrigin+member.windowHeight-height-element.Y, 0)
			minY = max(minY, low)
			maxY = min(maxY, high)
		}
		if id == ref {
			refX, refY = element.X, element.Y
		}
	}
	// A returned-to-start gesture is a no-op even for an off-grid reference.
	solved := solvedMove{ids: ids, crossPage: crossPage, target: target}
	if dx == 0 && dy == 0 && !crossPage {
		return solved, nil
	}
	if crossPage {
		// A move to another page lands where it is dropped: it is not clamped
		// into the target band. It snaps, and the mutation refuses a drop
		// that leaves the target band, naming the element.
		minX, maxX, minY, maxY = -bound, bound, -bound, bound
	}
	accept := func(proposed, low, high, origin geom.Length) geom.Length {
		legal := min(max(proposed, low), high)
		if !snap {
			return legal
		}
		grid := geom.Length(designer.GridIncrement)
		// The reference's feasible origins are nonnegative. Find the first and
		// last grid points; clamp the nearest grid point into that interval.
		first := ((origin + low + grid - 1) / grid) * grid
		last := ((origin + high) / grid) * grid
		if first > last {
			return legal
		}
		nearest, _ := snapToGrid(origin + legal)
		return min(max(nearest, first), last) - origin
	}
	solved.move = designer.ComponentMove{DX: int64(accept(dx, minX, maxX, refX)), DY: int64(accept(dy, minY, maxY, refY))}
	return solved, nil
}

func moveComponents(t *Template, raw map[string]json.RawMessage, fonts ...FontSet) (designer.CanvasProjection, error) {
	solved, err := solveComponentMove(t, raw, fonts...)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	ids, move := solved.ids, solved.move
	before, err := SerializeTemplate(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	working, err := ParseTemplate(before)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	members, err := groupMemberIndex(working)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if solved.crossPage {
		if err := moveComponentsToPage(working, members, ids, move, solved.target); err != nil {
			return designer.CanvasProjection{}, err
		}
		return installMovedComponents(t, working)
	}
	for _, id := range ids {
		member := members[id]
		band, element := member.band, member.element
		x, y := element.X+geom.Length(move.DX), element.Y+geom.Length(move.DY)
		width, height := projectedSize(*element)
		if err := containComponent(band, x, y, width, height); err != nil {
			return designer.CanvasProjection{}, componentFailure(id, "component.geometry", err.Error())
		}
		// spec-section-break: any member across the break refuses the whole
		// group, before anything is installed.
		candidate := *element
		candidate.X, candidate.Y = x, y
		if err := refuseSectionBreakStraddle(working, band.Name, candidate, "component.geometry"); err != nil {
			return designer.CanvasProjection{}, err
		}
		element.X, element.Y = x, y
	}
	return installMovedComponents(t, working)
}

// installMovedComponents installs a moved working copy through canonical
// bytes, so the caller's template changes only when every check passed.
func installMovedComponents(t, working *Template) (designer.CanvasProjection, error) {
	canonical, err := SerializeTemplate(working)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	installed, err := ParseTemplate(canonical)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	projection, err := canvas(installed)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	t.doc, t.derivedFooters = installed.doc, installed.derivedFooters
	return projection, nil
}

// moveComponentsToPage moves every member to page `target` of working, as one
// mutation: each keeps its id and lands at its own position plus move, in the
// target page's column. Every check runs before anything is spliced, so a
// refusal leaves working untouched.
func moveComponentsToPage(working *Template, members map[string]groupMember, ids []string, move designer.ComponentMove, target int) error {
	moving := make(map[string]bool, len(ids))
	for _, id := range ids {
		moving[id] = true
	}
	bound := geom.Length(designer.MaxCanvasMillipoints)
	positions := make(map[string][2]geom.Length, len(ids))
	for _, id := range ids {
		member := members[id]
		element := member.element
		x, y := element.X+geom.Length(move.DX), element.Y+geom.Length(move.DY)
		width, height := projectedSize(*element)
		if err := containComponent(member.band, x, y, width, height); err != nil {
			return componentFailure(id, "component.geometry", err.Error())
		}
		if y > bound-height {
			return componentFailure(id, "component.geometry", "component exceeds safe geometry")
		}
		// The target page's own break judges a move onto that page.
		candidate := *element
		candidate.X, candidate.Y = x, y
		if err := refuseSectionBreakStraddleOnPage(working, bandContent, target, candidate, "component.geometry"); err != nil {
			return err
		}
		positions[id] = [2]geom.Length{x, y}
	}
	// SPEC constraint: a keep-together group never spans pages, so a move that
	// leaves any member of a moved group behind is refused as a whole.
	bands := working.doc.ContentBands()
	for _, id := range ids {
		tag := members[id].element.KeepTogether
		if !tag.Set || tag.Null {
			continue
		}
		for page, band := range bands {
			for _, el := range band.Elements {
				if moving[string(el.ID)] || !el.KeepTogether.Set || el.KeepTogether.Null || el.KeepTogether.Value != tag.Value {
					continue
				}
				return componentFailure(id, pagesPath, fmt.Sprintf("keepTogether group %q would have members on %s and %s — a keep-together group cannot span pages; move every member of the group together", tag.Value, working.doc.PageField(target), working.doc.PageField(page)))
			}
		}
	}
	// Splice: the members leave their page in document order and join the end
	// of the target page's elements in that same order.
	var moved []template.Element
	for page, band := range bands {
		if page == target {
			continue
		}
		kept := make([]template.Element, 0, len(band.Elements))
		for _, el := range band.Elements {
			if !moving[string(el.ID)] {
				kept = append(kept, el)
				continue
			}
			at := positions[string(el.ID)]
			el.X, el.Y = at[0], at[1]
			moved = append(moved, el)
		}
		band.Elements = kept
	}
	bands[target].Elements = append(append([]template.Element{}, bands[target].Elements...), moved...)
	return nil
}
