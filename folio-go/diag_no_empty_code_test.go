package folio8

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/diag"
	"github.com/panitw/folio8/folio-go/internal/expr"
)

// missingGlyphTemplateJSON is a synthetic, test-only template (never a
// committed fixture — OPEN-1's obligation (b)) declaring a single-face
// chain that covers no Thai glyph, so a Thai data value reliably fires
// the missing-glyph Warning this test needs on the module's public
// Render path.
const missingGlyphTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "style": {"fontFamily": "body", "fontSize": 14}}
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
  "version": "1.0"
}
`

// TestNoDiagnosticCompositeLiteralOmitsCode is AC7 part 2's AST half:
// no `Diagnostic{...}` composite literal in this package's PRODUCTION
// (non-test) source may omit the Code field. This is the property over
// the CLASS of construction sites R12/D-3.6.7 requires — present sites
// (render.go, render_error.go) and any FUTURE one a later story adds,
// never a repair of the three sites that existed before this story.
//
// Scoped to the module-root folio8 package's own .go files (where every
// Diagnostic{...} literal lives today — Diagnostic is a root-package
// type, and nothing under internal/ constructs one, by AD-1's import
// boundary). Test files are excluded: a test's OWN Diagnostic{}
// literal (e.g. an expected-value table) is not a value reachable from
// Render/RenderTo, and AC7's property concerns what the library
// PRODUCES, not what a test compares against.
func TestNoDiagnosticCompositeLiteralOmitsCode(t *testing.T) {
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read module root: %v", err)
	}

	fset := token.NewFileSet()
	var filesScanned int
	var findings []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filesScanned++
		path := filepath.Join(dir, name)
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			ident, ok := cl.Type.(*ast.Ident)
			if !ok || ident.Name != "Diagnostic" {
				return true
			}
			hasCode := false
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				keyIdent, ok := kv.Key.(*ast.Ident)
				if !ok || keyIdent.Name != "Code" {
					continue
				}
				hasCode = true
				// A Code key present but set to the literal empty
				// string is exactly as bad as omitting it. And a Code
				// key set to a literal string naming NO registered
				// code is Finding 3's exact gap (QA review, Major):
				// the previous version checked only for non-emptiness,
				// so `Code: "MADE_UP_UNREGISTERED"` satisfied it. A
				// symbolic reference (e.g. `Code: DiagCodeTextMissingGlyph`)
				// cannot be resolved by this static scan, but every
				// DiagCode* identifier is bridged `= string(diag.CodeX)`
				// (AC3/R1) and so is registered by construction — only
				// a raw string literal can smuggle an unregistered
				// value past this guard, which is what is checked here.
				if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, uerr := strconv.Unquote(lit.Value); uerr == nil {
						switch {
						case v == "":
							hasCode = false
						case !diag.Registered(diag.Code(v)):
							p := fset.Position(kv.Pos())
							findings = append(findings, p.String()+": Diagnostic Code literal "+strconv.Quote(v)+" is not a member of the registry as constructed")
						}
					}
				}
			}
			if !hasCode {
				p := fset.Position(cl.Pos())
				findings = append(findings, p.String()+": Diagnostic{...} composite literal has no non-empty Code field")
			}
			return true
		})
	}

	if filesScanned == 0 {
		t.Fatalf("vacuity guard: scanned 0 production .go files in the module root")
	}
	if len(findings) > 0 {
		t.Fatalf("AC7: every Diagnostic reachable from Render/RenderTo must carry a registry Code:\n%s", strings.Join(findings, "\n"))
	}
}

// TestAllProducedDiagnosticsCarryARegisteredCode is AC7 part 2's
// runtime half: over every Diagnostic this module's KNOWN production
// paths can actually construct — the clip Warning, the empty-average
// Warning, the missing-glyph Warning and diagnosticFromCaveat's
// `default:` arm (exercised directly, in-package, with a FABRICATED
// expr.CaveatKind value outside the closed set — the only way to reach
// an arm that is unreachable through the closed set as it ships today,
// per its own doc comment) — the Code is a member of the registry as
// constructed (internal/diag.Registered), never merely non-empty.
//
// Mutation (AC7): set a construction site's Code to an unregistered
// string, or add a second expr.CaveatKind member without a matching
// arm, and this test must redden — recorded in the Delivery Log.
func TestAllProducedDiagnosticsCarryARegisteredCode(t *testing.T) {
	var got []Diagnostic

	// The clip Warning (render.go).
	clipRes := renderClipTemplate(t, clipNarrowTemplate)
	got = append(got, clipRes.Diagnostics...)

	// The empty-average Warning (render.go, via diagnosticFromCaveat's
	// matched arm), through the public Render path.
	tpl, err := ParseTemplate([]byte(emptyAverageTemplateJSON("")))
	if err != nil {
		t.Fatalf("parse empty-average template: %v", err)
	}
	avgRes, err := Render(tpl, Data(`{"t":[]}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render empty-average fixture: %v", err)
	}
	got = append(got, avgRes.Diagnostics...)

	// The missing-glyph Warning (render.go, shapeSegments) — Finding 3
	// (QA review, Major): this doc comment already claimed this path
	// was covered, but the body never exercised it, so a construction
	// site set to an unregistered literal Code satisfied this test
	// with the mutation caught only incidentally, by AC4's own
	// dispatch test. "ก" (Thai) has no glyph in Noto Sans and the
	// declared chain names no fallback, exactly as
	// TestMissingGlyphDiagnosticFiresOnUncoveredRune (ac4_coverage_test.go)
	// exercises it, via the public Render path.
	glyphTpl, err := ParseTemplate([]byte(missingGlyphTemplateJSON))
	if err != nil {
		t.Fatalf("parse missing-glyph template: %v", err)
	}
	glyphRes, err := Render(glyphTpl, Data(`{"name":"ก"}`), Params(`{}`), testShippedFontSet())
	if err != nil {
		t.Fatalf("render missing-glyph fixture: %v", err)
	}
	got = append(got, glyphRes.Diagnostics...)

	// diagnosticFromCaveat's matched arm, exercised directly too (belt
	// and braces — the Render path above already covers it).
	got = append(got, diagnosticFromCaveat("e-probe", expr.Caveat{Kind: expr.CaveatEmptyAverage, Path: "x"}))

	// diagnosticFromCaveat's default arm, exercised DIRECTLY: a
	// fabricated Kind value outside expr.CaveatKind's closed set (its
	// only member is CaveatEmptyAverage), which is exactly what the
	// arm's own doc comment says is otherwise unreachable.
	got = append(got, diagnosticFromCaveat("e-unhandled", expr.Caveat{Kind: expr.CaveatKind(99), Path: "y"}))

	if len(got) == 0 {
		t.Fatal("presence precondition: collected zero Diagnostics to check")
	}
	for _, d := range got {
		if !diag.Registered(diag.Code(d.Code)) {
			t.Errorf("Diagnostic %+v carries Code %q, which is not a member of the registry as constructed", d, d.Code)
		}
	}
}
