// Package untypecheckable is a RETAINED fixture for Story 2.3a Finding 4:
// a tree that PARSES cleanly but does not TYPE-CHECK.
//
// It exists to exercise ScanFloatTypedValues' per-package `Errors` sweep
// — the branch D-1.3.11 actually names — as distinct from the
// packages.Load top-level error branch, which the ruling says is "not
// sufficient" on its own. Pointing the scanner at a non-existent
// directory only ever reaches the latter, so before this fixture the
// former was exercised by nothing.
//
// The distinction matters because a type-aware checker that cannot type
// a tree has no findings to report, and "no findings" is exactly what a
// clean tree looks like. Silence must therefore be impossible: the
// scanner has to fail loudly instead.
//
// This fixture lives OUTSIDE testdata/lint/no-float-typed-value/, whose
// two tests assert an exact finding set and would be disturbed by an
// untypeable package.
//
// It is under testdata/, which the go tool excludes from package
// listing, so it never reaches folio-go's own build, vet or test runs.
package untypecheckable

// ParsesButDoesNotTypeCheck is syntactically valid Go — go/parser
// accepts it, which the test asserts as a precondition — but
// thisSymbolDoesNotExist is declared nowhere, so go/types cannot assign
// this expression a type.
//
// Note the shape deliberately mimics a real float-typed site
// (int64(someAccessor)), so a scanner that silently degraded to
// reporting zero findings here would be reporting zero on a file whose
// whole point is to look like a violation.
func ParsesButDoesNotTypeCheck() int64 {
	return int64(thisSymbolDoesNotExist)
}
