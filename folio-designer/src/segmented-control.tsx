import type { ReactNode } from 'react'

// STORY 14.7 — ONE ALIGNMENT CONTROL IN THE PRODUCT, WHICH MEANS ONE MODULE.
//
// The inspector has drawn a three-segment alignment control since Story 14.1
// while the table editor two panels away drew a `<select>` for the same
// concept. This module is the control itself, lifted out of `App.tsx` so both
// surfaces render literally the same code rather than two spellings of it.
//
// ⚠ IT LIVES HERE RATHER THAN IN `App.tsx` BECAUSE `App.tsx:33` IMPORTS
// `TableEditor`. Importing back would be a cycle, so the shared part had to
// leave both files. Nothing is duplicated: `App.tsx` and `TableEditor.tsx` both
// import from here.
//
// ⚠ A TABLE COLUMN TAKES THE THREE-VALUE ARRAY AND NEVER `justify`. The engine
// is explicit about this on both sides of the wire:
// `folio-go/internal/template/closedsets.go` declares `StyleAlignTokens` with
// four members and `ColumnAlignTokens` with THREE — a column's align is
// `left`/`center`/`right`, full stop. `justifySegment` below is exported
// separately, and it is widened onto `alignSegments` at exactly one call site
// (the inspector's TYPOGRAPHY row, for an all-text selection). Handing a column
// the four-segment array would offer a value the engine refuses; a table cell
// draws a justified value at its start edge, so it would also mean nothing.

export type SegmentSpec = Readonly<{ value: string; label: string; content: ReactNode }>

// The justify glyph is FOUR FLUSH RULES, drawn as an SVG path like its three
// siblings. It is never the CSS justify declaration: the browser must not be
// asked to justify anything, in production, unit or e2e sources, and
// canvas-authority-contract.test.ts bans the property/value pair outright —
// in comments too, which is why this sentence spells neither.
export type AlignVariant = 'left' | 'center' | 'right' | 'justify'
export const alignGlyphs: Readonly<Record<AlignVariant, string>> = { left: 'M2 4h12M2 8h8M2 12h11', center: 'M2 4h12M4 8h8M3 12h10', right: 'M2 4h12M6 8h8M3 12h11', justify: 'M2 3h12M2 7h12M2 11h12M2 15h12' }

export function AlignIcon({ variant }: { variant: AlignVariant }) {
  return <svg aria-hidden="true" className="segment-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2"><path d={alignGlyphs[variant]} /></svg>
}

export const alignSegments: ReadonlyArray<SegmentSpec> = [{ value: 'left', label: 'Align left', content: <AlignIcon variant="left" /> }, { value: 'center', label: 'Align center', content: <AlignIcon variant="center" /> }, { value: 'right', label: 'Align right', content: <AlignIcon variant="right" /> }]

// Offered only when every selected component is text. A table element's
// style.align cascades to its cells, which draw a justified value at their
// start edge — so justify means nothing there, and a control must not offer a
// value that is meaningless for the element type. A MIXED text+table
// selection gets the triple too: one command goes to every id in it.
export const justifySegment: SegmentSpec = { value: 'justify', label: 'Align justify', content: <AlignIcon variant="justify" /> }

// THE CONTROL IS PRESENTATIONAL AND OWNS NO COMMIT SEMANTICS, deliberately.
//
// The inspector's `SegmentedProperty` clears the property when the pressed
// segment is pressed again — the only way back to the value the document
// inherits, and the state the control already shows as "no segment pressed".
// A COLUMN'S ALIGN IS NOT CLEARABLE (`TableColumn.align` is always one of the
// three), so that semantic stays where it belongs, on the inspector side, and
// this module renders what it is told to render.
//
// `segmentProps` is how the table editor threads its roving-tabindex cell onto
// each segment: three segments are three reachable controls, so each one takes
// its own lattice position rather than the group taking one between them.
//
// `role` IS `group` UNLESS A CALLER HAS A REASON, and the Table Editor's HEADER
// ALIGN control has one: the spec forbids a new `role="group"` in that dialog,
// because `control-vocabulary-contract.test.tsx` pins the shrunk sweep's group
// count under its floor. `toolbar` keeps the set named for assistive
// technology without adding a counted group instance.
export function SegmentedControl({ label, segments, current, disabled, titleFor, onPick, trailing, segmentProps, role = 'group' }: {
  role?: 'group' | 'toolbar'
  label: string
  segments: ReadonlyArray<SegmentSpec>
  current?: string
  disabled?: boolean
  titleFor?: (segment: SegmentSpec) => string
  onPick: (value: string) => void
  trailing?: ReactNode
  segmentProps?: (index: number) => Record<string, unknown>
}) {
  return <div className="property-segmented" role={role} aria-label={label}>
    {segments.map((segment, index) => <button
      key={segment.value}
      type="button"
      className="property-segment"
      disabled={disabled}
      aria-pressed={current === segment.value}
      aria-label={segment.label}
      title={titleFor === undefined ? segment.label : titleFor(segment)}
      onClick={() => onPick(segment.value)}
      {...(segmentProps === undefined ? {} : segmentProps(index))}
    >{segment.content}</button>)}
    {trailing}
  </div>
}
