package folio8

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// STORY 12.2: THE DOCUMENT DECLARES ITS LOCALE AND ITS UTC OFFSET.
//
// Document.Locale and Document.UTCOffset had a loader, two consumers
// (render.go's two expr.NewFormatContext sites) and NO WRITER anywhere in the
// product. Everything below is about the writer: what it accepts, what it
// refuses, whether the refusal is located, and whether the document is
// byte-unchanged afterwards.
//
// THE OFFSET'S ACCEPT-SET IS D-12.C'S. The loader's utcOffsetPattern used to
// admit `+99:99`, which internal/expr then refused at render; D-12.C repaired
// the loader rather than have the command range-check on top of it, so
// `template.IsUTCOffset` is the single predicate both doors ask and the rows
// below are the loader's own answers, not a second opinion. `+99:99` and
// `+24:00` are in the REFUSE column by that ruling. `Z` is refused here and
// admitted by internal/expr's parseUTCOffsetMinutes; that one asymmetry is
// asserted, with its reason, at internal/expr/offset_divergence_test.go.
//
// EVERY ACCEPTING TEST ASSERTS THE OTHER FIELD IS UNCHANGED IN THE SERIALIZED
// BYTES. "A byte moved" is true of a command that wrote the wrong field, and
// the rotation this suite exists to catch — setDocumentLocale writing
// UTCOffset — is invisible to any assertion that reads only the field the
// command names.
//
// FIELDS ARE READ BACK AS RAW JSON LITERALS, never decoded into `any`:
// internal/arch_test.go's TestNoFloat64UnderModule scans _test.go files too,
// and `json.Unmarshal` into `any` is exactly how a float64 reaches a test that
// never spells one.

// documentSettingsDocument is the one fixture builder. The two fields it
// parameterises are the two the story is about, and they are DIFFERENT from
// each other in every call below so a command that wrote the wrong one could
// not pass by coincidence.
func documentSettingsDocument(locale, utcOffset string) []byte {
	return []byte(`{"version":"1.0","locale":"` + locale + `","utcOffset":"` + utcOffset + `",` +
		`"page":{"margin":{"top":36,"right":36,"bottom":36,"left":36},"orientation":"portrait","size":"A4"},` +
		`"fonts":{},"bands":{"content":{"elements":[]},` +
		`"pageFooter":{"elements":[],"height":20},` +
		`"pageHeader":{"elements":[],"height":20}},` +
		`"assets":{},"nextId":9}`)
}

func documentSettingsTemplate(t *testing.T, doc []byte) *Template {
	t.Helper()
	tpl, err := ParseTemplate(doc)
	if err != nil {
		t.Fatalf("fixture precondition: the document must load before the command can be measured: %v", err)
	}
	return tpl
}

// documentSettingsField returns the RAW JSON literal a top-level document key
// serializes to, and whether the key is present at all. Raw bytes rather than a
// decoded value: the assertion is about what is IN the file.
func documentSettingsField(t *testing.T, canonical []byte, key string) (string, bool) {
	t.Helper()
	var document map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &document); err != nil {
		t.Fatal(err)
	}
	value, ok := document[key]
	return string(value), ok
}

// documentSettingsPair reads BOTH fields at once, because every claim this file
// makes about one of them is only worth making beside the other.
func documentSettingsPair(t *testing.T, tpl *Template) (string, string) {
	t.Helper()
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	locale, okLocale := documentSettingsField(t, canonical, "locale")
	offset, okOffset := documentSettingsField(t, canonical, "utcOffset")
	if !okLocale || !okOffset {
		t.Fatalf("the serialized document lost a required key: locale present %v, utcOffset present %v\n%s", okLocale, okOffset, canonical)
	}
	return locale, offset
}

// documentSettingsRefusal asserts the three halves of every refusal row at
// once: the command was refused, the document is byte-identical afterwards, and
// the refusal is a *ComponentCommandError — an unlocated one reaches the author
// as pageSetupDiagnostic's fixed sentence with the field never named.
func documentSettingsRefusal(t *testing.T, tpl *Template, command string) *designer.ComponentCommandError {
	t.Helper()
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, applyErr := applyComponentCommand(tpl, []byte(command))
	if applyErr == nil {
		t.Fatalf("command unexpectedly succeeded: %s", command)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("a refused document-settings command mutated the document:\n%s\nbefore: %s\nafter:  %s", command, before, after)
	}
	var failure *designer.ComponentCommandError
	if !errors.As(applyErr, &failure) {
		t.Fatalf("refusal for %s is %T (%v), want *ComponentCommandError", command, applyErr, applyErr)
	}
	return failure
}

// TestSetDocumentLocaleWritesTheLocaleTheAuthorSet is the first row of the
// matrix and the whole point of the locale half: a tag nothing in the product
// could set before, set, projected, and serialized — with the OTHER field
// asserted unchanged in the same breath.
func TestSetDocumentLocaleWritesTheLocaleTheAuthorSet(t *testing.T) {
	tpl := documentSettingsTemplate(t, documentSettingsDocument("en", "+07:00"))
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentLocale","version":1,"locale":"th"}`))
	if err != nil {
		t.Fatalf("a locale in AD-12's closed set was refused: %v", err)
	}
	// A FRESH PROJECTION, carrying the new value. The panel seeds its row from
	// this and from nothing else, so a stale projection would show the author
	// the tag they replaced.
	if projection.Locale != template.LocaleTH {
		t.Fatalf("projected locale = %q, want %q", projection.Locale, template.LocaleTH)
	}
	if projection.UTCOffset != "+07:00" {
		t.Fatalf("projected utcOffset = %q, want the untouched +07:00", projection.UTCOffset)
	}
	locale, offset := documentSettingsPair(t, tpl)
	if locale != `"th"` {
		t.Fatalf("serialized locale = %s, want \"th\"", locale)
	}
	// THE HALF A ROTATION HIDES. "A byte moved" is equally true of an arm that
	// wrote UTCOffset instead; only this line can see that.
	if offset != `"+07:00"` {
		t.Fatalf("serialized utcOffset = %s, want the untouched \"+07:00\" — a setDocumentLocale command wrote the offset", offset)
	}
}

// TestSetDocumentLocaleAcceptsEveryTagInTheClosedSet is the non-vacuity half:
// the arm admits ALL FOUR of AD-12's tags, enumerated from template.LocaleTags
// rather than written out again here, so a fifth tag added in Go arrives in
// this loop without an edit and a predicate narrowed to one tag reddens.
func TestSetDocumentLocaleAcceptsEveryTagInTheClosedSet(t *testing.T) {
	if len(template.LocaleTags) == 0 {
		t.Fatal("presence precondition (D-000.9): template.LocaleTags is empty, so this loop asserts nothing")
	}
	for _, tag := range template.LocaleTags {
		t.Run(tag, func(t *testing.T) {
			tpl := documentSettingsTemplate(t, documentSettingsDocument("en", "+07:00"))
			if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentLocale","version":1,"locale":"`+tag+`"}`)); err != nil {
				t.Fatalf("locale %q is in template.LocaleTags and the command refused it: %v", tag, err)
			}
			locale, offset := documentSettingsPair(t, tpl)
			if locale != `"`+tag+`"` {
				t.Fatalf("serialized locale = %s, want %q", locale, tag)
			}
			if offset != `"+07:00"` {
				t.Fatalf("serialized utcOffset = %s, want the untouched \"+07:00\"", offset)
			}
		})
	}
}

// TestSetDocumentLocaleRefusesATagOutsideTheClosedSet is the Matrix's "locale
// outside AD-12's set" row. It asserts the LOCATED fields and that the sentence
// lists the legal tags — derived from template.LocaleTags, so a refusal cannot
// ship a message that lies about what is legal.
func TestSetDocumentLocaleRefusesATagOutsideTheClosedSet(t *testing.T) {
	tpl := documentSettingsTemplate(t, documentSettingsDocument("en", "+07:00"))
	failure := documentSettingsRefusal(t, tpl, `{"kind":"setDocumentLocale","version":1,"locale":"fr"}`)
	if failure.ElementID != "" {
		t.Errorf("a document-settings refusal is not addressed to an element, got ElementID %q", failure.ElementID)
	}
	if failure.DataPath != "locale" {
		t.Errorf("refusal DataPath = %q, want locale", failure.DataPath)
	}
	for _, tag := range template.LocaleTags {
		if !strings.Contains(failure.Message, tag) {
			t.Errorf("refusal = %q, want it to name the legal tag %q — the list is derived from template.LocaleTags", failure.Message, tag)
		}
	}
}

// TestSetDocumentLocaleRefusesAMalformedValue is the Matrix's "locale
// malformed" row: a non-string, an absent key, and the empty string. All three
// are refused by commandString and all three must arrive LOCATED on `locale`,
// because the plain error commandString itself returns would reach the author
// as the fixed page-setup sentence.
func TestSetDocumentLocaleRefusesAMalformedValue(t *testing.T) {
	for _, probe := range []struct {
		name    string
		command string
	}{
		{"a number", `{"kind":"setDocumentLocale","version":1,"locale":7}`},
		{"the key absent", `{"kind":"setDocumentLocale","version":1,"lcoale":"th"}`},
		{"the empty string", `{"kind":"setDocumentLocale","version":1,"locale":""}`},
		{"null", `{"kind":"setDocumentLocale","version":1,"locale":null}`},
	} {
		t.Run(probe.name, func(t *testing.T) {
			tpl := documentSettingsTemplate(t, documentSettingsDocument("en", "+07:00"))
			failure := documentSettingsRefusal(t, tpl, probe.command)
			if failure.ElementID != "" {
				t.Errorf("refusal ElementID = %q, want empty", failure.ElementID)
			}
			if failure.DataPath != "locale" {
				t.Errorf("refusal DataPath = %q, want locale", failure.DataPath)
			}
		})
	}
}

// TestSetDocumentLocaleIsRefusedByTheDoorsExistingGates keeps the arity and
// duplicate-key rows honest: they are refused by componentFields and
// refuseDuplicateCommandKeys BEFORE the handler runs, and componentFields
// returns a PLAIN error, so they cannot be asserted through
// documentSettingsRefusal's located-ness check.
func TestSetDocumentLocaleIsRefusedByTheDoorsExistingGates(t *testing.T) {
	for _, probe := range []struct {
		name string
		// located is true for the rows refuseDuplicateCommandKeys answers,
		// which DO produce a *ComponentCommandError — on `command`, not on
		// `locale`, because a duplicate key means the thing the command names
		// is exactly what cannot be trusted.
		located bool
		command string
	}{
		{"two keys", false, `{"kind":"setDocumentLocale","version":1}`},
		{"four keys", false, `{"kind":"setDocumentLocale","version":1,"locale":"th","utcOffset":"+09:00"}`},
		{"a repeated locale", true, `{"kind":"setDocumentLocale","version":1,"locale":"fr","locale":"th"}`},
	} {
		t.Run(probe.name, func(t *testing.T) {
			tpl := documentSettingsTemplate(t, documentSettingsDocument("en", "+07:00"))
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			_, applyErr := applyComponentCommand(tpl, []byte(probe.command))
			if applyErr == nil {
				t.Fatalf("the door accepted %s", probe.command)
			}
			after, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("a command the door refused still mutated the document")
			}
			var failure *designer.ComponentCommandError
			located := errors.As(applyErr, &failure)
			if located != probe.located {
				t.Fatalf("refusal for %s is located=%v (%T: %v), want located=%v", probe.command, located, applyErr, applyErr, probe.located)
			}
			if located && failure.DataPath != componentCommandPath {
				t.Fatalf("a duplicate-key refusal is located on %q, want %q", failure.DataPath, componentCommandPath)
			}
		})
	}
}

// TestSetDocumentLocaleResentUnchangedLeavesTheBytesIdentical is the Matrix's
// "value re-sent unchanged" row: the door ACCEPTS and writes, and the canonical
// bytes are equal — which is what makes wasm.Engine.Apply's own byte compare
// short-circuit to no revision, no undo entry and no dirty flag.
func TestSetDocumentLocaleResentUnchangedLeavesTheBytesIdentical(t *testing.T) {
	tpl := documentSettingsTemplate(t, documentSettingsDocument("th", "+07:00"))
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentLocale","version":1,"locale":"th"}`)); err != nil {
		t.Fatalf("re-sending the locale already in force was refused: %v", err)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("re-sending the current locale moved the bytes:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestUneditedDocumentKeepsItsLocaleAndOffsetByteIdentically is the Matrix's
// "document never edited" row and AC5: open, serialize, and both keys are still
// present, unmoved and byte-identical. Canonical output is alphabetical, so
// `locale` and `utcOffset` sit either side of the document and a serializer
// change that dropped one would be invisible to any test reading only the other.
func TestUneditedDocumentKeepsItsLocaleAndOffsetByteIdentically(t *testing.T) {
	document := documentSettingsDocument("th", "+07:00")
	tpl := documentSettingsTemplate(t, document)
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	again, err := SerializeTemplate(documentSettingsTemplate(t, canonical))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, again) {
		t.Fatalf("an unedited document did not round-trip:\nfirst:  %s\nsecond: %s", canonical, again)
	}
	locale, offset := documentSettingsPair(t, tpl)
	if locale != `"th"` || offset != `"+07:00"` {
		t.Fatalf("an unedited document reports locale=%s utcOffset=%s, want \"th\" and \"+07:00\"", locale, offset)
	}
}

// TestCanvasProjectsTheDocumentsLocaleAndOffset is AC1's engine half: the panel
// reads both rows off the projection and off nothing else, so the projection
// has to CARRY both — and carry the document's own values rather than a default.
//
// Two different documents, because a projection hard-coding "en"/"+00:00" would
// satisfy a single-document assertion.
func TestCanvasProjectsTheDocumentsLocaleAndOffset(t *testing.T) {
	for _, probe := range []struct{ locale, offset string }{
		{"en", "+00:00"},
		{"th", "+07:00"},
		{"ja", "-05:30"},
	} {
		t.Run(probe.locale, func(t *testing.T) {
			projection, err := canvas(documentSettingsTemplate(t, documentSettingsDocument(probe.locale, probe.offset)))
			if err != nil {
				t.Fatal(err)
			}
			if projection.Locale != probe.locale {
				t.Errorf("projected locale = %q, want %q", projection.Locale, probe.locale)
			}
			if projection.UTCOffset != probe.offset {
				t.Errorf("projected utcOffset = %q, want %q", projection.UTCOffset, probe.offset)
			}
		})
	}
}

// TestSetDocumentUTCOffsetWritesTheOffsetTheAuthorSet is the offset half's
// accepting row, with the OTHER field asserted unchanged in the same breath —
// the assertion the rotation this suite exists to catch cannot survive.
func TestSetDocumentUTCOffsetWritesTheOffsetTheAuthorSet(t *testing.T) {
	tpl := documentSettingsTemplate(t, documentSettingsDocument("th", "+00:00"))
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+07:00"}`))
	if err != nil {
		t.Fatalf("a legal ±HH:MM offset was refused: %v", err)
	}
	if projection.UTCOffset != "+07:00" {
		t.Fatalf("projected utcOffset = %q, want +07:00", projection.UTCOffset)
	}
	if projection.Locale != template.LocaleTH {
		t.Fatalf("projected locale = %q, want the untouched %q", projection.Locale, template.LocaleTH)
	}
	locale, offset := documentSettingsPair(t, tpl)
	if offset != `"+07:00"` {
		t.Fatalf("serialized utcOffset = %s, want \"+07:00\"", offset)
	}
	if locale != `"th"` {
		t.Fatalf("serialized locale = %s, want the untouched \"th\" — a setDocumentUTCOffset command wrote the locale", locale)
	}
}

// loaderUTCOffsetProbeRow is one row of internal/template's OWN probe table,
// read out of that package's source text.
type loaderUTCOffsetProbeRow struct {
	value string
	admit bool
}

// loaderUTCOffsetProbeTable extracts internal/template's utcOffsetProbes rows.
// It is deliberately the LOADER'S OWN TABLE and not a copy of it kept here:
// `internal/template`'s probe list lives in a `_test.go` file, so no import can
// reach it, and a second list written out beside this test is how the command
// door ends up probed over a strictly smaller set than the door it claims to
// match. Reading the authority's own source is this repo's shipped idiom for
// exactly that problem — canvas_projection_wire_test.go reads
// engine-protocol.ts's guard list the same way, for the same reason.
//
// A ROW ADDED IN internal/template ARRIVES HERE WITHOUT AN EDIT.
var loaderUTCOffsetProbeTable = regexp.MustCompile(`(?ms)^var utcOffsetProbes = \[\]struct \{.*?\n\}\{(.*?)\n\}$`)

// loaderUTCOffsetProbeRowPattern matches one `{"value", admit, "why"}` entry
// and captures the first two fields. The value is captured as a Go
// double-quoted literal and passed through strconv.Unquote, so a row carrying
// an escape — or a leading or trailing space, as two rows do — survives the
// extraction as the exact bytes the loader was probed with. A row written as a
// raw-string literal would not match; that is deliberate rather than an
// oversight, because it would also mean the authority's table had changed shape
// and the non-vacuity floor below is what says so.
var loaderUTCOffsetProbeRowPattern = regexp.MustCompile(`\{("(?:[^"\\]|\\.)*"), (true|false),`)

func loaderUTCOffsetProbes(t *testing.T) []loaderUTCOffsetProbeRow {
	t.Helper()
	path := filepath.Join(repoRootFromTest(t), "folio8-go", "internal", "template", "closedsets_test.go")
	source, err := os.ReadFile(path)
	if err != nil {
		// Not a skip. The other side of this seam is the authority this test
		// exists to agree with, and a missing one is a finding, not an excuse.
		t.Fatalf("read the loader's own offset probe table: %v", err)
	}
	block := loaderUTCOffsetProbeTable.FindSubmatch(source)
	if block == nil {
		t.Fatal("internal/template/closedsets_test.go no longer declares a utcOffsetProbes table this test can read; if it was restructured, re-derive this extraction rather than writing a second list here")
	}
	var rows []loaderUTCOffsetProbeRow
	for _, match := range loaderUTCOffsetProbeRowPattern.FindAllSubmatch(block[1], -1) {
		value, err := strconv.Unquote(string(match[1]))
		if err != nil {
			t.Fatalf("probe row %q does not unquote: %v", match[1], err)
		}
		rows = append(rows, loaderUTCOffsetProbeRow{value: value, admit: string(match[2]) == "true"})
	}
	return rows
}

// TestSetDocumentUTCOffsetAgreesWithTheLoader is the accept-set, asserted
// against the LOADER rather than against a second table: for every row of
// internal/template's OWN probe table, the command accepting it and
// template.IsUTCOffset admitting it are the same answer. D-12.C's whole result
// is that these two cannot disagree, and a table written out here again would
// be exactly the second opinion the ruling dissolved — and, measurably, a
// smaller one: the list this test used to keep omitted the leading-space and
// non-digit rows the loader's table carries.
func TestSetDocumentUTCOffsetAgreesWithTheLoader(t *testing.T) {
	probes := loaderUTCOffsetProbes(t)
	// NON-VACUITY FIRST, in three ways, because a regexp that quietly stopped
	// matching would make every assertion below true and meaningless.
	if len(probes) < 15 {
		t.Fatalf("extracted %d rows from the loader's probe table; it carried 18 when this test was written and a sudden collapse means the extraction broke, not that the table shrank", len(probes))
	}
	extracted := map[string]bool{}
	for _, probe := range probes {
		extracted[probe.value] = true
	}
	// The four rows whose presence this test's whole point depends on: the
	// D-12.C value, the two syntactic shapes the old inline list omitted, and
	// the RFC 3339 spelling that must NOT be admitted by a document.
	for _, required := range []string{"+99:99", " +07:00", "+07:0a", "Z"} {
		if !extracted[required] {
			t.Fatalf("the loader's probe table no longer carries %q; the extraction is reading the wrong thing, or the authority lost a row this door must be probed with", required)
		}
	}
	accepted, refused := 0, 0
	for _, probe := range probes {
		// The row's OWN recorded answer is not trusted either: the predicate is
		// asked directly, so a stale `admit` column in the authority's table
		// cannot make this test agree with a wrong expectation.
		want := template.IsUTCOffset(probe.value)
		if want != probe.admit {
			t.Errorf("the loader's probe table records %q as admit=%v and template.IsUTCOffset says %v — the authority's own table has drifted from the authority", probe.value, probe.admit, want)
		}
		if want {
			accepted++
		} else {
			refused++
		}
		t.Run(probe.value, func(t *testing.T) {
			tpl := documentSettingsTemplate(t, documentSettingsDocument("th", "+00:00"))
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			_, applyErr := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"`+probe.value+`"}`))
			if (applyErr == nil) != want {
				t.Fatalf("the command %s %q and template.IsUTCOffset %s it — the two doors have drifted (D-12.C)",
					map[bool]string{true: "ACCEPTED", false: "REFUSED"}[applyErr == nil], probe.value,
					map[bool]string{true: "ADMITS", false: "REFUSES"}[want])
			}
			if want {
				return
			}
			// A refused command leaves the document byte-identical and arrives
			// LOCATED on the field the author has to change.
			after, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("a refused offset command mutated the document:\nbefore: %s\nafter:  %s", before, after)
			}
			var failure *designer.ComponentCommandError
			if !errors.As(applyErr, &failure) {
				t.Fatalf("refusal for %q is %T (%v), want *ComponentCommandError", probe.value, applyErr, applyErr)
			}
			if failure.ElementID != "" {
				t.Errorf("refusal ElementID = %q, want empty", failure.ElementID)
			}
			if failure.DataPath != "utcOffset" {
				t.Errorf("refusal DataPath = %q, want utcOffset", failure.DataPath)
			}
			// THE ONE ±HH:MM AUTHORITY, rendered rather than re-typed. The empty
			// string is refused one gate earlier, by commandString, whose
			// sentence names the field instead.
			if probe.value != "" && !strings.Contains(failure.Message, template.UTCOffsetSyntax) {
				t.Errorf("refusal = %q, want it to carry %s", failure.Message, template.UTCOffsetSyntax)
			}
		})
	}
	// Non-vacuity in both directions: a probe list that was all-legal or
	// all-illegal would let a command stuck at one answer pass this test.
	if accepted == 0 || refused == 0 {
		t.Fatalf("the extracted probe list is %d accepted / %d refused; both must be non-zero", accepted, refused)
	}
}

// TestSetDocumentUTCOffsetRefusesAMalformedValue is the Matrix's malformed row:
// a non-string, an absent key, the empty string and null. All four are refused
// by commandString, and all four must arrive LOCATED on `utcOffset`.
func TestSetDocumentUTCOffsetRefusesAMalformedValue(t *testing.T) {
	for _, probe := range []struct {
		name    string
		command string
	}{
		{"a number", `{"kind":"setDocumentUTCOffset","version":1,"utcOffset":7}`},
		{"the key absent", `{"kind":"setDocumentUTCOffset","version":1,"utcOfset":"+07:00"}`},
		{"the empty string", `{"kind":"setDocumentUTCOffset","version":1,"utcOffset":""}`},
		{"null", `{"kind":"setDocumentUTCOffset","version":1,"utcOffset":null}`},
	} {
		t.Run(probe.name, func(t *testing.T) {
			tpl := documentSettingsTemplate(t, documentSettingsDocument("th", "+00:00"))
			failure := documentSettingsRefusal(t, tpl, probe.command)
			if failure.ElementID != "" {
				t.Errorf("refusal ElementID = %q, want empty", failure.ElementID)
			}
			if failure.DataPath != "utcOffset" {
				t.Errorf("refusal DataPath = %q, want utcOffset", failure.DataPath)
			}
		})
	}
}

// TestSetDocumentUTCOffsetIsRefusedByTheDoorsExistingGates is the offset half's
// door-gate row: arity and duplicate keys, refused before the handler runs.
func TestSetDocumentUTCOffsetIsRefusedByTheDoorsExistingGates(t *testing.T) {
	for _, probe := range []struct {
		name    string
		located bool
		command string
	}{
		{"two keys", false, `{"kind":"setDocumentUTCOffset","version":1}`},
		{"four keys", false, `{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+07:00","locale":"th"}`},
		{"a repeated utcOffset", true, `{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+99:99","utcOffset":"+07:00"}`},
	} {
		t.Run(probe.name, func(t *testing.T) {
			tpl := documentSettingsTemplate(t, documentSettingsDocument("th", "+00:00"))
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			_, applyErr := applyComponentCommand(tpl, []byte(probe.command))
			if applyErr == nil {
				t.Fatalf("the door accepted %s", probe.command)
			}
			after, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("a command the door refused still mutated the document")
			}
			var failure *designer.ComponentCommandError
			located := errors.As(applyErr, &failure)
			if located != probe.located {
				t.Fatalf("refusal for %s is located=%v (%T: %v), want located=%v", probe.command, located, applyErr, applyErr, probe.located)
			}
			if located && failure.DataPath != componentCommandPath {
				t.Fatalf("a duplicate-key refusal is located on %q, want %q", failure.DataPath, componentCommandPath)
			}
		})
	}
}

// TestSetDocumentUTCOffsetResentUnchangedLeavesTheBytesIdentical is the
// Matrix's "value re-sent unchanged" row for the offset.
func TestSetDocumentUTCOffsetResentUnchangedLeavesTheBytesIdentical(t *testing.T) {
	tpl := documentSettingsTemplate(t, documentSettingsDocument("th", "+07:00"))
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+07:00"}`)); err != nil {
		t.Fatalf("re-sending the offset already in force was refused: %v", err)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("re-sending the current offset moved the bytes:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestTheTwoArmsWriteTheirOwnFieldAndOnlyTheirOwn is the ROTATION guard stated
// as one claim rather than left implicit in two accepting tests: both commands
// applied in sequence to one document leave exactly the two values sent, and
// each applied alone leaves the other where it was. An arm writing its
// sibling's field passes "a byte moved" and fails here.
func TestTheTwoArmsWriteTheirOwnFieldAndOnlyTheirOwn(t *testing.T) {
	tpl := documentSettingsTemplate(t, documentSettingsDocument("en", "+00:00"))
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentLocale","version":1,"locale":"ja"}`)); err != nil {
		t.Fatalf("setDocumentLocale ja was refused: %v", err)
	}
	if locale, offset := documentSettingsPair(t, tpl); locale != `"ja"` || offset != `"+00:00"` {
		t.Fatalf("after the locale command the document reads locale=%s utcOffset=%s, want \"ja\" and the untouched \"+00:00\"", locale, offset)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+09:00"}`)); err != nil {
		t.Fatalf("setDocumentUTCOffset +09:00 was refused: %v", err)
	}
	if locale, offset := documentSettingsPair(t, tpl); locale != `"ja"` || offset != `"+09:00"` {
		t.Fatalf("after the offset command the document reads locale=%s utcOffset=%s, want the untouched \"ja\" and \"+09:00\"", locale, offset)
	}
	// AND A REFUSED SECOND COMMAND LEAVES THE FIRST ONE'S WRITE STANDING: the
	// two are independent commands, not one transaction, and an arm that rolled
	// its sibling back would be writing a field it does not name.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+99:99"}`)); err == nil {
		t.Fatal("the command accepted +99:99 (D-12.C)")
	}
	if locale, offset := documentSettingsPair(t, tpl); locale != `"ja"` || offset != `"+09:00"` {
		t.Fatalf("a refused offset command disturbed the document: locale=%s utcOffset=%s", locale, offset)
	}
}

// ---------------------------------------------------------------------------
// RED-PROOFS (D-000.14). Each was run by MUTATING THE SUBJECT — the production
// code in component_commands.go, page_setup.go and
// internal/template/closedsets.go — never the expectation, and each mutated
// line is recorded here so a later reader can repeat it exactly.
//
// Two failures are present in every run below and are NOT caused by these
// mutations: TestCorpusMeetsP6ExerciseFloors / P6g_(opaque_names), the mandated
// permanent red, and (while the designer side of the protocol had not yet
// moved) TestCanvasProjectionWireKeysAreTheOnesTheDesignerAccepts. Neither is
// counted below.
//
//  1. DELETE THE LOCALE PREDICATE CALL. Replacing setDocumentLocale's
//     `if !template.IsLocale(tag) { … }` block with `_ = tag`:
//     TestSetDocumentLocaleRefusesATagOutsideTheClosedSet FAILS ("command
//     unexpectedly succeeded"). NOTHING ELSE IN THE GO SUITE MOVES, and that is
//     the finding worth recording: Canvas(t) reads neither Locale nor
//     UTCOffset, so unlike Story 12.1's content-window case there is NO
//     trailing backstop to catch it. The arm's own validation is the only
//     refusal before the wasm layer's reparse, and the mutant writes
//     `"locale":"fr"` into the file.
//
//  2. SWAP THE TWO ARMS' WRITE TARGETS (the rotation). Replacing
//     setDocumentLocale's `previous := t.doc.Locale` / `t.doc.Locale = tag` /
//     `t.doc.Locale = previous` with the UTCOffset spellings, and
//     setDocumentUTCOffset's `previous := t.doc.UTCOffset` /
//     `t.doc.UTCOffset = offset` / `t.doc.UTCOffset = previous` with the Locale
//     ones — so each command writes its sibling's field:
//     EIGHT tests FAIL — TestSetDocumentLocaleWritesTheLocaleTheAuthorSet,
//     TestSetDocumentLocaleAcceptsEveryTagInTheClosedSet (all four subtests),
//     TestSetDocumentLocaleResentUnchangedLeavesTheBytesIdentical,
//     TestSetDocumentUTCOffsetWritesTheOffsetTheAuthorSet,
//     TestSetDocumentUTCOffsetResentUnchangedLeavesTheBytesIdentical,
//     TestTheTwoArmsWriteTheirOwnFieldAndOnlyTheirOwn and, end to end,
//     TestSetDocumentLocaleChangesWhatTheRendererDraws.
//     THE BOTH-FIELDS ASSERTION IS WHAT SEES IT. An accepting test reading only
//     the field its command names would report "the value did not change"
//     rather than "the other field did", and one asserting only "some byte
//     moved" would have stayed GREEN — which is exactly how this rotation
//     shipped green twice earlier in this run.
//
//  3. DELETE THE OFFSET PREDICATE CALL. Replacing setDocumentUTCOffset's
//     `if !template.IsUTCOffset(offset) { … }` block with `_ = offset`:
//     TestSetDocumentUTCOffsetAgreesWithTheLoader FAILS on eight of its
//     subtests (+99:99, +24:00, +00:60, Z, +7:00, +0700, 07:00, "+07:00 ") and
//     TestTheTwoArmsWriteTheirOwnFieldAndOnlyTheirOwn FAILS on its `+99:99`
//     row. Again nothing else moves: no backstop.
//
//  4. MAKE EACH ARM WRITE NOTHING. Deleting both assignments —
//     `t.doc.Locale = tag` -> `_ = tag` and `t.doc.UTCOffset = offset` ->
//     `_ = offset`, keeping the validation, the Canvas(t) call and the return:
//     TestSetDocumentLocaleWritesTheLocaleTheAuthorSet,
//     TestSetDocumentLocaleAcceptsEveryTagInTheClosedSet (th, zh-Hans and ja —
//     `en` legitimately stays green, because the fixture already declares it),
//     TestSetDocumentUTCOffsetWritesTheOffsetTheAuthorSet,
//     TestTheTwoArmsWriteTheirOwnFieldAndOnlyTheirOwn and
//     TestSetDocumentLocaleChangesWhatTheRendererDraws FAIL. That is the proof
//     the arms are REACHED and do the write, not merely that they are ordered
//     correctly in the switch.
//
//  5. DROP THE PROJECTION FIELDS. Removing `Locale: t.doc.Locale, UTCOffset:
//     t.doc.UTCOffset,` from Canvas's composite literal in page_setup.go:
//     TestCanvasProjectsTheDocumentsLocaleAndOffset FAILS on all three rows,
//     and both accepting tests fail on their projection assertions — while
//     TestCanvasProjectionWireKeysAreTheRecordedSet stays GREEN, because the
//     KEYS still come from the struct tags. Which is why both exist: the wire
//     record pins the key set and cannot see a field that is always empty.
//
//  6. REVERT THE LOADER'S PATTERN (D-12.C's own red-proof, and it is about
//     REACHABILITY rather than about the regexp). Restoring
//     internal/template/closedsets.go's `utcOffsetPattern` to the pre-D-12.C
//     `^[+-][0-9]{2}:[0-9]{2}$`:
//     internal/template's TestIsUTCOffsetMatchesTheLoader and
//     TestUTCOffsetLoadRefusalIsReachableAndLocated (+99:99, +24:00, +00:60)
//     FAIL — the loader ADMITS the document again — and internal/expr's
//     TestLoaderAdmitsNothingTheEvaluatorCannotParse FAILS, which is the defect
//     itself: a file that opens and then renders no dates at all.
//     TestTheTwoArmsWriteTheirOwnFieldAndOnlyTheirOwn fails on its `+99:99`
//     row for the same reason.
//
//     TestSetDocumentUTCOffsetAgreesWithTheLoader STAYS GREEN under this
//     mutation, and that is the point of writing it against the loader rather
//     than against a table of its own: the command and the file door moved
//     TOGETHER, because they are one predicate. D-12.C's result is exactly
//     that they cannot disagree.
//
//  7. BREAK THE SHARED PROBE TABLE (this door's own extraction). Two
//     mutations, both in internal/template/closedsets_test.go, because the
//     extraction has two failure modes and a guard that reads source text
//     must fail loudly in both:
//
//     Deleting the `{" +07:00", false, "leading space"},` row: this test
//     FAILS on its required-rows check, naming the row — "the loader's probe
//     table no longer carries \" +07:00\"". Not silently over a shorter list,
//     which is how the inline list this replaced came to be missing that row
//     and `+07:0a` in the first place.
//
//     Renaming the table `utcOffsetProbes` -> `utcOffsetProbeRows`: this test
//     FAILS on the block regexp, saying the table is no longer declared where
//     it can be read and to re-derive the extraction rather than write a
//     second list here.
//
//  8. THE OFFSET NEVER REACHES THE RENDERER. Covered in
//     formatlocale_command_test.go rather than here:
//     TestSetDocumentUTCOffsetChangesWhatTheRendererDraws renders an instant
//     that falls on a DIFFERENT CALENDAR DAY at +00:00 and at +07:00, so a
//     write that lands in the file and is then ignored by the format context
//     is caught by a drawn glyph and not only by bytes on disk. Mutation 4
//     above is caught for the LOCALE half by that file's locale test; before
//     this test existed, the offset half's only witness was serialization.
