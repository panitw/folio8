import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'

test('the five closed palette choices can begin an accessible local placement', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  await expect(page.getByRole('button', { name: 'Place Text' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Place Image' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Place Table' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Place Line' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Place Rectangle' })).toBeVisible()
  const content = page.getByRole('region', { name: 'Content', exact: true })
  const priorTextCount = await content.getByRole('button', { name: /text component/ }).count()
  await page.getByRole('button', { name: 'Place Text' }).click()
  await content.press('Enter')
  await expect(content.getByRole('button', { name: /text component/ })).toHaveCount(priorTextCount + 1)
})

test('a palette pointer drag drops, selects, moves, resizes, and deletes through the local engine', async ({ page }) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const palette = page.getByRole('button', { name: 'Place Rectangle' })
  const content = page.getByRole('region', { name: 'Content', exact: true })
  const paletteBox = await palette.boundingBox()
  const contentBox = await content.boundingBox()
  if (!paletteBox || !contentBox) throw new Error('canvas placement targets were not painted')
  await page.mouse.move(paletteBox.x + paletteBox.width / 2, paletteBox.y + paletteBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(contentBox.x + 120, contentBox.y + 120)
  await page.mouse.up()

  const component = page.getByLabel(/rect component e/)
  await expect(component).toBeVisible()
  const box = await component.boundingBox()
  if (!box) throw new Error('created component was not painted')
  await page.mouse.move(box.x + 8, box.y + 8)
  await page.mouse.down()
  await page.mouse.move(box.x + 20, box.y + 14)
  await page.mouse.up()
  await expect(page.getByText('Unsaved local changes')).toBeVisible()

  const handle = page.getByRole('button', { name: /Resize e/ })
  await expect(handle).toBeVisible()
  const handleBox = await handle.boundingBox()
  if (!handleBox) throw new Error('resize hit target was not painted')
  await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(handleBox.x + handleBox.width / 2 + 12, handleBox.y + handleBox.height / 2 + 8)
  await page.mouse.up()

  await component.focus()
  await page.keyboard.press('Delete')
  await expect(component).toHaveCount(0)
})

type ImageDropTarget = 'midpoint' | 'boundary' | 'footer-boundary'

const dragImageToBand = async (page: Page, band: Locator, at: ImageDropTarget) => {
  await band.scrollIntoViewIfNeeded()
  const palette = page.getByRole('button', { name: 'Place Image', exact: true })
  const paletteBox = await palette.boundingBox()
  const bandBox = await band.boundingBox()
  if (!paletteBox || !bandBox) throw new Error('image placement targets were not painted')
  const target = {
    x: bandBox.x + 120,
    y: bandBox.y + (at === 'midpoint' ? bandBox.height / 2 : at === 'footer-boundary' ? 1 : bandBox.height - 1),
  }
  expect(target.y).toBeGreaterThan(0)
  expect(target.y).toBeLessThan(page.viewportSize()!.height)
  await page.mouse.move(paletteBox.x + paletteBox.width / 2, paletteBox.y + paletteBox.height / 2)
  await page.mouse.down()
  await expect(palette).toHaveAttribute('aria-pressed', 'true')
  await page.mouse.move(target.x, target.y, { steps: 5 })
  await page.mouse.up()
  await expect(palette).toHaveAttribute('aria-pressed', 'false')
}

const expectSelectedImage = async (page: Page, band: Locator, at: ImageDropTarget) => {
  const image = band.getByRole('button', { name: /image component/ })
  await expect(image).toHaveCount(1)
  await expect(page.getByRole('button', { name: /image component/ })).toHaveCount(1)
  await expect(image).toHaveClass(/canvas-component-selected/)
  await expect(page.getByRole('textbox', { name: 'Width (pt)', exact: true })).toHaveValue('96')
  await expect(page.getByRole('textbox', { name: 'Height (pt)', exact: true })).toHaveValue(at === 'midpoint' ? '30' : at === 'footer-boundary' ? '40' : '6')
  await expect(page.getByRole('textbox', { name: 'Y (pt)', exact: true })).toHaveValue(at === 'midpoint' ? '30' : at === 'footer-boundary' ? '0' : '54')
}

const openTwoSheetFixture = async (page: Page, footerHeight: number) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const fixture = JSON.parse(readFileSync(new URL('../public/templates/starter.folio', import.meta.url), 'utf8'))
  fixture.bands.pageFooter.height = footerHeight
  // Visible content on both sheets makes the repeated bands reachable.
  fixture.bands.content.elements = [0, 800].map((y, index) => ({ id: `e${index + 1}`, type: 'rect', x: 0, y, width: 72, height: 24, style: { background: '#000000' } }))
  fixture.nextId = 3
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name: 'repeated-bands.folio', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(fixture)) })
}

test.describe('image palette placement in repeating bands', () => {
  test.use({ viewport: { width: 1440, height: 1000 } })

  for (const at of ['midpoint', 'boundary'] as const) {
    test(`a pointer drop at the header ${at} creates, selects, and disarms without resizing the band`, async ({ page }) => {
      await openWorkspace(page)
      await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
      const header = page.getByRole('region', { name: 'Page Header', exact: true })
      const height = await header.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))

      // The boundary target is one display pixel above the content band,
      // inside the resize strip's three-pixel overlap on the header side.
      await dragImageToBand(page, header, at)
      await expectSelectedImage(page, header, at)
      await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 2/)
      expect(await header.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))).toBe(height)
      await expect(page.locator('.band-boundary-proposal')).toHaveCount(0)

      if (at === 'boundary') {
        // The same strip must accept a resize after the palette disarms.
        // Press its left overhang to avoid the new image's selection handles.
        const handle = page.getByRole('button', { name: 'Resize the page header', exact: true })
        await handle.scrollIntoViewIfNeeded()
        const box = await handle.boundingBox()
        if (!box) throw new Error('the header boundary was not painted')
        const x = box.x + 10
        const y = box.y + box.height / 2
        await page.mouse.move(x, y)
        await page.mouse.down()
        await page.mouse.move(x, y + 12)
        await page.mouse.move(x, y + 24)
        await expect(page.locator('.band-boundary-proposal')).toHaveCount(1)
        await page.mouse.up()
        await expect.poll(() => header.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))).not.toBe(height)
        await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 3/)
      }
    })
  }

  test('a pointer drop inside the footer resize strip fits the image without resizing the band', async ({ page }) => {
    await openWorkspace(page)
    await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
    const footer = page.getByRole('region', { name: 'Page Footer', exact: true })
    const height = await footer.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))

    // One display pixel below the footer top is inside the resize strip.
    await dragImageToBand(page, footer, 'footer-boundary')
    await expectSelectedImage(page, footer, 'footer-boundary')
    await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 2/)
    expect(await footer.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))).toBe(height)
    await expect(page.locator('.band-boundary-proposal')).toHaveCount(0)
  })

  for (const bandName of ['Page Header', 'Page Footer'] as const) {
    test(`an image pointer drop on the repeated ${bandName} uses the template band drop semantics`, async ({ page }) => {
      // Standardize both bands at 60pt for the shared geometry assertions.
      await openTwoSheetFixture(page, 60)
      const repeated = page.getByRole('region', { name: `${bandName} on page 2 of 2`, exact: true })
      await expect(repeated).toHaveCount(1)
      const height = await repeated.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))

      await dragImageToBand(page, repeated, 'boundary')
      const home = page.getByRole('region', { name: `${bandName} on page 1 of 2`, exact: true })
      await expectSelectedImage(page, home, 'boundary')
      await expect(page.getByRole('textbox', { name: 'X (pt)', exact: true })).toHaveValue('120')
      await expect(repeated.locator('.canvas-component-image')).toHaveCount(1)
      expect(await repeated.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))).toBe(height)
    })
  }

  for (const key of ['Enter', 'Space']) {
    test(`${key} places and selects a fitted image in the repeated 40pt footer and disarms`, async ({ page }) => {
      await openTwoSheetFixture(page, 40)
      const repeated = page.getByRole('region', { name: 'Page Footer on page 2 of 2', exact: true })
      await expect(repeated).toHaveCount(1)
      const height = await repeated.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))
      const palette = page.getByRole('button', { name: 'Place Image', exact: true })
      await palette.click()
      await expect(palette).toHaveAttribute('aria-pressed', 'true')
      await repeated.press(key)

      const home = page.getByRole('region', { name: 'Page Footer on page 1 of 2', exact: true })
      await expectSelectedImage(page, home, 'footer-boundary')
      await expect(page.getByRole('textbox', { name: 'X (pt)', exact: true })).toHaveValue('0')
      await expect(repeated.locator('.canvas-component-image')).toHaveCount(1)
      await expect(palette).toHaveAttribute('aria-pressed', 'false')
      expect(await repeated.evaluate((band) => (band as HTMLElement).style.getPropertyValue('--band-height'))).toBe(height)
    })
  }
})
