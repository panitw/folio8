package template

import "testing"

// TestClosedLocalesMatchesLocaleTags is AC4's own-package half: the
// exported ordered LocaleTags and the unexported closedLocales lookup
// used by parse.go must name exactly the same set. Both are built from
// the same four constants (locale.go), so this is a construction
// check, not a coincidence — but it stays as an explicit assertion
// because "built from the same constants" is a property of the source
// text, not something the compiler enforces for two independently
// maintained declarations.
func TestClosedLocalesMatchesLocaleTags(t *testing.T) {
	if len(LocaleTags) == 0 {
		t.Fatal("presence precondition (D-000.9): LocaleTags is empty")
	}
	seen := map[string]bool{}
	for _, tag := range LocaleTags {
		seen[tag] = true
		if !closedLocales[tag] {
			t.Errorf("LocaleTags contains %q, but closedLocales does not", tag)
		}
	}
	for tag := range closedLocales {
		if !seen[tag] {
			t.Errorf("closedLocales contains %q, but LocaleTags does not", tag)
		}
	}
}

// TestLocaleTagsExactOrder is AC4a: the exported order is asserted as
// an exact literal sequence, not "some deterministic order" — pinning
// the sequence itself so a later switch to (e.g.) sorted order is a
// visible, deliberate edit to this test rather than a silent reorder.
func TestLocaleTagsExactOrder(t *testing.T) {
	want := []string{LocaleEN, LocaleTH, LocaleZhHans, LocaleJA}
	if len(LocaleTags) != len(want) {
		t.Fatalf("LocaleTags has %d entries, want %d: %v", len(LocaleTags), len(want), LocaleTags)
	}
	for i, tag := range want {
		if LocaleTags[i] != tag {
			t.Errorf("LocaleTags[%d] = %q, want %q (exact order pinned, AC4a)", i, LocaleTags[i], tag)
		}
	}
}

// TestIsLocaleMatchesLocaleTags ties the EXPORTED PREDICATE to the set
// it claims to read, on TestClosedLocalesMatchesLocaleTags' shape and
// with the same presence precondition. IsLocale is the one authority
// the loader (parse.go) and the command door
// (component_commands.go's setDocumentLocale) both ask, and a
// predicate nothing ties can drift from the set it claims to read —
// silently, because both callers would agree with each other while
// disagreeing with LocaleTags, which is what the refusal messages are
// derived from.
func TestIsLocaleMatchesLocaleTags(t *testing.T) {
	if len(LocaleTags) == 0 {
		t.Fatal("presence precondition (D-000.9): LocaleTags is empty")
	}
	for _, tag := range LocaleTags {
		if !IsLocale(tag) {
			t.Errorf("IsLocale(%q) = false, but %q is in LocaleTags", tag, tag)
		}
	}
	// The other direction, which the loop above cannot state: a
	// predicate that simply returned true would satisfy every row of
	// it. `fr` and `EN` are near-misses rather than nonsense — a
	// legitimate BCP-47 tag outside AD-12's set, and a case variant of
	// a member — because those are the two shapes a widened predicate
	// would let through first.
	for _, tag := range []string{"", "fr", "EN", "th-TH", "zh-Hant", "en ", " en"} {
		if IsLocale(tag) {
			t.Errorf("IsLocale(%q) = true, but %q is not in LocaleTags %v", tag, tag, LocaleTags)
		}
	}
}
