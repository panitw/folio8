//go:build !(js && wasm)

package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestRenderEntryImportsOnlyThePublicAPI holds the render entry to its
// contract: it calls the public folio8 API only, so it imports no
// folio8-go/internal package, and it compiles in no fonts, so it does not
// import folio8-go/fonts.
func TestRenderEntryImportsOnlyThePublicAPI(t *testing.T) {
	const module = "github.com/panitw/folio8/folio8-go"
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if path == module+"/fonts" || strings.HasPrefix(path+"/", module+"/fonts/") {
				t.Errorf("%s imports %s: the render entry must not compile in fonts", name, path)
			}
			if path == module+"/internal" || strings.HasPrefix(path, module+"/internal/") {
				t.Errorf("%s imports %s: the render entry may use only the public folio8 API", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("found no non-test Go files to check")
	}
}
