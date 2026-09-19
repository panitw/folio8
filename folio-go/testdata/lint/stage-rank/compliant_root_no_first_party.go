// Package arch stands in for the scan root's own directory — ranked
// rankNoStage, "not a pipeline stage". A file there importing no
// first-party internal package is compliant, which is exactly what the
// real folio-go/internal/'s test-only arch package does today.
package arch

import "strings"

func Trim(s string) string { return strings.TrimSpace(s) }
