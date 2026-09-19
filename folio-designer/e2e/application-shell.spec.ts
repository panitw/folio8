import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// These scenarios exercise the fallback input/download adapter. Chromium ships
// the File System Access API, so select the adapter explicitly before the app
// captures capabilities rather than waiting for a filechooser that cannot fire.
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
})

test('the initial shell exposes desktop landmarks and honest local-file controls', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByLabel('Document bar')).toBeVisible()
  await expect(page.getByLabel('Component palette')).toBeVisible()
  await expect(page.getByLabel('Canvas region')).toBeVisible()
  await expect(page.getByLabel('Properties panel')).toBeVisible()
  await expect(page.getByRole('tab', { name: 'PROPERTIES' })).toHaveAttribute('aria-selected', 'true')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.getByLabel('Data panel')).toBeVisible()
  await page.getByRole('tab', { name: 'PROPERTIES' }).click()

  await expect(page.getByRole('button', { name: 'Open local template' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save local template' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save As' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'New…' })).toBeVisible()
  await expect(page.getByText('Unsaved local changes')).toBeVisible()
  await expect(page.getByRole('button', { name: 'PREVIEW' })).toBeVisible()
})

test('Preview renders local identity evidence and marks an edited last-good PDF stale', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'preview.json', mimeType: 'application/json', buffer: Buffer.from('{"customer":{"name":"Preview customer"},"transactions":[]}') })
  await page.getByRole('button', { name: 'PREVIEW' }).click()
  await page.getByRole('tab', { name: 'INPUTS' }).click()
  await expect(page.getByLabel('Preview region')).toBeVisible()
  await expect(page.getByLabel('Canvas region')).toHaveCount(0)
  await expect(page.getByRole('textbox', { name: 'Raw parameter JSON' })).toBeVisible()
  // STORY 13.5 — THE EXISTENCE CONTRACT RETIRED, IN THE OTHER DIRECTION. This
  // line used to assert the preview heading's own Return-to-Design button was
  // visible. That was a SECOND mode control beside the document bar's switch,
  // where the design draws one, and it is gone; asserting its ABSENCE is what
  // keeps the removal from being quietly undone. `count(0)` is over the whole
  // page, so it also witnesses that no failure card is up — the failure card's
  // identically-named button is a different control and stays.
  await expect(page.getByRole('button', { name: /return to design/i })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'DESIGN' })).toBeVisible()
  await expect(page.locator('#preview-freshness-status')).toHaveText('Current exact local PDF')
  await expect(page.locator('#preview-freshness-status')).toHaveClass('sr-only')
  await expect(page.getByText('EXACT LOCAL PRODUCTION PDF')).toHaveCount(0)
  await page.getByRole('textbox', { name: 'Raw parameter JSON' }).fill('{"preview":"changed"}')
  await expect(page.locator('#preview-freshness-status')).toContainText('STALE — inputs changed')
  await expect(page.getByRole('region', { name: /Stale historical PDF|Current exact local production PDF/ })).toBeVisible()
  // Driven from the surviving control: the mode switch calls the same
  // `returnToDesign` the removed button called.
  await page.getByRole('button', { name: 'DESIGN' }).click()
  await expect(page.getByLabel('Canvas region')).toBeVisible()
})

test('the real worker projects page bands and accepts local page setup without changing transient controls', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const report = page.getByLabel('Report page with Page Header, Content, and Page Footer')
  await expect(report).toBeVisible()
  await expect(page.getByRole('region', { name: 'Page Header', exact: true })).toBeVisible()
  await expect(page.getByRole('region', { name: 'Content', exact: true })).toBeVisible()
  await expect(page.getByRole('region', { name: 'Page Footer', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Zoom in' }).click()
  await expect(page.getByLabel('Canvas zoom')).toHaveText('110%')
  await page.getByRole('button', { name: 'Grid on' }).click()
  await expect(page.getByRole('button', { name: 'Grid off' })).toHaveAttribute('aria-pressed', 'false')
  await page.getByRole('button', { name: 'Snap on' }).click()
  await expect(page.getByRole('button', { name: 'Snap off' })).toHaveAttribute('aria-pressed', 'false')
  await page.getByRole('textbox', { name: 'Top margin (pt)' }).fill('37')
  await page.getByRole('button', { name: 'Apply page setup' }).click()
  await expect(page.getByText('Unsaved local changes')).toBeVisible()
})

test('canvas keeps a keyboard-visible scroll target at a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 700 })
  await openWorkspace(page)
  const canvas = page.getByLabel('Canvas region')
  await expect(canvas).toBeVisible()
  await canvas.focus()
  await expect(canvas).toBeFocused()
  await expect(page.getByRole('button', { name: 'Zoom in' })).toBeVisible()
})
