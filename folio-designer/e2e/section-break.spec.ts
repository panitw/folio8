import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'

// spec-section-break CAP-1 — THE SECTION BREAK ON THE CANVAS, THROUGH THE REAL
// GO WORKER.
//
// jsdom applies no stylesheet and has no hit area, so App.test.tsx proves the
// commands and the state; this file proves that a press at a real coordinate
// on a real 7px strip places, selects, drags, undoes and deletes the break,
// and that a drag released onto an element is refused by the engine with the
// line left where it was.
//
// Nothing here measures the DOM for geometry: `boundingBox()` is Playwright's
// protocol-side measurement, as band-boundary-drag.spec.ts already uses.

const revision = (page: Page) => page.getByTestId('engine-snapshot')
const handle = (page: Page) => page.locator('.section-break-handle')
const yField = (page: Page) => page.getByRole('textbox', { name: 'Y (pt)' })

async function boxOf(target: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
  await target.scrollIntoViewIfNeeded()
  const box = await target.boundingBox()
  if (!box) throw new Error('the target was not painted')
  return box
}

async function revisionText(page: Page): Promise<string> {
  return (await revision(page).innerText()).trim()
}

// Arm the palette entry, then click the content band `down` pixels below its top.
async function placeBreak(page: Page, down: number): Promise<void> {
  await page.getByRole('button', { name: 'Place Section Break' }).click()
  const content = await boxOf(page.getByRole('region', { name: 'Content', exact: true }))
  await page.mouse.click(content.x + content.width / 2, content.y + down)
}

test('places, selects, drags, undoes and deletes the section break', async ({ page }) => {
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect(handle(page)).toHaveCount(0)
  const entry = page.getByRole('button', { name: 'Place Section Break' })
  await expect(entry).toBeEnabled()

  // PLACE. One command; the line is drawn once, the entry disables, and the
  // break arrives selected with its Y field in Properties.
  await placeBreak(page, 200)
  await expect(handle(page)).toHaveCount(1)
  await expect(entry).toBeDisabled()
  await expect(handle(page)).toHaveAttribute('aria-pressed', 'true')
  await expect(yField(page)).toBeVisible()
  const placed = Number(await yField(page).inputValue())
  expect(placed).toBeGreaterThan(0)

  // DRAG 40px DOWN. A proposal is shown while the pointer moves and nothing is
  // sent until release.
  const afterPlace = await revisionText(page)
  const strip = await boxOf(handle(page))
  const start = { x: strip.x + 60, y: strip.y + strip.height / 2 }
  await page.mouse.move(start.x, start.y)
  await page.mouse.down()
  await page.mouse.move(start.x, start.y + 20)
  await page.mouse.move(start.x, start.y + 40)
  await expect(page.locator('.section-break-proposal')).toHaveCount(1)
  expect(await revisionText(page)).toBe(afterPlace)
  await page.mouse.up()
  await expect(revision(page)).not.toHaveText(afterPlace)
  await expect(page.locator('.section-break-proposal')).toHaveCount(0)
  await expect.poll(async () => Number(await yField(page).inputValue())).toBeGreaterThan(placed)

  // UNDO. One entry: the line returns to where it was placed.
  await page.keyboard.press('ControlOrMeta+z')
  await expect.poll(async () => Number(await yField(page).inputValue())).toBe(placed)

  // DELETE. The key is removed and the palette entry re-enabled.
  await handle(page).focus()
  await page.keyboard.press('Delete')
  await expect(handle(page)).toHaveCount(0)
  await expect(entry).toBeEnabled()
})

// Owner decision: an armed Section Break placed by keyboard lands in the middle
// of that sheet's content window, which the real engine accepts.
test('Enter on the content band places the break at the middle of the window', async ({ page }) => {
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('button', { name: 'Place Section Break' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).press('Enter')
  await expect(handle(page)).toHaveCount(1)
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Place Section Break' })).toBeDisabled()
  expect(Number(await yField(page).inputValue())).toBeGreaterThan(0)
})

test('a drag released onto an element is refused and the line stays put', async ({ page }) => {
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)

  // A rectangle well down the content band, then a break above it.
  await page.getByRole('button', { name: 'Place Rectangle' }).click()
  const content = await boxOf(page.getByRole('region', { name: 'Content', exact: true }))
  await page.mouse.click(content.x + 40, content.y + 320)
  const rect = page.getByLabel(/rect component e/)
  await expect(rect).toHaveCount(1)
  await page.getByLabel('Canvas region').press('Escape')

  await placeBreak(page, 120)
  await expect(handle(page)).toHaveCount(1)
  const placed = await yField(page).inputValue()

  // Drag the line down onto the rectangle's middle and release there.
  const strip = await boxOf(handle(page))
  const target = await boxOf(rect)
  const x = strip.x + strip.width - 40
  await page.mouse.move(x, strip.y + strip.height / 2)
  await page.mouse.down()
  await page.mouse.move(x, (strip.y + target.y + target.height / 2) / 2)
  await page.mouse.move(x, target.y + target.height / 2)
  await page.mouse.up()

  // The engine refuses, the canvas says which element blocked it, and the
  // break is where it was.
  await expect(page.getByRole('alert').filter({ hasText: 'section break' })).toBeVisible()
  await expect(handle(page)).toHaveCount(1)
  await expect(yField(page)).toHaveValue(placed)
})

// spec-section-break CAP-7: the Anchor checkbox is one engine command, and the
// tab's anchor icon follows it through undo; removing an unanchored break and
// undoing restores both.
test('turns Anchor off and on through undo, and removes an unanchored break', async ({ page }) => {
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const anchor = page.getByRole('checkbox', { name: 'Anchor' })
  const icon = page.locator('.section-break-tab .section-break-anchor-icon')

  await placeBreak(page, 200)
  await expect(handle(page)).toHaveCount(1)
  await expect(anchor).toBeChecked()
  await expect(icon).toHaveCount(1)

  // UNCHECK. One command: the revision moves once and the icon goes.
  const anchored = await revisionText(page)
  // The box is controlled by the engine's answer, so it flips only once the
  // command lands — a plain click, not uncheck(), which demands it flip at once.
  await anchor.click()
  await expect(revision(page)).not.toHaveText(anchored)
  await expect(icon).toHaveCount(0)
  await expect(anchor).not.toBeChecked()
  await expect(page.getByRole('alert')).toHaveCount(0)

  // UNDO restores the icon and the check. The checkbox keeps focus after the
  // toggle, and the window shortcuts ignore keys from any input, so undo is
  // pressed from the line's handle — as for every other checkbox.
  await handle(page).focus()
  await page.keyboard.press('ControlOrMeta+z')
  await expect(icon).toHaveCount(1)
  await expect(anchor).toBeChecked()

  // REDO, then REMOVE the unanchored break, then UNDO: the break returns
  // unanchored.
  await page.keyboard.press('ControlOrMeta+Shift+z')
  await expect(icon).toHaveCount(0)
  await handle(page).focus()
  await page.keyboard.press('Delete')
  await expect(handle(page)).toHaveCount(0)
  await page.keyboard.press('ControlOrMeta+z')
  await expect(handle(page)).toHaveCount(1)
  await expect(icon).toHaveCount(0)
})
