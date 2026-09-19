import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'
import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { getDocument } from 'pdfjs-dist/legacy/build/pdf.mjs'

const dialog = (page: Page) => page.getByRole('dialog', { name: 'Table Editor' })
const table = (page: Page) => page.getByRole('button', { name: /table component/ })
const total = (page: Page) => dialog(page).getByRole('spinbutton', { name: 'Total table width in points' })
const ratio = (page: Page, n: number) => dialog(page).getByRole('textbox', { name: `Proportion for column ${n}`, exact: true })
const width = (page: Page, n: number) => dialog(page).getByLabel(`Resolved width for column ${n} in points`, { exact: true })
const binding = (page: Page, n: number) => dialog(page).getByLabel(`Binding for column ${n}`, { exact: true })

async function open(page: Page): Promise<void> {
  await table(page).focus(); await table(page).press('Enter')
  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  await page.getByRole('button', { name: 'Configure columns' }).click()
  await expect(total(page)).toBeVisible()
}
async function start(page: Page): Promise<void> {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('button', { name: 'Place Table' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).click({ position: { x: 120, y: 96 } })
  await open(page)
}
async function commit(input: Locator, value: string): Promise<void> {
  await input.fill(value); await input.press('Tab'); await expect(input).toBeEnabled()
}
async function done(page: Page): Promise<void> {
  await dialog(page).getByRole('button', { name: 'Done', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0)
}
async function download(page: Page, name: string): Promise<Buffer> {
  const downloading = page.waitForEvent('download')
  await page.getByRole('button', { name, exact: true }).click()
  const stream = await (await downloading).createReadStream()
  if (!stream) throw new Error('download has no bytes')
  const chunks: Buffer[] = []; for await (const chunk of stream) chunks.push(Buffer.from(chunk))
  return Buffer.concat(chunks)
}
async function add(page: Page, count: number): Promise<void> {
  await dialog(page).getByRole('button', { name: 'Add column', exact: true }).click()
  await expect(ratio(page, count)).toBeEnabled()
}
async function history(page: Page, action: 'Undo' | 'Redo'): Promise<void> {
  const snapshot = page.getByTestId('engine-snapshot')
  const before = await snapshot.textContent()
  await page.getByRole('button', { name: action, exact: true }).click()
  await expect(snapshot).not.toHaveText(before!)
}

// These assertions read Go's projected numbers. The browser does not allocate.
test('equal weights redistribute ten and five columns, unequal weights keep the total, and an empty table retains it', async ({ page }) => {
  await start(page)
  await expect(total(page)).toHaveValue('523.276')
  await expect(ratio(page, 1)).toHaveValue('1')
  await expect(dialog(page).getByRole('group', { name: 'Proportion sizing' })).toBeVisible()
  await commit(total(page), '500')
  for (let n = 2; n <= 10; n++) await add(page, n)
  for (let n = 1; n <= 10; n++) { await expect(ratio(page, n)).toHaveValue('1'); await expect(width(page, n)).toHaveText('50 pt') }
  for (let n = 10; n > 5; n--) { await dialog(page).getByRole('button', { name: `Remove column ${n}`, exact: true }).click(); await expect(ratio(page, n)).toHaveCount(0) }
  for (let n = 1; n <= 5; n++) await expect(width(page, n)).toHaveText('100 pt')
  for (let n = 5; n > 3; n--) { await dialog(page).getByRole('button', { name: `Remove column ${n}`, exact: true }).click(); await expect(ratio(page, n)).toHaveCount(0) }
  await expect(width(page, 1)).toHaveText('166.667 pt'); await expect(width(page, 2)).toHaveText('166.667 pt'); await expect(width(page, 3)).toHaveText('166.666 pt')
  await commit(ratio(page, 2), '2')
  for (const [n, value] of [[1, '125 pt'], [2, '250 pt'], [3, '125 pt']] as const) await expect(width(page, n)).toHaveText(value)
  await commit(total(page), '400')
  for (const [n, value] of [[1, '100 pt'], [2, '200 pt'], [3, '100 pt']] as const) await expect(width(page, n)).toHaveText(value)
  // A structural edit retains authored weights as well as the total.
  await dialog(page).getByRole('button', { name: 'Remove column 3', exact: true }).click()
  await expect(ratio(page, 2)).toHaveValue('2'); await expect(total(page)).toHaveValue('400')
  await add(page, 3)
  await expect(ratio(page, 3)).toHaveValue('1'); await expect(width(page, 2)).toHaveText('200 pt')
  for (let n = 3; n >= 1; n--) { await dialog(page).getByRole('button', { name: `Remove column ${n}`, exact: true }).click(); await expect(ratio(page, n)).toHaveCount(0) }
  await expect(total(page)).toHaveValue('400')
  await add(page, 1); await expect(width(page, 1)).toHaveText('400 pt')
  await expect(dialog(page).getByRole('alert')).toHaveCount(0)
})

test('dirty numeric Add, Done, Escape and Cancel preserve atomic history and invalid inputs restore committed controls', async ({ page }) => {
  await start(page); await commit(total(page), '500'); await done(page)
  const starter = await download(page, 'Save As')
  await open(page)
  await total(page).fill('400'); await add(page, 2)
  await expect(total(page)).toHaveValue('400'); await expect(width(page, 2)).toHaveText('200 pt')
  // Use the real current keyboard target: a locator.press would refocus a
  // stranded control and conceal the focus lost by the total's replacement.
  await expect(total(page)).toBeFocused()
  await page.keyboard.press('Escape'); await expect(dialog(page)).toHaveCount(0)
  await open(page)
  await ratio(page, 1).fill('2'); await add(page, 3)
  await expect(ratio(page, 1)).toHaveValue('2'); await expect(width(page, 1)).toHaveText('200 pt')
  await ratio(page, 2).fill('2'); await done(page)
  const authored = await download(page, 'Save As')
  await history(page, 'Undo')
  await open(page); await expect(ratio(page, 2)).toHaveValue('1'); await done(page)
  await history(page, 'Redo')
  expect(await download(page, 'Save As')).toEqual(authored)
  await open(page); await total(page).fill('500'); await total(page).press('Escape')
  await expect(dialog(page)).toHaveCount(0)
  await open(page); await expect(total(page)).toHaveValue('500'); await ratio(page, 3).fill('1.5'); await ratio(page, 3).press('Escape')
  await expect(dialog(page)).toHaveCount(0)
  const kept = await download(page, 'Save As')
  await open(page)
  const beforeRefusals = await page.getByTestId('engine-snapshot').textContent()
  for (const value of ['', '0', '-1', '1.0001', '9223372036854775.807']) {
    await ratio(page, 3).fill(value)
    await dialog(page).getByRole('button', { name: 'Add column', exact: true }).click()
    await expect(dialog(page).getByRole('alert')).toBeVisible()
    await expect(ratio(page, 3)).toHaveValue('1.5'); await expect(ratio(page, 4)).toHaveCount(0)
    expect(await page.getByTestId('engine-snapshot').textContent()).toEqual(beforeRefusals)
  }
  // Decimal text reaches Go unchanged, including malformed author drafts.
  await ratio(page, 3).fill(''); await ratio(page, 3).pressSequentially('1e')
  await ratio(page, 3).press('Escape')
  await expect(dialog(page)).toBeVisible(); await expect(ratio(page, 3)).toHaveValue('1.5')
  for (const value of ['524', '0.001', '', '400.0001']) {
    await total(page).fill(value); await dialog(page).getByRole('button', { name: 'Done', exact: true }).click()
    await expect(dialog(page)).toBeVisible(); await expect(dialog(page).getByRole('alert')).toBeVisible(); await expect(total(page)).toHaveValue('500')
    await expect(total(page)).toBeFocused()
  }
  await total(page).fill('450'); await dialog(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0); expect(await download(page, 'Save As')).toEqual(kept)
  await open(page); await commit(total(page), '450'); await ratio(page, 1).fill('3')
  await dialog(page).getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0); expect(await download(page, 'Save As')).toEqual(kept)
  // Undo all accepted authoring commands to the original table in one sequence.
  for (let n = 0; n < 7; n++) await history(page, 'Undo')
  expect(await download(page, 'Save As')).toEqual(starter)
})

test('exact extreme proportions remain editable through keyboard navigation and save/reopen', async ({ page }) => {
  await start(page)
  // Total commits must leave deliberate Tab and click destinations alone.
  await total(page).fill('400'); await total(page).press('Tab')
  await expect(total(page)).toBeEnabled()
  // The next Tab stop after the total is the table's cell padding, beside it.
  await expect(dialog(page).getByRole('textbox', { name: 'Cell padding left in points', exact: true })).toBeFocused()
  await expect(total(page)).toHaveValue('400')
  await total(page).fill('500'); await dialog(page).getByRole('textbox', { name: 'Row alias', exact: true }).click()
  await expect(total(page)).toHaveValue('500')
  await expect(dialog(page).getByRole('textbox', { name: 'Row alias', exact: true })).toBeFocused()
  for (const value of ['9007199254740.991', '9223372036854775.807']) {
    await commit(ratio(page, 1), value)
    await expect(ratio(page, 1)).toHaveValue(value)
    await ratio(page, 1).focus(); await page.keyboard.press('ArrowDown')
    await expect(ratio(page, 1)).toHaveValue(value)
    await page.keyboard.press('Escape'); await expect(dialog(page)).toHaveCount(0)
    const saved = await download(page, 'Save As')
    expect(saved.toString('utf8')).toContain(`"proportion": ${value}`)
    const snapshot = page.getByTestId('engine-snapshot')
    const beforeLoad = await snapshot.textContent()
    const choosing = page.waitForEvent('filechooser'); await page.getByRole('button', { name: 'Open local template' }).click()
    await (await choosing).setFiles({ name: 'exact-proportion.folio', mimeType: 'application/json', buffer: saved })
    await expect(page.locator('.document-name')).toHaveText('exact-proportion.folio')
    await expect(snapshot).not.toHaveText(beforeLoad!)
    await open(page); await expect(ratio(page, 1)).toHaveValue(value); await expect(width(page, 1)).toHaveText('500 pt')
  }
})

test('saved ratios, formulas and expanded footers keep aligned controls and exact canvas/browser/native PDF geometry', async ({ page }, testInfo) => {
  test.setTimeout(180000)
  await page.setViewportSize({ width: 1600, height: 1100 })
  await start(page); await commit(total(page), '500'); await add(page, 2); await add(page, 3)
  const formulas = ['{{upper(row.a)}}', '{{row.b}}', '{{formatNumber(row.c, "0.00")}}']
  for (let n = 1; n <= 3; n++) { await commit(binding(page, n), formulas[n - 1]!); await expect(binding(page, n)).toHaveValue(formulas[n - 1]!) }
  await dialog(page).getByRole('combobox', { name: 'Footer aggregate for column 3' }).selectOption('sum')
  await expect(dialog(page).getByRole('textbox', { name: 'Footer source for column 3' })).toBeVisible()
  await commit(ratio(page, 2), '2')
  for (let n = 1; n <= 3; n++) {
    const controls = [dialog(page).getByRole('textbox', { name: `Header for column ${n}`, exact: true }), binding(page, n), ratio(page, n), dialog(page).getByRole('combobox', { name: `Footer aggregate for column ${n}`, exact: true })]
    const boxes = await Promise.all(controls.map((control) => control.boundingBox()))
    for (const box of boxes) { expect(box).not.toBeNull(); expect(Math.abs(box!.y - boxes[0]!.y)).toBeLessThan(1) }
  }
  await binding(page, 2).focus(); await binding(page, 2).press('Alt+ArrowRight'); await expect(ratio(page, 2)).toBeFocused()
  await ratio(page, 2).press('ArrowDown'); await expect(ratio(page, 3)).toBeFocused()
  await dialog(page).getByRole('grid', { name: 'Table columns' }).screenshot({ path: testInfo.outputPath('aligned-proportions.png') })
  await done(page)
  const saved = await download(page, 'Save As')
  const savedDoc = JSON.parse(saved.toString('utf8'))
  const declared = savedDoc.bands.content.elements.find((element: { type: string }) => element.type === 'table')
  expect(declared.width).toBe(500); expect(declared.columns.map((column: { proportion: number }) => column.proportion)).toEqual([1, 2, 1])
  expect(declared.columns.every((column: { width?: number }) => column.width === undefined)).toBe(true)
  const boxBefore = await table(page).boundingBox()
  const choosing = page.waitForEvent('filechooser'); await page.getByRole('button', { name: 'Open local template' }).click()
  await (await choosing).setFiles({ name: 'proportions.folio', mimeType: 'application/json', buffer: saved })
  await expect(page.locator('.document-name')).toHaveText('proportions.folio')
  expect(await table(page).boundingBox()).toEqual(boxBefore)
  await open(page)
  await expect(total(page)).toHaveValue('500'); await expect(ratio(page, 2)).toHaveValue('2')
  for (let n = 1; n <= 3; n++) await expect(binding(page, n)).toHaveValue(formulas[n - 1]!)
  await expect(width(page, 1)).toHaveText('125 pt'); await expect(width(page, 2)).toHaveText('250 pt'); await expect(width(page, 3)).toHaveText('125 pt')
  await done(page); expect(await download(page, 'Save As')).toEqual(saved)
  const sample = Buffer.from('{"items":[{"a":"left","b":"middle","c":42}]}')
  await page.getByRole('tab', { name: 'DATA' }).click()
  const sampleChooser = page.waitForEvent('filechooser'); await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await sampleChooser).setFiles({ name: 'proportions.json', mimeType: 'application/json', buffer: sample })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
  await page.getByRole('button', { name: 'PREVIEW', exact: true }).click()
  await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible()
  const browserPDF = await download(page, 'Save PDF')
  const goRoot = path.resolve(import.meta.dirname, '../../folio-go')
  const binary = testInfo.outputPath('folio8'); const file = testInfo.outputPath('proportions.folio'); const data = testInfo.outputPath('proportions.json'); const output = testInfo.outputPath('native.pdf')
  writeFileSync(file, saved); writeFileSync(data, sample)
  execFileSync('go', ['build', '-o', binary, './cmd/folio8'], { cwd: goRoot, stdio: 'pipe' })
  execFileSync(binary, ['render', '-data', data, '-o', output, file], { cwd: goRoot, stdio: 'pipe' })
  expect(browserPDF).toEqual(readFileSync(output))
  const loading = getDocument({ data: new Uint8Array(browserPDF) })
  try {
    const document = await loading.promise
    const content = await (await document.getPage(1)).getTextContent()
    const texts = content.items.flatMap((item) => 'str' in item ? [item.str] : [])
    expect(texts).toContain('LEFT'); expect(texts).toContain('middle'); expect(texts).toContain('42.00')
  } finally { await loading.destroy() }
})
