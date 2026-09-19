import { readFileSync } from 'node:fs'
import { expect, test, type Page } from '@playwright/test'
import { openWorkspace, startBlankFromNew } from './app.js'

// SPEC-multi-pages story 2 — PAGES ON THE CANVAS, THROUGH THE REAL GO WORKER.
//
// App.test.tsx proves the commands and the selection state against a fake
// engine; this file proves the whole loop: Add page twice from a blank
// document, delete the middle page after confirming, undo it, clear page 2's
// Page Break, and save bytes that state `"pageBreak": false`.

// Tall enough to show two sheets at the zoom the cross-page drag uses.
test.use({ viewport: { width: 1600, height: 1900 } })

const revision = (page: Page) => page.getByTestId('engine-snapshot')
const tools = (page: Page) => page.getByLabel('Canvas controls')
const labels = (page: Page) => page.locator('.page-label')

test('adds pages, deletes one after confirming, undoes it, and saves Page Break off', async ({ page }) => {
  await page.addInitScript(() => {
    const handle = {
      name: 'pages.folio',
      getFile: async () => new File([], 'pages.folio', { type: 'application/json' }),
      createWritable: async () => ({ write: async (written: ArrayBuffer) => { (window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes = [Array.from(new Uint8Array(written))] }, close: async () => undefined }),
    }
    Object.assign(window, { showSaveFilePicker: async () => handle })
  })
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await startBlankFromNew(page)
  await expect(labels(page)).toHaveText(['Page 1'])
  await expect(tools(page).getByRole('button', { name: 'Delete page' })).toBeDisabled()

  // ADD TWICE. Each new page arrives selected, and the next goes after it.
  await tools(page).getByRole('button', { name: 'Add page' }).click()
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2'])
  await expect(page.getByRole('button', { name: 'Page 2', exact: true })).toHaveAttribute('aria-pressed', 'true')
  await tools(page).getByRole('button', { name: 'Add page' }).click()
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2', 'Page 3'])

  // DELETE PAGE 2, CANCELLED FIRST. Escape changes nothing.
  await page.getByRole('button', { name: 'Page 2', exact: true }).click()
  const beforeDelete = (await revision(page).innerText()).trim()
  await tools(page).getByRole('button', { name: 'Delete page' }).click()
  const dialog = page.getByRole('dialog', { name: 'Delete page 2?' })
  await expect(dialog).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  expect((await revision(page).innerText()).trim()).toBe(beforeDelete)

  // The Delete key never deletes a page.
  await page.getByRole('button', { name: 'Page 2', exact: true }).click()
  await page.keyboard.press('Delete')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(labels(page)).toHaveCount(3)

  // CONFIRMED, then UNDONE: one entry each way.
  await tools(page).getByRole('button', { name: 'Delete page' }).click()
  await page.getByRole('dialog', { name: 'Delete page 2?' }).getByRole('button', { name: 'Delete page' }).click()
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2'])
  await page.keyboard.press('ControlOrMeta+z')
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2', 'Page 3'])

  // PAGE BREAK. Disabled on page 1; cleared on page 2.
  await page.getByRole('button', { name: 'Page 1', exact: true }).click()
  await expect(page.getByRole('checkbox', { name: 'Page Break' })).toBeDisabled()
  await page.getByRole('button', { name: 'Page 2', exact: true }).click()
  const pageBreak = page.getByRole('checkbox', { name: 'Page Break' })
  await expect(pageBreak).toBeChecked()
  await pageBreak.click()
  await expect(pageBreak).not.toBeChecked()

  // SAVE. The written bytes state the setting.
  await page.getByRole('button', { name: 'Save As' }).click()
  await expect.poll(() => page.evaluate(() => (window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes?.[0]?.length ?? 0)).toBeGreaterThan(0)
  const saved = await page.evaluate(() => new TextDecoder().decode(new Uint8Array((window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes![0]!)))
  expect(saved).toContain('"pageBreak": false')
  expect(JSON.parse(saved).pages).toHaveLength(3)
})

// SPEC-multi-pages story 3 — ELEMENTS ON A SPECIFIC PAGE, THROUGH THE REAL GO
// WORKER. Place Text on page 2, place Text on page 1 and drag it onto page 2,
// undo (it is back on page 1), redo, and save bytes listing both under pages[1].
test('places on page 2, drags a page-1 element onto page 2, undoes, and saves it under pages[1]', async ({ page }) => {
  await page.addInitScript(() => {
    const handle = {
      name: 'elements.folio',
      getFile: async () => new File([], 'elements.folio', { type: 'application/json' }),
      createWritable: async () => ({ write: async (written: ArrayBuffer) => { (window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes = [Array.from(new Uint8Array(written))] }, close: async () => undefined }),
    }
    Object.assign(window, { showSaveFilePicker: async () => handle })
  })
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await startBlankFromNew(page)
  await tools(page).getByRole('button', { name: 'Add page' }).click()
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2'])
  const sheets = page.locator('.page-surface')
  const onSheet = (index: number) => sheets.nth(index).locator('.canvas-component:not(.canvas-component-echo)')
  // Zoom out so both pages sit inside the viewport: a pointer moved past the
  // viewport's edge reaches no sheet.
  for (let i = 0; i < 5; i++) await tools(page).getByRole('button', { name: 'Zoom out', exact: true }).click()

  // PLACE on page 2 from the keyboard: it lands on page 2's sheet.
  await page.getByRole('button', { name: 'Place Text' }).click()
  await page.getByLabel('Content on page 2 of 2').press('Enter')
  await expect(onSheet(1)).toHaveCount(1)
  await expect(onSheet(0)).toHaveCount(0)

  // PLACE on page 1, then DRAG it onto page 2's content band.
  await page.getByRole('button', { name: 'Place Text' }).click()
  await page.getByLabel('Content on page 1 of 2').press('Enter')
  await expect(onSheet(0)).toHaveCount(1)
  const source = onSheet(0)
  const from = await source.boundingBox()
  const target = await page.getByLabel('Content on page 2 of 2').boundingBox()
  if (!from || !target) throw new Error('canvas geometry is not visible')
  // The just-placed element is selected, and at this zoom its selection handles
  // cover most of its 36×12px body, so a press would resize. Clear the
  // selection first: a press on an unselected body selects it and drags.
  await page.keyboard.press('Escape')
  await expect(page.locator('.canvas-component-selected')).toHaveCount(0)
  const grab = { x: from.x + from.width / 2, y: from.y + from.height / 2 }
  await page.mouse.move(grab.x, grab.y)
  await page.mouse.down()
  await page.mouse.move(grab.x, (grab.y + target.y + 100) / 2, { steps: 5 })
  await page.mouse.move(grab.x, target.y + 100, { steps: 5 })
  // The preview follows the pointer onto page 2's sheet before release.
  await expect(onSheet(1)).toHaveCount(2)
  await page.mouse.up()
  await expect(onSheet(0)).toHaveCount(0)
  await expect(onSheet(1)).toHaveCount(2)

  // ONE UNDO puts it back on page 1; redo moves it again.
  await page.keyboard.press('ControlOrMeta+z')
  await expect(onSheet(0)).toHaveCount(1)
  await expect(onSheet(1)).toHaveCount(1)
  await page.keyboard.press('ControlOrMeta+Shift+z')
  await expect(onSheet(1)).toHaveCount(2)

  // SAVE. Both elements are listed under pages[1].
  await page.getByRole('button', { name: 'Save As' }).click()
  await expect.poll(() => page.evaluate(() => (window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes?.[0]?.length ?? 0)).toBeGreaterThan(0)
  const saved = JSON.parse(await page.evaluate(() => new TextDecoder().decode(new Uint8Array((window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes![0]!))))
  expect(saved.pages[0].elements).toHaveLength(0)
  expect(saved.pages[1].elements).toHaveLength(2)
})

// SPEC-multi-pages story 5 — A SECTION BREAK ON PAGE 2, THROUGH THE REAL GO
// WORKER. Place a break on page 2's content band: its handle is named for page
// 2 and drawn on page 2's sheet, the palette entry is disabled while page 2 is
// current and enabled on page 1, and the saved bytes hold pages[1].sectionBreak.
test('places a section break on page 2, follows the current page in the palette, and saves it under pages[1]', async ({ page }) => {
  await page.addInitScript(() => {
    const handle = {
      name: 'breaks.folio',
      getFile: async () => new File([], 'breaks.folio', { type: 'application/json' }),
      createWritable: async () => ({ write: async (written: ArrayBuffer) => { (window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes = [Array.from(new Uint8Array(written))] }, close: async () => undefined }),
    }
    Object.assign(window, { showSaveFilePicker: async () => handle })
  })
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await startBlankFromNew(page)
  await tools(page).getByRole('button', { name: 'Add page' }).click()
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2'])
  const sheets = page.locator('.page-surface')
  const entry = page.getByRole('button', { name: 'Place Section Break' })

  // PLACE on page 2 from the keyboard: the handle is page 2's, on page 2's sheet.
  await page.getByRole('button', { name: 'Page 2', exact: true }).click()
  await expect(entry).toBeEnabled()
  await entry.click()
  await page.getByLabel('Content on page 2 of 2').press('Enter')
  const pageTwoBreak = page.getByRole('button', { name: 'Section Break on page 2' })
  await expect(pageTwoBreak).toHaveAttribute('aria-pressed', 'true')
  await expect(sheets.nth(1).locator('.section-break-line')).toHaveCount(1)
  await expect(sheets.nth(0).locator('.section-break-line')).toHaveCount(0)

  // THE PALETTE FOLLOWS THE CURRENT PAGE (D-5.1).
  await expect(entry).toBeDisabled()
  await expect(page.getByText('This page already has its Section Break.')).toBeVisible()
  await page.getByRole('button', { name: 'Page 1', exact: true }).click()
  await expect(entry).toBeEnabled()
  await page.getByRole('button', { name: 'Page 2', exact: true }).click()
  await expect(entry).toBeDisabled()

  // SAVE. The break is page 2's key, and page 1 has none.
  await page.getByRole('button', { name: 'Save As' }).click()
  await expect.poll(() => page.evaluate(() => (window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes?.[0]?.length ?? 0)).toBeGreaterThan(0)
  const saved = JSON.parse(await page.evaluate(() => new TextDecoder().decode(new Uint8Array((window as typeof window & { __folio8Writes?: number[][] }).__folio8Writes![0]!))))
  expect(typeof saved.pages[1].sectionBreak).toBe('number')
  expect(saved.pages[0].sectionBreak).toBeUndefined()
})

// SPEC-multi-pages story 4 — THE HEADER, EDITABLE FROM ANY PAGE, THROUGH THE REAL
// GO WORKER. A three-page document with one header text: press page 3's header
// copy (a real hit test on an aria-hidden echo), edit the text there, see every
// copy change, undo once, then press page 2's echo.
test('edits the shared header from page 3, every copy changes, one undo reverts it, and page 2 echo takes a press', async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(revision(page)).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const fixture = JSON.parse(readFileSync(new URL('../public/templates/starter.folio', import.meta.url), 'utf8'))
  fixture.bands.pageHeader.elements = [{ id: 'e1', type: 'text', x: 0, y: 0, width: 200, height: 24, value: 'Acme', style: { fontFamily: 'Roboto', fontSize: 12 } }]
  fixture.nextId = 2
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name: 'header.folio', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(fixture)) })
  await expect(labels(page)).toHaveText(['Page 1'])
  await tools(page).getByRole('button', { name: 'Add page' }).click()
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2'])
  await tools(page).getByRole('button', { name: 'Add page' }).click()
  await expect(labels(page)).toHaveText(['Page 1', 'Page 2', 'Page 3'])
  // Page 1 current: its copy is the named one, and page 3's is an echo.
  await page.getByRole('button', { name: 'Page 1', exact: true }).click()
  const sheets = page.locator('.page-surface')
  const named = page.locator('[data-component-id="e1"]')
  const copies = page.locator('.page-band-pageHeader .canvas-component-text')
  await expect(named).toHaveCount(1)
  await expect(sheets.nth(0).locator('[data-component-id="e1"]')).toHaveCount(1)
  await expect(copies).toHaveCount(3)

  const pressEcho = async (sheet: number) => {
    const echo = sheets.nth(sheet).locator('.page-band-pageHeader .canvas-component-echo')
    await echo.scrollIntoViewIfNeeded()
    const at = await echo.boundingBox()
    if (!at) throw new Error('header echo is not visible')
    const point = { x: at.x + 5, y: at.y + 3 }
    // The echo itself is under the pointer, not the band beneath it.
    const hit = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.closest('.canvas-component')?.className ?? '', point)
    expect(hit).toContain('canvas-component-echo')
    await page.mouse.click(point.x, point.y)
  }

  // SELECT FROM PAGE 3. The named copy, its selection and focus move there.
  await pressEcho(2)
  await expect(named).toHaveCount(1)
  await expect(sheets.nth(2).locator('[data-component-id="e1"]')).toHaveCount(1)
  await expect(named).toHaveClass(/canvas-component-selected/)
  await expect(named).toBeFocused()

  // EDIT THERE. Every copy shows the new text.
  const before = (await revision(page).innerText()).trim()
  const field = page.getByRole('textbox', { name: 'Text', exact: true })
  await field.fill('Beta')
  await field.blur()
  await expect(revision(page)).not.toHaveText(before)
  await expect(copies).toHaveText([/Beta/, /Beta/, /Beta/])
  await expect(sheets.nth(2).locator('[data-component-id="e1"]')).toHaveCount(1)

  // ONE UNDO reverts every copy.
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect(copies).toHaveText([/Acme/, /Acme/, /Acme/])

  // PAGE 2's echo takes a press and becomes the named copy.
  await pressEcho(1)
  await expect(named).toHaveCount(1)
  await expect(sheets.nth(1).locator('[data-component-id="e1"]')).toHaveCount(1)
  await expect(named).toHaveClass(/canvas-component-selected/)
})
