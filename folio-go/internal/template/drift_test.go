package template

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This file is the drift test (D-1.4.7, AC45–AC47): internal/template's
// parser and serializer are normative; folio-format.md is documentation.
// A readability contract that lies is worse than none, so drift is
// caught mechanically in both directions.
//
// Neither side is a checked-in list of key names (D-1.4.7's forbidden
// fallback): the Go side is extracted by parsing serialize.go's own AST
// for the string keys passed to writeObject via kv{...} literals — this
// package does not use encoding/json struct tags at all (its serializer
// is hand-written, jsonstring.go/serialize.go), so struct-tag reflection
// (D-1.4.7's originally-envisioned mechanism) does not apply here; AST
// extraction over the actual key literals the serializer emits is
// necessary but was found NOT sufficient on its own (D-1.4.15, this
// story's finisher review, Finding 1): it is exact only for keys that
// pass through the specific kv{"lit", ...} / elided-two-element shapes
// extractGoKeys recognises — a keyed composite literal or a key hoisted
// to a named constant is invisible to it, silently. TestDriftASTMatchesRuntimeEmission
// below adds the required second, independently-derived extraction
// (parse what the serializer ACTUALLY emits for a maximal document) and
// asserts the two sets are equal, both directions — see that test's own
// comment for the full rationale.
//
// Scope boundary (AC10, AC47): both this file's comparisons are about
// KNOWN keys — the ones the Go model and folio-format.md's field table
// both claim to define. Unknown passthrough keys (AC8, AC9: an
// unrecognised key carried opaquely through parse and merged back on
// serialize) are explicitly OUTSIDE this file's scope in every
// direction — AST, doc-extraction and the runtime walk alike. A
// passthrough key appearing in a document's serialized bytes is not
// drift and must never be read as a doc→Go or Go→doc mismatch.

// goKeyRegistrySource is the file whose kv{...} literals name every key
// this package's serializer can emit.
const goKeyRegistrySource = "serialize.go"

// extractGoKeys parses path (a Go source file) and collects the first
// (key) argument of every unkeyed `kv{...}` composite literal whose key
// is a string literal. Dynamically-built keys (e.g. writeFonts' map
// key, writeAssets' hash key) are NOT literals in source and so are
// correctly excluded automatically — they are user-chosen data, not
// schema keys, exactly the fonts/assets exclusion AC46 asks for, but
// arrived at structurally rather than by name.
func extractGoKeys(path string) (keys map[string]bool, tagsReflected int, err error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, 0, err
	}
	keys = map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// A kv literal appears two ways in this file: explicitly typed
		// (`kv{"width", ...}`, used for conditionally-appended optional
		// fields) and, far more often, as an ELIDED-type element inside
		// a `[]kv{ {"version", ...}, {"locale", ...} }` slice literal
		// (Go elides the repeated `kv` on each element — those inner
		// composite literals have a nil Type, recognised here instead
		// by shape: exactly two unkeyed elements, the first a string
		// literal).
		isExplicitKV := false
		if ident, ok := cl.Type.(*ast.Ident); ok && ident.Name == "kv" {
			isExplicitKV = true
		}
		isElidedKVShape := cl.Type == nil && len(cl.Elts) == 2
		if !isExplicitKV && !isElidedKVShape {
			return true
		}
		if len(cl.Elts) == 0 {
			return true
		}
		lit, ok := cl.Elts[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		tagsReflected++
		key := strings.Trim(lit.Value, `"`)
		keys[key] = true
		return true
	})
	return keys, tagsReflected, nil
}

var (
	fenceDelim       = regexp.MustCompile("^```")
	fenceKeyAnywhere = regexp.MustCompile(`"([A-Za-z][A-Za-z0-9_]*)"\s*:`)
	// fenceKeyOpenObjAny matches ANY quoted key opening an object,
	// including a non-identifier one (e.g. assets' SHA-256 hash keys,
	// "9f2b…c41d") — needed to keep the parent-nesting stack accurate
	// even where the key itself is arbitrary user data and so must NOT
	// be registered as a schema key (identifierKey below decides that
	// separately).
	fenceKeyOpenObjAny = regexp.MustCompile(`^\s*"([^"]*)"\s*:\s*\{\s*$`)
	identifierKey      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	tableBacktick      = regexp.MustCompile("`([A-Za-z][A-Za-z0-9_.\\[\\]]*)`")
	// proseAddsKey matches this document's own consistent phrasing for
	// a type-specific extra field not otherwise in a table row (e.g.
	// "adds `\"value\"`, the string" / "adds `\"asset\"`, a key of…") —
	// tied to the doc's actual wording, not a name-based key list.
	proseAddsKey   = regexp.MustCompile("adds\\s+`\"([A-Za-z][A-Za-z0-9_]*)\"`")
	closeBraceLine = regexp.MustCompile(`^\s*\}`)
)

// extractDocKeys implements AC46's measured union corpus, extended with
// a third structural source (prose backtick-quoted key mentions, e.g.
// the doc's `"asset"` — distinguishable from a bare value example like
// `left` by the embedded quote marks, so it never picks up a
// closed-set VALUE): field-table first cells, code-fence object keys,
// and prose-quoted-key mentions. The fonts/assets exclusion (M-6's
// "body" leftover) is applied structurally, by tracking a small parent
// stack of object-opening keys within each fence, never by name.
func extractDocKeys(doc []byte) (keys map[string]bool, tokensExtracted int) {
	keys = map[string]bool{}
	lines := strings.Split(string(doc), "\n")

	inFence := false
	var parentStack []string // keys whose object is currently open, within the current fence

	for _, line := range lines {
		if fenceDelim.MatchString(strings.TrimSpace(line)) {
			inFence = !inFence
			if !inFence {
				parentStack = nil
			}
			continue
		}

		if inFence {
			if m := fenceKeyOpenObjAny.FindStringSubmatch(line); m != nil {
				tokensExtracted++
				parent := ""
				if len(parentStack) > 0 {
					parent = parentStack[len(parentStack)-1]
				}
				if parent != "fonts" && parent != "assets" && identifierKey.MatchString(m[1]) {
					keys[m[1]] = true
				}
				parentStack = append(parentStack, m[1])
				continue
			}
			// Any other key: value pair on this line (object or scalar
			// value, not itself opening a nested object on this line).
			for _, m := range fenceKeyAnywhere.FindAllStringSubmatch(line, -1) {
				tokensExtracted++
				parent := ""
				if len(parentStack) > 0 {
					parent = parentStack[len(parentStack)-1]
				}
				if parent != "fonts" && parent != "assets" {
					keys[m[1]] = true
				}
			}
			if closeBraceLine.MatchString(line) && len(parentStack) > 0 {
				parentStack = parentStack[:len(parentStack)-1]
			}
			continue
		}

		// Markdown table rows: "| `key1`, `key2` | description |"
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			if isTableSeparatorLine(line) {
				continue
			}
			cells := strings.SplitN(line, "|", 3)
			if len(cells) < 2 {
				continue
			}
			firstCell := cells[1]
			for _, m := range tableBacktick.FindAllStringSubmatch(firstCell, -1) {
				tokensExtracted++
				keys[normaliseDocToken(m[1])] = true
			}
		}

		// Prose "adds `"key"`" mentions (AC45's `"value"` / `"asset"`
		// style prose documentation of a type-specific extra field) —
		// tied to the document's own consistent wording, not a
		// name-based key list.
		for _, m := range proseAddsKey.FindAllStringSubmatch(line, -1) {
			tokensExtracted++
			keys[m[1]] = true
		}
	}
	return keys, tokensExtracted
}

func isTableSeparatorLine(line string) bool {
	s := strings.TrimSpace(line)
	for _, c := range s {
		switch c {
		case '|', '-', ':', ' ':
		default:
			return false
		}
	}
	return true
}

// normaliseDocToken reduces a dotted/bracketed table-first-cell token
// (e.g. "columns[].footerOf", "page.margin.top") to the leaf JSON key
// name it actually names on the Go side.
func normaliseDocToken(tok string) string {
	tok = strings.ReplaceAll(tok, "[]", "")
	if i := strings.LastIndex(tok, "."); i >= 0 {
		tok = tok[i+1:]
	}
	return tok
}

// TestDriftGoToDoc is AC45/AC47's Go→doc half: every JSON key the
// serializer can emit must appear as a backticked token somewhere in
// folio-format.md. Reports both sides' witnesses and fails on zero
// (AC47: "the extractors themselves need witnesses, not just the
// comparison").
func TestDriftGoToDoc(t *testing.T) {
	root := repoRootFromTest(t)
	goKeys, tagsReflected, err := extractGoKeys(filepath.Join(root, "folio-go", "internal", "template", goKeyRegistrySource))
	if err != nil {
		t.Fatalf("extract Go keys: %v", err)
	}
	if tagsReflected == 0 {
		t.Fatal("coverage witness: zero struct-key literals reflected from the Go source")
	}

	docBytes := mustReadFile(t, filepath.Join(root, "_bmad-output", "specs", "spec-folio", "folio-format.md"))
	docKeys, tokensExtracted := extractDocKeys(docBytes)
	if tokensExtracted == 0 {
		t.Fatal("coverage witness: zero tokens extracted from folio-format.md")
	}

	compared := 0
	var missing []string
	for k := range goKeys {
		compared++
		if !docKeys[k] {
			missing = append(missing, k)
		}
	}
	if compared == 0 {
		t.Fatal("coverage witness: zero keys compared")
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("keys the serializer can emit but folio-format.md never documents: %v", missing)
	}
}

// TestDriftDocToGo is AC45/AC47's doc→Go half: every backticked key
// token folio-format.md documents corresponds to a real key the Go
// serializer can emit — a readability contract that names a key the
// engine does not implement is worse than none.
func TestDriftDocToGo(t *testing.T) {
	root := repoRootFromTest(t)
	goKeys, tagsReflected, err := extractGoKeys(filepath.Join(root, "folio-go", "internal", "template", goKeyRegistrySource))
	if err != nil {
		t.Fatalf("extract Go keys: %v", err)
	}
	if tagsReflected == 0 {
		t.Fatal("coverage witness: zero struct-key literals reflected from the Go source")
	}

	docBytes := mustReadFile(t, filepath.Join(root, "_bmad-output", "specs", "spec-folio", "folio-format.md"))
	docKeys, tokensExtracted := extractDocKeys(docBytes)
	if tokensExtracted == 0 {
		t.Fatal("coverage witness: zero tokens extracted from folio-format.md")
	}

	compared := 0
	var extra []string
	for k := range docKeys {
		compared++
		if !goKeys[k] {
			extra = append(extra, k)
		}
	}
	if compared == 0 {
		t.Fatal("coverage witness: zero keys compared")
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Fatalf("keys folio-format.md documents but the serializer never emits: %v", extra)
	}
}

// TestDriftCatchesARemovedDocKey is AC47's red-proof, by construction,
// against a RETAINED FIXTURE DOCUMENT — never by mutating
// folio-format.md itself. driftFixtureMissingWidth simulates the doc
// losing its documentation of a real Go key ("width"); the Go→doc
// comparison must flag it.
func TestDriftCatchesARemovedDocKey(t *testing.T) {
	root := repoRootFromTest(t)
	goKeys, _, err := extractGoKeys(filepath.Join(root, "folio-go", "internal", "template", goKeyRegistrySource))
	if err != nil {
		t.Fatalf("extract Go keys: %v", err)
	}
	docKeys, tokensExtracted := extractDocKeys(driftFixtureMissingWidth)
	if tokensExtracted == 0 {
		t.Fatal("fixture extraction produced zero tokens — fixture is broken")
	}
	if docKeys["width"] {
		t.Fatal("fixture must NOT document \"width\" — it exists specifically to red-prove the comparison")
	}
	missing := false
	for k := range goKeys {
		if k == "width" && !docKeys[k] {
			missing = true
		}
	}
	if !missing {
		t.Fatal("expected the fixture to be missing a real Go key (\"width\"), red-proof did not fire")
	}
}

// driftFixtureMissingWidth is a retained miniature fixture document —
// same table shape as folio-format.md, deliberately never documenting
// "width" — used only by TestDriftCatchesARemovedDocKey.
var driftFixtureMissingWidth = []byte("| Field | Meaning |\n|---|---|\n| `id` | opaque id |\n| `type` | element type |\n| `x`, `y` | position |\n")

// --- Behavioural extraction (D-1.4.15, this story's finisher review,
// Finding 1) ---
//
// AST extraction alone is exact only "iff every emitted key passes
// through the kv{...} convention" (D-1.4.15). A keyed composite literal
// (kv{key: "x", write: ...}), a key hoisted to a named constant, or any
// other route to an output byte is invisible to extractGoKeys — proven
// by construction in TestExtractGoKeysMissesAKeyedLiteral below. The
// required fix is independent: serialize a document exercising every
// known field (maximalFixture, fixtures_test.go), parse the OUTPUT
// BYTES, and collect every key actually present at runtime. Two
// independently-derived producers of the same set — the AST and the
// runtime walk — catch each other's failure mode: a key emitted outside
// the kv{} convention appears in runtime and not AST; a fixture that
// fails to exercise a field appears in AST and not runtime. Neither
// alone is exact; together they are (D-1.4.15).
//
// This is explicitly scoped to KNOWN keys the serializer can emit from
// its own model — unknown passthrough keys (AC8, AC9) are, by
// construction, whatever a document happens to carry, and are outside
// both this test's and the AST extractor's scope (AC10, AC47; see also
// this file's header comment above).

// extractRuntimeKeys parses canonical JSON bytes (as produced by
// SerializeDocument) and collects every SCHEMA object key present
// anywhere in the document, recursively — the "actually emitted" half
// of D-1.4.15's two independent extractions. Keys nested directly under
// "fonts" or "assets" are user-chosen data (a fallback-chain name, a
// SHA-256 hash), not schema keys — excluded by the same category rule
// AC46 states for the doc-side extractor (extractDocKeys above), never
// by name.
func extractRuntimeKeys(t *testing.T, b []byte) (keys map[string]bool, tokensExtracted int) {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("extractRuntimeKeys: %v", err)
	}
	keys = map[string]bool{}
	var walk func(x interface{}, parentKey string)
	walk = func(x interface{}, parentKey string) {
		switch y := x.(type) {
		case map[string]interface{}:
			for k, v := range y {
				if parentKey != "fonts" && parentKey != "assets" {
					keys[k] = true
					tokensExtracted++
				}
				walk(v, k)
			}
		case []interface{}:
			for _, e := range y {
				walk(e, parentKey)
			}
		}
	}
	walk(v, "")
	return keys, tokensExtracted
}

// TestDriftASTMatchesRuntimeEmission is D-1.4.15's required behavioural
// guard: astKeys == runtimeKeys, both directions, over maximalFixture —
// the document built specifically to exercise every one of the 56 keys
// extractGoKeys finds in serialize.go (Finding 8: the round-trip corpus
// did not used to cover all of them; maximalFixture closes that gap and
// is reused here as D-1.4.15's own "cheap path first" measurement
// recommends — "one artifact closes both").
func TestDriftASTMatchesRuntimeEmission(t *testing.T) {
	root := repoRootFromTest(t)
	astKeys, tagsReflected, err := extractGoKeys(filepath.Join(root, "folio-go", "internal", "template", goKeyRegistrySource))
	if err != nil {
		t.Fatalf("extract Go keys: %v", err)
	}
	if tagsReflected == 0 {
		t.Fatal("coverage witness: zero struct-key literals reflected from the Go source")
	}

	d, err := ParseDocument(maximalFixture)
	if err != nil {
		t.Fatalf("parse maximalFixture: %v", err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize maximalFixture: %v", err)
	}
	runtimeKeys, tokensExtracted := extractRuntimeKeys(t, out)
	// The mutually exclusive proportional column representation exercises its
	// own fixture; the maximal point fixture continues covering width emission.
	proportional, err := ParseDocument(proportionalFixture())
	if err != nil {
		t.Fatal(err)
	}
	proportionalBytes, err := SerializeDocument(proportional)
	if err != nil {
		t.Fatal(err)
	}
	proportionalKeys, _ := extractRuntimeKeys(t, proportionalBytes)
	for key := range proportionalKeys {
		runtimeKeys[key] = true
	}
	// SPEC-multi-pages: `pages` and `pageBreak` exist only in the multi-page
	// shape, which maximalFixture (a one-page document) cannot carry.
	multiPage, err := ParseDocument(multiPageFixture)
	if err != nil {
		t.Fatal(err)
	}
	multiPageBytes, err := SerializeDocument(multiPage)
	if err != nil {
		t.Fatal(err)
	}
	multiPageKeys, _ := extractRuntimeKeys(t, multiPageBytes)
	for key := range multiPageKeys {
		runtimeKeys[key] = true
	}
	if tokensExtracted == 0 {
		t.Fatal("coverage witness: zero keys extracted from the runtime emission")
	}

	compared := 0
	var astOnly, runtimeOnly []string
	for k := range astKeys {
		compared++
		if !runtimeKeys[k] {
			astOnly = append(astOnly, k)
		}
	}
	for k := range runtimeKeys {
		compared++
		if !astKeys[k] {
			runtimeOnly = append(runtimeOnly, k)
		}
	}
	if compared == 0 {
		t.Fatal("coverage witness: zero keys compared")
	}
	sort.Strings(astOnly)
	sort.Strings(runtimeOnly)
	if len(astOnly) > 0 {
		t.Errorf("keys the AST extractor found but maximalFixture never actually emits (fixture gap — extend maximalFixture): %v", astOnly)
	}
	if len(runtimeOnly) > 0 {
		t.Errorf("keys actually emitted but invisible to AST extraction (a route to an output byte outside the kv{} convention — D-1.4.15's blocker): %v", runtimeOnly)
	}
}

// TestExtractGoKeysMissesAKeyedLiteral is Finding 1's red-proof, by
// construction, against a RETAINED FIXTURE SOURCE FILE (never against
// serialize.go itself): a keyed kv{key: "...", write: ...} composite
// literal — syntactically legal Go, and exactly the shape a future
// maintainer reaches for — is invisible to extractGoKeys, proving the
// AST extractor alone is not exact and the behavioural guard above is
// load-bearing, not redundant.
func TestExtractGoKeysMissesAKeyedLiteral(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "serialize.go")
	src := `package template

type kv struct {
	key   string
	write func(dst []byte, depth int) []byte
}

func writeProbe() []kv {
	return []kv{
		kv{key: "zzKeyedLiteralNotSeenByAST", write: nil},
	}
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture source: %v", err)
	}
	keys, _, err := extractGoKeys(path)
	if err != nil {
		t.Fatalf("extract Go keys: %v", err)
	}
	if keys["zzKeyedLiteralNotSeenByAST"] {
		t.Fatal("expected the keyed composite literal to be invisible to AST extraction — red-proof did not fire")
	}
}
