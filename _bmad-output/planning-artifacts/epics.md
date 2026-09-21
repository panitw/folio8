---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-folio-2026-08-22/prd.md
  - _bmad-output/planning-artifacts/prds/prd-folio-2026-08-22/addendum.md
  - _bmad-output/planning-artifacts/architecture/architecture-folio-2026-08-23/ARCHITECTURE-SPINE.md
  - _bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/EXPERIENCE.md
  - _bmad-output/specs/spec-folio/SPEC.md
  - _bmad-output/specs/spec-folio/acceptance.md
  - _bmad-output/specs/spec-folio/glossary.md
  - _bmad-output/specs/spec-fonts/SPEC.md
  - _bmad-output/specs/spec-fonts/format-changes.md
  - _bmad-output/specs/spec-fonts/font-catalogue.md
excludedDocuments:
  - docs/folio-mvp-plan.md  # superseded by PRD §4 departures D1-D7
  - "**/review-*.md, **/.memlog.md"  # process records, not content
---

# folio - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for folio, decomposing the requirements from the PRD, UX Design, and Architecture into implementable stories.

## Requirements Inventory

### Functional Requirements

**Folio Designer**

FR1: Lay out a report on a visual canvas with visible page boundaries, placing components at absolute coordinates by drag-and-drop, and resizing them directly.
FR2: Configure page setup — A4, Letter, or custom page size; portrait or landscape; page margins.
FR3: Align work using a grid with snapping.
FR4: Place components from an MVP palette of exactly five: Text, Image, Table, Line, Rectangle.
FR5: Edit component properties — position (X/Y), size (width/height), font family, font size, bold, italic, text alignment, vertical alignment, border, padding, background, visibility, data binding.
FR6: Compose the report from three bands: Page Header, Content, Page Footer. Group/report headers and group footers deferred.
FR7: Bind a component to a JSON path through the designer UI, choosing from paths discovered in a loaded sample JSON document.
FR8: Open a `.folio` file from the local machine and save it back, with no server round-trip and no account.
FR9: Load a sample JSON data document alongside the template, used for binding discovery and preview.
FR10: Edit table structure in the designer — add/remove columns, set each column's width, alignment, and header label, bind each column to a field of the row scope, configure per-column footer aggregates.

**Template Format**

FR11: Persist a report definition as a portable, human-readable text file with extension `.folio`, carrying a format `version`, page setup, and ordered band content.
FR12: Remain readable, diffable, and mergeable in Git; usable in CI/CD and over an API; editable directly by a human or an AI agent without the designer.
FR13: Carry a format version the library validates on load, so a template authored against a future format fails clearly rather than rendering incorrectly.

**Data Binding and Expressions**

FR14: Accept report data as JSON supplied by the calling application. Folio never connects to a database.
FR15: Bind a scalar field by dotted path, e.g. `{{customer.name}}`.
FR16: Bind a repeating region to a collection, e.g. `transactions[]`.
FR17: Reference the current row's fields inside a repeating region through a defined row scope.
FR18: Evaluate a small expression language inside `{{ }}` bindings, offering exactly eight functions: `sum()`, `count()`, `avg()`, `formatDate()`, `formatNumber()`, `upper()`, `lower()`, `if()`.
FR19: Aggregate over a collection field, e.g. `{{sum(transactions.amount)}}`.
FR20: Control component visibility conditionally. Conditional visibility in scope; conditional formatting out.

**Parameters**

FR21: Accept runtime parameters separately from report data, addressed through a distinct namespace: `{{params.reportDate}}`.

**Tables and Repeating Sections**

FR22: Generate table rows dynamically from a bound data collection.
FR23: Lay out columns at fixed widths with per-column alignment.
FR24: Render a header row, cell borders, and cell padding.
FR25: Break a table across pages: rows are atomic; a row that does not fit moves whole to the next page; a row taller than the content area renders at the top of a fresh page clipped to that page's content height with a diagnostic; a footer aggregate row is never orphaned; cell content wraps within its column width; column widths never negotiate against content.
FR26: Repeat the table header at the top of every continuation page.
FR27: Render footer aggregates — sum, count, average.
FR28: *(if capacity permits)* Alternating row styling.

**Rendering and Output**

FR29: Produce PDF as the only required output format for MVP.
FR30: Render multi-page documents with page headers and page footers on every page.
FR31: Render page numbers, including the `Page X of Y` form, resolved by a two-pass layout.
FR32: Embed and subset all fonts used in the document, so the PDF renders identically on a machine that has none installed, and so font subsetting is byte-reproducible.
FR33: Embed images inside the `.folio` template itself. Folio does not fetch images by URL and does not read them from the filesystem at render time.

**Preview**

FR34: Offer a fast Design Canvas view for interactive editing, understood to be an approximate visual representation.
FR35: Offer an exact PDF Preview produced by the real `folio-go` engine compiled to WebAssembly running in the browser tab — not a browser-based approximation, not a remote service.
FR36: Operate the designer and its exact preview fully offline, with neither template nor data leaving the user's machine.

**folio-go Rendering Library**

FR37: Load a template from a file.
FR38: Render a template plus data to PDF bytes in-process, through an "extremely simple" API — a load call and a render call.
FR39: Render to an `io.Writer`, so a caller can write a PDF directly to an HTTP response without a temporary file. An API shape, not a constant-memory guarantee.
FR40: Accept runtime parameters at render time alongside the data.
FR41: Fail with a located, actionable error — naming the template element and the data path — for malformed templates, unresolvable bindings, invalid expressions, missing glyphs, and content that cannot be laid out. Non-fatal diagnostics are reported without failing the render.
FR42: Validate a template independently of rendering, so a malformed `.folio` can be rejected in CI before production.
FR43: Render as a pure function of (template, data, parameters) — no wall clock, time zone, locale, environment variable, filesystem, or network on the render path.
FR44: Define overflow behaviour for absolutely-positioned content exceeding its declared bounds: clipped at the component boundary with a diagnostic. Never reflowed, never silently dropped.

**REST Rendering Service** *(optional)*

FR45: *(optional, only if capacity permits)* Expose an HTTP endpoint accepting parameters and data and responding with `application/pdf`. Template travels in the request or resolves from an operator-supplied path — no server-side report repository.

**Long-Form Body Text and the Multi-Page Canvas** *(post-MVP, Epic 7)*

FR46: Preserve a mandatory line break authored inside a text value, so one component can hold more than one paragraph.
FR47: Justify a text element's edges to its declared width, leaving the last line of a paragraph unjustified.
FR48: Control the space between a text element's lines, as an integer ratio applied to the ruled inter-baseline advance. Absent means today's font-derived leading.
FR49: Author and edit multi-paragraph body text, its line spacing and its alignment in the designer.
FR50: Author on a canvas that extends across every page the content column currently occupies, with page-header and page-footer chrome repeated per page and breaks drawn where the engine will take them.
FR51: Declare a group of content-band elements that paginate together, so a signature block is never split across a page boundary.

**Fonts an Author Can Choose and a File Can Carry** *(post-MVP, Epic 8)*

FR52: Create, rename and delete the document's font chains, and reorder the entries within a chain, from the designer, so `fontFamily` names a family the author chose. *(Wording made explicit 2026-08-31: "reorder" always had its referent in ENTRIES, because a chain is an ordered list and `fonts` is a mapping with no authored key order. See Story 8.1.)*
FR53: Embed a font face in the template, keyed by content hash like every other asset, and reference it from a chain entry.
FR54: Render a template from its embedded faces alone — no network, no host-installed font, no install step on the rendering machine.
FR55: Choose a family from a curated, freely-licensed catalogue that ships with the designer and works with the browser offline.
FR56: Fail with a located error when a chain names a font that is neither a shipped face nor a present, decodable asset in the file.

**Typography, document settings, and the preview surface (post-MVP)**

FR57: Realize `style.bold` and `style.italic` as real weighted and sloped faces, resolved per rune through the declared chain, with no synthetic emboldening or obliquing anywhere.
FR58: Set the page-header and page-footer band heights from the designer.
FR59: Set the document's `locale` and `utcOffset` from the designer.
FR60: Set a table's header height, header style and alternating row background from the table editor.
FR61: Save the previewed PDF to a local file, byte-for-byte as the engine produced it.
FR62: Navigate the previewed PDF — fit-width, fit-page, a typed or chosen zoom, a typed page number, and a persistent scroll position.

### NonFunctional Requirements

NFR1: **Byte-reproducible rendering.** Same template + data + parameters + `folio-go` version produces a byte-identical PDF regardless of OS, machine, or compilation target — including across compilation targets, so the WebAssembly preview hash-matches the native render. Decomposes into seven constraints:
- NFR1.a: Layout arithmetic is contraction-free — integer fixed-point for all position, advance, and dimension math.
- NFR1.b: Numeric emission is quantized through one normative formatter at fixed decimal precision.
- NFR1.c: Restricted numeric surface — only correctly-rounded/exact operations; `math` transcendentals prohibited, enforced by lint.
- NFR1.d: No unordered iteration reaches output; every emitted collection is deterministically sorted.
- NFR1.e: Font subsetting is byte-stable — no wall-clock timestamp; subset tags a deterministic hash of the glyph set.
- NFR1.f: Document metadata pinned — creation/modification dates and document ID omitted or content-derived; `SOURCE_DATE_EPOCH` adopted.
- NFR1.g: No encryption in MVP — a random IV per encrypted string/stream is mutually exclusive with byte-reproducibility.
- **Verification:** CI matrix must include an arm64 target alongside amd64 and wasm.

NFR2: **The browser is not the canonical renderer.** Layout authority belongs to `folio-go` alone; browser text and layout engines must never determine pagination, line breaking, or measurement — including in the design canvas.

NFR3: **Script and text support.** Correctly shape, measure, break, and render Latin, CJK, and Thai. Full OpenType GSUB/GPOS including mark-to-base positioning, without cgo. Line breaking whitespace-delimited for Latin, per-character for CJK, dictionary-based for Thai. RTL out of scope. Thai dictionary embedded in the binary and versioned with the library.

NFR4: **Memory and streaming.** `Page X of Y` requires total page count before the first page, so the pipeline is two-pass. `RenderTo`'s writer API is an ergonomic and API-stability choice, not a constant-memory guarantee. No memory ceiling or throughput target specified; 50-page statement is the working target.

NFR5: **Fidelity between design and production.** The exact preview and the production render are the same engine and produce the same bytes. No separate preview renderer that could drift.

NFR6: **Versioning.** Templates carry a format version; library ships as `folio-go v0.1`. Because golden-hash regression tests are the primary verification mechanism, any change to layout, subsetting, or emission is a breaking change for downstream test suites. Forward/backward compatibility rules undefined and need a policy before v1.

NFR7: **Font provisioning.** All fonts embedded and subsetted; subsetting byte-stable. Latin and Thai coverage is embedded in the engine; CJK coverage is fetched on first use and kept, so "fully offline" is a claim about what this browser has fetched. Measured budget: engine and font stack ~1.5 MB compressed, CJK face 4.72 MiB brotli (deferred, and no longer embedded in the engine wasm), Thai dictionary ~0.1 MB. Take the glyf/TrueType static build over CFF/OpenType. Accepted: ~10.82 MiB first load — the core tier only. Superseded by `specs/spec-deferred-offline-cache`, which tiers the release and takes the CJK face out of the engine wasm; the CJK face is fetched on first use, not embedded twice.

NFR8: **Privacy posture.** The draw.io model plus WebAssembly preview means templates and data never leave the user's machine during design — a property to state and protect. **AMENDED 2026-09-21 by OWNER DECISION (D-GA.1), recorded at `ARCHITECTURE-SPINE.md` AD-27.** The guarantee above is UNCHANGED: templates, sample data and rendered PDFs still never leave the machine. What changed is that a deployed build may load Google Tag Manager and report page loads plus four fixed action names (`open_template`, `export_pdf`, `font_import`, `enter_preview`) and nothing else, gated on a build-time env var that is unset in development and in every test run. See `ARCHITECTURE-SPINE.md` AD-27.

### Additional Requirements

*Extracted from `ARCHITECTURE-SPINE.md` (24 ADs) and `addendum.md`.*

**🚨 STARTER TEMPLATE — impacts Epic 1 Story 1**

- **There is NO application starter template for the Go engine.** The architecture commits to Folio emitting its own PDF (AD-6) — no third-party PDF writer is a dependency — so the Go module is initialized from scratch: `go mod init github.com/panitw/folio/folio-go`, Go toolchain **pinned to an exact 1.26.x** via the `toolchain` directive (AD-22).
- **The designer is scaffolded from the standard Vite React-TS template** (`npm create vite`, React 19.2.x / Vite 7.3.x, Node 20.19+ or 22.12+) — the only "starter" in the project, and it pre-decides nothing architectural because the engine holds document state (AD-15).
- **Repository layout is a single repo**: Go module at root, designer in `designer/`, font binaries in `fonts/` as the single source of truth.

**Determinism infrastructure — must be wired before the first feature lands (AD-1, AD-21)**

- Import-restriction lint banning `time`, `os`, `math/rand`, `net`, and all `math` transcendentals under `internal/`.
- Map-iteration check confirming no unordered iteration reaches PDF emission.
- CI matrix: `darwin/arm64`, `linux/amd64`, **`linux/arm64`**, and `js/wasm` under Node — hashes compared across all four.
- Golden-hash fixtures recorded per `folio-go` version **and** per Go toolchain version.
- No change lands without a fixture covering it.

**Core engine structure**

- Paradigm: functional core (`internal/`), imperative shell (`folio`, `cmd/folio`, `wasm/`, `designer/`).
- `internal/geom` defines `Length` as `int64` millipoints (1/1000 pt) and is the only package declaring a geometric scalar type (AD-2). No `float64` anywhere under `internal/`.
- Numbers reach PDF output in exactly two representations — a decimal emitter for geometric values and an integer writer for structural counts/offsets — both in one file; no other code writes a number to output (AD-3, as amended under D-1.1.b/D-000.6).
- Two-pass pipeline; pass two performs no measurement, line breaking, or pagination. Page numbers are late-bound slots. No expression may reference pagination (AD-4).
- `internal/pagemodel` is renderer-agnostic; `internal/layout` may not import `internal/pdf` (AD-5).
- Report data numbers parsed as exact scaled-integer decimals via `UseNumber`, never `float64` (AD-23).
- Element x/y is relative to its band's top-left; PageModel is top-left origin Y-down; the flip to PDF space happens in exactly one function (AD-24).

**Format and contracts**

- `.folio` has exactly one canonical serialization: sorted keys, two-space indent, LF, no trailing whitespace, trailing newline. Round-trip properties ship with the format (AD-9).
- Images: content-addressed `assets` map keyed by SHA-256, base64 hard-wrapped at 76 columns into an array of strings (AD-9).
- Element ids: opaque short strings from a monotonic per-document counter persisted in the template. No UUIDs (AD-10).
- Row scope: explicit alias — `{"bind":"transactions[]","as":"transaction"}`, default `row`. A row never shadows the root. Aggregates always whole-collection, never page-scoped (AD-11) — **resolves PRD Q3**.
- Locale: document declares one locale tag and fixed UTC offset; embedded table for a closed set `en`/`th`/`zh-Hans`/`ja`; `th` renders Buddhist-era years (AD-12) — **resolves PRD Q10**.
- PDF 1.7 target; content-derived `/ID`; `/CreationDate` and `/ModDate` omitted unless supplied via params; `cmd/folio` reads `SOURCE_DATE_EPOCH` and passes it in (AD-7) — **resolves PRD Q11**.
- One `Diagnostic` type: `Severity`, stable code from a closed registry, optional element id, optional data path, message. Absent path = Error; JSON `null` = empty; wrong-typed value = Error (AD-14).
- Table width is derived from the sum of column widths, never stored (AD-13).

**Fonts and text**

- The engine never embeds font data; `Render` takes an explicit `FontSet` (AD-8). Fonts live once in `fonts/`; the designer build copies from there.
- Ordered fallback chain declared per family and part of the FontSet's identity. Uncovered glyph = diagnostic naming element and rune.
- One subset per font per **document**, over the union of glyphs used.
- Shaping/subsetting: `boxesandglue/textshape` v0.0.15. Cross-validation only: `go-text/typesetting` v0.3.4.
- **Thai line breaking must be built** — embed **PyThaiNLP `words_th`** (62,106 words, CC0-1.0) as a `BytesTrie`, longest-match/DAG segmentation. Prerequisite of text wrapping, **not** a refinement of it. The deliverable is **break opportunities, not word segmentation** (AD-25): unknown runs are atomic and no break falls inside a Thai character cluster, so an unrecognised customer name overflows visibly under FR44 rather than being silently mis-split.
- Fonts: Noto Sans, Noto Sans Thai, Noto Sans SC (variable, glyf), all OFL 1.1.

**Licensing — PRD Q7 RESOLVED: MIT, open source (AD-26)**

- Folio ships under **MIT**. **No dependency may carry GPL, LGPL, AGPL, SSPL, or a commercial EULA at any depth** — Go links statically, so such a dependency attaches its obligations to the whole binary.
- A **CI licence check** over the Go module graph and the `designer/` lockfile fails the build rather than warning. This is not hypothetical: the only live Go Thai segmenter is MIT on its own tin but derived from LGPL-3.0 code.
- Redistributed assets keep their own terms and notices — Noto and IBM Plex under **SIL OFL 1.1** with licence text and copyright lines; `pdfjs-dist` under **Apache-2.0** with its NOTICE. A third-party licence manifest is a release artifact.

**Designer / wasm**

- The wasm engine **owns** the template document; TypeScript holds an immutable snapshot and sends committed mutations as commands. There is no TypeScript model of a `.folio` document (AD-15).
- One wasm module, one instance, one dedicated Worker. Render entry point consumes serialized `.folio` bytes, never a live document (AD-16).
- Preview render takes **three** inputs: template bytes, sample data, and an author-edited **parameter document** (AD-16).
- Preview identity = hash of (template ∥ data ∥ params ∥ engine version ∥ FontSet identity); mismatch marks stale (AD-18).
- Service worker precaches app shell, wasm, Thai dictionary, and font assets under content-hashed URLs; brotli + immutable cache headers (AD-19).
- File access is two-tier and capability-detected: File System Access API where present, `<input type="file">` + download fallback elsewhere. One interface, one branch at startup (AD-20).
- Expression parser is hand-written recursive descent — no generator, no dependency. Template validation is `internal/template` itself — **no JSON Schema library** (AD-9).
- Preview surface is a controlled `pdfjs-dist` 6.2.x canvas, never the browser's built-in viewer.

### UX Design Requirements

*Extracted from `EXPERIENCE.md` (5 surfaces, 6 component patterns, 10 state patterns) and `DESIGN.md` (60 colour tokens, 13 type roles, 15 component specs).*

**Design sources — the five artboards, by path.** Post-MVP stories that rebuild a designer surface
carry a `**Design:**` line naming the artboard to build against and what to look at in it. All five
live in `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/`:

| Artboard | Surface |
|---|---|
| `Main.dc.html` | S2 Workspace — document bar, palette rail, canvas with band tabs, inspector |
| `Binding.dc.html` | S3 Binding panel — the DATA tab, and the table as the canvas draws it |
| `TableEditor.dc.html` | S4 Table editor — the column matrix and its HEADER / CELLS / BORDERS sections |
| `Preview.dc.html` | S5 Preview — PAGES rail, production output, the evidence rail |
| `Load.dc.html` | S1 Load — first-run payload manifest |

They are static HTML: open one in a browser beside the running designer. Where an artboard and this
document disagree, the disagreement is named in the story rather than resolved silently — an artboard
can draw a control the format cannot carry, and several do.

**Design system foundation**

UX-DR1: Implement the DESIGN.md token file as the designer's single source of styling — 60 named colour tokens across seven groups (chrome, rules/edges, text, select, bind, status, page), 6 named rgba tint washes, 13 typography roles across two ramps (chrome and page), a 10-step spacing scale, and a zero-radius shape rule. No hard-coded hex anywhere in the app.
UX-DR2: Enforce the **two-accent grammar** in every surface without exception — `select` cyan means structure/focus/authority; `bind` amber means data, and only data. Selection handles stay cyan even on a bound element.
UX-DR3: Implement the single **dark-chrome / light-canvas** theme. Not a dual-theme system: the white page is the only bright surface. Contrast ratios must be met on dark chrome's own terms, with no light-mode fallback to lean on.
UX-DR4: Enforce the gradient rule — colour-ramp gradients forbidden; hard-stop patterns (page dot grid, transparency checkerboard) permitted.

**Surfaces**

UX-DR5: **S1 Load** — a first-run-only honest loading screen that names what is loading and why it happens once, itemising the real payload (engine 1.5 MB, Latin 0.4, Thai 0.1, CJK 7.4, Thai dictionary 0.12 = ~9.5 MB) and explaining why CJK dominates. Progress indication required; a spinner is not acceptable at this payload size. The one deliberately non-dense surface; introduces the progress-bar and manifest-row components and the only display-size type token.
UX-DR6: **S2 Workspace** — canvas, palette, properties, document bar as persistent regions. Page rendered at true proportions with visible boundaries, three bands, and a snapping grid.
UX-DR7: **S3 Binding Panel** — docked right, not a separate destination. Loads a sample JSON document and exposes its paths as a navigable tree; binding connects a path to the selected component.
UX-DR8: **S4 Table Editor** — a focused surface over the canvas, invoked from a selected Table. Column configuration presented as a **matrix** (columns × attributes), never as a repeated single-column form. The densest UI in the product.
UX-DR9: **S5 Preview** — a **mode switch, not a panel**. The canvas is replaced by the exact rendered PDF.

**Component patterns**

UX-DR10: Build the six component patterns with their DESIGN.md token counterparts — canvas component (selection handle, binding marker), band (band tab, band boundary), palette item, property field, binding tree node, table column matrix row.
UX-DR11: Make **band drop-targeting unambiguous** — the target band highlights before release. Flagged as the most consequential ambiguity on the canvas: which band a component lands in determines whether it repeats on every page, so an ambiguous boundary produces wrong output rather than a cosmetic error.
UX-DR12: Implement the `error-card` spec, marked "specified, not mocked" — no artboard shows a failed render, so `colors.danger` is unverified by any mockup.

**State patterns**

UX-DR13: Implement all ten declared states per surface: Loading (first run), Empty—no template, Empty—no sample data, Empty—table with no columns, Populated, Rendering, Diagnostic, Error, Unsaved changes, Stale preview.
UX-DR14: **Stale preview is the one to get right** — the preview must never silently present a stale document as the production artefact. Either invalidate visibly or re-render. Every other state failure is an inconvenience; this one breaks the product's central promise.
UX-DR15: Rendering state blocks **within the preview surface only** — the canvas remains interactive, and progress is indicated for a 50-page document.
UX-DR16: Unsaved-changes indicator is persistent, quiet, and always visible. **There is no autosave** in either file-access tier.
UX-DR17: Diagnostic state is non-blocking and dismissible, and **locates the offending element** back on the canvas. Error state blocks within Preview, names element and path, and offers return to canvas.

**Interaction**

UX-DR18: Implement the interaction primitives — select (click, shift-click extends, empty-canvas click clears), move (drag, grid-snapped), resize (handles, constrained to the containing band), drop (palette to band with pre-release highlight), bind, mode switch (preserves selection and scroll position), open/save.
UX-DR19: Provide shortcuts for save, undo, redo, delete, duplicate, nudge, toggle preview, toggle snapping, with hints in menus and tooltips. **No command palette in MVP.** Mouse-first with shortcuts, not keyboard-first.
UX-DR20: Undo covers every canvas and property mutation. **Loading sample data is not undoable and must not appear to be.**

**Determinism in the interface**

UX-DR21: Make the canvas-approximate / preview-exact asymmetry legible **without a tutorial**. A user must never believe the canvas is authoritative.
UX-DR22: Surface output-affecting diagnostics — clipped element, over-tall row, glyph with no coverage — in the preview where the consequence is visible, and locate back to the offending element on the canvas.
UX-DR23: **Nothing in the interface may imply server rendering, a cloud round-trip, or an account.** The engine runs in the tab; the design must not borrow the visual language of remote processing.

**Voice and accessibility**

UX-DR24: Voice is **terse and technical** — states the fact, names the location, offers no comfort (e.g. `No binding for transactions[].amount`).
UX-DR25: Meet the accessibility floor as behavioural obligations, independent of formal conformance being out of scope: every interactive element keyboard-reachable and operable (palette, properties, binding tree, table columns); visible focus on every focusable element using `colors.select`; accessible names on every icon-only control; errors and diagnostics announced and distinguished by **shape before colour**; canvas handle hit targets larger than their visual footprint; the Table Editor behaving as a data grid under keyboard navigation.

### FR Coverage Map

### FR Coverage Map

Every FR maps to exactly one epic. FR45 is the only requirement not scheduled — it is optional
in the PRD and a Non-goal in `SPEC.md`.

| FR | Epic | What lands |
|---|---|---|
| FR11 | Epic 1 | `.folio` as a portable versioned text file |
| FR12 | Epic 1 | Diffable, mergeable, agent-editable — canonical serialization |
| FR13 | Epic 1 | Format version validated on load |
| FR14 | Epic 1 | JSON report data accepted; no database |
| FR15 | Epic 1 | Scalar binding by dotted path |
| FR29 | Epic 1 | PDF output |
| FR32 | Epic 1 | Font embedding + byte-stable subsetting (Latin) |
| FR33 | Epic 1 | Images embedded in the template, never fetched |
| FR37 | Epic 1 | Load a template from a file |
| FR38 | Epic 1 | Render to PDF bytes in-process |
| FR39 | Epic 1 | Render to an `io.Writer` |
| FR40 | Epic 1 | Runtime parameters accepted at render time |
| FR43 | Epic 1 | Render is a pure function |
| FR6 | Epic 2 | Three bands: Page Header, Content, Page Footer |
| FR30 | Epic 2 | Multi-page with headers and footers on every page |
| FR31 | Epic 2 | Page numbers including `Page X of Y` |
| FR44 | Epic 2 | Overflow clipped at the component boundary with a diagnostic |
| FR16 | Epic 3 | Collection binding |
| FR17 | Epic 3 | Row scope inside a repeating region |
| FR18 | Epic 3 | The eight expression functions |
| FR19 | Epic 3 | Aggregation over a collection field |
| FR20 | Epic 3 | Conditional visibility |
| FR21 | Epic 3 | Runtime parameter namespace |
| FR41 | Epic 3 | Located, actionable errors |
| FR42 | Epic 3 | Standalone template validation |
| FR22 | Epic 4 | Rows generated from a bound collection |
| FR23 | Epic 4 | Fixed column widths with per-column alignment |
| FR24 | Epic 4 | Header row, cell borders, cell padding |
| FR25 | Epic 4 | The page-break contract (5 sub-rules) |
| FR26 | Epic 4 | Header repeated on every continuation page |
| FR27 | Epic 4 | Footer aggregates — sum, count, average |
| FR28 | Epic 4 | Alternating row styling *(if capacity permits)* |
| FR1 | Epic 5 | Visual canvas, absolute placement, drag and resize |
| FR2 | Epic 5 | Page setup — size, orientation, margins |
| FR3 | Epic 5 | Grid with snapping |
| FR4 | Epic 5 | The five-component palette |
| FR5 | Epic 5 | Component property editing |
| FR8 | Epic 5 | Local open and save, no account |
| FR34 | Epic 5 | Approximate Design Canvas view |
| FR35 | Epic 5 | Exact PDF Preview via wasm |
| FR36 | Epic 5 | Fully offline operation |
| FR7 | Epic 6 | Bind a component to a JSON path via the UI |
| FR9 | Epic 6 | Load a sample JSON document |
| FR10 | Epic 6 | Table structure editing |
| FR45 | — | *Not scheduled. Optional in PRD, Non-goal in SPEC.* |
| FR46 | Epic 7 | Mandatory line breaks inside a text value |
| FR47 | Epic 7 | Justified alignment |
| FR48 | Epic 7 | Author-controlled line spacing |
| FR49 | Epic 7 | Body-text authoring in the designer |
| FR50 | Epic 7 | Multi-page authoring canvas |
| FR51 | Epic 7 | Keep-together groups |
| FR52 | Epic 8 | Authorable font chains |
| FR53 | Epic 8 | Font faces embedded in the template |
| FR54 | Epic 8 | Render from embedded faces alone |
| FR55 | Epic 8 | Curated offline font catalogue in the designer |
| FR56 | Epic 8 | Located failure for a broken font reference |
| FR57 | Epic 11 | Bold and italic realized as faces |
| FR58 | Epic 12 | Band heights authorable |
| FR59 | Epic 12 | Document locale and UTC offset authorable |
| FR60 | Epic 12 | Table header and alternating-row styling authorable |
| FR61 | Epic 13 | The previewed PDF saved to a local file |
| FR62 | Epic 13 | Preview navigation: fit, zoom, page, scroll |

**NFR coverage:** NFR1 and NFR6 are established in Epic 1 and defended by every epic thereafter
(AD-21: no change lands without a fixture). NFR3 and NFR7 land in Epic 2. NFR4 lands in Epic 2
(two-pass). NFR2, NFR5 and NFR8 land in Epic 5.

**UX-DR coverage:** UX-DR1–6, 9, 10, 11, 13–19, 21–25 land in Epic 5. UX-DR7, 8, 12, 20 land in
Epic 6.

## Epic List

**Six epics.** Engine first, designer second — but not the source plan's order. Two of the PRD's
open questions are resolved by this sequencing and are recorded below.

> **PRD Q5 — RESOLVED.** The source plan put tables at Phase 3 and the expression engine at
> Phase 4, but table footer aggregates *are* `sum()`, `count()` and `avg()`. The plan's order was
> simply wrong. Expressions (Epic 3) now precede tables (Epic 4). Scalar binding moves earlier
> still, into Epic 1, because a template that cannot resolve `{{customer.name}}` renders nothing
> worth hashing.

> **PRD Q6 — RESOLVED, and partially against the source plan.** Engine-first stands: counter-metric
> C4 is honoured, and no designer work begins until Epic 4 renders the golden report. But the
> designer is *not* one trailing phase. It is two epics, and the exact preview ships **with** the
> first of them rather than after it. The source plan's Phase 6 — preview last — is rejected:
> building a canvas you cannot preview with means authoring blind, and it would leave NFR2 and
> NFR5 unvalidated until the final phase of the project.

> **File-churn check — consolidation considered and rejected.** Two overlaps exist. Epics 2 and 4
> both touch `internal/layout`, but band layout and table layout are separate files with no
> feedback loop between them; merging would produce one epic carrying both R2 and the MVP's
> highest-risk engineering area. Epics 5 and 6 both touch `folio-designer/`, but canvas and
> chrome versus binding panels and table editor are distinct surfaces. In both cases the split
> buys a genuine risk boundary, so it stands.

> **Build-order gap closed.** Thai line breaking appears in no phase of the source plan. It is a
> **prerequisite** of Epic 2's text wrapping, not a refinement of it, and is scheduled as such.

### Epic 1: A Go developer can render a deterministic PDF

Anan adds `folio-go` to his module, loads a `.folio` file, renders it with his JSON data, and gets
PDF bytes — with the same SHA-256 on his Mac, on the Linux CI runner, on arm64, and under Node.
The library is integrable from this epic onward; everything after it adds capability without
changing the contract.

This epic is deliberately whole. Byte-identity cannot be retrofitted: the fixed-point unit, the
single numeric emitter, the ordering discipline, and the four-target harness have to exist before
the first feature, or every feature after them has to be re-verified. It carries the project's
Critical risk (R1) and retires it on the smallest possible artefact.

**FRs covered:** FR11, FR12, FR13, FR14, FR15, FR29, FR32, FR33, FR37, FR38, FR39, FR40, FR43
**Also lands:** NFR1.a–g, NFR6, AD-1, AD-2, AD-3, AD-6, AD-7, AD-9, AD-10, AD-21, AD-22, AD-23, AD-26

### Epic 2: A Go developer can render real multi-page documents in three scripts

A statement with a page header and footer on every page, correct `Page X of Y`, and Latin, Thai
and CJK text that wraps and breaks where a human would break it. Thai marks sit on their base
characters; Thai words break by dictionary; CJK breaks per character.

Split from Epic 1 because it carries the project's second-highest risk (R2: Thai segmentation must
be written from scratch, with no usable Go library to lean on), and because the two-pass pipeline
is the structural decision that `Page X of Y` forces.

**FRs covered:** FR6, FR30, FR31, FR44
**Also lands:** NFR3, NFR4, NFR7, AD-4, AD-5, AD-8, AD-24, AD-25
**Opens with:** a Thai segmenter spike gated on a corpus that includes Thai proper nouns, before the rest of the epic commits.

### Epic 3: A Go developer can render computed, parameterised documents

Values that are calculated rather than copied — totals, averages, formatted dates and numbers,
conditionally hidden components — and runtime parameters supplied separately from report data. A
Thai statement shows a Buddhist-era date. A malformed template is rejected in CI with a message
naming the element and the path, before it ever reaches production.

**FRs covered:** FR16, FR17, FR18, FR19, FR20, FR21, FR41, FR42
**Also lands:** AD-11, AD-12, AD-14

### Epic 4: A Go developer can render the golden report

The transaction table: rows from a bound collection, fixed columns, a header repeated on every
continuation page, sum footers that never orphan, and rows that never split across a page. At the
end of this epic the Customer Account Statement renders at 1, 5, 20 and 50 pages with identical
hashes on all four targets.

The PRD calls table pagination the highest-risk engineering area in the MVP and rules that correct
pagination beats table styling wherever the two compete. **This epic is the C4 gate: designer work
begins only once it passes.**

**FRs covered:** FR22, FR23, FR24, FR25, FR26, FR27, FR28
**Also lands:** AD-13, and success measures S1, S2, S3, S4

### Epic 5: A template author can lay out a report and see the real PDF

Ploy opens Folio in a browser, opens a `.folio` from her laptop, sets up an A4 page, drops a logo
and text fields into the three bands, and switches to Preview to see the exact production document
— the same engine, in her tab, with nothing sent anywhere. It works with the network disconnected.

Preview ships in this epic, not after it. The canvas-approximate / preview-exact asymmetry is the
product concept, and a canvas without a preview cannot validate NFR2 or NFR5.

**FRs covered:** FR1, FR2, FR3, FR4, FR5, FR8, FR34, FR35, FR36
**Also lands:** NFR2, NFR5, NFR8, AD-15, AD-16, AD-17, AD-18, AD-19, AD-20
**UX-DRs:** UX-DR1–6, 9, 10, 11, 13–19, 21–25

### Epic 6: A template author can bind a report to data and build the golden report

Ploy loads the sample JSON her developer gave her, walks its paths as a tree, binds each field by
picking rather than typing, drops a table into the Content band, and configures its five columns
as a matrix — widths, alignment, headers, row-scoped bindings, and a sum footer. She saves the file
and commits it. Anan renders it, and the hash matches what she saw.

This epic closes the loop the whole product exists to prove, and ends on success measure S5.

**FRs covered:** FR7, FR9, FR10
**UX-DRs:** UX-DR7, 8, 12, 20
**Also lands:** success measures S5, S7, S8, S9


### Epic 7: A template author can lay out a multi-page legal document

**Post-MVP.** Ploy opens a contract template, types a numbered clause as real body copy — hard
line breaks between paragraphs, justified edges, the line spacing her firm's house style demands —
and scrolls the canvas past the foot of page one onto page two and page three, where the schedule
and the signature block sit. She sees page breaks where the engine will actually put them. She
saves, and Anan renders forty pages of it from a longer contract without touching the template.

Two pillars, and they are separable: **body text** (7.1–7.4, and 7.8) makes a text element hold a paragraph
rather than a line, and **the multi-page canvas** (7.5–7.6) makes the authoring surface as long as
the document. 7.7 is the one legal-specific layout primitive neither pillar supplies, and it is the
story to cut first if the epic needs trimming. **7.8 was added mid-epic** (2026-08-30) when Story
7.4's plan gate routed DW-29 out of it: 7.4 stopped the designer OFFERING a justified table, and 7.8
makes the file format refuse one. It belongs to the body-text pillar and can ship independently of
everything after 7.4.

The engine's pagination model is **not** changed by this epic and must not be. `internal/layout`
already treats the content band as a window onto one unbounded column, and `page-count-50` /
`statement-50` already render fifty pages from a template nobody designed fifty pages for. Epic 7
lets the *designer* see that column; it does not introduce per-page layouts, which would mean
content that never reflows and, for a contract, silently clipped clause text.

Every field this epic adds is optional and absent-by-default, so the existing golden corpus must
hash identically after it. That is an acceptance criterion, not an aspiration.

**FRs covered:** FR46, FR47, FR48, FR49, FR50, FR51
**Also lands:** AD-4, AD-24 and D-2.6.1 upheld unchanged — the window model is a constraint on this epic, not a target of it
---

### Epic 8: A template author can choose a font, and the file carries it

**Post-MVP.** Ploy needs the firm's typeface, not the three faces the library happens to ship. She
searches a catalogue inside the designer — offline, no account, nothing fetched — picks a family,
and saves. Anan renders that file on a build box with no fonts installed and no network, and gets
the PDF she previewed, hash for hash. Nobody installed a font, and nobody had to be told which one
was missing.

The unit the author picks is a **face**; the unit the document names is a **chain** — an ordered
list resolved per rune for coverage. Both halves are missing today: nothing edits the `fonts` map
(so every document created in the designer offers exactly one family, `body`, from the starter
file), and nothing but the three build-time faces can be named. 8.1–8.2 make chains authorable,
8.3–8.4 make a face something the file carries and the engine renders from, 8.5–8.6 make choosing
one a search rather than a hand-edit.

`.folio` stays a **single JSON text file**. Font bytes ride the existing content-addressed `assets`
map — the mechanism images already use — because the format exists so a person or an agent can edit
a template without the designer (FR12) and so a hand-written template renders (S9). A container
format ends both and puts entry order, timestamps and compression inside the byte-identity regime;
SPEC-fonts records it as a non-goal with the CJK weight case named as the only trigger to revisit.

Bold and italic are **not** in this epic. They are stored and projected today and consumed by no
producer, and no weighted face ships; giving them meaning is not a consequence of embedding. That
work is **Epic 11 (FR57)**, which owns the realize-or-retire decision in full — realize them as
shipped weighted and sloped faces, or retire the toggles. SPEC-fonts records the same question as
open; Epic 11 is where it is answered.

Every field this epic adds is optional and absent-by-default, so the existing golden corpus must
hash identically after it. That is an acceptance criterion, not an aspiration. 8.5 is the story to
trim rather than cut — the catalogue can ship with one family and grow by release.

**FRs covered:** FR52, FR53, FR54, FR55, FR56
**Also lands:** AD-9, AD-15, AD-22 and AD-26 upheld unchanged — canonical serialization, the
engine's ownership of the document, byte identity, and licence provenance are constraints on this
epic, not targets of it
---

## Epic 1: A Go developer can render a deterministic PDF

Anan adds `folio-go` to his module, loads a `.folio` file, renders it with his JSON data, and gets PDF bytes — with the same SHA-256 on his Mac, on the Linux CI runner, on arm64, and under Node. The library is integrable from this epic onward; everything after it adds capability without changing the contract.

### Story 1.1: A minimal PDF, reproducible on one machine

As an integrating Go developer,
I want `folio-go` to emit a minimal valid PDF whose bytes are identical every time I render the same input,
So that hash comparison becomes my regression test before any feature exists to regress.

**Covers:** FR29, FR43 · NFR1.a, NFR1.b, NFR1.d, NFR1.f, NFR6 · AD-1, AD-2, AD-3, AD-6, AD-7, AD-22

**Acceptance Criteria:**

**Given** a new Go module `github.com/panitw/folio/folio-go` with an exact `toolchain go1.26.x` directive
**When** the module is built
**Then** the pinned toolchain version is used, and the version is recorded in a file the golden fixtures reference
**And** no dependency on any third-party PDF writer exists in `go.mod`

**Given** the `internal/geom` package
**When** any position, advance, or dimension is computed
**Then** it is an `int64` `Length` in millipoints (1/1000 pt)
**And** `internal/geom` is the only package declaring a geometric scalar type
**And** font scaling is a single exported function with round-half-to-even on the exact integer quotient

**Given** a hard-coded page containing one filled rectangle
**When** `Render` is called
**Then** a valid PDF 1.7 document is produced with catalog, page tree, content stream, and cross-reference table
**And** every geometric number in the output was written by the single unexported decimal emitter as sign + integer part + up to three trimmed fractional digits, with structural counts, offsets and object/generation numbers routed through a separate integer writer in the same file (D-1.1.b: AD-3 governs numeric representations, not the number of functions)
**And** no `float64` appears in any signature under `internal/`

**Given** the same input rendered twice in two separate OS processes
**When** the two outputs are compared
**Then** they are byte-identical
**And** `/CreationDate` and `/ModDate` are absent
**And** `/ID` is the first 16 bytes of a SHA-256 over the pre-`/ID` body, with both array entries identical

**Given** the render completes
**When** the recorded golden hash is compared
**Then** it matches, and the fixture records both the `folio-go` version and the Go toolchain version

### Story 1.2: Cross-target byte-identity proven in CI

As an integrating Go developer,
I want CI to prove the same render produces identical bytes on every architecture and compilation target,
So that the PDF my Linux service returns is provably the same file that was verified on a developer's Mac.

**Covers:** NFR1 · AD-21 · retires Risk R1

**Acceptance Criteria:**

**Given** the CI configuration
**When** a build runs
**Then** it executes on `darwin/arm64`, `linux/amd64`, `linux/arm64`, and `js/wasm`
**And** the `js/wasm` target is executed under Node or an equivalent runtime
**And** `CGO_ENABLED=0` on every target

**Given** Story 1.1's minimal document rendered on all four targets
**When** the four outputs are hashed
**Then** all four hashes are identical
**And** the job fails if any pair differs

**Given** a deliberately introduced floating-point multiply-add in layout arithmetic
**When** CI runs
**Then** the arm64 job produces a different hash from the wasm job and the build fails
**And** this negative test is retained as proof the matrix can actually detect contraction

**Given** the reference render used for comparison
**When** it is validated
**Then** it is confirmed non-blank and page-count-correct, so hash equality cannot pass on two identical empty files

### Story 1.3: Guardrails that fail the build

As an integrating Go developer,
I want the determinism and licence rules enforced mechanically rather than by convention,
So that a reasonable-looking commit cannot quietly erode the property the whole library rests on.

**Covers:** NFR1.c, NFR1.d · AD-1, AD-26

**Acceptance Criteria:**

**Given** the import-restriction lint
**When** any package under `internal/` imports `time`, `os`, `math/rand`, `net`, or a `math` transcendental (`Sin`, `Cos`, `Tan`, `Log`, `Exp`, `Pow`, `Sinh`, `Erf`, …)
**Then** the build fails naming the file and the offending import
**And** the allow-listed numeric surface is `+ - * /`, comparison, `Sqrt`, `Floor`, `Ceil`, `Round`, `Trunc`, `Abs`, `Mod`

**Given** the map-iteration check
**When** a `range` over a map can reach an output byte
**Then** the build fails naming the location

**Given** the licence check
**When** any dependency in the Go module graph or the `designer/` lockfile carries GPL, LGPL, AGPL, SSPL, or a commercial EULA at any depth
**Then** the build fails rather than warning
**And** a third-party licence manifest is produced as a release artifact, carrying OFL 1.1 text for shipped faces and the Apache-2.0 NOTICE for `pdfjs-dist`

**Given** each guardrail
**When** it is added
**Then** a deliberately violating fixture proves it fires, and the fixture is retained

### Story 1.4: Load, validate, and round-trip a `.folio` template

As an integrating Go developer,
I want to load a `.folio` file and have malformed or future-versioned templates rejected with a located error,
So that a bad template is caught in CI rather than in production.

**Covers:** FR11, FR12, FR13, FR37 · NFR6 · AD-9, AD-10

**Acceptance Criteria:**

**Given** a well-formed `.folio` file carrying a `version`, page setup, and ordered band content
**When** `folio.LoadTemplate(path)` is called
**Then** a `Template` is returned with no error

**Given** a template declaring a format version newer than the library supports
**When** it is loaded
**Then** loading fails with an error naming the declared version and the supported version
**And** no render is attempted

**Given** any parsed template `d` and any canonical byte sequence `b`
**When** round-trip properties are tested
**Then** `Parse(Serialize(d)) == d` and `Serialize(Parse(b)) == b` hold
**And** these tests ship with the format, not after it

**Given** a template serialized by the library
**When** the bytes are inspected
**Then** object keys are sorted, indentation is two spaces, line endings are LF, there is no trailing whitespace, and the file ends with a newline

**Given** any element in a loaded template
**When** its identity is inspected
**Then** it carries an opaque short id from a monotonic counter persisted in the document
**And** ids are not UUIDs, are never reused, and are unchanged by a save

### Story 1.5: Embed and subset a font, byte-stably

As an integrating Go developer,
I want every font the document uses embedded as a subset that is identical on every run,
So that the PDF renders correctly on a machine with no fonts installed without breaking hash equality.

**Covers:** FR32 · NFR1.e, NFR7 · AD-8

**Acceptance Criteria:**

**Given** a `FontSet` value supplied to `Render`
**When** the render executes
**Then** fonts are resolved by pure lookup against that `FontSet`
**And** no package under `internal/` embeds font data
**And** no host font query occurs

**Given** a document using a subset of a Latin face
**When** the PDF is produced
**Then** the face is embedded as a `Type0`/`Identity-H` composite font with a `FontFile2` and a `ToUnicode` CMap
**And** the six-letter subset tag is derived from a hash of the sorted glyph-id set
**And** one subset is emitted per font per document over the union of glyphs used, never per page

**Given** the same document rendered in two separate processes
**When** the embedded font programs are compared
**Then** they are byte-identical
**And** no wall-clock timestamp appears in the subset output

**Given** the resulting PDF opened on a machine with none of its fonts installed
**When** it is displayed
**Then** the text renders correctly

### Story 1.6: Bind scalar JSON values into text

As an integrating Go developer,
I want to supply my data as JSON and have `{{customer.name}}` resolve to the right value,
So that the template is a template rather than a fixed document.

**Covers:** FR14, FR15, FR38, FR43 · AD-14, AD-23

**Acceptance Criteria:**

**Given** a JSON document supplied as report data
**When** it is decoded
**Then** number literals are preserved and converted to exact scaled-integer decimals carrying the literal's own precision
**And** no `float64` is produced anywhere in the binding path
**And** a literal too large for the representation is an error, never a silent narrowing

**Given** a text element bound to `{{customer.name}}`
**When** the document renders
**Then** the value at that dotted path appears in the output

**Given** a binding whose path is absent from the data
**When** the document renders
**Then** the render fails with an error naming the path and the element id

**Given** a binding whose value is JSON `null`
**When** the document renders
**Then** it renders as empty and is not an error

**Given** a binding whose value is of the wrong kind for its element
**When** the document renders
**Then** the render fails with an error; the value is never coerced

**Given** a render in progress
**When** the render path is inspected
**Then** it has read no wall clock, time zone, locale, environment variable, filesystem path, or network resource

### Story 1.7: Render to an `io.Writer` with runtime parameters

As an integrating Go developer,
I want to write a PDF straight to my HTTP response and pass runtime values separately from report data,
So that I can serve a generated document without a temporary file and without smuggling dates into the data.

**Covers:** FR39, FR40

**Acceptance Criteria:**

**Given** an `http.ResponseWriter`
**When** `folio.RenderTo(w, template, data, params)` is called
**Then** PDF bytes are written directly to it with no temporary file created

**Given** the same inputs
**When** rendered through `Render` and through `RenderTo`
**Then** the resulting bytes are identical

**Given** a parameter supplied at render time
**When** a `{{params.reportDate}}` binding is resolved
**Then** it resolves from the parameter namespace, not from report data
**And** report data cannot shadow the `params.` namespace

**Given** the public API surface
**When** an integrator reads the README
**Then** producing a first PDF requires a load call and a render call, and nothing ceremonial

### Story 1.8: Embed and render an image from the template

As a template author,
I want my logo carried inside the `.folio` file itself,
So that the template is self-contained and rendering never depends on a URL or a file on disk.

**Covers:** FR33 · AD-9, AD-24

**Acceptance Criteria:**

**Given** a template containing an image
**When** the file is inspected
**Then** the image lives in a top-level `assets` object keyed by the SHA-256 of its bytes
**And** its base64 is hard-wrapped at 76 columns into an array of strings
**And** the file remains valid JSON and its non-asset content stays readable in a text diff

**Given** two elements referencing the same image
**When** the template is serialized
**Then** the bytes are stored once and both elements reference the same asset key

**Given** a template whose image is rendered
**When** the PDF is produced
**Then** the image appears as an image XObject
**And** the render made no network request and read no file from disk

**Given** an image drawn into a declared box
**When** its placement is computed
**Then** it is scaled to fit preserving aspect ratio and centred within the box, computed in integer millipoints
**And** it is never stretched, never cropped, and never sized from its intrinsic pixel dimensions

---

## Epic 2: A Go developer can render real multi-page documents in three scripts

A statement with a page header and footer on every page, correct `Page X of Y`, and Latin, Thai and CJK text that wraps and breaks where a human would break it. Thai marks sit on their base characters; Thai words break by dictionary; CJK breaks per character.

### Story 2.1: Thai break-opportunity spike

As a solo builder,
I want to prove the Thai break approach works against real Thai text including proper nouns before committing the rest of this epic,
So that the project's second-highest risk is retired cheaply rather than discovered halfway through the layout engine.

**Covers:** NFR3 · AD-25 · retires Risk R2

**Acceptance Criteria:**

**Given** the PyThaiNLP `words_th` wordlist (62,106 words, CC0-1.0)
**When** it is compiled
**Then** it is a `BytesTrie` embedded in the binary via `go:embed`
**And** it is never read from disk, and no code path uses `runtime.Caller`
**And** the trie loads and queries correctly under `js/wasm`

**Given** a Thai evaluation corpus that deliberately includes Thai personal names, place names, and transaction descriptions
**When** break opportunities are computed
**Then** results are recorded and hand-reviewed, and the review is kept as the basis of the S4 fixture

**Given** a run of Thai characters the dictionary cannot cover
**When** break opportunities are computed
**Then** the run yields no interior break opportunities and is treated as atomic
**And** no unrecognised name is split at a guess

**Given** any Thai character cluster — a consonant with its vowels and tone marks
**When** break opportunities are computed
**Then** no break falls inside it

**Given** the spike's findings
**When** they are reviewed
**Then** either the approach is confirmed and the remaining stories proceed, or the deviation is recorded and the epic is re-planned before further work

### Story 2.2: The shipped font set and its fallback chain

As a template author,
I want Latin, Thai and CJK glyphs available without installing anything,
So that my statement renders the same for me as it does on a server that has no fonts at all.

**Covers:** NFR7 · AD-8

**Acceptance Criteria:**

**Given** the repository
**When** font binaries are located
**Then** they exist in exactly one place, `fonts/`, and the `folio/fonts` package wraps them with `go:embed` for native callers

**Given** the shipped set
**When** it is inspected
**Then** it covers Latin (Noto Sans), Thai (Noto Sans Thai), and CJK (Noto Sans SC, variable, glyf outline format), all SIL OFL 1.1
**And** each face travels with its licence text and copyright lines

**Given** a template naming a font family
**When** the family is resolved
**Then** it resolves against an ordered fallback chain declared in the `FontSet`
**And** the chain is part of the `FontSet`'s identity, so the same template with a different chain is a different render

**Given** a glyph covered by no font in the chain
**When** the document renders
**Then** a diagnostic is reported naming the element id and the offending rune
**And** no blank box is silently emitted

### Story 2.3: Shape Latin, Thai and CJK text

As a template author,
I want Thai vowels and tone marks to sit correctly on their base characters,
So that my customers' names are not merely present but correct.

**Covers:** NFR3

**Acceptance Criteria:**

**Given** text in Latin, Thai or CJK
**When** it is shaped
**Then** shaping runs through `boxesandglue/textshape` with full OpenType `GSUB` and `GPOS`
**And** no cgo is used and the build succeeds for `js/wasm`

**Given** Thai text containing vowels and tone marks
**When** it is shaped
**Then** marks are positioned by `GPOS` mark-to-base positioning
**And** output is cross-validated against an independent reference implementation for the same input

> **AMENDED IN DEVELOPMENT, Story 2.3 (clause 1: D-000.24 · clause 2: D-2.3.1).** Two clauses above
> were written before Stories 2.1 and 2.2 landed and were measurably wrong as stated.
>
> *(Citation corrected by Story 2.3's finisher. This block previously cited "D-2.3.2, D-2.3.3".
> **D-2.3.2** is the `/ToUnicode` CID-allocation ruling and governs neither clause; **D-2.3.3** is
> titled "`epics.md:756` is weak, not false, and is **NOT** amended" — a ruling whose entire content
> is a refusal to amend, offered as the authority for an amendment. The two blocks' citations were
> effectively swapped. D-000.6 requires an amendment to name its authorising ruling, and a citation
> that reads as support and inverts on inspection is the citation-integrity analogue of the false
> credit closed in `fixtures/shaped-text/README.md`: both point a reader at evidence that says the
> opposite of what they were told it says.)*
>
> **1. "marks are positioned by `GPOS` mark-to-base positioning" is true but is not the
> observable.** Measured across all three shipped faces: `GlyphPos.YOffset` is **0 for every glyph
> of every sample**. What actually moves is a **horizontal** offset plus a **`GSUB` substitution** —
> Thai's lowered mark forms. An acceptance criterion phrased as *"assert some mark has a non-zero
> `YOffset`"* is vacuously false and would block a correct implementation; one phrased as *"assert
> a non-zero `XOffset`"* guards only the field that happens to move in these three faces, and a
> fourth face positioning vertically sails past it. The story asserts **all five fields**
> (`GlyphID`, `Cluster`, `XAdvance`, `XOffset`, `YOffset`) for **every glyph of every row** against
> a declarative frozen table, with `YOffset` documented as a **forward guard with no available
> red-proof** rather than credited with one it does not have.
>
> **2. `go-text/typesetting` is not the oracle, and would have been a weak one.**
> `gomod_test.go`'s `wantModuleGraph` asserts `go list -m all` equals exactly two modules, and
> `go list -m all` includes test-only dependencies — so adding it fails a committed guard whose
> purpose is to make a new module a conscious act (D-1.5.1). **`epics.md` predates that guard.**
> Independently of the guard: `textshape`'s README credits `benoitkugler/textlayout` as its
> inspiration and `go-text/typesetting` descends from the same code, so two ports with a shared
> ancestor agreeing is the vacuous-citation shape — if both would make the same mistake, the
> agreement predicts the same observation either way.
>
> The oracle is **HarfBuzz itself** (`hb-shape` 14.2.0), the reference implementation, run against
> **folio's** corpus as a one-time offline reference run, hand-checked and frozen at
> `fixtures/shaped-text/harfbuzz-oracle.json` — **no new module, `wantModuleGraph` untouched**
> (AD-25; Story 1.1's `qpdf --check` precedent). Sixteen rows, all five fields per glyph, agreeing
> value for value.

**Given** the same text shaped in two separate processes
**When** the resulting glyph runs are compared
**Then** glyph ids and positions are byte-identical

> **CLARIFIED IN DEVELOPMENT, Story 2.3 (D-2.3.3).** *(Citation corrected by Story 2.3's finisher;
> this block previously cited D-2.3.1, the HarfBuzz oracle ruling, which governs `:753` and not this
> clause. D-2.3.3 is the ruling on this clause, and note what it licenses: it holds `:756` to be
> **weak, not false**, so this is a clarifying note appended beside the clause and **not** a D-000.6
> amendment — the clause text itself is untouched.)*
>
> Read as a property of the *intermediate glyph
> buffer*, this is a property of the **vendor**, not of folio, and it was already true at Story
> 2.3's baseline before any shaping code existed — near-vacuous as an acceptance criterion.
> Measured anyway and recorded: five separate process invocations over the same inputs produced
> **one distinct output digest, 5/5**.
>
> The artifact folio actually sells is **the rendered PDF**, so that is where the criterion is
> enforced — by D-000.4's four-target byte-identity matrix, which now covers
> `fixtures/shaped-text/` as its fifth registered document. The `js/wasm` and `CGO_ENABLED=0`
> clause above is likewise kept as a **regression** guard and labelled as one: it too was already
> true at baseline, so it is never evidence that this story landed.

### Story 2.4: Break and measure lines in all three scripts

As a template author,
I want text to wrap where a reader would expect it to,
So that a multi-line address or transaction description is legible rather than merely fitting.

**Covers:** NFR3 · AD-2, AD-25 · S4

**Acceptance Criteria:**

**Given** Latin text
**When** lines are broken
**Then** breaks occur at whitespace, and at script-appropriate opportunities in every other script

**Given** CJK text
**When** lines are broken
**Then** breaks occur per character

**Given** Thai text
**When** lines are broken
**Then** breaks come from the embedded dictionary, constrained by Story 2.1's two rules — unknown runs atomic, never inside a character cluster

**Given** the frozen expected-break fixture for Thai and CJK
**When** the breaker runs against it
**Then** results match the fixture exactly
**And** the fixture is hand-checked once and then never regenerated to make a test pass

**Given** any text measurement
**When** it is computed
**Then** it is exact integer arithmetic in millipoints with no `float64` anywhere in the path

### Story 2.5: Compose a page from three bands

As a template author,
I want my report composed of a Page Header, a Content region and a Page Footer,
So that which band I place something in decides whether it repeats on every page.

**Covers:** FR6 · AD-5, AD-24

**Acceptance Criteria:**

**Given** a template
**When** its structure is inspected
**Then** it has exactly three bands — Page Header, Content, Page Footer
**And** group headers, report headers and group footers are absent and unsupported

**Given** an element within a band
**When** its coordinates are resolved
**Then** x and y are relative to that band's top-left corner, never to the page
**And** bands are placed on the page by `internal/layout` alone

**Given** the page model produced by layout
**When** it is inspected
**Then** its origin is top-left with Y increasing downward
**And** it names only geometry, glyph runs, images and vector primitives — no PDF object references, resource dictionaries or content-stream operators
**And** `internal/layout` does not import `internal/pdf`

**Given** the flip from page-model space to PDF user space
**When** the code is inspected
**Then** it occurs in exactly one function in `internal/pdf` and nowhere else

**Given** page dimensions, margins and band heights
**When** the content area is computed
**Then** it is derived by one function as page height minus margins minus header height minus footer height

### Story 2.6: Render multi-page documents with headers and footers on every page

As an integrating Go developer,
I want a document longer than one page to carry its header and footer throughout,
So that page 34 of a statement is as complete as page 1.

**Covers:** FR30 · NFR4 · AD-4

**Acceptance Criteria:**

**Given** content exceeding one page
**When** the document renders
**Then** it paginates into multiple pages
**And** the Page Header and Page Footer bands appear on every page

**Given** the pipeline
**When** it executes
**Then** pass one produces a complete page model and pass two serializes it
**And** pass two performs no measurement, no line breaking and no pagination

**Given** a document that grows
**When** the Content band overflows
**Then** it grows by producing more pages, never by displacing sibling components within a page

### Story 2.7: Render `Page X of Y`

As a template author,
I want a footer that says which page of how many the reader is holding,
So that a printed statement is verifiably complete.

**Covers:** FR31 · AD-4

**Acceptance Criteria:**

**Given** a footer containing a `Page X of Y` construct
**When** the document renders
**Then** X is the current page and Y the document total, correct on every page

**Given** the page model after pass one
**When** page-number text is inspected
**Then** it is carried as a late-bound slot whose box is already measured
**And** it is resolved between the passes by substituting pre-measured glyphs

**Given** the expression language
**When** an author attempts to reference pagination
**Then** no `page` namespace exists and none can be added

**Given** documents of 1, 5, 20 and 50 pages
**When** each renders
**Then** `Page X of Y` is correct throughout and hashes match recorded goldens on all four targets

### Story 2.8: Clip overflowing content and say so

As a template author,
I want content that exceeds its box cut at the boundary with a diagnostic rather than silently lost,
So that I find out on the preview instead of from a customer.

**Covers:** FR44 · AD-14, AD-24

**Acceptance Criteria:**

**Given** absolutely-positioned content exceeding its declared bounds
**When** the document renders
**Then** it is clipped at the component boundary
**And** a diagnostic is reported naming the element id
**And** it is never reflowed and never silently dropped

**Given** the diagnostic
**When** the render completes
**Then** PDF bytes are still returned — clipping degrades output but does not fail the render

---

## Epic 3: A Go developer can render computed, parameterised documents

Values that are calculated rather than copied — totals, averages, formatted dates and numbers, conditionally hidden components — and runtime parameters supplied separately from report data.

### Story 3.1: Bind a repeating region to a collection with an explicit row scope

As a template author,
I want to bind a region to `transactions[]` and address each row's fields unambiguously,
So that the rule for what `{{transaction.amount}}` means is written down rather than guessed.

**Covers:** FR16, FR17, FR21 · AD-11

**Acceptance Criteria:**

**Given** a repeating region declaring `{"bind": "transactions[]", "as": "transaction"}`
**When** its contents are evaluated for a row
**Then** `transaction.*` resolves to that row's fields

**Given** a repeating region that omits `as`
**When** its contents are evaluated
**Then** the alias defaults to `row`

**Given** an unqualified path inside a repeating region
**When** it is resolved
**Then** it resolves from the document root — a row never shadows the root

**Given** a `params.` path anywhere, including inside a row scope
**When** it is resolved
**Then** it resolves from the parameter namespace and can be shadowed by nothing

**Given** a collection binding whose path is not an array
**When** the document renders
**Then** the render fails with an error naming the path and the element id

### Story 3.2: Evaluate the expression language

As a template author,
I want a small set of expressions inside my bindings,
So that I can compute a value without the template becoming a program.

**Covers:** FR18 · AD-9 · counter-metric C1

**Acceptance Criteria:**

**Given** the expression implementation
**When** it is inspected
**Then** it is a hand-written recursive-descent parser with no generator and no third-party dependency
**And** the function table is closed and contains exactly eight entries

**Given** `upper()`, `lower()` and `if()`
**When** they are evaluated
**Then** each behaves per its specification and has tests covering its edge cases

**Given** an expression with a syntax error
**When** the template is loaded
**Then** it fails with an error naming the element id and the offending expression text

**Given** an attempt to add a ninth function
**When** CI runs
**Then** the closed function table makes the addition visible in a diff, per counter-metric C1

### Story 3.3: Aggregate over a collection

As a template author,
I want a statement total that adds up,
So that the sum footer on my transaction table is correct to the last satang.

**Covers:** FR19 · AD-11, AD-23

**Acceptance Criteria:**

**Given** `{{sum(transactions.amount)}}`
**When** it is evaluated
**Then** the result matches a hand-computed total exactly
**And** the arithmetic is exact decimal, never binary floating point

**Given** `count()` and `avg()`
**When** they are evaluated
**Then** `count()` returns the collection length and `avg()` divides at a defined scale with round-half-to-even

**Given** an aggregate evaluated inside a row scope
**When** it resolves
**Then** it is computed over the whole collection, never over the rows on the current page
**And** no per-page subtotal behaviour exists

**Given** an aggregate over an empty collection
**When** it is evaluated
**Then** `sum` and `count` return zero and `avg` reports a diagnostic rather than dividing by zero

### Story 3.4: Format dates and numbers by declared locale

As a template author in Bangkok,
I want a Thai statement to show a Buddhist-era date and correctly grouped numbers,
So that the document reads as a Thai document rather than a translated one.

**Covers:** FR18 · AD-1, AD-12

**Acceptance Criteria:**

**Given** a document declaring a `locale` tag and a fixed UTC offset
**When** `formatDate` or `formatNumber` is evaluated
**Then** symbols and calendar come from the embedded locale table for that tag
**And** no host locale, host time zone or host clock is consulted

**Given** the locale table
**When** it is inspected
**Then** it covers exactly `en`, `th`, `zh-Hans` and `ja`
**And** an unlisted tag is a load error, never a silent fallback

**Given** the `th` locale
**When** a date is formatted
**Then** the year is rendered in the Buddhist era

**Given** a date value
**When** it is passed to `formatDate`
**Then** only an RFC 3339 string or an epoch-millisecond number is accepted; anything else is an error

**Given** `formatNumber` scaling by powers of ten
**When** the implementation is inspected
**Then** it uses an integer lookup table and never `math.Pow`

### Story 3.5: Hide a component conditionally

As a template author,
I want to hide a component when the data says it is irrelevant,
So that an empty section does not print as an empty box.

**Covers:** FR20 · AD-24

**Acceptance Criteria:**

**Given** a component with a visibility condition
**When** the condition evaluates false
**Then** the component is absent from the page model entirely

**Given** a hidden component
**When** the page is laid out
**Then** its siblings do not move — nothing in a band ever reflows

**Given** a visibility condition applied to a table row
**When** the template is validated
**Then** it is rejected: visibility applies to elements only, because row-level visibility would make pagination a function of data

**Given** conditional formatting — data-driven styling
**When** it is attempted
**Then** it is unsupported and out of scope; only visibility is conditional

### Story 3.6: Fail with located, actionable diagnostics

As an integrating Go developer,
I want every failure to tell me which element and which data path caused it,
So that debugging a template is reading one message rather than bisecting a document.

**Covers:** FR41 · AD-14

**Acceptance Criteria:**

**Given** the diagnostic type
**When** it is inspected
**Then** one `Diagnostic` value carries `Severity`, a stable code from a closed registry, an optional element id, an optional data path, and a message

**Given** severity
**When** a render runs
**Then** `Error` aborts it and `Warning` accompanies successfully returned PDF bytes

**Given** each failure mode named in FR41 — malformed template, unresolvable binding, invalid expression, missing glyph, unlayoutable content
**When** it occurs
**Then** it produces a distinct stable code, and a test asserts the message locates the problem

**Given** a caller matching on a failure
**When** they write the check
**Then** they match on the code, never on message text

**Given** an existing diagnostic code
**When** the registry changes
**Then** codes are additive only; changing a code's meaning is a breaking change

### Story 3.7: Validate a template and render it from the command line

As an integrating Go developer,
I want to reject a malformed template in CI before it reaches production, and to pin document dates reproducibly,
So that a bad template fails my pipeline rather than my customers' statements.

**Covers:** FR42 · AD-7

**Acceptance Criteria:**

**Given** a malformed `.folio` file
**When** `folio.Validate` is called
**Then** it is rejected with located diagnostics and no render is attempted

**Given** `cmd/folio`
**When** it is run
**Then** it offers a validate command and a render command

**Given** `SOURCE_DATE_EPOCH` set in the environment
**When** `cmd/folio render` runs
**Then** the CLI reads it and passes it in as a parameter
**And** the library core still reads no environment variable

**Given** no date supplied by any route
**When** the PDF is produced
**Then** `/CreationDate` and `/ModDate` are omitted

---

## Epic 4: A Go developer can render the golden report

The transaction table: rows from a bound collection, fixed columns, a header repeated on every continuation page, sum footers that never orphan, and rows that never split across a page. **This epic is the C4 gate: designer work begins only once it passes.**

### Story 4.1: Lay out a table's columns, header and cell chrome

As a template author,
I want fixed-width columns with their own alignment, a header row, borders and padding,
So that a transaction table reads as a table rather than as text in rows.

**Covers:** FR23, FR24 · AD-13

**Acceptance Criteria:**

**Given** a table with declared column widths
**When** its geometry is computed
**Then** the table's width is the sum of its column widths
**And** no independent table-width field is stored anywhere

**Given** each column
**When** it is laid out
**Then** it honours its own alignment and its declared width
**And** column widths are never negotiated against content

**Given** a table
**When** it renders
**Then** a header row, cell borders and cell padding are produced per the template's styling

### Story 4.2: Generate rows from a bound collection

As a template author,
I want one row per item in my data,
So that a statement with 400 transactions prints 400 rows without me placing any of them.

**Covers:** FR22, FR25 · AD-11

**Acceptance Criteria:**

**Given** a table bound to a collection
**When** the document renders
**Then** one row is generated per element of the collection, in data order

**Given** a cell whose content exceeds its column width
**When** the row is laid out
**Then** the content wraps within the column width and the row grows taller

**Given** each cell binding
**When** it is resolved
**Then** it resolves within the table's row scope per Story 3.1

**Given** a table bound to an empty collection
**When** the document renders
**Then** the header renders, no data rows are produced, and the render succeeds

### Story 4.3: Break a table across pages without splitting a row

As a template author,
I want a row to stay whole,
So that a transaction never appears half on one page and half on the next.

**Covers:** FR25

**Acceptance Criteria:**

**Given** a row that does not fit in the remaining content height
**When** pagination runs
**Then** the row moves whole to the next page
**And** no row is ever split across a page boundary

**Given** a table longer than one page
**When** it paginates
**Then** rows continue in data order across pages with no duplication and no omission

### Story 4.4: Repeat the table header on every continuation page

As a reader of a 50-page statement,
I want to know what each column means on page 34,
So that I do not have to page back to the start to read my own statement.

**Covers:** FR26

**Acceptance Criteria:**

**Given** a table continuing onto a further page
**When** that page renders
**Then** the table header row is repeated at the top of the table on that page

**Given** the repeated header
**When** the content height available for data rows is computed
**Then** the header's height is accounted for on every continuation page, not only the first

### Story 4.5: Render footer aggregates without orphaning them

As a template author,
I want the sum row to stay with its data,
So that a total never appears alone at the top of a page with nothing above it.

**Covers:** FR25, FR27

**Acceptance Criteria:**

**Given** a table with per-column footer aggregates
**When** it renders
**Then** sum, count and average render per the column's configuration

**Given** a footer aggregate row that does not fit in the remaining space
**When** pagination runs
**Then** it moves to the next page together with at least one preceding data row

**Given** the aggregate values
**When** they are computed
**Then** they cover the whole collection, matching Story 3.3

### Story 4.6: Handle a row taller than the page

As an integrating Go developer,
I want a pathological row to produce a diagnostic rather than an infinite loop,
So that one bad record cannot hang my statement service.

**Covers:** FR25, FR41 · AD-14

**Acceptance Criteria:**

**Given** a single row taller than a full content area
**When** pagination runs
**Then** it renders at the top of a fresh page, clipped to that page's content height
**And** a diagnostic is reported naming the row and the element id
**And** the render completes and returns PDF bytes

**Given** the same pathological row
**When** pagination runs
**Then** the algorithm terminates — no loop and no silent truncation

### Story 4.7: The golden report renders and hashes match

As a solo builder,
I want the Customer Account Statement to render correctly and reproducibly at four page counts,
So that the MVP's central claim is demonstrated rather than asserted.

**Covers:** S1, S2, S3, S4 · the C4 gate

**Acceptance Criteria:**

**Given** the golden report fixture — logo, customer block, account block, statement period, five-column transaction table with repeated header and sum footer, and a footer carrying confidentiality text, a params-supplied generated date and `Page X of Y`
**When** it renders at 1, 5, 20 and 50 pages
**Then** all four render correctly against recorded reference renders, not merely producing a file

**Given** the fixture data
**When** it is inspected
**Then** it contains Latin, Thai and CJK text in the same table, at least one row wrapping to multiple lines, and at least one embedded image
**And** the generated date arrives through `params` and is never read from a clock

**Given** the four page counts
**When** each is rendered on `darwin/arm64`, `linux/amd64`, `linux/arm64` and `js/wasm`
**Then** every hash is identical across all four targets
**And** each reference render is confirmed non-blank and page-count-correct

**Given** the Thai text in the fixture
**When** its line breaks are compared to the frozen expected-break fixture
**Then** they match exactly, and Thai marks are positioned by `GPOS`

### Story 4.8: Alternating row styling

As a template author,
I want alternating row shading,
So that a dense transaction table is easier to read across.

**Covers:** FR28 · optional, first to be cut

**Acceptance Criteria:**

**Given** a table with alternating row styling enabled
**When** it renders
**Then** alternate rows carry the configured background

**Given** a table paginating across pages
**When** alternation is applied
**Then** the alternation follows row index in the collection, so it does not reset per page

**Given** this story
**When** capacity is assessed
**Then** it is understood as optional — correct pagination takes priority over table styling wherever the two compete, and this story is cut before any of Stories 4.1 to 4.7 are compromised

---

## Epic 5: A template author can lay out a report and see the real PDF

Ploy opens Folio in a browser, opens a `.folio` from her laptop, sets up an A4 page, drops a logo and text fields into the three bands, and switches to Preview to see the exact production document — the same engine, in her tab, with nothing sent anywhere. It works with the network disconnected.

### Story 5.1: The design system and application shell

As a template author,
I want an interface that reads as a precision instrument rather than a web page,
So that the report I am building is the only thing competing for my attention.

**Covers:** UX-DR1, UX-DR2, UX-DR3, UX-DR4 · the only starter template in the project

**Acceptance Criteria:**

**Given** a new `folio-designer/` workspace
**When** it is scaffolded
**Then** it is created from the standard Vite React-TS template (`npm create vite`), React 19.2.x, Vite 7.3.x, Node 20.19+ or 22.12+
**And** it is the only starter template in the project — the Go module is initialized from scratch (Story 1.1)
**And** TypeScript strict mode is on

**Given** the designer's styling
**When** it is inspected
**Then** every colour, type role, spacing step and radius comes from the `DESIGN.md` token file
**And** no hard-coded hex value appears anywhere in the application

**Given** the token implementation
**When** it is inspected
**Then** it carries the 60 named colour tokens across chrome, rules, text, select, bind, status and page groups; the six named rgba tint washes; the 13 typography roles across the chrome and page ramps; the 10-step spacing scale; and the zero-radius shape rule

**Given** any surface
**When** accent colour is applied
**Then** `select` cyan means structure, focus and authority, and `bind` amber means data and only data
**And** selection handles remain cyan even on a bound element

**Given** the theme
**When** it renders
**Then** it is the single dark-chrome / light-canvas theme, with the page as the only bright surface
**And** contrast ratios are met on dark chrome's own terms with no light-mode fallback

**Given** the gradient rule
**When** it is applied
**Then** colour-ramp gradients are absent, while hard-stop patterns such as the page dot grid and the transparency checkerboard are permitted

### Story 5.2: The engine worker and command channel

As a template author,
I want the application to hold exactly one copy of my document,
So that what I see, what I save and what renders can never disagree.

**Covers:** AD-15, AD-16

**Acceptance Criteria:**

**Given** the designer at runtime
**When** the engine is instantiated
**Then** exactly one wasm module instance exists, in one dedicated Worker

**Given** the document
**When** ownership is inspected
**Then** the wasm engine parses, holds, mutates, validates and serializes it
**And** no TypeScript model of a `.folio` document exists anywhere in the codebase

**Given** the user interface
**When** it paints
**Then** it holds an immutable snapshot and sends every committed mutation as a command over one channel

**Given** transient interaction state — a drag in flight, a resize preview, an uncommitted property keystroke
**When** it occurs
**Then** it lives in the UI and never enters the document

**Given** any engine call
**When** it is made
**Then** it is asynchronous over a request/response channel and the UI never blocks on it

### Story 5.3: Work offline

As a template author,
I want the designer and its preview to work with the network disconnected,
So that my templates and my customers' data never depend on anyone else's servers.

**Covers:** FR36 · NFR8 · AD-19

**Acceptance Criteria:**

**Given** a completed first load
**When** the network is disconnected and the page is reloaded
**Then** the application loads, opens a template and produces a preview

**Given** the service worker
**When** it installs
**Then** it precaches the application shell, the wasm module, the Thai dictionary and the font assets under content-hashed URLs, and serves them cache-first

**Given** the assets
**When** they are served
**Then** they carry brotli compression and immutable long-lived cache headers

**Given** the application at any point after load
**When** network activity is observed
**Then** no request is made at render or preview time
**And** no template or data byte leaves the machine

### Story 5.4: An honest first-run load screen

As a first-time user,
I want to be told what is downloading and why it happens only once,
So that a nine-megabyte wait feels like an explanation rather than a hang.

**Covers:** UX-DR5

**Acceptance Criteria:**

**Given** a first visit
**When** the application loads
**Then** a load screen names what is loading and states that it happens once per browser cache lifetime

**Given** the payload
**When** it is presented
**Then** it is itemised from the generated current-release manifest: Engine, Latin font, Thai font, CJK font, and the Thai dictionary embedded in the engine (not a second request), with release-derived byte figures and an explanation of why the actually dominant CJK font payload dominates

**Given** the download in progress
**When** it is displayed
**Then** actual progress is shown; a spinner without progress is not acceptable at this payload size

**Given** this surface
**When** its density is compared to the rest of the application
**Then** it is deliberately the one non-dense surface, and it is the only place the display-size type token appears

**Given** a subsequent visit with the cache warm
**When** the application loads
**Then** the load screen does not appear

### Story 5.5: Open and save `.folio` files locally

As a template author,
I want to work on files on my own laptop with no account,
So that my templates are mine and live in my own Git repository.

**Covers:** FR8 · AD-9, AD-20 · UX-DR13, UX-DR16

**Acceptance Criteria:**

**Given** a browser implementing the File System Access API
**When** a file is opened and later saved
**Then** the file handle is held and the file is saved in place

**Given** a browser without it — Firefox or Safari, which ship only OPFS
**When** a file is opened and later saved
**Then** open falls back to `<input type="file">` and save falls back to a download
**And** the capability check happens once at startup, after which the rest of the application talks to one file-access interface and never branches again

**Given** unsaved changes
**When** the workspace is displayed
**Then** a persistent, quiet, always-visible indicator shows them
**And** no autosave exists in either tier

**Given** a file saved by the designer
**When** its bytes are compared to what the engine serializes
**Then** they are identical, because the engine performed the serialization

**Given** the application opened with no template
**When** the workspace is displayed
**Then** it shows an unnamed blank template and offers both open-a-file and start-blank
**And** the author is never left on a dead canvas with no path forward

### Story 5.6: The canvas — page, bands, grid and page setup

As a template author,
I want to see my page at its true proportions with its three bands clearly separated,
So that I always know which band I am working in.

**Covers:** FR2, FR3, FR6 · UX-DR6

**Acceptance Criteria:**

**Given** the workspace
**When** the canvas renders
**Then** the page appears at true proportions with visible page boundaries, on a dark surround

**Given** the three bands
**When** they are displayed
**Then** Page Header, Content and Page Footer are legible as distinct regions at all times

**Given** page setup
**When** it is configured
**Then** A4, Letter or a custom size may be chosen, with portrait or landscape orientation and page margins

**Given** the grid
**When** snapping is enabled
**Then** movement snaps to it, and snapping can be toggled

### Story 5.7: Place and manipulate the five components

As a template author,
I want to drop a text field or a logo onto the page and move it where I want it,
So that laying out a statement is direct manipulation rather than configuration.

**Covers:** FR1, FR4 · UX-DR10, UX-DR11, UX-DR18, UX-DR25

**Acceptance Criteria:**

**Given** the palette
**When** it is displayed
**Then** it offers exactly five components — Text, Image, Table, Line, Rectangle
**And** the set is closed and the interface does not imply extensibility

**Given** a component dragged from the palette toward a band
**When** it hovers before release
**Then** the target band highlights, so the author knows which band will receive it before committing
**And** an ambiguous drop is not possible

**Given** a placed component
**When** it is manipulated
**Then** click selects, shift-click extends, clicking empty canvas clears, drag moves with grid snapping, and handles resize
**And** resizing is constrained to the containing band

**Given** a selection handle
**When** its hit target is measured
**Then** it is larger than its visual footprint

### Story 5.8: Edit component properties

As a template author,
I want to set a component's exact position, size and typography,
So that I can be precise where dragging is only approximate.

**Covers:** FR5 · UX-DR10, UX-DR25

**Acceptance Criteria:**

**Given** a selected component
**When** the properties panel renders
**Then** it is contextual to the selection and exposes position (X/Y), size (width/height), font family, font size, bold, italic, text alignment, vertical alignment, border, padding, background and visibility

**Given** a property edit
**When** it is committed
**Then** it is sent as a command to the engine and the snapshot updates
**And** an uncommitted keystroke does not enter the document

**Given** every property field
**When** navigated by keyboard
**Then** it is reachable and operable, with visible focus using the select token

### Story 5.9: A canvas the browser never measures

As a template author,
I want the canvas to wrap text where the engine would wrap it,
So that the approximate view is approximate in scale, not wrong about where my content lands.

**Covers:** NFR2 · AD-17 · UX-DR21

**Acceptance Criteria:**

**Given** any text painted on the canvas
**When** its metrics are obtained
**Then** they come from the engine's measure API

**Given** text rendered on the canvas
**When** the DOM is inspected
**Then** it is painted as pre-broken lines with browser wrapping disabled — `white-space: pre`, no `text-wrap`, no justification

**Given** the browser
**When** its role is assessed
**Then** it contributes rasterization only, and determines no pagination, no line breaking and no measurement

**Given** Thai or CJK text on the canvas
**When** it wraps
**Then** it wraps at the same opportunities the engine would choose

**Given** the single-line text painting introduced with the palette in Story 5.7
**When** this story lands
**Then** it is superseded by engine-measured, pre-broken multi-line painting
**And** no code path remains in which the browser decides where a line ends

### Story 5.10: Preview the exact production document

As a template author,
I want to see the real PDF my developer's service will produce,
So that I am the first person to see the document rather than the last.

**Covers:** FR34, FR35 · NFR5 · AD-16, AD-18 · UX-DR9, UX-DR15, UX-DR23

**Acceptance Criteria:**

**Given** the workspace
**When** the author switches to Preview
**Then** the canvas is replaced by the rendered PDF as a full-surface mode switch, not a panel
**And** selection and scroll position are preserved where meaningful

**Given** a preview render
**When** it is invoked
**Then** it takes three inputs — the serialized `.folio` bytes from the same save path, the sample data document, and an author-supplied parameter document
**And** it never receives a live in-memory document

**Given** the previewed PDF
**When** its hash is compared to a native `folio-go` render of the same three inputs
**Then** they are identical

**Given** the preview surface
**When** it is implemented
**Then** it is a controlled `pdfjs-dist` canvas, never the browser's built-in viewer in an `iframe` or `embed`

**Given** a 50-page document rendering
**When** it is in progress
**Then** progress is indicated, the blocking is confined to the preview surface, and the canvas remains interactive

**Given** the interface as a whole
**When** it is reviewed
**Then** nothing implies server rendering, a cloud round-trip or an account
**And** the canvas-approximate / preview-exact asymmetry is legible without a tutorial

### Story 5.11: Never show a stale preview

As a template author,
I want a preview that has gone out of date to say so,
So that I never commit a template believing I have seen its output when I have not.

**Covers:** AD-18 · UX-DR14

**Acceptance Criteria:**

**Given** a rendered preview
**When** its identity is computed
**Then** it is a hash of the serialized template, the data, the parameters, the engine version and the `FontSet` identity

**Given** a committed command that changes any of those inputs
**When** the identity is recomputed
**Then** it differs and the preview is marked stale immediately

**Given** a stale preview
**When** it is displayed
**Then** it is either visibly invalidated or re-rendered
**And** it is never presented unmarked as the production artefact

### Story 5.12: Diagnostics that locate, and an interface that can be driven from the keyboard

As a template author,
I want a render complaint to point at the thing that caused it, and to be able to work without a mouse where it matters,
So that fixing a problem is one click and the tool does not exclude people who cannot drag.

**Covers:** FR41 · AD-10 · UX-DR17, UX-DR19, UX-DR20, UX-DR22, UX-DR25

**Acceptance Criteria:**

**Given** a render returning warnings — clipped content, an over-tall row, a glyph with no coverage
**When** they are presented
**Then** they surface in the preview where the consequence is visible, non-blocking and dismissible
**And** selecting one locates the offending element back on the canvas by its element id

**Given** a render that fails
**When** the error is presented
**Then** it blocks within the preview surface, names the element and the data path, and offers return to the canvas

**Given** any diagnostic or error
**When** it is displayed
**Then** it is announced, and distinguished by shape before colour

**Given** the application
**When** it is driven by keyboard
**Then** the palette, properties, binding tree and table columns are all reachable and operable
**And** every focusable element shows visible focus using the select token
**And** every icon-only control carries an accessible name

**Given** the frequent operations
**When** shortcuts are used
**Then** save, undo, redo, delete, duplicate, nudge, toggle preview and toggle snapping are covered, with hints in menus and tooltips
**And** no command palette exists

**Given** undo
**When** it is exercised
**Then** it covers every canvas and property mutation as engine-side history over committed commands
**And** loading sample data is not undoable and does not appear to be

### Story 5.13: Import an image and set it on an image component

Added after the Epic 5 boundary. Stories 5.7 and 5.8 delivered the Image palette item and the property
panel, but no story ever owned choosing the picture. The palette control has therefore always attached
one hardcoded shipped asset, added during Story 6.7 so a browser-authored golden report could contain an
embedded image at all. This story completes FR4 and FR5 for the one component that was left half-built,
and is why `epic-5` returns to `in-progress` and owes a second boundary gate.

As a template author,
I want to put my own logo on the page,
So that the statement I lay out is my organisation's document rather than Folio's.

**Covers:** FR4, FR5, FR33 · UX-DR10, UX-DR21, UX-DR24, UX-DR25

**Acceptance Criteria:**

**Given** a selected image component and an image file chosen from local disk
**When** the author sets it
**Then** one committed engine command carries the raw bytes to the engine, which computes their SHA-256, stores the asset under that key, and points the element at it
**And** the same picture chosen twice is stored once and referenced twice
**And** the browser never hashes, sniffs a format, or holds an asset model

**Given** a media type this library version cannot decode
**When** it is chosen
**Then** the command refuses it with a located diagnostic and nothing enters the document
**And** the author learns at the click, not at reopen

**Given** a selection that is exactly one image component
**When** the properties panel renders
**Then** an IMAGE section shows the current asset's media type, intrinsic pixel dimensions and abbreviated key, and offers a named control that opens the picker
**And** the section is keyboard-operable with visible focus and accessible names, and states each unavailable reason concretely

**Given** an image component whose asset is installed
**When** the canvas paints it
**Then** it draws inside the fit-and-centre rectangle the engine already computes for the PDF, supplied on the projection as engine-owned millipoints
**And** the browser computes no fit of its own, and the canvas stays explicitly approximate about text only

**Given** an asset whose last referencing element is deleted or repointed
**When** the document is serialized
**Then** the asset is collected in the same transaction, so replacing a logo ten times leaves one asset
**And** undoing back to a state that referenced it restores its bytes

**Given** the image path in its entirety
**When** it is inspected
**Then** no URL field, filesystem path, remote fetch or render-time IO exists anywhere on it
**And** the palette insert path is unchanged, so a dropped Image is still renderable before a file is picked

---

## Epic 6: A template author can bind a report to data and build the golden report

Ploy loads the sample JSON her developer gave her, walks its paths as a tree, binds each field by picking rather than typing, drops a table into the Content band, and configures its five columns as a matrix. She saves the file and commits it. Anan renders it, and the hash matches what she saw.

### Story 6.1: Load sample data and browse its paths

As a template author,
I want to see the shape of the data my developer will supply,
So that I am binding to something real rather than to a name I invented.

**Covers:** FR9 · UX-DR7, UX-DR13

**Acceptance Criteria:**

**Given** the binding panel before any data is loaded
**When** it is displayed
**Then** it states that binding is unavailable and offers the load action
**And** components can still be placed on the canvas

**Given** a sample JSON document loaded from the local machine
**When** the panel renders
**Then** its paths appear as a navigable tree, including nested objects and collections

**Given** the panel
**When** its placement is inspected
**Then** it is docked to the workspace, not a separate destination

### Story 6.2: Bind a component by picking a path

As a template author,
I want to bind a field by choosing it from the tree,
So that I never mistype a path and discover it at render time.

**Covers:** FR7 · UX-DR7

**Acceptance Criteria:**

**Given** a selected component and a path in the tree
**When** they are connected
**Then** the component becomes bound to that path and the binding is sent to the engine as a command

**Given** a bound component
**When** it is displayed on the canvas
**Then** its bound state is marked using the bind accent, while its selection handles remain the select accent

**Given** every scalar field in the golden report
**When** the template is authored
**Then** each was bound by selecting a path, with no path typed by hand

### Story 6.3: Supply parameters for preview

As a template author,
I want to provide the runtime values my developer's service will supply,
So that I can preview a statement whose generated date is real rather than blank.

**Covers:** FR21 · AD-16

**Acceptance Criteria:**

**Given** a template using `{{params.reportDate}}`
**When** the author opens the parameter document
**Then** they can supply values for the parameters the template references

**Given** a preview render
**When** it runs
**Then** the parameter document is passed as the third input alongside template bytes and sample data

**Given** a parameter referenced by the template but absent from the parameter document
**When** the preview renders
**Then** the failure is located, naming the parameter and the element id

### Story 6.4: Configure table columns as a matrix

As a template author,
I want to configure all my columns in one grid,
So that setting up five columns is one task rather than five repetitions of a form.

**Covers:** FR10 · UX-DR8, UX-DR25

**Acceptance Criteria:**

**Given** a selected Table component
**When** the table editor is invoked
**Then** it opens as a focused surface over the canvas

**Given** the editor with no columns yet
**When** it is displayed
**Then** add-column is the only meaningful action and is unmistakable

**Given** columns and their attributes
**When** they are presented
**Then** they appear as a matrix of columns against attributes — never as a repeated single-column form

**Given** the editor
**When** it is navigated by keyboard
**Then** it behaves as a data grid

**Given** each column
**When** it is configured
**Then** width, alignment and header label can all be set, and columns can be added and removed

### Story 6.5: Bind columns in row scope and configure footer aggregates

As a template author,
I want each column bound to a field of the current row, with a sum on the columns that need one,
So that my transaction table fills itself and totals itself.

**Covers:** FR10 · AD-11, AD-13

**Acceptance Criteria:**

**Given** a table bound to a collection
**When** a column's binding is configured
**Then** the fields offered are those of the row scope, expressed through the region's declared alias

**Given** a column
**When** a footer aggregate is configured
**Then** sum, count or average can be selected per column

**Given** the configured table
**When** it is previewed
**Then** rows generate from the collection and footer aggregates compute over the whole collection

### Story 6.6: Present a failed render honestly

As a template author,
I want a render failure explained in place,
So that I can fix it without leaving the tool or guessing.

**Covers:** UX-DR12, UX-DR24

**Acceptance Criteria:**

**Given** a render that fails
**When** the error card is displayed
**Then** it states the fact, names the location, and offers no comfort — matching the product's terse, technical voice
**And** it uses the danger token, which no mockup previously exercised

**Given** the error card
**When** it is presented
**Then** it names the element and the data path and offers return to the canvas

### Story 6.7: The round trip closes

As a solo builder,
I want a template authored in the browser to render byte-identically through the Go library,
So that the claim the entire product rests on is demonstrated end to end.

**Covers:** S5, S7, S8, S9

**Acceptance Criteria:**

**Given** the golden report authored in this session in the designer — not merely re-saved
**When** it is saved locally and rendered by native `folio-go` with the same data and parameters
**Then** the output is byte-identical to the preview the author saw

**Given** a template hand-edited in a text editor and never opened in the designer
**When** it is loaded and rendered
**Then** it succeeds

**Given** a second, structurally different template
**When** it is authored and rendered
**Then** it renders correctly and reproducibly

**Given** every error case in the diagnostic registry
**When** the suite runs
**Then** each produces a located, actionable message

---

## Epic 7: A template author can lay out a multi-page legal document

Ploy's firm authors contracts, not statements. A contract is mostly body copy — numbered clauses
that run for paragraphs, justified, set at the line spacing the house style fixes — followed by a
schedule and a signature block that sit on later pages. Today the canvas is one page tall and a
text element holds one run of text with no way to type a break, so neither half of that document
can be authored.

This epic adds both halves. It changes the **authoring surface and the typography**, and it
deliberately changes **nothing about pagination**: `internal/layout/paginate.go`'s four rules and
D-2.6.1's window model are inputs to this epic, not targets of it.

### Story 7.1: Break a line where the author typed a break

As a template author,
I want a line break I typed to survive into the PDF,
So that a clause can be more than one paragraph without being more than one component.

**Covers:** FR46 · AD-25

**Acceptance Criteria:**

**Given** a text value containing a line feed
**When** the element is rendered
**Then** the text following it begins on a new line, at the next baseline, regardless of how much
width remained on the line before it

**Given** a mandatory break
**When** line packing runs
**Then** it is not an optional opportunity that the packer may decline — `internal/text` reports it
as mandatory and `packLines` always takes it

**Given** two consecutive line feeds
**When** the element is rendered
**Then** an empty line is produced, occupying one full advance, so a paragraph gap is expressible

**Given** a value carrying `\r\n`
**When** it is broken
**Then** the pair is one break, never two

**Given** a mandatory break
**When** the line is measured
**Then** the break character itself is consumed and drawn on neither line, exactly as a whitespace
break already is

**Given** the entire existing fixture corpus, none of whose values contain a line feed
**When** it is re-rendered on all four targets
**Then** every hash is unchanged

### Story 7.2: Set the space between a paragraph's lines

As a template author,
I want to control the space between lines,
So that a contract matches the house style my firm files under.

**Covers:** FR48 · AD-23, AD-9

**Acceptance Criteria:**

**Given** a `lineSpacing` on an element's style
**When** the vertical model is computed
**Then** it scales `Advance` — the baseline-to-baseline span — and **nothing else**

**Given** the same `lineSpacing`
**When** the first line is placed
**Then** `FirstBaseline` is unchanged, so the element's top edge does not move and no sibling
appears to shift

**Given** `lineSpacing` is expressed
**When** the arithmetic runs
**Then** it is an integer ratio in thousandths applied in millipoints — never a binary float, which
AD-23 bans and which would break byte-identity across targets

**Given** an element with no `lineSpacing`
**When** it is rendered
**Then** the advance is byte-for-byte the value the ruled model produces today

**Given** an element whose lines are spaced wider
**When** the document paginates
**Then** the wider line extents feed `internal/layout` unchanged, so page breaks follow the spacing
rather than ignoring it

**Given** the same element on the canvas
**When** the canvas text paint plan is projected
**Then** it consumes the same advance the renderer does, so the canvas and the PDF do not disagree
— the Story 5.9 invariant holds

**Given** a `lineSpacing` outside the supported range, or one that is not a whole number of
thousandths
**When** the template is loaded
**Then** it is a located load error naming the element, never a silent clamp

### Story 7.3: Justify a paragraph's edges

As a template author,
I want body copy justified to both margins,
So that a clause reads the way a filed contract reads.

**Covers:** FR47

**Acceptance Criteria:**

**Given** `align: justify` on a text element
**When** the template is loaded
**Then** it is accepted — `justify` joins the closed align set

**Given** a justified paragraph
**When** a line other than its last is drawn
**Then** the slack is distributed across that line's break opportunities so the line meets the
declared width

**Given** the last line of a paragraph, or a line ended by a mandatory break
**When** it is drawn
**Then** it is not justified — it is set at the element's natural start edge

**Given** slack that does not divide evenly across a line's gaps
**When** it is distributed
**Then** the remainder is placed by one stated, ordered rule in integer millipoints, so all four
targets produce identical bytes

**Given** a justified line that still exceeds its declared width
**When** overflow is detected
**Then** the existing FR44 clip-and-warn behaviour applies unchanged

**Given** an element with no `align`, or with `left`, `center` or `right`
**When** it is rendered
**Then** its bytes are unchanged

### Story 7.4: Author body text in the designer

As a template author,
I want to type and paste a multi-paragraph clause into a component,
So that I can author the body of a contract without hand-editing the file.

**Covers:** FR49 · UX-DR10, UX-DR13

**Acceptance Criteria:**

**Given** a selected text component
**When** its value is edited
**Then** the editor accepts and preserves multiple lines — the single-line input at
`ComponentProperties`' content fields is no longer the only way in

**Given** a clause pasted from a word processor
**When** it lands in the editor
**Then** its paragraph breaks are preserved as mandatory breaks and its other formatting is
discarded without error

**Given** the inspector
**When** a text component is selected
**Then** line spacing and alignment — including justify — are editable there, alongside the
existing typography controls

**Given** any of these edits
**When** it is committed
**Then** it travels as an opaque command to the engine and the canvas re-projects from the engine's
answer, never from a browser measurement

### Story 7.5: The content column runs past the first page

As a template author,
I want to place a component below the foot of page one,
So that a schedule and a signature block can exist at all.

**Covers:** FR50 · AD-24

**Acceptance Criteria:**

**Given** a placement or bounds command in the **content** band with a Y beyond one page's content
height
**When** the engine validates it
**Then** it is accepted — the refusal at `component_commands.go`'s band-bounds checks no longer
treats one page as the column's limit

**Given** the same command in the **pageHeader** or **pageFooter** band
**When** it is validated
**Then** it is refused exactly as today: those bands are one page tall by definition and repeat

**Given** a coordinate that is negative, or beyond the JavaScript-safe geometry bound
**When** it is validated
**Then** it is still refused, with the message unchanged

**Given** a template whose content column is longer than one page
**When** the canvas is projected
**Then** the projection reports the page-height window and how many windows the column currently
occupies, so the designer can draw them without measuring anything itself

**Given** every existing template, none of which places anything past page one
**When** it is projected and rendered
**Then** the projection and the bytes are unchanged

### Story 7.6: The canvas draws every page the document will produce

As a template author,
I want to see my document as a run of pages,
So that I can lay out page three without imagining where page two ended.

**Covers:** FR50 · UX-DR6, UX-DR3

**Acceptance Criteria:**

**Given** a content column spanning more than one window
**When** the canvas renders
**Then** it draws one sheet per window, in order, each with the page header and page footer chrome
repeated — because the engine repeats them

**Given** the boundary between two windows
**When** it is drawn
**Then** it is marked where the engine will actually break, taken from the projection rather than
computed in the browser

**Given** a component dragged onto a later sheet
**When** the drop is committed
**Then** it is translated to a column coordinate and sent as an opaque command

**Given** the canvas
**When** a component sits on a later page
**Then** the interface states plainly that its page is a consequence of the content above it and
can change when the data does — it is a column position, not a pin to page three

**Given** a single-page template
**When** the canvas renders
**Then** it looks and behaves exactly as it does today

### Story 7.7: Keep a signature block together across a page break

As a template author,
I want a group of components to move to the next page rather than split,
So that a signature block is never severed by a page boundary.

**Covers:** FR51 · AD-14, D-4.6.2

**Acceptance Criteria:**

**Given** a set of content-band elements declared as a keep-together group
**When** the document paginates
**Then** the group is placed entirely within one window, or entirely within the next

**Given** the existing `ItemGroup` machinery, which today serves table rows alone
**When** this group is paginated
**Then** it reuses that machinery rather than adding a second grouping model to `internal/layout`

**Given** a group taller than one window in aggregate — every one of its elements fitting a window,
their union not
**When** it is paginated
**Then** the Story 4.6 exception applies unchanged — a page of its own, clipped, and recorded in
`Pagination.Clipped` as a Warning beside the PDF bytes, never a fatal error

*(Amended by Story 7.10, D-7.10.1: a group holding an element that is by itself taller than a window
is refused with a located fatal `OverflowError` naming that element. WHAT is over-tall decides the
disposition, never the fact that the element was grouped — a tag can only make an element atomic, so
it can never turn a refusal into a warning.)*

**Given** a template declaring no groups
**When** it is rendered
**Then** its bytes are unchanged

---

### Story 7.8: Refuse a justified table at load, in the author's own terms

As a template author,
I want a `.folio` file that justifies a table's cells to be refused when it loads, with an error
naming the element and the field,
So that I am not silently charged a MAJOR format bump for a setting the renderer cannot honour.

**Covers:** FR47 · AD-14, D-7.3.1, DW-29 — routed here from Story 7.4's plan gate (2026-08-30), which
judged the addition `multiple-goals` under DW-29's own escalation clause. Story 7.4 discharged the
**product** half: the inspector no longer offers `justify` for a table or a mixed selection. This
story owns the **format** half.

**The behaviour today.** A table element's `style.align: "justify"`, or its `headerStyle.align:
"justify"`, loads without error, raises the document to format **2.0** — unreadable to every 1.x
reader — and then renders **identically to `align: left`**, with no diagnostic. The author pays the
whole cost of the MAJOR and receives nothing for it.

**The reusable root cause, from the lead's own note.** D-7.3.1 split the alignment closed set **by
JSON key location** (`style`/`headerStyle` versus `columns[]`) rather than **by consumer**. Those are
different partitions: a table's `style.align` and `headerStyle.align` are read into `r.alignFallback`
(`folio-go/table_render.go:373-376`, `:440-441`) and consumed at the **same site** as
`columns[].align`, so the guardrail that was meant to make justified table cells impossible by
construction let the value in through the other door. **When splitting a closed set, partition it by
the code that consumes the value, not by where the value is written in the document.**

**Three things this story inherits, written down so they are not rediscovered:**

1. **Three shipped tests must be INVERTED, not deleted** — they pin today's acceptance and are
   correct against the contract as it was written: `folio-go/internal/template/closedsets_test.go:287-292`;
   `folio-go/internal/template/linespacing_test.go:230-237`, with its `justifyHeaderStyleDoc` const
   at `:479-503`; and `folio-go/table_render_test.go:1338`
   `TestTableCellsCascadedJustifyIsDrawnAtTheStartEdge`. A version-test fixture is deleted with them.
   *(Anchor corrected 2026-08-31 at 7.8's plan gate: the second was written against
   `folio-go/line_spacing_test.go`, which contains zero occurrences of `justify`. Two further tests
   the epic missed **rename-and-widen** rather than invert:
   `closedsets_test.go:215-275` `TestAlignSetsAreTwoSetsPinnedAgainstTheirMaps`, whose name asserts
   two sets and whose `:261-263` asserts `len(StyleAlignTokens) == len(ColumnAlignTokens)+1`; and
   `component_properties_test.go:217-249` `TestStyleAlignPropertyValidatesAgainstTheStyleSetOnly`,
   whose name becomes false.)*
2. **A TEXT element's `justify` must stay accepted.** Story 7.3 shipped it, Story 7.4 offers it in
   the inspector, and a golden fixture renders it. So the fix cannot be a blanket ban wearing a narrow
   name; it must thread the consumer's element type into `decodeStyle`, which both callers already
   have (`parse_bands.go:204` has `el.Type`; `:338` is statically `ElementTable`).
3. **It must decide whether to mint a THIRD per-field style diagnostic code.** A load error raised
   with `newLoadError` is **uncoded**, and `wasm/cmd/engine/main.go:276-281` destroys every uncoded
   load error into "The template could not be processed" — so "a located error naming the element and
   the field" cannot reach a designer author without one. `internal/diag/diag.go:249-252` explicitly
   reserves that decision: *"Before a THIRD is minted, someone must decide whether the general form is
   right or whether AD-14's closed registry accretes one entry per style field forever."* **That is a
   lead call and this story must not settle it unattended.**

**Acceptance Criteria:**

**Given** a `.folio` file setting `style.align: "justify"` on a table element, or on its
`headerStyle`
**When** it is loaded
**Then** it is refused with a located error naming the element and the field, and the document is
never raised to format 2.0 for that value

**Given** a `.folio` file setting `style.align: "justify"` on a **text** element
**When** it is loaded
**Then** it is accepted exactly as it is today, and the justified golden fixtures render
byte-identically

**Given** the alignment closed set
**When** a value is validated
**Then** it is validated against **the set its consumer accepts**, keyed on the element type, rather
than on the value's JSON key path

**Given** a designer author who opens such a file
**When** the engine reports the refusal
**Then** the reason reaches them in words — which requires the third-diagnostic-code question above
to have been answered, not worked around

---

### Story 7.9: The canvas tells the truth about the window count

As a template author,
I want the design canvas to show the same page boundaries the engine will actually take when my
document declares a keep-together group,
So that the preview I lay out against is not confidently wrong about where my pages break.

**Covers:** FR51 · AD-17, D-7.7.6, D-7.7.7, D-7.7.8, D-7.7.10 — DW-46 and DW-48, routed here by the
engineering lead at Story 7.7's close (2026-08-31). **This story gates `epic-7: done`** (D-7.7.8).

**What is wrong today.** Story 7.7 taught the render path to keep a declared group whole, and did
not teach the canvas the same thing. `addCanvasWindowCount` builds its `layout.ColumnItem`s with no
`Group` at all, so for a grouped document the canvas reports a window **count** that is too low and
window **origins** that point at the wrong column positions — and it reports the count as
**exact**, because grouping is not among the three causes `ContentWindowCountIsFloor` knows about.
A measured case: a body element, a two-member group at y 700 / y 740 and an untagged tail at y 1440
renders **3** pages while the canvas reports **2**, `IsFloor: false`, origins `[0, 740000]` where
the render's second window begins at **700000**. Untag the group and the canvas is right.

**Why this is a defect and not a shortfall — the ruling this story exists to carry.**
`keepTogetherTags` takes the Template and nothing else: no data, no params, no font set. Grouping is
a **pure template property**, so the canvas already holds every input it needs to be correct. That
is the opposite side of the line from the flag's three legitimate causes, each of which is something
the canvas genuinely cannot know. So this story **fixes the wiring**; it must **not** register a
fourth floor cause. Registering one would park a defect inside the mechanism that exists to be
honest about shortfalls, and the author has no way to avoid it — they declared a group in their file
and the canvas is simply wrong about it.

**One fix closes both halves.** A wrong origin is genuinely a different failure from a wrong count —
a floor flag is a claim about a count and says nothing about where a window begins, and there is no
flag on the origins array. But Story 7.6 **projected** origins from `pages[page].Shift` rather than
computing them, so once the canvas's items carry their groups the real `Paginate` produces the true
origins as a by-product. There is no second fix to write.

**Acceptance Criteria:**

**Given** a template declaring a keep-together group
**When** the canvas projection is built
**Then** its window count and its window origins are **equal to the render path's**, asserted
directly against a real render rather than against the flag

**Given** the same template
**When** `ContentWindowCountIsFloor` is computed
**Then** grouping adds **no** new floor cause — the flag keeps exactly its three existing causes,
and the tag refusal on tables stays asserted, since that refusal is what stops a group inheriting a
table's data dependency

**Given** a tagged element that is duplicated in the designer
**When** the copy is created
**Then** the copy carries **no** group tag and the original is unchanged — a duplicate must not
silently join a group the designer offers no way to see or clear

**Given** a template declaring no groups
**When** it is rendered and its canvas is projected
**Then** its bytes and its projected values are unchanged

**SCOPE WIDENED 2026-08-31, after this story halted on its own block condition.** Building it
surfaced two more problems at the same seam, and the engineering lead ruled both into it:

- **An element whose visibility depends on data is a genuine shortfall, and it is a CEILING.** The
  canvas places a `visibleIf` element; the render omits it, and AD-24 makes a hidden element absent
  with **no gap**. So canvas ≥ render — the opposite direction from the three shipped causes, which
  are all floors (a bound table's rows are added, not removed). **This predates the story**: an
  ungrouped `visibleIf` element has the identical problem, undisclosed since Story 7.5 shipped the
  count. Grouping is how it was found, not what caused it. Because a document can carry both a bound
  table and a `visibleIf` element and be wrong in **either** direction, `ContentWindowCountIsFloor`
  is renamed **`ContentWindowCountIsExact`** — a boolean named `IsFloor` set true on a ceiling case is
  a second confidently-wrong disclosure, which is the exact failure this epic's gate exists for.
  **The sense inverts on purpose.** `…IsApproximate` would have zero value `false`, reading *"this
  count is exact"*, so a path that forgot to set it would **claim exactness** — the original bug
  rebuilt into the field's default. `…IsExact`'s `false` reads *"do not trust this count"*, so a
  forgotten set degrades to the honest claim. A rename, not a redesign: one Go field, its TypeScript
  mirror, one UI string, no enum. Direction is dropped deliberately — it informs no decision, since
  neither a floor nor a ceiling is a safe side to act on — and that deliberateness is recorded where
  the field is declared, so a later reader does not restore a floor claim mistaking it for lost
  fidelity.
- **An unstyled non-text element is a DEFECT, not a shortfall, and is fixed rather than disclosed.**
  The render places a non-text element only if it declares a background or border; the canvas places
  every one. Both are **pure template properties**, so the canvas can apply the identical rule with
  no data. Fixing it here is not scope creep: this story is what would otherwise **propagate** the
  asymmetry into a count the flag calls exact, trading one wrong answer for another.

**Given** an element whose visibility depends on data
**When** the canvas projects the window count
**Then** the count is **not reported as exact** — and the flag's name claims no direction, because
the direction that is true for a bound table is false for this case

**Given** a non-text element declaring neither a background nor a border
**When** the canvas projects the window count
**Then** it is **not placed**, by the same predicate the render path uses, read from that one
authority rather than restated

**Limits to state.** Designer-side **authoring** of groups is deliberately out of Epic 7: FR51 asks
only that a group can be declared, and file-only authoring is in scope. That is a stated scope
boundary, not an omission; if a grouping control is wanted it belongs with the inspector work in
Epic 12 or 14 and is the owner's to schedule. If the implementation finds a grouping case the canvas
genuinely **cannot** know, it returns to the engineering lead before any fourth floor cause is
added.

---

### Story 7.10: An over-tall element is refused whether or not it is grouped

As a template author,
I want an element I have declared too tall for the page to be refused the same way whether or not it
carries a keep-together tag,
So that tagging a component into a group cannot quietly turn a hard error into a warning.

**Covers:** FR51 · AD-14, AD-22, D-4.6.2, D-2.6.1, D-7.7.9 — DW-47 and DW-50, ruled by the
engineering lead at Story 7.7's close (2026-08-31).

**This story does NOT gate `epic-7: done`. It DOES gate the `folio-go/v0.1.0` tag.** It changes
**when a render fails**: documents that render today — clipped, with a true Warning — fail fatally
afterwards. Under AD-22 that is breaking for every downstream suite, and it is free **only while
nothing is released**. It sits with Story 7.8 in the set of narrowings that must precede the tag,
because narrowing what is accepted is free exactly once.

**The collision it resolves.** Story 7.7's contract matrix contradicts itself. Row 3 says an
over-tall group is clipped and warned; row 5 says a single-member group changes nothing, and an
untagged over-tall element is a fatal `OverflowError`. Measured at 7.7's close: an over-tall rect
**tagged** renders 566 bytes with a `TABLE_ROW_CLIPPED_HEIGHT` Warning; the same rect **untagged**
is a fatal `*folio.RenderError`. A long text element behaves the same way and worse — untagged it
flows cleanly over 71,374 bytes with no diagnostic, tagged alone it is clipped to 66,636 bytes and
**loses content**.

**The ruling: the discriminator was split on the wrong axis.** It is not *is it grouped*. It is
*what is over-tall*. An over-tall **individual element** is a located `OverflowError`, fatal,
tagged or not. A group that exceeds a window **only in aggregate** — every member fitting, the sum
not — takes Story 4.6's clip-and-warn.

**The ground is D-4.6.2's own ratio: leniency follows authorship.** A table row's height is driven
by data the author cannot fix, so failing them fatally is unjust. A loose element's height is
declared by the author, determinable from the template alone, and fixable by them — which is why
D-2.6.1 made page-edge overflow a located template error in the first place. **A group tag does not
launder authorship.** And a group of one is a no-op, so it must be indistinguishable from no group;
otherwise an author escapes a fatal error through an unrelated feature, and a rule that can be
switched off by an unrelated declaration does not survive contact with a deadline.

**Acceptance Criteria:**

**Given** a keep-together group whose single tagged element's own extent exceeds the content window
**When** the document paginates
**Then** it is refused with a located fatal `OverflowError` **naming that element** — because the
author declared an atomic block that cannot fit, which is **unsatisfiable** rather than merely
degraded, and removing the tag is the author's fix
**And** this differs **deliberately** from the untagged case, where the same element's lines split
across windows and render cleanly. **The tag is what makes it unsatisfiable.**

**AC1 REPLACED 2026-08-31, at this story's plan gate.** It read *"refused with the same located
fatal `OverflowError` **the untagged element receives**"*, and that was a **proxy** — for *the
author declared this and can fix it* — not the rule. The proxy was true for the population the
ruling had in front of it (rects, images, which are fatal untagged) and **false** for the one it did
not (a multi-line text element, which untagged renders perfectly well across pages). A story must
**not** assert message-equality with the untagged case: under this rule the untagged document
renders, so there is no untagged error to be equal to.

**And the member unit is the TEMPLATE ELEMENT, not the column item.** The paginator splits a text
element into one item per shaped line and stamps the tag on every one, so read in items *every
member fits* and a one-element group is misclassified "aggregate-only" — which is how DW-50 fell
through the ruling written to fix it. **Reading the discriminator in items lets an implementation
detail decide a product behaviour.**

**Given** a two-member group whose members each fit a window but whose aggregate does not
**When** the document is rendered
**Then** Story 4.6's clip-and-warn applies — bytes, plus a Warning in `Pagination.Clipped`

**Given** those two cases
**When** they are covered
**Then** they are asserted in **one fixture carrying both arms**, because either arm alone is also
consistent with the rule being replaced and so proves nothing about the discriminator

**Given** Story 7.7's third acceptance criterion, which reads *"a group taller than one window …
never a fatal error"*
**When** this story lands
**Then** that clause is amended to read **"taller than one window in aggregate"** — the ruling makes
the text stale, and correcting it is part of the ruling rather than follow-up, so that the next
reader does not re-derive the collision from a paragraph contradicting the decision that answered it

**Given** `ARCHITECTURE-SPINE.md`'s over-tall carve-out
**When** this story lands
**Then** it gains the discriminator clause — that an individually over-tall element is fatal
regardless of tagging. (The *other* half of DW-49, widening the carve-out from "rows" to rows **and
author-declared groups**, describes HEAD and lands earlier, with Story 7.9.)

**Also worth fixing while here, if it is in reach:** the fatal message for an over-tall rect
currently calls it *"table is taller than the content window"*. It is not a table.

---

## Epic 8: A template author can choose a font, and the file carries it

Ploy picks the firm's typeface from a catalogue inside the designer, lays out the statement, and
saves. Anan renders that file on a box with no fonts installed and no network, and gets her PDF
byte for byte. The faces a document uses are declared in it, chosen in the designer, and carried in
the file — because the `.folio` is the whole contract between the two of them, and a font nobody
can install is not a choice she can make.

**Sequencing note (2026-08-31).** This epic opens with **Story 8.0**, which is a precondition
of Story 8.4 rather than a preface to the epic (see its own text). **Story 7.10** — Epic 7's
over-tall-element repair — is sequenced **immediately after 8.0 and before 8.1**, by the owner's
call: it does not gate `epic-7: done`, but it must land before the `folio-go/v0.1.0` tag, and a
position in a sequence is the only owner that has worked on this project. It is placed here, not
promised.

### Story 8.0: A stacked Thai mark reaches the page

As a template author writing in Thai,
I want a word whose base carries two stacked marks to render,
So that ordinary Thai legal prose produces a document at all.

**Covers:** FR-fonts · AD-3, AD-6, AD-21, AD-22 — DW-28, ruled HIGH by the engineering lead
2026-08-31 after the owner hit it on a real contract clause.

**Numbered 8.0 because it OPENS Epic 8** while Stories 8.1–8.6 keep the numbers they already carry
in `sprint-status.yaml` and in cross-references. It is not a preface to the epic; it is a
**precondition of it** (below).

**What is wrong.** `internal/pdf`'s `appendShapedRun` refuses any glyph carrying a non-zero
`YOffset`, because a TJ array cannot express a vertical offset. Thai puts two marks over one base
constantly — `ทั้งสิ้น`, `ครั้ง`, `ทั้งนี้`, `ตั้งแต่` — and the shipped Noto Sans Thai gives those
marks a real offset. So a large class of ordinary Thai **does not render at all**: not a diagnostic,
not a degraded page, a hard `Render` error and zero bytes. The design canvas draws the same text
correctly, because AD-5 makes the page model blind to the emission stage — the canvas is not lying,
it cannot see this.

**Severity HIGH, on a stated criterion:** *blocks a supported use case, with no workaround, for a
real user.* The shipped Thai face is the only Thai face; the document is the owner's real work; and
"avoid the character sequence" is mutilating the document, not a workaround. It is **not** grounded
on *the product lies to the author* — that framing would make AD-5 look like a defect when it is a
deliberate invariant.

**Why it is a PRECONDITION of Epic 8 and not merely urgent.** Epic 8 lets an author embed
**arbitrary faces**, whose mark positioning is arbitrary. Noto Sans Thai reaches this branch; a face
picked from a catalogue can reach it far more often and on scripts nobody tested. **Story 8.4's own
acceptance criterion** — *a template with embedded faces renders on a machine that has never seen
them* — is at risk from the first embedded face whose glyphs carry a vertical offset. Shipping 8.4
over an unfixed fail-closed branch ships a feature that can newly stop documents rendering.

**This is a gap, not a format limit.** PDF's **text-rise operator `Ts`** expresses exactly this, and
it is inside AD-6's pinned profile — AD-6's exclusion list (encryption, annotations, forms,
transparency groups, shading, ICC, tagging) does not contain the text-state operators. The refusal's
own comment concedes it: *"the alternative — splitting the run and emitting a fresh text matrix per
glyph — is not built here."* `grep` for `Ts` across `internal/pdf` returns nothing.

**The characterization already landed** (2026-08-31, `folio-go/thai_mark_stacking_test.go`), ahead of
this story and separately, at the owner's direction. It pins the message an author receives, proves
the branch reachable through `ParseTemplate` + `Render` on the shipped set, and carries a
same-script control. It is mutation-proved: neutering the refusal makes the document render 3,187
bytes with the marks silently misplaced. **So this story starts with a red-provable before-state
rather than having to build one.**

**Acceptance Criteria:**

**Given** a text element containing a glyph the shaper gives a **non-zero vertical offset**
**When** the document renders
**Then** it produces a PDF, with the mark placed at the offset the shaper computed

**PREDICATE CORRECTED 2026-08-31 at the plan gate, which measured it.** This story's original wording
said *"a Thai base carrying two stacked marks"*, and that is **over-broad**: `ที่`, `ป้ำ` and `ปั`
each stack two marks over one base and render **exit 0 today**. Noto Sans Thai resolves the
`ี`+tone case with a **GSUB lowered-form substitution** — one glyph, zero offset — and only the
`ั`+tone case with a **GPOS y-displacement** of −57. `ที่` already appears in `fixtures/shaped-text`,
in all four `statement-*` fixtures and in `justified-thai`, so an implementer building a
two-marks predicate would write the wrong test and then conclude the shipped goldens contradict this
story. **The trigger is a non-zero `YOffset`, full stop** — mark stacking is how Thai reaches it, not
what defines it.

**Given** the 21 shipped golden fixtures, in none of which any glyph carries a non-zero vertical
offset
**When** they render
**Then** all 21 digests are **byte-identical** across all four targets — the `Ts` path is entered
**only** when `YOffset != 0`, and that is asserted rather than assumed. This is the whole of the
byte-identity risk in this story.

**Given** a glyph whose positioning still cannot be expressed — after this story, one whose offset
**rounds away to a zero rise**, which is reachable only at sub-point font sizes because `fontSize`
has no positivity floor at parse
**When** it is emitted
**Then** it still refuses and emits zero bytes, and that refusal is **pinned by a test** — the
fail-closed branch narrows to `YOffset != 0 && rise == 0`, it does not disappear, and
`thai_mark_stacking_test.go`'s arms become this story's before/after, **re-pointed and never
deleted** (D-7.8.7)

**"Pinned as it is today" means PINNED BY A TEST, not byte-identical prose** — settled 2026-08-31 at
the plan gate, which raised it rather than deciding it silently. The message's reason clause, *"which
a TJ array cannot express"*, becomes **false** for the narrowed case: a TJ array is no longer why the
glyph is refused; a rise that rounded to zero is. Shipping a canonical statement that misdescribes
its own condition is exactly the failure D-8.0.1 was written to stop, so the reason clause is
**rewritten to describe the narrowed condition**. Its second half — *"would place it wrongly with no
observable difference in the output bytes, so this fails rather than degrades"* — stays **verbatim**,
because it remains true and is the sentence that explains why refusing beats degrading.

**Given** every number reaching an output byte
**When** the rise is emitted
**Then** it goes through `numbers.go`'s emitters (AD-3: no number reaches an output byte by any
other route), in `geom.Length` millipoints with no `float64` under `internal/` (AD-23)

**Given** the owner's own contract clause, which today fails
**When** this story lands
**Then** it renders, and a fixture built from it joins the corpus with a recorded digest

**Sequencing.** This story does **not** join D-7.8.3's before-the-tag set, and the reasoning should
not be lost: emitting `Ts` for glyphs that currently **refuse** changes no existing golden **by
construction**, because those documents produce no bytes today and so no fixture can contain one.
It **widens** what renders rather than narrowing it, so a downstream consumer upgrading past
`folio-go/v0.1.0` gets more documents rendering, never fewer. The tag is not the constraint here —
the product is.

---

### Story 8.1: The document's font chains become editable

As a template author,
I want to create, rename and delete the font chains my components name, and reorder the faces inside one,
So that `fontFamily` names a family I chose rather than whatever the starter file declared.

**Covers:** FR52 · AD-9, AD-15, AD-16, AD-22 — and AD-8, AD-14 bind here too.

**`Covers:` CORRECTED 2026-08-31 at this story's plan gate.** It omitted **AD-16**, whose rule —
commands are opaque, the engine owns the edit — is the **substance of AC1**, not a background
concern. AD-8 and AD-14 also bind and were unnamed.

**Acceptance Criteria:**

**Given** a loaded template
**When** a font-chain change is commanded — a chain added, renamed or deleted, or an entry added,
moved or removed
**Then** it travels as one opaque command with one history entry, the engine owns the edit, and the
designer re-projects from the engine's answer rather than writing the `fonts` map itself

**Given** a chain that a `style.fontFamily` **or a table's `headerStyle.fontFamily`** still names
**When** its deletion is commanded
**Then** it is refused with a located error naming the elements that would be left naming nothing —
never accepted with the orphaned elements left to fail at render

**Given** a rename or an add whose target name is already a declared chain
**When** it is commanded
**Then** it is refused — **a rename onto an existing key would silently destroy that chain**

**TWO REFERENCE POINTS, NOT ONE — corrected 2026-08-31 at this story's plan gate.** `fontFamily` has
**exactly two** attachment points in the model: `Element.Style` and `TableExt.HeaderStyle`. Both are
live — a table header resolves its own chain and fails at render identically — so **a rename that
walks only `style.fontFamily` dangles a table header**, and a delete that counts only `style`
bearers under-reports the orphans it is refusing to create. Every AC in this story reads on both.

**THE DUPLICATE-NAME REFUSAL BELONGS HERE, not to Story 8.2.** It was stated only in 8.2's AC4, but
8.2's panel reports what **the engine** answers; a refusal the engine does not make is a refusal the
panel cannot report. Moved to 8.1 at the same gate.

**Given** a chain rename
**When** it is committed
**Then** every element naming the old chain is updated inside that same command and history entry,
so one undo restores both the map and the elements

**Given** a command that would leave a chain with no entries
**When** it is applied
**Then** it is refused, upholding the existing rule that a chain with no entries is not a chain
`fontFamily` may name

**FR52'S "REORDER" IS ENTRY-LEVEL, AND THAT IS FR52 IN FULL — NOT A GAP. RULED 2026-08-31
(D-8.1.1); this paragraph recorded a gap and was corrected at Story 8.1's close.** FR52 reads
*"create, rename, reorder and delete the document's font chains **and their entries**"*: the verbs
distribute over both nouns, and **`reorder` has a referent only in entries**, because a chain is an
ordered list and the map is not. The reading under which every verb means something is the one that
wins, and under it **Story 8.1 delivers FR52 in full**.

Chain-*key* order is **INAPPLICABLE, not deferred**. `Fonts` is a Go map with no stored order and the
serializer sorts its keys — twice. Four authorities forbid adding an authored key order here: AD-9's
rule that object keys are sorted; `folio-format.md`'s verbatim *"`fonts` is a mapping with no
authored key order"*, a **discharged debt** from D-4.1.1; SPEC-fonts' own `format-changes.md`, whose
`fonts` Order row reads *"Unchanged"*; and D-R7.9, which puts the format change in Story 8.3. And the
absence is **load-bearing**: `folio-format.md` reasons *from* it to kill the font-default idea —
*"`fonts` is a mapping with no authored key order, so 'the first key' was never well-defined"* — so
adding an order would supply exactly the first key that sentence depends on not existing. Chain order
is also semantically inert: it reaches no byte, no lookup and no render. The only thing it could mean
is the order names appear in a panel, which the designer serves as **UI state needing no format
change at all**. Recorded as delivered rather than as partial, because **an FR fully delivered under
the correct reading and recorded as partial is a false record, and false records become precedent.**

**Given** a document whose chains are edited and then edited back
**When** it is saved
**Then** the bytes are identical to the original (AD-9), and the projection carries the chains in
the engine's own order

### Story 8.2: The chain editor sits where fonts are chosen

As a template author,
I want to see and edit the document's chains from the typography panel,
So that choosing a font and defining what a font *is* are not two different tools.

**Covers:** FR52 · AD-15, AD-16, UX-DR13, UX-DR24, UX-DR25

**`Covers:` CORRECTED 2026-08-31 at this story's plan gate**, the same omission caught on Story
8.1's line at its own gate. **AD-15** (no TypeScript document model) is the literal substance of AC2
— *"the browser never holds its own model of the `fonts` map"* — and **AD-16** (commands are opaque)
is the substance of AC1. Both were unnamed.

**AC3 IS NOT SATISFIABLE BEFORE STORY 8.3, measured at `bc671da`.** It asks that an entry naming an
embedded face *"reads as the face's family and style from the projection"*. There is no such
projection: `decodeFonts` routes every chain through a decoder requiring each element to be a JSON
**string**, `Fonts` is `map[string][]string`, no `FontRecord` type exists anywhere, and `"asset"` in
the format refers exclusively to image elements. **There is no family/style pair to read.** So this
story delivers AC3's **negative** half — the displayed text of an entry is the projected entry
**unmodified**: no parsing, no key detection, no extension stripping — which is assertable today and
is forward-compatible with 8.3. The projection validator rejects unknown entry shapes in **both**
directions, so 8.3 cannot change the entry shape without moving that validator in the same commit.

**Acceptance Criteria:**

**Given** a selected text component
**When** the family control is opened
**Then** it lists the chains the engine projects for this document, and offers an affordance that
opens the chain editor on the same panel — no separate mode, no dialog stack

**Given** the chain editor
**When** a chain or entry is changed
**Then** the change is a command to the engine, and every value shown afterwards comes from the
engine's answer — the browser never holds its own model of the `fonts` map

**Given** a chain entry
**When** it is displayed
**Then** it reads as **the projected entry, unmodified** — never as an asset key, a file name, or
anything parsed out of one: no key detection, no extension stripping, no splitting
**And** Story 8.3, which introduces the embedded-face entry shape, **must extend this to read as the
face's family and style** and must move the projection's entry-shape validator in the same commit

**AC3 AMENDED 2026-08-31 at this story's plan gate.** It promised the entry *"reads as the face's
family and style from the projection"*, and **there is no such projection to read.** Measured: chain
entries are plain JSON strings, `Fonts` is `map[string][]string`, **no font-record type exists
anywhere in the repository**, and `"asset"` in the format refers exclusively to image elements —
there is no family/style pair. So the positive half is Story 8.3's, named above as a **forward
obligation** rather than left implied. The **negative** half is delivered here and is assertable
today; and because the projection's entry-shape validator rejects unknown shapes in **both**
directions, **8.3 cannot change the entry shape without moving that validator in the same commit** —
the absence is asserted so that its arrival trips something. Leaving the AC promising the pair while
marking the story delivered would be a **false record**, which is the shape D-8.1.1 rejects.

**Given** a refused edit — an orphaning delete, an empty chain, a duplicate name
**When** the engine answers
**Then** the panel states the concrete reason in text at the control that caused it, following the
existing property-panel error, focus and accessible-name conventions

### Story 8.3: A font travels inside the template

As an integrating Go developer,
I want the faces a template uses to be inside the `.folio` file,
So that rendering it needs no font install step and no knowledge of what it needs.

**Covers:** FR53, FR56 · AD-8, AD-9, AD-14, AD-21, AD-26, NFR1

**`Covers:` CORRECTED 2026-08-31 at this story's plan gate — the THIRD consecutive story to omit an
AD its own acceptance criteria state.** **AD-8** is AC3's rule (*"never a substituted face"*);
**AD-21** is AC6's (*"its recorded digest still matches"*); AD-14 binds the located load errors. The
`Covers:` lines name FRs and NFRs reliably and ADs unreliably, which makes the omission systematic
rather than incidental — hence the standing plan-gate check: **read the ACs, list the invariants they
paraphrase, diff against `Covers:`.**

**Acceptance Criteria:**

**Given** an embedded face
**When** the document is serialized
**Then** it is an `assets` entry keyed by the lowercase hex SHA-256 of its raw bytes, `data` base64
hard-wrapped at 76 columns, a font `mediaType`, and a `font` record carrying family, style, licence
and source — the same asset mechanism images already use, with no second storage shape and no new
canonical-serialization rule

**Given** two chains naming the same face
**When** the document is saved
**Then** one copy is stored, and `assets` emission order is unchanged, so adding a font never moves
an image

**Given** a chain entry `{"asset": "<key>"}` whose key is not in `assets`
**When** the template is loaded
**Then** it is a located load error naming the chain, the entry index and the key — never a
substituted face and never a silent drop

**THE ENTRY INDEX DOES NOT EXIST YET, measured at this gate.** The array decoder collapses a chain
into **one** error located at `fonts.<name>`, with no index. So *"naming the entry index"* is not a
reporting detail this story inherits — **it is work this story must do**, threading the index through
that decoder. An AC that reads as a formatting requirement and is actually a plumbing change is the
kind that gets silently under-delivered.

**Given** an asset whose bytes do not decode as its declared font media type
**When** the template is loaded
**Then** it is a located load error naming the asset

**Given** an asset whose media type is one this build does not recognise
**When** the template is loaded
**Then** it is **preserved**, and it errors at **render** — never refused at load

**AC1 AND AC4 AMENDED 2026-08-31: THERE IS NO CLOSED SET OF FONT MEDIA TYPES, AND THERE MUST NOT
BE.** Both criteria said *"from a closed set"* / *"outside the closed set"*, and that contradicts
binding **D-1.8.1 as amended**, which forbids `mediaType` from ever being a closed set — its own note
**predicted this exact recurrence** *"later for font formats"*, and
`TestClosedSetsNeverIncludeMediaType` fatals on a media-type-shaped key. **The engineering lead had
already ruled the collision at this run's setup:** *"unrecognised media type is preserved at load and
errors at render; a recognised type whose bytes do not decode stays a load error."* The two halves
are now separate criteria because they have **different outcomes**, which is what the single
"closed set" wording concealed.

**Given** the version rule, which makes a higher MAJOR a load error rather than a best-effort render
**When** this story starts
**Then** the bump is **written into `folio-format.md` before any code lands** — it is **MAJOR, and it
joins the existing `2.0`**, adding no new rank

**AC5's PREMISE WAS ALREADY FALSE when it was written: the question is not open.** **D-R7.9 is an
owner decision naming this story and this version literally** — *"Epic 8's format change joins the
same 2.0 at Story 8.3"* — and it dissolved the only standing objection (*"no need to tag now"*).
Corroborated independently: a 1.x reader meeting an **object** where it expects a string refuses the
file, which is D-1.4.12's mechanism exactly. **So the trigger is the ENTRY SHAPE, not the presence of
a font asset**, and it maps onto the existing rank — no new constant, no renumbering. The obligation
that survives is the **writing-it-down**, not the deciding.

**Given** every template in the existing golden corpus, none of which embeds a font
**When** it is rendered after this story
**Then** its bytes are unchanged and its recorded digest still matches

### Story 8.4: The engine renders from an embedded face

As an integrating Go developer,
I want a template with embedded faces to render on a machine that has never seen them,
So that "the file is the contract" survives contact with a build box.

**Covers:** FR54, FR33 · AD-7, AD-8, AD-14, AD-17, AD-21, AD-22, D-1.8.1, NFR1, NFR1.d, NFR1.e —
and **DW-83** (AC4) and **DW-35** (AC5).

**`Covers:` CORRECTED 2026-08-31 at this story's plan gate (D-8.4.5), on two counts.**
*(a) An off-by-one that both this document and the deferral register carried.* The line previously
read "**DW-35** … because **AC4** below is DW-35 stated as an acceptance criterion". **AC4 is DW-83**
(D-8.3.5); **DW-35 is AC5**. Both are re-owned here, by **different** criteria. The epic text and the
register agreed only because one was derived from the other — when two documents agree, check whether
they are two sources or one source copied.
*(b) Four omissions, D-8.2.8(a)'s third occurrence in four stories.* **FR33** is AC2's in its own
words; **AD-7** binds AC3's subset tag and the pinned profile; **AD-17** is AC5's whole subject; and
**AD-21** is AC6's ("it carries a recorded digest like every other fixture").

**Acceptance Criteria:**

**Given** a chain that mixes an embedded face with shipped faces
**When** text is shaped
**Then** the embedded face joins the same per-rune coverage resolution as a shipped one, in declared
order, with no name-based substitution — the asset key decides, even where an embedded face and a
shipped face share a family name

**Given** a render whose supplied `FontSet` is the shipped set alone
**When** the document names an embedded face
**Then** it renders from the document's own bytes, reading no network, no host-installed font and no
path on disk (FR33)

**Given** the same document rendered on all four targets
**When** the outputs are compared
**Then** they are byte-identical, and the subset tag remains a deterministic hash of the **emitted
subset program bytes** — subsetting stays once per render inside the PDF producer, and no face is
subset at save time

**AC3 CORRECTED 2026-08-31 at this story's plan gate (D-8.4.2).** It previously said "a deterministic
hash of the **glyph set**", which is the reading **AD-7 names and rejects**: the tag is derived from
*"a hash of the embedded font program's own bytes — the value `subset.Subset()` returns, in full —
and, unlike a hash of the sorted glyph-id set alone, discriminates two pinned instances of one
variable face."* **D-1.5.8** rejected the glyph-set reading after two instances collided. Implementing
the old wording literally would have moved all 22 recorded digests. An acceptance criterion that
contradicts an invariant is not a fork to be routed — it is an error to be corrected, and it is
corrected in the **epic**, because the epic is the artifact later stories re-derive from.

**Given** a chain entry naming an asset that is **not a font** — an image, say
**When** the document is loaded, rendered, and validated
**Then** **load ACCEPTS it** (D-1.8.1 as amended: an unrecognised media type is preserved, never
refused at load); **Render ERRORS when something actually needs to draw it**, naming the chain and
the entry; **and `Validate` PREDICTS what Render would do**

**THIS AC EXISTS BECAUSE THE SHAPE IS CURRENTLY HALF-BUILT, measured at Story 8.3's close
(2026-08-31).** Load already accepts such an entry, correctly. But **nothing errors at render
either**, because `chainFaceNames` drops embedded entries before anything looks at them — so a
document naming an image as a font renders **silently wrong** rather than being refused. D-1.8.1 as
amended requires **all three** halves, and **the `Validate` half is the one most likely to be
dropped**: it is the only one with no user-visible symptom when it is missing. It is written here as
an acceptance criterion rather than left as a deferral note for exactly that reason.

**Given** the designer's canvas paint projection
**When** a component uses an embedded face
**Then** the preview **measures** with that same face through the same engine path — asserted, not
assumed — **and rasterizes with it too**: the face's bytes reach the browser through the projection
and are registered as a `FontFace` under a CSS family name **derived from the asset key**

**AC5 RULED AND WIDENED 2026-08-31 (D-8.4.1). The measurement half is already true and must be
PINNED; the paint half is this story's, because this story creates the condition.**

*Measurement.* **AD-17** draws the line in its own words — the canvas *"gets every text metric and
line break from the engine's measure API … The browser contributes rasterization only"* — and this
criterion says "measures" twice. The plan gate measured that `addCanvasTextPaint` already calls the
render path's own `fontChain` / `shapeSegments` / `chainVerticalModel`. So this half is satisfied
**structurally**. It is stated as a *verified* criterion rather than deleted, so that a later story
cannot quietly fork the path.

*Paint (DW-35).* This story does not merely **widen** DW-35 — it **creates the condition**. Before
it, an embedded face cannot render at all, so no author can reach the state; after it, the engine
measures with the embedded face while the browser has **no CSS family for it at all** and falls
through to `sans-serif`. That is the owner-reported defect fixed at `c6e4d03`, rebuilt for this
story's own headline use case. Under the rule set at Story 8.0 — **a defect a story makes reachable
is a precondition of that story** — the paint half is 8.4's.

*The design decision that blocked it, made (D-8.4.1).* **An embedded face's CSS family name is
derived from its ASSET KEY, never from `font.family`.** AD-8: *"the asset key decides, even where an
embedded face and a shipped face share a family name"*; and `font.family`/`font.style` are display
identity, *"never used to resolve or substitute a face — resolution is by asset key alone."*
Deriving the CSS family from `font.family` would let an embedded "Inter" collide with a shipped
"Inter" in the browser's font registry — precisely the hazard AD-8's sentence exists to prevent,
arriving one layer down.

*Sizing.* If the plan gate returns `multiple-goals`, the paint half splits to a **named successor
story sequenced immediately after 8.4** — not "later in Epic 8" — and 8.4's record states the canvas
limitation explicitly, so it is **disclosed rather than discovered**.

**SIZING EXERCISED 2026-09-01 (D-8.4.6): the gate returned `multiple-goals` and THE PAINT HALF IS
STORY 8.4a's.** Read this AC as its measurement half alone — *"the preview measures with that same
face through the same engine path, asserted, not assumed"*. The clause *"and rasterizes with it
too"*, and the CSS-family derivation that goes with it, are **Story 8.4a's** first two acceptance
criteria verbatim; the D-8.4.1 ruling above stays binding there and is quoted into DW-35's register
entry, so the decision travels with the work rather than staying behind in this paragraph.

**What Story 8.4 delivered against the half that remained here.** `CanvasWithTextPaint` over
`fixtures/embedded-font/` produces fragment origins, advances and line widths **identical** to the
PDF path's, pinned by `folio-go/canvas_embedded_face_test.go` and red-proved by building the
canvas's own `fontCache` without the document — the one thing the two paths do not share.
**The limitation is disclosed by a test, not a comment**:
`folio-designer/src/canvas-font-stack.test.ts` records that the browser has no family for a carried
face and names 8.4a.

**Given** a fixture that embeds a face
**When** it is added to the corpus
**Then** it carries a recorded digest like every other fixture, so a later change to font handling
cannot move these bytes silently (AD-21) — **and the fixture's document draws text the embedded face
actually covers**

**THE SECOND CLAUSE IS NOT DECORATION (D-8.4.4).** Measured at this story's plan gate:
`fixtures/embedded-font/` draws **Latin** text while carrying a **Thai** face, on purpose. A digest
over that document **cannot observe the embedded face being used at all**, so AC6 as previously
written asserted nothing and its protection was nominal. The fixture's document must change.
Re-recording that fixture's digest is deliberate and correct **within the story that owns it**, and
it is free before the tag.

### Story 8.4a: The canvas paints with the face the engine measured

As a template author,
I want the canvas to draw my embedded face, not a fallback that happens to be installed,
So that what I position on screen is what the PDF puts on the page.

**Covers:** FR54, FR34 · AD-8, AD-15, AD-17, UX-DR13 — and **DW-35**, which is this story's subject.

**SPLIT FROM STORY 8.4 ON 2026-09-01 (D-8.4.6), AND SEQUENCED IMMEDIATELY AFTER IT — not "later in
Epic 8".** Story 8.4's plan gate returned `multiple-goals` on three measured grounds: the engine half
is **separably shippable** (it discharges FR54 and 8.4's own "As an integrating Go developer"
sentence in full, while this half serves a different user on a different surface); this half carries
a mechanism with **no precedent anywhere in the designer** — no `new FontFace`, no
`document.fonts.add`, no dynamic style injection, all three shipped faces being build-time; and it
requires **re-authoring two guards that were written to forbid exactly this shape**
(the `no var(` assertion in `canvas-font-stack.test.ts`, and the one forbidding a chain entry in a
font-family position), with a third becoming false in spirit.

**LINE ANCHORS DELIBERATELY REMOVED 2026-09-01 (D-8.4.13).** This note originally cited
`:100-106` and `:123-132`. **Both were already stale when written** — that file grew 189→237 lines
during Story 8.4, putting the assertions at `:137` and `:224`. Every line anchor this run has carried
across a story boundary has rotted; they are named by **what they assert** from here on, because a
stale anchor sends a reader to a real line that says something else, which is worse than no anchor.
**Splitting on size was legal; dropping it silently was not** — Story 8.4 therefore discloses the
canvas limitation as a **test** (its Task 14), not a comment, so the gap is asserted rather than
described.

**Acceptance Criteria:**

**Given** a component whose chain resolves a rune to an embedded face
**When** the canvas paints it
**Then** the face's own bytes reach the browser through the projection and are registered as a
`FontFace`, and the fragment rule asks for that face — no fallback, no host-installed font

**Given** an embedded face and a shipped face that share a `font.family`
**When** both are registered for the canvas
**Then** they get **distinct** CSS family names, because an embedded face's family name is derived
from its **asset key** and never from `font.family` (**D-8.4.1**) — `font.family` is display
identity, never used to resolve or substitute a face, and deriving from it would let an embedded
"Inter" collide with a shipped "Inter" in the browser's font registry, which is AD-8's own hazard
one layer down

**Given** the canvas fragment rule
**When** a component's chain is projected
**Then** the family list follows the **engine's chain order**

**THE REQUIREMENT STANDS; ITS ORIGINAL JUSTIFICATION WAS FALSE AND IS REMOVED (D-8.4.13).** This
criterion previously justified itself with *"not `tokens.css`'s `--font-page` — which puts Thai first
and would hand Latin text Noto Sans Thai's Latin glyphs."* Measured at 8.4a's plan gate:
**`--font-page` never reaches canvas text at all.** It feeds three type tokens, only one of which is
used, and only on chrome; `.canvas-text-fragment` uses a **hardcoded Latin-first literal with no
`var()`**. So the named hazard is not the hazard. The **order requirement is unaffected** and is
over-satisfied by per-fragment attribution — this is the **D-8.4.2 shape**: a criterion whose demand
is right and whose stated reason is not, corrected in the epic because the epic is what later stories
re-derive from.

**Given** the two guards that currently forbid this shape
**When** they are re-authored
**Then** each is **widened, never weakened**: `canvas-font-stack.test.ts`'s tie moves from "every
family the rule asks for is declared" to "**for a carried face**, the family the rule asks for is the
one the engine measured with", and the prohibition on naming a chain entry in a font-family position
is replaced by a rule that permits **only** an asset-key-derived name

**THE TIE IS SCOPED TO THE CARRIED CASE, AND THE SCOPE IS LOAD-BEARING (D-8.4.13).** The universal
form — *"the families the rule asks for are the ones the engine measured with"* — is **false**, and
deliberately so: for a **shipped** face the rule asks `'IBM Plex Sans'` while the engine measured
`'Noto Sans'`, which `canvas-font-stack.test.ts` records as **intentionally disjoint**. **A builder
handed the universal form would find it red and weaken it — which is precisely the failure this
criterion exists to prevent.** So the scope is what makes AC4 writable at all.

**This is because DW-35 has TWO causes and this story closes ONE.** *Cause two* — a carried face has
no browser family at all — is this story. *Cause one* — shipped chains, where the engine measures
`Noto Sans Thai` while the browser asks `IBM Plex Sans` — needs the generated `@font-face` families
renamed, rippling into `tokens.css` and its contract test: **the design-system decision the register
calls above a builder's authority, and which no ruling has yet made.** D-8.4.1 settled the *embedded*
family name; it did not settle this one. **Cause one therefore survives this story and needs an owner
by RULING, not by recommendation** — routed to the engineering lead 2026-09-01.

**THIS AC EXISTS BECAUSE A GUARD RE-AUTHORED TO LET A FEATURE THROUGH IS THE CHEAPEST WAY TO LOSE
ONE.** Both guards were written deliberately, and a story whose job is to make them false is exactly
the story most likely to delete rather than widen them.

**Given** the browser never measures text (AD-17)
**When** this story is complete
**Then** every metric and line break still comes from the engine's measure API — this story changes
**rasterization only**, and Story 8.4's assertion that the measurement path is shared must still hold

**Given** `canvas-authority-contract.test.ts`
**When** it scans for prohibited `document.fonts` usage
**Then** the scan is measured to actually run — at `:145` it currently rewrites every
`document.fonts` occurrence globally **before** the prohibition scan, which makes its own rule at
`:24` dead and would let a measurement call through unnoticed (found at Story 8.4's plan gate)

### Story 8.4b: The canvas can name the face the engine measured

As a template author,
I want the canvas to draw shipped faces with the same faces the PDF uses,
So that a chain I did not embed still previews the way it prints.

**Covers:** FR34 · AD-8, AD-17, UX-DR13 — and **DW-35's cause one**, which Story 8.4a does not close.

**RULED INTO EXISTENCE 2026-09-01 (D-8.4.14), and the register's stated blocker was FALSE.**
DW-35 has **two** causes. 8.4a closes cause two (a carried face has no browser family at all). **Cause
one** is shipped chains: the engine measures `Noto Sans Thai` while the browser asks
`IBM Plex Sans Thai`, and `canvas-font-stack.test.ts` asserts those vocabularies **disjoint in both
directions**, deliberately.

The register called cause one *"a design-system decision above a builder's authority"* — **renaming
the generated `@font-face` families to Noto.** Measured at the ruling: that is **not the decision**,
and doing it would be **wrong**. IBM Plex is the UX design system's specified typeface, named
throughout `DESIGN.md`, and promised in the release **licence manifest**. Renaming would abandon a
real choice and falsify a release artifact.

**The fork dissolves instead of escalating, because DW-35 is about what the CANVAS paints with and
says nothing about chrome.** The two vocabularies never needed merging.

**Acceptance Criteria:**

**Given** the shipped faces
**When** the designer's stylesheet is generated
**Then** each is registered **additionally** under **the engine's own face name** — the same file, a
second `@font-face` per face, the family named from the `FontSet`'s own spelling

**Given** the canvas text fragment rule
**When** it asks for a family
**Then** it asks for the engine's face names, so the canvas vocabulary becomes the engine's **by
identity rather than by a mapping table** — a mapping table would be a second authority on which
browser family corresponds to which engine face, maintained in lockstep with the shipped `FontSet`,
which this codebase rejects by name everywhere else

**Given** every chrome token — `--font-sans`, `--font-mono`, `--font-page` and every `--type-*`
**When** this story is complete
**Then** **not one of them is edited.** No new binaries, no visual change to chrome, no doc
amendment. AD-17 is the ground: the browser contributes rasterization only, so it must rasterize with
the face the engine measured with — **and it cannot unless it can name that face.**

**Given** `canvas-font-stack.test.ts`'s disjointness assertion
**When** it is updated
**Then** it is **REPLACED, not weakened.** It records the old state and *should* go red. Its
successor asserts the canvas fragment stack's families **contain** the engine's face names. It must
**not** assert global disjointness, which will now be false, and it must **not** be softened to an
`arrayContaining` on the chrome half to keep it passing.

**Given** the interval between this story and Story 8.4c
**When** `IBM Plex Sans` and `Noto Sans` both resolve to the **same file**
**Then** that state is **ASSERTED, not defended by a comment** — a test pins that the two names
deliberately point at one file and **names Story 8.4c as the successor that makes them diverge**, so a
future reader who "simplifies" the apparent redundancy goes **red**

**THIS AC IS THE ANSWER TO THE OBJECTION THAT ORIGINALLY DELAYED THIS STORY (D-8.4.22).** The
sequencing ruling put 8.4c first precisely to avoid *"a state whose only defence is a comment."* That
objection was real and **cheaply answerable** — the correct response was always to **assert the
intermediate state**, not to reorder around it. Converting the objection into a guard with an anchor is
what should have been proposed in the first place.

**THE DISJOINTNESS THIS REMOVES IS THE CANVAS'S, NOT CHROME'S.** A future reader taking "the
vocabularies now meet" as licence to make chrome ask for Noto is reading it wrong — that is Story
8.4c's question, settled there, and settled the other way.

### Story 8.4c: The designer ships the typeface it specifies

As the product owner,
I want the designer to render in the typeface its design system names,
So that the tokens, the design documents and the licence manifest all describe what actually ships.

**Covers:** NFR7, AD-26 · UX-DR13

**OWNER DECISION, 2026-09-01 (D-8.4.15).** The escalation was: the product **specifies** IBM Plex
throughout `DESIGN.md`, **promises** IBM Plex in `epics.md`'s redistributed-asset licence manifest,
and **ships no IBM Plex file at all** — `find -iname '*plex*'` is empty, and three generated
`@font-face` rules give IBM Plex family names to three **Noto** files. Three options were put to the
owner: ship IBM Plex for real; adopt Noto and amend the documents; or keep the alias and record it.

**The owner chose to ship IBM Plex for real.** The engineering lead had recommended adopting Noto, on
the ground that the product has rendered in Noto its entire life — **and had named the fact that would
flip it**: whether the IBM Plex choice was real and the Noto files an unreplaced stand-in. It was. The
lead's hedge is what let this be answered in one pass, and the recommendation described the stand-in
rather than the intent.

**Acceptance Criteria — AC1 LANDS AS ITS OWN COMMIT, FIRST**

**Given** the family named `IBM Plex Mono`
**When** its `@font-face` source is read
**Then** it is an actual monospace IBM Plex face — **not `noto-sans-cjk.ttf`, a CJK sans**, which is
what it is bound to today, so `--type-mono*`, `--type-brand*`, `--type-band-tab` and
`--type-numeric-lg` render chrome in a CJK face

**THE OWNER ASKED FOR THIS "NOW, SEPARATELY", AND THE INTERPRETATION IS RECORDED AS AN
INTERPRETATION.** With "ship IBM Plex for real" chosen, the honest fix for the mono defect **is**
adding a real IBM Plex Mono binary — so "fix it separately" and "ship IBM Plex" became the same work,
differing only in scope and order. This is read as *its own commit, landing first*, rather than a
different story. **That reading is the orchestrator's, not the owner's words.**

**Given** any font file that reaches the runtime bundle
**When** the licence gate runs
**Then** it **fails** if that file carries an extension `fontExtensions` does not recognise — and this
guard lands in **the SAME FIRST COMMIT as the mono binary**, before any binary arrives by any route

**AN ALWAYS CONSTRAINT IN A SPEC IS NOT ENOUGH, AND THAT IS THE RULING (D-8.4.23).** `ResolveAssets`
walks the whole repo and hard-fails without a same-directory `LICENSE*` + `NOTICE*`, but
`fontExtensions` is `[".ttf", ".otf", ".ttc"]` — and the `@ibm/plex-*` npm packages ship `.woff2`/
`.woff` and **no `.ttf`**. The obvious procurement route would have put font binaries into the bundle
**the licence gate cannot see**, leaving it green over files it considers unlicensed. **A spec
constraint is prose: it binds one story, and the next person adding a font by a different route is not
reading this spec.** The guard closes the **class** by construction rather than this instance. **A
guard added after the thing it guards has already shipped is one that was never able to fail.**

**Given** the family-name → file binding
**When** any story changes which file sits behind a name
**Then** an assertion observes it — because **nothing in this repository can today.** Across 443 lines
of `canvas-font-stack.test.ts` **not one assertion opens an `@font-face` `src`, a filename or a byte**;
every assertion is about family **names**. **A guard that checks only names cannot observe a change of
the thing behind the name**, which is exactly how IBM Plex names over Noto bytes survived indefinitely.
This is **the same net** as the licence guard above, and it compounds: with 8.4b landing first, its
first job is to pin the deliberate two-names-one-file state, and this story's job is to **update it when
they diverge** — same guard, two stories, each story's change visible in it.

**Given** the chrome families `IBM Plex Sans` and `IBM Plex Sans Thai`
**When** the stylesheet is generated
**Then** each names an actual IBM Plex OFL binary, and **someone confirms IBM Plex has the Thai
coverage `--font-page` asks for before that binding is trusted** — this is the acceptance risk of the
chosen option and is written here rather than discovered

**Given** the release licence manifest
**When** it lists IBM Plex among redistributed assets requiring licence text and copyright lines
**Then** the statement is **true** — the binaries are present, their OFL text and NOTICE ship with
them, and `lint`'s licence check covers them under AD-26

**Given** the bundle size budget
**When** the IBM Plex binaries are added
**Then** the added weight is measured and recorded against it — the cost the owner accepted in
choosing this option, made visible rather than absorbed

**Given** Story 8.4b's second registration
**When** both stories have landed
**Then** chrome asks for **real IBM Plex** and the canvas asks for the **engine's Noto face names**,
and the two vocabularies are separate **by design rather than by accident** — which is what 8.4b was
ruled on AD-17 grounds before this decision was known

### Story 8.4e: A shipped face carries its identity to the fragment

As a template author,
I want a chain I authored to preview with the faces I chose,
So that the canvas does not quietly rasterize with a face the engine never measured.

**Covers:** FR34 · AD-8, AD-17 — and **DW-35's attribution residual**, which is what remains of cause
one after Story 8.4b.

**RULED INTO EXISTENCE 2026-09-01 (D-8.4.26).** Story 8.4b closed the **vocabulary** layer — the
canvas can now *name* the engine's faces. It **narrowed** cause one rather than closing it, and the
closer deliberately did **not** assign an owner to the residual, because that is a ruling.

**The residual, measured rather than assumed.** The fragment stack is a **fixed constant, not the
document's chain**, and a shipped-face fragment carries **no face identity on the wire**. Pairwise
cmap overlaps between the three shipped faces are **339 / 529 / 230** codepoints, and **all three
cover `A` and `5`** — so with an authored chain like `["Noto Sans Thai"]` the engine measures Latin
with Noto Sans Thai while the fixed Latin-first stack rasterizes it with **Noto Sans**. **This is not
an edge case reachable only by exotic documents.** It is the AD-17 violation 8.4b narrowed, surviving
on the shipped-face arm.

**Why this is a SMALL story.** Story 8.4a already built per-fragment face attribution for **carried**
faces — that is what makes its AC3 over-satisfied. This is **extending an existing mechanism to a
second population, not inventing one**, which is the same shape that made 8.4b small.

**Why it is NOT folded into 8.5 or 8.6.** Both are **authoring** stories — which faces an author may
pick, and picking one writing the chain. **This is rasterization fidelity.** Folding a fidelity fix
into a feature story makes it an incidental task, **which is how it becomes the first thing cut**.

**Acceptance Criteria:**

**Given** a component whose chain resolves a shipped face
**When** the canvas paints it
**Then** the fragment carries **that face's identity on the wire**, and the fragment rule asks for
**that** face — not a fixed stack that happens to start with the right one

**Given** an authored chain whose first covering entry is not the Latin-first default — `["Noto Sans
Thai"]`, say
**When** Latin text is drawn through it
**Then** the canvas rasterizes with **Noto Sans Thai**, the face the engine measured with, **not**
Noto Sans

**Given** the fixed stylesheet constant the fragment rule uses today
**When** it is replaced by per-fragment attribution
**Then** the guard asserting *"the fragment stack is a stylesheet constant with no document input"* is
**replaced, not weakened** — it records the old state and *should* go red

**Given** DW-35
**When** this story is complete
**Then** it **closes** — both causes and the residual — and the register says so

### Story 8.4g: The bundle is a function of the source, not of the tree

As a maintainer,
I want two builds of one commit to produce one bundle,
So that a byte figure means something and a stray file cannot change what ships.

**Covers:** NFR1, AD-22

**RULED AHEAD OF STORY 8.4f (D-8.5.7), and the diagnosis was MEASURED rather than reasoned.** The
question — raised at D-8.4.27(c), closed benign at D-8.4.29, reopened at D-8.4.35(a) and reproduced at
Story 8.4f's plan gate — is now **settled by a two-part discriminator run at `92cd590`:**

| probe | tree state | wasm sha256 |
|---|---|---|
| build 1 | `git status --porcelain` **empty** | `ed260565…fb8f8` |
| build 2 | **empty**, byte-compared to build 1's | **`ed260565…fb8f8` — IDENTICAL** |
| build 3 | **one stray UNTRACKED file** | `a13ab262…3b6c1` — **DIFFERS**, `vcs.modified=true` |

**Conclusion: the build IS deterministic. The variance was TREE STATE.** `go build` defaults to
`-buildvcs`, stamping `vcs.revision`, `vcs.time` and `vcs.modified` into the wasm — and Go derives
`vcs.modified` from `git status`, where **an untracked file is enough**. A ~4-byte input change
(`true`→`false`) produces an **arbitrary** Brotli delta, because Brotli output size is not continuous
in input size, which is why a one-word change moved a total by 2,203 bytes.

**`vcs.time` was REFUTED on its premise, not merely left unconfirmed** — it is the timestamp **of the
revision**, not of the build, so two builds at one commit carry the identical value however far apart
they run. That hypothesis is dead and cannot be repaired by leaning on it harder.

**The corollary is the sharp part: THIS PIPELINE WRITES UNTRACKED FILES INTO THE TREE** — halt files,
result files, spec files. **A run that writes an artifact between two measurements changes the stamp it
is measuring. The instrument perturbs the specimen.** That is why 8.4c's pair at one commit agreed and
8.4f's did not.

**NOT an AD-21 violation, and the ground matters more than the verdict.** AD-21's invariant is
byte-identity of **rendered output** across four targets — **not** byte-identity of the compiled
binary. **The wasm's bytes and the PDF's bytes are different artifacts.** And this is not an
assumption: **every golden digest held across all four targets at every story this epic, and
`TestCrossTargetByteIdentity` passed throughout** — a compiler-level nondeterminism reaching codegen
would have moved a golden. **A number was noisy; the product's premise is intact.**

**Acceptance Criteria:**

**Given** the engine wasm build
**When** it runs
**Then** it passes **`-buildvcs=false`**, so no VCS stamp enters the binary and the bundle is a
function of the **source**, not of the tree

**Given** the fix
**When** it is verified
**Then** **a clean build and a build with a stray untracked file produce the SAME wasm** — the
re-measurement is **not optional**: *a fix for a determinism defect that is not shown to have closed it
is the same category error as a guard whose red-proof was allowed to be commit ordering*

**Given** the loss of provenance from the binary
**When** the trade is recorded
**Then** it is recorded **as a deliberate trade, not a free win** — `-buildvcs=false` removes
provenance from the artifact. Acceptable here because the release manifest already carries `releaseId`
and `pageId` derived from asset hashes, and AD-22 pins the toolchain — **but stated, not assumed**

**Given** every byte figure this epic records afterwards
**When** it is quoted
**Then** it is trustworthy — which is why this lands **before** Story 8.4f, 8.5's per-asset Brotli, and
the metric 8.4d consumes. **Every story that records a byte figure before this lands adds another
figure to the pile 8.4d must dispose of**, and there are already four.

### Story 8.4f: A bound nobody can cross silently

As the product owner,
I want a bound that is enforced rather than merely declared,
So that crossing it breaks a build instead of quietly emptying the first screen a user sees.

**Covers:** NFR7

**RULED AHEAD OF STORY 8.5 AND SEPARABLE FROM IT (D-8.5.1).** `src/release-payload.ts` declares
**`maximumCacheAssets = 64`** against a measured `assetCount` of **23** — **41 further assets**. Past
it, **`parseS1Payload` returns `undefined`** and the first-run load screen **silently loses its
payload**. The read is wrapped in `try { … } catch { return undefined }`, so **the failure is soft
twice over**. **No build script catches it; runtime "catches" it by being quiet.**

**This is a defect INDEPENDENT of the catalogue** — any future asset addition crosses it the same way
— which is exactly why it must not wait on a story about fonts. It is also **the run's signature shape
sitting in the first thing a user sees**: an all-clear indistinguishable from a couldn't-look.

**It outranked the catalogue's contents, but not for the reason either the orchestrator or the lead
first gave — it outranks because ITS FAILURE IS SILENT, not because the count is close.**

**Acceptance Criteria:**

**Given** a release whose `assetCount` exceeds `maximumCacheAssets`
**When** the offline release is verified
**Then** **the build FAILS**, naming the count and the bound — it does not return `undefined`, and it
does not reach a user as an empty load screen

**Given** the verification job
**When** the assertion is added
**Then** it lands in `verify-offline-release.mjs`, which already validates this payload and already
carries red-proofs (`s1-total-mismatch`, `s1-delivery-fiction`) — **so the assertion and its red-proof
both have a pattern to follow**

**Given** the new assertion
**When** it is proved
**Then** it is **red-proved**: a release manufactured over the bound must fail it. **A bound whose only
enforcement is a silent parse failure is not a bound**, and an assertion that has never failed has not
been shown to discriminate

**Given** the soft `catch` that returns `undefined`
**When** this story is complete
**Then** the swallow is either removed or made to distinguish *"malformed"* from *"over the bound"* —
**two causes must not share one silent outcome**

### Story 8.5: A curated catalogue ships with the designer

As a template author,
I want a list of fonts I can legally use and actually reach,
So that choosing one is a search rather than a hunt for a licence and a file.

**Covers:** FR55, FR33 · NFR1, NFR7, AD-7, AD-26

**`Covers:` SWEPT 2026-08-31 (D-8.4.5), before this story is planned rather than at its gate.**
**AD-7** — *"the core never reads the world for it"* — is AC1's ground for deriving the catalogue
ahead of the build and AC3's for the offline pick; **FR33** is AC3 in its own words (no request
leaves the machine); **NFR1** is what AC1's *"would make the PDF a function of the build
environment"* is protecting. Three of the four preceding stories omitted a `Covers:` reference the
gate then had to add, so the omission is a **rate**, not three incidents, and a rate is worth one
batch rather than two more gate cycles.

**Acceptance Criteria:**

**Given** each catalogue face
**When** it is prepared
**Then** it is a **static, single-instance** face whose assurance is stated **per procurement route** —
**reproduction** (replayable derivation, committed output, both hashes) for a face **this project
derives**, or **provenance** (pinned version + NOTICE) for a **vendored static** face — and **never
generated at build time**, which would make the PDF a function of the build environment

**AC1's ORIGINAL PREMISE WAS FALSE AND IS REWRITTEN (D-8.5.4).** It required *"the same replayable
derivation the shipped set uses"* for every face. **That derivation is not executable for ANY new
family:** `tools/fontgen/instance_faces.py` drives a **hardcoded 3-entry `UPSTREAM` list**,
`.font-sources/` is **gitignored with zero tracked files**, **nothing in the repo fetches an upstream
source**, and the bootstrap is **self-contradictory on first run** — `UPSTREAM` demands an
`out_sha256` unknowable until after it has run once.

**But it does not have to be, and the precedent is three stories old.** Story 8.4c shipped three IBM
Plex faces as **pinned static files** on a **provenance** guarantee, under the owner's own decision, so
this project already ships faces on **both** standards. **Ruled: reproduction is required only of a
face this project DERIVES; provenance is sufficient for a vendored static face.** What must not happen
is **an acceptance criterion claiming a derivation that does not exist** — that would be the worst
instance yet of the failure this run keeps finding, because it would be written into the acceptance
itself. **If 8.5 derives even one new face, the bootstrap work is IN SCOPE; if it vendors all of them,
the bootstrap gap stays open with its own owner.**

**Regardless of route:** `fontgen_matrix_test.go`'s hardcoded `"derived and compared 3 of 3 faces"`
becomes **derived from `len(UPSTREAM)`**, so adding a face fails on a **byte divergence or not at
all — never on a string**. **Third hardcoded count of this epic** (after `declared`'s floor with no
ceiling, and the probe list that has missed a new site twice): **a count written next to the thing it
counts is a literal that stops being true the moment the thing grows.**

**Given** a font offered for the catalogue
**When** its licence is evaluated
**Then** it must be **OFL-1.1, Apache-2.0, MIT or UFL** — **and the build FAILS otherwise, never
warns**; a licence text the classifier **cannot identify also FAILS**

**OWNER DECISION, 2026-09-01 (D-8.5.3), AND IT CORRECTS A CLAIM THIS RUN REPEATEDLY OVERSTATED.**
`ResolveAssets` requires a `LICENSE*`, a `NOTICE*` and a `Copyright` line beside every committed font,
then records the classified licence **as a LABEL** with `"SEE NOTICE"` as fallback, **and returns
nil**. **A GPL font gets a manifest row and a clean build.** That is **faithful to AD-26**, whose Rule
has two clauses: a **family ban scoped to "No dependency"**, and for redistributed **assets** only that
they *"travel with their licence text and copyright lines."*

**Every artifact in this run that called `ResolveAssets` "the licence boundary" overstated it** — Story
8.4c's guard and its manifest work were both about whether the gate can **SEE** a font; **neither was
about whether the licence is acceptable.**

**Why this was the owner's call and not engineering's: fonts do not link.** AD-26's stated Prevents is
about **static linking attaching obligations to Folio's binary** — but Folio **embeds and subsets a
font program into the USER'S PDF**, so an asset's licence attaches to **documents the users produce**,
not to Folio. That is a decision about what obligations *their* users inherit.

**The unresolvable case was settled separately and does not depend on the allowlist (D-8.5.2).**
D-1.3.8's ground — *"a silent pass on an unidentifiable licence is the realistic failure mode"* — is
**licence-family-neutral**, so it transfers to assets unchanged. **`"SEE NOTICE"` stops being a
pass.**

**Given** the offline release
**When** it is built and verified
**Then** the catalogue and its faces are inside the bundle behind the same verified asset URLs as
every other release asset, and the offline verification job covers them

**Given** the designer's own sources
**When** they are scanned for a forbidden font host
**Then** **no `fonts.google.com`, `fonts.gstatic.com` or `googleapis` reference appears in the scanned
population** — proved by an in-repo scan that **strips comments** and is **red-proved in both
directions**

**AC3 IS PHRASED AGAINST WHAT THE INSTRUMENT CAN OBSERVE, AND MUST NOT CLAIM MORE (D-8.5.5).** It
previously said *"the flow completes with no request leaving the machine."* Measured: **there is no
forbidden-host scan anywhere** — zero hits for `gstatic|googleapis|fonts.google` in any scanned source
population. The only literal *"no request leaves"* proof is **Playwright**
(`e2e/engine-worker.spec.ts` calls `context.setOffline(true)`), and **CI contains zero occurrences of
`playwright`**.

**A source scan proves no forbidden host APPEARS IN THE SCANNED POPULATION. It does not prove no
request leaves.** Those are different claims and this criterion makes the **weaker** one — writing the
stronger claim over the weaker instrument is the exact failure this run keeps finding. **The scan must
strip comments**, or a host parked in a comment leaves it green: that is Story 8.4b's measured lesson,
not a hypothetical. Model: `canvas-authority-contract.test.ts`'s comment-stripping character scanner.

**The literal offline proof stays at the EPIC 8 GATE**, as the executed browser assertion already owed
under D-8.4.25(d). **DW-101 and DW-103 are load-bearing for the EPIC, not blocking for this story.**

**Given** the catalogue
**When** its weight is recorded
**Then** it is **per-asset BROTLI bytes**, computed here and **consumed by Story 8.4d** rather than
built twice — with the exact invocation, commit and tree state recorded alongside every figure

**THE OBVIOUS PLACE TO RECORD IT WOULD BE A FALSE ZERO (D-8.5.6).** `s1.cacheAssets[]` already carries
`{assetUrl, bytes}` per asset — **uncompressed**. Per-asset Brotli exists **only as `.br` files on
disk and is recorded nowhere**. And `s1VisibleBytes` sums **four hardcoded filename needles** and
**already misses 174,949 Brotli bytes of IBM Plex** — so recording the catalogue's weight against it
would produce **a number that reads as "this cost nothing."**

Computing it here is **nearly free**: `generate-offline-release.mjs` already writes a `.br` per
**immutable** asset. **Mind that exception explicitly** — non-immutable assets have no `.br` and need a
**stated treatment rather than a silent skip.** **Do NOT set a threshold; 8.4d owns that.**

**If this story takes the fetch-at-pick route, AC4's subject becomes "the catalogue adds ZERO
first-load bytes" — and that is then a CLAIM THAT MUST BE ASSERTED, not a happy consequence, precisely
because a true zero and a false zero look identical in a report.**

**Given** the 64-asset ceiling and the first-load budget
**When** the catalogue's delivery shape is chosen
**Then** the story **designs** it rather than discovering it — **and the ceiling's silent-failure half
is NOT this story's** (Story 8.4f lands it first)

**THE OWNER CHOSE 20+ FAMILIES WITH BOTH CONSTRAINTS STATED IN THE QUESTION (D-8.5.3).** The honest
reading is **not** that they discounted them, but that **they want the catalogue big and expect the
engineering to find a shape where size is not paid for at first load.**

**The fork, with its option set corrected by reading the contract rather than the summary
(D-8.5.1):** the **64 bound** and the **5-row pinning** constrain **two different things**.
`release-payload.ts` bounds **`assetCount`** — *every asset in the release*. `verify-offline-release.mjs`
pins the **rows** to exactly five ids by ordered exact equality. **So "no S1 row for a catalogue face"
is NOT an option — it is already forced** — **and it does not help with the ceiling, because the
ceiling counts ASSETS, not rows.** Any catalogue face shipped as a build asset counts toward 64 whether
or not it is displayed. **The real third option is therefore much larger than it looked: "catalogue
faces are not build assets at all."**

**The lead's steer:** this epic has already built the machinery for that — **8.3 made a face travel
inside a document, 8.4 made it draw, and 8.6 is "picking a family puts it in the file."** A face
**fetched at pick time and embedded as a carried asset** costs **zero build assets, zero S1 rows and
zero first-load bytes**, and is the arc the epic is already on. **Two things must then be established
rather than assumed:** what a picked family does **offline** (NFR7 promises the *designer* works
offline; a catalogue is a **palette, not coverage** — the three shipped Noto faces are the coverage —
**but it must be stated and tested, not inferred**); and that a pick-time fetch **does not violate
AC3**, which is a real tension to **resolve explicitly rather than discover**.

**If the bundled route is taken instead, raising `maximumCacheAssets` is legitimate and not a
compromise** — read in context it sits beside `minimumCacheAssets = 10` and `assetUrl.length <= 256`
inside a validator for JSON parsed out of `index.html`, so it is a **defensive shape bound, not a
semantic capacity limit**. **Two conditions:** raise it with **stated headroom** rather than tuning it
to fit the new count, and **only after Story 8.4f lands**, so the bound can never again be crossed
silently. **A bound tuned to the current population is one the code can move.**

**Given** a CJK family
**When** the catalogue is assembled
**Then** it is excluded in this epic — a full SC face is 10.6 MB against 646 KB and 47 KB for the
shipped Latin and Thai faces — and the shipped SC face remains the coverage fallback

### Story 8.6: Picking a family puts it in the file

As a template author,
I want picking a font to be all I do,
So that the face, the chain and the fallback are already right when I save.

**Covers:** FR55, FR53 · AD-8, AD-15, AD-16, UX-DR13, UX-DR20 — and **DW-80**

**`Covers:` SWEPT 2026-08-31 (D-8.4.5), same pass as Story 8.5.** **AD-16** binds AC1 (*"one command
… as one history entry, and one undo removes both"*); **AD-8** binds AC3, since a proposed chain tail
for uncovered scripts is chain identity, not a convenience; and **DW-80** — font assets invisible to
`assetKeyReferenced` — is named here because **AC5 is DW-80's fix stated as an acceptance
criterion**, and this story has been its owner by ruling since Story 8.3's close. Naming it in
`Covers:` is what stops it depending on a reader noticing the connection.

**Acceptance Criteria:**

**Given** a catalogue pick
**When** it is committed
**Then** one command embeds the face and declares a chain naming it, as one history entry, and one
undo removes both

**Given** a family already embedded in this document
**When** it is picked again
**Then** no second copy is stored — the content hash decides — and the existing chain is offered
rather than a duplicate declared

**Given** a picked face that does not cover every script the document may render
**When** the chain is proposed
**Then** its tail is the shipped faces for the uncovered scripts, and the author can edit that tail
in the chain editor

**Given** the family control
**When** the catalogue is shown
**Then** entries the document already declares and entries it does not are visibly distinct — one is
in the file, the other is not yet — and picking is what moves an entry from the second group to the
first

**Given** a font asset no chain names any longer
**When** the document is saved
**Then** it is dropped, so a file cannot accumulate megabytes of faces nothing draws with

### Story 8.4d: The size budget is a number something checks

As the product owner,
I want the first-load budget enforced rather than described,
So that the bundle cannot drift past it again without anyone noticing.

**Covers:** NFR7

**OWNER DECISION 2026-09-01 (D-8.4.24).** `epics.md` accepts *"~9 MB first load"* — itemised as engine
and font stack ~1.5 MB, CJK face ~7.4 MB, Thai dictionary ~0.1 MB. The committed release manifest
measures **12,372,693 Brotli bytes: 37% over, since before Epic 8, enforced by nothing.** No test reads
the figure; no gate compares against it. Story 8.4c adds **+490,280 bytes (+4.34%)**.

Three options were put to the owner: hold ~9 MB and get under it; **pick a figure and make that
enforceable**; or record the overage with a trigger and no gate. **The owner chose the second**, which
was also the engineering lead's recommendation — *"it stops the drift immediately, which is the failure
that actually occurred here, and it does not pretend a 7.4 MB CJK face fits in 9 MB."*

**THE EPIC MUST NOT BE REWRITTEN TO 12.4 MB, and that is a ruling, not a preference (D-8.4.24).**
Rewriting *"~9 MB"* to *"~12.4 MB"* is **moving the threshold to match the measurement** — the twin of
manufacturing sample data to meet a floor — and it **enshrines the overage as the target while looking
like a documentation fix**. **A budget rewritten to whatever the build currently weighs is not a
budget.** The new figure is set **deliberately**, and the epic records that it was chosen rather than
observed.

**WHY THIS IS A SUCCESSOR TO 8.4C RATHER THAN PART OF IT.** The enforceable figure is *today's
measurement plus 8.4c's addition and nothing more* — so it **cannot be written until 8.4c has landed**.
8.4c's own obligation stays what the lead ruled: **record the added weight, do not fix the budget.**

**Acceptance Criteria:**

**Given** the release manifest
**When** the build runs
**Then** a gate compares the measured first-load bytes against a **declared** figure and **fails** when
it is exceeded — the number lives in one place, and that place is read by the check rather than by a
human

**Given** the declared figure
**When** it is set
**Then** it is **today's measurement plus Story 8.4c's addition and nothing more**, and the artifact
that declares it records that it was **chosen deliberately after a 37% overage went unnoticed** — not
derived from whatever the build happened to weigh

**Given** a future change that grows the bundle past the figure
**When** it is proposed
**Then** the gate reds, and raising the figure is a **visible, deliberate edit** rather than a silent
drift — which is the entire failure this story exists to prevent

**Given** the ~9 MB itemisation in the epic
**When** the new figure is declared
**Then** the old one is **superseded in place with its history**, so a later reader sees that ~9 MB was
the original commitment and what replaced it — **not a document that has always said the current
number**

**RESEQUENCED TO LAST IN EPIC 8, 2026-09-01 (D-8.4.27d) — a correction to the sequencing that created
this story.** An enforceable threshold set **before** the stories that add to the bundle have landed
either **flaps** or gets **padded with an arbitrary allowance** — and padding it is the same move as
rewriting *"~9 MB"* to *"~12.4 MB"*, which D-8.4.24 forbade. **Story 8.5 in particular ships a curated
font catalogue and may add binaries.** The threshold is set **once, against the epic's finished
weight**. The DW-100 investigation may begin earlier; **the threshold is written last.**

**DW-100 IS THIS STORY'S FIRST AND GATING TASK, NOT A PREREQUISITE OUTSIDE IT (D-8.4.27a).** Three
reads of `s1VisibleBytes` gave **three different figures** — 12,423,974 recorded in a spec, 12,426,422
by a build, and 12,423,049 measured twice by a closer at baseline *and* HEAD. **No threshold may be
written until the number is trustworthy**, but making that a prerequisite *outside* the story leaves it
homeless in exactly the way DW-35's residual just was.

**"REPRODUCIBLE" IS NOT THE BAR — "EXPLAINED" IS (D-8.4.27b).** The temptation is to adopt the figure
that repeated twice and move on. **Do not.** The two other recorded figures came from somewhere, and
picking today's repeating number without knowing why the others differed produces **a gate whose
all-clear is indistinguishable from a couldn't-look.** Require the disposition of **each** prior
figure. And **record the exact invocation alongside every figure from now on — a number without its
command is not a measurement.**

**ESTABLISH WHICH THING VARIES FIRST, BECAUSE ONE ANSWER IS FAR WORSE (D-8.4.27c).** Either **the
measurement procedure varies** (benign — fix the procedure) or **the build output varies at one
commit**, which is a **determinism defect, and this product's entire premise is byte-identity**. The
12,423,167 read is attributed to an unclean `dist`, which supports the benign reading — **but it must
be SHOWN, not assumed. If the build turns out to be nondeterministic, that finding outranks the budget
gate entirely and goes straight back to the engineering lead.**

## Epic 9: A component's box prints

Ploy sets a border and a fill on a heading, drops a rule under it and a tinted panel behind the
totals, previews, and sees exactly what she drew. Today she sees none of it: `style.background` and
`style.border` are parsed, validated, round-tripped — and then consumed by nothing outside a table's
cell chrome, so on a text, image, rect or line element they are inert. Line and Rectangle are worse
than inert: they are two of FR4's five palette components and they never reach the page model at
all, so a rectangle prints nothing whatever it is styled with. The designer offers all four controls
on all five components, which makes the panel a promise the engine does not keep.

Closing the gap uncovered a second defect underneath it, fixed by Story 9.1: `appendEdge`
(`internal/pdf/rectdoc.go`) had emitted a stroked edge's operands BEFORE its operator since Story
4.1 — `m x1 y1 l x2 y2` in a format that is postfix — so every pair of numbers bound one operator
late and a four-edge border drew as diagonals between opposite corners. The existing byte-level
tests count how many subpaths an edge set produces, never what they say, so both spellings passed;
no golden moved when it was corrected, because no document in the corpus renders a stroked edge.

This epic closes that gap and nothing else. It adds no field, no property and no format version:
every value it starts consuming is one the format already carries, already validates and already
round-trips. A document whose elements declare no `background` and no `border` must hash identically,
which is what keeps the whole golden corpus a witness rather than a casualty.

### Story 9.1: The engine paints a component's background and border

As a template author,
I want the background and border I set on a component to appear in the PDF,
So that the box I drew in the designer is the box that prints.

**Covers:** FR4, FR5 · AD-5, AD-21, AD-24

**Acceptance Criteria:**

**Given** a text, image, rect or line element declaring `style.background`
**When** the document renders
**Then** its declared box is filled with that colour, beneath that element's own text or picture, in
every band

**Given** an element declaring `style.border`
**When** the document renders
**Then** its declared box is stroked at the border's width and colour, on the edges `border.edges`
names — the same width default, colour default and edge set a table cell already resolves, through
the same builder, never a second implementation of the same rule

**Given** an element carrying a box in the page header or page footer
**When** the document paginates
**Then** the box repeats on every page, exactly as that band's text already does

**Given** an element carrying a box in the content band
**When** the content column paginates
**Then** the box travels with the column like any other content item, and is clipped and shifted by
the same rules that already govern a table's chrome

**Given** an element whose visibility condition is false
**When** the document renders
**Then** it contributes no box at all — absent from the page model, leaving no gap (AD-24)

**Given** a table element
**When** it declares `style.background` or `style.border`
**Then** nothing changes: its style keeps painting as the cell chrome Epic 4 already draws, and is
never painted a second time as an element box

**Given** an element whose declared rectangle has no area — a zero or negative width or height,
which the loader accepts
**When** the document renders
**Then** no box is drawn — there is no rectangle to draw — and the render is otherwise unaffected

**Given** a stroked edge
**When** it is emitted into the content stream
**Then** its operands precede its operator, as every other operator this module emits already does,
and a test reads the ORDER rather than counting the subpaths

**Given** the entire existing golden corpus, none of which declares an element background or border
**When** it is rendered on every target
**Then** the bytes are identical to before this story (AD-21)

### Story 9.2: Line and Rectangle draw, in the designer and in the PDF

As a template author,
I want the Line and Rectangle I place to be visible on the canvas and in the PDF,
So that the palette's five components are five components, not three plus two that print nothing.

**Covers:** FR1, FR4, FR5 · UX-DR10, UX-DR25

**Acceptance Criteria:**

**Given** a rect element carrying a background or a border
**When** the document renders
**Then** it prints as that box — the same box Story 9.1 paints for every other kind, with no
rect-specific rule

**Given** a line element
**When** the document renders
**Then** it prints as a filled bar of its declared box, so its declared height is its thickness

**Given** a Line or Rectangle placed from the palette
**When** it is dropped
**Then** it starts visible — the engine gives it a fill it can be seen by — rather than as an
invisible element the author must style before anything appears

**Given** a component carrying a background or border
**When** the canvas draws it
**Then** the canvas paints the same fill, the same border width and colour, and the same edge set the
engine will paint, from the engine's own projection — never from a browser-side model of style

**Given** a component whose box the engine refuses
**When** the value is committed
**Then** the canvas keeps showing the engine's last accepted box, exactly as every other property
already behaves

## Epic 10: A component prints in the colour the author chose

The inspector offers a font, a size, bold, italic and two alignments — and no way to say what colour
the words are. That is not a missing control: `style` has no colour for text at all. `background` is
the box behind the glyphs; nothing in the format, the page model or the PDF text emitter has ever
carried the ink the glyphs themselves are painted in, so every document this engine has produced is
black on whatever the box is.

This epic adds one optional field, `style.color`, and carries it end to end. It is the format's
third colour and it behaves like the other two: `#RRGGBB`, no colour-by-data, validated at render
through the module's one hex parser. It is absent by default and emits no colour operator when
absent, so every document written before it renders byte-identically — the same condition Epics 7
and 8 hold themselves to.

### Story 10.1: Text prints in a declared colour

As a template author,
I want to set the colour of a component's text,
So that a heading, a total or a warning reads as what it is rather than as more black text.

**Covers:** FR5 · AD-5, AD-21, AD-24

**Acceptance Criteria:**

**Given** a text element declaring `style.color`
**When** the document renders
**Then** every run that element produces is painted in that colour — every line and every face
segment, because the colour is a property of the element and not of what was drawn

**Given** a table declaring `style.color`, and optionally `headerStyle.color`
**When** the document renders
**Then** its cells take the ink through the same cascade every other cell property already uses, and
`headerStyle.color` wins for the header row alone

**Given** an element declaring no colour
**When** the document renders
**Then** no colour operator is emitted at all and the text takes the PDF's own initial fill, so the
whole existing corpus hashes identically (AD-21)

**Given** a coloured run
**When** it is emitted into the content stream
**Then** its colour is bracketed in `q`/`Q`, so it cannot leak into whatever draws next

**Given** a `style.color` that is not `#RRGGBB`
**When** the document renders
**Then** it is a located render error naming the element and the field — the same treatment, through
the same parser, that `style.background` already gets

**Given** a `{{ }}` placeholder inside `style.color`
**When** the template loads
**Then** it is a load error, because conditional formatting is out of scope for every style field
and the fence is derived from the schema rather than hand-listed

**Given** a selected text or table component
**When** the inspector is shown
**Then** TYPOGRAPHY offers the colour beside the family and the size — it colours the type, not the
box — through the same picker-and-hex control the box colours already use, and the canvas paints the
text in the engine's own projected colour

## Epic 11: Bold and italic mean what they say

Ploy bolds a heading. The B lights up, the canvas thickens the strokes, the file records
`"bold": true`, she previews — and the heading prints in book weight. Nothing warned her. Two
documents differing only by `"bold": true, "italic": true` render to the same SHA-256; the toggle is
the most prominent inert control in the product.

The property is stored and projected and consumed by no producer. `Style.Bold` and `Style.Italic`
have exactly three readers in the whole engine — `page_setup.go`'s canvas projection,
`component_commands.go`'s edit command, and `serialize.go` — and `render.go` is not among them,
because there is nothing for it to resolve to: `folio-go/fonts/` embeds three faces and all three
are Regular. The canvas is the sharper half of the defect. Story 5.9's contract is that the browser
never measures; with bold on, the browser fakes the weight with `font-weight: 700` while every
advance, break and baseline still comes from the Regular metrics, so the canvas is wrong about the
shape *and* about the wrapping, and the author's first evidence of either is the printed page.

**This epic needs an owner ruling before 11.1 starts, and it is the ruling SPEC-fonts already
records as open** ("Do bold and italic get realized in this scope? ... this is the work that could
give them meaning, or they stay explicitly out and the panel says so"). Two branches, and only the
first is specified below:

- **Realize them.** Ship weighted and sloped instances of the three families, resolve a face from
  the declared weight and slope, and the toggles become true. Cost: three families × three new
  instances, embedded and subsetted, against the ~9 MB first-load budget the SPEC already treats as
  a considered price.
- **Retire them.** Remove the toggles, stop painting them on the canvas, and make the format field
  a documented no-op or a load error. Cheaper, honest, and it makes a bold heading impossible in a
  product whose fixture is a bank statement.

Synthetic bold and oblique are not a third branch. SPEC-fonts forbids them by name — *"A weight is a
face or it does not exist"* — and faux-bolding at emit time would put a fabricated outline inside
the byte-identity regime.

Every face this epic adds is named by a chain that did not exist before it, so a document declaring
no bold and no italic must hash identically, and the whole golden corpus stays a witness.

**FRs covered:** FR57
**Also lands:** AD-21, AD-22 and AD-26 upheld unchanged — byte identity, byte-stable subsetting and
licence provenance are constraints on this epic, not targets of it

### Story 11.1: The shipped families gain a weighted and a sloped face

As a Go developer embedding the library,
I want the shipped font set to carry bold and italic faces,
So that a template that asks for bold has something to be rendered in.

**Covers:** FR57 · AD-22, AD-26
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Load.dc.html`
  — the payload manifest rows the load screen itemises, which this story adds faces to

**Acceptance Criteria:**

**Given** the `make fonts` derivation, which today instances one Regular per family from the
upstream variable builds
**When** it is extended to the new instances
**Then** each is derived by the same replayable script, committed as output rather than generated at
build time, and `make fonts-verify` reproduces every committed face byte-for-byte — the rule that
keeps a font from becoming a function of the build environment

**Given** each new face
**When** it lands
**Then** it carries its own `NOTICE.md` recording the upstream release URL and source sha256, and
its OFL text, and `lint`'s `fonts-asset-unaccounted` / `fonts-asset-missing` guard accounts for it —
a face that ships without a licence record fails the build

**Given** the enlarged shipped set
**When** the offline release payload is generated
**Then** the manifest states the new measured byte cost per face, and the load screen itemises it as
it already itemises the CJK face, because the payload is the thing the author waits for

**Given** the corpus, no document of which declares bold or italic
**When** it is rendered on every target
**Then** the bytes are identical to before this story: a face nothing names is a face nothing embeds
(AD-21)

### Story 11.2: The engine resolves a face from the declared weight and slope

As a template author,
I want the bold and italic I set to select a real face,
So that the PDF prints the weight I asked for.

**Covers:** FR57 · AD-8, AD-21, AD-24

**Acceptance Criteria:**

**Given** a text element declaring `style.bold`, `style.italic`, or both, and a `style.fontFamily`
chain
**When** the document renders
**Then** the per-rune coverage resolution that already walks the chain resolves to the face carrying
that weight and slope, and the glyphs are shaped and measured from that face's own metrics — never
from the Regular face's

**Given** a table declaring `style.bold`, and optionally `headerStyle.bold`
**When** the document renders
**Then** its cells take the weight through the same cascade every other cell property already uses,
and `headerStyle` wins for the header row alone

**Given** a chain whose resolved face has no bold instance for some rune
**When** the document renders
**Then** the fallback is stated, not silent: the rune renders in the nearest available face and a
Warning names the element, the rune and the face — the same treatment an uncoverable rune already
gets, and never a fabricated outline

**Given** bold or italic set on an element
**When** its lines are broken and measured
**Then** the line breaks are those of the face actually used, so a bold heading may wrap differently
from the same words unbolded — and that difference appears on the canvas and in the PDF identically,
because both read the same engine measurement

**Given** an element declaring neither bold nor italic
**When** the document renders
**Then** nothing about its resolution changes and the corpus hashes identically (AD-21)

### Story 11.3: The canvas paints the weight the engine resolved

As a template author,
I want the canvas to show me the real face,
So that the preview holds no surprise the canvas could have shown me.

**Covers:** FR5, FR57 · AD-17 · UX-DR21
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the TYPOGRAPHY section's B / I pair in the inspector

**Acceptance Criteria:**

**Given** a component declaring bold or italic
**When** the canvas paints it
**Then** it paints in the actual face the engine resolved, delivered through the engine's own text
paint projection — never `font-weight: 700` or `font-style: italic` applied to the Regular face by
the browser

**Given** the canvas text paint
**When** it is inspected for a synthetic weight or slope
**Then** there is none: browser-side emboldening and obliquing are as forbidden on the canvas as they
are at emit time, and a contract test names the CSS properties the way the canvas-authority contract
already names the measurement APIs

**Given** a chain with no bold face and an element declaring bold
**When** the inspector is shown
**Then** the B control states that this family has no bold face rather than appearing to be on, so
the panel never shows a state the document cannot reach

### Story 11.4: Picking a family declares the cuts it has

As a template author,
I want the family I pick to bring its bold and italic with it,
So that pressing B works on a document I did not hand-edit.

**Covers:** FR57 · AD-8, AD-15, I-5
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the TYPOGRAPHY section's family control

**Added by OWNER DECISION D-11.0.1 (2026-09-06), which took the fourth option over the recommended
deferral.** The ruling that makes it necessary is D-11.2.1: the engine resolves a weight only to a face
the document explicitly NAMES, and it will never infer one — the route that does not break AD-8. So
after 11.2 and 11.3, `starter.folio` is the only document in the world that can bold, and only because
11.3 edits it. This story is what makes bold reachable in a document the author already has.

**Acceptance Criteria:**

**Given** an author picking a family in the family control
**When** the pick is applied
**Then** the chain entry it writes declares that family's available style variants alongside the base
face, using the same command family the control already uses — no new command kind, and nothing the
engine has to infer

**Given** a family with no bold cut, or no italic cut
**When** it is picked
**Then** the entry declares only the variants that exist, and the B or I control states the absence in
the words Story 11.3 gave it — an absent cut is declared absent, never declared and empty

**Given** a document whose chain entries predate this story
**When** it is opened
**Then** nothing is migrated, rewritten or repaired (I-5). The document reports honestly and acquires
variants only when its author re-picks the family — the same rule every other control in the product
follows

**Given** a document whose author never touches the family control
**When** it is rendered
**Then** the bytes are identical to before this story (AD-21)

### Story 11.5: A bold document is a pinned golden

As a maintainer,
I want a rendered bold document pinned byte-for-byte,
So that a face that silently changes is caught by the corpus rather than by a reader.

**Covers:** FR57 · AD-21, AD-22
**Design:** no mockup — this story ships a fixture and its goldens, not a surface.

**Split out of Story 11.3 by D-11.2.12.** Of everything 11.3 carried, this was the only item that moves a
rendered byte, and it was forcing the full four-target matrix onto a story that is otherwise entirely
designer-side. Splitting it means 11.3 runs no matrix at all — **the inverse of D-11.1.18, where splitting
would have multiplied the same gate.** This story depends on Story 11.2 alone; it does not depend on the
canvas.

**Why it exists (DW-237, measured).** `grep -rn '"bold"\|"italic"' fixtures/` returns **zero** across all
30 fixture directories — positive control `'"fontFamily"'` returns 23 files — and `worked-example.json`,
the one in-tree document that declares bold, has its rendered bytes pinned nowhere. **So a resolver that
silently switched face would fire no Warning, red no test and move no golden.** Story 11.2's DW-233
tripwire covers the *mechanism* behaviourally; this story covers the *outcome*.

**Acceptance Criteria:**

**Given** a new fixture document declaring bold and italic on a chain that declares its variants
**When** it is rendered on all four targets
**Then** the bytes are identical across targets and pinned by an `expected.json` and an `expected.pdf`,
registered in `goldenDigestRecord` and in CI's matrix slug list, and the full `-tags=matrix` suite runs
UNFILTERED in-story (D-11.1.9)

**Given** the new `expected.pdf`
**When** it is committed
**Then** it carries a **human attestation** — an `expected.pdf` is a human-attested artifact under
D-000.22 → D-2.3.5 (doctrinal, NOT architectural: AD-21 says nothing about attestation, and D-4.7.1 is
scoped to the statement family — see D-11.5.2), attestation is an OWNER action, and the story HALTS for it rather than self-attesting or
shipping the fixture without a golden

**Given** the resolver pointed at the base face instead of the declared variant
**When** the suite runs
**Then** the new golden moves and the story's own test fails — the red proof that this fixture is a witness
rather than a decoration

**Given** every existing fixture
**When** the corpus is rendered
**Then** its bytes are unchanged: this story adds a document, it does not alter one (AD-21)

**Do not edit `worked-example.json` to satisfy this.** `goldenfixture_test.go:16` byte-compares it against
the `## Worked example` fence at `folio-format.md:876`, so it is a document *and* a doc example and the two
move together. Ship a new fixture.

## Epic 12: The inspector reaches the engine that is already there

Six capabilities are shipped in the engine, tested by goldens, carried by the format — and reachable
from the designer by nobody. A page header is stuck at whatever height the starter file declared,
because no command in the product sets a band height: `pageSetup` takes a preset, an orientation, a
size and four margins, and `Band.Height` has no writer at all. A document's `locale` and `utcOffset`
have no control anywhere, so every date and number an author lays out formats as `en` — in a product
whose reason to exist is Thai and CJK statements. A table's `headerHeight`, its `headerStyle` and
Story 4.8's `altRowBackground` are authorable only by hand-editing the file the designer just wrote.

This is the mirror of Epic 9. There, the panel offered controls the engine did not honour; here, the
engine honours values the panel does not offer. Both leave the same author in the same place —
editing JSON by hand to finish a document the designer started — and FR12's promise that the format
is hand-editable was meant as a second door, not as the only way through.

`style.padding` is the one item that belongs to neither pattern, and 12.4 rules it rather than
building it: the format documents a default of `0` on all four edges for every element, the command
layer accepts all four keys on any component, and the only consumer on any render path is a table's
cell chrome. It is inert on text, image, rect and line exactly as `background` and `border` were
before Epic 9.

12.1 makes a band height settable and 12.5 makes it draggable — the boundary tabs the canvas already
draws are exactly where an author reaches for it, and the CONTENT tab sits on the header's lower edge
while the PAGE FOOTER tab sits on the footer's upper one. The command comes first because the drag
commits through it; the epic is not complete with only the number.

This epic adds no rendering behaviour that the engine does not already have. Every value it makes
authorable is one the loader already accepts and the renderer already consumes, so a document that
declares none of them must hash identically.

**FRs covered:** FR58, FR59, FR60
**Also lands:** FR5 completed for padding · AD-15 and AD-21 upheld unchanged

### Story 12.1: The page header and page footer take the height the author sets

As a template author,
I want to set how tall the page header and page footer are,
So that a letterhead fits without hand-editing the file the designer just saved.

**Covers:** FR2, FR6, FR58 · AD-15
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the document bar's page-setup summary and the inspector's PAGE SETUP panel

**Acceptance Criteria:**

**Given** the page setup panel
**When** it is shown
**Then** it offers a page-header height and a page-footer height beside the margins, in points, with
the engine's current values

**Given** a new band height
**When** it is committed
**Then** it travels as one versioned opaque command that Go validates and applies, exactly as the
page setup command already does, and the canvas re-projects from the engine's accepted document —
never from a browser-side model of the bands

**Given** a band height that would leave no content window — header plus footer at or beyond the
page's inner height
**When** the command is applied
**Then** it is refused with a located message naming the quantity and the space available, and the
document is unchanged, because a document with no content band is not a document

> ~~**Given** a band height reduced below the content its band already holds · **When** the command is
> applied · **Then** it is accepted and the overflow is clipped with the diagnostic FR44 already
> defines~~
>
> **STRUCK — the engine REFUSES, and Story 12.1 shipped the refusal.** D-12.B ruled this at 12.1's own
> checkpoint and the text was never corrected. Measured: shortening a header to 20pt with an element at
> y=50 h=30 gives *"a pageHeader height of 20pt would leave e1 outside the band: it reaches 80pt"*.
> Partial overflow refuses too; four shipped tests pin it. **FR44 has no vertical axis** — `diag.go`
> declares 17 codes and only `TEXT_CLIPPED_WIDTH` and `TABLE_ROW_CLIPPED_HEIGHT` are clip codes, while
> `render.go` states that D-2.8.1 *"fences the vertical axis out entirely"*. Building this literally
> would require minting a diagnostic code, adding render behaviour, and lifting `isCanvas`'s band
> clause.

**Given** a document whose band heights are not edited
**When** it is serialized
**Then** the bytes are unchanged, including a band whose height key was absent staying absent

### Story 12.2: The document declares its locale and its UTC offset

As a template author,
I want to set the document's locale and UTC offset in the designer,
So that a Thai statement formats its dates and numbers as Thai without hand-editing the file.

**Covers:** FR21, FR22, FR59 · AD-12
**Design:** no mockup draws this panel. `Main.dc.html`'s inspector draws PROPERTIES, POSITION,
  TYPOGRAPHY, CONTENT, BANDS and COMPONENTS only; no mockup contains "page setup", "locale" or
  "UTC offset". Follow the **shipped** PAGE SETUP panel instead — `folio-designer/src/App.tsx:1682`
  (`PageSetup`), applied by `applyPageSetup` at `:780` — and match its existing row idiom.

**Acceptance Criteria:**

**Given** the page setup panel, which is where document-level settings already live
**When** it is shown
**Then** it offers the locale as the closed set AD-12 defines — `en`, `th`, `zh-Hans`, `ja` — and the
UTC offset as a `±HH:MM` field, both showing the engine's current values

**Given** a locale change
**When** it is committed
**Then** every `formatDate` and `formatNumber` in the document formats under it on the next preview,
including the Buddhist-era year `th` already produces, with no other change to the document

**Given** a UTC offset that does not match `±HH:MM`
**When** it is committed
**Then** it is refused with a located message and the document is unchanged

**Given** a document whose locale and offset are not edited
**When** it is serialized
**Then** the bytes are unchanged

### Story 12.3: A table's header and its alternating rows are authorable

As a template author,
I want to set a table's header height, header style and alternating row colour,
So that the table styling the engine can already render is reachable from the editor that builds it.

**Covers:** FR10, FR60 · AD-13
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/TableEditor.dc.html`
  — the HEADER and CELLS sections beside the column matrix

**Acceptance Criteria:**

**Given** the table editor, which today configures columns only
**When** it is opened
**Then** it also offers the table's header height, its `altRowBackground`, and the header-row style
fields the engine already resolves — each showing the engine's committed value and each clearable
back to absent

**Given** an alternating row background
**When** the document renders
**Then** it paints exactly as Story 4.8 already renders it; this story adds no rendering rule and no
second implementation of the cascade

**Given** a header style field left absent
**When** the document renders
**Then** it falls back to the table's own style and then to that field's documented default, which
is the cascade Story 4.1 already defines

**Given** a table whose header and row styling are not edited
**When** it is serialized
**Then** the bytes are unchanged

### Story 12.4: Padding is honoured outside a table, or the format says it is not

As a template author,
I want `style.padding` to do what the format says it does,
So that a value the file carries and the panel can write is not silently ignored on four of the five
components.

**Covers:** FR5 · AD-21, AD-24

**Acceptance Criteria:**

**Given** the format's Style table, which documents `padding` with a default of `0` on all four
edges and scopes it to no particular element kind
**When** this story is planned
**Then** the owner rules one of two ways, and the ruling is recorded in the decision log before any
code is written: padding insets a text element's content box, or padding is documented as
table-only and the command layer stops accepting it on the other four kinds

**Given** the ruling is that padding insets
**When** a text element declaring `style.padding` renders
**Then** its lines are laid out, aligned and clipped against its declared box inset by the four
edges — the same quantity a table cell already computes, through the same helper — and the width
overflow diagnostic measures against the inset width

**Given** the ruling is that padding is table-only
**When** a non-table element declares padding
**Then** the command layer refuses the field with a located message, the loader keeps accepting it
for compatibility, and the format states the restriction in the Style table

**Given** either ruling
**When** the corpus is rendered
**Then** no document in it declares padding on a non-table element, so the bytes are identical
(AD-21)

### Story 12.5: A band boundary is dragged on the canvas

As a template author,
I want to resize the page header and page footer by dragging on the canvas,
So that fitting a letterhead is a gesture rather than a number I have to guess and retype.

**Covers:** FR1, FR3, FR6, FR58 · UX-DR6, UX-DR11, UX-DR18, UX-DR25
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the band tabs (PAGE HEADER / CONTENT / PAGE FOOTER) drawn at each band's top-left

**Acceptance Criteria:**

**Given** the band tabs, each of which already sits at the top-left of the band it names
**When** the author drags the **CONTENT** tab
**Then** the page-header/content boundary moves with the pointer, which is the page header's height —
and dragging the **PAGE FOOTER** tab moves the content/footer boundary, which is the page footer's
height

**Given** the **PAGE HEADER** tab, which sits at the top of the page rather than on a boundary
between two bands
**When** the author drags it
**Then** nothing resizes, because the edge above it is the page margin and not a band boundary — a
control that looks draggable and is not would be worse than one that does not look draggable

**Given** a drag in progress
**When** the boundary is moving
**Then** it is the same transient local proposal a component drag already is: the canvas paints the
proposed boundary, the height is shown in points as it moves, nothing is committed, and the pointer
release sends the one Story 12.1 command whose accepted value then replaces the proposal

**Given** snapping is on
**When** the boundary is dragged
**Then** it snaps to the same grid increment every other drag uses, and is released the same way

**Given** a drag that would leave no content window, or a negative band height
**When** the pointer moves past that point
**Then** the boundary stops there rather than being committed and refused — Story 12.1's engine rule
is enforced as a drag limit, so an impossible document is never proposed

**Given** the boundary
**When** it is reached by keyboard
**Then** it is focusable and resizable by the same nudge keys that move a component, to the floor
UX-DR25 sets — the drag is an affordance on top of Story 12.1's command, never the only way to reach
the value

> ~~**Given** components already placed in a band · **When** that band is made shorter than the content
> it holds · **Then** they are clipped with FR44's existing diagnostic and are never moved, deleted or
> reflowed (AD-24)~~
>
> **STRUCK for the same reason as Story 12.1's, above — this is the second instance of one false
> premise.** The engine refuses rather than clipping. Discharged by 12.1's shipped refusal and restated
> in 12.5's spec as a matrix row: the release is refused, located, carrying the ElementID, and the
> document's bytes are unchanged. **The never-moved-never-reflowed half (AD-24) survives and is not in
> question** — it is the clip half that was never true.

## Epic 13: A template author can read, navigate and keep the exact PDF

The preview is the screen where Folio's whole claim lands: this is the production document, produced
here, and here is the hash that proves it. `Preview.dc.html` draws that screen as three columns — a
PAGES thumbnail rail, the page, and an evidence rail carrying RENDER (engine, target, pages, rows,
elapsed, size), OUTPUT HASH with *Matches native render*, DIAGNOSTICS with its zero state and its
shape legend, and two buttons: **Re-render** and **Save PDF**.

What shipped is the middle column. The thumbnail rail was never built, and the *design-mode palette
rail* occupies its place — five dead placement controls beside a PDF. The evidence rail was never
built; the hash survives as one grey footnote line. Neither button exists. And the PDF cannot leave
the tab at all: there is no export path anywhere in the designer, so the only way to obtain the
document the author is looking at is to run the CLI over the same file.

Navigation is one page at a time behind Previous/Next, and zoom is `−`/`+` in ten-point steps
clamped to 50–200% — fifteen clicks end to end, on a document whose fixture is thirty-four pages.
There is no fit-width, no fit-page, no zoom entry and no page entry. The viewer's own scroll
container has a `min-height` and no cap, so it grows rather than scrolls and the outer region does
the scrolling instead; the `scrollTop` and `scrollLeft` the view state carries can therefore never
be non-zero, and the effect that restores them is dead code — which is why leaving Preview and
returning loses the author's place.

Preview also refuses to run at all without a loaded sample JSON, including for a template that
declares no bindings. The engine has no such requirement: the CLI renders that template with no
`-data` argument.

This epic touches no engine byte. Every PDF it displays, exports and describes is one the engine
already produced.

> **AMENDED BY OWNER DECISION, 2026-09-07 (D-13.4.1).** Story 13.4 is an exception to the sentence
> above. It adds a NEW read-only Go function — a type-directed stand-in document generator — plus a
> wasm op to expose it. The exception was taken knowingly, by the owner, because the story's second
> acceptance criterion is not deliverable otherwise: there is no single value that renders empty
> (`upper`/`lower` reject null, `formatNumber` rejects null AND "", `formatDate` accepts neither and has
> no empty form, and a null under `visibleIf` SILENTLY DELETES the element per D-3.2.3). **`Render`
> itself is still untouched** — it is called with genuinely supplied data — so the promise's intent, that
> every displayed PDF is one the engine really produced, holds. The literal claim does not, and this
> note exists so an auditor finds the exception rather than the contradiction.

**FRs covered:** FR61, FR62
**Also lands:** UX-DR9, UX-DR14, UX-DR17, UX-DR21, UX-DR22, UX-DR23 — the exactness claim is the
thing this screen exists to make legible, and it is a constraint on every story below

### Story 13.1: The preview keeps the PDF

As a template author,
I want to save the PDF I am looking at,
So that the document I just proved correct can be sent to someone.

**Covers:** FR61 · UX-DR9, UX-DR23
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Preview.dc.html`
  — the Save PDF control, paired with Re-render at the foot of the evidence rail

**Acceptance Criteria:**

**Given** a current preview
**When** the author chooses Save PDF
**Then** the exact bytes the engine returned are written to a local file of the author's choosing —
never re-rendered for the save, never re-serialized, so the saved file's hash is the hash the screen
displays

**Given** a browser with the File System Access API and one without
**When** Save PDF is used in either
**Then** it works in both, through the two file-access tiers the designer already implements for
`.folio`, with the picker's type and suggested name parameterised rather than a second download path
written

**Given** a stale preview
**When** Save PDF is offered
**Then** it saves the stale document only if the author is told it is stale in the same breath —
the preview's freshness rule governs the export exactly as it governs the display (UX-DR14)

**Given** no preview has rendered
**When** the screen is shown
**Then** Save PDF is absent or disabled with its reason stated, never a control that fails when
pressed

### Story 13.2: The viewer navigates like a PDF viewer

As a template author,
I want to fit the page, type a zoom, and jump to a page,
So that reading a thirty-four page statement is not sixty clicks.

**Covers:** FR62 · AD-17 · UX-DR9, UX-DR25
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Preview.dc.html`
  — the bottom status bar carrying the page stepper, the page indicator and the zoom

**Acceptance Criteria:**

**Given** the viewer controls
**When** they are shown
**Then** they offer fit-width, fit-page, a zoom the author can type or choose, and a page number the
author can type — alongside the existing previous/next and `−`/`+`

**Given** fit-width or fit-page
**When** it is chosen
**Then** the page is scaled to the viewer's own displayed width or height, and the choice persists
across page changes until the author zooms manually

**Given** the viewer needs its container's pixel width to fit
**When** that measurement is taken
**Then** it is taken inside `src/preview/` under the same narrow exception the canvas-authority
contract already grants that directory for `scroll*` — because a rasterized PDF's display scale is
viewer navigation, not document measurement — and the exception names the new property explicitly
rather than being widened by wildcard

**Given** a zoom beyond the size of the viewport
**When** the page is displayed
**Then** the viewer's own container scrolls, both axes, with the page centred when it is smaller
than the container — the container is height-constrained rather than growing, so the outer region
never scrolls in its place

**Given** an author who scrolls, zooms or changes page, then leaves Preview and returns
**When** the preview is shown again
**Then** the page, the zoom and the scroll position are the ones they left, because the view state
the viewer already carries is now actually reachable

**Given** the design, which puts the page stepper, the page indicator and the zoom in the
application's **bottom status bar** rather than on a toolbar above the page
**When** the controls are placed
**Then** they go where the design puts them, so the page area carries the page and nothing else

**Given** every new control
**When** it is used by keyboard alone
**Then** it is reachable, labelled and operable, to the same behavioural floor UX-DR25 sets for the
rest of the product

### Story 13.3: The preview screen is the evidence screen

As a template author,
I want the preview to show me what was rendered and prove it matches,
So that "this is the exact production document" is something I can read rather than something I am
told.

**Covers:** FR34, FR35, FR61 · AD-18 · UX-DR9, UX-DR17, UX-DR21, UX-DR22
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Preview.dc.html`
  — the whole screen: the PAGES rail, PRODUCTION OUTPUT, and the RENDER / OUTPUT HASH / DIAGNOSTICS rail

**Acceptance Criteria:**

**Given** the preview screen
**When** it is shown
**Then** the left rail is the PAGES thumbnail rail the design draws — one thumbnail per page,
numbered, the current page marked, clicking one navigates — and the component palette is not
rendered in preview mode at all, because nothing in this mode can be placed

**Given** a completed render
**When** the evidence rail is shown
**Then** it carries RENDER — engine version, target, page count, row count, elapsed, byte size — and
OUTPUT HASH as a first-class block rather than a footnote, each value coming from the render the
engine actually performed

**Given** the OUTPUT HASH block
**When** it is shown
**Then** the digest is set in mono and wrapped across two lines as the design sets it, inside its own
bordered block — a hash a person is expected to compare by eye is typeset to be compared by eye

**Given** the design's affirmation beneath the hash — *"Matches native render"*, and beneath that
*"Identical bytes on darwin/arm64, linux/arm64, linux/amd64"*
**When** it is implemented
**Then** the claim is earned or it is reworded, and this is ruled before it is drawn: **the tab cannot
compare itself to a native render**. What is true is that the wasm engine is the same engine compiled
to another target and that cross-target identity is proven by the matrix in CI for the release the
browser is running — a property of the build, not a live comparison. The wording must say the true
thing; an affirmation the product cannot substantiate is the one thing this screen must never print
(UX-DR21, UX-DR23)

**Given** the DIAGNOSTICS section
**When** it is shown
**Then** its header carries the counts the design shows — total, and errors separately — so the
section is readable without opening a card

**Given** one diagnostic
**When** its card is shown
**Then** it names its location the way the design does — the page, the bound path, the element kind
and the band (`page 3 · transactions[11].description · table · band content`) — and carries **Locate
on canvas** and **Dismiss**, which is the behaviour Story 5.12 already built and this story only
re-dresses

**Given** the PAGES rail on a document longer than the rail
**When** it is shown
**Then** it truncates rather than rendering thirty-four thumbnails, and the current page is marked in the
select accent (UX-DR2, UX-DR22).

**AMENDED 2026-09-09 BY OWNER DECISION D-13.6.7 — THE DIAGNOSTIC MAP IS DROPPED.** This AC previously
required "a page carrying a diagnostic is marked in the bind accent — so the rail is also the diagnostic
map". It is removed rather than deferred: the owner declined to extend the engine for it, so no document
should go on claiming it. **Why it was not deliverable:** `EngineDiagnostic` (`engine-protocol.ts:172`)
carries no page; `isDiagnostic` (`:413`) uses `hasExactKeys` over a closed five-key set, so a `page` field
on the wire is rejected rather than ignored; and of the seven `Diagnostic{` construction sites in
`folio-go/render.go`, five run **before pagination**, so no page yet exists to record. There is no join
anywhere in the wasm reply between a page index and a diagnostic's `elementId`. Supplying one meant widening
the exported engine contract a third time, which the owner had already ruled against for Story 13.3
(*"never add a page number on a diagnostic"*). DW-311 is closed as declined, not left open.

The `… 29 more` example is also removed: it is an illustration constrained by the mockup's own fixed 764px
canvas, not a specified bound. The shipped bound is **12**, one constant in `page-rail-facts.ts`.

**Given** the page area
**When** it is shown
**Then** it carries the design's **PRODUCTION OUTPUT** badge, and the page sits on the dark ground
with nothing else competing with it

**Given** Re-render and Save PDF
**When** they are shown
**Then** they are a paired action row at the foot of the evidence rail, as the design places them —
not one control in the rail and another in a tab

**Given** a render with no diagnostics
**When** the diagnostics section is shown
**Then** it states zero explicitly, with the shape legend the design specifies — triangle-dashed for
a render that proceeded, square-solid for one that failed — so a clean render reads as *checked*
rather than as *nothing here*

**Given** the evidence rail
**When** Re-render is pressed
**Then** the document re-renders from the current inputs, and the control sits with the evidence
rather than inside the INPUTS tab where it is today

**Given** a stale or failed preview
**When** the evidence rail is shown
**Then** it says so in the rail as well as in the status line, and never displays stale statistics as
though they describe the current document (UX-DR14)

### Story 13.4: Preview runs without sample data, and an absent value is empty

As a template author,
I want to preview immediately, with no JSON file,
So that laying out a page is not gated on inventing data I do not have yet.

**Covers:** FR9, FR34 · AD-18 · UX-DR13, UX-DR24
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Preview.dc.html`
  — the preview surface this state must degrade from
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/EXPERIENCE.md`
  — the ten declared states per surface, including Empty — no sample data

**Acceptance Criteria:**

**Given** no sample data loaded
**When** the author enters Preview
**Then** it renders — the screen is never disabled ahead of time on a condition the engine does not
impose, and the CLI already renders such a template with no data argument

**Given** a template that resolves data paths, and no sample data
**When** it is previewed
**Then** every unresolved path renders as **empty** and the page is produced, rather than the render
failing at the first absent path

**Given** that `BINDING_PATH_ABSENT` is today a located **Error** — Story 3.6's contract, S8's
coverage, and the behaviour a production render must keep, because a statement that silently prints
a blank where a customer's name belongs is the failure that contract exists to prevent
**When** this story is implemented
**Then** the empty-value behaviour is **scoped to preview with no data supplied** and is never a
change to `Render`'s own semantics: the engine's error contract, its diagnostic codes and the golden
corpus are untouched, and the same template rendered by `folio-go` with absent data still fails
exactly as it does today

**Given** a preview rendered this way
**When** it is shown
**Then** the screen states plainly that it is a no-data preview and that empty values are stand-ins —
the exactness claim is the product, so a preview that is *not* the production document must never be
presented as though it were (UX-DR14, UX-DR21)

**Given** a no-data preview
**When** its output hash is displayed
**Then** it is not offered as evidence of native/wasm equality, because the inputs were not the
production inputs

**Given** sample data is then loaded
**When** the preview re-renders
**Then** it returns to the exact-production path with no stand-ins, and the freshness rule marks the
earlier preview stale exactly as any other input change does

**Given** a path that is absent from data that *was* supplied
**When** it is previewed
**Then** it is still the located Error Story 6.6 presents — absent data and absent-from-present-data
are different conditions, and only the first is filled with an empty value

### Story 13.5: The chrome tells the truth about the preview

As a template author,
I want the frame around the preview to say what I am looking at and when it was made,
So that freshness is something I can see rather than something I have to remember.

**Covers:** FR34 · AD-18 · UX-DR9, UX-DR14, UX-DR16, UX-DR21, UX-DR23
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Preview.dc.html`
  — the status bar's standing assurance (the mockup shows the original "no network · nothing
  left this machine"; amended 2026-09-21 by D-GA.5 to "local render · your data stays here")
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the document bar this replaces in Preview mode

**Acceptance Criteria:**

**Given** the document bar, which in Design mode reads `A4 · portrait`
**When** the author is in Preview
**Then** it reads the render's own freshness instead — *"rendered 412 ms ago · current"* — because the
page setup is a Design-mode fact and freshness is the Preview-mode fact (UX-DR14)

**Given** the DESIGN / PREVIEW switch in the document bar
**When** the author wants to go back
**Then** that switch is the way back, and the separate "Return to Design" button inside the preview
heading is removed — one way to change mode, in the place the design puts it

**Given** a render in progress
**When** the author asks to leave
**Then** cancelling is still reachable, since removing the heading button must not remove the ability
to abandon a long render

**Given** the application status bar
**When** the author is in Preview
**Then** it carries the design's standing assurance — ~~*"no network · nothing left this machine"*~~
*"local render · your data stays here"* — which is the product's central promise stated where it is
always visible (UX-DR23). **AMENDED 2026-09-21 by OWNER DECISION (D-GA.5).** The original wording is
preserved above verbatim. **What still holds:** the promise, its slot, its `local-only-assurance`
hook and the assertions that pin it — it is narrowed, never dropped. **What changed:** D-GA.1 made
*"no network"* false for a build configured with Google Tag Manager (AD-27), so the assurance now
states the half that is substantive and still exactly true — the render is local and the author's
data stays on the machine — rather than a claim the page no longer honours.

**Given** a stale preview
**When** the chrome is shown
**Then** the freshness reads stale rather than `current`, in the same words the status line uses, so
the two never disagree

### Story 13.6: The preview navigates by page thumbnails

As a template author,
I want a rail of page thumbnails beside the preview that marks which pages carry diagnostics,
So that I can see the shape of the document and jump straight to the page a problem is on.

**Covers:** the Epic 13 goal's "page-thumbnail rail that doubles as a diagnostic map" - the half of
Story 13.3 that was split off at its plan gate (DW-304, D-13.3.1)
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Preview.dc.html`

**OWNER DECISION 2026-09-09 (D-13.6.2), AMENDING D-13.6.1: the rail is built from pdf.js's thumbnail
modules, VENDORED into this repository.** Scope narrowed by the orchestrator to **two files, 629 lines**
(D-13.6.4): `pdf_thumbnail_view.js` (557) + `renderable_view.js` (72), from tag `v6.2.108`, build SHA
`0365cbde0`, with `app_options.js`'s import dropped as a recorded modification because it supplies exactly
two numeric defaults that are already constructor options. **`pdf_thumbnail_viewer.js` is NOT vendored** -
measured, at this version it is Firefox's page-organiser (1,964 lines; keyword counts drag 190, undo 45,
merge 31, paste 22; private methods `#deletePages`, `#cutPages`, `#pastePages`, `#undo`, `#reportTelemetry`),
and folio needs about 5% of it. Folio writes its own ~200-line rail container instead.

D-13.6.1 said "built on `pdfjs-dist`'s viewer components - `PDFViewer` and `PDFThumbnailViewer`". **That
decision rested on a false premise supplied by the orchestrator: `PDFThumbnailViewer` is not in the published
package.** Verified on independent instruments with a positive control - it appears 0 times in
`web/pdf_viewer.mjs` while `PDFViewer` appears 4, and the bundle's own export statement lists 22 names with
no thumbnail class. Mozilla ships those modules in its repository and excludes them from the components
bundle.

Re-put to the owner with that correction and with both alternatives, the owner chose to **vendor
pdf.js's thumbnail modules from mozilla/pdf.js at a tag matching the installed `pdfjs-dist` version** (as
first put to them, `pdf_thumbnail_viewer.js` and its dependencies; narrowed to the two files named above by
D-13.6.4, which supersedes that phrasing). The orchestrator and the story's builder both recommended against it and both were
overruled. As with D-13.6.1, the objections raised become acceptance criteria rather than caveats.

**Acceptance Criteria:**

**Given** vendored third-party source
**When** it lands in this repository
**Then** its provenance is recorded and checkable: the exact upstream tag or commit, the file list, a hash
per file, the preserved Apache-2.0 headers, and a written procedure for re-vendoring. This project has a
precedent for that audit in `2-3a-audit-the-vendor-boundary.md` and it is followed, not reinvented

**Given** that `PDFViewer` hard-throws on any version drift between core and viewer builds
**When** the thumbnail modules are vendored
**Then** they are taken from a tag matching `pdfjs-dist` 6.2.108 exactly, and a test asserts the pin so a
future `pdfjs-dist` bump cannot silently desynchronise the fork

**Given** `canvas-authority-contract.test.ts:8` walks **all** of `src` recursively, and AD-17 forbids the
browser measuring text
**When** vendored pdf.js code is added
**Then** **no exclusion is written, because none is needed. AMENDED 2026-09-09 (D-13.6.5): this AC's premise
was measured false.** It assumed the vendored files measure their own rendered output. Ran by the builder
through the contract's own comment-stripping scanner: all 17 `prohibited` regexes and all 19
`refusalVocabulary` regexes against both files return **zero hits each**. The 36 hits that motivated this AC
are all inside `pdf_thumbnail_viewer.js` - the file D-13.6.4 removed from the vendored set. Separately,
`canvas-authority-contract.test.ts:8-16` builds its corpora with `/\.(?:ts|tsx|css)$/`, `/\.(?:ts|tsx)$/`
and `/\.test\.(?:ts|tsx)$/`, so a `.js` file enters none of them regardless.

The worry is honoured **positively instead of by carve-out**: `vendor-pin.test.ts` asserts each vendored file
contains **none** of the prohibited identifiers, importing that list from the contract rather than restating
it, and fails if a third file ever appears in the directory. A future re-vendor that drags in a measuring file
reds immediately. **The files are clean rather than exempt, and there is no exemption for a later file to
inherit** - which is the precise hazard the original wording named. Writing a carve-out for identifiers that
are not present would be a guard that cannot fail, the defect this run has found more often than any other.

**The vendored tree lives at `src/vendor/pdfjs/`, deliberately OUTSIDE `src/preview/`**, because
`withoutApprovedLocalPointerInput` (`canvas-authority-contract.test.ts:866-911`) waives
`scroll(?:Width|Height|Left|Top)` for every file matching `preview` as a path segment. A `.d.ts` sidecar
placed inside `src/preview/` would inherit that directory-wide waiver by accident; outside it, it inherits
nothing. The `.d.ts` sidecars **are** in the AD-17 production corpus, so prohibited spellings must stay out
of them

**Given** `maximumCacheAssets = 64` at `src/release-payload.ts:42` and a current built count of **61**
**When** this story adds anything to the release
**Then** the offline release still builds and still passes, with the measured `assetCount` reported **at the
Epic 13 boundary gate, which is the only place that can measure it.** AMENDED 2026-09-09: this AC originally
required the story to report a built `assetCount`, which D-000.33 forbids it from producing - `npm run build`
and the `verify:offline*` chain are the boundary gate's. The story states a **prediction for the gate to
confirm or refute: zero new asset rows, `s1.assetCount` unchanged at 61**, because two `.js` modules imported
from TypeScript are bundled into an existing chunk rather than emitted as their own rows, and no image, font
or CSS file is added. **If the gate measures anything but 61, that is a finding, not a rounding difference.**

**CORRECTED 2026-09-09 - THE PREDICTION WAS WRONG, AND THIS AC IS WHY WE KNOW.** Measured during
implementation: `s1.assetCount = 62`, not 61. The vendored module is **not** folded into an existing chunk; it
is emitted as its own 7.56 kB chunk. **Two** free slots remain against `maximumCacheAssets = 64`, not three.

The story's builder supplied the prediction, I hardened it into a falsifiable claim, and the claim was
falsified **at implementation rather than at the boundary gate** - which is the entire value of stating a
number instead of an expectation. The gate now confirms **62**, and a 61 would itself be the finding. Nothing
about this weakens the AC; a prediction that could not have been wrong would have told us nothing.
There are three free slots against `maximumCacheAssets = 64`; `vite.config.ts:16` sets `assetsInlineLimit: 0`,
so nothing is inlined, and `web/pdf_viewer.css` alone references 36 unique images - which is why it is not
imported at all

**Acceptance Criteria:**

**Given** a rendered preview of any length
**When** the author is in Preview
**Then** a rail shows a thumbnail of every page, numbered, with the current page marked

**Given** a thumbnail
**When** the author clicks it
**Then** the preview navigates to that page

**Given** a render that produced diagnostics
**When** the rail is shown
**Then** **nothing is marked, and that is deliberate. OWNER DECISION 2026-09-09 (D-13.6.3): the diagnostic
map is DEFERRED out of this story to DW-311.** It has no data source. Measured: `EngineDiagnostic`
(`engine-protocol.ts:172`) carries no page; `isDiagnostic` (`:413`) uses `hasExactKeys` over a closed
five-key set, so a `page` field on the wire is rejected outright; and of the seven `Diagnostic{` construction
sites in `folio-go/render.go`, only two know a page - the other five run **before pagination**, so no page
yet exists to record. There is no join anywhere in the wasm reply between a thumbnail's page index and a
diagnostic's `elementId`.

The owner already ruled against the obvious fix, for Story 13.3 on 2026-09-08: *"no page on the diagnostic …
a page number is the only one of the design's four components that is a property of layout rather than of the
element, and computing layout in the browser is forbidden outright"*, fenced as *"never add a page number on
a diagnostic"*. 13.3 handed this story the instruction to source page marking from its own thumbnail
enumeration; measured, that is not achievable. **A partial map was explicitly rejected** - marking the two
table sites and silently marking nothing for the other five would let an author read an unmarked page as
clean, which is the guard-that-cannot-fail shape this run has found more often than any other, inside the
story whose subject is a diagnostic map

**Given** a document longer than the rail's bound
**When** the rail is shown
**Then** it truncates with `... N more` rather than growing without limit

**Given** Preview mode
**When** the rail is shown
**Then** the component palette is not rendered. **This is real work, not a regression guard** - measured,
`App.tsx:2261` renders `<nav className="palette-rail">` as an unconditional child of `.workbench`, the mode
ternary does not begin until `:2276`, no CSS override hides it, and no existing test asserts its absence
(population searched: all `folio-designer/src/**/*.test.ts(x)` and all 20 `folio-designer/e2e/*.spec.ts`).
It renders at its full width in Preview today. Note `.workbench`'s `grid-template-columns` at `App.css:46`
is the declaration Story 13.3 deliberately left free for this rail, and the authority contract forbids
adding a second `@media` rule, so the rail's width cannot be responsive

**Given** that page state in the preview has exactly one owner today
**When** this story adds a rail that knows which page is current
**Then** that owner remains `App.tsx:254`'s `previewViewState`, written through its single funnel at
`App.tsx:814`, and the rail reads from it rather than keeping its own. **CORRECTED 2026-09-09:** this AC
originally said Story 13.2's `viewer-navigation.ts` must be retired or subordinated. That rested on a false
premise - measured, that file is 108 lines of pure arithmetic with no imports, no React, no DOM and no state
of any kind, so it is not a page-state authority and never was. It stays: retiring it would delete ten
zoom/fit tests encoding two measured regressions, including the one where Zoom out enlarged the page by 65%.
The AC's intent is unchanged and matters more than its original wording, because any vendored or adopted
pdf.js viewer component that tracks `currentPageNumber` internally WOULD be a genuine second authority.

**Given** the product promises "no network - nothing left this machine" (amended 2026-09-21 by D-GA.5
to "local render - your data stays here"; see NFR8 above and `ARCHITECTURE-SPINE.md` AD-27) and ships an offline release
**When** the viewer bundle is added
**Then** the release's size change is measured and recorded, not assumed - **at the boundary gate.**
**AMENDED 2026-09-09: this AC's figures priced a route that was ruled out.** It quoted `pdf_viewer.mjs`
307 KB + `pdf_viewer.css` 160 KB + 328 KB of images at roughly 90% PDF.js growth. Under D-13.6.4's narrowed
vendor **none of that is added**: the 629 vendored lines share the `build/pdf.mjs` already shipped, and the
upstream thumbnail rules reference no `images/` files at all. The expected change is therefore small and
additive, and the gate reports what it actually measures rather than checking against a superseded number.

## Epic 14: The designer's controls read as one product

The inspector and the document bar were built story by story, and it shows. There are five button
treatments in play: icon-only, text-only, text-with-shortcut, icon-with-text-and-shortcut, and
symbol-only — and they are mixed inside single rows. Open and Save are icons; **Save As**, the same
family of action, is text, and the design's own document bar draws all of them as text, so the icons
are drift rather than intent. In one row of the TYPOGRAPHY section, Align is **four** SVG icons (**corrected 2026-09-09**: this said three; justify landed since, and D-000.9 item 9 pre-registered the correction — measured at `App.tsx:2658`, an all-text selection offers four) and
Vertical align, beside it, is the words TOP / MID / BOT.

The per-kind defects are sharper than the cosmetics. A Line's thickness is its **H** field under
POSITION and its colour is **Background** under BOX, because Story 9.2 makes a line a filled bar — so
the two properties a line actually has are the two an author would never look for, while Border,
Border colour and four Edge checkboxes sit above them meaning nothing. A placed component is not
selected, so an author places a Line and must then find and click a 1pt target — **2 CSS pixels**,
because `.canvas-component` floors both axes at `max(2px, …)`; without that floor it would be 1px at
100%, folio doing its own millipoint arithmetic rather than the 96/72 CSS conversion — before they
can edit it. (Corrected 2026-09-09: this line read "1.33 CSS pixels at 100%", which is the CSS
pt-to-px ratio and not what this canvas does. The defect is real — 2px is far under the 24px
`.resize-handle` and 12px `.selection-handle` precedents — but smaller than first stated, and a story
should not inherit a justification whose arithmetic is wrong.) And every non-text component shows a BINDING section inviting the
author to "Pick a root scalar in the Data tab", when `bindComponentScalar` refuses everything that is
not a text element.

14.5 is the one addition rather than a correction: the design gives the product a mark — a square
outline around a solid block, drawn at three sizes across the mockups — and the chrome has never worn
it.

14.6 is the same failure as Epic 13's preview screen, one panel over. `Binding.dc.html` draws a
binding panel whose tree shows the author their own data — values beside paths, `{ }` and `[]`
markers, TABLE ONLY and RUNTIME badges, dimmed rows for what cannot be picked — and what shipped
shows a label, a full path and a concatenated type string. `SampleNode` already carries the values;
the panel simply never renders them.

14.9 and 14.10 close the loop the other four leave open. `Binding.dc.html` draws a table on the
canvas as a real table — a chip naming the bound collection and the column count, the header labels,
and one representative row showing each column's binding in the bind accent — while the canvas today
paints a box containing the word "Table". It cannot do better: the canvas projection carries one
string, `tableBind`, and the columns live only in the projection the table editor opens. With the
columns on the canvas, binding one stops being a reason to open a dialog and becomes what binding is
everywhere else — select the thing, pick the path.

14.7 and 14.8 are the third panel with the same story. `TableEditor.dc.html` draws a six-column
matrix with a width budget, a row-scope context line and three property sections; what shipped is an
eleven-column form with two bare inputs on top and no property sections at all. Two of the design's
choices there are not fidelity questions and are called out as rulings rather than drawn: **Cancel /
Apply** implies a local uncommitted buffer, which is the second document model AD-15 exists to
forbid; and the design's **millimetres** contradict the points every other panel shows, which is a
product-wide unit decision rather than a dialog's.

Nothing here changes the document model, the command surface or a single rendered byte. It changes
what the panel offers and how it is spelled.

**FRs covered:** none new — FR1, FR4, FR5, FR7 and FR10 completed
**Also lands:** UX-DR2, UX-DR7, UX-DR10, UX-DR18, UX-DR21, UX-DR24, UX-DR25

### Story 14.1: One button vocabulary

As a template author,
I want the controls to look like one product,
So that I can tell what a control is by looking at it.

**Covers:** UX-DR1, UX-DR10, UX-DR24
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the document bar's Open / Save / Save As row, and the inspector's two segmented controls
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md`
  — the token file the rule is written in terms of

**Acceptance Criteria:**

**Given** the rule for when a control is a word and when it is a glyph
**When** this story is planned
**Then** it is written down first, in DESIGN.md's terms, and every control in the product is then
audited against it — the rule is the deliverable, the sweep is its consequence

**Given** the document bar
**When** it is shown
**Then** Open and Save are spelled the way `Main.dc.html` draws them and the way Save As, Start
blank, Undo and Redo already are, so no two members of the same action family are spelled
differently

**Given** the TYPOGRAPHY section's two segmented controls
**When** they are shown
**Then** Align and Vertical align share one vocabulary — both iconic — rather than icons beside
words at the same size in the same row

**Given** any control the sweep changes
**When** it is used by keyboard or read by a screen reader
**Then** its accessible name is unchanged or improved, never lost to an icon without a label
(UX-DR25)

### Story 14.2: A Line is a thickness and a colour; a Rectangle is a fill and a border

As a template author,
I want a Line's properties to be the properties a line has,
So that drawing a rule does not require knowing it is implemented as a filled box.

**Covers:** FR4, FR5 · UX-DR10, UX-DR24
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the inspector's POSITION and BOX sections

  **What the mockup actually contains, verified 2026-09-09:** `Main.dc.html` draws a **Text** inspector
  only. There is no Line inspector in it — no Thickness, Fill, Length or orientation control, and no
  edge checkboxes. Its POSITION and BOX sections are therefore the *reference for section framing and
  field layout*, not a picture of the controls this story adds. The controls above are specified by these
  acceptance criteria and by the vocabulary contract from Story 14.1; they are not transcribed from a
  drawing, and no agent should look for one.

**Acceptance Criteria:**

**Given** a selected Line
**When** the inspector is shown
**Then** it offers Thickness, Colour, Length and an orientation — mapped onto the element's height,
background and width by the panel, with no new field, no new command and no change to what is
serialized

**Given** a selected Line
**When** the inspector is shown
**Then** the border controls and the four edge checkboxes are not offered, because a filled bar has
no edge set to draw

**Given** a selected Rectangle
**When** the inspector is shown
**Then** its background is labelled Fill, and its border and edge controls remain, because on a rect
they are what they say

**Given** a Line or Rectangle authored before this story, or hand-edited to a shape the panel's
vocabulary does not describe
**When** it is selected
**Then** the panel shows the engine's committed values without rewriting them, and never normalises
a document on selection

### Story 14.3: A placed component is the selected component

As a template author,
I want what I just placed to be selected and easy to grab,
So that placing a component and editing it is one gesture rather than a hunt.

**Covers:** FR1, FR4 · UX-DR18, UX-DR25
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the palette rail and the canvas selection treatment

**Acceptance Criteria:**

**Given** a component placed from the palette
**When** it is dropped
**Then** it is the selection, and the inspector shows its properties — rather than the page setup
panel and the words "Component properties require a selection"

**Given** a thin component — a 1pt Line, or any element under the comfortable hit size
**When** the author clicks near it on the canvas
**Then** it is selected: the hit target is padded independently of the drawn thickness, and the
padding never changes what is drawn or what is stored

**Given** two thin components whose padded hit targets overlap
**When** the author clicks in the overlap
**Then** the topmost in document order takes the selection, deterministically, and repeated clicks
in the same spot do not alternate

**Given** the keyboard
**When** a component is placed
**Then** focus lands where the author can act on it, to the same floor UX-DR25 sets elsewhere

### Story 14.4: The panel offers no control the engine will refuse

As a template author,
I want the panel to stop inviting me to do things that cannot be done,
So that a dead end is not something I discover by hitting it.

**Covers:** FR5, FR7 · UX-DR24
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the inspector's section stack
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Binding.dc.html`
  — the DATA tab a binding invitation points at

**Acceptance Criteria:**

**Given** a selected Line, Rectangle, Image or Table
**When** the inspector is shown
**Then** the BINDING section either is not shown or states that only text components take a scalar
binding — it never invites the author to pick a path the command will refuse

**Given** the Data panel with a non-text component selected
**When** a path is picked
**Then** the reason it cannot be connected is stated before the attempt, not after the engine
rejects it

**Given** a Table
**When** the inspector is shown
**Then** its binding is stated once, where it is editable, rather than displayed as read-only text
in one section and edited behind a button in another

**Given** any control this story hides for a component kind
**When** that kind's document already carries the underlying value
**Then** the value is preserved on save untouched — the panel declining to author a field never
removes it

### Story 14.5: The product wears its own mark

As a template author,
I want the designer to carry the mark the design gives it,
So that the application looks like the product it was designed to be.

**Covers:** UX-DR1, UX-DR2, UX-DR3
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Main.dc.html`
  — the mark at 18px in the document bar
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Load.dc.html`
  — the same mark at 22px, and at 13px as a manifest row bullet
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md`
  — the colour token the mark is drawn in

**Acceptance Criteria:**

**Given** the document bar, which today shows the word FOLIO alone
**When** it is shown
**Then** the mark sits before the word: a square outline containing a smaller solid block, centred —
the mark `Main.dc.html` draws at 18px with a 1.5px stroke

**Given** the mark's colour
**When** it is implemented
**Then** it comes from `--color-select`, the token whose value is already the design's `#58a6c4`, and
no hex is written anywhere in the app (UX-DR1)

**Given** the same mark drawn at **two** sizes across the mockups — 22px on the load screen and 18px in
the document bar
  <!-- CORRECTED 2026-09-09. This Given previously read "three sizes ... 13px as a manifest row bullet".
       DESIGN.md:436-438, the normative Rules section, states the mark has two sizes and adds
       "Nowhere else." The 13px shape in Load.dc.html:78-80 is the CJK row's IN-PROGRESS MARKER, one
       value of a three-value status vocabulary beside a green tick (:41) and a grey dash (:96) -- not
       a brand instance. Measured, the three drawings are not geometrically similar: inner-width over
       outer runs 0.333 / 0.318 / 0.385, and the 13px inner block is square where the mark's is
       portrait. The epic had read a status glyph as a third size of the logo. -->
**When** it is built
**Then** it is one component parameterised by size rather than a separate drawing per site, and it is
used on the load screen as well as the document bar, because a brand that appears only after loading
is not the thing the author first sees
  <!-- "three drawings" corrected to "a separate drawing per site" 2026-09-09, with the Given above.
       The first pass corrected the count in the Given and left it standing in the Then, which is the
       failure mode recorded at D-14.4.1: a correction is not finished until everything citing the
       corrected text has been swept with it. Found by 14.5's builder, not by me. -->

**Given** the mark
**When** it is read by assistive technology
**Then** it is decorative beside the word FOLIO and is not announced twice — the accessible name of
the pair is the product name, once

### Story 14.6: The DATA tab is the binding panel the design drew

As a template author,
I want the data panel to show me the values I am binding to,
So that picking a path is recognising my data rather than decoding a type name.

**Covers:** FR7, FR9 · UX-DR2, UX-DR7, UX-DR13, UX-DR24, UX-DR25
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Binding.dc.html`
  — the whole DATA tab: file row, selection context bar, PATHS tree with values, TABLE ONLY and RUNTIME badges

**Acceptance Criteria:**

**Given** each scalar path in the tree
**When** it is shown
**Then** its **value** is shown beside it, right-aligned against the label — `customer.name` reads
`สมชาย วงศ์ประเสริฐ`, `statement.openingBalance` reads `48250.00` — rather than the label, the full
path and a concatenated `kind · count · preview · candidate` string, which is what the panel renders
today. `SampleNode` already carries `preview`, `count` and `kind`; this is a presentation defect, not
a missing model

**Given** an object node
**When** it is shown
**Then** it is marked `{ }` beside its name, and a collection `[]`, rather than spelled out as the
words "object" and "collection"

**Given** the loaded file
**When** the panel header is shown
**Then** it names the file with its **size** — `sample-statement.json · 18 KB` — which `SampleData`
can already measure from the bytes it holds

**Given** a selected component
**When** the DATA tab is shown
**Then** a context bar above the tree states what is selected and what a pick would bind — *"Text
selected · binding to string"* — in the bind accent, so the panel answers "what will this do" before
the author picks rather than after the engine refuses (UX-DR2, UX-DR24)

**Given** a collection such as `transactions[]`
**When** it is shown
**Then** it carries a **TABLE ONLY** badge and one line of prose — *"Collection · 34 items. Text
cannot bind a collection."* — and its row-scope children are rendered dimmed and unpickable, because
they are bound in the table editor and not here

**Given** the runtime parameters the engine discovered in this template
**When** the tree is shown
**Then** they appear under `params` with a **RUNTIME** badge and their current values, visible but
never pickable — `bindComponentScalar` already refuses `params` as a root data binding, and the panel
should show the author that the namespace exists rather than leaving it invisible until Preview

**Given** the design shows no connect control beside the tree, while the panel today commits through
an explicit "Connect selected path" button
**When** this story is planned
**Then** the interaction is ruled and recorded before it is built: either a pick binds immediately
(undoable under UX-DR20, and the engine still validates and can still refuse), or the explicit commit
stays and the mockup's omission is a gap in the mockup. The row treatment the design draws — the
picked row carried on a bind-accent left bar — is adopted either way

**Given** the document's components
**When** the status bar is shown
**Then** it states how many carry a binding — *"7 of 10 elements bound"* — counted from the engine's
own projection, which already reports a `binding` per component, so no new engine surface is added

**Given** the panel with no sample data loaded
**When** it is shown
**Then** it says so in the design's own terms and states that sample data is never written into the
template — the footer note the mockup carries, which the panel does not say anywhere today

**Given** every value, badge and dimmed row this story adds
**When** it is read by assistive technology or operated by keyboard
**Then** the tree keeps the roving tab stop and arrow-key navigation it already has, a badge is
announced as part of its row rather than as a separate control, and an unpickable row reports that it
is unpickable rather than simply failing to respond (UX-DR25)

### Story 14.7: The table editor is the matrix the design drew

As a template author,
I want the table editor to be the column matrix the design specifies,
So that configuring a table is reading a table rather than filling in eleven fields per column.

**Covers:** FR10 · AD-13, AD-15 · UX-DR8, UX-DR18, UX-DR24, UX-DR25
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/TableEditor.dc.html`
  — the six-column matrix, the width budget, and the dialog's footer summary

**Acceptance Criteria:**

**Given** the matrix, which today carries eleven columns per row — Header, Width, Cell alignment, Row
field, Footer, Footer source, Footer format, Move earlier, Move later, Remove, Add after
**When** it is rebuilt
**Then** it carries the six the design draws — `#`, HEADER LABEL, BOUND FIELD · row scope, WIDTH,
ALIGN, FOOTER AGGREGATE — with reorder and remove as row affordances rather than as four more
columns, because UX-DR8 calls this the densest UI in the product and density is what the extra
columns destroyed

**Given** the ALIGN cell
**When** it is shown
**Then** it is the three-segment L / C / R control the design draws, which is the same segmented
control the inspector uses for alignment — one alignment control in the product, not a segmented one
in the panel and a `<select>` here

**Given** FOOTER AGGREGATE, which today is a `<select>` plus a Footer source field plus a Footer
format field
**When** it is shown
**Then** it is one control reading `none` or the aggregate, with source and format revealed only when
the chosen aggregate needs them — the engine's rules are unchanged (`count` takes no source; an
absent footer takes neither), and this story only stops presenting three fields where the design
presents one

**Given** the columns' widths
**When** the matrix is shown
**Then** the width budget is shown as the design shows it — `Σ 174.0 of 174.0 available`, with the
`exact` badge when they meet — so an author sees a table that does not fit before rendering it rather
than after

**Given** the table's scope — its collection, its item count in the loaded sample, and its band
**When** the editor is opened
**Then** they are read-only context as the design draws them (`transactions[] · 34 items in sample ·
band: content`), and the collection and row alias stay editable in a labelled, restyled group rather
than as two bare inputs at the top of the dialog
  <!-- CORRECTED 2026-09-10, before Story 14.7 was built. This read "edited where the design puts
       them". Measured against the mockup: `TableEditor.dc.html` draws the collection as a plain
       <span> with no input chrome, and draws NO row alias anywhere at all -- the only trace of row
       scope in the file is a dimmed `· row scope` sub-label in the BOUND FIELD column header. So the
       design puts them nowhere, and the AC had no referent.

       The design is SILENT here, not contradictory. AC5's real complaint is presentational -- "two
       bare inputs at the top of the dialog" -- and a labelled, restyled group answers it. The
       editable site itself does not move: DW-351 records TableEditor.tsx as the ONLY editable site
       for a table's collection in the product, and Story 14.6 has just begun routing authors to it.
       Moving it in the same story that rebuilds everything around it was the larger risk. -->

**Given** the dialog's footer
**When** it is shown
**Then** it states the summary the design states — `5 columns · 2 aggregates`

**Given** the design's **Cancel / Apply** buttons, against an editor that today commits every cell on
blur through the engine
**When** this story is planned
**Then** the transaction model is ruled and recorded first. **This is not a labelling choice.**
  <!-- CORRECTED 2026-09-10, before Story 14.7 was built. This AC previously asserted that a modal
       Cancel "would need a local uncommitted buffer — a second model of the document, which AD-15
       exists to forbid", and offered a two-option fork: Close-plus-undo, or the buffer.

       BOTH PREMISES ARE FALSE, measured at HEAD.

       (1) AD-15 does not forbid a buffer. Its rule reads: "Transient interaction state — a drag in
       flight, a resize preview, AN UNCOMMITTED PROPERTY KEYSTROKE — lives in the UI and never enters
       the document." A pending edit not yet sent is the named permitted case, not the forbidden one.
       What AD-15 forbids is a TypeScript reimplementation of the .folio SCHEMA. The product already
       ships a buffer behind an Apply button: PageSetup takes a `draft` and commits on `onApply`.
       Leaving the claim standing would have refused the next author of a buffer on a rule that does
       not say that.

       (2) The fork was incomplete, which is why neither arm was right. The engine's undo is a
       BYTE-SNAPSHOT RESTORE, not an inverse command (`wasm/engine.go` pushUndo(e.bytes); its own
       comment: "They never serialize history into .folio bytes or ask TypeScript to retain a
       mirror/inverse command"), and a no-op command is not a history entry (bytes.Equal short-
       circuit). That fact — stated nowhere in the AC — opens a third option.

       RULED (D-14.7.1): the buttons are Cancel / Done. Commit-on-blur is unchanged, so every refusal
       stays live and located, which is Epic 14's whole subject. The dialog counts only the commands
       that actually changed the document, and Cancel issues exactly that many undos. The UI holds an
       INTEGER, not document content — transient state by AD-15's own definition, and a smaller
       footprint than the draft object PageSetup already ships. Name it honestly in the spec: it is a
       compensating sequence, not a transaction. -->

**Given** the design's widths in **millimetres**, against an engine and an inspector that speak
points
**When** this story is planned
**Then** the unit is ruled **product-wide** rather than for this dialog alone — the design shows mm
here and `210 × 297 mm` in the document bar, while every panel in the product shows pt, and one
product does not carry two length units. Whatever is ruled, the stored value is unchanged: the format
is millipoints and stays millipoints

**Given** the matrix
**When** it is operated by keyboard
**Then** the roving grid navigation it already has survives the rebuild — this story reduces the cell
count, so it must not reduce what a keyboard can reach (UX-DR25)

### Story 14.7b: The table editor's Cancel discards what it counted

As a template author,
I want the table editor's Cancel to undo the editing I did in it,
So that opening the dialog to try something is not a commitment I have to unpick by hand.

**Covers:** FR10 · AD-15 · UX-DR20, UX-DR24, UX-DR25
**Split from Story 14.7 on 2026-09-10.** 14.7 rebuilds the matrix, which is presentation like every
other story in this epic. This is not presentation: it is an interaction-and-history model carrying a
named destructive failure mode, and bundled into a six-AC markup rebuild the review attention it needs
would compete with grid tracks and CSS specificity — the mechanism behind this epic's false greens.
**The transaction model is already ruled in full: see D-14.7.1, including its six guardrails.**

**Acceptance Criteria:**

**Given** the dialog, whose edits commit on blur through the engine
**When** its footer is shown
**Then** it carries **Cancel** and **Done** — not the design's Cancel/Apply, because nothing is applied
at close and a label must follow its model (D-14.7.1)

**Given** the author's edits in an open dialog
**When** Cancel is pressed
**Then** the dialog issues exactly as many `undo` operations as it counted commands that **changed the
document**, and closes — a compensating sequence replaying the engine's own byte snapshots, not a
transaction, and the spec must say so

**Given** a blur that changes nothing
**When** it commits
**Then** the count does not move — a no-op is not a history entry in the engine, and counting one would
make Cancel unwind edits from **before the dialog opened**, which is this design's only destructive
failure mode

**Given** the open modal
**When** the author presses the global undo or redo shortcut
**Then** the modal swallows it — today it does not, and the document mutates behind an `aria-modal`
dialog (DW-368), which under this model also desynchronises the count

**Given** an edit count that exceeds the engine's history limit
**When** Cancel is offered
**Then** it is disabled with a stated reason rather than silently under-unwinding

**Given** an undo in the sequence that fails
**When** it returns an error
**Then** the dialog stops, states the actual position, and never claims a discard that did not complete

**Given** a Cancel the author did not mean
**When** it has closed the dialog
**Then** the discarded edits remain **redoable** — the redo stack is not cleared, and this is stated
rather than left to be discovered

### Story 14.8: The table editor carries the header, cell and border sections

> **OWNER DECISION, 2026-09-05 — this story is now a RESTYLE, and the mockup is being corrected.**
> `TableEditor.dc.html` draws five controls. Measured against the format and the rulings since this
> story was written: **`Padding`** was ruled out of the panel (D-12.4.1); **`Row height`** and **`Repeat
> on continuation pages`** are facts the engine *derives*, so a control would only restate them; and
> **`Show header row`** and the **borders preset** have **no field in the file format at all**, which
> this story's own AC4 forbids inventing. Meanwhile the three things Story 12.3 makes authorable —
> header height, alternating-row colour, header text style — **are not drawn in that mockup.**
>
> **The owner chose to correct the drawing rather than build it.** A mockup that draws capabilities the
> product does not have is a drawing to correct, not a specification to implement. So:
> **14.8's remaining substance is presentation** — take the controls Story 12.3 ships in the existing
> editor idiom and group them into the drawn HEADER / CELLS / BORDERS sections, so the table editor
> reads as one designed thing, which is Epic 14's actual subject. **`Show header row` and the borders
> preset are not coming**, and the mockup is to be edited to stop promising them. **No format change,
> no version increment.** Rewrite the acceptance criteria against this before dispatch.


As a template author,
I want the table's header, cell and border settings beside its columns,
So that a table is configured in one place rather than in a dialog plus a hand-edited file.

**Covers:** FR10, FR60 · AD-13 · UX-DR8, UX-DR24
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/TableEditor.dc.html`
  — the HEADER, CELLS and BORDERS sections

**Acceptance Criteria:**

> **REWRITTEN 2026-09-10**, as the owner-decision block above instructs in its own last line. The
> criteria that stood here until now still specified `Show header row` and the BORDERS presets that the
> block had already ruled out, so the story contradicted its own decision and could not be dispatched.
> **The owner also amended the block itself on 2026-09-10**: BORDERS authors `headerStyle.border` —
> see the final criterion, which is capability rather than presentation and is admitted knowingly.

**Given** the editor's single `HEADER AND ROWS` group — `div.table-editor-header`, carrying `role="group" aria-label="Table header and rows"`, which is how to find it and **not** a line number ([D-14.7.4]'s neighbours: the anchor written here as `TableEditor.tsx:427` on 2026-09-10 rotted to `:448` the same day when Story 14.7b moved the file by +22 lines; **re-measure it, never quote it**) — which today holds everything Story 12.3 made authorable in one undifferentiated block
**When** it is restyled
**Then** it becomes the design's **HEADER** and **CELLS** sections — HEADER carrying the header height and the header text fields (font family, size, line spacing, background, colour, vertical align, align), CELLS carrying the alternating row background, which is the one field in the set scoped to data rows rather than to the header. No control is added, removed or rewired: this is a regrouping, and the story is done when the editor reads as one designed thing rather than when it can do something new

**Given** every control in those two sections
**When** each is placed
**Then** it is the control already shipped, sending the command already registered — `table.headerHeight`, `table.altRowBackground`, and `table.headerStyle.<field>` over the closed set — the `tableHeaderStyleFields` declaration in `folio-go/component_commands.go`, named rather than numbered because the line moves; `:2511-2518` cited below is the ruling **comment** above it, not the set — — so **the HEADER and CELLS sections add no field, no command and no projection**, and offer no second way to store a value the product can already store (D-12.4.1, D-14.4.Q2(a)). The **one** addition in this story is the BORDERS criterion below, ruled in by the owner on 2026-09-10; **no format field changes and no version increments** anywhere in the story

**Given** *Repeat on continuation pages*, which the design draws with a **REQUIRED** badge
**When** it is shown
**Then** it is a locked statement of what the engine does rather than a setting the author can change — the header is redrawn on continuation pages and no format field switches that off. **It must not be worded as an absolute.** `DiagCodeTableHeaderRepeatSuppressed` (`diagnostic.go:317-325`) is the engine's own record of the one case where the repeat is suppressed on a single page, because reserving the header would leave no room for a row to sit under it. A badge promising a guarantee the engine can suspend is the same defect as a control the engine will refuse, arriving from the other direction

**Given** *Row height*, which the design shows as `auto`
**When** it is shown
**Then** it likewise states a derived fact — a row's height comes from its content and its cell padding, and no format field sets it — rather than offering a number the engine would ignore

> **CORRECTED TWICE ON 2026-09-10**, both times to mechanism the engineering lead ruled before it was
> measured. The **first** correction replaced "the closed field set goes to ten" and "the projection by
> one block" with the flat twelve-member shape below. The **second** left that verdict standing and
> **withdrew the reason given for it**: the stated ground was that a nested record would be the
> projection's first of kind with no local example, and `engine-protocol.ts:517` falsifies that —
> `table.columns` already validates a ten-key nested sub-record with `isRecord` + `hasExactKeys` +
> per-field typed clauses in the same expression. The shape stands on two better grounds, given in the
> criterion itself. **The capability the owner ruled in on 2026-09-10 is unchanged throughout; only the
> mechanism moved.** Recorded this way deliberately: *a ruling corrected twice by measurement is worth
> more than one that looks stable because nobody re-checked it.*

**Given** `headerStyle.border`, a block the header cascade already resolves ahead of the table's own border (`table_render.go:358-361`) and which the closed command set deliberately withheld pending this very section — `component_commands.go:2511-2518` says in the engine's own source that `border` *"waits on Story 14.8's BORDERS section"*
**When** BORDERS is built
**Then** it authors that block's three attributes as **three flat members** of the closed set, taking it from nine to **twelve**, and `TableColumnsProjection` gains **six flat scalar keys** — width, colour and edges each with the `Resolved` twin every one of the nine incumbents already carries. **The shape is flat for two reasons, neither of them key count.** First, the command surface is one field and one operation per message, so a composite value could only ever be written *whole* — every border edit would re-transmit colour and edges read back from the projection, making the panel author values the author never chose, which the next criterion forbids outright. Second, the element path already authors this same nested block from three flat keys and materialises it on the first sub-key written (`component_commands.go:1082`, `:1139`, `:1457-1459`); a composite value would be a second, contradictory answer to a question this file has already answered. Each member's name must make its refusal path name a key the document actually has (**D-000.25**) — mirror whatever mapping the element path uses for its own three keys, measured rather than assumed. `edges` is projected as a **canonical comma-joined string** in the order `top,right,bottom,left`, empty for none, so the projection stays entirely flat scalars and the twin comparison is string equality rather than a set comparison; on the wire it remains an **array**, as `borderEdges` already is. The three-way None / Horizontal / All preset is still **not** built, because it still has no field. A table's *body* border stays where it already is — the inspector's **BOX** section (`App.tsx:3074`) — so this section adds the header-row override and does **not** restate what BOX already authors (D-14.4.Q2(a)). **No change to what the renderer draws, and no change to `internal/template/` or any version constant:** *(amended 2026-09-10 — this clause read "no parse, serialize or render code changes", which as written **forbade the change the criterion itself required**: the three defaults the disclosure criterion must share with the renderer were literals inlined in `table_render.go`, so satisfying one clause breached the other. It meant "do not change what is drawn"; it said "do not touch the file". **Refactoring within the render path is permitted where it serves the single-source requirement, and the proof is the golden corpus rendering byte-identically, not the diffstat.**)* the field already exists, already round-trips byte-identically, and already paints. **This criterion is capability rather than presentation.** The owner ruled it in on 2026-09-10, knowing it reverses the restyle half of their 2026-09-05 decision and against a recommendation to split it into its own story. The reversal is theirs and is recorded as theirs

**Given** an author setting one of the header border's three attributes for the first time
**When** the command commits
**Then** **only that attribute is written** — the panel never seeds the block from the table's border, because authoring one value must not make the panel author three the author never chose (I-5, D-12.4.1: the panel authors what the author authored) — and the two it did not write are **shown resolved** beside it, in the `<output>` idiom the nine sibling fields already use, reading the engine's own defaults of **0.5pt, `#000000` and all four edges** (`table_render.go:595-612`). Those defaults are **projected from Go**, never mirrored in TypeScript: the canvas already mirrors them at `App.tsx:5038`, and a third copy is a third thing to drift. A twin that echoes what the author just typed asserts nothing

**Given** the header border cascade, which is **block-granular** — `table_render.go:358-361` takes a present, non-null `headerStyle.border` **entirely** (`r.hasBorder, r.border = true, header.Border.Value`) and does **not** merge the table's own `style.border` into it field by field — and given that writing any one sub-key **materialises the whole block** (`component_commands.go:1457-1459`)
**When** the first attribute is authored
**Then** the panel says so **in words**, not only by showing three resolved numbers. The author is learning that the header border has **stopped inheriting the table's altogether**, which is a different fact from what it currently draws, and it is the fact the block-granular cascade makes irreversible without an explicit clear. **This is the reason `border` was withheld from the closed field set** (`component_commands.go:2511-2518`, which calls it "a nested block the cascade treats block-granularly") and it is this story's principal correctness risk: a control that looks like it edits one attribute and in fact replaces a block is the same class of defect as a control the engine will refuse, and it is **not visible from the field's name**

**Given** the last of the three attributes being cleared
**When** the command commits
**Then** **no `headerStyle` key remains** in the serialized document. `cleanupEmptyHeaderStyle` (`:2829-2838`) counts `style.Border.Set` at `:2835` but carries **no empty-`Border` collapse**, unlike its element twin `cleanupEmptyStyle` (`:1485-1490`) — so without the mirror, clearing the last sub-field leaves `border: {}` and pins `headerStyle` alive forever by a block that means nothing, which `:2503-2509` says the design refuses. This is **not a cost of the flat shape but the completion of the mechanism it follows**: flat keys + materialise-on-write + collapse-on-clear is one coherent three-part mechanism at the element level, and `cleanupEmptyHeaderStyle` carries two of the three parts only because nothing could reach the third until this story admits the field. It ships with a test that **fails without the mirror**, red-proved by deleting the collapse rather than by changing it

**Given** `TableEditor.dc.html`, which draws `Show header row`, a three-way BORDERS preset (None / Horizontal / All), `Padding` and `Row height` as settings the product does not have
**When** this story is done
**Then** the mockup itself is edited so it stops promising them. The drawing is part of this story's deliverable and not context for it: a mockup that draws capabilities the product cannot carry is how this epic's defects were made, and leaving it uncorrected leaves the next author a specification to implement

**Given** any section this story cannot back with a field the engine actually consumes
**When** the editor is shown
**Then** it is absent rather than disabled-and-mysterious, and this epic records why

> **THE STRUCK PADDING CRITERION IS RETAINED BELOW DELIBERATELY.** It is not live scope. It is kept
> because it is the record of *why* padding is not in this panel, and because it is the clearest
> worked example this epic has of a criterion every clause of which is true about the engine and which
> is still an instruction to build a defect. Deleting it would remove the warning and leave only the
> absence, which is how it survived four stories the first time.

> ~~**Given** cell padding in this panel · **When** it is edited · **Then** it writes the table's
> `style.padding`, which the cell chrome already consumes — the one element kind for which padding is
> not in question (Story 12.4)~~
>
> **STRUCK by D-12.4.1 and the owner's 14.8 decision above.** Found unstruck during Story 12.4's late
> close, four stories after the ruling that killed it. **It survived because it is right about the
> engine and wrong only about the panel** — padding on a table *is* still an accepted command, so
> every clause here reads as true, and the single false premise is the unstated one: that the panel
> should offer it. D-12.4.1 says the panel never authors padding, on a table or anywhere else.
>
> **This is DW-203's predicted failure mode, written down as an instruction.** A padding control
> offered only when a table is selected would satisfy the engine's grant and red nothing — no gate in
> the project would catch it, because the command it sends is legal. The only thing standing between
> that defect and the tree was this paragraph, and it was still telling someone to build it.


### Story 14.9: The canvas draws the table it will print

As a template author,
I want a table on the canvas to show its columns and its bindings,
So that I can see the table I am building without opening a dialog or rendering a preview.

**Covers:** FR1, FR4, FR10 · AD-15, AD-17 · UX-DR7, UX-DR10, UX-DR21
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Binding.dc.html`
  — the table component as the canvas draws it — the collection chip, the header labels, and the one specimen row

**Acceptance Criteria:**

**Given** a table component, which the canvas today paints as a small box containing the word "Table"
**When** it is drawn
**Then** it is drawn as `Binding.dc.html` draws it — a chip along its top carrying the table glyph,
the bound collection in the bind accent (`transactions[]`) and the column count on the right
(`5 columns`); beneath that the real header labels (Date, Description, Debit, Credit, Balance); and
beneath that **one** representative row

**Given** the representative row
**When** it is drawn
**Then** each cell shows its column's binding as the placeholder it is — `{{date}}`, `{{debit}}` — in
the bind accent, because amber means data and only data (UX-DR2), and one row is drawn rather than
the sample's thirty-four, since the canvas shows structure and the preview shows the document

**Given** each column
**When** the header and the representative row are drawn
**Then** the column's declared width and its alignment are honoured, so a right-aligned money column
reads as right-aligned on the canvas exactly as it will print

**Given** the canvas projection, which today carries only `tableBind` — one string — for a table
component, while the columns live in the separate projection the table editor opens
**When** this story is built
**Then** the canvas projection carries the table's columns too: label, width, alignment and binding,
from the engine, so the canvas paints from the engine's own projection and never from a
browser-side model of the table (AD-15, AD-17). This is a new **projection** field and not a new
format field; nothing about the document changes

**Given** a table with no columns yet
**When** it is drawn
**Then** it says so on the canvas in the design's own terms rather than drawing an empty frame that
looks like a rendering failure (UX-DR13)

**Given** a column whose binding is not set
**When** the representative row is drawn
**Then** that cell reads as unbound rather than as an empty string, so an unfinished table is
visibly unfinished

### Story 14.10: A table column is bound from the main window

As a template author,
I want to bind a table column the same way I bind everything else,
So that binding lives in one place instead of behind a dialog.

**Covers:** FR7, FR10 · AD-15 · UX-DR7, UX-DR18, UX-DR24
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Binding.dc.html`
  — the canvas table and the DATA tab that offers its row-scope fields
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/TableEditor.dc.html`
  — the matrix's BOUND FIELD column, which this story rules as display

**Acceptance Criteria:**

**Given** a table drawn on the canvas with its columns visible (Story 14.9)
**When** the author clicks a column
**Then** that column is the selection, and the inspector and DATA panel address it — selecting a
column is how binding one begins, rather than opening the table editor to reach it

**Given** a selected table column
**When** the DATA tab is shown
**Then** the row-scope fields under the table's bound collection become **pickable** — the same
fields Story 14.6 dims when a text element is selected, because for a column they are exactly the
right paths — and the context bar names what is selected and what will be bound

**Given** a picked row-scope field
**When** it is connected
**Then** it commits through the existing `updateTableColumnBinding` command, unchanged: the engine
still owns the document, still validates, and can still refuse, and the refusal is presented where
the pick was made

**Given** the collection itself, or a path outside the table's row scope
**When** a column is selected
**Then** it is not pickable for that column, and the panel says why rather than letting the engine
refuse after the fact (UX-DR24)

**Given** `TableEditor.dc.html`, whose matrix carries a BOUND FIELD column
**When** the two surfaces are reconciled
**Then** the table editor keeps **showing** each column's bound field as context, and the ruling this
story records is where it is **edited** — the owner's instruction is that binding happens in the main
window, so the editor is for structure (add, remove, reorder, width, align, footer aggregate) and the
matrix's BOUND FIELD is **display-only**. The owner ruled this on 2026-09-07 (D-14.10.1 in
`epic-11-14-decision-log.md`); the clause is settled and is not an open question for this story

**Given** a table column bound this way
**When** it is undone
**Then** it undoes as one step, like every other binding (UX-DR20)

## Epic 15: Folio can be released

Every epic above assumes a tree someone can ship. Today that assumption does not hold, and the
reasons are process rather than product.

**The golden report's hash moved and nobody had said why.** *(Written before Story 15.1 ran; kept in
the past tense it now belongs in, with the current state stated after it.)* The cross-target matrix
was red on `main`, and red on **all four** golden statements — `statement-1`, `-5`, `-20` and `-50` —
each by exactly **+4 bytes per page** (+4 / +20 / +80 / +200). `statement-1` — the Customer Account
Statement, the primary acceptance fixture — produced `114df1d6…` against a then-recorded
`ef58bbf6…`. The four targets agreed with each other throughout, so byte identity across
`darwin/arm64`, `linux/amd64`, `linux/arm64` and `js/wasm` never lapsed and CAP-13 stayed intact;
what had failed was that a golden moved unexplained. Counter-metric C6 makes that a defect until
proven to be an intended versioned change.

*Attributed 2026-08-30 by Story 15.1 (D-15.1.1): **Epic 9 alone** is implicated, and not through its
element-box paint. `791ed00` also created `folio-go/text_alignment.go` and wired `style.align` into
the emitter, moving one `Tm` x-operand on the page-footer element that repeats on every page. Epic 10
(`304442f`) moved nothing — its text-ink bracket emits zero bytes when no colour is declared. The
move is ruled intended (D-R7.6): the pre-`791ed00` output drew right-aligned text at its box's left
edge. The goldens are re-recorded and the human sign-off is re-attested rather than patched.*

*Current state, so the paragraph above is not read as live: `114df1d6…` **is** now `statement-1`'s
recorded digest, and all four targets agree on it and on the other three. The matrix is still red,
but for a different and intended reason — the human sign-off is pending re-attestation, which
Story 15.1 halted on deliberately rather than fabricate. There is no longer an unexplained hash.*

**CI's red cannot be read.** The guardrails workflow contains `folio-go-known-red`, red by design so
DW-11's unmet floor stays visible, which makes the whole workflow permanently red and camouflages
any genuine failure beside it. DW-23 records the consequence already realised: a gofmt break in
`lint/` has been in the tree since Story 5.10 and survived two boundary gates, because the local
gate procedure ran gofmt in `folio-go` only and the gate read the workflow badge rather than the
per-job conclusions.

**There is no release.** `version.go` reads `0.0.0-dev`, `folio-go/v0.1.0` has never been cut, and
RELEASING.md says of itself that version stamping, changelog policy, the tag command and how a
matrix result is recorded against a release are simply unwritten. The owner's recorded decision is
to cut after Epic 6; Epics 7–14 all postdate that decision without amending it.

This epic ships no feature. It is the difference between a repository and a product.

**FRs covered:** none — this epic exists to make the others releasable
**Also lands:** AD-21, AD-22 and AD-26 enforced rather than extended; C6 and DW-23 discharged; DW-4's
remaining engineering-lead checkpoint closed

### Story 15.0: A catalogue face arrives when it is picked

As a template author,
I want the catalogue's families to download the first time I pick one,
So that the release I install is not carrying thirty-one typefaces I may never use.

**Covers:** NFR7, AD-19 · D-8.4d.1 as amended by D-16.1 · sequenced 15.0 → 8.4d → 15.3 (D-8.4d.2)

**Read this before the acceptance criteria.** This story was created on 2026-09-02 and three epics have
shipped since. Most of what its originating decision describes now exists, and the parts of that decision
quoted in older records have been **struck**. Build against this boundary, not against D-8.4d.1's text.

- **What is true today, and the second consumer is the one that bites.** The catalogue's **31 families
  are bundled and precached** — `brotli.catalogue.totalBytes` is **3,151,569** of a **16,850,005**-byte
  first load, **18.7%**. The precache slot budget (`maximumCacheAssets = 64`) is self-imposed, and its
  remaining margin is real *only because this story has not landed*. **But the bytes have two consumers,
  not one.** `scripts/build-wasm.mjs:410` also emits **one `@font-face` rule per catalogue face** into
  `runtime-fonts.css`, `src: url('./runtime/<filename>')` — 31 rules. Its own comment at `:340-346` says
  the catalogue faces *"reached Vite only through the `url()` in the emitted stylesheet. They still do
  for the CSS"*. **Unprecaching the bytes without removing those rules leaves 31 `@font-face`
  declarations pointing at URLs that are no longer on the machine**, and a `@font-face` rule fetches
  lazily — at the moment its family is used to paint, which under AD-17 is exactly what the canvas does.
- **What Epic 16 already built, and this story must not rebuild.** A face arrives from the web with its
  terms attached (16.1), stays on this machine once fetched (16.2), is offered by a family control that
  names three sources (16.4), and is installed without being embedded (16.5). **The fetch tier exists.**
  This story does not invent fetching; it moves the catalogue onto machinery that is already shipped.
- **What is already amended, and must not be amended again.** `SPEC-fonts` `## Non-goals` and
  `font-catalogue.md` have **both** been struck and annotated for the live-source reversal. The
  paired-amendment obligation D-8.4d.1 created is **discharged**.
- **What this story changes.** The catalogue's 31 families stop being bundled and precached, and are
  obtained on first pick instead — **and the 31 emitted `@font-face` rules go with them.** Catalogue
  faces are registered at runtime by the same `FontFace` path that carried and fetched faces already
  use.
- **The accepted cost is inherited, and its weight has changed even though its truth has not.**
  D-8.4d.1 accepted *"you cannot pick that family right now"* for a curated palette of 21. The same
  sentence now covers 31 families plus Epic 16's open library, so it describes the ordinary state of
  most families rather than a rare inconvenience. **The mitigant is unchanged**: the three shipped Noto
  faces are the coverage (D-8.5.1) and a face already embedded travels inside the `.folio` (Story 8.6),
  so an unfetched family never degrades to a document that will not render.

**Design:** `ux-designs/ux-folio-2026-08-23/mockups/Font Browser.dc.html` — no new surface. The catalogue
group is already drawn and already shipped by Story 16.4; only where its bytes come from changes.

**Acceptance Criteria:**

**Given** a release built with this story
**When** the offline release manifest is read
**Then** the catalogue's faces are not among the precached assets, and `brotli.totalBytes` has fallen by
the catalogue's own subtotal — reported as a measured before-and-after, not as an estimate

**Given** a catalogue family that has not been picked before, and a working network
**When** the author picks it
**Then** its bytes arrive, it is usable, and it stays on this machine for the next session — by the same
path Story 16.2 already established, with no second mechanism introduced

**One mechanism means one path for byte arrival and face registration — it does not mean one provenance.**
The catalogue's curated licence text and its build-time allowlist are a difference in *what is known
about* a face, not in *how its bytes arrive and are registered*. **The catalogue keeps its own
provenance**; only the transport is shared.

**Given** a catalogue family that has not been picked before, and no network
**When** the author picks it
**Then** they are told they cannot pick that family right now, and **no document fails to render** — the
three shipped Noto faces are the coverage, and a face already embedded in a `.folio` travels inside the
file

**Given** a document that already embeds a catalogue face
**When** it is opened after this story with the network blocked or instrumented
**Then** **zero network requests are made for catalogue assets**, and it renders byte-identically to
before — the byte comparison alone proves only the engine-side half, which was never at risk, and would
pass on a machine whose old bundle is still cached

**Given** the release built by this story
**When** its emitted stylesheet is read
**Then** **no `@font-face` rule declares a catalogue family from a build-time URL**, and the collision
guard that keeps a catalogue entry from redeclaring a shipped family — `catalogueFamilies`, seeded from
`shippedFamilies` at `build-wasm.mjs:195` — still covers the hazard in its new form, with its comment at
`:399` rewritten to describe runtime registration rather than a second CSS rule

**Given** the tripwire this story is required to discharge (see below)
**When** the suite runs after this story
**Then** it has been **replaced** by an assertion of the new behaviour — never deleted

**Before this story is dispatched — the tripwire.** A decision whose correctness depends on something not
having happened must carry an executable assertion of that absence, not a prose caveat. A test must assert
that **no path fetches a catalogue face on pick**, keyed on the capability rather than on a proxy such as
a sprint-status value or a filename, and its failure message must name what it invalidates and what must
be re-decided — **including `16-1a-…:911`**, whose measured payload figures are correct only while the
catalogue is precached, so that it is named at the moment the assertion fires and not only in a list
somebody has to go and read. It passes today because the capability does not exist; it goes red the moment this story
lands, and cannot be merged around in silence.

**Discharge list — this story is not done until each is re-taken or amended with the reason:**

1. `font-catalogue.md:182` re-run trigger 2 — the goal-set may be re-derived once the slot constraint
   dissolves.
2. `epic-16-decision-log.md:787` — the 20-slot precache margin, which this story dissolves.
3. `epic-16-decision-log.md:966` — "D-8.4d.1 is a policy with no implementation", which stops being true.
4. `epic-16-decision-log.md:1427` — the warning that a future reader will reverse a decision *"believing
   it was a budget call"*. Amend it with the reason it was actually made.
5. `deferred-work.md:7585` and `:7731`, and `16-1a-…:380` and `:911` — every entry whose figures assume
   the catalogue is precached.
6. `scripts/forbidden-font-hosts.mjs`'s docstring — *"The hosts a bundled-and-precached catalogue must
   never reach for."* **This story does not invert that guard's premise**: Story 16.1 already amended it,
   and the two hosts stay forbidden for a measured reason about what `css2` serves, not because the
   catalogue is bundled. But the docstring still describes the catalogue as bundled-and-precached, which
   this story falsifies. One line.
7. `build-wasm.mjs:340-346`'s comment — *"the pick reads bytes already on the machine and fetches
   nothing."* That stops being true, and it is load-bearing prose sitting directly above the code it
   describes.

**Note for 8.4d, which is unanswerable before this story.** D-8.4d.1 recorded that removing the catalogue
still leaves ~13.5 MB. On today's larger catalogue the figure is **~13.70 MB** — still about 1.5× the
`~9 MB` `epics.md` records, so **the conclusion is unchanged and the number is not**. 8.4d sets its
threshold against the payload this story leaves behind, measured, once.

### Story 15.1: The golden report's moved hash is explained

As a Go developer depending on this library,
I want a moved golden to be investigated rather than regenerated,
So that the regression suite is still a test.

**Covers:** AD-21, AD-22 · C6

**Acceptance Criteria:**

**Given** `statement-1`'s new digest
**When** it is investigated
**Then** the change is attributed to a named commit and a named behaviour — the element-box paint,
the text ink, or something else — by diffing the produced PDF against the recorded one, never by
inspection of the commit log alone

**Given** the attribution
**When** it is recorded
**Then** it names whether the change is intended, and the decision log carries it, before any golden
file is touched

**Given** an intended change
**When** the golden is re-recorded
**Then** every affected fixture is re-recorded in one commit that says what moved and why, and the
matrix is green on all four targets afterwards

**Given** an unintended change
**When** it is found
**Then** it is fixed rather than absorbed, and a fixture is added that would have caught it — because
no document in the corpus exercised the path that moved

**Given** the heavy statement fixtures at 5, 20 and 50 pages, which CI does not run
**When** this story closes
**Then** they have been run, and their digests are recorded, so the four page counts S1 names are
actually verified rather than assumed from the one-page case

### Story 15.2: CI's red means something

As a contributor,
I want a red build to mean a broken build,
So that the signal is worth reading.

**Covers:** AD-21 · DW-23

**Acceptance Criteria:**

**Given** the deliberately-red known-red job
**When** the workflows are restructured
**Then** it lives in its own workflow, so the guardrails workflow's conclusion is green when the
guardrails pass and red only when something is actually broken

**Given** the gofmt break in `lint/`
**When** this story closes
**Then** it is fixed, and gofmt runs across all three Go modules in the boundary-gate procedure as
it already does in CI — a gate that measures fewer modules than CI cannot certify what CI reports

**Given** a boundary gate
**When** it records CI as clean
**Then** it records the per-job conclusions it read, not the workflow badge, and the procedure says
so in the terms DW-23 sets

**Given** the restructuring
**When** it is done
**Then** the known-red job's purpose is unchanged and still visible: it stays red, it still names
DW-11's unmet floor, and its going green is still the surprising event

### Story 15.2b: The font-host scan sees the whole tree

As a developer relying on the font-host scan,
I want it to read the files a story just wrote,
So that "clean" means it looked and found nothing, not that it could not look.

**Covers:** AD-26, I-7 · D-15.2.1 and `epic-11-14-decision-log.md`'s D-000.21 (*"A gate that states
  its own hole honestly"*) — **not** `folio-mvp-decision-log.md`'s D-000.21, which is a different rule;
  `D-000.x` is numbered per log, so a bare cross-log citation names two things

**Dispatch order: alongside Story 12.3, NOT with the rest of Epic 15.** It lives here by subject — this
epic ships no feature and exists to make the others releasable — but every story dispatched before it
carries the hole, so it must not wait for the tail.

**The problem, stated as what the gate cannot say.** `scan:font-hosts` builds its population with
`git ls-files` (`scripts/forbidden-font-hosts.mjs:253`), so it reads **tracked files only**. The script
is honest about this — `:8-13` names *"a host in an untracked file"* as a known hole and words every
message to claim only the scanned population. But because subagents in this run never stage, **every
file a story creates stays untracked for that story's entire life**. Story 12.2 added four such files
and the scan was green over a tree containing none of them. The gate today **cannot distinguish "no
forbidden host found" from "I did not look at the four files you just wrote."**

**What this story changes.** The population is widened to include untracked-but-not-ignored files
(`git ls-files --others --exclude-standard` in addition to the tracked listing), so there is nothing in
the non-ignored tree the scan did not read. **The property being bought is not a bigger number — it is
the collapse of all-clear into couldn't-look.**

**Measured before drafting, so the scope is known:** the build's generated artifacts are each
individually gitignored — `src/generated/runtime/`, `offline-assets.ts`, `runtime-fonts.css`,
`font-catalogue.ts`, `font-index.ts` (`.gitignore:68-75`) — so the emitted stylesheet and the font-index
snapshot, which legitimately carries host-shaped strings, **stay out of the widened population**. The
change is small. The repository currently holds **6** untracked-not-ignored files.

**Acceptance Criteria:**

**Given** a forbidden host planted in an **untracked, non-ignored** file **in a throwaway repository
built in a temp directory** — `git init`, one file committed, a second left untracked carrying the host —
with the scan invoked against that root
**When** the scan runs
**Then** it is **RED** — the existing positive control is over a tracked fixture and would stay green
through a broken widening, so this arm must be red-proved on its own

**The red-proof must be hermetic, and this is a requirement rather than a preference.** After this story
widens the population, *an untracked file containing a forbidden host is exactly what the scan is designed
to catch* — and the red-proof plants one. Run inside this repository it would turn any concurrent story's
scan red for a reason unconnected to that story, and if the prover is ever interrupted the file persists
and poisons every scan afterwards. **A mutating prover must not share a tree with another story's gates**;
contamination of this kind arrives as a plausible intermittent, which is the most expensive way to find it.
A fixture directory inside this repository cannot serve, because the one property under test — being
untracked — is the property a committed fixture cannot have. Invoking the scan against a supplied root is
already precedented by the existing positive control.

**Note the symmetry, because it is the same hazard twice.** This is exactly the *"a red they cannot
explain, in a guard they never touched"* problem this story records for a future contributor — arriving
immediately, between two concurrent stories, rather than months later.

**Given** the widened population
**When** the scan runs on a clean tree
**Then** it is green, and the reported count is higher than the tracked-only count by exactly the number
of untracked non-ignored files scanned

**Given** the exemption list
**When** it is re-checked against the widened population
**Then** every generated artifact that carries host-shaped strings is still excluded, **verified by
measurement rather than assumed** — and the check is recorded, because `.gitignore:68-75` ignores those
artifacts **one file at a time, not by directory**, so the next emitted artifact joins the scanned
population silently unless someone adds a line

**Given** the population floor
**When** the widening lands
**Then** the floor still holds and any test asserting an exact count is updated **deliberately**, with
the new number stated as a decision, rather than adjusted to whatever the scan now reports

**Given** the compensating per-story grep that D-000.21 put in force
**When** this story lands
**Then** it is stood down — it was explicitly time-boxed to this story, so that it has a named end
rather than becoming permanent by habit

**Given** the set of artifacts `build-wasm.mjs` emits and the set `.gitignore` ignores
**When** the build runs
**Then** an assertion proves the **emitted set is a subset of the ignored set**, failing with a message
that names the new artifact and tells the author to ignore it or justify tracking it — with
`pdfjs-assets.ts` **named in the assertion as the deliberate tracked exception**, rather than merely
absent from the emission list

**The forward hazard this story must record, found while scoping it.** `src/generated/` is **not**
gitignored as a directory; its five generated files are ignored **individually**, and one file in it —
`pdfjs-assets.ts` — is tracked on purpose. That per-file pattern is deliberate and it is also a trap:
under a tracked-only scan a new generated artifact was invisible either way, but **under the widened
population an unignored new artifact enters the scan the moment it is emitted.** A comment beside the ignore block is
not enough on its own: **prose discharges only if someone finds it**, and `build-wasm.mjs` knows exactly
what it emits while `.gitignore` states exactly what is ignored — so the relationship is assertable, and
the criterion above asserts it. The comment stays as the explanation; the assertion is what closes it.

### Story 15.2a: A component command means exactly what it names

As an integrating developer,
I want a command that names one component and one property to be incapable of changing another,
So that the engine's guarantee about what a command does is enforced rather than assumed.

**Covers:** AD-16, AD-22 · DW-32 (raised to HIGH 2026-08-31) — ruled into its own story at Story
8.2's plan gate.

**Sequenced before Story 15.3 cuts the tag, and that placement is the deadline.** Its Go half
**narrows** what the exported `ApplyComponentCommand` accepts and has **not** shipped, so by
D-8.2.2's test it joins **D-7.8.3's before-the-tag set — which this story makes two items, not
one.** Putting it in Epic 15, whose stated purpose is *"the difference between a repository and a
product"*, satisfies that deadline **by construction rather than by a promise**: tagging over a
known command-integrity defect is squarely what that epic exists to prevent.

**The defect, measured at `bc671da`.** `rawNumberLiteral` (`folio-designer/src/component-property-command.ts:28`)
splices an author value into command JSON **unquoted**. A value carrying

```
0}},"ids":["other"],"changes":{"width":{"op":"set","value":10
```

produces **valid JSON with duplicate keys**. Go decodes into `map[string]json.RawMessage`, where
**last key wins**, while `componentFields(raw, 4)` still counts four — so **the command mutates a
different component's different property.** Escalation to another command `kind` is blocked only by
an **arity coincidence**, not by any check. The register's earlier claim that no bad value reaches
the document is false.

**BOTH ENFORCEMENT POINTS ARE ONE SUBJECT, and the story must not ship half.** The property is *a
component command means exactly what it names*, and today **neither side enforces it**: the encoder
can produce an ambiguous command, and the decoder resolves the ambiguity silently. Splitting the
halves across stories is the pattern that produced the five untied Go/TS invariants, and the
standing obligation binds it: **an invariant duplicated across the Go/TS boundary moves in one
commit, with a test that reads both sides.**

**And the Go half is what makes the property ASSERTABLE AT ALL.** Without it the only available test
is *"the encoder produces well-formed JSON"* — a test **of the fix**, not of the property, which
goes green again the moment a future encoder regresses. With it, the engine can be handed
duplicate-key bytes directly and asserted to refuse, which is a test **of the invariant** and
survives any encoder.

**Acceptance Criteria:**

**Given** a command object carrying a duplicate key at any level
**When** it reaches `ApplyComponentCommand`
**Then** it is **refused**, rather than resolved silently by last-wins

**Given** an author value containing `}`, `"`, `\` or a brace-bearing payload
**When** the designer encodes a property command
**Then** the command JSON is well-formed and names exactly the component and property the author
edited

**Given** the invariant
**When** it is asserted
**Then** the test hands the **engine** duplicate-key bytes directly — never only the encoder — so
the property survives any future encoder

**Given** both halves
**When** they land
**Then** they land in **one commit**, with a test that reads both sides

**URGENCY, measured rather than assumed — and one condition that changes it.** `rawNumberLiteral`
takes `PropertyIntent['value']`, reached from a properties-panel input on an explicit commit, and
the projection carries numeric fields as JSON **numbers** — so a document value cannot arrive there
as a brace-bearing string. Today's exposure is therefore **keystroke-originated and self-inflicted
in a local, serverless application**: HIGH by mechanism, low by encounter. That is what justifies
*before the tag* rather than *next*.

**The condition this story must MEASURE rather than assume:** if **any** path lets **document
content** reach `rawNumberLiteral` — a value round-tripped as a string, a paste path, a future field
— then a hostile `.folio` mutates arbitrary components on edit, and **this jumps the queue
immediately**. The type signature and the projection's numeric typing were measured; **not every
panel path was.** Treat that as a strong indication to confirm, not a finding to rely on.

**TWO OF THE FIVE ENCODERS ARE NOT MERELY "UNCONVERTED" — THEY CORRUPT NON-BMP TEXT, and this was
found by executing them at Story 8.2's build (2026-08-31).** `component-command.ts` and
`component-asset-command.ts` iterate their input **by code point** but escape from
**`charCodeAt(0)`** — the first UTF-16 unit. So `'a😀b'` encodes as `"a\ud83db"`: a **lone
surrogate**, which Go replaces with U+FFFD. **A bind segment or an asset key therefore binds to a
DIFFERENT PATH than the author typed**, silently. That is a live data-integrity bug in its own
right, not a stylistic inconsistency, and it means this story's scope is larger than "consolidate
five encoders behind one authority": **two of them are actively wrong today**, and the
consolidation is what fixes them. Assert the non-BMP case explicitly — a lone surrogate reaching Go
is the failure, and it is invisible to any test using only BMP text.

**One file-level coordination note.** Story 8.2 makes a **minimal** fix to `quote()` in the same
file — routing it through `JSON.stringify`, because it escapes only `\`, `"`, `\n`, `\r` and
`\t` and JSON requires all of U+0000–U+001F. **This story must re-read that file rather than assume
its earlier shape**, and it owns the shared-authority consolidation that 8.2 was explicitly
forbidden from growing into.

---

### Story 15.3: `folio-go/v0.1.0` is cut

As a Go developer,
I want a version I can depend on,
So that `go get` gives me a fixed public API rather than a moving pseudo-version.

**Covers:** FR36, FR37, FR38 · AD-26 · C5 · DW-4

**Acceptance Criteria:**

**Given** RELEASING.md's three existing obligations — the licence manifest published as an artifact,
the public API surface reviewed as a whole, the call-graph walker precise or its precondition
confirmed
**When** the tag is cut
**Then** each is discharged and recorded, and the API surface census is re-measured against the
surface Epics 5–14 actually left rather than the 40 items measured at the Epic 3 boundary

**Given** RELEASING.md's own list of what it does not yet cover
**When** this story closes
**Then** version stamping, changelog policy, the tag command and how a matrix result is recorded
against a release are written into it, by whoever cuts the release, as that document already
instructs

**Given** the owner's recorded decision to cut after Epic 6, which Epics 7–14 postdate
**When** the tag is planned
**Then** the decision is re-affirmed or amended explicitly, naming which epics are inside v0.1.0 and
which are not — a decision that has been overtaken by events is re-made, not assumed

**Given** `version.go`, which reads `0.0.0-dev`
**When** the tag is cut
**Then** it carries the released version, and a test asserts the two agree, so a tag without a
stamp cannot ship

**Given** a new integrator with the README alone
**When** they follow it after the tag
**Then** they produce a PDF in minutes against the released version — C5 re-verified against the
thing that actually ships, not against the working tree

## Epic 16: A font arrives from the web, and stays on this machine

Epic 8 ended with a font list an author can finally use and a file that carries what it names. It also
ended with **21 families**, chosen by a build-time licence check, changing only when the designer is
released. The owner has looked at that list beside the 1,946 families Google Fonts publishes — 34 of
them Thai, against this catalogue's three — and ruled that the curated set is not the product
(**D-16.1**). The author reaches the library, not the shortlist.

This epic is therefore **the second reversal of the same Non-goal in one day**, and the record says so
plainly. D-8.4d.1 moved *when* a face is fetched, on a measurement. D-16.1 moves *what may be fetched,
and from whom*, on a product judgement with no measurement behind it. Under D-000.8 one reversal is
the system working; two in a day is worth naming rather than absorbing.

**What is actually being built is narrower than "live", and the epic is honest about it from its first
line.** CORS decides the shape, not preference (**D-16.3**, measured): `fonts.google.com/metadata/fonts`
returns all 1,946 families and **no `access-control-allow-origin`**, so a browser cannot read it and the
**index is a build-time snapshot** that ages between releases exactly as `font-catalogue.json` did.
What *is* live is the part that matters — **the bytes, the licence text and the per-family metadata**,
fetched from `raw.githubusercontent.com/google/fonts` on the author's pick, where the face is a **full
static TTF** and `OFL.txt` is the upstream licence text D-8.6.1 made mandatory. The obvious route,
`fonts.googleapis.com/css2`, is **unusable twice over** from a browser: it serves `woff2`, which
`decodeRecognisedFont` refuses by design, and it splits the family by `unicode-range` into subsets,
which would embed partial coverage into a document claiming the whole family.

Three things follow that are not UI work. **The licence check moves from build time to run time** —
the defect D-8.6.5 caught across 17 of 21 faces is now reachable in a place no build gate watches, and
16.1 owes that check its own home. **The forbidden-host scan is amended and grows a second half rather
than being deleted** (D-16.4): the two hosts it already forbids stay forbidden, because D-16.3 proved
them unusable anyway, and the newly-allowed hosts are policed by a rule that a source occurrence
outside the one module that declares them is still a failure. And **a face fetched once is kept**
(D-16.2) — in IndexedDB, keyed by the same SHA-256 content address the `.folio` uses, because the
owner's "local storage" is a ~5 MB string quota and one measured face is 90,220 bytes before base64's
+33%.

16.3 and 16.4 are the design. `Font Browser.dc.html` draws a browser this product has no equivalent of
— specimens at an author-set size in the author's own text, a Thai sample toggle, script and category
chips, staged multi-select, and a footer that counts what is about to be embedded — and a family
dropdown that names **three sources** where today's names one. The dropdown is where the epic is
either coherent or not: *in this template*, *added from web fonts*, and *available locally* are three
different relationships to a font, and an author who cannot tell them apart cannot reason about what
their file will contain.

**One question was open at planning and is now ruled** (**D-16.5**, owner, 2026-09-02): **558 of 1,946
families — 28.7% — ship variable-only**, with no static Regular to fetch, against a Non-goal that says
a weight is a face or it does not exist. They are **refused, and said to be refused**; the ones worth
having are **derived ahead of the build** by `tools/fontgen`, which is how two of the six affected Thai
families are already available. **Instancing in the browser was refused explicitly** — it would make
the embedded face a function of the author's runtime, and the 28.7% is the price of *"the same template
renders the same PDF everywhere"*.

Planning that question turned up a defect in shipped code (**D-16.6**, measured): **the embed command
accepts a variable face the renderer refuses**, so a pick can write a `.folio` that saves cleanly and
fails at render — reachable today, without any of this epic. It is Story 16.0's second goal, because it
is the same boundary being wrong in the same way.

**FRs covered:** none new — FR33's boundary is re-drawn, not crossed: nothing is fetched at RENDER time
**Also lands:** D-16.1, D-16.2, D-16.3, D-16.4 · UX-DR24, UX-DR25

### Story 16.0: The embed boundary stops throwing, and refuses what render will refuse

As a template author,
I want picking a typeface to work,
So that the rest of this epic has something to build on.

**Covers:** the epic's precondition · no FR
**Reported:** by the owner, 2026-09-02, against `main` at `a40c34d`, with a screenshot: the typography
panel shows `The engine returned an invalid response` under the family control after a pick

**Acceptance Criteria:**

**Given** the reported failure
**When** it is diagnosed
**Then** the cause is **named and evidenced**, not guessed at. What is already known bounds the search
and is recorded here so it is not re-derived: the string appears in exactly ONE place,
`folio-designer/src/engine.worker.ts:82`, reached only from `execute`'s bare `catch` — so it means
`host.handle(...)` threw or its output would not parse, i.e. a WASM-side fault, and **never** an engine
refusal, which travels the located-diagnostic path with a message of its own

**Given** the suspicion that some catalogue faces are simply bad input
**When** it is tested at the Go layer
**Then** it is **already refuted**, measured 2026-09-02 (wd `folio-go`, `go test ./wasm`, tree clean at
`a40c34d`): all 21 catalogue faces pass `embedFontFamily` natively through `wasm.Engine.Apply`. The
only failure in the whole shipped font tree was `notosanssc` at 10,595,932 bytes, refused **correctly**
and **located** — `folio: face exceeds the 6288384-byte supported size` — and that face is not in the
catalogue. **The fault is at the WASM/worker boundary, not in the command**

**Given** the boundary
**When** it is examined
**Then** the payload path is measured rather than reasoned about: a 598,060-byte face becomes ~800 KB
of base64 inside the command JSON, which `execute` base64-encodes **a second time** — byte by byte,
with string concatenation, at `engine.worker.ts:99` — into a ~1.06 MB string handed to Go. Whether the
throw is that path, wasm memory, or something else is what this story establishes

**Given** the diagnosis
**When** the fix lands
**Then** the bare `catch` at `engine.worker.ts:80-82` **stops erasing the cause**: whatever threw is
reported in terms someone can act on, because a boundary whose only vocabulary is *"invalid response"*
cannot be debugged from a user's screenshot — which is exactly how this defect arrived

**Given** a browser
**When** the fix is verified
**Then** it is verified **in one**, not only in Go. This repository's e2e specs are compile-checked and
never executed (D-000.4), and that is precisely why a browser-boundary defect reached the owner; this
story's own claim is unprovable without a real browser run, so it either runs one or states plainly
that it did not

**Given** the story's SECOND goal, found at plan time and measured (D-16.6)
**When** a variable face reaches `embedFontFamily`
**Then** it is **refused at the command**, because today it is **accepted** — measured against
`Anuphan[wght].ttf` (231,712 bytes): the command's only structural check is `checkSfnt`, which does not
look at `fvar`, while `fontset.New` refuses it at `internal/fontset/fontset.go:228`. **A pick can
currently write a `.folio` that saves cleanly and fails at render**, which is the one outcome D-8.4d.1
and D-16.1 both promise cannot happen, and it is reachable today without any of Epic 16

**Given** that refusal
**When** its message is written
**Then** it is the engine's existing one, not a second worse sentence: `fontset.go:228` already ends
with the `fonttools varLib.instancer` remedy and states why it names one — *"most Google Fonts
downloads are variable builds today, so a caller hitting this needs an action, not a refusal"*

**Given** the renderer's own `fvar` guard
**When** the command gains one
**Then** the renderer keeps its guard — a `.folio` can be hand-written and the loader is not the only
door. Both, or halt; this is `setComponentAsset`'s existing shape applied to the class it misses

### Story 16.1b: The binary is asked what it says about itself

*Sequenced **before** Story 16.1 despite its number (D-16.R.8). Numeric order is not build order here —
Story 16.0 is already the precedent.*

As the person who receives a template,
I want a typeface in it to be unable to lie about whose it is,
So that what the file says it may redistribute is what it actually may.

**Covers:** AD-26 · D-16.R.5 as replaced by D-16.R.7, D-16.R.8
**Design:** none — this is an engine guard

**Acceptance Criteria:**

**Given** Epic 16 moves the licence admission from a build gate to a runtime decision over unreviewed
bytes
**When** a face is embedded
**Then** the engine **re-reads the name table from the bytes in hand** and compares what the binary says
about its own licence with what is declared for it — the tie that gives the build gate its teeth
(`font-catalogue.test.ts:355-366`), which nothing else carries to runtime

**Given** that comparison
**When** it resolves
**Then** it has **three outcomes, not two**: a **contradiction** refuses, located, naming both what was
declared and what the bytes say; a **confirmation** admits; and **no evidence** — no signature matched,
or the name table is absent or unparseable — **admits**

**Given** a reader who will think admitting silence is a hole
**When** the code is written
**Then** the reasoning is stated in it: the threat is a face travelling under **another project's**
terms, which is a *false* statement and not a missing one. Measured, refusing silence rejected **50 of
100** sampled upstream faces — including **all three static UFL families, which carry no nameID 13 at
all** — while catching none of the threat

**Given** any face at all
**When** its own bytes name a **GPL-family or ShareAlike** licence
**Then** it is refused **whatever** licence is declared for it — AD-26 Binds *all*, and its Prevents is
copyleft arriving through a plausible-looking package. This half is new; the build gate never had it

**Given** a face with no nameID 13
**When** the guard runs
**Then** nameID 0 is consulted — and **only** on absence, never when nameID 13 is present and
disagreeing, or a face whose 13 says GPL could be laundered by a permissive-sounding 0

**Given** the guard
**When** it is proposed for merge
**Then** a **positive-control fixture** proves it fires — a guard never observed to refuse anything has
shipped vacuous — and the re-run sample shows **every refusal is a contradiction and none is a silence**

**Given** the Go table and the TS build-time table
**When** they diverge
**Then** a **test** fails, not a comment: the Go table is the authority for documents and strictly
subsumes the TS one

**Given** this narrows `embedFontFamily`, which is reachable through the exported API
**When** it is planned
**Then** it is recorded on D-000.15's running list for Story 15.3 — it is in the **before-the-tag set**
and lands in this epic or it does not land

### Story 16.1: A font arrives from the web with its terms attached

As a template author,
I want the fonts I can choose from to be the ones the web publishes, not a shortlist of twenty-one,
So that our brand's typeface is a search rather than a request to whoever builds the designer.

**Covers:** CAP-3 (re-drawn) · D-16.1, D-16.3, D-16.4, D-16.5
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Font Browser.dc.html`
  — the browser's header count, and the `+ Add` / `In template` button states that a fetch drives

**Acceptance Criteria:**

**Given** the family index
**When** the designer is built
**Then** a snapshot of `fonts.google.com/metadata/fonts` is taken at build time and trimmed to the
fields the browser renders — family, category, subsets, designers, style count, popularity, trending,
and whether the family declares axes — because **the browser cannot fetch it** (D-16.3: no
`access-control-allow-origin`), and the snapshot's staleness is stated in the UI rather than implied

**Given** an author picking a family
**When** its bytes are fetched
**Then** they come from `raw.githubusercontent.com/google/fonts` as a **full static TTF**, never from
`fonts.googleapis.com/css2` — which serves `woff2` the engine refuses by design and `unicode-range`
subsets that would embed partial coverage — and the file is chosen from that family's `METADATA.pb`
`fonts { filename }` entry for weight 400, normal, rather than by constructing a filename

**Given** every face that reaches a document this way
**When** it is embedded
**Then** its `licenceText` is the family's **upstream `OFL.txt` fetched with it**, its `copyright` is
read from the face's own `name` table nameID 0, and its `licence` identifier is `METADATA.pb`'s —
so D-8.6.1's mandatory record is satisfied from sources that cannot disagree with the bytes, and the
command still refuses to embed a face missing any of the three

**Given** the licence admission that used to be a build gate (D-8.5.2/D-8.5.3)
**When** it now happens at authoring time
**Then** it happens **somewhere named, in code, with a test** — the four-identifier allowlist is
applied to the fetched `METADATA.pb` licence before any byte is embedded, and an unclassifiable
licence is a refusal, never a warning, exactly as the build gate treated it

**Given** `scripts/forbidden-font-hosts.mjs`
**When** the newly-allowed hosts appear in source
**Then** the scan is **amended, not deleted** (D-16.4): `fonts.googleapis.com` and `fonts.gstatic.com`
stay forbidden, the allowed hosts are declared in one module, an occurrence anywhere else still fails
the build, and the population floor and positive control that keep the scan from passing vacuously are
extended to the new half

**Given** a family that ships variable-only, with no static Regular
**When** it is shown in the browser
**Then** it is **refused, and visibly so with its reason** — D-16.5 ruled (owner, 2026-09-02): refuse
variable-only, derive the ones worth having ahead of the build with `tools/fontgen`, and never instance
in the browser, which would make the embedded face a function of the author's runtime. 558 of 1,946
families (28.7%) are refused, and that is the price of *"the same template renders the same PDF
everywhere"*

**Given** a family that is variable upstream but already ships here as a derived static face —
`Noto Sans Thai`, `Noto Serif Thai`
**When** it is shown
**Then** it is **offered normally**. A row is refused because no static face is obtainable, never
because upstream happens to be variable, so the browser never refuses a family already in the author's
font menu

**Given** no network
**When** the author opens the browser
**Then** it says so in its own terms and offers what the machine already holds, degrading to *"you
cannot add a family right now"* — never to a document that will not render, because the shipped Noto
faces remain the coverage and an embedded face travels inside the `.folio`

### Story 16.1a: The local face tier covers the head of the library

As a template author,
I want the typefaces I am most likely to reach for to be there,
So that the browser's promise survives contact with the first family I type.

**Covers:** D-16.R.2 (owner), corrected by D-16.R.2a
**Design:** none — this story adds assets and changes no screen

**Acceptance Criteria:**

**Given** the measured refusal — **37 of the 50 most popular families are variable-only** on the
`google/fonts` mirror, Roboto, Open Sans, Inter and Montserrat among them
**When** the batch is assembled
**Then** each admitted family's static Regular comes from **that family's own upstream release**, not
from the mirror — because measured, the mirror is the only shelf that lacks them, and this repository
already ships `Roboto` and `Inter` exactly that way

**Given** D-16.5's claim that the remedy is `tools/fontgen` derivation
**When** this story is planned
**Then** that premise is treated as **corrected, not inherited** (D-16.R.2a): `instance_faces.py` drives
a hardcoded three-entry `UPSTREAM` list of ENGINE faces, **none of the 21 catalogue faces is derived**,
and derivation is the exception for a family that genuinely publishes no static

**Given** each added family
**When** it is committed
**Then** it carries the unmodified upstream `LICENSE*`, a `NOTICE.md` recording **both** the upstream
archive digest and the committed digest, a `font-catalogue.json` row, and a copyright read from its own
binary — the regime the existing 21 already satisfy, copied exactly rather than approximated

**Given** each added face
**When** the build runs
**Then** its declared `scripts` agree with its own `cmap` in both directions, and its declared SPDX id
agrees with its nameID 13 — the gates that exist, applied to the new population

**Given** the batch is a snapshot of a popularity ranking that moves
**When** it lands
**Then** its **membership rule, its size, its owner and its re-run trigger are written down** — D-16.5
left the batch unbounded and unowned, and that is the defect this story exists to close

**Given** an author after this story
**When** they type `Roboto`, `Open Sans` or `Inter`
**Then** the family is offered and embeds with **no download at all**

### Story 16.2: A fetched face stays on this machine

As a template author,
I want a typeface I have already fetched to be there next time,
So that adding our brand font to the next statement is not another download and another wait.

**Covers:** D-16.2
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Font Browser.dc.html`
  — the dropdown's `AVAILABLE LOCALLY` group

**Acceptance Criteria:**

**Given** a face fetched by a pick
**When** the fetch succeeds
**Then** its bytes, its licence text, its copyright and its family metadata are written to an
**IndexedDB** store keyed by the **SHA-256 of the bytes** — the same content address the `.folio`'s
`assets` map uses — so the store and the document agree on what a face is by construction

**Given** the owner's words were "local storage"
**When** the store is built
**Then** it is **not** `localStorage`, and the reason is recorded in code: a ~5 MB per-origin string
quota against a measured 90,220-byte face that base64 inflates by 33%, with a synchronous
`QuotaExceededError` and no partial-write path

**Given** a face already in the store
**When** the author picks that family again, in this or any later document on this machine
**Then** **nothing is fetched** — the store answers — and the existing embed path runs over the stored
bytes exactly as it runs over a fetched face today

**Given** the store
**When** it is read at startup
**Then** its contents populate the dropdown's `AVAILABLE LOCALLY` group, which is **faces this machine
has fetched** and never the operating system's font list (D-16.2) — no host font is enumerated or read

**Given** a browser that clears site data, a private window, or a storage failure
**When** the store cannot be read or written
**Then** the designer works without it — the group is empty, picks still fetch, and the failure is a
stated degradation rather than an error the author cannot act on

**Given** a face in the store that no document on this machine uses
**When** the author wants the space back
**Then** there is a way to remove it that says what it is removing, because a store that only ever
grows is a store nobody can reason about

### Story 16.3: The font browser is the dialog the design drew

As a template author,
I want to see a typeface set in my own words at the size I will print it,
So that choosing a font is looking at it rather than reading its name.

**Covers:** FR33's authoring side · UX-DR24, UX-DR25
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Font Browser.dc.html`
  — the whole modal: header, PREVIEW / WRITING SYSTEM / CATEGORY rail, Row and Grid results, footer

**Acceptance Criteria:**

**Given** the browser is opened from the dropdown's `Add fonts…` row
**When** it appears
**Then** it is the design's modal — header with the family count and a search field, a left rail
carrying PREVIEW, WRITING SYSTEM and CATEGORY, a results area, and a footer that counts what is staged
— and the count it prints is the **snapshot's** count, said in words that do not claim to be live

**Given** each result
**When** it is rendered
**Then** it carries a **specimen set in that family**, at the author's chosen size, in the author's
preview text or the design's default — which means the family's face must be registered for preview
independently of any document embedding it, since the canvas registers only faces the document already
carries

**Given** the Thai sample toggle
**When** it is on and the family covers Thai
**Then** the specimen is the design's Thai sentence rather than the Latin one, and the family carries
the design's `Thai + Latin` badge

**Given** the WRITING SYSTEM and CATEGORY chips and the sort control
**When** they are used
**Then** they filter and order the snapshot's own fields, `reset filters` appears exactly when a filter
is active, and an empty result says which query produced it

**Given** several families staged with `+ Add`
**When** the footer's confirm is pressed
**Then** each is fetched and embedded, the footer states progress rather than freezing, a family that
fails to fetch is reported by name without abandoning the others, and the successful ones are in the
document

**Given** the design draws `Row` and `Grid` views and a `⌘G` shortcut on the opening row
**When** the browser is built
**Then** both views ship and the shortcut opens the browser, or the omission is ruled and recorded
rather than left as a silent gap between the design and the product

**Given** the modal
**When** it is operated by keyboard or read by assistive technology
**Then** focus is trapped inside it, Escape closes it, every chip and toggle is a real control with a
name, and a staged family reports that it is staged (UX-DR25)

### Story 16.4: The family control names three sources

As a template author,
I want the font list to tell me where each font comes from,
So that I know what my file will contain before I save it.

**Covers:** UX-DR24
**Design:** `_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/mockups/Font Browser.dc.html`
  — the TYPOGRAPHY dropdown, its three groups and its `Add fonts…` footer

**Acceptance Criteria:**

**Given** the family control
**When** it is opened
**Then** it draws the design's three groups — `IN THIS TEMPLATE`, `ADDED FROM WEB FONTS` and
`AVAILABLE LOCALLY` — replacing today's two, with the filter field the design puts at the top, and the
`Add fonts…` row pinned at the bottom

**Given** the three groups
**When** an author reads them
**Then** they name three different relationships to a font and the difference is legible without
explanation: *in the file*, *fetched and now in the file*, *on this machine but not in this file* —
and a font's group changes because the author did something, never on its own

**Given** the disk-font decline Story 8.6 stated at this control
**When** the control is rebuilt
**Then** the decline is **re-examined against D-16.1, not carried forward unread** — it was derived
from the catalogue being the only source of licence terms, and that premise has changed; it is either
re-derived and restated, or lifted deliberately

**Given** the listbox's `role="presentation"` children, registered as deferred at Story 8.6
**When** the control is rebuilt
**Then** that defect is fixed here rather than multiplied — three groups is three more headings inside
a listbox that already breaks its required-owned-elements rule four times

**Given** a font in `AVAILABLE LOCALLY`
**When** it is picked
**Then** it is embedded into the document from the machine-local store with no fetch, and it moves to
`IN THIS TEMPLATE` — the same one-command, one-undo transaction Story 8.6 built

**Given** the status bar
**When** fonts have been added
**Then** it states how many and which is selected, as the design's own status line does

## Epic 17: The panel behaves like a panel

**This epic shipped without ever being written here.** All six stories are `done` in
`sprint-status.yaml` and each has a full spec in `_bmad-output/implementation-artifacts/`, but this
document — the one builders resolve epic context from — recorded nothing. It is written now as a pointer
rather than reconstructed, because the specs are the primary record and re-deriving acceptance criteria
from shipped code would manufacture a second, weaker source. **Read the spec files, not this section.**

| Story | Spec | Status |
|---|---|---|
| 17.1: The canvas follows the content field | `17-1-the-canvas-follows-the-content-field.md` | done |
| 17.2: Clicking beside the page keeps the selection | `17-2-clicking-beside-the-page-keeps-the-selection.md` | done |
| 17.3: Size and leading show the value they use | `17-3-size-and-leading-show-the-value-they-use.md` | done |
| 17.4: Arrow keys step a number field | `17-4-arrow-keys-step-a-number-field.md` | done |
| 17.5: The CONTENT box resizes from its bottom edge | `17-5-the-content-box-resizes-from-its-bottom-edge.md` | done |
| 17.6: The AD-17 scan can see a new violation | `17-6-the-ad-17-scan-can-see-a-new-violation.md` | done |

**Two of these are cited as governing precedent elsewhere and must not be treated as history.**

- **Story 17.3 decides how a panel field shows a value the document has not set.** It replaced grey
  placeholder defaults with the effective value, and it writes that value **only when the author commits
  the field — never on load, never on selection, never on render**, under the safety property *"opening a
  document may never mutate it."* Any later story adding a field with a documented default inherits this,
  **Story 12.3 included**, where AC1's *"clearable back to absent"* and AC3's cascade have to be
  reconciled against it.
- **Story 17.4 carries a human ruling on bounds** — that a floor and a ceiling are not the same kind of
  bound, because zero and negative are a property of the *field* while a band-edge ceiling is a property
  of the *layout*. That ruling forbids browser-side clamping even where the engine refuses, and it is
  binding on every later control.

**Record hygiene, flagged not fixed:** `17-1`, `17-2` and `17-3` carry `status: in-review` /
`in-progress` in their own frontmatter while the tracker says `done`. The tracker is the authority; the
frontmatter is stale. Reconcile downward when each is next touched, the way Story 12.1's closer did.

