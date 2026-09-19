// Package numericformattingtemplatestandin is a retained compliant
// near-miss for the numeric-formatting rule (AC8, D-1.3.2), standing in
// for what Story 1.4's internal/template/version.go will contain (F-2,
// measured at f9c27b3): an error naming the declared and supported
// versions, built with fmt.Errorf.
//
// Blocker 4 (this story's QA review): this file used to be named
// numbers.go, which hit the guard's pre-existing, unrelated
// numbers.go-filename carve-out — a fixture that would have been
// equally green before D-1.3.2's narrowing, proving only the old
// exemption and nothing about the new scope. Named version.go — the
// name Story 1.4's real file will actually have, per F-2 — and living
// under template/, OUTSIDE the fixture tree's pdf/ subdirectory, it is
// unreported for the reason that actually unblocks Story 1.4: the
// production caller's scope is exactly folio-go/internal/pdf/ (AC7), so
// this file is never reached at all. TestNumberFormattingFixtureScan
// red-proves that scope claim directly by additionally scanning the
// parent directory and asserting this file IS reported once the root
// widens to include it.
package numericformattingtemplatestandin

import "fmt"

func versionError(declared, supported string) error {
	return fmt.Errorf("template declares version %s; supported version is %s", declared, supported)
}
