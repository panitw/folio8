import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it, vi } from 'vitest'
import { coreTierBrotliBytes, declaredCacheAssetWarning, declaredCoreCacheAssetBounds, declaredCoreCacheByteCeiling, declaredCoreCacheByteWarning } from './offline-release-contract.mjs'
import { documentationFontHostFinding, reportCacheAssetApproach, reportCoreCacheByteApproach, templateAssetFinding } from './verify-offline-release.mjs'
import { FORBIDDEN_FONT_HOSTS } from './forbidden-font-hosts.mjs'
// THE TYPESCRIPT DECLARATION ITSELF, IMPORTED AS A VALUE. See the tie below for
// why this import is the point rather than a convenience.
import { cacheAssetApproachWarning, cacheAssetCeiling, coreCacheAssetCeiling, coreCacheAssetFloor, coreCacheByteApproachWarning, coreCacheByteCeiling } from '../src/release-payload'

// ---------------------------------------------------------------------------
// THE CACHE-ASSET APPROACH WARNING, EXECUTED (Story 11.1, D-11.1.10).
//
// THE GAP THIS CLOSES. The warning is the sole realization of an acceptance
// criterion — "the build warns that the cache-asset margin has fallen to 3" —
// and until this file the only test near it exercised the READER
// (`declaredCacheAssetWarning`, in `offline-release-contract.test.mjs`) and
// never the emission. The reader validates that the threshold sits inside
// [minimum, maximum]; raising `warnCacheAssets` from 56 to 64 stays inside that
// envelope, so it would have stopped the warning firing on a 61-asset release
// with every test in the repository still green. An acceptance criterion whose
// only realization is unexecuted code is an acceptance criterion nothing holds.
//
// WHY THE EMISSION IS OBSERVABLE AT ALL. `reportCacheAssetApproach` takes its
// `warn` sink as an option and returns the message it emitted (or `null`),
// so a test asserts on the emission itself rather than scraping stderr — and
// the return value is checked to BE the argument the sink received, so a
// function that returned the right string and printed nothing cannot pass.
// ---------------------------------------------------------------------------

const { warnCacheAssets, maximumCacheAssets } = declaredCacheAssetWarning()
const { warnCoreCacheBytes, maximumCoreCacheBytes } = declaredCoreCacheByteWarning()

// ---------------------------------------------------------------------------
// THE CORE TIER'S BYTE APPROACH WARNING, EXECUTED
// (spec-install-all-face-cuts story 3, owner-authorised at review).
//
// THE GAP THIS CLOSES is the one the block above closes for the COUNT, and it
// stood open in the more expensive half: the core tier's weight is the blocking
// download every first-time visitor waits for, it moves with every bundle
// change rather than with a decision, and its only signal was a hard build
// failure on an already-assembled release. At the time this landed the core
// tier stood at 97.8% of its ceiling.
// ---------------------------------------------------------------------------
describe('the core-tier byte approach warning', () => {
  it('reads the same threshold the TypeScript module declares, text reader against evaluated value', () => {
    expect(warnCoreCacheBytes, 'scripts/offline-release-contract.mjs reads `warnCoreCacheBytes` out of src/release-payload.ts as TEXT; this is the value that file actually evaluates to').toBe(coreCacheByteApproachWarning)
    expect(maximumCoreCacheBytes, 'and the ceiling it is measured against').toBe(coreCacheByteCeiling)
    // AND THE THRESHOLD MUST SIT INSIDE THE ENVELOPE IT WARNS ABOUT, or it is a
    // warning that can never fire.
    expect(warnCoreCacheBytes).toBeLessThan(maximumCoreCacheBytes)
    expect(warnCoreCacheBytes).toBeGreaterThan(0)
  })

  it('emits the warning AT the declared threshold, and names the remaining margin', () => {
    const warn = vi.fn()
    const message = reportCoreCacheByteApproach(warnCoreCacheBytes, { warn })
    expect(warn, 'a release standing exactly on the threshold is the first release the warning exists for').toHaveBeenCalledTimes(1)
    expect(message, 'the returned message must BE the emitted one; a reporter that returns a string and prints nothing warns nobody').toBe(warn.mock.calls[0][0])
    expect(message).toContain(`weighs ${warnCoreCacheBytes} Brotli bytes`)
    expect(message).toContain(`a declared ceiling of ${maximumCoreCacheBytes}`)
    // THE MARGIN IS THE POINT, for the reason it is the point in the count
    // warning: a number nobody prints is a number nobody watches.
    expect(message, 'the warning must state the REMAINING MARGIN, not just the weight').toContain(`the margin is ${maximumCoreCacheBytes - warnCoreCacheBytes}`)
    expect(message).toContain('`warnCoreCacheBytes`')
    // AND IT SAYS WHOSE DOWNLOAD THIS IS, which is what distinguishes it from
    // the release-total warning in a build log carrying both.
    expect(message).toContain('first-time visitor')
  })

  it('emits it for every weight ABOVE the threshold, with the margin each one actually has', () => {
    for (const bytes of [warnCoreCacheBytes + 1, Math.floor((warnCoreCacheBytes + maximumCoreCacheBytes) / 2), maximumCoreCacheBytes]) {
      const warn = vi.fn()
      const message = reportCoreCacheByteApproach(bytes, { warn })
      expect(warn, `a core tier of ${bytes} bytes is over the threshold ${warnCoreCacheBytes} and must warn`).toHaveBeenCalledTimes(1)
      expect(message, `the margin at ${bytes} of ${maximumCoreCacheBytes}`).toContain(`the margin is ${maximumCoreCacheBytes - bytes}`)
    }
  })

  it('is SILENT below the threshold, and says so by returning null rather than by printing nothing', () => {
    for (const bytes of [warnCoreCacheBytes - 1, Math.floor(warnCoreCacheBytes / 2), 1, 0]) {
      const warn = vi.fn()
      expect(reportCoreCacheByteApproach(bytes, { warn }), `a core tier of ${bytes} bytes is under the threshold and must not warn`).toBeNull()
      expect(warn).not.toHaveBeenCalled()
    }
  })

  // AND THE DECLARATION ITSELF IS HELD COHERENT, on the fixture idiom the
  // asset-count reader's failure cases use: a threshold above the ceiling is a
  // warning that could never fire, and the fault is named as the declaration's
  // rather than any release's.
  it('refuses a threshold above the ceiling, naming the declaration and not a release', () => {
    const fixture = 'const minimumCoreCacheAssets = 30\nconst maximumCoreCacheAssets = 30\nconst maximumCoreCacheBytes = 6553600\nconst warnCoreCacheBytes = 6553601\n'
    expect(() => declaredCoreCacheByteWarning(fixture, 'the injected release-payload source')).toThrow(/could never fire/)
  })

  // WHETHER THE REAL RELEASE IS ALREADY OVER THE THRESHOLD IS NOT ASKED HERE,
  // AND DELIBERATELY SO. It was, for one commit, by reading
  // `dist/offline-release-manifest.json` — and that made it the ONLY test in
  // this suite depending on an artifact the suite does not build: `npm test` is
  // `build:wasm && vitest run`, so the manifest is present only when someone
  // happened to run `npm run build` first. A guard that reds on a clean
  // checkout and greens on a dirty one measures the developer, not the release.
  //
  // The claim itself is not lost, and is made somewhere that always has the
  // manifest: `verifyOfflineRelease` calls `reportCoreCacheByteApproach` on the
  // real release (verify-offline-release.mjs:470), inside `npm run build`, so a
  // core tier over the threshold warns on EVERY build with its remaining
  // margin. What is tested here is the mechanism that warning is built from —
  // it fires AT the threshold, fires above it, and is silent below — which is
  // the part a unit suite can hold without an artifact.
})

// The host is read off the exported list, never spelled here: the source scan
// would flag a literal host in this file.
describe('the precached documentation pages', () => {
  it('fail verification when a page references a forbidden remote font host', () => {
    const { host } = FORBIDDEN_FONT_HOSTS[0]
    const clean = { url: '/assets/rendering-library-0123456789abcdef0123.html', html: '<title>Guide</title><p>system fonts</p>' }
    const remote = { url: '/assets/expression-reference-0123456789abcdef0123.html', html: `<link rel="stylesheet" href="https://${host}/css2">` }
    expect(documentationFontHostFinding([clean])).toBeNull()
    expect(documentationFontHostFinding([clean, remote])).toEqual({ url: remote.url, host })
  })
})

describe('the starter and example template assets', () => {
  const hex = '0123456789abcdef0123'
  const starter = { url: `/assets/starter.${hex}-Ab12Cd34.folio`, immutable: true }
  const invoice = [
    { url: `/assets/invoice.${hex}-Ab12Cd34.folio`, immutable: true },
    { url: `/assets/invoice.sample.${hex}-Ab12Cd34.json`, immutable: true },
    { url: `/assets/invoice.thumbnail.${hex}-Ab12Cd34.png`, immutable: true },
  ]

  it('pass with the starter and every example asset', () => {
    expect(templateAssetFinding([starter, ...invoice], ['invoice'])).toBeNull()
  })

  it('fail when only an example .folio is present, because the starter is required by name', () => {
    expect(templateAssetFinding(invoice, ['invoice'])).toBe('missing the starter template runtime asset')
  })

  it('fail naming the example when one of its assets is missing', () => {
    expect(templateAssetFinding([starter, invoice[0], invoice[1]], ['invoice'])).toBe("example 'invoice' must ship exactly one immutable thumbnail asset (found 0)")
    expect(templateAssetFinding([starter, invoice[1], invoice[2]], ['invoice'])).toBe("example 'invoice' must ship exactly one immutable template asset (found 0)")
  })
})

describe('the offline release approach warning', () => {
  // NON-VACUITY FIRST. Every assertion below is parameterised on the two
  // declared numbers, and a threshold equal to the maximum would make the
  // "below the threshold" case and the margin arithmetic degenerate.
  it('is driven by a threshold that leaves room to warn in', () => {
    expect(Number.isSafeInteger(warnCacheAssets)).toBe(true)
    expect(Number.isSafeInteger(maximumCacheAssets)).toBe(true)
    expect(warnCacheAssets, 'a threshold at or above the maximum makes the warning unreachable and every case below vacuous').toBeLessThan(maximumCacheAssets)
    expect(warnCacheAssets, 'a threshold of 1 or 0 leaves no "below the threshold" case to assert silence on').toBeGreaterThan(1)
  })

  // AND THE NUMBER THE VERIFIER READS IS THE NUMBER TypeScript DECLARES.
  //
  // THIS IS `cacheAssetApproachWarning`'s ONE CONSUMER, and it is a real one.
  // `scripts/offline-release-contract.mjs` deliberately CANNOT import
  // `src/release-payload.ts` (it needs node:crypto; tsconfig.app.json includes
  // only src/), so it derives the threshold by reading the `const` line as
  // TEXT. That reader is a regex over source, and a regex can read a number out
  // of a line the TypeScript compiler treats differently — a duplicated
  // declaration in a scope the pattern cannot see, a value the module then
  // shadows. This is the only assertion in the repository that puts the
  // text-derived number beside the value the module actually evaluates to, and
  // it is why the export exists rather than existing to satisfy
  // `noUnusedLocals`.
  it('reads the same threshold the TypeScript module declares, text reader against evaluated value', () => {
    expect(declaredCacheAssetWarning().warnCacheAssets, 'scripts/offline-release-contract.mjs reads `warnCacheAssets` out of src/release-payload.ts as TEXT; this is the value that file actually evaluates to').toBe(cacheAssetApproachWarning)
    // AND THE BOUND ON THE SAME TERMS (spec-install-all-face-cuts, story 3).
    // `cacheAssetCeiling` is what `src/release-payload.test.ts` derives its
    // over-the-bound red proof from, so the number that proof is built on is
    // tied to the number the text reader pulls out of the same file.
    expect(declaredCacheAssetWarning().maximumCacheAssets, 'scripts/offline-release-contract.mjs reads `maximumCacheAssets` out of src/release-payload.ts as TEXT; this is the value that file actually evaluates to').toBe(cacheAssetCeiling)
  })

  it('emits the warning for a release AT the declared threshold, and names the remaining margin', () => {
    const warn = vi.fn()
    const message = reportCacheAssetApproach(warnCacheAssets, { warn })
    expect(warn, 'a release standing exactly on the threshold is the first release the warning exists for').toHaveBeenCalledTimes(1)
    expect(message, 'the returned message must BE the emitted one; a reporter that returns a string and prints nothing warns nobody').toBe(warn.mock.calls[0][0])
    expect(message).toContain(`carries ${warnCacheAssets} cache assets`)
    expect(message).toContain(`a declared maximum of ${maximumCacheAssets}`)
    // THE MARGIN IS THE POINT. DW-162's figure aged 41 -> 20 -> 10 while three
    // stories walked past it, because the number nobody printed was the number
    // nobody watched. A warning that named only the count would have the same
    // defect.
    expect(message, 'the warning must state the REMAINING MARGIN, not just the count').toContain(`the margin is ${maximumCacheAssets - warnCacheAssets}`)
    expect(message).toContain('`warnCacheAssets`')
  })

  it('emits it for every release ABOVE the threshold, with the margin each one actually has', () => {
    for (let assetCount = warnCacheAssets; assetCount <= maximumCacheAssets; assetCount++) {
      const warn = vi.fn()
      const message = reportCacheAssetApproach(assetCount, { warn })
      expect(warn, `a release of ${assetCount} assets is at or over the threshold ${warnCacheAssets} and must warn`).toHaveBeenCalledTimes(1)
      expect(message, `the margin at ${assetCount} of ${maximumCacheAssets}`).toContain(`the margin is ${maximumCacheAssets - assetCount}`)
    }
  })

  it('is SILENT below the threshold, and says so by returning null rather than by printing nothing', () => {
    for (const assetCount of [warnCacheAssets - 1, Math.floor(warnCacheAssets / 2), 1, 0]) {
      const warn = vi.fn()
      expect(reportCacheAssetApproach(assetCount, { warn }), `a release of ${assetCount} assets is under the threshold ${warnCacheAssets} and must not warn`).toBeNull()
      expect(warn, `a release of ${assetCount} assets is under the threshold ${warnCacheAssets} and must not warn`).not.toHaveBeenCalled()
    }
  })

  // AND THE REPORTER IS WIRED INTO THE VERIFIER, on an explicit option.
  //
  // The four assertions above drive `reportCacheAssetApproach` directly, so
  // they would all stay green if its CALL SITE were deleted — the shape of
  // vacuous green this file exists against. `verifyOfflineRelease` needs a
  // whole built dist to execute, which is what put the warning out of a test's
  // reach in the first place, so the wiring is asserted against the source text
  // instead: the one property a unit test can hold without a release build.
  //
  // AND THE LATCH IS GONE. `approachWarningReported` was module scope, never
  // reset, and was set INSIDE the firing branch — so it latched on the first
  // FIRING rather than the first CALL. Under `--red-only` the real release is
  // never passed to `verifyOfflineRelease` at all, so the single warning line a
  // run printed could describe a deliberately mutated red-proof fixture while
  // reading as a statement about the release. Its absence is asserted, not
  // assumed.
  it('is called by verifyOfflineRelease on an explicit option, and by the CLI only for the real release', () => {
    // `import.meta.dirname` rather than `new URL(..., import.meta.url)`: under
    // Vitest's jsdom environment `import.meta.url` is not a file: URL, and
    // `offline-release-contract.test.mjs` already uses this idiom next door.
    const source = readFileSync(join(import.meta.dirname, 'verify-offline-release.mjs'), 'utf8')
    expect(source, 'verifyOfflineRelease no longer calls the reporter, so the warning is unreachable in a real build however green this file is').toContain('if (reportApproach) reportCacheAssetApproach(release.assets.length)')
    expect(source, 'the CLI must ask for the report on the branch that verifies the REAL dist').toContain('verifyOfflineRelease(dist, { wasmWitness, reportApproach: true })')
    expect(source, 'the option must default to OFF, so the two dozen red-proof calls over a mutated dist stay quiet without a latch').toContain('reportApproach = false')
    expect(source, 'a module-scope first-call latch is what let a red-proof fixture consume the one warning the real release was owed').not.toContain('approachWarningReported')
  })
})

// ---------------------------------------------------------------------------
// THE CORE TIER'S PIN, TIED THE SAME WAY (spec-deferred-offline-cache, story 1).
//
// `coreCacheAssetFloor` and `coreCacheAssetCeiling` exist for the reason
// `cacheAssetApproachWarning` does: nothing in `src/` reads the two `const`
// lines they reference, so without a real consumer the only thing holding them
// would be `noUnusedLocals` — and a public symbol whose callers are nobody is a
// symbol the next person deletes. These are that consumer, and they are the
// only assertions in the repository that put the number the regex reader pulls
// out of the file's SOURCE TEXT beside the value the module actually evaluates
// to.
// ---------------------------------------------------------------------------
describe('the core cache-asset pin', () => {
  it('reads the same two numbers the TypeScript module declares, text reader against evaluated values', () => {
    const { minimumCoreCacheAssets, maximumCoreCacheAssets } = declaredCoreCacheAssetBounds()
    expect(minimumCoreCacheAssets, 'scripts/offline-release-contract.mjs reads `minimumCoreCacheAssets` out of src/release-payload.ts as TEXT; this is the value that file actually evaluates to').toBe(coreCacheAssetFloor)
    expect(maximumCoreCacheAssets, 'scripts/offline-release-contract.mjs reads `maximumCoreCacheAssets` out of src/release-payload.ts as TEXT; this is the value that file actually evaluates to').toBe(coreCacheAssetCeiling)
  })

  // THE PIN IS THE POINT. Owner decision, 2026-09-19: the core tier is pinned
  // exactly rather than given headroom, so that growth AND shrinkage of the
  // blocking set both fail the build. Headroom reintroduced by a later edit
  // would leave every other assertion here green while the guard stopped
  // guarding one of the two directions.
  it('is declared as an exact pin rather than an envelope with headroom', () => {
    expect(coreCacheAssetFloor, 'both ends equal is what makes any movement of the blocking set fail the build').toBe(coreCacheAssetCeiling)
  })

  // THE WEIGHT, TIED THE SAME WAY (spec-deferred-offline-cache, story 5).
  // `coreCacheByteCeiling` is exported for exactly the reason the two above
  // are: nothing in `src/` reads `maximumCoreCacheBytes`, so this assertion is
  // the only thing that puts the number the regex reader pulls out of the
  // file's SOURCE TEXT beside the value the module evaluates to.
  it('reads the same core byte ceiling the TypeScript module declares', () => {
    const { maximumCoreCacheBytes } = declaredCoreCacheByteCeiling()
    expect(maximumCoreCacheBytes, 'scripts/offline-release-contract.mjs reads `maximumCoreCacheBytes` out of src/release-payload.ts as TEXT; this is the value that file actually evaluates to').toBe(coreCacheByteCeiling)
  })

  // ⚠ WHERE THE CEILING IS ACTUALLY ENFORCED, AND WHY IT IS NOT ENFORCED HERE.
  // The comparison against a real release lives in `verify-offline-release.mjs`
  // and in `generate-offline-release.mjs`, both of which run under
  // `npm run build` and neither of which Vitest executes — and the falsifier is
  // the `core-tier-bytes-over-ceiling` red proof, which lowers the declaration
  // below the honest measurement and requires the build to refuse. A copy of
  // that comparison here would need a built `dist/`, which this suite does not
  // have and must not require.
  it('states the guard that enforces it, in the two scripts that run at build time', () => {
    // `import.meta.dirname`, for the reason the sibling test above states it.
    const verifier = readFileSync(join(import.meta.dirname, 'verify-offline-release.mjs'), 'utf8')
    const generator = readFileSync(join(import.meta.dirname, 'generate-offline-release.mjs'), 'utf8')
    expect(verifier, 'the verifier must compare the core tier to the declared ceiling').toContain('core-tier-bytes-over-ceiling')
    expect(verifier, 'and it must read the ceiling rather than re-typing it').toContain('declaredCoreCacheByteCeiling(releasePayloadText)')
    expect(generator, 'the generator must refuse to emit an over-ceiling release, so `build:offline` alone cannot produce one').toContain('declaredCoreCacheByteCeiling(releasePayloadText)')
    expect(verifier, 'the guard needs a falsifier, and the only honest one moves the declaration rather than the release').toContain("redProof('core-tier-bytes-over-ceiling'")
    // ⚠ AND THE FALSIFIER MUST REACH BOTH COPIES. The comparison is duplicated
    // so that `build:offline` alone cannot emit an over-budget release; a
    // proof that exercised only the verifier would leave the generator's copy
    // free to be inverted with every proof green.
    expect(verifier, 'the proof must run the GENERATOR under the lowered ceiling too, or half the guard is unproved').toContain('generateOfflineRelease(dist, { releasePayloadText: loweredCeiling })')
    // ⚠ AND IT MUST NOT DO IT BY EDITING TRACKED SOURCE. A proof that rewrote
    // src/release-payload.ts and restored it in a `finally` leaves the lowered
    // number in a committed file on any SIGINT, and races a `vite dev` or
    // `vitest --watch` reading it.
    expect(verifier, 'no red proof may write to the working tree\'s source').not.toContain('writeFileSync(releasePayloadSource')
  })

  // ⚠ AND THE WEIGHT IS SUMMED IN ONE PLACE. Both scripts call
  // `coreTierBrotliBytes`, so the number the build records and the number the
  // verifier compares cannot be two different arithmetics over the same rows.
  it('sums the core tier through one shared derivation rather than two', () => {
    expect(coreTierBrotliBytes([
      { tier: 'core', immutable: true, brotliBytes: 10 },
      { tier: 'core', immutable: true, brotliBytes: 5 },
      { tier: 'core', immutable: false },
      { tier: 'deferred', immutable: true, brotliBytes: 1000 },
    ]), "the mutable navigation entry carries no sidecar and the deferred tier is not blocking; neither belongs in a first load's weight").toBe(15)
  })
})
