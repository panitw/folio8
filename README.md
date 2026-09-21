# folio8

folio8 turns a **template** plus **JSON data** into a **PDF** — byte-identically,
every time, on a given build toolchain. It is a report designer and rendering
engine in the space JasperReports occupies, built around four commitments:

- **JSON-first.** folio8 never talks to a database. Your application prepares the
  data and hands it over; folio8 renders it.
- **Deterministic.** The same template and the same data produce the same bytes.
  No clock, no locale, no network, no filesystem beyond the calls you make.
- **Portable templates.** A `.folio` file is text. It diffs, it reviews, it lives
  in git, and a person or an agent can edit it without opening the designer.
- **A library, not a service.** `folio-go` is the reference engine and a normal Go
  dependency. The designer is a static page that runs that same engine in your
  browser.

The core workflow the project exists to prove is **Design → Bind → Preview →
Render**, with the preview being the *actual* production document rather than an
approximation of it.

The designer is live at **<https://folio8.report>** — design a `.folio` template
there, then render it from Go, Node (`npm install folio8`) or .NET
(`dotnet add package folio8`).

**What the hosted designer sends, and what it never sends.** Your templates, your
data and your rendered PDFs stay on your machine: the engine runs as WebAssembly
in your browser, and there is no server, no account and no upload. The hosted
build does load Google Tag Manager, which counts page loads and four fixed
actions — open template, export, font import, preview — and nothing else. No file
name, template name, font name or document value is ever part of an event; that
is enforced in the code by a closed type, not by convention. Running the designer
yourself sends nothing at all unless you set `VITE_GA_CONTAINER_ID` on your own
build. The decision, and the bound it was allowed under, are recorded as AD-27 in
the [architecture spine](_bmad-output/planning-artifacts/architecture/architecture-folio-2026-08-23/ARCHITECTURE-SPINE.md).

---

## Repository layout

| Path | What it is |
| --- | --- |
| [folio-go/](folio-go/) | The rendering engine and reference implementation — expression evaluation, layout, pagination, PDF output. A Go module: `github.com/panitw/folio8/folio-go`. See its [README](folio-go/README.md). |
| [folio-go/cmd/folio8/](folio-go/cmd/folio8/) | The `folio8` CLI: `validate` and `render`, and nothing else. |
| [folio-go/wasm/cmd/engine/](folio-go/wasm/cmd/engine/) | The designer's js/wasm entry point over the internal session engine in `folio-go/internal/wasm` — the same engine, compiled to wasm. Not public API. |
| [folio-designer/](folio-designer/) | The visual designer: React + Vite, running the wasm engine in a worker. No server, no account, no upload — your templates, data and rendered PDFs never leave your machine. On usage measurement in the hosted build, see above. |
| [fixtures/](fixtures/) | The golden corpus — template, data, params and the expected PDF for each fixture document. These bytes are the contract every renderer conforms against. |
| [lint/](lint/) | The guardrails that fail the build: architecture/import rules, the float ban, and the third-party licence check ([MANIFEST.md](lint/MANIFEST.md)). A separate Go module. |
| [hashmatrix/](hashmatrix/) | A deliberately-broken floating-point probe, kept out of the guards' reach, that proves the cross-target matrix can actually *detect* divergence. See its [README](hashmatrix/README.md). |
| [tools/fontgen/](tools/fontgen/) | Derives the shipped static faces from upstream variable builds. The outputs are committed; this exists so the derivation can be replayed. |
| [docs/](docs/) | User documentation, the source of truth: the [rendering library guide](docs/rendering-library.md), the [folio-js guide](docs/folio-js.md) (the npm package `folio8`), the [folio-dotnet guide](docs/folio-dotnet.md) (the NuGet package `folio8`), the [`.folio` format reference](docs/folio-format.md), the [expression reference](docs/expression-reference.md), and the original [MVP plan](docs/folio8-mvp-plan.md). |
| [_bmad-output/](_bmad-output/) | Planning and delivery record: PRD, architecture spine, specs, epics, and [sprint status](_bmad-output/implementation-artifacts/sprint-status.yaml). |

Three independent Go modules (`folio-go`, `lint`, `hashmatrix`) with no
`go.work` between them. Each builds and tests from its own directory.

---

## Quick start

### Install

folio8 ships as three packages. They are one engine — the Go source, and that
same source compiled to WebAssembly and to a C ABI — so the same template, data,
params and font set produce the same PDF bytes whichever you pick.

| Build | Install | Needs |
| --- | --- | --- |
| **folio-go** — the reference engine | `go get github.com/panitw/folio8/folio-go@v1.0.0` | Go 1.25 or newer; byte-identity is pinned to the go1.26.0 toolchain |
| **folio-js** — npm `folio8` | `npm install folio8` | Node 22.12 or newer, ESM only |
| **folio-dotnet** — NuGet `folio8` | `dotnet add package folio8` | .NET Framework 4.6+ or .NET Core 2.0+, **Windows x86/x64 only** |

Each package carries the engine and all eleven shipped font faces, so there is
nothing to build, no toolchain to install beside it and no network access at
render time. On a host that is not Windows, folio-js is the portable build.

### Render a PDF

A template at `invoice.folio`, its data at `invoice.json`, and the same three
steps in every language: load the template, render it with a font set, write the
bytes out. Warnings arrive beside a successful render rather than instead of it.

**Fonts are always an explicit argument.** There is no default set and no lookup
on the machine that renders — which is half of why the output is reproducible.

#### Go

```go
tpl, err := folio8.LoadTemplate("invoice.folio")
if err != nil {
	log.Fatal(err)
}
data, err := os.ReadFile("invoice.json")
if err != nil {
	log.Fatal(err)
}
res, err := folio8.Render(tpl, folio8.Data(data), nil, fonts.Shipped())
if err != nil {
	log.Fatal(err)
}
for _, d := range res.Diagnostics {
	log.Printf("%s %s: %s", d.Severity, d.Code, d.Message)
}
if err := os.WriteFile("invoice.pdf", res.Bytes, 0o644); err != nil {
	log.Fatal(err)
}
```

Imports are `folio8 "github.com/panitw/folio8/folio-go"` and
`"github.com/panitw/folio8/folio-go/fonts"`.

#### Node

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

#### .NET

```csharp
// folio8's public types sit in the global namespace — no `using` to add.
Template template = Template.Load("invoice.folio");
Data data = new Data(File.ReadAllBytes("invoice.json"));

RenderResult result = Folio8.Render(template, data, null, Fonts.Shipped());

File.WriteAllBytes("invoice.pdf", result.Bytes);
foreach (Diagnostic d in result.Diagnostics)
{
    Console.Error.WriteLine(d.Severity + " " + d.Code + ": " + d.Message);
}
```

`Fonts.Shipped()` hands every caller its own copy of the face set, so call it
once and hold what it returns rather than calling it per render.

The third argument is params in all three, and `nil`/`null` says the template
reads none. Each guide walks through the same ground in full — a complete first
PDF, errors and warnings, template features and the whole API: [folio-go](docs/rendering-library.md),
[folio-js](docs/folio-js.md), [folio-dotnet](docs/folio-dotnet.md). Measured
throughput and memory, and how to size a host, are in the
[performance report](docs/performance.md).

`folio8.RenderTo` writes straight to an `io.Writer` for HTTP handlers and large
documents. Bring your own `FontSet` whenever you want typography other than the
shipped faces. The [folio-go README](folio-go/README.md) explains why `folio8`
and `folio8/fonts` are separate imports — the embedded faces are opt-in, and in
your binary only if you ask for them — and covers `Data` vs `Params`, the
`locale` field, and the known limitations.

### Render from the command line

```
folio8 validate [-data <path>] [-params <path>] [-fonts <dir>] [-strict] <template.folio>
folio8 render   [-data <path>] [-params <path>] [-fonts <dir>] [-o <path>] [-strict] <template.folio>
```

`SOURCE_DATE_EPOCH` supplies the reserved `documentDate` param when no other
route has — it never overwrites one you passed explicitly.

`-fonts` names a directory of `.ttf`/`.otf` faces to render with in addition to
the shipped eleven. Each is keyed by the name its own binary declares, not by
its filename, and **a face there whose name matches a shipped face replaces it**.
A missing or unreadable directory **fails the run**. A file inside it that is not
a readable face is reported on stderr and skipped, as is a directory that yields
no faces at all; neither fails the run on its own.

`-strict` turns warnings (a dropped character, say) into a non-zero exit, and
counts a skipped font or an empty font directory as one — so a typo'd `-fonts`
path fails a build that asked for strict. Exit codes: `0` success, `1` validation
or render failure, `2` usage error.

### Run the designer

```
cd folio-designer
npm ci
npm run dev
```

It opens a `.folio` file from your machine and saves it back — no round-trip and
no account — and works offline after first load. `npm run build` produces the
static, offline-capable release and verifies it.

---

## Templates and data

A `.folio` file is JSON: page setup, three bands (page header, content, page
footer), and components placed at absolute coordinates. Five component types —
Text, Image, Table, Line, Rectangle.

Data binds through double braces, `{{customer.name}}`, and tables bind to a
collection with an explicit row scope. Expressions are deliberately small:
**eight functions, no loops, no variables, no arithmetic operators**. If you want
a ninth, the calculation belongs in the data. See the
[expression reference](docs/expression-reference.md).

Runtime values that are *not* report data — a page-count cap, a document date,
an approval flag — arrive separately as **params**, which is also how folio8
avoids ever reading the clock.

---

## Determinism, and what actually guarantees it

The byte-identity claim is a tested property, not an aspiration:

- **A golden corpus.** Every fixture under [fixtures/](fixtures/) records its
  expected PDF and its SHA-256. A hash that moves is a defect until someone
  proves it was intended.
- **A cross-target matrix.** [`.github/workflows/matrix.yml`](.github/workflows/matrix.yml)
  renders the corpus on four targets and compares bytes. `hashmatrix`'s
  retained float64 multiply-add proves that comparison has teeth.
- **Guardrails that fail the build.** `float64` may not appear under
  `folio-go/internal/` at all — the check parses the source rather than compiling
  it, so build tags don't hide it. Import rules, a map-range determinism check
  and a licence check run beside it, all from the `lint` module.
- **No ambient input.** The render path reads no environment variable. The CLI is
  the one place `SOURCE_DATE_EPOCH` is read, and it passes the value in as an
  ordinary parameter, exactly as any other caller would.

The caveat is honest and stated: reproducibility holds *on a given build
toolchain*. Fonts are committed rather than generated at build time for the same
reason — a different fontTools produces a different font, which produces a
different PDF.

---

## Working on it

Each module builds and tests with plain `go` commands from its own directory —
CI invokes the guardrails, it never re-implements them, and neither does the
Makefile:

```
cd folio-go && go build ./... && go vet ./... && go test ./...
cd lint      && go test ./...
cd hashmatrix && go test ./...
```

```
cd folio-designer
npm run test        # unit and contract tests
npm run typecheck
npm run lint
npm run test:e2e    # Playwright; CI compiles the suite, gates run it
```

Repository-level targets — only what spans modules or drives non-Go tooling:

```
make help
make fonts          # regenerate the shipped faces from FONT_SOURCES
make fonts-verify   # assert the committed faces still reproduce, writing nothing
```

CI ([ci.yml](.github/workflows/ci.yml)) runs build, vet, gofmt, tests and
guardrails per module. One test is **expected red** and is run in its own job to
keep that visible rather than skipped.

---

## Status

The MVP is delivered: Epics 1–6 are closed — deterministic rendering, three
scripts, expressions and aggregates, tables with pagination and repeating
headers, the designer, and data binding round-tripping through the file.

Post-MVP work is in flight: long-form body text and a multi-page authoring canvas
(Epic 7), embeddable fonts (8), component box paint and colour (9–10), the
inspector and designer chrome (12–14), and the release blockers (15).
[sprint-status.yaml](_bmad-output/implementation-artifacts/sprint-status.yaml)
is the current record, including what is deliberately deferred and why.

`folio-go/v1.0.0` is the first release. Its public Go API — rendering, validation, the
font input and the diagnostic types — is frozen under semver: a breaking change needs a
`/v2` import path. See [RELEASING.md](RELEASING.md).

---

## Licence

MIT. See [LICENSE](LICENSE). Third-party dependency licences across all three Go
modules and the designer's lockfile are resolved and recorded in
[lint/MANIFEST.md](lint/MANIFEST.md); an unresolved or forbidden licence fails
the build rather than appearing there silently.
