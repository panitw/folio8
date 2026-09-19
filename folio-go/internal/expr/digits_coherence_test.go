package expr

import "testing"

// TestDigitsFieldAppliesToBothDateAndNumber is Finding 5's fix and its
// forcing function, both at once. DECISION-2 (AC11a, the owner's
// ruling, D-3.4.3) is that a locale's digit set is an explicit, named
// table field precisely so a later table edit is a visible, versioned
// change rather than something that silently does nothing — and the
// engineering lead's ruling on this story requires the COHERENCE
// itself to be asserted: for a given locale, every digit character
// formatDate produces and every digit character formatNumber produces
// come from the SAME numeral system that locale's `digits` field
// names. Without this test, the "incoherent middle" the owner was
// shown and rejected (a Buddhist-era year in Thai numerals sitting
// above a column of Western-digit amounts, or the reverse) is one
// table edit away with nothing objecting.
//
// WHY A SYNTHETIC LOCALE, NOT ONE OF THE FOUR SHIPPED ROWS (D-000.50):
// every shipped locale uses digitsWestern today, so "assert both
// outputs match the field" passes whether or not formatDate actually
// reads entry.digits — the hardcoded-ASCII implementation and the
// correct implementation are indistinguishable on every real subject.
// This constructs a localeEntry with Thai digits directly (never added
// to the shipped localeTable — no such locale ships) and calls
// renderDatePattern/renderNumberPattern with it, the only way this
// property can be told apart from "every real row happens to already
// agree" (the same move D-2.6.5 required: the red-proof needs a
// synthetic subject, not a fixture).
//
// Measured red-proof: with renderDatePattern's translateDigits call
// removed, this test reddens ON THE DATE half only — "got \"15 August
// 2026\", want \"๑๕ August ๒๐๒๖\"" — while the number half still
// passes, which is exactly the asymmetric defect Finding 5 named
// rather than a generic mismatch.
func TestDigitsFieldAppliesToBothDateAndNumber(t *testing.T) {
	synthetic := localeEntry{
		groupSep: ",", decimalSep: ".", groupSize: 3,
		digits: "๐๑๒๓๔๕๖๗๘๙", // Thai numerals — never a shipped locale's digit set
		monthNames: [12]string{
			"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December",
		},
	}

	tokens, err := parseDatePattern("d MMMM yyyy")
	if err != nil {
		t.Fatalf("parseDatePattern: %v", err)
	}
	civil := civilInstant{Year: 2026, Month: 8, Day: 15}
	gotDate := renderDatePattern(tokens, civil, synthetic)
	wantDate := "๑๕ August ๒๐๒๖"
	if gotDate != wantDate {
		t.Errorf("formatDate with a Thai-digit synthetic locale: got %q, want %q — the digits field must move BOTH functions' output (AC11a)", gotDate, wantDate)
	}

	spec, err := validateNumberPattern("#,##0.00")
	if err != nil {
		t.Fatalf("validateNumberPattern: %v", err)
	}
	scaled, err := scaleToFractionDigits(Decimal{Coefficient: 123456, Exponent: -2}, spec.fracDigits)
	if err != nil {
		t.Fatalf("scaleToFractionDigits: %v", err)
	}
	gotNumber := renderNumberPattern(scaled, spec, synthetic)
	wantNumber := "๑,๒๓๔.๕๖"
	if gotNumber != wantNumber {
		t.Fatalf("formatNumber with a Thai-digit synthetic locale: got %q, want %q", gotNumber, wantNumber)
	}
}
