import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FontBrowser } from './FontBrowser'
import { familiesPerPage } from './font-browser-model'
import { addableFamilyCount, offeredFamilies, type FamilySource } from './font-index'
import type { StoredFace } from './font-store'
import { previewFaceFamily } from './preview-face-family'

// THE MODAL THE DESIGN DREW (Story 16.3), ASSERTED THROUGH THE NAMES A KEYBOARD
// AND A SCREEN READER REACH IT BY.
//
// Every control is addressed by its ACCESSIBLE NAME rather than by a class or a
// test id, because "keyboard and assistive technology are contract, not polish"
// (UX-DR25) is only a contract if the tests would notice a control losing its
// name.

function installStubFontSet(): Readonly<{ restore: () => void; live: () => ReadonlyArray<string> }> {
  class StubFace {
    readonly family: string
    constructor(family: string) { this.family = family }
    load(): Promise<StubFace> { return Promise.resolve(this) }
  }
  const live: StubFace[] = []
  const set = {
    add: (loaded: StubFace) => { live.push(loaded); return undefined },
    delete: (loaded: StubFace) => { const at = live.indexOf(loaded); if (at >= 0) live.splice(at, 1); return undefined },
  }
  Object.defineProperty(globalThis, 'FontFace', { value: StubFace, configurable: true, writable: true })
  Object.defineProperty(document, 'fonts', { value: set, configurable: true, writable: true })
  return {
    restore: () => { Reflect.deleteProperty(globalThis, 'FontFace'); Reflect.deleteProperty(document, 'fonts') },
    live: () => live.map((face) => face.family),
  }
}

let installed: Readonly<{ restore: () => void; live: () => ReadonlyArray<string> }> | undefined
afterEach(() => { installed?.restore(); installed = undefined })

const web = (family: string, category: string, scripts: ReadonlyArray<'latin' | 'thai'>, popularity: number): FamilySource =>
  ({ tier: 'web', family, row: { family, category, scripts, variable: false, popularity } })

const sources: ReadonlyArray<FamilySource> = [
  web('Sarabun', 'Sans Serif', ['latin', 'thai'], 1),
  web('Lora', 'Serif', ['latin'], 2),
  web('Chonburi', 'Display', ['latin', 'thai'], 3),
  web('Zilla Slab', 'Serif', ['latin'], 4),
]

const bytes = () => new Uint8Array([0, 1, 2, 3]).buffer

type Overrides = Partial<Parameters<typeof FontBrowser>[0]>

function open(overrides: Overrides = {}) {
  const onClose = vi.fn()
  const onAddFamily = vi.fn(async (source: FamilySource): Promise<string | undefined> => { void source; return undefined })
  const result = render(<FontBrowser sources={sources} inTemplate={[]} previewBytes={async () => bytes()} onAddFamily={onAddFamily} storeKeepsFaces onClose={onClose} {...overrides} />)
  return { ...result, onClose, onAddFamily }
}

describe('the font browser is the design\'s modal', () => {
  it('opens as a trapped dialog with the search focused and the five regions present', async () => {
    installed = installStubFontSet()
    open()
    const dialog = screen.getByRole('dialog', { name: 'Font browser' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Search fonts' })).toHaveFocus())
    // Header, rail (two chip groups plus the preview controls), results, footer.
    expect(screen.getByRole('group', { name: 'Sort' })).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'Results view' })).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'Writing system' })).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'Category' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Preview text' })).toBeInTheDocument()
    expect(screen.getByRole('slider', { name: 'Preview size in pixels' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Thai sample text' })).toBeInTheDocument()
    expect(screen.getByRole('list', { name: 'Font families' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install on this machine' })).toBeDisabled()
  })

  // THE HEADER SLOT IS EMPTY, AND BOTH CANDIDATE SENTENCES ARE REFUSED
  // (Story 16.10). The design's header is a single 46px row with no paragraph
  // under it, so this asserts the paragraph's ABSENCE rather than its wording.
  // The mockup's own `web font library · 1,946 families` stays refused on top of
  // that — following the design strictly here means drawing nothing in that
  // slot, not swapping one claim for a shorter one.
  //
  // THE LOAD-BEARING ASSERTION IS THE STRUCTURAL ONE, AND IT IS WORDING-FREE ON
  // PURPOSE. A phrase list only catches the sentences we already retired: mint a
  // NEW short one into the empty metadata slot — `1304 families`, say — and every
  // phrase assertion below still passes, which is precisely the defect the story's
  // "Ask First" and "Never" lists forbid. So the header's ELEMENT CHILDREN are
  // pinned as a CLOSED SET, by tag and class, in the design's own order. Any node
  // minted into that slot reds this test whatever it says and whatever tag it uses.
  // The phrase assertions are kept underneath as a second, weaker net that names
  // the specific claims this epic has ruled false.
  //
  // TWO POSITIVE CONTROLS, ONE PER QUERY MECHANISM. Every negative below is a
  // `getByText`, so a `getByText` control is what proves the negatives mean
  // something — a broken or renamed text query would otherwise report every
  // sentence gone and the guard would pass on nothing. The `getByRole` control
  // covers the accessible-name mechanism the rest of this file addresses controls
  // by, which is a different lookup and cannot vouch for the first.
  it('draws no disclosure paragraph in its header, and claims no live library either', () => {
    installed = installStubFontSet()
    open()
    const header = document.querySelector('.font-browser-header')
    expect(header, 'the header the design draws must exist before its contents can be pinned').not.toBeNull()
    expect(Array.from(header!.children).map((child) => `${child.tagName}.${child.className}`)).toEqual([
      'H2.font-browser-title',
      'DIV.font-browser-search',
      'DIV.font-browser-toggles',
      'DIV.font-browser-segmented',
      'BUTTON.font-browser-inline-action',
    ])
    expect(header!.querySelectorAll('p').length, 'the design draws no paragraph in the header').toBe(0)

    // POSITIVE CONTROLS — the role mechanism, then the text mechanism.
    expect(screen.getByRole('heading', { name: 'Font browser' })).toBeInTheDocument()
    expect(screen.getByText('Font browser')).toBeInTheDocument()

    // NOT `/already on this machine/`: `buttonName(family, 'installed')` returns
    // `${family} is already on this machine` as a row's accessible name, so
    // asserting that string's absence asserts the absence of something the browser
    // is SUPPOSED to print. It passed only because this fixture is four `web`-tier
    // sources; one `local` or `stored` source and it reds on correct behaviour.
    // The closed set above is what that assertion was reaching for. Do not re-add it.
    expect(screen.queryByText(/families you can add/)).toBeNull()
    expect(screen.queryByText(/The list itself is a snapshot taken on/)).toBeNull()
    expect(screen.queryByText(/variable file are not shown/)).toBeNull()
    expect(screen.queryByText(/web font library/)).toBeNull()

    // AND THE COUNT IS NOT LOST WITH THE SENTENCE. `resultLine` prints it in the
    // results toolbar from the same `addableFamilyCount` the paragraph quoted,
    // which is what keeps a count in the browser at all.
    expect(screen.getByText(new RegExp(`\\d+ of ${addableFamilyCount} families`))).toBeInTheDocument()
  })

  it('closes on Escape and on Cancel, discarding whatever was staged', async () => {
    installed = installStubFontSet()
    const { onClose, onAddFamily } = open()
    fireEvent.click(screen.getByRole('button', { name: 'Install Lora on this machine' }))
    expect(screen.getByRole('button', { name: 'Install 1 on this machine' })).toBeEnabled()
    fireEvent.keyDown(screen.getByRole('dialog', { name: 'Font browser' }), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalledTimes(2)
    // Nothing was sent: the staged set is UI state and closing discards it.
    expect(onAddFamily).not.toHaveBeenCalled()
  })

  it('keeps Tab inside the dialog in both directions', () => {
    installed = installStubFontSet()
    open()
    const dialog = screen.getByRole('dialog', { name: 'Font browser' })
    const focusable = Array.from(dialog.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled])'))
    expect(focusable.length).toBeGreaterThan(5)
    focusable.at(-1)?.focus()
    fireEvent.keyDown(dialog, { key: 'Tab' })
    expect(document.activeElement).toBe(focusable[0])
    focusable[0]?.focus()
    fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(focusable.at(-1))
  })
})

describe('a specimen is set in its own family, or says it is not', () => {
  it('sets the sample in the family the browser registered a face for', async () => {
    installed = installStubFontSet()
    open()
    await waitFor(() => expect(installed?.live()).toContain(previewFaceFamily('Sarabun')))
    // Two of the four families cover Thai, so two rows carry the Thai sample —
    // each set in ITS OWN face, which is the claim.
    const specimens = await screen.findAllByText('ทุกคนมีสิทธิในเสรีภาพแห่งความคิด')
    expect(specimens.map((node) => node.style.fontFamily)).toEqual([previewFaceFamily('Sarabun'), previewFaceFamily('Chonburi')])
    for (const specimen of specimens) expect(specimen.style.fontSize).toBe('34px')
  })

  it('never renders the sample in a fallback while implying it is the family', async () => {
    installed = installStubFontSet()
    open({ previewBytes: async (family: string) => family === 'Lora' ? undefined : bytes() })
    // Lora's face cannot be had. Its row says so, in words, and no element
    // anywhere carries Lora's sample text set in something that is not Lora.
    expect(await screen.findByText(/Lora cannot be shown set in itself/)).toBeInTheDocument()
    for (const node of screen.queryAllByText('Everyone has the right to freedom of thought')) {
      expect(node.style.fontFamily).not.toBe('')
    }
  })

  it('re-sets every specimen when the preview text and the size change', async () => {
    installed = installStubFontSet()
    open()
    await screen.findAllByText('ทุกคนมีสิทธิในเสรีภาพแห่งความคิด')
    fireEvent.change(screen.getByRole('textbox', { name: 'Preview text' }), { target: { value: 'ธนาคาร' } })
    fireEvent.change(screen.getByRole('slider', { name: 'Preview size in pixels' }), { target: { value: '48' } })
    const specimens = await screen.findAllByText('ธนาคาร')
    expect(specimens).toHaveLength(sources.length)
    for (const specimen of specimens) expect(specimen.style.fontSize).toBe('48px')
  })

  it('switches the default sample with the Thai toggle, and badges the coverage', async () => {
    installed = installStubFontSet()
    open()
    await screen.findAllByText('ทุกคนมีสิทธิในเสรีภาพแห่งความคิด')
    expect(screen.getAllByText('Thai + Latin')).toHaveLength(2)
    fireEvent.click(screen.getByRole('button', { name: 'Thai sample text' }))
    await waitFor(() => expect(screen.queryByText('ทุกคนมีสิทธิในเสรีภาพแห่งความคิด')).toBeNull())
    expect(screen.getAllByText('Everyone has the right to freedom of thought')).toHaveLength(sources.length)
  })

  it('releases every preview face when the modal unmounts', async () => {
    installed = installStubFontSet()
    const { unmount } = open()
    await waitFor(() => expect(installed?.live().length).toBe(sources.length))
    unmount()
    await waitFor(() => expect(installed?.live()).toEqual([]))
  })
})

describe('filters, sort, views and the empty state', () => {
  it('filters by query, intersects the chips, and follows with the result line', async () => {
    installed = installStubFontSet()
    open()
    expect(screen.getByText(`4 of ${addableFamilyCount} families`)).toBeInTheDocument()
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'sara' } })
    // The mockup's own shape: how many are shown, out of the whole population.
    expect(screen.getByText(`1 of ${addableFamilyCount} families`)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear the search' }))
    fireEvent.click(screen.getByRole('button', { name: 'Thai coverage' }))
    fireEvent.click(screen.getByRole('button', { name: 'Display category' }))
    const list = within(screen.getByRole('list', { name: 'Font families' }))
    expect(list.getByText('Chonburi')).toBeInTheDocument()
    expect(list.queryByText('Sarabun')).toBeNull()
  })

  it('shows `reset filters` exactly while a filter is active, and clears all three', () => {
    installed = installStubFontSet()
    open()
    expect(screen.queryByRole('button', { name: 'reset filters' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Serif category' }))
    fireEvent.click(screen.getByRole('button', { name: 'reset filters' }))
    expect(screen.queryByRole('button', { name: 'reset filters' })).toBeNull()
    expect(screen.getByText(`4 of ${addableFamilyCount} families`)).toBeInTheDocument()
  })

  it('offers Trending and A – Z, and no Most styles arm at all', () => {
    installed = installStubFontSet()
    open()
    const sort = within(screen.getByRole('group', { name: 'Sort' }))
    expect(sort.getByRole('button', { name: 'Sort by Trending' })).toHaveAttribute('aria-pressed', 'true')
    expect(sort.queryByRole('button', { name: /Most styles/ })).toBeNull()
    fireEvent.click(sort.getByRole('button', { name: 'Sort by A – Z' }))
    const names = within(screen.getByRole('list', { name: 'Font families' })).getAllByRole('listitem').map((item) => item.textContent ?? '')
    expect(names[0]).toContain('Chonburi')
  })

  it('draws the design\'s cards in Grid view over the same families', async () => {
    installed = installStubFontSet()
    open()
    fireEvent.click(within(screen.getByRole('group', { name: 'Results view' })).getByRole('button', { name: 'Grid view' }))
    const list = screen.getByRole('list', { name: 'Font families' })
    expect(list.className).toContain('font-browser-grid')
    expect(within(list).getAllByRole('listitem')).toHaveLength(sources.length)
    // The card specimen is capped at 26px however far the slider is pushed.
    fireEvent.change(screen.getByRole('slider', { name: 'Preview size in pixels' }), { target: { value: '56' } })
    const specimens = await screen.findAllByText('Everyone has the right to freedom of thought')
    for (const specimen of specimens) expect(specimen.style.fontSize).toBe('26px')
  })

  it('shows the design\'s empty state, naming the query', () => {
    installed = installStubFontSet()
    open()
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'nothing at all' } })
    expect(screen.getByText('No families match “nothing at all”')).toBeInTheDocument()
    expect(screen.getByText('Try a different spelling, or clear the category filters.')).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Font families' })).toBeNull()
  })

  it('pages the results rather than painting the whole snapshot', async () => {
    installed = installStubFontSet()
    open({ sources: offeredFamilies('') })
    const list = screen.getByRole('list', { name: 'Font families' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(familiesPerPage)
    expect(screen.getByRole('button', { name: 'Previous page of families' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Next page of families' }))
    expect(screen.getByText(/^Page 2 of /)).toBeInTheDocument()
    // AND THE REGISTRY FOLLOWS THE PAGE: never more faces on the page's font set
    // than there are rows on screen.
    await waitFor(() => expect(installed?.live().length).toBeLessThanOrEqual(familiesPerPage))
  })
})

describe('staging and confirming', () => {
  it('stages several families and dispatches one command each, in order', async () => {
    installed = installStubFontSet()
    const { onAddFamily, onClose } = open()
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Lora on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Chonburi on this machine' }))
    expect(screen.getByText(/^3 families ready to install/)).toBeInTheDocument()
    expect(screen.getByText(/3 faces · one upright Regular each/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Install 3 on this machine' }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(onAddFamily.mock.calls.map(([source]) => source.family)).toEqual(['Sarabun', 'Lora', 'Chonburi'])
  })

  it('un-stages a family that is pressed twice, and refuses to stage one already in the template', () => {
    installed = installStubFontSet()
    open({ inTemplate: ['Lora'] })
    expect(screen.getByRole('button', { name: 'Lora is in this template' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    expect(screen.getByText(/^1 family ready to install/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Remove Sarabun from the families to install' }))
    expect(screen.getByText(/^Select families to install on this machine/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install on this machine' })).toBeDisabled()
  })

  it('names one family\'s refusal, adds the rest, and leaves only the refused one staged', async () => {
    installed = installStubFontSet()
    const onAddFamily = vi.fn(async (source: FamilySource): Promise<string | undefined> => source.family === 'Lora' ? 'Lora could not be reached right now.' : undefined)
    const { onClose } = open({ onAddFamily })
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Lora on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Chonburi on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install 3 on this machine' }))

    // ONE FAMILY FAILING DOES NOT ABANDON THE OTHERS: all three were dispatched.
    await waitFor(() => expect(onAddFamily).toHaveBeenCalledTimes(3))
    expect(await screen.findByText(/Lora: Lora could not be reached right now\./)).toBeInTheDocument()
    // The modal stays open so the refusal can be read, and only Lora is left
    // staged so it can be retried without being found again.
    expect(onClose).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.getByText(/^1 family ready to install/)).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Remove Lora from the families to install' })).toBeInTheDocument()
  })

  it('turns a seam that REJECTS into a named refusal, and never leaves the modal stuck busy', async () => {
    installed = installStubFontSet()
    const onAddFamily = vi.fn(async (source: FamilySource): Promise<string | undefined> => {
      if (source.family === 'Lora') throw new Error('the worker went away')
      return undefined
    })
    const { onClose } = open({ onAddFamily })
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Lora on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install 2 on this machine' }))
    // The throw is the seam failing in a way it did not anticipate. Before this
    // was caught, `setBusy(false)` never ran: every control stayed disabled for
    // the life of the modal with Escape the only way out.
    expect(await screen.findByText(/Lora could not be added: the worker went away/)).toBeInTheDocument()
    expect(onAddFamily).toHaveBeenCalledTimes(2)
    expect(onClose).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled())
    expect(screen.getByRole('textbox', { name: 'Search fonts' })).toBeEnabled()
  })

  it('clears the batch count when the batch ends, so two moments are not reported as one line', async () => {
    installed = installStubFontSet()
    const onAddFamily = vi.fn(async (source: FamilySource): Promise<string | undefined> => source.family === 'Lora' ? 'Lora could not be reached.' : undefined)
    open({ onAddFamily })
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Lora on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install 2 on this machine' }))
    await screen.findByText(/Lora could not be reached\./)
    // ASSERTED AS AN ABSENCE, because the footer's own line is prefix-anchored
    // elsewhere and a stale suffix slips straight past a prefix match.
    await waitFor(() => expect(screen.queryByText(/added 2 of 2/)).toBeNull())
    expect(screen.getByText('1 family ready to install')).toBeInTheDocument()
  })

  it('does not let Escape dismiss the modal while a batch is running', async () => {
    installed = installStubFontSet()
    let release: (() => void) | undefined
    const onAddFamily = vi.fn(async (source: FamilySource): Promise<string | undefined> => { void source; await new Promise<void>((resolve) => { release = resolve }); return undefined })
    const { onClose } = open({ onAddFamily })
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install 1 on this machine' }))
    await screen.findByText(/added 0 of 1/)
    // Every other control is disabled here. Escape was the one route to a
    // half-dismissed modal with dispatches still in flight.
    fireEvent.keyDown(screen.getByRole('dialog', { name: 'Font browser' }), { key: 'Escape' })
    expect(onClose).not.toHaveBeenCalled()
    release?.()
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
  })

  it('states progress while the batch runs', async () => {
    installed = installStubFontSet()
    let release: (() => void) | undefined
    const onAddFamily = vi.fn(async (source: FamilySource): Promise<string | undefined> => { void source; await new Promise<void>((resolve) => { release = resolve }); return undefined })
    open({ onAddFamily })
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install Lora on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: 'Install 2 on this machine' }))
    expect(await screen.findByText(/added 0 of 2/)).toBeInTheDocument()
    release?.()
    expect(await screen.findByText(/added 1 of 2/)).toBeInTheDocument()
    release?.()
  })
})

// THE MATRIX'S `Offline` ROW, WHICH HAD NO COMMITTED TEST.
//
// The browser run exercised it, but that witness was a temporary spec that was
// deleted afterwards — so nothing re-runnable covered this row, which is the
// same state as a test that exists and never runs. Both halves of the row are
// asserted here: the web tier SAYS SO in words, and the machine store keeps
// working while it does.
//
// The second half is the one worth having. `browserSpecimenBytes` resolves a
// stored family out of IndexedDB and a web family through `fetchWebFamily`, so a
// face this machine already holds needs no network at all — and the whole point
// of Story 16.2 is that this stays true when the network is gone.
// STORY 16.5, LEAD GUARDRAIL — IN DEGRADED MODE THE CONFIRM CONTROL NAMES THE
// COUNT, BECAUSE IT NO LONGER DOES WHAT ITS LABEL SAYS.
//
// A browser that cannot keep typefaces gets the pre-16.5 model: a pick embeds.
// In this dialog that means confirming five staged families writes FIVE FACES
// into the document — the exact outcome the owner's reversal was made to
// prevent. It is acceptable as a stated degradation of a rare path and
// unacceptable unstated, so the number travels in the name of the control that
// does it, at the moment the author is deciding whether to press it.
describe('a browser that cannot keep typefaces says what confirming will do instead', () => {
  it('names the count in the confirm control and says so in the footer, and stops when the store works', () => {
    installed = installStubFontSet()
    const { rerender } = open({ storeKeepsFaces: false })

    // WITH NOTHING STAGED THE WARNING IS ALREADY THERE, because the author is
    // deciding whether to stage at all, not only whether to confirm.
    expect(screen.getByRole('button', { name: 'Install on this machine — this browser will not keep fonts, so confirming adds families straight to this document' })).toBeDisabled()
    expect(screen.getByText(/This browser will not keep typefaces, so these go straight into the document/)).toBeInTheDocument()

    // AND THE COUNT IS THE STAGED COUNT, not a fixed sentence. A warning that
    // did not move with the number would be one an author skims.
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    expect(screen.getByRole('button', { name: 'Install 1 on this machine — this browser will not keep fonts, so confirming adds 1 family to this document' })).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Install Lora on this machine' }))
    const confirm = screen.getByRole('button', { name: 'Install 2 on this machine — this browser will not keep fonts, so confirming adds 2 families to this document' })
    expect(confirm).toBeEnabled()
    // THE VISIBLE LABEL IS STILL THE DESIGN'S, AND THE ACCESSIBLE NAME CONTAINS
    // IT (WCAG 2.5.3, Label in Name). A name sharing no words with the button's
    // own text is unspeakable — a speech-input author says what they can see —
    // so the label leads and the warning follows it. The footer note is the half
    // that reaches a sighted reader.
    expect(confirm.textContent).toBe('Install 2 on this machine')
    expect(confirm.getAttribute('aria-label')).toContain(confirm.textContent)

    // THE OVER-BROADNESS CONTROL: with a working store none of it appears. A
    // dialog that warned unconditionally would pass every line above.
    rerender(<FontBrowser sources={sources} inTemplate={[]} previewBytes={async () => bytes()} onAddFamily={vi.fn(async () => undefined)} storeKeepsFaces onClose={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Install 2 on this machine' })).toBeInTheDocument()
    expect(screen.queryByText(/go straight into the document/)).toBeNull()
    expect(screen.queryByRole('button', { name: /will not keep fonts/ })).toBeNull()
  })
})

describe('with no network the browser says so, and the faces this machine holds still work', () => {
  const stored: FamilySource = {
    tier: 'stored',
    family: 'Kanit',
    record: {
      key: 'b'.repeat(64), family: 'Kanit', style: 'Regular', licence: 'OFL-1.1', licenceText: 'terms',
      copyright: 'c', source: 'google/fonts — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03',
      mediaType: 'font/ttf', scripts: ['latin', 'thai'], fetchedAt: '2026-09-03', byteLength: 4,
    } satisfies StoredFace,
  }

  it('states the web rows it cannot fetch, and still sets the stored family in itself', async () => {
    installed = installStubFontSet()
    // Offline: every web family fails to resolve; the stored one is on this
    // machine and resolves without a network.
    const offlineBytes = async (family: string) => family === 'Kanit' ? bytes() : undefined
    render(<FontBrowser sources={[stored, ...sources]} inTemplate={[]} previewBytes={offlineBytes} onAddFamily={vi.fn(async () => undefined)} storeKeepsFaces onClose={vi.fn()} />)

    // THE STORED FAMILY IS SET IN ITSELF, and that is the half that proves the
    // store is read rather than merely present.
    await waitFor(() => expect(installed?.live()).toContain(previewFaceFamily('Kanit')))
    // AND THE WEB ROWS SAY WHY THEY ARE NOT, in words, rather than rendering the
    // sample in the panel's own typeface.
    await waitFor(() => expect(screen.getByText(/Sarabun cannot be shown set in itself/)).toBeTruthy())
    // NO WEB FAMILY MAY HAVE REACHED THE FONT SET.
    for (const family of ['Sarabun', 'Lora', 'Chonburi', 'Zilla Slab']) {
      expect(installed?.live()).not.toContain(previewFaceFamily(family))
    }
  })

  // BEHAVIOUR-CHANGED (Story 16.5), AND THE CHANGE IS THE POINT OF THE STORY.
  //
  // This used to stage the stored family and confirm it, because confirming
  // EMBEDDED and a stored family had a document to be embedded into. The dialog's
  // action is now INSTALL, and a face the store already holds has nothing to
  // install — so the row reports that it is here and is not stageable, while the
  // web rows beside it, on the same screen with the same dead network, still are.
  //
  // THE CLAIM IT STILL CARRIES IS THE ONE WORTH HAVING: the store is READ. A
  // browser that ignored `familyIsInstalled` would show `+ Install` over a face
  // this machine holds and would dispatch a pointless fetch for it, and both
  // halves of that are asserted below. Embedding a stored family is now first
  // use, and it is covered where it happens — `App.test.tsx`'s third-arm case.
  it('reports the stored family as already here rather than offering to install it, while web rows stay stageable', async () => {
    installed = installStubFontSet()
    const onAddFamily = vi.fn(async (source: FamilySource): Promise<string | undefined> => { void source; return undefined })
    render(<FontBrowser sources={[stored, ...sources]} inTemplate={[]} previewBytes={async (family) => family === 'Kanit' ? bytes() : undefined} onAddFamily={onAddFamily} storeKeepsFaces onClose={vi.fn()} />)

    const kanit = screen.getByLabelText(/Kanit/i, { selector: 'button.font-browser-add' })
    expect(kanit).toHaveAccessibleName('Kanit is already on this machine')
    expect(kanit, 'installing a face the store already holds would fetch bytes it already has').toBeDisabled()
    fireEvent.click(kanit)
    expect(screen.getByText('Select families to install on this machine')).toBeInTheDocument()

    // THE OVER-BROADNESS CONTROL, on the same screen and the same dead network:
    // a filter that reported every row as installed would pass everything above.
    fireEvent.click(screen.getByRole('button', { name: 'Install Sarabun on this machine' }))
    fireEvent.click(screen.getByRole('button', { name: /Install 1 on this machine/i }))
    await waitFor(() => expect(onAddFamily).toHaveBeenCalledTimes(1))
    expect(onAddFamily.mock.calls[0][0]).toMatchObject({ tier: 'web', family: 'Sarabun' })
  })
})
