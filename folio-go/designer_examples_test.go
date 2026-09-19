package folio8

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The folio8 Designer ships example templates (startup templates, CAP-4) from
// folio-designer/public/templates/examples/: each <id>.folio beside its
// <id>.sample.json. The designer build renders them with the CLI in strict mode
// and fails on any diagnostic; this test holds the same two properties in
// `go test`, so an engine change that breaks an example reds here first:
//
//   - the committed bytes are canonical, so a no-op load/save round trip in the
//     designer is byte-identical;
//   - rendering against the sample data raises zero diagnostics.
func TestDesignerExamplesAreCanonicalAndRenderClean(t *testing.T) {
	dir := filepath.Join(repoRootFromTest(t), "folio-designer", "public", "templates", "examples")
	templates, err := filepath.Glob(filepath.Join(dir, "*.folio"))
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) == 0 {
		t.Fatalf("no example templates found in %s", dir)
	}
	for _, path := range templates {
		id := strings.TrimSuffix(filepath.Base(path), ".folio")
		t.Run(id, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tpl, err := ParseTemplate(source)
			if err != nil {
				t.Fatalf("ParseTemplate(%s): %v", id, err)
			}
			canonical, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatalf("SerializeTemplate(%s): %v", id, err)
			}
			if !bytes.Equal(source, canonical) {
				t.Errorf("%s.folio is not in canonical form; SerializeTemplate writes:\n%s", id, canonical)
			}
			sample, err := os.ReadFile(filepath.Join(dir, id+".sample.json"))
			if err != nil {
				t.Fatalf("%s has no sample data: %v", id, err)
			}
			res, err := Render(tpl, Data(sample), nil, testShippedFontSet())
			if err != nil {
				t.Fatalf("Render(%s): %v", id, err)
			}
			requireNoDiagnostics(t, id, res.Diagnostics)
			if !bytes.HasPrefix(res.Bytes, []byte("%PDF-")) {
				t.Fatalf("%s: Render did not return a PDF", id)
			}
			// Per-resource decoding: the examples embed several faces, so a
			// merged ToUnicode map would recover the wrong text.
			var pages [][]string
			for _, runs := range statementPageRuns(t, res.Bytes) {
				pages = append(pages, statementPageTexts(runs))
			}
			want, listed := designerExamplePages[id]
			if !listed {
				t.Fatalf("%s has no page-count expectation in designerExamplePages", id)
			}
			if len(pages) < want.min || len(pages) > want.max {
				t.Errorf("%s renders %d page(s) against its sample; want %d to %d", id, len(pages), want.min, want.max)
			}
			for _, group := range designerExampleSamePage[id] {
				requireTextsOnOnePage(t, id, pages, group)
			}
		})
	}
}

// designerExamplePages is each example's page count against its own sample,
// so pagination the example exists to show is actually exercised.
var designerExamplePages = map[string]struct{ min, max int }{
	"invoice":          {1, 1},
	"bank-statement":   {2, 2},
	"legal-contract":   {2, 2},
	"electricity-bill": {1, 1},
}

// designerExampleSamePage lists texts that must be drawn, each exactly once and
// together on one page: the contract's two signatory names share a keepTogether
// group, and the bill's overdue notice is shown because its sample sets
// `overdue: true` (a broken `visibleIf` would silently drop it).
var designerExampleSamePage = map[string][][]string{
	"legal-contract":   {{"Signed by the authorised representatives", "Owen Castellan", "Priya Lindqvist"}},
	"electricity-bill": {{"PAYMENT OVERDUE"}},
}

func requireTextsOnOnePage(t *testing.T, id string, pages [][]string, texts []string) {
	t.Helper()
	refPage, refText := -1, ""
	for _, want := range texts {
		total, page := 0, -1
		for p, runs := range pages {
			if n := strings.Count(strings.Join(runs, "\n"), want); n > 0 {
				total += n
				page = p
			}
		}
		if total != 1 {
			t.Errorf("%s: %q is drawn %d time(s); want exactly once", id, want, total)
			continue
		}
		if refPage == -1 {
			refPage, refText = page, want
		} else if page != refPage {
			t.Errorf("%s: %q is on page %d but %q is on page %d; want them on the same page", id, want, page+1, refText, refPage+1)
		}
	}
}
