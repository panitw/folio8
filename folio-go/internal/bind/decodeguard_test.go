package bind

// AC20 (D-1.7.5): params and report data share ONE decode path — the
// same UseNumber discipline, the same single literal splitter
// (D-1.6.1), the same Decimal. "A second decoder would be the drift
// hazard refused three times already — and worse here, since params
// carry dates and amounts, where a divergence would be least visible
// and most expensive" (D-1.7.5, verbatim). This file asserts the
// property by AST (D-000.14: a load-bearing count is derived from a
// parsed AST, never text matching), not by re-reading DecodeData's
// source and trusting it stayed singular.
//
// This story's review, Finding 2 (Major): counting ONLY UseNumber call
// sites catches the harmless duplicate (a second json.NewDecoder that
// ALSO calls UseNumber) but misses the harmful one — a second decoder
// that omits UseNumber and silently narrows every number literal
// through float64, or a bare json.Unmarshal — exactly the divergence
// D-1.7.5 names as "least visible and most expensive". The guard now
// counts json.NewDecoder and json.Unmarshal sites alongside UseNumber
// and asserts all three counts, so a second decoder of ANY shape
// reddens, not only one that copies the first one's discipline.
//
// Scope note (Finding 2's secondary point): this scan is deliberately
// NOT recursive — it walks files directly under dir only. Today
// internal/bind has no subpackages, so this is not a live gap; if a
// subpackage is ever added under internal/bind, it is invisible to
// this guard until this comment is revisited.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodeCallSites walks every non-test .go file directly under dir and
// returns the "file:line" location of every *ast.CallExpr matching one
// of three shapes, by AST, never by grep (D-000.14):
//
//   - newDecoder: a call to json.NewDecoder (identified by the local
//     import alias for "encoding/json", so a renamed import cannot
//     hide a site)
//   - useNumber:  a call whose selector is named "UseNumber"
//   - unmarshal:  a call to json.Unmarshal — the OTHER decode entry
//     point encoding/json exposes, which silently narrows every
//     number literal through float64 and never calls UseNumber at all
func decodeCallSites(t *testing.T, dir string) (newDecoder, useNumber, unmarshal []string, filesSeen int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filesSeen++
		path := filepath.Join(dir, name)
		file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}

		jsonAlias := ""
		for _, imp := range file.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p != "encoding/json" {
				continue
			}
			alias := "json"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			jsonAlias = alias
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pos := fset.Position(sel.Pos())
			loc := name + ":" + strconvItoa(pos.Line)

			if sel.Sel.Name == "UseNumber" {
				useNumber = append(useNumber, loc)
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || jsonAlias == "" || pkgIdent.Name != jsonAlias {
				return true
			}
			switch sel.Sel.Name {
			case "NewDecoder":
				newDecoder = append(newDecoder, loc)
			case "Unmarshal":
				unmarshal = append(unmarshal, loc)
			}
			return true
		})
	}
	return newDecoder, useNumber, unmarshal, filesSeen
}

// strconvItoa avoids importing "strconv" for a single conversion,
// matching this package's existing no-frills style.
func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TestExactlyOneUseNumberSiteUnderBind is AC20's guard. Vacuity guard
// (AC26 Q1, D-000.9): zero files seen is a hard failure — a scan that
// looked at nothing must not be able to report the same "count == 1"
// a healthy run reports by coincidence.
//
// Finding 2's fix: the guard now asserts THREE counts, not one — a
// second decoder that omits UseNumber (bumping newDecoder to 2 while
// useNumber stays at 1) or a bare json.Unmarshal (bumping unmarshal
// off zero) reddens exactly as a second UseNumber-calling decoder
// already did.
func TestExactlyOneUseNumberSiteUnderBind(t *testing.T) {
	newDecoder, useNumber, unmarshal, filesSeen := decodeCallSites(t, ".")
	if filesSeen == 0 {
		t.Fatalf("vacuity guard: scanned zero non-test files under internal/bind — AC20's property is unassertable (D-000.9)")
	}
	if len(newDecoder) != 1 {
		t.Fatalf("AC20: expected exactly ONE json.NewDecoder call site under internal/bind, found %d: %v", len(newDecoder), newDecoder)
	}
	if len(useNumber) != 1 {
		t.Fatalf("AC20: expected exactly ONE UseNumber call site under internal/bind, found %d: %v", len(useNumber), useNumber)
	}
	if len(unmarshal) != 0 {
		t.Fatalf("AC20: json.Unmarshal must never appear under internal/bind (it discards UseNumber's precision guarantee) — found %d: %v", len(unmarshal), unmarshal)
	}
}
