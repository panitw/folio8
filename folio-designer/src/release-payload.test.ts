import { afterEach, describe, expect, it, vi } from 'vitest'
import { isDevBypassReason, loadS1Payload, parseS1Payload, payloadForLifecycle, type S1PayloadRejection, type S1PayloadResult } from './release-payload'

const hash = 'a'.repeat(64)
const cacheAssets = ['/index.html', '/engine', '/latin', '/thai', '/cjk', ...Array.from({ length: 15 }, (_, index) => `/asset-${index}`)]
// TWELVE ROWS SINCE STORY 11.1 — the emitted shape, in the emitted order. It is
// written out rather than generated so a reader can see which ids the parser is
// keyed to; `parseS1Payload` checks the ids and labels positionally, so a
// fixture short of a row rejects as `payload-shape` and every case below would
// then be asserting the wrong reason.
const cachedRow = (id: string, label: string, assetUrl: string) => ({ id, label, delivery: 'cached-asset', assetUrl, bytes: 10, sha256: hash })
const payload = () => ({ version: 1, releaseId: hash, pageId: 'b'.repeat(64), unit: 'MiB', decimals: 2, cachedBytes: 200, assetCount: 20, cacheAssets: cacheAssets.map((assetUrl) => ({ assetUrl, bytes: 10 })), rows: [
  cachedRow('engine', 'Engine', '/engine'),
  cachedRow('latin-font', 'Latin font', '/latin'),
  cachedRow('thai-font', 'Thai font', '/thai'),
  cachedRow('cjk-font', 'CJK font', '/cjk'),
  cachedRow('noto-sans-bold-font', 'Noto Sans Bold', '/asset-0'),
  cachedRow('noto-sans-italic-font', 'Noto Sans Italic', '/asset-1'),
  cachedRow('noto-sans-bold-italic-font', 'Noto Sans Bold Italic', '/asset-2'),
  cachedRow('noto-sans-thai-bold-font', 'Noto Sans Thai Bold', '/asset-3'),
  cachedRow('roboto-bold-font', 'Roboto Bold', '/asset-4'),
  cachedRow('roboto-italic-font', 'Roboto Italic', '/asset-5'),
  cachedRow('roboto-bold-italic-font', 'Roboto Bold Italic', '/asset-6'),
  { id: 'thai-dictionary', label: 'Thai dictionary', delivery: 'embedded-in-engine', assetUrl: '/engine', bytes: 5, sha256: hash },
  ] })

// A rejection is only evidence if it NAMES its cause, so every assertion below
// reads the reason rather than the mere absence of a payload. `accepted` is
// returned for a success so a case that stops being rejected reddens loudly
// instead of quietly satisfying a `not.toBe` somewhere.
const reasonOf = (value: unknown): S1PayloadRejection | 'accepted' => { const result = parseS1Payload(value); return result.ok ? 'accepted' : result.reason }

const overBound = () => { const over = payload(); over.assetCount = 91; over.cacheAssets = Array.from({ length: 91 }, (_, index) => ({ assetUrl: `/asset-${index}`, bytes: 10 })); over.cachedBytes = 910; return over }
const underBound = () => { const under = payload(); under.assetCount = 9; under.cacheAssets = under.cacheAssets.slice(0, 9); under.cachedBytes = 90; return under }
const staleArithmetic = () => { const total = payload(); total.cachedBytes = 41; return total }
// KEYED BY ID, LIKE THE ASSERTION IT FALSIFIES. `rows[4]` was the dictionary
// until Story 11.1 inserted seven rows in front of it; an index-keyed mutation
// would now flip a FONT row's delivery and still produce a rejection — for the
// wrong reason, which is a red proof that has stopped proving its own claim.
const deliveryFiction = () => { const delivery = payload(); const dictionary = delivery.rows.find((row) => row.id === 'thai-dictionary'); if (!dictionary) throw new Error('the delivery-fiction fixture has no thai-dictionary row to mutate'); dictionary.delivery = 'cached-asset'; return delivery }
const surplusField = () => ({ ...payload(), document: 'must-not-cross-boundary' })
const rowNotAnObject = () => { const rows = payload(); (rows.rows as unknown[])[2] = null; return rows }
const rowMislabelled = () => { const rows = payload(); rows.rows[1].label = 'Engine'; return rows }
const duplicateCacheAsset = () => { const duplicate = payload(); duplicate.cacheAssets[3] = { assetUrl: '/engine', bytes: 10 }; return duplicate }

// Every rejecting input this file asserts, in one table, so the exclusivity claim
// below is over the whole set rather than over whichever cases someone remembered.
const rejections: readonly (readonly [string, () => unknown, S1PayloadRejection])[] = [
  ['undefined', () => undefined, 'not-an-object'],
  ['null', () => null, 'not-an-object'],
  ['a bare string', () => 'not a payload', 'not-an-object'],
  ['a surplus field', surplusField, 'payload-shape'],
  ['a payload over the release bound', overBound, 'asset-count-over-maximum'],
  ['a payload under the release floor', underBound, 'asset-count-under-minimum'],
  ['a duplicated cache asset', duplicateCacheAsset, 'cache-assets-invalid'],
  ['stale cached-byte arithmetic', staleArithmetic, 'cached-bytes-mismatch'],
  ['a row that is not an object', rowNotAnObject, 'row-not-an-object'],
  ['a mislabelled row', rowMislabelled, 'row-shape'],
  ['dictionary delivery fiction', deliveryFiction, 'row-delivery-composition'],
]

const bootstrapNode = (text: string) => {
  const node = document.createElement('script')
  node.id = 'folio8-release-bootstrap'
  node.type = 'application/json'
  node.textContent = text
  document.body.append(node)
  return node
}

afterEach(() => { document.getElementById('folio8-release-bootstrap')?.remove(); vi.restoreAllMocks() })

describe('release-derived S1 payload boundary', () => {
  it('accepts the current bounded generated shape', () => expect(parseS1Payload(payload())).toMatchObject({ ok: true, payload: { cachedBytes: 200, assetCount: 20, rows: expect.any(Array) } }))

  it('names the cause of every rejection rather than answering the same value to all of them', () => {
    expect(rejections.map(([name, input]) => [name, reasonOf(input())])).toEqual(rejections.map(([name, , reason]) => [name, reason]))
  })

  it('rejects cache asset counts outside the bounded release envelope, one reason per end', () => {
    expect(reasonOf(underBound())).toBe('asset-count-under-minimum')
    expect(reasonOf(overBound())).toBe('asset-count-over-maximum')
  })

  // THE DISCRIMINATION CLAIM. A named reason that several unrelated causes can
  // also produce is no better than the bare `undefined` it replaced.
  it('produces the over-the-bound reason from that cause and from no other asserted input', () => {
    expect(rejections.filter(([, input]) => reasonOf(input()) === 'asset-count-over-maximum').map(([name]) => name)).toEqual(['a payload over the release bound'])
    expect(rejections.filter(([, input]) => reasonOf(input()) === 'asset-count-under-minimum').map(([name]) => name)).toEqual(['a payload under the release floor'])
  })
})

describe('S1 bootstrap reader', () => {
  it('reports an absent bootstrap node by its own reason', () => expect(loadS1Payload()).toEqual({ ok: false, reason: 'no-bootstrap' }))

  it('reports an empty bootstrap node by that same reason', () => {
    bootstrapNode('')
    expect(loadS1Payload()).toEqual({ ok: false, reason: 'no-bootstrap' })
  })

  it('reads the emitted bootstrap shape', () => {
    bootstrapNode(JSON.stringify({ s1: payload() }))
    expect(loadS1Payload()).toMatchObject({ ok: true, payload: { assetCount: 20 } })
  })

  // The two causes that used to be one bare `undefined`.
  it('tells a bootstrap that is not JSON apart from one that is over the bound', () => {
    bootstrapNode('{"s1": {"version": 1, "cache')
    const malformed = loadS1Payload()
    document.getElementById('folio8-release-bootstrap')!.remove()
    bootstrapNode(JSON.stringify({ s1: overBound() }))
    const over = loadS1Payload()
    expect(malformed).toEqual({ ok: false, reason: 'malformed-json' })
    expect(over).toEqual({ ok: false, reason: 'asset-count-over-maximum' })
    expect(malformed).not.toEqual(over)
  })

  // The catch exists for JSON.parse's SyntaxError and for nothing else. Anything
  // else thrown here is a fault in the page, and disguising it as a malformed
  // bootstrap is how a defect becomes invisible.
  it('rethrows a throw that is not a JSON.parse SyntaxError instead of converting it to a rejection', () => {
    bootstrapNode('{"s1": null}')
    vi.spyOn(JSON, 'parse').mockImplementation(() => { throw new RangeError('bootstrap read faulted') })
    expect(() => loadS1Payload()).toThrow(RangeError)
  })

  // `JSON.parse('null')` succeeds, so reading `.s1` off it would throw a TypeError
  // the narrowed catch no longer admits. It must still reach the parser as a
  // rejection: narrowing may not change what a user sees on this path.
  it('keeps a null bootstrap a rejection rather than a propagating TypeError', () => {
    bootstrapNode('null')
    expect(loadS1Payload()).toEqual({ ok: false, reason: 'not-an-object' })
  })
})

// THE TWO DECISIONS main.tsx MAKES ABOUT A RESULT, EXECUTED.
// Nothing in this repo imports main.tsx, and Vitest collects only
// `src/**/*.test.{ts,tsx}` and `scripts/**/*.test.mjs` (the Playwright suite is
// compile-only and never runs), so while both decisions were expressions inside
// that module they were executed by NO test: replacing the payload mapping with
// a bare `undefined` type-checked and shipped an app permanently reporting
// "Offline cache unavailable", and the dev bypass could be re-widened to any
// falsy payload with every gate green. They are pure, so they are asserted here.
//
// The bypass predicate is driven over the SAME rejection table this file already
// builds, plus the two reader-level reasons that table cannot reach, and the
// union is held to the declared reason set below — so "fires on `no-bootstrap`
// and on NO other reason" is a claim over every reason that exists.
const readerRejections: readonly (readonly [string, () => S1PayloadResult, S1PayloadRejection])[] = [
  ['an absent bootstrap node', () => { document.getElementById('folio8-release-bootstrap')?.remove(); return loadS1Payload() }, 'no-bootstrap'],
  ['a bootstrap that is not JSON', () => { document.getElementById('folio8-release-bootstrap')?.remove(); bootstrapNode('{"s1": {"version": 1, "cache'); return loadS1Payload() }, 'malformed-json'],
]

// Exhaustive BY TYPE: a new member of S1PayloadRejection that no table above
// drives is a typecheck failure here, not a silently uncovered reason.
const declaredReasons: Readonly<Record<S1PayloadRejection, true>> = { 'not-an-object': true, 'payload-shape': true, 'asset-count-over-maximum': true, 'asset-count-under-minimum': true, 'cache-assets-invalid': true, 'cached-bytes-mismatch': true, 'row-not-an-object': true, 'row-shape': true, 'row-delivery-composition': true, 'no-bootstrap': true, 'malformed-json': true }

describe("the decisions main.tsx makes about a result", () => {
  it('names the cause of every reader-level rejection too', () => {
    expect(readerRejections.map(([name, read]) => { const result = read(); return [name, result.ok ? 'accepted' : result.reason] })).toEqual(readerRejections.map(([name, , reason]) => [name, reason]))
  })

  it('drives every declared rejection reason through these tables', () => {
    expect([...new Set([...rejections.map(([, , reason]) => reason), ...readerRejections.map(([, , reason]) => reason)])].sort()).toEqual(Object.keys(declaredReasons).sort())
  })

  it('hands the lifecycle the parsed payload itself when the result is ok', () => {
    const accepted = parseS1Payload(payload())
    if (!accepted.ok) throw new Error(`the bounded generated shape must parse; got ${accepted.reason}`)
    expect(payloadForLifecycle(accepted)).toBe(accepted.payload)
    expect(payloadForLifecycle(accepted)?.assetCount).toBe(20)
  })

  it('hands the lifecycle undefined for every rejection, so the load screen still says so', () => {
    expect([...rejections.map(([name, input]) => [name, payloadForLifecycle(parseS1Payload(input()))]), ...readerRejections.map(([name, read]) => [name, payloadForLifecycle(read())])])
      .toEqual([...rejections.map(([name]) => [name, undefined]), ...readerRejections.map(([name]) => [name, undefined])])
  })

  // THE NARROWING, HELD TO THE WHOLE REASON SET. A bypass that also fired on a
  // malformed or over-bound bootstrap would be the same "two causes, one
  // outcome" defect this story removed one line away.
  it('fires the dev bypass on the absent-bootstrap reason and on no other reason that exists', () => {
    const fired = [...rejections.map(([name, input]) => [name, isDevBypassReason(parseS1Payload(input()))] as const), ...readerRejections.map(([name, read]) => [name, isDevBypassReason(read())] as const)]
    expect(fired.filter(([, bypass]) => bypass).map(([name]) => name)).toEqual(['an absent bootstrap node'])
    expect(fired).toHaveLength(rejections.length + readerRejections.length)
    expect(isDevBypassReason(parseS1Payload(payload()))).toBe(false)
  })
})
