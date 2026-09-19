import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// WHEN THIS FILE ACTUALLY RUNS. `npm run test:e2e` is its own CI job — it
// executes on every push to main and on every pull request, with no
// `continue-on-error` anywhere in the workflow — so this spec runs in a real
// Chromium there. Locally it is COMPILED at story cadence
// (`npm run test:e2e:compile`, which is `tsc --noEmit` over the spec files):
// that proves the spec still typechecks, which is not the same claim as the
// spec still passing, and DW-268 records the two days a broken roundtrip spec
// survived on exactly that difference. The older note here said the executable
// pass "remains Epic 6 boundary evidence under D-000.4", which stopped being
// true when the browser job was added.
//
// STORY 14.7 REWROTE THE ROVING WALK FOR THE MATRIX LATTICE. The matrix now
// draws seven columns — `#`, HEADER LABEL, BINDING, WIDTH, HEADER ALIGN,
// CELL ALIGN, FOOTER AGGREGATE — while the KEYBOARD lattice behind them is wider
// than seven, because each alignment control is three segments and the row's reorder/remove
// affordances are three more controls inside the `#` cell. Every control the
// eleven-column matrix could reach by arrow keys is still reachable by arrow
// keys (UX-DR25); what changed is which cell each one sits in.
test('table editor is a named keyboard-operable matrix', async ({ page }) => {
  await openWorkspace(page)
  await page.getByRole('button', { name: 'Place Table' }).click()
	await page.getByRole('region', { name: 'Content', exact: true }).click({ position: { x: 24, y: 24 } })
  await page.getByRole('button', { name: /table component/ }).click()
  await page.getByRole('button', { name: 'Configure columns' }).click()
  await expect(page.getByRole('button', { name: 'Add column' })).toBeVisible()
  // Add splits the full-width starter without a preparatory width edit.
  const starterWidth = page.getByRole('textbox', { name: 'Proportion for column 1' })
  await expect(starterWidth).toHaveValue('1')
  await expect(page.getByRole('textbox', { name: 'Header for column 1' })).toHaveValue('')
  await page.getByRole('button', { name: 'Add column' }).click()
  await expect(page.getByRole('textbox', { name: 'Header for column 2' })).toBeVisible()
  await expect(starterWidth).toHaveValue('1')
  await expect(page.getByRole('textbox', { name: 'Proportion for column 2' })).toHaveValue('1')
  await expect(page.getByRole('combobox', { name: 'Binding for column 2' })).toBeEditable()
  await page.getByRole('button', { name: 'Remove column 2' }).click()
  await expect(page.getByRole('textbox', { name: 'Header for column 2' })).toHaveCount(0)
  const grid = page.getByRole('grid', { name: 'Table columns' })
  await expect(grid).toBeVisible()
	await expect(grid).toHaveAttribute('aria-rowcount', '2')
	await expect(grid).toHaveAttribute('aria-colcount', '7')
	// The retired four are gone as COLUMNS and present as row affordances.
	await expect(page.getByRole('columnheader')).toHaveText(['#', 'HEADER LABEL', 'BINDING', 'PROPORTION', 'HEADER ALIGN', 'CELL ALIGN', 'FOOTER AGGREGATE'])
	await expect(page.getByRole('button', { name: 'Add column after column 1' })).toHaveCount(0)
	// The collection and the row alias are still edited here, and nowhere else.
	await expect(page.getByRole('combobox', { name: 'Root collection' })).toBeVisible()
	await expect(page.getByRole('textbox', { name: 'Row alias' })).toBeVisible()
	// The two read-outs this story added, stated rather than implied.
	await expect(page.getByRole('status', { name: 'Table scope' })).toContainText('band: content')
	await expect(page.getByRole('status', { name: 'Width budget' })).toContainText('Σ')
	await expect(page.getByRole('status', { name: 'Column summary' })).toContainText('1 column · 0 aggregates')
  const header = page.getByRole('textbox', { name: 'Header for column 1' })
  await header.focus()
	// The row-field input occupies its original lattice address.
	await page.keyboard.press('ArrowRight')
	await expect(page.getByRole('combobox', { name: 'Binding for column 1' })).toBeFocused()
	await page.keyboard.press('Alt+ArrowRight')
	await expect(page.getByRole('textbox', { name: 'Proportion for column 1' })).toBeFocused()
	// THE ALIGNMENT CONTROL IS THREE REACHABLE SEGMENTS, not one select: each
	// takes its own lattice position, so no segment becomes unreachable.
	// HEADER ALIGN comes first, then CELL ALIGN — both three-segment runs. Names
	// are matched EXACTLY: `Header align left for column 1` contains
	// `align left for column 1`, and Playwright's default match is a substring.
	for (const name of ['Header align left for column 1', 'Header align center for column 1', 'Header align right for column 1', 'Align left for column 1', 'Align center for column 1', 'Align right for column 1']) {
		await page.keyboard.press('ArrowRight')
		await expect(page.getByRole('button', { name, exact: true })).toBeFocused()
	}
	// End reaches the row's LAST ENABLED control. This column aggregates
	// nothing, so its source and its format do not exist and End stops at the
	// aggregate rather than landing on either hole.
	await page.keyboard.press('End')
	await expect(page.getByRole('combobox', { name: 'Footer aggregate for column 1' })).toBeFocused()
	await expect(page.getByRole('textbox', { name: 'Footer source for column 1' })).toHaveCount(0)
	await expect(page.getByRole('textbox', { name: 'Footer format for column 1' })).toHaveCount(0)
	// Home reaches the row's FIRST ENABLED control. A single column can move
	// neither earlier nor later, so both reorder affordances are disabled and
	// Home declines to land on either.
	await page.keyboard.press('Home')
	await expect(page.getByRole('button', { name: 'Remove column 1' })).toBeFocused()
	await expect(page.getByRole('button', { name: 'Move column 1 earlier' })).toBeDisabled()
	await expect(page.getByRole('button', { name: 'Move column 1 later' })).toBeDisabled()
	// THE FOOTER BAR CARRIES THE PAIR (Story 14.7b). The single `Close Table
	// Editor` moved out of the heading at Story 14.7 and has become `Cancel` /
	// `Done`: `Done` closes and keeps, `Cancel` discards this session's edits by
	// undoing exactly as many of them as the engine agreed changed the document.
	//
	// ⚠ SCOPED TO THE DIALOG AND MATCHED EXACTLY. `FontBrowser.tsx` ships a
	// `Cancel` of its own, and an unscoped, non-exact `getByRole` would resolve
	// against whichever `Cancel` the page happened to hold — passing on the wrong
	// one, or striking two and failing for a reason that has nothing to do with
	// this footer.
	const dialog = page.getByRole('dialog', { name: 'Table Editor' })
	await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible()
	await expect(dialog.getByRole('button', { name: 'Done', exact: true })).toBeVisible()
	// ⚠ AND ESCAPE IS `Done`, NOT `Cancel`. It closes and KEEPS — deliberately, and
	// stated here so the assertion below is not read as proving a discard. Cancel is
	// disabled above the engine's history bound, and an Escape that meant Cancel
	// would leave the dialog undismissable by keyboard in exactly that state.
	await page.keyboard.press('Escape')
	await expect(dialog).toHaveCount(0)
})
