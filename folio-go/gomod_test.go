package folio8

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// TestGoModPinsToolchainExactly is AC1: go.mod carries a literal
// "toolchain go1.26.0" line (exact version, no range) and a literal
// "go 1.25.0" line. Both lines are asserted, not just the toolchain one:
// Go strips a toolchain directive that is not strictly greater than the
// go directive (F-1, RP-9), so a go.mod with "go 1.26.0" and
// "toolchain go1.26.0" would look pinned right up until `go mod tidy`
// silently drops the pin — the low `go` line is what makes the pin
// survive, and this test must not be satisfiable by that broken shape.
func TestGoModPinsToolchainExactly(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	lines := strings.Split(string(data), "\n")

	hasGoLine := false
	hasToolchainLine := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "go 1.25.0" {
			hasGoLine = true
		}
		if trimmed == "toolchain go1.26.0" {
			hasToolchainLine = true
		}
	}

	if !hasGoLine {
		t.Error(`go.mod does not contain a literal "go 1.25.0" line`)
	}
	if !hasToolchainLine {
		t.Error(`go.mod does not contain a literal "toolchain go1.26.0" line`)
	}
}

// wantModuleGraph is the AC20 allowlist: exactly these modules, asserted
// by name, after Story 1.5 added github.com/boxesandglue/textshape. This
// is the primary assertion D-1.5.1 requires — replacing the old "exactly
// one module" count, which broke by design the moment this story added a
// real dependency (AC20):
//
//	"Two assertions in gomod_test.go, not one. Allowlist (primary).
//	go list -m all must equal exactly the expected set — after this
//	story {github.com/panitw/folio8/folio-go, github.com/boxesandglue/
//	textshape} — asserted by name, with the difference printed both
//	ways on failure. (mechanism: binding)" (D-1.5.1)
var wantModuleGraph = []string{
	"github.com/panitw/folio8/folio-go",
	"github.com/boxesandglue/textshape",
}

// knownPDFWriterModulePrefixes is AC21's independent denylist backstop —
// illustrative for the list itself (it must be extendable), binding for
// the property that no module at any depth of the resolved graph matches
// one of these. D-1.5.1, verbatim: "A denylist alone is weaker than the
// count it replaces: a PDF writer nobody listed passes silently, and
// lists rot. The count assertion's spirit was right — anything new must
// be a conscious act — and only its form was brittle." The allowlist
// above and this denylist are "two independent checks with different
// failure modes; neither substitutes for the other" (AC21).
var knownPDFWriterModulePrefixes = []string{
	"github.com/signintech/gopdf",
	"github.com/boxesandglue/baseline-pdf",
	"github.com/jung-kurt/gofpdf",
	"github.com/pdfcpu/pdfcpu",
	"github.com/unidoc/unipdf",
	"github.com/phpdave11/gofpdi",
}

// goListModuleNames runs `go list -m all` and returns each module's bare
// path (the version column, if any, is dropped — go list -m all's plain
// output is "path" for the main module and "path version" for every
// other module in the graph). It is skipped on js/wasm for the same
// measured reason Story 1.2 skipped its predecessor: js/wasm has no
// os/exec, so `go` itself is not on $PATH inside the binary's own
// execution environment ("exec: \"go\": executable file not found in
// $PATH", Story 1.2 F-3). This is an environmental gap, not a
// module-graph defect — the cross-target matrix harness
// (folio-go/matrix_test.go, //go:build matrix) still builds every target
// from the host, where `go list -m all` is available.
func goListModuleNames(t *testing.T) []string {
	t.Helper()
	if runtime.GOOS == "js" {
		t.Skip("js/wasm has no os/exec (\"go\" is not on $PATH); covered by the cross-target matrix harness (folio-go/matrix_test.go)")
	}
	cmd := exec.Command("go", "list", "-m", "all")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list -m all failed: %v\nstderr: %s", err, stderr.String())
	}

	var names []string
	for _, l := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		names = append(names, strings.Fields(l)[0])
	}
	return names
}

// diffModuleSets renders the two-way difference the AC20 allowlist
// requires ("the difference printed both ways on failure"): what the
// graph has that the allowlist does not, and what the allowlist expects
// that the graph does not have.
func diffModuleSets(got, want []string) (extra, missing []string) {
	gotSet := map[string]bool{}
	for _, g := range got {
		gotSet[g] = true
	}
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	for _, g := range got {
		if !wantSet[g] {
			extra = append(extra, g)
		}
	}
	for _, w := range want {
		if !gotSet[w] {
			missing = append(missing, w)
		}
	}
	return extra, missing
}

// TestModuleGraphAllowlist is AC20, D-1.5.1's primary assertion: the
// resolved module graph equals wantModuleGraph exactly, by name.
// Red-proofed against a scratch graph missing a required module and a
// scratch graph carrying an extra one (Task 1) via
// TestDiffModuleSetsCatchesExtraAndMissing below, which exercises
// diffModuleSets directly rather than shelling out to a second go.mod.
func TestModuleGraphAllowlist(t *testing.T) {
	got := goListModuleNames(t)
	extra, missing := diffModuleSets(got, wantModuleGraph)
	if len(extra) != 0 || len(missing) != 0 {
		t.Fatalf(
			"module graph does not equal the allowlist exactly (AC20, D-1.5.1).\n"+
				"got:     %v\n"+
				"want:    %v\n"+
				"extra (present, not allowed): %v\n"+
				"missing (allowed, not present): %v",
			got, wantModuleGraph, extra, missing,
		)
	}
}

// TestDiffModuleSetsCatchesExtraAndMissing is TestModuleGraphAllowlist's
// red-proof (Task 1): a scratch graph missing a required module, and one
// carrying an extra module, both produce a non-empty diff.
func TestDiffModuleSetsCatchesExtraAndMissing(t *testing.T) {
	want := []string{"github.com/panitw/folio8/folio-go", "github.com/boxesandglue/textshape"}

	extra, missing := diffModuleSets([]string{"github.com/panitw/folio8/folio-go"}, want)
	if len(missing) != 1 || missing[0] != "github.com/boxesandglue/textshape" {
		t.Fatalf("missing-module case: got extra=%v missing=%v, want missing=[textshape]", extra, missing)
	}

	extra, missing = diffModuleSets([]string{
		"github.com/panitw/folio8/folio-go",
		"github.com/boxesandglue/textshape",
		"github.com/example/unexpected",
	}, want)
	if len(extra) != 1 || extra[0] != "github.com/example/unexpected" {
		t.Fatalf("extra-module case: got extra=%v missing=%v, want extra=[unexpected]", extra, missing)
	}
}

// deniedHits is the denylist's ONE matcher, shared by the production
// check and its red-proof (Finding 3, QA review). Before this fix, the
// red-proof inlined a duplicate of this loop, sharing only
// knownPDFWriterModulePrefixes with production: neutering the
// production matcher left BOTH tests green (proved by construction —
// see the red-proof note on TestModuleGraphDenylistRedProof below).
// Extracting a shared function is the same shape diffModuleSets already
// models correctly for the allowlist.
func deniedHits(modules []string) []string {
	var hits []string
	for _, g := range modules {
		for _, forbidden := range knownPDFWriterModulePrefixes {
			if g == forbidden || strings.HasPrefix(g, forbidden+"/") {
				hits = append(hits, g)
			}
		}
	}
	return hits
}

// TestModuleGraphDenylistsKnownPDFWriters is AC21, D-1.5.1's independent
// backstop: no module in the resolved graph matches a known Go PDF
// writer at any depth, regardless of what the allowlist above says.
func TestModuleGraphDenylistsKnownPDFWriters(t *testing.T) {
	got := goListModuleNames(t)
	hits := deniedHits(got)
	if len(hits) != 0 {
		t.Fatalf("module graph contains a known Go PDF writer (AC21, AD-6): %v", hits)
	}
}

// TestModuleGraphDenylistRedProof is the denylist's own red-proof (Task
// 1): a scratch graph containing one of the six named modules must be
// caught, exercised through deniedHits — the SAME matcher production
// uses (Finding 3, QA review), not a copy of it. Re-red-proofed live:
// neutering deniedHits to always return nil reddens this test.
func TestModuleGraphDenylistRedProof(t *testing.T) {
	scratch := []string{
		"github.com/panitw/folio8/folio-go",
		"github.com/boxesandglue/textshape",
		"github.com/signintech/gopdf",
	}
	hits := deniedHits(scratch)
	if len(hits) != 1 || hits[0] != "github.com/signintech/gopdf" {
		t.Fatalf("denylist red-proof: got hits=%v, want exactly [github.com/signintech/gopdf]", hits)
	}
}
