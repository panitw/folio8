import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { familyIsInstalled, indexCategories, indexScripts, type FamilySource } from './font-index'
import { browserRows, browserSorts, browserViews, buttonLabel, buttonName, confirmLabel, confirmName, degradedFooterNote, defaultSpecimenSize, emptyStateHeading, emptyStateHint, filterRows, filtersActive, maxSpecimenSize, minSpecimenSize, noFilters, pageCount, pageLine, pageOf, pendingLine, resultLine, rowState, rowTierNote, scriptBadge, sizeReadout, sortRows, specimenFor, specimenSize, weightLine, type BrowserFilters, type BrowserRow, type BrowserSort, type BrowserView } from './font-browser-model'
import { previewFaceFamily } from './preview-face-family'
import { openPreviewFaceRegistry, type PreviewFaceBytes, type PreviewFaceRegistry, type PreviewFaceStatus } from './preview-face-registry'

// THE FONT BROWSER — THE MODAL `Font Browser.dc.html` DRAWS (Story 16.3).
//
// FIVE REGIONS, IN THE DESIGN'S OWN ORDER: header, rail, results, empty state,
// footer. Everything about WHAT they say is in `font-browser-model.ts`, ported
// from the mockup's own `renderVals()`; this file owns only how they are drawn
// and how a keyboard moves through them.
//
// THE STAGED SET IS UI STATE AND NOTHING ELSE (AD-15). It is a list of family
// NAMES held in this component and destroyed when it unmounts. There is no
// partial document, no uncommitted buffer and no second document model: until
// Add is pressed nothing has been sent, and pressing Cancel or Escape discards
// the list without a command. The scope is this modal because the design put a
// Cancel/Apply pair in this modal, and it is not a precedent for the inspector.
//
// THE HEADER'S METADATA SLOT DRAWS NOTHING, AND THAT IS THE DESIGN (Story 16.10).
// The mockup says *"web font library · 1,946 families"*, and both halves of that
// are wrong here: 1,946 is what the source PUBLISHED on the snapshot date, not
// what this designer can add, and "library" reads as live when the list is a
// build-time snapshot. THAT REFUSAL STANDS. What changes at 16.10 is that the
// three-clause disclosure paragraph this file drew in its place also goes: the
// design's header is a single 46px row and draws no paragraph at all, so
// following it strictly means drawing nothing in that slot rather than drawing a
// shorter claim of our own. Minting one would be the two-authorities-on-one-count
// defect this epic has now refused three times.
//
// THE COUNT IS NOT LOST WITH THE SENTENCE. `resultLine` prints
// `N of <addableFamilyCount> families` in the results toolbar below, from the
// same number the paragraph quoted, and that line is the browser's one authority
// on how many families it offers.
type Props = Readonly<{
  /** Every family the author may add, in the order `offeredFamilies` returns them. */
  sources: ReadonlyArray<FamilySource>
  /** The chains the document carries. A family here is `In template` and cannot be staged. */
  inTemplate: ReadonlyArray<string>
  /** The bytes a specimen is set in. Owned by the caller, because the tiers are. */
  previewBytes: PreviewFaceBytes
  /** THE SEAM. One call per staged family; resolves to a refusal sentence, or `undefined` when the family went in. */
  onAddFamily: (source: FamilySource) => Promise<string | undefined>
  /**
   * Whether this browser can keep typefaces on the machine at all. `false` puts
   * the dialog in the pre-16.5 degraded model, where confirming EMBEDS — which
   * the footer and the confirm control both say, because a confirm that adds five
   * faces to a document must not look like one that adds five to a machine.
   */
  storeKeepsFaces: boolean
  onClose: () => void
}>

type Refusal = Readonly<{ family: string; message: string }>

export function FontBrowser({ sources, inTemplate, previewBytes, onAddFamily, storeKeepsFaces, onClose }: Props) {
  const [filters, setFilters] = useState<BrowserFilters>(noFilters)
  const [sort, setSort] = useState<BrowserSort>('Trending')
  const [view, setView] = useState<BrowserView>('Row')
  const [previewText, setPreviewText] = useState('')
  const [size, setSize] = useState(defaultSpecimenSize)
  const [thaiSample, setThaiSample] = useState(true)
  const [page, setPage] = useState(0)
  const [staged, setStaged] = useState<ReadonlyArray<string>>([])
  const [busy, setBusy] = useState(false)
  const [progress, setProgress] = useState<Readonly<{ done: number; total: number }>>()
  const [refusals, setRefusals] = useState<ReadonlyArray<Refusal>>([])
  const [facesTick, setFacesTick] = useState(0)
  const [registry, setRegistry] = useState<PreviewFaceRegistry>()
  const dialog = useRef<HTMLElement>(null)
  const search = useRef<HTMLInputElement>(null)
  // The resolver is read through a ref so that a caller re-creating the closure
  // on every render cannot tear down and re-open the registry — which would
  // release every face on screen and fetch them all again, once per keystroke.
  const bytes = useRef(previewBytes)
  useEffect(() => { bytes.current = previewBytes })

  const rows = useMemo(() => browserRows(sources), [sources])
  const matching = useMemo(() => sortRows(filterRows(rows, filters), sort), [rows, filters, sort])
  const shown = pageOf(matching, page)
  const pages = pageCount(matching.length)
  const byFamily = useMemo(() => new Map(rows.map((row) => [row.family, row])), [rows])
  // WHICH FAMILIES THIS MACHINE ALREADY HOLDS (Story 16.5), read off each row's
  // own `FamilySource` through the one predicate the family control's fork also
  // reads. It is derived from `rows` — the UNFILTERED set — for the same reason
  // `byFamily` is: filtering, sorting and paging must not change what a family's
  // relationship to this machine IS.
  const installedFamilies = useMemo(() => rows.filter((row) => familyIsInstalled(row.source)).map((row) => row.family), [rows])

  // THE REGISTRY'S LIFETIME IS THIS COMPONENT'S. It opens once, on mount, and
  // its release runs on unmount — so closing the modal removes every preview
  // face from the page's font set and the canvas is left exactly as it was.
  useEffect(() => {
    const opened = openPreviewFaceRegistry((family) => bytes.current(family), () => setFacesTick((tick) => tick + 1))
    setRegistry(opened)
    return () => opened.close()
  }, [])

  // AND THE SET IT HOLDS IS THE SET ON SCREEN. Every filter, sort, page or view
  // change re-states the list; the registry releases what left and registers
  // what arrived. The bound is `familiesPerPage`, argued where it is defined.
  // NUL-JOINED, BECAUSE FAMILY NAMES CONTAIN SPACES. `Noto Sans Thai` split on
  // a space is three families the registry would look for and never find. It is
  // the same separator the document's own carried listing is joined with.
  const shownKey = shown.map((row) => row.family).join('\u0000')
  useEffect(() => { registry?.show(shownKey === '' ? [] : shownKey.split('\u0000')) }, [registry, shownKey])

  useEffect(() => { search.current?.focus() }, [])

  // THE REGISTRY IS THE AUTHORITY ON ITS OWN STATUSES AND IS NOT COPIED INTO
  // STATE — a copy would be a second answer to "is this face registered", and
  // the two would disagree for exactly one render. `facesTick` is read here
  // rather than passed anywhere: it is the notification that a status MOVED, so
  // touching it is what ties this render to the registry's last change.
  const statusOf = (family: string): PreviewFaceStatus => {
    void facesTick
    return registry?.statusOf(family) ?? 'preparing'
  }

  const amend = (patch: Partial<BrowserFilters>) => { setFilters((current) => ({ ...current, ...patch })); setPage(0) }
  const toggleCategory = (category: string) => amend({ categories: filters.categories.includes(category) ? filters.categories.filter((entry) => entry !== category) : [...filters.categories, category] })
  const toggleStaged = (family: string) => setStaged((current) => current.includes(family) ? current.filter((entry) => entry !== family) : [...current, family])

  // FOCUS IS TRAPPED AND ESCAPE CLOSES — contract, not polish (UX-DR25). The
  // shape is `TableEditor`'s, deliberately: this designer has one way of
  // trapping a dialog and a second one would be a second answer.
  const trap = (event: KeyboardEvent<HTMLElement>) => {
    // ESCAPE IS GATED ON `busy` BECAUSE EVERY OTHER CONTROL IS. Cancel, the ×,
    // the chips, the search field and confirm are all `disabled={busy}` while a
    // batch runs; leaving Escape live made the keyboard the one route to a
    // half-dismissed modal, with the remaining dispatches still running against
    // an unmounted component and their per-family refusals landing nowhere. The
    // batch is short, bounded and already announced — waiting for it is the
    // behaviour the rest of the footer promises.
    if (event.key === 'Escape') { event.preventDefault(); if (!busy) onClose(); return }
    if (event.key !== 'Tab') return
    const focusable = Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), select:not([disabled])') ?? []).filter((element) => element.tabIndex >= 0)
    if (focusable.length === 0) return
    const index = focusable.indexOf(document.activeElement as HTMLElement)
    if ((!event.shiftKey && index === focusable.length - 1) || (event.shiftKey && index <= 0)) {
      event.preventDefault()
      focusable[event.shiftKey ? focusable.length - 1 : 0]?.focus()
    }
  }

  // CONFIRM: ONE DISPATCH PER STAGED FAMILY, THROUGH THE ONE SEAM, IN ORDER.
  //
  // SEQUENTIAL AND NOT CONCURRENT, for two reasons that both bite. Each call is
  // its own history entry, and a document's history is an ordered thing — three
  // commands raced would land in whatever order the network settled in. And the
  // pick path holds a re-entry flag across its whole resolution, so a concurrent
  // second call would be dropped on the floor rather than queued.
  //
  // A REFUSAL IS PER FAMILY AND NAMED, AND THE REST PROCEED. One family failing
  // to fetch is that family's fact; abandoning the other two would make an
  // upstream 404 lose work the author had already done.
  const confirm = async () => {
    if (busy || staged.length === 0) return
    setBusy(true)
    setRefusals([])
    setProgress({ done: 0, total: staged.length })
    const failed: Refusal[] = []
    let handled = 0
    try {
      for (const family of staged) {
        const row = byFamily.get(family)
        // UNREACHABLE TODAY, AND NAMED RATHER THAN SKIPPED. `byFamily` is built
        // from the UNFILTERED row set, so a family that could be staged is
        // always in it — filtering, sorting and paging never remove it. It used
        // to `continue`, which would have dropped the family from the staged set
        // with no refusal, no progress and nothing said. If the two sets ever
        // stop agreeing, this says so against the family it happened to.
        const refusal = row === undefined
          ? `${family} is no longer among the families this designer offers.`
          // A REJECTION IS A REFUSAL, NOT A DEAD MODAL. The seam resolves to a
          // sentence for every failure it anticipates; anything it does NOT
          // anticipate arrives here as a throw, and letting it escape the loop
          // left `busy` true for ever — every control disabled, with no way out.
          : await onAddFamily(row.source).catch((error: unknown) => `${family} could not be added: ${error instanceof Error ? error.message : String(error)}`)
        handled += 1
        if (refusal !== undefined) { failed.push({ family, message: refusal }); setRefusals([...failed]) }
        setProgress({ done: handled, total: staged.length })
      }
      // The families that went in are gone from the staged set; the ones that
      // were refused stay staged, so the author can retry them without
      // re-finding them.
      setStaged(failed.map((entry) => entry.family))
    } finally {
      // THE BATCH IS OVER, SO ITS COUNT STOPS BEING TRUE OF ANYTHING. Left
      // standing, "added 3 of 3" sat beside "1 family ready to embed" until the
      // next confirm — two numbers about two different moments, in one line.
      setProgress(undefined)
      setBusy(false)
    }
    if (failed.length === 0) onClose()
  }

  const chip = (label: string, active: boolean, name: string, press: () => void) =>
    <button key={label} type="button" className={`font-browser-chip${active ? ' font-browser-chip-active' : ''}`} aria-pressed={active} aria-label={name} disabled={busy} onClick={press}>{label}</button>

  const specimen = (row: BrowserRow, pixels: number) => {
    const status = statusOf(row.family)
    // A SPECIMEN IS NEVER SET IN A FALLBACK WHILE IMPLYING IT IS THE FAMILY.
    // Until the face is registered — and permanently, if its bytes cannot be
    // had — the row says which of those two it is, in words, instead of
    // rendering the sample in the panel's own typeface and letting the author
    // believe they are looking at the typeface they are about to embed.
    if (status !== 'ready' || previewFaceFamily(row.family) === undefined) {
      return <p className="font-browser-specimen-absent">{status === 'unavailable'
        ? `${row.family} cannot be shown set in itself: its face could not be fetched.`
        : `Fetching ${row.family} to set this specimen in it…`}</p>
    }
    return <p className="font-browser-specimen" lang={row.scripts.includes('thai') && thaiSample && previewText.trim() === '' ? 'th' : undefined} style={{ fontFamily: previewFaceFamily(row.family), fontSize: `${pixels}px` } as CSSProperties}>{specimenFor(row, previewText, thaiSample)}</p>
  }

  // THE ADD CONTROL, AND STORY 16.5 GAVE IT A SECOND INERT STATE. `in-template`
  // was already disabled because the action was meaningless there; `installed`
  // joins it for the same reason and not a weaker one — this dialog's action is
  // now INSTALL, and a face the store already holds has nothing to install. The
  // row still says which of the two it is, in the control's own accessible name,
  // because a disabled button with no explanation is the thing this screen's
  // contract forbids.
  const addButton = (row: BrowserRow) => {
    const state = rowState(row.family, inTemplate, staged, installedFamilies)
    return <button type="button" className={`font-browser-add font-browser-add-${state}`} aria-pressed={state === 'staged'} aria-label={buttonName(row.family, state)} disabled={busy || state === 'in-template' || state === 'installed'} onClick={() => toggleStaged(row.family)}>{buttonLabel(state)}</button>
  }

  // THE TWO VIEWS ARE AN EXHAUSTIVE SWITCH, NOT A TERNARY. As `view === 'Row' ?
  // … : …` a third view added later would silently render as Grid — the wrong
  // screen, drawn without complaint. This is `rowTierNote`'s idiom, applied to
  // the other closed set this file walks.
  const results = () => {
    switch (view) {
      case 'Row':
        return <ul className="font-browser-rows" aria-label="Font families">
          {shown.map((row) => <li key={row.family} className="font-browser-row">
            <div className="font-browser-row-head">
              <span className="font-browser-family">{row.family}</span>
              <span className="font-browser-meta">{row.category ?? 'category not stated'}</span>
              <span className="font-browser-meta">·</span>
              <span className="font-browser-meta">{rowTierNote(row.source)}</span>
              {badge(row)}
              <span className="font-browser-spacer" />
              {addButton(row)}
            </div>
            {specimen(row, specimenSize(view, size))}
          </li>)}
        </ul>
      case 'Grid':
        return <ul className="font-browser-grid" aria-label="Font families">
          {shown.map((row) => <li key={row.family} className="font-browser-card">
            <div className="font-browser-row-head">
              <span className="font-browser-family">{row.family}</span>
              <span className="font-browser-spacer" />
              {addButton(row)}
            </div>
            {specimen(row, specimenSize(view, size))}
            <div className="font-browser-row-foot">
              <span className="font-browser-meta">{row.category ?? 'category not stated'}</span>
              <span className="font-browser-meta">·</span>
              <span className="font-browser-meta">{rowTierNote(row.source)}</span>
              {badge(row)}
            </div>
          </li>)}
        </ul>
      default: {
        const unhandled: never = view
        throw new Error(`a results view nothing draws reached the font browser: ${String(unhandled as BrowserView)}`)
      }
    }
  }

  const badge = (row: BrowserRow) => <span className={`font-browser-badge${row.scripts.includes('thai') ? ' font-browser-badge-thai' : ''}`}>{scriptBadge(row)}</span>

  return <section ref={dialog} className="font-browser-backdrop" role="dialog" aria-modal="true" aria-label="Font browser" aria-busy={busy || undefined} onKeyDownCapture={trap}>
    <div className="font-browser">

      <div className="font-browser-header">
        <h2 className="font-browser-title">Font browser</h2>
        <div className="font-browser-search">
          <input ref={search} type="text" aria-label="Search fonts" placeholder="Search fonts" value={filters.query} disabled={busy} onChange={(event) => amend({ query: event.target.value })} />
          {filters.query !== '' && <button type="button" className="font-browser-inline-action" aria-label="Clear the search" disabled={busy} onClick={() => amend({ query: '' })}>×</button>}
        </div>
        <div className="font-browser-toggles" role="group" aria-label="Sort">
          <span className="font-browser-toggle-label">Sort</span>
          {browserSorts.map((option) => <button key={option} type="button" className={`font-browser-sort${sort === option ? ' font-browser-sort-active' : ''}`} aria-pressed={sort === option} aria-label={`Sort by ${option}`} disabled={busy} onClick={() => { setSort(option); setPage(0) }}>{option}</button>)}
        </div>
        <div className="font-browser-segmented" role="group" aria-label="Results view">
          {browserViews.map((option) => <button key={option} type="button" className={`font-browser-view${view === option ? ' font-browser-view-active' : ''}`} aria-pressed={view === option} aria-label={`${option} view`} disabled={busy} onClick={() => setView(option)}>{option}</button>)}
        </div>
        <button type="button" className="font-browser-inline-action" aria-label="Close font browser" disabled={busy} onClick={onClose}>×</button>
      </div>

      <div className="font-browser-body">
        <div className="font-browser-rail">
          <p className="section-label">PREVIEW</p>
          <div className="font-browser-rail-group">
            <input type="text" aria-label="Preview text" placeholder="Type something" value={previewText} disabled={busy} onChange={(event) => setPreviewText(event.target.value)} />
            <label className="font-browser-size">
              <span className="font-browser-size-value">{sizeReadout(view, size)}</span>
              <input type="range" aria-label="Preview size in pixels" min={minSpecimenSize} max={maxSpecimenSize} value={size} disabled={busy} onChange={(event) => setSize(Number(event.target.value))} />
            </label>
            <button type="button" className={`font-browser-switch${thaiSample ? ' font-browser-switch-on' : ''}`} aria-pressed={thaiSample} aria-label="Thai sample text" disabled={busy} onClick={() => setThaiSample((on) => !on)}>
              <span>Thai sample text</span><span className="font-browser-switch-dot" aria-hidden="true" />
            </button>
          </div>

          <p className="section-label">WRITING SYSTEM</p>
          <div className="font-browser-rail-group font-browser-chips" role="group" aria-label="Writing system">
            {chip('All', filters.script === undefined, 'All writing systems', () => amend({ script: undefined }))}
            {indexScripts.map((script) => chip(scriptLabel(script), filters.script === script, `${scriptLabel(script)} coverage`, () => amend({ script })))}
          </div>

          <p className="section-label">CATEGORY</p>
          <div className="font-browser-rail-group font-browser-chips" role="group" aria-label="Category">
            {indexCategories.map((category) => chip(category, filters.categories.includes(category), `${category} category`, () => toggleCategory(category)))}
          </div>
        </div>

        <div className="font-browser-results">
          <div className="font-browser-result-line">
            <span>{resultLine(shown.length, matching.length)}</span>
            {filtersActive(filters) && <button type="button" className="font-browser-link" disabled={busy} onClick={() => { setFilters(noFilters); setPage(0) }}>reset filters</button>}
            <span className="font-browser-spacer" />
            {matching.length > 0 && <>
              <span>{pageLine(page, matching.length)}</span>
              <button type="button" className="font-browser-link" aria-label="Previous page of families" disabled={busy || page === 0} onClick={() => setPage((current) => Math.max(0, current - 1))}>Previous</button>
              <button type="button" className="font-browser-link" aria-label="Next page of families" disabled={busy || page >= pages - 1} onClick={() => setPage((current) => Math.min(pages - 1, current + 1))}>Next</button>
            </>}
          </div>

          <div className="font-browser-scroll">
            {matching.length === 0
              ? <div className="font-browser-empty"><p className="font-browser-empty-heading">{emptyStateHeading(filters.query)}</p><p>{emptyStateHint}</p></div>
              : results()}
          </div>
        </div>
      </div>

      <div className="font-browser-footer">
        <span className={`font-browser-dot${staged.length > 0 ? ' font-browser-dot-active' : ''}`} aria-hidden="true" />
        <p className="font-browser-pending" role="status">{pendingLine(staged.length)}{progress === undefined ? '' : ` — added ${progress.done} of ${progress.total}`}</p>
        <span className="font-browser-weight">{weightLine(staged.length)}</span>
        {/* THE DEGRADED MODEL SAYS SO IN THE FOOTER FOR A SIGHTED READER, and in
            the confirm control's own accessible name for everybody else — an
            `aria-label` REPLACES the contents, so the visible label alone would
            be inaudible and the note alone would be unreachable. */}
        {!storeKeepsFaces && <span className="font-browser-degraded" role="status">{degradedFooterNote(storeKeepsFaces)}</span>}
        <span className="font-browser-spacer" />
        <button type="button" className="font-browser-cancel" disabled={busy} onClick={onClose}>Cancel</button>
        <button type="button" className="font-browser-confirm" aria-label={confirmName(staged.length, storeKeepsFaces)} disabled={busy || staged.length === 0} onClick={() => void confirm()}>{confirmLabel(staged.length)}</button>
      </div>
      {refusals.length > 0 && <ul className="font-browser-refusals" aria-label="Families that could not be added">
        {refusals.map((refusal) => <li key={refusal.family} role="alert" className="property-error">{refusal.family}: {refusal.message}</li>)}
      </ul>}
    </div>
  </section>
}

function scriptLabel(script: string): string {
  return script === 'cjk' ? 'CJK' : `${script.slice(0, 1).toUpperCase()}${script.slice(1)}`
}
