import { describe, expect, it } from 'vitest'

import { MAX_CANVAS_PROPERTY_STRING } from './engine-protocol'
import { canvasFragmentFallbackStack, isShippedFaceName, shippedFaceFamily } from './shipped-face-family'

// THE SHIPPED HALF OF THE PER-FRAGMENT FACE DERIVATION (Story 8.4e).
//
// The engine tells the canvas which face it measured each fragment with. For a
// face the document carries that is an ASSET KEY (embedded-face-family.ts);
// for a face the build ships it is the engine's own `FontSet` NAME, and this
// module turns that name into the CSS `font-family` value the fragment asks
// for. Everything below is a claim about a string: nothing here loads a font
// and nothing here measures anything.

/** The family sequence a CSS font-family value names, quotes removed. */
function familiesIn(value: string): ReadonlyArray<string> {
  return value.split(',').map((entry) => entry.trim().replace(/^['"]|['"]$/g, ''))
}

describe('a shipped face name becomes the family the fragment asks for', () => {
  it('puts the attributed face FIRST, which is the whole of what it exists to do', () => {
    // A CSS stack is a first-match-wins search PER CODEPOINT, so position is
    // the entire mechanism: the three shipped faces' cmaps overlap (339 / 529
    // / 230 codepoints pairwise, measured) and all three cover `A` and `5`. A
    // value that merely CONTAINED the attributed face would let a document
    // whose chain is ["Noto Sans Thai"] have its Latin rasterized by Noto
    // Sans, which is the defect this story closes.
    for (const face of ['Noto Sans', 'Noto Sans Thai', 'Noto Sans SC']) {
      expect(familiesIn(shippedFaceFamily(face) as string)[0], face).toBe(face)
    }
  })

  it('carries the declared stack as its tail, and names each family exactly once', () => {
    // An inline declaration REPLACES the stylesheet rule rather than extending
    // it, so without a tail a codepoint the attributed face does not cover
    // would fall to the browser's default instead of to the other two shipped
    // faces. The attributed face is moved to the front rather than repeated.
    const value = shippedFaceFamily('Noto Sans Thai') as string
    expect(familiesIn(value)).toEqual(['Noto Sans Thai', 'Noto Sans', 'Noto Sans SC', 'sans-serif'])
    for (const entry of canvasFragmentFallbackStack) {
      expect(familiesIn(value), entry).toContain(entry.replace(/^'|'$/g, ''))
    }
    // The generic keyword is a last resort and stays last, exactly as it does
    // in `.canvas-text-fragment`'s own rule.
    expect(familiesIn(value)[familiesIn(value).length - 1]).toBe('sans-serif')
  })

  it('names a face the tail does not already carry without dropping any of it', () => {
    // `fonts.Shipped()` is the engine's to change, and a fourth face would
    // reach here before the stylesheet's tail knew about it. It must be named
    // first and the whole tail must survive behind it.
    expect(familiesIn(shippedFaceFamily('Noto Serif') as string)).toEqual(['Noto Serif', 'Noto Sans', 'Noto Sans Thai', 'Noto Sans SC', 'sans-serif'])
  })

  it('declines a name that is not usable as a CSS family rather than interpolating it', () => {
    // THE INJECTION SHAPES. This value is written into an inline `font-family`
    // declaration, so a name carrying a quote, a semicolon, a comma, a brace
    // or a newline would not be a family — it would be extra CSS. The engine
    // can only put a FontSet key here today; that is the engine's guarantee,
    // not the browser's, and this is where the browser makes its own.
    for (const hostile of [
      "Noto', sans-serif; background: url(evil)",
      'Noto"',
      'Noto\\Sans',
      'Noto Sans, monospace',
      'Noto Sans; color: red',
      'Noto Sans } .x { color: red',
      'Noto\nSans',
      'Noto Sans<script>',
      ' Noto Sans',
      'Noto Sans ',
      '1Noto',
      '-Noto',
      '',
      undefined,
    ]) {
      expect(shippedFaceFamily(hostile), JSON.stringify(hostile)).toBeUndefined()
      expect(isShippedFaceName(hostile), JSON.stringify(hostile)).toBe(false)
    }
  })

  it('admits the shapes a face name legitimately takes', () => {
    for (const name of ['Noto Sans', 'Noto Sans Thai', 'Noto Sans SC', 'Roboto-Regular', 'IBM Plex Sans Thai', 'Arial', 'Noto Sans 2']) {
      expect(isShippedFaceName(name), name).toBe(true)
      expect(familiesIn(shippedFaceFamily(name) as string)[0], name).toBe(name)
    }
  })

  it('applies the same bound the projection applies to a chain entry\'s face', () => {
    // 512, reused rather than re-numbered: it is already what
    // engine-protocol.ts admits for a chain entry's `face` and what
    // page_setup.go applies to every string it puts on the wire.
    const atLimit = 'N'.repeat(MAX_CANVAS_PROPERTY_STRING)
    expect(isShippedFaceName(atLimit)).toBe(true)
    expect(isShippedFaceName(`${atLimit}N`)).toBe(false)
    expect(shippedFaceFamily(`${atLimit}N`)).toBeUndefined()
  })

  it('answers the predicate and the derivation from ONE rule', () => {
    // Two rules that merely agree today are two rules. The predicate is
    // defined as the derivation's own answer, and this is the assertion that
    // keeps it that way.
    for (const candidate of ['Noto Sans', "Noto', sans-serif", '', 'Roboto-Regular', undefined]) {
      expect(isShippedFaceName(candidate), JSON.stringify(candidate)).toBe(shippedFaceFamily(candidate) !== undefined)
    }
  })
})
