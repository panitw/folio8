import { describe, expect, it } from 'vitest'
import { exampleAssets } from './generated/example-assets'

describe('exampleAssets', () => {
  it('lists the four examples in dialog order, each with its template, sample and thumbnail URLs', () => {
    expect(exampleAssets.map((example) => example.id)).toEqual(['invoice', 'bank-statement', 'legal-contract', 'electricity-bill'])
    for (const example of exampleAssets) {
      expect(example.template).toMatch(new RegExp(`${example.id}\\.[a-f0-9]{20}\\.folio`))
      expect(example.sample).toMatch(new RegExp(`${example.id}\\.sample\\.[a-f0-9]{20}\\.json`))
      expect(example.thumbnail).toMatch(new RegExp(`${example.id}\\.thumbnail\\.[a-f0-9]{20}\\.png`))
      expect(new Set([example.template, example.sample, example.thumbnail]).size).toBe(3)
    }
  })
})
