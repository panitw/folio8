package rules

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWordlistAssetsProductionScan is AC9's production caller: at the
// real repo root, folio-go/internal/text/wordlist/ exists (Story 2.1
// created it) and contains exactly the three expected files, so this
// must report zero findings today.
func TestWordlistAssetsProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	findings, stats, err := ScanWordlistAssets(root)
	if err != nil {
		t.Fatalf("scan repo root: %v", err)
	}
	if !stats.LocationExists {
		t.Fatal("vacuity guard: wordlistAssetLocation was not found at the real repo root — Story 2.1 should have created it")
	}
	if stats.FilesSeen == 0 {
		t.Fatal("vacuity guard: scanner reports it saw zero files at a location expected to hold three")
	}
	if len(findings) > 0 {
		t.Fatalf("unaccounted or missing file(s) at %s (AC9):\n%v", wordlistAssetLocation, findings)
	}
}

// TestWordlistAssetsFixtureScan red-proves AC9's fail-closed property
// on retained fixture roots — never by mutating the real location
// permanently (RP-10 precedent). This story's reopening added the
// subdirectory case (a plain os.ReadDir missed anything nested) and the
// missing-files case (the original version only ever reported EXTRA
// files, so deleting a required one stayed silent).
func TestWordlistAssetsFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	base := filepath.Join(root, "folio-go", "testdata", "lint", "wordlist-assets")

	t.Run("violating", func(t *testing.T) {
		got, stats, err := ScanWordlistAssets(filepath.Join(base, "violating"))
		if err != nil {
			t.Fatalf("scan violating/: %v", err)
		}
		if !stats.LocationExists {
			t.Fatal("expected the fixture's wordlist location to exist")
		}
		want := []Finding{
			{Path: wordlistAssetLocation + "/stray.trie", Rule: RuleWordlistAssetUnaccounted},
			{Path: wordlistAssetLocation + "/subdir/nested-stray.txt", Rule: RuleWordlistAssetUnaccounted},
		}
		assertExactFindings(t, got, want)
	})

	t.Run("compliant", func(t *testing.T) {
		got, stats, err := ScanWordlistAssets(filepath.Join(base, "compliant"))
		if err != nil {
			t.Fatalf("scan compliant/: %v", err)
		}
		if !stats.LocationExists {
			t.Fatal("expected the fixture's wordlist location to exist")
		}
		if len(got) > 0 {
			t.Fatalf("compliant/ must report zero findings, got %v", got)
		}
	})

	t.Run("missing files", func(t *testing.T) {
		got, stats, err := ScanWordlistAssets(filepath.Join(base, "missing-files"))
		if err != nil {
			t.Fatalf("scan missing-files/: %v", err)
		}
		if !stats.LocationExists {
			t.Fatal("expected the fixture's wordlist location to exist")
		}
		want := []Finding{
			{Path: wordlistAssetLocation + "/LICENSE-CC0-1.0.txt", Rule: RuleWordlistAssetMissing},
			{Path: wordlistAssetLocation + "/NOTICE", Rule: RuleWordlistAssetMissing},
		}
		assertExactFindings(t, got, want)
	})

	t.Run("location absent", func(t *testing.T) {
		got, stats, err := ScanWordlistAssets(filepath.Join(base, "location-absent"))
		if err != nil {
			t.Fatalf("scan location-absent/: %v", err)
		}
		if stats.LocationExists {
			t.Fatal("expected LocationExists=false when the wordlist directory itself is absent")
		}
		if len(got) > 0 {
			t.Fatalf("a missing location must report zero findings (not itself a violation), got %v", got)
		}
	})
}

// TestWordlistAssetsRedProofByInjectionAtTheDeclaredLocation is AC9's
// binding red-proof, performed at the DECLARED location —
// wordlistAssetLocation itself, the constant production reads — in TWO
// independent directions per the reopening (D-2.1.2 unchanged — either
// direction passing silently means it is still fail-open):
//
//  1. An unaccounted-for file placed in a SUBDIRECTORY of the declared
//     location fails, by rule id and message.
//  2. Deleting BOTH required licence files fails, by rule id and
//     message, for each missing file.
//
// Both directions used to mutate the REAL committed tree and restore it
// in t.Cleanup. That raced lint/internal/manifest's ResolveAssets tests,
// which read the same working tree while `go test ./...` runs the two
// packages concurrently, and an interrupted run left committed files
// deleted. Each direction now gets its own synthetic root under
// t.TempDir(), carrying exactly wordlistExpectedFiles at the declared
// location — the same claim, made where no other package can observe it.
func TestWordlistAssetsRedProofByInjectionAtTheDeclaredLocation(t *testing.T) {
	t.Run("stray file in a subdirectory", func(t *testing.T) {
		root := scratchWordlistRoot(t)
		dir := filepath.Join(root, filepath.FromSlash(wordlistAssetLocation))
		subdir := filepath.Join(dir, "temp-red-proof-subdir")
		strayPath := filepath.Join(subdir, "nested-stray.txt")

		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(strayPath, []byte("this file must not be accounted for\n"), 0o644); err != nil {
			t.Fatalf("write stray file: %v", err)
		}

		findings, stats, err := ScanWordlistAssets(root)
		if err != nil {
			t.Fatalf("scan with injected subdirectory stray file: %v", err)
		}
		if !stats.LocationExists {
			t.Fatal("LocationExists should be true — the synthetic root carries wordlistAssetLocation")
		}

		wantRel := wordlistAssetLocation + "/temp-red-proof-subdir/nested-stray.txt"
		var found bool
		for _, f := range findings {
			if f.Rule == RuleWordlistAssetUnaccounted && f.Path == wantRel {
				found = true
				if f.Message == "" {
					t.Error("finding message must be non-empty (D-000.13: assert on rule id AND message)")
				}
			}
		}
		if !found {
			t.Fatalf("expected a %s finding for %s, got %v", RuleWordlistAssetUnaccounted, wantRel, findings)
		}
	})

	t.Run("deleting both required licence files", func(t *testing.T) {
		root := scratchWordlistRoot(t)
		dir := filepath.Join(root, filepath.FromSlash(wordlistAssetLocation))
		licencePath := filepath.Join(dir, "LICENSE-CC0-1.0.txt")
		noticePath := filepath.Join(dir, "NOTICE")

		if err := os.Remove(licencePath); err != nil {
			t.Fatalf("remove licence file: %v", err)
		}
		if err := os.Remove(noticePath); err != nil {
			t.Fatalf("remove notice file: %v", err)
		}

		findings, stats, err := ScanWordlistAssets(root)
		if err != nil {
			t.Fatalf("scan with both licence files deleted: %v", err)
		}
		if !stats.LocationExists {
			t.Fatal("LocationExists should be true — the synthetic wordlist directory still exists")
		}

		wantMissing := map[string]bool{
			wordlistAssetLocation + "/LICENSE-CC0-1.0.txt": false,
			wordlistAssetLocation + "/NOTICE":              false,
		}
		for _, f := range findings {
			if f.Rule != RuleWordlistAssetMissing {
				continue
			}
			if _, ok := wantMissing[f.Path]; ok {
				wantMissing[f.Path] = true
				if f.Message == "" {
					t.Error("finding message must be non-empty (D-000.13)")
				}
			}
		}
		for path, found := range wantMissing {
			if !found {
				t.Errorf("expected a %s finding for %s, got %v", RuleWordlistAssetMissing, path, findings)
			}
		}
	})
}

// scratchWordlistRoot builds a synthetic repo root under t.TempDir()
// carrying wordlistAssetLocation populated with exactly
// wordlistExpectedFiles — a clean, compliant starting point each
// red-proof direction then mutates in its own throwaway tree, so the
// real committed wordlist assets are never touched.
//
// It reads the expected set from wordlistExpectedFiles rather than
// listing the three names again: a fourth required file added to
// production must not leave this fixture silently one file short, which
// would turn the missing-file direction green for the wrong reason.
func scratchWordlistRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(wordlistAssetLocation))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if len(wordlistExpectedFiles) == 0 {
		t.Fatal("vacuity guard: wordlistExpectedFiles is empty, so this fixture would assert nothing")
	}
	for name := range wordlistExpectedFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("synthetic\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	// The fixture must itself be clean, or a red-proof below could pass
	// on a finding it did not inject.
	findings, stats, err := ScanWordlistAssets(root)
	if err != nil {
		t.Fatalf("scan synthetic root: %v", err)
	}
	if !stats.LocationExists {
		t.Fatal("synthetic root does not carry wordlistAssetLocation")
	}
	if len(findings) > 0 {
		t.Fatalf("synthetic root must start clean, got %v", findings)
	}
	return root
}
