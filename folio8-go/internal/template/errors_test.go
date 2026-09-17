package template

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/panitw/folio8/folio8-go/internal/diag"
)

// TestLoadErrorsCarryFieldValueAndTheGeneralCode is AC41, RE-POINTED at
// Story 7.8.
//
// Its subject was "call sites of the UNCODED constructor": 1.4 minted no
// stable codes, so its load failures were plain Go errors naming field,
// element id and value, and this test enumerated a representative set of
// them. D-7.8.1 emptied that set — newLoadError now supplies
// diag.CodeTemplateFieldInvalid itself — so an unchanged test would have
// kept passing while measuring a population that no longer exists, which
// is the dead-detector failure recorded at D-7.4.2.
//
// The subject is therefore now "call sites of the GENERAL constructor",
// and the property is the one that actually matters at the boundary: a
// general load error carries field, value, element id where applicable,
// AND the general code — never "", which folio8.ParseTemplate would read
// as TEMPLATE_MALFORMED and the WASM host would then replace the whole
// message of. The code assertion is the half that reddens if anyone
// reverts the constructor, and it is the half that did not exist before.
// It reports how many it enumerated and fails on zero (D-000.9 shape).
func TestLoadErrorsCarryFieldValueAndTheGeneralCode(t *testing.T) {
	base := string(minimalDocWithElements([]string{"e1"}, 2))
	cases := []struct {
		name   string
		doc    []byte
		wantID bool // whether an element id is expected on the error
	}{
		{"bad-locale", []byte(strings.Replace(base, `"locale": "en",`, `"locale": "xx",`, 1)), false},
		{"bad-element-type", []byte(strings.Replace(base, `"type": "text",`, `"type": "chart",`, 1)), true},
		{"duplicate-id", minimalDocWithElements([]string{"e1", "e1"}, 5), true},
		{"bad-id-spelling", minimalDocWithElements([]string{"e01"}, 5), true},
		{"nextid-too-low", minimalDocWithElements([]string{"e1", "e2"}, 2), false},
		{"higher-major", withVersion("9.0"), false},
		// Story 7.8's own site, enumerated here rather than only in
		// closedsets_test.go: the refusal this story adds must be a
		// general-code load error like every other, not a special case.
		{"table-style-justify", alignDocWithTable(`"align": "justify"`, "", ""), true},
	}
	if len(cases) == 0 {
		t.Fatal("coverage witness: zero cases enumerated")
	}
	enumerated, coded := 0, 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDocument(c.doc)
			if err == nil {
				t.Fatalf("expected a load error")
			}
			enumerated++
			le, ok := err.(*LoadError)
			if !ok {
				// checkVersionLoadable returns a plain fmt.Errorf, not a
				// *LoadError, but it still names both versions in text
				// (asserted separately in version_test.go) — accept it
				// here as a valid "load error" shape while still
				// counting it toward the enumeration. It is also the one
				// load failure that legitimately still becomes
				// TEMPLATE_MALFORMED at the boundary: it names no field.
				if !strings.Contains(err.Error(), "version") {
					t.Fatalf("error is neither a *LoadError nor a version error: %v", err)
				}
				return
			}
			if le.Field == "" {
				t.Fatalf("LoadError.Field is empty: %+v", le)
			}
			if le.Value == "" {
				t.Fatalf("LoadError.Value is empty: %+v (AC41 names field, element id AND value)", le)
			}
			if c.wantID && le.ElementID == "" {
				t.Fatalf("expected ElementID to be populated: %+v", le)
			}
			// STORY 7.8, and the re-measurement this test exists for.
			// An empty Code here is not cosmetic: folio8.ParseTemplate
			// reads it as TEMPLATE_MALFORMED, and wasm/cmd/engine's
			// reportableMessage replaces that one code's message with
			// "The template could not be processed" — so everything
			// asserted above would be true of a value the author never
			// gets to read.
			if le.Code == "" {
				t.Fatalf("LoadError.Code is empty: %+v — an uncoded load error is destroyed at the WASM boundary before its field and element reach the author (D-7.8.1)", le)
			}
			if le.Code != diag.CodeTemplateFieldInvalid {
				t.Fatalf("LoadError.Code = %q, want the general code %q — a specific code is minted only when a named consumer must BRANCH on it (D-7.8.1); these sites have no such consumer", le.Code, diag.CodeTemplateFieldInvalid)
			}
			coded++
		})
	}
	if enumerated == 0 {
		t.Fatal("coverage witness: zero load errors actually enumerated")
	}
	if coded == 0 {
		t.Fatal("coverage witness: zero *LoadErrors reached the CODE assertion, so the re-pointed half of this test measured nothing (D-7.4.2)")
	}
	t.Logf("enumerated %d load failures; %d were *LoadErrors and all carried %s", enumerated, coded, diag.CodeTemplateFieldInvalid)
}

// TestTheOverridingConstructorStillWins is the other side of D-7.8.1's
// ruling, and it is what stops "the constructor supplies the code" from
// quietly meaning "there is only one code". The conditions a named
// consumer branches on keep theirs.
func TestTheOverridingConstructorStillWins(t *testing.T) {
	err := newLoadErrorCoded("columns[0].footerOf", "e1", "transactions.amount", "footerOf beside footer: count", diag.CodeTableFooterSourceForbidden)
	le, ok := err.(*LoadError)
	if !ok {
		t.Fatalf("newLoadErrorCoded returned %T", err)
	}
	if le.Code != diag.CodeTableFooterSourceForbidden {
		t.Fatalf("Code = %q, want %q — the override must not be overwritten by the general code", le.Code, diag.CodeTableFooterSourceForbidden)
	}
}

// TestLoadErrorMessageBoundsEveryAuthorSuppliedFragment is D-7.8.5.
//
// D-7.8.1 moved every general load error off TEMPLATE_MALFORMED so its
// field and element could reach the author. TEMPLATE_MALFORMED's message
// is destroyed at the WASM host for a reason — it may quote the
// offending document back — and the ruling's stated ground for the move
// was that a LoadError's message does not. It did: nine call sites in
// this package pass a whole JSON sub-object as Value, and three more
// fragments carry author content (the element id, the `assets.<k>`
// field path, and the reasons that quote an id or a collection path).
//
// This test pins the bound ON THE MESSAGE and the completeness OF THE
// STRUCT, which is the whole shape of the ruling: a Go integrator's CI
// log still gets the entire offending JSON off the error value; only the
// sentence rendered for a person is bounded.
//
// EVERY LEG USES A MULTI-BYTE (Thai) FRAGMENT ON PURPOSE. A bound that
// counted bytes would pass on ASCII and hand a Thai or CJK author a
// third of the budget — the script-dependence defect ruled on at Story
// 7.4 — so the assertions are in RUNES and redden under a byte count.
func TestLoadErrorMessageBoundsEveryAuthorSuppliedFragment(t *testing.T) {
	const thai = "ก"
	long := strings.Repeat(thai, 2048)

	t.Run("value-through-the-real-load-path", func(t *testing.T) {
		// Not a hand-built LoadError: this is the measured regression
		// itself — a WELL-FORMED document whose `style` key holds a
		// 2048-rune string, refused by decodeStyle's "must be an object"
		// arm, which passes string(raw) as Value.
		doc := strings.Replace(string(minimalDocWithElements([]string{"e1"}, 2)),
			`"type": "text",`, `"style": "`+long+`",`+"\n          "+`"type": "text",`, 1)
		err := parseAndRequireLoadError(t, []byte(doc))
		if !strings.HasPrefix(err.Field, "style") {
			t.Fatalf("Field = %q, want the style block", err.Field)
		}
		// The STRUCT keeps everything: 2048 runes plus the two JSON
		// quotes string(raw) includes.
		if got := utf8.RuneCountInString(err.Value); got != 2050 {
			t.Fatalf("LoadError.Value was truncated in the struct: %d runes, want 2050 — the datum must stay complete (D-7.8.5)", got)
		}
		assertBoundedFragment(t, err.Error(), long, loadErrorValueRunes)
	})

	t.Run("element-id-and-reason-through-the-real-load-path", func(t *testing.T) {
		// claimID passes the RAW id as ElementID, as Value, and — via
		// validateElementID's %q — inside the Reason too, so one
		// runaway id was reflected three times in one message. Measured
		// before this bound: 12,424 bytes for a 4096-character id.
		doc := minimalDocWithElements([]string{long}, 5)
		err := parseAndRequireLoadError(t, doc)
		msg := err.Error()
		if utf8.RuneCountInString(err.ElementID) != 2048 {
			t.Fatalf("LoadError.ElementID was truncated in the struct: %d runes", utf8.RuneCountInString(err.ElementID))
		}
		if n := strings.Count(msg, thai); n > loadErrorElementIDRunes+loadErrorReasonRunes+loadErrorValueRunes {
			t.Fatalf("the message reflected %d runes of the author's id across its three slots; the bounds allow at most %d", n, loadErrorElementIDRunes+loadErrorReasonRunes+loadErrorValueRunes)
		}
		if !strings.Contains(msg, "…") {
			t.Fatalf("no visible elision in %q — a truncated fragment that looks whole is a new lie", msg)
		}
		if !utf8.ValidString(msg) {
			t.Fatal("the message is not valid UTF-8: a bound split a rune")
		}
	})

	t.Run("field-path", func(t *testing.T) {
		// `assets.` + an author-supplied object key. Measured at 4,263
		// bytes for a 4096-character key before this bound.
		e := &LoadError{Field: "assets." + long, Value: "v", Reason: "must be an object"}
		assertBoundedFragment(t, e.Error(), long, loadErrorFieldRunes)
	})

	t.Run("reason", func(t *testing.T) {
		e := &LoadError{Field: "footerOf", ElementID: "e1", Value: "v", Reason: "must be prefixed by " + long}
		assertBoundedFragment(t, e.Error(), long, loadErrorReasonRunes)
	})

	t.Run("nothing-under-the-bound-is-touched", func(t *testing.T) {
		// The bound must be invisible for every message the engine
		// actually authors. The longest field path the format can
		// produce is `assets.<64-hex>.mediaType` (81 runes) and the
		// longest reason is the asset-digest mismatch (200 runes); both
		// are under their bounds by construction, and this pins that
		// the ordinary refusal is byte-identical to the unbounded form.
		e := &LoadError{Field: "style.align", ElementID: "e1", Value: "justify", Reason: closedSetMessage(TableStyleAlignTokens)}
		want := "template: field style.align (element e1): not one of the closed set left, center, right (value: justify)"
		if got := e.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
		if strings.Contains(e.Error(), "…") {
			t.Fatal("an elision marker appeared on a message that was never truncated")
		}
	})
}

// assertBoundedFragment checks that the author's run of fragment appears
// in msg at most bound times over, that the cut is visible, and that it
// is a RUNE cut. The last assertion is the one that reddens under a byte
// count: bound bytes of a 3-byte rune is bound/3 runes.
func assertBoundedFragment(t *testing.T, msg, fragment string, bound int) {
	t.Helper()
	unit := string([]rune(fragment)[0])
	got := strings.Count(msg, unit)
	if got > bound {
		t.Fatalf("the message reflected %d runes of the author's own document; the bound is %d (D-7.8.5)", got, bound)
	}
	if got <= bound/3 {
		t.Fatalf("only %d runes survived a bound of %d: the bound is counting BYTES, which gives a Thai or CJK author a third of the budget an English one gets (D-7.8.5, and the Story 7.4 ruling two floors down)", got, bound)
	}
	if !strings.Contains(msg, "…") {
		t.Fatalf("no visible elision in a truncated message: %q", msg)
	}
	if !utf8.ValidString(msg) {
		t.Fatal("the message is not valid UTF-8: the bound split a rune")
	}
}

func parseAndRequireLoadError(t *testing.T, doc []byte) *LoadError {
	t.Helper()
	_, err := ParseDocument(doc)
	if err == nil {
		t.Fatal("expected a load error")
	}
	le, ok := err.(*LoadError)
	if !ok {
		t.Fatalf("expected a *LoadError, got %T: %v", err, err)
	}
	return le
}

// TestLoadErrorMessageFitsTheHostWindow is the second half of D-7.8.5,
// and the half that was missing.
//
// The four per-fragment bounds above are each correct in isolation, but
// the criterion they were derived from is a property of the ASSEMBLED
// message — "the message must stay dominated by the engine's own words
// … inside the wasm host's bounded(message, 512) window". Four
// independent bounds share no budget: 84 + 24 + 96 + 256 = 460 runes of
// author content plus the sentence frame, and a rune of arbitrary JSON
// is up to utf8.UTFMax = 4 bytes.
//
// MEASURED: a document whose element id is 2048 Thai runes produced a
// 1139-byte message. The host's bounded() is a raw BYTE slice
// value[:512], so the author received that message cut mid-rune —
// invalid UTF-8, no elision marker — which is exactly the "can split a
// rune" and "invisible elision" that D-7.8.5 forbids. The host cut is
// the last resort; this bound is what stops it ever firing.
//
// EVERY LEG USES MULTI-BYTE (Thai) CONTENT. A byte-counting regression
// in the message bound must redden here rather than pass on ASCII.
func TestLoadErrorMessageFitsTheHostWindow(t *testing.T) {
	const thai = "ก"
	long := strings.Repeat(thai, 2048)

	assertFitsWindow := func(t *testing.T, msg string) {
		t.Helper()
		t.Logf("assembled message: bytes=%d runes=%d", len(msg), utf8.RuneCountInString(msg))
		if len(msg) > loadErrorMessageBytes {
			t.Fatalf("the assembled message is %d bytes; the host's window is %d, so bounded() would slice it raw at a byte offset (D-7.8.5)", len(msg), loadErrorMessageBytes)
		}
		if !utf8.ValidString(msg) {
			t.Fatal("the message is not valid UTF-8: a bound split a rune")
		}
		if !strings.HasSuffix(msg, loadErrorElision) {
			t.Fatalf("a message truncated to the window must END in a visible elision marker; got %q", msg)
		}
	}

	t.Run("element-id-through-the-real-load-path", func(t *testing.T) {
		// The multi-fragment vector: claimID passes the raw id as
		// ElementID, again as Value, and again inside Reason via
		// validateElementID's %q, so three per-fragment budgets are
		// spent at once on one runaway id.
		err := parseAndRequireLoadError(t, minimalDocWithElements([]string{long}, 5))
		assertFitsWindow(t, err.Error())
	})

	t.Run("footer-of-shaped-reason", func(t *testing.T) {
		// parse_bands.go's out-of-collection arm interpolates the
		// table's own collection path into the reason with %q while the
		// author's footerOf is the value, so both author fragments run
		// long together.
		e := &LoadError{
			Field:     "footerOf",
			ElementID: long,
			Value:     long,
			Reason:    `must be prefixed by the table's collection path "` + long + `" (D-1.4.2)`,
		}
		assertFitsWindow(t, e.Error())
	})

	t.Run("the-window-bound-never-fires-under-it", func(t *testing.T) {
		// Nothing the engine actually authors goes near 512 bytes, and
		// a Thai reason must not be touched either — this is the leg
		// that reddens if the message bound is ever tightened or made
		// byte-blind about what it is measuring.
		e := &LoadError{Field: "style.align", ElementID: "e1", Value: "จัดชิดขอบ", Reason: "ต้องเป็นหนึ่งในชุดปิด left, center, right"}
		want := "template: field style.align (element e1): ต้องเป็นหนึ่งในชุดปิด left, center, right (value: จัดชิดขอบ)"
		got := e.Error()
		if got != want {
			t.Fatalf("Error() = %q, want %q — the message bound touched a message that fits the window", got, want)
		}
		if len(got) >= loadErrorMessageBytes {
			t.Fatalf("witness broken: the leg's own message is %d bytes, not under the window", len(got))
		}
		if strings.Contains(got, loadErrorElision) {
			t.Fatal("an elision marker appeared on a message that was never truncated")
		}
	})
}
