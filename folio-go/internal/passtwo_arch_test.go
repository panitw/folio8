package arch

// Story 2.6's AD-4 guard: "Two passes, and the second one lays nothing
// out."
//
// WHY THIS IS A FORWARD GUARD AND NOT A RESTATEMENT (D-000.42, and
// D-000.16 (limitation) which ruled it). `internal/pdf` performs no
// measurement, no line breaking and no pagination today, so an assertion
// that it does not would pass on day zero. What is NOT true today is
// that anything PREVENTS it from acquiring one:
//
//   - lint's `stage-rank` rule says "a package may import only LOWER
//     ranks". `layout` is rank 7 and `pdf` is rank 8, so
//     `internal/pdf -> internal/layout` is a DOWNWARD edge and is LEGAL
//     today. The rank guard enforces AD-5 and does nothing for AD-4.
//   - The limitation is structural, not an oversight in the table. A
//     rank order expresses "no BACKWARD edges"; it cannot express "no
//     edge to THIS PARTICULAR lower package". `internal/pdf`
//     legitimately depends on `internal/pagemodel` — the VALUE — and
//     must not depend on `internal/layout` — the COMPUTATION. Both sit
//     below it, so no rank assignment separates them.
//   - internal/bandcomposition_arch_test.go's
//     TestInternalLayoutDoesNotReachInternalPDF asserts the OPPOSITE
//     arrow (layout -> pdf, AD-5's). It cannot see this one.
//
// So this file is a SUPPLEMENTARY FORBIDDEN EDGE the rank table
// structurally cannot carry. Its counterpart — that pass two emits
// exactly len(pages) page objects and invents none — lives in
// internal/pdf/passtwo_pagecount_test.go, because only a caller inside
// that package can hand SerializeTextDocument an input and read the
// bytes back.

import (
	"go/ast"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// layoutSymbolSubstrings is the closed list of LAYOUT-COMPUTATION
// vocabulary that must not appear as an identifier anywhere under
// internal/pdf. Each entry names a quantity that pass ONE decides.
//
// Matched as lower-case substrings against every identifier the package
// declares or references, the same fail-safe idiom pdfConceptSubstrings
// uses: an exact-name list is the erosion path, because
// `paginatePages` and `repaginate` are both the thing being excluded.
//
// WHAT IS DELIBERATELY ABSENT FROM THIS LIST, and why, so a future
// reader does not "complete" it and turn a proven guard into a broken
// one:
//
//   - "page" / "pages". internal/pdf is a PDF writer; `pageIDs`,
//     `/Type /Pages` and `numPages` are its own legitimate vocabulary.
//     Banning it would make the guard un-satisfiable, and a guard that
//     must be suppressed is not a guard.
//   - "baseline". pagemodel.TextRun.BaselineOffset is a value pass one
//     COMPUTED and pass two merely READS. Reading a finished layout
//     quantity is exactly what AD-4 asks pass two to do; DERIVING one is
//     what it forbids. "firstbaseline" — the name of the derivation in
//     wrap.go — is banned instead.
var layoutSymbolSubstrings = []string{
	"paginat",          // any pagination, spelled any way
	"linebreak",        // line breaking
	"breakopportun",    // internal/text.Opportunities' subject
	"opportunit",       // ditto, however spelled
	"packline",         // wrap.go's packLines
	"wrapline",         // its type, wrappedLine
	"contentheight",    // layout.ContentHeight — a derived band height
	"bandorigin",       // band placement
	"placeinband",      // layout.PlaceInBand
	"composepage",      // layout.ComposePage
	"pagegeometry",     // layout.PageGeometry
	"shapesegment",     // render.go's shapeSegments
	"verticalmodel",    // wrap.go's verticalModel
	"verticalmetric",   // its result type
	"firstbaseline",    // the first-baseline DERIVATION (see note above)
	"lineadvance",      // wrap.go's lineAdvance
	"measurerunerange", // wrap.go's measureRuneRange
	"columnwindow",     // Story 2.6's own pagination vocabulary

	// Story 2.7, AC2.2: the {{page}} slot's BOX is measured in pass
	// one and its RESOLUTION happens BETWEEN the passes, in package
	// folio8 — never inside internal/pdf, which only ever reads
	// pagemodel.TextRun.Glyphs (never .PageSlot, which resolution has
	// already cleared by the time internal/pdf sees a page). This is
	// not a restatement of "internal/pdf performs no measurement": it
	// is the story's OWN vocabulary, added because a re-implementation
	// naming none of the ABOVE substrings could still smuggle the
	// slot's resolution into pass two under a fresh name.
	"pageslot",        // pagemodel.PageNumberSlot / TextRun.PageSlot
	"digitcid",        // pagemodel.PageNumberSlot.DigitCID
	"digittable",      // page_number.go's digitTableRun
	"resolvepagerun",  // page_number.go's resolvePageRunForPage
	"buildpagenumber", // page_number.go's buildPageNumberSlot
	"pageslotspan",    // page_number.go's pageSlotSpan
}

// TestInternalPDFReachesNoLayoutComputation is Story 2.6's AC5, first
// half: pass two may not IMPORT the layout stage, transitively, and may
// not NAME a layout computation.
//
// TRANSITIVE, not direct: an arrow laundered through a third package is
// the same arrow — the same reasoning
// TestInternalLayoutDoesNotReachInternalPDF gives for its own direction.
//
// PRESENCE PRECONDITIONS (D-000.21 sharpened). Two, because two
// different kinds of nothing would otherwise read as a pass:
//
//  1. internal/pdf must exist and have at least one first-party import,
//     or "layout is not among them" is satisfied by a package that
//     imports nothing.
//  2. the identifier sweep must have actually collected identifiers, or
//     "none of them matches" is satisfied by an empty sweep.
func TestInternalPDFReachesNoLayoutComputation(t *testing.T) {
	internalDir := filepath.Join(moduleRoot(t), "internal")
	graph := firstPartyImportGraph(t, internalDir)

	direct, ok := graph["pdf"]
	if !ok {
		t.Fatal("presence precondition: no package directory \"pdf\" found under folio-go/internal/ — AD-4's pass-two arrow is unassertable")
	}
	if len(direct) == 0 {
		t.Fatal("presence precondition (D-000.9): internal/pdf has ZERO first-party imports — \"internal/layout is not among them\" is satisfied vacuously by a package that imports nothing")
	}

	// Transitive reachability, breadth-first, deterministic witness path.
	seen := map[string]bool{"pdf": true}
	queue := []string{"pdf"}
	pathTo := map[string]string{"pdf": "internal/pdf"}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		targets := make([]string, 0, len(graph[cur]))
		for target := range graph[cur] {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		for _, target := range targets {
			if seen[target] {
				continue
			}
			seen[target] = true
			pathTo[target] = pathTo[cur] + " -> internal/" + target
			queue = append(queue, target)
		}
	}

	for _, forbidden := range []string{"layout", "text"} {
		if seen[forbidden] {
			t.Fatalf(
				"AD-4 VIOLATED: internal/pdf reaches internal/%s (%s).\n\n"+
					"AD-4, verbatim: \"Two passes, and the second one lays nothing out.\" internal/pdf is "+
					"pass two. It consumes a FINISHED []pagemodel.Page and serializes it.\n\n"+
					"lint's `stage-rank` rule does NOT catch this: layout is rank 7, pdf is rank 8, so the "+
					"edge is DOWNWARD and legal under \"may import only lower ranks\". A rank order cannot "+
					"express \"no edge to this particular lower package\" (D-000.16 (limitation)) — which is "+
					"why this test exists.\n\n"+
					"The remedy is never to add a rank: internal/pdf legitimately outranks internal/layout. "+
					"Move the decision into pass one and carry its RESULT through internal/pagemodel, which "+
					"both stages may see.",
				forbidden, pathTo[forbidden])
		}
	}

	// --- Identifier sweep -------------------------------------------
	//
	// The import graph catches the edge. This catches a layout
	// computation RE-IMPLEMENTED inside internal/pdf with no import at
	// all — which is the cheaper and likelier way to violate AD-4,
	// because it trips no dependency guard anywhere.
	pdfDir := filepath.Join(internalDir, "pdf")
	files := parseDirNonTest(t, pdfDir)
	if len(files) == 0 {
		t.Fatalf("presence precondition: no non-test .go file under %s", pdfDir)
	}

	identifiers := 0
	type hit struct{ file, ident, banned string }
	var hits []hit
	for _, pf := range files {
		ast.Inspect(pf.file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			identifiers++
			lower := strings.ToLower(id.Name)
			for _, banned := range layoutSymbolSubstrings {
				if strings.Contains(lower, banned) {
					hits = append(hits, hit{pf.rel, id.Name, banned})
				}
			}
			return true
		})
	}
	if identifiers == 0 {
		t.Fatalf("presence precondition (D-000.9): the identifier sweep over %s collected ZERO identifiers — \"none of them names a layout computation\" would be satisfied by an empty sweep", pdfDir)
	}
	t.Logf("identifier sweep: %d identifiers over %d non-test files under internal/pdf, against %d banned substrings",
		identifiers, len(files), len(layoutSymbolSubstrings))

	for _, h := range hits {
		t.Errorf(
			"AD-4 VIOLATED: internal/pdf/%s declares or references %q, which names the layout vocabulary %q.\n"+
				"Pass two lays nothing out. If this quantity is needed here, pass one must compute it and "+
				"carry it through internal/pagemodel.",
			h.file, h.ident, h.banned)
	}
}
