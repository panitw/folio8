package folio8

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fixtures/declared-variants/ is Story 11.5's artifact: THE FIRST
// DOCUMENT IN THIS REPOSITORY THAT DECLARES BOLD OR ITALIC AT ALL, and
// therefore the first recorded byte in the corpus that can tell a build
// which READS a declared cut from one which quietly draws bold text in
// the regular face.
//
// WHAT IT RED-PROVES, AND WHY IT HAD TO EXIST (DW-237). Story 11.2 gave
// chainFaceNames a second, index-aligned slice of styled names and made
// `entry.Variant(want)` the one place a cut is resolved. Nothing pinned
// the OUTCOME. Measured at this story's baseline, twice and by two
// independent mechanisms: `grep -a` for "bold" and for "italic" under
// fixtures/ returned NOTHING, and a byte-walk over every file in all 29
// fixture directories agreed — with "fontFamily" returning 23 files as
// the positive control, so the instrument was live. A resolver that
// silently returned the base face would have moved no golden, raised no
// diagnostic and redded no test. Story 11.2's DW-233 tripwire covers the
// MECHANISM behaviourally; this document covers the OUTCOME in bytes.
//
// THE PAGE IS DRAWN WITH FOUR DISTINCT FACES, and that is asserted by
// IDENTITY rather than by count alone — the /BaseFont PostScript names,
// the four distinct FontFile2 programs, and the four distinct font
// resources four Tf operators select. A count of four is satisfiable by
// four copies of one program; a set of four PostScript names is not.
//
// ITS GOLDEN IS ATTESTED, AND THE ROUTE THERE IS WORTH KEEPING. Under
// D-000.22 -> D-2.3.5 the claim "a person looked at this page and it is
// a real bold face rather than a thickened regular" is the one claim no
// agent may make on someone else's behalf. D-11.5.1 arm [A] ruled that
// the obligation ship as a RED gate rather than a register entry, and it
// did: declared_variants_signoff_matrix_test.go shipped failing, with the
// expected.pdf a CANDIDATE and the story HALTED. That red was TRANSIENT
// by construction — a countdown, not a floor nobody has met — and it was
// discharged inside the story, on 2026-09-06, when Panit Wechasil read
// the page and the record landed in
// fixtures/declared-variants/signoff.json. The gate is green now, and it
// reds again by construction the moment the golden is re-recorded, which
// is D-2.3.5's anti-rot condition doing its job rather than a
// regression.

// declaredVariantsFixtureDir is the fixture's home, relative to the repo
// root. Declared once: this file, the matrix gate and the sign-off gate
// all address the same directory, and three spellings of one path is
// three chances for one of them to point somewhere that does not exist.
const declaredVariantsFixtureDir = "declared-variants"

// declaredVariantsPostScriptNames is the set of embedded PostScript
// names this document's page must carry — one per declared cut, and the
// positive identity behind the whole fixture.
//
// They are the programs' OWN names, read out of /BaseFont with the
// six-letter subset tag stripped (ISO 32000-1 Table 117), never a key
// the renderer chose: a build that resolved four resources to the same
// underlying face would satisfy a resource count and fail here.
var declaredVariantsPostScriptNames = map[string]bool{
	"Roboto-Regular":    true,
	"Roboto-Bold":       true,
	"Roboto-Italic":     true,
	"Roboto-BoldItalic": true,
}

// declaredVariantsCentredRegularXMilli and …BoldXMilli are the centred
// pair's line-start x origins, in millipoints, AS MEASURED off the
// recorded artifact.
//
// THEY ARE PINNED EXACTLY, and not merely relationally, because the
// relational clause alone (bold starts left of regular) is satisfied by
// BOTH LINES MOVING TOGETHER to some third face — a real defect that the
// ordering check cannot see. Two clauses that fail for different reasons
// are two clauses; one of them restated is one.
//
// Pinning them costs nothing a moved golden does not already cost. These
// numbers move only when the rendered bytes move, and a deliberate
// re-record already invalidates the human attestation by digest
// (D-2.3.5's anti-rot condition) and demands a fresh reading — so a story
// that legitimately moves them is a story that was going to edit this
// fixture's recorded values anyway.
//
// The arithmetic that ties them to the page: centring places a line at
// left + (box - measured) / 2, so HALF of any width difference shows in
// the origin. The 1824-millipoint offset between these two is therefore a
// line 3648 millipoints — 3.648 pt — wider, on a regular line measuring
// 206.904 pt. That is ~1.76% wider, not ~1.4%: the doubling is the step
// it is easy to drop.
const (
	declaredVariantsCentredRegularXMilli = 132548
	declaredVariantsCentredBoldXMilli    = 130724
)

// declaredVariantsResourceName spells a FontSet face name the way
// internal/pdf spells it in a resource dictionary and a content stream:
// keep [A-Za-z0-9_-], drop everything else (pdfNameEscape, textdoc.go).
//
// It is a THIRD spelling of that rule in this repository — the authority
// is internal/pdf's own unexported pdfNameEscape, and chain_face_names_
// test.go carries a second for the embedded-face namespace. It is
// tolerable here for the same reason that one is: it is checked against
// REAL OUTPUT on every run, so a wrong escape fails loudly against the
// produced bytes rather than agreeing with itself.
func declaredVariantsResourceName(faceName string) string {
	var b strings.Builder
	for _, r := range faceName {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// declaredVariantsExpectedResourceNames is the resource name EACH OF THE
// SIX RUNS must select, in page order, held as a LITERAL.
//
// WHY INDEX-KEYED AND NOT A SET (D-11.1.16). A set of four names is
// satisfied by a resolver that SWAPS two of them — italic drawn in the
// boldItalic face and boldItalic in the italic. All four cuts are
// present, all four are distinct, all four PostScript names appear, and
// the page is wrong in exactly the way this fixture exists to catch.
// Only a per-run assertion sees it.
//
// WHY A LITERAL, AND THIS IS THE CORRECTION THAT MATTERS. The first
// version of this derived each name by calling entry.Variant(style) on
// the parsed document, and its comment argued that reading the DOCUMENT
// is not reading the resolver's own input. MEASURED, AND THE ARGUMENT WAS
// FALSE: entry.Variant is exactly the lookup chainFaceNames performs, so
// swapping FontStyleItalic and FontStyleBoldItalic in
// internal/template/model.go's fontChainVariants table moved BOTH sides
// together and TestDeclaredVariantsEmitsFourCuts PASSED a page whose e3
// and e4 were drawn in each other's faces. Only the golden digest caught
// it. That is D-11.2.8 precisely — an assertion whose two sides could be
// equal is not an assertion — and the rule D-11.2.2 states for this
// repository is literal-vs-derived, never derived-vs-derived.
//
// e5/e6 repeat e1's and e2's faces — the centred pair is the same regular
// and bold cuts again — which is why six runs select four distinct names.
var declaredVariantsExpectedResourceNames = []string{
	"Roboto",           // e1 — the base face
	"RobotoBold",       // e2 — the `bold` sibling
	"RobotoItalic",     // e3 — the `italic` sibling
	"RobotoBoldItalic", // e4 — the `boldItalic` sibling
	"Roboto",           // e5 — centred pair, regular half
	"RobotoBold",       // e6 — centred pair, bold half
}

// declaredVariantsAssertLiteralMatchesTheDocument ties the literal above
// back to the committed document, in the one direction that is safe.
//
// It reads the chain entry's STRUCT FIELDS (Face, Bold, Italic,
// BoldItalic) rather than calling Variant, so the style->field dispatch
// the literal exists to police is not on both sides of this comparison
// either. What it catches is the literal going stale: if the fixture ever
// declared different cuts, the literal would silently stop describing it.
func declaredVariantsAssertLiteralMatchesTheDocument(t *testing.T, fatalf func(format string, args ...any)) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(declaredVariantsTemplateJSON))
	if err != nil {
		fatalf("parse declared-variants template: %v", err)
		return
	}
	chain, ok := tpl.doc.Fonts.Chain("body")
	if !ok || len(chain) != 1 {
		fatalf("the fixture's `body` chain must be exactly ONE entry (found %d, present=%v)", len(chain), ok)
		return
	}
	e := chain[0]
	want := []string{
		declaredVariantsResourceName(e.Face),
		declaredVariantsResourceName(e.Bold),
		declaredVariantsResourceName(e.Italic),
		declaredVariantsResourceName(e.BoldItalic),
		declaredVariantsResourceName(e.Face),
		declaredVariantsResourceName(e.Bold),
	}
	for i := range want {
		if want[i] != declaredVariantsExpectedResourceNames[i] {
			fatalf("declaredVariantsExpectedResourceNames[%d] is %q, but the committed document declares %q for that run — the literal has gone stale against the fixture it describes",
				i, declaredVariantsExpectedResourceNames[i], want[i])
			return
		}
	}
}

// declaredVariantsAssertBaseFontNames is assertBaseFontNames' assertion,
// reported through the CALLER'S fatalf.
//
// assertBaseFontNames reports through t.Errorf, which on a matrix leg
// loses the target's name and — worse — does not stop the leg. The
// /BaseFont check is the positive identity behind this whole fixture, and
// requireDeclaredVariantsUsesFourCuts states the rule it must obey: a
// matrix leg comparing bytes it has not first established are the RIGHT
// bytes is worse than no leg. So this one routes through fatalf like
// every other clause in the guard.
//
// It reuses baseFontPattern (shipped_faces_test.go), so the ISO 32000-1
// §9.6.4 subset-tag shape is checked by the same regexp and not by a
// second copy of it.
func declaredVariantsAssertBaseFontNames(t *testing.T, raw []byte, fatalf func(format string, args ...any)) {
	t.Helper()
	matches := regexp.MustCompile(`/BaseFont\s*/([^\s/\[\]<>()]+)`).FindAllSubmatch(raw, -1)
	if len(matches) == 0 {
		fatalf("PRESENCE PRECONDITION FAILED — the PDF contains no /BaseFont entry at all, so every assertion here would pass vacuously")
		return
	}
	seen := map[string]bool{}
	for _, m := range matches {
		got := string(m[1])
		parts := baseFontPattern.FindStringSubmatch(got)
		if parts == nil {
			fatalf("/BaseFont %q does not match ^[A-Z]{6}\\+.+$ — ISO 32000-1 §9.6.4 requires exactly a six-upper-case-letter subset tag, one '+', then the CIDFont program's name", got)
			return
		}
		psName := parts[2]
		if strings.Contains(psName, "+") {
			fatalf("/BaseFont %q carries MORE THAN ONE '+' — the subset tag looks doubly applied", got)
			return
		}
		if !declaredVariantsPostScriptNames[psName] {
			fatalf("/BaseFont %q names PostScript font %q, which is not one of this fixture's four declared cuts %v.\nISO 32000-1 Table 117: /BaseFont shall be the CIDFontName in the CIDFont program — not the FontSet key the caller happened to file the face under", got, psName, sortedKeys(declaredVariantsPostScriptNames))
			return
		}
		seen[psName] = true
	}
	for name := range declaredVariantsPostScriptNames {
		if !seen[name] {
			fatalf("expected PostScript name %q never appeared in any /BaseFont — the page is not set in all four declared cuts", name)
			return
		}
	}
}

// renderDeclaredVariants renders the fixture from the const this package
// commits — the same const the four matrix legs render — and fails on
// any diagnostic at all.
//
// ZERO DIAGNOSTICS IS PART OF THE FIXTURE'S DEFINITION, not a nicety.
// shapeSegments raises DiagCodeTextStyleFaceUndeclared whenever a styled
// element lands on an entry declaring no variant for it; a correctly
// authored document on a chain that declares all three cuts never
// reaches that arm, so a diagnostic here means the fixture stopped being
// the clean case it exists to be — a thing to fix in the document, never
// a thing to exempt.
func renderDeclaredVariants(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(declaredVariantsTemplateJSON))
	if err != nil {
		t.Fatalf("parse declared-variants template: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render declared-variants: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the declared-variants fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// declaredVariantsAssertFourCuts is this document's SEMANTIC GUARD,
// DECLARED ONCE here and read by this file's own test AND by
// matrix_test.go's per-leg feature guard (which is matrix-tagged and
// would otherwise carry a second copy — the two-literals hazard that
// makes a guard agree with itself while disagreeing with the document,
// D-7.4.5).
//
// It asserts two independent properties, and they fail for different
// reasons, which is why they are two blocks and not one:
//
//  1. THE FOUR CUTS REACHED THE PAGE. Six runs, four distinct font
//     resources selected by four Tf operators, four distinct FontFile2
//     programs, and four distinct /BaseFont PostScript names. A resolver
//     that returned the base face for a declared variant collapses all
//     four to one and every clause here fails.
//
//  2. BOLD METRICS REACHED LAYOUT. The centred pair — same string, same
//     400 pt box, one regular and one bold — must start at DIFFERENT x,
//     with the bold line starting further LEFT because it measures
//     wider, AND at the two exact origins this fixture recorded. Equal x
//     means the layout measured the regular face and only the drawing
//     switched, which is a defect a digest alone would ratify; two
//     origins that hold their order while both moving is a defect the
//     relational clause alone cannot see.
//
// The obvious version of (2) was a WRAP demonstration and it was
// vacuous: swept across 14 box widths from 150 to 340 pt, regular and
// bold took the same number of lines every single time. Bold metrics do
// reach layout — Roboto's bold is only ~1.76% wider than its regular
// (the centred origins differ by 1.824 pt, and centring shows HALF the
// width difference, so the line is 3.648 pt wider on a 206.904 pt line),
// so a break almost never moves. The centred pair is the sensitive form
// of the same assertion, and it is pinned unequal against real output.
//
// fatalf is the caller's own failure reporter, so a matrix leg can name
// its target in the message.
func declaredVariantsAssertFourCuts(t *testing.T, raw []byte, fatalf func(format string, args ...any)) {
	t.Helper()

	streams := splitPageContentStreams(t, raw)
	if len(streams) != 1 {
		fatalf("declared-variants is a one-page document; got %d page content stream(s)", len(streams))
		return
	}

	runs := readEmittedRuns(t, raw)
	if len(runs) != 6 {
		fatalf("the emitted content stream carries %d text run(s), want 6 (e1..e6, one line each) — this guard would assert nothing about a document that drew something else", len(runs))
		return
	}

	// (1) THE FOUR CUTS, BY IDENTITY, RUN BY RUN.
	//
	// e1..e4 are the four styles in authored order, so their four
	// resources must be pairwise distinct. e5/e6 re-select two of the
	// same four, which is why the DISTINCT count over all six runs is
	// four and not six.
	seen := map[string]int{}
	for i, r := range runs {
		if r.Resource == "" {
			fatalf("run %d selected no font resource", i)
			return
		}
		seen[r.Resource]++
	}
	if len(seen) != 4 {
		fatalf(
			"the page selects %d distinct font resource(s), want 4 (one per declared cut) — a resolver that answered a declared variant with the entry's BASE face collapses them, which is exactly DW-237's failure.\nresources: %v",
			len(seen), seen,
		)
		return
	}
	// THE INDEX IS THE ASSERTION (D-11.1.16). Four distinct names in
	// four runs is satisfied by a resolver that SWAPPED two of them —
	// bold drawn in the italic face, italic in the bold — which is the
	// index-keyed defect a set comparison cannot see. Each run is tied to
	// a LITERAL face name, not to entry.Variant's answer — see
	// declaredVariantsExpectedResourceNames for the measurement that
	// forced that (a derived expectation moved with the defect and passed).
	declaredVariantsAssertLiteralMatchesTheDocument(t, fatalf)
	wantRes := declaredVariantsExpectedResourceNames
	if len(wantRes) != len(runs) {
		fatalf("hold %d expected resource name(s) for %d runs", len(wantRes), len(runs))
		return
	}
	elements := []string{"e1 (regular)", "e2 (bold)", "e3 (italic)", "e4 (boldItalic)", "e5 (centred regular)", "e6 (centred bold)"}
	for i, r := range runs {
		if r.Resource != wantRes[i] {
			fatalf(
				"run %d, %s, selected font resource %q but the document declares %q for that style — the cuts are present but MISASSIGNED, which a set comparison over the four names cannot see (D-11.1.16).\nper-run resources: got %v, want %v",
				i, elements[i], r.Resource, wantRes[i],
				[]string{runs[0].Resource, runs[1].Resource, runs[2].Resource, runs[3].Resource, runs[4].Resource, runs[5].Resource},
				wantRes,
			)
			return
		}
	}
	programs := extractAllFontFile2Programs(t, raw)
	if len(programs) != 4 {
		fatalf("the render embeds %d font program(s), want exactly 4 — one subset per declared cut, and no more", len(programs))
		return
	}
	for i := range programs {
		for j := i + 1; j < len(programs); j++ {
			if string(programs[i]) == string(programs[j]) {
				fatalf("embedded font programs %d and %d are byte-identical — four resources resolved to the same face, so the page is not set in four cuts", i, j)
				return
			}
		}
	}
	// The PDF-level identity: the programs' own PostScript names. A
	// count of four distinct programs is satisfiable by four different
	// subsets of ONE face; a set of four names is not. Routed through
	// the caller's fatalf so a matrix leg names its target AND STOPS.
	declaredVariantsAssertBaseFontNames(t, raw, fatalf)

	// (2) BOLD METRICS REACHED LAYOUT.
	//
	// e5 (regular) and e6 (bold) are the same string, centred in the
	// same 400 pt box. Centring places the line at
	// left + (box - measured) / 2, so a WIDER line starts further LEFT
	// by HALF the width difference.
	//
	// TWO CLAUSES, FAILING FOR DIFFERENT REASONS. The relational one
	// catches "bold measured as regular" (equal x) and "bold measured
	// narrower" (wrong order). The exact one catches what the relational
	// one structurally cannot: BOTH lines moving together to some third
	// face, which preserves the ordering and the sign while changing the
	// page.
	regular, bold := runs[4].OriginXMilli, runs[5].OriginXMilli
	if regular == bold {
		fatalf(
			"the centred pair starts at the SAME x (%d millipoints) — the same string measured identically in the regular and the bold face, so bold metrics never reached layout and only the drawing switched",
			regular,
		)
		return
	}
	if bold >= regular {
		fatalf(
			"the centred BOLD line starts at x=%d millipoints, at or right of the regular line's x=%d — a centred line that measures wider must start further LEFT, so this is not the bold face's metrics",
			bold, regular,
		)
		return
	}
	if regular != declaredVariantsCentredRegularXMilli || bold != declaredVariantsCentredBoldXMilli {
		fatalf(
			"the centred pair starts at x=%d (regular) and x=%d (bold) millipoints, want exactly %d and %d.\n"+
				"The relational clause above is satisfied by BOTH lines shifting together to a third face, so the "+
				"measured values are pinned as well. If this is a deliberate re-record, move these constants in the "+
				"same commit — and note that a re-record already invalidates the human attestation in "+
				"fixtures/declared-variants/signoff.json by digest, so a fresh reading is owed regardless.",
			regular, bold,
			int64(declaredVariantsCentredRegularXMilli), int64(declaredVariantsCentredBoldXMilli),
		)
		return
	}

	t.Logf(
		"declared-variants witness — 6 runs selecting 4 distinct resources %v, 4 distinct embedded programs; the centred pair starts at %d (regular) against %d (bold) millipoints, a bold line %d millipoints wider",
		seen, regular, bold, 2*(regular-bold),
	)
}

// TestDeclaredVariantsEmitsFourCuts is the semantic half, and it runs
// BEFORE the digest assertion in file order and in intent (D-000.22): a
// hash frozen before anyone checked what it contained certifies only
// that the bytes have not changed.
func TestDeclaredVariantsEmitsFourCuts(t *testing.T) {
	declaredVariantsAssertFourCuts(t, renderDeclaredVariants(t), t.Fatalf)
}

// TestDeclaredVariantsIsCanonicalAndDeclaresTwoPointZero is the
// document's own acceptance, copied wholesale from
// TestEmbeddedFontFixtureIsCanonicalAndDeclaresTwoPointZero.
//
// The version half is not decoration. An object-form chain entry raises
// the saved version through fontsRequireMajor, which is keyed on
// FontChainEntry.SerialisesAsObject — the SAME predicate writeFontChain
// uses to choose the emitted shape. A document that serialised as an
// object while declaring "1.0" would be a version that lies, and sharing
// the predicate is what makes that unrepresentable. This asserts the
// TEMPLATE CONST is on the right side of it. It does not open
// fixtures/declared-variants/input.folio — TestDeclaredVariantsGolden-
// Fixture below is what ties the committed file to this const, byte for
// byte, and a second copy of that check here would buy nothing.
func TestDeclaredVariantsIsCanonicalAndDeclaresTwoPointZero(t *testing.T) {
	source := declaredVariantsTemplateJSON

	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) != source {
		t.Fatalf("the fixture is not a serializer fixed point — the engine would rewrite it on save:\n--- got ---\n%s\n--- want ---\n%s", firstBytes(string(out)), firstBytes(source))
	}
	if !strings.Contains(source, `"version": "2.0"`) {
		t.Error(`a document whose chain entry serialises as an OBJECT must declare version 2.0 (fontsRequireMajor, keyed on FontChainEntry.SerialisesAsObject)`)
	}
	// The chain really is the object form Story 11.2 introduced: the
	// base face plus all three cuts. Without this the version check
	// above would pass over a document that declared "2.0" for some
	// other reason and no cut at all.
	for _, want := range []string{
		`"face": "Roboto"`,
		`"bold": "Roboto Bold"`,
		`"italic": "Roboto Italic"`,
		`"boldItalic": "Roboto Bold Italic"`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the fixture's chain entry no longer carries %s — the entry must name its base face AND all three cuts, or it stops witnessing the resolution it exists to pin", want)
		}
	}
	// And the four styles the elements declare, which is the other half
	// of the pair: a chain declaring three cuts that nothing asks for
	// draws one face.
	for _, want := range []string{`"bold": true`, `"italic": true`} {
		if !strings.Contains(source, want) {
			t.Errorf("the fixture no longer declares %s on any element — the corpus is back to the DW-237 state where no committed document asks for a cut", want)
		}
	}
}

// TestDeclaredVariantsGoldenFixture is the byte-identity half: the live
// render must reproduce fixtures/declared-variants/expected.pdf exactly,
// and the committed input.folio must still be byte-identical to the const
// this package renders (the hand-sync precedent font-text,
// multi-script-fallback, wrapped-text, mandatory-break, line-spacing,
// justified-text, justified-thai, alignment-rounding and
// thai-stacked-marks set).
func TestDeclaredVariantsGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", declaredVariantsFixtureDir)

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != declaredVariantsTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/declaredVariantsTemplateJSON (declared_variants_template.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per line-spacing's precedent)",
			inputPath,
		)
	}

	fixture := loadExpectedFixture(t, filepath.Join(dir, "expected.json"))
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not 64 lower-case hex characters", fixture.SHA256)
	}

	b := renderDeclaredVariants(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/declared-variants/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/declared-variants). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}
