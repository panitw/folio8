import { useMemo, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { SCALAR_BINDING_COMPONENT_TYPES, type CanvasComponentType } from './engine-protocol'
import { formatRenderSize } from './preview/evidence-rail-facts'
import type { SampleData, SampleNode } from './sample-data'

type VisibleNode = Readonly<{ node: SampleNode; level: number; key: string; parent?: string; rootKey?: string; inCollection: boolean }>
// Everything the designed row needs, derived once per visible node from the
// COMPONENT KIND (inherited, never re-spelled) and the PATH's root namespace
// and shape. Nothing here inspects a scalar's sampled runtime kind.
type RowShape = Readonly<{ pickable: boolean; dimmed: boolean; marker?: string; value?: string; badge?: string; reason?: string }>

// This panel retains only local discovery/selection interaction. It never turns
// a display path into folio8 syntax, and it never re-derives the engine's
// component-type gate: `SCALAR_BINDING_COMPONENT_TYPES` and its mirror in
// `engine-bounds-mirror.test.ts` are INHERITED here.
//
// ⚠ AMENDED BY STORY 14.6, AND THE AMENDMENT IS THE POINT. Story 14.4 left this
// comment saying the panel "never decides whether a PATH is bindable", citing
// D-6.2.1. That over-reached, and the over-reach cost an author a broken render.
// D-6.2.1 governs COMMAND LEGALITY — Go accepts `segments:["items"]` for an
// empty collection and the mismatch surfaces at render — and
// `folio-go/component_commands_test.go` says in its own words that this "is
// legal command grammar even though a picker would withhold an observed
// collection". Withholding a COLLECTION, and withholding the `params` namespace
// Go refuses outright at `component_commands.go:749-751`, is therefore
// contemplated by that ruling rather than forbidden by it.
//
// What the panel still never does is pre-judge whether a SCALAR path will yield
// a scalar at runtime. That is the binder's call; a second copy of it here would
// drift the moment the runtime data differed from the sample.
// STORY 14.10 WIDENED THIS BY TWO OPTIONAL MEMBERS, and they are optional
// because a COLUMN refusal is scoped by different facts than a scalar one. A
// scalar bind is identified by the component and the picked PATH; a column bind
// has no path at all — a row-scope leaf carries no `segments` — so it is
// identified by the column and the row-relative FIELD. Re-narrowing on the
// wrong pair is how a refusal ends up rendered beside a pick that did not
// cause it.
export type BindingErrorScope = Readonly<{ sample: SampleData; componentID: string; segments: ReadonlyArray<string>; message: string; columnId?: string; field?: string }>

// STORY 14.10 — WHAT THE PANEL NEEDS TO OFFER A TABLE COLUMN ITS ROW FIELDS,
// AND IT IS ONE SET RATHER THAN A SECOND DERIVATION.
//
// `rowFields` IS `tableSampleCandidates`' OWN ANSWER, threaded in from App.tsx
// (Q2(a)). The panel cannot derive it: a row-scope leaf carries no `segments`,
// so the row-relative path the engine wants is not on the node. And it must not
// derive it — `tableSampleCandidates` already builds the collection key
// byte-identically to the canvas's `tableBind`, and that agreement with the
// engine is shipped and exercised. A second derivation would owe a proof that
// the two agree.
//
// ⚠ ONE SET, BOTH CONSUMERS. Pickability is `rowFields.has(node)` and AC4's
// refusal reason is stated exactly when that is false. Computing the two from
// different sets is how a panel comes to say "outside row scope" about a path
// it is also offering (D-000.25's shape).
export type ColumnBindScope = Readonly<{ tableId: string; columnId: string; label: string; collection: string; rowFields: ReadonlyMap<SampleNode, string> }>

// The engine's own discovered parameter namespace, projected by the host from
// `parameter-references` plus the Preview parameter document. It is a DIFFERENT
// SOURCE from a `params` key in the author's sample JSON, and neither may stand
// in for the other: the engine finds these in the TEMPLATE, so they exist even
// when no sample is loaded and even when the sample has no `params` key.
export type RuntimeParameters = Readonly<{ status: 'pending' | 'ready' | 'failed'; names: ReadonlyArray<string>; values: Readonly<Record<string, string>> }>

// The capitalised noun the selection context bar leads with. Kept total over the
// projection's kinds so a kind added there cannot reach the bar without a word,
// and spelled `Rectangle` for `rect` because the abbreviation is not a word.
const kindNoun: Readonly<Record<CanvasComponentType, string>> = { text: 'Text', image: 'Image', table: 'Table', line: 'Line', rect: 'Rectangle', barcode: 'Barcode', qrcode: 'QR code' }

export function DataPanel({ sample, error, busy, available, saveDisabled, selectedComponentId, selectedComponentType, selectedBinding, bindingError, bindingBusy, runtimeParameters, columnScope, onLoad, onSave, onConnect, onConnectColumn }: Readonly<{ sample?: SampleData; error?: string; busy: boolean; available: boolean; saveDisabled?: boolean; selectedComponentId?: string; selectedComponentType?: CanvasComponentType; selectedBinding?: string; bindingError?: BindingErrorScope; bindingBusy?: boolean; runtimeParameters?: RuntimeParameters; columnScope?: ColumnBindScope; onLoad: () => void; onSave?: () => void; onConnect?: (segments: ReadonlyArray<string>) => void; onConnectColumn?: (field: string) => void }>) {
  const action = sample ? 'Replace sample JSON' : 'Load sample JSON'
  const [pickedState, setPickedState] = useState<Readonly<{ sample: SampleData; node: SampleNode }>>()
  const picked = pickedState && pickedState.sample === sample ? pickedState.node : undefined
  const candidate = picked?.segments
  // The row-relative field the picked node stands for, or undefined when no
  // column is selected. It is read out of the threaded set, never recomputed.
  const pickedColumnField = picked && columnScope ? columnScope.rowFields.get(picked) : undefined
  // ⚠ NARROWED ON BOTH IDS, BECAUSE THAT IS THE KEY THE SELECTION ITSELF IS
  // KEYED ON. App.tsx:311-313 states it outright — "a bare column id is not
  // unique across tables, and a column id that outlives its table would resolve
  // against whichever table happened to reuse the spelling" — and a refusal
  // narrowed on the column id alone contradicts the state shape it is reading.
  // `bindPickedColumn` records the owning table in `componentID`, so the table
  // half is already on the record and only had to be asked for. Without it a
  // refusal raised against one table's column `eN` renders beside a DIFFERENT
  // table's column `eN` the moment the author selects it — the scalar arm has
  // always compared its component, and this arm now compares its table.
  const currentBindingError = bindingError && sample === bindingError.sample && (columnScope
    ? bindingError.componentID === columnScope.tableId && bindingError.columnId === columnScope.columnId && bindingError.field !== undefined && bindingError.field === pickedColumnField
    : selectedComponentId === bindingError.componentID && candidate && sameSegments(candidate, bindingError.segments)) ? bindingError.message : undefined
  // THE COMPONENT-KIND GATE, INHERITED FROM STORY 14.4 AND NOT RE-DERIVED. It
  // fails CLOSED on an unknown kind — the id is passed from the selection
  // unconditionally while the kind comes from the projection, so an absent
  // canvas or an id no longer in the projection yields id-present/kind-absent,
  // and a panel that does not know the kind cannot know the engine will accept
  // it.
  const bindableKind = selectedComponentType !== undefined && SCALAR_BINDING_COMPONENT_TYPES.includes(selectedComponentType)
  // A whole table offers root collections; a selected column keeps its own
  // row-field scope and never dispatches a collection pick.
  const collectionMode = selectedComponentId !== undefined && selectedComponentType === 'table' && columnScope === undefined
  const columnBindable = columnScope !== undefined && columnScope.collection !== ''
  const accented = columnScope ? columnBindable : bindableKind || collectionMode
  const context = columnScope
    ? columnBindable
      ? `Column ${columnScope.label} selected · binding to a row field of ${columnScope.collection}`
      : `Column ${columnScope.label} selected · its table is bound to no collection, so it has no row fields to offer.`
    : selectedComponentId === undefined
    ? 'No single component selected · select one component, then pick a path.'
    : selectedComponentType === undefined
      ? 'The selected component is not in the current projection · no path can be bound to it.'
      : selectedComponentType === 'table'
        ? 'Table selected · pick a root collection to bind its rows.'
        : bindableKind
          ? `${kindNoun[selectedComponentType]} selected · binding to string`
          : `${kindNoun[selectedComponentType]} selected · only text, barcode and QR code components can receive a scalar binding.`
  // A PICK BINDS IMMEDIATELY (owner ruling, 2026-09-09). There is no
  // intermediate "connect" control; the mockup's omission of one is a design
  // decision. The bind is undoable like any other edit, and the engine still
  // validates and can still refuse. The dispatch is withheld only where the
  // panel already states a refusal in the bar above the tree, so no gesture
  // sends a command the engine is known to reject.
  const bind = (node: SampleNode) => {
    // ⚠ THE ACCENT MARKS A BIND, NOT A GESTURE. `.data-tree-picked` is the bind
    // accent, so setting it before the dispatch is decided paints a row as
    // bound when the command was withheld — a refused kind, no selection, a
    // bind already in flight, or no `onConnect` at all. Decide first, then mark.
    if (!sample) return
    // STORY 14.10 — A COLUMN PICK COMMITS THE ROW-RELATIVE FIELD THE THREADED
    // SET NAMES, and a node the set does not name is not dispatchable at all —
    // the same membership test that decided pickability, asked again at the
    // dispatch so a stale row cannot send a field this table has no business
    // binding.
    if (columnScope) {
      const field = columnScope.rowFields.get(node)
      if (field === undefined || bindingBusy || onConnectColumn === undefined) return
      setPickedState({ sample, node })
      onConnectColumn(field)
      return
    }
    if (!node.segments) return
    if ((!bindableKind && !collectionMode) || selectedComponentId === undefined || bindingBusy || onConnect === undefined) return
    if (collectionMode !== (node.kind === 'collection')) return
    setPickedState({ sample, node })
    onConnect(node.segments)
  }
  return <div className="data-panel" aria-label="Data panel">
    {/* STORY 5 (startup templates) — SAVE SITS BESIDE LOAD, AND ONLY ONCE THERE
        IS SOMETHING TO SAVE. It is withheld rather than disabled when no sample
        is loaded: there is no file the author could mean, so there is no reason
        to state beside a dead control. Once a sample exists the control is
        always rendered and only ever DISABLED — `saveDisabled` carries the file
        boundary's own two conditions (a local write already in flight, or no
        local file access at all), which are not `busy`/`available`'s: those two
        describe the SAMPLE picker, and a sample can be loaded from a shell whose
        save tier is a download and vice versa. */}
    <div className="data-actions">
      <button className="file-button" type="button" onClick={onLoad} disabled={busy || !available}>{action}</button>
      {sample && onSave && <button className="file-button" type="button" onClick={onSave} disabled={saveDisabled}>Save sample data</button>}
    </div>
    {error && <p role="alert" className="data-message">{error}</p>}
    {!sample ? <>
      <p className="data-empty" role="status">No sample data loaded.</p>
      <p className="honest-note">{available ? 'Load one local JSON document to inspect its paths.' : 'Local sample selection is unavailable in this shell.'}</p>
      <p className="honest-note">Sample data is never written into the template.</p>
      <RuntimeParameterSection parameters={runtimeParameters} />
    </> : <>
      <p className="data-file" role="status"><span className="data-file-name">{sample.name}</span><code className="data-file-size">{formatRenderSize(sample.bytes.byteLength)}</code></p>
      <p className={`binding-chip data-context${accented ? '' : ' data-context-refused'}`} role="status">{accented && <span className="binding-dot" aria-hidden="true" />}{context}</p>
      {selectedBinding && <p className="binding-status" role="status">Current engine binding: <code>{selectedBinding}</code></p>}
      {sample.truncated && <p className="data-message" role="status">Tree inspection is truncated to keep this local panel responsive.</p>}
      <p className="section-label">PATHS</p>
      <DataTree key={treeIdentity(sample.tree)} root={sample.tree} picked={picked} scope={columnScope} collectionMode={collectionMode} onPick={bind} />
      <RuntimeParameterSection parameters={runtimeParameters} />
      {bindingBusy && <p className="binding-status" role="status">Asking the engine to bind the picked path…</p>}
      {currentBindingError && <p className="data-message" role="alert">{currentBindingError}</p>}
      <p className="honest-note">Sample data is never written into the template.</p>
    </>}
  </div>
}

// AC6's namespace, and it is DISPLAY ONLY. Nothing in here is a control: no
// button, no treeitem, no pick handler and no `segments`, because Go refuses a
// `params` root as a data binding outright. The author sees that the namespace
// exists and what each name currently holds; that is the whole affordance.
function RuntimeParameterSection({ parameters }: Readonly<{ parameters?: RuntimeParameters }>) {
  if (!parameters) return null
  return <section className="data-params" aria-label="Runtime parameters from the template">
    <p className="data-params-head"><span className="tree-label">params</span><span className="tree-badge">RUNTIME</span></p>
    {parameters.status === 'pending'
      ? <p className="tree-reason" role="status">Discovering runtime parameters from the local engine…</p>
      : parameters.status === 'failed'
        // Never guess an empty list: "the engine could not say" and "the
        // template declares none" are different facts and read differently.
        ? <p className="tree-reason" role="status">The local engine could not provide the runtime parameters for this template.</p>
        : parameters.names.length === 0
          ? <p className="tree-reason">The local engine found no runtime parameters in this template.</p>
          : <ul className="data-params-list">{parameters.names.map((name) => <li key={name} className="data-param-row"><span className="tree-label">{name}</span>{parameters.values[name] === undefined ? <span className="tree-value tree-value-unset">not set</span> : <span className="tree-value">{parameterText(parameters.values[name]!)}</span>}</li>)}</ul>}
    <p className="tree-reason">Runtime parameters are supplied in Preview · the engine does not bind one as a data path.</p>
  </section>
}

function DataTree({ root, picked, scope, collectionMode, onPick }: Readonly<{ root: SampleNode; picked?: SampleNode; scope?: ColumnBindScope; collectionMode: boolean; onPick: (node: SampleNode) => void }>) {
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set([keyFor(root, 0)]))
  const visible = useMemo(() => flatten(root, expanded), [root, expanded])
  const [active, setActive] = useState(() => keyFor(root, 0))
  const focus = (key: string) => { setActive(key); requestAnimationFrame(() => Array.from(document.querySelectorAll<HTMLElement>('[data-tree-key]')).find((element) => element.dataset.treeKey === key)?.focus()) }
  const toggle = (key: string) => setExpanded((value) => { const next = new Set(value); if (next.has(key)) next.delete(key); else next.add(key); return next })
  // Activation binds an offered collection and toggles its disclosure. Arrow
  // navigation remains browsing only, even on a bindable collection branch.
  const activate = (entry: VisibleNode) => {
    if (entry.node.children.length > 0) toggle(entry.key)
    if (rowFor(entry, scope, collectionMode).pickable) onPick(entry.node)
  }
  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>, current: VisibleNode) => {
    // Tree navigation is local interaction. Never let an arrow intended for a
    // focused discovery node become a canvas nudge shortcut.
    event.stopPropagation()
    const index = visible.findIndex((entry) => entry.key === current.key)
    const move = (next: number) => { event.preventDefault(); if (visible[next]) focus(visible[next]!.key) }
    const branch = current.node.children.length > 0
    if (event.key === 'ArrowDown') return move(index + 1)
    if (event.key === 'ArrowUp') return move(index - 1)
    if (event.key === 'Home') return move(0)
    if (event.key === 'End') return move(visible.length - 1)
    if (event.key === 'ArrowRight' && branch) { event.preventDefault(); if (!expanded.has(current.key)) setExpanded((value) => new Set(value).add(current.key)); else if (visible[index + 1]?.parent === current.key) focus(visible[index + 1]!.key); return }
    if (event.key === 'ArrowLeft') { event.preventDefault(); if (branch && expanded.has(current.key)) setExpanded((value) => { const next = new Set(value); next.delete(current.key); return next }); else if (current.parent) focus(current.parent); return }
    if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); activate(current) }
  }
  return <ul className="data-tree" role="tree" aria-label="Sample data paths">{visible.map((entry) => {
    const shape = rowFor(entry, scope, collectionMode)
    const branch = entry.node.children.length > 0
    // ⚠ EVERY BADGE AND EVERY REASON IS A PLAIN `<span>` INSIDE THE TREEITEM
    // BUTTON, never a control of its own: a badge must be announced as part of
    // the row it qualifies, and a second focusable thing on a tree row would
    // break the roving tab stop as well as lie about what is operable.
    return <li className={`data-tree-node${picked === entry.node ? ' data-tree-picked' : ''}${shape.dimmed ? ' data-tree-dim' : ''}`} role="none" key={entry.key} style={{ '--tree-level': entry.level } as CSSProperties}><button className="tree-item" type="button" role="treeitem" data-tree-key={entry.key} aria-level={entry.level} aria-expanded={branch ? expanded.has(entry.key) : undefined} aria-selected={shape.pickable ? picked === entry.node : undefined} aria-disabled={!branch && !shape.pickable ? true : undefined} tabIndex={active === entry.key ? 0 : -1} onFocus={() => setActive(entry.key)} onClick={() => activate(entry)} onKeyDown={(event) => onKeyDown(event, entry)}><span className="tree-row">{shape.pickable && <span className="binding-dot" aria-hidden="true" />}<span className="tree-label">{entry.node.label}</span>{shape.marker && <span className="tree-marker">{shape.marker}</span>}{shape.value !== undefined && <span className="tree-value">{shape.value}</span>}{shape.badge && <span className="tree-badge">{shape.badge}</span>}</span>{shape.reason && <span className="tree-reason">{shape.reason}</span>}</button></li>
  })}</ul>
}

// Row offering is a presentation decision; Go still owns command admission.
// Collection segments are retained for table discovery even in scalar mode.
const rowFor = (entry: VisibleNode, scope: ColumnBindScope | undefined, collectionMode: boolean): RowShape => {
  const node = entry.node
  const collection = node.kind === 'collection'
  const runtime = entry.rootKey === 'params'
  const scoped = entry.inCollection
  // STORY 14.10 — COLUMN MODE, AND IT IS A DIFFERENT QUESTION RATHER THAN A
  // RELAXATION OF THE ONE BELOW.
  //
  // With a column selected the panel is not asking "may a TEXT component bind
  // this path" — it is asking "is this node one of THIS table's row fields",
  // which is membership in the threaded set and nothing else. Flipping `scoped`
  // out of the expression below would not have worked and would have been the
  // wrong shape anyway: a row-scope leaf carries no `segments`, so there is no
  // path here to offer.
  //
  // ⚠ THE REASON IS STATED EXACTLY WHEN THE MEMBERSHIP TEST SAYS NO, so the
  // panel cannot refuse a path it also offers, and it is stated BEFORE the
  // engine would refuse it (AC4, UX-DR24). A BRANCH that is neither a
  // collection nor a candidate — an object inside the row whose own leaves are
  // the candidates — is left unmarked: it is operable, it expands, and calling
  // it refused would misdescribe the one gesture it answers.
  //
  // ⚠ THREE THINGS THE FIRST SPELLING OF THIS BRANCH GOT WRONG, all of them on
  // reachable paths and all of them fixed here rather than papered over.
  //
  // (1) AN UNBOUND TABLE HAS NO COLLECTION TO NAME. `columnBindScope` is built
  //     whenever a column is selected — only `rowFields` is emptied when the
  //     table binds nothing — so `scope.collection` is `''` on a perfectly
  //     normal mid-authoring table, and the sentences below interpolated it
  //     into a missing noun and a double space. UX-DR24 wants the reason to
  //     name the location, and "a row field of " names nothing. The unbound
  //     case gets its own sentences, in the register the context bar already
  //     ships one sentence up. Suppressing them instead is not open: DESIGN.md
  //     requires a stated reason next to anything disabled.
  //
  // (2) `TABLE ONLY` IS INHERITED VOCABULARY THAT STOPPED BEING TRUE HERE. The
  //     badge was minted for the scalar gate, where it means "a table, not this
  //     text component, is what binds a collection". In column mode the author
  //     is inside a table, being told this table's column cannot bind the
  //     collection — the badge and the sentence beside it then contradict each
  //     other. It is dropped rather than reworded: the sentence already says
  //     the whole of what is true, and a second, terser half-truth beside it
  //     only competes with it.
  //
  // (3) A `params` LEAF IS NOT MERELY "NOT A ROW FIELD". `runtime` was computed
  //     here and read only by the badge, so a params path fell through to the
  //     row-scope sentence — accurate but silent about the refusal that
  //     actually governs it. Go refuses a `params` root as a data binding
  //     outright (`component_commands.go:749-751`), for a column exactly as for
  //     a scalar, so it is stated first and in the shipped runtime register.
  if (scope) {
    const pickable = scope.rowFields.has(node)
    const unbound = scope.collection === ''
    return {
      pickable,
      dimmed: !pickable && node.children.length === 0,
      marker: node.kind === 'object' ? '{ }' : undefined,
      value: node.children.length === 0 ? valueText(node) : undefined,
      reason: pickable
        ? undefined
        : runtime
          ? 'Runtime parameter · not a row field, and the engine refuses a params path as a data binding.'
          : collection
            ? unbound
              ? 'Collection · this column’s table is bound to no collection, so it has no row scope.'
              : `Collection · a column binds one row field of ${scope.collection}, never a collection.`
            : node.children.length > 0
              ? undefined
              : unbound
                ? 'Not a row field · this column’s table is bound to no collection.'
                : `Not a row field of ${scope.collection} · that is all a table column can bind.`,
    }
  }
  if (collectionMode) {
    const pickable = collection && !runtime && !scoped && node.segments !== undefined && node.segments.length > 0
    return {
      pickable,
      dimmed: !pickable && node.children.length === 0,
      marker: node.kind === 'object' ? '{ }' : undefined,
      value: node.children.length === 0 ? valueText(node) : undefined,
      reason: runtime
        ? 'Runtime parameter · the engine refuses a params path as a data binding.'
        : scoped
          ? 'Inside a collection · a table requires a root collection.'
          : collection
            ? pickable
              ? `Collection · ${node.count ?? 0} ${node.count === 1 ? 'item' : 'items'}. Pick to bind this table.`
              : 'Collection · no complete root key path is available to bind this table.'
            : node.children.length > 0
              ? undefined
              : 'A table binds a root collection, not a scalar or object.',
    }
  }
  return {
    pickable: node.segments !== undefined && !collection && !runtime && !scoped,
    // DESIGN.md:550 — a node that can be neither picked NOR expanded is
    // disabled, so it drops to 0.42 AND states a reason. An expandable branch
    // is not disabled: it is operable, and dimming it would misdescribe the one
    // gesture it does answer.
    dimmed: runtime || scoped || (node.children.length === 0 && (collection || node.kind === 'object')),
    marker: node.kind === 'object' ? '{ }' : undefined,
    value: node.children.length === 0 ? valueText(node) : undefined,
    badge: collection && !runtime ? 'TABLE ONLY' : undefined,
    reason: runtime
      ? 'Runtime parameter · the engine refuses a params path as a data binding.'
      : collection
        ? `Collection · ${node.count ?? 0} ${node.count === 1 ? 'item' : 'items'}. Text cannot bind a collection.`
        : scoped
          ? 'Inside a collection · a table row binds these, not a text component.'
          : node.kind === 'object' && node.children.length === 0
            ? 'Empty object · it holds no value to bind.'
            : undefined,
  }
}

function flatten(root: SampleNode, expanded: ReadonlySet<string>): VisibleNode[] {
  const result: VisibleNode[] = []
  // `rootKey` is the label of the level-2 ancestor — the JSON document's own
  // top-level key, which is the only thing Go's root-namespace refusal looks at.
  // It is threaded rather than read off a node because an OBJECT never carries
  // `segments`, so `params` itself and every branch under it would otherwise
  // have no way to say which namespace it belongs to.
  const visit = (node: SampleNode, level: number, parent: string | undefined, ordinal: number, rootKey: string | undefined, inCollection: boolean) => {
    const key = parent ? `${parent}/${keyFor(node, ordinal)}` : keyFor(node, ordinal); result.push({ node, level, key, parent, rootKey, inCollection })
    if (node.children.length && expanded.has(key)) node.children.forEach((child, index) => visit(child, level + 1, key, index, rootKey ?? (child.kind === 'collection' ? child.label.slice(0, -2) : child.label), inCollection || node.kind === 'collection'))
  }
  visit(root, 1, undefined, 0, undefined, false); return result
}

const keyFor = (node: SampleNode, ordinal: number): string => `${node.path}:${ordinal}`
// THE VALUE, NOT THE TYPE NAME. `preview` holds a string leaf JSON-quoted, which
// is the right spelling for a machine-readable projection and the wrong one for
// a row that exists to show an author their own data. The quotes come off; the
// bounded-preview ellipsis stays where it is.
//
// ⚠ THE QUOTES ARE STRIPPED, NOT PARSED. `JSON.parse` here would put this module
// on `engine-ownership-contract.test.ts`'s document-JSON census for the sake of
// a preview, and this panel has no business decoding anything. What is shown is
// the projected token with its delimiters removed.
const unquoted = (text: string): string => text.replace(/^"/, '').replace(/"(…?)$/, '$1')
const valueText = (node: SampleNode): string | undefined => node.preview === undefined ? undefined : node.kind === 'string' ? unquoted(node.preview) : node.preview
// A parameter value arrives as the raw JSON token slice the Preview document
// holds, because the token locator that produced it never parses either. A
// quoted string reads as its text; anything else is shown as it was written.
const parameterText = (raw: string): string => raw.length > 1 && raw.startsWith('"') && raw.endsWith('"') ? unquoted(raw) : raw
// DESIGN.md:331-334 names a byte count among the values set in mono, so the
// header states the exact accepted byte length rather than the mockup's rounded
// ⚠ AC3's OWN FORMAT, AND A SHIPPED FORMATTER RATHER THAN A THIRD ONE.
// An earlier spelling here printed an exact `18,432 bytes`, reasoning that
// DESIGN.md outranks the mockup. It does — but `DESIGN.md:331-334` is a rule
// about which TYPEFACE a machine-read value is set in ("is set in mono"), not
// about how the number is formatted; it is silent on rounding. AC3 is normative
// and names the rendering outright — `sample-statement.json · 18 KB` — and
// DESIGN.md:418 calls the load screen's size display "the megabyte count", so
// where the design does speak about a size it speaks in rounded units. Mockup
// and AC agree, DESIGN.md does not contradict them, and citing it to overrule
// both was reaching.
// `formatRenderSize` is the product's existing size formatter (the evidence
// rail's). It chooses the unit AFTER rounding — the documented reason it never
// prints `1024 KB` — and trims a trailing `.0`. Reusing it keeps one rounding
// rule in the product instead of two that can disagree. The <code> element it
// sits in is what satisfies DESIGN.md's mono requirement.
const sameSegments = (left: ReadonlyArray<string>, right: ReadonlyArray<string>) => left.length === right.length && left.every((segment, index) => segment === right[index])
// A changed visible path set remounts this transient discovery widget, so its
// initial root remains the one roving tab stop after sample replacement.
const treeIdentity = (node: SampleNode): string => `${node.path}:${node.children.map(treeIdentity).join('|')}`
