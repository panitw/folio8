# folio8 for Node

The folio8 engine for Node. Turn a `.folio` template plus JSON data into PDF
bytes in-process, with the folio8 engine itself — compiled to WebAssembly —
doing the rendering.

The same template, data, params and font set produce the same PDF bytes here
as under the Go engine, the `folio8` CLI and the designer's preview. That is a
tested property: this package's suite renders corpus fixtures and compares
SHA-256 against the committed expected hashes.

```sh
npm install folio8
```

Nothing else is needed. No Go, no C compiler, no build step, no network access
after the install — the prebuilt `.wasm` and all eleven shipped font faces are
in the package.

**Requires Node 22.12 or newer.** The package is **ESM only** — there is no
`require()` entry point. `import` it from ESM, or from CommonJS on Node 22.12
and newer with `require('folio8')`, which Node resolves through its
require(esm) support.

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

`render` rejects when the engine cannot produce a document; everything it can
render around arrives in `diagnostics` as a warning instead. That snippet is
the one this package's offline-install test runs against a corpus fixture, so
it is checked rather than illustrated.

## The API

| Export | What it does |
| --- | --- |
| `parseTemplate(bytes)` | Parses `.folio` bytes into a `Template`. Go's `ParseTemplate`. |
| `loadTemplate(path)` | Reads and parses a `.folio` file. Go's `LoadTemplate`. |
| `render(tpl, data, params, fonts)` | Resolves to `{ bytes, diagnostics }`. Go's `Render`. |
| `renderTo(writable, tpl, data, params, fonts)` | Renders, then writes the PDF to a Node `Writable`. Go's `RenderTo`. |
| `validate(bytes, data, params, fonts)` | Diagnostics without rendering. Go's `Validate`. |
| `parameterReferences(tpl)` | The param names a template reads. |
| `version` | The folio8 engine version this package's wasm was built from. |
| `shipped()` — from `folio8/fonts` | The shipped font set, as a `Map<string, Uint8Array>`. |

`params` is optional: pass `null` or `undefined` when the template reads none.
`data` and `params` each take raw `Uint8Array` JSON, a JSON string, or a plain
object this package serialises for you.

Diagnostic codes, severities and message text are the engine's, identical
across this package, the .NET package and Go, so a caller can port between
them.

A render runs synchronously inside WebAssembly and blocks the event loop while
it runs. Offload a large one to a worker thread if that matters to you.

Node only — there is no browser or bundler build.

## Fonts

Fonts are always an explicit argument. `shipped()` is a convenience, not a
default: it reads the packaged faces and resolves to a `Map` equal to the one
the engine's `fonts.Shipped()` gives a Go caller, keys and bytes alike. The
face bytes are read once and cached, so calling it per render costs one read of
each file for the life of the process — but **each call returns its own `Map`**
over those cached bytes, so mutating what you are handed cannot change what the
next caller gets. Build your own `Map<string, Uint8Array>` instead whenever you
want a different set.

Eleven faces ship, about 14 MB, and they are the reason the tarball is large.
`NotoSansSC-Regular.ttf` alone is 10 MB of it.

| Face name | File | Licence |
| --- | --- | --- |
| `Noto Sans` | `NotoSans-Regular.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Bold` | `NotoSans-Bold.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Italic` | `NotoSans-Italic.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Bold Italic` | `NotoSans-BoldItalic.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Thai` | `NotoSansThai-Regular.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Thai Bold` | `NotoSansThai-Bold.ttf` | SIL Open Font License 1.1 |
| `Noto Sans SC` | `NotoSansSC-Regular.ttf` | SIL Open Font License 1.1 |
| `Roboto` | `Roboto-Regular.ttf` | SIL Open Font License 1.1 |
| `Roboto Bold` | `Roboto-Bold.ttf` | SIL Open Font License 1.1 |
| `Roboto Italic` | `Roboto-Italic.ttf` | SIL Open Font License 1.1 |
| `Roboto Bold Italic` | `Roboto-BoldItalic.ttf` | SIL Open Font License 1.1 |

Every face ships under the **SIL Open Font License, Version 1.1**. Each face's
unmodified licence text (`LICENSE-OFL.txt`) and its provenance notice
(`NOTICE.md`) sit beside the `.ttf` in this package, under `fonts/<face>/`.

## Licence

This package itself is MIT — see `LICENSE`. The shipped faces are not: they are
OFL-1.1, as above.
