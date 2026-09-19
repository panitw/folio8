package folio8

import (
	"bytes"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// threeBandImageTemplateJSON places the same asset on one image element per
// band, each with a DIFFERENT box (AC3's "wider-than-box, taller-than-box
// and exactly-proportioned" fit variety), so a single document exercises
// both the fit computation and the band-frame/page-absolute translation
// across all three band origins.
func threeBandImageTemplateJSON(key string, wrapped []string, mediaType string) string {
	dataJSON := `["` + joinQuoted(wrapped) + `"]`
	return `{
  "assets": {"` + key + `": {"data": ` + dataJSON + `, "mediaType": "` + mediaType + `"}},
  "bands": {
    "pageHeader": {"elements": [
      {"id": "e1", "type": "image", "asset": "` + key + `", "x": 5, "y": 2, "width": 12, "height": 12}
    ], "height": 20},
    "content": {"elements": [
      {"id": "e2", "type": "image", "asset": "` + key + `", "x": 10, "y": 30, "width": 150, "height": 30}
    ]},
    "pageFooter": {"elements": [
      {"id": "e3", "type": "image", "asset": "` + key + `", "x": 8, "y": 1, "width": 30, "height": 15}
    ], "height": 30}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
}

func joinQuoted(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += `","`
		}
		out += v
	}
	return out
}

func canvasComponentByID(projection designer.CanvasProjection, id string) *designer.CanvasComponent {
	for i := range projection.Components {
		if projection.Components[i].ID == id {
			return &projection.Components[i]
		}
	}
	return nil
}

// TestCanvasImagePaintExactlyMatchesTheShippingRunPath is D-5.13.2's
// two-producer proof, named after TestCanvasTextPaintExactlyMatchesTheShippingRunPath
// so the paint is proven equal to THE SHIPPING RUN PATH — collectImageRuns
// plus resolveImagePlacement, render.go's own producer — never merely
// computed from the same function in a way both sides could drift together.
// It also proves the band-frame/page-absolute translation equality
// explicitly (D-5.13.2's "Frame" clause) across all three bands, and covers
// AC3's fit variety (wider-than-box, taller-than-box, exactly-proportioned)
// via threeBandImageTemplateJSON's three distinct boxes against one 3x2
// image.
func TestCanvasImagePaintExactlyMatchesTheShippingRunPath(t *testing.T) {
	key := sha256Hex(png3x2RGB)
	doc := threeBandImageTemplateJSON(key, wrap76(png3x2RGB), "image/png")
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}

	runs, err := collectImageRuns(tpl)
	if err != nil {
		t.Fatalf("shipping run collection: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("shipping run collection: got %d runs, want 3", len(runs))
	}
	asset := tpl.doc.Assets[key]
	raw, err := template.DecodeAssetBytes(asset)
	if err != nil {
		t.Fatal(err)
	}
	img, err := template.DecodeImageForRender(asset.MediaType, raw, key, "shared")
	if err != nil {
		t.Fatal(err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatal(err)
	}
	bandOrigin := map[string]int64{"pageHeader": int64(bands[0].origin), "content": int64(bands[1].origin), "pageFooter": int64(bands[2].origin)}

	for _, run := range runs {
		component := canvasComponentByID(projection, run.elementID)
		if component == nil || component.Image == nil {
			t.Fatalf("component %s has no image paint: %#v", run.elementID, component)
		}
		wantX, wantY, wantW, wantH := resolveImagePlacement(run, img)
		origin := bandOrigin[component.Band]
		if component.Image.DrawX != int64(wantX) {
			t.Errorf("%s: drawX = %d, want shipping %d (X is not band-translated)", run.elementID, component.Image.DrawX, int64(wantX))
		}
		if component.Image.DrawY != int64(wantY)-origin {
			t.Errorf("%s: drawY (band-relative) = %d, want page-absolute %d minus band origin %d = %d", run.elementID, component.Image.DrawY, int64(wantY), origin, int64(wantY)-origin)
		}
		if component.Image.DrawWidth != int64(wantW) || component.Image.DrawHeight != int64(wantH) {
			t.Errorf("%s: draw size = %dx%d, want shipping %dx%d", run.elementID, component.Image.DrawWidth, component.Image.DrawHeight, int64(wantW), int64(wantH))
		}
		if component.Image.Width != img.Width() || component.Image.Height != img.Height() {
			t.Errorf("%s: intrinsic = %dx%d, want %dx%d", run.elementID, component.Image.Width, component.Image.Height, img.Width(), img.Height())
		}
		if component.Image.MediaType != asset.MediaType {
			t.Errorf("%s: mediaType = %q, want %q", run.elementID, component.Image.MediaType, asset.MediaType)
		}
	}

	after, err := SerializeTemplate(tpl)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("image paint mutated canonical template: %v", err)
	}
}

// TestCanvasImagePaintTracksAResize proves the paint is recomputed (not
// cached/derived from a stale rectangle) when the element's box changes.
func TestCanvasImagePaintTracksAResize(t *testing.T) {
	key := sha256Hex(png3x2RGB)
	tpl, err := ParseTemplate([]byte(imageTemplateJSON(key, wrap76(png3x2RGB), "image/png", 60, 60)))
	if err != nil {
		t.Fatal(err)
	}
	before, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	beforeImage := canvasComponentByID(before, "e1").Image
	if beforeImage == nil {
		t.Fatal("expected an image paint before resize")
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"resizeComponent","version":1,"id":"e1","width":180,"height":30,"snap":false}`)); err != nil {
		t.Fatal(err)
	}
	after, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	afterImage := canvasComponentByID(after, "e1").Image
	if afterImage == nil {
		t.Fatal("expected an image paint after resize")
	}
	if *afterImage == *beforeImage {
		t.Fatalf("image paint did not change after resizing the box: %#v", afterImage)
	}
	// Finding 16 (review of 2026-08-29): "changed" alone would also pass
	// for a paint that resized to the WRONG rectangle. Recompute the
	// expected draw rectangle straight from the shipping run path
	// (resolveImagePlacement over the post-resize element) and assert the
	// paint equals it — the same correctness shape
	// TestCanvasImagePaintExactlyMatchesTheShippingRunPath uses.
	afterElement := findElementByID(t, tpl, "e1")
	asset := tpl.doc.Assets[key]
	raw, err := template.DecodeAssetBytes(asset)
	if err != nil {
		t.Fatal(err)
	}
	img, err := template.DecodeImageForRender(asset.MediaType, raw, key, "e1")
	if err != nil {
		t.Fatal(err)
	}
	run := imageRunSource{elementID: "e1", assetKey: key, x: afterElement.X, y: afterElement.Y, boxW: afterElement.Width.Value, boxH: afterElement.Height.Value}
	wantX, wantY, wantW, wantH := resolveImagePlacement(run, img)
	if int64(wantX) != afterImage.DrawX || int64(wantY) != afterImage.DrawY || int64(wantW) != afterImage.DrawWidth || int64(wantH) != afterImage.DrawHeight {
		t.Fatalf("post-resize paint = {x:%d y:%d w:%d h:%d}, want {x:%d y:%d w:%d h:%d} (resolveImagePlacement on the resized run)",
			afterImage.DrawX, afterImage.DrawY, afterImage.DrawWidth, afterImage.DrawHeight, wantX, wantY, wantW, wantH)
	}
}

// TestCanvasImagePaintIsAbsentNotZeroForAnUndecodableAsset proves D-5.13.2's
// "Absence, not zero": an element whose asset is a legally-loaded but
// unrecognised media type has NO Image paint at all (nil, not a
// zero-valued struct).
func TestCanvasImagePaintIsAbsentNotZeroForAnUndecodableAsset(t *testing.T) {
	garbage := []byte("not any recognised format")
	key := sha256Hex(garbage)
	doc := imageTemplateJSON(key, wrap76(garbage), "image/webp", 50, 50)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	component := canvasComponentByID(projection, "e1")
	if component == nil {
		t.Fatal("component e1 missing from projection")
	}
	if component.Image != nil {
		t.Fatalf("expected no image paint for an undecodable asset, got %#v", component.Image)
	}
	// Finding 9 (review of 2026-08-29): the reason discriminant must say
	// UNDECODABLE here, not MISSING — the key resolves fine, only the
	// bytes/media-type do not.
	if component.ImageUnavailable == nil || *component.ImageUnavailable != imageUnavailableUndecodable {
		t.Fatalf("ImageUnavailable = %s, want %q", describeStringPtr(component.ImageUnavailable), imageUnavailableUndecodable)
	}
}

// describeStringPtr renders a *string for a test failure message without
// dereferencing a nil pointer.
func describeStringPtr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// TestCanvasImagePaintIsAbsentForAMissingAssetKey proves the same absence
// discipline when the referenced key is not in the document's assets map at
// all (a document Render would separately, fatally reject) — the canvas
// projection must still stay paintable rather than failing outright.
func TestCanvasImagePaintIsAbsentForAMissingAssetKey(t *testing.T) {
	realKey := sha256Hex(png3x2RGB)
	missingKey := sha256Hex([]byte("something else entirely, never inserted"))
	doc := `{
  "assets": {"` + realKey + `": {"data": ["` + base64Std(png3x2RGB) + `"], "mediaType": "image/png"}},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "image", "asset": "` + missingKey + `", "x": 10, "y": 20, "width": 60, "height": 60}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	component := canvasComponentByID(projection, "e1")
	if component == nil {
		t.Fatal("component e1 missing from projection")
	}
	if component.Image != nil {
		t.Fatalf("expected no image paint for a missing asset key, got %#v", component.Image)
	}
	// Finding 9 (review of 2026-08-29): a dangling asset reference must be
	// distinguished from an undecodable one — the media type here is not
	// even known (the key was never in the map), so calling this
	// "undecodable" would be false.
	if component.ImageUnavailable == nil || *component.ImageUnavailable != imageUnavailableMissing {
		t.Fatalf("ImageUnavailable = %s, want %q", describeStringPtr(component.ImageUnavailable), imageUnavailableMissing)
	}
}
