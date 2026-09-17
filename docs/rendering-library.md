# folio8 rendering library for Go

`folio8-go` turns a `.folio` template, JSON data and runtime parameters into a PDF 1.7 document. It
reads no clock, no environment, no network and no host fonts while rendering: everything a document
needs is passed in. The same inputs rendered with the same Go toolchain produce the same bytes.

This guide covers installing the module, rendering your first PDF, the inputs and failures you have to
handle, the template features that change what is drawn, and every exported API of the `folio8`
and `fonts` packages. Two companion references hold the rules this guide does not repeat:

- [The `.folio` format](folio-format.md) — every field of a template, version rules and load errors.
- [Expressions](expression-reference.md) — the syntax inside `{{ }}` and Visibility formulas.

Contents: [Install](#install) · [Your first PDF](#your-first-pdf) ·
[Writing to an `io.Writer`](#writing-to-an-iowriter) · [Inputs](#inputs) ·
[Warnings and errors](#warnings-and-errors) · [Load time versus render time](#load-time-versus-render-time) ·
[Reproducible output](#reproducible-output) · [Known limitations](#known-limitations) ·
[Template features that change the PDF](#template-features-that-change-the-pdf) ·
[API reference](#api-reference) · [Command-line tool](#command-line-tool)

## Install

folio8 has no tagged release yet, so you install a commit of the `main` branch. From your
application's directory:

```sh
go mod init example.com/folio8-demo
go get github.com/panitw/folio8/folio8-go@main
go mod tidy
```

On 2026-09-15 `@main` resolved to `github.com/panitw/folio8/folio8-go v0.0.0-20260914182357-563352e6f92a`
(commit `563352e`), and this guide's programs and example templates were verified against that
version from a fresh module. `go get` records the resolved pseudo-version in your `go.mod`, so your
build stays pinned to that commit until you run `go get github.com/panitw/folio8/folio8-go@main` again
to upgrade. The public API is not frozen before a release is tagged, so read the changes before you
upgrade. If a module proxy still serves an older commit for `@main`, fetch with `GOPROXY=direct`.

Import the module root as `folio8`. The shipped fonts are a separate, opt-in package:

```go
import (
	folio8 "github.com/panitw/folio8/folio8-go"
	"github.com/panitw/folio8/folio8-go/fonts"
)
```

**Toolchain.** `folio8-go/go.mod` declares `go 1.25.0` as the minimum language version and
`toolchain go1.26.0`. A dependency's `toolchain` line does not choose the compiler for your
application: your own `go.mod` `toolchain` line, or your `GOTOOLCHAIN` setting, does. If you record
PDF hashes in your own tests and expect them to hold, pin your own toolchain as well as your inputs.

**Binary size.** `fonts` embeds its faces with `go:embed`, about 14.8 MB of raw font data. Package
`folio8` never imports `fonts`, so the data is in your binary only if you import `fonts` yourself.

## Your first PDF

A template, a data file and a short program. Save these two files next to the program:

`first-pdf.folio`

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "Hello, {{customer.name}}!", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {
    "body": ["Noto Sans"]
  },
  "locale": "en",
  "nextId": 2,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
```

`first-pdf.data.json`

```json
{"customer": {"name": "Ada Lovelace"}}
```

`main.go`

```go
// Command first-pdf renders docs/examples/first-pdf.folio to first-pdf.pdf.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	folio8 "github.com/panitw/folio8/folio8-go"
	"github.com/panitw/folio8/folio8-go/fonts"
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
```

`go run .` prints `wrote first-pdf.pdf (53035 bytes, 0 warnings)` with the verified version and
toolchain; the byte count is a property of the inputs and the library version, so treat it as
illustrative. The page reads "Hello, Ada Lovelace!" in Noto Sans.

What each step does:

1. `folio8.LoadTemplate` reads a file and calls `folio8.ParseTemplate`. Use `ParseTemplate` directly
   when the template bytes come from memory, an embed or a database. Both return an opaque
   `*folio8.Template`; you cannot build one field by field, only by parsing.
2. `folio8.Data` is your report data as raw JSON bytes. `nil` params means no runtime values.
3. `fonts.Shipped()` supplies the face named by the template's `fonts` chain (`"Noto Sans"`).
4. `folio8.Render` returns a `folio8.Result`: `Bytes` is the complete PDF whenever the error is nil,
   and `Diagnostics` holds warnings that accompanied that successful render.
5. A failure is an ordinary Go `error`. When it concerns the template, the data or a render rule, it
   is a `*folio8.RenderError` whose `Diagnostic` carries a stable `Code`; `errors.As` finds it. Other
   failures, such as malformed JSON data or a file that cannot be read, are plain errors — keep the
   fallback branch.

`folio8.SerializeTemplate(tpl)` returns the template's canonical `.folio` bytes, which is how an
editor saves a template it loaded. Saving writes keys in canonical order, keeps the declared
`version` unless the content requires a higher one, and never lowers it.

## Writing to an `io.Writer`

`folio8.RenderTo` takes the same arguments as `Render` with a writer first, writes the PDF and
returns the warnings:

```go
diagnostics, err := folio8.RenderTo(writer, tpl, data, params, fontSet)
```

It builds the whole document in memory first — the PDF's cross-reference table and `/ID` depend on
the complete file — and then makes one `Write` call with exactly the bytes `Render` returns. It does
not stream. If the writer returns an error, or accepts fewer bytes than it was given, `RenderTo`
returns an error; the writer may already have accepted part of the document, so discard or remove
partial output. A render failure returns before anything is written.

This program renders the first-PDF template both ways, compares the bytes, shows `Validate`, and
shows a failing writer:

```go
// Command render-to renders the same document twice — once with Render and
// once with RenderTo into a file — and checks the bytes are identical.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"

	folio8 "github.com/panitw/folio8/folio8-go"
	"github.com/panitw/folio8/folio8-go/fonts"
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
```

Verified output:

```text
validate: 0 warnings
bytes equal: true
failing writer: folio8: RenderTo: write failed after 0 of 53035 bytes: disk full
is a RenderError: false
```

A writer failure is a plain error, not a `*folio8.RenderError`: nothing is wrong with the document.

**In an HTTP handler**, a render error returned by `RenderTo` happens before any byte is written, so
you can still send an error status. A write error happens after the response has started — the
status line and part of the body may already be on the wire — so you can only log it. To choose the
status code with certainty, call `Render` first and write `res.Bytes` yourself:

```go
res, err := folio8.Render(tpl, data, params, fontSet)
if err != nil {
	http.Error(w, "could not render statement", http.StatusInternalServerError)
	return
}
w.Header().Set("Content-Type", "application/pdf")
if _, err := w.Write(res.Bytes); err != nil {
	log.Printf("statement write failed: %v", err) // headers are already sent
}
```

**`Validate`** takes template **bytes** (it parses them itself), plus the same data, params and fonts
you will render with. It runs the checks a render runs and returns the error the render would
return, plus the warnings found while preparing the document, without producing a PDF. It is a prediction for those inputs, not a structural lint: with
empty data it correctly reports that the paths your template binds to are absent.

## Inputs

### Data and params

`folio8.Data` and `folio8.Params` are distinct types over raw JSON bytes, so swapping them at a call
site does not compile.

- **Data** is the report: `{{customer.name}}` and `transactions[]` read from it. It must be valid JSON.
- **Params** are runtime values that are not report data, read only through the reserved
  `params.` prefix: `{{params.reportDate}}`. A path starting with `params.` never reads data, so a
  top-level `"params"` key in your data is legal and unreachable. `nil` or empty `Params` means no
  runtime values were supplied, not malformed JSON.
- A path absent from the data — or `{{params.x}}` with no such param — is an error
  (`BINDING_PATH_ABSENT`) naming the element and the path. It is never rendered as empty. A value that
  is present and JSON `null` renders empty.
- Numbers keep the precision written in the JSON. Both inputs are decoded as exact decimals, never as
  `float64`. Build the JSON from your own values (strings, `json.Number`, decimal types) rather than
  decoding into `float64` and re-encoding, which rounds before folio8 ever sees the value.
- `params.documentDate` is reserved: an RFC 3339 timestamp that, when present, is written as the PDF's
  creation and modification date. A present value that is not a valid timestamp fails with
  `DOCUMENT_DATE_INVALID`. Without it the PDF has no date at all.

### Fonts

A render uses only the fonts you pass. `folio8.FontSet` maps a face name to raw OpenType/TrueType bytes:

```go
type FontSet map[string][]byte
```

A template's `fonts` object declares named fallback chains, and each text element names a chain in
`style.fontFamily`. Each character is drawn from the first face in the chain that covers it. A chain
entry is either a face **name**, looked up in your `FontSet`, or an `{"asset": "<key>"}` entry, a
face the template carries in its own `assets` — resolved by asset key only, never by name, so a
`FontSet` entry can never replace an embedded face or the reverse. A chain entry may also declare
`bold`, `italic` and `boldItalic` faces; `style.bold`/`style.italic` pick those, and folio8 never
synthesises a weight or slant. See [`fonts`](folio-format.md#fonts) and
[`assets`](folio-format.md#assets) for the rules.

`fonts.Shipped()` returns a fresh `FontSet` with these face names:

| Family | Face names |
|---|---|
| Noto Sans | `Noto Sans`, `Noto Sans Bold`, `Noto Sans Italic`, `Noto Sans Bold Italic` |
| Noto Sans Thai | `Noto Sans Thai`, `Noto Sans Thai Bold` |
| Noto Sans SC (Simplified Chinese, also used for Japanese) | `Noto Sans SC` |
| Roboto | `Roboto`, `Roboto Bold`, `Roboto Italic`, `Roboto Bold Italic` |

The shipped faces are licensed under the SIL Open Font License 1.1; the licence text and notice ship
next to each face in the `fonts` package directory. Faces you supply yourself, or embed in a template,
are under their own licences: a template that embeds a font redistributes that font with the file.

To use your own faces, build the map yourself, optionally starting from the shipped set:

```go
fontSet := fonts.Shipped()
brand, err := os.ReadFile("BrandSans-Regular.ttf")
if err != nil {
	log.Fatal(err)
}
fontSet["Brand Sans"] = brand // referenced by a chain such as "body": ["Brand Sans", "Noto Sans Thai"]
```

A character that no face in its chain covers is omitted, never drawn as a box, and reported with
the warning `TEXT_MISSING_GLYPH`. A bold or italic request that the covering entry declares no face
for is drawn in that entry's regular face with the warning `TEXT_STYLE_FACE_UNDECLARED`. A chain face
missing from the `FontSet`, or bytes that are not a usable font, fail the render with an error naming the element and the chain;
that error is not a `*folio8.RenderError`, so handle it in your fallback branch.

## Warnings and errors

folio8 reports every problem as a `folio8.Diagnostic`:

| Field | Meaning |
|---|---|
| `Severity` | `folio8.SeverityWarning` or `folio8.SeverityError`. `Severity.String()` returns `"Warning"` or `"Error"`; the zero value prints `"Severity(unset)"` and is never produced by folio8. |
| `Code` | A stable string from a closed registry, such as `"TEXT_CLIPPED_WIDTH"`. Compare against the `folio8.DiagCode…` constants. A code's meaning never changes. |
| `ElementID` | The template element (`"e7"`) or table column the problem concerns, when there is one. |
| `DataPath` | The data path, or for load errors the template field path (`"pages[1].sectionBreak"`, `"table.width"`), when there is one. |
| `Message` | A human-readable sentence. Print it; never parse it. |

**Warnings accompany a successful render.** The PDF in `Result.Bytes` (or written by `RenderTo`) is
complete, and something could not be honoured exactly — text clipped at its box, a missing glyph, a
barcode that could not be drawn. `Result.Diagnostics` lists them in document order, and is `nil`
when there are none. `folio8 render -strict` turns warnings into a failure; in Go, decide yourself.

**Errors abort.** `LoadTemplate`, `ParseTemplate`, `Render`, `RenderTo` and `Validate` return a Go
`error`, and no PDF. When the error is a known condition it is a `*folio8.RenderError` whose
`Diagnostic` has `Severity` `SeverityError` and a code; `Unwrap` exposes the underlying error.

```go
res, err := folio8.Render(tpl, data, params, fontSet)
var re *folio8.RenderError
switch {
case errors.As(err, &re):
	switch re.Diagnostic.Code {
	case folio8.DiagCodeBindingPathAbsent:
		return fmt.Errorf("data is missing %s (element %s)", re.Diagnostic.DataPath, re.Diagnostic.ElementID)
	default:
		return fmt.Errorf("%s: %w", re.Diagnostic.Code, err)
	}
case err != nil:
	return err // not a document problem: invalid JSON data, a nil template, an I/O failure
}
for _, d := range res.Diagnostics {
	if d.Code == folio8.DiagCodeTextMissingGlyph {
		log.Printf("element %s lost a character: %s", d.ElementID, d.Message)
	}
}
```

Every code, with its value and meaning, is listed in [Diagnostic codes](#diagnostic-codes).

## Load time versus render time

Some problems are decidable from the template alone, and loading refuses them: `ParseTemplate` and
`LoadTemplate` fail and there is no `*Template` to render. Others depend on data, params or fonts
and surface only when rendering.

| Refused at load (`ParseTemplate`/`LoadTemplate`) | Reported at render (`Render`/`RenderTo`/`Validate`) |
|---|---|
| Malformed JSON or a MAJOR version above 4 — `TEMPLATE_MALFORMED` | An absent data or params path — `BINDING_PATH_ABSENT` (error) |
| A field value outside its rules: unknown closed-set member, missing required field, duplicate id, bad table width allocation, a colour that is not `#RRGGBB`, a `lineSpacing` outside its range — `TEMPLATE_FIELD_INVALID`, located by `DataPath` | An expression that fails on this data (wrong kind, division by zero) — `EXPRESSION_INVALID` (error) |
| An invalid expression (syntax, unknown function, provably wrong kind) — `EXPRESSION_INVALID` | A line or image taller than the content window — `CONTENT_UNLAYOUTABLE` (error) |
| A table `minHeight` taller than the content window — `TABLE_MIN_HEIGHT_UNPLACEABLE` | Clipped text, missing glyphs, undeclared faces, suppressed table headers or footers, clipped rows — warnings |
| A section break out of range, duplicated, on a header/footer band, or `sectionBreakAnchor` without `sectionBreak` or not a boolean — `SECTION_BREAK_INVALID`; an element across the break — `SECTION_BREAK_STRADDLED` | Barcode/QR content that cannot be encoded or fitted — warnings |
| An invalid `pages` array — `PAGES_INVALID` | An `avg` over an empty collection — `AGGREGATE_EMPTY_AVERAGE` (warning) |

`PAGES_INVALID` and `SECTION_BREAK_INVALID` locate the problem through `Diagnostic.DataPath` —
`pages`, `pages[1]`, `pages[1].sectionBreak`, `bands.content.sectionBreakAnchor` — not through
`ElementID`, because a band or a page is not an element. `SECTION_BREAK_STRADDLED` names the
element in `ElementID`. Table width allocation problems are `TEMPLATE_FIELD_INVALID` with `DataPath`
`table.width` or `column.<field>`.

## Reproducible output

- Output depends only on the template, data, params, `FontSet` and the Go toolchain that built your
  binary. Rendering never reads the clock, environment variables, the host locale, host fonts or the
  network.
- Locale and time are document properties: the template's `locale` (`en`, `th`, `zh-Hans` or `ja`)
  and fixed `utcOffset` drive `formatDate`, `formatNumber` and line breaking. Any other locale is a
  load error.
- No PDF date is written unless you pass `params.documentDate`. The `folio8` command-line tool fills
  that param from `SOURCE_DATE_EPOCH` when you have not supplied one; the library never reads that
  variable.
- Nothing in this guide is a statement about concurrent use; the library makes no documented
  concurrency guarantee for sharing one `*Template` or `FontSet` across goroutines while it is being
  mutated. Treat `FontSet` maps you pass as read-only for the duration of a call.

## Known limitations

- **Japanese glyph forms.** `ja` text renders with coverage from `Noto Sans SC`, which draws the
  Simplified Chinese shape for the ideographs whose Japanese form differs. Supply a Japanese face in
  your `FontSet` and chain if that matters.
- **Thai names.** Thai has no spaces between words and breaks from a dictionary, which cannot tell a
  name from the ordinary words it is made of. List data paths whose values must never break in the
  document's `unbreakableValues`; a name inside free-form text is still breakable. See
  [Line breaking](folio-format.md#line-breaking).
- **Latin breaking** is at spaces only: no hyphenation, no break after `-`, and it is not UAX #14.
- **CJK kinsoku** is not implemented: a line may begin with `，` or end with an opening bracket.
- Expression syntax, limits and division scale: see [Expressions](expression-reference.md).

## Template features that change the PDF

Each feature below has a minimal template. The docs tests render every one of them on every change
and assert the key outcomes — page counts, where the moved content lands, and diagnostic codes. A template must declare at least the version its features
require — saving through `SerializeTemplate` raises `version` for you. Versions up to `4.1` load; a
higher MINOR also loads, a higher MAJOR is refused. The examples use a small 200 × 150 pt page with
10 pt margins and 10 pt header and footer bands, so the content window is 110 pt tall.

| Feature | Requires | Diagnostics |
|---|---|---|
| Formulas: comparisons, arithmetic, ternaries, `true`/`false`/`null` | `2.0`; `3.3` when a text expression can return a number | `EXPRESSION_INVALID`, `BINDING_PATH_ABSENT`, `AGGREGATE_EMPTY_AVERAGE` |
| Proportional table widths | `3.0` | `TEMPLATE_FIELD_INVALID` at `table.width` or `column.<field>` |
| Table frame, `rules`, `minHeight` | `3.1` | `TABLE_MIN_HEIGHT_UNPLACEABLE` |
| Column `headerAlign` | `3.2` | — |
| `barcode` and `qrcode` elements | `4.0` | `BARCODE_UNENCODABLE`, `BARCODE_DOES_NOT_FIT`, `BARCODE_MODULE_TOO_SMALL`, `QRCODE_TOO_LONG`, `QRCODE_DOES_NOT_FIT`, `QRCODE_MODULE_TOO_SMALL` |
| Section break, anchored and unanchored | `4.1` | `SECTION_BREAK_INVALID`, `SECTION_BREAK_STRADDLED`, `SECTION_BREAK_SPLITS_KEEP_TOGETHER` |
| Designed pages, `pageBreak` true and false | `4.1` | `PAGES_INVALID` |

### Formulas

Visibility (`visibleIf`) takes a bare formula; text takes `{{ }}` expressions. Comparisons, exact
decimal arithmetic, nested `? :` and the literals `true`, `false` and `null` work in both, and in
`if()`. A number printed bare in text appears as its exact decimal (`{{1 / 3}}` prints `0.3333`); use
`formatNumber` for grouping. A hidden element leaves its siblings where they are. Syntax, precedence,
limits and division scale are in [Expressions](expression-reference.md#formulas-and-visibility); field rules in
[Expressions](folio-format.md#expressions).

`formula-visibility.folio`

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 180, "height": 12, "value": "Total {{formatNumber(loanAmount + fee, \"#,##0.00\")}}", "style": {"fontFamily": "body", "fontSize": 8}},
        {"id": "e2", "type": "text", "x": 0, "y": 16, "width": 180, "height": 12, "value": "Manual review required", "visibleIf": "loanAmount > 20000", "style": {"fontFamily": "body", "fontSize": 8}},
        {"id": "e3", "type": "text", "x": 0, "y": 32, "width": 180, "height": 12, "value": "{{loanAmount > 20000 ? \"Tier: large\" : \"Tier: standard\"}}", "style": {"fontFamily": "body", "fontSize": 8}}
      ]
    },
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "2.0"
}
```

`formula-visibility.above.json`

```json
{"loanAmount": 25000, "fee": 150.50}
```

`formula-visibility.below.json`

```json
{"loanAmount": 20000, "fee": 150.50}
```

With the first data the page reads "Total 25,150.50", "Manual review required" and "Tier: large".
With the second it reads "Total 20,150.50" and "Tier: standard"; the review line is not drawn and the
tier line stays at y 32. There are no warnings. A missing `loanAmount` fails with
`BINDING_PATH_ABSENT`; `"visibleIf": "loanAmount"` with a number is a located error, because
conditions have no truthiness.

### Barcode and QR code

A `barcode` element encodes its `value` as Code 128; a `qrcode` element encodes it as a QR Code at
error correction `L`, `M` (the default), `Q` or `H`. `value` binds like text. Both draw black vector
modules of one whole-millipoint width, as large as fits the box with the quiet zone inside it, centred
and never distorted, with no human-readable text and no `style`. Store control characters as real
characters (`"\r"` in JSON). See [Elements](folio-format.md#elements).

`barcode-qrcode.folio`

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 260, "height": 50, "value": "INV-{{invoice}}"},
        {"id": "e2", "type": "qrcode", "x": 0, "y": 60, "width": 100, "height": 100, "value": "https://example.com/pay/{{invoice}}"},
        {"id": "e3", "type": "qrcode", "x": 120, "y": 60, "width": 100, "height": 100, "errorCorrection": "H", "value": "{{invoice}}"}
      ]
    },
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 300, "height": 220}},
  "utcOffset": "+00:00",
  "version": "4.0"
}
```

`barcode-qrcode.data.json`

```json
{"invoice": "2026-000123"}
```

`barcode-qrcode.unencodable.json`

```json
{"invoice": "2026-ใบที่-7"}
```

With the first data the page carries three codes, which an independent decoder reads as Code 128
`INV-2026-000123`, QR (level M) `https://example.com/pay/2026-000123` and QR (level H) `2026-000123`,
with no warnings. With the second data the barcode is omitted with the warning
`BARCODE_UNENCODABLE` on `e1` — Code 128 encodes ASCII only — while both QR codes still draw, because
QR encodes the UTF-8 bytes. The render succeeds. Static text that cannot be encoded (a non-ASCII
character outside `{{ }}` in a barcode, or a QR value already too long) is refused at load instead. A
symbol that cannot fit at one millipoint per module is omitted with `BARCODE_DOES_NOT_FIT` or
`QRCODE_DOES_NOT_FIT`; modules narrower than 0.25 mm (barcode) or 0.5 mm (QR) still draw, with
`BARCODE_MODULE_TOO_SMALL` or `QRCODE_MODULE_TOO_SMALL`; QR data too long for version 40 is omitted
with `QRCODE_TOO_LONG`.

### Section break

`sectionBreak` on the content band is an offset in points. Elements declared at or below it form a
section that moves as one rigid block when the content above grows past the line. Anchored (the
default) moves the section by whole pages, keeping its declared position; with
`"sectionBreakAnchor": false` the section follows where the content above ends when it fits on that
page. When nothing crosses the line, the PDF is byte-identical to the same template without the key.
The break is never drawn, and a `keepTogether` group split by it is split with the warning
`SECTION_BREAK_SPLITS_KEEP_TOGETHER`. See [Pagination](folio-format.md#pagination) and
[`bands`](folio-format.md#bands).

`section-break.folio` (anchored)

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
          "style": {"fontFamily": "body", "fontSize": 8},
          "columns": [
            {"id": "e2", "label": "Item", "width": 100, "bind": "{{row.name}}"},
            {"id": "e3", "label": "Amount", "width": 80, "align": "right", "bind": "{{row.amount}}"}
          ]},
        {"id": "e5", "type": "text", "x": 0, "y": 80, "width": 180, "height": 12, "value": "Legend", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "sectionBreak": 75
    },
    "pageFooter": {"elements": [{"id": "e4", "type": "text", "x": 0, "y": 0, "width": 180, "height": 8, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 6}}], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "4.1"
}
```

`section-break-unanchored.folio` differs only in the content band's keys:

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
          "style": {"fontFamily": "body", "fontSize": 8},
          "columns": [
            {"id": "e2", "label": "Item", "width": 100, "bind": "{{row.name}}"},
            {"id": "e3", "label": "Amount", "width": 80, "align": "right", "bind": "{{row.amount}}"}
          ]},
        {"id": "e5", "type": "text", "x": 0, "y": 80, "width": 180, "height": 12, "value": "Legend", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "sectionBreak": 75, "sectionBreakAnchor": false
    },
    "pageFooter": {"elements": [{"id": "e4", "type": "text", "x": 0, "y": 0, "width": 180, "height": 8, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 6}}], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "4.1"
}
```

`section-break.5-rows.json`

```json
{"items": [{"name": "Item 1", "amount": "10.00"}, {"name": "Item 2", "amount": "20.00"}, {"name": "Item 3", "amount": "30.00"}, {"name": "Item 4", "amount": "40.00"}, {"name": "Item 5", "amount": "50.00"}]}
```

`section-break.7-rows.json`

```json
{"items": [{"name": "Item 1", "amount": "10.00"}, {"name": "Item 2", "amount": "20.00"}, {"name": "Item 3", "amount": "30.00"}, {"name": "Item 4", "amount": "40.00"}, {"name": "Item 5", "amount": "50.00"}, {"name": "Item 6", "amount": "60.00"}, {"name": "Item 7", "amount": "70.00"}]}
```

The header row is 10 pt and each row about 10.9 pt, so five rows end at 64.5 pt and seven at 86.3 pt.

| Template | Data | Result |
|---|---|---|
| either | 5 rows | One page; the legend at its declared y 80. The PDF is identical to the template without `sectionBreak`. |
| anchored | 7 rows | Two pages. Page 1 holds the table; the legend moves to page 2 at its declared y 80. Footers read `Page 1 of 2` and `Page 2 of 2`. |
| unanchored | 7 rows | One page. The legend moves down 11.272 pt — the distance the rows ran past the line — and sits directly under the table. |

No warnings in any case. A `sectionBreak` of 0 or at least the content height, a second break, a
break on `pageHeader`/`pageFooter`, or `sectionBreakAnchor` without `sectionBreak` fails at load
with `SECTION_BREAK_INVALID` located at the band's key; an element whose box lies on both sides of
the line fails with `SECTION_BREAK_STRADDLED` naming it.

### Designed pages

A template with a top-level `pages` array has two or more designed pages, rendered in order. They
share the page setup, page header and page footer, and `{{page}}`/`{{pages}}` count the output pages
of all of them. Each page's content overflows onto further output pages as usual, and each page may
have its own section break. With `bands.content` then empty, a page's `pageBreak` decides how it
follows the previous page: `true` (Page Break on, the default after page 1) starts a new output page
after the previous page and all its overflow; `false` continues directly where the previous page's
content ended, as one rigid block, if the previous page overflowed onto more than one output page
and the block fits in the room left — otherwise it starts a new output page. See
[Designed pages](folio-format.md#designed-pages).

`designed-pages.folio` (Page Break on)

```json
{
  "assets": {},
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [{"id": "e4", "type": "text", "x": 0, "y": 0, "width": 180, "height": 8, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 6}}], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "pages": [
    {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "body", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "Item", "width": 100, "bind": "{{row.name}}"},
          {"id": "e3", "label": "Amount", "width": 80, "align": "right", "bind": "{{row.amount}}"}
        ]}
    ]},
    {"elements": [
      {"id": "e5", "type": "text", "x": 0, "y": 0, "width": 180, "height": 12, "value": "Approved by", "style": {"fontFamily": "body", "fontSize": 8}}
    ], "pageBreak": true}
  ],
  "utcOffset": "+00:00",
  "version": "4.1"
}
```

`designed-pages-page-break-off.folio`

```json
{
  "assets": {},
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [{"id": "e4", "type": "text", "x": 0, "y": 0, "width": 180, "height": 8, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 6}}], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 6,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "pages": [
    {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "body", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "Item", "width": 100, "bind": "{{row.name}}"},
          {"id": "e3", "label": "Amount", "width": 80, "align": "right", "bind": "{{row.amount}}"}
        ]}
    ]},
    {"elements": [
      {"id": "e5", "type": "text", "x": 0, "y": 0, "width": 180, "height": 12, "value": "Approved by", "style": {"fontFamily": "body", "fontSize": 8}}
    ], "pageBreak": false}
  ],
  "utcOffset": "+00:00",
  "version": "4.1"
}
```

`designed-pages.data.json`

```json
{"items": [{"name": "Item 1", "amount": "10.00"}, {"name": "Item 2", "amount": "20.00"}, {"name": "Item 3", "amount": "30.00"}, {"name": "Item 4", "amount": "40.00"}, {"name": "Item 5", "amount": "50.00"}, {"name": "Item 6", "amount": "60.00"}, {"name": "Item 7", "amount": "70.00"}, {"name": "Item 8", "amount": "80.00"}, {"name": "Item 9", "amount": "90.00"}, {"name": "Item 10", "amount": "100.00"}, {"name": "Item 11", "amount": "110.00"}, {"name": "Item 12", "amount": "120.00"}]}
```

Designed page 1's twelve rows need two output pages: rows 1–9 on the first, a repeated header and
rows 10–12 on the second.

| Page Break | Result |
|---|---|
| on | Three output pages. "Approved by" is alone on output page 3 at its declared position. Footers read `Page 1 of 3` to `Page 3 of 3`. |
| off | Two output pages. "Approved by" is drawn on output page 2 directly under row 12. Footers read `Page 1 of 2` and `Page 2 of 2`. |

No warnings. A `pages` array with fewer than two entries, elements or a section break left in
`bands.content`, an unknown key on a page, a non-boolean `pageBreak`, or a `keepTogether` group on two
pages fails at load with `PAGES_INVALID`, located by `DataPath`.

### Table frame, rules and minHeight

A table's `style.border` and `style.background` draw one frame around each page's slice of the table,
not a border on every cell. `rules` draws interior lines between `columns` and/or `rows`, never on
the frame's edge. `minHeight` is a floor for each page's slice: the frame and column rules extend to
it, rows are never stretched, and following content starts below it. Column labels may wrap onto
several lines, and `headerHeight` is then the header's minimum height. See [`table`](folio-format.md#table).

`ruled-table.folio`

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
          "minHeight": 90,
          "rules": {"between": ["columns", "rows"], "width": 0.5, "color": "#808080"},
          "style": {"fontFamily": "body", "fontSize": 8, "border": {"width": 1, "color": "#000000"}},
          "columns": [
            {"id": "e2", "label": "Item", "width": 100, "bind": "{{row.name}}"},
            {"id": "e3", "label": "Amount", "width": 80, "align": "right", "bind": "{{row.amount}}"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "3.1"
}
```

`ruled-table.data.json`

```json
{"items": [{"name": "Item 1", "amount": "10.00"}, {"name": "Item 2", "amount": "20.00"}, {"name": "Item 3", "amount": "30.00"}]}
```

The page shows a 180 × 90 pt frame although three rows fill only about 43 pt, one vertical rule
between the two columns running the frame's full height, and three horizontal rules: under the
header and between the rows, none under the last row. No warnings.

`ruled-table-unplaceable.folio` asks for a floor taller than the 110 pt content window:

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
          "minHeight": 200,
          "rules": {"between": ["columns", "rows"], "width": 0.5, "color": "#808080"},
          "style": {"fontFamily": "body", "fontSize": 8, "border": {"width": 1, "color": "#000000"}},
          "columns": [
            {"id": "e2", "label": "Item", "width": 100, "bind": "{{row.name}}"},
            {"id": "e3", "label": "Amount", "width": 80, "align": "right", "bind": "{{row.amount}}"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "3.1"
}
```

`ParseTemplate` refuses it with a `*folio8.RenderError` whose code is `TABLE_MIN_HEIGHT_UNPLACEABLE`
and `ElementID` is `e1`; there is no template to render. The same page with data or fonts cannot
change that outcome.

The frame meaning of `style.border` applies to every table, including templates written before
`3.1`: a template that relied on `style.border` drawing a grid now draws only the frame. Declare
`"rules": {"between": ["columns", "rows"]}` to draw the interior lines.

### Other version-gated table features

- **Proportional widths** (`3.0`): declare the table's total `width` and a `proportion` on each column
  instead of column widths. folio8 allocates exact millipoint widths that sum to the total. A zero-width
  allocation or a mix of both representations fails at load with `TEMPLATE_FIELD_INVALID`, `DataPath`
  `table.width` or `column.<field>`.
- **Column `headerAlign`** (`3.2`): aligns one column's header cell independently of its data.

## API reference

This section lists every exported identifier in the two packages of the `github.com/panitw/folio8/folio8-go` module:

| Import path | Package | Role |
|---|---|---|
| `github.com/panitw/folio8/folio8-go` | `folio8` | Parsing, rendering, validation, diagnostics and template helpers. |
| `github.com/panitw/folio8/folio8-go/fonts` | `fonts` | The shipped font faces, as a ready-made `folio8.FontSet`. Opt-in: package `folio8` never imports it. |

**Stability.** The module has no release tag yet: `folio8.Version` is `"0.0.0-dev"`. The rendering and validation entry points, the font input and the diagnostic types are the whole public surface. The canvas and authoring engine behind folio8 Designer is internal to the module and is not part of this API.

**Concurrency.** Nothing in these packages documents or tests concurrent use. Do not share a `*folio8.Template` across goroutines without your own locking.

### Rendering and validation

#### `ParseTemplate`

```go
func ParseTemplate(b []byte) (*Template, error)
```

Parses `b` as a `.folio` document and returns an opaque `*Template`. Beyond decoding, it performs every check that can be decided from the document alone:

- It parses and statically checks every `{{ }}` expression: syntax, arity, unknown function names and literal argument kinds. It does not evaluate them.
- It derives `footerOf`/`footerFormat` for `sum`/`avg` table footers that omit `footerOf`.
- It refuses a table `minHeight` taller than the content window.
- It refuses an invalid section break or an element straddling one.

Errors are `*RenderError` values carrying a load code:

- `DiagCodeTemplateMalformed` for bytes that are not a loadable document at all.
- Otherwise the code the loader attached, by default `DiagCodeTemplateFieldInvalid`, or a specific code such as `DiagCodeTableFooterSourceForbidden`, `DiagCodeTableFooterSourceUnresolved`, `DiagCodeExpressionInvalid`, `DiagCodeTableMinHeightUnplaceable`, `DiagCodeSectionBreakInvalid`, `DiagCodeSectionBreakStraddled` or `DiagCodePagesInvalid`.

For `DiagCodeSectionBreakInvalid` and `DiagCodePagesInvalid`, `Diagnostic.DataPath` carries the band or page field path, such as `pages` or `pages[1]`, because there is no element id.

`b` is not retained after the call.

#### `LoadTemplate`

```go
func LoadTemplate(path string) (*Template, error)
```

Reads `path` with `os.ReadFile` and delegates to `ParseTemplate`. A file-system error is returned unwrapped, not as a `*RenderError`. Parse failures are exactly those of `ParseTemplate`.

#### `Template`

```go
type Template struct {
	// Has unexported fields.
}
```

A parsed, canonicalised `.folio` document. It is opaque: there are no exported fields or accessors, and a composite literal cannot construct a usable one. Obtain it only from `ParseTemplate` or `LoadTemplate`. No exported function modifies it.

#### `SerializeTemplate`

```go
func SerializeTemplate(t *Template) ([]byte, error)
```

Returns the engine's canonical `.folio` bytes for `t`. This is the save path the designer uses. A nil `t` returns an error. The written format version is raised when the document's expressions need it:

- `"2.0"` for formula syntax or boolean/null literals.
- `"3.3"` for a text expression whose static kind includes a number.

The returned slice is newly allocated.

#### `Render`

```go
func Render(t *Template, d Data, p Params, f FontSet) (Result, error)
```

Produces a PDF 1.7 document. It resolves every placeholder against `d` (report data) and `p` (runtime parameters) and embeds a subset of every font face used from `f`.

- **Preconditions.**
  - `t` must be non-nil, or a plain error is returned.
  - `d` must be syntactically valid JSON. It is decoded once with number literals preserved exactly, and a decode failure is a plain error prefixed `folio8: Render:`.
  - A nil or empty `p` means "no runtime values": `{{params.x}}` is then absent.
  - `f` must contain every face the document's font chains actually need. The engine never looks for fonts on the host.
- **Returns.** On success, `Result.Bytes` is the complete PDF and `Result.Diagnostics` holds every Warning. A non-nil error means nothing was rendered; ignore `Result` in that case.
- **Errors.** Located failures are `*RenderError` with a `SeverityError` diagnostic: for example `DiagCodeBindingPathAbsent`, `DiagCodeExpressionInvalid`, `DiagCodeContentUnlayoutable` or `DiagCodeDocumentDateInvalid`. Missing fonts and invalid input JSON may arrive as ordinary errors, so always keep a non-`RenderError` fallback.
- **Ownership.** `t`, `d`, `p` and `f` are not modified.

#### `RenderTo`

```go
func RenderTo(w io.Writer, t *Template, d Data, p Params, f FontSet) ([]Diagnostic, error)
```

Renders exactly as `Render` does, then writes the finished bytes to `w` in **one** `Write` call. RenderTo cannot stream: the PDF `/ID` and cross-reference offsets require the whole document first. It returns the Warning diagnostics.

Errors:

- `w == nil` returns an error without rendering.
- Any `Render` error is returned unchanged.
- A `Write` error is wrapped, with the message stating how many of the total bytes were accepted.
- A short write (`n < len` with a nil error) is reported as an error.

After a write failure `w` may already hold a partial document.

#### `Validate`

```go
func Validate(b []byte, d Data, p Params, f FontSet) ([]Diagnostic, error)
```

A dry-run predictor of `Render` over **template bytes**. It parses `b` (the same errors as `ParseTemplate`), decodes `d` and `p`, checks the reserved `documentDate` parameter, then runs the same preparation `Render` uses without composing pages or producing PDF bytes.

It returns the Warning diagnostics that preparation produced, or the first located error `Render` would report for the **same inputs**. Pass the data you will actually render with. An empty `Data` produces `BINDING_PATH_ABSENT` errors, which are correct predictions for a render with empty data, not template defects. On error the returned slice is nil.

#### `Data`

```go
type Data []byte
```

The report data as raw JSON bytes. It is a distinct defined type, so swapping `Data` and `Params` in a call is a compile error. Pass bytes, not a decoded Go value; the library owns the decode so that decimal precision survives. A top-level `"params"` key inside `Data` is legal but unreachable by bindings.

#### `Params`

```go
type Params []byte
```

Runtime values (for example a statement date) as raw JSON bytes. They are reachable only as `{{params.…}}` and decoded the same way as `Data`. Nil or empty means no runtime values; it is not a decode error.

#### `Result`

```go
type Result struct {
	Bytes       []byte
	Diagnostics []Diagnostic
}
```

| Field | Type | Meaning |
|---|---|---|
| `Bytes` | `[]byte` | The complete PDF. Meaningful only when `Render` returned a nil error. |
| `Diagnostics` | `[]Diagnostic` | Every Warning from the render, in document order: page header, then content, then page footer, and within a band in element declaration order. It is `nil`, never an empty non-nil slice, when there are none. It is never needed to decide whether `Bytes` is valid. |

### Fonts

#### `FontSet`

```go
type FontSet map[string][]byte
```

The engine's only font input. It maps a **face name**, as written in a document's `fonts` fallback chains (for example `"Noto Sans"`), to that face's raw OpenType/TrueType bytes. Rendering resolves faces only from this map and never queries the host system. Faces a document embeds in its own `assets` are resolved from the document. The map and its byte slices are read, not modified.

#### `fonts.Shipped`

```go
func Shipped() folio8.FontSet
```

Returns a new `folio8.FontSet` map holding the eleven faces embedded in the `fonts` package. Each face ships with its OFL-1.1 licence text and NOTICE under `folio8-go/fonts/`. The keys are exactly:

| Family | Face names (map keys) |
|---|---|
| Noto Sans | `Noto Sans`, `Noto Sans Bold`, `Noto Sans Italic`, `Noto Sans Bold Italic` |
| Noto Sans Thai | `Noto Sans Thai`, `Noto Sans Thai Bold` (no italic cuts exist upstream) |
| Noto Sans SC | `Noto Sans SC` (Regular only) |
| Roboto | `Roboto`, `Roboto Bold`, `Roboto Italic`, `Roboto Bold Italic` |

A chain entry must name one of these keys verbatim. Weight and slope come from the variants a chain entry *declares* (for example an entry object with `bold: "Roboto Bold"`); the engine never derives `"Roboto Bold"` from `"Roboto"`.

Each call builds a fresh map, but the byte slices are shared package data: never modify them. Importing `fonts` adds roughly 14.8 MB of raw font bytes to a binary.

### Diagnostics and errors

#### `Diagnostic`

```go
type Diagnostic struct {
	Severity  Severity
	Code      string
	ElementID string
	DataPath  string
	Message   string
}
```

| Field | Type | Meaning |
|---|---|---|
| `Severity` | `Severity` | `SeverityWarning` on `Result.Diagnostics` or `RenderTo`/`Validate` returns; `SeverityError` inside a `*RenderError`. |
| `Code` | `string` | A stable code from the closed registry below. Dispatch on this, never on `Message`. |
| `ElementID` | `string` | The template element concerned, when there is one. |
| `DataPath` | `string` | The data path concerned (for example the absent binding path), or a document field path for band/page load errors (`pages[1]`, `bands.content`, `table.width`, `column.<field>`). Empty when not applicable. |
| `Message` | `string` | A human-readable sentence, safe to display. Not a parsing target. |

`Diagnostic` has no JSON struct tags, so encoding it directly produces the Go field names and an integer `Severity`.

#### `Severity`, `SeverityWarning`, `SeverityError`, `Severity.String`

```go
type Severity int

const (
	SeverityWarning Severity // = 1
	SeverityError            // = 2
)

func (s Severity) String() string
```

A Warning accompanies a successful render; an Error aborts it and travels as Go's error return. The zero value is deliberately **not** a valid severity: it is an unexported "unset" constant, so a `Diagnostic{}` that forgot its severity is not mistaken for a Warning. Compare against the named constants, not integers.

`String` returns `"Warning"`, `"Error"`, `"Severity(unset)"` for the zero value, and `"Severity(N)"` for any other integer.

#### `RenderError`, `RenderError.Error`, `RenderError.Unwrap`

```go
type RenderError struct {
	Diagnostic Diagnostic
	Err        error
}

func (e *RenderError) Error() string
func (e *RenderError) Unwrap() error
```

| Field | Type | Meaning |
|---|---|---|
| `Diagnostic` | `Diagnostic` | `Severity` is `SeverityError`. `Code`, `ElementID` and `DataPath` locate the failure; `Message` equals `Err.Error()`. |
| `Err` | `error` | The underlying error. |

`Error()` returns `Err.Error()` unchanged. `Unwrap()` returns `Err`, so `errors.Is`/`errors.As` reach the wrapped error. Match on the code:

```go
res, err := folio8.Render(tpl, data, params, fontSet)
if err != nil {
	var re *folio8.RenderError
	if errors.As(err, &re) {
		switch re.Diagnostic.Code {
		case folio8.DiagCodeBindingPathAbsent:
			log.Printf("missing data %s (element %s)", re.Diagnostic.DataPath, re.Diagnostic.ElementID)
		default:
			log.Printf("%s: %s", re.Diagnostic.Code, re.Diagnostic.Message)
		}
	} else {
		log.Printf("render failed: %v", err)
	}
	return
}
_ = res
```

#### Diagnostic codes

Each constant is an untyped string constant whose value is the registry string. "Load" means refused by `ParseTemplate`/`LoadTemplate` (and therefore also by `Validate`), always as a `*RenderError`. "Render error" means `Render`/`RenderTo`/`Validate` return a `*RenderError`. "Render warning" means the render succeeds and the diagnostic is on `Result.Diagnostics`, the `RenderTo` return, or the `Validate` return.

| Go constant | String value | Disposition / when | Meaning |
|---|---|---|---|
| `DiagCodeTemplateMalformed` | `TEMPLATE_MALFORMED` | Load error | The bytes are not a loadable document: not a JSON object, an unreadable value, or an unsupported major version. |
| `DiagCodeTemplateFieldInvalid` | `TEMPLATE_FIELD_INVALID` | Load error | A well-formed document has an unacceptable field value: outside its closed set, missing, of the wrong JSON kind, or a duplicate/misspelled id. Also a colour that is not `#RRGGBB`, a `lineSpacing` outside [0.001, 1000] or with more than three decimal places, and static barcode/QR content that cannot be encoded. The message names the field, element and value. |
| `DiagCodeTableFooterSourceForbidden` | `TABLE_FOOTER_SOURCE_FORBIDDEN` | Load error | `footerOf` paired with `footer: "count"`, or a footer companion field without a `footer`. |
| `DiagCodeTableFooterSourceUnresolved` | `TABLE_FOOTER_SOURCE_UNRESOLVED` | Load error | A `sum`/`avg` footer omits `footerOf`, and its column bind is not a shape the source can be derived from. |
| `DiagCodeTableMinHeightUnplaceable` | `TABLE_MIN_HEIGHT_UNPLACEABLE` | Load error | A table's `minHeight` is taller than the content window. |
| `DiagCodeSectionBreakInvalid` | `SECTION_BREAK_INVALID` | Load error (`DataPath` = band) | A `sectionBreak` at or above the band top or at or below the content height, declared twice, or declared on the page header or footer. |
| `DiagCodeSectionBreakStraddled` | `SECTION_BREAK_STRADDLED` | Load error (`ElementID`) | An element's declared box lies on both sides of the section break. |
| `DiagCodePagesInvalid` | `PAGES_INVALID` | Load error (`DataPath` = `pages`, `pages[i]` or `bands.content`) | The `pages` array cannot be loaded: fewer than two entries, a non-object entry or unknown key, a non-boolean `pageBreak`, content in both `pages` and `bands.content`, or a keepTogether group spanning pages. |
| `DiagCodeExpressionInvalid` | `EXPRESSION_INVALID` | Load error (static check) and render error (evaluation) | An expression does not parse or check, or fails when evaluated. |
| `DiagCodeBindingPathAbsent` | `BINDING_PATH_ABSENT` | Render error (`DataPath` = the path) | A data or `params` path an expression needs is absent from the supplied JSON. |
| `DiagCodeContentUnlayoutable` | `CONTENT_UNLAYOUTABLE` | Render error | An ungrouped item (a text line or an image box) is taller than the content window. |
| `DiagCodeDocumentDateInvalid` | `DOCUMENT_DATE_INVALID` | Render error (also `Validate`) | The reserved `documentDate` parameter is present but not a valid RFC 3339 timestamp. |
| `DiagCodeTextClippedWidth` | `TEXT_CLIPPED_WIDTH` | Render warning | A text element's widest line exceeds its declared width and is clipped at the box edge. |
| `DiagCodeTextMissingGlyph` | `TEXT_MISSING_GLYPH` | Render warning | No face in the element's declared chain covers a character. The character is omitted (no glyph, no advance); the message names the rune and the chain. |
| `DiagCodeTextStyleFaceUndeclared` | `TEXT_STYLE_FACE_UNDECLARED` | Render warning | Bold and/or italic was requested, but the chain entry covering the character declares no such face. It is drawn in that entry's base face, with no synthetic emboldening or slant. |
| `DiagCodeEmptyAverage` | `AGGREGATE_EMPTY_AVERAGE` | Render warning | `avg()` over a present but empty collection; the aggregate resolves to empty. |
| `DiagCodeTableHeaderRepeatSuppressed` | `TABLE_HEADER_REPEAT_SUPPRESSED` | Render warning | The repeated table header was dropped on one continuation page because the next row would not fit under it. |
| `DiagCodeTableFooterOrphanSuppressed` | `TABLE_FOOTER_ORPHAN_SUPPRESSED` | Render warning | The footer and its preceding row together exceed the window, so the footer was placed alone. |
| `DiagCodeTableRowClippedHeight` | `TABLE_ROW_CLIPPED_HEIGHT` | Render warning | A header, data or footer row is taller than the whole content window. It was placed alone and cut off at the page bottom, so content is lost. |
| `DiagCodeBarcodeUnencodable` | `BARCODE_UNENCODABLE` | Render warning | A data-bound barcode value contains a character Code 128 cannot encode; the barcode is omitted. |
| `DiagCodeBarcodeModuleTooSmall` | `BARCODE_MODULE_TOO_SMALL` | Render warning | The barcode fits only with modules narrower than 0.25 mm (709 mp). It is still drawn. |
| `DiagCodeBarcodeDoesNotFit` | `BARCODE_DOES_NOT_FIT` | Render warning | The symbol plus quiet zones cannot fit even at 1 mp per module; the barcode is omitted. |
| `DiagCodeQRCodeTooLong` | `QRCODE_TOO_LONG` | Render warning | A data-bound value exceeds a version-40 QR symbol at its error-correction level; the QR code is omitted. |
| `DiagCodeQRCodeModuleTooSmall` | `QRCODE_MODULE_TOO_SMALL` | Render warning | The QR code fits only with modules narrower than 0.5 mm (1418 mp). It is still drawn. |
| `DiagCodeQRCodeDoesNotFit` | `QRCODE_DOES_NOT_FIT` | Render warning | The symbol plus its 4-module quiet zone cannot fit the box's smaller side at 1 mp per module; the QR code is omitted. |
| `DiagCodeSectionBreakSplitsKeepTogether` | `SECTION_BREAK_SPLITS_KEEP_TOGETHER` | Render warning | A keepTogether group has members on both sides of a section break. The break wins and each side is kept together separately. |
| `DiagCodeInternalUnhandledCaveat` | `INTERNAL_UNHANDLED_CAVEAT` | Render warning | Safety net for an internal evaluation caveat with no mapping; not reachable with the current engine. |

Codes are additive: once shipped, a code's string and meaning do not change. The Designer worker additionally uses its own transport codes, such as `COMPONENT_INVALID` and `PAGE_SETUP_INVALID`. Those are not exported by these packages.

### Template helpers

#### `ParameterReferences` and `MaxParameterReferenceNameLength`

```go
func ParameterReferences(tpl *Template) ([]string, error)

const MaxParameterReferenceNameLength = 128
```

Returns the sorted, de-duplicated top-level parameter names the template requests directly. For `{{params.statement.date}}` the name is `"statement"`. It scans every element's `visibleIf` and the `{{ }}` placeholders in text, barcode and qrcode `value`s. Table column bindings are not scanned, and data paths never appear.

Errors:

- a nil template
- an expression that does not parse
- a name longer than `MaxParameterReferenceNameLength` bytes (names are ASCII)
- more than 128 distinct names

#### `LocaleTableVersion` and `Version`

```go
const LocaleTableVersion = expr.LocaleTableVersion // currently 1 (untyped integer)
const Version = "0.0.0-dev"
```

`LocaleTableVersion` identifies the built-in locale formatting table; its current value is `1`. `Version` is the library version string. No release tag exists yet.

## Command-line tool

The module also contains a `folio8` command with `validate` and `render` subcommands
(`go run github.com/panitw/folio8/folio8-go/cmd/folio8@main render -data data.json -o out.pdf template.folio`).
It is a thin wrapper over `Validate` and `Render`; its flags are described in the
[repository README](../README.md#render-from-the-command-line).
