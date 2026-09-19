package expr

import (
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// localeTableCoverageMismatches is TestLocaleTableCoversExactlyClosedSet's
// own comparison, factored into a pure function (Finding 12, this
// story's QA review) so its two named red-proofs below can call this
// SAME implementation against a locally-built COPY of the table,
// rather than re-implementing the comparison inline and observing
// their own copy agree with itself — which is a property of Go maps,
// never of this guard, and was measured to leave both red-proofs
// PASSING even after removing `th`'s entry from the REAL localeTable.
// Neither direction mutates the real, package-level localeTable — Finding
// 13 flags exactly that pattern as an AD-1 risk elsewhere in this
// package, and there is no need to take it on here: passing a COPY to
// the real function exercises the real guard just as directly.
func localeTableCoverageMismatches(tags []string, table map[string]localeEntry) (missingEntries, extraEntries []string) {
	declared := map[string]bool{}
	for _, tag := range tags {
		declared[tag] = true
		if _, ok := table[tag]; !ok {
			missingEntries = append(missingEntries, tag)
		}
	}
	for tag := range table {
		if !declared[tag] {
			extraEntries = append(extraEntries, tag)
		}
	}
	return missingEntries, extraEntries
}

// TestLocaleTableCoversExactlyClosedSet is AC4/AC4b: the set difference
// is asserted BOTH WAYS — a declared tag with no table entry, and a
// table entry keyed by a tag absent from the closed set, are different
// bugs and are named separately (never a count, which is a lossy set).
func TestLocaleTableCoversExactlyClosedSet(t *testing.T) {
	if len(template.LocaleTags) == 0 {
		t.Fatal("presence precondition (D-000.9): template.LocaleTags is empty")
	}
	if len(localeTable) == 0 {
		t.Fatal("presence precondition (D-000.9): localeTable is empty")
	}
	missing, extra := localeTableCoverageMismatches(template.LocaleTags, localeTable)
	for _, tag := range missing {
		t.Errorf("declared tag %q has no localeTable entry — a tag nobody can format", tag)
	}
	for _, tag := range extra {
		t.Errorf("localeTable has an entry for %q, which is not one of the declared closed-set tags — formatting data for a tag the loader rejects", tag)
	}
}

// TestLocaleTableRedProofMissingEntry is AC4b's red-proof, first
// direction: a declared tag with no table entry, checked by the REAL
// localeTableCoverageMismatches against a COPY of the real localeTable
// with th's entry removed — measured to redden in exactly the first
// direction, naming th, with the second direction (extraEntries)
// staying empty.
func TestLocaleTableRedProofMissingEntry(t *testing.T) {
	mutated := map[string]localeEntry{}
	for k, v := range localeTable {
		mutated[k] = v
	}
	delete(mutated, template.LocaleTH)

	missing, extra := localeTableCoverageMismatches(template.LocaleTags, mutated)
	if len(missing) != 1 || missing[0] != template.LocaleTH {
		t.Fatalf("RED-PROOF FAILED: expected missingEntries=[%s], got %v", template.LocaleTH, missing)
	}
	if len(extra) != 0 {
		t.Fatalf("removing th's entry must not ALSO report an extra entry (the two directions are independent bugs), got %v", extra)
	}
}

// TestLocaleTableRedProofExtraEntry is AC4b's red-proof, second
// direction: a table entry keyed by an undeclared tag, checked by the
// REAL localeTableCoverageMismatches against a COPY of the real
// localeTable with a bogus entry added — measured to redden in exactly
// the second direction, naming the bogus tag, with the first direction
// (missingEntries) staying empty.
func TestLocaleTableRedProofExtraEntry(t *testing.T) {
	mutated := map[string]localeEntry{}
	for k, v := range localeTable {
		mutated[k] = v
	}
	mutated["xx-Bogus"] = localeEntry{}

	missing, extra := localeTableCoverageMismatches(template.LocaleTags, mutated)
	if len(missing) != 0 {
		t.Fatalf("adding a bogus entry must not ALSO report a missing entry (the two directions are independent bugs), got %v", missing)
	}
	if len(extra) != 1 || extra[0] != "xx-Bogus" {
		t.Fatalf("RED-PROOF FAILED: expected extraEntries=[xx-Bogus], got %v", extra)
	}
}

// TestJaAndZhHansShareMonthNamesByConstruction is AC4b/D-3.4.1: the two
// rows are identical for everything this story implements, and they
// share their data BY CONSTRUCTION (one declaration, referenced
// twice), not by two literals that happen to agree today.
func TestJaAndZhHansShareMonthNamesByConstruction(t *testing.T) {
	ja := localeTable[template.LocaleJA]
	zh := localeTable[template.LocaleZhHans]
	if ja.monthNames != zh.monthNames {
		t.Fatalf("ja/zh-Hans month names diverge: ja=%v zh-Hans=%v", ja.monthNames, zh.monthNames)
	}
	if ja.digits != zh.digits || ja.groupSep != zh.groupSep || ja.decimalSep != zh.decimalSep || ja.groupSize != zh.groupSize || ja.eraOffset != zh.eraOffset {
		t.Fatalf("ja/zh-Hans formatting data diverges beyond month names: ja=%+v zh-Hans=%+v", ja, zh)
	}
}

// TestLocaleTableVersionIsDeclared is AC6/AD-22: the table carries a
// version, surfaced as a package-level constant. No mechanism beyond
// declaration is claimed (D-000.24) — this test only pins that the
// label exists and is a positive integer, since AD-22 makes bumping it
// on a table edit a human obligation this constant makes visible, not
// one this test can enforce by itself.
func TestLocaleTableVersionIsDeclared(t *testing.T) {
	if LocaleTableVersion < 1 {
		t.Fatalf("LocaleTableVersion = %d, want >= 1", LocaleTableVersion)
	}
}

// TestUnlistedLocaleIsLocatedErrorNeverDefault is AC5b: an unknown
// (or zero-value) locale tag is a located error, never a default
// entry.
func TestUnlistedLocaleIsLocatedErrorNeverDefault(t *testing.T) {
	if _, err := lookupLocale("xx"); err == nil {
		t.Fatal("expected a located error for an unlisted locale tag")
	}
	if _, err := lookupLocale(""); err == nil {
		t.Fatal("expected a located error for the zero-value locale tag — FormatContext{}'s zero value must be unusable")
	}
}

// TestUnlistedLocaleRedProof is AC5b's own red-proof: a lookup that
// silently falls back to `en` must redden this test while
// TestUnlistedLocaleIsLocatedErrorNeverDefault above still passes for
// the LOAD layer's own arm (a) — parse.go's closed-set rejection,
// asserted separately in internal/template. This test only exercises
// arm (b), the formatter layer.
func TestUnlistedLocaleRedProof(t *testing.T) {
	fallbackToEnglish := func(tag string) (localeEntry, error) {
		if e, ok := localeTable[tag]; ok {
			return e, nil
		}
		return localeTable[template.LocaleEN], nil // the hazard AC5b forbids
	}
	e, err := fallbackToEnglish("xx")
	if err != nil {
		t.Fatal("the deliberately-wrong fallback should not itself error")
	}
	if e != localeTable[template.LocaleEN] {
		t.Fatal("the fallback mutation did not even reproduce the hazard it is meant to demonstrate")
	}
	// The REAL lookupLocale must NOT behave this way:
	if _, err := lookupLocale("xx"); err == nil {
		t.Fatal("RED-PROOF FAILED: lookupLocale must reject an unlisted tag, unlike the fallback mutation above")
	}
}
