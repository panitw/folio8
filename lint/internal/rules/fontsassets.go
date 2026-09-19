package rules

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RuleFontsAssetUnaccounted is this guard's stable rule id for a file
// found under fontsAssetLocation that this scan does not recognise as
// one of the shapes AC5 expects to see there (AC5).
const RuleFontsAssetUnaccounted = "fonts-asset-unaccounted"

// RuleFontsAssetMissing is this guard's stable rule id for an EXPECTED
// artifact being absent — either fontsAssetLocation itself, or a face
// that folio-go/fonts/fonts.go declares it embeds (AC5).
//
// This is the deliberate polarity FLIP from the "absence-fonts-dir"
// tripwire it replaces (Story 1.3, D-1.3.4): that guard required
// folio-go/fonts/ to be ABSENT until Story 2.2 shipped faces. Story 2.2
// has now shipped them, so the fail-closed direction inverts — the
// declared location and every declared face are now REQUIRED, and their
// absence is itself the violation, mirroring wordlistassets.go's
// two-rule-id shape for a location that is expected to exist.
const RuleFontsAssetMissing = "fonts-asset-missing"

// RuleFontsAssetNotSfnt is this guard's stable rule id for a file that
// CLAIMS to be a font by its extension but is not one by its contents.
//
// This rule exists because of a measured fail-open. The first version of
// this guard accounted for files by FILENAME EXTENSION alone, so a text
// file named `x.ttf` passed, and so did a stray face nothing embeds.
// D-000.15: key a guard on its purpose, never on a proxy for its
// purpose. The purpose here is "the shipped faces are present and are
// real fonts"; the extension is a proxy for it, and the gap between them
// is where the false pass lived.
const RuleFontsAssetNotSfnt = "fonts-asset-not-a-font"

// fontsAssetLocation is the declared shipped-fonts location this guard
// scans (AC1, AC5, AC9 of Story 2.2): folio-go/fonts/, at the module
// root, NOT under internal/ (AD-8 scopes "no package under internal/
// embeds font data" to internal/ packages only).
const fontsAssetLocation = "folio-go/fonts"

// fontsAssetEmbedSource is the file whose //go:embed directives DEFINE
// which faces ship. It is the source of truth for the expected set, and
// deliberately not a list maintained here — see expectedShippedFaces.
const fontsAssetEmbedSource = "folio-go/fonts/fonts.go"

// fontsAssetFontExtensions are the binary font formats a shipped face
// under fontsAssetLocation may carry — matching
// lint/internal/manifest.go's fontExtensions exactly, since that is the
// package that actually enforces the licence/notice pairing for each
// one (delegated, not duplicated logic).
var fontsAssetFontExtensions = []string{".ttf", ".otf", ".ttc"}

// FontsAssetsStats reports what ScanFontsAssets actually examined
// (D-000.9), the same vacuity-guard shape every other guard in this
// package follows: a scanner that silently visited nothing must not
// report the same "zero findings" a healthy scan of a real, populated
// directory reports.
type FontsAssetsStats struct {
	LocationExists bool
	FilesSeen      int
	// ExpectedFaces is how many faces the embed source declared. Zero
	// with LocationExists true means the expected set could not be
	// derived at all, which is itself a finding — never a quiet pass.
	ExpectedFaces int
	// FacesValidated is how many expected faces were found AND confirmed
	// to be real sfnt binaries. The "N of N" coverage witness.
	FacesValidated int
}

// expectedShippedFaces derives the expected face set from the //go:embed
// directives in folio-go/fonts/fonts.go, returning slash-relative paths
// under fontsAssetLocation.
//
// WHY DERIVED RATHER THAN LISTED. A hand-maintained list here would be a
// second copy of "what ships", in a different module from the first, and
// the two would drift — which is the same defect this guard was rebuilt
// to close one level down. The embed directives ARE what ships: if a
// face is not named by one, its bytes are in no binary; if one names a
// file that does not exist, the Go build fails. So deriving from them
// means the expected set cannot be wrong about what ships, and a face
// deleted from disk is caught HERE (the directive still names it) rather
// than only by a build error somewhere else.
func expectedShippedFaces(root string) ([]string, error) {
	path := filepath.Join(root, filepath.FromSlash(fontsAssetEmbedSource))
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		const directive = "//go:embed "
		if !strings.HasPrefix(line, directive) {
			continue
		}
		for _, pattern := range strings.Fields(strings.TrimPrefix(line, directive)) {
			pattern = strings.Trim(pattern, `"`)
			if pattern == "" {
				continue
			}
			// Only exact font-file paths participate in the expected
			// set. A glob or directory embed (`fonts/*`, `all:fonts`)
			// names no specific face, so it cannot be checked for
			// presence; those are left to the unaccounted-file half.
			lower := strings.ToLower(pattern)
			isFont := false
			for _, ext := range fontsAssetFontExtensions {
				if strings.HasSuffix(lower, ext) {
					isFont = true
					break
				}
			}
			if !isFont || strings.ContainsAny(pattern, "*?[") {
				continue
			}
			out = append(out, pattern)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// looksLikeSfnt reports whether data begins with one of the four sfnt
// magic numbers. This is a CONTENT check, deliberately: the whole reason
// this function exists is that a filename extension is a claim, not a
// fact, and the previous version of this guard believed the claim.
func looksLikeSfnt(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	switch {
	case bytes.HasPrefix(data, []byte{0x00, 0x01, 0x00, 0x00}): // TrueType
		return true
	case bytes.HasPrefix(data, []byte("true")): // Apple TrueType
		return true
	case bytes.HasPrefix(data, []byte("ttcf")): // TrueType collection
		return true
	case bytes.HasPrefix(data, []byte("OTTO")): // CFF-flavoured OpenType
		return true
	}
	return false
}

// ScanFontsAssets is AC5's fail-closed walk over fontsAssetLocation,
// replacing the "absence-fonts-dir" tripwire now that Story 2.2 has
// shipped faces there.
//
// IT CHECKS BOTH DIRECTIONS, which the first version did not:
//
//  1. EVERY EXPECTED FACE IS PRESENT AND IS REALLY A FONT. The expected
//     set is derived from folio-go/fonts/fonts.go's //go:embed
//     directives (expectedShippedFaces), each file must exist, and each
//     must begin with an sfnt magic number.
//  2. EVERY FILE PRESENT IS ACCOUNTED FOR. Recursively; anything that is
//     not an expected face, a LICENSE* file, a NOTICE* file or a .go
//     source is a RuleFontsAssetUnaccounted finding.
//
// The first version did (2) only, and did it by filename extension, so
// both of these passed green at the real location:
//
//   - a stray face copied in as `TotallyBogusFace.ttf` — undeclared,
//     embedded by nothing, accounted for purely by its suffix;
//   - `NotoSansThai-VF.ttf` MOVED OUT of the tree entirely — a deleted
//     face was invisible, because the guard only ever flagged files it
//     did not expect and never asked for files it did.
//
// That is D-000.15's failure (a guard keyed on a proxy for its purpose)
// and D-1.4.11's shrunk-list hazard (a checker whose emptiness passes)
// in one place. It mattered more than it looks: this guard sits directly
// underneath the switch that replaced all three shipped faces.
//
// This guard still deliberately does NOT re-check that each subdirectory
// carries a valid LICENSE*/NOTICE* pair with a Copyright line — that is
// lint/internal/manifest.ResolveAssets' job (AC25), which already walks
// the whole repository for exactly that property and already fails the
// build when it is missing. Duplicating it here would be two
// independently-maintained copies of one rule, drifting apart.
func ScanFontsAssets(root string) ([]Finding, FontsAssetsStats, error) {
	var stats FontsAssetsStats
	dir := filepath.Join(root, filepath.FromSlash(fontsAssetLocation))

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				Path:    fontsAssetLocation,
				Rule:    RuleFontsAssetMissing,
				Message: fontsAssetLocation + ": declared shipped-fonts location is missing (AC5) — Story 2.2 requires it to exist and hold the shipped faces",
			}}, stats, nil
		}
		return nil, stats, err
	}
	stats.LocationExists = true

	expected, err := expectedShippedFaces(root)
	if err != nil {
		return nil, stats, err
	}
	stats.ExpectedFaces = len(expected)

	var findings []Finding

	// The expected set could not be derived. Fail rather than fall back
	// to the shape-only check: a silent downgrade to the weaker guard is
	// exactly how this rule was fail-open in the first place.
	if len(expected) == 0 {
		findings = append(findings, Finding{
			Path: fontsAssetEmbedSource,
			Rule: RuleFontsAssetMissing,
			Message: fontsAssetEmbedSource + ": no //go:embed directive naming a font file could be read, so the " +
				"expected shipped-face set is EMPTY and this guard would verify nothing (AC5, D-000.9)",
		})
	}

	expectedSet := make(map[string]bool, len(expected))
	for _, rel := range expected {
		expectedSet[rel] = true
	}

	// --- direction 1: every expected face present, and really a font ---
	for _, rel := range expected {
		full := fontsAssetLocation + "/" + rel
		data, rerr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if rerr != nil {
			if os.IsNotExist(rerr) {
				findings = append(findings, Finding{
					Path: full,
					Rule: RuleFontsAssetMissing,
					Message: full + ": " + fontsAssetEmbedSource + " declares a //go:embed for this face, but the file is " +
						"MISSING (AC5) — a shipped face that has been deleted or renamed must fail here, not silently " +
						"shrink the set the guard covers",
				})
				continue
			}
			return nil, stats, rerr
		}
		if !looksLikeSfnt(data) {
			findings = append(findings, Finding{
				Path: full,
				Rule: RuleFontsAssetNotSfnt,
				Message: full + ": named by a //go:embed directive and carrying a font extension, but its contents are " +
					"NOT an sfnt font (no TrueType/OpenType magic number) — the extension is a claim, not a fact (D-000.15)",
			})
			continue
		}
		stats.FacesValidated++
	}

	// --- direction 2: every file present is accounted for ---
	var unaccountedRel []string
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		stats.FilesSeen++
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if !fontsAssetFileAccountedFor(rel, d.Name(), expectedSet) {
			unaccountedRel = append(unaccountedRel, rel)
		}
		return nil
	})
	if walkErr != nil {
		return nil, stats, walkErr
	}

	sort.Strings(unaccountedRel)
	for _, rel := range unaccountedRel {
		full := fontsAssetLocation + "/" + rel
		findings = append(findings, Finding{
			Path: full, Rule: RuleFontsAssetUnaccounted,
			Message: full + ": file present at the declared shipped-fonts location is not accounted for (AC5) — expected " +
				"a face named by a //go:embed directive in " + fontsAssetEmbedSource + ", a LICENSE* file, a NOTICE* file, " +
				"or a .go source file",
		})
	}

	return findings, stats, nil
}

// fontsAssetFileAccountedFor reports whether a file living under
// fontsAssetLocation is one this guard expects to see.
//
// A font-extensioned file is accounted for ONLY by being in the expected
// set — i.e. only by actually being embedded. This is the half that
// closes the stray-face fail-open: `TotallyBogusFace.ttf` is no longer
// waved through by its suffix.
func fontsAssetFileAccountedFor(rel, name string, expectedSet map[string]bool) bool {
	if expectedSet[rel] {
		return true
	}
	if strings.HasPrefix(name, "LICENSE") {
		return true
	}
	if strings.HasPrefix(name, "NOTICE") {
		return true
	}
	if strings.HasSuffix(name, ".go") {
		return true
	}
	return false
}
