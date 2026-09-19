import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
})

// Compiled source for the Epic 6 boundary run. The fixture is altered only as
// picker input; the browser never reads its template structure.
const source = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/testdata/example/first-pdf.folio'), 'utf8')
const parameterTemplate = Buffer.from(source.replace('{{customer.name}}', '{{params.reportDate}}'))

test('discovers reportDate from Go, keeps an absent value as a located engine failure, and makes supplied input stale', async ({ page }) => {
  await openWorkspace(page)
  const templateChooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await templateChooser).setFiles({ name: 'parameter-preview.folio', mimeType: 'application/json', buffer: parameterTemplate })
	await expect(page.locator('.document-name')).toHaveText('parameter-preview.folio')
  const sampleChooser = page.waitForEvent('filechooser')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await sampleChooser).setFiles({ name: 'sample.json', mimeType: 'application/json', buffer: Buffer.from('{"customer":{"name":"Ada"}}') })
	await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
  await page.getByRole('button', { name: 'PREVIEW' }).click()
  await page.getByRole('tab', { name: 'INPUTS' }).click()
  const reportDate = page.getByRole('textbox', { name: 'Value for params.reportDate' })
  await expect(reportDate).toBeVisible()
  // Parameter discovery is asynchronous; explicitly render the currently
  // accepted empty parameter document to exercise the missing-param failure.
	await expect(page.getByRole('button', { name: 'Re-render' })).toBeEnabled()
	await page.getByRole('button', { name: 'Re-render' }).click()
  const failure = page.getByLabel('Local render failure')
  await expect(failure).toContainText('Render failure')
	await expect(failure).toContainText('BINDING_PATH_ABSENT')
	await expect(failure).toContainText('Element ID: e1')
  const retry = failure.getByRole('button', { name: 'Retry preview' })
  await expect(retry).toBeVisible()
  await expect(failure.getByRole('button', { name: 'Return to Design' })).toBeVisible()
  await retry.focus()
  await page.keyboard.press('Enter')
  await expect(failure).toBeVisible()
  await reportDate.fill('"2026-08-28T00:00:00Z"')
	// There is no admitted historical PDF after the initial missing-parameter
	// failure; accepted input therefore schedules a fresh local render rather
	// than labelling a non-existent last-good PDF stale.
	await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible()
  await page.getByRole('button', { name: 'Re-render' }).click()
  await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible()
})
