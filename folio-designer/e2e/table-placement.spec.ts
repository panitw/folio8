import { expect, test, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'

async function revision(page: Page): Promise<number> {
  const text = await page.getByTestId('engine-snapshot').textContent()
  const match = text?.match(/REVISION (\d+)/)
  if (!match) throw new Error(`could not read engine revision from ${text}`)
  return Number(match[1])
}

test('maps a freshly placed items table to the sample transactions array from DATA', async ({ page }, testInfo) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('button', { name: 'Place Table' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).click({ position: { x: 120, y: 96 } })
  const table = page.getByRole('button', { name: /table component/ })
  await expect(table).toHaveClass(/canvas-component-selected/)
  const label = table.locator('.canvas-table-collection')
  await expect(label).toHaveText('items[]')
  const originalBox = await table.boundingBox()
  const originalColumn = await table.locator('[data-column-id]').first().getAttribute('data-column-id')
  await page.getByRole('tab', { name: 'DATA' }).click()
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'transactions.json', mimeType: 'application/json', buffer: Buffer.from('{"transactions":[{"ref":"A"}],"adjustments":[{"ref":"B"}],"refunds":[{"ref":"C"}]}') })
  const transactions = page.getByRole('treeitem').filter({ hasText: /^transactions\[\]/ })
  await transactions.click()
  await expect(label).toHaveText('transactions[]')
  expect(await table.boundingBox()).toEqual(originalBox)
  await expect(table.locator('[data-column-id]').first()).toHaveAttribute('data-column-id', originalColumn!)
  await expect(table.locator('.canvas-table-grid .canvas-table-unset')).toHaveText('Not set')
  await page.screenshot({ path: testInfo.outputPath('fresh-table-collection-binding.png'), fullPage: true })
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect(label).toHaveText('items[]')
  await page.getByRole('button', { name: 'Redo', exact: true }).click()
  await expect(label).toHaveText('transactions[]')

  // Real focus and native key events exercise the button defaults as well as
  // the tree handler. History clears selection, so first select the whole table.
  await table.focus()
  await page.keyboard.press('Enter')
  const root = page.getByRole('tree', { name: 'Sample data paths' }).getByRole('treeitem').first()
  await root.focus()
  await expect(root).toBeFocused()
  await page.keyboard.press('ArrowDown')
  await expect(transactions).toBeFocused()
  await page.keyboard.press('ArrowLeft')
  await expect(transactions).toHaveAttribute('aria-expanded', 'false')
  await page.keyboard.press('ArrowDown')
  const adjustments = page.getByRole('treeitem').filter({ hasText: /^adjustments\[\]/ })
  await expect(adjustments).toBeFocused()
  // Browse an array the table is not bound to, so an accidental bind cannot
  // hide as a history-neutral repeat of the current collection.
  const beforeBrowsing = await revision(page)
  await page.keyboard.press('ArrowRight')
  await expect(adjustments).toHaveAttribute('aria-expanded', 'true')
  await page.keyboard.press('ArrowLeft')
  await expect(adjustments).toHaveAttribute('aria-expanded', 'false')
  await expect(page.getByText('Asking the engine to bind the picked path…')).toHaveCount(0)
  expect(await revision(page)).toBe(beforeBrowsing)
  await expect(label).toHaveText('transactions[]')

  for (const [key, collection, previous] of [['Enter', 'adjustments[]', 'transactions[]'], ['Space', 'refunds[]', 'adjustments[]']] as const) {
    const before = await revision(page)
    await page.keyboard.press(key)
    await expect(label).toHaveText(collection)
    await expect.poll(() => revision(page)).toBe(before + 1)
    await expect(page.getByRole('treeitem').filter({ hasText: new RegExp(`^${collection.replace('[]', '\\[\\]')}`) })).toHaveAttribute('aria-expanded', 'true')
    await page.getByRole('button', { name: 'Undo', exact: true }).click()
    await expect(label).toHaveText(previous)
    await page.getByRole('button', { name: 'Redo', exact: true }).click()
    await expect(label).toHaveText(collection)
    if (key === 'Enter') {
      await table.focus()
      await page.keyboard.press('Enter')
      await root.focus()
      await page.keyboard.press('ArrowDown')
      await expect(transactions).toBeFocused()
      await page.keyboard.press('ArrowDown')
      await expect(adjustments).toBeFocused()
      await page.keyboard.press('ArrowLeft')
      await page.keyboard.press('ArrowDown')
      await expect(page.getByRole('treeitem').filter({ hasText: /^refunds\[\]/ })).toBeFocused()
    }
  }
})

for (const snap of [false, true]) {
  test(`a pointer-placed table fills content with one blank editable column (snap=${snap})`, async ({ page }, testInfo) => {
    await openWorkspace(page)
    await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
    if (!snap) await page.getByRole('button', { name: /^Snap on/ }).click()
    const content = page.getByRole('region', { name: 'Content', exact: true })
    const band = await content.boundingBox()
    expect(band).not.toBeNull()
    await page.getByRole('button', { name: 'Place Table' }).click()
    // A real pointer close to the right edge used to place a zero-width box
    // there, and provisional free-box snapping could also refuse this drop.
    await content.click({ position: { x: band!.width - 5, y: 79 } })
    const table = content.getByRole('button', { name: /table component/ })
    await expect(table).toBeVisible()
    await expect(table).toHaveClass(/canvas-component-selected/)
    const placed = await table.boundingBox()
    expect(placed).not.toBeNull()
    expect(placed!.x).toBeCloseTo(band!.x, 0)
    expect(placed!.width).toBeCloseTo(band!.width, 0)
    const pixelsPerPoint = band!.width / 523.276
    const intendedY = snap ? Math.round(79 / pixelsPerPoint / 6) * 6 * pixelsPerPoint : 79
    expect(placed!.y - band!.y).toBeCloseTo(intendedY, 0)
    expect(placed!.height).toBeCloseTo(24 * pixelsPerPoint, 1)
    const chip = await table.locator('.canvas-table-chip').boundingBox()
    const selection = await page.locator('.canvas-selection-chrome').boundingBox()
    expect(chip).not.toBeNull()
    expect(selection).not.toBeNull()
    expect(chip!.y).toBeCloseTo(placed!.y, 1)
    expect(chip!.height).toBeCloseTo(placed!.height, 1)
    expect(selection!.height).toBeCloseTo(chip!.height, 1)
    const tableID = await table.getAttribute('data-component-id')
    const column = table.locator('.canvas-table-heading[data-column-id]')
    await expect(column).toHaveCount(1)
    const columnID = await column.getAttribute('data-column-id')
    expect(columnID).toBeTruthy()
    expect(columnID).not.toBe(tableID)
    await page.screenshot({ path: testInfo.outputPath('table-full-width.png'), fullPage: true })

    // Creation and its column are one engine history entry.
    await page.getByRole('button', { name: 'Undo', exact: true }).click()
    await expect(table).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Undo', exact: true })).toBeDisabled()
    await page.getByRole('button', { name: 'Redo', exact: true }).click()
    await expect(table).toHaveAttribute('data-component-id', tableID!)
    await expect(column).toHaveAttribute('data-column-id', columnID!)
    await table.click()
    await page.getByRole('button', { name: 'Configure columns' }).click()
    const dialog = page.getByRole('dialog', { name: 'Table Editor' })
    // A NEWLY PLACED TABLE IS PROPORTIONALLY SIZED, which is the engine's own
    // ruling (`folio-go/table_proportions_test.go` fails a starter whose
    // `Sizing` is anything else), so the editor offers a PROPORTION per column
    // and one total width for the table — not a per-column width in points.
    // This block asserted the points vocabulary and went red when proportion
    // sizing shipped: the control it named no longer exists in this state.
    //
    // THE WIDTH CLAIM IS UNCHANGED AND IS STILL THE POINT OF THE TEST. A table
    // dropped near the right edge fills its band, so the one starter column
    // still resolves to the full 523.276, and driving the table to 120 still
    // has to reach the canvas. Only the control that gets it there is new:
    // with a single column at proportion 1 the column IS the total, so the
    // total width box is where 120 is typed.
    const proportion = dialog.getByRole('textbox', { name: 'Proportion for column 1' })
    const resolved = dialog.getByRole('status', { name: 'Resolved width for column 1 in points' })
    const total = dialog.getByRole('spinbutton', { name: 'Total table width in points' })
    const header = dialog.getByRole('textbox', { name: 'Header for column 1' })
    await expect(proportion).toHaveValue('1')
    await expect(resolved).toHaveText('523.276 pt')
    await expect(total).toHaveValue('523.276')
    await expect(header).toHaveValue('')
    await expect(dialog.getByRole('grid', { name: 'Table columns' })).toHaveAttribute('aria-rowcount', '2')
    await total.fill('120')
    await total.press('Tab')
    await expect(dialog.getByRole('status', { name: 'Width budget' })).toContainText('Σ 120.0')
    // The proportion is untouched by a total change, and the resolved width is
    // the engine's answer to both.
    await expect(proportion).toHaveValue('1')
    await expect(resolved).toHaveText('120 pt')
    await header.fill('Description')
    await header.press('Tab')
    await expect(header).toHaveValue('Description')
    await dialog.getByRole('button', { name: 'Done', exact: true }).click()
    await expect(column).toHaveText('Description')
    await expect.poll(async () => (await table.boundingBox())!.width).toBeCloseTo(120 * pixelsPerPoint, 0)
  })
}

test('the table bar and selection share the authored height at several zoom levels', async ({ page }) => {
  await openWorkspace(page)
  await page.getByRole('button', { name: 'Place Table' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).click({ position: { x: 120, y: 96 } })
  const table = page.getByRole('button', { name: /table component/ })
  await expect(table).toBeVisible()
  let zoom = 10
  for (const height of [24, 12, 36]) {
    await page.getByRole('button', { name: 'Configure columns' }).click()
    const dialog = page.getByRole('dialog', { name: 'Table Editor' })
    const headerHeight = dialog.getByRole('spinbutton', { name: 'Header height in points' })
    if (height === 24) await expect(headerHeight).toHaveValue('24')
    else {
      await headerHeight.fill(String(height))
      await headerHeight.press('Tab')
      await expect(headerHeight).toHaveValue(String(height))
    }
    await dialog.getByRole('button', { name: 'Done', exact: true }).click()
    for (const target of [10, 5, 11, 20]) {
      while (zoom !== target) {
        await page.getByRole('button', { name: zoom < target ? 'Zoom in' : 'Zoom out', exact: true }).click()
        zoom += zoom < target ? 1 : -1
      }
      await expect(page.getByLabel('Canvas zoom')).toHaveText(`${target * 10}%`)
      await table.scrollIntoViewIfNeeded()
      await expect.poll(async () => (await table.boundingBox())!.height).toBeCloseTo(height * target / 10, 1)
      const box = (await table.boundingBox())!
      const chip = (await table.locator('.canvas-table-chip').boundingBox())!
      const selection = (await page.locator('.canvas-selection-chrome').boundingBox())!
      expect(chip.y).toBeCloseTo(box.y, 1)
      expect(chip.height).toBeCloseTo(box.height, 1)
      expect(selection.y).toBeCloseTo(chip.y, 1)
      expect(selection.height).toBeCloseTo(chip.height, 1)
      for (const selector of ['.canvas-table-collection', '.canvas-table-count', '.canvas-table-chip-icon']) {
        const content = (await table.locator(selector).boundingBox())!
        expect(content.y).toBeGreaterThanOrEqual(chip.y - 0.1)
        expect(content.y + content.height).toBeLessThanOrEqual(chip.y + chip.height + 0.1)
      }
    }
  }
})
