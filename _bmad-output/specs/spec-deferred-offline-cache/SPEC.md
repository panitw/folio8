---
id: SPEC-deferred-offline-cache
companions: [asset-tiers.md]
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Deferred offline cache for the designer's first load

## Why

A pain, on the first screen anyone ever sees. The designer refuses to start until a service
worker has downloaded and verified every asset in the release — today 80 assets and 18.63 MiB
over the wire — so a first-time visitor stares at a progress screen through the whole download
before touching anything. Nearly half of those bytes are for work most authors never do: a
4.72 MiB CJK font, 31 catalogue faces totalling 3.01 MiB, bundled examples and documentation.
The offline guarantee those bytes buy is real and worth keeping, but it is currently charged in
full, up front, to every visitor, including the one who leaves after thirty seconds. The cost
is also growing unwatched: `spec-folio` still records the accepted first load as "~9 MB", which
the measured release passed some time ago.

## Capabilities

- **CAP-1 — The designer starts on the core tier alone**
  - **intent:** An author reaches a usable designer once the assets needed to open, edit,
    preview and render an ordinary Latin or Thai document are cached and verified — not once
    the whole release is.
  - **success:** With a cold cache, the designer is interactive after the core tier
    (30 assets, 10.82 MiB — see `asset-tiers.md`) verifies and no later; the deferred tier is
    provably not requested during that first load.

- **CAP-2 — Deferred assets are fetched the first time they are needed, then kept**
  - **intent:** An asset outside the core tier is fetched at the moment an author's action
    first requires it — choosing a catalogue face, opening a document with CJK text, opening
    the examples gallery — and is cached permanently thereafter, so the cost is paid once by
    the authors who incur it and never by the ones who do not.
  - **success:** A session that never uses CJK, the catalogue or the bundled examples transfers
    no bytes beyond the core tier; the second use of a deferred asset transfers nothing and
    works with the network disconnected.

- **CAP-3 — Documents are preflighted on open, and the preflight fetches**
  - **intent:** Opening a document establishes up front every asset it needs, fetches whatever
    is missing while the network is available, and refuses the open only when something is both
    missing and unreachable — so an author learns what is wrong before they start editing
    rather than at the moment a render fails.
  - **success:** A first CJK document opens successfully on a connected machine, fetching the
    CJK font as part of the open; the same open with no network is refused, naming each missing
    asset; no document ever opens and then fails mid-edit for an asset that was already absent
    at open.

- **CAP-4 — The offline guarantee is stated per asset, truthfully**
  - **intent:** Wherever the product tells an author that something is available locally — the
    font browser's `AVAILABLE LOCALLY` tier above all — the claim reflects what this browser
    actually holds, not what the release nominally ships.
  - **success:** A catalogue face that has not yet been fetched is not presented as locally
    available; once fetched, it is, and the transition needs no reload.

- **CAP-5 — The release carries its tiering**
  - **intent:** The tier an asset belongs to is decided at build time and travels with the
    release, so the worker, the app and the verifier all read one authority rather than three
    heuristics.
  - **success:** The emitted manifest assigns every asset to exactly one tier; a build that
    leaves an asset untiered, or that moves an asset into the core tier without the bound
    being raised deliberately, fails verification rather than shipping.

## Constraints

- **Byte-identity survives untouched.** A missing deferred font is refused, never substituted.
  The canvas and the preview keep showing the real production output or nothing at all;
  no fallback face is ever rendered in place of the one the document declares.
- **A ~10.82 MiB core gate is the accepted destination.** The engine wasm is 8.11 MiB of it and
  stays whole and in the core tier; tiering roughly halves the first load and that is the win
  this change is scoped to deliver. This supersedes `spec-folio`'s "~9 MB first load".
- **pdf.js and its cmaps are core.** Preview sits close enough to the primary workflow that
  its 0.76 MiB across eleven assets does not justify a second refusal path through it.
- **Readiness means the core tier, and says so.** `cacheReady` today asserts all 80 assets
  verified and gates the engine on it. The replacement signal must assert the core tier and
  must not be readable as a claim that the deferred tier is present.
- **A deferred fetch is bound to the running release.** It resolves the content-addressed URL
  of the release this page is running, so a worker with a pending release behind it can never
  mix a new asset into an old release's cache.
- **The cache-asset bounds and the S1 payload split with the tiers.** `minimumCacheAssets`,
  `maximumCacheAssets` and `warnCacheAssets` in `folio-designer/src/release-payload.ts`, the
  payload shape that reads them, and `scripts/verify-offline-release.mjs` all count one
  undifferentiated asset set today; the core tier is the set that now needs a defended ceiling.
- **No background prefetch.** The deferred tier is fetched on demand and by nothing else. A
  post-load top-up would re-spend the bytes the core tier just saved.
- **The tiering is invisible to the author.** No "download everything" control and no
  cache-status panel. The only place tiering surfaces is a refusal that names what is missing
  (CAP-3) and the per-face truthfulness of CAP-4.

## Non-goals

- **Shrinking the engine wasm.** Compressing, splitting or streaming-compiling the 8.11 MiB
  engine is a separate problem from tiering, and this spec does not attempt it.
- **A cache management UI.** No inspector, no eviction control, no manual sync, no "make
  available offline" button.
- **Serving fonts from a third-party CDN.** Deferred assets come from the same origin and the
  same content-addressed release; the forbidden-font-host rules are unchanged.
- **Changing what the release contains.** No asset is dropped, added or re-encoded by this
  change — only when it is fetched.
- **Revising the update, pending-release or mandatory-upgrade behaviour.** Those keep working
  as they do, over whichever assets are cached.

## Success signal

A first-time visitor on a cold cache reaches an editable, previewable document after
transferring 10.82 MiB instead of 18.63 MiB and verifying 30 assets instead of 80 — and a
session that stays on Latin text never transfers the other 7.81 MiB at all. An author who has
used the designer once still opens it, edits, previews and renders with the network
disconnected, and is told precisely which asset is missing on the one occasion that is not true.

## Assumptions

- A catalogue face picked mid-session while offline is refused the same way a preflight miss is
  refused — the preflight-on-open decision applied consistently to mid-session demand.
- The existing IndexedDB face store (`folio-designer/src/font-store.ts`) and the web-font tier
  it already serves are the precedent for the deferred fetch-and-keep path, not a reason to
  build a second one.
- `spec-folio`'s "~9 MB first load is accepted" constraint is superseded by this spec rather
  than contradicted by it; that spec needs the corresponding update.
