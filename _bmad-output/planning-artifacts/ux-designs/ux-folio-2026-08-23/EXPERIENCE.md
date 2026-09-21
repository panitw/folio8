---
name: Folio
description: Experience contract for the Folio report designer — information architecture, behaviour, states, interactions, and flows.
status: final
created: 2026-08-23
updated: 2026-08-23
sources:
  - ../../prds/prd-folio-2026-08-22/prd.md
  - ../../prds/prd-folio-2026-08-22/addendum.md
design: ./DESIGN.md
---

# Folio — EXPERIENCE.md

Owns *how it works*. Visual identity lives in [DESIGN.md](./DESIGN.md); this document
references its tokens by name as `{path.to.token}`. Where a spine and any mock, wireframe,
or import disagree, **the spines win**.

Mockups: [First load](./mockups/Load.dc.html) · [Workspace](./mockups/Main.dc.html) ·
[Data binding](./mockups/Binding.dc.html) · [Table structure](./mockups/TableEditor.dc.html) ·
[Preview mode](./mockups/Preview.dc.html) — all five also live on one pan/zoom canvas at
[Folio Designer UI](https://claude.ai/code/artifact/9a0f4532-0c1a-4962-ae30-4b12e66739da).

Requirement IDs (FR·, NFR·) refer to the source PRD.

---

## Foundation

**Form factor:** desktop web browser. Single surface. No mobile responsiveness — a layout
tool for paged documents demands pointer precision and screen area.

**Delivery model:** the draw.io model. The user opens a URL, works on `.folio` files stored
on their own machine, and never creates an account (FR8). Nothing is stored server-side.
After first load the application is fully offline (FR36): ~~no network is required and none is
used.~~ no network is required, and none is used for the user's work. **AMENDED 2026-09-21 by OWNER DECISION (D-GA.1), recorded at `ARCHITECTURE-SPINE.md` AD-27.** **What still
holds:** the application is fully offline after first load — every byte it needs to open, edit,
render and export is precached, and the user's templates, data and PDFs are never sent anywhere.
**What changed:** a deployed build may load Google Tag Manager, which reports page loads and four
fixed action names. It is `async` and non-blocking, so with it unreachable — offline, or blocked —
nothing about the experience changes; and it is absent entirely from any build without
`VITE_GA_CONTAINER_ID` set. See `ARCHITECTURE-SPINE.md` AD-27.

**UI system:** none inherited. Folio defines its own tokens in DESIGN.md rather than
extending shadcn, MUI, or a platform system. The register is a precision instrument in the
tradition of Figma, Linear, and a digital audio workstation: dense, technical, and
unapologetically for people who will use it repeatedly.

**Theme:** single theme — dark chrome, light canvas. There is no light mode. The chrome
recedes so the A4 page is the only bright surface on screen, and the page reads as the
artefact rather than as a document window. Consequence: contrast obligations fall entirely
on the dark palette, with no light-mode fallback to lean on.

**Input model:** mouse-first. Direct manipulation on the canvas is the primary interaction;
keyboard shortcuts cover frequent operations. There is no command palette in MVP.

---

## Information Architecture

Five surfaces. Every stated need in the PRD lands on one of them, and every surface after
Load is reachable from the Workspace.

```
S1 Load  ──▶  S2 Workspace ──┬──▶  S3 Binding Panel   (docked, right)
                             ├──▶  S4 Table Editor    (focused, over canvas)
                             └──▶  S5 Preview         (full-surface mode switch)
```

### S1 · Load

→ [mockups/Load.dc.html](./mockups/Load.dc.html)

First run only — while the current release is becoming ready for this browser cache lifetime.
Covers the release-derived rendering engine and fonts; the Thai line-breaking dictionary is
embedded in the engine, not a separately requested payload (NFR7).

Stated, not hidden: the screen names what is loading and why it happens only once. A
progress indication is required — a spinner without progress is not acceptable at this
release-derived payload size.

### S2 · Workspace

→ [mockups/Main.dc.html](./mockups/Main.dc.html)

The primary surface and the application's home. Persistent regions:

| Region | Contains |
|---|---|
| **Canvas** | The page at its true proportions, with visible page boundaries (FR1), the three bands (FR6), and a grid with snapping (FR3) |
| **Palette** | Exactly five components — Text, Image, Table, Line, Rectangle (FR4). The set is closed; the UI must not imply extensibility |
| **Properties** | Contextual to selection. Position, size, typography, alignment, border, padding, background, visibility, binding (FR5) |
| **Document bar** | File open/save (FR8), page setup (FR2), mode switch to Preview |

The three bands — Page Header, Content, Page Footer — are **structural, not decorative**.
They must be legible as distinct regions at all times, because which band a component sits
in determines whether it repeats on every page. A user who cannot tell which band they are
dropping into will produce wrong output.

### S3 · Binding Panel

→ [mockups/Binding.dc.html](./mockups/Binding.dc.html)

Docked to the Workspace, not a separate destination. Loads a sample JSON document (FR9) and
exposes its paths as a navigable tree. Binding is performed by connecting a path to the
selected component (FR7).

This panel is where the two-user handoff becomes concrete: it is the only place a template
author sees the shape of the data the developer will supply.

### S4 · Table Editor

→ [mockups/TableEditor.dc.html](./mockups/TableEditor.dc.html)

Focused surface over the canvas, invoked from a selected Table component. Owns column
structure: add and remove columns, per-column width, alignment, header label, field binding
within row scope, and footer aggregate (FR10).

This is the densest UI in the product and the easiest to get wrong. Column configuration is
a matrix — columns against attributes — and must be presented as one, not as a repeated
single-column form.

### S5 · Preview

→ [mockups/Preview.dc.html](./mockups/Preview.dc.html)

A **mode switch, not a panel**. The canvas is replaced by the exact rendered PDF, produced by
`folio-go` compiled to WebAssembly running in the tab (FR35).

The distinction between S2 and S5 is a genuine product concept, not a visual nicety: the
canvas is an approximation, and the preview is the production artefact byte-for-byte (NFR1,
NFR5). See *Determinism in the Interface* below.

---

## Voice and Tone

**Terse and technical.** State the fact, name the location, offer no comfort. Trust the
reader.

- `No binding for transactions[].amount`
- `Row 1 exceeds content height. Clipped.`
- `Template version 2.0 — this build reads 1.0`

**Rules.**

- Name the element and the data path in every error (FR41). An error that cannot be located
  is a defect, not a message.
- Never apologize. No "Sorry", no "Oops", no "Something went wrong".
- Never use exclamation marks.
- Prefer the noun over the sentence for labels. `Page header`, not `Set the page header`.
- Distinguish **errors** (render cannot proceed) from **diagnostics** (render proceeded, with
  a caveat). FR25's over-tall row and FR44's clipped overflow are diagnostics. The two must
  not look alike.
- Technical vocabulary is correct vocabulary. Say *binding*, *band*, *aggregate*, *row scope*.
  Do not soften to *link*, *section*, *total*, *each row*. Both users benefit from one shared
  vocabulary; the `.folio` file uses these words too.

---

## Component Patterns

Behavioural contracts. Visual specification lives in DESIGN.md's *Components* section; each
pattern below names its token counterpart there.

### Canvas component — `{components.selection-handle}`, `{components.binding-chip}`

Selectable, movable, resizable by pointer, positioned absolutely within its band (PRD §6).
Components **do not reflow and do not push siblings** — a component that grows overlaps or
clips rather than displacing its neighbours (FR44).

The design must make absolute positioning feel intentional rather than broken. A user who
expects flow behaviour and gets overlap will read it as a bug.

Bound components display their binding, not a preview of the bound value — the value depends
on data that changes per render, and implying otherwise sets a false expectation.

### Band — `{components.band-tab}`, `{components.band-boundary}`

A container: always visible, always labelled. Cannot be deleted, reordered, or nested.
Resizable in height. Accepts components by drop.

Drop targeting must be unambiguous at the boundary between two bands. This is the most
consequential ambiguity in the whole canvas.

### Palette item — `{components.palette-item}`

Drag source. Five, fixed. A disabled state exists only during Load (S1).

### Property field — `{components.property-field}`

Contextual to selection; reflects the selected component's current value; commits on blur or
Enter. Multi-selection shows shared properties and marks divergent ones as mixed rather than
picking one arbitrarily.

### Binding tree node — `{components.tree-node}`

Represents a path in the loaded sample JSON. Leaf nodes are bindable; collection nodes
(`transactions[]`) are bindable only to a Table. Attempting an invalid binding must explain
the type mismatch rather than silently refusing.

### Table column row — `{components.matrix-row}`

One row per column in the Table Editor, carrying width, alignment, header label, bound field,
and footer aggregate. Reorderable. The row is the unit of manipulation — adding a column adds
a row here, not a step in a wizard.

---

## State Patterns

Every surface declares these. Absent states are the most common source of a broken-feeling
build.

| State | Behaviour |
|---|---|
| **Loading (first run)** | S1 only. Named, progressive, explained as one-time. |
| **Empty — no template** | Workspace (S2) with an unnamed blank template. Offer open-a-file and start-blank. Never a dead canvas with no path forward. |
| **Empty — no sample data** | Binding Panel (S3) before JSON is loaded. Binding is unavailable; the panel says so and offers the load action. Components can still be placed. |
| **Empty — table with no columns** | Table Editor (S4) at first open. Add-column is the only meaningful action and must be unmistakable. |
| **Populated** | The working state. |
| **Rendering** | Preview (S5) is computing. Blocking within the preview surface only; the canvas remains interactive. Must indicate progress for a 50-page document. |
| **Diagnostic** | Render succeeded with caveats — clipped content, over-tall row, missing glyph. Non-blocking, dismissible, locates the offending element. |
| **Error** | Render failed. Blocking within Preview (S5), names element and path, offers return to canvas. |
| **Unsaved changes** | Persistent, quiet, always visible. There is no autosave — the file lives on the user's machine and only they can write it. |
| **Stale preview** | The template changed since the last render. The preview must not silently present a stale document as the production artefact. Either invalidate visibly or re-render. |

**Stale preview is the one to get right.** Every other state failure is an inconvenience;
this one breaks the product's central promise.

---

## Interaction Primitives

- **Select** — click. Shift-click extends. Click on empty canvas clears.
- **Move** — drag. Snaps to grid when snapping is on (FR3).
- **Resize** — handles on selection. Constrained to the containing band.
- **Drop** — from palette to a band. The target band highlights before release.
- **Bind** — connect a binding-tree path to the selected component.
- **Switch mode** — Workspace ⇄ Preview. Preserves selection and scroll position where
  meaningful.
- **Open / Save** — the local filesystem. Save is explicit and manual, always.

**Shortcuts** cover the frequent operations: save, undo, redo, delete, duplicate, nudge,
toggle preview, toggle snapping. Hints appear in menus and tooltips where the action lives.
No command palette.

**Undo** covers every canvas and property mutation. Loading sample data is not undoable and
should not appear to be.

---

## Determinism in the Interface

*Invented section. Folio's signature technical property has a direct interface obligation,
and no standard UX vocabulary covers it.*

The PRD requires the previewed PDF to be byte-identical to the one a Go service renders in
production (NFR1, NFR5). The interface must earn that claim rather than assert it.

**Obligations:**

- The canvas is an **approximation** and the preview is **exact**. The interface must make
  this asymmetry legible without a tutorial. A user should never believe the canvas is
  authoritative.
- The preview must never present stale output as current (see State Patterns).
- Diagnostics that affect output — a clipped element, a row exceeding page height, a glyph
  with no coverage — surface in the preview where the consequence is visible, and locate back
  to the offending element on the canvas.
- Nothing in the interface may imply server rendering, a cloud round-trip, or an account.
  The engine runs in the tab, and the design must not borrow the visual language of remote
  processing.

---

## Accessibility Floor

Behavioural obligations. Contrast is DESIGN.md's responsibility.

The PRD places formal accessibility conformance out of MVP scope. That is a decision about
*conformance*, not a licence to build something unusable. The floor:

- Every interactive element is reachable and operable by keyboard, including palette,
  properties, binding tree, and table columns. Canvas manipulation may remain pointer-primary,
  but selection and property editing must not be pointer-only.
- Visible focus on every focusable element, using `{colors.select}`. The dark chrome makes
  this harder, not optional.
- Every icon-only control carries an accessible name.
- Errors and diagnostics are announced, and distinguished by shape before colour — the
  shapes are specified on `{components.diagnostic-card}` and `{components.error-card}`.
- Hit targets on canvas handles are larger than their visual footprint.
- The Table Editor is a data grid and must behave like one under keyboard navigation.

**Known gap, deliberate:** no screen-reader model exists for the canvas itself. Recorded as
UX1 rather than silently omitted.

---

## Key Flows

### KF-1 · Ploy builds the customer statement

Ploy handles customer communications at a mid-sized bank in Bangkok. She has laid out
statements in Word for years and has never used a report designer. Her data comes from a
developer as a JSON file.

1. She opens Folio. The load screen tells her the rendering engine and fonts are downloading,
   once. It takes a moment; she knows why.
2. She opens `customer-statement.folio` from her laptop.
3. She sets A4 portrait and drops the bank logo into the **Page Header** band — the band
   labels tell her this is the part that repeats.
4. She loads `sample-statement.json` in the binding panel and sees the data's shape for the
   first time.
5. She places text fields for customer name and account number, binding each by picking a
   path from the tree.
6. She drops a Table into the **Content** band and binds it to `transactions[]`.
7. In the Table Editor she configures five columns — Date, Description, Debit, Credit,
   Balance — setting widths, alignment, and a sum aggregate on Balance.
8. **She switches to Preview.** The real 34-page statement renders in her browser. Her Thai
   customer names wrap at correct word boundaries. The table header repeats on every page.
   The totals are right. *This is the moment Folio either earns her trust or loses it — and
   nothing she sees here will change when the developer renders it in production.*
9. A diagnostic tells her one description row was clipped. She clicks it, lands back on the
   offending element, widens the column, and previews again.
10. She saves the file to her laptop and commits it to the team repo.

### KF-2 · Anan consumes the template

Anan maintains the statements microservice. **He never opens the Folio designer.**

1. He pulls `customer-statement.folio` from the repo and reads it as text — it is legible
   JSON (FR12).
2. He adds `folio-go` to his module, loads the template, and renders with real customer data.
3. He records the output hash as a regression fixture. It matches what Ploy saw.

*Included because it constrains the designer's output, not because it needs UI.* Anan needs
the file readable, diffable, and hand-editable — which means the designer must never write
output that only the designer can read.

---

## Inspiration and Anti-patterns

**Draws from:** draw.io's ownership model — open a URL, work on your own files, no account,
no lock-in. Figma and DAW interfaces for chrome density and the discipline of letting the
artefact be the brightest thing on screen.

**Explicitly not:**

- **Not a document editor.** Folio designs a template, not a document. The canvas shows
  structure and bindings, never a finished-looking fake of the output. Google Docs is the
  wrong mental model.
- **Not a website builder.** No flow layout, no responsive breakpoints, no theme gallery.
  Content sits where it is placed.
- **Not a wizard.** No stepped setup, no guided template creation, no first-run tour beyond
  the load screen's explanation.
- **Not a cloud tool.** No sync indicator, no collaborator avatars, no share button, no
  autosave. Every one of these would be a lie.

---

## Open Questions

All five are **deferred non-blockers** — none prevents `bmad-architecture` or
`bmad-create-epics-and-stories` from proceeding. Each carries a revisit condition.

| # | Question | Revisit when |
|---|---|---|
| UX1 | What is the screen-reader model for the canvas? The rest of the accessibility floor does not depend on it. | Accessibility conformance moves into scope |
| UX2 | Does the sample JSON path persist in the `.folio` file, or must it be re-associated each session? PRD FR9 assumes separate, which has a real cost on reopen. | First real session with a template author |
| UX3 | How does the user discover which glyphs a font lacks before previewing? Relevant to Thai and CJK coverage (NFR3). | Font provisioning is settled — PRD Q2 |
| UX4 | Is there template-level undo across a save boundary, or does undo history die on reload? | Save/reload behaviour is implemented |
| UX5 | How is row scope (`{{transaction.amount}}`) expressed in the binding UI? | **Blocked upstream on PRD Q3** — the underlying rule is undefined, so the UI cannot be |
