package folio8

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// THE GO HALF OF THE MACHINE FONT STORE'S KEY TIE (Story 16.2, patch 7).
//
// ⚠ READ THIS TOGETHER WITH ITS OTHER HALF. The designer's half is
// `folio-designer/src/font-store.test.ts`, in the describe block
// `the content address the store keys by`, whose constant is named
// `goDerivedAssetKey`. That file names this one; this one names that one. A
// reader who finds either finds the other, which is the whole reason both
// comments exist.
//
// WHAT THE TIE IS, STATED PLAINLY: TWO SUITES PINNED TO ONE SHARED CONSTANT.
// It is not a generated fixture, not a golden file and not a cross-process
// check. It is one 64-character hex digest and one 110-byte input, written out
// as literals on both sides of the language boundary, with each side computing
// the digest by its own means — `crypto/sha256` here, `crypto.subtle` there.
//
// WHY THAT IS WORTH HAVING. The store keys a face by the SHA-256 of its bytes
// because that is the address `.folio`'s `assets` map uses, derived in
// `embedFontFamily` (`component_commands.go`) as
// `fmt.Sprintf("%x", sha256.Sum256(decoded))`. If the two addressings ever
// disagreed, a store "hit" would be a hit on bytes the document does not hold —
// the silent substitution the content hash exists to refuse. Before this file
// the digest appeared in exactly ONE place in the repository, a designer test,
// so changing the Go derivation reddened Go's own tests and left the designer's
// claim about Go standing and green. Now a change on either side has to face
// the same literal from both.
//
// WHAT IT IS NOT. It does not prove the two hash implementations agree on
// arbitrary input — one shared vector is one shared vector — and it is not a
// substitute for the designer's own second witness (that file also checks
// `crypto.subtle` against Node's `createHash`, which catches a hex-encoding
// mistake). It pins agreement at one point that both sides must pass through.

// storedFaceKeyTieDigest is THE SHARED CONSTANT. The same 64 characters are
// written in `folio-designer/src/font-store.test.ts` as `goDerivedAssetKey`.
// Neither copy may be "adjusted" to make the other pass: a disagreement means
// one of the two addressings moved, and the answer is to find out which.
const storedFaceKeyTieDigest = "71a8f6e9b586701b742afb6a357afc3f01e4a817ec26fe43a83365f6611a847f"

// storedFaceKeyTieByteLength is the length guard the designer's half also
// carries. The digest is only meaningful over a known input, so if the fixture
// moves, this reds FIRST and says the digest must be RE-DERIVED rather than
// edited to match.
const storedFaceKeyTieByteLength = 110

// storedFaceKeyTieFixture rebuilds, in Go, exactly the bytes
// `folio-designer/src/test/sfnt-fixture.ts` builds for
// `sfntWithCopyright("Copyright 2026 The Folio Authors")`: a minimal sfnt
// carrying one table, `name`, holding one Windows-platform (3/1/0) nameID 0
// record in UTF-16BE.
//
// IT IS REBUILT RATHER THAN COMMITTED AS A BLOB ON PURPOSE. A committed blob
// would agree with the designer by being a copy of it; an independent
// construction from the same written-down layout can DISAGREE, and the length
// and digest assertions below are where that disagreement would surface. It is
// not a font — no glyphs, no `head`, no `OS/2` — and nothing here asks the
// engine to read it as one.
func storedFaceKeyTieFixture() []byte {
	const copyright = "Copyright 2026 The Folio Authors"

	// The name record's string data: UTF-16BE code UNITS, which is what the
	// `name` table stores. Every character here is ASCII, so one unit each.
	storage := make([]byte, 0, len(copyright)*2)
	for _, unit := range []rune(copyright) {
		storage = append(storage, byte(uint16(unit)>>8), byte(uint16(unit)&0xff))
	}

	be16 := func(target []byte, value int) []byte {
		return append(target, byte((value>>8)&0xff), byte(value&0xff))
	}
	be32 := func(target []byte, value int) []byte {
		return append(target, byte((value>>24)&0xff), byte((value>>16)&0xff), byte((value>>8)&0xff), byte(value&0xff))
	}

	// The `name` table: format 0, one record, storage beginning just past the
	// single 12-byte record.
	name := make([]byte, 0, 6+12+len(storage))
	name = be16(name, 0)      // format
	name = be16(name, 1)      // count
	name = be16(name, 6+1*12) // stringOffset
	name = be16(name, 3)      // platformID: Windows
	name = be16(name, 1)      // encodingID: Unicode BMP
	name = be16(name, 0)      // languageID
	name = be16(name, 0)      // nameID 0: copyright
	name = be16(name, len(storage))
	name = be16(name, 0) // offset into storage
	name = append(name, storage...)

	// The table directory: sfnt version 1.0, one table, and the three binary
	// search fields left zero exactly as the designer's builder leaves them.
	const directorySize = 12 + 1*16
	out := make([]byte, 0, directorySize+len(name))
	out = be32(out, 0x00010000)
	out = be16(out, 1) // numTables
	out = be16(out, 0)
	out = be16(out, 0)
	out = be16(out, 0)
	out = append(out, 'n', 'a', 'm', 'e')
	out = be32(out, 0) // checksum, zero in the fixture
	out = be32(out, directorySize)
	out = be32(out, len(name))
	return append(out, name...)
}

// TestStoredFaceKeyTie is the Go half of the tie. See the file comment: the
// designer's half is `folio-designer/src/font-store.test.ts`.
func TestStoredFaceKeyTie(t *testing.T) {
	fixture := storedFaceKeyTieFixture()

	// THE LENGTH GUARD FIRST. A digest over unknown bytes says nothing, and if
	// this reds the answer is to RE-DERIVE the constant over the new fixture on
	// both sides, never to edit either copy until it passes.
	if len(fixture) != storedFaceKeyTieByteLength {
		t.Fatalf("the shared fixture is %d bytes, want %d: the designer's `sfntWithCopyright` fixture and this rebuild of it have diverged, so the shared digest must be re-derived on BOTH sides rather than adjusted on either",
			len(fixture), storedFaceKeyTieByteLength)
	}

	// THE DERIVATION `embedFontFamily` USES, spelled the way it spells it:
	// `fmt.Sprintf("%x", sha256.Sum256(decoded))` at `component_commands.go`.
	if key := fmt.Sprintf("%x", sha256.Sum256(fixture)); key != storedFaceKeyTieDigest {
		t.Fatalf("Go derives %s for the shared fixture, and the designer's store pins %s.\n"+
			"These are the SAME address by construction: `embedFontFamily` keys a document's `assets` map by the SHA-256 of the face bytes, and `folio-designer/src/font-store.ts`'s `storedFaceKey` keys this machine's store by the same digest. A disagreement means one of the two addressings moved, and a store hit would then be a hit on bytes the document does not hold.\n"+
			"Find out which side moved. Do not edit either constant to make the other pass.",
			key, storedFaceKeyTieDigest)
	}

	// AND A SECOND, INDEPENDENT SPELLING OF THE SAME DIGEST — the streaming
	// API and `encoding/hex` rather than `Sum256` and `%x` — because `%x` over
	// a `[32]byte` is the one part of the Go side that is a formatting choice
	// rather than the hash. The designer's half runs the mirror image of this
	// check, `crypto.subtle` against Node's `createHash`.
	digest := sha256.New()
	digest.Write(fixture)
	if key := hex.EncodeToString(digest.Sum(nil)); key != storedFaceKeyTieDigest {
		t.Fatalf("the streaming spelling derives %s, want %s — the disagreement is in this file's hex formatting, not in the addressing", key, storedFaceKeyTieDigest)
	}
}

// TestEmbedFontFamilyKeysByTheSameDerivationTheTiePins connects the shared
// constant above to the code path that actually matters.
//
// The tie pins a digest. THIS pins the claim that `embedFontFamily` addresses a
// document's asset by that same derivation, over a real committed face driven
// through the real command — so a change to the key line in
// `component_commands.go` cannot leave the tie above looking healthy while the
// document is keyed some other way.
func TestEmbedFontFamilyKeysByTheSameDerivationTheTiePins(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai

	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))

	// Derived here by the streaming spelling, so this is not the same
	// expression as the one under test restated.
	digest := sha256.New()
	digest.Write(face)
	want := hex.EncodeToString(digest.Sum(nil))
	if _, ok := tpl.doc.Assets[want]; !ok {
		keys := make([]string, 0, len(tpl.doc.Assets))
		for key := range tpl.doc.Assets {
			keys = append(keys, key)
		}
		t.Fatalf("embedFontFamily stored the face under %v, not under the SHA-256 of its bytes (%s).\n"+
			"The designer's machine store keys by that digest — see `folio-designer/src/font-store.ts` and the tie above — so a document keyed any other way makes every store hit a hit on bytes the document does not hold.",
			keys, want)
	}
}
