package template

import (
	"strings"
	"testing"
)

// spec-font-sources-and-embedding CAP-2's LOADER and SERIALIZER halves.
//
// `embedFonts` is the format's first optional top-level BOOLEAN, and the one
// property that matters most about it is the one a boolean makes easy to get
// wrong: ABSENT MEANS TRUE. Go's zero value is the opposite, so every test
// below is written against a document that never mentions the key as well as
// against one that does.

// embedDoc is an otherwise-minimal, canonical document, with the `embedFonts`
// line spliced in at its SORTED position — between `bands` and `fonts` — so a
// fixture carrying the key is itself canonical and the round-trip assertions
// below are about the serializer rather than about this builder.
func embedDoc(declared string) []byte {
	line := ""
	if declared != "" {
		line = "  \"embedFonts\": " + declared + ",\n"
	}
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
` + line + `  "fonts": {},
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
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
}

// TestEmbedFontsAbsentMeansTheDocumentCarriesItsFaces is the default, and the
// property that keeps every file already on disk byte-unchanged: absent parses
// to true and serializes back to NO KEY.
func TestEmbedFontsAbsentMeansTheDocumentCarriesItsFaces(t *testing.T) {
	src := embedDoc("")
	d, err := ParseDocument(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !d.EmbedFonts {
		t.Error("an absent embedFonts parsed to false — absent must mean \"this document carries its faces\", which is what every document written before the key existed does")
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if strings.Contains(string(out), "embedFonts") {
		t.Errorf("a document that never declared embedFonts serialized WITH the key:\n%s", out)
	}
	if string(out) != string(src) {
		t.Errorf("round-trip is not a fixed point:\n--- got ---\n%s\n--- want ---\n%s", out, src)
	}
}

// TestEmbedFontsFalseRoundTripsByteForByte is the other polarity: the only
// value that is ever written is written, in sorted position, and survives.
func TestEmbedFontsFalseRoundTripsByteForByte(t *testing.T) {
	src := embedDoc("false")
	d, err := ParseDocument(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.EmbedFonts {
		t.Error("embedFonts: false parsed to true")
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) != string(src) {
		t.Errorf("round-trip is not a fixed point:\n--- got ---\n%s\n--- want ---\n%s", out, src)
	}
}

// TestEmbedFontsTrueCanonicalisesToTheAbsentKey is the normalisation rule, and
// it is what makes "turn it off, turn it back on" return the ORIGINAL bytes
// rather than bytes that merely mean the same thing. The format already
// normalises this way — `{"face": "X"}` with no variants writes back as `"X"` —
// so a hand-authored `true` is ADMITTED and canonicalised, never refused.
func TestEmbedFontsTrueCanonicalisesToTheAbsentKey(t *testing.T) {
	d, err := ParseDocument(embedDoc("true"))
	if err != nil {
		t.Fatalf("an authored embedFonts: true was REFUSED — it is legal and means the default: %v", err)
	}
	if !d.EmbedFonts {
		t.Fatal("embedFonts: true parsed to false")
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) != string(embedDoc("")) {
		t.Errorf("an authored embedFonts: true did not canonicalise to the absent key:\n--- got ---\n%s\n--- want ---\n%s", out, embedDoc(""))
	}
}

// TestEmbedFontsIsNeverCoerced is the loader's refusal row. The value is a
// boolean and only a boolean: a quoted one, a number, a container and `null`
// are located load errors naming the field, never coerced into an answer the
// author did not write.
func TestEmbedFontsIsNeverCoerced(t *testing.T) {
	for _, probe := range []struct{ declared, message string }{
		{`"true"`, EmbedFontsTypeMessage},
		{`"false"`, EmbedFontsTypeMessage},
		{`1`, EmbedFontsTypeMessage},
		{`0`, EmbedFontsTypeMessage},
		{`[]`, EmbedFontsTypeMessage},
		{`{}`, EmbedFontsTypeMessage},
		// `null` IS THE ROW WITH A REMEDY, and the one that would otherwise be
		// READ rather than refused: encoding/json admits the literal into a
		// bool without an error, leaving `false`. The sentence says to remove
		// the key, because for this field the ABSENT key is the default and
		// "correct the value" is not the same gesture. It is pinned so that
		// deliberate sentence cannot be collapsed into the generic one without
		// a test going red, and it is the SAME constant the command door
		// renders (component_commands.go's setDocumentEmbedFonts).
		{`null`, EmbedFontsNullMessage},
	} {
		t.Run(probe.declared, func(t *testing.T) {
			_, err := ParseDocument(embedDoc(probe.declared))
			if err == nil {
				t.Fatalf("embedFonts: %s was admitted", probe.declared)
			}
			if !strings.Contains(err.Error(), "embedFonts") {
				t.Errorf("the refusal does not name the field: %v", err)
			}
			if !strings.Contains(err.Error(), probe.message) {
				t.Errorf("refusal = %v, want it to carry %q", err, probe.message)
			}
		})
	}
}

// TestEmbedFontsRaisesNoVersion is D1 as a measurement. The key governs what a
// SAVE writes and nothing a render reads, so a document declaring it still
// declares the lowest version its own content requires — adding a ladder rank
// would make a document claim a version it does not need.
func TestEmbedFontsRaisesNoVersion(t *testing.T) {
	for _, declared := range []string{"", "true", "false"} {
		d, err := ParseDocument(embedDoc(declared))
		if err != nil {
			t.Fatalf("parse %q: %v", declared, err)
		}
		out, err := SerializeDocument(d)
		if err != nil {
			t.Fatalf("serialize %q: %v", declared, err)
		}
		if !strings.Contains(string(out), `"version": "1.0"`) {
			t.Errorf("a document declaring embedFonts %q no longer saves as 1.0:\n%s", declared, out)
		}
	}
}

// TestAnUnknownTopLevelBooleanIsCarriedThroughVerbatim is D1's PREMISE, and the
// only test that measures it.
//
// The whole no-version-move argument rests on unknown-key passthrough: a reader
// that has never heard of `embedFonts` must load the document, render the same
// page set and write the key back untouched. That is a claim about how this
// loader treats a key IT DOES NOT KNOW, and a test whose reader knows the key
// cannot exercise it — so this one uses a key nothing in the library
// recognises, with the same SHAPE the argument is made for: a top-level
// boolean. If this goes red, D1's ladder decision has lost its ground.
func TestAnUnknownTopLevelBooleanIsCarriedThroughVerbatim(t *testing.T) {
	// Spliced at the position the serializer sorts it to, so the fixture is
	// itself canonical and a moved byte is the serializer's doing.
	const unknown = "  \"embedsItsFacesSomeday\": false,\n"
	src := []byte(strings.Replace(string(embedDoc("")), "  \"fonts\": {},", unknown+"  \"fonts\": {},", 1))
	if !strings.Contains(string(src), "embedsItsFacesSomeday") {
		t.Fatal("fixture precondition: the unknown key was not spliced in")
	}

	d, err := ParseDocument(src)
	if err != nil {
		t.Fatalf("an unknown top-level boolean was REFUSED, which is the premise failing: %v", err)
	}
	// It is carried as OPAQUE passthrough — in Extra, not decoded into a field.
	carried := false
	for _, field := range d.Extra {
		if field.Key == "embedsItsFacesSomeday" {
			carried = true
		}
	}
	if !carried {
		t.Fatalf("the unknown key was dropped on load rather than carried: %#v", d.Extra)
	}

	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) != string(src) {
		t.Fatalf("an unknown top-level boolean was not written back verbatim:\n--- got ---\n%s\n--- want ---\n%s", out, src)
	}
}
