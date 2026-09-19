package folio8_test

// TestCLIBinaryIsNamedCmdFolio8 is Story 3.7's AC3 (D-1's own obligation,
// divergence D-1 confirmed by the lead): the binary MUST be named
// `cmd/folio8` — it is named in AD-7's own `Binds` line, in the
// architecture spine's source tree, and in NFR1.f, so a rename to
// (say) `cmd/folio8cli` would make three canonical documents name a
// nonexistent artifact, with nothing else catching it — every other AC
// in this story would still pass. This test is the thing that catches
// it: a literal path this test owns, checked against the real
// filesystem, plus the compiler's own package/func declaration.
import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIBinaryIsNamedCmdFolio8(t *testing.T) {
	const wantRelPath = "cmd/folio8/main.go"

	path := filepath.FromSlash(wantRelPath)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("AC3: %s does not exist (the CLI binary must be named cmd/folio8 — AD-7's Binds line, the spine's source tree, and NFR1.f all name this exact path): %v", wantRelPath, err)
	}
	if info.IsDir() {
		t.Fatalf("AC3: %s is a directory, want a file", wantRelPath)
	}

	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly|parser.SkipObjectResolution)
	if perr != nil {
		t.Fatalf("parse %s: %v", path, perr)
	}
	if file.Name.Name != "main" {
		t.Fatalf("AC3: %s declares package %q, want \"main\"", wantRelPath, file.Name.Name)
	}

	full, ferr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if ferr != nil {
		t.Fatalf("parse %s: %v", path, ferr)
	}
	hasMain := false
	for _, decl := range full.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "main" {
			hasMain = true
		}
	}
	if !hasMain {
		t.Fatalf("AC3: %s declares no func main", wantRelPath)
	}
}
