// Package producer stands in for a real folio-go/internal/ package (like
// internal/geom) that declares a named map type or type alias and a
// function returning one — the exact shape Blocker 1 (D-1.3.11, this
// story's QA review) proved was invisible to the dependency-free
// tolerantImporter path: a type declared in one package and ranged in
// another, which internal/pdf importing internal/geom makes a live
// hazard, not a hypothetical. Never compiled as part of any real build —
// testdata/ is invisible to the go command (AC2) — but IS visible to
// ScanMapRange, which loads it with golang.org/x/tools/go/packages
// specifically so this cross-package shape resolves.
package producer

// ScaleTable is a named map type, exactly like a plausible future
// internal/geom.ScaleTable.
type ScaleTable map[string]int

// NewScaleTable stands in for a map-returning constructor in another
// package.
func NewScaleTable() ScaleTable { return ScaleTable{} }

// Alias is a type alias to a map type — the third shape the QA review
// measured missed (a named map type and a function's map return value
// are the other two, both exercised via ScaleTable above).
type Alias = map[string]int

// NewAlias stands in for a map-returning constructor whose return type
// is a type alias rather than a named type.
func NewAlias() Alias { return Alias{} }
