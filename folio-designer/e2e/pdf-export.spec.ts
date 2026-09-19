import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// STORY 13.1 — THE BROWSER WITNESS FOR SAVE PDF.
//
// COMPILE-CHECKED LOCALLY, EXECUTED IN CI. The story's own heavy-test cadence
// runs unit + lint + typecheck, and the Playwright suite is not run on the
// authoring machine — its web server builds the release, which that cadence
// forbids. Nothing in this file may be reported as locally executed coverage.
// CI runs the whole browser suite on every push (DW-268, discharged at
// `adf905a`), so these two tests execute per-commit rather than only at the
// epic-boundary gate, which is why they are written as real assertions.
//
// It uses the REAL two tiers over fake browser seams — a nulled picker pair for
// the download tier, a picker pair for the activation-gated one — which is the
// one thing jsdom cannot witness: that the bytes leaving the tab are the bytes
// the displayed producer digest covers.
//
// STORY 13.3 — THAT DIGEST MOVED FROM A FOOTNOTE TO THE EVIDENCE RAIL. It is
// now `.rail-hash-value`, a bordered mono block carrying all 64 characters in
// two fixed 32-character lines; `toContainText` reads the block's text, which
// is the two lines concatenated, so what these two tests compare is unchanged.
const template = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/testdata/example/first-pdf.folio'))
const sampleData = Buffer.from('{"customer":{"name":"Ada"}}')

type PdfSaveProbe = typeof window & {
  __folio8PdfBytes?: number[]
  __folio8PdfPicker?: { suggestedName: string; types: ReadonlyArray<{ description: string; accept: Record<string, string[]> }> }
  __folio8TemplateWrites?: number
}

test('the fallback tier downloads the current preview as a .pdf without touching the template name', async ({ page }) => {
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
  await (await sampleChooser).setFiles({ name: 'sample.json', mimeType: 'application/json', buffer: sampleData })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
  await page.getByRole('button', { name: 'PREVIEW' }).click()
  await page.getByRole('tab', { name: 'INPUTS' }).click()
  await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible()
  const download = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Save PDF' }).click()
  const saved = await download
  // The `.folio` suffix is REPLACED, never appended to.
  expect(saved.suggestedFilename()).toBe('statement.pdf')
  await expect(page.getByText(/Downloaded PDF of revision \d+ as statement\.pdf/)).toBeVisible()
  // THE BYTES THAT LEFT THE TAB, not merely the name they left under. The
  // download stream is hashed and compared with the producer digest the
  // evidence line displays, exactly as the native tier below does — a tier that
  // downloaded a truncated or re-encoded body under the right filename would
  // otherwise pass this test.
  const streamed = await saved.createReadStream()
  const chunks: Buffer[] = []
  for await (const chunk of streamed) chunks.push(Buffer.from(chunk))
  const body = Buffer.concat(chunks)
  expect(body.subarray(0, 5).toString('latin1')).toBe('%PDF-')
  await expect(page.locator('.rail-hash-value')).toContainText(createHash('sha256').update(body).digest('hex'))
  // The document the author is editing is untouched by an output save.
  await expect(page.locator('.document-name')).toHaveText('statement.folio')
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test('the activation-gated tier writes exactly the bytes the displayed producer digest covers and never reaches the held .folio handle', async ({ page }) => {
  await page.addInitScript(([rawTemplate, rawSample]: [number[], number[]]) => {
    const probe = window as PdfSaveProbe
    probe.__folio8TemplateWrites = 0
    const held = {
      name: 'statement.folio',
      getFile: async () => new File([new Uint8Array(rawTemplate)], 'statement.folio', { type: 'application/json' }),
      // A PDF save that passed the template's retained target would arrive
      // HERE, and this counter is how that becomes visible rather than silent.
      createWritable: async () => ({ write: async () => { probe.__folio8TemplateWrites = (probe.__folio8TemplateWrites ?? 0) + 1 }, close: async () => undefined }),
    }
    const sample = {
      name: 'sample.json',
      getFile: async () => new File([new Uint8Array(rawSample)], 'sample.json', { type: 'application/json' }),
      createWritable: async () => { throw new Error('the sample handle is never written') },
    }
    const saved = {
      name: 'statement.pdf',
      getFile: async () => new File([], 'statement.pdf', { type: 'application/pdf' }),
      createWritable: async () => ({ write: async (written: ArrayBuffer) => { probe.__folio8PdfBytes = Array.from(new Uint8Array(written)) }, close: async () => undefined }),
    }
    // Ordered, not content-sniffed: the first open is the template and the
    // second is the sample, so the fake never decides by inspecting the very
    // request under test.
    let opens = 0
    Object.assign(window, {
      showOpenFilePicker: async () => [++opens === 1 ? held : sample],
      showSaveFilePicker: async (options: PdfSaveProbe['__folio8PdfPicker']) => { probe.__folio8PdfPicker = options; return saved },
    })
  }, [[...template], [...sampleData]] as [number[], number[]])
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('button', { name: 'Open local template' }).click()
  await expect(page.locator('.document-name')).toHaveText('statement.folio')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
  await page.getByRole('button', { name: 'PREVIEW' }).click()
  await page.getByRole('tab', { name: 'INPUTS' }).click()
  await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible()
  await page.getByRole('button', { name: 'Save PDF' }).click()
  await expect.poll(() => page.evaluate(() => (window as PdfSaveProbe).__folio8PdfBytes?.length ?? 0)).toBeGreaterThan(0)
  await expect(page.getByText(/Saved PDF of revision \d+ as statement\.pdf/)).toBeVisible()

  const picker = await page.evaluate(() => (window as PdfSaveProbe).__folio8PdfPicker)
  expect(picker?.suggestedName).toBe('statement.pdf')
  expect(picker?.types?.[0]).toEqual({ description: 'PDF document', accept: { 'application/pdf': ['.pdf'] } })

  // THE BYTES ARE A PDF, AND THEY ARE THE ONES THE PAGE CLAIMS. The digest is
  // computed over what the handle actually received and compared with the
  // producer digest the evidence line displays: nothing re-rendered, nothing
  // re-serialized, nothing truncated on the way out.
  const written = await page.evaluate(async () => {
    const bytes = Uint8Array.from((window as PdfSaveProbe).__folio8PdfBytes ?? [])
    const hash = await crypto.subtle.digest('SHA-256', bytes)
    return { header: new TextDecoder().decode(bytes.slice(0, 5)), digest: [...new Uint8Array(hash)].map((byte) => byte.toString(16).padStart(2, '0')).join('') }
  })
  expect(written.header).toBe('%PDF-')
  await expect(page.locator('.rail-hash-value')).toContainText(written.digest)

  // AND THE AUTHOR'S TEMPLATE WAS NEVER OPENED FOR WRITING.
  expect(await page.evaluate(() => (window as PdfSaveProbe).__folio8TemplateWrites)).toBe(0)
  await expect(page.locator('.document-name')).toHaveText('statement.folio')
  await expect(page.getByRole('alert')).toHaveCount(0)
})
