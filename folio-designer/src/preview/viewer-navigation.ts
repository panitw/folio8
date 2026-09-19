// STORY 13.2 — THE VIEWER'S NAVIGATION ARITHMETIC, WITH NO DOM IN IT.
//
// Everything here is pure and total: it is handed numbers and returns numbers.
// The one reading of the browser — the scroll container's pixel box — is taken
// in `pdf-viewer.tsx`, under the canvas-authority exception that names its two
// property spellings, and arrives here as a plain `PreviewBox`. Keeping the
// arithmetic in its own module gives it a test that needs no DOM at all, and
// keeps `pdf-viewer.tsx` at the two value exports the lint gate counts.
//
// EVERY FUNCTION THAT CAN FAIL RETURNS `undefined` RATHER THAN A NUMBER. A fit
// over an unmeasurable box, a typed zoom of "abc" and a typed page of "0" are
// all the same shape of answer — "no change" — and the caller is then unable to
// hand a zero, a negative or a NaN scale to the rasterizer by accident.

export type PreviewFit = 'width' | 'page'
export type PreviewBox = Readonly<{ width: number; height: number }>

export const MIN_PREVIEW_SCALE = 0.5
export const MAX_PREVIEW_SCALE = 2
export const PREVIEW_ZOOM_STEP = 0.1
// The percentages the zoom control offers beside `Fit width` and `Fit page`.
// Typed as `readonly number[]` rather than as a literal tuple so a caller can
// ask whether an arbitrary scale is one of them.
export const PREVIEW_ZOOM_CHOICES: readonly number[] = [0.5, 0.75, 1, 1.5, 2]

const measurable = (box: PreviewBox) => Number.isFinite(box.width) && Number.isFinite(box.height) && box.width > 0 && box.height > 0
// Float noise is stripped at the sixth decimal wherever a scale is ARRIVED AT
// by repeated addition — `1 - 0.1 - 0.1` is 0.8000000000000001 and would keep
// accumulating. A fit scale is not arrived at that way: it is one division, so
// it is left exact. Rounding it would make the page miss the edge it was asked
// to meet by a fraction of a pixel per hundred.
const tidy = (value: number) => Math.round(value * 1e6) / 1e6

/** Clamps a scale into the viewer's zoom bounds, refusing anything unusable. */
export function clampPreviewScale(scale: number): number | undefined {
  if (!Number.isFinite(scale) || scale <= 0) return undefined
  return tidy(Math.min(MAX_PREVIEW_SCALE, Math.max(MIN_PREVIEW_SCALE, scale)))
}

/**
 * Resolves a fit choice against the page's intrinsic CSS size at scale 1 and
 * the container's usable box. `undefined` means "leave the scale exactly as it
 * is" — the container was not measurable, which is every jsdom render, every
 * hidden panel and every unmounted host.
 *
 * The result is deliberately NOT clamped to the zoom bounds. A fit is a
 * geometric answer to "how big must this page be to fit here", and clamping it
 * would silently stop it fitting; the bounds belong to the manual zoom, which
 * is the control an author drives by hand.
 */
export function fitPreviewScale(fit: PreviewFit, page: PreviewBox, container: PreviewBox): number | undefined {
  if (!measurable(page) || !measurable(container)) return undefined
  const byWidth = container.width / page.width
  const resolved = fit === 'width' ? byWidth : Math.min(byWidth, container.height / page.height)
  if (!Number.isFinite(resolved) || resolved <= 0) return undefined
  return resolved
}

/**
 * One press of `−` or `+`, taken from wherever the scale actually stands.
 *
 * A STEP NEVER MOVES THE SCALE THE OTHER WAY. `fitPreviewScale` above is
 * deliberately unclamped, and the viewer writes the resolved fit back into
 * `state.scale`, so the scale handed in here can legitimately sit OUTSIDE the
 * zoom bounds — at which point clamping alone inverts the control. Measured,
 * both directions: `fitPreviewScale('page', {816, 1056}, {900, 320})` resolves
 * to 0.30303030303030304, and clamping `0.20303…` gave 0.5, so `Zoom out PDF`
 * enlarged the page by 65%; `fitPreviewScale('width', {100, 100}, {900, 320})`
 * resolves to 9, and clamping `9.1` gave 2, so `Zoom in PDF` shrank it.
 *
 * So when the scale already sits outside the bounds on the side this step is
 * heading away from, the answer is "no change" — `undefined`, the same shape
 * every other refusal in this module returns, which `stepPreviewZoom` in App
 * already handles by doing nothing at all. A step heading back TOWARDS the
 * bounds still clamps exactly as it did: from 9, `−` gives 2; from 0.30303, `+`
 * gives 0.5. Still pure and still total.
 */
export function steppedPreviewScale(scale: number, steps: number): number | undefined {
  if (!Number.isFinite(scale) || !Number.isFinite(steps)) return undefined
  if (steps > 0 && scale > MAX_PREVIEW_SCALE) return undefined
  if (steps < 0 && scale < MIN_PREVIEW_SCALE) return undefined
  return clampPreviewScale(tidy(scale + steps * PREVIEW_ZOOM_STEP))
}

/**
 * A typed percentage. Out of range is clamped, because "250" is an unambiguous
 * request for the largest zoom there is; anything that is not a number at all
 * is refused, and the caller puts the current zoom back in the field.
 */
export function typedPreviewZoom(text: string): number | undefined {
  const trimmed = text.trim().replace(/%$/, '').trim()
  if (!/^\d+(?:\.\d+)?$/.test(trimmed)) return undefined
  return clampPreviewScale(Number(trimmed) / 100)
}

/**
 * A typed page number. Out of range is REFUSED rather than clamped: "35" on a
 * 34-page document is far more likely a typo than a request for the last page,
 * and a viewer that silently navigated somewhere else would hide it.
 */
export function typedPreviewPage(text: string, pages: number | undefined): number | undefined {
  const trimmed = text.trim()
  if (!/^\d+$/.test(trimmed)) return undefined
  const page = Number(trimmed)
  if (page < 1) return undefined
  if (pages !== undefined && page > pages) return undefined
  return page
}
