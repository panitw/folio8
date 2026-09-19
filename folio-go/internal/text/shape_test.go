package text

import "testing"

// TestClusterTextsPutsClusterTextOnTheFirstGlyphOnly is D-2.3-Q1(a)'s
// mechanism, tested as a pure function so it needs no font.
//
// The rejected alternative is the whole reason this is worth a test:
// mapping every glyph of a cluster to the cluster's text satisfies
// "every CID has an entry" perfectly, and makes a four-glyph Thai
// cluster extract its word four times over. Only the round-trip
// distinguishes correct from plausible.
func TestClusterTextsPutsClusterTextOnTheFirstGlyphOnly(t *testing.T) {
	cases := []struct {
		name   string
		source string
		// clusters, in drawing order, as the shaper reports them.
		clusters []int
		want     []string
	}{
		{
			name:     "one glyph per rune",
			source:   "abc",
			clusters: []int{0, 1, 2},
			want:     []string{"a", "b", "c"},
		},
		{
			name:     "ffi ligature: three runes collapse onto one glyph",
			source:   "office",
			clusters: []int{0, 1, 4, 5},
			want:     []string{"o", "ffi", "c", "e"},
		},
		{
			name:     "merged Thai cluster: four glyphs, one cluster",
			source:   "น้ำ",
			clusters: []int{0, 0, 0, 0},
			want:     []string{"น้ำ", "", "", ""},
		},
		{
			name:     "per-syllable Thai clusters",
			source:   "ณัฐวุฒิ",
			clusters: []int{0, 0, 2, 3, 3, 5, 5},
			want:     []string{"ณั", "", "ฐ", "วุ", "", "ฒิ", ""},
		},
		{
			name:     "leading vowel reordered before its consonant",
			source:   "เกิด",
			clusters: []int{0, 1, 1, 3},
			want:     []string{"เ", "กิ", "", "ด"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			glyphs := make([]ShapedGlyph, len(c.clusters))
			for i, cl := range c.clusters {
				glyphs[i] = ShapedGlyph{Cluster: cl}
			}
			got, err := ClusterTexts(c.source, glyphs)
			if err != nil {
				t.Fatalf("ClusterTexts: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %d texts, want %d", len(got), len(c.want))
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("glyph %d: got %q, want %q", i, got[i], c.want[i])
				}
			}

			// The property that actually matters: concatenating every
			// glyph's text reproduces the source EXACTLY — once, not
			// once per glyph of a merged cluster.
			var round string
			for _, s := range got {
				round += s
			}
			if round != c.source {
				t.Errorf("round-trip: got %q, want %q", round, c.source)
			}
		})
	}
}

// TestClusterTextsRejectsAnOutOfRangeCluster is the presence
// precondition (D-000.21 sharpened): a cluster value outside the
// source's runes is a located error, never a silent empty string that
// would make a round-trip assertion elsewhere pass by losing text.
func TestClusterTextsRejectsAnOutOfRangeCluster(t *testing.T) {
	if _, err := ClusterTexts("ab", []ShapedGlyph{{Cluster: 0}, {Cluster: 9}}); err == nil {
		t.Fatal("a cluster past the end of the source must be a located error")
	}
	if _, err := ClusterTexts("ab", []ShapedGlyph{{Cluster: -1}}); err == nil {
		t.Fatal("a negative cluster must be a located error")
	}
}

// TestShapeOfEmptyStringIsEmpty pins the one degenerate input the render
// path can hand this seam: an empty face-segment shapes to no glyphs and
// is not an error.
func TestShapeOfEmptyStringIsEmpty(t *testing.T) {
	s := &Shaper{name: "test"}
	got, err := s.Shape("")
	if err != nil {
		t.Fatalf("Shape(\"\"): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Shape(\"\") produced %d glyphs, want 0", len(got))
	}
}
