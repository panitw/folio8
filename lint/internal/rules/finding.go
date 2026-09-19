// Package rules holds the pure, testable AD-1 and map-iteration checkers
// for Story 1.3 (D-1.3.6): the AD-1 import/selector lint and the
// map-range check. Each checker is a pure function over a target
// directory returning (findings, error) — no *testing.T parameter, no
// hard-coded root, no repo-root discovery inside it (AC1) — so the same
// function can be pointed at the real folio-go/internal/ tree (asserting
// zero) and at a retained fixture tree (asserting exactly the named
// findings, by file and rule).
package rules

// Finding is one violation. Path is relative to the directory the
// checker was pointed at (AC4: findings carry a defined path base — the
// scanned target directory — so the same function produces paths on the
// same base whether pointed at the real tree or a fixture tree). Rule is
// a stable rule id used by fixture-scan assertions, which compare by
// file and rule, never by count (AC1, RP-3c).
type Finding struct {
	Path    string
	Rule    string
	Line    int
	Message string
}
