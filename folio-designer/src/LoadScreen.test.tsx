import { render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LoadScreen } from './LoadScreen'
import { formatMiB, type S1Payload, type S1Row } from './release-payload'

const payload: S1Payload = { version: 1, releaseId: 'a'.repeat(64), pageId: 'b'.repeat(64), unit: 'MiB', decimals: 2, cachedBytes: 100, assetCount: 10, cacheAssets: ['/index.html', '/engine', '/latin', '/thai', '/cjk', '/a', '/b', '/c', '/d', '/e'].map((assetUrl) => ({ assetUrl, bytes: 10 })), rows: [
  { id: 'engine', label: 'Engine', delivery: 'cached-asset', assetUrl: '/engine', bytes: 10, sha256: 'a'.repeat(64) }, { id: 'latin-font', label: 'Latin font', delivery: 'cached-asset', assetUrl: '/latin', bytes: 10, sha256: 'a'.repeat(64) }, { id: 'thai-font', label: 'Thai font', delivery: 'cached-asset', assetUrl: '/thai', bytes: 10, sha256: 'a'.repeat(64) }, { id: 'cjk-font', label: 'CJK font', delivery: 'cached-asset', assetUrl: '/cjk', bytes: 10, sha256: 'a'.repeat(64) }, { id: 'thai-dictionary', label: 'Thai dictionary', delivery: 'embedded-in-engine', assetUrl: '/engine', bytes: 5, sha256: 'a'.repeat(64) },
] }
describe('honest first-run load screen', () => {
  it('uses named live numeric progress and never invents a second Thai request', () => {
    render(<LoadScreen lifecycle={{ state: 'caching', cacheReady: false, verifiedAssetUrls: ['/engine'] }} payload={payload} engineState="waiting" onRetry={vi.fn()} />)
    expect(screen.getByRole('status', { name: 'Offline preparation status' })).toHaveTextContent('Caching verified assets')
    expect(screen.getByRole('progressbar', { name: 'Verified offline cache progress' })).toHaveAttribute('aria-valuenow', '10')
    expect(screen.getByLabelText('Offline payload manifest')).toHaveTextContent('embedded in engine; no second request')
    expect(screen.queryByText(/spinner/i)).not.toBeInTheDocument()
  })
  it('uses active and failed words before colour and never calls an unmarked cache complete', () => {
    render(<LoadScreen lifecycle={{ state: 'caching', cacheReady: false, verifiedAssetUrls: ['/engine', '/latin', '/thai', '/cjk', '/a', '/b', '/c', '/d', '/e'], activeAssetUrl: '/index.html' }} payload={payload} engineState="waiting" onRetry={vi.fn()} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '90')
    expect(screen.getByLabelText('Offline payload manifest')).toHaveTextContent('✓')
    expect(screen.getByText(/10 release assets/)).toBeInTheDocument()
  })
  // THE DISPLAYED TOTAL IS THE SUM OF `cacheAssets`, NEVER THE SUM OF `rows`
  // (Story 11.1, AC3, D-11.1.8).
  //
  // Non-additivity is already TRUE by three independent mechanisms, and
  // asserted by none of them at the surface an author reads. `parseS1Payload`
  // rejects `cached-bytes-mismatch` unless `cachedBytes` equals the
  // `cacheAssets` sum, so rows contribute nothing structurally; this component
  // reduces `verified` over `payload.cacheAssets`; and the generator computes
  // `s1VisibleBytes` from the cached rows BEFORE appending the dictionary. None
  // of the three would stop someone rewriting the total below as a row sum, and
  // every existing test would stay green if they did — `parseS1Payload` never
  // sees the UI.
  //
  // WHY IT MATTERS MORE AFTER THIS STORY, NOT LESS. `thai-dictionary` is now the
  // ONLY `embedded-in-engine` row among twelve: its bytes are inside the engine
  // wasm this release already counts, so adding it to the total would tell a
  // user they are downloading it twice. With twelve rows instead of five, a
  // row-sum total is a bigger lie, not a moot one.
  //
  // RED-PROVABLE BY CONSTRUCTION: the two sums are deliberately different and
  // that difference is asserted first, so this cannot pass by both answers
  // happening to agree. Replace `payload.cachedBytes` in LoadScreen.tsx with a
  // reduce over `payload.rows` and this test reads 16.00 MiB where the release
  // says 13.00 MiB.
  it('shows the cache-asset total and never the sum of the rows, which double-counts the embedded dictionary', () => {
    const MiB = 1024 * 1024
    const cachedRow = (id: S1Row['id'], label: S1Row['label'], assetUrl: string, bytes: number): S1Row => ({ id, label, delivery: 'cached-asset', assetUrl, bytes, sha256: 'a'.repeat(64) })
    const rows: readonly S1Row[] = [
      cachedRow('engine', 'Engine', '/engine', MiB),
      cachedRow('latin-font', 'Latin font', '/latin', MiB),
      cachedRow('thai-font', 'Thai font', '/thai', MiB),
      cachedRow('cjk-font', 'CJK font', '/cjk', MiB),
      cachedRow('noto-sans-bold-font', 'Noto Sans Bold', '/sans-bold', MiB),
      cachedRow('noto-sans-italic-font', 'Noto Sans Italic', '/sans-italic', MiB),
      cachedRow('noto-sans-bold-italic-font', 'Noto Sans Bold Italic', '/sans-bold-italic', MiB),
      cachedRow('noto-sans-thai-bold-font', 'Noto Sans Thai Bold', '/thai-bold', MiB),
      cachedRow('roboto-bold-font', 'Roboto Bold', '/roboto-bold', MiB),
      cachedRow('roboto-italic-font', 'Roboto Italic', '/roboto-italic', MiB),
      cachedRow('roboto-bold-italic-font', 'Roboto Bold Italic', '/roboto-bold-italic', MiB),
      // The one embedded row, and the whole reason the two sums differ. Its
      // bytes live inside the engine asset already counted above.
      { id: 'thai-dictionary', label: 'Thai dictionary', delivery: 'embedded-in-engine', assetUrl: '/engine', bytes: 5 * MiB, sha256: 'a'.repeat(64) },
    ]
    const cacheAssets = ['/index.html', '/engine', '/latin', '/thai', '/cjk', '/sans-bold', '/sans-italic', '/sans-bold-italic', '/thai-bold', '/roboto-bold', '/roboto-italic', '/roboto-bold-italic', '/spare'].map((assetUrl) => ({ assetUrl, bytes: MiB }))
    const cachedBytes = cacheAssets.reduce((total, asset) => total + asset.bytes, 0)
    const rowBytes = rows.reduce((total, row) => total + row.bytes, 0)
    // NON-VACUITY: if the two sums agreed, every assertion below would pass over
    // a component computing either one.
    expect(rowBytes, 'the fixture must make the two sums differ, or this test cannot tell them apart').not.toBe(cachedBytes)
    expect(formatMiB(cachedBytes)).toBe('13.00 MiB')
    expect(formatMiB(rowBytes)).toBe('16.00 MiB')

    const additive: S1Payload = { version: 1, releaseId: 'a'.repeat(64), pageId: 'b'.repeat(64), unit: 'MiB', decimals: 2, cachedBytes, assetCount: cacheAssets.length, cacheAssets, rows }
    render(<LoadScreen lifecycle={{ state: 'caching', cacheReady: false, verifiedAssetUrls: [] }} payload={additive} engineState="waiting" onRetry={vi.fn()} />)

    const numeric = screen.getByRole('progressbar', { name: 'Verified offline cache progress' })
    expect(numeric).toHaveAttribute('aria-valuemax', String(cachedBytes))
    const status = screen.getByText(/release assets/)
    expect(status, 'the displayed denominator must be the cache-asset total').toHaveTextContent(`of ${formatMiB(cachedBytes)} verified`)
    expect(status.textContent, 'a row-sum total would double-count the dictionary the engine already carries').not.toContain(formatMiB(rowBytes))
    // AND THE ROWS ARE STILL ITEMISED — twelve of them, each with its own
    // measured weight. The total ignoring `rows` is not the same claim as the
    // screen ignoring them (D-11.1.15).
    expect(screen.getByLabelText('Offline payload manifest').querySelectorAll('li')).toHaveLength(rows.length)
    expect(screen.getByLabelText('Offline payload manifest')).toHaveTextContent('Noto Sans Bold Italic')
  })

  // STORY 14.5 / AC3 + AC4 — THE BRAND IS THERE BEFORE LOADING FINISHES, AND
  // IT IS THE SAME DRAWING AT A SECOND SIZE.
  //
  // The `width` assertion is the executable half of "one component
  // parameterised by `size`": the document-bar half of this pair asserts 18
  // from the same component, so a second hand-written drawing here would have
  // to reproduce this geometry to pass, and `BrandMark.test.tsx` separately
  // asserts that only one production file draws it.
  //
  // The role sweep passes `hidden: true` for the same reason it does in
  // `App.test.tsx`: the default spelling stays green against a `role="img"`
  // added under a still-present `aria-hidden`, which is the mutation this
  // fence exists to catch.
  //
  // ⚠ THE EXPECTED TEXT IS `FOLIO8 / OFFLINE`, NOT `FOLIO8`. The mockup draws
  // `FOLIO8` alone here; shipped wording is a content decision no AC asks for,
  // so it is left as shipped and AC4 is read as "the mark adds no SECOND
  // announcement" rather than as a copy change.
  it('wears the same mark at the load screen size, announcing nothing of its own', () => {
    const { container } = render(<LoadScreen lifecycle={{ state: 'caching', cacheReady: false, verifiedAssetUrls: [] }} payload={payload} engineState="waiting" onRetry={vi.fn()} />)
    const lockup = container.querySelector('.load-brand .brand-lockup')
    expect(lockup, 'the load screen must carry the mark-and-word lockup').not.toBeNull()
    const svg = lockup!.querySelector('svg')
    expect(svg, 'the lockup must contain the inline mark').not.toBeNull()

    // ⚠ THE ROOT IS SWEPT ALONGSIDE ITS DESCENDANTS — see the twin of this
    // fence in `App.test.tsx`. A name on the LOCKUP escapes every
    // descendant-scoped query, and here it is a real regression rather than a
    // tidiness point: `role="img"` on the wrapper makes its children
    // presentational, so AT would announce "folio8" instead of the shipped
    // `FOLIO8 / OFFLINE`, silencing the very state this screen reports.
    const namedNodesIn = (root: Element) => [root, ...Array.from(root.querySelectorAll('*'))]
      .filter((node) => ['aria-label', 'aria-labelledby', 'role', 'title'].some((attribute) => node.hasAttribute(attribute)) || node.tagName.toLowerCase() === 'title')
      .map((node) => `${node.tagName.toLowerCase()}${node.getAttribute('role') ? `[role=${node.getAttribute('role')}]` : ''}`)

    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg!.getAttribute('class'), 'the .brand-mark rule reaches the SVG through this attribute alone').toBe('brand-mark')
    expect(svg, 'the load screen draws the mark at 22, the document bar at 18 — one component, one parameter').toHaveAttribute('width', '22')
    expect(svg).toHaveAttribute('height', '22')

    // THE GEOMETRY AS RENDERED HERE, AS LITERALS. This is what a hand-written
    // second drawing reds: transcribing the mockup's rounded 7×10 into a local
    // `<svg>` would satisfy the size assertions above and fail here, because
    // the shared rule gives 7.333×9.778 at this size.
    const rects = lockup!.querySelectorAll('rect')
    expect(rects).toHaveLength(2)
    expect(rects[0]).toHaveAttribute('width', '20.5')
    expect(rects[0]).toHaveAttribute('stroke-width', '1.5')
    expect(rects[1], 'the shared rule gives 7.333 — the mockup\'s rounded 7 would be a second drawing').toHaveAttribute('width', '7.333')
    expect(rects[1], 'the shared rule gives 9.778 — the mockup\'s rounded 10 would be a second drawing').toHaveAttribute('height', '9.778')
    expect(rects[1]).toHaveAttribute('fill', 'currentColor')

    expect(within(lockup as HTMLElement).queryAllByRole('img')).toEqual([])
    expect(within(lockup as HTMLElement).queryAllByRole('img', { hidden: true }), 'the mark must carry no role at all, not merely a role hidden from the tree').toEqual([])

    const announced = lockup!.cloneNode(true) as Element
    for (const hidden of Array.from(announced.querySelectorAll('[aria-hidden="true"]'))) hidden.remove()
    expect(announced.textContent?.replace(/\s+/g, ' ').trim(), 'the shipped wordmark, unchanged — the mark adds no second announcement').toBe('FOLIO8 / OFFLINE')
    expect(namedNodesIn(lockup!), 'nothing in the lockup — the wrapper INCLUDED — may contribute a name of its own').toEqual([])

    // (AC1) the mark is drawn BEFORE the word here too. The wordmark is a bare
    // text node on this screen, so document order is checked against the
    // lockup's first child rather than against a sibling element.
    expect(lockup!.firstElementChild, 'the mark is the first thing in the lockup; the word follows it').toBe(svg)
    expect(lockup!.textContent?.trim(), 'the word is the lockup\'s only text, and it comes after the mark').toBe('FOLIO8 / OFFLINE')
  })

  it('shows a keyboard retry only for a bounded failure', () => {
    const retry = vi.fn(); render(<LoadScreen lifecycle={{ state: 'unavailable', cacheReady: false, verifiedAssetUrls: [] }} payload={payload} engineState="waiting" onRetry={retry} />)
    expect(screen.getByRole('button', { name: 'Retry preparation' })).toHaveFocus()
  })
})
