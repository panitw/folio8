// Package manifest generates AD-26's third-party licence manifest
// (AC19, AC20): every module in each of the repo's three Go module
// graphs, with a resolved licence, labelled with the module it serves
// and whether that module is shipped or build-time-only.
package manifest

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/panitw/folio8/lint/internal/licence"
)

// servedModule is one of the repo's three real Go modules, and whether
// what it produces is shipped to consumers or exists only to build/test
// the repo (D-1.3.9's shipped/build-time-only label — it does not soften
// AC18's uniform ban; it exists so a future "this copyleft tool is only
// used at build time" argument has to be made explicitly, to the owner).
type servedModule struct {
	dirName   string // relative to repo root
	name      string // as it appears in the manifest's "serves" column
	shippedBy string // "shipped" or "build-time-only"
}

// servedModules is deliberately not derived from a directory listing:
// AD-26 binds all three named modules explicitly (D-1.3.9), and a
// silently-discovered fourth module must not silently join the ban's
// scope without a developer noticing it in a diff.
// CommittedRelPath is the repository-relative path of the committed
// third-party licence manifest, and it is the SINGLE declaration of that
// path (D-3.7.8's one-declaration discipline). Two independent things
// must agree about where the manifest lives — TestManifestUpToDate, which
// asserts the committed file matches what Generate/Render produce, and
// RELEASING.md, which names it as a release obligation (DW-3, retired at
// Epic 4 planning). Both read it from here, so a rename is one edit and
// cannot leave one side pointing at a file that no longer exists.
const CommittedRelPath = "lint/MANIFEST.md"

var servedModules = []servedModule{
	{dirName: "folio-go", name: "folio-go", shippedBy: "shipped"},
	{dirName: "hashmatrix", name: "hashmatrix", shippedBy: "build-time-only"},
	{dirName: "lint", name: "lint", shippedBy: "build-time-only"},
}

// Row is one manifest entry: a resolved dependency, the licence it
// classified as, which repo module it serves, and whether that module
// ships to consumers.
type Row struct {
	Module    string
	Version   string
	Licence   string
	Serves    string
	ShippedBy string
}

// Generate resolves all three of the repo's Go module graphs and
// returns the manifest as a sorted set of rows (AC19). A module whose
// licence cannot be resolved is still included, with Licence set to
// "UNRESOLVED" — Generate itself never fails the build; that is AC18's
// job (rules.ScanLicenceGraph), kept as a separate concern so the
// manifest can always be regenerated and inspected even while red.
func Generate(repoRoot string) ([]Row, error) {
	var rows []Row
	for _, sm := range servedModules {
		modDir := filepath.Join(repoRoot, sm.dirName)
		modules, err := licence.ResolveGraph(modDir)
		if err != nil {
			return nil, fmt.Errorf("resolve %s graph: %w", sm.dirName, err)
		}
		for _, m := range modules {
			row := Row{Module: m.Path, Version: m.Version, Serves: sm.name, ShippedBy: sm.shippedBy}
			text, ok := licence.ReadLicenceText(m.Dir)
			if !ok {
				row.Licence = "UNRESOLVED"
			} else if _, spdx := licence.ClassifyLicenceText(text); spdx != "" {
				row.Licence = spdx
			} else {
				row.Licence = "UNRESOLVED"
			}
			rows = append(rows, row)
		}
	}
	designerPackages, err := licence.ResolveNPMGraph(filepath.Join(repoRoot, "folio-designer"))
	if err != nil {
		return nil, fmt.Errorf("resolve folio-designer lockfile graph: %w", err)
	}
	for _, p := range designerPackages {
		shippedBy := "shipped"
		if p.Dev {
			shippedBy = "build-time-only"
		}
		rows = append(rows, Row{Module: p.Path, Version: p.Version, Licence: p.Licence, Serves: "folio-designer", ShippedBy: shippedBy})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Serves != rows[j].Serves {
			return rows[i].Serves < rows[j].Serves
		}
		return rows[i].Module < rows[j].Module
	})
	return rows, nil
}

// fontExtensions are the binary font formats this scan recognises.
var fontExtensions = []string{".ttf", ".otf", ".ttc"}

// fontAssetLicenceAllowlist is THE OWNER'S DECISION (D-8.5.3, Story
// 8.4h): the complete set of licences under which this repository will
// redistribute a font asset. It is enforced fail-closed at SITE A in
// ResolveAssets below — a font whose licence text classifies outside
// this set, or does not classify at all, FAILS THE BUILD naming its
// directory. AD-26's enforcement posture is "fail the build, never
// warn", and D-8.5.3 extends that posture to redistributed font assets.
//
// THIS IS NOT licence.permissiveSPDX, AND MUST NEVER BE POINTED AT IT.
// That list is the dependency-side permissive set, and it deliberately
// carries CC0-1.0 (Story 2.1, D-2.1.3) because the Thai wordlist is
// legitimately CC0. CC0 is not an acceptable FONT licence and CC0-1.0
// is not one of the owner's four ids. The wordlist path
// (resolveWordlistAssetRow) enforces the OTHER list, on purpose. One
// string literal can appear in both without the two being one policy:
// VALIDATE BY CONSUMER, NOT BY KEY LOCATION. Collapsing these into a
// shared constant because the strings overlap would either reject the
// shipped wordlist on day one or admit CC0 fonts — a scoping error
// wearing the costume of a tidy-up (Design Note 6, Design Note 7).
//
// "UFL" as the owner spelled it is the Ubuntu Font Licence; its
// canonical SPDX identifier is "Ubuntu-font-1.0" (verified by fetch —
// see licence.permissiveSPDX's own note and Design Note 3). Nothing in
// this repository ships under it yet; it exists here before Story 8.5
// needs it.
//
// Ordered, not a bare map, so the refusal message below names the
// permitted set in a stable order.
var fontAssetLicenceAllowlist = []string{"OFL-1.1", "Apache-2.0", "MIT", "Ubuntu-font-1.0"}

var fontAssetLicenceAllowed = func() map[string]bool {
	allowed := make(map[string]bool, len(fontAssetLicenceAllowlist))
	for _, id := range fontAssetLicenceAllowlist {
		allowed[id] = true
	}
	return allowed
}()

// firstTermNotOn is the SHARED MECHANISM of both asset gates' per-term
// admission: it reports whether EVERY term of a resolved licence label
// is on the list the caller supplies, and names the first term that is
// not. Story 8.4j (D-8.4j.1, D-8.4j.9).
//
// THE POLICY STAYS AT THE CALL SITE, AND THE TWO CALL SITES DISAGREE ON
// PURPOSE. The list is a PARAMETER because SITE A (fonts) is governed by
// the owner's four ids (D-8.5.3) and SITE B (the Thai wordlist) by
// licence.permissiveSPDX, which has carried CC0-1.0 deliberately since
// Story 2.1. D-8.5.13 separated those two POLICIES — which list governs
// which population — and it did not separate the MECHANISM of testing a
// label against a list. Sharing the mechanism keeps both policies and
// stops both sites from being wrong in the same way; it is NOT the
// "tidy the two sites onto one constant" that
// TestWordlistSiteEnforcesThePermissiveSetNotTheFontAllowlist forbids,
// and that test still reds if anyone tries it.
//
// THE LABEL AND THIS GATE KEY ON DIFFERENT THINGS, AND THAT IS CORRECT.
// The label states WHAT THE FILE SAYS: since Story 8.4j a compound
// declaration is labelled with its whole expression, because attributing
// a dual-licensed file to one of its two licences is a partial label
// produced by reading part of the line, and lint/MANIFEST.md is a
// release artifact (AD-26). This gate states something else entirely:
// WHETHER EVERY OPTION THE FILE OFFERS IS ACCEPTABLE. So a manifest row
// reading "OFL-1.1 OR Apache-2.0" beside a four-id allowlist is NOT a
// bug, and must not be "fixed" by either widening the list or shortening
// the label. The same note is stated in MANIFEST.md's own assets header,
// for the reader who has the artifact but not this file.
//
// ADMISSION IS NO MORE PERMISSIVE THAN THE CLASSIFICATION IT CONSUMES.
// licence.ClassifySPDXExpressionTerms has resolved conservatively across
// terms since Story 1.3 — copyleft if ANY term is copyleft, unknown if
// ANY term is unrecognised — so it already declines to elect the
// favourable term. If admission elected one (admit on the first
// allowlisted term), the gate would re-open exactly what the classifier
// just closed: DW-131 is first-term-wins in the classifier, and admitting
// by first term is first-term-wins in the gate, one function along.
//
// NO STRING SPLITTING HAPPENS HERE. The term set comes from the single
// existing enumerator — ONE parser, ONE term enumeration, and this is
// the only enumeration either gate performs. A strings.Split(spdx,
// " OR ") in this function would be a second SPDX expression parser.
//
// ITS PRECONDITION FOR ADMISSION IS "EVERY TERM ON THE LIST **AND** THE
// ENUMERATION COMPLETE", AND THE SECOND CONJUNCT IS PART OF THE
// CONTRACT, NOT AN INCIDENTAL. licence.ClassifySPDXExpressionTerms
// returns the terms it found BEFORE a failure TOGETHER WITH a non-nil
// error — "MIT XOR Apache-2.0" enumerates as ([MIT], unsupported SPDX
// operator "XOR") — so a caller consulting only the term slice would
// admit a label on a PARTIAL READ whenever every term it managed to
// enumerate happened to be on the list. That is verbatim the failure
// class Story 8.4j exists to close, one function along. No such label
// reaches either gate today (an expression that fails to enumerate names
// nothing, so arm 1 catches it on spdx == ""), but this helper is
// written and documented as a GENERAL per-term admission primitive, and
// a primitive whose safety depends on facts about its present callers is
// not a primitive. A non-nil error, exactly like an empty term slice,
// means "not admissible".
//
// The known and accepted cost: "OFL-1.1 OR <proprietary>" is refused
// even though the OFL term is takeable on its own. The refusal is LOUD,
// naming the directory and the failing term, so a real case surfaces at
// build time rather than shipping mislabelled. Whether to elect a term
// is an OWNER question about D-8.5.3's four ids; it is not resolved in
// the build and not resolved by widening the allowlist.
func firstTermNotOn(spdx string, onList func(string) bool) (string, bool) {
	_, terms, err := licence.ClassifySPDXExpressionTerms(spdx)
	if err != nil || len(terms) == 0 {
		// Either too malformed to enumerate at all, or enumerated only
		// as far as the failure. "No terms" and "not all the terms" are
		// both "not admissible" — fail closed, and name what we were
		// given rather than the prefix we happened to read.
		return spdx, false
	}
	for _, term := range terms {
		if !onList(term) {
			return term, false
		}
	}
	return "", true
}

// failingTermPhrase renders the clause that names WHICH term a per-term
// refusal objected to, and renders it only when the term is not the
// whole label. For a single-identifier declaration the two are the same
// string, and "classifies as \"CC-BY-SA-4.0\", whose term
// \"CC-BY-SA-4.0\" is not…" says the id twice; that shape is also
// byte-identical to the message both gates carried before Story 8.4j,
// so a single-identifier refusal reads exactly as it always has.
func failingTermPhrase(label, term string) string {
	if term == label {
		return "which"
	}
	return fmt.Sprintf("whose term %q", term)
}

// assetServesLabel derives AC25's manifest "Serves" label from an
// asset's directory, relative to repoRoot (forward-slash form). Finding
// 9 (QA review): the previous implementation scanned only a fixed,
// two-entry directory list, so a font committed ANYWHERE else — a
// typo'd path, a stray copy — was invisible to the accounting even
// though AC25 reads "ANY committed font binary". This function still
// gives the two known locations their specific, already-published
// labels; anything else still gets accounted for (LICENSE*/NOTICE*
// still required, still fails the build if missing), under a label
// that names where it was actually found rather than silently matching
// one of the two known locations.
func assetServesLabel(relDir string) string {
	switch {
	case relDir == "folio-go/testdata/fonts" || strings.HasPrefix(relDir, "folio-go/testdata/fonts/"):
		return "folio-go test fixture"
	case relDir == "folio-go/fonts" || strings.HasPrefix(relDir, "folio-go/fonts/"):
		return "folio-go shipped"
	default:
		return "committed asset (" + relDir + ")"
	}
}

// AssetRow is one committed non-code redistributed asset (AD-26,
// D-1.5.6, AC25): a font binary, the directory it was found in, and
// (once resolvable) the licence and copyright line its accompanying
// notice files carry.
type AssetRow struct {
	Path      string // relative to repo root
	Licence   string
	Copyright string
	Serves    string
}

// ResolveAssets walks the WHOLE repository (Finding 9, QA review — a
// fixed two-entry directory list missed a font committed anywhere else,
// proved by construction: a font at folio-go/assets/Stray.ttf, or at
// folio-go/testdata/fonts/extra/Second.ttf, both stayed green under the
// old implementation) for committed font binaries and requires each
// directory containing one to carry both a licence text file
// (LICENSE*) and a notice file (NOTICE*) naming a copyright line
// (AC25's binding property: "Any committed font binary — fixture or
// shipped — travels with its licence text and copyright line"). A font
// binary present without both is a build failure, not a silent gap —
// mirroring AC18/AC19's "unresolved licence fails the build" treatment
// of the Go-module half of this same rule (AD-26).
//
// The walk excludes ".git" (not a source tree) and
// "*/testdata/lint/**" (lint's OWN guard fixtures — e.g. ScanEmbedFont's
// evading-shape fixtures under folio-go/testdata/lint/embed-font/,
// which deliberately carry .ttf-extensioned files with no real font
// content and no LICENSE/NOTICE, because they exist to test detection,
// not to be redistributed). Every other font-extensioned file in the
// repository, at any depth, is in scope.
func ResolveAssets(repoRoot string) ([]AssetRow, error) {
	fontsByDir := map[string][]string{} // relative dir (slash form) -> font file basenames
	var dirOrder []string

	walkErr := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if d.Name() == "lint" && filepath.Base(filepath.Dir(path)) == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		isFont := false
		for _, fe := range fontExtensions {
			if ext == fe {
				isFont = true
				break
			}
		}
		if !isFont {
			return nil
		}
		rel, rerr := filepath.Rel(repoRoot, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))
		if _, seen := fontsByDir[dir]; !seen {
			dirOrder = append(dirOrder, dir)
		}
		fontsByDir[dir] = append(fontsByDir[dir], filepath.Base(rel))
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s for redistributed font assets: %w", repoRoot, walkErr)
	}

	sort.Strings(dirOrder)

	// Story 3.6, AC10/D-3.6.5 (DW-19, fixed at its specified shape,
	// D-000.58): a directory holding a real font file on disk but ZERO
	// files git actually TRACKS is a local, untracked scratch folder —
	// an ENVIRONMENTAL artifact of THIS machine's working tree, never
	// a licensing violation — and this resolver's own prior wording
	// ("contains a committed font binary but no LICENSE*") was FALSE
	// for exactly that directory: nothing in it is committed, so it is
	// not a redistributed asset this resolver has any business
	// assessing at all.
	//
	// THE SCAN-ERROR CONDITION IS ASSESSED HERE, BEFORE ANY FINDING IS
	// COMPUTED for the directory (git's own index — a fact the
	// directory's on-disk contents cannot move — is consulted first,
	// never after a LICENSE/NOTICE read has already run): a directory
	// that fails this check is EXCLUDED from the findings loop below
	// entirely — skipped, not flagged as a content violation, and
	// never silently promoted into an AssetRow either. This is why the
	// mutation naming a REAL violation in a TRACKED directory (AC10)
	// still fires correctly regardless of sort order between the two:
	// an untracked directory contributes nothing — neither a row nor
	// an error — so it can never mask, or be masked by, a tracked
	// directory's genuine LICENSE/NOTICE finding elsewhere in
	// dirOrder. A silent EXCLUSION here is not the "quiet failure"
	// D-000.58 declined (t.Skip-ing a test, or fabricating committed
	// files under .font-sources/ to make the premise true) — those
	// traded a real problem for the appearance of none; excluding a
	// directory git never tracked reflects what ResolveAssets is
	// actually FOR (AD-26: assets this repository REDISTRIBUTES),
	// which an untracked local cache is not.
	var rows []AssetRow
	sawTrackedDirectory := false
	for _, dir := range dirOrder {
		tracked, terr := gitTrackedFileCount(repoRoot, dir)
		if terr != nil {
			return nil, fmt.Errorf("check git-tracked files under %s: %w", dir, terr)
		}
		if tracked == 0 {
			continue
		}
		sawTrackedDirectory = true

		fontFiles := append([]string(nil), fontsByDir[dir]...)
		sort.Strings(fontFiles)

		absDir := filepath.Join(repoRoot, filepath.FromSlash(dir))
		entries, err := os.ReadDir(absDir)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", absDir, err)
		}

		var licenceText, noticeText string
		var haveLicence, haveNotice bool
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, "LICENSE") {
				b, rerr := os.ReadFile(filepath.Join(absDir, name))
				if rerr != nil {
					return nil, fmt.Errorf("read %s: %w", filepath.Join(absDir, name), rerr)
				}
				licenceText = string(b)
				haveLicence = true
			}
			if strings.HasPrefix(name, "NOTICE") {
				b, rerr := os.ReadFile(filepath.Join(absDir, name))
				if rerr != nil {
					return nil, fmt.Errorf("read %s: %w", filepath.Join(absDir, name), rerr)
				}
				noticeText = string(b)
				haveNotice = true
			}
		}

		if !haveLicence {
			return nil, fmt.Errorf("%s: contains a committed font binary but no LICENSE* file (AC25, AD-26)", dir)
		}
		if !haveNotice {
			return nil, fmt.Errorf("%s: contains a committed font binary but no NOTICE* file naming its copyright line (AC25, AD-26)", dir)
		}

		copyrightLine, ok := extractCopyrightLine(noticeText)
		if !ok {
			return nil, fmt.Errorf("%s: NOTICE file does not contain a line starting with \"Copyright\" (AC25, AD-26)", dir)
		}

		// Story 8.4h (AC1, D-8.5.3): SITE A, the FONT path, FAIL-CLOSED.
		//
		// What was here until this story RECORDED a licence instead of
		// ENFORCING one: `licenceLabel := "SEE NOTICE"` followed by
		// `if _, spdx := ...; spdx != "" { licenceLabel = spdx }`, then
		// a row either way and no error either way. That is TWO holes at
		// two lines, and the smaller one is the more obvious (Design
		// Note 1, measured):
		//
		//   - an UNCLASSIFIABLE licence text fell through to the literal
		//     "SEE NOTICE" and produced a clean row and a clean build;
		//   - a GPL-3.0 licence text never reached that fall-through at
		//     all. It classifies perfectly well, and passed because the
		//     Family return was DISCARDED (`_, spdx :=`) and nothing
		//     compared the id to any list. A GPL font shipped on a green
		//     build with nothing said.
		//
		// Both are closed here. The family verdict is now CONSULTED, so
		// a copyleft text is refused BY NAME rather than merely by
		// absence from a list, and an unclassifiable text is refused
		// rather than labelled.
		// Story 8.4j (D-8.4j.1): ADMISSION IS PER-TERM. The label is now
		// the WHOLE SPDX expression a licence text declares, so a
		// compound declaration is admitted iff EVERY term it offers is
		// on the owner's four-id list. Computed before the switch
		// because a Go switch case cannot bind; arm order still decides,
		// and the first two arms are reached first for the inputs they
		// own (firstTermNotOn on "" is harmless).
		family, spdx := licence.ClassifyLicenceText(licenceText)
		failingTerm, everyTermAllowed := firstTermNotOn(spdx, func(term string) bool { return fontAssetLicenceAllowed[term] })
		switch {
		case spdx == "":
			return nil, fmt.Errorf("%s: licence text could not be classified, so this redistributed font cannot be shown to carry a permitted licence (AC25, AD-26, D-8.5.3)", dir)
		case family == licence.FamilyCopyleft:
			return nil, fmt.Errorf("%s: licence text classifies as %q, a copyleft licence AD-26 forbids for a redistributed font (AC25, AD-26, D-8.5.3)", dir, spdx)
		case !everyTermAllowed:
			return nil, fmt.Errorf("%s: licence text classifies as %q, %s is not one of the licences permitted for a redistributed font: %s (AC25, AD-26, D-8.5.3)", dir, spdx, failingTermPhrase(spdx, failingTerm), strings.Join(fontAssetLicenceAllowlist, ", "))
		}
		licenceLabel := spdx

		serves := assetServesLabel(dir)
		for _, ff := range fontFiles {
			rows = append(rows, AssetRow{
				Path:      dir + "/" + ff,
				Licence:   licenceLabel,
				Copyright: copyrightLine,
				Serves:    serves,
			})
		}
	}

	// D-3.6.5 amendment (Story 3.6, AC10 — "Required"): finding at
	// least one candidate font directory (dirOrder non-empty) but
	// EVERY one of them resolving to zero git-tracked files is not
	// "nothing to report" — it is "could not look," and D-000.9 makes
	// that indistinguishable from "all clear" unless it is surfaced as
	// its own scan error, assessed before any finding is returned.
	// Deliberately narrow: dirOrder being EMPTY (no font-extensioned
	// file exists anywhere on disk) stays legitimately empty — that
	// was true of this repository before fonts shipped, and nothing
	// about an absent candidate is a scan failure. Only "candidates
	// exist and NONE of them is tracked" trips this — the shape a
	// directory rename, a wrong repoRoot, or a layout refactor would
	// produce, and the shape the two directory-scoped tests above
	// cannot see because each holds at least one tracked directory.
	if len(dirOrder) > 0 && !sawTrackedDirectory {
		return nil, fmt.Errorf("scan error: found %d font-bearing director(ies) on disk (%s) but git tracks zero files in any of them — cannot assess licensing (D-3.6.5 amendment, AC10)", len(dirOrder), strings.Join(dirOrder, ", "))
	}

	// The CC0 wordlist (Story 2.1, AC9, AD-26) is a second, distinct
	// declared asset location — NOT font-extensioned, so the walk above
	// never sees it (that gap is exactly what motivated AC9's own
	// separate fail-closed guard, lint/internal/rules/wordlistassets.go
	// — D-1.8.11's premise, confirmed with a second live instance).
	// Resolved explicitly here, rather than widening fontExtensions
	// (which would be the forbidden fail-open shortcut for a completely
	// different reason: fontExtensions is keyed on FONT files, and a
	// wordlist is not one).
	if wordlistRow, ok, err := resolveWordlistAssetRow(repoRoot); err != nil {
		return nil, err
	} else if ok {
		rows = append(rows, wordlistRow)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows, nil
}

// wordlistAssetDir is the same declared location AC9's fail-closed
// guard scans (lint/internal/rules/wordlistassets.go's
// wordlistAssetLocation) — duplicated here deliberately rather than
// imported, matching this codebase's existing precedent of duplicating
// small helpers (e.g. repoRootFromTest) across packages rather than
// creating a shared-utility import just to avoid one constant.
const wordlistAssetDir = "folio-go/internal/text/wordlist"

// resolveWordlistAssetRow produces the wordlist's AssetRow if the
// declared location exists and carries its wordlist, licence text and
// NOTICE (AC9's OWN guard is what enforces "exists and is complete" —
// this function reports "not present" rather than erroring when the
// location is simply absent, e.g. at a revision before Story 2.1).
func resolveWordlistAssetRow(repoRoot string) (AssetRow, bool, error) {
	absDir := filepath.Join(repoRoot, filepath.FromSlash(wordlistAssetDir))
	if _, err := os.Stat(filepath.Join(absDir, "words_th.txt")); err != nil {
		return AssetRow{}, false, nil
	}

	licenceText, err := os.ReadFile(filepath.Join(absDir, "LICENSE-CC0-1.0.txt"))
	if err != nil {
		return AssetRow{}, false, fmt.Errorf("%s: wordlist present but licence text unreadable (AC9, AD-26): %w", wordlistAssetDir, err)
	}
	noticeText, err := os.ReadFile(filepath.Join(absDir, "NOTICE"))
	if err != nil {
		return AssetRow{}, false, fmt.Errorf("%s: wordlist present but NOTICE unreadable (AC9, AD-26): %w", wordlistAssetDir, err)
	}

	copyrightLine, ok := extractCopyrightLine(string(noticeText))
	if !ok {
		return AssetRow{}, false, fmt.Errorf("%s: NOTICE file does not contain a line starting with \"Copyright\" (AC25-equivalent for AC9, AD-26)", wordlistAssetDir)
	}

	// Story 8.4h (AC3, D-8.5.3): SITE B, the WORDLIST path, FAIL-CLOSED
	// — AND DELIBERATELY AGAINST A DIFFERENT LIST FROM THE FONT PATH.
	//
	// THIS SITE'S POLICY IS NOT THE FONT SITE'S, ON PURPOSE: the owner
	// ruled about FONTS (D-8.5.3's four ids are OFL-1.1, Apache-2.0,
	// MIT and the Ubuntu Font Licence), and this asset is a Thai
	// dictionary, not a typeface — legitimately under CC0-1.0, which is
	// not one of those four. So this site consults the EXISTING
	// permissive SPDX set, which has carried CC0-1.0 deliberately since
	// Story 2.1 (D-2.1.3) precisely because of this asset. Pointing
	// this line at manifest.go's fontAssetLicenceAllowlist would reject
	// a shipped, legitimate, owner-unobjected asset on day one. Same
	// phrase, two rooms: DO NOT TIDY THE TWO SITES INTO ONE CONSTANT
	// (Design Note 6).
	//
	// The predicate reads licence.permissiveSPDX itself rather than a
	// second copy of the list, because a duplicated list is a list the
	// code can move (D-8.5.8c).
	//
	// Closed for the same two reasons the font site was: the literal
	// "SEE NOTICE" fall-through passed an unclassifiable licence with a
	// clean row, and the discarded Family return passed a GPL one.
	//
	// STORY 8.4j (D-8.4j.9): ADMISSION HERE IS PER-TERM TOO, AGAINST
	// THIS SITE'S OWN LIST. Arm 3 used to be a bare exact-map lookup,
	// licence.IsPermissiveSPDX(wordlistSPDX). Once half 1 of this story
	// made the label the WHOLE SPDX expression a text declares, no
	// compound could ever be a member of an exact-id map, so a wordlist
	// LICENSE declaring "CC0-1.0 OR MIT" — both terms permissive, and
	// CC0-1.0 permissiveSPDX's deliberate member since Story 2.1 — was
	// refused with a message asserting the project does not recognise
	// it as permissive WHILE wordlistFamily held FamilyPermissive in
	// scope from the same call. The refusal contradicted its own
	// function's other return value inside one switch.
	//
	// This is the same MECHANISM repair as SITE A and emphatically NOT
	// the same POLICY: the list passed to firstTermNotOn here is
	// licence.IsPermissiveSPDX, not the font allowlist. D-8.5.13
	// separated which list governs which population; it did not licence
	// two different ways of testing a label against a list.
	wordlistFamily, wordlistSPDX := licence.ClassifyLicenceText(string(licenceText))
	failingTerm, everyTermPermissive := firstTermNotOn(wordlistSPDX, licence.IsPermissiveSPDX)
	switch {
	case wordlistSPDX == "":
		return AssetRow{}, false, fmt.Errorf("%s: wordlist licence text could not be classified, so this redistributed asset cannot be shown to carry a permitted licence (AC9, AD-26)", wordlistAssetDir)
	case wordlistFamily == licence.FamilyCopyleft:
		return AssetRow{}, false, fmt.Errorf("%s: wordlist licence text classifies as %q, a copyleft licence AD-26 forbids for a redistributed asset (AC9, AD-26)", wordlistAssetDir, wordlistSPDX)
	case !everyTermPermissive:
		return AssetRow{}, false, fmt.Errorf("%s: wordlist licence text classifies as %q, %s this project does not recognise as a permissive licence (AC9, AD-26)", wordlistAssetDir, wordlistSPDX, failingTermPhrase(wordlistSPDX, failingTerm))
	}
	licenceLabel := wordlistSPDX

	return AssetRow{
		Path:      wordlistAssetDir + "/words_th.txt",
		Licence:   licenceLabel,
		Copyright: copyrightLine,
		Serves:    "folio-go shipped (embedded dictionary)",
	}, true, nil
}

// extractCopyrightLine returns the first line of text that contains the
// word "Copyright", trimmed of Markdown emphasis markers.
func extractCopyrightLine(text string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "Copyright") {
			return strings.Trim(strings.TrimSpace(line), "*"), true
		}
	}
	return "", false
}

// RenderAssets formats AssetRows as a second Markdown table, appended
// after the module-dependency table (AC25).
func RenderAssets(rows []AssetRow) string {
	var b strings.Builder
	b.WriteString("\n## Redistributed non-code assets\n\n")
	b.WriteString("AD-26, verbatim: \"Redistributed non-code assets keep their own terms and\n")
	b.WriteString("their notices.\" Every committed font binary — fixture or shipped — appears\n")
	b.WriteString("below with the licence and copyright line its accompanying LICENSE*/NOTICE*\n")
	b.WriteString("files carry (AC25, D-1.5.6). A font binary committed without both files is a\n")
	b.WriteString("build failure (`ResolveAssets`), not a silent gap.\n\n")
	if len(rows) == 0 {
		b.WriteString("_No redistributed non-code assets are committed at this commit._\n")
		return b.String()
	}
	// The Licence-column note goes BELOW the empty-corpus return, not
	// above it: it explains a column of a table, so it must not be
	// rendered over "no assets are committed", where it would describe a
	// cell no reader can see.
	b.WriteString("A Licence cell may read as a whole SPDX expression (`OFL-1.1 OR Apache-2.0`)\n")
	b.WriteString("rather than a single identifier, because the label states **what the file\n")
	b.WriteString("says** — a dual-licensed file is not attributable to one of its two\n")
	b.WriteString("licences. The gate keys on something else: **whether every option the file\n")
	b.WriteString("offers is acceptable**, term by term, against the owner's font allowlist\n")
	b.WriteString("(D-8.5.3). So a compound label beside a four-identifier allowlist is\n")
	b.WriteString("correct and is not a bug to be \"fixed\" by shortening the label or widening\n")
	b.WriteString("the list (Story 8.4j).\n\n")
	b.WriteString("| Path | Licence | Copyright | Serves |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Path, r.Licence, r.Copyright, r.Serves)
	}
	return b.String()
}

// Render formats rows as the committed Markdown manifest (AC19: "a
// committed, reviewable file"). The header states explicitly that a
// checker covering its own dependency is correct, not a bootstrap
// problem (AC20, D-1.3.6, D-1.3.9), so nobody reading it mistakes
// lint's own row for a mistake.
func Render(rows []Row) string {
	var b strings.Builder
	b.WriteString("# Third-party licence manifest\n\n")
	b.WriteString("Generated by `lint/internal/manifest` (AC19, AD-26). Every module in the\n")
	b.WriteString("resolved dependency graph of this repository's Go modules and the committed\n")
	b.WriteString("`folio-designer/package-lock.json` graph appears below with its resolved\n")
	b.WriteString("licence, which module it serves, and whether that module ships to\n")
	b.WriteString("consumers or exists only to build/test the repo. An unresolved or\n")
	b.WriteString("unrecognised licence fails the build (AC19) rather than appearing here\n")
	b.WriteString("silently; a forbidden licence (GPL, LGPL, AGPL, SSPL, or a commercial\n")
	b.WriteString("EULA) fails the build uniformly across all three modules, with no\n")
	b.WriteString("build-time-only relaxation (AC18, D-1.3.9).\n\n")
	b.WriteString("**`lint`'s own dependency, if any, is covered by this same table below.\n")
	b.WriteString("A checker covering its own dependency is correct, not a bootstrap\n")
	b.WriteString("problem** (AC20, D-1.3.6, D-1.3.9) — it must not be \"fixed\" by excluding\n")
	b.WriteString("lint's own module: AD-26 binds all, and a licence manifest that omits the\n")
	b.WriteString("one module doing the checking is exactly the self-exemption this rule\n")
	b.WriteString("exists to prevent.\n\n")
	if len(rows) == 0 {
		b.WriteString("_No external dependencies exist in any of the three module graphs at\n")
		b.WriteString("this commit (F-8). This table is not empty theatre: it is complete for\n")
		b.WriteString("what exists, and AC19 makes an unresolvable licence a build failure so it\n")
		b.WriteString("stays complete as the graph grows._\n")
		return b.String()
	}
	b.WriteString("| Module | Version | Licence | Serves | Shipped / build-time-only |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", r.Module, r.Version, r.Licence, r.Serves, r.ShippedBy)
	}
	return b.String()
}

// gitTrackedFileCount reports how many files git actually tracks under
// dir (a slash-form path relative to repoRoot) — DW-19's fix (Story
// 3.6, AC10, D-3.6.5). git's own index is the anchor: the resolver's
// own walk (above) already found real files on disk under dir; this
// asks the ONE independent source of truth for whether any of them
// were ever committed, rather than trusting the walk's own premise
// that "present on disk" means "shipped".
func gitTrackedFileCount(repoRoot, dir string) (int, error) {
	cmd := exec.Command("git", "-C", repoRoot, "ls-files", "--", filepath.FromSlash(dir))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("git ls-files -- %s: %w (%s)", dir, err, out.String())
	}
	count := 0
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}
