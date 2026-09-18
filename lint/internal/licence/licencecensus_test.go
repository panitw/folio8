package licence

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// censusVerdict is one pinned population verdict: a licence text this
// repository relies on, and the (family, id) the classifier MUST give
// it.
type censusVerdict struct {
	where  string
	family Family
	id     string
}

// pinnedCensus is the WHOLE licence population of this repository with
// its verdict written down — 82 rows: 73 committed LICENSE*/COPYING
// files (every row whose `where` is a repository-relative path) plus the
// 9 dependency licences the three Go module graphs resolve to (every row
// whose `where` begins "dep ").
//
// BOTH NUMBERS WERE ALREADY WRONG BY ONE BEFORE STORY 11.1 TOUCHED THEM:
// this sentence read "67 rows: 58 committed" while the table held 68 and
// 59. The figures above were COUNTED off the rows, not obtained by adding
// fourteen to a number that was already wrong — which is the same defect
// the next paragraph records, reaching its second instance here.
//
// THOSE COUNTS ARE PROPERTIES OF THE ROWS BELOW, not a second source of
// truth, and this sentence is the ONLY place either is typed by hand.
// Prose rots: this header read "48 committed" for the whole life of the
// ten rows Story 16.1a announced three paragraphs down, so the addition
// and the total it invalidated sat in the same comment, disagreeing.
// Where the test needs the committed count it therefore DERIVES it from
// the rows (committedPinCount, below) rather than reading a number a
// person typed.
//
// IT GREW BY 22 AT STORY 8.5: the catalogue's 21 faces, and the compound
// fixture D-8.4j.2 requires. That is the largest single addition this
// table has taken, and every row is written out by hand for the reason
// the paragraph below gives — a new redistributed licence is exactly the
// event that must not land unrecorded.
//
// AND BY 10 AT STORY 16.1a: the local face tier batch. Nine are OFL-1.1
// and one — `robotoslab/LICENSE-APACHE.txt` — is the FIRST Apache-2.0
// FONT ASSET this repository has ever carried, so it is the first row
// that exercises the second of the owner's four ids (D-8.5.3) from the
// asset side rather than from a dependency or a fixture.
//
// AND BY 14 AT STORY 11.1: the seven weighted and sloped cuts, each
// committed TWICE — once under folio8-go/fonts/ and once under
// folio8-designer/public/fonts/ — because AD-26 binds the directory, not
// the family, so a cut that ships on both sides carries its OFL text on
// both sides. All fourteen are OFL-1.1 and all fourteen are the same
// upstream text as the Regular beside them; they are pinned one by one
// anyway, in path order with the rest, because the population is what
// this table records and a duplicated text is still a redistributed
// file. MEASURED, not assumed: with the rows absent the cross-check at
// the end of this test named exactly these fourteen paths and printed
// (permissive, "OFL-1.1") for every one.
//
// THIS TEST IS WHY `-count=1` IS NOT OPTIONAL. The census walks the
// filesystem, so its verdict changes when FILES change and not when Go
// code does; `go test ./...` will happily serve a cached PASS from
// before a batch of licence texts landed. A batch that adds redistributed
// licences and reads a cached green has measured nothing.
//
// THIS TABLE IS THE OBSERVATION. It replaces a differential that stopped
// observing anything the moment ClassifyLicenceText's body became a call
// to classifyByAllSignals: from that commit the census compared a
// function to ITSELF, so its "0 of 35 changed verdict" line was true by
// construction and could not have reported otherwise. MEASURED: with the
// SIL OPEN FONT LICENSE entry's requiredVersion set to "VERSION 9.9" —
// which reclassifies all eleven committed OFL files to (unknown, "") and
// fails the shipped asset gate outright — the differential census still
// printed "0 change verdict" and PASSED. A guard whose comment claims a
// safety it does not have is this story's own subject (D-8.0.1), and it
// reached instance four here, in the test written to catch instance
// three.
//
// The table is TEST-OWNED and written out longhand, on D-8.4i.3's
// reasoning about the allowlist: an expectation DERIVED from the thing
// it checks passes any edit to that thing. Adding a dependency or a
// committed licence file therefore requires recording its verdict here.
// That is not friction to be engineered away — under AD-26 a new
// redistributed licence is exactly the event that must not land
// unrecorded.
var pinnedCensus = []censusVerdict{
	{"LICENSE", FamilyPermissive, "MIT"},
	// SPEC-client-libraries story 5: the npm package redistributes the
	// repository's own MIT terms as its own LICENSE file, because a tarball
	// installed from a registry carries no repository around it. The bytes
	// are a copy of the root LICENSE above, and the row is pinned
	// separately because AD-26 records verdicts about PATHS this repository
	// redistributes, not about distinct texts. The font licences that ship
	// beside the faces in that package are copied at build time from
	// folio8-go/fonts/ (already pinned below) and are not tracked here.
	{"folio-js/LICENSE", FamilyPermissive, "MIT"},
	// STORY 8.5'S CATALOGUE, 21 NEW COMMITTED LICENCE TEXTS, PINNED ONE BY
	// ONE. They are the first faces this repository has ever put through the
	// fail-closed asset gate in bulk, and two of them —
	// ubuntusans/ and ubuntusansmono/ — are the first Ubuntu-font-1.0 ASSET
	// this repository has ever carried, closing the gap classify.go:167-171
	// records ("there is no analogue of TestCommittedOFLTextClassifiesAsOFL11
	// to write until Story 8.5 lands a face under it"). Written out longhand,
	// on D-8.4i.3's reasoning: an expectation derived from the thing it
	// checks passes any edit to that thing.
	// STORY 16.1a'S BATCH, TEN NEW COMMITTED LICENCE TEXTS, PINNED ONE BY
	// ONE and kept in path order with the rest. Each is the unmodified
	// upstream text fetched from the same pinned artifact as the binary
	// beside it, never a hand-copy, so a row here is a verdict about bytes
	// this repository actually redistributes (AD-26).
	{"folio8-designer/public/fonts/arimo/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/cascadiacode/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/cascadiamono/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/cousine/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/dmsans/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/firacode/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/geist/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/geistmono/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/ibmplexmono/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/ibmplexsans/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/ibmplexsansthai/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/intelonemono/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/inter/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/interdisplay/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/jetbrainsmono/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/literata/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/lora/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/montserrat/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosans-bold/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosans-bolditalic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosans-italic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosans/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosanssc/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosansthai-bold/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosansthai/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notosansthailooped/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notoserif/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/notoserifthai/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/opensans/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/oswald/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/plusjakartasans/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/roboto-bold/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/roboto-bolditalic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/roboto-italic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/roboto/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/robotocondensed/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/robotomono/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	// THE FIRST Apache-2.0 FONT ASSET IN THE REPOSITORY. `googlefonts/robotoslab`
	// ships an Apache-2.0 LICENSE.txt and the binary's own nameID 13 reads
	// "Licensed under the Apache License, Version 2.0" — the two agree, which is
	// what font-catalogue.test.ts's licenceSignatures checks from the other side.
	{"folio8-designer/public/fonts/robotoslab/LICENSE-APACHE.txt", FamilyPermissive, "Apache-2.0"},
	{"folio8-designer/public/fonts/sourcecodepro/LICENSE-OFL.md", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/sourcesans3/LICENSE-OFL.md", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/sourceserif4/LICENSE-OFL.md", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/spacegrotesk/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-designer/public/fonts/ubuntusans/LICENSE-UFL.txt", FamilyPermissive, "Ubuntu-font-1.0"},
	{"folio8-designer/public/fonts/ubuntusansmono/LICENSE-UFL.txt", FamilyPermissive, "Ubuntu-font-1.0"},
	{"folio8-designer/third-party-notices/pdfjs-dist/LICENSE-APACHE-2.0", FamilyPermissive, "Apache-2.0"},
	{"folio8-designer/third-party-notices/pdfjs-dist/LICENSE-CMAPS", FamilyPermissive, "BSD-3-Clause"},
	{"folio8-designer/third-party-notices/pdfjs-dist/LICENSE-LIBERATION", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/notosans-bold/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/notosans-bolditalic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/notosans-italic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/notosans/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/notosanssc/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/notosansthai-bold/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/notosansthai/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/roboto-bold/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/roboto-bolditalic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/roboto-italic/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/fonts/roboto/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/internal/text/wordlist/LICENSE-CC0-1.0.txt", FamilyPermissive, "CC0-1.0"},
	{"folio8-go/testdata/fonts/LICENSE-Roboto.txt", FamilyPermissive, "Apache-2.0"},
	{"folio8-go/testdata/fonts/notosansthai-variable-testonly/LICENSE-OFL.txt", FamilyPermissive, "OFL-1.1"},
	{"folio8-go/testdata/lint/wordlist-assets/compliant/folio8-go/internal/text/wordlist/LICENSE-CC0-1.0.txt", FamilyUnknown, ""},
	{"folio8-go/testdata/lint/wordlist-assets/violating/folio8-go/internal/text/wordlist/LICENSE-CC0-1.0.txt", FamilyUnknown, ""},
	{"lint/testdata/licence/copyleft/example.test/agpl-lib/LICENSE", FamilyCopyleft, "AGPL-3.0-only"},
	{"lint/testdata/licence/copyleft/example.test/gpl-lib/LICENSE", FamilyCopyleft, "GPL-3.0-only"},
	{"lint/testdata/licence/copyleft/example.test/lgpl-lib/LICENSE", FamilyCopyleft, "LGPL-3.0-only"},
	{"lint/testdata/licence/copyleft/example.test/sspl-lib/LICENSE", FamilyCopyleft, "SSPL-1.0"},
	{"lint/testdata/licence/permissive/example.test/apache-lib/LICENSE", FamilyPermissive, "Apache-2.0"},
	{"lint/testdata/licence/permissive/example.test/bsd-lib/LICENSE", FamilyPermissive, "BSD-3-Clause"},
	// D-8.4j.2'S COMPOUND CASE, seeded as a fixture because no procurable
	// font carries one the four-id allowlist admits (Design Note 3, measured
	// before the decision per D-000.12: Hack is MIT + Bitstream Vera and
	// Public Sans is OFL-1.1 + CC0-1.0, and BOTH must fail the gate). The id
	// pinned here is the WHOLE EXPRESSION, which is what Story 8.4j's
	// whole-line capture returns and what per-term admission then tests; a
	// regression to the one-token capture reports "MIT" and reds this row.
	{"lint/testdata/licence/permissive/example.test/compound-lib/LICENSE", FamilyPermissive, "MIT OR Apache-2.0"},
	{"lint/testdata/licence/permissive/example.test/mit-lib/LICENSE", FamilyPermissive, "MIT"},
	{"lint/testdata/licence/permissive/example.test/ufl-lib/LICENSE", FamilyPermissive, "Ubuntu-font-1.0"},
	{"dep folio8-go -> github.com/boxesandglue/textshape", FamilyPermissive, "MIT"},
	{"dep lint -> github.com/google/go-cmp", FamilyPermissive, "BSD-3-Clause"},
	{"dep lint -> github.com/yuin/goldmark", FamilyPermissive, "MIT"},
	{"dep lint -> golang.org/x/mod", FamilyPermissive, "BSD-3-Clause"},
	{"dep lint -> golang.org/x/net", FamilyPermissive, "BSD-3-Clause"},
	{"dep lint -> golang.org/x/sync", FamilyPermissive, "BSD-3-Clause"},
	{"dep lint -> golang.org/x/sys", FamilyPermissive, "BSD-3-Clause"},
	{"dep lint -> golang.org/x/telemetry", FamilyPermissive, "BSD-3-Clause"},
	{"dep lint -> golang.org/x/tools", FamilyPermissive, "BSD-3-Clause"},
}

// committedPinCount is how many pinnedCensus rows pin a COMMITTED licence
// file rather than a dependency licence. The dependency half is the half
// with a synthetic `where`: TestLicenceSignalCensus builds it as
// "dep "+module+" -> "+path, so the prefix is what tells the two halves
// apart, and everything without it is a repository-relative path that
// `git ls-files` can produce.
//
// THAT DISCRIMINATOR IS SOUND BY OBSERVATION, NOT BY CONSTRUCTION.
// Nothing stops a committed path from beginning "dep ". A pin at
// `dep-vendor-dir/LICENSE` is counted as committed and raises the floor
// to 59; the same name with a SPACE, `dep vendor-dir/LICENSE`, is filed
// as a dependency instead and lowers the floor by one — silently, and in
// the direction that LOOSENS it. `git ls-files` prints such a path
// unquoted, so it would reach `where` verbatim. Today zero tracked paths
// begin "dep" and exactly one contains a space, so this is latent rather
// than live; it is written down because the failure would not announce
// itself.
//
// This exists so the vacuity floor below is derived from the table
// instead of being a second hand-typed number. It is NOT a derivation
// that weakens the test: the count it produces bounds a population walked
// off the filesystem, and D-8.4i.3's objection is to an expectation
// derived from the THING IT CHECKS — which this is not.
func committedPinCount() int {
	n := 0
	for _, v := range pinnedCensus {
		if !strings.HasPrefix(v.where, "dep ") {
			n++
		}
	}
	return n
}

// TestLicenceSignalCensus is Story 8.4i's task 1 — D-8.4i.1's
// population census, and D-8.4i.6's hard constraint that the gate must
// not become fatal in the same commit that first measures the
// population.
//
// AS COMMITTED AT 2e9365e it was report-only and DIFFERENTIAL: the
// collect-all-signals rule ran beside the then-shipped first-match
// switch over the whole population, and 0 of 35 texts changed verdict.
// That measurement is a fact about THAT COMMIT, where the two
// classifiers were genuinely different code, and it is what licensed
// the next commit to make refusals fatal. It is recorded in the story's
// Delivery Log and in DW-117's closing note, attributed there.
//
// AS IT STANDS NOW the differential is meaningless — both columns would
// be the same function — so the test PINS the population's verdicts
// against the table above instead. What it asserts today: every licence
// text this repository redistributes or depends on classifies exactly as
// recorded, no population member is missing from the table, and no table
// entry has silently lost its file.
//
// WHY THE CENSUS EXISTS AT ALL. ClassifyLicenceText is shared between
// the ASSET path (lint/internal/manifest/manifest.go, whose files are
// visible in this repository) and the DEPENDENCY path
// (lint/internal/rules/licencegraph.go, whose files are not visible here
// at all). NEITHER CALLER LIVES IN THIS PACKAGE, which is why both are
// written as full paths: an earlier draft named them bare, as
// "manifest.go" and "licencegraph.go", and a reader who greps for those
// inside internal/licence finds nothing and concludes the sentence has
// rotted. It has not — but the population SIZES it used to quote had,
// twice over, so they are deliberately not restated. This names WHERE
// each population is defined, not HOW BIG it is; a reader who wants a
// count should go to those two files and take one, against something
// that maintains itself. If a change reds something legitimate here, the
// answer is to
// HALT and route it to the engineering lead — never to weaken the rule,
// narrow the population or exempt a file. Moving the bar to fit the
// instrument (D-8.5.10) is the failure this whole thread exists to stop.
func TestLicenceSignalCensus(t *testing.T) {
	root := repoRootForCensus(t)

	want := make(map[string]censusVerdict, len(pinnedCensus))
	for _, v := range pinnedCensus {
		if _, dup := want[v.where]; dup {
			t.Fatalf("pinnedCensus lists %q twice", v.where)
		}
		want[v.where] = v
	}
	seen := map[string]bool{}
	measured := 0

	record := func(where, text string) {
		measured++
		family, id := ClassifyLicenceText(text)
		seen[where] = true
		pin, ok := want[where]
		if !ok {
			t.Errorf("%s: classifies as (%v, %q) but is NOT in pinnedCensus. A licence text this "+
				"repository redistributes or depends on must have its verdict RECORDED (AD-26): add it "+
				"to the table — do not delete it from the population.", where, family, id)
			return
		}
		if family != pin.family || id != pin.id {
			t.Errorf("%s: classifies as (%v, %q), pinned as (%v, %q). If the new verdict is CORRECT, the "+
				"table is the place that records the change and says who decided it. If it is not, this is "+
				"a HALT and a finding for the engineering lead — never a reason to weaken the rule, narrow "+
				"the population or exempt a file (D-8.5.10).",
				where, family, id, pin.family, pin.id)
			return
		}
		t.Logf("    %-72s (%v, %q)", where, family, id)
	}

	// --- population A: every committed LICENSE*/COPYING* file ---
	//
	// Deliberately a SUPERSET of the files the font/wordlist asset gate
	// classifies and of the fixtures under lint/testdata/licence: a census
	// that enumerated only the files it expected to find would not be a
	// census. The third-party notice texts and the repository's own LICENSE
	// are carried too, at no cost. Those sub-populations are named and not
	// counted here on purpose — the counts this paragraph used to give
	// ("the 12 asset files, the 8 lint fixtures") were both wrong by the
	// time anyone read them again.
	//
	// THE FLOOR BELOW IS VACUITY DETECTION, AND IT CATCHES EXACTLY ONE
	// DIRECTION: UNDER-COLLECTION. It fails once, early and legibly, when
	// the walk returns FEWER files than the table records — a `git ls-files`
	// that returns nothing, a root that lands on a smaller tree, a basename
	// regex that stops matching — instead of letting a collapsed walk emit
	// one "was NOT found in the population" error per pinned row.
	//
	// IT IS BLIND TO THE OPPOSITE BREAK. That is a bound to state, not a
	// gap to guard here. MEASURED: broaden the basename regex to also match
	// README and Makefile — a walk broken toward collecting TOO MUCH — and
	// this floor stays silent while the cross-check below prints a 32-line
	// wall of "is NOT in pinnedCensus", the exact wall the floor exists to
	// pre-empt. A root resolving HIGHER behaves the same way, for the same
	// reason: more files, not fewer. Over-collection belongs to the
	// cross-check, which names each file; `<` does not duplicate it.
	//
	// IT COUNTS ENTRIES, NOT DISTINCT FILES, and its zero slack today is a
	// coincidence of today's numbers. MEASURED: replace ten walked paths
	// with duplicates of an eleventh — len(committed) stays 58, the floor
	// passes, and ten real licence files go unmeasured with only the
	// cross-check to say so. `git ls-files` does not emit duplicates, so
	// that shape is not the live risk; the general one is. Population A is
	// a DESIGNED SUPERSET of the pins, so the day someone commits N licence
	// files without pinning them, the walk sits N above the table and a
	// walk that then loses N files clears this floor again. That is the
	// same fuse the literal 20 lit, only shorter: deriving caps it at N and
	// holds it at zero whenever the test is green, which is the most a
	// cheap early bail can honestly promise.
	//
	// WHY DERIVED AT ALL. The literal it replaces (20) was written against
	// a population this table has since nearly tripled, so it went on
	// passing while bounding nothing; a literal 58 would only re-arm that
	// on a longer fuse. Deriving is honest here because the two sides are
	// produced INDEPENDENTLY: one side WALKS THE FILESYSTEM, the other is
	// TYPED BY A PERSON.
	committed := committedLicenceFiles(t, root)
	if pinned := committedPinCount(); len(committed) < pinned {
		t.Fatalf("census walked up only %d committed licence files, but pinnedCensus records %d of them — "+
			"the walk itself looks broken (repository root, git ls-files, or the basename regex), and this "+
			"measurement would be vacuous. This is the vacuity check, NOT the completeness check: if the "+
			"walk is sound and a pinned file was genuinely removed, the per-file cross-check at the end of "+
			"this test names it and says what to do about it.", len(committed), pinned)
	}
	for _, rel := range committed {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read committed licence file %s: %v", rel, err)
		}
		record(rel, string(data))
	}

	// --- population B: every dependency LICENSE the Go module graphs
	// resolve to ---
	//
	// This is the half nobody can see by reading the repository, and the
	// half D-8.4i.1's Block If was written about. hashmatrix resolves
	// zero non-main modules; its scan is vacuous, which is itself worth
	// recording rather than assuming.
	deps := 0
	for _, mod := range []string{"folio8-go", "lint", "hashmatrix"} {
		modules, err := ResolveGraph(filepath.Join(root, mod))
		if err != nil {
			t.Fatalf("resolve %s module graph: %v", mod, err)
		}
		t.Logf("module graph %-10s resolves %d non-main module(s)", mod, len(modules))
		for _, m := range modules {
			text, ok := ReadLicenceText(m.Dir)
			if !ok {
				record("dep "+mod+" -> "+m.Path+" (NO LICENCE FILE)", "")
				continue
			}
			deps++
			record("dep "+mod+" -> "+m.Path, text)
		}
	}
	if deps == 0 {
		t.Fatal("census resolved no dependency licence text at all — the population this rule is least " +
			"able to inspect by eye is exactly the one that must not go unmeasured (D-8.4i.1)")
	}

	// --- the record ---
	for _, v := range pinnedCensus {
		if !seen[v.where] {
			t.Errorf("%s is pinned in pinnedCensus but was NOT found in the population. A licence file "+
				"disappearing silently is how a census stops being a census — if it was removed "+
				"deliberately, remove its pin in the same change.", v.where)
		}
	}
	t.Logf("CENSUS: %d licence texts measured (%d committed files + %d dependency licences), all matching "+
		"their pinned verdicts", measured, len(committed), deps)
}

// committedLicenceFiles lists every tracked file whose basename names a
// licence text. Tracked, not walked: the population under audit is what
// the repository redistributes.
func committedLicenceFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files in %s: %v", root, err)
	}
	licenceBasename := regexp.MustCompile(`^(LICENSE|LICENCE|COPYING)`)
	var rels []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		base := path.Base(line)
		if !licenceBasename.MatchString(base) {
			continue
		}
		if strings.HasSuffix(base, ".go") {
			continue
		}
		rels = append(rels, line)
	}
	return rels
}

// repoRootForCensus walks up from this package's directory to the
// repository root, identified by the .git directory rather than by a
// hard-coded number of parent hops.
func repoRootForCensus(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no repository root above %s", dir)
		}
		dir = parent
	}
}
