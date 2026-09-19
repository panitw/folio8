---
title: 'Gate the designer on the core tier and fetch the rest on demand'
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '1acc9cd568a149159efdc3cae8c240d779d6858b'
context: ['{project-root}/_bmad-output/specs/spec-deferred-offline-cache/SPEC.md', '{project-root}/_bmad-output/specs/spec-deferred-offline-cache/asset-tiers.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The worker precaches all 80 assets and broadcasts `offline-status: ready` only after
every one verifies, so `cacheReady` — and with it `engineMayStart` — means "the whole release is
here". A first-time visitor waits for 18.63 MiB before the designer is usable, 7.96 MiB of it for
work they may never do.

**Approach:** Precache the core tier only and let readiness mean the core tier; serve deferred
assets from the cache when held and fetch-verify-store them on first demand when not. Story 1
already stamps every asset's tier into the manifest the worker embeds.

## Boundaries & Constraints

**Always:**
- A deferred asset is fetched only against its own manifest entry, and its bytes are verified
  against that entry's `sha256` before being cached or returned — the same check `completeCache`
  already makes. An unverified byte never reaches the page.
- Readiness means the core tier. `cacheReady` must not be readable as a claim that any deferred
  asset is present.
- The `import.meta.env.DEV` / `dev-bypass` branch stays, and stays absent from production
  bundles — `verify-offline-release.mjs` greps shipped js/mjs/html/css for the literals
  `dev-bypass` and `Offline layer bypassed` and fails the release if either appears.
- Leave the pending-release, mandatory-upgrade, update-polling and `skipWaiting` behaviour
  exactly as it is.
- No background prefetch. On demand means on demand.
- AVAILABLE LOCALLY MEANS GENUINELY HELD (owner decision, 2026-09-19). A catalogue family whose
  bytes this browser has not fetched is removed from the family control entirely — not marked,
  not relabelled. `Add fonts…` (the font browser) is the door to it, exactly as Story 16.9
  already made it the only door to the web tier. Once its bytes are held it appears under
  AVAILABLE LOCALLY like any other held face.
- THE DOCUMENT'S OWN FONTS ARE NEVER AFFECTED. The family control's first group,
  `IN THIS TEMPLATE` (`App.tsx:5350`), lists the families the `.folio` declares and carries.
  That group is unconditional and must stay so: a font embedded in the open document is always
  offered in the typography dialog, whatever the cache holds. Verified already true; this
  constraint exists so narrowing AVAILABLE LOCALLY cannot erode it.
- A CANVAS MISS SUBSTITUTES AND SAYS SO ONCE (owner decision, 2026-09-19). When a deferred face
  cannot be fetched, the browser substitutes on the canvas and the author gets a dismissible
  warning naming the family. The engine keeps its embedded copies and stays the layout
  authority, so metrics, line breaks and the previewed PDF are unaffected — only painted glyphs
  differ. The warning is per occurrence and dismissible, never a modal block.
- THE LOAD SCREEN ITEMISES CORE ROWS ONLY (owner decision, 2026-09-19), so the list names
  exactly what is being waited for.

**Never:**
- Do not emit the literals `cache.addAll` or `fetch(event.request)` into `sw.js` — the verifier
  bans both as a generic network fallback. The on-demand path is not an exception to that ban
  and must not become one: it is restricted to deferred-tier manifest paths, with hash
  verification, and the verifier must be re-expressed to say so rather than merely stop
  matching.
- Do not change which assets the release contains, or any asset's tier.
- Do not let a deferred fetch resolve against a release other than the one this page runs.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Cold first load | Empty cache, network up | Core tier verifies; `cacheReady` true; engine starts; no deferred asset requested | N/A |
| Deferred asset first demanded, online | CJK or catalogue face requested | Fetched, hash-verified, cached, served | Hash mismatch → refuse, do not cache |
| Deferred asset demanded again | Already cached | Served from cache, no network | N/A |
| Deferred asset demanded, offline | Not cached, network down | Request fails; nothing cached; core stays usable | Failure is per-asset, never clears `cacheReady` |
| Catalogue face not yet held | Family control opened | The family is absent from AVAILABLE LOCALLY; it is offered in the font browser | N/A |
| Catalogue face held | Its bytes are cached | The family appears under AVAILABLE LOCALLY | N/A |
| Document declares a face | `.folio` carries the font | The family is listed under IN THIS TEMPLATE regardless of cache state | N/A |
| Canvas cannot paint a deferred face | Face unavailable offline | Browser substitutes; one dismissible warning names the family; preview still shows true output | Warning, never a block |
| Core asset missing at install | Integrity mismatch on a core asset | Install fails, cache deleted, `unavailable` | As today |
| Deferred asset fails to verify | Bytes do not match `sha256` | Not cached, not served | Refuse; the release stays ready |

</frozen-after-approval>

## Code Map

- `scripts/offline-service-worker-template.mjs` — `:46` `STATIC_PATHS` is the one derivation
  from `RELEASE.assets`; `:58-78` `completeCache()` is the precache loop (`:61` iterates every
  asset, `:66-69` is the hash check to reuse, `:75` writes the MARKER); `:80-85`
  `hasCompleteCache()` requires every asset present; `:93-95` install; `:99`/`:106` activate is
  the only place `offline-status: ready` is broadcast; `:110-123` fetch is cache-only and a miss
  is `Response.error()`; `:87-91` `notify()` broadcasts unless given a port.
- `scripts/verify-offline-release.mjs` — `:213` lists seven substrings `sw.js` must contain
  (`paths.has(url.pathname)` among them); `:214` bans `cache.addAll` and `fetch(event.request)`;
  `:222-229` pin `skipWaiting` to exactly one gated site; `:345-352` the dev-bypass grep;
  `:154-175` story 1's tier guards and core pin.
- `src/offline-lifecycle.ts` — `:12` `engineMayStart`; `:57` the ONLY place `cacheReady` becomes
  true (an `offline-status: ready` that `matches()` the page's release); `:44` `known()` filters
  progress events against `payload.cacheAssets`; `:59` an unknown asset URL is a silent no-op.
- `src/release-payload.ts` — `:91` asserts exactly 9 `s1` keys, `:98` exactly 2 per `cacheAssets`
  entry, `:105` exactly 6 per row. The page has NO tier data today; this is the parser to widen,
  and story 1 deliberately left it alone. `coreCacheAssetFloor`/`coreCacheAssetCeiling` are
  already exported.
- `src/LoadScreen.tsx` — `:15-22` progress maths; denominator is `payload.cachedBytes` and
  `payload.assetCount`, and `cacheReady` doubles as the clamp that stops it reading 100% early.
- `src/main.tsx` — `:46-64` wires `loadS1Payload` → `payloadForLifecycle` → `registerOfflineLifecycle`.
- `src/generated/runtime-fonts.css` — `@font-face` for all 44 faces, shipped and catalogue.
  Browsers fetch these lazily, so canvas painting is already on-demand; the worker only has to
  serve the request.
- `folio-go/fonts/fonts.go` — `go:embed` of the shipped faces. THE ENGINE CARRIES ITS OWN
  COPIES: the 11 shipped `.ttf` assets are the canvas copies, so deferring the CJK asset cannot
  affect engine capability.
- `src/App.tsx` — `:2688` the local arm of `embedInstalledFamily` does `fetch(face.url)` for a
  catalogue face's bytes; `:5318`/`:5350` build the `AVAILABLE LOCALLY` group via
  `familyIsInstalled`.
- `src/font-index.ts` — `:77-83` `FamilySource`; `:119`/`:312` treat catalogue membership as
  proof the machine holds the face. `src/font-store.ts` — the IndexedDB fetched-and-kept tier.

## Tasks & Acceptance

**Execution:**
- [x] `scripts/offline-service-worker-template.mjs` -- precache core-tier assets only; complete
  on the core set; add an on-demand path that fetches, hash-verifies and caches a deferred
  manifest asset on first request -- the gate and the fetch are two halves of one mechanism.
- [x] `scripts/verify-offline-release.mjs` -- re-express the generic-fallback ban so it still
  refuses an unrestricted fallback while permitting the verified deferred path; add red proofs
  for a deferred fetch that skips verification and for a core asset left out of the precache.
- [x] `src/release-payload.ts` -- carry the tier into the S1 payload and widen the parser's key
  counts deliberately, with the rejection reasons kept distinct -- the page cannot compute core
  readiness or core progress without it.
- [x] `src/offline-lifecycle.ts` -- make `cacheReady` mean the core tier, leaving the
  pending-release and timeout behaviour alone.
- [x] `src/LoadScreen.tsx` -- count progress against the core tier.
- [x] `src/font-index.ts`, `src/App.tsx` -- make `familyIsInstalled` / the `local` tier mean the
  bytes are held rather than shipped, so an unfetched catalogue family leaves the family control
  and remains reachable only through `Add fonts…`; leave `IN THIS TEMPLATE` unconditional.
- [x] `src/App.tsx` -- raise one dismissible warning naming the family when a deferred face
  cannot be fetched for the canvas.

**Acceptance Criteria:**
- Given an empty cache, when the designer loads, then it becomes interactive after the 29
  core assets verify, and no deferred asset is requested during that load.
- Given a ready designer, when a catalogue face is first picked online, then its bytes are
  fetched once, hash-verified, cached, and the second pick performs no network request.
- Given a ready designer with the network disconnected, when a deferred asset is demanded, then
  the request fails without clearing `cacheReady` and the designer stays usable.
- Given `npm run build`, when the verifier runs, then `sw.js` still satisfies the required
  substrings and the re-expressed fallback ban, and every new red proof fails on its own message.
- Given an unfetched catalogue family, when the family control is opened, then that family is
  absent from AVAILABLE LOCALLY and present in the font browser; once its bytes are held it
  appears under AVAILABLE LOCALLY without a reload.
- Given a document declaring a font it carries, when the typography dialog is opened, then that
  family is listed under IN THIS TEMPLATE whatever the cache holds.
- Given `npm run test` and `npm run verify:offline:red`, then both pass.

## Implementation Notes

**The worker has one network read, and the verifier's ban says so.** `fetchVerified(asset)`
is the single `fetch(` in the emitted worker: it takes a manifest ENTRY, fetches that entry's
url, and compares the digest before returning. `completeCache` (install) and `serveFromRelease`
(on demand) are its only callers. The old ban — two absent substrings — is re-expressed as a
whitelist of one: `verify-offline-release.mjs` counts network reads, requires exactly one,
requires the fetch-through-digest block verbatim and contiguously, and pins the deferred lookup
to `DEFERRED_ASSETS.get(pathname)` plus its `Response.error()` refusal. The two old spellings
are still banned by name beside it, because a guard found only by arithmetic is a guard nobody
greps for. Four new red proofs: `precache-not-the-core-tier`, `unverified-network-read`,
`deferred-fetch-unverified`, `unrestricted-deferred-path`.

**The core pins are matched as whole lines, not substrings.** The first attempt used
`sw.includes(...)`, and the red proof that drops an asset off the derivation
(`…tier === 'core').slice(1)`) sailed through it — the mutated line still CONTAINS the pin.
The check now compares trimmed lines.

**`AVAILABLE LOCALLY` reads the release cache, and `Add fonts…` became a real door.**
`familyIsInstalled` takes the held set as a REQUIRED second argument, so both surfaces that
read it had to be looked at together. `held-local-faces.ts` probes `caches.match` per catalogue
face; the family control re-probes each time the list opens, which is what makes the transition
reload-free. A consequence that had to be built rather than assumed: with an unheld catalogue
family now reading `addable` in the font browser, `installFamily` had to gain a `local` arm —
installing a catalogue face means fetching its bytes through the worker. Without it the browser
would have offered Install on a row its own handler refused.

**No Cache API is not the same answer as an empty cache.** `initialHeldLocalFamilies()` returns
every catalogue family where `caches` is undefined — `vite dev` and the unit suites, which have
no service worker, no content-addressed release and therefore no deferral to be honest about —
and an empty set where there is a cache to probe. Reading "cannot tell" as "not held" would
empty the group on a designer with no offline layer at all.

**The canvas-miss warning needed an AD-17 carve-out, and it is the narrowest one.**
`document.fonts` is prohibited across the designer because the browser may not be an authority
on layout. `canvas-face-misses.ts` subscribes to `loadingerror` and reads a family NAME; it
measures nothing, and the engine's embedded copies remain the layout authority.
`canvas-authority-contract.test.ts` waives `document.fonts` inside that one function, asserts
the function still exists and is unique, and leaves `new FontFace` and every non-font
prohibition red there.

**Two e2e specs asserted the guarantee this story deliberately narrows**, and both were
rewritten rather than accommodated. `startup-dialog.spec.ts` opened an example offline that had
never been opened — true only because all 80 assets were precached — and now opens it once
online and again offline, which is the guarantee the spec actually makes. The two
`font-embed-boundary.spec.ts` catalogue measurements now prime the cache through the worker
(`holdEveryCatalogueFace`), after a reload so the worker is actually CONTROLLING the page —
without that reload the priming fetches bypass the worker and cache nothing, which read as a
product failure for one debugging round.

**Measured after the change:** 29 core assets / 10.67 MiB blocking, 51 deferred / 7.96 MiB —
`asset-tiers.md`'s figures exactly, now read from the manifest's own `tier` field.

## Spec Change Log

## Review Triage Log

**Round 1 — all findings accepted and fixed.**

- The held probe searched every cache on the origin; a face held only in a superseded release's
  cache read as held while `serveFromRelease` would miss it. Scoped to
  `cacheName: 'folio8-release-<releaseId>'` — narrowing rather than opening, since opening would
  create a cache for a release this page is not running, and is what the local-file contract bans
  outside `font-store.ts`. `releaseId` now travels from the S1 payload.
- The tier behaviour was vacuous at every level (jsdom has no `caches`; every fixture all-`core`).
  Added `held-local-faces.test.ts` (partial held set, throwing lookup, no-release fallback), a
  mixed-tier `LoadScreen` case that asserts the two totals DIFFER before asserting which is used,
  an `s1-tier-drift` verifier check plus its red proof (page bootstrap moved in step so the guard
  is reachable on its own message), and two VM-harness worker cases — install asks for core URLs
  only, and `ready` is answered with the deferred tier absent.
- `font-index.test.ts`'s run-structure tests were narrowed to an all-held set, which is the one
  input where the invariant still held. They now measure TIER, and assert with a half-held set
  that installedness is deliberately NOT two runs — the family control filters before it groups.
  The docblock at `:189` was corrected to match.
- `serveFromRelease` had no in-flight map (concurrent demands each fetched 4.72 MiB) and its cache
  write could turn verified bytes into a refusal. One shared `fetchVerifiedAndKeep` per pathname,
  cleared on settle; every consumer gets a clone so the shared body is never disturbed; the keep
  fails independently of the serve.
- The canvas warning deduped nothing within an event (duplicate React keys), un-dismissed itself
  on the next event, announced nothing (`role="status"` on each `<li>` overrode `listitem`, and a
  pre-populated list is not read out), and said "could not be fetched" for a cause that is also a
  parse failure. Fixed at each point: dedupe in the watcher, a separate sticky dismissal set, the
  always-mounted `<ul>` as the live region with plain children, and "could not be loaded".
- `installFamily`'s `local` arm now refuses by name when `navigator.serviceWorker.controller` is
  absent: an uncontrolled page fetches past the worker and caches nothing, so the install would
  have reported success over a family that stayed absent.
- A cold load does request deferred assets (the dialog's four thumbnails). A new e2e asserts the
  boundary: no deferred URL before the engine wasm, and the only deferred assets a first load
  touches are the starter and those four. The other test's title lost "with no second request".
- `App.tsx:2590`'s docblock (only a `web` row has anything to install) corrected.
- `cache-asset-tier-missing` renamed `cache-asset-tier-unrecognised`: the exact-key-count check
  runs first, so nothing missing ever reaches that arm.
- `LoadScreen` now calls `coreCachedBytes` instead of re-deriving the sum.

Layers: blind-hunter (BH), edge-case-hunter (EC), verification-gap (VG). Gap findings arrive pre-verified per that layer's evidence rules.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| BH2 / VG1-VG6 | The story's new tier-aware behaviour is untested at every level: worker core-precache, core readiness, LoadScreen core denominator and core rows, the S1 `tier`, `readHeldLocalFamilies`, the canvas warning, and `installFamily`'s local arm | high | Confirmed. VG supplies a concrete revert for each that leaves the suite green — e.g. restoring `payload.cacheAssets`/`payload.cachedBytes` in LoadScreen passes every assertion because its fixtures are all-`core`; `tier: assetTier(...)` → `tier: 'core'` in the generator passes verify and every red proof. jsdom has no `caches`, so `initialHeldLocalFamilies()` returns every family and the existing suite keeps its pre-story behaviour. Grouped as one root cause: the test environments default to all-held / all-core, so every new branch is vacuous. |
| BH1 | `font-index.ts:189`'s "exactly two runs" invariant is false once the local tier is partially held, and the two run-structure tests were narrowed to `allHeld` | medium | Confirmed at `font-index.ts:189` and `font-index.test.ts:409`/`:428`. `allHeld` is the one input under which the invariant still holds, so a live measurement became vacuous. Grouped with the above. |
| BH11a / EC4 / EC16 / VG-other | The held-set probe asks the wrong cache | high | Confirmed at `held-local-faces.ts:61`: it calls global `caches.match(url)`, which searches every cache on the origin, while its own comment claims it "looks in the release cache the service worker fills". `activate` retains a superseded release's cache while a window is open, so a face held only there reads as held — and `serveFromRelease` looks only in the current `CACHE_NAME` and would miss. That is exactly the CAP-4 untruth the probe exists to remove. |
| BH9 / BH10 / EC9 / EC17 | First use of a deferred asset can fetch it several times, and a failed cache write discards bytes already verified | medium | Confirmed by reading `serveFromRelease`: no in-flight map, and `cache.put` sits inside the same `try` as the fetch. CSS `@font-face`, the specimen read and the install path can all demand one URL at once — 4.72 MiB each on the CJK face — and a `QuotaExceededError` on the write turns a verified response into `Response.error()`. |
| BH3 / BH4 / EC6 / EC7 | The substitution warning misreports its cause, does not announce, duplicates rows, and forgets dismissals | medium | Confirmed. `watchCanvasFaceMisses` is unfiltered and maps without a `Set`, so one event carrying two cuts of a family yields duplicate React keys; the copy says "could not be fetched" though `loadingerror` also fires when held bytes fail to parse; `<li role="status">` overrides the implicit `listitem` role and the `<ul>` is only mounted once populated, which screen readers generally do not announce. The reassurance itself is sound — the engine holds embedded bytes for every face in a document. |
| EC18 | A cold first load does request deferred assets: the startup dialog draws four example thumbnails, which story 1 tiers `deferred` | medium | Confirmed by running the classifier on a thumbnail URL. They are requested after the gate opens, so the blocking load is core-only, but the frozen matrix row says "no deferred asset requested" without naming that boundary. Patched by asserting the boundary rather than by changing behaviour; the reading is disclosed to the owner. |
| BH5 | `installFamily`'s docblock still says only a `web` row has anything to install, immediately above the new `local` arm | low | Confirmed at `App.tsx:2590`. This codebase treats these blocks as the contract, so a contradicted one misleads the next reader. |
| BH6 | `cache-asset-tier-missing` never fires for a missing tier | low | Confirmed by reading `parseS1Payload`: the `Object.keys(...).length === 3` check precedes the tier arm, so a pre-tiering two-key entry rejects as `cache-assets-invalid`. The new reason only fires for a present-but-unrecognised value. |
| BH7a / EC15 / VG-other | `coreCachedBytes` is exported and never called; `LoadScreen` re-derives the same sum inline | low | Confirmed by grep: the only match is its own definition. Two definitions that can drift. |
| BH8 / EC12 | The verifier counts the text `fetch(` rather than network reads | low | Confirmed at the new `networkReads` line. Evadable by `self.fetch(` or an alias, and brittle against any future comment containing the literal. Still strictly stronger than the two-spelling ban it replaced. |
| BH12 | The rewritten e2e test's title claims "no second request" while nothing counts requests | low | Confirmed by reading the test: `context.setOffline(true)` proves "did not need the network", not "made none". |
| EC1 | Drawing specimens in the font browser fetches catalogue faces over the network and promotes them to held | medium | Confirmed at `browserSpecimenBytes`' `local` arm — a plain `fetch(source.face.url)` per row. Deferred to the owner rather than patched: the fetch is demand-driven (the author asked to see the font), the cost is bounded by the catalogue and user-initiated, and the alternative — blank specimens until install — is a UX decision this spec does not settle. |
| BH11b | Every fetched deferred asset is lost on each release update | medium | Real and caused by this story: `activate` deletes the previous release cache and the new one precaches core only, so catalogue faces the author had fetched must be fetched again and AVAILABLE LOCALLY empties silently. Deferred: carrying verified entries across releases is a design decision the spec does not settle. |
| EC10 | A core asset evicted after install can never be refetched | medium | Real but pre-existing: a core cache miss was `Response.error()` before this story too. The deferred path makes the asymmetry visible without introducing it. Deferred. |
| EC2 | `installFamily`'s local arm can fetch while the page is uncontrolled, caching nothing yet reporting success | medium | Plausible and unrefuted; folded into the patch round as a guard on the install arm. |
| EC3 / EC5 | Probe races: install resolving before its refresh, and a stale probe overwriting a newer held set | low | Real but narrow; the set is re-probed on each dropdown open, so a stale answer is corrected by the next open. |
| EC8 | A Dismiss click can be swallowed by the canvas click-capture | low | Confirmed narrow: `consumeClick()` returns true only immediately after a drag, so this needs a Dismiss clicked directly after a marquee. |
| EC11 | A never-opened documentation page offline shows the browser's error page, not a designer refusal | low | Real, and explicitly CAP-3's (preflight and its worded refusal), not this story's. Deferred to story 3. |
| EC13 | The e2e priming helper may run before the worker controls the page | low | The implementation already reloads before priming for exactly this reason; no residual defect shown. |
| EC14 | An empty core row list would render a manifest heading with nothing under it | low | Unreachable: the core tier is pinned at 29 and always contains the engine row. |
| BH7b | The core tier's byte budget is still unguarded | medium | Real, and already recorded as deferred work by story 1's review. Not re-filed. |

## Design Notes

**The engine is not affected by deferral.** `folio-go/fonts/fonts.go` embeds the shipped faces,
so the 8.11 MiB core wasm already carries NotoSans, NotoSansSC, Roboto and their cuts. The
shipped `.ttf` assets exist for the canvas, reached through `runtime-fonts.css`. Deferring the
4.72 MiB CJK asset therefore costs nothing but the canvas's first CJK paint.

**Both deferred consumers are the same request.** Canvas painting (CSS `@font-face`) and engine
embedding (`fetch(face.url)` at `App.tsx:2688`) are both same-origin GETs for content-addressed
URLs that appear in `RELEASE.assets`. One worker path serves both.

## Verification

**Commands:**
- `cd folio-designer && npm run build` -- expected: succeeds, verifier green.
- `cd folio-designer && npm run verify:offline:red` -- expected: all proofs pass, new ones
  included.
- `cd folio-designer && npm run test` -- expected: green.
- `cd folio-designer && npm run lint && npm run typecheck` -- expected: clean.
- `cd folio-designer && npm run test:e2e` -- expected: the offline lifecycle specs still pass.

**Manual checks:**
- With a cold cache and DevTools throttling, confirm the designer becomes interactive after the
  core tier and that no `catalogue-*.ttf` or `noto-sans-cjk*.ttf` is requested during load.
  Record the transferred bytes before and after.
