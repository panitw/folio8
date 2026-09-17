import type { Diagnostic } from './types.js'

/**
 * Thrown when the engine returns a Go `*RenderError`. `message` is Go's error
 * text and `diagnostic` carries the error's code, element and data path.
 */
export class FolioRenderError extends Error {
  readonly diagnostic: Diagnostic

  constructor(diagnostic: Diagnostic) {
    super(diagnostic.message)
    this.name = 'FolioRenderError'
    this.diagnostic = diagnostic
  }
}
