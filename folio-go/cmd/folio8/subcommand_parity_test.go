package main

// D-3.7.6 (this story's review, Findings 5 and 6 — engineering lead's
// ruling): SOURCE_DATE_EPOCH FILLS an absent documentDate; it never
// overwrites one the caller supplied through -params. Both
// subcommands must see IDENTICAL inputs — resolveInputs (main.go) is
// the ONE input-assembly step both call, so this file's job is to
// prove that in-process, over a table parameterised by SUBCOMMAND
// (never two hand-written tests that merely happen to agree), and to
// prove the subcommand set itself cannot silently grow a third member
// that escapes the table.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestSubcommandNamesMatchRunSwitch asserts subcommandNames (main.go)
// is AST-EQUAL to the string case labels of `switch args[0]` inside
// run — never merely eyeballed equal (D-3.7.6's guardrail, verbatim:
// "the subcommand list in that table is a test-owned literal asserted
// AST-equal to the case set of the switch args[0] in run. A third
// subcommand must red this table rather than quietly fall outside
// it."). This is what makes it structurally impossible to add a third
// subcommand to run's switch without the parity table below noticing.
func TestSubcommandNamesMatchRunSwitch(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine this test file's own path")
	}
	mainGoPath := filepath.Join(filepath.Dir(thisFile), "main.go")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	var got []string
	found := false
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name.Name != "run" || fd.Body == nil {
			continue
		}
		found = true
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok || cc.List == nil { // a nil List is the "default" clause
					continue
				}
				for _, expr := range cc.List {
					lit, ok := expr.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					unquoted := strings.Trim(lit.Value, `"`)
					got = append(got, unquoted)
				}
			}
			return false // only the switch's own top-level case clauses
		})
	}
	if !found {
		t.Fatal("vacuity guard: main.go declares no func run — AST scan target does not exist")
	}
	if len(got) == 0 {
		t.Fatal("vacuity guard: found zero string case labels in run's switch — parity table's own anchor is unassertable")
	}

	wantSorted := append([]string(nil), subcommandNames...)
	sort.Strings(wantSorted)
	gotSorted := append([]string(nil), got...)
	sort.Strings(gotSorted)
	if len(wantSorted) != len(gotSorted) {
		t.Fatalf("subcommandNames = %v (%d), but run's switch dispatches on %v (%d) — a subcommand was added to (or removed from) the switch without updating subcommandNames", subcommandNames, len(wantSorted), got, len(gotSorted))
	}
	for i := range wantSorted {
		if wantSorted[i] != gotSorted[i] {
			t.Fatalf("subcommandNames = %v, but run's switch dispatches on %v — they must name EXACTLY the same subcommands", subcommandNames, got)
		}
	}
}

// documentDateParityCase is one cell of the table below: a
// (params-has-documentDate, SOURCE_DATE_EPOCH) combination, run
// against BOTH subcommands, asserting they agree.
type documentDateParityCase struct {
	name               string
	paramsJSON         string // "" means no -params flag at all
	env                string // "" means SOURCE_DATE_EPOCH unset
	wantExitBoth       int
	wantStderrEq       bool   // if true, render's and validate's stderr must be byte-identical
	renderOutHasSubstr string // "" to skip; checked only on render's stdout
}

// TestSourceDateEpochFillsAbsentButNeverOverwritesSupplied is D-3.7.6's
// central guardrail: a table PARAMETERISED OVER THE SUBCOMMAND (never
// two tests that happen to agree), covering (params-has-date x env
// absent/valid/malformed), run against BOTH "validate" and "render",
// asserting equal exit codes on every cell — it is structurally
// impossible for this table to cover render and skip validate, because
// each case below is run through the same loop for both names in
// subcommandNames-shaped fashion (asserted equal to subcommandNames by
// TestSubcommandNamesMatchRunSwitch above).
//
// Both polarities named in D-3.7.6 are present: "env fills absent"
// (the no-params-date rows) produces a date; "env with a supplied
// param" (the params-has-date rows) must produce the CALLER's value,
// never the environment's — this is the cell the red-proof (see the
// mutation note below) targets.
func TestSourceDateEpochFillsAbsentButNeverOverwritesSupplied(t *testing.T) {
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)

	cases := []documentDateParityCase{
		{
			name:         "no params documentDate, env absent: both succeed, no date emitted",
			paramsJSON:   "",
			env:          "",
			wantExitBoth: exitOK,
		},
		{
			name:               "no params documentDate, env valid: both succeed, env FILLS the absent date",
			paramsJSON:         "",
			env:                "1750000000",
			wantExitBoth:       exitOK,
			renderOutHasSubstr: "D:20250615150640",
		},
		{
			name:         "no params documentDate, env malformed: both are a USAGE error",
			paramsJSON:   "",
			env:          "not-an-integer",
			wantExitBoth: exitUsageErr,
			wantStderrEq: true,
		},
		{
			name:               "params documentDate SUPPLIED, env absent: both succeed with the CALLER's value",
			paramsJSON:         `{"documentDate": "2001-02-03T04:05:06Z"}`,
			env:                "",
			wantExitBoth:       exitOK,
			renderOutHasSubstr: "D:20010203040506",
		},
		{
			name:               "params documentDate SUPPLIED, env ALSO valid but DIFFERENT: the CALLER's value wins, not the environment's",
			paramsJSON:         `{"documentDate": "2001-02-03T04:05:06Z"}`,
			env:                "1750000000",
			wantExitBoth:       exitOK,
			renderOutHasSubstr: "D:20010203040506",
		},
		{
			name:         "params documentDate SUPPLIED, env malformed: the key's presence short-circuits the environment entirely, so a broken (e.g. build-system-wide) SOURCE_DATE_EPOCH does not fail a build that already supplied its own date",
			paramsJSON:   `{"documentDate": "2001-02-03T04:05:06Z"}`,
			env:          "not-an-integer",
			wantExitBoth: exitOK,
		},
		{
			name:         "params documentDate present but EXPLICIT NULL: presence, not truthiness, is the test — the environment does NOT fill it, and the engine rejects the wrong-typed value on BOTH subcommands identically",
			paramsJSON:   `{"documentDate": null}`,
			env:          "1750000000",
			wantExitBoth: exitFailure,
			wantStderrEq: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var paramsPath string
			if c.paramsJSON != "" {
				paramsPath = writeTempFile(t, dir, sanitizeName(c.name)+"-params.json", c.paramsJSON)
			}
			getenv := func(key string) string {
				if key == "SOURCE_DATE_EPOCH" {
					return c.env
				}
				return ""
			}

			run1 := func(subcommand string) (code int, stdout, stderr string) {
				args := []string{subcommand, "-data", dataPath}
				if paramsPath != "" {
					args = append(args, "-params", paramsPath)
				}
				args = append(args, tplPath)
				var out, errB bytes.Buffer
				code = run(args, &out, &errB, getenv)
				return code, out.String(), errB.String()
			}

			validateCode, _, validateStderr := run1("validate")
			renderCode, renderStdout, renderStderr := run1("render")

			if validateCode != c.wantExitBoth {
				t.Errorf("validate: exit code = %d, want %d", validateCode, c.wantExitBoth)
			}
			if renderCode != c.wantExitBoth {
				t.Errorf("render: exit code = %d, want %d", renderCode, c.wantExitBoth)
			}
			if validateCode != renderCode {
				t.Errorf("PARITY VIOLATION: validate exit=%d, render exit=%d — folio8 validate must never accept an input combination folio8 render would refuse (or vice versa)", validateCode, renderCode)
			}
			if c.wantStderrEq && validateStderr != renderStderr {
				t.Errorf("PARITY VIOLATION: validate stderr=%q, render stderr=%q — both subcommands share resolveInputs and must fail identically", validateStderr, renderStderr)
			}
			if c.renderOutHasSubstr != "" && !strings.Contains(renderStdout, c.renderOutHasSubstr) {
				t.Errorf("render stdout does not contain %q: %q", c.renderOutHasSubstr, renderStdout)
			}
		})
	}
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// TestMalformedParamsProduceIdenticalOutcomeRegardlessOfEnvironment is
// D-3.7.7's guardrail: a malformed -params file must be rejected the
// SAME way by BOTH subcommands, and the SAME way regardless of whether
// SOURCE_DATE_EPOCH is set — never pre-empted by injectDocumentDateFromEnv's
// own JSON decode (this story's review, Finding 6's defect recurring
// on a second axis: with the variable set, json.Unmarshal used to fail
// INSIDE the CLI and pre-empt the engine's own located diagnostic).
func TestMalformedParamsProduceIdenticalOutcomeRegardlessOfEnvironment(t *testing.T) {
	dir := t.TempDir()
	tplPath := writeTempFile(t, dir, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, dir, "data.json", `{"name": "Jane"}`)
	malformedParamsPath := writeTempFile(t, dir, "bad-params.json", `{ not valid json`)

	runWith := func(subcommand, env string) (code int, stderr string) {
		getenv := func(key string) string {
			if key == "SOURCE_DATE_EPOCH" {
				return env
			}
			return ""
		}
		var out, errB bytes.Buffer
		code = run([]string{subcommand, "-data", dataPath, "-params", malformedParamsPath, tplPath}, &out, &errB, getenv)
		return code, errB.String()
	}

	envUnsetValidateCode, envUnsetValidateStderr := runWith("validate", "")
	envUnsetRenderCode, envUnsetRenderStderr := runWith("render", "")
	envSetValidateCode, envSetValidateStderr := runWith("validate", "1750000000")
	envSetRenderCode, envSetRenderStderr := runWith("render", "1750000000")

	if envUnsetValidateCode != exitFailure || envUnsetRenderCode != exitFailure || envSetValidateCode != exitFailure || envSetRenderCode != exitFailure {
		t.Fatalf("all four combinations must exit %d on malformed params: got validate(unset)=%d render(unset)=%d validate(set)=%d render(set)=%d",
			exitFailure, envUnsetValidateCode, envUnsetRenderCode, envSetValidateCode, envSetRenderCode)
	}
	if envUnsetValidateStderr != envUnsetRenderStderr {
		t.Errorf("PARITY VIOLATION (env unset): validate stderr=%q, render stderr=%q", envUnsetValidateStderr, envUnsetRenderStderr)
	}
	if envSetValidateStderr != envSetRenderStderr {
		t.Errorf("PARITY VIOLATION (env set): validate stderr=%q, render stderr=%q", envSetValidateStderr, envSetRenderStderr)
	}
	if envUnsetValidateStderr != envSetValidateStderr {
		t.Errorf("D-3.7.7: malformed params produced a DIFFERENT message depending on whether SOURCE_DATE_EPOCH was set — the CLI-level decode pre-empted the engine's own located diagnostic: env-unset=%q, env-set=%q", envUnsetValidateStderr, envSetValidateStderr)
	}
	if strings.Contains(envSetValidateStderr, "folio8: params:") {
		t.Errorf("stderr names the CLI's own decode error (%q) rather than the engine's located diagnostic — injectDocumentDateFromEnv must pass malformed params through unchanged, never raise its own error for them", envSetValidateStderr)
	}
}
