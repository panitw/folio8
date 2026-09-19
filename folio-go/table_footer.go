package folio8

// This file is Story 4.5's orphan-tie mechanism (AC5): "a footer that does
// not fit moves to the next page together with at least one preceding data
// row." DECISION-1 (developer's call, D-000.79 §2), re-derived twice during
// this story after the engineering lead's rulings on DECISION-2 changed its
// cost — see the story's Delivery Log for the full re-pricing. The shape
// landed here:
//
// A footer's OWN layout.ItemGroup (table_render.go's
// tableRectSource.chromeRowGroup / textRunSource.lineRowGroup, Index -1) is
// never permanently merged with the preceding data row's group. Instead,
// this file runs layout.Paginate ONCE as a probe; if — and only if — that
// probe would orphan the footer (place it on a later page than the row
// immediately preceding it in the collection), it runs layout.Paginate a
// SECOND time with the footer's group temporarily redirected to the
// preceding row's own Key, for that one call only.
//
// WHY THIS SHAPE, RE-PRICED AGAINST THE RULED "NEVER ERROR" (DECISION-2):
// internal/layout/paginate.go itself is UNTOUCHED — its OverflowError
// contract for an over-tall group is not widened, narrowed or bypassed at
// all, for a footer group or for any other. The "never error, footer-only"
// carve-out required by DECISION-2 therefore lives entirely in THIS file,
// which is the only caller that ever asks Paginate to try the merged
// grouping — an over-tall SINGLE data row (Story 4.6's subject) never goes
// through this second call at all, and so is never able to reach the catch
// below (TestOverTallSingleRowStillOverflows is this claim's red-proof:
// widening the catch to match any OverflowError, not only one produced by
// this file's own merge attempt, is exactly the mutation that test exists
// to catch).
//
// This is DECISION-1 restated, not shape (i) as originally chosen: this
// story's first pass at DECISION-1 picked "the footer joins the preceding
// row's group, unconditionally" on the grounds that it cost zero new
// machinery. The lead's "never error" ruling on DECISION-2 made that
// unconditional merge unacceptable (an over-tall joined group would have to
// either error — ruled out — or bypass Paginate's OverflowError generically
// — which would silently swallow Story 4.6's own over-tall-single-row case
// too). Re-derived: the PROBE-THEN-MERGE shape below keeps the common case
// exactly as cheap (one extra Paginate call, only when a real orphan is
// detected — which is the rare case, not the common one) while keeping the
// exception keyed on footer-ness BY CONSTRUCTION, because the merge is
// something only this file ever asks for.
//
// DECISION-1's FACT-CHECK (Story 4.8, epics.md: "the alternation follows
// row index in the collection, so it does not reset per page"). 4.8's
// striping, whichever mechanism it uses, keys off a ROW-TYPE tag
// (isDataRow/rowIndex, tableRectSource/textRunSource) — never off
// layout.ItemGroup.Key. The footer's row-type tag is isFooterRow/
// isFooterLine, permanently distinct from isDataRow, in EVERY case: the
// non-orphan common case (footer keeps its own Key, Index -1) and the
// orphan case (footer's Key is redirected to the row's Key for ONE
// Paginate call, but its isFooterRow/isFooterLine tag never changes). 4.8
// cannot mistake the footer for a row under either case.
import (
	"errors"

	"github.com/panitw/folio8/folio-go/internal/layout"
)

// footerGroupIndex is the layout.ItemGroupKey.Index a table's FOOTER row
// carries (Story 4.5), distinguishing its group from the header's
// (IsHeader) and from every data row's (Index >= 0, the row's own
// 0-based position in the bound collection). Negative by construction so
// no real row index can ever collide with it. Declared here, once, and
// referenced by tableRectSource.chromeRowGroup and
// textRunSource.lineRowGroup rather than each spelling -1 for itself.
const footerGroupIndex = -1

// footerOrphanTarget names one table's footer-tie candidate: the footer's
// own solo group Key, and the Key of the data row immediately preceding it
// in the bound collection (AC5: "the row that immediately precedes it in
// the collection" — never any other row).
type footerOrphanTarget struct {
	elementID    string
	footerKey    layout.ItemGroupKey
	precedingKey layout.ItemGroupKey
}

// footerOrphanTargetsFrom builds one footerOrphanTarget per table that
// carries a footer group AND at least one data-row group, read from the
// layout.ColumnItems THEMSELVES — never from either producer's own source
// structs.
//
// WHY FROM THE ITEMS AND NOT FROM tableRects, which was this function's
// first shape. AC5's part (a) is D-000.80's founding accident aimed at
// exactly this: a row's CHROME rect is a single item spanning the whole
// row, so anything that reads footer-ness out of the chrome producer makes
// the orphan tie an artifact of the chrome's existence rather than a real
// relation between two groups. Keying on the items' own Group.Key — which
// BOTH the chrome item and the value-line items carry identically
// (tableRectSource.chromeRowGroup / textRunSource.lineRowGroup, the same
// Index -1 sentinel) — means the tie survives the footer's chrome being
// deleted entirely, which is precisely what part (a) requires and what
// the first shape would have failed.
//
// It also strengthens AC7 rather than weakening it: PHASE A
// (contentColumnItems, page_number.go) and PHASE B (paginateDocument,
// render.go) append in DIFFERENT ORDERS but produce the same GROUPS, so
// reading the targets off the items makes the two passes agree by
// construction on the input to the tie, not merely on the input to
// Paginate.
//
// A table whose bound collection is EMPTY (DECISION-3: the footer still
// renders) has no preceding row to tie to — no target is built for it, and
// its footer is placed exactly where an ordinary ungrouped item would be
// (there is nothing to orphan it FROM).
//
// Determinism (R5): `elements` is a SLICE walked in first-appearance
// order; maxRow/hasFooter are maps consulted by LOOKUP only. No map is
// ever ranged for this function's own output order.
func footerOrphanTargetsFrom(items []layout.ColumnItem) []footerOrphanTarget {
	var elements []string
	seen := map[string]bool{}
	maxRow := map[string]int{}
	hasFooter := map[string]bool{}

	for i := range items {
		g := items[i].Group
		if !g.Present || g.Key.IsHeader {
			continue
		}
		id := g.Key.ElementID
		if !seen[id] {
			seen[id] = true
			elements = append(elements, id)
			maxRow[id] = -1
		}
		switch {
		case g.Key.Index == footerGroupIndex:
			hasFooter[id] = true
		case g.Key.Index > maxRow[id]:
			maxRow[id] = g.Key.Index
		}
	}

	var out []footerOrphanTarget
	for _, id := range elements {
		if !hasFooter[id] || maxRow[id] < 0 {
			continue
		}
		out = append(out, footerOrphanTarget{
			elementID:    id,
			footerKey:    layout.ItemGroupKey{ElementID: id, Index: footerGroupIndex},
			precedingKey: layout.ItemGroupKey{ElementID: id, Index: maxRow[id]},
		})
	}
	return out
}

// pageOfGroup finds the page a group landed on in plan, by direct lookup
// against refPage — a page-index built once by paginateWithFooterOrphanFix
// from plan's own Pages, never re-derived per target.
func pageOfGroup(items []layout.ColumnItem, key layout.ItemGroupKey, rectPage map[layout.RectRef]int, runPage map[layout.TextRunRef]int) (int, bool) {
	for i := range items {
		if !items[i].Group.Present || items[i].Group.Key != key {
			continue
		}
		if len(items[i].Rects) > 0 {
			if p, ok := rectPage[items[i].Rects[0]]; ok {
				return p, true
			}
		}
		if len(items[i].Runs) > 0 {
			if p, ok := runPage[items[i].Runs[0]]; ok {
				return p, true
			}
		}
	}
	return 0, false
}

// groupWasClipped reports whether Paginate cut this group off (Story
// 4.6). A single SLICE WALK over a list that is EMPTY for every document
// with no over-tall group — never a map range (R5).
func groupWasClipped(clipped []layout.TableRowClipped, key layout.ItemGroupKey) bool {
	for _, c := range clipped {
		if c.Key == key {
			return true
		}
	}
	return false
}

// anyTieWasClipped reports whether the merged pagination CLIPPED any of
// the ties it was asked to try (Story 4.6). A merged group is identified
// by the target's precedingKey, because applyFooterMerge re-keys the
// footer ONTO the preceding row — so the tie, if it exists at all, exists
// under that key.
func anyTieWasClipped(plan layout.Pagination, targets []footerOrphanTarget) bool {
	for _, t := range targets {
		if groupWasClipped(plan.Clipped, t.precedingKey) {
			return true
		}
	}
	return false
}

// paginateWithFooterOrphanFix runs layout.Paginate, then AC5's orphan
// check, then — only for a table that is actually orphaned — a second
// Paginate call with that ONE table's footer temporarily tied to its
// preceding row. See this file's own package-level doc comment for why
// this shape, not a permanent merge inside layout.ItemGroup.
//
// Returns the SAME (layout.Pagination, error) shape layout.Paginate
// itself does, plus any DiagCodeTableFooterOrphanSuppressed Warnings
// DECISION-2(b) requires when even the merged group does not fit.
func paginateWithFooterOrphanFix(g layout.PageGeometry, items []layout.ColumnItem, targets []footerOrphanTarget) (layout.Pagination, []Diagnostic, error) {
	plan, _, diags, err := paginateWithFooterOrphanFixPages(g, items, targets)
	return plan, diags, err
}

// paginateWithFooterOrphanFixPages is paginateWithFooterOrphanFix, and it also
// returns the page each item landed on in the pagination it chose (indexed
// like items — applyFooterMerge re-keys items but never reorders them). The
// section break (section_break.go) reads it to find where the content above
// the break ends.
func paginateWithFooterOrphanFixPages(g layout.PageGeometry, items []layout.ColumnItem, targets []footerOrphanTarget) (layout.Pagination, []int, []Diagnostic, error) {
	plan, planPages, err := layout.PaginateWithItemPages(g, items)
	if err != nil {
		return layout.Pagination{}, nil, nil, err
	}
	if len(targets) == 0 {
		return plan, planPages, nil, nil
	}

	rectPage := map[layout.RectRef]int{}
	runPage := map[layout.TextRunRef]int{}
	for pageIdx, pa := range plan.Pages {
		for _, r := range pa.ContentRects {
			rectPage[r] = pageIdx
		}
		for _, r := range pa.ContentRuns {
			runPage[r] = pageIdx
		}
	}

	var toMerge []footerOrphanTarget
	for _, t := range targets {
		// Story 4.6, AC5: a footer group that was CLIPPED is alone on a
		// fresh page BY DESIGN, not orphaned. D-4.5.2's own words —
		// "'place it alone' assumes the footer FITS alone" — are the
		// reason: this one fits nowhere, and tying it to the preceding
		// row cannot help (the merged group is strictly taller still, so
		// it clips too, and the clip's Warning would then name the
		// PRECEDING ROW's index instead of the footer). Standing the
		// orphan tie down here is what keeps the diagnostic naming the
		// group that was actually cut.
		if groupWasClipped(plan.Clipped, t.footerKey) {
			continue
		}
		fp, fok := pageOfGroup(items, t.footerKey, rectPage, runPage)
		pp, pok := pageOfGroup(items, t.precedingKey, rectPage, runPage)
		if !fok || !pok {
			// Structurally unreachable: footerOrphanTargetsFrom only
			// builds a target when both the footer's own tableRectSource
			// and at least one data row of the same table exist among
			// tableRects, and every tableRectSource in the content band
			// becomes exactly one grouped layout.ColumnItem above.
			continue
		}
		if fp != pp {
			toMerge = append(toMerge, t)
		}
	}
	if len(toMerge) == 0 {
		return plan, planPages, nil, nil
	}

	plan2, plan2Pages, err2 := layout.PaginateWithItemPages(g, applyFooterMerge(items, toMerge))
	if err2 == nil && !anyTieWasClipped(plan2, toMerge) {
		return plan2, plan2Pages, nil, nil
	}

	var of *layout.OverflowError
	if err2 != nil && !errors.As(err2, &of) {
		return layout.Pagination{}, nil, nil, err2
	}

	// DECISION-2(b), as ruled: SOME merged group's preceding row and
	// footer together exceed the content window. Never error for this —
	// place that footer ALONE and record it.
	//
	// STORY 4.6 CHANGED THE SIGNAL, NOT THE RULE. "The tie does not fit"
	// used to arrive here as a *layout.OverflowError; an over-tall GROUP
	// is now CLIPPED instead, so a merged group that does not fit comes
	// back as a Pagination that succeeded and clipped the tie. Both
	// spellings mean the same thing and both must revert the tie — and
	// reverting is not a formality here: the footer ALONE fits perfectly
	// well, so accepting the clipped tie would DESTROY content that had
	// somewhere to go, purely as an artefact of an optimisation attempt.
	// D-4.5.2's ruling stands unchanged: place the footer alone, record
	// the suppression.
	//
	// PER TABLE, NEVER FOR THE WHOLE CALL (this story's review, Finding
	// 9). The single Paginate call above merges EVERY orphaned table's
	// candidate at once, so one pathological table's OverflowError used
	// to discard plan2 wholesale: every other table's orphan tie — which
	// would have succeeded — was silently abandoned, and every candidate
	// got a Warning naming it as the cause. The reversion and the Warning
	// now name only the table that actually could not fit: each candidate
	// is re-tried on top of the ones already accepted, so a table whose
	// tie fits keeps it. This walk is reached ONLY on the rare
	// unsatisfiable path (the common orphan case returned above after one
	// extra Paginate call), and `toMerge` is a SLICE walked in
	// first-appearance order, so its outcome is deterministic (R5).
	var accepted, suppressed []footerOrphanTarget
	best, bestPages := plan, planPages
	for _, c := range toMerge {
		trial := append(append([]footerOrphanTarget(nil), accepted...), c)
		p, pPages, err := layout.PaginateWithItemPages(g, applyFooterMerge(items, trial))
		switch {
		case err == nil && !groupWasClipped(p.Clipped, c.precedingKey):
			accepted, best, bestPages = trial, p, pPages
		case err == nil:
			// Story 4.6: the tie fit no window and was clipped. Same
			// verdict as the OverflowError below — revert and record.
			suppressed = append(suppressed, c)
		case errors.As(err, &of):
			suppressed = append(suppressed, c)
		default:
			return layout.Pagination{}, nil, nil, err
		}
	}

	var diags []Diagnostic
	for _, c := range suppressed {
		diags = append(diags, Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeTableFooterOrphanSuppressed,
			ElementID: c.elementID,
			Message: "folio8: Render: element " + c.elementID +
				": the footer row and the data row immediately preceding it do not fit together in the content window, so the orphan rule (FR25) could not be honoured — the footer is placed alone on its page instead. Reduce the table's row height (font size or cell padding), or increase the page's content height (smaller margins, or a smaller page-header/page-footer)",
		})
	}
	return best, bestPages, diags, nil
}

// applyFooterMerge returns a copy of items in which exactly the FOOTER
// group of each named target is re-keyed to that target's preceding data
// row, so one layout.Paginate call sees the two as one group. The input
// slice is never mutated.
//
// The re-key is keyed on footer-ness BY CONSTRUCTION and this is the
// property the story's D-4.5.4 fence is really about (this story's
// review, Finding 10): every key this function rewrites is a
// footerOrphanTarget.footerKey, whose Index is footerGroupIndex, so no
// data row's, header's or unrelated element's grouping can be changed by
// the merge — and therefore no OverflowError raised by the merged
// pagination can originate anywhere but a merged footer group.
// Extracted from paginateWithFooterOrphanFix so that property is
// ASSERTABLE (TestFooterMergeRewritesOnlyFooterGroupKeys) rather than
// only argued in a comment: the previous fence
// (TestOverTallSingleRowStillOverflows) reddens on pass 1's error
// passthrough and never reaches the merge at all.
func applyFooterMerge(items []layout.ColumnItem, targets []footerOrphanTarget) []layout.ColumnItem {
	mergeKey := map[layout.ItemGroupKey]layout.ItemGroupKey{}
	for _, t := range targets {
		mergeKey[t.footerKey] = t.precedingKey
	}
	merged := make([]layout.ColumnItem, len(items))
	copy(merged, items)
	for i := range merged {
		if !merged[i].Group.Present {
			continue
		}
		if newKey, ok := mergeKey[merged[i].Group.Key]; ok {
			merged[i].Group.Key = newKey
		}
	}
	return merged
}
