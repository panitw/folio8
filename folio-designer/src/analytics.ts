/**
 * USAGE MEASUREMENT — Google Tag Manager, env-gated (spec-google-analytics).
 *
 * This module is the ONE place in the designer that contacts a third party.
 * It exists because of an OWNER DECISION (D-GA.1) that REVERSES the
 * no-telemetry posture recorded at `ARCHITECTURE-SPINE.md` AD-27. Read that
 * entry before changing anything here; the reversal is bounded, and the bound
 * is this file.
 *
 * ⚠ THE GUARANTEE THAT SURVIVES IS ABOUT THE USER'S DATA, NOT ABOUT THE PAGE
 * MAKING ZERO REQUESTS. NFR8's substantive promise — templates and sample data
 * never leave the machine — is UNCHANGED and is enforced here by construction:
 * `trackEvent` takes a value from the closed `AnalyticsAction` union, never a
 * `string`, so no file name, template name, font family, path, parameter value
 * or document byte can reach a `dataLayer` push from any call site. That rule
 * is a COMPILE-TIME property, deliberately, rather than a convention the next
 * contributor has to be told about.
 *
 * ⚠ THE ENV VAR IS THE SOLE SWITCH (D-GA.3). Unset, empty, or malformed means
 * no script, no `window.dataLayer`, and no push — so `npm run dev`, Vitest and
 * Playwright transmit nothing, ever, without anyone having to remember to
 * disable something. Fail CLOSED: a junk id is treated exactly as unset.
 *
 * ⚠ THE SNIPPET IS INJECTED AT RUNTIME, NOT PASTED INTO `index.html` (D-GA.7).
 * GTM's own copy-paste instructions are generic boilerplate and are
 * deliberately not followed to the letter: a static paste fires the container
 * on every dev server and every e2e run, which is precisely what D-GA.3
 * forbids. The injected snippet is FUNCTIONALLY equivalent to GTM's, minus the
 * `<noscript>` iframe (D-GA.6 — the designer renders nothing without
 * JavaScript, so a no-JS visitor is never a real user and the iframe would
 * measure nothing, at the cost of a second remote subresource to gate).
 *
 * ⚠ FUNCTIONALLY, NOT LITERALLY, EQUIVALENT — do not rely on a byte match. It
 * differs from GTM's paste in three ways that are all deliberate and none of
 * which change what the container receives: `Date.now()` rather than
 * `new Date().getTime()`; no `&l=` parameter, because the queue is GTM's
 * default `dataLayer` name; and `head.appendChild` rather than inserting before
 * the first existing script, because the tag is `async` and its position in the
 * head decides nothing.
 */

// The four actions this product measures (D-GA.2). A CLOSED union, not a
// `string`: adding a fifth action is an edit HERE, where the privacy rule is
// written down, rather than an invention at a call site.
export type AnalyticsAction =
  | 'open_template'
  | 'export_pdf'
  | 'font_import'
  | 'enter_preview'

declare global {
  interface Window {
    dataLayer?: unknown[]
  }
}

// GTM container ids are `GTM-` plus an upper-case alphanumeric run. Anything
// else — a GA4 `G-` id, a truncated paste, a shell variable that never
// expanded — is NOT a container and must not be injected as one.
const CONTAINER_ID_PATTERN = /^GTM-[A-Z0-9]+$/

// READ ONCE, AT MODULE SCOPE, ON PURPOSE. Vite substitutes a literal for
// `import.meta.env.VITE_GA_CONTAINER_ID` at build time, so with the var unset
// this becomes `const CONTAINER_ID = undefined` and every branch below is
// statically dead — which is what lets an unconfigured build contain no
// googletagmanager URL at all, rather than merely never requesting one.
const RAW_CONTAINER_ID: unknown = import.meta.env.VITE_GA_CONTAINER_ID
const CONTAINER_ID: string | undefined =
  typeof RAW_CONTAINER_ID === 'string' && CONTAINER_ID_PATTERN.test(RAW_CONTAINER_ID.trim())
    ? RAW_CONTAINER_ID.trim()
    : undefined

// ⚠ `trackEvent` GATES ON THIS, NOT ON `CONTAINER_ID` ALONE. A push that ran
// before initialisation would create `window.dataLayer` as a side effect, and
// the acceptance criterion is that an unconfigured page has no `dataLayer` at
// all — not that it has an empty one nobody reads.
let active = false

/**
 * Install the container. Idempotent: a second call injects nothing, because
 * two `gtm.js` scripts would double every pageview.
 *
 * Returns whether measurement is on, which is what the tests assert against;
 * no caller needs the value.
 */
export function initAnalytics(): boolean {
  if (active) return true
  if (CONTAINER_ID === undefined) return false
  if (typeof document === 'undefined' || typeof window === 'undefined') return false
  // GTM's snippet, in order: seed the queue, stamp the start, then load the
  // container. The queue is a plain array precisely so pushes made before —
  // or entirely without — the container arriving are harmless appends.
  // ⚠ ADOPT THE EXISTING QUEUE ONLY IF IT IS ONE. `window.dataLayer ?? []`
  // adopts whatever is already on `window` — a browser extension's object, a
  // host page's stub — and the `.push` below then throws, inside the module
  // that boots the designer. An occupied name is not a queue; replace it.
  const dataLayer = Array.isArray(window.dataLayer) ? window.dataLayer : (window.dataLayer = [])
  dataLayer.push({ 'gtm.start': Date.now(), event: 'gtm.js' })
  const script = document.createElement('script')
  // ⚠ `async`, ALWAYS. The app must behave identically when
  // googletagmanager.com is unreachable — ad-blocked, offline, or DNS-refused
  // — and a synchronous third-party script in the head would make an outage
  // there an outage here. Nothing in this module observes the load, and there
  // is no error handler because there is nothing to recover: the container
  // either arrives or the queue simply grows and is never drained.
  script.async = true
  script.src = `https://www.googletagmanager.com/gtm.js?id=${encodeURIComponent(CONTAINER_ID)}`
  document.head.appendChild(script)
  active = true
  return true
}

/**
 * Report one action. Safe to call at any time, from any state: before
 * initialisation, with the container blocked, with the network down.
 *
 * ⚠ SAFE MEANS IT NEVER THROWS — IT DOES NOT MEAN DELIVERED LATER. Called
 * before `initAnalytics`, or in any build without a container id, it DISCARDS
 * the event silently; there is no backlog and nothing is replayed when the
 * container arrives. Do not build anything on the other reading.
 *
 * ⚠ THE PAYLOAD IS THE ACTION NAME AND NOTHING ELSE. Do not add a parameter
 * here, and do not widen `action`. Every value that reaches GTM must be drawn
 * from `AnalyticsAction`, which is the whole of the privacy bound this feature
 * was allowed to exist under.
 */
export function trackEvent(action: AnalyticsAction): void {
  if (!active) return
  window.dataLayer?.push({ event: 'folio8_action', folio8_action: action })
}
