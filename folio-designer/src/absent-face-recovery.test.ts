import { describe, expect, it, vi } from 'vitest'
import { AbsentFaceInstaller, absentFaceNames } from './absent-face-recovery'
import type { EngineClient } from './engine-client'
import type { EngineError } from './engine-protocol'
import { canvasFaceAssets } from './generated/canvas-face-assets'

// The two sentence shapes Go actually writes, quoted from the source rather
// than paraphrased: `render.go`'s faceAbsentMessage for a rune no present face
// covers, and `wrap.go`'s for a chain with no present member at all. They are
// different shapes on purpose — the point of `absentFaceNames` is that it reads
// both without parsing either.
const runeRefusal = 'face "Noto Sans SC" is not present in the supplied FontSet, and no present face in chain [Roboto, Noto Sans Thai, Noto Sans SC] covers U+6C49 (汉) in element e1 — the render is refused rather than omitting the rune, because whether an absent face would have covered it cannot be known here (AD-8)'
const chainRefusal = "folio8: none of the fallback chain's faces [Noto Sans SC] is present in the supplied FontSet, so no line height can be derived from it"

const refusal = (message: string): EngineError => ({ code: 'TEXT_FACE_ABSENT', message })

describe('reading the face out of a refusal', () => {
  it('names the face in both of the sentence shapes Go writes', () => {
    expect(absentFaceNames(runeRefusal)).toContain('Noto Sans SC')
    expect(absentFaceNames(chainRefusal)).toEqual(['Noto Sans SC'])
  })

  // ⚠ THE CANDIDATE SET IS THE CANVAS FACE MAP, so a name it does not carry is
  // one this function structurally cannot return. That is what makes the
  // reading safe against a message this side does not author.
  it('invents nothing, however the message is worded', () => {
    expect(absentFaceNames('face "Helvetica Neue Ultralight" is not present in the supplied FontSet')).toEqual([])
    expect(absentFaceNames('')).toEqual([])
  })

  it('reads the map this build actually emits, not a fixture of it', () => {
    expect(canvasFaceAssets.get('Noto Sans SC'), 'the release must declare an asset for the CJK face, or the recovery has nothing to fetch').toBeDefined()
  })

  // THE SAME LONGEST-FIRST DEFENCE, RE-PROVEN AT THE SCALE STORY 3 CREATED.
  //
  // `absent-face-recovery.ts` is NOT MODIFIED by spec-install-all-face-cuts
  // story 3, and the base/cut ambiguity it defends against is PRE-EXISTING:
  // `canvasFaceAssets` has carried `Noto Sans` beside `Noto Sans Bold`, and
  // `Roboto` beside `Roboto Bold`, since Story 11.1. What story 3 changed is
  // the SIZE of the candidate set — from 44 names to 120 — and the number of
  // base/cut pairs in it, from 4 to 30-odd. A scan that reported the base for a
  // message naming the cut would now be wrong for most of the committed tier
  // rather than for two shipped families, so the property is measured here
  // rather than assumed to have survived the population change.
  it('resolves a cut against its own base at the whole catalogue\'s scale', () => {
    const names = [...canvasFaceAssets.keys()]
    // NON-VACUITY, IN BOTH DIRECTIONS. The set must be the big one, and it must
    // genuinely contain names that are prefixes of other names — a candidate
    // set with no such pair would make every assertion below a tautology.
    expect(names.length, 'the canvas face map must carry the whole tier; story 3 took it from 44 names to 120').toBeGreaterThanOrEqual(120)
    const prefixPairs = names.filter((name) => names.some((other) => other !== name && other.startsWith(`${name} `)))
    expect(prefixPairs.length, 'no face name is a prefix of another, so the longest-first scan has nothing to be right about').toBeGreaterThan(20)

    // EVERY BASE/CUT PAIR, NOT A SAMPLE: a bracket naming ONLY the cut must
    // resolve to the cut alone, never to the base whose name sits inside it.
    for (const base of prefixPairs) {
      for (const cut of names.filter((name) => name.startsWith(`${base} `))) {
        expect(absentFaceNames(`folio8: none of the fallback chain's faces [${cut}] is present in the supplied FontSet, so no line height can be derived from it`), `a chain naming ${cut} alone must not report ${base}`).toEqual([cut])
      }
    }

    // AND A BRACKET NAMING BOTH REPORTS BOTH, in the map's own order — the
    // blanking step must consume the cut's occurrence without swallowing the
    // base's separate one.
    expect([...absentFaceNames('folio8: none of the fallback chain\'s faces [Inter Bold, Inter] is present in the supplied FontSet, so no line height can be derived from it')].sort(), 'a chain naming a base AND its cut must report both').toEqual(['Inter', 'Inter Bold'])
    // The quoted shape takes the same pair through the other arm of the reader.
    expect(absentFaceNames('face "Inter Bold" is not present in the supplied FontSet, and no present face in chain [Inter, Inter Bold] covers U+0041 (A) in element e1')).toEqual(['Inter Bold'])
  })
})

type Installed = { face: string; bytes: ArrayBuffer }

function fakeClient(): { client: EngineClient; installs: Installed[] } {
  const installs: Installed[] = []
  const client = {
    request: vi.fn(async (_operation: string, payload: Installed) => { installs.push(payload); return { snapshot: { documentState: 'loaded', revision: 1, byteLength: 0 } } }),
  } as unknown as EngineClient
  return { client, installs }
}

const okFetch = (bytes = 8) => vi.fn(async () => new Response(new Uint8Array(bytes), { status: 200 })) as unknown as typeof fetch

describe('the absent-face installer', () => {
  it('fetches the named face once, installs its raw bytes, and reports that a retry is worth making', async () => {
    const { client, installs } = fakeClient()
    const request = okFetch()
    const installer = new AbsentFaceInstaller(client, 50, request)
    await expect(installer.recover(refusal(runeRefusal))).resolves.toBe(true)
    expect(installs).toHaveLength(1)
    expect(installs[0].face).toBe('Noto Sans SC')
    expect(installs[0].bytes.byteLength).toBe(8)
    expect(client.request).toHaveBeenCalledWith('install-face', installs[0])
    expect(request).toHaveBeenCalledTimes(1)
  })

  // ⚠ ONE ATTEMPT PER FACE, FOR THE SESSION. The second CJK document of a
  // session must neither refuse nor fetch — but even if it somehow refused,
  // this must not fetch 10 MiB again. The answer is REPLAYED, not recomputed:
  // `EngineClient` retries one request at most once, so a replayed `true`
  // costs one extra round trip and never a second download.
  it('never fetches the same face twice, whatever refuses next', async () => {
    const { client } = fakeClient()
    const request = okFetch()
    const installer = new AbsentFaceInstaller(client, 50, request)
    await installer.recover(refusal(runeRefusal))
    await installer.recover(refusal(runeRefusal))
    expect(request, 'a face already fetched must never be fetched again').toHaveBeenCalledTimes(1)
    expect(client.request, 'nor installed again').toHaveBeenCalledTimes(1)
  })

  // ⚠ CONCURRENT REFUSALS SHARE ONE FETCH AND ONE ANSWER. `Apply` fires about
  // every 200 ms while an author types, so the commit after the first CJK
  // character refuses while the first commit's ~10 MiB fetch is still in the
  // air. Both must end up succeeding, over ONE download.
  it('makes concurrent refusals wait for the one fetch already in flight', async () => {
    const { client } = fakeClient()
    let release!: () => void
    const gate = new Promise<void>((done) => { release = done })
    const request = vi.fn(async () => { await gate; return new Response(new Uint8Array(8), { status: 200 }) }) as unknown as typeof fetch
    const installer = new AbsentFaceInstaller(client, 500, request)
    const first = installer.recover(refusal(runeRefusal))
    const second = installer.recover(refusal(runeRefusal))
    release()
    await expect(first).resolves.toBe(true)
    await expect(second, 'a commit that refused mid-fetch must wait for it, not be told there is nothing doing').resolves.toBe(true)
    expect(request).toHaveBeenCalledTimes(1)
    expect(client.request).toHaveBeenCalledTimes(1)
  })

  // The matrix's "engine refuses for a face with no asset URL" row: a document
  // naming a face this release does not ship. Nothing to fetch, nothing to
  // install, and the refusal reaches the author.
  it('reports no recovery for a face the release carries no asset for', async () => {
    const { client } = fakeClient()
    const request = okFetch()
    const installer = new AbsentFaceInstaller(client, 50, request)
    await expect(installer.recover(refusal('face "Helvetica Neue Ultralight" is not present in the supplied FontSet'))).resolves.toBe(false)
    expect(request).not.toHaveBeenCalled()
    expect(client.request).not.toHaveBeenCalled()
  })

  // The matrix's "offline, face never fetched" row, from this side of it: the
  // fetch fails, nothing is installed, and the refusal is what the author sees.
  //
  // ⚠ AND IT IS TRIED AGAIN LATER, which is the opposite of what this test
  // used to pin. Memoising a failed FETCH poisons the face for the rest of the
  // session: the author opens a CJK document with no network, reconnects, and
  // goes on being refused until they reload the page with nothing telling them
  // to. The bound on retrying lives where it belongs — `EngineClient` re-sends
  // one request at most once — so a face that will not arrive costs one fetch
  // per refusal an author provokes, never a loop.
  it('reports no recovery when the face cannot be fetched, and tries again next time', async () => {
    const { client } = fakeClient()
    const failing = vi.fn(async () => { throw new TypeError('Failed to fetch') })
    const installer = new AbsentFaceInstaller(client, 50, failing as unknown as typeof fetch)
    await expect(installer.recover(refusal(runeRefusal))).resolves.toBe(false)
    expect(client.request).not.toHaveBeenCalled()
    await expect(installer.recover(refusal(runeRefusal))).resolves.toBe(false)
    expect(failing, 'a fetch that did not arrive must be attempted again, or reconnecting cannot help').toHaveBeenCalledTimes(2)
  })

  it('installs the face on a later attempt once the network is back', async () => {
    const { client, installs } = fakeClient()
    let online = false
    const request = vi.fn(async () => {
      if (!online) throw new TypeError('Failed to fetch')
      return new Response(new Uint8Array(8), { status: 200 })
    }) as unknown as typeof fetch
    const installer = new AbsentFaceInstaller(client, 50, request)
    await expect(installer.recover(refusal(runeRefusal))).resolves.toBe(false)
    online = true
    await expect(installer.recover(refusal(runeRefusal)), 'the author reconnected; the same document must now open').resolves.toBe(true)
    expect(installs).toHaveLength(1)
  })

  // ⚠ THE TWO UNRETRYABLE OUTCOMES STAY REMEMBERED, and they are the two that
  // no amount of network can change: a face this release declares no asset for
  // (covered above), and bytes the engine would not accept.
  it('reports no recovery when the engine refuses the bytes it was handed, and does not fetch them again', async () => {
    const request = okFetch()
    const client = { request: vi.fn(async () => { throw new Error('WASM_INPUT_INVALID') }) } as unknown as EngineClient
    const installer = new AbsentFaceInstaller(client, 50, request)
    await expect(installer.recover(refusal(runeRefusal))).resolves.toBe(false)
    await expect(installer.recover(refusal(runeRefusal))).resolves.toBe(false)
    expect(request, 'the bytes arrived and the engine would not have them; fetching them again cannot change that').toHaveBeenCalledTimes(1)
  })

  // Matrix row 5: "CJK document, offline, face already cached — opens and
  // renders normally from cache." The service worker answers a held deferred
  // asset without touching the network, so the ONLY way to reach a cached face
  // is to make the request — which means this recovery must ask even when the
  // browser reports itself offline. The canvas prefetch declines in that state,
  // rightly, because its failure costs an author nothing; this one's failure is
  // a document that will not open.
  it('asks for the face even while the browser reports itself offline, so a cached one still arrives', async () => {
    const { client, installs } = fakeClient()
    const request = okFetch()
    const installer = new AbsentFaceInstaller(client, 50, request)
    const onLine = Object.getOwnPropertyDescriptor(navigator, 'onLine')
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: false })
    try {
      await expect(installer.recover(refusal(runeRefusal))).resolves.toBe(true)
    } finally {
      if (onLine) Object.defineProperty(navigator, 'onLine', onLine); else Reflect.deleteProperty(navigator, 'onLine')
    }
    expect(request, 'declining to ask offline would turn a face this browser already holds into a refusal').toHaveBeenCalledTimes(1)
    expect(installs).toHaveLength(1)
  })
})
