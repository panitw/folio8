import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App from './App'
import type { EngineClient } from './engine-client'

// STORY 14.1 — THE CONTROL VOCABULARY, IN ITS CHECKABLE FORM.
//
// The rule this file enforces is written in the story spec
// (`_bmad-output/implementation-artifacts/14-1-one-button-vocabulary.md`,
// `## Design Notes` → THE RULE) in `DESIGN.md`'s own vocabulary:
//
//   V1  A control is a word.
//   V2  A control is a glyph only as one member of a segmented control.
//   V3  Uniformity: (a) every control sharing a control class is spelled the
//       same way; (b) every control inside one named control group is spelled
//       the same way.
//   V4  A glyph always carries an accessible name.
//
// V3(a), V3(b) and V4 are enforced here as clauses R1, R2 and R3. V2 is
// recorded rather than enforced, as clause R4 — a pin, by accessible name,
// over the glyph controls that are not members of a segmented control. Story
// 14.1 changes none of them; 14.2, 14.3, 14.4 and 14.7 each own one of those
// surfaces and will rule on it with the surface in front of them.
//
// ⚠ THE POPULATION IS DERIVED, NOT LISTED. Every clause below runs over the
// controls a RENDER produces — never over a hand-written list of control names.
// That is the failure shape DW-305 recorded: `design-contract.test.ts:165`
// asserts a 4.5:1 contrast floor over five hand-written pairs, and a 2.25:1
// pairing shipped in Story 13.5 with nothing red, because the pair was not on
// the list. A list cannot see what it does not name.
//
// ⚠ AND IT RENDERS RATHER THAN SCANS SOURCE, deliberately. `SegmentedProperty`
// (`App.tsx`) is a SINGLE JSX site whose children are `{segment.content}`.
// Align and Vertical align differ only in the DATA handed to it — `alignSegments`
// versus `valignSegments` — so the exact defect AC3 names (icons in one cell,
// the words TOP / MID / BOT in the cell beside it, same class, same row, same
// size) is invisible to a source-text or AST scan of the markup. Only rendered
// output can tell an `<svg>` child from the string `TOP`. The cost is that the
// guard sees only what it renders, which is why the coverage assertions below
// pin the VISITED GROUP NAMES and not merely a count: a count floor cannot see
// a render state quietly dropping out.
//
// ⚠ NO MEASUREMENT IDENTIFIERS. This file is inside the corpus
// `canvas-authority-contract.test.ts` scans, so it must not name
// `getComputedStyle`, `getBoundingClientRect`, `measureText`, `document.fonts`,
// `devicePixelRatio`, or any of the `offset*` / `client*` / `scroll*`
// identifiers. Nothing here needs them: the subject is what a control SAYS, not
// how large it is.

type Treatment = 'word' | 'glyph' | 'empty'

type Control = Readonly<{
  element: Element
  state: string
  treatment: Treatment
  name: string
  classes: ReadonlyArray<string>
  segmented: boolean
  where: string
}>

// A letter or a digit in any script. `×`, `∅`, `−`, `+`, `◀`, `▶` and `·` carry
// none, which is exactly the line V1 and V2 draw: a typographic character
// standing in for an icon is a GLYPH, not a word, however it is encoded.
const LETTER_OR_DIGIT = /[\p{L}\p{N}]/u

// THE VISUALLY-HIDDEN CLASSES, DERIVED FROM THE STYLESHEET rather than listed.
//
// ⚠ WITHOUT THIS, THE CLASSIFIER HAS A BLIND SPOT WIDE ENOUGH TO DRIVE A GLYPH
// THROUGH. `<button><svg aria-hidden="true"/><span class="sr-only">Zoom in</span></button>`
// is a control that LOOKS like an icon and READS like a word: strip only the
// `aria-hidden` subtree and the remaining text is `Zoom in`, so it classifies as
// a word and escapes R1, R2, R3 and R4's census together. That is DW-305's shape
// again — a guard that passes because it cannot see the thing. `.sr-only`
// (`App.css:7`) is real, Story 13.5 introduced it, `.diagnostic-announcement`
// (`App.css:594`) is a second copy of the same idiom, and neither sits inside a
// control TODAY. The point is that neither has to.
//
// The set is read out of `App.css` by the declarations that DEFINE the idiom —
// clipped to nothing while staying in the accessibility tree — so a third class
// written tomorrow is covered the day it is written, with no edit here.
// `getComputedStyle` is not available to this file (it is inside
// `canvas-authority-contract.test.ts`'s scanned corpus) and would not help in
// jsdom regardless; the stylesheet source is the honest reading.
const stylesheet = fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), 'App.css'), 'utf8')
const VISUALLY_HIDDEN_CLASSES: ReadonlySet<string> = new Set(
  Array.from(stylesheet.matchAll(/([^{}]+)\{([^{}]*)\}/g))
    .filter((rule) => /clip-path\s*:\s*inset\(\s*50%\s*\)/.test(rule[2] ?? '') && /position\s*:\s*absolute/.test(rule[2] ?? ''))
    .flatMap((rule) => Array.from((rule[1] ?? '').matchAll(/\.([A-Za-z0-9_-]+)/g)).map((match) => match[1] as string))
    .sort(),
)
// `[hidden]` joins them for the same reason: the platform removes it from the
// page and from the tree, so text inside one is not what the control says.
const HIDDEN_SUBTREES = ['[aria-hidden="true"]', '[hidden]', ...[...VISUALLY_HIDDEN_CLASSES].map((token) => `.${token}`)].join(', ')

// The visible text a reader is left with once the hidden subtrees are gone.
// `Undo <kbd aria-hidden="true">⌘Z</kbd>` reads as `Undo`, so a shortcut hint
// never turns a word into something else and never turns a glyph into a word.
function visibleText(control: Element): string {
  const clone = control.cloneNode(true) as Element
  clone.querySelectorAll(HIDDEN_SUBTREES).forEach((node) => node.remove())
  return (clone.textContent ?? '').replace(/\s+/g, ' ').trim()
}

// THE CLASSIFIER — one helper, applied to every control the sweep visits.
//
// ⚠ THE `aria-hidden` STRIP APPLIES TO TEXT, AND THE `<svg>` TEST DOES NOT, and
// that asymmetry is the whole point rather than an oversight. Every icon in this
// designer is written `<svg aria-hidden="true">` and every icon-bearing control
// carries its own `aria-label` — that IS the correct spelling (UX-DR25,
// `EXPERIENCE.md:282`), and it is what makes the glyph decorative to a screen
// reader while remaining the only thing a sighted reader sees. Stripping the
// icon before asking "is this a glyph?" would classify every correctly-written
// icon control as empty and leave R4's census permanently blank — a guard that
// passes because it looks in the wrong place. So: strip `aria-hidden` to read
// the TEXT, and look for the `<svg>` in the control as it actually renders.
export function treatmentOf(control: Element): Treatment {
  const text = visibleText(control)
  if (LETTER_OR_DIGIT.test(text)) return 'word'
  if (control.querySelector('svg') !== null) return 'glyph'
  if (text.length > 0) return 'glyph'
  return 'empty'
}

// The accessible name, computed over the forms this designer actually uses:
// `aria-label`, then `aria-labelledby`, then name-from-content with the
// `aria-hidden` subtrees excluded, then `title`.
//
// ⚠ IT IS NOT TRUSTED ON ITS OWN. The row below titled "agrees with the
// accessibility tree about every swept control's name" asserts this function's
// answer against jest-dom's `toHaveAccessibleName` for EVERY control the sweep
// visits, in every state. A resolver that quietly disagreed with the
// accessibility tree would make R3 and R4 pin the wrong strings, and nothing
// else in this file would notice.
export function accessibleName(element: Element): string {
  const label = element.getAttribute('aria-label')
  if (label !== null && label.trim() !== '') return label.replace(/\s+/g, ' ').trim()
  const ids = (element.getAttribute('aria-labelledby') ?? '').split(/\s+/).filter((id) => id !== '')
  if (ids.length > 0) {
    const referenced = ids.map((id) => element.ownerDocument.getElementById(id)?.textContent ?? '').join(' ').replace(/\s+/g, ' ').trim()
    if (referenced !== '') return referenced
  }
  // ⚠ NAME-FROM-CONTENT READS THE `.sr-only` CAPTION, and `visibleText` does not.
  // The two are deliberately different readings of the same control: what a
  // SIGHTED reader sees (which is the subject of V1/V2/V3) versus what a SCREEN
  // READER announces (which is the subject of V4). A visually-hidden caption is
  // invisible to the first and load-bearing for the second. Only `aria-hidden`
  // and `[hidden]` are dropped here, because those leave the tree too.
  const named = element.cloneNode(true) as Element
  named.querySelectorAll('[aria-hidden="true"], [hidden]').forEach((node) => node.remove())
  const content = (named.textContent ?? '').replace(/\s+/g, ' ').trim()
  if (content !== '') return content
  const title = element.getAttribute('title')
  return title === null ? '' : title.replace(/\s+/g, ' ').trim()
}

// A container is a NAMED CONTROL GROUP if the accessibility tree gives it both a
// grouping role and a name. `aria-label` on a roleless `<div>` is neither — the
// tree drops it — which is the defect Story 14.1 fixed on `.document-actions`
// and, by orchestrator ruling, on `.mode-switch`.
function groupsIn(root: ParentNode): ReadonlyArray<Element> {
  return Array.from(root.querySelectorAll('[role="group"], [role="tablist"]'))
}

function groupName(group: Element): string {
  const label = group.getAttribute('aria-label')
  if (label !== null && label.trim() !== '') return label.replace(/\s+/g, ' ').trim()
  const ids = (group.getAttribute('aria-labelledby') ?? '').split(/\s+/).filter((id) => id !== '')
  const referenced = ids.map((id) => group.ownerDocument.getElementById(id)?.textContent ?? '').join(' ').replace(/\s+/g, ' ').trim()
  return referenced
}

function controlsIn(root: ParentNode): ReadonlyArray<Element> {
  return Array.from(root.querySelectorAll('button, [role="button"]'))
}

// SEGMENTED-CONTROL MEMBERSHIP, DERIVED — never a class name, because the class
// is exactly the thing a future control could quietly adopt or shed. A group is
// a `{components.segmented-control}` when it holds two or more controls and
// EVERY one of them carries `aria-pressed`: that is what "the mutually-exclusive
// values of a single closed-set property" looks like in the tree. Align,
// Vertical align and the DESIGN/PREVIEW mode switch qualify; `PDF navigation`,
// `Local file actions`, `Border edges` and the inspector tablist do not.
function isSegmentedControl(group: Element): boolean {
  const members = controlsIn(group)
  return members.length >= 2 && members.every((member) => member.hasAttribute('aria-pressed'))
}

function sweepControls(root: ParentNode, state: string): ReadonlyArray<Control> {
  const segmented = groupsIn(root).filter(isSegmentedControl)
  const elements = controlsIn(root)
  return elements.map((element, index) => ({
    element,
    state,
    treatment: treatmentOf(element),
    name: accessibleName(element),
    classes: (element.getAttribute('class') ?? '').split(/\s+/).filter((token) => token !== ''),
    segmented: segmented.some((group) => group.contains(element)),
    where: `${state} · control ${index + 1} of ${elements.length}`,
  }))
}

const describeControl = (control: Control) => `${control.name === '' ? '(no accessible name)' : `"${control.name}"`} [${control.treatment}]`

// R1 — V3(a). Every control that shares a control class is spelled the same way.
// `empty` controls (the drag handles) are excluded: they have no spelling to
// disagree about, and R3 is what holds them to account.
function r1Violations(controls: ReadonlyArray<Control>): ReadonlyArray<string> {
  const byClass = new Map<string, Control[]>()
  for (const control of controls) {
    if (control.treatment === 'empty') continue
    for (const token of control.classes) byClass.set(token, [...(byClass.get(token) ?? []), control])
  }
  return [...byClass.entries()].filter(([, members]) => members.length >= 2 && new Set(members.map((member) => member.treatment)).size > 1)
    .map(([token, members]) => `R1 class "${token}" mixes treatments: ${members.map(describeControl).join(', ')}`)
    .sort()
}

// R2 — V3(b). Every control inside one named control group is spelled the same
// way. A row that puts icons beside words at the same size is this clause.
function r2Violations(root: ParentNode, state: string): ReadonlyArray<string> {
  return groupsIn(root).map((group) => {
    const members = controlsIn(group).map((element) => ({ element, state, treatment: treatmentOf(element), name: accessibleName(element), classes: [], segmented: false, where: state } as Control)).filter((member) => member.treatment !== 'empty')
    if (members.length < 2 || new Set(members.map((member) => member.treatment)).size === 1) return undefined
    return `R2 group "${groupName(group) === '' ? '(no accessible name)' : groupName(group)}" mixes treatments: ${members.map(describeControl).join(', ')}`
  }).filter((message): message is string => message !== undefined).sort()
}

// R3 — V4 / UX-DR25 / `EXPERIENCE.md:282`. A control a reader cannot read has to
// be a control a screen reader can announce.
function r3Violations(controls: ReadonlyArray<Control>): ReadonlyArray<string> {
  return controls.filter((control) => control.treatment !== 'word' && control.name === '')
    .map((control) => `R3 ${control.treatment} control with an empty accessible name at ${control.where}`)
    .sort()
}

// R4 — the V2 census. Pinned BY ACCESSIBLE NAME, never by count: a count floor
// passes when one control is renamed and another appears in its place. This
// story records the set and changes none of it; one more reds and forces a
// human ruling.
//
// GLYPH, not glyph-or-empty, and R4's own wording is why: "the set of GLYPH
// controls that are not members of a segmented control". An `empty` control has
// no spelling to be wrong about — it is a drag handle or a canvas element, not a
// picture standing in for a word — and R3 is what holds it to an accessible
// name. Folding empties in here would also pull the canvas's own elements into a
// census of chrome controls, which is a different subject.
function censusMembers(controls: ReadonlyArray<Control>): ReadonlyArray<Control> {
  return controls.filter((control) => control.treatment === 'glyph' && !control.segmented)
}
// ⚠ A MULTISET KEYED BY STATE AND NAME, NOT A SET OF NAMES, and that is R-Q2's
// ruling carried all the way through rather than half way. A `Set` of names
// collapses a repeat: a SECOND glyph control appearing under a name the census
// already holds produces neither an arrival nor a departure, so the clause whose
// whole purpose is "never by count" would have been silently insensitive to
// exactly one more control. Keying by state as well as name also means a glyph
// that stops rendering in ONE state while surviving in another still reds.
const censusKey = (control: Control) => `${control.state} · ${control.name === '' ? '(no accessible name)' : control.name}`
const tally = (keys: ReadonlyArray<string>): ReadonlyMap<string, number> => {
  const counts = new Map<string, number>()
  for (const key of keys) counts.set(key, (counts.get(key) ?? 0) + 1)
  return counts
}
function r4Violations(controls: ReadonlyArray<Control>, census: ReadonlyArray<string>): ReadonlyArray<string> {
  const found = tally(censusMembers(controls).map(censusKey))
  const recorded = tally(census)
  return [...new Set([...found.keys(), ...recorded.keys()])].sort().flatMap((key) => {
    const now = found.get(key) ?? 0
    const before = recorded.get(key) ?? 0
    if (now === before) return []
    return [now > before
      ? `R4 the closed set moved: ${now} glyph control(s) outside every segmented control now render as ${key}, where the census records ${before}`
      : `R4 the closed set shrank: ${now} glyph control(s) outside every segmented control now render as ${key}, where the census records ${before}`]
  })
}

// ---------------------------------------------------------------------------
// THE DECLARED RENDER STATES.
// ---------------------------------------------------------------------------

const canvas = { width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+07:00', marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [{ face: 'Noto Sans', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }
const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
const tableComponent = { id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 12_000, resizable: false }
// STORY 14.2 / D-14.2.Q4 — THE LINE AND THE RECTANGLE JOIN THE SWEEP.
//
// 14.1 deferred V2 (R-Q2) on the explicit promise that 14.2, 14.3, 14.4 and
// 14.7 would each own one of the remaining surfaces and rule on it with the
// surface in front of them. This is the first of the four, and it ships new
// controls: an orientation segmented control, and a `background` row spelled
// `Colour` on a Line and `Fill` on a Rectangle, which renames up to four
// accessible names apiece. Shipping those under a guard that cannot see them
// would have made the deferral retroactively hollow.
//
// The Line CARRIES A BORDER on purpose: the panel withholds the border
// controls for a Line and discloses the stored one in prose, so this state
// sweeps the withholding rather than a Line that had nothing to withhold.
const lineComponent = { id: 'e2', type: 'line' as const, band: 'content' as const, x: 0, y: 40_000, width: 72_000, height: 1_000, resizable: true, background: '#000000', borderWidth: 2_000, borderColor: '#c81e1e', borderEdges: ['bottom' as const] }
const rectComponent = { id: 'e3', type: 'rect' as const, band: 'content' as const, x: 0, y: 60_000, width: 72_000, height: 24_000, resizable: true, background: '#1b2a4a', borderWidth: 1_000, borderColor: '#000000', borderEdges: ['bottom' as const] }
// STORY 14.7 — THE MOCK LEARNED TO ANSWER `table-columns`, BECAUSE THE TABLE
// EDITOR IS A DECLARED RENDER STATE NOW AND A MOCK THAT CANNOT ANSWER IT MOUNTS
// NOTHING. `App.tsx:openTableEditor` admits a projection only when the
// snapshot's revision, the reply's snapshot revision AND the reply's
// `tableColumns.revision` all agree with the revision that was current when the
// request went out — so the reply below carries revision 1, matching the
// mounted snapshot. A mismatched revision is silently refused: the state would
// have swept the design surface behind the dialog and reported a healthy count
// while proving nothing about the dialog at all.
//
// The projection is the FULL twenty-six-member header block plus a `columns`
// array, because `TableColumns` is exact on the wire and a partial one is not a
// projection this panel can render. Twenty-six is `headerHeight` and
// `altRowBackground` plus twelve committed/resolved pairs — Story 14.8 took it
// from twenty by adding the border trio's three pairs.
const tableHeaderProjection = { headerHeight: 12_000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12_000, headerLineSpacing: 0, headerLineSpacingResolved: 1_000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false }
const tableColumnsReply = {
  snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3 },
  tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72_000, align: 'right' as const, headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: 'sum' as const, footerOf: 'items.amount', footerFormat: '#,##0.00' }] } },
}
const engine = () => ({ request: vi.fn(async (operation: string) => operation === 'table-columns' ? tableColumnsReply : { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } }) }) as unknown as EngineClient

const mount = (components: ReadonlyArray<typeof textComponent | typeof tableComponent | typeof lineComponent | typeof rectComponent>) =>
  render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [...components] } }} />)

// Each state is a NAMED render, so a state that stops producing controls is
// named in the failure rather than absorbed into a smaller total.
// ⚠ `open` IS ASYNC AS OF STORY 14.7, AND EVERY CALLER AWAITS IT. It was
// synchronous, which was enough while every state was one click away. Opening
// the Table Editor is not: it dispatches a `table-columns` request and mounts
// on the reply, so a synchronous `open` would have returned a container with no
// dialog in it and swept the design surface behind it — a state that reports a
// healthy control count and proves nothing about the surface it names.
type RenderState = Readonly<{ name: string; open: () => Promise<Element> }>
const states: ReadonlyArray<RenderState> = [
  {
    name: 'design · nothing selected',
    open: async () => mount([textComponent, tableComponent]).container,
  },
  {
    name: 'design · a text element selected',
    open: async () => {
      const view = mount([textComponent, tableComponent])
      fireEvent.click(within(view.container).getByLabelText('text component e1'))
      return view.container
    },
  },
  {
    name: 'design · a table element selected',
    open: async () => {
      const view = mount([textComponent, tableComponent])
      fireEvent.click(within(view.container).getByLabelText('table component e7'))
      return view.container
    },
  },
  {
    name: 'design · a line element selected',
    open: async () => {
      const view = mount([textComponent, tableComponent, lineComponent, rectComponent])
      fireEvent.click(within(view.container).getByLabelText('line component e2'))
      return view.container
    },
  },
  {
    name: 'design · a rect element selected',
    open: async () => {
      const view = mount([textComponent, tableComponent, lineComponent, rectComponent])
      fireEvent.click(within(view.container).getByLabelText('rect component e3'))
      return view.container
    },
  },
  {
    // STORY 14.7 — THE DENSEST SURFACE IN THE PRODUCT (UX-DR8) JOINS THE SWEEP,
    // which is what 14.1's R-Q2 deferral promised for this story. It ships the
    // shared alignment control into a second surface and three new row
    // affordances, so shipping them under a guard that could not see them would
    // have made the deferral retroactively hollow.
    //
    // ⚠ INSERTED BEFORE `preview`, NOT APPENDED. The R0 shrink proof below is
    // `states.slice(0, -1)` — it drops the LAST state on purpose, which must
    // stay `preview`, because that is the state whose `PDF navigation` and
    // `Render actions` groups the proof names. Appending here would have made
    // that red drop THIS state instead and prove something other than what it
    // claims. The comment on that proof has been bitten once already, at 14.2.
    name: 'design · the table editor open',
    open: async () => {
      const view = mount([textComponent, tableComponent])
      fireEvent.click(within(view.container).getByLabelText('table component e7'))
      fireEvent.click(within(view.container).getByRole('button', { name: 'Configure columns' }))
      await within(view.container).findByRole('dialog', { name: 'Table Editor' })
      return view.container
    },
  },
  {
    name: 'preview',
    open: async () => {
      const view = mount([textComponent, tableComponent])
      fireEvent.click(within(view.container).getByRole('button', { name: 'PREVIEW' }))
      return view.container
    },
  },
]

type Swept = Readonly<{ state: string; root: Element; controls: ReadonlyArray<Control> }>
const sweepStates = async (declared: ReadonlyArray<RenderState>): Promise<ReadonlyArray<Swept>> => {
  const swept: Swept[] = []
  for (const state of declared) {
    const root = await state.open()
    swept.push({ state: state.name, root, controls: sweepControls(root, state.name) })
  }
  return swept
}
const sweepEveryState = (): Promise<ReadonlyArray<Swept>> => sweepStates(states)

// THE V2 CENSUS AS IT STANDS at Story 14.1 — eleven distinct names over twenty
// renderings, DERIVED from the sweep above and then written down here so that
// one more reds. Story 14.1
// changes none of them; 14.2, 14.3, 14.4 and 14.7 each own one of these surfaces
// and will rule on it with the surface in front of them (R-Q2).
//
// Two items the story's prose enumerates are deliberately ABSENT from this set,
// and the classifier is why rather than an oversight:
//   • the `B` / `I` weight and slope toggles classify as WORDS. `B` and `I` are
//     letters, and V1's own test is "contains a letter or digit" — they are the
//     initial of the property they set, not a picture of it. Whether an initial
//     is a word is a judgement, and this story does not make it silently.
//   • the drag handles (`Resize e1`, `Resize the page header`, `Resize the page
//     footer`) and the canvas's own elements classify as EMPTY: they render no
//     text and no `<svg>`, so they have no spelling to be wrong. R3 still holds
//     each of them to an accessible name, and every one passes.
// Both are reported to the orchestrator with the rest of the audit rather than
// quietly folded in here.
const V2_CENSUS: ReadonlyArray<string> = [
  // `.canvas-tools` — a roleless `<div>` whose `aria-label` the tree drops, so
  // R2 cannot see that it also puts these two beside Grid / Snap / Duplicate /
  // Delete. Reported, not fixed: no AC names it. (Design's three states only.)
  'design · nothing selected · Zoom out',
  'design · nothing selected · Zoom in',
  'design · a text element selected · Zoom out',
  'design · a text element selected · Zoom in',
  'design · a table element selected · Zoom out',
  'design · a table element selected · Zoom in',
  'design · a line element selected · Zoom out',
  'design · a line element selected · Zoom in',
  'design · a rect element selected · Zoom out',
  'design · a rect element selected · Zoom in',
  // SPEC-multi-pages story 2 / D-2.3 — THE OWNER OVERRIDES V2 FOR THE TWO PAGE
  // BUTTONS. Add page and Delete page are icon-only glyph buttons in
  // `.canvas-tools`, matching the toolbar they sit in, so the closed set grows
  // by exactly these two controls, deliberately, in every design state that
  // renders the toolbar.
  'design · nothing selected · Add page',
  'design · nothing selected · Delete page',
  'design · a text element selected · Add page',
  'design · a text element selected · Delete page',
  'design · a table element selected · Add page',
  'design · a table element selected · Delete page',
  'design · a line element selected · Add page',
  'design · a line element selected · Delete page',
  'design · a rect element selected · Add page',
  'design · a rect element selected · Delete page',
  'design · the table editor open · Add page',
  'design · the table editor open · Delete page',
  // `.property-inline-action` — the inspector's `×` clear, `∅` null and the
  // font-family disclosure chevron. Uniform within their class, so R1 is green.
  // They render only while something is selected.
  'design · a text element selected · Clear Font size (pt)',
  'design · a text element selected · Clear Line spacing',
  'design · a text element selected · Set Background null',
  'design · a text element selected · Show fonts',
  'design · a table element selected · Clear Font size (pt)',
  'design · a table element selected · Clear Line spacing',
  'design · a table element selected · Set Background null',
  'design · a table element selected · Show fonts',
  // STORY 14.2 — THE LINE AND THE RECTANGLE, DERIVED BY RUNNING THE SWEEP AND
  // READING WHAT IT REPORTED, never hand-written from the spec.
  //
  // Read the two blocks against each other and the story is legible in them.
  // The LINE carries no `Clear Border …` row at all: this panel withholds the
  // border stack for a Line, and this census is where that withholding is
  // visible to a guard rather than only to a reader. It also renders no
  // TYPOGRAPHY section, so `Clear Font size (pt)`, `Clear Line spacing` and
  // `Show fonts` are absent for both kinds — neither is `typographic`.
  //
  // And the renamed row appears under its NEW name in both: `Set Colour null`
  // for the Line, `Set Fill null` for the Rectangle. `Set Background null`
  // survives untouched in the text and table states above, which is exactly
  // what gating the spelling on a SINGLE selection buys.
  //
  // The line's `Clear Colour` and the rect's `Clear Fill` render because each
  // fixture carries a committed colour; `canClear` is field-keyed and offers
  // the reset beside the value it resets.
  'design · a line element selected · Clear Colour',
  'design · a line element selected · Set Colour null',
  'design · a rect element selected · Clear Border width (pt)',
  'design · a rect element selected · Clear Border colour',
  'design · a rect element selected · Clear Border edges',
  'design · a rect element selected · Clear Fill',
  'design · a rect element selected · Set Fill null',
  // STORY 14.7 — THE TABLE EDITOR STATE, DERIVED BY RUNNING THE SWEEP AND
  // READING WHAT IT REPORTED, never hand-written from the story.
  //
  // The dialog is an OVERLAY, not a replacement, so this state re-reports the
  // design surface behind it (`Zoom out` / `Zoom in`, and the table's own
  // inspector clears) and then adds what the dialog itself draws. Two families
  // are new:
  //   • six `Clear …` glyphs — Epic 12's HEADER AND ROWS section, whose `×`
  //     clears no declared state could see before this one existed; and
  //   • `Move column 1 earlier` / `Move column 1 later` / `Remove column 1` —
  //     the three ROW AFFORDANCES this story put in place of four columns that
  //     wore column headers. They are glyphs OUTSIDE a segmented control, which
  //     is exactly the set V2 says has to be recorded rather than assumed.
  //
  // ⚠ THE ALIGNMENT SEGMENTS ARE DELIBERATELY ABSENT FROM THIS LIST. All three
  // carry `aria-pressed` inside one named group, so `isSegmentedControl`
  // derives them as a segmented control and V2 permits a glyph there — the same
  // reason `Align`, `Vertical align` and `Orientation` have never been census
  // members. If they ever appeared here, the control would have stopped being a
  // segmented control, which is the fact worth reddening on.
  'design · the table editor open · Zoom out',
  'design · the table editor open · Zoom in',
  'design · the table editor open · Clear Font size (pt)',
  'design · the table editor open · Clear Line spacing',
  'design · the table editor open · Set Background null',
  'design · the table editor open · Show fonts',
  'design · the table editor open · Move column 1 earlier',
  'design · the table editor open · Move column 1 later',
  'design · the table editor open · Remove column 1',
  'design · the table editor open · Clear Alternating row background',
  'design · the table editor open · Clear Header font family',
  'design · the table editor open · Clear Header font size (pt)',
  'design · the table editor open · Clear Header line spacing',
  'design · the table editor open · Clear Header background',
  'design · the table editor open · Clear Header text colour',
  // STORY 14.8's BORDERS section, and only TWO members join the census from it.
  // The width and the colour are the shipped `styleNumber`/`styleColour` rows,
  // each with the same `×` clear its six siblings above carry. The four EDGE
  // controls do not appear, and that is by construction rather than by luck:
  // they are `<input type="checkbox">` and the sweep's population is
  // `button, [role="button"]`, so a checkbox is never swept — which is exactly
  // why the edge control was built from bare checkboxes instead of copying
  // `BorderEdgesProperty`, whose `role="group"` and `×` clear would have moved
  // `GROUP_INSTANCE_FLOOR` and this census at once.
  //
  // MEASURED by executing the sweep and reading the two names it reported, per
  // the rule recorded at the bottom of this file — never derived on paper.
  'design · the table editor open · Clear Header border width (pt)',
  'design · the table editor open · Clear Header border colour',
  // SPEC-table-rules' RULED AREA section, and its three × clears join the
  // census for exactly the reason the two above it did: each is the shipped
  // clear affordance on a field that CAN be cleared, and the section's own
  // checkbox pair (the ruled boundaries) deliberately carries none — unchecking
  // both IS the clear there, so no fourth glyph control arrives with it.
  //
  // MEASURED by executing the sweep and reading the three names it reported,
  // per the rule recorded at the bottom of this file — never derived on paper.
  'design · the table editor open · Clear Minimum height',
  'design · the table editor open · Clear Rule width (pt)',
  'design · the table editor open · Clear Rule colour',
  // spec-table-cell-padding-header-align-info. The two (i) explanation buttons
  // are glyphs outside any segmented control. The HEADER ALIGN segments are
  // too, and deliberately: the spec forbids a new `role="group"` in the Table
  // Editor (it would clear GROUP_INSTANCE_FLOOR), so that control is a named
  // `toolbar`, which this sweep does not count as a group — and so its three
  // segments are recorded here rather than derived as a segmented control.
  // MEASURED by executing the sweep and reading the five names it reported.
  'design · the table editor open · About bindings',
  'design · the table editor open · About column widths',
  'design · the table editor open · Header align left for column 1',
  'design · the table editor open · Header align center for column 1',
  'design · the table editor open · Header align right for column 1',
  // The PDF navigation group — uniform within its group, so R2 is green.
  'preview · Previous PDF page',
  'preview · Next PDF page',
  'preview · Zoom out PDF',
  'preview · Zoom in PDF',
  // OWNER RULING AFTER 14.1 — THE DOCUMENT BAR AND THE CANVAS TOOLBAR ARE GLYPH
  // CONTROLS WITH A HOVER GUIDE. Both families are uniform (`.tool-button`, and
  // `Local file actions` is all glyphs), so R1 and R2 stay green; they are
  // outside any segmented control, so V2 records them here. MEASURED by running
  // the sweep: the six file actions render in every state, and the four canvas
  // toggles/actions beside the zoom steppers in every design state.
  ...states.flatMap((state) => [
    ...['Open local template', 'Save local template', 'Save As', 'New…', 'Undo', 'Redo'],
    ...(state.name === 'preview' ? [] : ['Grid on', 'Snap on', 'Duplicate', 'Delete']),
  ].map((name) => `${state.name} · ${name}`)),
]

// R0 — COVERAGE HONESTY, and it is a CLAUSE like the other four rather than a
// bare assertion, so that it too can be run against a planted shrink and watched
// to fail. A guard whose own coverage check has never been seen to red is the
// same list-shaped trap the rest of this file is written against.
//
// ⚠ THE GROUPS ARE SPLIT INTO CHECKED AND PRESENT-BUT-UNDER-ARITY, because
// R2 returns early on a group holding fewer than two non-empty swept controls
// and would otherwise report such a group as covered when nothing checked it.
// `Border edges` is exactly that: its four edge controls are
// `<input type="checkbox">` and its only button is a `×` clear that renders
// solely once an edge is set, so in every declared state R2 sees zero members
// there. Recording it in the second bucket makes the hole VISIBLE — and a group
// silently sliding from the first bucket into the second now reds.
//
// (`PDF navigation` is in the FIRST bucket, measured: its `<select>` and two
// text inputs are outside this story's swept population, but its four `◀ ▶ − +`
// buttons are not, so R2 does check it over four members.)
const GROUP_ARITY_FLOOR = 2
const CHECKED_GROUPS: ReadonlySet<string> = new Set([
  'Local file actions',
  // Story 14.2's orientation control. Two glyph segments, both carrying
  // `aria-pressed`, so it is a segmented control by derivation — its members
  // are outside R4's census by construction and inside R2's arity.
  'Orientation',
  'Designer mode',
  'Inspector tabs',
  'Align',
  'Vertical align',
  'PDF navigation',
  'Render actions',
  // STORY 14.7's two Table Editor groups that R2 actually gets to work with.
  // `Cell alignment for column 1` is the shared segmented control in its second
  // surface — three glyph segments, every one carrying `aria-pressed`, so it is
  // a segmented control by derivation exactly as `Align` and `Orientation` are.
  // `Table header and rows` is Epic 12's section, whose six `×` clears R2 has
  // never been able to see until this state existed.
  'Cell alignment for column N',
  'Table header and rows',
])
// `Table row scope` joins `Border edges` in the second bucket, and for the same
// honest reason: it is a labelled group of two TEXT INPUTS and holds no button
// at all, so R2 returns early on it in every state. Recording it here makes the
// hole VISIBLE rather than letting the group read as covered.
// `Documentation` is the third, and it is under arity by design rather than by
// accident: the group holds ONE LINK and no button at all, so R2 has nothing to
// compare. It is its own group precisely so that `Local file actions` stays six
// buttons; the link's own spelling is pinned by name in the row further down.
const UNDER_ARITY_GROUPS: ReadonlySet<string> = new Set(['Border edges', 'Table row scope', 'Documentation'])
// RE-BASELINED AT STORY 14.2, and the re-baselining is the point rather than
// bookkeeping. Floors, not equalities, so ordinary growth never churns the
// guard while any shrink reddens — but a floor left at an old measurement is a
// SLACK ratchet, and a comment naming the wrong baseline is worse than no
// ratchet at all, because it reads as maintained. 14.2 added two render states
// and the totals rose; leaving 14.1's numbers would have let the guard lose
// most of a state without a word.
//
// RE-MEASURED AT STORY 14.7 by EXECUTING the sweep and reading what it
// reported — not by adding this story's controls to 14.2's numbers on paper.
// The Table Editor state is an overlay over the design surface, so it sweeps
// far more than the dialog's own controls, and a hand-written total would have
// been wrong in a direction nobody could check.
//
// MEASURED: 248 controls, 20 class tokens, 37 group instances, per state
// [27, 41, 40, 34, 35, 54, 17] in declared order.
// PER_STATE_CONTROL_FLOOR does NOT move: the smallest state is still `preview`
// at 17, and the new state is the LARGEST at 54.
const PER_STATE_CONTROL_FLOOR = 15
const CONTROL_FLOOR = 220
const CLASS_FAMILY_FLOOR = 18
// RE-BASELINED BY THE DOCUMENTATION LINK: its group renders in every declared
// state, adding one instance per state. The floor moves from 33 to 39 so the
// shrunk sweep below (38) stays UNDER it; left at 33 the dropped-state clause
// could no longer fail.
const GROUP_INSTANCE_FLOOR = 39

// ⚠ THE COVERAGE RECORD IS KEYED BY A FIXTURE-INDEPENDENT NAME. A group whose
// label carries a row number — `Cell alignment for column 1` — is a name bound
// to the fixture that happened to render it: the moment the Table Editor state
// declares two columns, `Cell alignment for column 2` is a group R2 sweeps and
// the record cannot name, and R0 reds on a coverage record that grew a row
// rather than on anything being uncovered. Per-row groups are one group in the
// coverage sense, so the ordinal is normalised away HERE and nowhere else — R1,
// R2, R3 and R4 keep reporting the real names, because a violation has to name
// the control a reader can find.
const coverageName = (name: string) => name.replace(/ for column \d+$/, ' for column N')

// The most non-empty swept controls any single state puts inside a group of this
// name — the arity R2 actually got to work with at its best.
function groupArity(swept: ReadonlyArray<Swept>): ReadonlyMap<string, number> {
  const best = new Map<string, number>()
  for (const entry of swept) {
    for (const group of groupsIn(entry.root)) {
      const key = coverageName(groupName(group))
      const members = controlsIn(group).filter((element) => treatmentOf(element) !== 'empty').length
      best.set(key, Math.max(best.get(key) ?? 0, members))
    }
  }
  return best
}

function r0Violations(swept: ReadonlyArray<Swept>): ReadonlyArray<string> {
  const messages: string[] = []
  const controls = swept.flatMap((entry) => entry.controls)
  const families = new Set(controls.flatMap((control) => control.classes))
  const groupInstances = swept.flatMap((entry) => groupsIn(entry.root))
  for (const entry of swept) {
    if (entry.controls.length < PER_STATE_CONTROL_FLOOR) messages.push(`R0 state "${entry.state}" swept ${entry.controls.length} controls, under its floor of ${PER_STATE_CONTROL_FLOOR}`)
  }
  if (controls.length < CONTROL_FLOOR) messages.push(`R0 the sweep visited ${controls.length} controls, under the floor of ${CONTROL_FLOOR}`)
  if (families.size < CLASS_FAMILY_FLOOR) messages.push(`R0 the sweep visited ${families.size} class families, under the floor of ${CLASS_FAMILY_FLOOR}`)
  if (groupInstances.length < GROUP_INSTANCE_FLOOR) messages.push(`R0 the sweep visited ${groupInstances.length} group instances, under the floor of ${GROUP_INSTANCE_FLOOR}`)
  // ⚠ AND THE FLOORS ABOVE CANNOT SEE A SHRINK BY NAME, which is what this
  // clause exists for: drop the preview state and every total above still clears
  // its floor while `PDF navigation` and `Render actions` quietly stop being
  // checked. These say WHICH group went, and whether it went missing or merely
  // fell below the arity R2 needs.
  const arity = groupArity(swept)
  const checked = new Set([...arity].filter(([, members]) => members >= GROUP_ARITY_FLOOR).map(([name]) => name))
  const underArity = new Set([...arity].filter(([, members]) => members < GROUP_ARITY_FLOOR).map(([name]) => name))
  for (const name of [...CHECKED_GROUPS].sort()) {
    if (checked.has(name)) continue
    messages.push(arity.has(name)
      ? `R0 the group "${name}" is no longer checked by R2: it renders ${arity.get(name)} non-empty control(s), under the arity of ${GROUP_ARITY_FLOOR}`
      : `R0 the group "${name}" renders in no declared state, so nothing checked it`)
  }
  for (const name of [...checked].sort()) {
    if (!CHECKED_GROUPS.has(name)) messages.push(`R0 a group R2 now checks is not recorded as checked: "${name}"`)
  }
  for (const name of [...UNDER_ARITY_GROUPS].sort()) {
    if (!underArity.has(name)) messages.push(`R0 the group "${name}" is recorded as present-but-unchecked and no longer renders that way`)
  }
  for (const name of [...underArity].sort()) {
    if (!UNDER_ARITY_GROUPS.has(name)) messages.push(`R0 a group is present but under R2's arity and is not recorded as such: "${name}" (${arity.get(name)} non-empty control(s))`)
  }
  // Every treatment the classifier can return is actually exercised, so a
  // classifier that had collapsed to one answer would red here rather than pass
  // every clause vacuously.
  const treatments = new Set(controls.map((control) => control.treatment))
  for (const treatment of ['word', 'glyph', 'empty'] as const) {
    if (!treatments.has(treatment)) messages.push(`R0 no swept control classified as ${treatment}, so the clauses over that treatment ran vacuously`)
  }
  return messages
}

describe('control vocabulary contract', () => {
  it('R0 — the sweep is non-vacuous, and every recorded group is still covered the way it was', async () => {
    const swept = await sweepEveryState()
    expect(swept.map((entry) => entry.state)).toEqual(states.map((state) => state.name))
    expect(r0Violations(swept)).toEqual([])
    // The clause's own inputs are non-vacuous: a `VISUALLY_HIDDEN_CLASSES` that
    // parsed to nothing would silently restore the P3 blind spot, and no other
    // assertion in this file would notice.
    expect([...VISUALLY_HIDDEN_CLASSES]).toEqual(['diagnostic-announcement', 'sr-only'])
  })

  it('agrees with the accessibility tree about every swept control\'s name', async () => {
    for (const entry of await sweepEveryState()) {
      for (const control of entry.controls) {
        if (control.name === '') expect(control.element, control.where).not.toHaveAccessibleName()
        else expect(control.element, control.where).toHaveAccessibleName(control.name)
      }
    }
  })

  it('R1 — every control sharing a control class is spelled the same way', async () => {
    for (const entry of await sweepEveryState()) expect(r1Violations(entry.controls), entry.state).toEqual([])
  })

  it('R2 — every control inside one named control group is spelled the same way', async () => {
    for (const entry of await sweepEveryState()) expect(r2Violations(entry.root, entry.state), entry.state).toEqual([])
  })

  it('R3 — every glyph and every empty control carries an accessible name', async () => {
    for (const entry of await sweepEveryState()) expect(r3Violations(entry.controls), entry.state).toEqual([])
  })

  it('R4 — the set of glyph controls outside every segmented control has not moved', async () => {
    expect(r4Violations((await sweepEveryState()).flatMap((entry) => entry.controls), V2_CENSUS)).toEqual([])
  })

  // -------------------------------------------------------------------------
  // THE CLAUSES, RUN AGAINST A PLANTED VIOLATION AND AGAINST THE NEAREST
  // LEGITIMATE SPELLING OF THE SAME THING (D-11.3.7). Reading a rule is not
  // testing a rule; each red below is EXECUTED and each is asserted to name the
  // clause it must wake and NO OTHER, so "something failed" cannot pass for a
  // proof.
  // -------------------------------------------------------------------------

  // The harness every planted fragment runs through, and the reason each proof
  // can claim ONE clause: `census` defaults to the fragment's own glyph set, so
  // R4 is silent by construction unless a proof perturbs it deliberately.
  const rulesOver = (root: Element, census?: ReadonlyArray<string>): ReadonlyArray<string> => {
    const controls = sweepControls(root, 'planted')
    const settled = census ?? censusMembers(controls).map(censusKey)
    return [...r1Violations(controls), ...r2Violations(root, 'planted'), ...r3Violations(controls), ...r4Violations(controls, settled)]
  }
  const clausesWoken = (root: Element, census?: ReadonlyArray<string>): ReadonlyArray<string> => [...new Set(rulesOver(root, census).map((message) => message.slice(0, 2)))].sort()
  const glyph = <svg aria-hidden="true" viewBox="0 0 16 16"><path d="M2 8h12" /></svg>

  it('R1 reds on the exact defect AC3 names — one class, an icon in one cell and a word in the next', () => {
    // The shipped defect, transcribed: ONE `.property-segment` class, one row,
    // an `<svg>` on one side and the string TOP on the other.
    const { container } = render(<div>
      <button type="button" className="property-segment" aria-label="Align left">{glyph}</button>
      <button type="button" className="property-segment" aria-label="Vertical align top">TOP</button>
    </div>)
    expect(rulesOver(container)).toEqual(['R1 class "property-segment" mixes treatments: "Align left" [glyph], "Vertical align top" [word]'])
    expect(clausesWoken(container)).toEqual(['R1'])
  })

  it('R1 passes over the nearest legitimate spelling — the same class, both cells iconic', () => {
    const { container } = render(<div>
      <button type="button" className="property-segment" aria-label="Align left">{glyph}</button>
      <button type="button" className="property-segment" aria-label="Vertical align top">{glyph}</button>
    </div>)
    expect(rulesOver(container)).toEqual([])
  })

  it('R2 reds on the exact defect AC2 names — one named group holding glyphs beside words', () => {
    // The document bar as it shipped: Open and Save as icon-only buttons in the
    // same named family as Save As and Start blank.
    const { container } = render(<div role="group" aria-label="Local file actions">
      <button type="button" aria-label="Open local template">{glyph}</button>
      <button type="button" aria-label="Save local template">{glyph}</button>
      <button type="button">Save As</button>
      <button type="button">Start blank</button>
    </div>)
    expect(rulesOver(container)).toEqual([
      'R2 group "Local file actions" mixes treatments: "Open local template" [glyph], "Save local template" [glyph], "Save As" [word], "Start blank" [word]',
    ])
    expect(clausesWoken(container)).toEqual(['R2'])
  })

  it('R2 passes over the nearest legitimate spelling — the same group, every member a word', () => {
    const { container } = render(<div role="group" aria-label="Local file actions">
      <button type="button" aria-label="Open local template">Open</button>
      <button type="button" aria-label="Save local template">Save</button>
      <button type="button">Save As</button>
      <button type="button">Start blank</button>
    </div>)
    expect(rulesOver(container)).toEqual([])
  })

  it('R2 does not mistake a shortcut hint for a treatment — an `aria-hidden` <kbd> beside a word is still a word', () => {
    // The nearest legitimate spelling of the R2 red above, and the one the
    // document bar actually ships: Undo and Redo end in an `aria-hidden` <kbd>
    // while Save As does not. A classifier that read raw `textContent` would
    // still call all three words; one that stripped the whole control would call
    // two of them glyphs and red a shipped, correct row.
    const { container } = render(<div role="group" aria-label="Local file actions">
      <button type="button">Save As</button>
      <button type="button">Undo <kbd aria-hidden="true">⌘Z</kbd></button>
      <button type="button">Redo <kbd aria-hidden="true">⇧⌘Z</kbd></button>
    </div>)
    expect(rulesOver(container)).toEqual([])
    expect(treatmentOf(screen.getByRole('button', { name: 'Undo' }))).toEqual('word')
  })

  it('R3 reds on a glyph with no accessible name, and on an empty control with none', () => {
    const { container } = render(<div>
      <button type="button">{glyph}</button>
      <button type="button" className="resize-handle" />
    </div>)
    expect(rulesOver(container)).toEqual([
      'R3 empty control with an empty accessible name at planted · control 2 of 2',
      'R3 glyph control with an empty accessible name at planted · control 1 of 2',
    ])
    expect(clausesWoken(container)).toEqual(['R3'])
  })

  it('R3 passes over the nearest legitimate spelling — the same two controls, each named', () => {
    const { container } = render(<div>
      <button type="button" aria-label="Align left">{glyph}</button>
      <button type="button" className="resize-handle" aria-label="Resize e1" />
    </div>)
    expect(rulesOver(container)).toEqual([])
  })

  it('sees a glyph wearing a visually-hidden caption, and does not mistake it for a word', () => {
    // THE P3 BLIND SPOT, PLANTED. `<svg aria-hidden/>` plus a `.sr-only` caption
    // is a control that looks like an icon and reads like a word: strip only the
    // `aria-hidden` subtree and the leftover text is `Zoom in`, so it would
    // classify as a WORD and escape R1, R2, R3 and the R4 census together.
    const { container } = render(<div>
      <button type="button" className="canvas-stepper">{glyph}<span className="sr-only">Zoom in</span></button>
      <button type="button" className="canvas-stepper">Grid on</button>
    </div>)
    const stepper = screen.getAllByRole('button')[0] as Element
    expect(treatmentOf(stepper)).toEqual('glyph')
    // Its NAME still comes from that caption — the two readings are different on
    // purpose, and V4 must keep working through a `.sr-only` label.
    expect(accessibleName(stepper)).toEqual('Zoom in')
    expect(stepper).toHaveAccessibleName('Zoom in')
    // And now the class family it shares with a word is a live R1 violation
    // instead of two words agreeing about nothing.
    expect(rulesOver(container)).toEqual(['R1 class "canvas-stepper" mixes treatments: "Zoom in" [glyph], "Grid on" [word]'])
    expect(clausesWoken(container)).toEqual(['R1'])
  })

  it('R4 reds when a glyph control appears outside every segmented control, and names it', () => {
    const { container } = render(<div>
      <button type="button" aria-label="Collapse the inspector">×</button>
    </div>)
    expect(rulesOver(container, [])).toEqual([
      'R4 the closed set moved: 1 glyph control(s) outside every segmented control now render as planted · Collapse the inspector, where the census records 0',
    ])
    expect(clausesWoken(container, [])).toEqual(['R4'])
  })

  it('R4 reds when a recorded member stops rendering, rather than passing over a smaller set', () => {
    const { container } = render(<div />)
    expect(rulesOver(container, ['planted · Zoom in'])).toEqual([
      'R4 the closed set shrank: 0 glyph control(s) outside every segmented control now render as planted · Zoom in, where the census records 1',
    ])
  })

  // R-Q2 IN FULL: "pin by accessible name, NEVER BY COUNT". A `Set` of names
  // would pass this — a second control under a name the census already holds is
  // neither an arrival nor a departure — which is the same shape as the count
  // floor the ruling rejected. The multiset sees it.
  it('R4 reds when a SECOND control appears under a name the census already holds', () => {
    const { container } = render(<div>
      <button type="button" aria-label="Zoom in">+</button>
      <button type="button" aria-label="Zoom in">+</button>
    </div>)
    expect(rulesOver(container, ['planted · Zoom in'])).toEqual([
      'R4 the closed set moved: 2 glyph control(s) outside every segmented control now render as planted · Zoom in, where the census records 1',
    ])
    expect(clausesWoken(container, ['planted · Zoom in'])).toEqual(['R4'])
  })

  it('R4 passes over the nearest legitimate spelling — a glyph INSIDE a segmented control is not in the census', () => {
    // The same glyph, one row over. `Align left` is a member of a
    // `{components.segmented-control}` — two or more mutually-exclusive values of
    // one closed-set property, each carrying `aria-pressed` — which is the one
    // place V2 permits a glyph, so it never enters R4's set at all.
    const { container } = render(<div role="group" aria-label="Align">
      <button type="button" aria-pressed={true} aria-label="Align left">{glyph}</button>
      <button type="button" aria-pressed={false} aria-label="Align center">{glyph}</button>
    </div>)
    expect(rulesOver(container, [])).toEqual([])
  })

  it('R0 reds when a render state is dropped, and NAMES the groups that stopped being checked', async () => {
    // THE FAILURE THE COVERAGE CLAUSE EXISTS FOR, EXECUTED rather than claimed.
    // Drop the preview state — the cheapest way for this guard to quietly get
    // smaller. The control and class-family totals STILL CLEAR their floors, so
    // neither notices. The group-instance floor does red, but only as a smaller
    // number; it cannot say WHAT left. The by-name half names both groups that
    // stopped being checked, which is the whole point of recording them by name.
    //
    // STORY 14.2 moved this from `slice(0, 3)` to `slice(0, -1)`. It is the
    // SAME mutation — drop the preview state — but the declared states are no
    // longer three, and a fixed index would have quietly dropped the new Line
    // and Rectangle states too, taking `Orientation` with them and proving
    // something other than what this clause claims to prove.
    //
    // STORY 14.7 KEPT `preview` LAST FOR EXACTLY THAT REASON: its Table Editor
    // state is inserted BEFORE `preview`, not appended, so this mutation still
    // drops the state whose two groups the expectation below names. The
    // expected list was re-derived by running it, never edited by hand.
    const shrunk = await sweepStates(states.slice(0, -1))
    // ⚠ STORY 14.8's OWN PROOF, AND IT IS A MEASURED NULL-DIFF RATHER THAN A
    // GREEN SUITE. That story split the Table Editor's one undivided run into
    // HEADER / CELLS / BORDERS, and the arm it chose — three `<h3>` headings
    // inside the ONE existing `role="group"` — spends nothing on this guard only
    // if it really added no group instance. "The suite stayed green" does not
    // establish that: a sweep that stopped visiting the Table Editor state
    // altogether would also stay green here, because the pinned list below is
    // about the states that REMAIN. The count is therefore asserted as a NUMBER,
    // against the floor it must stay under, so a heading promoted to a group
    // (whether by `role="group"` or by `aria-labelledby`) reddens with the
    // number that moved rather than with a list that happens to still match.
    //
    // ⚠ AND THE MEASUREMENT IS COMPARED AGAINST THE CONSTANT, NOT THE CONSTANT
    // AGAINST ITSELF. `expect(GROUP_INSTANCE_FLOOR).toBe(33)` was a guard that
    // could not fail: anyone editing the constant edits the assertion in the same
    // breath, so it pinned a literal to its own literal and said nothing about
    // the sweep. What actually has to hold is the RELATION — the shrunk sweep
    // must stay UNDER the floor, because that is the only condition under which
    // the pinned R0 clause below reports a dropped state rather than nothing.
    const shrunkGroups = shrunk.flatMap((entry) => groupsIn(entry.root)).length
    expect(shrunkGroups, 'Story 14.8 regrouped the table editor with headings, not groups — a group instance appearing here would clear GROUP_INSTANCE_FLOOR and turn the pinned clause below into a guard that cannot fail').toBe(38)
    expect(shrunkGroups, `the shrunk sweep must stay under GROUP_INSTANCE_FLOOR (${GROUP_INSTANCE_FLOOR}) or the pinned R0 clause below stops proving that a dropped state is reported`).toBeLessThan(GROUP_INSTANCE_FLOOR)
    expect(shrunk.flatMap((entry) => entry.controls).length).toBeGreaterThanOrEqual(CONTROL_FLOOR)
    expect(new Set(shrunk.flatMap((entry) => entry.controls).flatMap((control) => control.classes)).size).toBeGreaterThanOrEqual(CLASS_FAMILY_FLOOR)
    expect(r0Violations(shrunk)).toEqual([
      'R0 the sweep visited 38 group instances, under the floor of 39',
      'R0 the group "PDF navigation" renders in no declared state, so nothing checked it',
      'R0 the group "Render actions" renders in no declared state, so nothing checked it',
    ])
  })

  it('R0 reds when a recorded group slips below the arity R2 needs, rather than reporting it covered', () => {
    // The other half of P4's disclosure: a group that still RENDERS but stops
    // holding two swept controls is one R2 returns early on. Before this clause
    // it would have kept its place in the visited set and read as checked.
    const { container } = render(<div>
      <div role="group" aria-label="Designer mode"><button type="button">DESIGN</button></div>
    </div>)
    expect(r0Violations([{ state: 'planted', root: container, controls: sweepControls(container, 'planted') }])
      .filter((message) => message.includes('Designer mode'))).toEqual([
      'R0 the group "Designer mode" is no longer checked by R2: it renders 1 non-empty control(s), under the arity of 2',
      'R0 a group is present but under R2\'s arity and is not recorded as such: "Designer mode" (1 non-empty control(s))',
    ])
  })

  // -------------------------------------------------------------------------
  // AC2 / AC3 / AC4 — the two surfaces this story respells, asserted where the
  // clauses above cannot: a clause says "no member disagrees", not "this member
  // says Open".
  // -------------------------------------------------------------------------

  // OWNER RULING after 14.1: the document bar is six GLYPHS, still one family in
  // one named group, still answering to every name it had as words.
  it('draws all six local-file controls as glyphs inside one named group', async () => {
    const root = await states[0]!.open()
    const group = screen.getByRole('group', { name: 'Local file actions' })
    expect(root.contains(group)).toBe(true)
    const members = controlsIn(group).map((element) => ({ name: accessibleName(element), treatment: treatmentOf(element), text: visibleText(element) }))
    expect(members).toEqual([
      { name: 'Open local template', treatment: 'glyph', text: '' },
      { name: 'Save local template', treatment: 'glyph', text: '' },
      { name: 'Save As', treatment: 'glyph', text: '' },
      { name: 'New…', treatment: 'glyph', text: '' },
      { name: 'Undo', treatment: 'glyph', text: '' },
      { name: 'Redo', treatment: 'glyph', text: '' },
    ])
  })

  // The documentation link joins the bar in the same glyph vocabulary, as a
  // LINK in a group of its own — never a seventh member of the file actions.
  it('draws the documentation link as one glyph link in its own named group, outside the six file actions', async () => {
    const root = await states[0]!.open()
    const group = screen.getByRole('group', { name: 'Documentation' })
    expect(root.contains(group)).toBe(true)
    expect(controlsIn(group), 'the documentation group holds a link, not a button').toEqual([])
    const links = Array.from(group.querySelectorAll('a[href]'))
    expect(links.map((element) => ({ name: accessibleName(element), treatment: treatmentOf(element), text: visibleText(element), title: element.getAttribute('title') }))).toEqual([
      { name: 'Rendering library documentation', treatment: 'glyph', text: '', title: null },
    ])
    expect(controlsIn(screen.getByRole('group', { name: 'Local file actions' }))).toHaveLength(6)
    expect(screen.getByRole('group', { name: 'Local file actions' }).querySelectorAll('a')).toHaveLength(0)
  })

  it('draws both TYPOGRAPHY segmented controls in one vocabulary, with every accessible name intact', async () => {
    await states[1]!.open()
    const align = controlsIn(screen.getByRole('group', { name: 'Align' }))
    const valign = controlsIn(screen.getByRole('group', { name: 'Vertical align' }))
    expect(align.map(accessibleName)).toEqual(['Align left', 'Align center', 'Align right', 'Align justify'])
    expect(valign.map(accessibleName)).toEqual(['Vertical align top', 'Vertical align middle', 'Vertical align bottom'])
    expect([...align, ...valign].map(treatmentOf)).toEqual(Array.from({ length: 7 }, () => 'glyph'))
    // No visible word survives anywhere in either row — the defect was words
    // BESIDE icons at the same size, so an icon plus a caption would not fix it.
    expect([...align, ...valign].map(visibleText)).toEqual(Array.from({ length: 7 }, () => ''))
    // ⚠ AND THE SEVEN PICTURES ARE SEVEN DIFFERENT PICTURES. Everything above is
    // satisfied by a control drawing the same glyph three times: each segment
    // would still hold one `svg.segment-icon`, still show no text, still classify
    // `glyph`, still answer to its own name — and Vertical align would be
    // unreadable. `valignGlyphs` is a one-line record of three path strings,
    // which is precisely where a copy/paste slip lives; nothing else in this
    // suite would see it. Asserted over the path data, so two segments cannot
    // draw the same stroke.
    const paths = [...align, ...valign].map((segment) => segment.querySelector('svg.segment-icon path')?.getAttribute('d') ?? '')
    expect(paths.filter((data) => data === '')).toEqual([])
    expect(new Set(paths).size).toEqual(paths.length)
  })

  it('offers Align three ways when a table is in the selection, still in one vocabulary', async () => {
    await states[2]!.open()
    const align = controlsIn(screen.getByRole('group', { name: 'Align' }))
    expect(align.map(accessibleName)).toEqual(['Align left', 'Align center', 'Align right'])
    expect(align.map(treatmentOf)).toEqual(['glyph', 'glyph', 'glyph'])
  })
})
