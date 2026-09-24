package folio8

import "github.com/panitw/folio8/folio-go/internal/expr"

// Version is the folio-go module version, matching the directory-prefixed
// release tag folio-go/v1.2.0 (AD-22). Each golden fixture's expected.json
// records the version that produced it, which may be older than this one.
// TestVersionAgreesWithReleasingDoc holds it equal to the release RELEASING.md
// names.
const Version = "1.2.0"

// LocaleTableVersion is AC6/AD-22, surfaced here (Finding 9, Story
// 3.4's QA review): internal/expr's locale table carries its own
// version, but being unexported and living under internal/, it could
// not previously reach anywhere the library version is surfaced —
// AD-22 makes it "part of the library version" and D-3.4.3's ruling
// rests on that link existing. Defined as exactly expr.LocaleTableVersion
// (never a second literal) so the two can never drift.
const LocaleTableVersion = expr.LocaleTableVersion
