# API surface — folio-js and folio-dotnet

The render-and-validate subset of `folio8-go`, and its shape in each language.
This is the complete public contract for both libraries: an item absent here is
absent from the library, and CAP-8/CAP-9 require every row to appear in that
language's API reference page.

Names are idiomatic per language; **behaviour is identical**, including
diagnostic codes, severities and message text.

**folio-js is promise-based throughout.** Every folio-js entry point below
returns a `Promise` — `await parseTemplate(…)`, `await render(…)` — over the
`js/wasm` target. folio-dotnet is synchronous, matching Go.

## Entry points

| `folio8-go` | folio-js | folio-dotnet | Notes |
| --- | --- | --- | --- |
| `ParseTemplate(b []byte) (*Template, error)` | `parseTemplate(bytes)` | `Template.Parse(byte[])` | The primary constructor. Bytes in, template out. |
| `LoadTemplate(path string) (*Template, error)` | `loadTemplate(path)` | `Template.Load(string)` | Convenience over the filesystem. The only API in either library that touches disk on the caller's behalf. |
| `Render(t, d, p, f) (Result, error)` | `render(tpl, data, params, fonts)` | `Folio8.Render(Template, Data, Params, FontSet)` | Returns bytes plus warnings. |
| `RenderTo(w io.Writer, t, d, p, f) ([]Diagnostic, error)` | `renderTo(writable, tpl, data, params, fonts)` — a Node `Writable` | `Folio8.RenderTo(Stream, …)` | For HTTP responses and large documents. Streams rather than materialising. |
| `Validate(b []byte, d, p, f) ([]Diagnostic, error)` | `validate(bytes, data, params, fonts)` | `Folio8.Validate(byte[], …)` | Takes raw template bytes, not a parsed `Template`, matching Go. |
| `ParameterReferences(tpl) ([]string, error)` | `parameterReferences(tpl)` | `Template.ParameterReferences()` | Which params a template expects — needed to build a params object without reading the template by hand. |

## Value types

| `folio8-go` | folio-js | folio-dotnet | Notes |
| --- | --- | --- | --- |
| `Data []byte` | `Uint8Array \| string \| object` | `Data` (wraps `byte[]`) | JSON. JS accepts an object and serialises; .NET 4.6 has no `System.Text.Json`, so the wrapper takes bytes or a string and leaves serialiser choice to the caller. |
| `Params []byte` | same as `Data` | `Params` | Runtime values that are not report data. Carries `documentDate`. |
| `FontSet map[string][]byte` | `Map<string, Uint8Array>` / `fonts.shipped()` | `FontSet` (`IDictionary<string, byte[]>`) / `Fonts.Shipped()` | Explicit argument, never ambient. `shipped()` resolves embedded bytes under Node and fetches from the app origin in the browser — same signature, different delivery. |
| `Result{Bytes, Diagnostics}` | `{ bytes, diagnostics }` | `RenderResult` | `diagnostics` is always present and empty rather than null, mirroring Go's "empty is nil, one representation" rule. |
| `Diagnostic{Severity, Code, ElementID, DataPath, Message}` | same fields, camelCase | `Diagnostic` | `code` is the stable registry string and the thing callers dispatch on. `message` is prose and must never be parsed. |
| `Severity` (`Warning`, `Error`) | `'warning' \| 'error'` | `Severity` enum | Two dispositions, closed set. The unset zero value is not representable in either binding. |
| `RenderError{Diagnostic, Err}` | thrown `FolioRenderError` | `FolioRenderException` | An error aborts the render and arrives as a throw carrying its `Diagnostic`, not in `diagnostics`. |

## Rules that hold in both languages

1. **A warning reaches `diagnostics`; an error is thrown.** A successful render
   never contains a `Severity.Error` diagnostic, and a thrown error never
   appears in a result — the same split `Result.Diagnostics` and `*RenderError`
   make in Go.
2. **`code` is the contract, `message` is not.** Callers dispatch on `code`;
   message text is for humans and carries no stability promise beyond matching
   Go's for the same condition.
3. **Fonts are an explicit argument.** There is no default font set and no
   ambient lookup. Omitting fonts is a caller error, not a fallback.
4. **Nothing reads the clock, the environment, or the network.** `documentDate`
   arrives through `Params` or it is absent. `SOURCE_DATE_EPOCH` is a CLI
   behaviour and is not honoured by either library.
5. **Templates are immutable here.** Neither library exposes a way to change a
   template — that is the canvas surface, and it is a non-goal.
