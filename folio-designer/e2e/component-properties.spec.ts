import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// THIS FILE IS COMPILED, NEVER EXECUTED. `npm run test:e2e:compile` is
// `tsc --noEmit` over this directory; `npm run test:e2e` (Playwright) appears
// in no workflow and browser e2e remains deferred by D-000.4. So what follows
// is a typed witness that the roles, names and flows below EXIST in the
// source — not evidence that any of them work in a browser. Nothing here may
// be reported as verified behaviour.
// Compile-covered at the Epic 5 cadence; Go is the property-validation
// authority.
test('a selected component exposes committed properties and a mixed selection omits non-shared size', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const content = page.getByRole('region', { name: 'Content', exact: true })
  await page.getByRole('button', { name: 'Place Text' }).click()
  await content.press('Enter')
  const text = content.getByRole('button', { name: /text component/ }).last()
  await text.click()
  await expect(page.getByRole('textbox', { name: 'X (pt)' })).toBeVisible()
  await page.getByRole('textbox', { name: 'X (pt)' }).fill('12')
  await page.getByRole('textbox', { name: 'X (pt)' }).press('Enter')

  await page.getByRole('button', { name: 'Place Table' }).click()
  await content.click({ position: { x: 24, y: 96 } })
  const table = content.getByRole('button', { name: /table component/ }).last()
  await text.click()
  await table.click({ modifiers: ['Shift'] })
  await expect(page.getByRole('textbox', { name: 'Width (pt)', exact: true })).toHaveCount(0)
  await expect(page.getByText(/Table size and binding are not editable here/)).toBeVisible()
})

// RETIRED (Story 16.9): this witnessed the font-chain editor — Rename,
// Delete, Add entry, move/remove entry, and the disclosure button beside the
// family combobox that revealed it (`Edit font chains` / `Hide font chains`).
// `FontChainEditor.tsx` is deleted along with its render site, so none of
// those roles exist in the source any more for this typed witness to name.
// THE CHAIN DATA IS UNTOUCHED: a document's `fonts` map still carries its
// fallback chains exactly as before, and `engine-protocol.ts` still names
// the six chain commands the editor used to issue — only the UI that issued
// them is gone.
