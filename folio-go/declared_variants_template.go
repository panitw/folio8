package folio8

// declaredVariantsTemplateJSON is fixtures/declared-variants/input.folio,
// kept byte-identical to it BY HAND and pinned by
// TestDeclaredVariantsGoldenFixture (line-spacing's precedent, the same
// hand-sync tie font-text, multi-script-fallback, wrapped-text,
// mandatory-break, justified-text, justified-thai, alignment-rounding,
// keep-together and thai-stacked-marks all carry).
//
// IT IS THE FIRST DOCUMENT IN THE PINNED CORPUS THAT DECLARES BOLD OR
// ITALIC AT ALL, and that absence is the hole it exists to close
// (DW-237). Measured at Story 11.5's baseline, twice and by two
// independent mechanisms: `grep -a` for "bold" and for "italic" over
// every file under fixtures/ returned ZERO, and a python3 byte-walk over
// every file in all 29 fixture directories returned ZERO as well — with
// the positive control "fontFamily" returning 23 files, so the
// instrument was live. Story 11.2 resolves a declared cut per rune
// through the DECLARED chain; until this document, the OUTCOME of that
// resolution was pinned by no recorded byte in this repository, so an
// engine that quietly drew bold text in the regular face would have
// moved no golden, reddened no test and raised no diagnostic.
//
// WHAT IT DECLARES. One chain entry, in the object form Story 11.2
// introduced and the shipped starter now writes for an author
// (folio-designer/public/templates/starter.folio):
//
//	{"bold": "Roboto Bold", "boldItalic": "Roboto Bold Italic",
//	 "face": "Roboto", "italic": "Roboto Italic"}
//
// An object-form chain entry raises the saved version — the shared
// predicate FontChainEntry.SerialisesAsObject drives both writeFontChain
// and fontsRequireMajor — so this document declares "2.0", and its own
// fixture test asserts that rather than assuming it.
//
// THE SIX ELEMENTS ARE TWO TESTS, AND BOTH HALVES ARE LOAD-BEARING.
//
//	e1  24pt              "Handgloves — Roboto Regular"
//	e2  24pt bold         "Handgloves — Roboto Bold"
//	e3  24pt italic       "Handgloves — Roboto Italic"
//	e4  24pt bold+italic  "Handgloves — Roboto Bold Italic"
//	e5  24pt align:center "Handgloves quickly"   (regular)
//	e6  24pt align:center "Handgloves quickly"   (bold)
//
// e1–e4 ARE THE HUMAN'S TEST, and each line makes its claim in the face
// it claims. A line that SAYS "Roboto Bold" while LOOKING regular is a
// swap a person sees instantly, so the page witnesses itself.
// "Handgloves" is the type-tester's word because it carries ascender,
// descender, round bowl and tight counters in ten letters.
//
// e5/e6 ARE THE MACHINE'S TEST, and they exist because the obvious
// version was VACUOUS. The first design was a wrap demonstration — the
// same paragraph in the same box, regular against bold, expecting bold
// to take an extra line. It never did: swept across 14 box widths from
// 150 to 340pt, the line counts were equal every single time. Measured
// rather than tuned: centred in a 400pt box the same string starts at
// x=132.548 regular and x=130.724 bold. Centring places a line at
// left + (box - measured)/2, so the origin shows HALF the width
// difference — the 1.824pt offset is a line 3.648pt wider, on a regular
// line measuring 206.904pt. Bold metrics DO reach layout; Roboto's bold
// is simply only ~1.76% wider than its regular, so a break almost never
// moves. The centred pair is the sensitive form of the same assertion:
// any width delta shows.
//
// SOMEONE WILL PROPOSE THE WRAP TEST AGAIN, because it is the obvious
// way to show bold metrics reaching layout. It is recorded here so the
// next reader inherits the measurement instead of repeating the sweep: a
// wrap assertion on this family would be a test that passes for the
// wrong reason.
//
// NO THAI AND NO CJK, DELIBERATELY. A bolded CJK run earns a Warning
// (no cut exists for Noto Sans SC — D-A), and Thai would drag in the
// mark-placement sign-off precedent, muddying what the owner is being
// asked to judge about this page.
//
// THERE IS NO DATA CONST. This document binds nothing: what it witnesses
// is which FACE a declared cut resolves to, and bound data would only
// add a way for the fixture to move for reasons that are not its
// subject.
const declaredVariantsTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "height": 40,
          "id": "e1",
          "style": {
            "fontFamily": "body",
            "fontSize": 24
          },
          "type": "text",
          "value": "Handgloves — Roboto Regular",
          "width": 500,
          "x": 0,
          "y": 0
        },
        {
          "height": 40,
          "id": "e2",
          "style": {
            "bold": true,
            "fontFamily": "body",
            "fontSize": 24
          },
          "type": "text",
          "value": "Handgloves — Roboto Bold",
          "width": 500,
          "x": 0,
          "y": 40
        },
        {
          "height": 40,
          "id": "e3",
          "style": {
            "fontFamily": "body",
            "fontSize": 24,
            "italic": true
          },
          "type": "text",
          "value": "Handgloves — Roboto Italic",
          "width": 500,
          "x": 0,
          "y": 80
        },
        {
          "height": 40,
          "id": "e4",
          "style": {
            "bold": true,
            "fontFamily": "body",
            "fontSize": 24,
            "italic": true
          },
          "type": "text",
          "value": "Handgloves — Roboto Bold Italic",
          "width": 500,
          "x": 0,
          "y": 120
        },
        {
          "height": 40,
          "id": "e5",
          "style": {
            "align": "center",
            "fontFamily": "body",
            "fontSize": 24
          },
          "type": "text",
          "value": "Handgloves quickly",
          "width": 400,
          "x": 0,
          "y": 170
        },
        {
          "height": 40,
          "id": "e6",
          "style": {
            "align": "center",
            "bold": true,
            "fontFamily": "body",
            "fontSize": 24
          },
          "type": "text",
          "value": "Handgloves quickly",
          "width": 400,
          "x": 0,
          "y": 210
        }
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {
    "body": [
      {
        "bold": "Roboto Bold",
        "boldItalic": "Roboto Bold Italic",
        "face": "Roboto",
        "italic": "Roboto Italic"
      }
    ]
  },
  "locale": "en",
  "nextId": 7,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "2.0"
}
`
