import { catalogueFaces } from './generated/font-catalogue'

/**
 * WHICH CATALOGUE FAMILIES THIS BROWSER ACTUALLY HOLDS
 * (spec-deferred-offline-cache, story 2 — AVAILABLE LOCALLY MEANS GENUINELY
 * HELD, owner decision 2026-09-19).
 *
 * The 31 catalogue faces are in the `deferred` tier since this story: the
 * release still CONTAINS them, the worker no longer PRECACHES them, and each
 * one arrives the first time something asks for it. So "this face ships inside
 * the release" and "this browser can paint it with the network down" stopped
 * being the same sentence, and every surface that said the first while meaning
 * the second had to pick one. `familyIsInstalled` picks the second, and this is
 * the read that answers it.
 *
 * THE CACHE IS ASKED, NOT A LEDGER. The release cache the service worker fills
 * is the only authority on what these bytes are: a list the page kept for
 * itself would go stale the moment storage was evicted, and would then claim
 * offline availability for a face that is gone — precisely the untruth CAP-4
 * exists to remove. A probe costs one cache lookup per catalogue face and no
 * network at all.
 *
 * AND IT IS THIS RELEASE'S CACHE BY NAME, NEVER THE ORIGIN'S CACHES AT LARGE.
 * `caches.match(url)` searches EVERY cache on the origin, and the worker's
 * `activate` deliberately keeps a superseded release's cache alive while a
 * window can still rely on it. A face held only in that older cache would read
 * as held here while `serveFromRelease` — which opens `CACHE_NAME` and nothing
 * else — would miss it: the family offered under AVAILABLE LOCALLY and then
 * failing to paint offline, which is the exact untruth this probe exists to
 * remove. So the default lookup passes `cacheName: 'folio8-release-<releaseId>'`
 * — the name the worker derives from the same `RELEASE.id` the page carries in
 * its S1 payload — which narrows the search to that one cache.
 *
 * IT NARROWS RATHER THAN OPENS, AND THAT IS NOT A STYLE CHOICE.
 * `CacheQueryOptions.cacheName` answers exactly the question here, while
 * opening the cache by name would additionally CREATE it if it were absent — a
 * page manufacturing an empty release cache for a release it is not running.
 * Opening is also what `src/file/file-access-contract.test.ts` bans outside the
 * one module exempted to hold font bytes, and this module holds nothing: it
 * only ever asks.
 *
 * A FAILED LOOKUP READS AS NOT HELD. A storage error or a rejected match leaves
 * that family out, which is the safe direction: an unheld face left out of
 * AVAILABLE LOCALLY is still reachable through `Add fonts…`, while a held face
 * wrongly claimed would be a promise of offline availability the product cannot
 * keep.
 *
 * NO CACHE API AT ALL IS A DIFFERENT ANSWER, AND IT IS `ALL HELD`. That is not
 * the same failure wearing a different coat: where there is no Cache API there
 * is no service worker, no content-addressed release and therefore NO
 * DEFERRAL — `vite dev` and the unit suites, where every face is served
 * directly by whatever is answering. Reading "cannot tell" as "not held" there
 * would empty AVAILABLE LOCALLY on a designer that has no offline layer to be
 * honest about, which is a lie in the other direction and a broken development
 * experience besides. The truthfulness claim CAP-4 makes is about a browser
 * running the offline release, and such a browser always has `caches`.
 *
 * `match` IS INJECTED so this is testable without a service worker, and so a
 * caller can hold it to a different cache if it ever needs to.
 */
const everyCatalogueFamily = (): ReadonlySet<string> => new Set(catalogueFaces.map((face) => face.family))

/**
 * THE ANSWER BEFORE THE PROBE HAS RUN, WITHOUT WAITING FOR IT.
 *
 * The probe is asynchronous and React renders first, so something has to stand
 * in for one paint. WHERE THERE IS A CACHE that stand-in is the EMPTY set: this
 * browser is not yet known to hold any catalogue face, AVAILABLE LOCALLY is
 * allowed to be short for a moment, and claiming families and then withdrawing
 * some would offer rows that vanish under the pointer.
 *
 * WHERE THERE IS NO CACHE API the probe's answer is already known and cannot
 * change — there is no offline layer, so nothing is deferred and every face is
 * served directly — and making the caller wait a microtask to be told that
 * would flash an empty group on every `vite dev` load and in every test.
 */
export const initialHeldLocalFamilies = (releaseId?: string): ReadonlySet<string> => typeof caches === 'undefined' || releaseId === undefined ? everyCatalogueFamily() : new Set()

// The worker's own `CACHE_NAME`, spelled once. Its twin is in
// `scripts/offline-service-worker-template.mjs`; both derive it from the
// release id, so a page and its worker cannot look in two different caches.
const releaseCacheName = (releaseId: string) => `folio8-release-${releaseId}`

export async function readHeldLocalFamilies(releaseId?: string, match?: (url: string) => Promise<unknown>): Promise<ReadonlySet<string>> {
  // A PAGE WITH NO RELEASE ID HAS NO RELEASE CACHE TO OPEN, which is the dev
  // server and the unit suites — the same state `typeof caches === 'undefined'`
  // describes, and it takes the same answer for the same reason.
  const lookup = match ?? (typeof caches === 'undefined' || releaseId === undefined
    ? undefined
    : (url: string) => caches.match(url, { cacheName: releaseCacheName(releaseId) }))
  if (lookup === undefined) return everyCatalogueFamily()
  const held = new Set<string>()
  await Promise.all(catalogueFaces.map(async (face) => {
    try { if (await lookup(face.url)) held.add(face.family) } catch { /* storage that cannot be read holds nothing, as far as this page may claim */ }
  }))
  return held
}
