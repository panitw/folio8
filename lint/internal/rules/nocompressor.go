package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

// RuleNoCompressor is this guard's stable rule id (D-000.14: findings
// carry a rule id, never a bare string match) for AC10's BINDING half:
// no file under folio-go/ may import compress/flate, compress/zlib or
// compress/gzip. D-1.8.1: "no compressor is invoked" is the mechanism
// that keeps R4 closed (`acceptance.md:83`: "compressor output is
// stable by observation, not by contract") — the PNG/JPEG passthrough
// route embeds the file's OWN already-compressed bytes, never a
// compressor's output.
const RuleNoCompressor = "no-compressor-import"

// RuleNoImageDecoder is the ILLUSTRATIVE half (AC10): no NON-TEST file
// under folio-go/ may import image, image/png or image/jpeg — reaching
// for a stdlib decoder is the concrete shape the "simpler" re-encoding
// route this story explicitly forbids would take.
const RuleNoImageDecoder = "no-image-decoder-import"

var bannedCompressorPaths = map[string]bool{
	"compress/flate": true,
	"compress/zlib":  true,
	"compress/gzip":  true,
}

var bannedImageDecoderPaths = map[string]bool{
	"image":      true,
	"image/png":  true,
	"image/jpeg": true,
}

// NoCompressorStats reports what ScanNoCompressorImports actually
// examined (D-000.9: a guard that examined zero files is a hard
// failure, not a pass, and must be distinguishable from one that
// examined everything and found nothing).
type NoCompressorStats struct {
	DirsVisited []string
	FilesSeen   int
}

// ScanNoCompressorImports walks root (intended to be folio-go/ itself,
// not just internal/ — AC10 says "no file under folio-go/", a wider
// scope than the forbidden-imports scan) and reports every import of a
// banned compressor path (in ANY file, binding) or a banned image
// decoder path (in any NON-TEST file, illustrative).
func ScanNoCompressorImports(root string) ([]Finding, NoCompressorStats, error) {
	var findings []Finding
	var stats NoCompressorStats
	dirsSeen := map[string]bool{}

	err := walkGoFiles(root, func(rel string, file *ast.File, fset *token.FileSet) error {
		stats.FilesSeen++
		dir := filepath.ToSlash(filepath.Dir(rel))
		if !dirsSeen[dir] {
			dirsSeen[dir] = true
			stats.DirsVisited = append(stats.DirsVisited, dir)
		}

		isTest := strings.HasSuffix(rel, "_test.go")

		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			pos := fset.Position(imp.Pos())

			if bannedCompressorPaths[path] {
				findings = append(findings, Finding{
					Path: rel, Rule: RuleNoCompressor, Line: pos.Line,
					Message: fmt.Sprintf(
						"%s:%d: forbidden compressor import %q — D-1.8.1's passthrough design "+
							"embeds the file's own already-compressed bytes; no compressor may be "+
							"invoked anywhere under folio-go/ (keeps R4 closed)",
						rel, pos.Line, path,
					),
				})
				continue
			}
			if !isTest && bannedImageDecoderPaths[path] {
				findings = append(findings, Finding{
					Path: rel, Rule: RuleNoImageDecoder, Line: pos.Line,
					Message: fmt.Sprintf(
						"%s:%d: forbidden image-decoder import %q in a non-test file — reaching for "+
							"a stdlib image decoder is the concrete shape the forbidden "+
							"decode-then-re-encode route takes (D-1.8.1)",
						rel, pos.Line, path,
					),
				})
			}
		}
		return nil
	})
	return findings, stats, err
}
