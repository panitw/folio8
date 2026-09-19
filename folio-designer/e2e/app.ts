import { expect, type Page } from '@playwright/test'

// THE WORKSPACE LANDING, SHARED (spec-startup-templates, story 3).
//
// Every launch now opens the "New template" dialog once the engine is ready.
// The specs in this directory were written against a launch that landed
// straight on the starter canvas; Escape is exactly that canvas — no engine
// request, the starter still at revision 1 — so dismissing it here keeps every
// one of them meaning what it meant.
export async function openWorkspace(page: Page): Promise<void> {
  await page.goto('/')
  await dismissStartupDialog(page)
}

// THE DOCUMENT BAR'S NEW… THEN START BLANK (story 4): the dialog's Blank
// replaces the open document. On real edits New… warns first; Discard there.
export async function startBlankFromNew(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'New…' }).click()
  const dialog = page.getByRole('dialog', { name: 'New template' })
  const warning = page.getByRole('dialog', { name: 'Discard unsaved changes?' })
  await expect(dialog.or(warning)).toBeVisible()
  if (await warning.count() > 0) await warning.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(dialog.getByRole('button', { name: 'Blank', exact: true })).toBeFocused()
  await dialog.getByRole('button', { name: 'Start blank' }).click()
  await expect(dialog).toHaveCount(0)
}

// Waits for the dialog, and for its initial focus, before pressing Escape: the
// dialog's key handling is on the dialog, so a key pressed before focus lands
// inside it would reach the page instead.
export async function dismissStartupDialog(page: Page): Promise<void> {
  const dialog = page.getByRole('dialog', { name: 'New template' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Blank', exact: true })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
}
