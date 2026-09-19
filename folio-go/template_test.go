package folio8

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// minimalTemplateJSON is a well-formed `.folio` document (AC4): a
// version, page setup, and ordered (empty) band content.
const minimalTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": []
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestParseTemplate is AC4/AC1: a well-formed `.folio` file parses with
// no error via ParseTemplate.
func TestParseTemplate(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	if tpl == nil {
		t.Fatal("ParseTemplate returned a nil *Template with no error")
	}
	if tpl.doc.Version != "1.0" {
		t.Fatalf("Version = %q, want 1.0", tpl.doc.Version)
	}
}

// TestLoadTemplate is AC1/AC4: LoadTemplate reads a path from disk and
// delegates to ParseTemplate.
func TestLoadTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minimal.folio")
	if err := os.WriteFile(path, []byte(minimalTemplateJSON), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	tpl, err := LoadTemplate(path)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if tpl.doc.Version != "1.0" {
		t.Fatalf("Version = %q, want 1.0", tpl.doc.Version)
	}
}

// TestLoadTemplateMissingFile confirms LoadTemplate surfaces the
// underlying os error rather than swallowing it.
func TestLoadTemplateMissingFile(t *testing.T) {
	_, err := LoadTemplate(filepath.Join(t.TempDir(), "does-not-exist.folio"))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

// TestHigherMajorVersionIsRejected is AC6: a template declaring a MAJOR
// format version newer than the library supports fails to load, naming
// the declared and supported version, and no render is attempted (there
// is no *Template to pass to Render at all).
func TestHigherMajorVersionIsRejected(t *testing.T) {
	bad := `{"assets":{},"bands":{"content":{"elements":[]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{},"locale":"en","nextId":1,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"9.0"}`
	_, err := ParseTemplate([]byte(bad))
	if err == nil {
		t.Fatal("expected a load error for a higher MAJOR version")
	}
	// This story's finisher review (Finding 19, Minor): the assertion
	// used to stop at err != nil, though the comment above already
	// claimed the error names both versions — the stronger check existed
	// only in internal/template/version_test.go, one layer down. Assert
	// it here too, at the module-root public API this AC actually names.
	//
	// Story 7.2 raised the library ceiling from 1.0 to 1.1 (D-7.2.1), so
	// the supported half is read from template.SupportedVersion rather
	// than spelled as a literal here: this assertion is about the error
	// NAMING both versions, and a hand-copied ceiling would make it a
	// second, drifting declaration of what the ceiling is.
	if !strings.Contains(err.Error(), "9.0") || !strings.Contains(err.Error(), template.SupportedVersion) {
		t.Fatalf("error must name both the declared (9.0) and supported (%s) version, got: %v", template.SupportedVersion, err)
	}
}

// TestLoadTemplateRejectsHugeExponentQuickly is AC4c/D-1.6.6's retained
// CORPUS fixture, proved through the PUBLIC loader (not only the unit
// test in internal/template/decimal_test.go): a syntactically valid
// `.folio` file (testdata/template/malformed/huge-exponent.folio)
// containing element e1's x set to 1e99999999999999999999 must produce
// a located error, quickly.
//
// Red-proof wrinkle (D-1.2.5, AC4c doc note): BEFORE the fix in
// internal/template/decimal.go, this hung — killed by go test -timeout
// at 20s, per this story's Dev Notes M-1, reconfirmed independently for
// this exact file/entry point:
//
//	panic: test timed out after 20s
//	    running tests:
//	        TestProbePublicLoaderHang (20s)
//
// with the same ~74 math/big.nat.sqr/karatsubaSqr frames rooted at
// decodePoints. That timeout IS the red — never recorded as an
// assertion failure. This test does not re-add its own timer (this
// package's non-test and test files stay under AD-1's "no time" ban,
// D-1.3.1): the fix makes this call return in well under a second, and
// the surrounding `go test` process timeout is the safety net.
func TestLoadTemplateRejectsHugeExponentQuickly(t *testing.T) {
	root := repoRootFromTest(t)
	path := filepath.Join(root, "folio-go", "testdata", "template", "malformed", "huge-exponent.folio")

	tpl, err := LoadTemplate(path)
	if err == nil {
		t.Fatalf("LoadTemplate(%s): expected a located error for an absurd exponent, got a template and no error", path)
	}
	if tpl != nil {
		t.Fatalf("LoadTemplate(%s): expected a nil *Template alongside the error, got %#v", path, tpl)
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the offending element id (e1): %v", err)
	}
}

// TestLoadTemplateRejectsNegativeWrapExponentQuicklyToo is QA Finding 6
// (this story's review, Minor): the original retained corpus fixture's
// literal (1e99999999999999999999) wraps to a POSITIVE int64
// (7766279631452241919), which AC4a's Fix 2 (the magnitude bound,
// `n > MaxSplitExponentMagnitude`) still catches on its own — so
// removing Fix 1 (the during-accumulation overflow check) alone left
// that fixture GREEN, giving no public-loader coverage for the input
// class Fix 1 exists to close. The literal that actually defeats Fix 2
// alone is 1e9223372036854775808 (one past int64 max), which wraps to
// math.MinInt64 — a NEGATIVE value, so `n > MaxSplitExponentMagnitude`
// is false and Fix 2 cannot catch it. This fixture proves Fix 1 through
// the public loader the same way TestLoadTemplateRejectsHugeExponentQuickly
// proves Fix 2 (AC4c: "the fixture proves Story 1.4's promise now holds
// through the public path" — for BOTH input classes now).
func TestLoadTemplateRejectsNegativeWrapExponentQuicklyToo(t *testing.T) {
	root := repoRootFromTest(t)
	path := filepath.Join(root, "folio-go", "testdata", "template", "malformed", "huge-exponent-negative-wrap.folio")

	tpl, err := LoadTemplate(path)
	if err == nil {
		t.Fatalf("LoadTemplate(%s): expected a located error for an absurd exponent, got a template and no error", path)
	}
	if tpl != nil {
		t.Fatalf("LoadTemplate(%s): expected a nil *Template alongside the error, got %#v", path, tpl)
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the offending element id (e1): %v", err)
	}
}

// TestRenderAcceptsATemplate is AC22: Render's signature is
// func Render(t *Template, d Data, p Params, f FontSet) (Result, error).
func TestRenderAcceptsATemplate(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, FontSet{})
	if err != nil {
		t.Fatalf("Render(tpl): %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render(tpl) produced no bytes")
	}
}

// TestRenderIgnoresItsArgumentToday (Story 1.4's D-1.4.16 pinning test,
// folio-go/template_test.go:129-150) is DELETED here, not weakened
// (AC27, D-1.5.5): "The test was built in Story 1.4 to fail and force
// its own deletion (D-1.4.16); this is the story that does it... First
// self-retiring assertion in this run to reach its expiry as designed."
// Render(tpl) and Render(nil) can no longer even be compared: Render now
// requires a non-nil template (AC14b) and genuinely consumes it (text
// elements, fonts) — the property this test existed to police is exactly
// the one this story exists to make untrue.
