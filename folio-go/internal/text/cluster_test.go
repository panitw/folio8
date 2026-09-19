package text

import "testing"

// TestClusterBoundariesNeverInsideCluster is AC7's mechanical property,
// exercised against known Thai clusters — a bare consonant would pass
// vacuously (V5), so every case here contains an actual vowel/tone mark
// combination.
func TestClusterBoundariesNeverInsideCluster(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		noBreak []int // rune indices that must NOT be reported as boundaries
	}{
		{
			name:    "leading vowel + consonant (เก)",
			text:    "เก",
			noBreak: []int{1},
		},
		{
			name:    "consonant + above vowel + tone mark (กิ่ว-shaped: ก ิ ่)",
			text:    "กิ่ว",
			noBreak: []int{1, 2},
		},
		{
			name:    "stacked vowel+tone on one base (สั่ง: ส ั ่ ง)",
			text:    "สั่ง",
			noBreak: []int{1, 2},
		},
		{
			name:    "consonant + spacing following vowel (มา)",
			text:    "มา",
			noBreak: []int{1},
		},
		{
			name:    "leading vowel + consonant + below vowel (ถูก-shaped: consonant+ู)",
			text:    "ถูก",
			noBreak: []int{1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runes := []rune(tc.text)
			b := ClusterBoundaries(runes)
			for _, i := range tc.noBreak {
				if b[i] {
					t.Errorf("%q: expected NO cluster boundary at rune index %d, got true (runes=%q)", tc.text, i, runes)
				}
			}
		})
	}
}

// TestClusterBoundariesEndpoints guards against the trivial vacuous
// case (V5): index 0 and len(runes) are always reported as boundaries.
func TestClusterBoundariesEndpoints(t *testing.T) {
	runes := []rune("สวัสดี")
	b := ClusterBoundaries(runes)
	if !b[0] {
		t.Error("expected boundary at index 0")
	}
	if !b[len(runes)] {
		t.Error("expected boundary at len(runes)")
	}
}

// TestClusterBoundariesSeparatesDistinctSyllables confirms the
// function is not simply "always false inside any Thai text" (which
// would also satisfy the no-break assertions above vacuously): distinct
// consonant-initial syllables with no attaching mark between them must
// report a boundary.
func TestClusterBoundariesSeparatesDistinctSyllables(t *testing.T) {
	// "กขค" - three bare consonants with nothing attaching between them.
	runes := []rune("กขค")
	b := ClusterBoundaries(runes)
	if !b[1] || !b[2] {
		t.Fatalf("expected boundaries between three unrelated bare consonants, got %v", b)
	}
}
