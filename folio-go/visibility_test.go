// This file is Story 3.5's own test suite (FR20, AD-24): visibleIf
// already shipped inert (Story 3.2) — these tests exercise the render-
// time behaviour this story adds. See render_visibility.go for the
// implementation and its own doc comments for the design this suite
// checks.
package folio8

import (
	"slices"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/bind"
)

// visTextTemplateJSON builds a two-element content-band document: e1
// carries visibleIfField verbatim (e.g. `"visibleIf": "customer.flag", `
// or "" for none), e2 is an always-visible sibling positioned well
// clear of e1 (AC2's "siblings never move" clause). Both elements are
// short enough to lay out in a single line each, on testFontSet's
// single face.
func visTextTemplateJSON(visibleIfField string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Conditional line", ` + visibleIfField + `"style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 200, "height": 20, "value": "Always visible", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// visImageTemplateJSON is visTextTemplateJSON's image analogue: a
// single image element carrying visibleIfField, referencing png3x2RGB
// (render_image_test.go's own fixture) under its real key so the asset
// decodes cleanly.
func visImageTemplateJSON(visibleIfField string) string {
	assetKey := sha256Hex(png3x2RGB)
	dataJSON := `["` + strings.Join(wrap76(png3x2RGB), `","`) + `"]`
	return `{
  "assets": {"` + assetKey + `": {"data": ` + dataJSON + `, "mediaType": "image/png"}},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "image", "x": 0, "y": 0, "width": 100, "height": 60, "asset": "` + assetKey + `", ` + visibleIfField + `}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// visBuildPages runs buildPageModel directly (this story's own seam,
// added so a test can assert on pagemodel.Page.Runs/Images counts —
// AC1's own literal anchor — without decoding serialized PDF bytes).
func visBuildPages(t *testing.T, tpl *Template, dataJSON string) []pagemodelPageForTest {
	t.Helper()
	data, err := bind.DecodeData(Data(dataJSON))
	if err != nil {
		t.Fatalf("DecodeData: %v", err)
	}
	params, err := decodeParams(nil)
	if err != nil {
		t.Fatalf("decodeParams: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, data, params, testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	out := make([]pagemodelPageForTest, len(pages))
	for i, p := range pages {
		out[i] = pagemodelPageForTest{runs: len(p.Runs), images: len(p.Images)}
	}
	return out
}

type pagemodelPageForTest struct {
	runs, images int
}

func totalRunsImages(pages []pagemodelPageForTest) (runs, images int) {
	for _, p := range pages {
		runs += p.runs
		images += p.images
	}
	return
}

// TestVisibleIfFalseElementContributesZeroRuns is AC1's text half: a
// literal the test owns (two data sets, expected counts) drives a
// render-time comparison of pagemodel.Page.Runs.
func TestVisibleIfFalseElementContributesZeroRuns(t *testing.T) {
	tpl, err := ParseTemplate([]byte(visTextTemplateJSON(`"visibleIf": "customer.flag", `)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	visiblePages := visBuildPages(t, tpl, `{"customer": {"flag": true}}`)
	hiddenPages := visBuildPages(t, tpl, `{"customer": {"flag": false}}`)

	visibleRuns, _ := totalRunsImages(visiblePages)
	hiddenRuns, _ := totalRunsImages(hiddenPages)

	// e1 contributes exactly one line/run when visible; e2 always
	// contributes one. So visible=2, hidden=1 — the DELTA is e1's own
	// contribution, which must be exactly zero when hidden (AC1: zero
	// entries to Page.Runs, not merely "fewer").
	if visibleRuns != 2 {
		t.Fatalf("presence precondition: expected 2 runs (e1+e2) when visible, got %d", visibleRuns)
	}
	if hiddenRuns != 1 {
		t.Fatalf("AC1: expected exactly 1 run (e2 only) when e1 is hidden, got %d — a hidden element must contribute ZERO entries to Page.Runs", hiddenRuns)
	}
}

// TestVisibleIfFalseImageContributesZeroImages is AC1's image half.
func TestVisibleIfFalseImageContributesZeroImages(t *testing.T) {
	tpl, err := ParseTemplate([]byte(visImageTemplateJSON(`"visibleIf": "customer.flag"`)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	visiblePages := visBuildPages(t, tpl, `{"customer": {"flag": true}}`)
	hiddenPages := visBuildPages(t, tpl, `{"customer": {"flag": false}}`)

	_, visibleImages := totalRunsImages(visiblePages)
	_, hiddenImages := totalRunsImages(hiddenPages)

	if visibleImages != 1 {
		t.Fatalf("presence precondition: expected 1 image when visible, got %d", visibleImages)
	}
	if hiddenImages != 0 {
		t.Fatalf("AC1: expected exactly 0 images when hidden, got %d", hiddenImages)
	}
}

// TestVisibleIfHiddenIsByteIdenticalToElementDeleted is AC2's first
// clause: rendering with the element hidden by DATA must be byte-
// identical to rendering a SECOND template with the element deleted
// from the JSON outright — the strongest available statement of
// "absent from the page model entirely".
func TestVisibleIfHiddenIsByteIdenticalToElementDeleted(t *testing.T) {
	withCondition, err := ParseTemplate([]byte(visTextTemplateJSON(`"visibleIf": "customer.flag", `)))
	if err != nil {
		t.Fatalf("ParseTemplate(withCondition): %v", err)
	}
	data := Data(`{"customer": {"flag": false}}`)

	hiddenRes, err := Render(withCondition, data, nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(hidden): %v", err)
	}

	// elementDeleted still HAS e1 declared (visTextTemplateJSON always
	// emits it) — AC2 asks for a template with e1 DELETED, so build
	// that JSON directly rather than reusing the two-element builder.
	deletedJSON := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 200, "height": 20, "value": "Always visible", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	deletedTpl, err := ParseTemplate([]byte(deletedJSON))
	if err != nil {
		t.Fatalf("ParseTemplate(deleted): %v", err)
	}
	deletedRes, err := Render(deletedTpl, data, nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(deleted): %v", err)
	}

	if string(hiddenRes.Bytes) != string(deletedRes.Bytes) {
		t.Fatal("AC2: rendering with the element hidden by data must be byte-identical to rendering the same document with the element deleted from the template — a hidden element must be absent from the page model, not merely invisible")
	}

	// Second clause: e2's own coordinates must be IDENTICAL whether e1
	// is visible or hidden — siblings never move (AD-24's gap, not a
	// reflow).
	visiblePages, _, _, _, err := buildPageModel(withCondition, mustDecodeData(t, `{"customer": {"flag": true}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel(visible): %v", err)
	}
	hiddenPagesRaw, _, _, _, err := buildPageModel(withCondition, mustDecodeData(t, `{"customer": {"flag": false}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel(hidden): %v", err)
	}
	if len(visiblePages) != 1 || len(hiddenPagesRaw) != 1 {
		t.Fatalf("presence precondition: expected exactly one page in both renders, got %d/%d", len(visiblePages), len(hiddenPagesRaw))
	}
	// e1 visible: two runs, e1 then e2 (declaration order) — e2 is the
	// LAST run. e1 hidden: one run, e2 alone.
	visibleRuns := visiblePages[0].Runs
	hiddenRuns := hiddenPagesRaw[0].Runs
	if len(visibleRuns) != 2 || len(hiddenRuns) != 1 {
		t.Fatalf("presence precondition: expected 2/1 runs, got %d/%d", len(visibleRuns), len(hiddenRuns))
	}
	e2Visible := visibleRuns[len(visibleRuns)-1]
	e2Hidden := hiddenRuns[0]
	if e2Visible.X != e2Hidden.X {
		t.Errorf("AC2: e2's X moved between e1-visible (%d) and e1-hidden (%d) — siblings must never move", e2Visible.X, e2Hidden.X)
	}
	if e2Visible.Y != e2Hidden.Y {
		t.Errorf("AC2: e2's Y moved between e1-visible (%d) and e1-hidden (%d) — siblings must never move", e2Visible.Y, e2Hidden.Y)
	}
}

// TestVisibleIfHiddenImageIsByteIdenticalToElementDeleted is AC2's
// first clause, IMAGE half (Story 3.5 finisher review, Finding 1 /
// Blocker). The text-only test above shipped as AC2's sole subject,
// which is why a hidden image's /XObject stayed embedded in the PDF —
// pagemodel.Page.Images correctly showed zero images (AC1), but the
// bytes themselves were not byte-identical to the element-deleted
// rendering, because assetKeys/pdfImages were built from imageRuns
// BEFORE visibility was consulted. Both templates below carry the
// identical `assets` map and the identical always-visible text
// sibling e2; the only difference is whether e1 is declared-and-hidden
// or deleted outright.
func TestVisibleIfHiddenImageIsByteIdenticalToElementDeleted(t *testing.T) {
	assetKey := sha256Hex(png3x2RGB)
	dataJSON := `["` + strings.Join(wrap76(png3x2RGB), `","`) + `"]`
	assetsJSON := `{"` + assetKey + `": {"data": ` + dataJSON + `, "mediaType": "image/png"}}`

	withImageJSON := `{
  "assets": ` + assetsJSON + `,
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "image", "x": 0, "y": 0, "width": 100, "height": 60, "asset": "` + assetKey + `", "visibleIf": "customer.flag"},
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 200, "height": 20, "value": "Always visible", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	withImage, err := ParseTemplate([]byte(withImageJSON))
	if err != nil {
		t.Fatalf("ParseTemplate(withImage): %v", err)
	}
	data := Data(`{"customer": {"flag": false}}`)

	hiddenRes, err := Render(withImage, data, nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(hidden): %v", err)
	}

	// deletedJSON keeps the SAME assets map — e1 is deleted from the
	// element list, not from the document's assets — so the only
	// difference from withImageJSON is whether e1 is declared at all.
	deletedJSON := `{
  "assets": ` + assetsJSON + `,
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 200, "height": 20, "value": "Always visible", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	deletedTpl, err := ParseTemplate([]byte(deletedJSON))
	if err != nil {
		t.Fatalf("ParseTemplate(deleted): %v", err)
	}
	deletedRes, err := Render(deletedTpl, data, nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(deleted): %v", err)
	}

	if string(hiddenRes.Bytes) != string(deletedRes.Bytes) {
		t.Fatalf("AC2: rendering with the image element hidden by data must be byte-identical to rendering the element-deleted template (hidden=%d bytes, deleted=%d bytes) — a hidden image must not remain embedded as a PDF /XObject", len(hiddenRes.Bytes), len(deletedRes.Bytes))
	}

	// Second clause: e2's own coordinates must be identical whether e1
	// is visible or hidden.
	visiblePages, _, _, _, err := buildPageModel(withImage, mustDecodeData(t, `{"customer": {"flag": true}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel(visible): %v", err)
	}
	hiddenPagesRaw, _, _, _, err := buildPageModel(withImage, mustDecodeData(t, `{"customer": {"flag": false}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel(hidden): %v", err)
	}
	if len(visiblePages) != 1 || len(hiddenPagesRaw) != 1 {
		t.Fatalf("presence precondition: expected exactly one page in both renders, got %d/%d", len(visiblePages), len(hiddenPagesRaw))
	}
	if len(visiblePages[0].Runs) != 1 || len(hiddenPagesRaw[0].Runs) != 1 {
		t.Fatalf("presence precondition: expected e2's single run in both renders, got %d/%d", len(visiblePages[0].Runs), len(hiddenPagesRaw[0].Runs))
	}
	if len(visiblePages[0].Images) != 1 || len(hiddenPagesRaw[0].Images) != 0 {
		t.Fatalf("presence precondition: expected 1/0 images, got %d/%d", len(visiblePages[0].Images), len(hiddenPagesRaw[0].Images))
	}
	e2Visible := visiblePages[0].Runs[0]
	e2Hidden := hiddenPagesRaw[0].Runs[0]
	if e2Visible.X != e2Hidden.X || e2Visible.Y != e2Hidden.Y {
		t.Errorf("AC2: e2 moved between e1-visible (%d,%d) and e1-hidden (%d,%d) — siblings must never move", e2Visible.X, e2Visible.Y, e2Hidden.X, e2Hidden.Y)
	}
}

func mustDecodeData(t *testing.T, json string) bind.Value {
	t.Helper()
	v, err := bind.DecodeData(Data(json))
	if err != nil {
		t.Fatalf("DecodeData: %v", err)
	}
	return v
}

func mustDecodeParams(t *testing.T) bind.Value {
	t.Helper()
	v, err := decodeParams(nil)
	if err != nil {
		t.Fatalf("decodeParams: %v", err)
	}
	return v
}

// TestVisibleIfConditionSemanticsMatchIf is AC5: a table-driven test,
// one row per if()'s own five documented condition-resolution cases
// (eval.go:148-175), enumerated here rather than derived from evalIf's
// switch (D-000.68: deriving the cases from the implementation would
// be asking the code whether it agrees with itself).
func TestVisibleIfConditionSemanticsMatchIf(t *testing.T) {
	tpl, err := ParseTemplate([]byte(visTextTemplateJSON(`"visibleIf": "customer.flag", `)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	cases := []struct {
		name        string
		dataJSON    string
		wantErr     bool
		errContains string
		wantRuns    int // meaningful only when wantErr is false
	}{
		{name: "true is visible", dataJSON: `{"customer": {"flag": true}}`, wantRuns: 2},
		{name: "false is hidden", dataJSON: `{"customer": {"flag": false}}`, wantRuns: 1},
		{name: "explicit null is hidden, silently", dataJSON: `{"customer": {"flag": null}}`, wantRuns: 1},
		{name: "absent path is a located error", dataJSON: `{"customer": {}}`, wantErr: true, errContains: "flag"},
		{name: "string is a located error, no truthiness", dataJSON: `{"customer": {"flag": "false"}}`, wantErr: true, errContains: "string"},
		{name: "number is a located error, no truthiness", dataJSON: `{"customer": {"flag": 0}}`, wantErr: true, errContains: "number"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := Render(tpl, Data(c.dataJSON), nil, testFontSet())
			if c.wantErr {
				if err == nil {
					t.Fatal("expected a located render error, got nil")
				}
				if !strings.Contains(err.Error(), c.errContains) {
					t.Errorf("error must mention %q, got: %v", c.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			pages, _, _, _, berr := buildPageModel(tpl, mustDecodeData(t, c.dataJSON), mustDecodeParams(t), testFontSet())
			if berr != nil {
				t.Fatalf("buildPageModel: %v", berr)
			}
			pf := make([]pagemodelPageForTest, len(pages))
			for i, p := range pages {
				pf[i] = pagemodelPageForTest{runs: len(p.Runs), images: len(p.Images)}
			}
			runs, _ := totalRunsImages(pf)
			if runs != c.wantRuns {
				t.Errorf("expected %d run(s), got %d", c.wantRuns, runs)
			}
			_ = res
		})
	}
}

// TestVisibleIfNullConditionEmitsNoDiagnostic is D-3.2.3's own
// "silent false, no diagnostic of any severity" clause, checked
// specifically against Result.Diagnostics rather than only against the
// absence of a render error.
func TestVisibleIfNullConditionEmitsNoDiagnostic(t *testing.T) {
	tpl, err := ParseTemplate([]byte(visTextTemplateJSON(`"visibleIf": "customer.flag", `)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data(`{"customer": {"flag": null}}`), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("an explicit-null condition must emit NO diagnostic of any severity, got: %+v", res.Diagnostics)
	}
}

// TestStyleFieldIsNeverDataDriven is AC4, rebuilt after DECISION-1
// (ruled): a style field containing a "{{ }}" placeholder is now a
// LOAD ERROR (see TestParseTemplateRejectsPlaceholderInStyleField,
// folio8_expr_validate_test.go), so AC4's original two-data-sets-select-
// different-branches subject is no longer constructible — that
// rejection proves "an author's ATTEMPT to make style data-driven is
// refused", a narrower property than AC4's own.
//
// AC4 proves something the rejection does NOT: that a style field
// NEVER varies with data by ANY route, for a template that loads
// clean. The subject below carries a literal (placeholder-free) style
// value — style.fontFamily: "body" — and a text binding that DOES read
// data, rendered against two data documents that differ only in a
// field the template never binds anywhere.
//
// Story 3.5 finisher review, Finding 3 (Major), and the engineering
// lead's ruling: fontFamily is deliberately the field under test here,
// not background (the field the pre-ruling subject implied). The
// population of ten style fields was measured by construction and it
// dissolves the "which subject" question rather than leaving it a
// judgment call:
//
//   - fontFamily is the ONLY style field that is both a string and has
//     a render consumer today (fontChain, this file's render.go) — so
//     it is the only field a "conditional formatting" implementation
//     could plausibly target that this test can catch by a REAL
//     mutation. Red-proof, run and reverted (finisher Delivery Log):
//     making font-chain resolution consult `data` at all reddens this
//     test — either by byte divergence or by turning one of the two
//     data sets' otherwise-identical renders into an error.
//   - fontSize is type-impossible for this defect: it is
//     Presence[geom.Length], so a "{{ }}" string cannot inhabit it —
//     never mind a data-driven value.
//   - The remaining eight fields (background, align, valign, border,
//     bold, italic, padding, altRowBackground — DECISION-1's own
//     six-field style list plus table's altRowBackground) have NO
//     render consumer outside internal/template's own parse/serialize
//     and the load-time rejection check itself. "Style does not vary
//     with data" is TRUE of them today because they are inert, and no
//     assertion over them can have teeth until Epic 4/4.8 gives them a
//     consumer (Story 4.1's cell borders/padding, Story 4.8's
//     altRowBackground) — D-000.24's labelled category used correctly:
//     not "we could not prove it," but "there is nothing here to prove
//     yet, and this is the measured count." That obligation is named
//     against Stories 4.1 and 4.8 in this story's own text (AC4
//     section) so they inherit it rather than rediscover it.
func TestStyleFieldIsNeverDataDriven(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "Statement for {{customer.name}}", "style": {"fontFamily": "body", "fontSize": 14, "background": "#FF0000"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	resA, err := Render(tpl, Data(`{"customer": {"name": "Ada Lovelace", "overdue": true}}`), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(overdue=true): %v", err)
	}
	resB, err := Render(tpl, Data(`{"customer": {"name": "Ada Lovelace", "overdue": false}}`), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(overdue=false): %v", err)
	}
	if string(resA.Bytes) != string(resB.Bytes) {
		t.Fatal("AC4: a style field must not vary with data — output diverged between two data sets that differ only in a field the template never binds")
	}
}

// visBrokenFontFamilyTemplateJSON, visMissingAssetTemplateJSON and
// visBadTableBindTemplateJSON are AC7's three named subjects: a hidden
// element that is ALSO independently invalid. Each still produces its
// EXISTING located error, unchanged, exactly as it does when visible —
// R2: visibility suppresses OUTPUT, never VALIDATION.
const visBrokenFontFamilyTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "unreachable", "visibleIf": "customer.flag", "style": {"fontFamily": "nonexistentChain", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const visMissingAssetTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "image", "x": 0, "y": 0, "width": 100, "height": 60, "asset": "0000000000000000000000000000000000000000000000000000000000000000", "visibleIf": "customer.flag"}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const visBadTableBindTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "customer.name", "headerHeight": 14, "visibleIf": "customer.flag",
          "columns": [
            {"id": "e2", "label": "Amount", "width": 80, "bind": "{{transaction.amount}}"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestHiddenElementStillFailsItsOwnValidation is AC7: for each of the
// three subjects, rendering with the element HIDDEN (customer.flag ==
// false) still produces the SAME located error as rendering it VISIBLE
// (customer.flag == true) — render.go:601-616's precedent (Story 2.5,
// QA Finding 5, Major) applied to visibility instead of an empty-text
// short-circuit.
func TestHiddenElementStillFailsItsOwnValidation(t *testing.T) {
	cases := []struct {
		name        string
		tplJSON     string
		errContains string
	}{
		{name: "unresolvable fontFamily chain", tplJSON: visBrokenFontFamilyTemplateJSON, errContains: "nonexistentChain"},
		// Story 3.5 finisher review, Finding 7 (Minor): "asset" alone
		// cannot discriminate this subject's OWN error from a different
		// asset failure on the same path ("folio8: Render: asset %q:
		// %w" from DecodeAssetBytes, or DecodeImageForRender's own
		// text also say "asset"). The distinctive substring, plus the
		// element id, is what AC7's "unchanged in text AND IN LOCATION"
		// actually requires — and it is exactly the code path Finding
		// 1's fix (pdfImages built from visible-only asset keys)
		// touches, so an under-specified assertion here would matter
		// now, not hypothetically.
		{name: "asset key absent from assets map", tplJSON: visMissingAssetTemplateJSON, errContains: "element e1: an image element references asset \"0000000000000000000000000000000000000000000000000000000000000000\", which is not present in the document's assets map"},
		// "table bind does not resolve to an array" is a REGRESSION
		// GUARD with NO CONSTRUCTIBLE RED-PROOF at HEAD, labelled as
		// such per D-000.24 (Story 3.5 finisher review, Finding 5 /
		// Minor): checkTableBindings (render.go) runs BEFORE
		// computeVisibility even exists in the pipeline, and no table
		// element's visibility verdict is ever consulted anywhere — so
		// there is no "move the skip above validation" mutation to run
		// for a table, unlike the text and image subjects above (M1,
		// M2, both reddened and are recorded in the Delivery Log). The
		// story's AC7 text previously implied this subject redded
		// against a mutation the way the other two do; it does not, and
		// this is worth keeping anyway — a table gaining a visibility
		// consumer in Epic 4 without this subject continuing to pass
		// would be worth noticing.
		{name: "table bind does not resolve to an array", tplJSON: visBadTableBindTemplateJSON, errContains: "not an array"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(c.tplJSON))
			if err != nil {
				t.Fatalf("ParseTemplate: %v", err)
			}
			for _, flag := range []bool{true, false} {
				dataJSON := `{"customer": {"flag": true, "name": "not an array"}}`
				if !flag {
					dataJSON = `{"customer": {"flag": false, "name": "not an array"}}`
				}
				_, err := Render(tpl, Data(dataJSON), nil, testFontSet())
				if err == nil {
					t.Fatalf("flag=%v: expected a located render error, got nil — hidden must never suppress VALIDATION, only output", flag)
				}
				if !strings.Contains(err.Error(), c.errContains) {
					t.Errorf("flag=%v: error must mention %q, got: %v", flag, c.errContains, err)
				}
			}
		})
	}
}

// TestHiddenElementEmitsNoDiagnosticOfItsOwn is AC8: a text element
// that, when visible, emits DiagCodeTextClippedWidth (its content
// exceeds its declared width) emits NO diagnostic at all when hidden —
// and every OTHER element's diagnostics are unaffected, in the same
// order, with the hidden element removed from the MIDDLE.
//
// Story 3.5 finisher review, Finding 9 (Minor): the shipped version of
// this test had exactly two content-band elements and hid the FIRST —
// there was no middle to remove, and no cross-band case, so AC8's own
// singled-out clause ("removing an element from the middle must not
// perturb the rest," across header/content/footer document order) was
// never exercised. This version puts one clipping element in EACH band
// (e4/header, e5/footer) plus THREE in content (e1, e2 — hidden, e3),
// so e2 is genuinely in the middle of its own band, and asserts the
// WHOLE Diagnostics slice — element ids, in document order — against a
// literal this test owns, not merely a count.
func TestHiddenElementEmitsNoDiagnosticOfItsOwn(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 20, "height": 20, "value": "This line is far too long for its box", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 50, "width": 20, "height": 20, "value": "This line is also far too long", "visibleIf": "customer.flag", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "text", "x": 0, "y": 100, "width": 20, "height": 20, "value": "This one clips too, always visible", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [
      {"id": "e5", "type": "text", "x": 0, "y": 4, "width": 20, "height": 16, "value": "Footer text that clips its narrow box", "style": {"fontFamily": "body", "fontSize": 9}}
    ], "height": 20},
    "pageHeader": {"elements": [
      {"id": "e4", "type": "text", "x": 0, "y": 2, "width": 20, "height": 16, "value": "Header text that clips its narrow box", "style": {"fontFamily": "body", "fontSize": 9}}
    ], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	elementIDs := func(diags []Diagnostic) []string {
		ids := make([]string, len(diags))
		for i, d := range diags {
			ids[i] = d.ElementID
			if d.Code != DiagCodeTextClippedWidth {
				t.Errorf("diagnostic %d: expected code %q, got %q", i, DiagCodeTextClippedWidth, d.Code)
			}
		}
		return ids
	}

	visibleRes, err := Render(tpl, Data(`{"customer": {"flag": true}}`), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(visible): %v", err)
	}
	wantVisible := []string{"e4", "e1", "e2", "e3", "e5"}
	if got := elementIDs(visibleRes.Diagnostics); !slices.Equal(got, wantVisible) {
		t.Fatalf("presence precondition: visible diagnostics = %v, want %v (header, content in declaration order, footer)", got, wantVisible)
	}

	hiddenRes, err := Render(tpl, Data(`{"customer": {"flag": false}}`), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render(hidden): %v", err)
	}
	// AC8: e2 removed from the MIDDLE of the content band, and neither
	// the header's nor the footer's diagnostic — nor the content band's
	// own siblings — is perturbed in content, order or count.
	wantHidden := []string{"e4", "e1", "e3", "e5"}
	if got := elementIDs(hiddenRes.Diagnostics); !slices.Equal(got, wantHidden) {
		t.Fatalf("AC8: hidden diagnostics = %v, want %v (e2 removed from the middle, nothing else perturbed)", got, wantHidden)
	}

	// The four surviving diagnostics must be byte-identical to their
	// visible-render counterparts (eh, e1, e3, ef at visible indices
	// 0, 1, 3, 4) — not merely "the same element ids."
	survivingVisibleIdx := []int{0, 1, 3, 4}
	for i, vi := range survivingVisibleIdx {
		if hiddenRes.Diagnostics[i] != visibleRes.Diagnostics[vi] {
			t.Errorf("AC8: diagnostic %d (%s) must be byte-identical whether e2 is visible or hidden; got %+v vs %+v",
				i, wantHidden[i], hiddenRes.Diagnostics[i], visibleRes.Diagnostics[vi])
		}
	}
}

// TestVisibilityIsConsistentAcrossPages is AC9's behavioural regression
// half: a header/footer element's visibility verdict is the SAME on
// every page of a multi-page document. computeVisibility's SIGNATURE
// (render_visibility.go) is AC9's real anchor — it takes bands, data,
// params and fc, and CANNOT receive pageCount or any page-derived
// value, so a page-dependent verdict is a compile error. This test is
// the regression guard alongside that compile-time proof.
func TestVisibilityIsConsistentAcrossPages(t *testing.T) {
	tplJSON := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 700, "value": "` + strings.Repeat("word ", 400) + `", "style": {"fontFamily": "body", "fontSize": 24}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 6, "width": 480, "height": 16, "value": "FOOTER", "visibleIf": "customer.flag", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 4, "width": 480, "height": 16, "value": "HEADER", "visibleIf": "customer.flag", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{"customer": {"flag": true}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel(flag=true): %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("presence precondition: expected more than one page, got %d", len(pages))
	}
	// Story 3.5 finisher review, Finding 2 (Major): `len(p.Runs) == 0`
	// cannot discriminate anything here, because the content element
	// (`strings.Repeat("word ", 400)`) puts runs on every page unaided —
	// the loop measured the wrong thing, not the right thing weakly.
	// The fix, per the engineering lead: assert a PER-PAGE SET of which
	// of {"HEADER","FOOTER"} are present (by SourceText, the only
	// per-run signal available to distinguish them from the content
	// band's own runs), not merely a run count. That set must be BOTH
	// present on EVERY page when visible — exactly what mutation M8
	// (header/footer emitted on page 1 only) falsifies: pages 2..N would
	// report an incomplete set while page 1 still reports the full one.
	for i, p := range pages {
		var sawHeader, sawFooter bool
		for _, r := range p.Runs {
			switch r.SourceText {
			case "HEADER":
				sawHeader = true
			case "FOOTER":
				sawFooter = true
			}
		}
		if !sawHeader || !sawFooter {
			t.Errorf("page %d: expected BOTH the header and footer runs to be present when visible, got header=%v footer=%v", i, sawHeader, sawFooter)
		}
	}

	hiddenPages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{"customer": {"flag": false}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel(flag=false): %v", err)
	}
	if len(hiddenPages) != len(pages) {
		t.Fatalf("presence precondition: page count must be unaffected by header/footer visibility, got %d vs %d", len(hiddenPages), len(pages))
	}
	for i, p := range hiddenPages {
		for _, r := range p.Runs {
			if r.SourceText == "HEADER" || r.SourceText == "FOOTER" {
				t.Errorf("page %d: header/footer must be hidden on EVERY page, found a run with SourceText %q", i, r.SourceText)
			}
		}
	}
}
