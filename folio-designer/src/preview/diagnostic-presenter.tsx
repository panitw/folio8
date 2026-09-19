import type { EngineDiagnostic, EngineError } from '../engine-protocol'
import { diagnosticDismissalKey, diagnosticLocationText, placementFor } from './evidence-rail-facts'

export type DiagnosticLocation = Readonly<{ elementId?: string; dataPath?: string }>
// The join's subject, supplied by the caller and only while the displayed
// preview is still the admitted one — the same guard `locateDiagnostic`
// already applies. `undefined` means "do not join", which is why an absent
// projection quietly yields the path alone rather than a guess.
export type DiagnosticComponents = ReadonlyArray<Readonly<{ id: string; type: string; band: string }>>

// STORY 13.3 — THE CARD NAMES THE ELEMENT'S KIND AND BAND, NOT ITS ID.
//
// `dataPath · kind · band <name>` reads as a place in the document; the raw
// `e7` the line used to lead with is an internal handle that says nothing to
// an author, and `Locate on canvas` is the control that actually resolves it.
// A PAGE NUMBER IS DELIBERATELY NOT HERE (owner, Q2, 2026-09-08): it is a
// property of layout, and computing layout in the browser is forbidden
// outright by the canvas-authority contract.
function diagnosticLocation(diagnostic: DiagnosticLocation, components?: DiagnosticComponents): string | undefined {
  return diagnosticLocationText(diagnostic.dataPath ?? '', placementFor(diagnostic.elementId ?? '', components))
}

// Presentation only: diagnostics arrive from the closed Go/wasm producer
// contract. This module deliberately has no code registry or template parser.
export function PreviewDiagnostics({ diagnostics, dismissed, onDismiss, onLocate, components }: { diagnostics: ReadonlyArray<EngineDiagnostic>; dismissed: ReadonlySet<string>; onDismiss: (key: string) => void; onLocate: (location: DiagnosticLocation) => void; components?: DiagnosticComponents }) {
  const visible = diagnostics.map((diagnostic, index) => ({ diagnostic, key: diagnosticDismissalKey(diagnostic, index) })).filter(({ key }) => !dismissed.has(key))
  if (!visible.length) return null
  // This text describes the admitted render generation, not the mutable local
  // dismissal view. Keeping it stable prevents every dismiss click from
  // re-announcing the same producer facts.
  const announcement = `${diagnostics.length} render ${diagnostics.length === 1 ? 'warning' : 'warnings'} available${diagnostics.length ? `: ${diagnostics.map((diagnostic) => diagnostic.code).join(', ')}` : ''}.`
  return <section className="diagnostic-list" aria-label="Render diagnostics"><p className="diagnostic-announcement" role="status" aria-live="polite" aria-atomic="true">{announcement}</p>{visible.map(({ diagnostic, key }) => {
    const location = diagnosticLocation(diagnostic, components)
    return <article className="diagnostic-card" key={key}><span className="diagnostic-triangle" aria-hidden="true">▲</span><div><p className="diagnostic-code">{diagnostic.severity} · {diagnostic.code}</p><p>{diagnostic.message}</p>{location && <code className="diagnostic-location">{location}</code>}{diagnostic.elementId && <button type="button" className="diagnostic-locate" onClick={() => onLocate(diagnostic)}>Locate on canvas</button>}</div><button type="button" className="diagnostic-dismiss" aria-label={`Dismiss ${diagnostic.code} diagnostic`} onClick={() => onDismiss(key)}>Dismiss</button></article>
  })}</section>
}

export function PreviewFailure({ error, onRetry, onReturn }: { error: EngineError; onRetry: () => void; onReturn: () => void }) {
  return <section className="preview-failure" aria-label="Local render failure" role="alert" aria-atomic="true"><span className="preview-failure-marker" aria-hidden="true">■</span><div className="preview-failure-facts"><p className="diagnostic-code">Render failure · {error.code}</p><p>Local PDF render failed: {error.message}</p>{error.elementId !== undefined && <code className="diagnostic-location">Element ID: {error.elementId}</code>}{error.dataPath !== undefined && <code className="diagnostic-location">Data path: {error.dataPath}</code>}</div><div className="preview-failure-actions"><button type="button" className="file-button" onClick={onRetry}>Retry preview</button><button type="button" className="file-button" onClick={onReturn}>Return to Design</button></div></section>
}
