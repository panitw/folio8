package rules

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoRealFontHidesUnderAnExcludedPath is DW-123's tripwire and
// D-8.4i.4's stated discharge.
//
// THE HOLE IT WATCHES. manifest.ResolveAssets — the AD-26 asset gate
// that requires every committed font binary to travel with its licence
// text and copyright line — SkipDirs any directory named "lint" whose
// immediate parent is named "testdata". The exclusion is deliberate and
// correct: those are lint's own evading-shape fixtures, .ttf-extensioned
// files with no real font content, which exist to test detection and are
// not redistributed. But folio-go/testdata/ SHIPS INSIDE THE MODULE ZIP,
// so a REAL font binary placed there would be redistributed under AD-26
// with NO manifest row and nothing said.
//
// Today's occupant is a text stub. "Today's occupant is a stub" is
// exactly the kind of fact that stops being true silently, and a
// register entry keyed on nobody adding a real font is not a defence
// (D-8.4i.4).
//
// KEYED ON MAGIC BYTES, NEVER ON THE EXTENSION. Extension-keyed is the
// blind spot D-8.5.2 already found in this area, and the fixtures under
// the excluded path are themselves proof that an extension is a claim
// rather than a fact — all three .ttf files there begin "not a real
// font; extension-matching fixt…".
//
// NO SECOND SFNT READER IS INTRODUCED (D-8.4i.4, explicit prohibition).
// D-8.4.12's checkSfnt is at folio-go/internal/template/fontasset.go and
// is genuinely unreachable from here — unexported, and in a different Go
// module with no go.work and no root go.mod. But the lint module already
// owns an sfnt reader: looksLikeSfnt, in this very package. This test
// declares `package rules` and calls it AS-IS, with no production change
// and no reader written. Its coverage is FOUR magics — 00 01 00 00,
// "true", "ttcf", "OTTO" — and NOT wOFF or wOF2; that is stated rather
// than implied, because a web font placed under an excluded path would
// slip past this tripwire.
func TestNoRealFontHidesUnderAnExcludedPath(t *testing.T) {
	root := repoRootFromTest(t)
	childName, parentName := assetWalkStructuralExclusion(t, root)

	// Walk the whole repository looking for the EXCLUDED directories —
	// the mirror image of the gate's own walk.
	var excludedDirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Name() == childName && filepath.Base(filepath.Dir(path)) == parentName {
			excludedDirs = append(excludedDirs, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(excludedDirs) == 0 {
		t.Fatalf("found no directory named %q under a parent named %q — the gate's exclusion covers "+
			"nothing, so this tripwire would be vacuous", childName, parentName)
	}

	scanned := 0
	for _, dir := range excludedDirs {
		walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, readErr := readFirstBytes(path, 12)
			if readErr != nil {
				return readErr
			}
			scanned++
			if looksLikeSfnt(data) {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				t.Errorf("%s: a REAL font program (sfnt magic number) is committed under a path the AD-26 "+
					"asset gate excludes from its walk (manifest.go's %q-under-%q SkipDir). folio-go/testdata "+
					"ships inside the module zip, so this font is REDISTRIBUTED with no licence row in "+
					"lint/MANIFEST.md and nothing said (DW-123, D-8.4i.4). Either move it somewhere the gate "+
					"assesses, or replace it with a non-font stub as the fixtures beside it are — do NOT "+
					"widen the exclusion or narrow this test.", rel, childName, parentName)
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk excluded dir %s: %v", dir, walkErr)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no files under any excluded path — this tripwire would be vacuous")
	}
	t.Logf("tripwire scanned %d file(s) across %d excluded director(ies)", scanned, len(excludedDirs))

	// NON-VACUITY CONTROL. A tripwire that never fires is
	// indistinguishable from a tripwire that cannot fire, so the same
	// detector is run over a REAL committed font and must say yes. If
	// this arm ever goes quiet, the arm above proves nothing.
	const realFont = "folio-go/fonts/notosans/NotoSans-Regular.ttf"
	control, err := readFirstBytes(filepath.Join(root, filepath.FromSlash(realFont)), 12)
	if err != nil {
		t.Fatalf("read control font %s: %v", realFont, err)
	}
	// The control also proves readFirstBytes did not come back SHORT on
	// a file that has the bytes — the fail-open direction P3 closed.
	if len(control) != 12 {
		t.Fatalf("readFirstBytes(%s, 12) returned %d bytes; a short read on a file this size means the "+
			"tripwire's detector is being fed truncated input and every clean result above is worthless",
			realFont, len(control))
	}
	if !looksLikeSfnt(control) {
		t.Fatalf("%s does not read as an sfnt program — the detector this tripwire keys on is not "+
			"working, so the clean result above is not a measurement", realFont)
	}
}

// assetWalkStructuralExclusion DERIVES the asset gate's structural
// exclusion from manifest.go's own source, rather than restating it as a
// literal here.
//
// This is DW-119's lesson applied before the defect is committed a
// second time: the designer guard hand-copied these same two SkipDir
// rules into TypeScript literals with nothing tying them back to
// manifest.go, so the two could drift apart silently. The precedent for
// doing it this way is in that same file — licenceGateFontExtensions
// parses `var fontExtensions` out of manifest.go source precisely so the
// guard agrees with the gate rather than with a copy of the gate.
//
// If manifest.go's exclusion changes shape, this extraction reds and the
// tripwire is re-derived deliberately — which is the whole point. It
// must not be "fixed" by pasting the literals in.
func assetWalkStructuralExclusion(t *testing.T, root string) (childName, parentName string) {
	t.Helper()
	const rel = "lint/internal/manifest/manifest.go"
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read the gate's own source %s: %v", rel, err)
	}
	// The gate's rule is STRUCTURAL, not a glob: a directory whose own
	// name is X and whose IMMEDIATE PARENT is named Y.
	re := regexp.MustCompile(`d\.Name\(\) == "([^"]+)" && filepath\.Base\(filepath\.Dir\(path\)\) == "([^"]+)"`)
	m := re.FindAllStringSubmatch(string(src), -1)
	if len(m) != 1 {
		t.Fatalf("expected exactly ONE structural directory exclusion in %s, found %d — the gate's walk has "+
			"changed shape, so this tripwire must be re-derived from it deliberately rather than repaired "+
			"by restating the rule here (DW-119)", rel, len(m))
	}
	if !strings.Contains(string(src), `d.Name() == ".git"`) {
		t.Fatalf("%s no longer skips .git; the walk this tripwire mirrors has changed", rel)
	}
	return m[0][1], m[0][2]
}

// readFirstBytes reads up to n bytes from path, retrying until the file
// is exhausted or n bytes are in hand.
//
// THE SINGLE-Read VERSION OF THIS FUNCTION FAILED OPEN, and it did so in
// DW-123's own recorded shape. A single (*os.File).Read may legally
// return fewer bytes than the buffer holds even for a regular file that
// has more to give; the first draft used buf[:read] regardless, so a
// REAL font could come back as three bytes, looksLikeSfnt would say no,
// and the tripwire would report the tree clean. The entry this test
// discharges records exactly that defect on the designer side — "compares
// a four-byte buffer without checking how many bytes readSync actually
// returned" — so reintroducing it in the discharging test would have been
// the finding committed a second time.
//
// io.ReadFull's short-read errors are NOT failures here: a file genuinely
// shorter than n bytes is a legitimate answer, and it is returned SHORT
// rather than zero-padded. That distinction matters in the other
// direction — a zero-padded buffer would match the TrueType magic
// 00 01 00 00 and report a FALSE font.
func readFirstBytes(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buf[:read], nil
}
