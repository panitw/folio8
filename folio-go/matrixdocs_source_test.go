package folio8

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// MatrixDocumentSlugsFromSource is THE reader of matrix_test.go's
// matrixDocuments list. Every untagged guard that needs to know which
// documents are registered calls this and nothing else.
//
// Why it exists (Story 2.5 review, Finding 1 — a Major):
//
// Two guards claimed to close the "a matrix leg nobody compares,
// reported as green" drift from opposite sides —
// TestEpic2GateObligationsMatchTheDeclaredSet (is it a declared gate
// obligation?) and TestMatrixDocumentSlugsAreRegisteredInCI (is it in
// matrix.yml?). Both read matrix_test.go as source text, because that
// file carries //go:build matrix and is therefore not compiled into the
// ordinary test binary. And both used the SAME regex:
//
//	(?m)^\s*slug:\s*"([^"]+)"
//
// which requires `slug:` to begin a line. So they were never two
// independent guards. They were ONE guard wearing two names, and one
// parsing assumption defeated both at once. Measured, not theorised: a
// seventh entry written as a single-line composite literal —
//
//	{label: "evader", slug: "zz-evader", capture: …, requireFontFile2: true},
//
// is gofmt-clean (`gofmt -l` silent), compiles under `-tags matrix`
// (`go vet -tags matrix` clean), and BOTH guards report PASS with the
// document registered in neither matrix.yml nor
// declaredEpic2GateObligations. Formatting alone reopened the drift
// Story 2.3 closed.
//
// Fixing the regex would have left that structure intact — the next
// spelling nobody thought of defeats the next regex, and still defeats
// both guards at once, because there is still only one of them. So the
// parsing assumption is removed rather than widened: the list is read
// through go/parser, against the Go grammar itself, where a composite
// literal has no second spelling. Line breaks, alignment, field order
// and gofmt all become invisible to the reader, because they are
// invisible to the language.
//
// This is the discipline stagerank.go already states in its own words —
// "never a regex, never the literal text" — applied to the one place in
// the module that had not adopted it.
//
// D-000.36 (a remedy for a vacuity is itself subject to vacuity) is why
// an unreadable entry is an ERROR and never a skip. A walker that
// silently ignores a shape it does not understand shrinks the observed
// set, and a shrunken set is exactly the failure being fixed: the
// evader would simply have moved from "not matched by the regex" to
// "not understood by the walker", with both guards green again. Every
// element of the literal must yield a slug, or the read fails and names
// the element that defeated it. The element count is returned alongside
// the slugs so each caller can assert its own N-of-N witness rather
// than trusting this function's word for it.
//
// It is declared in package folio8 (not folio8_test) because
// matrix_registration_test.go is an internal test file; it is exported
// so byte_neutrality_test.go, which is package folio8_test, reaches the
// same implementation. It is test-only source, so nothing is added to
// the module's public API.
func MatrixDocumentSlugsFromSource(path string) (slugs []string, elements int, err error) {
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if perr != nil {
		return nil, 0, fmt.Errorf("parse %s: %w", path, perr)
	}

	var lit *ast.CompositeLit
	declsFound := 0
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if name.Name != "matrixDocuments" {
					continue
				}
				declsFound++
				if i >= len(vs.Values) {
					return nil, 0, fmt.Errorf(
						"%s: matrixDocuments is declared without a value at %s — the registered document set cannot be read",
						path, fset.Position(name.Pos()))
				}
				cl, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok {
					return nil, 0, fmt.Errorf(
						"%s: matrixDocuments at %s is not a composite literal (%T) — this reader understands only `var matrixDocuments = []matrixDocument{…}`. Restore that shape, or teach this reader the new one; do NOT let it read fewer documents than are registered",
						path, fset.Position(vs.Values[i].Pos()), vs.Values[i])
				}
				lit = cl
			}
		}
	}
	if declsFound != 1 {
		return nil, 0, fmt.Errorf(
			"%s: found %d declarations of matrixDocuments, want exactly 1 — with none the document set reads as empty, and with more than one it reads as whichever came last",
			path, declsFound)
	}

	elements = len(lit.Elts)
	if elements == 0 {
		return nil, 0, fmt.Errorf("%s: matrixDocuments is empty — every document-registration comparison built on it would pass vacuously", path)
	}

	seen := make(map[string]int, elements)
	slugs = make([]string, 0, elements)
	for i, el := range lit.Elts {
		pos := fset.Position(el.Pos())
		entry, ok := el.(*ast.CompositeLit)
		if !ok {
			return nil, 0, fmt.Errorf(
				"%s: matrixDocuments[%d] at %s is a %T, not a composite literal — this reader cannot extract its slug, and a document it cannot read must FAIL rather than vanish from the set (D-000.36)",
				path, i, pos, el)
		}
		slug := ""
		for _, f := range entry.Elts {
			kv, ok := f.(*ast.KeyValueExpr)
			if !ok {
				// A positional literal (`{"label", "slug", …}`) carries
				// no field names, so no field can be identified by name.
				// It is rejected below by the empty slug, deliberately:
				// it is another shape in which an entry could otherwise
				// have gone unread.
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "slug" {
				continue
			}
			bl, ok := kv.Value.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return nil, 0, fmt.Errorf(
					"%s: matrixDocuments[%d]'s slug at %s is not a string literal — a slug computed at run time cannot be compared against .github/workflows/matrix.yml, which is text",
					path, i, fset.Position(kv.Value.Pos()))
			}
			unquoted, uerr := strconv.Unquote(bl.Value)
			if uerr != nil {
				return nil, 0, fmt.Errorf("%s: matrixDocuments[%d]'s slug at %s: %w", path, i, fset.Position(bl.Pos()), uerr)
			}
			slug = unquoted
		}
		if slug == "" {
			return nil, 0, fmt.Errorf(
				"%s: matrixDocuments[%d] at %s declares no non-empty `slug:` field — every registered document must be nameable, because its slug is the key it is registered under in .github/workflows/matrix.yml and in declaredEpic2GateObligations",
				path, i, pos)
		}
		if prev, dup := seen[slug]; dup {
			return nil, 0, fmt.Errorf(
				"%s: matrixDocuments[%d] at %s repeats slug %q, already used by entry %d — two entries under one slug collapse into one in every set comparison, so the second is a leg nobody compares",
				path, i, pos, slug, prev)
		}
		seen[slug] = i
		slugs = append(slugs, slug)
	}

	if len(slugs) != elements {
		return nil, 0, fmt.Errorf(
			"%s: read %d slugs from %d matrixDocuments entries — they must be equal, or a registered document is missing from the observed set",
			path, len(slugs), elements)
	}
	return slugs, elements, nil
}

// TestMatrixDocumentSlugsFromSourceReadsShapesTheRegexMissed holds, as a
// test, the red-proofs that were run by hand against this reader when it
// replaced the regex.
//
// Under D-000.36 this is the point: a remedy for a vacuity is itself
// subject to vacuity, so the remedy's own failure modes must be held by
// something that runs every story — not by a reviewer's transcript. Each
// case below is a shape that COMPILES under -tags matrix and is
// gofmt-clean, so none of them is caught by the toolchain; the first is
// the exact shape that was measured green against both guards before
// this reader existed.
//
// The synthetic sources are written to t.TempDir() rather than to
// testdata/, because a testdata/ Go file naming `slug:` would be read by
// the very guards this reader feeds.
func TestMatrixDocumentSlugsFromSourceReadsShapesTheRegexMissed(t *testing.T) {
	const preamble = "package folio8\n\ntype matrixDocument struct {\n\tlabel string\n\tslug  string\n}\n\n"

	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "matrix_test_source.go")
		if err := os.WriteFile(path, []byte(preamble+body), 0o600); err != nil {
			t.Fatalf("write synthetic source: %v", err)
		}
		return path
	}

	t.Run("the single-line literal that defeated the regex is read", func(t *testing.T) {
		// Verbatim the shape from the Story 2.5 review's Finding 1.
		path := write(t, `var matrixDocuments = []matrixDocument{
	{
		label: "kept",
		slug:  "kept-slug",
	},
	{label: "evader", slug: "zz-evader"},
}
`)
		slugs, elements, err := MatrixDocumentSlugsFromSource(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if elements != 2 || len(slugs) != 2 {
			t.Fatalf("read %d slugs from %d entries, want 2 from 2", len(slugs), elements)
		}
		// Anti-vacuity: the multi-line entry alone would satisfy a count
		// of 1, so the single-line one is asserted BY NAME. This is the
		// assertion the regex could not have made.
		if slugs[0] != "kept-slug" || slugs[1] != "zz-evader" {
			t.Fatalf("got %v, want [kept-slug zz-evader] — the single-line entry must be read, in order", slugs)
		}
	})

	t.Run("field order and line breaks are invisible", func(t *testing.T) {
		path := write(t, `var matrixDocuments = []matrixDocument{
	{slug: "first", label: "reversed field order"}, {
		slug: "second",
	},
}
`)
		slugs, elements, err := MatrixDocumentSlugsFromSource(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if elements != 2 || len(slugs) != 2 || slugs[0] != "first" || slugs[1] != "second" {
			t.Fatalf("got %v from %d entries, want [first second] from 2", slugs, elements)
		}
	})

	// Every case below must FAIL rather than return a shorter list. A
	// reader that skips what it cannot understand reproduces the exact
	// defect it was written to fix, one spelling further along.
	unreadable := []struct {
		name     string
		body     string
		wantHint string
	}{
		{
			name: "an entry with no slug field",
			body: `var matrixDocuments = []matrixDocument{
	{slug: "kept-slug"},
	{label: "no slug here"},
}
`,
			wantHint: "declares no non-empty `slug:` field",
		},
		{
			name: "an entry whose slug is empty",
			body: `var matrixDocuments = []matrixDocument{
	{slug: "kept-slug"},
	{label: "blank", slug: ""},
}
`,
			wantHint: "declares no non-empty `slug:` field",
		},
		{
			name: "a slug computed at run time",
			body: `var matrixDocuments = []matrixDocument{
	{slug: "kept-slug"},
	{label: "computed", slug: "zz-" + "evader"},
}
`,
			wantHint: "is not a string literal",
		},
		{
			name: "two entries sharing one slug",
			body: `var matrixDocuments = []matrixDocument{
	{slug: "kept-slug"},
	{label: "dup", slug: "kept-slug"},
}
`,
			wantHint: "repeats slug",
		},
		{
			name: "an element that is an identifier, not a literal",
			body: `var elsewhere = matrixDocument{label: "sneaky", slug: "zz-sneaky"}

var matrixDocuments = []matrixDocument{
	{slug: "kept-slug"},
	elsewhere,
}
`,
			wantHint: "not a composite literal",
		},
		{
			name: "the list is empty",
			body: `var matrixDocuments = []matrixDocument{}
`,
			wantHint: "is empty",
		},
		{
			name: "the list is not declared at all",
			body: `var somethingElse = []matrixDocument{{slug: "kept-slug"}}
`,
			wantHint: "found 0 declarations of matrixDocuments",
		},
		{
			// This one does not COMPILE (Go forbids the redeclaration),
			// but it PARSES, and parsing is all this reader does. The
			// assertion is that it refuses rather than quietly reporting
			// whichever declaration came last.
			name: "the list is declared twice",
			body: `var matrixDocuments = []matrixDocument{{slug: "a"}}

var matrixDocuments = []matrixDocument{{slug: "b"}}
`,
			wantHint: "found 2 declarations of matrixDocuments",
		},
	}
	for _, tc := range unreadable {
		t.Run(tc.name+" is an error, not a shorter list", func(t *testing.T) {
			path := write(t, tc.body)
			slugs, elements, err := MatrixDocumentSlugsFromSource(path)
			if err == nil {
				t.Fatalf("read %d slugs from %d entries with NO error (%v) — an unreadable entry that shrinks the set is the defect this reader exists to prevent",
					len(slugs), elements, slugs)
			}
			if !strings.Contains(err.Error(), tc.wantHint) {
				t.Fatalf("error %q does not name %q — the message is the remedy a human executes (D-000.37)", err, tc.wantHint)
			}
			if slugs != nil || elements != 0 {
				t.Fatalf("on error the reader returned %d slugs / %d elements, want nil/0 — a partial read invites a partial comparison", len(slugs), elements)
			}
		})
	}

	t.Run("the real matrix_test.go is read, and every entry yields a slug", func(t *testing.T) {
		// The presence precondition (D-000.9) for both production
		// callers: this reader is only worth having if it can read the
		// file it was written for.
		root := repoRootFromTest(t)
		slugs, elements, err := MatrixDocumentSlugsFromSource(filepath.Join(root, "folio-go", "matrix_test.go"))
		if err != nil {
			t.Fatalf("read the real matrix_test.go: %v", err)
		}
		if elements == 0 || len(slugs) != elements {
			t.Fatalf("read %d slugs from %d entries in the real matrix_test.go — they must be equal and non-zero", len(slugs), elements)
		}
		t.Logf("reader witness — %d slugs read from %d matrixDocuments entries in the shipped matrix_test.go: %v", len(slugs), elements, slugs)
	})
}
