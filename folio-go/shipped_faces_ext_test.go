package folio8_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/panitw/folio8/folio-go/fonts"
)

// shippedPostScriptNames is the expected (FontSet key -> PostScript
// name) mapping for the shipped set. It duplicates the two fields of
// shippedFaceSpecs that this external test package cannot see —
// shipped_faces_test.go is `package folio8`, and Go gives an external
// test package no access to another test package's identifiers.
//
// That duplication is closed rather than tolerated: the counts and keys
// are asserted equal to fonts.Shipped() below, and
// TestShippedSpecCoversEverythingShipped (package folio8) asserts the
// same keys against testShippedFontSet(). Both chains terminate at
// fonts.Shipped(), so a face added there and nowhere else fails in this
// file, and a face added to the spec and nowhere else fails in that one.
//
// ⚠ "Noto Sans Italic" and "Noto Sans Bold Italic" map to
// `NotoSansItalic-Italic` and `NotoSansItalic-BoldItalic`. Those are not
// typos — fontTools composes name[6] from the SOURCE variable font's
// variations-PostScript prefix, which for the italic VF is
// `NotoSansItalic`. shipped_faces_test.go's shippedFaceSpecs carries the
// full explanation; "correcting" either string here reds this file and
// shipped_faces_test.go together, because both are compared against the
// binary rather than against each other.
var shippedPostScriptNames = map[string]string{
	"Noto Sans":             "NotoSans-Regular",
	"Noto Sans Bold":        "NotoSans-Bold",
	"Noto Sans Italic":      "NotoSansItalic-Italic",
	"Noto Sans Bold Italic": "NotoSansItalic-BoldItalic",
	"Noto Sans Thai":        "NotoSansThai-Regular",
	"Noto Sans Thai Bold":   "NotoSansThai-Bold",
	"Noto Sans SC":          "NotoSansSC-Regular",
	"Roboto":                "Roboto-Regular",
	"Roboto Bold":           "Roboto-Bold",
	"Roboto Italic":         "Roboto-Italic",
	"Roboto Bold Italic":    "Roboto-BoldItalic",
}

// shippedFaceFiles is where each shipped face's bytes are supposed to
// come from, so this test can prove fonts.Shipped() returns the bytes of
// the COMMITTED file rather than something built another way.
var shippedFaceFiles = map[string]string{
	"Noto Sans":             "notosans/NotoSans-Regular.ttf",
	"Noto Sans Bold":        "notosans-bold/NotoSans-Bold.ttf",
	"Noto Sans Italic":      "notosans-italic/NotoSans-Italic.ttf",
	"Noto Sans Bold Italic": "notosans-bolditalic/NotoSans-BoldItalic.ttf",
	"Noto Sans Thai":        "notosansthai/NotoSansThai-Regular.ttf",
	"Noto Sans Thai Bold":   "notosansthai-bold/NotoSansThai-Bold.ttf",
	"Noto Sans SC":          "notosanssc/NotoSansSC-Regular.ttf",
	"Roboto":                "roboto/Roboto-Regular.ttf",
	"Roboto Bold":           "roboto-bold/Roboto-Bold.ttf",
	"Roboto Italic":         "roboto-italic/Roboto-Italic.ttf",
	"Roboto Bold Italic":    "roboto-bolditalic/Roboto-BoldItalic.ttf",
}

// TestFontsShippedMatchesExpectedFaceSet is the outermost link in the
// parity chain. Before Story 2.2's finisher, NOTHING tied the number of
// recorded goldens to fonts.Shipped()'s cardinality: the reviewer added
// a fourth face to Shipped() and the entire root package stayed green,
// with four shipped (face, instance) pairs against three goldens. The
// guard NAMED for that hazard compared a slice literal's length to a
// hard-coded 3 declared six lines above it.
func TestFontsShippedMatchesExpectedFaceSet(t *testing.T) {
	shipped := fonts.Shipped()

	if len(shipped) == 0 {
		t.Fatal("fonts.Shipped() is EMPTY — every parity assertion below would be vacuous")
	}
	if len(shipped) != len(shippedPostScriptNames) {
		t.Fatalf(
			"fonts.Shipped() carries %d faces, but %d are accounted for here.\n"+
				"Every shipped face needs: a spec row (shipped_faces_test.go), a PostScript name here, "+
				"and a per-instance golden (fixture_test.go). Adding one to Shipped() alone means it ships "+
				"with its weight, its names and its byte-stability asserted by NOTHING — D-2.2.1's V7 hazard, "+
				"and the standing condition every later pinned instance inherits (DW-12).",
			len(shipped), len(shippedPostScriptNames),
		)
	}
	for key := range shippedPostScriptNames {
		data, ok := shipped[key]
		if !ok {
			t.Fatalf("face %q is accounted for here but absent from fonts.Shipped()", key)
		}
		if len(data) == 0 {
			t.Fatalf("fonts.Shipped()[%q] is zero bytes", key)
		}
	}
	for key := range shipped {
		if _, ok := shippedPostScriptNames[key]; !ok {
			t.Fatalf(
				"fonts.Shipped() carries face %q, which is accounted for nowhere — no spec row, no "+
					"PostScript name, no golden", key,
			)
		}
	}
}

// TestFontsShippedReturnsTheCommittedBytes proves fonts.Shipped()'s
// embeds resolve to the committed face files — the artifact, not a
// same-named file produced some other way (D-000.21).
func TestFontsShippedReturnsTheCommittedBytes(t *testing.T) {
	root := repoRootForExtTest(t)
	shipped := fonts.Shipped()

	compared := 0
	for key, rel := range shippedFaceFiles {
		embedded, ok := shipped[key]
		if !ok {
			t.Fatalf("fonts.Shipped() has no face %q", key)
		}
		path := filepath.Join(root, "folio-go", "fonts", filepath.FromSlash(rel))
		onDisk, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		// Assert both operands are non-empty BEFORE comparing. Two
		// failed reads comparing empty to empty is how a reproducibility
		// check reports IDENTICAL for files that were never produced.
		if len(embedded) == 0 || len(onDisk) == 0 {
			t.Fatalf(
				"face %q: refusing to compare — embedded %d bytes, on-disk %d bytes",
				key, len(embedded), len(onDisk),
			)
		}
		got := sha256.Sum256(embedded)
		want := sha256.Sum256(onDisk)
		if got != want {
			t.Fatalf(
				"face %q: fonts.Shipped() returned bytes that are NOT %s\n  embedded sha256 %s\n  on-disk  sha256 %s",
				key, path, hex.EncodeToString(got[:]), hex.EncodeToString(want[:]),
			)
		}
		compared++
	}
	if compared != len(shippedFaceFiles) {
		t.Fatalf("coverage witness: compared %d of %d faces", compared, len(shippedFaceFiles))
	}
}

// repoRootForExtTest walks up from the test's working directory to the
// repository root, the same way repoRootFromTest does for the white-box
// tests (which this package cannot call).
func repoRootForExtTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "folio-go", "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "fixtures")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate the repository root from the test working directory")
	return ""
}
