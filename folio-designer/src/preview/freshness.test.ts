import { describe, expect, it } from 'vitest'
import { canInstallPreview, formatRenderAge, freshnessChrome, PREVIEW_DEBOUNCE_MS, PREVIEW_FRESHNESS_STATES, PreviewWorkScheduler, renderAgeTickMs, staleCopy } from './freshness'

describe('preview freshness authority', () => {
  it('documents the short debounce and never installs an outdated authority tuple', () => {
    expect(PREVIEW_DEBOUNCE_MS).toBe(250)
    const authority = { token: 4, generation: 7, revision: 11, identity: 'a'.repeat(64), mode: 'preview' as const }
    expect(canInstallPreview({ token: 4, generation: 7, revision: 11, identity: 'a'.repeat(64) }, authority)).toBe(true)
    expect(canInstallPreview({ token: 3, generation: 7, revision: 11, identity: 'a'.repeat(64) }, authority)).toBe(false)
    expect(canInstallPreview({ token: 4, generation: 8, revision: 11, identity: 'a'.repeat(64) }, authority)).toBe(false)
    expect(canInstallPreview({ token: 4, generation: 7, revision: 11, identity: 'b'.repeat(64) }, authority)).toBe(false)
  })

  it('uses permanent truthful stale language', () => {
    expect(staleCopy('inputs-changed')).toBe('STALE — inputs changed')
    expect(staleCopy('render-failed')).toBe('STALE — latest local render failed')
  })

  it('keeps only an active worker operation and its newest replacement', async () => {
    const scheduler = new PreviewWorkScheduler()
    const order: string[] = []
    let release!: () => void
    const active = () => new Promise<void>((resolve) => { order.push('active'); release = resolve })
    scheduler.submit(active)
    scheduler.submit(async () => { order.push('obsolete') })
    scheduler.submit(async () => { order.push('newest') })
    expect(scheduler.active).toBe(true)
    expect(scheduler.hasPending).toBe(true)
    expect(order).toEqual(['active'])
    release()
    await Promise.resolve()
    await Promise.resolve()
    expect(order).toEqual(['active', 'newest'])
    expect(scheduler.active).toBe(false)
  })
})

describe('preview freshness chrome', () => {
  const chrome = (status: (typeof PREVIEW_FRESHNESS_STATES)[number], over: Partial<Parameters<typeof freshnessChrome>[0]> = {}) =>
    freshnessChrome({ status, staleReason: 'inputs-changed', standIn: false, hasRecord: true, ...over })

  // THE COPY IS THE COPY. This story moved the status line's production behind
  // one function; it did not get to reword it on the way through. Every string
  // below is the wording that was on screen before the move, asserted whole so
  // a paraphrase is a failure and not a diff nobody reads.
  it('reproduces every status line exactly as the screen already said it', () => {
    expect(chrome('current').statusLine).toBe('Current exact local PDF')
    expect(chrome('current', { standIn: true }).statusLine).toBe('Current no-data layout PDF')
    expect(chrome('stale').statusLine).toBe('STALE — inputs changed')
    expect(chrome('stale', { staleReason: 'render-failed' }).statusLine).toBe('STALE — latest local render failed')
    expect(chrome('stale', { failureMessage: 'boom' }).statusLine).toBe('STALE — inputs changed; local PDF render failed: boom')
    expect(chrome('stale', { issue: 'the digest did not match' }).statusLine).toBe('STALE — inputs changed; the digest did not match')
    // A failure OUTRANKS an issue on the stale line and an issue outranks a
    // failure on the error line — opposite precedences, exactly as they were.
    expect(chrome('stale', { issue: 'i', failureMessage: 'f' }).statusLine).toBe('STALE — inputs changed; local PDF render failed: f')
    for (const status of ['checking', 'debouncing', 'rendering'] as const) expect(chrome(status).statusLine).toBe('Rendering local PDF')
    expect(chrome('error').statusLine).toBe('Local Preview work failed')
    expect(chrome('error', { issue: 'the digest did not match' }).statusLine).toBe('Local Preview work failed: the digest did not match')
    expect(chrome('error', { failureMessage: 'boom' }).statusLine).toBe('Local Preview work failed: boom')
    expect(chrome('error', { issue: 'i', failureMessage: 'f' }).statusLine).toBe('Local Preview work failed: i')
    expect(chrome('idle').statusLine).toBe('Preview is waiting for local inputs')
  })

  // AN EMPTY DETAIL IS NOT A DETAIL, AND THE TWO SUFFIXES AGREE ABOUT THAT.
  //
  // The two branches originally tested their two suffixes differently — one for
  // a non-empty string, the other for `!== undefined` — and the difference was
  // reachable rather than theoretical: the call site passes
  // `currentFailure?.error.message`, and an `Error`'s `message` is an ordinary
  // string that nothing forces non-empty. Under the `!== undefined` test an
  // empty one printed a colon introducing nothing — `…failed: ` — which is the
  // chrome asserting there is a reason and then not giving one, in a story whose
  // whole subject is the chrome telling the truth.
  it('says nothing rather than trailing a colon when a failure carries an empty message', () => {
    expect(chrome('stale', { failureMessage: '' }).statusLine).toBe('STALE — inputs changed')
    expect(chrome('stale', { staleReason: 'render-failed', failureMessage: '' }).statusLine).toBe('STALE — latest local render failed')
    expect(chrome('error', { failureMessage: '' }).statusLine).toBe('Local Preview work failed')
    expect(chrome('error', { issue: '' }).statusLine).toBe('Local Preview work failed')
    // AND AN EMPTY LEAD-IN DOES NOT SWALLOW A DETAIL THAT DOES EXIST: the stale
    // line falls through to the issue instead of printing the failure's empty
    // one, which is what "consistent" has to mean to be worth anything.
    expect(chrome('stale', { failureMessage: '', issue: 'the digest did not match' }).statusLine).toBe('STALE — inputs changed; the digest did not match')
    expect(chrome('error', { issue: '', failureMessage: 'boom' }).statusLine).toBe('Local Preview work failed: boom')
    // No status line anywhere ends in a colon or a semicolon-space lead-in.
    for (const status of PREVIEW_FRESHNESS_STATES) for (const detail of ['', undefined]) {
      const line = freshnessChrome({ status, staleReason: 'render-failed', standIn: false, hasRecord: true, issue: detail, failureMessage: detail }).statusLine
      expect(line).toBe(line.trimEnd())
      expect(line).not.toMatch(/[:;]$/)
    }
  })

  // THE INVARIANT, OVER THE VOCABULARY ITSELF. The loop iterates the module's
  // own exported state list rather than seven strings restated here: a mirror
  // maintained by hand is exactly the guard that stays green while an eighth
  // state goes unexamined.
  //
  // `current` WITHOUT A RECORD IS NOT IN THE DOMAIN, and that is a statement
  // about the app rather than a gap in the loop: `App.tsx`'s `viewerPages` is
  // the only path to a durable `current` and it runs off an installed record.
  // Asserting the undefined arm over an unreachable pair would force the token
  // to contradict its own status line there.
  it('ties the bar token to the status line and to the presence of a record, across every declared state', () => {
    expect(PREVIEW_FRESHNESS_STATES).toHaveLength(7)
    const tokens = new Set<string | undefined>()
    for (const status of PREVIEW_FRESHNESS_STATES) {
      for (const standIn of [false, true]) {
        const held = freshnessChrome({ status, staleReason: 'inputs-changed', standIn, hasRecord: true })
        tokens.add(held.token)
        // The iff, in both directions at once.
        expect(held.token === 'current').toBe(held.statusLine.startsWith('Current'))
        // With a record there is always something to say about it.
        expect(held.token).toBeDefined()
        if (held.token === 'stale') expect(held.statusLine.startsWith('Current')).toBe(false)
        // THE TOKEN IS A FRESHNESS CLAIM AND NEVER AN EXACTNESS CLAIM: a
        // no-data render that is current still reads `current` in the bar, and
        // 13.4's disclosure stays where 13.4 put it.
        expect(held.token).toBe(freshnessChrome({ status, staleReason: 'inputs-changed', standIn: !standIn, hasRecord: true }).token)
        if (status === 'current') continue
        const none = freshnessChrome({ status, staleReason: 'inputs-changed', standIn, hasRecord: false })
        expect(none.token).toBeUndefined()
        // And the line itself does not move when the record does — only the
        // token does, which is what keeps the two from disagreeing.
        expect(none.statusLine).toBe(held.statusLine)
      }
    }
    // NON-VACUOUS IN BOTH DIRECTIONS: a switch that answered `stale` to
    // everything would satisfy every assertion above.
    expect(tokens).toEqual(new Set(['current', 'stale']))
  })

  // MATRIX ROW: a digest mismatch over a surviving preview sets `error` WITHOUT
  // installing, so a record is on screen that the app has just refused to
  // affirm. The bar must be conservative there — never `current` — while the
  // status line stays specific. This is the divergence, asserted deliberately.
  it('refuses to affirm a render the app itself refused', () => {
    const refused = chrome('error', { issue: 'the rendered PDF does not match the digest' })
    expect(refused.token).toBe('stale')
    expect(refused.statusLine).toBe('Local Preview work failed: the rendered PDF does not match the digest')
  })
})

describe('render age', () => {
  // THE LADDER AT EVERY BOUNDARY IT HAS. The invariant being defended is that
  // the figure is never wrong by more than the unit it prints, so each tier is
  // asserted at its last millisecond and at the first millisecond of the next.
  it('walks the four tiers and never prints a figure wrong by more than its own unit', () => {
    const at = (age: number) => formatRenderAge(1_000_000, 1_000_000 + age)
    expect(at(0)).toBe('0 ms ago')
    expect(at(999)).toBe('999 ms ago')
    expect(at(1000)).toBe('1 s ago')
    expect(at(59_999)).toBe('59 s ago')
    expect(at(60_000)).toBe('1 min ago')
    expect(at(3_599_999)).toBe('59 min ago')
    expect(at(3_600_000)).toBe('1 h ago')
    expect(at(86_400_000)).toBe('24 h ago')
  })

  // `Date.now()` IS NOT MONOTONIC. An NTP step backwards while a preview is on
  // screen must never put `-3 s ago` in the chrome.
  it('clamps a clock that steps backwards to zero rather than counting down', () => {
    expect(formatRenderAge(1_000_000, 999_000)).toBe('0 ms ago')
    expect(formatRenderAge(1_000_000, 0)).toBe('0 ms ago')
    expect(renderAgeTickMs(1_000_000, 999_000)).toBe(100)
  })

  // A NON-FINITE ARGUMENT IS THE OTHER DEGENERATE CLOCK, and it was unguarded
  // while the stated sibling `formatElapsed` guards `!Number.isFinite`.
  //
  // TWO CONSEQUENCES, AND THE SECOND IS THE SERIOUS ONE. The visible one is
  // `rendered NaN ms ago · current` in the bar. The other is that the SAME
  // subtraction feeds the tick period, and `setInterval(fn, NaN)` is
  // `setInterval(fn, 0)` — a busy loop repainting the document bar as fast as
  // the browser will schedule it, for as long as the preview is open. Both are
  // closed by clamping to zero at the one place the arithmetic happens.
  it('clamps a clock that is not a number at all, and never hands the ticker a period that is not one either', () => {
    for (const bad of [Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY]) {
      expect(formatRenderAge(bad, 1_000_000)).toBe('0 ms ago')
      expect(formatRenderAge(1_000_000, bad)).toBe('0 ms ago')
      expect(renderAgeTickMs(bad, 1_000_000)).toBe(100)
      expect(renderAgeTickMs(1_000_000, bad)).toBe(100)
    }
    expect(formatRenderAge(Number.NaN, Number.NaN)).toBe('0 ms ago')
    // The guard reads both arguments, so a figure is a figure whichever end is
    // broken; nothing here may print the word `NaN` or `Infinity`.
    expect(formatRenderAge(Number.POSITIVE_INFINITY, Number.POSITIVE_INFINITY)).not.toMatch(/NaN|Infinity/)
  })

  // THE TICK PERIOD IS THE LADDER'S, taken from the same two arguments the text
  // is, so a display printing whole minutes cannot go on repainting ten times a
  // second and a display printing milliseconds cannot go a second between
  // repaints.
  it('paces the repaint to the unit on screen', () => {
    const tick = (age: number) => renderAgeTickMs(1_000_000, 1_000_000 + age)
    expect(tick(0)).toBe(100)
    expect(tick(999)).toBe(100)
    expect(tick(1000)).toBe(1000)
    expect(tick(59_999)).toBe(1000)
    expect(tick(60_000)).toBe(60_000)
    expect(tick(3_599_999)).toBe(60_000)
    expect(tick(3_600_000)).toBe(60_000)
  })
})
