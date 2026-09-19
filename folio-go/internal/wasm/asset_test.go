package wasm

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// png3x2RGB mirrors folio-go's own package-level test fixture of the same
// name and bytes (package boundaries mean it cannot be shared directly —
// the same reasoning render_image_test.go's own duplicate comment gives).
var png3x2RGB = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x02, 0x08, 0x02, 0x00, 0x00, 0x00, 0x12, 0x16, 0xf1,
	0x4d, 0x00, 0x00, 0x00, 0x18, 0x49, 0x44, 0x41, 0x54, 0x78, 0xda, 0x62, 0xfa, 0xcf, 0xc0, 0xc0,
	0x00, 0xc6, 0x4c, 0x10, 0xea, 0x3f, 0x03, 0x03, 0x20, 0x00, 0x00, 0xff, 0xff, 0x3c, 0x14, 0x05,
	0xff, 0xcd, 0x8e, 0xc5, 0xb9, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60,
	0x82,
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

func setAssetCommand(id, mediaType string, data []byte) []byte {
	return []byte(`{"kind":"setComponentAsset","version":1,"id":"` + id + `","mediaType":"` + mediaType + `","data":"` + base64.StdEncoding.EncodeToString(data) + `"}`)
}

// loadedImageEngine loads the shipped worked example and drops an image
// component from the palette. The dropped box starts EMPTY — its asset is
// present and null until the author chooses a file — so a test that needs a
// picture already in place sets one itself.
func loadedImageEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	created, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"image","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, component := range created.Canvas.Components {
		if component.Type == "image" {
			id = component.ID
		}
	}
	if id == "" {
		t.Fatal("image component was not projected")
	}
	return engine, id
}

// TestEngineApplySetComponentAssetIsOneHistoryEventWithUndoRedo proves AC1's
// "advancing the document revision and engine-side undo history exactly
// once" clause end-to-end through the wasm transaction boundary, and AC4's
// claim that undo needs no new machinery: Engine.undo already holds
// canonical byte snapshots, so undoing the mutation that collected an asset
// restores it BY CONSTRUCTION.
func TestEngineApplySetComponentAssetIsOneHistoryEventWithUndoRedo(t *testing.T) {
	engine, id := loadedImageEngine(t)
	// The box arrives empty, so the picture this test replaces has to be put
	// there first: the subject is what happens to the asset a REPLACEMENT
	// collects, which needs one to collect.
	firstPicture := encodePNG(t, 2, 3)
	if _, err := engine.Apply(setAssetCommand(id, "image/png", firstPicture)); err != nil {
		t.Fatal(err)
	}
	before := engine.Snapshot()

	after, err := engine.Apply(setAssetCommand(id, "image/png", png3x2RGB))
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision+1 || !after.CanUndo || after.CanRedo {
		t.Fatalf("history after set = %#v, want revision %d, canUndo, no canRedo", after, before.Revision+1)
	}

	newKey := sha256Hex(png3x2RGB)
	raw, _, err := engine.AssetBytes(newKey)
	if err != nil || string(raw) != string(png3x2RGB) {
		t.Fatalf("AssetBytes(newKey) = %v, %v", raw, err)
	}

	undone, err := engine.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if undone.Revision == after.Revision || !undone.CanRedo {
		t.Fatalf("undo snapshot = %#v", undone)
	}
	// D-5.13.3/AC4: undo restores the COLLECTED asset's bytes by construction
	// — proving that, not building new machinery.
	restored, _, err := engine.AssetBytes(sha256Hex(firstPicture))
	if err != nil {
		t.Fatalf("expected the orphan-collected asset to be restored by undo: %v", err)
	}
	if len(restored) == 0 {
		t.Fatal("restored default asset bytes are empty")
	}

	redone, err := engine.Redo()
	if err != nil {
		t.Fatal(err)
	}
	// Revision is monotonic even across undo/redo (Engine.install always
	// increments it, even when canonical bytes return to an earlier state)
	// — it does not return to `after`'s exact number, only forward past it.
	if redone.Revision != undone.Revision+1 {
		t.Fatalf("redo revision = %d, want %d", redone.Revision, undone.Revision+1)
	}
	if _, _, err := engine.AssetBytes(newKey); err != nil {
		t.Fatalf("expected the new asset to be present again after redo: %v", err)
	}
}

// TestEngineApplySetComponentAssetChoosingSamePictureDoesNotAdvanceHistory
// proves AC1's re-choosing clause at the wasm.Engine.Apply layer: the
// existing bytes.Equal(canonical, e.bytes) short-circuit means re-setting
// the identical picture already installed must not advance revision or
// push undo.
func TestEngineApplySetComponentAssetChoosingSamePictureDoesNotAdvanceHistory(t *testing.T) {
	engine, id := loadedImageEngine(t)
	beforeSet, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	first, err := engine.Apply(setAssetCommand(id, "image/png", png3x2RGB))
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Apply(setAssetCommand(id, "image/png", png3x2RGB))
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != first.Revision {
		t.Fatalf("re-choosing the same picture advanced revision: %d -> %d", first.Revision, second.Revision)
	}
	// Only ONE history event should exist for the pair of applies (the
	// second was a no-op short-circuit, D-5.13.1): a single undo must
	// return exactly to the canonical bytes captured before EITHER call.
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	afterOneUndo, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if string(afterOneUndo) != string(beforeSet) {
		t.Fatal("re-choosing the same picture must not have pushed a second undo entry")
	}
}

// TestEngineAssetBytesIsReadOnly proves the per-key asset-bytes query never
// advances revision or touches history.
func TestEngineAssetBytesIsReadOnly(t *testing.T) {
	engine, id := loadedImageEngine(t)
	if _, err := engine.Apply(setAssetCommand(id, "image/png", png3x2RGB)); err != nil {
		t.Fatal(err)
	}
	before := engine.Snapshot()
	key := sha256Hex(png3x2RGB)
	if _, _, err := engine.AssetBytes(key); err != nil {
		t.Fatal(err)
	}
	after := engine.Snapshot()
	if after.Revision != before.Revision || after.CanUndo != before.CanUndo || after.CanRedo != before.CanRedo {
		t.Fatalf("AssetBytes mutated engine state: before=%#v after=%#v", before, after)
	}
}

// TestEngineAssetBytesRejectsAnUnknownKey proves the query is located and
// bounded rather than silently returning nothing.
func TestEngineAssetBytesRejectsAnUnknownKey(t *testing.T) {
	engine, _ := loadedImageEngine(t)
	if _, _, err := engine.AssetBytes("0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("expected an overlong key to be refused")
	}
	if _, _, err := engine.AssetBytes(sha256Hex([]byte("never inserted"))); err == nil {
		t.Fatal("expected an absent key to be refused")
	}
}

// encodePNG builds a decodable picture distinct from png3x2RGB, so a test
// can replace one asset with another and watch what happens to the first.
func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	// Every pixel opaque: the encoder writes truecolor rather than
	// truecolor+alpha, and this library refuses transparency (AC12/D-1.8.1).
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			picture.Set(x, y, color.RGBA{R: uint8(0x20 + x), G: uint8(0x60 + y), B: 0xa8, A: 0xff})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, picture); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
