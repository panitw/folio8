package main

// The -fonts flag: spec-font-sources-and-embedding CAP-3 reaching the
// command line. Everything here is asserted on BOTH subcommands, from
// one table, for the reason subcommand_parity_test.go already states —
// a flag wired into render and not into validate is the defect D-3.7.6
// exists to make impossible, and it would be invisible to a test that
// exercised render alone.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/fonts"
)

// cliBrandFaceTemplateJSON names a face NO SHIPPED SET CARRIES. Rendering
// it succeeds only if the -fonts directory supplied that face.
const cliBrandFaceTemplateJSON = `{
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
  "fonts": {"body": ["Foto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// utf16be is how a Windows-platform `name` record spells a string.
func utf16be(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		out = append(out, 0, byte(r))
	}
	return out
}

// rebrandFace renames a face inside its own binary, same-length so every
// table offset stays exactly where it was. It exists so this test can
// have a face that is genuinely NOT in fonts.Shipped() without committing
// a second font binary to the repository — "Noto Sans" becomes "Foto
// Sans", nine characters for nine.
func rebrandFace(t *testing.T, data []byte, from, to string) []byte {
	t.Helper()
	if len(from) != len(to) {
		t.Fatalf("rebrand %q -> %q changes the byte length, which would move every table offset", from, to)
	}
	out := append([]byte(nil), data...)
	out = bytes.ReplaceAll(out, []byte(from), []byte(to))
	out = bytes.ReplaceAll(out, utf16be(from), utf16be(to))
	if bytes.Equal(out, data) {
		t.Fatalf("rebrand found no occurrence of %q — the fixture is not what this test thinks it is", from)
	}
	return out
}

// brandFontDir writes a directory holding one face called "Foto Sans",
// under a filename that says nothing about it.
func brandFontDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	face, ok := fonts.Shipped()["Noto Sans"]
	if !ok {
		t.Fatal("fonts.Shipped() has no Noto Sans")
	}
	if err := os.WriteFile(filepath.Join(dir, "00-brand.ttf"), rebrandFace(t, face, "Noto Sans", "Foto Sans"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestFontsDirSuppliesAFaceNoShippedSetCarries is the acceptance
// criterion itself, on both subcommands: with the directory the document
// renders clean, and without it the same document does not render at
// all — so the face really did come from the directory.
func TestFontsDirSuppliesAFaceNoShippedSetCarries(t *testing.T) {
	work := t.TempDir()
	tplPath := writeTempFile(t, work, "brand.folio", cliBrandFaceTemplateJSON)
	dataPath := writeTempFile(t, work, "data.json", `{"name": "Jane"}`)
	fontsDir := brandFontDir(t)

	for _, subcommand := range subcommandNames {
		t.Run(subcommand+" with -fonts", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{subcommand, "-data", dataPath, "-fonts", fontsDir, tplPath}, &stdout, &stderr, noEnv)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, exitOK, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty — a face supplied from disk must render with no diagnostic at all", stderr.String())
			}
			if subcommand == "render" && !bytes.HasPrefix(stdout.Bytes(), []byte("%PDF-1.7")) {
				t.Errorf("stdout does not start with a PDF header")
			}
		})
		t.Run(subcommand+" without -fonts", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{subcommand, "-data", dataPath, tplPath}, &stdout, &stderr, noEnv)
			if code == exitOK {
				t.Fatalf("exit code = %d: the document named a face nothing supplied and was accepted anyway — this test proves nothing about where the face came from", code)
			}
		})
	}
}

// TestFontsDirFaceIsKeyedByItsBinaryNotItsFilename: the same directory,
// with the file renamed, must behave identically. A loader that keyed on
// the filename stem would pass the test above and fail this one.
func TestFontsDirFaceIsKeyedByItsBinaryNotItsFilename(t *testing.T) {
	work := t.TempDir()
	tplPath := writeTempFile(t, work, "brand.folio", cliBrandFaceTemplateJSON)
	dataPath := writeTempFile(t, work, "data.json", `{"name": "Jane"}`)

	dir := brandFontDir(t)
	old := filepath.Join(dir, "00-brand.ttf")
	renamed := filepath.Join(dir, "zz-unrelated-name.ttf")
	if err := os.Rename(old, renamed); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"render", "-data", dataPath, "-fonts", dir, tplPath}, &stdout, &stderr, noEnv); code != exitOK {
		t.Fatalf("exit code = %d after renaming the file, want %d; stderr = %q", code, exitOK, stderr.String())
	}
}

// TestFontsDirSkipIsReportedOnStderrAndDoesNotFailTheRun: one corrupt
// file beside a good one is the normal case on a real server. Both
// subcommands must report it and carry on, and on render the report must
// not reach stdout, which carries only PDF bytes.
func TestFontsDirSkipIsReportedOnStderrAndDoesNotFailTheRun(t *testing.T) {
	work := t.TempDir()
	tplPath := writeTempFile(t, work, "brand.folio", cliBrandFaceTemplateJSON)
	dataPath := writeTempFile(t, work, "data.json", `{"name": "Jane"}`)
	dir := brandFontDir(t)
	if err := os.WriteFile(filepath.Join(dir, "truncated.ttf"), []byte("not a font at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, subcommand := range subcommandNames {
		t.Run(subcommand, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{subcommand, "-data", dataPath, "-fonts", dir, tplPath}, &stdout, &stderr, noEnv)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d — a corrupt file in the font directory must never fail the run; stderr = %q", code, exitOK, stderr.String())
			}
			if !strings.Contains(stderr.String(), "truncated.ttf") {
				t.Errorf("stderr does not report the skipped file: %q", stderr.String())
			}
			if subcommand == "render" {
				if !bytes.HasPrefix(stdout.Bytes(), []byte("%PDF-1.7")) {
					t.Errorf("stdout does not start with a PDF header")
				}
				if bytes.Contains(stdout.Bytes(), []byte("truncated.ttf")) {
					t.Errorf("the skip report reached STDOUT, which carries only PDF bytes")
				}
			}
		})
	}
}

// TestFontsDirMissingDirectoryFailsIdenticallyOnBothSubcommands: a
// directory an integrator named and that is not there is a deployment
// mistake, and both subcommands must say so the same way — the parity
// property, applied to the new flag.
func TestFontsDirMissingDirectoryFailsIdenticallyOnBothSubcommands(t *testing.T) {
	work := t.TempDir()
	tplPath := writeTempFile(t, work, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, work, "data.json", `{"name": "Jane"}`)
	missing := filepath.Join(work, "no-such-directory")

	stderrByName := map[string]string{}
	codeByName := map[string]int{}
	for _, subcommand := range subcommandNames {
		var stdout, stderr bytes.Buffer
		codeByName[subcommand] = run([]string{subcommand, "-data", dataPath, "-fonts", missing, tplPath}, &stdout, &stderr, noEnv)
		stderrByName[subcommand] = stderr.String()
		if stdout.Len() != 0 {
			t.Errorf("%s: stdout = %q, want empty", subcommand, stdout.String())
		}
	}
	if codeByName["validate"] != exitFailure || codeByName["render"] != exitFailure {
		t.Errorf("exit codes = validate %d, render %d, want %d for both", codeByName["validate"], codeByName["render"], exitFailure)
	}
	if stderrByName["validate"] != stderrByName["render"] {
		t.Errorf("PARITY VIOLATION: validate stderr = %q, render stderr = %q", stderrByName["validate"], stderrByName["render"])
	}
	if !strings.Contains(stderrByName["render"], missing) {
		t.Errorf("stderr does not name the missing directory: %q", stderrByName["render"])
	}
}

// TestFontsDirIsNamedInTheUsageText — the flag is discoverable, on both
// subcommand lines, wherever the usage block is printed.
func TestFontsDirIsNamedInTheUsageText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run(nil, &stdout, &stderr, noEnv)
	usage := stderr.String()
	for _, line := range strings.Split(usage, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "validate [") || strings.HasPrefix(strings.TrimSpace(line), "render [") {
			if !strings.Contains(line, "-fonts") {
				t.Errorf("usage line %q does not offer -fonts", line)
			}
		}
	}
	if !strings.Contains(usage, "-fonts") {
		t.Errorf("usage text does not mention -fonts at all: %q", usage)
	}
}

// cliShippedFaceTemplateJSON names "Roboto" — a face fonts.Shipped()
// DOES carry — which is what makes precedence observable.
const cliShippedFaceTemplateJSON = `{
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
  "fonts": {"body": ["Roboto"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestFontsDirFaceReplacesASameNamedShippedFace is the precedence the
// usage text, the README and both doc twins promise: maps.Copy, second
// wins. Inverting that one call at the merge site reds this test and
// nothing else would.
//
// The fixture is the shipped NOTO SANS binary whose family record has
// been renamed to "Roboto" (padded to the same byte length, and trimmed
// back by the loader) — so a document naming "Roboto" that renders
// through the directory paints a DIFFERENT font program than the same
// document rendered without it. Equal bytes out means the shipped face
// won.
func TestFontsDirFaceReplacesASameNamedShippedFace(t *testing.T) {
	work := t.TempDir()
	tplPath := writeTempFile(t, work, "shipped.folio", cliShippedFaceTemplateJSON)
	dataPath := writeTempFile(t, work, "data.json", `{"name": "Jane"}`)

	dir := t.TempDir()
	face, ok := fonts.Shipped()["Noto Sans"]
	if !ok {
		t.Fatal("fonts.Shipped() has no Noto Sans")
	}
	// "Noto Sans" and "Roboto   " are both nine characters, so no table
	// offset moves; fontdir trims the padding before keying.
	if err := os.WriteFile(filepath.Join(dir, "impostor.ttf"), rebrandFace(t, face, "Noto Sans", "Roboto   "), 0o644); err != nil {
		t.Fatal(err)
	}

	render := func(args ...string) []byte {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"render", "-data", dataPath}, append(args, tplPath)...), &stdout, &stderr, noEnv); code != exitOK {
			t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
		}
		return stdout.Bytes()
	}

	shippedOnly := render()
	withDirectory := render("-fonts", dir)
	if bytes.Equal(shippedOnly, withDirectory) {
		t.Error("rendering with the directory produced the SAME bytes as rendering without it — the directory's \"Roboto\" did not replace the shipped one, so the merge precedence is inverted")
	}
}

// TestFontsDirThatYieldsNoFacesIsSaidOutLoud: without this line a typo'd
// -fonts path and a correct one are indistinguishable — both render,
// both exit 0, and the brand face's absence surfaces months later.
func TestFontsDirThatYieldsNoFacesIsSaidOutLoud(t *testing.T) {
	work := t.TempDir()
	tplPath := writeTempFile(t, work, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, work, "data.json", `{"name": "Jane"}`)
	empty := t.TempDir()

	for _, subcommand := range subcommandNames {
		t.Run(subcommand, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{subcommand, "-data", dataPath, "-fonts", empty, tplPath}, &stdout, &stderr, noEnv)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d — an empty font directory is a legal configuration; stderr = %q", code, exitOK, stderr.String())
			}
			if !strings.Contains(stderr.String(), empty) {
				t.Errorf("stderr does not say the directory yielded no faces: %q", stderr.String())
			}
		})
	}
}

// TestStrictCountsSkippedFontsAndEmptyFontDirectories pins the decision
// the usage text and the README state: -strict already means "a Warning
// is a failure", and a brand face that did not load is exactly the
// deployment gap a strict build exists to catch. Without -strict both
// stay notices and the run succeeds.
func TestStrictCountsSkippedFontsAndEmptyFontDirectories(t *testing.T) {
	work := t.TempDir()
	tplPath := writeTempFile(t, work, "t.folio", cliWellFormedTemplateJSON)
	dataPath := writeTempFile(t, work, "data.json", `{"name": "Jane"}`)

	empty := t.TempDir()
	withSkip := brandFontDir(t)
	if err := os.WriteFile(filepath.Join(withSkip, "truncated.ttf"), []byte("not a font at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []struct{ label, path string }{{"an empty directory", empty}, {"a directory with a skipped file", withSkip}} {
		for _, subcommand := range subcommandNames {
			t.Run(dir.label+"/"+subcommand, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				if code := run([]string{subcommand, "-data", dataPath, "-fonts", dir.path, tplPath}, &stdout, &stderr, noEnv); code != exitOK {
					t.Fatalf("without -strict: exit code = %d, want %d; stderr = %q", code, exitOK, stderr.String())
				}
				var sout, serr bytes.Buffer
				if code := run([]string{subcommand, "-data", dataPath, "-fonts", dir.path, "-strict", tplPath}, &sout, &serr, noEnv); code != exitFailure {
					t.Errorf("with -strict: exit code = %d, want %d; stderr = %q", code, exitFailure, serr.String())
				}
			})
		}
	}
}
