package text

import (
	"fmt"

	"github.com/boxesandglue/textshape/ot"
)

// ShapedGlyph is one glyph produced by the OpenType shaper, in FONT
// UNITS, with every numeric field an exact integer (AD-2, AD-23).
//
// The five fields are not a convenience selection: they are the five
// independent places "this run is correctly shaped" is represented, and
// D-000.23's consequent obligation is that a guard cover every one of
// them rather than the one that happened to burn us. GlyphID carries
// GSUB's answer (Thai's lowered mark forms, Latin's ligatures);
// XAdvance carries GPOS kerning; XOffset carries mark positioning;
// YOffset carries vertical mark positioning — RED-PROVED against the
// shipped Noto Sans Thai by internal/fontset/shape_shipped_face_test.go, which shapes
// ทั้ and finds a displaced glyph, with ที่ as the control that comes
// back at zero because the face resolves that pair by a GSUB
// lowered-form substitution instead; Cluster
// carries which source runes the glyph came from, which is what
// /ToUnicode is rebuilt out of once shaping breaks the one-rune-one-
// glyph assumption.
//
// No float appears here or anywhere on this path. ot.GlyphPos is
// int16 throughout and the shaper does no scaling, so the shaped output
// is exact integer data in font units; scaling to the PDF's 1000-unit em
// happens exactly once, in geom.ScaleRound, at the emission site.
// WHAT THIS COMMENT USED TO SAY, AND WHY IT IS WORTH RECORDING. Until
// 2026-08-31 the YOffset clause above read "zero for every glyph of
// every sample across all three shipped faces today", and called the
// guard over it "a FORWARD GUARD WITH NO AVAILABLE RED-PROOF, never a
// red-proved one". Both halves were false. The claim traces to Story
// 2.3, which measured ITS OWN SAMPLES and reported on THE SHIPPED SET —
// two different populations — and it then propagated into four more
// places, textdoc.go's fail-closed branch among them, where it stood as
// the stated justification for there being no render-path test.
//
// A large class of ordinary Thai consequently did not render at all,
// and it was the project's owner who found that in production rather
// than any test, because a comment asserting a negative is what a
// reader checks BEFORE they go looking — so a wrong one protects
// itself (DW-28, D-8.0.1).
//
// A COMMENT THAT ASSERTS A NEGATIVE — unreachable, never, impossible —
// CARRIES THE SAME EVIDENTIARY BURDEN AS A TEST, AND MUST NAME THE
// POPULATION IT MEASURED RATHER THAN THE POPULATION IT CONCLUDED ABOUT.
type ShapedGlyph struct {
	// GlyphID is the glyph index in the SOURCE face's numbering — not a
	// subset glyph id and not a PDF CID. internal/fontset maps it
	// through the subset plan; internal/pdf never sees this number.
	GlyphID uint16

	// Cluster is the index of the first RUNE of the source cluster this
	// glyph belongs to.
	//
	// Measured at textshape v0.0.15, and cross-checked against
	// hb-shape 14.2.0, which agrees value for value: clusters are RUNE
	// indices, not byte offsets. ("office" -> 0,1,4,5 is ambiguous
	// between the two readings because it is ASCII; "ณัฐวุฒิ" ->
	// 0,0,2,3,3,5,5 is not — byte offsets would be multiples of 3.)
	// TestClusterValuesAreRuneIndices pins this, because ClusterTexts
	// below indexes a []rune with it.
	Cluster int

	// XAdvance is the glyph's horizontal advance in font units,
	// INCLUDING any GPOS kerning. It is deliberately not
	// ot.Face.HorizontalAdvance, for two independent reasons; see the
	// Shape method's comment.
	XAdvance int16

	// XOffset / YOffset are GPOS's placement of this glyph relative to
	// the pen position, in font units.
	XOffset int16
	YOffset int16
}

// Shaper is one face's OpenType shaper, constructed ONCE per face and
// reused across every shaping call — the vendor's documented contract
// ("A Shaper is created once per font and reused across shaping calls"),
// and the reason this type exists at all rather than a bare function.
// internal/fontset.Font owns exactly one of these and hands it out
// through an accessor whose return type is this folio8 type, so no vendor
// pointer crosses that seam (AC17a, D-1.5.10).
//
// Nothing here selects a weight, an instance or an axis. ot.Shaper
// exposes SetVariations, SetVariation, SetNamedInstance,
// SetSyntheticBold and SetSyntheticSlant; NONE of them is called, and
// none may be added (D-2.2.1, D-2.2.4).
//
// CORRECTION, D-2.2.4 (correction) / Story 2.3a AC9. This comment used
// to justify that by saying "four take float32, which
// internal/arch_test.go bans under internal/ and the module root
// (AD-23)". THAT MECHANISM WAS FALSE. internal/arch_test.go matches the
// SPELLING of a type identifier and the kind of a literal; an untyped
// integer constant handed to such a parameter writes no identifier and
// is a BasicLit of kind INT, so it passes the guard untouched.
// internal/fontset/fontset_test.go's
// TestSubsetPinnedInstancesProduceDifferentTags calls the vendor's
// PinAxisLocation with the constant 700 today, guard green. A reader
// trusting the old sentence would believe AD-23 fenced a door that was
// open.
//
// The conclusion is unchanged and rests on its own reasons: none of the
// five is called, the four variation setters would select an instance
// the project deliberately does not ship, and the fifth would fabricate
// a weight. What is new is that AD-23 now genuinely reaches this shape —
// lint's type-aware no-float-typed-value rule matches on the RESOLVED
// type, so any call passing a fractional value to any of them is
// reported whether or not a type is ever spelled. The shipped faces are static,
// Regular-only, and a caller-supplied variable face is rejected at
// ingestion by fontset.New rather than instanced here.
type Shaper struct {
	name string
	sh   *ot.Shaper
}

// NewShaper builds the one Shaper for face. name is the caller's face
// name and is used only in error messages.
func NewShaper(name string, face *ot.Face) (*Shaper, error) {
	sh, err := ot.NewShaperFromFace(face)
	if err != nil {
		return nil, fmt.Errorf("text: face %q: build shaper: %w", name, err)
	}
	return &Shaper{name: name, sh: sh}, nil
}

// Name returns the face name this Shaper was constructed with.
func (s *Shaper) Name() string { return s.name }

// Shape shapes one contiguous, single-face, single-script segment and
// returns its glyphs in drawing order.
//
// The segment boundary matters and is the caller's job: the fallback
// chain's per-rune coverage resolution (Story 2.2) has already split the
// text into maximal runs sharing one face, and GuessSegmentProperties
// below derives script and direction from the buffer's OWN contents — so
// handing it a mixed-script string would let one script's rules govern
// another's characters. One buffer per face-segment, never one per
// element and never one per document.
//
// Only the shaper's DEFAULT feature set runs (Shape(buf, nil)). No
// smcp, no onum, no ss01, no hand-built Feature list: discretionary
// typography is not this seam's to decide.
//
// Advances come from GlyphPos.XAdvance (int16), never from
// ot.Face.HorizontalAdvance, and the reason is two independent defects
// rather than one. HorizontalAdvance returns float32 — which
// internal/arch_test.go's guard cannot see through an int64(x)
// conversion, because that conversion names neither banned identifier —
// and it reports the hmtx advance, which OMITS GPOS kerning. For "AV" in
// Noto Sans the shaped advance of 'A' is 599 against an hmtx 639: the
// accessor is wrong about the number as well as wrong about the type.
func (s *Shaper) Shape(text string) ([]ShapedGlyph, error) {
	if text == "" {
		return nil, nil
	}

	buf := ot.NewBuffer()
	buf.AddString(text)
	buf.GuessSegmentProperties()
	s.sh.Shape(buf, nil)

	// The vendor returns two PARALLEL slices; a length disagreement
	// between them would silently truncate or misalign the run, so it is
	// checked rather than assumed (D-000.21 sharpened: prove the
	// artifact carries the fields before reading them).
	if len(buf.Info) != len(buf.Pos) {
		return nil, fmt.Errorf(
			"text: face %q: shaper returned %d glyph infos but %d glyph positions",
			s.name, len(buf.Info), len(buf.Pos),
		)
	}

	// Ranges a SLICE, never a map (D-1.3.5, ScanMapRange), so the output
	// order is the shaper's drawing order and nothing else.
	out := make([]ShapedGlyph, 0, len(buf.Info))
	for i := range buf.Info {
		out = append(out, ShapedGlyph{
			GlyphID:  uint16(buf.Info[i].GlyphID),
			Cluster:  buf.Info[i].Cluster,
			XAdvance: buf.Pos[i].XAdvance,
			XOffset:  buf.Pos[i].XOffset,
			YOffset:  buf.Pos[i].YOffset,
		})
	}
	return out, nil
}

// ClusterTexts returns, for each glyph of a shaped run, the source text
// that glyph carries for /ToUnicode purposes: the cluster's FULL rune
// sequence on the cluster's FIRST glyph, and the empty string on every
// other glyph of the same cluster.
//
// This is D-2.3-Q1(a), and the alternative it rules out is the reason it
// is worth a function. Mapping every glyph of a cluster to the cluster's
// text satisfies "every CID has an entry" perfectly and makes "น้ำ"
// extract as "น้ำน้ำน้ำน้ำ" — the round-trip is what distinguishes
// correct from plausible.
//
// It is a pure function of (source, glyphs) so that it can be tested
// without a font, and so the caller — which is also the site that
// allocates CIDs — sees the empty strings explicitly rather than
// inferring them.
func ClusterTexts(source string, glyphs []ShapedGlyph) ([]string, error) {
	runes := []rune(source)

	texts := make([]string, len(glyphs))
	for i, g := range glyphs {
		if g.Cluster < 0 || g.Cluster > len(runes) {
			return nil, fmt.Errorf(
				"text: glyph %d reports cluster %d, outside the source's %d runes",
				i, g.Cluster, len(runes),
			)
		}
		if i > 0 && glyphs[i-1].Cluster == g.Cluster {
			// Not the first glyph of its cluster: contributes no text.
			texts[i] = ""
			continue
		}
		// First glyph of this cluster: it carries every rune from this
		// cluster's start up to the next cluster's start (or the end of
		// the source). Found by scanning FORWARD through the glyph
		// slice, never by assuming clusters are contiguous integers —
		// they are not: a ligature's cluster is followed by the cluster
		// of its last component plus one.
		end := len(runes)
		for j := i + 1; j < len(glyphs); j++ {
			if glyphs[j].Cluster != g.Cluster {
				end = glyphs[j].Cluster
				break
			}
		}
		if end < g.Cluster || end > len(runes) {
			return nil, fmt.Errorf(
				"text: glyph %d's cluster %d ends at rune %d, which is not a valid span of the source's %d runes",
				i, g.Cluster, end, len(runes),
			)
		}
		texts[i] = string(runes[g.Cluster:end])
	}
	return texts, nil
}
