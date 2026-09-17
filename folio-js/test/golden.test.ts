import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { parseTemplate, render } from '../src/index.js'
import { repoFile, repoRoot, sha256, shippedFonts } from './helpers.js'

// Fixtures whose Go golden test renders with exactly fonts.Shipped(). The
// expected hash is read from the committed expected.json, never restated.
// justified-thai and shaped-text exercise Thai shaping; statement-5 passes
// params; colour-strokes covers colour.
const fixtures = ['colour-strokes', 'justified-thai', 'shaped-text', 'statement-5', 'alternating-rows']

function optional(path: string): Uint8Array | undefined {
  try {
    return repoFile(path)
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === 'ENOENT') return undefined
    throw error
  }
}

describe('golden corpus renders byte-identically', () => {
  for (const name of fixtures) {
    it(name, async () => {
      const dir = join('fixtures', name)
      const expected = JSON.parse(readFileSync(join(repoRoot, dir, 'expected.json'), 'utf8')) as { sha256: string }
      const tpl = await parseTemplate(repoFile(join(dir, 'input.folio')))
      const result = await render(tpl, optional(join(dir, 'data.json')) ?? '{}', optional(join(dir, 'params.json')), shippedFonts())
      expect(sha256(result.bytes)).toBe(expected.sha256)
      expect(result.diagnostics).toEqual([])
    })
  }
})
