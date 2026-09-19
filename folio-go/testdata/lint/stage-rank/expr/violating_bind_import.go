// Package expr is the retained violating fixture for D-1.6.1's
// pre-commitment — expr(3) may not import bind(4) — the second arrow
// D-000.16's table subsumes. It stopped the decimal type being
// duplicated once already; the rank makes it structural.
package expr

import "github.com/panitw/folio8/folio-go/internal/bind"

func Eval() { _ = bind.Value{} }
