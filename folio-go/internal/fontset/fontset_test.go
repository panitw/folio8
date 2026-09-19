package fontset

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/boxesandglue/textshape/ot"
	"github.com/boxesandglue/textshape/subset"
)

// testFontPath is Story 1.5's one small Latin test face (AC26), committed
// with its licence text and copyright line (AC25) at
// folio-go/testdata/fonts/Roboto-Regular.ttf.
func testFontBytes(t *testing.T) []byte {
	t.Helper()
	// internal/fontset -> internal -> folio-go
	path := filepath.Join("..", "..", "testdata", "fonts", "Roboto-Regular.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test font %s: %v", path, err)
	}
	return data
}

// patchUnitsPerEm returns a COPY of a well-formed sfnt with the head
// table's unitsPerEm field (offset 18 within head, per ot/metrics.go)
// overwritten to value. It locates the head table via the sfnt table
// directory (12-byte header, then 16-byte records) rather than assuming
// a fixed offset, so it survives a different test font being substituted
// later.
func patchUnitsPerEm(t *testing.T, data []byte, value uint16) []byte {
	t.Helper()
	out := make([]byte, len(data))
	copy(out, data)

	numTables := binary.BigEndian.Uint16(out[4:6])
	for i := 0; i < int(numTables); i++ {
		rec := out[12+i*16 : 12+i*16+16]
		if string(rec[0:4]) != "head" {
			continue
		}
		headOffset := binary.BigEndian.Uint32(rec[8:12])
		binary.BigEndian.PutUint16(out[headOffset+18:headOffset+20], value)
		return out
	}
	t.Fatal("test font has no head table")
	return nil
}

// TestNewRejectsZeroUnitsPerEm is AC19's first retained fixture: a font
// with unitsPerEm == 0 produces a located error, not a panic. D-000.9:
// what would this print if it were unable to run at all? A test harness
// panic (not a t.Fatal) — which this test would surface immediately as a
// crashed test binary, distinguishable from a clean failure.
func TestNewRejectsZeroUnitsPerEm(t *testing.T) {
	patched := patchUnitsPerEm(t, testFontBytes(t), 0)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New panicked instead of returning a located error (AC19): %v", r)
		}
	}()

	_, err := New("zero-upem-face", patched)
	if err == nil {
		t.Fatal("expected an error for unitsPerEm == 0, got nil")
	}
	if !strings.Contains(err.Error(), "zero-upem-face") {
		t.Errorf("error does not name the font: %v", err)
	}
	if !strings.Contains(err.Error(), "0") {
		t.Errorf("error does not name the value: %v", err)
	}
}

// TestNewRejectsUnitsPerEmAboveMax is AC19's second retained fixture: a
// font with unitsPerEm > 16384 produces a located error.
func TestNewRejectsUnitsPerEmAboveMax(t *testing.T) {
	patched := patchUnitsPerEm(t, testFontBytes(t), 20000) // > 16384, representable in uint16 (F-4)

	_, err := New("too-large-upem-face", patched)
	if err == nil {
		t.Fatal("expected an error for unitsPerEm == 20000 (> 16384), got nil")
	}
	if !strings.Contains(err.Error(), "too-large-upem-face") || !strings.Contains(err.Error(), "20000") {
		t.Errorf("error does not name both the font and the value: %v", err)
	}
}

// TestNewAcceptsAValidFace is AC19's third retained fixture: a valid
// font (unitsPerEm == 2048, measured F-5) loads and scales correctly.
func TestNewAcceptsAValidFace(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if f.UnitsPerEm() != 2048 {
		t.Fatalf("UnitsPerEm() = %d, want 2048 (F-5 measurement)", f.UnitsPerEm())
	}
}

// TestHeadTimesMatchMeasuredValues is F-5's measured values, pinned:
// SOURCE head.created=3304067374 head.modified=3573633780.
func TestHeadTimesMatchMeasuredValues(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	created, modified := f.HeadTimes()
	if created != 3304067374 {
		t.Errorf("created = %d, want 3304067374 (F-5)", created)
	}
	if modified != 3573633780 {
		t.Errorf("modified = %d, want 3573633780 (F-5)", modified)
	}
}

// shapedGlyphIDs returns the SOURCE glyph ids f actually draws for text,
// in drawing order, deduplicated by first occurrence — the exact shape
// of input Subset now takes (Story 2.3, AC5: the subset is keyed on the
// glyphs the renderer draws, never on the runes the author typed).
//
// The tests below permute THIS, not a rune string. That is the point of
// AC5's clause about the permutation-invariance test: permuting runes
// upstream of a glyph-keyed subsetter would be permuting an input the
// function no longer has, which deletes the check's discriminating power
// exactly as a defensive sort would (AC8a).
func shapedGlyphIDs(t *testing.T, f *Font, text string) []uint16 {
	t.Helper()
	glyphs, err := f.Shaper().Shape(text)
	if err != nil {
		t.Fatalf("Shape(%q): %v", text, err)
	}
	if len(glyphs) == 0 {
		t.Fatalf("Shape(%q) produced no glyphs — a subset built from this would certify nothing", text)
	}
	seen := map[uint16]bool{}
	var out []uint16
	for _, g := range glyphs {
		if seen[g.GlyphID] {
			continue
		}
		seen[g.GlyphID] = true
		out = append(out, g.GlyphID)
	}
	return out
}

// TestSubsetContainsRequestedGlyphs is a basic sanity check: subsetting
// "AB" produces a subset in which both runes resolve to a glyph id.
func TestSubsetContainsRequestedGlyphs(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	gids := shapedGlyphIDs(t, f, "AB")
	sub, err := f.Subset(gids)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}
	if len(sub.Program) == 0 {
		t.Fatal("Subset produced no program bytes")
	}
	if len(gids) != 2 {
		t.Fatalf("shaping \"AB\" produced %d distinct glyphs, want 2", len(gids))
	}
	for _, g := range gids {
		if _, ok := sub.GlyphForSource[g]; !ok {
			t.Errorf("subset has no mapping for source glyph %d", g)
		}
	}
	if len(sub.Tag) != 6 {
		t.Fatalf("tag %q is not exactly six characters", sub.Tag)
	}
	for _, c := range sub.Tag {
		if c < 'A' || c > 'Z' {
			t.Fatalf("tag %q contains a non A-Z character", sub.Tag)
		}
	}
}

// TestRepeatInvarianceWithinOneProcess is AC8's repeat-invariance proof:
// calling Subset N>=16 times in one process on the same rune set produces
// byte-identical output every time (D-1.5.7). Go randomises map
// iteration per range statement, not once per process — the lead
// measured 8 distinct iteration orders ranging an 8-element map 200
// times in one process — so this reddens immediately if a future
// textshape (or a regression in this package) drops its internal
// determinism.
func TestRepeatInvarianceWithinOneProcess(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const n = 32 // >= 16, AC8.
	gids := shapedGlyphIDs(t, f, "Hello, World! 0123456789")

	first, err := f.Subset(gids)
	if err != nil {
		t.Fatalf("Subset (call 1): %v", err)
	}
	// Finding 18 (QA review): close this test's own vacuity rather than
	// relying on TestSubsetContainsRequestedGlyphs running first in the
	// same package, which is a property of test scheduling (declaration
	// order), not a dependency Go guarantees — -run filtering or a file
	// split breaks it (D-000.9).
	if len(first.Program) == 0 {
		t.Fatal("Subset (call 1) produced no program bytes — repeat-invariance would pass vacuously")
	}
	for i := 1; i < n; i++ {
		got, err := f.Subset(gids)
		if err != nil {
			t.Fatalf("Subset (call %d): %v", i+1, err)
		}
		if !bytes.Equal(first.Program, got.Program) {
			t.Fatalf("call %d diverged from call 1 (AC8 repeat-invariance, N=%d)", i+1, n)
		}
		if first.Tag != got.Tag {
			t.Fatalf("call %d tag %q diverged from call 1 tag %q", i+1, got.Tag, first.Tag)
		}
	}
}

// TestPermutationInvariance is AC8's permutation-invariance proof:
// distinct orderings of the SAME rune set produce byte-identical
// output. This is only meaningful because Subset does NOT sort its
// input (D-1.5.7, AC8a) — a sort here would make permuted-then-sorted
// inputs the same input, and this test would pass vacuously regardless
// of whether the property actually held.
func TestPermutationInvariance(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ascending := shapedGlyphIDs(t, f, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	reversed := make([]uint16, len(ascending))
	for i, g := range ascending {
		reversed[len(ascending)-1-i] = g
	}
	shuffled := shapedGlyphIDs(t, f, "MZAQBWCXDVEYFUGTHSRIJKLNOP")
	// Finding 19 (QA review): assert SET equality, not just length. A
	// length-only check would still pass if the literal above were
	// edited to introduce a duplicate plus an omission — which would
	// silently compare subsets of DIFFERENT rune sets and read as either
	// a false determinism failure or a coincidental pass.
	sortedAscending := slices.Clone(ascending)
	slices.Sort(sortedAscending)
	sortedShuffled := slices.Clone(shuffled)
	slices.Sort(sortedShuffled)
	if !slices.Equal(sortedAscending, sortedShuffled) {
		t.Fatalf("test bug: shuffled %q is not a permutation of ascending %q", shuffled, ascending)
	}

	base, err := f.Subset(ascending)
	if err != nil {
		t.Fatalf("Subset(ascending): %v", err)
	}
	// Finding 18 (QA review): close this test's own vacuity — see the
	// identical rationale in TestRepeatInvarianceWithinOneProcess.
	if len(base.Program) == 0 {
		t.Fatal("Subset(ascending) produced no program bytes — permutation-invariance would pass vacuously")
	}
	rev, err := f.Subset(reversed)
	if err != nil {
		t.Fatalf("Subset(reversed): %v", err)
	}
	shuf, err := f.Subset(shuffled)
	if err != nil {
		t.Fatalf("Subset(shuffled): %v", err)
	}

	if !bytes.Equal(base.Program, rev.Program) {
		t.Fatal("Subset(ascending) and Subset(reversed) produced different bytes (AC8 permutation-invariance)")
	}
	if !bytes.Equal(base.Program, shuf.Program) {
		t.Fatal("Subset(ascending) and Subset(shuffled) produced different bytes (AC8 permutation-invariance)")
	}
	if base.Tag != rev.Tag || base.Tag != shuf.Tag {
		t.Fatalf("tags diverged across permutations: ascending=%s reversed=%s shuffled=%s", base.Tag, rev.Tag, shuf.Tag)
	}
}

// TestDeriveTagUsesClosureSetNotOutputNumbering is AC7b's retained
// fixture, killing the "output numbering" rejected reading permanently
// (D-1.5.8): two documents whose glyph sets differ but are the SAME SIZE
// must produce DIFFERENT tags. Under the rejected output-numbering
// reading (hash of {0..n-1}), both would collide because
// createCompactMapping always produces a dense {0..n-1} range regardless
// of which source glyphs those are — this test is red under that reading
// and green under the ruled one (source-font numbering), and it is not
// vacuous: the two glyph sets below are deliberately chosen to be
// same-size, different-content.
func TestDeriveTagUsesClosureSetNotOutputNumbering(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Two 3-rune sets, same size, disjoint content, chosen so their
	// glyph closures differ (Latin letters map to distinct, non-composite
	// glyphs in Roboto — no shared components to accidentally collapse
	// the closures to the same set).
	setA := shapedGlyphIDs(t, f, "ABC")
	setB := shapedGlyphIDs(t, f, "DEF")

	subA, err := f.Subset(setA)
	if err != nil {
		t.Fatalf("Subset(setA): %v", err)
	}
	subB, err := f.Subset(setB)
	if err != nil {
		t.Fatalf("Subset(setB): %v", err)
	}

	if subA.NumGlyphs != subB.NumGlyphs {
		// Finding 2 (QA review): a skip here is a green build, and it
		// would silently retire the only permanent guard against the
		// catastrophic output-numbering reading the day a textshape
		// upgrade changes GSUB/composite closure for these two sets.
		// The fixture's precondition failing means the FIXTURE is
		// broken and needs a human to re-choose it — never a pass.
		t.Fatalf("test fixture assumption violated: NumGlyphs differ (%d vs %d) — not a same-size case; choose two rune sets whose closures are the same size", subA.NumGlyphs, subB.NumGlyphs)
	}
	if subA.Tag == subB.Tag {
		t.Fatalf(
			"two same-size, different-content glyph sets produced the SAME tag %q — this is exactly the "+
				"catastrophic output-numbering reading AC7b exists to kill (D-1.5.8)",
			subA.Tag,
		)
	}
}

// TestDeriveTagRedProofAgainstOutputNumbering is Finding 1's rewrite
// (QA review): the shipped version asserted only `rejectedTag(8) !=
// rejectedTag(8)` — the same argument twice, never calling deriveTag,
// never comparing two DIFFERENT glyph sets, and unfailable by
// construction. Proved by construction that it was worthless: swapping
// fontset.go's `deriveTag(plan.GlyphSet())` for the rejected
// {0..n-1} derivation reddened TestDeriveTagUsesClosureSetNotOutputNumbering
// (collided on tag "BQJKPS") while the old version of this test stayed
// green.
//
// This version exercises the REAL production path (f.Subset, which
// calls deriveTag internally) on the same two same-size, different-
// content sets AC7b uses, and separately reproduces what the REJECTED
// output-numbering derivation would have produced over the identical
// NumGlyphs, using a LOCAL, self-contained re-implementation of that
// rejected fold (Story 2.2 removed production's own GID-folding helper,
// fnv64aOverGIDs, when AC6/D-2.2.2 (superseded) moved deriveTag onto the
// returned program bytes — this test still needs the OLD algorithm to
// prove production no longer behaves like it, so it carries its own
// copy rather than reaching into production internals that no longer
// exist). The rejected derivation MUST collide (it is a pure function
// of count alone, by construction of createCompactMapping's always-dense
// {0..n-1} output numbering), and the real, shipped tags MUST NOT — so
// this test fails the moment deriveTag's call site is swapped back to
// EITHER the old glyph-set reading or the rejected output-numbering one.
func TestDeriveTagRedProofAgainstOutputNumbering(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	setA := shapedGlyphIDs(t, f, "ABC")
	setB := shapedGlyphIDs(t, f, "DEF")

	subA, err := f.Subset(setA)
	if err != nil {
		t.Fatalf("Subset(setA): %v", err)
	}
	subB, err := f.Subset(setB)
	if err != nil {
		t.Fatalf("Subset(setB): %v", err)
	}
	if subA.NumGlyphs != subB.NumGlyphs {
		t.Fatalf("test fixture precondition violated: NumGlyphs differ (%d vs %d) — not a same-size case", subA.NumGlyphs, subB.NumGlyphs)
	}

	// rejectedTag reproduces the REJECTED output-numbering derivation
	// directly: a hash over {0..numGlyphs-1}, using a local copy of the
	// FNV-1a-over-glyph-ids fold this package's production code used
	// before AC6/AC6a (D-2.2.2 (superseded)), so the only variable is
	// which glyph-id set is hashed.
	rejectedTag := func(numGlyphs int) string {
		const offset64 = uint64(14695981039346656037)
		const prime64 = uint64(1099511628211)
		h := offset64
		for i := 0; i < numGlyphs; i++ {
			gid := ot.GlyphID(i)
			b := [2]byte{byte(gid >> 8), byte(gid)}
			for _, c := range b {
				h ^= uint64(c)
				h *= prime64
			}
		}
		const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		var tag [6]byte
		v := h
		for i := 0; i < 6; i++ {
			tag[i] = letters[v%26]
			v /= 26
		}
		return string(tag[:])
	}

	rejectedA := rejectedTag(subA.NumGlyphs)
	rejectedB := rejectedTag(subB.NumGlyphs)
	if rejectedA != rejectedB {
		t.Fatalf("test bug: rejected output-numbering derivation must collide for equal NumGlyphs (got %q vs %q)", rejectedA, rejectedB)
	}

	// The assertion that actually exercises production and fails under
	// the mutation this test is named for: if fontset.go's deriveTag
	// call site used the output-numbering derivation instead of the
	// returned program bytes, subA.Tag would equal subB.Tag here, just
	// like the rejected derivation does above.
	if subA.Tag == subB.Tag {
		t.Fatalf(
			"deriveTag collided (%q) for two same-size, different-content glyph sets — indistinguishable from the "+
				"rejected output-numbering reading, which always collides for equal NumGlyphs (%q)",
			subA.Tag, rejectedA,
		)
	}
}

// testVariableFontBytes is Story 2.2's variable-font (fvar/gvar/avar,
// glyf outlines) test fixture, committed with its OFL 1.1 licence text
// and copyright line (AC25, AD-26) at
// folio-go/testdata/fonts/notosansthai-variable-testonly/NotoSansThai-VF.ttf
// — used ONLY to exercise axis pinning/instancing in this package's own
// tests (V5). It is NOT the shipped copy (folio-go/fonts/notosansthai/).
func testVariableFontBytes(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fonts", "notosansthai-variable-testonly", "NotoSansThai-VF.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read variable test font %s: %v", path, err)
	}
	return data
}

// TestSubsetPinnedInstancesProduceDifferentTags is V5's retained
// discrimination fixture (D-2.2.2's original fixture, retained under the
// superseding ruling): two PINNED INSTANCES of the SAME variable face,
// over the SAME glyph set, must receive DIFFERENT subset tags — because
// they are different embedded programs (different `gvar`-interpolated
// outlines), and AC6's tag hashes the program bytes. This is RED against
// the pre-Story-2.2 derivation (hash of Plan.GlyphSet() alone): two
// pinned instances retain the identical glyph-id closure for an
// identical rune set, so the old tag was identical too (B6) — this test
// fails under that reading and passes under the ruled one.
//
// BOTH instances are built by calling the vendor subsetter DIRECTLY
// from this test file, never through (*Font).Subset. Since D-2.2.4 that
// is not merely a stylistic choice: New() REJECTS a variable face
// outright (TestNewRejectsVariableFace below), so a variable fixture
// cannot reach (*Font).Subset at all. Production exposes no way to
// request an instance for D-2.2.4's reasons: textshape never rewrites
// OS/2.usWeightClass, so a pinned instance would carry metadata
// contradicting its own outlines; the shipped static faces halve the
// payload (4.82 MB compressed against 8.30); and deleting the seam
// removes the float `gvar` interpolation path from the render.
//
// CORRECTION, D-2.2.4 (correction, amended) / Story 2.3a Finding 1.
// This comment used to say instead that reaching
// subset.Input.PinAxisLocation "needs the identifier `float32`, which
// AD-23's arch guard bans under internal/ and the module root". THAT
// WAS FALSE — and it was falsified three lines below, by this test's
// own call at the `wght=700` line. The next paragraph always said so:
// the literal axis values below are UNTYPED constants; passed directly
// as PinAxisLocation's argument, Go converts them to float32 with no
// `float32`/`float64` identifier ever appearing in this file's source —
// confirmed by this package's own `go vet`/arch-guard gates passing.
// The identifier was never required, so the guard never fenced this
// door. AD-23 reaches this call now through lint's type-aware
// no-float-typed-value rule, which reports it as the float-typed value
// expression it is; this file is a named entry in that rule's test-scope
// inventory rather than an exemption.
func TestSubsetPinnedInstancesProduceDifferentTags(t *testing.T) {
	data := testVariableFontBytes(t)

	runes := []rune("กขค") // three Thai letters — identical rune set for both instances below.

	// instanceProgram builds one subset of the variable fixture over
	// `runes`, pinned as `pin` directs, entirely through the vendor API.
	instanceProgram := func(t *testing.T, label string, pin func(*subset.Input, *ot.Font)) ([]byte, int) {
		t.Helper()
		parsed, err := ot.ParseFont(data, 0)
		if err != nil {
			t.Fatalf("%s: ot.ParseFont: %v", label, err)
		}
		face, err := ot.NewFace(parsed)
		if err != nil {
			t.Fatalf("%s: ot.NewFace: %v", label, err)
		}
		cmap := face.Cmap()
		var gids []ot.GlyphID
		for _, r := range runes {
			gid, ok := cmap.Lookup(ot.Codepoint(r))
			if !ok {
				t.Fatalf("%s: no glyph for rune %U in test fixture", label, r)
			}
			gids = append(gids, gid)
		}
		input := subset.NewInput()
		input.AddGlyphs(gids...)
		pin(input, face.Font)
		plan, err := subset.CreatePlan(face.Font, input)
		if err != nil {
			t.Fatalf("%s: subset.CreatePlan: %v", label, err)
		}
		program, err := plan.Execute()
		if err != nil {
			t.Fatalf("%s: plan.Execute: %v", label, err)
		}
		return program, plan.NumOutputGlyphs()
	}

	defaultProgram, defaultGlyphs := instanceProgram(t, "default instance",
		func(in *subset.Input, f *ot.Font) { in.PinAllAxesToDefault(f) })
	pinnedProgram, pinnedGlyphs := instanceProgram(t, "wght=700 instance",
		func(in *subset.Input, _ *ot.Font) { in.PinAxisLocation(ot.MakeTag('w', 'g', 'h', 't'), 700) })

	defaultTag, err := deriveTag(defaultProgram)
	if err != nil {
		t.Fatalf("deriveTag(default instance program): %v", err)
	}
	pinnedTag, err := deriveTag(pinnedProgram)
	if err != nil {
		t.Fatalf("deriveTag(pinned wght=700 program): %v", err)
	}

	if defaultGlyphs != pinnedGlyphs {
		t.Fatalf(
			"test fixture assumption violated: NumGlyphs differ between the two instances (%d vs %d) — "+
				"not a same-glyph-set case; the fixture and rune set must be re-chosen",
			defaultGlyphs, pinnedGlyphs,
		)
	}
	if bytes.Equal(defaultProgram, pinnedProgram) {
		t.Fatalf(
			"test bug: two differently-pinned instances (default vs wght=700) produced BYTE-IDENTICAL programs — " +
				"the fixture's pinning had no effect, so this test cannot discriminate anything",
		)
	}
	if defaultTag == pinnedTag {
		t.Fatalf(
			"deriveTag collided (%q) for two DIFFERENT pinned instances of one face over the SAME glyph set — "+
				"exactly the collision AC6/D-2.2.2 (superseded) exists to make impossible by construction "+
				"(the old glyph-set-only tag WOULD collide here, since both instances retain the same glyph "+
				"closure for the same runes)",
			defaultTag,
		)
	}
}

// TestNewRejectsVariableFace is D-2.2.4's deleted-seam assertion. A
// caller-supplied face that still carries `fvar` must be REJECTED at
// face ingestion — not instanced, and not mid-render — so the failure
// is early, located and actionable.
//
// The message content is asserted, not just the error's existence: most
// Google Fonts downloads are variable builds today, so a caller hitting
// this needs an ACTION. A refusal that does not name the remedy would
// satisfy a nil-check test and still leave the caller stuck.
func TestNewRejectsVariableFace(t *testing.T) {
	_, err := New("noto-sans-thai-variable", testVariableFontBytes(t))
	if err == nil {
		t.Fatalf(
			"New accepted a VARIABLE face (one carrying `fvar`). D-2.2.4 requires rejection at ingestion: " +
				"PDF 1.7 cannot express a variable font, and silently pinning axes to their defaults is what " +
				"embedded Simplified Chinese as Thin (D-000.21) — Noto Sans SC's `wght` axis defaults to 100.",
		)
	}
	msg := err.Error()
	for _, want := range []string{
		"noto-sans-thai-variable", // located: names the face
		"fvar",                    // names the property that disqualified it
		"variable",                // in plain words, not just the table tag
		"instancer",               // names the remedy: instance it first
		"wght=400",                // and gives a concrete, runnable pointer
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf(
				"the variable-face rejection message does not contain %q, so it does not tell the caller "+
					"what to do about it.\nD-2.2.4 (binding): the message must NAME THE REMEDY.\ngot: %s",
				want, msg,
			)
		}
	}
}

// TestSubsetTagNonCircularity is V5a, binding (D-2.2.2 (superseded)): the
// tag's non-circularity must be ASSERTED BY A TEST, not merely stated in
// a comment. The tag string must never occur inside the program bytes
// it was derived FROM — if it did, a later change that patches the
// font's `name` table with the tag would make the derivation circular,
// and this test's failure must be legible as exactly that, not as an
// opaque assertion failure.
func TestSubsetTagNonCircularity(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sub, err := f.Subset(shapedGlyphIDs(t, f, "Hello, World!"))
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}
	if bytes.Contains(sub.Program, []byte(sub.Tag)) {
		t.Fatalf(
			"NON-CIRCULARITY VIOLATED: subset tag %q appears inside the program bytes it was derived FROM. "+
				"The tag must be computed from the program and never written back into it (e.g. into the "+
				"font's `name` table) — if something now does that, the derivation has become circular and "+
				"must be revisited (D-2.2.2 (superseded), V5a), not patched around by choosing a different tag.",
			sub.Tag,
		)
	}
}
