package fontdir

import (
	"bytes"
	"encoding/binary"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// The fixtures are BUILT, not committed. Every face these tests need is
// already in the repository — the eleven the fonts package embeds, and
// the variable build under folio-go/testdata/fonts — so writing a second
// copy of any of them into a fontdir/testdata directory would add a
// redistributed font binary to the repo (with the LICENSE/NOTICE
// obligations AD-26 attaches to one) to prove something about a
// directory listing. The tests write the bytes they need into
// t.TempDir() instead, under filenames THEY choose, which is also what
// makes the "the key comes from the binary, not the filename" assertions
// below mean anything.

func shippedFace(t *testing.T, key string) []byte {
	t.Helper()
	data, ok := fonts.Shipped()[key]
	if !ok {
		t.Fatalf("fonts.Shipped() has no face %q", key)
	}
	return data
}

// variableFace is the repo's one committed variable build.
func variableFace(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "fonts", "notosansthai-variable-testonly", "NotoSansThai-VF.ttf"))
	if err != nil {
		t.Fatalf("read the variable fixture: %v", err)
	}
	return data
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// stripNameTable removes the `name` table's DIRECTORY RECORD from an
// sfnt, leaving a face that still parses, still carries every table
// folio8 actually reads, and declares no family name at all. That is the
// I/O matrix's "unreadable name table" row, and it has to be built
// rather than found: no font anybody ships is missing its own name.
//
// The table's bytes are left where they are and only the record is
// dropped, so every remaining record's absolute (offset, length) is
// still in bounds and the structural check still passes — which is the
// point. This face is well-formed; it simply cannot be keyed.
func stripNameTable(t *testing.T, data []byte) []byte {
	t.Helper()
	out := append([]byte(nil), data...)
	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	records := out[12 : 12+16*numTables]
	found := -1
	for i := 0; i < numTables; i++ {
		if string(records[i*16:i*16+4]) == "name" {
			found = i
			break
		}
	}
	if found < 0 {
		t.Fatal("fixture face has no name table to strip — the helper is pointed at the wrong bytes")
	}
	copy(records[found*16:], records[(found+1)*16:])
	clear(records[(numTables-1)*16:])
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables-1))
	return out
}

func setKeys(set folio8.FontSet) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func skipReasons(skipped []Skipped) string {
	lines := make([]string, 0, len(skipped))
	for _, s := range skipped {
		lines = append(lines, s.String())
	}
	return strings.Join(lines, "\n")
}

// reportsOnly asserts the skip report names exactly these files (by base
// name) and no others — the half of the contract that says an ignored
// file is NOT a skip.
func reportsOnly(t *testing.T, skipped []Skipped, wantBases ...string) {
	t.Helper()
	got := make([]string, 0, len(skipped))
	for _, s := range skipped {
		if s.Reason == "" {
			t.Errorf("skip of %s carries no reason — a silent skip is the one thing this type exists to prevent", s.File)
		}
		got = append(got, filepath.Base(s.File))
	}
	sort.Strings(got)
	want := append([]string(nil), wantBases...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("skips = %v, want %v\n%s", got, want, skipReasons(skipped))
	}
}

// TestOrdinaryDirectoryYieldsAFaceForEveryFile is the first I/O matrix
// row: three readable faces in, three keys out, taken from the binaries.
func TestOrdinaryDirectoryYieldsAFaceForEveryFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.ttf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "b.ttf", shippedFace(t, "Noto Sans Thai"))
	writeFile(t, dir, "c.ttf", shippedFace(t, "Roboto"))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped)
	want := []string{"Noto Sans", "Noto Sans Thai", "Roboto"}
	if got := setKeys(set); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

// TestKeysComeFromTheBinaryAndNotTheFilename is D2, asserted on both
// polarities at once: every one of the eleven shipped faces, written to
// disk under a filename that says NOTHING about it, is keyed back to the
// exact name fonts.Shipped() files it under.
//
// That equality is the load-bearing one. It is what lets a chain entry
// name a face without knowing whether the renderer got it from the
// shipped set or from a directory, and it would be the first thing to
// break if anybody ever keyed from the filename stem: these filenames
// are fa.ttf … fk.ttf.
func TestKeysComeFromTheBinaryAndNotTheFilename(t *testing.T) {
	dir := t.TempDir()
	shipped := fonts.Shipped()
	want := setKeys(shipped)
	for i, key := range want {
		writeFile(t, dir, "f"+string(rune('a'+i))+".ttf", shipped[key])
	}

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped)
	if got := setKeys(set); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("keys built from the binaries = %v,\nfonts.Shipped() keys           = %v", got, want)
	}
}

// TestBoldCutIsKeyedFamilyPlusSubfamily and the Regular case below are
// the matrix's two keying rows, kept apart because they are two
// different rules: a non-Regular subfamily is APPENDED, a Regular one is
// DROPPED.
func TestBoldCutIsKeyedFamilyPlusSubfamily(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "whatever-the-author-called-it.ttf", shippedFace(t, "Noto Sans Bold"))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped)
	if _, ok := set["Noto Sans Bold"]; !ok {
		t.Errorf("keys = %v, want the binary's own \"Noto Sans Bold\"", setKeys(set))
	}
}

func TestRegularCutIsKeyedByTheBareFamily(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "NotoSans-Regular-Renamed.ttf", shippedFace(t, "Noto Sans"))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped)
	if _, ok := set["Noto Sans"]; !ok {
		t.Errorf("keys = %v, want the bare family \"Noto Sans\" — a Regular subfamily is dropped, never appended", setKeys(set))
	}
}

// TestCorruptFileIsSkippedAndTheGoodFacesStillLoad is the row this
// design exists for: a directory on a real server holds a half-written
// file sooner or later, and one of those must not stop every render.
func TestCorruptFileIsSkippedAndTheGoodFacesStillLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good-a.ttf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "good-b.ttf", shippedFace(t, "Roboto"))
	writeFile(t, dir, "truncated.ttf", shippedFace(t, "Noto Sans Bold")[:64])

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v — a corrupt file must be a skip, never a fatal error", err)
	}
	reportsOnly(t, skipped, "truncated.ttf")
	want := []string{"Noto Sans", "Roboto"}
	if got := setKeys(set); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

// TestVariableFaceIsSkippedAndReported: a variable build is refused by
// the renderer's own guard, so it is refused here too, as a skip like
// any other refusal — and the message is the renderer's, carrying the
// instancer remedy, not a second sentence written by this package.
func TestVariableFaceIsSkippedAndReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.ttf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "variable.ttf", variableFace(t))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped, "variable.ttf")
	if len(set) != 1 {
		t.Errorf("keys = %v, want only the static face", setKeys(set))
	}
	if !strings.Contains(strings.ToLower(skipReasons(skipped)), "variable") {
		t.Errorf("the skip does not say the face is variable: %s", skipReasons(skipped))
	}
}

// TestNonFontFilesAreIgnoredWithoutBeingReported: a font directory that
// also holds the licences those fonts ship with is an ordinary, correct
// font directory. Reporting its NOTICE as a fault would train a reader
// to stop reading the report, so ignoring and skipping are kept
// distinct.
func TestNonFontFilesAreIgnoredWithoutBeingReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.ttf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "LICENSE", []byte("SIL Open Font License"))
	writeFile(t, dir, "readme.txt", []byte("drop your brand faces here"))
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A face inside a subdirectory is NOT found: the read is one
	// directory deep, by design.
	writeFile(t, dir, filepath.Join("subdir", "buried.ttf"), shippedFace(t, "Roboto"))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped)
	if got := setKeys(set); strings.Join(got, "|") != "Noto Sans" {
		t.Errorf("keys = %v, want only [Noto Sans] — no recursion", got)
	}
}

// TestOtfFacesAreCandidatesToo: .otf is advertised in the guide, the
// README and the CLI's own help, so it is asserted here. Deleting the
// entry from fontExtensions, or mistyping its media type, reds this.
func TestOtfFacesAreCandidatesToo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "brand.otf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "BRAND-UPPER.OTF", shippedFace(t, "Roboto"))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped)
	want := []string{"Noto Sans", "Roboto"}
	if got := setKeys(set); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("keys = %v, want %v — an .otf face is a candidate, and the extension match is case-insensitive", got, want)
	}
}

// TestFontShapedNearMissesAreReported: a .woff, a .woff2 or a collection
// sitting where a face was expected is somebody's brand face that will
// simply not appear. Silence there is how an operator ends up staring at
// an empty FontSet, so these are reported even though they are not
// candidates — unlike a LICENSE, which is not reported.
func TestFontShapedNearMissesAreReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.ttf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "brand.woff", []byte("wOFF"))
	writeFile(t, dir, "brand.woff2", []byte("wOF2"))
	writeFile(t, dir, "family.ttc", []byte("ttcf"))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped, "brand.woff", "brand.woff2", "family.ttc")
	if got := setKeys(set); strings.Join(got, "|") != "Noto Sans" {
		t.Errorf("keys = %v, want only [Noto Sans]", got)
	}
}

// TestSymlinkedFaceIsReportedNotFollowed is its own test because it
// SKIPS on a filesystem without symlinks, and a skip inside the
// non-font-files test above would take that test's other assertions with
// it.
//
// The rule taken is the strict one — no symlink is followed, including
// one pointing inside the directory — so the compensating property is
// that a link whose name looks like a face is REPORTED. An operator who
// staged fonts by linking them sees exactly why they are missing.
func TestSymlinkedFaceIsReportedNotFollowed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.ttf", shippedFace(t, "Noto Sans"))
	outside := t.TempDir()
	linked := writeFile(t, outside, "elsewhere.ttf", shippedFace(t, "Roboto"))
	if err := os.Symlink(linked, filepath.Join(dir, "link.ttf")); err != nil {
		t.Skipf("symlinks unavailable on this filesystem: %v", err)
	}
	// A link pointing INSIDE the directory is treated the same way.
	if err := os.Symlink(filepath.Join(dir, "good.ttf"), filepath.Join(dir, "inside.ttf")); err != nil {
		t.Skipf("symlinks unavailable on this filesystem: %v", err)
	}

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped, "link.ttf", "inside.ttf")
	if got := setKeys(set); strings.Join(got, "|") != "Noto Sans" {
		t.Errorf("keys = %v, want only [Noto Sans] — no symlink is followed", got)
	}
}

// TestFaceWithNoFamilyNameIsSkipped: a face that parses, and that the
// renderer would happily draw, but that declares no family. There is
// nothing to key it by, and inventing one from its filename is the whole
// mistake this package refuses to make — so it is reported and left out.
func TestFaceWithNoFamilyNameIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.ttf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "nameless.ttf", stripNameTable(t, shippedFace(t, "Roboto")))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped, "nameless.ttf")
	if _, ok := set["nameless"]; ok {
		t.Fatal("the nameless face was keyed from its FILENAME — exactly what D2 forbids")
	}
	if got := setKeys(set); strings.Join(got, "|") != "Noto Sans" {
		t.Errorf("keys = %v, want only [Noto Sans]", got)
	}
}

// TestTwoFilesOneKeyResolveDeterministically: the winner must not depend
// on the order a filesystem happened to list the directory in, or the
// same directory would render differently on two machines.
//
// The two fixtures are DISTINGUISHABLE — the loser carries trailing
// padding an sfnt reader ignores — so the assertion is which BYTES
// survived, not merely that one of two identical copies did.
func TestTwoFilesOneKeyResolveDeterministically(t *testing.T) {
	dir := t.TempDir()
	winner := shippedFace(t, "Roboto")
	loser := append(append([]byte(nil), winner...), make([]byte, 32)...)
	writeFile(t, dir, "a-roboto.ttf", winner)
	writeFile(t, dir, "z-roboto.ttf", loser)

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped, "z-roboto.ttf")
	if got := setKeys(set); strings.Join(got, "|") != "Roboto" {
		t.Fatalf("keys = %v, want one [Roboto]", got)
	}
	if !bytes.Equal(set["Roboto"], winner) {
		t.Errorf("the face that survived is not a-roboto.ttf's — the lexically first file must win")
	}
	if !strings.Contains(skipReasons(skipped), filepath.Join(dir, "a-roboto.ttf")) {
		t.Errorf("the duplicate report does not name the winning file by the same path spelling it reports the loser with: %s", skipReasons(skipped))
	}
}

// TestFaceWithControlCharactersInItsNameIsSkipped: a name record
// carrying control characters is not a face name anybody can type into a
// chain entry, and letting one become a key would put an unprintable
// string into a document and into every message that quotes it.
func TestFaceWithControlCharactersInItsNameIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.ttf", shippedFace(t, "Noto Sans"))
	writeFile(t, dir, "hostile.ttf", rebrandUTF16(t, shippedFace(t, "Roboto"), "Roboto", "Rob\x00to"))

	set, skipped, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	reportsOnly(t, skipped, "hostile.ttf")
	if got := setKeys(set); strings.Join(got, "|") != "Noto Sans" {
		t.Errorf("keys = %v, want only [Noto Sans]", got)
	}
}

// TestMissingDirectoryIsAnErrorNamingThePath — a directory an integrator
// named and that is not there is a deployment mistake, and the one thing
// the error has to carry is which path was looked at.
func TestMissingDirectoryIsAnErrorNamingThePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-there")
	set, skipped, err := Set(missing)
	if err == nil {
		t.Fatalf("Set(%q) returned no error", missing)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error does not name the path: %v", err)
	}
	if set != nil || skipped != nil {
		t.Errorf("a failed call returned a set (%v) or skips (%v)", set, skipped)
	}
}

// TestEmptyDirectoryIsAnEmptySetAndNoError — an empty FontSet has been a
// legal render input since the engine stopped requiring one, so an empty
// directory is a legal configuration and not a fault.
func TestEmptyDirectoryIsAnEmptySetAndNoError(t *testing.T) {
	set, skipped, err := Set(t.TempDir())
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(set) != 0 || len(skipped) != 0 {
		t.Errorf("set = %v, skips = %v, want both empty", setKeys(set), skipped)
	}
}

// TestSetIsMergedByTheCaller is D3, shown rather than asserted about:
// the helper returns the directory's set and the caller combines it with
// maps.Copy — second wins — which is the repo's existing precedent.
// There is no merge helper here to test, and this is the guard on there
// never being one that behaves differently.
func TestSetIsMergedByTheCaller(t *testing.T) {
	dir := t.TempDir()
	// A face keyed "Roboto" that is NOT byte-identical to the shipped
	// Roboto, so "second wins" is observable rather than assumed.
	brand := append(append([]byte(nil), shippedFace(t, "Roboto")...), make([]byte, 32)...)
	writeFile(t, dir, "brand.ttf", brand)

	supplied, _, err := Set(dir)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok := supplied["Roboto"]; !ok {
		t.Fatalf("keys = %v, want [Roboto]", setKeys(supplied))
	}

	combined := fonts.Shipped()
	before := len(combined)
	maps.Copy(combined, supplied)
	if len(combined) != before {
		t.Errorf("merging a face the shipped set already names changed its size: %d, was %d", len(combined), before)
	}
	if !bytes.Equal(combined["Roboto"], brand) {
		t.Errorf("maps.Copy did not put the DIRECTORY's Roboto on top of the shipped one")
	}
}

// rebrandUTF16 rewrites a face's Windows-platform name records in place,
// same-length so no table offset moves. It is how this file builds a
// face declaring something no shipped face declares.
func rebrandUTF16(t *testing.T, data []byte, from, to string) []byte {
	t.Helper()
	if len([]rune(from)) != len([]rune(to)) {
		t.Fatalf("rebrand %q -> %q changes the length, which would move every table offset", from, to)
	}
	utf16be := func(s string) []byte {
		out := make([]byte, 0, len(s)*2)
		for _, r := range s {
			out = append(out, byte(r>>8), byte(r))
		}
		return out
	}
	out := bytes.ReplaceAll(data, utf16be(from), utf16be(to))
	if bytes.Equal(out, data) {
		t.Fatalf("rebrand found no UTF-16 occurrence of %q", from)
	}
	return out
}
