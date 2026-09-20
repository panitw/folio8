package wasm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// cjkFixture is the one committed document whose text reaches the CJK
// face: "Ada ก 汉", over the chain [Noto Sans, Noto Sans Thai, Noto Sans
// SC]. It is read from disk rather than written inline so the shape the
// designer actually opens is the shape under test.
const cjkFixture = "../../../fixtures/multi-script-fallback/input.folio"

// tenFaceSet is what the designer's engine wasm is handed: fonts.Shipped()
// without the CJK face. It is built by DELETING the key rather than by
// listing ten names, so a shipped face added later is covered here without
// an edit — and the count is checked, because a set that lost a second
// face would still satisfy every assertion below for the wrong reason.
//
// IT IS WRITTEN TO RUN UNDER BOTH BUILDS. Untagged, the delete removes the
// key; under `-tags nocjkface` the key was never there and the delete is a
// no-op. Either way the engine under test is handed ten faces, which is
// the state the designer's wasm is actually in.
func tenFaceSet(t *testing.T) folio8.FontSet {
	t.Helper()
	set := fonts.Shipped()
	delete(set, "Noto Sans SC")
	if len(set) != 10 {
		t.Fatalf("expected ten faces after removing the CJK one, got %d — this test states the DESIGNER's set and a different size means it is stating something else", len(set))
	}
	return set
}

// cjkFaceBytes reads the face off DISK rather than out of fonts.Shipped(),
// and that is what lets this file run under `-tags nocjkface` — the build
// whose whole point is that those bytes are not in the binary. It is also
// the more honest source: the browser fetches the deferred release asset,
// which is a copy of this same committed file, not of anything Go embeds.
func cjkFaceBytes(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "fonts", "notosanssc", "NotoSansSC-Regular.ttf")
	face, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(face) < 10_000_000 {
		t.Fatalf("%s read as %d bytes, far under the face's ~10.6 MB — a truncated read would make every install below vacuous", path, len(face))
	}
	return face
}

// elevenFaceSet is tenFaceSet with the CJK face put back at CONSTRUCTION
// time. It is the control the installed-later engine is compared against.
func elevenFaceSet(t *testing.T) folio8.FontSet {
	t.Helper()
	set := tenFaceSet(t)
	set["Noto Sans SC"] = cjkFaceBytes(t)
	return set
}

// jsonOf renders a projection as its canonical JSON, which is the form
// that actually crosses to the browser — comparing the Go structs with
// reflect.DeepEqual would compare fields the wire never carries.
func jsonOf(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	return string(encoded)
}

func readCJKFixture(t *testing.T) []byte {
	t.Helper()
	input, err := os.ReadFile(cjkFixture)
	if err != nil {
		t.Fatalf("read %s: %v", cjkFixture, err)
	}
	return input
}

// TestEngineWithoutTheCJKFaceRefusesAndNamesIt is CAP-6 meeting CAP-7 in
// the one place the designer depends on: `load` is where a CJK document
// first touches a face, because parsing is font-free and fonts enter at
// the canvas projection.
//
// THE ASSERTION IS THE NAME, NOT MERELY THE FAILURE. The browser's whole
// recovery is "read the face out of the refusal, fetch that asset,
// install, retry" — a refusal that failed without naming the face would
// be a refusal the designer cannot act on.
func TestEngineWithoutTheCJKFaceRefusesAndNamesIt(t *testing.T) {
	engine := NewEngine(testClock(), tenFaceSet(t))
	_, err := engine.Load(readCJKFixture(t))
	if err == nil {
		t.Fatal("a ten-face engine loaded a document whose text needs the CJK face; CAP-7 requires a refusal, never a substitute")
	}
	var renderErr *folio8.RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("the refusal is not a *folio8.RenderError, so the wasm host cannot report a diagnostic code for it: %v", err)
	}
	if renderErr.Diagnostic.Code != folio8.DiagCodeTextFaceAbsent {
		t.Fatalf("refusal code is %q, expected %q", renderErr.Diagnostic.Code, folio8.DiagCodeTextFaceAbsent)
	}
	if !strings.Contains(renderErr.Diagnostic.Message, "Noto Sans SC") {
		t.Fatalf("the refusal does not name the absent face, so nothing tells the browser which asset to fetch: %s", renderErr.Diagnostic.Message)
	}
}

// TestInstalledCJKFaceProjectsByteIdenticallyToTheShippedSet is the
// spec's "byte-identically once the face is installed", asserted on the
// canvas projection the designer paints from — the surface CAP-6 moved
// the face off, and therefore the surface on which a difference would
// show up.
//
// THE COMPARISON IS AGAINST AN ENGINE BUILT WITH THE WHOLE SHIPPED SET,
// not against a recorded snapshot: what has to hold is that supplying the
// face and embedding it are the same thing, and only a live pair can say
// that.
func TestInstalledCJKFaceProjectsByteIdenticallyToTheShippedSet(t *testing.T) {
	input := readCJKFixture(t)

	embedded := NewEngine(testClock(), elevenFaceSet(t))
	reference, err := embedded.Load(input)
	if err != nil {
		t.Fatalf("the eleven-face engine could not open the CJK fixture at all, so there is nothing to compare against: %v", err)
	}

	supplied := NewEngine(testClock(), tenFaceSet(t))
	if _, err := supplied.Load(input); err == nil {
		t.Fatal("the ten-face engine opened the CJK document, so the install below would prove nothing")
	}
	if err := supplied.InstallFace("Noto Sans SC", cjkFaceBytes(t)); err != nil {
		t.Fatalf("install: %v", err)
	}
	installed, err := supplied.Load(input)
	if err != nil {
		t.Fatalf("the retry after installing the named face still refused: %v", err)
	}

	if installed.Canvas == nil || reference.Canvas == nil {
		t.Fatal("one of the two opens produced no canvas projection, so the comparison below would be vacuous")
	}
	if got, want := jsonOf(t, installed.Canvas), jsonOf(t, reference.Canvas); got != want {
		t.Fatalf("the canvas projection differs once the face is supplied rather than embedded.\nsupplied: %s\nembedded: %s", got, want)
	}
	// NON-VACUITY: the projection must actually carry the CJK fragment.
	// Two empty canvases are equal and would prove nothing.
	if !strings.Contains(jsonOf(t, installed.Canvas), "Noto Sans SC") {
		t.Fatal("the projection names no CJK face at all, so the equality above compared two documents that never reached it")
	}
}

// TestInstallFaceIsIdempotentAndRefusesEmptyInput pins the two properties
// the browser relies on: one install per face per session is enough (the
// worker is never restarted), and an install with nothing in it is a
// refusal rather than a face keyed to zero bytes — which would join to
// nothing and compare vacuously to anything downstream.
func TestInstallFaceIsIdempotentAndRefusesEmptyInput(t *testing.T) {
	engine := NewEngine(testClock(), tenFaceSet(t))
	face := cjkFaceBytes(t)
	for attempt := 0; attempt < 2; attempt++ {
		if err := engine.InstallFace("Noto Sans SC", face); err != nil {
			t.Fatalf("install attempt %d: %v", attempt+1, err)
		}
	}
	if _, err := engine.Load(readCJKFixture(t)); err != nil {
		t.Fatalf("a twice-installed face did not open the document: %v", err)
	}
	if err := engine.InstallFace("Noto Sans SC", nil); err == nil {
		t.Fatal("an install with no bytes was accepted, which would replace a working face with an empty one")
	}
	if err := engine.InstallFace("", face); err == nil {
		t.Fatal("an install with no name was accepted")
	}
	// ⚠ AND BYTES THAT ARE NOT A FONT ARE REFUSED AT THE INSTALL, not
	// accepted and discovered later. A truncated or wrong download would
	// otherwise be a permanently installed bad face for the session — the
	// worker is never restarted — failing somewhere deeper than
	// TEXT_FACE_ABSENT, where the browser's recovery cannot read it.
	if err := engine.InstallFace("Noto Sans SC", []byte("not a font, not even close")); err == nil {
		t.Fatal("bytes that are not a font were installed; the failure would land in a later render instead of on the install the recovery can act on")
	}
	// A TRUNCATED DOWNLOAD IS THE REALISTIC SHAPE OF IT: a valid prefix of
	// the real face, which a length check alone would happily accept.
	if err := engine.InstallFace("Noto Sans Truncated", face[:len(face)/2]); err == nil {
		t.Fatal("half a font was installed")
	}
	if _, err := engine.Load(readCJKFixture(t)); err != nil {
		t.Fatalf("a refused install disturbed the face already held: %v", err)
	}
}

// TestNewEngineDoesNotAliasTheCallersFontSet: the shell hands over its own
// fonts.Shipped() result, and an install must not write into it.
func TestNewEngineDoesNotAliasTheCallersFontSet(t *testing.T) {
	callers := tenFaceSet(t)
	engine := NewEngine(testClock(), callers)
	if err := engine.InstallFace("Noto Sans SC", cjkFaceBytes(t)); err != nil {
		t.Fatal(err)
	}
	if _, ok := callers["Noto Sans SC"]; ok {
		t.Fatal("InstallFace wrote back into the set NewEngine was given")
	}
}
