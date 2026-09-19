import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'

const editor = (page: Page) => page.getByRole('dialog', { name: 'Table Editor' })
const width = (page: Page, column: number) => editor(page).getByLabel(`Resolved width for column ${column} in points`)
const field = (page: Page, column: number) => editor(page).getByRole('combobox', { name: `Binding for column ${column}`, exact: true })
const binding = (page: Page, column: number) => editor(page).getByRole('combobox', { name: `Binding for column ${column}`, exact: true })
const table = (page: Page) => page.getByRole('button', { name: /table component/ })
const sample = Buffer.from('{"transactions":[{"date":"2026-09-12","amount":42,"customer":{"name":"Alice"}}],"other":[{"unrelated":"hidden"}]}')

async function loadSample(page: Page): Promise<void> {
  await page.getByRole('tab', { name: 'DATA' }).click()
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'transactions.json', mimeType: 'application/json', buffer: sample })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
}

async function startTable(page: Page): Promise<void> {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('button', { name: 'Place Table' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).click({ position: { x: 120, y: 96 } })
  await expect(table(page)).toHaveClass(/canvas-component-selected/)
}

async function openEditor(page: Page): Promise<void> {
  await table(page).focus()
  await table(page).press('Enter')
  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  await page.getByRole('button', { name: 'Configure columns' }).click()
  await expect(editor(page)).toBeVisible()
}

async function commit(input: Locator, value: string): Promise<void> {
  await input.fill(value)
  await input.press('Tab')
}

async function done(page: Page): Promise<void> {
  await editor(page).getByRole('button', { name: 'Done', exact: true }).click()
  await expect(editor(page)).toHaveCount(0)
}

async function savedBytes(page: Page): Promise<Buffer> {
  const downloading = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Save As', exact: true }).click()
  const download = await downloading
  const stream = await download.createReadStream()
  if (!stream) throw new Error('saved template has no download stream')
  const chunks: Buffer[] = []
  for await (const chunk of stream) chunks.push(Buffer.from(chunk))
  return Buffer.concat(chunks)
}

test('authors three bound columns from a full-width starter and saves/reopens their exact engine state', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1600, height: 1000 })
  await startTable(page)
  await loadSample(page)
  await page.getByRole('treeitem').filter({ hasText: /^transactions\[\]/ }).click()
  await expect(table(page).locator('.canvas-table-collection')).toHaveText('transactions[]')
  await openEditor(page)
  await expect(width(page, 1)).toHaveText('523.276 pt')
  await expect(field(page, 1)).toBeVisible()
  const listId = await field(page, 1).getAttribute('list')
  expect(listId).toBeTruthy()
  const suggestions = editor(page).locator(`datalist[id="${listId}"] option`)
  await expect(suggestions).toHaveCount(3)
  expect(await suggestions.evaluateAll((options) => options.map((option) => option.getAttribute('value')))).toEqual(['{{row.amount}}', '{{row.customer.name}}', '{{row.date}}'])

  await commit(editor(page).getByRole('textbox', { name: 'Row alias', exact: true }), 'txn')
  await expect(editor(page).getByRole('textbox', { name: 'Row alias', exact: true })).toBeEnabled()
  // Clicking Add directly from a dirty field must save it and still add.
  await field(page, 1).fill('{{txn.date}}')
  await editor(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(binding(page, 1)).toHaveValue('{{txn.date}}')
  await expect(width(page, 1)).toHaveText('261.638 pt')
  await expect(width(page, 2)).toHaveText('261.638 pt')
  await commit(editor(page).getByRole('textbox', { name: 'Header for column 1', exact: true }), 'Date')
  await expect(field(page, 2)).toBeEnabled()
  await commit(field(page, 2), '{{txn.amount}}')
  await expect(binding(page, 2)).toHaveValue('{{txn.amount}}')
  await editor(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(width(page, 1)).toHaveText('174.426 pt')
  await expect(width(page, 2)).toHaveText('174.425 pt')
  await expect(width(page, 3)).toHaveText('174.425 pt')
  await commit(field(page, 3), '{{txn.customer.name}}')
  await expect(binding(page, 3)).toHaveValue('{{txn.customer.name}}')
  await expect(editor(page).getByRole('status', { name: 'Width budget' })).toContainText('Σ 523.3 of 523.3 available')
  await expect(editor(page).getByRole('alert')).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('bound-table-columns.png'), fullPage: true })
  await done(page)
  const originalBox = await table(page).boundingBox()
  const before = await savedBytes(page)
  const reopen = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await reopen).setFiles({ name: 'authored-table.folio', mimeType: 'application/json', buffer: before })
  await expect(page.locator('.document-name')).toHaveText('authored-table.folio')
  expect(await table(page).boundingBox()).toEqual(originalBox)
  await openEditor(page)
  for (const [column, value, size] of [[1, 'date', '174.426'], [2, 'amount', '174.425'], [3, 'customer.name', '174.425']] as const) {
    await expect(field(page, column)).toHaveValue(`{{txn.${value}}}`)
    await expect(binding(page, column)).toHaveValue(`{{txn.${value}}}`)
    await expect(width(page, column)).toHaveText(`${size} pt`)
  }
  await done(page)
  expect(await savedBytes(page)).toEqual(before)

  // The same column can still be rebound from DATA after authoring here.
  await loadSample(page)
  await table(page).locator('.canvas-table-heading').first().click()
  const tree = page.getByRole('tree', { name: 'Sample data paths' })
  await tree.getByRole('treeitem').filter({ hasText: /^transactions\[\]/ }).click()
  await tree.getByRole('treeitem').filter({ hasText: /^item 1/ }).click()
  await tree.getByRole('treeitem').filter({ hasText: /^amount/ }).click()
  await expect(table(page).locator('.canvas-table-grid .canvas-table-cell').first()).toHaveText('{{txn.amount}}')
  await openEditor(page)
  await expect(field(page, 1)).toHaveValue('{{txn.amount}}')
  await expect(binding(page, 1)).toHaveValue('{{txn.amount}}')
})

test('Add resizes atomically through undo/redo and Cancel restores entered, cleared and rejected fields without a sample', async ({ page }) => {
  await startTable(page)
  const starter = await savedBytes(page)
  await openEditor(page)
  await editor(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(width(page, 2)).toHaveText('261.638 pt')
  await done(page)
  const split = await savedBytes(page)
  expect(split).not.toEqual(starter)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  expect(await savedBytes(page)).toEqual(starter)
  await page.getByRole('button', { name: 'Redo', exact: true }).click()
  expect(await savedBytes(page)).toEqual(split)
  await openEditor(page)
  await commit(field(page, 1), '{{row.date}}')
  await expect(binding(page, 1)).toHaveValue('{{row.date}}')
  await field(page, 1).fill('{{row.}}')
  await editor(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(editor(page).getByRole('alert')).toContainText("expected identifier after '.', got end of expression")
  await expect(binding(page, 1)).toHaveValue('{{row.date}}')
  await expect(field(page, 1)).toHaveValue('{{row.date}}')
  await expect(width(page, 3)).toHaveCount(0)
  await commit(field(page, 1), '')
  await expect(binding(page, 1)).toHaveValue('')
  await expect(field(page, 1)).toHaveValue('')
  // An authored total stays fixed while new equal columns redistribute it.
  await commit(editor(page).getByRole('spinbutton', { name: 'Total table width in points' }), '500')
  await expect(editor(page).getByRole('status', { name: 'Width budget' })).toContainText('Σ 500.0')
  await editor(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(width(page, 1)).toHaveText('166.667 pt')
  await expect(width(page, 2)).toHaveText('166.667 pt')
  await expect(width(page, 3)).toHaveText('166.666 pt')
  await field(page, 3).fill('draft.only')
  await editor(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(editor(page)).toHaveCount(0)
  expect(await savedBytes(page)).toEqual(split)
})

test('Escape saves the pending field while Cancel discards it and invalid fields stay open', async ({ page }) => {
  await startTable(page)
  const starter = await savedBytes(page)
  await openEditor(page)
  await field(page, 1).fill('{{row.date}}')
  await field(page, 1).press('Escape')
  await expect(editor(page)).toHaveCount(0)
  await expect(table(page).locator('.canvas-table-grid .canvas-table-cell').first()).toHaveText('{{row.date}}')
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  expect(await savedBytes(page)).toEqual(starter)

  await openEditor(page)
  await field(page, 1).fill('unsaved.path')
  await editor(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(editor(page)).toHaveCount(0)
  expect(await savedBytes(page)).toEqual(starter)

  await openEditor(page)
  await field(page, 1).fill('{{row.}}')
  await field(page, 1).press('Escape')
  await expect(editor(page)).toBeVisible()
  await expect(editor(page).getByRole('alert')).toContainText("expected identifier after '.', got end of expression")
  await expect(field(page, 1)).toHaveValue('')
  await editor(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(editor(page)).toHaveCount(0)
  expect(await savedBytes(page)).toEqual(starter)
})
