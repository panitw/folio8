// THE STARTUP DIALOG'S COPY, IN ONE PLACE (spec-startup-templates, story 3).
//
// The bundled assets (`generated/example-assets.ts`) carry ids and URLs only;
// what a person reads on each card lives here. Order is the dialog's order:
// Blank first, then the examples in the order the bundle lists them.
// `startup-examples.test.ts` holds this list and the bundle to each other, so an
// example cannot ship with no card, and a card cannot name an example that
// does not ship.

export type StartupChoice = Readonly<{
  id: string
  name: string
  description: string
  /** The example's sample JSON file name; `undefined` for Blank. */
  sample?: string
}>

export const BLANK_CHOICE_ID = 'blank'
export const DEFAULT_STARTUP_CHOICE_ID = BLANK_CHOICE_ID

export const startupChoices: ReadonlyArray<StartupChoice> = [
  { id: BLANK_CHOICE_ID, name: 'Blank', description: 'Empty A4 page' },
  { id: 'invoice', name: 'Invoice', description: 'Line items, totals, payment QR', sample: 'invoice.sample.json' },
  { id: 'bank-statement', name: 'Bank Statement', description: 'Paginated transactions', sample: 'bank-statement.sample.json' },
  { id: 'legal-contract', name: 'Legal Contract', description: 'Clauses, signature block', sample: 'legal-contract.sample.json' },
  { id: 'electricity-bill', name: 'Electricity Bill', description: 'Usage, charges, barcode', sample: 'electricity-bill.sample.json' },
]
