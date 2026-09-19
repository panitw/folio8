// Package resolvermethodsetcompliant is the retained fixture tree for
// the resolver-method-set-closed rule (Story 3.3 finisher pass, Finding
// 2). It is never built as part of any real Go module: it lives under
// testdata/, which the go tool excludes from "./..." package matching,
// and it exists only to be pointed at by this rule's tests — including
// the negative control (asserting zero findings against an exactly-
// compliant Resolver shape).
package resolvermethodsetcompliant

// Value mirrors internal/expr.Value's role for this fixture: the
// rule's signature comparison only needs a same-package type to
// qualify against, never the real Value's fields.
type Value struct{}

// Resolver mirrors the real expr.Resolver exactly (AC1/AC22): three
// methods, none widened.
type Resolver interface {
	Resolve(path []string) (Value, error)
	CollectionLength(path []string) (int, error)
	ProjectCollection(path []string) ([]Value, error)
}
