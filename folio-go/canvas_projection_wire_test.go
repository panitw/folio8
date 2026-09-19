package folio8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

// This file guards the ONE SEAM neither side's own tests can see: the JSON
// key set CanvasProjection actually puts on the wire, against the exact key
// set the browser will accept.
//
// WHY IT IS NEEDED, and it is not hypothetical — Story 7.9 renamed a field
// across this seam. Go's tests read the STRUCT FIELD
// (`projection.ContentWindowCountIsExact`), so they never see the json tag;
// the designer's tests hand-author their canvas fixtures as object literals
// in their own files, so they never see Go. Both sides can therefore agree
// with themselves while disagreeing with each other, and a one-sided rename
// is green twice over.
//
// WHAT THE FAILURE LOOKS LIKE IF IT SHIPS, which is why a merely mechanical
// check is worth this much prose. engine-protocol.ts's `hasOnly` is an
// EXACT-KEY check: a key Go sends that the list does not name makes isCanvas
// return false, parseInbound discards the WHOLE SNAPSHOT, and the canvas
// blanks with no attributable error — no diagnostic, no console line, nothing
// naming the field. That is the same class of silent-blank failure
// canvasWindowOrigins refuses a bad origin sequence to avoid, one layer up.
//
// The shape is internal/template/drift_test.go's: neither side is trusted as
// the definition. The Go side is EXTRACTED from what encoding/json actually
// emits (not read off the struct tags, which is the same file a rename would
// have edited), the TypeScript side is EXTRACTED from the guard's own key
// list in engine-protocol.ts, and the recorded literal below is a third,
// independent statement that both must equal — so a rename applied to both
// halves in one edit still reddens here until the record is updated
// deliberately.

// canvasProjectionWireKeys is the recorded key set, sorted. Every entry is a
// name the designer's isCanvas guard lists and Go's CanvasProjection emits.
// Changing this list is a PROTOCOL CHANGE: both the Go json tag and
// engine-protocol.ts's hasOnly list have to move with it, in the same commit.
var canvasProjectionWireKeys = []string{
	"bands",
	"commandHeight",
	"commandWidth",
	"components",
	"contentWindowCount",
	"contentWindowCountIsExact",
	"contentWindowHeight",
	"contentWindowOrigins",
	"contentWindowPages",
	"defaultFontSize",
	"defaultLineSpacing",
	"fontChains",
	"fontFamilies",
	"gridIncrement",
	"height",
	"locale",
	"marginBottom",
	"marginLeft",
	"marginRight",
	"marginTop",
	"orientation",
	"pageBreaks",
	"preset",
	"utcOffset",
	"width",
}

// canvasProjectionOptionalWireKeys are the top-level keys the designer's guard
// accepts that a projection carries only SOMETIMES — spec-section-break's
// `sectionBreak`, present exactly when the document declares a break. They are
// recorded apart from the always-present set above so that the zero-value
// identity check keeps its meaning for every other key, and each one has its
// own typed clause in isCanvas (hasOnly cannot see an absent key).
// SPEC-multi-pages story 5 adds the per-page pair, `sectionBreaks` and
// `sectionBreakAnchors`, present exactly on a projection with more than one
// designed page, in place of the one-page pair.
var canvasProjectionOptionalWireKeys = []string{"sectionBreak", "sectionBreakAnchor", "sectionBreakAnchors", "sectionBreaks"}

// canvasFontChainWireKeys is the recorded key set of the NESTED object, sorted.
// fontChains is the first nested object this projection carries, and the
// browser checks it with hasExactKeys — a check that is BOTH ways, so a field
// added to CanvasFontChain in Go and not to engine-protocol.ts makes isCanvas
// return false, and the symptom is the blank canvas this file's header
// describes, one level down where the top-level key list cannot see it.
var canvasFontChainWireKeys = []string{"entries", "name"}

// canvasFontChainEntryWireKeys is the recorded key set of the ENTRY object —
// one level below the chain — and it closes a gap that was MEASURED rather
// than supposed.
//
// The chain-level record above pins key NAMES only. It says nothing about an
// entry's value TYPE or nesting depth, so Story 8.3's change — CanvasFontChain
// .Entries going from []string to a slice of four-field structs — would have
// left that record green while the browser's own entry guard rejected every
// snapshot: isCanvas false, parseInbound undefined, engine-client terminates
// the worker, and the canvas is permanently blank with no element id and
// nothing to attribute it to. A key added to CanvasFontChainEntry in Go and
// not to engine-protocol.ts's entry guard does exactly the same thing, two
// levels down where neither existing record can see it.
//
// The browser's entry guard is hasExactKeys, so it rejects in BOTH
// directions: a key Go stops sending fails it as surely as a key Go starts
// sending.
var canvasFontChainEntryWireKeys = []string{"assetKey", "bold", "boldItalic", "face", "family", "italic", "style"}

// canvasTextFragmentWireKeys is the recorded key set of the PAINT FRAGMENT —
// three levels below the projection's top level, inside components ->
// textPaint -> lines -> fragments — and the level at which Story 8.4a's
// attribution travels.
//
// IT IS THE ACCEPTED SET, NOT THE EMITTED ONE, and that is the difference
// from the three records above. TWO of its keys are optional: `assetKey` is
// present exactly when the engine resolved that fragment to a face the
// document CARRIES, and `face` — Story 8.4e — exactly when it resolved one the
// caller SHIPPED. Neither is ever emitted for the other's population, so no
// fragment ever marshals the whole recorded set. The browser's fragment guard
// is `hasOnly`, a SUBSET check, so the accepted list is what this record pins
// on the TypeScript side, and the Go side is pinned in both directions against
// the two exact emission sets derived below.
//
// WHY THIS LEVEL NEEDED A RECORD AT ALL, measured rather than supposed. The
// three records above descend into `fontChains` and stop; NOTHING in this file
// or anywhere else in Go descended into `components`, `textPaint`, `lines` or
// `fragments`. So a fragment field added in Go and not in engine-protocol.ts
// reddened NOTHING here — and on the browser it is the sharpest failure this
// protocol has: `hasOnly` rejects the unlisted key, isCanvasTextPaint fails,
// isCanvas fails, parseInbound returns undefined, and engine-client raises
// PROTOCOL_INVALID, which TERMINATES THE WORKER, rejects the ready promise and
// rejects every pending request. That is not a blank canvas — the session is
// dead until reload, with no edit, save, undo or preview possible.
// TWO OPTIONAL KEYS SINCE STORY 8.4e, AND THEY ARE MUTUALLY EXCLUSIVE.
// `face` carries the SHIPPED face's FontSet name, `assetKey` the carried
// face's key, and exactly one of the two is non-empty on every emitted
// fragment — the same discriminated pair canvasFontChainEntryWireKeys already
// carries one level up. The Go side is therefore pinned in both directions
// against TWO exact sets rather than one set and a subtraction: a carried
// fragment marshals {assetKey,text,x} and a shipped one {face,text,x}, and
// neither list is the other minus a key.
var canvasTextFragmentWireKeys = []string{"assetKey", "face", "text", "x"}

// canvasTextFragmentAttributionKeys are the two optional, mutually exclusive
// members of the set above. Named rather than open-coded so the two exact
// emission sets below are each derived from the record, never restated.
var canvasTextFragmentAttributionKeys = []string{"assetKey", "face"}

// canvasTextFragmentWireKeysWithout is the record minus one of its optional
// attribution keys — the exact set a fragment of the OTHER population emits.
func canvasTextFragmentWireKeysWithout(key string) []string {
	var out []string
	for _, k := range canvasTextFragmentWireKeys {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

// marshalledCanvasKeys is the Go side, taken from the bytes rather than from
// the struct: whatever encoding/json puts on the wire for this value, sorted.
func marshalledCanvasKeys(t *testing.T, projection designer.CanvasProjection) []string {
	t.Helper()
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal CanvasProjection: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("unmarshal CanvasProjection: %v", err)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// TestCanvasProjectionWireKeysAreTheRecordedSet is the Go half. It marshals a
// REAL projection — one built from a template, through the entry point that
// reaches the browser — and, separately, the zero value, because the two can
// only differ if a field acquires an `omitempty` that would silently drop a
// key from a document that happens not to set it.
func TestCanvasProjectionWireKeysAreTheRecordedSet(t *testing.T) {
	projected := marshalledCanvasKeys(t, projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON)))
	if !reflect.DeepEqual(projected, canvasProjectionWireKeys) {
		t.Errorf("CanvasProjection marshals the keys\n\t%v\nand the recorded protocol set is\n\t%v", projected, canvasProjectionWireKeys)
	}
	zero := marshalledCanvasKeys(t, designer.CanvasProjection{})
	if !reflect.DeepEqual(zero, projected) {
		t.Errorf("a zero CanvasProjection marshals\n\t%v\nand a projected one\n\t%v — a key that appears only sometimes is a key the browser's exact-key guard will reject only sometimes", zero, projected)
	}
	// And the NESTED object, from the same bytes. A field added to
	// CanvasFontChain is invisible to the check above — the top-level key set
	// does not change — and the browser rejects it just as hard.
	chainKeys := marshalledObjectKeys(t, chainFromProjectionBytes(t, projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON))))
	if !reflect.DeepEqual(chainKeys, canvasFontChainWireKeys) {
		t.Errorf("CanvasFontChain marshals the keys\n\t%v\nand the recorded protocol set is\n\t%v", chainKeys, canvasFontChainWireKeys)
	}
	zeroChain := marshalledObjectKeys(t, mustMarshal(t, designer.CanvasFontChain{}))
	if !reflect.DeepEqual(zeroChain, chainKeys) {
		t.Errorf("a zero CanvasFontChain marshals\n\t%v\nand a projected one\n\t%v — an omitempty here drops a key the browser's exact-key guard requires", zeroChain, chainKeys)
	}
	// And the ENTRY, one level below that. Same reason, same failure mode:
	// invisible to both records above, and a blank canvas either way.
	entryKeys := marshalledObjectKeys(t, entryFromProjectionBytes(t, projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON))))
	if !reflect.DeepEqual(entryKeys, canvasFontChainEntryWireKeys) {
		t.Errorf("CanvasFontChainEntry marshals the keys\n\t%v\nand the recorded protocol set is\n\t%v", entryKeys, canvasFontChainEntryWireKeys)
	}
	zeroEntry := marshalledObjectKeys(t, mustMarshal(t, designer.CanvasFontChainEntry{}))
	if !reflect.DeepEqual(zeroEntry, entryKeys) {
		t.Errorf("a zero CanvasFontChainEntry marshals\n\t%v\nand a projected one\n\t%v — an entry key that appears only for SOME entries (a named face carries no family; an embedded one does) is one the browser's exact-key guard rejects only for some documents", zeroEntry, entryKeys)
	}
	// And the PAINT FRAGMENT, three levels down inside components, which none
	// of the three checks above can see. Both halves of the optional key are
	// asserted, from two documents that differ in exactly one thing: whether
	// the face the engine resolved is one the document carries.
	carriedKeys := marshalledObjectKeys(t, fragmentFromProjectionBytes(t, carriedFaceProjection(t, embeddedFontTemplateJSON())))
	if want := canvasTextFragmentWireKeysWithout("face"); !reflect.DeepEqual(carriedKeys, want) {
		t.Errorf("a CARRIED-face CanvasTextFragment marshals the keys\n\t%v\nand the recorded set minus the SHIPPED attribution key is\n\t%v — the two attribution keys are mutually exclusive, and a carried fragment that also named a face would ask the browser to rasterize with a face the document does not draw with", carriedKeys, want)
	}
	shippedKeys := marshalledObjectKeys(t, fragmentFromProjectionBytes(t, projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON))))
	if want := canvasTextFragmentWireKeysWithout("assetKey"); !reflect.DeepEqual(shippedKeys, want) {
		t.Errorf("a SHIPPED-face CanvasTextFragment marshals the keys\n\t%v\nand the recorded set minus the CARRIED attribution key is\n\t%v — a shipped fragment names the engine's own FontSet face (Story 8.4e) and never an asset key, and every non-optional key must always be present", shippedKeys, want)
	}
	// AND THE TWO ATTRIBUTION KEYS ARE THE ONLY OPTIONAL ONES, checked from
	// the two emission sets rather than asserted in prose: their union is the
	// record and their intersection is empty.
	for _, key := range canvasTextFragmentAttributionKeys {
		if slices.Contains(carriedKeys, key) == slices.Contains(shippedKeys, key) {
			t.Errorf("both populations agree about the optional key %q (carried %v, shipped %v) — exactly one of assetKey and face is emitted per fragment, and that exclusivity is the wire shape's premise", key, carriedKeys, shippedKeys)
		}
	}
}

// fragmentFromProjectionBytes digs the first painted fragment out of the
// marshalled projection, by the same rule the two helpers above use: from the
// bytes, never from the struct. The fixture must actually paint something, or
// the check asserts nothing.
func fragmentFromProjectionBytes(t *testing.T, projection designer.CanvasProjection) []byte {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(mustMarshal(t, projection), &object); err != nil {
		t.Fatalf("unmarshal CanvasProjection: %v", err)
	}
	var components []json.RawMessage
	if err := json.Unmarshal(object["components"], &components); err != nil {
		t.Fatalf("unmarshal components: %v", err)
	}
	for _, raw := range components {
		var component map[string]json.RawMessage
		if err := json.Unmarshal(raw, &component); err != nil {
			t.Fatalf("unmarshal a component: %v", err)
		}
		paint, ok := component["textPaint"]
		if !ok {
			continue
		}
		var textPaint map[string]json.RawMessage
		if err := json.Unmarshal(paint, &textPaint); err != nil {
			t.Fatalf("unmarshal a textPaint: %v", err)
		}
		var lines []json.RawMessage
		if err := json.Unmarshal(textPaint["lines"], &lines); err != nil {
			t.Fatalf("unmarshal a textPaint's lines: %v", err)
		}
		for _, rawLine := range lines {
			var line map[string]json.RawMessage
			if err := json.Unmarshal(rawLine, &line); err != nil {
				t.Fatalf("unmarshal a paint line: %v", err)
			}
			var fragments []json.RawMessage
			if err := json.Unmarshal(line["fragments"], &fragments); err != nil {
				t.Fatalf("unmarshal a paint line's fragments: %v", err)
			}
			if len(fragments) > 0 {
				return fragments[0]
			}
		}
	}
	t.Fatal("fixture precondition: the wire-key fixture projected no paint fragment, so the fragment-level key check asserts nothing")
	return nil
}

// entryFromProjectionBytes digs the first projected chain's first ENTRY out of
// the marshalled projection, by the same rule chainFromProjectionBytes uses:
// from the bytes, never from the struct. The fixture must declare a chain with
// at least one entry, or the check asserts nothing.
func entryFromProjectionBytes(t *testing.T, projection designer.CanvasProjection) []byte {
	t.Helper()
	var chain map[string]json.RawMessage
	if err := json.Unmarshal(chainFromProjectionBytes(t, projection), &chain); err != nil {
		t.Fatalf("unmarshal the first font chain: %v", err)
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(chain["entries"], &entries); err != nil {
		t.Fatalf("unmarshal the first font chain's entries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("fixture precondition: the wire-key fixture's first font chain must declare at least one entry, or the entry-level key check asserts nothing")
	}
	return entries[0]
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %T: %v", value, err)
	}
	return encoded
}

// marshalledObjectKeys is marshalledCanvasKeys' half that reads an object's
// keys, over raw bytes, so a NESTED object can be read the same way the
// top-level one is: from what encoding/json actually emitted, never from the
// struct tags a rename would have edited in the same breath.
func marshalledObjectKeys(t *testing.T, object []byte) []string {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(object, &fields); err != nil {
		t.Fatalf("unmarshal object %s: %v", object, err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// chainFromProjectionBytes digs the first projected font chain out of the
// marshalled projection. The fixture must declare one: a projection with no
// chains would let this whole check pass while saying nothing.
func chainFromProjectionBytes(t *testing.T, projection designer.CanvasProjection) []byte {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(mustMarshal(t, projection), &object); err != nil {
		t.Fatalf("unmarshal CanvasProjection: %v", err)
	}
	var chains []json.RawMessage
	if err := json.Unmarshal(object["fontChains"], &chains); err != nil {
		t.Fatalf("unmarshal fontChains: %v", err)
	}
	if len(chains) == 0 {
		t.Fatal("fixture precondition: the wire-key fixture must declare at least one font chain, or the nested key check asserts nothing")
	}
	return chains[0]
}

// canvasGuardKeyList extracts the key list engine-protocol.ts's isCanvas
// guard passes to hasOnly. It is deliberately the guard's OWN list and not a
// second copy of it kept here: a copy would be one more thing to forget to
// rename, which is the defect this file exists for.
var canvasGuardKeyList = regexp.MustCompile(`(?s)const isCanvas = .*?hasOnly\(value, \[(.*?)\]\)`)

// canvasGuardChainKeyList extracts the nested object's key list from the same
// guard, for the same reason and by the same rule: the guard's OWN list, never
// a copy of it kept here.
var canvasGuardChainKeyList = regexp.MustCompile(`hasExactKeys\(chain, \[(.*?)\]\)`)

// canvasGuardChainEntryKeyList extracts the ENTRY object's key list from the
// guard that checks it, for the same reason and by the same rule: the guard's
// OWN list, never a copy of it kept here. Anchored on isFontChainEntry's own
// name because `hasExactKeys(value, [...])` is the whole file's idiom and an
// unanchored match would read some other guard's list and compare it to this
// record, which is a green test asserting the wrong thing.
var canvasGuardChainEntryKeyList = regexp.MustCompile(`(?s)const isFontChainEntry = .*?hasExactKeys\(value, \[(.*?)\]\)`)

// canvasGuardFragmentKeyList extracts the PAINT FRAGMENT's key list from the
// guard that checks it, by the same rule: the guard's OWN list, never a copy
// of it kept here. Anchored on the `fragment` parameter name because
// `hasOnly(value, [...])` is the file's idiom and an unanchored match would
// read some other guard's list and compare it to this record — a green test
// asserting the wrong thing.
var canvasGuardFragmentKeyList = regexp.MustCompile(`hasOnly\(fragment, \[(.*?)\]\)`)

// TestCanvasProjectionWireKeysAreTheOnesTheDesignerAccepts is the TypeScript
// half, and the one that ties the two languages together. `hasOnly` rejects
// any key not on this list, so a Go-side rename that stops here — or a
// designer-side rename that Go never hears about — blanks the canvas.
func TestCanvasProjectionWireKeysAreTheOnesTheDesignerAccepts(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "folio-designer", "src", "engine-protocol.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		// Not a skip. The guard on the other side of this seam is part of the
		// protocol, and a missing one is a finding, not an excuse.
		t.Fatalf("read the designer's protocol guard: %v", err)
	}
	match := canvasGuardKeyList.FindSubmatch(source)
	if match == nil {
		t.Fatal("engine-protocol.ts no longer has an isCanvas guard whose hasOnly list this test can read; if the guard was restructured, re-derive this extraction rather than deleting the check")
	}
	keys := extractedKeyList(string(match[1]))
	accepted := slices.Sorted(slices.Values(append(slices.Clone(canvasProjectionWireKeys), canvasProjectionOptionalWireKeys...)))
	if !reflect.DeepEqual(keys, accepted) {
		t.Errorf("the designer's isCanvas guard accepts the keys\n\t%v\nand the recorded protocol set (always-present plus optional) is\n\t%v — one side of this seam has been renamed and the other has not, and the symptom is a blank canvas with nothing to attribute it to", keys, accepted)
	}
	// The NESTED object, whose guard is hasExactKeys rather than hasOnly and
	// so rejects in both directions: a key Go stops sending fails it as surely
	// as a key Go starts sending. Nothing else in either language sees this
	// pair — Go's own tests read the struct field, and the designer's read
	// hand-authored fixtures.
	chainMatch := canvasGuardChainKeyList.FindSubmatch(source)
	if chainMatch == nil {
		t.Fatal("engine-protocol.ts no longer checks the projected font chain's keys where this test can read it; if the guard was restructured, re-derive this extraction rather than deleting the check")
	}
	chainKeys := extractedKeyList(string(chainMatch[1]))
	if !reflect.DeepEqual(chainKeys, canvasFontChainWireKeys) {
		t.Errorf("the designer's font-chain guard accepts the keys\n\t%v\nand the recorded protocol set is\n\t%v — fontChains is the first NESTED object on this projection, and a field added to CanvasFontChain on one side only blanks the canvas exactly as a top-level one would", chainKeys, canvasFontChainWireKeys)
	}

	// The ENTRY object, one level below the chain — Story 8.3's addition, and
	// the level the two records above are blind to. They pin key NAMES only,
	// so an entry's shape could change from a string to an object with neither
	// of them noticing, while the browser rejected every snapshot.
	entryMatch := canvasGuardChainEntryKeyList.FindSubmatch(source)
	if entryMatch == nil {
		t.Fatal("engine-protocol.ts no longer checks the projected font chain ENTRY's keys where this test can read it; if the guard was restructured, re-derive this extraction rather than deleting the check")
	}
	entryKeys := extractedKeyList(string(entryMatch[1]))
	if !reflect.DeepEqual(entryKeys, canvasFontChainEntryWireKeys) {
		t.Errorf("the designer's font-chain ENTRY guard accepts the keys\n\t%v\nand the recorded protocol set is\n\t%v — a field added to CanvasFontChainEntry on one side only blanks the canvas exactly as a chain-level or a top-level one would, and neither of this file's other two records can see it", entryKeys, canvasFontChainEntryWireKeys)
	}

	// The PAINT FRAGMENT, three levels down inside components — Story 8.4a's
	// addition, and the level every record above is blind to. Its guard is
	// `hasOnly`, so what is pinned here is the ACCEPTED set: a key Go starts
	// sending that this list does not name terminates the designer's worker.
	fragmentMatch := canvasGuardFragmentKeyList.FindSubmatch(source)
	if fragmentMatch == nil {
		t.Fatal("engine-protocol.ts no longer checks the projected paint FRAGMENT's keys where this test can read it; if the guard was restructured, re-derive this extraction rather than deleting the check")
	}
	fragmentKeys := extractedKeyList(string(fragmentMatch[1]))
	if !reflect.DeepEqual(fragmentKeys, canvasTextFragmentWireKeys) {
		t.Errorf("the designer's paint-FRAGMENT guard accepts the keys\n\t%v\nand the recorded protocol set is\n\t%v — a fragment key added in Go and not here does not blank the canvas, it raises PROTOCOL_INVALID and terminates the worker, and the session is dead until reload", fragmentKeys, canvasTextFragmentWireKeys)
	}
}

// extractedKeyList turns a TypeScript array literal's inner text into the
// sorted key set it names.
func extractedKeyList(literal string) []string {
	var keys []string
	for _, field := range strings.Split(literal, ",") {
		key := strings.Trim(strings.TrimSpace(field), "'\"")
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// tableColumnsProjectionWireKeys is the FIFTH record on this seam, and the
// first one that is not part of CanvasProjection at all.
//
// WHY IT EXISTS ONLY NOW. TableColumnsProjection is the surface the TABLE
// EDITOR reads — per table, requested by element id when the editor opens —
// and until Story 12.3 it carried four keys that no story had reason to move:
// measured at 12.3's baseline, `isTableColumns` appeared in ZERO Go files and
// `TableColumnsProjection` in ZERO Go test files, against 12 and 38 for their
// canvas counterparts. So it was pinned by NOTHING while the canvas projection
// was pinned four ways. 12.3 puts sixteen load-bearing members on it, which
// makes it the first story that can break it, which makes adding the pin part
// of that story.
//
// ITS GUARD IS hasExactKeys, WHICH REJECTS IN BOTH DIRECTIONS — stricter than
// isCanvas's `hasOnly`, which is a subset check and accepts a key Go stops
// sending. So a member added in Go and not here, AND a key listed here that Go
// never sends, both fail.
//
// AND THE FAILURE IS NOT A BLANK CANVAS. The header comment at the top of this
// file describes isCanvas's symptom; this guard's is different and worse.
// Guard false -> parseInbound returns undefined -> engine-client.ts's
// #fail('PROTOCOL_INVALID') -> state `failed`, handlers detached,
// worker.terminate(), every pending request rejected, and NO RE-SPAWN EXISTS.
// On a FIRST table-editor open it is completely silent: openTableEditor's catch
// sets tableEditorError, which renders only inside <TableEditor>, which never
// mounts because the editor never opened. The session is dead and nothing says
// so.
var tableColumnsProjectionWireKeys = []string{
	"alias",
	"altRowBackground",
	"collection",
	"columns",
	"headerAlign",
	"headerAlignResolved",
	"headerBackground",
	"headerBackgroundResolved",
	"headerBold",
	"headerBoldResolved",
	// STORY 14.8's THREE PAIRS. The DOT is part of the key, not a typo: the
	// command spells the authorable attributes `border.width`, `border.color`
	// and `border.edges` so that its located path is one the document has, and
	// table_header_style_test.go derives these key names from those field names.
	// `extractedKeyList` splits the TypeScript literal on COMMAS, so a dotted
	// key extracts whole; a key with a comma in it would not.
	"headerBorder.color",
	"headerBorder.colorResolved",
	"headerBorder.edges",
	"headerBorder.edgesResolved",
	"headerBorder.width",
	"headerBorder.widthResolved",
	"headerColor",
	"headerColorResolved",
	"headerFontFamily",
	"headerFontFamilyResolved",
	"headerFontSize",
	"headerFontSizeResolved",
	"headerHeight",
	"headerItalic",
	"headerItalicResolved",
	"headerLineSpacing",
	"headerLineSpacingResolved",
	"headerValign",
	"headerValignResolved",
	// SPEC-table-rules: the interior lines and the ruled area's floor.
	"minHeight",
	// The table's own cell padding (left/right) and whether headerStyle.padding
	// takes the header row over — the Table Editor's CELL PADDING boxes.
	"paddingHeaderOverride",
	"paddingLeft",
	"paddingRight",
	"rules.between",
	"rules.color",
	"rules.colorResolved",
	"rules.width",
	"rules.widthResolved",
	"sizing",
	"tableId",
	"totalWidth",
}

// tableProjectionGuardKeyList extracts the key list engine-protocol.ts's
// isTableColumns guard passes to hasExactKeys for the TABLE object.
//
// Anchored on `value.table` rather than on a bare parameter name, deliberately:
// `hasExactKeys(value, [...])` is this file's whole idiom, and the two existing
// nested extractors are anchored on the identifiers `chain` and `fragment`
// ALONE. Reusing either name in a new guard would silently point an existing
// regexp at the wrong list — a green test asserting the wrong thing, which is
// the exact defect this file exists to prevent.
var tableProjectionGuardKeyList = regexp.MustCompile(`hasExactKeys\(value\.table, \[(.*?)\]\)`)

// projectedTableForWireKeys builds a REAL table projection — through
// ApplyComponentCommand and TableColumns, the same path the editor takes — and
// puts a header style on it, so the recorded set is checked against what the
// engine actually emits for a table that uses these members rather than against
// a struct literal.
//
// IT SETS ALL TWELVE HEADER-STYLE FIELDS, not one. It used to set only
// `altRowBackground` and `fontSize` while its own comment claimed a table "that
// uses these members", leaving five committed members zero-valued in the very
// fixture the wire-key record is measured against. The key SET does not depend
// on the values — nothing here carries `omitempty`, which is the property the
// zero-value comparison below exists to hold — but a fixture that says it
// exercises the members and does not is a fixture that will be believed by the
// next reader.
func projectedTableForWireKeys(t *testing.T) designer.TableColumnsProjection {
	t.Helper()
	tpl := componentTemplate(t)
	before, err := canvas(tpl)
	if err != nil {
		t.Fatalf("project the fixture before creating a table: %v", err)
	}
	after, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatalf("create a table: %v", err)
	}
	table := newProjectedComponent(t, before, after)
	for _, command := range []string{
		`{"kind":"setTableHeaderHeight","version":1,"id":"` + table.ID + `","height":18}`,
		`{"kind":"setTableAltRowBackground","version":1,"id":"` + table.ID + `","op":"set","value":"#DDEEFF"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"fontFamily","op":"set","value":"body"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"fontSize","op":"set","value":14}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"lineSpacing","op":"set","value":1.5}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"background","op":"set","value":"#101010"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"color","op":"set","value":"#c81e1e"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"valign","op":"set","value":"middle"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"align","op":"set","value":"center"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"bold","op":"set","value":true}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"italic","op":"set","value":true}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"border.width","op":"set","value":2}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"border.color","op":"set","value":"#334455"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"` + table.ID + `","field":"border.edges","op":"set","value":["top","bottom"]}`,
	} {
		if _, err := applyComponentCommand(tpl, []byte(command)); err != nil {
			t.Fatalf("apply %s: %v", command, err)
		}
	}
	projection, err := tableColumns(tpl, table.ID)
	if err != nil {
		t.Fatalf("project the table columns: %v", err)
	}
	// AND THE CLAIM ABOVE IS CHECKED RATHER THAN ASSERTED IN PROSE: every
	// committed member this fixture says it exercises is non-zero. A comment
	// that outlives the commands it describes is how the fixture drifted the
	// first time.
	if projection.HeaderHeight == 0 || projection.AltRowBackground == "" ||
		projection.HeaderFontFamily == "" || projection.HeaderFontSize == 0 ||
		projection.HeaderLineSpacing == 0 || projection.HeaderBackground == "" ||
		projection.HeaderColor == "" || projection.HeaderValign == "" || projection.HeaderAlign == "" ||
		!projection.HeaderBold || !projection.HeaderItalic ||
		projection.HeaderBorderWidth == "" || projection.HeaderBorderColor == "" || projection.HeaderBorderEdges == "" {
		t.Fatalf("the wire-key fixture leaves a committed member zero-valued, so the record is measured against a table that does not use it: %#v", projection)
	}
	return projection
}

// TestTableColumnsProjectionWireKeysAreTheRecordedSet is the Go half of the
// fifth record: what encoding/json actually emits for a real projection, and
// for the zero value, because the two can differ only if a member acquires an
// `omitempty` that would drop a key for exactly the documents that do not set
// it — which under hasExactKeys is a dead session for exactly those documents.
func TestTableColumnsProjectionWireKeysAreTheRecordedSet(t *testing.T) {
	projected := marshalledObjectKeys(t, mustMarshal(t, projectedTableForWireKeys(t)))
	if !reflect.DeepEqual(projected, tableColumnsProjectionWireKeys) {
		t.Errorf("TableColumnsProjection marshals the keys\n\t%v\nand the recorded protocol set is\n\t%v", projected, tableColumnsProjectionWireKeys)
	}
	zero := marshalledObjectKeys(t, mustMarshal(t, designer.TableColumnsProjection{}))
	if !reflect.DeepEqual(zero, projected) {
		t.Errorf("a zero TableColumnsProjection marshals\n\t%v\nand a projected one\n\t%v — a key that appears only sometimes is a key the browser's exact-key guard rejects only sometimes, and the symptom is a terminated worker on exactly those documents", zero, projected)
	}
}

// TestTableColumnsProjectionWireKeysAreTheOnesTheDesignerAccepts is the
// TypeScript half. isTableColumns checks the table object with hasExactKeys, so
// this pins BOTH directions at once: a member Go adds and the guard does not
// list, and a key the guard lists that Go does not send.
func TestTableColumnsProjectionWireKeysAreTheOnesTheDesignerAccepts(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "folio-designer", "src", "engine-protocol.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the designer's protocol guard: %v", err)
	}
	match := tableProjectionGuardKeyList.FindSubmatch(source)
	if match == nil {
		t.Fatal("engine-protocol.ts no longer checks the projected TABLE object's keys where this test can read it; if isTableColumns was restructured, re-derive this extraction rather than deleting the check")
	}
	keys := extractedKeyList(string(match[1]))
	if !reflect.DeepEqual(keys, tableColumnsProjectionWireKeys) {
		t.Errorf("the designer's isTableColumns guard accepts the table keys\n\t%v\nand the recorded protocol set is\n\t%v — one side of this seam has moved and the other has not, and the symptom is not a blank canvas: parseInbound returns undefined, engine-client terminates the worker, and a FIRST table-editor open shows nothing at all because the panel that would render the error never mounts", keys, tableColumnsProjectionWireKeys)
	}
}

// ---------------------------------------------------------------------------
// THE SIXTH AND SEVENTH RECORDS — THE COMPONENT LEVEL, AND ONE TABLE COLUMN
// INSIDE IT. Story 14.9, closing DW-74.
// ---------------------------------------------------------------------------
//
// WHY THIS LEVEL WAS UNPINNED UNTIL NOW, measured rather than supposed.
// `canvasGuardKeyList` above is `(?s)const isCanvas = .*?hasOnly\(value, \[(.*?)\]\)`
// and the `.*?` is NON-GREEDY, so it stops at the TOP-LEVEL key list and never
// reaches `hasOnly(component, [`. None of the five records above descends into
// a component's own keys: the fragment record jumps straight past them to
// `textPaint -> lines -> fragments`. So every one of the thirty-one keys a
// component carries — `tableBind`, `imageUnavailable`, the whole typography
// block — has been pinned by NOTHING on the TypeScript side since the day it
// was added. Story 14.9 is the first story to add a component key and therefore
// the first that can break it.
//
// THE RECORD PINS THE WHOLE ACCEPTED SET, ALL THIRTY-SIX KEYS, AND THAT IS THE
// POINT RATHER THAN A SIDE EFFECT. A record naming only the new key would be a
// DENYLIST: it would go green on the next field someone adds, which is the
// shape this project refuses. A key-set record is a key SET.
//
// AND IT IS THE ONLY ARTEFACT IN THE REPOSITORY THAT CAN PROVE STORY 14.9 AT
// THE SEAM. `CanvasProjection` appears zero times in serialize.go and zero
// times in internal/pdf; it is never serialized into a golden and never enters
// a PDF, so the matrix legs, `hashmatrix` and the cross-target byte-identity
// workflow are all blind to a canvas-projection field BY CONSTRUCTION — not by
// an oversight a new fixture could fix.
//
// THE GUARD IS `hasOnly`, A SUBSET CHECK, so what is recorded here is the
// ACCEPTED set, exactly as the fragment record is — not an emitted one. A
// component NEVER marshals all thirty-six keys: almost every member is a
// pointer or a slice with `omitempty`, and `value`, `tableBind`, `columns`,
// `textPaint`, `image`, `imageUnavailable`, `barcode`, `barcodeUnavailable`, `qrcode` and `qrcodeUnavailable` belong to mutually exclusive
// populations. So the Go side is pinned two ways: every key any component
// actually emits must be IN the record (a key Go sends that the guard does not
// list is what terminates the worker), and the TABLE component's own emission
// set is pinned exactly, so the new member cannot silently stop being sent.
var canvasComponentWireKeys = []string{
	"align",
	"authored",
	"background",
	"band",
	"barcode",
	"barcodeUnavailable",
	"belowSectionBreak",
	"binding",
	"bold",
	"borderColor",
	"borderEdges",
	"borderWidth",
	"color",
	"columns",
	"fontFamily",
	"fontSize",
	"height",
	"id",
	"image",
	"imageUnavailable",
	"italic",
	"lineSpacing",
	"paddingBottom",
	"paddingLeft",
	"paddingRight",
	"paddingTop",
	"page",
	"qrcode",
	"qrcodeUnavailable",
	"resizable",
	"tableBind",
	"textPaint",
	"type",
	"valign",
	"value",
	"visibleIf",
	"width",
	"x",
	"y",
}

// canvasComponentColumnWireKeys is the recorded key set of ONE TABLE COLUMN —
// one level below a component, and the level Story 14.9 added.
//
// ITS GUARD IS `hasExactKeys`, WHICH REJECTS IN BOTH DIRECTIONS: a key Go stops
// sending fails it as surely as a key Go starts sending. Nothing on
// CanvasTableColumn carries `omitempty`, and the zero-value identity below is
// what holds that property — `label` and `bind` are REQUIRED keys whose values
// may legally be empty, so an `omitempty` on either would drop the key for
// exactly the documents that leave it empty and terminate the worker for
// exactly those.
var canvasComponentColumnWireKeys = []string{"bind", "cellAlign", "headerAlign", "id", "label", "labelLines", "width"}

// canvasTableComponentEmittedKeys is the EXACT set a bound table with one
// column marshals, recorded so the new member cannot quietly stop being sent.
// The subset check below cannot see that: a component that emits FEWER keys
// than the record is still a subset of it.
var canvasTableComponentEmittedKeys = []string{"authored", "band", "columns", "fontFamily", "fontSize", "height", "id", "page", "resizable", "tableBind", "type", "width", "x", "y"}

// componentsFromProjectionBytes reads the projected components as raw objects,
// by the same rule every helper in this file uses: from the marshalled bytes,
// never from the struct tags a rename would have edited in the same breath.
func componentsFromProjectionBytes(t *testing.T, projection designer.CanvasProjection) []json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(mustMarshal(t, projection), &object); err != nil {
		t.Fatalf("unmarshal CanvasProjection: %v", err)
	}
	var components []json.RawMessage
	if err := json.Unmarshal(object["components"], &components); err != nil {
		t.Fatalf("unmarshal components: %v", err)
	}
	return components
}

// TestCanvasComponentWireKeysAreTheRecordedSet is the Go half of the component
// record.
//
// ⚠ ITS FIXTURE IS TABLE-BEARING, AND THAT IS LOAD-BEARING RATHER THAN
// INCIDENTAL. All four canvas records above project
// `canvasWindowCountControlTemplateJSON`, which contains NO TABLE AT ALL
// (measured: zero occurrences of `table` in it). A component record built on it
// would never see `tableBind` and would never see `columns`, and would go green
// proving nothing about either — a fixture more complete than the defect's
// precondition is a guard that cannot see it. The preconditions below are
// `t.Fatal`, not skips, for that reason.
func TestCanvasComponentWireKeysAreTheRecordedSet(t *testing.T) {
	projection := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountOneColumnTableTemplateJSON))
	components := componentsFromProjectionBytes(t, projection)
	if len(components) == 0 {
		t.Fatal("fixture precondition: the component wire-key fixture projected no components at all, so this record asserts nothing")
	}
	tables, columned, nonTables := 0, 0, 0
	for _, raw := range components {
		keys := marshalledObjectKeys(t, raw)
		for _, key := range keys {
			if !slices.Contains(canvasComponentWireKeys, key) {
				t.Errorf("a projected component marshals the key %q, which the recorded component key set does not name (the record is\n\t%v\n) — the designer's guard is hasOnly, so an unlisted key makes isCanvas false, parseInbound undefined and PROTOCOL_INVALID terminate the worker, and the session is dead until reload", key, canvasComponentWireKeys)
			}
		}
		var component map[string]json.RawMessage
		if err := json.Unmarshal(raw, &component); err != nil {
			t.Fatalf("unmarshal a component: %v", err)
		}
		if string(component["type"]) != `"table"` {
			nonTables++
			if _, ok := component["columns"]; ok {
				t.Errorf("a %s component carries a columns member: %s — the designer's cross-clause rejects that outright, exactly as it does a tableBind on a non-table", string(component["type"]), raw)
			}
			continue
		}
		tables++
		if !reflect.DeepEqual(keys, canvasTableComponentEmittedKeys) {
			t.Errorf("a bound one-column table marshals the keys\n\t%v\nand the recorded emission set is\n\t%v", keys, canvasTableComponentEmittedKeys)
		}
		rawColumns, ok := component["columns"]
		if !ok {
			continue
		}
		columned++
		var columns []json.RawMessage
		if err := json.Unmarshal(rawColumns, &columns); err != nil {
			t.Fatalf("unmarshal a component's columns: %v", err)
		}
		if len(columns) == 0 {
			t.Fatal("a columns member was emitted as an EMPTY array; the member is omitempty precisely so that a table with no columns carries no member at all")
		}
		for _, rawColumn := range columns {
			columnKeys := marshalledObjectKeys(t, rawColumn)
			if !reflect.DeepEqual(columnKeys, canvasComponentColumnWireKeys) {
				t.Errorf("a projected table column marshals the keys\n\t%v\nand the recorded protocol set is\n\t%v — the designer checks a column with hasExactKeys, so a dropped key terminates the worker as surely as a surplus one", columnKeys, canvasComponentColumnWireKeys)
			}
		}
	}
	if tables == 0 {
		t.Fatal("fixture precondition: the component wire-key fixture projects no TABLE component, so neither `tableBind` nor `columns` is ever emitted and this record passes vacuously — which is exactly what a record built on canvasWindowCountControlTemplateJSON would have done")
	}
	if columned == 0 {
		t.Fatal("fixture precondition: the component wire-key fixture's table emitted no `columns` member, so the key set Story 14.9 added is unmeasured")
	}
	if nonTables == 0 {
		t.Fatal("fixture precondition: the component wire-key fixture projects only tables, so the cross-clause assertion — that a non-table carries no columns member — said nothing")
	}
	// AND THE COLUMN CARRIES NO `omitempty`, checked the way the chain and
	// entry records check theirs: a zero value must marshal the same key set a
	// projected one does. `label` and `bind` are required keys whose values may
	// be empty, so this is the property that keeps an empty label from dropping
	// a key the browser's exact-key guard requires.
	zeroColumn := marshalledObjectKeys(t, mustMarshal(t, designer.CanvasTableColumn{}))
	if !reflect.DeepEqual(zeroColumn, canvasComponentColumnWireKeys) {
		t.Errorf("a zero CanvasTableColumn marshals\n\t%v\nand the recorded protocol set is\n\t%v — an omitempty here drops a key for exactly the documents that leave that column's label or bind empty, and those are the documents this story exists to draw honestly", zeroColumn, canvasComponentColumnWireKeys)
	}
}

// TestCanvasQRCodeWireKeysAreTheRecordedSet is the component record's qrcode
// half: the JSON Go actually emits for a painted qrcode and for one that
// cannot fit, read from the marshalled bytes. A renamed `qrcode`,
// `qrcodeUnavailable`, `moduleWidth`, `rects` or rect tag reds here, where
// the designer's guard would otherwise reject the snapshot.
func TestCanvasQRCodeWireKeysAreTheRecordedSet(t *testing.T) {
	tpl, err := ParseTemplate([]byte(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 72, "height": 72, "errorCorrection": "Q", "value": "folio8"},
      {"id": "e2", "type": "qrcode", "x": 0, "y": 100, "width": 0.02, "height": 0.02, "value": "folio8"}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "4.0"
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	painted, unavailable := 0, 0
	for _, raw := range componentsFromProjectionBytes(t, projectWithPaint(t, tpl)) {
		for _, key := range marshalledObjectKeys(t, raw) {
			if !slices.Contains(canvasComponentWireKeys, key) {
				t.Errorf("a projected qrcode marshals the key %q, which the recorded component key set does not name", key)
			}
		}
		var component map[string]json.RawMessage
		if err := json.Unmarshal(raw, &component); err != nil {
			t.Fatalf("unmarshal a component: %v", err)
		}
		if _, ok := component["qrcodeUnavailable"]; ok {
			unavailable++
		}
		rawPaint, ok := component["qrcode"]
		if !ok {
			continue
		}
		painted++
		if keys := marshalledObjectKeys(t, rawPaint); !reflect.DeepEqual(keys, []string{"moduleWidth", "rects"}) {
			t.Errorf("a qrcode paint marshals the keys %v, want exactly [moduleWidth rects] — the designer checks it with hasExactKeys", keys)
		}
		var paint map[string]json.RawMessage
		if err := json.Unmarshal(rawPaint, &paint); err != nil {
			t.Fatalf("unmarshal qrcode paint: %v", err)
		}
		var rects []json.RawMessage
		if err := json.Unmarshal(paint["rects"], &rects); err != nil || len(rects) == 0 {
			t.Fatalf("qrcode paint carries no rects: %s", rawPaint)
		}
		if keys := marshalledObjectKeys(t, rects[0]); !reflect.DeepEqual(keys, []string{"height", "width", "x", "y"}) {
			t.Errorf("a qrcode rect marshals the keys %v, want exactly [height width x y] — the designer checks it with hasExactKeys", keys)
		}
	}
	if painted == 0 {
		t.Fatal("fixture precondition: no component emitted a `qrcode` key, so the paint record is unmeasured")
	}
	if unavailable == 0 {
		t.Fatal("fixture precondition: no component emitted a `qrcodeUnavailable` key, so the reason record is unmeasured")
	}
}

// canvasGuardComponentKeyList extracts the key list engine-protocol.ts's
// isCanvas guard passes to hasOnly FOR ONE COMPONENT.
//
// Anchored on the `component` parameter name, for the reason the fragment and
// chain extractors are anchored on theirs: `hasOnly(value, [...])` is this
// file's whole idiom, and an unanchored match would read the TOP-LEVEL list and
// compare it to this record — a green test asserting the wrong thing. It is
// also why `canvasGuardKeyList` above could never reach this list: its `.*?` is
// non-greedy and stops at the first `hasOnly(value, [`.
var canvasGuardComponentKeyList = regexp.MustCompile(`hasOnly\(component, \[(.*?)\]\)`)

// canvasGuardComponentColumnKeyList extracts the TABLE COLUMN's key list from
// the same guard.
//
// ⚠ ANCHORED ON `const isCanvas =` AND NOT ON `column` ALONE, because
// `hasExactKeys(column, [...])` is ALSO isTableColumns' spelling one screen
// earlier in the same file — the table EDITOR's ten-member column, a different
// surface with a different contract. An unanchored match reads THAT list and
// compares it to this record, which is the "green test asserting the wrong
// thing" every other extractor in this file is anchored to avoid. isCanvas is
// declared after isTableColumns, so the non-greedy `.*?` lands on the canvas
// clause.
var canvasGuardComponentColumnKeyList = regexp.MustCompile(`(?s)const isCanvas = .*?hasExactKeys\(column, \[(.*?)\]\)`)

// TestCanvasComponentWireKeysAreTheOnesTheDesignerAccepts is the TypeScript
// half, and the one assertion in this repository that would have caught Story
// 14.9's field being added on one side only — before it shipped green and
// terminated the worker at runtime.
func TestCanvasComponentWireKeysAreTheOnesTheDesignerAccepts(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "folio-designer", "src", "engine-protocol.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the designer's protocol guard: %v", err)
	}
	match := canvasGuardComponentKeyList.FindSubmatch(source)
	if match == nil {
		t.Fatal("engine-protocol.ts no longer has an isCanvas component clause whose hasOnly list this test can read; if the guard was restructured, re-derive this extraction rather than deleting the check")
	}
	keys := extractedKeyList(string(match[1]))
	if !reflect.DeepEqual(keys, canvasComponentWireKeys) {
		t.Errorf("the designer's component guard accepts the keys\n\t%v\nand the recorded protocol set is\n\t%v — one side of this seam has moved and the other has not; a Go key the guard does not list makes isCanvas false, parseInbound undefined and PROTOCOL_INVALID terminate the worker, and there is no respawn path", keys, canvasComponentWireKeys)
	}
	columnMatch := canvasGuardComponentColumnKeyList.FindSubmatch(source)
	if columnMatch == nil {
		t.Fatal("engine-protocol.ts no longer checks a projected canvas table COLUMN's keys where this test can read it; if the guard was restructured, re-derive this extraction rather than deleting the check")
	}
	columnKeys := extractedKeyList(string(columnMatch[1]))
	if !reflect.DeepEqual(columnKeys, canvasComponentColumnWireKeys) {
		t.Errorf("the designer's canvas table COLUMN guard accepts the keys\n\t%v\nand the recorded protocol set is\n\t%v — this clause is hasExactKeys, so it rejects in both directions", columnKeys, canvasComponentColumnWireKeys)
	}
	// AND THE ANCHOR IS PROVED TO HAVE FOUND THE RIGHT CLAUSE. isTableColumns
	// spells `hasExactKeys(column, [...])` too, one screen earlier, and its list
	// is the table EDITOR's ten-member column. If this extraction ever drifted
	// onto that one it would still find A list, and the failure would read as a
	// protocol change rather than as a broken regexp.
	if slices.Contains(columnKeys, "rowFieldEditable") || slices.Contains(columnKeys, "footerFormat") {
		t.Fatalf("the canvas column extraction matched isTableColumns' editor column instead of isCanvas's: %v", columnKeys)
	}
}
