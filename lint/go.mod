module github.com/panitw/folio8/lint

// See folio-go/go.mod for why the go directive sits below the toolchain
// pin: a `go` line that is not strictly lower than `toolchain` gets its
// toolchain pin stripped by `go mod tidy` (D-1.1.a). This module matches
// folio-go's and hashmatrix's floor and pin exactly, for the same reason.
go 1.25.0

toolchain go1.26.0

// D-1.3.6 named golang.org/x/tools/go/packages as this module's "expected"
// dependency, while permitting "an exact, dependency-free go/types path" as
// "a permitted refinement, not a deviation" — but only if that path met
// invariant (a), exact detection with no false positives. D-1.3.11 (Story
// 1.3's QA review, Blocker 1) found the shipped dependency-free path was
// not exact — it silently missed a map type declared in one internal/
// package and ranged in another, which is a live shape today
// (internal/pdf already imports internal/geom) — so the refinement's
// precondition was not met and the require below returns to D-1.3.6's
// expected shape. Invariant (b), "no dependency added to folio-go's
// module graph", still holds: this is lint's own graph, and folio-go's
// go.mod is untouched.

require golang.org/x/tools v0.49.0

require (
	golang.org/x/mod v0.39.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
)
