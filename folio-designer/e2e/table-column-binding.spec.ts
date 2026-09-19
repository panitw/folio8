import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// STORY 14.10 — THE ONLY INSTRUMENT THAT CAN OBSERVE THIS STORY'S CENTRAL
// MECHANISM.
//
// AC1's acceptance is *"the author clicks a column"*, and jsdom DOES NOT
// IMPLEMENT `pointer-events` HIT TESTING: `fireEvent.click(span)` makes the span
// the event target whatever App.css says, while a real browser removes the whole
// `pointer-events: none` subtree from hit testing and resolves the click to the
// wrapper. A DOM-targeting implementation therefore passes every unit test in
// the repository while being COMPLETELY INERT in the product if the CSS is
// omitted or later removed. The source-text pin in
// `src/table-column-binding.test.tsx` is the second cover, and it is a proxy:
// it proves the rule is in the stylesheet, not that the click resolves to the
// column. This file is the first.
//
// ⚠ THE NEGATIVE CONTROL AT THE FOOT IS PART OF THE CLAIM, NOT DECORATION. It
// puts `pointer-events: none` back on the same spans, through a stylesheet the
// page itself carries, and shows the click falling through to the table with NO
// column selected. Without it "the click resolved to the column" is a sentence
// this file could keep printing after the declaration was deleted.

const fixtures = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../fixtures/statement-1')
// READ-ONLY, and proven to meet this spec's precondition: element `e8` is a
// table bound to `transactions[]` with alias `txn` and five columns, every one
// of them bound. `data.json` is that collection's own sample.
const template = readFileSync(path.join(fixtures, 'input.folio'))
const sample = readFileSync(path.join(fixtures, 'data.json'))

// The Note column, `ec`, and the field this spec re-points it at. `Note` is
// chosen because its sample values are the only ones that differ per row, so a
// mis-binding is visible rather than plausible.
const NOTE_COLUMN = 'ec'

async function openFixture(page: Page, sampleBytes = sample): Promise<void> {
  // THE FALLBACK FILE TIER, FORCED. Headless Chromium HAS the File System
  // Access API, so the app would call `showOpenFilePicker()` directly and emit
  // no `filechooser` event at all. Every passing spec that waits on one forces
  // this tier first (`local-file-actions.spec.ts:11` is the model).
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const templateChooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await templateChooser).setFiles({ name: 'statement.folio', mimeType: 'application/json', buffer: template })
  await expect(page.locator('.document-name')).toHaveText('statement.folio')
  const sampleChooser = page.waitForEvent('filechooser')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await sampleChooser).setFiles({ name: 'statement-data.json', mimeType: 'application/json', buffer: sampleBytes })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
}

// A REAL POINTER AT THE PAINTED SPAN'S CENTRE, never `locator.click()`.
// Playwright's own click refuses an element that does not receive pointer
// events, which would make the negative control below fail for the wrong
// reason; a mouse click at a coordinate is what a person does and lets the
// browser's own hit testing answer.
async function clickCentre(page: Page, target: Locator): Promise<void> {
  const box = await target.boundingBox()
  if (!box) throw new Error('the column span painted no box to click')
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2)
}

const heading = (page: Page, columnId: string) => page.locator(`.canvas-component:not(.canvas-component-echo) .canvas-table-heading[data-column-id="${columnId}"]`)
const bodyCell = (page: Page, columnId: string) => page.locator(`.canvas-component:not(.canvas-component-echo) .canvas-table-grid .canvas-table-cell[data-column-id="${columnId}"]`)
// The `Not set` cell of a column with no binding. It is a DIFFERENT class from
// the bound cell above and a separate compound in the shipped rule, so a click
// on one proves nothing about the other.
const unsetCell = (page: Page) => page.locator('.canvas-component:not(.canvas-component-echo) .canvas-table-grid .canvas-table-unset[data-column-id]')
const markedColumns = (page: Page) => page.locator('.canvas-component:not(.canvas-component-echo) .canvas-table-column-selected')

test('binds a whole table collection, preserves its columns, and binds a field from the new collection after one-step undo and redo', async ({ page }, testInfo) => {
  test.setTimeout(180_000)
  await openFixture(page, Buffer.from('{"transactions":[{"oldField":"old"}],"report":{"entries":[{"code":"next","amount":42}]}}'))
  const table = page.getByRole('button', { name: /table component e8/ })
  const collectionLabel = table.locator('.canvas-table-collection')
  await expect(collectionLabel).toHaveText('transactions[]')
  const originalBindings = await table.locator('.canvas-table-grid .canvas-table-cell').allTextContents()
  const originalBox = await table.boundingBox()
  expect(originalBox).not.toBeNull()
  await clickCentre(page, table.locator('.canvas-table-chip'))
  await expect(markedColumns(page)).toHaveCount(0)
  await expect(page.locator('.data-context')).toHaveText('Table selected · pick a root collection to bind its rows.')
  const tree = page.getByRole('tree', { name: 'Sample data paths' })
  await tree.getByRole('treeitem').filter({ hasText: /^report/ }).click()
  const entries = tree.getByRole('treeitem').filter({ hasText: /^entries\[\]/ })
  const before = await revision(page)
  await entries.click()
  await expect(collectionLabel).toHaveText('report.entries[]')
  await expect.poll(() => revision(page)).toBe(before + 1)
  await expect(page.locator('.binding-status')).toContainText('Current engine binding: report.entries[]')
  await expect(entries).toHaveAttribute('aria-expanded', 'true')
  expect(await table.locator('.canvas-table-grid .canvas-table-cell').allTextContents()).toEqual(originalBindings)
  expect(await table.boundingBox()).toEqual(originalBox)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect(collectionLabel).toHaveText('transactions[]')
  await page.getByRole('button', { name: 'Redo', exact: true }).click()
  await expect(collectionLabel).toHaveText('report.entries[]')
  expect(await table.locator('.canvas-table-grid .canvas-table-cell').allTextContents()).toEqual(originalBindings)

  await clickCentre(page, heading(page, NOTE_COLUMN))
  await expect(page.locator('.data-context')).toHaveText('Column Note selected · binding to a row field of report.entries[]')
  await tree.getByRole('treeitem').filter({ hasText: /^transactions\[\]/ }).click()
  await tree.getByRole('treeitem').filter({ hasText: /^item 1/ }).first().click()
  const oldField = tree.getByRole('treeitem').filter({ hasText: /^oldField/ })
  await expect(oldField).toHaveAttribute('aria-disabled', 'true')
  await tree.getByRole('treeitem').filter({ hasText: /^item 1/ }).last().click()
  const code = tree.getByRole('treeitem').filter({ hasText: /^code/ })
  await expect(code).not.toHaveAttribute('aria-disabled', 'true')
  await code.click()
  await expect(bodyCell(page, NOTE_COLUMN)).toHaveText('{{txn.code}}')
  await expect(collectionLabel).toHaveText('report.entries[]')
  await page.screenshot({ path: testInfo.outputPath('collection-and-column-binding.png'), fullPage: true })
})

test('binds a table column from the main window: click the column, pick its row field', async ({ page }) => {
  test.setTimeout(180_000)
  await openFixture(page)

  const table = page.getByRole('button', { name: /table component e8/ })
  await expect(table).toBeVisible()
  await expect(heading(page, NOTE_COLUMN)).toHaveText('Note')

  // AC1 — THE CLICK RESOLVES TO THE COLUMN. This is the assertion the whole
  // file exists for and the one the negative control below turns red.
  await clickCentre(page, heading(page, NOTE_COLUMN))
  await expect(markedColumns(page)).toHaveCount(2)
  await expect(markedColumns(page).first()).toHaveAttribute('data-column-id', NOTE_COLUMN)
  // The owning table remains the COMPONENT selection…
  await expect(table).toHaveClass(/canvas-component-selected/)
  // …the inspector acknowledges the column and offers no control for it…
  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  await expect(page.locator('.column-identity')).toContainText('Note')
  await expect(page.locator('.column-identity').getByRole('button')).toHaveCount(0)
  // …and "Configure columns" stays live throughout.
  await expect(page.getByRole('button', { name: 'Configure columns' })).toBeEnabled()

  // AC2 — THE DATA PANEL OFFERS THAT COLUMN'S ROW-SCOPE FIELDS AND NAMES IT.
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.locator('.data-context')).toHaveText('Column Note selected · binding to a row field of transactions[]')
  const tree = page.getByRole('tree', { name: 'Sample data paths' })
  const collection = tree.getByRole('treeitem').filter({ hasText: /^transactions\[\]/ })
  // AC4 — THE COLLECTION ITSELF IS REFUSED, WITH THE REASON STATED BEFORE THE
  // ENGINE WOULD REFUSE IT.
  await expect(collection).toContainText('Collection · a column binds one row field of transactions[], never a collection.')
  await collection.click()
  const item = tree.getByRole('treeitem').filter({ hasText: /^item 1/ })
  await item.click()
  const field = tree.getByRole('treeitem').filter({ hasText: /^ref/ })
  await expect(field).toBeVisible()
  await expect(field).not.toHaveAttribute('aria-disabled', 'true')

  // AC3 — THE PICK IS THE BIND, and one command reaches the engine.
  const before = await revision(page)
  await field.click()
  await expect.poll(() => revision(page), { timeout: 30_000 }).toBeGreaterThan(before)
  // THE CANVAS REPAINTS THE ENGINE'S OWN ANSWER — `{{txn.ref}}`, with the alias
  // the DOCUMENT declares and the browser never sent. The command carries the
  // bare row-relative field; Go resolves `txn` itself.
  await expect(page.locator(`.canvas-component:not(.canvas-component-echo) .canvas-table-cell[data-column-id="${NOTE_COLUMN}"]`)).toHaveText('{{txn.ref}}')

  // AC6 — ONE UNDO STEP. Asserted, not built: `folio-go/internal/wasm/engine.go`'s `Apply` pushes
  // exactly one undo per accepted byte-changing command, so a single Undo must
  // put the previous binding back.
  await page.getByRole('button', { name: 'Undo' }).click()
  await expect(page.locator(`.canvas-component:not(.canvas-component-echo) .canvas-table-cell[data-column-id="${NOTE_COLUMN}"]`)).toHaveText('{{txn.note}}')
})

// THE BODY CELLS, IN A REAL BROWSER, BECAUSE THE HEADING PROVES NOTHING ABOUT
// THEM.
//
// The shipped rule is three compounds — `.canvas-table-heading`,
// `.canvas-table-grid .canvas-table-cell` and
// `.canvas-table-grid .canvas-table-unset` — and until this test only the first
// had ever been clicked by a browser. The other two were exercised solely in
// jsdom, which cannot observe `pointer-events` at all, so dropping either of
// them from App.css left the whole repository green while the primary gesture
// for an unbound column — *"click the column that reads Not set"* — was inert.
//
// THE `Not set` CELL IS MANUFACTURED IN-RUN, NOT IN THE FIXTURE. `input.folio`
// binds all five of `e8`'s columns and is read-only here, so a sixth column is
// ADDED through the product's own Add column control, which is what leaves an
// unbound column behind. Its width is the engine's default 72pt against a
// 523pt content band already holding 414pt of columns, so the add fits and the
// new cell paints.
test('resolves the column from a body cell and from an unbound column s Not set cell', async ({ page }) => {
  test.setTimeout(180_000)
  await openFixture(page)
  const table = page.getByRole('button', { name: /table component e8/ })
  await expect(table).toBeVisible()

  // (1) A BOUND BODY CELL resolves the same column its heading does.
  await expect(bodyCell(page, NOTE_COLUMN)).toHaveText('{{txn.note}}')
  await clickCentre(page, bodyCell(page, NOTE_COLUMN))
  await expect(markedColumns(page)).toHaveCount(2)
  await expect(markedColumns(page).first()).toHaveAttribute('data-column-id', NOTE_COLUMN)
  await expect(table).toHaveClass(/canvas-component-selected/)

  // (2) AN UNBOUND COLUMN'S `Not set` CELL — the class the author actually
  // aims at when a column has nothing bound yet.
  await expect(unsetCell(page)).toHaveCount(0)
  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  await page.getByRole('button', { name: 'Configure columns' }).click()
  await expect(page.getByRole('dialog', { name: 'Table Editor' })).toBeVisible()
  await page.getByRole('button', { name: 'Add column' }).click()
  await expect(page.getByRole('textbox', { name: 'Header for column 6' })).toBeVisible()
  // Closed with `Done` rather than Escape: Escape is handled as
  // `onKeyDownCapture` ON THE DIALOG, so it only closes while focus is inside
  // it, and after an Add the focus lands wherever `focusCell` put it. `Done`
  // and Escape are the same act here (both close and KEEP), and `Done` is the
  // one that does not depend on where focus happens to be.
  await page.getByRole('dialog', { name: 'Table Editor' }).getByRole('button', { name: 'Done' }).click()
  await expect(page.getByRole('dialog', { name: 'Table Editor' })).toHaveCount(0)
  await expect(unsetCell(page)).toHaveCount(1)
  const added = await unsetCell(page).getAttribute('data-column-id')
  expect(added).toBeTruthy()
  expect(added).not.toBe(NOTE_COLUMN)
  await clickCentre(page, unsetCell(page))
  await expect(markedColumns(page)).toHaveCount(2)
  await expect(markedColumns(page).first()).toHaveAttribute('data-column-id', added as string)
  // AND THE PANEL FOLLOWS THE COLUMN THE CLICK RESOLVED, so this is the whole
  // gesture rather than a class landing on a span.
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.locator('.data-context')).toHaveText('Column Column 6 selected · binding to a row field of transactions[]')
})

// THE NEGATIVE CONTROL FOR `pointer-events: auto`.
//
// The declaration is put back to `none` on exactly the spans App.css restores
// it on. If the assertion in the test above did not depend on real hit testing,
// this would still find a marked column — and it must not.
test('resolves no column when hit testing is taken off the column spans', async ({ page }) => {
  test.setTimeout(180_000)
  await openFixture(page)
  await page.addStyleTag({ content: '.canvas-table-heading, .canvas-table-grid .canvas-table-cell, .canvas-table-grid .canvas-table-unset { pointer-events: none !important; }' })
  const table = page.getByRole('button', { name: /table component e8/ })
  await expect(table).toBeVisible()
  await table.click()
  await expect(table).toHaveClass(/canvas-component-selected/)
  await clickCentre(page, heading(page, NOTE_COLUMN))
  // The grid begins below the table's header box. With its hit targets
  // disabled, the click reaches the canvas and clears the prior selection.
  await expect(table).not.toHaveClass(/canvas-component-selected/)
  await expect(markedColumns(page)).toHaveCount(0)
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.locator('.data-context')).toHaveText('No single component selected · select one component, then pick a path.')
})

async function revision(page: Page): Promise<number> {
  const text = await page.getByTestId('engine-snapshot').textContent()
  const found = text?.match(/REVISION (\d+)/)
  if (!found) throw new Error(`could not read engine revision from ${text}`)
  return Number(found[1])
}
