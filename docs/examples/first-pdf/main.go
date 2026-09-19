// Command first-pdf renders docs/examples/first-pdf.folio to first-pdf.pdf.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

func main() {
	tpl, err := folio8.LoadTemplate("first-pdf.folio")
	if err != nil {
		log.Fatal(describe("load", err))
	}

	data, err := os.ReadFile("first-pdf.data.json")
	if err != nil {
		log.Fatal(err)
	}

	res, err := folio8.Render(tpl, folio8.Data(data), nil, fonts.Shipped())
	if err != nil {
		log.Fatal(describe("render", err))
	}
	for _, d := range res.Diagnostics {
		// Warnings accompany a successful render. Dispatch on d.Code, never on d.Message.
		fmt.Fprintf(os.Stderr, "%s %s element=%q path=%q: %s\n", d.Severity, d.Code, d.ElementID, d.DataPath, d.Message)
	}

	if err := os.WriteFile("first-pdf.pdf", res.Bytes, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote first-pdf.pdf (%d bytes, %d warnings)\n", len(res.Bytes), len(res.Diagnostics))
}

// describe adds the stable diagnostic code when err carries one.
func describe(stage string, err error) string {
	var re *folio8.RenderError
	if errors.As(err, &re) {
		d := re.Diagnostic
		return fmt.Sprintf("%s failed: %s element=%q path=%q: %v", stage, d.Code, d.ElementID, d.DataPath, err)
	}
	return fmt.Sprintf("%s failed: %v", stage, err)
}
