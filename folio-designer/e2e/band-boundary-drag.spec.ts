import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// STORY 12.5 — A BAND BOUNDARY IS DRAGGED ON THE CANVAS.
//
// THIS SUITE IS EXECUTED BY NO WORKFLOW TODAY (DW-193, open). `npm run
// test:e2e` is not part of `npm test` and no CI job runs it: it needs a full
// production build plus PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH, and the suite
// carries a standing red (browser-native-roundtrip.spec.ts's fresh-authored
// close case HANGS — measured at both 300s and 1500s budgets). The one gate
// that looks at this file at all is `npx tsc -p tsconfig.e2e.json --noEmit`,
// which checks that it COMPILES, not that it passes.
//
// SO IT MUST BE REPORTED AS WRITTEN-NOT-RUN AND NEVER COUNTED AS COVERAGE.
// It is written now anyway because DW-193 is about the RUNNER, not the spec: an
// unrun spec still pins intent, costs nothing to hold, and is nearly free to
// write beside the implementation rather than reconstructed in three months.
//
// WHY IT EXISTS AT ALL WHEN App.test.tsx COVERS THE MATRIX: jsdom cannot
// express pointer capture, cannot lose a button mid-gesture, applies no
// stylesheet and so has no hit area — a press at a real coordinate on a real
// 7px strip is a thing only a browser can attempt. This is the only suite that
// can watch a proposal appear over UNMOVED band rects and then watch the rects
// move when, and only when, the engine has accepted.
//
// THE HANDLE NAMES MATTER, AND THEY ARE NOT THE BAND NAMES.
// `application-shell.spec.ts` and `browser-native-roundtrip.spec.ts` already
// reach the bands with `getByRole('region', { name: 'Page Header', exact: true
// })` under Playwright STRICT MODE, so a handle named `Page Header` would break
// two shipped specs on a duplicate match. The handles name the ACT.

// The projected band rect, read as the two custom properties the projection
// wrote. Nothing here measures the DOM: this is the browser reading back what
// Go's numbers were painted as, which is exactly the claim under test.
// THE PRESS POINT MUST BE INSIDE THE VIEWPORT, and this helper exists because
// the first execution of this suite proved it is not free.
//
// `page.mouse.move()` does NOT scroll: it drives the pointer at raw viewport
// coordinates. The default viewport is 720px tall and the page-footer handle
// sits at y=880 on a resting A4 canvas, so the footer test pressed 160px BELOW
// the window, `document.elementFromPoint` returned null, and no drag ever
// began — the run reported `.band-boundary-proposal` count 0 while the feature
// was working perfectly. The header handle is at y=213 and passed, which is
// exactly why the defect looked like a footer-only product bug.
//
// Neither `toHaveCount(1)` nor a non-null `boundingBox()` can see this: an
// off-screen element has both. So this helper scrolls first, then ASSERTS the
// centre is inside the viewport — turning a silent miss into a named failure.
const pressPoint = async (page: import('@playwright/test').Page, handle: import('@playwright/test').Locator) => {
  await handle.scrollIntoViewIfNeeded()
  const box = await handle.boundingBox()
  expect(box, 'the handle must have a box to press').not.toBeNull()
  const viewport = page.viewportSize()
  expect(viewport, 'the run must declare a viewport').not.toBeNull()
  const point = { x: box!.x + box!.width / 2, y: box!.y + box!.height / 2 }
  expect(point.y, 'the press point is below the viewport — mouse.move does not scroll').toBeLessThanOrEqual(viewport!.height)
  expect(point.y, 'the press point is above the viewport — mouse.move does not scroll').toBeGreaterThanOrEqual(0)
  return point
}

const bandGeometry = async (page: import('@playwright/test').Page): Promise<ReadonlyArray<string>> =>
  page.locator('.page-surface').first().locator('.page-band').evaluateAll((bands) =>
    bands.map((band) => `${(band as HTMLElement).style.getPropertyValue('--band-y')}/${(band as HTMLElement).style.getPropertyValue('--band-height')}`))

test('dragging the header/content boundary proposes, then commits through the real Go worker', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const handle = page.getByRole('button', { name: 'Resize the page header', exact: true })
  await expect(handle).toHaveCount(1)
  const press = await pressPoint(page, handle)
  const resting = await bandGeometry(page)

  // PRESS AND MOVE, WITHOUT RELEASING. Two mouse.move calls, because one is
  // delivered as a single jump and a drag that only ever sees its endpoint is
  // not the gesture an author makes.
  await page.mouse.move(press.x, press.y)
  await page.mouse.down()
  await page.mouse.move(press.x, press.y + 20)
  await page.mouse.move(press.x, press.y + 40)

  // The proposal is up, the readout shows the PROPOSAL — not a prediction of the
  // accepted height, because Snap is on by default and the ENGINE rounds — and
  // NOTHING ELSE HAS MOVED. No band rect was re-placed by the browser (AD-24) and the
  // engine has not been asked anything: the revision is still 1.
  await expect(page.locator('.band-boundary-proposal')).toHaveCount(1)
  await expect(page.locator('.band-boundary-readout')).toHaveText(/^\d+(?:\.\d+)?$/)
  const proposed = await page.locator('.band-boundary-readout').innerText()
  expect(await bandGeometry(page)).toEqual(resting)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)

  // RELEASE. Now — and only now — one command goes, and the canvas re-projects
  // from whatever the engine accepted, including the snap.
  await page.mouse.up()
  await expect(page.getByTestId('engine-snapshot')).not.toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect(page.locator('.band-boundary-proposal')).toHaveCount(0)
  await expect
    .poll(async () => (await bandGeometry(page)).join('|'))
    .not.toBe(resting.join('|'))

  // AND THE PANEL AGREES, which is what makes this a round trip rather than a
  // repaint: the typed row reads the height the engine accepted. Snapping is
  // on by default, so the accepted value may differ from the proposal — the
  // assertion is that the panel shows what the DOCUMENT holds.
  const row = page.getByRole('textbox', { name: 'Page header height (pt)' })
  await expect(row).toBeVisible()
  const accepted = await row.inputValue()
  expect(Number(accepted)).toBeGreaterThan(Number(proposed) - 6)
  expect(Number(accepted)).toBeLessThan(Number(proposed) + 6)
})

test('dragging the content/footer boundary upward grows the page footer', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const handle = page.getByRole('button', { name: 'Resize the page footer', exact: true })
  await expect(handle).toHaveCount(1)
  const press = await pressPoint(page, handle)
  const before = Number(await page.getByRole('textbox', { name: 'Page footer height (pt)' }).inputValue())
  const resting = await bandGeometry(page)

  // A FOOTER GROWS AS THE POINTER RISES, which is the direction a suite that
  // only ever dragged the header would never exercise.
  await page.mouse.move(press.x, press.y)
  await page.mouse.down()
  await page.mouse.move(press.x, press.y - 12)
  await page.mouse.move(press.x, press.y - 24)
  await expect(page.locator('.band-boundary-proposal')).toHaveCount(1)
  expect(await bandGeometry(page)).toEqual(resting)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)

  await page.mouse.up()
  await expect(page.getByTestId('engine-snapshot')).not.toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect
    .poll(async () => Number(await page.getByRole('textbox', { name: 'Page footer height (pt)' }).inputValue()))
    .toBeGreaterThan(before)
})

// THE EDGE ABOVE THE PAGE HEADER TAB IS THE PAGE MARGIN, not a band boundary,
// and this is the browser's statement of Matrix row 3: there is no third handle
// and the three region names are still exactly one each.
test('exactly two boundary handles exist, and none of them collides with a band name', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect(page.locator('.band-boundary-handle')).toHaveCount(2)
  await expect(page.getByRole('region', { name: 'Page Header', exact: true })).toHaveCount(1)
  await expect(page.getByRole('region', { name: 'Content', exact: true })).toHaveCount(1)
  await expect(page.getByRole('region', { name: 'Page Footer', exact: true })).toHaveCount(1)
  await expect(page.getByRole('button', { name: 'Page Header', exact: true })).toHaveCount(0)
  await expect(page.getByRole('region', { name: 'Page Header', exact: true }).locator('.band-boundary-handle')).toHaveCount(0)
})
