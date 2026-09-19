import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test, type Page } from '@playwright/test'

// STARTUP TEMPLATES, STORY 3 — THE BROWSER WITNESS FOR THE LAUNCH DIALOG.
//
// jsdom can prove the dialog's wiring against a fake engine; only a real build
// can prove the bundled thumbnails decode, that Escape and Cancel leave the REAL
// starter at revision 1, that an example's template and sample render through
// the real wasm engine into an admitted production PDF, and that the cards lay
// out without overlapping when the window is narrow.

const usableOfflineState = /^(Offline ready|Update available; current release remains usable)$/

const cardNames = ['Blank', 'Invoice', 'Bank Statement', 'Legal Contract', 'Electricity Bill']

async function expectLaunchDialog(page: Page) {
  const dialog = page.getByRole('dialog', { name: 'New template' })
  await expect(dialog).toBeVisible()
  const cards = dialog.getByRole('group', { name: 'Start from' }).getByRole('button')
  await expect(cards).toHaveCount(cardNames.length)
  for (const [index, name] of cardNames.entries()) await expect(cards.nth(index)).toHaveAccessibleName(name)
  const blank = dialog.getByRole('button', { name: 'Blank', exact: true })
  await expect(blank).toHaveAttribute('aria-pressed', 'true')
  await expect(blank).toBeFocused()
  // FIVE THUMBNAILS: Blank's drawn page and four engine-rendered PNGs, each
  // actually decoded — a broken image is still an <img>.
  await expect(dialog.getByTestId('startup-blank-page')).toBeVisible()
  const images = dialog.locator('img.startup-thumbnail')
  await expect(images).toHaveCount(4)
  await expect.poll(() => images.evaluateAll((nodes) => nodes.every((node) => (node as HTMLImageElement).complete && (node as HTMLImageElement).naturalWidth > 0))).toBe(true)
  return dialog
}

async function expectStarterCanvas(page: Page) {
  await expect(page.getByRole('dialog', { name: 'New template' })).toHaveCount(0)
  await expect(page.getByLabel('Canvas region')).toBeVisible()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect(page.locator('.document-name')).toHaveText('Untitled template')
}

async function expectExampleInPreview(page: Page, name: string) {
  await expect(page.getByRole('dialog', { name: 'New template' })).toHaveCount(0)
  await expect(page.locator('.document-name')).toHaveText(name)
  await expect(page.getByText('Unsaved local changes')).toBeVisible()
  await expect(page.getByLabel('Preview region')).toBeVisible()
  await expect(page.getByRole('region', { name: /Current exact local production PDF, revision \d+/ })).toBeVisible({ timeout: 60_000 })
  await expect(page.getByRole('note', { name: 'No-data preview notice' })).toHaveCount(0)
}

test('launch shows the dialog with Blank selected, and Escape lands on the starter canvas at revision 1', async ({ page }) => {
  await page.goto('/')
  const dialog = await expectLaunchDialog(page)
  await expect(dialog.getByRole('button', { name: 'Start blank' })).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expectStarterCanvas(page)
})

test('Cancel lands on the starter canvas at revision 1, even with an example selected', async ({ page }) => {
  await page.goto('/')
  const dialog = await expectLaunchDialog(page)
  await dialog.getByRole('button', { name: 'Invoice', exact: true }).click()
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expectStarterCanvas(page)
})

test('opening Invoice lands in Preview with its sample loaded', async ({ page }) => {
  await page.goto('/')
  const dialog = await expectLaunchDialog(page)
  await dialog.getByRole('button', { name: 'Invoice', exact: true }).click()
  await expect(dialog.getByRole('status')).toContainText('Invoice opens in Preview with')
  await expect(dialog.getByRole('status')).toContainText('invoice.sample.json')
  await dialog.getByRole('button', { name: 'Open example' }).click()
  await expectExampleInPreview(page, 'Invoice')
  // The sample tree is the one Load sample JSON would have installed.
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
})

// STORY 4 — NEW… OVER REAL EDITS, THROUGH THE REAL ENGINE. Keep editing must
// send nothing: the revision the edit produced is still the one on screen, and
// Undo still has the edit to undo. Discard then opens the example in Preview.
test('New… over an edited document warns before the dialog; Keep editing keeps it, Discard opens the dialog and Invoice lands in Preview', async ({ page }) => {
  await page.goto('/')
  await expectLaunchDialog(page)
  await page.keyboard.press('Escape')
  await expectStarterCanvas(page)
  // A REAL EDIT: add a page, which moves the engine past revision 1.
  await page.getByLabel('Canvas controls').getByRole('button', { name: 'Add page' }).click()
  const revision = page.getByTestId('engine-snapshot')
  await expect(revision).not.toHaveText(/GO SNAPSHOT · REVISION 1\b/)
  const edited = await revision.textContent()
  await expect(page.getByRole('button', { name: 'Undo' })).toBeEnabled()

  const dialog = page.getByRole('dialog', { name: 'New template' })
  const warning = page.getByRole('dialog', { name: 'Discard unsaved changes?' })
  await page.getByRole('button', { name: 'New…' }).click()
  await expect(warning).toBeVisible()
  await expect(warning).toHaveAccessibleDescription('Untitled template has unsaved changes.')
  await expect(dialog).toHaveCount(0)
  const keep = warning.getByRole('button', { name: 'Keep editing' })
  const discard = warning.getByRole('button', { name: 'Discard', exact: true })
  await expect(keep).toBeFocused()
  // Tab cycles through Keep editing and Discard only.
  await page.keyboard.press('Tab')
  await expect(discard).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(keep).toBeFocused()

  await keep.click()
  await expect(warning).toHaveCount(0)
  await expect(dialog).toHaveCount(0)
  await expect(revision).toHaveText(edited ?? '')
  await expect(page.getByRole('button', { name: 'Undo' })).toBeEnabled()

  await page.getByRole('button', { name: 'New…' }).click()
  await warning.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(warning).toHaveCount(0)
  await expect(dialog.getByRole('button', { name: 'Blank', exact: true })).toBeFocused()
  await expect(revision).toHaveText(edited ?? '')
  await dialog.getByRole('button', { name: 'Invoice', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open example' }).click()
  await expectExampleInPreview(page, 'Invoice')
})

test('Open existing file… opens a .folio from the dialog and closes it', async ({ page }) => {
  const fixture = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../fixtures/statement-1/input.folio'))
  await page.addInitScript((rawBytes) => {
    const bytes = new Uint8Array(rawBytes)
    const handle = { name: 'statement.folio', getFile: async () => new File([bytes], 'statement.folio', { type: 'application/json' }), createWritable: async () => { throw new Error('not reached') } }
    Object.assign(window, { showOpenFilePicker: async () => [handle] })
  }, [...fixture])
  await page.goto('/')
  const dialog = await expectLaunchDialog(page)
  await dialog.getByRole('button', { name: 'Open existing file…' }).click()
  await expect(dialog).toHaveCount(0)
  await expect(page.locator('.document-name')).toHaveText('statement.folio')
  // Proof the fixture really went through the engine: the bar's Open status.
  await expect(page.getByLabel('Local file actions').getByRole('status')).toHaveText(/^Opened local file statement\.folio(; canonical local changes need saving)?$/)
  await expect(page.getByLabel('Canvas region')).toBeVisible()
})

// A NARROW WINDOW. Five fixed-width thumbnails in five squeezed columns used to
// spill over their neighbours; the grid must wrap and each thumbnail stay
// inside its own card, with nothing wider than the scrolling card area.
for (const width of [720, 480]) {
  test(`at ${width}px wide every thumbnail stays inside its own card`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 })
    await page.goto('/')
    const dialog = await expectLaunchDialog(page)
    const cards = dialog.getByRole('group', { name: 'Start from' }).getByRole('button')
    for (let index = 0; index < cardNames.length; index++) {
      const card = await cards.nth(index).boundingBox()
      const thumbnail = await cards.nth(index).locator('.startup-thumbnail').boundingBox()
      if (!card || !thumbnail) throw new Error(`card ${cardNames[index]} has no box`)
      expect(thumbnail.x, `${cardNames[index]} thumbnail left edge`).toBeGreaterThanOrEqual(card.x - 0.5)
      expect(thumbnail.x + thumbnail.width, `${cardNames[index]} thumbnail right edge`).toBeLessThanOrEqual(card.x + card.width + 0.5)
    }
    // No card wider than the card area: every card box inside the body's box.
    const body = await dialog.locator('.startup-body').boundingBox()
    if (!body) throw new Error('card area has no box')
    for (let index = 0; index < cardNames.length; index++) {
      const card = await cards.nth(index).boundingBox()
      if (!card) throw new Error(`card ${cardNames[index]} has no box`)
      expect(card.x + card.width, `${cardNames[index]} card must not spill past the card area`).toBeLessThanOrEqual(body.x + body.width + 0.5)
    }
    const sheet = await dialog.locator('.startup-sheet').boundingBox()
    if (!sheet) throw new Error('sheet has no box')
    expect(sheet.x + sheet.width).toBeLessThanOrEqual(width)
  })
}

// THE GATE IS THE CORE TIER, MEASURED AT THE WIRE
// (spec-deferred-offline-cache, story 2, CAP-1).
//
// A cold first load DOES request deferred assets — the launch dialog draws four
// engine-rendered example thumbnails, and story 1 tiers those `deferred` — so
// "no deferred asset is requested" would be false and this test says something
// narrower and true: NONE OF THEM IS REQUESTED BEFORE THE ENGINE STARTS. The
// engine wasm is the boundary and it is an observable one: the page asks for it
// only once `engineMayStart` is true, which is once the worker has verified the
// core tier and broadcast `ready`.
//
// AND WHAT FOLLOWS IT IS NAMED RATHER THAN LEFT AS "SOMETHING". After the gate
// opens the designer asks for the starter template it opens into and the four
// thumbnails the dialog draws — 0.14 MiB — and for nothing else in the deferred
// tier. The CJK font (4.72 MiB), the 31 catalogue faces (3.01 MiB), the example
// templates and samples and the bundled documentation are not touched at all,
// which is the 7.96 MiB this spec exists to stop charging every visitor.
test('the blocking load asks for the core tier alone, and the dialog\'s thumbnails follow the engine', async ({ page }) => {
  const manifest = JSON.parse(readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'dist', 'offline-release-manifest.json'), 'utf8')) as { assets: ReadonlyArray<{ url: string; tier: string }> }
  const tierOf = new Map(manifest.assets.map((asset) => [asset.url, asset.tier]))
  const deferredCount = [...tierOf.values()].filter((tier) => tier === 'deferred').length
  expect(deferredCount, 'a release with no deferred tier would make every assertion below vacuous').toBeGreaterThan(40)
  const engineWasm = manifest.assets.find((asset) => asset.url.endsWith('.wasm'))?.url
  if (!engineWasm) throw new Error('the release carries no engine wasm for this test to take as the gate boundary')

  const requested: string[] = []
  page.on('request', (request) => {
    const { pathname } = new URL(request.url())
    if (tierOf.has(pathname)) requested.push(pathname)
  })
  await page.goto('/')
  await expectLaunchDialog(page)

  const engineAt = requested.indexOf(engineWasm)
  expect(engineAt, 'the engine wasm must have been requested, or there is no boundary to measure against').toBeGreaterThanOrEqual(0)
  const deferredBeforeEngine = requested.slice(0, engineAt).filter((url) => tierOf.get(url) === 'deferred')
  expect(deferredBeforeEngine, 'nothing outside the core tier may be asked for before the designer can start').toEqual([])

  // AND THE DEFERRED TIER THE FIRST LOAD DOES TOUCH IS EXACTLY THE DIALOG'S OWN
  // — the starter it opens into and the four thumbnails it draws. Naming them
  // is what keeps the assertion above from being satisfied by a load that
  // fetched the whole deferred tier one millisecond later.
  const deferred = [...new Set(requested.filter((url) => tierOf.get(url) === 'deferred'))]
  const thumbnails = deferred.filter((url) => url.includes('.thumbnail.'))
  expect(thumbnails, 'the dialog draws four engine-rendered thumbnails, and they are deferred assets').toHaveLength(4)
  expect(deferred.filter((url) => !url.includes('.thumbnail.') && !url.includes('/starter.')), 'no catalogue face, no CJK font, no example template or sample and no documentation page may be fetched by a first load').toEqual([])
})

// REWRITTEN BY spec-deferred-offline-cache STORY 2, AND THE REWRITE IS THE
// POINT RATHER THAN AN ACCOMMODATION.
//
// It used to open an example offline that had never been opened before, and it
// passed because the worker precached all 80 release assets — the bundled
// examples among them — before the designer would start at all. That is the
// 18.63 MiB first load this spec exists to remove: the examples are now in the
// DEFERRED tier and are fetched the first time one is opened.
//
// SO THE GUARANTEE IT PROVES IS THE ONE THE SPEC ACTUALLY MAKES: *"An author
// who has used the designer once still opens it, edits, previews and renders
// with the network disconnected."* The example is opened once online — which is
// the fetch, the hash verification and the cache write — and then opened again
// with the network down, off the cache, through a reload, with no request at
// all. An example NEVER opened is a different case: it is refused, and giving
// that refusal its words is CAP-3's, not this story's.
//
// THE THUMBNAILS ARE STILL PROVED OFFLINE, unchanged, and they are proved
// through the same mechanism rather than by exception: the dialog draws them on
// the first load, so they are fetched and kept then, and `expectLaunchDialog`
// after `setOffline(true)` requires all four to decode with no network.
test('an example opened once opens again offline, thumbnails and all', async ({ page, context }) => {
  await page.goto('/')
  await page.reload() // the first installation must activate before it can control a reload
  await expect(page.getByTestId('offline-status')).toHaveText(usableOfflineState)
  const online = await expectLaunchDialog(page)
  await online.getByRole('button', { name: 'Bank Statement', exact: true }).click()
  await expect(online.getByRole('button', { name: 'Bank Statement', exact: true })).toHaveAttribute('aria-pressed', 'true')
  await online.getByRole('button', { name: 'Open example' }).click()
  await expectExampleInPreview(page, 'Bank Statement')

  await context.setOffline(true)
  await page.reload()
  const offline = await expectLaunchDialog(page)
  await offline.getByRole('button', { name: 'Bank Statement', exact: true }).click()
  await expect(offline.getByRole('button', { name: 'Bank Statement', exact: true })).toHaveAttribute('aria-pressed', 'true')
  await offline.getByRole('button', { name: 'Open example' }).click()
  await expectExampleInPreview(page, 'Bank Statement')
})
