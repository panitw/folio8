package folio8

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// THE COMMAND DOORS UNDER spec-font-sources-and-embedding CAP-6.
//
// One test per row of the story's I/O matrix that a COMMAND can reach; the
// load-path and version rows are asserted in
// internal/template/font_acknowledgement_test.go, at the doors that own them.
//
// The subject is always a face this repository already commits, with its own
// `name` table rewritten where the row needs a particular statement in it —
// never a mislabelled binary added to the tree.

// acknowledgedEmbedCommand is embedCommand with the acknowledgement and the
// three terms fields under the caller's control, which is the whole of what
// this story changed about the wire shape. `family`, `style` and `source` are
// fixed and non-blank here, because D4 leaves them required whatever was
// acknowledged — a test that varied them would be measuring the identity gate
// rather than the terms one.
func acknowledgedEmbedCommand(t *testing.T, chain string, face []byte, acknowledged bool, licence, licenceText, copyright string) string {
	t.Helper()
	return `{"kind":"embedFontFamily","version":1,"name":` + quoteForCommand(t, chain) +
		`,"family":"Brand Grotesk","style":"Regular"` +
		`,"licence":` + quoteForCommand(t, licence) +
		`,"licenceText":` + quoteForCommand(t, licenceText) +
		`,"copyright":` + quoteForCommand(t, copyright) +
		`,"source":"imported from the author's own machine, acknowledged 2026-09-21"` +
		`,"authorAcknowledged":` + strconv.FormatBool(acknowledged) +
		`,"mediaType":"font/ttf","data":"` + base64.StdEncoding.EncodeToString(face) + `","tail":["Noto Sans"]}`
}

// acknowledgedCutCommand is the same for embedFontCut.
func acknowledgedCutCommand(t *testing.T, chain string, index int, cut string, face []byte, acknowledged bool, licence, licenceText, copyright string) string {
	t.Helper()
	return `{"kind":"embedFontCut","version":1,"name":` + quoteForCommand(t, chain) +
		`,"index":` + strconv.Itoa(index) + `,"cut":` + quoteForCommand(t, cut) +
		`,"family":"Brand Grotesk","style":"Bold"` +
		`,"licence":` + quoteForCommand(t, licence) +
		`,"licenceText":` + quoteForCommand(t, licenceText) +
		`,"copyright":` + quoteForCommand(t, copyright) +
		`,"source":"imported from the author's own machine, acknowledged 2026-09-21"` +
		`,"authorAcknowledged":` + strconv.FormatBool(acknowledged) +
		`,"mediaType":"font/ttf","data":"` + base64.StdEncoding.EncodeToString(face) + `"}`
}

// ---------------------------------------------------------------------------
// A COPYLEFT BINARY, BUILT RATHER THAN COMMITTED.
//
// The matrix's second row needs a face whose OWN `name` table names GPL, and
// no such binary belongs in this repository — committing one to test a refusal
// would put a copyleft font program in the tree for every consumer to carry.
// So the committed Roboto's `name` table is rewritten in memory, which is the
// same technique internal/fontset/licencesignature_test.go uses for the same
// reason; the helpers live here because that file's are in a test package this
// one cannot import.
//
// ⚠ THE ORIGINAL RECORDS ARE KEPT AND ONE IS ADDED, never replaced. A
// one-record table would leave a binary that names nothing but a GPL
// statement — no family (ID 1), no subfamily (ID 2), no copyright (ID 0), no
// licence URL (ID 14) — and "an acknowledged copyleft face embeds" proven over
// THAT is proven over a face no author could have picked. The evidence has to
// be a real face that also says GPL.
//
// Checksums are not recomputed, on the fontset file's reasoning: ot.ParseFont
// validates none of them, and a helper that quietly repaired the file would be
// asserting over bytes no author could produce by hand.
// ---------------------------------------------------------------------------

const gplStatement = "This font is licensed under the GNU General Public License version 3"

func faceDeclaringGPL(t *testing.T) []byte {
	t.Helper()
	return replaceNameTableForTest(t, testRobotoFontBytes,
		nameTableWithAddedRecord(t, nameTableOf(t, testRobotoFontBytes), 13, gplStatement))
}

// nameTableOf returns the `name` table bytes an sfnt already carries.
func nameTableOf(t *testing.T, src []byte) []byte {
	t.Helper()
	numTables := int(binary.BigEndian.Uint16(src[4:6]))
	for i := 0; i < numTables; i++ {
		record := src[12+i*16 : 12+i*16+16]
		if string(record[0:4]) != "name" {
			continue
		}
		offset := binary.BigEndian.Uint32(record[8:12])
		length := binary.BigEndian.Uint32(record[12:16])
		return src[offset : offset+length]
	}
	t.Fatal("the subject face carries no `name` table, so there is nothing to extend")
	return nil
}

// nameTableWithAddedRecord returns table with ONE more Windows/en-US record in
// it and every existing record untouched.
//
// The surgery is cheap because the format allows it: each record's offset is
// relative to the START of the string storage pool, so appending a string
// leaves every existing offset valid, and only the record count and the
// storage offset in the 6-byte header move.
func nameTableWithAddedRecord(t *testing.T, table []byte, id uint16, value string) []byte {
	t.Helper()
	if format := binary.BigEndian.Uint16(table[0:2]); format != 0 {
		t.Fatalf("the subject face's `name` table is format %d; this helper only extends format 0", format)
	}
	count := int(binary.BigEndian.Uint16(table[2:4]))
	storageOffset := int(binary.BigEndian.Uint16(table[4:6]))
	directory := table[6 : 6+12*count]
	storage := table[storageOffset:]

	units := utf16.Encode([]rune(value))
	encoded := make([]byte, 0, len(units)*2)
	for _, unit := range units {
		encoded = append(encoded, byte(unit>>8), byte(unit))
	}
	if len(encoded) > 0xFFFF || len(storage)+len(encoded) > 0xFFFF {
		t.Fatalf("adding %d bytes to a %d-byte storage pool overflows the `name` table's 16-bit fields, and every assertion made over the result would be made over a corrupt table", len(encoded), len(storage))
	}

	entry := make([]byte, 12)
	binary.BigEndian.PutUint16(entry[0:], 3)      // platformID: Windows
	binary.BigEndian.PutUint16(entry[2:], 1)      // encodingID: Unicode BMP
	binary.BigEndian.PutUint16(entry[4:], 0x0409) // languageID: en-US
	binary.BigEndian.PutUint16(entry[6:], id)
	binary.BigEndian.PutUint16(entry[8:], uint16(len(encoded)))
	binary.BigEndian.PutUint16(entry[10:], uint16(len(storage)))

	header := make([]byte, 6)
	binary.BigEndian.PutUint16(header[0:], 0)
	binary.BigEndian.PutUint16(header[2:], uint16(count+1))
	binary.BigEndian.PutUint16(header[4:], uint16(6+12*(count+1)))

	out := append([]byte{}, header...)
	out = append(out, directory...)
	out = append(out, entry...)
	out = append(out, storage...)
	return append(out, encoded...)
}

func replaceNameTableForTest(t *testing.T, src, table []byte) []byte {
	t.Helper()
	out := make([]byte, len(src))
	copy(out, src)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	offset := uint32(len(out))
	out = append(out, table...)
	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	for i := 0; i < numTables; i++ {
		record := out[12+i*16 : 12+i*16+16]
		if string(record[0:4]) != "name" {
			continue
		}
		binary.BigEndian.PutUint32(record[8:12], offset)
		binary.BigEndian.PutUint32(record[12:16], uint32(len(table)))
		return out
	}
	t.Fatal("the subject face carries no `name` table record to repoint")
	return nil
}

// TestTheCopyleftFixtureIsARealFaceThatSaysGPL is the precondition every row
// below rests on, and it asserts the PROPERTY rather than a substring of a
// message this story promises not to change.
//
// Two halves, and both are load-bearing. The face must READ as copyleft
// through the same function the guard reads it with — a rewrite that stopped
// landing in the `name` table would leave the guard admitting NO EVIDENCE, and
// "an acknowledged copyleft face embeds" would pass having tested nothing. And
// it must still BE a face: its own identity records have to survive, or the
// admitted binary is not one an author could ever have picked.
func TestTheCopyleftFixtureIsARealFaceThatSaysGPL(t *testing.T) {
	face := faceDeclaringGPL(t)

	statement, present := fontset.ReadLicenceStatement(face)
	if !present {
		t.Fatal("the rewritten face makes no readable licence statement, so the guard would admit it as NO EVIDENCE and nothing below tests what it claims to")
	}
	if statement != gplStatement {
		t.Fatalf("the face's licence statement reads %q, want the GPL sentence this fixture exists to carry", statement)
	}
	// AND IT IS STILL ROBOTO. Read through the renderer's own ingestion, which
	// is the strongest available statement that the surgery left a face
	// behind: fontset.New parses the binary and would refuse a broken one.
	if _, err := fontset.New("Brand", face); err != nil {
		t.Fatalf("the rewritten face is no longer ingestible, so it is not a face an author could have picked: %v", err)
	}
	original, hadOriginal := fontset.ReadLicenceStatement(testRobotoFontBytes)
	if !hadOriginal || original == gplStatement {
		t.Fatal("precondition: the committed Roboto must itself make a NON-GPL statement, or this fixture proves nothing about the rewrite")
	}
}

// MATRIX ROW 1 — ACKNOWLEDGED, BLANK TERMS. The face an author-supplied
// binary usually is: its `name` table says nothing, so name IDs 0/13/14
// transcribe to three empty strings, and before this story the command door
// refused it outright.
func TestAnAcknowledgedFaceEmbedsWithBlankTerms(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	fontChainAccepted(t, tpl, acknowledgedEmbedCommand(t, "Brand", face, true, "", "", ""))

	asset, ok := tpl.doc.Assets[embeddedKeyOf(face)]
	if !ok {
		t.Fatal("the acknowledged face was not stored")
	}
	if !asset.FaceAcknowledged() {
		t.Fatal("the document did not record the acknowledgement, so it cannot travel with the file")
	}
	// IDENTITY IS STILL RECORDED, which is what says the relaxation is scoped
	// to terms rather than to the record as a whole.
	if asset.Font.Value.Family.Value != "Brand Grotesk" || asset.Font.Value.Source.Value == "" {
		t.Errorf("identity did not survive the acknowledged pick: %#v", asset.Font.Value)
	}
	// AND THE TERMS ARE RECORDED AS WHAT THEY ARE: present and empty, not
	// invented. Nothing in this path may compose a licence nobody read.
	for _, empty := range []template.Presence[string]{asset.Font.Value.Licence, asset.Font.Value.LicenceText, asset.Font.Value.Copyright} {
		if !empty.Set || empty.Null || empty.Value != "" {
			t.Errorf("a terms field was not carried through as the empty string the binary stated: %#v", empty)
		}
	}

	// AND THE DOCUMENT REOPENS. This is the acceptance criterion's second
	// half, and it is the one a guard honoured at only one door fails: a
	// document that saves and will not reload.
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := ParseTemplate(saved)
	if err != nil {
		t.Fatalf("a document carrying an acknowledged face saved and would not reopen: %v", err)
	}
	if !reopened.doc.Assets[embeddedKeyOf(face)].FaceAcknowledged() {
		t.Fatal("the acknowledgement did not survive the save/open round trip")
	}
	if !strings.Contains(string(saved), `"version": "5.0"`) {
		t.Fatalf("the saved document does not declare 5.0:\n%s", saved)
	}
}

// MATRIX ROW 2 — ACKNOWLEDGED, COPYLEFT BINARY. The guard is not asked at all,
// so a face whose own `name` table names GPL embeds on the author's word.
func TestAnAcknowledgedFaceEmbedsThoughItsBinaryNamesCopyleft(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := faceDeclaringGPL(t)
	fontChainAccepted(t, tpl, acknowledgedEmbedCommand(t, "Brand", face, true, "", "", ""))
	if _, ok := tpl.doc.Assets[embeddedKeyOf(face)]; !ok {
		t.Fatal("an acknowledged copyleft face was refused at the embed door — the acknowledgement is not honoured there")
	}
}

// MATRIX ROW 3 — UNACKNOWLEDGED, BLANK TERMS. Refused, and with today's
// message: the acknowledgement adds a condition on whether the question is
// asked, never a new answer.
func TestAnUnacknowledgedFaceIsStillRefusedForBlankTerms(t *testing.T) {
	for _, field := range []string{"licence", "licenceText", "copyright"} {
		t.Run(field, func(t *testing.T) {
			licence, licenceText, copyright := "OFL-1.1", "terms", "c"
			switch field {
			case "licence":
				licence = ""
			case "licenceText":
				licenceText = ""
			default:
				copyright = ""
			}
			tpl := fontChainTemplate(t)
			failure := fontChainRefusal(t, tpl, acknowledgedEmbedCommand(t, "Brand", testShippedNotoSansThai, false, licence, licenceText, copyright))
			if !strings.Contains(failure.Message, field+" must be a non-empty string") {
				t.Fatalf("the refusal's wording moved: %s", failure.Message)
			}
		})
	}
}

// AND BLANK IS STILL EMPTY for an unacknowledged record: a space satisfies a
// length check and states exactly as much as `""`.
func TestAnUnacknowledgedFaceIsStillRefusedForWhitespaceTerms(t *testing.T) {
	tpl := fontChainTemplate(t)
	failure := fontChainRefusal(t, tpl, acknowledgedEmbedCommand(t, "Brand", testShippedNotoSansThai, false, "OFL-1.1", "   ", "c"))
	if !strings.Contains(failure.Message, "licenceText must be a non-empty string") {
		t.Fatalf("whitespace-only terms were admitted, or the wording moved: %s", failure.Message)
	}
}

// MATRIX ROW 4 — UNACKNOWLEDGED, COPYLEFT. The guard is asked, and it answers
// exactly as it did before this story; the message is the fontset package's
// own and names the face and the licence it read.
func TestAnUnacknowledgedCopyleftFaceIsStillRefused(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := faceDeclaringGPL(t)
	failure := fontChainRefusal(t, tpl, acknowledgedEmbedCommand(t, "Brand", face, false, "OFL-1.1", "terms", "c"))
	for _, want := range []string{"Brand", "copyleft or share-alike"} {
		if !strings.Contains(failure.Message, want) {
			t.Errorf("the refusal no longer names %q: %s", want, failure.Message)
		}
	}
	if _, exists := tpl.doc.Assets[embeddedKeyOf(face)]; exists {
		t.Error("the refused face was written to the assets map anyway")
	}
}

// MATRIX ROW 5 — THE VARIANT CUT. AD-26 / I-7 makes a cut an embedded face in
// its own right, so it clears the same bar through the same functions — and
// must be excused by the same acknowledgement. A guard honoured at the base
// and not at the cut is an author who can embed a family and not bold it.
func TestAnAcknowledgedCutEmbedsWithBlankTermsAndACopyleftBinary(t *testing.T) {
	tpl := fontChainTemplate(t)
	base := testShippedNotoSansThai
	fontChainAccepted(t, tpl, acknowledgedEmbedCommand(t, "Brand", base, true, "", "", ""))

	bold := faceDeclaringGPL(t)
	fontChainAccepted(t, tpl, acknowledgedCutCommand(t, "Brand", 0, "bold", bold, true, "", "", ""))

	chain := tpl.doc.Fonts["Brand"]
	if chain[0].Bold != embeddedKeyOf(bold) {
		t.Fatalf("the acknowledged cut was not declared: %#v", chain[0])
	}
	if !tpl.doc.Assets[chain[0].Bold].FaceAcknowledged() {
		t.Fatal("the cut's own record carries no acknowledgement, so the load door would refuse the document this command just wrote")
	}
}

// AND THE CUT DOOR STILL REFUSES AN UNACKNOWLEDGED ONE, both ways.
func TestAnUnacknowledgedCutIsStillRefusedAtBothGates(t *testing.T) {
	for _, c := range []struct {
		label                           string
		face                            []byte
		licence, licenceText, copyright string
		want                            string
	}{
		{"blank terms", testShippedNotoSans, "OFL-1.1", "", "c", "licenceText must be a non-empty string"},
		{"a copyleft binary", nil, "OFL-1.1", "terms", "c", "copyleft or share-alike"},
	} {
		t.Run(c.label, func(t *testing.T) {
			face := c.face
			if face == nil {
				face = faceDeclaringGPL(t)
			}
			tpl := fontChainTemplate(t)
			fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", testShippedNotoSansThai, `["Noto Sans SC"]`))
			failure := fontChainRefusal(t, tpl, acknowledgedCutCommand(t, "Noto Sans Thai", 0, "bold", face, false, c.licence, c.licenceText, c.copyright))
			if !strings.Contains(failure.Message, c.want) {
				t.Fatalf("the cut door's refusal moved: %s", failure.Message)
			}
		})
	}
}

// MATRIX ROW 6 — IDENTITY IS STILL REQUIRED. The acknowledgement is a
// statement about TERMS. A record that cannot name its own family is not an
// acknowledged face, it is a broken one, and D4 says so in as many words.
func TestAnAcknowledgedFaceStillNeedsItsIdentity(t *testing.T) {
	full := acknowledgedEmbedCommand(t, "Brand", testShippedNotoSansThai, true, "", "", "")
	for _, blanked := range []struct{ field, from string }{
		{"family", `"family":"Brand Grotesk"`},
		{"style", `"style":"Regular"`},
		{"source", `"source":"imported from the author's own machine, acknowledged 2026-09-21"`},
	} {
		t.Run(blanked.field, func(t *testing.T) {
			tpl := fontChainTemplate(t)
			command := strings.Replace(full, blanked.from, `"`+blanked.field+`":""`, 1)
			failure := fontChainRefusal(t, tpl, command)
			if !strings.Contains(failure.Message, blanked.field+" must be a non-empty string") {
				t.Fatalf("an acknowledged record with a blank %s was admitted, or the wording moved: %s", blanked.field, failure.Message)
			}
		})
	}
}

// THE KEY IS STILL REQUIRED AND STILL A BOOLEAN. What the acknowledgement
// excuses is an EMPTY terms value, never a missing or mistyped field — which
// is what keeps the wire shape one thing rather than two.
func TestTheAcknowledgementIsARequiredBooleanOnTheWire(t *testing.T) {
	full := acknowledgedEmbedCommand(t, "Brand", testShippedNotoSansThai, true, "", "", "")
	tpl := fontChainTemplate(t)

	missing := strings.Replace(full, `"authorAcknowledged":true,`, ``, 1)
	if _, err := applyComponentCommand(tpl, []byte(missing)); err == nil || !strings.Contains(err.Error(), "unknown or missing fields") {
		t.Errorf("omitting the key was not an arity refusal: %v", err)
	}

	mistyped := strings.Replace(full, `"authorAcknowledged":true`, `"authorAcknowledged":"true"`, 1)
	failure := fontChainRefusal(t, tpl, mistyped)
	if !strings.Contains(failure.Message, "authorAcknowledged must be a boolean") {
		t.Fatalf("a quoted acknowledgement was admitted, or the wording moved: %s", failure.Message)
	}

	// `false` IS NOT RECORDED. A catalogue pick sends it on every command, and
	// a document that wrote the key anyway would carry a key that means
	// nothing and would then have to answer for its version.
	declined := strings.Replace(full, `"authorAcknowledged":true`, `"authorAcknowledged":false`, 1)
	declined = strings.Replace(declined, `"licence":"","licenceText":"","copyright":""`, `"licence":"OFL-1.1","licenceText":"terms","copyright":"c"`, 1)
	fresh := fontChainTemplate(t)
	fontChainAccepted(t, fresh, declined)
	asset := fresh.doc.Assets[embeddedKeyOf(testShippedNotoSansThai)]
	if asset.Font.Value.AuthorAcknowledged.Set {
		t.Fatal("a declined acknowledgement was written into the document — the key is recorded only when it is true")
	}
}

// MATRIX ROW 10 — THE CATALOGUE TIER NEVER WRITES ONE, asserted over the
// designer source that builds those picks rather than over a mock, because the
// claim is about what the catalogue path SENDS.
//
// It is the one thing that would quietly collapse the distinction between a
// face Folio distributes and a face the author supplied: a catalogue face
// carrying an acknowledgement would take the licence gate off the tier whose
// whole warrant is that it has one.
func TestACatalogueFaceNeverCarriesAnAcknowledgement(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	// embedCommand IS the catalogue shape — the helper every catalogue test in
	// this package sends — so this asserts over the path, not over a literal
	// written here.
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))
	asset := tpl.doc.Assets[embeddedKeyOf(face)]
	if asset.FaceAcknowledged() || asset.Font.Value.AuthorAcknowledged.Set {
		t.Fatal("a catalogue pick wrote an acknowledgement — a catalogue face states real terms and passes its own licence gate, and an acknowledgement on one is a defect")
	}
	// AND A CATALOGUE CUT LIKEWISE.
	bold := testShippedNotoSans
	fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold))
	if tpl.doc.Assets[embeddedKeyOf(bold)].FaceAcknowledged() {
		t.Fatal("a catalogue cut wrote an acknowledgement")
	}
}

// THE ACKNOWLEDGEMENT EXCUSES THE THREE TERMS FIELDS AND NOTHING ELSE — the
// half D4 states as a limit, asserted as refusals rather than trusted to the
// reading of one `if`.
//
// Each row below is a gate that sits BESIDE the licence question at the same
// door, over the same bytes, and each answers a question the author's
// acknowledgement cannot speak to: a variable face is one PDF 1.7 cannot
// express, a binary that is not what its media type says is a file lying about
// itself, and a face over the payload bound is one the protocol cannot carry.
// None is a statement about terms, so none may move inside the acknowledged
// branch — and a maintainer who moved one there would break exactly the
// promise D-8.4d.1 makes, that a document which saves is a document that
// renders.
func TestAnAcknowledgementExcusesTermsAndNoOtherGate(t *testing.T) {
	oversize := make([]byte, maxComponentAssetBytes+1)
	copy(oversize, testRobotoFontBytes)

	for _, c := range []struct {
		label   string
		command func(*testing.T) string
		want    string
	}{
		{
			// fontset.RefuseVariableFace, whose subject is the FACE and not
			// its licence: an acknowledgement says nothing about `fvar`.
			"a variable face",
			func(t *testing.T) string {
				return acknowledgedEmbedCommand(t, "Brand", testNotoSansThaiVariableFontBytes, true, "", "", "")
			},
			"fvar",
		},
		{
			// DecodeFontForRender / checkSfnt: the bytes are not the container
			// the command declares. Reader-independent, and nobody's terms.
			"bytes that contradict the declared mediaType",
			func(t *testing.T) string {
				return acknowledgedEmbedCommand(t, "Brand", []byte("\x89PNG\r\n\x1a\nnot a font at all"), true, "", "", "")
			},
			"sfnt",
		},
		{
			// embeddedFaceBytes' payload bound, which is the host's memory and
			// not the author's licence.
			"a face over the size bound",
			func(t *testing.T) string {
				return acknowledgedEmbedCommand(t, "Brand", oversize, true, "", "", "")
			},
			fmt.Sprintf("face exceeds the %d-byte supported size", maxComponentAssetBytes),
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			tpl := fontChainTemplate(t)
			failure := fontChainRefusal(t, tpl, c.command(t))
			if !strings.Contains(strings.ToLower(failure.Message), strings.ToLower(c.want)) {
				t.Fatalf("an acknowledged record was excused a gate that is not about terms, or the refusal moved: %s", failure.Message)
			}
		})
	}
}

// AND THE SAME AT THE CUT DOOR, which clears the identical bar through the
// identical functions.
func TestAnAcknowledgedCutIsStillRefusedAVariableFace(t *testing.T) {
	tpl := fontChainTemplate(t)
	fontChainAccepted(t, tpl, acknowledgedEmbedCommand(t, "Brand", testShippedNotoSansThai, true, "", "", ""))
	failure := fontChainRefusal(t, tpl, acknowledgedCutCommand(t, "Brand", 0, "bold", testNotoSansThaiVariableFontBytes, true, "", "", ""))
	if !strings.Contains(failure.Message, "fvar") {
		t.Fatalf("an acknowledged variable CUT was admitted, or the refusal moved: %s", failure.Message)
	}
}

// THE RECORD IS AS FROZEN AS THE BYTES. Two picks of byte-identical faces that
// disagree about the acknowledgement are refused rather than reconciled: an
// assets key IS the content, both doors write "only if absent", and whichever
// record happened to arrive second would otherwise be silently discarded —
// putting an acknowledgement nobody made on a catalogue face, or losing the
// author's own and with it the 5.0 the document needs to reopen.
func TestBytesAlreadyHeldUnderADifferentAcknowledgementAreRefused(t *testing.T) {
	face := testShippedNotoSansThai

	t.Run("a catalogue pick may not inherit an acknowledgement", func(t *testing.T) {
		tpl := fontChainTemplate(t)
		fontChainAccepted(t, tpl, acknowledgedEmbedCommand(t, "Brand", face, true, "", "", ""))
		failure := fontChainRefusal(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))
		if !strings.Contains(failure.Message, "the author acknowledged") {
			t.Fatalf("a catalogue pick over acknowledged bytes was not refused as such: %s", failure.Message)
		}
	})

	t.Run("an acknowledgement may not be dropped", func(t *testing.T) {
		tpl := fontChainTemplate(t)
		fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))
		failure := fontChainRefusal(t, tpl, acknowledgedEmbedCommand(t, "Brand", face, true, "", "", ""))
		if !strings.Contains(failure.Message, "without an acknowledgement") {
			t.Fatalf("an acknowledged pick over unacknowledged bytes was not refused as such: %s", failure.Message)
		}
	})

	t.Run("at the cut door too", func(t *testing.T) {
		tpl := fontChainTemplate(t)
		bold := testShippedNotoSans
		fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))
		fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold))
		// A SECOND chain, whose acknowledged cut is the same bytes the first
		// chain already holds as a catalogue cut.
		fontChainAccepted(t, tpl, acknowledgedEmbedCommand(t, "Brand", testRobotoFontBytes, true, "", "", ""))
		failure := fontChainRefusal(t, tpl, acknowledgedCutCommand(t, "Brand", 0, "bold", bold, true, "", "", ""))
		if !strings.Contains(failure.Message, "without an acknowledgement") {
			t.Fatalf("the cut door reconciled the records instead of refusing: %s", failure.Message)
		}
	})

	// AND AGREEMENT IS STILL A NO-OP, which is what says the refusal is about
	// the disagreement and not about the second pick.
	t.Run("agreeing picks are unaffected", func(t *testing.T) {
		tpl := fontChainTemplate(t)
		fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))
		fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))
	})
}

// NULL IS NOT FALSE, AND NULL IS NOT THE EMPTY STRING. Both readers refuse it
// explicitly, because unmarshalling `null` into a Go string or bool is a no-op
// that returns no error — so without those branches the command door would
// record values the author never wrote, into a record the loader keeps
// three-valued.
func TestTheCommandDoorRefusesANullAcknowledgementAndNullTerms(t *testing.T) {
	full := acknowledgedEmbedCommand(t, "Brand", testShippedNotoSansThai, true, "", "", "")
	for _, c := range []struct{ label, from, to, want string }{
		{"a null acknowledgement", `"authorAcknowledged":true`, `"authorAcknowledged":null`, "authorAcknowledged must be a boolean"},
		{"a null licence on an acknowledged record", `"licence":""`, `"licence":null`, "licence must be a string"},
	} {
		t.Run(c.label, func(t *testing.T) {
			tpl := fontChainTemplate(t)
			failure := fontChainRefusal(t, tpl, strings.Replace(full, c.from, c.to, 1))
			if !strings.Contains(failure.Message, c.want) {
				t.Fatalf("null was admitted, or the wording moved: %s", failure.Message)
			}
		})
	}
}
