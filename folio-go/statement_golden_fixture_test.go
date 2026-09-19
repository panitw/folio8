package folio8

// Story 4.7, AC6 and AC10: the four recorded goldens, their inputs, and
// the coverage history each of them is the first to change.
//
// ORDER, because it is the part that is easy to get backwards and
// impossible to fix afterwards: every semantic assertion in
// statement_semantics_test.go was written and passing BEFORE any digest
// in this family was frozen. A hash records that bytes have not moved
// since somebody accepted them; it has nothing to say about whether the
// acceptance was sound, and it cannot acquire that ability later
// (D-000.22).

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// statementDigestRemedy is this file's own copy of the standing remedy
// wording (byte_neutrality_test.go's statementDigestRemedy lives in package
// folio8_test and is not reachable from package folio8). It is a value
// duplicate of a CONSTANT MESSAGE, not of a mechanism.
const statementDigestRemedy = "\nDO NOT UPDATE A DIGEST TO MAKE A TEST GO GREEN. A moved digest is a " +
	"versioned behaviour change (AD-22): find out what moved the bytes and why, decide whether that " +
	"change is wanted, and if it is, re-record deliberately — re-running the semantic acceptance step " +
	"(statement_semantics_test.go) and re-requesting the human sign-off, which the move has just " +
	"invalidated IN WHOLE across all four documents."

// TestStatementFixtureInputsMatchTheInRepoDefinitions keeps each fixture
// directory's three input documents byte-identical to the definitions
// the test binary renders — the discipline font-text, multi-page,
// three-band-page, wrapped-text and page-count-* all carry.
//
// It matters more here than in any of those, and for a reason specific
// to this family: the matrix legs render from the IN-BINARY definitions
// (js/wasm has no filesystem, and the Docker runners bind-mount only the
// prebuilt binaries directory), while a human reading the fixture to
// decide whether to sign it off reads the ON-DISK ones. If the two
// diverged, the human would be signing off a document nobody rendered.
func TestStatementFixtureInputsMatchTheInRepoDefinitions(t *testing.T) {
	root := repoRootFromTest(t)
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			dir := filepath.Join(root, "fixtures", f.slug)
			for _, c := range []struct{ name, want string }{
				{"input.folio", statementTemplateJSON},
				{"params.json", statementParamsJSON},
				{"data.json", statementDataJSON(f.rows)},
			} {
				path := filepath.Join(dir, c.name)
				onDisk, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("presence precondition: %s could not be read: %v", path, err)
					continue
				}
				if len(onDisk) == 0 {
					t.Errorf("presence precondition: %s is empty", path)
					continue
				}
				if string(onDisk) != c.want {
					t.Errorf("%s and the in-binary definition have DIVERGED (in-binary %d bytes, on disk %d bytes). The matrix legs render the in-binary form and a human reads the on-disk form; a divergence means the signed document and the rendered document are different files.",
						path, len(c.want), len(onDisk))
				}
			}
		})
	}
}

// TestStatementGoldenFixtures is AC6, observable (i): the render matches
// the committed golden byte for byte, and the artifact's OWN sha256
// equals the digest recorded in expected.json.
//
// The THIRD site — the second literal in goldenDigestRecord — is checked
// by TestGoldenDigestAgreesAtEveryDeclaredSite, which also re-hashes the
// artifact and enforces that no UNDECLARED site records the digest. This
// test is the per-fixture half; that one is the completeness half.
func TestStatementGoldenFixtures(t *testing.T) {
	root := repoRootFromTest(t)
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			dir := filepath.Join(root, "fixtures", f.slug)
			fixture := loadExpectedFixture(t, filepath.Join(dir, "expected.json"))

			golden, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
			if err != nil {
				t.Fatalf("presence precondition: %s/expected.pdf could not be read: %v", f.slug, err)
			}
			if len(golden) == 0 {
				t.Fatal("presence precondition: the golden is empty — two empty files are byte-identical")
			}
			sum := sha256.Sum256(golden)
			if got := hex.EncodeToString(sum[:]); got != fixture.SHA256 {
				t.Fatalf("fixtures/%s/expected.pdf hashes to %s, but expected.json records %s.\n%s", f.slug, got, fixture.SHA256, statementDigestRemedy)
			}

			produced := renderStatement(t, f)
			if string(produced) != string(golden) {
				t.Errorf("fixtures/%s: the render is %d bytes and the committed golden is %d bytes, and they are NOT identical.\n%s",
					f.slug, len(produced), len(golden), statementDigestRemedy)
			}
			t.Logf("%s: %d bytes, sha256 %s, %d pages", f.slug, len(golden), fixture.SHA256, f.pages)
		})
	}
}

// TestStatementFixtureReadmesRecordTheCoverageHistory is AC10, and it is
// the one assertion in this story that guards a WRITTEN record rather
// than a mechanism. AC10 declares ZERO machine observables; this test
// exists because the record is cheap to check for presence and because a
// README that silently lost the sentence would leave a future reader at a
// gate re-deriving the whole measurement.
//
// What each README must record, in writing:
//   - that NO committed golden contained a table before this story, and
//   - that Story 4.6's unconditional-clip mutation reddened NO GOLDEN AT
//     ALL while reddening the table behaviour suite.
//
// The second is the sharper of the two: it is the direct measurement
// that the golden corpus could not express a table defect, made by the
// story immediately before this one.
func TestStatementFixtureReadmesRecordTheCoverageHistory(t *testing.T) {
	root := repoRootFromTest(t)
	required := []string{
		"no committed golden contained a table",
		"reddened no golden at all",
	}
	for _, f := range statementFixtures {
		t.Run(f.slug, func(t *testing.T) {
			path := filepath.Join(root, "fixtures", f.slug, "README.md")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("presence precondition: %s could not be read: %v", path, err)
			}
			// Whitespace-normalised, because these sentences are
			// PROSE: a required phrase that happens to fall across a
			// line wrap is still recorded, and a guard that missed it
			// would be checking the paragraph's line breaks rather than
			// what it says.
			lower := strings.Join(strings.Fields(strings.ToLower(string(body))), " ")
			for _, want := range required {
				if !strings.Contains(lower, want) {
					t.Errorf("%s does not record %q. AC10 is a written record and this is the only thing standing between it and quiet deletion.", path, want)
				}
			}
		})
	}
}

// statementSignOffRelPath is the record's home. It sits at the fixtures/
// ROOT rather than inside any one fixture directory because it spans all
// four — putting it in statement-1/ would imply the other three were
// signed off somewhere else.
const statementSignOffRelPath = "fixtures/statement-signoff.json"

// statementSignOff is the record's schema. Every field is required and
// every field is load-bearing.
type statementSignOff struct {
	// Reader is the human who looked. A name, not a role and not an
	// agent: D-000.22's whole point is that this is the one claim no
	// machine and no agent can make on someone else's behalf.
	Reader string `json:"reader"`
	// Date is when they looked, ISO-8601 (YYYY-MM-DD).
	Date string `json:"date"`
	// Examined is what they actually looked at and what they saw. The
	// reviewer READS this field: the schema's non-empty check cannot
	// tell "looked fine" from a real examination, and a record saying
	// "looked fine" is a defect.
	Examined string `json:"examined"`
	// Digests maps each statement fixture's slug to the sha256 that
	// fixture's expected.pdf had when it was read. ALL FOUR are named,
	// and any one of them moving invalidates the WHOLE record.
	Digests map[string]string `json:"digests"`
}

// statementSignOffStaleness is the PURE comparison behind AC8's second
// observable — the digest binding that makes a re-record invalidate the
// human sign-off.
//
// It is a pure function of (record, live digests) so that it can be
// red-proved in the ORDINARY suite against SYNTHETIC records, without a
// real sign-off existing and without anybody fabricating one. D-000.28
// forbids writing a claim before the event it asserts; a red-proof over
// obviously-synthetic inputs asserts nothing about any human.
//
// INVALIDATION IS ALL-OR-NOTHING (the engineering lead's ruling on this
// story): over-invalidating costs a re-read, under-invalidating ships an
// attestation nobody made. So `moved` being non-empty invalidates the
// WHOLE record, and a record whose named SET of documents differs from
// the registered set is rejected outright rather than partially honoured.
func statementSignOffStaleness(rec statementSignOff, live map[string]string) (scopeMismatch bool, gotSlugs []string, moved []string) {
	for slug := range rec.Digests {
		gotSlugs = append(gotSlugs, slug)
	}
	sort.Strings(gotSlugs)

	var wantSlugs []string
	for slug := range live {
		wantSlugs = append(wantSlugs, slug)
	}
	sort.Strings(wantSlugs)

	if strings.Join(gotSlugs, ",") != strings.Join(wantSlugs, ",") {
		return true, gotSlugs, nil
	}
	for _, slug := range wantSlugs {
		if rec.Digests[slug] != live[slug] {
			moved = append(moved, slug+": signed "+rec.Digests[slug]+", now "+live[slug])
		}
	}
	return false, gotSlugs, moved
}

// statementSignOffFieldProblems is AC8 observable (i)'s MECHANICAL HALF,
// extracted here as a pure function for one reason: as first written it
// lived inline inside the matrix-gated gate, BELOW the os.IsNotExist
// branch that t.Fatalf's and returns. The record does not exist and must
// not, so not one of those checks had ever executed, and an inverted one
// would have left every test in the tree green (this story's review,
// Finding 3). A guard that has never run is not a guard.
//
// It returns one problem string per empty required field, in a fixed
// order, so the red-proof below can assert WHICH check fired rather than
// only that something did.
//
// What it can and cannot see, stated because AC8(i)'s wording is
// compound: it witnesses PRESENCE and NON-EMPTINESS. It cannot witness
// SPECIFICITY — "looked fine" passes every one of these checks, and the
// only thing that catches it is the reviewer reading the `examined`
// field. That half is not machine-checkable at all, which is why the
// ledger records AC8(i) as a compound observable.
func statementSignOffFieldProblems(rec statementSignOff) []string {
	var problems []string
	for _, f := range []struct{ name, value string }{
		{"reader", rec.Reader},
		{"date", rec.Date},
		{"examined", rec.Examined},
	} {
		if strings.TrimSpace(f.value) == "" {
			problems = append(problems, "empty "+f.name)
		}
	}
	if len(rec.Digests) == 0 {
		problems = append(problems, "no digests")
	}
	return problems
}

// statementSignOffGateVerdict is the WRAPPER that turns
// statementSignOffStaleness' verdict into the gate's failure, as a pure
// function of (record, live digests, registered slugs).
//
// It lives here, in the untagged file, for the same reason
// statementSignOffStaleness does: a wrapper that only ever runs behind
// the matrix tag has no way to be shown going red, and its inversion
// (`if len(moved) == 0`) would have been invisible. The matrix gate's
// job is now reduced to reading the file, hashing the four artifacts,
// and printing whatever this returns.
func statementSignOffGateVerdict(rec statementSignOff, live map[string]string, wantSlugs []string) (msg string, failed bool) {
	scopeMismatch, gotSlugs, moved := statementSignOffStaleness(rec, live)
	if scopeMismatch {
		sorted := append([]string(nil), wantSlugs...)
		sort.Strings(sorted)
		return fmt.Sprintf(
			"%s names digests for %v, but the registered statement documents are %v.\n\n"+
				"ONE RECORD COVERS ALL FOUR, and it is invalidated IN WHOLE. A record that names three of "+
				"four documents is not three-quarters of a sign-off — it is a record whose scope no longer "+
				"matches what it claims to certify.",
			statementSignOffRelPath, gotSlugs, sorted,
		), true
	}
	if len(moved) != 0 {
		return fmt.Sprintf(
			"the statement sign-off in %s is STALE. %d of the %d documents have been RE-RECORDED since it "+
				"was signed:\n  %s\n\n"+
				"THE WHOLE RECORD IS INVALIDATED, not just the entries that moved, and that is deliberate: "+
				"a change that moves one of these documents is nearly always evidence about all four, and "+
				"three surviving attestations over a systemic change is exactly the stale-provenance defect "+
				"this binding exists to close.\n\n"+
				"Re-read the documents and update BOTH what was examined AND all four digests. Do NOT "+
				"simply paste the new digests in.\n\n"+
				"%s",
			statementSignOffRelPath, len(moved), len(wantSlugs), strings.Join(moved, "\n  "),
			statementSignOffReadingRemedy,
		), true
	}
	return "", false
}

// statementSignOffReadingRemedy is D-4.7.7, and it is carried in the
// STALE-record message because that is the message a re-attestation is
// read against.
//
// A text extractor was measured on exactly this document's shape during
// this story and returned confident garbage — mpParseToUnicode merges
// every embedded face's /ToUnicode section into one CID map, so
// "Customer: Ada Lovelace" came back as "ไustomerะบdaบศovelace". A
// sign-off performed against a broken extractor would be WORSE THAN
// NONE, because it would carry a name.
const statementSignOffReadingRemedy = "HOW THE TEXT CHECKS MUST BE PERFORMED, and this is not a style " +
	"preference (D-4.7.7): LOOK AT THE RENDERED PAGES. Open the PDF in a viewer and read what is drawn. " +
	"Do NOT check the Thai, the Chinese, the totals or the headers by extracting text — not with " +
	"pdftotext, not with a copy-paste out of a viewer, and not with any of this repository's own " +
	"instruments. This document embeds THREE faces, and a text extractor that merges their /ToUnicode " +
	"CMaps returns plausible-looking, confidently-wrong text on exactly this shape: measured during " +
	"Story 4.7, \"Customer: Ada Lovelace\" came back as \"ไustomerะบdaบศovelace\". A sign-off performed " +
	"against a broken extractor is worse than no sign-off at all, because it carries a name."

// TestStatementSignOffFieldChecksAndGateWrapperRedProof is AC8 observable
// (i)'s witness, and the answer to this story's review Finding 3: the
// mechanical half of AC8(i) had shipped unexecuted and unproved, while
// its sibling AC8(ii) had a synthetic red-proof by exactly the technique
// used here.
//
// Everything below is driven over OBVIOUSLY SYNTHETIC records. D-000.28
// forbids writing a claim before the event it asserts; a synthetic record
// asserts nothing about any human, and no file is written.
func TestStatementSignOffFieldChecksAndGateWrapperRedProof(t *testing.T) {
	live := map[string]string{}
	var wantSlugs []string
	for _, f := range statementFixtures {
		live[f.slug] = "aa" + f.slug
		wantSlugs = append(wantSlugs, f.slug)
	}
	populated := func() statementSignOff {
		d := map[string]string{}
		for k, v := range live {
			d[k] = v
		}
		return statementSignOff{Reader: "synthetic reader", Date: "2026-01-01", Examined: "synthetic examination", Digests: d}
	}

	// The control FIRST: a fully populated record must produce NO
	// problems. Without it, a field check that rejected everything would
	// satisfy every case below.
	t.Run("control: a fully populated record raises no field problem", func(t *testing.T) {
		if got := statementSignOffFieldProblems(populated()); len(got) != 0 {
			t.Fatalf("the populated control was rejected with %v — every case below would then pass for the wrong reason", got)
		}
	})

	for _, c := range []struct {
		name    string
		mutate  func(*statementSignOff)
		problem string
	}{
		{"an empty reader is refused", func(r *statementSignOff) { r.Reader = "" }, "empty reader"},
		{"a whitespace-only date is refused", func(r *statementSignOff) { r.Date = "   \t " }, "empty date"},
		{"an empty examined is refused", func(r *statementSignOff) { r.Examined = "" }, "empty examined"},
		{"a record naming no digests at all is refused", func(r *statementSignOff) { r.Digests = nil }, "no digests"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := populated()
			c.mutate(&rec)
			got := statementSignOffFieldProblems(rec)
			found := false
			for _, p := range got {
				if p == c.problem {
					found = true
				}
			}
			if !found {
				t.Errorf("the field checks reported %v, which does not include %q — an unattributed, undated or unspecific sign-off would be indistinguishable from no sign-off at all", got, c.problem)
			}
			if len(got) != 1 {
				t.Errorf("the field checks reported %v (%d problems), want exactly the one that was mutated — a check that fires on everything witnesses nothing", got, len(got))
			}
		})
	}

	// ...and the WRAPPER's two failure paths, over a synthetic live map.
	// This is what makes an inverted verdict (`if len(moved) == 0`)
	// visible: before this test, the wrapper had never executed.
	t.Run("wrapper control: a current, in-scope record PASSES the gate", func(t *testing.T) {
		msg, failed := statementSignOffGateVerdict(populated(), live, wantSlugs)
		if failed {
			t.Fatalf("the gate wrapper rejected a current, in-scope record:\n%s", msg)
		}
	})
	t.Run("wrapper: ONE moved digest fails the gate as STALE, in whole", func(t *testing.T) {
		rec := populated()
		rec.Digests["statement-20"] = "0000000000000000000000000000000000000000000000000000000000000000"
		msg, failed := statementSignOffGateVerdict(rec, live, wantSlugs)
		if !failed {
			t.Fatal("moving one of four digests did NOT fail the gate — the wrapper's verdict is inverted or unreachable")
		}
		for _, want := range []string{"STALE", "statement-20", "1 of the 4 documents", "LOOK AT THE RENDERED PAGES"} {
			if !strings.Contains(msg, want) {
				t.Errorf("the stale-record failure message does not contain %q:\n%s", want, msg)
			}
		}
	})
	t.Run("wrapper: an out-of-scope record fails the gate as a SCOPE MISMATCH", func(t *testing.T) {
		rec := populated()
		delete(rec.Digests, "statement-50")
		msg, failed := statementSignOffGateVerdict(rec, live, wantSlugs)
		if !failed {
			t.Fatal("a record naming three of four documents did NOT fail the gate")
		}
		if !strings.Contains(msg, "ONE RECORD COVERS ALL FOUR") || strings.Contains(msg, "STALE") {
			t.Errorf("a scope mismatch must be reported as a scope mismatch, not as staleness:\n%s", msg)
		}
	})
}

// TestStatementSignOffDigestBindingRedProof is AC8 observable (ii)'s
// witness, and it is the ONLY way that observable can be witnessed
// before a real sign-off exists.
//
// The four cases are the four things the binding must do, and the FIRST
// one is what makes the other three mean anything: a record naming the
// current digests must be accepted, or "reddens on a moved digest" would
// be satisfied by a function that reddens on everything.
func TestStatementSignOffDigestBindingRedProof(t *testing.T) {
	live := map[string]string{}
	for _, f := range statementFixtures {
		live[f.slug] = "aa" + f.slug
	}
	current := func() statementSignOff {
		d := map[string]string{}
		for k, v := range live {
			d[k] = v
		}
		return statementSignOff{Reader: "synthetic", Date: "2026-01-01", Examined: "synthetic", Digests: d}
	}

	t.Run("control: a record naming every current digest is accepted", func(t *testing.T) {
		mismatch, _, moved := statementSignOffStaleness(current(), live)
		if mismatch || len(moved) != 0 {
			t.Fatalf("the unmutated control was rejected (scopeMismatch=%v, moved=%v) — every case below would then pass for the wrong reason", mismatch, moved)
		}
	})

	t.Run("ONE moved digest invalidates the record", func(t *testing.T) {
		rec := current()
		rec.Digests["statement-20"] = "0000000000000000000000000000000000000000000000000000000000000000"
		mismatch, _, moved := statementSignOffStaleness(rec, live)
		if mismatch {
			t.Fatal("a moved digest must be reported as STALE, not as a scope mismatch")
		}
		if len(moved) != 1 {
			t.Fatalf("moving ONE of four digests reported %d moved entr(ies), want 1 — and it must invalidate the WHOLE record", len(moved))
		}
		if !strings.Contains(moved[0], "statement-20") {
			t.Errorf("the staleness report does not name the document that moved: %q", moved[0])
		}
	})

	t.Run("a record naming only three of the four documents is rejected outright", func(t *testing.T) {
		rec := current()
		delete(rec.Digests, "statement-50")
		mismatch, got, _ := statementSignOffStaleness(rec, live)
		if !mismatch {
			t.Fatalf("a record covering only %v was accepted against the registered set — three quarters of a sign-off is not a sign-off", got)
		}
	})

	t.Run("a record naming a document that is not registered is rejected outright", func(t *testing.T) {
		rec := current()
		rec.Digests["statement-999"] = "beef"
		mismatch, _, _ := statementSignOffStaleness(rec, live)
		if !mismatch {
			t.Fatal("a record naming an unregistered document was accepted")
		}
	})
}
