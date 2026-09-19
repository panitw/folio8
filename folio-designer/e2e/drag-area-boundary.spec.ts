import { expect, test, type Page, type Locator } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'

test.use({ viewport: { width: 1600, height: 1900 } })
const component = (page: Page, id: string) => page.locator(`[data-component-id="${id}"]`)
const revision = async (page: Page) => Number((await page.getByTestId('engine-snapshot').textContent())?.match(/REVISION (\d+)/)?.[1])
async function bounds(locator: Locator) { const value = await locator.boundingBox(); if (!value) throw new Error('Missing box'); return value }
async function openFixture(page: Page, later = false, orphan: boolean | 'tall' = false) {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/REVISION 1/)
  const fixture = JSON.parse(readFileSync(new URL('../public/templates/starter.folio', import.meta.url), 'utf8'))
  fixture.bands.pageHeader.height = 61.123
  fixture.bands.pageFooter.height = 40.456
  fixture.bands.content.elements = [
    { id: 'e1', type: 'text', x: 70, y: orphan === 'tall' ? 10 : orphan ? 710 : later ? 820 : 120, width: 80, height: orphan === 'tall' ? 4000 : 24, value: 'Text', ...(orphan ? {} : { style: { fontFamily: 'Roboto', fontSize: 12 } }) },
    { id: 'e2', type: 'rect', x: 220, y: later ? 800 : 80, width: 80, height: 40, style: { background: '#e8e8e8' } },
  ]
  if (later) fixture.bands.content.elements.push({ id: 'e3', type: 'rect', x: 10, y: 10, width: 20, height: 20, style: { background: '#e8e8e8' } })
  fixture.nextId = 4
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name: 'drag-boundary.folio', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(fixture)) })
  await expect(component(page, 'e1')).toBeVisible()
}
async function drag(page: Page, id: string, dy: number, release = true) {
  const box = await bounds(component(page, id))
  await page.mouse.move(box.x + box.width * 0.3, box.y + box.height / 2)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width * 0.3, box.y + box.height / 2 + dy, { steps: 6 })
  if (release) await page.mouse.up()
}
async function expectContained(page: Page, id: string, band: Locator) {
  await expect.poll(async () => {
    const box = await bounds(component(page, id)), area = await bounds(band)
    return box.y >= area.y - 0.1 && box.y + box.height <= area.y + area.height + 0.1
  }).toBe(true)
}

for (const snap of [true, false]) test(`single text stops at content edges and remains selectable, Snap ${snap}`, async ({ page }, testInfo) => {
  await openFixture(page)
  if (!snap) await page.getByRole('button', { name: /^Snap / }).click()
  const band = page.locator('.page-band-content')
  const original = await bounds(component(page, 'e1')), area = await bounds(band), before = await revision(page)
  // Release well inside the footer. Both the visible proposal and committed
  // hit target must stay inside Content, including a non-grid-aligned foot.
  await drag(page, 'e1', area.y + area.height + 20 - original.y - original.height / 2, false)
  await expectContained(page, 'e1', band)
  const expectedBottomY = area.y + (snap ? Math.floor((area.height - original.height) / 6) * 6 : area.height - original.height)
  await expect.poll(async () => (await bounds(component(page, 'e1'))).y).toBeCloseTo(expectedBottomY, 1)
  await page.mouse.up()
  await expect.poll(() => revision(page)).toBe(before + 1)
  await expectContained(page, 'e1', band)
  expect((await bounds(component(page, 'e1'))).y).toBeCloseTo(expectedBottomY, 1)
  await page.mouse.click(area.x + area.width - 30, area.y + 120)
  await expect(component(page, 'e1')).not.toHaveClass(/canvas-component-selected/)
  await component(page, 'e1').click()
  await expect(component(page, 'e1')).toHaveClass(/canvas-component-selected/)
  if (snap) await page.screenshot({ path: testInfo.outputPath('footer-boundary.png') })
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(async () => (await bounds(component(page, 'e1'))).y).toBeCloseTo(original.y, 1)
  await drag(page, 'e1', -180)
  await expectContained(page, 'e1', band)
  await expect.poll(async () => (await bounds(component(page, 'e1'))).y).toBeCloseTo(area.y, 1)
})

test('a group stops at the first member boundary and undo restores both', async ({ page }) => {
  await openFixture(page)
  await component(page, 'e1').click()
  await component(page, 'e2').click({ modifiers: ['Shift'] })
  const one = await bounds(component(page, 'e1')), two = await bounds(component(page, 'e2'))
  const band = page.locator('.page-band-content'), area = await bounds(band), before = await revision(page)
  await drag(page, 'e1', area.height, false)
  await expectContained(page, 'e1', band)
  await expectContained(page, 'e2', band)
  const expectedY = area.y + Math.floor((area.height - one.height) / 6) * 6
  await expect.poll(async () => (await bounds(component(page, 'e1'))).y).toBeCloseTo(expectedY, 1)
  await page.mouse.up()
  await expect.poll(() => revision(page)).toBe(before + 1)
  await expectContained(page, 'e1', band)
  await expectContained(page, 'e2', band)
  expect((await bounds(component(page, 'e1'))).y).toBeCloseTo(expectedY, 1)
  expect((await bounds(component(page, 'e1'))).y - (await bounds(component(page, 'e2'))).y).toBeCloseTo(one.y - two.y, 1)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(async () => (await bounds(component(page, 'e1'))).y).toBeCloseTo(one.y, 1)
  expect((await bounds(component(page, 'e2'))).y).toBeCloseTo(two.y, 1)
})

test('page two movement stops at its own content edges', async ({ page }) => {
  await openFixture(page, true)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom out', exact: true }).click()
  await page.getByRole('button', { name: /^Snap / }).click()
  const band = page.locator('.page-band-content').nth(1)
  await expect(page.locator('.page-surface')).toHaveCount(2)
  await drag(page, 'e1', -150)
  await expectContained(page, 'e1', band)
  await expect.poll(async () => (await bounds(component(page, 'e1'))).y).toBeCloseTo((await bounds(band)).y, 1)
  await drag(page, 'e1', 450)
  await expectContained(page, 'e1', band)
  await expect.poll(async () => { const box = await bounds(component(page, 'e1')); return box.y + box.height }).toBeCloseTo((await bounds(band)).y + (await bounds(band)).height, 1)
  await expect(page.locator('.page-surface')).toHaveCount(2)
})

test('previously stranded text is selectable inside content without rewriting it on load', async ({ page }) => {
  await openFixture(page, false, true)
  const band = page.locator('.page-band-content')
  await expectContained(page, 'e1', band)
  const before = await revision(page)
  await component(page, 'e1').click()
  await expect(component(page, 'e1')).toHaveClass(/canvas-component-selected/)
  await expect(page.getByRole('textbox', { name: 'Y (pt)', exact: true })).toHaveValue('710')
  expect(await revision(page)).toBe(before)
  await drag(page, 'e1', -80)
  await expectContained(page, 'e1', band)
})

test('clicking a body from the inspector restores canvas keyboard actions', async ({ page }) => {
  await openFixture(page)
  await component(page, 'e1').click()
  await page.getByRole('textbox', { name: 'Y (pt)', exact: true }).focus()
  const box = await bounds(component(page, 'e1'))
  await page.mouse.click(box.x + box.width * 0.3, box.y + box.height / 2)
  await page.keyboard.press('Delete')
  await expect(component(page, 'e1')).toHaveCount(0)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect(component(page, 'e1')).toBeVisible()
})

for (const zoom of [0.5, 1, 1.5]) test(`selection chrome remains fully painted and usable at the content top-left corner at ${zoom}`, async ({ page }, testInfo) => {
  await openFixture(page)
  for (let i = 0; i < Math.abs(zoom - 1) * 10; i++) await page.getByRole('button', { name: zoom < 1 ? 'Zoom out' : 'Zoom in', exact: true }).click()
  await page.getByRole('button', { name: /^Snap / }).click()
  await component(page, 'e1').click()
  for (const name of ['X (pt)', 'Y (pt)']) {
    const before = await revision(page)
    await page.getByRole('textbox', { name, exact: true }).fill('0')
    await page.getByRole('textbox', { name, exact: true }).press('Tab')
    await expect.poll(() => revision(page)).toBe(before + 1)
  }
  const body = await bounds(component(page, 'e1'))
  const label = page.locator('.canvas-dimension')
  await expect(label).toHaveText('80 × 24')
  const labelBox = await bounds(label)
  expect(labelBox.y + labelBox.height).toBeLessThan(body.y)
  const painted = await page.screenshot({ clip: labelBox })
  // The label must paint identically with content clipping enabled. A normal
  // visibility assertion cannot detect an ancestor cropping its pixels.
  const unclip = await page.addStyleTag({ content: '.band-window { overflow: visible !important; }' })
  const unclipped = await page.screenshot({ clip: labelBox })
  await unclip.evaluate((node) => node.parentNode?.removeChild(node))
  expect(painted.equals(unclipped), 'Size label must not be cropped by the content window').toBe(true)
  const corner = { x: body.x - 2, y: body.y - 2 }
  const hit = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.className, corner)
  expect(hit).toContain('selection-handle-nw')
  await page.screenshot({ path: testInfo.outputPath('top-left-selection.png') })
  const beforeResize = await revision(page)
  await page.mouse.move(corner.x, corner.y); await page.mouse.down()
  await page.mouse.move(corner.x + 12 * zoom, corner.y + 6 * zoom, { steps: 3 }); await page.mouse.up()
  await expect.poll(() => revision(page)).toBe(beforeResize + 1)
  await expect(page.getByRole('textbox', { name: 'Width (pt)', exact: true })).toHaveValue('68')
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(async () => (await bounds(component(page, 'e1'))).width).toBeCloseTo(80 * zoom, 1)
  await component(page, 'e1').click()
  await expect(page.getByRole('textbox', { name: 'Width (pt)', exact: true })).toHaveValue('80')
})

test('selecting oversized text does not expose off-page handles or enlarge the scroll area', async ({ page }) => {
  await openFixture(page, false, 'tall')
  const region = page.getByRole('main', { name: 'Canvas region' })
  const pageTop = (await bounds(page.locator('.page-surface'))).y
  await component(page, 'e1').click({ position: { x: 30, y: 10 } })
  await expect(page.locator('.canvas-dimension')).toHaveText('80 × 4000')
  // A single fitting page must not gain scrollable space merely because its
  // oversized element is selected. Exercise scrolling and inspect the page
  // through Playwright instead of reading browser geometry in product code.
  await region.evaluate((node) => node.scrollTo({ top: 100000, behavior: 'instant' }))
  expect((await bounds(page.locator('.page-surface'))).y).toBe(pageTop)
  await expect(page.getByRole('button', { name: 'Resize e1', exact: true })).toHaveCount(0)
  const area = await bounds(page.locator('.page-band-content'))
  const body = await bounds(component(page, 'e1'))
  const hit = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.closest('[data-component-id]')?.getAttribute('data-component-id'), { x: body.x + 20, y: area.y + area.height + 20 })
  expect(hit).toBeUndefined()
})
