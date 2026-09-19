import { fireEvent, screen, within } from '@testing-library/react'

// THE DOCUMENT BAR'S NEW… THEN START BLANK, SHARED (spec-startup-templates,
// story 4). The bar's Start blank became New…, which opens the "New template"
// dialog; Blank there replaces the open document. On a document with real
// edits New… warns first, so the warning's Discard is pressed whenever it
// appears — every caller of this helper means "replace the document with the
// starter".
export function startBlankFromNew(): void {
  fireEvent.click(screen.getByRole('button', { name: 'New…' }))
  const warning = screen.queryByRole('dialog', { name: 'Discard unsaved changes?' })
  if (warning) fireEvent.click(within(warning).getByRole('button', { name: 'Discard' }))
  const dialog = screen.getByRole('dialog', { name: 'New template' })
  fireEvent.click(within(dialog).getByRole('button', { name: 'Start blank' }))
}
