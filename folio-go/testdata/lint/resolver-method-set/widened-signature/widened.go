// Package resolvermethodsetwidenedsignature is a retained VIOLATING
// fixture for the resolver-method-set-closed rule (Story 3.3 finisher
// pass, Finding 2, evasion 1): ProjectCollection gains an offset/limit
// parameter under its OWN, unchanged name — exactly the shape AC5
// forbids ("neither method takes a range, offset, index or limit") and
// the shape the review measured leaves the AST-only name-list guard
// (TestExprResolverMethodSetIsClosed) passing, because a name list
// cannot see a signature change.
package resolvermethodsetwidenedsignature

// Value mirrors internal/expr.Value's role for this fixture.
type Value struct{}

// Resolver — ProjectCollection widened with offset/limit.
type Resolver interface {
	Resolve(path []string) (Value, error)
	CollectionLength(path []string) (int, error)
	ProjectCollection(path []string, offset, limit int) ([]Value, error)
}
