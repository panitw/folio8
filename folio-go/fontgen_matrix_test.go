//go:build matrix

// fontgen_matrix_test.go is Story 2.2's REGENERATION test (D-2.2.4).
//
// SEVEN of the ELEVEN faces under folio-go/fonts/ are static instances
// DERIVED from upstream variable builds, and the derived files are
// committed rather than produced at build time. (The other four — Story
// 16.8's Roboto and Story 11.1's Roboto Bold, Italic and Bold Italic — are
// not derived at all: upstream publishes static TTFs, so they are copied
// verbatim and their provenance is the identity of their source and
// shipped digests. They are accounted for by the `static-upstream` route in
// folio-go/fonts/accounting_test.go, which runs untagged on every commit.
// This file used to open by saying "the three shipped faces", which stopped
// being true at 4d2b27e — the exact stale-premise class the 16.8 story
// existed to remove, in the file that story was editing — and "three of the
// four" stopped being true again at Story 11.1, which is why the counts here
// are restated rather than left to age.) That choice is what
// keeps the shipped font from being a function of the build environment
// — a different fontTools produces a different font, which produces a
// different PDF, which is AD-22's drift class reintroduced at the asset
// layer. The cost of the choice is that nothing in an ordinary build
// re-checks that the committed bytes are still what the recorded
// derivation produces. This file is that check.
//
// WHY IT CARRIES THE `matrix` BUILD TAG. Same reasoning as
// matrix_test.go, and the same pair of properties:
//
//   - It is COMPILED on every story — CI runs `go build -tags=matrix
//     ./...` and `go vet -tags=matrix ./...` in the folio-go job — so it
//     cannot silently rot while nobody is looking at it.
//   - It is RUN at the epic gate, alongside the four-target byte-identity
//     harness, because a full run needs the upstream sources and a pinned
//     fontTools that an ordinary `go test ./...` has no business
//     requiring.
//
// WHY THIS FILE IS `package folio8_test` AND NOT `package folio8`. It reads
// its denominator out of fonts.Shipped() (below), and package `fonts`
// imports package `folio8` for its FontSet type (fonts.go:26) — a
// one-directional import AC9 makes load-bearing. `package folio8` therefore
// cannot import `fonts` without a cycle, while the external test package
// can. `repoRootForExtTest` (shipped_faces_ext_test.go:129-149) is this
// package's own repo-root walk, standing in for the white-box
// `repoRootFromTest` it cannot see.
//
// IT DOES NOT SKIP WHEN ITS INPUTS ARE ABSENT. A t.Skip here would make
// "the sources were not present" indistinguishable from "the faces
// reproduce", which is D-000.9's absence-reads-as-success in the one
// place it would be least visible. When the sources are missing the test
// FAILS, and the failure message names every URL and sha256 needed to
// obtain them.
package folio8_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/fonts"
)

// fontSourcesDir is where the upstream VARIABLE builds are expected.
// Overridable so the epic gate can point at a cache; the default matches
// the Makefile's FONT_SOURCES.
func fontSourcesDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("FOLIO8_FONT_SOURCES"); dir != "" {
		return dir
	}
	return filepath.Join(repoRootForExtTest(t), ".font-sources")
}

// TestShippedFacesReproduceFromUpstream re-runs the recorded derivation
// and asserts the committed faces come back byte-for-byte.
//
// The generator does the comparison itself, in both directions — it
// verifies each SOURCE's sha256 before instancing and each PRODUCED
// file's sha256 after, refuses to compare an operand it did not
// successfully read, and prints an explicit "N of N" coverage witness.
// That last part is not ceremony: an earlier reproducibility check in
// this story's own history printed IDENTICAL for four faces that were
// never generated, because two failed hash computations compared empty
// to empty.
func TestShippedFacesReproduceFromUpstream(t *testing.T) {
	root := repoRootForExtTest(t)
	sources := fontSourcesDir(t)

	if _, err := os.Stat(sources); err != nil {
		t.Fatalf(
			"the upstream variable font sources are not at %s, so the shipped faces' derivation cannot be "+
				"replayed.\n\n"+
				"This test FAILS rather than skipping on purpose: a skip would make \"the sources were not "+
				"present\" read exactly like \"the faces reproduce\".\n\n"+
				"Obtain them (each face's NOTICE.md carries the release URL and the source sha256):\n"+
				"  NotoSans-VF.ttf         bfb7bb691513f12e734dc346c03a03f784912432d7e3fa8e56efcf906fe86b3d\n"+
				"                          -> Noto Sans (wght=400) and Noto Sans Bold (wght=700), wdth=100\n"+
				"  NotoSans-Italic-VF.ttf  58e6e0ebd1931b29a365aa2d3e2ee9a9e831a3af7cf3ad1462d4e72154f0b291\n"+
				"                          -> Noto Sans Italic (wght=400) and Noto Sans Bold Italic (wght=700), wdth=100\n"+
				"                          Story 11.1's FOURTH source. It comes from the SAME\n"+
				"                          NotoSans-v2.015 archive as the roman VF above — the src_url\n"+
				"                          differs only after the '->', at\n"+
				"                          NotoSans/googlefonts/variable-ttf/NotoSans-Italic[wdth,wght].ttf\n"+
				"  NotoSansThai-VF.ttf     5a1c559bb539583c8a1fd99d1c5b9491e5e14478c9cd2bd0970d5c3096cc9ef8\n"+
				"                          -> Noto Sans Thai (wght=400) and Noto Sans Thai Bold (wght=700), wdth=100\n"+
				"  NotoSansSC-VF.ttf       a3041811a78c361b1de50f953c805e0244951c21c5bd412f7232ef0d899af0da\n"+
				"                          -> Noto Sans SC (wght=400; ONE axis, no wdth pin)\n\n"+
				"Four source files, SEVEN derived faces: three of the four are instanced twice, at two\n"+
				"different wght pins. The three Roboto cuts are NOT here and never will be — they take the\n"+
				"static-upstream route, accounted for by folio-go/fonts/accounting_test.go.\n\n"+
				"then either set FOLIO8_FONT_SOURCES=<dir> or place them in %s and re-run.\n"+
				"stat error: %v",
			sources, sources, err,
		)
	}

	python := os.Getenv("FOLIO8_FONTGEN_PYTHON")
	if python == "" {
		python = "python3"
	}
	script := filepath.Join(root, "tools", "fontgen", "instance_faces.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("the generator %s is missing, so the derivation is no longer recorded anywhere: %v", script, err)
	}

	cmd := exec.Command(python, script,
		"--sources", sources,
		"--repo-root", root,
		"--verify-only",
	)
	out, err := cmd.CombinedOutput()
	text := string(out)
	t.Logf("fontgen --verify-only output:\n%s", text)

	if err != nil {
		t.Fatalf(
			"the committed shipped faces do NOT reproduce from their recorded derivation.\n\n"+
				"Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change: the "+
				"shipped font is an input to every font-bearing golden in this repo, so a face that has "+
				"moved moves all of them.\n\n"+
				"Check the toolchain first — the derivation is pinned to Python 3.12.13 / fontTools 4.63.0, "+
				"and fontTools is NOT byte-deterministic across versions.\n\nexit: %v",
			err,
		)
	}

	// THE GENERATOR'S OWN WITNESS, PARSED FIRST SO EVERY CHECK BELOW CAN
	// ACTUALLY FIRE.
	//
	// This used to be a bare `strings.Contains(text, "derived and compared 3 of
	// 3 faces")` placed AHEAD of the parse, which made the parse and its error
	// branches unreachable and made `compared` always literally 3 — a
	// cross-check that could not fail, guarding a number it had already
	// assumed. Worse, a fourth UPSTREAM entry would have failed on that literal
	// FIRST, with exactly the stale `3 of 3` message the block below exists to
	// replace. So the reading comes first and the pins come after it, in order
	// of what they are about.
	reported := witnessCount.FindStringSubmatch(text)
	if reported == nil {
		t.Fatalf(
			"the generator exited 0 but printed no coverage witness this test can read. A run that compared "+
				"nothing exits 0 too (D-000.13: assert on the reported result, never on the exit code alone).\n"+
				"got:\n%s",
			text,
		)
	}
	compared, err := strconv.Atoi(reported[1])
	if err != nil {
		t.Fatalf("the generator's coverage witness reported %q as its compared count, which is not a number: %v", reported[1], err)
	}
	listed, err := strconv.Atoi(reported[2])
	if err != nil {
		t.Fatalf("the generator's coverage witness reported %q as its total, which is not a number: %v", reported[2], err)
	}

	// CLAIM 1 — THE MANIFEST'S INTERNAL SELF-CHECK: of the faces fontgen lists,
	// it derived and compared all of them. Both numbers are the generator's
	// own, so this is fontgen auditing itself, and it is a different claim from
	// anything below.
	if compared != listed {
		t.Fatalf(
			"the generator compared %d of the %d faces its own UPSTREAM manifest lists. A partial run is not a "+
				"reproduction.\ngot:\n%s",
			compared, listed, text,
		)
	}
	if !strings.Contains(text, "fontgen: OK") {
		t.Fatalf("the generator exited 0 without printing its success line.\ngot:\n%s", text)
	}

	// CLAIM 2 — THE WITNESS THIS TEST OWES, WHOSE DENOMINATOR IS THE SHIPPED SET.
	//
	// Claim 1 is about the manifest. It is NOT the claim a reader of this test
	// takes away, which is "the faces folio-go ships reproduce" — and `3 of 3`
	// said that while four faces shipped, ever since Story 16.8 added Roboto
	// (D-000.12/D-000.14). A guard whose total comes from the artifact it checks
	// can only ever report full coverage.
	//
	// So the shipped total is taken from fonts.Shipped(), the derived set is the
	// intersection of that with tools/fontgen's UPSTREAM manifest read as TEXT
	// (the generator has no machine-readable mode: --sources, --repo-root, --out
	// and --verify-only are all it offers, so asking it what it derives costs
	// ~20 MB of sources and three fontTools runs), and the remainder is reported
	// as accounted static-upstream — which the UNTAGGED
	// TestEveryShippedFaceIsAccountedForByExactlyOneRoute in folio-go/fonts
	// proves is genuinely accounted rather than merely unmatched. That guard,
	// not this one, is what refuses an unaccounted face; this line only has to
	// stop claiming a coverage it does not have.
	shipped := fonts.Shipped()
	if len(shipped) == 0 {
		t.Fatal("fonts.Shipped() is EMPTY, so `N of 0` would be the only witness this test could print")
	}
	manifestKeys := fontgenManifestKeys(t, script)
	derived := []string{}
	for key := range shipped {
		if manifestKeys[key] {
			derived = append(derived, key)
		}
	}
	sort.Strings(derived)
	if len(derived) == 0 {
		t.Fatalf(
			"none of the %d face(s) fonts.Shipped() carries appears in %s's UPSTREAM manifest, so this run "+
				"reproduced nothing that ships. Either the join key stopped being the FontSet key or the "+
				"manifest reader has gone stale — an extraction that finds nothing is not an outcome.",
			len(shipped), script,
		)
	}
	if compared != len(derived) {
		t.Fatalf(
			"the generator says it compared %d face(s), but %d of the %d face(s) fonts.Shipped() carries appear "+
				"in its UPSTREAM manifest (%s). The manifest holds a derivation for a face that is not shipped, "+
				"or this test's manifest reader is extracting the wrong keys.",
			compared, len(derived), len(shipped), strings.Join(derived, ", "),
		)
	}

	// CLAIM 3 — THE INDEPENDENT PIN, AND IT IS DELIBERATELY A LITERAL.
	//
	// Every number above is read out of the manifest or the generator, and
	// fontgen prints `total = len(UPSTREAM)`, so an expectation derived from
	// UPSTREAM would compare the manifest to itself — the tautology this whole
	// file is a correction for. One independent number therefore stays written
	// down here: how many of the SHIPPED faces are expected to be reproducible
	// at all. A manifest that quietly shrank to two entries passes every check
	// above and fails this one.
	//
	// It is pinned against the shipped intersection, not against fontgen's raw
	// output, so when it does fire the message is about the shipped set rather
	// than about `3 of 3`.
	//
	// MOVED 3 -> 7 AT STORY 11.1, deliberately and with the reviewed change
	// this comment demands: the derived population really did change. The four
	// new derived cuts are Noto Sans Bold, Noto Sans Italic, Noto Sans Bold
	// Italic and Noto Sans Thai Bold. Story 11.1's other three cuts — Roboto
	// Bold, Italic and Bold Italic — are NOT derived and must not be counted
	// here: they take the static-upstream route, so `len(shipped)` grows by
	// seven while this pin grows by four, and folio-go/fonts/accounting_test.go
	// is what proves the remaining three are accounted rather than merely
	// unmatched.
	const wantDerivedShippedFaces = 7
	if len(derived) != wantDerivedShippedFaces {
		t.Fatalf(
			"%d of the %d face(s) fonts.Shipped() carries are derived by tools/fontgen (%s); this test is "+
				"pinned to %d.\n"+
				"This literal is the one expectation here that does NOT come from the manifest, on purpose — "+
				"deriving it from UPSTREAM would compare the manifest to itself. If the derived population "+
				"really changed, that is a reviewed change to what this repository can reproduce: move the pin "+
				"deliberately, and check folio-go/fonts/accounting_test.go agrees about the other route.",
			len(derived), len(shipped), strings.Join(derived, ", "), wantDerivedShippedFaces,
		)
	}

	staticUpstream := len(shipped) - len(derived)
	t.Logf(
		"fontgen: derived and compared %d of %d shipped faces (%s); %d accounted static-upstream",
		len(derived), len(shipped), strings.Join(derived, ", "), staticUpstream,
	)
}

// witnessCount reads the generator's own compared-count out of its
// coverage line, so this test's cross-check is against the number fontgen
// printed rather than against a second copy of it.
var witnessCount = regexp.MustCompile(`derived and compared (\d+) of (\d+) faces`)

// THE MANIFEST, READ AS TEXT, AND THROWING IF IT EXTRACTS NOTHING.
// `UPSTREAM[i]["key"]` is byte-identical to the fonts.Shipped() map key —
// measured, exact equality, no normalisation — so the join needs no table
// and introduces no fourth authority (D-000.14).
//
// THE SAME READER EXISTS IN folio-go/fonts/accounting_test.go, AND THE
// DUPLICATION IS FORCED BY THE PACKAGE BOUNDARY RATHER THAN CHOSEN. This file
// must be `package folio8_test` (it imports `fonts`, and `fonts` imports
// `folio8`), and Go gives one test package no access to another test package's
// identifiers; sharing it would mean adding exported production API to a
// package whose stated purpose is to hold font data, for two tests. What makes
// two copies safe is that neither can rot quietly: both t.Fatalf on an
// unmatched UPSTREAM block or an empty key set, so a format change fails
// LOUDLY in both places rather than leaving one silently extracting nothing.
var (
	upstreamBlock = regexp.MustCompile(`(?s)\nUPSTREAM = \[\n(.*?)\n\]\n`)
	upstreamKey   = regexp.MustCompile(`"key":\s*"([^"]+)"`)
)

func fontgenManifestKeys(t *testing.T, script string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("could not read the fontgen manifest at %s: %v", script, err)
	}
	block := upstreamBlock.FindStringSubmatch(string(raw))
	if block == nil {
		t.Fatalf(
			"%s no longer declares an `UPSTREAM = [ … ]` list this test can read. Re-derive the reader rather "+
				"than dropping the denominator back onto the manifest's own count.",
			script,
		)
	}
	keys := map[string]bool{}
	for _, match := range upstreamKey.FindAllStringSubmatch(block[1], -1) {
		keys[match[1]] = true
	}
	if len(keys) == 0 {
		t.Fatalf("%s declares an UPSTREAM list from which no `\"key\"` field could be extracted", script)
	}
	return keys
}
