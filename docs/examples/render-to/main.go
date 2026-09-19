// Command render-to renders the same document twice — once with Render and
// once with RenderTo into a file — and checks the bytes are identical.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

func main() {
	templateBytes, err := os.ReadFile("first-pdf.folio")
	if err != nil {
		log.Fatal(err)
	}
	tpl, err := folio8.ParseTemplate(templateBytes)
	if err != nil {
		log.Fatal(err)
	}
	data := folio8.Data(`{"customer": {"name": "Ada Lovelace"}}`)
	params := folio8.Params(`{}`)
	fontSet := fonts.Shipped()

	// Validate runs the same checks as a render, with the same inputs, without
	// producing a PDF.
	warnings, err := folio8.Validate(templateBytes, data, params, fontSet)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("validate: %d warnings\n", len(warnings))

	res, err := folio8.Render(tpl, data, params, fontSet)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("render-to.pdf")
	if err != nil {
		log.Fatal(err)
	}
	diagnostics, err := folio8.RenderTo(out, tpl, data, params, fontSet)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		// A failed write may already have put some bytes into the file.
		_ = os.Remove("render-to.pdf")
		log.Fatal(err)
	}
	for _, d := range diagnostics {
		fmt.Fprintf(os.Stderr, "%s %s: %s\n", d.Severity, d.Code, d.Message)
	}

	written, err := os.ReadFile("render-to.pdf")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("bytes equal:", bytes.Equal(written, res.Bytes))

	// A writer that fails is reported as an error; the PDF was fully built first.
	_, err = folio8.RenderTo(failingWriter{}, tpl, data, params, fontSet)
	fmt.Println("failing writer:", err)
	var re *folio8.RenderError
	fmt.Println("is a RenderError:", errors.As(err, &re))
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, errors.New("disk full") }
