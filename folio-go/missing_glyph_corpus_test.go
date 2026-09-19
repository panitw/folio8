package folio8

import (
	"strings"
	"testing"
)

// TestCorpusFixturesProduceNoMissingGlyphWarnings is the obligation the
// engineering lead attached to OPEN-1's ruling (Story 3.6): rendering
// any COMMITTED fixture must produce ZERO missing-glyph Warnings. This
// is what stops the corpus quietly normalising a dropped character —
// D-000.22's own warning, "a wrong first recording is not a bug that
// gets caught later, it is a bug that gets RATIFIED later" — applied to
// a Warning whose only in-band effect is a silently shorter string.
//
// It renders every fixture this module commits to its acceptance
// corpus — every document first_baseline_acceptance_test.go's
// baselineAcceptanceFixtures walks, plus any fixture declared in
// beyondBaselineAcceptance below — and asserts the COUNT of
// DiagCodeTextMissingGlyph diagnostics is zero — DERIVED by counting
// res.Diagnostics, never a literal len(res.Diagnostics) == 0, because
// at least one of these fixtures (wrapped-text, e4) legitimately
// carries an UNRELATED Warning (DiagCodeTextClippedWidth) that this
// test must not treat as a violation.
//
// This is deliberately over the COMMITTED corpus only. The genuinely
// uncoverable-rune subject this story needs for AC4's own coverage
// (TestMissingGlyphDiagnosticFiresOnUncoveredRune, ac4_coverage_test.go)
// is a SYNTHETIC, test-only template built inline in that test, never
// added here or to any retained fixture directory — the two obligations
// would look like they conflict without that said explicitly, so it is
// said here as well as in that test's own doc comment.
func TestCorpusFixturesProduceNoMissingGlyphWarnings(t *testing.T) {
	type corpusFixture struct {
		name string
		tpl  func(t *testing.T) *Template
		data Data
		fs   func(t *testing.T) FontSet
	}

	fixtures := []corpusFixture{
		{
			name: "font-text",
			tpl:  func(t *testing.T) *Template { return parseFontTestTemplate(t) },
			data: Data("{}"),
			fs:   func(t *testing.T) FontSet { return testFontSet() },
		},
		{
			name: "multi-script-fallback",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(multiScriptTestTemplateJSON))
				if err != nil {
					t.Fatalf("parse multi-script template: %v", err)
				}
				return tpl
			},
			data: Data("{}"),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			name: "shaped-text",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(shapedTextTemplateJSON))
				if err != nil {
					t.Fatalf("parse shaped-text template: %v", err)
				}
				return tpl
			},
			data: Data("{}"),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			name: "three-band-page",
			tpl:  func(t *testing.T) *Template { return parseThreeBandPageTemplate(t) },
			data: Data("{}"),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			name: "wrapped-text",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(wrappedTextTemplateJSON))
				if err != nil {
					t.Fatalf("parse wrapped-text template: %v", err)
				}
				return tpl
			},
			data: Data(wrappedTextDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 7.1. Added here per its own task list, and it is
			// the FIRST entry that is not also one of Story 2.5a's five
			// re-recorded goldens — see beyondBaselineAcceptance below
			// for why the two tables are no longer required to be
			// equal, and for what still holds them together.
			name: "mandatory-break",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(mandatoryBreakTemplateJSON))
				if err != nil {
					t.Fatalf("parse mandatory-break template: %v", err)
				}
				return tpl
			},
			data: Data(mandatoryBreakDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 7.2. Like mandatory-break above, not one of Story
			// 2.5a's five re-recorded goldens — declared in
			// beyondBaselineAcceptance below.
			name: "line-spacing",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(lineSpacingTemplateJSON))
				if err != nil {
					t.Fatalf("parse line-spacing template: %v", err)
				}
				return tpl
			},
			data: Data(lineSpacingDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 7.3, and it matters here specifically: justification
			// splits a line into several positioned pieces, so this is
			// the first document whose runs are word-grained. A missing
			// glyph that only appeared at a piece boundary would show up
			// nowhere else.
			name: "justified-text",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(justifiedTemplateJSON))
				if err != nil {
					t.Fatalf("parse justified-text template: %v", err)
				}
				return tpl
			},
			data: Data(justifiedDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 7.3, owner scope amendment. It matters here
			// specifically: this is the only committed document whose
			// word-grained pieces are cut at DICTIONARY seams rather
			// than at spaces, so a missing glyph exposed by a Thai
			// piece boundary would show up nowhere else.
			name: "justified-thai",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(justifiedThaiTemplateJSON))
				if err != nil {
					t.Fatalf("parse justified-thai template: %v", err)
				}
				return tpl
			},
			data: Data(justifiedThaiDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 7.7 (FR51). It matters here specifically because it
			// is the first committed document whose content is split
			// across two pages by an AUTHOR'S declaration rather than by
			// the four pagination rules alone — a missing glyph on the
			// far side of a break the author asked for would show up
			// nowhere else.
			name: "keep-together",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(keepTogetherTemplateJSON))
				if err != nil {
					t.Fatalf("parse keep-together template: %v", err)
				}
				return tpl
			},
			data: Data(keepTogetherDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 8.0 (DW-28). It matters here specifically because it
			// is the only committed document whose runs are SPLIT by a
			// text rise: a segment boundary is a fresh show-text
			// operator, and a glyph dropped at one of those boundaries
			// would show up nowhere else in the corpus.
			name: "thai-stacked-marks",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(thaiStackedMarksTemplateJSON))
				if err != nil {
					t.Fatalf("parse thai-stacked-marks template: %v", err)
				}
				return tpl
			},
			data: Data(thaiStackedMarksDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 8.3 (FR53/FR56), rendering from the carried face since
			// Story 8.4 (FR54). It matters here specifically because it is
			// the only committed document whose text is covered by NO face
			// the caller supplies: its runes are Thai, its chain's first
			// entry is the shipped Latin Noto Sans, and the only face that
			// covers them is the one the DOCUMENT carries.
			//
			// So this row is now a direct test of the feature, in the
			// negative form this table is built for. An implementation that
			// dropped the embedded entry — the pre-8.4 boundary — would
			// leave every rune on the page uncovered and would emit one
			// missing-glyph Warning per distinct rune, and this is where
			// that is seen. It reads as a coverage test because a chain
			// that silently stops covering its text IS a missing-glyph
			// warning.
			name: "embedded-font",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(embeddedFontTemplateJSON()))
				if err != nil {
					t.Fatalf("parse embedded-font template: %v", err)
				}
				return tpl
			},
			data: Data("{}"),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 11.5 (DW-237). It matters here specifically because
			// it is the only committed document that asks its chain for
			// a CUT: coverage chooses an entry on its BASE face, and the
			// declared variant is then applied WITHIN that entry — so a
			// rune the base face covers and the variant does not is a
			// case no other fixture in this table can reach. Roboto's
			// four cuts all cover this document's Latin, which is
			// exactly why a Warning here would mean the resolution went
			// somewhere it should not have.
			name: "declared-variants",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(declaredVariantsTemplateJSON))
				if err != nil {
					t.Fatalf("parse declared-variants template: %v", err)
				}
				return tpl
			},
			data: Data("{}"),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
		{
			// Story 7.3, closing DW-24.
			name: "alignment-rounding",
			tpl: func(t *testing.T) *Template {
				tpl, err := ParseTemplate([]byte(alignmentRoundingTemplateJSON))
				if err != nil {
					t.Fatalf("parse alignment-rounding template: %v", err)
				}
				return tpl
			},
			data: Data(alignmentRoundingDataJSON),
			fs:   func(t *testing.T) FontSet { return testShippedFontSet() },
		},
	}

	// Finding 11 (QA review, Minor): a length check alone cannot see a
	// fixture SWAP (count unchanged, membership changed) — this test's
	// own doc comment promises "the two must name the same committed
	// corpus," which a count does not verify. Compared here as sets of
	// NAMES, both directions, so a swap in either table reddens.
	//
	// WIDENED BY STORY 7.1, DELIBERATELY AND NARROWLY. The two tables
	// were required to be EQUAL, which silently made this corpus
	// unable to grow: baselineAcceptanceFixtures is Story 2.5a's list
	// of THE FIVE GOLDENS THAT STORY RE-RECORDED — its own
	// TestFirstBaselineSemanticAcceptanceAcrossEveryReRecordedGolden
	// asserts len == 5 and every entry carries hand-derived baseline
	// arithmetic for that re-recording — so adding a fixture there
	// would be a false statement about what 2.5a did.
	//
	// The relation is now: EVERY baselineAcceptanceFixtures entry must
	// appear here (unchanged, and it is the direction Finding 11 was
	// about — a golden dropping out of the missing-glyph corpus), and
	// every entry here that is NOT one of them must be DECLARED below.
	// A swap in either table still reddens, because an undeclared
	// addition fails just as a disappearance does.
	beyondBaselineAcceptance := map[string]string{
		"mandatory-break":    "Story 7.1 (FR46) — the first committed document whose text or bound data carries a line feed",
		"line-spacing":       "Story 7.2 — the first committed document that declares style.lineSpacing, and the first declaring format version 1.1",
		"justified-text":     "Story 7.3 (FR47) — the first committed document that is justified at all, the first whose drawn runs are word-grained, and the first declaring format version 2.0",
		"justified-thai":     "Story 7.3, owner scope amendment — the first committed document whose justified content carries no spaces, so its pieces are cut at dictionary seams (AD-25) rather than at whitespace",
		"alignment-rounding": "Story 7.3, closing DW-24 — the first committed document declaring align center or valign at all, and therefore the first that reaches any of the branches which halve a slack",
		"thai-stacked-marks": "Story 8.0 (DW-28) — the first committed document carrying a glyph the shaper gives a non-zero YOffset, and the only one whose runs are split into segments by a text rise",
		"embedded-font":      "Story 8.3 (FR53/FR56), rendering from the carried face since Story 8.4 (FR54) — the first committed document that CARRIES a font face rather than naming one, and the only one whose text no face the CALLER supplies covers a rune of",
		"declared-variants":  "Story 11.5 (DW-237) — the first committed document that declares bold or italic at all, and the only one that asks its chain for a CUT. It is the NEGATIVE CONTROL for the narrower-variant hazard, not an instance of it: coverage picks an entry on its BASE face and the declared variant is then applied WITHIN that entry, so a variant whose cmap is narrower than its base would warn here — and Roboto's four cuts all cover this document's Latin, so a Warning from this row means the resolution went somewhere it should not have",
		"keep-together":      "Story 7.7 (FR51) — the first committed document that declares a keep-together group, the first whose page break is placed by an author's own declaration rather than by the four pagination rules alone, and the first declaring format version 1.2",
	}

	wantNames := make(map[string]bool, len(baselineAcceptanceFixtures))
	for _, bf := range baselineAcceptanceFixtures {
		wantNames[bf.name] = true
	}
	gotNames := make(map[string]bool, len(fixtures))
	for _, f := range fixtures {
		gotNames[f.name] = true
	}
	for name := range wantNames {
		if !gotNames[name] {
			t.Errorf("presence precondition: baselineAcceptanceFixtures names %q, which this test's corpus table does not — every fixture that table names must be covered here", name)
		}
	}
	for name := range gotNames {
		if wantNames[name] {
			continue
		}
		if _, declared := beyondBaselineAcceptance[name]; !declared {
			t.Errorf("presence precondition: this test's corpus table names %q, which baselineAcceptanceFixtures does not and which beyondBaselineAcceptance does not declare — add a one-line entry naming the story that added it, or this table can grow without anyone reading the diff", name)
		}
	}
	for name, why := range beyondBaselineAcceptance {
		// The declaration is only worth having if it SAYS something —
		// key presence alone would let `{"foo": ""}` satisfy the
		// documented "name the story that added it" convention.
		if strings.TrimSpace(why) == "" {
			t.Errorf("beyondBaselineAcceptance declares %q with an empty reason — the entry must name the story that added the fixture, or it records nothing a reader can act on", name)
		}
		if !gotNames[name] {
			t.Errorf("beyondBaselineAcceptance declares %q, which this test's corpus table does not name — a declaration for a fixture that is not here covers nothing", name)
		}
		if wantNames[name] {
			t.Errorf("beyondBaselineAcceptance declares %q, which baselineAcceptanceFixtures ALSO names — the declaration is for entries BEYOND that table, and a stale one hides a real disappearance", name)
		}
	}
	if len(fixtures) != len(baselineAcceptanceFixtures)+len(beyondBaselineAcceptance) {
		t.Fatalf("presence precondition: this test's corpus table has %d entries, baselineAcceptanceFixtures has %d and beyondBaselineAcceptance declares %d — the three must account for each other exactly", len(fixtures), len(baselineAcceptanceFixtures), len(beyondBaselineAcceptance))
	}

	total := 0
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			res, err := Render(f.tpl(t), f.data, nil, f.fs(t))
			if err != nil {
				t.Fatalf("Render(%s): %v", f.name, err)
			}
			if len(res.Bytes) == 0 {
				t.Fatalf("Render(%s) produced no bytes", f.name)
			}
			count := 0
			for _, d := range res.Diagnostics {
				if d.Code == DiagCodeTextMissingGlyph {
					count++
				}
			}
			if count != 0 {
				t.Errorf("%s: rendering produced %d missing-glyph Warning(s), want 0 — a committed fixture must never exercise this path: %+v", f.name, count, res.Diagnostics)
			}
			total += count
		})
	}
	if total != 0 {
		t.Errorf("corpus total missing-glyph Warnings across %d fixture(s): %d, want 0", len(fixtures), total)
	}
}
