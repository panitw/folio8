---
id: SPEC-folio
companions:
  - ./acceptance.md
  - ./folio-format.md
  - ./glossary.md
  - ../../planning-artifacts/architecture/architecture-folio-2026-08-23/ARCHITECTURE-SPINE.md
  - ../../planning-artifacts/prds/prd-folio-2026-08-22/prd.md
  - ../../planning-artifacts/prds/prd-folio-2026-08-22/addendum.md
  - ../../planning-artifacts/ux-designs/ux-folio-2026-08-23/EXPERIENCE.md
  - ../../planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Folio — Report Designer and Rendering Platform (MVP v0.1)

## Why

A **vision to realize**, built first for its author. Producing a 20–50 page enterprise statement
from JSON today means JasperReports — a server to operate, a database coupling Folio refuses,
and a template format nobody can read in a diff. Folio bets on a different shape: a template
file the user owns and Git can merge, a rendering engine that is an in-process Go library
rather than a service, and a designer that opens like draw.io with no account and no upload.
The bet's load-bearing claim is **byte-identical output** — the PDF previewed in the browser
hash-matches the PDF a Linux service renders — which turns "what you see is what you get" from
an aspiration into an equality a test can assert. Two users of equal standing sit on either
side of the `.folio` file: the template author who lays out the statement, and the Go developer
who embeds the library. That file is the contract between them, and every trade-off below
resolves toward making that handoff reliable rather than toward feature breadth.

## Capabilities

- **CAP-1 — Visual report authoring**
  - **intent:** A template author lays out a report on a visual canvas — page setup, three
    bands, components placed and resized at absolute coordinates, properties edited — so the
    document takes its shape without writing code.
  - **success:** The golden report's non-table content (logo, customer block, account block,
    statement period, footer text) is produced entirely through the canvas, and which band each
    component sits in is unambiguous to the author before release of a drag.

- **CAP-2 — Data binding from sample data**
  - **intent:** The author loads a sample JSON document and binds components to paths
    discovered in it, so they can see the shape of the data the developer will supply.
  - **success:** Every scalar field in the golden report is bound by selecting a path from the
    loaded sample, with no path typed by hand.

- **CAP-3 — Table structure authoring**
  - **intent:** The author configures a table's columns — add, remove, width, alignment, header
    label, row-scoped field binding, footer aggregate — as a matrix of columns against
    attributes.
  - **success:** The golden report's five-column transaction table, including its repeated
    header and its sum footer, is built entirely in the designer.

- **CAP-4 — Local file lifecycle**
  - **intent:** The author opens and saves `.folio` files on their own machine, with no account
    and no server-side storage.
  - **success:** A round trip — open, edit, save — works in a browser without the File System
    Access API as well as one with it, and no template or data byte leaves the machine.

- **CAP-5 — Portable template format**
  - **intent:** A report definition persists as a single human-readable text file that is
    diffable and mergeable in Git, usable in CI/CD and over an API, and editable by a person or
    an AI agent without opening the designer.
  - **success:** A template hand-written in a text editor and never opened in the designer
    loads and renders (S9); a template saved by the designer and one saved after a no-op
    round trip are byte-identical; a template declaring a future format version fails with a
    located error rather than rendering.

- **CAP-6 — Data and parameter binding model**
  - **intent:** The engine resolves scalar paths, collection bindings, row scope inside
    repeating regions, and a runtime parameter namespace distinct from report data.
  - **success:** `{{customer.name}}`, `transactions[]`, `{{transaction.amount}}` inside the
    table, and `{{params.reportDate}}` all resolve correctly in the golden report, and an
    absent path fails with an error naming the path.

- **CAP-7 — Expression evaluation**
  - **intent:** A deliberately small expression language evaluates aggregation, formatting,
    string, and logic functions inside bindings, and controls component visibility
    conditionally.
  - **success:** All eight functions have tests; `{{sum(transactions.amount)}}` matches a
    hand-computed total; `formatDate` renders a Buddhist-era year under the `th` locale; a
    conditionally hidden component is absent from the output and displaces nothing.

- **CAP-8 — Table rendering and pagination**
  - **intent:** A table generates rows from its bound collection, breaks across pages without
    splitting a row, repeats its header on every continuation page, and renders footer
    aggregates without orphaning them.
  - **success:** The golden report paginates correctly at 1, 5, 20, and 50 pages against
    recorded reference renders; a row taller than the content area renders at the top of a
    fresh page, clipped, with a diagnostic rather than a loop or a silent truncation.

- **CAP-9 — PDF document assembly**
  - **intent:** The engine produces a multi-page PDF, with page headers and footers on every
    page, `Page X of Y` numbering, and a document that carries everything it needs to render
    anywhere — its fonts and its images included.
  - **success:** The golden report opens correctly in a machine with none of its fonts
    installed; `Page X of Y` is correct on every page at all four page counts; no render
    performs a network request or a filesystem read.

- **CAP-10 — Exact, offline, in-browser preview**
  - **intent:** The author sees the real production document before anyone else does, produced
    on their own machine with nothing sent anywhere, and can tell it apart from the approximate
    design canvas without a tutorial.
  - **success:** The previewed PDF hash-matches the PDF the native library renders from the
    same inputs (S2); the designer loads and previews with the network disconnected after
    first load; a preview whose inputs have changed is never presented as current.

- **CAP-11 — Go library integration surface**
  - **intent:** A Go developer loads a template, supplies data and parameters, and writes PDF
    bytes to a writer in-process, through an API with no ceremony.
  - **success:** A new integrator produces their first PDF in minutes from the README alone
    (C5); a template renders straight to an HTTP response with no temporary file.

- **CAP-12 — Error and diagnostic contract**
  - **intent:** Failures name the template element and the data path that caused them, and
    conditions that degrade output without preventing it are reported without failing the
    render. A template can be validated independently of rendering.
  - **success:** Every failure mode — malformed template, unresolvable binding, invalid
    expression, missing glyph, unlayoutable content — has a stable code and a test asserting
    its message locates the problem (S8); a malformed `.folio` is rejected in CI without
    rendering it.

- **CAP-13 — Byte-reproducible rendering**
  - **intent:** The same template, data, parameters, and library version produce a
    byte-identical PDF on every machine and every compilation target, so a regression test is a
    hash compare.
  - **success:** Identical hashes across `darwin/arm64`, `linux/amd64`, `linux/arm64`, and
    `js/wasm` on the golden report at all four page counts (S2, S3), with the reference render
    confirmed non-blank and page-count-correct.

- **CAP-14 — Latin, CJK, and Thai text**
  - **intent:** Text in all three scripts is shaped, measured, broken, and rendered correctly —
    including Thai, whose marks must sit on their base characters and whose words are not
    delimited by spaces.
  - **success:** Thai line breaks match a hand-checked expected-break fixture exactly; CJK
    breaks per character; Thai vowels and tone marks are positioned by GPOS (S4). All three
    scripts appear in the same table in the golden report.

## Constraints

- **Byte-identity is the product.** No determinism exception is ever carved out to ship a
  feature. Any change to layout, subsetting, emission, or the locale table is a breaking change
  for downstream test suites and is released as one.
- **All layout arithmetic is integer fixed-point.** `float64` appears nowhere in the engine —
  not for geometry, and not for report data, which is parsed as exact decimals. Go permits FMA
  contraction and offers no flag to disable it; arm64 fuses and wasm does not, so a float
  approach diverges invisibly and shows up only in the hash.
- **The engine's numeric surface is restricted.** Only correctly-rounded and exact operations;
  `math` transcendentals are banned and the ban is enforced by lint, not convention.
- **Rendering is a pure function** of template, data, and parameters — no clock, time zone,
  locale, environment variable, filesystem, or network on the render path. Any date in output
  arrives through parameters.
- **The browser is never the layout authority**, including on the design canvas. Every text
  metric and line break comes from the engine.
- **The layout model is banded**: absolute placement within Page Header, Content, and Page
  Footer; the table is the only flowing construct; content exceeding its bounds is clipped with
  a diagnostic and is never reflowed and never silently dropped.
- **Correct pagination beats table styling** wherever the two compete.
- **No cgo**, because the engine must compile to WebAssembly.
- **No PDF encryption** — ISO 32000-1 §7.6.2 mandates a random IV per encrypted string and
  stream, which is mutually exclusive with byte-identity.
- **The MVP component palette is exactly five** (Text, Image, Table, Line, Rectangle) and the
  **expression language is exactly eight functions**. Growth in either without a spec that
  moves it into scope is a defined failure signal, not a feature. `spec-barcode-qr-elements`
  adds Barcode and QR Code post-MVP.
- **Images are embedded in the template**; Folio never fetches an image by URL or reads one
  from disk at render time.
- **All fonts are embedded and subsetted**, and subsetting is byte-stable — no wall-clock
  timestamp in subset output, and subset tags derived by hash.
- **No server, no accounts, no server-side template storage.** Templates and data never leave
  the user's machine during design.
- **A ~10.82 MiB first load is accepted** in exchange for a designer that is genuinely offline
  with no font-fetch failure mode. That load is the *core tier* only — the engine, the base
  Latin and Thai faces, the shell and pdf.js. The CJK font and the font catalogue are fetched
  on first use and kept, so "genuinely offline" is a claim about what this browser has actually
  fetched, not about the whole release. See `spec-deferred-offline-cache`, which supersedes the
  earlier "~9 MB" figure this constraint carried.
- **Reliability over feature breadth**, enforced against a solo delivery capacity.
- **Folio ships under MIT, and nothing copyleft enters.** No dependency may carry GPL, LGPL,
  AGPL, SSPL, or a commercial EULA at any depth — Go links statically, so such a dependency
  would attach its obligations to the whole binary. Enforced by a CI licence check over the
  module graph and the designer lockfile.
- **Thai line breaking fails toward not breaking.** The engine's contract is break
  *opportunities*, not word segmentation. A run no dictionary covers is atomic, and no break
  falls inside a Thai character cluster — a customer's name overflows visibly rather than
  being silently split in the wrong place.

## Non-goals

- **JasperReports compatibility.** No `.jrxml` import, no migration path. Folio competes on the
  workflow, not on import fidelity.
- **Any output format other than PDF.** HTML, CSV, PNG, SVG, PowerPoint, and Word are excluded
  because different output formats carry different layout semantics. The layout model stays
  separable so PNG, SVG, and HTML become possible later — **Excel is explicitly not protected
  this way.**
- **Right-to-left scripts.**
- **Database connections, SQL query design, scheduled reports, email delivery, report
  bursting.** The calling application prepares the data as JSON; Folio never touches a database.
- **Accounts, RBAC, multi-tenancy, a server-side report repository.** There is no operator or
  administrator role in MVP.
- **Pivot tables, charts, subreports, digital signatures, general-purpose
  scripting, advanced conditional formatting** (conditional *visibility* is in scope;
  data-driven *styling* is not), **advanced font management UI, PDF encryption.**
- **Accessibility conformance** — neither designer UI conformance nor tagged PDF (PDF/UA,
  PDF/A). A behavioural usability floor still binds; see `EXPERIENCE.md`.
- **Group headers, report headers, and group footers.** Three bands only.
- **The optional REST rendering service, and SDKs for Java, .NET, and Node** — deferred, not
  rejected. The REST service is the first thing that would give Folio an operated runtime and
  a threat model, neither of which MVP has.
- **A translated designer UI and a general locale model.** Rendering Thai and CJK text is in
  scope; localizing the product is not.

## Success signal

A 34-page customer account statement — Thai, CJK, and Latin names in one transaction table,
an embedded logo, `Page X of Y`, a sum footer — is laid out in the browser by someone who has
never used a report designer, saved to their laptop, and committed to Git. A Go developer pulls
that file, adds three lines to an HTTP handler, and the PDF their service returns has the same
SHA-256 as the one the author saw in their browser tab. The regression test for it is one line:
compare the hash.

## Assumptions

- Both user journeys in the PRD are inferred from the source plan, not captured from a real
  session with a template author or an integrating developer.
- Sample JSON is a separate local file alongside the template, not embedded in the `.folio`.
- Byte-identity is proven in Folio's own code — the Epic 4 boundary matrix renders fifteen
  registered documents on darwin/arm64, linux/amd64, linux/arm64 and js/wasm and compares the
  resulting SHA-256 digests.
- No memory ceiling or throughput target exists; a 50-page statement rendering without
  pathological memory growth is the working target.
- Success is acceptance-based rather than adoption-based, given the current build-for-self
  distribution intent.
- Folio emits its own PDF rather than depending on a third-party writer — a consequence of
  shaping text in-house, but the single most expensive call in the spine to reverse.
- Report data numbers are exact decimals; the four supported locales (`en`, `th`, `zh-Hans`,
  `ja`) are inferred from the required script set rather than stated by any input.
- React, Vite, and a DOM/SVG canvas are the conservative designer stack, chosen because the
  engine holds document state — not from a stated preference.

## Open Questions

- Which specific font faces ship, and under what licences? Named faces are all OFL 1.1, but the
  final set and a user's path to supplying a custom font are unsettled (PRD Q2).
- Does the build order stand, given tables depend on aggregation from the expression engine
  scheduled after them (PRD Q5)?
- Does the designer really come last, when it carries success measure S5 and serves half the
  user base (PRD Q6)?
- Where does sample JSON live relative to the template during design, and does its path persist
  in the `.folio` (PRD Q8, UX2)?
- What are the forward and backward compatibility rules between template versions and library
  versions (NFR6)?
- What is the screen-reader model for the design canvas (UX1)?
- How does the author discover which glyphs a font lacks before previewing (UX3)?
- Is there template-level undo across a save boundary, or does undo history die on reload
  (UX4)?
