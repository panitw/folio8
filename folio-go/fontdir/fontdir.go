// Package fontdir builds a folio8.FontSet from a directory of font
// files on a host's own disk.
//
// IT IS A HOST-SIDE HELPER, AND THAT IS THE WHOLE POINT. The engine
// still never reads the filesystem: folio8.FontSet remains its only font
// input, and this package does nothing but produce one. An integrator
// who drops a licensed brand typeface into a directory on a render
// server gets a FontSet out of it; package folio8 is untouched and never
// learns that a directory was involved.
//
// It is its OWN package rather than part of folio8 or of folio8/fonts,
// deliberately and on both counts. Putting it in fonts would force a
// consumer who wants only a disk loader to pull in roughly 14.8 MB of
// embedded faces, which is the exact cost fonts.go's one-directional
// import rule exists to avoid. Putting it in folio8 would put
// filesystem-reading code in the engine's own package, which is the one
// property the FontSet contract promises is absent.
//
// MERGING IS THE CALLER'S BUSINESS. Set returns this directory's faces
// and nothing else — there is no merge helper here and no precedence
// rule invented here. A caller who wants the shipped faces underneath
// writes the two lines the repo already writes everywhere else:
//
//	faces := fonts.Shipped()
//	extra, skipped, err := fontdir.Set("/srv/fonts")
//	if err != nil {
//		return err
//	}
//	maps.Copy(faces, extra) // second wins, as everywhere in this repo
//
// WHAT IT WILL NOT DO, named here so no reader has to discover it: it
// does not recurse into subdirectories, does not read the operating
// system's font book, and touches no network. The directory an
// integrator names is the whole of the source.
//
// A SYMLINK IS NEVER FOLLOWED, INCLUDING ONE POINTING INSIDE THE
// DIRECTORY ITSELF. Only regular files are read. The frozen boundary
// forbids following a link OUT of the directory, and distinguishing the
// two cases means resolving a target and deciding whether it is "inside"
// — a question with no cheap, correct answer once a directory is itself
// a link, or is relocated between deploys. The stricter rule is taken
// instead, and a symlink whose name looks like a face is REPORTED as a
// skip rather than silently dropped, so an operator who staged fonts by
// linking them sees exactly why they are missing and can copy them.
package fontdir

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// Skipped is one font file the directory held and the returned FontSet
// does not carry, with the reason it was left out.
//
// A SKIP IS REPORTED, NEVER SILENT, AND NEVER FATAL. A directory with
// one corrupt .ttf in it is the normal case on a real server — a partial
// download, a file half-written by a deploy script, a variable build
// somebody grabbed from a foundry's website. Failing the whole call
// would make one bad file stop every render; saying nothing would make
// a missing brand face look like a document bug months later. So every
// candidate that does not become a key comes back here, and the caller
// decides whether to log it, ignore it, or fail their own build on it.
type Skipped struct {
	// File is the path of the file that was skipped, under the
	// directory as the caller named it.
	File string
	// Reason says what was wrong, in one sentence, already carrying
	// whatever the underlying validator said.
	Reason string
}

// String is the skip as one log line.
func (s Skipped) String() string { return s.File + ": " + s.Reason }

// fontExtensions are the file extensions Set treats as CANDIDATES, and
// the media type each one declares to template.DecodeFontForRender.
//
// Everything else in the directory — a LICENSE, a README, a .txt, a
// subdirectory — is IGNORED, and ignored is not the same as skipped: it
// never appears in the returned []Skipped. A font directory that also
// holds the licences those fonts ship with is an ordinary, correct font
// directory, and reporting its NOTICE file as a fault would train a
// reader to stop reading the report.
var fontExtensions = map[string]string{
	".ttf": "font/ttf",
	".otf": "font/otf",
}

// nearMissExtensions are the file shapes that ARE fonts and still are
// not candidates. They are reported rather than ignored, which is the
// difference between the two maps: a NOTICE file in a font directory is
// somebody being tidy, but a .woff sitting where a face was expected is
// somebody's brand face that will simply not appear, and silence there
// is how an operator ends up staring at an empty FontSet.
var nearMissExtensions = map[string]string{
	".woff":  "is a web font (.woff); only .ttf and .otf faces are read — convert it to an sfnt face first",
	".woff2": "is a web font (.woff2); only .ttf and .otf faces are read — convert it to an sfnt face first",
	".ttc":   "is a font collection (.ttc); only single-face .ttf and .otf files are read — extract the face you want first",
	".otc":   "is a font collection (.otc); only single-face .ttf and .otf files are read — extract the face you want first",
}

// isControl reports whether a name record carries a C0 or C1 control
// character or a DEL. Such a record is not a family name anybody can
// type into a chain entry, and letting one become a FontSet key would
// put an unprintable string into a document's `fonts` object and into
// every error message that quotes it.
func isControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return true
		}
	}
	return false
}

// Set reads dir and returns the faces it holds, keyed by what each
// binary says its own name is, together with every candidate that was
// left out and why.
//
// THE KEY COMES FROM THE BINARY, NEVER FROM THE FILENAME. It is the
// face's own `name` table record 1 (the family), plus record 2 (the
// subfamily) when that is anything other than "Regular", joined by a
// space — so a file called Sarabun-Bold.ttf whose name table says
// family "Sarabun", subfamily "Bold" is keyed "Sarabun Bold", and
// renaming that file on disk does not change its key. This is the same
// shape fonts.Shipped()'s own keys have ("Noto Sans", "Noto Sans Bold"),
// so a chain entry cannot tell a disk face from a shipped one.
//
// WHY NOT THE FILENAME, WHICH WOULD BE SIMPLER. fonts.go forbids
// deriving a family from a FontSet key at all — "the machine-readable
// family is the face's own sfnt name ID 1" (D-B) — and trusting a
// filename is that same mistake one layer out, with the same
// consequence: two files a human renamed become two different faces, or
// one face silently shadows another.
//
// A missing directory is an error naming the path. An EMPTY directory
// is not: it returns an empty FontSet and no error, because an empty
// font set has been a legal input to a render since the engine stopped
// requiring one.
//
// TWO FILES THAT PRODUCE THE SAME KEY resolve deterministically — the
// first in lexical filename order wins, and the other comes back as a
// skip. Determinism matters more than which one wins: a render must not
// change its output because a directory listing came back in a
// different order.
func Set(dir string) (folio8.FontSet, []Skipped, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("fontdir: read directory %s: %w", dir, err)
	}

	// os.ReadDir already sorts by filename; sorting the candidate names
	// again makes the "first in lexical order wins" rule above a
	// property of THIS function rather than of a documented behaviour
	// somewhere else that a future reader would have to go and check.
	var skipped []Skipped
	report := func(path, format string, args ...any) {
		skipped = append(skipped, Skipped{File: path, Reason: fmt.Sprintf(format, args...)})
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		_, candidate := fontExtensions[ext]
		nearMiss, shaped := nearMissExtensions[ext]
		// A subdirectory is not descended into and a symlink is not
		// followed — both by asking for a REGULAR FILE rather than by
		// excluding the two shapes we happened to think of. A
		// subdirectory is ignored outright; a LINK whose name looks like
		// a face is reported, because that one is somebody's staged font
		// silently going missing.
		if !entry.Type().IsRegular() {
			if entry.Type()&os.ModeSymlink != 0 && (candidate || shaped) {
				report(filepath.Join(dir, entry.Name()), "is a symbolic link; only regular files are read — copy the face into the directory instead")
			}
			continue
		}
		if !candidate {
			if shaped {
				report(filepath.Join(dir, entry.Name()), "%s", nearMiss)
			}
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	set := folio8.FontSet{}
	// keyedFrom remembers which file claimed each key, so the duplicate
	// report can name BOTH files rather than only the loser — and it
	// stores the same spelling Skipped.File carries, so one report line
	// never names the same directory two different ways.
	keyedFrom := map[string]string{}
	for _, name := range names {
		path := filepath.Join(dir, name)
		skip := func(format string, args ...any) {
			report(path, format, args...)
		}

		data, rerr := os.ReadFile(path)
		if rerr != nil {
			skip("cannot be read: %v", rerr)
			continue
		}

		// THE VALIDATION IS THE RENDERER'S OWN, NOT A SECOND OPINION
		// WRITTEN HERE, and it is the same pair, in the same order, that
		// the designer's embed command runs over a face an author picks
		// (component_commands.go): DecodeFontForRender answers "can this
		// build read these bytes as a single face", and RefuseVariableFace
		// answers the one class that check structurally cannot see,
		// carrying the fonttools remedy in its own message.
		mediaType := fontExtensions[strings.ToLower(filepath.Ext(name))]
		if derr := template.DecodeFontForRender(mediaType, data, template.FontChainSite{AssetKey: name}); derr != nil {
			skip("is not a font this build can read: %v", derr)
			continue
		}
		if verr := fontset.RefuseVariableFace(name, data); verr != nil {
			skip("%v", verr)
			continue
		}

		// The full ingestion the renderer performs — the validated
		// unitsPerEm and the presence of every table folio8 reads — run
		// here so that the classes IT catches are caught while there is a
		// caller holding a report to put them in. It is not a render:
		// a face with no cmap or no outlines still keys clean here and
		// fails at the point of use, which is why the guide promises a
		// readable FACE, never a drawable one.
		face, nerr := fontset.New(name, data)
		if nerr != nil {
			skip("%v", nerr)
			continue
		}

		family := strings.TrimSpace(face.Family())
		if family == "" {
			skip("declares no family name (`name` table record 1), so it cannot be keyed")
			continue
		}
		subfamily := strings.TrimSpace(face.Subfamily())
		if isControl(family) || isControl(subfamily) {
			skip("declares a family or subfamily carrying control characters, which cannot be a face name")
			continue
		}
		key := faceKey(family, subfamily)
		if first, taken := keyedFrom[key]; taken {
			skip("names the same face %q as %s, which was taken first", key, first)
			continue
		}
		keyedFrom[key] = path
		set[key] = data
	}
	return set, skipped, nil
}

// faceKey joins a family and a subfamily into the FontSet key shape
// fonts.Shipped() already uses: the bare family for the Regular cut, and
// "<family> <subfamily>" for every other one.
//
// A subfamily of "Regular" is dropped rather than appended, which is
// what makes the Regular cut of Sarabun key as "Sarabun" and not as
// "Sarabun Regular" — the shape a human writes in a chain. A face whose
// subfamily record is ABSENT is treated the same way: an unstated cut is
// the family's default cut, and there is nothing to append.
//
// This is a JOIN, in one direction only. Nothing anywhere may split a
// key back into its parts (D-B); a key is produced here and thereafter
// read as an opaque string.
func faceKey(family, subfamily string) string {
	if subfamily == "" || strings.EqualFold(subfamily, "Regular") {
		return family
	}
	return family + " " + subfamily
}
