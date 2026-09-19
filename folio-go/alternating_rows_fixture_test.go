package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const subprocessAlternatingRowsEnvVar = "FOLIO8_SUBPROCESS_RENDER_ALTERNATINGROWS"

const alternatingRowsTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "id": "e1",
          "type": "table",
          "x": 0,
          "y": 0,
          "bind": "items[]",
          "headerHeight": 20,
          "style": {
            "fontFamily": "body",
            "fontSize": 9
          },
          "headerStyle": {
            "background": "#445566"
          },
          "altRowBackground": "#DDEEFF",
          "columns": [
            {
              "id": "e2",
              "label": "Transaction",
              "width": 180,
              "bind": "{{row.label}}"
            }
          ]
        }
      ]
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
  "fonts": {
    "body": [
      "Noto Sans"
    ]
  },
  "locale": "en",
  "nextId": 3,
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
}`

const alternatingRowsDataJSON = `{
  "items": [
    {"label": "Alpha"},
    {"label": "Bravo"},
    {"label": "Charlie"},
    {"label": "Delta"},
    {"label": "Echo"}
  ]
}`

func renderAlternatingRowsFixture() ([]byte, error) {
	tpl, err := ParseTemplate([]byte(alternatingRowsTemplateJSON))
	if err != nil {
		return nil, err
	}
	res, err := Render(tpl, Data(alternatingRowsDataJSON), nil, testShippedFontSet())
	if err != nil {
		return nil, err
	}
	return res.Bytes, nil
}

func TestAlternatingRowsFixtureSourcesMatchMatrixConstants(t *testing.T) {
	root := repoRootFromTest(t)
	for _, tc := range []struct {
		name     string
		path     string
		constant string
	}{
		{name: "input.folio", path: filepath.Join(root, "fixtures", "alternating-rows", "input.folio"), constant: alternatingRowsTemplateJSON},
		{name: "data.json", path: filepath.Join(root, "fixtures", "alternating-rows", "data.json"), constant: alternatingRowsDataJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			var gotCompact, wantCompact bytes.Buffer
			if err := json.Compact(&gotCompact, got); err != nil {
				t.Fatalf("compact fixture source: %v", err)
			}
			if err := json.Compact(&wantCompact, []byte(tc.constant)); err != nil {
				t.Fatalf("compact matrix constant: %v", err)
			}
			if !bytes.Equal(gotCompact.Bytes(), wantCompact.Bytes()) {
				t.Fatalf("%s and its matrix subprocess constant have drifted", tc.name)
			}
		})
	}
}

func TestAlternatingRowsGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureDir := filepath.Join(root, "fixtures", "alternating-rows")
	fixture := loadExpectedFixture(t, filepath.Join(fixtureDir, "expected.json"))
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if runtime.Version() != fixture.GoToolchain {
		t.Fatalf("toolchain mismatch: running %s, fixture recorded with %s", runtime.Version(), fixture.GoToolchain)
	}

	produced, err := renderAlternatingRowsFixture()
	if err != nil {
		t.Fatalf("render alternating-rows: %v", err)
	}
	if len(produced) == 0 {
		t.Fatal("produced PDF is empty")
	}
	assertWellFormedPDF(t, "alternating-rows produced PDF", produced, 1)

	expected, err := os.ReadFile(filepath.Join(fixtureDir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	expectedSum := sha256.Sum256(expected)
	if got := hex.EncodeToString(expectedSum[:]); got != fixture.SHA256 {
		t.Fatalf("expected.pdf digest = %s, expected.json records %s", got, fixture.SHA256)
	}
	if !bytes.Equal(produced, expected) {
		producedSum := sha256.Sum256(produced)
		t.Fatalf("produced PDF digest = %s, want committed %s", hex.EncodeToString(producedSum[:]), fixture.SHA256)
	}

	const alternateOperator = "0.867 0.933 1 rg\n"
	if got := bytes.Count(produced, []byte(alternateOperator)); got != 2 {
		t.Fatalf("produced PDF carries %d alternate-colour operators, want exactly 2 for collection indexes 1 and 3", got)
	}
	first := []byte("0.867 0.933 1 rg\n36 741.374 180 12.258 re f\n")
	second := []byte("0.867 0.933 1 rg\n36 716.858 180 12.258 re f\n")
	firstAt := bytes.Index(produced, first)
	secondAt := bytes.Index(produced, second)
	if firstAt == -1 || secondAt == -1 {
		t.Fatalf("produced PDF does not carry the two test-owned alternate fill rectangles at y=741.374 and y=716.858 (found offsets %d/%d)", firstAt, secondAt)
	}
	if firstAt >= secondAt {
		t.Fatalf("alternate fill rectangles are not in vertical row order: first offset=%d second offset=%d", firstAt, secondAt)
	}
	if got := bytes.Count(produced, []byte("0.267 0.333 0.4 rg\n")); got != 1 {
		t.Errorf("produced PDF carries %d header-colour operators, want exactly 1 distinct header fill", got)
	}
}
