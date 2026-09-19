package template

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestWorkedExampleMatchesGoldenFixture is AC16 step 3: a test asserts
// the document's fenced worked-example block is byte-identical to the
// shipped golden fixture, closing the self-ratification loop
// permanently. It extracts the fence from the live document — never
// embeds a copy (D-1.4.7's forbidden third-artifact shape).
func TestWorkedExampleMatchesGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	docBytes := mustReadFile(t, filepath.Join(root, "_bmad-output", "specs", "spec-folio", "folio-format.md"))
	golden := mustReadFile(t, filepath.Join(root, "folio-go", "testdata", "template", "golden", "worked-example.json"))

	fence := extractWorkedExampleFence(t, string(docBytes))
	if fence != string(golden) {
		t.Fatalf("folio-format.md's worked example fence is no longer byte-identical to the golden fixture:\n--- doc ---\n%s\n--- golden ---\n%s", fence, golden)
	}
}

// extractWorkedExampleFence returns the content of the LAST ```json
// fence in the document (the worked example is the final fenced block,
// under "## Worked example").
func extractWorkedExampleFence(t *testing.T, doc string) string {
	t.Helper()
	idx := strings.Index(doc, "## Worked example")
	if idx < 0 {
		t.Fatal("could not find \"## Worked example\" heading in folio-format.md")
	}
	rest := doc[idx:]
	start := strings.Index(rest, "```json\n")
	if start < 0 {
		t.Fatal("could not find a ```json fence under the worked example heading")
	}
	rest = rest[start+len("```json\n"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		t.Fatal("worked example fence was never closed")
	}
	return rest[:end]
}

// TestAssetExampleMatchesGoldenFragment is AC27b (D-1.8.7, citation
// corrected: this extends TestWorkedExampleMatchesGoldenFixture's
// mechanism above — D-1.4.10's AC16 step 3, NOT D-1.4.11 as D-1.8.7's
// own text misattributed; D-1.8.7's obligation itself stands unchanged,
// only the ruling id was wrong). It closes the format doc's asset
// example permanently, the same way the worked example was closed:
// extract the fence from the LIVE document, compare against a golden
// fragment, never embed a second copy.
//
// A SECOND extractor, keyed to the "## `assets`" heading, not an edit
// to extractWorkedExampleFence above — the asset example is a
// DIFFERENT, EARLIER fence (M-2 of D-1.8.7's mechanism note): editing
// the existing extractor to also serve this fence would silently
// re-point the worked-example assertion instead of adding independent
// coverage. The two extractors' independence is itself asserted by
// RP-13: mutating only the asset fence must redden this test while
// TestWorkedExampleMatchesGoldenFixture stays green.
//
// The asset example is a FRAGMENT, not a whole `.folio` document (it
// cannot be parsed and round-tripped the way the worked example is) —
// AC27b requires picking one of two ways to close it and saying which:
// this compares it to a golden FRAGMENT emitted by the shipped assets
// writer (writeAssets, via SerializeDocument on a scratch Document
// carrying exactly this one asset, sliced down to the "assets": {...}
// object) — recorded once as testdata/template/golden/asset-example.json,
// never regenerated inline at test time (that would make the test
// trivially self-confirming).
func TestAssetExampleMatchesGoldenFragment(t *testing.T) {
	root := repoRootFromTest(t)
	docBytes := mustReadFile(t, filepath.Join(root, "_bmad-output", "specs", "spec-folio", "folio-format.md"))
	golden := mustReadFile(t, filepath.Join(root, "folio-go", "testdata", "template", "golden", "asset-example.json"))

	fence := extractAssetExampleFence(t, string(docBytes))
	if fence != string(golden) {
		t.Fatalf("folio-format.md's assets example fence is no longer byte-identical to the golden fragment (D-1.8.7):\n--- doc ---\n%s\n--- golden ---\n%s", fence, golden)
	}

	// Finding 10 (Story 1.8 review): the check above proves the doc and
	// the golden agree with EACH OTHER — it proves nothing about either
	// being canonical, correctly keyed, or a supported image. If a
	// future edit changed both consistently (the natural way anyone
	// "updates the example"), this test would stay green over a
	// non-canonical wrapping, a wrong digest, or an unsupported image
	// back in the spec. D-1.8.7's own words: "every fenced block
	// claiming to be complete or canonical should be EXECUTED, not
	// read." The block below executes the golden fragment using the
	// package's own real helpers, not a second, hand-rolled check.
	executeAssetExampleFragment(t, golden)
}

// executeAssetExampleFragment parses fragment — the literal
// `"assets": { "<key>": {"data": [...], "mediaType": "..."} }` text —
// and asserts, using this package's own real functions (never a
// reimplementation): the key is the SHA-256 of the decoded bytes
// (AC5/AC6), the wrapping is canonical at 76 columns (AC1,
// splitBase64Canonical reproduces the stored array exactly) and the
// declared mediaType actually recognises and accepts the decoded bytes
// (AC9/AC11 — decodeRecognisedImage returns recognised == true with no
// error).
func executeAssetExampleFragment(t *testing.T, fragment []byte) {
	t.Helper()
	var parsed struct {
		Assets map[string]struct {
			Data      []string `json:"data"`
			MediaType string   `json:"mediaType"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(append(append([]byte("{"), fragment...), '}'), &parsed); err != nil {
		t.Fatalf("golden fragment is not valid JSON once wrapped in braces: %v", err)
	}
	if len(parsed.Assets) != 1 {
		t.Fatalf("expected exactly one asset entry in the golden fragment, got %d", len(parsed.Assets))
	}
	for key, entry := range parsed.Assets {
		decoded, err := decodeBase64Asset(entry.Data)
		if err != nil {
			t.Fatalf("decodeBase64Asset on the golden fragment's data: %v", err)
		}
		if len(decoded) == 0 {
			t.Fatal("golden fragment decodes to zero bytes")
		}
		if wantKey := sha256HexOf(decoded); key != wantKey {
			t.Fatalf("golden fragment's key %q does not equal SHA-256(decoded) = %q (AC5/AC6)", key, wantKey)
		}
		if got := splitBase64Canonical(decoded); !slices.Equal(got, entry.Data) {
			t.Fatalf("golden fragment's data array %v is not the canonical 76-column split of its own decoded "+
				"bytes (AC1); splitBase64Canonical produced %v", entry.Data, got)
		}
		_, recognised, derr := decodeRecognisedImage(entry.MediaType, decoded)
		if !recognised {
			t.Fatalf("golden fragment's mediaType %q is not a RECOGNISED image type (AC9/AC11)", entry.MediaType)
		}
		if derr != nil {
			t.Fatalf("golden fragment's declared mediaType %q does not accept its own decoded bytes: %v", entry.MediaType, derr)
		}
	}
}

// extractAssetExampleFence returns the content of the FIRST ```json
// fence found under the "## `assets`" heading, bounded so it can never
// cross into the next "## " heading and accidentally return the wrong
// section's fence. "heading not found" and "fence never closed" both
// t.Fatal (D-000.9: a failure path distinguishable from a match — an
// empty string returned on failure would compare equal to an empty
// golden).
func extractAssetExampleFence(t *testing.T, doc string) string {
	t.Helper()
	idx := strings.Index(doc, "## `assets`")
	if idx < 0 {
		t.Fatal("could not find \"## `assets`\" heading in folio-format.md")
	}
	rest := doc[idx+len("## `assets`"):]

	// Bound the search to this section only: stop at the next "## "
	// heading, so a later section's fence can never be mistaken for
	// this one's.
	if nextHeading := strings.Index(rest, "\n## "); nextHeading >= 0 {
		rest = rest[:nextHeading]
	}

	start := strings.Index(rest, "```json\n")
	if start < 0 {
		t.Fatal("could not find a ```json fence under the \"## `assets`\" heading")
	}
	rest = rest[start+len("```json\n"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		t.Fatal("assets example fence was never closed")
	}
	return rest[:end]
}

// TestAssetExampleAndWorkedExampleExtractorsAreIndependent is RP-13:
// mutating ONLY the asset example's fence in a scratch copy of the
// document must redden extractAssetExampleFence's comparison while
// leaving extractWorkedExampleFence's comparison completely unaffected
// — proving the two extractors are independent (a second extractor,
// not an edit to the first, per AC27b).
func TestAssetExampleAndWorkedExampleExtractorsAreIndependent(t *testing.T) {
	root := repoRootFromTest(t)
	doc := string(mustReadFile(t, filepath.Join(root, "_bmad-output", "specs", "spec-folio", "folio-format.md")))
	golden := string(mustReadFile(t, filepath.Join(root, "folio-go", "testdata", "template", "golden", "asset-example.json")))
	workedGolden := string(mustReadFile(t, filepath.Join(root, "folio-go", "testdata", "template", "golden", "worked-example.json")))

	// Sanity: the real document currently matches both goldens (this is
	// the "control" per D-000.13 — a green baseline before the mutation).
	if extractAssetExampleFence(t, doc) != golden {
		t.Fatal("control failed: the real document's asset fence does not match its golden before mutation")
	}
	if extractWorkedExampleFence(t, doc) != workedGolden {
		t.Fatal("control failed: the real document's worked-example fence does not match its golden before mutation")
	}

	// RP-13 mutation: edit the asset example's fence WITHOUT
	// regenerating its golden — valid Markdown, forbidden semantics
	// (D-000.13).
	mutated := strings.Replace(doc, `"mediaType": "image/png"`, `"mediaType": "image/jpeg"`, 1)
	if mutated == doc {
		t.Fatal("test fixture assumption violated: the mutation string was not found in the document")
	}

	if extractAssetExampleFence(t, mutated) == golden {
		t.Fatal("RP-13: mutating the asset fence must redden the asset-example comparison")
	}
	if extractWorkedExampleFence(t, mutated) != workedGolden {
		t.Fatal("RP-13: the worked-example fence must be UNAFFECTED by an asset-fence-only mutation — the two extractors must be independent")
	}
}
