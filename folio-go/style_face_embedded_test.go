package folio8

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// This file is Story 11.2's EMBEDDED arm at render: a chain entry that
// names a face the DOCUMENT carries, declaring a style variant that is
// another face the document carries.
//
// It exists because the arm was otherwise unexercised end to end.
// Measured by mutation: making FontChainEntry.EmbeddedAssetKeys return
// only the discriminant — which un-mints every variant's reserved name,
// so nothing can ever resolve one — left the whole suite green. Every
// other embedded test in this package predates style variants, and every
// variant test uses supplied faces, so the two halves never met.

// embeddedVariantBoldBytes is the SECOND carried face: the shipped Noto
// Sans Thai BOLD, embedded as an asset exactly as embedded_font_fixture_test.go
// embeds the Regular. NO NEW BINARY ENTERS THE REPOSITORY — it is the
// same folio-go/fonts/notosansthai-bold/NotoSansThai-Bold.ttf the module
// already commits, reached through the test binary's own embed so the
// bytes travel to every cross-target leg.
func embeddedVariantBoldBytes() []byte { return testShippedNotoSansThaiBold }

func embeddedVariantBoldKey() string {
	sum := sha256.Sum256(embeddedVariantBoldBytes())
	return fmt.Sprintf("%x", sum)
}

// embeddedVariantDoc builds a document carrying BOTH Thai cuts as assets
// and drawing Thai text through the chain and style it is given.
//
// The text is pure Thai and the chain names no shipped face, for
// fixtures/embedded-font's own reason (D-8.4.4b): the carried faces are
// then the ONLY faces that can draw this page, so which one reached it
// is an identity the run's Face states outright rather than something
// inferred from a count.
func embeddedVariantDoc(t *testing.T, chain, style string) string {
	t.Helper()
	asset := func(key string, bytes []byte, family, styleName, licence, source string) string {
		encoded := base64Wrapped76(bytes)
		var data strings.Builder
		for i, line := range encoded {
			if i > 0 {
				data.WriteString(",\n")
			}
			data.WriteString("        \"" + line + "\"")
		}
		firstLine, _, _ := strings.Cut(licence, "\n")
		return `    "` + key + `": {
      "data": [
` + data.String() + `
      ],
      "font": {
        "copyright": ` + jsonStringLiteral(strings.TrimSpace(firstLine)) + `,
        "family": ` + jsonStringLiteral(family) + `,
        "licence": "SIL Open Font License 1.1",
        "licenceText": ` + jsonStringLiteral(strings.TrimRight(licence, "\n")) + `,
        "source": ` + jsonStringLiteral(source) + `,
        "style": ` + jsonStringLiteral(styleName) + `
      },
      "mediaType": "font/ttf"
    }`
	}
	assets := []string{
		asset(embeddedFontAssetKey(), embeddedFontAssetBytes(), "Noto Sans Thai", "Regular",
			testShippedNotoSansThaiLicence, "folio-go/fonts/notosansthai/NotoSansThai-Regular.ttf"),
		asset(embeddedVariantBoldKey(), embeddedVariantBoldBytes(), "Noto Sans Thai Bold", "Bold",
			testShippedNotoSansThaiBoldLicence, "folio-go/fonts/notosansthai-bold/NotoSansThai-Bold.ttf"),
	}
	// The assets map's key order is the author's here; the loader sorts
	// on write and this document is never compared byte-for-byte.
	return `{
  "assets": {
` + strings.Join(assets, ",\n") + `
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 40, "value": "สัญญา", "style": ` + style + `}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ` + chain + `},
  "locale": "th",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "2.0"
}`
}

// TestAnEmbeddedEntrysDeclaredVariantDrawsTheCarriedBoldFace is P3: the
// whole embedded arm, end to end.
//
// It pins three things a supplied-face test cannot reach:
//   - chainFaceNames mints the VARIANT's reserved name through
//     embeddedFaceName, exactly as it mints the entry's own;
//   - newEmbeddedFaceIndex walks the variant asset key, so that reserved
//     name resolves to real bytes (the second mint site, which used to
//     read entry.AssetKey alone and would have failed SILENTLY here);
//   - the parser accepted a same-namespace sibling and required its
//     terms.
func TestAnEmbeddedEntrysDeclaredVariantDrawsTheCarriedBoldFace(t *testing.T) {
	regular, bold := embeddedFontAssetKey(), embeddedVariantBoldKey()
	if regular == bold {
		t.Fatal("precondition: the two carried faces hash to one key, so the document carries one asset and the variant is not a SECOND face at all")
	}
	// The FontSet supplies nothing this document names, so a face on the
	// page can only have come from the document's own assets.
	fs := FontSet{"Noto Sans": testShippedNotoSans}

	source := embeddedVariantDoc(t,
		`[{"asset": "`+regular+`", "bold": "`+bold+`"}]`,
		`{"fontFamily": "body", "fontSize": 12, "bold": true}`)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("a document carrying a licensed embedded bold variant did not load: %v", err)
	}
	pages, _, _, diags, err := buildPageModel(tpl, mustDecodeData(t, `{}`), mustDecodeParams(t), fs)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var faces []string
	for _, p := range pages {
		for _, r := range p.Runs {
			faces = append(faces, r.Face)
		}
	}
	requireFaces(t, faces, embeddedFaceName(bold))
	if w := styleFaceWarnings(diags); len(w) != 0 {
		t.Fatalf("a declared and resolvable embedded variant still warned: %+v", w)
	}
}

// TestAnEmbeddedEntrysAbsenceWarningNamesTheFaceByItsDISPLAYName is the
// other half, and it is the only path on which AC3's message differs
// from a raw face name.
//
// A carried face's render-path name is "asset:" plus 64 hex characters.
// Printed verbatim in a Warning it reads as though the author mistyped a
// font name, so the message goes through faceDisplayName and spells the
// display identity the asset itself carries — which is exactly what
// formatFontChain has always done for a chain, applied here to one face.
func TestAnEmbeddedEntrysAbsenceWarningNamesTheFaceByItsDISPLAYName(t *testing.T) {
	regular := embeddedFontAssetKey()
	fs := FontSet{"Noto Sans": testShippedNotoSans}

	source := embeddedVariantDoc(t,
		`[{"asset": "`+regular+`"}]`,
		`{"fontFamily": "body", "fontSize": 12, "bold": true}`)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, _, _, diags, err := buildPageModel(tpl, mustDecodeData(t, `{}`), mustDecodeParams(t), fs)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	w := styleFaceWarnings(diags)
	if len(w) == 0 {
		t.Fatal("an embedded entry with no declared variant rendered its base face silently")
	}
	for _, d := range w {
		if !strings.Contains(d.Message, "Noto Sans Thai") {
			t.Errorf("Warning %q does not spell the carried face by its DISPLAY identity", d.Message)
		}
		if strings.Contains(d.Message, regular) {
			t.Errorf("Warning %q prints the carried face's 64-hex render-path name, which reads to an author as a mistyped font name", d.Message)
		}
	}
}
