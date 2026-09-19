import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { PreviewEvidenceRail, type EvidenceRailProps } from './evidence-rail'
import { BYTE_IDENTITY_SENTENCE, RENDER_TARGET, formatElapsed, formatRenderSize, hashLines, placementFor, diagnosticLocationText } from './evidence-rail-facts'
import { PDF_FIXTURE_DIGEST } from '../test/pdf-fixture'

// STORY 13.3 — THE EVIDENCE RAIL.
//
// ⚠ EVERY FIXTURE HERE SUPPLIES WHAT THE REAL CALLER SUPPLIES. Story 13.2's
// DW-191 guard was rigorous, mutation-proof, and about a viewer the application
// never renders, because its props were built once while `App` rebuilds them
// every render — the mutation and the assertion agreed with each other in a
// world the app never enters. So the digest below is a real 64-character
// digest of real fixture bytes, and the integration rows that matter live in
// `App.test.tsx` against the actual `App`.
const props = (over: Partial<EvidenceRailProps> = {}): EvidenceRailProps => ({
  render: { engineVersion: '0.0.0-dev', target: RENDER_TARGET, pages: 34, elapsedMs: 412, sizeBytes: 253_952 },
  hash: { digest: PDF_FIXTURE_DIGEST, standIn: false },
  stale: false,
  warnings: 0,
  warningsOnScreen: 0,
  errors: 0,
  onRerender: vi.fn(),
  exportLabel: 'Save PDF',
  exportDisabled: false,
  exportUnavailable: undefined,
  onExport: vi.fn(),
  ...over,
})

describe('the preview evidence rail', () => {
  it('carries exactly the five render values, and never a row count', () => {
    render(<PreviewEvidenceRail {...props()} />)
    const facts = screen.getByLabelText('Render facts')
    for (const [term, value] of [['engine', '0.0.0-dev'], ['target', 'wasm · in browser'], ['pages', '34'], ['elapsed', '412 ms'], ['size', '248 KB']]) {
      expect(facts, `${term} is a fact of the render that happened`).toHaveTextContent(new RegExp(`${term}\\s*${value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`))
    }
    // FIVE, NOT THE MOCKUP'S SIX. `rows` was ruled out by the owner: it exists
    // only inside the renderer, surfacing it would change `folio8.Render`'s
    // exported contract, and one scalar cannot distinguish no tables from a
    // table bound to an empty collection.
    expect(facts.querySelectorAll('.rail-fact')).toHaveLength(5)
    // Read off the TERMS, not off the block's text: `wasm · in browser` carries
    // the letters "rows" in the middle of a word, and a whole-block scan would
    // fail on a value that is correct.
    expect(Array.from(facts.querySelectorAll('dt')).map((term) => term.textContent)).toEqual(['engine', 'target', 'pages', 'elapsed', 'size'])
    // THE VERSION IS PRINTED AS THE CONSTANT READS. `folio-go v0.1` is the
    // mockup's invention and no git tag names a release.
    expect(facts).not.toHaveTextContent('folio-go v0.1')
  })

  it('withholds the page count until the viewer has one, rather than guessing', () => {
    render(<PreviewEvidenceRail {...props({ render: { engineVersion: '0.0.0-dev', target: RENDER_TARGET, pages: undefined, elapsedMs: 0, sizeBytes: 900 } })} />)
    const facts = screen.getByLabelText('Render facts')
    expect(facts.querySelectorAll('.rail-fact')).toHaveLength(4)
    expect(facts).not.toHaveTextContent(/pages/)
    // AND A ZERO ELAPSED IS DISPLAYED, NOT DROPPED. A render that finished
    // inside a millisecond is a fact; a blank there would be a silent lie.
    expect(facts).toHaveTextContent(/elapsed\s*0 ms/)
  })

  it('shows the whole digest, in mono, in its own bordered block, across two lines', () => {
    render(<PreviewEvidenceRail {...props()} />)
    const block = screen.getByLabelText('Output hash')
    const value = block.querySelector('.rail-hash-value')!
    // ALL 64 CHARACTERS. The mockup's 32-character excerpt is a mockup
    // artifact: a hash to be compared by eye must be the whole hash.
    expect(value.textContent).toBe(PDF_FIXTURE_DIGEST)
    expect(PDF_FIXTURE_DIGEST).toMatch(/^[a-f0-9]{64}$/)
    const lines = Array.from(value.querySelectorAll('.rail-hash-line')).map((line) => line.textContent)
    expect(lines).toEqual([PDF_FIXTURE_DIGEST.slice(0, 32), PDF_FIXTURE_DIGEST.slice(32)])
    expect(block).toHaveTextContent('Historical producer digest')
  })

  it('states the cross-target claim the build actually earns, and affirms no live comparison', () => {
    render(<PreviewEvidenceRail {...props()} />)
    const block = screen.getByLabelText('Output hash')
    expect(block).toHaveTextContent(BYTE_IDENTITY_SENTENCE)
    expect(BYTE_IDENTITY_SENTENCE).toBe('Byte-identical across darwin/arm64, linux/amd64, linux/arm64 and js/wasm — proven by the build\'s cross-target matrix, not compared here.')
    // The mockup's check-circle and "Matches native render" claimed a
    // comparison that happens nowhere in the browser.
    expect(block).not.toHaveTextContent(/Matches native render/)
  })

  it('withholds the byte-identity sentence entirely for a stand-in digest', () => {
    const { rerender } = render(<PreviewEvidenceRail {...props({ hash: { digest: PDF_FIXTURE_DIGEST, standIn: true } })} />)
    const standIn = screen.getByLabelText('Output hash')
    expect(standIn).toHaveTextContent('Stand-in local digest')
    expect(standIn).not.toHaveTextContent('Byte-identical across')
    expect(standIn.textContent).toContain(PDF_FIXTURE_DIGEST.slice(0, 32))
    // THE MUTATION, IN THE TEST ITSELF: flipping the one flag must change
    // whether the sentence is there. A withholding that does not move with its
    // condition is not a withholding.
    rerender(<PreviewEvidenceRail {...props({ hash: { digest: PDF_FIXTURE_DIGEST, standIn: false } })} />)
    const production = screen.getByLabelText('Output hash')
    expect(production).toHaveTextContent('Byte-identical across')
    expect(production).not.toHaveTextContent('Stand-in local digest')
  })

  it('marks a stale rail as describing the earlier render, never the current document', () => {
    render(<PreviewEvidenceRail {...props({ stale: true })} />)
    expect(screen.getByLabelText('Render facts')).toHaveTextContent('These values describe the earlier render, not the current document.')
  })

  it('reads as checked rather than empty when the render reported zero diagnostics', () => {
    render(<PreviewEvidenceRail {...props()} />)
    const block = screen.getByLabelText('Diagnostics summary')
    expect(block).toHaveTextContent('warnings 0')
    expect(block).toHaveTextContent('errors 0')
    expect(block).toHaveTextContent('The render completed and reported zero diagnostics.')
    // BOTH LEGEND ROWS, ALWAYS. They are the same partition the two counts are,
    // named twice: dashed triangle = the diagnostics array, solid square = the
    // failure card.
    expect(block).toHaveTextContent('Triangle, dashed — render proceeded')
    expect(block).toHaveTextContent('Square, solid — render failed')
  })

  it('counts a failed render as one error, and says nothing about zero when one is present', () => {
    render(<PreviewEvidenceRail {...props({ warnings: 2, warningsOnScreen: 2, errors: 1 })} />)
    const block = screen.getByLabelText('Diagnostics summary')
    expect(block).toHaveTextContent('warnings 2')
    expect(block).toHaveTextContent('errors 1')
    expect(block).not.toHaveTextContent('zero diagnostics')
  })

  // REVIEW P1 — THE ZERO STATE IS AN AFFIRMATION, AND IT IS ONLY EVER ABOUT A
  // RENDER THAT COMPLETED AND IS THE ONE ON SCREEN.
  //
  // A stale rail carries the earlier render's counts, which for a clean
  // document are 0 and 0 — so a section gated on the counts alone says "The
  // render completed and reported zero diagnostics." about whatever happened
  // most recently, including a render that was refused outright.
  it('never affirms a clean render while the counts describe an earlier one', () => {
    render(<PreviewEvidenceRail {...props({ stale: true })} />)
    const block = screen.getByLabelText('Diagnostics summary')
    expect(block).not.toHaveTextContent('The render completed and reported zero diagnostics.')
    expect(block).toHaveTextContent('These counts describe the earlier render; its cards are not shown.')
    // The legend still stands: it explains the two shapes, not a claim about
    // this render.
    expect(block).toHaveTextContent('Triangle, dashed — render proceeded')
  })

  // REVIEW P2 — A COUNT THE AUTHOR CANNOT POINT AT ANYTHING FOR SAYS SO.
  it('names the gap between the warnings a render reported and the cards on screen', () => {
    const { rerender } = render(<PreviewEvidenceRail {...props({ warnings: 2, warningsOnScreen: 2 })} />)
    expect(screen.getByLabelText('Diagnostics summary')).not.toHaveTextContent('dismissed on this screen')
    rerender(<PreviewEvidenceRail {...props({ warnings: 2, warningsOnScreen: 0 })} />)
    const block = screen.getByLabelText('Diagnostics summary')
    expect(block).toHaveTextContent('warnings 2')
    expect(block).toHaveTextContent('2 dismissed on this screen.')
    // And a stale rail says the cards are withheld rather than dismissed: the
    // two absences have different causes and are not interchangeable.
    rerender(<PreviewEvidenceRail {...props({ warnings: 2, warningsOnScreen: 0, stale: true })} />)
    expect(screen.getByLabelText('Diagnostics summary')).toHaveTextContent('These counts describe the earlier render; its cards are not shown.')
    expect(screen.getByLabelText('Diagnostics summary')).not.toHaveTextContent('dismissed on this screen')
  })

  it('states its own absence before anything has been rendered', () => {
    render(<PreviewEvidenceRail {...props({ render: undefined, hash: undefined })} />)
    expect(screen.getByLabelText('Render facts')).toHaveTextContent('No local render has produced a document yet.')
    expect(screen.getByLabelText('Output hash')).toHaveTextContent('Go production digest pending')
    // With nothing rendered there is no "checked" claim to make either.
    expect(screen.getByLabelText('Diagnostics summary')).not.toHaveTextContent('zero diagnostics')
  })

  it('pairs the two actions at the foot, keyboard-operable, with the reason bound to its own button', () => {
    const onRerender = vi.fn()
    const onExport = vi.fn()
    const { rerender } = render(<PreviewEvidenceRail {...props({ onRerender, onExport })} />)
    const rerenderButton = screen.getByRole('button', { name: 'Re-render' })
    const save = screen.getByRole('button', { name: 'Save PDF' })
    rerenderButton.focus()
    expect(document.activeElement).toBe(rerenderButton)
    fireEvent.click(rerenderButton)
    expect(onRerender).toHaveBeenCalledOnce()
    save.focus()
    expect(document.activeElement).toBe(save)
    fireEvent.click(save)
    expect(onExport).toHaveBeenCalledOnce()
    expect(save).not.toHaveAttribute('aria-describedby')

    // THE REASON TRAVELS WITH THE BUTTON. `aria-describedby` names it by id, so
    // a reason rendered anywhere else is an association that silently is not one.
    rerender(<PreviewEvidenceRail {...props({ exportDisabled: true, exportLabel: 'Save stale no-data PDF', exportUnavailable: 'Save stale no-data PDF is unavailable: no local PDF has been rendered yet.' })} />)
    const disabled = screen.getByRole('button', { name: 'Save stale no-data PDF' })
    expect(disabled).toBeDisabled()
    expect(disabled).toHaveAttribute('aria-describedby', 'preview-pdf-export-reason')
    const reason = document.getElementById('preview-pdf-export-reason')!
    expect(reason).toHaveTextContent('Save stale no-data PDF is unavailable')
    // REVIEW P8 — ASSERTED BY ROLE, not by the bare attribute: a label on a
    // generic `<div>` announces to nothing, and `getByLabelText` alone cannot
    // tell the difference.
    expect(screen.getByRole('group', { name: 'Render actions' }).contains(reason)).toBe(true)
  })
})

describe('the evidence rail\'s facts', () => {
  it('formats sizes and elapsed times without inventing precision, and keeps zero a value', () => {
    expect(formatRenderSize(0)).toBe('0 B')
    expect(formatRenderSize(1023)).toBe('1023 B')
    expect(formatRenderSize(1024)).toBe('1 KB')
    expect(formatRenderSize(253_952)).toBe('248 KB')
    expect(formatRenderSize(1_600_000)).toBe('1.5 MB')
    // REVIEW P11 — THE UNIT IS CHOSEN AFTER ROUNDING, NOT BEFORE. Just under a
    // mebibyte the old order printed `1024 KB`, which is a unit that has
    // already been superseded by the number in front of it.
    expect(formatRenderSize(1024 * 1024 - 1)).toBe('1 MB')
    expect(formatRenderSize(1024 * 1024)).toBe('1 MB')
    expect(formatRenderSize(1023 * 1024)).toBe('1023 KB')
    expect(formatElapsed(0)).toBe('0 ms')
    expect(formatElapsed(412)).toBe('412 ms')
    expect(formatElapsed(999)).toBe('999 ms')
    expect(formatElapsed(1000)).toBe('1 s')
    expect(formatElapsed(1450)).toBe('1.5 s')
  })

  it('splits a digest at 32 characters and refuses to cut anything that is not one', () => {
    expect(hashLines(PDF_FIXTURE_DIGEST)).toEqual([PDF_FIXTURE_DIGEST.slice(0, 32), PDF_FIXTURE_DIGEST.slice(32)])
    expect(hashLines(PDF_FIXTURE_DIGEST).join('')).toBe(PDF_FIXTURE_DIGEST)
    expect(hashLines('short')).toEqual(['short'])
  })

  it('joins kind and band only from a projection that carries the element', () => {
    const components = [{ id: 'e7', type: 'table', band: 'content' }]
    expect(placementFor('e7', components)).toEqual({ kind: 'table', band: 'content' })
    expect(placementFor('e9', components)).toBeUndefined()
    expect(placementFor('e7', undefined)).toBeUndefined()
    expect(placementFor('', components)).toBeUndefined()
    expect(diagnosticLocationText('transactions[11].description', { kind: 'table', band: 'content' })).toBe('transactions[11].description · table · band content')
    // A STRUCTURAL PATH IS RENDERED AS THE ENGINE SENT IT. `dataPath` is not
    // guaranteed to be a data binding — `bands.content.e7` is one of the
    // presenter's own fixtures — so nothing here claims it is one.
    expect(diagnosticLocationText('bands.content.e7', undefined)).toBe('bands.content.e7')
    expect(diagnosticLocationText('', undefined)).toBeUndefined()
  })
})
