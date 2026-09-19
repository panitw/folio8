import { createHash, randomBytes } from 'node:crypto'
import { describe, expect, it } from 'vitest'
import { pdfDigest } from './pdf-digest'
import { PDF_FIXTURE_BYTES, PDF_FIXTURE_DIGEST, fixtureDigest } from '../test/pdf-fixture'

// STORY 13.3 / DW-270 — THE DIGEST THE BROWSER COMPUTES.
//
// ⚠ EVERY EXPECTATION HERE IS DERIVED BY NODE, NEVER BY THE FUNCTION UNDER
// TEST. `pdfDigest` uses `crypto.subtle`; `createHash` is a different
// implementation in a different runtime, so agreement between them is evidence
// rather than a tautology (D-11.2.2, D-11.2.8). A test that hashed with
// `pdfDigest` and compared against `pdfDigest` would pass over any consistent
// wrong answer, including a truncated one.
describe('the browser-side PDF digest', () => {
  it('agrees with an independently computed SHA-256 over the same bytes, in lowercase hex', async () => {
    const digest = await pdfDigest(PDF_FIXTURE_BYTES.slice().buffer)
    expect(digest).toBe(createHash('sha256').update(Uint8Array.from(PDF_FIXTURE_BYTES)).digest('hex'))
    expect(digest).toBe(PDF_FIXTURE_DIGEST)
    // The protocol admits a digest by this pattern; a value this function
    // produced must satisfy the same one, or an honest recomputation could
    // never equal an admitted reply.
    expect(digest).toMatch(/^[a-f0-9]{64}$/)
  })

  it('is length-preserving and byte-sensitive across a spread of real sizes', async () => {
    for (const size of [0, 1, 31, 32, 64, 4096]) {
      const bytes = randomBytes(size)
      const view = Uint8Array.from(bytes)
      const digest = await pdfDigest(view.slice().buffer)
      expect(digest, `size ${size}`).toBe(fixtureDigest(view))
      if (size === 0) continue
      // ONE FLIPPED BIT, WHICH IS THE WHOLE POINT OF INSTALLING ON THIS CHECK:
      // the four copies between the engine and the screen are byte-preserving,
      // and a digest that did not move on a single-bit change would notice
      // none of them going wrong.
      const mutated = Uint8Array.from(view)
      mutated[size - 1] = (mutated[size - 1]! ^ 1) & 0xff
      expect(await pdfDigest(mutated.slice().buffer), `size ${size}`).not.toBe(digest)
    }
  })

  it('hashes a buffer that has crossed a structured clone, as every real preview buffer has', async () => {
    // The bytes reaching `installPreview` were structured-cloned out of a
    // worker and copied twice more. `font-store.ts` records a MEASURED case
    // where a buffer that made that trip reports the right `byteLength` while
    // `instanceof ArrayBuffer` is false, because it was produced in another
    // realm; hashing a `Uint8Array` VIEW rather than the buffer is what keeps
    // that working, and this is the transport that produces it.
    const source = Uint8Array.from([37, 80, 68, 70, 45, 49, 46, 55])
    const cloned = structuredClone(source.slice().buffer)
    expect(cloned.byteLength).toBe(source.byteLength)
    expect(await pdfDigest(cloned)).toBe(fixtureDigest(source))
  })
})
