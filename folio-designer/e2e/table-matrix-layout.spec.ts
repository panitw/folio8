import { expect, test, type Locator } from '@playwright/test'
import { openWorkspace } from './app.js'

// STORY 14.7 — WHAT jsdom CANNOT SEE, AND THEREFORE WHAT THE UNIT SUITE CANNOT
// CLAIM. jsdom applies no stylesheet and computes no layout, so four of this
// story's claims are unprovable there and every one of them is a claim about
// the thing an author actually looks at:
//
//   1. the seven-track grid needs no horizontal scrolling at any ordinary width,
//      and the column labels sit over the columns they name;
//   2. when the window really is too narrow, the MATRIX scrolls and the sheet
//      does not — the failure the retired `overflow-x: auto` used to prevent;
//   3. the width budget renders and moves with the numbers it reports; and
//   4. — the one that matters most — the ALIGN cell renders as ONE JOINED
//      SEGMENTED CONTROL rather than three bordered mono buttons.
//
// (4) IS THE SPECIFICITY HAZARD, OBSERVED. `.matrix-row button` is (0,1,1) and
// sets `border`, `border-radius`, `background`, `color`, `font` and
// `min-height`. `.property-segment` is (0,1,0) and LOSES to it, while
// `.property-segment[aria-pressed="true"]` is (0,2,0) and still WINS on
// background — the pressed fill inside the wrong border, half-broken and
// invisible to every unit test. `App.css` scopes the exclusion with
// `:not(.property-segment)`.
//
// ⚠ AND THE PROOF IS AN EQUALITY, NOT A MAGIC NUMBER. This assertion used to
// read `26px ≤ h < 28px`, standing in for five declarations the matrix rule
// would also have applied — and changing `.property-segment`'s own `min-height`
// to 27px would have kept it green while telling nobody anything. The claim is
// "literally the same control the inspector renders", so the measurement is
// against the INSPECTOR'S OWN Align group, live on the same page: if the
// cascade ever reaches one and not the other, they stop matching, whatever the
// numbers happen to be. Story 14.3 shipped a padded hit region that passed every
// unit test and did not work in a browser at all, which is why this file exists.
//
// ⚠ `boundingBox()` ONLY — never `getComputedStyle` or `getBoundingClientRect`.
// `canvas-authority-contract.test.ts` scans this corpus for those identifiers
// by name, and Playwright's box is the reading the rest of the e2e suite
// already uses (`brand-mark.spec.ts` records the same rule).

type Box = Readonly<{ x: number; y: number; width: number; height: number }>

async function box(locator: Locator, what: string): Promise<Box> {
  const measured = await locator.boundingBox()
  expect(measured, `${what} must be laid out to be measured`).not.toBeNull()
  return measured!
}

async function openEditorOverOneColumn(page: import('@playwright/test').Page): Promise<Locator> {
  await openWorkspace(page)
  await page.getByRole('button', { name: 'Place Table' }).click()
  await page.getByRole('region', { name: 'Content', exact: true }).click({ position: { x: 24, y: 24 } })
  await page.getByRole('button', { name: /table component/ }).click()
  await page.getByRole('button', { name: 'Configure columns' }).click()
  const dialog = page.getByRole('dialog', { name: 'Table Editor' })
  await expect(dialog.getByRole('spinbutton', { name: 'Total table width in points' })).toHaveValue('523.276')
  await expect(dialog.getByRole('grid', { name: 'Table columns' })).toBeVisible({ timeout: 12_000 })
  await expect(dialog.getByRole('combobox', { name: 'Binding for column 1' })).toBeEditable()
  return dialog
}

test('the seven-track matrix fits the dialog and its labels sit over their cells', async ({ page }) => {
  const dialog = await openEditorOverOneColumn(page)
  const grid = dialog.getByRole('grid', { name: 'Table columns' })

  // 1a — NO HORIZONTAL SCROLLING AT THE DEFAULT WIDTH. The grid's own box must
  // fit inside the sheet that holds it. Under the eleven-track rule the grid
  // was declared `min-width: 1320px` inside a sheet capped at `100vw - 80px`,
  // so this was false by construction at every viewport this suite runs at.
  const sheet = await box(dialog.locator('.table-editor'), 'the dialog sheet')
  const gridBox = await box(grid, 'the matrix')
  expect(gridBox.width).toBeLessThanOrEqual(sheet.width)
  const headerRow = await box(grid.getByRole('row').first(), 'the header row')
  expect(headerRow.width).toBeLessThanOrEqual(sheet.width)

  // 1b — THE LABELS SIT OVER THEIR CELLS, which is a separate claim from
  // fitting and the one a shared track list can silently lose. The header and
  // the data rows are SEPARATE grid containers: every track has to resolve to
  // the same size in each of them independently, and an `auto` track does not —
  // it sizes to its own container's content. Comparing widths against the sheet
  // cannot see that at all; comparing a header cell's x against its row cell's
  // x is exactly the drift it produces.
  const headers = grid.getByRole('columnheader')
  await expect(headers).toHaveCount(7)
  const cells = grid.getByRole('row').nth(1).getByRole('gridcell')
  await expect(cells).toHaveCount(7)
  for (let column = 0; column < 7; column++) {
    const label = await box(headers.nth(column), `columnheader ${column + 1}`)
    const cell = await box(cells.nth(column), `gridcell ${column + 1}`)
    expect(Math.abs(label.x - cell.x), `column ${column + 1}: the label must start where its cell starts`).toBeLessThan(1)
    expect(Math.abs(label.width - cell.width), `column ${column + 1}: the label must be as wide as its cell`).toBeLessThan(1)
  }

  // 3 — THE BUDGET RENDERS, AND IT MOVES WITH THE NUMBER IT REPORTS.
  const budget = dialog.getByRole('status', { name: 'Width budget' })
  await expect(budget).toBeVisible()
  const width = dialog.getByRole('spinbutton', { name: 'Total table width in points' })
  await width.fill('120')
  await width.press('Tab')
  await expect(budget).toContainText('Σ 120.0', { timeout: 12_000 })
  await expect(budget).toContainText('available')
})

test('the ALIGN cell is the inspector\'s own segmented control, measured against it', async ({ page }) => {
  const dialog = await openEditorOverOneColumn(page)

  // THE TWO RENDERINGS OF THE ONE CONTROL, side by side on one page: the
  // dialog's ALIGN cell, and the inspector's TYPOGRAPHY row behind the modal
  // (a selected table is `typographic`, so its Align group is laid out even
  // while the overlay covers it).
  const tableGroup = dialog.getByRole('group', { name: 'Cell alignment for column 1' })
  const inspectorGroup = page.getByRole('group', { name: 'Align', exact: true })
  await expect(tableGroup).toBeVisible()
  const tableSegments = tableGroup.getByRole('button')
  const inspectorSegments = inspectorGroup.getByRole('button')
  await expect(tableSegments).toHaveCount(3)
  await expect(inspectorSegments).toHaveCount(3)

  const tableBoxes = [await box(tableSegments.nth(0), 'table segment 1'), await box(tableSegments.nth(1), 'table segment 2'), await box(tableSegments.nth(2), 'table segment 3')]
  const inspectorBox = await box(inspectorSegments.nth(0), 'inspector segment 1')

  // EQUAL, not "under 28" (`toBeCloseTo(…, 0)` is |diff| < 0.5, which still
  // reds on the 2px the hazard produces while tolerating sub-pixel layout
  // noise). Height is the axis the two share — the widths are
  // container-dependent by design, because a `1fr` inspector cell and a 112px
  // matrix track are different amounts of room for the same control. Everything
  // the matrix rule would have imposed (a 28px min-height, a border, a radius, a
  // panel background and the mono font) moves the table's height off the
  // inspector's; nothing legitimate does.
  for (const [index, segment] of tableBoxes.entries()) {
    expect(segment.height, `segment ${index + 1} must be exactly as tall as the inspector's`).toBeCloseTo(inspectorBox.height, 0)
  }
  // NON-VACUITY: the shared height is a real control's height, not zero and not
  // the page. A comparison of two collapsed boxes would otherwise pass.
  expect(inspectorBox.height).toBeGreaterThan(16)
  expect(inspectorBox.height).toBeLessThan(48)
  const tableGroupBox = await box(tableGroup, 'the table align group')
  const inspectorGroupBox = await box(inspectorGroup, 'the inspector align group')
  expect(tableGroupBox.height).toBeCloseTo(inspectorGroupBox.height, 0)
  // The icons inside are the same drawing at the same size in both.
  const tableIcon = await box(tableSegments.nth(0).locator('svg.segment-icon'), 'the table segment icon')
  const inspectorIcon = await box(inspectorSegments.nth(0).locator('svg.segment-icon'), 'the inspector segment icon')
  expect(tableIcon.width).toBeCloseTo(inspectorIcon.width, 0)
  expect(tableIcon.height).toBeCloseTo(inspectorIcon.height, 0)

  // JOINED: the three sit flush against one another on one line, with no gap
  // between them and no vertical stagger, inside ONE frame rather than three.
  expect(Math.abs(tableBoxes[1]!.x - (tableBoxes[0]!.x + tableBoxes[0]!.width))).toBeLessThan(1)
  expect(Math.abs(tableBoxes[2]!.x - (tableBoxes[1]!.x + tableBoxes[1]!.width))).toBeLessThan(1)
  expect(Math.abs(tableBoxes[1]!.y - tableBoxes[0]!.y)).toBeLessThan(1)
  expect(Math.abs(tableBoxes[2]!.y - tableBoxes[0]!.y)).toBeLessThan(1)
  expect(tableGroupBox.height).toBeLessThanOrEqual(tableBoxes[0]!.height + 2)
  expect(tableGroupBox.width).toBeLessThanOrEqual(tableBoxes[0]!.width + tableBoxes[1]!.width + tableBoxes[2]!.width + 2)

  // And the pressed segment is a real, distinguishable state rather than a
  // border the matrix rule painted over it.
  await tableSegments.nth(1).click()
  await expect(tableSegments.nth(1)).toHaveAttribute('aria-pressed', 'true', { timeout: 12_000 })
  await expect(tableSegments.nth(0)).toHaveAttribute('aria-pressed', 'false')
})

// ⚠ THIS CASE IS UNREACHABLE AT THE SUITE'S DEFAULT SIZE. `playwright.config.ts`
// declares no viewport, so every other test in this repository runs at
// 1280x720, where the seven tracks fit with room to spare and the failure below
// simply cannot occur. It is set explicitly here.
test('at a narrow window the matrix scrolls and the sheet does not', async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 800 })
  const dialog = await openEditorOverOneColumn(page)
  const grid = dialog.getByRole('grid', { name: 'Table columns' })
  const sheetLocator = dialog.locator('.table-editor')
  const heading = dialog.locator('.table-editor-heading')

  // The seven tracks (HEADER ALIGN beside CELL ALIGN) demand more than the
  // sheet has once gaps and row padding count; the sheet is
  // `min(1440px, 100vw - 80px)` less its own padding, so at 900px the matrix
  // genuinely overflows. That is the precondition for anything below to mean
  // something.
  const sheet = await box(sheetLocator, 'the dialog sheet')
  const gridBox = await box(grid, 'the matrix')
  const headerRow = await box(grid.getByRole('row').first(), 'the header row')
  expect(sheet.width).toBeLessThanOrEqual(900)
  expect(gridBox.width).toBeLessThanOrEqual(sheet.width)
  expect(headerRow.width, 'the matrix must actually overflow, or this test proves nothing').toBeGreaterThan(gridBox.width + 1)

  // THE CLAIM: scrolling to reach the last column moves THE MATRIX and leaves
  // the heading where it is. Without `overflow-x: auto` on `.table-matrix` the
  // nearest scroll container is `.table-editor` itself, so the same gesture
  // drags the dialog's heading and its footer bar off to the left.
  const headingBefore = await box(heading, 'the dialog heading')
  // HOW FAR THE GESTURE SHOULD ACTUALLY MOVE IT, derived from boxes this test
  // has already taken. The overflow is `headerRow.width - gridBox.width` -- the
  // same quantity `scrollWidth - clientWidth` would report, obtained without
  // asking the browser to measure itself. AD-17's corpus scan
  // (`src/canvas-authority-contract.test.ts`) covers `e2e/` too, and waives
  // exactly one file that is not this one; `boundingBox()` and `mouse.wheel()`
  // are deliberately outside what it names.
  const overflow = headerRow.width - gridBox.width
  const expectedScroll = Math.min(220, overflow)
  await grid.hover()
  await page.mouse.wheel(220, 0)
  // `mouse.wheel` DISPATCHES the event and returns -- it does not wait for the
  // scroll to be applied. Measuring straight afterwards caught the matrix
  // mid-move at exactly 1px of a 220px gesture, and the claim read as "the
  // matrix did not scroll" when the matrix was scrolling correctly. The
  // product was right and the measurement was early. Both other wheel
  // assertions in this suite poll for exactly this reason
  // (`preview-navigation.spec.ts:146,174`); this one did not, and it was the
  // only claim in this file resting on a synthesised input device rather than
  // on layout alone.
  //
  // Polling also fixes the QUANTITY. The old threshold (`x < headerRow.x - 1`)
  // was satisfied by a SINGLE PIXEL, so a matrix that scrolled 1px out of 220
  // and stopped would have passed. The gesture asks for 220px and the matrix
  // can give `overflow`, so the settled position is the smaller of the two --
  // less 1px for fractional device pixel ratios.
  await expect.poll(async () => (await grid.getByRole('row').first().boundingBox())!.x, { timeout: 10_000 })
    .toBeLessThanOrEqual(headerRow.x - (expectedScroll - 1))
  const headerRowAfter = await box(grid.getByRole('row').first(), 'the header row after scrolling')
  const headingAfter = await box(heading, 'the dialog heading after scrolling')
  expect(headerRowAfter.x, 'the matrix must be the thing that scrolled').toBeLessThan(headerRow.x - 1)
  expect(Math.abs(headingAfter.x - headingBefore.x), 'the sheet must not scroll: the heading stays put').toBeLessThan(1)
  const footer = await box(dialog.locator('.table-editor-footer'), 'the footer bar')
  expect(footer.x).toBeGreaterThanOrEqual(sheet.x - 1)
  expect(footer.x + footer.width).toBeLessThanOrEqual(sheet.x + sheet.width + 1)
})
