# folio-go

`folio-go` turns a `.folio` template plus report data into a PDF 1.7 document —
byte-identically, every time, on a given build toolchain (see
[Reproducible bytes](#reproducible-bytes--the-toolchain-caveat) below). It
never reads the clock, the filesystem outside the calls you make, the
network, or the host's locale: everything the document needs — data, runtime
parameters, and fonts — is handed in explicitly.

**Documentation.** The [rendering library guide](../docs/rendering-library.md) covers installation,
inputs, warnings and errors, the template features that change output, and every exported API. The
[`.folio` format reference](../docs/folio-format.md) defines the template file, and the
[expression reference](../docs/expression-reference.md) the syntax inside `{{ }}`.

## Your first PDF

Two calls — load a template, render — and the font set the render needs
comes from a single, no-argument expression: `fonts.Shipped()`. This is
`example_test.go` in full, reproduced here **verbatim** — and that word is
mechanically checked, not promised: `TestREADMEExampleBlockMatchesSource`
compares this fenced block byte-for-byte against the file, so the two
cannot drift. It is a real, compiled, executed Go testable example
(`go test -run Example -v ./...` runs it and checks its `// Output:`
comment matches):

```go
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
```

The template's own `fonts` fallback chain names `"Noto Sans"` — one of the
three face names `fonts.Shipped()` returns (`"Noto Sans"`, `"Noto Sans
Thai"`, `"Noto Sans SC"`) — so the chain resolves against the shipped set
with no font bytes read from disk by the caller at all. An integrator with
special typography needs (a brand face, a script the shipped set doesn't
cover) still builds a `folio8.FontSet` by hand from their own bytes, exactly
as before Story 2.2 — `fonts.Shipped()` is a convenience for the common
case, not the only way to construct a `FontSet`.

### Why `folio8` and `folio8/fonts` are separate imports

This is load-bearing, not ceremony. `package folio8` (the render engine,
module root) and `package fonts` (the shipped face data, `folio-go/fonts/`)
are two separate imports, and `folio8` never imports `fonts` — only the
other direction. A caller who wants the shipped set opts in explicitly with
a second import line.

If `folio8` re-exported the shipped set itself, importing `folio8` would mean
importing `folio8/fonts`, and the **~11.3 MB** of embedded Noto face data
would be compiled into **every** consumer's binary, whether or not that consumer
ever calls `fonts.Shipped()` — Go does not strip an `//go:embed`'d
byte slice out of a binary just because nothing reads the variable at
runtime. Two callers pay that cost involuntarily under a merged import:

- An integrator who supplies their own single Latin face and has no use
  for Thai or Simplified-Chinese coverage still gets all three embedded.
- Every wasm build — where binary size is a shipped, user-visible cost,
  not just a build artifact on a server — pays for CJK support it may
  never render a single glyph of.

Keeping `fonts` a separate, leaf package that only *depends on* `folio8`
(never the reverse) means the ~11.3 MB is opt-in: it's in your binary only
if your own code imports `folio-go/fonts` and calls `Shipped()`.

> **Two different numbers, and mixing them up understates this argument by
> about 2×.** `go:embed` stores **raw** bytes, so what lands in a binary is
> the faces' uncompressed size — **11,289,880 B ≈ 11.3 MB**. The **~9 MB**
> figure quoted elsewhere in the planning documents is a *compressed
> download* budget, which is the right unit for a wasm payload over the
> wire (**5.07 MB** for the three faces at `brotli -q 11`) and the wrong
> one for a sentence about binary size. Use the raw figure for the binary
> argument; use the compressed figure only where a download is meant.

### The `locale` field

A `.folio` document carries a top-level `"locale"`, and it is the document's
own property — folio8 never consults the host's locale, environment or
system settings for it (that is what makes a render reproducible across
machines, AD-1).

`locale` selects the language-dependent behaviour folio8 applies to the
document's text: number and date formatting conventions, and the
line-breaking rules used for scripts that need them. It is a closed set —
an unrecognised value is a load error naming the field, never a silent
fallback to a default, because a document that quietly formatted its
currency under the wrong convention would be wrong in a way nobody
notices until a customer does.

`"th"` is what engages Thai dictionary-based break opportunities; `"en"` is
the ordinary Latin case. `"ja"` renders, with the caveat immediately below
— which is worth reading before choosing it.

### A known limitation: Japanese glyph forms

`locale: "ja"` **renders** correctly with the shipped font set — `"Noto Sans
SC"` is a Pan-CJK face, so it carries kana and the ideograph set Japanese
text draws from, and no `MISSING_GLYPH`-style diagnostic fires (there is
real glyph coverage, so AC4's coverage diagnostic correctly stays silent).

What the shipped set does **not** do is give Japanese text Japanese glyph
*forms*. A meaningful minority of CJK ideographs are drawn with different
canonical shapes under the Simplified-Chinese and Japanese conventions —
"Noto Sans SC" draws the Simplified-Chinese shape for those, not the
Japanese one. A `ja` document rendered against `fonts.Shipped()` today is
therefore readable, but shows Simplified-Chinese glyph forms for that
subset of characters — a **glyph-form quality gap**, not tofu, not a
missing-coverage bug, and not something AC4 is expected to catch (AC4
answers "is there a glyph at all", not "is it the regionally correct
shape").

This is a stated, accepted limitation, not an oversight: fixing it means
shipping a dedicated Noto Sans JP face, which is not size-neutral — Noto
Sans JP would add several megabytes on top of the shipped set's measured
**5.07 MB** compressed download (the figure NFR7's ~9 MB budget is written
in), for every consumer, including the wasm build this repo already treats
size as a real cost for (see above). Until that
trade-off is made deliberately, `ja` documents get correct Japanese text
*coverage* through Noto Sans SC, with Simplified-Chinese glyph *shapes*
where the two conventions diverge.

### A stated limitation: only *declared* values are protected from breaking

Text wraps inside its element's declared `width`. Where a line may end is
decided per script: Latin breaks after whitespace, CJK between adjacent
Han or kana characters, and Thai — which is written without interword
spaces — from an embedded dictionary. Where the dictionary cannot account
for a stretch of Thai, folio8 keeps that stretch whole rather than guessing.

**folio8 does not try to recognise names, and this is deliberate.** Thai
surnames are coined by law, one per family, out of ordinary everyday
words: `ศรีสุข` as a surname is character-for-character the two common
words `ศรี` and `สุข`. Measured over the shipped 62,107-word list, **83%
of the personal names** in folio8's own test corpus decompose entirely into
dictionary entries. No amount of dictionary can separate a person's name
from the words it was built from, so a bigger dictionary, a surname list,
and "never break Thai at all" were each considered and rejected.

Instead, a template **declares** which data paths hold values that must
never be split, in a document-level list:

```json
"unbreakableValues": ["customer.name", "transactions.payee"]
```

Every value substituted from a listed path stays on one line. Literal text
around the placeholder is unaffected, so `"Statement for {{customer.name}}"`
still wraps between *Statement* and *for*.

**What this does not reach, stated plainly:** a Thai name that appears
inside **free-form text** — a transaction description reading "โอนให้ศรีสุข",
or "transfer to Srisuk" — is not a bound value, carries no declaration, and
remains breakable. That is the accepted cost of the mechanism. It is
disclosed here rather than fixed, because the alternative is guessing, and
guessing wrong on a customer's name across fifty thousand statements is the
failure this design exists to prevent.

Two further narrowings, named so you do not have to discover them:
**folio8's Latin breaking is not UAX #14** (no hyphenation, no break at
`-`, none of the contextual pair rules), and **kinsoku is not
implemented** for CJK (a line may begin with `，` or end with an opening
bracket).

## Writing straight to a destination: `RenderTo`

`Render` returns a `[]byte`. `RenderTo` writes the same bytes to an
`io.Writer` you already have open — an HTTP response, a file, anything —
instead of handing you a byte slice to copy yourself:

```go
func statementHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/pdf")
	diagnostics, err := folio8.RenderTo(w, tpl, data, params, fontSet)
	if err != nil {
		// A render error is returned before anything is written; a write
		// error may follow a partially written response.
		log.Printf("statement: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, d := range diagnostics {
		log.Printf("statement warning %s: %s", d.Code, d.Message)
	}
}
```

`RenderTo` returns the render's warnings and an error. See
[Writing to an `io.Writer`](../docs/rendering-library.md#writing-to-an-iowriter) for choosing an
HTTP status before the response starts.

`RenderTo` takes the exact same arguments as `Render`, in the exact same
order, with the writer added at the front. There is no options struct — an
options struct would let the font set be *omitted* at compile time, and a
render missing its fonts is exactly the mistake AD-8 exists to catch at
build time rather than at run time.

**`RenderTo` cannot stream the document out as it is produced, and this is
deliberate, not a missed optimisation.** The PDF's `/ID` is a hash taken over
the entire serialized body, and its cross-reference table records the byte
offset of every object in the finished file — both require the whole
document to exist before the first byte can be written. `RenderTo` builds
the complete document once (through the exact same code `Render` uses, so
the two are never able to drift apart), then performs one write of the
result — never a partial write left unchecked, and never a trailing write
carrying anything extra. If the writer you pass fails partway, returns
fewer bytes accepted than it was given, or otherwise misbehaves, `RenderTo`
reports it as an error rather than looking like it succeeded.

**"Never touches disk" is a capability, not a proof.** `RenderTo` writes
straight to the `io.Writer` you give it, and folio-go's own code never opens
a temporary file to get there — that's what makes serving a PDF from an HTTP
handler with no scratch file possible. What folio-go does not do is *forbid*
a caller from wiring a filesystem-backed `io.Writer` in front of `RenderTo`
themselves; that has always been the caller's own choice to make, the same
way it is for any `io.Writer`-based API. Read this as "you *can* serve a PDF
without touching disk", not as a sandboxed guarantee that nothing in the
process ever will.

## Data vs. Params

`Render` and `RenderTo` take **two** separate JSON inputs:

- **`Data`** — your report's data: the rows, totals and customer details a
  template's placeholders bind against, e.g. `{{customer.name}}`.
- **`Params`** — runtime values that are not part of the report itself, e.g.
  the date the statement was generated: `{{params.reportDate}}`.

They are kept apart on purpose. A placeholder whose first path segment is
`params` — `{{params.reportDate}}` — **always** resolves against the params
document, never against report data, no matter what report data happens to
contain. This means:

- **A top-level `"params"` key in your report data is legal, ordinary JSON,
  and simply unreachable by any binding.** It is your data; folio-go does
  not reject it, and it cannot be used to smuggle a value into
  `{{params.…}}`.
- **`{{params.x}}` with no params document supplied (or one missing that
  key) is an error**, naming the path and the element — the same way an
  absent report-data path is an error. A statement silently missing its
  report date is a worse failure than a render that refuses to produce one.
- Passing `nil` (or an empty `Params`) means "no runtime values supplied" —
  it is not treated as malformed JSON.

Both `Data` and `Params` are decoded through the exact same path, so a
number in either one keeps its author-written precision (no float64
rounding) — there is no separate, second decoder for params that could
someday drift from how report data is read.

`Data` and `Params` are distinct Go types with the same underlying
representation (`[]byte`), and that distinction is enforced by the
compiler, not by convention: swapping them at a call site —
`Render(t, params, data, fonts)` — fails to build. In a library whose
acceptance fixture is a bank statement, that has to be a compile error, not
a support ticket filed after the fact.

## Reproducible bytes — the toolchain caveat

folio-go's whole reason to exist is that the same template and the same
inputs produce the same bytes, every time. `folio-go/go.mod` pins an exact
Go toolchain (`toolchain go1.26.0`) for that reason.

**Where the golden hashes actually live.** folio-go's own regression fixtures
(`folio-go/testdata/`) and the cross-platform byte-identity matrix
(`hashmatrix/`, verified at each epic boundary — see
[What's not here yet](#whats-not-here-yet)) are what pin folio-go's output
against itself. Anyone recording those hashes — or their own — in an
external test suite is the reader the next paragraph is for.

**That pin only binds folio-go's OWN build.** Go's `toolchain` directive
binds the *main* module of whatever `go build` you run — not every module in
the dependency graph. If you `go get` folio-go into your own application,
your application is the main module, and **your own `go.mod`'s `toolchain`
line (or your `GOTOOLCHAIN` setting) decides which Go compiles folio-go**,
not folio-go's own pin. An integrator who records folio-go's golden hashes
in their own test suite, expecting them to hold indefinitely, needs to pin
their *own* toolchain the same way — folio-go's pin alone does not do it for
them.

(If you look at `folio-go/go.mod`, you'll notice the `go` directive sits
*below* the `toolchain` directive, and at a lower version. That is
deliberate, not a stray typo to "fix": Go silently strips a `toolchain` line
that is not strictly greater than the `go` line on `go mod tidy`, which
would quietly undo the pin this section is about.)

## What's not here yet

- **A dedicated Japanese glyph-form face.** Story 2.2's shipped set covers
  `ja` text via the Pan-CJK "Noto Sans SC" face (correct coverage,
  Simplified-Chinese glyph shapes) — see "A known limitation: Japanese
  glyph forms" above. A Noto Sans JP face would fix the shapes at the cost
  of roughly 7 MB more in every consumer's download.
- **A PDF metadata date that nobody asked for.** This is a deliberate
  absence, not a missing feature. Render with no date supplied by any route
  and you get a document with no `/CreationDate`, no `/ModDate` and no
  `/Info` dictionary at all (AD-7) — that is what makes two renders of the
  same input byte-identical. Supply one and it is emitted: pass a
  `documentDate` param, or set `SOURCE_DATE_EPOCH` in the environment and
  let the `folio8` command-line tool fill the param for you. The environment
  route is **fill-only** — a `documentDate` you supplied yourself, including
  an explicit `null`, is never displaced by `SOURCE_DATE_EPOCH`.
- **The cross-platform byte-identity matrix** (Linux amd64/arm64, WASM under
  Node, compared against local `darwin/arm64`) is verified **at each epic
  boundary**, not on every change — most recently at the Epic 4 boundary,
  over fifteen documents on all four targets. See `hashmatrix/` and
  `matrix_test.go`.
