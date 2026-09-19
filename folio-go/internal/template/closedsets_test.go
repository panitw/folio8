package template

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// This file is AC11's mandated second half (D-1.8.1, D-1.4.12; Finding
// 5, Story 1.8 review): "mediaType must not be added to
// internal/template/closedsets.go, AND A TEST ASSERTS IT IS ABSENT from
// that file's set inventory." Before this file, the first half held —
// grepping closedsets.go for "mediaType" found nothing — but nothing in
// the tree enforced it or would even notice a future agent adding it:
// the property held because nobody had added it, the exact "unexercised,
// not enforced" pattern AC17b names for AD-24. Under D-1.4.12, adding
// mediaType to this file's closed-set inventory is a permanent MAJOR
// version bump on the format — the one-way door this test exists to
// guard.
//
// Enumerated by AST, same technique as drift_test.go's extractGoKeys:
// no checked-in list of set names to fall out of date.

// closedSetsSource is the file whose package-level `closed*` map
// declarations are this package's closed-set inventory.
const closedSetsSource = "closedsets.go"

// scanClosedSets parses path and returns the name of every package-level
// var whose declared name starts with "closed" (this package's
// closed-set naming convention — closedLocales, closedElementTypes, and
// so on), together with every string literal found inside each such
// var's initializer (so a mediaType set added under an EXISTING, more
// generically-named var would still be caught by its contents, not just
// by a new var's name).
//
// IT FOLLOWS A SET DECLARED THROUGH ANOTHER NAME, and that is not a
// refinement — it is the difference between the contents arm working and
// not. Story 12.3 rewrote closedValigns from a map literal into a builder
// over a named slice:
//
//	var StyleValignTokens = []string{"top", "middle", "bottom"}
//	var closedValigns = func() map[string]bool { ...range StyleValignTokens... }()
//
// which is a GOOD change — one declaration, and the loader's refusal
// sentence is derived from it rather than restated — but it took the set's
// members out of the composite literal this scanner used to read, so the
// second arm of TestClosedSetsNeverIncludeMediaType (catch a media-type key
// by CONTENTS under an existing var) silently stopped seeing them. The
// named slice is not itself scanned, because `StyleValignTokens` has no
// `closed` prefix.
//
// So the walk is now: every string literal anywhere in the initializer's
// subtree, and every identifier in that subtree resolved against this
// file's own package-level var and const declarations and walked in turn.
// `visited` bounds the recursion against a cycle. The widening is
// deliberate and one-directional — it can only find MORE keys, never
// fewer, and a guard that admits too much of its own file is the safe
// error here.
func scanClosedSets(path string) (setNames []string, allKeys []string, err error) {
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, path, nil, 0)
	if perr != nil {
		return nil, nil, perr
	}

	// Every package-level var and const in the file, by name, so an
	// initializer that merely NAMES its members can be followed to them.
	declared := map[string][]ast.Expr{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, ident := range vs.Names {
				if i < len(vs.Values) {
					declared[ident.Name] = append(declared[ident.Name], vs.Values[i])
				}
			}
		}
	}

	visited := map[string]bool{}
	var collect func(expr ast.Expr)
	collect = func(expr ast.Expr) {
		ast.Inspect(expr, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					allKeys = append(allKeys, strings.Trim(n.Value, `"`))
				}
			case *ast.Ident:
				if visited[n.Name] {
					return true
				}
				visited[n.Name] = true
				for _, value := range declared[n.Name] {
					collect(value)
				}
			}
			return true
		})
	}

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 {
				continue
			}
			name := vs.Names[0].Name
			if !strings.HasPrefix(name, "closed") {
				continue
			}
			setNames = append(setNames, name)
			for _, val := range vs.Values {
				collect(val)
			}
		}
	}
	return setNames, allKeys, nil
}

// TestClosedSetsNeverIncludeMediaType is AC11's mechanical half.
func TestClosedSetsNeverIncludeMediaType(t *testing.T) {
	setNames, allKeys, err := scanClosedSets(closedSetsSource)
	if err != nil {
		t.Fatalf("scan %s: %v", closedSetsSource, err)
	}
	if len(setNames) == 0 {
		t.Fatal("vacuity guard (D-000.9): found zero closed* set declarations in closedsets.go")
	}
	for _, name := range setNames {
		if strings.Contains(strings.ToLower(name), "mediatype") {
			t.Fatalf("AC11 violation: closedsets.go declares %q — mediaType must never join this "+
				"file's closed-set inventory (D-1.4.12: doing so is a permanent MAJOR version bump)", name)
		}
	}
	// The prefix population, WIDENED BY STORY 8.3 to include "font/".
	//
	// THE POPULATION THIS MEASURED: every mediaType string appearing anywhere
	// under fixtures/ at f51dd5e is image/png or image/jpeg, and the only
	// media types this library recognises at all are those two (image.go's
	// decodeRecognisedImage) plus font/ttf and font/otf (fontasset.go's
	// decodeRecognisedFont) — four strings, over three prefixes: "image/",
	// "font/" and the "application/" spelling older font wrappers use
	// (application/font-sfnt, application/x-font-ttf), which was already
	// listed. FONT MEDIA TYPES ARE NOT A CLOSED SET, and that sentence is
	// this loop's job rather than a comment's: D-1.8.1 (as amended) forbids
	// one, and its own note predicted this recurrence "later for font
	// formats". Adding font/ttf to closedsets.go would make every future font
	// container — WOFF2 and whatever follows it — a permanent MAJOR version
	// bump on the format under D-1.4.12.
	for _, key := range allKeys {
		if strings.HasPrefix(key, "image/") || strings.HasPrefix(key, "font/") || strings.HasPrefix(key, "application/") {
			t.Fatalf("AC11 violation: closedsets.go contains a media-type-shaped key %q inside one of "+
				"its closed sets (D-1.4.12)", key)
		}
	}
}

// TestClosedSetsMediaTypeRedProof is this test's own red-proof: add a
// closedMediaTypes map (the suggested resolution's own example) to a
// scratch copy of the real closedsets.go and confirm the scanner names
// it, per D-000.13.
func TestClosedSetsMediaTypeRedProof(t *testing.T) {
	real, err := os.ReadFile(closedSetsSource)
	if err != nil {
		t.Fatalf("read %s: %v", closedSetsSource, err)
	}
	mutated := string(real) + "\n" +
		"var closedMediaTypes = map[string]bool{\n\t\"image/png\": true, \"image/jpeg\": true,\n}\n"

	dir := t.TempDir()
	scratch := filepath.Join(dir, "closedsets.go")
	if err := os.WriteFile(scratch, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write scratch closedsets.go: %v", err)
	}

	setNames, allKeys, err := scanClosedSets(scratch)
	if err != nil {
		t.Fatalf("scan mutated closedsets.go: %v", err)
	}
	if len(setNames) == 0 {
		t.Fatal("vacuity guard: mutated scratch file produced zero set names")
	}

	named := false
	for _, name := range setNames {
		if strings.Contains(strings.ToLower(name), "mediatype") {
			named = true
		}
	}
	if !named {
		t.Fatalf("RP: expected closedMediaTypes to be named among %v", setNames)
	}
	foundKey := false
	for _, key := range allKeys {
		if key == "image/png" {
			foundKey = true
		}
	}
	if !foundKey {
		t.Fatalf("RP: expected the media-type-shaped key \"image/png\" to be extracted, got %v", allKeys)
	}
}

// TestClosedSetsNeverIncludeFontMediaTypeRedProof is Story 8.3's red proof
// for the "font/" prefix the guard above gained, and it is a SEPARATE test
// rather than another arm of the image one on purpose: the image proof passes
// on the "image/" branch of the same `||`, so extending it would have left the
// new branch unexercised while the suite went green — the exact
// "unexercised, not enforced" pattern this whole file exists to close.
//
// It builds a scratch closedsets.go declaring a font media-type set — the
// shape a future agent implementing "the closed set of font formats" would
// reach for — and asserts BOTH halves the real test checks: the set's NAME is
// scanned, and its font-media-type-shaped KEY is extracted.
func TestClosedSetsNeverIncludeFontMediaTypeRedProof(t *testing.T) {
	real, err := os.ReadFile(closedSetsSource)
	if err != nil {
		t.Fatalf("read %s: %v", closedSetsSource, err)
	}
	mutated := string(real) + "\n" +
		"var closedFontMediaTypes = map[string]bool{\n\t\"font/ttf\": true, \"font/otf\": true,\n}\n"

	dir := t.TempDir()
	scratch := filepath.Join(dir, "closedsets.go")
	if err := os.WriteFile(scratch, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write scratch closedsets.go: %v", err)
	}

	setNames, allKeys, err := scanClosedSets(scratch)
	if err != nil {
		t.Fatalf("scan mutated closedsets.go: %v", err)
	}
	if len(setNames) == 0 {
		t.Fatal("vacuity guard: mutated scratch file produced zero set names")
	}
	named := false
	for _, name := range setNames {
		if strings.Contains(strings.ToLower(name), "mediatype") {
			named = true
		}
	}
	if !named {
		t.Fatalf("RP: expected closedFontMediaTypes to be named among %v", setNames)
	}
	found := false
	for _, key := range allKeys {
		if strings.HasPrefix(key, "font/") {
			found = true
		}
	}
	if !found {
		t.Fatalf("RP: expected a font-media-type-shaped key to be extracted, got %v", allKeys)
	}
}

// alignDocWithStyle is one text element carrying `style` verbatim, so a
// case can put any align spelling — legal or not — at the STYLE
// attachment point.
func alignDocWithStyle(style string) []byte {
	return []byte(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v", "style": {` + style + `}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}
`)
}

// alignDocWithTable is one table carrying a headerStyle, a single
// column and — since Story 7.8 — its own `style` block, so a case can
// put an align spelling at any of the THREE table attachment points and
// see which one rejected it. The table's own `style` was absent from
// this helper until Story 7.8, which is why nothing in the repository
// pinned a table's `style.align: "justify"` while it was silently
// accepted (DW-29).
func alignDocWithTable(tableStyle, headerStyle, columnExtra string) []byte {
	return []byte(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "columns": [{"id": "e2", "label": "L", "width": 100, "bind": "{{row.v}}"` + columnExtra + `}],
          "style": {` + tableStyle + `},
          "headerStyle": {` + headerStyle + `}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}
`)
}

// TestAlignSetsAreThreeSetsPinnedAgainstTheirMaps is Story 7.3's
// structural guard, WIDENED at Story 7.8 from two sets to three. It
// exists because NOTHING enumerated the shared map's members before the
// first split — TestClosedSetsNeverIncludeMediaType scans this file's
// inventory but only for media-type shapes, so a `justify` added to the
// shared map would have passed every test in the repository while
// silently legalising justified table cells.
//
// It pins each ordered slice against its map in BOTH directions and as
// an exact literal SEQUENCE (LocaleTags' own AC4a shape): a set that
// gains a member without gaining it in the slice ships a rejection
// message that lies about what is legal, and a slice reordered by
// accident reorders the message every load error prints.
//
// THE ASSERTION THIS TEST USED TO CARRY WAS WRONG, and its name said
// so. `len(StyleAlignTokens) == len(ColumnAlignTokens)+1` states "there
// are exactly two sets and they differ by `justify`", which was true of
// the KEY-PATH partition Story 7.3 shipped and false of the CONSUMER
// partition that actually holds. The replacement below states the
// consumer invariant instead: `justify` is admitted by exactly one of
// the three sets, and the two TABLE sets are the same three values
// because one alignFallback consumes both.
func TestAlignSetsAreThreeSetsPinnedAgainstTheirMaps(t *testing.T) {
	for _, c := range []struct {
		name   string
		tokens []string
		set    map[string]bool
		want   []string
	}{
		{"style", StyleAlignTokens, closedStyleAligns, []string{"left", "center", "right", "justify"}},
		{"table-style", TableStyleAlignTokens, closedTableStyleAligns, []string{"left", "center", "right"}},
		{"column", ColumnAlignTokens, closedColumnAligns, []string{"left", "center", "right"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if len(c.tokens) != len(c.want) {
				t.Fatalf("%s tokens = %v, want exactly %v", c.name, c.tokens, c.want)
			}
			for i, w := range c.want {
				if c.tokens[i] != w {
					t.Errorf("%s token %d is %q, want %q — the order IS the message's order", c.name, i, c.tokens[i], w)
				}
			}
			// slice -> map
			for _, tok := range c.tokens {
				if !c.set[tok] {
					t.Errorf("%s: %q is in the ordered slice but the map does not admit it — the message would name a value the loader refuses", c.name, tok)
				}
			}
			// map -> slice
			if len(c.set) != len(c.tokens) {
				t.Errorf("%s: the map has %d members and the slice %d — the loader would admit a value the message never names", c.name, len(c.set), len(c.tokens))
			}
		})
	}

	// THE PARTITION ITSELF, stated as the facts D-7.3.1 and Story 7.8
	// turn on. `justify` belongs to exactly ONE of the three sets: the
	// one whose consumer is the paragraph justifier.
	if !closedStyleAligns[AlignJustify] {
		t.Error("a non-table element's style.align must admit justify (FR47)")
	}
	justifying := 0
	for _, set := range []map[string]bool{closedStyleAligns, closedTableStyleAligns, closedColumnAligns} {
		if set[AlignJustify] {
			justifying++
		}
	}
	if justifying != 1 {
		t.Fatalf("%d of the three align sets admit justify, want exactly 1 — the paragraph justifier's set alone. A table's style.align, its headerStyle.align and its columns[].align are read into ONE alignFallback and drawn by ONE set of switches, so any of them admitting justify re-opens DW-29", justifying)
	}
	if closedTableStyleAligns[AlignJustify] {
		t.Fatal("a table's style.align/headerStyle.align must NEVER admit justify — it cascades into the same cell switches columns[].align feeds, and the document would pay a MAJOR version bump for a render identical to `left` (DW-29, D-7.3.1's partition corrected at Story 7.8)")
	}
	if closedColumnAligns[AlignJustify] {
		t.Fatal("columns[].align must NEVER admit justify — justified table cells are a separate scope decision, not a side effect of a map edit (D-7.3.1)")
	}

	// THE TWO TABLE SETS ARE THE SAME SET OF VALUES, and that is the
	// consumer partition made visible: one alignFallback, one
	// vocabulary. They are separate DECLARATIONS (separately documented
	// closed sets, either free to move) but a divergence between them
	// would mean the same switch could receive a value it cannot draw.
	if len(TableStyleAlignTokens) != len(ColumnAlignTokens) {
		t.Errorf("the table-style set has %d members and the column set %d — both feed the same alignFallback, so a value legal at one and not the other is a value the cell switches cannot account for", len(TableStyleAlignTokens), len(ColumnAlignTokens))
	}
	for i, tok := range TableStyleAlignTokens {
		if i < len(ColumnAlignTokens) && ColumnAlignTokens[i] != tok {
			t.Errorf("table-style token %d is %q but column token %d is %q", i, tok, i, ColumnAlignTokens[i])
		}
	}

	// And no set is another by subtraction at a call site: every table
	// value is also a legal non-table style value, so the three sets
	// have not diverged in the wrong direction.
	for _, tok := range TableStyleAlignTokens {
		if !closedStyleAligns[tok] {
			t.Errorf("%q is a legal table align but not a legal style align — the sets have diverged in the wrong direction", tok)
		}
	}
	for _, tok := range ColumnAlignTokens {
		if !closedStyleAligns[tok] {
			t.Errorf("%q is a legal column align but not a legal style align — the sets have diverged in the wrong direction", tok)
		}
	}

	// The exported predicates are the single source the property-command
	// path validates through; each must agree with the map it wraps, and
	// they must disagree with each other on exactly `justify`.
	for _, tok := range StyleAlignTokens {
		if !IsStyleAlign(tok) {
			t.Errorf("IsStyleAlign(%q) = false", tok)
		}
	}
	if IsStyleAlign("middle") || IsStyleAlign("") {
		t.Error("IsStyleAlign admits a value the closed set does not")
	}
	for _, tok := range TableStyleAlignTokens {
		if !IsTableStyleAlign(tok) {
			t.Errorf("IsTableStyleAlign(%q) = false", tok)
		}
	}
	if IsTableStyleAlign(AlignJustify) {
		t.Error("IsTableStyleAlign admits justify — the property-command path would write onto a table exactly the value the loader refuses")
	}
	if IsTableStyleAlign("middle") || IsTableStyleAlign("") {
		t.Error("IsTableStyleAlign admits a value the closed set does not")
	}
}

// TestAlignClosedSetsRejectAtTheRightSiteWithTheirOwnMessage is the
// partition's BEHAVIOURAL red-proof: the same word, `justify`, must load
// at a TEXT element's style.align and be a located load error at all
// three of a table's attachment points — and each message must list
// exactly the members of the set that rejected it, not another set's.
//
// Legs (b) and (b2) are Story 7.8's inversion. (b) asserted that
// `headerStyle.align: "justify"` MUST LOAD, which was correct against
// the contract as Story 7.3 wrote it and is the shipped behaviour
// DW-29 records as the defect. It now asserts the refusal. (b2) is new:
// a table's OWN style.align was pinned by nothing at all, because the
// helper emitted no table `style` block.
func TestAlignClosedSetsRejectAtTheRightSiteWithTheirOwnMessage(t *testing.T) {
	// (a) justify LOADS at a TEXT element's style.align.
	if _, err := ParseDocument(alignDocWithStyle(`"align": "justify"`)); err != nil {
		t.Errorf("style.align: \"justify\" on a text element must load: %v", err)
	}
	// (b) justify is REFUSED at a table's headerStyle.align, located at
	//     the element and the field.
	_, berr := ParseDocument(alignDocWithTable("", `"align": "justify"`, ""))
	if berr == nil {
		t.Fatal("headerStyle.align: \"justify\" must be a load error — a header cell is a table cell, and it takes the same alignFallback columns[].align feeds (DW-29)")
	}
	assertLocatedTableAlignRefusal(t, berr, "headerStyle.align")

	// (b2) justify is REFUSED at a table's OWN style.align. This is the
	//      door D-7.3.1's key-path partition left open: the guard was
	//      written for `columns[]` versus `style`, but a table's own
	//      style cascades into every cell that declares no column align.
	_, b2err := ParseDocument(alignDocWithTable(`"align": "justify"`, "", ""))
	if b2err == nil {
		t.Fatal("a TABLE's own style.align: \"justify\" must be a load error — it cascades into every header, body and footer cell through alignFallback (DW-29)")
	}
	assertLocatedTableAlignRefusal(t, b2err, "style.align")

	// (c) justify is REFUSED at columns[].align, located at the column.
	_, err := ParseDocument(alignDocWithTable("", "", `, "align": "justify"`))
	if err == nil {
		t.Fatal("columns[].align: \"justify\" must be a load error — justified table cells are not in scope")
	}
	msg := err.Error()
	if !strings.Contains(msg, "e2") {
		t.Errorf("the column rejection must be LOCATED at the column that carries it, got: %v", err)
	}
	if !strings.Contains(msg, "align") {
		t.Errorf("the column rejection must name the field, got: %v", err)
	}
	if !strings.Contains(msg, "not one of the closed set left, center, right") {
		t.Errorf("the column rejection must list exactly the COLUMN set, got: %v", err)
	}
	if strings.Contains(msg, "justify,") || strings.Contains(msg, ", justify") {
		t.Errorf("the column rejection must not name justify as legal, got: %v", err)
	}

	// (d) An illegal value on a TEXT element's style is rejected with
	//     the STYLE set, which includes justify. This is the honesty
	//     half: before the split both sites printed one hand-written
	//     literal, so extending either set would have shipped a message
	//     that lied.
	_, serr := ParseDocument(alignDocWithStyle(`"align": "middle"`))
	if serr == nil {
		t.Fatal("style.align: \"middle\" must be a load error")
	}
	smsg := serr.Error()
	if !strings.Contains(smsg, "not one of the closed set left, center, right, justify") {
		t.Errorf("the style rejection must list exactly the STYLE set, justify included, got: %v", serr)
	}
	if !strings.Contains(smsg, "style.align") {
		t.Errorf("the style rejection must be located at style.align, got: %v", serr)
	}

	// (e) …and at a table's headerStyle it names headerStyle, not its
	//     sibling — and it does NOT name justify, because that element
	//     can never carry it. A type-guard bolted above the two shipped
	//     sets would pass every other leg here and fail this one.
	_, herr := ParseDocument(alignDocWithTable("", `"align": "middle"`, ""))
	if herr == nil {
		t.Fatal("headerStyle.align: \"middle\" must be a load error")
	}
	if !strings.Contains(herr.Error(), "headerStyle.align") {
		t.Errorf("the headerStyle rejection must be located at headerStyle.align, got: %v", herr)
	}
	if !strings.Contains(herr.Error(), closedSetMessage(TableStyleAlignTokens)) {
		t.Errorf("a table's headerStyle rejection must list the TABLE set, got: %v", herr)
	}
	if strings.Contains(herr.Error(), AlignJustify) {
		t.Errorf("a table's rejection message must never name justify as legal — no table element can carry it, got: %v", herr)
	}

	// (e2) …and at a table's OWN style.align, the same. This is the
	//      I/O matrix's "table style middle" row: the message must
	//      list left, center, right and must NOT name justify.
	_, terr := ParseDocument(alignDocWithTable(`"align": "middle"`, "", ""))
	if terr == nil {
		t.Fatal("a table's style.align: \"middle\" must be a load error")
	}
	if !strings.Contains(terr.Error(), "style.align") || !strings.Contains(terr.Error(), "e1") {
		t.Errorf("the table-style rejection must be located at the element and style.align, got: %v", terr)
	}
	if !strings.Contains(terr.Error(), closedSetMessage(TableStyleAlignTokens)) {
		t.Errorf("a table's style rejection must list the TABLE set, got: %v", terr)
	}
	if strings.Contains(terr.Error(), AlignJustify) {
		t.Errorf("a table's rejection message must never name justify as legal, got: %v", terr)
	}

	// (f) THE MESSAGES ARE DERIVED, not restated: each one contains the
	//     exact rendering of its own ordered slice. A future edit to any
	//     set changes both the enforcement and the message, or this
	//     fails.
	if !strings.Contains(msg, closedSetMessage(ColumnAlignTokens)) {
		t.Errorf("the column message is not derived from ColumnAlignTokens: %v", err)
	}
	if !strings.Contains(smsg, closedSetMessage(StyleAlignTokens)) {
		t.Errorf("the style message is not derived from StyleAlignTokens: %v", serr)
	}
	if !strings.Contains(b2err.Error(), closedSetMessage(TableStyleAlignTokens)) {
		t.Errorf("the table-style message is not derived from TableStyleAlignTokens: %v", b2err)
	}
}

// assertLocatedTableAlignRefusal is AC1's "located" clause, checked the
// same way at both of a table's style attachment points: the element id,
// the field, and a legal-values list that is the TABLE set and names no
// value that element cannot carry.
func assertLocatedTableAlignRefusal(t *testing.T, err error, field string) {
	t.Helper()
	le, ok := err.(*LoadError)
	if !ok {
		t.Fatalf("the refusal must be a *LoadError so the field and element travel as DATA, got %T: %v", err, err)
	}
	if le.ElementID != "e1" {
		t.Errorf("LoadError.ElementID = %q, want %q — the refusal must name the element", le.ElementID, "e1")
	}
	if le.Field != field {
		t.Errorf("LoadError.Field = %q, want %q", le.Field, field)
	}
	if le.Value != AlignJustify {
		t.Errorf("LoadError.Value = %q, want %q", le.Value, AlignJustify)
	}
	msg := le.Error()
	for _, want := range []string{"e1", field, closedSetMessage(TableStyleAlignTokens)} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal message must contain %q, got: %s", want, msg)
		}
	}
	if strings.Contains(msg, AlignJustify+",") || strings.Contains(msg, ", "+AlignJustify) {
		t.Errorf("the refusal must not name justify among the legal values for a table, got: %s", msg)
	}
}

// ---------------------------------------------------------------------------
// STORY 12.2 / D-12.C: THE OFFSET'S ONE PREDICATE, AND THE GAP IT CLOSES.

// utcOffsetProbes is the shared candidate table. Both tests below read
// it, so the predicate tie and the loader tie cannot disagree about
// what the answer is supposed to be — which is the whole point of
// having one predicate.
//
// THE OUT-OF-RANGE ROWS ARE IN THE REFUSE COLUMN BY RULING (D-12.C).
// `+99:99` and `+24:00` are not fixed offsets, folio-format.md's `utcOffset`
// field-table row says it is a fixed offset spelled ±HH:MM, and the repaired
// pattern implements that rather than narrowing it — 0 of the 31
// `.folio` files in the corpus is excluded (D-12.C.3; D-12.C's own
// table says 28, a smaller population, not a different answer).
//
// THIS TABLE IS READ BY THE COMMAND DOOR'S TEST TOO. `package folio8`'s
// document_settings_command_test.go extracts these rows out of this
// file's SOURCE TEXT and drives setDocumentUTCOffset with them, so a
// row added here reaches the command door without an edit there. That
// is the repo's own idiom for a fixture two packages must share
// (canvas_projection_wire_test.go reads engine-protocol.ts the same
// way); the alternative — a second list beside the command test — is
// exactly the clerical drift that lets one door be probed over a
// strictly smaller set than the door it claims to match.
//
// `Z` is refused HERE and admitted by internal/expr's
// parseUTCOffsetMinutes; that asymmetry is deliberate, is the one
// expected one, and is asserted with its reason at
// internal/expr/offset_divergence_test.go.
var utcOffsetProbes = []struct {
	value string
	admit bool
	why   string
}{
	{"+00:00", true, "UTC, the corpus's own commonest value (24 of the 31 .folio files, D-12.C.3)"},
	{"-00:00", true, "negative zero is a legal spelling of the same offset and the format does not forbid it"},
	{"+07:00", true, "Bangkok; the corpus's only other value (7 of the 31 .folio files, D-12.C.3)"},
	{"-05:30", true, "a half-hour offset, negative"},
	{"+14:00", true, "Kiritimati, the largest real offset"},
	{"-12:00", true, "the smallest real offset"},
	{"+23:59", true, "the last value HH:MM can spell"},
	{"+99:99", false, "D-12.C: no clock has it; it LOADED before Story 12.2 and then failed at render"},
	{"+24:00", false, "HH is an hour; 24 is not one"},
	{"+00:60", false, "MM is a minute; 60 is not one"},
	{"Z", false, "it is not ±HH:MM, the syntax folio-format.md's `utcOffset` field-table row states; it is RFC 3339's UTC spelling and belongs to report DATA (D-12.C). That is the whole ground — -00:00 above is admitted because it IS ±HH:MM, and the two rows differ by syntax and nothing else (D-12.C.4)"},
	{"+7:00", false, "one hour digit"},
	{"+0700", false, "no colon"},
	{"07:00", false, "no sign"},
	{"", false, "absent is a different refusal; empty is this one"},
	{"+07:00 ", false, "trailing space"},
	{" +07:00", false, "leading space"},
	{"+07:0a", false, "a non-digit where a digit must be"},
}

// TestIsUTCOffsetMatchesTheLoader ties the EXPORTED PREDICATE to the
// loader that consumes it, on TestClosedLocalesMatchesLocaleTags'
// shape. IsUTCOffset is the one authority the loader (parse.go) and the
// command door (component_commands.go's setDocumentUTCOffset) both ask,
// and a predicate nothing ties can drift from the rule it claims to
// enforce — silently, because both callers would agree with each other.
func TestIsUTCOffsetMatchesTheLoader(t *testing.T) {
	if len(utcOffsetProbes) == 0 {
		t.Fatal("presence precondition (D-000.9): the probe table is empty")
	}
	// Non-vacuity in BOTH directions, so a predicate stuck at true or at
	// false cannot pass this file.
	admits, refuses := 0, 0
	for _, probe := range utcOffsetProbes {
		if probe.admit {
			admits++
		} else {
			refuses++
		}
		if IsUTCOffset(probe.value) != probe.admit {
			t.Errorf("IsUTCOffset(%q) = %v, want %v — %s", probe.value, !probe.admit, probe.admit, probe.why)
		}
	}
	if admits == 0 || refuses == 0 {
		t.Fatalf("the probe table has %d admit rows and %d refuse rows; both must be non-zero", admits, refuses)
	}
	// AND THE PHRASE, spelled once. The loader's refusal renders it and
	// so does the command's, so a re-typing here is the drift the
	// constant exists to prevent.
	if UTCOffsetSyntax != "±HH:MM" {
		t.Errorf("UTCOffsetSyntax = %q, want ±HH:MM — folio-format.md's own utcOffset field-table row", UTCOffsetSyntax)
	}
}

// TestUTCOffsetLoadRefusalIsReachableAndLocated is D-12.C's RED-PROOF,
// and it is deliberately about REACHABILITY rather than about the
// regexp. A test asserting only that the pattern rejects `+99:99`
// proves the pattern changed; it does not prove the defect closed.
//
// The defect: before Story 12.2 a document declaring `"+99:99"` LOADED,
// and every formatDate in it then failed at render with
// `expr: invalid UTC offset "+99:99"` — a refusal the author reached
// only by rendering, attributed to nothing they could see. It must now
// be refused AT LOAD, with the field named.
func TestUTCOffsetLoadRefusalIsReachableAndLocated(t *testing.T) {
	base := string(minimalDocWithElements(nil, 1))
	// The fixture precondition, stated rather than assumed: the
	// unmodified document LOADS, so a refusal below is attributable to
	// the offset and not to the fixture.
	if _, err := ParseDocument([]byte(base)); err != nil {
		t.Fatalf("fixture precondition: the base document must load: %v", err)
	}
	for _, probe := range utcOffsetProbes {
		t.Run(probe.value, func(t *testing.T) {
			bad := strings.Replace(base, `"utcOffset": "+00:00",`, `"utcOffset": "`+probe.value+`",`, 1)
			// The one row where the substitution is legitimately a
			// no-op is the base document's own value; every other row
			// must actually have changed the bytes, or it would be
			// measuring the fixture instead of the probe.
			if bad == base && probe.value != "+00:00" {
				t.Fatal("fixture substitution did not match anything")
			}
			_, err := ParseDocument([]byte(bad))
			if probe.admit {
				if err != nil {
					// +00:00 substituted for itself is a no-op and is
					// the one row that cannot reach here.
					t.Fatalf("the loader refused %q, which IsUTCOffset admits: %v — the two doors have drifted", probe.value, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("the loader ADMITTED %q — %s", probe.value, probe.why)
			}
			var loadErr *LoadError
			if !errors.As(err, &loadErr) {
				t.Fatalf("refusal for %q is %T (%v), want a *LoadError naming the field", probe.value, err, err)
			}
			if loadErr.Field != "utcOffset" {
				t.Fatalf("refusal for %q names field %q, want utcOffset — an unlocated refusal is the gap this test exists for", probe.value, loadErr.Field)
			}
			if !strings.Contains(loadErr.Reason, UTCOffsetSyntax) {
				t.Fatalf("refusal for %q reads %q, want it to carry %s", probe.value, loadErr.Reason, UTCOffsetSyntax)
			}
		})
	}
}

// TestClosedSetsSeeAMemberDeclaredThroughAnotherName is the review patch set's
// red proof for the hole Story 12.3 opened in the CONTENTS arm above.
//
// 12.3 rewrote closedValigns from a map literal into a builder over the named
// slice StyleValignTokens. That is the right shape — one declaration, and the
// loader's refusal sentence is derived from it — but the scanner only read a
// `*ast.CompositeLit` initializer, so the set's members stopped being scanned
// at all, and `StyleValignTokens` is not scanned under its own name either
// because it carries no `closed` prefix. The guard's stated second arm — catch
// a media-type key by CONTENTS, under an EXISTING var — had a hole in it, and
// the suite was green.
//
// This plants a media-type key into the NAMED SLICE the real builder reads and
// asserts the scanner surfaces it. Under the pre-patch scanner it does not,
// which is the proof.
func TestClosedSetsSeeAMemberDeclaredThroughAnotherName(t *testing.T) {
	real, err := os.ReadFile(closedSetsSource)
	if err != nil {
		t.Fatalf("read %s: %v", closedSetsSource, err)
	}
	// The plant goes into StyleValignTokens itself — the slice closedValigns is
	// built FROM — so the only way to see it is to follow the builder through
	// the name.
	const original = `var StyleValignTokens = []string{"top", "middle", "bottom"}`
	planted := `var StyleValignTokens = []string{"top", "middle", "bottom", "image/png"}`
	if !strings.Contains(string(real), original) {
		t.Fatalf("red-proof precondition: %s no longer declares StyleValignTokens as this test expects; re-derive the plant rather than deleting the check", closedSetsSource)
	}
	mutated := strings.Replace(string(real), original, planted, 1)

	dir := t.TempDir()
	scratch := filepath.Join(dir, "closedsets.go")
	if err := os.WriteFile(scratch, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write scratch closedsets.go: %v", err)
	}

	setNames, allKeys, err := scanClosedSets(scratch)
	if err != nil {
		t.Fatalf("scan mutated closedsets.go: %v", err)
	}
	if len(setNames) == 0 {
		t.Fatal("vacuity guard: mutated scratch file produced zero set names")
	}
	if !slices.Contains(setNames, "closedValigns") {
		t.Fatalf("vacuity guard: closedValigns is not among the scanned sets %v, so this proves nothing", setNames)
	}
	if !slices.Contains(allKeys, "image/png") {
		t.Fatalf("the scanner cannot see a member of a closed set declared through another name: planted \"image/png\" into StyleValignTokens, which closedValigns is built from, and the extracted keys are %v", allKeys)
	}
	// AND THE REAL FILE IS CLEAN, which is the same claim
	// TestClosedSetsNeverIncludeMediaType makes — asserted here too so the
	// widened scanner is exercised against the shipping file and not only
	// against a plant.
	_, realKeys, err := scanClosedSets(closedSetsSource)
	if err != nil {
		t.Fatalf("scan %s: %v", closedSetsSource, err)
	}
	if !slices.Contains(realKeys, "top") {
		t.Fatalf("vacuity guard: the widened scanner reads no valign member out of the shipping file, got %v", realKeys)
	}
}
