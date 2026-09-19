package folio8

import (
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// A text element's style.align and style.valign were parsed, checked against
// their closed sets and serialized long before anything drew with them: both
// producers placed every line at the element's own top-left corner, so an
// authored "align": "right" was accepted, round-tripped, shown in the
// inspector — and then ignored on the page. These functions are the whole of
// the placement rule, and they are shared by the PDF producer (render.go) and
// the canvas paint projection (page_setup.go) so that the two cannot drift:
// the canvas exists to show what prints.
//
// SLACK ONLY. Alignment distributes the space an element's declared box has
// left over and nothing else: a line wider than its declared width, or a
// block taller than its declared height, keeps the element's own start edge,
// where FR44's existing clip-and-warn applies unchanged. One rule for both
// producers, and every canvas coordinate stays inside the non-negative,
// band-relative bound the paint plan is checked against.
//
// STORY 7.3 ADDS A FOURTH ALIGNMENT THAT PLACES THE SAME SLACK DIFFERENTLY.
// left, right and center move a whole line BEFORE it is drawn — one offset,
// one cursor, one run per face segment. `justify` distributes the same slack
// BETWEEN the line's pieces instead, so the line is drawn as several rune
// ranges at several x positions and its right edge lands on the declared
// width exactly. It is still slack only: nothing is stretched, nothing is
// re-shaped, and an overflowing line is still set at the start edge and
// clipped.
//
// THREE INDEPENDENT CONDITIONS LEAVE A JUSTIFIED LINE RAGGED, and they are
// three because each answers a different question (D-7.1.5):
//
//   - the break-kind field answers "which break ended this line" — a line
//     the author ended by typing a break is not stretched;
//   - the line's INDEX answers "is this the last line" — the field cannot,
//     because a line no break ended carries the zero value BreakOptional,
//     which is why 7.1 wrote that field for this consumer and told it to
//     derive the last line from the index instead;
//   - the interior gap count answers "is there anywhere to put slack" — an
//     atomic unknown Thai run (AD-25) offers no interior opportunity at all.
//
// Collapsing any two into one flag would make the third re-derivable
// wrongly, which is the hazard BreakKind was made a named kind rather than a
// bool to prevent.
//
// The arithmetic is integer millipoints throughout, halved by geom.ScaleRound
// — the same round-half-to-even rule every other derived span in the producer
// is rounded by — so all four targets produce identical bytes. The justified
// remainder is integer-exact for the same reason and by a stated ORDER: no
// float, no second rounding site, and no division whose remainder is
// discarded.

// elementAlignment reads one element's committed alignment. An unset style,
// an explicit null, and "left"/"top" all mean the same thing here: no offset.
func elementAlignment(el template.Element) (align string, valign string) {
	if !el.Style.Set || el.Style.Null {
		return "", ""
	}
	st := el.Style.Value
	if st.Align.Set && !st.Align.Null {
		align = st.Align.Value
	}
	if st.Valign.Set && !st.Valign.Null {
		valign = st.Valign.Value
	}
	return align, valign
}

// textAlignOffset is one packed line's horizontal offset inside the element's
// declared width. A centred line splits the slack with geom.ScaleRound, whose
// exact-half case rounds to even.
func textAlignOffset(align string, boxWidth, lineWidth geom.Length) geom.Length {
	slack := boxWidth - lineWidth
	if boxWidth <= 0 || slack <= 0 {
		return 0
	}
	switch align {
	case "right":
		return slack
	case "center":
		return geom.ScaleRound(slack, 1, 2)
	default: // "left", and an unset align: the element's own start edge
		return 0
	}
}

// textValignOffset is the whole packed block's vertical offset inside the
// element's declared height. It moves the block, never the spacing within it:
// the inter-baseline advance is settled upstream, by the font chain, the size
// and (since Story 7.2) style.lineSpacing, and this function only re-seats
// whatever height that produced.
//
// So for valign middle/bottom a lineSpacing DOES move the drawn first
// baseline — not because the ratio touched the first line, but because a
// taller block seated against the box's bottom starts higher. Under the
// default valign (top) the offset is 0 and the first baseline never moves.
func textValignOffset(valign string, boxHeight, blockHeight geom.Length) geom.Length {
	slack := boxHeight - blockHeight
	if boxHeight <= 0 || slack <= 0 {
		return 0
	}
	switch valign {
	case "bottom":
		return slack
	case "middle":
		return geom.ScaleRound(slack, 1, 2)
	default: // "top", and an unset valign: the element's own top edge
		return 0
	}
}

// textBlockHeight is the vertical extent of `lines` packed lines under one
// vertical model: the inter-baseline advance for every line after the first,
// plus the first line's ascent and the last line's descent — the same two
// spans positionSegments and the page-splitting item extents already use.
func textBlockHeight(lines int, vm verticalMetrics) geom.Length {
	if lines <= 0 {
		return 0
	}
	return geom.Length(int64(lines-1))*vm.Advance + vm.FirstBaseline + vm.LastDescent
}

// alignJustify is the style-only alignment value Story 7.3 added to the
// format's closed set. Named here so the placement rule and the load-time
// set cannot drift on a spelling; the SET itself lives in
// internal/template/closedsets.go and is the single source both loader and
// property command validate through.
const alignJustify = template.AlignJustify

// justifiedPiece is one contiguous rune range of a justified line and the
// offset, from the element's own start edge, at which it is drawn.
//
// A piece is exactly what positionSegments already takes — a half-open
// element-global rune range at an x — so justification adds no new
// placement primitive, only more calls to the existing one.
type justifiedPiece struct {
	from, to int
	offset   geom.Length
}

// justifiedLinePieces is THE justification rule, and it is one function
// consumed identically by the PDF producer (render.go) and the canvas paint
// projection (page_setup.go), exactly as textAlignOffset already is. The
// canvas must show the word positions the PDF prints (AD-17, the Story 5.9
// invariant), and it achieves that by consuming these engine-computed
// positions — never by enabling browser justification, which
// canvas-authority-contract.test.ts bans outright.
//
// It returns nil for "set this line at the element's natural start edge",
// which is what every ragged case is: not justified, not centred, no offset.
// The caller's existing single positionSegments call then runs unchanged,
// which is what keeps the whole pre-7.3 corpus byte-identical — an
// unjustified element never reaches a new branch at all.
//
// THE GAPS ARE THE OPPORTUNITIES THE CALLER ALREADY HOLDS, filtered, never
// breaks computed a second time. packLines retains only the opportunity it
// took, but both call sites still have the same `ops` slice live, and a
// second derivation of where a line may break is precisely the hazard
// verticalMetrics' doc comment says such types exist to close. An interior
// opportunity is never mandatory: a mandatory break always ends its line,
// so it can never sit strictly inside one.
//
// THE PIECE BOUNDARY IS THE OPPORTUNITY'S OWN LineEnd, and the pieces
// PARTITION [ln.from, ln.to) with no rune left out. Interior inter-word
// spaces are real shaped glyphs with their own advance — consumption applies
// only at the break actually taken — so the whitespace run following a gap
// is drawn at the head of the next piece. Justification adds space BETWEEN
// pieces; it deletes and re-shapes nothing.
//
// THE SLACK IS BASED ON THE SUMMED PIECE WIDTHS, NOT ON ln.width, and that
// is what makes the right edge land on the declared width EXACTLY. Each
// piece's advance is rounded on its own, and a sum of roundings is not the
// rounding of a sum, so a line positioned from the packer's single
// measurement could miss the edge by a millipoint or two. measureRuneRange's
// own doc comment states it reproduces positionSegments' cursor arithmetic
// because it IS that arithmetic, so measuring the pieces with it and taking
// slack = boxWidth − Σ pieceWidths closes the loop with one derivation and
// no second measurement path. Overflow detection keeps reading ln.width, so
// FR44 is untouched.
//
// THE REMAINDER RULE IS STATED AND ORDERED: with g gaps and slack s, every
// gap receives s / g, and the FIRST s mod g gaps in ascending position along
// the line each receive one additional millipoint. Ascending order is the
// line's own reading order — not map iteration, not locale, not target — and
// the distributed amounts sum to s exactly. Worked: 7 across 3 gaps gives
// 3, 2, 2; 6 across 3 gives 2, 2, 2; 2 across 3 gives 1, 1, 0, and a gap
// legitimately receiving nothing is not a defect.
func justifiedLinePieces(
	align string,
	ln wrappedLine,
	lineIndex, lineCount int,
	segs []faceSegment,
	ops []text.Opportunity,
	fontSize, boxWidth geom.Length,
) []justifiedPiece {
	if align != alignJustify {
		return nil
	}
	// No declared width is no box to justify to — consistent with the
	// slack-only rule, and with packMandatoryOnly, which is the packer's
	// own maxWidth <= 0 path.
	if boxWidth <= 0 {
		return nil
	}
	// RAGGED CONDITION 1: the author ended this line by typing a break.
	// Read from the field Story 7.1 wrote for this consumer.
	if ln.endedBy == text.BreakMandatory {
		return nil
	}
	// RAGGED CONDITION 2: this is the last line of the element. DERIVED
	// FROM THE INDEX AND NEVER STORED — endedBy answers a different
	// question and its zero value would answer this one wrongly.
	if lineIndex >= lineCount-1 {
		return nil
	}

	// RAGGED CONDITION 3: nowhere to put the slack.
	pieces := make([]justifiedPiece, 0, 8)
	widths := make([]geom.Length, 0, 8)
	var summed geom.Length
	from := ln.from
	for _, op := range ops {
		if op.LineEnd <= ln.from || op.LineEnd >= ln.to {
			continue
		}
		w := measureRuneRange(segs, from, op.LineEnd, fontSize)
		pieces = append(pieces, justifiedPiece{from: from, to: op.LineEnd})
		widths = append(widths, w)
		summed += w
		from = op.LineEnd
	}
	gaps := len(pieces)
	if gaps == 0 {
		return nil
	}
	w := measureRuneRange(segs, from, ln.to, fontSize)
	pieces = append(pieces, justifiedPiece{from: from, to: ln.to})
	widths = append(widths, w)
	summed += w

	// SLACK ONLY. A line whose pieces already fill or overflow the box has
	// none to distribute: FR44's clip-and-warn applies unchanged, from the
	// packer's own ln.width, and nothing here ever distributes a negative.
	slack := boxWidth - summed
	if slack <= 0 {
		return nil
	}

	base := geom.Length(int64(slack) / int64(gaps))
	remainder := int64(slack) - int64(base)*int64(gaps)

	var cursor geom.Length
	for i := range pieces {
		pieces[i].offset = cursor
		cursor += widths[i]
		if i < gaps {
			cursor += base
			if int64(i) < remainder {
				cursor++
			}
		}
	}
	return pieces
}
