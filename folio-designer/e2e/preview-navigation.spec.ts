import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// STORY 13.2 — THE BROWSER WITNESS FOR THE ONE CLAIM jsdom CANNOT SEE.
//
// jsdom performs no layout, so nothing in the unit suite can distinguish a
// scrollable container from a non-scrollable one, or a pinned element from one
// that has scrolled out of view. Every CSS claim this story makes therefore has
// its only proof here. The distinguishing observable is WHAT STAYS PUT: if
// `.preview-region` is the scroller, the no-data notice
// and the evidence line scroll away with the page; if `.pdf-preview-scroll` is,
// they hold while the page moves under them.
//
// COMPILE-CHECKED LOCALLY, EXECUTED IN CI. The story's cadence forbids a local
// Playwright run because `webServer.command` is `npm run build`, which the
// story's Boundaries forbid outright. CI runs the whole browser suite on every
// push (DW-268, discharged at `adf905a`), so this is real per-commit coverage
// and is written as such — never as a compile-only placeholder.
//
// NOT ONE PROHIBITED IDENTIFIER IS SPELLED HERE, deliberately. Every `e2e/`
// file is auto-enrolled by the independent directory walk at
// `canvas-authority-contract.test.ts:673` and the `src/preview/` exception does
// not reach it, so the whole witness is built out of Playwright's own
// `boundingBox()` and `mouse.wheel()`, neither of which the scan names.
const template = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/testdata/example/first-pdf.folio'))

// The viewport is Playwright's default 1280x720 — `playwright.config.ts`
// declares none. A letter page at 200% is 1224 CSS px wide and 1584 tall, and
// the page area is a few hundred px of a 648px workbench, so the page overflows
// BOTH axes. That is a precondition of everything below, and it is asserted
// rather than assumed: a page that fits its container cannot witness a scroll.
const zoomed = 2

test('scrolls the page area alone, and leaves the chrome that describes it fixed and reachable', async ({ page }) => {
  // THE FALLBACK FILE TIER, FORCED — without which this spec cannot pass at all.
  // Headless Chromium HAS the File System Access API, so the app takes the
  // native tier, calls `showOpenFilePicker()` directly, and emits no
  // `filechooser` event ever: the wait below then times out at 90s on a
  // perfectly working application. Population, measured: ten specs under `e2e/`
  // wait on a `filechooser`, and the eight that pass all force this tier first
  // (`local-file-actions.spec.ts:11` is the model). This spec and
  // `preview-no-data.spec.ts` were the only two that did not, and they were
  // exactly the two that failed the first time the suite was ever run.
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const templateChooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await templateChooser).setFiles({ name: 'statement.folio', mimeType: 'application/json', buffer: template })
  await expect(page.locator('.document-name')).toHaveText('statement.folio')

  // The no-data fixture is chosen because it is the one that renders the most
  // chrome around the page: 13.4's amber notice, the
  // freshness line and the evidence line. Story 13.4's own witness establishes
  // that this template reaches an admitted preview with no sample data.
  await page.getByRole('button', { name: 'PREVIEW' }).click()
  const pageArea = page.getByRole('img', { name: /Current no-data layout PDF, revision \d+/ })
  await expect(pageArea).toBeVisible({ timeout: 60_000 })

  // THE CONTROLS ARE IN THE STATUS BAR, WHICH IS OUTSIDE BOTH `<main>`s. Their
  // presence here is what makes the page area able to carry the page and
  // nothing else, and the zoom below is driven through the real control rather
  // than through a style, so the witness exercises the shipped path.
  const statusBar = page.getByRole('contentinfo', { name: 'Status bar' })
  const navigation = page.getByRole('group', { name: 'PDF navigation' })
  await expect(navigation.getByRole('button', { name: 'Previous PDF page' })).toBeVisible()
  await expect(navigation.getByRole('button', { name: 'Next PDF page' })).toBeVisible()
  await expect(navigation.getByRole('combobox', { name: 'PDF zoom' })).toBeVisible()
  await navigation.getByRole('combobox', { name: 'PDF zoom' }).selectOption({ label: `${zoomed * 100}%` })
  await expect(navigation.getByRole('textbox', { name: 'PDF zoom percentage' })).toHaveValue(String(zoomed * 100))

  const canvas = pageArea.locator('canvas')
  await expect(canvas).toBeVisible()
  const notice = page.getByRole('note', { name: 'No-data preview notice' })
  // ⚠ STORY 13.3 — THE DIGEST IS NO LONGER ONE OF THE HELD-POSITION SUBJECTS,
  // AND REMOVING IT FROM THAT LIST IS THE POINT (review P7).
  //
  // It used to be `.preview-evidence`, a line INSIDE the preview scroller, so
  // "it did not move when the page scrolled" was a claim that could fail. The
  // rail put it in the Inspector column, which is a different, non-scrolling
  // container: `after.evidence.y === before.evidence.y` there is true of any
  // element in any sibling of the scroller, guards nothing, and reads as a pin.
  // The notice is still inside the preview region and still carries
  // that claim. What survives for the digest is the claim that IS still real —
  // it stays on screen and keeps saying the same thing across the scroll — and
  // it is asserted below rather than measured here.
  const evidence = page.getByLabel('Output hash').locator('.rail-hash-value')
  const digestBefore = await evidence.textContent()
  expect(digestBefore).toMatch(/^[a-f0-9]{64}$/)

  // Read once, at rest, and then compared after the scroll. `boundingBox()` is
  // viewport-relative, which is exactly the frame this claim is about.
  const boxes = async () => {
    const measured = await Promise.all([canvas, pageArea, notice, statusBar].map(async (locator) => {
      const box = await locator.boundingBox()
      expect(box).not.toBeNull()
      return box!
    }))
    return { canvas: measured[0]!, pageArea: measured[1]!, notice: measured[2]!, statusBar: measured[3]! }
  }
  const before = await boxes()

  // THE PRECONDITION, ASSERTED. Without an overflowing page there is nothing to
  // scroll, and every "held its position" assertion below would pass over a
  // viewer that simply never moves. This is the arm that keeps the witness from
  // going vacuous if a future layout change makes the page area large enough to
  // hold a 200% page whole.
  expect(before.canvas.height).toBeGreaterThan(before.pageArea.height)
  expect(before.canvas.width).toBeGreaterThan(before.pageArea.width)

  // THE LEFT AND TOP EDGES OF A ZOOMED PAGE ARE REACHABLE — the `safe center`
  // fix, on BOTH axes. `.pdf-preview-scroll` centres its single child; with
  // plain `center` and a child larger than the box, the overflow is split
  // evenly and the leading edge sits at a negative offset the scroller can
  // never reach, because a scroll offset does not go below zero. `safe center`
  // falls back to start-alignment in exactly that case, which puts the leading
  // edge AT the container's own edge and no further. One pixel for the border.
  //
  // BOTH axes are asserted because the rule sets both keywords and each is a
  // separate declaration: `justify-content` governs x, `align-content` governs
  // y, and a revert of either alone is a defect the other cannot see.
  expect(before.canvas.x).toBeGreaterThanOrEqual(before.pageArea.x - 1)
  expect(before.canvas.y).toBeGreaterThanOrEqual(before.pageArea.y - 1)

  // THE PREVIEW STATUS BAR IS THE TALLER OF THE TWO DECLARED SIZES. `.status-bar`
  // declares no height of its own — its height IS the `.app-shell` grid track,
  // which `.app-shell-preview` switches to `--status-bar-height-preview`. So
  // this is the one observable that can tell a minted-and-used token from a
  // minted-and-forgotten one, and no unit test can see it: jsdom applies no
  // stylesheet at all.
  expect(before.statusBar.height).toBe(32)

  // Everything the chrome assertions depend on is on screen to begin with, so
  // "still visible afterwards" is a change and not a restatement.
  for (const locator of [notice, evidence]) await expect(locator).toBeInViewport()

  // THE SCROLL ITSELF, DRIVEN AS A USER DRIVES IT. A wheel over the page area
  // scrolls the nearest scrollable ancestor. Which element that is IS the
  // property under test: before this story `.pdf-preview-scroll` had no height
  // at all, so its own overflow never activated vertically and the wheel
  // reached `.preview-region` instead, taking the notice with
  // it. Both outcomes move something; only one of them moves the right thing.
  await canvas.hover()
  await page.mouse.wheel(0, 600)
  await expect.poll(async () => (await canvas.boundingBox())!.y, { timeout: 10_000 }).toBeLessThan(before.canvas.y - 100)

  const after = await boxes()

  // THE PAGE MOVED...
  expect(after.canvas.y).toBeLessThan(before.canvas.y - 100)

  // ...AND EVERYTHING AROUND IT DID NOT. Asserted as exact viewport positions,
  // not as `toBeVisible()`: Playwright's visibility means a non-empty box, so a
  // diagnostic that had scrolled clean out of the window would still be
  // "visible" by that measure. Position is the property; presence is not.
  expect(after.pageArea.y).toBe(before.pageArea.y)
  expect(after.pageArea.height).toBe(before.pageArea.height)
  expect(after.notice.y).toBe(before.notice.y)
  expect(after.statusBar.y).toBe(before.statusBar.y)

  // AND STILL REACHABLE — the condition attached to this story at approval. A
  // diagnostic the author cannot see while looking at the thing it describes is
  // one that may as well not be rendered, and `toBeInViewport` is the assertion
  // that can tell the two apart.
  for (const locator of [notice, evidence]) await expect(locator).toBeInViewport()
  await expect(navigation.getByRole('button', { name: 'Next PDF page' })).toBeInViewport()

  // The horizontal axis is DW-191's own axis — the one on which the tear-down
  // was live before this story — so it is scrolled too, and the chrome holds
  // through that as well.
  await page.mouse.wheel(400, 0)
  await expect.poll(async () => (await canvas.boundingBox())!.x, { timeout: 10_000 }).toBeLessThan(before.canvas.x - 100)
  const sideways = await boxes()
  expect(sideways.notice.x).toBe(before.notice.x)
  expect(sideways.pageArea.x).toBe(before.pageArea.x)
  for (const locator of [notice, evidence]) await expect(locator).toBeInViewport()
  // AND THE DIGEST STILL SAYS THE SAME THING. This is the digest claim that a
  // scroll can actually break — a rail re-rendered or re-keyed mid-scroll would
  // change or lose it — where its viewport position cannot.
  await expect(evidence).toHaveText(digestBefore!)

  // FIT WIDTH, AGAINST A CONTAINER THAT WAS REALLY LAID OUT. This is the only
  // place in the repository where the story's container measurement runs against
  // actual layout: `pdf-viewer.test.tsx` proves the arithmetic by installing a
  // fake box on `HTMLDivElement.prototype`, which is exactly as big as the test
  // says it is. Here the number comes from the browser, so the whole chain —
  // the padding moved off the measured element, `scrollbar-gutter: stable`, the
  // grid track, the flex cap — has to be right for the page to end up fitting.
  await navigation.getByRole('combobox', { name: 'PDF zoom' }).selectOption({ label: 'Fit width' })
  // The readout shows the RESOLVED percentage, never the word "fit" — the
  // matrix row that says so has no other witness.
  const zoomReadout = navigation.getByRole('textbox', { name: 'PDF zoom percentage' })
  await expect(zoomReadout).not.toHaveValue(String(zoomed * 100))
  await expect(zoomReadout).toHaveValue(/^\d+$/)
  // AND THE PAGE ACTUALLY FITS. `boundingBox()` on the scroll host is its border
  // box, which includes the stable scrollbar gutter the canvas does not get, so
  // a fitted page is strictly narrower than its host with room to spare.
  await expect.poll(async () => (await canvas.boundingBox())!.width, { timeout: 10_000 }).toBeLessThanOrEqual((await pageArea.boundingBox())!.width)
  // The chrome is still where it was through all of that.
  const fitted = await boxes()
  expect(fitted.notice.y).toBe(before.notice.y)
  expect(fitted.statusBar.y).toBe(before.statusBar.y)
  for (const locator of [notice, evidence]) await expect(locator).toBeInViewport()
})

test('centers navigation above the PDF and keeps the footer readable at narrow desktop width', async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT/)
  await page.getByRole('button', { name: 'PREVIEW' }).click()
  const region = page.getByRole('main', { name: 'Preview region' })
  const navigation = region.getByRole('group', { name: 'PDF navigation' })
  const viewer = region.locator('.pdf-preview')
  await expect(viewer).toBeVisible()
  const controls = await navigation.boundingBox()
  const document = await viewer.boundingBox()
  expect(controls).not.toBeNull()
  expect(document).not.toBeNull()
  expect(controls!.x + controls!.width / 2).toBeCloseTo(document!.x + document!.width / 2, 0)
  expect(controls!.y + controls!.height).toBeLessThanOrEqual(document!.y)
  await expect(navigation).toBeInViewport({ ratio: 1 })
  await expect(page.getByTestId('local-only-assurance')).toBeInViewport({ ratio: 1 })
  await expect(page.getByRole('contentinfo', { name: 'Status bar' }).getByRole('group', { name: 'PDF navigation' })).toHaveCount(0)
  await navigation.getByRole('textbox', { name: 'PDF zoom percentage' }).fill('133')
  await navigation.getByRole('textbox', { name: 'PDF zoom percentage' }).press('Enter')
  await expect(navigation.getByRole('combobox', { name: 'PDF zoom' })).toHaveValue('custom')
})
