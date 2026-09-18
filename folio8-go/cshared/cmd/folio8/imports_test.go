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

// TestCSharedEntryImportsOnlyThePublicAPI holds the c-shared entry to the same
// contract folio8-go/wasm/cmd/render is held to: it calls the public folio8
// API only, so it imports no folio8-go/internal package, and it compiles in no
// fonts, so it does not import folio8-go/fonts. A caller supplies every face,
// on every call, exactly as in Go.
func TestCSharedEntryImportsOnlyThePublicAPI(t *testing.T) {
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
				t.Errorf("%s imports %s: the c-shared entry must not compile in fonts", name, path)
			}
			if path == module+"/internal" || strings.HasPrefix(path, module+"/internal/") {
				t.Errorf("%s imports %s: the c-shared entry may use only the public folio8 API", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("found no non-test Go files to check")
	}
}

// TestCSharedEntryBuildsWithoutCgo pins the CGO_ENABLED=0 stub's existence.
// matrix.yml builds every leg with cgo off; without a !cgo file here the
// toolchain fails the whole directory rather than skipping it.
func TestCSharedEntryBuildsWithoutCgo(t *testing.T) {
	src, err := os.ReadFile("nocgo.go")
	if err != nil {
		t.Fatalf("read nocgo.go: %v (it is what keeps CGO_ENABLED=0 builds green)", err)
	}
	if !strings.Contains(string(src), "//go:build !cgo") {
		t.Error("nocgo.go no longer carries the //go:build !cgo constraint")
	}
	cgoSrc, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cgoSrc), "//go:build cgo") {
		t.Error("main.go no longer carries the //go:build cgo constraint, so the two files would collide")
	}
}
