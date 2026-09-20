package fontset

// LINE METRICS FOR A FACE A BUILD DECLARES BUT DOES NOT CARRY
// (spec-deferred-offline-cache, CAP-6).
//
// WHY THIS EXISTS, IN ONE MEASUREMENT. The vertical model is a MAXIMUM
// over the line metrics of a chain's PRESENT faces (wrap.go's
// chainLineMetrics feeding verticalModel), so a chain member contributes
// to the height of every line even when it draws not one glyph. Take the
// CJK face out of the designer's engine and a paragraph of ENGLISH on the
// starter's own chain [Roboto, Noto Sans Thai, Noto Sans SC] lays out
// differently from the same document rendered by the CLI, folio-js or
// folio-dotnet: measured, two different PDFs, one byte apart in length.
//
// The face's glyphs are 10,595,932 bytes. Its contribution to that
// arithmetic is THREE INTEGERS. So the designer's engine carries the
// three integers and fetches the glyphs only when a rune must actually be
// DRAWN with them — which is story 4's rune-level TEXT_FACE_ABSENT
// refusal, and which the browser already recovers from. A Latin-only
// session fetches nothing at all, which is this story's own matrix row 1.
//
// ⚠ THE NUMBERS ARE NOT A COMMENT, THEY ARE A TIE. A hand-copied metric
// that drifted from the face by one unit would move every line of every
// CJK-tailed document by a fraction and break byte-identity silently —
// the exact failure class this whole file exists to close. So
// declared_metrics_test.go parses the committed binary and asserts these
// three values against its real LineMetrics(), in the UNTAGGED build
// where the face is present; and folio-go/fonts/accounting_test.go
// independently joins that binary to fonts.Shipped() BY BYTES. Change the
// face, and the tie reds before anything ships.

// notoSansSCLineMetrics is fonts.Shipped()["Noto Sans SC"]'s own
// LineMetrics(), scaled to the 1000-unit em exactly as Font.LineMetrics
// scales them.
//
// ⚠ IT IS DECLARED UNTAGGED, DELIBERATELY, while the LOOKUP below is
// tagged. Twenty-four bytes in every build costs nothing and is read by
// nothing; the tie test, which must run on the ordinary `go test ./...`
// that CI actually executes, needs to see the value. A value behind the
// tag would be checked only by the build that cannot check it.
var notoSansSCLineMetrics = LineMetrics{Ascent: 1160, Descent: -288, LineGap: 0}

// notoSansSCFaceName is the fonts.Shipped() key those metrics belong to,
// spelled once. It is never parsed and never derived from (D-B).
const notoSansSCFaceName = "Noto Sans SC"
