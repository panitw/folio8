package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// RuleStageRank is this guard's stable rule id (D-000.16): a
// stage-rank violation — a package under folio-go/internal/ importing
// another internal package of EQUAL OR HIGHER rank — or an internal
// package that carries no rank at all.
const RuleStageRank = "stage-rank"

// D-000.16, verbatim on why one table beats AD-5's single arrow: "both
// arrows we already care about fall out BY CONSTRUCTION — layout(7)
// cannot import pdf(8), satisfying AD-5; expr(3) cannot import bind(4),
// satisfying D-1.6.1's pre-commitment that stopped the decimal type
// being duplicated. And it pre-forbids the arrows nobody has thought of
// yet — the entire category that made AD-5 worth naming."
//
// The spine's rule: THE SIGNAL RIDES ON THE VALUE, NEVER THROUGH AN
// IMPORT. "Stages communicate by what they pass, not by what they
// import." A stage that needs something from a later stage receives it
// as a parameter.
//
// stageRankTable is the table, keyed by the package's DIRECTORY
// relative to the scanned root (folio-go/internal/). A package may
// import only a STRICTLY LOWER rank; equal ranks may not import each
// other, so two rank-1 siblings stay independent.
//
// RATIFIED AS MEASURED by D-000.16 (ranks corrected) — it is no longer
// illustrative. D-000.16 published `fontset` 5 and `text` 6; Story 2.5
// measured the real graph and found `fontset -> [geom, text]`
// (fontset.go:25, :245, :445 — Story 2.3a's own vendor containment,
// which returns folio8's `internal/text.Shaper` so no vendor pointer
// crosses the boundary), while `text` imports nothing first-party.
// D-000.16 marked the ranks "illustrative… for the implementing story
// to validate"; the validation ran and swapped exactly these two.
//
// The corrected order is right for a REASON, not merely for
// compatibility: subsetting needs the glyph set, and the glyph set comes
// from shaping (AD-8: "one subset per font per document over the union
// of glyphs used" — you cannot know the union without shaping first).
// So SHAPE -> COLLECT -> SUBSET is the true pipeline order and `text`
// genuinely precedes `fontset`. The original ordering assumed `text`
// depended on `fontset` for metrics; measurement shows metrics arrive as
// VALUES.
//
// It is declared as an ORDERED SLICE, not a map literal, for two
// reasons: the table reads in pipeline order, and nothing in this file
// ever has to range a map to render or search it (D-1.3.5's escape
// hatch is unnecessary if there is no map to range).
var stageRankTable = []stageRank{
	{"geom", 0},
	{"diag", 1}, // Epic 3; ranked ahead of arrival so its first commit is already guarded
	{"pagemodel", 1},
	{"barcode", 1},  // spec-barcode-qr-elements: a pure encoder over geom only; the module root consumes it
	{"designer", 1}, // spec-client-libraries story 1: the designer's wire types and bridge; imports geom only
	{"template", 2},
	{"expr", 3}, // Epic 3; D-1.6.1's expr -/-> bind pre-commitment lives in this number
	{"bind", 4},
	{"text", 5},
	{"fontset", 6},
	{"layout", 7},
	{"pdf", 8},
	{"wasm", 9}, // spec-client-libraries story 1: the designer's session engine; a shell above every stage, importing designer

	// "." is the scan root itself — folio-go/internal/, which holds the
	// test-only `arch` fitness package (no non-test files; it exists to
	// assert properties no single package's own tests can see past their
	// directory). It is NOT a pipeline stage, so it is ranked BELOW
	// every stage rather than above them: at rankNoStage it may import
	// no first-party internal package at all, which is what it does
	// today (measured: arch_test.go and arch_blindspot_test.go import
	// only the standard library). Ranking an observer LAST would have
	// licensed it to import everything; ranking it here means a future
	// fitness test that genuinely needs an import is a deliberate,
	// reviewable one-line change to this table — D-2.5.1's shape.
	{".", rankNoStage},
}

// stageRank is one row of the table: a package directory and its rank.
type stageRank struct {
	Name string
	Rank int
}

// rankNoStage is the rank of a directory that is not a pipeline stage.
// Nothing has a lower rank, so no first-party internal package may be
// imported from one.
const rankNoStage = -1

// rankOf looks up a package directory's rank by scanning the declared
// table in order. Linear over eleven entries, and it keeps the table the
// single declaration of both the ranks and their order.
func rankOf(name string) (int, bool) {
	for _, r := range stageRankTable {
		if r.Name == name {
			return r.Rank, true
		}
	}
	return 0, false
}

// internalMarker is the path segment after which an import path names a
// package this table ranks. Matching on the PATH — never on the local
// alias or on literal source text — is the same discipline
// importAliases exists for (AC12: "never a regex, never the literal
// text").
const internalMarker = "/internal/"

// internalPackageOf returns the ranked package name an import path
// refers to, and whether the path is first-party-internal at all.
// ".../internal/text/wordlist" ranks as "text": a subpackage is part of
// its parent stage, not a stage of its own.
func internalPackageOf(importPath string) (string, bool) {
	idx := strings.Index(importPath, internalMarker)
	if idx == -1 {
		return "", false
	}
	rest := importPath[idx+len(internalMarker):]
	if rest == "" {
		return "", false
	}
	if slash := strings.Index(rest, "/"); slash != -1 {
		rest = rest[:slash]
	}
	return rest, true
}

// StageRankStats reports what ScanStageRank actually examined, from the
// scanner's OWN execution (Major 5's precedent, D-000.9): a vacuity
// guard built on a second, independently-derived walk cannot see a dead
// scanner, because injecting `return nil, stats, nil` as this
// function's first statement would zero these numbers and never touch
// an unrelated walk.
type StageRankStats struct {
	// PackagesVisited is every package directory the walk actually
	// entered, relative to root, sorted. A production caller asserts
	// the packages it cares about are in here BY NAME.
	PackagesVisited []string
	// FilesSeen is every .go file parsed, test files included.
	FilesSeen int
	// FirstPartyImports is how many first-party internal import edges
	// were actually examined. Zero means the rank comparison below never
	// ran even once, whatever the findings say.
	FirstPartyImports int
}

// ScanStageRank is the pure checker (D-1.3.3's two-caller shape: no
// *testing.T, no hard-coded root, no repo-root discovery inside it). It
// walks every .go file under root — TEST FILES INCLUDED, D-1.3.1's
// precedent of not granting an exemption pre-emptively — assigns each
// file's directory a stage rank, and reports:
//
//   - an import of an internal package whose rank is EQUAL OR HIGHER
//     than the importing package's;
//   - a directory under root that carries NO rank (fail-safe default:
//     an unranked package is a FINDING, never a pass, so a new stage
//     must be ranked deliberately — the shape mathAllowedCalls already
//     uses in forbiddenimports.go);
//   - an import of an internal package that carries no rank.
//
// Directories named testdata are skipped by walkGoFiles (AC2), which is
// also why pointing this at a fixture tree INSIDE testdata/ works: the
// skip is by directory NAME during the walk, and the walk's own root is
// never itself skipped.
func ScanStageRank(root string) ([]Finding, StageRankStats, error) {
	var findings []Finding
	var stats StageRankStats
	dirsSeen := map[string]bool{}
	unrankedReported := map[string]bool{}

	err := walkGoFiles(root, func(rel string, file *ast.File, fset *token.FileSet) error {
		stats.FilesSeen++
		dir := filepath.ToSlash(filepath.Dir(rel))
		if !dirsSeen[dir] {
			dirsSeen[dir] = true
			stats.PackagesVisited = append(stats.PackagesVisited, dir)
		}

		// A package's rank is its TOP-LEVEL directory under root: a
		// subpackage belongs to its parent stage.
		stage := dir
		if slash := strings.Index(stage, "/"); slash != -1 {
			stage = stage[:slash]
		}

		ownRank, ranked := rankOf(stage)
		if !ranked {
			// Fail-safe: reported once per package, on the first file
			// seen in it, so the message names a package rather than
			// repeating per file.
			if !unrankedReported[stage] {
				unrankedReported[stage] = true
				findings = append(findings, Finding{
					Path: rel, Rule: RuleStageRank, Line: 1,
					Message: fmt.Sprintf(
						"%s:1: package directory %q under internal/ carries no stage rank — an unranked package is a finding, never a pass: rank it deliberately in lint/internal/rules/stagerank.go's stageRankTable (known ranks: %s)",
						rel, stage, knownRanks()),
				})
			}
			return nil
		}

		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			target, isInternal := internalPackageOf(path)
			if !isInternal {
				continue
			}
			if target == stage {
				// A package importing ITSELF is Go's external-test-package
				// idiom (`package fontset_test` in fontset/, importing
				// fontset to exercise its exported surface — measured:
				// internal/fontset/vendorboundary_test.go:17, which is
				// Story 2.3a's vendor-boundary test). It is one stage
				// talking to itself, not an arrow between stages, so the
				// strictly-lower rule does not apply and this edge is not
				// counted as a first-party edge examined either — counting
				// it would let the vacuity guard's FirstPartyImports be
				// satisfied entirely by self-imports.
				continue
			}
			stats.FirstPartyImports++
			pos := fset.Position(imp.Pos())

			targetRank, targetRanked := rankOf(target)
			if !targetRanked {
				findings = append(findings, Finding{
					Path: rel, Rule: RuleStageRank, Line: pos.Line,
					Message: fmt.Sprintf(
						"%s:%d: %q imports %q, which carries no stage rank — an unranked package is a finding, never a pass (known ranks: %s)",
						rel, pos.Line, stage, target, knownRanks()),
				})
				continue
			}
			if targetRank < ownRank {
				continue
			}
			relation := "a HIGHER"
			if targetRank == ownRank {
				relation = "an EQUAL"
			}
			findings = append(findings, Finding{
				Path: rel, Rule: RuleStageRank, Line: pos.Line,
				Message: fmt.Sprintf(
					"%s:%d: stage-rank violation: %q (rank %d) imports %q (rank %d) — %s rank. "+
						"The pipeline is strictly forward: a package may import only a STRICTLY LOWER rank. "+
						"The signal rides on the VALUE, never through an import — stages communicate by what "+
						"they pass, not by what they import (D-000.16), so pass what is needed as a parameter. "+
						"Known ranks: %s",
					rel, pos.Line, stage, ownRank, target, targetRank, relation, knownRanks()),
			})
		}
		return nil
	})

	sort.Strings(stats.PackagesVisited)
	return findings, stats, err
}

// knownRanks renders the table for a failure message, in the declared
// (rank) order, so the message tells a reader what the legal shape
// actually is rather than only that they broke it.
func knownRanks() string {
	parts := make([]string, 0, len(stageRankTable))
	for _, r := range stageRankTable {
		parts = append(parts, fmt.Sprintf("%s=%d", r.Name, r.Rank))
	}
	return strings.Join(parts, " ")
}
