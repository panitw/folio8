package folio8

import (
	"os/exec"
	"strings"
	"testing"
)

// TestTemplateCompositeLiteralDoesNotTypeCheck is the Epic 1 boundary
// gate finding's binding requirement: a COMPILE-TIME proof, not a
// comment, that folio8.Template — migrated from a type ALIAS
// (`type Template = template.Document`) to an opaque handle
// (`type Template struct { doc *template.Document }`) — can no longer
// be constructed field-by-field from outside package folio8. The alias
// let a caller write `folio8.Template{Version: "1.0", ...}` and
// sidestep ParseTemplate's asset-key validation, version rules, the
// eight closed sets, exact-decimal discipline and nextId
// monotonicity entirely; this test proves the opaque shape actually
// shipped closes that hole, rather than merely asserting it in prose
// (the same discipline Story 1.7's TestParamsDataSwapDoesNotTypeCheck,
// paramsswap_test.go, applies to the Data/Params swap).
//
// The fixtures live under testdata/templateopaque/ (good, bad):
// testdata/ is excluded from "go build ./..." and "go test ./...", so
// neither is part of the normal build; each is built here by an
// EXPLICIT path, which Go's tooling does not exempt.
//
// The "good" fixture constructs a Template only through ParseTemplate
// (the sanctioned path) and must build — the control that proves "bad"
// failing to build is about the composite literal specifically, not
// about some unrelated breakage (the same reasoning as AC26 Q2 in
// Story 1.7's swap proof).
func TestTemplateCompositeLiteralDoesNotTypeCheck(t *testing.T) {
	// Control: constructing a Template via ParseTemplate must compile.
	if out, err := goBuildTemplateOpaqueFixture(t, "good"); err != nil {
		t.Fatalf("control fixture (ParseTemplate construction) must build, got error: %v\noutput:\n%s", err, out)
	}

	// The composite-literal construction itself must fail to build,
	// and the diagnostic must name the reason — an unknown field on
	// folio8.Template — not merely "failed" (D-000.13's "did it fail for
	// the reason it names").
	out, err := goBuildTemplateOpaqueFixture(t, "bad")
	if err == nil {
		t.Fatal("Epic 1 boundary gate: folio8.Template{Version: \"1.0\"} must NOT type-check from outside package folio8, but it built successfully")
	}
	if !strings.Contains(out, "unknown field") || !strings.Contains(out, "Template") {
		t.Fatalf("Epic 1 boundary gate: build failure must name the unknown field on folio8.Template, got:\n%s", out)
	}
}

// goBuildTemplateOpaqueFixture builds testdata/templateopaque/<name> by
// its explicit package path — never via "./..." (which skips
// testdata/ entirely) — and returns the combined build output alongside
// the exec error, so callers can assert on the diagnostic TEXT
// (D-000.13), never on exit status alone.
func goBuildTemplateOpaqueFixture(t *testing.T, name string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", t.TempDir()+"/out", "./testdata/templateopaque/"+name)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
