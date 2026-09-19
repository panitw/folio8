package template

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// modelStructFiles are every file that can declare a struct type this
// package's serializer might write. Kept in sync with the hardcoded
// type list below by TestOmitemptyTypeListIsComplete (Finding 22, this
// story's finisher review, Nit): a new model.go struct, or RawValue /
// Presence — declared in other files and both serialized, which the
// original hardcoded list of 14 silently excluded — is caught the
// moment the AST-derived count and the list's length diverge, rather
// than staying invisible until a human happens to notice.
var modelStructFiles = []string{"model.go", "presence.go", "rawvalue.go"}

// countStructTypeDecls parses each of modelStructFiles (resolved
// relative to this package's own directory via repoRootFromTest) and
// counts top-level `type X struct{...}` declarations, including generic
// ones (Presence[T]).
func countStructTypeDecls(t *testing.T) int {
	t.Helper()
	root := repoRootFromTest(t)
	count := 0
	for _, f := range modelStructFiles {
		path := filepath.Join(root, "folio-go", "internal", "template", f)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := ts.Type.(*ast.StructType); ok {
					count++
				}
			}
		}
	}
	return count
}

// TestOmitemptyTypeListIsComplete binds TestNoOmitemptyStructTags'
// hardcoded type list to the model's own struct count, so an added (or
// removed) struct type in any of modelStructFiles is caught here rather
// than silently escaping reflection.
func TestOmitemptyTypeListIsComplete(t *testing.T) {
	want := countStructTypeDecls(t)
	got := len(omitemptyCheckedTypes())
	if got != want {
		t.Fatalf("omitemptyCheckedTypes() has %d entries, but %v declare %d struct types — update the list in omitempty_test.go", got, modelStructFiles, want)
	}
}

// omitemptyCheckedTypes is every struct type TestNoOmitemptyStructTags
// reflects over — every model.go struct, plus RawValue and one
// instantiation of the generic Presence[T] (its Set/Null/Value fields
// carry the same tags regardless of T, so a single instantiation is
// enough to catch a reintroduced tag).
func omitemptyCheckedTypes() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf(Document{}), reflect.TypeOf(Page{}), reflect.TypeOf(PageSize{}),
		reflect.TypeOf(Margin{}), reflect.TypeOf(Padding{}), reflect.TypeOf(Bands{}),
		reflect.TypeOf(Band{}), reflect.TypeOf(ContentPage{}), reflect.TypeOf(Element{}), reflect.TypeOf(TableExt{}),
		reflect.TypeOf(Column{}), reflect.TypeOf(Style{}), reflect.TypeOf(Border{}),
		reflect.TypeOf(TableRules{}),
		reflect.TypeOf(Asset{}), reflect.TypeOf(Field{}),
		reflect.TypeOf(FontChainEntry{}), reflect.TypeOf(FontRecord{}),
		reflect.TypeOf(RawValue{}), reflect.TypeOf(Presence[string]{}),
	}
}

// TestNoOmitemptyStructTags is AC22's structural half. This package's
// serializer never uses encoding/json struct tags at all — every field
// is written by hand via explicit Presence.Set checks (serialize.go) —
// so there is no `omitempty` anywhere to turn an authored zero value
// into an absent key. This test asserts that invariant by construction:
// it reflects over every exported struct type in this package's model
// (omitemptyCheckedTypes, bound to the model's own struct count by
// TestOmitemptyTypeListIsComplete above) and fails if it ever finds a
// `json:"...,omitempty"` tag, which would mean a future edit had
// reintroduced the encoding/json-tag-driven shape D-1.4.3 forbids. It
// reports how many struct fields it inspected and fails on zero
// (D-000.9 shape).
func TestNoOmitemptyStructTags(t *testing.T) {
	types := omitemptyCheckedTypes()
	inspected := 0
	for _, ty := range types {
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			inspected++
			tag := string(f.Tag)
			if strings.Contains(tag, "omitempty") {
				t.Errorf("%s.%s carries an omitempty struct tag: %q (D-1.4.3 forbids omitempty on any field whose zero value is legal content)", ty.Name(), f.Name, tag)
			}
		}
	}
	if inspected == 0 {
		t.Fatal("coverage witness: zero struct fields inspected")
	}
}

// TestNilContainersSerializeAsEmptyNotNull is AC22/AC23's construction
// half: a Document built with nil (not empty) maps/slices must still
// serialize as "{}"/"[]", never "null" — M-2's fifth trap. This is
// asserted directly at the Go level (constructing a zero-value nested
// struct), since the parser itself always constructs non-nil empty
// containers and so cannot exercise this path on its own.
func TestNilContainersSerializeAsEmptyNotNull(t *testing.T) {
	d := &Document{
		Version:   "1.0",
		Locale:    "en",
		UTCOffset: "+00:00",
		Page: Page{
			Orientation: "portrait",
			SizeIsName:  true,
			SizeName:    "A4",
		},
		Fonts:  nil, // deliberately nil, not Fonts{}
		Assets: nil, // deliberately nil, not map[string]Asset{}
		Bands: Bands{
			Content:    Band{Elements: nil}, // deliberately nil, not []Element{}
			PageFooter: Band{Elements: nil, Height: present(geom.Length(20000))},
			PageHeader: Band{Elements: nil, Height: present(geom.Length(20000))},
		},
		NextID: 1,
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "null") {
		t.Fatalf("a nil map/slice must never serialize as null (M-2's fifth trap, AC23):\n%s", s)
	}
	if !strings.Contains(s, `"fonts": {}`) {
		t.Fatalf("nil Fonts must serialize as {}:\n%s", s)
	}
	if !strings.Contains(s, `"assets": {}`) {
		t.Fatalf("nil Assets must serialize as {}:\n%s", s)
	}
	if !strings.Contains(s, `"elements": []`) {
		t.Fatalf("nil Elements must serialize as []:\n%s", s)
	}
}
