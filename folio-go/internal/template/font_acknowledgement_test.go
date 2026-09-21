package template

import (
	"strings"
	"testing"
)

// THE LOAD DOOR AND THE VERSION LADDER UNDER
// spec-font-sources-and-embedding CAP-6. The command-door rows of the story's
// I/O matrix are asserted in folio-go/font_acknowledgement_test.go; these are
// the rows the loader and versionRequiredByContent own.

// acknowledgedFontAssetBody is fontAssetBody with the three terms fields
// stripped and the acknowledgement in their place — the record an
// author-supplied binary that says nothing actually produces.
const acknowledgedFontAssetBody = `
      "data": ` + embeddedFontData + `,
      "font": {
        "authorAcknowledged": true,
        "family": "Brand Grotesk",
        "source": "imported from the author's own machine, acknowledged 2026-09-21",
        "style": "Regular"
      },
      "mediaType": "font/ttf"`

// blankTermsFontAssetBody is the SAME record with the three terms fields
// present and empty rather than absent, because "absent" and "present and
// empty" are different states and requireEmbeddedFaceLicence refuses both.
const blankTermsFontAssetBody = `
      "data": ` + embeddedFontData + `,
      "font": {
        "authorAcknowledged": true,
        "copyright": "",
        "family": "Brand Grotesk",
        "licence": "",
        "licenceText": "   ",
        "source": "imported from the author's own machine, acknowledged 2026-09-21",
        "style": "Regular"
      },
      "mediaType": "font/ttf"`

// acknowledgedSecondFontAssetBody is the SECOND fixture face — the one a
// style-variant sibling can name — carrying the acknowledgement and no terms.
const acknowledgedSecondFontAssetBody = `
      "data": ` + secondFontData + `,
      "font": {
        "authorAcknowledged": true,
        "family": "Brand Grotesk",
        "source": "imported from the author's own machine, acknowledged 2026-09-21",
        "style": "Bold"
      },
      "mediaType": "font/ttf"`

// MATRIX ROW 1, AT THE LOAD DOOR. A chain names an acknowledged asset whose
// terms are absent, and the document loads.
func TestAnAcknowledgedFaceLoadsWithNoTermsAtAll(t *testing.T) {
	for _, body := range []struct{ label, body string }{
		{"terms absent", acknowledgedFontAssetBody},
		{"terms present and blank", blankTermsFontAssetBody},
	} {
		t.Run(body.label, func(t *testing.T) {
			d, err := ParseDocument([]byte(embeddedFontDoc(body.body, embeddedChain)))
			if err != nil {
				t.Fatalf("an acknowledged face was refused at load: %v", err)
			}
			if !d.Assets[embeddedFontKey].FaceAcknowledged() {
				t.Fatal("the acknowledgement did not survive the load, so no door downstream can honour it")
			}
		})
	}
}

// MATRIX ROW 9 — THE OLDER READER, asserted the only way this library can
// assert it: the SAME document without the flag is refused, by the very rule a
// 4.x reader still applies to the key it carries through. That refusal is why
// this is a MAJOR rather than an additive key.
func TestTheSameDocumentWithoutTheAcknowledgementIsRefused(t *testing.T) {
	stripped := strings.Replace(acknowledgedFontAssetBody, `"authorAcknowledged": true,`, ``, 1)
	err := requireLoadError(t, embeddedFontDoc(stripped, embeddedChain), "assets."+embeddedFontKey+".font.licence")
	if !strings.Contains(err.Reason, "a font that travels without its terms") {
		t.Fatalf("the refusal's wording moved: %s", err.Reason)
	}
}

// AND `false` AND `null` ACKNOWLEDGE NOTHING. Three-valued, like every other
// key on this record: a guard written only for the absent case would let
// `"authorAcknowledged": null` excuse a document that asserts nothing.
func TestOnlyATrueAcknowledgementExcusesAnything(t *testing.T) {
	for _, spelling := range []string{`"authorAcknowledged": false,`, `"authorAcknowledged": null,`} {
		t.Run(spelling, func(t *testing.T) {
			body := strings.Replace(acknowledgedFontAssetBody, `"authorAcknowledged": true,`, spelling, 1)
			requireLoadError(t, embeddedFontDoc(body, embeddedChain), "assets."+embeddedFontKey+".font.licence")
		})
	}
}

// THE KEY IS A BOOLEAN, and a string spelling of one is refused where it is
// written rather than carried through as an unknown key.
func TestTheAcknowledgementMustBeABoolean(t *testing.T) {
	body := strings.Replace(acknowledgedFontAssetBody, `"authorAcknowledged": true,`, `"authorAcknowledged": "true",`, 1)
	err := requireLoadError(t, embeddedFontDoc(body, embeddedChain), "assets."+embeddedFontKey+".font.authorAcknowledged")
	if !strings.Contains(err.Reason, "must be a boolean") {
		t.Fatalf("reason = %q", err.Reason)
	}
}

// MATRIX ROW 5 — THE VARIANT ARM. A style-variant sibling names an asset in
// its own right and clears the same bar through the same function, so the
// acknowledgement must reach it too. A document whose regular is a catalogue
// face and whose bold is the author's own is the shape this row exists for.
func TestAnAcknowledgedVariantCutLoads(t *testing.T) {
	doc := twoFontAssetDoc(fontAssetBody, acknowledgedSecondFontAssetBody,
		`[{"asset": "`+embeddedFontKey+`", "bold": "`+secondFontKey+`"}]`)
	d, err := ParseDocument([]byte(doc))
	if err != nil {
		t.Fatalf("an acknowledged variant cut was refused at load: %v", err)
	}
	if !d.Assets[secondFontKey].FaceAcknowledged() {
		t.Fatal("the cut's acknowledgement did not survive the load")
	}

	// AND THE SAME DOCUMENT WITHOUT IT IS REFUSED AT THE VARIANT'S OWN FIELD,
	// which is what says the arm is really the one being exercised.
	stripped := strings.Replace(acknowledgedSecondFontAssetBody, `"authorAcknowledged": true,`, ``, 1)
	requireLoadError(t, twoFontAssetDoc(fontAssetBody, stripped,
		`[{"asset": "`+embeddedFontKey+`", "bold": "`+secondFontKey+`"}]`),
		"assets."+secondFontKey+".font.licence")
}

// MATRIX ROW 7 — THE VERSION TRIGGER. A document whose chain names an
// acknowledged record declares 4.2, and the trigger is the FLAG, not the
// symptom: this fixture's terms are perfectly good and it still declares 4.2,
// because its correctness depends on the flag being honoured.
func TestAnAcknowledgedRecordAChainNamesDeclaresFourTwo(t *testing.T) {
	withTerms := strings.Replace(fontAssetBody, `"copyright":`, `"authorAcknowledged": true,
        "copyright":`, 1)
	d, err := ParseDocument([]byte(embeddedFontDoc(withTerms, embeddedChain)))
	if err != nil {
		t.Fatal(err)
	}
	if got := versionRequiredByContent(d); got != "4.2" {
		t.Fatalf("versionRequiredByContent = %q, want 4.2", got)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"version": "4.2"`) {
		t.Fatalf("the saved document does not declare 4.2:\n%s", out)
	}
}

// AND THE VARIANT REACHES IT TOO. A chain whose base is an ordinary catalogue
// face and whose BOLD is the author's own acknowledged cut still requires 4.2
// — the load door will be asked about that sibling, so a probe that looked
// only at the base would stamp a version that lies.
func TestAnAcknowledgedVariantCutAlsoDeclaresFourTwo(t *testing.T) {
	d, err := ParseDocument([]byte(twoFontAssetDoc(fontAssetBody, acknowledgedSecondFontAssetBody,
		`[{"asset": "`+embeddedFontKey+`", "bold": "`+secondFontKey+`"}]`)))
	if err != nil {
		t.Fatal(err)
	}
	if got := versionRequiredByContent(d); got != "4.2" {
		t.Fatalf("versionRequiredByContent = %q, want 4.2 — the probe does not reach a style-variant sibling", got)
	}
}

// MATRIX ROW 8 — NO TRIGGER, AND NO MOVED BYTES. Two halves, and they are
// different claims: a document with no acknowledged record keeps its version,
// and a document whose acknowledged asset NO CHAIN NAMES keeps it too, on the
// same "the trigger is the entry, not the asset" rule every other font probe
// follows.
func TestADocumentWithNoAcknowledgedRecordIsUnmoved(t *testing.T) {
	plain := embeddedFontDoc(fontAssetBody, embeddedChain)
	before := canonicalFixedPoint(t, plain)
	if strings.Contains(before, "authorAcknowledged") {
		t.Fatal("a document that carries no acknowledgement wrote the key anyway")
	}
	d, err := ParseDocument([]byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	if got := versionRequiredByContent(d); got != "2.0" {
		t.Fatalf("versionRequiredByContent = %q, want the 2.0 the embedded entry shape already required", got)
	}
}

func TestAnAcknowledgedAssetNoChainNamesRaisesNothing(t *testing.T) {
	d, err := ParseDocument([]byte(embeddedFontDoc(acknowledgedFontAssetBody, unreferencedChain)))
	if err != nil {
		t.Fatalf("an unreferenced acknowledged asset was refused: %v", err)
	}
	// EXACTLY the version this document's other content requires — its chain
	// is one bare face name, so that is the floor — and not merely "some
	// version that is not 4.2", which a probe returning any other wrong rank
	// would satisfy.
	if got := versionRequiredByContent(d); got != baseVersion {
		t.Fatalf("versionRequiredByContent = %q, want %q — an acknowledged asset NO chain names raises nothing; the trigger is the entry, not the asset", got, baseVersion)
	}
}

// AND THE KEY ROUND-TRIPS IN ALL THREE STATES, because absence, an explicit
// null and a set value must stay distinguishable on the way out or a
// hand-authored document changes meaning when this library saves it.
func TestTheAcknowledgementRoundTripsInAllThreeStates(t *testing.T) {
	for _, spelling := range []struct{ label, text, want string }{
		{"true", `"authorAcknowledged": true,`, `"authorAcknowledged": true`},
		{"false", `"authorAcknowledged": false,`, `"authorAcknowledged": false`},
		{"null", `"authorAcknowledged": null,`, `"authorAcknowledged": null`},
	} {
		t.Run(spelling.label, func(t *testing.T) {
			// UNREFERENCED, so `false` and `null` — which excuse nothing — are
			// not also being asked to satisfy the terms rule.
			body := strings.Replace(acknowledgedFontAssetBody, `"authorAcknowledged": true,`, spelling.text, 1)
			out := canonicalFixedPoint(t, embeddedFontDoc(body, unreferencedChain))
			if !strings.Contains(out, spelling.want) {
				t.Fatalf("the %s spelling did not survive the round trip:\n%s", spelling.label, out)
			}
		})
	}
}
