package template

import (
	"strings"
	"testing"
)

// docWith wraps a top-level key fragment in an otherwise-minimal, valid
// document, so each case below differs from the next in exactly the
// field under test.
func docWith(fragment string) []byte {
	return []byte(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": []
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },` + fragment + `
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
}

// TestUnbreakableValuesRoundTripsAsAnAbsentKey is the canonical
// fixed-point property Story 1.4 pins, applied to Story 2.4's new
// optional key: a document that never declared it must serialize
// WITHOUT it, not with an empty array.
//
// This is not a style point. Every existing golden template in
// fixtures/ omits the key; emitting "[]" for them would move their
// bytes, and AC12 permits exactly one recorded artifact to move in this
// story — a break table, not a template.
func TestUnbreakableValuesRoundTripsAsAnAbsentKey(t *testing.T) {
	src := docWith("")
	d, err := ParseDocument(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.UnbreakableValues != nil {
		t.Errorf("an absent unbreakableValues parsed to %v, want nil — absent must stay absent", d.UnbreakableValues)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if strings.Contains(string(out), "unbreakableValues") {
		t.Errorf("a document that never declared unbreakableValues serialized WITH the key:\n%s", out)
	}
	if string(out) != string(src) {
		t.Errorf("round-trip is not a fixed point:\n--- got ---\n%s\n--- want ---\n%s", out, src)
	}
}

// TestUnbreakableValuesRoundTripsWhenPresent is the other polarity: a
// declared list survives load/save byte-for-byte, in authored order.
func TestUnbreakableValuesRoundTripsWhenPresent(t *testing.T) {
	src := docWith(`
  "unbreakableValues": [
    "customer.name",
    "transactions.payee"
  ],`)
	d, err := ParseDocument(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"customer.name", "transactions.payee"}
	if len(d.UnbreakableValues) != len(want) {
		t.Fatalf("parsed %v, want %v", d.UnbreakableValues, want)
	}
	for i := range want {
		if d.UnbreakableValues[i] != want[i] {
			t.Errorf("entry %d = %q, want %q — authored order is preserved", i, d.UnbreakableValues[i], want[i])
		}
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) != string(src) {
		t.Errorf("round-trip is not a fixed point:\n--- got ---\n%s\n--- want ---\n%s", out, src)
	}
}

// TestUnbreakableValuesRejectsNonBarePaths is D-2.4.1's "one path
// convention in the format, not two", enforced at load.
//
// Each rejected spelling is a shape a reader might plausibly reach for,
// and each would otherwise match no substituted path at render time —
// producing a declaration that LOOKS honoured and silently is not. The
// failure direction matters: an unenforced declaration splits a
// customer's name with no diagnostic anywhere.
func TestUnbreakableValuesRejectsNonBarePaths(t *testing.T) {
	cases := []struct {
		name, path string
	}{
		{"binding braces", "{{customer.name}}"},
		{"function call", "sum(customer.name)"},
		{"collection subscript", "transactions[].payee"},
		{"interior whitespace", "customer. name"},
		{"empty string", ""},
		{"leading dot", ".customer"},
		{"trailing dot", "customer."},
		{"segment starting with a digit", "customer.1name"},
		{"hyphen", "customer-name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := docWith("\n  \"unbreakableValues\": [\n    \"" + tc.path + "\"\n  ],")
			_, err := ParseDocument(src)
			if err == nil {
				t.Fatalf("%q must be a load error — it is not a bare root-relative dotted path, and it would match no substituted value at render time", tc.path)
			}
			if !strings.Contains(err.Error(), "unbreakableValues") {
				t.Errorf("the load error must name the field it concerns (AD-14); got: %v", err)
			}
		})
	}

	// Presence precondition for the whole table: the ACCEPTED spelling
	// really does load, so the cases above are rejections of a
	// particular shape and not of the key itself.
	if _, err := ParseDocument(docWith("\n  \"unbreakableValues\": [\n    \"customer.name\"\n  ],")); err != nil {
		t.Fatalf("precondition: the accepted bare-path spelling must load, but it did not: %v", err)
	}
}

// TestUnbreakableValuesRejectsDuplicates: a duplicate is a defect at
// rest — never repaired, never silently collapsed (AD-10's discipline
// applied to a declaration list).
func TestUnbreakableValuesRejectsDuplicates(t *testing.T) {
	src := docWith("\n  \"unbreakableValues\": [\n    \"customer.name\",\n    \"customer.name\"\n  ],")
	_, err := ParseDocument(src)
	if err == nil {
		t.Fatal("a duplicated path must be a load error")
	}
	if !strings.Contains(err.Error(), "more than once") {
		t.Errorf("the error should say what is wrong; got: %v", err)
	}
}

// TestUnbreakableValuesRejectsWrongType pins the schema shape: a bare
// string is not a list, however convenient a single-entry shorthand
// would be. One spelling, not two.
func TestUnbreakableValuesRejectsWrongType(t *testing.T) {
	for _, frag := range []string{
		"\n  \"unbreakableValues\": \"customer.name\",",
		"\n  \"unbreakableValues\": {\"customer.name\": true},",
		"\n  \"unbreakableValues\": [1],",
	} {
		if _, err := ParseDocument(docWith(frag)); err == nil {
			t.Errorf("must be a load error: %s", frag)
		}
	}
}
