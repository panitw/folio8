package manifest

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// releasingRelPath is the release procedure, repo-relative. This literal
// is the test's ANCHOR (D-000.68): a document at the repository root,
// authored outside both Go modules, which no code under test can move or
// rename. The manifest path it must agree with is NOT a second literal —
// it is CommittedRelPath, the single declaration that TestManifestUpToDate
// also reads, so the compiler keeps the two sides pointing at one string.
const releasingRelPath = "RELEASING.md"

// manifestPathPattern matches any repo-relative path ending in
// MANIFEST.md that RELEASING.md mentions, in backticks or bare.
var manifestPathPattern = regexp.MustCompile(`[A-Za-z0-9_./-]*MANIFEST\.md`)

// releasedVersionLine matches RELEASING.md's "Released version" line.
var releasedVersionLine = regexp.MustCompile("(?m)^\\*\\*Released version:\\*\\* `folio-go/v[0-9]+\\.[0-9]+\\.[0-9]+`\\s*$")

// TestReleasingDocNamesTheGuardedManifest closes DW-3 by holding its two
// halves together.
//
// WHY THIS EXISTS. DW-3 deferred "publish the licence manifest as a
// release artifact" to an owner that was written as two moments —
// "Epic 4 close" and "the first folio-go tag" — which were the same
// moment when written and are now three epics apart (D-000.78). The
// engineering lead retired the entry rather than picking one: AD-26's
// substance shipped at Story 1.3 and is guarded live by
// TestManifestUpToDate, so what remained was a line item in a release
// procedure that did not exist. Leaving it under "## Open" asserted that
// something remained to be BUILT, and a record that overstates is a
// defect even when it errs toward caution (D-000.49) — this one spent
// attention at three consecutive gates.
//
// Discharge by replacement (D-000.59): the obligation moved into
// RELEASING.md, where the story that cuts the tag must necessarily read
// it, instead of a backlog that story has no reason to open. This test is
// what stops the two drifting: rename the manifest and RELEASING.md still
// names the old path; move the procedure and this reddens.
//
// It asserts three things, and the middle one is the reason it is not
// merely decorative:
//  1. RELEASING.md exists and names at least one MANIFEST.md path.
//  2. Every such path it names IS CommittedRelPath — not merely "a path
//     that exists", so a second, stale manifest reference cannot hide
//     behind a correct one.
//  3. That file is actually on disk.
func TestReleasingDocNamesTheGuardedManifest(t *testing.T) {
	root := repoRootFromTest(t)
	docPath := filepath.Join(root, releasingRelPath)

	// D-000.9: a procedure that cannot be READ must never report as a
	// procedure that agrees. Fatal, never a skip.
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read the release procedure at %s: %v — DW-3's obligation lives in that document; if it moved, update releasingRelPath deliberately rather than letting the obligation go unheld", docPath, err)
	}

	found := manifestPathPattern.FindAllString(string(raw), -1)

	// VACUITY GUARD. An extractor that matches nothing reports the same
	// "every path agrees" all-clear a healthy one does when the document
	// genuinely names the manifest. Zero is a FAILURE, never a pass.
	if len(found) == 0 {
		t.Fatalf("%s names no MANIFEST.md path at all — DW-3 was retired on the strength of that document carrying the obligation (D-000.59, discharge by replacement). A release procedure that has stopped naming the manifest has silently un-retired it", docPath)
	}

	for _, got := range found {
		if got != CommittedRelPath {
			t.Errorf("%s names manifest path %q, but the single declaration says %q (manifest.CommittedRelPath).\n"+
				"One of the two moved. The declaration is authoritative — TestManifestUpToDate reads it too — so fix the document, or move the constant and let both follow.",
				releasingRelPath, got, CommittedRelPath)
		}
	}

	// And the path both sides agree on must actually be a file. Agreement
	// on a path that does not exist is agreement about nothing.
	manifestPath := filepath.Join(root, filepath.FromSlash(CommittedRelPath))
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("%s and manifest.CommittedRelPath agree on %q, but nothing is there: %v", releasingRelPath, CommittedRelPath, err)
	}

	// A cheap sanity check that the document is the release procedure and
	// not some other file that happens to mention a manifest: it names the
	// tag it releases on its single "Released version" line, which
	// folio-go's TestVersionAgreesWithReleasingDoc also reads, so a release
	// bumps that line and nothing here.
	if n := len(releasedVersionLine.FindAllString(string(raw), -1)); n != 1 {
		t.Errorf("%s carries %d lines of the form **Released version:** `folio-go/vX.Y.Z`, want exactly 1 — this test assumes that document is the release procedure for that tag", releasingRelPath, n)
	}
}
