import { expect, test, type Locator, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'

type Position = { x: number; y: number }

const coordinate = (page: Page, axis: 'X' | 'Y'): Locator =>
  page.getByRole('textbox', { name: `${axis} (pt)`, exact: true })

async function positionOf(page: Page): Promise<Position> {
  return { x: Number(await coordinate(page, 'X').inputValue()), y: Number(await coordinate(page, 'Y').inputValue()) }
}

async function paintedPosition(component: Locator): Promise<Position> {
  const box = await component.boundingBox()
  if (!box) throw new Error('the selected component was not painted')
  return { x: box.x, y: box.y }
}

async function expectPosition(page: Page, component: Locator, position: Position, paint: Position): Promise<void> {
  // These fields display the real engine projection; no inspector draft is
  // edited here. Check the browser's painted box separately from that answer.
  await expect(coordinate(page, 'X')).toHaveValue(String(position.x))
  await expect(coordinate(page, 'Y')).toHaveValue(String(position.y))
  await expect.poll(() => paintedPosition(component)).toEqual(paint)
}

test.use({ viewport: { width: 1440, height: 1000 } })

for (const snap of ['on', 'off'] as const) {
  test(`keyboard nudges move the real component precisely with Snap ${snap} and preserve undo/redo`, async ({ page }) => {
    await openWorkspace(page)
    await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
    await expect(page.getByRole('button', { name: /^Snap on/ })).toHaveAttribute('aria-pressed', 'true')
    if (snap === 'off') await page.getByRole('button', { name: /^Snap on/ }).click()
    const snapButton = page.getByRole('button', { name: new RegExp(`^Snap ${snap}`) })

    await page.getByRole('button', { name: 'Place Rectangle' }).click()
    await page.getByRole('region', { name: 'Content', exact: true }).press('Enter')
    const component = page.getByLabel(/rect component e/)
    await expect(component).toBeFocused()
    await expect(page.getByLabel('Canvas zoom')).toHaveText('100%')
    const origin = await positionOf(page)
    expect(origin).toEqual({ x: 0, y: 0 })
    const initialPaint = await paintedPosition(component)

    // Settle each command before the next key. The first arrow reproduces the
    // default-grid no-op; later arrows start off-grid on both axes, and the
    // repeated right arrows prove that each move uses the returned geometry.
    const nudges = [
      ['ArrowRight', 1, 0],
      ['ArrowDown', 0, 1],
      ['ArrowRight', 1, 0],
      ['ArrowRight', 1, 0],
      ['Shift+ArrowRight', 10, 0],
      ['Shift+ArrowDown', 0, 10],
      ['ArrowLeft', -1, 0],
      ['ArrowUp', 0, -1],
      ['Shift+ArrowLeft', -10, 0],
      ['Shift+ArrowUp', 0, -10],
    ] as const
    for (const [key, dx, dy] of nudges) {
      await test.step(key, async () => {
        const before = await positionOf(page)
        const next = { x: before.x + dx, y: before.y + dy }
        await component.press(key)
        // At 100% zoom, one point occupies one CSS pixel.
        await expectPosition(page, component, next, { x: initialPaint.x + next.x, y: initialPaint.y + next.y })
        await expect(snapButton).toHaveAttribute('aria-pressed', String(snap === 'on'))
      })
    }

    const moved = await positionOf(page)
    const movedPaint = await paintedPosition(component)
    // The last step was ten points up, so undo must restore that precise
    // predecessor without consuming a placement or an earlier nudge.
    await page.getByRole('button', { name: /^Undo/ }).click()
    // History deliberately clears selection. Reselect the same component to
    // inspect its new projection after that reset has settled.
    await expect(page.getByText('Component properties require a selection.')).toBeVisible()
    await component.click()
    await expectPosition(page, component, { x: moved.x, y: moved.y + 10 }, { x: movedPaint.x, y: movedPaint.y + 10 })
    await page.getByRole('button', { name: /^Redo/ }).click()
    await expect(page.getByText('Component properties require a selection.')).toBeVisible()
    await component.click()
    await expectPosition(page, component, moved, movedPaint)
    await expect(snapButton).toHaveAttribute('aria-pressed', String(snap === 'on'))
  })
}
