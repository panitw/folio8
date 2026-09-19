import { BYTE_IDENTITY_SENTENCE, formatElapsed, formatRenderSize, hashLines } from './evidence-rail-facts'

/**
 * STORY 13.3 — THE EVIDENCE RAIL.
 *
 * The preview's central claim is *these are the exact production bytes, and here
 * is the hash that proves it*. Until this story that claim survived on screen as
 * one grey footnote line. This is the surface the design drew for it: the facts
 * about the render that happened, the digest in its own bordered block big
 * enough to compare by eye, the diagnostics with their counts and a shape
 * legend, and Re-render / Save PDF paired at the foot beside the evidence they
 * act on.
 *
 * ⚠ IT IS A SIBLING OF THE INSPECTOR'S TABPANELS, NEVER INSIDE ONE. DW-281 —
 * an owner request, verbatim *"Later this button should be moved to the preview
 * area."* — is about `hidden={inspectorTab !== 'properties'}`: with the DATA tab
 * selected, Save PDF left the accessibility tree entirely, and a rail placed
 * inside that same tabpanel would inherit the defect rather than discharge it.
 *
 * COMPONENTS AND TYPES ONLY. Every non-component value lives in
 * `evidence-rail-facts.ts`; a value export from this file would add a fifth
 * `only-export-components` warning to a lint baseline that is pinned at four.
 */

/**
 * THE FIVE VALUES, AND THE PROVENANCE OF EACH — which is not uniform, and the
 * difference matters more than the list does.
 *
 * `engineVersion` and `elapsedMs` are the ENGINE's own answers, measured inside
 * Go and carried on the render reply. `sizeBytes` is the length of the buffer
 * the digest covers. `target` is a fact about which engine ran. `pages` is the
 * VIEWER's count, from PDF.js's admission of these same bytes (Story 13.2) — a
 * different authority from the other four, and the only one that can be absent
 * while a render exists, because bytes are displayed before PDF.js has admitted
 * them.
 *
 * `rows` IS DELIBERATELY ABSENT and its absence is a decision (owner, Q1,
 * 2026-09-08): the count exists only inside the renderer, surfacing it would
 * change `folio8.Render`'s exported contract, and one scalar cannot distinguish
 * *no tables* from *a table bound to an empty collection*.
 */
export type RenderFacts = Readonly<{
  engineVersion: string
  target: string
  pages: number | undefined
  elapsedMs: number
  sizeBytes: number
}>

export type HashFacts = Readonly<{ digest: string; standIn: boolean }>

export type EvidenceRailProps = Readonly<{
  render: RenderFacts | undefined
  hash: HashFacts | undefined
  stale: boolean
  /** What the described render reported, whether or not its cards are on screen. */
  warnings: number
  /** How many of those cards the author can actually point at right now. */
  warningsOnScreen: number
  errors: number
  onRerender: () => void
  exportLabel: string
  exportDisabled: boolean
  exportUnavailable: string | undefined
  onExport: () => void
}>

function RenderBlock({ facts, stale }: { facts: RenderFacts | undefined; stale: boolean }) {
  return <section className="rail-section rail-render" aria-label="Render facts">
    <p className="section-label">RENDER</p>
    {/* UX-DR14. A stale rail keeps showing the earlier render's numbers,
        because they describe the bytes still on screen — but it must never
        let them read as a description of the current document. */}
    {stale && facts && <p className="rail-qualifier">These values describe the earlier render, not the current document.</p>}
    {facts
      ? <dl className="rail-facts">
        <div className="rail-fact"><dt>engine</dt><dd>{facts.engineVersion}</dd></div>
        <div className="rail-fact"><dt>target</dt><dd>{facts.target}</dd></div>
        {/* THE PAGE COUNT IS THE VIEWER'S, NOT THE ENGINE'S, and it is withheld
            until PDF.js has actually admitted the document. An engine-reported
            count would be a different number with different authority; showing
            a guess here would put an unearned figure in the one block whose
            rule is that every value came from the render that happened. */}
        {facts.pages !== undefined && <div className="rail-fact"><dt>pages</dt><dd>{facts.pages}</dd></div>}
        <div className="rail-fact"><dt>elapsed</dt><dd>{formatElapsed(facts.elapsedMs)}</dd></div>
        <div className="rail-fact"><dt>size</dt><dd>{formatRenderSize(facts.sizeBytes)}</dd></div>
      </dl>
      : <p className="honest-note">No local render has produced a document yet.</p>}
  </section>
}

function HashBlock({ facts }: { facts: HashFacts | undefined }) {
  return <section className="rail-section rail-hash" aria-label="Output hash">
    <p className="section-label">OUTPUT HASH</p>
    {facts
      ? <>
        <p className="rail-hash-kind">{facts.standIn ? 'Stand-in local digest' : 'Historical producer digest'}</p>
        {/* THE WHOLE HASH, IN TWO FIXED LINES. The mockup shows 32 characters;
            a digest a person is told to compare by eye must be all 64 of them.
            The break is in the markup so it lands in the same place on every
            machine — see `hashLines`. */}
        {/* KEYED BY POSITION, because the list IS positional and the two halves
            can be equal — `'a'.repeat(64)` is a real fixture in these suites,
            and identical keys make React drop one of the two lines. */}
        <code className="rail-hash-value">{hashLines(facts.digest).map((line, index) => <span className="rail-hash-line" key={index}>{line}</span>)}</code>
        {/* WITHHELD ENTIRELY FOR A STAND-IN PREVIEW. A no-data digest is not
            evidence of cross-target equality (D-13.4.1), so the sentence is
            absent rather than softened. */}
        {!facts.standIn && <p className="honest-note rail-identity">{BYTE_IDENTITY_SENTENCE}</p>}
      </>
      : <p className="honest-note">Go production digest pending</p>}
  </section>
}

function DiagnosticsBlock({ warnings, warningsOnScreen, errors, hasRender, stale }: { warnings: number; warningsOnScreen: number; errors: number; hasRender: boolean; stale: boolean }) {
  // ⚠ "A RENDER COMPLETED AND THIS IS IT" IS A NARROWER CLAIM THAN "A RENDER
  // EXISTS", AND THE ZERO STATE IS ONLY EVER ABOUT THE FORMER (review P1).
  //
  // The zero state used to be gated on `hasRender && warnings === 0 && errors
  // === 0`, which reads the counts of whichever record is installed. A digest
  // mismatch installs nothing and sets `previewIssue` rather than
  // `previewError`, so `errors` stays 0 and the PREVIOUS clean preview's counts
  // are still 0 — and the screen affirmed "The render completed and reported
  // zero diagnostics." about a render it had just refused for corruption. On
  // the one surface whose rule is never to print an affirmation it cannot earn,
  // that is the defect this story exists to prevent, arriving through the
  // section nobody was looking at.
  const describesCurrent = hasRender && !stale
  const dismissedHere = describesCurrent ? warnings - warningsOnScreen : 0
  return <section className="rail-section rail-diagnostics" aria-label="Diagnostics summary">
    <div className="rail-diagnostics-header">
      <p className="section-label">DIAGNOSTICS</p>
      <span className="rail-count rail-count-warnings">warnings {warnings}</span>
      {/* ⚠ THE ERROR COUNT HAS NO SOURCE IN THE DIAGNOSTICS ARRAY, AND THAT IS
          NOT AN OVERSIGHT. `EngineDiagnostic.severity` is the literal
          'warning' — the type carries no error severity at all — so counting
          that array for errors would write a number that can only ever be
          zero and would look correct forever. It comes from the render
          failure, which is also exactly what the second legend row describes. */}
      <span className="rail-count rail-count-errors">errors {errors}</span>
    </div>
    {/* AND THE COUNT SAYS WHOSE IT IS WHENEVER IT IS NOT THE VISIBLE SET
        (review P2). `warnings 2` above an empty list is a number the author
        cannot point at anything for; the cards are dismissible locally and are
        withheld entirely while the preview is not the admitted one, so both
        gaps are named rather than left to be inferred. */}
    {hasRender && !describesCurrent && <p className="honest-note rail-diagnostics-scope">These counts describe the earlier render; its cards are not shown.</p>}
    {dismissedHere > 0 && <p className="honest-note rail-diagnostics-scope">{dismissedHere} dismissed on this screen.</p>}
    {describesCurrent && warnings === 0 && errors === 0 && <p className="honest-note rail-zero">The render completed and reported zero diagnostics.</p>}
    {/* THE LEGEND IS THE SAME PARTITION AS THE TWO COUNTS, NAMED TWICE: the
        dashed triangle is the diagnostics array (a render that proceeded), the
        solid square is the failure card (a render that stopped). */}
    {/* ⚠ BOTH MARKS TAKE THE SAME NEUTRAL INK, DELIBERATELY. The mockup draws
        the legend's triangle and square in one grey: a legend is a KEY, not a
        signal, and colouring these two the way live diagnostics are coloured
        would put a warning and an error on screen that nothing is reporting.
        The shape carries the distinction, which is also why it survives for a
        reader who cannot see the accents at all. (Review P11: the per-mark
        class names were dropped rather than given rules — they promised a
        distinction this design does not make, and a hook nothing styles is
        indistinguishable from its own absence.) */}
    <ul className="rail-legend">
      <li><span className="rail-legend-mark" aria-hidden="true">▲</span>Triangle, dashed — render proceeded</li>
      <li><span className="rail-legend-mark" aria-hidden="true">■</span>Square, solid — render failed</li>
    </ul>
  </section>
}

export function PreviewEvidenceRail({ render, hash, stale, warnings, warningsOnScreen, errors, onRerender, exportLabel, exportDisabled, exportUnavailable, onExport }: EvidenceRailProps) {
  return <section className="evidence-rail" aria-label="Render evidence">
    {/* THE EVIDENCE SCROLLS; THE ACTIONS DO NOT. Measured in the browser: with
        the rail as one scroller, a rail whose diagnostics run long pushed
        Re-render and Save PDF below the fold — the controls DW-281 exists to
        keep reachable, unreachable again by a different mechanism. */}
    <div className="rail-scroll">
      <RenderBlock facts={render} stale={stale} />
      <HashBlock facts={hash} />
      <DiagnosticsBlock warnings={warnings} warningsOnScreen={warningsOnScreen} errors={errors} hasRender={render !== undefined} stale={stale} />
    </div>
    {/* THE ACTION ROW, AT THE RAIL'S FOOT AND OUTSIDE EVERY TABPANEL. The
        export's reason note travels WITH its button: `aria-describedby` points
        at it by id, and splitting the two silently breaks the association. */}
    {/* ⚠ `role="group"`, NOT A BARE `<div>` (review P8). A plain div has a
        generic role, so `aria-label` on it announces to nothing — the label was
        reachable only because RTL and Playwright match the attribute directly,
        which is a testing-library fact rather than an accessibility one. */}
    <div className="rail-actions" role="group" aria-label="Render actions">
      <div className="rail-action-row">
        <button type="button" className="file-button" onClick={onRerender}>Re-render</button>
        <button type="button" className="file-button" onClick={onExport} disabled={exportDisabled} aria-describedby={exportUnavailable ? 'preview-pdf-export-reason' : undefined}>{exportLabel}</button>
      </div>
      {exportUnavailable && <p id="preview-pdf-export-reason" className="honest-note">{exportUnavailable}</p>}
    </div>
  </section>
}
