package folio8

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// --- Story 7.7's discriminating fixture (FR51) ----------------------------
//
// Every assertion below is about the DIFFERENCE between two documents that
// differ in exactly one respect — three `keepTogether` tags — because
// "the block is on page 2" is a fact any number of unrelated
// implementations produce, and "the block is on page 2 HERE and split
// THERE" is not.

// keepTogetherPages renders one of the pair and returns its page model,
// insisting the document builds cleanly.
func keepTogetherPages(t *testing.T, templateJSON string) []pageProbe {
	t.Helper()
	pages, diags := keepTogetherPagesWithDiagnostics(t, templateJSON)
	if len(diags) != 0 {
		t.Fatalf("the keep-together pair must build with NO diagnostics; got %+v", diags)
	}
	return pages
}

// keepTogetherPagesWithDiagnostics is the same probe for a document that
// is SUPPOSED to say something — the over-tall cases, which return a
// Warning beside their bytes and would otherwise be unprobeable here.
func keepTogetherPagesWithDiagnostics(t *testing.T, templateJSON string) ([]pageProbe, []Diagnostic) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(templateJSON))
	if err != nil {
		t.Fatalf("parse keep-together template: %v", err)
	}
	pages, _, _, diags, err := buildPageModel(tpl, mustDecodeData(t, keepTogetherDataJSON), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("build keep-together page model: %v", err)
	}
	out := make([]pageProbe, 0, len(pages))
	for _, p := range pages {
		probe := pageProbe{rects: len(p.Rects), images: len(p.Images)}
		for _, r := range p.Runs {
			probe.text += r.SourceText
		}
		out = append(out, probe)
	}
	return out, diags
}

// pageProbe is one page reduced to what these assertions are about: the
// text it carries, how many rectangles it draws and how many images it
// places. The signature block's members are a text, a rect, an image and
// a second text, so all three fields are load-bearing — a check that
// looked only at runs could see neither the ruled line nor a signature
// image move.
type pageProbe struct {
	text   string
	rects  int
	images int
}

// The three signature members, named by what a reader would look for on
// the page. e3, the ruled line, has no text at all and is counted
// through the page's rectangles instead.
const (
	keepTogetherSignatureText = "Signed for the Company"
	keepTogetherDateText      = "Date: 31 August 2026"
)

// TestKeepTogetherTwinDiffersOnlyByTheTags is the precondition every
// other assertion in this file rests on. If the pair ever drifted into
// differing for a second reason, "the two renders differ" would stop
// being evidence about grouping.
func TestKeepTogetherTwinDiffersOnlyByTheTags(t *testing.T) {
	const tag = `"keepTogether": "signature", `
	if n := strings.Count(keepTogetherTemplateJSON, tag); n != 3 {
		t.Fatalf("the grouped template must carry exactly three tags, found %d — the fixture is a three-element signature block", n)
	}
	if strings.Contains(keepTogetherUngroupedTemplateJSON, "keepTogether") {
		t.Fatal("the ungrouped twin must carry no keepTogether tag at all")
	}
	if stripped := strings.ReplaceAll(keepTogetherTemplateJSON, tag, ""); stripped != keepTogetherUngroupedTemplateJSON {
		t.Fatalf("the twin is not the grouped template minus its tags — the pair differs for some SECOND reason, so it no longer discriminates:\n%s", stripped)
	}
}

// TestKeepTogetherMovesTheWholeBlock is AC1 and AC5 together: grouped,
// every member is on page 2 and page 1 carries none of them; ungrouped,
// the SAME document splits. Removing the tags must make the grouped
// assertion fail — which is exactly what the second half measures.
func TestKeepTogetherMovesTheWholeBlock(t *testing.T) {
	grouped := keepTogetherPages(t, keepTogetherTemplateJSON)
	if len(grouped) != 2 {
		t.Fatalf("the grouped document must paginate to 2 pages, got %d", len(grouped))
	}
	if strings.Contains(grouped[0].text, keepTogetherSignatureText) ||
		strings.Contains(grouped[0].text, keepTogetherDateText) ||
		grouped[0].rects != 0 {
		t.Fatalf("page 1 must carry NO member of the signature block, got text containing the block and %d rect(s)", grouped[0].rects)
	}
	if !strings.Contains(grouped[1].text, keepTogetherSignatureText) {
		t.Error("page 2 must carry the signature line")
	}
	if !strings.Contains(grouped[1].text, keepTogetherDateText) {
		t.Error("page 2 must carry the date line")
	}
	if grouped[1].rects != 1 {
		t.Errorf("page 2 must carry the ruled line's rectangle, got %d rect(s)", grouped[1].rects)
	}
	// The body is UNTOUCHED: no sibling moved (AC1). Page 1 still ends
	// with the body's own last words.
	if !strings.Contains(grouped[0].text, "and the same instrument.") {
		t.Error("the body text must stay on page 1 — honouring a group moves the group, never a sibling")
	}

	ungrouped := keepTogetherPages(t, keepTogetherUngroupedTemplateJSON)
	if len(ungrouped) != 2 {
		t.Fatalf("the ungrouped twin must paginate to 2 pages, got %d", len(ungrouped))
	}
	if !strings.Contains(ungrouped[0].text, keepTogetherSignatureText) {
		t.Fatal("WITHOUT the tags the signature line must be STRANDED on page 1 — if it is not, this fixture no longer discriminates and every other assertion here is vacuous")
	}
	if strings.Contains(ungrouped[0].text, keepTogetherDateText) || ungrouped[0].rects != 0 {
		t.Fatal("without the tags the rule and the date must fall to page 2 — the split is what the grouped render is being compared against")
	}
	if !strings.Contains(ungrouped[1].text, keepTogetherDateText) || ungrouped[1].rects != 1 {
		t.Fatal("without the tags page 2 must carry the rule and the date")
	}
}

// TestKeepTogetherChangesTheRenderedBytes is the difference at the
// PUBLIC boundary, which is where AC5 actually lives: the same
// document with and without its tags produces different documents.
func TestKeepTogetherChangesTheRenderedBytes(t *testing.T) {
	grouped := renderKeepTogether(t)
	tpl, err := ParseTemplate([]byte(keepTogetherUngroupedTemplateJSON))
	if err != nil {
		t.Fatalf("parse ungrouped twin: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render ungrouped twin: %v", err)
	}
	if len(grouped) == 0 || len(res.Bytes) == 0 {
		t.Fatal("both renders must produce bytes")
	}
	if sha256Hex(grouped) == sha256Hex(res.Bytes) {
		t.Fatal("the grouped and ungrouped renders hash identically — the tags changed nothing, so this fixture proves nothing")
	}
}

// keepTogetherNonContiguousTemplate is D-7.7.1's no-contiguity case,
// which is the property that lets the shipped machinery carry this
// feature at all: paginate.go records R7's contiguity premise as
// MEASURED FALSE and removed, so two tagged elements separated in column
// order by an intervening UNTAGGED one are still one group.
//
// It is built so the intervening element does the OPPOSITE of riding
// along, which is what makes it discriminating rather than merely
// present:
//
//	e2  tagged, y 690  -> 690.000 .. 704.982
//	e3  UNTAGGED rect, y 700, height 40 -> 700.000 .. 740.000   OUT
//	e4  tagged, y 710  -> 710.000 .. 724.982
//
// The group's union extent is 690.000 .. 724.982, which FITS window one
// (729.890), so the group's page is decided as page 1 at e2. e3 then
// fails the fit test on its own account and advances the window to
// page 2. e4 is visited LAST, with the window already on page 2 — and it
// lands on page 1 anyway, because its group's page was resolved at e2
// and every later-visited member rides along without asking the window a
// second question.
//
// So: an intervening item advances the window FOR ITSELF and can never
// split the group. Without the tags, e4 follows the window to page 2.
const keepTogetherNonContiguousTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 523, "height": 40, "value": "Body", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 690, "width": 240, "height": 16, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "rect", "x": 300, "y": 700, "width": 200, "height": 40, "style": {"background": "#000000"}},
        {"id": "e4", "type": "text", "x": 0, "y": 710, "width": 240, "height": 16, "keepTogether": "signature", "value": "Date: 31 August 2026", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherMembersNeedNotBeContiguous asserts both halves of the
// property, and the second half against the same document with its tags
// removed, so neither half can pass by accident.
func TestKeepTogetherMembersNeedNotBeContiguous(t *testing.T) {
	pages := keepTogetherPages(t, keepTogetherNonContiguousTemplate)
	if len(pages) != 2 {
		t.Fatalf("want 2 pages, got %d", len(pages))
	}
	if !strings.Contains(pages[0].text, keepTogetherSignatureText) {
		t.Error("the group fits window one, so its first member belongs on page 1")
	}
	if !strings.Contains(pages[0].text, keepTogetherDateText) {
		t.Error("the LAST-visited member must ride along to its group's page even though the window has already advanced past it — that ride-along IS the no-contiguity property")
	}
	if pages[0].rects != 0 || pages[1].rects != 1 {
		t.Errorf("the intervening UNTAGGED rect must advance the window for itself and land on page 2; got %d rect(s) on page 1 and %d on page 2", pages[0].rects, pages[1].rects)
	}

	// The control, without which the assertions above would hold for a
	// build that simply never advanced the window here.
	ungrouped := strings.ReplaceAll(keepTogetherNonContiguousTemplate, `"keepTogether": "signature", `, "")
	loose := keepTogetherPages(t, ungrouped)
	if len(loose) != 2 {
		t.Fatalf("the untagged twin must also paginate to 2 pages, got %d", len(loose))
	}
	if strings.Contains(loose[0].text, keepTogetherDateText) {
		t.Fatal("WITHOUT the tags the last element must follow the window to page 2 — if it does not, this case no longer discriminates")
	}
	if !strings.Contains(loose[1].text, keepTogetherDateText) {
		t.Fatal("without the tags the last element belongs on page 2")
	}
}

// keepTogetherSingleMemberTemplate is the degenerate case: ONE element
// carries a tag, so its "group" has exactly one member and its union
// extent is its own extent. Placement must be identical to the same
// element untagged — a group of one constrains nothing.
//
// The element is deliberately SINGLE-LINE, and this row is about an item
// whose group adds no member it did not already have.
//
// A MULTI-LINE text element tagged alone is a DIFFERENT ROW, and since
// Story 7.10 it has a different answer. Its lines really are something to
// keep together, so the "group of one is a no-op" reading was false for it
// (D-7.10.1): if their union exceeds a content window the document is
// REFUSED, because the author declared an atomic block that fits nowhere.
// See TestKeepTogetherOverTallTaggedElementIsRefused. What survives here
// unchanged is the case this const actually is — a tagged element that
// FITS a window places exactly where the same element untagged places.
const keepTogetherSingleMemberTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 523, "height": 40, "value": "Body", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 720, "width": 240, "height": 16, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherSingleMemberChangesNothing asserts the identity at the
// bytes, which is the strongest form available: the two documents render
// to the same document.
func TestKeepTogetherSingleMemberChangesNothing(t *testing.T) {
	tagged := keepTogetherRenderOf(t, keepTogetherSingleMemberTemplate)
	untagged := keepTogetherRenderOf(t, strings.ReplaceAll(keepTogetherSingleMemberTemplate, `"keepTogether": "signature", `, ""))
	if sha256Hex(tagged) != sha256Hex(untagged) {
		t.Fatal("a one-member group must place its element exactly where the same element untagged is placed — the two renders differ")
	}
	// And the placement is the interesting one: the element does not fit
	// window one, so both documents put it on page 2. Without this the
	// identity above would hold vacuously for an element that never went
	// near a boundary.
	pages := keepTogetherPages(t, keepTogetherSingleMemberTemplate)
	if len(pages) != 2 || strings.Contains(pages[0].text, keepTogetherSignatureText) || !strings.Contains(pages[1].text, keepTogetherSignatureText) {
		t.Fatalf("the one-member group's element must fall to page 2 exactly as an untagged element would; got %d page(s)", len(pages))
	}
}

// keepTogetherTwoGroupsTemplate is the independence case: TWO tags in one
// content band.
//
//	group "a": e2 (690.000 .. 704.982) and e3 (700.000 .. 714.982)
//	           union 690.000 .. 714.982 — FITS window one (729.890)
//	group "b": e4 (720.000 .. 734.982) and e5 (760.000 .. 774.982)
//	           union 720.000 .. 774.982 — does NOT fit
//
// The discriminator is that group "a" STAYS. If the two tags were folded
// into one group the union would be 690.000 .. 774.982 and all four
// elements would move together, which is exactly the failure a test that
// only checked "b" moved would miss.
const keepTogetherTwoGroupsTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 523, "height": 40, "value": "Body", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 690, "width": 240, "height": 16, "keepTogether": "first-signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "text", "x": 0, "y": 700, "width": 240, "height": 16, "keepTogether": "first-signature", "value": "Director", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e4", "type": "text", "x": 300, "y": 720, "width": 240, "height": 16, "keepTogether": "second-signature", "value": "Countersigned by the Auditor", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e5", "type": "text", "x": 300, "y": 760, "width": 240, "height": 16, "keepTogether": "second-signature", "value": "Date: 31 August 2026", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherTwoGroupsAreIndependent is the I/O matrix's
// "two distinct tags" row: each group moves whole on its own, and
// neither constrains the other.
func TestKeepTogetherTwoGroupsAreIndependent(t *testing.T) {
	pages := keepTogetherPages(t, keepTogetherTwoGroupsTemplate)
	if len(pages) != 2 {
		t.Fatalf("want 2 pages, got %d", len(pages))
	}
	if !strings.Contains(pages[0].text, keepTogetherSignatureText) || !strings.Contains(pages[0].text, "Director") {
		t.Error("the first group fits window one and must stay whole on page 1 — a second group's extent must not drag it anywhere")
	}
	if strings.Contains(pages[0].text, "Countersigned by the Auditor") || strings.Contains(pages[0].text, keepTogetherDateText) {
		t.Error("the second group does not fit window one and must move whole to page 2")
	}
	if !strings.Contains(pages[1].text, "Countersigned by the Auditor") || !strings.Contains(pages[1].text, keepTogetherDateText) {
		t.Error("both members of the second group belong on page 2")
	}
}

// keepTogetherRenderOf renders an inline template through the public
// entry point and returns its bytes.
func keepTogetherRenderOf(t *testing.T, templateJSON string) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(templateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return res.Bytes
}

// keepTogetherOverTallTemplate is AC3's population: a declared group
// whose union extent exceeds a whole content window (729.890 pt), so no
// window of any position contains it.
//
// Its two members span 0 .. 900+, which is taller than any page this
// document has. Under D-4.6.2 as amended on 2026-08-31, that is CLIPPED
// with a Warning beside the bytes — never a fatal error, and never
// silent.
const keepTogetherOverTallTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 240, "height": 16, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 900, "width": 240, "height": 16, "keepTogether": "signature", "value": "Date: 31 August 2026", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherOverTallGroupIsClippedWithAWarning is AC3.
//
// It asserts the disposition (bytes plus exactly one Warning, never an
// error), the code (TABLE_ROW_CLIPPED_HEIGHT, reused with a fourth role
// arm rather than minted — D-4.6.2 as amended), the location (the
// group's first member) and, most importantly, the PROSE: without the
// fourth arm a signature block is announced to its author as "row 0 of
// the bound collection" with a remedy about cell padding.
func TestKeepTogetherOverTallGroupIsClippedWithAWarning(t *testing.T) {
	tpl, err := ParseTemplate([]byte(keepTogetherOverTallTemplate))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("an over-tall keep-together group must never be fatal: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("an over-tall keep-together group must still return a document")
	}
	var clips []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			clips = append(clips, d)
		}
	}
	if len(clips) != 1 {
		t.Fatalf("want exactly one TABLE_ROW_CLIPPED_HEIGHT diagnostic, got %d: %+v", len(clips), res.Diagnostics)
	}
	c := clips[0]
	if c.Severity != SeverityWarning {
		t.Errorf("severity = %v, want Warning — AD-14 makes clipped content a Warning returned alongside the bytes", c.Severity)
	}
	if c.ElementID != "e1" {
		t.Errorf("ElementID = %q, want the group's FIRST member e1", c.ElementID)
	}
	if strings.Contains(c.Message, "of the bound collection") {
		t.Errorf("the message announces a keep-together group as a table row — the fourth role arm is missing:\n%s", c.Message)
	}
	if !strings.Contains(c.Message, `keep-together group "signature"`) {
		t.Errorf("the message must name the keep-together group the author declared:\n%s", c.Message)
	}
	if !strings.Contains(c.Message, "stop declaring the group") {
		t.Errorf("the message must name an action its author can actually take on a group:\n%s", c.Message)
	}
	if strings.Contains(c.Message, "cell padding") {
		t.Errorf("the remedy for an over-tall group is not a table row's remedy:\n%s", c.Message)
	}
}

// --- Story 7.10's discriminator: WHAT is over-tall, not WHETHER it is
// --- grouped (D-7.10.1, D-7.10.2) ----------------------------------------
//
// ONE FIXTURE SUBJECT, THREE DOCUMENTS. One geometry, one element set,
// differing in exactly which element carries the tag — because either arm
// alone is ALSO consistent with the rule being replaced. An isolated fatal
// case is consistent with "untagged elements are fatal"; an isolated clip
// case is consistent with "grouped things are clipped". Only the pair, held
// against a common base, measures the discriminator itself.
//
// IT IS A DISCRIMINATOR, NOT A DEMONSTRATION — the standard this file's own
// keepTogetherTemplateJSON/keepTogetherUngroupedTemplateJSON pair sets, and
// the one this pair is held to.
//
// WHY THREE DOCUMENTS AND NOT ONE. AD-14 makes an Error ABORT the render and
// a Warning ACCOMPANY a successful one, so no single rendered document can
// yield both arms: a document that is refused produces no bytes and cannot
// also carry a clip Warning. That is arithmetic on AD-14's own definitions,
// not a gap in the fixture (D-7.10.6).
//
//	e1  text, y 0, width 40 — sixty short words, one per line, so its own
//	    extent is ~899pt against a 729.890pt content window. INDIVIDUALLY
//	    over-tall, and it renders perfectly well untagged.
//	e2  text, y 900   — one line, 14.982pt. Fits any window.
//	e3  text, y 1700  — one line, 14.982pt. Fits any window; the union
//	    e2..e3 is 814.982pt, which fits none.
//
// The three documents:
//
//	NONE tagged      → renders cleanly, no diagnostic. The control.
//	e1 tagged alone  → REFUSED, a located fatal OverflowError naming e1.
//	e2+e3 tagged     → Story 4.6's clip-and-warn, unchanged (D-7.10.4).
const keepTogetherDiscriminatorTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 40, "height": 16, %TAG_e1%"value": "` + keepTogetherSixtyWords + `", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 300, "y": 900, "width": 240, "height": 16, %TAG_e2%"value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "text", "x": 300, "y": 1700, "width": 240, "height": 16, %TAG_e3%"value": "Date: 31 August 2026", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// keepTogetherTagSlot names the slot belonging to ONE element. Every
// document in the triple is built from the SAME base by filling these three
// slots, so "the pair differs for some second reason" is unrepresentable
// rather than merely unasserted.
//
// THE SLOTS ARE NAMED, NOT POSITIONAL, and that is this helper's whole
// point. They were three identical `%TAG%` markers filled by
// `strings.Replace(..., 1)` in a loop over {e1, e2, e3} — correct only while
// the base lists its elements in exactly that order, an assumption nothing
// checked. Reordering the elements is a legitimate edit to hand-maintained
// JSON, and it would silently move every tag to a different element with the
// arms AND the precondition test below still green while measuring something
// else entirely. A slot carrying its own element's id cannot be
// mis-assigned, so the property is enforced by construction rather than
// asserted.
func keepTogetherTagSlot(id string) string { return "%TAG_" + id + "%" }

// keepTogetherTagFill is what a filled slot becomes — the one spelling, so
// the arms, the doc builder and the precondition test cannot disagree about
// it.
const keepTogetherTagFill = `"keepTogether": "block", `

// keepTogetherDiscriminatorIDs are the base template's three elements, in
// the order the fixture's doc comment above describes them.
var keepTogetherDiscriminatorIDs = []string{"e1", "e2", "e3"}

// keepTogetherSixtyWords shapes to sixty lines at 11pt inside a 40pt box —
// each word is far too wide to share a line with the next and far too narrow
// to be clipped at the box edge, so neither the line count nor a width
// diagnostic is at the mercy of the shaper.
const keepTogetherSixtyWords = "word word word word word word word word word word " +
	"word word word word word word word word word word " +
	"word word word word word word word word word word " +
	"word word word word word word word word word word " +
	"word word word word word word word word word word " +
	"word word word word word word word word word word"

// keepTogetherDiscriminatorDoc fills the three tag slots BY NAME: an element
// named in `tagged` gets the tag, every other slot is emptied. Naming an
// element the base does not declare is a bug in the caller, not a document
// with one fewer tag, so it panics rather than returning quietly.
func keepTogetherDiscriminatorDoc(tagged ...string) string {
	want := map[string]bool{}
	for _, id := range tagged {
		if !strings.Contains(keepTogetherDiscriminatorTemplate, keepTogetherTagSlot(id)) {
			panic("keepTogetherDiscriminatorDoc: the base declares no tag slot for element " + id)
		}
		want[id] = true
	}
	out := keepTogetherDiscriminatorTemplate
	for _, id := range keepTogetherDiscriminatorIDs {
		fill := ""
		if want[id] {
			fill = keepTogetherTagFill
		}
		out = strings.ReplaceAll(out, keepTogetherTagSlot(id), fill)
	}
	return out
}

// TestKeepTogetherDiscriminatorTripleDiffersOnlyByTheTags is the precondition
// every arm rests on, in this file's own established shape: strip the tags
// from any of the documents and you get the same document back.
//
// It also asserts WHICH element each tag landed on, which the positional fill
// this helper used to do could never assert about itself.
func TestKeepTogetherDiscriminatorTripleDiffersOnlyByTheTags(t *testing.T) {
	for _, id := range keepTogetherDiscriminatorIDs {
		if n := strings.Count(keepTogetherDiscriminatorTemplate, keepTogetherTagSlot(id)); n != 1 {
			t.Fatalf("element %s must own exactly one named tag slot in the base, found %d", id, n)
		}
	}
	if strings.Contains(keepTogetherDiscriminatorTemplate, "%TAG%") {
		t.Fatal("the base still carries an unnamed positional slot — nothing fills it, so it would reach the JSON parser verbatim")
	}
	control := keepTogetherDiscriminatorDoc()
	if strings.Contains(control, "keepTogether") {
		t.Fatal("the untagged control must carry no keepTogether tag at all")
	}
	if strings.Contains(control, "%TAG_") {
		t.Fatal("an unfilled tag slot survived into the control document")
	}
	for _, tagged := range [][]string{{"e1"}, {"e2", "e3"}, {"e1", "e2"}} {
		doc := keepTogetherDiscriminatorDoc(tagged...)
		if n := strings.Count(doc, keepTogetherTagFill); n != len(tagged) {
			t.Fatalf("%v: want %d tag(s), found %d", tagged, len(tagged), n)
		}
		if stripped := strings.ReplaceAll(doc, keepTogetherTagFill, ""); stripped != control {
			t.Fatalf("%v is not the control plus its tags — the arms differ for some SECOND reason, so they no longer discriminate:\n%s", tagged, stripped)
		}

		// AND ON THE RIGHT ELEMENTS. Each element occupies one line of
		// the base, so "which element carries the tag" is readable
		// directly — this is the assertion a positional fill cannot
		// make, and the reason the slots are named.
		var carrying []string
		for _, line := range strings.Split(doc, "\n") {
			if !strings.Contains(line, keepTogetherTagFill) {
				continue
			}
			for _, id := range keepTogetherDiscriminatorIDs {
				if strings.Contains(line, `"id": "`+id+`"`) {
					carrying = append(carrying, id)
				}
			}
		}
		if strings.Join(carrying, ",") != strings.Join(tagged, ",") {
			t.Fatalf("the tags landed on %v; %v was asked for. A tag on the wrong element leaves every arm green while measuring a different document.", carrying, tagged)
		}
	}
}

// TestKeepTogetherOverTallTaggedElementIsRefused is Story 7.10's AC1 and the
// FATAL arm of the discriminator. Its twin is
// TestKeepTogetherAggregateOnlyGroupIsStillClipped, immediately below, which
// is the same subject with the tag moved to two elements that each fit; the
// two are only evidence together.
//
// WHAT IS ASSERTED, AND WHY IT IS NOT MESSAGE-EQUALITY WITH THE UNTAGGED
// CASE (D-7.10.3). "The same error the untagged element receives" was a
// PROXY for "the author declared this and can fix it". It held for the
// population the ruling had in front of it — a rect, an image, fatal either
// way — and is FALSE for this one: untagged, e1's sixty lines split across
// windows and the document renders perfectly. There is no untagged error to
// be equal to, and asserting one would be asserting a falsehood.
//
// THE TAG IS WHAT MAKES IT UNSATISFIABLE, and that is the whole ruling: the
// author declared an atomic block that fits on no page, which is
// UNSATISFIABLE rather than merely degraded, and the fix is in their own
// hands — remove the tag. AD-25's "declared atomic, so clip it" precedent is
// WIDTH-only and does not transfer (D-2.6.1 excluded page-edge overflow from
// FR44), and Story 4.6's clip is an exception justified by a row's height
// being data-driven and UNFIXABLE — which a tag never is.
//
// Deleting the discriminator (ItemGroup.AuthorDeclared's arm in the clip
// branch) reddens this test and nothing else in this file.
func TestKeepTogetherOverTallTaggedElementIsRefused(t *testing.T) {
	// THE CONTROL FIRST, so the refusal below is known to be the TAG's
	// doing and not the element's. Untagged, this very element renders.
	control, controlDiags := keepTogetherPagesWithDiagnostics(t, keepTogetherDiscriminatorDoc())
	if len(control) < 2 {
		t.Fatalf("presence precondition: e1 must be tall enough to span more than one window, so that 'it renders untagged' is a fact about splitting rather than about fitting; got %d page(s)", len(control))
	}
	if len(controlDiags) != 0 {
		t.Fatalf("the untagged control must render CLEANLY — no clip, no warning, no error; got %+v", controlDiags)
	}
	var controlWords int
	for _, p := range control {
		controlWords += strings.Count(p.text, "word")
	}
	if controlWords != 60 {
		t.Fatalf("presence precondition: the control must print all sixty of e1's words across its pages, got %d — if the untagged document already loses content, 'it renders cleanly' is not what is being contrasted", controlWords)
	}

	// THE ARM. The same element, now tagged, and tagged ALONE.
	tpl, err := ParseTemplate([]byte(keepTogetherDiscriminatorDoc("e1")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err == nil {
		t.Fatalf("a keep-together group whose single tagged element is taller than the content window must be REFUSED, not clipped. Got %d byte(s) and %+v.\n\nA group of one is a no-op ONLY where the group adds nothing: here the author declared this element's own sixty lines inseparable, and no window holds them. Clipping it destroys content the author never agreed to lose (DW-50).", len(res.Bytes), res.Diagnostics)
	}
	if len(res.Bytes) != 0 {
		t.Errorf("a refused document must carry NO bytes; got %d — AD-14 makes an Error abort the render", len(res.Bytes))
	}
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			t.Errorf("the refusal must not also carry a clip Warning: %+v", d)
		}
	}

	var overflow *layout.OverflowError
	if !errors.As(err, &overflow) {
		t.Fatalf("Render returned %v (%T); the refusal must be a *layout.OverflowError so a caller can tell it from an I/O failure", err, err)
	}
	if overflow.ElementID != "e1" {
		t.Errorf("the refusal names element %q; the over-tall element is e1 — an UNLOCATED refusal is what D-1.8.1 exists to prevent, and it is the whole reason this is an error rather than a silent clip", overflow.ElementID)
	}
	if overflow.Kind != "line" {
		t.Errorf("the refusal calls e1 a %q; its extent is the union of its own shaped LINES", overflow.Kind)
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("the message a human reads must name the element:\n%s", err.Error())
	}

	// AND THE PUBLIC CONTRACT, not only the internal type. *layout.OverflowError
	// lives in an internal package no caller outside this module can name;
	// what a CLI or the designer actually dispatches on is the RenderError's
	// coded Diagnostic. Asserting only the internal type would let the
	// refusal reach the boundary uncoded — flattened to "The template could
	// not be processed" at the WASM edge — with this test still green.
	keepTogetherRefusalPublicContract(t, err, "e1")

	// THE MEASUREMENT THAT MAKES THE MEMBER UNIT THE TEMPLATE ELEMENT
	// (D-7.10.1). Every one of e1's sixty items is a single ~15pt line and
	// fits a window on its own; only their union does not. Read in ITEMS,
	// this group is "aggregate-only" and would be clipped forever — which
	// is exactly how DW-50 fell through D-7.7.9. Stated as a number so a
	// future reader can see the trap rather than take it on trust.
	if overflow.ItemHeight <= overflow.ContentHeight {
		t.Errorf("the refusal reports an item height of %d mp against a content height of %d — it must report e1's OWN UNION extent, not one line's", overflow.ItemHeight, overflow.ContentHeight)
	}
}

// TestKeepTogetherAggregateOnlyGroupIsStillClipped is the SECOND arm, and it
// is required rather than decorative (D-7.10.6). Without it the change reads
// as "tagging makes things fatal", and the next story generalises it in
// exactly the wrong direction.
//
// Same geometry, same three elements, same base document — the tag has
// simply moved from e1 (individually over-tall) to e2 and e3 (each fitting a
// window, their union not). That is the ONLY difference between this test
// and TestKeepTogetherOverTallTaggedElementIsRefused above, and it is the
// difference the discriminator reads.
//
// Story 7.7 shipped this case as a clip and Story 7.10 does not touch it
// (D-7.10.4) — named honestly: the fixability argument that makes the arm
// above fatal, pushed all the way, would make this one fatal too, since the
// author can untag it just the same. It is left because it is shipped,
// deliberate and outside this story's subject, NOT because it is obviously
// right. What would reopen it is a real document losing content this way.
//
// THE TWO MUTATION DIRECTIONS, BOTH WAYS ROUND, because the obvious reading
// of them is the wrong one and this comment had it backwards.
//
// NARROWING the discriminator — testing "some ITEM is over-tall" instead of
// "some ELEMENT is" — leaves THIS arm GREEN (e2 and e3 are one ~15pt line
// each, so no item of theirs is over-tall either way, and the group is still
// clipped) and REDDENS THE FATAL ARM ABOVE, whose e1 is sixty ~15pt lines
// that each fit a window while their union does not. That is DW-50's exact
// shape, which is why the member unit is the template element.
//
// OVER-BROADENING it — deciding on the group's UNION alone and dropping the
// per-element test — is what reddens THIS one: every aggregate-only group
// becomes fatal, and "tagging makes things fatal" is what shipped.
func TestKeepTogetherAggregateOnlyGroupIsStillClipped(t *testing.T) {
	tpl, err := ParseTemplate([]byte(keepTogetherDiscriminatorDoc("e2", "e3")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("a group that is over-tall only IN AGGREGATE keeps Story 4.6's clip-and-warn: %v.\n\nEvery member of this group fits a window on its own; only their union does not. If this reddened, the discriminator is reading ITEMS or is not reading the group's membership at all, and 'tagging makes things fatal' is what shipped.", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("an aggregate-only over-tall group must still return a document — AD-14 makes it a Warning ALONGSIDE the bytes")
	}
	var clips []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			clips = append(clips, d)
		}
	}
	if len(clips) != 1 {
		t.Fatalf("want exactly one TABLE_ROW_CLIPPED_HEIGHT Warning, got %d: %+v", len(clips), res.Diagnostics)
	}
	if clips[0].Severity != SeverityWarning {
		t.Errorf("severity = %v, want Warning", clips[0].Severity)
	}
	if clips[0].ElementID != "e2" {
		t.Errorf("ElementID = %q, want the group's FIRST member e2", clips[0].ElementID)
	}
	if !strings.Contains(clips[0].Message, `keep-together group "block"`) {
		t.Errorf("the Warning must name the group the author declared:\n%s", clips[0].Message)
	}

	// AND e1 IS UNTOUCHED. It is the very element the arm above refuses,
	// sitting untagged in this same document, printing all sixty of its
	// words across the pages. So the fatal arm is about the TAG, and this
	// arm is about the group's SHAPE — neither is about the element kind.
	pages, _ := keepTogetherPagesWithDiagnostics(t, keepTogetherDiscriminatorDoc("e2", "e3"))
	var words int
	for _, p := range pages {
		words += strings.Count(p.text, "word")
	}
	if words != 60 {
		t.Errorf("e1 is untagged here and must print in full: got %d of its 60 words", words)
	}
}

// keepTogetherRefusalPublicContract asserts the PUBLIC half of an over-tall
// refusal: the coded Diagnostic a caller outside this module can actually
// see.
//
// *layout.OverflowError lives in an INTERNAL package. No CLI, no designer
// build and no third-party caller can name that type, so a test that asserts
// only it is asserting a private detail: the refusal could reach the module
// boundary with the wrong code, or with none — an uncoded error is flattened
// to "The template could not be processed" at the WASM edge and never
// reaches the author at all — and every internal assertion would still pass.
// Same idiom as render_error_test.go's own unlayoutable-content row.
func keepTogetherRefusalPublicContract(t *testing.T, err error, wantElementID string) {
	t.Helper()
	var renderErr *RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("the refusal must reach the caller as a *RenderError: %T %v", err, err)
	}
	if renderErr.Diagnostic.Code != DiagCodeContentUnlayoutable {
		t.Errorf("Diagnostic.Code = %q, want %q — a caller dispatches on the CODE, never on the message", renderErr.Diagnostic.Code, DiagCodeContentUnlayoutable)
	}
	if renderErr.Diagnostic.Severity != SeverityError {
		t.Errorf("Diagnostic.Severity = %v, want Error — AD-14 makes an Error abort the render, and the severity is how a caller knows there are no bytes", renderErr.Diagnostic.Severity)
	}
	if renderErr.Diagnostic.ElementID != wantElementID {
		t.Errorf("the public Diagnostic names element %q, want %q — AD-10 requires every diagnostic that concerns an element to carry its id", renderErr.Diagnostic.ElementID, wantElementID)
	}
}

// TestKeepTogetherMixedGroupIsRefusedNamingTheOverTallElement is the arm that
// separates the SHIPPED predicate from the weaker one that passes every other
// test in this file.
//
// The group spans TWO distinct ElementIDs — e1, whose own sixty lines union
// to ~899pt against a 729.890pt window, and e2, one line that fits any window
// — so it is neither "a group of one" nor "aggregate-only". Under the shipped
// rule it is REFUSED, naming e1: the member unit is the template element, and
// one member is by itself taller than any window.
//
// WHY IT IS REQUIRED. Read the discriminator as "fatal iff the group spans
// exactly ONE ElementID" — the reading Task 1 of the story spec invited, and
// the one a distinct-count on ColumnItem makes natural — and every other arm
// here stays green: the fatal arm's group spans one id, the aggregate arm's
// spans two and is clipped. This document is the only one that tells them
// apart, and it is not a contrivance: it is the real-world signature block,
// an over-tall paragraph tagged together with the name printed beneath it.
// Under the weaker predicate that document is silently CLIPPED — DW-50
// reintroduced, with a green suite over it.
func TestKeepTogetherMixedGroupIsRefusedNamingTheOverTallElement(t *testing.T) {
	// PRESENCE PRECONDITION on the group's shape, so a future edit to the
	// base cannot turn this into a group-of-one arm without saying so.
	doc := keepTogetherDiscriminatorDoc("e1", "e2")
	if n := strings.Count(doc, keepTogetherTagFill); n != 2 {
		t.Fatalf("presence precondition: this arm's group must span two tagged elements, found %d tag(s)", n)
	}

	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err == nil {
		t.Fatalf("a keep-together group holding an individually over-tall element must be REFUSED even when its OTHER members fit. Got %d byte(s) and %+v.\n\nThis group spans two elements, so a discriminator that asks 'does the group span exactly one ElementID' calls it aggregate-only and clips it — and e1's sixty lines are destroyed with a Warning where the author declared them inseparable (DW-50).", len(res.Bytes), res.Diagnostics)
	}
	if len(res.Bytes) != 0 {
		t.Errorf("a refused document must carry NO bytes; got %d", len(res.Bytes))
	}
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			t.Errorf("the refusal must not also carry a clip Warning: %+v", d)
		}
	}

	var overflow *layout.OverflowError
	if !errors.As(err, &overflow) {
		t.Fatalf("Render returned %v (%T); want *layout.OverflowError", err, err)
	}
	if overflow.ElementID != "e1" {
		t.Errorf("the refusal names %q; the individually over-tall member is e1, and e2 fits any window — naming the wrong member sends the author to an element there is nothing wrong with", overflow.ElementID)
	}
	if overflow.Kind != "line" {
		t.Errorf("the refusal calls e1 a %q; its extent is the union of its own shaped LINES", overflow.Kind)
	}
	if overflow.ItemHeight <= overflow.ContentHeight {
		t.Errorf("the refusal reports %d mp against a content height of %d — it must report e1's OWN union extent, not the whole group's and not one line's", overflow.ItemHeight, overflow.ContentHeight)
	}
	// e1's own extent, NOT the group's union: the group runs to e2's
	// bottom at ~914.982pt, and reporting that number would name e1 while
	// measuring something e1 is not responsible for.
	if overflow.ItemHeight >= 914_000 {
		t.Errorf("the refusal reports an item height of %d mp, which is the GROUP's union rather than e1's own extent (~899pt) — the message names one element, so the quantity beside it must be that element's", overflow.ItemHeight)
	}
	keepTogetherRefusalPublicContract(t, err, "e1")
}

// keepTogetherOverTallTaggedImageTemplate is the IMAGE arm's document: an
// image element whose DECLARED BOX is 900pt tall against a 729.890pt content
// window, tagged into a group with a one-line text element that fits.
//
// The asset is the 3x2 PNG fixtures/image-embed/ uses. Its pixel dimensions
// are irrelevant — AD-24 scales the image to fit its box, so "does it fit on
// a page" is a question about what the TEMPLATE declared, which is exactly
// the property this arm needs.
const keepTogetherOverTallTaggedImageTemplate = `{
  "assets": {
    "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078": {"data": ["iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg", "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"], "mediaType": "image/png"}
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "image", "asset": "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078", "x": 0, "y": 0, "width": 100, "height": 900, "keepTogether": "block"},
        {"id": "e2", "type": "text", "x": 0, "y": 950, "width": 240, "height": 16, "keepTogether": "block", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherOverTallTaggedImageIsRefused is the arm that keeps
// overTallGroupMember's Kind derivation honest across ALL THREE item slices,
// not just the two a text document happens to produce.
//
// WHY IT IS REQUIRED. Narrow overTallGroupMember to items carrying Runs or
// Rects — an entirely natural "the group is text and chrome" simplification,
// since every other arm in this file is built from text elements — and this
// document silently reverts to Story 4.6's clip: the image is cut off at the
// page's content bottom and, because a clip drops whole members rather than
// halves, VANISHES FROM THE PDF ENTIRELY while the render reports success.
// The whole suite stays green. An untagged over-tall image has been refused
// since Story 2.6 (D-2.6.1); the tag must not be able to switch that off,
// which is D-7.10.1's rule stated on the one population where it is most
// obviously destructive.
func TestKeepTogetherOverTallTaggedImageIsRefused(t *testing.T) {
	tpl, err := ParseTemplate([]byte(keepTogetherOverTallTaggedImageTemplate))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err == nil {
		t.Fatalf("a keep-together group holding an image whose DECLARED BOX exceeds the content window must be REFUSED. Got %d byte(s) and %+v.\n\nClipping it does not shrink the image — a clip drops whole members, so the image disappears from the document while the render reports success.", len(res.Bytes), res.Diagnostics)
	}
	if len(res.Bytes) != 0 {
		t.Errorf("a refused document must carry NO bytes; got %d", len(res.Bytes))
	}
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			t.Errorf("the refusal must not also carry a clip Warning: %+v", d)
		}
	}

	var overflow *layout.OverflowError
	if !errors.As(err, &overflow) {
		t.Fatalf("Render returned %v (%T); want *layout.OverflowError", err, err)
	}
	if overflow.ElementID != "e1" {
		t.Errorf("the refusal names %q, want e1 — the over-tall member is the image, and e2's single line fits any window", overflow.ElementID)
	}
	if overflow.Kind != "image" {
		t.Errorf("the refusal calls e1 a %q; it is an IMAGE, and the word is what tells its author which declaration to shrink (D-7.10.5)", overflow.Kind)
	}
	if !strings.Contains(err.Error(), "element e1: image is taller than the content window") {
		t.Errorf("the message a human reads must name the element AND call it an image:\n%s", err.Error())
	}
	keepTogetherRefusalPublicContract(t, err, "e1")
}

// TestKeepTogetherOverTallElementBoxIsRefusedTaggedOrNot is the I/O matrix's
// first two rows on the DECLARED-BOX population, where — unlike the
// multi-line text above — the untagged case IS fatal, so the two really are
// the same error and the tag really is a no-op.
//
// THIS EQUALITY IS A FACT ABOUT THIS POPULATION AND MUST NEVER BE
// GENERALISED (D-7.10.3). Asserting it for a multi-line text element would
// be asserting a falsehood: untagged, that element renders. What generalises
// is the rule the equality is evidence for — an over-tall element is refused
// whether or not it is grouped, because WHAT is over-tall decides, never
// whether it happens to be tagged.
//
// The element declares style.background because that is what makes an
// element box exist at all (elementDeclaresBox, element_box.go): a rect with
// neither background nor border builds no source and is as tall as it likes
// with nothing to say about it.
func TestKeepTogetherOverTallElementBoxIsRefusedTaggedOrNot(t *testing.T) {
	const base = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "rect", "x": 0, "y": 0, "width": 200, "height": 900, %TAG%"style": {"background": "#1b2a4a"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`
	refusalFor := func(t *testing.T, doc string) *layout.OverflowError {
		t.Helper()
		tpl, err := ParseTemplate([]byte(doc))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
		if err == nil {
			t.Fatalf("an element box taller than the content window must be REFUSED; got %d byte(s) and %+v", len(res.Bytes), res.Diagnostics)
		}
		var overflow *layout.OverflowError
		if !errors.As(err, &overflow) {
			t.Fatalf("Render returned %v (%T); want *layout.OverflowError", err, err)
		}
		return overflow
	}

	untagged := refusalFor(t, strings.Replace(base, "%TAG%", "", 1))
	tagged := refusalFor(t, strings.Replace(base, "%TAG%", `"keepTogether": "block", `, 1))

	if *untagged != *tagged {
		t.Fatalf("the tag laundered the authorship: untagged %+v, tagged %+v.\n\nA group of one adds nothing the element did not already have, and an author must not be able to switch off a hard error by declaring an unrelated feature (D-7.10.1).", *untagged, *tagged)
	}
	if tagged.ElementID != "e1" {
		t.Errorf("the refusal names %q, want e1", tagged.ElementID)
	}
	if tagged.Kind != "box" {
		t.Errorf("the refusal calls the element a %q; it is a declared element BOX and the document contains no table (D-7.10.5)", tagged.Kind)
	}
}

// TestKeepTogetherGroupKeyIsNotAValidElementID verifies D-7.7 Ruling C
// rather than asserting it in prose.
//
// The keep-together key's ElementID lives in a namespace no element id
// can occupy, and THAT — not a comment — is what makes every FR26
// header-repeat, row-displacement, header-suppression and footer-orphan
// path unreachable for such a group. The day someone "tidies" the
// prefix into a plain identifier, every one of those paths reopens
// silently, and this is the test that says so.
func TestKeepTogetherGroupKeyIsNotAValidElementID(t *testing.T) {
	key := keepTogetherIndex{"e1": {tag: "signature"}}.keepTogetherGroup("e1").Key
	if key.IsHeader {
		t.Error("a keep-together group must never be a header group — IsHeader gates the FR26 repeat paths")
	}
	if key.Index == footerGroupIndex {
		t.Errorf("a keep-together group's Index must not collide with the footer sentinel %d", footerGroupIndex)
	}
	// The grammar itself, through the public loader: a document whose
	// element id IS the group key must be REFUSED at load, which is what
	// makes the two namespaces provably disjoint.
	doc := strings.Replace(keepTogetherOverTallTemplate, `"id": "e1"`, `"id": "`+key.ElementID+`"`, 1)
	if _, err := template.ParseDocument([]byte(doc)); err == nil {
		t.Fatalf("element id %q was ACCEPTED — the keep-together namespace is not disjoint from the element-id grammar, and every table-shaped path in internal/layout is reachable by a colliding key", key.ElementID)
	}
	// And the control, so the refusal above is about the id and not
	// about the document: the same document with a legal id loads.
	if _, err := template.ParseDocument([]byte(keepTogetherOverTallTemplate)); err != nil {
		t.Fatalf("the control document must load: %v", err)
	}
}

// TestKeepTogetherReachesNoTablePath is the behavioural half of D-7.7.1's
// co-extensiveness audit.
//
// The prefix argument says a keep-together key cannot reach any of the
// four sites in internal/layout that read an ItemGroupKey as a TABLE
// (enumerated in keepTogetherKeyPrefix's own doc comment). This measures
// the consequence instead of restating the argument: a document made
// ENTIRELY of keep-together groups, including one over-tall enough to
// take the Story 4.6 clip branch — the branch that contains two of the
// four sites — must produce no header repeat, no row displacement and no
// header suppression at all.
func TestKeepTogetherReachesNoTablePath(t *testing.T) {
	geometry := layout.PageGeometry{
		Width: 595_276, Height: 841_890,
		MarginTop: 36_000, MarginBottom: 36_000,
		MarginLeft: 36_000, MarginRight: 36_000,
		PageHeaderHeight: 20_000, PageFooterHeight: 20_000,
	}
	idx := keepTogetherIndex{"e1": {tag: "signature"}, "e2": {tag: "signature"}}
	g := idx.keepTogetherGroup("e1")
	items := []layout.ColumnItem{
		{ElementID: "e1", Top: 20_000, Bottom: 34_443, Runs: []layout.TextRunRef{0}, Group: g},
		// Far enough below to make the UNION taller than a whole content
		// window (729.890 pt), which is what routes this group through
		// the clip branch rather than the ordinary slide.
		{ElementID: "e2", Top: 940_000, Bottom: 954_443, Runs: []layout.TextRunRef{1}, Group: g},
	}
	plan, err := layout.Paginate(geometry, items)
	if err != nil {
		t.Fatalf("an over-tall keep-together group must be clipped, never fatal: %v", err)
	}
	if len(plan.Clipped) != 1 {
		t.Fatalf("want exactly one clip record, got %d", len(plan.Clipped))
	}
	if got := plan.Clipped[0].Key; got != g.Key {
		t.Errorf("the clip record must carry the group's own key, got %+v", got)
	}
	for i, p := range plan.Pages {
		if len(p.HeaderRepeats) != 0 {
			t.Errorf("page %d produced %d TableHeaderRepeat(s) for a group that is not a table — Gate B let the key through", i, len(p.HeaderRepeats))
		}
		if len(p.RowDisplacement) != 0 {
			t.Errorf("page %d produced %d RowDisplacement(s) for a group that is not a table", i, len(p.RowDisplacement))
		}
	}
	if len(plan.Suppressed) != 0 {
		t.Errorf("a keep-together group produced %d TableHeaderSuppressed record(s) — and TableHeaderSuppressed carries no Key, so a spurious one is indistinguishable at the caller", len(plan.Suppressed))
	}
	// The footer-orphan machinery must build no target for it either.
	if targets := footerOrphanTargetsFrom(items); len(targets) != 0 {
		t.Errorf("footerOrphanTargetsFrom built %d target(s) for a keep-together group: %+v", len(targets), targets)
	}
}

// TestKeepTogetherLeavesEveryTableRowUntouched is AC4's mechanism,
// asserted at the derivation rather than only through the goldens: the
// substitution fills ONLY the ungrouped case, so a table row's own key
// survives it and an untagged element gets the zero value it always had.
func TestKeepTogetherLeavesEveryTableRowUntouched(t *testing.T) {
	idx := keepTogetherIndex{"e9": {tag: "signature"}}
	row := layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: "e9", Index: 3}}
	if got := idx.orKeepTogether(row, "e9"); got != row {
		t.Fatalf("a table row's own group must survive the substitution even when its element carries a tag, got %+v", got)
	}
	if got := idx.orKeepTogether(layout.ItemGroup{}, "e7"); got.Present {
		t.Fatalf("an UNTAGGED element must keep the zero ItemGroup, got %+v", got)
	}
	if got := idx.orKeepTogether(layout.ItemGroup{}, "e9"); !got.Present {
		t.Fatal("a tagged, otherwise-ungrouped element must acquire the keep-together group")
	}
}

// --- PHASE A must see the grouping too (Story 7.7) -----------------------
//
// contentColumnItems (page_number.go) is PHASE A: the page-count-only
// pass whose len(Pages) is the Y that {{pages}} prints and that
// D-2.7.2's {{page}} reservation is sized from. paginateDocument
// (render.go) is PHASE B, the pass that actually places content. Both
// carry the keep-together substitution, and BOTH comments say why: a
// grouping seen by only one of them makes the page COUNT disagree with
// the render.
//
// Nothing measured that until this test. Every other keep-together
// document here declares an EMPTY pageHeader and pageFooter, so nothing
// in them ever reads PHASE A's count at all, and all three PHASE A
// substitutions could be mutated away with the whole suite still green.
// This document is the one that reads it.

// keepTogetherPageCountTemplate is authored so the GROUPING CHANGES THE
// PAGE COUNT, not merely where the block sits — which is the only shape
// of document in which PHASE A's answer is observable at all.
//
//	e1  y 0     -> 0.000 .. 14.982
//	e2  y 100   -> 100.000 .. 114.982    tagged
//	e3  y 720   -> 720.000 .. 734.982    tagged
//	e4  y 1000  -> 1000.000 .. 1014.982
//
// UNGROUPED: window one is [0, 729.890], so e1 and e2 sit on page 1 and
// e3 opens page 2 with the window sliding to e3's own top, 720 — window
// two is [720, 1449.890], which also holds e4. TWO pages.
//
// GROUPED: the group's union extent is 100.000 .. 734.982 (634.982 pt,
// comfortably shorter than a whole window, so it is not over-tall) and
// it does not fit window one, so the window slides to the group's
// EARLIEST top, 100 — window two is [100, 829.890]. e2 and e3 ride there
// together, and e4 at 1000 no longer fits, so it opens a THIRD page.
// THREE pages.
//
// The page footer prints "Page {{page}} of {{pages}}". {{pages}} is not
// a late-bound slot: it is resolved once, from PHASE A's count, and
// printed literally (D-2.7.1/D-2.7.2). So if PHASE A does not see the
// grouping, every footer in a three-page document reads "of 2".
const keepTogetherPageCountTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 240, "height": 16, "value": "Body", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 240, "height": 16, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "text", "x": 0, "y": 720, "width": 240, "height": 16, "keepTogether": "signature", "value": "Date: 31 August 2026", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e4", "type": "text", "x": 0, "y": 1000, "width": 240, "height": 16, "value": "Continued overleaf", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [{"id": "e5", "type": "text", "x": 0, "y": 0, "width": 240, "height": 16, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 11}}], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherPageCountReachesTheRenderedFooter is the PHASE A
// assertion, made at the only place PHASE A's answer is visible to a
// reader of the document: the printed total in the page footer.
//
// It fails — with the footers reading "of 2" over three pages — if any
// of contentColumnItems' three keep-together substitutions is removed,
// because the two passes then disagree about how many pages the
// document has and the count the footers print is PHASE A's.
func TestKeepTogetherPageCountReachesTheRenderedFooter(t *testing.T) {
	grouped := keepTogetherPages(t, keepTogetherPageCountTemplate)
	if len(grouped) != 3 {
		t.Fatalf("the grouped document must paginate to 3 pages, got %d — this fixture only measures PHASE A while the grouping CHANGES the count", len(grouped))
	}
	for i, p := range grouped {
		if !strings.Contains(p.text, "of 3") {
			t.Errorf("page %d's footer must state the GROUPED total, 3 — PHASE A (contentColumnItems) and PHASE B (paginateDocument) disagree about the page count, and {{pages}} prints PHASE A's answer. Page text: %q", i+1, p.text)
		}
	}
	if !strings.Contains(grouped[1].text, keepTogetherSignatureText) || !strings.Contains(grouped[1].text, keepTogetherDateText) {
		t.Error("both members of the group belong on page 2")
	}
	if !strings.Contains(grouped[2].text, "Continued overleaf") {
		t.Error("the element after the group opens page 3 — that displacement IS the page-count change")
	}

	// The control, so "of 3" is evidence about the grouping rather than
	// about this document: the SAME document without its tags is two
	// pages, and says so.
	ungrouped := keepTogetherPages(t, strings.ReplaceAll(keepTogetherPageCountTemplate, `"keepTogether": "signature", `, ""))
	if len(ungrouped) != 2 {
		t.Fatalf("without the tags the document must paginate to 2 pages, got %d — if it does not, this case no longer discriminates", len(ungrouped))
	}
	for i, p := range ungrouped {
		if !strings.Contains(p.text, "of 2") {
			t.Errorf("untagged page %d's footer must state 2: %q", i+1, p.text)
		}
	}
}

// TestBothPaginationPassesAgreeWithAKeepTogetherGroup is
// TestBothPaginationPassesAgreeOnRowPartition's sibling for this story's
// population (table_pagination_test.go).
//
// It asserts the stronger fact the footer test observes only the
// consequence of: with a group declared, the two builders — which append
// their items in DIFFERENT orders (PHASE A text-then-images, PHASE B
// rects-then-text-then-images) — produce the same page count AND the
// same per-element partition. PHASE A here is the production function
// itself, contentColumnItems, so removing its substitution reddens this
// too.
func TestBothPaginationPassesAgreeWithAKeepTogetherGroup(t *testing.T) {
	tpl, err := ParseTemplate([]byte(keepTogetherImageTemplate))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	data := mustDecodeData(t, keepTogetherDataJSON)
	params := mustDecodeParams(t)
	geometry, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	visible, _, err := computeVisibility(bands, data, params, testFormatContext())
	if err != nil {
		t.Fatalf("computeVisibility: %v", err)
	}
	imageRuns, err := collectImageRuns(tpl)
	if err != nil {
		t.Fatalf("collectImageRuns: %v", err)
	}
	contentRuns, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, data, params, testShippedFontSet(), newFontCache(), contentBandResolver, visible)
	if err != nil {
		t.Fatalf("collectBandTextRuns: %v", err)
	}
	keepTogether := keepTogetherTags(tpl)
	if len(keepTogether) != 2 {
		t.Fatalf("presence precondition: the fixture must declare a two-member group, got %d tagged element(s)", len(keepTogether))
	}

	// PHASE A: the real production function.
	phaseA, err := layout.Paginate(geometry, contentColumnItems(contentRuns, imageRuns, nil, visible, keepTogether))
	if err != nil {
		t.Fatalf("PHASE A Paginate: %v", err)
	}

	// PHASE B: paginateDocument's OWN item order, over the SAME slices,
	// so any divergence is about order and grouping alone.
	var phaseBItems []layout.ColumnItem
	for i := 0; i < len(contentRuns); i++ {
		j := i
		item := layout.ColumnItem{
			ElementID: contentRuns[i].elementID,
			Top:       contentRuns[i].itemTop,
			Bottom:    contentRuns[i].itemBottom,
			Group:     keepTogether.orKeepTogether(contentRuns[i].lineRowGroup(), contentRuns[i].elementID),
		}
		for j < len(contentRuns) &&
			contentRuns[j].elementID == contentRuns[i].elementID &&
			contentRuns[j].lineIndex == contentRuns[i].lineIndex {
			item.Runs = append(item.Runs, layout.TextRunRef(j))
			j++
		}
		phaseBItems = append(phaseBItems, item)
		i = j - 1
	}
	for i, r := range imageRuns {
		if r.band != contentBandIndex || !isVisible(visible, template.ElementID(r.elementID)) {
			continue
		}
		phaseBItems = append(phaseBItems, layout.ColumnItem{
			ElementID: r.elementID,
			Top:       r.y,
			Bottom:    r.y + r.boxH,
			Images:    []layout.ImageRef{layout.ImageRef(i)},
			Group:     keepTogether.keepTogetherGroup(r.elementID),
		})
	}
	phaseB, err := layout.Paginate(geometry, phaseBItems)
	if err != nil {
		t.Fatalf("PHASE B Paginate: %v", err)
	}

	if len(phaseA.Pages) != len(phaseB.Pages) {
		t.Fatalf("PHASE A produced %d pages, PHASE B produced %d — {{page}}/{{pages}} (D-2.7.2) would print the WRONG total", len(phaseA.Pages), len(phaseB.Pages))
	}
	if len(phaseA.Pages) < 2 {
		t.Fatalf("presence precondition: the fixture must paginate to >= 2 pages, got %d", len(phaseA.Pages))
	}
	pagesPerElement := func(plan layout.Pagination) map[string]map[int]bool {
		out := map[string]map[int]bool{}
		add := func(id string, p int) {
			if out[id] == nil {
				out[id] = map[int]bool{}
			}
			out[id][p] = true
		}
		for p, pg := range plan.Pages {
			for _, ref := range pg.ContentRuns {
				add(contentRuns[ref].elementID, p)
			}
			for _, ref := range pg.ContentImages {
				add(imageRuns[ref].elementID, p)
			}
		}
		return out
	}
	a, b := pagesPerElement(phaseA), pagesPerElement(phaseB)
	if len(a) == 0 {
		t.Fatal("coverage witness: PHASE A placed no element at all")
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("the two passes partitioned the SAME elements differently:\nPHASE A: %v\nPHASE B: %v", a, b)
	}
}

// --- a tagged IMAGE (Story 7.7) ------------------------------------------
//
// Both item builders tag images as well as text and rects, and D-4.6.2's
// amendment makes an image-specific promise: an image inside an
// over-tall group is REMOVED, not moved. Every other keep-together
// document here is text and rect only, so both image substitutions could
// be deleted with the suite still green. These two documents are what
// exercise them.

// keepTogetherImageAsset is the 3x2 8-bit RGB PNG maximalFixture ships,
// under its own SHA-256 key. Its CONTENT is irrelevant here — what
// matters is that the element is a real, resolvable image, so it reaches
// the page model as an ImagePlacement and its travel is observable.
const keepTogetherImageAsset = `"assets": {
    "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078": {
      "data": ["iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg", "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"],
      "mediaType": "image/png"
    }
  }`

// keepTogetherImageTemplate is a signature block whose second member is
// a signature IMAGE.
//
//	e2  text,  y 700 -> 700.000 .. 714.982   tagged — FITS window one
//	e3  image, y 718 -> 718.000 .. 758.000   tagged — does NOT fit
//
// Untagged, that severs the block exactly as the text-only fixture does,
// with the name at the foot of page 1 and the image at the head of
// page 2. Tagged, the union 700.000 .. 758.000 slides whole to page 2.
const keepTogetherImageTemplate = `{
  ` + keepTogetherImageAsset + `,
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 240, "height": 16, "value": "Body", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 700, "width": 240, "height": 16, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "image", "x": 0, "y": 718, "width": 60, "height": 40, "keepTogether": "signature", "asset": "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherCarriesATaggedImage is the image member's own travel,
// asserted as the difference between the tagged document and the same
// document untagged — the same discriminating shape every other case
// here uses.
func TestKeepTogetherCarriesATaggedImage(t *testing.T) {
	grouped := keepTogetherPages(t, keepTogetherImageTemplate)
	if len(grouped) != 2 {
		t.Fatalf("want 2 pages, got %d", len(grouped))
	}
	if strings.Contains(grouped[0].text, keepTogetherSignatureText) || grouped[0].images != 0 {
		t.Errorf("page 1 must carry NO member of the block: got %d image(s) and text %q", grouped[0].images, grouped[0].text)
	}
	if !strings.Contains(grouped[1].text, keepTogetherSignatureText) {
		t.Error("page 2 must carry the signature line")
	}
	if grouped[1].images != 1 {
		t.Errorf("page 2 must carry the signature IMAGE, got %d image(s) — an image is a group member like any other, and both item builders must tag it", grouped[1].images)
	}

	// The control: untagged, the text is stranded on page 1 and only the
	// image falls to page 2. Without this the assertions above would
	// hold for a build that never tagged the image at all.
	loose := keepTogetherPages(t, strings.ReplaceAll(keepTogetherImageTemplate, `"keepTogether": "signature", `, ""))
	if len(loose) != 2 {
		t.Fatalf("the untagged twin must also paginate to 2 pages, got %d", len(loose))
	}
	if !strings.Contains(loose[0].text, keepTogetherSignatureText) || loose[0].images != 0 {
		t.Fatalf("WITHOUT the tags the signature line must be STRANDED on page 1 and the image must not be there — this case no longer discriminates; got %d image(s) and text %q", loose[0].images, loose[0].text)
	}
	if loose[1].images != 1 {
		t.Fatalf("without the tags the image alone falls to page 2, got %d image(s)", loose[1].images)
	}
}

// keepTogetherOverTallImageTemplate is D-4.6.2's consciously accepted
// consequence, made measurable: a tagged image inside a group too tall
// for any window.
//
//	e1  text,  y 0   -> 0.000 .. 14.982     tagged
//	e2  image, y 900 -> 900.000 .. 940.000  tagged
//
// The union is 940.000 pt against a 729.890 pt window, so the group
// takes the Story 4.6 clip branch: a page of its own, clipped at that
// page's content bottom, one Warning beside the bytes. The clip drops
// whole members' Runs AND Images, so the image is REMOVED from the
// document rather than moved to a page of its own.
const keepTogetherOverTallImageTemplate = `{
  ` + keepTogetherImageAsset + `,
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 240, "height": 16, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "image", "x": 0, "y": 900, "width": 60, "height": 40, "keepTogether": "signature", "asset": "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// TestKeepTogetherOverTallGroupDropsATaggedImage holds folio-format.md's
// image-specific promise — "an image inside an over-tall group is
// removed, not moved" — to a measurement rather than to prose.
func TestKeepTogetherOverTallGroupDropsATaggedImage(t *testing.T) {
	pages, diags := keepTogetherPagesWithDiagnostics(t, keepTogetherOverTallImageTemplate)
	if len(pages) != 1 {
		t.Fatalf("an over-tall group gets a page of its OWN and is clipped there, so this document is one page; got %d", len(pages))
	}
	if pages[0].images != 0 {
		t.Errorf("the tagged image is past the clipped page's content bottom, so it must be ABSENT from the document, not moved to a page of its own; got %d image(s)", pages[0].images)
	}
	if !strings.Contains(pages[0].text, keepTogetherSignatureText) {
		t.Error("the member that DOES fit the clipped page must still print — the clip drops what is past the bottom, not the whole group")
	}
	var clips int
	for _, d := range diags {
		if d.Code == DiagCodeTableRowClippedHeight {
			clips++
		}
	}
	if clips != 1 {
		t.Errorf("want exactly one TABLE_ROW_CLIPPED_HEIGHT Warning beside the bytes, got %d: %+v", clips, diags)
	}

	// The control: untagged, the very same image is MOVED to page 2 and
	// nothing is clipped or warned about. So the absence above is the
	// grouping's doing, not the image's own placement.
	loose, looseDiags := keepTogetherPagesWithDiagnostics(t, strings.ReplaceAll(keepTogetherOverTallImageTemplate, `"keepTogether": "signature", `, ""))
	if len(loose) != 2 || loose[1].images != 1 {
		t.Fatalf("WITHOUT the tags the image is simply moved to page 2 and nothing is dropped — this case no longer discriminates; got %d page(s)", len(loose))
	}
	if len(looseDiags) != 0 {
		t.Fatalf("the untagged twin must be clean; got %+v", looseDiags)
	}
}

// renderKeepTogether renders fixtures/keep-together/ through the public
// entry point.
func renderKeepTogether(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(keepTogetherTemplateJSON))
	if err != nil {
		t.Fatalf("parse keep-together template: %v", err)
	}
	res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render keep-together: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the keep-together fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// TestKeepTogetherDeclaresItsOwnVersion: the fixture carries the key, so
// it must declare 1.2 and must keep declaring it on re-save — never the
// library's ceiling (D-7.2.1, D-7.7.2).
func TestKeepTogetherDeclaresItsOwnVersion(t *testing.T) {
	if !strings.Contains(keepTogetherTemplateJSON, `"version": "1.2"`) {
		t.Fatal("the keep-together fixture declares the 1.2 key, so it must declare 1.2")
	}
	d, err := template.ParseDocument([]byte(keepTogetherTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := template.SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "1.2"`) {
		t.Fatalf("re-serializing the fixture must keep 1.2, never raise it to the library's ceiling:\n%s", out)
	}
	if !strings.Contains(string(out), `"keepTogether": "signature"`) {
		t.Fatalf("the tag must round-trip:\n%s", out)
	}
}

// TestKeepTogetherGoldenFixture is the byte-identity half. It runs AFTER
// the semantic assertions above in file order and in intent (D-000.22).
func TestKeepTogetherGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "keep-together")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != keepTogetherTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/keepTogetherTemplateJSON (keep_together_template.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per alignment-rounding's precedent)",
			inputPath,
		)
	}

	fixture := loadExpectedFixture(t, filepath.Join(dir, "expected.json"))
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not 64 lower-case hex characters", fixture.SHA256)
	}

	b := renderKeepTogether(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/keep-together/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/keep-together). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}
