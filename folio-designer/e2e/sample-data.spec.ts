import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
})

test('loads local sample JSON into the docked navigable discovery panel without opening a destination', async ({ page }) => {
  await openWorkspace(page)
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.getByLabel('Data panel')).toBeVisible()
  await expect(page.getByText('No sample data loaded.')).toBeVisible()
  await expect(page.getByLabel('Canvas region')).toBeVisible()
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'sample.json', mimeType: 'application/json', buffer: Buffer.from('{"customer":{"name":"Ada"},"items":[{"sku":"A-1"}]}') })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toContainText('items[]')
  await expect(page.getByRole('button', { name: 'Replace sample JSON' })).toBeVisible()
  const tree = page.getByRole('tree', { name: 'Sample data paths' })
  // The tree is a projection that may start with a synthetic root; selecting
  // the visible scalar is stable across that implementation detail.
  await tree.getByRole('treeitem').filter({ hasText: /^customer/ }).click()
  await tree.getByRole('treeitem').filter({ hasText: /^name/ }).click()
  await expect(tree.getByRole('treeitem').filter({ hasText: /^name/ })).toBeFocused()

  const replacement = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Replace sample JSON' }).click()
  await (await replacement).setFiles({ name: 'invalid.json', mimeType: 'application/json', buffer: Buffer.from('{"customer":}') })
  await expect(page.getByRole('alert')).toContainText('one valid JSON document')
  await expect(tree).toContainText('Ada')
})

// STORY 5 (spec-startup-templates), CAP-7 — THE BROWSER WITNESS FOR SAVE SAMPLE
// DATA.
//
// `beforeEach` above nulls both pickers, so this runs on the DOWNLOAD tier —
// the one jsdom cannot witness, because only a real browser can say what bytes
// actually left the tab. The loaded file and the downloaded one are compared
// buffer to buffer: a save that re-serialized the panel's bounded projection
// would come back reformatted (and, for this deliberately truncating document,
// short) under exactly the right filename.
test('saves the loaded sample back out, byte for byte, through the download tier', async ({ page }) => {
  // Wider than the panel's 50-child display limit, so the DATA tree the author
  // is looking at is a truncated view of this document rather than all of it.
  const loaded = Buffer.from(`{\n  "customer": { "name": "Ada" },\n${Array.from({ length: 60 }, (_, index) => `  "k${index}": ${index}`).join(',\n')}\n}\n`)
  await openWorkspace(page)
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.getByRole('button', { name: 'Save sample data' })).toHaveCount(0)
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'ledger.json', mimeType: 'application/json', buffer: loaded })
  await expect(page.getByText('Tree inspection is truncated to keep this local panel responsive.')).toBeVisible()

  const download = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Save sample data' }).click()
  const saved = await download
  expect(saved.suggestedFilename()).toBe('ledger.json')
  const streamed = await saved.createReadStream()
  const chunks: Buffer[] = []
  for await (const chunk of streamed) chunks.push(Buffer.from(chunk))
  expect(Buffer.concat(chunks).equals(loaded)).toBe(true)
  await expect(page.getByText('Downloaded sample data ledger.json')).toBeVisible()
  // The document is untouched by a sample save: still the unnamed starter.
  await expect(page.locator('.document-name')).toHaveText('Untitled template')
  await expect(page.getByRole('alert')).toHaveCount(0)
})
