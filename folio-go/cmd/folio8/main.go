// Command folio8 is Story 3.7's command-line tool (FR42, AD-7): a small
// wrapper offering two subcommands, "validate" and "render" — nothing
// else (scope fence: no watch mode, no config file, no plugins, no
// packaging, no shell completion). It composes package folio8 with
// folio8/fonts (R3: "the shell composes; the library does not") and is
// the ONE place under folio-go/ that may read SOURCE_DATE_EPOCH (R1,
// R2, D-3.7.2) — package folio8's own render path reads no environment
// variable, ever (FR43, NFR1.f), and this file is what honours the
// convention on the library's behalf, exactly like any other caller
// would: it reads the value and passes it in as an ordinary parameter.
//
// This file is stdlib-only (R10): no flag-parsing dependency, no CLI
// framework — the standard library's "flag" package is enough for two
// subcommands and one switch.
//
// SOURCE_DATE_EPOCH precedence (D-3.7.6, this story's review, Findings
// 5 and 6 — resolved by the engineering lead's ruling): SOURCE_DATE_EPOCH
// FILLS an absent documentDate; it NEVER overwrites one the caller
// supplied through -params, regardless of that value (an explicit
// `null` counts as supplied — presence, not truthiness, is the test).
// The reproducible-builds convention defines SOURCE_DATE_EPOCH as a
// replacement for "the current date and time" — and folio8 never reads
// the current time in the first place (AD-7 omits the date unless one
// arrives through params), so there is no "current time" for the
// variable to override. Its only coherent role here is to supply a
// date where the caller supplied none. An explicit documentDate the
// caller passed already satisfies the reproducibility SOURCE_DATE_EPOCH
// exists to protect (the build is still bit-reproducible), so silently
// deferring to it raises no diagnostic — CONTRAST the missing-glyph
// Warning, where the property the mechanism protects (the customer's
// name is intact) genuinely IS violated with no signal.
//
// Both subcommands see IDENTICAL inputs (D-3.7.6 part 2): resolveInputs
// below is the ONE input-assembly step both runValidate and runRender
// call, so "folio8 validate" can never accept an environment combination
// "folio8 render" would refuse (this story's review, Finding 6) —
// nothing between flag parsing and the Validate/Render dispatch may
// diverge by subcommand.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// Exit codes (D-3.7.4, AC4): 0 success (warnings may have been
// printed); 1 validation/render failure, or --strict with warnings; 2
// a USAGE error (unknown subcommand, missing argument, or — D-3.7.6
// part 3 — a malformed SOURCE_DATE_EPOCH, which is squarely "you
// called me wrong" rather than "your template is bad") — kept distinct
// from 1 so a CI failure is triageable.
const (
	exitOK       = 0
	exitFailure  = 1
	exitUsageErr = 2
)

// subcommandNames is the test-owned literal (D-3.7.6's guardrail)
// naming every subcommand run's switch dispatches to, asserted
// AST-equal to the case set of `switch args[0]` below by
// TestSubcommandNamesMatchRunSwitch (main_test.go) — so a THIRD
// subcommand added to that switch without updating this slice reds
// there, rather than silently falling outside the parity table that
// asserts validate and render see identical inputs.
var subcommandNames = []string{"validate", "render"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

// run is main's testable core: every world-reading effect (argv,
// stdout, stderr, environment) arrives as a parameter, so tests drive
// it without a subprocess where a subprocess adds nothing (AC3-AC7);
// AC8's SOURCE_DATE_EPOCH assertion and AC11's fourth case instead run
// this binary out-of-process (cmd/folio8's own _test.go), because what
// they must prove is that THIS ENTRY POINT — os.Args/os.Getenv, wired
// in main above — reads the child process's real environment, which a
// getenv func parameter passed directly would not exercise.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitUsageErr
	}

	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr, getenv)
	case "render":
		return runRender(args[1:], stdout, stderr, getenv)
	default:
		printUsage(stderr)
		return exitUsageErr
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: folio8 <validate|render> [flags] <template.folio>")
	fmt.Fprintln(w, "  validate [-data <path>] [-params <path>] [-strict] <template.folio>")
	fmt.Fprintln(w, "  render [-data <path>] [-params <path>] [-o <path>] [-strict] <template.folio>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "SOURCE_DATE_EPOCH, if set, supplies the reserved params key \"documentDate\"")
	fmt.Fprintln(w, "(both /CreationDate and /ModDate) WHEN NO ROUTE HAS ALREADY SUPPLIED ONE.")
	fmt.Fprintln(w, "An explicit -params documentDate is never overwritten by the environment.")
	fmt.Fprintln(w, "-strict makes any Warning (e.g. a dropped, unsupported character) exit")
	fmt.Fprintln(w, "non-zero; off by default, because a Warning alone must not fail a build.")
}

// usageInputError marks an input-assembly failure that is a USAGE
// error (D-3.7.6 part 3) rather than "your template is bad": a
// malformed SOURCE_DATE_EPOCH is something the CALLER of the CLI got
// wrong (an environment variable), squarely exit 2's own criterion.
type usageInputError struct{ err error }

func (u *usageInputError) Error() string { return u.err.Error() }
func (u *usageInputError) Unwrap() error { return u.err }

// loadJSONOrEmpty reads path (if non-empty) and returns its bytes; an
// empty path returns "{}" — "no data/params file supplied" and "an
// empty object" are the same thing to folio8.Render/folio8.Validate
// (AC16's own premise, Story 1.7).
func loadJSONOrEmpty(path string) ([]byte, error) {
	if path == "" {
		return []byte("{}"), nil
	}
	return os.ReadFile(path)
}

// resolveInputs is the ONE input-assembly step BOTH subcommands call
// (D-3.7.6 part 2, this story's review Finding 6): it reads the
// template and data/params files and honours SOURCE_DATE_EPOCH
// identically regardless of which subcommand is running. Before this,
// only runRender saw getenv at all, so `folio8 validate` could accept
// an env/params combination `folio8 render` would refuse.
func resolveInputs(templatePath, dataPath, paramsPath string, getenv func(string) string) (tplBytes []byte, data folio8.Data, params folio8.Params, err error) {
	tplBytes, err = os.ReadFile(templatePath)
	if err != nil {
		return nil, nil, nil, err
	}
	dataBytes, err := loadJSONOrEmpty(dataPath)
	if err != nil {
		return nil, nil, nil, err
	}
	paramsBytes, err := loadJSONOrEmpty(paramsPath)
	if err != nil {
		return nil, nil, nil, err
	}
	paramsBytes, err = injectDocumentDateFromEnv(paramsBytes, getenv)
	if err != nil {
		return nil, nil, nil, err
	}
	return tplBytes, folio8.Data(dataBytes), folio8.Params(paramsBytes), nil
}

// printDiagnostics is DW-17's first discharge (D-3.7.4, AC6): every
// Diagnostic the caller receives is printed to stderr, one line each,
// naming severity, the stable code, the element id (where present) and
// the message — the only record a human reading a CI log has of a
// missing-glyph Warning, which has NO in-band signal anywhere in the
// produced PDF (D-3.6.8). A clean run (zero diagnostics) prints
// nothing (AC6's negative control).
func printDiagnostics(w io.Writer, diags []folio8.Diagnostic) {
	for _, d := range diags {
		sev := "WARNING"
		switch d.Severity {
		case folio8.SeverityError:
			sev = "ERROR"
		case folio8.SeverityWarning:
			sev = "WARNING"
		default:
			// Nit 3 (this story's review): a Diagnostic carrying the
			// zero Severity value is itself a bug (DW-18 makes
			// severityUnset a distinct value from Warning) — naming
			// the raw numeric value here is louder than silently
			// printing "WARNING" for something that is not one.
			sev = fmt.Sprintf("SEVERITY(%d)", d.Severity)
		}
		loc := d.ElementID
		if loc == "" {
			loc = "-"
		}
		fmt.Fprintf(w, "%s %s element=%s: %s\n", sev, d.Code, loc, d.Message)
	}
}

func exitCodeForInputError(err error) int {
	var uerr *usageInputError
	if errors.As(err, &uerr) {
		return exitUsageErr
	}
	return exitFailure
}

func runValidate(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fset := flag.NewFlagSet("validate", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	dataPath := fset.String("data", "", "path to a JSON report data file")
	paramsPath := fset.String("params", "", "path to a JSON params file")
	strict := fset.Bool("strict", false, "exit non-zero if any Warning is present")
	if err := fset.Parse(args); err != nil {
		// QA Nit 7 (this story's review): -h/--help is ALSO
		// flag.ErrHelp, a usage request rather than a usage MISTAKE —
		// printing flag's own "flag: help requested" ahead of the usage
		// block reads as an error to the CI-triage audience D-3.7.4
		// names, for a request that was answered correctly. Skip that
		// one line for ErrHelp specifically; every other parse error
		// (an unknown flag, a bad value) still prints it.
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, err)
		}
		printUsage(stderr)
		return exitUsageErr
	}
	if fset.NArg() != 1 {
		fmt.Fprintln(stderr, "validate: exactly one <template.folio> argument is required")
		printUsage(stderr)
		return exitUsageErr
	}
	templatePath := fset.Arg(0)

	tplBytes, data, params, err := resolveInputs(templatePath, *dataPath, *paramsPath, getenv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitCodeForInputError(err)
	}

	diags, verr := folio8.Validate(tplBytes, data, params, fonts.Shipped())
	if verr != nil {
		fmt.Fprintln(stderr, verr)
		return exitFailure
	}
	printDiagnostics(stderr, diags)
	if *strict && len(diags) > 0 {
		return exitFailure
	}
	return exitOK
}

func runRender(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fset := flag.NewFlagSet("render", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	dataPath := fset.String("data", "", "path to a JSON report data file")
	paramsPath := fset.String("params", "", "path to a JSON params file")
	outPath := fset.String("o", "", "output PDF path (default: stdout)")
	strict := fset.Bool("strict", false, "exit non-zero if any Warning is present")
	if err := fset.Parse(args); err != nil {
		// QA Nit 7 (this story's review): -h/--help is ALSO
		// flag.ErrHelp, a usage request rather than a usage MISTAKE —
		// printing flag's own "flag: help requested" ahead of the usage
		// block reads as an error to the CI-triage audience D-3.7.4
		// names, for a request that was answered correctly. Skip that
		// one line for ErrHelp specifically; every other parse error
		// (an unknown flag, a bad value) still prints it.
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, err)
		}
		printUsage(stderr)
		return exitUsageErr
	}
	if fset.NArg() != 1 {
		fmt.Fprintln(stderr, "render: exactly one <template.folio> argument is required")
		printUsage(stderr)
		return exitUsageErr
	}
	templatePath := fset.Arg(0)

	tplBytes, data, params, err := resolveInputs(templatePath, *dataPath, *paramsPath, getenv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitCodeForInputError(err)
	}
	tpl, err := folio8.ParseTemplate(tplBytes)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitFailure
	}

	res, rerr := folio8.Render(tpl, data, params, fonts.Shipped())
	if rerr != nil {
		fmt.Fprintln(stderr, rerr)
		return exitFailure
	}
	printDiagnostics(stderr, res.Diagnostics)

	if *outPath == "" {
		// D-3.7.4: when bytes go to stdout, NOTHING else may — no
		// progress, no summary, no banner. Diagnostics already went to
		// stderr above, never here.
		if _, werr := stdout.Write(res.Bytes); werr != nil {
			fmt.Fprintln(stderr, werr)
			return exitFailure
		}
	} else {
		if werr := os.WriteFile(*outPath, res.Bytes, 0o644); werr != nil {
			fmt.Fprintln(stderr, werr)
			return exitFailure
		}
	}

	if *strict && len(res.Diagnostics) > 0 {
		return exitFailure
	}
	return exitOK
}

// injectDocumentDateFromEnv reads SOURCE_DATE_EPOCH via getenv and, if
// set AND params does not already carry a "documentDate" key, decodes
// params, sets "documentDate" to the epoch's RFC 3339 form, and
// re-encodes the result.
//
// D-3.7.6 (this story's review, Findings 5 and 6): FILL-ONLY. A
// documentDate key the caller supplied through -params is NEVER
// overwritten — regardless of its value, including an explicit
// `null`. Presence, not truthiness, is the test: if the key is merely
// present with a non-string value, this function still leaves it
// alone and Validate/Render itself raises the located
// DiagCodeDocumentDateInvalid diagnostic, which is the correct
// outcome (the caller supplied something, and what they supplied is
// wrong) — very different from the environment silently supplying a
// different value than the one the caller wrote.
//
// D-3.7.7 (this story's review, a THIRD defect the engineering lead
// found reading this function): this must never become the reason a
// malformed -params file is rejected, and must never normalise a
// well-formed one it has nothing to add to.
//   - SOURCE_DATE_EPOCH absent: return params UNCHANGED, byte for
//     byte, without even attempting to parse them — there is nothing
//     to inject, so there is no reason to touch the caller's bytes at
//     all (this also means a malformed -params file, with no
//     SOURCE_DATE_EPOCH set, always reaches Validate/Render exactly as
//     written, and gets FR41's located diagnostic over the caller's
//     own byte offsets).
//   - params fail to decode as a JSON object: also return the
//     ORIGINAL bytes UNCHANGED, with no error from THIS function —
//     Validate/Render is what rejects malformed params, with a
//     located diagnostic, never this CLI-level decode. Before this
//     fix, a malformed -params file was rejected differently
//     depending on whether SOURCE_DATE_EPOCH happened to be set: with
//     it set, json.Unmarshal failed HERE and the caller got a bare
//     "folio8: params: ..." message pre-empting the engine's diagnostic
//     (this story's review, Finding 6's defect recurring on a second
//     axis); without it, the same bytes reached the engine untouched.
//     The CLI's job is to add a key, never to validate JSON.
//   - documentDate already present: return params UNCHANGED (fill-only,
//     above) — no re-marshal, so a params-located diagnostic's byte
//     offsets still refer to the caller's own file, not a
//     re-serialised, whitespace-normalised copy of it.
//   - otherwise: parse the epoch; a non-integer value is a
//     usageInputError (D-3.7.6 part 3, exit 2) — this IS this
//     function's own error to raise, because a malformed
//     SOURCE_DATE_EPOCH with nothing else to fall back on is squarely
//     "you called me wrong".
func injectDocumentDateFromEnv(params []byte, getenv func(string) string) ([]byte, error) {
	raw := getenv("SOURCE_DATE_EPOCH")
	if raw == "" {
		return params, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(params, &obj); err != nil {
		// D-3.7.7: not this function's error to raise — let
		// Validate/Render reject the malformed params themselves, over
		// the caller's own bytes.
		return params, nil
	}
	if _, present := obj["documentDate"]; present {
		// D-3.7.6: fill-only. A key the caller supplied — including an
		// explicit null — is never displaced by the environment.
		return params, nil
	}

	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, &usageInputError{fmt.Errorf("folio8: SOURCE_DATE_EPOCH=%q is not an integer: %w", raw, err)}
	}
	rfc3339 := time.Unix(epoch, 0).UTC().Format(time.RFC3339)

	if obj == nil {
		obj = map[string]json.RawMessage{}
	}
	encodedDate, err := json.Marshal(rfc3339)
	if err != nil {
		return nil, err
	}
	obj["documentDate"] = encodedDate

	return json.Marshal(obj)
}
