---
title: Folio — Product Requirements Document (MVP v0.1)
status: final
created: 2026-08-22
updated: 2026-08-23
---

# Folio — PRD (MVP v0.1)

**Source input:** `docs/folio-mvp-plan.md`
**Technical depth:** `addendum.md` — handoff to `bmad-architecture`
**Reviews:** `review-rubric.md` · `review-feasibility.md` · `review-adversarial.md` · `reconcile-folio-mvp-plan.md`

---

## 1. Product Goal

Folio is a report designer and rendering platform: a visual designer that produces a
portable template file, and a Go library that turns that template plus JSON data into a
PDF.

The MVP exists to prove one workflow end to end:

> **Design → Bind → Preview → Render**

**Governing principle:** the first release optimizes for *reliability over feature
breadth*. Folio v0.1 succeeds if it can reliably generate a professional 20–50 page
enterprise statement from JSON, using a `.folio` template and `folio-go`.

---

## 2. Problem and Positioning

Folio is positioned as an alternative to JasperReports — not a replacement for it.
`.jrxml` compatibility is out of scope, and Folio does not attempt to reproduce Jasper
feature-for-feature. No migration path from JasperReports is offered in MVP; Folio competes
on the workflow, not on import fidelity.

The bet is on a different set of properties:

| Property | Folio's position |
|---|---|
| Data source | JSON-first. Folio never touches a database; the calling application prepares the data. |
| Template artifact | A plain text file the user owns — human-readable, diffable, versionable in Git, **CI/CD friendly, API friendly, and editable by an AI agent** without opening the designer. |
| Rendering | Deterministic and byte-reproducible, with the browser explicitly *not* the canonical renderer. |
| Integration | An in-process Go library, not a server you have to operate. |
| Design tool | A web app in the draw.io mould — open it, work on local files, no account. |

---

## 3. Users and the Handoff

Folio has **two primary users of equal standing**, and the relationship between them is the
product.

**The template author** lays out an invoice or statement visually and binds its fields to
paths in a JSON document. They may or may not write code. They care about the canvas,
components, properties, and seeing the real output before anyone else does.

**The integrating Go developer** embeds `folio-go` in a service and calls it to produce
PDFs on demand. They care about API ergonomics, determinism, and predictable behaviour
under load.

The `.folio` file is the **contract between them**. Every requirement here should be read
against the question: *does this make the handoff more reliable?*

Two roles are deliberately absent from MVP: there is no operator or administrator (no
accounts, no RBAC, no multi-tenancy), and the end recipient of the PDF is not a Folio user
— they receive a document, not an experience.

> **Tension resolved.** The source plan placed the designer at Phase 5 of 6, in real tension
> with the co-primary claim above. `bmad-create-epics-and-stories` settled it: engine-first
> stands and C4 holds — no designer work begins until the engine renders the golden report —
> but the designer is two epics rather than one trailing phase, and the exact preview ships
> **with** the first of them rather than after it. Building a canvas that cannot be previewed
> would mean authoring blind, and would leave NFR2 and NFR5 unvalidated until the final phase.

### 3.1 User Journeys

`[ASSUMPTION]` These journeys are inferred from the source plan, not captured from a real
session. Correct the protagonists and beats where they ring false.

**UJ-1 — Ploy lays out the statement.**
Ploy handles customer communications at a mid-sized bank. She opens the Folio web app and
opens `customer-statement.folio` from her laptop. She sets the page to A4 portrait, drops
the bank logo into the page header, and places text fields for the customer's name and
account number — binding each by picking a path out of a sample JSON file she loaded
alongside it. She adds a table to the content region, binds it to `transactions[]`, sets
five columns with fixed widths, and turns on a repeated header row and a sum footer. She
clicks PDF Preview and sees the real 34-page document, Thai customer names wrapping at
correct word boundaries. She saves the file back to her laptop and commits it.

**UJ-2 — Anan wires it into the service.**
Anan maintains the statements microservice. He pulls `customer-statement.folio` from the
repo, adds `folio-go` to his module, and in his HTTP handler loads the template, assembles
the customer's data as JSON, and writes the PDF to the response. He adds one regression
test: render the golden fixture, compare the SHA-256 against the recorded hash. It matches
the file Ploy saw in her browser. He ships it.

---

## 4. Departures from the Source Plan

These decisions **override** `docs/folio-mvp-plan.md`. They are listed rather than quietly
absorbed, so a reader of both documents can see where they diverge and why.

| # | Source plan says | This PRD says | Why |
|---|---|---|---|
| D1 | §10: "the same layout in every environment" | **Byte-identical output** (NFR1) | Makes regression testing a hash compare, and matches "reliability over feature breadth". Verified achievable — see NFR1. |
| D2 | §14: "Internationalization" out of scope | **Latin + CJK + Thai required** (NFR3); only right-to-left deferred | Thai is the real use case. The plan's own §7 example uses a Bangkok branch parameter while excluding i18n — an unresolved contradiction in the source. |
| D3 | §11: a "Preview API" between designer and folio-go | **folio-go compiled to WebAssembly, in the browser** (FR35) | Removes the server the plan never scoped, and keeps the draw.io model coherent. |
| D4 | §13: "Streaming rendering API — Important" | **Two-pass layout; streaming is an API shape, not a memory guarantee** (FR39, NFR4) | `Page X of Y` requires the total page count before page one can be emitted. The two cannot both hold. |
| D5 | Designer delivery unstated (§13 says only "web") | **draw.io model — local files, no account, no server storage** | Resolves the plan's own conflict between a REST service addressing templates by name and "report repository" being out of scope. |
| D6 | §4: `panic(err)` in every example | **A specified error contract** (FR41, FR42) | A library with no error contract is not integrable. |
| D7 | §1 lists "Visibility"; §14 defers "advanced conditional formatting" | **Conditional visibility in, conditional formatting out** (FR20) | The source leaves this boundary undrawn. |

---

## 5. Scope

### 5.1 In scope for MVP

The four core deliverables: **Folio Designer**, the **Folio Template Format** (`.folio`),
the **folio-go rendering library**, and **PDF output**.

### 5.2 Deferred to a later release — *not* rejected

Roadmap items the source plan marked "Later" — excluded from MVP, still intended:

Java SDK · .NET SDK · Node.js SDK · Excel export · charts · subreports

### 5.3 Out of scope — not planned

Pivot tables · SQL query designer · direct database connections ·
scheduled reports · email delivery · role-based access control · multi-tenancy ·
server-side report repository · general-purpose scripting · advanced conditional
formatting · right-to-left text · advanced font management UI · digital signatures ·
report bursting · JasperReports `.jrxml` compatibility · **PDF encryption** (see NFR1.g).

Barcode (Code 128) and QR code came off this list on 2026-09-13 and are specified in
[`spec-barcode-qr-elements`](../../../specs/spec-barcode-qr-elements/SPEC.md).

Non-PDF output formats — HTML, CSV, PNG, SVG, PowerPoint, Word — are excluded from MVP
because different output formats carry different layout semantics.

### 5.4 Deferred but architecturally protected

The layout model must remain separable from the renderer so PNG, SVG, and HTML renderers
become possible later without a rewrite. **Excel is explicitly not protected this way** —
it has different layout semantics and must not be forced into the same model.

---

## 6. Layout Model

*This section resolves the single largest gap in the first draft: whether components sit at
fixed coordinates or participate in a flow. Every other requirement depends on the answer.*

Folio uses a **banded layout with flow confined to tables**:

- A report is composed of three **bands** — Page Header, Content, Page Footer (FR6).
- Within a band, components are placed at **absolute coordinates** (FR1, FR5). They do not
  reflow, and they do not push each other down.
- A **table is the only flowing construct** (FR22–FR28). It consumes as many rows as its
  bound collection provides, breaks across pages, and repeats its header.
- The Content band grows by pagination — that is, by producing more pages — never by
  displacing sibling components within a page.

**Consequence:** absolutely-positioned content that grows beyond its
declared bounds is **clipped or overflows**, not reflowed. FR44 defines which.

This is the banded model used by traditional report tools, and it is the smallest engine
that renders the golden report in §9.

---

## 7. Functional Requirements

### 7.1 Folio Designer

The designer is a **web application following the draw.io model**: the user opens it in a
browser, works on `.folio` files stored on their own machine, and never creates an account.
No template is stored server-side.

- **FR1** — Lay out a report on a visual canvas with visible page boundaries, placing
  components at absolute coordinates by drag-and-drop, and resizing them directly.
- **FR2** — Configure page setup: A4, Letter, or a custom page size; portrait or landscape
  orientation; and page margins.
- **FR3** — Align work using a grid with snapping.
- **FR4** — Place components from an MVP palette of exactly five: **Text, Image, Table,
  Line, Rectangle**. Post-MVP, **Barcode** and **QR Code** extend the palette to seven
  (`spec-barcode-qr-elements`).
- **FR5** — Edit component properties: position (X/Y), size (width/height), font family,
  font size, bold, italic, text alignment, vertical alignment, border, padding, background,
  visibility, and data binding.
- **FR6** — Compose the report from three bands: **Page Header**, **Content**, and **Page
  Footer**. Group headers, report headers, and group footers are deferred.
- **FR7** — Bind a component to a JSON path through the designer UI, choosing from paths
  discovered in a loaded sample JSON document.
- **FR8** — Open a `.folio` file from the user's local machine and save it back, with no
  server round-trip and no account.
- **FR9** — Load a sample JSON data document alongside the template, used for binding
  discovery and preview. `[ASSUMPTION]` The sample data is a separate local file, not stored
  inside the `.folio`.
- **FR10** — Edit **table structure** in the designer: add and remove columns, set each
  column's width, alignment, and header label, bind each column to a field of the row scope,
  and configure per-column footer aggregates. *Without this, the designer cannot produce the
  golden report — the table is where the layout work actually is.*

### 7.2 Template Format

- **FR11** — Persist a report definition as a portable, human-readable text file with the
  extension `.folio`, carrying a format `version`, page setup, and ordered band content.
- **FR12** — Remain readable, diffable, and mergeable in Git; usable in CI/CD pipelines and
  over an API; and editable directly by a human or an AI agent without the designer.
- **FR13** — Carry a format version that the library validates on load, so a template
  authored against a future format fails clearly rather than rendering incorrectly.

### 7.3 Data Binding and Expressions

- **FR14** — Accept report data as JSON supplied by the calling application. Folio never
  connects to a database.
- **FR15** — Bind a scalar field by dotted path, e.g. `{{customer.name}}`.
- **FR16** — Bind a repeating region to a collection, e.g. `transactions[]`.
- **FR17** — Reference the current row's fields inside a repeating region through a defined
  **row scope**. `[ASSUMPTION]` The source plan writes `{{transaction.amount}}` (singular)
  without ever naming the scoping rule. The rule must be specified before implementation —
  see Q3.
- **FR18** — Evaluate a small expression language inside `{{ }}` bindings, offering exactly
  **eight** functions for MVP:
  - aggregation — `sum()`, `count()`, `avg()`
  - formatting — `formatDate()`, `formatNumber()`, each taking a pattern argument
  - string — `upper()`, `lower()`
  - logic — `if()`
- **FR19** — Aggregate over a collection field, e.g. `{{sum(transactions.amount)}}`.
- **FR20** — Control component visibility conditionally. Conditional *visibility* is in
  scope; conditional *formatting* — styling driven by data — is out.

The expression language is deliberately small. It is not, and must not grow into, a
general-purpose scripting language in MVP.

### 7.4 Parameters

- **FR21** — Accept runtime parameters separately from report data, addressed through a
  distinct namespace: `{{params.reportDate}}`, `{{params.branchName}}`.

### 7.5 Tables and Repeating Sections

Table rendering is the highest-risk area of the MVP. **Correct pagination takes priority
over table styling** wherever the two compete.

- **FR22** — Generate table rows dynamically from a bound data collection.
- **FR23** — Lay out columns at fixed widths with per-column alignment.
- **FR24** — Render a header row, cell borders, and cell padding.
- **FR25** — Break a table across pages according to these rules:
  - a row is **atomic** — it is never split across a page boundary;
  - if a row does not fit in the remaining space, it moves whole to the next page;
  - **if a single row is taller than a full content area**, it is rendered at the top of a
    fresh page and clipped to that page's content height, and the render reports a
    diagnostic (FR41) rather than looping or silently truncating;
  - a footer aggregate row is never orphaned — if it does not fit, it moves to the next page
    together with at least one preceding data row;
  - cell content wraps within its column width; column widths are fixed and never
    negotiated against content.
- **FR26** — Repeat the table header at the top of every continuation page.
- **FR27** — Render footer aggregates: sum, count, and average.
- **FR28** — *(if capacity permits)* Alternating row styling. Capacity is judged at the point
  the table work is otherwise complete; FR28 is cut before any of FR22–FR27 is compromised,
  per the rule that correct pagination beats table styling.

### 7.6 Rendering and Output

- **FR29** — Produce PDF as the only required output format for MVP.
- **FR30** — Render multi-page documents with page headers and page footers on every page.
- **FR31** — Render page numbers, including the `Page X of Y` form, resolved by a two-pass
  layout (see FR39 and NFR4).
- **FR32** — Embed and subset all fonts used in the document, so the PDF renders identically
  on a machine that has none of them installed, and so font subsetting is byte-reproducible
  (NFR1.e).
- **FR33** — Embed images inside the `.folio` template itself. Folio does **not** fetch
  images by URL and does **not** read them from the filesystem at render time; both would
  break determinism and introduce a mid-render failure mode.

### 7.7 Preview

- **FR34** — Offer a fast **Design Canvas** view for interactive editing, understood to be
  an approximate visual representation.
- **FR35** — Offer an exact **PDF Preview** produced by the real `folio-go` engine compiled
  to WebAssembly and running in the browser tab — not by a browser-based approximation, and
  not by a remote service. The previewed document is the production document, byte for byte
  (NFR1).
- **FR36** — Operate the designer and its exact preview fully offline, with neither template
  nor data leaving the user's machine.

### 7.8 folio-go Rendering Library

`folio-go` is a **core MVP deliverable and the reference implementation** of the rendering
engine — not a later SDK. It is the same engine the designer previews with.

- **FR37** — Load a template from a file.
- **FR38** — Render a template plus data to PDF bytes in-process, through an API that is
  "extremely simple" — a load call and a render call, nothing ceremonial.
- **FR39** — Render to an `io.Writer`, so a caller can write a PDF directly to an HTTP
  response without a temporary file. **This is an API shape, not a constant-memory
  guarantee** — see NFR4.
- **FR40** — Accept runtime parameters at render time alongside the data.
- **FR41** — Fail with a located, actionable error — naming the template element and the
  data path — for malformed templates, unresolvable bindings, invalid expressions, missing
  glyphs, and content that cannot be laid out. Diagnostics that do not prevent a render
  (such as FR25's over-tall row) are reported without failing it.
- **FR42** — Validate a template independently of rendering, so a malformed `.folio` file
  can be rejected in CI before it reaches production.
- **FR43** — Render as a **pure function** of (template, data, parameters). The rendering
  path reads no wall clock, no time zone, no locale, no environment variable, no filesystem,
  and no network. Any date or time appearing in output arrives through `params` (FR21).
- **FR44** — Define overflow behaviour for absolutely-positioned content that exceeds its
  declared bounds: content is **clipped at the component boundary** and a diagnostic is
  reported (FR41). Content is never reflowed and never silently dropped.

### 7.9 REST Rendering Service *(Optional)*

- **FR45** — *(optional, judged only once the golden report renders reproducibly — never before)* Expose an HTTP endpoint accepting
  parameters and data and responding with `application/pdf`. The template travels in the
  request or is resolved from a path supplied by the operator — **not** from a server-side
  report repository, which is out of scope. The Go library remains the higher-priority
  runtime integration.

---

## 8. Non-Functional Requirements

### NFR1 — Byte-reproducible rendering *(the signature requirement)*

The same template, the same data, the same parameters, and the same `folio-go` version
**must produce a byte-identical PDF** — the same file hash — regardless of operating system,
machine, or compilation target.

**The consequential form:** this holds *across compilation targets*. The PDF previewed by
the WebAssembly build in the browser must hash-match the PDF rendered by the native build on
a Linux server. That turns "what you see is what you get" from a design aspiration into a
testable equality.

**This is spec-conforming.** ISO 32000-1:2008 §14.4 makes `/ID` a *should*, not a *shall*,
and attaches an explicit note: *"The calculation of the file identifier need not be
reproducible."* `/CreationDate` and `/ModDate` are optional. Exactly one `shall`-level
randomness requirement exists in the specification, and it lives inside public-key
encryption — which is out of scope (NFR1.g).

The requirement decomposes into seven constraints. **The first is the one that actually
threatens the product**; the ones the first draft emphasized turned out to be cheap.

- **NFR1.a — Layout arithmetic is contraction-free.** All layout and measurement arithmetic
  must produce bit-identical results on every supported target. Go permits FMA contraction
  and offers no flag to disable it: wasm forbids fusion by specification, arm64 fuses. The
  same expression therefore yields different results on the two targets, with **no visible
  difference in output** — the divergence appears only in the hash.
  **The engine must use integer fixed-point arithmetic** for all position, advance, and
  dimension math, so the guarantee is structural rather than a rule every contributor must
  remember.
- **NFR1.b — Numeric emission is quantized.** Every number written into the PDF passes
  through one normative formatter at fixed decimal precision. Free-form float formatting
  never reaches the output stream.
- **NFR1.c — Restricted numeric surface.** The rendering path may use only operations IEEE
  754 requires to be correctly rounded, plus exact ones. `math.Sin`, `Cos`, `Tan`, `Log`,
  `Exp`, `Pow` and other transcendentals are **prohibited** — Go's implementations are not
  bit-identical across architectures. Enforced by an import-restriction lint, not by
  convention.
- **NFR1.d — No unordered iteration reaches output.** Every collection emitted into the PDF
  — objects, resources, glyph sets, character maps — is emitted in a deterministic sorted
  order.
- **NFR1.e — Font subsetting is byte-stable.** Subset output must not embed a wall-clock
  timestamp, and subset tags must be a deterministic hash of the glyph set. ISO 32000-1
  §9.6.4 requires only that the six letters be unique within a file, so a hash is conforming.
- **NFR1.f — Document metadata is pinned.** Creation and modification dates and the document
  ID are either omitted or derived from content. The **library core reads no environment
  variable** — FR43 is absolute. The `SOURCE_DATE_EPOCH` convention is honoured instead by
  callers and by Folio's own command-line tooling, which read it and pass the date in as a
  parameter (FR21). This resolves what was an internal contradiction with FR43.
- **NFR1.g — No encryption in MVP.** ISO 32000-1 §7.6.2 mandates a random initialization
  vector per encrypted string and stream. Encryption and byte-reproducibility are mutually
  exclusive; encryption is therefore out of scope (§5.3).

**Verification requirement:** the CI matrix must include **an arm64 target alongside amd64
and wasm**. FMA contraction is an architecture property, not an operating-system one: arm64
contracts, amd64 and wasm do not. A matrix covering only amd64 and wasm therefore passes
while every arm64 user receives different bytes.

### NFR2 — The browser is not the canonical renderer

Layout authority belongs to `folio-go` alone. The browser's own text and layout engines must
never determine pagination, line breaking, or measurement — including in the design canvas.
`folio-go` compiled to WebAssembly *is* `folio-go`, and satisfies this; DOM-based rendering
does not.

### NFR3 — Script and text support

MVP must correctly shape, measure, break, and render **Latin, CJK, and Thai**:

- **Shaping** — full OpenType `GSUB`/`GPOS` including mark-to-base positioning, without cgo.
  Thai vowel and tone-mark placement *is* GPOS mark positioning; a metrics-only font layer
  cannot render Thai correctly.
- **Line breaking** — whitespace-delimited for Latin; per-character for CJK; **dictionary-based
  for Thai**, which does not delimit words with spaces.
- Right-to-left scripts are out of scope for MVP.

**Thai line breaking must be built.** Unicode UAX #14 assigns Thai to a complex-context class
and defers to a dictionary, so every general segmenter yields effectively no break
opportunities within Thai text. The dictionary must be embedded in the binary and versioned
with the library — never read from disk (fatal under WebAssembly) and never taken from the
host platform (breaks NFR1). See Risk R2.

**The deliverable is break opportunities, not word segmentation.** The distinction is a
requirement rather than an implementation detail: a segmentation error is a wrong linguistic
analysis, whereas a break error is an awkward line, and Folio resolves the difference by
offering *fewer* opportunities rather than wrong ones. A run of Thai the dictionary cannot
cover is treated as atomic, and no break ever falls inside a Thai character cluster — so an
unrecognised customer name overflows visibly under FR44 rather than being silently split in
the wrong place.

### NFR4 — Memory and streaming

`Page X of Y` (FR31) requires the total page count before the first page can be emitted, so
the pipeline is **two-pass**: lay out fully, then serialize. FR39's writer-based API is
therefore an ergonomic and API-stability choice, **not** a constant-memory guarantee.
The API shape is preserved so genuinely incremental rendering can arrive post-MVP without a
breaking change.

`[ASSUMPTION]` No memory ceiling or throughput target is specified. The working target is a
50-page statement rendering without pathological memory growth.

### NFR5 — Fidelity between design and production

The exact preview and the production render are the same engine and produce the same bytes
(NFR1). There is no separate preview renderer that could drift.

### NFR6 — Versioning

Templates carry a format version; the library ships as `folio-go v0.1`. `[ASSUMPTION]`
Forward and backward compatibility rules between template versions and library versions are
undefined and need a policy before v1.

Because golden-hash regression tests are the primary verification mechanism (§9), **any
change to layout, subsetting, emission, or the pinned toolchain is a breaking change for
downstream users' test suites** and must be treated as such in the versioning policy.

Now that Folio ships publicly (Q7), this is no longer an internal note: third parties will pin
these hashes, so the compatibility policy becomes a commitment rather than a convention. It
remains undefined, and it is the one open item that grew more expensive when the licence was
decided.

### NFR7 — Font provisioning

All fonts are embedded and subsetted (FR32), and subsetting is byte-stable (NFR1.e).
Covering Latin + CJK + Thai requires a shipped font set with adequate coverage, embedded so
the designer works fully offline (FR36).

**Measured payload budget:** the engine and font stack compile to roughly 1.5 MB compressed;
a full CJK face dominates at roughly 7.5 MB compressed, and a Thai line-breaking dictionary
adds around 0.1 MB. Outline format matters more than any other choice — glyf/TrueType
variable fonts compress to roughly 44% of their size while CFF/OpenType compresses only to
roughly 70%, making the glyf build several megabytes smaller over the wire despite being
larger on disk.

**Accepted:** a first load of roughly 9 MB, in exchange for a designer that is genuinely
offline with no font-fetch failure mode.

`[ASSUMPTION]` Which specific faces ship, and the licensing of each embedded face, remain
unresolved — see Q2.

### NFR8 — Privacy posture

The draw.io model plus WebAssembly preview means templates and data never leave the user's
machine during design. This is a property to state and protect, not an accident of the
architecture.

**AMENDED 2026-09-21 by OWNER DECISION (D-GA.1), recorded at `ARCHITECTURE-SPINE.md` AD-27.** **What still holds — and it is the
whole of NFR8's substance:** templates, sample data, runtime parameters and rendered PDFs
**never leave the user's machine**, during design or at any other time. Rendering and preview
remain wholly local. **What changed:** a deployed build may load **Google Tag Manager** and
report page loads plus four fixed action names — `open_template`, `export_pdf`, `font_import`,
`enter_preview` — and nothing else. The tag is gated on a build-time env var, so an
unconfigured build (development, Vitest, Playwright) transmits nothing at all. The enforcement
is structural, not a promise: the event parameter is a closed TypeScript union, so no file
name, template name, font family or document value can reach it from any call site.

---

## 9. Success Criteria

Because Folio is built to scratch its author's own itch, success is defined by
**capability and reliability, not adoption**.

### 9.1 The acceptance report

The primary acceptance test is a **Customer Account Statement** carrying a logo, customer
information, account information, a statement period, a transaction table (Date,
Description, Debit, Credit, Balance), and a footer with confidentiality text, a generated
date, and `Page X of Y`.

**The generated date is supplied through `params` (FR21), never read from the clock.** A
fixture that stamps wall-clock time cannot satisfy a hash-equality test; per FR43 the engine
cannot read a clock at all.

The fixture must contain **Latin, Thai, and CJK text in the same table**, at least one row
whose content wraps to multiple lines, and at least one embedded image.

It must render correctly at **1, 5, 20, and 50 pages**.

### 9.2 Success measures

Each measure names what would otherwise let it pass vacuously.

| # | Measure | Target |
|---|---|---|
| S1 | Golden report renders correctly at 1, 5, 20, 50 pages — verified against recorded reference renders, not merely "produces a file" | 4/4 |
| S2 | Native-vs-WASM hash equality on the golden report at all four page counts, with the reference render confirmed non-blank and page-count-correct | Identical |
| S3 | Cross-platform hash equality across `darwin/arm64`, `linux/amd64`, **and `linux/arm64`** | Identical |
| S4 | Thai line breaks match a hand-checked expected-break fixture; CJK breaks per-character; Thai marks positioned by GPOS | Fixture matches exactly |
| S5 | Round trip — author in the browser, save locally, render via `folio-go` — where the template was *authored in this session*, not merely re-saved | Byte-identical to the preview |
| S6 | A 50-page render completes within a recorded memory ceiling, established as a baseline and enforced against regression | No regression |
| S7 | A second, structurally different template (not the golden report) renders correctly and reproducibly | Pass |
| S8 | Every error case in FR41 produces a located, actionable message | All cases covered |
| S9 | A template hand-edited in a text editor — never opened in the designer — loads and renders | Pass |

### 9.3 Counter-metrics

Signals that MVP is succeeding in the wrong direction:

| # | Counter-metric | Why it matters |
|---|---|---|
| C1 | Expression function count exceeding the **eight** specified | The language is becoming a scripting language |
| C2 | Component palette exceeding five during MVP, or growing afterwards without a spec (Barcode and QR Code: `spec-barcode-qr-elements`) | Breadth is displacing reliability |
| C3 | Any determinism exception carved out to ship a feature | NFR1 is the product; erosion is fatal |
| C4 | Designer work starting before the engine renders the golden report | The sequencing rule, violated. Upheld by the epic breakdown — see §3 |
| C5 | Time-to-first-PDF for a new integrator exceeding a few minutes | The "extremely simple API" goal has failed |
| C6 | Golden hashes regenerated rather than investigated when CI goes red | The regression suite has stopped being a test |

---

## 10. Risks

Scope was deliberately held rather than cut, so the risks are carried explicitly.

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | **FMA contraction silently breaks NFR1.** Divergence is invisible in output and appears only in the hash; an amd64-and-wasm-only CI matrix passes anyway. | Critical | Fixed-point arithmetic (NFR1.a) makes it structural. An arm64 target in CI is mandatory. |
| R2 | **Thai line breaking must be written from scratch**, on the critical path. Re-verified after the licence decision: the maintained pure-Go options are dead, LGPL-licensed, read their dictionary from disk, or are permissively wrapped around copyleft code. | High → Medium | Scope the deliverable to break opportunities rather than segmentation (NFR3); embed a permissively-licensed dictionary as a compiled trie; open the text epic with a spike gated on real Thai text including proper nouns, which can halt and re-plan the epic. |
| R3 | **Library beta risk.** The strongest candidates for shaping, subsetting, and PDF writing are pre-1.0 with explicitly unstable APIs. | Medium | Pin versions; keep a second implementation available for cross-validation of shaping output. |
| R4 | **Compressor output is stable by observation, not by contract.** Go's compatibility promise says nothing about encoder bytes, so a future Go release could invalidate every recorded golden hash downstream. | Medium | Record the Go version alongside golden hashes; treat a toolchain bump as a versioned breaking change (NFR6). |
| R5 | **Solo capacity against held scope.** Reviewers identified this as the dominant risk to delivery. | High | Sequencing discipline: the engine must render the golden report before designer investment (C4). |
| R6 | **`Page X of Y` forecloses true streaming** for the life of MVP. | Low | Accepted (NFR4); API shape preserved so it can arrive later without a break. |

---

## 11. Open Questions

Q1, Q4, and Q9 from the first draft are answered and recorded in §8 and `addendum.md`;
their numbers are left unused so existing references stay valid. Q2, Q3, Q5, Q6, Q7, Q10 and
Q11 were answered after this PRD was first finalized; they are kept here with their answers
rather than deleted, so a reader who followed a reference still lands somewhere useful.

### Still open

| # | Question | Blocks |
|---|---|---|
| Q8 | Where does sample JSON live relative to the template during design (FR9), and does its path persist in the `.folio`? | Designer UX. Revisit at the first real authoring session |
| — | What are the forward and backward compatibility rules between template versions and library versions (NFR6)? Public distribution made this a commitment to third parties rather than a convention. | Needed before v1, not before MVP |

### Answered

| # | Question | Answer |
|---|---|---|
| Q2 | Which font faces ship, under what licences, and how does a user supply a custom font? | Noto Sans, Noto Sans Thai and Noto Sans SC (variable, glyf outline), all SIL OFL 1.1, travelling with their licence text. A custom font is supplied as part of the explicit font set passed to a render; there is no font-upload UI in MVP. |
| Q3 | What is the row scope rule inside a repeating region (FR17)? | A repeating region declares its alias explicitly, defaulting to `row`. Inside it the alias resolves to the current row; unqualified paths still resolve from the document root, so a row never shadows the root; `params.` can be shadowed by nothing. |
| Q5 | Does the build order stand, given tables depend on aggregation scheduled after them? | No — the source plan's order was wrong, because table footer aggregates *are* `sum()`, `count()` and `avg()`. Expressions now precede tables, and scalar binding moves earlier still. |
| Q6 | Does the designer really come last? | Engine-first stands and C4 holds, but the designer is two epics rather than one trailing phase, and the exact preview ships with the first of them. See §3. |
| Q7 | Licence and distribution? | **MIT, open source.** No dependency may carry GPL, LGPL, AGPL, SSPL or a commercial EULA at any depth, since Go links statically; this is enforced mechanically rather than by habit. Redistributed fonts keep their OFL terms and notices. |
| Q10 | What is the locale model for `formatDate` and `formatNumber`? | The template declares one locale tag and a fixed UTC offset. A compiled table covering a closed set — `en`, `th`, `zh-Hans`, `ja` — ships with the library; an unlisted tag is a load error, never a silent fallback. `th` renders Buddhist-era years. No host locale or host time zone is ever consulted. |
| Q11 | Which PDF version does Folio target? | PDF 1.7 (ISO 32000-1). The document ID is content-derived; creation and modification dates are omitted unless a date arrives through parameters. |

---

## 12. Assumption Register

| Location | Assumption |
|---|---|
| §3.1 | Both user journeys are inferred, not captured from a real session |
| FR9 | Sample JSON is a separate local file, not embedded in the template |
| FR17 | ~~Row scope needs explicit definition~~ — **resolved**, see Q3 |
| NFR1 | Byte-identity is proven in Folio's own code — the Epic 4 boundary matrix renders fifteen registered documents on four targets and compares their SHA-256 digests |
| NFR4 | No memory or throughput targets exist; a 50-page statement is the working target |
| NFR6 | Template/library compatibility policy undefined |
| NFR7 | ~~Specific font faces and their licensing unresolved~~ — **resolved**, see Q2 |
| §9 | Success is acceptance-based, not adoption-based, per current distribution intent |

---

## 13. Deliberate Omissions

Stated so a reviewer knows they were considered, not overlooked:

- **No monetization, pricing, or packaging.** Distribution is now decided — MIT, open source (Q7) — and no revenue model follows from it. Packaging remains out of scope.
- **No security or threat model** — MVP has no server, no accounts, and no untrusted
  multi-tenant surface. This must be revisited the moment the optional REST service (FR45)
  becomes real. Font parsing is an untrusted-input surface even in MVP, and a template arriving
  from an untrusted author is one too. Shipping publicly (Q7) does not change the runtime
  surface, since Folio still operates nothing — but it does mean untrusted templates and fonts
  will reach the library through people the author has never met.
- **No accessibility requirements** — neither designer UI accessibility nor tagged/accessible
  PDF (PDF/UA, PDF/A) is in MVP. Likely a requirement for any enterprise buyer, should Folio
  go public.
- **No locale model** — NFR3 covers *rendering* Thai and CJK text; it does not provide a
  locale model for formatting functions (Q10) or a translated designer UI.
- **No team, timeline, or budget** — solo project, sequencing over dates.
