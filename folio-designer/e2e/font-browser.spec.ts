import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// STORY 16.3 — THE FONT BROWSER, IN A REAL BROWSER.
//
// WHAT THIS FILE CAN AND CANNOT WITNESS, STATED RATHER THAN IMPLIED. Every
// assertion below is over the modal's own structure, its keyboard behaviour and
// its staged state — none of which needs a network. The two claims that DO need
// one — a specimen actually rendered in its family's fetched face, and a
// confirmed add reaching the document — are exercised by the by-hand run this
// story's Verification section records, because the harness this repository runs
// e2e under has no route to the upstream repository host and a test that
// silently asserts the degraded path would be worse than no test.
//
// THE NAMES ARE THE CONTRACT. Story 8.2's spec pins `Font family` as the family
// control's accessible name and says in its own comment that the name must not
// move. This file adds names; it renames nothing.

const openBrowser = async (page: import('@playwright/test').Page) => {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const content = page.getByRole('region', { name: 'Content', exact: true })
  await page.getByRole('button', { name: 'Place Text' }).click()
  await content.press('Enter')
  // PLACING THE COMPONENT IS ITSELF A COMMAND, so the document is at revision 2
  // before the modal has been opened at all. Anything asserting that the browser
  // left the document untouched has to compare against THIS, not against 1.
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 2/)
  await content.getByRole('button', { name: /text component/ }).last().click()
  // THE DOOR IS THE LAST ROW OF THE OPEN FAMILY DROPDOWN, where the design puts
  // it — reached by focusing the control the panel already had, never by a
  // keyboard shortcut. No shortcut is bound in this epic (D-16.R.33 R2), and no
  // hint glyph is drawn, so there is none to press and none to assert.
  await page.getByRole('combobox', { name: 'Font family' }).focus()
  await page.getByRole('button', { name: 'Add fonts…' }).click()
  return page.getByRole('dialog', { name: 'Font browser' })
}

test('the browser opens from the family control as the design\'s five-region modal', async ({ page }) => {
  const browser = await openBrowser(page)
  await expect(browser).toBeVisible()
  await expect(browser).toHaveAttribute('aria-modal', 'true')

  // Focus lands in the search box, so a keyboard-only author can type
  // immediately rather than tabbing in from wherever the panel left them.
  await expect(page.getByRole('textbox', { name: 'Search fonts' })).toBeFocused()

  // The rail's three groups, the results, and the footer.
  await expect(browser.getByRole('textbox', { name: 'Preview text' })).toBeVisible()
  await expect(browser.getByRole('slider', { name: 'Preview size in pixels' })).toBeVisible()
  await expect(browser.getByRole('button', { name: 'Thai sample text' })).toBeVisible()
  await expect(browser.getByRole('group', { name: 'Writing system' })).toBeVisible()
  await expect(browser.getByRole('group', { name: 'Category' })).toBeVisible()
  await expect(browser.getByRole('group', { name: 'Sort' })).toBeVisible()
  await expect(browser.getByRole('group', { name: 'Results view' })).toBeVisible()
  await expect(browser.getByRole('list', { name: 'Font families' })).toBeVisible()
  await expect(browser.getByRole('button', { name: 'Install on this machine' })).toBeDisabled()

  // THE HEADER IS ONE ROW AND SAYS NOTHING ABOUT COUNTS (Story 16.10). The
  // design's header is a fixed 46px row — title, search, sort, view, close — and
  // draws no paragraph under it, so the disclosure this file used to require is
  // gone. The mockup's own "web font library · 1,946 families" is still refused
  // on top of that: drawing nothing in that slot is the design, and a shorter
  // sentence of our own would be the same two-authorities defect.
  //
  // ABSENCE, AGAINST A POSITIVE CONTROL — the heading text that IS drawn.
  await expect(browser.getByRole('heading', { name: 'Font browser' })).toBeVisible()
  await expect(browser.getByText(/families you can add/)).toHaveCount(0)
  await expect(browser.getByText(/The list itself is a snapshot taken on/)).toHaveCount(0)
  await expect(browser.getByText(/variable file are not shown/)).toHaveCount(0)
  await expect(browser.getByText(/web font library/)).toHaveCount(0)

  // AND THE COUNT SURVIVES WHERE IT ALWAYS LIVED: the results toolbar, printed
  // by `resultLine` from the same number the paragraph quoted. Unfiltered and
  // paginated at 12, the real population takes `resultLine`'s SECOND arm —
  // `12 of 1304 matching families, out of 1304` — so the pattern admits both.
  await expect(browser.getByText(/\d+ of \d+ (?:matching )?families/)).toBeVisible()

  // THE HEADER IS THE 46px ROW THE DESIGN DRAWS. This is the one claim no jsdom
  // test can make — it needs layout — so it is made here.
  const header = browser.locator('.font-browser-header')
  await expect(header).toHaveCount(1)
  const box = await header.boundingBox()
  // NULL FIRST, SO A HIDDEN OR DETACHED HEADER NAMES ITSELF. Without this the
  // failure reads `undefined !== 46`, which points at the height and not at the
  // header having no box at all.
  expect(box, 'the header must be laid out before its height means anything').not.toBeNull()
  expect(box?.height).toBe(46)
  await expect(header.locator('p')).toHaveCount(0)
})

test('search, chips and sort narrow the results, and reset filters appears exactly while one is active', async ({ page }) => {
  const browser = await openBrowser(page)
  await expect(browser.getByRole('button', { name: 'reset filters' })).toHaveCount(0)

  await page.getByRole('textbox', { name: 'Search fonts' }).fill('sarabun')
  await expect(browser.getByRole('list', { name: 'Font families' }).getByText('Sarabun', { exact: true })).toBeVisible()
  await expect(browser.getByRole('button', { name: 'reset filters' })).toBeVisible()

  await browser.getByRole('button', { name: 'Clear the search' }).click()
  await browser.getByRole('button', { name: 'Thai coverage', exact: true }).click()
  // `exact` because the vocabulary is derived from the snapshot and it carries
  // both `Serif` and `Sans Serif`: a substring match would resolve to two chips.
  await browser.getByRole('button', { name: 'Serif category', exact: true }).click()
  await expect(browser.getByRole('button', { name: 'Sort by A – Z' })).toBeVisible()
  // The arm the port dropped: this product embeds one face per family, so a
  // style count is not a difference the author can act on (D-16.R.33 R3).
  await expect(browser.getByRole('button', { name: /Most styles/ })).toHaveCount(0)

  await browser.getByRole('button', { name: 'reset filters' }).click()
  await expect(browser.getByRole('button', { name: 'reset filters' })).toHaveCount(0)
})

test('an unmatched query gets the design\'s empty state, naming the query', async ({ page }) => {
  const browser = await openBrowser(page)
  await page.getByRole('textbox', { name: 'Search fonts' }).fill('zzzz no such family')
  await expect(browser.getByText('No families match “zzzz no such family”')).toBeVisible()
  await expect(browser.getByRole('list', { name: 'Font families' })).toHaveCount(0)
})

test('the Grid view draws the same families as cards', async ({ page }) => {
  const browser = await openBrowser(page)
  const rows = await browser.getByRole('list', { name: 'Font families' }).getByRole('listitem').count()
  await browser.getByRole('button', { name: 'Grid view' }).click()
  await expect(browser.getByRole('list', { name: 'Font families' }).getByRole('listitem')).toHaveCount(rows)
})

// BEHAVIOUR-CHANGED (Story 16.5): the dialog's action is now INSTALL, and its
// footer says so. A row this machine already holds — every local-tier family —
// reports `On this machine` and is not stageable at all, because installing it
// again would fetch bytes the store already carries; so the staging locator is
// narrowed to the rows that CAN be staged rather than to every row on the page.
test('staging several families states what is about to be installed, and Escape discards it', async ({ page }) => {
  const browser = await openBrowser(page)
  const list = browser.getByRole('list', { name: 'Font families' })
  const add = list.getByRole('button', { name: /^Install .* on this machine$/ })
  await add.nth(0).click()
  await add.nth(0).click()
  await expect(browser.getByText(/^2 families ready to install/)).toBeVisible()
  // THE FOOTER STATES ONE FACT ABOUT WHAT A FACE IS — one upright Regular per
  // family, no bold and no italic — and deliberately NOT where it goes. Story
  // 16.5 HAS NOW inverted the destination, and this line needed no edit for it,
  // which is what keeping destination language out of it bought. It also says
  // nothing about subsetting in either direction: this product DOES subset, at
  // PDF render over the glyphs the document uses, so "whole file, not subset"
  // was as false as the mockup's "subset latin+thai" and shipped briefly before
  // review caught it.
  //
  // THIS ASSERTION IS WHY THE BROWSER RUN IS NOT OPTIONAL. When the string was
  // corrected in `font-browser-model.ts`, this line still matched the old one —
  // and `test:e2e:compile` is `tsc --noEmit`, which cannot see inside a regex.
  // Only executing it in a browser failed.
  await expect(browser.getByText(/2 faces · one upright Regular each, no bold or italic/)).toBeVisible()
  await expect(browser.getByRole('button', { name: 'Install 2 on this machine' })).toBeEnabled()

  // ESCAPE DISCARDS THE STAGED SET AND LEAVES THE DOCUMENT UNTOUCHED. Nothing
  // was sent, so the revision has not moved off the one the placement produced.
  await page.keyboard.press('Escape')
  await expect(browser).toHaveCount(0)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 2/)
})

test('every family the browser lists has a specimen state it states in words', async ({ page }) => {
  const browser = await openBrowser(page)
  const rows = browser.getByRole('list', { name: 'Font families' }).getByRole('listitem')
  await expect(rows.first()).toBeVisible()
  // A ROW EITHER SHOWS THE SAMPLE SET IN ITS OWN FACE OR SAYS WHY IT IS NOT, AND
  // THE CLAIM IS ABOUT EVERY ROW. This addressed the specimen by CSS CLASS under
  // a header that says the names are the contract, and then asserted only the
  // first row — so eleven rows could have gone silent, or rendered the sample in
  // the panel's own typeface, with this green. Both halves are fixed: the states
  // are read as TEXT, which is what the author actually sees, and the loop is
  // over the whole page.
  //
  // In a harness with no route upstream every web-tier row takes the second
  // branch. That is the point rather than a limitation: the one thing the
  // contract forbids outright is a specimen drawn in a fallback while implying
  // it is the family, and this is the run where that would happen.
  const sample = 'Everyone has the right to freedom of thought'
  const count = await rows.count()
  expect(count).toBeGreaterThan(1)
  for (let index = 0; index < count; index++) {
    const row = rows.nth(index)
    const name = await row.getByRole('button').first().getAttribute('aria-label') ?? ''
    // FOUR STATES SINCE STORY 16.5, and every one of them must still NAME THE
    // FAMILY: a screen reader hearing twelve buttons all called `+ Install`
    // learns nothing, and the fourth state (`On this machine`) is the one most
    // likely to be added without an accessible name because it is inert.
    const family = /^(?:Install (.+) on this machine|Remove (.+) from the families to install|(.+) is in this template|(.+) is already on this machine)$/.exec(name)
    expect(family, `row ${index} must carry a family-named add control, not "${name}"`).not.toBeNull()
    const spoken = family?.[1] ?? family?.[2] ?? family?.[3] ?? family?.[4] ?? ''
    const text = await row.innerText()
    const states = text.includes(sample)
      || text.includes('ทุกคนมีสิทธิในเสรีภาพแห่งความคิด')
      || text.includes(`${spoken} cannot be shown set in itself`)
      || text.includes(`Fetching ${spoken} to set this specimen in it`)
    expect(states, `${spoken} states no specimen state at all: ${text}`).toBe(true)
  }
})
