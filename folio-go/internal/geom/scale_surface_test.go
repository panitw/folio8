package geom

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExactlyOneExportedScalingFunction is AC18, D-1.5.2: "ScaleRound
// gets NO second variant... internal/geom gains no new exported scaling
// entry point in this story." AD-2: "Font scaling is one exported
// function with one documented rounding mode." D-1.5.2: "the fix is
// layering, not a second door" — Story 1.5 must add a validation layer
// in internal/fontset, never a second door into internal/geom.
//
// This scans internal/geom's own non-test .go files for every
// top-level exported func declaration and asserts the set is exactly
// {ScaleRound}. A future second scaling function — ScaleRoundF,
// ScaleTruncate, whatever its name — trips this the moment it is
// exported, regardless of what it is called (D-000.9: a rename cannot
// silently pass).
func TestExactlyOneExportedScalingFunction(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	var exportedFuncs []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	filesScanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filesScanned++
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil { // top-level functions only, not methods
				continue
			}
			if ast.IsExported(fd.Name.Name) {
				exportedFuncs = append(exportedFuncs, fd.Name.Name)
			}
		}
	}

	// Vacuity guard (D-000.9): what would this have printed if it were
	// unable to run at all? Zero files scanned is indistinguishable from
	// "scanned everything and it was all methods/unexported" unless
	// asserted separately.
	if filesScanned == 0 {
		t.Fatalf("vacuity guard: scanned zero non-test .go files under %s", dir)
	}

	want := []string{"ScaleRound"}
	if len(exportedFuncs) != len(want) || (len(exportedFuncs) > 0 && exportedFuncs[0] != want[0]) {
		t.Fatalf(
			"internal/geom's exported top-level functions = %v, want exactly %v "+
				"(AC18, D-1.5.2: no second scaling door — layer validation elsewhere instead)",
			exportedFuncs, want,
		)
	}
}
