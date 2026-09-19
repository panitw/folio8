package folio8

// THE STRUCTURAL VALIDITY ORACLE, and why it lives in package folio8's own
// test files rather than beside the golden test that first needed it.
//
// It was written for TestEveryGoldenPDFResolvesItsPageTree
// (golden_structural_validity_test.go, package folio8_test), after a
// committed golden passed its hash AND its whole semantic acceptance step
// while not being a PDF at all — read that file's header for the full
// account; it is the better half of this one.
//
// Story 4.6 moved it here, unchanged in substance, because the story's AC1
// needs the SAME oracle applied to bytes produced IN MEMORY by a test that
// must live in package folio8 (it reads tableRectSource/textRunSource
// identity fields the external test package cannot see). Two copies of a
// validity oracle is exactly the "second producer" this repository refuses
// everywhere else, so there is one, exported so both test packages reach
// it. Exported here means exported into the TEST BINARY only — nothing in
// this file ships in the library.

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"
)

// kidsArrayRE captures the whole /Kids array body of the page tree.
var kidsArrayRE = regexp.MustCompile(`/Type /Pages /Kids \[([^\]]*)\] /Count (\d+)`)

// wellFormedRefRE matches ONE indirect reference, anchored, so a run-together
// pair like "8 0 R10 0 R" cannot match as a single reference.
var wellFormedRefRE = regexp.MustCompile(`^(\d+) 0 R$`)

// definedPageObjRE captures the object number of every "N 0 obj\n<< /Type
// /Page /Parent " definition — used to build the SET of defined page-object
// numbers for step (4)'s orphan check (finisher Finding 4: a COUNT of these
// occurrences cannot distinguish "every defined page is referenced" from
// "the counts happen to agree while one definition sits outside the
// referenced set and one reference is duplicated").
var definedPageObjRE = regexp.MustCompile(`(\d+) 0 obj\n<< /Type /Page /Parent `)

// AssertPDFPageTreeResolves is TestEveryGoldenPDFResolvesItsPageTree's own
// structural check, EXTRACTED (Story 4.6) so that a PDF produced in-memory
// by a test can be held to the same standard as a committed golden without
// a second, drifting implementation of it. `label` names the subject in
// failure messages — a golden's path, or a description of the render that
// produced the bytes.
//
// It reports whether THIS CHECK passed, so a caller that wants to count
// what it checked reads the check's OWN answer rather than assuming one.
//
// "This check" is meant strictly, and it is why the failures below are
// recorded in a local flag rather than read back off t.Failed() (this
// story's reviewer, Finding 12). t.Failed() is true if ANYTHING in the
// calling (sub)test has already failed, so a caller with an earlier
// unrelated non-fatal failure would be told this document's page tree is
// invalid when it is fine — a false verdict from the one helper that is
// deliberately extracted to be the single shared oracle. Harmless at both
// of today's call sites, each preceded only by t.Fatalf; wrong the moment
// a third arrives.
func AssertPDFPageTreeResolves(t *testing.T, b []byte, label string) bool {
	t.Helper()
	failed := false
	errorf := func(format string, args ...any) {
		t.Helper()
		failed = true
		t.Errorf(format, args...)
	}
	path := label
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatalf("presence precondition: %s does not begin with %%PDF-", path)
	}

	// (1) The page tree.
	m := kidsArrayRE.FindSubmatch(b)
	if m == nil {
		t.Fatalf("no page tree of the form \"/Type /Pages /Kids [...] /Count N\" found in %s — the document has no page tree at all", path)
	}
	kidsBody := string(m[1])
	declaredCount, cerr := strconv.Atoi(string(m[2]))
	if cerr != nil || declaredCount < 1 {
		t.Fatalf("the page tree declares /Count %q, which is not a positive integer", m[2])
	}

	// (2) THE ASSERTION THAT WOULD HAVE CAUGHT THE BUG. Split the
	// array on whitespace-separated reference triples and require
	// each to be well formed. A missing separator collapses two
	// references into one unparseable token.
	refs := splitIndirectRefs(kidsBody)
	if len(refs) != declaredCount {
		t.Fatalf("the page tree declares /Count %d but its /Kids array yields %d reference(s): %q\n\n"+
			"This is the failure mode a hash CANNOT see and a page-object COUNT cannot see: both page "+
			"objects may exist and be correct while the array pointing at them is unparseable. "+
			"\"[8 0 R10 0 R]\" tokenizes as one unknown token, so NEITHER kid resolves and the page tree "+
			"is EMPTY — the /Count is a claim about the kids, not a fact derived from them.",
			declaredCount, len(refs), kidsBody)
	}
	for _, ref := range refs {
		if !wellFormedRefRE.MatchString(ref) {
			errorf("the /Kids array contains %q, which is not a well-formed indirect reference (\"N 0 R\")", ref)
		}
	}
	if failed {
		return false
	}

	// (3) Every reference resolves to a defined page object, AND
	// the referenced object numbers are pairwise DISTINCT.
	//
	// Distinctness is asserted here rather than left to (4)'s
	// count comparison, per the finisher's Finding 4: "[8 0 R 8 0
	// R]", with objects 8 and 10 both defined as /Type /Page, has
	// len(refs) == declaredCount == 2 and every reference resolves
	// — object 10 is a defined page object nothing references,
	// and page 2 is a silent duplicate of page 1. A COUNT
	// comparison cannot see this; a SET can.
	refNums := map[string]bool{}
	for _, ref := range refs {
		num := wellFormedRefRE.FindStringSubmatch(ref)[1]
		if refNums[num] {
			errorf("the /Kids array references object %s more than once — a duplicated reference is how an orphaned page object stays invisible to a COUNT of referenced objects, even though this reference itself resolves fine", num)
			continue
		}
		refNums[num] = true

		def := []byte("\n" + num + " 0 obj\n")
		idx := bytes.Index(b, def)
		if idx < 0 {
			errorf("the page tree references object %s, which is never defined in the file — a syntactically valid reference to a non-existent object is still an empty page tree", num)
			continue
		}
		rest := b[idx+len(def):]
		end := bytes.Index(rest, []byte("\nendobj"))
		if end < 0 {
			errorf("object %s has no endobj", num)
			continue
		}
		if !bytes.Contains(rest[:end], []byte("/Type /Page ")) {
			errorf("the page tree references object %s, which is not a /Type /Page", num)
		}
	}
	if failed {
		return false
	}

	// (4) No page object is orphaned — compared as a SET against
	// the SET of definitions, not as a count against a count
	// (finisher Finding 4). bytes.Count("<< /Type /Page /Parent
	// ") == declaredCount is satisfied by [8 0 R 8 0 R] with
	// objects 8 AND 10 both defined: the count matches while
	// object 10 sits outside the referenced set entirely.
	definedNums := map[string]bool{}
	for _, m := range definedPageObjRE.FindAllSubmatch(b, -1) {
		definedNums[string(m[1])] = true
	}
	for num := range definedNums {
		if !refNums[num] {
			errorf("object %s is defined as a /Type /Page but the page tree's /Kids array never references it — a page object that exists and is unreachable is invisible to a reader and to a hash alike, even when the page-object COUNT agrees with /Count", num)
		}
	}
	if len(definedNums) != len(refNums) {
		errorf("the file defines %d distinct /Type /Page object(s) but the page tree references %d distinct one(s)", len(definedNums), len(refNums))
	}
	return !failed
}

// splitIndirectRefs splits a /Kids array body into its whitespace-delimited
// reference triples: "8 0 R 10 0 R" -> ["8 0 R", "10 0 R"].
//
// It groups tokens in THREES rather than searching for "R", because grouping
// is what exposes the defect: "8 0 R10 0 R" tokenizes as
// ["8", "0", "R10", "0", "R"] — five tokens, which is not a multiple of
// three, and whose first triple "8 0 R10" is not a well-formed reference.
// A search for "R" would happily find two of them and report success.
func splitIndirectRefs(body string) []string {
	fields := bytes.Fields([]byte(body))
	var out []string
	for i := 0; i+2 < len(fields); i += 3 {
		out = append(out, string(fields[i])+" "+string(fields[i+1])+" "+string(fields[i+2]))
	}
	// A trailing partial group is itself a malformation; surface it as an
	// extra (unparseable) entry rather than discarding it, so the count
	// comparison above fails loudly instead of silently rounding down.
	if rem := len(fields) % 3; rem != 0 {
		var tail string
		for _, f := range fields[len(fields)-rem:] {
			tail += string(f) + " "
		}
		out = append(out, tail)
	}
	return out
}
