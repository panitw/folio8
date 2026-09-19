// Command good is the control fixture for
// TestTemplateCompositeLiteralDoesNotTypeCheck (folio-go/templateopaque_test.go,
// Epic 1 boundary gate finding): it constructs a folio8.Template only
// through the sanctioned path, ParseTemplate, and must build
// successfully. Without this control, "bad" failing to build would
// prove nothing about composite-literal construction specifically — it
// could fail for any reason (AC26 Q2's reasoning, reused here from
// Story 1.7's swap proof).
package main

import "github.com/panitw/folio8/folio-go"

const minimalTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

func main() {
	tpl, err := folio8.ParseTemplate([]byte(minimalTemplateJSON))
	if err != nil {
		panic(err)
	}
	_ = tpl
}
