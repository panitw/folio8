package folio8

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------
// Story 2.3, AC13: cross-validation against an INDEPENDENT reference
// implementation.
//
// The oracle is HarfBuzz itself, run offline, once, and frozen as
// fixtures/shaped-text/harfbuzz-oracle.json. Three candidates were
// considered and two rejected:
//
//   - go-text/typesetting (epics.md:753 named it) is rejected twice
//     over. gomod_test.go's wantModuleGraph asserts `go list -m all`
//     equals exactly two modules, and `go list -m all` includes
//     test-only dependencies — so adding it fails a committed guard
//     whose entire purpose is to make a new module a conscious act
//     (D-1.5.1). And it is a SIBLING: textshape's README credits
//     benoitkugler/textlayout as its inspiration and go-text/typesetting
//     descends from the same code, so two ports with a shared ancestor
//     agreeing is the vacuous-citation shape — if both would make the
//     same mistake, the agreement predicts the same observation either
//     way and nothing has been learned.
//   - textshape's own harfbuzz-tests/ corpus is rejected because a
//     vendor's test corpus is curated by the vendor and selected to
//     pass. It says what the vendor already knew it got right.
//   - hb-shape, run against FOLIO8'S corpus, is the reference
//     implementation answering OUR questions. No module, no runtime
//     dependency, wantModuleGraph untouched.
//
// AD-25 permits exactly this: provenance may be "a one-time offline
// reference run, hand-checked — never a runtime dependency." Story 1.1
// set the precedent with qpdf --check, for the same reason.
//
// The oracle records its tool version and each row's exact argv
// verbatim, because a DESCRIBED command drifts from the run command.
// ---------------------------------------------------------------------

// hbOracleGlyph mirrors hb-shape's --output-format=json glyph object.
type hbOracleGlyph struct {
	G  int `json:"g"`
	Cl int `json:"cl"`
	Dx int `json:"dx"`
	Dy int `json:"dy"`
	Ax int `json:"ax"`
	Ay int `json:"ay"`
}

type hbOracleRow struct {
	Face       string          `json:"face"`
	FontFile   string          `json:"fontFile"`
	FontSHA256 string          `json:"fontSha256"`
	Text       string          `json:"text"`
	Argv       []string        `json:"argv"`
	Glyphs     []hbOracleGlyph `json:"glyphs"`
}

type hbOracle struct {
	Tool        string        `json:"tool"`
	ToolPath    string        `json:"toolPath"`
	ToolVersion string        `json:"toolVersion"`
	Method      string        `json:"method"`
	Rows        []hbOracleRow `json:"rows"`
	Notes       []string      `json:"notes"`
}

func loadHarfBuzzOracle(t *testing.T) hbOracle {
	t.Helper()
	root := repoRootFromTest(t)
	path := filepath.Join(root, "fixtures", "shaped-text", "harfbuzz-oracle.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read HarfBuzz oracle %s: %v", path, err)
	}
	var o hbOracle
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatalf("parse HarfBuzz oracle: %v", err)
	}

	// D-000.21 sharpened: prove the artifact carries the fields before
	// asserting about their contents. An oracle with no provenance is
	// not an oracle — it is a second copy of our own answer, and it
	// would agree with us for the wrong reason.
	if o.ToolVersion == "" {
		t.Fatal("oracle records no toolVersion — a described command drifts from the run command; the version is provenance, not decoration")
	}
	if o.Tool == "" || o.Method == "" {
		t.Fatal("oracle records no tool or method")
	}
	if len(o.Rows) == 0 {
		t.Fatal("oracle carries no rows — every assertion below would pass vacuously")
	}
	return o
}

// TestShapedExpectationsAgreeWithHarfBuzz is AC13: every row of the
// frozen expectation table must match HarfBuzz's answer for the same
// input, glyph for glyph and field for field.
//
// The two answers are independent in the way that matters: ours comes
// from textshape v0.0.15 running in this process; the oracle's comes
// from a separate binary, built from HarfBuzz's own C++ sources, that
// this module does not link, import or depend on in any way.
func TestShapedExpectationsAgreeWithHarfBuzz(t *testing.T) {
	oracle := loadHarfBuzzOracle(t)

	key := func(face, text string) string { return face + "\x00" + text }
	byKey := make(map[string]hbOracleRow, len(oracle.Rows))
	for _, r := range oracle.Rows {
		byKey[key(r.Face, r.Text)] = r
	}

	// Both directions. A table row with no oracle row is an uncovered
	// case reported as covered; an oracle row with no table row is an
	// expectation nobody checks.
	if len(oracle.Rows) != len(shapedExpectations) {
		t.Fatalf(
			"oracle has %d rows but the expectation table has %d — the cross-validation would silently cover a subset",
			len(oracle.Rows), len(shapedExpectations),
		)
	}

	checked := 0
	for _, row := range shapedExpectations {
		hb, ok := byKey[key(row.Face, row.Text)]
		if !ok {
			t.Errorf("expectation row %q/%q has no HarfBuzz oracle row — it is cross-validated by nothing", row.Face, row.Text)
			continue
		}

		// Cite the subject, not just the result: the oracle names the
		// exact font file it was run against, and that file's digest is
		// checked against the committed face here. A correct-looking
		// agreement measured against the wrong artifact is how this
		// story's own F3 went wrong.
		assertOracleSubject(t, hb)

		if len(hb.Glyphs) != len(row.Glyphs) {
			t.Errorf("%q/%q: HarfBuzz produced %d glyphs, the frozen table expects %d", row.Face, row.Text, len(hb.Glyphs), len(row.Glyphs))
			continue
		}
		for i := range row.Glyphs {
			want := row.Glyphs[i]
			got := hb.Glyphs[i]
			if got.G != int(want.GlyphID) {
				t.Errorf("%q/%q glyph %d: HarfBuzz glyph id %d, table expects %d", row.Face, row.Text, i, got.G, want.GlyphID)
			}
			if got.Cl != want.Cluster {
				t.Errorf("%q/%q glyph %d: HarfBuzz cluster %d, table expects %d", row.Face, row.Text, i, got.Cl, want.Cluster)
			}
			if got.Ax != int(want.XAdvance) {
				t.Errorf("%q/%q glyph %d: HarfBuzz x-advance %d, table expects %d", row.Face, row.Text, i, got.Ax, want.XAdvance)
			}
			if got.Dx != int(want.XOffset) {
				t.Errorf("%q/%q glyph %d: HarfBuzz x-offset %d, table expects %d", row.Face, row.Text, i, got.Dx, want.XOffset)
			}
			if got.Dy != int(want.YOffset) {
				t.Errorf("%q/%q glyph %d: HarfBuzz y-offset %d, table expects %d", row.Face, row.Text, i, got.Dy, want.YOffset)
			}
		}
		checked++
	}

	if checked == 0 {
		t.Fatal("vacuity guard: zero rows were cross-validated")
	}
	t.Logf(
		"HarfBuzz cross-validation witness — %d of %d expectation rows checked against %s, all five fields per glyph",
		checked, len(shapedExpectations), oracle.ToolVersion,
	)
}

// assertOracleSubject checks that the face the oracle was run against is
// byte-for-byte the face folio8 ships today. Without this, the oracle
// would keep agreeing with us after a face was replaced — reporting
// "cross-validated" about an artifact nobody uses any more.
func assertOracleSubject(t *testing.T, row hbOracleRow) {
	t.Helper()
	root := repoRootFromTest(t)
	data, err := os.ReadFile(filepath.Join(root, row.FontFile))
	if err != nil {
		t.Errorf("oracle row %q/%q names font file %s, which cannot be read: %v", row.Face, row.Text, row.FontFile, err)
		return
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != row.FontSHA256 {
		t.Errorf(
			"oracle row %q/%q was recorded against %s at sha256 %s, but the committed file is now %s — "+
				"the cross-validation is against an artifact folio8 no longer ships. Re-run the oracle:\n  %s",
			row.Face, row.Text, row.FontFile, row.FontSHA256, got, fmt.Sprint(row.Argv),
		)
	}
	if len(row.Argv) == 0 {
		t.Errorf("oracle row %q/%q records no argv — a described command drifts from the run command", row.Face, row.Text)
	}
}
