package folio8

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/expr"
)

// TestLocaleTableVersionSurfacedAtLibraryLevel is Finding 9's fix,
// asserted (this story's QA review): AC6/AD-22 requires the locale
// table's version to be surfaced wherever the library version is
// surfaced. internal/expr.LocaleTableVersion is unexported-adjacent
// (it lives under internal/, unreachable from outside the module);
// this pins that package folio8's own exported LocaleTableVersion is
// defined as exactly that value, so a future edit to either constant
// in isolation reddens this test rather than silently drifting.
func TestLocaleTableVersionSurfacedAtLibraryLevel(t *testing.T) {
	if LocaleTableVersion != expr.LocaleTableVersion {
		t.Fatalf("folio8.LocaleTableVersion = %d, internal/expr.LocaleTableVersion = %d: must be defined as exactly the same value (AC6/AD-22)", LocaleTableVersion, expr.LocaleTableVersion)
	}
	if LocaleTableVersion < 1 {
		t.Fatalf("LocaleTableVersion = %d, want >= 1", LocaleTableVersion)
	}
}

// releasedVersionLine matches RELEASING.md's single "Released version" line,
// capturing the version without the directory prefix and the leading v.
var releasedVersionLine = regexp.MustCompile("(?m)^\\*\\*Released version:\\*\\* `folio-go/v([0-9]+\\.[0-9]+\\.[0-9]+)`\\s*$")

// TestVersionAgreesWithReleasingDoc holds the version stamp and the release
// procedure together: folio8.Version must equal the release RELEASING.md names,
// so a release commit cannot bump one and forget the other.
func TestVersionAgreesWithReleasingDoc(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "RELEASING.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the release procedure at %s: %v", path, err)
	}
	matches := releasedVersionLine.FindAllStringSubmatch(string(raw), -1)
	if len(matches) != 1 {
		t.Fatalf("%s must carry exactly one line of the form **Released version:** `folio-go/vX.Y.Z`; found %d", path, len(matches))
	}
	if doc := matches[0][1]; doc != Version {
		t.Fatalf("folio8.Version = %q, but RELEASING.md names folio-go/v%s as released: bump both in the release commit", Version, doc)
	}
}
