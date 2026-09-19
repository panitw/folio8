import { useEffect, useRef, type KeyboardEvent } from 'react'
import { BLANK_CHOICE_ID } from './startup-examples'
import { ToolIcon } from './toolbar-icons'

// THE STARTUP DIALOG — `ux-startup-templates-2026-09-15/mockups/Main.dc.html`
// (spec-startup-templates, CAP-1, CAP-3, CAP-8; stories 3 and 4).
//
// PRESENTATIONAL. App owns which card is selected, whether an example is being
// opened, what went wrong and what Cancel means; this file draws them and moves
// a keyboard through them. The focus trap is `FontBrowser`'s shape,
// deliberately — this designer has one way of trapping a dialog.
//
// CANCEL AND ESCAPE ARE `onCancel`. App decides what closing means: at launch
// the untouched starter already is Blank; after New… the open document stays.
// No choice here asks about unsaved changes: New… asked before this opened.
export type StartupCard = Readonly<{
  id: string
  name: string
  description: string
  /** The example's sample JSON file name; `undefined` for Blank. */
  sample?: string
  /** The engine-rendered first-page thumbnail URL; `undefined` for Blank. */
  thumbnail?: string
}>

type Props = Readonly<{
  cards: ReadonlyArray<StartupCard>
  selected: string
  /** The name of the example being opened, while it is. Every action waits. */
  busy?: string
  error?: string
  onSelect: (id: string) => void
  onConfirm: (id: string) => void
  onCancel: () => void
  /** Present only when the browser exposes local file access. */
  onOpenFile?: () => void
}>

export function StartupDialog({ cards, selected, busy, error, onSelect, onConfirm, onCancel, onOpenFile }: Props) {
  const dialog = useRef<HTMLElement>(null)
  const initial = useRef<HTMLButtonElement>(null)
  const current = cards.find((card) => card.id === selected) ?? cards[0]
  const blank = current?.id === BLANK_CHOICE_ID
  const busyNow = busy !== undefined

  useEffect(() => { initial.current?.focus() }, [])

  const trap = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Escape') { event.preventDefault(); if (!busyNow) onCancel(); return }
    if (event.key !== 'Tab') return
    const focusable = Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not([disabled])') ?? []).filter((element) => element.tabIndex >= 0)
    if (focusable.length === 0) return
    const index = focusable.indexOf(document.activeElement as HTMLElement)
    // Focus on the dialog section itself (after a click on its background) is
    // pulled back into the cycle rather than let out of it.
    if (index === -1) { event.preventDefault(); focusable[event.shiftKey ? focusable.length - 1 : 0]?.focus(); return }
    if ((!event.shiftKey && index === focusable.length - 1) || (event.shiftKey && index === 0)) {
      event.preventDefault()
      focusable[event.shiftKey ? focusable.length - 1 : 0]?.focus()
    }
  }

  // BUSY IS `aria-disabled`, NOT `disabled`. A disabled button drops focus to
  // the body, and a focus trap with nothing focused inside it traps nothing.
  const confirm = (id: string) => { if (!busyNow) onConfirm(id) }
  const select = (id: string) => { if (!busyNow) onSelect(id) }

  // `tabIndex={-1}`: a click on a non-focusable part of the dialog focuses the
  // dialog itself rather than the body, so Escape and the trap keep working.
  return <section ref={dialog} tabIndex={-1} className="startup-backdrop" role="dialog" aria-modal="true" aria-labelledby="startup-title" aria-busy={busyNow || undefined} onKeyDownCapture={trap}>
    <div className="startup-sheet">
      <div className="startup-header">
        <h2 id="startup-title" className="startup-title">New template</h2>
        <span className="startup-hint">Start empty, or open an example with sample data</span>
      </div>

      <div className="startup-body">
        <div className="startup-sections" aria-hidden="true">
          <span className="startup-section-label">START</span>
          <span className="startup-section-rule" />
          <span className="startup-section-label">EXAMPLES</span>
          <span className="startup-section-note">fictional data · opens in Preview</span>
        </div>
        <div className="startup-cards" role="group" aria-label="Start from">
          {cards.map((card) => {
            const active = card.id === current?.id
            return <button
              key={card.id}
              ref={active ? initial : undefined}
              type="button"
              className={`startup-card${active ? ' startup-card-active' : ''}`}
              aria-pressed={active}
              aria-label={card.name}
              aria-describedby={`startup-card-${card.id}-description startup-card-${card.id}-sample`}
              aria-disabled={busyNow || undefined}
              onClick={() => select(card.id)}
              onDoubleClick={() => confirm(card.id)}
              onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); select(card.id); confirm(card.id) } }}
            >
              <span className="startup-thumbnail-shell">
                {card.thumbnail === undefined
                  ? <span className="startup-thumbnail startup-thumbnail-blank" data-testid="startup-blank-page" />
                  : <img className="startup-thumbnail" src={card.thumbnail} alt="" draggable={false} />}
              </span>
              <span className="startup-card-text">
                <span className="startup-card-name">{card.name}</span>
                <span id={`startup-card-${card.id}-description`} className="startup-card-description">{card.description}</span>
                <span id={`startup-card-${card.id}-sample`} className="startup-card-sample">
                  {card.sample === undefined
                    ? <span className="startup-sample-none">no sample data</span>
                    : <><span className="startup-sample-dot" aria-hidden="true" /><span className="startup-sample-name">{card.sample}</span></>}
                </span>
              </span>
            </button>
          })}
        </div>
      </div>

      <div className="startup-footer">
        {onOpenFile !== undefined && <>
          <button type="button" className="startup-open-file" aria-disabled={busyNow || undefined} onClick={() => { if (!busyNow) onOpenFile() }}><ToolIcon glyph="open" />Open existing file…</button>
          <span className="startup-footer-rule" aria-hidden="true" />
        </>}
        {error !== undefined
          ? <p className="startup-error" role="alert">{error}</p>
          : <p className="startup-outcome" role="status">
            <svg aria-hidden="true" className="startup-outcome-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.25" strokeLinecap="square"><path d="M3.5 2.5h9v11h-9z" />{!blank && <path d="M5.5 5.5h5M5.5 8h5M5.5 10.5h3" />}</svg>
            {busyNow
              ? <span>Opening {busy}…</span>
              : blank || current === undefined
                ? <span>Blank starts an empty A4 page with no sample data</span>
                : <><span>{current.name} opens in Preview with</span>{' '}<span className="startup-sample-name">{current.sample}</span></>}
          </p>}
        {/* Cancel and the primary action wrap as one group, right-aligned. */}
        <span className="startup-actions">
          <button type="button" className="startup-cancel" aria-disabled={busyNow || undefined} onClick={() => { if (!busyNow) onCancel() }}>Cancel</button>
          <button type="button" className="startup-confirm" aria-disabled={busyNow || undefined} onClick={() => current && confirm(current.id)}>{blank ? 'Start blank' : 'Open example'}</button>
        </span>
      </div>
    </div>
  </section>
}
