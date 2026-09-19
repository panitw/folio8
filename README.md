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

---

## Repository layout

| Path | What it is |
| --- | --- |
| [folio-go/](folio-go/) | The rendering engine and reference implementation — expression evaluation, layout, pagination, PDF output. A Go module: `github.com/panitw/folio8/folio-go`. See its [README](folio-go/README.md). |
| [folio-go/cmd/folio8/](folio-go/cmd/folio8/) | The `folio8` CLI: `validate` and `render`, and nothing else. |
| [folio-go/wasm/cmd/engine/](folio-go/wasm/cmd/engine/) | The designer's js/wasm entry point over the internal session engine in `folio-go/internal/wasm` — the same engine, compiled to wasm. Not public API. |
| [folio-designer/](folio-designer/) | The visual designer: React + Vite, running the wasm engine in a worker. No server, no account, no upload. |
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

### Render from Go

```go
tpl, err := folio8.LoadTemplate("statement.folio")
if err != nil {
	log.Fatal(err)
}
res, err := folio8.Render(tpl, folio8.Data(dataJSON), folio8.Params(paramsJSON), fonts.Shipped())
if err != nil {
	log.Fatal(err)
}
for _, d := range res.Diagnostics {
	log.Printf("warning %s: %s", d.Code, d.Message)
}
if err := os.WriteFile("statement.pdf", res.Bytes, 0o644); err != nil {
	log.Fatal(err)
}
```

Install with `go get github.com/panitw/folio8/folio-go@v1.0.0`. The
[rendering library guide](docs/rendering-library.md) walks through installation, a complete first
PDF, errors and warnings, template features and the full API.

`folio8.RenderTo` writes straight to an `io.Writer` for HTTP handlers and large
documents. The font set arrives as an explicit argument — `fonts.Shipped()` gives
you Noto Sans, Noto Sans Thai and Noto Sans SC; bring your own `FontSet` if you
need other typography. The [folio-go README](folio-go/README.md) explains why
`folio8` and `folio8/fonts` are separate imports (~11.3 MB of embedded faces you
opt into), and covers `Data` vs `Params`, the `locale` field, and the known
limitations.

### Render from the command line

```
folio8 validate [-data <path>] [-params <path>] [-strict] <template.folio>
folio8 render   [-data <path>] [-params <path>] [-o <path>] [-strict] <template.folio>
```

`SOURCE_DATE_EPOCH` supplies the reserved `documentDate` param when no other
route has — it never overwrites one you passed explicitly. `-strict` turns
warnings (a dropped character, say) into a non-zero exit. Exit codes: `0`
success, `1` validation or render failure, `2` usage error.

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
