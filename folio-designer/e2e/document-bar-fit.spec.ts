import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// EPIC 14 BOUNDARY GATE — THE INSTRUMENTS FOR DW-332 AND DW-339.
//
// Both entries are LAYOUT claims, and both were registered as "unverified by
// any run": DW-332's document-bar fit at the shell's 1024px minimum rested on
// arithmetic done over a measurement taken in ANOTHER story (13.5's 179px of
// slack), and DW-339's `OrientationProperty` width rested on a read of the
// stylesheet. jsdom performs no layout, so neither can be answered anywhere in
// the unit suite; a real browser at a fixed viewport is the only witness.
//
// THIS IS A MEASURING INSTRUMENT, NOT A REPAIR. It changes no product code.
//
// ⚠ NOT ONE PROHIBITED IDENTIFIER IS SPELLED IN THIS FILE. Every `e2e/` file is
// auto-enrolled by the directory walk at `canvas-authority-contract.test.ts:11`
// and the scan is plain regex over the WHOLE file text, `page.evaluate` bodies
// included. So the whole witness is built from Playwright's own `boundingBox()`
// and `viewportSize()` — the idiom `e2e/preview-navigation.spec.ts:307` already
// uses green for the preview STATUS bar — and never from
// `getBoundingClientRect`, `offset*`, `client*`, `scroll*`, `getComputedStyle`
// or `ResizeObserver`.
const template = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/testdata/example/first-pdf.folio'))

type Box = { x: number; y: number; width: number; height: number }

async function boxOf(target: Locator): Promise<Box> {
  const measured = await target.boundingBox()
  expect(measured, 'the target was not painted').not.toBeNull()
  return measured!
}

// THE FALLBACK FILE TIER, FORCED. Headless Chromium HAS the File System Access
// API, so without this the app takes the native tier, never emits a
// `filechooser`, and the wait below times out at 90s on a working application.
// `e2e/local-file-actions.spec.ts:11` is the model.
async function openTemplate(page: Page, name: string): Promise<void> {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name, mimeType: 'application/json', buffer: template })
  await expect(page.locator('.document-name')).toHaveText(name)
}

// ── DW-332 ────────────────────────────────────────────────────────────────────
// 1024px is the shell's DECLARED MINIMUM, and the bar is `display: flex` with no
// `flex-wrap` (`App.css:29`), so an overfull bar does not wrap — it grows
// sideways past the window, which is invisible to every jsdom assertion in the
// repository and visible to `boundingBox()` here.
//
// THE VIEWPORT IS THE ONLY BOX THAT DOES NOT GROW WITH THE CONTENT, so every
// inequality below is taken against it and never against the bar's own parent.
test('fits the document bar into the shell\'s declared 1024px minimum, in both modes', async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 })
  await openTemplate(page, 'statement.folio')

  const bar = page.getByRole('banner', { name: 'Document bar' })
  const lockup = bar.locator('.brand-lockup')
  const actions = bar.getByRole('group', { name: 'Local file actions' })
  const later = bar.locator('.later-control')
  const modes = bar.getByRole('group', { name: 'Designer mode' })
  const docs = bar.getByRole('group', { name: 'Documentation' })
  const docsLink = docs.getByRole('link', { name: 'Rendering library documentation' })
  const viewport = page.viewportSize()
  expect(viewport).not.toBeNull()
  const width = viewport!.width

  // THE SIX WORDS ARE REALLY IN THE LAID-OUT BAR. Without this the fit below
  // could be passing because the family is missing rather than because it fits.
  await expect(actions.getByRole('button')).toHaveCount(6)
  // AND SO IS THE DOCUMENTATION LINK, the bar's last item since it arrived.
  await expect(docsLink).toHaveCount(1)

  // Returns the bar's real spare room. `.later-control` is `margin-left: auto`
  // (`App.css:66`), so it IS the free space: the distance from the actions
  // group's right edge to the later control's left edge, less the one
  // `--space-3` gap that would separate them in a full bar.
  const fitsIn = async (state: string): Promise<number> => {
    const barBox = await boxOf(bar)
    const lockupBox = await boxOf(lockup)
    const actionsBox = await boxOf(actions)
    const laterBox = await boxOf(later)
    const modesBox = await boxOf(modes)
    const docsBox = await boxOf(docs)

    // THE BAR STAYS INSIDE THE WINDOW, on both ends. This is the assertion the
    // arithmetic in DW-332 stood in for.
    expect(barBox.x, state).toBeGreaterThanOrEqual(0)
    expect(barBox.width, state).toBeLessThanOrEqual(width)

    // AND SO DOES ITS CONTENT. The documentation link is now the last item, so
    // its right edge is where an overfull bar shows first, and it sits after the
    // mode switch; the brand lockup's left edge is the other end of the same
    // claim, and it would move if a future change reached the fit by pulling
    // content off the left instead.
    expect(lockupBox.x, state).toBeGreaterThanOrEqual(0)
    expect(modesBox.x + modesBox.width, state).toBeLessThanOrEqual(width)
    expect(docsBox.x, `${state} / documentation follows the mode switch`).toBeGreaterThanOrEqual(modesBox.x + modesBox.width)
    expect(docsBox.x + docsBox.width, state).toBeLessThanOrEqual(width)

    // A SINGLE ROW. Nothing in this bar may wrap or stack — the other way an
    // overfull bar hides. Every item shares the bar's own vertical band.
    for (const [name, box] of [['lockup', lockupBox], ['actions', actionsBox], ['later', laterBox], ['modes', modesBox], ['documentation', docsBox]] as const) {
      expect(box.y, `${state} / ${name}`).toBeGreaterThanOrEqual(barBox.y)
      expect(box.y + box.height, `${state} / ${name}`).toBeLessThanOrEqual(barBox.y + barBox.height)
    }

    // AND THE WHOLE LINE IS ON SCREEN, NOT MERELY TOUCHING IT: `toBeInViewport()`
    // at its default ratio is satisfied by one visible pixel, which is exactly
    // what an overfull bar leaves.
    await expect(modes, state).toBeInViewport({ ratio: 1 })
    await expect(lockup, state).toBeInViewport({ ratio: 1 })
    await expect(docsLink, state).toBeInViewport({ ratio: 1 })

    // THE LINK IS REACHABLE BY KEYBOARD: one Tab from the last mode button.
    await modes.getByRole('button').last().focus()
    await page.keyboard.press('Tab')
    await expect(docsLink, `${state} / Tab reaches the documentation link`).toBeFocused()
    await docsLink.blur()

    const room = laterBox.x - (actionsBox.x + actionsBox.width)
    expect(room, `${state} / spare room`).toBeGreaterThan(0)
    console.log(`DW-332 ${state}: viewport ${width}, bar ${barBox.width.toFixed(2)} x ${barBox.height.toFixed(2)}, actions right ${(actionsBox.x + actionsBox.width).toFixed(2)}, later-control x ${laterBox.x.toFixed(2)}, modes right ${(modesBox.x + modesBox.width).toFixed(2)}, gap between actions and later-control ${room.toFixed(2)}px`)
    return room
  }

  await fitsIn('design mode, saved, one-line file name')

  // DIRTY IS THE LONGER STATUS COPY — 'Unsaved local changes' against 'Saved
  // local file' — so the state is visited rather than assumed to be the same bar.
  await page.getByRole('button', { name: 'Place Text' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).press('Enter')
  await expect(bar.locator('.status-copy')).toHaveText('Unsaved local changes')
  await fitsIn('design mode, dirty')

  // PREVIEW SWAPS THE `.later-control` FOR THE RENDER FRESHNESS LINE, which is
  // the longer of the two texts that slot can carry, and it is measured over a
  // REAL rendered preview because that is the widest the bar ever gets.
  await page.getByRole('button', { name: 'PREVIEW' }).click()
  await expect(page.getByRole('img', { name: /revision \d+/ })).toBeVisible({ timeout: 60_000 })
  await expect(later).toHaveText(/^rendered /)
  await fitsIn('preview mode, real render')
})

// THE POSITIVE CONTROL FOR THE FIT ABOVE. Three of this gate's items conclude on
// an absence, and a fit assertion that never visits a failing case reports the
// same green as one that cannot fail at all. `.document-name` carries the file
// name and does not shrink below its own min-content, so a pathological
// single-token name is a route THROUGH THE PRODUCT — no product mutation, no
// perturbed constant — to a bar that genuinely overflows 1024px.
//
// This asserts the overflow, so it stays green while proving the measurement is
// live. Inverting it (asserting the fit instead) is what was run at the gate to
// see the fit assertion actually red; the RED figures are in the gate report.
test('control: the same measurement reports an overflow when the bar is genuinely overfull', async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 })
  await openTemplate(page, `${'a'.repeat(220)}.folio`)
  const bar = page.getByRole('banner', { name: 'Document bar' })
  const modes = bar.getByRole('group', { name: 'Designer mode' })
  const viewport = page.viewportSize()
  expect(viewport).not.toBeNull()
  const width = viewport!.width
  const barBox = await boxOf(bar)
  const modesBox = await boxOf(modes)
  console.log(`DW-332 control: viewport ${width}, bar ${barBox.width.toFixed(2)}, modes right ${(modesBox.x + modesBox.width).toFixed(2)}`)
  // THE INSTRUMENT SEES IT. If this ever goes green-by-fitting, the assertions in
  // the test above are no longer capable of failing and their green means nothing.
  expect(barBox.width).toBeGreaterThan(width)
  expect(modesBox.x + modesBox.width).toBeGreaterThan(width)
})

// ── DW-339 ────────────────────────────────────────────────────────────────────
// `OrientationProperty` is rendered as a SIBLING of `.property-grid`, not a
// child of it (`App.tsx`, the POSITION section), and `.property-section` is a
// single-column grid (`App.css:575`). So the stylesheet reading says its box
// should be the full section width — TWO columns — while the register recorded
// the question as open. It is a number, not an opinion.
test('measures the orientation control\'s box against the inspector\'s 1fr 1fr columns', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('button', { name: 'Place Line' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).press('Enter')
  await expect(page.getByLabel(/line component e/)).toHaveCount(1)

  const section = page.locator('.property-section-position')
  const grid = section.locator('.property-grid')
  const orientation = page.getByRole('group', { name: 'Orientation' })
  const editor = section.locator('.property-editor').filter({ has: orientation })
  await expect(orientation).toHaveCount(1)
  await expect(grid).toHaveCount(1)

  // THE GRID REALLY HAS TWO COLUMNS AT THIS VIEWPORT — the premise the whole
  // comparison rests on, measured rather than read off the stylesheet. Its first
  // two children share a row and each is about half the grid.
  const gridBox = await boxOf(grid)
  const cells = grid.locator(':scope > *')
  expect(await cells.count()).toBeGreaterThanOrEqual(2)
  const first = await boxOf(cells.nth(0))
  const second = await boxOf(cells.nth(1))
  expect(first.y, 'the first two grid cells share a row').toBeCloseTo(second.y, 0)
  expect(second.x, 'the second cell starts in a second column').toBeGreaterThan(first.x + first.width)
  expect(first.width, 'the two columns are 1fr 1fr').toBeCloseTo(second.width, 0)

  const editorBox = await boxOf(editor)
  const orientationBox = await boxOf(orientation)
  console.log(`DW-339: grid ${gridBox.width.toFixed(2)}, column ${first.width.toFixed(2)}, orientation editor ${editorBox.width.toFixed(2)} at x ${editorBox.x.toFixed(2)} (grid x ${gridBox.x.toFixed(2)}), segmented group ${orientationBox.width.toFixed(2)}, columns spanned ${(editorBox.width / first.width).toFixed(3)}`)

  // THE ANSWER. The control's box is the FULL section width — two columns, not
  // one — and it is left-aligned with the grid rather than indented or inset.
  expect(editorBox.width).toBeCloseTo(gridBox.width, 0)
  expect(editorBox.x).toBeCloseTo(gridBox.x, 0)
  expect(editorBox.width).toBeGreaterThan(first.width * 1.5)

  // AND IT DOES NOT ESCAPE THE PANEL. Full-bleed inside the section is the
  // finding; spilling past the inspector would be a different, worse one.
  const panel = page.locator('.property-section-position').locator('xpath=..')
  const panelBox = await boxOf(panel)
  expect(editorBox.x).toBeGreaterThanOrEqual(panelBox.x - 1)
  expect(editorBox.x + editorBox.width).toBeLessThanOrEqual(panelBox.x + panelBox.width + 1)

  // THE SEGMENTED GROUP FILLS THE FULL-BLEED BOX RATHER THAN SITTING IN HALF OF
  // IT — the distinction between "the wrapper is two columns wide" and "the
  // painted control is".
  expect(orientationBox.width).toBeCloseTo(editorBox.width, 0)
})
