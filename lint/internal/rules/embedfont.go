package rules

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RuleEmbedFont is this guard's stable rule id (AC3, matching the other
// guards' "by file and rule, never by count" convention).
const RuleEmbedFont = "embed-font"

// fontFileExtensions are the binary font formats a go:embed directive
// naming a font file could target. Illustrative, not binding — AC3's
// property is "no go:embed directive naming a font file", not this
// specific extension list; extending it costs nothing.
var fontFileExtensions = []string{".ttf", ".otf", ".ttc", ".woff", ".woff2"}

// EmbedFontStats reports what ScanEmbedFont actually examined, from the
// scanner's own execution — the same D-000.9 reasoning every other
// guard in this package follows (MapRangeStats, noFloat64Stats): a
// second, independently-derived re-walk cannot see a
// scanner that silently does nothing.
type EmbedFontStats struct {
	FilesParsed int
	DirsVisited []string
}

// ScanEmbedFont is AC3's guard: "no go:embed directive naming a font
// file exists anywhere under folio-go/internal/" (AD-8's Rule,
// verbatim). Detection is line-based rather than AST-based — a
// //go:embed directive is always a full comment line immediately
// preceding the declaration it attaches to, and the property this guard
// polices is textual (a directive naming a font-extensioned path), not
// structural — but every .go file this walk visits (including
// _test.go: AD-8's "no package under internal/ embeds font data" names
// no test exemption, unlike D-1.3.5's map-range ban) is still parsed
// line by line, and the walk reports a coverage witness so "zero
// candidate files" is distinguishable from "a healthy scan of a real
// tree" (D-000.9).
func ScanEmbedFont(root string) ([]Finding, EmbedFontStats, error) {
	var findings []Finding
	var stats EmbedFontStats
	dirsSeen := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		stats.FilesParsed++
		dir := filepath.ToSlash(filepath.Dir(rel))
		if !dirsSeen[dir] {
			dirsSeen[dir] = true
			stats.DirsVisited = append(stats.DirsVisited, dir)
		}

		fileFindings, scanErr := scanFileForFontEmbeds(path, rel)
		if scanErr != nil {
			return scanErr
		}
		findings = append(findings, fileFindings...)
		return nil
	})
	if err != nil {
		return nil, EmbedFontStats{}, err
	}
	return findings, stats, nil
}

// scanFileForFontEmbeds is Finding 5's rewrite (QA review): the shipped
// version was purely suffix-matching against the raw directive text, so
// it missed every one of the measured evading shapes — `//go:embed
// fonts`, `//go:embed fonts/*`, `//go:embed all:fonts`, and a quoted
// argument with an embedded space (`//go:embed "shipped face.ttf"`,
// legal Go, which strings.Fields split on the internal space and broke
// in two). Story 2.2 ships production faces via `//go:embed fonts` or
// `//go:embed all:fonts` — precisely the shape this guard could not see
// before this fix.
func scanFileForFontEmbeds(path, rel string) ([]Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileDir := filepath.Dir(path)

	var findings []Finding
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "//go:embed") {
			continue
		}
		for _, rawArg := range splitEmbedArgs(strings.TrimPrefix(line, "//go:embed")) {
			// Strip the "all:" prefix (embeds files starting with '.'/'_'
			// too — irrelevant to font detection) and surrounding quotes
			// (a go:embed argument containing whitespace must be quoted;
			// Go's own directive parser accepts this).
			cleaned := strings.TrimPrefix(rawArg, "all:")
			cleaned = strings.Trim(cleaned, `"`)

			if hasFontExtension(cleaned) {
				findings = append(findings, fontEmbedFinding(rel, lineNum, rawArg, cleaned))
				continue
			}

			// Not a direct font-extensioned filename: the argument may
			// still be a directory (a bare directory name embeds its
			// whole subtree recursively per go:embed's own semantics) or
			// a glob pattern (`fonts/*`) that resolves to one. Resolve
			// relative to the FILE'S OWN directory, matching how
			// go:embed itself resolves paths.
			matched, detail, rerr := resolveEmbedArgToFont(fileDir, cleaned)
			if rerr != nil {
				return nil, rerr
			}
			if matched {
				findings = append(findings, fontEmbedFinding(rel, lineNum, rawArg, detail))
			}
		}
	}
	if serr := scanner.Err(); serr != nil {
		return nil, serr
	}
	return findings, nil
}

func fontEmbedFinding(rel string, lineNum int, rawArg, detail string) Finding {
	return Finding{
		Path: rel,
		Rule: RuleEmbedFont,
		Line: lineNum,
		Message: fmt.Sprintf(
			"%s:%d: go:embed directive %q resolves to a font file %q — no package under "+
				"internal/ embeds font data (AD-8's Rule, verbatim; AC3)",
			rel, lineNum, rawArg, detail,
		),
	}
}

// splitEmbedArgs tokenizes the text following "//go:embed" on
// whitespace, EXCEPT inside a double-quoted argument — a bare
// strings.Fields split breaks `"shipped face.ttf"` into two tokens on
// its internal space (Finding 5).
func splitEmbedArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inQuotes := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			cur.WriteRune(r)
		case (r == ' ' || r == '\t') && !inQuotes:
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

func hasFontExtension(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range fontFileExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// resolveEmbedArgToFont resolves a go:embed argument that is NOT itself
// a font-extensioned filename, relative to fileDir (the directory of
// the .go file carrying the directive — go:embed's own resolution
// base). It reports whether the argument's target — a directory,
// walked recursively per go:embed's directory semantics, or a glob
// pattern's matches — contains at least one font-extensioned file. A
// target that does not exist on disk (e.g. an argument this guard
// cannot otherwise interpret) is reported as no match rather than an
// error: this guard's job is detection, not validating that every
// embed target exists (the Go compiler already enforces that).
func resolveEmbedArgToFont(fileDir, arg string) (matched bool, detail string, err error) {
	if strings.ContainsAny(arg, "*?[") {
		matches, gerr := filepath.Glob(filepath.Join(fileDir, arg))
		if gerr != nil {
			return false, "", gerr
		}
		for _, m := range matches {
			if hasFontExtension(m) {
				return true, m, nil
			}
			if info, statErr := os.Stat(m); statErr == nil && info.IsDir() {
				if found, detail := dirContainsFont(m); found {
					return true, detail, nil
				}
			}
		}
		return false, "", nil
	}

	full := filepath.Join(fileDir, arg)
	info, statErr := os.Stat(full)
	if statErr != nil {
		return false, "", nil
	}
	if info.IsDir() {
		matched, detail := dirContainsFont(full)
		return matched, detail, nil
	}
	return hasFontExtension(full), full, nil
}

// dirContainsFont walks dir recursively (go:embed's own semantics for a
// bare directory argument: the whole subtree, excluding names starting
// with '.' or '_') and reports the first font-extensioned file found.
func dirContainsFont(dir string) (bool, string) {
	var found string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() {
			return nil
		}
		if hasFontExtension(p) {
			found = p
		}
		return nil
	})
	return found != "", found
}
