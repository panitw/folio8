package template

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// This file is Story 11.2's FORMAT half: the object-form chain entry, its
// closed variant key set, and the one predicate that keeps the emitted
// shape and the declared version from ever disagreeing.

// TestTheVariantKeySetIsClosedAndTheFormatDocSaysSo is the story's ONE
// targeted enumeration guard, and both halves of it are deliberate.
//
// (a) LITERAL-VS-DERIVED, NEVER DERIVED-VS-DERIVED. The expected set is
// spelled out here as three string literals and compared to the parser's
// own enumeration. A test that read the enumeration on both sides would
// move with it and guard nothing.
//
// (b) THE DOC ROW MUST EXIST, AND IT MUST BE THE CHAIN ENTRY'S OWN. The
// slice is `## `+"`fonts`"+“ to the next `^## `, so `style`'s
// `| `+"`bold`, `italic`"+` |` row — which lives inside `### style`,
// several sections later — is outside it BY CONSTRUCTION. Without that
// scoping the assertion would pass on style's row alone and prove
// nothing about chain entries at all, which is exactly the trap the
// shared key spellings set. Red-proof: delete the chain-entry row and
// this reds while style's row stays put.
//
// (c) AND THE SERIALIZER MUST EMIT EVERY MEMBER. serialize.go spells the
// three keys as literal kv{} pairs (so drift_test.go's AST extractor can
// see them), which is a second copy of the spelling; this closes the
// gap that copy opens.
func TestTheVariantKeySetIsClosedAndTheFormatDocSaysSo(t *testing.T) {
	// (a) The literal set. Adding a member here without adding it to
	// model.go's enumeration — or the reverse — is the failure.
	want := []string{"bold", "italic", "boldItalic"}
	got := fontChainVariantKeys()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the parser's variant enumeration is %v, want exactly %v — the set is CLOSED, and extending it is a MAJOR change the format doc must price", got, want)
	}

	// (c) Every member reaches the emitted bytes.
	entry := FontChainEntry{Face: "Roboto"}
	for _, v := range fontChainVariants {
		*v.field(&entry) = "Face " + v.key
	}
	emitted := string(writeFontChain(nil, 1, []FontChainEntry{entry}))
	for _, key := range want {
		if !strings.Contains(emitted, `"`+key+`"`) {
			t.Errorf("the serializer never emits the closed set's member %q:\n%s", key, emitted)
		}
	}

	// (b) The format doc carries a row per key, inside the `fonts`
	// section and nowhere else.
	doc := string(mustReadFile(t, filepath.Join(repoRootFromTest(t), "_bmad-output", "specs", "spec-folio", "folio-format.md")))
	lines := strings.Split(doc, "\n")
	heading := -1
	headings := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "## `fonts`" {
			headings++
			heading = i
		}
	}
	if headings != 1 {
		t.Fatalf("folio-format.md has %d `## \\x60fonts\\x60` headings, want exactly 1 — the section slice below is only well defined when there is one", headings)
	}
	end := len(lines)
	for i := heading + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	section := lines[heading:end]

	// The vacuity guard, stated as the trap it closes: style's row must
	// NOT be inside the slice, or every assertion below could be
	// satisfied by a row about a completely different key.
	styleRow := regexp.MustCompile("^\\| `bold`, `italic` \\|")
	for _, line := range section {
		if styleRow.MatchString(strings.TrimSpace(line)) {
			t.Fatalf("`style`'s own bold/italic row is inside the `fonts` section slice, so this guard cannot tell a chain-entry row from it:\n%s", line)
		}
	}

	for _, key := range want {
		found := false
		for _, line := range section {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "|") {
				continue
			}
			cells := strings.SplitN(trimmed, "|", 3)
			if len(cells) < 2 {
				continue
			}
			if strings.Contains(cells[1], "`"+key+"`") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("folio-format.md's `fonts` section documents no chain-entry row for the variant key %q — the closed set and the doc have drifted", key)
		}
	}
}

// TestObjectFormAndTheSavedVersionCannotDisagree is the VERSION TIE,
// proved BEHAVIOURALLY: output against output, never the shared
// predicate against itself.
//
// The property is an equivalence — the serialized entry begins with '{'
// IF AND ONLY IF versionForSave raises the document to 2.0 — and it is
// asserted over a literal table of entry shapes that covers both sides
// of it. A test that asked `SerialisesAsObject() == SerialisesAsObject()`
// would be trivially true and would have shipped the exact defect this
// story found: a `{"face": …, "bold": …}` entry stamped 1.0 on a
// document no 1.x reader can decode.
func TestObjectFormAndTheSavedVersionCannotDisagree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry FontChainEntry
	}{
		{"a bare face", FaceEntry("Roboto")},
		{"an embedded face", AssetEntry("k")},
		{"a face with a bold variant", FontChainEntry{Face: "Roboto", Bold: "Roboto Bold"}},
		{"a face with an italic variant", FontChainEntry{Face: "Roboto", Italic: "Roboto Italic"}},
		{"a face with a boldItalic variant", FontChainEntry{Face: "Roboto", BoldItalic: "Roboto Bold Italic"}},
		{"a face with all three", FontChainEntry{Face: "Roboto", Bold: "B", Italic: "I", BoldItalic: "BI"}},
		{"an embedded face with a bold variant", FontChainEntry{AssetKey: "k", Bold: "k2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The SERIALIZER's real output.
			emitted := string(writeFontChain(nil, 1, []FontChainEntry{tc.entry}))
			firstEntryLine := ""
			for _, line := range strings.Split(emitted, "\n") {
				if s := strings.TrimSpace(line); s != "" && s != "[" {
					firstEntryLine = s
					break
				}
			}
			emitsObject := strings.HasPrefix(firstEntryLine, "{")

			// The VERSION's real output, from a whole document whose only
			// version-raising content is this chain.
			d := &Document{Version: baseVersion, Fonts: Fonts{"body": {tc.entry}}}
			raises := versionForSave(d.Version, d) == majorFeatureVersion

			if emitsObject != raises {
				t.Fatalf("the serializer emitted %q (object=%v) while versionForSave said %q (raises=%v) — an entry shape a 1.x reader cannot decode must declare 2.0, and one it can must not",
					firstEntryLine, emitsObject, versionForSave(d.Version, d), raises)
			}
		})
	}

	// The table must cover BOTH sides of the equivalence, or an
	// implementation that answered "true" (or "false") to everything
	// would pass it.
	var sawBare, sawObject bool
	for _, e := range []FontChainEntry{FaceEntry("Roboto"), {Face: "Roboto", Bold: "Roboto Bold"}} {
		if e.SerialisesAsObject() {
			sawObject = true
		} else {
			sawBare = true
		}
	}
	if !sawBare || !sawObject {
		t.Fatal("coverage witness: the table exercises only one side of the equivalence")
	}
}

// TestAnEntryWithNoVariantStillSerialisesAsABareString is AC5's
// serialization half: a chain of plain names is byte-identical to what
// it always was, so the twenty-odd recorded digests cannot move.
func TestAnEntryWithNoVariantStillSerialisesAsABareString(t *testing.T) {
	got := string(writeFontChain(nil, 1, []FontChainEntry{FaceEntry("Noto Sans"), FaceEntry("Noto Sans Thai")}))
	const want = "[\n    \"Noto Sans\",\n    \"Noto Sans Thai\"\n  ]"
	if got != want {
		t.Fatalf("a variant-free chain no longer serialises as bare strings:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestTheObjectFormRoundTripsAsAFixedPoint: parse -> serialize -> parse
// over a document whose chain carries every shape, so the new keys are
// canonical rather than merely accepted.
func TestTheObjectFormRoundTripsAsAFixedPoint(t *testing.T) {
	source := twoFontAssetDoc(fontAssetBody, secondFontAssetBody,
		`["Noto Sans", {"boldItalic": "Noto Sans Bold Italic", "italic": "Noto Sans Italic", "bold": "Noto Sans Bold", "face": "Noto Sans"}, {"asset": "`+embeddedFontKey+`", "bold": "`+secondFontKey+`"}]`)
	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	// Keys are emitted byte-sorted, so the authored order above (which is
	// deliberately shuffled) must come back canonicalised.
	order := []string{`"bold": "Noto Sans Bold"`, `"boldItalic": "Noto Sans Bold Italic"`, `"face": "Noto Sans"`, `"italic": "Noto Sans Italic"`}
	at := -1
	for _, want := range order {
		i := strings.Index(string(out), want)
		if i < 0 {
			t.Fatalf("the object entry never emitted %s:\n%s", want, out)
		}
		if i < at {
			t.Fatalf("the object entry's keys are not emitted byte-sorted (%s came early):\n%s", want, out)
		}
		at = i
	}
	again, err := ParseDocument(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	out2, err := SerializeDocument(again)
	if err != nil {
		t.Fatalf("reserialize: %v", err)
	}
	if string(out) != string(out2) {
		t.Fatalf("the object form is not a fixed point:\n%s\n---\n%s", out, out2)
	}
	if got := d.Fonts["body"][1]; got.Face != "Noto Sans" || got.Bold != "Noto Sans Bold" || got.Italic != "Noto Sans Italic" || got.BoldItalic != "Noto Sans Bold Italic" {
		t.Fatalf("the parsed entry lost a variant: %+v", got)
	}
	if got := d.Fonts["body"][2]; got.AssetKey != embeddedFontKey || got.Bold != secondFontKey {
		t.Fatalf("the parsed embedded entry lost its variant asset key: %+v", got)
	}
}

// TestTwoVariantsOfOneEntryMayNameTheSameFace is DW-241's OTHER DIRECTION, and
// it is not optional (D-11.2.11).
//
// The narrowing compares each variant to THAT ARM'S OWN DISCRIMINANT and to
// nothing else. A cross-variant collision — one face declared as both the bold
// and the italic cut — is LEGAL and SILENT: it is a real declaration, whatever
// a reader thinks of it, and refusing it would be a second, wider rule wearing
// the first one's name. Run the guard against the nearest LEGITIMATE spelling,
// not only against the defect (D-11.3.7).
func TestTwoVariantsOfOneEntryMayNameTheSameFace(t *testing.T) {
	source := embeddedFontDoc(fontAssetBody, `[{"face": "Roboto", "bold": "Roboto Bold", "italic": "Roboto Bold"}]`)
	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("a cross-variant collision must LOAD — only the base is privileged: %v", err)
	}
	entry := d.Fonts["body"][0]
	if entry.Bold != "Roboto Bold" || entry.Italic != "Roboto Bold" {
		t.Fatalf("the parsed entry lost a variant: %+v", entry)
	}
	// AND IT ROUND-TRIPS AS A FIXED POINT, so nothing downstream quietly
	// deduplicates the two into one.
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	again, err := ParseDocument(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	out2, err := SerializeDocument(again)
	if err != nil {
		t.Fatalf("reserialize: %v", err)
	}
	if string(out) != string(out2) {
		t.Fatalf("a cross-variant collision is not a fixed point:\n%s\n---\n%s", out, out2)
	}
	// THE POSITIVE CONTROL FOR THE GUARD ITSELF: change one of those two to the
	// entry's own base and the SAME document is refused, so this test's green
	// is a statement about the collision and not about the check being absent.
	requireLoadError(t, embeddedFontDoc(fontAssetBody, `[{"face": "Roboto", "bold": "Roboto", "italic": "Roboto Bold"}]`), "fonts.body[0].bold")
}

// TestTheSelfReferenceCheckIsSTRINGEQUALITYAndSaysSo is DW-245: the
// DISCLOSED LIMIT of the D-11.2.11 refusal, asserted rather than described.
//
// The refusal compares a variant to its entry's own discriminant with string
// equality, so its reach ends exactly one character from the base. A variant
// naming a DIFFERENT face that happens to hold IDENTICAL BYTES renders
// bold-as-regular just as silently, and nothing here can see it: detecting it
// would mean comparing what is inside the two faces, which D-11.2.1 forbids.
//
// THIS TEST EXISTS BECAUSE THE LIMIT IS THE EASY THING TO FORGET. A reader who
// meets the self-reference refusal concludes that bold-as-regular-with-no-
// warning is closed. It is closed for the case an author reaches by writing
// the same name twice and open for the case they reach by a stranger route,
// and *a check whose limit is unstated ages into a false reassurance*. So the
// behaviour is pinned here AND the two required statements of the limit are
// pinned with it — otherwise either could be deleted and every test stay green.
//
// It is also D-11.3.7's discipline applied to this guard: run the pattern
// against the defect it forbids AND against the nearest LEGITIMATE spelling.
// `"Roboto Copy"` is that nearest spelling, one edit away from the refusal.
func TestTheSelfReferenceCheckIsSTRINGEQUALITYAndSaysSo(t *testing.T) {
	// The nearest legitimate spelling LOADS. This is the limit, exercised.
	source := embeddedFontDoc(fontAssetBody, `[{"face": "Roboto", "bold": "Roboto Copy"}]`)
	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("a variant naming a DIFFERENT face must load — the check is string equality against the base, not a likeness test: %v", err)
	}
	if got := d.Fonts["body"][0].Bold; got != "Roboto Copy" {
		t.Fatalf("the parsed entry lost or rewrote its variant: %q", got)
	}

	// THE POSITIVE CONTROL, and it is what makes the green above mean
	// something: strip " Copy" — one edit — and the SAME document is refused.
	// Without this, a test asserting "this loads" would pass just as well if
	// the self-reference check had never been written at all.
	requireLoadError(t, embeddedFontDoc(fontAssetBody, `[{"face": "Roboto", "bold": "Roboto"}]`), "fonts.body[0].bold")

	// AND THE LIMIT IS STATED IN BOTH PLACES DW-241's RULING REQUIRES.
	// Neither statement has any other guard: the doc guard above reads a table
	// row's FIRST cell only, and nothing at all reads the comment in parse.go.
	root := repoRootFromTest(t)
	for _, site := range []struct {
		what string
		path string
		want []string
	}{
		{
			"the comment beside the check in parse.go",
			filepath.Join(root, "folio-go", "internal", "template", "parse.go"),
			[]string{"Roboto Copy", "IDENTICAL BYTES"},
		},
		{
			"the format doc's chain-entry section",
			filepath.Join(root, "_bmad-output", "specs", "spec-folio", "folio-format.md"),
			[]string{"Roboto Copy", "identical bytes"},
		},
	} {
		text := string(mustReadFile(t, site.path))
		for _, want := range site.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s no longer discloses the limit of the self-reference refusal (missing %q). An undisclosed limit reads as a closed hole (DW-245).", site.what, want)
			}
		}
	}
}

// TestAVariantFreeObjectEntryCanonicalisesBackToABareString is P8, and
// it is the round trip the whole version rule rests on.
//
// `{"face": "X"}` with no siblings is LEGAL — the object form's `face`
// discriminant does not require a variant — and it carries exactly the
// information a bare `"X"` carries. So it must come back as `"X"`, and a
// document containing nothing else must stay at 1.0. If it serialised as
// an object it would raise itself to 2.0 on every save, and a `1.0`
// document would become a `2.0` document for writing down the same
// chain a second way.
//
// This is where "the version rule is applied ON SAVE" bites: the loaded
// document's own `version` string is carried verbatim, and versionForSave
// is what decides what the NEXT save declares. Both are asserted.
func TestAVariantFreeObjectEntryCanonicalisesBackToABareString(t *testing.T) {
	source := embeddedFontDoc(fontAssetBody, `["Noto Sans", {"face": "Noto Sans Thai"}]`)
	// The document declares 2.0 because it CARRIES a font asset the
	// chain does not name... no: embeddedFontDoc's asset is unreferenced
	// here, so nothing in this document requires 2.0 at all.
	source = strings.Replace(source, `"version": "2.0"`, `"version": "1.0"`, 1)

	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("a variant-free object entry did not load: %v", err)
	}
	if got := d.Fonts["body"][1]; got.Face != "Noto Sans Thai" || got.SerialisesAsObject() {
		t.Fatalf("a variant-free `face` entry parsed as %+v — it carries exactly what a bare string carries, so it must not serialise as an object", got)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if strings.Contains(string(out), `"face"`) {
		t.Fatalf("a variant-free object entry was written back as an object:\n%s", out)
	}
	if !strings.Contains(string(out), "\"Noto Sans Thai\"") {
		t.Fatalf("the entry did not survive as a bare string:\n%s", out)
	}
	// AND THE VERSION DID NOT MOVE. This is the half that makes the round
	// trip load-bearing rather than cosmetic.
	if got := versionForSave(d.Version, d); got != baseVersion {
		t.Fatalf("versionForSave stamped %q on a document whose only object-shaped input canonicalises to a bare string, want %q", got, baseVersion)
	}
	if !strings.Contains(string(out), `"version": "1.0"`) {
		t.Fatalf("the saved document does not declare 1.0:\n%s", out)
	}
	// A fixed point on the second pass, so the bare string is stable.
	again, err := ParseDocument(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	out2, err := SerializeDocument(again)
	if err != nil {
		t.Fatalf("reserialize: %v", err)
	}
	if string(out) != string(out2) {
		t.Fatalf("not a fixed point:\n%s\n---\n%s", out, out2)
	}
}

// TestAFullyExercisedDocumentIsNotVersionINFLATED is P9. maximalFixture
// used to be the standing witness that a document carrying every key the
// serializer can emit still requires only what its CONTENT requires;
// Story 11.2 moved it to 2.0 (correctly — its chain now carries an
// object entry), so that witness went with it.
//
// This replaces it as a TARGETED assertion rather than a second maximal
// fixture. It is stated over versionRequiredByContent and over a
// versionForSave whose LOADED version is the floor, because
// versionForSave never LOWERS (D-1.4.13): asking it about a document
// that already declares 2.0 can only ever answer 2.0, which would make
// the assertion vacuous whatever the content said.
func TestAFullyExercisedDocumentIsNotVersionINFLATED(t *testing.T) {
	d, err := ParseDocument(maximalFixture)
	if err != nil {
		t.Fatalf("parse maximalFixture: %v", err)
	}
	// Precondition: the fixture really is the maximal one — it is the
	// document TestDriftASTMatchesRuntimeEmission uses for exactly that,
	// and if it stopped exercising the whole model this test would be
	// stating the rule over a thin document.
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	for _, key := range []string{`"boldItalic"`, `"face"`, `"keepTogether"`, `"lineSpacing"`} {
		if !strings.Contains(string(out), key) {
			t.Fatalf("fixture precondition: maximalFixture no longer emits %s, so this rule is being stated over a document that does not exercise it", key)
		}
	}

	// SPEC-table-rules added `rules` and `minHeight` to the maximal
	// fixture (drift_test.go requires it to emit every key the serializer
	// can), and those two require 3.1 — which is ALSO the library's
	// ceiling, so the document as written can no longer witness "not
	// inflated to the ceiling" at all. They are cleared here rather than
	// kept out of the fixture, because this test is about what the
	// CHAIN's object entry requires and the two table keys carry their
	// own version assertions in version_test.go. The precondition above
	// still proves the fixture is the maximal one.
	// spec-section-break added the content band's `sectionBreak` to the
	// maximal fixture; it requires 4.1, the ceiling, so it is cleared here
	// for the same reason as the keys below.
	d.Bands.Content.SectionBreak = Presence[geom.Length]{}
	d.Bands.Content.SectionBreakAnchor = Presence[bool]{}
	for _, band := range []*Band{&d.Bands.PageHeader, &d.Bands.Content, &d.Bands.PageFooter} {
		// spec-barcode-qr-elements added a qrcode (and its `errorCorrection`)
		// to the maximal fixture; the element type requires 4.0, the ceiling,
		// so it is removed here for the same reason as the table keys below.
		kept := band.Elements[:0]
		for _, el := range band.Elements {
			if el.Type != ElementQRCode {
				kept = append(kept, el)
			}
		}
		band.Elements = kept
		for i := range band.Elements {
			if !band.Elements[i].Table.Set {
				continue
			}
			tbl := band.Elements[i].Table.Value
			tbl.Rules = Presence[TableRules]{}
			tbl.MinHeight = Presence[geom.Length]{}
			// `columns[].headerAlign` requires 3.2, the ceiling since that
			// key landed — cleared for the same reason as the two above.
			for c := range tbl.Columns {
				tbl.Columns[c].HeaderAlign = Presence[string]{}
			}
			band.Elements[i].Table = present(tbl)
		}
	}

	// The ONE thing in this document that requires 2.0 is its object-form
	// chain entry. Everything else is untouched, so what the requirement
	// does next is a statement about that key alone.
	if got := versionRequiredByContent(d); got != majorFeatureVersion {
		t.Fatalf("maximalFixture requires %q, want %q — its chain carries an object entry", got, majorFeatureVersion)
	}
	d.Fonts["body"] = []FontChainEntry{FaceEntry("Noto Sans"), FaceEntry("Noto Sans Thai"), FaceEntry("Noto Sans SC")}
	stripped := versionRequiredByContent(d)
	if stripped == majorFeatureVersion {
		t.Fatalf("a document whose chain is bare strings still REQUIRES %q — something other than the object form is raising it", stripped)
	}
	if stripped == SupportedVersion {
		t.Fatalf("a fully exercised document requires the library's own ceiling %q, which is the inflation versionForSave exists to prevent", SupportedVersion)
	}

	// And the save path agrees, given a document whose loaded version
	// does not already hold it up. versionForSave never lowers, so the
	// floor has to be the floor for this to observe anything.
	d.Version = baseVersion
	if got := versionForSave(d.Version, d); got != stripped {
		t.Fatalf("versionForSave stamped %q where the content requires %q", got, stripped)
	}
	if got := versionForSave(d.Version, d); got == SupportedVersion {
		t.Fatalf("versionForSave stamped the library's ceiling %q on a document that does not need it", SupportedVersion)
	}
	// Putting the object entry back raises it again, so the assertions
	// above are about that key and not about an inert code path.
	d.Fonts["body"][1] = FontChainEntry{Face: "Noto Sans Thai", Bold: "Noto Sans Thai Bold"}
	if got := versionForSave(d.Version, d); got != majorFeatureVersion {
		t.Fatalf("restoring the object-form entry left the version at %q, want %q — the assertions above were vacuous", got, majorFeatureVersion)
	}
}

// TestAVariantAssetKeyMustStateItsTerms is AD-26 / I-7: a variant asset
// key IS an embedded face and clears the licence bar exactly as the
// entry's own `asset` value does. A sibling that skipped it would let a
// document carry an unlicensed embedded bold — the whole point of the
// requirement, routed around by one key.
func TestAVariantAssetKeyMustStateItsTerms(t *testing.T) {
	// The positive control first: the SAME document with a fully licensed
	// second asset loads, so the refusal below is about the terms and not
	// about the shape.
	licensed := twoFontAssetDoc(fontAssetBody, secondFontAssetBody,
		`[{"asset": "`+embeddedFontKey+`", "bold": "`+secondFontKey+`"}]`)
	if _, err := ParseDocument([]byte(licensed)); err != nil {
		t.Fatalf("a licensed variant asset was refused: %v", err)
	}

	unlicensed := twoFontAssetDoc(fontAssetBody, unlicensedFontAssetBody,
		`[{"asset": "`+embeddedFontKey+`", "bold": "`+secondFontKey+`"}]`)
	le := requireLoadErrorIn(t, unlicensed)
	if !strings.HasPrefix(le.Field, "assets."+secondFontKey+".font.") {
		t.Fatalf("the refusal is located at %q, want the SECOND asset's own font record — that is where the fix goes", le.Field)
	}
	if !strings.Contains(le.Reason, "fonts.body[0].bold") {
		t.Fatalf("the refusal %q does not name the chain entry sibling that made the asset an embedded face", le.Reason)
	}
}

// secondFontKey is the SHA-256 of a second hand-built 156-byte sfnt (the
// same shape as embeddedFontKey's, differing only in its table
// contents), so a document can carry TWO embedded faces — which is what
// a chain entry with a variant asset key needs.
const secondFontKey = "35573263bb78c4a0b0866ff63489bcfeb36b56ac2abe42206967541ba829eea7"

const secondFontData = `[
        "AAEAAAADACAABAAQY21hcAAAAAAAAAA8AAAAIGdseWYAAAAAAAAAXAAAACBoZWFkAAAAAAAAAHwA",
        "AAAgRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUxRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUxRklYVFVS",
        "RTFGSVhUVVJFMUZJWFRVUkUxRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUx"
      ]`

// secondFontAssetBody is the second face WITH its terms, so the licence
// assertion below has a positive control that differs in exactly one
// thing.
const secondFontAssetBody = `
      "data": ` + secondFontData + `,
      "font": {
        "copyright": "Copyright 2026 The Folio Fixture Authors",
        "family": "Second Sans",
        "licence": "SIL Open Font License 1.1",
        "licenceText": "This fixture face is licensed under the SIL Open Font License, Version 1.1.",
        "source": "hand-built 156-byte sfnt — a fixture, not a face",
        "style": "Bold"
      },
      "mediaType": "font/ttf"`

// unlicensedFontAssetBody is the second face with its terms MISSING —
// a `font` record that names a family and states no licence at all.
const unlicensedFontAssetBody = `
      "data": ` + secondFontData + `,
      "font": {
        "family": "Unlicensed Sans"
      },
      "mediaType": "font/ttf"`

// twoFontAssetDoc is embeddedFontDoc with a SECOND font asset, so a
// chain entry can name one asset as its face and another as a variant.
// The two bodies are the inner text of each asset object; secondFontKey
// sorts before embeddedFontKey, and the assets map is emitted sorted, so
// the second asset is written first.
func twoFontAssetDoc(firstBody, secondBody, chainBody string) string {
	return `{
  "assets": {
    "` + secondFontKey + `": {` + secondBody + `
    },
    "` + embeddedFontKey + `": {` + firstBody + `
    }
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ` + chainBody + `},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}`
}

// requireLoadErrorIn is requireLoadError without the field expectation,
// for the cases whose whole point is WHICH field is named.
func requireLoadErrorIn(t *testing.T, doc string) *LoadError {
	t.Helper()
	_, err := ParseDocument([]byte(doc))
	if err == nil {
		t.Fatal("expected a load error, got none")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("expected a *LoadError, got %T: %v", err, err)
	}
	return le
}
