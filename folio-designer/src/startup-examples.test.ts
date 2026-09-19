import { describe, expect, it } from 'vitest'
import { exampleAssets } from './generated/example-assets'
import { BLANK_CHOICE_ID, DEFAULT_STARTUP_CHOICE_ID, startupChoices } from './startup-examples'

describe('startup dialog copy', () => {
  it('names Blank and then every bundled example, in the bundle\'s order, and nothing else', () => {
    expect(startupChoices.map((choice) => choice.id)).toEqual([BLANK_CHOICE_ID, ...exampleAssets.map((example) => example.id)])
  })

  it('carries the names and one-line descriptions the spec fixes', () => {
    expect(startupChoices.map(({ name, description }) => [name, description])).toEqual([
      ['Blank', 'Empty A4 page'],
      ['Invoice', 'Line items, totals, payment QR'],
      ['Bank Statement', 'Paginated transactions'],
      ['Legal Contract', 'Clauses, signature block'],
      ['Electricity Bill', 'Usage, charges, barcode'],
    ])
  })

  it('names each example\'s sample by the file the bundle fingerprints, and gives Blank none', () => {
    for (const choice of startupChoices) {
      if (choice.id === BLANK_CHOICE_ID) { expect(choice.sample).toBeUndefined(); continue }
      expect(choice.sample).toBe(`${choice.id}.sample.json`)
      const asset = exampleAssets.find((example) => example.id === choice.id)
      expect(asset?.sample).toContain(`${choice.id}.sample.`)
    }
  })

  it('selects Blank by default', () => {
    expect(DEFAULT_STARTUP_CHOICE_ID).toBe(BLANK_CHOICE_ID)
    expect(startupChoices[0]?.id).toBe(DEFAULT_STARTUP_CHOICE_ID)
  })
})
