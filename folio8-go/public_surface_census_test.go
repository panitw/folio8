package folio8

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// publicSurfacePins names the frozen public API of folio8-go/v1.0.0: every
// exported identifier of package folio8 and package fonts, by kind and name.
// It pins identifiers only — not signatures, types or constant values, which
// other tests cover. D-1.1.c fixes this surface at the tag, and the tag commits
// to semver, so removing or renaming any line below needs a /v2 import path.
//
// The list is closed in both directions. An addition is not a breaking change,
// but under a frozen API it is still a deliberate act that must be reviewed as
// a whole (RELEASING.md, precondition 2), so it reddens here until this literal
// names it in the same commit.
var publicSurfacePins = []string{
	// package folio8 — functions
	"folio8 func LoadTemplate",
	"folio8 func ParameterReferences",
	"folio8 func ParseTemplate",
	"folio8 func Render",
	"folio8 func RenderTo",
	"folio8 func SerializeTemplate",
	"folio8 func Validate",

	// package folio8 — types
	"folio8 type Data",
	"folio8 type Diagnostic",
	"folio8 type FontSet",
	"folio8 type Params",
	"folio8 type RenderError",
	"folio8 type Result",
	"folio8 type Severity",
	"folio8 type Template",

	// package folio8 — constants
	"folio8 const DiagCodeBarcodeDoesNotFit",
	"folio8 const DiagCodeBarcodeModuleTooSmall",
	"folio8 const DiagCodeBarcodeUnencodable",
	"folio8 const DiagCodeBindingPathAbsent",
	"folio8 const DiagCodeContentUnlayoutable",
	"folio8 const DiagCodeDocumentDateInvalid",
	"folio8 const DiagCodeEmptyAverage",
	"folio8 const DiagCodeExpressionInvalid",
	"folio8 const DiagCodeInternalUnhandledCaveat",
	"folio8 const DiagCodePagesInvalid",
	"folio8 const DiagCodeQRCodeDoesNotFit",
	"folio8 const DiagCodeQRCodeModuleTooSmall",
	"folio8 const DiagCodeQRCodeTooLong",
	"folio8 const DiagCodeSectionBreakInvalid",
	"folio8 const DiagCodeSectionBreakSplitsKeepTogether",
	"folio8 const DiagCodeSectionBreakStraddled",
	"folio8 const DiagCodeTableFooterOrphanSuppressed",
	"folio8 const DiagCodeTableFooterSourceForbidden",
	"folio8 const DiagCodeTableFooterSourceUnresolved",
	"folio8 const DiagCodeTableHeaderRepeatSuppressed",
	"folio8 const DiagCodeTableMinHeightUnplaceable",
	"folio8 const DiagCodeTableRowClippedHeight",
	"folio8 const DiagCodeTemplateFieldInvalid",
	"folio8 const DiagCodeTemplateMalformed",
	"folio8 const DiagCodeTextClippedWidth",
	"folio8 const DiagCodeTextMissingGlyph",
	"folio8 const DiagCodeTextStyleFaceUndeclared",
	"folio8 const LocaleTableVersion",
	"folio8 const MaxParameterReferenceNameLength",
	"folio8 const SeverityError",
	"folio8 const SeverityWarning",
	"folio8 const Version",

	// package folio8 — methods
	"folio8 method RenderError.Error",
	"folio8 method RenderError.Unwrap",
	"folio8 method Severity.String",

	// package folio8 — struct fields
	"folio8 field Diagnostic.Code",
	"folio8 field Diagnostic.DataPath",
	"folio8 field Diagnostic.ElementID",
	"folio8 field Diagnostic.Message",
	"folio8 field Diagnostic.Severity",
	"folio8 field RenderError.Diagnostic",
	"folio8 field RenderError.Err",
	"folio8 field Result.Bytes",
	"folio8 field Result.Diagnostics",

	// package fonts
	"fonts func Shipped",
}

// scanPublicSurface lists every exported identifier declared by the non-test
// sources in dir, qualified as "<pkg> <kind> <name>". Kinds are func, type,
// const, var, method (Type.Method, exported receiver types only) and field
// (Type.Field, including embedded fields and exported interface methods, on
// exported types only). Unlike exportedIdentifiers it never deduplicates
// across kinds, so a field and a type sharing a name stay two entries.
//
// Build constraints are deliberately ignored: a file compiled only for js/wasm
// still ships public API on that target.
func scanPublicSurface(t *testing.T, dir, pkgName string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var out []string
	add := func(kind, name string) { out = append(out, pkgName+" "+kind+" "+name) }
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		if file.Name.Name != pkgName {
			t.Fatalf("%s declares package %s, want %s — the census would be scanning the wrong package", name, file.Name.Name, pkgName)
		}
		for _, d := range file.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				if !decl.Name.IsExported() {
					continue
				}
				if decl.Recv == nil {
					add("func", decl.Name.Name)
					continue
				}
				if recv := surfaceTypeName(decl.Recv.List[0].Type); ast.IsExported(recv) {
					add("method", recv+"."+decl.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						kind := "var"
						if decl.Tok == token.CONST {
							kind = "const"
						}
						for _, id := range s.Names {
							if id.IsExported() {
								add(kind, id.Name)
							}
						}
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						add("type", s.Name.Name)
						var fields *ast.FieldList
						switch tt := s.Type.(type) {
						case *ast.StructType:
							fields = tt.Fields
						case *ast.InterfaceType:
							fields = tt.Methods
						}
						if fields == nil {
							continue
						}
						for _, f := range fields.List {
							if len(f.Names) == 0 {
								if emb := surfaceTypeName(f.Type); ast.IsExported(emb) {
									add("field", s.Name.Name+"."+emb)
								}
								continue
							}
							for _, n := range f.Names {
								if n.IsExported() {
									add("field", s.Name.Name+"."+n.Name)
								}
							}
						}
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// surfaceTypeName strips pointers, qualifiers and type parameters from a
// receiver or embedded-field type expression.
func surfaceTypeName(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.SelectorExpr:
			return e.Sel.Name
		case *ast.Ident:
			return e.Name
		default:
			return ""
		}
	}
}

// TestOnlyRootAndFontsAreImportableLibraryPackages closes the census's other
// door: the census scans package folio8 and package fonts only, so a new
// importable library package anywhere under folio8-go/ (as folio8-go/wasm was
// before client-libraries story 1) would ship public API without reddening it.
// Directories Go itself ignores (a leading "." or "_", testdata) and anything
// under internal/ are skipped; command packages (package main) are allowed.
func TestOnlyRootAndFontsAreImportableLibraryPackages(t *testing.T) {
	root := filepath.Join(repoRootFromTest(t), "folio8-go")
	allowed := map[string]bool{".": true, "fonts": true}
	seen := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "internal") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if perr != nil {
			return perr
		}
		if file.Name.Name == "main" {
			return nil
		}
		rel, rerr := filepath.Rel(root, filepath.Dir(path))
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		if !allowed[rel] {
			t.Errorf("UNEXPECTED importable package folio8-go/%s (package %s, in %s) — it is public API the v1 census does not scan. Move it under internal/, or add it to the census deliberately.", rel, file.Name.Name, name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	for dir := range allowed {
		if !seen[dir] {
			t.Errorf("EXPECTED library package folio8-go/%s is GONE — the walk found no non-main Go source there, so this check is scanning the wrong tree", dir)
		}
	}
}

// TestPublicSurfaceMatchesTheFrozenV1Census is the pinned surface census
// RELEASING.md's precondition 2 relies on: it reddens the commit that adds an
// exported identifier to package folio8 or package fonts (UNEXPECTED) or
// removes one (GONE).
func TestPublicSurfaceMatchesTheFrozenV1Census(t *testing.T) {
	root := filepath.Join(repoRootFromTest(t), "folio8-go")
	var got []string
	for _, pkg := range []struct{ dir, name string }{{".", "folio8"}, {"fonts", "fonts"}} {
		scanned := scanPublicSurface(t, filepath.Join(root, pkg.dir), pkg.name)
		// VACUITY GUARD, per package: a scan of the wrong directory, or one that
		// stopped recognising a declaration form, must never read as a surface
		// that agrees.
		if len(scanned) == 0 {
			t.Fatalf("public surface census found ZERO exported identifiers in package %s (%s). That is not a pass: the scan is broken or pointed at the wrong directory.", pkg.name, filepath.Join(root, pkg.dir))
		}
		got = append(got, scanned...)
	}

	want := slices.Clone(publicSurfacePins)
	sort.Strings(want)
	for i := 1; i < len(want); i++ {
		if want[i] == want[i-1] {
			t.Errorf("publicSurfacePins lists %q twice", want[i])
		}
	}

	for _, g := range got {
		if !slices.Contains(want, g) {
			t.Errorf("UNEXPECTED public identifier %q — folio8-go's v1 API is frozen (D-1.1.c, RELEASING.md). If the addition is deliberate and reviewed, add it to publicSurfacePins in the same commit; otherwise unexport it.", g)
		}
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("EXPECTED public identifier %q is GONE — removing or renaming a v1 identifier is a breaking change that needs a /v2 import path, not an edit to this list.", w)
		}
	}
	t.Logf("public surface census: %d identifiers across package folio8 and package fonts", len(got))
}
