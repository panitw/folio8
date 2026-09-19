//go:build !(js && wasm)

package main

// TestCorpusManifest derives folio-js/test/data/go-corpus.json — the corpus
// conformance manifest both bindings drive their byte-identity suites from.
// CAP-5 asserts byte-identity across every supported runtime and platform, so
// the fixture list the bindings render has to be the WHOLE renderable corpus
// and it has to come from Go, not from a hand-kept array in each binding.
//
// What the manifest carries, and what it deliberately does not:
//
//   - every fixtures/ DIRECTORY is classified, included or excluded with a
//     stated reason. A directory that is neither fails this test, so a new
//     fixture cannot be silently uncovered;
//   - for each included fixture: the slug, whether data.json and params.json
//     are used, and the diagnostic sequence Go produces;
//   - NO GOLDEN DIGEST. TestGoldenDigestAgreesAtEveryDeclaredSite treats a
//     golden digest in an undeclared file as a defect, and expected.json is
//     meant to be the single source for every hash. Each binding therefore
//     reads the hash out of the fixture's own expected.json at test time, and
//     this file declares no new digest site.
//
// The generator verifies itself: rendering each included fixture with
// fonts.Shipped() must reproduce the hash its committed expected.json records,
// or it is not included and the failure names it. It also refuses to write a
// manifest with fewer fixtures than the one on disk already records, so the
// corpus cannot quietly shrink.
//
// Regeneration is explicit, exactly as the parity file's is:
//
//	FOLIO8_UPDATE_JS_CORPUS=1 go test ./wasm/cmd/render
//
// and a regeneration that would SHRINK the corpus needs a second, separate
// word — FOLIO8_CORPUS_MAY_SHRINK=1 — so a fixture leaving coverage cannot be
// an accident of running the usual command.
//
// It runs on the host, not under js/wasm, and uses only the public API.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// corpusExclusions is the declared reason each unrenderable fixture directory
// is out. Silence is not an option: a directory that is in neither this map
// nor the rendered set fails the test.
//
// The causes, not the names, are what this records — a new fixture that hits
// one of them is added here with its cause, and a fixture that stops hitting
// one is moved into the manifest by deleting its line.
var corpusExclusions = map[string]string{
	// Wants a face fonts.Shipped() does not carry. render_test.go supplies a
	// test-only "Roboto-Regular"; the bindings ship exactly Shipped(), so
	// this fixture cannot render through them.
	"font-text": "needs the face Roboto-Regular, which is not in fonts.Shipped()",

	// The render data lives only as a Go literal in the fixture's own test.
	// There is no data.json to hand a binding, so the fixture is not
	// reproducible from files alone.
	"alignment-rounding": "its render data exists only as a Go literal, not as data.json",
	"line-spacing":       "its render data exists only as a Go literal, not as data.json",
	"mandatory-break":    "its render data exists only as a Go literal, not as data.json",
	"wrapped-text":       "its render data exists only as a Go literal, not as data.json",

	// No input.folio at all — these fixtures record something other than a
	// template render.
	"minimal-rect":      "no input.folio; the golden is built from an in-Go template",
	"hidden-image":      "no input.folio; the golden is built from an in-Go template",
	"expected-breaks":   "no input.folio; the fixture records line-break positions, not a PDF",
	"thai-break-corpus": "no input.folio; the fixture records a break corpus, not a PDF",

	// A template but no committed golden, so there is no hash to compare
	// against. page-count-20 does carry one and IS in the manifest.
	"page-count-1":  "no expected.json; only page-count-20 of this family carries a golden",
	"page-count-5":  "no expected.json; only page-count-20 of this family carries a golden",
	"page-count-50": "no expected.json; only page-count-20 of this family carries a golden",
}

type corpusFixture struct {
	Slug string `json:"slug"`
	// Data and Params say which of the fixture's optional files the render
	// uses. A binding that finds the file present but the flag false — or the
	// other way round — is not rendering what Go rendered.
	Data   bool `json:"data"`
	Params bool `json:"params"`
	// Diagnostics is Go's sequence for this render, in order. The hash is NOT
	// here: it stays in the fixture's own expected.json.
	Diagnostics []parityDiagnostic `json:"diagnostics"`
}

type corpusExclusion struct {
	Slug   string `json:"slug"`
	Reason string `json:"reason"`
}

type corpusFile struct {
	Comment string `json:"comment"`
	Version string `json:"folio8Version"`
	// Count is the manifest's own record of how many fixtures it carries. The
	// generator refuses to write a manifest whose count is below the one
	// already on disk, so the corpus cannot shrink by accident.
	Count    int               `json:"count"`
	Fixtures []corpusFixture   `json:"fixtures"`
	Excluded []corpusExclusion `json:"excluded"`
}

func corpusManifestPath(root string) string {
	return filepath.Join(root, "folio-js", "test", "data", "go-corpus.json")
}

// The two environment gates. Regeneration is deliberate; ALLOWING THE CORPUS
// TO SHRINK is more deliberate still, and needs its own word.
const (
	corpusUpdateEnv = "FOLIO8_UPDATE_JS_CORPUS"
	corpusShrinkEnv = "FOLIO8_CORPUS_MAY_SHRINK"
)

// corpusAttempt is what rendering one fixture directory from its own files
// with fonts.Shipped() produced.
type corpusAttempt struct {
	SHA256     string
	Diags      []parityDiagnostic
	DataUsed   bool
	ParamsUsed bool
}

// readOptionalFile distinguishes "this fixture has no such file" from "this
// file could not be read". Swallowing the second would silently render a
// fixture without its data and then blame the hash.
func readOptionalFile(path string) (contents []byte, present bool, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

// attemptCorpusRender renders one fixture from its own files and reports what
// came out alongside the hash its expected.json commits to. An error means the
// fixture is not renderable from files at all; a SHA256 that differs from want
// means it is renderable and the bytes moved, which is a different finding.
func attemptCorpusRender(dir string) (attempt corpusAttempt, want string, err error) {
	expBytes, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if err != nil {
		return attempt, "", err
	}
	var rec struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(expBytes, &rec); err != nil {
		return attempt, "", fmt.Errorf("expected.json: %w", err)
	}
	if rec.SHA256 == "" {
		return attempt, "", fmt.Errorf("expected.json records no sha256")
	}
	want = rec.SHA256

	tplBytes, err := os.ReadFile(filepath.Join(dir, "input.folio"))
	if err != nil {
		return attempt, want, err
	}
	dataBytes, dataUsed, err := readOptionalFile(filepath.Join(dir, "data.json"))
	if err != nil {
		return attempt, want, fmt.Errorf("data.json: %w", err)
	}
	paramsBytes, paramsUsed, err := readOptionalFile(filepath.Join(dir, "params.json"))
	if err != nil {
		return attempt, want, fmt.Errorf("params.json: %w", err)
	}

	data := folio8.Data("{}")
	if dataUsed {
		data = folio8.Data(dataBytes)
	}
	var params folio8.Params
	if paramsUsed {
		params = paramsBytes
	}

	tpl, err := folio8.ParseTemplate(tplBytes)
	if err != nil {
		return attempt, want, fmt.Errorf("parse: %w", err)
	}
	res, err := folio8.Render(tpl, data, params, fonts.Shipped())
	if err != nil {
		return attempt, want, fmt.Errorf("render with fonts.Shipped(): %w", err)
	}

	sum := sha256.Sum256(res.Bytes)
	diags := make([]parityDiagnostic, 0, len(res.Diagnostics))
	for _, d := range res.Diagnostics {
		diags = append(diags, parityDiagnostic{strings.ToLower(d.Severity.String()), d.Code, d.ElementID, d.DataPath, d.Message})
	}
	return corpusAttempt{
		SHA256:     hex.EncodeToString(sum[:]),
		Diags:      diags,
		DataUsed:   dataUsed,
		ParamsUsed: paramsUsed,
	}, want, nil
}

func TestCorpusManifest(t *testing.T) {
	root := repoRoot(t)
	fixturesDir := filepath.Join(root, "fixtures")

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures/: %v", err)
	}

	var (
		fixtures []corpusFixture
		excluded []corpusExclusion
		// THREE BUCKETS, THREE DIFFERENT FINDINGS. They are kept apart
		// because their remedies are opposites: an unclassified directory
		// wants a decision, a moved hash wants an investigation, and a
		// stale exclusion wants a deletion. One shared message would offer
		// "exclude it" as the answer to a byte-identity regression.
		unclassified []string
		mismatched   []string
		stale        []string
		seenDirs     = map[string]bool{}
	)

	for _, entry := range entries {
		// Only directories are fixtures. fixtures/ also holds loose files
		// (statement-signoff.json), which are records about fixtures rather
		// than fixtures themselves.
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		// Editor and tool directories are not fixtures and must not fail the
		// generator with a message about renderability. ".git", ".idea",
		// "__pycache__" and friends all start with one of these.
		if strings.HasPrefix(slug, ".") || strings.HasPrefix(slug, "_") {
			continue
		}
		seenDirs[slug] = true
		dir := filepath.Join(fixturesDir, slug)

		if reason, ok := corpusExclusions[slug]; ok {
			if strings.TrimSpace(reason) == "" {
				t.Errorf("fixtures/%s is excluded with an empty reason", slug)
			}
			// AN EXCLUSION IS RE-TESTED, NOT TRUSTED. Its cause can go away
			// — a data.json gets committed, a face joins fonts.Shipped() —
			// and an exclusion nobody re-checks then hides a fixture that
			// could be covered, which is the gap this manifest exists to
			// close.
			if attempt, want, err := attemptCorpusRender(dir); err == nil && attempt.SHA256 == want {
				stale = append(stale, fmt.Sprintf("fixtures/%s (excluded because: %s)", slug, reason))
			}
			excluded = append(excluded, corpusExclusion{Slug: slug, Reason: reason})
			continue
		}

		attempt, want, err := attemptCorpusRender(dir)
		if err != nil {
			unclassified = append(unclassified, fmt.Sprintf("fixtures/%s: %v", slug, err))
			continue
		}
		if attempt.SHA256 != want {
			mismatched = append(mismatched, fmt.Sprintf(
				"fixtures/%s: produced %s, but expected.json records %s", slug, attempt.SHA256, want))
			continue
		}
		fixtures = append(fixtures, corpusFixture{
			Slug:        slug,
			Data:        attempt.DataUsed,
			Params:      attempt.ParamsUsed,
			Diagnostics: attempt.Diags,
		})
	}

	// A MOVED HASH IS A DEFECT UNTIL PROVEN OTHERWISE (AD-21/AD-22). This
	// bucket is reported FIRST and on its own, because the cheapest available
	// response to it — dropping the fixture out of the corpus — is the one
	// thing that must not happen.
	if len(mismatched) > 0 {
		sort.Strings(mismatched)
		t.Fatalf("these fixtures render from their own files with fonts.Shipped() but NO LONGER MATCH their committed golden —\n  %s\n"+
			"A MOVED HASH IS A DEFECT UNTIL PROVEN OTHERWISE (AD-21/AD-22). Find what moved the bytes. "+
			"Do NOT add these to corpusExclusions and do NOT re-record expected.json to match.",
			strings.Join(mismatched, "\n  "))
	}
	// EVERY DIRECTORY IS CLASSIFIED. A directory that cannot be rendered from
	// its own files and carries no stated exclusion is the failure this
	// generator exists to produce: it is what "a new fixture cannot be
	// silently uncovered" means.
	if len(unclassified) > 0 {
		sort.Strings(unclassified)
		t.Fatalf("these fixtures/ directories are neither in the manifest nor excluded with a reason —\n  %s\n"+
			"either fix the fixture so it renders from its own files with fonts.Shipped(), or add it to corpusExclusions with its cause",
			strings.Join(unclassified, "\n  "))
	}
	// An exclusion whose cause has gone away is a hidden fixture.
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("these fixtures are excluded but now render from their own files and REPRODUCE their committed golden —\n  %s\n"+
			"the stated cause no longer holds: delete the line from corpusExclusions so the corpus covers them",
			strings.Join(stale, "\n  "))
	}
	// An exclusion naming a directory that does not exist is stale in the
	// other direction, and would otherwise sit here forever pretending to
	// cover something.
	for slug := range corpusExclusions {
		if !seenDirs[slug] {
			t.Errorf("corpusExclusions names %q, but fixtures/%s does not exist", slug, slug)
		}
	}
	if t.Failed() {
		return
	}

	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].Slug < fixtures[j].Slug })
	sort.Slice(excluded, func(i, j int) bool { return excluded[i].Slug < excluded[j].Slug })

	manifest := corpusFile{
		Comment: "Generated by folio-go/wasm/cmd/render/corpus_test.go (FOLIO8_UPDATE_JS_CORPUS=1). Do not edit by hand. " +
			"Carries no golden digest: each binding reads the hash from the fixture's own expected.json.",
		Version:  folio8.Version,
		Count:    len(fixtures),
		Fixtures: fixtures,
		Excluded: excluded,
	}
	got, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	path := corpusManifestPath(root)
	mayShrink := os.Getenv(corpusShrinkEnv) == "1"
	previous, readErr := os.ReadFile(path)

	// The anti-shrink guard needs a number to compare against, so a manifest
	// that is missing or carries no readable count DISABLES it — which is
	// exactly when it matters most. Say so instead of proceeding.
	priorCount, priorKnown := 0, false
	if readErr == nil {
		var prior corpusFile
		if err := json.Unmarshal(previous, &prior); err == nil {
			priorCount, priorKnown = prior.Count, true
		}
	}
	if !priorKnown && !mayShrink {
		t.Fatalf("%s is missing or carries no readable fixture count, so the anti-shrink guard cannot run. "+
			"To create it for the first time, or to rebuild it after it was corrupted, say so deliberately:\n"+
			"\t%s=1 %s=1 go test ./wasm/cmd/render\n"+
			"every later regeneration then needs only %s=1", path, corpusUpdateEnv, corpusShrinkEnv, corpusUpdateEnv)
	}
	// A CORPUS THAT SHRINKS NEEDS A SENTENCE, NOT A WORKAROUND. The override
	// is named here because without it the only way past this message is to
	// widen corpusExclusions — which shrinks the corpus further and is the
	// opposite of what a legitimate removal wants.
	if priorKnown && len(fixtures) < priorCount && !mayShrink {
		t.Fatalf("refusing to write a SHRUNKEN corpus manifest: %d fixtures now, %d recorded in %s.\n"+
			"A fixture leaving the corpus is a deliberate act. If it IS deliberate — a fixture was removed from "+
			"fixtures/, or newly excluded for a stated cause — regenerate with both gates and say why in the commit:\n"+
			"\t%s=1 %s=1 go test ./wasm/cmd/render",
			len(fixtures), priorCount, path, corpusUpdateEnv, corpusShrinkEnv)
	}

	if os.Getenv(corpusUpdateEnv) == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if readErr != nil {
		t.Fatalf("read %s: %v (regenerate with %s=1)", path, readErr, corpusUpdateEnv)
	}
	if !bytes.Equal(got, previous) {
		t.Fatalf("folio-js/test/data/go-corpus.json has drifted from what Go produces; "+
			"regenerate with %s=1 and review the diff.\n"+
			"A MOVED HASH OR A CHANGED DIAGNOSTIC IS A DEFECT UNTIL PROVEN OTHERWISE (AD-21/AD-22) — %d fixtures included, %d excluded",
			corpusUpdateEnv, len(fixtures), len(excluded))
	}
}
