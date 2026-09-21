---
title: 'Google Tag Manager in the Designer'
type: 'feature'
created: '2026-09-21'
status: 'done'
route: 'dispatch'
baseline_commit: 'd099c20252b2b75cebe54edf2845ed89df1316fa'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Designer reports nothing about how it is used, and its architecture
records that as a decision — `ARCHITECTURE-SPINE.md:616` states *"No backend, no database,
no accounts, no telemetry"*. The owner wants usage measurement, which means reversing that
decision rather than quietly contradicting it.

**Approach:** Load the Google Tag Manager container `GTM-NQZRC9V4` from `index.html`,
gated on a build-time environment variable so it is inert unless deployed with one.
Push four bounded, non-identifying action events to `dataLayer`. Then amend every place
in the repository that asserts the no-telemetry posture, so the repo stops claiming
something that is no longer true.

**Owner decisions (settled before planning, do not re-open):**
- D-GA.1 — **Reverse** the no-telemetry decision AND amend the architecture docs to record
  the reversal with rationale. NFR8's substantive guarantee — *templates and data never
  leave the user's machine* — is PRESERVED. GTM may report usage; it may never carry
  document or data content.
- D-GA.2 — Scope is pageview plus custom events for four actions: open template, export,
  font import, preview.
- D-GA.3 — The container id comes from a Vite env var. Unset means **no script load, no
  `dataLayer`, no events**, so dev, vitest and Playwright never transmit.
- D-GA.4 — The tag is Google Tag Manager (container `GTM-NQZRC9V4`), not GA4 `gtag.js`
  directly. Events are `dataLayer` pushes; tag configuration lives in the GTM console.
- D-GA.5 — **The visible standing promise is amended, not abandoned.** Preview's
  `no network · nothing left this machine` becomes a statement that is still true once GTM
  loads — the guarantee that survives is about the user's *data*, not about the page making
  zero requests. The three assertions that pin the old string are updated with it. This
  reverses the half of the promise D-GA.1 made false; it does not delete the promise.
- D-GA.6 — **The `<noscript>` iframe is omitted.** The Designer renders nothing without
  JavaScript, so a no-JS visitor is never a real user and the iframe would measure nothing.
  Omitting it also avoids a second remote subresource that would need its own env gate.
- D-GA.7 — **Env-gated runtime injection, not GTM's literal static paste.** GTM's
  copy-paste instructions would fire the container on every `npm run dev` and every
  Playwright e2e run. A module injects the identical snippet only when the env var is set,
  which is what D-GA.3 requires. GTM's instructions are generic boilerplate and are
  deliberately not followed to the letter here.
- D-GA.8 — Spec kept whole at ~3,800 tokens (workflow ceiling is 1,600) by owner choice, so
  the doc amendment stays coupled to the code that makes it necessary. Implementation should
  work section by section rather than holding the whole spec at once.

## Boundaries & Constraints

**Always:**
- The env var is the sole switch. With it unset, the built page must contain no
  googletagmanager URL, define no `dataLayer`, and register no event calls.
- Event payloads carry a fixed vocabulary of action names only. No file name, template
  name, font family name, path, parameter value, or document content may ever become an
  event parameter.
- The app must behave identically when `googletagmanager.com` is unreachable. GTM is
  loaded `async` and every `dataLayer` push must be safe when the container never arrives.
- `index.html` must keep a literal `</head>`, and must not split the contiguous
  `<meta folio8-page-release>` / `<meta folio8-app-version>` / `<script id="folio8-release-bootstrap">`
  run that `generate-offline-release.mjs` injects and `verify-offline-release.mjs:232-235`
  re-reads.
- Any inline script added to `index.html` must NOT be `type="module"` — Vite extracts those
  into `/assets/*.js`, which lands in the `core` tier and breaks the exact 30/30 pin at
  `verify-offline-release.mjs:200-202`.

**Never:**
- No GTM request may enter the service-worker precache or the offline release manifest.
- Do not add a `fetch(` to the service worker: `verify-offline-release.mjs:283` permits
  exactly one and fails on any second.
- Do not weaken, delete, or narrow the font-host scanners, the offline verifier, or the
  `no third-party host` font-dropdown assertion at `App.test.tsx:4357`.
- No consent banner, no cookie UI, no GA4 `gtag.js` alongside GTM, no server-side tagging.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Deployed with container id | env var = `GTM-NQZRC9V4` | GTM snippet injected; `dataLayer` exists; pageview fires | N/A |
| Dev / test / unconfigured | env var unset or empty | No script, no `dataLayer`, no network call | N/A |
| Tracked action | user exports a PDF | one `dataLayer` push with a fixed action name, no document data | N/A |
| Offline or blocked | container unreachable / ad-blocked | App fully functional; pushes queue into the array harmlessly | Never throw, never surface UI |
| Malformed id | env var set to junk | Treated as unset (fail closed) rather than injecting a bad tag | Skip silently |

</frozen-after-approval>

## Code Map

- `folio-designer/index.html` -- the page. Head currently holds charset, viewport, title;
  `build:offline` injects the release bootstrap trio before `</head>`. GTM goes here.
- `folio-designer/src/analytics.ts` -- NEW. Env read, container injection, `trackEvent`.
  Style exemplars for a commented ~100-line module: `src/startup-sequence.ts` (36 lines,
  ALL-CAPS assertions, `⚠` hazards, spec refs) and `src/offline-lifecycle.ts` (157 lines).
- `folio-designer/src/analytics.test.ts` -- NEW. Convention is the test beside its module.
- `folio-designer/src/main.tsx:17` -- `createRoot`; `:31` `render()`; `:40` `startObservation`;
  `:64` the `import.meta.env.DEV` bypass. Initialise analytics here, before first render.
- `folio-designer/src/App.tsx:4362` `openExample` -- "open template" hook (both the startup
  dialog and New… funnel through `chooseStartup:4292` into this).
- `folio-designer/src/App.tsx:4520` `exportPreviewPdf` -- "export" hook; write lands at `:4540`.
- `folio-designer/src/App.tsx:3763` `requestFontImport` -- "font import" hook; fire after
  `importFontFiles` succeeds at `:3788`.
- `folio-designer/src/App.tsx:1601` `enterPreview` -- "preview" hook. Use this mode switch,
  NOT `runPreview:1402`, which is debounced and would fire per re-render.
- `folio-designer/vite.config.ts` -- no `define`, no `envPrefix`, no `loadEnv` today; test
  block sets `setupFiles: ['./src/test/setup.ts']`. `VITE_GA_*` needs no config: Vite exposes
  `VITE_`-prefixed vars on `import.meta.env` by default.
- `folio-designer/src/test/setup.ts` -- 5 lines, jest-dom + `afterEach(cleanup)`.
  `vi.stubGlobal` precedent lives in `engine-worker-boundary.test.ts:58`.
- **The standing promise (D-GA.5), four sites that must move together:**
  `src/App.tsx:5291` renders `<span data-testid="local-only-assurance">` — the string itself;
  `src/App.tsx:5285-5290` is the comment block calling it *"the product's standing promise"*
  and fencing the Preview bar's 228 px assurance on design grounds — the prose there must be
  amended too, not just the string; `src/App.test.tsx:10219` (also `:10281`, `:12860`) and
  `e2e/preview-navigation.spec.ts:225` assert it. `epics.md:4658` and `:4812` quote the old
  wording as *"the product's central promise"* (UX-DR23) and are part of the doc sweep.
- **Do not change:** `scripts/generate-offline-release.mjs`, `scripts/verify-offline-release.mjs`,
  `scripts/offline-service-worker-template.mjs`, `scripts/forbidden-font-hosts.mjs`,
  `scripts/host-font-access.mjs`, `src/release-payload.ts`. Investigation confirmed a remote
  `<script src>` plus a non-module inline script passes all of them untouched.

**Documents asserting the posture, all of which must be amended (D-GA.1):**
- `ARCHITECTURE-SPINE.md:616` -- *"No backend, no database, no accounts, no telemetry."*
- `ARCHITECTURE-SPINE.md:414` (inside AD-19) -- *"Nothing in the designer performs a network
  request at render or preview time."*
- `prd.md:470-474` -- NFR8 Privacy posture.
- `epics.md:152` -- NFR8 restatement.
- `README.md:34` -- *"No server, no account, no upload."*
- `_bmad-output/planning-artifacts/ux-designs/**/EXPERIENCE.md:34-36` -- *"no network is
  required and none is used."*
- AD entries live in `## Invariants & Rules` (`ARCHITECTURE-SPINE.md:54`), format
  `### AD-N — <title>` with three bullets: **Binds** / **Prevents** / **Rule**. Highest in
  use is AD-26 (`:521`), so a new one is **AD-27**.
- Amendment convention: strike the original, keep it verbatim, append a bolded
  **AMENDED <date> by OWNER DECISION** block naming what still holds and what changed.
  Canonical example: `_bmad-output/specs/spec-fonts/SPEC.md:135-152`.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/src/analytics.ts` -- NEW module: read `import.meta.env.VITE_GA_CONTAINER_ID`,
      validate it against `/^GTM-[A-Z0-9]+$/` (fail closed), inject the GTM snippet once, and
      export a `trackEvent` taking a value from a closed union of the four action names --
      one switch, one vocabulary, no free-form strings reachable from call sites.
- [x] `folio-designer/index.html` -- add only what must be static; keep `</head>` literal and
      the injected bootstrap trio contiguous; no `type="module"` inline script.
- [x] `folio-designer/src/main.tsx` -- initialise analytics before the first `render()`.
- [x] `folio-designer/src/App.tsx` -- four `trackEvent` calls at the mapped hook points, each
      on the success path only.
- [x] `folio-designer/src/App.tsx:5291` + `:5285-5290` -- amend the Preview assurance string to
      one that stays true with GTM loaded (D-GA.5), and amend the comment block above it so its
      rationale matches the new wording. Keep the `data-testid="local-only-assurance"` hook and
      respect the 228 px width budget the Preview bar is priced against.
- [x] `folio-designer/src/App.test.tsx:10219,10281,12860` and
      `folio-designer/e2e/preview-navigation.spec.ts:225` -- update the three assertions to the
      amended string. Update them to assert the NEW promise; never to stop asserting one.
- [x] `folio-designer/src/analytics.test.ts` -- cover every I/O Matrix row, especially: unset
      env injects nothing and defines no `dataLayer`; a malformed id is treated as unset; a
      push carries no argument derived from a file, template or font name.
- [x] `ARCHITECTURE-SPINE.md` -- amend `:616` and AD-19's `:414` sentence in the repo's
      strike-and-append style; add **AD-27** recording the reversal, what it costs, and the
      preserved NFR8 guarantee.
- [x] `prd.md`, `epics.md`, `README.md`, `EXPERIENCE.md` -- narrow each claim to what stays
      true: the Designer contacts one third party for usage measurement; templates and data
      still never leave the machine. `epics.md:4658` and `:4812` quote the OLD Preview wording
      as the product's central promise (UX-DR23) and must move to the amended string with it.
- [x] `_bmad-output/implementation-artifacts/ga-decision-log.md` -- NEW. Decisions D-GA.1
      through D-GA.8 in the established `| ID | What it settles |` table format, following
      `epic-16-decision-log.md`.

**Acceptance Criteria:**
- Given no `VITE_GA_CONTAINER_ID`, when the app is built and loaded, then the output contains
  no `googletagmanager` string, `window.dataLayer` is undefined, and no request is attempted.
- Given a valid container id, when the app loads, then exactly one GTM script is injected and
  a pageview reaches `dataLayer`.
- Given a valid container id and an unreachable `googletagmanager.com`, when a user opens a
  template, exports, imports a font and enters preview, then every action completes normally
  and no error surfaces in the UI.
- Given any tracked action, when its event is pushed, then the payload's values are drawn
  solely from the fixed action vocabulary — asserted by a test, not by inspection.
- Given Preview mode with GTM loaded, when the status bar renders, then the assurance it
  displays is true of the page as it actually behaves, and a test still asserts that wording.
- Given the full build, when `npm run build` runs, then `scan:font-hosts`, `scan:host-fonts`,
  `build:offline` and `verify:offline` all pass unchanged.
- Given the repository after this change, when a reader greps for the no-telemetry claim,
  then every occurrence either reflects the new reality or is struck with its amendment.

## Implementation Notes

**Delivered 2026-09-21.** All tasks complete; full suite, typecheck, lint, e2e-compile, build,
`verify:offline` and `verify:offline:red` green.

**A defect was found in review and fixed before this reached review-proper. It is recorded
here because the way it hid is more instructive than the bug.**

The first implementation added an explanatory comment to `index.html` whose closing line read
``⚠ Keep `</head>` on its own line and literal``. That mention was the FIRST closing head tag in
the file, and both injections that target it — Vite's bundle/stylesheet, and
`generate-offline-release.mjs:168`, a plain `String.replace` — landed inside the comment. The
comment's `-->` did not appear until after them, so the shipped `dist/index.html` contained **no
executable script outside a comment at all**: an empty `<div id="root">` and a blank designer.

⚠ **Every guard stayed green.** `npm run build` succeeded; `build:offline` found a tag to replace,
so its own `'production index has no head for release bootstrap'` throw never fired;
`verify:offline` matched the bootstrap by regex and never asked whether the match was commented
out; the asset set was unchanged, so the exact-set check and the 30/30 core pin both passed. The
only symptom was a blank page in a browser, and nothing in the pipeline opens one. The
implementation's own report — *"npm run build — all pass"* — was true and worthless. This is the
case for judging a change by its diff and its artifact rather than by its author's summary.

Fixed by rewriting the comment so it never spells the tag, and guarded by a new test:

- `folio-designer/src/index-head-injection.test.ts` — asserts `index.html` spells the closing head
  tag exactly once and opens no comment above the injection site that it does not close. Includes
  a **positive control** that runs both checks against the exact broken shape and requires them to
  fail, because both assertions pass trivially on a page with no comments and would otherwise have
  been green on the day the bug shipped.

**Verified in both directions, not one.** Unconfigured build: `grep -r googletagmanager dist/`
returns nothing, and the built page carries the module script, stylesheet, release metas and
bootstrap *outside* any comment (checked by stripping comments first, which is how the bug was
caught). Configured build (`VITE_GA_CONTAINER_ID=GTM-NQZRC9V4`): the container id and the gtm.js
URL appear in `dist/assets/index-*.js`, and `dist/index.html` still carries no static tag. Vite's
literal substitution plus esbuild DCE makes the unconfigured absence structural rather than merely
unobserved.

**Known and accepted:**
- Opening a template emits `open_template` **and** `enter_preview`, because `openExample` genuinely
  enters preview. `enter_preview` ≥ `open_template` by construction — read the GTM numbers knowing
  that, or split the trigger in the console.
- The e2e assertion is compile-verified only; `npm run test:e2e` (Playwright) was not run.
- The live-container DevTools check ("exactly one gtm.js request") is unperformed — no browser here.
- `verify:offline`'s core-weight *approach warning* (6,430,981 of 6,553,600 Brotli bytes) is
  pre-existing and unrelated; this change adds no dist asset.

**Review round (2026-09-21).** Three layers ran; 20 findings triaged in the Review Triage Log —
13 patched, 5 deferred to `deferred-work.md`, 2 rejected. The two that mattered most:

- `Dockerfile` declared no `ARG` for `VITE_GA_CONTAINER_ID`, so a Railway service variable could
  never reach `npx vite build`. **The feature would have shipped inert** — reversed architecture
  decision, five amended documents, a narrowed user promise, and zero measurement, with every gate
  green. Fixed with a build ARG/ENV and an operator section in `RELEASING.md`.
- `font_import` fired before the acknowledgement the user can decline, and `enter_preview` fired on
  every PREVIEW press rather than on a mode switch. Both counted things that did not happen.

Also patched: a throw in `initAnalytics()` could abort `main.tsx` before `startObservation` (a
blank app); a pre-existing non-array `window.dataLayer` would throw on push; and the four call
sites had no tests at all — `App.test.tsx` and `App.font-import.test.tsx` now carry 10 cases
behind a `vi.mock` spy, asserting success-path-only and that each call carries exactly one
argument while the file/example/family name is visibly on screen in the same test.
`src/single-measurement-vendor.test.ts` now enforces AD-27's "one vendor, from `src/analytics.ts`
and nowhere else" with two positive controls.

**Independently re-verified after the patch round, not taken on report:** full suite 2,315 pass
(97 files), `tsc -b` clean, `oxlint` clean, e2e compile clean, `npm run build` green through
`verify:offline`. Artifact checked directly — script and bootstrap outside any comment, no stray
body text, no `googletagmanager` anywhere in an unconfigured `dist/`; configured build contains
the id and the gtm.js URL while `dist/index.html` stays static-tag-free. One call site was
mutation-tested here (removing `trackEvent('open_template')` fails the suite; `App.tsx` restored
byte-identical), so the new tests are not vacuous.

**Out of scope, left for the owner:** the implementation also made commit `c3c7d06`
(`ci(deploy): retry the Railway CLI fetch`) touching `.github/workflows/deploy.yml`. It is
unrelated to this spec, was not authorised by it, and is unpushed. Not reverted here — flagged.

**The env gate is a build-time constant, not a runtime branch.** `analytics.ts` reads
`import.meta.env.VITE_GA_CONTAINER_ID` **once at module scope** into a `const`. Vite substitutes a
literal there, so an unconfigured build becomes `const CONTAINER_ID = undefined` and esbuild folds
every branch below it away. This is what makes the acceptance criterion *structurally* true rather
than merely observed: verified by building without the var and grepping `dist/` for
`googletagmanager` — **no matches** — and by building with `VITE_GA_CONTAINER_ID=GTM-NQZRC9V4`,
where the URL **does** appear in `dist/assets/index-*.js`. Both directions were checked, because
only the pair proves the gate rather than an absent feature.

**`trackEvent` gates on an `active` latch set by `initAnalytics`, not on `CONTAINER_ID` alone.**
A push that ran before initialisation would create `window.dataLayer` as a side effect, and the
criterion is that an unconfigured page has **no** `dataLayer` — not an empty one.

**Hook points, all on the success path only:**
- `App.tsx:4399` `openExample` — after both files are installed, before `enterPreview()`.
- `App.tsx:4560` `exportPreviewPdf` — after `writeSave` returned, so a cancelled picker is not an
  export.
- `App.tsx:3800` `requestFontImport` — after `importFontFiles` admitted at least one family,
  before the acknowledgement is raised.
- `App.tsx:1605` `enterPreview` — the mode switch, not `runPreview`.

Opening a template therefore emits **two** events, `open_template` then `enter_preview`, because
opening an example genuinely does enter Preview. They are distinct actions and are counted as such.

**`index.html` gained no markup**, only a comment recording why: a `preconnect` or `preload` would
put a third-party host in `dist/index.html` unconditionally, which is the exact thing D-GA.3
forbids. `</head>` stays literal and the bootstrap trio is untouched.

**The amended promise is `local render · your data stays here`** — 35 characters against the old
36, so it is still priced inside the Preview bar's 228 px budget. It is asserted in `App.test.tsx`
(text and structure) and in `e2e/preview-navigation.spec.ts` (text **and** the existing viewport
fence), so the promise is narrowed, never dropped.


## Spec Change Log

- 2026-09-21 — implemented. No section of the frozen intent block was changed; no Open Question was
  raised. The only judgement the spec left to implementation was the exact amended wording of the
  standing promise, which D-GA.5 delegated by describing the property it must have rather than the
  words.


## Review Triage Log

Three layers ran (blind-hunter, edge-case-hunter, verification-gap) against the staged diff.

| # | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|
| 1 | `index.html` comment spelled the closing head tag; both injections landed inside the comment and the page shipped with no executable script | high | Reproduced in `dist/index.html` before the fix: stripping comments left no module script. Fixed and guarded before review. | patch (done) |
| 2 | The first fix's comment spelled a literal terminator, ending the comment early and leaking ~340 chars of prose into `<body>` | high | Confirmed by parsing the built page: first `-->` at offset 1500, prose after it. Caught by edge-case-hunter, not by my own guard, which counted `<!--`/`-->` pairs rather than parsing. Both fixed; guard now parses. | patch (done) |
| 3 | `Dockerfile` declares no ARG/ENV for `VITE_GA_CONTAINER_ID`, so a Railway variable cannot reach `npx vite build` (line 33) — the feature ships inert | high | Verified: no `ARG` in Dockerfile; `deploy.yml` passes only `RAILWAY_TOKEN`. Spec's Code Map omitted deployment; the intent did not. | patch |
| 4 | A throw in `initAnalytics()` aborts `main.tsx` before `startObservation`, turning a measurement fault into a blank app | high | `initAnalytics()` is unguarded at module scope in `main.tsx`. | patch |
| 5 | `window.dataLayer ?? []` adopts a pre-existing non-array, then `.push` throws | medium | `analytics.ts` uses `??`, not `Array.isArray`. Compounds #4. | patch |
| 6 | `font_import` fires before the acknowledgement the user can decline | medium | Verified at `App.tsx:3800`: the call precedes `setFontImportRequest(outcome)`, and step two is a separate handler. Declined imports counted. | patch |
| 7 | `enter_preview` fires on the PREVIEW button even when already in preview | medium | Verified: `enterPreview` is the onClick at `App.tsx:5027` and has no mode guard. Repeat clicks inflate the count. | patch |
| 8 | Nothing tests the four call sites; deleting any one leaves the suite green | medium | `grep` for `trackEvent` finds no match in any test but `analytics.test.ts`, which never renders `App`. Under Vitest `active === false`, so a `dataLayer` assertion could not observe a push at all. | patch |
| 9 | AD-27's "from `src/analytics.ts` and nowhere else" is enforced by nothing; a preconnect in `index.html` passes every guard | medium | Font-host scanners match a fixed font-host list only; `googletagmanager` appears nowhere in `scripts/`. | patch |
| 10 | Comment's width arithmetic wrong: old string is 38 chars, not 36 | low | Measured: old 38, new 35. Conclusion (fits the 228 px budget) unaffected. | patch |
| 11 | "byte-equivalent"/"identical snippet" to GTM's is false | low | Differs in `Date.now()`, the missing `&l=` parameter, and `appendChild` vs `insertBefore`. Behaviour fine; the claim is not. | patch |
| 12 | `trackEvent` docblock "safe to call at any time" reads as queued, but it discards before init | low | The `active` latch returns early. Design is right; the prose misleads. | patch |
| 13 | `PERMITTED_VALUES` carries `'gtm.js'`, made unreachable by `.slice(1)` | low | Dead entry. | patch |
| 14 | Opening a template emits both `open_template` and `enter_preview` | low | Real but intended: `openExample` genuinely enters preview. Documented in Implementation Notes and AD-27 rather than suppressed. | rejected (documented) |
| 15 | A configured offline release "ships a build that contacts a third party" | false | AD-27 forbids GTM entering the precache, not the configured build contacting GTM — that is the feature. No GTM asset enters the manifest; the SW gates every branch on same-origin. | rejected |
| 16 | The GTM container's own contents are outside every bound this change establishes | medium (unverified) | Real governance hole: the closed union constrains only what this repo pushes; a console-side Custom HTML tag could read the DOM with no repo-side evidence. Not a code defect — needs an owner policy. | defer |
| 17 | Consent/cookie/regulatory reasoning absent from an otherwise meticulous decision record | medium (unverified) | GTM/GA4 set cookies and transmit IP addresses; D-GA.6 forbids a consent banner without stating why. Owner's call, with legal exposure. | defer |
| 18 | The product's own UI discloses nothing about the third party, while five documents now do | medium | Real asymmetry: users of the hosted build are told less than any internal artifact. Needs an owner decision on user-facing disclosure. | defer |
| 19 | The dist-level injection hole remains: only the source page is guarded | low | The new guard disclaims this explicitly; a Vite plugin emitting a second head would still pass. Fix would mean editing `verify-offline-release.mjs`, which the spec forbids. | defer |
| 20 | Unrelated unauthorised commit `c3c7d06` rides along | n/a | Flagged to the owner; outside this diff. | defer |


## Design Notes

The offline pipeline tolerates this change because of three specific facts established
during investigation, each worth keeping in view while implementing:

1. `generate-offline-release.mjs:15 assetsFromDist()` builds the precache by walking `dist/`,
   never by parsing `index.html`. A remote script emits no file, so the exact-set check at
   `verify-offline-release.mjs:204` sees nothing new.
2. `offline-release-contract.mjs:releaseIdentity()` filters `/index.html` out of the release
   identity, so editing the page does not churn `releaseId`.
3. Every branch of the service worker's fetch handler gates on
   `url.origin === self.location.origin`, so a cross-origin GTM request is never intercepted
   — it passes through to the network and simply fails, harmlessly, when offline.

The event vocabulary is a closed TypeScript union rather than a `string` parameter on purpose:
it makes the "no document data in events" rule a compile-time property at every call site,
instead of a convention that the next contributor has to know about.

## Verification

**Commands:**
- `cd folio-designer && npm run test` -- expected: all pass, including the new `analytics.test.ts`
- `cd folio-designer && npm run typecheck` -- expected: clean
- `cd folio-designer && npm run lint` -- expected: clean
- `cd folio-designer && npm run build` -- expected: scan:font-hosts, scan:host-fonts,
  build:offline and verify:offline all green
- `cd folio-designer && npm run verify:offline:red` -- expected: the negative controls still fail
  as designed, proving the verifier is alive
- `grep -rn "googletagmanager" folio-designer/dist/` (after an unconfigured build) -- expected:
  no matches

**Manual checks:**
- Build with the container id set, load the page with DevTools Network open: exactly one
  `gtm.js` request, and the four actions each produce one `dataLayer` entry with no
  document-derived values.
