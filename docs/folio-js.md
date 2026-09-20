<!-- twin:begin -->
# folio-js

folio-js is the folio8 engine for Node, published to npm as `folio8`. It turns a `.folio` template, JSON data and runtime parameters into PDF bytes in process, with the engine itself — compiled to WebAssembly — doing the rendering. There is no service to run, no binary to shell out to and no rendering logic written in JavaScript.

The same template, data, params and font set produce the same bytes here as under the Go library, the `folio8` command-line tool and the designer's preview. That is a tested property rather than a claim: the package renders the fixture corpus and compares SHA-256 against the committed expected hashes on every supported Node version.

**Every entry point is a promise.** The engine runs on Go's `js/wasm` target, so `parseTemplate`, `loadTemplate`, `render`, `renderTo`, `validate` and `parameterReferences` all resolve rather than return, and are written with `await`. Go's synchronous shape is what a Go caller wants; this is what a Node caller wants, because it keeps a large render offloadable to a worker thread.

Three companion references hold the rules this guide does not repeat: [the `.folio` format](folio-format.md) is every field of a template, [expressions](expression-reference.md) is the syntax inside the double braces, and [the rendering library guide](rendering-library.md) covers the same engine from Go, including the shared [diagnostic code registry](rendering-library.md#diagnostic-codes). [folio-dotnet](folio-dotnet.md) is this same library for .NET.

## Install

```sh
npm install folio8
```

Nothing else is needed. No Go, no C compiler, no build step, and no network access after the install: the prebuilt WebAssembly engine and all eleven shipped font faces travel inside the package, and the package declares no dependencies of its own.

### The Node version floor

**Node 22.12 is the floor.** That is the version `engines.node` declares, and it is also the lowest version the corpus is rendered on in continuous integration. A test holds the two equal, so the supported range and the tested range cannot drift apart.

Below the floor, `npm install` reports `EBADENGINE` — a warning by default, and a refusal when `engine-strict` is set — and `require` of this package throws `ERR_REQUIRE_ESM`, because requiring an ES module is exactly the capability that arrives in 22.12. Nothing below 22.12 is tested, and nothing below it is supported.

### ESM, and `require` from CommonJS

**The package is ESM only.** It publishes `import` entry points and no CommonJS build:

```js
import { loadTemplate, render } from 'folio8'
import { shipped } from 'folio8/fonts'
```

A CommonJS caller on Node 22.12 or newer reaches the same exports through `require`, which Node resolves with its support for requiring an ES module. There is no separate CommonJS entry point to name, and nothing in the package is reachable any other way:

```js
const { loadTemplate, render } = require('folio8')
const { shipped } = require('folio8/fonts')
```

### Node only

Node is the only target. There is no browser build and no bundler build, which is a scope choice rather than a technical limit — the designer runs this same engine in a browser worker.

## Your first PDF

Put a template at `invoice.folio` and its data at `invoice.json`, then:

```js
import { readFile, writeFile } from 'node:fs/promises'
import { loadTemplate, render } from 'folio8'
import { shipped } from 'folio8/fonts'

const template = await loadTemplate('invoice.folio')
const data = JSON.parse(await readFile('invoice.json', 'utf8'))

const { bytes, diagnostics } = await render(template, data, null, await shipped())

await writeFile('invoice.pdf', bytes)
for (const d of diagnostics) console.warn(`${d.severity} ${d.code}: ${d.message}`)
```

`loadTemplate` reads and parses the file; `render` resolves to the PDF bytes and to every warning the engine raised on the way. The third argument is the params object, and `null` says the template reads none.

**That snippet is executed, not illustrated.** It is the one the package's own offline-install test runs: it packs the tarball, installs it into an empty project with the network refused, renders a corpus fixture with it, and compares the PDF's SHA-256 against the committed expected hash.

**Fonts are an explicit argument, always.** There is no default font set and no lookup on the machine the render runs on. `shipped` is a convenience that resolves the eleven faces packaged with folio-js — the same faces, byte for byte, that Go's `fonts.Shipped` returns. Build your own map instead whenever you want a different set.

**The map may be empty.** A template that carries every face it names — one saved with embedding on — has nothing for a font set to contribute, so `new Map()` is a legitimate call and not a caller error. Nothing is refused before the engine has tried to resolve the chains; an entry that genuinely cannot be resolved still fails with `TEXT_FACE_ABSENT`, located at the element.

**When a face is missing, choose what happens.** `render` and `validate` take an optional fifth argument of type `FaceFallback`; `renderTo` takes it sixth, after its own leading `writable`:

| Value | Behaviour |
|---|---|
| `'strict'` | The default, and what every call made before this argument existed does: a character covered by no present face of its chain, where some face that chain declares was never supplied, fails the render with `TEXT_FACE_ABSENT`. |
| `'substitute'` | The character is painted in a face this renderer actually holds — the document's own embedded assets first, then your map, each searched in face-name order, the first face that covers the character winning — and the warning `TEXT_FACE_SUBSTITUTED` names the element, the character, the face requested and the face painted. A renderer holding nothing that covers the character still fails with `TEXT_FACE_ABSENT`. |

```js
const { bytes, diagnostics } = await render(template, data, null, await shipped(), 'substitute')
for (const d of diagnostics) {
  if (d.code === 'TEXT_FACE_SUBSTITUTED') console.warn(d.message) // fail your own build on this if you want to
}
```

The choice is deterministic: the same inputs produce the same bytes on any machine. Anything other than `'strict'` or `'substitute'` is a `TypeError` from the binding, never a silent default.

**A lenient render may space its lines differently.** Under substitution any face the renderer holds may end up painted into an element whose chain has an unsupplied member, so such an element is given a line box tall enough for the tallest of them — the alternative is a substituted character overflowing its box with nothing reported. The leading of those elements therefore differs from the same document rendered strictly. An element whose chain is fully supplied is unaffected, and a strict render is unchanged in every case.

**Call `shipped` once and hold what it gives you.** The first call reads about 14 MB from the package and later calls resolve from memory, but each call returns its own map over those shared bytes, so deleting or replacing an entry cannot change what the next caller sees.

**A render blocks the event loop.** It runs synchronously inside WebAssembly for its whole duration. Offload a large document to a worker thread if the same process is also serving requests.

### Passing params

Params are the runtime values that are not report data — a run date, a branch code, a report title. They live in their own namespace, which a template reads as `params.branchCode`, and they can never be shadowed by report data. `documentDate` is the one reserved name: a template that prints a document date reads it from params, because nothing in the library reads the clock.

Build the object, hand it over as the third argument, and ask the template which names it wants:

```js
import { loadTemplate, parameterReferences, render } from 'folio8'
import { shipped } from 'folio8/fonts'

const template = await loadTemplate('statement.folio')
console.log(await parameterReferences(template))

const params = {
  documentDate: '2026-08-15T00:00:00Z',
  branchCode: '004',
  reportTitle: 'Monthly statement',
}

const { bytes } = await render(template, data, params, await shipped())
```

`documentDate` must be an RFC 3339 timestamp; anything else is a `DOCUMENT_DATE_INVALID` error. Params take the same three forms `data` does — raw bytes, a JSON string, or a plain object serialised for you.

## Warnings and errors

folio8 reports every problem as a `Diagnostic`, field for field the object the Go library and folio-dotnet produce for the same condition:

| Field | Meaning |
| --- | --- |
| `severity` | Either the string warning or the string error. The set is closed, and Go's unset zero value is not representable here. |
| `code` | A stable string from a closed registry, such as TEXT_CLIPPED_WIDTH. This is the field to dispatch on. |
| `elementId` | The template element the problem concerns, when there is one. |
| `dataPath` | The data path, or for a load failure the template field path, when there is one. |
| `message` | A human-readable sentence. Print it; never parse it. |

**A warning reaches `diagnostics`; an error is thrown.** A successful render never carries a diagnostic of severity error, and a thrown error never appears in `diagnostics` — the same split Go makes between `Result.Diagnostics` and a `RenderError`. The `diagnostics` array is always present, and empty rather than absent when nothing went wrong.

**`code` is the contract and `message` is not.** Codes are additive: once shipped, a code's string and its meaning never change. Message text is prose for a person to read, matches the Go library's word for word for the same condition, and carries no promise beyond that.

A rejection the engine raised as a known document condition is a `FolioRenderError` carrying the `Diagnostic` that caused it. Everything else — a value that is not a template, data that will not serialise, a stream that failed — rejects with an ordinary `TypeError` or `Error`:

```js
import { FolioRenderError, render } from 'folio8'

try {
  const { bytes, diagnostics } = await render(template, data, params, fonts)
  for (const d of diagnostics) {
    if (d.code === 'TEXT_MISSING_GLYPH') console.warn(`element ${d.elementId} lost a character: ${d.message}`)
  }
  return bytes
} catch (error) {
  if (error instanceof FolioRenderError) {
    if (error.diagnostic.code === 'BINDING_PATH_ABSENT') throw new Error(`data is missing ${error.diagnostic.dataPath}`)
    throw new Error(`${error.diagnostic.code}: ${error.message}`)
  }
  throw error
}
```

**A partial font set now refuses.** If you pass a `fonts` object that is short of a face a document's chain declares, a character no supplied face covers rejects with a `FolioRenderError` carrying `TEXT_FACE_ABSENT` and produces no PDF, where earlier releases resolved with a `TEXT_MISSING_GLYPH` warning and a PDF with those characters silently missing. The message names every absent face. `TEXT_MISSING_GLYPH` still means what it always did: every declared face was supplied and none of them covers the character.

Every code, with its string value and its meaning, is listed under [diagnostic codes](rendering-library.md#diagnostic-codes) in the rendering library guide. The registry belongs to the engine, so it reads the same from all three languages and a caller can port between them.

`validate` answers the same question without producing a document. It takes raw template bytes, checks them against data, params and fonts, and resolves to the diagnostic list the engine would have raised — rejecting on an error exactly as `render` does.

## API reference

Six functions and `version` come from `folio8`; `shipped` comes from `folio8/fonts`. Everything else named here is a type, and TypeScript declarations for all of it ship with the package.

### Rendering and validation

#### parseTemplate

```ts
parseTemplate(bytes: Uint8Array | string): Promise<Template>
```

Parses `.folio` bytes into a `Template`, which is Go's `ParseTemplate`. This is the primary constructor: bytes in, template out. A malformed document rejects with `FolioRenderError`.

#### loadTemplate

```ts
loadTemplate(path: string): Promise<Template>
```

Reads the file at the path and parses it, which is Go's `LoadTemplate`. It is the only call in the library that touches the filesystem on your behalf.

#### render

```ts
render(tpl: Template, data: Data, params: Params | null | undefined, fonts: FontSet): Promise<RenderResult>
```

Renders the document and resolves to its bytes plus its warnings, which is Go's `Render`.

#### renderTo

```ts
renderTo(writable: Writable, tpl: Template, data: Data, params: Params | null | undefined, fonts: FontSet): Promise<Diagnostic[]>
```

Renders, writes the PDF to a Node `Writable`, and resolves to the warnings once the write completes. This is Go's `RenderTo` with a stream in place of an `io.Writer`, for HTTP responses and large documents. The stream is never ended and never destroyed, and nothing is written when the render fails.

#### validate

```ts
validate(bytes: Uint8Array | string, data: Data, params: Params | null | undefined, fonts: FontSet): Promise<Diagnostic[]>
```

Checks a template against data, params and fonts without producing a document, which is Go's `Validate`. It takes raw template bytes rather than a parsed `Template`, matching Go.

#### parameterReferences

```ts
parameterReferences(tpl: Template): Promise<string[]>
```

The sorted, de-duplicated top-level parameter names the template reads, which is Go's `ParameterReferences`. Use it to build a params object without reading the template by hand.

#### version

```ts
version: string
```

The folio8 engine version the packaged WebAssembly was built from. A test holds it equal to the value the engine reports about itself, so it can never describe a build it did not come from.

### Fonts

#### shipped

```ts
shipped(): Promise<FontSet>
```

The eleven faces Go's `fonts.Shipped` returns, read from the files inside the package: Roboto and Noto Sans in regular, bold, italic and bold italic, Noto Sans Thai in regular and bold, and Noto Sans SC. Keys and bytes are the engine's own, so a document rendered from this set here and from `fonts.Shipped` in Go produces identical PDF bytes.

It is imported from the `folio8/fonts` entry point rather than from the package root, so a caller who supplies their own faces never pays for reading these.

These are the keys, and a template's font chain must name one of them verbatim — the engine never derives `Roboto Bold` from `Roboto`:

| Family | Face names (map keys) |
| --- | --- |
| Noto Sans | `Noto Sans`, `Noto Sans Bold`, `Noto Sans Italic`, `Noto Sans Bold Italic` |
| Noto Sans Thai | `Noto Sans Thai`, `Noto Sans Thai Bold` |
| Noto Sans SC | `Noto Sans SC` |
| Roboto | `Roboto`, `Roboto Bold`, `Roboto Italic`, `Roboto Bold Italic` |

To use your own faces, build the map yourself, optionally starting from the shipped set:

```js
const fonts = await shipped()
fonts.set('Acme Display', new Uint8Array(await readFile('AcmeDisplay.ttf')))
```

**The shipped faces are licensed under the SIL Open Font License 1.1**, and each face's licence text and provenance notice travel beside it in the package under `fonts/`. Faces you supply yourself, or embed in a template, are under their own licences: a template that embeds a font redistributes that font with the file. The Go guide's [fonts](rendering-library.md#fonts-1) section carries the same rules for the same face set.

### Types

#### Template

An opaque parsed template. It holds the engine's canonical bytes and offers no way to read or change the document — editing is the designer's surface and is deliberately not in this library. Obtain one from `parseTemplate` or `loadTemplate`; constructing one directly throws.

#### Data and Params

```ts
type Data = Uint8Array | string | object
type Params = Uint8Array | string | object
```

Both are JSON. Pass raw bytes, a JSON string, or a plain object that the library serialises for you. `Data` is the report data; `Params` carries the runtime values that are not report data, such as `documentDate`. Params are optional: pass `null` or `undefined` when the template reads none.

#### FontSet

```ts
type FontSet = Map<string, Uint8Array>
```

Face name to font file bytes, which is Go's `FontSet`. It is always an argument and never ambient.

#### RenderResult

```ts
interface RenderResult {
  bytes: Uint8Array
  diagnostics: Diagnostic[]
}
```

What `render` resolves to: the PDF, and every warning in the engine's order.

#### Diagnostic and Severity

```ts
type Severity = 'warning' | 'error'

interface Diagnostic {
  severity: Severity
  code: string
  elementId: string
  dataPath: string
  message: string
}
```

One engine diagnostic, field for field Go's `Diagnostic`, with the names in the casing JavaScript expects. `Severity` is a closed set of two dispositions.

### Errors

#### FolioRenderError

```ts
class FolioRenderError extends Error {
  readonly diagnostic: Diagnostic
}
```

Thrown when the engine aborts on a known document condition, which is Go's `RenderError`. Its `message` is the engine's error text and its `diagnostic` carries the code, the element and the data path. An error arrives as a throw and never as an entry in `diagnostics`.

## Determinism

Nothing here reads the clock, the locale, the environment or the network, and nothing touches the filesystem beyond the `loadTemplate` call you make yourself. A document that needs a date takes it as an ordinary param named `documentDate`; the `SOURCE_DATE_EPOCH` convenience belongs to the command-line tool and is not honoured by this library.

Templates are immutable here. Neither this library nor folio-dotnet exposes a way to change one.
<!-- twin:end -->

Source of truth: this file. The published page at `docs/folio-js.html` carries the same text.
