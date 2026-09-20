//go:build nocjkface

package fonts

import (
	"strings"
	"testing"
)

// unshippedFaceDirs names the face directories `-tags nocjkface` reads,
// checks and then leaves out of the shipped accounting, because that
// build does not compile their go:embed directive
// (spec-deferred-offline-cache, CAP-6).
//
// It is exactly the CJK face. Its directory, binary, LICENSE and
// NOTICE.md are all still present and are all still parsed by
// readNoticeRecords; what this declares is only that fonts.Shipped()
// does not carry it in THIS build, which is what keeps the join total in
// both directions rather than failing on a NOTICE for bytes nothing
// ships.
var unshippedFaceDirs = []string{"notosanssc"}

// unshippedFaceKeys is the fonts.Shipped() side of the same statement:
// the KEY the default build files those bytes under. It is kept apart
// from the directory list above because the two live in different
// namespaces — one is a path under folio-go/fonts/, the other is a face
// name a `.folio` document's fallback chain writes — and D-B forbids
// deriving either from the other.
var unshippedFaceKeys = []string{"Noto Sans SC"}

// TestShippedOmitsTheCJKFaceUnderTheTag is the measurement CAP-6 turns
// on, taken in the build that matters. Ten faces, no "Noto Sans SC", and
// — the part a key check alone would miss — no OTHER key carrying those
// bytes either: a build that merely renamed the face would still ship
// the 10 MiB this tag exists to leave out.
func TestShippedOmitsTheCJKFaceUnderTheTag(t *testing.T) {
	shipped := Shipped()
	if len(shipped) != 10 {
		t.Fatalf("`-tags nocjkface` fonts.Shipped() returned %d faces, expected ten: %s", len(shipped), strings.Join(sortedKeys(shipped), ", "))
	}
	if _, ok := shipped["Noto Sans SC"]; ok {
		t.Fatal(`"Noto Sans SC" is still in fonts.Shipped() under -tags nocjkface, so the designer's engine still embeds 4.72 MiB brotli of CJK glyphs`)
	}
	for _, key := range sortedKeys(shipped) {
		if len(shipped[key]) > 5_000_000 {
			t.Fatalf("face %q is %d bytes under -tags nocjkface; no face this build should carry is anywhere near that size, so the CJK bytes are still being shipped under another name", key, len(shipped[key]))
		}
	}
}
