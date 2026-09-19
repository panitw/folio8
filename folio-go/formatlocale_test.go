package folio8_test

// This file is AC18 (per-locale evaluation-layer goldens) and AC19
// (end-to-end renders through the public Render API, producing real
// PDF bytes, for th and ja). It exercises the shipped font set
// (fonts.Shipped()) — never a synthetic test face — because AC19's
// `ja` assertion specifically needs the measured cmap coverage this
// story's own findings recorded (D-3.4.1).
//
// Finding 2's blocker was that the golden loop discarded Render's
// result and compared the test's own wantDate constant to itself. The
// fix below decodes what Render ACTUALLY drew, through the rendered
// document's own /ToUnicode CMap.
//
// WHY THIS FILE CARRIES ITS OWN SMALL PDF-OBJECT/CMap PARSER RATHER
// THAN REUSING multi_page_fixture_test.go's mpParseToUnicode/
// mpExtractRuns or shaped_fixture_test.go's pdfObjects/streamBody:
// those are unexported and live in `package folio8` (white-box), and
// this file must stay in `package folio8_test` (black-box) because it
// needs `fonts.Shipped()` — the `fonts` package itself imports `folio8`
// (fonts/fonts.go), so a `package folio8` test file importing `fonts`
// is an import cycle (verified: `go test` refuses to build one).
// ac4_coverage_test.go and example_test.go are `folio8_test` for the
// identical reason. This is the same shape multi_page_fixture_test.go
// documents for its own local mpKidsArrayRE/mpContentsRefRE ("this file
// carries no matrix build tag, so it cannot reference matrix_test.go's
// symbols") and three_band_page_fixture_test.go's own separate
// parseToUnicodeCMap: this codebase already carries more than one
// small, package-scoped PDF-structure parser for exactly this reason,
// rather than fighting the package graph to share one.

import (
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// formatLocaleTemplateJSON parameterises locale/utcOffset/fonts for one
// text element binding both formatDate and formatNumber — the smallest
// document that exercises AC18/AC19 through the real Render/ParseTemplate
// path, rather than calling internal/expr directly.
func formatLocaleTemplateJSON(locale, utcOffset, fontsChain, pattern string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{formatDate(when, \"` + pattern + `\")}} {{formatNumber(amount, \"#,##0.00\")}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": [` + fontsChain + `]},
  "locale": "` + locale + `",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "` + utcOffset + `",
  "version": "1.0"
}
`
}

// flPdfObjects splits a classic (uncompressed) PDF into object number ->
// object body — the same technique shaped_fixture_test.go's pdfObjects
// uses (re-derived here rather than imported: see this file's top
// comment for why).
func flPdfObjects(t *testing.T, b []byte) map[int][]byte {
	t.Helper()
	objs := map[int][]byte{}
	rest := b
	for {
		idx := bytes.Index(rest, []byte(" 0 obj\n"))
		if idx == -1 {
			break
		}
		start := idx
		for start > 0 && rest[start-1] >= '0' && rest[start-1] <= '9' {
			start--
		}
		num, err := strconv.Atoi(string(rest[start:idx]))
		if err != nil {
			rest = rest[idx+7:]
			continue
		}
		bodyStart := idx + len(" 0 obj\n")
		end := bytes.Index(rest[bodyStart:], []byte("\nendobj\n"))
		if end == -1 {
			break
		}
		objs[num] = rest[bodyStart : bodyStart+end]
		rest = rest[bodyStart+end:]
	}
	if len(objs) == 0 {
		t.Fatal("could not parse any PDF objects out of the rendered document")
	}
	return objs
}

// flStreamBody returns the bytes between "stream\n" and "endstream" in
// an object body, and whether the object has one at all. folio8 never
// compresses a content stream (D-2.0.4), so this is readable as written.
func flStreamBody(obj []byte) ([]byte, bool) {
	i := bytes.Index(obj, []byte(">>\nstream\n"))
	if i == -1 {
		return nil, false
	}
	start := i + len(">>\nstream\n")
	j := bytes.Index(obj[start:], []byte("endstream"))
	if j == -1 {
		return nil, false
	}
	return obj[start : start+j], true
}

var (
	flKidsArrayRE   = regexp.MustCompile(`/Type /Pages /Kids \[([^\]]*)\] /Count (\d+)`)
	flContentsRefRE = regexp.MustCompile(`/Contents (\d+) 0 R`)
)

// flSplitPageContentStreams returns each page's content stream, in the
// page tree's declared /Kids order — it follows references, it does not
// scan bytes for text (the same reasoning multi_page_fixture_test.go's
// splitPageContentStreams records).
func flSplitPageContentStreams(t *testing.T, b []byte) []string {
	t.Helper()
	m := flKidsArrayRE.FindSubmatch(b)
	if m == nil {
		t.Fatal("presence precondition: no page tree of the form \"/Type /Pages /Kids [...] /Count N\" found")
	}
	fields := bytes.Fields(m[1])
	if len(fields)%3 != 0 {
		t.Fatalf("/Kids array %q does not tokenize into whole \"N 0 R\" triples", m[1])
	}
	objs := flPdfObjects(t, b)
	var out []string
	for i := 0; i+2 < len(fields); i += 3 {
		num, err := strconv.Atoi(string(fields[i]))
		if err != nil {
			t.Fatalf("unparseable page object number %q in /Kids", fields[i])
		}
		pageBody, ok := objs[num]
		if !ok {
			t.Fatalf("the page tree references object %d, which is never defined in the file", num)
		}
		cm := flContentsRefRE.FindSubmatch(pageBody)
		if cm == nil {
			t.Fatalf("page object %d carries no /Contents reference", num)
		}
		contentNum, err := strconv.Atoi(string(cm[1]))
		if err != nil {
			t.Fatalf("unparseable /Contents object number %q for page %d", cm[1], num)
		}
		contentBody, ok := objs[contentNum]
		if !ok {
			t.Fatalf("page %d's /Contents references object %d, which is never defined in the file", num, contentNum)
		}
		s, ok := flStreamBody(contentBody)
		if !ok {
			t.Fatalf("object %d (page %d's /Contents) carries no stream body", contentNum, num)
		}
		out = append(out, string(s))
	}
	if len(out) == 0 {
		t.Fatal("presence precondition: the document carries no pages — every per-page assertion below would iterate over nothing")
	}
	return out
}

// flParseHex16 parses a 4-hex-digit string into a uint16.
func flParseHex16(s string) (uint16, bool) {
	if len(s) != 4 {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}

// flObjRefAfter returns the object number of an indirect reference
// following key in obj (e.g. "/ToUnicode 7 0 R").
func flObjRefAfter(obj []byte, key string) (int, bool) {
	i := bytes.Index(obj, []byte(key))
	if i == -1 {
		return 0, false
	}
	after := obj[i+len(key):]
	sp := bytes.Index(after, []byte(" 0 R"))
	if sp == -1 {
		return 0, false
	}
	num, err := strconv.Atoi(strings.TrimSpace(string(after[:sp])))
	if err != nil {
		return 0, false
	}
	return num, true
}

// flResourceType0Objects maps each PDF font RESOURCE NAME (as it appears
// on a page's /Font resource dictionary) to its Type0 font object body —
// read off EVERY page object, not an arbitrary one.
func flResourceType0Objects(t *testing.T, objs map[int][]byte) map[string][]byte {
	t.Helper()
	nums := make([]int, 0, len(objs))
	for num := range objs {
		nums = append(nums, num)
	}
	sort.Ints(nums)

	var fontDicts [][]byte
	for _, num := range nums {
		obj := objs[num]
		if !bytes.Contains(obj, []byte("/Type /Page ")) {
			continue
		}
		i := bytes.Index(obj, []byte("/Font <<"))
		if i == -1 {
			continue
		}
		after := obj[i+len("/Font <<"):]
		j := bytes.Index(after, []byte(">>"))
		if j == -1 {
			t.Fatal("page's /Font resource dictionary is unterminated")
		}
		fontDicts = append(fontDicts, after[:j])
	}
	if len(fontDicts) == 0 {
		t.Fatal("produced PDF carries no page /Font resource dictionary — every assertion about fonts below would be vacuous")
	}

	out := map[string][]byte{}
	for _, fontDict := range fontDicts {
		fields := strings.Fields(string(fontDict))
		for i := 0; i < len(fields); i++ {
			if !strings.HasPrefix(fields[i], "/") {
				continue
			}
			if i+3 >= len(fields) || fields[i+3] != "R" {
				continue
			}
			name := strings.TrimPrefix(fields[i], "/")
			num, err := strconv.Atoi(fields[i+1])
			if err != nil {
				continue
			}
			type0, ok := objs[num]
			if !ok {
				t.Fatalf("resource /%s names object %d, which is not present", name, num)
			}
			out[name] = type0
		}
	}
	if len(out) == 0 {
		t.Fatal("no font resources resolved to a Type0 font object")
	}
	return out
}

// flParseBfCharMap reads a ToUnicode CMap's bfchar entries into
// CID -> extracted text. An empty destination string (<>) is a REAL
// entry meaning "this glyph contributes no text".
func flParseBfCharMap(t *testing.T, label string, stream []byte) map[uint16]string {
	t.Helper()
	i := bytes.Index(stream, []byte("beginbfchar\n"))
	if i == -1 {
		t.Fatalf("%s: /ToUnicode stream has no beginbfchar section", label)
	}
	j := bytes.Index(stream, []byte("endbfchar"))
	if j == -1 {
		t.Fatalf("%s: /ToUnicode stream has no endbfchar", label)
	}
	out := map[uint16]string{}
	for _, line := range strings.Split(string(stream[i+len("beginbfchar\n"):j]), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("%s: unparseable bfchar line %q", label, line)
		}
		cid, ok := flParseHex16(strings.Trim(parts[0], "<>"))
		if !ok {
			t.Fatalf("%s: unparseable bfchar CID %q", label, parts[0])
		}
		hexText := strings.Trim(parts[1], "<>")
		var sb strings.Builder
		for k := 0; k+4 <= len(hexText); k += 4 {
			cp, ok := flParseHex16(hexText[k : k+4])
			if !ok {
				t.Fatalf("%s: unparseable bfchar destination %q", label, parts[1])
			}
			sb.WriteRune(rune(cp))
		}
		out[cid] = sb.String()
	}
	return out
}

// flToUnicodeForResources maps each PDF font RESOURCE NAME to its OWN
// /ToUnicode CID -> text map. CID numbering is PER FONT, not
// document-global: an earlier version of this file built one combined
// map[uint16]string across every /ToUnicode stream in the document and
// silently conflated NotoSans's and NotoSansThai's independently
// numbered CID spaces, producing a scrambled reconstruction of the `th`
// golden's text (measured: CIDs that mean "1", "5", " " in NotoSans's
// own table were read back as unrelated Thai characters belonging to
// NotoSansThai's CID 1/4/5/8, because both faces number their glyphs
// from a low integer). Resolving the resource name -> Type0 font ->
// /ToUnicode object chain per BT/ET block (flExtractRuns, below) is what
// this file's earlier version skipped.
func flToUnicodeForResources(t *testing.T, b []byte) map[string]map[uint16]string {
	t.Helper()
	objs := flPdfObjects(t, b)
	type0s := flResourceType0Objects(t, objs)

	out := map[string]map[uint16]string{}
	for name, type0 := range type0s {
		cmapNum, ok := flObjRefAfter(type0, "/ToUnicode ")
		if !ok {
			t.Fatalf("resource /%s's Type0 font declares no /ToUnicode — its text is not extractable", name)
		}
		cmapObj, ok := objs[cmapNum]
		if !ok {
			t.Fatalf("resource /%s's /ToUnicode names object %d, which is not present", name, cmapNum)
		}
		stream, ok := flStreamBody(cmapObj)
		if !ok {
			t.Fatalf("resource /%s's /ToUnicode object %d carries no stream", name, cmapNum)
		}
		out[name] = flParseBfCharMap(t, name, stream)
	}
	if len(out) == 0 {
		t.Fatal("no font resources resolved to a /ToUnicode CMap")
	}
	return out
}

// flExtractRuns decodes one content stream's Tj/TJ hex runs back to
// text, in emission order, resolving each BT/ET block's OWN font
// resource (its "/Name size Tf" line) to that resource's OWN
// /ToUnicode map via cidMaps — never a single document-wide CID space
// (see flToUnicodeForResources).
func flExtractRuns(t *testing.T, stream string, cidMaps map[string]map[uint16]string) []string {
	t.Helper()
	var out []string
	for _, block := range strings.Split(stream, "BT\n")[1:] {
		end := strings.Index(block, "ET\n")
		if end == -1 {
			continue
		}
		body := block[:end]

		tf := strings.Index(body, " Tf\n")
		if tf == -1 {
			t.Fatalf("BT block selects no font: %q", body)
		}
		resource := strings.TrimPrefix(strings.Fields(body[:tf])[0], "/")
		cidToText, ok := cidMaps[resource]
		if !ok {
			t.Fatalf("BT block selects font resource /%s, which has no resolved /ToUnicode map", resource)
		}

		var sb strings.Builder
		sawText := false
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			isTJ := strings.HasPrefix(line, "[") && strings.HasSuffix(line, "] TJ")
			isTj := strings.HasPrefix(line, "<") && strings.HasSuffix(line, "> Tj")
			if !isTJ && !isTj {
				continue
			}
			sawText = true
			rest := line
			for {
				open := strings.Index(rest, "<")
				if open == -1 {
					break
				}
				shut := strings.Index(rest[open:], ">")
				if shut == -1 {
					break
				}
				hexRun := rest[open+1 : open+shut]
				for i := 0; i+4 <= len(hexRun); i += 4 {
					cid, ok := flParseHex16(hexRun[i : i+4])
					if !ok {
						t.Fatalf("content stream carries a malformed CID %q", hexRun[i:i+4])
					}
					text, known := cidToText[cid]
					if !known {
						t.Fatalf("content stream emits CID %d for resource /%s, which its /ToUnicode CMap does not map — the text is unextractable", cid, resource)
					}
					sb.WriteString(text)
				}
				rest = rest[open+shut:]
			}
		}
		if sawText {
			out = append(out, sb.String())
		}
	}
	if len(out) == 0 {
		t.Fatal("presence precondition: no text-showing BT/ET block found in this page's content stream")
	}
	return out
}

// formatLocaleExtractedText decodes the ONE page a formatLocaleTemplateJSON
// document renders to back into the text it actually draws — reading the
// artifact Render actually produced, not a constant the test already
// knew (Finding 2 / D-000.21).
func formatLocaleExtractedText(t *testing.T, pdfBytes []byte) string {
	t.Helper()
	streams := flSplitPageContentStreams(t, pdfBytes)
	if len(streams) != 1 {
		t.Fatalf("formatLocaleTemplateJSON's document must render to exactly one page, got %d", len(streams))
	}
	cidMaps := flToUnicodeForResources(t, pdfBytes)
	var sb strings.Builder
	for _, run := range flExtractRuns(t, streams[0], cidMaps) {
		sb.WriteString(run)
	}
	return sb.String()
}

// TestFormatLocaleGoldensAllFourLocales is AC18: one golden per
// locale, at the evaluation layer (through BindText via the public
// Render API), covering both formatDate and formatNumber, pinned as
// recorded expected STRINGS — compared against what Render's own PDF
// bytes decode to, never against the test's own constant (Finding 2).
// Non-vacuity: the four goldens differ from each other in at least the
// era, the month name and the date shape — `ja` and `zh-Hans`
// legitimately agree on numbers and on the "年月日" date shape, and that
// agreement is stated here explicitly rather than silently read as
// coverage.
//
// The `ja` case's SCOPE FENCE (AC18): this golden covers date and
// number FORMATTING ONLY, never typography. It is mechanically
// verifiable by any reader (it decodes the PDF's own /ToUnicode CMap,
// above) and no claim about Japanese typography is made or asserted
// anywhere in it — the Simplified-Chinese kanji shapes a Japanese
// reader would see are AC21's documented, written limitation, not
// something this test could or does check.
func TestFormatLocaleGoldensAllFourLocales(t *testing.T) {
	// The instant is 2026-08-15T00:00:00, in each document's own
	// declared offset — chosen so the CIVIL DATE is 15 August 2026 in
	// every one of these offsets (F7's published outputs).
	cases := []struct {
		name       string
		locale     string
		utcOffset  string
		fontsChain string
		pattern    string
		when       string
		wantDate   string
	}{
		{
			name: "en", locale: "en", utcOffset: "+00:00",
			fontsChain: `"Noto Sans"`, pattern: `d MMMM yyyy`,
			when: "2026-08-15T00:00:00Z", wantDate: "15 August 2026",
		},
		{
			name: "th", locale: "th", utcOffset: "+07:00",
			fontsChain: `"Noto Sans", "Noto Sans Thai"`, pattern: `d MMMM yyyy`,
			when: "2026-08-15T00:00:00+07:00", wantDate: "15 สิงหาคม 2569",
		},
		{
			name: "zh-Hans", locale: "zh-Hans", utcOffset: "+08:00",
			fontsChain: `"Noto Sans SC"`, pattern: `yyyy年M月d日`,
			when: "2026-08-15T00:00:00+08:00", wantDate: "2026年8月15日",
		},
		{
			// Finding 16 (this story's QA review): AC18/D-3.4.1 name the
			// RULED `ja` literal as `2026年8月26日` (the instant this
			// story's other examples use). This golden deliberately
			// keeps 2026-08-15 instead, the SAME shared instant every
			// other case above uses, so the four-locale civil-date
			// comparison (F7: "the civil date is 15 August 2026 in
			// every one of these offsets") and the ja==zh-Hans
			// non-vacuity check below stay meaningful across all four
			// cases at once. The exact ruled literal `2026年8月26日` IS
			// pinned and asserted — at
			// internal/expr/formatdate_test.go:235,
			// TestFormatDateJapaneseGoldenFormattingOnly — this golden
			// is additional coverage at the Render/PDF layer, not a
			// second, competing source of truth for that literal.
			name: "ja", locale: "ja", utcOffset: "+09:00",
			fontsChain: `"Noto Sans SC"`, pattern: `yyyy年M月d日`,
			when: "2026-08-15T00:00:00+09:00", wantDate: "2026年8月15日",
		},
	}

	got := map[string]string{}
	for _, c := range cases {
		tplJSON := formatLocaleTemplateJSON(c.locale, c.utcOffset, c.fontsChain, c.pattern)
		tpl, err := folio8.ParseTemplate([]byte(tplJSON))
		if err != nil {
			t.Fatalf("%s: ParseTemplate: %v", c.name, err)
		}
		data := folio8.Data(`{"when": "` + c.when + `", "amount": 1234.56}`)
		res, err := folio8.Render(tpl, data, folio8.Params(`{}`), fonts.Shipped())
		if err != nil {
			t.Fatalf("%s: Render: %v", c.name, err)
		}

		want := c.wantDate + " 1,234.56"
		gotText := formatLocaleExtractedText(t, res.Bytes)
		if gotText != want {
			t.Errorf("%s: rendered text decoded from the PDF = %q, want %q", c.name, gotText, want)
		}
		got[c.name] = gotText
	}

	// Non-vacuity: at least the era, month name and date shape differ
	// between en/th/zh-Hans; ja legitimately equals zh-Hans on both
	// (D-3.4.1), stated here rather than silently passing as coverage.
	if got["en"] == got["th"] || got["en"] == got["zh-Hans"] || got["th"] == got["zh-Hans"] {
		t.Fatalf("expected en/th/zh-Hans date goldens to differ, got %v", got)
	}
	if got["ja"] != got["zh-Hans"] {
		t.Fatalf("ja and zh-Hans are expected to agree on formatting (D-3.4.1), got ja=%q zh-Hans=%q", got["ja"], got["zh-Hans"])
	}
	t.Logf("goldens (decoded from the rendered PDF's own /ToUnicode CMap): %v", got)
}

// TestFormatLocaleEndToEndRenderThai is AC19: the `th` end-to-end
// render through the public Render API, producing real PDF bytes, and
// the rendered text is decoded back from those bytes and compared
// against the expected string (Finding 2: a font-resource-name Contains
// check alone cannot discriminate a broken formatter from a correct
// one).
func TestFormatLocaleEndToEndRenderThai(t *testing.T) {
	tplJSON := formatLocaleTemplateJSON("th", "+07:00", `"Noto Sans", "Noto Sans Thai"`, `d MMMM yyyy`)
	tpl, err := folio8.ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := folio8.Data(`{"when": "2026-08-15T00:00:00+07:00", "amount": 1234.56}`)
	res, err := folio8.Render(tpl, data, folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render produced no bytes")
	}
	if !strings.Contains(string(res.Bytes), "NotoSansThai") {
		t.Error("rendered PDF does not reference the Thai face as a resource — the render did not actually exercise Thai glyphs")
	}

	want := "15 สิงหาคม 2569 1,234.56"
	if gotText := formatLocaleExtractedText(t, res.Bytes); gotText != want {
		t.Errorf("rendered text decoded from the PDF = %q, want %q", gotText, want)
	}
}

// TestFormatLocaleEndToEndRenderJapanese is AC19's `ja` render: the
// document RENDERS with the shipped font set — complete, every glyph
// resolved (the measured cmap coverage this story's findings recorded).
// It decodes the rendered text back from the PDF's own /ToUnicode CMap
// and compares it against the expected date/number string (Finding 2:
// the previous `strings.Contains(res.Bytes, "NotoSansSC")` check cannot
// discriminate a broken formatter, because every ASCII rune in this
// element already pulls that face regardless of what formatDate
// returns). This test still asserts NOTHING about glyph SHAPE — the
// Simplified-Chinese kanji forms a Japanese reader would see are AC21's
// documented limitation, not a defect this test may report, and no
// runtime glyph-coverage diagnostic is added here or anywhere
// (D-000.15's false-positive hazard).
//
// Bound, stated explicitly rather than left implied (D-000.24): this
// test cannot and does not confirm that the correct glyph OUTLINES are
// drawn in the PDF's content stream — only that the correct CODE POINTS
// are, via the font's declared cmap/ToUnicode tables. Confirming actual
// glyph-outline drawing would require decoding the embedded font
// program's glyf/CFF table against the shaped glyph IDs, which is
// beyond this story's scope.
func TestFormatLocaleEndToEndRenderJapanese(t *testing.T) {
	tplJSON := formatLocaleTemplateJSON("ja", "+09:00", `"Noto Sans SC"`, `yyyy年M月d日`)
	tpl, err := folio8.ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := folio8.Data(`{"when": "2026-08-15T00:00:00+09:00", "amount": 1234.56}`)
	res, err := folio8.Render(tpl, data, folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render produced no bytes")
	}
	if !strings.Contains(string(res.Bytes), "NotoSansSC") {
		t.Error("rendered PDF does not reference the CJK face as a resource — the render did not actually exercise the ja text")
	}

	want := "2026年8月15日 1,234.56"
	if gotText := formatLocaleExtractedText(t, res.Bytes); gotText != want {
		t.Errorf("rendered text decoded from the PDF = %q, want %q", gotText, want)
	}
}
