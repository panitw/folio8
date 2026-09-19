// Package pdf is a COMPLIANT fixture covering the arrow this story
// actually adds: pdf(8) importing pagemodel(1). AD-5 forbids
// layout -> pdf; it says nothing against pdf -> pagemodel, and the
// table must not accidentally forbid it.
package pdf

import "github.com/panitw/folio8/folio-go/internal/pagemodel"

func Emit(p pagemodel.Page) { _ = p }
