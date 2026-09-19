// Package pdf serializes the fixed document described by Story 1.1: a
// single A4 page containing one filled rectangle, written as classic
// (uncompressed) PDF 1.7. No compression, no /Info dictionary, no
// /CreationDate or /ModDate (AD-7). This is a throwaway fixture document,
// not a general page-layout system — real page setup, text, fonts and
// templates arrive in later stories.
package pdf

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// Fixed document dimensions and rectangle, in millipoints. These are
// fixture constants for this story only — there is no runtime mm→pt (or
// any other) conversion here; a conversion would be a rounding decision
// this story has no mandate to make.
const (
	mediaBoxWidth  geom.Length = 595276 // A4 width,  595.276pt
	mediaBoxHeight geom.Length = 841890 // A4 height, 841.89pt

	rectX geom.Length = 72500  // 72.5pt
	rectY geom.Length = 100250 // 100.25pt
	rectW geom.Length = 200125 // 200.125pt
	rectH geom.Length = 50000  // 50pt
)

// binaryComment is the four bytes >= 128 that follow the header's binary
// comment marker, signalling to naive line-based tools that this is a
// binary file (a standard PDF convention).
//
// A const string, not a package-level []byte var: this package's whole
// job is producing identical bytes every time, and a mutable
// package-level variable was exactly the kind of state that could let a
// later file in this package silently change every rendered byte from the
// header onward — Dev Notes' "no package-level mutable state" (NFR1.d),
// and this story's QA review, Major 9.
const binaryComment = "\xE2\xE3\xCF\xD3"

// Serialize renders the fixed minimal PDF document: a catalog, a page
// tree, a content stream containing one filled rectangle, and a classic
// (non-stream) cross-reference table.
//
// The output is deterministic by construction, not by coincidence: every
// geometric number — including the MediaBox's own lower-left corner,
// declared as a geom.Rect below rather than as string-literal zeros — is
// written by appendLength, and every structural count, byte length,
// object number, generation number and xref field is written by appendInt
// or appendIntPadded (AD-3 as amended, D-1.1.b). An earlier version of
// this file wrote the MediaBox's first two entries as literal bytes
// inside a dict string constant; the output was correct only because
// appendLength(0) also happens to emit "0", so the invariant AC6 and AD-3
// require held by coincidence rather than by construction — this story's
// QA review, Blocker 1. /ID is derived from the document's own content
// rather than from wall-clock time or randomness (AD-7).
func Serialize() []byte {
	var body []byte

	body = append(body, "%PDF-1.7\n%"...)
	body = append(body, binaryComment...)
	body = append(body, '\n')

	content := buildContentStream()

	// offsets[N] is the byte offset at which "N 0 obj" begins, for
	// N in [1,4]. offsets[0] is unused (index 0 is the xref free entry).
	var offsets [5]int

	offsets[1] = len(body)
	body = appendObjHeader(body, 1)
	body = append(body, "<< /Type /Catalog /Pages "...)
	body = appendRef(body, 2)
	body = append(body, " >>"...)
	body = appendObjFooter(body)

	offsets[2] = len(body)
	body = appendObjHeader(body, 2)
	body = append(body, "<< /Type /Pages /Kids "...)
	// Routed through appendRefArray, not hand-rolled, per Story 2.6
	// finisher's Finding 3: this was the SECOND of the two page-tree
	// emitters appendRefArray's own docblock names as the reason /Kids
	// being single-kid today is a fact about the feature set, not the
	// code. Byte-identical for this always-one-kid document (leading
	// separator design): "[" + appendRef(3) + "]" either way.
	body = appendRefArray(body, []int64{3})
	body = append(body, " /Count "...)
	body = appendInt(body, 1)
	body = append(body, " >>"...)
	body = appendObjFooter(body)

	offsets[3] = len(body)
	body = appendObjHeader(body, 3)
	body = append(body, "<< /Type /Page /Parent "...)
	body = appendRef(body, 2)
	body = append(body, " /MediaBox ["...)
	body = appendMediaBox(body, geom.Rect{X: 0, Y: 0, W: mediaBoxWidth, H: mediaBoxHeight})
	body = append(body, "] /Resources << >> /Contents "...)
	body = appendRef(body, 4)
	body = append(body, " >>"...)
	body = appendObjFooter(body)

	offsets[4] = len(body)
	body = appendObjHeader(body, 4)
	body = append(body, "<< /Length "...)
	body = appendInt(body, int64(len(content)))
	body = append(body, " >>\nstream\n"...)
	body = append(body, content...)
	body = append(body, "endstream"...)
	body = appendObjFooter(body)

	xrefOffset := len(body)
	body = appendXref(body, offsets)

	// Trailer, built up to (not including) the literal "/ID" key: the
	// digest input is every byte written so far, since startxref's offset
	// is already fixed by this point and /ID's own fixed-width value
	// cannot shift anything recorded earlier in the file (see the
	// non-circularity note on computeID).
	body = append(body, "trailer\n<< /Size "...)
	body = appendInt(body, 5)
	body = append(body, " /Root "...)
	body = appendRef(body, 1)
	body = append(body, ' ')

	idHex := computeID(body)

	body = append(body, "/ID ["...)
	body = append(body, '<')
	body = append(body, idHex...)
	body = append(body, '>')
	body = append(body, '<')
	body = append(body, idHex...)
	body = append(body, ">] >>\nstartxref\n"...)
	body = appendInt(body, int64(xrefOffset))
	body = append(body, "\n%%EOF\n"...)

	return body
}

// computeID derives the trailer's /ID value from everything written so
// far. Non-circularity: the trailer is written after the cross-reference
// table, so startxref's offset is already fixed before /ID is computed.
// /ID's value is a fixed-width 32-hex-character string, so writing it
// cannot shift any offset recorded earlier in the file. The digest input
// is every byte from "%PDF-1.7" up to but not including the literal "/ID".
func computeID(soFar []byte) string {
	digest := sha256.Sum256(soFar)
	return strings.ToUpper(hex.EncodeToString(digest[:16]))
}

// buildContentStream renders the one operation this story emits: fill a
// rectangle with the PDF default fill colour (black) — no colour operator
// is emitted, since that would be a number that is not a geom.Length
// reaching the stream, for no benefit (see Dev Notes). The rectangle is a
// geom.Rect (AD-2's "one owner" of geometric scalars, now with an actual
// caller — this story's QA review, Minor 21) whose four fields map
// directly onto the PDF "re" operator's x/y/width/height operands, chosen
// to exercise three, two, one and zero fractional digits in one line.
func buildContentStream() []byte {
	fillRect := geom.Rect{X: rectX, Y: rectY, W: rectW, H: rectH}

	var c []byte
	c = appendLength(c, fillRect.X)
	c = append(c, ' ')
	c = appendLength(c, fillRect.Y)
	c = append(c, ' ')
	c = appendLength(c, fillRect.W)
	c = append(c, ' ')
	c = appendLength(c, fillRect.H)
	c = append(c, " re\nf\n"...)
	return c
}

// appendMediaBox appends a MediaBox array's four bare numbers
// (llx lly urx ury), space-separated, all four routed through
// appendLength — including the lower-left corner, which an earlier
// version of this file wrote as literal "0 0 " bytes untouched by any
// emitter (this story's QA review, Blocker 1).
func appendMediaBox(dst []byte, box geom.Rect) []byte {
	dst = appendLength(dst, box.X)
	dst = append(dst, ' ')
	dst = appendLength(dst, box.Y)
	dst = append(dst, ' ')
	dst = appendLength(dst, box.X+box.W)
	dst = append(dst, ' ')
	dst = appendLength(dst, box.Y+box.H)
	return dst
}

// appendObjHeader appends "N 0 obj\n" for an indirect object, routing
// both the object number and its (always zero, in this document)
// generation number through appendInt, per D-1.1.b's routing table.
func appendObjHeader(dst []byte, objNum int64) []byte {
	dst = appendInt(dst, objNum)
	dst = append(dst, ' ')
	dst = appendInt(dst, 0)
	dst = append(dst, " obj\n"...)
	return dst
}

// appendObjFooter appends the "\nendobj\n" that closes every indirect
// object opened by appendObjHeader.
func appendObjFooter(dst []byte) []byte {
	return append(dst, "\nendobj\n"...)
}

// appendRef appends an indirect reference "N 0 R", routing both the
// target object number and its (always zero, in this document)
// generation number through appendInt. An earlier version of this file
// wrote every reference — /Pages, /Kids, /Parent, /Contents, /Root — as a
// literal string inside a dict constant, contradicting D-1.1.b's routing
// table, which the Delivery Log claimed (incorrectly) was fully exercised
// (this story's QA review, Major 8).
// appendRefArray appends a PDF array of indirect references — "[8 0 R 10 0 R]"
// — from the slice itself, so the separator CANNOT be omitted.
//
// WHY THIS EXISTS RATHER THAN A GUARD OVER THE HAND-ROLLED LOOP. Story 2.6
// emitted the page tree as `for _, id := range ids { b.writeRef(id) }`, and
// appendRef emits "N 0 R" with no trailing space — correct at every other
// call site, because each of them follows the ref with a literal. Two refs
// back-to-back produced "[8 0 R10 0 R]". A PDF tokenizer reads "R10" as one
// unknown token, so NEITHER kid resolved, the page tree was EMPTY, and a
// recorded golden shipped that no viewer would open.
//
// The missing separator is not a fact about /Kids. appendRef has around
// thirteen call sites and there are TWO page-tree emitters (document.go's
// single-kid literal and textdoc.go's loop). CORRECTED by the finisher
// (Finding 3): /Kids is NOT the only ref array — textdoc.go's
// /DescendantFonts is also one, live in most goldens. What is true is
// narrower: every ref array this module emits is always SINGLE-ELEMENT
// today except /Kids, which is a fact about today's feature set, not about
// this code — and both document.go's page tree and textdoc.go's
// /DescendantFonts now route through this function too, so the class is
// prevented by construction wherever it is reachable at all. Story 2.7's
// page numbering, Epic 4's tables, and any future /Annots or name tree
// remain the ones that can make a NEW array multi-element. So the
// separator is made unomittable rather than guarded: prevention by
// construction outranks any check over it.
//
// THE SEPARATOR IS LEADING, not trailing. That keeps the one-element case
// exactly "[8 0 R]" — byte-for-byte what every single-page golden already
// contains — so this change moves no committed artifact.
func appendRefArray(dst []byte, objNums []int64) []byte {
	dst = append(dst, '[')
	for i, num := range objNums {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = appendRef(dst, num)
	}
	return append(dst, ']')
}

func appendRef(dst []byte, objNum int64) []byte {
	dst = appendInt(dst, objNum)
	dst = append(dst, ' ')
	dst = appendInt(dst, 0)
	dst = append(dst, " R"...)
	return dst
}

// appendXref appends a classic (non-stream) cross-reference table: the
// free entry for object 0, then one 20-byte in-use entry per object
// 1..4, each of the form "%010d 00000 n \n" (note the trailing space
// before the newline). /Size, the free entry's offset/generation fields
// and every in-use offset/generation field are routed through appendInt /
// appendIntPadded, per D-1.1.b.
func appendXref(dst []byte, offsets [5]int) []byte {
	dst = append(dst, "xref\n0 "...)
	dst = appendInt(dst, 5)
	dst = append(dst, '\n')

	dst = appendIntPadded(dst, 0, 10)
	dst = append(dst, ' ')
	dst = appendIntPadded(dst, 65535, 5)
	dst = append(dst, " f \n"...)

	for i := 1; i <= 4; i++ {
		off := int64(offsets[i])
		// appendIntPadded widens rather than truncates when a value
		// exceeds its field width — correct for a general zero-fill
		// helper, wrong for a fixed-width xref entry: a 21-byte entry
		// desynchronises every subsequent entry's byte offset. Guard the
		// one place this document's structure actually depends on the
		// field staying exactly 10 digits wide (this story's QA review,
		// Minor 17).
		if off > 9999999999 {
			panic("internal/pdf: xref offset exceeds the 10-digit fixed field width")
		}
		dst = appendIntPadded(dst, off, 10)
		dst = append(dst, ' ')
		dst = appendIntPadded(dst, 0, 5)
		dst = append(dst, " n \n"...)
	}
	return dst
}
