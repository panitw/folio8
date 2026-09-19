package bind

import (
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// testFormatContext is the ordinary `en`/UTC formatting context this
// package's own tests thread through BindText/BindTextSpans/Resolve
// (Story 3.4's mandatory FormatContext parameter, R1) whenever the
// locale is not the point of the test. The zero value is deliberately
// unusable (expr's own AC5b), so it is never used here as a stand-in
// for "don't care".
func testFormatContext() expr.FormatContext {
	return expr.NewFormatContext(template.LocaleEN, "+00:00")
}
