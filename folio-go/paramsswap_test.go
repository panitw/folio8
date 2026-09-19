package folio8

import (
	"os/exec"
	"strings"
	"testing"
)

// TestParamsDataSwapDoesNotTypeCheck is AC19 (D-1.7.5): a COMPILE-TIME
// proof, not a comment, that Data and Params — adjacent, same-
// underlying-type arguments in Render's signature — cannot be
// accidentally swapped. "An alias (type Params = []byte) would make
// the two mutually assignable and destroy the entire point" (D-1.7.5,
// verbatim); this test proves the defined-type shape actually shipped
// rejects the swap, rather than merely asserting it in prose (which is
// exactly what Story 1.6 did for Data alone — AC19a, M-8).
//
// The fixtures live under testdata/swapproof/ (bad, good, convert):
// testdata/ is excluded from "go build ./..." and "go test ./...", so
// none of the three is part of the normal build; each is built here by
// an EXPLICIT path, which Go's tooling does not exempt.
//
// AC26 Q2 (D-000.9 extended — the probe is not vacuous): the "good"
// fixture is byte-identical to "bad" except for argument order, and it
// must build. Without that control, "bad" failing to build would prove
// nothing about the SWAP specifically — it could fail for any reason.
func TestParamsDataSwapDoesNotTypeCheck(t *testing.T) {
	// Control: the unmutated argument order must compile (AC26 Q2).
	if out, err := goBuildFixture(t, "good"); err != nil {
		t.Fatalf("control fixture (correct argument order) must build, got error: %v\noutput:\n%s", err, out)
	}

	// The swap itself must fail to build, and the diagnostic must name
	// the assignability — not merely "failed" (D-000.13's "did it fail
	// for the reason it names").
	out, err := goBuildFixture(t, "bad")
	if err == nil {
		t.Fatal("AC19: Render(t, p, d, f) — Params and Data swapped — must NOT type-check, but it built successfully")
	}
	if !strings.Contains(out, "cannot use") || !strings.Contains(out, "folio8.Params") || !strings.Contains(out, "folio8.Data") {
		t.Fatalf("AC19: build failure must name the assignability (folio8.Params/folio8.Data), got:\n%s", out)
	}

	// AC19b (illustrative): a DELIBERATE conversion still compiles —
	// the guard proves the accidental swap is a compile error, not
	// that a deliberate cast is impossible. Recorded so a later reader
	// does not mistake the guard for a stronger proof than it is.
	if out, err := goBuildFixture(t, "convert"); err != nil {
		t.Fatalf("AC19b: an explicit folio8.Data(p)/folio8.Params(d) conversion is expected to still compile, got error: %v\noutput:\n%s", err, out)
	}
}

// goBuildFixture builds testdata/swapproof/<name> by its explicit
// package path — never via "./..." (which skips testdata/ entirely) —
// and returns the combined build output alongside the exec error, so
// callers can assert on the diagnostic TEXT (D-000.13), never on exit
// status alone.
func goBuildFixture(t *testing.T, name string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", t.TempDir()+"/out", "./testdata/swapproof/"+name)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
