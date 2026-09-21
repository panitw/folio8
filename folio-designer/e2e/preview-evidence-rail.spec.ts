import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// STORY 13.3 — THE BROWSER WITNESS FOR THE EVIDENCE RAIL'S LAYOUT.
//
// EXECUTED LOCALLY. This file is not written as compile-only coverage and must
// not be reported as CI-executed: `origin/main` is live but no CI run of this
// suite has been observed by anyone, and a test that has never been executed is
// not coverage whatever the config says.
//
// ⚠ WHY A BROWSER TEST AT ALL, when 1,000-odd unit tests already read this
// screen's DOM. DW-289: `npm test` is vitest over jsdom, which APPLIES NO
// STYLESHEET. Story 13.2 shipped with every CSS claim guarded by nothing —
// eight mutations, including deleting a design token outright, left the unit
// suite green. The rail is this story's entire visual deliverable, so the half
// of it that lives in `App.css` needs a witness that can actually see a box.
//
// ⚠ THE PICKERS ARE NULLED even though this spec never saves: the rail carries
// `Save PDF`, and a stray activation on a headless Chromium that HAS the File
// System Access API emits no `filechooser` event at all and times out at 90s.
// Two specs shipped without this and did exactly that the first time anyone ran
// them.
const template = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/testdata/example/first-pdf.folio'))

// THE ENGINE VERSION IS READ FROM THE ENGINE, NOT TYPED HERE. This assertion
// was `toContainText('1.0.0')`, which is a correct claim about the rail that
// stops being true at every release — it reddened this suite on the 1.1.0
// bump, in a job whose purpose is finding DESIGNER regressions. What the test
// actually means is "the rail shows the version the engine reports", so it
// reads `folio8.Version` from the one file that declares it and a bump moves
// nothing here. The regex is anchored to the const declaration so a mention
// of a version in a comment above it cannot be picked up instead.
const engineVersion = (() => {
  const source = readFileSync(
    path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/version.go'),
    'utf8',
  )
  const match = /^const Version = "([^"]+)"$/m.exec(source)
  if (!match) throw new Error('could not read folio8.Version from folio-go/version.go')
  return match[1]
})()
const sampleData = Buffer.from('{"customer":{"name":"Ada"}}')

// `--panel-width` in `tokens.css`, and `.workbench`'s third grid column. The
// rail deliberately reuses the Inspector's column rather than minting a
// `--rail-width` for the mockup's 320px: the token names are pinned to
// DESIGN.md by exact equality, the shared `grid-template-columns` line is left
// free for the deferred PAGES rail, and a per-viewport width would need an
// `@media` rule this file's contract forbids. The 20px difference from the
// mockup is that decision, not a drift to close.
const PANEL_WIDTH = 300
// `.inspector-panel`'s own `border-left: var(--hairline)`, which the rail sits
// inside; `box-sizing: border-box` is global, so the column is 300 and its
// content box is one pixel narrower.
const HAIRLINE = 1

test('the evidence rail occupies the inspector column, with the whole hash wrapped and the actions at its foot', async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const templateChooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await templateChooser).setFiles({ name: 'statement.folio', mimeType: 'application/json', buffer: template })
  await expect(page.locator('.document-name')).toHaveText('statement.folio')
  const sampleChooser = page.waitForEvent('filechooser')
  await page.getByRole('tab', { name: 'DATA' }).click()
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await sampleChooser).setFiles({ name: 'sample.json', mimeType: 'application/json', buffer: sampleData })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()

  await page.getByRole('button', { name: 'PREVIEW' }).click()
  // The REAL Go engine, the REAL wasm render and the REAL PDF.js admission —
  // and the digest below is the one the browser recomputed over the bytes it is
  // holding before it would install them at all.
  await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible({ timeout: 60_000 })

  const inspector = page.getByRole('complementary', { name: 'Inspector' })
  const rail = page.getByLabel('Render evidence')
  await expect(rail).toBeVisible()

  const box = async (locator: ReturnType<typeof page.locator>) => {
    const measured = await locator.boundingBox()
    expect(measured, 'the element must be laid out to be measured').not.toBeNull()
    return measured!
  }

  // ---- PLACEMENT AND WIDTH ----
  const inspectorBox = await box(inspector)
  const railBox = await box(rail)
  expect(Math.round(inspectorBox.width), 'the inspector column is `--panel-width`').toBe(PANEL_WIDTH)
  expect(Math.round(railBox.width), 'the rail fills that column, inside its border').toBe(PANEL_WIDTH - HAIRLINE)
  expect(Math.round(railBox.x)).toBe(Math.round(inspectorBox.x + HAIRLINE))
  // It is IN the column, not beside it: the rail's box lies inside the aside's.
  expect(railBox.y).toBeGreaterThanOrEqual(inspectorBox.y - 1)
  expect(railBox.y + railBox.height).toBeLessThanOrEqual(inspectorBox.y + inspectorBox.height + 1)
  // And the preview page area is to its LEFT, so nothing overlaps the document.
  const preview = await box(page.getByRole('main', { name: 'Preview region' }))
  expect(preview.x + preview.width).toBeLessThanOrEqual(railBox.x + 1)

  // ---- THE HASH: ALL 64 CHARACTERS, ON TWO LINES, IN ITS OWN BLOCK ----
  const hash = page.getByLabel('Output hash')
  const value = hash.locator('.rail-hash-value')
  await expect(value).toHaveText(/^[a-f0-9]{64}$/)
  const lines = value.locator('.rail-hash-line')
  await expect(lines).toHaveCount(2)
  const first = await box(lines.nth(0))
  const second = await box(lines.nth(1))
  // WRAPPED, MEASURED: the second line is BELOW the first, not beside it. A
  // single-line hash — the failure this is here to catch — would put them on
  // one baseline or produce one element.
  expect(second.y).toBeGreaterThan(first.y + first.height - 1)
  // The block is inside the rail's width, so the digest is readable rather than
  // clipped or overflowing the column.
  const valueBox = await box(value)
  expect(valueBox.x).toBeGreaterThanOrEqual(railBox.x)
  expect(valueBox.x + valueBox.width).toBeLessThanOrEqual(railBox.x + railBox.width + 1)
  await expect(hash).toContainText('Historical producer digest')
  await expect(hash).toContainText('Byte-identical across darwin/arm64, linux/amd64, linux/arm64 and js/wasm')
  await expect(hash).not.toContainText('Matches native render')

  // ---- THE RENDER BLOCK, FROM THE RENDER THAT ACTUALLY HAPPENED ----
  const facts = page.getByLabel('Render facts')
  await expect(facts.locator('.rail-fact')).toHaveCount(5)
  await expect(facts).toContainText('engine')
  await expect(facts).toContainText(engineVersion)
  await expect(facts).toContainText('wasm · in browser')
  // ⚠ NON-ZERO, DELIBERATELY (review P6). `\d+ ms` matches `0 ms`, which is
  // also what the field reads when nothing measured it at all — and this is the
  // ONLY executed path through `folio-go/wasm/cmd/engine/main.go`'s two new
  // fields: that file is `//go:build js && wasm`, so no Go test compiles it,
  // here or in CI. A dropped `ElapsedMs: rendered.ElapsedMs` is visible nowhere
  // else. A five-page shipped-font render through real wasm does not finish
  // inside a millisecond.
  await expect(facts).toContainText(/elapsed\s*(?:[1-9]\d*\s*ms|\d+(?:\.\d+)?\s*s)/)

  // ---- THE ACTION ROW SITS AT THE RAIL'S FOOT ----
  const actions = await box(page.getByLabel('Render actions'))
  const diagnostics = await box(page.getByLabel('Diagnostics summary'))
  expect(actions.y).toBeGreaterThan(diagnostics.y)
  expect(Math.round(actions.y + actions.height)).toBeLessThanOrEqual(Math.round(railBox.y + railBox.height) + 1)
  const rerender = await box(page.getByRole('button', { name: 'Re-render' }))
  const save = await box(page.getByRole('button', { name: 'Save PDF' }))
  // PAIRED ON ONE ROW, sharing the width: same top, side by side.
  expect(Math.round(rerender.y)).toBe(Math.round(save.y))
  expect(rerender.x + rerender.width).toBeLessThanOrEqual(save.x + 1)

  // ---- DW-281: THE CONTROLS DO NOT VANISH WITH THE TAB (an owner request) ----
  await page.getByRole('tab', { name: 'DATA' }).click()
  await expect(page.getByRole('button', { name: 'Re-render' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save PDF' })).toBeVisible()
  await expect(page.getByLabel('Output hash')).toBeVisible()
  // Reachable and operable by keyboard alone from the tab the author is on.
  await page.getByRole('button', { name: 'Re-render' }).focus()
  await expect(page.getByRole('button', { name: 'Re-render' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible({ timeout: 60_000 })
  await expect(page.getByRole('alert')).toHaveCount(0)
})
