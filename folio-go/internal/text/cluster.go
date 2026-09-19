package text

import "unicode"

// runeClass is a mechanical classification of a single rune for Thai
// Character Cluster (TCC) boundary detection (AC7). It is derived from
// Unicode's own combining-class data (unicode.Mn) plus the Thai
// block's own structure — specifically, which code points are
// "leading vowels" that are written before the consonant they attach
// to (AD-25: "This is mechanical and needs no dictionary"). It does
// NOT depend on shaping (Story 2.3) or a font (Trap 4).
type runeClass int

const (
	classOther runeClass = iota
	classThaiConsonant
	classThaiLeadingVowel
	classThaiCombining      // Unicode Mn: vowel signs and tone marks rendered above/below a base
	classThaiFollowingVowel // spacing (non-combining) vowels that still attach to the preceding base
)

// Leading vowels (Thai block structure, not combining marks): a
// consonant's vowel sound is written BEFORE it — เ แ โ ใ ไ. A break
// between one of these and the consonant it belongs to would separate
// a single orthographic unit.
const (
	saraE          = 0x0E40 // เ
	saraAE         = 0x0E41 // แ
	saraO          = 0x0E42 // โ
	saraAiMaimuan  = 0x0E43 // ใ
	saraAiMaimalai = 0x0E44 // ไ
)

// Following (spacing) vowels: written after the consonant, not
// combining marks in the Unicode sense, but still part of the same
// orthographic cluster as the consonant they follow.
const (
	saraA       = 0x0E30 // ะ
	saraAA      = 0x0E32 // า
	saraAM      = 0x0E33 // ำ
	lakkhangyao = 0x0E45 // ๅ
)

func classify(r rune) runeClass {
	switch r {
	case saraE, saraAE, saraO, saraAiMaimuan, saraAiMaimalai:
		return classThaiLeadingVowel
	case saraA, saraAA, saraAM, lakkhangyao:
		return classThaiFollowingVowel
	}
	if r >= 0x0E01 && r <= 0x0E2E {
		return classThaiConsonant
	}
	// Thai combining marks (tone marks, above/below vowel signs,
	// thanthakhat, nikhahit, ...) are exactly Unicode's Mn category
	// within the Thai block — grounding the classification in the
	// Unicode combining-class data AD-25 names, rather than a
	// hand-picked code-point list.
	if r >= 0x0E00 && r <= 0x0E7F && unicode.Is(unicode.Mn, r) {
		return classThaiCombining
	}
	return classOther
}

// isThaiScript reports whether r belongs to the Thai Unicode block
// (U+0E01-U+0E5B — consonants, vowels, tone marks, Thai digits and
// punctuation). Used to split text into Thai vs. non-Thai script runs
// (break.go) — a coarser, purely script-membership question, distinct
// from classify's finer TCC classification.
func isThaiScript(r rune) bool {
	return r >= 0x0E01 && r <= 0x0E5B
}

// ClusterBoundaries returns, for each rune index i in [0, len(runes)],
// whether a Thai character cluster boundary exists there — i.e. a
// position mechanically admissible as a break candidate under AD-25's
// cluster absolute (AC7, P1). Index 0 and len(runes) are always
// boundaries. Positions inside a non-Thai run are also reported as
// boundaries here (this function's only binding property is that it
// NEVER reports a boundary strictly inside a Thai cluster); break.go
// applies its own, stricter, atomicity rule to non-Thai runs (P6d).
func ClusterBoundaries(runes []rune) []bool {
	n := len(runes)
	b := make([]bool, n+1)
	b[0] = true
	b[n] = true
	for i := 1; i < n; i++ {
		cur := classify(runes[i])
		prev := classify(runes[i-1])
		switch {
		case cur == classThaiCombining:
			// A combining mark always attaches to whatever precedes it.
			b[i] = false
		case cur == classThaiFollowingVowel && (prev == classThaiConsonant || prev == classThaiCombining):
			// A spacing vowel immediately after a consonant (or a
			// consonant already carrying a combining mark) is still
			// part of that consonant's cluster.
			b[i] = false
		case cur == classThaiConsonant && prev == classThaiLeadingVowel:
			// The consonant a leading vowel was written in front of.
			b[i] = false
		default:
			b[i] = true
		}
	}
	return b
}
