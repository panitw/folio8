import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

const usableOfflineState = /^(Offline ready|Update available; current release remains usable)$/

// This package is compile-covered each story; execution is deferred to the
// Epic 5 D-000.4 boundary because it needs a real browser SW lifecycle.
test('the built application loads and commits page setup through the real Go worker', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByLabel('Document bar')).toBeVisible()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.getByRole('textbox', { name: 'Top margin (pt)' }).fill('37')
  await page.getByRole('button', { name: 'Apply page setup' }).click()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 2/)
})

test('a complete release reloads through the service worker while offline without a network failure', async ({ page, context }) => {
  const failures: string[] = []
  page.on('requestfailed', (request) => failures.push(request.url()))
  await openWorkspace(page)
  await page.reload() // the first installation must activate before it can control a reload
  await expect(page.getByTestId('offline-status')).toHaveText(usableOfflineState)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await page.evaluate(async () => { await document.fonts.ready })
  failures.length = 0
  await context.setOffline(true)
  await page.reload()
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect(page.getByTestId('offline-status')).toHaveText(usableOfflineState)
  expect(failures).toEqual([])
})
