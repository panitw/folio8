// Package resolvermethodsetembeddedinterface is a retained VIOLATING
// fixture for the resolver-method-set-closed rule (Story 3.3 finisher
// pass, Finding 2, evasion 2): a fourth method is contributed to
// Resolver through an EMBEDDED interface field rather than a named
// method — the shape the review measured leaves BOTH the AST-only
// name-list guard AND its own red-proof passing silently, because an
// *ast.Field for an embedded interface has zero Names to extract.
// go/types' *types.Interface method-set expansion has no such blind
// spot: an embedded interface's methods are part of the method set BY
// DEFINITION.
package resolvermethodsetembeddedinterface

// Value mirrors internal/expr.Value's role for this fixture.
type Value struct{}

// pageScopedResolver is the undeclared, page-scoped method AD-4
// forbids under any spelling — embedded rather than named on Resolver
// directly, which is exactly the evasion this fixture demonstrates.
type pageScopedResolver interface {
	PageIndex(path []string) (int, error)
}

// Resolver — a fourth method arrives via embedding, not a named
// method declaration.
type Resolver interface {
	Resolve(path []string) (Value, error)
	CollectionLength(path []string) (int, error)
	ProjectCollection(path []string) ([]Value, error)
	pageScopedResolver
}
