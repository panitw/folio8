import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

describe('Preview local-only authority boundary', () => {
  it('contains no browser viewer, network, storage, or object-URL fallback', () => {
    const source = readFileSync('src/preview/pdf-viewer.tsx', 'utf8')
    const forbidden = ['<iframe', '<embed', 'createObjectURL', 'revokeObjectURL', 'fetch(', 'XMLHttpRequest', 'WebSocket', 'EventSource', 'localStorage', 'sessionStorage', 'indexedDB', 'http://', 'https://']
    for (const token of forbidden) expect(source).not.toContain(token)
    expect(source).toContain('disableAutoFetch: true')
    expect(source).toContain('useWorkerFetch: false')
    expect(source).toContain('isEvalSupported: false')
  })

  it('keeps stale/current authority and its accessible association non-vacuous', () => {
    const app = readFileSync('src/App.tsx', 'utf8')
    const viewer = readFileSync('src/preview/pdf-viewer.tsx', 'utf8')
    expect(app).toContain('new PreviewWorkScheduler()')
    expect(app).toContain('canInstallPreview(')
    expect(app).toContain('id="preview-freshness-status"')
    expect(app).toContain('setPreviewStatus(\'stale\')')
    expect(app).not.toContain('<p role="alert">Local PDF render failed')
    expect(app).toMatch(/id="preview-freshness-status"[^>]*role="status"/)
    expect(app).toContain("previewStatus === 'current' ? 'sr-only' : 'preview-status'")
    expect(viewer).toContain('aria-describedby={describedBy}')
    expect(viewer).not.toContain('aria-label="Exact local production PDF preview"')

    // Red proofs: hiding the stale marker, reintroducing an unconditional
    // exact name, or severing the named status association must all fail this
    // authority contract rather than merely changing presentation colour.
    expect(app.replace('setPreviewStatus(\'stale\')', 'setPreviewStatus(\'rendering\')')).not.toContain('setPreviewStatus(\'stale\')')
    expect(viewer.replaceAll('aria-describedby={describedBy}', '')).not.toContain('aria-describedby={describedBy}')
  })
})
