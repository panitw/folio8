# Usage measurement (Google Tag Manager) — decision log

The run-scoped record for `spec-google-analytics`, delivered 2026-09-21. One entry per decision,
newest appended. `sprint-status.yaml` carries status values only; the per-story narrative lives in
the spec's `## Delivery Log`.

**Every decision here was taken by the OWNER before planning began**, and none is open to
re-litigation by an implementer. They are load-bearing for the whole change, because the feature is
not a dependency bump — it **reverses a recorded architecture decision**. The reversal itself is
D-GA.1; the rest bound it.

| ID | What it settles |
|---|---|
| D-GA.1 | **Owner.** The *no telemetry* posture is **reversed**, and the architecture docs are **amended to record the reversal with its rationale** rather than quietly contradicted. NFR8's substantive guarantee — *templates and data never leave the user's machine* — is **PRESERVED**: GTM may report usage, it may never carry document or data content. Recorded as **AD-27**. |
| D-GA.2 | **Owner.** Scope is the pageview plus custom events for **four** actions: open template, export, font import, preview. Not a fifth, and not a free-form event name. |
| D-GA.3 | **Owner.** The container id comes from a **Vite env var** (`VITE_GA_CONTAINER_ID`). Unset means **no script load, no `dataLayer`, no events** — so `npm run dev`, Vitest and Playwright never transmit, without anyone having to remember to disable anything. |
| D-GA.4 | **Owner.** The tag is **Google Tag Manager** (container `GTM-NQZRC9V4`), not GA4 `gtag.js` directly. Events are `dataLayer` pushes; tag configuration lives in the GTM console, not in this repository. |
| D-GA.5 | **Owner.** The visible standing promise is **amended, not abandoned**. Preview's `no network · nothing left this machine` becomes `local render · your data stays here` — a statement that is still true once GTM loads, because the guarantee that survives is about the user's *data*, not about the page making zero requests. The three assertions and the two `epics.md` quotations that pin the old string move with it. This reverses the half of the promise D-GA.1 made false; it does not delete the promise. |
| D-GA.6 | **Owner.** The `<noscript>` iframe is **omitted**. The designer renders nothing without JavaScript, so a no-JS visitor is never a real user and the iframe would measure nothing — and omitting it avoids a second remote subresource needing its own env gate. |
| D-GA.7 | **Owner.** **Env-gated runtime injection, not GTM's literal static paste.** GTM's copy-paste instructions would fire the container on every dev server and every e2e run, which is exactly what D-GA.3 forbids. A module injects a functionally equivalent snippet — minus the `<noscript>` iframe, and not byte-identical: see the `analytics.ts` docblock for the three deliberate differences — only when the env var holds a valid id. GTM's instructions are generic boilerplate and are deliberately not followed to the letter. |
| D-GA.8 | **Owner.** The spec is kept whole at ~3,800 tokens (the workflow ceiling is 1,600) so the doc amendment stays coupled to the code that makes it necessary. Implementation works section by section rather than holding the whole spec at once. |

## What the implementation added to these, and why it is not a ninth decision

Two choices were forced by D-GA.1 and D-GA.3 rather than left open, and are recorded here so a
reader does not mistake them for latitude:

- **The event parameter is a closed TypeScript union, not a `string`.** D-GA.1's preserved
  guarantee — no document content in an event — is otherwise a convention the next contributor has
  to be told about. As a union it is a compile-time property of every call site. Widening it to
  `string` would silently repeal the only thing that makes D-GA.1 acceptable.
- **A malformed container id is treated as unset (fail closed).** D-GA.3 says the env var is the
  sole switch; a truncated paste, an unexpanded shell variable or a GA4 `G-` id is not a container,
  and injecting it would be a live third-party request that measures nothing.

## Amended documents

The sweep D-GA.1 requires, so a reader grepping for the old claim finds either the new reality or a
struck original with its amendment beside it:

| Document | What moved |
|---|---|
| `ARCHITECTURE-SPINE.md` | AD-19's *"Nothing in the designer performs a network request at render or preview time"* struck and amended; the Deployment table's *"no telemetry"* struck and amended; **AD-27** added, recording the reversal, its bound and its cost. |
| `prd.md` | NFR8 narrowed to what stays true, with the amendment appended below the original paragraph. |
| `epics.md` | NFR8's restatement amended; UX-DR23's two quotations of the old Preview wording moved to the amended string. |
| `README.md` | The designer row states the surviving guarantee and names the one third party. |
| `EXPERIENCE.md` | *"no network is required and none is used"* struck and amended to *"none is used for the user's work"*. |
| `src/App.tsx` | The Preview assurance string, and the comment block that calls it the product's standing promise, both amended under D-GA.5. |
