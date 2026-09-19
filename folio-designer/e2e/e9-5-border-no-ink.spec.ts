import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// EPIC 9/10 BOUNDARY GATE — ONE assertion, deliberately.
//
// The claim: a component whose border paints no ink must project no border
// fields, and the canvas must paint no border. That is the E9-5 repair
// (`border: {"edges": [], "color": "#ff0000"}` previously painted a full red
// border for a document that prints none, because BorderEdges' `omitempty`
// dropped the empty slice and App.tsx's `component.borderEdges ?? boxEdges`
// fell back to all four).
//
// DW-74 is exactly blind to this: the Go/TS wire test records the
// projection's top-level and CanvasFontChain key lists and NOT
// CanvasComponent's, so the per-component projected fields have no
// browser-side witness. Today the claim is asserted only by a Go-side test
// (TestABorderPaintingNoInkProjectsNoBorderFields).
//
// The document carries BOTH arms so the single assertion is non-vacuous:
// e1 declares a border that paints nothing, e2 declares one that paints.
// The expected value is therefore an exact list of one — absence proved
// against a control that is present in the same DOM.
//
// THIS IS NOT A BROWSER SUITE. Do not grow it.

const inertVsPainting = JSON.stringify({
  assets: {},
  bands: {
    content: {
      elements: [
        {
          id: 'e1', type: 'text', x: 12, y: 20, width: 200, height: 30, value: 'Inert',
          style: { border: { edges: [], color: '#ff0000' }, fontFamily: 'body', fontSize: 12 },
        },
        {
          id: 'e2', type: 'text', x: 12, y: 100, width: 200, height: 30, value: 'Painting',
          style: { border: { edges: ['bottom'], color: '#00ff00', width: 2 }, fontFamily: 'body', fontSize: 12 },
        },
      ],
    },
    pageFooter: { elements: [], height: 20 },
    pageHeader: { elements: [], height: 20 },
  },
  fonts: { body: ['Noto Sans', 'Noto Sans Thai', 'Noto Sans SC'] },
  locale: 'en',
  nextId: 3,
  page: { margin: { bottom: 36, left: 36, right: 36, top: 36 }, orientation: 'portrait', size: 'A4' },
  utcOffset: '+00:00',
  version: '1.0',
})

test('a border that paints no ink paints no border on the canvas', async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name: 'border.folio', mimeType: 'application/json', buffer: Buffer.from(inertVsPainting, 'utf8') })
  await expect(page.locator('.document-name')).toHaveText('border.folio')
  await expect(page.locator('[aria-label*="component e2"]').first()).toBeVisible()

  // THE assertion. Every border-painting box the canvas drew, keyed by the
  // component that owns it. Exactly one entry — e2's — may appear, and no
  // entry may be red.
  await expect.poll(() => page.evaluate(() => Array.from(document.querySelectorAll('.canvas-box')).map((box) => {
    const owner = box.closest('[aria-label]')?.getAttribute('aria-label') ?? '?'
    const style = getComputedStyle(box)
    const sides = (['Top', 'Right', 'Bottom', 'Left'] as const)
      .filter((side) => parseFloat(style.getPropertyValue(`border-${side.toLowerCase()}-width`)) > 0)
      .map((side) => `${side.toLowerCase()}=${style.getPropertyValue(`border-${side.toLowerCase()}-color`)}`)
    return `${owner.replace(/^(\w+) component (\w+).*$/, '$1 $2')} :: ${sides.join(',') || 'none'}`
  }).sort())).toEqual(['text e2 :: bottom=rgb(0, 255, 0)'])
})
