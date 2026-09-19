package folio8

// Story 2.7: FR31, AD-4's {{page}}/{{pages}} construct.
//
// THE OPERATING PRINCIPLE, settled by D-2.7.1/D-2.7.2 and implemented
// here: {{pages}} needs no reservation at all — Y is the same value on
// every page, so it renders at its exact width. {{page}} is the only
// late-bound slot, and its box is FIXED once, at pass one, at
// digits(Y) digit-advances (D-2.7.2) — never re-measured per page
// (D-2.7.1 rejects re-summing advances at substitution). What varies
// per page is which of the pre-shaped digit glyphs 0-9 are drawn in
// that fixed box, right-aligned, with the leading slack expressed as a
// positioning adjustment on the first drawn digit rather than a pad
// glyph (none exists across the shipped face set, measured at this
// story's creation) or zero-padding (AD-14 forbids the coercion).
//
// D-2.7.3 fences the construct to the page-header and page-footer
// bands: content-band placement is a located template error, because
// only the two repeated bands make Y independent of the construct's
// own width (internal/layout/band.go's ContentHeight takes page
// geometry alone).

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// contentColumnItems groups CONTENT-BAND-ONLY runs and images into
// layout.ColumnItems, with LOCAL indices into contentRuns/imageRuns.
//
// It exists so renderDocument can learn the document's page count
// (Y) from layout.Paginate BEFORE the page-header/page-footer bands are
// collected — D-2.7.2's reservation is a function of Y, so Y must be
// known first. paginateDocument re-derives the FINAL, globally-indexed
// items once every run is known and re-runs layout.Paginate.
//
// CORRECTED CLAIM (Story 4.3, D3 — the previous wording here was measured
// FALSE and must not be reintroduced). This function and paginateDocument
// do NOT build `items` in the same order: this one appends text, then
// images, then table rects; paginateDocument appends rects, then text,
// then images. layout.Paginate's sort is stable, so that difference breaks
// ties on Top — and a data row's chrome item ties its first line's Top
// EXACTLY when the body style's top padding is zero (the documented
// default). Before this story that was harmless: this pass's Pagination is
// discarded except for len(Pages), and the two passes' page COUNTS still
// agreed in every configuration measured, because nothing here read the
// per-item partition. It stopped being harmless the moment a table's rows
// gained a grouping identity (this story's Group field): if only one of
// the two builders carried it, the two passes could disagree on Y itself
// — the value {{page}}/{{pages}} print (D-2.7.2) — for a boundary row.
//
// WHAT IS ACTUALLY GUARANTEED NOW. Both builders attach the SAME
// layout.ItemGroup to a row's items (tableRectSource.chromeRowGroup /
// textRunSource.lineRowGroup, direct field lookup, R3), so a boundary row
// moves as one unit in BOTH passes regardless of which order tied it
// against its own chrome. TestBothPaginationPassesAgreeOnRowPartition
// (page_number_test.go) asserts the stronger fact directly — same page
// COUNT and same per-page rowIndex partition — rather than relying on an
// argument about tie order, because a tie-order argument is exactly what
// went stale here once before.
// visible (Story 3.5, R3/AC7) filters ONLY imageRuns here: contentRuns
// has already had every hidden text element's runs excluded upstream,
// by collectBandTextRuns, strictly after that element's own validation
// ran (render_visibility.go). imageRuns, by contrast, is DELIBERATELY
// unfiltered up to this point (collectImageRuns' own doc comment) so
// that every image element — hidden or not — still goes through
// buildPageModel's asset-resolution pass; visible is consulted here,
// at content-band PAGE-MODEL construction, which is the first point
// after that pass where a hidden image's placement would otherwise be
// built.
//
// Table/line/rect elements are not among contentRuns/imageRuns at all
// today, so their visibility verdicts (computeVisibility computes one
// for every kind) are not consulted here either — see
// computeVisibility's own doc comment (Story 3.5 finisher review,
// Finding 6 / Minor) before wiring one of those kinds' placement in a
// future story: THAT story owns consulting isVisible for its own kind.
func contentColumnItems(contentRuns []textRunSource, imageRuns []imageRunSource, tableRects []tableRectSource, visible visibilityVerdicts, keepTogether keepTogetherIndex) []layout.ColumnItem {
	var items []layout.ColumnItem
	for i := 0; i < len(contentRuns); i++ {
		j := i
		item := layout.ColumnItem{
			ElementID: contentRuns[i].elementID,
			Top:       contentRuns[i].itemTop,
			Bottom:    contentRuns[i].itemBottom,
			// Story 4.3, AC3: row identity by DIRECT FIELD LOOKUP (R3),
			// carried through to THIS builder too — not only
			// paginateDocument's — so the page-count-only pass (PHASE A)
			// sees the same grouping the final pass (PHASE B) does.
			// Story 7.7 applies the SAME substitution here that
			// paginateDocument applies, from the SAME index, for the
			// same reason this comment gives above: a group seen by one
			// pass and not the other makes the page count disagree with
			// the render.
			Group: keepTogether.orKeepTogether(contentRuns[i].lineRowGroup(), contentRuns[i].elementID),
		}
		for j < len(contentRuns) &&
			contentRuns[j].elementID == contentRuns[i].elementID &&
			contentRuns[j].lineIndex == contentRuns[i].lineIndex {
			item.Runs = append(item.Runs, layout.TextRunRef(j))
			extendItem(&item, contentRuns[j].x, contentRuns[j].x)
			j++
		}
		items = append(items, item)
		i = j - 1
	}
	for i, r := range imageRuns {
		if r.band != contentBandIndex {
			continue
		}
		if !isVisible(visible, template.ElementID(r.elementID)) {
			// AD-24/R3: absent from the page model entirely — no
			// ColumnItem, no gap-filling substitute. r's own
			// validation (width/height/asset presence, and asset
			// resolution downstream of collectImageRuns) already ran
			// unconditionally, regardless of this verdict.
			continue
		}
		items = append(items, layout.ColumnItem{
			ElementID: r.elementID,
			Top:       r.y,
			Bottom:    r.y + r.boxH,
			Images:    []layout.ImageRef{layout.ImageRef(i)},
			Group:     keepTogether.keepTogetherGroup(r.elementID),
		})
		extendItem(&items[len(items)-1], r.x, r.x+r.boxW)
	}
	// Story 4.1: table header rects. collectBandTableRuns already
	// filters to VISIBLE tables with >=1 column before returning
	// tableRectSource values, so — unlike the image loop above — there
	// is no isVisible check to repeat here. This function is used ONLY
	// to learn the CONTENT band's pageCount before header/footer text
	// exists (PHASE A); the actual RectRef values are never read back
	// (that Pagination result is discarded except for len(Pages)), so
	// they need only be non-empty, one per rect, to satisfy Paginate's
	// exclusivity check.
	for _, ts := range tableRects {
		if ts.band != contentBandIndex {
			continue
		}
		items = append(items, layout.ColumnItem{
			ElementID: ts.elementID,
			Top:       ts.top,
			Bottom:    ts.bottom,
			Rects:     make([]layout.RectRef, len(ts.rects)),
			// Story 4.3, AC3: same grouping identity as the final pass —
			// this pass's Pagination.Pages length is what {{pages}}/
			// {{page}} resolve against (D-2.7.2), so it must agree with
			// paginateDocument's own partition, not merely its count.
			Group: keepTogether.orKeepTogether(ts.chromeRowGroup(), ts.elementID),
			// SPEC-table-rules: the SAME slice request paginateDocument
			// makes, so a floor's push decides the same page count here.
			Slice: ts.frame.sliceRequest(),
		})
		items[len(items)-1].Left, items[len(items)-1].Right, items[len(items)-1].HasExtent = rectSourceExtent(ts)
	}
	return items
}

// digitTableBandIndex marks a textRunSource that exists ONLY to force a
// face's ten digit CIDs into the document's subset and CID allocation
// (buildShapedPDFRuns) — it is never drawn. paginateDocument's
// composition step skips it explicitly, by name, rather than folding it
// into the "unrecognised band" internal error Story 2.6 finisher Finding
// 10 raised — that error exists to catch an ENUMERATION invariant
// breaking, and this band is deliberately outside the enumeration.
const digitTableBandIndex = -1

// pageSlotSpan is one {{page}} occurrence's RESERVED rune range within
// the text handed to shapeSegments — a digitsY-wide run of filler
// digits standing in for whatever the page's own number will be. The
// filler's IDENTITY never matters (D-2.7.2: every shipped face's ten
// digit advances are equal), only its WIDTH — digitsY glyphs — because
// that width is the reservation pass one commits packLines to.
type pageSlotSpan struct {
	from, to int // element-global rune range in the RESOLVED text
	digitsY  int
}

// digitsOf is digits(n) — n's decimal digit count, treating n < 1 as
// 1 digit (a document always has at least one page).
func digitsOf(n int) int {
	if n < 1 {
		n = 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

// decimalDigits returns n's decimal digits, most significant first, as
// values 0-9. n <= 0 returns a single 0 (unreachable through Render — a
// page number is always >= 1 — but total rather than partial).
func decimalDigits(n int) []int {
	if n <= 0 {
		return []int{0}
	}
	var rev []int
	for n > 0 {
		rev = append(rev, n%10)
		n /= 10
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// runeRangeOverlapsSubstitution reports whether the rune range
// [start,end) overlaps ANY of subs' OUTPUT spans (internal/bind's
// bind.Substitution.Start/End, rune indices into the same bound text
// this file scans).
//
// This story's review, Finding 4 (Major): a "{{page}}"/"{{pages}}"-
// shaped SUBSTRING can arrive inside a bound DATA VALUE — a report data
// field reading literally "{{page}}" — rather than from the element's
// authored template text. internal/bind resolves a template's text
// EXACTLY ONCE (page_number.go's own operating principle, restated by
// this fix): a construct that only exists because a substituted value
// happens to spell it is not a construct the template author wrote, and
// must never be treated as one. Any occurrence found inside a
// Substitution's own span came from data, by construction — a reserved
// token, by contrast, produces NO Substitution (internal/bind/text.go's
// reservation branch), so a GENUINE reserved token never overlaps one.
func runeRangeOverlapsSubstitution(start, end int, subs []bind.Substitution) bool {
	for _, s := range subs {
		if start < s.End && s.Start < end {
			return true
		}
	}
	return false
}

// firstReservedPageToken reports the first {{page}} or {{pages}} token
// found in text that did NOT arrive inside a bound data value (Finding
// 4: subs' spans mark exactly the ranges a substitution supplied) — text
// ALREADY PASSED THROUGH bind.BindTextSpans, so every OTHER "{{…}}" has
// already been resolved or has already errored; the only tokens that
// can still appear AS AUTHORED TEMPLATE TEXT are the two internal/bind
// leaves untouched (internal/bind/text.go's reservation, 'page'/'pages'
// trimmed exactly). The scan is the same "{{" … "}}" tokenizer bind.go
// uses, because these are the same tokens it declined to touch.
func firstReservedPageToken(text string, subs []bind.Substitution) (name string, found bool) {
	rest := text
	runePos := 0
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			return "", false
		}
		runePos += utf8.RuneCountInString(rest[:start])
		afterOpen := rest[start+2:]
		end := strings.Index(afterOpen, "}}")
		if end < 0 {
			return "", false
		}
		inner := afterOpen[:end]
		tokenRunes := utf8.RuneCountInString("{{" + inner + "}}")
		trimmed := strings.TrimSpace(inner)
		if (trimmed == "page" || trimmed == "pages") &&
			!runeRangeOverlapsSubstitution(runePos, runePos+tokenRunes, subs) {
			return trimmed, true
		}
		runePos += tokenRunes
		rest = afterOpen[end+2:]
	}
}

// tokenReplacement records one {{page}}/{{pages}} token's rune range in
// the ORIGINAL bound text and its replacement's rune range in the
// RESOLVED text — the bookkeeping shiftSubstitutions needs to keep any
// OTHER bind.Substitution in the same element (a mixed
// "{{customer.name}} — Page {{page}} of {{pages}}") pointing at the
// right runes once the reservation changes the text's length.
type tokenReplacement struct{ origStart, origEnd, newStart, newEnd int }

// resolvePageTokens replaces every {{pages}} with pageCount's exact
// decimal digits (Y is the same value on every page — no reservation,
// D-2.7.2) and every {{page}} with a digits(pageCount)-wide run of
// filler digits (the RESERVATION — D-2.7.2's "digits(Y) digit-advances,
// exactly sufficient and never over-wide"), recording each {{page}}
// occurrence's rune range in the result as a pageSlotSpan and every
// token's before/after rune range as a tokenReplacement.
//
// subs (Finding 4, this story's review) marks every range of boundText
// a DATA SUBSTITUTION supplied. A "{{page}}"/"{{pages}}"-shaped
// substring inside one of those ranges did not come from the element's
// authored template text and is left INERT — written back verbatim,
// exactly like any other non-reserved "{{…}}"-shaped text a bound value
// happens to contain — never treated as the construct.
// firstReservedPageToken's presence check and this function's dispatch
// apply the SAME rule so the two can never disagree about which
// occurrences are genuine.
//
// It performs no shaping and consults no font: it is a text transform,
// the same kind bind.BindText already performs on every OTHER
// placeholder. The box those filler digits occupy is measured
// downstream, by the ordinary shapeSegments/packLines pass every text
// element already goes through (D-2.7.1: the box is fixed HERE, at
// pass one, going through the SAME measurement path as any other
// text).
func resolvePageTokens(boundText string, pageCount int, subs []bind.Substitution) (resolved string, slots []pageSlotSpan, replacements []tokenReplacement) {
	var out strings.Builder
	runesWritten := 0
	origRunePos := 0
	write := func(s string) {
		out.WriteString(s)
		runesWritten += utf8.RuneCountInString(s)
	}
	rest := boundText
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			write(rest)
			break
		}
		prefix := rest[:start]
		write(prefix)
		origRunePos += utf8.RuneCountInString(prefix)

		afterOpen := rest[start+2:]
		end := strings.Index(afterOpen, "}}")
		if end < 0 {
			// Unterminated "{{" — bind.BindTextSpans would already have
			// rejected this before this function ever sees the text, so
			// this is unreachable through Render. Written through
			// verbatim rather than panicking, matching this file's
			// "fail loud only where the failure is reachable" posture.
			write(rest)
			break
		}
		inner := afterOpen[:end]
		tokenText := "{{" + inner + "}}"
		tokenRuneLen := utf8.RuneCountInString(tokenText)
		rest = afterOpen[end+2:]
		trimmed := strings.TrimSpace(inner)

		origTokenStart := origRunePos
		newTokenStart := runesWritten
		// Finding 4: a token that LOOKS reserved but arrived inside a
		// data substitution is not the construct — treat it exactly
		// like the "default" (unreachable third-token) case below:
		// written back verbatim, no reservation, no pageSlotSpan.
		reserved := (trimmed == "page" || trimmed == "pages") &&
			!runeRangeOverlapsSubstitution(origTokenStart, origTokenStart+tokenRuneLen, subs)
		switch {
		case trimmed == "pages" && reserved:
			write(strconv.Itoa(pageCount))
		case trimmed == "page" && reserved:
			digitsY := digitsOf(pageCount)
			from := runesWritten
			write(strings.Repeat("0", digitsY))
			slots = append(slots, pageSlotSpan{from: from, to: runesWritten, digitsY: digitsY})
		default:
			// Reached for a THIRD reserved token (internal/bind's
			// reservedPlaceholders does not declare one — unreachable
			// through the shipped set) OR for a data-substituted
			// occurrence of "{{page}}"/"{{pages}}" (Finding 4: reached
			// and exercised — inert text, written through verbatim).
			write(tokenText)
		}
		origRunePos += tokenRuneLen
		replacements = append(replacements, tokenReplacement{
			origStart: origTokenStart, origEnd: origRunePos,
			newStart: newTokenStart, newEnd: runesWritten,
		})
	}
	return out.String(), slots, replacements
}

// shiftSubstitutions adjusts every OTHER bind.Substitution's rune span
// (still expressed against the pre-resolvePageTokens text) to describe
// the same content in the post-resolvePageTokens text. A GENUINE
// {{page}}/{{pages}} reservation can never overlap a Substitution's
// span — reserved tokens produce no Substitution (internal/bind's
// reservation branch), and resolvePageTokens itself now REFUSES to
// treat a "{{page}}"/"{{pages}}"-shaped occurrence found inside a
// Substitution's span as reserved (Finding 4, this story's review: a
// bound data VALUE spelling that text is not the construct) — so every
// span is shifted uniformly by the cumulative length delta of every
// token replacement entirely before it, never split.
func shiftSubstitutions(subs []bind.Substitution, replacements []tokenReplacement) []bind.Substitution {
	if len(subs) == 0 || len(replacements) == 0 {
		return subs
	}
	out := make([]bind.Substitution, len(subs))
	for i, s := range subs {
		delta := 0
		for _, r := range replacements {
			if r.origEnd <= s.Start {
				delta += (r.newEnd - r.newStart) - (r.origEnd - r.origStart)
			}
		}
		out[i] = bind.Substitution{Path: s.Path, Start: s.Start + delta, End: s.End + delta}
	}
	return out
}

// pendingPageSlot is one {{page}} occurrence collected while shaping a
// page-header/page-footer element: WHICH run holds the reservation
// (runIndex, and the local glyph range within it — set by
// positionSegments), and WHICH run is that face's digit table
// (digitTableIndex) — both indices are LOCAL to the collection call
// that produced them; renderDocument shifts them to the final combined
// runs slice's indices once that slice's layout is known.
type pendingPageSlot struct {
	runIndex         int
	glyphLo, glyphHi int
	digitsY          int
	digitTableIndex  int
}

// digitTableRun shapes "0123456789" once, through the SAME
// shapeSegments seam every text element uses (never a second shaping
// route), and returns it as a textRunSource tagged digitTableBandIndex
// — never drawn, present solely so its ten glyphs enter the document's
// glyph union (renderDocument) and receive CIDs (buildShapedPDFRuns)
// exactly as any other shaped text would.
//
// It resolves to the SAME face a {{page}} slot in this element resolves
// to, and that identity is LOAD-BEARING rather than incidental:
// buildPageNumberSlot pairs the SLOT RUN's face name with THIS run's
// CIDs and injects them with no re-shaping, so two different faces would
// put one font program's glyph ids into another's — arbitrary wrong
// glyphs in a page number, with nothing to report it.
//
// ⚠ IT IS SHAPED FROM THE SAME (chain, styled) PAIR THE SLOT'S OWN RUN
// IS, AND STORY 11.2 IS WHY IT HAS TO SAY SO. The identity used to hold
// "by construction: both are shaped from the same chain, and digits
// never fall back within the shipped set — resolveRuneFace always
// returns chain[0] for a digit". Style variants broke the premise in two
// places at once: the element's runes resolve through the styled list,
// and a declared variant is NOT guaranteed to cover the digits, so the
// slot's run can fall back to its base face while a differently-derived
// chain resolves the digit table elsewhere. Passing the pair verbatim is
// what keeps the two resolutions ONE resolution. Do not hand this
// function a pre-coalesced list; that is the shape that diverged.
// Because digits never fall back within the shipped set,
// the digit table can never itself produce a missing-glyph Diagnostic
// (Story 3.6) — decimal digits 0-9 are always covered by chain[0] in
// every shipped/testdata font — so its diagnostics return is discarded
// here, never silently swallowing a real one: the len(segs) check below
// would already fail loudly (a different message) if coverage ever
// broke for a digit.
func digitTableRun(chain, styled []string, fontSize geom.Length, fs FontSet, cache *fontCache) (textRunSource, error) {
	// The element id is EMPTY and the diagnostics are discarded, which is
	// what keeps this site from minting an unlocated Warning when a
	// declared variant does not cover the digits — the element's own runs
	// have already reported that absence, located, for the runes that
	// actually reached the page.
	segs, _, err := shapeSegments("", chain, styled, "0123456789", fs, cache, breaksAreDrawn)
	if err != nil {
		return textRunSource{}, err
	}
	if len(segs) != 1 || len(segs[0].glyphs) != 10 {
		return textRunSource{}, fmt.Errorf(
			"folio8: Render: internal error: the digit table for font chain %v shaped to %d segment(s) "+
				"and %d glyph(s) — Story 2.7's page-number reservation requires the decimal digits 0-9 to "+
				"shape to exactly one face segment of ten glyphs",
			chain, len(segs), glyphCountOf(segs),
		)
	}
	placed, poserr := positionSegments(segs, 0, 10, 0, 0, fontSize, 0, nil)
	if poserr != nil {
		// Unreachable: slots is nil above, so positionSegments' only
		// error path (a straddling {{page}} reservation) cannot fire.
		// Propagated rather than assumed away.
		return textRunSource{}, fmt.Errorf("folio8: Render: internal error: digit table positioning: %w", poserr)
	}
	if len(placed) != 1 {
		return textRunSource{}, fmt.Errorf(
			"folio8: Render: internal error: the digit table for font chain %v positioned into %d run(s), want 1",
			chain, len(placed),
		)
	}
	run := placed[0]
	run.band = digitTableBandIndex
	return run, nil
}

func glyphCountOf(segs []faceSegment) int {
	n := 0
	for _, s := range segs {
		n += len(s.glyphs)
	}
	return n
}

// buildPageNumberSlot turns one pendingPageSlot into a
// pagemodel.PageNumberSlot, reading the digit table's ALREADY-ALLOCATED
// CIDs and 1000-em advances out of pdfRuns (buildShapedPDFRuns has
// already run by the time this is called — D-2.7.1: nothing here
// shapes anything; it only reads what pass one already measured).
//
// It fails closed if the ten digit advances are not identical — the
// residual hazard D-2.7.2 names: "the face set is not frozen, and a
// proportional-figure face would silently break the reservation." Every
// shipped face is tabular (measured at this story's creation); this is
// the render-time consequence of that ceasing to hold for a
// caller-supplied face, and it is a LOCATED error, not a silently wrong
// reservation.
func buildPageNumberSlot(faceName string, dt pagemodel.TextRun, ps pendingPageSlot) (*pagemodel.PageNumberSlot, error) {
	if len(dt.Glyphs) != 10 {
		return nil, fmt.Errorf(
			"folio8: Render: internal error: face %q's digit table carries %d glyphs, want 10",
			faceName, len(dt.Glyphs),
		)
	}
	adv := dt.Glyphs[0].XAdvance
	var cids [10]uint16
	for d := 0; d < 10; d++ {
		if dt.Glyphs[d].XAdvance != adv {
			return nil, fmt.Errorf(
				"folio8: Render: face %q: digit %d's advance (%d) differs from digit 0's advance (%d) — "+
					"the {{page}} reservation (D-2.7.2) requires every decimal digit to advance identically "+
					"(a tabular-figures face), which every face this project ships satisfies; a face lacking "+
					"it cannot host a Page X of Y construct",
				faceName, d, dt.Glyphs[d].XAdvance, adv,
			)
		}
		cids[d] = dt.Glyphs[d].CID
	}
	return &pagemodel.PageNumberSlot{
		GlyphLo:      ps.glyphLo,
		GlyphHi:      ps.glyphHi,
		DigitsY:      ps.digitsY,
		DigitAdvance: adv,
		DigitCID:     cids,
	}, nil
}

// resolvePageRunForPage is AC2's between-passes step: run, unchanged,
// for every run but the one carrying a PageSlot; for that one, the
// reserved glyph range is replaced by pageNum's own digits,
// RIGHT-ALIGNED within the reservation (D-2.7.2).
//
// NO SHAPING HAPPENS HERE — this function's own signature is the proof:
// it receives no FontSet, no fontCache, nothing that could consult a
// font. DigitCID and DigitAdvance were measured once, in pass one
// (buildPageNumberSlot, fed by digitTableRun's single shapeSegments
// call); this function only SELECTS among the ten pre-measured digits
// and computes an integer position adjustment — advance arithmetic over
// already-measured glyphs, D-2.7.1's guardrail, not a re-measurement.
//
// THE POSITION ARITHMETIC. A page whose own digit count n is less than
// the reservation's DigitsY leaves DigitsY-n reserved glyph-widths of
// slack BEFORE the digits (right alignment). There is no pad glyph
// (finding 4, story creation) and no zero-padding (AD-14), so the slack
// is expressed as a positioning adjustment on the FIRST drawn digit
// alone: XOffset shifts it right by the slack, and XAdvance is widened
// by the same amount so every LATER glyph in the run — including the
// literal text following the slot — lands exactly where the
// worst-case (DigitsY-wide) canonical layout already put it. A GPOS
// offset's "after" correction (internal/pdf's appendShapedRun) makes
// the run's total consumed advance depend only on XAdvance, never on
// XOffset, which is what keeps this a pure position shift and not a
// re-layout.
//
// MORE THAN ONE SLOT (this story's review, Blocker 1). A run's
// PageSlots entries are DISJOINT, non-overlapping glyph ranges — one
// per {{page}} occurrence positionSegments found in it — so they are
// substituted in a single left-to-right pass over Glyphs: sorted
// ascending by GlyphLo (defensively; positionSegments already emits
// them in that order, since it walks slots in occurrence order), each
// slot consumes the unslotted glyphs before it plus its own
// reservation, and the final unslotted tail is appended once at the
// end. Earlier glyph indices are never invalidated by a later
// substitution, because the pass is forward-only and never re-reads
// indices behind cursor.
func resolvePageRunForPage(run pagemodel.TextRun, pageNum int) pagemodel.TextRun {
	if len(run.PageSlots) == 0 {
		return run
	}
	slots := run.PageSlots
	if len(slots) > 1 {
		slots = append([]pagemodel.PageNumberSlot(nil), run.PageSlots...)
		sort.Slice(slots, func(i, j int) bool { return slots[i].GlyphLo < slots[j].GlyphLo })
	}
	digits := decimalDigits(pageNum)
	n := len(digits)

	newGlyphs := make([]pagemodel.ShapedGlyph, 0, len(run.Glyphs)+n*len(slots))
	cursor := 0
	for _, slot := range slots {
		newGlyphs = append(newGlyphs, run.Glyphs[cursor:slot.GlyphLo]...)
		skip := slot.DigitsY - n
		for i, d := range digits {
			g := pagemodel.ShapedGlyph{CID: slot.DigitCID[d], XAdvance: slot.DigitAdvance}
			if i == 0 && skip > 0 {
				shift := slot.DigitAdvance * int64(skip)
				g.XOffset = shift
				g.XAdvance = shift + slot.DigitAdvance
			}
			newGlyphs = append(newGlyphs, g)
		}
		cursor = slot.GlyphHi
	}
	newGlyphs = append(newGlyphs, run.Glyphs[cursor:]...)

	run.Glyphs = newGlyphs
	run.PageSlots = nil // resolved: pass two reads Glyphs only and never sees this field anyway.
	return run
}
