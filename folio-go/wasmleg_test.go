package folio8

// This file lives at the folio-go module root (package folio8), NOT
// under internal/ — matching matrix_test.go's precedent. AD-1's
// forbidden-imports guard scans folio-go/internal/ only; os/exec is
// banned there (even in _test.go files: D-1.3.1's exemption is exactly
// os/testing/path-filepath/embed, and os/exec is a banned subpackage of
// os per Finding 6's fix), so a harness that spawns a child process to
// build/run a second binary belongs here, one directory up, exactly
// where Story 1.2's matrix harness already put the same kind of code.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInternalTextJSWasmLeg is Story 2.1's AC3, run IN-STORY (not
// deferred): D-2.1.1 ruled this story is NOT a D-000.4 matrix override
// — it needs one leg (js/wasm execution, a target-capability question),
// not all four (byte-identity legs, deferred to the Epic 2 gate; see
// AC10 and the Delivery Log). This builds folio-go/internal/text's own
// test binary for GOOS=js GOARCH=wasm with `go test -c`, reusing the
// SAME seam matrix_test.go already established (go_js_wasm_exec
// resolved from `go env GOROOT`, run under Node — never a vendored
// wasm_exec.js) rather than inventing a second one.
//
// It asserts on the child's REPORTED OUTCOME — the probe log lines
// internal/text's TestProbeQueries prints — never on exit status alone
// (D-000.13), and it asserts the child actually reports GOOS=js
// GOARCH=wasm (V6): a run that silently fell back to native, or exited
// 0 without executing the trie at all, would look identical to a
// healthy run on exit status alone.
func TestInternalTextJSWasmLeg(t *testing.T) {
	if testing.Short() {
		t.Skip("js/wasm leg builds and runs a second binary under Node; skipped under -short")
	}

	if _, err := exec.LookPath("node"); err != nil {
		t.Fatalf("node not found on PATH; AC3's js/wasm leg requires Node.js: %v", err)
	}

	goroot := goEnvGOROOTForTextWasm(t)
	wrapper := filepath.Join(goroot, "lib", "wasm", "go_js_wasm_exec")
	if _, err := os.Stat(wrapper); err != nil {
		t.Fatalf("wasm exec wrapper not found at %s (resolved from `go env GOROOT`, never a vendored wasm_exec.js): %v", wrapper, err)
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "text.js-wasm.test")

	buildCmd := exec.Command("go", "test", "-c", "-o", binPath, "./internal/text")
	buildCmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("building internal/text js/wasm test binary failed: %v\nstderr: %s", err, buildStderr.String())
	}

	// Also runs TestAC10ComputedBreaksMatchS4Basis in the same wasm
	// process (DoD: "AC10 asserted on a target other than the native
	// one at least once — the js/wasm leg"), so the deferred Docker
	// legs at the Epic 2 gate are proven, here, to genuinely exercise
	// this story's work rather than merely compiling.
	runCmd := exec.Command(wrapper, binPath, "-test.run=TestProbeQueries|TestAC10ComputedBreaksMatchS4Basis", "-test.v")
	runCmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr
	if err := runCmd.Run(); err != nil {
		t.Fatalf("js/wasm run FAILED — this is a failure, not a skip. wrapper=%s\nerror: %v\nstdout: %s\nstderr: %s",
			wrapper, err, stdout.String(), stderr.String())
	}

	out := stdout.String()

	if !strings.Contains(out, "PROBE-TARGET: GOOS=js GOARCH=wasm") {
		t.Fatalf("js/wasm run did not report GOOS=js GOARCH=wasm (V6: a silent native fallback would look identical on exit status alone) — full output:\n%s", out)
	}
	if !strings.Contains(out, "PROBE-RESULT: all four probe categories returned the expected outcome") {
		t.Fatalf("js/wasm run did not report the expected probe outcome line — full output:\n%s", out)
	}
	if !strings.Contains(out, "--- PASS: TestProbeQueries") {
		t.Fatalf("TestProbeQueries did not report PASS under js/wasm — full output:\n%s", out)
	}
	if !strings.Contains(out, "--- PASS: TestAC10ComputedBreaksMatchS4Basis") {
		t.Fatalf("AC10: TestAC10ComputedBreaksMatchS4Basis did not report PASS under js/wasm (this target must reproduce the S4-basis fixture too) — full output:\n%s", out)
	}

	t.Logf("js/wasm leg: internal/text's TestProbeQueries AND TestAC10ComputedBreaksMatchS4Basis both ran and passed under Node (%s) — AC10 asserted on this target, not just compiled", wrapper)
}

func goEnvGOROOTForTextWasm(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "env", "GOROOT")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go env GOROOT failed: %v\nstderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(out.String())
}
