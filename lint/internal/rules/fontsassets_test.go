package rules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panitw/folio8/lint/internal/manifest"
)

// TestFontsAssetsProductionScan is AC5's production caller: at the real
// repo root, folio-go/fonts/ exists (Story 2.2 created it) and holds
// only recognised shapes, so this must report zero findings today.
func TestFontsAssetsProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	findings, stats, err := ScanFontsAssets(root)
	if err != nil {
		t.Fatalf("scan repo root: %v", err)
	}
	if !stats.LocationExists {
		t.Fatal("vacuity guard: fontsAssetLocation was not found at the real repo root — Story 2.2 should have created it")
	}
	if stats.FilesSeen == 0 {
		t.Fatal("vacuity guard: scanner reports it saw zero files at a location expected to hold the shipped faces")
	}
	if len(findings) > 0 {
		t.Fatalf("unaccounted file(s) at %s (AC5):\n%v", fontsAssetLocation, findings)
	}
}

// TestFontsAssetsRedProofByInjectionAtTheDeclaredLocation red-proves
// AC5's fail-closed property at the DECLARED location — fontsAssetLocation
// itself, the constant production reads — rather than at the real
// working tree: a non-font stray file placed under folio-go/fonts/ must
// be reported, by rule id and message.
//
// It used to inject that file into the real committed tree and remove it
// in t.Cleanup. That mutated shared state a CONCURRENTLY-RUNNING package
// reads (lint/internal/manifest's ResolveAssets tests walk the same
// paths, and `go test ./...` runs packages in parallel), and an
// interrupted run left the injected file behind. The synthetic root
// keeps the claim — the scanner's own declared location, a real stray
// file, an assertion by rule id and message — and drops only the
// coupling to this machine's working tree.
//
// This is the non-font polarity; TestFontsAssetsRejectsUndeclaredFace is
// the font-extensioned one, and the two are not the same branch.
func TestFontsAssetsRedProofByInjectionAtTheDeclaredLocation(t *testing.T) {
	root := scratchFontsRoot(t,
		map[string][]byte{"notosans/NotoSans-Regular.ttf": minimalSfnt()},
		map[string][]byte{"notosans/temp-red-proof-stray.txt": []byte("this file must not be accounted for\n")},
	)

	findings, stats, err := ScanFontsAssets(root)
	if err != nil {
		t.Fatalf("scan with injected stray file: %v", err)
	}
	if !stats.LocationExists {
		t.Fatal("LocationExists should be true — the synthetic root carries fontsAssetLocation")
	}

	wantRel := fontsAssetLocation + "/notosans/temp-red-proof-stray.txt"
	assertFinding(t, findings, RuleFontsAssetUnaccounted, wantRel, "is not accounted for")
}

// --- helpers -------------------------------------------------------

// assertFinding asserts a finding exists with the given rule id AND path
// AND a message containing wantMsgSubstr.
//
// D-000.13 requires red-proofs to assert BY RULE ID AND MESSAGE, never by
// exit status. The previous version of this file asserted the rule id and
// path but checked only `f.Message != ""` — inherited verbatim from
// wordlistassets_test.go. A non-empty string is not a message assertion:
// it passes for any message at all, including one describing a different
// violation entirely.
func assertFinding(t *testing.T, findings []Finding, wantRule, wantPath, wantMsgSubstr string) {
	t.Helper()
	for _, f := range findings {
		if f.Rule == wantRule && f.Path == wantPath {
			if !strings.Contains(f.Message, wantMsgSubstr) {
				t.Fatalf(
					"finding %s at %s has message %q, which does not contain %q",
					wantRule, wantPath, f.Message, wantMsgSubstr,
				)
			}
			return
		}
	}
	t.Fatalf("expected a %s finding at %s, got %v", wantRule, wantPath, findings)
}

// minimalSfnt returns the smallest byte sequence that passes
// looksLikeSfnt: a TrueType magic number plus a table-directory header.
// It is not a usable font and is not meant to be — these tests exercise
// the GUARD, and the guard's question is "does this claim to be a font
// and actually begin like one".
func minimalSfnt() []byte {
	b := make([]byte, 12)
	b[0], b[1], b[2], b[3] = 0x00, 0x01, 0x00, 0x00
	return b
}

// scratchFontsRoot builds a synthetic repo root under t.TempDir() with a
// folio-go/fonts/ tree and a fonts.go declaring one //go:embed per entry
// in faces. Faces whose value is nil are DECLARED but not written, which
// is how the missing-face polarity is exercised.
//
// A temp root rather than a committed fixture tree, deliberately: the
// precedent's four fixture roots (wordlist-assets/...) hold text files,
// whereas every polarity here needs a real font binary or a convincing
// near-miss. Committing four more copies of font-shaped bytes into
// testdata — where lint/internal/manifest.ResolveAssets would then demand
// a LICENSE/NOTICE pair for each — buys nothing this does not.
func scratchFontsRoot(t *testing.T, faces map[string][]byte, extraFiles map[string][]byte) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(fontsAssetLocation))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var embeds strings.Builder
	embeds.WriteString("package fonts\n\nimport _ \"embed\"\n\n")
	names := make([]string, 0, len(faces))
	for rel := range faces {
		names = append(names, rel)
	}
	sort.Strings(names)
	for i, rel := range names {
		embeds.WriteString("//go:embed " + rel + "\n")
		embeds.WriteString(fmt.Sprintf("var face%d []byte\n\n", i))
		if data := faces[rel]; data != nil {
			full := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", full, err)
			}
			if err := os.WriteFile(full, data, 0o644); err != nil {
				t.Fatalf("write %s: %v", full, err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "fonts.go"), []byte(embeds.String()), 0o644); err != nil {
		t.Fatalf("write fonts.go: %v", err)
	}
	for rel, data := range extraFiles {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

// TestFontsAssetsRejectsUndeclaredFace is red-proof (1) of the two the
// reviewer used to prove this guard was fail-open. A face copied into the
// tree that no //go:embed directive names used to be accounted for purely
// by its `.ttf` suffix; it must now be reported.
func TestFontsAssetsRejectsUndeclaredFace(t *testing.T) {
	root := scratchFontsRoot(t,
		map[string][]byte{"notosans/NotoSans-Regular.ttf": minimalSfnt()},
		map[string][]byte{"notosans/TotallyBogusFace.ttf": minimalSfnt()},
	)
	findings, stats, err := ScanFontsAssets(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats.FacesValidated != 1 {
		t.Fatalf("coverage witness: validated %d faces, want 1", stats.FacesValidated)
	}
	assertFinding(t, findings,
		RuleFontsAssetUnaccounted,
		fontsAssetLocation+"/notosans/TotallyBogusFace.ttf",
		"not accounted for",
	)
}

// TestFontsAssetsDetectsDeletedFace is red-proof (2). Moving a face out
// of the tree entirely used to leave every TestFontsAssets* green,
// because the guard only ever flagged files it did not expect and never
// asked for files it did — D-1.4.11's shrunk-list hazard exactly.
func TestFontsAssetsDetectsDeletedFace(t *testing.T) {
	root := scratchFontsRoot(t, map[string][]byte{
		"notosans/NotoSans-Regular.ttf":         minimalSfnt(),
		"notosansthai/NotoSansThai-Regular.ttf": nil, // declared, never written
	}, nil)

	findings, stats, err := ScanFontsAssets(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats.ExpectedFaces != 2 {
		t.Fatalf("expected 2 declared faces, got %d", stats.ExpectedFaces)
	}
	if stats.FacesValidated != 1 {
		t.Fatalf("coverage witness: validated %d of 2 declared faces, want 1", stats.FacesValidated)
	}
	assertFinding(t, findings,
		RuleFontsAssetMissing,
		fontsAssetLocation+"/notosansthai/NotoSansThai-Regular.ttf",
		"MISSING",
	)
}

// TestFontsAssetsRejectsNonFontWithFontExtension closes the other half of
// the extension-as-proxy gap: a text file named `.ttf`. The old predicate
// asked only about the suffix, so this passed.
func TestFontsAssetsRejectsNonFontWithFontExtension(t *testing.T) {
	root := scratchFontsRoot(t, map[string][]byte{
		"notosans/NotoSans-Regular.ttf": []byte("I am definitely not a font, but I am named like one.\n"),
	}, nil)

	findings, stats, err := ScanFontsAssets(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats.FacesValidated != 0 {
		t.Fatalf("a text file was validated as a font: FacesValidated=%d", stats.FacesValidated)
	}
	assertFinding(t, findings,
		RuleFontsAssetNotSfnt,
		fontsAssetLocation+"/notosans/NotoSans-Regular.ttf",
		"NOT an sfnt font",
	)
}

// TestFontsAssetsReportsAbsentLocation covers RuleFontsAssetMissing's
// polarity-flip branch, which had no test at all: no test called
// ScanFontsAssets on a root lacking folio-go/fonts/.
func TestFontsAssetsReportsAbsentLocation(t *testing.T) {
	findings, stats, err := ScanFontsAssets(t.TempDir())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats.LocationExists {
		t.Fatal("LocationExists must be false for a root with no folio-go/fonts/")
	}
	assertFinding(t, findings, RuleFontsAssetMissing, fontsAssetLocation, "declared shipped-fonts location is missing")
}

// TestFontsAssetsFailsWhenExpectedSetCannotBeDerived proves the guard
// does NOT silently downgrade to its old shape-only behaviour when the
// embed source yields nothing. A quiet fall-back to the weaker check is
// how this rule was fail-open to begin with.
func TestFontsAssetsFailsWhenExpectedSetCannotBeDerived(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(fontsAssetLocation))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// fonts.go present but declaring no font embeds at all.
	if err := os.WriteFile(filepath.Join(dir, "fonts.go"), []byte("package fonts\n"), 0o644); err != nil {
		t.Fatalf("write fonts.go: %v", err)
	}
	findings, stats, err := ScanFontsAssets(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats.ExpectedFaces != 0 {
		t.Fatalf("expected 0 derivable faces, got %d", stats.ExpectedFaces)
	}
	assertFinding(t, findings, RuleFontsAssetMissing, fontsAssetEmbedSource, "expected shipped-face set is EMPTY")
}

// TestFontsAssetsNoticeRemovalRedProof red-proves AC5/AC25's other
// direction: removing a face's licence/notice pairing must fail the
// build. ScanFontsAssets itself deliberately does NOT re-check this
// (see its doc comment) — that property is
// lint/internal/manifest.ResolveAssets' job (AC25), which already walks
// the whole repo, folio-go/fonts/ included, for exactly this. This test
// proves that enforcement actually fires, by the production error
// message, not by exit status — removing NOTICE.md from
// folio-go/fonts/notosans/ and asserting ResolveAssets errors with the
// "no NOTICE* file" message.
//
// The removal happens in a throwaway repository built by
// scratchRepoFromCommittedFace, NOT in the real working tree. It used to
// delete the committed NOTICE.md and restore it in t.Cleanup, which
// raced lint/internal/manifest's own ResolveAssets tests under a bare
// `go test ./...` and left a committed licence artifact deleted if the
// run was interrupted. What the proof binds to — the real committed
// licence bytes, the real repo-relative path, the production error
// message — is unchanged; the copy is what moved.
func TestFontsAssetsNoticeRemovalRedProof(t *testing.T) {
	root, faceDir := scratchRepoFromCommittedFace(t, "notosans")
	noticePath := filepath.Join(faceDir, "NOTICE.md")

	if err := os.Remove(noticePath); err != nil {
		t.Fatalf("remove NOTICE.md: %v", err)
	}

	_, err := manifest.ResolveAssets(root)
	if err == nil {
		t.Fatal("expected manifest.ResolveAssets to fail with NOTICE.md removed from folio-go/fonts/notosans/, got nil error")
	}
	wantSubstr := "no NOTICE* file"
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("expected error to mention %q, got: %v", wantSubstr, err)
	}
}

// TestFontsAssetsLicenceRemovalRedProof is the polarity AC5 literally
// names — "removing a face's OFL TEXT turns the manifest check red" —
// and it had no test.
//
// TestFontsAssetsNoticeRemovalRedProof above was named for it but removes
// NOTICE.md, which drives manifest.go's `!haveNotice` branch. The
// `!haveLicence` branch — the one that fires when LICENSE-OFL.txt itself
// is deleted, i.e. when the licence text stops travelling with the face
// (AD-26) — was executed by nothing. The mechanism does work; what was
// missing was the proof, and a test that only ever drives one branch has
// shown the other is PRESENT, not that it WORKS.
//
// Like its sibling above, the deletion happens in a throwaway repository
// holding a copy of the real committed face, never in the working tree.
func TestFontsAssetsLicenceRemovalRedProof(t *testing.T) {
	root, faceDir := scratchRepoFromCommittedFace(t, "notosansthai")
	licencePath := filepath.Join(faceDir, "LICENSE-OFL.txt")

	info, err := os.Stat(licencePath)
	if err != nil {
		t.Fatalf("stat LICENSE-OFL.txt before mutating: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s is empty — this red-proof would prove nothing", licencePath)
	}

	if err := os.Remove(licencePath); err != nil {
		t.Fatalf("remove LICENSE-OFL.txt: %v", err)
	}

	_, err = manifest.ResolveAssets(root)
	if err == nil {
		t.Fatal("expected manifest.ResolveAssets to fail with LICENSE-OFL.txt removed from folio-go/fonts/notosansthai/, got nil error")
	}
	wantSubstr := "no LICENSE* file"
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("expected error to mention %q, got: %v", wantSubstr, err)
	}
}

// scratchRepoFromCommittedFace copies the REAL committed
// folio-go/fonts/<face>/ directory into a fresh git repository under
// t.TempDir(), at the same repository-relative path, git-adds it, and
// returns (synthetic root, copied face directory).
//
// It exists so the two licence/notice removal red-proofs can delete a
// licence artifact without deleting a licence artifact THIS repository
// ships. manifest.ResolveAssets consults git's own index before it
// assesses a directory (DW-19), so the synthetic root has to be a real
// repository with the face's files tracked, not a bare directory tree —
// the same recipe lint/internal/manifest's own synthetic-root tests use.
//
// The bytes are the committed ones, deliberately: a red-proof that the
// real OFL text is what classification accepts, and that the real
// NOTICE.md is what satisfies the notice branch, is a stronger claim
// than one made against fabricated placeholder text.
func scratchRepoFromCommittedFace(t *testing.T, face string) (root, faceDir string) {
	t.Helper()

	src := filepath.Join(repoRootFromTest(t), filepath.FromSlash(fontsAssetLocation), face)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read committed face directory %s: %v", src, err)
	}

	root = t.TempDir()
	faceDir = filepath.Join(root, filepath.FromSlash(fontsAssetLocation), face)
	if err := os.MkdirAll(faceDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", faceDir, err)
	}

	var sawFont, sawLicence, sawNotice bool
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("read %s: %v", filepath.Join(src, name), err)
		}
		if err := os.WriteFile(filepath.Join(faceDir, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", filepath.Join(faceDir, name), err)
		}
		switch {
		case strings.EqualFold(filepath.Ext(name), ".ttf"), strings.EqualFold(filepath.Ext(name), ".otf"):
			sawFont = true
		case strings.HasPrefix(name, "LICENSE"):
			sawLicence = true
		case strings.HasPrefix(name, "NOTICE"):
			sawNotice = true
		}
	}
	// Vacuity guard: if the copy did not reproduce the shape the removal
	// red-proofs mutate, they would "prove" the gate fires for the wrong
	// reason — an absent font binary, not an absent licence file.
	if !sawFont || !sawLicence || !sawNotice {
		t.Fatalf("copy of %s is not a redistributable face directory (font=%v licence=%v notice=%v)",
			src, sawFont, sawLicence, sawNotice)
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"add", "-A"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, root, err, out)
		}
	}
	return root, faceDir
}
