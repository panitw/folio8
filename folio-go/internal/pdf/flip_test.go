package pdf

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is AC17a's guard (D-1.8.10): a POSITIVE assertion that
// EVERY content-stream coordinate-emission site obtains its Y value
// from flipY — not a search for "nobody else writes a minus sign",
// which D-1.8.10 rejects as unfalsifiable.
//
// Rebuilt for Finding 3 (Story 1.8 review): the previous version keyed
// its notion of "candidate site" on a NAMING CONVENTION — a bare
// identifier ending in "Y" passed to appendLength/writeLength, derived
// from a directly-assigned `flipY(...)` call. That is a style check,
// not a routing assertion, and the reviewer open-coded a real second
// flip in a copy of the real imagedoc.go three ways: a Y-suffixed
// variable (caught by the old guard), a differently-named variable
// ("bottomEdge" — silently invisible to it, not even counted as a
// candidate) and an inline expression with no variable at all (same
// blind spot). Two of three passed green.
//
// The guard below is operand-shape-independent. It does not look at
// variable names, and it does not require the value to reach
// appendLength/writeLength at all (D-1.8.10's own suggested simplest
// sound form): it asserts, structurally, that NO function in this
// package other than flipY itself (flip.go, excluded below as flipY's
// own declaration site) contains a subtraction expression whose
// left-associative chain bottoms out at a page-height-shaped operand
// (a "X.Height" selector, or a bare "pageHeight" identifier — flipY's
// own parameter name). Since flip.go/textdoc.go/imagedoc.go together
// are the only places in the module that ever compute
// pageHeight-minus-something (verified: `grep -n "Height -" internal/pdf/*.go`
// finds nothing else), this is sound with no false-positive risk today,
// and — unlike the naming convention it replaces — it cannot be
// defeated by renaming a variable or inlining an expression, because it
// never looks at variable names or call sites at all.

const ruleFlipRouting = "flip-y-routing"

type flipFinding struct {
	path string
	line int
	msg  string
}

// scanFlipRouting scans every PRODUCTION (non-test) .go file in dir
// except flip.go itself. It returns:
//   - findings: every page-height-derived subtraction found outside
//     flipY — a real AD-24 violation, regardless of variable naming or
//     whether the result reaches appendLength/writeLength.
//   - flipCallsByFile: D-000.9's coverage witness, refined per Finding
//     13 (Story 1.8 review) — the previous witness was `candidates ==
//     0`, a single global counter whose message claimed a stronger,
//     UNENFORCED per-file property ("textdoc.go's buildTextContentStream
//     and imagedoc.go's appendImageContentStream must both contribute at
//     least one candidate each"); this return value lets the caller
//     enforce that literally, per file, by counting genuine direct
//     calls to flipY(...) in each file.
//   - filesScanned: D-000.9's blunter vacuity witness (did the scan look
//     at anything at all).
func scanFlipRouting(dir string) (findings []flipFinding, flipCallsByFile map[string]int, filesScanned int, err error) {
	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		return nil, nil, 0, rerr
	}
	flipCallsByFile = map[string]int{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "flip.go" {
			continue // flipY's own declaration site, not a candidate emission site
		}
		path := filepath.Join(dir, name)
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil, nil, 0, perr
		}
		filesScanned++

		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}

			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.CallExpr:
					if fn, ok := v.Fun.(*ast.Ident); ok && fn.Name == "flipY" {
						flipCallsByFile[name]++
					}
					return true
				case *ast.BinaryExpr:
					if v.Op != token.SUB {
						return true
					}
					if !isPageHeightDerivedChain(v) {
						return true // some other subtraction, unrelated to page height
					}
					pos := fset.Position(v.Pos())
					findings = append(findings, flipFinding{
						path: name, line: pos.Line,
						msg: filepath.Base(name) + ":" + itoaFlip(pos.Line) + ": function " + fd.Name.Name +
							" computes a page-height-derived subtraction directly, not via a call to " +
							"flipY (AD-24, D-1.8.10: exactly one function may invert a coordinate)",
					})
					// Do not descend into this chain's children: in a
					// left-associative chain every nested SUB node shares
					// the same leftmost leaf, so recursing would report
					// the same violation once per operator instead of
					// once per chain.
					return false
				}
				return true
			})
		}
	}
	return findings, flipCallsByFile, filesScanned, nil
}

// isPageHeightDerivedChain reports whether bin's left-associative
// subtraction chain (bin.X, and bin.X.X, and so on) bottoms out at a
// page-height-shaped leaf: a "<something>.Height" selector (how both
// real call sites — textdoc.go's and imagedoc.go's — spell it,
// `page.Height`) or a bare "pageHeight" identifier (flipY's own
// parameter name, in case a future site takes it unpacked rather than
// through a "page" struct).
func isPageHeightDerivedChain(bin *ast.BinaryExpr) bool {
	x := bin.X
	for {
		inner, ok := x.(*ast.BinaryExpr)
		if !ok || inner.Op != token.SUB {
			break
		}
		x = inner.X
	}
	switch v := x.(type) {
	case *ast.SelectorExpr:
		return v.Sel.Name == "Height"
	case *ast.Ident:
		return v.Name == "pageHeight"
	}
	return false
}

func itoaFlip(n int) string {
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

// TestContentStreamYCoordinatesRouteThroughFlipY is AC17/AC17a's
// production gate: no function in this package other than flipY may
// compute a page-height-derived subtraction directly.
func TestContentStreamYCoordinatesRouteThroughFlipY(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	findings, flipCallsByFile, filesScanned, err := scanFlipRouting(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if filesScanned == 0 {
		t.Fatal("vacuity guard: scanned zero files (D-000.9)")
	}
	// Finding 13 (Story 1.8 review): this per-file requirement is
	// enforced literally now, not just claimed in a message next to a
	// weaker global `candidates == 0` check. Both files must genuinely
	// call flipY at least once, or this guard is not exercising the two
	// real content-stream emitters it exists to police.
	for _, mustCall := range []string{"textdoc.go", "imagedoc.go"} {
		if flipCallsByFile[mustCall] == 0 {
			t.Fatalf("vacuity guard (D-000.9, Finding 13): %s contributes zero direct flipY(...) calls — "+
				"this guard is not exercising it", mustCall)
		}
	}
	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.msg)
		}
		t.Fatalf("AC17a violation(s):\n%s", strings.Join(msgs, "\n"))
	}
}

// TestFlipRoutingRedProof is RP-7 (Finding 3, Story 1.8 review): all
// three bypass shapes the reviewer demonstrated against a copy of the
// REAL imagedoc.go must redden, not just the Y-suffixed-variable one
// the old naming-convention guard happened to catch. The real file's
// current bytes are read from disk and a bypass function is appended —
// so this red-proof runs against production source, not a synthetic
// fixture invented to fit the scanner.
func TestFlipRoutingRedProof(t *testing.T) {
	real, err := os.ReadFile("imagedoc.go")
	if err != nil {
		t.Fatalf("read real imagedoc.go: %v", err)
	}

	// Control: the real, unmodified file must scan clean — the
	// legitimate `imgY := flipY(page.Height, page.MarginTop, im.Y,
	// im.DrawHeight)` call must never itself be flagged.
	t.Run("control_unmodified_real_file", func(t *testing.T) {
		dir := writeScratchImagedoc(t, string(real))
		findings, flipCalls, _, err := scanFlipRouting(dir)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if flipCalls["imagedoc.go"] == 0 {
			t.Fatal("vacuity guard: the real imagedoc.go must contain at least one flipY(...) call")
		}
		if len(findings) != 0 {
			t.Fatalf("control: the real imagedoc.go must not be flagged, got: %+v", findings)
		}
	})

	// Bypass (a): a Y-suffixed bare identifier — the shape the OLD
	// naming-convention guard happened to catch. Kept as a regression
	// check: the new structural guard must still catch it too.
	t.Run("bypass_Y_suffixed_variable", func(t *testing.T) {
		extra := `
func bypassSecondY(dst []byte, page pageLike, im imLike) []byte {
	secondY := page.Height - page.MarginTop - im.Y - im.DrawHeight
	return appendLength(dst, secondY)
}
`
		mustRedden(t, string(real)+extra, "bypassSecondY", "bypassSecondY")
	})

	// Bypass (b): a differently-named variable ("bottomEdge") — the
	// shape the OLD guard silently passed and did not even count as a
	// candidate (Finding 3's table).
	t.Run("bypass_non_Y_suffixed_variable", func(t *testing.T) {
		extra := `
func bypassBottomEdge(dst []byte, page pageLike, im imLike) []byte {
	bottomEdge := page.Height - page.MarginTop - im.Y - im.DrawHeight
	return appendLength(dst, bottomEdge)
}
`
		mustRedden(t, string(real)+extra, "bypassBottomEdge", "bypassBottomEdge")
	})

	// Bypass (c): an inline expression, no variable at all — the OLD
	// guard's other silent blind spot (Finding 3's table).
	t.Run("bypass_inline_expression", func(t *testing.T) {
		extra := `
func bypassInline(dst []byte, page pageLike, im imLike) []byte {
	return appendLength(dst, page.Height-page.MarginTop-im.Y-im.DrawHeight)
}
`
		mustRedden(t, string(real)+extra, "bypassInline", "bypassInline")
	})
}

// writeScratchImagedoc writes src to a fresh t.TempDir() as
// "imagedoc.go" and returns the directory.
func writeScratchImagedoc(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "imagedoc.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write imagedoc.go: %v", err)
	}
	return dir
}

// mustRedden asserts that scanning src (a mutated copy of the real
// imagedoc.go) produces at least one finding whose message names
// wantSubstr, and fails with the label if not.
func mustRedden(t *testing.T, src, wantSubstr, label string) {
	t.Helper()
	dir := writeScratchImagedoc(t, src)
	findings, flipCalls, _, err := scanFlipRouting(dir)
	if err != nil {
		t.Fatalf("scan (%s): %v", label, err)
	}
	if flipCalls["imagedoc.go"] == 0 {
		t.Fatalf("vacuity guard (%s): the real imgY := flipY(...) call must still be present and counted", label)
	}
	if len(findings) == 0 {
		t.Fatalf("RP-7 (%s): expected the bypass to redden, got no findings", label)
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.msg, wantSubstr) {
			found = true
		}
	}
	if !found {
		t.Fatalf("RP-7 (%s): expected a finding naming %q, got: %+v", label, wantSubstr, findings)
	}
}
