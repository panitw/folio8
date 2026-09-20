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
 * THE TWO QUESTIONS ONE PROBE ANSWERS, AND THEY ARE NOT THE SAME QUESTION
 * (spec-install-all-face-cuts story 3, review finding F1).
 *
 *   `usable`   — CAN THESE BYTES BE USED? The family's upright **Regular** is in
 *                the cache. That is the face a pick embeds, so a family whose
 *                Regular is here can be applied to a component and painted
 *                offline, whatever else it is short of. `familyIsInstalled`
 *                reads this, and the family control's AVAILABLE LOCALLY group
 *                is drawn from it.
 *
 *   `complete` — IS ANYTHING LEFT TO FETCH? EVERY cut the family declares is in
 *                the cache. `familyIsComplete` reads this, and the font
 *                browser's row state is drawn from it: a family short of a cut
 *                must keep offering that cut for install.
 *
 * ⚠ THEY WERE ONE SET AND THE FUSION WAS A LIVE DEFECT. Both predicates read
 * the all-cuts set, so a family holding its Regular alone read NOT INSTALLED and
 * DROPPED OUT of AVAILABLE LOCALLY — a font whose bytes are sitting in this
 * browser's cache, unusable, with nothing on screen to say why. The reachable
 * path needs no install at all: `browserSpecimenBytes` fetches each local row's
 * Regular to draw its specimen, so merely BROWSING caches one cut of a family
 * and puts it in exactly that state.
 *
 * THEY ARE A RECORD RATHER THAN TWO ARGUMENTS SO THE WRONG ONE CANNOT BE
 * PASSED. Two bare `ReadonlySet<string>` parameters are interchangeable to the
 * compiler, which is how the fusion survived review once already; a caller now
 * names the field, and the field names the question.
 */
export type LocalFaceHoldings = Readonly<{ usable: ReadonlySet<string>; complete: ReadonlySet<string> }>

/**
 * HOW MANY CUTS EACH CATALOGUE FAMILY DECLARES — the denominator the probe
 * below counts against (spec-install-all-face-cuts, story 3).
 *
 * A family used to be one catalogue face, so "this browser holds the family"
 * and "this browser holds a face of the family" were the same sentence and the
 * probe could `held.add(face.family)` on the first hit. A family now declares
 * up to four cuts, and a family whose Regular alone is cached would read as
 * COMPLETE under that rule: AVAILABLE LOCALLY would promise bytes for a bold
 * this browser does not have, and CAP-4 would never offer to finish it. So the
 * family is held only when EVERY cut it declares is in the cache.
 *
 * ⚠ ROBOTO IS DELIBERATELY SHORT HERE, AND THAT IS CORRECT. The catalogue
 * declares plain `Roboto` and no Roboto cut: `Roboto Bold`, `Roboto Italic` and
 * `Roboto Bold Italic` ship as hardcoded CORE release assets rather than as
 * catalogue rows (see `scripts/build-wasm.mjs`), so they are precached before
 * anything asks and cannot be missing while the release is usable. Counting
 * them here would mean probing a URL this module has no map to. Roboto's
 * denominator is therefore 1, and the family reads as held once its one
 * deferred face is — which is the truth about what this browser can paint.
 */
const declaredCutsPerFamily: ReadonlyMap<string, number> = (() => {
  const counts = new Map<string, number>()
  for (const face of catalogueFaces) counts.set(face.family, (counts.get(face.family) ?? 0) + 1)
  return counts
})()

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
export const initialLocalFaceHoldings = (releaseId?: string): LocalFaceHoldings => {
  const answer = typeof caches === 'undefined' || releaseId === undefined ? everyCatalogueFamily() : new Set<string>()
  // BOTH FIELDS TAKE THE SAME STAND-IN, and for the same reason: where there is
  // no cache there is no deferral, so every family is both usable and complete;
  // where there is one, nothing is known yet and claiming either would offer
  // rows that vanish under the pointer.
  return { usable: answer, complete: answer }
}



// The worker's own `CACHE_NAME`, spelled once. Its twin is in
// `scripts/offline-service-worker-template.mjs`; both derive it from the
// release id, so a page and its worker cannot look in two different caches.
const releaseCacheName = (releaseId: string) => `folio8-release-${releaseId}`

// THE ONE PLACE THE RELEASE CACHE IS OPENED FOR READING, spelled once because
// three readers below need the identical rule and a second spelling of it would
// be a second answer to "is there a cache to ask".
//
// A PAGE WITH NO RELEASE ID HAS NO RELEASE CACHE TO OPEN, which is the dev
// server and the unit suites — the same state `typeof caches === 'undefined'`
// describes, and it takes the same answer for the same reason.
const releaseCacheLookup = (releaseId?: string, match?: (url: string) => Promise<unknown>): ((url: string) => Promise<unknown>) | undefined =>
  match ?? (typeof caches === 'undefined' || releaseId === undefined
    ? undefined
    : (url: string) => caches.match(url, { cacheName: releaseCacheName(releaseId) }))

export async function readLocalFaceHoldings(releaseId?: string, match?: (url: string) => Promise<unknown>): Promise<LocalFaceHoldings> {
  const lookup = releaseCacheLookup(releaseId, match)
  if (lookup === undefined) { const every = everyCatalogueFamily(); return { usable: every, complete: every } }
  // EVERY CUT IS PROBED, IN ONE PASS, AND THE TWO ANSWERS ARE DERIVED FROM THE
  // SAME READ. Two probes would ask the same cache the same question twice and
  // could disagree across an eviction landing between them.
  const cutsHeld = new Map<string, number>()
  const usable = new Set<string>()
  await Promise.all(catalogueFaces.map(async (face) => {
    try {
      if (!(await lookup(face.url))) return
      cutsHeld.set(face.family, (cutsHeld.get(face.family) ?? 0) + 1)
      // THE REGULAR IS WHAT MAKES A FAMILY USABLE, because the Regular is what
      // a pick embeds. A family holding only its Bold can paint nothing the
      // author can ask for, so it is not counted here.
      if (face.style === 'Regular') usable.add(face.family)
    } catch { /* storage that cannot be read holds nothing, as far as this page may claim */ }
  }))
  // A FAMILY IS COMPLETE ONLY WHEN ALL OF ITS CUTS ANSWERED. One hit per family
  // was the old rule and it is a false claim once a family declares four — see
  // `declaredCutsPerFamily` above.
  const complete = new Set<string>()
  for (const [family, count] of cutsHeld) if (count === declaredCutsPerFamily.get(family)) complete.add(family)
  return { usable, complete }
}

/**
 * IS THIS ONE FACE'S BYTES IN THIS RELEASE'S CACHE?
 *
 * THE PRESS-TIME PROBE (spec-install-all-face-cuts story 3, finding F2). The
 * panel decides what to SAY from the family-level holdings above, which are
 * read when the family list opens; the embed path asks about the one URL it is
 * about to read, at the moment it reads it, because the cache is the authority
 * and it can lose an entry between those two moments. A press that fetched a
 * missing cut over the network instead would make an author's keystroke depend
 * on a connection — the thing the committed tier exists to avoid.
 *
 * IT ANSWERS `true` WHERE THERE IS NO CACHE API, for the reason every other
 * read in this module does: no cache means no service worker, no
 * content-addressed release and therefore NO DEFERRAL — `vite dev` and the unit
 * suites, where every face is served directly by whatever is answering.
 */
export async function localFaceIsHeld(url: string, releaseId?: string, match?: (url: string) => Promise<unknown>): Promise<boolean> {
  const lookup = releaseCacheLookup(releaseId, match)
  if (lookup === undefined) return true
  try { return Boolean(await lookup(url)) } catch { return false }
}
