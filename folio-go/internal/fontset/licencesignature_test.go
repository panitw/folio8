package fontset

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

// ---------------------------------------------------------------------------
// STORY 16.1b: THE BINARY IS ASKED WHAT IT SAYS ABOUT ITSELF.
//
// THE TWO CONTROLS ARE COMMITTED BYTES AND SYNTHESISED BYTES, and neither is
// a new font file in the tree (D-16.R.9).
//
//   - CONTRADICTION, over real bytes: testdata/fonts/Roboto-Regular.ttf is
//     recorded `Apache-2.0` (lint/internal/licence/licencecensus_test.go) and
//     its own record 13 reads "Licensed under the Apache License, Version
//     2.0". Declaring it `OFL-1.1` is therefore a TRUE contradiction — the
//     bytes match a different ADMITTED licence's signature than the one
//     claimed — with no mislabelled binary committed anywhere.
//
//   - REFUSE-SIGNATURE, over bytes built in this process: no committed face
//     carries a copyleft statement and none should, so that half of the guard
//     gets a name table SYNTHESISED here. Committing a font whose sole
//     purpose is to lie about its own licence would trip the manifest guard,
//     the binary-identity walk and the licence census — three gates that
//     exist to keep exactly that file out — in a product whose whole subject
//     is licence honesty.
//
// The synthesis rebuilds the whole `name` table and REPOINTS the sfnt table
// directory record at it, rather than patchUnitsPerEm's same-length overwrite
// in place. That is a deliberate widening of that helper's method and not a
// departure from it: `name` records are variable-length behind a string
// storage pool, so a same-length substitution can only ever say something the
// same size as what is already there, and the arms below need records that
// are absent, present-but-unrecognised, and present in two different
// combinations of 13 and 0. The directory walk is patchUnitsPerEm's, verbatim
// in shape.

// nameRecord is one (nameID, string) pair for buildNameTable.
type nameRecord struct {
	id    uint16
	value string
}

// buildNameTable assembles a format-0 `name` table carrying exactly the
// given records, each as platform 3 (Windows) / encoding 1 (UCS-2) /
// language 0x0409 — the combination ot.ParseName decodes as UTF-16BE, and
// the one every real face in this repository uses for these records.
//
// EVERY LENGTH AND OFFSET IN THIS TABLE IS A uint16, AND THE HELPER FATALS
// RATHER THAN WRAPPING. A `name` record's length and its offset into the
// string storage pool are both 16-bit fields, so a record over 65535 bytes —
// or a cumulative storage pool over 65535 bytes — would silently wrap into a
// table that parses to something other than what the caller asked for, and
// every assertion built on top of it would then be passing over garbage. The
// multi-kilobyte statements the truncation test builds put a caller within
// reach of this, so it is a t.Fatal and not a comment.
func buildNameTable(t *testing.T, records []nameRecord) []byte {
	t.Helper()
	var directory, storage []byte
	for _, record := range records {
		encoded := encodeUTF16BEForTest(record.value)
		if len(encoded) > 0xFFFF {
			t.Fatalf("buildNameTable: record %d encodes to %d bytes, which does not fit the `name` table's 16-bit length field — it would wrap and the assertions made over this face would be made over a corrupt table", record.id, len(encoded))
		}
		if len(storage)+len(encoded) > 0xFFFF {
			t.Fatalf("buildNameTable: the string storage pool would reach %d bytes at record %d, which does not fit the `name` table's 16-bit offset field — the next record's offset would wrap and point into the middle of an earlier string", len(storage)+len(encoded), record.id)
		}
		entry := make([]byte, 12)
		binary.BigEndian.PutUint16(entry[0:], 3)      // platformID: Windows
		binary.BigEndian.PutUint16(entry[2:], 1)      // encodingID: Unicode BMP
		binary.BigEndian.PutUint16(entry[4:], 0x0409) // languageID: en-US
		binary.BigEndian.PutUint16(entry[6:], record.id)
		binary.BigEndian.PutUint16(entry[8:], uint16(len(encoded)))
		binary.BigEndian.PutUint16(entry[10:], uint16(len(storage)))
		directory = append(directory, entry...)
		storage = append(storage, encoded...)
	}
	if 6+12*len(records) > 0xFFFF {
		t.Fatalf("buildNameTable: %d records put the string storage offset past the header's 16-bit field", len(records))
	}
	header := make([]byte, 6)
	binary.BigEndian.PutUint16(header[0:], 0) // format 0
	binary.BigEndian.PutUint16(header[2:], uint16(len(records)))
	binary.BigEndian.PutUint16(header[4:], uint16(6+12*len(records))) // storageOffset
	out := append(header, directory...)
	return append(out, storage...)
}

func encodeUTF16BEForTest(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(units)*2)
	for _, unit := range units {
		out = append(out, byte(unit>>8), byte(unit))
	}
	return out
}

// replaceNameTable returns a COPY of a well-formed sfnt whose `name` table
// record points at table instead. The new table is appended at the end of
// the file on a 4-byte boundary; the old bytes are left where they are and
// simply stop being referenced, which is what keeps every other table's
// offset valid.
//
// Checksums are not recomputed, deliberately: ot.ParseFont validates none of
// them (ot/font.go), and a test helper that quietly repaired the file would
// be asserting over bytes no author could produce by hand.
func replaceNameTable(t *testing.T, src, table []byte) []byte {
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
	t.Fatal("the subject face carries no `name` table record to repoint, so nothing this helper builds could be read back")
	return nil
}

// removeNameTable drops the `name` table's directory record entirely, so the
// face HAS no name table rather than an empty one. Same directory walk.
func removeNameTable(t *testing.T, src []byte) []byte {
	t.Helper()
	out := make([]byte, len(src))
	copy(out, src)
	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	for i := 0; i < numTables; i++ {
		offset := 12 + i*16
		if string(out[offset:offset+4]) != "name" {
			continue
		}
		copy(out[offset:12+(numTables-1)*16], out[offset+16:12+numTables*16])
		for j := 12 + (numTables-1)*16; j < 12+numTables*16; j++ {
			out[j] = 0
		}
		binary.BigEndian.PutUint16(out[4:6], uint16(numTables-1))
		return out
	}
	t.Fatal("the subject face carries no `name` table record to remove, so no assertion about its absence can be made")
	return nil
}

// faceWithNames is the synthesis in one call: the committed Roboto with its
// name table replaced by exactly the records given.
func faceWithNames(t *testing.T, records ...nameRecord) []byte {
	t.Helper()
	return replaceNameTable(t, testFontBytes(t), buildNameTable(t, records))
}

const (
	silSentence    = "This Font Software is licensed under the SIL Open Font License, Version 1.1."
	apacheSentence = "Licensed under the Apache License, Version 2.0"
	ubuntuSentence = "Licensed under the Ubuntu Font Licence 1.0."
)

// TestLicenceGuardPreconditions asserts the two committed controls really do
// say what this file's other tests assume, BEFORE anything asserts an outcome
// over them. A fixture that stopped carrying its licence sentence would
// otherwise turn every CONTRADICTION assertion below into a NO EVIDENCE one
// and take the whole file green for the wrong reason.
func TestLicenceGuardPreconditions(t *testing.T) {
	roboto, present := ReadLicenceStatement(testFontBytes(t))
	if !present {
		t.Fatal("precondition: testdata/fonts/Roboto-Regular.ttf makes no readable licence statement, so it cannot serve as the contradiction control")
	}
	if !strings.Contains(roboto, "Apache License") {
		t.Fatalf("precondition: Roboto-Regular.ttf's licence statement is %q and no longer names the Apache License — the contradiction control has stopped being a contradiction", roboto)
	}

	noto, present := ReadLicenceStatement(variableNotoBytes(t))
	if !present {
		t.Fatal("precondition: NotoSansThai-VF.ttf makes no readable licence statement, so it cannot serve as the confirmation control")
	}
	if !strings.Contains(noto, "SIL Open Font License") {
		t.Fatalf("precondition: NotoSansThai-VF.ttf's licence statement is %q and no longer names the SIL OFL", noto)
	}

	// AND THE THREE SYNTHESISED SENTENCES, pinned to the rows they stand for.
	// The committed fixtures above are pinned because a fixture that stopped
	// carrying its sentence would turn a CONTRADICTION assertion into a NO
	// EVIDENCE one; the constants below have exactly the same failure mode and
	// were not pinned. ubuntuSentence is the reachable case: it is the only
	// statement the record-0 tests have, so if it and the Ubuntu row's pattern
	// ever drift apart — a spelling, a "License"/"Licence" — every assertion
	// in TestNameID0IsConsultedOnlyWhenNameID13IsAbsent about record 0 being
	// CONSULTED goes quiet, answered by NO EVIDENCE, and stays green.
	for _, pin := range []struct {
		sentence string
		id       string
	}{
		{silSentence, "OFL-1.1"},
		{apacheSentence, "Apache-2.0"},
		{ubuntuSentence, "Ubuntu-font-1.0"},
	} {
		index := slices.IndexFunc(admitLicenceSignatures, func(s licenceSignature) bool { return s.id == pin.id })
		if index < 0 {
			t.Fatalf("precondition: this file synthesises %q to stand for the admit row %q, and no such row exists any more", pin.sentence, pin.id)
		}
		if !admitLicenceSignatures[index].pattern.MatchString(pin.sentence) {
			t.Errorf("precondition: the sentence this file synthesises for %q is %q, and the row's own pattern %v does not match it — the constant and the pattern have drifted apart, so every test built on that sentence is answered by NO EVIDENCE and passes for the wrong reason", pin.id, pin.sentence, admitLicenceSignatures[index].pattern)
		}
	}
}

// variableNotoBytes is the OFL confirmation control. It is VARIABLE, which is
// why every assertion over it in this file goes through the licence door
// DIRECTLY: routed through embedFontFamily the `fvar` refusal fires first
// (component_commands.go:2414) and would mask the licence outcome entirely —
// a green assertion about an arm that was never reached.
func variableNotoBytes(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fonts", "notosansthai-variable-testonly", "NotoSansThai-VF.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the confirmation control %s: %v", path, err)
	}
	return data
}

// TestRefuseContradictedLicenceRefusesTheDeclaredIdTheBytesContradict is the
// POSITIVE CONTROL the ruling requires: without it the guard could ship
// having never been observed to refuse anything at all.
//
// RED-PROVED BY DELETING THE GUARD, not by falsifying a condition (recorded
// 2026-09-03, wd folio-go, `go test ./internal/fontset/`): with the body of
// RefuseContradictedLicence replaced by `return nil` this test reds, and so
// does the copyleft control below. Inverting a predicate would only have
// proved arm order.
func TestRefuseContradictedLicenceRefusesTheDeclaredIdTheBytesContradict(t *testing.T) {
	err := RefuseContradictedLicence("Roboto", "OFL-1.1", testFontBytes(t))
	if err == nil {
		t.Fatal("a face whose own name table names the Apache License was admitted under a declared OFL-1.1 — the tie the build gate holds the 21 committed faces to does not exist at runtime, and Epic 16 has replaced that gate with something strictly weaker (D-8.6.5)")
	}
	// THE MESSAGE MUST NAME BOTH SIDES. Either alone is unactionable: the
	// author cannot tell whether the catalogue row is wrong or the binary is
	// the wrong binary without seeing the two statements together.
	for _, want := range []string{"Roboto", "OFL-1.1", "Apache License"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// TestRefuseContradictedLicenceRefusesWhatTheOFLRowContradicts is the
// CONTRADICTION control for the OFL row, and it is owed because every other
// assertion in this file that a refusal HAPPENS is driven by the Apache row or
// the Ubuntu row. The OFL row covers 19 of the 21 catalogue faces and the OFL
// majority of upstream, and until this test it was only ever asserted to
// ADMIT — which is also what a row matching NOTHING returns. A broken OFL row
// is therefore invisible to an admit-only assertion.
//
// Measured, at the review that added this: narrowing the OFL pattern to
// `(?i)^This Font Software is licensed under the SIL Open Font License` left
// the whole internal/fontset suite GREEN while breaking real faces —
// `cascadiacode`'s record 13 OPENS "Microsoft supplied font..." and carries
// the OFL sentence further in, which is the very reason the table's comment
// says SUBSTRING and not prefix.
//
// So both shapes are asserted, and both use the REAL committed OFL fixture's
// own statement rather than a sentence invented here: the fixture as
// committed, and the same bytes' statement carried behind a leading clause —
// the cascadiacode shape, which no committed face in THIS repository has.
func TestRefuseContradictedLicenceRefusesWhatTheOFLRowContradicts(t *testing.T) {
	oflStatement, present := ReadLicenceStatement(variableNotoBytes(t))
	if !present {
		t.Fatal("precondition: the OFL fixture makes no readable statement, so neither shape below is a contradiction")
	}

	// The fixture as committed, declared the one other permissive id whose
	// bytes are also committed here. A TRUE contradiction: the statement names
	// an admitted licence, and it is not the one being claimed.
	if err := RefuseContradictedLicence("Noto Sans Thai", "Apache-2.0", variableNotoBytes(t)); err == nil {
		t.Error("a face whose own name table names the SIL OFL was admitted under a declared Apache-2.0 — the OFL row, which covers 19 of the 21 catalogue faces and the OFL majority of upstream, refuses nothing")
	} else {
		requireNamesBothSides(t, err, "Noto Sans Thai", "Apache-2.0", "SIL Open Font License")
	}

	// THE SUBSTRING SHAPE. The same real statement, behind the leading clause
	// cascadiacode carries. This is the arm that dies under an anchored
	// pattern while every other test in the file stays green.
	embedded := faceWithNames(t, nameRecord{id: 13, value: "Microsoft supplied font. " + oflStatement})
	err := RefuseContradictedLicence("Cascadia-shaped", "Apache-2.0", embedded)
	if err == nil {
		t.Fatal("a face whose record 13 OPENS with another clause and names the SIL OFL further in was admitted under a declared Apache-2.0 — the OFL row has become a prefix or equality match, and `cascadiacode` and `cascadiamono` are exactly this shape")
	}
	requireNamesBothSides(t, err, "Cascadia-shaped", "Apache-2.0", "SIL Open Font License")
}

// requireNamesBothSides is the "either side alone is unactionable" assertion,
// factored out because four tests now make it: the author cannot tell whether
// the catalogue row is wrong or the binary is the wrong binary without seeing
// the declaration and the bytes' own words together.
func requireNamesBothSides(t *testing.T, err error, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// TestContradictionRefusalNamesEveryMatchedLicence pins the decision the admit
// scan already makes and the message used not to report. The scan visits EVERY
// row before deciding, so that table order does not become policy; naming only
// the first match would hand that policy straight back in the one artefact a
// reader actually diagnoses from, and would send an author to fix half of what
// is wrong with the face.
func TestContradictionRefusalNamesEveryMatchedLicence(t *testing.T) {
	// A statement naming TWO admitted licences, declared a THIRD. Neither
	// match is the declaration, so both are contradictions and both are owed.
	both := faceWithNames(t, nameRecord{id: 13, value: "Portions " + apacheSentence + ". Portions licensed under the SIL Open Font License."})
	err := RefuseContradictedLicence("Two-Terms Face", "Ubuntu-font-1.0", both)
	if err == nil {
		t.Fatal("a face naming two admitted licences, neither of them the declared one, was admitted")
	}
	// ASSERTED OVER THE CLAUSE THAT NAMES THE ROWS, NOT OVER THE WHOLE
	// MESSAGE. The message also QUOTES the statement, and the statement is
	// where both licence names came from — so a search of the whole string
	// finds "the Apache License, Version 2.0" in the face's own words and
	// passes no matter what the guard concluded. That is the vacuity this
	// test exists to avoid, so it reads only the head.
	head, _, found := strings.Cut(err.Error(), "The face says:")
	if !found {
		t.Fatalf("the refusal no longer carries a `The face says:` clause, so this test cannot separate what the GUARD concluded from what the FACE said: %v", err)
	}
	for _, want := range []string{"the SIL Open Font License", "the Apache License, Version 2.0"} {
		if !strings.Contains(head, want) {
			t.Errorf("the refusal names only some of what the bytes matched — %q is missing from %q, so the message reports whichever row happens to come first in the table and table order has become policy after all", want, head)
		}
	}
}

// TestRefuseBeatsConfirmWhenAStatementNamesBoth pins the PRECEDENCE between
// the two halves over a statement that triggers both, which is the case real
// dual-licensed faces present: an OFL sentence and a GPL-with-font-exception
// in one record. The refuse half runs first and against every face, so this
// must be a REFUSAL — a confirmation of the declared OFL-1.1 would be a face
// whose own bytes name the GPL walking in under a token this project admits,
// which is AD-26's stated Prevents in full.
func TestRefuseBeatsConfirmWhenAStatementNamesBoth(t *testing.T) {
	dual := faceWithNames(t, nameRecord{id: 13, value: silSentence + " It is also available under the GNU General Public License version 2 with the font exception."})
	err := RefuseContradictedLicence("Dual Licensed", "OFL-1.1", dual)
	if err == nil {
		t.Fatal("a face whose record 13 names the SIL OFL AND the GNU GPL was admitted under a declared OFL-1.1 — the admit half is deciding before the refuse half, so any copyleft statement can be waved through by pairing it with a permissive sentence and declaring the permissive id")
	}
	if !strings.Contains(err.Error(), "GNU GPL family") {
		t.Errorf("the refusal of a dual OFL/GPL statement does not name the copyleft family, so it was refused as a mere contradiction and the copyleft floor did not fire: %v", err)
	}
}

// hostFailureMessageCutBytes is maxComponentFailureMessageBytes
// (component_commands.go:1974), HAND-COPIED across a package boundary and tied
// by the source read in the test below rather than by the compiler — package
// `folio8` keeps it unexported and imports this package, not the reverse.
const hostFailureMessageCutBytes = 512

// hostFailureMessageCutLiteral reads that constant back out of the host's own
// source, the way canvas_projection_wire_test.go reads engine-protocol.ts. A
// bound copied by hand and never checked is the defect this file is otherwise
// full of guards against.
var hostFailureMessageCutLiteral = regexp.MustCompile(`maxComponentFailureMessageBytes\s*=\s*(\d+)`)

// TestRefusalsSurviveTheHostsFailureMessageCut is the bound on the quoted
// statement, asserted rather than described.
//
// The host cuts a component failure message at 512 bytes and the cut takes the
// TAIL, which is where `The face says: %q` sits — the last clause of both
// refusals and the whole reason either is actionable. A face's record 13 is
// frequently the ENTIRE OFL body and record 0 can run to kilobytes, so without
// a bound the common case is a refusal whose most informative clause is the
// part the host deletes.
func TestRefusalsSurviveTheHostsFailureMessageCut(t *testing.T) {
	// The hand-copied bound, tied to the host's own literal.
	host, err := os.ReadFile(filepath.Join("..", "..", "component_commands.go"))
	if err != nil {
		t.Fatalf("read the host's own failure-message bound: %v", err)
	}
	literal := hostFailureMessageCutLiteral.FindSubmatch(host)
	if literal == nil {
		t.Fatal("component_commands.go no longer declares maxComponentFailureMessageBytes where this test can read it; re-derive the extraction rather than deleting the check — the bound below is hand-copied and this read is the only thing tying it to the host")
	}
	if got := string(literal[1]); got != "512" {
		t.Fatalf("the host now cuts failure messages at %s bytes and this file still bounds its excerpt against %d — the two moved apart, and a refusal's quoted statement is what the difference deletes", got, hostFailureMessageCutBytes)
	}

	// A record 13 carrying the whole licence body, which is the ordinary
	// shape rather than an invented one.
	body := strings.Repeat(silSentence+" This license is available with a FAQ at: https://scripts.sil.org/OFL. ", 40)
	if len(body) < 4096 {
		t.Fatalf("this test's subject is a multi-kilobyte statement and the one it built is %d bytes", len(body))
	}

	for _, arm := range []struct {
		what      string
		declared  string
		statement string
		names     []string
	}{
		{
			what:      "CONTRADICTION",
			declared:  "Apache-2.0",
			statement: body,
			names:     []string{"Big Face", "Apache-2.0", "SIL Open Font License"},
		},
		{
			what:      "REFUSE-SIGNATURE",
			declared:  "OFL-1.1",
			statement: "Licensed under the GNU General Public License version 3. " + body,
			names:     []string{"Big Face", "OFL-1.1", "GNU GPL family"},
		},
		{
			// THE CUT LANDS INSIDE A MULTI-BYTE RUNE HERE, which is what
			// makes the rune-boundary walk in statementExcerpt a witnessed
			// requirement rather than a defensive habit. The prefix is 8
			// bytes and every rune after it is 3, so byte 72 falls one byte
			// into a rune: a cut by bytes alone emits a broken UTF-8
			// sequence into a message the host must encode as JSON.
			what:      "REFUSE-SIGNATURE, NON-LATIN",
			declared:  "OFL-1.1",
			statement: "GPL-3.0 " + strings.Repeat("本字型軟體採用開放字型授權條款釋出並依規定散布。", 200),
			names:     []string{"Big Face", "OFL-1.1", "GNU GPL family"},
		},
	} {
		face := faceWithNames(t, nameRecord{id: 13, value: arm.statement})
		err := RefuseContradictedLicence("Big Face", arm.declared, face)
		if err == nil {
			t.Errorf("%s: a %d-byte statement was admitted, so this arm asserts nothing about the message", arm.what, len(arm.statement))
			continue
		}
		if len(err.Error()) > hostFailureMessageCutBytes {
			t.Errorf("%s: the refusal is %d bytes and the host cuts at %d, so the tail — `The face says:`, the one side of the comparison the author cannot look up — is deleted before it is ever read: %v", arm.what, len(err.Error()), hostFailureMessageCutBytes, err)
		}
		// AND IT STILL NAMES BOTH SIDES. A bound that made the message fit by
		// losing what it is for would be worse than the truncation.
		requireNamesBothSides(t, err, arm.names...)
		// AND THE CUT IS MARKED, so nobody reads the excerpt as the whole of
		// what the bytes said.
		if !strings.Contains(err.Error(), statementExcerptElision) {
			t.Errorf("%s: a %d-byte statement was quoted with no mark of the elision, so the excerpt reads as the face's entire statement: %v", arm.what, len(arm.statement), err)
		}
		// AND THE CUT IS AT A RUNE BOUNDARY. %q escapes an invalid byte as
		// \xNN rather than failing, so a mid-rune cut is silent in the string
		// itself; the mangled rune is what a reader sees instead of the word
		// the face wrote.
		if strings.Contains(err.Error(), `\x`) {
			t.Errorf("%s: the quoted statement carries an escaped raw byte, so the excerpt was cut in the middle of a multi-byte rune: %v", arm.what, err)
		}
	}
}

// TestRefuseContradictedLicenceAdmitsWhatTheBytesConfirm is the
// over-broadness control. A guard that refused everything would satisfy the
// test above and be worthless, so the same two faces are declared the licence
// their own bytes actually name and both must be admitted.
func TestRefuseContradictedLicenceAdmitsWhatTheBytesConfirm(t *testing.T) {
	if err := RefuseContradictedLicence("Roboto", "Apache-2.0", testFontBytes(t)); err != nil {
		t.Errorf("a face declared Apache-2.0 whose own name table names the Apache License was refused: %v", err)
	}
	if err := RefuseContradictedLicence("Noto Sans Thai", "OFL-1.1", variableNotoBytes(t)); err != nil {
		t.Errorf("a face declared OFL-1.1 whose own name table names the SIL OFL was refused: %v", err)
	}
}

// TestRefuseContradictedLicenceRefusesCopyleftWhateverIsDeclared is the
// SECOND positive control, and it covers the half the committed fixtures
// cannot reach (D-16.R.9). This is the part D-16.R.7 added as NEW — the
// build-time tie never had it — and a new guard with no control is exactly
// the vacuity the ruling exists to prevent.
//
// The declared id is `OFL-1.1` in every row, which is the whole point: the
// declaration is the thing under suspicion, so the refuse half must not
// consult it. AD-26 binds ALL, and its Prevents is copyleft arriving through
// a plausible-looking package.
func TestRefuseContradictedLicenceRefusesCopyleftWhateverIsDeclared(t *testing.T) {
	for _, statement := range []string{
		"Licensed under the GNU General Public License version 3",
		"Licensed under the GPL",
		"This font is released under the GPLv2 with the font exception",
		"Licensed under the LGPL-2.1",
		"Licensed under the GNU Affero General Public License",
		"Licensed under AGPL-3.0",
		"Licensed under the Server Side Public License",
		"Licensed under SSPL",
		"Creative Commons Attribution-ShareAlike 4.0 International",
		"Licensed under CC BY-SA 4.0",
	} {
		face := faceWithNames(t, nameRecord{id: 13, value: statement})
		err := RefuseContradictedLicence("Copyleft Face", "OFL-1.1", face)
		if err == nil {
			t.Errorf("a face whose own name table reads %q was admitted under a declared OFL-1.1 — the copyleft floor does not exist, and a share-alike font program would be embedded and subset into every PDF the author produces", statement)
			continue
		}
		if !strings.Contains(err.Error(), statement) {
			t.Errorf("the refusal of %q does not quote what the bytes said: %v", statement, err)
		}
	}

	// AND THE OVER-BROADNESS CONTROL FOR THIS HALF TOO. The refuse patterns
	// must not fire on the permissive sentences real faces carry, or every
	// admitted face becomes a refusal and the guard is turned off within a
	// week.
	//
	// EACH SENTENCE IS DECLARED THE ID ITS OWN BYTES NAME, and the assertion
	// is `err == nil`. Both of those are corrections. Declaring every row
	// `OFL-1.1` made the Apache and Ubuntu rows TRUE CONTRADICTIONS, so they
	// returned a non-nil error for a reason that had nothing to do with the
	// refuse half; and the check they were put through — `err != nil &&
	// strings.Contains(err.Error(), "copyleft")` — then passed having asserted
	// nothing at all, and would have gone fully vacuous for the OFL row too
	// the moment the refusal's wording dropped the word "copyleft". Declaring
	// the confirming id makes NIL the only correct answer, which is an
	// assertion that depends on no substring of the refusal prose.
	for _, confirmed := range []struct {
		statement string
		declared  string
	}{
		{silSentence, "OFL-1.1"},
		{apacheSentence, "Apache-2.0"},
		{ubuntuSentence, "Ubuntu-font-1.0"},
	} {
		face := faceWithNames(t, nameRecord{id: 13, value: confirmed.statement})
		if err := RefuseContradictedLicence("Permissive Face", confirmed.declared, face); err != nil {
			t.Errorf("the permissive statement %q, declared %q — the very id its own bytes name, so a CONFIRMATION and the least refusable face there is — was refused: a signature has widened onto the sentences real faces carry, and every admitted face becomes a refusal: %v", confirmed.statement, confirmed.declared, err)
		}
	}
}

// TestRefuseContradictedLicenceAdmitsSilence is the arm that will look like a
// hole to the next reader, which is why the reasoning is in the code beside
// it and asserted here rather than merely described.
//
// The threat is a face travelling under ANOTHER PROJECT'S terms — a FALSE
// statement. A face that says nothing has made no statement to be false, so
// refusing it catches none of that; and it was MEASURED to cost about a sixth
// of the library (50 of 100 sampled upstream faces refused under the original
// "absent or unparseable => REFUSE" contract). Genuinely unreadable bytes are
// refused one step later by template.DecodeFontForRender / checkSfnt.
func TestRefuseContradictedLicenceAdmitsSilence(t *testing.T) {
	// No name table at all.
	nameless := removeNameTable(t, testFontBytes(t))
	if _, present := ReadLicenceStatement(nameless); present {
		t.Fatal("precondition: the name-stripped face still reports a licence statement, so the strip did not take effect and the arm below is untested")
	}
	if err := RefuseContradictedLicence("Nameless", "OFL-1.1", nameless); err != nil {
		t.Errorf("a face with no name table at all was refused: %v", err)
	}

	// Bytes that are not a font at all: unparseable, and admitted here
	// because this door answers one question only. checkSfnt refuses them.
	if err := RefuseContradictedLicence("Rubbish", "OFL-1.1", []byte("not a font")); err != nil {
		t.Errorf("unreadable bytes were refused by the LICENCE door, which is not its question: %v", err)
	}

	// A statement that is present and recognised by nothing in either table.
	unknown := faceWithNames(t, nameRecord{id: 13, value: "All rights reserved by a licence this table has never heard of."})
	if err := RefuseContradictedLicence("Unknown Terms", "OFL-1.1", unknown); err != nil {
		t.Errorf("a face whose statement matches no signature was refused: %v", err)
	}

	// A SENTENCE IN A NON-LATIN SCRIPT — the matrix's own row, which named
	// `ofl/wdxllubrifonttc` (it states OFL 1.1 in Traditional Chinese) and had
	// no test. Every pattern in both tables is ASCII, so no ASCII regex
	// reaches under this floor: the statement is present and readable, matches
	// nothing, and is NO EVIDENCE. It admits under any declared id, including
	// one it plainly is not — which is the cost of the floor, stated here
	// rather than discovered.
	chinese := "本字型軟體採用 SIL 開放字型授權條款第 1.1 版釋出，並依該授權條款之規定散布。"
	statement, present := ReadLicenceStatement(faceWithNames(t, nameRecord{id: 13, value: chinese}))
	if !present || statement != chinese {
		t.Fatalf("precondition: a Traditional Chinese statement did not survive the name table round trip (present=%v, got %q), so the arm below is testing a different string than the one it names", present, statement)
	}
	for _, declared := range []string{"OFL-1.1", "Apache-2.0"} {
		if err := RefuseContradictedLicence("WDXL Lubrifont TC", declared, faceWithNames(t, nameRecord{id: 13, value: chinese})); err != nil {
			t.Errorf("a face stating its terms in Traditional Chinese, declared %q, was refused: no signature in either table is anything but ASCII, so this can only be a pattern that has started matching what it cannot read: %v", declared, err)
		}
	}

	// A declared id with NO admit-signature row (D-16.R.10). MIT is the
	// reachable case: D-8.5.3 admits it and google/fonts publishes no MIT
	// token for a signature to be minted against — "absence, not narrowing".
	// Refusing here would refuse a licence the OWNER has admitted.
	if slices.ContainsFunc(admitLicenceSignatures, func(s licenceSignature) bool { return s.id == "MIT" }) {
		t.Fatal("this test's subject has gone: admitLicenceSignatures now has an MIT row, so `a declared id with no signature entry` is no longer reachable through MIT — re-derive the case rather than deleting the check")
	}
	if err := RefuseContradictedLicence("MIT Face", "MIT", faceWithNames(t, nameRecord{id: 13, value: "Licensed under the MIT License"})); err != nil {
		t.Errorf("a face declared MIT — an id D-8.5.3 admits and this table deliberately does not cover — was refused: %v", err)
	}
}

// TestNameID0IsConsultedOnlyWhenNameID13IsAbsent holds BOTH halves of
// D-16.R.7's widening, and the second half is the one that keeps it a
// widening rather than a weakening.
//
// Measured at Story 16.1's plan gate: all three static `ufl/` families carry
// NO record 13 and state their terms in record 0, so a tie that looked only
// at 13 would have refused every genuine Ubuntu-licensed family. But if 0
// were consulted whenever 13 failed to MATCH, a face whose 13 says GPL could
// be laundered by a permissive-sounding 0 — defeating the contradiction check
// with the very thing it exists to catch.
func TestNameID0IsConsultedOnlyWhenNameID13IsAbsent(t *testing.T) {
	// ABSENT 13, terms in 0: the `ufl/` shape. Consulted, and it confirms.
	ubuntu := faceWithNames(t, nameRecord{id: 0, value: "Copyright 2011 Canonical Ltd. " + ubuntuSentence})
	statement, present := ReadLicenceStatement(ubuntu)
	if !present || !strings.Contains(statement, "Ubuntu Font Licence") {
		t.Fatalf("record 0 was not consulted for a face carrying no record 13: present=%v statement=%q", present, statement)
	}
	if err := RefuseContradictedLicence("Ubuntu", "Ubuntu-font-1.0", ubuntu); err != nil {
		t.Errorf("a face declaring Ubuntu-font-1.0 and stating those terms in record 0 was refused: %v", err)
	}
	// And the contradiction arm works through record 0 too, or the widening
	// would have opened a door that only ever admits.
	if err := RefuseContradictedLicence("Ubuntu", "OFL-1.1", ubuntu); err == nil {
		t.Error("a face stating the Ubuntu Font Licence in record 0 was admitted under a declared OFL-1.1 — record 0 is being read as a licence-shaped decoration rather than as a statement")
	}

	// PRESENT AND MISMATCHED 13, PERMISSIVE 0: the laundering attempt. 13
	// alone decides, so this must refuse.
	laundered := faceWithNames(t,
		nameRecord{id: 0, value: "Copyright 2020. " + silSentence},
		nameRecord{id: 13, value: "Licensed under the GNU General Public License version 3"},
	)
	err := RefuseContradictedLicence("Laundered", "OFL-1.1", laundered)
	if err == nil {
		t.Fatal("a face whose record 13 names the GPL was admitted because its record 0 sounded permissive — record 0 is being used as a fallback rather than as a widening, which defeats the contradiction check with the very thing it exists to catch")
	}
	if !strings.Contains(err.Error(), "General Public License") {
		t.Errorf("the refusal did not read record 13: %v", err)
	}
	if strings.Contains(err.Error(), "SIL Open Font License") {
		t.Errorf("the refusal quoted record 0, which must not have been consulted at all: %v", err)
	}
}

// TestLicenceSignatureTablesAreOrderedAndNotEmpty is AD-1's guard in this
// file: an ordered slice, never a map, so a face matching two rows produces
// the same sentence on every run. It also stops both tables being emptied
// into vacuous green.
func TestLicenceSignatureTablesAreOrderedAndNotEmpty(t *testing.T) {
	if len(admitLicenceSignatures) == 0 {
		t.Fatal("vacuity guard: the admit table is empty, so every CONFIRMATION and CONTRADICTION assertion in this file would be answered by NO EVIDENCE")
	}
	if len(refuseLicenceSignatures) == 0 {
		t.Fatal("vacuity guard: the refuse table is empty, so the copyleft floor does not exist")
	}
	for _, signature := range admitLicenceSignatures {
		if signature.id == "" || signature.label == "" || signature.pattern == nil {
			t.Errorf("an admit row is incomplete: %+v", signature)
		}
	}
	for _, signature := range refuseLicenceSignatures {
		if signature.id != "" {
			t.Errorf("a refuse row carries an SPDX id (%q); the refuse half is checked against every face whatever it declares and must be keyed by nothing", signature.id)
		}
		if signature.label == "" || signature.pattern == nil {
			t.Errorf("a refuse row is incomplete: %+v", signature)
		}
	}
}

// ---------------------------------------------------------------------------
// THE MIRROR CONTRACT (D-7.4.5 / DW-25, applied by D-16.R.7).

// designerLicenceSignatureTable extracts the object literal the designer's
// build-time tie is keyed off. Anchored on the TS declaration's OWN
// identifier, because an object literal is that file's commonest shape and an
// unanchored match would read some other table and compare it to this one — a
// green test asserting the wrong thing.
var designerLicenceSignatureTable = regexp.MustCompile(`(?s)const licenceSignatures[^=]*=\s*\{(.*?)\n\}`)

// designerLicenceSignatureKey pulls the SPDX ids out of that literal.
var designerLicenceSignatureKey = regexp.MustCompile(`(?m)^\s*'([^']+)'\s*:`)

// designerTokenTable extracts the RUNTIME licence table Story 16.1 added —
// `folio-designer/src/font-licence.ts`'s `licenceTokens`, the closed map from an
// upstream `METADATA.pb` token to the SPDX id a `.folio` carries.
//
// A SECOND TABLE ON THE OTHER SIDE OF THE SAME SEAM, and the one that actually
// decides what reaches `RefuseContradictedLicence`. The build-time table above
// runs over 21 reviewed faces; this one runs over ~1,946 families nobody has
// looked at, and its output is literally this package's input:
// `component_commands.go` passes the browser's `licence` wire field straight in.
// D-16.R.10 names exactly this gap and nominates this test to close it.
//
// Anchored on the declaration's OWN identifier for the same reason as above: an
// object literal is that file's commonest shape, and an unanchored match would
// read some other table and compare it to this one.
var designerTokenTable = regexp.MustCompile(`(?s)const licenceTokens[^=]*=\s*\{(.*?)\n\}`)

// designerTokenSPDX pulls the ADMITTED SPDX ids out of that literal — the
// `{ spdx: 'X' }` rows only. A `{ refusal: … }` row admits nothing and must not
// be compared against Go's signature table: a refused token never produces an
// SPDX id at all.
var designerTokenSPDX = regexp.MustCompile(`\{\s*spdx:\s*'([^']+)'\s*\}`)

// TestGoLicenceTableSubsumesTheDesignerTable is the mirror contract, enforced
// by a test and not by a comment.
//
// TWO TABLES IN TWO LANGUAGES, and the Go one is the authority for documents:
// the designer's runs at BUILD time over the 21 committed faces, while this
// one runs at the moment bytes are written into a `.folio`, over faces nobody
// has reviewed. If the designer's table admitted an id Go's did not, a face
// the build gate had passed would be refused at the pick — or, worse, the two
// halves would disagree about what a licence sentence looks like and only one
// of them would be watched.
//
// It reads the TypeScript as SOURCE TEXT, which is the established idiom here
// (canvas_projection_wire_test.go does the same to engine-protocol.ts). The
// designer file exports nothing, and no shared generated artifact is
// introduced for this: the only Go/TS bridge in the repository,
// folio-designer/src/generated/font-catalogue.ts, is gitignored and TS-only.
//
// SUBSUMPTION IS STRICT TODAY, so this is not vacuous on landing: TS declares
// {OFL-1.1, Ubuntu-font-1.0} and Go declares those plus Apache-2.0. Red-prove
// it by adding a row to the TS table with no Go counterpart.
func TestGoLicenceTableSubsumesTheDesignerTable(t *testing.T) {
	path := filepath.Join("..", "..", "..", "folio-designer", "src", "font-catalogue.test.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		// NOT A SKIP. The guard on the other side of this seam is half of
		// "both, or halt"; a missing one is a finding, not an excuse.
		t.Fatalf("read the designer's build-time licence tie %s: %v", path, err)
	}
	match := designerLicenceSignatureTable.FindSubmatch(source)
	if match == nil {
		t.Fatal("font-catalogue.test.ts no longer declares `const licenceSignatures` where this test can read it; if the table was restructured, re-derive this extraction rather than deleting the check — D-16.R.7 requires BOTH guards or a halt")
	}
	var designerIDs []string
	for _, key := range designerLicenceSignatureKey.FindAllSubmatch(match[1], -1) {
		designerIDs = append(designerIDs, string(key[1]))
	}
	// VACUITY GUARD. "Every TS id is covered by Go" is also true of a table
	// this test read as empty, which is what a restructured literal or a
	// changed quote style would silently produce.
	if len(designerIDs) == 0 {
		t.Fatal("vacuity guard: the extraction read 0 ids out of the designer's licenceSignatures table, so `Go subsumes TS` says nothing")
	}
	// AND THE VACUITY GUARD ABOVE IS NOT ENOUGH, because the extraction can
	// truncate without emptying. designerLicenceSignatureTable is non-greedy
	// to the FIRST column-0 `}`, so a nested object literal — or a reformat
	// that puts a brace at column 0 mid-table — ends the match early and drops
	// every id after that point while still returning a non-empty list. `len
	// == 0` never fires, and "Go subsumes TS" is then asserted over a PARTIAL
	// table: precisely the ids the extraction lost are the ids nothing checks.
	//
	// So the ids the TS table is KNOWN to declare today are named here. A miss
	// means the extraction has rotted, NOT that the table shrank — a table
	// that genuinely shrank is a decision somebody made in font-catalogue.
	// test.ts, and updating this list is part of making it.
	for _, known := range []string{"OFL-1.1", "Ubuntu-font-1.0"} {
		if !slices.Contains(designerIDs, known) {
			t.Errorf("the extraction read %v out of the designer's licenceSignatures table and %q is not among them. The likeliest cause is NOT that the TS table shrank but that THIS TEST'S EXTRACTION ROTTED: the regexp is non-greedy to the first column-0 `}`, so a nested object or a reformat truncates it silently and leaves the vacuity guard above green over a partial table. Re-derive the extraction before touching either table.", designerIDs, known)
		}
	}

	goIDs := make([]string, 0, len(admitLicenceSignatures))
	for _, signature := range admitLicenceSignatures {
		goIDs = append(goIDs, signature.id)
	}
	for _, id := range designerIDs {
		if !slices.Contains(goIDs, id) {
			t.Errorf("the designer's build-time tie admits %q and the Go table — the authority for what goes into a document — has no row for it. The two are a mirror contract: they move in ONE commit. Go declares %v.", id, goIDs)
		}
	}

	// ── STORY 16.1: THE SECOND DESIGNER TABLE, AND WHY IT IS READ HERE ──
	//
	// Until 16.1 the designer had ONE licence table, at build time, over 21
	// reviewed faces — and that is what the extraction above reads. 16.1 added
	// a SECOND, at RUNTIME, which decides what SPDX id a family fetched from
	// the open library publishes into a document. Nothing watched it.
	//
	// The consequence of an unwatched second table is quiet rather than loud:
	// an SPDX id this module can EMIT that `admitLicenceSignatures` has no row
	// for lands in NO EVIDENCE and ADMITS (D-16.R.10), with no tie at all.
	// D-16.R.10 accepted that as a bounded gap FOR TODAY'S CLOSED TOKEN SET —
	// the moment the TS table can emit a fourth id, the gap is bounded by
	// nothing a test reads. This is the test that reads it.
	tokenPath := filepath.Join("..", "..", "..", "folio-designer", "src", "font-licence.ts")
	tokenSource, err := os.ReadFile(tokenPath)
	if err != nil {
		// NOT A SKIP, for the same reason as above: a missing guard on the
		// other side of a mirror contract is a finding, not an excuse.
		t.Fatalf("read the designer's runtime licence token table %s: %v", tokenPath, err)
	}
	tokenMatch := designerTokenTable.FindSubmatch(tokenSource)
	if tokenMatch == nil {
		t.Fatal("font-licence.ts no longer declares `const licenceTokens` where this test can read it; if the table was restructured, re-derive this extraction rather than deleting the check — the runtime table is what decides which SPDX id reaches RefuseContradictedLicence")
	}
	var tokenIDs []string
	for _, key := range designerTokenSPDX.FindAllSubmatch(tokenMatch[1], -1) {
		tokenIDs = append(tokenIDs, string(key[1]))
	}
	// VACUITY GUARD, the same shape as the one above and for the same reason:
	// "every TS id is covered by Go" is also true of a table read as empty.
	if len(tokenIDs) == 0 {
		t.Fatal("vacuity guard: the extraction read 0 admitted SPDX ids out of the designer's licenceTokens table, so `Go subsumes TS` says nothing about the runtime path")
	}
	// AND THE KNOWN-IDS LIST, because the extraction can TRUNCATE without
	// emptying: designerTokenTable is non-greedy to the first column-0 `}`, so
	// a nested literal or a reformat ends the match early and drops every row
	// after it while the vacuity guard stays green over a partial table. A miss
	// here means the extraction rotted, NOT that the table shrank — a table
	// that genuinely shrank is a decision somebody made in font-licence.ts, and
	// updating this list is part of making it.
	for _, known := range []string{"OFL-1.1", "Apache-2.0", "Ubuntu-font-1.0"} {
		if !slices.Contains(tokenIDs, known) {
			t.Errorf("the extraction read %v out of the designer's licenceTokens table and %q is not among them. The likeliest cause is NOT that the TS table shrank but that THIS TEST'S EXTRACTION ROTTED: the regexp is non-greedy to the first column-0 `}`, so a nested object or a reformat truncates it silently. Re-derive the extraction before touching either table.", tokenIDs, known)
		}
	}
	// SUBSUMPTION IS STRICT TODAY AND THAT IS WHY THIS IS NOT VACUOUS: the TS
	// runtime table admits {OFL-1.1, Apache-2.0, Ubuntu-font-1.0} and Go
	// declares exactly those three. Red-prove it by adding a row with a fourth
	// SPDX id to `licenceTokens` and running this test.
	for _, id := range tokenIDs {
		if !slices.Contains(goIDs, id) {
			t.Errorf("the designer's RUNTIME token table can put %q into a document's font.licence and the Go signature table has no row for it, so a face declaring it would be tied against nothing and admitted as NO EVIDENCE. D-16.R.10 bounded that gap to today's closed token set; a fourth id unbounds it. Go declares %v.", id, goIDs)
		}
	}
	// AND THE REFUSED ROWS ARE NOT SILENTLY DROPPED BY THE EXTRACTION. `CC-BY-SA`
	// is in the TS table precisely so that ABSENT and REFUSED stop looking the
	// same; an extraction that read the whole table as admitted rows, or that
	// missed the refused one entirely, would be reading a different table from
	// the one that ships.
	if !bytes.Contains(tokenMatch[1], []byte("CC-BY-SA")) {
		t.Error("the designer's licenceTokens table no longer carries its refused row. It is there so a fifth upstream directory reads as `we have not classified this` rather than as a policy decision nobody made; if it was removed, that is a decision to make in font-licence.ts and to record, not a rotted extraction to paper over")
	}
	if slices.Contains(tokenIDs, "CC-BY-SA") {
		t.Error("the extraction read the REFUSED row as an admitted SPDX id; a refused token produces no SPDX id at all, and comparing it against Go's signature table asserts the wrong thing")
	}
}

// TestTheBuildTimeTieIsStillInPlace is the other half of "both, or halt". The
// designer's tie is not deleted or weakened because Go now checks it: Go
// guards the DOCUMENT, and the designer's guards the 21 faces this repository
// itself redistributes, which Go never sees a pick of.
func TestTheBuildTimeTieIsStillInPlace(t *testing.T) {
	path := filepath.Join("..", "..", "..", "folio-designer", "src", "font-catalogue.test.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the designer's build-time licence tie %s: %v", path, err)
	}
	for _, needle := range []string{
		"const licenceSignatures",
		"licenceSignatures[face.licence]",
		".toMatch(signature as RegExp)",
	} {
		if !strings.Contains(string(source), needle) {
			t.Errorf("font-catalogue.test.ts no longer contains %q — the build-time nameID 13 tie has been deleted or restructured. Story 16.1b ADDS a runtime tie; it does not replace this one (D-16.R.7: both, or halt).", needle)
		}
	}
}
