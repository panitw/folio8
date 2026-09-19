import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// STORY 13.4 — THE BROWSER WITNESS FOR A PREVIEW WITH NO SAMPLE DATA.
//
// COMPILE-CHECKED LOCALLY, EXECUTED IN CI. The story's heavy-test cadence runs
// unit + lint + typecheck on the authoring machine; the Playwright suite builds
// the release, which that cadence forbids locally. CI runs the whole browser
// suite on every push (DW-268, discharged at `adf905a`), so this test executes
// per-commit and is written as real coverage — never as a compile-only
// placeholder.
//
// It is the one witness jsdom cannot give: the REAL Go engine generating the
// stand-in document, the real wasm render, and the real PDF.js rasterizer
// admitting bytes produced from values no author supplied.
const template = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/testdata/example/first-pdf.folio'))

test('previews a bound template with no sample data, and claims nothing about production', async ({ page }) => {
  // THE FALLBACK FILE TIER, FORCED (repaired 2026-09-07, Story 13.2's gate).
  // Headless Chromium HAS the File System Access API, so without this the app
  // takes the native tier, calls `showOpenFilePicker()` and emits no
  // `filechooser` event at all — the wait below timed out at 90s. This spec had
  // never been executed: it was written under the belief that CI ran the
  // Playwright suite per push, and the first real run failed it. Eight of the
  // ten `filechooser` specs already did this; this was one of the two that did
  // not. See `local-file-actions.spec.ts:11` for the model.
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const templateChooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await templateChooser).setFiles({ name: 'statement.folio', mimeType: 'application/json', buffer: template })
  await expect(page.locator('.document-name')).toHaveText('statement.folio')

  // STRAIGHT TO PREVIEW. No JSON is loaded, and the template's one text element
  // binds {{customer.name}} — the shape that used to refuse three ways over.
  await page.getByRole('button', { name: 'PREVIEW' }).click()

  // A REAL PDF ARRIVES, AND PDF.js ADMITS IT. Waiting for the ADMITTED name is
  // the whole assertion: the viewer carries `Stale historical PDF` the moment
  // bytes are handed to it, and only admission promotes it to `Current`, since
  // App.tsx reaches `'current'` nowhere except the viewer's own onPageCount.
  //
  // DW-296, fixed 2026-09-08 with evidence rather than reasoning. A second wait
  // on `Stale historical PDF` used to sit above this line. It passed on the
  // authoring machine and spent the full 60s failing on ubuntu-24.04 — the only
  // red in the workflow across six runs. The page snapshot from run
  // `34177610962` settles why, and the product was never at fault:
  //
  //     - main "Preview region":
  //       - status: Current no-data layout PDF
  //       - region "Current no-data layout PDF, revision 2":
  //
  // Already admitted, already at revision 2. The removed line was not asserting
  // a fact about the product; it was asserting that a poll would land inside the
  // window between hand-off and admission, which is a property of how fast the
  // machine is. It also added no separating power — `Current` is unreachable
  // EXCEPT through the pre-admission state, so the line below implies it. If
  // that transient label is ever worth guarding, it belongs in a unit test that
  // controls the clock, not a browser test that races it.
  await expect(page.getByRole('region', { name: /Current no-data layout PDF, revision \d+/ })).toBeVisible({ timeout: 60_000 })
  await expect(page.getByRole('note', { name: 'No-data preview notice' })).toBeVisible()
  await expect(page.getByText('NO-DATA LAYOUT PREVIEW')).toHaveCount(0)
  await expect(page.locator('#preview-freshness-status')).toHaveClass('sr-only')
  // STORY 13.3 — THE DIGEST IS IN THE RAIL NOW, IN TWO FIXED LINES.
  // `Stand-in local digest` labels the block; the 64 characters live in
  // `.rail-hash-value` as two 32-character lines, so they are asserted off the
  // block's own text rather than as one run in a sentence.
  const hashBlock = page.getByLabel('Output hash')
  await expect(hashBlock).toContainText('Stand-in local digest')
  await expect(hashBlock.locator('.rail-hash-value')).toHaveText(/^[a-f0-9]{64}$/)
  // AND THE PRODUCTION CLAIM IS WITHHELD FROM A NO-DATA DIGEST (D-13.4.1).
  await expect(hashBlock).not.toContainText('Byte-identical across')

  // AND NOTHING ON THE SCREEN CLAIMS PRODUCTION.
  await expect(page.getByText('EXACT LOCAL PRODUCTION PDF')).toHaveCount(0)
  await expect(page.getByRole('region', { name: /Current exact local production PDF/ })).toHaveCount(0)
  await expect(page.getByText(/Historical producer digest/)).toHaveCount(0)
  await expect(page.getByText('Preview unavailable: no sample data loaded')).toHaveCount(0)

  // The control the third gate disabled is reachable and usable, and the
  // export names what it would be writing.
  await page.getByRole('tab', { name: 'INPUTS' }).click()
  await expect(page.getByRole('button', { name: 'Re-render' })).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Save no-data PDF' })).toBeVisible()
})
