import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'

// STORY 14.3 — THE ONLY HONEST WITNESS FOR POINTER GEOMETRY.
//
// jsdom applies no stylesheet, computes no layout and no stacking, and reports
// every rect as zeros, so nothing in `src/App.test.tsx` can observe a pointer
// landing near-but-not-on a 1pt rule, a pseudo-element that reaches outside its
// box, or which of two overlapping boxes a press resolves to. The unit suite
// proves the arithmetic, the declarations and the mechanism; this file proves
// the consequence, in a real browser at real coordinates.
//
// `boundingBox()` is Playwright's own protocol-side measurement and is NOT the
// `getBoundingClientRect`/`getClientRects` that `canvas-authority-contract.test.ts`
// bans across this corpus — `e2e/component-manipulation.spec.ts` already drives
// `page.mouse` off it and is green.
//
// ⚠ PER D-000.33 THIS SPEC IS COMPILED IN STORY 14.3 AND EXECUTED AT THE EPIC 14
// BOUNDARY GATE. Until a run log exists it is not coverage and must not be
// described as any.

// The one selected component, by id, as the inspector states it.
const identity = (page: Page): Locator => page.locator('.component-identity-meta')

// DOCUMENT ORDER, READ FROM THE DOM ITSELF. AC3's contract is "the last in
// document order wins", and a test that instead read the winner off the
// inspector would be deriving its expected answer from AC1 — the very
// behaviour this story adds — so a defect making both placement-selection and
// hit resolution favour the same wrong component would pass. `getAttribute` is
// not a layout query and is not in the contract's banned set.
async function documentOrder(page: Page): Promise<Array<string | null>> {
  return page.locator('.canvas-component[data-component-id]')
    .evaluateAll((nodes) => nodes.map((node) => node.getAttribute('data-component-id')))
}

// The pad the element itself declares, so the probe points below are derived
// from the shipped arithmetic rather than from a number copied into this file.
async function padOf(target: Locator): Promise<number> {
  return Number((await target.evaluate((node) => (node as HTMLElement).style.getPropertyValue('--hit-pad-y'))).replace('px', ''))
}

// Escape on the canvas region is the deliberate way to clear a selection and it
// is not gated on the event's target, so it never depends on where a click would
// have landed — which matters in a file whose whole subject is what a given
// coordinate hits.
async function deselect(page: Page): Promise<void> {
  await page.getByLabel('Canvas region').press('Escape')
  await expect(page.getByText('Component properties require a selection.')).toBeVisible()
}

async function press(page: Page, x: number, y: number): Promise<void> {
  await page.mouse.move(x, y)
  await page.mouse.down()
  await page.mouse.up()
}

// The armed palette places on the band's own pointerup. Placing by POINTER is
// the gesture under test where a coordinate matters; the rest take the keyboard
// route, which lands on the band origin deterministically.
async function placeByKeyboard(page: Page, item: string): Promise<void> {
  await page.getByRole('button', { name: item }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).press('Enter')
}

async function boxOf(target: Locator): Promise<{ x: number; y: number; width: number; height: number }> {
  const box = await target.boundingBox()
  if (!box) throw new Error('the target was not painted')
  return box
}

// AC1/AC4 in a real browser, and the precondition for everything below it: a
// placed component is the selected component, and it is the focused one.
test('a placed component arrives selected, focused, and showing its own properties', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect(page.getByText('Component properties require a selection.')).toBeVisible()
  await placeByKeyboard(page, 'Place Line')
  const line = page.getByLabel(/line component e/)
  await expect(line).toHaveCount(1)
  await expect(identity(page)).toHaveText(/ · band: content$/)
  await expect(page.getByText('Component properties require a selection.')).toHaveCount(0)
  await expect(line).toBeFocused()
})

// AC2, AND THE CLAIM JSDOM CANNOT MAKE. The press lands OUTSIDE the drawn box —
// four CSS pixels below a rule that draws two — and inside the padded region the
// `::before` hangs off it. Before this story that press reached the band and
// selected nothing.
test('a press beside a thin rule, outside its drawn box, still selects the rule', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await placeByKeyboard(page, 'Place Line')
  const line = page.getByLabel(/line component e/)
  await expect(line).toHaveCount(1)
  const drawn = await boxOf(line)
  // The rule really is under the 12px comfortable hit size, and the pad really
  // does reach past the probe below — or the press is not testing anything.
  expect(drawn.height).toBeLessThan(12)
  const pad = await padOf(line)
  expect(pad).toBeGreaterThan(4)
  // The pad brings the reachable extent to exactly the ruled 12px, measured off
  // what is DRAWN rather than off the projection.
  expect(drawn.height + 2 * pad).toBeCloseTo(12, 5)
  await deselect(page)
  // Four pixels below the drawn box: outside the paint, inside the pad.
  await press(page, drawn.x + drawn.width / 2, drawn.y + drawn.height + 4)
  await expect(identity(page)).toHaveText(/ · band: content$/)
  await expect(page.getByText('Component properties require a selection.')).toHaveCount(0)
})

// ⚠ THE TWO ROWS THE FROZEN BLOCK'S 2026-09-09 AMENDMENT ADDED, AND THEY ARE
// ONE TEST BECAUSE THEY ARE ONE CONTRACT: a component's own paint outranks a
// neighbour's invisible pad, AND the pad still wins where it crosses no
// neighbour's paint. Either half alone is satisfiable by a wrong build — the
// first by deleting the pad, the second by the regression this fixes — so
// neither is allowed to stand on its own.
//
// The mechanism is `.canvas-component::before { z-index: -1 }` over
// `.page-band { isolation: isolate }`. Without the z-index a LATER sibling's pad
// covers an EARLIER sibling's whole painted box; without the isolation the
// negative z-index escapes to the root stacking context and the pad stops being
// reachable at all.
test('a component own paint outranks a later neighbour pad, and the pad still wins over empty canvas', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  // Snap off, so the rule lands where this test puts it rather than on the 6pt
  // grid — the overlap under test is a few pixels wide.
  await page.getByRole('button', { name: /^Snap on/ }).click()
  await placeByKeyboard(page, 'Place Text')
  const text = page.getByLabel(/text component e/)
  await expect(text).toHaveCount(1)
  const textBox = await boxOf(text)

  // The rule is placed AFTER the text box, so it is later in document order and
  // its pad is the one that would otherwise win everywhere.
  await page.getByRole('button', { name: 'Place Line' }).click()
  await press(page, textBox.x + textBox.width / 2, textBox.y + textBox.height + 2)
  const rule = page.getByLabel(/line component e/)
  await expect(rule).toHaveCount(1)
  const order = await documentOrder(page)
  expect(order).toHaveLength(2)
  const ruleId = order[order.length - 1]
  const textId = order[0]
  expect(ruleId).not.toBe(textId)

  const ruleBox = await boxOf(rule)
  const pad = await padOf(rule)
  // THE PRECONDITION, ASSERTED: the rule's pad really does reach up into the
  // text box's painted area. Without this the test could pass by testing
  // nothing.
  expect(ruleBox.y - pad).toBeLessThan(textBox.y + textBox.height)
  expect(ruleBox.y).toBeGreaterThan(textBox.y + textBox.height)

  // ROW ONE: inside the text box's paint AND inside the rule's pad. The paint
  // wins. Before the amendment this selected the rule and dragged it.
  await deselect(page)
  await press(page, textBox.x + textBox.width / 2, textBox.y + textBox.height - 1)
  await expect(identity(page)).toHaveText(new RegExp(`^${textId} `))

  // ROW TWO, THE MANDATORY COMPLEMENT: below the text box, inside the rule's pad
  // and crossing no neighbour's paint. The pad wins. This is what stops the
  // arbitration degrading into "the pad never wins anything".
  await deselect(page)
  await press(page, textBox.x + textBox.width / 2, ruleBox.y + ruleBox.height + 2)
  await expect(identity(page)).toHaveText(new RegExp(`^${ruleId} `))
})

// AC3 IS PRESERVATION. Two thin components are stacked at the same coordinates
// and one spot is pressed three times: the SAME one answers every time, and it
// is the last in DOCUMENT ORDER — read from the DOM, never from the inspector,
// so this row cannot be satisfied by the placement-selection behaviour it sits
// beside. No `.canvas-component` box carries a `z-index`, and `begin()` stops
// the pointerdown so exactly one component handles it. This row still passes
// against a build with the padded region reverted, which is what makes it a
// determinism guard rather than a padding guard.
test('one spot over two overlapping thin components selects the same one every time', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await placeByKeyboard(page, 'Place Line')
  await expect(page.getByLabel(/line component e/)).toHaveCount(1)
  const first = await boxOf(page.getByLabel(/line component e/))
  await placeByKeyboard(page, 'Place Line')
  await expect(page.getByLabel(/line component e/)).toHaveCount(2)
  // Both keyboard placements land on the band origin, so the two rules already
  // occupy one spot. Confirm that rather than assume it.
  const second = await boxOf(page.getByLabel(/line component e/).last())
  expect(Math.round(second.x)).toBe(Math.round(first.x))
  expect(Math.round(second.y)).toBe(Math.round(first.y))
  // THE EXPECTED ANSWER COMES FROM THE TREE, NOT FROM THE FEATURE UNDER TEST.
  const order = await documentOrder(page)
  expect(order).toHaveLength(2)
  const lastInDocumentOrder = order[order.length - 1]
  expect(lastInDocumentOrder).toBeTruthy()
  for (let attempt = 0; attempt < 3; attempt++) {
    await deselect(page)
    await press(page, second.x + second.width / 2, second.y + second.height / 2)
    await expect(identity(page)).toHaveText(new RegExp(`^${lastInDocumentOrder} `))
  }
})

// THE Q3 ROW: PLACEMENT BEATS PADDING. With a palette kind armed, a click four
// pixels from an existing rule — inside that rule's padded region — must PLACE a
// component in the band, not select the neighbour. This is the same defect class
// `.canvas-component-echo` was made inert for, and the suppression rule is
// modelled on it; the pad takes pointer events again the moment the placement is
// disarmed, which the second half of this test measures.
test('an armed placement beats the padded region, which takes the pointer back afterwards', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await placeByKeyboard(page, 'Place Line')
  await expect(page.getByLabel(/line component e/)).toHaveCount(1)
  const drawn = await boxOf(page.getByLabel(/line component e/))
  expect(await padOf(page.getByLabel(/line component e/))).toBeGreaterThan(4)
  const beside = { x: drawn.x + drawn.width / 2, y: drawn.y + drawn.height + 4 }
  const ruleId = (await documentOrder(page))[0]
  expect(ruleId).toBeTruthy()

  await page.getByRole('button', { name: 'Place Rectangle' }).click()
  await press(page, beside.x, beside.y)
  // A rectangle was placed: the rule beside the press stole no pointerup. And
  // because a placed component is now the selected component, the rectangle is
  // what the inspector shows — the rule was not selected instead.
  const rect = page.getByLabel(/rect component e/)
  await expect(rect).toHaveCount(1)
  await expect(rect).toBeFocused()
  await expect(identity(page)).not.toHaveText(new RegExp(`^${ruleId} `))

  // PADDING RESTORED. Take the rectangle away — it covers the offset under test
  // — and press the same spot again with nothing armed. The pad answers.
  await page.keyboard.press('Delete')
  await expect(rect).toHaveCount(0)
  await deselect(page)
  await press(page, beside.x, beside.y)
  await expect(identity(page)).toHaveText(new RegExp(`^${ruleId} `))
})
