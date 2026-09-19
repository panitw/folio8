// Package unrankedstage is the retained violating fixture for the
// FAIL-SAFE DEFAULT: a package directory under internal/ that carries no
// rank is a FINDING, never a pass. A new stage must be ranked
// deliberately, the same closed-allow-list shape mathAllowedCalls uses.
//
// Note that it imports nothing at all: the finding is about the
// package's own absence from the table, so it must fire before any
// import is even considered.
package unrankedstage

func Nothing() {}
