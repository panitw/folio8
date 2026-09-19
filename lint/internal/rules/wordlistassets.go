package rules

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// RuleWordlistAssetUnaccounted is this guard's stable rule id for an
// EXTRA, unrecognised file at the declared location (AC9).
const RuleWordlistAssetUnaccounted = "wordlist-asset-unaccounted"

// RuleWordlistAssetMissing is this guard's stable rule id for a
// REQUIRED file being absent (AC9, D-2.1.2's reopening — a walk that
// only ever notices EXTRA files is still fail-open: deleting the
// licence text and the NOTICE left the original implementation
// reporting zero findings, which is silence over an actual AD-26
// violation, not detection of one).
const RuleWordlistAssetMissing = "wordlist-asset-missing"

// wordlistAssetLocation is the ONE declared asset location this story
// scopes AC9 to (D-2.1.2: "a one-location fail-closed walk is strictly
// better than a ten-extension fail-open list" — the number of locations
// is negotiable, the shape is not; widening this to every asset
// location in the repository is D-1.8.11's full inversion and is
// explicitly out of scope here).
const wordlistAssetLocation = "folio-go/internal/text/wordlist"

// wordlistExpectedFiles is every file AD-26 requires (or this story's
// own provenance record adds) at wordlistAssetLocation today: the
// wordlist itself, its CC0-1.0 licence text, and its NOTICE (AC9).
// Route (b)'s criterion (D-2.1.2, binding) is "does the mechanism FAIL
// on a file it does not recognise, or IGNORE it?" — ScanWordlistAssets
// below fails on anything not in this set, AND fails when anything in
// this set is missing; it does neither the "extra file" nor the
// "missing file" version of silent acceptance.
var wordlistExpectedFiles = map[string]bool{
	"words_th.txt":        true,
	"LICENSE-CC0-1.0.txt": true,
	"NOTICE":              true,
}

// WordlistAssetsStats reports what ScanWordlistAssets actually examined
// (D-000.9): a scanner that silently visited nothing must not report
// the same "zero findings" a healthy scan of a real, populated
// directory reports.
type WordlistAssetsStats struct {
	LocationExists bool
	FilesSeen      int
}

// ScanWordlistAssets is AC9's fail-closed walk over wordlistAssetLocation
// (D-2.1.2, route (b)). Two independent failure modes are checked, both
// required for the property to actually be fail-closed (the reopening's
// finding: the original version only implemented the first):
//
//  1. Every file found anywhere UNDER the location (recursively —
//     os.ReadDir alone missed subdirectories entirely) must be named in
//     wordlistExpectedFiles, or it is a RuleWordlistAssetUnaccounted
//     finding.
//  2. Every name in wordlistExpectedFiles must actually be present at
//     the top level of the location, or it is a RuleWordlistAssetMissing
//     finding — deleting the licence text or the NOTICE is exactly the
//     AD-26 violation this guard exists to catch, and a walk that only
//     ever reports EXTRA files stays silent on that.
//
// A missing location (root does not contain wordlistAssetLocation at
// all — e.g. a fixture root standing in for a repo that predates this
// story) is not itself a finding; LocationExists reports which case
// occurred, so a caller asserting "zero findings" cannot mistake "the
// directory doesn't exist" for "the directory is clean".
func ScanWordlistAssets(root string) ([]Finding, WordlistAssetsStats, error) {
	var stats WordlistAssetsStats
	dir := filepath.Join(root, filepath.FromSlash(wordlistAssetLocation))

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, stats, nil
		}
		return nil, stats, err
	}
	stats.LocationExists = true

	seenTopLevel := map[string]bool{}
	var extraRel []string

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
		if !wordlistExpectedFiles[rel] {
			extraRel = append(extraRel, rel)
		} else {
			seenTopLevel[rel] = true
		}
		return nil
	})
	if walkErr != nil {
		return nil, stats, walkErr
	}

	var findings []Finding

	sort.Strings(extraRel)
	for _, rel := range extraRel {
		full := wordlistAssetLocation + "/" + rel
		findings = append(findings, Finding{
			Path: full, Rule: RuleWordlistAssetUnaccounted,
			Message: full + ": file present at the declared wordlist asset location is not accounted for (AC9, D-2.1.2) — every file here must be named in wordlistExpectedFiles, or removed",
		})
	}

	var missing []string
	for name := range wordlistExpectedFiles {
		if !seenTopLevel[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		full := wordlistAssetLocation + "/" + name
		findings = append(findings, Finding{
			Path: full, Rule: RuleWordlistAssetMissing,
			Message: full + ": required file is missing from the declared wordlist asset location (AC9, AD-26, D-2.1.2)",
		})
	}

	return findings, stats, nil
}
