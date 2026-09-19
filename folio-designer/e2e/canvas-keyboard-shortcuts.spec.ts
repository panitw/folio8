import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'

type BandName = 'Page Header' | 'Content' | 'Page Footer'

test.use({ viewport: { width: 1440, height: 1000 } })

const band = (page: Page, name: BandName): Locator => page.getByRole('region', { name, exact: true })
const componentsIn = (scope: Page | Locator): Locator => scope.locator('[data-component-id]')
const idsOf = (locator: Locator): Promise<string[]> => locator.evaluateAll((elements) => elements.map((element) => element.getAttribute('data-component-id') ?? ''))
const selectedIds = (page: Page): Promise<string[]> => idsOf(page.locator('.canvas-component-selected[data-component-id]'))

async function open(page: Page): Promise<void> {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
}

async function place(page: Page, name: BandName): Promise<void> {
  const before = await componentsIn(band(page, name)).count()
  await page.getByRole('button', { name: 'Place Rectangle' }).click()
  await band(page, name).press('Enter')
  await expect(componentsIn(band(page, name))).toHaveCount(before + 1)
}

async function selectAllIn(page: Page, name: BandName): Promise<string[]> {
  const ids = await idsOf(componentsIn(band(page, name)))
  // Placement puts every rectangle at the band origin, so they overlap; the
  // last one placed is on top and is the one a click can reach.
  await componentsIn(band(page, name)).last().click()
  await page.keyboard.press('ControlOrMeta+a')
  await expect.poll(() => selectedIds(page).then((selected) => selected.sort())).toEqual([...ids].sort())
  return ids
}

test('copy and paste a group, then one undo removes every pasted copy', async ({ page }) => {
  await open(page)
  await place(page, 'Content')
  await place(page, 'Content')
  const content = band(page, 'Content')
  const originals = await selectAllIn(page, 'Content')
  expect(originals).toHaveLength(2)

  await page.keyboard.press('ControlOrMeta+c')
  await page.keyboard.press('ControlOrMeta+v')
  await expect(componentsIn(content)).toHaveCount(4)
  const pasted = (await idsOf(componentsIn(content))).filter((id) => !originals.includes(id))
  expect(pasted).toHaveLength(2)
  await expect.poll(() => selectedIds(page).then((selected) => selected.sort())).toEqual([...pasted].sort())

  await page.getByRole('button', { name: /^Undo/ }).click()
  await expect(componentsIn(content)).toHaveCount(2)
  expect((await idsOf(componentsIn(content))).sort()).toEqual([...originals].sort())
})

test('Delete removes a drag-free group selection and one undo restores it', async ({ page }) => {
  await open(page)
  await place(page, 'Content')
  await place(page, 'Content')
  const content = band(page, 'Content')
  const originals = await selectAllIn(page, 'Content')

  await page.keyboard.press('Delete')
  await expect(componentsIn(content)).toHaveCount(0)
  await page.getByRole('button', { name: /^Undo/ }).click()
  await expect(componentsIn(content)).toHaveCount(2)
  expect((await idsOf(componentsIn(content))).sort()).toEqual([...originals].sort())
})

test('Select All selects exactly the components of the band last touched', async ({ page }) => {
  await open(page)
  await place(page, 'Page Header')
  await place(page, 'Content')
  await place(page, 'Page Footer')
  // Records whether the app prevented the browser's default (select every text
  // on the page). The app re-registers its window listener on each render, so
  // listener order is not fixed: read the flag after dispatch has finished.
  await page.evaluate(() => {
    const record = window as unknown as { selectAllPrevented?: boolean[] }
    record.selectAllPrevented = []
    window.addEventListener('keydown', (event) => { if (event.key.toLowerCase() === 'a' && (event.metaKey || event.ctrlKey)) setTimeout(() => record.selectAllPrevented!.push(event.defaultPrevented), 0) })
  })
  for (const name of ['Page Header', 'Content', 'Page Footer'] as const) {
    await test.step(name, async () => {
      await selectAllIn(page, name)
    })
  }
  await expect.poll(() => page.evaluate(() => (window as unknown as { selectAllPrevented: boolean[] }).selectAllPrevented)).toEqual([true, true, true])
})
