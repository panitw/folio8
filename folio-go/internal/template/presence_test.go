package template

import (
	"strings"
	"testing"
)

// TestPresenceThreePolarity is AC39 (D-1.4.8, corrected): a three-
// polarity fixture (absent / null / present) asserts absent, null and
// present-with-value stay distinguishable after parse. style.background
// is the representative field (AD-14's forward-looking case).
func TestPresenceThreePolarity(t *testing.T) {
	mk := func(styleBody string) []byte {
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
      "elements": [
        {
          "height": 10,
          "id": "e1",
          ` + styleBody + `
          "type": "text",
          "value": "v",
          "width": 10,
          "x": 0,
          "y": 0
        }
      ],
      "height": 20
    }
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {
    "margin": { "bottom": 36, "left": 36, "right": 36, "top": 36 },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
	}

	t.Run("absent", func(t *testing.T) {
		d, err := ParseDocument(mk(""))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		st := d.Bands.PageHeader.Elements[0].Style
		if st.Set {
			t.Fatal("style must be absent")
		}
	})

	t.Run("null", func(t *testing.T) {
		d, err := ParseDocument(mk(`"style": null,`))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		st := d.Bands.PageHeader.Elements[0].Style
		if !st.Set || !st.Null {
			t.Fatalf("style must be Set && Null, got %+v", st)
		}
	})

	t.Run("present-with-background-null", func(t *testing.T) {
		d, err := ParseDocument(mk(`"style": {"background": null},`))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		st := d.Bands.PageHeader.Elements[0].Style
		if !st.Set || st.Null {
			t.Fatalf("style must be Set && !Null, got %+v", st)
		}
		bg := st.Value.Background
		if !bg.Set || !bg.Null {
			t.Fatalf("style.background must be Set && Null, got %+v", bg)
		}
	})

	t.Run("present-with-background-value", func(t *testing.T) {
		d, err := ParseDocument(mk(`"style": {"background": "#FFFFFF"},`))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		bg := d.Bands.PageHeader.Elements[0].Style.Value.Background
		if !bg.Set || bg.Null || bg.Value != "#FFFFFF" {
			t.Fatalf("style.background must be Set, !Null, value #FFFFFF, got %+v", bg)
		}
	})

	// TestPresenceThreePolarity/serializes-differently is D-1.4.16's
	// required behavioural guard (this story's finisher review, Finding
	// 6, Major): "a test must prove {} and {"a":null} produce different
	// Presence values AND serialize back differently … That test is what
	// makes D-1.4.8 real rather than declared." Parse-side distinctness
	// was already asserted above (the three subtests before this one);
	// this subtest is the missing serialization-side half — without it,
	// a future edit collapsing Null into absent at write time would go
	// unnoticed, since nothing downstream of parse ever reads Presence's
	// Null flag today (D-1.4.16's own diagnosis: "nothing branches on
	// Null vs absent").
	t.Run("serializes-differently", func(t *testing.T) {
		dAbsent, err := ParseDocument(mk(""))
		if err != nil {
			t.Fatalf("parse absent: %v", err)
		}
		dNull, err := ParseDocument(mk(`"style": null,`))
		if err != nil {
			t.Fatalf("parse null: %v", err)
		}
		dPresent, err := ParseDocument(mk(`"style": {"background": "#FFFFFF"},`))
		if err != nil {
			t.Fatalf("parse present: %v", err)
		}

		outAbsent, err := SerializeDocument(dAbsent)
		if err != nil {
			t.Fatalf("serialize absent: %v", err)
		}
		outNull, err := SerializeDocument(dNull)
		if err != nil {
			t.Fatalf("serialize null: %v", err)
		}
		outPresent, err := SerializeDocument(dPresent)
		if err != nil {
			t.Fatalf("serialize present: %v", err)
		}

		if strings.Contains(string(outAbsent), `"style"`) {
			t.Fatalf("absent style must not appear as a key at all:\n%s", outAbsent)
		}
		if !strings.Contains(string(outNull), `"style": null`) {
			t.Fatalf("null style must serialize as \"style\": null:\n%s", outNull)
		}
		if !strings.Contains(string(outPresent), `"style": {`) {
			t.Fatalf("present style must serialize as an object:\n%s", outPresent)
		}
		if string(outAbsent) == string(outNull) || string(outNull) == string(outPresent) || string(outAbsent) == string(outPresent) {
			t.Fatalf("absent, null and present-with-value must all three produce DISTINCT serialized bytes:\nabsent:\n%s\nnull:\n%s\npresent:\n%s", outAbsent, outNull, outPresent)
		}
	})
}
