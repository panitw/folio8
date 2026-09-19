import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'
import { getDocument } from 'pdfjs-dist/legacy/build/pdf.mjs'

const dialog = (page: Page) => page.getByRole('dialog', { name: 'Table Editor' })
const table = (page: Page) => page.getByRole('button', { name: /table component/ })
const binding = (page: Page, column: number) => dialog(page).getByRole('combobox', { name: `Binding for column ${column}`, exact: true })
const sample = Buffer.from('{"transactions":[{"date":"2026-09-12T00:00:00Z","trn_code":"abc-12","amount":100}],"other":[{"unrelated":"hidden"}]}')

async function loadSample(page: Page): Promise<void> {
  await page.getByRole('tab', { name: 'DATA' }).click()
  const choosing = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await choosing).setFiles({ name: 'formulas.json', mimeType: 'application/json', buffer: sample })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
}

async function start(page: Page): Promise<void> {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('button', { name: 'Place Table' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).click({ position: { x: 120, y: 96 } })
  await expect(table(page)).toHaveClass(/canvas-component-selected/)
  // The table uses a declared font so the PDF assertions exercise the formulas.
  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  const font = page.getByRole('combobox', { name: 'Font family' })
  await font.fill('Roboto')
  await page.getByRole('group', { name: 'IN THIS TEMPLATE' }).getByRole('option', { name: 'Roboto', exact: true }).click()
  await loadSample(page)
  await page.getByRole('treeitem').filter({ hasText: /^transactions\[\]/ }).click()
  await expect(table(page).locator('.canvas-table-collection')).toHaveText('transactions[]')
}

async function open(page: Page): Promise<void> {
  await table(page).focus()
  await table(page).press('Enter')
  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  await page.getByRole('button', { name: 'Configure columns' }).click()
  await expect(dialog(page)).toBeVisible()
}

async function commit(input: Locator, text: string): Promise<void> {
  await input.fill(text)
  await input.press('Tab')
  await expect(input).toBeEnabled()
}

async function done(page: Page): Promise<void> {
  await dialog(page).getByRole('button', { name: 'Done', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0)
}

async function download(page: Page, button: string): Promise<Buffer> {
  const downloading = page.waitForEvent('download')
  await page.getByRole('button', { name: button, exact: true }).click()
  const stream = await (await downloading).createReadStream()
  if (!stream) throw new Error('download has no readable bytes')
  const chunks: Buffer[] = []
  for await (const chunk of stream) chunks.push(Buffer.from(chunk))
  return Buffer.concat(chunks)
}

async function expectAligned(page: Page, column: number): Promise<void> {
  const controls = [
    dialog(page).getByRole('textbox', { name: `Header for column ${column}`, exact: true }),
    dialog(page).getByLabel(`Binding for column ${column}`, { exact: true }),
    dialog(page).getByRole('textbox', { name: `Proportion for column ${column}`, exact: true }),
    dialog(page).getByRole('combobox', { name: `Footer aggregate for column ${column}`, exact: true }),
  ]
  const boxes = await Promise.all(controls.map((control) => control.boundingBox()))
  for (const box of boxes) expect(box).not.toBeNull()
  const top = boxes[0]!.y
  for (const [index, box] of boxes.entries()) {
    expect(box!.height).toBeGreaterThan(20)
    expect(Math.abs(box!.y - top), `column ${column}, control ${index}: primary input tops align`).toBeLessThan(1)
  }
}

test('full binding inputs align, remain editable, and render formulas after save and reopen', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1600, height: 1100 })
  await start(page)
  await open(page)
  const listId = await binding(page, 1).getAttribute('list')
  expect(listId).toBeTruthy()
  const suggestions = dialog(page).locator(`datalist[id="${listId}"] option`)
  expect(await suggestions.evaluateAll((options) => options.map((option) => option.getAttribute('value')))).toEqual(['{{row.amount}}', '{{row.date}}', '{{row.trn_code}}'])
  await binding(page, 1).fill('{{row.date}}')
  await dialog(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(binding(page, 2)).toBeEnabled()
  await commit(binding(page, 2), '{{row.trn_code}}')
  await dialog(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(binding(page, 3)).toBeEnabled()
  await commit(binding(page, 3), '{{formatNumber(row.amount, "#,##0.00")}}')
  await dialog(page).getByRole('combobox', { name: 'Footer aggregate for column 3', exact: true }).selectOption('sum')
  const footerSource = dialog(page).getByRole('textbox', { name: 'Footer source for column 3', exact: true })
  await expect(footerSource).toBeEnabled()
  await commit(footerSource, 'transactions.amount')

  const formulas = ['{{formatDate(row.date, "dd/MM/yyyy")}}', '{{upper(row.trn_code)}}', '{{formatNumber(row.amount * 1.07, "#,##0.00")}}']
  for (const [index, formula] of formulas.entries()) {
    await commit(binding(page, index + 1), formula)
    await expect(binding(page, index + 1)).toHaveValue(formula)
    await expect(binding(page, index + 1)).toBeEditable()
    await expectAligned(page, index + 1)
  }
  await expect(dialog(page).locator('.matrix-bound output')).toHaveCount(0)
  await expect(dialog(page).getByRole('alert')).toHaveCount(0)
  await dialog(page).getByRole('grid', { name: 'Table columns' }).screenshot({ path: testInfo.outputPath('aligned-formula-inputs.png') })
  await done(page)
  const saved = await download(page, 'Save As')
  const choosing = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await choosing).setFiles({ name: 'formulas.folio', mimeType: 'application/json', buffer: saved })
  await expect(page.locator('.document-name')).toHaveText('formulas.folio')
  await open(page)
  for (const [index, formula] of formulas.entries()) {
    await expect(binding(page, index + 1)).toHaveValue(formula)
    await expect(binding(page, index + 1)).toBeEditable()
    await expectAligned(page, index + 1)
  }
  await done(page)
  expect(await download(page, 'Save As')).toEqual(saved)
  await loadSample(page)
  await page.getByRole('button', { name: 'PREVIEW', exact: true }).click()
  await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible()
  const pdf = await download(page, 'Save PDF')
  const loading = getDocument({ data: new Uint8Array(pdf) })
  const document = await loading.promise
  try {
    const content = await (await document.getPage(1)).getTextContent()
    const text = content.items.flatMap((item) => 'str' in item ? [item.str] : []).join(' ')
    expect(text).toContain('12/09/2026')
    expect(text).toContain('ABC-12')
    expect(text).toContain('107.00')
  } finally { await loading.destroy() }
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test('formula editing preserves caret controls and refuses invalid drafts without disturbing history', async ({ page }) => {
  await start(page)
  await open(page)
  await commit(binding(page, 1), '{{row.trn_code}}')
  await done(page)
  const simple = await download(page, 'Save As')
  await open(page)
  await binding(page, 1).fill('{{upper(row.trn_code)}}')
  await binding(page, 1).press('Home')
  await binding(page, 1).press('ArrowRight')
  await expect(binding(page, 1)).toBeFocused()
  expect(await binding(page, 1).evaluate((input: HTMLInputElement) => [input.selectionStart, input.selectionEnd])).toEqual([1, 1])
  await binding(page, 1).press(process.platform === 'darwin' ? 'Meta+ArrowRight' : 'End')
  await binding(page, 1).press('ArrowLeft')
  expect(await binding(page, 1).evaluate((input: HTMLInputElement) => input.selectionStart)).toBe('{{upper(row.trn_code)}}'.length - 1)
  await binding(page, 1).press('Escape')
  await expect(dialog(page)).toHaveCount(0)
  const formula = await download(page, 'Save As')
  expect(formula).not.toEqual(simple)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  expect(await download(page, 'Save As')).toEqual(simple)
  await page.getByRole('button', { name: 'Redo', exact: true }).click()
  expect(await download(page, 'Save As')).toEqual(formula)
  await open(page)
  await binding(page, 1).fill('{{unknown(row.trn_code)}}')
  await dialog(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(dialog(page).getByRole('alert')).toContainText('unknown function')
  await expect(binding(page, 1)).toHaveValue('{{upper(row.trn_code)}}')
  await expect(binding(page, 2)).toHaveCount(0)
  await done(page)
  expect(await download(page, 'Save As')).toEqual(formula)
  await open(page)
  await binding(page, 1).fill('{{lower(row.trn_code)}}')
  await dialog(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0)
  expect(await download(page, 'Save As')).toEqual(formula)
})

test('opening multiline binding text preserves its bytes and intentional multiline edits remain editable', async ({ page }) => {
  await start(page)
  await open(page)
  await commit(binding(page, 1), '{{row.trn_code}}')
  await done(page)
  const original = await download(page, 'Save As')
  // Prepare an existing authored file without constructing a second document model.
  const multiline = 'Heading\r\n{{row.trn_code}}'
  const source = original.toString('utf8')
  const needle = JSON.stringify('{{row.trn_code}}')
  expect(source.split(needle)).toHaveLength(2)
  const fixture = Buffer.from(source.replace(needle, JSON.stringify(multiline)))
  const choosing = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await choosing).setFiles({ name: 'multiline.folio', mimeType: 'application/json', buffer: fixture })
  await expect(page.locator('.document-name')).toHaveText('multiline.folio')
  const canonical = await download(page, 'Save As')
  const text = () => dialog(page).getByRole('textbox', { name: 'Binding for column 1', exact: true })
  await open(page)
  await expect(text()).toBeEditable()
  await expect(text()).toHaveValue('Heading\n{{row.trn_code}}')
  await expectAligned(page, 1)
  await page.setViewportSize({ width: 900, height: 800 })
  await expect(text()).toBeVisible()
  await expectAligned(page, 1)
  await text().focus()
  await text().press('Shift+Tab')
  // Shift+Tab walks back through the row's own fields before leaving the matrix.
  await expect(dialog(page).getByRole('textbox', { name: 'Header for column 1', exact: true })).toBeFocused()
  await dialog(page).getByRole('textbox', { name: 'Header for column 1', exact: true }).press('Shift+Tab')
  // Leaving the matrix backwards reaches the column headers' (i) explanation
  // buttons, which sit between the sizing/padding rows and the matrix cells.
  await expect(dialog(page).getByRole('button', { name: 'About proportion sizing', exact: true })).toBeFocused()
  await text().focus()
  await dialog(page).getByRole('textbox', { name: 'Header for column 1', exact: true }).focus()
  await done(page)
  expect(await download(page, 'Save As')).toEqual(canonical)

  await open(page)
  const revised = 'Title\n{{upper(row.trn_code)}}'
  await text().fill(revised)
  await text().press('Escape')
  await expect(dialog(page)).toHaveCount(0)
  const edited = await download(page, 'Save As')
  expect(edited).not.toEqual(canonical)
  await open(page)
  await expect(text()).toHaveValue(revised)
  await text().fill('Unwanted\n{{lower(row.trn_code)}}')
  await dialog(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0)
  expect(await download(page, 'Save As')).toEqual(edited)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  expect(await download(page, 'Save As')).toEqual(canonical)
})

test('pasting a multiline formula into a simple binding preserves the draft and Cancel discards it', async ({ page }) => {
  await start(page)
  await open(page)
  await commit(binding(page, 1), '{{row.trn_code}}')
  await done(page)
  const before = await download(page, 'Save As')
  await open(page)
  await binding(page, 1).selectText()
  const pasted = 'Code:\n{{upper(row.trn_code)}}'
  // Deliver the browser clipboard event; the handler must preserve its exact
  // text while replacing the selected range in a newly promoted textarea.
  await binding(page, 1).evaluate((input, value) => {
    const clipboardData = new DataTransfer()
    clipboardData.setData('text/plain', value)
    input.dispatchEvent(new ClipboardEvent('paste', { clipboardData, bubbles: true, cancelable: true }))
  }, pasted)
  const text = dialog(page).getByRole('textbox', { name: 'Binding for column 1', exact: true })
  await expect(text).toHaveValue(pasted)
  await expect(text).toBeFocused()
  await expectAligned(page, 1)
  await text.press('ArrowUp')
  await expect(text).toBeFocused()
  expect(await text.evaluate((input: HTMLTextAreaElement) => input.selectionStart)).toBeLessThan('Code:\n'.length)
  await text.press('Shift+ArrowDown')
  expect(await text.evaluate((input: HTMLTextAreaElement) => input.selectionStart !== input.selectionEnd)).toBe(true)
  await text.press('Escape')
  await expect(dialog(page)).toHaveCount(0)
  const committed = await download(page, 'Save As')
  expect(committed).not.toEqual(before)
  await open(page)
  await expect(text).toHaveValue(pasted)
  await text.fill('Discard this\n{{lower(row.trn_code)}}')
  await dialog(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0)
  expect(await download(page, 'Save As')).toEqual(committed)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  expect(await download(page, 'Save As')).toEqual(before)
})
