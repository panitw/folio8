/**
 * STORY 13.3 — THE EVIDENCE RAIL'S NON-COMPONENT HELPERS.
 *
 * They live in a `.ts` beside `evidence-rail.tsx` rather than inside it because
 * a `.tsx` that exports anything other than a component or a type earns an
 * `only-export-components` warning, and this repository's lint baseline is a
 * measured count rather than a rough one.
 *
 * Nothing here decides anything. Every function formats or joins a value the
 * engine or the viewer already committed; none derives a document fact, and none
 * measures the DOM.
 */

/**
 * THE BYTE-IDENTITY SENTENCE, WORD FOR WORD, AND WHAT IT DELIBERATELY DOES NOT
 * SAY.
 *
 * The mockup put "Matches native render" beside a check-circle. The tab cannot
 * compare itself to a native render and nothing in the browser does, so an
 * affirmation of a live comparison would be exactly the fabricated claim this
 * screen exists to refuse. What IS true is that
 * `.github/workflows/matrix.yml`'s `compare-render-hashes` job asserts
 * byte-identical output across four targets — the mockup named three — over 25
 * documents on every push, and that is what the sentence cites.
 *
 * It is WITHHELD ENTIRELY from a stand-in preview (D-13.4.1): a no-data digest
 * is not evidence of cross-target equality, which is why Story 13.4 labels it
 * `Stand-in local digest` in the first place.
 */
export const BYTE_IDENTITY_SENTENCE = 'Byte-identical across darwin/arm64, linux/amd64, linux/arm64 and js/wasm — proven by the build\'s cross-target matrix, not compared here.'

/**
 * THE TARGET THAT ACTUALLY RENDERED THESE BYTES. The designer loads exactly one
 * engine — `folio8.wasm`, in this tab — so this is a fact about the render that
 * happened rather than a configurable label.
 */
export const RENDER_TARGET = 'wasm · in browser'

/**
 * A byte count for a person, not for arithmetic.
 *
 * ⚠ THE UNIT IS CHOSEN AFTER ROUNDING, NOT BEFORE. Picking the unit from the
 * raw byte count and rounding afterwards prints `1024 KB` for anything in the
 * last kibibyte below a mebibyte — a number that has already outgrown the unit
 * printed beside it.
 */
export function formatRenderSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return ''
  if (bytes < 1024) return `${bytes} B`
  // One decimal, and the `.0` trimmed: `248 KB`, never `248.0 KB`.
  const round = (value: number) => Math.round(value * 10) / 10
  const kilobytes = round(bytes / 1024)
  const [value, unit] = kilobytes < 1024 ? [kilobytes, 'KB'] : [round(bytes / (1024 * 1024)), 'MB']
  return `${Number.isInteger(value) ? value : value.toFixed(1)} ${unit}`
}

/**
 * A ZERO IS AN ANSWER, NOT AN ABSENCE. A render that finished inside a
 * millisecond reports `0 ms`, and this must print it rather than treating it as
 * a missing measurement — the same reason neither Go struct marks the field
 * `omitempty`.
 */
export function formatElapsed(milliseconds: number): string {
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return ''
  if (milliseconds < 1000) return `${milliseconds} ms`
  const seconds = Math.round(milliseconds / 100) / 10
  return `${Number.isInteger(seconds) ? seconds : seconds.toFixed(1)} s`
}

/**
 * THE HASH IS SPLIT IN MARKUP, NOT LEFT TO THE STYLESHEET.
 *
 * A hash a person is asked to compare by eye must be the WHOLE hash — the
 * mockup's 32-character display is a mockup artifact — and it must break in a
 * place that does not move. A CSS-only wrap breaks wherever the rail happens to
 * be that day, which makes two screenshots of the same digest look different;
 * two fixed 32-character lines are comparable across machines, and are the same
 * two lines the browser witness can measure.
 *
 * Anything that is not a 64-character digest is returned as a single line
 * rather than cut at an arbitrary point.
 */
export function hashLines(digest: string): ReadonlyArray<string> {
  if (digest.length !== 64) return [digest]
  return [digest.slice(0, 32), digest.slice(32)]
}

export type ElementPlacement = Readonly<{ kind: string; band: string }>
type JoinableComponent = Readonly<{ id: string; type: string; band: string }>

/**
 * THE LOCAL JOIN, AND IT IS ONLY HONEST WHILE THE PREVIEW IS ADMITTED.
 *
 * `EngineDiagnostic` carries an `elementId` and nothing about what that element
 * IS. Kind and band are already on the canvas projection, so the card can name
 * them without a third engine surface — but the projection describes the CURRENT
 * revision, so the caller must supply it only when the displayed preview is
 * still the admitted one, exactly as `locateDiagnostic` already does. An element
 * that is not there is not guessed at: the join is simply omitted.
 */
export function placementFor(elementId: string, components: ReadonlyArray<JoinableComponent> | undefined): ElementPlacement | undefined {
  if (!elementId || !components) return undefined
  const found = components.find((component) => component.id === elementId)
  return found ? { kind: found.type, band: found.band } : undefined
}

/**
 * `dataPath · kind · band <name>`, and the first part is printed WITHOUT
 * claiming it is a data binding. The engine also emits structural paths
 * (`bands.content.e7` is in the presenter's own fixtures), so the rail renders
 * the path the engine sent and says nothing about what kind of path it is.
 */
export function diagnosticLocationText(dataPath: string, placement: ElementPlacement | undefined): string | undefined {
  const parts = [dataPath, ...(placement ? [placement.kind, `band ${placement.band}`] : [])].filter((part) => part.length > 0)
  return parts.length ? parts.join(' · ') : undefined
}

/**
 * THE DISMISSAL KEY, SPELT ONCE (Story 13.3 review, P2).
 *
 * The presenter uses it to decide which cards are still on screen; the rail's
 * header uses it to say how many of the render's warnings that is. Those two
 * numbers have to describe the SAME set — a header reading `warnings 2` over an
 * empty list is a count of something the reader cannot point at — and the only
 * way to guarantee that is for both to ask the same function.
 *
 * ⚠ THE STRING ITSELF IS UNCHANGED AND MUST STAY SO. It is index-prefixed and
 * pinned verbatim by `diagnostic-presenter.test.tsx`, because a key that moved
 * would silently un-dismiss every card an author had already dealt with.
 */
export function diagnosticDismissalKey(diagnostic: Readonly<{ severity: string; code: string; elementId: string; dataPath: string; message: string }>, index: number): string {
  return `${index}:${diagnostic.severity}:${diagnostic.code}:${diagnostic.elementId}:${diagnostic.dataPath}:${diagnostic.message}`
}
