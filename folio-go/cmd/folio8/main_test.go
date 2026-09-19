package main

// Story 3.7's CLI tests. AC3-AC7, AC9 and AC11's first three cases run
// run() DIRECTLY, in-process (no subprocess needed: run's own
// signature already takes stdout/stderr/getenv as parameters, so a
// fake getenv exercises everything except "does this binary read the
// REAL process environment", which only AC8 and AC11's fourth case
// need — see main_subprocess_test.go for those, run out-of-process
// against the actual compiled binary).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliWellFormedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// QA Nit 1 (this story's review): a second template constant,
// cliMissingGlyphTemplateJSON, used to live here, byte-identical to
// cliWellFormedTemplateJSON above — the missing-glyph document's
// distinguishing feature is its DATA (Thai text a single "Noto Sans"
// chain does not cover), never the template. Removed; every use site
// below shares cliWellFormedTemplateJSON.

const cliMalformedTemplateJSON = `{ not valid json`

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func noEnv(string) string { return "" }

// AC3: run with no arguments, or with an unknown subcommand, names
// both "validate" and "render" on stderr, and exits with the usage
// code.
func TestUsageNamesBothSubcommands(t *testing.T) {
	cases := [][]string{
		{},
		{"bogus"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr, noEnv)
		if code != exitUsageErr {
			t.Errorf("args=%v: exit code = %d, want %d", args, code, exitUsageErr)
		}
		if !strings.Contains(stderr.String(), "validate") || !strings.Contains(stderr.String(), "render") {
			t.Errorf("args=%v: usage text does not name both subcommands: %q", args, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("args=%v: stdout = %q, want empty (usage goes to stderr)", args, stdout.String())
		}
	}
}

// AC3: both subcommands are reachable and work end-to-end.
func TestValidateAndRenderAreReachable(t *testing.T) {
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)

	var vout, verr bytes.Buffer
	if code := run([]string{"validate", "-data", dataPath, tplPath}, &vout, &verr, noEnv); code != exitOK {
		t.Fatalf("validate: exit code = %d, stderr = %q", code, verr.String())
	}

	var rout, rerr bytes.Buffer
	if code := run([]string{"render", "-data", dataPath, tplPath}, &rout, &rerr, noEnv); code != exitOK {
		t.Fatalf("render: exit code = %d, stderr = %q", code, rerr.String())
	}
	if !bytes.HasPrefix(rout.Bytes(), []byte("%PDF-1.7")) {
		t.Errorf("render: stdout does not start with a PDF header: %q", rout.Bytes()[:min(20, rout.Len())])
	}
}

// AC4: exit codes and stream discipline.
func TestExitCodesAndStreamDiscipline(t *testing.T) {
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	malformedPath := writeTempFile(t, dir, "bad.folio", cliMalformedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)

	t.Run("success exits 0, bytes on stdout, nothing else", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "-data", dataPath, tplPath}, &stdout, &stderr, noEnv)
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty (a clean render prints no diagnostic text)", stderr.String())
		}
		if !bytes.HasPrefix(stdout.Bytes(), []byte("%PDF-1.7")) {
			t.Errorf("stdout does not start with a PDF header")
		}
	})

	t.Run("render/validate failure exits 1, error on stderr", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "-data", dataPath, malformedPath}, &stdout, &stderr, noEnv)
		if code != exitFailure {
			t.Fatalf("exit code = %d, want %d", code, exitFailure)
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want empty on failure", stdout.String())
		}
		if stderr.Len() == 0 {
			t.Errorf("stderr is empty, want the error message")
		}

		stdout.Reset()
		stderr.Reset()
		code = run([]string{"validate", "-data", dataPath, malformedPath}, &stdout, &stderr, noEnv)
		if code != exitFailure {
			t.Fatalf("validate: exit code = %d, want %d", code, exitFailure)
		}
	})

	t.Run("usage error exits 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"render"}, &stdout, &stderr, noEnv) // missing required template argument
		if code != exitUsageErr {
			t.Fatalf("exit code = %d, want %d", code, exitUsageErr)
		}
	})

	t.Run("output to a file: nothing on stdout", func(t *testing.T) {
		outPath := filepath.Join(dir, "out.pdf")
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "-data", dataPath, "-o", outPath, tplPath}, &stdout, &stderr, noEnv)
		if code != exitOK {
			t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want empty when -o is given", stdout.String())
		}
		b, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read output file: %v", err)
		}
		if !bytes.HasPrefix(b, []byte("%PDF-1.7")) {
			t.Errorf("output file does not start with a PDF header")
		}
	})

	// This is a PERMANENT test, not a one-off (AC4's own text): a later
	// "helpful" summary line on stdout alongside the bytes is exactly
	// how `folio8 render t.folio > out.pdf` gets silently corrupted.
	//
	// QA Finding 1 (this story's review, BLOCKER): the original version
	// of this subtest asserted only bytes.HasPrefix(stdout, "%PDF-1.7")
	// — character-for-character the SAME assertion the "success exits
	// 0..." subtest above it already makes. That catches a summary line
	// PREPENDED to the bytes and NOTHING else: appending
	// `fmt.Fprintf(stdout, "\nfolio8: wrote %d bytes\n", len(res.Bytes))`
	// after the write in runRender left every assertion in this package
	// GREEN while the piped output was a corrupt PDF (confirmed live
	// during review). The fix asserts the WHOLE stream, both ends:
	// stdout must be byte-for-byte identical to the same document
	// rendered to a file via -o (an equality this test owns on both
	// sides, so a stray byte in ANY position — prepended, appended, or
	// injected in the middle — reddens it), AND must end in "%%EOF\n"
	// specifically, which a bare prefix check could never catch no
	// matter how it was written.
	t.Run("mutation: a stdout summary line must be caught", func(t *testing.T) {
		referencePath := filepath.Join(dir, "reference-for-purity-check.pdf")
		var refOut, refErr bytes.Buffer
		if code := run([]string{"render", "-data", dataPath, "-o", referencePath, tplPath}, &refOut, &refErr, noEnv); code != exitOK {
			t.Fatalf("reference render (-o): exit code = %d, stderr=%q", code, refErr.String())
		}
		referenceBytes, err := os.ReadFile(referencePath)
		if err != nil {
			t.Fatalf("read reference output: %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "-data", dataPath, tplPath}, &stdout, &stderr, noEnv)
		if code != exitOK {
			t.Fatalf("exit code = %d", code)
		}
		if !bytes.Equal(stdout.Bytes(), referenceBytes) {
			t.Fatalf("stdout must be EXACTLY the rendered PDF bytes and NOTHING ELSE — any prepended or appended text (e.g. a \"helpful\" summary line) must be caught here; stdout len=%d, reference len=%d", stdout.Len(), len(referenceBytes))
		}
		if !bytes.HasSuffix(stdout.Bytes(), []byte("%%EOF\n")) {
			t.Fatalf("stdout does not end in %%%%EOF\\n — a byte appended after the PDF's own trailer must be caught here")
		}
	})
}

// AC5: --strict makes any Warning exit non-zero, and it is OFF by
// default. Both arms asserted on the SAME document (the missing-glyph
// fixture from AC6) so the flag is the only variable.
func TestStrictFlag(t *testing.T) {
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "ก"}`) // Thai, not covered by "Noto Sans"

	t.Run("without --strict: exits 0, complete bytes written", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "-data", dataPath, tplPath}, &stdout, &stderr, noEnv)
		if code != exitOK {
			t.Fatalf("exit code = %d, want %d (AD-14: a Warning accompanies a successful render); stderr=%q", code, exitOK, stderr.String())
		}
		if !bytes.HasPrefix(stdout.Bytes(), []byte("%PDF-1.7")) {
			t.Errorf("stdout does not carry complete PDF bytes despite the Warning")
		}
		if !strings.Contains(stderr.String(), "TEXT_MISSING_GLYPH") {
			t.Errorf("stderr does not report the missing-glyph Warning: %q", stderr.String())
		}
	})

	t.Run("with --strict: the SAME document exits non-zero", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"render", "-strict", "-data", dataPath, tplPath}, &stdout, &stderr, noEnv)
		// QA Finding 16 (this story's review, Minor): "code == exitOK"
		// is too loose — D-3.7.4 assigns exit 1 specifically to
		// "--strict with warnings" and reserves 2 for usage errors, so
		// a regression that turned a strict failure into a USAGE error
		// (exactly the confusion exit 2 exists to prevent) would still
		// pass this assertion. exitFailure is the one code this arm
		// may produce.
		if code != exitFailure {
			t.Fatalf("exit code = %d, want %d (--strict with a Warning present)", code, exitFailure)
		}
	})

	// QA Nit 2 (this story's review): a third subtest, "mutation
	// control: default (no flag) must never behave like --strict", used
	// to live here — same argv, same expectation as "without --strict:
	// exits 0..." above, with no additional discrimination. AC5's
	// default-must-not-flip-to-on mutation is already the property that
	// first subtest asserts; this was a duplicate, not a second guard,
	// so it is removed rather than kept for the look of extra coverage.
}

// AC6: the CLI PRINTS the diagnostics it receives, on ITS OWN stderr
// stream — asserted by running the command and reading the captured
// stream, never by asserting Result.Diagnostics directly (D-000.21).
func TestDiagnosticsPrintedOnStderr(t *testing.T) {
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "ก"}`)

	var stdout, stderr bytes.Buffer
	code := run([]string{"render", "-data", dataPath, tplPath}, &stdout, &stderr, noEnv)
	if code != exitOK {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "TEXT_MISSING_GLYPH") {
		t.Errorf("stderr does not name the stable code: %q", out)
	}
	if !strings.Contains(out, "e1") {
		t.Errorf("stderr does not name the element id: %q", out)
	}
	if !strings.Contains(out, "U+0E01") {
		t.Errorf("stderr does not name the offending rune's U+XXXX form: %q", out)
	}

	// Negative control: a clean render prints no diagnostic text at all.
	cleanDataPath := writeTempFile(t, dir, "clean-data.json", `{"name": "Jane"}`)
	cleanTplPath := writeTempFile(t, dir, "clean.folio", cliWellFormedTemplateJSON)
	var cout, cerr bytes.Buffer
	code = run([]string{"render", "-data", cleanDataPath, cleanTplPath}, &cout, &cerr, noEnv)
	if code != exitOK {
		t.Fatalf("clean render: exit code = %d", code)
	}
	if cerr.Len() != 0 {
		t.Errorf("clean render: stderr = %q, want empty — \"the CLI printed something\" must not be mistaken for \"the CLI printed this\"", cerr.String())
	}
}

// AC9: the same date arrives when a caller supplies documentDate
// directly through -params, with no environment involved — proving the
// CLI's params path is "a caller like any other" (AD-7), not a
// privileged route only SOURCE_DATE_EPOCH can reach.
func TestParamsSuppliedDocumentDateNoEnvironment(t *testing.T) {
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)
	paramsPath := writeTempFile(t, dir, "params.json", `{"documentDate": "2023-06-15T12:00:00Z"}`)

	var stdout, stderr bytes.Buffer
	code := run([]string{"render", "-data", dataPath, "-params", paramsPath, tplPath}, &stdout, &stderr, noEnv)
	if code != exitOK {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	b := stdout.Bytes()
	if !bytes.Contains(b, []byte("/CreationDate")) || !bytes.Contains(b, []byte("/ModDate")) {
		t.Fatalf("output does not carry /CreationDate and /ModDate despite documentDate supplied via -params")
	}
	if !bytes.Contains(b, []byte("D:20230615120000")) {
		t.Errorf("output does not carry the expected formatted date string: %q", b)
	}
}

// QA Nit 5 (this story's review): a local min(a, b int) int used to be
// declared here, shadowing the Go 1.21+ builtin for this whole test
// build. Deleted — go.mod pins go 1.25.0, so the builtin is available.
