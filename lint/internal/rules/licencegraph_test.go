package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/lint/internal/licence"
)

// TestLicenceGraphProductionScan is AC18's production caller: it walks
// each of the repo's three real Go modules — folio-go, hashmatrix, lint
// itself — and asserts zero findings, uniformly (D-1.3.9: AD-26's scope
// line is "Binds: all"; a checker covering its own dependency is
// correct, not a bootstrap problem, D-1.3.6/D-1.3.9).
func TestLicenceGraphProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	for _, mod := range []string{"folio-go", "hashmatrix", "lint"} {
		mod := mod
		t.Run(mod, func(t *testing.T) {
			findings, err := ScanLicenceGraph(filepath.Join(root, mod))
			if err != nil {
				t.Fatalf("scan %s module graph: %v", mod, err)
			}
			if len(findings) > 0 {
				var msgs []string
				for _, f := range findings {
					msgs = append(msgs, f.Message)
				}
				t.Fatalf("forbidden or unresolvable licence(s) found in %s's module graph (AD-26):\n%s", mod, strings.Join(msgs, "\n"))
			}
		})
	}
	t.Run("folio-designer npm lockfile", func(t *testing.T) {
		packages, resolveErr := licence.ResolveNPMGraph(filepath.Join(root, "folio-designer"))
		if resolveErr != nil || len(packages) < 2 {
			t.Fatalf("expected complete non-empty npm graph, packages=%d err=%v", len(packages), resolveErr)
		}
		findings, err := ScanNPMGraph(filepath.Join(root, "folio-designer"))
		if err != nil {
			t.Fatalf("scan designer lockfile: %v", err)
		}
		if len(findings) > 0 {
			t.Fatalf("forbidden or unresolvable npm licence(s): %v", findings)
		}
		noticeFindings, err := ScanPDFJSNotice(filepath.Join(root, "folio-designer"))
		if err != nil || len(noticeFindings) != 0 {
			t.Fatalf("scan designer pdfjs notice: findings=%v err=%v", noticeFindings, err)
		}
	})
}

func TestNPMGraphFixtureScan(t *testing.T) {
	makeFixture := func(t *testing.T, nestedLicence string) string {
		t.Helper()
		dir := t.TempDir()
		lock := `{"lockfileVersion":3,"packages":{"":{"name":"fixture"},"node_modules/permissive":{"version":"1.0.0","license":"MIT"},"node_modules/permissive/node_modules/nested":{"version":"1.0.0","license":"` + nestedLicence + `"}}}`
		if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lock), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	t.Run("prohibited transitive", func(t *testing.T) {
		findings, err := ScanNPMGraph(makeFixture(t, "GPL-3.0"))
		if err != nil {
			t.Fatal(err)
		}
		assertExactFindings(t, findings, []Finding{{Path: "nested", Rule: RuleLicence}})
	})
	t.Run("unknown transitive", func(t *testing.T) {
		findings, err := ScanNPMGraph(makeFixture(t, "Proprietary"))
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || findings[0].Path != "folio-designer/package-lock.json" {
			t.Fatalf("unknown lockfile licence must fail closed: %v", findings)
		}
	})
	t.Run("optional GPL and missing metadata fail without node_modules", func(t *testing.T) {
		dir := t.TempDir()
		lock := `{"lockfileVersion":3,"packages":{"":{"name":"fixture"},"node_modules/@esbuild/evil":{"version":"1.0.0","optional":true,"license":"GPL-3.0"}}}`
		if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lock), 0o644); err != nil {
			t.Fatal(err)
		}
		findings, err := ScanNPMGraph(dir)
		if err != nil || len(findings) != 1 || findings[0].Path != "@esbuild/evil" {
			t.Fatalf("optional GPL lockfile record must fail without node_modules: findings=%v err=%v", findings, err)
		}
		lock = `{"lockfileVersion":3,"packages":{"":{"name":"fixture"},"node_modules/missing":{"version":"1.0.0"}}}`
		if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lock), 0o644); err != nil {
			t.Fatal(err)
		}
		findings, err = ScanNPMGraph(dir)
		if err != nil || len(findings) != 1 || findings[0].Path != "folio-designer/package-lock.json" {
			t.Fatalf("missing lockfile licence must fail closed without node_modules: findings=%v err=%v", findings, err)
		}
	})
	t.Run("pdfjs-dist requires its live NOTICE", func(t *testing.T) {
		dir := t.TempDir()
		lock := `{"lockfileVersion":3,"packages":{"":{"name":"fixture"},"node_modules/pdfjs-dist":{"version":"1.0.0","license":"Apache-2.0"}}}`
		if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lock), 0o644); err != nil {
			t.Fatal(err)
		}
		findings, err := ScanPDFJSNotice(dir)
		if err != nil || len(findings) != 1 {
			t.Fatalf("missing pdfjs NOTICE must fail: findings=%v err=%v", findings, err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "third-party-notices", "pdfjs-dist"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"NOTICE", "LICENSE-APACHE-2.0", "LICENSE-CMAPS", "LICENSE-LIBERATION"} {
			if err := os.WriteFile(filepath.Join(dir, "third-party-notices", "pdfjs-dist", name), []byte("license material"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		findings, err = ScanPDFJSNotice(dir)
		if err != nil || len(findings) != 0 {
			t.Fatalf("present pdfjs NOTICE must pass: findings=%v err=%v", findings, err)
		}
	})
}

// TestLicenceGraphFixtureScan is AC29's fixture caller: copyleft/ fails
// naming the module and licence for each of its four sibling stubs;
// permissive/ passes; unknown/ — no LICENSE file at all — fails as
// unresolvable (RP-13, V11).
func TestLicenceGraphFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	base := filepath.Join(root, "lint", "testdata", "licence")

	t.Run("copyleft", func(t *testing.T) {
		findings, err := ScanLicenceGraph(filepath.Join(base, "copyleft"))
		if err != nil {
			t.Fatalf("scan copyleft/ graph: %v", err)
		}
		want := []Finding{
			{Path: "example.test/gpl-lib", Rule: RuleLicence},
			{Path: "example.test/lgpl-lib", Rule: RuleLicence},
			{Path: "example.test/agpl-lib", Rule: RuleLicence},
			{Path: "example.test/sspl-lib", Rule: RuleLicence},
		}
		assertExactFindings(t, findings, want)
	})

	t.Run("permissive", func(t *testing.T) {
		findings, err := ScanLicenceGraph(filepath.Join(base, "permissive"))
		if err != nil {
			t.Fatalf("scan permissive/ graph: %v", err)
		}
		if len(findings) > 0 {
			t.Fatalf("permissive/ graph must PASS with zero findings, got %v", findings)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		findings, err := ScanLicenceGraph(filepath.Join(base, "unknown"))
		if err != nil {
			t.Fatalf("scan unknown/ graph: %v", err)
		}
		want := []Finding{
			{Path: "example.test/mystery-lib", Rule: RuleLicence},
		}
		assertExactFindings(t, findings, want)
	})
}
