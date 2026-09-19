package manifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panitw/folio8/lint/internal/licence"
)

// repoRootFromTest mirrors rules.repoRootFromTest (duplicated: this is a
// different package, and it lives only in a _test.go file).
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		folio8Go, err1 := os.Stat(filepath.Join(dir, "folio-go"))
		lintDir, err2 := os.Stat(filepath.Join(dir, "lint"))
		if err1 == nil && folio8Go.IsDir() && err2 == nil && lintDir.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root walking up from %s", dir)
		}
		dir = parent
	}
}

// TestManifestUpToDate is AC19's completeness assertion (RP-9): the
// committed lint/MANIFEST.md must match what Generate/Render produce
// right now. A dependency added to a Go module graph or the designer lockfile
// without regenerating and committing the manifest fails this test —
// `cd lint && go run ./cmd/genmanifest` (lint has its own go.mod — run
// from inside lint/, not the repo root) regenerates it (Finding 16, QA
// review: the previously-documented command failed with "cannot find
// main module").
func TestManifestUpToDate(t *testing.T) {
	root := repoRootFromTest(t)

	rows, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	assetRows, err := ResolveAssets(root)
	if err != nil {
		t.Fatalf("ResolveAssets: %v", err)
	}
	want := Render(rows) + RenderAssets(assetRows)

	committedPath := filepath.Join(root, filepath.FromSlash(CommittedRelPath))
	got, err := os.ReadFile(committedPath)
	if err != nil {
		t.Fatalf("read committed manifest %s: %v (run `cd lint && go run ./cmd/genmanifest`)", committedPath, err)
	}

	if string(got) != want {
		t.Fatalf("lint/MANIFEST.md is out of date — run `cd lint && go run ./cmd/genmanifest` and commit the result")
	}
}

func TestDesignerManifestClassifiesRuntimeAndBuildDependencies(t *testing.T) {
	rows, err := Generate(repoRootFromTest(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"react": "shipped", "react-dom": "shipped", "vite": "build-time-only"}
	for module, shippedBy := range want {
		found := false
		for _, row := range rows {
			if row.Serves == "folio-designer" && row.Module == module {
				found = true
				if row.ShippedBy != shippedBy {
					t.Errorf("%s classification = %q, want %q", module, row.ShippedBy, shippedBy)
				}
			}
		}
		if !found {
			t.Errorf("designer manifest is missing %s", module)
		}
	}
}

// TestResolveAssetsIncludesWordlist is Story 2.1's addition (AC9): the
// CC0 wordlist at folio-go/internal/text/wordlist/ must appear in
// ResolveAssets' output — a font-extension-only walk would never see
// it (that gap is exactly what motivated AC9's separate guard); this
// asserts the manifest generator ALSO accounts for it, not just the
// lint guard.
func TestResolveAssetsIncludesWordlist(t *testing.T) {
	root := repoRootFromTest(t)
	rows, err := ResolveAssets(root)
	if err != nil {
		t.Fatalf("ResolveAssets: %v", err)
	}
	const wantPath = "folio-go/internal/text/wordlist/words_th.txt"
	var found *AssetRow
	for i := range rows {
		if rows[i].Path == wantPath {
			found = &rows[i]
		}
	}
	if found == nil {
		t.Fatalf("expected an AssetRow for %s, got %d rows: %v", wantPath, len(rows), rows)
	}
	if found.Licence != "CC0-1.0" {
		t.Errorf("wordlist AssetRow.Licence = %q, want %q", found.Licence, "CC0-1.0")
	}
	if found.Copyright == "" {
		t.Error("wordlist AssetRow.Copyright is empty")
	}
}

// initGitRepo creates a fresh, empty git repository at dir — DW-19's
// fix consults git's own index, so a synthetic test subject needs a
// REAL repository, not a bare directory tree.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitAdd(t *testing.T, dir string, paths ...string) {
	t.Helper()
	args := append([]string{"add"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %v: %v\n%s", paths, err, out)
	}
}

// TestResolveAssetsExcludesUntrackedDirectoryWithoutError is DW-19's
// own regression guard (Story 3.6, AC10, D-3.6.5): a directory holding
// a real font file on disk but ZERO files git tracks is a local,
// untracked scratch folder — excluded from ResolveAssets' output
// entirely (no row, no error), never mistaken for a licensing
// violation. Anchor: git's own index (a fact this test's synthetic
// repository controls directly).
//
// A second, TRACKED and fully compliant font directory sits alongside
// the untracked one — the real repository's own shape (`.font-sources`
// untracked, `folio-go/fonts/*` tracked) and, since the D-3.6.5
// amendment (Finding 1, QA review, Blocker), a REQUIRED precondition:
// "candidates exist and every one is untracked" is now its own scan
// error (see TestResolveAssetsAllDirectoriesUntrackedIsAScanError),
// so a test proving the untracked-exclusion behaviour in isolation
// must not itself BE that all-untracked case.
func TestResolveAssetsExcludesUntrackedDirectoryWithoutError(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)

	untrackedDir := filepath.Join(root, "scratch-fonts")
	if err := os.MkdirAll(untrackedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A real font-extensioned file, present on disk but NEVER git-added
	// — the exact shape .font-sources/ has on a developer's machine.
	if err := os.WriteFile(filepath.Join(untrackedDir, "Untracked.ttf"), []byte("not a real font, just bytes"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	compliantDir := filepath.Join(root, "tracked-fonts")
	if err := os.MkdirAll(compliantDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fontPath := filepath.Join(compliantDir, "Tracked.ttf")
	if err := os.WriteFile(fontPath, []byte("not a real font, just bytes"), 0o644); err != nil {
		t.Fatalf("write font: %v", err)
	}
	licensePath := filepath.Join(compliantDir, "LICENSE.txt")
	// Story 8.4h (task 8): this fixture carried the string "a licence",
	// which MEASURES as (FamilyUnknown, "") — harmless while an
	// unclassifiable licence produced a "SEE NOTICE" row, fatal the
	// moment it became a build failure (AC1). Upgraded to a
	// classifiable, ALLOWLISTED marker so this test keeps testing what
	// its name says — the untracked-directory exclusion — rather than
	// tripping over the new refusal. This is repairing a fixture whose
	// premise the gate changed, not weakening a claim: the subject
	// under test is git's index, not licence classification.
	if err := os.WriteFile(licensePath, []byte("SPDX-License-Identifier: OFL-1.1\n"), 0o644); err != nil {
		t.Fatalf("write licence: %v", err)
	}
	noticePath := filepath.Join(compliantDir, "NOTICE")
	if err := os.WriteFile(noticePath, []byte("Copyright 2026 Test\n"), 0o644); err != nil {
		t.Fatalf("write notice: %v", err)
	}
	gitAdd(t, root, fontPath, licensePath, noticePath)

	// A trivial tracked file so the repository has at least one commit
	// worth of history to query against.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitAdd(t, root, "README.md")

	rows, err := ResolveAssets(root)
	if err != nil {
		t.Fatalf("ResolveAssets must not error over an untracked scratch directory (DW-19), got: %v", err)
	}
	for _, r := range rows {
		if strings.HasPrefix(r.Path, "scratch-fonts/") {
			t.Errorf("ResolveAssets produced a row for the untracked directory: %+v", r)
		}
	}
}

// TestResolveAssetsStillReportsATrackedViolation is AC10's own named
// mutation: pointing the resolver at a TRACKED directory carrying a
// real violation (a committed font binary with no LICENSE* file) must
// still report it — proving DW-19's fix excludes only what git never
// tracked, and does not "trade one blind spot for another" by
// swallowing a real, committed finding.
//
// Finding 7 (QA review): the original version of this test built a
// synthetic repo holding ONLY the violating tracked directory, so the
// masking scenario AC10's mutation exists to rule out — an untracked
// directory sorting ahead of the real violation — was never exercised.
// This version co-locates an untracked "scratch-fonts" directory that
// sorts alphabetically BEFORE "committed-fonts" (s < c is false, so
// rename it to sort first below) to prove the exclusion added for the
// untracked case cannot swallow a later, genuine finding.
func TestResolveAssetsStillReportsATrackedViolation(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)

	// Sorts alphabetically AHEAD of "committed-fonts" (D-3.6.5's own
	// masking scenario: ".font-sources" < a real asset directory).
	untrackedDir := filepath.Join(root, "aaa-scratch-fonts")
	if err := os.MkdirAll(untrackedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(untrackedDir, "Untracked.ttf"), []byte("not a real font, just bytes"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Deliberately never git-added — this is the co-present untracked
	// directory Finding 7 says the previous version omitted.

	violatingDir := filepath.Join(root, "committed-fonts")
	if err := os.MkdirAll(violatingDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fontPath := filepath.Join(violatingDir, "Committed.ttf")
	if err := os.WriteFile(fontPath, []byte("not a real font, just bytes"), 0o644); err != nil {
		t.Fatalf("write font: %v", err)
	}
	// Deliberately NO LICENSE* file alongside it.
	gitAdd(t, root, fontPath)

	_, err := ResolveAssets(root)
	if err == nil {
		t.Fatal("expected ResolveAssets to report the missing LICENSE* file in a TRACKED directory, got nil error")
	}
	if !strings.Contains(err.Error(), "no LICENSE* file") {
		t.Errorf("expected error to mention %q, got: %v", "no LICENSE* file", err)
	}
}

// TestResolveAssetsAllDirectoriesUntrackedIsAScanError is the D-3.6.5
// amendment's "Required" floor (Finding 1, QA review, Blocker):
// dirOrder non-empty (candidate font-bearing directories exist on
// disk) but EVERY one of them resolves to zero git-tracked files is a
// SCAN ERROR — "could not look" — never silently reported as "nothing
// to report" (D-000.9: the two must not produce identical output).
// Anchor: git's own index. Both directories here hold a real
// font-extensioned file on disk and neither is ever git-added — the
// exact shape a directory rename, a wrong repoRoot, or a layout
// refactor would produce.
func TestResolveAssetsAllDirectoriesUntrackedIsAScanError(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)

	for _, name := range []string{"scratch-one", "scratch-two"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "Untracked.ttf"), []byte("not a real font, just bytes"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	// A trivial tracked file so the repository has at least one commit
	// worth of history to query against, and so dirOrder's two entries
	// are the ONLY font-bearing candidates.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitAdd(t, root, "README.md")

	_, err := ResolveAssets(root)
	if err == nil {
		t.Fatal("expected ResolveAssets to return a scan error when every candidate directory is untracked, got nil")
	}
	if !strings.Contains(err.Error(), "scan error") {
		t.Errorf("expected error to mention %q, got: %v", "scan error", err)
	}
}

// TestResolveAssetsNoCandidateDirectoriesIsNotAScanError pins the
// precision boundary the D-3.6.5 amendment draws explicitly: dirOrder
// being EMPTY (no font-extensioned file exists anywhere on disk) is a
// legitimately empty result, never the scan error above. Without this
// test, TestResolveAssetsAllDirectoriesUntrackedIsAScanError's fix
// could be over-implemented as "zero rows is always an error," which
// would break this repository's own history before any font shipped.
func TestResolveAssetsNoCandidateDirectoriesIsNotAScanError(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitAdd(t, root, "README.md")

	rows, err := ResolveAssets(root)
	if err != nil {
		t.Fatalf("ResolveAssets must not error when no candidate font directory exists at all, got: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected zero asset rows, got %d: %v", len(rows), rows)
	}
}

// ---------------------------------------------------------------------------
// Story 8.4h (AC1, AC2, AC3, D-8.5.13): THE POPULATION-INDEPENDENT RED-PROOFS.
//
// WHY SYNTHETIC AND NOT THE REAL TREE. lint/internal/rules/fontsassets_test.go
// red-proves its guards by MUTATING the live repository's own files and
// restoring them in t.Cleanup. That shape proves the guard fires for the faces
// committed TODAY, and stops proving anything the moment the population
// changes — a new face, a moved directory, a story that ships a catalogue.
// These proofs instead build a NEW, OTHERWISE-EMPTY directory in a fresh
// scratch repository, so the claim they establish is about the GATE, not about
// the tree: proved this way, any real face in any directory is covered by
// construction (D-8.5.4, D-8.5.8b).
//
// EACH ARM ASSERTS A SUBSTRING UNIQUE TO ITS OWN MESSAGE. "Something failed"
// is not a proof of anything — a test that reds on a neighbouring guard (the
// missing-LICENSE refusal, the missing-NOTICE refusal, the D-3.6.5 scan-error
// floor) would keep passing after the refusal it claims to prove was deleted.
// The three font-site refusals are deliberately worded so no substring below
// matches more than one of them.
// ---------------------------------------------------------------------------

// writeSyntheticFontDirectory builds one tracked, otherwise-complete font
// directory — a font binary, a LICENSE carrying licenceText, and a NOTICE with
// a copyright line — inside an already-initialised scratch repository, and git
// ADDS all three. `git add` only, never a commit: gitTrackedFileCount reads
// `git ls-files`, which reads the INDEX.
//
// The font "binary" is the same 27 bytes the DW-19 tests use. The walk keys on
// EXTENSION alone, never on content, so no real sfnt is needed and using one
// would make the fixture claim more than the gate reads.
func writeSyntheticFontDirectory(t *testing.T, root, name, licenceText string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
	files := map[string]string{
		"Synthetic.ttf": "not a real font, just bytes",
		"LICENSE.txt":   licenceText,
		"NOTICE":        "Copyright 2026 Test\n",
	}
	var paths []string
	for base, content := range files {
		full := filepath.Join(dir, base)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
		paths = append(paths, full)
	}
	sort.Strings(paths)
	gitAdd(t, root, paths...)
	return dir
}

// scratchRepoWithFontDirectory is the whole recipe: a fresh temp repository
// holding exactly ONE font directory, carrying licenceText. Nothing else in
// the tree is font-extensioned, so whatever ResolveAssets says is about THIS
// directory and cannot be an artifact of a neighbour.
func scratchRepoWithFontDirectory(t *testing.T, licenceText string) string {
	t.Helper()
	root := t.TempDir()
	initGitRepo(t, root)
	writeSyntheticFontDirectory(t, root, "synthetic-fonts", licenceText)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitAdd(t, root, "README.md")
	return root
}

// TestResolveAssetsRefusesAnUnclassifiableFontLicence is AC2's first arm and
// the hole D-8.5.13's charter names directly: a licence text that classifies
// to nothing used to fall through to the literal label "SEE NOTICE" and
// produce a clean row on a clean build.
//
// "a licence" is the measured unclassifiable input — the same string this
// file's own DW-19 fixture carried until Story 8.4h repaired it.
func TestResolveAssetsRefusesAnUnclassifiableFontLicence(t *testing.T) {
	root := scratchRepoWithFontDirectory(t, "a licence")

	rows, err := ResolveAssets(root)
	if err == nil {
		t.Fatalf("expected ResolveAssets to refuse an unclassifiable font licence, got nil and %d row(s): %v", len(rows), rows)
	}
	// THIS ARM'S OWN MESSAGE. Not shared with the copyleft arm, not shared
	// with the off-allowlist arm, and not produced by any neighbouring guard.
	if !strings.Contains(err.Error(), "could not be classified") {
		t.Errorf("expected the UNCLASSIFIABLE refusal, got: %v", err)
	}
	if !strings.Contains(err.Error(), "synthetic-fonts") {
		t.Errorf("expected the error to LOCATE the directory (AD-14), got: %v", err)
	}
	if rows != nil {
		t.Errorf("a refused directory must produce NO row, got: %v", rows)
	}
}

// TestResolveAssetsRefusesACopyleftFontLicence is AC2's second arm, and the
// hole the charter did NOT name (Design Note 1, measured): a GPL-3.0 licence
// text never reached the "SEE NOTICE" fall-through at all. It classifies
// perfectly well, and shipped a clean row labelled "GPL-3.0" on a clean build,
// because both asset sites DISCARDED the classifier's Family return and
// nothing compared the id to any list.
//
// It must red on the COPYLEFT message specifically, not on the
// off-allowlist one: a copyleft licence is refused BY NAME, not merely by
// absence from the owner's four ids.
func TestResolveAssetsRefusesACopyleftFontLicence(t *testing.T) {
	root := scratchRepoWithFontDirectory(t, "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n")

	rows, err := ResolveAssets(root)
	if err == nil {
		t.Fatalf("expected ResolveAssets to refuse a GPL-3.0 font licence, got nil and %d row(s): %v", len(rows), rows)
	}
	if !strings.Contains(err.Error(), "a copyleft licence") {
		t.Errorf("expected the COPYLEFT refusal, got: %v", err)
	}
	// Explicitly NOT the unclassifiable arm's message — this is the
	// assertion that keeps the two proofs independent.
	if strings.Contains(err.Error(), "could not be classified") {
		t.Errorf("a GPL-3.0 text classifies fine; it must not be refused as unclassifiable: %v", err)
	}
	if !strings.Contains(err.Error(), `"GPL-3.0"`) {
		t.Errorf("expected the error to NAME the classified identifier, got: %v", err)
	}
	if !strings.Contains(err.Error(), "synthetic-fonts") {
		t.Errorf("expected the error to LOCATE the directory (AD-14), got: %v", err)
	}
	if rows != nil {
		t.Errorf("a refused directory must produce NO row, got: %v", rows)
	}
}

// TestResolveAssetsRefusesTheDW125BypassAtTheGate asserts DW-125 AT THE
// SURFACE IT WAS REPORTED AT.
//
// Story 8.4i's classifier tests pin ClassifyLicenceText's return value
// for this input. That is not the same claim. DW-125's finding was that
// a licence file of this shape PASSES THE FAIL-CLOSED ASSET GATE — a
// green build shipping a GPL-licensed font — and the I/O matrix states
// the gate outcome ("Gate fails naming the directory and the copyleft
// id") as a separate expectation from the tuple. A tuple assertion
// cannot show that ResolveAssets consults the family, reaches the
// copyleft arm, or names anything.
//
// THE INPUT IS THE REPORTED ONE: the full GNU GPL v3 title block with a
// stray "SPDX-License-Identifier: MIT" line below it — the bundled
// multi-licence shape that is ordinary in redistributed font packages
// and needs no adversary. Before Story 8.4i this classified
// (permissive, "MIT"), which is on the owner's four-id allowlist, so
// ResolveAssets produced a clean row on a clean build.
//
// IT MUST RED ON THE COPYLEFT ARM'S OWN MESSAGE, not the unclassifiable
// one: D-8.4i.1 fixes the resolution ORDER at copyleft-before-conflict
// precisely so a maintainer reads "GPL detected" and removes the
// dependency, rather than reading "conflicting identifiers" and adding
// an SPDX line. Hazard indicators fail toward the loudest, never the
// most precise — and this test is what proves that ordering survives at
// the gate rather than only in the resolver.
//
// RED-PROVED BY DELETION, and the two probes disagree in a way worth
// recording. Deleting the copyleft arm from resolveLicenceSignals
// reproduces DW-125's reported symptom EXACTLY: this test reds on a
// clean, passing row reading
// {synthetic-fonts/Synthetic.ttf MIT …} — a GPL-licensed font shipped
// on a green build under MIT's label. But narrowing the SPDX collection
// back to the FIRST match (FindAllStringSubmatch(text, 1)) does NOT red
// it, because the GPL NAME signal reaches the copyleft arm on its own
// path. So this test measures the COPYLEFT ARM AT THE GATE, not the
// all-lines collection; TestClassifyCollectsEverySignal's
// "MIT line then GPL-3.0-only line" row is what covers the latter, and
// the two are not substitutes.
func TestResolveAssetsRefusesTheDW125BypassAtTheGate(t *testing.T) {
	const gplTextWithStrayMITLine = "GNU GENERAL PUBLIC LICENSE\n" +
		"Version 3, 29 June 2007\n\n" +
		"Everyone is permitted to copy and distribute verbatim copies of this\n" +
		"license document, but changing it is not allowed.\n\n" +
		"SPDX-License-Identifier: MIT\n"

	root := scratchRepoWithFontDirectory(t, gplTextWithStrayMITLine)

	rows, err := ResolveAssets(root)
	if err == nil {
		t.Fatalf("DW-125: a GPL text carrying a stray permissive SPDX line must not pass the fail-closed "+
			"asset gate; got nil and %d row(s): %v", len(rows), rows)
	}
	if !strings.Contains(err.Error(), "a copyleft licence") {
		t.Errorf("expected the COPYLEFT refusal — the loudest arm, D-8.4i.1's fixed order — got: %v", err)
	}
	if !strings.Contains(err.Error(), `"GPL-3.0"`) {
		t.Errorf("expected the error to NAME the copyleft identifier, not the stray permissive one: %v", err)
	}
	if strings.Contains(err.Error(), `"MIT"`) {
		t.Errorf("the stray SPDX line must not reach the message at all; it is the thing that used to "+
			"decide the verdict: %v", err)
	}
	if strings.Contains(err.Error(), "could not be classified") {
		t.Errorf("this text classifies perfectly well as copyleft; refusing it as unclassifiable would "+
			"send a maintainer to add an SPDX line rather than remove the font (D-8.4i.1): %v", err)
	}
	if !strings.Contains(err.Error(), "synthetic-fonts") {
		t.Errorf("expected the error to LOCATE the directory (AD-14), got: %v", err)
	}
	if rows != nil {
		t.Errorf("a refused directory must produce NO row, got: %v", rows)
	}
}

// TestResolveAssetsRefusesAPermissiveButOffAllowlistFontLicence is the arm
// that proves the font site enforces THE OWNER'S FOUR IDS and not merely
// "permissive". ISC is in licence.permissiveSPDX and would pass any
// family-only or IsPermissiveSPDX-based check — so if a later change routes
// the font path through the dependency-side permissive set (the collapse
// Design Note 6 warns about, in its other direction), this test is what reds.
func TestResolveAssetsRefusesAPermissiveButOffAllowlistFontLicence(t *testing.T) {
	root := scratchRepoWithFontDirectory(t, "SPDX-License-Identifier: ISC\n")

	rows, err := ResolveAssets(root)
	if err == nil {
		t.Fatalf("expected ResolveAssets to refuse an off-allowlist font licence, got nil and %d row(s): %v", len(rows), rows)
	}
	if strings.Contains(err.Error(), "could not be classified") || strings.Contains(err.Error(), "a copyleft licence") {
		t.Errorf("ISC classifies as permissive; it must be refused on its own message, not a neighbouring arm's: %v", err)
	}
	// THE WHOLE MESSAGE, BYTE FOR BYTE, NOT A SUBSTRING OF IT.
	//
	// A single identifier is its own only term, so failingTermPhrase
	// must collapse to "which" and this refusal must read EXACTLY as it
	// read before Story 8.4j made admission per-term. Asserted as one
	// literal because substring checks do not have that property:
	// measured, forcing failingTermPhrase to always return the
	// `whose term %q` form — which stutters here, "classifies as
	// \"ISC\", whose term \"ISC\" is not one of…" — left this test GREEN
	// and reddened only the wordlist site's subtest. A message this
	// story deliberately kept unchanged needs a guard that can see it
	// change.
	want := `synthetic-fonts: licence text classifies as "ISC", which is not one of the licences permitted for a redistributed font: OFL-1.1, Apache-2.0, MIT, Ubuntu-font-1.0 (AC25, AD-26, D-8.5.3)`
	if err.Error() != want {
		t.Errorf("off-allowlist refusal message\n got: %s\nwant: %s", err.Error(), want)
	}
	if rows != nil {
		t.Errorf("a refused directory must produce NO row, got: %v", rows)
	}
}

// TestRenderAssetsPutsTheLicenceColumnNoteWithTheTableItExplains guards
// a rendering detail that only an EMPTY corpus can expose, and the real
// corpus is never empty — so nothing else in this suite can see it.
//
// The "A Licence cell may read as a whole SPDX expression…" paragraph
// explains a COLUMN. Written above the empty-corpus early return it was
// emitted over "_No redistributed non-code assets are committed at this
// commit._", describing a cell of a table that is not there: a reader
// arriving at that section is told how to interpret a Licence value in a
// document containing none.
func TestRenderAssetsPutsTheLicenceColumnNoteWithTheTableItExplains(t *testing.T) {
	const note = "A Licence cell may read as a whole SPDX expression"

	empty := RenderAssets(nil)
	if !strings.Contains(empty, "_No redistributed non-code assets are committed at this commit._") {
		t.Fatalf("expected the empty-corpus sentence, got:\n%s", empty)
	}
	if strings.Contains(empty, note) {
		t.Errorf("the Licence-column note must not be rendered when there is no table to explain:\n%s", empty)
	}

	populated := RenderAssets([]AssetRow{{Path: "p", Licence: "OFL-1.1 OR Apache-2.0", Copyright: "c", Serves: "s"}})
	if !strings.Contains(populated, note) {
		t.Errorf("the Licence-column note must accompany an actual table:\n%s", populated)
	}
	// It explains the header it sits under, so it must precede the table
	// rather than merely appear somewhere in the section.
	if strings.Index(populated, note) > strings.Index(populated, "| Path | Licence |") {
		t.Error("the note must be rendered ABOVE the table it explains")
	}
}

// TestFirstTermNotOnFailsClosedOnAnIncompleteEnumeration pins the
// SECOND conjunct of the shared per-term primitive's precondition:
// admission requires every term to be on the list AND the enumeration to
// have COMPLETED.
//
// licence.ClassifySPDXExpressionTerms returns the terms it found BEFORE
// a failure together with a non-nil error, so "MIT XOR Apache-2.0"
// enumerates as ([MIT], unsupported operator). A helper that consulted
// only the slice would answer "every term is allowed" from a PARTIAL
// READ of the label — reading part of a licence line and admitting on
// the acceptable half is verbatim the defect Story 8.4j exists to close,
// relocated one function along.
//
// TESTED DIRECTLY RATHER THAN THROUGH EITHER GATE ON PURPOSE. No such
// label reaches a gate today: an expression that fails to enumerate
// names nothing, so arm 1 fires on spdx == "" before this helper's
// answer is consulted. But this is a general primitive taking its list
// as a parameter, and a primitive whose safety rests on facts about its
// current two callers is not a primitive — the next caller inherits a
// guarantee nothing states and nothing checks.
func TestFirstTermNotOnFailsClosedOnAnIncompleteEnumeration(t *testing.T) {
	for _, label := range []string{
		"MIT XOR Apache-2.0", // enumerates [MIT], then fails on the operator
		"MIT OR",             // even field count: malformed, no terms
		"(MIT)",              // parenthesised: no terms
	} {
		t.Run(label, func(t *testing.T) {
			// The most permissive possible list: EVERYTHING is on it.
			// Only the completeness conjunct can refuse here, so this
			// cannot pass by accident of the membership test.
			failingTerm, everyTermOn := firstTermNotOn(label, func(string) bool { return true })
			if everyTermOn {
				t.Errorf("firstTermNotOn(%q) admitted a label it could not fully enumerate — "+
					"admission on a PARTIAL read of a licence line is the defect this story closes", label)
			}
			if failingTerm != label {
				t.Errorf("an incompletely enumerated label must be named WHOLE, not by the prefix that "+
					"happened to parse: got %q, want %q", failingTerm, label)
			}
		})
	}
}

// TestResolveAssetsRefusesAnUnenumerableSPDXExpressionAsUNCLASSIFIABLE is
// the GATE-SURFACE half of the composition rule's load-bearing detail:
// an SPDX expression that cannot be enumerated AT ALL names nothing, so
// it arrives at SITE A with spdx == "" and is refused through the FIRST
// arm — "could not be classified" — and not through the third.
//
// WHY IT MATTERS AT THE GATE RATHER THAN ONLY AT THE CLASSIFIER. The
// third arm's message asserts that the text "classifies as" the label it
// prints. For "(MIT)" there is no such classification: the enumerator
// returns ZERO terms. Deciding "single term" by counting whitespace
// FIELDS named it anyway, and the build then refused a real directory on
// a statement that is false — the wrong-ground refusal, which is worse
// than the right refusal because it sends the maintainer to fix a
// licence the tool never actually read.
func TestResolveAssetsRefusesAnUnenumerableSPDXExpressionAsUnclassifiable(t *testing.T) {
	for _, declaration := range []string{"(MIT)", "MIT()", "MIT XOR Apache-2.0"} {
		t.Run(declaration, func(t *testing.T) {
			root := scratchRepoWithFontDirectory(t, "SPDX-License-Identifier: "+declaration+"\n")

			rows, err := ResolveAssets(root)
			if err == nil {
				t.Fatalf("expected ResolveAssets to refuse %q, got nil and %d row(s): %v", declaration, len(rows), rows)
			}
			if !strings.Contains(err.Error(), "could not be classified") {
				t.Errorf("an expression that enumerates to no terms must be refused on the UNCLASSIFIABLE arm, got: %v", err)
			}
			if strings.Contains(err.Error(), "not one of the licences permitted") {
				t.Errorf("this must NOT reach the off-allowlist arm, whose message would assert the text "+
					"classifies as %q — it demonstrably does not: %v", declaration, err)
			}
			if !strings.Contains(err.Error(), "synthetic-fonts") {
				t.Errorf("expected the error to LOCATE the directory (AD-14), got: %v", err)
			}
			if rows != nil {
				t.Errorf("a refused directory must produce NO row, got: %v", rows)
			}
		})
	}
}

// TestResolveAssetsAcceptsEveryAllowlistedFontLicence is the green arm — the
// one that stops the three refusals above from being satisfiable by a gate
// that simply refuses everything. All FOUR of the owner's ids are exercised,
// including Ubuntu-font-1.0, which has no asset in this repository and
// therefore no other live witness (Design Note 8).
func TestResolveAssetsAcceptsEveryAllowlistedFontLicence(t *testing.T) {
	for _, c := range []struct{ name, licenceText, wantID string }{
		{"OFL-1.1 by marker", "SIL OPEN FONT LICENSE\nVersion 1.1 - 26 February 2007\n", "OFL-1.1"},
		{"Apache-2.0 by marker", "Apache License\nVersion 2.0, January 2004\n", "Apache-2.0"},
		{"MIT by marker", "MIT License\n\nPermission is hereby granted, free of charge\n", "MIT"},
		{"Ubuntu-font-1.0 by marker", "UBUNTU FONT LICENCE Version 1.0\n", "Ubuntu-font-1.0"},
		{"Ubuntu-font-1.0 by SPDX line", "SPDX-License-Identifier: Ubuntu-font-1.0\n", "Ubuntu-font-1.0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := scratchRepoWithFontDirectory(t, c.licenceText)

			rows, err := ResolveAssets(root)
			if err != nil {
				t.Fatalf("an allowlisted licence must resolve without error, got: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("expected exactly one row, got %d: %v", len(rows), rows)
			}
			if rows[0].Licence != c.wantID {
				t.Errorf("row licence = %q, want %q", rows[0].Licence, c.wantID)
			}
			if rows[0].Licence == "SEE NOTICE" {
				t.Error(`"SEE NOTICE" is unreachable as a passing outcome after Story 8.4h (AC4)`)
			}
		})
	}
}

// TestWordlistSiteEnforcesThePermissiveSetNotTheFontAllowlist is AC3, and
// Design Note 6's THE-COLLAPSE-HAS-HAPPENED detector. It is stated as a test
// rather than as a principle because a shared string is not the tell — the
// tell is that Site B starts refusing CC0-1.0.
//
// CC0-1.0 is permissive, is the Thai dictionary's real and legitimate licence
// (Story 2.1, D-2.1.3), and is NOT one of the owner's four font ids. So a
// future change that "tidies" the two sites onto one constant reds HERE, on a
// test whose name says why, instead of reddening the whole build on a shipped,
// owner-unobjected asset — or, in the other direction, admitting CC0 fonts.
func TestWordlistSiteEnforcesThePermissiveSetNotTheFontAllowlist(t *testing.T) {
	// The wordlist path has no git-tracked filter — it is gated only on
	// os.Stat of words_th.txt — so a bare temp tree is the whole fixture.
	scratchWordlist := func(t *testing.T, licenceText string) string {
		t.Helper()
		root := t.TempDir()
		dir := filepath.Join(root, filepath.FromSlash(wordlistAssetDir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		for base, content := range map[string]string{
			"words_th.txt":        "คำ\n",
			"LICENSE-CC0-1.0.txt": licenceText,
			"NOTICE":              "Copyright: none asserted\n",
		} {
			if err := os.WriteFile(filepath.Join(dir, base), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", base, err)
			}
		}
		return root
	}

	t.Run("CC0-1.0 passes — it is NOT one of the four font ids", func(t *testing.T) {
		root := scratchWordlist(t, "Creative Commons Legal Code\n\nCC0 1.0 Universal\n")

		row, ok, err := resolveWordlistAssetRow(root)
		if err != nil {
			t.Fatalf("the wordlist's own CC0 legal code must resolve without error — "+
				"refusing it is the signature of Site B having been collapsed onto the font "+
				"allowlist (Design Note 6). Got: %v", err)
		}
		if !ok {
			t.Fatal("expected a wordlist row")
		}
		if row.Licence != "CC0-1.0" {
			t.Errorf("wordlist row licence = %q, want %q", row.Licence, "CC0-1.0")
		}
	})

	t.Run("an unclassifiable wordlist licence is refused, in this function's own voice", func(t *testing.T) {
		root := scratchWordlist(t, "a licence")

		_, ok, err := resolveWordlistAssetRow(root)
		if err == nil {
			t.Fatalf("expected an unclassifiable wordlist licence to be refused, got nil (ok=%v)", ok)
		}
		if !strings.Contains(err.Error(), "wordlist licence text could not be classified") {
			t.Errorf("expected Site B's OWN unclassifiable refusal, got: %v", err)
		}
		// AC9, not AC25: the wordlist site cites its own criterion.
		if !strings.Contains(err.Error(), "(AC9, AD-26)") {
			t.Errorf("expected the wordlist function's own citation voice, got: %v", err)
		}
		if !strings.Contains(err.Error(), wordlistAssetDir) {
			t.Errorf("expected the error to LOCATE the wordlist (AD-14), got: %v", err)
		}
	})

	// The THIRD arm, which the two above cannot reach: an id that
	// classifies to something KNOWN and is neither copyleft nor in
	// permissiveSPDX. CC-BY-SA-4.0 is exactly that shape — a real SPDX
	// identifier, a real licence, and one this project does not accept
	// — so it exercises the !IsPermissiveSPDX branch on its own.
	// Without this subtest that branch could be deleted outright and
	// the whole lint suite stayed green.
	t.Run("a known but non-permissive wordlist licence is refused on its own message", func(t *testing.T) {
		root := scratchWordlist(t, "SPDX-License-Identifier: CC-BY-SA-4.0\n")

		_, _, err := resolveWordlistAssetRow(root)
		if err == nil {
			t.Fatal("expected a CC-BY-SA-4.0 wordlist licence to be refused, got nil")
		}
		if !strings.Contains(err.Error(), "does not recognise as a permissive licence") {
			t.Errorf("expected Site B's NON-PERMISSIVE refusal, got: %v", err)
		}
		// Not a neighbouring arm's message: CC-BY-SA-4.0 classifies
		// fine and is not copyleft, so neither of the other two may fire.
		if strings.Contains(err.Error(), "could not be classified") || strings.Contains(err.Error(), "a copyleft licence") {
			t.Errorf("this arm must red on its OWN message, not a neighbour's: %v", err)
		}
		if !strings.Contains(err.Error(), `"CC-BY-SA-4.0"`) {
			t.Errorf("expected the error to NAME the classified identifier, got: %v", err)
		}
	})

	// The SAME arm-disjointness at SITE B for an expression that cannot
	// be enumerated at all. It names nothing, so it must arrive with
	// wordlistSPDX == "" and be refused as UNCLASSIFIABLE — never on the
	// non-permissive arm, whose message would assert the text
	// "classifies as \"(MIT)\"" while the classifier returned no
	// classification whatever. This is the disjointness this test's own
	// name is about, exercised from the side a field COUNT of the
	// expression got wrong.
	t.Run("an unenumerable SPDX expression is refused as unclassifiable, not as non-permissive", func(t *testing.T) {
		for _, declaration := range []string{"(MIT)", "MIT()", "MIT XOR Apache-2.0"} {
			t.Run(declaration, func(t *testing.T) {
				root := scratchWordlist(t, "SPDX-License-Identifier: "+declaration+"\n")

				row, ok, err := resolveWordlistAssetRow(root)
				if err == nil {
					t.Fatalf("expected %q to be refused, got nil (ok=%v, row=%v)", declaration, ok, row)
				}
				if !strings.Contains(err.Error(), "wordlist licence text could not be classified") {
					t.Errorf("expected Site B's UNCLASSIFIABLE refusal, got: %v", err)
				}
				if strings.Contains(err.Error(), "does not recognise as a permissive licence") {
					t.Errorf("this must NOT reach the non-permissive arm, whose message would assert the "+
						"text classifies as %q — it demonstrably does not: %v", declaration, err)
				}
				if ok {
					t.Error("a refused wordlist must produce NO row")
				}
			})
		}
	})

	t.Run("a copyleft wordlist licence is refused", func(t *testing.T) {
		root := scratchWordlist(t, "GNU GENERAL PUBLIC LICENSE\nVersion 3\n")

		if _, _, err := resolveWordlistAssetRow(root); err == nil {
			t.Fatal("expected a GPL wordlist licence to be refused, got nil")
		} else if !strings.Contains(err.Error(), "a copyleft licence") {
			t.Errorf("expected Site B's copyleft refusal, got: %v", err)
		}
	})
}

// TestFontAssetLicenceAllowlistIsTheOwnersFourIds is DW-120's guard and
// D-8.4i.3's ruling: the owner's font-asset allowlist is pinned against
// an EXACT, TEST-OWNED []string literal, so widening it requires
// changing a test that says WHOSE DECISION it is.
//
// WHY A LITERAL AND NOT A DERIVATION. A guard computed from
// fontAssetLicenceAllowlist — a length check, a set membership loop, a
// round-trip through fontAssetLicenceAllowed — passes ANY edit to the
// constant, which is no guard at all. An anchor the code cannot move is
// the only kind that holds. The four ids below are written out here, in
// this test's own source, as the authority's own list.
//
// WHY THIS TEST AND NOT ITS CONSUMER (D-8.4i.3, D-8.5.8c). It lands in
// the story that repairs the classifier, not in Story 8.5 which is the
// allowlist's first real consumer: a guard owned by its consumer is a
// guard the consumer can move.
//
// MEASURED BEFORE THIS TEST EXISTED (Story 8.4h's close, re-measured at
// 8.4i): appending "GPL-3.0" to fontAssetLicenceAllowlist and running
// `cd lint && go test -count=1 ./...` left ALL FOUR lint packages GREEN.
// The copyleft refusal arm sits ABOVE the allowlist arm in ResolveAssets,
// so a copyleft id smuggled onto the owner's list is not even caught by
// the copyleft refusal on the path that reaches it. D-8.5.13 forbids
// silent list-widening; a list anyone can widen in one line with nothing
// reddening makes that prohibition unenforceable (D-8.4.31).
//
// RED-PROOF, measured at 8.4i's implementation: appending "GPL-3.0" to
// fontAssetLicenceAllowlist reds THIS TEST AND NOTHING ELSE, in any
// package. If four packages red, the guard is not the thing catching it.
//
// This is deliberately NOT the "no cardinality assertion" case D-8.5.4
// ruled on. That objection is to counting a set that legitimately GROWS.
// fontAssetLicenceAllowlist encodes a FIXED owner decision (D-8.5.3) and
// does not grow by design; if the owner changes it, this test is where
// that change is recorded.
func TestFontAssetLicenceAllowlistIsTheOwnersFourIds(t *testing.T) {
	// THE OWNER'S DECISION, D-8.5.3, WRITTEN OUT IN FULL. The owner
	// named four licences for a redistributed font: OFL-1.1,
	// Apache-2.0, MIT and "UFL" — the Ubuntu Font Licence, whose
	// canonical SPDX identifier is Ubuntu-font-1.0. Order included: the
	// refusal message joins this slice, so the permitted set is named
	// in a stable order.
	//
	// CHANGING THIS LITERAL AMENDS AN OWNER DECISION. It is not a
	// maintenance edit. In particular CC0-1.0 must NEVER be added to
	// fix a wordlist scoping error (D-8.5.13) — the two asset sites are
	// independent in SCOPE, not merely in mechanism.
	ownersFourIDs := []string{"OFL-1.1", "Apache-2.0", "MIT", "Ubuntu-font-1.0"}

	if len(fontAssetLicenceAllowlist) != len(ownersFourIDs) {
		t.Fatalf("fontAssetLicenceAllowlist = %v (%d ids), but the owner's decision (D-8.5.3) names exactly %d: %v — "+
			"changing this list amends an owner decision and must be ruled on, not edited",
			fontAssetLicenceAllowlist, len(fontAssetLicenceAllowlist), len(ownersFourIDs), ownersFourIDs)
	}
	for i, want := range ownersFourIDs {
		if fontAssetLicenceAllowlist[i] != want {
			t.Errorf("fontAssetLicenceAllowlist[%d] = %q, want %q — the owner's four ids (D-8.5.3), in order: %v",
				i, fontAssetLicenceAllowlist[i], want, ownersFourIDs)
		}
	}

	// The derived map is a VIEW onto the list, not a second list: a
	// membership added directly to fontAssetLicenceAllowed would slip
	// past the literal above.
	if len(fontAssetLicenceAllowed) != len(ownersFourIDs) {
		t.Errorf("fontAssetLicenceAllowed has %d entries, want %d — the map must be a view onto the "+
			"owner's list (D-8.5.3), never a second list that can drift from it (D-8.5.8c)",
			len(fontAssetLicenceAllowed), len(ownersFourIDs))
	}
	for _, id := range ownersFourIDs {
		if !fontAssetLicenceAllowed[id] {
			t.Errorf("fontAssetLicenceAllowed[%q] = false; it is one of the owner's four ids (D-8.5.3)", id)
		}
	}
}

// TestTheTwoAssetSitesDoNotShareAPolicy states the two-populations invariant
// directly, as a property of the two lists rather than of one input: there is
// an id each site accepts and the other refuses, in BOTH directions. A shared
// constant, a shared helper or a shared list cannot satisfy this test.
func TestTheTwoAssetSitesDoNotShareAPolicy(t *testing.T) {
	// CC0-1.0: legitimate for the wordlist, forbidden for a font.
	if fontAssetLicenceAllowed["CC0-1.0"] {
		t.Error(`CC0-1.0 must NEVER be in the four-id font asset allowlist — adding it to fix a ` +
			`wordlist scoping error would amend an owner decision (D-8.5.3)`)
	}
	if !licence.IsPermissiveSPDX("CC0-1.0") {
		t.Error("CC0-1.0 must remain in the permissive SPDX set — the shipped Thai wordlist is under it (D-2.1.3)")
	}
	// Ubuntu-font-1.0: on the font allowlist AND permissive; ISC: permissive
	// but NOT an acceptable font licence. The second is the direction that
	// proves the font list is not merely "the permissive set".
	if !fontAssetLicenceAllowed["Ubuntu-font-1.0"] {
		t.Error("Ubuntu-font-1.0 is the owner's fourth id (D-8.5.3) and must be on the font allowlist")
	}
	if fontAssetLicenceAllowed["ISC"] {
		t.Error("ISC is permissive but is not one of the owner's four ids; the font site is not the permissive set")
	}
	if !licence.IsPermissiveSPDX("ISC") {
		t.Error("ISC must remain in the permissive SPDX set")
	}
}

// TestCommittedAssetPopulationClassifiesCleanly is AC6, and the assertion that
// makes "the gate does not land red" a fact the build RE-CHECKS rather than a
// claim made once at implementation time. A gate that lands red is not a gate;
// a gate proved green only by a human running a command once is a gate that
// silently rots.
//
// Deliberately NOT a cardinality assertion over the population (D-8.5.4): a
// count written next to the thing it counts stops being true the moment the
// thing grows, and this population grows by design at Story 8.5. What is
// asserted instead is a PROPERTY of every row — which stays true however many
// rows there are, and reds the moment one of them is not permitted.
func TestCommittedAssetPopulationClassifiesCleanly(t *testing.T) {
	root := repoRootFromTest(t)

	rows, err := ResolveAssets(root)
	if err != nil {
		t.Fatalf("the committed asset population must classify cleanly IN THE SAME COMMIT that makes "+
			"classification fatal — halt and report the directory rather than weakening the gate or "+
			"adding an exemption (D-8.5.13). Got: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("ResolveAssets returned no rows at all — this assertion would be vacuous")
	}

	const wordlistPath = wordlistAssetDir + "/words_th.txt"
	// shippedFontsPrefix is the REACH assertion, and it is here because
	// lint/internal/rules' licence/notice red-proofs no longer carry it.
	// Those proofs used to delete a LICENSE/NOTICE file from the real
	// folio-go/fonts/<face>/ and assert ResolveAssets went red — which,
	// incidentally, also proved that folio-go/fonts/ is INSIDE this
	// resolver's walk. They now run against a throwaway copy (they raced
	// this package's own tests, and an interrupted run left a committed
	// licence artifact deleted), and a throwaway copy cannot prove
	// anything about where the real walk goes.
	//
	// So the reach claim is stated here, directly, against the real tree:
	// if a future exclusion, skip-rule or path change quietly removed the
	// shipped fonts from the walk, every assertion below would still pass
	// on the wordlist row alone, and the gate would be enforcing licence
	// policy over a population that no longer contains the fonts.
	const shippedFontsPrefix = "folio-go/fonts/"
	sawWordlist := false
	sawShippedFont := false
	for _, row := range rows {
		if strings.HasPrefix(row.Path, shippedFontsPrefix) {
			sawShippedFont = true
		}
		if row.Licence == "SEE NOTICE" {
			t.Errorf("%s: no generated manifest row may read %q after Story 8.4h (AC4)", row.Path, "SEE NOTICE")
		}
		if row.Path == wordlistPath {
			sawWordlist = true
			// The wordlist is under the OTHER policy, deliberately.
			if row.Licence != "CC0-1.0" {
				t.Errorf("wordlist row licence = %q, want %q", row.Licence, "CC0-1.0")
			}
			continue
		}
		if !fontAssetLicenceAllowed[row.Licence] {
			t.Errorf("%s: font row licence %q is not one of the owner's four ids %v (AC6, D-8.5.3)",
				row.Path, row.Licence, fontAssetLicenceAllowlist)
		}
	}
	if !sawWordlist {
		t.Errorf("expected a row for %s — its absence would make the two-policy assertion above vacuous", wordlistPath)
	}
	if !sawShippedFont {
		t.Errorf("expected at least one row under %s — ResolveAssets' walk must REACH the shipped faces, "+
			"or this test asserts a licence policy over a population the fonts are not in (AC25, AD-26)",
			shippedFontsPrefix)
	}
}

// ────────────────────────────────────────────────────────────────────
// STORY 8.4j — THE COMPOUND DECLARATION, AT THE GATE (D-8.4j.1).
//
// These are the SURFACE proofs. The classifier tests in
// internal/licence pin ClassifyLicenceText's return value for these
// inputs; that is not the same claim. DW-131's finding was that a
// licence file of this shape PASSES THE FAIL-CLOSED ASSET GATE — a
// green build shipping a GPL-offered font under a permissive label in
// lint/MANIFEST.md — and a tuple assertion cannot show that
// ResolveAssets consults the family, reaches the copyleft arm, or names
// anything.

// TestResolveAssetsRefusesACompoundFontLicenceOfferingCopyleft is RP1
// and RP2 at the gate, and the second subtest is the whole point: under
// the single-token capture the two subtests DISAGREED, and the only
// difference between their inputs is which identifier was written
// first.
//
// IT MUST RED ON THE COPYLEFT ARM'S OWN MESSAGE, not the
// off-allowlist one: D-8.4i.1 fixes the resolution order at
// copyleft-before-conflict precisely so a maintainer reads "GPL
// detected" and removes the font, rather than reading "not permitted"
// and asking for the list to be widened.
func TestResolveAssetsRefusesACompoundFontLicenceOfferingCopyleft(t *testing.T) {
	for _, c := range []struct{ name, declaration string }{
		{"copyleft term second", "OFL-1.1 OR GPL-3.0-only"},
		{"copyleft term first", "GPL-3.0-only OR OFL-1.1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := scratchRepoWithFontDirectory(t, "SPDX-License-Identifier: "+c.declaration+"\n")

			rows, err := ResolveAssets(root)
			if err == nil {
				t.Fatalf("DW-131: a font LICENSE offering a copyleft term must not pass the fail-closed "+
					"asset gate; got nil and %d row(s): %v", len(rows), rows)
			}
			if !strings.Contains(err.Error(), "a copyleft licence") {
				t.Errorf("expected the COPYLEFT refusal — the loudest arm, D-8.4i.1's fixed order — got: %v", err)
			}
			if !strings.Contains(err.Error(), `"`+c.declaration+`"`) {
				t.Errorf("expected the error to name the WHOLE declaration %q, not the term the reader "+
					"stopped at: %v", c.declaration, err)
			}
			if strings.Contains(err.Error(), "could not be classified") {
				t.Errorf("this declaration classifies perfectly well as copyleft; refusing it as "+
					"unclassifiable would send a maintainer to edit the SPDX line rather than remove "+
					"the font (D-8.4i.1): %v", err)
			}
			if !strings.Contains(err.Error(), "synthetic-fonts") {
				t.Errorf("expected the error to LOCATE the directory (AD-14), got: %v", err)
			}
			if rows != nil {
				t.Errorf("a refused directory must produce NO row, got: %v", rows)
			}
		})
	}
}

// TestResolveAssetsAdmitsACompoundFontLicenceWhoseTermsAreAllAllowlisted
// IS THE REGRESSION GUARD, and it is the reason admission lives inside
// this story rather than after it.
//
// fontAssetLicenceAllowed is an EXACT-ID map of four ids, so once the
// label becomes the whole expression, NO COMPOUND CAN EVER BE A MEMBER.
// The label fix alone would therefore refuse a legitimately
// dual-licensed permissive face — trading a bypass for a false refusal
// that does not exist in the tree today. Admission is per-term for
// exactly that reason, and this test is what reds if it stops being.
//
// The row must carry the WHOLE expression as its label: the manifest
// states what the file says, and a dual-licensed file is not
// attributable to one of its two licences.
// Story 8.5 (D-8.5.17, matrix row "`WITH` form"): A `WITH` EXPRESSION FAILS
// CLOSED, AND IT FAILS ON THE UNCLASSIFIABLE ARM.
//
// WHY THIS TEST EXISTS AT ALL. Story 8.5's procurement record excludes exactly
// one otherwise-qualifying family — Linux Libertine, whose real expression is
// `OFL-1.1 OR GPL-2.0-or-later WITH Font-exception-2.0` — and it excludes it on
// PARSER SCOPE, NOT LICENCE POLICY. That distinction is load-bearing: the
// answer when someone asks for the family is "widen the parser", never "that
// licence is unacceptable". Nothing in the repository pinned the behaviour that
// sentence rests on, so the claim was prose. This makes it a measurement.
//
// IT ASSERTS THE ARM, NOT MERELY THE FAILURE. "It failed" would be satisfied by
// the copyleft arm firing on the `GPL-2.0-or-later` term, which would mean the
// family is refused as COPYLEFT — a different fact, carrying a different remedy,
// and one that would make the procurement record's wording wrong. The second
// case therefore carries NO copyleft term at all: `MIT WITH
// Font-exception-2.0` is refused for the operator alone.
//
// THIS TEST ADDS NO GATE BEHAVIOUR AND CHANGES NONE. `classify.go` already
// returns FamilyUnknown for an unsupported operator at either position; Story
// 8.5 is forbidden from touching the gate (D-000.11) and does not. It only
// writes down what the gate already does.
func TestResolveAssetsRefusesAWITHExpressionAsUnclassifiableNotAsCopyleft(t *testing.T) {
	for _, c := range []struct{ name, declaration string }{
		{"the Linux Libertine expression", "OFL-1.1 OR GPL-2.0-or-later WITH Font-exception-2.0"},
		{"a WITH clause with no copyleft term", "MIT WITH Font-exception-2.0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := scratchRepoWithFontDirectory(t, "SPDX-License-Identifier: "+c.declaration+"\n")

			rows, err := ResolveAssets(root)
			if err == nil {
				t.Fatalf("a font LICENSE carrying a WITH expression must fail the gate CLOSED; "+
					"got nil and %d row(s): %v", len(rows), rows)
			}
			if !strings.Contains(err.Error(), "could not be classified") {
				t.Errorf("expected the UNCLASSIFIABLE arm — the exclusion is parser scope, pending a "+
					"parser widening, and the remedy is to widen the parser; got: %v", err)
			}
			if strings.Contains(err.Error(), "a copyleft licence") {
				t.Errorf("must NOT reach the copyleft arm: that would say the family is refused because "+
					"its licence is unacceptable, which is the one thing D-8.5.17 says it is not; got: %v", err)
			}
			if rows != nil {
				t.Errorf("a refused directory must produce NO row, got: %v", rows)
			}
		})
	}
}

func TestResolveAssetsAdmitsACompoundFontLicenceWhoseTermsAreAllAllowlisted(t *testing.T) {
	for _, declaration := range []string{"OFL-1.1 OR Apache-2.0", "Apache-2.0 OR OFL-1.1", "MIT AND Apache-2.0"} {
		t.Run(declaration, func(t *testing.T) {
			root := scratchRepoWithFontDirectory(t, "SPDX-License-Identifier: "+declaration+"\n")

			rows, err := ResolveAssets(root)
			if err != nil {
				t.Fatalf("a compound declaration whose every term is on the owner's four-id list must be "+
					"ADMITTED — the fix is a classifier, not a ban on listing two names; got: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("expected exactly one row, got %d: %v", len(rows), rows)
			}
			if rows[0].Licence != declaration {
				t.Errorf("row licence = %q, want the WHOLE expression %q — labelling a dual-licensed file "+
					"with one of its two licences is the partial label this story exists to fix", rows[0].Licence, declaration)
			}
		})
	}
}

// TestResolveAssetsRefusesACompoundFontLicenceNamingTheFailingTerm is
// what proves admission is PER-TERM rather than all-or-nothing.
//
// CC0-1.0 is permissive and classifiable — it is in
// licence.permissiveSPDX, and is legitimately the Thai wordlist's
// licence — but it is emphatically NOT one of the owner's four font
// ids (D-8.5.3). So "OFL-1.1 OR CC0-1.0" resolves permissive, survives
// both earlier arms, and must still be refused: an admission rule that
// elected the first allowlisted term would be first-term-wins IN THE
// GATE, which is DW-131's own defect one function along.
//
// AND IT MUST NAME CC0-1.0 SPECIFICALLY. Naming only the expression
// would leave a maintainer to work out which half of their declaration
// the build objected to.
func TestResolveAssetsRefusesACompoundFontLicenceNamingTheFailingTerm(t *testing.T) {
	root := scratchRepoWithFontDirectory(t, "SPDX-License-Identifier: OFL-1.1 OR CC0-1.0\n")

	rows, err := ResolveAssets(root)
	if err == nil {
		t.Fatalf("admission must be PER-TERM: a compound with an off-allowlist term must be refused even "+
			"though its first term is allowlisted; got nil and %d row(s): %v", len(rows), rows)
	}
	if !strings.Contains(err.Error(), "not one of the licences permitted") {
		t.Errorf("expected the OFF-ALLOWLIST refusal, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"CC0-1.0"`) {
		t.Errorf("expected the error to name the FAILING TERM, not merely the expression: %v", err)
	}
	if strings.Contains(err.Error(), "could not be classified") || strings.Contains(err.Error(), "a copyleft licence") {
		t.Errorf("this declaration classifies as permissive; it must be refused on its own arm's message, "+
			"not a neighbouring arm's: %v", err)
	}
	if rows != nil {
		t.Errorf("a refused directory must produce NO row, got: %v", rows)
	}
}

// TestResolveWordlistAssetRowAdmitsACompoundPermissiveDeclaration is
// RP5: the SAME mechanism defect as SITE A, at SITE B (D-8.4j.9).
//
// This story's half 1 makes the label the WHOLE SPDX expression a
// licence text declares. resolveWordlistAssetRow's third arm was a bare
// exact-id lookup — licence.IsPermissiveSPDX(wordlistSPDX) — and no
// compound expression can ever be a member of an exact-id map. So half
// 1 ALONE turned a wholly-permissive compound declaration into a
// refusal that did not exist in the tree before this story, and the
// refusal's own message asserted the project does not recognise the
// text as permissive WHILE wordlistFamily held FamilyPermissive in
// scope from the same ClassifyLicenceText call.
//
// CC0-1.0 is permissiveSPDX's deliberate member since Story 2.1
// (D-2.1.3) because the shipped Thai dictionary is under it, and MIT is
// permissive everywhere. "CC0-1.0 OR MIT" therefore offers nothing this
// site objects to, and must be ADMITTED — term by term, against THIS
// site's list and not the font allowlist.
func TestResolveWordlistAssetRowAdmitsACompoundPermissiveDeclaration(t *testing.T) {
	scratchWordlist := func(t *testing.T, licenceText string) string {
		t.Helper()
		root := t.TempDir()
		dir := filepath.Join(root, filepath.FromSlash(wordlistAssetDir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		for base, content := range map[string]string{
			"words_th.txt":        "คำ\n",
			"LICENSE-CC0-1.0.txt": licenceText,
			"NOTICE":              "Copyright: none asserted\n",
		} {
			if err := os.WriteFile(filepath.Join(dir, base), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", base, err)
			}
		}
		return root
	}

	for _, declaration := range []string{"CC0-1.0 OR MIT", "MIT OR CC0-1.0"} {
		t.Run(declaration, func(t *testing.T) {
			root := scratchWordlist(t, "SPDX-License-Identifier: "+declaration+"\n")

			row, ok, err := resolveWordlistAssetRow(root)
			if err != nil {
				t.Fatalf("RP5: every term of %q is on THIS site's permissive set, so the wordlist must be "+
					"ADMITTED. An exact-id lookup on a label that may now be a whole expression refuses it, "+
					"with a message contradicting wordlistFamily in scope from the same call: %v", declaration, err)
			}
			if !ok {
				t.Fatal("expected a wordlist row")
			}
			if row.Licence != declaration {
				t.Errorf("wordlist row licence = %q, want the WHOLE expression %q", row.Licence, declaration)
			}
		})
	}

	// The gate is still a gate, and its refusal message for a SINGLE
	// identifier is byte-identical to the one it carried before this
	// story — that is what failingTermPhrase's same-string branch is
	// for, and a stuttering `classifies as "X", whose term "X" is not…`
	// would red here.
	//
	// A COMPOUND cannot reach this arm at SITE B, and the reason is
	// worth stating so nobody adds a case for it: classifyBySPDX calls
	// a term permissive iff it is in permissiveSPDX, so a compound with
	// a term outside that set does not resolve permissive at all — it
	// arrives as FamilyUnknown with no identifier and is caught by arm
	// 1, or as FamilyCopyleft and is caught by arm 2. SITE A is
	// different, and that difference is the whole point of the two
	// policies: its list is a strict SUBSET of permissiveSPDX, so
	// "OFL-1.1 OR CC0-1.0" resolves permissive, survives both earlier
	// arms, and must be refused per term there.
	t.Run("a single unrecognised identifier is refused in the message this site always used", func(t *testing.T) {
		root := scratchWordlist(t, "SPDX-License-Identifier: CC-BY-SA-4.0\n")

		_, _, err := resolveWordlistAssetRow(root)
		if err == nil {
			t.Fatal("expected a CC-BY-SA-4.0 wordlist licence to be refused")
		}
		const want = `classifies as "CC-BY-SA-4.0", which this project does not recognise as a permissive licence`
		if !strings.Contains(err.Error(), want) {
			t.Errorf("a single-identifier refusal must read exactly as it did before Story 8.4j.\n got: %v\nwant substring: %s", err, want)
		}
	})
}
