export const ENGINE_PROTOCOL_VERSION = 1 as const

export type EngineOperation = 'initialize' | 'load' | 'snapshot' | 'parameter-references' | 'stand-in-data' | 'group-move-preview' | 'table-columns' | 'validate' | 'serialize' | 'command' | 'undo' | 'redo' | 'identity' | 'render' | 'asset'

export const MAX_ENGINE_REQUEST_ID_LENGTH = 128
export const MAX_ENGINE_PAYLOAD_BYTES = 8 * 1024 * 1024
export const MAX_ENGINE_RENDER_PDF_BYTES = 32 * 1024 * 1024
export const MAX_ENGINE_DIAGNOSTICS = 256
export const MAX_ENGINE_ELEMENT_ID_LENGTH = 128
export const MAX_ENGINE_DATA_PATH_LENGTH = 256
export const MAX_ENGINE_BINDING_LENGTH = 256
export const MAX_ENGINE_PARAMETER_REFERENCES = 128
export const MAX_ENGINE_PARAMETER_NAME_LENGTH = 128
// The same bound Go projects the document's declared font chains under.
export const MAX_ENGINE_FONT_FAMILIES = 256
// And the bound ONE chain's entry list is projected under. Story 8.1 put the
// entries themselves on the wire, so the per-chain array needs the same
// treatment the chain list already had.
export const MAX_ENGINE_FONT_CHAIN_ENTRIES = 64
// STORY 14.7b — HOW MANY UNDO ENTRIES THE ENGINE KEEPS, AND IT IS A MIRROR OF A
// GO CONSTANT: the `historyLimit` declared in `folio8-go/internal/wasm/engine.go`. It is tied to
// that declaration in `engine-bounds-mirror.test.ts`, because it is spelled on
// both sides of the channel with nothing but that test between them.
//
// THE CITATION IS A FILE AND A NAME, NEVER A LINE. A line number here is
// unverified by anything and rots on the first insertion above it, and the tie
// asserts the SPELLING of the declaration rather than its position — so the
// name is what a reader can actually follow.
//
// ⚠ THE ENGINE ENFORCES IT AS A RING BUFFER THAT SILENTLY EVICTS THE OLDEST
// ENTRY, NOT AS A REFUSAL. `appendBounded` shifts the history down and drops
// entry zero when the stack is full; no error is returned and nothing on the
// wire says it happened. So an over-run is invisible, and a compensating
// sequence longer than this number would land part-way and claim success —
// which is why the table editor's Cancel DISABLES itself above this count
// rather than trying.
//
// NO TYPE, GUARD OR VALIDATOR CHANGE COMES WITH IT: the edit count this bounds
// is the application's own integer and never crosses the worker boundary.
export const MAX_ENGINE_HISTORY_ENTRIES = 100
// A CHANNEL BACKSTOP, NOT A MIRROR. Go declares no maximum number of content
// windows — internal/layout bounds the count only by the column-item count —
// so this number is deliberately NOT in the pair list below: there is nothing
// on the Go side for it to drift against. It exists for the reason
// MAX_ENGINE_DIAGNOSTICS does, to keep an absurd array from being iterated,
// and it is set orders of magnitude above anything the projection produces
// because the cost of it biting is severe and silent: a rejected field
// discards the WHOLE snapshot and blanks the canvas. Epic 7's narrative
// target is forty pages; the canvas's own sheet budget is 120.
export const MAX_ENGINE_CONTENT_WINDOWS = 100_000

// ---------------------------------------------------------------------------
// THE CANVAS PROJECTION BOUNDS, MIRRORED FROM folio8-go/page_setup.go.
//
// There is no shared source and no codegen: these are hand-copied, which is
// the drift pattern in pure form — the Go side can be raised and this side
// will silently keep rejecting, blanking the projection with no error anyone
// can attribute (D-7.4.5). They are hoisted out of the validators and named
// after their Go counterparts so `engine-bounds-mirror.test.ts` can read both
// files and assert the pairs are equal. Change one, change the other, in the
// same commit.
//
// A UNIT MISMATCH IS BUILT INTO THE TWO STRING BOUNDS, and it is recorded
// rather than "fixed": Go counts BYTES (`len()`), these count UTF-16 CODE
// UNITS (`.length`). For non-ASCII this side is the more permissive of the
// pair, so the Go side refuses first and nothing unrepresentable arrives.
// The tie assertion compares LITERALS, not quantities, and says so.
//
// maxCanvasBodyText — the body-text channel backstop (bytes/code units).
export const MAX_CANVAS_BODY_TEXT = 1048576
// maxCanvasBodyTextLines — 40 pages × 48 lines per A4 page at 11pt.
export const MAX_CANVAS_BODY_TEXT_LINES = 1920
// maxCanvasBodyTextFragments — CUMULATIVE across the whole component, which
// is the quantity counted below. Go's own maxCanvasTextFragments bounds one
// LINE and is deliberately NOT mirrored here; the two are different
// quantities, and pairing them would be a false tie.
export const MAX_CANVAS_BODY_TEXT_FRAGMENTS = 65536
// maxCanvasPropertyString — identifiers, colours and expressions only. Body
// text no longer shares it on either side of the channel (DW-25).
export const MAX_CANVAS_PROPERTY_STRING = 512
// A column label's bound is CODE POINTS, never UTF-16 units (SPEC-table-rules
// §4). Go counts runes (component_commands.go's updateTableColumn); counting
// `.length` here would refuse a label of 200 emoji the engine accepted, and a
// refused projection means the Table Editor never opens.
export const MAX_TABLE_COLUMN_HEADER_CODE_POINTS = 256
// A canvas table column's `labelLines` are the ENGINE's packed header label
// lines — wrapped and split on line feeds in Go, line feeds removed — so the
// canvas paints one element per line and the browser makes no break decision
// (AD-17 condition 2). Go caps the list at maxCanvasTableLabelLines and each
// line at maxCanvasTableLabelLineLength — label TEXT, so not the identifier
// bound MAX_CANVAS_PROPERTY_STRING. A Go test parses these two lines from this
// file and pins them against Go's constants: keep each `export const NAME =
// <integer>` on one line.
export const MAX_CANVAS_TABLE_LABEL_LINES = 256
export const MAX_CANVAS_TABLE_LABEL_LINE_LENGTH = 1024
// THE FIFTH HAND-COPIED CROSS-LANGUAGE BOUND, and the only one that does not
// come from `page_setup.go`: `template.MinLineSpacingThousandths` and
// `template.MaxLineSpacingThousandths` (folio8-go/internal/template/
// linespacing.go), which Story 7.4 projects across the channel for the first
// time. The Go comment there calls the maximum "A STATED SANITY CEILING, NOT
// A DERIVED SAFETY BOUND" — i.e. a number somebody will one day adjust — and
// a raised ceiling with these literals left behind would make `parseInbound`
// drop every snapshot of such a document silently, with no canvas and no
// error. So they are named here and tied to the Go declarations by
// engine-bounds-mirror.test.ts alongside the other four.
export const MIN_LINE_SPACING_THOUSANDTHS = 1
export const MAX_LINE_SPACING_THOUSANDTHS = 1000000

// THE FIFTH MIRROR, and the first one that is a PREDICATE rather than a
// number: which bands cap a component vertically.
//
// DW-25 closed the four size caps above. Band containment is a different
// invariant that merely happens to live in the same file, and an audit closes
// only what it measured — so the standing obligation is widened here from
// "the size caps move together" to: ANY invariant duplicated across the
// Go/TypeScript boundary moves in ONE commit, with a test that reads both
// sides. `folio8-go/component_commands.go` declares this same list under this
// same name and `engine-bounds-mirror.test.ts` reads both files.
//
// The content band is absent by MEANING. A page header and a page footer
// repeat on every page, so each is exactly one page tall; the content band is
// a COLUMN that Go's internal/layout slices into page-height windows, so a
// component below the foot of page one is on page two, not outside the
// document. What a stale copy of this list costs is not a hidden component:
// `isCanvas` returning false makes `parseInbound` return undefined, which
// terminates the worker, rejects every in-flight request and leaves the
// canvas blank — with no element id and no attributable error.
export const BANDS_CAPPING_VERTICALLY = ['pageHeader', 'pageFooter']

// AD-12's CLOSED LOCALE SET, ONCE — the same discipline, for the same reason,
// on a different invariant.
//
// Go spells each of these four tags EXACTLY ONCE, as the right-hand side of a
// named constant in `folio8-go/internal/template/locale.go`, and builds both
// `LocaleTags` and `closedLocales` from those constants so the set cannot drift
// by a spelling mistake. This is the browser's single spelling of the same set,
// tied to Go's by `engine-bounds-mirror.test.ts` on the same idiom the band
// list uses — it resolves Go's named constants before comparing, so
// `[]string{LocaleEN, LocaleTH, …}` and `['en', 'th', …]` are compared as the
// same CLAIM rather than as the same text.
//
// EVERYTHING ON THIS SIDE READS THIS ARRAY: the isCanvas guard's typed clause,
// the panel's <option> list, and the command factory's parameter type. A tag
// written out anywhere else would be a fourth spelling standing outside that
// census, which is the only place a stale copy can hide — and the cost of a
// stale copy here is the cost of every other one: isCanvas returns false,
// parseInbound returns undefined, the worker is terminated and the canvas is
// permanently blank with nothing to attribute it to.
//
// A FIFTH TAG IS NOT ADDED HERE. Widening the set is a MAJOR change under
// folio-format.md's MINOR-increment rule — every existing library validates it as a load error —
// so it is Go's decision and an owner's, and it reaches this file through the
// mirror rather than by an edit that starts here.
export const LOCALE_TAGS = ['en', 'th', 'zh-Hans', 'ja'] as const
export type LocaleTag = (typeof LOCALE_TAGS)[number]

// THE SAME LIST, ONCE AS A TYPE AND ONCE AS AN ARRAY A CALLER MAY ITERATE —
// and neither of them is a second copy of it.
//
// Story 12.1 first shipped `settableBands` in App.tsx and `SettableBand` in
// band-height-command.ts, each spelling the two names out again. That made four
// and five copies of a list whose whole safety property is that
// engine-bounds-mirror.test.ts reads it on BOTH sides of the Go/TypeScript
// boundary and refuses to let it drift: two of the five were outside that
// census, which is the only place a stale copy can hide.
//
// CAPPING_BANDS is the SAME ARRAY OBJECT, narrowed to the element type; the
// union is taken out of the projection's own band-name union, which
// canvas_projection_wire_test.go pins against Go, minus the one band that has
// no height to cap with. Both are tied back to Go's list by name in
// engine-bounds-mirror.test.ts — the union through a Record whose keys must be
// exactly the union's, so a member gained or lost on either side is a
// compile-time error and a red test rather than a silent widening.
export type CappingBand = Exclude<CanvasProjection['bands'][number]['name'], 'content'>
export const CAPPING_BANDS = BANDS_CAPPING_VERTICALLY as ReadonlyArray<CappingBand>

// STORY 12.5: THE ONE NUMBER IN THE CONTENT-WINDOW CEILING THAT IS NOT
// PROJECTED.
//
// Go's `bandContentWindowCeiling(other, innerH)` is `innerH - other - 1`: the
// tallest a capping band may be beside a sibling of `other` in a printable
// column of `innerH`, one millipoint short of the whole column because the
// content region must be STRICTLY positive and a geom.Length is an integer
// count of millipoints.
//
// Every input to that expression is already projected — `innerH` is the three
// band heights summed, which `isCanvas`'s contiguity invariant makes exact, and
// `other` is one of them. The `- 1` is not. It is a property of the ENGINE's
// rule, not of this document, so it is mirrored here and tied to its Go
// declaration by `engine-bounds-mirror.test.ts` rather than written into the
// consumer as a bare literal.
//
// IT IS MIRRORED AT ALL ONLY BECAUSE A GESTURE CONSUMES IT (DW-36's standing
// condition, applied by 12.5's R1). The band-height PANEL still holds no bound:
// see band-height-command.ts's header for why a typed field and a pointer
// gesture answer 17.4 differently.
export const BAND_CONTENT_WINDOW_MARGIN = 1

// STORY 14.4: WHICH COMPONENT KINDS MAY RECEIVE A SCALAR BINDING — ONE
// SPELLING, TWO JUDGEMENTS.
//
// Go's authority is `bindComponentScalar`'s single line
// (`component_commands.go`): `if element.Type != template.ElementText` → *"only
// text components can receive a scalar binding"*. It reads `element.Type` and
// NOTHING ELSE — no band, no geometry, no sibling, no document — so it is a
// property of the FIELD rather than of the open template, which is what makes
// it mirrorable at all.
//
// THIS STORY DID NOT CREATE THE COPY; IT REGISTERED ONE THAT WAS ALREADY
// SHIPPED. `isCanvas` below has re-derived this rule inline since it was
// written, untied, and in the harshest failure mode on this boundary: a
// one-sided Go edit makes the guard return false, `parseInbound` return
// undefined, and the canvas go permanently blank. The choice was never "one
// copy or two" — it was "two copies untied, or one constant tied". This is the
// constant; `engine-bounds-mirror.test.ts`'s `scalar binding legality mirror`
// is the tie, and it asserts every consumer actually READS this array rather
// than re-spelling `=== 'text'` for itself.
//
// ⚠ THE RULE IS THE COMPONENT-TYPE GATE AND NOTHING ELSE. Whether a picked
// PATH yields a scalar at render time is runtime data decided by
// `internal/bind`, and D-6.2.1 deliberately keeps sample runtime kind out of
// command legality — Go accepts `{{items}}` for an empty collection and reports
// the mismatch at render. A consumer that widened this into "is this path
// bindable" would be a second, drifting copy of the binder.
//
// "Binding" is overloaded, and the name is deliberate: a TABLE legally takes a
// binding through `configureTableBinding` / `updateTableColumnBinding`. What
// this array closes is SCALAR binding, and nothing else.
export type CanvasComponentType = CanvasProjection['components'][number]['type']
export const SCALAR_BINDING_COMPONENT_TYPES: ReadonlyArray<CanvasComponentType> = ['text', 'barcode', 'qrcode']

// STORY 14.9 — ONE TABLE COLUMN AS THE CANVAS PROJECTION CARRIES IT, derived
// from the projection type rather than restated, so the painter and the guard
// can never name a different shape than the one this file admits.
//
// It is DELIBERATELY NOT `TableColumns`' column. That one is the TABLE EDITOR's
// projection — ten members, a validating producer that hard-errors on
// `width <= 0`, on more than 128 columns and on a bind that fails
// `rootCollectionPath` — and the canvas tolerates every one of those. Sharing a
// type between the two would invite sharing the gate, and the gate is what
// would blank a document that paints today.
//
// TWO RESOLVED ALIGNMENTS, NOT ONE (Story 14.9 / R2). The engine resolves a
// header cell's alignment through `resolveHeaderStyle` (headerStyle.align, then
// style.align, then "left") and a data cell's through `resolveBodyStyle`
// (style.align, then "left"), and the column's own `align` wins over either. A
// canvas that drew both rows from one value would be wrong on every table whose
// `headerStyle.align` differs from its `style.align` — authorable from the
// shipped UI since Story 14.8.
export type CanvasTableColumn = NonNullable<CanvasProjection['components'][number]['columns']>[number]

export type EngineError = Readonly<{
  code: string
  message: string
  elementId?: string
  dataPath?: string
}>

export type RenderPayload = Readonly<{ template: ArrayBuffer; data: ArrayBuffer; params: ArrayBuffer }>
export type IdentityPayload = Readonly<{ data: ArrayBuffer; params: ArrayBuffer }>
export type EngineDiagnostic = Readonly<{ severity: 'warning'; code: string; elementId: string; dataPath: string; message: string }>
// STORY 13.3 — `elapsedMs` AND `version` RIDE THE RENDER-ONLY PAIRED ARM.
//
// An `identity` reply carries only `{ revision, identity }`; a `render` reply
// carries all four of the optional members or none of them. Admission below is
// therefore all-or-nothing across the four, never member-by-member, so a reply
// that lost one field on the way is refused rather than displayed with a gap.
export type PreviewEvidence = Readonly<{ revision: number; identity: string; pdfSha256?: string; diagnostics?: ReadonlyArray<EngineDiagnostic>; elapsedMs?: number; version?: string }>
export type ParameterReferences = Readonly<{ revision: number; names: ReadonlyArray<string> }>
export type TableColumn = Readonly<{ id: string; header: string; width: number; proportion: string; align: 'left' | 'center' | 'right'; headerAlign: '' | 'left' | 'center' | 'right'; headerAlignResolved: 'left' | 'center' | 'right'; binding: string; rowField: string; rowFieldEditable: boolean; footer: '' | 'sum' | 'avg' | 'count'; footerOf: string; footerFormat: string }>
// STORY 12.3 — the table's own header and row properties, beside its columns.
//
// TWO MEMBERS PER HEADER-STYLE FIELD, and the pair is the whole point. The bare
// name (`headerAlign`) is what the DOCUMENT DECLARES — '' or 0 when the key is
// absent — so a control can tell set from unset and offer to clear back to
// absent. The `…Resolved` twin is what the document WILL USE, which for an
// absent field is the table's own `style.<field>` and then that field's
// documented default.
//
// THE RESOLVED HALF IS THE ENGINE'S ANSWER AND IS NEVER RECOMPUTED HERE. Go's
// resolveHeaderStyle is the one cascade in this program; it runs at the
// projection's construction site and its result travels on the wire. Handing
// the browser the committed field plus the table's own style and letting it
// choose IS implementing the cascade in the browser — forbidden by AD-15 and
// AD-17 and by this story's own AC2 and AC3.
//
// `headerHeight` AND `altRowBackground` CARRY ONE MEMBER EACH. Neither has a
// cascade to resolve through — the first is required so it is never absent, the
// second is a flat override with no fallback level of its own — so committed IS
// resolved and a second member would be ceremony: a duplicate of the committed
// value, carried on every projection, that a later reader has to keep agreeing
// with itself.
//
// STORY 11.3 / DW-240 ADDS THE NINTH AND TENTH PAIRS. `headerStyle.bold` and
// `headerStyle.italic` have cascaded through `resolveHeaderStyle` since Story
// 11.2 and the engine's `tableHeaderStyleFields` has carried both since then;
// only the read-back was missing, which is a weight an author could write and
// could not see. For a BOOLEAN, `false` is this projection's spelling of
// absence — committed-absent and committed-`false` are the same wire value —
// and `TableColumnsProjection`'s own comment discloses that limit.
export type TableHeaderStyle = Readonly<{
  headerFontFamily: string; headerFontFamilyResolved: string
  headerFontSize: number; headerFontSizeResolved: number
  headerLineSpacing: number; headerLineSpacingResolved: number
  headerBackground: string; headerBackgroundResolved: string
  headerColor: string; headerColorResolved: string
  headerValign: string; headerValignResolved: string
  headerAlign: string; headerAlignResolved: string
  headerBold: boolean; headerBoldResolved: boolean
  headerItalic: boolean; headerItalicResolved: boolean
  // STORY 14.8 ADDS THE TENTH, ELEVENTH AND TWELFTH PAIRS, AND THE DOTS IN
  // THEIR NAMES ARE NOT A STYLE CHOICE. The engine authors the header border one
  // attribute at a time through the field names `border.width`, `border.color`
  // and `border.edges` — dotted so that the refusal it locates,
  // `table.headerStyle.border.width`, is a path the document actually has — and
  // `table_header_style_test.go` derives these key names from those field names
  // by string transformation. The dot therefore arrives here from the command
  // spelling, and it is why these three pairs are quoted properties.
  //
  // `headerBorder.edgesResolved` IS THE ONE MEMBER THAT SAYS "NOTHING IS
  // PAINTED", by being empty — which happens both when no border reaches the
  // header row and when a declared edge list names no side.
  //
  // ⚠ THE WIDTH PAIR IS A PAIR OF STRINGS, AND IT IS THE ONLY LENGTH ON THIS
  // PROJECTION THAT IS. The units are UNCHANGED — integer thousandths of a
  // point, exactly like `headerHeight` and `headerFontSize`, so `authored()`
  // renders them the same way; what changes is only how ABSENCE is spelled.
  // `''` is absent, `'0'` is a declared zero, `'500'` is a declared half point.
  //
  // The reason is the file door's, not this file's. `parse_bands.go` says of a
  // border width: "ZERO IS VALID and stays accepted: it is the thinnest device
  // line PDF can draw, not an absent border." So `0` is a legal AUTHORED value
  // here in a way it is not for a header height — a zero header height is not a
  // meaningful declaration, a zero border width is — and a number whose absence
  // is spelled `0` cannot tell the two apart. It did not: a header border
  // authored as nothing but `{"width": 0}` read as unauthored, and the panel
  // told the author "nothing here is set, so this header row takes the table's
  // own border" about a header that had taken the border over.
  //
  // BOTH halves are strings and the resolved one is formatted in GO. Every pair
  // on this projection shares one type across the pair; a committed string
  // beside a resolved number would be the first breach of that invariant, and
  // it would breach it on the pair a reader is most likely to mis-read.
  'headerBorder.width': string; 'headerBorder.widthResolved': string
  'headerBorder.color': string; 'headerBorder.colorResolved': string
  'headerBorder.edges': string; 'headerBorder.edgesResolved': string
}>
export type GroupMovePreview = Readonly<{ revision: number; dx: number; dy: number }>

// TableRules is SPEC-table-rules' half of the table projection: the interior
// lines (`rules`) and the ruled area's floor (`minHeight`). It is a separate
// type from TableHeaderStyle because it governs the TABLE, not the header row
// — and because the two arrive through different commands.
//
// The width and colour pairs are spelled exactly as the header border's are,
// committed beside resolved, strings throughout, '' for absent. `between` is
// the boundary set comma-joined in the format's own order — never an array,
// so the whole block moves as six scalars and the guard below stays one
// closed-set check per member. `minHeight` is thousandths and 0 is absent, the
// same spelling `headerHeight` uses.
export type TableRules = Readonly<{
  minHeight: number
  'rules.width': string; 'rules.widthResolved': string
  'rules.color': string; 'rules.colorResolved': string
  'rules.between': string
}>

export type TableColumns = Readonly<{ revision: number; table: Readonly<{ tableId: string; sizing: 'points' | 'proportion'; totalWidth: number; collection: string; alias: string; headerHeight: number; altRowBackground: string; columns: ReadonlyArray<TableColumn> }> & TableHeaderStyle & TableRules & TableCellPadding }>
// THE TABLE'S OWN CELL PADDING, left and right, as the document declares it
// (`style.padding`). Thousandths of a point spelled as strings, so '' is absent
// and '0' a declared zero; the loader admits a negative length, so '-1500' is a
// legal spelling. `paddingHeaderOverride` says `headerStyle.padding` exists and
// takes the header row over, so these reach the data and footer rows only.
export type TableCellPadding = Readonly<{ paddingLeft: string; paddingRight: string; paddingHeaderOverride: boolean }>

// Opaque bytes/JSON are deliberately the only document-bearing values on this
// boundary. These types describe transport, not the .folio file format.
export type EngineRequest = Readonly<{
  protocolVersion: typeof ENGINE_PROTOCOL_VERSION
  kind: 'request'
  requestId: string
  operation: EngineOperation
  payload?: ArrayBuffer | RenderPayload | IdentityPayload
}>

export type EngineSnapshot = Readonly<{
  documentState: 'empty' | 'loaded'
  revision: number
  byteLength: number
	canUndo?: boolean
	canRedo?: boolean
	canvas?: CanvasProjection
}>

// This is paint-only output from Go, not a .folio page model. Values are
// millipoints and are never used to derive a browser document layout.
export type AuthoredProperty<T> = Readonly<{ state: 'absent' | 'null' }> | Readonly<{ state: 'value'; value: T }>
export type AuthoredProperties = Readonly<Record<'fontFamily', AuthoredProperty<string>> & {
  visibleIf: AuthoredProperty<string>
  fontSize: AuthoredProperty<number>
  lineSpacing: AuthoredProperty<number>
  bold: AuthoredProperty<boolean>
  italic: AuthoredProperty<boolean>
  align: AuthoredProperty<string>
  valign: AuthoredProperty<string>
  color: AuthoredProperty<string>
  background: AuthoredProperty<string>
  borderWidth: AuthoredProperty<number>
  borderColor: AuthoredProperty<string>
  borderEdges: AuthoredProperty<ReadonlyArray<'top' | 'right' | 'bottom' | 'left'>>
  // spec-barcode-qr-elements: a qrcode's level; absent (the default M) on every other kind.
  errorCorrection: AuthoredProperty<'L' | 'M' | 'Q' | 'H'>
}>

export type CanvasProjection = Readonly<{
	width: number; height: number; orientation: 'portrait' | 'landscape'; preset: 'A4' | 'Letter' | 'custom'
	// The DOCUMENT's two declared formatting authorities (Story 12.2). `locale`
	// is one of AD-12's four tags and decides how every formatDate and
	// formatNumber in the document renders — including the Buddhist-era year
	// under `th`; `utcOffset` is the ±HH:MM string those dates are resolved in.
	// Both come from Go and neither is defaulted, derived or validated here: the
	// panel shows what the engine holds, proposes what the author typed, and
	// lets the engine refuse it in the engine's own sentence.
	locale: LocaleTag; utcOffset: string
	marginTop: number; marginRight: number; marginBottom: number; marginLeft: number; gridIncrement: number; commandWidth: number; commandHeight: number
	// contentWindowHeight is ONE page's worth of content column, and
	// contentWindowCount is how many of those windows the column occupies —
	// both from Go, neither derived here. The count is a claim about the
	// column as the ENGINE currently paints it, and a floor rather than a
	// prediction wherever a bound table is involved: the canvas has no data,
	// so a table contributes its header and none of its rows.
	contentWindowHeight: number; contentWindowCount: number
	// contentWindowOrigins is where each of those windows BEGINS, in the
	// content column's own band-relative frame — one entry per window,
	// origins[0] === 0, strictly increasing. It comes from Go's own
	// PageAssignment.Shift and is NEVER the window height multiplied by an
	// index: that
	// closed form is the spelling internal/layout/paginate.go forbids by
	// name, and it is wrong by 110 millipoints per window on a column of
	// round 728pt spacing and by nine whole windows on a column with a
	// declared gap. contentWindowCountIsExact is Go saying the count can be
	// TRUSTED — false wherever a registered cause applies: a bound table, a
	// pagination that degraded, text that could not be shaped, or an element
	// whose visibility depends on data. Its sense is deliberately this way
	// round so that its zero value, false, is the SAFE claim; direction —
	// whether the true number is higher or lower — is deliberately not
	// carried, because neither side is safe to act on. Both are engine facts,
	// and neither is a rule this side gets to restate.
	contentWindowOrigins: ReadonlyArray<number>; contentWindowCountIsExact: boolean
	// SPEC-multi-pages CAP-8: the designed page each window belongs to, one
	// entry per window, grouped by page in page order. Origins are PAGE-LOCAL:
	// 0 at each page's first window, strictly rising within a page. A one-page
	// document projects all zeros.
	contentWindowPages: ReadonlyArray<number>
	// SPEC-multi-pages story 2: each designed page's Page Break, one entry per
	// page, page 1's always true. Go always sends it; the guard admits its
	// absence only on a one-page projection, where it can only be [true].
	pageBreaks?: ReadonlyArray<boolean>
	// spec-section-break CAP-6: the content band's break offset, in the content
	// column's band-relative millipoints (the frame a content component's `y`
	// is in). ABSENT when the document declares no break. It comes from Go and
	// is never derived here; the window count ignores it.
	sectionBreak?: number
	// spec-section-break CAP-7: the break's Anchor setting. PRESENT, and false,
	// only when the document declares a break and that break is unanchored;
	// absent means anchored (or no break at all). From Go, never derived here.
	sectionBreakAnchor?: false
	// SPEC-multi-pages story 5: on a projection with more than one designed
	// page, INSTEAD of the pair above, each page's break offset (in its own
	// column's band-relative millipoints) or null, and each page's Anchor —
	// false only where that page's break is unanchored. Absent on a one-page
	// projection. Read through section-break.ts's sectionBreakOnPage.
	sectionBreaks?: ReadonlyArray<number | null>
	sectionBreakAnchors?: ReadonlyArray<boolean>
	// fontFamilies is the closed set style.fontFamily may name in THIS
	// document, from Go, sorted; defaultFontSize is the size the producer
	// draws an element that commits none at. Neither is restated here.
	//
	// defaultLineSpacing (Story 17.3) is the same promise for LEADING: the
	// ratio the producer measures an element with when its style declares
	// none, in THOUSANDTHS, so 1000 is a ratio of 1.0. It is here because the
	// inspector used to spell that `1` itself, and a designer-side copy of an
	// engine-owned default is a second authority that can drift silently.
	fontFamilies: ReadonlyArray<string>; defaultFontSize: number; defaultLineSpacing: number
	// fontChains is the SAME set of chains, with the ordered ENTRIES behind each
	// name: fontChains.map(c => c.name) is fontFamilies, entry for entry, and
	// the validator asserts it rather than trusting it. Entry order is the
	// document's own authored order and is never re-sorted here.
	//
	// An entry is a discriminated OBJECT since Story 8.3: `face` names a face the
	// renderer is given, `assetKey` names one the document carries, exactly one of
	// them is non-empty, and `family`/`style` are what the panel DISPLAYS for an
	// embedded entry — read by Go from the asset's own `font` record, never
	// derived in the browser.
	//
	// `bold`/`italic`/`boldItalic` are Story 11.3's read-back of the entry's own
	// DECLARED style variants (DW-239). '' is absent. They say what the DOCUMENT
	// declares, never what a painted fragment resolved to — that is
	// `textPaint.…fragments[].face`, and confusing the two would put a chain
	// entry in a paint position, which canvas-font-stack.test.ts forbids by name.
	fontChains: ReadonlyArray<Readonly<{ name: string; entries: ReadonlyArray<Readonly<{ face: string; assetKey: string; family: string; style: string; bold: string; italic: string; boldItalic: string }>> }>>
	bands: ReadonlyArray<Readonly<{ name: 'pageHeader' | 'content' | 'pageFooter'; x: number; y: number; width: number; height: number }>>
	components: ReadonlyArray<Readonly<{ id: string; type: 'text' | 'image' | 'table' | 'line' | 'rect' | 'barcode' | 'qrcode'; band: 'pageHeader' | 'content' | 'pageFooter'; x: number; y: number; width: number; height: number; resizable: boolean; authored?: AuthoredProperties; value?: string; binding?: string; visibleIf?: string; fontFamily?: string; fontSize?: number; lineSpacing?: number; bold?: boolean; italic?: boolean; align?: 'left' | 'center' | 'right' | 'justify'; valign?: 'top' | 'middle' | 'bottom'; color?: string; background?: string; borderWidth?: number; borderColor?: string; borderEdges?: ReadonlyArray<'top' | 'right' | 'bottom' | 'left'>; paddingTop?: number; paddingRight?: number; paddingBottom?: number; paddingLeft?: number; tableBind?: string; columns?: ReadonlyArray<Readonly<{ id: string; label: string; labelLines: ReadonlyArray<string>; width: number; headerAlign: 'left' | 'center' | 'right'; cellAlign: 'left' | 'center' | 'right'; bind: string }>>; textPaint?: Readonly<{ overflow: boolean; truncated: boolean; lines: ReadonlyArray<Readonly<{ top: number; baseline: number; advance: number; width: number; fragments: ReadonlyArray<Readonly<{ text: string; x: number; face?: string; assetKey?: string }>> }>> }>; image?: Readonly<{ mediaType: string; assetKey: string; width: number; height: number; drawX: number; drawY: number; drawWidth: number; drawHeight: number }>; imageUnavailable?: 'missing' | 'undecodable'; barcode?: Readonly<{ moduleWidth: number; bars: ReadonlyArray<Readonly<{ x: number; width: number }>> }>; barcodeUnavailable?: 'unencodable' | 'doesNotFit'; qrcode?: Readonly<{ moduleWidth: number; rects: ReadonlyArray<Readonly<{ x: number; y: number; width: number; height: number }>> }>; qrcodeUnavailable?: 'tooLong' | 'doesNotFit'; belowSectionBreak?: boolean; page?: number }>>
}>

export type EngineSuccess = Readonly<{
  protocolVersion: typeof ENGINE_PROTOCOL_VERSION
  kind: 'response'
  requestId: string
  ok: true
  snapshot: EngineSnapshot
  bytes?: ArrayBuffer
	preview?: PreviewEvidence
	parameterReferences?: ParameterReferences
	tableColumns?: TableColumns
	groupMove?: GroupMovePreview
}>

export type EngineFailure = Readonly<{
  protocolVersion: typeof ENGINE_PROTOCOL_VERSION
  kind: 'response'
  requestId: string
  ok: false
  error: EngineError
}>

export type EngineLifecycle = Readonly<{
  protocolVersion: typeof ENGINE_PROTOCOL_VERSION
  kind: 'lifecycle'
  state: 'ready' | 'failed'
  error?: EngineError
}>

export type EngineInbound = EngineSuccess | EngineFailure | EngineLifecycle

// isFontChainEntry is the projected chain entry's own guard (Story 8.3).
// Split out of the isCanvas one-liner rather than inlined there because it
// carries a RULE — the discriminated shape — and a rule buried inside a chain
// of `&&` is a rule nobody edits deliberately.
//
// THE DISCRIMINANT IS PROJECTED, NEVER DERIVED HERE. Exactly one of `face` and
// `assetKey` is non-empty, and this ASSERTS that rather than guessing from a
// value's shape: a 64-character face name is a legal face name, so "looks like
// a digest" was never available as a test, and FontChainEditor is forbidden a
// rule of its own.
//
// An embedded entry always carries a non-empty `family` — Go decides what the
// panel shows and falls back to the asset key — and a named face carries no
// family and no style at all, because its name IS its identity.
//
// STORY 11.3 ADDS THE THREE DECLARED VARIANTS (DW-239), AND THEY ARE ADMITTED,
// NOT ADJUDICATED. '' is absent — the same spelling `family` and `style`
// already use — so every combination of present and absent is a legal entry,
// including an entry that declares none.
//
// A SIBLING'S NAMESPACE IS ITS ENTRY'S DISCRIMINANT (AD-8): on a `face` entry
// these three name FontSet faces, on an `assetKey` entry they name `assets`
// keys. THAT IS CARRIED BY THE DISCRIMINANT, NOT BY THE VALUE, and this guard
// deliberately imposes no rule that would read one namespace as the other. No
// shape check is available or wanted: the same ruling recorded above for
// `assetKey` holds for a variant asset key — a 64-character face name is a
// legal face name, so "looks like a digest" was never a test — and a face-name
// pattern would be the naming-convention weight carrier written backwards
// (D-11.2.1). Whether a declared variant NAMES anything is the engine's
// question, answered where the document is loaded, never here.
const isFontChainEntry = (value: unknown): boolean => {
  if (!isRecord(value) || !hasExactKeys(value, ['face', 'assetKey', 'family', 'style', 'bold', 'italic', 'boldItalic'])) return false
  const { face, assetKey, family, style, bold, italic, boldItalic } = value
  if (typeof face !== 'string' || typeof assetKey !== 'string' || typeof family !== 'string' || typeof style !== 'string') return false
  if (typeof bold !== 'string' || typeof italic !== 'string' || typeof boldItalic !== 'string') return false
  if ([face, assetKey, family, style, bold, italic, boldItalic].some((text) => text.length > MAX_CANVAS_PROPERTY_STRING)) return false
  if ((face.length > 0) === (assetKey.length > 0)) return false
  if (assetKey.length > 0) return family.length > 0
  return family.length === 0 && style.length === 0
}

const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && Object.getPrototypeOf(value) === Object.prototype
const isArrayBuffer = (value: unknown): value is ArrayBuffer => Object.prototype.toString.call(value) === '[object ArrayBuffer]'
const isRenderPayload = (value: unknown): value is RenderPayload => isRecord(value) && hasExactKeys(value, ['template', 'data', 'params']) && ['template', 'data', 'params'].every((key) => isArrayBuffer(value[key]) && value[key].byteLength > 0 && value[key].byteLength <= MAX_ENGINE_PAYLOAD_BYTES)
const isIdentityPayload = (value: unknown): value is IdentityPayload => isRecord(value) && hasExactKeys(value, ['data', 'params']) && ['data', 'params'].every((key) => isArrayBuffer(value[key]) && value[key].byteLength > 0 && value[key].byteLength <= MAX_ENGINE_PAYLOAD_BYTES)
// DW-70. Go sorts the projected chain names with slices.Sorted over Go
// strings, which compares them BY BYTE — and those keys are the canonical
// `.folio`'s own `fonts` key order under AD-9, so Go's order IS the document's
// order and is NORMATIVE. JavaScript's `<` compares UTF-16 CODE UNITS, and the
// two disagree wherever a name mixes the astral planes with U+E000-U+FFFF: a
// surrogate pair (0xD800-) sorts BELOW U+E000 in UTF-16 and ABOVE it in UTF-8.
// The measured pair that motivated this is `'\uE000'` before `'\u{1F600}'` —
// Go's order, and the order this guard used to REJECT, taking the whole
// snapshot with it (isCanvas false -> parseInbound undefined -> PROTOCOL_INVALID
// -> engine-client terminates the worker and the canvas is permanently blank).
// Comparing by CODE POINT is the same sequence as comparing UTF-8 bytes, so the
// browser adopts Go's order. Never the reverse: changing Go's comparator would
// move golden bytes for any document whose chain names cross the boundary.
const compareCodePoints = (left: string, right: string): number => {
  const a = Array.from(left, (unit) => unit.codePointAt(0) as number)
  const b = Array.from(right, (unit) => unit.codePointAt(0) as number)
  for (let index = 0; index < Math.min(a.length, b.length); index++) if (a[index] !== b[index]) return (a[index] as number) < (b[index] as number) ? -1 : 1
  return a.length === b.length ? 0 : a.length < b.length ? -1 : 1
}
const hasOnly = (value: Record<string, unknown>, keys: readonly string[]) => Object.keys(value).every((key) => keys.includes(key))
const hasExactKeys = (value: Record<string, unknown>, keys: readonly string[]) => Object.keys(value).length === keys.length && keys.every((key) => Object.prototype.hasOwnProperty.call(value, key))
export const isEngineRequestId = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= MAX_ENGINE_REQUEST_ID_LENGTH && /^[A-Za-z0-9][A-Za-z0-9._:-]*$/.test(value)

// A supplied provenance field is a producer fact, not a display hint.  Empty
// values have no useful meaning in the closed contract, so reject them at the
// boundary instead of accepting and silently dropping them later by truthiness.
const isError = (value: unknown): value is EngineError => isRecord(value) && hasOnly(value, ['code', 'message', 'elementId', 'dataPath']) && typeof value.code === 'string' && value.code.length > 0 && value.code.length <= 96 && typeof value.message === 'string' && value.message.length > 0 && value.message.length <= 512 && (value.elementId === undefined || typeof value.elementId === 'string' && value.elementId.length > 0 && value.elementId.length <= MAX_ENGINE_ELEMENT_ID_LENGTH) && (value.dataPath === undefined || typeof value.dataPath === 'string' && value.dataPath.length > 0 && value.dataPath.length <= MAX_ENGINE_DATA_PATH_LENGTH)
const isDiagnostic = (value: unknown): value is EngineDiagnostic => isRecord(value) && hasExactKeys(value, ['severity', 'code', 'elementId', 'dataPath', 'message']) && value.severity === 'warning' && typeof value.code === 'string' && value.code.length > 0 && value.code.length <= 96 && typeof value.elementId === 'string' && value.elementId.length <= MAX_ENGINE_ELEMENT_ID_LENGTH && typeof value.dataPath === 'string' && value.dataPath.length <= MAX_ENGINE_DATA_PATH_LENGTH && typeof value.message === 'string' && value.message.length <= 512
const isPreview = (value: unknown): value is PreviewEvidence => isRecord(value) && hasOnly(value, ['revision', 'identity', 'pdfSha256', 'diagnostics', 'elapsedMs', 'version']) && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision >= 0 && typeof value.identity === 'string' && /^[a-f0-9]{64}$/.test(value.identity) && ((value.pdfSha256 === undefined && value.diagnostics === undefined && value.elapsedMs === undefined && value.version === undefined) || (typeof value.pdfSha256 === 'string' && /^[a-f0-9]{64}$/.test(value.pdfSha256) && Array.isArray(value.diagnostics) && value.diagnostics.length <= MAX_ENGINE_DIAGNOSTICS && value.diagnostics.every(isDiagnostic) && typeof value.elapsedMs === 'number' && Number.isSafeInteger(value.elapsedMs) && value.elapsedMs >= 0 && typeof value.version === 'string' && value.version.length > 0 && value.version.length <= 64))
const isParameterReferences = (value: unknown): value is ParameterReferences => isRecord(value) && hasExactKeys(value, ['revision', 'names']) && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision >= 0 && Array.isArray(value.names) && value.names.length <= MAX_ENGINE_PARAMETER_REFERENCES && value.names.every((name) => typeof name === 'string' && name.length > 0 && name.length <= MAX_ENGINE_PARAMETER_NAME_LENGTH && /^[A-Za-z_][A-Za-z0-9_]*$/.test(name)) && new Set(value.names).size === value.names.length && value.names.every((name, index, names) => index === 0 || names[index - 1]! < name)
// The four border edges in the FORMAT's own order, which is the order Go joins
// them in. It is a wire fact for this guard and nothing else — the closed set
// itself lives in `internal/template/closedsets.go`, the loader is the door that
// refuses an unknown name, and this admits only what that door already admitted.
const BORDER_EDGE_ORDER = ['top', 'right', 'bottom', 'left'] as const
const isCanonicalEdgeList = (value: unknown): boolean => {
  if (typeof value !== 'string') return false
  if (value === '') return true
  let next = 0
  for (const part of value.split(',')) {
    // `indexOf` FROM `next`, so the list is strictly ascending in the canonical
    // order: that refuses an unknown name, a repeated one and a re-ordered pair
    // in one expression, and every list Go can send passes it.
    const at = (BORDER_EDGE_ORDER as ReadonlyArray<string>).indexOf(part, next)
    if (at < 0) return false
    next = at + 1
  }
  return true
}
// A LENGTH IN INTEGER THOUSANDTHS, SPELLED AS A STRING SO THAT ABSENCE HAS ITS
// OWN VALUE: `''` is absent, `'0'` is a declared zero, `'500'` is half a point.
//
// ⚠ THE CANONICAL SPELLING IS ASSERTED, NOT MERELY THE CHARACTER CLASS, and the
// two refusals below are the point of this helper rather than pedantry. This
// clause exists because a string member replaced a bounded NUMBER, and the
// number carried `Number.isSafeInteger`; a guard that only asked for digits
// would be WIDER than the member it replaced, which is the opposite of what a
// re-spelling is allowed to do.
//
//  - NO LEADING ZEROS. `'007'` and `'0'` would both be 7 and 0 to `Number()`
//    while being different strings, so a member the panel branches on as a
//    STRING (`committed === ''`, `committed === '0'`) would have two spellings
//    for one value and the remount key would treat them as different states.
//    `strconv.FormatInt` cannot emit one, so this refuses only what Go cannot
//    send. Hence `0|[1-9][0-9]*` rather than `[0-9]+`.
//  - AND THE SAFE-INTEGER MAGNITUDE, which the digit pattern alone does not
//    bound at all: `'9'.repeat(30)` is all digits and is not a number this or
//    any other member can carry through arithmetic. `authored()` divides these
//    by 1000, and past 2^53 that division is silently lossy.
//
// Digits with no sign is the same bound the old `>= 0` carried, and it is
// measured rather than inherited: `parse_bands.go` REFUSES a negative border
// width outright (ISO 32000-1 §8.4.3.2 — a PDF line width is non-negative) and
// accepts zero as the thinnest device line, so a negative one cannot come from a
// loaded document. This is deliberately the same spirit as `isCanonicalEdgeList`
// beside it, which also refuses non-canonical spellings of a legal value.
const isThousandthsLengthString = (value: unknown): boolean => typeof value === 'string' && (value === '' || (/^(?:0|[1-9][0-9]*)$/.test(value) && Number.isSafeInteger(Number(value))))
const authoredKeys = ['visibleIf', 'fontFamily', 'fontSize', 'lineSpacing', 'bold', 'italic', 'align', 'valign', 'color', 'background', 'borderWidth', 'borderColor', 'borderEdges', 'errorCorrection'] as const
const isAuthoredProperties = (value: unknown): value is AuthoredProperties => isRecord(value) && hasExactKeys(value, [...authoredKeys]) && authoredKeys.every((key) => {
  const field = value[key]
  if (!isRecord(field)) return false
  if (field.state === 'absent' || field.state === 'null') return hasExactKeys(field, ['state'])
  if (field.state !== 'value' || !hasExactKeys(field, ['state', 'value'])) return false
  if (key === 'bold' || key === 'italic') return typeof field.value === 'boolean'
  if (key === 'fontSize' || key === 'borderWidth' || key === 'lineSpacing') return Number.isSafeInteger(field.value)
  if (key === 'borderEdges') return Array.isArray(field.value) && field.value.length <= 4 && new Set(field.value).size === field.value.length && field.value.every((edge) => ['top', 'right', 'bottom', 'left'].includes(edge))
  if (key === 'align') return ['left', 'center', 'right', 'justify'].includes(field.value as string)
  if (key === 'valign') return ['top', 'middle', 'bottom'].includes(field.value as string)
  if (key === 'errorCorrection') return ['L', 'M', 'Q', 'H'].includes(field.value as string)
  return typeof field.value === 'string' && field.value.length <= MAX_CANVAS_PROPERTY_STRING
})

const isGroupMove = (value: unknown): value is GroupMovePreview => isRecord(value) && hasExactKeys(value, ['revision', 'dx', 'dy']) && Number.isSafeInteger(value.revision) && (value.revision as number) >= 0 && Number.isSafeInteger(value.dx) && Number.isSafeInteger(value.dy)

// Go sends canonical authored decimals as strings so even the largest exact
// int64 weight survives transport without JavaScript rounding. This checks the
// wire spelling and capacity, never allocates widths or validates author drafts.
const isProportionString = (value: string): boolean => {
  if (value.length > 20 || !/^(?:0|[1-9][0-9]*)(?:\.[0-9]{0,2}[1-9])?$/.test(value)) return false
  const [whole, fraction = ''] = value.split('.')
  const scaled = BigInt(whole! + fraction.padEnd(3, '0'))
  return scaled > 0n && scaled <= 9223372036854775807n
}
const isTableColumns = (value: unknown): value is TableColumns => {
  if (!isRecord(value) || !hasExactKeys(value, ['revision', 'table']) || typeof value.revision !== 'number' || !Number.isSafeInteger(value.revision) || value.revision < 0 || !isRecord(value.table) || !hasExactKeys(value.table, ['tableId', 'sizing', 'totalWidth', 'collection', 'alias', 'headerHeight', 'altRowBackground', 'headerFontFamily', 'headerFontFamilyResolved', 'headerFontSize', 'headerFontSizeResolved', 'headerLineSpacing', 'headerLineSpacingResolved', 'headerBackground', 'headerBackgroundResolved', 'headerColor', 'headerColorResolved', 'headerValign', 'headerValignResolved', 'headerAlign', 'headerAlignResolved', 'headerBold', 'headerBoldResolved', 'headerItalic', 'headerItalicResolved', 'headerBorder.width', 'headerBorder.widthResolved', 'headerBorder.color', 'headerBorder.colorResolved', 'headerBorder.edges', 'headerBorder.edgesResolved', 'minHeight', 'rules.width', 'rules.widthResolved', 'rules.color', 'rules.colorResolved', 'rules.between', 'paddingLeft', 'paddingRight', 'paddingHeaderOverride', 'columns'])) return false
  const table = value.table
  if (!['points', 'proportion'].includes(table.sizing as string) || typeof table.totalWidth !== 'number' || !Number.isSafeInteger(table.totalWidth) || table.totalWidth < 0 || table.sizing === 'proportion' && table.totalWidth === 0) return false
  // THE TYPED CLAUSES FOR STORY 12.3's SIXTEEN MEMBERS. Every one is REQUIRED
  // and never optional: hasExactKeys above already refuses a response that
  // omits one, and the Go side has no `omitempty` for exactly that reason
  // (canvas_projection_wire_test.go's fifth record pins both directions).
  //
  // ADMITTING A VALUE IS NOT ADJUDICATING IT. These bounds exist so a malformed
  // response cannot reach React, not so the browser can second-guess the
  // engine: a value Go committed is a value Go already ruled on. '' and 0 are
  // admitted throughout because that is how this projection spells ABSENT.
  const headerString = (key: keyof TableHeaderStyle | 'altRowBackground') => typeof table[key] === 'string' && (table[key] as string).length <= MAX_CANVAS_PROPERTY_STRING
  // THE GUARD MUST ADMIT EXACTLY WHAT THE FILE DOOR ADMITS, and for these two
  // lengths the file door admits a NEGATIVE one. `internal/template/decimal.go`
  // negates on `sign < 0` and neither `parse_bands.go` (`t.HeaderHeight = hh`)
  // nor the style decoder (`st.FontSize = present(v)`) bounds the result, so a
  // hand-authored `"headerHeight": -5` loads and renders today. A guard
  // requiring `>= 0` therefore refused a document the engine had already
  // accepted — and the symptom was not a warning: parseInbound returns
  // undefined, engine-client terminates the worker with no re-spawn, and on a
  // FIRST table-editor open nothing is shown at all, because the panel that
  // would render the error never mounts.
  //
  // The bound is relaxed HERE rather than added at the loader on purpose:
  // bounding the loader is a format narrowing, and this story's Never list
  // forbids one. DW-26 already records those lengths as unbounded at load.
  const headerLength = (key: keyof TableHeaderStyle | 'headerHeight') => typeof table[key] === 'number' && Number.isSafeInteger(table[key])
  // The line-spacing pair keeps `>= 0`, because for IT the file door really
  // does bound: DecodeLineSpacingRaw refuses anything outside
  // [MinLineSpacingThousandths, MaxLineSpacingThousandths] = [1, 1000000], and
  // 0 is this projection's spelling of absent.
  const headerRatio = (key: keyof TableHeaderStyle) => headerLength(key) && (table[key] as number) >= 0
  if (!(['altRowBackground', 'headerFontFamily', 'headerFontFamilyResolved', 'headerBackground', 'headerBackgroundResolved', 'headerColor', 'headerColorResolved', 'headerBorder.color', 'headerBorder.colorResolved'] as const).every(headerString)) return false
  if (!(['headerHeight', 'headerFontSize', 'headerFontSizeResolved'] as const).every(headerLength)) return false
  if (!(['headerLineSpacing', 'headerLineSpacingResolved'] as const).every(headerRatio)) return false
  // A COMMITTED alignment may be '' — that is what absent looks like — while a
  // RESOLVED one never is: resolveHeaderStyle seeds `left` and `top` before it
  // cascades anything, so an empty resolved value would mean the engine skipped
  // its own default.
  if (!['', 'left', 'center', 'right'].includes(table.headerAlign as string) || !['left', 'center', 'right'].includes(table.headerAlignResolved as string)) return false
  if (!['', 'top', 'middle', 'bottom'].includes(table.headerValign as string) || !['top', 'middle', 'bottom'].includes(table.headerValignResolved as string)) return false
  // STORY 11.3's TWO BOOLEAN PAIRS. `false` is both absent and off — the
  // projection has one member for what is committed and no third state to put
  // an absence in — so both values are admitted on both halves, and there is
  // nothing to adjudicate beyond the type.
  if (!(['headerBold', 'headerBoldResolved', 'headerItalic', 'headerItalicResolved'] as const).every((key) => typeof table[key] === 'boolean')) return false
  // STORY 14.8's BORDER TRIO. THE TWO WIDTHS ARE STRINGS, AND A STRING MEMBER
  // REPLACING A BOUNDED NUMBER MUST TIGHTEN THE CLAUSE, NEVER LOOSEN IT — a
  // string with no format assertion is how a garbage value walks onto a
  // projection that used to be a checked integer. `isThousandthsLengthString`
  // above asserts the CANONICAL spelling and the safe-integer magnitude, which
  // together admit exactly what `strconv.FormatInt` emits and no more; read its
  // comment for why the two refusals it adds are load-bearing.
  //
  // `headerString` supplies the typeof and the length bound every other string
  // member on this projection already gets; the helper supplies the format.
  if (!(['headerBorder.width', 'headerBorder.widthResolved'] as const).every((key) => headerString(key) && isThousandthsLengthString(table[key]))) return false
  // THE EDGE LISTS, CHECKED THE WAY `column.align` IS CHECKED — against the
  // closed set, not against a shape. `''` is admitted on BOTH halves and means
  // two different things: on the committed member it is "the document declares
  // no edge list", and on the resolved one it is "nothing is painted" (no border
  // reaches the header row, or a declared list names no side). Order is
  // CANONICAL because Go joins in the format's own order; a re-ordered or
  // repeated list is a projection this engine does not produce.
  if (!(['headerBorder.edges', 'headerBorder.edgesResolved'] as const).every((key) => isCanonicalEdgeList(table[key]))) return false
  // SPEC-table-rules' SIX MEMBERS, each checked the way its nearest sibling
  // already is — the two width strings against the canonical thousandths
  // spelling, the two colours as ordinary bounded strings, `minHeight` as a
  // length (never negative: the loader refuses a non-positive floor outright,
  // which is why this one CAN take the bound `headerHeight` cannot), and
  // `between` against the closed boundary set in the format's own order.
  if (!(['rules.width', 'rules.widthResolved'] as const).every((key) => typeof table[key] === 'string' && (table[key] as string).length <= MAX_CANVAS_PROPERTY_STRING && isThousandthsLengthString(table[key]))) return false
  if (!(['rules.color', 'rules.colorResolved'] as const).every((key) => typeof table[key] === 'string' && (table[key] as string).length <= MAX_CANVAS_PROPERTY_STRING)) return false
  if (typeof table.minHeight !== 'number' || !Number.isSafeInteger(table.minHeight) || table.minHeight < 0) return false
  if (!['', 'columns', 'rows', 'columns,rows'].includes(table['rules.between'] as string)) return false
  // THE CELL PADDING PAIR: strconv.FormatInt's own spelling, which may carry a
  // leading '-' because the loader bounds no padding length. A guard stricter
  // than the file door would kill the worker over a document Go admitted.
  if (!(['paddingLeft', 'paddingRight'] as const).every((key) => typeof table[key] === 'string' && (table[key] === '' || (/^-?(?:0|[1-9][0-9]*)$/.test(table[key] as string) && table[key] !== '-0' && Number.isSafeInteger(Number(table[key])))))) return false
  if (typeof table.paddingHeaderOverride !== 'boolean') return false
  return typeof table.tableId === 'string' && table.tableId.length > 0 && table.tableId.length <= MAX_ENGINE_ELEMENT_ID_LENGTH && typeof table.collection === 'string' && table.collection.length > 0 && table.collection.length <= MAX_ENGINE_BINDING_LENGTH && typeof table.alias === 'string' && table.alias.length > 0 && table.alias.length <= 64 && Array.isArray(table.columns) && table.columns.length <= 128 && table.columns.every((column) => isRecord(column) && hasExactKeys(column, ['id', 'header', 'width', 'proportion', 'align', 'headerAlign', 'headerAlignResolved', 'binding', 'rowField', 'rowFieldEditable', 'footer', 'footerOf', 'footerFormat']) && typeof column.id === 'string' && column.id.length > 0 && column.id.length <= MAX_ENGINE_ELEMENT_ID_LENGTH && typeof column.header === 'string' && Array.from(column.header).length <= MAX_TABLE_COLUMN_HEADER_CODE_POINTS && typeof column.width === 'number' && Number.isSafeInteger(column.width) && column.width > 0 && typeof column.proportion === 'string' && (table.sizing === 'points' ? column.proportion === '' : isProportionString(column.proportion)) && ['left', 'center', 'right'].includes(column.align as string) && ['', 'left', 'center', 'right'].includes(column.headerAlign as string) && ['left', 'center', 'right'].includes(column.headerAlignResolved as string) && typeof column.binding === 'string' && column.binding.length <= MAX_ENGINE_BINDING_LENGTH && typeof column.rowField === 'string' && column.rowField.length <= MAX_ENGINE_BINDING_LENGTH && typeof column.rowFieldEditable === 'boolean' && ['','sum','avg','count'].includes(column.footer as string) && typeof column.footerOf === 'string' && column.footerOf.length <= MAX_ENGINE_BINDING_LENGTH && typeof column.footerFormat === 'string' && column.footerFormat.length <= 256) && new Set(table.columns.map((item) => (item as Record<string, unknown>).id)).size === table.columns.length
}
const isCanvas = (value: unknown): value is CanvasProjection => {
  if (!isRecord(value) || !hasOnly(value, ['width', 'height', 'orientation', 'preset', 'locale', 'utcOffset', 'marginTop', 'marginRight', 'marginBottom', 'marginLeft', 'gridIncrement', 'commandWidth', 'commandHeight', 'fontFamilies', 'fontChains', 'defaultFontSize', 'defaultLineSpacing', 'contentWindowHeight', 'contentWindowCount', 'contentWindowOrigins', 'contentWindowPages', 'pageBreaks', 'contentWindowCountIsExact', 'sectionBreak', 'sectionBreakAnchor', 'sectionBreaks', 'sectionBreakAnchors', 'bands', 'components']) || !['A4', 'Letter', 'custom'].includes(value.preset as string) || (value.orientation !== 'portrait' && value.orientation !== 'landscape')) return false
  // THE TWO DOCUMENT-SETTINGS CLAUSES ARE LOAD-BEARING, and `hasOnly` above
  // cannot stand in for them: it is a SUBSET check, so a key Go simply failed
  // to send passes it and reaches the panel as `undefined` — a locale row with
  // no value and an offset row that would send the string "undefined" back.
  // Only a typed clause catches an ABSENT key, which is the failure this story
  // could otherwise ship in silence.
  //
  // `locale` is checked against LOCALE_TAGS, the browser's one spelling of
  // AD-12's closed set, exactly as `preset` and `orientation` are checked
  // against theirs on the line above. `utcOffset` is checked only for shape —
  // a non-empty string within the projection's ordinary string bound. ITS
  // GRAMMAR IS NOT RESTATED HERE: ±HH:MM is the engine's rule
  // (template.IsUTCOffset, the one predicate the loader and the command door
  // both ask), and a browser-side copy of it would be a second authority that
  // could refuse a document Go admits.
  if (!LOCALE_TAGS.includes(value.locale as LocaleTag)) return false
  if (typeof value.utcOffset !== 'string' || value.utcOffset.length === 0 || value.utcOffset.length > MAX_CANVAS_PROPERTY_STRING) return false
  const integer = (key: string, positive = false) => typeof value[key] === 'number' && Number.isSafeInteger(value[key]) && (positive ? value[key] > 0 : value[key] >= 0)
  if (!['width', 'height', 'gridIncrement', 'commandWidth', 'commandHeight', 'defaultFontSize', 'defaultLineSpacing', 'contentWindowHeight', 'contentWindowCount'].every((key) => integer(key, true)) || !['marginTop', 'marginRight', 'marginBottom', 'marginLeft'].every((key) => integer(key))) return false
  // The declared font chain names, as Go sorted them: bounded in count and
  // length like every other list on this projection, unique, and in the order
  // Go sent so the browser never re-sorts an engine-owned set. The ordering
  // check is compareCodePoints, which is Go's byte order — see DW-70 above;
  // `>=` on JavaScript strings is NOT, and dropped a legitimate snapshot.
  if (!Array.isArray(value.fontFamilies) || value.fontFamilies.length > MAX_ENGINE_FONT_FAMILIES || !value.fontFamilies.every((name) => typeof name === 'string' && name.length > 0 && name.length <= MAX_CANVAS_PROPERTY_STRING) || value.fontFamilies.some((name, index, names) => index > 0 && compareCodePoints(names[index - 1] as string, name as string) >= 0)) return false
  // The chains those names stand for. Bounded in count and in per-chain entry
  // count, no chain empty — an empty chain is not one Go projects, because it
  // is not one style.fontFamily may name. The `chain.name === fontFamilies[i]`
  // clause is the cross-check the two lists exist to give each other: Go builds
  // fontFamilies FROM fontChains, so any disagreement here is a channel fault
  // and the snapshot is not trusted.
  //
  // AN ENTRY IS AN OBJECT, NOT A STRING (Story 8.3). It used to be
  // `typeof face === 'string'`, and that clause rejected an object entry
  // outright — isCanvas false, parseInbound undefined, the worker terminated
  // and the canvas permanently blank with no element id and nothing to
  // attribute it to. That is why the Go projection and this guard change in
  // ONE commit; canvas_projection_wire_test.go reddens if only one of them
  // moves, at the entry level as well as at the chain level.
  const chains = value.fontChains
  if (!Array.isArray(chains) || chains.length !== value.fontFamilies.length) return false
  if (!chains.every((chain, index) => isRecord(chain) && hasExactKeys(chain, ['name', 'entries']) && chain.name === (value.fontFamilies as ReadonlyArray<unknown>)[index] && Array.isArray(chain.entries) && chain.entries.length > 0 && chain.entries.length <= MAX_ENGINE_FONT_CHAIN_ENTRIES && chain.entries.every((entry) => isFontChainEntry(entry)))) return false
  // The window origins, in the same shape: bounded in count, every entry a
  // safe non-negative integer, and in the order and at the length Go's own
  // pagination fixes. `hasOnly` is a SUBSET check, so an origins key Go
  // simply failed to send is caught HERE and nowhere else — as is a `nil`
  // slice, which marshals to null and is not an array.
  const origins = value.contentWindowOrigins
  if (!Array.isArray(origins) || origins.length === 0 || origins.length > MAX_ENGINE_CONTENT_WINDOWS || origins.length !== value.contentWindowCount) return false
  // SPEC-multi-pages CAP-8: every window names its designed page. Pages start
  // at 0 and step by at most one, so windows are grouped by page in page
  // order; origins are page-local — 0 at each page's first window and
  // strictly rising within a page.
  const windowPages = value.contentWindowPages
  if (!Array.isArray(windowPages) || windowPages.length !== origins.length || !windowPages.every((page) => typeof page === 'number' && Number.isSafeInteger(page) && page >= 0) || windowPages[0] !== 0 || windowPages.some((page, index) => index > 0 && (page as number) !== (windowPages[index - 1] as number) && (page as number) !== (windowPages[index - 1] as number) + 1)) return false
  const pageStart = (index: number) => index === 0 || windowPages[index] !== windowPages[index - 1]
  if (!origins.every((origin) => typeof origin === 'number' && Number.isSafeInteger(origin) && origin >= 0) || origins.some((origin, index) => pageStart(index) ? origin !== 0 : (origins[index - 1] as number) >= (origin as number))) return false
  // SPEC-multi-pages story 2: one Page Break per designed page, page 1's
  // always true. Absent only on a one-page projection (see the type).
  const pageCount = (windowPages[windowPages.length - 1] as number) + 1
  const pageBreaks = value.pageBreaks
  if (pageBreaks === undefined ? pageCount !== 1 : !Array.isArray(pageBreaks) || pageBreaks.length !== pageCount || !pageBreaks.every((entry) => typeof entry === 'boolean') || pageBreaks[0] !== true) return false
  if (typeof value.contentWindowCountIsExact !== 'boolean') return false
  const bands = value.bands
  const components = value.components
  if (!Array.isArray(bands) || bands.length !== 3 || !Array.isArray(components)) return false
  const names = ['pageHeader', 'content', 'pageFooter']
  const page = value as Record<string, number>
  const bandsValid = bands.every((band, index) => {
    if (!isRecord(band) || !hasOnly(band, ['name', 'x', 'y', 'width', 'height']) || band.name !== names[index] || !['x', 'y', 'width', 'height'].every((key) => typeof band[key] === 'number' && Number.isSafeInteger(band[key]))) return false
    const paint = band as Record<string, number>
    if (!(paint.x >= 0 && paint.y >= 0 && paint.width > 0 && paint.height >= 0 && paint.x + paint.width <= page.width && paint.y + paint.height <= page.height)) return false
    if (index > 0) {
      const prior = bands[index - 1] as Record<string, number>
      if (paint.x !== prior.x || paint.width !== prior.width || paint.y !== prior.y + prior.height) return false
    }
    return true
  })
  const componentTypes = ['text', 'image', 'table', 'line', 'rect', 'barcode', 'qrcode']
  const bandNames = ['pageHeader', 'content', 'pageFooter']
  if (!bandsValid) return false
  // spec-section-break CAP-6. `hasOnly` cannot see this key, and it is
  // OPTIONAL, so its clause is typed here: absent, or a safe integer strictly
  // inside the content band — the range Go's loader already enforces, so a
  // snapshot outside it is a channel fault, not a document.
  const contentBand = bands[1] as Record<string, number>
  const breakInBand = (offset: unknown) => typeof offset === 'number' && Number.isSafeInteger(offset) && offset > 0 && offset < contentBand.height
  // SPEC-multi-pages story 5: a ONE-PAGE projection carries the one-page pair
  // and never the per-page pair; a MULTI-PAGE projection carries the per-page
  // pair, one entry per designed page (an offset or null; Anchor false only
  // beside a break), and never the one-page pair. Go always sends the
  // per-page pair on a multi-page projection.
  const breaks = value.sectionBreaks
  const anchors = value.sectionBreakAnchors
  if (pageCount === 1) {
    if (breaks !== undefined || anchors !== undefined) return false
    if (value.sectionBreak !== undefined && !breakInBand(value.sectionBreak)) return false
    // CAP-7: the Anchor key is absent, or exactly `false` beside a break.
    if (value.sectionBreakAnchor !== undefined && !(value.sectionBreakAnchor === false && value.sectionBreak !== undefined)) return false
  } else {
    if (value.sectionBreak !== undefined || value.sectionBreakAnchor !== undefined) return false
    if (!Array.isArray(breaks) || breaks.length !== pageCount || !breaks.every((offset) => offset === null || breakInBand(offset))) return false
    if (!Array.isArray(anchors) || anchors.length !== pageCount || !anchors.every((anchor, index) => anchor === true || (anchor === false && breaks[index] !== null))) return false
  }
  const pageHasBreak = (page: number) => pageCount === 1 ? value.sectionBreak !== undefined : (breaks as ReadonlyArray<unknown>)[page] !== null
  const ids = new Set<string>()
  let priorBand = -1
	return components.every((component) => {
	if (!isRecord(component) || !hasOnly(component, ['id', 'type', 'band', 'x', 'y', 'width', 'height', 'resizable', 'authored', 'value', 'binding', 'visibleIf', 'fontFamily', 'fontSize', 'lineSpacing', 'bold', 'italic', 'align', 'valign', 'color', 'background', 'borderWidth', 'borderColor', 'borderEdges', 'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft', 'tableBind', 'columns', 'textPaint', 'image', 'imageUnavailable', 'barcode', 'barcodeUnavailable', 'qrcode', 'qrcodeUnavailable', 'belowSectionBreak', 'page']) || typeof component.id !== 'string' || component.id.length === 0 || component.id.length > MAX_ENGINE_ELEMENT_ID_LENGTH || ids.has(component.id) || !componentTypes.includes(component.type as string) || !bandNames.includes(component.band as string) || typeof component.resizable !== 'boolean' || !['x', 'y', 'width', 'height'].every((key) => typeof component[key] === 'number' && Number.isSafeInteger(component[key]) && (component[key] as number) >= 0)) return false
    if (component.authored !== undefined && !isAuthoredProperties(component.authored)) return false
    // SPEC-multi-pages story 2: the designed page a content component belongs
    // to, indexing an existing page; 0 for the page header and footer. Go
    // always sends it; absent is admitted only on a one-page projection.
    if (component.page === undefined ? pageCount !== 1 : typeof component.page !== 'number' || !Number.isSafeInteger(component.page) || component.page < 0 || component.page >= pageCount || (component.band !== 'content' && component.page !== 0)) return false
    // Section membership is Go's: carried by every CONTENT component exactly
    // when ITS OWN PAGE has a break (story 5), and by nothing else.
    const withBreak = component.band === 'content' && pageHasBreak((component.page as number | undefined) ?? 0)
    if (component.belowSectionBreak === undefined ? withBreak : typeof component.belowSectionBreak !== 'boolean' || !withBreak) return false
    ids.add(component.id)
    const bandIndex = bandNames.indexOf(component.band as string)
    if (bandIndex < priorBand) return false
    priorBand = bandIndex
    const band = bands[bandIndex] as Record<string, number>
    const box = component as Record<string, number>
    const table = component.type === 'table'
    if (table ? component.resizable || box.height <= 0 : !component.resizable || box.width <= 0 || box.height <= 0) return false
    // THE HORIZONTAL CAP IS UNIVERSAL; the vertical one is not. A band is as
    // wide as the printable page and nothing may hang off its side, in any
    // band. The vertical cap belongs only to the bands that HAVE a capacity —
    // see BANDS_CAPPING_VERTICALLY, which Go's containComponent mirrors.
    if (!(box.x + box.width <= band.width)) return false
    if (BANDS_CAPPING_VERTICALLY.includes(component.band as string) && !(box.y + box.height <= band.height)) return false
    // THE FOURTH HAND-COPIED MIRROR (DW-25). This one predicate used to cap
    // `value` — the document's BODY TEXT — at the same 512 as seven
    // identifier and colour keys: maxCanvasPropertyString's two-jobs
    // conflation, reproduced exactly on the browser side. Splitting Go's
    // constant without splitting this one would have changed nothing
    // observable: the browser would go on dropping the whole response at 512
    // bytes of clause text, with no attributable error.
    const boundedString = (key: string, limit: number) => component[key] === undefined || typeof component[key] === 'string' && (component[key] as string).length <= limit
    const optionalString = (key: string) => boundedString(key, MAX_CANVAS_PROPERTY_STRING)
    const optionalLength = (key: string) => component[key] === undefined || typeof component[key] === 'number' && Number.isSafeInteger(component[key]) && (component[key] as number) >= 0
	if (!boundedString('value', MAX_CANVAS_BODY_TEXT) || !['binding', 'visibleIf', 'fontFamily', 'color', 'background', 'borderColor', 'tableBind'].every(optionalString) || !['fontSize', 'borderWidth', 'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft'].every(optionalLength) || (component.bold !== undefined && typeof component.bold !== 'boolean') || (component.italic !== undefined && typeof component.italic !== 'boolean')) return false
	if (component.binding !== undefined && (typeof component.binding !== 'string' || component.binding.length === 0 || component.binding.length > MAX_ENGINE_BINDING_LENGTH)) return false
    // Story 7.3 / FR47: the COMPONENT alignment vocabulary admits
    // `justify`; the COLUMN one (isTableColumns, above) deliberately does
    // not. This validator GATES the projection — an unrecognised value
    // drops the whole response and blanks the canvas — so a justified
    // document would otherwise show as nothing at all rather than as
    // itself. The inspector control that OFFERS the choice is Story 7.4's;
    // admitting the value is not offering it.
    if (component.align !== undefined && !['left', 'center', 'right', 'justify'].includes(component.align as string) || component.valign !== undefined && !['top', 'middle', 'bottom'].includes(component.valign as string)) return false
    if (component.borderEdges !== undefined && (!Array.isArray(component.borderEdges) || component.borderEdges.length === 0 || component.borderEdges.some((edge) => !['top', 'right', 'bottom', 'left'].includes(edge)))) return false
    // style.lineSpacing, projected for the first time by Story 7.4: a
    // dimensionless ratio in THOUSANDTHS, positive, and bounded by the same
    // range the engine's one validator enforces at load and on the property
    // command alike (template.MinLineSpacingThousandths ..
    // MaxLineSpacingThousandths, D-7.2.3). Admitting the value is not
    // adjudicating it — a value Go committed is a value Go already ruled on.
    if (component.lineSpacing !== undefined && (typeof component.lineSpacing !== 'number' || !Number.isSafeInteger(component.lineSpacing) || component.lineSpacing < MIN_LINE_SPACING_THOUSANDTHS || component.lineSpacing > MAX_LINE_SPACING_THOUSANDTHS)) return false
	if (component.type !== 'text' && component.type !== 'barcode' && component.type !== 'qrcode' && component.value !== undefined) return false
	// STORY 14.4: the same array the panel's pre-flight reads, so a Go-side
	// change to the scalar-binding gate cannot leave this guard and that gate
	// spelling two different rules. A HOIST, NOT A CHANGE — `['text']` was
	// byte-for-byte the set the inline literal admitted; `barcode` joined it when
	// the barcode element shipped, in Go and here together.
	if (!SCALAR_BINDING_COMPONENT_TYPES.includes(component.type as CanvasComponentType) && component.binding !== undefined) return false
	if (component.type !== 'table' && component.tableBind !== undefined) return false
	// STORY 14.9 — THE PER-COLUMN CLAUSE, in the shape isTableColumns' own
	// per-column clause uses: a record per column, `hasExactKeys` so a key Go
	// stops sending fails as hard as a key Go starts sending, and a CLOSED SET
	// for each of the two resolved alignments. `columns[].align`'s vocabulary is
	// three values, never four — `ColumnAlignTokens` in
	// internal/template/closedsets.go — and a justified table is refused at
	// load, so `justify` cannot reach either of these keys.
	//
	// ⚠ THE WIDTH IS ANY SAFE INTEGER, AND `> 0` WOULD BE A DEFECT. A column
	// with `width: 0` or a negative width LOADS, PROJECTS AND PAINTS today;
	// requiring positivity here would make isCanvas false, parseInbound
	// undefined and PROTOCOL_INVALID terminate the worker on a document that
	// works now. isTableColumns DOES require `width > 0`, because the table
	// editor's producer refuses such a column outright — a different surface
	// with a different contract, and copying its bound onto this one is the
	// specific mistake this comment exists to stop.
	//
	// ⚠ THE IDS ARE UNIQUE, matching the component-level `ids` Set above and
	// isTableColumns' own per-table dedupe. This CANNOT newly refuse a document
	// that ships today — `parse.go`'s `claimID` is the one door into an element
	// id and it refuses a duplicate outright, "ids are unique document-wide
	// (AD-10)", for a column exactly as for a component
	// (`columns[].id`, parse_bands.go), which
	// TestColumnIdsAreUniqueDocumentWide proves rather than assumes. What it
	// stops is a malformed message reaching the painter, which keys its two
	// rows on the column id.
	//
	// ⚠ AND THE ARRAY CARRIES NO LENGTH CAP, for the same reason. The loader
	// (internal/template/parse_bands.go) imposes no minimum and no maximum on
	// `columns`; the 128 cap belongs to the table EDITOR's projection and to
	// `addTableColumn`, neither of which is on this path. A hand-authored
	// hundred-and-fifty-column document paints today, so a cap here would newly
	// kill it — and `components` itself, one level up, is unbounded in this same
	// guard for exactly this reason.
	if (component.columns !== undefined && (!Array.isArray(component.columns) || !component.columns.every((column) => isRecord(column) && hasExactKeys(column, ['id', 'label', 'labelLines', 'width', 'headerAlign', 'cellAlign', 'bind']) && typeof column.id === 'string' && column.id.length > 0 && column.id.length <= MAX_ENGINE_ELEMENT_ID_LENGTH && typeof column.label === 'string' && column.label.length <= MAX_CANVAS_PROPERTY_STRING && Array.isArray(column.labelLines) && column.labelLines.length <= MAX_CANVAS_TABLE_LABEL_LINES && column.labelLines.every((line) => typeof line === 'string' && line.length <= MAX_CANVAS_TABLE_LABEL_LINE_LENGTH) && typeof column.width === 'number' && Number.isSafeInteger(column.width) && ['left', 'center', 'right'].includes(column.headerAlign as string) && ['left', 'center', 'right'].includes(column.cellAlign as string) && typeof column.bind === 'string' && column.bind.length <= MAX_CANVAS_PROPERTY_STRING) || new Set(component.columns.map((column) => (column as Record<string, unknown>).id)).size !== component.columns.length)) return false
	// The twin of the `tableBind` cross-clause above: only a table has columns,
	// and a non-table component carrying them is a producer that has lost track
	// of which element it is projecting.
	if (component.type !== 'table' && component.columns !== undefined) return false
	if (!['text', 'table'].includes(component.type as string) && ['fontFamily', 'fontSize', 'bold', 'italic', 'align', 'valign'].some((key) => component[key] !== undefined)) return false
	if (!isTextPaint(component.textPaint, box)) return false
	if (component.type === 'text' ? component.textPaint === undefined : component.textPaint !== undefined) return false
	if (!isImagePaint(component.image, box)) return false
	if (component.type !== 'image' && component.image !== undefined) return false
	// Finding 9 (review of 2026-08-29): the bounded, enumerated reason
	// discriminant Go emits alongside an absent image paint — only legal
	// for an 'image' component whose image paint is itself absent (the
	// two are the same "one Go-side signal", D-5.13.2), never alongside a
	// present paint and never for a non-image component.
	if (component.imageUnavailable !== undefined && (component.type !== 'image' || component.image !== undefined || !['missing', 'undecodable'].includes(component.imageUnavailable as string))) return false
	// spec-barcode-qr-elements CAP-5: a barcode's bars are Go-computed geometry
	// (AD-17), legal only on a barcode, and its bounded unavailable reason only
	// alongside an absent paint — the image pair's rule, restated for its kind.
	if (!isBarcodePaint(component.barcode, box)) return false
	if (component.type !== 'barcode' && component.barcode !== undefined) return false
	if (component.barcodeUnavailable !== undefined && (component.type !== 'barcode' || component.barcode !== undefined || !['unencodable', 'doesNotFit'].includes(component.barcodeUnavailable as string))) return false
	// The qrcode pair, on the barcode pair's terms: Go-computed module runs,
	// legal only on a qrcode, with a bounded reason only beside an absent paint.
	if (!isQRCodePaint(component.qrcode, box)) return false
	if (component.type !== 'qrcode' && component.qrcode !== undefined) return false
	if (component.qrcodeUnavailable !== undefined && (component.type !== 'qrcode' || component.qrcode !== undefined || !['tooLong', 'doesNotFit'].includes(component.qrcodeUnavailable as string))) return false
	return true
  })
}

// isBarcodePaint admits a barcode's bars as Go projects them: one positive
// module width, and bars ordered left to right, each a whole number of modules
// wide, inside the component's own width. X is relative to the component.
const isBarcodePaint = (value: unknown, box: Record<string, number>): boolean => {
  if (value === undefined) return true
  if (!isRecord(value) || !hasExactKeys(value, ['moduleWidth', 'bars'])) return false
  const module = value.moduleWidth
  if (typeof module !== 'number' || !Number.isSafeInteger(module) || module <= 0 || !Array.isArray(value.bars) || value.bars.length === 0) return false
  let right = 0
  return value.bars.every((bar) => {
    if (!isRecord(bar) || !hasExactKeys(bar, ['x', 'width'])) return false
    const { x, width } = bar
    if (typeof x !== 'number' || typeof width !== 'number' || !Number.isSafeInteger(x) || !Number.isSafeInteger(width) || x < right || width <= 0 || width % module !== 0) return false
    right = x + width
    return right <= box.width
  })
}

// isQRCodePaint admits a qrcode's module runs as Go projects them: one positive
// module width, and rects with exactly x, y, width and height, all integers,
// each one module tall (square modules) and a whole number of modules wide,
// ordered top to bottom and left to right within a row with no two touching,
// and inside the component's own box. Coordinates are component-relative.
const isQRCodePaint = (value: unknown, box: Record<string, number>): boolean => {
  if (value === undefined) return true
  if (!isRecord(value) || !hasExactKeys(value, ['moduleWidth', 'rects'])) return false
  const module = value.moduleWidth
  if (typeof module !== 'number' || !Number.isSafeInteger(module) || module <= 0 || !Array.isArray(value.rects) || value.rects.length === 0) return false
  let rowY = -1
  let right = 0
  return value.rects.every((rect) => {
    if (!isRecord(rect) || !hasExactKeys(rect, ['x', 'y', 'width', 'height'])) return false
    const { x, y, width, height } = rect
    if (![x, y, width, height].every((n) => typeof n === 'number' && Number.isSafeInteger(n))) return false
    const [rx, ry, rw, rh] = [x, y, width, height] as number[]
    if (rx < 0 || ry < 0 || rh !== module || rw <= 0 || rw % module !== 0) return false
    if (ry < rowY || (ry === rowY && rx <= right)) return false
    if (ry > rowY && rowY >= 0 && (ry - rowY) % module !== 0) return false
    rowY = ry
    right = rx + rw
    return right <= (box.width as number) && ry + rh <= (box.height as number)
  })
}

// isImagePaint admits Story 5.13's optional per-component paint-only image
// projection. Absence is always legal for an 'image' component (D-5.13.2:
// "absence, not zero" — an unrecognised or undecodable asset simply has no
// paint). When present, every field must be a bounded, positive, in-box
// value: the draw rectangle is asserted to sit INSIDE the component's own
// box, exactly like the fit-and-centre invariant it is meant to project.
const isImagePaint = (value: unknown, box: Record<string, number>): boolean => {
  if (value === undefined) return true
  if (!isRecord(value) || !hasOnly(value, ['mediaType', 'assetKey', 'width', 'height', 'drawX', 'drawY', 'drawWidth', 'drawHeight'])) return false
  if (typeof value.mediaType !== 'string' || value.mediaType.length === 0 || value.mediaType.length > 128) return false
  // Finding 12 (review of 2026-08-29): D-5.13.2's amendment settled the wire
  // key as the FULL 64-hex digest — a per-key bytes request cannot address
  // an asset by a prefix (Go's isAssetKeyShape, asset_bytes.go, requires
  // exactly 64). This admission previously accepted 1..64, so a truncated
  // key passed straight through to a per-key fetch that could never
  // succeed (Finding 13's permanent "Loading image…").
  if (typeof value.assetKey !== 'string' || value.assetKey.length !== 64 || !/^[a-f0-9]{64}$/.test(value.assetKey)) return false
  const integer = (key: string, positive = false) => typeof (value as Record<string, unknown>)[key] === 'number' && Number.isSafeInteger((value as Record<string, unknown>)[key]) && (positive ? ((value as Record<string, unknown>)[key] as number) > 0 : ((value as Record<string, unknown>)[key] as number) >= 0)
  if (!['width', 'height', 'drawWidth', 'drawHeight'].every((key) => integer(key, true)) || !['drawX', 'drawY'].every((key) => integer(key))) return false
  const paint = value as Record<string, number>
  return paint.drawX >= box.x && paint.drawY >= box.y && paint.drawX + paint.drawWidth <= box.x + box.width && paint.drawY + paint.drawHeight <= box.y + box.height
}

// isTextPaint admits the engine's own honest measurement and checks only
// what the JS boundary can genuinely go wrong at. It deliberately does
// NOT check `paint.baseline > paint.top + paint.advance` any more
// (Story 7.2, D-7.2.2): the engine emits `baseline = top + FirstBaseline`
// while `advance` is the SCALED value, so that clause reduced to
// `FirstBaseline <= Advance` — an ENGINE invariant restated on the
// browser's side of the channel, and one `style.lineSpacing`
// deliberately dissolves.
//
// `FirstBaseline > Advance` means one line's baseline sits below the
// next line's top: the line boxes overlap. That IS tight leading, it is
// what the PDF draws, and refusing it here failed one line, then
// isCanvas, then isSnapshot, and blanked the WHOLE projection. AD-17
// says the canvas takes every text metric FROM the engine; the browser
// adjudicating them was that invariant inverted, not enforced.
//
// The real invariants all survive on the line below and must stay:
// `paint.advance <= 0`, `paint.baseline < paint.top` (FirstBaseline is
// an ascent clamped at zero, and lineSpacing scales only Advance), the
// Number.isSafeInteger checks (the actual JS-boundary concern), and
// `paint.top < priorTop + priorAdvance` — which is `originY+i·A <
// originY+i·A`, false for any positive advance, so it does not become
// the next cliff.
const isTextPaint = (value: unknown, component: Record<string, number>): boolean => {
  if (value === undefined) return true
  // `truncated` is required exactly as `overflow` is: Go emits both
  // unconditionally, and a paint arriving without it is a producer that has
  // drifted from this contract, not an older one to be tolerated.
  if (!isRecord(value) || !hasOnly(value, ['overflow', 'truncated', 'lines']) || typeof value.overflow !== 'boolean' || typeof value.truncated !== 'boolean' || !Array.isArray(value.lines) || value.lines.length > MAX_CANVAS_BODY_TEXT_LINES) return false
  let priorTop = -1
  let priorAdvance = 0
  // CUMULATIVE across every line of the component, never reset — which is a
  // different quantity from Go's per-line maxCanvasTextFragments. Go carries
  // its own cumulative counter (maxCanvasBodyTextFragments) precisely so it
  // never emits a projection this line would discard.
  let fragments = 0
  return value.lines.every((line) => {
    if (!isRecord(line) || !hasOnly(line, ['top', 'baseline', 'advance', 'width', 'fragments']) || !['top', 'baseline', 'advance', 'width'].every((key) => typeof line[key] === 'number' && Number.isSafeInteger(line[key]))) return false
    const paint = line as Record<string, number>
    if (paint.top < component.y || paint.baseline < paint.top || paint.advance <= 0 || paint.width < 0 || (priorTop >= 0 && paint.top < priorTop + priorAdvance) || (!value.overflow && paint.width > component.width) || !Array.isArray(line.fragments)) return false
    priorTop = paint.top
    priorAdvance = paint.advance
    // A FRAGMENT'S TWO ATTRIBUTION KEYS ARE A DISCRIMINATED PAIR, on the model
    // isFontChainEntry already applies one level up: `assetKey` names a face
    // the DOCUMENT carries, `face` (Story 8.4e) names one the ENGINE ships,
    // and no fragment may carry both. NEITHER is legal and deliberately so —
    // that is the wire's own statement of "unattributed", and such a fragment
    // paints on the stylesheet's declared stack rather than terminating the
    // worker. `face` is bounded by MAX_CANVAS_PROPERTY_STRING, the same bound
    // a chain entry's `face` already uses, and is checked HERE rather than
    // trusted: Go can only put a FontSet key on this field today, but a
    // guard's job is to hold when the other side is wrong.
    return line.fragments.every((fragment) => {
      fragments++
      return fragments <= MAX_CANVAS_BODY_TEXT_FRAGMENTS && isRecord(fragment) && hasOnly(fragment, ['text', 'x', 'face', 'assetKey']) && typeof fragment.text === 'string' && fragment.text.length > 0 && fragment.text.length <= MAX_CANVAS_BODY_TEXT && typeof fragment.x === 'number' && Number.isSafeInteger(fragment.x) && fragment.x >= component.x && fragment.x <= component.x + Math.max(paint.width, component.width) && (fragment.assetKey === undefined || (typeof fragment.assetKey === 'string' && /^[a-f0-9]{64}$/.test(fragment.assetKey))) && (fragment.face === undefined || (typeof fragment.face === 'string' && fragment.face.length > 0 && fragment.face.length <= MAX_CANVAS_PROPERTY_STRING)) && !(fragment.face !== undefined && fragment.assetKey !== undefined)
    })
  })
}
const isSnapshot = (value: unknown): value is EngineSnapshot => isRecord(value) && hasOnly(value, ['documentState', 'revision', 'byteLength', 'canUndo', 'canRedo', 'canvas']) && (value.documentState === 'empty' || value.documentState === 'loaded') && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision >= 0 && typeof value.byteLength === 'number' && Number.isSafeInteger(value.byteLength) && value.byteLength >= 0 && (value.canUndo === undefined || typeof value.canUndo === 'boolean') && (value.canRedo === undefined || typeof value.canRedo === 'boolean') && (value.canvas === undefined || isCanvas(value.canvas))

export function requestCorrelationId(value: unknown): string | undefined {
  return isRecord(value) && isEngineRequestId(value.requestId) ? value.requestId : undefined
}

export function parseRequest(value: unknown): EngineRequest | undefined {
  if (!isRecord(value) || !hasOnly(value, ['protocolVersion', 'kind', 'requestId', 'operation', 'payload']) || value.protocolVersion !== ENGINE_PROTOCOL_VERSION || value.kind !== 'request' || !isEngineRequestId(value.requestId)) return undefined
	if (!['initialize', 'load', 'snapshot', 'parameter-references', 'stand-in-data', 'group-move-preview', 'table-columns', 'validate', 'serialize', 'command', 'undo', 'redo', 'identity', 'render', 'asset'].includes(value.operation as string)) return undefined
  if (value.payload !== undefined && (!isArrayBuffer(value.payload) || value.payload.byteLength > MAX_ENGINE_PAYLOAD_BYTES) && !(value.operation === 'render' && isRenderPayload(value.payload)) && !(value.operation === 'identity' && isIdentityPayload(value.payload))) return undefined
	const needsPayload = value.operation === 'initialize' || value.operation === 'load' || value.operation === 'command' || value.operation === 'table-columns' || value.operation === 'group-move-preview' || value.operation === 'asset'
  if (value.operation === 'render' ? !isRenderPayload(value.payload) : value.operation === 'identity' ? !isIdentityPayload(value.payload) : needsPayload !== (value.payload !== undefined)) return undefined
  return value as EngineRequest
}

export function parseInbound(value: unknown): EngineInbound | undefined {
  if (!isRecord(value) || value.protocolVersion !== ENGINE_PROTOCOL_VERSION || typeof value.kind !== 'string') return undefined
  if (value.kind === 'lifecycle') {
    if ((hasExactKeys(value, ['protocolVersion', 'kind', 'state']) && value.state === 'ready') || (hasExactKeys(value, ['protocolVersion', 'kind', 'state', 'error']) && value.state === 'failed' && isError(value.error))) return value as EngineLifecycle
    return undefined
  }
  if (value.kind !== 'response' || !isEngineRequestId(value.requestId) || typeof value.ok !== 'boolean') return undefined
	if (value.ok && hasOnly(value, ['protocolVersion', 'kind', 'requestId', 'ok', 'snapshot', 'bytes', 'preview', 'parameterReferences', 'tableColumns', 'groupMove']) && isSnapshot(value.snapshot) && (value.bytes === undefined || isArrayBuffer(value.bytes) && value.bytes.byteLength <= MAX_ENGINE_RENDER_PDF_BYTES) && (value.preview === undefined || isPreview(value.preview)) && (value.parameterReferences === undefined || isParameterReferences(value.parameterReferences)) && (value.tableColumns === undefined || isTableColumns(value.tableColumns)) && (value.preview === undefined || value.preview.revision === value.snapshot.revision) && (value.parameterReferences === undefined || value.parameterReferences.revision === value.snapshot.revision) && (value.tableColumns === undefined || value.tableColumns.revision === value.snapshot.revision) && (value.groupMove === undefined || isGroupMove(value.groupMove) && value.groupMove.revision === value.snapshot.revision) && (value.preview?.pdfSha256 === undefined || value.bytes !== undefined)) return value as EngineSuccess
  if (!value.ok && hasExactKeys(value, ['protocolVersion', 'kind', 'requestId', 'ok', 'error']) && isError(value.error)) return value as EngineFailure
  return undefined
}

export function copyBytes(bytes: ArrayBuffer): ArrayBuffer { return bytes.slice(0) }

export function deepFreeze<T>(value: T): Readonly<T> {
  if (value && typeof value === 'object' && !Object.isFrozen(value)) {
    Object.freeze(value)
    for (const child of Object.values(value)) deepFreeze(child)
  }
  return value as Readonly<T>
}
