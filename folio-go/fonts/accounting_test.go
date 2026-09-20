package fonts

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// THE ACCOUNTING GUARD — WHAT WOULD HAVE TO CHANGE FOR IT TO FAIL.
//
// Story 16.8 grew fonts.Shipped() from three faces to four. Four things
// closed over the three-face state; one failed loudly. The rest did not,
// and the reason is the same in every case: their denominator came from
// the artifact they were checking rather than from the shipped set. A
// guard whose total is `len(its own list)` can only ever report N of N.
//
// So both tests in this file take their denominator from fonts.Shipped()
// and from nothing else, and both fail in BOTH directions — a face
// shipped with no record, and a record for a face that is not shipped.
// They are modelled on TestShippedSpecCoversEverythingShipped
// (folio-go/shipped_faces_test.go:389-444): a cardinality pre-check, a
// zero-byte non-vacuity floor, and a fresh on-disk read.
//
// WHY THIS FILE CARRIES NO BUILD TAG. It is pure file reading — no
// fontTools, no venv, no network, no upstream sources — so it runs in
// `go test ./...` on every commit. folio-go/fontgen_matrix_test.go is
// tagged `matrix` because REPRODUCING a face genuinely needs Python
// 3.12.13 and fontTools 4.63.0, which CI does not have and cannot
// acquire. Putting this cheap guard inside that expensive suite would be
// D-000.11 exactly: the check that would have caught the drift, gated
// behind the toolchain nobody runs.
//
// EXACTLY TWO NAMED ROUTES, AND NO FALLBACK BUCKET. A shipped face is
// either `derived` — its FontSet key appears in tools/fontgen's UPSTREAM
// manifest, so folio-go/fontgen_matrix_test.go replays its instancing —
// or `static-upstream`, its own NOTICE.md recording `copied unmodified,
// no derivation`. A face in NEITHER fails; a face in BOTH fails. There is
// no default arm, because a default arm is how an unaccounted face gets
// silently counted as accounted.

// noticeRecord is one directory's on-disk provenance record, read fresh
// from that directory's own NOTICE.md and its own binary. Nothing here is
// tabulated in Go: the face->directory relation is DERIVED by hashing the
// binary each NOTICE names and joining it to fonts.Shipped() BY BYTES
// (D-000.14 — a hand-written name->directory table would be a fourth
// authority and would make the join a tautology).
type noticeRecord struct {
	dir            string // e.g. "roboto", relative to folio-go/fonts/
	noticePath     string // e.g. "roboto/NOTICE.md"
	binaryPath     string // the file the NOTICE's **Shipped file** row names
	recordedDigest string // the NOTICE's recorded sha256 of the SHIPPED file
	recordedSource string // the NOTICE's recorded sha256 of the SOURCE (upstream) file
	recordedSize   int    // the NOTICE's recorded Size row, in bytes
	derivative     bool   // the NOTICE declares the file a DERIVATIVE of an upstream variable build
	staticUpstream bool   // the NOTICE records `copied unmodified, no derivation`
	diskDigest     string // sha256 of the binary as it is on disk right now
	diskSize       int
}

// THE THREE ROWS THAT ARE IDENTICAL IN ALL FOUR NOTICES, and therefore the
// only rows it is safe to parse. Each is anchored to a whole line so a
// sentence quoting one of these phrases in prose cannot be read as the row.
var (
	shippedFileRow   = regexp.MustCompile("(?m)^\\| \\*\\*Shipped file\\*\\* \\| `([^`]+)` \\|$")
	shippedDigestRow = regexp.MustCompile("(?m)^\\| \\*\\*sha256 of the SHIPPED \\(produced\\) file\\*\\* \\| `([0-9a-f]{64})` \\|$")
	sourceDigestRow  = regexp.MustCompile("(?m)^\\| \\*\\*sha256 of the SOURCE \\(upstream\\) file\\*\\* \\| `([0-9a-f]{64})` \\|$")
	shippedSizeRow   = regexp.MustCompile(`(?m)^\| Size \| ([0-9][0-9,]*) bytes \|$`)
)

// THE DERIVATION STATEMENT IS *NOT* UNIFORM, and pretending it is would be
// the same defect this file exists to remove. It is a BLOCKQUOTE in the
// three Noto notices ("> **This file is a DERIVATIVE of the upstream
// release, not the upstream\n> file itself.**") and a MULTI-LINE TABLE
// CELL in roboto's ("| Relation to source | **copied unmodified, no
// derivation** — all three digests recorded in this\nproject … |"). So the
// two shapes get one matcher each, neither of them line-anchored, and a
// NOTICE must carry exactly one of them.
//
// The static-upstream phrase is matched IN FULL. roboto/NOTICE.md also
// says its LICENCE file is "copied unmodified from" the designer's copy —
// a true sentence about a different file — and a matcher looking only for
// `copied unmodified` would read that as a derivation statement.
var (
	derivativeStatement     = regexp.MustCompile(`This file is a DERIVATIVE of the upstream release`)
	staticUpstreamStatement = regexp.MustCompile(`copied unmodified, no derivation`)
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// exactlyOneRow is the parser contract for every NOTICE row this file
// reads: 0 matches and 2+ matches are BOTH failures, and neither is ever
// read as "no constraint". A parser that extracts nothing is the failure
// mode being designed against, not an outcome — the same rule
// folio-designer/src/font-catalogue.test.ts's `recordedShippedDigest`,
// `recordedShippedSize` and `recordedArchive` and
// e2e/font-embed-boundary.spec.ts's `readBoundarySentences` already apply
// on the TS side.
//
// ⚠ CITED BY NAME, NOT BY LINE, AND THAT IS THE FIX RATHER THAN A STYLE
// CHOICE. This read ":139-152" and ":130-136" and both had drifted to
// unrelated code long before spec-install-all-face-cuts story 3 moved
// every line in those files again. A line citation into another
// language's test file is a fact with no gate behind it and rots on the
// next edit either side; a symbol is greppable and moves with the thing
// it names.
func exactlyOneRow(t *testing.T, re *regexp.Regexp, source, path, what string) string {
	t.Helper()
	found := re.FindAllStringSubmatch(source, -1)
	if len(found) != 1 {
		t.Fatalf(
			"%s: the %s row matched %d times, and this parser accepts exactly one.\n"+
				"A row that matches ZERO times must never be read as \"this NOTICE places no constraint\", "+
				"and a row that matches TWICE means two different values are both claiming to be the record.\n"+
				"Either restore the row to the shape every other folio-go/fonts/*/NOTICE.md uses, or re-derive "+
				"this parser against the new shape — do not delete the check.\npattern: %s",
			path, what, len(found), re.String(),
		)
	}
	return found[0][1]
}

// fontBinaryExtensions is what makes a subdirectory a FACE directory. The
// walk is gated on holding one of these rather than on merely being a
// directory: a future folio-go/fonts/testdata/ — Go's conventional fixture
// location — would otherwise fail this guard for a reason that has nothing
// to do with provenance, and a guard that fires on the wrong thing gets
// deleted rather than fixed.
var fontBinaryExtensions = map[string]bool{".ttf": true, ".otf": true, ".ttc": true, ".woff": true, ".woff2": true}

// readNoticeRecords walks every subdirectory of folio-go/fonts/ that holds a
// font binary. It walks rather than consulting a list for the same reason the
// join below hashes rather than tabulates: a list is a second authority that a
// new face can be added without touching.
//
// NOTHING IS SKIPPED SILENTLY, WHICH IS THE POINT OF GATING RATHER THAN
// FILTERING. A directory holding a font binary and no NOTICE.md is a hard
// failure (AD-26, AC25: "redistributed non-code assets keep their own terms and
// their notices"), and a directory holding a binary its NOTICE does not name is
// a hard failure too — that is how a second face dropped beside a recorded one
// would otherwise ride along with no provenance of its own. The only thing the
// gate lets through is a directory with no font binary in it at all, which by
// construction cannot be a shipped face.
//
// ONE LEVEL, NOT A RECURSIVE WALK: folio-go/fonts/ is flat by layout — one
// directory per face — and the join below is total over fonts.Shipped(), so a
// face hidden one level deeper would still fail as an unaccounted shipped face
// rather than pass unnoticed.
func readNoticeRecords(t *testing.T) []noticeRecord {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not read the folio-go/fonts/ package directory: %v", err)
	}
	records := []noticeRecord{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		inner, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("could not read folio-go/fonts/%s/: %v", dir, err)
		}
		binaries := []string{}
		for _, file := range inner {
			if !file.IsDir() && fontBinaryExtensions[strings.ToLower(filepath.Ext(file.Name()))] {
				binaries = append(binaries, file.Name())
			}
		}
		if len(binaries) == 0 {
			continue
		}
		sort.Strings(binaries)
		noticePath := filepath.Join(dir, "NOTICE.md")
		raw, err := os.ReadFile(noticePath)
		if err != nil {
			t.Fatalf(
				"folio-go/fonts/%s/ holds font binar(y/ies) %s but no readable NOTICE.md (%v).\n"+
					"A redistributed non-code asset carries its own terms and its own notice (AD-26, AC25) — this "+
					"test refuses to skip the directory instead, because a skipped directory is exactly how a "+
					"shipped face ends up with no provenance record.",
				dir, strings.Join(binaries, ", "), err,
			)
		}
		text := string(raw)

		record := noticeRecord{
			dir:            dir,
			noticePath:     noticePath,
			recordedDigest: exactlyOneRow(t, shippedDigestRow, text, noticePath, "**sha256 of the SHIPPED (produced) file**"),
			recordedSource: exactlyOneRow(t, sourceDigestRow, text, noticePath, "**sha256 of the SOURCE (upstream) file**"),
		}
		record.binaryPath = filepath.Join(dir, exactlyOneRow(t, shippedFileRow, text, noticePath, "**Shipped file**"))
		sizeText := exactlyOneRow(t, shippedSizeRow, text, noticePath, "Size")
		size, err := strconv.Atoi(strings.ReplaceAll(sizeText, ",", ""))
		if err != nil {
			t.Fatalf("%s: the Size row reads %q, which is not a byte count: %v", noticePath, sizeText, err)
		}
		record.recordedSize = size

		// THE DERIVATION STATEMENT IS READ HERE AND ACCOUNTED FOR IN
		// TestEveryShippedFaceIsAccountedForByExactlyOneRoute, not validated
		// here. Reading it and then deciding the route from the manifest alone
		// would leave the one on-disk sentence about derivation asserting
		// nothing — so "both shapes present", "neither present" and
		// "disagrees with the manifest" are all failure arms of the ROUTE
		// accounting, where the manifest is in hand to name the disagreement.
		//
		// What this parser owns is the same rule every other row here follows:
		// a statement that appears TWICE is two records that can disagree, and
		// is refused before anything reads either of them.
		derivativeHits := len(derivativeStatement.FindAllString(text, -1))
		staticHits := len(staticUpstreamStatement.FindAllString(text, -1))
		if derivativeHits > 1 || staticHits > 1 {
			t.Fatalf(
				"%s repeats its derivation statement (%d DERIVATIVE, %d static-upstream). At most one of each "+
					"is the record; two are two records that can disagree.",
				noticePath, derivativeHits, staticHits,
			)
		}
		record.derivative = derivativeHits == 1
		record.staticUpstream = staticHits == 1

		// NOTHING RIDES ALONG UNRECORDED. The NOTICE names one shipped file;
		// if the directory holds a font binary it does not name, that binary
		// has no provenance record of its own and this walk would have skipped
		// straight past it.
		named := filepath.Base(record.binaryPath)
		if len(binaries) != 1 || binaries[0] != named {
			t.Fatalf(
				"folio-go/fonts/%s/ holds font binar(y/ies) %s, but %s names only `%s` as its **Shipped file**. "+
					"Every font binary under folio-go/fonts/ needs its own directory and its own NOTICE — a "+
					"second one beside a recorded face is a face with no provenance record at all.",
				dir, strings.Join(binaries, ", "), noticePath, named,
			)
		}

		onDisk, err := os.ReadFile(record.binaryPath)
		if err != nil {
			t.Fatalf(
				"%s names `%s` as its **Shipped file**, but that file cannot be read: %v",
				noticePath, record.binaryPath, err,
			)
		}
		if len(onDisk) == 0 {
			t.Fatalf("%s: the shipped file %s is ZERO bytes — every comparison against it would be vacuous", noticePath, record.binaryPath)
		}
		record.diskDigest = sha256Hex(onDisk)
		record.diskSize = len(onDisk)
		records = append(records, record)
	}
	if len(records) == 0 {
		t.Fatal(
			"the walk of folio-go/fonts/ found NO face directory at all, so everything below it would be " +
				"vacuously true. This test reports that as a failure rather than as a pass over an empty set.",
		)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].dir < records[j].dir })
	return withoutUnshippedFaceDirs(t, records)
}

// withoutUnshippedFaceDirs drops the face directories THIS BUILD does not
// embed, and it is the one seam the `nocjkface` build needs in this file
// (spec-deferred-offline-cache, CAP-6).
//
// WHY A DROP AND NOT A SKIP EARLIER IN THE WALK. The directory is still
// there, still holds its binary, its LICENSE and its NOTICE, and is still
// shipped by every other build of this module — so it is read and parsed
// exactly as every other face directory is, and only then removed from the
// set the join is total over. A NOTICE that stopped parsing under the tag
// therefore still fails here rather than being skipped into silence.
//
// IT IS EMPTY IN THE DEFAULT BUILD, which is what keeps the untagged
// accounting byte-for-byte the guard it has always been: eleven faces,
// eleven NOTICEs, total in both directions.
//
// AND A DECLARED NAME THAT THE WALK DID NOT FIND IS A HARD FAILURE, not a
// quiet no-op: that is how this list would rot into excluding nothing
// while reading as though it excluded something.
func withoutUnshippedFaceDirs(t *testing.T, records []noticeRecord) []noticeRecord {
	t.Helper()
	if len(unshippedFaceDirs) == 0 {
		return records
	}
	kept := make([]noticeRecord, 0, len(records))
	dropped := map[string]bool{}
	for _, record := range records {
		excluded := false
		for _, dir := range unshippedFaceDirs {
			if record.dir == dir {
				excluded = true
				dropped[dir] = true
				break
			}
		}
		if !excluded {
			kept = append(kept, record)
		}
	}
	for _, dir := range unshippedFaceDirs {
		if !dropped[dir] {
			t.Fatalf(
				"this build declares folio-go/fonts/%s/ as a face it does not embed, but the walk of "+
					"folio-go/fonts/ found no such face directory. An exclusion that excludes nothing reads "+
					"as an accounting over a set it never narrowed — correct the list in the build-tagged "+
					"pair beside this file, or restore the directory.",
				dir,
			)
		}
	}
	if len(kept) == 0 {
		t.Fatal("every face directory was excluded as unshipped, which would make the accounting below vacuous")
	}
	return kept
}

// TestCJKFaceIsBuildTagged pins the pair that CAP-6 rests on. The embed
// and its stand-in must carry COMPLEMENTARY constraints: matching both
// would declare buildTaggedFaces twice and matching neither would not
// compile, so the pair is asserted rather than trusted — the same guard
// folio-go/cshared/cmd/folio8/imports_test.go keeps over its cgo/!cgo
// pair, which is the precedent this split follows.
func TestCJKFaceIsBuildTagged(t *testing.T) {
	for _, pair := range []struct{ path, constraint string }{
		{"notosanssc.go", "//go:build !nocjkface"},
		{"notosanssc_absent.go", "//go:build nocjkface"},
	} {
		source, err := os.ReadFile(pair.path)
		if err != nil {
			t.Fatalf("read %s: %v (it is one half of the pair that keeps the CJK face out of the designer's engine)", pair.path, err)
		}
		if !strings.Contains(string(source), pair.constraint) {
			t.Errorf("%s no longer carries the %s constraint, so the two files would collide or both drop out", pair.path, pair.constraint)
		}
	}
	// AND THE EMBED LIVES IN EXACTLY ONE OF THEM. A go:embed of the SC
	// face left behind in fonts.go would compile under both tags and the
	// designer's wasm would carry the 10 MiB again with every test here
	// still green.
	unconditional, err := os.ReadFile("fonts.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unconditional), "notosanssc/") {
		t.Error("fonts.go embeds notosanssc/ unconditionally again; the whole point of the pair is that the directive is not compiled under `nocjkface`")
	}

	// AND THE FACE NAME SURVIVES THE SPLIT. The key the designer's engine
	// installs at run time, and the key internal/fontset declares line
	// metrics under, must stay the key this package ships the face under —
	// three files in three packages agreeing on one string, with nothing
	// deriving it from anything else (D-B).
	if !strings.Contains(string(unconditional), "Noto Sans SC") {
		t.Error("fonts.go no longer mentions \"Noto Sans SC\" at all; the key the designer's shell declares as deferred must stay the key this package ships it under")
	}
}

// joinShippedToNotices derives the face -> directory relation instead of
// tabulating it: each NOTICE names its own shipped file, that file is
// hashed, and the hash is looked up in fonts.Shipped(). BOTH DIRECTIONS
// fall out of the same join — a NOTICE'd binary nothing ships, and a
// shipped face no NOTICE describes — and no new authority is introduced
// that a future face could be added without touching.
func joinShippedToNotices(t *testing.T, shipped map[string][]byte, records []noticeRecord) map[string]noticeRecord {
	t.Helper()
	if len(shipped) == 0 {
		t.Fatal("fonts.Shipped() returned an EMPTY set — every total below would be 0 of 0, which is the one result this guard exists to refuse")
	}

	keysByDigest := map[string][]string{}
	for key, embedded := range shipped {
		if len(embedded) == 0 {
			t.Fatalf("fonts.Shipped()[%q] is ZERO bytes — it would join to nothing and compare vacuously to anything", key)
		}
		digest := sha256Hex(embedded)
		keysByDigest[digest] = append(keysByDigest[digest], key)
	}
	for digest, keys := range keysByDigest {
		if len(keys) > 1 {
			sort.Strings(keys)
			t.Fatalf(
				"fonts.Shipped() ships the SAME bytes (sha256 %s) under %d keys (%s), so the face -> directory "+
					"join by bytes is ambiguous. Two names for one file need a ruling, not a tie-break here.",
				digest, len(keys), strings.Join(keys, ", "),
			)
		}
	}

	byKey := map[string]noticeRecord{}
	for _, record := range records {
		keys := keysByDigest[record.diskDigest]
		if len(keys) == 0 {
			t.Fatalf(
				"THE OTHER DIRECTION: %s records a shipped file (%s, sha256 %s, %d bytes) whose bytes "+
					"fonts.Shipped() does not carry under any key.\n"+
					"Either that face stopped shipping and its directory and NOTICE should go with it, or the "+
					"go:embed directive in fonts.go is now reading a different file than the NOTICE describes. "+
					"fonts.Shipped() currently carries %d face(s): %s.",
				record.noticePath, record.binaryPath, record.diskDigest, record.diskSize,
				len(shipped), strings.Join(sortedKeys(shipped), ", "),
			)
		}
		key := keys[0]
		if previous, duplicate := byKey[key]; duplicate {
			t.Fatalf(
				"face %q is claimed by TWO directories — %s and %s — which both hold the same bytes. One face, "+
					"one provenance record.",
				key, previous.dir, record.dir,
			)
		}
		byKey[key] = record
	}

	missing := []string{}
	for key := range shipped {
		if _, ok := byKey[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		dirs := make([]string, 0, len(records))
		for _, record := range records {
			dirs = append(dirs, record.dir)
		}
		t.Fatalf(
			"THE SILENT DIRECTION: %d of the %d face(s) fonts.Shipped() carries have NO provenance record "+
				"beside them — %s.\n"+
				"A face reaches this state by being added to fonts.Shipped() with no NOTICE.md of its own, which "+
				"fails nothing else in this repository: it renders, it embeds, and every other count still reads "+
				"N of N because every other count is taken from its own list.\n"+
				"Add folio-go/fonts/<dir>/ holding the face, its LICENSE and a NOTICE.md in the shape the four "+
				"existing ones use (AD-26), then account for it by one of the two named routes.\n"+
				"directories walked (%d): %s",
			len(missing), len(shipped), strings.Join(missing, ", "), len(records), strings.Join(dirs, ", "),
		)
	}
	return byKey
}

func sortedKeys(shipped map[string][]byte) []string {
	keys := make([]string, 0, len(shipped))
	for key := range shipped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// THIS READER EXISTS TWICE, AND THE DUPLICATION IS FORCED BY THE PACKAGE
// BOUNDARY RATHER THAN CHOSEN. folio-go/fontgen_matrix_test.go needs the same
// keys and is `package folio8_test` (it must be: it imports `fonts`, and `fonts`
// imports `folio8`, so `package folio8` would be a cycle) — and Go gives one test
// package no access to another test package's identifiers. The only way to
// share it would be to add exported production API to `fonts`, a package whose
// stated purpose is to hold font data, for the sole benefit of two tests.
//
// WHAT MAKES THE DUPLICATION SAFE IS THAT NEITHER COPY CAN ROT QUIETLY. Both
// refuse rather than degrade: if the UPSTREAM literal changes shape, each
// reader t.Fatalf's on an unmatched block or an empty key set, so a manifest
// format change fails LOUDLY in both places at once instead of leaving one copy
// silently extracting nothing. A reader that returned an empty map would be the
// dangerous version, and neither does.
//
// THE FONTGEN MANIFEST, READ AS TEXT — because there is no other way to
// ask it. tools/fontgen/instance_faces.py has no machine-readable mode:
// its only flags are --sources (required), --repo-root, --out and
// --verify-only, so asking it what it derives costs ~20 MB of upstream
// sources and three fontTools runs. The UPSTREAM literal is the manifest,
// and its `"key"` fields are byte-identical to the fonts.Shipped() map
// keys — measured, exact equality, no normalisation.
var (
	upstreamManifestBlock = regexp.MustCompile(`(?s)\nUPSTREAM = \[\n(.*?)\n\]\n`)
	upstreamManifestKey   = regexp.MustCompile(`"key":\s*"([^"]+)"`)
)

func fontgenManifestPath() string {
	return filepath.Join("..", "..", "tools", "fontgen", "instance_faces.py")
}

// readFontgenManifestKeys throws rather than returning an empty set. An
// empty manifest read would silently move every face out of the `derived`
// route and into whatever arm handled the remainder — which is why there
// is no such arm.
func readFontgenManifestKeys(t *testing.T) map[string]bool {
	t.Helper()
	path := fontgenManifestPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read the fontgen manifest at %s, so the `derived` route has no source: %v", path, err)
	}
	block := upstreamManifestBlock.FindStringSubmatch(string(raw))
	if block == nil {
		t.Fatalf(
			"%s no longer declares an `UPSTREAM = [ … ]` list this guard can read. Re-derive the reader against "+
				"the new shape rather than deleting the check: an unreadable manifest would move every shipped "+
				"face out of the `derived` route at once, and this test refuses to report that as an accounting.",
			path,
		)
	}
	keys := map[string]bool{}
	for _, match := range upstreamManifestKey.FindAllStringSubmatch(block[1], -1) {
		keys[match[1]] = true
	}
	if len(keys) == 0 {
		t.Fatalf(
			"%s declares an UPSTREAM list from which this guard extracted NO `\"key\"` field. An extraction that "+
				"finds nothing is the failure mode being designed against, not an outcome.",
			path,
		)
	}
	return keys
}

// TestEveryShippedFaceHasARecordedProvenance is obligation (a): every face
// fonts.Shipped() carries has a NOTICE.md beside it whose RECORDED shipped
// digest equals the embedded bytes and whose RECORDED size equals their
// length.
//
// THE RECORDED DIGEST IS COMPARED, NOT RESTATED. fonts_test.go:27-34
// forbids putting Roboto's sha256 into Go as a literal, and that rule is
// kept here for every face: the number this test compares against is read
// out of the NOTICE at run time, so a byte moving in the binary without
// the NOTICE moving with it is what turns this red. The NOTICE pin is a
// THIRD comparison — designer catalogue, embedded bytes, recorded record —
// not a fourth copy of the number.
func TestEveryShippedFaceHasARecordedProvenance(t *testing.T) {
	shipped := Shipped()
	records := readNoticeRecords(t)
	byKey := joinShippedToNotices(t, shipped, records)

	for _, key := range sortedKeys(shipped) {
		record := byKey[key]
		embedded := shipped[key]

		if record.recordedDigest != record.diskDigest {
			t.Fatalf(
				"face %q: %s records sha256 %s for its shipped file, but %s hashes to %s.\n"+
					"One of the two moved without the other. If the BINARY was replaced deliberately, the NOTICE's "+
					"recorded digest, its Size row and (for a derived face) tools/fontgen's out_sha256 all have to "+
					"move with it — the recorded provenance is the only engine-side record of what these bytes are.",
				key, record.noticePath, record.recordedDigest, record.binaryPath, record.diskDigest,
			)
		}
		// AGAINST THE EMBEDDED BYTES, NOT ONLY THE FILE ON DISK: go:embed
		// is what actually ships, and a directive pointing at a different
		// file is exactly the drift a disk-only comparison cannot see.
		if embeddedDigest := sha256Hex(embedded); embeddedDigest != record.recordedDigest {
			t.Fatalf(
				"face %q: the bytes fonts.Shipped() returns hash to %s, while %s records %s. The go:embed "+
					"directive and this NOTICE are describing different files.",
				key, embeddedDigest, record.noticePath, record.recordedDigest,
			)
		}
		if record.recordedSize != len(embedded) {
			t.Fatalf(
				"face %q: %s records Size %d bytes, but fonts.Shipped() returns %d bytes (%s is %d bytes on "+
					"disk). A size that disagrees with the file is a record of something that is not being shipped.",
				key, record.noticePath, record.recordedSize, len(embedded), record.binaryPath, record.diskSize,
			)
		}
	}

	// NO "checked N of N" WITNESS HERE, DELIBERATELY. The loop ranges over
	// fonts.Shipped() itself and every iteration either fatals or completes,
	// so a counter compared against len(shipped) could not fail — it would be
	// a tautology wearing the costume of the very thing this file argues for.
	// TOTALITY IS STRUCTURAL INSTEAD: joinShippedToNotices refuses a shipped
	// face with no record and a record with no shipped face before this loop
	// starts, so there is no way to reach here having skipped one.
	t.Logf("provenance: %d shipped faces, each with a NOTICE whose recorded digest and size match its embedded bytes (%s)", len(shipped), strings.Join(sortedKeys(shipped), ", "))
}

// TestEveryShippedFaceIsAccountedForByExactlyOneRoute is obligation (b):
// every face fonts.Shipped() carries is accounted for by EXACTLY ONE named
// route.
//
//   - `derived`         — its FontSet key appears in tools/fontgen's UPSTREAM
//     manifest, so folio-go/fontgen_matrix_test.go replays
//     the instancing and compares the produced bytes.
//   - `static-upstream` — its own NOTICE.md records `copied unmodified, no
//     derivation`: the upstream release publishes a static
//     TTF, the file is that file, and the identity of the
//     source and shipped digests IS the record.
//
// TWO KINDS OF ASSURANCE UNDER ONE HEADING, AND THE POINT IS THAT NEITHER
// IS A DEFAULT. Unaccounted fails. Accounted twice fails. There is no
// third arm, because a face that is neither derived nor a verbatim upstream
// copy needs a third NAMED route ruled on by a human — a fallback bucket
// would quietly absorb it and report full coverage.
func TestEveryShippedFaceIsAccountedForByExactlyOneRoute(t *testing.T) {
	shipped := Shipped()
	records := readNoticeRecords(t)
	byKey := joinShippedToNotices(t, shipped, records)
	manifestKeys := readFontgenManifestKeys(t)

	derived := []string{}
	staticUpstream := []string{}
	for _, key := range sortedKeys(shipped) {
		record := byKey[key]
		inManifest := manifestKeys[key]
		// THE NOTICE'S OWN DERIVATION STATEMENT IS ONE OF THE TWO OPERANDS, not
		// a thing that was parsed and then ignored. A route decided by manifest
		// membership alone would let the single on-disk sentence about how these
		// bytes came to exist assert nothing at all — so every arm below is a
		// comparison BETWEEN the manifest and the NOTICE, and the two failure
		// arms are the two ways they can disagree.
		switch {
		case record.derivative && record.staticUpstream:
			t.Fatalf(
				"%s states BOTH derivation shapes for face %q: a DERIVATIVE blockquote and `copied unmodified, "+
					"no derivation`. A file is either produced from an upstream variable build or copied from an "+
					"upstream static one; it cannot be both, and until the NOTICE picks one there is no route to "+
					"account it to.",
				record.noticePath, key,
			)
		case inManifest && record.staticUpstream:
			t.Fatalf(
				"face %q is claimed by BOTH named routes, and exactly one of them is a lie.\n"+
					"  · derived         — %q appears in tools/fontgen/instance_faces.py's UPSTREAM manifest\n"+
					"  · static-upstream — %s records `copied unmodified, no derivation`\n"+
					"A file produced by an instancing run is not a verbatim copy of its source. Either remove the "+
					"manifest entry or correct the NOTICE; the accounting is over %d shipped face(s) and every one "+
					"of them takes exactly one route.",
				key, key, record.noticePath, len(shipped),
			)
		case record.derivative && !inManifest:
			t.Fatalf(
				"face %q: %s declares the file a DERIVATIVE of an upstream variable build, but %q appears in NO "+
					"UPSTREAM entry of tools/fontgen/instance_faces.py, so nothing replays that derivation and "+
					"nothing proves the committed bytes still come back.\n"+
					"A recorded derivation with no way to run it is a claim, not a route. Either add the manifest "+
					"entry or correct the NOTICE.",
				key, record.noticePath, key,
			)
		case inManifest && record.derivative:
			derived = append(derived, key)
		case record.staticUpstream:
			// THE STATIC-UPSTREAM ROUTE, MEASURED RATHER THAN TAKEN ON THE
			// SENTENCE'S WORD. roboto/NOTICE.md says it itself: "the two sha256
			// values below being equal is the provenance record, not a
			// coincidence to reconcile." Until this comparison existed, that
			// record was read by nothing in the repository and the route was
			// satisfied by prose alone — which is D-000.12's own defect one
			// layer up, a guard reading what is easy to read instead of what
			// carries the meaning.
			embeddedDigest := sha256Hex(shipped[key])
			if record.recordedSource != record.recordedDigest || record.recordedDigest != embeddedDigest {
				t.Fatalf(
					"face %q takes the `static-upstream` route, whose ENTIRE content is that the file IS its "+
						"upstream source — and the three digests that would make that true do not agree.\n"+
						"  recorded SOURCE  (%s) %s\n"+
						"  recorded SHIPPED (%s) %s\n"+
						"  embedded bytes        %s\n"+
						"`copied unmodified, no derivation` means there is no instancing step to replay, so this "+
						"identity is the only provenance this face has. If the file really was re-derived or "+
						"re-downloaded, it needs the `derived` route and a manifest entry — not a corrected "+
						"sentence.",
					key,
					record.noticePath, record.recordedSource,
					record.noticePath, record.recordedDigest,
					embeddedDigest,
				)
			}
			staticUpstream = append(staticUpstream, key)
		default:
			t.Fatalf(
				"face %q is accounted for by NO named route, so nothing in this repository asserts where its "+
					"bytes came from.\n"+
					"  · derived         — its key is absent from tools/fontgen/instance_faces.py's UPSTREAM "+
					"manifest (%s), so no reproduction test covers it\n"+
					"  · static-upstream — %s states neither the DERIVATIVE blockquote the three Noto notices "+
					"carry nor roboto/NOTICE.md's `| Relation to source | **copied unmodified, no derivation** … |`\n"+
					"fonts.Shipped() carries %d face(s): %s. Pick one of the two routes and make the record say "+
					"so; if the face is genuinely neither, it needs a THIRD NAMED ROUTE — that is a ruling, not a "+
					"default arm to be added here.",
				key, fontgenManifestPath(), record.noticePath, len(shipped), strings.Join(sortedKeys(shipped), ", "),
			)
		}
	}

	// THE OTHER DIRECTION FOR THE `derived` ROUTE: a manifest entry that
	// derives a face nothing ships. That is how the manifest's own `N of N`
	// witness stays green while the shipped set has moved underneath it.
	//
	// ⚠ A FACE THIS BUILD DELIBERATELY DOES NOT EMBED IS NOT AN ORPHAN.
	// Under `-tags nocjkface` the CJK face is still derived, still
	// committed, still NOTICE'd and still shipped by every other build of
	// this module — it is only absent from THIS Shipped(). Excluding it by
	// name here is what keeps the check pointed at real manifest drift;
	// `unshippedFaceKeys` is empty in the default build, and a name in it
	// that the manifest does not carry fails below rather than excluding
	// nothing.
	for _, key := range unshippedFaceKeys {
		if !manifestKeys[key] {
			t.Fatalf(
				"this build declares face %q as one it does not embed, but tools/fontgen's UPSTREAM manifest "+
					"does not derive it either, so the exclusion narrows nothing. Correct the list in the "+
					"build-tagged pair beside this file.",
				key,
			)
		}
		delete(manifestKeys, key)
	}
	orphaned := []string{}
	for key := range manifestKeys {
		if _, ok := shipped[key]; !ok {
			orphaned = append(orphaned, key)
		}
	}
	if len(orphaned) > 0 {
		sort.Strings(orphaned)
		t.Fatalf(
			"tools/fontgen/instance_faces.py's UPSTREAM manifest derives %d face(s) that fonts.Shipped() does "+
				"not carry — %s. fonts.Shipped() carries: %s.",
			len(orphaned), strings.Join(orphaned, ", "), strings.Join(sortedKeys(shipped), ", "),
		)
	}

	// NO `derived + static == total` WITNESS. The switch above has no arm that
	// falls through — every iteration either fatals or appends to exactly one
	// of the two slices — so that sum could not disagree with len(shipped), and
	// asserting it would be a tautology in a file whose whole argument is about
	// denominators that cannot fail.
	//
	// EVERY MESSAGE TAKES ITS DENOMINATOR FROM fonts.Shipped(), NEVER A BARE
	// RATIO, so this line cannot read as full coverage of a set that grew.
	t.Logf(
		"accounting: %d of %d shipped faces derived (%s); %d of %d accounted static-upstream (%s)",
		len(derived), len(shipped), strings.Join(derived, ", "),
		len(staticUpstream), len(shipped), strings.Join(staticUpstream, ", "),
	)
}
