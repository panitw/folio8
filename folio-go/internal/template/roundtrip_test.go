package template

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// canonicalFixtures is the corpus of canonical `.folio` bytes this
// story's P1/P2/P3 tests run over (AC12–AC17). Every entry here is
// itself asserted, by mustBeCanonical, to already be a fixed point
// under the serializer before it is used to test anything else — a
// fixture that was never actually canonical would make every
// downstream assertion about it meaningless.
func canonicalFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	root := repoRootFromTest(t)
	golden := mustReadFile(t, filepath.Join(root, "folio-go", "testdata", "template", "golden", "worked-example.json"))

	minimal := []byte(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": []
    },
    "pageFooter": {
      "elements": [],
      "height": 40
    },
    "pageHeader": {
      "elements": [],
      "height": 80
    }
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
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

	return map[string][]byte{
		"worked-example": golden,
		"minimal":        minimal,
		"multi-page":     multiPageFixture,
		"unknown-keys":   unknownKeysFixture,
		"html-trap":      htmlEscapeTrapFixture,
		"utf8-trap":      utf8TrapFixture,
		"escape-trap":    minimalEscapeTrapFixture,
		"maximal":        maximalFixture,
		"null-field":     nullFieldFixture,
	}
}

func mustBeCanonical(t *testing.T, name string, b []byte) {
	t.Helper()
	d, err := ParseDocument(b)
	if err != nil {
		t.Fatalf("[%s] parse: %v", name, err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("[%s] serialize: %v", name, err)
	}
	if string(out) != string(b) {
		t.Fatalf("[%s] fixture is not itself canonical (P3 precondition failed):\nwant:\n%s\ngot:\n%s", name, b, out)
	}
}

// TestP3FixturesAreCanonical is P3's forward half over the whole corpus:
// b canonical => Serialize(Parse(b)) == b.
func TestP3FixturesAreCanonical(t *testing.T) {
	for name, b := range canonicalFixtures(t) {
		t.Run(name, func(t *testing.T) { mustBeCanonical(t, name, b) })
	}
}

// TestHandWrappedAssetNormalisesToCanonical is AC3's NON-VACUOUS half
// (D-1.8.2, D-1.8.9), RP-2's positive control: maximalFixture's asset
// `data` is hand-re-wrapped at 64 columns (a legal hand-edit, AD-9's
// Prevents line contemplates exactly this) instead of the canonical
// 76-column split it ships with — the parser must still ACCEPT it (AC2:
// "accept any wrapping"), and the serializer must produce EXACTLY the
// same canonical 76-column bytes as the untouched fixture (AC1/AC3a).
//
// This is the test D-1.8.9 names explicitly as the ONLY discriminating
// one: "canonical-in/canonical-out cannot fail against an echoing
// serializer; 64-in/76-out cannot pass against one." A serializer that
// merely echoed a.Data (the shipped baseline before this story, M-4)
// would reproduce the 64-column wrapping verbatim here and this test
// would fail — unlike TestP3FixturesAreCanonical, which passes against
// an echoing serializer whenever the input already happens to be
// 76-wrapped (exactly M-4's vacuity trap).
func TestHandWrappedAssetNormalisesToCanonical(t *testing.T) {
	canonical := maximalFixture

	d, err := ParseDocument(canonical)
	if err != nil {
		t.Fatalf("parse canonical maximalFixture: %v", err)
	}
	handWrapped := string(canonical)
	for key, asset := range d.Assets {
		joined := strings.Join(asset.Data, "")
		var wrapped64 []string
		for i := 0; i < len(joined); i += 64 {
			end := i + 64
			if end > len(joined) {
				end = len(joined)
			}
			wrapped64 = append(wrapped64, joined[i:end])
		}
		if len(wrapped64) < 2 {
			t.Fatalf("test fixture assumption violated: asset %s's payload is too short to produce more than one 64-column element", key)
		}
		canonicalArray := "[\n        \"" + strings.Join(asset.Data, "\",\n        \"") + "\"\n      ]"
		handArray := "[\n        \"" + strings.Join(wrapped64, "\",\n        \"") + "\"\n      ]"
		if !strings.Contains(handWrapped, canonicalArray) {
			t.Fatalf("could not locate asset %s's canonical data array in the fixture text to hand-rewrap it", key)
		}
		handWrapped = strings.Replace(handWrapped, canonicalArray, handArray, 1)
	}

	if handWrapped == string(canonical) {
		t.Fatal("test fixture assumption violated: the hand-wrapped variant is byte-identical to the canonical fixture")
	}

	d2, err := ParseDocument([]byte(handWrapped))
	if err != nil {
		t.Fatalf("parse hand-wrapped (64-column) variant: %v (AC2: any wrapping must be accepted)", err)
	}
	out, err := SerializeDocument(d2)
	if err != nil {
		t.Fatalf("serialize hand-wrapped variant: %v", err)
	}
	if string(out) != string(canonical) {
		t.Fatalf(
			"RP-2: a 64-column hand-wrapped asset must re-serialize to EXACTLY the canonical 76-column bytes "+
				"(D-1.8.9) — got:\n%s\n--- want ---\n%s", out, canonical,
		)
	}
}

// TestP1RoundTripThroughValue is AC12: for every Document d obtained by
// parsing a valid file, Parse(Serialize(d)) == d — equality on the
// parsed value, not on bytes.
func TestP1RoundTripThroughValue(t *testing.T) {
	for name, b := range canonicalFixtures(t) {
		t.Run(name, func(t *testing.T) {
			d, err := ParseDocument(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			out, err := SerializeDocument(d)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			d2, err := ParseDocument(out)
			if err != nil {
				t.Fatalf("re-parse: %v", err)
			}
			if !reflect.DeepEqual(d, d2) {
				t.Fatalf("Parse(Serialize(d)) != d:\nd:  %#v\nd2: %#v", d, d2)
			}
		})
	}
}

// TestP2NormalisationIsIdempotent is AC13: for every valid input b,
// Serialize(Parse(b)) == Serialize(Parse(Serialize(Parse(b)))).
func TestP2NormalisationIsIdempotent(t *testing.T) {
	inputs := canonicalFixtures(t)
	inputs["non-canonical-numbers"] = nonCanonicalNumberFixture

	for name, b := range inputs {
		t.Run(name, func(t *testing.T) {
			d, err := ParseDocument(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			out1, err := SerializeDocument(d)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			d2, err := ParseDocument(out1)
			if err != nil {
				t.Fatalf("re-parse: %v", err)
			}
			out2, err := SerializeDocument(d2)
			if err != nil {
				t.Fatalf("re-serialize: %v", err)
			}
			if string(out1) != string(out2) {
				t.Fatalf("normalisation is not idempotent:\nfirst:\n%s\nsecond:\n%s", out1, out2)
			}
		})
	}
}

// TestP3NonCanonicalNormalises is P3's "iff": a non-canonical but valid
// input does NOT round-trip byte-identically to itself, yet normalises
// to exactly the same bytes as the already-canonical form of the same
// document (AC27: legality is a property of the value, not the
// spelling).
func TestP3NonCanonicalNormalises(t *testing.T) {
	d, err := ParseDocument(nonCanonicalNumberFixture)
	if err != nil {
		t.Fatalf("parse non-canonical: %v", err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) == string(nonCanonicalNumberFixture) {
		t.Fatalf("non-canonical fixture unexpectedly round-tripped byte-identically to itself")
	}

	canonical := canonicalFixtures(t)["minimal"]
	if string(out) != string(canonical) {
		t.Fatalf("normalised non-canonical input does not match the canonical form of the same document:\nnormalised:\n%s\ncanonical:\n%s", out, canonical)
	}
}

// nonCanonicalNumberFixture is semantically identical to the "minimal"
// canonical fixture but spells its numbers non-canonically (AC27:
// "36.0000" -> "36" style normalisation, on margin top/right/bottom/left
// and the two band heights) and is deliberately squeezed onto few lines
// (also non-canonical whitespace) to keep this file smaller — whitespace
// outside string/number tokens carries no semantic weight, only the
// decoded value does.
var nonCanonicalNumberFixture = []byte(`{"assets":{},"bands":{"content":{"elements":[]},"pageFooter":{"elements":[],"height":4.0e1},"pageHeader":{"elements":[],"height":80.000}},"fonts":{},"locale":"en","nextId":1,"page":{"margin":{"bottom":36.0000,"left":3.6e1,"right":36.0,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"1.0"}`)
