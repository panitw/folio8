package arch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ruleDiagZeroImports is this guard's stable rule id (D-000.68's
// finding-identity convention).
const ruleDiagZeroImports = "diag-zero-first-party-imports"

// TestDiagPackageHasZeroFirstPartyImports is AC1's SECOND, independent
// anchor (alongside lint's ScanStageRank): internal/diag imports NO
// first-party package at all — not even internal/geom, which every
// other rank-1 leaf (internal/pagemodel) is free to import (R1). The
// zero-imports property is what keeps diag importable by every one of
// AC4's five condition-owning packages (ranks 2 through 7); the first
// first-party import added here would quietly foreclose whichever of
// those five sits at or above this package's own rank.
//
// This scans internal/diag's Go files DIRECTLY, by AST, rather than
// trusting `go list`'s dependency graph — the same "measure by AST"
// discipline this file's sibling arch tests already use — so the
// mutation below (adding an `internal/expr` import) is caught by an
// instrument nobody has yet defeated, independent of lint's
// stage-rank guard living in a different module entirely. AC1's
// mutation requires BOTH `ScanStageRank` (lint module) and this
// assertion to redden; running only one would leave the property
// resting on a single, possibly-broken guard.
func TestDiagPackageHasZeroFirstPartyImports(t *testing.T) {
	internalRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve internal/ root: %v", err)
	}
	diagDir := filepath.Join(internalRoot, "diag")

	entries, err := os.ReadDir(diagDir)
	if err != nil {
		t.Fatalf("read internal/diag: %v", err)
	}

	fset := token.NewFileSet()
	var goFiles int
	var findings []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		goFiles++
		path := filepath.Join(diagDir, entry.Name())
		file, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		for _, imp := range file.Imports {
			value, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				value = imp.Path.Value
			}
			// A first-party import is one rooted at this module's
			// import path — "github.com/panitw/folio8/folio-go" itself
			// (the root `folio8` package) or anything beneath it
			// ("github.com/panitw/folio8/folio-go/internal/...").
			// Everything else (the standard library, a vendored
			// dependency) is fine — this rule concerns the stage-rank
			// graph alone (AD-1), never import hygiene in general.
			if value == "github.com/panitw/folio8/folio-go" ||
				strings.HasPrefix(value, "github.com/panitw/folio8/folio-go/") {
				p := fset.Position(imp.Pos())
				findings = append(findings, p.String()+": internal/diag imports first-party package "+value)
			}
		}
	}

	// Vacuity guard: a run that silently visited zero files would
	// report zero findings for the wrong reason.
	if goFiles == 0 {
		t.Fatalf("vacuity guard: found no .go files under internal/diag — internal/diag/ must exist (AC1)")
	}

	if len(findings) > 0 {
		t.Fatalf("[%s] internal/diag must import zero first-party packages (R1):\n%s", ruleDiagZeroImports, strings.Join(findings, "\n"))
	}
}
