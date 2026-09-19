// Sibling declaration for the vendored `renderable_view.js`.
//
// TOP-LEVEL `export declare`, NOT `declare module`. The `declare module 'x'`
// shape used by `src/preview/pdfjs-dist.d.ts` only resolves BARE package
// specifiers; a relative import of a sibling `.js` is matched by the compiler's
// extension-substitution rule (`./renderable_view.js` → `./renderable_view.d.ts`)
// instead, which needs an ordinary module declaration. `allowJs` is false and
// `strict` is on, so without this file the `.js` import is a hard TS7016.
//
// ⚠ A `.d.ts` IS A `.ts`, so this file sits in the AD-17 production corpus that
// `canvas-authority-contract.test.ts` scans. Nothing here may name a browser
// measurement — no `getBoundingClientRect`, no `client*`/`offset*`/`scroll*`
// member, no observer, no display pixel ratio. Only the surface folio8 actually
// drives is declared, which is what keeps that true by having very little in it.

export declare const RenderingStates: {
  readonly INITIAL: number
  readonly RUNNING: number
  readonly PAUSED: number
  readonly FINISHED: number
}

export declare class RenderableView {
  renderingId: string
  renderTask: { promise: Promise<void>; cancel(): void } | null
  resume: (() => void) | null
  get renderingState(): number
  set renderingState(state: number)
  draw(): Promise<void>
}
