package folio8

import (
	"fmt"

	"unicode/utf8"

	"github.com/panitw/folio8/folio8-go/internal/bind"
	"github.com/panitw/folio8/folio8-go/internal/expr"
	"github.com/panitw/folio8/folio8-go/internal/fontset"
	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/template"
	"github.com/panitw/folio8/folio8-go/internal/text"
)

// This file is Story 2.4's measurement and line-packing core: given one
// text element's already-shaped face segments and the positions at
// which it may break, it produces the lines that element occupies.
//
// It emits no PDF and decides no vertical placement. That separation is
// deliberate: everything here is decided by WIDTHS, and widths are
// exactly the part that must not disagree with what is drawn.

// faceSegment is one maximal run of an element's text whose runes all
// resolve to the SAME face, together with the shaper's answer for it.
//
// runeStart/runeEnd are ELEMENT-GLOBAL rune indices, while
// ShapedGlyph.Cluster is SEGMENT-LOCAL. The two units meet in exactly
// one place — segmentLocal — so there is one site that can get the
// conversion wrong rather than one per call.
type faceSegment struct {
	face         string
	segText      string
	runeStart    int
	runeEnd      int
	glyphs       []text.ShapedGlyph
	clusterTexts []string
	unitsPerEm   int64
}

func (s faceSegment) segmentLocal(elementRune int) int { return elementRune - s.runeStart }

// glyphRangeForRunes returns the half-open glyph index range covering
// the element-global rune range [from, to) within this segment.
//
// AC10, AND THIS IS THE WHOLE OF IT: the slice boundary is the FIRST
// GLYPH WHOSE Cluster >= r. Break opportunities never fall inside a
// cluster (AD-25's cluster absolute, and CJK/whitespace breaks are
// cluster boundaries trivially), so that position is exact — no glyph
// straddles the cut.
//
// Cluster IS A RUNE INDEX, NOT A BYTE OFFSET (D-2.3.2, pinned by
// TestClusterValuesAreRuneIndices). Reading it as a byte offset would
// agree on ASCII and diverge on the first Thai character, where byte
// offsets run at three times the rune index. Nothing here converts to
// bytes.
//
// Clusters are non-decreasing across the slice for the left-to-right
// scripts folio8 ships, but this scan does not ASSUME contiguity: a
// ligature's cluster is followed by the cluster of its last component
// plus one (ClusterTexts' own finding), so the search is a linear scan
// for the first index satisfying the predicate, not arithmetic on
// cluster values.
func (s faceSegment) glyphRangeForRunes(from, to int) (lo, hi int) {
	localFrom := s.segmentLocal(from)
	localTo := s.segmentLocal(to)

	lo = len(s.glyphs)
	for i, g := range s.glyphs {
		if g.Cluster >= localFrom {
			lo = i
			break
		}
	}
	hi = len(s.glyphs)
	for i, g := range s.glyphs {
		if g.Cluster >= localTo {
			hi = i
			break
		}
	}
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

// advance1000 sums the per-glyph advances of glyphs[lo:hi], each scaled
// INDIVIDUALLY to the 1000-unit em.
//
// PER GLYPH, THEN SUMMED — never summed and then scaled. That is the
// already-rounded space the viewer's pen consumes, and it is the same
// order positionSegments' segment cursor and appendShapedRun's advance
// correction use. Changing the order here would reintroduce a second
// derivation of the same quantity, which is precisely Story 2.3's
// Blocker 1.
//
// The advances come from the SAME []text.ShapedGlyph the run is drawn
// from. fontset.AdvanceForRune — the per-rune hmtx path — must never
// appear on this path: shaped advances include GPOS kerning and hmtx
// advances do not, and text drawn kerned but measured unkerned is the
// defect 2.3 removed.
func (s faceSegment) advance1000(lo, hi int) int64 {
	var total int64
	for _, g := range s.glyphs[lo:hi] {
		total += int64(geom.ScaleRound(geom.Length(int64(g.XAdvance)), 1000, s.unitsPerEm))
	}
	return total
}

// measureRuneRange returns the width, in exact integer millipoints, of
// the element-global rune range [from, to) across segments.
//
// Per segment: scale each glyph advance to the 1000-em, sum, then scale
// by font size. Across segments: sum those. This reproduces
// positionSegments' cursor arithmetic exactly, because it IS that
// arithmetic — a line's width and the position of the next face
// segment on it are the same number computed once.
//
// No float32 or float64 appears anywhere on this path (AD-23, D-000.25).
func measureRuneRange(segs []faceSegment, from, to int, fontSize geom.Length) geom.Length {
	var total geom.Length
	for _, s := range segs {
		if to <= s.runeStart || from >= s.runeEnd {
			continue
		}
		lo, hi := s.glyphRangeForRunes(maxInt(from, s.runeStart), minInt(to, s.runeEnd))
		if hi <= lo {
			continue
		}
		total += geom.ScaleRound(geom.Length(s.advance1000(lo, hi)), int64(fontSize), 1000)
	}
	return total
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// wrappedLine is one laid-out line of a text element: the element-global
// rune range it draws, the width that range measured to, and the kind of
// break that ended it.
//
// Width is recorded rather than recomputed by the caller, so the number
// the packer decided with and the number anything downstream asserts on
// are the same number.
//
// endedBy IS WRITTEN HERE AND READ BY STORY 7.3 / FR47 (D-7.1.5), which
// must not justify "the last line of a paragraph, or a line ended by a
// mandatory break". It lands at the site that already knows, because
// reconstructing it later would be a SECOND derivation of where a break
// came from — the hazard verticalMetrics' own doc comment says that type
// exists to close. Nothing in production reads it yet; the precedent for
// shipping such a field, and for asserting it directly over fabricated
// input in the meantime, is verticalMetrics.LastDescent.
//
// A line that NO break ended — the last line of an element — carries the
// zero value, text.BreakOptional. That is deliberate and it is why 7.3
// must DERIVE the last-line case from the line's index rather than read
// it here: this field answers "which break ended this line", not "is
// this the last line", and the two are independent conditions.
type wrappedLine struct {
	from, to int
	width    geom.Length
	endedBy  text.BreakKind
}

// packLines is the greedy line breaker: given an element's segments,
// its break opportunities and its declared box width, it returns the
// lines the element occupies, in order.
//
// GREEDY, AND DELIBERATELY SO. Each line takes the LAST opportunity
// that still fits. There is no Knuth-Plass paragraph optimisation here
// and none is implied — folio8 wraps a statement line, not a book.
//
// THE OVERFLOW RULE (AC11, AD-25's own words: "it overflows visibly
// under FR44 — clipped, with a located diagnostic"). When not even the
// FIRST available opportunity fits, the line takes it anyway and
// overflows its box. It is NOT re-broken at a guess, NOT squeezed, and
// NOT silently dropped. The atomic unit that overflowed is an
// unbreakable declared value, an uncoverable Thai run, or an unbroken
// Latin word — in every case something the engine has been told, or has
// determined, it may not split. Clipping and the diagnostic are Story
// 2.8's; this function's obligation is to not paper over it.
//
// A MANDATORY BREAK IS NOT A CANDIDATE, IT IS A BOUND (Story 7.1,
// FR46). Where the caller supplied a line feed the text starts a new
// line, however much width remained — so this function never gets to
// decline one. Three sites would each have declined one silently, and
// all three are handled below: the maxWidth <= 0 early return, the
// "does everything that is left fit?" short-circuit, and the candidate
// loop's greedy "last that fits" (D-7.1.7).
//
// maxWidth <= 0 means "no declared box": the element's lines are
// delimited by mandatory breaks ALONE, and an element carrying none is
// one line, exactly as it rendered before Story 2.4. That is what keeps
// every existing golden still (AC13) — no committed fixture's text
// contains a line feed — though see the Delivery Log for which
// population that claim was measured over.
func packLines(segs []faceSegment, ops []text.Opportunity, totalRunes int, fontSize, maxWidth geom.Length) []wrappedLine {
	if totalRunes == 0 {
		return nil
	}
	if maxWidth <= 0 {
		return packMandatoryOnly(segs, ops, totalRunes, fontSize)
	}

	var lines []wrappedLine
	start := 0
	// endedByMandatory tracks the LAST line appended, so that an input
	// ending ON a mandatory break gets the empty line that break
	// separates from nothing. Mandatory breaks are separators: k of them
	// yield k+1 lines, and "a\n" is two lines with the second empty
	// (D-7.1.2). Without this, a trailing blank line is inexpressible
	// and the character is thrown away without saying so.
	endedByMandatory := false
	// mandCursor is a CARRIED position, not a fresh scan per line. ops
	// is in ascending LineEnd order and start only ever moves forward,
	// so the search for the next mandatory break resumes where the last
	// one left off — otherwise every line of every element would rescan
	// the whole opportunity list, and the population that pays for it is
	// the corpus, none of whose documents carries a line feed at all.
	mandCursor := 0
	for start < totalRunes {
		// The FIRST mandatory break at or after start bounds this line.
		// It is not "a candidate that happens to be late": no line may
		// extend past it, so it is where the candidate set stops.
		mand := -1
		for mandCursor < len(ops) {
			op := ops[mandCursor]
			// Skipping is PERMANENT and that is what makes the cursor
			// sound: start never moves backwards, so a mandatory break
			// already behind it can never qualify again, and an
			// optional one is not what this search is looking for.
			if op.Kind != text.BreakMandatory || op.LineEnd < start {
				mandCursor++
				continue
			}
			mand = mandCursor
			break
		}

		// A mandatory break AT start ends a ZERO-LENGTH line. This is
		// the leading-empty-line case ("\na", and every interior line
		// of a paragraph gap). It is still progress, because the
		// break's NextStart advances past the whitespace run it
		// consumes.
		if mand >= 0 && ops[mand].LineEnd == start {
			lines = append(lines, wrappedLine{from: start, to: start, endedBy: text.BreakMandatory})
			start = ops[mand].NextStart
			endedByMandatory = true
			continue
		}

		// Does everything that is left fit? Then it is the last line —
		// BUT ONLY IF NO MANDATORY BREAK REMAINS. This short-circuit
		// bypasses the opportunity list entirely, so it is exactly
		// where "the break is taken regardless of how much width
		// remained" is won or lost: a line feed in a value that fits
		// its box would otherwise render as one line (D-7.1.7).
		if mand < 0 {
			if w := measureRuneRange(segs, start, totalRunes, fontSize); w <= maxWidth {
				lines = append(lines, wrappedLine{from: start, to: totalRunes, width: w})
				endedByMandatory = false
				break
			}
		}

		chosen := -1
		first := -1
		for i, op := range ops {
			if op.LineEnd <= start {
				continue
			}
			// ops are in ascending LineEnd order, so this bounds the
			// candidate set at the mandatory break rather than
			// filtering around it: the greedy pick below then lands ON
			// the mandatory break whenever it fits, and short of it
			// otherwise.
			if mand >= 0 && op.LineEnd > ops[mand].LineEnd {
				break
			}
			if first < 0 {
				first = i
			}
			// Not short-circuited on the first miss: advances are
			// normally non-negative so width grows with the end
			// position, but GPOS may legitimately produce a negative
			// advance, and a packer that assumed monotonicity would
			// then stop early and under-fill the line for reasons no
			// test would explain.
			if measureRuneRange(segs, start, op.LineEnd, fontSize) <= maxWidth {
				chosen = i
			}
		}

		switch {
		case chosen >= 0:
			op := ops[chosen]
			lines = append(lines, wrappedLine{
				from:    start,
				to:      op.LineEnd,
				width:   measureRuneRange(segs, start, op.LineEnd, fontSize),
				endedBy: op.Kind,
			})
			start = op.NextStart
			endedByMandatory = op.Kind == text.BreakMandatory
		case first >= 0:
			// AC11: the first available unit does not fit. It goes on
			// this line and overflows visibly.
			op := ops[first]
			lines = append(lines, wrappedLine{
				from:    start,
				to:      op.LineEnd,
				width:   measureRuneRange(segs, start, op.LineEnd, fontSize),
				endedBy: op.Kind,
			})
			start = op.NextStart
			endedByMandatory = op.Kind == text.BreakMandatory
		default:
			// No break opportunity remains at all — one atomic unit
			// runs to the end of the element. Same rule: it overflows.
			lines = append(lines, wrappedLine{
				from:  start,
				to:    totalRunes,
				width: measureRuneRange(segs, start, totalRunes, fontSize),
			})
			start = totalRunes
			endedByMandatory = false
		}
	}
	if endedByMandatory {
		lines = append(lines, wrappedLine{from: totalRunes, to: totalRunes})
	}
	return lines
}

// packMandatoryOnly is packLines' maxWidth <= 0 path: with no declared
// box there is no width to break against, so the only breaks that apply
// are the ones the caller supplied. An element carrying none comes back
// as the single line it has always been.
func packMandatoryOnly(segs []faceSegment, ops []text.Opportunity, totalRunes int, fontSize geom.Length) []wrappedLine {
	var lines []wrappedLine
	start := 0
	endedByMandatory := false
	for _, op := range ops {
		if op.Kind != text.BreakMandatory {
			continue
		}
		lines = append(lines, wrappedLine{
			from:    start,
			to:      op.LineEnd,
			width:   measureRuneRange(segs, start, op.LineEnd, fontSize),
			endedBy: text.BreakMandatory,
		})
		start = op.NextStart
		endedByMandatory = true
	}
	switch {
	case start < totalRunes:
		lines = append(lines, wrappedLine{
			from:  start,
			to:    totalRunes,
			width: measureRuneRange(segs, start, totalRunes, fontSize),
		})
	case endedByMandatory:
		// The value ended ON a break: k breaks, k+1 lines.
		lines = append(lines, wrappedLine{from: totalRunes, to: totalRunes})
	}
	return lines
}

// placeholderSpans reports the rune span of every "{{ }}" placeholder in
// an element's UNBOUND value, delimiters included, for the canvas.
//
// The canvas paints the expression source, not a resolved value, and the
// source of a formatNumber or formatDate call carries spaces a Latin
// break would take — so without this the designer wraps an expression
// the PDF prints as one short value. Kept whole, a long expression
// overflows its box on one line and is clipped, as a table cell's
// binding is. Render never calls this: its placeholders are gone by the
// time it breaks lines.
//
// A value the scanner refuses (an unterminated "{{") gets no spans; its
// load error is reported elsewhere and the canvas still paints it.
func placeholderSpans(value string) []text.Span {
	literal, placeholders, _, err := expr.ScanPlaceholders(value)
	if err != nil {
		return nil
	}
	spans := make([]text.Span, 0, len(placeholders))
	at := 0
	for i, ph := range placeholders {
		at += utf8.RuneCountInString(literal[i])
		end := at + utf8.RuneCountInString(ph.Inner) + 4
		spans = append(spans, text.Span{Start: at, End: end})
		at = end
	}
	return spans
}

// atomicSpansFor maps a document's declared unbreakable data paths onto
// the rune spans one element's binding actually substituted.
//
// The declaration is document-level and names DATA PATHS (D-2.4.1); the
// spans are per-element and come from internal/bind's report of what it
// substituted where. This function is the only place the two meet, and
// it produces the parameter internal/text receives — internal/text
// never learns where the spans came from (D-000.16).
//
// Coverage is by CONSTRUCTION over the substitution set: every
// substituted span whose path is declared, whatever its script and
// whatever its content. There is no list of sample values anywhere on
// this path (D-000.23).
func atomicSpansFor(declared []string, subs []bind.Substitution) []text.Span {
	if len(declared) == 0 || len(subs) == 0 {
		return nil
	}
	declaredSet := make(map[string]bool, len(declared))
	for _, p := range declared {
		declaredSet[p] = true
	}
	var spans []text.Span
	for _, s := range subs {
		if declaredSet[s.Path] {
			spans = append(spans, text.Span{Start: s.Start, End: s.End})
		}
	}
	return spans
}

// THE VERTICAL MODEL (D-2.4.2 as AMENDED). One rule, three maxima.
//
// A chain declares what MAY appear in an element. The vertical model
// must accommodate what MAY appear — and it must do so PER AXIS,
// because ascent and descent are independent axes and the worst case
// takes the worst of each:
//
//	span                        value
//	-------------------------   -------------------------------
//	top -> first baseline       max(A)
//	baseline -> baseline        max(A) + max(D) + max(gap)
//	last baseline -> bottom     max(D)
//
// each maximised INDEPENDENTLY over the faces of the DECLARED chain
// that are present in the supplied FontSet, scaled to fontSize.
//
// # What the amendment corrected, and why the original was wrong
//
// D-2.4.2 as first ruled took the inter-baseline distance to be
// max(A - D + gap) over the chain — the worst SINGLE FACE. That
// maximises the wrong quantity. The space between two baselines must
// hold line N's DESCENDERS plus line N+1's ASCENDERS, so the constraint
// is the worst adjacent PAIR, and the original form implicitly assumed
// one face supplies both axes. That is FALSE on the shipped set, where
// the two axes are won by DIFFERENT faces:
//
//	face             A       D      gap    A - D + gap
//	Noto Sans        1069    -293   0      1362
//	Noto Sans Thai   1061    -450   0      1511   <- wins DESCENT
//	Noto Sans SC     1160    -288   0      1448   <- wins ASCENT
//
//	worst pair = max(A) + max(|D|) = 1160 + 450 = 1610
//	superseded = max(A - D + gap)               = 1511
//	shortfall                                   =   99
//
// So the superseded rule under-spaced the shipped chain by 99 units of
// the em — a potential ink overlap between a Thai line's below-vowels
// and the next line's ideograph ascenders, in the DEFAULT chain, on the
// scripts this epic exists to support.
//
// Note what the amendment does NOT change, because it bounds the blast
// radius: for a chain resolving to ONE present face, max(A) + max(|D|)
// + max(gap) is IDENTICALLY A - D + gap. A single face cannot fail to
// supply both axes, so every single-face chain's advance is unchanged.
// Measured: the shipped Noto x3 chain moves 1511 -> 1610; ["Noto Sans"]
// and the Roboto test face do not move at all.
//
// # Why the maximum over the chain, and why that is not content-dependent
//
//	A chain declares what MAY appear in an element. The vertical model
//	must accommodate what MAY appear — not what DOES appear.
//
// "What does appear" is content-dependent, and content-dependent
// vertical placement is forbidden: adding one CJK character would
// reflow the element, which AD-24's "boxes are absolute, and nothing
// negotiates" rules out. "What may appear" is exactly the declared
// chain — a static property of the template, identical for every value
// the element is ever bound to. So all three spans depend on the chain
// and the size and on nothing that is drawn.
//
// # Why the FIRST baseline uses max(A), and why that is no longer a judgment
//
// Before the amendment, "whose ascent places the first baseline" looked
// like a free choice between the tallest ascent (1160) and the ascent
// of whichever face won the advance maximisation (1061). It is not a
// choice: the first span is the same accommodate-what-may-appear
// question asked about the ascent axis alone, and its answer is max(A)
// by the same argument the other two spans use. Under the alternative,
// an ideograph on the first line overshoots the element's declared top
// by 99 units of the em. That the two candidates coincided on the
// shipped set only — both landing on a Noto face — is exactly the
// fit-to-the-sample hazard D-000.32 names.
//
// The defect this replaces on the first span was worse than either
// candidate: internal/pdf placed the first baseline at run.FontSize
// below the element top — the point SIZE used as a proxy for an ascent
// it has no relationship to (DW-15). It is off by max(A) - 1000 units
// per em, which is +160/em on the shipped chain and -72/em on the
// Roboto test face. NOTE THE SIGN: it is not a consistent downward
// drift, it flips direction with the face, so no assertion anywhere may
// be phrased as a DIRECTION (D-000.45).
//
// # Why hhea, and not OS/2 typo metrics or a multiple of the size
//
// Measured. Noto Sans SC declares USE_TYPO_METRICS = false (fsSelection
// 0x0040), so its sTypoAscender/Descender of 880/-120 are explicitly
// NOT its line metrics — yet 1000/em is a perfectly plausible number,
// and it is below that face's own 1448. That is the same class of
// fiction as a substituted /CapHeight. And a fixed multiple fails
// outright: 1.2 em is below all three shipped faces, and even 1.5 em is
// below the ruled 1610.
//
// The bounded cost, stated: a Latin-only element in a Latin+Thai+CJK
// chain gets taller lines than Noto Sans alone would need. The author
// declared that chain; an author who wants Latin metrics declares a
// Latin-only chain. No element pays for a face its own chain does not
// name.

// verticalMetrics is one declared chain's finished vertical model at one
// font size, in millipoints. Every field is derived from the SAME chain
// walk, which is the whole point of the type existing: two independent
// walks answering two halves of one geometric question is precisely the
// defect Story 2.5a fixed, and a struct returned from one walk is what
// stops it being recreated one level up.
type verticalMetrics struct {
	// FirstBaseline is top-of-element -> first baseline: max(A) scaled.
	FirstBaseline geom.Length

	// Advance is baseline -> baseline: max(A)+max(|D|)+max(gap) scaled.
	Advance geom.Length

	// LastDescent is last baseline -> the bottom of the text's vertical
	// extent: max(|D|) scaled.
	//
	// It is the third span of the ruled model, computed here so the model
	// is stated ONCE, in the one place that walks the chain, rather than
	// re-derived by whoever first needs it — the second-derivation hazard
	// this type exists to close. It went a long time with no production
	// consumer at all, stated honestly as such. Two consume it now: the
	// per-line item extent the page splitter reads, and textBlockHeight,
	// which is the height vertical alignment distributes an element's
	// slack against (text_alignment.go). It is also asserted directly by
	// TestVerticalModelArithmeticOverFabricatedMetrics.
	LastDescent geom.Length
}

// chainLineMetrics is THE place in this module that walks a declared
// chain, decides which of its members are present in the supplied
// FontSet, tolerates an absent member, and reports the line metrics of
// those that remain. It is the same "chain entry present in fs"
// tolerance fontChain and resolveRuneFace already apply.
//
// It reads (*fontset.Font).LineMetrics and NOTHING else. That reads the
// hhea TABLE, never (*ot.Face).Ascender/Descender/LineGap, which
// substitute 800/-200/0 for an absent table (D-2.4.2 constraint 2: the
// vertical model must never inherit a substituted default), and
// requireReadableTables makes an absent hhea a load error. In
// particular it must never read (*fontset.Font).Metrics's Ascent, which
// IS populated through the substituting vendor accessor and exists for
// the PDF /FontDescriptor, not for placement.
func chainLineMetrics(chain []string, fs FontSet, cache *fontCache) ([]fontset.LineMetrics, error) {
	out := make([]fontset.LineMetrics, 0, len(chain))
	for _, name := range chain {
		// A chain member the caller did not supply cannot appear in the
		// element, so it does not constrain the vertical model. Since
		// Story 8.4 the same tolerance covers a member the DOCUMENT
		// carries whose asset this build cannot read as a font: it cannot
		// appear in the element either. metricsFace is where that one
		// rule is stated, and where the asymmetry with resolveRuneFace —
		// which REFUSES the same entry, because it is being asked to draw
		// with it — is written down.
		f, present, err := cache.metricsFace(name, fs)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		out = append(out, f.LineMetrics())
	}
	return out, nil
}

// verticalModel is the PURE arithmetic of the ruled model: given the
// line metrics of a chain's present faces and a font size, produce the
// three spans.
//
// It takes metrics as a VALUE and never touches a *fontset.Font, and
// that shape is load-bearing rather than tidy. hhea lineGap is 0 on all
// four faces this repository commits, so the lineGap term is
// BYTE-NEUTRAL on every artifact that ships: no golden can distinguish
// a model that includes it from one that drops it, and strengthening a
// golden assertion to try would manufacture a guard against a
// difference that does not exist on this input set and would fire on a
// legitimate refactor (D-000.39 sharpened). The only way that term can
// have teeth is a direct unit test over FABRICATED metrics carrying a
// non-zero LineGap — which is possible only if the arithmetic is
// reachable without a font. Hence this signature.
//
// chain is carried for the error messages alone; nothing is read from
// it, so a test may pass any label it likes.
//
// lineSpacing is Story 7.2's author-set leading ratio in THOUSANDTHS
// (template.LineSpacingUnit is the neutral value, and an absent
// `style.lineSpacing` passes exactly that). It is threaded as a
// parameter for the same reason fontSize is: neither constructor takes
// a style, and the vertical model is a function of the chain, the size
// and now the ratio — of nothing that is drawn.
func verticalModel(chain []string, metrics []fontset.LineMetrics, fontSize geom.Length, lineSpacing int64) (verticalMetrics, error) {
	if len(metrics) == 0 {
		return verticalMetrics{}, fmt.Errorf(
			"folio8: none of the fallback chain's faces %v is present in the supplied FontSet, so no line height can be derived from it",
			chain,
		)
	}

	// Each axis maximised INDEPENDENTLY — the correction the amendment
	// turns on. Taking max(A - D + gap) here instead would silently
	// re-assume that one face supplies both axes.
	//
	// The maxima are clamped at zero: a face declaring a negative hhea
	// ascent must not pull the first baseline ABOVE the element's top,
	// and a positive hhea descent must not pull the bottom above the
	// baseline.
	var maxAscent, maxDescent, maxLineGap int64
	for _, lm := range metrics {
		if lm.Ascent > maxAscent {
			maxAscent = lm.Ascent
		}
		if d := -lm.Descent; d > maxDescent {
			maxDescent = d
		}
		if lm.LineGap > maxLineGap {
			maxLineGap = lm.LineGap
		}
	}

	units := maxAscent + maxDescent + maxLineGap
	if units <= 0 {
		return verticalMetrics{}, fmt.Errorf(
			"folio8: the fallback chain %v yields a line height of %d font units — over its present faces max(hhea ascent)=%d, max(-hhea descent)=%d and max(hhea lineGap)=%d sum to nothing a line can be drawn in",
			chain, units, maxAscent, maxDescent, maxLineGap,
		)
	}

	scale := func(u int64) geom.Length {
		return geom.ScaleRound(geom.Length(u), int64(fontSize), 1000)
	}

	// THE ONE SITE THE RATIO IS APPLIED AT. FirstBaseline and
	// LastDescent are built from their own maxima and are not touched,
	// which is what makes "lineSpacing scales Advance and nothing else"
	// true BY CONSTRUCTION rather than by discipline — a multi-line
	// element's top edge cannot move, so no sibling can appear to jump
	// (D-2.5a/DW-15's two-model split; Story 2.5a exists solely because
	// the two were once conflated).
	//
	// Everything downstream inherits the ratio for free precisely
	// because it is applied HERE: the three longhand block-height copies
	// (text_alignment.go's textBlockHeight, table_render.go's rowHeight
	// and footerRowHeight) and the four `origin + i*Advance` stepping
	// loops all read this Advance. Scaling at any consumer instead would
	// need all seven to agree forever.
	ruledAdvance := scale(units)
	advance, err := scaleAdvanceByLineSpacing(chain, fontSize, ruledAdvance, lineSpacing)
	if err != nil {
		return verticalMetrics{}, err
	}
	return verticalMetrics{
		FirstBaseline: scale(maxAscent),
		Advance:       advance,
		LastDescent:   scale(maxDescent),
	}, nil
}

// scaleAdvanceByLineSpacing applies the leading ratio to the ruled
// advance, and is where D-7.2.4's two typographic failures are refused —
// both of them CHECKED WHERE BOTH OPERANDS EXIST, which is the whole
// reason neither is a load-time bound: a load-time check cannot see the
// font size.
//
//  1. OVERFLOW. geom.ScaleRound PANICS when the exact product v*num
//     overflows int64, and a Go panic aborts the package's whole test
//     binary — every other test in folio8-go then silently stops
//     reporting, which is a suite-wide blindfold rather than a crash. So
//     the precondition is discharged BEFORE the call.
//
//     SCOPED HONESTLY: this closes the panic route THE RATIO OPENS, and
//     only that one. verticalModel's own `scale` closure multiplies by an
//     UNBOUNDED authored fontSize and is NOT guarded, so a panic remains
//     reachable from a template through that field alone, with no
//     lineSpacing declared anywhere. That is DW-26, recorded rather than
//     closed because bounding fontSize is a format-domain decision about
//     a second field (D-7.2.4). Do not read this guard as making the
//     function panic-free.
//
//  2. A RESOLVED ADVANCE OF ZERO — but ONLY ONE THE RATIO CAUSED.
//     ScaleRound returns 0 whenever v*num < den/2 — measured:
//     ScaleRound(400, 1, 1000) is 0 — so a small face at lineSpacing
//     0.001 yields zero-height lines, which layout cannot draw and the
//     canvas correctly refuses. It is a DISTINCT condition from the
//     load-time range and carries its own error, never the load-time
//     range's refusal: raising the load-time minimum to
//     prevent it would only move the blindness.
//
//     THE PASS-THROUGH BELOW IS LOAD-BEARING, NOT A CONVENIENCE. A ruled
//     advance that is ALREADY non-positive is a degeneracy this story
//     did not introduce and must not start refusing: it comes from an
//     unbounded authored `fontSize` (DW-26, deliberately left open),
//     it is reachable with no `lineSpacing` declared anywhere, and a
//     document carrying none must produce byte-identical output to
//     today. Refusing it here would make the neutral ratio reject
//     templates that render at this story's baseline. Only a POSITIVE
//     ruled advance driven to zero BY the ratio is this story's to
//     refuse.
func scaleAdvanceByLineSpacing(chain []string, fontSize geom.Length, ruledAdvance geom.Length, lineSpacing int64) (geom.Length, error) {
	if ruledAdvance <= 0 {
		// The degeneracy predates the ratio, so the ratio has no opinion
		// about it. Hand back exactly what the unscaled model produced.
		return ruledAdvance, nil
	}
	if int64MulWouldOverflow(int64(ruledAdvance), lineSpacing) {
		return 0, fmt.Errorf(
			"folio8: line spacing %s applied to the ruled advance of %d millipoints (chain %v at font size %d millipoints) overflows int64",
			template.FormatLineSpacing(lineSpacing), ruledAdvance, chain, fontSize,
		)
	}
	advance := geom.ScaleRound(ruledAdvance, lineSpacing, template.LineSpacingUnit)
	if advance <= 0 {
		return 0, fmt.Errorf(
			"folio8: line spacing %s applied to the ruled advance of %d millipoints (chain %v at font size %d millipoints) resolves to an advance of %d — a line cannot be drawn in it",
			template.FormatLineSpacing(lineSpacing), ruledAdvance, chain, fontSize, advance,
		)
	}
	return advance, nil
}

// int64MulWouldOverflow reports whether a*b cannot be represented as an
// int64. It is deliberately a ROOT-PACKAGE, UNEXPORTED predicate sitting
// beside the one guard that uses it, and not a call into internal/geom.
//
// internal/geom already has this arithmetic (int64MulOverflows), but it
// is unexported and must stay that way: scale_surface_test.go asserts
// that package's exported surface is exactly {ScaleRound}, and AD-2's
// "scaling is one function" is what that test protects. A local overflow
// PREDICATE is not a second scaling door — it decides whether the one
// door may be opened.
func int64MulWouldOverflow(a, b int64) bool {
	if a == 0 || b == 0 {
		return false
	}
	if (a == minInt64 && b == -1) || (b == minInt64 && a == -1) {
		return true
	}
	p := a * b
	return p/b != a
}

// minInt64 is math.MinInt64, spelled as a literal to keep this file's
// import list unchanged.
const minInt64 = -1 << 63

// chainVerticalModel is the production entry point: ONE chain walk
// feeding the one arithmetic. A caller needing both the first-baseline
// offset and the inter-baseline advance takes both from a single call —
// AC1's "there is no second derivation left to drift".
func chainVerticalModel(chain []string, fontSize geom.Length, lineSpacing int64, fs FontSet, cache *fontCache) (verticalMetrics, error) {
	metrics, err := chainLineMetrics(chain, fs, cache)
	if err != nil {
		return verticalMetrics{}, err
	}
	return verticalModel(chain, metrics, fontSize, lineSpacing)
}

// lineAdvance is the inter-baseline span of the ruled model, kept under
// the name the rest of the module and its tests already use. It is a
// PROJECTION of chainVerticalModel, never a second derivation.
func lineAdvance(chain []string, fontSize geom.Length, lineSpacing int64, fs FontSet, cache *fontCache) (geom.Length, error) {
	vm, err := chainVerticalModel(chain, fontSize, lineSpacing, fs, cache)
	if err != nil {
		return 0, err
	}
	return vm.Advance, nil
}
