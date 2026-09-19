---
name: Folio
description: Visual identity for Folio — a precision instrument for designing paged reports in the browser.
status: final
created: 2026-08-23
updated: 2026-08-23
experience: ./EXPERIENCE.md
sources:
  - ../../prds/prd-folio-2026-08-22/prd.md

colors:
  # chrome — the dark surround. Five steps, no sixth.
  ground: '#0D0F12'
  ground-preview: '#0A0C0E'
  base: '#15181C'
  panel: '#1A1E23'
  raised: '#22272D'
  active: '#2A3037'
  row-focus: '#1D2228'
  # rules and edges, dimmest first
  line-faint: '#191D21'
  line-subtle: '#22272D'
  line: '#272C33'
  line-strong: '#2F353D'
  dim: '#363D46'
  edge: '#3A4048'
  edge-strong: '#454D57'
  # text on chrome
  ink-high: '#E6E9EC'
  ink: '#AAB2BB'
  ink-low: '#737C86'
  ink-faint: '#5E666F'
  ink-ghost: '#4E565F'
  ink-disabled: '#454C55'
  # brand — the mark's own cyan, taken from resources/logo.png and used by
  # the brand mark alone. Deliberately NOT select: the logo is brighter than
  # the UI's structure colour, and routing the mark through select would
  # either dull the brand or repaint every selection in the app.
  brand: '#87F0FF'
  # select — structure, focus, authority
  select: '#58A6C4'
  select-bright: '#8FD0E4'
  select-on-fill: '#CFE8F2'
  select-fill: '#1E3B46'
  select-tint: '#16232A'
  select-tint-deep: '#12222A'
  select-edge: '#2F4F5C'
  select-hover: '#7CC0DA'
  # bind — data, and only data
  bind: '#C9A758'
  bind-text: '#E0C07A'
  bind-muted: '#8A7440'
  bind-tint: '#241F14'
  bind-tint-warm: '#1E1B13'
  bind-edge: '#4A3D1E'
  bind-edge-soft: '#3A3118'
  bind-edge-dash: '#6B5726'
  bind-on-page: '#A8801F'
  # status
  ok: '#7FBF8A'
  danger: '#D1655C'
  # the page — the only light surface
  page: '#FFFFFF'
  page-ink: '#1C2530'
  page-ink-body: '#2A3440'
  page-ink-muted: '#6B7480'
  page-thead: '#F1F4F7'
  page-thead-line: '#D5DBE2'
  page-rule: '#F0F2F5'
  page-guide: '#EEF1F4'
  page-dot: '#DFE3E8'
  page-outline-dot: '#CDD4DB'
  page-outline-dash: '#B6BEC7'
  page-placeholder: '#98A2AD'
  page-shell: '#E9ECEF'

# Translucent washes. Both accents run the same alpha ladder.
tints:
  band: 'rgba(88,166,196,0.035)'
  select-fill-soft: 'rgba(88,166,196,0.07)'
  select-focus-ring: 'rgba(88,166,196,0.22)'
  bind-row: 'rgba(201,167,88,0.14)'
  bind-marker: 'rgba(201,167,88,0.28)'
  scrim: 'rgba(6,8,10,0.72)'

typography:
  # chrome ramp — 10px is the default
  label:
    fontFamily: "'IBM Plex Sans', system-ui, sans-serif"
    fontSize: '9px'
    fontWeight: 600
    letterSpacing: '0.1em'
  label-tight:
    fontFamily: "'IBM Plex Sans', system-ui, sans-serif"
    fontSize: '9px'
    fontWeight: 600
    letterSpacing: '0.09em'
  body:
    fontFamily: "'IBM Plex Sans', system-ui, sans-serif"
    fontSize: '10px'
    fontWeight: 400
  body-em:
    fontFamily: "'IBM Plex Sans', system-ui, sans-serif"
    fontSize: '11px'
    fontWeight: 400
  title:
    fontFamily: "'IBM Plex Sans', system-ui, sans-serif"
    fontSize: '12px'
    fontWeight: 500
  display:
    fontFamily: "'IBM Plex Sans', system-ui, sans-serif"
    fontSize: '19px'
    fontWeight: 500
    letterSpacing: '-0.01em'
  mono:
    fontFamily: "'IBM Plex Mono', ui-monospace, monospace"
    fontSize: '10px'
  mono-em:
    fontFamily: "'IBM Plex Mono', ui-monospace, monospace"
    fontSize: '11px'
  numeric-lg:
    fontFamily: "'IBM Plex Mono', ui-monospace, monospace"
    fontSize: '22px'
    fontWeight: 500
  brand:
    fontFamily: "'IBM Plex Mono', ui-monospace, monospace"
    fontSize: '11px'
    fontWeight: 500
    letterSpacing: '0.04em'
  brand-load:
    fontFamily: "'IBM Plex Mono', ui-monospace, monospace"
    fontSize: '13px'
    fontWeight: 500
    letterSpacing: '0.06em'
  band-tab:
    fontFamily: "'IBM Plex Mono', ui-monospace, monospace"
    fontSize: '9px'
    letterSpacing: '0.08em'
  # page ramp — a document at 96px/inch, never mixed with the chrome ramp
  page-title:
    fontFamily: "'IBM Plex Sans Thai', 'IBM Plex Sans', sans-serif"
    fontSize: '13px'
    fontWeight: 600
  page-eyebrow:
    fontFamily: "'IBM Plex Sans', sans-serif"
    fontSize: '8px'
    letterSpacing: '0.14em'
  page-body:
    fontFamily: "'IBM Plex Sans Thai', 'IBM Plex Sans', sans-serif"
    fontSize: '8px'
  page-mono:
    fontFamily: "'IBM Plex Mono', ui-monospace, monospace"
    fontSize: '8px'
  page-fine:
    fontFamily: "'IBM Plex Sans Thai', 'IBM Plex Sans', sans-serif"
    fontSize: '7px'

rounded:
  DEFAULT: '0'
  sm: '0'
  md: '0'
  lg: '0'
  dot: '50%'

spacing:
  '1': '4px'
  '2': '6px'
  '3': '8px'
  '4': '10px'
  '5': '12px'
  '6': '14px'
  '7': '16px'
  '8': '22px'
  '9': '28px'
  '10': '34px'
  panel-x: '12px'
  sheet-x: '16px'
  row-y: '5px'

components:
  doc-bar:
    height: '40px'
    background: '{colors.panel}'
    borderBottom: '1px solid {colors.line}'
    paddingX: '{spacing.5}'
    gap: '{spacing.7}'
  status-bar:
    height: '24px'
    heightPreview: '32px'
    background: '{colors.panel}'
    borderTop: '1px solid {colors.line}'
    color: '{colors.ink-low}'
    font: '{typography.mono}'
  panel:
    width: '300px'
    widthRender: '320px'
    background: '{colors.panel}'
    border: '1px solid {colors.line}'
  palette-item:
    paddingY: '7px'
    paddingX: '{spacing.5}'
    activeBackground: '{colors.raised}'
    activeMarker: '2px solid {colors.select}'
    iconSize: '16px'
  property-field:
    background: '{colors.raised}'
    border: '1px solid {colors.line-strong}'
    paddingY: '{spacing.1}'
    paddingX: '{spacing.3}'
    valueFont: '{typography.mono-em}'
    valueColor: '{colors.ink-high}'
    affixColor: '{colors.ink-ghost}'
  segmented-control:
    border: '1px solid {colors.edge}'
    itemPaddingX: '{spacing.6}'
    itemPaddingY: '{spacing.1}'
    font: '{typography.body}'
    letterSpacing: '0.04em'
    activeBackground: '{colors.active}'
    activeColor: '{colors.ink-high}'
    activeWeight: 500
    activeBackgroundPreview: '{colors.select-fill}'
    activeColorPreview: '{colors.select-on-fill}'
    activeWeightPreview: 600
    inactiveColor: '{colors.ink-low}'
  band-tab:
    font: '{typography.band-tab}'
    color: '{colors.select}'
    background: '{colors.select-tint}'
    border: '1px solid {colors.select-edge}'
    paddingY: '3px'
    paddingX: '7px'
    offsetFromPage: '104px'
  band-boundary:
    borderTop: '1px dashed {colors.select}'
    opacity: '0.75'
    overhang: '14px'
    bandTint: '{tints.band}'
  selection-handle:
    size: '6px'
    background: '{colors.select}'
    border: '1px solid {colors.ground}'
    fill: '{tints.select-fill-soft}'
    outline: '1px solid {colors.select}'
  binding-chip:
    background: '{colors.bind-tint}'
    border: '1px solid {colors.bind-edge}'
    color: '{colors.bind-text}'
    dotSize: '5px'
    dotColor: '{colors.bind}'
    dotSizeDense: '4px'
    borderDense: '1px solid {colors.bind-edge-soft}'
  tree-node:
    paddingY: '{spacing.1}'
    indentLevel1: '18px'
    indentLevel2: '12px'
    activeBackground: '{colors.bind-tint}'
    activeMarker: '2px solid {colors.bind}'
    disabledOpacity: '0.42'
  matrix-row:
    paddingY: '{spacing.2}'
    focusMarker: 'inset 2px 0 0 {colors.select}'
    focusBackground: '{colors.row-focus}'
    focusRing: '2px solid {tints.select-focus-ring}'
    dragHandleColor: '{colors.ink-ghost}'
  diagnostic-card:
    border: '1px dashed {colors.bind-edge-dash}'
    borderLeft: '3px dashed {colors.bind}'
    background: '{colors.bind-tint-warm}'
    iconShape: 'triangle'
    rowTint: '{tints.bind-row}'
  error-card:
    border: '1px solid {colors.danger}'
    borderLeft: '3px solid {colors.danger}'
    iconShape: 'square'
    status: 'specified, not mocked'
  sheet:
    background: '{colors.panel}'
    border: '1px solid {colors.edge}'
    shadow: '0 18px 60px rgba(0,0,0,0.6)'
    scrim: '{tints.scrim}'
    headerHeight: '44px'
    footerHeight: '48px'
  progress-bar:
    height: '5px'
    heightInline: '3px'
    track: '{colors.panel}'
    trackBorder: '1px solid {colors.line-subtle}'
    fill: '{colors.select}'
  manifest-row:
    paddingY: '9px'
    borderBottom: '1px solid {colors.line-faint}'
    doneIcon: '{colors.ok}'
    activeIcon: '{colors.select}'
    queuedIcon: '{colors.ink-disabled}'
  page-surface:
    background: '{colors.page}'
    shadow: '0 2px 24px rgba(0,0,0,0.55)'
    shadowPreview: '0 4px 32px rgba(0,0,0,0.7)'
    gridDot: '{colors.page-dot}'
    gridPitch: '12px'
    size: '496 × 701 px at 68%'
---

# Folio — DESIGN.md

Owns *how it looks*. Behaviour, states, and flows live in
[EXPERIENCE.md](./EXPERIENCE.md), which references these tokens by name. Where this file
and any mockup disagree, this file wins.

Mockups: [First load](./mockups/Load.dc.html) · [Workspace](./mockups/Main.dc.html) ·
[Data binding](./mockups/Binding.dc.html) · [Table structure](./mockups/TableEditor.dc.html) ·
[Preview mode](./mockups/Preview.dc.html) — all five on one canvas at
[Folio Designer UI](https://claude.ai/code/artifact/9a0f4532-0c1a-4962-ae30-4b12e66739da).

---

## Brand & Style

Folio is a **precision instrument**. It belongs to the family of tools people open every
working day and never think about again — Figma, Linear, a digital audio workstation. It
does not court first impressions and it does not explain itself twice.

The governing idea is one sentence: **the page is the only bright thing on screen.**

Everything else — panels, rails, bars, fields — is a dark, low-contrast surround whose job
is to be legible without competing. When a user looks at Folio, their eye should land on the
A4 page they are laying out, because that is the artefact. The chrome is the workbench, not
the work.

That produces a specific discipline: small type, tight rhythm, no ornament, no radius, no
gradient, no shadow except where a surface genuinely floats above another. Density is a
feature — this is a tool for someone binding forty fields to a JSON document, and space
between controls is space they have to travel.

Monospace does real work here rather than decorating. Every value that a machine also reads
— a binding path, a coordinate, a hash, a file name, a byte count — is set in mono. Every
value a human wrote — a label, a heading, a description — is set in sans. The distinction is
load-bearing: if it is monospaced, it came from or goes to the system.

The register breaks exactly once, deliberately, on the first-load screen. See
*Layout & Spacing*.

---

## Colors

**Two accents, and each means one thing.** This is the palette's whole grammar. A user who
learns it once can read any screen.

**`{colors.select}` — cyan — means structure.** Selection, focus, band boundaries, the
active mode, progress, page navigation. Anything about *where you are* or *what is chosen*.
It is also the authority colour: preview mode's production marker and the output hash carry
it, because they assert that what you see is what ships.

**`{colors.bind}` — amber — means data, and only data.** A bound component's path, a
bindable node in the tree, the unsaved-changes dot, a diagnostic that a value did not fit.
It never marks selection and never marks structure.

> The rule is easy to break where the two overlap. A selected element that is *also* bound
> takes cyan handles and a cyan border — because it is selected — while its binding stays
> amber in the path text and the panel chip. Selection is selection regardless of what
> flows through the thing selected.

The two accents share lightness and chroma and differ only in hue, so neither dominates and
both read at the same weight against the chrome.

**The chrome is a five-step neutral ramp** — `{colors.ground}`, `{colors.base}`,
`{colors.panel}`, `{colors.raised}`, `{colors.active}` — cool but barely, well under the
saturation at which a grey starts to look tinted. Panels sit above the base; inputs sit
above panels; hover and active sit above inputs. **There is no sixth step.** An interface
that needs one is expressing hierarchy that a hairline should carry instead.
`{colors.row-focus}` is the single exception, and exists only for the focused matrix row.

`{colors.ground}` is the darkest surface and exists to make the page glow.
`{colors.ground-preview}` is one step darker still, used only in preview mode — a small,
deliberate signal that this surface is more serious than the canvas.

**Rules and edges are a seven-step ramp of their own**, from `{colors.line-faint}` to
`{colors.edge-strong}`; *Shapes* assigns them. `{colors.dim}` sits mid-ramp and does double
duty: the dimmest legible text (a version pin) and the faintest usable outline (a badge).

**Translucent washes run one alpha ladder for both accents** — see the `tints` block. The
band tint at `0.035` is the faintest thing in the product and must stay that way; anything
stronger makes the page look coloured rather than sectioned.

**The page palette is separate and must stay separate.** `{colors.page}` through
`{colors.page-shell}` describe a light surface and never appear in chrome; chrome colours
never appear on the page. The one exception is `{colors.bind-on-page}`, a darkened amber for
binding paths printed on white — the chrome amber fails contrast there and must never be
used on the page.

**`{colors.ok}` and `{colors.danger}`** are semantic and rare. Green appears once, on the
hash match. Red is reserved for a failed render. Neither carries meaning alone — see
*Do's and Don'ts*.

---

## Typography

**Three faces, one family.** IBM Plex Sans, IBM Plex Mono, and IBM Plex Sans Thai. Chosen
because it is engineered rather than neutral, because Mono and Sans share skeletons so
mixing them in one row does not jar, and because Plex Sans Thai gives real Thai coverage in
the same voice — which matters when Thai is a first-class rendering claim.

Explicitly not Inter, Roboto, or Arial.

### The chrome ramp

**Ten pixels is the default.** Not a floor to escape — the working size for labels, values,
and most UI text.

| Role | Token | Size | Use |
|---|---|---|---|
| Label | `{typography.label}` | 9px / 600 / `0.1em` | Section headings — `POSITION`, `BINDING`, `PAGES` |
| Label tight | `{typography.label-tight}` | 9px / 600 / `0.09em` | Matrix column headers |
| Body | `{typography.body}` | 10px | **The default.** Most UI text |
| Body emphasis | `{typography.body-em}` | 11px | Primary values, panel headings, selected labels |
| Title | `{typography.title}` | 12px | Sheet titles |
| Mono | `{typography.mono}` | 10px | Machine values in dense rows and status bars |
| Mono emphasis | `{typography.mono-em}` | 11px | Machine values in property fields |
| Display | `{typography.display}` | 19px | The load screen heading. Once, in the product. |
| Numeric large | `{typography.numeric-lg}` | 22px | The megabyte count on load. Once. |

**Emphasis is one step, not three.** Going from 10 to 11 px is the entire mechanism for
making something more important. Beyond that, use weight or colour. Reaching for 13 or 14 px
to signal significance is wrong; only the load screen carries display type, and it earns it
by being the one screen where the user is not working.

### The page ramp

Separate, sized for a document at 96 px per inch, and never mixed with the chrome ramp.
`{typography.page-fine}` at 7 px through `{typography.page-title}` at 13 px. A page value
set in a chrome token — or the reverse — is a defect.

### Rules

**Uppercase belongs only to section labels and mode switches**, always with tracking and 600
weight. Uppercasing a value, a name, or a sentence is wrong.

**The brand mark has two sizes:** `{typography.brand}` at 11 px in the document bar, and
`{typography.brand-load}` at 13 px with wider tracking on the load screen, where its
container also grows from 18 to 22 px. Nowhere else.

---

## Layout & Spacing

**Three-column workspace.** A 180 px palette rail, a fluid canvas, a 300 px properties
panel, with a 40 px document bar above and a 24 px status bar below. Preview mode swaps the
palette for a 132 px page-thumbnail rail, the properties panel for a 320 px render panel,
and the status bar for a 32 px page-navigation bar — the one place the frame changes height,
because page navigation needs a real hit target.

**The spacing scale is tight.** 4, 6, 8, 10, 12, 14, 16 px, with 22, 28, 34 reserved for the
load screen. Panel padding is 12 px horizontal; sheet padding is 16 px. Dense rows use 5 px
vertical padding, standard rows 7 to 9 px.

**Sibling groups are flex or grid with `gap`.** Never margins, never whitespace between
inline elements. Matrix rows, property pairs, and toolbars all declare their track sizes
explicitly, and the matrix header shares its template with its rows so columns align exactly.

**The canvas has an air budget the panels do not.** Panels are packed; the canvas around the
page is generous, because the page needs room to read as an object sitting on a surface.

**The load screen breaks the density rule on purpose.** A 560 px centred column, 19 px
heading, 22 px numerals, 28 to 34 px vertical gaps. It is the only moment in the product
where the user is waiting rather than working, and the only screen that has to explain
itself. Density there would read as indifference. The exception must not be generalized.

---

## Elevation & Depth

**Depth comes from tonal steps and hairlines, not shadow.** A panel is a step lighter than
the base; an input a step lighter than the panel. A 1 px `{colors.line}` rule separates
regions. This keeps the interface flat, quiet, and cheap to render.

Three shadows exist in the entire product, and no fourth may be added:

| Surface | Shadow |
|---|---|
| The page, on canvas | `0 2px 24px rgba(0,0,0,0.55)` |
| The page, in preview | `0 4px 32px rgba(0,0,0,0.7)` |
| The table-structure sheet | `0 18px 60px rgba(0,0,0,0.6)` over `{tints.scrim}` |

The page carries one so it reads as a physical sheet rather than a white rectangle; it
deepens in preview because the artefact is more present there. The sheet carries one because
it floats and the canvas must visibly recede. Nothing else casts a shadow.

---

## Shapes

**Every corner is square. `{rounded.DEFAULT}` is `0`.**

Not a stylistic tic — it is the most economical way to say *instrument, not app*. Rounded
corners read as friendly and consumer; Folio is neither. Squareness also makes the dense
grids legible, because adjacent cells share exact edges with no optical gap.

`{rounded.dot}` — `50%` — exists only for the 4–5 px status dots: the unsaved-changes marker
and the bindable-node indicator. Those are the only curves in the product.

**Borders are 1 px** and carry the whole burden of separation. Their tone encodes hierarchy:
`{colors.line-faint}` inside a list, `{colors.line}` between regions, `{colors.line-strong}`
around inputs, `{colors.edge}` around floating surfaces and mode switches.

**Dashed and dotted have fixed meanings** and are never decorative:

| Style | Meaning |
|---|---|
| Dashed cyan | A band boundary on the page |
| Solid cyan, with a mono **SECTION BREAK** tab | The content band's Section Break: the line below which content moves to the page where the content above it ends. An anchor icon before SECTION BREAK means Anchor is on; no icon means it is off. Canvas only — never printed |
| Dashed amber | A diagnostic — render completed with a caveat |
| Dotted grey on page | An unselected component's bounds |
| Dashed grey on page | A placeholder with no content yet |

---

## Components

**Document bar** — 40 px, `{colors.panel}`, bottom hairline. Left to right: mark, file name
with unsaved dot, file actions, then right-aligned page setup and the mode switch. The mode
switch is the only control in the bar with a border.

**Mode switch** — `{components.segmented-control}`. Frame is always `{colors.edge}`; only
the active chip changes between modes. In design mode it is `{colors.active}` on
`{colors.ink-high}` at weight 500. In preview mode it is `{colors.select-fill}` on
`{colors.select-on-fill}` at weight 600 — heavier and cyan, because preview asserts
authority and should not look like a peer of the canvas.

**Palette item** — 32 px row, 16 px stroke icon, label, right-aligned shortcut hint in
`{colors.ink-ghost}`. Active state takes `{colors.raised}` plus a 2 px left marker in
`{colors.select}`. Five items, no scroll, no search — the closed set is a design statement.

**Property field** — `{colors.raised}` on a `{colors.line-strong}` border. Prefix label in
mono at `{colors.ink-ghost}`, value in `{typography.mono-em}` at `{colors.ink-high}`, unit
suffix at `{colors.ink-ghost}`. Numeric values right-align; text values left-align.

**Section Break line** — a solid 1 px `{colors.select}` rule with the band boundary's 14 px
overhang on both sides, and a `{typography.band-tab}` **SECTION BREAK** tab beside it in
the band tab's colours. While the break is anchored, the tab shows a stroked anchor icon before
SECTION BREAK, in the tab's own colour; unanchored, it shows none. Drawn once, on the sheet that
holds it, at its declared position only — never where an unanchored section would be pushed; never
printed. Selected, it brightens to `{colors.select-bright}`. Elements below it carry no marking.

**Band tab** — `{typography.band-tab}` in `{colors.select}` on `{colors.select-tint}` with a
`{colors.select-edge}` border, positioned *outside* the page's left edge at 104 px. Paired
with a dashed boundary rule that overhangs the page by 14 px on both sides. The overhang is
the point: it makes the boundary read as a property of the page rather than a line drawn on
it.

**Selection handles** — 6 px squares in `{colors.select}`, ringed in `{colors.ground}` so
they stay visible over both white page and dark ground. Eight on a sized element, four on a
text run, plus a `{tints.select-fill-soft}` wash on the element itself. A dimension readout
in mono sits above the top-right handle.

**Binding chip** — amber dot, mono path, right-aligned type, on `{colors.bind-tint}` with a
`{colors.bind-edge}` border. The dense variant used in matrix cells drops the dot to 4 px
and softens the border to `{colors.bind-edge-soft}`.

**Tree node** — 18 px for the first indent, 12 px for the second. A 5 px `{colors.bind}` dot
marks a bindable leaf. Disabled nodes drop to `0.42` opacity and must carry a stated reason,
never a bare grey-out. Active node takes `{colors.bind-tint}` and a 2 px left marker.

**Matrix row** — the table editor's unit. Drag handle, index, then one cell per attribute on
an explicit grid. Focused row takes `{colors.row-focus}` plus `inset 2px 0 0 {colors.select}`,
and its focused cell a `{tints.select-focus-ring}` outline.

**Diagnostic card** — dashed border, 3 px dashed left edge, triangle icon,
`{colors.bind-tint-warm}` ground. Carries a severity label, the message, the location in
mono, and its actions. The affected page row takes `{tints.bind-row}`.

**Error card** — its counterpart: solid border, square icon, `{colors.danger}`. The shape
difference is the primary signal; colour is secondary. **Specified but not mocked** — no
artboard shows a failed render, only the legend describing one. Build it from this spec and
check it against `{components.diagnostic-card}` for symmetry.

**Progress bar** — 5 px overall, 3 px inline within a manifest row. Square,
`{colors.select}` fill on `{colors.panel}` track. Always paired with a numeric readout. A bar
without numbers is not acceptable.

**Manifest row** — the load screen's unit. Status glyph, name in mono, size in mono, state
word. Three glyphs: check (`{colors.ok}`), filled square in a ring (`{colors.select}`), dash
(`{colors.ink-disabled}`).

---

## Do's and Don'ts

**Do**

- Let the page be the brightest surface in every view.
- Use mono for anything a machine reads or writes; sans for anything a person wrote.
- Keep cyan for structure and amber for data. A selected bound element takes cyan.
- Encode state in shape and icon first, colour second.
- Pack panels tightly; give the canvas room.
- State the reason next to anything disabled.
- Pair every progress indication with a number.

**Don't**

- Don't round a corner. `{rounded.DEFAULT}` is `0`; `{rounded.dot}` is for status dots only.
- Don't add a shadow. Three exist and the list is closed.
- Don't add a sixth chrome surface step. Use a hairline instead.
- Don't use a colour-ramp gradient. Hard-stop patterns — the page's dot grid, the
  transparency checkerboard — are drawing techniques, not gradients, and are permitted.
- Don't introduce a third accent. If something needs emphasis, use weight, tone, or position.
- Don't use amber for selection or cyan for data — it breaks the one rule users learn.
- Don't distinguish a diagnostic from an error by colour alone.
- Don't jump more than one step up the type ramp for emphasis. Only the load screen has
  display type.
- Don't mix page tokens into chrome or chrome tokens onto the page. `{colors.bind-on-page}`
  exists precisely so this rule can hold.
- Don't add an emoji, anywhere. Icons are stroked SVG on a 16 px grid.
- Don't draw an affordance the product cannot honour — no share button, no avatar stack, no
  sync indicator, no autosave. There is no server and no account.
