package folio8_test

import (
	"bytes"
	"fmt"
	"log"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// Example demonstrates the whole path from a `.folio` template on disk to
// rendered PDF bytes: a load call, a render call, and — the one thing this
// example exists to prove — the FontSet arriving as a single, no-argument
// expression, fonts.Shipped(). There is no builder, no options struct, and
// no field-by-field assembly of font bytes: Story 2.2's shipped face set is
// simply asked for.
func Example() {
	tpl, err := folio8.LoadTemplate("testdata/example/first-pdf.folio")
	if err != nil {
		log.Fatal(err)
	}

	data := folio8.Data(`{"customer": {"name": "Ada Lovelace"}}`)
	params := folio8.Params(`{}`)

	res, err := folio8.Render(tpl, data, params, fonts.Shipped())
	if err != nil {
		log.Fatal(err)
	}
	pdfBytes := res.Bytes

	// The exact byte length is not asserted here: it is a faithful,
	// reproducible function of this template and these inputs (AD-1,
	// AD-21), but pinning that number in this example would make it
	// fail on any unrelated, legitimate change to the rendering
	// pipeline — the fixture tests under testdata/ already carry that
	// burden with a recorded SHA-256, deliberately re-recorded. `err`
	// is already known nil at this point (log.Fatal above exits on any
	// non-nil err), so asserting it again would be a vacuous conjunct —
	// printing "true" whether or not this line ever ran against a real
	// error.
	//
	// What IS asserted is that the data actually reached the page. The
	// template's text element reads "Hello, {{customer.name}}!", so the
	// bound name has to survive binding, shaping and subsetting to end
	// up in the font's glyph coverage. Before this, the template's only
	// element was the literal "Hello, World!" with no placeholder at
	// all: the customer data above bound to nothing, never reached the
	// PDF, and binding could have been entirely broken with this
	// example still printing "true".
	renderedNonEmpty := len(pdfBytes) > 0
	boundNameReachedThePDF := pdfContainsGlyphsFor(pdfBytes, "Ada Lovelace")
	fmt.Println(renderedNonEmpty && boundNameReachedThePDF)
	// Output: true
}

// pdfContainsGlyphsFor reports whether every distinct rune of s ended up
// in the rendered document's ToUnicode mapping — i.e. whether the text
// really was placed, rather than merely supplied.
//
// It reads the produced PDF rather than the inputs (D-000.21). Checking
// for the literal string would not work: text is written as Identity-H
// glyph ids, not as readable characters, so the name is present only as
// the CMap entries that map those glyphs back to Unicode — measured as
// LOWER-case hex, e.g. "<0041>" for 'A'.
func pdfContainsGlyphsFor(pdfBytes []byte, s string) bool {
	for _, r := range s {
		if r == ' ' {
			continue
		}
		if !bytes.Contains(pdfBytes, []byte(fmt.Sprintf("<%04x>", r))) {
			return false
		}
	}
	return true
}
