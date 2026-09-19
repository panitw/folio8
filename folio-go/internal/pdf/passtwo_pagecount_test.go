package pdf

// Story 2.6's AC5, second half: pass two emits exactly len(pages) page
// objects — it neither invents a page nor drops one.
//
// The first half (internal/passtwo_arch_test.go) forbids pass two from
// REACHING the layout stage. This half asserts, POSITIVELY, that the one
// quantity pass two could still decide on its own — how many sheets
// exist — is a function of its INPUT and of nothing else.
//
// D-000.45: this asserts a COMPUTED VALUE FROM A DECLARATIVE TABLE, never
// a direction. "More pages in yields more page objects out" is monotone
// and is satisfied by a serializer that emits 2, 5, 900 — the table below
// names the integer.
//
// D-000.33: `/Count` and the `/Type /Page ` object count are TWO
// INDEPENDENT SITES in the byte stream (one in the /Pages parent, one per
// page object) and both are asserted against the same declared integer.
// Asserting only their agreement would be a conservation law, satisfied
// by a serializer that got both wrong the same way.

import (
	"bytes"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// A4, restated as the literals this file compares against rather than
// imported from a page-setup type internal/pdf must not know.
const (
	ptPageWidthMP  geom.Length = 595276
	ptPageHeightMP geom.Length = 841890
)

// TestPassTwoEmitsExactlyOnePageObjectPerInputPage is AC5's positive
// half.
//
// PRESENCE PRECONDITION: each row first asserts the serializer produced a
// non-empty PDF that actually contains a /Pages parent. Without it, a
// serializer returning zero bytes would report 0 page objects and 0
// /Count occurrences, and a row declaring 0 would "pass".
func TestPassTwoEmitsExactlyOnePageObjectPerInputPage(t *testing.T) {
	// The declarative table. Left: how many pages pass one produced.
	// Right: how many /Type /Page objects and what /Count pass two must
	// write. Hand-declared integers, not computed from len(pages) at the
	// assertion site — a table that recomputes its own expectation from
	// the input cannot fail.
	table := []struct {
		name       string
		inputPages int
		wantObjs   int
		wantCount  string
		wantKids   int
	}{
		{"one page", 1, 1, "1", 1},
		{"two pages", 2, 2, "2", 2},
		{"three pages", 3, 3, "3", 3},
		{"seven pages", 7, 7, "7", 7},
	}

	for _, row := range table {
		t.Run(row.name, func(t *testing.T) {
			pages := make([]pagemodel.Page, row.inputPages)
			for i := range pages {
				pages[i] = pagemodel.Page{
					Width:      ptPageWidthMP,
					Height:     ptPageHeightMP,
					MarginTop:  36000,
					MarginLeft: 36000,
				}
			}

			out, err := SerializeTextDocument(pages, nil, nil, nil)
			if err != nil {
				t.Fatalf("SerializeTextDocument(%d pages): %v", row.inputPages, err)
			}
			if len(out) == 0 {
				t.Fatal("presence precondition: SerializeTextDocument returned zero bytes — every count below would read as 0 and a row declaring 0 would pass on a serializer that produced nothing")
			}
			if !bytes.Contains(out, []byte("/Type /Pages /Kids [")) {
				t.Fatal("presence precondition: the produced bytes carry no /Pages parent — the page tree this test measures is not present")
			}

			// Site 1: the per-page objects.
			gotObjs := bytes.Count(out, []byte("<< /Type /Page /Parent "))
			if gotObjs != row.wantObjs {
				t.Errorf("AD-4: pass two emitted %d page objects for %d input pages; the declared value is %d",
					gotObjs, row.inputPages, row.wantObjs)
			}

			// Site 2: /Count in the page tree, independently.
			marker := []byte("] /Count ")
			idx := bytes.Index(out, marker)
			if idx < 0 {
				t.Fatal("presence precondition: no \"] /Count \" in the produced bytes")
			}
			rest := out[idx+len(marker):]
			end := bytes.IndexByte(rest, ' ')
			if end < 0 {
				t.Fatal("malformed /Count in the produced bytes")
			}
			gotCount := string(rest[:end])
			if gotCount != row.wantCount {
				t.Errorf("AD-4: pass two wrote /Count %s for %d input pages; the declared value is %s",
					gotCount, row.inputPages, row.wantCount)
			}

			// Site 3: /Kids must reference exactly that many objects.
			kidsStart := bytes.Index(out, []byte("/Type /Pages /Kids ["))
			kidsEnd := bytes.Index(out[kidsStart:], []byte("] /Count "))
			kids := out[kidsStart:][:kidsEnd]
			gotKids := bytes.Count(kids, []byte(" 0 R"))
			if gotKids != row.wantKids {
				t.Errorf("AD-4: /Kids names %d page objects for %d input pages; the declared value is %d",
					gotKids, row.inputPages, row.wantKids)
			}
		})
	}
}

// TestPagesTreeKidsAreSeparated is the emitter-level guard for the defect
// Story 2.6 shipped into a recorded golden and the owner caught by opening
// the file.
//
// appendRef emits "N 0 R" with NO trailing space, which is correct at every
// other call site in this package: /Font and /XObject write " /"+name+" "
// before their ref, and the singular sites (/Parent, /Contents, the
// catalog's /Pages) are followed by a literal. /Kids is the ONLY site that
// emits two refs BACK TO BACK — so it is the only place the missing
// separator can manifest, and it could not manifest at all until a document
// had more than one page. Seven single-page goldens had passed over it.
//
// WHY THE ASSERTION IS ON TOKENIZATION AND NOT ON A SUBSTRING. "[8 0 R10 0
// R]" contains "8 0 R" and contains "10 0 R"; a substring search finds both
// and reports success. A PDF tokenizer reads "R10" as one unknown token and
// resolves NEITHER kid. So the check groups the array's tokens in THREES,
// which is what exposes a run-together pair: five tokens is not a multiple
// of three.
func TestPagesTreeKidsAreSeparated(t *testing.T) {
	for _, n := range []int{1, 2, 3, 7} {
		pages := make([]pagemodel.Page, n)
		for i := range pages {
			pages[i] = pagemodel.Page{Width: ptPageWidthMP, Height: ptPageHeightMP}
		}
		out, err := SerializeTextDocument(pages, nil, nil, nil)
		if err != nil {
			t.Fatalf("SerializeTextDocument(%d pages): %v", n, err)
		}

		open := bytes.Index(out, []byte("/Type /Pages /Kids ["))
		if open < 0 {
			t.Fatalf("%d pages: no page tree emitted", n)
		}
		body := out[open+len("/Type /Pages /Kids ["):]
		shut := bytes.IndexByte(body, ']')
		if shut < 0 {
			t.Fatalf("%d pages: the /Kids array is never closed", n)
		}
		fields := bytes.Fields(body[:shut])
		if len(fields) != 3*n {
			t.Errorf("%d pages: the /Kids array tokenizes into %d token(s) %q; a well-formed array of %d references has exactly %d.\n"+
				"A count that is not a multiple of three means two references have run together — the tokenizer "+
				"reads \"R10\" as one unknown token and NEITHER kid resolves, leaving an EMPTY page tree behind a "+
				"correct-looking /Count.", n, len(fields), fields, n, 3*n)
			continue
		}
		for i := 0; i < n; i++ {
			if string(fields[3*i+1]) != "0" || string(fields[3*i+2]) != "R" {
				t.Errorf("%d pages: reference %d is %q %q %q, not \"N 0 R\"", n, i, fields[3*i], fields[3*i+1], fields[3*i+2])
			}
		}

		// The one-kid spelling is UNCHANGED, which is what keeps all seven
		// single-page goldens from moving: the separator is LEADING, so it
		// is never emitted for a single kid.
		if n == 1 && !bytes.Contains(out, []byte("/Kids [")) {
			t.Error("the single-page /Kids spelling changed; every pre-2.6 golden would move")
		}
		if n == 1 && bytes.Contains(out, []byte("/Kids [ ")) {
			t.Error("a separator is emitted BEFORE the first kid; the single-page goldens have moved")
		}
	}
}
