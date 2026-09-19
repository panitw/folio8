import { expect, test, type Page, type Locator } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'

test.use({ viewport: { width: 1600, height: 1900 } })
const element = (id: string) => `[data-component-id="${id}"]`
const selectedIds = (page: Page) => page.locator('.canvas-component-selected[data-component-id]').evaluateAll((nodes) => nodes.map((node) => node.getAttribute('data-component-id')))
const revision = async (page: Page) => Number((await page.getByTestId('engine-snapshot').textContent())?.match(/REVISION (\d+)/)?.[1])
const positions = async (page: Page) => page.locator('[data-component-id]').evaluateAll((nodes) => Object.fromEntries(nodes.map((node) => [node.getAttribute('data-component-id'), { x: (node as HTMLElement).style.getPropertyValue('--component-x'), y: (node as HTMLElement).style.getPropertyValue('--component-y'), width: (node as HTMLElement).style.getPropertyValue('--component-width'), height: (node as HTMLElement).style.getPropertyValue('--component-height') }])))
const rect = (id: string, x: number, y: number, width = 30, height = 20) => ({ id, type: 'rect', x, y, width, height, style: { background: '#e8e8e8' } })
const text = (id: string, x: number, y: number, size?: number) => ({ id, type: 'text', x, y, width: 60, height: 24, value: 'Sample', style: { fontFamily: 'Roboto', ...(size ? { fontSize: size } : {}) } })

async function openFixture(page: Page, pages = false, continuation: boolean | 'tail' = false, border = false, gap = false) {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/REVISION 1/)
  const fixture = JSON.parse(readFileSync(new URL('../public/templates/starter.folio', import.meta.url), 'utf8'))
  fixture.bands.pageHeader.elements = [rect('e8', 20, 12, 30, 12)]
  fixture.bands.content.elements = [text('e1', 20, 30, 10), text('e2', 110, 80, 14), rect('e3', 180, 110, 80, 40), { id: 'e4', type: 'line', x: 20, y: 180, width: 100, height: 1, style: { background: '#112233' } }, { id: 'e5', type: 'image', x: 145, y: 180, width: 30, height: 20, asset: null }, { id: 'e6', type: 'table', x: 220, y: 180, headerHeight: 12, bind: 'items[]', as: 'item', columns: [{ id: 'ea', width: 60, label: 'Item', bind: '{{item.name}}' }], style: { fontFamily: 'Roboto', fontSize: 10 } }, text('e7', 340, 80)]
  if (pages) fixture.bands.content.elements.push(rect('e9', 20, 800, 30, 20), continuation ? { ...text('eb', 400, continuation === 'tail' ? 200 : 620, 10), width: 80, height: 1200, value: continuation === 'tail' ? 'Short' : 'Continuation line\n'.repeat(90) } : rect('eb', 400, 620, 30, 100))
  if (gap) for (const component of fixture.bands.content.elements.slice(-2)) component.y = 900
  if (border) for (const component of fixture.bands.content.elements.slice(0, 2)) component.style.border = { edges: ['bottom'] }
  fixture.page.margin.left = 37
  fixture.page.margin.top = 35
  fixture.bands.pageHeader.height = 61
  fixture.nextId = 12
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name: 'selection.folio', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(fixture)) })
  await expect(page.locator(element('e1'))).toBeVisible()
  await expect(page.locator('[data-component-id]')).toHaveCount(pages ? 10 : 8)
}

for (const action of ['last checkbox', 'Clear button']) test(`bulk border ${action} clears atomically and one undo restores both targets`, async ({ page }) => {
  await openFixture(page, false, false, true)
  await selectPair(page)
  const before = await revision(page)
  if (action === 'last checkbox') await page.getByRole('checkbox', { name: 'Border bottom' }).click()
  else await page.getByRole('button', { name: 'Clear Border edges' }).click()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${before + 1}\\b`))
  await expect(page.getByRole('button', { name: 'Clear Border edges' })).toHaveCount(0)
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(page.getByRole('textbox', { name: 'Font size (pt)' })).toHaveValue('')
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => selectedIds(page)).toEqual([])
  await selectPair(page)
  await expect(page.getByRole('button', { name: 'Clear Border edges' })).toBeVisible()
  await expect(page.getByRole('checkbox', { name: 'Border bottom' })).toBeChecked()
})

// Compare actual raster output with Grid off. This checks pitch and phase
// without consulting browser layout/style measurements or golden fixtures.
async function assertPaintedGrid(page: Page, band: Locator, phase: number, pitch: number) {
  const on = await band.screenshot()
  await page.getByRole('button', { name: /^Grid / }).click()
  const off = await band.screenshot()
  await page.getByRole('button', { name: /^Grid / }).click()
  const evidence = await page.evaluate(async ({ on, off, phase, pitch }) => {
    const pixels = async (base64: string) => {
      const bytes = Uint8Array.from(atob(base64), (character) => character.charCodeAt(0))
      const bitmap = await createImageBitmap(new Blob([bytes], { type: 'image/png' }))
      const surface = document.createElement('canvas'); surface.width = bitmap.width; surface.height = bitmap.height
      const context = surface.getContext('2d')!; context.drawImage(bitmap, 0, 0); bitmap.close()
      return context.getImageData(0, 0, surface.width, surface.height)
    }
    const a = await pixels(on), b = await pixels(off)
    const differs = (x: number, y: number) => {
      for (let row = Math.floor(y) - 1; row <= Math.floor(y) + 1; row++) for (let col = Math.floor(x) - 1; col <= Math.floor(x) + 1; col++) {
        const index = (row * a.width + col) * 4
        if (Math.abs(a.data[index]! - b.data[index]!) + Math.abs(a.data[index + 1]! - b.data[index + 1]!) + Math.abs(a.data[index + 2]! - b.data[index + 2]!) > 4) return true
      }
      return false
    }
    return Array.from({ length: 9 }, (_, i) => {
      const x = (50 + i % 3) * pitch, y = (50 + Math.floor(i / 3)) * pitch + phase
      return { dot: differs(x, y), gap: differs(x + pitch / 2, y + pitch / 2) }
    })
  }, { on: on.toString('base64'), off: off.toString('base64'), phase, pitch })
  expect(evidence).toEqual(Array.from({ length: 9 }, () => ({ dot: true, gap: false })))
}
async function box(locator: Locator) { const bounds = await locator.boundingBox(); if (!bounds) throw new Error('missing painted target'); return bounds }
async function marquee(page: Page, a: { x: number; y: number }, b: { x: number; y: number }, shift = false) {
  if (shift) await page.keyboard.down('Shift')
  await page.mouse.move(a.x, a.y); await page.mouse.down(); await page.mouse.move(b.x, b.y, { steps: 5 })
  await expect(page.getByLabel('Selection rectangle')).toBeVisible()
  await page.mouse.up()
  if (shift) await page.keyboard.up('Shift')
  await expect(page.getByLabel('Selection rectangle')).toHaveCount(0)
}
async function selectPair(page: Page) {
  await page.locator(element('e1')).click()
  await page.locator(element('e2')).click({ modifiers: ['Shift'] })
  await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2'])
}
async function dragMember(page: Page, id: string, dx: number, dy: number, release = true) {
  const member = await box(page.locator(element(id)))
  const start = { x: member.x + member.width / 2, y: member.y + member.height / 2 }
  await page.mouse.move(start.x, start.y); await page.mouse.down()
  await page.mouse.move(start.x + dx, start.y + dy, { steps: 5 })
  if (release) await page.mouse.up()
}

test('blank canvas outside the page gutters and below the last page starts rectangles', async ({ page }) => {
  await page.setViewportSize({ width: 2400, height: 1900 })
  await openFixture(page)
  await selectPair(page)
  const region = await box(page.getByRole('main', { name: 'Canvas region' }))
  const band = await box(page.getByRole('region', { name: 'Content', exact: true }))
  const sheet = await box(page.locator('.page-surface'))
  const outside = { x: region.x + 20, y: band.y + 20 }
  expect(outside.x).toBeLessThan(sheet.x - 116)
  await page.mouse.click(outside.x, outside.y)
  await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2'])
  await marquee(page, outside, { x: band.x + 200, y: band.y + 160 })
  await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2'])
  await marquee(page, { x: sheet.x - 30, y: sheet.y + sheet.height + 80 }, { x: band.x + 90, y: band.y + 20 })
  await expect.poll(() => selectedIds(page)).toEqual(['e1'])
})

test('loaded image paint stays inside its moving group box through preview and cancellation', async ({ page }) => {
  await openFixture(page)
  await page.locator(element('e5')).click()
  const picker = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Choose image…' }).click()
  await (await picker).setFiles({ name: 'logo.png', mimeType: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAADklEQVR4nGJqAAQAAP//AIYAg0yeIEsAAAAASUVORK5CYII=', 'base64') })
  const image = page.locator(`${element('e5')} img`)
  await expect(image).toBeVisible()
  const paint = () => image.evaluate((node) => { const s = (node as HTMLElement).style; return { left: s.left, top: s.top, width: s.width, height: s.height } })
  const initial = await paint()
  expect(initial.left).toBe('5px')
  await page.locator(element('e1')).click({ modifiers: ['Shift'] })
  const before = await positions(page), oldRevision = await revision(page)
  await dragMember(page, 'e1', 25, 15, false)
  await expect.poll(() => positions(page)).not.toEqual(before)
  expect(await paint()).toEqual(initial)
  await page.keyboard.press('Escape'); await page.mouse.up()
  await expect.poll(() => positions(page)).toEqual(before)
  expect(await paint()).toEqual(initial)
  expect(await revision(page)).toBe(oldRevision)
})

test('accepted group previews stop inside every member starting content window', async ({ page }) => {
  await openFixture(page, true)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom out', exact: true }).click()
  await page.getByRole('button', { name: /^Snap / }).click()
  await page.locator(element('e3')).click()
  await page.locator(element('e9')).click({ modifiers: ['Shift'] })
  const before = await positions(page), oldRevision = await revision(page)
  const last = await box(page.getByRole('region', { name: /^Content on page 2 / }))
  await dragMember(page, 'e3', 0, 500, false)
  await expect.poll(() => positions(page)).not.toEqual(before)
  const preview = page.locator(element('e9'))
  await expect.poll(async () => { const b = await box(preview); return b.y >= last.y && b.y + b.height <= last.y + last.height + 0.1 }).toBe(true)
  await expect(preview).toBeInViewport()
  await page.keyboard.press('Escape'); await page.mouse.up()
  await expect.poll(() => positions(page)).toEqual(before)
  expect(await revision(page)).toBe(oldRevision)
})

test('accepted group previews stop before a skipped column interval between windows', async ({ page }) => {
  await openFixture(page, true, false, false, true)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom out', exact: true }).click()
  await page.getByRole('button', { name: /^Snap / }).click()
  await page.locator(element('e3')).click()
  await page.locator(element('e7')).click({ modifiers: ['Shift'] })
  const before = await positions(page), oldRevision = await revision(page)
  const first = await box(page.getByRole('region', { name: /^Content on page 1 / }))
  const nextOrigin = 900 - Number.parseFloat(before.e9!.y) / 0.5
  const windowEnd = first.height / 0.5
  expect(nextOrigin).toBeGreaterThan(windowEnd)
  const targetY = (nextOrigin + windowEnd) / 2
  await dragMember(page, 'e3', 0, (targetY - 110) * 0.5, false)
  await expect.poll(() => positions(page)).not.toEqual(before)
  const preview = page.locator(element('e3'))
  await expect.poll(async () => { const b = await box(preview); return b.y >= first.y && b.y + b.height <= first.y + first.height + 0.1 }).toBe(true)
  await expect(preview).toBeInViewport()
  await page.keyboard.press('Escape'); await page.mouse.up()
  await expect.poll(() => positions(page)).toEqual(before)
  expect(await revision(page)).toBe(oldRevision)
})

test('a selected oversized continuation does not increase its existing overflow', async ({ page }) => {
  await openFixture(page, true, 'tail')
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom out', exact: true }).click()
  await page.getByRole('button', { name: /^Snap / }).click()
  await page.locator(element('e3')).evaluate((node) => (node as HTMLElement).focus({ preventScroll: true })); await page.keyboard.press('Enter')
  await page.locator(element('eb')).evaluate((node) => (node as HTMLElement).focus({ preventScroll: true })); await page.keyboard.press('Shift+Enter')
  const before = await positions(page), oldRevision = await revision(page)
  await dragMember(page, 'e3', 12, 250)
  await expect.poll(() => revision(page)).toBe(oldRevision + 1)
  const after = await positions(page)
  for (const id of ['e3', 'eb']) {
    expect(after[id]!.y).toBe(before[id]!.y)
    expect(Number.parseFloat(after[id]!.x) - Number.parseFloat(before[id]!.x)).toBeCloseTo(12, 3)
    expect(after[id]!.height).toBe(before[id]!.height)
  }
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => positions(page)).toEqual(before)
})

for (const zoom of [0.5, 1, 1.5]) {
  test(`AC-1/3/5/7: four directions, additive and empty rectangles at ${zoom * 100}%`, async ({ page }) => {
    await openFixture(page)
    const control = zoom < 1 ? 'Zoom out' : 'Zoom in'
    for (let i = 0; i < Math.abs(zoom - 1) * 10; i++) await page.getByRole('button', { name: control, exact: true }).click()
    const band = await box(page.getByRole('region', { name: 'Content', exact: true }))
    const a = { x: band.x + 10 * zoom, y: band.y + 20 * zoom }, b = { x: band.x + 200 * zoom, y: band.y + 160 * zoom }
    const before = await revision(page)
    for (const [start, end] of [[a, b], [b, a], [{ x: a.x, y: b.y }, { x: b.x, y: a.y }], [{ x: b.x, y: a.y }, { x: a.x, y: b.y }]]) {
      await marquee(page, start!, end!)
      await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2'])
    }
    await page.getByRole('button', { name: /^Grid / }).click()
    await page.getByRole('button', { name: /^Snap / }).click()
    await marquee(page, { x: band.x + 330 * zoom, y: band.y + 70 * zoom }, { x: band.x + 410 * zoom, y: band.y + 115 * zoom }, true)
    await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2', 'e7'])
    await marquee(page, { x: band.x + 400 * zoom, y: band.y + 300 * zoom }, { x: band.x + 450 * zoom, y: band.y + 350 * zoom }, true)
    await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2', 'e7'])
    await marquee(page, { x: band.x + 400 * zoom, y: band.y + 300 * zoom }, { x: band.x + 450 * zoom, y: band.y + 350 * zoom })
    await expect.poll(() => selectedIds(page)).toEqual([])
    expect(await revision(page)).toBe(before)
  })
}

test('AC-8/9/10/11/12 and G-10: common fields, inert mixed/default blur, atomic property undo/refusal', async ({ page }) => {
  await openFixture(page)
  await selectPair(page)
  const before = await revision(page)
  const size = page.getByRole('textbox', { name: 'Font size (pt)' })
  await expect(size).toHaveValue(''); await expect(size).toHaveAttribute('aria-description', /Mixed/)
  await size.focus(); await size.press('Tab')
  await size.fill('15'); await size.fill(''); await size.press('Tab')
  expect(await revision(page)).toBe(before)
  await size.fill('18'); await size.press('Enter')
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${before + 1}\\b`))
  await expect(size).toHaveValue('18')
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => selectedIds(page)).toEqual([])
  await selectPair(page)
  await expect(size).toHaveValue('')
  const beforePosition = await positions(page)
  const x = page.getByRole('textbox', { name: 'X (pt)', exact: true })
  await x.fill('42'); await x.press('Enter')
  await expect(x).toHaveValue('42')
  await expect.poll(() => positions(page)).toMatchObject({ e1: { x: '42px' }, e2: { x: '42px' } })
  const absolute = await positions(page)
  expect(absolute.e1!.x).toBe('42px'); expect(absolute.e2!.x).toBe('42px')
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => selectedIds(page)).toEqual([])
  await selectPair(page)
  await expect.poll(() => positions(page)).toEqual(beforePosition)
  const y = page.getByRole('textbox', { name: 'Y (pt)', exact: true })
  await y.fill('56'); await y.press('Enter')
  await expect(y).toHaveValue('56')
  await expect.poll(() => positions(page)).toMatchObject({ e1: { y: '56px' }, e2: { y: '56px' } })
  const absoluteY = await positions(page)
  expect(absoluteY.e1!.y).toBe('56px'); expect(absoluteY.e2!.y).toBe('56px')
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => selectedIds(page)).toEqual([])
  await selectPair(page)
  await expect.poll(() => positions(page)).toEqual(beforePosition)
  const width = page.getByRole('textbox', { name: 'Width (pt)', exact: true })
  const beforeRefusal = await revision(page)
  await width.fill('490'); await width.press('Enter')
  await expect(page.getByRole('alert')).toContainText(/e2|geometry/)
  expect(await revision(page)).toBe(beforeRefusal)
  await page.locator(element('e3')).click({ modifiers: ['Shift'] })
  await expect(size).toHaveCount(0); await expect(page.getByRole('textbox', { name: 'Background', exact: true })).toBeVisible()
  await expect(page.locator('.property-section-binding')).toHaveCount(0)
  await page.locator(element('e4')).click({ modifiers: ['Shift'] })
  await expect(page.getByRole('textbox', { name: 'Border width (pt)' })).toHaveCount(0)
  await page.locator(element('e6')).click({ modifiers: ['Shift'] })
  await expect(width).toHaveCount(0)
  await page.locator(element('e7')).click()
  await page.locator(element('e1')).click({ modifiers: ['Shift'] })
  await page.getByRole('button', { name: 'Clear Font size (pt)', exact: true }).click()
  const defaultSize = page.getByRole('textbox', { name: 'Font size (pt)' })
  await expect(defaultSize).toHaveValue('12')
  await expect(defaultSize).toHaveAttribute('placeholder', '12')
  const defaultRevision = await revision(page)
  await defaultSize.focus(); await defaultSize.press('Tab')
  expect(await revision(page)).toBe(defaultRevision)
})

test('G-1/3/5/6: shared preview, click retention, snap, cancel and one undo/redo', async ({ page }) => {
  await openFixture(page)
  await selectPair(page)
  await page.locator(element('e7')).click({ modifiers: ['Shift'] })
  const before = await positions(page), oldRevision = await revision(page)
  await page.locator(element('e1')).click()
  await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2', 'e7'])
  expect(await revision(page)).toBe(oldRevision)
  await dragMember(page, 'e2', 17, 13, false)
  await expect(page.getByRole('textbox', { name: 'X (pt)', exact: true })).toHaveAttribute('readonly', '')
  await expect.poll(() => positions(page)).not.toEqual(before)
  await page.mouse.up()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 1}\\b`))
  const moved = await positions(page)
  const delta = (id: string, axis: 'x' | 'y') => Number.parseFloat(moved[id]![axis]) - Number.parseFloat(before[id]![axis])
  for (const id of ['e2', 'e7']) { expect(delta('e1', 'x')).toBe(delta(id, 'x')); expect(delta('e1', 'y')).toBe(delta(id, 'y')) }
  expect(moved.e3).toEqual(before.e3)
  await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2', 'e7'])
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => positions(page)).toEqual(before)
  await page.getByRole('button', { name: 'Redo', exact: true }).click()
  await expect.poll(() => positions(page)).toEqual(moved)
  await selectPair(page)
  await page.locator(element('e7')).click({ modifiers: ['Shift'] })
  const cancelRevision = await revision(page)
  await dragMember(page, 'e1', 15, 15, false)
  await page.keyboard.press('Escape'); await page.mouse.up()
  await expect.poll(() => positions(page)).toEqual(moved)
  expect(await revision(page)).toBe(cancelRevision)
  const originalMember = await box(page.locator(element('e1')))
  await page.mouse.move(originalMember.x + 15, originalMember.y + 10); await page.mouse.down()
  await page.mouse.move(originalMember.x + 35, originalMember.y + 30)
  await page.mouse.move(originalMember.x + 15, originalMember.y + 10); await page.mouse.up()
  await expect.poll(() => positions(page)).toEqual(moved)
  expect(await revision(page)).toBe(cancelRevision)
  await page.getByRole('button', { name: /^Snap / }).click()
  await dragMember(page, 'e1', 1.125, 3.375)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${cancelRevision + 1}\\b`))
})

test('AC-2/6 and G-2/4/7: all kinds move from a selected table cell and stop together at the edge', async ({ page }) => {
  await openFixture(page)
  const band = await box(page.getByRole('region', { name: 'Content', exact: true }))
  await marquee(page, { x: band.x + 5, y: band.y + 20 }, { x: band.x + 310, y: band.y + 240 })
  await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2', 'e3', 'e4', 'e5', 'e6'])
  const before = await positions(page), oldRevision = await revision(page)
  await dragMember(page, 'e6', 1000, 0)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 1}\\b`))
  const after = await positions(page)
  const dx = Number.parseFloat(after.e1!.x) - Number.parseFloat(before.e1!.x)
  for (const id of ['e1', 'e2', 'e3', 'e4', 'e5', 'e6']) {
    expect(Number.parseFloat(after[id]!.x) - Number.parseFloat(before[id]!.x)).toBe(dx)
    expect(after[id]!.width).toBe(before[id]!.width); expect(after[id]!.height).toBe(before[id]!.height)
  }
  await expect(page.locator('.canvas-table-column-selected')).toHaveCount(0)
  expect(after.e7).toEqual(before.e7)
})

test('AC-4/14 and G-8/9: page-spanning rectangle, repeated echo drag, zoom and scroll cancellation', async ({ page }) => {
  await openFixture(page, true)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom out', exact: true }).click()
  const first = await box(page.locator('.page-surface').first()), second = await box(page.locator('.page-surface').nth(1))
  await marquee(page, { x: first.x + 1, y: first.y + 1 }, { x: second.x + second.width - 1, y: second.y + second.height - 1 })
  await expect.poll(() => selectedIds(page)).toEqual(['e8', 'e1', 'e2', 'e3', 'e4', 'e5', 'e6', 'e7', 'e9', 'eb'])
  const oldRevision = await revision(page)
  const echo = page.locator('.page-surface').nth(1).locator('.page-band-pageHeader .canvas-component-echo')
  const at = await box(echo)
  await page.mouse.move(at.x + 5, at.y + 3); await page.mouse.down(); await page.mouse.move(at.x + 11, at.y + 5); await page.mouse.up()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 1}\\b`))
  await expect(page.getByLabel('Selection rectangle')).toHaveCount(0)
  const beforeRightClick = await selectedIds(page)
  await echo.click({ button: 'right', modifiers: ['Shift'] })
  await expect.poll(() => selectedIds(page)).toEqual(beforeRightClick)
  await echo.click({ modifiers: ['Shift'] })
  await expect.poll(() => selectedIds(page)).toEqual(['e1', 'e2', 'e3', 'e4', 'e5', 'e6', 'e7', 'e9', 'eb'])
  await page.locator(element('e8')).click({ modifiers: ['Shift'] })
  const afterMove = await positions(page)
  await page.setViewportSize({ width: 1600, height: 700 })
  await dragMember(page, 'e6', 8, 8, false)
  await expect(page.getByRole('textbox', { name: 'X (pt)', exact: true })).toHaveAttribute('readonly', '')
  await page.mouse.wheel(0, 120)
  await expect(page.getByRole('textbox', { name: 'X (pt)', exact: true })).not.toHaveAttribute('readonly', '')
  await page.mouse.up()
  await expect.poll(() => positions(page)).toEqual(afterMove)
  await page.getByRole('button', { name: 'Zoom in', exact: true }).click()
  await expect(page.getByLabel('Canvas zoom')).toHaveText('60%')
  expect(await revision(page)).toBe(oldRevision + 1)
})


test('Snap grid shares the non-grid-aligned band origin at 150%', async ({ page }) => {
  await openFixture(page)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom in', exact: true }).click()
  await selectPair(page)
  const oldRevision = await revision(page)
  await dragMember(page, 'e2', 19, 17)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 1}\\b`))
  const moved = await positions(page)
  expect(Number.parseFloat(moved.e2!.x) / 1.5 % 6).toBe(0)
  expect(Number.parseFloat(moved.e2!.y) / 1.5 % 6).toBe(0)
  const band = page.getByRole('region', { name: 'Content', exact: true })
  expect(await band.evaluate((node) => (node as HTMLElement).style.getPropertyValue('--band-grid-offset'))).toBe('0px')
  await assertPaintedGrid(page, band, 0, 9)
  await page.screenshot({ path: '/tmp/folio8-selection-grid.png', fullPage: true })
})

test('the painted grid retains a nonzero content continuation phase', async ({ page }) => {
  await openFixture(page, true, true)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom in', exact: true }).click()
  const band = page.getByRole('region', { name: /^Content on page 2 / })
  const phase = await band.evaluate((node) => Number.parseFloat((node as HTMLElement).style.getPropertyValue('--band-grid-offset')))
  const echo = page.locator('.page-surface').nth(1).locator('.page-band-content .canvas-component-echo.canvas-component-text')
  const origin = 620 - await echo.evaluate((node) => Number.parseFloat((node as HTMLElement).style.getPropertyValue('--component-y'))) / 1.5
  const expectedPhase = -(origin % 6) * 1.5
  expect(expectedPhase).not.toBe(0)
  expect(phase).toBeCloseTo(expectedPhase, 6)
  await assertPaintedGrid(page, band, expectedPhase, 9)
})


test('G-8: dragging a continuation toward another page preserves its existing vertical overflow', async ({ page }) => {
  await openFixture(page, true, true)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom out', exact: true }).click()
  await page.getByRole('button', { name: /^Snap / }).click()
  await page.locator(element('e9')).evaluate((node) => (node as HTMLElement).focus({ preventScroll: true })); await page.keyboard.press('Enter')
  await page.locator(element('eb')).evaluate((node) => (node as HTMLElement).focus({ preventScroll: true })); await page.keyboard.press('Shift+Enter')
  await expect.poll(async () => (await selectedIds(page)).sort()).toEqual(['e9', 'eb'])
  const before = await positions(page), oldRevision = await revision(page)
  const band2 = await box(page.getByRole('region', { name: /^Content on page 2 / }))
  const band3 = await box(page.getByRole('region', { name: /^Content on page 3 / }))
  const origin = async (index: number) => 620 - await page.locator('.page-surface').nth(index).locator('.page-band-content .canvas-component-echo.canvas-component-text').evaluate((node) => Number.parseFloat((node as HTMLElement).style.getPropertyValue('--component-y'))) / 0.5
  const origin2 = await origin(1), origin3 = await origin(2)
  const start = { x: band2.x + 207, y: band2.y + Math.min(band2.height, (origin3 - origin2) * 0.5) - 20 }
  const end = { x: band3.x + 215, y: band3.y + 20 }
  const expectedDY = 0 // Oversized content may move horizontally, but cannot increase overflow.
  const hit = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.outerHTML.slice(0, 500), start)
  expect(hit, JSON.stringify({ start, end, origin2, origin3, band2, band3 })).toContain('canvas-component-echo')
  await page.mouse.move(start.x, start.y); await page.mouse.down()
  await page.mouse.move(end.x, end.y, { steps: 5 }); await page.mouse.up()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 1}\\b`))
  await expect.poll(async () => (await selectedIds(page)).sort()).toEqual(['e9', 'eb'])
  await page.getByRole('main', { name: 'Canvas region' }).press('Escape')
  for (const [id, originalY] of [['e9', 800], ['eb', 620]] as const) {
    await page.locator(element(id)).evaluate((node) => (node as HTMLElement).focus({ preventScroll: true })); await page.keyboard.press('Enter')
    const actual = Number(await page.getByRole('textbox', { name: 'Y (pt)', exact: true }).inputValue())
    expect(actual - originalY).toBeCloseTo(expectedDY, 3)
  }
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => positions(page)).toEqual(before)
})

test('AC-6: armed placement passes through a selected repeated echo', async ({ page }) => {
  await openFixture(page, true)
  for (let i = 0; i < 5; i++) await page.getByRole('button', { name: 'Zoom out', exact: true }).click()
  await page.locator(element('e8')).click()
  await page.locator(element('e1')).click({ modifiers: ['Shift'] })
  const echo = page.locator('.page-surface').nth(1).locator('.page-band-pageHeader .canvas-component-echo')
  const at = await box(echo)
  await page.getByRole('button', { name: 'Place Rectangle' }).click()
  const oldRevision = await revision(page)
  await page.mouse.click(at.x + 5, at.y + 3)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 1}\\b`))
  await expect(page.getByLabel('Selection rectangle')).toHaveCount(0)
  await expect(page.locator('[data-component-id]')).toHaveCount(11)
})


test('G-7: unselected drag owns one element and line endpoint resize keeps thickness', async ({ page }) => {
  await openFixture(page)
  await selectPair(page)
  const before = await positions(page), oldRevision = await revision(page)
  await dragMember(page, 'e3', 12, 6)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 1}\\b`))
  await expect.poll(() => selectedIds(page)).toEqual(['e3'])
  const moved = await positions(page)
  expect(moved.e1).toEqual(before.e1); expect(moved.e2).toEqual(before.e2)
  expect(moved.e3).not.toEqual(before.e3)
  await page.locator(element('e4')).click()
  const end = await box(page.getByRole('button', { name: 'Resize e4 end', exact: true }))
  await page.mouse.move(end.x + end.width / 2, end.y + end.height / 2); await page.mouse.down()
  await page.mouse.move(end.x + end.width / 2 + 24, end.y + end.height / 2 + 30); await page.mouse.up()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(new RegExp(`REVISION ${oldRevision + 2}\\b`))
  const resized = await positions(page)
  expect(resized.e4!.height).toBe(before.e4!.height)
  expect(resized.e4!.width).not.toBe(before.e4!.width)
  expect(resized.e1).toEqual(before.e1); expect(resized.e2).toEqual(before.e2)
})
