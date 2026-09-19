package main

// AC8 and AC11's fourth case run the ACTUAL COMPILED BINARY out of
// process, with a controlled child environment (D-000.59 part (a):
// "asserted THROUGH THE PARAMS PATH, not by the env var appearing in
// source" — a subprocess is what lets a test control SOURCE_DATE_EPOCH
// as the real os.Getenv this binary calls sees it, which an in-process
// getenv func parameter would not exercise for main() itself).

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// buildFolio8Binary compiles this package to a temp path once per test
// and returns it.
func buildFolio8Binary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "folio8-under-test")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build cmd/folio8: %v\n%s", err, out)
	}
	return bin
}

// AC8: cmd/folio8 render reads SOURCE_DATE_EPOCH, passes it in as the
// documentDate PARAMETER, and the date reaches /CreationDate AND
// /ModDate in the produced bytes — asserted through the params path
// (a subprocess with the env var set in its OWN environment), reading
// both keys off the produced PDF bytes (D-000.21), asserting the
// ACTUAL formatted date string, not merely that the keys are present.
func TestRenderReadsSourceDateEpochFromEnvironment(t *testing.T) {
	bin := buildFolio8Binary(t)
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)

	// 1750000000 -> 2025-06-15T15:06:40Z (a fixed epoch this test owns
	// and computes the expected PDF date string for independently;
	// QA Finding 11, this story's review: this comment previously said
	// 16:26:40Z, wrong by 80 minutes — the assertion below, and the
	// live binary, both agree on 15:06:40Z).
	cmd := exec.Command(bin, "render", "-data", dataPath, tplPath)
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=1750000000")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("render with SOURCE_DATE_EPOCH set: %v, stderr=%s", err, stderr.String())
	}

	b := stdout.Bytes()
	if !bytes.Contains(b, []byte("/CreationDate")) {
		t.Errorf("output does not contain /CreationDate")
	}
	if !bytes.Contains(b, []byte("/ModDate")) {
		t.Errorf("output does not contain /ModDate")
	}
	// epoch 1750000000 == 2025-06-15T15:06:40Z (verified against the
	// standard library's own time.Unix(1750000000, 0).UTC()).
	want := "D:20250615150640+00'00'"
	if !bytes.Contains(b, []byte(want)) {
		t.Errorf("output does not contain the expected formatted date %q derived from SOURCE_DATE_EPOCH=1750000000; got:\n%s", want, b)
	}
}

// A hard-coded epoch instead of the one actually read from the
// environment must be caught: this test asserts the ACTUAL derived
// string for the SPECIFIC epoch supplied, so a CLI that ignores the
// environment and emits a fixed date fails here even though
// /CreationDate would still be present (AC8's own named mutation).
func TestRenderSourceDateEpochValueIsHonoured(t *testing.T) {
	bin := buildFolio8Binary(t)
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)

	cmd := exec.Command(bin, "render", "-data", dataPath, tplPath)
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("render: %v, stderr=%s", err, stderr.String())
	}
	// epoch 0 == 1970-01-01T00:00:00Z.
	want := "D:19700101000000+00'00'"
	if !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Errorf("output does not contain %q for SOURCE_DATE_EPOCH=0 — a hard-coded date instead of the one read from the environment must be caught here", want)
	}
}

// AC11's fourth case: the CLI run with SOURCE_DATE_EPOCH UNSET in the
// child environment produces output BYTE-IDENTICAL to the same
// document rendered with no params at all — no route supplied a date,
// so neither /CreationDate nor /ModDate (nor /Info) appears (AC11,
// D-000.59 part (c)).
func TestRenderWithSourceDateEpochUnsetIsByteIdenticalToNoParams(t *testing.T) {
	bin := buildFolio8Binary(t)
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)

	// Build an environment with SOURCE_DATE_EPOCH explicitly ABSENT
	// (never merely empty) — filter it out of the current process's
	// own environment in case it happens to be set.
	var env []string
	for _, kv := range os.Environ() {
		if len(kv) >= len("SOURCE_DATE_EPOCH=") && kv[:len("SOURCE_DATE_EPOCH=")] == "SOURCE_DATE_EPOCH=" {
			continue
		}
		env = append(env, kv)
	}

	run := func() []byte {
		t.Helper()
		cmd := exec.Command(bin, "render", "-data", dataPath, tplPath)
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("render: %v, stderr=%s", err, stderr.String())
		}
		return stdout.Bytes()
	}

	first := run()
	second := run()
	if !bytes.Equal(first, second) {
		t.Fatalf("two renders with SOURCE_DATE_EPOCH unset produced different bytes")
	}
	if bytes.Contains(first, []byte("/CreationDate")) || bytes.Contains(first, []byte("/ModDate")) || bytes.Contains(first, []byte("/Info")) {
		t.Errorf("output with SOURCE_DATE_EPOCH unset unexpectedly carries a date/Info key")
	}

	// Compare against the SAME document rendered through a genuinely
	// DIFFERENT route: folio8.Render called directly, in-process, with
	// Params(nil) — never through mainRunForTest/run with an empty
	// "{}" params file, which QA Finding 15 (this story's review,
	// Minor) pointed out parses the identical argv, loads the
	// identical files and reaches injectDocumentDateFromEnv with no
	// epoch: the SAME code path with the SAME inputs as `first`/`second`
	// above, so it could not fail unless the renderer were itself
	// nondeterministic (already tested by first == second). Calling
	// folio8.Render directly instead exercises decodeParams' nil
	// short-circuit rather than the "{}" JSON path, which is a real
	// second route to the same byte-identity property, per AC14's own
	// corpus obligation applied here to the CLI's no-date path.
	tpl, perr := folio8.ParseTemplate([]byte(cliWellFormedTemplateJSON))
	if perr != nil {
		t.Fatalf("ParseTemplate: %v", perr)
	}
	res, rerr := folio8.Render(tpl, folio8.Data(`{"name": "Jane"}`), folio8.Params(nil), fonts.Shipped())
	if rerr != nil {
		t.Fatalf("in-process Render (Params(nil)): %v", rerr)
	}
	if !bytes.Equal(first, res.Bytes) {
		t.Fatalf("CLI output with SOURCE_DATE_EPOCH unset is not byte-identical to the same document rendered in-process via folio8.Render with Params(nil)")
	}
}
