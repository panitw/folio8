package folio8_test

// STORY 12.2 / AC2, END TO END.
//
// AC2 is PRESERVATION, not construction: render.go already routes every
// formatDate and formatNumber through expr.NewFormatContext(doc.Locale,
// doc.UTCOffset), and internal/expr already carries the Buddhist-era offset for
// `th`. What had no writer was doc.Locale itself. So the claim this file makes
// is the one nothing else can make: a locale set BY COMMAND — not by
// hand-editing the file — reaches the renderer and changes the drawn glyphs.
//
// It is the only AC2 test that can fail, and it also catches the field rotation
// end to end: an arm that wrote UTCOffset instead of Locale leaves the drawn
// text in English while every "some byte moved" assertion stays green.
//
// IT RENDERS THE SAME DOCUMENT TWICE — before the command and after — because
// "the text is Thai" is a claim about a CHANGE, and a fixture that had always
// been Thai would satisfy it. The `en` render is the control.
//
// The helpers are formatlocale_test.go's: same package (folio8_test, no build
// tag), so formatLocaleTemplateJSON and formatLocaleExtractedText are reused
// rather than re-derived. See that file's header for why the /ToUnicode CMap
// decode lives in this package at all.

import (
	"encoding/json"
	"testing"

	folio8 "github.com/panitw/folio8/folio8-go"
	"github.com/panitw/folio8/folio8-go/fonts"
	"github.com/panitw/folio8/folio8-go/internal/designer"
)

// The instant, the pattern and the amount are formatlocale_test.go's own: the
// civil date is 15 August 2026 at +00:00, and `th` renders it in the Buddhist
// era as 2569. The chain carries Noto Sans Thai from the START, in the `en`
// document too, so the only thing that changes between the two renders is the
// locale — a chain edited alongside it would confound the two.
const (
	formatLocaleCommandChain   = `"Noto Sans", "Noto Sans Thai"`
	formatLocaleCommandPattern = `d MMMM yyyy`
	formatLocaleCommandData    = `{"when": "2026-08-15T00:00:00Z", "amount": 1234.56}`
	formatLocaleCommandEnglish = "15 August 2026 1,234.56"
	formatLocaleCommandThai    = "15 สิงหาคม 2569 1,234.56"
)

// THE OFFSET'S OWN INSTANT, and it is chosen so the OFFSET ALONE decides the
// answer. 2026-08-14T20:00:00Z is still 14 August at +00:00 and is already 15
// August at +07:00 — the two renders differ by a CALENDAR DAY, which no
// rounding, locale table or font chain can produce. An instant at midday would
// have drawn the same date under both offsets and made the test vacuous while
// looking identical.
const (
	formatOffsetCommandData    = `{"when": "2026-08-14T20:00:00Z", "amount": 1234.56}`
	formatOffsetCommandAtUTC   = "14 August 2026 1,234.56"
	formatOffsetCommandAtSeven = "15 August 2026 1,234.56"
)

func formatLocaleCommandRenderWith(t *testing.T, tpl *folio8.Template, data string) string {
	t.Helper()
	res, err := folio8.Render(tpl, folio8.Data(data), folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return formatLocaleExtractedText(t, res.Bytes)
}

func formatLocaleCommandRender(t *testing.T, tpl *folio8.Template) string {
	t.Helper()
	return formatLocaleCommandRenderWith(t, tpl, formatLocaleCommandData)
}

// TestSetDocumentLocaleChangesWhatTheRendererDraws is AC2.
func TestSetDocumentLocaleChangesWhatTheRendererDraws(t *testing.T) {
	source := formatLocaleTemplateJSON("en", "+00:00", formatLocaleCommandChain, formatLocaleCommandPattern)
	tpl, err := folio8.ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	// THE CONTROL, and it is a precondition rather than a bonus assertion: if
	// this document did not draw the English form to begin with, the Thai
	// assertion below would be a statement about a document that was already
	// Thai.
	if before := formatLocaleCommandRender(t, tpl); before != formatLocaleCommandEnglish {
		t.Fatalf("fixture precondition: the en document draws %q, want %q", before, formatLocaleCommandEnglish)
	}

	if _, err := designer.ApplyComponentCommand(tpl, []byte(`{"kind":"setDocumentLocale","version":1,"locale":"th"}`)); err != nil {
		t.Fatalf("setDocumentLocale th was refused: %v", err)
	}

	// THROUGH THE FILE, not through the in-memory template alone. The author's
	// next act is a save, and the value has to survive serialization and the
	// loader — which is also where a command that wrote a tag the loader refuses
	// would finally surface, one save later, with nothing to attribute it to.
	canonical, err := folio8.SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("SerializeTemplate: %v", err)
	}
	// The two keys are read as RAW JSON LITERALS out of the parsed document
	// rather than by substring: canonical output is indented, so a `Contains`
	// on `"locale":"th"` would be a test of the serializer's whitespace.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &top); err != nil {
		t.Fatal(err)
	}
	if string(top["locale"]) != `"th"` {
		t.Fatalf("the serialized document declares locale %s, want \"th\":\n%s", top["locale"], canonical)
	}
	// AC2's other half: the offset is byte-unchanged by a locale command.
	if string(top["utcOffset"]) != `"+00:00"` {
		t.Fatalf("a setDocumentLocale command moved utcOffset to %s:\n%s", top["utcOffset"], canonical)
	}
	reparsed, err := folio8.ParseTemplate(canonical)
	if err != nil {
		t.Fatalf("the document the command produced does not load: %v", err)
	}

	// AND THE GLYPHS THEMSELVES, decoded back out of the PDF's own /ToUnicode
	// CMap — the artifact Render actually produced, never a constant the test
	// already knew (D-000.21). The Buddhist-era year 2569 is the assertion that
	// a locale-blind fallback could not satisfy.
	after := formatLocaleCommandRender(t, reparsed)
	if after != formatLocaleCommandThai {
		t.Fatalf("after setDocumentLocale th the document draws %q, want %q", after, formatLocaleCommandThai)
	}
	if after == formatLocaleCommandEnglish {
		t.Fatal("the command changed nothing the renderer reads")
	}
}

// TestSetDocumentUTCOffsetChangesWhatTheRendererDraws is AC2's OTHER HALF, and
// it exists because nothing else in this suite proves a `setDocumentUTCOffset`
// reaches the renderer at all.
//
// The locale arm has TestSetDocumentLocaleChangesWhatTheRendererDraws above;
// the offset arm had only serialization assertions, and the red-proof block in
// document_settings_command_test.go says so in as many words — mutation 4 ("make
// each arm write nothing") is caught for the offset by bytes on disk and never
// by a drawn glyph. A field that is written to the file and then ignored by the
// renderer would pass every one of those tests.
//
// THE ASSERTION IS A DIFFERENT CALENDAR DAY, not a different clock time. The
// pattern this document carries is `d MMMM yyyy`, which prints no hours at all,
// so an offset that failed to reach the format context would draw a date that
// is simply WRONG by one day rather than one that looks plausibly rounded.
func TestSetDocumentUTCOffsetChangesWhatTheRendererDraws(t *testing.T) {
	source := formatLocaleTemplateJSON("en", "+00:00", formatLocaleCommandChain, formatLocaleCommandPattern)
	tpl, err := folio8.ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	// THE CONTROL, as a precondition: at +00:00 the instant is still the 14th.
	// If this document did not draw the 14th to begin with, the assertion below
	// would be a statement about a document that already read +07:00.
	if before := formatLocaleCommandRenderWith(t, tpl, formatOffsetCommandData); before != formatOffsetCommandAtUTC {
		t.Fatalf("fixture precondition: the +00:00 document draws %q, want %q", before, formatOffsetCommandAtUTC)
	}

	if _, err := designer.ApplyComponentCommand(tpl, []byte(`{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+07:00"}`)); err != nil {
		t.Fatalf("setDocumentUTCOffset +07:00 was refused: %v", err)
	}

	// THROUGH THE FILE, exactly as the locale half goes: the author's next act
	// is a save, and the value has to survive serialization and the loader.
	canonical, err := folio8.SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("SerializeTemplate: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &top); err != nil {
		t.Fatal(err)
	}
	if string(top["utcOffset"]) != `"+07:00"` {
		t.Fatalf("the serialized document declares utcOffset %s, want \"+07:00\":\n%s", top["utcOffset"], canonical)
	}
	// The other half of the rotation guard, end to end: a locale command was
	// never sent, so the locale is byte-unchanged.
	if string(top["locale"]) != `"en"` {
		t.Fatalf("a setDocumentUTCOffset command moved locale to %s:\n%s", top["locale"], canonical)
	}
	reparsed, err := folio8.ParseTemplate(canonical)
	if err != nil {
		t.Fatalf("the document the command produced does not load: %v", err)
	}

	after := formatLocaleCommandRenderWith(t, reparsed, formatOffsetCommandData)
	if after != formatOffsetCommandAtSeven {
		t.Fatalf("after setDocumentUTCOffset +07:00 the document draws %q, want %q — the offset did not reach the format context", after, formatOffsetCommandAtSeven)
	}
	if after == formatOffsetCommandAtUTC {
		t.Fatal("the command changed nothing the renderer reads")
	}
}
