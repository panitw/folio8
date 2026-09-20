---
title: "Take the CJK face out of the designer's engine"
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '431e288680d66dc9b3a5967688d8cb97b3895969'
context: ['{project-root}/_bmad-output/specs/spec-deferred-offline-cache/SPEC.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The designer's engine wasm embeds its own copy of `Noto Sans SC` — 4.72 MiB brotli of
the 8.11 MiB blocking download, shipped to every visitor who will only ever type Latin. The
release already serves the identical bytes as a deferred asset for canvas painting, so the face is
downloaded twice and the larger copy is charged to the first load.

**Approach:** The designer's engine stops embedding the CJK face and receives it from JavaScript,
held once for the session rather than passed per request. Because story 4 made the engine *name*
the face it is missing, the designer does not need to predict the need: it acts on the refusal —
fetches the named face from the deferred asset it already has a URL for, installs it, and retries.
A build tag keeps this confined to the designer's wasm; every other consumer of `fonts.Shipped()`
is untouched.

## Boundaries & Constraints

**Decisions (owner-settled or investigation-settled, do not revisit):**
- **Discovery is reactive, never proactive.** The face is fetched when a render or projection
  refuses with `TEXT_FACE_ABSENT`, not from scanning the document's font chains. The starter
  template's only chain ends in `Noto Sans SC` and contains no text at all, so chain-scanning
  would refetch 4.72 MiB on every first load and undo this entire spec.
- **Transport is raw bytes, not base64.** The face is 10.11 MiB raw, over the 8 MiB payload bound
  enforced on both sides (`engine-protocol.ts:6`, Go's `decodeBase64Bounded`), and base64 would
  build a ~13.5 MiB intermediate string through a `String.fromCharCode` loop. Use a dedicated
  host entry point taking a `Uint8Array`, the way `wasm/cmd/render/main.go`'s `bytesArg` does.
  The existing `handle(jsonString)` envelope and its 8 MiB bound are unchanged.
- **The tag trims `fonts`, and the engine merges over it.** `//go:build` variant files in
  `folio-go/fonts`, following the `cgo`/`!cgo` pair at `folio-go/cshared/cmd/folio8/`. Under the
  tag `Shipped()` returns ten faces; the installed CJK bytes are merged over that set.
- **A byte ceiling now guards the core tier.** Today nothing does —
  `generate-offline-release.mjs:214` says of its own total "It is a MEASUREMENT, not a budget, and
  nothing in this repository compares it to a threshold." Without one, re-adding the embed would
  silently restore 4.72 MiB.

**Always:**
- The core asset **count** stays 30: the wasm is still one asset, only smaller. What this story
  pins is its **weight**, measured from the build and derived by the reader idiom story 1
  established — never a retyped number.
- A CJK document still lays out, paginates, previews and renders **byte-identically** once the
  face is installed. No substitute face ever reaches the engine.
- The install is idempotent and survives for the session. The worker is never restarted
  (`engine-client.ts:76` terminates without recreating), so one install per face per session.

**Never:**
- Do not remove any face from the untagged `fonts.Shipped()`, and do not change what any other
  consumer embeds. `folio-js`, `folio-dotnet`, the six CJK golden fixtures and the eleven-face
  documentation stay exactly as they are.
- Do not let the CJK face become byte-embedded in a `.folio`. Picking a catalogue family embeds
  bytes; a shipped family writes a name. It must stay on the name-declared path or every CJK
  document gains 10 MiB.
- Do not widen the 8 MiB bound on the existing request envelope.
- No background prefetch of the face on boot. On demand still means on demand.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Latin-only session | starter or Latin document | Engine never asks for the CJK face; nothing is fetched | N/A |
| First CJK document, online | document with Han text, face not yet installed | Engine refuses once naming the face; designer fetches, installs, retries; document opens and renders byte-identically | transparent to the author |
| Second CJK document, same session | face already installed | Opens with no refusal and no fetch | N/A |
| CJK document, offline, face never fetched | no network, no cached asset | Open is refused, naming the face; no substitute is laid out | worded refusal, per CAP-7 |
| CJK document, offline, face already cached | no network, asset in cache | Opens and renders normally from cache | N/A |
| Engine refuses for a face with no asset URL | a document naming a face the release does not ship | Refusal surfaces to the author; no retry loop | refuse once, never retry forever |
| Canvas table label needing an absent face | CJK text in a table column label | Refuses like any other text; not silently dropped | see `page_setup.go:1632` |

</frozen-after-approval>

## Code Map

**Go — the engine's font input**
- `folio-go/internal/wasm/engine.go:168` `Engine` struct, `:187` `NewEngine(clock)`. `clock` is the
  existing precedent for a caller-injected dependency (its test is `internal/wasm/clock_test.go`);
  a held `FontSet` is the second, injected the same way.
- The seven `fonts.Shipped()` call sites, all in that file — `:133` `GroupMovePreview` (hot, during
  drag), `:164` `PreviewIdentity`, `:203` `load` (once per open), `:259` `Render`, `:325` and
  `:349` `Apply` (hot — every committed command, ~every 200 ms while typing), `:388` `restore`.
  All must read one set the engine holds.
- Parsing is font-free: `ParseTemplate`/`SerializeTemplate` (`:195`, `:199`) take no `FontSet`.
  Fonts enter only at the canvas projection, so `load` is where a CJK document first refuses.
- `folio-go/wasm/cmd/engine/main.go:80` `main` — where the new raw-bytes host entry point is
  registered beside `handle`. `folio-go/wasm/cmd/render/main.go:136` `bytesArg` and `:150`
  `fontSet` are the shape to copy.
- Dispatch switch at `wasm/cmd/engine/main.go:97`; host tests in `main_test.go`.

**Go — the build-tagged font set**
- `folio-go/fonts/fonts.go` — one file, eleven package-scope `//go:embed` vars, all referenced by
  `Shipped()` at `:159`. No linker DCE is possible, so a filtered set at the call site cannot drop
  the bytes; the package must not embed them in that build.
- Precedent: `folio-go/cshared/cmd/folio8/main.go:1` (`//go:build cgo`) and `nocgo.go:1`
  (`//go:build !cgo`), with a guard test at `imports_test.go:64-71` asserting both constraints stay.
- `folio-go/fonts/accounting_test.go:50,144,263-276` joins every `fonts/<face>/NOTICE.md` to
  `Shipped()` **by bytes**, in both directions, and is total over `Shipped()`. Under the tag it must
  exclude `notosanssc/` or it fails. `fonts_test.go:108-154` pins per-key presence and byte floors
  the same way.
- Tag plumbing: `folio-designer/scripts/wasm-vcs-stamp.mjs:60` `ENGINE_BUILD_FLAGS` is the single
  declaration point (`build-wasm.mjs:50` calls `buildEngineWasm`). CI already builds with a custom
  tag at `.github/workflows/ci.yml:135`, so the pattern exists.

**JS — supply and retry**
- `folio-designer/src/engine.worker.ts:47` captures the host, `:113` builds the one request
  envelope, `:114` calls `handle`. Single chokepoint. `bytesToBase64` at `:161` is what the raw
  transport avoids.
- `folio-designer/src/engine-client.ts:52` `request`, `:180` `workerPost` (transfer list), `:33`
  `#ready`, `:76` `terminate` (no restart). `engine-protocol.ts:3` closed `EngineOperation` union
  and the runtime allow-list duplicated at `:1136` — a new operation needs both.
- `folio-designer/src/startup-sequence.ts:12` — the existing "tell the engine something first"
  seam, after the ready handshake and before any document.
- `folio-designer/src/document-face-prefetch.ts:128` `prefetchDeferredFaces` already has the
  `ArrayBuffer` in hand at `:139` and throws it away; `:73` `paintedCanvasFaces` reads the face
  names out of the engine's own projection — which is exactly the cycle this story cannot rely on.
- `folio-designer/src/generated/canvas-face-assets.ts` maps face name → content-addressed URL; the
  refusal names the face, this map turns it into the URL to fetch.
- `folio-designer/src/App.tsx:3074-3117` `installOpenedDocument` — `load` at `:3078`, prefetch at
  `:3108`, snapshot installed at `:3110`.

**User-facing text that becomes false**
- `folio-designer/src/App.tsx:3897` — the dismissible warning says "The layout, the page breaks and
  the PDF preview are unaffected — they come from the engine's own copy." Under this story the
  engine has no copy until one is installed, so that clause must change.

**The measurement and its guard**
- `folio-designer/src/release-payload.ts:150-151` `minimumCoreCacheAssets`/`maximumCoreCacheAssets`
  both `30` — a COUNT, unchanged by this story. `:161-162` export them as floor/ceiling.
- `folio-designer/scripts/generate-offline-release.mjs:209-221` computes brotli totals and states
  at `:214` that nothing compares them to a threshold — the byte ceiling goes here and in
  `verify-offline-release.mjs`, read by the same line-anchored reader story 1 established.
- `folio-designer/scripts/offline-release-contract.mjs:66` `ENGINE_WASM_ASSET`, `:132` its core tier
  assignment. `verify-offline-release.mjs:508-526` pins two-build byte equality.

**Carried in from story 4's review**
- `folio-go/page_setup.go:1632` `addCanvasTableLabelLines` swallows `shapeSegments` errors with
  `if serr != nil { continue }`. Harmless while the host passes `fonts.Shipped()` wholesale; this
  story is what makes it reachable, and a swallowed refusal means a silently blank CJK table label.

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/fonts/` -- split into build-tagged variants so the tagged build embeds ten faces and
      the untagged one keeps all eleven -- the embeds are package-scope and cannot be dead-stripped.
- [x] `folio-go/fonts/accounting_test.go`, `fonts_test.go` -- make the NOTICE-to-bytes accounting and
      the per-key pins correct under BOTH builds -- they are total over `Shipped()` in both directions.
- [x] `folio-go/internal/wasm/engine.go` -- hold a `FontSet` on `Engine` and read it at all seven
      sites -- one set, injected like `clock`, merged over the build's `Shipped()`.
- [x] `folio-go/wasm/cmd/engine/main.go` -- register a raw-bytes host entry point that installs a
      named face -- avoids base64 and the 8 MiB envelope bound entirely.
- [x] `folio-designer/scripts/wasm-vcs-stamp.mjs` -- add the tag to `ENGINE_BUILD_FLAGS` -- the one
      declaration point for the designer's engine build.
- [x] `folio-designer/src/engine.worker.ts`, `engine-client.ts`, `engine-protocol.ts` -- carry face
      bytes to the host as transferable raw bytes -- the operation union and its runtime allow-list
      are closed and both need the new member.
- [x] `folio-designer/src/document-face-prefetch.ts` -- return the fetched bytes instead of
      discarding them -- the ArrayBuffer is already in hand.
- [x] `folio-designer/src/App.tsx` (or a module beside it) -- on a `TEXT_FACE_ABSENT` refusal, resolve
      the named face to its asset URL, fetch, install, retry once -- retry exactly once per face, so
      a face the release does not ship surfaces instead of looping.
- [x] `folio-designer/src/App.tsx:3897` -- correct the substitution warning, which now claims the
      engine has its own copy -- it is about to stop being true.
- [x] `folio-go/page_setup.go:1632` -- stop swallowing `shapeSegments` errors on the canvas label
      path -- a swallowed refusal is a silently blank CJK table label.
- [x] `folio-designer/scripts/generate-offline-release.mjs` + `verify-offline-release.mjs` +
      `src/release-payload.ts` -- add a core-tier BYTE ceiling, derived from the build -- nothing
      guards the weight today, so the saving could silently regress.
- [x] MEASURE FIRST, then pin: report the engine wasm's `.br` size and the core tier's total before
      and after, and pin the ceiling to what the build emits -- the 6.1 MiB in SPEC.md is a
      projection from compressing the face standalone, not a measurement.

**Acceptance Criteria:**
- Given a cold cache and a Latin-only session, when the designer loads and is used, then the CJK
  asset is never requested and the core tier is materially smaller than 10.82 MiB — report both.
- Given a first CJK document on a connected machine, when it is opened, then it opens and renders,
  and the CJK asset is fetched exactly once.
- Given the same session and a second CJK document, when it is opened, then no further fetch occurs.
- Given a CJK document with the face already cached, when the network is disconnected, then it opens
  and renders byte-identically.
- Given a CJK document offline with the face never fetched, when it is opened, then the open is
  refused naming the face, and no substitute is laid out.
- Given the untagged build, when the full Go suite runs, then `fonts.Shipped()` returns eleven faces,
  no golden PDF hash moves, and `folio-js` and `folio-dotnet` are unchanged.
- Given a rebuilt release, when verification runs, then a build whose core tier exceeds the pinned
  byte ceiling fails rather than shipping.

## Implementation Notes

**The tag is `nocjkface`,** declared once in `folio-designer/scripts/wasm-vcs-stamp.mjs`'s
`ENGINE_BUILD_FLAGS`. `folio-go/fonts/notosanssc.go` (`!nocjkface`) holds the embed and
`notosanssc_absent.go` (`nocjkface`) does not; `Shipped()` merges `buildTaggedFaces()` over its
ten unconditional keys, so the untagged set is the same eleven it always was.

**MEASURED, BEFORE AND AFTER** (`npm run build`, Brotli sidecar bytes):

| | before | after |
|---|---|---|
| engine wasm `.br` | 8,508,122 (8.11 MiB) | 3,564,400 (3.40 MiB) |
| core tier (29 immutable assets) | 11,335,794 (10.81 MiB) | 6,392,910 (6.10 MiB) |
| whole release (80 assets) | 19,530,533 (18.62 MiB) | 14,584,292 (13.91 MiB) |
| core asset COUNT | 30 | 30 |

The ceiling is pinned at `maximumCoreCacheBytes = 6553600` (6.25 MiB) — the measurement plus
160,690 bytes, about 2.5%. Re-embedding the face costs ~4.9 MiB, thirty times that headroom.

### The line-metrics dependency, and the owner's ruling on it

`chainLineMetrics` (`folio-go/wrap.go`) SKIPS a chain member the caller did not supply, and the
vertical model is a MAXIMUM over the chain's PRESENT faces. So dropping `Noto Sans SC` changed the
line height of any element whose chain names it — including a paragraph of **English** on the
starter's own chain `[Roboto, Noto Sans Thai, Noto Sans SC]`. Measured: 15,985 bytes against
15,986, different digests, for a two-line Latin paragraph. The designer would have disagreed with
the CLI, folio-js and folio-dotnet on ordinary Latin documents, silently.

An earlier attempt made the engine REFUSE such a layout and fetch the face. The owner rejected
that consequence: it would have pulled 4.72 MiB the first time an author typed anything, breaking
this story's own matrix row 1 ("Latin-only session … nothing is fetched").

**The ruling, as built.** The face's glyphs are 10,595,932 bytes; its contribution to the leading
arithmetic is three integers. `folio-go/internal/fontset/declared_metrics.go` declares
`Noto Sans SC`'s `LineMetrics` (Ascent 1160, Descent −288, LineGap 0, on the 1000-unit em), and
`DeclaredLineMetrics` — a `!nocjkface`/`nocjkface` pair — answers for it in the designer's build
and in no other. `chainLineMetrics` consults it when a chain member is not present. Metrics only,
never coverage: a rune that must be DRAWN with the absent face still refuses through
`shapeSegments` with `TEXT_FACE_ABSENT`, which is when the browser fetches it.

Nothing else in the module changes. folio-js, folio-dotnet, the CLI, every golden fixture and any
caller with a genuinely partial FontSet compile the untagged half, where `DeclaredLineMetrics`
answers nothing and the long-standing skip is exactly what it was.

**The numbers are tied, not commented.** `internal/fontset/declared_metrics_test.go` parses the
committed `fonts/notosanssc/NotoSansSC-Regular.ttf` and asserts the three declared values equal
its real `LineMetrics()`, in the UNTAGGED build — the only build that has the face to compare
against. `fonts/accounting_test.go` independently joins that same binary to `fonts.Shipped()` by
bytes, which is what makes "the committed binary" and "the face the module ships" one thing
without `internal/fontset` importing `fonts` (it cannot: that is the module's import cycle).

**The byte-identity proof is a PAIR, over a discovered corpus** (`declared_metrics_parity_test.go`
plus its two build-tagged halves). Twenty-five documents — three authored inline (the measured
Latin paragraph, Thai shaping, a bound table, all on CJK-tailed chains) and twenty-two committed
fixtures covering multi-page flow, section breaks, justified text, Thai mark stacking, embedded
faces, images, barcodes and page counts from 1 to 50:

- **untagged** (`TestTenFaceRenderDivergesWithoutTheDeclaredMetrics`): the three CJK-tailed
  documents render DIFFERENTLY with ten faces than with eleven. That is the defect, measured, and
  it is also the guard that no other consumer's output moved.
- **tagged** (`TestTenFaceRenderIsByteIdenticalUnderTheTag`): all twenty-five render BYTE-IDENTICALLY.

Line metrics were the only difference a missing chain member made: nothing else diverged anywhere
in that corpus. Six fixtures (`multi-script-fallback`, `shaped-text`, `statement-1/5/20/50` — the
last four because their sample DATA carries a Han rune) refuse in a ten-face engine in both
builds, coded `TEXT_FACE_ABSENT`; that is CAP-7 working and is exactly when the face is fetched.

### The retry path

**The retry lives in `EngineClient`, not at the call sites.** `onAbsentFace` installs one recovery
for the life of the client (from `startup-sequence.ts`, the seam after the ready handshake and
before any document); a `TEXT_FACE_ABSENT` refusal is answered by the recovery and the request is
re-sent AT MOST ONCE, never for `install-face` itself. That covers all seven engine call sites
without a wrapper at each. The retained payload is copied per send, because a transferred
ArrayBuffer is detached.

**`AbsentFaceInstaller` keeps one promise per face**, not a flag: `Apply` fires about every 200 ms
while typing, so a second commit can refuse while the first commit's ~10 MiB fetch is still in the
air, and it must WAIT for that fetch rather than be told there is nothing doing. The recovery asks
the network even when `navigator.onLine === false`, because the service worker answers a held
deferred asset from cache — the matrix's "offline, face already cached" row.

**Face names are read out of the refusal by quoting, not by substring.** `%q` marks the ABSENT
faces in `render.go`'s message while the bracketed chain beside them lists the present ones too;
`wrap.go`'s no-present-member message quotes nothing, so its bracket is scanned longest-candidate
first. Candidates are `canvasFaceAssets` keys, so no name the release cannot fetch is returnable.

### Running the tagged suite

Most of `package folio8`'s tests are ABOUT the eleven-face shipped set and correctly fail under a
tag whose purpose is that `Shipped()` returns ten. The tag is therefore run over the packages that
have something to say about it:

```
go test -tags nocjkface ./fonts/ ./internal/fontset/ ./internal/wasm/
go test -tags nocjkface ./ -run 'TestTenFaceRender|TestCJKTextStillRefuses'
```

## Spec Change Log

## Review Triage Log

| # | Finding | Verdict | Evidence | Route |
|---|---------|---------|----------|-------|
| 1 | No CI step runs the `nocjkface` tag, so the entire tagged half of CAP-6 is asserted by test files no pipeline ever compiles | **high** | Verified by grep across `.github/workflows/`, `package.json`, `Makefile`: no match for `nocjkface`. `TestTenFaceRenderIsByteIdenticalUnderTheTag` and `TestShippedOmitsTheCJKFaceUnderTheTag` carry `//go:build nocjkface`. The property the whole story rests on can be reverted with every pipeline green | patch |
| 2 | Matrix row 5 ("CJK document, offline, face already cached → opens from cache") has no covering test | **high** | Verified: `absent-face-recovery.test.ts` never touches `navigator.onLine`, so every case runs jsdom-online. Dropping the `true` at `absent-face-recovery.ts:171` leaves all suites green while the row regresses to a refusal. A matrix row with no test that ran is an audit failure | patch |
| 3 | `js.CopyBytesToGo` panics on a non-Uint8Array object with `byteLength`, killing the wasm instance instead of refusing | **high** | `installFace` checks only `js.TypeObject`. An `ArrayBuffer` satisfies that and carries `byteLength`. A panic in js/wasm terminates the instance, contradicting the contract stated three lines above that it answers with the same `response` JSON. The copy's return count is also discarded, so a short copy installs a zero-padded face | patch |
| 4 | A failed offline fetch poisons the face for the whole session — reconnecting does not help | **high** | `AbsentFaceInstaller.#attempt` memoises the promise permanently and a test pins that it "spends the attempt anyway". Author opens a CJK document offline, reconnects, and every later refusal replays the stale `false`; only a page reload cures it, and nothing says so. Contradicts the owner's founding answer, "fetch when the network is up" | patch |
| 5 | The red proof for the byte ceiling rewrites tracked source (`release-payload.ts`) and relies on a `finally` to restore it | **medium** | It is the only proof in `runRedProofs` mutating a file outside `dist/`. A SIGINT or harness crash leaves `maximumCoreCacheBytes = 1000000` in a committed file, and it races any `vite dev`/`vitest --watch` reading it. `declaredCoreCacheByteCeiling` already accepts an injected `source` | patch |
| 6 | The generator's copy of the ceiling guard is never executed against an over-ceiling release | **medium** | Its only check is `expect(generator).toContain('declaredCoreCacheByteCeiling()')` — source text, never execution. `redProof('core-tier-bytes-over-ceiling')` calls only `verifyOfflineRelease`. Inverting the generator's comparison leaves everything green, and `build:offline` alone — the reason the guard was duplicated — then emits an over-ceiling release | patch |
| 7 | `maxInstalledFaceNameBytes = 256` states it "matches the browser's own bound `MAX_CANVAS_PROPERTY_STRING`", which is 512 | **medium** | Verified at `engine-protocol.ts:92`. A 257–512-char name passes the browser guard and is refused by the host. Go counts bytes and TS counts UTF-16 units, so the two are not even the same quantity | patch |
| 8 | `InstallFace` stores any non-empty bytes without parsing them, so a truncated download becomes a permanently installed bad face | **medium** | `installed_face_test.go` covers empty bytes and empty name but not invalid bytes — the case `engine-client.ts`'s own comment anticipates. The later failure lands deeper than `TEXT_FACE_ABSENT`, where the recovery cannot read it | patch |
| 9 | `FACE_FETCH_TIMEOUT_MS = 20_000` is an engine-step budget spent on a ~10 MiB transfer, untied to the constant it cites | **medium** | Below roughly 4 Mbps the fetch aborts, and with finding 4 that abort is final for the session. `App.tsx`'s `ENGINE_FILE_STEP_TIMEOUT_MS` is a separate copy with no test holding them together | patch |
| 10 | `fetchDeferredFaces` now retains every face body concurrently where each was previously read and dropped | **medium** | It accumulates `arrayBuffer()` results into a Map across `Promise.all`; the void wrapper discards the map afterwards. Peak heap on the document-open path rises from the largest body to the sum of all of them | patch |
| 11 | AC "the CJK asset is fetched exactly once" is not asserted; the recovery fetches the URL and the canvas prefetch then requests it again | **medium** | Only the service-worker/HTTP cache collapses the two, and no test counts requests. A cache regression turns one ~10 MiB download into two with every test green | patch |
| 12 | `App.tsx`'s new comment claims the engine "already holds every face the document's text needs" by the time the prefetch runs | **medium** | False for Han runes carried in sample DATA rather than authored text — which is exactly the `browser-native-roundtrip` fixture. `CanvasWithTextPaint` projects only authored text | patch |
| 13 | `Engine.HasFace` and `AbsentFaceInstaller.installed` have no production reader | **low** | `HasFace` says so in its own doc; `startup-sequence.ts` constructs the installer inline and keeps no handle, so `installed` is unreachable in the app. Direct deletion | patch |
| 14 | `case 'install-face': return none` duplicates the `default` arm, and the core-tier predicate `tier === 'core' && immutable` is spelled twice in a file whose comment says the derivation lives in one place | **low** | Both are direct corrections; the second contradicts its own stated purpose that the build and verifier "cannot compute it two ways" | patch |
| 15 | The wasm host's `installFace` bound checks (name length, size bounds) have no executing test | **medium** | Real: `main_test.go` calls `dispatch` directly and nothing calls `main()`, so the `js.FuncOf` closures are unreachable from any test. Closing it means extracting the closure body — the same untestable layer as the pre-existing `handle` wrapper | defer |
| 16 | A document needing TWO absent faces can never recover, because `EngineClient` retries at most once per request | **low** | Real but unreachable today: only one face is deferred. Nothing in the code states that precondition, and `canvasFaceAssets` is the candidate set for every face in the release | defer |
| 17 | The e2e comment justifies "one `load`" by the line metrics rather than by the absence of authored Han runes | **low** | A non-sequitur that sends a reader to the wrong place when they add CJK and see two loads. Direct correction to a comment | patch |

## Design Notes

**Why the refusal is the discovery mechanism.** The designer cannot ask the engine which faces a
document needs, because the engine answers that by projecting the canvas, which is what needs the
face. `paintedCanvasFaces` (`document-face-prefetch.ts:73`) reads the answer out of a projection
that has already succeeded. Story 4's refusal breaks the cycle by naming the face in the failure
itself:

```
load(document) -> TEXT_FACE_ABSENT, Diagnostic.Message names "Noto Sans SC"
  -> canvasFaceAssets.get("Noto Sans SC") -> content-addressed URL
  -> fetch (service worker caches it) -> install raw bytes -> load(document) again
```

The alternative — reading the document's `fontChains` — was rejected on evidence: the starter
template's single chain is `[Roboto…, Noto Sans Thai…, "Noto Sans SC"]` with empty bands, so every
first load would fetch 4.72 MiB for a document containing no CJK text whatsoever.

**Why the engine holds the set rather than receiving it per call.** `Apply` fires roughly every
200 ms while typing (`PROSE_COMMIT_DEBOUNCE_MS`). Sending 10.11 MiB with each command is not an
option, and the request envelope's 8 MiB bound forbids it anyway.

## Verification

**Commands:**
- `cd folio-go && go build ./... && go vet ./... && go test ./...` -- expected: clean; eleven faces
  in the untagged `Shipped()`; no golden hash moved
- `cd folio-go && go test -tags nocjkface ./fonts/ ./internal/fontset/ ./internal/wasm/` and
  `go test -tags nocjkface ./ -run 'TestTenFaceRender|TestCJKTextStillRefuses'` -- expected: clean
  under the tagged build too. `./wasm/...` has no native test binary (js/wasm only); it is covered
  by `GOOS=js GOARCH=wasm go vet -tags nocjkface ./wasm/...`
- `cd folio-designer && npm run build:wasm && npm run build` -- expected: clean, verify:offline green
- `cd folio-designer && npm test` -- expected: all pass
- `cd folio-designer && npx playwright test` -- expected: all pass
- `cd folio-js && npm test` and `cd folio-dotnet && dotnet test` -- expected: unchanged
- Report the engine wasm `.br` bytes and the core-tier total, before and after.
