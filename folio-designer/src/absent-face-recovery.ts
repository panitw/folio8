/**
 * THE FACE THE ENGINE IS SHORT OF, FETCHED AND INSTALLED FROM THE REFUSAL
 * THAT NAMED IT (spec-deferred-offline-cache, CAP-6).
 *
 * WHY DISCOVERY IS REACTIVE AND NEVER PROACTIVE. The designer cannot ask the
 * engine which faces a document needs, because the engine answers that by
 * projecting the canvas — which is the thing that needs the face.
 * `paintedCanvasFaces` (`document-face-prefetch.ts`) reads the answer out of a
 * projection that has ALREADY SUCCEEDED, so it cannot be the input to opening
 * the document in the first place. The refusal breaks that cycle: it names the
 * face in the failure itself.
 *
 *     load(document) -> TEXT_FACE_ABSENT, naming "Noto Sans SC"
 *       -> canvasFaceAssets.get("Noto Sans SC") -> content-addressed URL
 *       -> fetch (the service worker verifies and caches it)
 *       -> install the raw bytes -> load(document) again
 *
 * ⚠ THE ALTERNATIVE — READING THE DOCUMENT'S `fontChains` — WAS REJECTED ON
 * EVIDENCE, not on taste. The starter template's only chain is
 * `[Roboto…, Noto Sans Thai…, "Noto Sans SC"]` and its bands are EMPTY, so a
 * chain-scan would fetch 4.72 MiB on every first load for a document that
 * contains no CJK codepoint at all — undoing this entire spec on the one screen
 * it exists to make fast.
 *
 * ⚠ AND IT IS NOT A PREFETCH. Nothing here runs on boot, on idle, or on a
 * timer. The only thing that starts it is the engine refusing, which is the
 * spec's `No background prefetch` stated as a control-flow fact rather than as
 * a promise.
 */
import type { EngineClient } from './engine-client'
import type { EngineError } from './engine-protocol'
import { canvasFaceAssets } from './generated/canvas-face-assets'
import { fetchDeferredFaces } from './document-face-prefetch'

/**
 * THE FACES A REFUSAL NAMES.
 *
 * Go writes them into the diagnostic's message, and it writes them in TWO
 * SHAPES, because two different conditions carry the same code:
 *
 *   render.go  `face "Noto Sans SC" is not present in the supplied FontSet,
 *              and no present face in chain [Roboto, Noto Sans Thai,
 *              Noto Sans SC] covers U+6C49 …`
 *   wrap.go    `none of the fallback chain's faces [Noto Sans SC] is present
 *              in the supplied FontSet, so no line height can be derived …`
 *
 * ⚠ THE FIRST SHAPE NAMES BOTH ABSENT AND PRESENT FACES, and only the QUOTED
 * ones are absent — the bracket is the whole chain, `Roboto` and
 * `Noto Sans Thai` very much included, and fetching those would spend bytes on
 * faces the engine already holds. So when the message quotes anything, the
 * quoted names are the answer and the bracket is ignored. `%q` is what puts
 * those quotes there, and it is the only thing in either sentence that
 * distinguishes the two roles.
 *
 * ⚠ THE SECOND SHAPE QUOTES NOTHING, because `%v` of a `[]string` does not.
 * There the bracket IS the absent set — no member of that chain was supplied —
 * so it is scanned, longest candidate first: face names contain one another
 * ("Noto Sans" is a prefix of "Noto Sans SC" and of "Noto Sans Thai"), and a
 * shortest-first scan would answer "Noto Sans" for a chain that names none of
 * it.
 *
 * ⚠ AND NOTHING IS INVENTED IN EITHER SHAPE. The candidate set is
 * `canvasFaceAssets` — the faces this release can actually fetch — so a name
 * that map does not carry is not a name this function can return, whatever the
 * message says. The ordering is the map's, which is the stylesheet's, so the
 * result is stable to read and to assert.
 */
export function absentFaceNames(message: string): ReadonlyArray<string> {
  const known = [...canvasFaceAssets.keys()]
  const quoted = new Set([...message.matchAll(/"([^"]+)"/g)].map((match) => match[1]))
  const fromQuotes = known.filter((family) => quoted.has(family))
  if (fromQuotes.length > 0) return fromQuotes
  // LONGEST FIRST, AND EACH MATCH CONSUMED. Scanning "[Noto Sans SC]" for
  // "Noto Sans" before "Noto Sans SC" would report a face the chain never
  // named; blanking each hit stops the longer match's own substring from
  // matching again behind it.
  const brackets = [...message.matchAll(/\[([^\]]*)\]/g)].map((match) => match[1])
  if (brackets.length === 0) return []
  const byLength = [...known].sort((left, right) => right.length - left.length)
  const found = new Set<string>()
  for (const bracket of brackets) {
    let remaining = bracket
    for (const family of byLength) {
      if (!remaining.includes(family)) continue
      found.add(family)
      remaining = remaining.split(family).join(' ')
    }
  }
  return known.filter((family) => found.has(family))
}

/**
 * ONE SESSION'S WORTH OF INSTALLED FACES, AND AT MOST ONE ATTEMPT IN FLIGHT
 * PER FACE.
 *
 * ⚠ WHAT IS REMEMBERED IS A SETTLED ANSWER, AND A FAILED FETCH IS NOT ONE.
 * Three outcomes end an attempt and they are not alike:
 *
 *   - INSTALLED — remembered. The face is held; there is nothing to redo.
 *   - NO ASSET FOR THIS FACE — remembered. The release declares no URL for it
 *     and never will within this page's lifetime, so a second look would ask
 *     the same map the same question.
 *   - THE FETCH DID NOT ARRIVE — NOT remembered. This is the offline author
 *     who reconnects. Memoising it poisons the face for the rest of the
 *     session: they open a CJK document with no network, plug the cable back
 *     in, and keep being refused until they reload the page, with nothing
 *     telling them to. That contradicts this spec's founding decision, which
 *     is to fetch when the network is up.
 *
 * A retry is still BOUNDED, and by the thing that actually bounds it:
 * `EngineClient` retries one request at most once, so a face that will not
 * arrive costs one extra fetch attempt per refusal an author provokes, never a
 * loop. The spec's "refuse once, never retry forever" is about a single
 * request's recovery, which is enforced there.
 *
 * ⚠ AND THE INSTALL SURVIVES FOR THE SESSION because the engine's set does. The
 * worker is never restarted — `EngineClient.terminate` tears one down and
 * builds no replacement — so one install per face is the whole cost, and the
 * second CJK document of a session neither refuses nor fetches.
 */
export class AbsentFaceInstaller {
  // ONE PROMISE PER FACE, KEPT FOR THE SESSION. The map is the record of
  // "this face has been attempted" AND the in-flight attempt itself, which
  // is what makes concurrent refusals safe: `Apply` fires about every 200 ms
  // while an author types, so the second commit after the first CJK
  // character refuses while the first commit's ~10 MiB fetch is still in
  // the air. A flag would answer "already attempted, nothing doing" and
  // fail that commit; the promise makes it WAIT for the same fetch and then
  // succeed on the same answer.
  readonly #attempts = new Map<string, Promise<boolean>>()
  readonly #client: EngineClient
  readonly #timeoutMs: number
  readonly #request: typeof fetch

  constructor(client: EngineClient, timeoutMs: number, request: typeof fetch = fetch) {
    this.#client = client
    this.#timeoutMs = timeoutMs
    this.#request = request
  }

  /**
   * The recovery `EngineClient.onAbsentFace` calls. `true` means at least one
   * of the faces the refusal named is now held — and therefore that retrying
   * the refused request can produce a different answer, which is the only
   * claim a retry is entitled to rest on.
   */
  recover = async (error: EngineError): Promise<boolean> => {
    const wanted = absentFaceNames(error.message)
    if (wanted.length === 0) return false
    const outcomes = await Promise.all(wanted.map((face) => this.#attempt(face)))
    return outcomes.some((installed) => installed)
  }

  // THE ENTRY IS MADE BEFORE THE FETCH IS AWAITED — that is what makes a
  // second refusal arriving mid-fetch WAIT for the one already in flight
  // rather than start a second download — and it is REMOVED again if the
  // attempt ended because the bytes did not arrive. So concurrency is
  // collapsed while the attempt is live, and a later attempt is free to
  // succeed once the network is back.
  #attempt(face: string): Promise<boolean> {
    const existing = this.#attempts.get(face)
    if (existing !== undefined) return existing
    const attempt = this.#install(face).then((installed) => {
      if (!installed && this.#retryable.delete(face)) this.#attempts.delete(face)
      return installed
    })
    this.#attempts.set(face, attempt)
    return attempt
  }

  // The faces whose attempt failed for a reason a later attempt could
  // survive: a fetch that did not arrive. `#install` marks a face here, and
  // `#attempt` reads the mark exactly once, when the attempt settles.
  readonly #retryable = new Set<string>()

  async #install(face: string): Promise<boolean> {
    const url = canvasFaceAssets.get(face)
    // A face with no row in the canvas face map is one this release declares
    // no asset for. There is nothing to fetch and nothing to install, and no
    // amount of network will change that — so this outcome IS remembered.
    // It is the matrix's "engine refuses for a face with no asset URL" row.
    if (url === undefined) return false
    // ⚠ AN OFFLINE BROWSER IS ASKED ANYWAY, and that is the spec's own row:
    // "CJK document, offline, face already cached — opens and renders
    // normally from cache." The service worker answers a held deferred asset
    // without touching the network, so the only way to reach a cached face is
    // to make the request. Declining on `navigator.onLine === false` — which
    // is what the canvas prefetch does, rightly, because its failure costs an
    // author nothing — would turn a face this browser already holds into a
    // refusal.
    const fetched = await fetchDeferredFaces([url], this.#timeoutMs, this.#request, true)
    const bytes = fetched.get(url)
    if (bytes === undefined || bytes.byteLength === 0) {
      // THE ONE RETRYABLE OUTCOME. An offline author who reconnects must be
      // able to open the document they were just refused, without reloading
      // the page and without being told to.
      this.#retryable.add(face)
      return false
    }
    try {
      await this.#client.request('install-face', { face, bytes })
    } catch {
      // An install the engine refused is a face this session does not have,
      // and it is NOT retryable: the bytes arrived and the engine would not
      // have them, which fetching them again cannot change. The ORIGINAL
      // refusal is what the author is told, never this one.
      return false
    }
    return true
  }
}
