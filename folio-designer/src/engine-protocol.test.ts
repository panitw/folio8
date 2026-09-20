import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { ENGINE_PROTOCOL_VERSION, LOCALE_TAGS, MAX_CANVAS_BODY_TEXT_LINES, MAX_ENGINE_CONTENT_WINDOWS, MAX_ENGINE_FONT_CHAIN_ENTRIES, MAX_ENGINE_FONT_FAMILIES, MAX_CANVAS_PROPERTY_STRING, MAX_ENGINE_BINDING_LENGTH, MAX_ENGINE_DATA_PATH_LENGTH, MAX_ENGINE_ELEMENT_ID_LENGTH, MAX_ENGINE_FACE_BYTES, MAX_ENGINE_PAYLOAD_BYTES, MAX_ENGINE_RENDER_PDF_BYTES, deepFreeze, parseInbound, parseRequest } from './engine-protocol'

// face() builds the PROJECTED shape of a named-face chain entry (Story 8.3:
// an entry is a discriminated object, not a string). A named face carries no
// family and no style — its name is its identity.
const face = (name: string, variants: Partial<Readonly<{ bold: string; italic: string; boldItalic: string }>> = {}) => ({ face: name, assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '', ...variants })

const canvas = { width: 1000, height: 2000, orientation: 'portrait', preset: 'custom', locale: 'th', utcOffset: '+07:00', embedFonts: true, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, gridIncrement: 100, commandWidth: 1000, commandHeight: 2000, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 1800, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader', x: 0, y: 0, width: 1000, height: 100 }, { name: 'content', x: 0, y: 100, width: 1000, height: 1800 }, { name: 'pageFooter', x: 0, y: 1900, width: 1000, height: 100 }], components: [] }

describe('canvas projection protocol guard', () => {
  // STORY 12.2: THE DOCUMENT'S TWO DECLARED FORMATTING AUTHORITIES. The third
  // document setting, `embedFonts`, has its own `it` block directly below, on
  // the same terms and for the same reason.
  //
  // The projection gained `locale` and `utcOffset` so the PAGE SETUP panel can
  // show what the engine holds. `hasOnly` cannot carry them: it is a SUBSET
  // check, so a key Go simply failed to send passes it and arrives at the panel
  // as `undefined` — a locale row with no value, and an offset row that would
  // send the string "undefined" straight back to the engine. THE ABSENCE CASES
  // BELOW ARE THE ONLY THING THAT CATCHES THAT, and they are the failure this
  // story could otherwise have shipped in silence.
  it('requires both document-settings fields, and requires the locale to be one of AD-12\'s four tags', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    // The positive control first: the fixture as it stands is accepted, so
    // every rejection below is attributable to the patch and not to the base.
    expect(projection(canvas)).toBeDefined()
    // EVERY TAG IN THE CLOSED SET IS ACCEPTED, enumerated from LOCALE_TAGS
    // rather than written out again — a guard narrowed to one tag would blank
    // the canvas for the documents this story exists to make authorable.
    expect(LOCALE_TAGS.length).toBeGreaterThan(0)
    for (const tag of LOCALE_TAGS) expect(projection({ ...canvas, locale: tag })).toBeDefined()
    // ABSENT. Neither key can be caught by hasOnly.
    const { locale: _locale, ...noLocale } = canvas
    const { utcOffset: _offset, ...noOffset } = canvas
    expect(projection(noLocale)).toBeUndefined()
    expect(projection(noOffset)).toBeUndefined()
    // ILLEGAL. A tag outside AD-12's set — one Go's loader would refuse — and
    // an empty offset, which is what an emptied box would round-trip as if the
    // engine ever echoed one back.
    expect(projection({ ...canvas, locale: 'fr' })).toBeUndefined()
    expect(projection({ ...canvas, locale: 'EN' })).toBeUndefined()
    expect(projection({ ...canvas, locale: '' })).toBeUndefined()
    expect(projection({ ...canvas, locale: 7 })).toBeUndefined()
    expect(projection({ ...canvas, locale: null })).toBeUndefined()
    expect(projection({ ...canvas, utcOffset: '' })).toBeUndefined()
    expect(projection({ ...canvas, utcOffset: 7 })).toBeUndefined()
    expect(projection({ ...canvas, utcOffset: null })).toBeUndefined()
    expect(projection({ ...canvas, utcOffset: 'x'.repeat(MAX_CANVAS_PROPERTY_STRING + 1) })).toBeUndefined()
    // AND THE OFFSET'S GRAMMAR IS NOT RESTATED HERE. ±HH:MM is the engine's
    // rule, asked through the one predicate its loader and its command door
    // share; a browser-side copy could refuse a snapshot Go legitimately sent,
    // and the symptom would be a permanently blank canvas. So a value this side
    // cannot judge is ACCEPTED on shape alone.
    expect(projection({ ...canvas, utcOffset: '+99:99' })).toBeDefined()
    expect(projection({ ...canvas, utcOffset: 'Z' })).toBeDefined()
    // The extra-key direction, on the document settings' own account: a FOURTH
    // document-settings key Go started sending drops the snapshot.
    expect(projection({ ...canvas, timeZone: 'Asia/Bangkok' })).toBeUndefined()
  })

  // spec-font-sources-and-embedding CAP-2: the THIRD document setting, and it
  // needs its own clause for exactly the reason the two above do. `hasOnly` is
  // a subset check, so an `embedFonts` Go failed to send would reach the panel
  // as `undefined` — a checkbox painted UNCHECKED for a document that embeds,
  // which is the wrong answer stated confidently. Its type is its whole rule:
  // a bare boolean, no closed set and no syntax, so `true` and `false` are both
  // accepted and everything else is not.
  it('requires embedFonts, and requires it to be a boolean', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    expect(projection(canvas)).toBeDefined()
    // BOTH VALUES ARE LEGAL CONTENT. `false` is the authored state this
    // setting exists for, so a guard that admitted only `true` would blank the
    // canvas for precisely the documents the feature is for.
    expect(projection({ ...canvas, embedFonts: true })).toBeDefined()
    expect(projection({ ...canvas, embedFonts: false })).toBeDefined()
    // ABSENT — the case hasOnly cannot catch.
    const { embedFonts: _embed, ...noEmbed } = canvas
    expect(projection(noEmbed)).toBeUndefined()
    // AND NOT COERCED FROM ANYTHING THAT MERELY READS AS ONE.
    expect(projection({ ...canvas, embedFonts: 'true' })).toBeUndefined()
    expect(projection({ ...canvas, embedFonts: 'false' })).toBeUndefined()
    expect(projection({ ...canvas, embedFonts: 0 })).toBeUndefined()
    expect(projection({ ...canvas, embedFonts: 1 })).toBeUndefined()
    expect(projection({ ...canvas, embedFonts: null })).toBeUndefined()
  })

  // spec-section-break CAP-6: an OPTIONAL offset and per-content-component
  // membership, both from Go. hasOnly cannot see an absent key, so each has a
  // typed clause, and the pair must agree about whether there is a break.
  it('accepts the section break offset and membership only in the shapes Go projects', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const rect = (patch: object) => ({ id: 'e1', type: 'rect', band: 'content', x: 0, y: 0, width: 100, height: 10, resizable: true, ...patch })
    expect(projection({ ...canvas, sectionBreak: 900 })).toBeDefined()
    expect(projection({ ...canvas, sectionBreak: 900, components: [rect({ belowSectionBreak: false })] })).toBeDefined()
    expect(projection({ ...canvas, sectionBreak: 900, components: [rect({ belowSectionBreak: true, y: 950 })] })).toBeDefined()
    // Out of the content band, or not a safe integer.
    for (const offset of [0, -1, 1800, 2000, 12.5, '900', null]) expect(projection({ ...canvas, sectionBreak: offset })).toBeUndefined()
    // Membership without a break, a break without membership, membership off the content band.
    expect(projection({ ...canvas, components: [rect({ belowSectionBreak: false })] })).toBeUndefined()
    expect(projection({ ...canvas, sectionBreak: 900, components: [rect({})] })).toBeUndefined()
    expect(projection({ ...canvas, sectionBreak: 900, components: [rect({ band: 'pageHeader', belowSectionBreak: false })] })).toBeUndefined()
    expect(projection({ ...canvas, sectionBreak: 900, components: [rect({ belowSectionBreak: 'yes' })] })).toBeUndefined()
  })

  // spec-section-break CAP-7: the Anchor key is optional, present only as
  // `false`, and only beside a break.
  it('accepts sectionBreakAnchor only as false beside a break', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    expect(projection({ ...canvas, sectionBreak: 900, sectionBreakAnchor: false })).toBeDefined()
    expect(projection({ ...canvas, sectionBreakAnchor: false })).toBeUndefined()
    for (const anchor of [true, null, 'false', 0]) expect(projection({ ...canvas, sectionBreak: 900, sectionBreakAnchor: anchor })).toBeUndefined()
  })

  it('accepts and deeply freezes the exact three bounded bands', () => {
    const inbound = parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas } })
    expect(inbound).toBeDefined()
    const frozen = deepFreeze(canvas)
    expect(Object.isFrozen(frozen.bands)).toBe(true)
    expect(Object.isFrozen(frozen.bands[0])).toBe(true)
  })

  it.each([
    [{ ...canvas, bands: [canvas.bands[1], canvas.bands[0], canvas.bands[2]] }],
    [{ ...canvas, bands: [{ ...canvas.bands[0], name: 'content' }, ...canvas.bands.slice(1)] }],
    [{ ...canvas, bands: [{ ...canvas.bands[0], x: 1_000 }, ...canvas.bands.slice(1)] }],
    [{ ...canvas, width: Number.MAX_SAFE_INTEGER + 1 }],
  ])('rejects structurally false paint geometry', (bad) => {
    expect(parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: bad } })).toBeUndefined()
  })

  it.each([
    { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true }, { id: 'e1', type: 'rect', band: 'content', x: 20, y: 0, width: 10, height: 10, resizable: true }] },
    { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 991, y: 0, width: 10, height: 10, resizable: true }] },
    // The two REPEATING bands still cap vertically, and nothing in this file
    // exercised that before Story 7.5: every content component here sat at
    // y: 0, so the vertical conjunct was never reached and a Y-only lift
    // would have left the whole file green and vacuous.
    { ...canvas, components: [{ id: 'e1', type: 'rect', band: 'pageHeader', x: 0, y: 0, width: 10, height: 101, resizable: true }] },
    { ...canvas, components: [{ id: 'e1', type: 'rect', band: 'pageFooter', x: 0, y: 95, width: 10, height: 10, resizable: true }] },
    { ...canvas, components: [{ id: 'e1', type: 'table', band: 'content', x: 0, y: 0, width: 0, height: 10, resizable: true }] },
    { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: false }] },
  ])('rejects ambiguous, out-of-band, or incoherent component paint geometry', (bad) => {
    expect(parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: bad } })).toBeUndefined()
  })

  it('admits a content component below the foot of page one, and only in the content band', () => {
    // Story 7.5. The content band is a COLUMN: a component five windows down
    // is on a later page, not outside the document. Dropping the snapshot for
    // it would terminate the worker and blank the canvas with no attributable
    // error, so the browser's copy of the band-containment gate has to lift
    // with Go's — in the same commit, which engine-bounds-mirror.test.ts
    // reads both sides to enforce.
    const tall = { ...canvas, components: [{ id: 'e1', type: 'rect', band: 'content', x: 0, y: 9_000, width: 10, height: 10, resizable: true }] }
    expect(parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: tall } })).toBeDefined()
    // And the lift is vertical only: the column is unbounded downwards, never
    // sideways.
    const wide = { ...canvas, components: [{ id: 'e1', type: 'rect', band: 'content', x: 0, y: 9_000, width: 1_001, height: 10, resizable: true }] }
    expect(parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: wide } })).toBeUndefined()
  })

  it('requires the engine-owned window height and window count', () => {
    // WHICH GUARD DOES WHAT, because the two are easy to confuse and only one
    // of them is doing the work here. `hasOnly` is a SUBSET check — it rejects
    // keys the build does not know, not keys it is missing (`hasExactKeys` is
    // the strict sibling, and the canvas is not checked with it). What rejects
    // an OMITTED field is `integer(key, true)`, which reads `undefined` and
    // fails both `Number.isSafeInteger` and `> 0`. So both fields are required
    // and both must be strictly positive: a zero count is not a document,
    // since internal/layout answers ONE page for a column with no items.
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const { contentWindowCount: _count, ...noCount } = canvas
    const { contentWindowHeight: _height, ...noHeight } = canvas
    expect(projection(noCount)).toBeUndefined()
    // The second half, which the first version of this test never reached:
    // omitting the HEIGHT has to be refused on the same terms, or a Go build
    // that shipped one field and not the other would be admitted.
    expect(projection(noHeight)).toBeUndefined()
    for (const bad of [0, -1, 1.5, '4', null]) {
      expect(projection({ ...canvas, contentWindowCount: bad })).toBeUndefined()
      expect(projection({ ...canvas, contentWindowHeight: bad })).toBeUndefined()
    }
    expect(projection({ ...canvas, contentWindowCount: 4, contentWindowOrigins: [0, 1800, 3600, 5400], contentWindowPages: [0, 0, 0, 0] })).toBeDefined()
  })

  // Story 7.6. The origins are what the canvas draws every sheet boundary
  // from, so a malformed sequence is not a cosmetic problem: the browser
  // would either draw a boundary in the wrong place or, worse, derive one
  // itself. Every shape below is refused by the SAME path as any other
  // malformed projection field — parseInbound returns undefined and the
  // whole snapshot is discarded — which is what makes an honest Go field the
  // only way to get a drawing at all.
  it('requires one window origin per window, starting at zero and strictly increasing', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const three = { ...canvas, contentWindowCount: 3, contentWindowOrigins: [0, 1800, 5400], contentWindowPages: [0, 0, 0] }
    // The positive case first, so every rejection below is a discrimination
    // rather than a fixture that never parsed.
    expect(projection(three)).toBeDefined()
    // Windows do NOT have to be one window apart: the engine advances to the
    // top of the first item that did not fit, so a declared gap is a legal
    // and expected sequence. A validator that required a fixed stride would
    // be the forbidden closed form wearing a guard's clothes.
    expect(projection({ ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 7_280_000], contentWindowPages: [0, 0] })).toBeDefined()
    const { contentWindowOrigins: _origins, ...noOrigins } = canvas
    const { contentWindowCountIsExact: _exact, ...noExact } = canvas
    // Absent entirely. `hasOnly` is a subset check and says nothing about a
    // MISSING key; these two value predicates are the whole guard.
    expect(projection(noOrigins)).toBeUndefined()
    expect(projection(noExact)).toBeUndefined()
    // A nil Go slice marshals to null, not to [].
    expect(projection({ ...canvas, contentWindowOrigins: null })).toBeUndefined()
    expect(projection({ ...canvas, contentWindowOrigins: 1 })).toBeUndefined()
    expect(projection({ ...canvas, contentWindowOrigins: [], contentWindowPages: [] })).toBeUndefined()
    // Wrong length, both directions.
    expect(projection({ ...three, contentWindowOrigins: [0, 1800], contentWindowPages: [0, 0] })).toBeUndefined()
    expect(projection({ ...three, contentWindowOrigins: [0, 1800, 5400, 9000], contentWindowPages: [0, 0, 0, 0] })).toBeUndefined()
    // Not starting at zero: window one begins at the top of the column,
    // unconditionally, and internal/layout guarantees it.
    expect(projection({ ...three, contentWindowOrigins: [900, 1800, 5400], contentWindowPages: [0, 0, 0] })).toBeUndefined()
    // Not increasing, and not strictly increasing.
    expect(projection({ ...three, contentWindowOrigins: [0, 5400, 1800], contentWindowPages: [0, 0, 0] })).toBeUndefined()
    expect(projection({ ...three, contentWindowOrigins: [0, 1800, 1800], contentWindowPages: [0, 0, 0] })).toBeUndefined()
    // Entries that are not safe non-negative integers.
    for (const bad of [-1, 1.5, '1800', null, Number.MAX_SAFE_INTEGER + 1]) {
      expect(projection({ ...three, contentWindowOrigins: [0, 1800, bad], contentWindowPages: [0, 0, 0] })).toBeUndefined()
    }
    // The honesty flag is a boolean and nothing else — never a truthy string
    // a disclosure would then render. 0 and '' matter twice over here: the
    // sense is inverted from the field this replaced, so a falsy non-boolean
    // slipping through would read as "do not trust this count" on a document
    // that is exact.
    for (const bad of [0, 1, 'true', null, '']) {
      expect(projection({ ...canvas, contentWindowCountIsExact: bad })).toBeUndefined()
    }
    expect(projection({ ...canvas, contentWindowCountIsExact: false })).toBeDefined()
    // The declared cap, at its edge on both sides.
    const long = (count: number) => ({ ...canvas, contentWindowCount: count, contentWindowOrigins: Array.from({ length: count }, (_value, index) => index * 1800), contentWindowPages: Array.from({ length: count }, () => 0) })
    expect(projection(long(MAX_ENGINE_CONTENT_WINDOWS))).toBeDefined()
    expect(projection(long(MAX_ENGINE_CONTENT_WINDOWS + 1))).toBeUndefined()
  })

  // SPEC-multi-pages CAP-8. Every window names its designed page, and origins
  // restart at 0 at each page's first window.
  it('accepts page-local window origins grouped by designed page', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const pages = { ...canvas, contentWindowCount: 4, contentWindowOrigins: [0, 1800, 3600, 0], contentWindowPages: [0, 0, 0, 1], pageBreaks: [true, true], sectionBreaks: [null, null], sectionBreakAnchors: [true, true] }
    expect(projection(pages)).toBeDefined()
    // An empty later page is one window of its own.
    expect(projection({ ...canvas, contentWindowCount: 3, contentWindowOrigins: [0, 0, 0], contentWindowPages: [0, 1, 2], pageBreaks: [true, false, true], sectionBreaks: [null, null, null], sectionBreakAnchors: [true, true, true] })).toBeDefined()
    const { contentWindowPages: _pages, ...noPages } = canvas
    expect(projection(noPages)).toBeUndefined()
    expect(projection({ ...pages, contentWindowPages: null })).toBeUndefined()
    // One entry per window.
    expect(projection({ ...pages, contentWindowPages: [0, 0, 1] })).toBeUndefined()
    // Pages start at 0 and never skip or go back.
    expect(projection({ ...pages, contentWindowPages: [1, 1, 1, 2] })).toBeUndefined()
    expect(projection({ ...pages, contentWindowPages: [0, 0, 0, 2] })).toBeUndefined()
    expect(projection({ ...pages, contentWindowOrigins: [0, 0, 1800, 0], contentWindowPages: [0, 1, 0, 1] })).toBeUndefined()
    // A page's first window starts at 0; a page's later windows rise strictly.
    expect(projection({ ...pages, contentWindowOrigins: [0, 1800, 3600, 900], contentWindowPages: [0, 0, 0, 1] })).toBeUndefined()
    expect(projection({ ...pages, contentWindowOrigins: [0, 1800, 1800, 0], contentWindowPages: [0, 0, 0, 1] })).toBeUndefined()
    for (const bad of [-1, 1.5, '1', null]) {
      expect(projection({ ...pages, contentWindowPages: [0, 0, 0, bad] })).toBeUndefined()
    }
  })

  // SPEC-multi-pages story 2: each page's Page Break and each component's page.
  it('accepts one Page Break per designed page and a component page that names an existing page', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const two = { ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 0], contentWindowPages: [0, 1], pageBreaks: [true, false], sectionBreaks: [null, null], sectionBreakAnchors: [true, true] }
    const content = (page: unknown) => ({ id: 'e1', type: 'rect', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, page })
    expect(projection(two)).toBeDefined()
    expect(projection({ ...two, components: [content(1)] })).toBeDefined()
    // One page may omit both (every existing one-page projection); Go sends them.
    expect(projection({ ...canvas, pageBreaks: [true] })).toBeDefined()
    expect(projection({ ...canvas, components: [content(0)] })).toBeDefined()
    // A multi-page projection must carry both.
    const { pageBreaks: _breaks, ...noBreaks } = two
    expect(projection(noBreaks)).toBeUndefined()
    expect(projection({ ...two, components: [{ ...content(1), page: undefined }] })).toBeUndefined()
    // One entry per page, booleans, page 1's always true.
    for (const bad of [[true], [true, false, true], [false, false], [true, 'false'], null]) expect(projection({ ...two, pageBreaks: bad })).toBeUndefined()
    // A page must exist, and a header or footer component is always page 0.
    for (const bad of [2, -1, 0.5, '1', null]) expect(projection({ ...two, components: [content(bad)] })).toBeUndefined()
    expect(projection({ ...two, components: [{ ...content(1), band: 'pageHeader' }] })).toBeUndefined()
    expect(projection({ ...two, components: [{ ...content(0), band: 'pageHeader' }] })).toBeDefined()
  })

  // SPEC-multi-pages story 5: a multi-page projection carries one break per
  // page instead of the one-page pair, and membership follows each content
  // component's OWN page.
  it('accepts per-page breaks only on a multi-page projection, with membership on the pages that have one', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const rect = (id: string, page: number, extra: object = {}) => ({ id, type: 'rect', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, page, ...extra })
    const two = (patch: object = {}) => ({ ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 0], contentWindowPages: [0, 1], pageBreaks: [true, true], sectionBreaks: [null, 400], sectionBreakAnchors: [true, false], components: [], ...patch })
    expect(projection(two())).toBeDefined()
    expect(projection(two({ components: [rect('e1', 0), rect('e2', 1, { belowSectionBreak: true, y: 500 })] }))).toBeDefined()
    // Membership on a page without a break, or missing on a page with one.
    expect(projection(two({ components: [rect('e1', 0, { belowSectionBreak: false })] }))).toBeUndefined()
    expect(projection(two({ components: [rect('e2', 1)] }))).toBeUndefined()
    // Both per-page lists are required on a multi-page projection.
    const { sectionBreaks: _breaks, ...noBreaks } = two()
    expect(projection(noBreaks)).toBeUndefined()
    const { sectionBreakAnchors: _anchors, ...noAnchors } = two()
    expect(projection(noAnchors)).toBeUndefined()
    // One entry per page, each null or strictly inside the content band.
    for (const bad of [[null], [null, null, null], [0, null], [null, 1800], [null, 12.5], [null, '400'], null]) expect(projection(two({ sectionBreaks: bad }))).toBeUndefined()
    // One boolean per page, false only beside a break.
    for (const bad of [[false, true], [true], [true, 'false'], [true, null], null]) expect(projection(two({ sectionBreakAnchors: bad }))).toBeUndefined()
    // Never the one-page pair on a multi-page projection, nor the reverse.
    expect(projection(two({ sectionBreak: 400 }))).toBeUndefined()
    expect(projection(two({ sectionBreakAnchor: false }))).toBeUndefined()
    expect(projection({ ...canvas, sectionBreaks: [null], sectionBreakAnchors: [true] })).toBeUndefined()
  })

  // STORY 8.1. fontChains is the first projection field that carries the
  // document's font MAP rather than a name list, and both of its failure modes
  // are silent: hasOnly is a SUBSET check, so a key Go sends and this file does
  // not list drops the whole snapshot and blanks the canvas with no
  // attributable error; and a fontChains/fontFamilies disagreement would let
  // the chain editor offer a name the engine does not hold. Both are measured
  // here, not assumed.
  it('accepts the projected font chains only when they agree with fontFamilies', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const two = { ...canvas, fontFamilies: ['body', 'heading'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }, { name: 'heading', entries: [face('Noto Sans'), face('Noto Sans Thai')] }] }
    expect(projection(two)).toBeDefined()
    // THE KEY, BOTH WAYS. Go omits it; and Go sends it while the guard's own
    // hasOnly list does not name it — the second direction is asserted against
    // the real list by reading engine-protocol.ts in
    // canvas_projection_wire_test.go, and here by the extra-key case below.
    const { fontChains: _chains, ...noChains } = canvas
    expect(projection(noChains)).toBeUndefined()
    expect(projection({ ...canvas, extraProjectionKey: 1 })).toBeUndefined()
    // And the POSITIVE case the extra-key assertion used to be conflated
    // with: a document declaring `"fonts": {}` — the component-asset-import
    // and image-embed fixtures both do — projects no chains and no families,
    // and that is VALID. Carrying the extra key made that assertion pass on
    // the key alone, so it said nothing either way about a zero-chain
    // projection, and a guard that rejected one would have gone unnoticed.
    expect(projection({ ...canvas, fontChains: [], fontFamilies: [] })).toBeDefined()
    // Disagreement with fontFamilies, in each of its three shapes: a different
    // name, a different length, and the same names in a different order.
    expect(projection({ ...canvas, fontChains: [{ name: 'brand', entries: [face('Noto Sans')] }] })).toBeUndefined()
    expect(projection({ ...two, fontChains: [two.fontChains[0]] })).toBeUndefined()
    expect(projection({ ...two, fontChains: [two.fontChains[1], two.fontChains[0]] })).toBeUndefined()
    // A chain with no entries is not one Go projects, because it is not one
    // style.fontFamily may name.
    expect(projection({ ...canvas, fontChains: [{ name: 'body', entries: [] }] })).toBeUndefined()
    expect(projection({ ...canvas, fontChains: [{ name: 'body', entries: [face('')] }] })).toBeUndefined()
    // Shape and bounds.
    expect(projection({ ...canvas, fontChains: null })).toBeUndefined()
    expect(projection({ ...canvas, fontChains: [{ name: 'body' }] })).toBeUndefined()
    expect(projection({ ...canvas, fontChains: [{ name: 'body', entries: [face('Noto Sans')], extra: 1 }] })).toBeUndefined()
    expect(projection({ ...canvas, fontChains: [{ name: 'body', entries: [7] }] })).toBeUndefined()
    const entries = (count: number) => ({ ...canvas, fontChains: [{ name: 'body', entries: Array.from({ length: count }, (_value, index) => face(`face-${index}`)) }] })
    expect(projection(entries(MAX_ENGINE_FONT_CHAIN_ENTRIES))).toBeDefined()
    expect(projection(entries(MAX_ENGINE_FONT_CHAIN_ENTRIES + 1))).toBeUndefined()
    expect(projection({ ...canvas, fontChains: [{ name: 'body', entries: [face('f'.repeat(MAX_CANVAS_PROPERTY_STRING))] }] })).toBeDefined()
    expect(projection({ ...canvas, fontChains: [{ name: 'body', entries: [face('f'.repeat(MAX_CANVAS_PROPERTY_STRING + 1))] }] })).toBeUndefined()
    // The count bound the mirror test ties to Go's maxCanvasFontFamilies, at
    // its edge — fontFamilies and fontChains cross the boundary together.
    const families = (count: number) => {
      const names = Array.from({ length: count }, (_value, index) => `f${String(index).padStart(6, '0')}`)
      return { ...canvas, fontFamilies: names, fontChains: names.map((name) => ({ name, entries: [face('Noto Sans')] })) }
    }
    expect(projection(families(MAX_ENGINE_FONT_FAMILIES))).toBeDefined()
    expect(projection(families(MAX_ENGINE_FONT_FAMILIES + 1))).toBeUndefined()
  })

  // Story 8.3. A chain entry is a DISCRIMINATED OBJECT, not a string. The Go
  // projection and this guard changed in one commit for the usual reason: the
  // old `typeof face === 'string'` clause rejected an object entry outright,
  // isCanvas returned false, parseInbound returned undefined, engine-client
  // terminated the worker, and the canvas was permanently blank with nothing
  // to attribute it to.
  it('accepts the projected chain ENTRY only in its discriminated shape', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const chain = (...entries: ReadonlyArray<unknown>) => ({ ...canvas, fontChains: [{ name: 'body', entries }] })
    const key = 'c'.repeat(64)
    // STORY 11.3: an entry carries its three DECLARED variants too, always
    // present, '' for absent. `carried` spells the embedded shape once so the
    // rows below vary one thing each.
    const carried = (patch: object = {}) => ({ face: '', assetKey: key, family: 'Inter', style: 'Regular', bold: '', italic: '', boldItalic: '', ...patch })

    // Both legal shapes, and a chain mixing them.
    expect(projection(chain(face('Noto Sans')))).toBeDefined()
    expect(projection(chain(carried()))).toBeDefined()
    expect(projection(chain(carried({ style: '' })))).toBeDefined()
    expect(projection(chain(face('Noto Sans'), carried()))).toBeDefined()

    // Not an object at all: the shapes the pre-8.3 wire could carry, and the
    // ones a hostile or stale sender might.
    expect(projection(chain('Noto Sans'))).toBeUndefined()
    expect(projection(chain(7))).toBeUndefined()
    expect(projection(chain(null))).toBeUndefined()
    expect(projection(chain([face('Noto Sans')]))).toBeUndefined()

    // The key set is EXACT, both directions: a key Go stops sending fails as
    // surely as a key Go starts sending.
    expect(projection(chain({ assetKey: key, family: 'Inter', style: 'Regular', bold: '', italic: '', boldItalic: '' }))).toBeUndefined()
    expect(projection(chain({ ...face('Noto Sans'), weight: 700 }))).toBeUndefined()
    expect(projection(chain({ face: 'Noto Sans', assetKey: '', family: '' }))).toBeUndefined()
    // STORY 11.3's THREE, each missing on its own. They are ALWAYS-PRESENT
    // keys precisely because this guard is hasExactKeys: an entry key Go sends
    // for only SOME entries rejects the whole snapshot for exactly those
    // documents, and the symptom is a blank canvas.
    for (const absent of ['bold', 'italic', 'boldItalic'] as const) {
      const { [absent]: _dropped, ...rest } = face('Noto Sans')
      expect(projection(chain(rest)), absent).toBeUndefined()
    }

    // Every value is a string.
    expect(projection(chain({ ...face('Noto Sans'), assetKey: null }))).toBeUndefined()
    expect(projection(chain(carried({ family: 7 })))).toBeUndefined()
    expect(projection(chain({ ...face('Noto Sans'), bold: 7 }))).toBeUndefined()
    expect(projection(chain({ ...face('Noto Sans'), italic: null }))).toBeUndefined()
    expect(projection(chain({ ...face('Noto Sans'), boldItalic: true }))).toBeUndefined()

    // EXACTLY ONE of face and assetKey. Neither is an entry of no kind;
    // both is an entry of two.
    expect(projection(chain(face('')))).toBeUndefined()
    expect(projection(chain(carried({ face: 'Noto Sans', style: '' })))).toBeUndefined()

    // A named face carries no display strings — its name IS its identity —
    // and an embedded one always carries a family, because Go falls back to
    // the asset key rather than sending a name the panel cannot draw.
    expect(projection(chain({ ...face('Noto Sans'), family: 'Inter' }))).toBeUndefined()
    expect(projection(chain({ ...face('Noto Sans'), style: 'Regular' }))).toBeUndefined()
    expect(projection(chain(carried({ family: '' })))).toBeUndefined()

    // STORY 11.3 / DW-239 — THE DECLARED VARIANTS ARE ADMITTED, NOT
    // ADJUDICATED, and every combination of present and absent is legal: a
    // family with a bold and no italic is the SHIPPED condition (Noto Sans
    // Thai), not a corner case.
    expect(projection(chain(face('Roboto', { bold: 'Roboto Bold', italic: 'Roboto Italic', boldItalic: 'Roboto Bold Italic' })))).toBeDefined()
    expect(projection(chain(face('Noto Sans Thai', { bold: 'Noto Sans Thai Bold' })))).toBeDefined()
    expect(projection(chain(face('Noto Sans SC')))).toBeDefined()
    // AN EMBEDDED ENTRY'S VARIANT IS AN ASSETS KEY (AD-8), and the guard reads
    // no shape into it: a 64-character face name is a legal face name, so
    // "looks like a digest" was never available as a test — the DISCRIMINANT is
    // what says which namespace the value is in, and a variant is admitted on
    // either kind of entry.
    expect(projection(chain(carried({ bold: 'd'.repeat(64) })))).toBeDefined()
    expect(projection(chain(carried({ bold: 'Second Sans Bold' })))).toBeDefined()

    // The per-string bound applies to EVERY projected string, not only the
    // face name — a bound on three of four fields is a bound on nothing.
    const long = 'f'.repeat(MAX_CANVAS_PROPERTY_STRING + 1)
    expect(projection(chain(carried({ family: long, style: '' })))).toBeUndefined()
    expect(projection(chain(carried({ style: long })))).toBeUndefined()
    expect(projection(chain(carried({ assetKey: long })))).toBeUndefined()
    // …AND TO THE THREE NEW ONES. A bound on four of seven fields is a bound
    // on nothing, which is the sentence the Go projection's own comment makes.
    expect(projection(chain(face('Noto Sans', { bold: long })))).toBeUndefined()
    expect(projection(chain(face('Noto Sans', { italic: long })))).toBeUndefined()
    expect(projection(chain(face('Noto Sans', { boldItalic: long })))).toBeUndefined()
    expect(projection(chain(face('Noto Sans', { bold: 'f'.repeat(MAX_CANVAS_PROPERTY_STRING) })))).toBeDefined()
  })

  // DW-70. Go sorts these keys with slices.Sorted over Go strings — BY BYTE —
  // and those keys are the canonical `.folio`'s own `fonts` key order under
  // AD-9, so Go's order IS the document's and is NORMATIVE. The guard used
  // `>=` on JavaScript strings, which compares UTF-16 CODE UNITS, and the two
  // disagree wherever a name mixes the astral planes with U+E000-U+FFFF: a
  // surrogate pair sorts BELOW U+E000 in UTF-16 and ABOVE it in UTF-8.
  //
  // The consequence was not a dropped frame. isCanvas false makes parseInbound
  // return undefined, which engine-client raises as PROTOCOL_INVALID, which
  // TERMINATES the worker and leaves the canvas permanently blank. Story 8.2
  // is what lets an author type a chain name, so two keystrokes reached it.
  it('accepts the projected chain names in Go\'s byte order, not the browser\'s UTF-16 order', () => {
    const projection = (patch: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: patch } })
    const ordered = (...names: ReadonlyArray<string>) => ({ ...canvas, fontFamilies: names, fontChains: names.map((name) => ({ name, entries: [face('Noto Sans')] })) })
    // THE MEASURED PAIR. '\uE000' is EE 80 80 and '\u{1F600}' is F0 9F 98 80,
    // so Go sends them in this order. In UTF-16 the emoji begins 0xD83D, which
    // is BELOW 0xE000 — so `>=` called this pair out of order and dropped the
    // whole snapshot.
    expect(projection(ordered('\uE000', '\u{1F600}'))).toBeDefined()
    // And the reverse pair is still rejected, so the fix widened the accepted
    // set rather than removing the check: a genuinely out-of-order projection
    // is a channel fault and is still not trusted.
    expect(projection(ordered('\u{1F600}', '\uE000'))).toBeUndefined()
    // The ordinary cases the check has always covered, both ways.
    expect(projection(ordered('body', 'heading'))).toBeDefined()
    expect(projection(ordered('heading', 'body'))).toBeUndefined()
    // Equal names are neither ascending nor unique.
    expect(projection(ordered('body', 'body'))).toBeUndefined()
    // A prefix sorts before its extension in both orders, and does here.
    expect(projection(ordered('body', 'bodyweight'))).toBeDefined()
    expect(projection(ordered('bodyweight', 'body'))).toBeUndefined()
    // A second astral pair, on the other side of the boundary: U+FFFD (EF BF
    // BD) still precedes the emoji in byte order.
    expect(projection(ordered('\uFFFD', '\u{1F600}'))).toBeDefined()
    expect(projection(ordered('\u{1F600}', '\uFFFD'))).toBeUndefined()
  })

  it('bounds opaque producer failure provenance at the main-thread boundary', () => {
    const response = (error: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: false, error })
    expect(response({ code: 'COMPONENT_INVALID', message: 'invalid', elementId: 'e'.repeat(MAX_ENGINE_ELEMENT_ID_LENGTH), dataPath: 'p'.repeat(MAX_ENGINE_DATA_PATH_LENGTH) })).toBeDefined()
    expect(response({ code: 'COMPONENT_INVALID', message: 'invalid', elementId: 'e'.repeat(MAX_ENGINE_ELEMENT_ID_LENGTH + 1) })).toBeUndefined()
    expect(response({ code: 'COMPONENT_INVALID', message: 'invalid', dataPath: 'p'.repeat(MAX_ENGINE_DATA_PATH_LENGTH + 1) })).toBeUndefined()
    expect(response({ code: 'C'.repeat(97), message: 'invalid' })).toBeUndefined()
    expect(response({ code: 'COMPONENT_INVALID', message: 'm'.repeat(513) })).toBeUndefined()
    expect(response({ code: 'COMPONENT_INVALID', message: '' })).toBeUndefined()
    expect(response({ code: 'COMPONENT_INVALID', message: 'invalid', elementId: '' })).toBeUndefined()
    expect(response({ code: 'COMPONENT_INVALID', message: 'invalid', dataPath: '' })).toBeUndefined()
    expect(response({ code: 'COMPONENT_INVALID', message: 'invalid', elementId: 7 })).toBeUndefined()
    expect(response({ code: 'COMPONENT_INVALID', message: 'invalid', dataPath: [] })).toBeUndefined()
  })

  it('rejects surplus authority-bearing fields at every projection level', () => {
    const response = (bad: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: bad } })
    expect(response({ ...canvas, style: {} })).toBeUndefined()
    expect(response({ ...canvas, bands: [{ ...canvas.bands[0], extra: true }, ...canvas.bands.slice(1)] })).toBeUndefined()
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, Extra: {} }] })).toBeUndefined()
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'image', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, fontFamily: 'body' }] })).toBeUndefined()
  })

  it('accepts only bounded, ordered engine text paint and rejects browser-shaped substitutes', () => {
    const textPaint = { overflow: false, truncated: false, lines: [{ top: 0, baseline: 8, advance: 12, width: 10, fragments: [{ text: 'engine line', x: 0 }] }] }
    const response = (projection: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: projection } })
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint }] })).toBeDefined()
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint: { ...textPaint, viewportWidth: 100 } }] })).toBeUndefined()
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint: { ...textPaint, lines: [{ ...textPaint.lines[0], width: 11 }] } }] })).toBeUndefined()
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint: { ...textPaint, lines: [{ ...textPaint.lines[0], fragments: [{ text: 'engine line', x: 0, fontMetrics: 1 }] }] } }] })).toBeUndefined()
  })

  // STORY 8.4a. A fragment may carry the ASSET KEY of the face the engine
  // resolved it to, and the key is OPTIONAL: its absence is the projection's
  // own statement that this fragment is a SHIPPED face, so both shapes have to
  // be admitted and the optional one has to be proved optional.
  //
  // WHAT A WRONG ANSWER COSTS HERE, and it is why the shape is checked rather
  // than merely typed. `hasOnly` rejects a key it does not list, isCanvas then
  // fails, parseInbound returns undefined, and engine-client raises
  // PROTOCOL_INVALID — which TERMINATES the worker and rejects every pending
  // request. Not a blank canvas: a dead session.
  it('admits a paint fragment attributed to a carried face, and one attributed to none', () => {
    const key = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
    const response = (fragment: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 8, advance: 12, width: 10, fragments: [fragment] }] } }] } } })
    expect(response({ text: 'engine line', x: 0, assetKey: key })).toBeDefined()
    // THE SHIPPED-FACE PATH: no key at all, which is the common case and the
    // one that must not have become mandatory.
    expect(response({ text: 'engine line', x: 0 })).toBeDefined()
    // An explicit `undefined` is the same statement, and is what a projection
    // reconstructed in JavaScript will hand this guard.
    expect(response({ text: 'engine line', x: 0, assetKey: undefined })).toBeDefined()
    // AND THE KEY IS THE FORMAT'S OWN SHAPE — 64 lowercase hex characters, the
    // same rule the image projection's key is held to. Anything else is a
    // producer that has drifted, not an older one to tolerate: the browser
    // hands this string straight back to the `asset` operation and derives a
    // CSS family from it.
    expect(response({ text: 'engine line', x: 0, assetKey: '' })).toBeUndefined()
    expect(response({ text: 'engine line', x: 0, assetKey: key.toUpperCase() })).toBeUndefined()
    expect(response({ text: 'engine line', x: 0, assetKey: key.slice(0, 63) })).toBeUndefined()
    expect(response({ text: 'engine line', x: 0, assetKey: 'body' })).toBeUndefined()
    expect(response({ text: 'engine line', x: 0, assetKey: 7 })).toBeUndefined()
  })

  // STORY 8.4e. A fragment may instead carry the ENGINE'S OWN FontSet NAME for
  // the SHIPPED face it was measured with — the other half of the same
  // attribution, and `assetKey`'s mutually exclusive twin. The pair
  // discriminates exactly as a chain ENTRY's `face`/`assetKey` pair does one
  // level up, with one deliberate difference: NEITHER is legal here, because
  // the absence of both is the wire's statement that the projection did not
  // attribute this fragment, and such a fragment must still paint on the
  // stylesheet's declared stack rather than kill the session.
  //
  // THE BOUND IS CHECKED HERE EVEN THOUGH GO CANNOT BREACH IT TODAY. The
  // engine can only put a key of the FontSet it was given on this field; that
  // is the ENGINE's guarantee, and a guard's job is to hold when the other
  // side is wrong. The value is written into an inline `font-family`
  // declaration on the browser, so an unbounded string here is an unbounded
  // string in a stylesheet.
  it('admits a paint fragment attributed to a shipped face, and refuses one carrying both identities', () => {
    const key = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
    const response = (fragment: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 8, advance: 12, width: 10, fragments: [fragment] }] } }] } } })
    expect(response({ text: 'engine line', x: 0, face: 'Noto Sans Thai' })).toBeDefined()
    expect(response({ text: 'engine line', x: 0, face: undefined })).toBeDefined()
    // THE SAME BOUND A CHAIN ENTRY'S `face` ALREADY USES — no new numeral, and
    // both of its directions.
    expect(response({ text: 'engine line', x: 0, face: 'f'.repeat(MAX_CANVAS_PROPERTY_STRING) })).toBeDefined()
    expect(response({ text: 'engine line', x: 0, face: 'f'.repeat(MAX_CANVAS_PROPERTY_STRING + 1) })).toBeUndefined()
    // An empty string is not an absence: absence is spelled by omission, and a
    // producer sending '' has drifted rather than said anything.
    expect(response({ text: 'engine line', x: 0, face: '' })).toBeUndefined()
    expect(response({ text: 'engine line', x: 0, face: 7 })).toBeUndefined()
    expect(response({ text: 'engine line', x: 0, face: null })).toBeUndefined()
    // EXACTLY ONE OF THE TWO. Both is a producer contradicting itself about
    // which face drew this fragment, and the browser would have to pick.
    expect(response({ text: 'engine line', x: 0, face: 'Noto Sans', assetKey: key })).toBeUndefined()
    // NEITHER IS LEGAL, deliberately — see above. Re-asserted here so the
    // exclusivity is never tightened into a requirement by accident.
    expect(response({ text: 'engine line', x: 0 })).toBeDefined()
  })

  // Story 7.3 / FR47. The alignment vocabulary is TWO closed sets on this
  // boundary as well as in Go: a COMPONENT may be justified, a table
  // COLUMN may not. The validator gates the projection — an unrecognised
  // value drops the whole response — so a justified document that this
  // check refused would blank the entire canvas rather than merely lose
  // its alignment.
  it('admits a justified component, refuses a justified table column, and accepts word-grained fragments', () => {
    const response = (projection: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: projection } })
    const emptyPaint = { overflow: false, truncated: false, lines: [] }
    const component = (align: string) => ({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, align, textPaint: emptyPaint }] })
    for (const align of ['left', 'center', 'right', 'justify']) expect(response(component(align))).toBeDefined()
    for (const align of ['middle', 'JUSTIFY', 'flush', '']) expect(response(component(align))).toBeUndefined()

    // The COLUMN set stays the triple, on its own projection.
    const columns = (align: string) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'table-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, tableColumns: { revision: 7, table: { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'rows[]', alias: 'row', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align, headerAlign: '' as const, headerAlignResolved: 'left' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '', footerOf: '', footerFormat: '' }] } } })
    expect(columns('right')).toBeDefined()
    expect(columns('justify')).toBeUndefined()

    // A justified line arrives as SEVERAL fragments with ascending x — the
    // engine positions each word; the browser never justifies anything.
    const wordGrained = { overflow: false, truncated: false, lines: [{ top: 0, baseline: 8, advance: 12, width: 10, fragments: [{ text: 'one', x: 0 }, { text: ' two', x: 4 }, { text: ' three', x: 8 }] }] }
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, align: 'justify', textPaint: wordGrained }] })).toBeDefined()
    // …and a fragment placed outside the component's own box is still
    // refused, word-grained or not.
    expect(response({ ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, align: 'justify', textPaint: { ...wordGrained, lines: [{ ...wordGrained.lines[0], fragments: [...wordGrained.lines[0].fragments, { text: 'far', x: 11 }] }] } }] })).toBeUndefined()
  })

  // Story 7.4 / DW-25. The three places a body-text projection could still be
  // dropped silently: the exact-key `hasOnly` on the paint, the split
  // `optionalString`, and the line bound. Each of these failures blanks the
  // WHOLE canvas — isTextPaint false fails the component, which fails
  // isCanvas, isSnapshot and finally parseInbound — so there is no
  // attributable error, only a designer with no snapshot.
  it('admits a truncated prefix paint, a clause past the identifier bound, and a projected line spacing', () => {
    const response = (component: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [component] } } })
    const line = { top: 0, baseline: 8, advance: 12, width: 10, fragments: [{ text: 'engine line', x: 0 }] }
    const text = (extra: object) => ({ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, ...extra })

    // A PREFIX with the flag set is the degraded state, and it is admitted.
    expect(response(text({ textPaint: { overflow: false, truncated: true, lines: [line] } }))).toBeDefined()
    // Both flags are required, exactly as `overflow` always was: a producer
    // that has stopped emitting one has drifted from this contract.
    expect(response(text({ textPaint: { overflow: false, lines: [line] } }))).toBeUndefined()
    expect(response(text({ textPaint: { overflow: false, truncated: 'yes', lines: [line] } }))).toBeUndefined()
    // And an unknown key still drops the response — hasOnly is exact-key.
    expect(response(text({ textPaint: { overflow: false, truncated: false, clipped: true, lines: [line] } }))).toBeUndefined()

    // THE FOURTH MIRROR. An element's value is BODY TEXT and no longer shares
    // the identifier bound; the seven identifier keys still keep it.
    const clause = 'x'.repeat(MAX_CANVAS_PROPERTY_STRING + 1)
    expect(response(text({ value: clause, textPaint: { overflow: false, truncated: false, lines: [] } }))).toBeDefined()
    expect(response(text({ fontFamily: clause, textPaint: { overflow: false, truncated: false, lines: [] } }))).toBeUndefined()
    expect(response(text({ color: clause, textPaint: { overflow: false, truncated: false, lines: [] } }))).toBeUndefined()

    // style.lineSpacing, projected for the first time: thousandths, inside
    // the range the engine's one validator enforces on both entry points.
    expect(response(text({ lineSpacing: 1500, textPaint: { overflow: false, truncated: false, lines: [] } }))).toBeDefined()
    expect(response(text({ lineSpacing: 0, textPaint: { overflow: false, truncated: false, lines: [] } }))).toBeUndefined()
    expect(response(text({ lineSpacing: 1000001, textPaint: { overflow: false, truncated: false, lines: [] } }))).toBeUndefined()
    expect(response(text({ lineSpacing: '1.5', textPaint: { overflow: false, truncated: false, lines: [] } }))).toBeUndefined()
  })

  it('admits a forty-page paint and refuses one line past the mirrored bound', () => {
    const response = (count: number) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 1800, resizable: true, textPaint: { overflow: false, truncated: false, lines: Array.from({ length: count }, (_value, index) => ({ top: index, baseline: index, advance: 1, width: 10, fragments: [] })) } }] } } })
    expect(response(MAX_CANVAS_BODY_TEXT_LINES)).toBeDefined()
    expect(response(MAX_CANVAS_BODY_TEXT_LINES + 1)).toBeUndefined()
    // The old bound must no longer bite: 257 lines is about six pages, and
    // refusing them here is what blanked the canvas after a paste.
    expect(response(257)).toBeDefined()
  })

  it('admits one bounded text-binding paint label but rejects an editable projection', () => {
    const response = (component: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [component] } } })
    const text = { id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, binding: 'customer.name', textPaint: { overflow: false, truncated: false, lines: [] } }
    expect(response(text)).toBeDefined()
    expect(response({ ...text, binding: 'a'.repeat(MAX_ENGINE_BINDING_LENGTH + 1) })).toBeUndefined()
    expect(response({ ...text, binding: '' })).toBeUndefined()
    expect(response({ ...text, binding: { path: 'customer.name' } })).toBeUndefined()
    expect(response({ ...text, type: 'image' })).toBeUndefined()
  })

  it('admits a barcode with Go-computed bars, a value and a binding, and refuses malformed barcode paint', () => {
    const response = (component: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [component] } } })
    // Inside this fixture's 1000 mp band, so the geometry guard admits the box
    // and every refusal below is the barcode rule's doing.
    const barcode = { id: 'e1', type: 'barcode', band: 'content', x: 0, y: 0, width: 900, height: 500, resizable: true }
    const barcodePaint = { moduleWidth: 10, bars: [{ x: 100, width: 20 }, { x: 130, width: 10 }] }
    expect(response(barcode)).toBeDefined()
    expect(response({ ...barcode, value: '|0994000123456{{suffix}}\\r{{ref1}}', binding: 'ref1', barcode: barcodePaint })).toBeDefined()
    expect(response({ ...barcode, barcodeUnavailable: 'unencodable' })).toBeDefined()
    // A bar that is not a whole number of modules, bars out of order, a bar past
    // the box, paint on another kind, and a reason beside a present paint.
    expect(response({ ...barcode, barcode: { moduleWidth: 10, bars: [{ x: 0, width: 15 }] } })).toBeUndefined()
    expect(response({ ...barcode, barcode: { moduleWidth: 10, bars: [{ x: 500, width: 10 }, { x: 0, width: 10 }] } })).toBeUndefined()
    expect(response({ ...barcode, barcode: { moduleWidth: 10, bars: [{ x: 895, width: 10 }] } })).toBeUndefined()
    expect(response({ ...barcode, type: 'rect', barcode: barcodePaint })).toBeUndefined()
    expect(response({ ...barcode, barcode: barcodePaint, barcodeUnavailable: 'doesNotFit' })).toBeUndefined()
    expect(response({ ...barcode, barcodeUnavailable: 'something-else' })).toBeUndefined()
  })

  it('admits a qrcode with Go-computed module runs, a value, a binding and a level, and refuses malformed qrcode paint', () => {
    const response = (component: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [component] } } })
    // Inside this fixture's 1000 mp band, so every refusal below is the qrcode rule's doing.
    const qrcode = { id: 'e1', type: 'qrcode', band: 'content', x: 0, y: 0, width: 500, height: 500, resizable: true }
    const qrcodePaint = { moduleWidth: 10, rects: [{ x: 40, y: 40, width: 70, height: 10 }, { x: 130, y: 40, width: 10, height: 10 }, { x: 40, y: 50, width: 10, height: 10 }] }
    expect(response(qrcode)).toBeDefined()
    expect(response({ ...qrcode, value: 'ชำระเงิน {{ref}}\\r', binding: 'ref', qrcode: qrcodePaint })).toBeDefined()
    expect(response({ ...qrcode, qrcodeUnavailable: 'tooLong' })).toBeDefined()
    // A non-square module, a width that is not whole modules, a run before the
    // previous one in its row, a row out of order, a rect past the box, a
    // non-integer, a surplus key, paint on another kind, and a reason beside paint.
    expect(response({ ...qrcode, qrcode: { moduleWidth: 10, rects: [{ x: 0, y: 0, width: 10, height: 20 }] } })).toBeUndefined()
    expect(response({ ...qrcode, qrcode: { moduleWidth: 10, rects: [{ x: 0, y: 0, width: 15, height: 10 }] } })).toBeUndefined()
    expect(response({ ...qrcode, qrcode: { moduleWidth: 10, rects: [{ x: 100, y: 0, width: 10, height: 10 }, { x: 50, y: 0, width: 10, height: 10 }] } })).toBeUndefined()
    expect(response({ ...qrcode, qrcode: { moduleWidth: 10, rects: [{ x: 0, y: 20, width: 10, height: 10 }, { x: 0, y: 10, width: 10, height: 10 }] } })).toBeUndefined()
    expect(response({ ...qrcode, qrcode: { moduleWidth: 10, rects: [{ x: 495, y: 0, width: 10, height: 10 }] } })).toBeUndefined()
    expect(response({ ...qrcode, qrcode: { moduleWidth: 10, rects: [{ x: 0.5, y: 0, width: 10, height: 10 }] } })).toBeUndefined()
    expect(response({ ...qrcode, qrcode: { moduleWidth: 10, rects: [{ x: 0, y: 0, width: 10, height: 10, dark: true }] } })).toBeUndefined()
    expect(response({ ...qrcode, type: 'rect', qrcode: qrcodePaint })).toBeUndefined()
    expect(response({ ...qrcode, qrcode: qrcodePaint, qrcodeUnavailable: 'doesNotFit' })).toBeUndefined()
    expect(response({ ...qrcode, qrcodeUnavailable: 'unencodable' })).toBeUndefined()
  })

  // STORY 14.4 / P3. THE SCALAR-BINDING KIND GATE, TESTED AS BEHAVIOUR.
  //
  // ⚠ WHY THE ROW ABOVE DOES NOT COVER IT. `response({ ...text, type: 'image' })`
  // does reject — but it would reject with the binding guard DELETED, because
  // that fixture also carries `textPaint`, and a non-text component carrying a
  // text paint is refused by a different line entirely. Measured: deleting the
  // guard left 1245 of 1246 tests passing, and the one failure was the mirror's
  // `toMatch` — a claim about this file's WORDING, not about parseInbound's
  // behaviour. A rule whose only witness is a regex over its own source text is
  // not tested.
  //
  // So each case below differs from an ADMITTED sibling in the `binding` key
  // and nothing else. That is what makes the rejection attributable to this
  // rule rather than to any of the dozen others in `isCanvas`.
  it('refuses a projected binding on every kind but text and barcode, and admits the bare component otherwise', () => {
    const response = (component: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [component] } } })
    // A TABLE IS THE ONE KIND THAT MUST NOT BE `resizable` — `isCanvas` refuses
    // a resizable table outright (its geometry is derived from its columns), so
    // the flag is per-kind here rather than shared. Getting this wrong is how a
    // "rejected" case can look like proof of a rule it never reached.
    const box = (type: string) => ({ id: 'e1', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: type !== 'table', type })
    for (const type of ['line', 'rect', 'image', 'table']) {
      // NON-VACUITY, PER KIND: the same component without a binding is
      // ADMITTED, so the rejection below is the binding key's doing and not a
      // malformed fixture quietly failing some other guard.
      expect(response(box(type)), `a bare ${type} component must be admitted`).toBeDefined()
      expect(response({ ...box(type), binding: 'customer.name' }), `a ${type} component carrying a binding must be refused`).toBeUndefined()
    }
    // AND THE SYMMETRIC HALF. `text` and `barcode` are the members of
    // SCALAR_BINDING_COMPONENT_TYPES, so the identical addition must be
    // ADMITTED on text (the barcode case is admitted in the test above) — otherwise a guard that refused every binding outright
    // would pass every assertion above.
    const textPaint = { overflow: false, truncated: false, lines: [] }
    expect(response({ ...box('text'), textPaint })).toBeDefined()
    expect(response({ ...box('text'), textPaint, binding: 'customer.name' })).toBeDefined()
  })

  // admittedTextLines pulls the first component's paint lines out of a
  // parsed inbound, failing the test if anything on the way is missing.
  // It exists so the tight-leading cases can assert on VALUES: parseInbound
  // is a type guard that returns its input unchanged, so `toBeDefined()`
  // passes for anything non-undefined and would keep passing if the
  // geometry were quietly rewritten on the way through.
  const admittedTextLines = (parsed: ReturnType<typeof parseInbound>) => {
    if (parsed === undefined || parsed.kind !== 'response' || !parsed.ok) throw new Error('the projection was rejected at the boundary')
    const lines = parsed.snapshot.canvas?.components[0]?.textPaint?.lines
    if (lines === undefined) throw new Error('the admitted snapshot carries no text paint lines')
    return lines
  }

  it('rejects non-advancing or out-of-box text paint geometry', () => {
    const response = (textPaint: object) => parseInbound({
      protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true,
      snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint }] } },
    })
    const line = { top: 0, baseline: 8, advance: 12, width: 10, fragments: [{ text: 'engine', x: 0 }] }
    expect(response({ overflow: false, truncated: false, lines: [line, { ...line, top: 11, baseline: 19 }] })).toBeUndefined()
    expect(response({ overflow: false, truncated: false, lines: [{ ...line, fragments: [{ text: 'engine', x: 11 }] }] })).toBeUndefined()
    expect(response({ overflow: false, truncated: false, lines: [{ ...line, top: -1, baseline: 8 }] })).toBeUndefined()
    // Story 7.2 / D-7.2.2, INVERTED DELIBERATELY. This assertion used to
    // read `.toBeUndefined()`: a baseline of 13 against top 0 and advance
    // 12 sits below the next line's top, so the line boxes overlap. That
    // IS tight leading, it is what the PDF draws, and the clause refusing
    // it restated an engine invariant on the browser's side of the
    // channel — blanking the entire projection over one line. It is
    // pinned here as deliberately gone rather than left silently untested.
    //
    // Asserted on the PARSED VALUES, not with `toBeDefined()`: the
    // predicate is a type guard that returns the input unchanged, so
    // `toBeDefined()` would pass for anything at all that is not
    // `undefined` and would keep passing if the geometry were quietly
    // rewritten on the way through.
    const tightLine = { ...line, baseline: 13 }
    const accepted = response({ overflow: false, truncated: false, lines: [tightLine] })
    expect(admittedTextLines(accepted)).toEqual([tightLine])
  })

  it('accepts a whole snapshot whose text lines are set at tight leading', () => {
    // The projection-level half of the assertion above: a MULTI-LINE
    // component at an advance tighter than its own first-baseline
    // offset. Every line here has baseline > top + advance, and
    // consecutive tops still step by exactly one advance — the shape
    // page_setup.go emits for `style.lineSpacing: 0.6`. Before D-7.2.2
    // one such line failed isTextPaint, then isCanvas, then isSnapshot,
    // and parseInbound dropped the whole response.
    const tight = {
      overflow: false, truncated: false,
      lines: [
        { top: 0, baseline: 11, advance: 9, width: 10, fragments: [{ text: 'first', x: 0 }] },
        { top: 9, baseline: 20, advance: 9, width: 10, fragments: [{ text: 'second', x: 0 }] },
        { top: 18, baseline: 29, advance: 9, width: 10, fragments: [{ text: 'third', x: 0 }] },
      ],
    }
    const parsed = parseInbound({
      protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true,
      snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 40, resizable: true, textPaint: tight }] } },
    })
    // Every line's own numbers must survive the boundary intact — the
    // canvas paints from these, and the engine is the only thing allowed
    // to have decided them (AD-17).
    const lines = admittedTextLines(parsed)
    expect(lines).toEqual(tight.lines)
    expect(lines.map((l) => [l.top, l.baseline, l.advance])).toEqual([[0, 11, 9], [9, 20, 9], [18, 29, 9]])
    // And the overlap really is present in what was admitted, so this
    // case cannot quietly stop exercising tight leading.
    expect(lines.every((l) => l.baseline > l.top + l.advance)).toBe(true)
    // A non-advancing projection is still refused, so the acceptance
    // above is not "the predicate stopped checking anything".
    expect(parseInbound({
      protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true,
      snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 40, resizable: true, textPaint: { ...tight, lines: tight.lines.map((l) => ({ ...l, advance: 0 })) } }] } },
    })).toBeUndefined()
  })

  it('accepts a well-formed image paint inside its own box and rejects malformed or out-of-box substitutes', () => {
    const box = { x: 0, y: 0, width: 100, height: 50 }
    // Finding 12 (review of 2026-08-29): the wire key is the FULL 64-hex
    // digest (D-5.13.2 amendment) — a truncated key passed admission here
    // before the fix, then could never resolve through the per-key 'asset'
    // fetch. This fixture's OWN well-formedness now pins that width.
    const image = { mediaType: 'image/png', assetKey: 'ab'.repeat(32), width: 300, height: 150, drawX: 0, drawY: 0, drawWidth: 100, drawHeight: 50 }
    const response = (img: object | undefined) => parseInbound({
      protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true,
      snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'image', band: 'content', resizable: true, ...box, ...(img === undefined ? {} : { image: img }) }] } },
    })
    expect(response(image)).toBeDefined()
    expect(response(undefined)).toBeDefined()
    expect(response({ ...image, mediaType: '' })).toBeUndefined()
    expect(response({ ...image, assetKey: '' })).toBeUndefined()
    expect(response({ ...image, assetKey: 'z'.repeat(64) })).toBeUndefined() // 64 chars, not hex
    // A well-formed but TRUNCATED key (12 hex characters, the inspector's
    // own display-abbreviation width) must be rejected, not merely a
    // wrong-length string of the wrong shape.
    expect(response({ ...image, assetKey: 'abcdef012345' })).toBeUndefined()
    expect(response({ ...image, width: 0 })).toBeUndefined()
    expect(response({ ...image, height: -1 })).toBeUndefined()
    expect(response({ ...image, drawWidth: 0 })).toBeUndefined()
    expect(response({ ...image, drawX: -1 })).toBeUndefined()
    expect(response({ ...image, drawX: 1, drawWidth: 100 })).toBeUndefined() // spills past box.x+box.width
    expect(response({ ...image, drawY: 1, drawHeight: 50 })).toBeUndefined() // spills past box.y+box.height
    expect(response({ ...image, extra: true })).toBeUndefined()
    // A non-image component must never carry an image paint.
    expect(parseInbound({
      protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true,
      snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint: { overflow: false, truncated: false, lines: [] }, image }] } },
    })).toBeUndefined()
  })

  it("admits Finding 9's imageUnavailable discriminant only for an image component with no image paint, and only its two named values", () => {
    const box = { x: 0, y: 0, width: 100, height: 50 }
    const response = (component: object) => parseInbound({
      protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true,
      snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'image', band: 'content', resizable: true, ...box, ...component }] } },
    })
    expect(response({ imageUnavailable: 'missing' })).toBeDefined()
    expect(response({ imageUnavailable: 'undecodable' })).toBeDefined()
    expect(response({})).toBeDefined() // absent is legal (an image paint present, or nothing yet)
    expect(response({ imageUnavailable: 'something-else' })).toBeUndefined()
    // Not a text component.
    expect(parseInbound({
      protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true,
      snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, textPaint: { overflow: false, truncated: false, lines: [] }, imageUnavailable: 'missing' }] } },
    })).toBeUndefined()
    // Not alongside a PRESENT image paint — the two are one signal.
    const image = { mediaType: 'image/png', assetKey: 'ab'.repeat(32), width: 300, height: 150, drawX: 0, drawY: 0, drawWidth: 100, drawHeight: 50 }
    expect(response({ image, imageUnavailable: 'missing' })).toBeUndefined()
  })

  it('admits only operation-coherent closed worker requests and responses', () => {
    const load = new Uint8Array([1]).buffer
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'load-1', operation: 'load', payload: load })).toBeDefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'load-2', operation: 'load' })).toBeUndefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'snapshot-1', operation: 'snapshot', payload: load })).toBeUndefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'snapshot-2', operation: 'snapshot', viewport: 900 })).toBeUndefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'undo-1', operation: 'undo' })).toBeDefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'redo-1', operation: 'redo' })).toBeDefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'undo-2', operation: 'undo', payload: load })).toBeUndefined()
    const dpr = ['device', 'PixelRatio'].join('')
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'command-1', operation: 'command', payload: load, [dpr]: 2 })).toBeUndefined()
    expect(parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'response-1', ok: false, error: { code: 'NO', message: 'no' }, font: 'browser' })).toBeUndefined()
    expect(parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'lifecycle', state: 'ready', snapshot: {} })).toBeUndefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'asset-1', operation: 'asset', payload: load })).toBeDefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'asset-2', operation: 'asset' })).toBeUndefined()
    const assetResponse = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response' as const, requestId: 'asset-1', ok: true as const, snapshot: { documentState: 'loaded' as const, revision: 3, byteLength: 1 }, bytes: load }
    expect(parseInbound(assetResponse)).toBeDefined()
  })

  it('accepts only a correlated, bounded three-byte render envelope and producer digest', () => {
    const part = new Uint8Array([1]).buffer
    const render = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'render-1', operation: 'render', payload: { template: part, data: part.slice(0), params: part.slice(0) } }
    expect(parseRequest(render)).toBeDefined()
    expect(parseRequest({ ...render, payload: { template: part, data: part.slice(0) } })).toBeUndefined()
    expect(parseRequest({ ...render, viewport: 900 })).toBeUndefined()
    const response = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'render-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, bytes: part, preview: { revision: 7, identity: 'b'.repeat(64), pdfSha256: 'a'.repeat(64), diagnostics: [], elapsedMs: 7, version: '0.0.0-dev' } }
    expect(parseInbound(response)).toBeDefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, pdfSha256: 'not-a-digest' } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, identity: 'not-an-identity' } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, revision: -1 } })).toBeUndefined()
    expect(parseInbound({ ...response, bytes: undefined })).toBeUndefined()
    expect(parseRequest({ ...render, payload: { template: new ArrayBuffer(MAX_ENGINE_PAYLOAD_BYTES + 1), data: part.slice(0), params: part.slice(0) } })).toBeUndefined()
    expect(parseInbound({ ...response, bytes: new ArrayBuffer(MAX_ENGINE_RENDER_PDF_BYTES + 1) })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, diagnostics: [{}] } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, diagnostics: [], snapshot: {} } })).toBeUndefined()
    // STORY 13.3 — THE RENDER ARM IS ALL FOUR MEMBERS OR NONE, AND EACH IS
    // NAMED IN ITS OWN ROW so a failure says which one went missing rather
    // than only that the reply was invalid.
    expect(parseInbound({ ...response, preview: { ...response.preview, elapsedMs: undefined } }), 'a render reply that lost elapsedMs must be refused, not displayed with a gap').toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, version: undefined } }), 'a render reply that lost version must be refused, not displayed with a gap').toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, elapsedMs: -1 } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, elapsedMs: 1.5 } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, elapsedMs: '7' } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, version: '' } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, version: 'v'.repeat(65) } })).toBeUndefined()
    // A ZERO IS ADMITTED. `0 ms` is what a very fast render reports, and an
    // implementation that rejected it by truthiness would silently refuse the
    // fastest documents — which is why neither Go struct is `omitempty`.
    expect(parseInbound({ ...response, preview: { ...response.preview, elapsedMs: 0 } })).toBeDefined()
  })

  it('accepts only an identity-only engine response with revision-bound opaque evidence', () => {
    const part = new Uint8Array([1]).buffer
    const request = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'identity-1', operation: 'identity', payload: { data: part, params: part.slice(0) } }
    expect(parseRequest(request)).toBeDefined()
    expect(parseRequest({ ...request, payload: { data: part } })).toBeUndefined()
    const response = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'identity-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, preview: { revision: 7, identity: 'a'.repeat(64) } }
    expect(parseInbound(response)).toBeDefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, pdfSha256: 'b'.repeat(64) } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, revision: 6 } })).toBeUndefined()
    // THE OTHER HALF OF THE ALL-OR-NOTHING ARM. An identity reply carries
    // neither render fact; one arriving alone is a protocol breach, not a
    // bonus.
    expect(parseInbound({ ...response, preview: { ...response.preview, elapsedMs: 7 } })).toBeUndefined()
    expect(parseInbound({ ...response, preview: { ...response.preview, version: '0.0.0-dev' } })).toBeUndefined()
  })

  it('admits only a bounded, revision-correlated engine parameter-reference projection', () => {
    const request = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'params-1', operation: 'parameter-references' }
    expect(parseRequest(request)).toBeDefined()
    expect(parseRequest({ ...request, payload: new Uint8Array([1]).buffer })).toBeUndefined()
    const response = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'params-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, parameterReferences: { revision: 7, names: ['branch', 'reportDate'] } }
    expect(parseInbound(response)).toBeDefined()
    expect(parseInbound({ ...response, parameterReferences: { revision: 6, names: ['branch'] } })).toBeUndefined()
    expect(parseInbound({ ...response, parameterReferences: { revision: 7, names: ['reportDate', 'branch'] } })).toBeUndefined()
    expect(parseInbound({ ...response, parameterReferences: { revision: 7, names: ['reportDate', 'reportDate'] } })).toBeUndefined()
    expect(parseInbound({ ...response, parameterReferences: { revision: 7, names: ['reportDate'], expression: 'params.reportDate' } })).toBeUndefined()
  })

  // STORY 13.4 — THE STAND-IN DOCUMENT RIDES THE ENVELOPE THAT ALREADY
  // CARRIED BYTES. No new request field, no new response field, no protocol
  // version change: the success envelope's key list already permits `bytes`,
  // so a bytes-returning projection needed nothing widened for it.
  it('admits a byte-returning stand-in data projection on the existing envelope', () => {
    const request = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'standin-1', operation: 'stand-in-data' }
    expect(parseRequest(request)).toBeDefined()
    // It takes NO byte input, exactly as parameter-references does.
    expect(parseRequest({ ...request, payload: new Uint8Array([1]).buffer })).toBeUndefined()
    const response = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'standin-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, bytes: new TextEncoder().encode('{"customer":{"name":""}}').buffer }
    expect(parseInbound(response)).toBeDefined()
    // And nothing extra rides along with it.
    expect(parseInbound({ ...response, standInData: '{}' })).toBeUndefined()
  })

  it('admits only a selected, revision-correlated table-column paint projection', () => {
    const payload = new TextEncoder().encode('{"id":"e7"}').buffer
    const request = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'table-1', operation: 'table-columns', payload }
    expect(parseRequest(request)).toBeDefined()
    expect(parseRequest({ ...request, payload: undefined })).toBeUndefined()
    const response = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'table-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, tableColumns: { revision: 7, table: { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'transactions[]', alias: 'transaction', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right', headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '{{transaction.amount}}', rowField: 'amount', rowFieldEditable: true, footer: 'sum', footerOf: 'transactions.amount', footerFormat: '#,##0.00' }] } } }
    expect(parseInbound(response)).toBeDefined()
    expect(parseInbound({ ...response, tableColumns: { ...response.tableColumns, revision: 6 } })).toBeUndefined()
    expect(parseInbound({ ...response, tableColumns: { ...response.tableColumns, table: { ...response.tableColumns.table, columns: [{ ...response.tableColumns.table.columns[0], bind: 'row.amount' }] } } })).toBeUndefined()
    expect(parseInbound({ ...response, tableColumns: { ...response.tableColumns, table: { ...response.tableColumns.table, columns: [{ ...response.tableColumns.table.columns[0], width: 0 }] } } })).toBeUndefined()
    const proportional = (proportion: unknown, totalWidth: unknown = 72000, sizing: unknown = 'proportion') => ({ ...response, tableColumns: { ...response.tableColumns, table: { ...response.tableColumns.table, sizing, totalWidth, columns: [{ ...response.tableColumns.table.columns[0], proportion }] } } })
    for (const proportion of ['1', '0.001', '1.234', '1001', '9223372036854775.807']) expect(parseInbound(proportional(proportion))).toBeDefined()
    for (const proportion of ['', '0', '-1', '1.0001', '1.0', '01', '1e3', '9223372036854775.808', 1, null]) expect(parseInbound(proportional(proportion))).toBeUndefined()
    for (const totalWidth of [0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1, '72000']) expect(parseInbound(proportional('1', totalWidth))).toBeUndefined()
    expect(parseInbound(proportional('1', 72000, 'mixed'))).toBeUndefined()
    expect(parseInbound(proportional('1', 72000, 'points'))).toBeUndefined()

  })

  // The engine's rowFieldEditable flag gates simple-field authoring. It must
  // remain present on the exact wire shape even for arbitrary expressions.

  it('still admits rowFieldEditable on the wire, though [D-14.10.1] left it with no production reader', () => {
    const response = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'table-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, tableColumns: { revision: 7, table: { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'transactions[]', alias: 'transaction', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right', headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '{{transaction.amount}}', rowField: 'amount', rowFieldEditable: true, footer: 'sum', footerOf: 'transactions.amount', footerFormat: '#,##0.00' }] } } }
    // Non-vacuity: the fixture must be admitted as it stands, or the two
    // refusals below prove nothing.
    const admitted = parseInbound(response)
    expect(admitted, 'the table-columns fixture must be admitted for this row to be about anything').toBeDefined()
    expect(admitted && 'tableColumns' in admitted ? admitted.tableColumns?.table.columns[0]?.rowFieldEditable : undefined).toBe(true)
    // DROPPED FROM THE WIRE — the exact-key set refuses the whole projection.
    const { rowFieldEditable: _dropped, ...withoutMember } = response.tableColumns.table.columns[0]!
    expect(parseInbound({ ...response, tableColumns: { ...response.tableColumns, table: { ...response.tableColumns.table, columns: [withoutMember] } } }), '[D-14.10.1] left `rowFieldEditable` with no production reader, but `hasExactKeys` still requires it: dropping it in Go makes every table-columns response fail admission and the Table Editor stop opening').toBeUndefined()
    // AND RETYPED — `false` is still a boolean and still admitted; a string is
    // not, so the member is checked rather than merely counted.
    expect(parseInbound({ ...response, tableColumns: { ...response.tableColumns, table: { ...response.tableColumns.table, columns: [{ ...response.tableColumns.table.columns[0], rowFieldEditable: false }] } } })).toBeDefined()
    expect(parseInbound({ ...response, tableColumns: { ...response.tableColumns, table: { ...response.tableColumns.table, columns: [{ ...response.tableColumns.table.columns[0], rowFieldEditable: 'yes' }] } } })).toBeUndefined()
  })

  // STORY 12.3 — THE TABLE OBJECT'S KEY SET, RED-PROVED IN BOTH DIRECTIONS.
  //
  // `hasExactKeys` is a LENGTH check AND a membership check, so it rejects a
  // key Go stops sending exactly as hard as one it starts sending. Only the
  // second direction was possible under `hasOnly`, which is what isCanvas uses,
  // so both arms are asserted here rather than assumed from the canvas guard.
  //
  // WHAT A FAILURE COSTS, because it is not the blank canvas the wire test's
  // header describes: parseInbound returns undefined, engine-client raises
  // PROTOCOL_INVALID, the worker is TERMINATED and every pending request
  // rejected, and no re-spawn exists. On a FIRST table-editor open it is
  // silent — openTableEditor's catch sets an error that renders only inside
  // <TableEditor>, which never mounts. So the assertion is on parseInbound's
  // RETURN VALUE and never on a visual symptom.
  // SPEC-table-rules §4: Go bounds a column label in CODE POINTS, so the guard
  // must too. A 256-emoji label is 512 UTF-16 units; counting `.length` would
  // refuse a reply the engine produced and the Table Editor would never open.
  it('bounds a table column header in code points, not UTF-16 units', () => {
    const tableWith = (header: string) => ({ tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'transactions[]', alias: 'transaction', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [{ id: 'e8', header, width: 72000, proportion: '', align: 'left', headerAlign: '' as const, headerAlignResolved: 'left' as const, binding: '', rowField: '', rowFieldEditable: false, footer: '', footerOf: '', footerFormat: '' }] })
    const responseFor = (header: string) => ({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'table-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, tableColumns: { revision: 7, table: tableWith(header) } })
    const astral256 = '\u{1F600}'.repeat(256)
    expect(astral256.length).toBe(512)
    expect(parseInbound(responseFor(astral256))).toBeDefined()
    expect(parseInbound(responseFor('\u{1F600}'.repeat(257)))).toBeUndefined()
    // A line feed is one code point like any other.
    expect(parseInbound(responseFor(`${'x'.repeat(127)}\n${'y'.repeat(128)}`))).toBeDefined()
    expect(parseInbound(responseFor('x'.repeat(257)))).toBeUndefined()
  })

  it('admits the table cell padding as signed thousandths strings and a committed column headerAlign, and refuses anything else', () => {
    const column = (headerAlign: unknown) => ({ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right', headerAlign, headerAlignResolved: 'right', binding: '', rowField: '', rowFieldEditable: false, footer: '', footerOf: '', footerFormat: '' })
    const table = (patch: object, headerAlign: unknown = '') => ({ tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'transactions[]', alias: 'transaction', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [column(headerAlign)], ...patch })
    const admitted = (value: object) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'table-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, tableColumns: { revision: 7, table: value } }) !== undefined
    expect(admitted(table({}))).toBe(true)
    for (const [left, right] of [['3000', '0'], ['-1500', ''], ['', '250']]) expect(admitted(table({ paddingLeft: left, paddingRight: right })), `${left}/${right}`).toBe(true)
    expect(admitted(table({ paddingHeaderOverride: true }))).toBe(true)
    for (const bad of ['-0', '03', '1.5', '3pt', 3000, null]) expect(admitted(table({ paddingLeft: bad })), String(bad)).toBe(false)
    expect(admitted(table({ paddingHeaderOverride: 'true' }))).toBe(false)
    for (const align of ['left', 'center', 'right']) expect(admitted(table({}, align)), align).toBe(true)
    for (const align of ['justify', 'LEFT', null]) expect(admitted(table({}, align)), String(align)).toBe(false)
    // headerAlignResolved is what prints: always one of the triple, never ''.
    for (const resolved of ['left', 'center', 'right']) expect(admitted(table({ columns: [{ ...column(''), headerAlignResolved: resolved }] })), resolved).toBe(true)
    for (const resolved of ['', 'justify', null]) expect(admitted(table({ columns: [{ ...column(''), headerAlignResolved: resolved }] })), `resolved ${String(resolved)}`).toBe(false)
    // A column WITHOUT the key is refused too: the column record is exact.
    const { headerAlign: _absent, ...legacyColumn } = column('')
    expect(admitted(table({ columns: [legacyColumn] }))).toBe(false)
    const { headerAlignResolved: _unresolved, ...unresolvedColumn } = column('')
    expect(admitted(table({ columns: [unresolvedColumn] }))).toBe(false)
    const { paddingLeft: _dropped, ...missing } = table({})
    expect(admitted(missing)).toBe(false)
  })

  it('refuses a table projection with a missing member and one with a surplus key alike', () => {
    const table = { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'transactions[]', alias: 'transaction', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [] }
    const responseFor = (value: Record<string, unknown>) => ({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'table-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, tableColumns: { revision: 7, table: value } })
    expect(parseInbound(responseFor(table))).toBeDefined()
    // DIRECTION ONE — a projected member the guard's list does not name. Modelled
    // as the guard being BEHIND Go: the response carries a seventeenth member.
    expect(parseInbound(responseFor({ ...table, headerBorder: '' }))).toBeUndefined()
    // DIRECTION TWO — a guard key with no projected member, one per member, so
    // the proof is not carried by whichever key happens to be checked first.
    for (const key of Object.keys(table)) {
      const missing: Record<string, unknown> = { ...table }
      delete missing[key]
      expect(parseInbound(responseFor(missing)), `dropping ${key} must be refused`).toBeUndefined()
    }
    // And the typed clauses behind the key list, so a member of the right NAME
    // and the wrong shape is refused too.
    expect(parseInbound(responseFor({ ...table, headerHeight: '12000' }))).toBeUndefined()
    expect(parseInbound(responseFor({ ...table, headerFontSize: 12.5 }))).toBeUndefined()
    expect(parseInbound(responseFor({ ...table, headerAlignResolved: '' }))).toBeUndefined()
    expect(parseInbound(responseFor({ ...table, headerValignResolved: 'centre' }))).toBeUndefined()
    expect(parseInbound(responseFor({ ...table, headerAlign: 'justify' }))).toBeUndefined()
    // A COMMITTED alignment of '' is ABSENT and must stay admissible: refusing
    // it would make an unstyled table's own projection unparseable.
    expect(parseInbound(responseFor({ ...table, headerAlign: '', headerValign: '' }))).toBeDefined()
    // STORY 11.3 / DW-240 — THE TWO BOOLEAN PAIRS. Wrong shape refused on each
    // of the four members separately, so the proof is not carried by whichever
    // one the guard happens to test first.
    for (const key of ['headerBold', 'headerBoldResolved', 'headerItalic', 'headerItalicResolved'] as const) {
      expect(parseInbound(responseFor({ ...table, [key]: 'true' })), `${key} as a string must be refused`).toBeUndefined()
      expect(parseInbound(responseFor({ ...table, [key]: 1 })), `${key} as a number must be refused`).toBeUndefined()
    }
    // AND BOTH VALUES ARE ADMITTED ON BOTH HALVES. `false` is this projection's
    // spelling of absence for a bool — committed-absent and committed-`false`
    // are the same wire value, a limit TableColumnsProjection's own comment
    // discloses — so nothing here may read a meaning into either one.
    expect(parseInbound(responseFor({ ...table, headerBold: true, headerBoldResolved: true, headerItalic: false, headerItalicResolved: true }))).toBeDefined()
    // STORY 14.8's THREE PAIRS. The DOTTED key names are not cosmetic: the engine
    // authors the header border one attribute at a time through the field names
    // `border.width`/`border.color`/`border.edges`, so its located refusals name
    // a path the document has, and Go derives these projection keys from those
    // field names. A bare `headerBorder` is therefore still a SURPLUS key —
    // asserted above, and it is a different claim from these.
    //
    // THE WIDTH PAIR IS A PAIR OF STRINGS — the only length on this projection
    // that is — BECAUSE `0` IS A LEGAL AUTHORED WIDTH AND THEREFORE CANNOT ALSO
    // BE THE SPELLING OF ABSENCE. `parse_bands.go`: zero "is the thinnest device
    // line PDF can draw, not an absent border". The units did not move (integer
    // thousandths, as `headerFontSize`); only absence acquired a spelling of its
    // own, `''`.
    //
    // AND THE CLAUSE HAD TO TIGHTEN RATHER THAN LOOSEN, because a string member
    // replacing a bounded number is how a garbage value walks in. Digits only,
    // with no sign, no decimal point, no exponent and no surrounding space —
    // which is exactly what Go's `strconv.FormatInt` can emit — plus `''`. The
    // no-negative half is the same bound the numeric clause carried and it is
    // measured: `parse_bands.go` REFUSES a negative border width (ISO 32000-1
    // §8.4.3.2), so one cannot come from a loaded document.
    for (const key of ['headerBorder.width', 'headerBorder.widthResolved'] as const) {
      for (const admitted of ['', '0', '500', '3000']) {
        expect(parseInbound(responseFor({ ...table, [key]: admitted })), `${key} = ${JSON.stringify(admitted)} must be admitted`).toBeDefined()
      }
      // A NUMBER IS NOW REFUSED, AND THAT IS THE HALF THAT PINS THE SEAM. Go
      // sends a string; a number arriving here is a Go/TypeScript disagreement
      // about this member's type, which is the failure `hasExactKeys` exists to
      // turn into a refusal rather than into a silently wrong panel.
      // ⚠ A LEADING ZERO IS REFUSED, AND SO IS A MAGNITUDE PAST THE SAFE-INTEGER
      // RANGE. The digit pattern alone admitted both, which made this clause
      // WIDER than the bounded number it replaced — the number carried
      // `Number.isSafeInteger`, and a re-spelling is not allowed to spend that.
      //
      // `'007'` matters because the panel branches on this member as a STRING
      // (`committed === ''`, `committed === '0'`) and keys a remount on it: two
      // spellings of one value are two states to the key and one value to
      // `Number()`, which is the exact conflation the string spelling was
      // introduced to remove. `'0007'` and `'00'` are the same defect.
      //
      // The magnitudes matter because `authored()` divides these by 1000, and
      // past 2^53 that division is silently lossy — a projected length that
      // cannot survive its own display. `strconv.FormatInt` emits neither shape,
      // so refusing them refuses only what Go cannot send.
      for (const refused of [0, 500, -1, 0.5, null, undefined, ['500'], '-1', '+500', '0.5', '1e3', ' 500', '500 ', '5 0 0', 'abc', '٥٠٠', '007', '0007', '00', '01', '9007199254740992', '99999999999999999999999999999999']) {
        expect(parseInbound(responseFor({ ...table, [key]: refused })), `${key} = ${JSON.stringify(refused)} must be refused`).toBeUndefined()
      }
      // AND THE BOUNDARY ITSELF IS ADMITTED, so the magnitude refusal above is
      // pinned as a bound rather than as an arbitrary cut: 2^53 - 1 is the last
      // integer `Number` represents exactly.
      expect(parseInbound(responseFor({ ...table, [key]: '9007199254740991' })), `${key} = the largest safe integer must be admitted`).toBeDefined()
    }
    // THE EDGE LISTS, AGAINST THE CLOSED SET AND IN THE FORMAT'S OWN ORDER. Go
    // joins them canonically, so a re-ordered or repeated list is a projection
    // this engine does not produce and an unknown name is one the loader already
    // refused. `''` is admitted on both halves and means two different things:
    // "the document declares no list" on the committed member, and "nothing is
    // painted" on the resolved one — the only member that reports the second way
    // nothing gets painted, a border that DOES resolve while its declared edges
    // name no side. (The first way, no border at all, the resolved width now
    // reports too, by being '' rather than a digit.)
    for (const key of ['headerBorder.edges', 'headerBorder.edgesResolved'] as const) {
      for (const admitted of ['', 'top', 'bottom', 'top,left', 'top,right,bottom,left']) {
        expect(parseInbound(responseFor({ ...table, [key]: admitted })), `${key} = ${JSON.stringify(admitted)} must be admitted`).toBeDefined()
      }
      for (const refused of ['middle', 'left,top', 'top,top', 'TOP', 'top, left', 'top,', ',top', ['top']]) {
        expect(parseInbound(responseFor({ ...table, [key]: refused })), `${key} = ${JSON.stringify(refused)} must be refused`).toBeUndefined()
      }
    }
  })

  // A NEGATIVE LENGTH THE FILE DOOR ADMITS MUST NOT KILL THE WORKER.
  //
  // The guard used to require `>= 0` for every projected length, but the loader
  // bounds NEITHER headerHeight NOR style.fontSize: decimal.go negates on
  // `sign < 0`, parse_bands.go assigns `t.HeaderHeight = hh` unchecked, and the
  // style decoder assigns `st.FontSize = present(v)` the same way. So a
  // hand-authored `"headerHeight": -5` loads and renders TODAY, and after this
  // story opening its table editor failed the guard — parseInbound undefined,
  // PROTOCOL_INVALID, worker.terminate(), no re-spawn, and on a first open
  // nothing shown at all because <TableEditor> never mounts to carry the error.
  //
  // The remedy is here rather than at the loader on purpose: bounding the
  // loader would narrow the format, which the story forbids itself. The guard's
  // job is to admit exactly what the file door admits.
  it('admits the negative lengths the loader itself admits, and still refuses a negative line spacing', () => {
    const table = { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'transactions[]', alias: 'transaction', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [] }
    const responseFor = (value: Record<string, unknown>) => ({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'table-1', ok: true, snapshot: { documentState: 'loaded', revision: 7, byteLength: 1 }, tableColumns: { revision: 7, table: value } })
    // The document that loads today: a negative headerHeight, and the negative
    // fontSize that cascades into its resolved twin.
    expect(parseInbound(responseFor({ ...table, headerHeight: -5000 }))).toBeDefined()
    expect(parseInbound(responseFor({ ...table, headerFontSize: -4000, headerFontSizeResolved: -4000 }))).toBeDefined()
    // THE LINE-SPACING PAIR IS NOT RELAXED, because for it the file door really
    // does bound: DecodeLineSpacingRaw refuses anything outside [1, 1000000]
    // thousandths, so a negative one cannot come from a loaded document and
    // admitting it would only widen the guard past its source.
    expect(parseInbound(responseFor({ ...table, headerLineSpacing: -1 }))).toBeUndefined()
    expect(parseInbound(responseFor({ ...table, headerLineSpacingResolved: -1000 }))).toBeUndefined()
    // And the shape clauses still hold on the relaxed members: a length is
    // still an INTEGER count of millipoints and still a number.
    expect(parseInbound(responseFor({ ...table, headerHeight: -5.5 }))).toBeUndefined()
    expect(parseInbound(responseFor({ ...table, headerFontSizeResolved: null }))).toBeUndefined()
  })
})

// STORY 13.3 — THE TWO HAND-ENUMERATED HOPS, WHICH DROP UNKNOWN FIELDS IN
// SILENCE.
//
// `isPreview` above is a guard on the SHAPE that arrives; it cannot see a field
// that was thrown away before it. Between Go and React the preview object is
// rebuilt member by member TWICE — once in `engine.worker.ts` when the wasm
// response is turned into a protocol message, and once in `engine-client.ts`
// inside `deepFreeze` — and a Go field named in neither literal never reaches
// App.tsx at all. That is not a hypothetical: `engine-client.ts`'s own comment
// records Story 12.3's table projection being lost exactly this way, with no
// protocol failure and nothing in the DOM to say so.
//
// A behavioural test cannot separate the two hops (the worker's output is the
// client's input, so one covers for the other), and it cannot say WHICH field
// went missing. This reads the two literals directly and names the field.
describe('the preview literal is rebuilt in full at both protocol hops', () => {
  const hops = [
    ['engine.worker.ts', /const preview = request\.operation === 'render' \? \{([^}]*)\}/],
    ['engine-client.ts', /\.\.\.\(message\.preview\.pdfSha256 \? \{([\s\S]*?)\} : \{\}\)/],
  ] as const

  it('names every render-arm member at each hop, so a dropped field reds here rather than vanishing', () => {
    for (const [file, pattern] of hops) {
      const source = readFileSync(`src/${file}`, 'utf8')
      const literal = source.match(pattern)?.[1]
      expect(literal, `${file} no longer spells the render preview literal this scan reads; re-derive the extraction rather than deleting the check`).toBeTruthy()
      for (const field of ['pdfSha256', 'diagnostics', 'elapsedMs', 'version']) {
        expect(literal, `${file} builds the render preview without naming '${field}', which is dropped in silence before anything downstream can notice`).toContain(field)
      }
    }
  })

  // THE RED PROOF, so the scan cannot pass by matching nothing: deleting a
  // field from either literal must make the extraction stop containing it.
  it('turns a field deleted from either literal red', () => {
    for (const [file, pattern] of hops) {
      const source = readFileSync(`src/${file}`, 'utf8')
      for (const field of ['elapsedMs', 'version']) {
        const mutated = source.replace(new RegExp(`, ${field}: [^,}]+`), '')
        expect(mutated, `the mutation must actually change ${file}`).not.toEqual(source)
        expect(mutated.match(pattern)?.[1] ?? '').not.toContain(field)
      }
    }
  })
})


describe('authored common property evidence', () => {
  const fields = ['visibleIf', 'fontFamily', 'fontSize', 'lineSpacing', 'bold', 'italic', 'align', 'valign', 'color', 'background', 'borderWidth', 'borderColor', 'borderEdges', 'errorCorrection']
  const absent = Object.fromEntries(fields.map((key) => [key, { state: 'absent' }]))
  const response = (authored: object) => ({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'authored-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components: [{ id: 'e1', type: 'rect', band: 'content', x: 0, y: 0, width: 10, height: 10, resizable: true, authored }] } } })
  it('admits absent/null/false/zero/empty independently of resolved paint', () => {
    expect(parseInbound(response({ ...absent, visibleIf: { state: 'value', value: '' }, background: { state: 'null' }, bold: { state: 'value', value: false }, borderWidth: { state: 'value', value: 0 }, borderEdges: { state: 'value', value: [] } }))).toBeDefined()
  })
  it.each([
    { ...absent, extra: { state: 'absent' } },
    { ...absent, bold: { state: 'value', value: 'false' } },
    { ...absent, color: { state: 'null', value: '#ffffff' } },
    { ...absent, color: { state: 'value', value: 'x'.repeat(MAX_CANVAS_PROPERTY_STRING + 1) } },
    { ...absent, borderWidth: { state: 'value', value: Number.MAX_SAFE_INTEGER + 1 } },
    { ...absent, borderEdges: { state: 'value', value: ['middle'] } },
    { ...absent, errorCorrection: { state: 'value', value: 'X' } },
  ])('rejects malformed, surplus, or unbounded authored evidence', (authored) => {
    expect(parseInbound(response(authored))).toBeUndefined()
  })
  it('requires every field and the value on value states', () => {
    const { color: _color, ...missing } = absent
    expect(parseInbound(response(missing))).toBeUndefined()
    expect(parseInbound(response({ ...absent, color: { state: 'value' } }))).toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// `install-face` (spec-deferred-offline-cache, story 5). The union and its
// runtime allow-list are both closed, and a new member has to reach both: a
// type-only addition compiles and is then refused at run time by the very
// guard that exists to admit it.
// ---------------------------------------------------------------------------
describe('the install-face operation', () => {
  const ok = (payload: unknown) => parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'request-1', operation: 'install-face', payload })

  it('admits a named face with its bytes', () => {
    expect(ok({ face: 'Noto Sans SC', bytes: new Uint8Array([1, 2, 3]).buffer })).toBeDefined()
  })

  it('refuses a face with no name, no bytes, or a key it never declared', () => {
    expect(ok({ face: '', bytes: new Uint8Array([1]).buffer })).toBeUndefined()
    expect(ok({ face: 'Noto Sans SC', bytes: new ArrayBuffer(0) })).toBeUndefined()
    expect(ok({ face: 'Noto Sans SC' })).toBeUndefined()
    expect(ok({ face: 'Noto Sans SC', bytes: new Uint8Array([1]).buffer, extra: 1 })).toBeUndefined()
    expect(ok(new Uint8Array([1]).buffer), 'a bare ArrayBuffer carries no face name, so the host would not know what to install').toBeUndefined()
    expect(ok(undefined)).toBeUndefined()
  })

  // ⚠ THE FACE PAYLOAD IS BOUNDED BY ITS OWN NUMBER, NOT BY THE ENVELOPE'S.
  // The CJK face is over 10 MiB, which the 8 MiB request bound forbids — and
  // widening that bound would relax every operation that rides it, which the
  // spec names as a thing not to do.
  it('is bounded well above the request envelope, and the envelope is untouched', () => {
    expect(MAX_ENGINE_FACE_BYTES).toBeGreaterThan(10_595_932)
    expect(MAX_ENGINE_PAYLOAD_BYTES, 'the existing envelope bound must stay exactly where it was').toBe(8 * 1024 * 1024)
    expect(MAX_ENGINE_FACE_BYTES).toBeGreaterThan(MAX_ENGINE_PAYLOAD_BYTES)
    expect(ok({ face: 'Noto Sans SC', bytes: new ArrayBuffer(MAX_ENGINE_FACE_BYTES + 1) })).toBeUndefined()
  })

  // No other operation may carry a face payload, and `install-face` may not
  // carry anything else: both halves of the discrimination, so a widening in
  // either direction reds.
  it('is the only operation that takes a face payload', () => {
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'request-1', operation: 'load', payload: { face: 'Noto Sans SC', bytes: new Uint8Array([1]).buffer } })).toBeUndefined()
    expect(parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'request-1', operation: 'install-face' }), 'an install with no payload installs nothing').toBeUndefined()
  })
})
