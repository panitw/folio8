package template

import (
	"errors"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/diag"
)

// multiPageFixture is SPEC-multi-pages' canonical multi-page document
// (D-G.1): every page in `pages`, an empty `bands.content`, Page Break
// written explicitly on pages[1..] (one false, one true), a section break on
// pages[0], a keepTogether group on one page, and an empty last page.
var multiPageFixture = []byte(`{
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
  "nextId": 4,
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
  "pages": [
    {
      "elements": [
        {
          "height": 14,
          "id": "e1",
          "keepTogether": "clause",
          "type": "text",
          "value": "Clause one",
          "width": 200,
          "x": 0,
          "y": 0
        },
        {
          "height": 14,
          "id": "e2",
          "keepTogether": "clause",
          "type": "text",
          "value": "Clause two",
          "width": 200,
          "x": 0,
          "y": 20
        }
      ],
      "sectionBreak": 300,
      "sectionBreakAnchor": false
    },
    {
      "elements": [
        {
          "height": 14,
          "id": "e3",
          "type": "text",
          "value": "Signature",
          "width": 200,
          "x": 0,
          "y": 0
        }
      ],
      "pageBreak": false
    },
    {
      "elements": [],
      "pageBreak": true
    }
  ],
  "utcOffset": "+00:00",
  "version": "4.1"
}
`)

func mustSerialize(t *testing.T, d *Document) string {
	t.Helper()
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	return string(out)
}

func replaceOnce(t *testing.T, s, old, new string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("fixture precondition: %q not found", old)
	}
	return strings.Replace(s, old, new, 1)
}

func TestMultiPageDocumentRoundTripsByteIdentically(t *testing.T) {
	d, err := ParseDocument(multiPageFixture)
	if err != nil {
		t.Fatal(err)
	}
	if d.PageCount() != 3 || len(d.Bands.Content.Elements) != 0 {
		t.Fatalf("pages %d, bands.content elements %d", d.PageCount(), len(d.Bands.Content.Elements))
	}
	if !d.Pages[0].PageBreak || d.Pages[1].PageBreak || !d.Pages[2].PageBreak {
		t.Fatalf("page breaks %v %v %v, want true false true", d.Pages[0].PageBreak, d.Pages[1].PageBreak, d.Pages[2].PageBreak)
	}
	if !d.Pages[0].SectionBreak.Set || d.Pages[0].SectionBreak.Value != 300000 {
		t.Fatalf("pages[0] section break %+v", d.Pages[0].SectionBreak)
	}
	if got := mustSerialize(t, d); got != string(multiPageFixture) {
		t.Fatalf("round trip changed the bytes:\n%s", got)
	}
}

func TestAMissingPageBreakLoadsAsOnAndIsWrittenExplicitly(t *testing.T) {
	src := replaceOnce(t, string(multiPageFixture), `"elements": [],
      "pageBreak": true`, `"elements": []`)
	d, err := ParseDocument([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Pages[2].PageBreak {
		t.Fatal("pages[2] with no pageBreak loaded as off")
	}
	if got := mustSerialize(t, d); got != string(multiPageFixture) {
		t.Fatalf("save did not write pageBreak true:\n%s", got)
	}
}

func TestAPageBreakOnTheFirstPageIsIgnoredAndDroppedOnSave(t *testing.T) {
	src := replaceOnce(t, string(multiPageFixture), `"sectionBreak": 300,`, `"pageBreak": false,
      "sectionBreak": 300,`)
	d, err := ParseDocument([]byte(src))
	if err != nil {
		t.Fatalf("a pageBreak on pages[0] must load: %v", err)
	}
	if got := mustSerialize(t, d); got != string(multiPageFixture) {
		t.Fatalf("save kept pages[0]'s pageBreak:\n%s", got)
	}
}

func TestTheMultiPageShapeDeclares41(t *testing.T) {
	src := replaceOnce(t, string(multiPageFixture), `"version": "4.1"`, `"version": "1.0"`)
	src = replaceOnce(t, src, `,
      "sectionBreak": 300,
      "sectionBreakAnchor": false`, "")
	d, err := ParseDocument([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustSerialize(t, d); !strings.Contains(got, `"version": "4.1"`) {
		t.Fatalf("a multi-page document saved without 4.1:\n%s", got)
	}
}

func TestAOnePageDocumentKeepsItsShape(t *testing.T) {
	d := &Document{}
	if d.PageCount() != 1 || len(d.ContentBands()) != 1 || d.ContentBands()[0] != &d.Bands.Content || d.PageField(0) != "bands.content" {
		t.Fatal("a document with no Pages is not the one-page shape")
	}
}

func TestInvalidPagesAreRefusedNamingTheirLocation(t *testing.T) {
	base := string(multiPageFixture)
	cases := []struct {
		name, src string
		code      diag.Code
		field     string
		reason    []string
	}{
		{"both places", replaceOnce(t, base, `"content": {
      "elements": []`, `"content": {
      "elements": [{"height": 14, "id": "e9", "type": "text", "value": "x", "width": 10, "x": 0, "y": 0}]`), diag.CodePagesInvalid, "bands.content", nil},
		{"zero pages", base[:strings.Index(base, `"pages": [`)] + `"pages": [],
  "utcOffset": "+00:00",
  "version": "4.1"
}
`, diag.CodePagesInvalid, "pages", nil},
		{"one page", base[:strings.Index(base, `"pages": [`)] + `"pages": [{"elements": []}],
  "utcOffset": "+00:00",
  "version": "4.1"
}
`, diag.CodePagesInvalid, "pages", nil},
		{"not an array", base[:strings.Index(base, `"pages": [`)] + `"pages": {},
  "utcOffset": "+00:00",
  "version": "4.1"
}
`, diag.CodePagesInvalid, "pages", nil},
		{"unknown key", replaceOnce(t, base, `"pageBreak": false`, `"pageBreak": false, "height": 10`), diag.CodePagesInvalid, "pages[1].height", nil},
		{"string pageBreak", replaceOnce(t, base, `"pageBreak": false`, `"pageBreak": "false"`), diag.CodePagesInvalid, "pages[1].pageBreak", nil},
		{"null pageBreak", replaceOnce(t, base, `"pageBreak": false`, `"pageBreak": null`), diag.CodePagesInvalid, "pages[1].pageBreak", nil},
		{"entry not an object", replaceOnce(t, base, `{
      "elements": [],
      "pageBreak": true
    }`, `7`), diag.CodePagesInvalid, "pages[2]", nil},
		{"duplicate id", replaceOnce(t, base, `"id": "e3"`, `"id": "e1"`), diag.CodeTemplateFieldInvalid, "pages[1].elements[].id", []string{"duplicate id"}},
		{"tag spans pages", replaceOnce(t, base, `"id": "e3",`, `"id": "e3",
          "keepTogether": "clause",`), diag.CodePagesInvalid, "pages[1]", []string{"pages[0]", "pages[1]"}},
		// SPEC-multi-pages story 5: a later page may declare a break; a
		// malformed one is refused at that page's key.
		{"later-page null break", replaceOnce(t, base, `"pageBreak": false`, `"pageBreak": false, "sectionBreak": null`), diag.CodeSectionBreakInvalid, "pages[1].sectionBreak", nil},
		{"later-page anchor without break", replaceOnce(t, base, `"pageBreak": false`, `"pageBreak": false, "sectionBreakAnchor": false`), diag.CodeSectionBreakInvalid, "pages[1].sectionBreakAnchor", nil},
		{"break on bands.content beside pages", replaceOnce(t, base, `"content": {
      "elements": []`, `"content": {
      "elements": [], "sectionBreak": 100`), diag.CodePagesInvalid, "bands.content.sectionBreak", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDocument([]byte(c.src))
			var le *LoadError
			if !errors.As(err, &le) {
				t.Fatalf("got %v, want a *LoadError", err)
			}
			if le.Code != c.code || le.Field != c.field {
				t.Fatalf("code %q field %q, want %q at %q (%v)", le.Code, le.Field, c.code, c.field, err)
			}
			for _, want := range c.reason {
				if !strings.Contains(le.Reason, want) {
					t.Errorf("reason %q does not mention %q", le.Reason, want)
				}
			}
		})
	}
}
