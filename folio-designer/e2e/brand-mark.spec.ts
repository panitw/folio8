import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// STORY 14.5 — THE ONLY WITNESS FOR THE RENDERED BOX.
//
// jsdom parses no stylesheet and computes no layout, so the unit suite can say
// the `<svg>` carries `width="18"` but cannot say that an 18px square with a
// 1.837px stroke actually lays out at 18×18 in a real engine. This spec is the
// only place that claim is checked.
//
// ⚠ `boundingBox()` ONLY — never `getComputedStyle` or `getBoundingClientRect`
// in source text. Both are banned across the whole corpus by the prohibition
// scan in `canvas-authority-contract.test.ts`, whose one carve-out is
// `e9-5-border-no-ink.spec.ts`. Playwright's `boundingBox()` reads the box from
// the browser side without either spelling appearing here.

// Matches `e2e/application-shell.spec.ts`: Chromium ships the File System Access
// API, so the fallback adapter is selected before the app captures capabilities.
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => { Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined }) })
})

test('the document bar wears the brand mark at its declared 18px box', async ({ page }) => {
  await openWorkspace(page)
  const bar = page.getByLabel('Document bar')
  await expect(bar).toBeVisible()

  const mark = bar.locator('.brand-lockup svg.brand-mark')
  await expect(mark).toHaveCount(1)
  await expect(mark).toBeVisible()

  const box = await mark.boundingBox()
  expect(box).not.toBeNull()
  expect(box!.width).toBeCloseTo(18, 1)
  expect(box!.height).toBeCloseTo(18, 1)

  // THE ONLY ASSERTION IN THE TREE THAT RESOLVES THE COLOUR CASCADE.
  //
  // The unit suite pins that `.brand-mark { color: var(--color-brand) }` is
  // SPELLED in App.css; it cannot say the rule WINS. jsdom parses no stylesheet,
  // so a later `.document-bar svg { color: var(--color-ink-low) }` appended to
  // App.css leaves the whole unit suite green while the mark quietly stops being
  // cyan — measured during review: 452/452 still passed under exactly that edit.
  // `toHaveCSS` reads the resolved value from the browser and is the only thing
  // that notices.
  //
  // `--color-brand: #87F0FF` (tokens.css), which the engine reports as rgb. The
  // mark takes the LOGO's cyan, not `--color-select`'s: they are different
  // colours on purpose, so a regression that reroutes the mark through the
  // selection token lands here as a concrete rgb mismatch.
  // ⚠ `toHaveCSS` is NOT in the corpus prohibition list — only `getComputedStyle`
  // and `getBoundingClientRect` and friends are — so this is the permitted way
  // to ask, and neither banned spelling appears in this file.
  await expect(mark, 'the mark paints --color-brand, and no later rule outranks .brand-mark').toHaveCSS('color', 'rgb(135, 240, 255)')

  // The mark sits BEFORE the word, and the word is unchanged.
  const word = bar.locator('.brand-lockup .brand')
  await expect(word).toHaveText('Folio8')
  const wordBox = await word.boundingBox()
  expect(wordBox).not.toBeNull()
  expect(box!.x, 'the mark is drawn before the wordmark, not after it').toBeLessThan(wordBox!.x)
})

// THE 22px SITE IS NOT WITNESSED IN A BROWSER, AND THAT IS A STATED GAP RATHER
// THAN AN OVERSIGHT.
//
// The load screen renders only while `loadState && !engine` (`App.tsx:2174`),
// and under the dev server `engineMayStart` short-circuits `dev-bypass` to the
// "Starting local engine" screen instead — so `.load-brand` is never reliably on
// screen in a normal run. Holding it there means driving the service-worker
// cache lifecycle into `caching` and pinning it, which no spec in this directory
// does: `page.route` and request interception appear nowhere in the e2e corpus
// (measured), so it would be a new fixture pattern, and a racy one, invented for
// a single size assertion.
//
// WHAT COVERS THE 22px SITE INSTEAD: `LoadScreen.test.tsx` asserts the rendered
// width, the outer 20.5 box and the 7.333 × 9.778 inner block as literals, and
// `BrandMark.test.tsx` proves only one drawing and only two call sites exist. So
// the GEOMETRY at 22 is proven; only the fact that it LAYS OUT at 22 in a real
// engine rests on the shared component plus the 18px witness above.
