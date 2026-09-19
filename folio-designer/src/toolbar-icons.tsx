// The document bar's file actions and the canvas toolbar are drawn as glyphs,
// by owner ruling after Story 14.1 (which had spelled the document bar as
// words). Every glyph is a stroked SVG on the 16px grid, written inline so the
// offline release gains no emitted asset (D-14.0.1). The picture is decorative:
// each control carries its accessible name in `aria-label` and its hover guide
// — the name plus any shortcut — in `data-tip`, which `.tool-button` paints.
export type ToolGlyph = 'open' | 'save' | 'save-as' | 'blank' | 'undo' | 'redo' | 'zoom-out' | 'zoom-in' | 'grid' | 'snap' | 'duplicate' | 'delete' | 'add-page' | 'delete-page' | 'nudge' | 'docs'

const toolGlyphs: Readonly<Record<ToolGlyph, string>> = {
  open: 'M2 4.5h4l1.5 1.5H14v6.5H2z',
  save: 'M3 2.5h8l2.5 2.5v8.5H2.5v-11z M5.5 2.5v3h5v-3 M5 13.5v-4h6v4',
  'save-as': 'M8 13.5H2.5v-11h7l2 2V7 M5 2.5v2.5h4V2.5 M9 14l.6-2.1 3.4-3.4 1.5 1.5-3.4 3.4z',
  blank: 'M4 2h5.5L12 4.5V14H4z M9.5 2v2.5H12',
  undo: 'M5.5 3.5l-3 3 3 3 M2.5 6.5h7a3.5 3.5 0 0 1 0 7H7',
  redo: 'M10.5 3.5l3 3-3 3 M13.5 6.5h-7a3.5 3.5 0 0 0 0 7H9',
  'zoom-out': 'M7 2.5a4.5 4.5 0 1 0 0 9a4.5 4.5 0 1 0 0-9z M10.2 10.2L14 14 M5 7h4',
  'zoom-in': 'M7 2.5a4.5 4.5 0 1 0 0 9a4.5 4.5 0 1 0 0-9z M10.2 10.2L14 14 M5 7h4 M7 5v4',
  grid: 'M2.5 2.5h11v11h-11z M2.5 6.2h11 M2.5 9.8h11 M6.2 2.5v11 M9.8 2.5v11',
  snap: 'M3.5 2.5v5a4.5 4.5 0 0 0 9 0v-5h-3v5a1.5 1.5 0 0 1-3 0v-5z M3.5 5h3 M9.5 5h3',
  duplicate: 'M5.5 5.5h8v8h-8z M10.5 5.5v-3h-8v8h3',
  delete: 'M2.5 4h11 M6 4V2.5h4V4 M4 4l.7 9.5h6.6L12 4 M6.8 6.5V11 M9.2 6.5V11',
  // SPEC-multi-pages story 2 (D-2.3): a page with a plus, and a page with a
  // minus — the page outline sets them apart from Duplicate and Delete.
  'add-page': 'M3.5 1.5h6l3 3v10h-9z M9.5 1.5v3h3 M8 7v5 M5.5 9.5h5',
  'delete-page': 'M3.5 1.5h6l3 3v10h-9z M9.5 1.5v3h3 M5.5 9.5h5',
  nudge: 'M8 2v12 M2 8h12 M6.5 3.5L8 2l1.5 1.5 M6.5 12.5L8 14l1.5-1.5 M3.5 6.5L2 8l1.5 1.5 M12.5 6.5L14 8l-1.5 1.5',
  // The document bar's documentation link: an open book, two facing pages.
  docs: 'M8 4.5C6.5 3.3 4.5 3 2.5 3.5v9c2-.5 4-.2 5.5 1 M8 4.5c1.5-1.2 3.5-1.5 5.5-1v9c-2-.5-4-.2-5.5 1z M8 4.5v9',
}

export function ToolIcon({ glyph }: { glyph: ToolGlyph }) {
  return <svg aria-hidden="true" className="tool-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round"><path d={toolGlyphs[glyph]} /></svg>
}
