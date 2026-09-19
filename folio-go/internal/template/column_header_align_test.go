package template

import (
	"bytes"
	"strings"
	"testing"
)

// columnHeaderAlignDoc is a one-table document at version 1.0 whose single
// column carries `extra` verbatim (e.g. `, "headerAlign": "center"`).
func columnHeaderAlignDoc(extra string) []byte {
	return []byte(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "bind": "items[]",
          "columns": [
            {
              "align": "right",
              "bind": "{{row.a}}",
              "id": "e2",
              "label": "A",
              "width": 60` + extra + `
            }
          ],
          "headerHeight": 10,
          "id": "e1",
          "type": "table",
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 10
    },
    "pageHeader": {
      "elements": [],
      "height": 10
    }
  },
  "fonts": {},
  "locale": "en",
  "nextId": 3,
  "page": {
    "margin": {
      "bottom": 10,
      "left": 10,
      "right": 10,
      "top": 10
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
}

// A declared headerAlign loads, round-trips, and raises the document to 3.2.
func TestColumnHeaderAlignRequires32(t *testing.T) {
	d, err := ParseDocument(columnHeaderAlignDoc(`,
              "headerAlign": "center"`))
	if err != nil {
		t.Fatalf("a column headerAlign must load: %v", err)
	}
	if got := d.Bands.Content.Elements[0].Table.Value.Columns[0].HeaderAlign; !got.Set || got.Value != AlignCenter {
		t.Fatalf("headerAlign decoded as %#v, want present center", got)
	}
	if got := versionRequiredByContent(d); got != columnHeaderAlignVersion {
		t.Fatalf("versionRequiredByContent = %q, want %q", got, columnHeaderAlignVersion)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"headerAlign": "center"`)) || !bytes.Contains(out, []byte(`"version": "3.2"`)) {
		t.Fatalf("serialized document must carry headerAlign at 3.2:\n%s", out)
	}
}

// A document without the key is untouched: same bytes, same version.
func TestColumnHeaderAlignAbsentIsByteIdentical(t *testing.T) {
	in := columnHeaderAlignDoc("")
	d, err := ParseDocument(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(in, out) {
		t.Fatalf("a document without headerAlign must serialize byte-identically:\n--- in\n%s\n--- out\n%s", in, out)
	}
}

// Its own closed set: justify and anything else is a located load error whose
// message names exactly left, center, right.
func TestColumnHeaderAlignIsAClosedSet(t *testing.T) {
	for _, value := range []string{`"justify"`, `"middle"`, `""`, `3`} {
		_, err := ParseDocument(columnHeaderAlignDoc(`,
              "headerAlign": ` + value))
		if err == nil {
			t.Fatalf("headerAlign %s loaded; it must be refused", value)
		}
		if !strings.Contains(err.Error(), "headerAlign") || !strings.Contains(err.Error(), "e2") {
			t.Errorf("headerAlign %s: error %q must name the field and the column", value, err)
		}
		if strings.Contains(err.Error(), "not one of") && strings.Contains(err.Error(), "justify, ") {
			t.Errorf("headerAlign %s: error %q names justify as legal", value, err)
		}
	}
}
