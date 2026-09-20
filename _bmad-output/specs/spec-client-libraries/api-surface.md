# API surface — folio-js and folio-dotnet

The render-and-validate subset of `folio-go`, and its shape in each language.
This is the complete public contract for both libraries: an item absent here is
absent from the library, and CAP-8/CAP-9 require every row to appear in that
language's API reference page.

Names are idiomatic per language; **behaviour is identical**, including
diagnostic codes, severities and message text.

**folio-js is promise-based throughout.** Every folio-js entry point below
returns a `Promise` — `await parseTemplate(…)`, `await render(…)` — over the
`js/wasm` target. folio-dotnet is synchronous, matching Go.

## Entry points

| `folio-go` | folio-js | folio-dotnet | Notes |
| --- | --- | --- | --- |
| `ParseTemplate(b []byte) (*Template, error)` | `parseTemplate(bytes)` | `Template.Parse(byte[])` | The primary constructor. Bytes in, template out. |
| `LoadTemplate(path string) (*Template, error)` | `loadTemplate(path)` | `Template.Load(string)` | Convenience over the filesystem. The only API in either library that touches disk on the caller's behalf. |
| `Render(t, d, p, f, fallback ...FaceFallback) (Result, error)` | `render(tpl, data, params, fonts, fallback?)` | `Folio8.Render(Template, Data, Params, FontSet, FaceFallback)` | Returns bytes plus warnings. `fallback` is optional and defaults to strict. |
| `RenderTo(w io.Writer, t, d, p, f, fallback ...FaceFallback) ([]Diagnostic, error)` | `renderTo(writable, tpl, data, params, fonts, fallback?)` — a Node `Writable` | `Folio8.RenderTo(Stream, …, FaceFallback)` | For HTTP responses and large documents. Streams rather than materialising. `fallback` is forwarded verbatim. |
| `Validate(b []byte, d, p, f, fallback ...FaceFallback) ([]Diagnostic, error)` | `validate(bytes, data, params, fonts, fallback?)` | `Folio8.Validate(byte[], …, FaceFallback)` | Takes raw template bytes, not a parsed `Template`, matching Go. It takes the same `fallback`, because a prediction made under a different selector is not a prediction of that render. |
| `ParameterReferences(tpl) ([]string, error)` | `parameterReferences(tpl)` | `Template.ParameterReferences()` | Which params a template expects — needed to build a params object without reading the template by hand. |

## Value types

| `folio-go` | folio-js | folio-dotnet | Notes |
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
3. **Fonts are an explicit argument, and the argument may be empty.** There is
   no default font set and no ambient lookup — the engine never goes looking
   for fonts on the machine it runs on. But a document that carries every face
   it names has nothing for a font set to contribute, so **no binding may
   refuse an empty set before resolution has been attempted**; an entry that
   genuinely cannot be resolved still fails as `TEXT_FACE_ABSENT`. Passing
   `null` where the language has one is still a caller error.
   - **The fallback selector is the same closed set in all three.** Strict
     (the default, and the value a caller who passes nothing gets) refuses;
     substitute paints a face the renderer holds and warns with
     `TEXT_FACE_SUBSTITUTED`. A value outside the set is **refused by the
     binding** — Go a named error, JS a `TypeError`, .NET an
     `ArgumentException` — never clamped.
4. **Nothing reads the clock, the environment, or the network.** `documentDate`
   arrives through `Params` or it is absent. `SOURCE_DATE_EPOCH` is a CLI
   behaviour and is not honoured by either library.
5. **Templates are immutable here.** Neither library exposes a way to change a
   template — that is the canvas surface, and it is a non-goal.
