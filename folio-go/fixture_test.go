package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/pdf"
)

// expectedFixture mirrors fixtures/minimal-rect/expected.json.
type expectedFixture struct {
	Folio8GoVersion string `json:"folio8GoVersion"`
	GoToolchain     string `json:"goToolchain"`
	SHA256          string `json:"sha256"`
}

// repoRootFromTest finds the folio8 repository root by walking up from the
// current working directory until it finds a directory containing both
// folio-go/ and fixtures/. Duplicated (rather than shared) with
// internal/pdf's copy because Go test helpers do not cross package
// boundaries and internal/pdf is not importable from here in reverse; both
// copies exist only in _test.go files, which are exempt from AD-1's os
// ban (D-000.5 §Consequences).
func repoRootFromTest(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if isRepoRootDir(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root (a directory containing both folio-go/ and fixtures/) walking up from %s", dir)
		}
		dir = parent
	}
}

// isSHA256HexString reports whether s is exactly 64 lower-case hex
// characters — the JSON-string shape AC16 requires of expected.json's
// sha256 field.
func isSHA256HexString(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func isRepoRootDir(dir string) bool {
	folio8Go, err1 := os.Stat(filepath.Join(dir, "folio-go"))
	fixtures, err2 := os.Stat(filepath.Join(dir, "fixtures"))
	return err1 == nil && folio8Go.IsDir() && err2 == nil && fixtures.IsDir()
}

// checkFixtureShape is the AC1/AC6 pure checker underlying DW-1: it reads
// and validates the fixture at the given path — parses as JSON, carries
// both version fields, and has a sha256 field that is a JSON string of
// exactly 64 lower-case hex characters (AC16), never a per-target map,
// array or object (D-1.2.2). It takes the fixture's location as a
// parameter rather than a hard-coded relative path (AC6: the seam
// applied generally here, not patched into this one call site as a
// one-off), so it can be red-proved against a scratch copy without ever
// touching the real golden fixture (RP-4, DW-1).
//
// Blocker 3 (this story's QA review): this function, loadExpectedFixture
// and TestFixtureShapeCheckRedProof used to live in matrix_test.go,
// which carries the "matrix" build tag — so DW-1's entire red-proof
// executed in zero gates (the tagged suite is deferred to the Epic 1
// boundary, D-000.4; CI's only `-tags=matrix` steps are `go build` and
// `go vet`, neither of which runs a test). They live here instead,
// untagged, in package folio8's ordinary `go test ./...` path — the same
// home isSHA256HexString already has — so DW-1's closure actually runs
// on every story. matrix_test.go's TestCrossTargetByteIdentity and
// TestTargetRenderHash still call loadExpectedFixture below; being in a
// different file of the same package changes nothing about that call.
func checkFixtureShape(path string) (expectedFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return expectedFixture{}, fmt.Errorf("read fixture: %w", err)
	}
	var f expectedFixture
	if err := json.Unmarshal(data, &f); err != nil {
		return expectedFixture{}, fmt.Errorf("parse fixture JSON: %w", err)
	}
	if f.GoToolchain == "" {
		return expectedFixture{}, fmt.Errorf("fixture is missing goToolchain")
	}
	if f.SHA256 == "" {
		return expectedFixture{}, fmt.Errorf("fixture is missing sha256")
	}
	if !isSHA256HexString(f.SHA256) {
		return expectedFixture{}, fmt.Errorf("fixture sha256 %q is not a JSON string of exactly 64 lower-case hex characters (AC16)", f.SHA256)
	}
	return f, nil
}

// loadExpectedFixture reads fixtures/minimal-rect/expected.json — the
// same recorded golden Story 1.1's TestRenderMatchesGoldenFixture reads
// (AC7, vacuity guard 3: no second golden) — via checkFixtureShape, and
// fails the test on any shape violation.
func loadExpectedFixture(t *testing.T, path string) expectedFixture {
	t.Helper()
	f, err := checkFixtureShape(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestFixtureShapeCheckRedProof is DW-1's closure (AC6, RP-4): it
// red-proves AC16's shape check against scratch copies of
// fixtures/minimal-rect/expected.json, and confirms the real fixture
// file was never touched. Story 1.2's finisher deferred exactly this
// (DW-1) because the check previously had no way to be shown going red
// without mutating the real golden fixture.
//
// Finding 12 (this story's QA review): the fixture inventory promises
// two scratch shapes — "a scratch expected.json whose sha256 is a
// per-target object, and one whose value is upper-case or 63
// characters" — but only the first existed, so the isSHA256HexString
// branch of checkFixtureShape (as opposed to the earlier JSON-unmarshal
// failure) had no red-proof at all. Both now run as subtests.
func TestFixtureShapeCheckRedProof(t *testing.T) {
	root := repoRootFromTest(t)
	realPath := filepath.Join(root, "fixtures", "minimal-rect", "expected.json")

	before, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read real fixture: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(before, &raw); err != nil {
		t.Fatalf("parse real fixture: %v", err)
	}

	writeScratch := func(t *testing.T, mutated map[string]any) string {
		t.Helper()
		widened, err := json.MarshalIndent(mutated, "", "  ")
		if err != nil {
			t.Fatalf("marshal scratch fixture: %v", err)
		}
		scratchPath := filepath.Join(t.TempDir(), "expected.json")
		if err := os.WriteFile(scratchPath, widened, 0o644); err != nil {
			t.Fatalf("write scratch fixture: %v", err)
		}
		return scratchPath
	}

	t.Run("sha256 widened into a per-target object", func(t *testing.T) {
		mutated := map[string]any{}
		for k, v := range raw {
			mutated[k] = v
		}
		// The exact D-1.2.2 hazard AC16 exists to block.
		mutated["sha256"] = map[string]string{
			"darwin-arm64": "0000000000000000000000000000000000000000000000000000000000000000",
		}
		if _, err := checkFixtureShape(writeScratch(t, mutated)); err == nil {
			t.Fatal("expected checkFixtureShape to fail on a widened per-target sha256 object, got nil error")
		}
	})

	t.Run("sha256 wrong length or case", func(t *testing.T) {
		cases := []struct {
			name string
			sha  string
		}{
			{"63 characters", strings.Repeat("a", 63)},
			{"upper-case", strings.ToUpper(strings.Repeat("a", 64))},
		}
		for _, c := range cases {
			c := c
			t.Run(c.name, func(t *testing.T) {
				mutated := map[string]any{}
				for k, v := range raw {
					mutated[k] = v
				}
				mutated["sha256"] = c.sha
				if _, err := checkFixtureShape(writeScratch(t, mutated)); err == nil {
					t.Fatalf("expected checkFixtureShape to fail on sha256 = %q, got nil error", c.sha)
				}
			})
		}
	})

	after, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("re-read real fixture: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("real fixtures/minimal-rect/expected.json was mutated by this red-proof")
	}
}

// TestRenderMatchesGoldenFixture is AC11: the live render's hash must
// match the recorded golden fixture, and the fixture must carry both
// version fields (vacuity guard: a fixture missing either field fails
// before any hash comparison runs — this is RP-8's target).
//
// Ordering matters here, and this story's QA review (Major 5) measured
// why: the toolchain-mismatch check used to run, and t.Fatalf, before
// Render() was ever called. On any machine not running the exact
// recorded toolchain — every contributor who is not the recording
// machine, and every leg Story 1.2 is about to add — that meant *nothing
// in the module observed the output bytes at all*, because the golden
// hash was the only assertion that did. Render() now runs unconditionally
// and its output is checked structurally (assertWellFormedPDF, which this
// story's review also strengthened — Major 4/18) before the
// toolchain-gated hash comparison, so a machine on a different toolchain
// still observes the document's self-consistency instead of nothing.
func TestRenderMatchesGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	fixturePath := filepath.Join(root, "fixtures", "minimal-rect", "expected.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture expectedFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture JSON: %v", err)
	}

	// Vacuity guard: the fixture must carry both version fields before
	// any hash comparison runs, and the hash field must be present too.
	// This does NOT require fixture.Folio8GoVersion == Version: AC11
	// requires the fixture to *record* the version, not to match the
	// module's current one, and asserting equality would fail the golden
	// test on the very first legitimate version bump even though the
	// rendered bytes are unchanged — training the "just regenerate it"
	// reflex vacuity guard 4 exists to suppress, on a false positive
	// (this story's QA review, Major 11).
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain (RP-8: this must fail before the hash comparison runs)")
	}
	if fixture.SHA256 == "" {
		t.Fatal("fixture is missing sha256")
	}
	// AC16: sha256 must be a JSON string of exactly 64 lower-case hex
	// characters — never a map, array, or per-target object. A developer
	// meeting a red matrix arm (Story 1.2) who starts "fixing" it by
	// converting this field into a per-target map must hit a red here
	// before they can reach a green (D-1.2.2's escalation rule: a real
	// cross-target divergence is an owner decision, never a developer's
	// to paper over by recording what each target produced).
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not a JSON string of exactly 64 lower-case hex characters "+
			"(AC16: never a per-target map, array, or object — see D-1.2.2)", fixture.SHA256)
	}

	// Serialize and validate structurally first, unconditionally — see
	// the ordering note above (Major 5).
	//
	// AC14a, D-1.5.9: this fixture is RETAINED UNCHANGED and
	// RECLASSIFIED as pinning internal/pdf's fontless emission path —
	// AD-3 number emission, xref arithmetic, content-derived /ID — not
	// the public Render API, which D-1.1.c designed to change every
	// story and which now requires a non-nil template (AC14b). No
	// `.folio` template can reproduce these exact 547 bytes (F-8: the
	// golden's content stream has no colour operator, and any
	// template-authored rectangle's style.background emits one via
	// D-1.1.b's colour rule) — so this test calls internal/pdf.Serialize()
	// directly, exactly as it did before Render accepted a template at
	// all.
	b := pdf.Serialize()
	assertWellFormedPDF(t, "golden fixture render", b, 1)

	// expected.pdf is kept for human diffing, but nothing previously read
	// it, so it could silently drift from the normative hash in
	// expected.json without any test noticing (this story's QA review,
	// Minor 19).
	expectedPDFPath := filepath.Join(root, "fixtures", "minimal-rect", "expected.pdf")
	expectedPDFBytes, err := os.ReadFile(expectedPDFPath)
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	expectedPDFSum := sha256.Sum256(expectedPDFBytes)
	expectedPDFHex := hex.EncodeToString(expectedPDFSum[:])
	if expectedPDFHex != fixture.SHA256 {
		t.Fatalf(
			"fixtures/minimal-rect/expected.pdf's own sha256 (%s) does not match "+
				"expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			expectedPDFHex, fixture.SHA256,
		)
	}

	// A toolchain bump is a versioned breaking change under AD-22 (C6):
	// fail loudly and instructively rather than silently comparing
	// against a hash measured under a different compiler. This gate now
	// runs after the structural checks above, not before Render() is
	// even called (Major 5).
	if runtime.Version() != fixture.GoToolchain {
		t.Fatalf(
			"toolchain mismatch: running under %s, fixture was recorded under %s. "+
				"A Go toolchain bump is a versioned breaking change under AD-22 — "+
				"the golden hash must be re-measured deliberately, not regenerated "+
				"automatically, and the change must be recorded as an intended, "+
				"versioned event (C6).",
			runtime.Version(), fixture.GoToolchain,
		)
	}

	sum := sha256.Sum256(b)
	gotHex := hex.EncodeToString(sum[:])

	if gotHex != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/minimal-rect). "+
				"Under AD-21/AD-22 this is a defect until proven to be an intended, "+
				"versioned change — see fixtures/minimal-rect/README.md. Do not "+
				"regenerate the fixture to make this pass.",
			gotHex, fixture.SHA256,
		)
	}
}

// TestRenderMatchesFontTextGoldenFixture is AC14: fixtures/font-text/,
// the NEW golden fixture beside fixtures/minimal-rect/ (AC14a: that one
// is retained unchanged, never replaced). Same shape and same rules as
// TestRenderMatchesGoldenFixture above, applied to the font-embedding
// path through the public Render API instead of internal/pdf.Serialize()
// directly.
func TestRenderMatchesFontTextGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	fixturePath := filepath.Join(root, "fixtures", "font-text", "expected.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture expectedFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture JSON: %v", err)
	}

	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain (RP-8: this must fail before the hash comparison runs)")
	}
	if fixture.SHA256 == "" {
		t.Fatal("fixture is missing sha256")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not a JSON string of exactly 64 lower-case hex characters (AC16)", fixture.SHA256)
	}

	// Finding 14 (QA review): fixtures/font-text/input.folio's own README
	// stated it is "kept in sync by hand — the fixture is not read at
	// test time", with nothing asserting that claim. AD-21 makes the
	// fixture the normative record of what produced the hash; if
	// fontTestTemplateJSON were edited and input.folio were not, the
	// fixture would document a document that does not produce its own
	// recorded hash — a silently lying artifact. This costs nothing new:
	// the test already reads fixtures/ (out-of-module) under -count=1.
	inputFolio8Path := filepath.Join(root, "fixtures", "font-text", "input.folio")
	inputFolio8Bytes, err := os.ReadFile(inputFolio8Path)
	if err != nil {
		t.Fatalf("read %s: %v", inputFolio8Path, err)
	}
	if string(inputFolio8Bytes) != fontTestTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/fontTestTemplateJSON (render_test.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per the fixture's own "+
				"README); update input.folio to match, or this fixture no longer documents what "+
				"actually produced its recorded hash",
			inputFolio8Path,
		)
	}

	tpl := parseFontTestTemplate(t)
	res, err := Render(tpl, Data("{}"), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	b := res.Bytes
	assertWellFormedPDF(t, "font-text golden fixture render", b, 1)

	// AC10b's vacuity guard, applied here too: confirm the render
	// actually contains a FontFile2 before comparing any hash — a
	// fontless render matching a fontless golden would prove nothing
	// about font byte-stability (D-000.9).
	if !containsFontFile2(b) {
		t.Fatal("font-text golden fixture render does not contain a FontFile2 — the fixture would certify nothing (AC10b)")
	}

	// Story 2.2 (AC6/AC6a, D-2.2.2 (superseded)): the six-letter subset
	// tag re-derivation moved this fixture's whole-document hash (the
	// tag changed at its three sites, and the content-derived /ID moved
	// as a consequence — see fixtures/font-text/README.md's "Re-recorded
	// at Story 2.2" section for the measured, tag-only delta). AC8's
	// original "bytes unchanged" clause is corrected by this same
	// commit into a narrower, still-meaningful one: the EMBEDDED FONT
	// PROGRAM itself — independent of its tag, and independent of the
	// PDF's other content-derived bytes — must still be byte-identical.
	// Pinning this constant is what makes a FUTURE change that moves
	// the program (not just the tag) distinguishable from one that
	// doesn't, which the whole-document hash alone cannot tell apart.
	const wantFontTextProgramSHA256 = "2ef52cca3f6bdb76de8cf5a4c73ca3f7e0f9154bb7bb0b1122425127419db4de"
	programSum := sha256.Sum256(extractFontFile2Program(t, b))
	if gotProgram := hex.EncodeToString(programSum[:]); gotProgram != wantFontTextProgramSHA256 {
		t.Fatalf(
			"font-text's embedded FontFile2 PROGRAM bytes changed: got sha256 %s, want %s — "+
				"this is the assertion that replaced AC8's original whole-fixture \"bytes unchanged\" "+
				"clause (Story 2.2): the program itself must stay stable even though the fixture's "+
				"whole-document hash was deliberately re-recorded for the tag-derivation change",
			gotProgram, wantFontTextProgramSHA256,
		)
	}

	expectedPDFPath := filepath.Join(root, "fixtures", "font-text", "expected.pdf")
	expectedPDFBytes, err := os.ReadFile(expectedPDFPath)
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	expectedPDFSum := sha256.Sum256(expectedPDFBytes)
	expectedPDFHex := hex.EncodeToString(expectedPDFSum[:])
	if expectedPDFHex != fixture.SHA256 {
		t.Fatalf(
			"fixtures/font-text/expected.pdf's own sha256 (%s) does not match "+
				"expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			expectedPDFHex, fixture.SHA256,
		)
	}

	if runtime.Version() != fixture.GoToolchain {
		t.Fatalf(
			"toolchain mismatch: running under %s, fixture was recorded under %s. "+
				"A Go toolchain bump is a versioned breaking change under AD-22 — "+
				"the golden hash must be re-measured deliberately (C6).",
			runtime.Version(), fixture.GoToolchain,
		)
	}

	// ------------------------------------------------------------------
	// Story 2.3's SEMANTIC ACCEPTANCE STEP for this fixture's
	// re-recording (D-000.22, and a binding condition of the ruling that
	// authorised the re-record).
	//
	// This golden MOVED in Story 2.3, and it is a versioned change under
	// AD-21/AD-22, not a drift. The story's F3 recorded this fixture as
	// blind to shaping. It is not, and the reason is worth stating
	// because it is how a measurement misleads: F3 measured the string
	// "Hello" against a shipped Noto face, while this fixture renders
	// "Hello, World!" and "Page footer 0123456789" through
	// testdata/fonts/Roboto-Regular.ttf at unitsPerEm 2048 — a different
	// string AND a different face. Roboto's GPOS kerns two pairs in that
	// text, so the PREVIOUS bytes recorded UNKERNED output: two Tj
	// operators, no TJ, 22,299 bytes. Re-recording retires a regression
	// rather than accepting one.
	//
	// This fixture has therefore MOVED FROM the "blind to shaping" list
	// TO the OBSERVING list. The fixtures that genuinely cannot observe
	// this story are minimal-rect (fontless), image-embed (no text) and
	// multi-script-fallback (measured: "Ada ก 汉" shapes to itself on all
	// three faces — TestShapedTextFixtureObservabilityRedProof asserts
	// exactly that, and depends on it).
	//
	// Cross-checked against the REFERENCE implementation before
	// re-recording — hb-shape (HarfBuzz) 14.2.0, a one-time offline run,
	// never a build/test/runtime dependency (AD-25; the same precedent
	// Story 1.1 set with qpdf --check). Exact invocations, verbatim,
	// from the repository root:
	//
	//	hb-shape --no-glyph-names folio-go/testdata/fonts/Roboto-Regular.ttf "Hello, World!"
	//	  [44=0+1460|73=1+1085|80=2+497|80=3+497|83=4+1168|16=5+402|4=6+507|
	//	   59=7+1786|83=8+1168|86=9+693|80=10+497|72=11+1155|5=12+527]
	//
	//	hb-shape --no-glyph-names folio-go/testdata/fonts/Roboto-Regular.ttf "Page footer 0123456789"
	//	  [52=0+1281|69=1+1114|75=2+1149|73=3+1085|4=4+507|74=5+711|83=6+1168|
	//	   83=7+1168|88=8+669|73=9+1085|86=10+693|4=11+507|20=12+1150|...]
	//
	// HarfBuzz reports W (gid 59) at 1786 against its hmtx 1817 — minus
	// 31 font units — and P (gid 52) at 1281 against its hmtx 1292 —
	// minus 11. Scaled to the PDF's 1000-unit em by geom.ScaleRound,
	// which is where the content stream's numbers live: 887 - 872 = 15,
	// and 631 - 625 = 6.
	//
	// THE ADJUSTMENTS ARE ASSERTED BY VALUE, NOT BY HASH, and that is
	// the whole point of writing them out. A future change that silently
	// loses kerning produces a hash mismatch, and the cheapest available
	// response to a hash mismatch is to re-record it. Naming the expected
	// adjustments makes re-recording a kerning regression impossible
	// without DELETING AN ASSERTION — which is visible in a diff, where a
	// changed digest is not.
	if !bytes.Contains(b, []byte("] TJ\n")) {
		t.Fatal(
			"fixtures/font-text/ emits no TJ array at all. Since Story 2.3 this document's text is KERNED " +
				"and must be shown with a TJ array carrying the adjustments; Tj-only output is the unkerned " +
				"pre-2.3 behaviour this fixture was re-recorded to retire (AC6, D-000.22).",
		)
	}
	if bytes.Contains(b, []byte("> Tj\n")) {
		t.Fatal(
			"fixtures/font-text/ still emits a bare Tj operator. Both of this document's runs carry a GPOS " +
				"kern, so both must be TJ arrays — a Tj here means one run lost its adjustment.",
		)
	}
	for _, want := range []struct {
		adjustment string
		why        string
	}{
		{
			">15<",
			`the "Wo" kern in "Hello, World!": W (gid 59) has hmtx 1817 against a shaped advance of 1786, ` +
				`i.e. -31 font units at unitsPerEm 2048, which is 887 - 872 = 15 in the PDF's 1000-unit em`,
		},
		{
			">6<",
			`the "Pa" kern in "Page footer 0123456789": P (gid 52) has hmtx 1292 against a shaped advance ` +
				`of 1281, i.e. -11 font units, which is 631 - 625 = 6`,
		},
	} {
		if !bytes.Contains(b, []byte(want.adjustment)) {
			t.Errorf(
				"fixtures/font-text/'s content stream does not carry the TJ adjustment %q — %s. "+
					"If the kerning has genuinely changed, that is a finding to investigate; do NOT delete "+
					"this assertion to make a re-record pass (D-000.22).",
				want.adjustment, want.why,
			)
		}
	}

	sum := sha256.Sum256(b)
	gotHex := hex.EncodeToString(sum[:])
	if gotHex != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/font-text). "+
				"Under AD-21/AD-22 this is a defect until proven to be an intended, "+
				"versioned change — see fixtures/font-text/README.md.",
			gotHex, fixture.SHA256,
		)
	}
}

// TestRenderMatchesImageEmbedGoldenFixture is AC22/AC25 (D-1.8.6): the
// THIRD matrix document, joining minimal-rect and font-text — same
// shape as TestRenderMatchesFontTextGoldenFixture above, applied to the
// image-embedding path.
func TestRenderMatchesImageEmbedGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	fixturePath := filepath.Join(root, "fixtures", "image-embed", "expected.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture expectedFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture JSON: %v", err)
	}

	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain (RP-8: this must fail before the hash comparison runs)")
	}
	if fixture.SHA256 == "" {
		t.Fatal("fixture is missing sha256")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not a JSON string of exactly 64 lower-case hex characters (AC16)", fixture.SHA256)
	}

	// AC25a: input.folio is byte-identical to the Go constant that
	// renders it — the same obligation font-text/input.folio carries
	// (Finding 14, Story 1.5's QA review), inherited here.
	inputFolio8Path := filepath.Join(root, "fixtures", "image-embed", "input.folio")
	inputFolio8Bytes, err := os.ReadFile(inputFolio8Path)
	if err != nil {
		t.Fatalf("read %s: %v", inputFolio8Path, err)
	}
	if string(inputFolio8Bytes) != imageTestTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/imageTestTemplateJSON (render_test.go) — the two are "+
				"supposed to be byte-identical; update input.folio to match, or this fixture no "+
				"longer documents what actually produced its recorded hash",
			inputFolio8Path,
		)
	}

	tpl, err := ParseTemplate([]byte(imageTestTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	b := res.Bytes
	assertWellFormedPDF(t, "image-embed golden fixture render", b, 1)

	// AC23's vacuity guard, applied here too: confirm the render
	// actually contains an image XObject before comparing any hash — a
	// render that silently dropped the image would certify nothing
	// (D-000.9).
	if !containsImageXObject(b) {
		t.Fatal("image-embed golden fixture render does not contain an image XObject — the fixture would certify nothing (AC23)")
	}

	expectedPDFPath := filepath.Join(root, "fixtures", "image-embed", "expected.pdf")
	expectedPDFBytes, err := os.ReadFile(expectedPDFPath)
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	expectedPDFSum := sha256.Sum256(expectedPDFBytes)
	expectedPDFHex := hex.EncodeToString(expectedPDFSum[:])
	if expectedPDFHex != fixture.SHA256 {
		t.Fatalf(
			"fixtures/image-embed/expected.pdf's own sha256 (%s) does not match "+
				"expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			expectedPDFHex, fixture.SHA256,
		)
	}

	if runtime.Version() != fixture.GoToolchain {
		t.Fatalf(
			"toolchain mismatch: running under %s, fixture was recorded under %s. "+
				"A Go toolchain bump is a versioned breaking change under AD-22 — "+
				"the golden hash must be re-measured deliberately (C6).",
			runtime.Version(), fixture.GoToolchain,
		)
	}

	sum := sha256.Sum256(b)
	gotHex := hex.EncodeToString(sum[:])
	if gotHex != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/image-embed). "+
				"Under AD-21/AD-22 this is a defect until proven to be an intended, "+
				"versioned change — see fixtures/image-embed/README.md.",
			gotHex, fixture.SHA256,
		)
	}
}

// TestMultiScriptFallbackGoldenFixture is Story 2.2's AC8 fourth
// fixture: renders multiScriptTestTemplateJSON (fixtures/multi-script-
// fallback/input.folio) through the public Render path against the
// REAL shipped face set, and compares against the recorded golden —
// the same shape TestRenderMatchesFontTextGoldenFixture already
// established for font-text, extended with AC7's per-(face,pinned-
// instance) program digests (V7): the whole-document hash proves the
// DOCUMENT is stable; the three per-face digests below prove each
// enumerated PAIR individually is, which is what "a golden for EVERY
// (face, pinned instance) pair — not one sample" asks for.
func TestMultiScriptFallbackGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	fixturePath := filepath.Join(root, "fixtures", "multi-script-fallback", "expected.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture expectedFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture JSON: %v", err)
	}
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain (RP-8: this must fail before the hash comparison runs)")
	}
	if fixture.SHA256 == "" {
		t.Fatal("fixture is missing sha256")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not a JSON string of exactly 64 lower-case hex characters (AC16)", fixture.SHA256)
	}

	inputFolio8Path := filepath.Join(root, "fixtures", "multi-script-fallback", "input.folio")
	inputFolio8Bytes, err := os.ReadFile(inputFolio8Path)
	if err != nil {
		t.Fatalf("read %s: %v", inputFolio8Path, err)
	}
	if string(inputFolio8Bytes) != multiScriptTestTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/multiScriptTestTemplateJSON (render_test.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per font-text's precedent)",
			inputFolio8Path,
		)
	}

	tpl, err := ParseTemplate([]byte(multiScriptTestTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	b := res.Bytes
	assertWellFormedPDF(t, "multi-script-fallback golden fixture render", b, 1)

	if !containsFontFile2(b) {
		t.Fatal("multi-script-fallback golden fixture render does not contain a FontFile2 — the fixture would certify nothing (AC10b)")
	}
	programs := extractAllFontFile2Programs(t, b)
	// THE EXPECTED COUNT IS THIS FIXTURE'S OWN CHAIN LENGTH, READ FROM THE
	// PARSED TEMPLATE — NOT `len(shippedFaceSpecs)` (Story 16.8).
	//
	// Until Story 16.8 those two numbers were the same value for a reason
	// that was never stated: `multiScriptTestTemplateJSON`'s "body" chain
	// happened to name every face `fonts.Shipped()` carried, because Story
	// 2.2 built this fixture specifically to exercise all of them at once.
	// Story 16.8 ships a fourth face (Roboto) that this fixture's own
	// document does NOT declare — Roboto is an engine-shipped face meant
	// to need no embedding at all, and per this story's own boundary the
	// golden fixtures stay UNMOVED, so this document is not extended to
	// use it. `len(shippedFaceSpecs)` therefore stopped being this
	// fixture's own invariant the moment a shipped face could legitimately
	// go unused here. The count that IS still this fixture's own
	// invariant is its declared chain's length, read off the parsed
	// template rather than restated as a literal — so a chain edited here
	// without its golden being re-measured still reddens this line.
	wantEmbeddedCount := len(tpl.doc.Fonts["body"])
	if wantEmbeddedCount == 0 {
		t.Fatal("multiScriptTestTemplateJSON's \"body\" chain parsed to zero entries — this assertion would be vacuous")
	}
	if len(programs) != wantEmbeddedCount {
		t.Fatalf(
			"expected exactly %d embedded FontFile2 programs (one per face this fixture's own \"body\" chain names), got %d",
			wantEmbeddedCount, len(programs),
		)
	}

	// D-000.22's semantic acceptance step, on the EMBEDDED programs:
	// static, glyf, and Regular. This is the assertion whose absence let
	// Story 2.2 record a golden with Simplified Chinese at
	// usWeightClass=100 while four-target byte-identity, the program
	// digest and the missing-glyph diagnostic all reported success.
	assertEmbeddedProgramsAreStaticRegular(t, "multi-script-fallback golden", programs, 400)

	// ...and on the produced PDF: /BaseFont must carry the embedded
	// program's own PostScript name behind exactly one six-letter tag
	// (ISO 32000-1 Table 117), not the FontSet key.
	// SCOPED TO THIS FIXTURE'S OWN CHAIN, NOT EVERY SHIPPED FACE (Story
	// 16.8). assertBaseFontNames is a COVERAGE witness in both
	// directions — every name in `want` must actually appear — so
	// building it from all of `shippedFaceSpecs` would demand Roboto's
	// PostScript name show up in a PDF this fixture never asks the
	// engine to embed Roboto into. `wantEmbeddedFaces` is the same
	// chain-derived set `wantEmbeddedCount` above is the length of.
	wantEmbeddedFaces := map[string]bool{}
	for _, entry := range tpl.doc.Fonts["body"] {
		wantEmbeddedFaces[entry.Face] = true
	}
	wantPS := map[string]bool{}
	for _, spec := range shippedFaceSpecs {
		if wantEmbeddedFaces[spec.Key] {
			wantPS[spec.PostScriptName] = true
		}
	}
	assertBaseFontNames(t, "multi-script-fallback golden", b, wantPS)

	// V5a's exclusivity half: every occurrence of each subset tag in
	// the whole PDF must sit in a name position.
	assertSubsetTagsAppearOnlyInNames(t, "multi-script-fallback golden", b)

	// AC7's per-(face, instance) goldens (V7): one per shipped face.
	// Object-number order in the rendered PDF is Noto Sans, Noto Sans
	// SC, Noto Sans Thai (measured, matches alphabetical face-name
	// sort — render.go's faceNames := slices.Sorted(maps.Keys(...))).
	//
	// RE-RECORDED by Story 2.2's finisher, and this is a versioned
	// change under AD-22, not a drift. The shipped faces are no longer
	// the upstream VARIABLE builds instanced at render time to their axis
	// defaults; they are static Regular instances derived ahead of the
	// build (D-2.2.4). The old Noto Sans SC program here recorded
	// OS/2.usWeightClass = 100 — Thin — because that face's `wght` axis
	// defaults to 100 and "pin every axis to its default" faithfully
	// produced it. Every guard agreed the artifact was correct; none was
	// asked whether the value meant what its name implied (D-000.21).
	// assertEmbeddedProgramsAreStaticRegular above is the assertion that
	// was missing, and it now runs before these hashes are compared.
	wantProgramSHA256 := []string{
		"51620d3ae7f511c3b537439f40737d277907c44b0b2cc9d643529daffcd9ccd6", // Noto Sans
		"4a2c1e286e0628124e90931afd0b017f16eea3fd38b5c9ff978d3d82315a495d", // Noto Sans SC
		"beca96084cf1277a6b234727a373c66f8fbc2671a189cf6cea387e7b17702a28", // Noto Sans Thai
	}
	faceLabels := []string{"Noto Sans", "Noto Sans SC", "Noto Sans Thai"}

	// V7's guard, REBUILT AT STORY 2.2, AND NARROWED AT STORY 16.8.
	//
	// It previously read
	//
	//     if len(wantProgramSHA256) != 3 { ... }
	//
	// which compares a slice literal's length to a hard-coded 3 declared
	// six lines above it — unable to detect the hazard V7 is named for
	// (red-proved open: adding a fourth face to fonts.Shipped() left the
	// whole root package green, with four shipped pairs against three
	// goldens) — so Story 2.2 asserted the count against `len(shippedFaceSpecs)`
	// instead, THE ENUMERATION rather than a number narrated beside it.
	//
	// THAT FORM ASSUMED "every shipped face appears in THIS fixture",
	// which was true only because Story 2.2 built this document
	// specifically to exercise all of them at once — an assumption Story
	// 16.8 breaks on purpose: Roboto ships as a fourth face that is
	// engine-only and never embedded in any document (the whole point of
	// an engine-shipped face), so it is legitimately absent from every
	// per-instance PDF-embedding golden, this one included, forever. The
	// comparator is `wantEmbeddedCount` (this fixture's own chain length,
	// derived above) rather than `len(shippedFaceSpecs)` for the same
	// reason the earlier fix moved off the bare literal 3: it must answer
	// to something that would actually change if this ONE document's
	// declared chain did, not to a number that changes for a reason
	// unrelated to this fixture.
	//
	// V7's ORIGINAL HAZARD — a face shipping with NO weight/name/
	// staticness assertion anywhere — is still closed for Roboto, just by
	// a different guard: TestShippedFacesMatchSpec (shipped_faces_test.go)
	// runs assertShippedFaceMatchesSpec against EVERY shippedFaceSpecs row
	// including Roboto's, off the committed source file, and
	// fonts_test.go's TestShippedRobotoMatchesDesignerCatalogue ties that
	// same source file's bytes to the designer catalogue's. What is NOT
	// covered, stated rather than hidden, is a per-instance PDF-embedded-
	// program digest for Roboto — because there is no document, golden or
	// otherwise, that ever asks the engine to embed it.
	if len(wantProgramSHA256) != wantEmbeddedCount {
		t.Fatalf(
			"AC7/V7: %d per-instance goldens are recorded, but this fixture's own chain embeds %d (face, "+
				"pinned instance) pairs. The matrix would be monitoring a subset of the risk surface and "+
				"reporting green over the uncovered ones — D-2.2.1's exact hazard.",
			len(wantProgramSHA256), wantEmbeddedCount,
		)
	}
	if len(faceLabels) != len(wantProgramSHA256) {
		t.Fatalf("test bug: %d face labels for %d goldens", len(faceLabels), len(wantProgramSHA256))
	}
	for i, prog := range programs {
		sum := sha256.Sum256(prog)
		got := hex.EncodeToString(sum[:])
		if got != wantProgramSHA256[i] {
			t.Fatalf(
				"%s's embedded, instanced program changed: got sha256 %s, want %s (AC7 per-instance golden). "+
					"Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change.",
				faceLabels[i], got, wantProgramSHA256[i],
			)
		}
	}

	expectedPDFPath := filepath.Join(root, "fixtures", "multi-script-fallback", "expected.pdf")
	if expectedPDFBytes, err := os.ReadFile(expectedPDFPath); err == nil {
		expectedPDFSum := sha256.Sum256(expectedPDFBytes)
		expectedPDFHex := hex.EncodeToString(expectedPDFSum[:])
		if expectedPDFHex != fixture.SHA256 {
			t.Fatalf(
				"fixtures/multi-script-fallback/expected.pdf's own sha256 (%s) does not match "+
					"expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
				expectedPDFHex, fixture.SHA256,
			)
		}
	}

	if runtime.Version() != fixture.GoToolchain {
		t.Fatalf(
			"toolchain mismatch: running under %s, fixture was recorded under %s. "+
				"A Go toolchain bump is a versioned breaking change under AD-22 — "+
				"the golden hash must be re-measured deliberately (C6).",
			runtime.Version(), fixture.GoToolchain,
		)
	}

	sum := sha256.Sum256(b)
	gotHex := hex.EncodeToString(sum[:])
	if gotHex != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/multi-script-fallback). "+
				"Under AD-21/AD-22 this is a defect until proven to be an intended, "+
				"versioned change — see fixtures/multi-script-fallback/README.md.",
			gotHex, fixture.SHA256,
		)
	}
}

// TestRenderMatchesComponentAssetImportGoldenFixture is Story 5.13's
// golden fixture, part (a): the same shape as
// TestRenderMatchesImageEmbedGoldenFixture, applied to
// fixtures/component-asset-import/ — but unlike image-embed, this
// fixture's input.folio is not merely a document that names an asset; it
// is the CAPTURED CANONICAL OUTPUT of one real setComponentAsset command
// (see part (b), TestComponentAssetImportCommandReproducesTheFixtureInput,
// below — part (a) alone would make this indistinguishable from a second
// image-embed).
func TestRenderMatchesComponentAssetImportGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	fixturePath := filepath.Join(root, "fixtures", "component-asset-import", "expected.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture expectedFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture JSON: %v", err)
	}

	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain (RP-8: this must fail before the hash comparison runs)")
	}
	if fixture.SHA256 == "" {
		t.Fatal("fixture is missing sha256")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not a JSON string of exactly 64 lower-case hex characters (AC16)", fixture.SHA256)
	}

	// input.folio is byte-identical to the Go constant that renders it —
	// the same obligation every other fixture's input.folio carries.
	inputFolio8Path := filepath.Join(root, "fixtures", "component-asset-import", "input.folio")
	inputFolio8Bytes, err := os.ReadFile(inputFolio8Path)
	if err != nil {
		t.Fatalf("read %s: %v", inputFolio8Path, err)
	}
	if string(inputFolio8Bytes) != componentAssetImportTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/componentAssetImportTemplateJSON (render_test.go) — "+
				"the two are supposed to be byte-identical; this fixture no longer documents what "+
				"actually produced its recorded hash",
			inputFolio8Path,
		)
	}

	tpl, err := ParseTemplate([]byte(componentAssetImportTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	b := res.Bytes
	assertWellFormedPDF(t, "component-asset-import golden fixture render", b, 1)

	// Vacuity guard, same shape as image-embed's: confirm the render
	// actually contains an image XObject before comparing any hash — a
	// render that silently dropped the image would certify nothing.
	if !containsImageXObject(b) {
		t.Fatal("component-asset-import golden fixture render does not contain an image XObject — the fixture would certify nothing")
	}

	expectedPDFPath := filepath.Join(root, "fixtures", "component-asset-import", "expected.pdf")
	expectedPDFBytes, err := os.ReadFile(expectedPDFPath)
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	expectedPDFSum := sha256.Sum256(expectedPDFBytes)
	expectedPDFHex := hex.EncodeToString(expectedPDFSum[:])
	if expectedPDFHex != fixture.SHA256 {
		t.Fatalf(
			"fixtures/component-asset-import/expected.pdf's own sha256 (%s) does not match "+
				"expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			expectedPDFHex, fixture.SHA256,
		)
	}

	if runtime.Version() != fixture.GoToolchain {
		t.Fatalf(
			"toolchain mismatch: running under %s, fixture was recorded under %s. "+
				"A Go toolchain bump is a versioned breaking change under AD-22 — "+
				"the golden hash must be re-measured deliberately (C6).",
			runtime.Version(), fixture.GoToolchain,
		)
	}

	sum := sha256.Sum256(b)
	gotHex := hex.EncodeToString(sum[:])
	if gotHex != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/component-asset-import). "+
				"Under AD-21/AD-22 this is a defect until proven to be an intended, "+
				"versioned change — see fixtures/component-asset-import/README.md.",
			gotHex, fixture.SHA256,
		)
	}
}

// TestComponentAssetImportCommandReproducesTheFixtureInput is Story
// 5.13's golden fixture, part (b) — the part that makes this a 5.13
// fixture rather than a second image-embed. It re-runs the REAL
// setComponentAsset command (component_commands.go, via the public
// ApplyComponentCommand entry point, not a hand-rolled equivalent)
// against the same starting document (componentAssetImportBaseTemplateJSON,
// render_test.go) and the same source image bytes (png1x1Gray(),
// component_asset_command_test.go) used to generate this fixture, and
// asserts the resulting canonical bytes are byte-for-byte identical to
// fixtures/component-asset-import/input.folio.
//
// This is what pins the AUTHORING COMMAND's canonical-bytes-producing
// behaviour (AD-9: digest-as-key, 76-column wrap, sorted keys,
// insert-if-absent, repoint-with-orphan-collection) rather than merely
// pinning a render of a document that already names an asset. It
// red-proofs cleanly: change the base64 wrap width or the digest-as-key
// derivation, and this test fails while TestRenderMatchesComponentAssetImportGoldenFixture
// above might not (that test only observes the rendered PDF, not the
// canonical `.folio` bytes the command produced). Attribution note
// (Finding 20, review of 2026-08-29): mutating the digest-as-key
// derivation specifically reddens through ApplyComponentCommand's own
// reparse hitting decodeAssets's PRE-EXISTING digest-match enforcement
// (parse.go) — a real catch, but a mechanism Story 1.8 already shipped,
// not new coverage this fixture adds.
//
// Finding 7 (review of 2026-08-29): the base document carries a SECOND
// image element (e2) on a DIFFERENT asset that the command never touches
// — componentAssetImportBaseTemplateJSON's own doc comment explains why.
// Without it, the command's own orphan-collection of e1's old asset left
// exactly ONE surviving asset, and a one-entry map has no ordering to get
// wrong: reversing the sort in serialize.go passed this test and the
// entire `go test ./...` suite. With two surviving assets under two
// different keys, that same mutation now reorders committed bytes this
// test actually compares.
func TestComponentAssetImportCommandReproducesTheFixtureInput(t *testing.T) {
	root := repoRootFromTest(t)
	inputFolio8Path := filepath.Join(root, "fixtures", "component-asset-import", "input.folio")
	want, err := os.ReadFile(inputFolio8Path)
	if err != nil {
		t.Fatalf("read %s: %v", inputFolio8Path, err)
	}

	tpl, err := ParseTemplate([]byte(componentAssetImportBaseTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate(componentAssetImportBaseTemplateJSON): %v", err)
	}

	command := setAssetCommand("e1", "image/png", png1x1Gray())
	if _, err := applyComponentCommand(tpl, command); err != nil {
		t.Fatalf("ApplyComponentCommand(setComponentAsset): %v", err)
	}

	// Finding 7's own precondition: this fixture proves nothing about key
	// ordering unless two DIFFERENT assets actually survive the command.
	if len(tpl.doc.Assets) != 2 {
		t.Fatalf("expected exactly 2 surviving assets (e1's new one, e2's untouched one), got %d: %#v", len(tpl.doc.Assets), tpl.doc.Assets)
	}

	got, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("SerializeTemplate: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf(
			"running the real setComponentAsset command against componentAssetImportBaseTemplateJSON + "+
				"png1x1Gray() no longer reproduces fixtures/component-asset-import/input.folio "+
				"byte-for-byte — the authoring command's canonical output has drifted from the "+
				"committed golden bytes (AD-9). This is a defect until proven to be an intended, "+
				"versioned change — see fixtures/component-asset-import/README.md.\ngot:\n%s\nwant:\n%s",
			got, want,
		)
	}

	if string(got) != componentAssetImportTemplateJSON {
		t.Fatalf(
			"the command's output also disagrees with folio-go/componentAssetImportTemplateJSON " +
				"(render_test.go) — that constant and fixtures/component-asset-import/input.folio " +
				"were supposed to be kept byte-identical",
		)
	}
}
