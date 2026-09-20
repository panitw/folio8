package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// THE DEFECT THIS FILE IS ABOUT, AND THE TWO HALVES OF ITS PROOF
// (spec-deferred-offline-cache, CAP-6).
//
// The designer's engine wasm is built `-tags nocjkface` and does not embed
// Noto Sans SC. The vertical model is a MAXIMUM over the line metrics of a
// chain's PRESENT faces (chainLineMetrics feeding verticalModel), so
// dropping the face changes the layout of every element whose chain NAMES
// it — including elements containing no CJK codepoint at all. Measured,
// before the fix: an English paragraph on the starter's own chain rendered
// to two different PDFs, 15,985 bytes against 15,986.
//
// The fix is that the tagged build carries the face's three hhea integers
// (internal/fontset's DeclaredLineMetrics) in place of its 10,595,932
// bytes, so the arithmetic is identical and nothing is fetched. A Latin
// session therefore fetches nothing at all, which is this story's own
// matrix row 1.
//
// ⚠ THE PROOF IS A PAIR, AND BOTH HALVES LIVE BESIDE THIS FILE.
//
//   - declared_metrics_identity_test.go (//go:build nocjkface) asserts the
//     PDFs are IDENTICAL. That is the property.
//   - declared_metrics_divergence_test.go (untagged) asserts they DIFFER.
//     That is what makes the first half non-vacuous: without it, a change
//     that quietly made every chain member optional — or a corpus that
//     stopped containing a CJK-tailed document — would leave the first half
//     green while proving nothing. It is ALSO the guard that folio-js,
//     folio-dotnet, the CLI and every golden fixture are untouched: an
//     unconditional metrics table would red there first.
//
// ⚠ THE CORPUS IS DISCOVERED, NOT LISTED. Every committed fixture that
// renders is measured, and the classification — "byte-identical" against
// "refuses for want of coverage" — is taken from the run rather than from
// a table somebody has to remember to update. The point of a range this
// wide is that line metrics are the difference this fix ADDRESSES, and
// whole-PDF comparison over tables, multi-page flows, Thai shaping,
// justified text, embedded faces, images and barcodes is what would
// surface a SECOND difference a missing chain member makes, if there is
// one.

// twoLineLatinParagraph is the document whose divergence was measured and
// reported: English text, a chain whose tail is the CJK face, and no CJK
// codepoint anywhere in it. It is inline rather than a fixture because it
// is evidence about a specific reported measurement and must not drift.
const twoLineLatinParagraph = `{
  "assets": {},
  "bands": {
    "content": {"elements": [{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 60, "value": "Hello world, this is a Latin paragraph that will wrap over a couple of lines.", "style": {"fontFamily": "body", "fontSize": 12}}]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto", "Noto Sans Thai", "Noto Sans SC"]},
  "locale": "en", "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00", "version": "1.0"
}`

// thaiOnCJKChainDoc: Thai shaping — mark stacking and dictionary
// breaking — on the same CJK-tailed chain. Thai's own descent (-450) is
// deeper than the CJK face's (-288) while its ascent is shallower, so this
// row exercises a chain where the two absent-face axes disagree about
// which face wins.
const thaiOnCJKChainDoc = `{
  "assets": {},
  "bands": {
    "content": {"elements": [{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 300, "height": 80, "value": "ภาษาไทยเป็นภาษาราชการของประเทศไทยและมีเครื่องหมายวรรณยุกต์", "style": {"fontFamily": "body", "fontSize": 14}}]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto", "Noto Sans Thai", "Noto Sans SC"]},
  "locale": "th", "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00", "version": "1.0"
}`

// tableOnCJKChainDoc: a bound table on the same chain. A table's header
// labels and its rows are both laid out through it, and the row height is
// where a leading difference compounds page by page.
const tableOnCJKChainDoc = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 28,
       "style": {"fontFamily": "body", "fontSize": 10, "padding": {"bottom": 4, "left": 3, "right": 3, "top": 4}},
       "columns": [
         {"id": "e2", "label": "Description", "width": 240, "align": "left", "bind": "{{row.text}}"},
         {"id": "e3", "label": "Amount", "width": 100, "align": "right", "bind": "{{row.text}}"}
       ]}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto", "Noto Sans Thai", "Noto Sans SC"]},
  "locale": "en", "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00", "version": "1.0"
}`

const tableRowsData = `{"rows": [{"text": "First row of a table on a CJK-tailed chain"}, {"text": "Second row, long enough to wrap inside its own column"}, {"text": "Third"}]}`

type parityCase struct {
	name     string
	template []byte
	data     []byte
	params   Params
}

// inlineParityCases are authored here because each one states a specific
// claim about the chain under test; the discovered fixtures below widen
// the range around them.
func inlineParityCases() []parityCase {
	return []parityCase{
		{name: "inline/two-line-latin-paragraph", template: []byte(twoLineLatinParagraph), data: []byte(`{}`)},
		{name: "inline/thai-on-cjk-chain", template: []byte(thaiOnCJKChainDoc), data: []byte(`{}`)},
		{name: "inline/table-on-cjk-chain", template: []byte(tableOnCJKChainDoc), data: []byte(tableRowsData)},
	}
}

// discoveredParityCases is every committed fixture that renders at all
// with the whole shipped set. A fixture needing data or params this
// harness does not have simply does not appear — it is skipped by its
// ELEVEN-face render failing, which is a statement about the fixture's
// inputs and not about fonts.
func discoveredParityCases(t *testing.T) []parityCase {
	t.Helper()
	root := filepath.Join("..", "fixtures")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	cases := []parityCase{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		template, err := os.ReadFile(filepath.Join(dir, "input.folio"))
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, "data.json"))
		if err != nil {
			data = []byte(`{}`)
		}
		var params Params
		if raw, err := os.ReadFile(filepath.Join(dir, "params.json")); err == nil {
			params = Params(raw)
		}
		cases = append(cases, parityCase{name: "fixture/" + entry.Name(), template: template, data: data, params: params})
	}
	if len(cases) == 0 {
		t.Fatalf("found no fixture under %s, so this corpus would be the inline cases alone", root)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].name < cases[j].name })
	return cases
}

func renderParityCase(t *testing.T, subject parityCase, fs FontSet) ([]byte, error) {
	t.Helper()
	tpl, err := ParseTemplate(subject.template)
	if err != nil {
		t.Fatalf("%s: a corpus document must parse: %v", subject.name, err)
	}
	result, err := Render(tpl, Data(subject.data), subject.params, fs)
	if err != nil {
		return nil, err
	}
	return result.Bytes, nil
}

// tenFaceSet is the designer's own set: the shipped eleven minus the CJK
// face, built by SUBTRACTION so a face added later is covered here without
// an edit, and checked so it cannot silently withhold nothing.
//
// ⚠ testShippedFontSet EMBEDS ITS FACES IN THE TEST BINARY and carries no
// build constraint, so it is the same eleven under both builds. That is
// what lets one comparison be run in each and mean opposite things: the
// only variable between the two runs is whether DeclaredLineMetrics
// answers.
func tenFaceSet(t *testing.T) FontSet {
	t.Helper()
	fs := testShippedFontSet()
	if _, ok := fs["Noto Sans SC"]; !ok {
		t.Fatal(`the test font set does not carry "Noto Sans SC", so removing it withholds nothing`)
	}
	delete(fs, "Noto Sans SC")
	if len(fs) != 10 {
		t.Fatalf("expected ten faces after removing the CJK one, got %d", len(fs))
	}
	return fs
}

func parityDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// parityOutcome is what one run of the corpus measured. It asserts
// nothing about identical/different: the two halves of the proof want
// opposite answers from the same measurement, and both want `refused` to
// be exactly the documents that DRAW a rune only the absent face covers.
type parityOutcome struct {
	identical []string
	different []string
	refused   []string
	skipped   []string
}

// compareTenAgainstEleven renders the whole corpus both ways.
//
// A TEN-FACE REFUSAL IS A RESULT, NOT A FAILURE, and it is checked on the
// spot: it must be TEXT_FACE_ABSENT, because declared metrics supply
// LEADING and never COVERAGE — a rune that must actually be drawn with the
// missing face still refuses, named, which is precisely the signal the
// designer acts on to fetch it. Any other error is a fault.
func compareTenAgainstEleven(t *testing.T) parityOutcome {
	t.Helper()
	eleven := testShippedFontSet()
	ten := tenFaceSet(t)
	out := parityOutcome{}
	for _, subject := range append(inlineParityCases(), discoveredParityCases(t)...) {
		want, err := renderParityCase(t, subject, eleven)
		if err != nil {
			if strings.HasPrefix(subject.name, "inline/") {
				t.Fatalf("%s: the ELEVEN-face render failed, so there is nothing to compare against: %v", subject.name, err)
			}
			out.skipped = append(out.skipped, subject.name)
			continue
		}
		if len(want) == 0 {
			t.Fatalf("%s: the eleven-face render produced no PDF bytes, so the comparison would be vacuous", subject.name)
		}
		got, err := renderParityCase(t, subject, ten)
		if err != nil {
			var refusal *RenderError
			if !errors.As(err, &refusal) || refusal.Diagnostic.Code != DiagCodeTextFaceAbsent {
				t.Fatalf("%s: the ten-face render failed for something other than an absent face: %v", subject.name, err)
			}
			out.refused = append(out.refused, subject.name)
			continue
		}
		if bytes.Equal(got, want) {
			out.identical = append(out.identical, subject.name)
			continue
		}
		out.different = append(out.different, subject.name)
		t.Logf("%s: ten-face %s (%d bytes), eleven-face %s (%d bytes)", subject.name, parityDigest(got), len(got), parityDigest(want), len(want))
	}
	return out
}

// TestCJKTextStillRefusesInATenFaceEngine is the boundary of the fix,
// asserted in BOTH builds: declared metrics supply LEADING and never
// COVERAGE.
//
// A document that must actually DRAW a Han rune refuses, named — which is
// when the designer fetches the face. If the metrics ever started standing
// in for the face itself this would go green in a way that shipped a PDF
// with the CJK text silently missing, which is the outcome CAP-7 exists to
// prevent.
//
// ⚠ IT IS NOT DRIVEN BY A LIST OF FIXTURE NAMES. `compareTenAgainstEleven`
// already classifies every corpus document by what the run did, and both
// halves of the proof call it; this test asserts that the refusing set is
// non-empty and that each of its members is a CJK-drawing document —
// a refusal for any other reason has already failed inside the helper.
func TestCJKTextStillRefusesInATenFaceEngine(t *testing.T) {
	outcome := compareTenAgainstEleven(t)
	if len(outcome.refused) == 0 {
		t.Fatal("no corpus document refused in a ten-face engine, so nothing here proves that declared metrics stop short of coverage; the corpus no longer contains a document that DRAWS a rune only the CJK face covers")
	}
	t.Logf("refused for want of coverage (%d): %v", len(outcome.refused), outcome.refused)
	t.Logf("skipped, their eleven-face render needing inputs this harness has not got (%d): %v", len(outcome.skipped), outcome.skipped)
}
