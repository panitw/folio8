package folio8

import (
	"encoding/json"
	"fmt"
	"testing"
)

// segmentOriginCase is one hand-derived statement of AC8's positioning
// half: an element whose text splits into exactly two face-segments,
// and the exact distance the second segment's origin must sit from the
// first's.
//
// Both numbers are DERIVED, not captured. wantDeltaMilli is the shaped
// advance of the first segment; naiveDeltaMilli is the sum of its runes'
// raw `hmtx` advances — the pre-fix answer. Provenance for every value
// below, cited per D-000.26 as the SUBJECT of the measurement and not
// only its result:
//
//	face      folio-go/fonts/notosans/NotoSans-Regular.ttf, unitsPerEm 1000
//	          (so a font-unit advance IS a 1000-em advance; no scaling
//	          step hides between the oracle and these numbers)
//	oracle    hb-shape (HarfBuzz) 14.2.0, --output-format=json
//	fontSize  16 pt, i.e. milli = units1000 * 16
//
//	"AV "   shaped  ax 599 + 600 + 260 = 1459  ->  1459*16 = 23344
//	        hmtx     A 639 + 600 + 260 = 1499  ->  1499*16 = 23984  (A/V kerns -40)
//	"Wo. "  shaped  ax 910 + 605 + 268 + 260 = 2043  ->  32688
//	        hmtx     W 930 + 605 + 268 + 260 = 2063  ->  33008  (W/o kerns -20)
//	"Ada "  shaped  ax 639 + 615 + 561 + 260 = 2075  ->  33200
//	        hmtx    identical — no kern pair. THE NEGATIVE CONTROL.
type segmentOriginCase struct {
	text            string
	firstSegment    string
	wantDeltaMilli  int64
	naiveDeltaMilli int64
	note            string
}

// TestFaceSegmentOriginsUseShapedAdvances is Story 2.3 finisher's
// Blocker 1 guard, and it is deliberately NOT written against
// positionSegments' own cursor arithmetic.
//
// The defect it exists to catch was that a face-segment's text was DRAWN
// kerned (appendShapedRun emits a trailing TJ adjustment so the run's
// total advance is exactly the shaped advance) while the NEXT segment's
// origin was computed from a sum of raw `hmtx` advances. The two
// derivations agreed before this story, because nothing was kerned, and
// diverged silently the moment kerning landed. A test that recomputed
// the origin the way production computes it would agree with any
// arithmetic production chose, including the wrong one — so the expected
// values here are literals, hand-derived from HarfBuzz above, and the
// actual values are read off the PRODUCED PDF's text matrices.
//
// The negative control matters as much as the positive cases: "Ada ก"
// must place at 33200 under BOTH derivations, so a test that simply
// rejected the old numbers would still have to accept this one.
func TestFaceSegmentOriginsUseShapedAdvances(t *testing.T) {
	cases := []segmentOriginCase{
		{
			text:            "AV ก",
			firstSegment:    "AV ",
			wantDeltaMilli:  23344,
			naiveDeltaMilli: 23984,
			note:            "the A/V kern pair, -40 units at 1000-em, 0.64 pt at 16 pt",
		},
		{
			text:            "Wo. ก",
			firstSegment:    "Wo. ",
			wantDeltaMilli:  32688,
			naiveDeltaMilli: 33008,
			note:            "the W/o kern pair, -20 units at 1000-em, 0.32 pt at 16 pt",
		},
		{
			text:            "Ada ก",
			firstSegment:    "Ada ",
			wantDeltaMilli:  33200,
			naiveDeltaMilli: 33200,
			note:            "NEGATIVE CONTROL: no kern pair, so both derivations agree and this must not move",
		},
	}

	checked := 0
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(oneElementShapedTemplate(tc.text)))
			if err != nil {
				t.Fatalf("parse template for %q: %v", tc.text, err)
			}
			res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
			if err != nil {
				t.Fatalf("render %q: %v", tc.text, err)
			}
			b := res.Bytes

			runs := readEmittedRuns(t, b)
			// D-000.21 sharpened: prove the artifact carries the thing
			// being read before reading it. Two face-segments is the
			// whole premise — one segment would make the assertion
			// vacuously unreachable rather than false.
			if len(runs) != 2 {
				t.Fatalf(
					"%q emitted %d text runs, want exactly 2 (a Latin segment followed by a Thai one); "+
						"with one run there is no segment boundary to place and this test asserts nothing",
					tc.text, len(runs),
				)
			}
			if runs[0].Resource == runs[1].Resource {
				t.Fatalf(
					"%q emitted both runs against font resource %q, so the fallback chain did not split it "+
						"across two faces and no cross-face cursor was exercised",
					tc.text, runs[0].Resource,
				)
			}

			got := runs[1].OriginXMilli - runs[0].OriginXMilli
			if got != tc.wantDeltaMilli {
				extra := ""
				if got == tc.naiveDeltaMilli && tc.naiveDeltaMilli != tc.wantDeltaMilli {
					extra = fmt.Sprintf(
						"\n  this is EXACTLY the unkerned `hmtx` sum (%d millipoints): the segment cursor has "+
							"regressed to fontset.AdvanceForRune, so %q is drawn kerned and placed unkerned",
						tc.naiveDeltaMilli, tc.firstSegment,
					)
				}
				t.Errorf(
					"%q: segment 2 sits %d millipoints after segment 1, want %d\n"+
						"  segment 1 is %q; its SHAPED advance is the only correct cursor (%s)%s",
					tc.text, got, tc.wantDeltaMilli, tc.firstSegment, tc.note, extra,
				)
			}
			checked++
		})
	}

	if checked != len(cases) {
		t.Fatalf("vacuity: %d of %d cases actually reached the origin assertion", checked, len(cases))
	}
	t.Logf("%d of %d two-face elements evaluated, origins compared against hand-derived hb-shape 14.2.0 advances", checked, len(cases))
}

// jsonString renders one Go string as a JSON string literal, so the
// template above is built without hand-escaping.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// oneElementShapedTemplate builds a single-element document over the
// shipped three-face chain at 16 pt — the same chain and size
// fixtures/shaped-text/input.folio uses, so the numbers in
// segmentOriginCase describe the same conditions the golden records.
func oneElementShapedTemplate(value string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 24, "value": ` + jsonString(value) + `, "style": {"fontFamily": "body", "fontSize": 16}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {
    "body": ["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]
  },
  "locale": "th",
  "nextId": 2,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}
