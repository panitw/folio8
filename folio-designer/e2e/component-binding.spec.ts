import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// Binding examples provide files through the fallback adapter; force it before
// the application probes browser picker capabilities.
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
})

// Compile-covered in Story 6.2. Real browser execution remains deferred to the
// Epic 6 D-000.4 boundary cadence.
test('binds a selected text component to a picked root scalar and undoes/redoes the one command', async ({ page }) => {
  await openWorkspace(page)
  const content = page.getByRole('region', { name: 'Content', exact: true })
  await page.getByRole('button', { name: 'Place Text' }).click()
  await content.press('Enter')
  const text = content.getByRole('button', { name: /text component/ }).last()
  await text.click()

  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'sample.json', mimeType: 'application/json', buffer: Buffer.from('{"customer":{"name":"Ada"}}') })
  const tree = page.getByRole('tree', { name: 'Sample data paths' })
  // Select the projected scalar itself. The previous keyboard sequence relied
  // on an old tree focus order and could leave the connect control disabled.
  await tree.getByRole('treeitem').filter({ hasText: /^customer/ }).click()
  await tree.getByRole('treeitem').filter({ hasText: /^name/ }).click()
  // STORY 14.6 — THE PICK IS THE BIND. The leaf click above dispatches
  // `bindComponentScalar` directly; the intermediate control is gone.

  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  await expect(page.getByText('Bound to').locator('..')).toContainText('customer.name')
  await expect(content.getByRole('button', { name: /bound to customer.name/ })).toBeVisible()
  await page.getByRole('button', { name: 'Undo' }).click()
  await expect(page.getByText('Bound to').locator('..')).toHaveCount(0)
  await page.getByRole('button', { name: 'Redo' }).click()
  // Undo/redo intentionally clears transient selection; reselect the restored
  // Go-projected component before asserting its committed binding.
  await content.getByRole('button', { name: /bound to customer.name/ }).click()
  await expect(page.getByText('Bound to').locator('..')).toContainText('customer.name')
})

test('offers another golden-report scalar through the tree and has no binding path textbox', async ({ page }) => {
  await openWorkspace(page)
  const content = page.getByRole('region', { name: 'Content', exact: true })
  await page.getByRole('button', { name: 'Place Text' }).click()
  await content.press('Enter')
  await content.getByRole('button', { name: /text component/ }).last().click()

  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'golden-paths.json', mimeType: 'application/json', buffer: Buffer.from('{"account":{"number":"001-9"}}') })
  const tree = page.getByRole('tree', { name: 'Sample data paths' })
  await tree.getByRole('treeitem').filter({ hasText: /^account/ }).click()
  await tree.getByRole('treeitem').filter({ hasText: /^number/ }).click()
  // STORY 14.6 — THE PICK IS THE BIND. The leaf click above dispatches
  // `bindComponentScalar` directly; the intermediate control is gone.

  await page.getByRole('tab', { name: 'PROPERTIES' }).click()
  await expect(page.getByText('Bound to').locator('..')).toContainText('account.number')
  await expect(page.getByRole('textbox', { name: /binding path|path expression/i })).toHaveCount(0)
})
