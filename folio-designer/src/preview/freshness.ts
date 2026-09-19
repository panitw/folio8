// This state vocabulary is intentionally independent of PDF.js and document
// commands. A preview can become stale synchronously, while the worker keeps
// its single FIFO operation in flight and the UI discards obsolete results.
export const PREVIEW_DEBOUNCE_MS = 250

// STORY 13.5 — THE VOCABULARY IS AN ARRAY AND THE TYPE IS DERIVED FROM IT.
// It was a hand-written union, which meant any guard claiming to cover "every
// state" had to restate the seven strings and would go on passing, unexamined,
// against an eighth. Exported as values, the states can be ITERATED, so an
// exhaustiveness claim is made against the vocabulary itself.
export const PREVIEW_FRESHNESS_STATES = ['idle', 'checking', 'debouncing', 'rendering', 'current', 'stale', 'error'] as const
export type PreviewFreshness = (typeof PREVIEW_FRESHNESS_STATES)[number]
export type StaleReason = 'inputs-changed' | 'render-failed'

export const staleCopy = (reason: StaleReason): string => reason === 'inputs-changed' ? 'STALE — inputs changed' : 'STALE — latest local render failed'

// The worker is FIFO and cannot pull a message back once posted.  Keep one
// request active and retain at most the newest replacement locally; when the
// active request settles, only that replacement is admitted.  This is the
// boundary that prevents manual clicks and debounce expiry from building an
// unbounded queue of abandoned worker messages.
export class PreviewWorkScheduler {
  #active = false
  #pending: (() => Promise<void>) | undefined

  submit(job: () => Promise<void>): void {
    if (this.#active) { this.#pending = job; return }
    this.#start(job)
  }

  clear(): void { this.#pending = undefined }

  get active(): boolean { return this.#active }
  get hasPending(): boolean { return this.#pending !== undefined }

  #start(job: () => Promise<void>): void {
    this.#active = true
    void job().finally(() => {
      const next = this.#pending
      this.#pending = undefined
      if (next) this.#start(next)
      else this.#active = false
    })
  }
}

export function canInstallPreview(candidate: Readonly<{ token: number; generation: number; revision: number; identity: string }>, authority: Readonly<{ token: number; generation: number; revision: number; identity: string; mode: 'design' | 'preview' }>): boolean {
  return authority.mode === 'preview' && candidate.token === authority.token && candidate.generation === authority.generation && candidate.revision === authority.revision && candidate.identity === authority.identity
}

/**
 * STORY 13.5 — ONE PRODUCER FOR BOTH RENDERINGS OF FRESHNESS.
 *
 * The document bar's short token and the preview heading's status line are two
 * statements about the same fact, and until this function existed they were two
 * independent string expressions that could disagree. They now come out of ONE
 * switch, together, so "the chrome and the status line never disagree" is true
 * by construction rather than by review.
 *
 * THE TOKEN IS A FRESHNESS CLAIM AND CARRIES NO EXACTNESS CLAIM. `standIn` is an
 * input — the status line branches on it — but the token deliberately does not:
 * a no-data render that is current is still `current`, and Story 13.4's
 * exactness disclosure stays where 13.4 put it (the heading, the status line's
 * own `Current no-data layout PDF`, and the stand-in notice). Taking `standIn`
 * anyway means a later story that DOES want the bar to distinguish a no-data
 * preview must change this switch rather than write a second string elsewhere.
 *
 * `error` WITH A RECORD READS `stale`, DELIBERATELY DIVERGING from the status
 * line's more specific copy. `App.tsx`'s digest-mismatch path sets `error`
 * WITHOUT installing, over a preview that is still on screen — so the bar is
 * describing bytes the app has just refused to affirm. Story 13.3 spent a patch
 * on exactly this failure in the evidence rail; the bar is conservative here for
 * the same reason.
 */
export type FreshnessChrome = Readonly<{ token: 'current' | 'stale' | undefined; statusLine: string }>

/**
 * `issue` and `failureMessage` are the two detail suffixes the status line has
 * always carried. They are inputs rather than a second call site's concatenation
 * because the whole point of this function is that no call site builds any part
 * of either string itself.
 *
 * BOTH SUFFIXES ARE TESTED THE SAME WAY — for a NON-EMPTY string, not for
 * `!== undefined`. The two branches below used to disagree about that, and the
 * disagreement was visible: an `EngineError` whose `message` is `''` is a real
 * shape (`error.message` is a plain string that nothing forces non-empty), and
 * against a `!== undefined` test it produced `STALE — latest local render
 * failed; local PDF render failed: ` and `Local Preview work failed: ` —
 * a colon introducing nothing. An empty detail is not a detail, so it now reads
 * exactly as an absent one does, and the `stale` branch falls through to `issue`
 * rather than printing an empty lead-in over a detail it could have shown.
 * The ORDER of the two suffixes is unchanged in both branches: `stale` prefers
 * the render failure, `error` prefers the issue, exactly as the call site did.
 */
export function freshnessChrome({ status, staleReason, standIn, hasRecord, issue, failureMessage }: Readonly<{ status: PreviewFreshness; staleReason: StaleReason; standIn: boolean; hasRecord: boolean; issue?: string; failureMessage?: string }>): FreshnessChrome {
  // The token, first, from the same `status` the line below reads. `current`
  // leads: it is the only state that can truthfully affirm the render, and it
  // is unreachable without an installed record (`App.tsx`'s `viewerPages` is
  // the only path to a durable `current`, and it runs off the record).
  const token = status === 'current' ? 'current' as const : hasRecord ? 'stale' as const : undefined
  switch (status) {
    case 'current': return { token, statusLine: standIn ? 'Current no-data layout PDF' : 'Current exact local PDF' }
    case 'stale': return { token, statusLine: `${staleCopy(staleReason)}${failureMessage ? `; local PDF render failed: ${failureMessage}` : issue ? `; ${issue}` : ''}` }
    case 'checking': case 'debouncing': case 'rendering': return { token, statusLine: 'Rendering local PDF' }
    case 'error': return { token, statusLine: `Local Preview work failed${issue ? `: ${issue}` : failureMessage ? `: ${failureMessage}` : ''}` }
    case 'idle': return { token, statusLine: 'Preview is waiting for local inputs' }
  }
}

/**
 * HOW LONG AGO THE RENDER FINISHED — NOT HOW LONG IT TOOK.
 *
 * These are different quantities and the screen shows both. `formatElapsed`
 * (`evidence-rail-facts.ts`) prints the engine's own `elapsedMs`, the duration
 * of the render; this prints the age of its result. They are equal for exactly
 * one instant and diverge forever after, so they are deliberately NOT one
 * function: sharing a formatter would be the conflation itself, and
 * `formatElapsed` has no minutes tier and could not carry this ladder anyway.
 *
 * PURE, AND TAKING ITS CLOCK AS AN ARGUMENT. `Date.now()` is never read in this
 * module at all — the three reads all live in `App.tsx` (the `now` state's
 * initializer, the install stamp, and the ticker), and none of them happens
 * during render — so the whole ladder is testable with no fake timers. This is
 * the repo's existing idiom (`font-source.ts` injects today's date the same
 * way).
 *
 * The ladder never prints a figure wrong by more than the unit it prints, and a
 * NEGATIVE AGE CLAMPS TO ZERO: `Date.now()` is not monotonic, and an NTP step
 * backwards must never put `-3 s ago` on the chrome.
 *
 * A NON-FINITE ARGUMENT CLAMPS TO ZERO TOO, and the guard lives HERE rather
 * than in the formatter because both public callers run through this one
 * subtraction. `formatElapsed` (`evidence-rail-facts.ts`) guards
 * `!Number.isFinite` and returns an empty string; this one cannot copy that
 * answer, because its result is interpolated into `rendered … · current` and an
 * empty figure would print `rendered  · current`. Zero is the answer that
 * matches the clamp already stated above: never a figure that is not a figure.
 * The second caller is why it matters more than tidiness — an unguarded `NaN`
 * reaches `renderAgeTickMs`, and `setInterval(fn, NaN)` is `setInterval(fn, 0)`,
 * which is a busy loop repainting the bar forever.
 */
export function renderAge(installedAt: number, now: number): number {
  if (!Number.isFinite(installedAt) || !Number.isFinite(now)) return 0
  return Math.max(0, Math.floor(now - installedAt))
}

export function formatRenderAge(installedAt: number, now: number): string {
  const age = renderAge(installedAt, now)
  if (age < 1000) return `${age} ms ago`
  if (age < 60_000) return `${Math.floor(age / 1000)} s ago`
  if (age < 3_600_000) return `${Math.floor(age / 60_000)} min ago`
  return `${Math.floor(age / 3_600_000)} h ago`
}

/**
 * THE TICK PERIOD COMES OFF THE SAME LADDER THE TEXT DOES, from the same two
 * arguments, so the effect and the formatter cannot drift into a display that
 * repaints on a schedule its own unit has outgrown.
 */
export function renderAgeTickMs(installedAt: number, now: number): number {
  const age = renderAge(installedAt, now)
  if (age < 1000) return 100
  if (age < 60_000) return 1000
  return 60_000
}
