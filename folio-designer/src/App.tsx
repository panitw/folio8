import './App.css'
import { createPortal } from 'react-dom'
import { createContext, useContext, useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type ClipboardEvent as ReactClipboardEvent, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent, type ReactNode } from 'react'
import { isProducerRenderFailure, type EngineClient, type EngineResult } from './engine-client'
import { CAPPING_BANDS, LOCALE_TAGS, MAX_ENGINE_HISTORY_ENTRIES, MAX_LINE_SPACING_THOUSANDTHS, MIN_LINE_SPACING_THOUSANDTHS, SCALAR_BINDING_COMPONENT_TYPES, type CanvasProjection, type CanvasTableColumn, type CappingBand, type EngineDiagnostic, type EngineError, type EngineSnapshot, type LocaleTag, type TableColumns } from './engine-protocol'
import type { OfflineLifecycleState } from './offline-lifecycle'
import type { OfflineLifecycle } from './offline-lifecycle'
import { activatePendingRelease, engineMayStart } from './offline-lifecycle'
import type { S1Payload } from './release-payload'
import { LoadScreen } from './LoadScreen'
import { BrandMark } from './BrandMark'
import type { BindingErrorScope } from './DataPanel'
import { FileAccessFailure, folioFileFormat, isFileAccessCancelled, jsonSampleFileFormat, pdfFileFormat, type FileAccess, type FileTarget, type LocalFile } from './file/file-access'
import { pageSetupCommand } from './page-setup-command'
import { bandHeightCommand } from './band-height-command'
import { bandBoundaryCeiling, boundaryOffset, proposedBandHeight } from './band-boundary'
import { removeSectionBreakCommand, setSectionBreakAnchorCommand, setSectionBreakCommand } from './section-break-command'
import { addPageCommand, deletePageCommand, setPageBreakCommand } from './page-command'
import { contentBandHeight, proposedSectionBreak, sectionBreakOnPage, sectionBreakPlacement } from './section-break'
import { documentLocaleCommand, documentUTCOffsetCommand } from './document-settings-command'
import { bindComponentScalarCommand, bindTableCollectionCommand, createComponentCommand, deleteComponentCommand, deleteComponentsCommand, dropComponentCommand, duplicateComponentCommand, duplicateComponentsCommand, moveComponentCommand, setComponentBoundsCommand, type PaletteKind } from './component-command'
import { ORIGIN_FLOOR_FIELDS, POSITIVE_LENGTH_FIELDS, isPropertyField, updateComponentPropertiesCommand, type PropertyField, type PropertyIntent, type PropertyIntents } from './component-property-command'
import { FontBrowser } from './FontBrowser'
import { type FontChainCommitError, type FontChainControl } from './font-chain-control'
import { addFontChainCommand, embedFontFamilyCommand, type FontChainEntryAsk } from './font-chain-command'
import { catalogueFaces, scriptFallbackFaces } from './generated/font-catalogue'
import { familyIsInstalled, indexRowFor, offeredFamilies, type FamilySource } from './font-index'
import { initialHeldLocalFamilies, readHeldLocalFamilies } from './held-local-faces'
import { watchCanvasFaceMisses } from './canvas-face-misses'
import { isShippedFamily, shippedFamilyEntry } from './shipped-face-cuts'
import { browserRows } from './font-browser-model'
import { fetchWebFamily } from './font-source'
import { openFontStore, storeWriteRefusal, storedFaceKey, type FontStore, type StoredFace } from './font-store'
import { previewFaceFamily } from './preview-face-family'
import { openPreviewFaceRegistry, type PreviewFaceBytes, type PreviewFaceRegistry, type PreviewFaceStatus } from './preview-face-registry'
import { proposedBounds, resizeAnchors, type DragAnchor, type DragLimit } from './resize-anchor'
import { columnEdgeAfterDrag, componentPage, pageCountOf, sheetPitch, sheetStack, SHEET_STACK_GAP, type Sheet, type SheetOccurrence, type SheetStack } from './sheet-stack'
import { pageUnder, translatedCanvas } from './canvas-selection'
import { useCanvasSelection, type GroupPreview } from './use-canvas-selection'
import { tableWidthCommand, addTableColumnCommand, configureTableBindingCommand, moveTableColumnCommand, removeTableColumnCommand, updateTableColumnBindingCommand, updateTableColumnCommand, updateTableColumnExpressionCommand, updateTableColumnFooterCommand } from './table-column-command'
import { TableEditor } from './TableEditor'
import { alignSegments, justifySegment, SegmentedControl, type SegmentSpec } from './segmented-control'
import { ToolIcon } from './toolbar-icons'
import { documentationAssetUrls } from './generated/documentation-assets'

// A glyph tool button's hover guide: its name, then its shortcut when it has one.
const toolTip = (name: string, shortcut: string) => `${name} (${shortcut})`
// On macOS Option+letter types another character (⌥P is "π"), so an Alt
// shortcut matches the physical key as well as the typed one.
const altLetter = (event: KeyboardEvent, letter: string) => event.code === `Key${letter.toUpperCase()}` || event.key.toLowerCase() === letter
import { isHexColour, swatchColor } from './swatch-color'
import { tableAltRowBackgroundCommand, tableHeaderHeightCommand, tableHeaderStyleCommand, tableMinHeightCommand, tableRulesCommand } from './table-style-command'
import { initialPDFPreviewViewState, PDFPreviewViewer, samePDFPreviewViewState, type PDFPreviewViewState } from './preview/pdf-viewer'
import { clampPreviewScale, PREVIEW_ZOOM_CHOICES, steppedPreviewScale, typedPreviewPage, typedPreviewZoom } from './preview/viewer-navigation'
import { canInstallPreview, formatRenderAge, freshnessChrome, PREVIEW_DEBOUNCE_MS, PreviewWorkScheduler, renderAgeTickMs } from './preview/freshness'
import { PreviewDiagnostics, PreviewFailure, type DiagnosticLocation } from './preview/diagnostic-presenter'
import { PreviewEvidenceRail } from './preview/evidence-rail'
import { PageRail } from './preview/page-rail'
import { diagnosticDismissalKey, RENDER_TARGET } from './preview/evidence-rail-facts'
import { pdfDigest } from './preview/pdf-digest'
import { isMacPlatform, primaryModifier, shortcutHintsFor } from './shortcuts'
import { DataPanel } from './DataPanel'
import { acceptSampleData, type SampleData, type SampleNode } from './sample-data'
import { StartupDialog, type StartupCard } from './StartupDialog'
import { BLANK_CHOICE_ID, DEFAULT_STARTUP_CHOICE_ID, startupChoices } from './startup-examples'
import type { ExampleAsset } from './generated/example-assets'
import type { SampleFileAccess } from './sample-file'
import { assetBytesRequest, setComponentAssetCommand } from './component-asset-command'
import { embeddedFaceFamily, isCarriedFaceAssetKey } from './embedded-face-family'
import { isShippedFaceName, shippedFaceFamily } from './shipped-face-family'
import { registerCarriedFaces } from './embedded-face-registry'
import type { ImageFileAccess } from './image-file'

const CANVAS_GUTTER = 116

// THE FILE BAR'S ENGINE STEPS ARE BOUNDED, AND THIS IS WHY THAT IS NOT
// PARANOIA.
//
// Open, Save, Save As and Start blank each hold `fileBusy` across their engine
// round-trip, and `fileBusy` is the sole condition that disables all four
// buttons. EngineClient rejects every pending request when the worker reports
// an error or is terminated (engine-client.ts's `#fail`/`terminate`), so a
// worker that DIES releases the bar. A worker that does not die — a wasm call
// that spins, or a Go instance that stops replying without raising — posts no
// response and raises no error, so its request settles never. Before this
// bound, that latched Open/Save/Save As/Start blank off for the life of the
// tab, with `cursor: not-allowed` and no sentence anywhere saying why; the only
// recovery was a reload nobody could know to perform.
//
// 20s is a CEILING ON A WEDGE, not a performance budget. Load and serialize are
// millisecond operations on every fixture in this repository, so a request that
// is still outstanding at 20s is not slow, it is stuck. It is deliberately not
// tighter: the bound must never fire on a large template on a loaded machine,
// because a false trip reports a failure that did not happen.
//
// ⚠ ABORTING RELEASES THE BAR; IT DOES NOT UNWEDGE THE WORKER. The abort drops
// the pending entry on THIS side (engine-client.ts's `abort`), and the worker
// thread stays exactly as stuck as it was. That is the honest division: the
// author gets their buttons and a stated failure instead of a dead bar, and the
// next action fails the same way rather than silently doing nothing.
//
// BUILT FROM `AbortController` AND `setTimeout`, NOT FROM `AbortSignal.timeout`.
// The one-liner reads better and cannot be tested: `AbortSignal.timeout` is
// implemented by the runtime and does not go through the global `setTimeout`
// that fake timers replace, so the only witness for this bound would be a test
// that really waits 20 seconds — which is to say, no witness at all. The timer
// is cleared when the request settles, so a normal file action leaves nothing
// pending behind it.
// A SETTLED STATUS RETIRES ITSELF AFTER THIS LONG. THE ALERT NEVER DOES.
//
// The file messages hang under the buttons they explain, over the canvas, so a
// finished outcome that stays forever is a sticker on the workspace: "Saved
// locally as x.folio" is worth reading once and is then just something in the
// way. An ERROR is the opposite — it is the only record of a failure the
// author will get, it is the half a truncation must never take, and it already
// clears itself the moment the next file action starts (`setFileError(undefined)`
// at the head of open/save/startBlank/export). Nothing retires it on a timer.
//
// ⚠ A BUSY STATUS IS NOT A SETTLED ONE, and the effect below is gated on
// `fileBusy` for that reason rather than for tidiness. "Opening local file…"
// describes a control that is disabled RIGHT NOW; retiring it after six
// seconds would restore the exact defect this bar was changed to fix — four
// buttons wearing `cursor: not-allowed` with nothing on screen saying why —
// and would do it only on the slow actions, which are the ones that need the
// sentence most.
const SETTLED_FILE_STATUS_MS = 6_000
const ENGINE_FILE_STEP_TIMEOUT_MS = 20_000
const engineFileStep = (run: (signal: AbortSignal) => Promise<EngineResult>): Promise<EngineResult> => {
  const deadline = new AbortController()
  const handle = setTimeout(() => deadline.abort(), ENGINE_FILE_STEP_TIMEOUT_MS)
  return run(deadline.signal).finally(() => clearTimeout(handle))
}

// A bundled example's file, from the offline release. Same-origin, never with
// credentials — the startup sequence fetches `starter.folio` the same way.
// It carries the file bar's deadline: a fetch that never settles would
// otherwise hold the dialog busy, with Escape and every action ignored, for ever.
const fetchExampleFile = async (url: string): Promise<ArrayBuffer> => {
  const deadline = new AbortController()
  const handle = setTimeout(() => deadline.abort(), ENGINE_FILE_STEP_TIMEOUT_MS)
  try {
    const response = await fetch(url, { credentials: 'omit', signal: deadline.signal })
    if (!response.ok) throw new Error(`its bundled file could not be read (HTTP ${response.status})`)
    return await response.arrayBuffer()
  } catch (error) {
    if (deadline.signal.aborted) throw new Error('its bundled file did not arrive in time')
    throw error
  } finally { clearTimeout(handle) }
}

const paletteItems: ReadonlyArray<readonly [string, PaletteKind]> = [['Text', 'text'], ['Image', 'image'], ['Table', 'table'], ['Line', 'line'], ['Rectangle', 'rect'], ['Barcode', 'barcode'], ['QR Code', 'qrcode']]
type InspectorTab = 'properties' | 'data'
const inspectorTabs: ReadonlyArray<readonly [InspectorTab, string, string]> = [['properties', 'PROPERTIES', 'INPUTS'], ['data', 'DATA', 'DATA']]
const EMPTY_PARAMETER_DOCUMENT = '{}'
const MAX_PARAMETER_DOCUMENT_BYTES = 8 * 1024 * 1024

// Sample inspection is strictly a local affordance. These values are never
// sent with a command; Go still owns all collection and field admission.
// The one spelling of "this JSON key is addressable as a folio8 path segment".
// It was written out twice — once here and once in the item-count walk below —
// over the SAME `segments.join('.')` collection spelling, so the two could
// disagree about which nodes are addressable while appearing to agree.
const SAMPLE_PATH_SEGMENT = /^[A-Za-z_][A-Za-z0-9_]*$/
// STORY 14.10 — THE RETURN IS WIDENED, AND `candidates` IS BYTE-FOR-BYTE WHAT
// IT ALWAYS WAS.
//
// The DATA panel now has to answer "is THIS tree node one of this table's row
// fields, and which one" — and it cannot work that out for itself: a row-scope
// leaf carries NO `segments` at all (`sample-data.ts`'s `array()` recurses with
// a hardcoded `rootScoped: false`), so the row-relative path the engine wants is
// simply not on the node. Q2(a) therefore threads THIS walk's answer through
// rather than deriving the path a second time, because a second derivation would
// owe a proof that the two agree and there is only one.
//
// `byNode` IS BUILT AFTER THE 50-CANDIDATE SLICE, deliberately: the panel's
// pickable set must be EXACTLY `candidates`, and a node whose candidate was
// sliced off would otherwise be offered by the panel and be absent from the
// editor's datalist — two sets again, wearing one function's name.
//
// One (collection, field) pair can be reached from SEVERAL item nodes — the
// parser keeps up to `SAMPLE_LIMITS.items` children — so every one of those
// nodes maps to the one deduplicated candidate.
export type TableCandidate = Readonly<{ collection: string; field: string }>
export type TableCandidateScan = Readonly<{ candidates: ReadonlyArray<TableCandidate>; byNode: ReadonlyMap<SampleNode, TableCandidate> }>
const EMPTY_TABLE_CANDIDATE_SCAN: TableCandidateScan = { candidates: [], byNode: new Map() }
export function tableSampleCandidates(root: SampleNode | undefined): TableCandidateScan {
  if (!root) return EMPTY_TABLE_CANDIDATE_SCAN
  const candidates = new Map<string, TableCandidate>()
  const sources = new Map<string, SampleNode[]>()
  const visit = (node: SampleNode) => {
    if (node.kind === 'collection' && node.segments?.length && node.segments.every((part) => SAMPLE_PATH_SEGMENT.test(part))) {
      const collection = `${node.segments.join('.')}[]`
      const fields = (value: SampleNode, prefix: ReadonlyArray<string>): void => {
        if (value.kind === 'collection' || value.kind === 'truncated') return
        if (value.kind === 'object') { value.children.forEach((child) => fields(child, [...prefix, child.label])); return }
        if (prefix.length && prefix.every((part) => SAMPLE_PATH_SEGMENT.test(part))) {
          const field = prefix.join('.')
          const key = `${collection}\u0000${field}`
          candidates.set(key, { collection, field })
          const seen = sources.get(key)
          if (seen) seen.push(value); else sources.set(key, [value])
        }
      }
      node.children.forEach((item) => fields(item, []))
    }
    node.children.forEach(visit)
  }
  visit(root)
  const admitted = [...candidates.entries()].sort(([, a], [, b]) => a.collection.localeCompare(b.collection) || a.field.localeCompare(b.field)).slice(0, 50)
  const byNode = new Map<SampleNode, TableCandidate>()
  for (const [key, candidate] of admitted) for (const node of sources.get(key) ?? []) byNode.set(node, candidate)
  return { candidates: admitted.map(([, candidate]) => candidate), byNode }
}

// STORY 14.7 — HOW MANY ITEMS THE LOADED SAMPLE HOLDS FOR THE TABLE'S OWN
// COLLECTION, spelled the way the Table Editor's collection field spells it
// (`transactions[]`).
//
// ⚠ `node.count` IS THE TRUE COUNT AND `node.children.length` IS NOT. The
// parser keeps only `SAMPLE_LIMITS.items` children while `count++` runs on
// every item, so a 34-item collection arrives with 5 children and a count of
// 34. Reading the children would understate every sample the parser truncated,
// which is most of them.
//
// ⚠ AND `undefined` MEANS UNKNOWN, NEVER ZERO. A collection the parser
// truncated away is a `kind: 'truncated'` node with no `count` at all; an empty
// collection is a `kind: 'collection'` node whose count is 0. The scope header
// tells those two apart because this function does.
//
// This reads what the browser already holds. Nothing is added to the wire
// projection and no Go file is touched: the item count is sample-inspection
// data, which has never been the engine's to answer for.
// ⚠ FIRST MATCH WINS, AND "MATCHED" IS TRACKED SEPARATELY FROM "COUNTED".
// Keying the stop condition on the count itself meant a matching node that
// carried NO count did not stop the walk — so the search carried on into its
// siblings and could answer with a DIFFERENT node that happens to join to the
// same path. The two facts are different: whether the collection was found, and
// what it counted. Only the first ends the walk.
//
// The identifier test is the same one `tableSampleCandidates` applies, from the
// same constant, so the two readers agree about which nodes are addressable.
// There is no `collection === ''` guard: `isTableColumns` admits a projection
// only when `table.collection.length > 0`.
function tableSampleItemCount(root: SampleNode | undefined, collection: string): number | undefined {
  if (!root) return undefined
  let matched = false
  let count: number | undefined
  const visit = (node: SampleNode) => {
    if (matched) return
    if (node.kind === 'collection' && node.segments?.length && node.segments.every((part) => SAMPLE_PATH_SEGMENT.test(part)) && `${node.segments.join('.')}[]` === collection) { matched = true; count = node.count; return }
    node.children.forEach(visit)
  }
  visit(root)
  return count
}

type ParameterReferenceState = Readonly<{ status: 'pending' | 'ready' | 'failed'; names: ReadonlyArray<string> }>

const paletteGlyphs: Readonly<Record<PaletteKind, ReactNode>> = {
  text: <><path d="M3.5 4.5V3h9v1.5" /><path d="M8 3v10" /><path d="M5.75 13h4.5" /></>,
  image: <><path d="M2.5 3.5h11v9h-11z" /><path d="M2.5 10.25 5.75 7l2.25 2.25 2-2 3.5 3.5" /><circle cx="10.5" cy="6.25" r="1" /></>,
  table: <><path d="M2.5 3.5h11v9h-11z" /><path d="M2.5 6.5h11" /><path d="M6.5 6.5v6" /><path d="M10 6.5v6" /></>,
  line: <><path d="M3 12.5 13 3.5" /></>,
  rect: <><path d="M2.5 4.5h11v7h-11z" /></>,
  barcode: <><path d="M3 3.5v9" /><path d="M5.5 3.5v9" /><path d="M7 3.5v9" /><path d="M9.5 3.5v9" /><path d="M12.5 3.5v9" /></>,
  qrcode: <><path d="M2.5 2.5h4v4h-4z" /><path d="M9.5 2.5h4v4h-4z" /><path d="M2.5 9.5h4v4h-4z" /><path d="M9.5 9.5h1.5v1.5" /><path d="M13.5 12v1.5h-2" /></>,
}

// spec-section-break: the Section Break entry's glyph — a rule between two blocks.
function SectionBreakIcon() {
  return <svg aria-hidden="true" className="palette-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.25" strokeLinecap="square"><path d="M4 3.5h8" /><path d="M1.5 8h13" /><path d="M4 12.5h8" /></svg>
}

// spec-section-break CAP-7: the anchor drawn in the SECTION BREAK tab while the
// break is anchored — stroked, on SectionBreakIcon's 16px grid, sized to the tab.
function SectionBreakAnchorIcon() {
  return <svg aria-hidden="true" className="section-break-anchor-icon" data-testid="section-break-anchor-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="square"><path d="M8 5v9" /><path d="M8 1.5a1.75 1.75 0 1 1 0 3.5 1.75 1.75 0 0 1 0-3.5z" /><path d="M5 7.5h6" /><path d="M2.5 9.5c0 2.5 2.5 4.5 5.5 4.5s5.5-2 5.5-4.5" /></svg>
}

function PaletteIcon({ kind }: { kind: PaletteKind }) {
  return <svg aria-hidden="true" className="palette-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.25" strokeLinecap="square">{paletteGlyphs[kind]}</svg>
}

type AppProps = Readonly<{ engine?: EngineClient; fileAccess?: FileAccess; sampleFileAccess?: SampleFileAccess; imageFileAccess?: ImageFileAccess; initialSnapshot?: EngineSnapshot; initialSampleData?: SampleData; blankBytes?: ArrayBuffer; initializationError?: string; offlineState?: OfflineLifecycleState; loadState?: OfflineLifecycle; payload?: S1Payload; engineState?: 'waiting' | 'starting' | 'failed'; onRetry?: () => void; examples?: ReadonlyArray<ExampleAsset> }>
// The document carries no readable face until one is registered, and this is
// the value that says so. A stable reference, so resetting it between
// documents is not itself a state change React has to re-render for.
const NO_CARRIED_FACES: ReadonlySet<string> = new Set()
// The same trick for the machine store's listing: an empty store and a store
// that could not be opened both render nothing, and neither should re-render
// the tree for the privilege.
const NO_FACE_MISSES: ReadonlyArray<string> = []
// A stable identity, for the reason `NO_STORED_FACES` is one: a fresh `new Set()`
// in a `useState` initialiser is a new value on every render.
const NO_FACE_MISS_DISMISSALS: ReadonlySet<string> = new Set()
const NO_STORED_FACES: ReadonlyArray<StoredFace> = []
/**
 * THE PROPOSED FALLBACK TAIL — the shipped faces for the scripts the picked
 * face does not cover, in the order `scriptFallbackFaces` names them, each
 * declaring the cuts that face has.
 *
 * ⚠ IT IS ONE COMPUTATION WITH TWO CALLERS, AND THAT IS THE WHOLE REASON IT IS
 * A FUNCTION. Story 11.4 gave a pick two paths — `dispatchEmbed` for a family
 * that has to travel, `declareShippedFamily` for one the release already ships
 * — and the second was written with NO tail at all. A `Roboto` pick, whose own
 * fallback tail is the two entries `starter.folio` itself declares, produced a
 * ONE-entry chain: latin kept working and every Thai and CJK run in the
 * document silently lost its fallback. A pick must never yield a chain with
 * less script coverage than the path it replaced, and two implementations that
 * agree today are how that comes back.
 *
 * A FALLBACK OUTSIDE THE MIRROR FALLS BACK TO A BARE NAME, which is the same
 * entry today's pick writes: a family whose cuts are not declared has none to
 * declare, and that is an honest entry rather than a degraded one. `S1`'s
 * build-time throw in `scripts/build-wasm.mjs` is what keeps that fallback from
 * silently swallowing a real drift between `scriptFallbacks` and the mirror.
 */
const proposedFallbackTail = (scripts: ReadonlyArray<string>): ReadonlyArray<FontChainEntryAsk> =>
  scriptFallbackFaces.filter(([script]) => !scripts.includes(script)).map(([, shipped]) => shippedFamilyEntry(shipped) ?? shipped)

/**
 * The scripts a pickable row's face covers, read off whichever tier the row is.
 * Every tier carries them; only the field they sit in differs, and spelling the
 * discriminant here keeps the narrowing the compiler's rather than a comment's.
 */
const scriptsOfSource = (source: FamilySource): ReadonlyArray<string> => {
  if (source.tier === 'local') return source.face.scripts
  if (source.tier === 'stored') return source.record.scripts
  return source.row.scripts
}

// STORY 13.4 — `standIn` IS ON THE RECORD, NOT DERIVED FROM `sampleData`.
// A no-data render that is later superseded by a real one stays visible while
// it is stale, and the screen must keep withholding the production claim for
// the bytes it is actually showing. `!sampleData` answers "what would we build
// now"; this answers "what were these bytes built from", which is the question
// the label and the digest line are about.
// STORY 13.3 — `elapsedMs` AND `version` ARE ON THE RECORD FOR THE REASON
// `standIn` IS. They describe the render that produced THESE bytes, so a stale
// record keeps reporting its own render's numbers while its own PDF is on
// screen. Reading them off anything current would describe a render the author
// is not looking at.
// STORY 13.5 — `installedAt` OBEYS THAT SAME RULE, and obeying it is the whole
// of "leave Preview and come back and the age keeps counting from the render".
// It is the instant THESE bytes were installed, stamped once at the single
// install site and never re-stamped, so it survives a trip through Design
// exactly as `elapsedMs` does. A stamp taken on entering Preview would be a
// fact about the author's navigation, not about the render on screen.
// The name is `installedAt` rather than anything shorter for a mechanical
// reason as well: `engine-ownership-contract.test.ts` refuses a type literal
// carrying two or more of `version`/`page`/`bands`/`elements`/`assets`, and
// this record already carries `version`.
type PreviewRecord = Readonly<{ bytes: ArrayBuffer; revision: number; identity: string; digest: string; diagnostics: ReadonlyArray<EngineDiagnostic>; token: number; generation: number; standIn: boolean; elapsedMs: number; version: string; installedAt: number }>
type PreviewFailureRecord = Readonly<{ error: EngineError; token: number; generation: number; revision: number }>

export default function App({ engine, fileAccess, sampleFileAccess, imageFileAccess, initialSnapshot, initialSampleData, blankBytes, initializationError, offlineState = 'unavailable', loadState, payload, engineState = 'waiting', onRetry = () => undefined, examples }: AppProps = {}) {
  const [snapshot, setSnapshot] = useState(initialSnapshot)
  const [commitError, setCommitError] = useState<string>()
  const [propertyError, setPropertyError] = useState<PropertyCommitError>()
  const [fontChainError, setFontChainError] = useState<FontChainCommitError>()
  const [fontChainBusy, setFontChainBusy] = useState(false)
  // THE BUSY FLAG IS ALSO HELD IN A REF, because the window it now guards is a
  // NETWORK CHAIN. Since Story 16.1 a web-tier pick awaits up to six sequential
  // cross-origin round-trips before any command is sent, and a React state read
  // inside an event handler is the value that handler CLOSED OVER: two picks
  // dispatched before the re-render both see `false`, both resolve, and two
  // embeds commit. The ref is the same value read at the instant of the call.
  const fontChainBusyRef = useRef(false)
  const holdFontChain = (busy: boolean) => { fontChainBusyRef.current = busy; setFontChainBusy(busy) }
  const [fileError, setFileError] = useState<string>()
  const [fileStatus, setFileStatus] = useState<string>()
  const [fileBusy, setFileBusy] = useState(false)
  const [title, setTitle] = useState('Untitled template')
  const [target, setTarget] = useState<FileTarget>()
  const [savedRevision, setSavedRevision] = useState<number>()
  // THE STARTUP DIALOG (spec-startup-templates, story 3). It opens once per
  // launch: `main.tsx` hands over the examples only when the engine is ready,
  // and App is remounted (`key`) at that moment, so this initialiser runs once
  // with both present. Mounted without examples — every unit test that does not
  // ask for them — there is no dialog and App behaves as it always has.
  const [startupOpen, setStartupOpen] = useState(() => examples !== undefined && engine !== undefined)
  const [startupSelected, setStartupSelected] = useState(DEFAULT_STARTUP_CHOICE_ID)
  const [startupBusy, setStartupBusy] = useState<string>()
  const [startupError, setStartupError] = useState<string>()
  // STORY 4. The dialog has two origins. At launch the untouched starter already
  // is Blank, so Blank, Cancel and Escape only close it. After New… Blank must
  // really replace whatever is open, and Cancel leaves that document alone.
  const [startupOrigin, setStartupOrigin] = useState<'launch' | 'new'>('launch')
  // THE UNSAVED-CHANGES WARNING (CAP-6, owner renegotiation). New… on a document
  // with real edits opens this first; only its Discard opens the startup
  // dialog, and nothing inside that dialog asks again.
  const [unsavedWarningOpen, setUnsavedWarningOpen] = useState(false)
  // Per-tab and deliberately not persisted. "Later" means later in THIS sitting;
  // a reload is already a chance to take the update, so remembering the refusal
  // across one would be remembering it past the moment it was about.
  const [updateDismissed, setUpdateDismissed] = useState(false)
  // THE REVISION THE DOCUMENT HAD WHEN IT WAS LAST STARTED, OPENED, OPENED AS AN
  // EXAMPLE OR SAVED. Separate from `savedRevision`, which drives the bar's
  // "Unsaved local changes" label: an untouched starter, example or opened file
  // is "unsaved" there but has no real edits to lose, so it is replaced without
  // asking. Only a different revision from this one warns.
  const [baselineRevision, setBaselineRevision] = useState(initialSnapshot?.revision)
  // Blank is always a card, with or without examples, so New… works everywhere.
  const startupCards: ReadonlyArray<StartupCard> = startupChoices.flatMap((choice): StartupCard[] => {
    if (choice.id === BLANK_CHOICE_ID) return [choice]
    const asset = examples?.find((example) => example.id === choice.id)
    return asset ? [{ ...choice, thumbnail: asset.thumbnail }] : []
  })
  const [zoom, setZoom] = useState(1)
  const [gridVisible, setGridVisible] = useState(true)
  const [snapEnabled, setSnapEnabled] = useState(true)
  // STORY 12.5. THE BOUNDARY DRAG, and it is TRANSIENT UI STATE and nothing
  // else (AD-15). No band rect is re-placed while it is live — the three
  // .page-band sections keep the geometry the engine projected for the whole
  // gesture (AD-24) — and no command is sent until release. What moves is one
  // proposed line and one readout.
  //
  // The REF is what the handlers read, for the reason the prose resize records:
  // a gesture must be IDENTIFIED, not merely "in progress", or a second
  // pointer's press rebases the anchor under the first one's drag. The state is
  // what renders.
  const [boundaryDrag, setBoundaryDrag] = useState<BoundaryDrag>()
  const boundaryDragRef = useRef<BoundaryDrag | undefined>(undefined)
  // ONE ABORT, reachable without a pointer event, because three different
  // things end a boundary gesture with nothing sent: pointercancel, Escape, and
  // the document being replaced underneath it (undo, load, Start blank). The
  // last is why clearInteraction below calls this: a release that commits
  // against a projection the author has already left would send a height read
  // off a document that is gone.
  const abortBoundaryDrag = () => { boundaryDragRef.current = undefined; setBoundaryDrag(undefined) }
  // spec-section-break: the palette's Section Break entry arms placement too,
  // without joining PaletteKind — it creates no component.
  const [placing, setPlacing] = useState<PaletteKind | 'sectionBreak'>()
  // THE SECTION BREAK'S OWN SELECTION, never a component id: it joins no bulk
  // edit, group move, copy, duplicate or select all. installSelection clears it.
  // SPEC-multi-pages story 5: every page has its own break, so the selection
  // is the 0-based page whose break is selected.
  const [sectionBreakSelected, setSectionBreakSelected] = useState<number>()
  // SPEC-multi-pages story 2: THE SELECTED DESIGNED PAGE, 0-based. Designer
  // state only, never saved: selecting an element or the section break,
  // Escape, and undo, redo or a replaced document all clear it.
  const [selectedPage, setSelectedPage] = useState<number>()
  // SPEC-multi-pages story 4 (D-G.2): THE CURRENT PAGE, 0-based — the page the
  // author last selected something on. The shared header and footer draw their
  // one interactive, accessibly named copy on this page's first sheet. Designer
  // state only; the ref lets installSelection read it without a stale closure.
  const [currentPageState, setCurrentPageState] = useState(0)
  const currentPageRef = useRef(0)
  const setCurrentPage = (page: number) => {
    // A header or footer copy holding focus unmounts when the current page
    // moves; focus follows it to its new named copy.
    const focused = document.activeElement instanceof HTMLElement ? document.activeElement.closest<HTMLElement>('.page-band-pageHeader [data-component-id], .page-band-pageFooter [data-component-id]') : null
    if (page !== currentPageRef.current && focused?.dataset.componentId) setPendingFocus({ id: focused.dataset.componentId, from: document.activeElement, preventScroll: true })
    currentPageRef.current = page; setCurrentPageState(page)
  }
  // The page a Delete page confirmation is open for.
  const [pageDeleteConfirm, setPageDeleteConfirm] = useState<number>()
  const deletePageButtonRef = useRef<HTMLButtonElement>(null)
  const [sectionBreakDrag, setSectionBreakDrag] = useState<SectionBreakDrag>()
  const sectionBreakDragRef = useRef<SectionBreakDrag | undefined>(undefined)
  const abortSectionBreakDrag = () => { sectionBreakDragRef.current = undefined; setSectionBreakDrag(undefined) }
  // A placed break takes focus once its handle is mounted (selectPlaced's rule).
  // The page whose break handle is waiting for focus.
  const [pendingBreakFocus, setPendingBreakFocus] = useState<number>()
  // STORY 14.3. The component a placement is still trying to focus, and where
  // focus was when it made the claim. Transient chrome: it names no document
  // state, sends nothing, and is dropped the moment the claim is honoured,
  // withdrawn, or outlived by its component. See the effect beside
  // `selectPlaced` for why this is state and not a timer.
  const [pendingFocus, setPendingFocus] = useState<Readonly<{ id: string; from: Element | null; preventScroll?: boolean }>>()
  // Where the armed palette kind is following the pointer. One transient
  // client coordinate for chrome that never touches document geometry: it
  // places no component and proposes nothing to Go.
  const [placingAt, setPlacingAt] = useState<Readonly<{ x: number; y: number }>>()
  // The hovered band, keyed by SHEET as well as by band: three sheets carry
  // three content bands, and highlighting all of them would be exactly the
  // ambiguous drop target this epic's own rule forbids.
  const [hoverBand, setHoverBand] = useState<string>()
  const [selected, setSelected] = useState<ReadonlyArray<string>>([])
  // STORY 14.10 — THE COLUMN SELECTION, AND IT IS NOT IN `selected` (Q1(b)).
  //
  // `selected` holds ELEMENT IDS and continues to: all 32 read expressions over
  // it were written on that premise, `openTableEditor` guards on
  // `selectedRef.current[0] !== id`, and a compound id in there would have
  // killed the table's own "Configure columns" button — working function
  // repaired to accommodate this story's own state shape. AD-15 / I-4 puts
  // transient interaction state in the UI; this is that state, and nothing here
  // reaches the document.
  //
  // ⚠ KEYED ON BOTH IDS, and both halves are load-bearing. A bare column id is
  // not unique across tables, and a column id that outlives its table would
  // resolve against whichever table happened to reuse the spelling.
  //
  // ⚠ IT HOLDS IDS, NEVER THE COLUMN OBJECT. `selectedTableColumn` below
  // re-resolves it from the projection on EVERY render, so a column removed in
  // the editor stops resolving and the selection simply drops.
  const [columnSelection, setColumnSelection] = useState<Readonly<{ tableId: string; columnId: string }>>()
  const [drag, setDrag] = useState<DragState>()
  const [preset, setPreset] = useState<string>(initialSnapshot?.canvas?.preset ?? 'A4')
  const [orientation, setOrientation] = useState<string>(initialSnapshot?.canvas?.orientation ?? 'portrait')
  const [draft, setDraft] = useState(() => draftFor(initialSnapshot?.canvas))
  const [mode, setMode] = useState<'design' | 'preview'>('design')
  // One right-hand inspector with two tabs, per the UX design. The tab is a
  // purely local view preference; it never changes what the engine owns.
  const [inspectorTab, setInspectorTab] = useState<InspectorTab>('properties')
  const [preview, setPreview] = useState<PreviewRecord | undefined>(undefined)
  const [previewStatus, setPreviewStatus] = useState<'idle' | 'checking' | 'debouncing' | 'rendering' | 'current' | 'stale' | 'error'>('idle')
  const [staleReason, setStaleReason] = useState<'inputs-changed' | 'render-failed'>('inputs-changed')
  const [previewError, setPreviewError] = useState<PreviewFailureRecord>()
  const [previewIssue, setPreviewIssue] = useState<string>()
  // STORY 13.5 — THE `now` THE AGE IS MEASURED AGAINST, held as state rather
  // than read during render. `formatRenderAge` is pure and takes
  // `(installedAt, now)`; this is the `now` it is given, and it is advanced by
  // the ticking effect below and by nothing else. Reading `Date.now()` inside
  // the JSX instead would make the render impure and the ladder untestable
  // without fake timers.
  //
  // THE CLOCK IS READ IN EXACTLY THREE PLACES, all of them in this file and all
  // of them outside render: this initializer, the install stamp in `runPreview`,
  // and the ticker's own `tick`/mount reads. Population searched: the whole of
  // `folio-designer/src` with `grep -arn` (so `App.tsx`'s two NUL bytes cannot
  // hide a fourth); the only other hit anywhere is the vendored Go wasm shim
  // under `src/generated/runtime`, which is not ours. What is single here is
  // not the call count but the SOURCE: every age on screen is this one value
  // minus one stamp, so no two parts of the chrome can be dating the render
  // against different clocks.
  const [renderAgeNow, setRenderAgeNow] = useState(() => Date.now())
  const [dismissedDiagnostics, setDismissedDiagnostics] = useState<ReadonlySet<string>>(new Set())
  const [undoAvailable, setUndoAvailable] = useState(initialSnapshot?.canUndo === true)
  const [redoAvailable, setRedoAvailable] = useState(initialSnapshot?.canRedo === true)
  const [locateStatus, setLocateStatus] = useState<string>()
  const [previewViewState, setPreviewViewState] = useState<PDFPreviewViewState>(initialPDFPreviewViewState)
  // THE PAGE COUNT LIVES HERE NOW, because the page indicator does. The
  // viewer has always reported it through `onPageCount`; App used the call
  // only to promote the preview's status and threw the number itself away,
  // which is why a status-bar indicator had nothing to read.
  const [previewPages, setPreviewPages] = useState<number>()
  // Two uncommitted typed entries. `undefined` means "the field is reading the
  // view state", which is also how a refused entry puts the real value back.
  const [previewPageDraft, setPreviewPageDraft] = useState<string>()
  const [previewZoomDraft, setPreviewZoomDraft] = useState<string>()
  // Accepted bytes and editor draft are intentionally separate. The engine
  // receives only accepted raw text; invalid local input cannot silently turn
  // into an alternate runtime value.
  const [previewParams, setPreviewParams] = useState(EMPTY_PARAMETER_DOCUMENT)
  const [previewParamsDraft, setPreviewParamsDraft] = useState(EMPTY_PARAMETER_DOCUMENT)
  const [previewParamsError, setPreviewParamsError] = useState<string>()
  const [parameterReferenceState, setParameterReferenceState] = useState<ParameterReferenceState>({ status: 'pending', names: [] })
  const [sampleData, setSampleData] = useState<SampleData | undefined>(initialSampleData)
  const [sampleError, setSampleError] = useState<string>()
  const [sampleBusy, setSampleBusy] = useState(false)
  const [bindingError, setBindingError] = useState<BindingErrorScope>()
  const [bindingBusy, setBindingBusy] = useState(false)
  const [assetError, setAssetError] = useState<Readonly<{ id: string; message: string }>>()
  const [assetBusy, setAssetBusy] = useState(false)
  const [tableEditor, setTableEditor] = useState<TableColumns>()
  const [tableEditorBusy, setTableEditorBusy] = useState(false)
  const [tableEditorError, setTableEditorError] = useState<string>()
  const snapshotRef = useRef(snapshot)
  const saveInFlight = useRef(false)
  // STORY 13.1. A SECOND IN-FLIGHT LATCH, and it is a REF for the reason the
  // font-chain one is: the guard is read at the instant of the click, and a
  // React state read inside a handler is the value that handler closed over —
  // two presses dispatched before the re-render would both see `false` and both
  // open a picker. It is deliberately its own latch rather than `saveInFlight`:
  // a PDF save and a template save are different writes to different files, and
  // conflating them would make one silently swallow the other's press.
  const exportInFlight = useRef(false)
  // STORY 5 (startup templates). A THIRD LATCH, for the reason 13.1 minted the
  // second: the sample save is a write to a THIRD file, and folding it into
  // either of the others would let one press silently swallow the other's.
  const sampleSaveInFlight = useRef(false)
  // ONE APPLY AT A TIME. bindingInFlight is the shipped precedent; this one
  // exists because Apply became a SEQUENCE of commands rather than a single
  // one, and two interleaved sequences would send band heights derived from
  // drafts either of them may already have superseded.
  const pageSetupInFlight = useRef(false)
  const draftGeneration = useRef(0)
  const documentGeneration = useRef(0)
  // CANVAS CLIPBOARD. It holds ids, never element data: paste copies the live
  // originals as they are at paste time, and a clipboard from another document
  // generation is ignored. It never touches the OS clipboard.
  //
  // It is keyed on DOCUMENT IDENTITY, which advances only when a different
  // document is installed (Open, Start blank) — never on undo/redo, which bump
  // `documentGeneration`. `lists` is the stair-step history, newest last: paste
  // uses the newest list that still has ids in the document.
  const documentIdentity = useRef(0)
  const clipboardRef = useRef<Readonly<{ identity: number; lists: ReadonlyArray<ReadonlyArray<string>> }>>(undefined)
  // SPEC-multi-pages: the designed page whose sheet the pointer is over, read
  // off the event target's `data-page` (never a coordinate). A paste lands there.
  const pointerPageRef = useRef<number | undefined>(undefined)
  // One keyboard delete or paste at a time: a second press or key repeat must
  // not duplicate the same sources again or delete ids already gone.
  const mutationInFlight = useRef(false)
  // FOCUS AREA: the band of the most recent canvas press, keyboard focus or
  // selection change. Select All selects every component in it.
  const focusBandRef = useRef<CanvasProjection['bands'][number]['name']>('content')
  const installDocumentIdentity = () => { documentIdentity.current++; clipboardRef.current = undefined; focusBandRef.current = 'content' }
  const [documentGenerationValue, setDocumentGenerationValue] = useState(0)
  // The asset keys whose faces have ACTUALLY reached the page's font set.
  // Not the keys the document declares: a fragment may only ask for a derived
  // family once there is a face registered under it, or a failed fetch would
  // move it off the stylesheet's declared stack onto a family nothing
  // declares. See the registration effect below.
  const [carriedFaces, setCarriedFaces] = useState<ReadonlySet<string>>(NO_CARRIED_FACES)
  // STORY 16.2 — THE MACHINE FONT STORE. What this machine holds, kept for the
  // family control's `AVAILABLE LOCALLY` group. A private window, cleared site
  // data or a quota refusal leaves a WORKING designer with an empty group —
  // Story 16.6 deleted the panel that used to say so on screen, deliberately
  // reversing 16.2's stated-degradation clause.
  const [storedFaces, setStoredFaces] = useState<ReadonlyArray<StoredFace>>(NO_STORED_FACES)
  // WHETHER THIS BROWSER CAN KEEP TYPEFACES AT ALL. Optimistic until the store
  // answers, because the modal cannot be open before it has: flashing the
  // degraded copy and then withdrawing it would be a worse lie than either state.
  const [storeKeepsFaces, setStoreKeepsFaces] = useState(true)
  // WHICH CATALOGUE FAMILIES THIS BROWSER ACTUALLY HOLDS
  // (spec-deferred-offline-cache, story 2). The 31 catalogue faces are deferred
  // now, so shipping in the release no longer means being on this machine, and
  // `familyIsInstalled` is a function of THIS rather than of the tier alone.
  //
  // THE FIRST PAINT'S ANSWER IS `initialHeldLocalFamilies`, which argues itself:
  // empty where there is a cache to probe, and every catalogue family where
  // there is no Cache API and therefore no deferral to be honest about.
  const [heldLocalFamilies, setHeldLocalFamilies] = useState<ReadonlySet<string>>(() => initialHeldLocalFamilies(payload?.releaseId))
  // RE-READ, NEVER INCREMENTED. The release cache is the authority and it can
  // LOSE entries — eviction, cleared site data — so a set this page only ever
  // added to would go on claiming a face that is gone. Every caller re-probes.
  const refreshHeldLocalFamilies = useCallback(() => { void readHeldLocalFamilies(payload?.releaseId).then((held) => setHeldLocalFamilies(held)) }, [payload?.releaseId])
  // THE THREE MOMENTS THE ANSWER CAN HAVE CHANGED, and none of them is a timer:
  // mount, opening the typography dialog's family list, and finishing an
  // install. `cacheReady` is deliberately NOT one of them — readiness is the
  // core tier and says nothing about any catalogue face.
  useEffect(() => { refreshHeldLocalFamilies() }, [refreshHeldLocalFamilies])
  // A CANVAS MISS SUBSTITUTES AND SAYS SO ONCE (spec-deferred-offline-cache,
  // story 2, owner decision 2026-09-19).
  //
  // WHAT IS AND IS NOT AT STAKE. The canvas paints with CSS `@font-face` rules
  // from `generated/runtime-fonts.css`, and a deferred face behind one of those
  // rules can now fail to arrive — offline, or a fetch that will not verify.
  // The ENGINE is unaffected: `folio-go/fonts/fonts.go` embeds its own copies,
  // so metrics, line breaks and the previewed PDF are exactly what they would
  // have been. Only the glyphs drawn on this screen differ, and the browser has
  // already substituted by the time this fires.
  //
  // SO IT IS A WARNING AND NEVER A BLOCK. A modal over a document that is
  // rendering correctly would be the designer lying about the severity of its
  // own cosmetic degradation. It is per occurrence, it names the family, and it
  // is dismissible — the author may want to fetch the face, or may not care.
  //
  // `loadingerror` IS THE BROWSER'S OWN REPORT, which is the only thing that
  // knows a substitution happened; nothing here probes or predicts. The
  // subscription itself lives in `canvas-face-misses.ts`, which is where its
  // relationship to AD-17 is argued and bounded.
  const [canvasFaceMisses, setCanvasFaceMisses] = useState<ReadonlyArray<string>>(NO_FACE_MISSES)
  // DISMISSAL IS STICKY FOR THE SESSION, AND IT HAS TO BE KEPT SEPARATELY TO BE.
  // Filtering the list alone is not a dismissal: the browser retries a missing
  // face every time something asks for it, so the next `loadingerror` would put
  // the row the author just closed straight back on screen. A warning that
  // cannot be got rid of is worse than no warning.
  const [dismissedFaceMisses, setDismissedFaceMisses] = useState<ReadonlySet<string>>(NO_FACE_MISS_DISMISSALS)
  // ONE ROW PER FAMILY, NOT ONE PER EVENT. A page can ask for the same face
  // several times in a second, and one row per attempt would be a wall of the
  // same sentence about one missing font.
  useEffect(() => watchCanvasFaceMisses((families) => setCanvasFaceMisses((current) => [...current, ...families.filter((family) => !current.includes(family))])), [])
  const shownFaceMisses = canvasFaceMisses.filter((family) => !dismissedFaceMisses.has(family))
  // STORY 16.3 — THE FONT BROWSER IS OPEN OR IT IS NOT, AND THAT IS ALL THE
  // STATE IT HAS UP HERE. Everything the modal knows — the query, the chips,
  // the sort, the staged families — lives inside it and dies with it, which is
  // what keeps a Cancel/Apply pair from becoming a second document model
  // (AD-15). The one thing App owns is whether it is on screen.
  const [fontBrowserOpen, setFontBrowserOpen] = useState(false)
  const [machineFaces, setMachineFaces] = useState<ReadonlySet<string>>(NO_CARRIED_FACES)
  // THE HANDLE IS A PROMISE, NOT A RESOLVED VALUE, AND THAT IS A CORRECTNESS
  // POINT RATHER THAN A STYLE ONE. Opening a database is asynchronous, so a ref
  // holding the OPENED store is empty for the first moments of the session — and
  // a pick in that window would silently skip the store, keeping nothing, with
  // no failure anywhere to show it. Holding the OPENING lets every caller await
  // the same one open, whenever it asks.
  const fontStore = useRef<Promise<FontStore | undefined> | undefined>(undefined)
  const selectedRef = useRef(selected)
  const previewToken = useRef(0)
  const previewAbort = useRef<AbortController | undefined>(undefined)
  const previewTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const previewScheduler = useRef(new PreviewWorkScheduler())
  const previewRef = useRef<PreviewRecord | undefined>(undefined)
  const previewGeneration = useRef(0)
  const retryingFailure = useRef<number | undefined>(undefined)
  const previewNeedsFreshRender = useRef(false)
  const sampleDataRef = useRef(sampleData)
  const bindingInFlight = useRef(false)
	const tableEditorSession = useRef(0)
	const tableEditorInvoker = useRef<HTMLElement | undefined>(undefined)
  // STORY 14.7b — THE DIALOG'S EDIT COUNT, HELD AS A REF AND A STATE MIRROR
  // WRITTEN TOGETHER, exactly as `documentGeneration` /
  // `setDocumentGenerationValue` already are (setCurrentSnapshot, below).
  //
  // The REF is what the Cancel loop reads: it runs N synchronous-ish iterations
  // inside one handler, and a React state read there is the value that handler
  // closed over. The STATE is what the footer renders, because a ref cannot
  // re-render the disabled reason into view.
  //
  // IT COUNTS ONLY COMMITS THAT ACTUALLY MOVED THE DOCUMENT — the engine's own
  // `revision !== priorRevision` answer, never a dispatch. A no-op that consumed
  // a Cancel step would make Cancel reach back PAST the moment the dialog
  // opened and unwind work the author did before it, which is the one
  // destructive failure this whole mechanism has.
  //
  // IT IS AN INTEGER AND NOTHING MORE. No buffered edits, no second document
  // model: every edit is already committed when the author sees it, and Cancel
  // is a compensating sequence over the engine's own byte snapshots.
  const tableEditorEdits = useRef(0)
  const [tableEditorEditCount, setTableEditorEditCount] = useState(0)
  // Written together, always, so the loop and the footer cannot disagree.
  const setTableEditorEdits = (next: number) => { tableEditorEdits.current = next; setTableEditorEditCount(next) }
  // What a completed Cancel discarded, stated in the design-mode announcement
  // region. Its own state rather than `fileStatus`: a discard is not a local
  // file outcome and overloading that line would make either message erase the
  // other.
  const [tableEditorDiscarded, setTableEditorDiscarded] = useState<string>()
  // WHETHER THE COMPENSATING SEQUENCE IS IN FLIGHT — its own indicator, and
  // deliberately NOT `tableEditorBusy`.
  //
  // The dialog's two ways out tear the session down (`revokeTableEditor`
  // advances `tableEditorSession`), so a click on `Done` or a press of Escape
  // mid-unwind makes the loop below return at its session guard BEFORE it
  // installs the snapshot it reached: the engine ends k undos back while
  // `snapshotRef`, the canvas and the preview still show the pre-Cancel
  // document, with nothing on screen saying so. Both are gated on THIS flag.
  //
  // ⚠ IT IS NOT `tableEditorBusy` BECAUSE `tableEditorBusy` CAN LATCH.
  // `setCurrentSnapshot`'s `clearDocumentInteraction` branch clears the dialog
  // without clearing that flag, so gating Escape on it could leave a modal that
  // cannot be closed at all — a worse defect than the one this guards. This flag
  // is cleared unconditionally in the loop's `finally` (which runs on the
  // teardown return too) and again in `openTableEditor` and
  // `revokeTableEditor`, so the dialog can never become inescapable.
  const [tableEditorDiscarding, setTableEditorDiscarding] = useState(false)
  // Local picker results are authority-scoped independently of React renders.
  // A document replacement revokes a picker started for the old document.
  const sampleLoadGeneration = useRef(0)
  const previewParamsRef = useRef(previewParams)
  const parameterReferenceRequest = useRef(0)
  const modeRef = useRef(mode)
  const canvasRegionRef = useRef<HTMLElement>(null)
  useEffect(() => { modeRef.current = mode }, [mode])
  useEffect(() => { selectedRef.current = selected }, [selected])
  const canvas = snapshot?.canvas
  // The break is selected only while the document still has one: an undo or a
  // replaced document takes the selection with it.
  const selectedBreak = sectionBreakSelected !== undefined && canvas ? sectionBreakOnPage(canvas, sectionBreakSelected) : undefined
  const breakPage = selectedBreak !== undefined ? sectionBreakSelected : undefined
  const breakSelected = breakPage !== undefined
  // A selected page exists only while the document still has it.
  const pageCount = canvas ? pageCountOf(canvas) : 1
  const pageSelection = selectedPage !== undefined && canvas && selectedPage < pageCount ? selectedPage : undefined
  // The current page (D-G.2), clamped to the pages the document still has; with
  // nothing selected and no page selected it is page 1. A selected section
  // break makes its own page current (story 5).
  const currentPage = pageSelection ?? breakPage ?? (selected.length === 0 ? 0 : Math.min(currentPageState, pageCount - 1))
  // D-5.1: Place Section Break follows the current page.
  const breakOnCurrentPage = canvas !== undefined && sectionBreakOnPage(canvas, currentPage) !== undefined
  // WHICH PAGE DELETE PAGE TARGETS (D-2.1): the selected page, else the one page
  // every selected CONTENT element is on. Otherwise the reason it is disabled.
  const deletePageTarget: Readonly<{ page: number } | { reason: string }> = (() => {
    if (!canvas) return { reason: 'no document is open' }
    if (pageCount === 1) return { reason: 'a document keeps at least one page' }
    if (pageSelection !== undefined) return { page: pageSelection }
    const pages = new Set(canvas.components.filter((component) => component.band === 'content' && selected.includes(component.id)).map(componentPage))
    if (pages.size === 1) return { page: [...pages][0]! }
    if (pages.size > 1) return { reason: 'the selection is on more than one page' }
    return { reason: 'select a page, or content on one page' }
  })()
  // STORY 8.4a — THE FACES THIS DOCUMENT CARRIES, REGISTERED ONCE FOR THE
  // WHOLE DOCUMENT.
  //
  // The engine measures and renders with a face the document carries in its
  // `assets` map, and now attributes each painted fragment to the asset it
  // resolved. The browser had no CSS family for such a face at all — no
  // `@font-face`, no name, no bytes — so the canvas rasterized at the engine's
  // x-positions with a fallback face's metrics and the glyphs collided. This
  // fetches those bytes over the SAME `asset` operation images already use and
  // registers each one under the family embedded-face-family.ts derives from
  // its key.
  //
  // IT IS DOCUMENT-SCOPED, WHICH IS THE ONE STRUCTURAL DECISION HERE.
  // ImagePaint is the nearest precedent and the wrong lifetime: it is mounted
  // once per component AND once per repeated sheet, so it makes an independent
  // request per mounted instance. `document.fonts` is a global, name-keyed
  // registry, so that shape would add one face many times under one family and
  // let an unmounting instance delete a face another is still painting with.
  //
  // THE LISTING IS PART OF THE KEY, AND `documentGenerationValue` ALONE IS NOT
  // ENOUGH. The generation advances only when the document is REPLACED (open a
  // file, new template, undo/redo); an ordinary font-chain command commits
  // through setCurrentSnapshot without it and can add or remove a carried
  // entry. The listing is over the carried keys only, sorted and de-duplicated
  // — precisely the input this effect reads — so a chain rename or a shipped
  // entry's edit does not needlessly re-register a face that is already good.
  //
  // THE KEY'S SHAPE IS ADMITTED HERE, NOT ASSUMED. `isCarriedFaceAssetKey` is
  // the derivation module's own predicate, and this is the one production
  // caller that hands it a key the FRAGMENT guard never saw: a fragment's
  // `assetKey` is admitted as 64 lowercase hex at the protocol boundary, a
  // chain ENTRY's is admitted on length alone. An entry whose key is not that
  // shape is a carried face the browser declines: no bytes are fetched, no
  // family is derived, nothing is registered, and every fragment keeps the
  // stylesheet's declared stack. It is a degrade, not a refusal — the
  // projection is already admitted and the session is untouched.
  //
  // ⚠ AND AN EMBEDDED ENTRY'S STYLE VARIANTS ARE ASSET KEYS TOO (Story 11.3,
  // AD-8). Reading only `entry.assetKey` was correct while an entry named one
  // face; since 11.2 an entry may name up to four, and for
  // `{"asset": K1, "bold": K2}` the ENGINE resolves a bold run to `K2` and puts
  // it on the fragment. If K2 is not fetched here, `carriedFaces.has(K2)` is
  // false, `fragment.face` is empty on that arm, the fragment gets NO
  // `fontFamily` at all and falls to the stylesheet's stack — a document whose
  // own bold face is right there in its `assets` map, drawn in something else.
  // That is the same shape as D-11.3.1: removing a compensation (the synthetic
  // `font-weight: 700` this story deletes) without supplying what it
  // compensated for.
  //
  // THE DISCRIMINANT DECIDES, NOT THE SHAPE. Variants are collected only from
  // entries that ARE embedded; a `face` entry's variants are FontSet face
  // names, and a 64-character face name is a legal face name, so filtering the
  // whole population by `isCarriedFaceAssetKey` would have crossed the two
  // namespaces on exactly the value that looks like it could not.
  const carriedFaceKeys = [...new Set((canvas?.fontChains ?? []).flatMap((chain) => chain.entries).flatMap((entry) => entry.assetKey.length > 0 ? [entry.assetKey, entry.bold, entry.italic, entry.boldItalic] : []).filter(isCarriedFaceAssetKey))].sort()
  const carriedFaceListing = carriedFaceKeys.join('\u0000')
  useEffect(() => {
    setCarriedFaces(NO_CARRIED_FACES)
    if (!engine || carriedFaceListing === '') return
    // A rejected request or a response with no bytes is a document fact, not a
    // session fault: registerCarriedFaces reports the key as unregistered, the
    // fragment keeps the stylesheet's declared stack, and nothing reaches the
    // engine's failure channel.
    return registerCarriedFaces(carriedFaceListing.split('\u0000'), async (assetKey) => (await engine.request('asset', assetBytesRequest(assetKey))).bytes, setCarriedFaces)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [engine, documentGenerationValue, carriedFaceListing])

  // STORY 16.2 — THE MACHINE STORE IS OPENED ONCE, FOR THE SESSION.
  //
  // THE LIFETIME IS THE THIRD ONE IN THIS FILE AND IT IS ARGUED, NOT ASSUMED,
  // because the effect above is the precedent for exactly that obligation.
  // `registerCarriedFaces` is DOCUMENT-scoped: it is re-run when the document
  // is replaced and when the set of carried entries changes, and its release
  // removes what it added. This one is MACHINE-scoped — strictly, the browser
  // profile's origin — and is deliberately NOT keyed on
  // `documentGenerationValue`: the whole property of the store is that it
  // survives the document, so re-opening it per document would be re-asking a
  // question whose answer cannot have changed, and clearing it per document
  // would delete the feature.
  //
  // AN OPEN FAILURE SAYS NOTHING ON SCREEN (Story 16.6, reversing 16.2's
  // stated-degradation clause by owner decision). A browser with storage
  // blocked simply keeps `storedFaces` empty; the designer works and picks
  // still embed straight into the document.
  useEffect(() => {
    let live = true
    const opening = openFontStore().then((opened) => {
      if (live) setStoreKeepsFaces(opened.ok)
      if (opened.ok) return opened.value
      return undefined
    })
    fontStore.current = opening
    void (async () => {
      const store = await opening
      if (!live || !store) return
      const listed = await store.list()
      if (!live || !listed.ok) return
      setStoredFaces(listed.value)
    })()
    return () => { live = false }
  }, [])

  // AND THE FACES THIS MACHINE HOLDS ARE REGISTERED FOR PREVIEW, ALONGSIDE THE
  // DOCUMENT'S OWN, WITHOUT DISTURBING THE EFFECT ABOVE.
  //
  // Two separate registrations rather than one merged list, and the separation
  // is the point: the document effect's key is `documentGenerationValue` plus
  // the carried listing, and folding a machine-scoped input into it would make
  // a store write re-register every carried face in the open document. They
  // meet only at the canvas, in the union below.
  //
  // THE FAMILY NAME IS THE SAME ON BOTH SIDES BECAUSE THE KEY IS. A stored face
  // and a carried face of the same bytes share a content address, so they share
  // the family `embedded-face-family.ts` derives, and `document.fonts` — a
  // global, name-keyed registry — simply holds one name over two identical
  // faces. That is a duplicate, not a conflict: either release removes only the
  // `FontFace` object it added, and the surviving one draws the same glyphs.
  //
  // THE COST, STATED: this reads every stored face's bytes once per session.
  // The store grows by one face per pick of a new family, so that is a handful
  // of megabytes for an author who has picked a handful of families, and it
  // buys a preview that does not wait for a network the store exists to avoid.
  const machineFaceListing = storedFaces.map((face) => face.key).sort().join(' ')
  useEffect(() => {
    setMachineFaces(NO_CARRIED_FACES)
    if (machineFaceListing === '') return
    return registerCarriedFaces(machineFaceListing.split(' '), async (key) => {
      const read = await (await fontStore.current)?.get(key)
      return read?.ok ? read.value?.bytes : undefined
    }, setMachineFaces)
  }, [machineFaceListing])

  // The canvas asks one question — "is there a face registered under this asset
  // key" — and both registrations can answer it.
  const paintableFaces = useMemo(() => machineFaces.size === 0 ? carriedFaces : new Set([...carriedFaces, ...machineFaces]), [carriedFaces, machineFaces])

  // STORY 14.4 / P1. ONE LOOKUP FOR THE ONE SELECTED COMPONENT, and everything
  // the data panel is told about it derives from THIS object rather than from
  // its own repeat scan of the projection. It used to be scanned once for the
  // binding and — when the kind gate was added — a second time for the type,
  // which is how the two could disagree about whether the component exists at
  // all: `selectedComponentId` was passed unconditionally from `selected`,
  // while the type came from a `find` that returns undefined for an absent
  // canvas or an id no longer in the projection. That id-present/type-absent
  // state is exactly what the panel's fifth arm must fail CLOSED on.
  //
  // The id still comes from `selected`, not from this lookup, and deliberately:
  // "one component is selected" is true whether or not the projection currently
  // carries it, and demoting it to "select one component first" would state
  // something false. The KIND is what becomes unknown, and the panel says so.
  const selectedComponent = selected.length === 1 ? canvas?.components.find((component) => component.id === selected[0]) : undefined

  // STORY 14.10 — THE COLUMN SELECTION, RE-RESOLVED FROM THE PROJECTION ON
  // EVERY RENDER, AND NEVER CACHED.
  //
  // The state holds two ids and nothing else. If either stops resolving — the
  // column removed in the editor and `commitTableColumn` re-projected, the
  // table deleted, the document replaced — this reads `undefined` and every
  // consumer of it goes with it: the identity strip, the cyan mark, the DATA
  // panel's column mode. The panel can therefore never offer a column that does
  // not exist, without anyone remembering to clear anything.
  //
  // ⚠ IT ALSO NEEDS `selected` TO STILL BE THAT ONE TABLE. The column mode
  // borrows the single-component selection's own gate rather than inventing a
  // second one, so a multi-selection or a different component leaves no column
  // selected and every `length === 1` gate keeps its current meaning.
  // ONE WALK OF THE SAMPLE, TWO CONSUMERS (Q2(a)). The table editor takes
  // `.candidates` — byte-for-byte what it has always received — and the DATA
  // panel takes the node map built from the same admitted set. Two calls would
  // be two sets that merely look alike.
  const sampleCandidateScan = useMemo(() => tableSampleCandidates(sampleData?.tree), [sampleData])
  const selectedTableColumn = (() => {
    if (!columnSelection || !canvas) return undefined
    if (selected.length !== 1 || selected[0] !== columnSelection.tableId) return undefined
    const table = canvas.components.find((component) => component.id === columnSelection.tableId)
    const column = table?.columns?.find((entry) => entry.id === columnSelection.columnId)
    return table && column ? { tableId: table.id, columnId: column.id, label: column.label, collection: table.tableBind ?? '' } : undefined
  })()
  // WHAT THE DATA PANEL IS GIVEN FOR THAT COLUMN: the two ids, the label the
  // canvas and the editor already show, the collection the table is bound to,
  // and the row fields of THAT collection — filtered out of the one walk above
  // rather than re-derived. An unnamed column falls back to its id so the bar
  // and the strip still have something to name it by.
  const columnBindScope = selectedTableColumn === undefined ? undefined : {
    tableId: selectedTableColumn.tableId,
    columnId: selectedTableColumn.columnId,
    label: selectedTableColumn.label === '' ? selectedTableColumn.columnId : selectedTableColumn.label,
    collection: selectedTableColumn.collection,
    rowFields: new Map([...sampleCandidateScan.byNode].filter(([, candidate]) => candidate.collection === selectedTableColumn.collection && selectedTableColumn.collection !== '').map(([node, candidate]) => [node, candidate.field] as const)),
  }

  // The listing is re-read from the store rather than patched in memory, so the
  // store stays the single authority on what this machine holds. A refresh that
  // fails leaves the previous listing standing, silently — a stale listing
  // still picks correctly, because every pick re-reads the bytes.
  const refreshStoredFaces = async () => {
    const store = await fontStore.current
    if (!store) return
    const listed = await store.list()
    if (listed.ok) setStoredFaces(listed.value)
  }

  // STORY 16.3 — WHAT THE FONT BROWSER OFFERS, AND WHAT ITS SPECIMENS ARE SET IN.
  //
  // THE OFFERED LIST IS `offeredFamilies` WITH AN EMPTY QUERY, WHICH IS THE SAME
  // FUNCTION THE FAMILY CONTROL ASKS. There is one answer to "which families may
  // this author add", and a browser that computed its own would be a second one
  // — with its own opinion about variable-only families and its own tier order.
  // The browser filters and sorts what it is given; it never decides what it is
  // given.
  const browsableFamilies = useMemo(() => offeredFamilies('', storedFaces), [storedFaces])

  // AND THE BYTES A SPECIMEN IS SET IN COME FROM THE SAME THREE TIERS A PICK
  // RESOLVES FROM, THROUGH THE SAME THREE READS.
  //
  // NOTHING BUT BYTES TRAVELS. No licence record is kept, nothing is written to
  // the machine store and no command is sent: a preview is a face for one
  // `<span>` in one modal, and `preview-face-registry.ts` releases it when the
  // row leaves the page.
  //
  // THE STORE IS READ AND NOT WRITTEN, AND THAT ASYMMETRY IS THE POINT. THE
  // STORE HOLDS FACES THE AUTHOR CHOSE, NOT FACES THEY SCROLLED PAST. Reading it
  // is free and makes a family this machine already holds cost no network at
  // all; writing it would fill a store that carries a slot and byte budget
  // (Story 16.2) with every family that happened to cross the viewport, so the
  // budget would be spent by browsing rather than by deciding. A face earns its
  // place on this machine by being picked.
  //
  // THE WEB TIER GOES THROUGH `fetchWebFamily` — THE FULL RESOLUTION, LICENCE
  // CLASSIFICATION AND ALL — AND THAT IS NOT AN OVERSIGHT. A cheaper
  // bytes-only fetch would be a SECOND fetch path in a designer whose whole
  // host discipline rests on there being one, and it would let the browser set
  // a specimen in a face the pick would then refuse on its terms: `+ Add`
  // promising something the product declines. Reusing the pick's own resolution
  // means a specimen appears exactly for the families that can actually be
  // added. The cost is the pick's cost, bounded by `familiesPerPage`.
  const browserSpecimenBytes = async (family: string): Promise<ArrayBuffer | undefined> => {
    const source = browsableFamilies.find((entry) => entry.family === family)
    if (source === undefined) return undefined
    if (source.tier === 'local') {
      try {
        const response = await fetch(source.face.url)
        return response.ok ? await response.arrayBuffer() : undefined
      } catch {
        return undefined
      }
    }
    if (source.tier === 'stored') {
      const read = await (await fontStore.current)?.get(source.record.key)
      return read?.ok ? read.value?.bytes : undefined
    }
    const outcome = await fetchWebFamily(source.family)
    return outcome.ok ? outcome.face.bytes : undefined
  }

  // THE FAMILY CONTROL'S OWN READER (Story 16.7) — deliberately a SEPARATE
  // function from `browserSpecimenBytes` above, so that this story's own
  // divergence never has to edit the browser's (Ask First on that one is a
  // boundary of this story, not an invitation to fork a shared branch inside
  // it).
  //
  // LOCAL AND STORED READ EXACTLY AS THE BROWSER'S DO — no network for a face
  // this machine already holds, a store read for one it fetched before. `web`
  // IS WHERE THIS READER DIVERGES, ON PURPOSE: fetching to draw a specimen for
  // a family not on this machine is the thing Design Note (1) refuses — a
  // pick already blocks up to 30s on a stall and 180s against a slow host,
  // and a MENU must never cost that. So a `web` row resolves to `undefined`
  // with NO CALL TO `fetchWebFamily` AT ALL. STORY 16.9 removed the
  // dropdown's own web-tier group entirely, so this branch is now reachable
  // only through `familyControlSpecimenBytes`'s general contract (any
  // `family` string), never through a row this control renders.
  const familyControlSpecimenBytes = async (family: string): Promise<ArrayBuffer | undefined> => {
    const source = browsableFamilies.find((entry) => entry.family === family)
    if (source === undefined || source.tier === 'web') return undefined
    if (source.tier === 'local') {
      try {
        const response = await fetch(source.face.url)
        return response.ok ? await response.arrayBuffer() : undefined
      } catch {
        return undefined
      }
    }
    const read = await (await fontStore.current)?.get(source.record.key)
    return read?.ok ? read.value?.bytes : undefined
  }

  const installPreview = (next: PreviewRecord | undefined) => { previewRef.current = next; setPreview(next) }
  const cancelPreviewWork = () => {
    previewToken.current++
    previewAbort.current?.abort()
    previewAbort.current = undefined
    if (previewTimer.current !== undefined) clearTimeout(previewTimer.current)
    previewTimer.current = undefined
    previewScheduler.current.clear()
    retryingFailure.current = undefined
  }
  const invalidatePreview = (clear = false) => {
    // STORY 14.7b — AND THE DISCARD SENTENCE IS WITHDRAWN HERE, because this is
    // the one site every committed change already passes through.
    //
    // The sentence promises "Redo restores them until your next committed edit",
    // and Go's next `install` sets `e.redo = nil` — so the moment the document
    // moves again the promise is FALSE, and a live region still asserting it is
    // worse than silence. Clearing it only where the edit COUNT is cleared is not
    // enough: a nudge on the still-selected table commits without touching the
    // count at all. Every revision-moving commit, every document replacement and
    // every undo/redo calls this function; a completed Cancel calls it too, and
    // then states its sentence AFTER — which is why the set has to follow the
    // close rather than precede it.
    setTableEditorDiscarded(undefined)
    // A request already admitted to the FIFO worker must drain; invalidation
    // revokes its authority synchronously and lets the scheduler coalesce a
    // single newest replacement behind it instead of posting duplicates.
    previewToken.current++
    previewGeneration.current++
    if (clear) {
      installPreview(undefined)
      setPreviewViewState(initialPDFPreviewViewState)
      setPreviewPages(undefined)
      setPreviewStatus('idle')
    } else {
      setPreviewStatus(previewRef.current ? 'stale' : 'idle')
      setStaleReason('inputs-changed')
    }
    setPreviewError(undefined)
    setPreviewIssue(undefined)
    retryingFailure.current = undefined
    setDismissedDiagnostics(new Set())
  }
  // STORY 13.4 — NO SAMPLE-DATA GATE. Laying out a page is not gated on
  // inventing data the author does not have: with no sample loaded, runPreview
  // asks the engine for a stand-in document and renders that. The engine never
  // required data either — the CLI passes Data("{}") when -data is omitted.
  const renderPreview = (force = false) => {
    if (previewTimer.current !== undefined) clearTimeout(previewTimer.current)
    previewTimer.current = undefined
    previewScheduler.current.submit(() => runPreview(force))
  }
  const runPreview = async (force = false) => {
    const sample = sampleDataRef.current
    if (!engine || !snapshotRef.current) return
    const generation = previewGeneration.current
    const documentAtStart = documentGeneration.current
    const revisionAtStart = snapshotRef.current.revision
    const params = new TextEncoder().encode(previewParamsRef.current).buffer
    const token = ++previewToken.current
    const controller = new AbortController()
    previewAbort.current = controller
    const current = (identity?: string) => token === previewToken.current && !controller.signal.aborted && modeRef.current === 'preview' && previewGeneration.current === generation && documentGeneration.current === documentAtStart && snapshotRef.current?.revision === revisionAtStart && (!identity || canInstallPreview({ token, generation, revision: revisionAtStart, identity }, { token: previewToken.current, generation: previewGeneration.current, revision: snapshotRef.current?.revision ?? -1, identity, mode: modeRef.current }))
    const mustRender = force || previewNeedsFreshRender.current
    setPreviewStatus(previewRef.current ? 'stale' : 'checking'); setPreviewError(undefined); setPreviewIssue(undefined)
    try {
      // THE DATA CHANNEL, AND ONLY ITS CONTENTS DIFFER. With a sample loaded
      // the engine receives the accepted file bytes, not the local inspection
      // projection; with none, it receives the stand-in document IT generated
      // for this template. ArrayBuffer slicing is a transport copy, never a
      // rewrite, in both arms.
      //
      // The projection is admitted under the same request/generation/revision
      // agreement loadParameterReferences uses: `current()` re-checks the
      // request token, the abort signal, the mode, the preview generation, the
      // document generation and the LIVE snapshot's revision against the one
      // captured at entry, and the response's OWN snapshot revision is compared
      // against that same captured value on the line below. An unavailable
      // projection is NEVER turned into a guessed empty document: the render
      // simply does not happen.
      let data: ArrayBuffer
      if (sample) data = sample.bytes.slice(0)
      else {
        const projected = await engine.request('stand-in-data', undefined, controller.signal)
        if (!projected.bytes || projected.snapshot.revision !== revisionAtStart || !current()) return
        data = projected.bytes.slice(0)
      }
      const checked = await engine.request('identity', { data, params }, controller.signal)
      const identity = checked.preview?.identity
      if (!identity || checked.preview.revision !== revisionAtStart || !current(identity)) return
      if (!mustRender && previewStatus === 'current' && previewRef.current && previewRef.current.identity === identity && previewRef.current.revision === revisionAtStart && previewRef.current.generation === generation) {
        setPreviewStatus('current')
        return
      }
      const canonical = await engine.request('serialize', undefined, controller.signal)
      if (!canonical.bytes) throw new Error('Current canonical document is unavailable')
      const revision = canonical.snapshot.revision
      if (revision !== revisionAtStart || !current(identity)) return
      setPreviewStatus(previewRef.current ? 'stale' : 'rendering')
      const result = await engine.request('render', { template: canonical.bytes, data, params }, controller.signal)
      if (!result.bytes || !result.preview?.pdfSha256 || !result.preview.diagnostics || result.preview.elapsedMs === undefined || result.preview.version === undefined || result.preview.identity !== identity || result.preview.revision !== revision || !current(identity)) return
      // STORY 13.3 / DW-270 — THE BROWSER HASHES THE BYTES IT IS HOLDING.
      //
      // Until here the digest had been admitted by SHAPE alone. The rail
      // promotes it to a bordered block a person is told to compare against a
      // producer's, and asking someone to verify a claim we have not verified
      // ourselves is worse than not showing it. The bytes crossed four
      // value-preserving copies to get here; this is the first thing that
      // checks that they still are the bytes the digest names.
      //
      // ⚠ `crypto.subtle.digest` IS A NEW SUSPENSION POINT, so `current` is
      // re-checked after it exactly as after every other await in this
      // function: an install decided before the await and performed after it
      // would install a preview the author has already left behind.
      const recomputed = await pdfDigest(result.bytes)
      if (!current(identity)) return
      if (recomputed !== result.preview.pdfSha256) {
        // NOT INSTALLED AND NOT DISPLAYED. The refusal is routed through
        // `previewIssue`/`previewStatus`, which is the sentence the freshness
        // line already reads, rather than a second alert shape of its own.
        setPreviewIssue(`the rendered PDF does not match the digest the engine reported for it (engine ${result.preview.pdfSha256.slice(0, 16)}…, bytes ${recomputed.slice(0, 16)}…)`)
        setPreviewStatus('error')
        return
      }
      installPreview({ bytes: result.bytes.slice(0), revision, identity, digest: result.preview.pdfSha256, diagnostics: result.preview.diagnostics, token, generation, standIn: !sample, elapsedMs: result.preview.elapsedMs, version: result.preview.version, installedAt: Date.now() })
      previewNeedsFreshRender.current = false
      setDismissedDiagnostics(new Set())
      // STORY 13.2 — THE VIEW STATE IS NOT RESET HERE ANY MORE, AND THAT IS THE
      // WHOLE OF "leaving Preview and coming back keeps your place". Leaving
      // Preview marks the render stale, so `runPreview`'s nothing-changed early
      // return cannot fire and execution always reached this line: every return
      // to Preview threw the author's page, zoom, scroll and fit away. The reset
      // inside `invalidatePreview(clear)` stays — there the preview really is
      // gone — and a document that got SHORTER is handled where it always was,
      // by the viewer's own `safePage` clamp.
      // PDF.js is a separate boundary. The bytes become current only when its
      // matching document has admitted successfully through onPageCount. The
      // candidate remains marked stale while it is visible but unconfirmed.
      setStaleReason('inputs-changed')
      setPreviewStatus('stale')
    } catch (error) {
      if (token !== previewToken.current || controller.signal.aborted) return
      if (!current()) return
      if (isProducerRenderFailure(error)) {
        setStaleReason('render-failed')
        setPreviewStatus(previewRef.current ? 'stale' : 'error'); setPreviewError({ error: previewFailure(error), token, generation, revision: revisionAtStart })
      } else {
        // This failed before/around the closed producer response boundary. It
        // remains a local Preview issue rather than invented render provenance.
        setStaleReason('inputs-changed')
        setPreviewStatus(previewRef.current ? 'stale' : 'error')
        setPreviewIssue(localPreviewIssue(error))
      }
    } finally {
      if (previewAbort.current === controller) previewAbort.current = undefined
    }
  }
  const schedulePreview = () => {
    if (modeRef.current !== 'preview') return
    if (previewTimer.current !== undefined) clearTimeout(previewTimer.current)
    setPreviewStatus(previewRef.current ? 'stale' : 'debouncing')
    previewTimer.current = setTimeout(() => { previewTimer.current = undefined; renderPreview() }, PREVIEW_DEBOUNCE_MS)
  }
  const acceptPreviewParameters = (draftValue: string) => {
    setPreviewParamsDraft(draftValue)
    try {
      const parsed: unknown = JSON.parse(draftValue)
      if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('Parameter input must be a JSON object')
      if (new TextEncoder().encode(draftValue).byteLength > MAX_PARAMETER_DOCUMENT_BYTES) throw new Error('Parameter input exceeds the local engine limit')
      previewParamsRef.current = draftValue
      setPreviewParams(draftValue)
      setPreviewParamsError(undefined)
      invalidatePreview()
      schedulePreview()
    } catch (error) {
      setPreviewParamsError(error instanceof Error ? error.message : 'Parameter input must be valid JSON')
    }
  }
  const setNamedParameter = (name: string, value: string) => {
    try {
      const document: unknown = JSON.parse(previewParamsRef.current)
      JSON.parse(value)
      if (document === null || Array.isArray(document) || typeof document !== 'object') throw new Error('Parameter input must be a JSON object')
      // Never stringify the accepted document here. Its numeric lexemes and
      // untouched source bytes are runtime input evidence, not UI state to
      // normalize. The small JSON token locator only replaces this one value.
      const next = replaceTopLevelJSONValue(previewParamsRef.current, name, value)
      if (next === undefined) throw new Error('Parameter input must be a JSON object')
      acceptPreviewParameters(next)
    } catch {
      setPreviewParamsError(`Value for params.${name} must be valid JSON`)
    }
  }
  const clearPreviewParameters = (clearReferences = false) => {
    previewParamsRef.current = EMPTY_PARAMETER_DOCUMENT
    setPreviewParams(EMPTY_PARAMETER_DOCUMENT)
    setPreviewParamsDraft(EMPTY_PARAMETER_DOCUMENT)
    setPreviewParamsError(undefined)
    if (clearReferences) { parameterReferenceRequest.current++; setParameterReferenceState({ status: 'pending', names: [] }) }
  }
  // Returns whether a READY list was installed, so the design-mode caller can
  // re-arm itself instead of latching a transient failure for the document's life.
  const loadParameterReferences = async (): Promise<boolean> => {
    const currentSnapshot = snapshotRef.current
    const generation = documentGeneration.current
    const request = ++parameterReferenceRequest.current
    if (!engine || !currentSnapshot) {
      setParameterReferenceState({ status: 'failed', names: [] })
      return false
    }
    setParameterReferenceState({ status: 'pending', names: [] })
    try {
      const result = await engine.request('parameter-references')
      if (parameterReferenceRequest.current === request && documentGeneration.current === generation) {
        if (snapshotRef.current?.revision === currentSnapshot.revision && result.snapshot.revision === currentSnapshot.revision && result.parameterReferences?.revision === currentSnapshot.revision) { setParameterReferenceState({ status: 'ready', names: result.parameterReferences.names }); return true }
        setParameterReferenceState({ status: 'failed', names: [] })
      }
      return false
    } catch {
      // Never turn an unavailable projection into a guessed empty one.
      if (parameterReferenceRequest.current === request && documentGeneration.current === generation) setParameterReferenceState({ status: 'failed', names: [] })
      return false
    }
  }
  // STORY 14.6 / AC6 — THE ENGINE'S `params` NAMESPACE IS VISIBLE IN DESIGN.
  //
  // Every other `parameter-references` fetch on this component is gated on
  // `modeRef.current === 'preview'`, so before this the namespace could not
  // appear in the DATA tab at all: an author had to enter Preview to learn it
  // existed, which is precisely what AC6 exists to prevent. This is the one
  // design-mode fetch the story authorises, and it is LAZY and IDEMPOTENT —
  // armed by opening the DATA tab, fired at most once per document generation,
  // never per keystroke, per selection change or per re-render. A new document
  // bumps the generation, so the next DATA-tab render asks again rather than
  // showing the previous template's parameters.
  const designReferenceGeneration = useRef(-1)
  useEffect(() => {
    if (inspectorTab !== 'data' || modeRef.current !== 'design') return
    if (designReferenceGeneration.current === documentGeneration.current) return
    designReferenceGeneration.current = documentGeneration.current
    // ⚠ RE-ARM ON FAILURE, OR A TRANSIENT ONE STICKS FOR THE DOCUMENT'S LIFE.
    // The guard is claimed BEFORE the await so two renders cannot race a second
    // request; releasing it when no ready list arrived leaves the next DATA-tab
    // or mode change free to try again. It does not retry on its own, so this
    // stays one request per arming rather than a loop.
    void loadParameterReferences().then((ready) => { if (!ready) designReferenceGeneration.current = -1 })
    // `loadParameterReferences` is re-created every render by design; keying the
    // effect on it would defeat the idempotence the guard above provides.
    // `mode` IS a dependency: the body reads `modeRef.current`, so returning
    // from Preview with the DATA tab already open must re-evaluate this.
  }, [inspectorTab, documentGenerationValue, mode]) // eslint-disable-line react-hooks/exhaustive-deps
  const enterPreview = () => {
    modeRef.current = 'preview'
    setMode('preview')
    void loadParameterReferences()
    renderPreview()
  }
  const returnToDesign = () => { cancelPreviewWork(); modeRef.current = 'design'; setPreviewStatus(previewRef.current ? 'stale' : 'idle'); setPreviewError(undefined); setMode('design') }
  const viewerError = useCallback((token: number, error: Error) => {
    // Invalidate before unmounting the failed viewer: an older PDF.js callback
    // must never turn a newer render into an error state.
    if (token !== previewToken.current || modeRef.current !== 'preview') return
    cancelPreviewWork()
    const current = previewRef.current
    if (!current) return
    previewNeedsFreshRender.current = true
    setPreviewError(undefined)
    setPreviewIssue(`The local PDF viewer could not display the admitted PDF: ${error.message.slice(0, 160)}`)
    setStaleReason('inputs-changed')
    setPreview((current) => { previewRef.current = current; setPreviewStatus(current ? 'stale' : 'error'); return current })
  }, [])
  const viewerPages = useCallback((token: number, pages: number) => {
    const current = previewRef.current
    if (pages > 0 && current && current.token === token && token === previewToken.current && modeRef.current === 'preview' && canInstallPreview({ token, generation: current.generation, revision: current.revision, identity: current.identity }, { token: previewToken.current, generation: previewGeneration.current, revision: snapshotRef.current?.revision ?? -1, identity: current.identity, mode: modeRef.current })) { previewNeedsFreshRender.current = false; setPreviewIssue(undefined); setPreviewPages(pages); setPreviewStatus('current') }
  }, [])
  const changePreviewViewState = useCallback((next: PDFPreviewViewState) => setPreviewViewState((current) => samePDFPreviewViewState(current, next) ? current : next), [])
  const clearInteraction = () => { canvasSelection.cancel(); setPlacing(undefined); setPlacingAt(undefined); setHoverBand(undefined); setDrag(undefined); abortBoundaryDrag(); abortSectionBreakDrag() }
  // STORY 14.3 — THE COMMIT REPORTS WHICH COMPONENT IT MADE, AND IT REPORTS IT
  // BY DIFF.
  //
  // The protocol carries no created-id: an `EngineResult` for a `command`
  // holds a snapshot and nothing else (engine-client.ts). So the id has to be
  // DERIVED, and the derivation is a SET DIFFERENCE across the await — never
  // "the last component in the band". Where Go appends is a Go implementation
  // detail that this side has no standing to depend on; that exactly one
  // component id appears which was not there before is a property of the
  // create commands themselves.
  //
  // It answers `undefined` for every other commit — a move, a resize, a
  // delete, a refusal, or an accepted command whose snapshot added no id —
  // because a caller that selects "whatever came back" must have NOTHING to
  // select in those cases. Selecting a stale id is the failure this shape
  // forecloses, and the I/O matrix names it as its own row.
  const commitComponent = async (payload: ArrayBuffer, after?: (added: ReadonlyArray<string>) => void) => {
    if (!engine || fileBusy) return
    setCommitError(undefined)
    const generation = documentGeneration.current
    try {
      const priorRevision = snapshotRef.current?.revision
      const priorIds = new Set((snapshotRef.current?.canvas?.components ?? []).map((component) => component.id))
      const result = await engine.request('command', payload)
      if (documentGeneration.current !== generation) return
      if (result.snapshot.revision !== priorRevision) invalidatePreview()
      if ((snapshotRef.current?.revision ?? -1) <= result.snapshot.revision) setCurrentSnapshot(result.snapshot)
      // Every id the snapshot added, in projection order. A paste selects the
      // whole list; the single-id return below stays the create commands' door.
      const added = (result.snapshot.canvas?.components ?? []).filter((component) => !priorIds.has(component.id)).map((component) => component.id)
      after?.(added)
      return added.length === 1 ? added[0]! : undefined
    }
    catch (error) { if (documentGeneration.current === generation) { setCommitError(componentDiagnostic(error)); clearInteraction() } }
  }
  const installSelection = (ids: ReadonlyArray<string>) => {
    setBindingError(undefined); setPropertyError(undefined); setCommitError(undefined); revokeTableEditor(); setColumnSelection(undefined); setSectionBreakSelected(undefined); setSelectedPage(undefined)
    selectedRef.current = ids; setSelected(ids)
    const components = snapshotRef.current?.canvas?.components ?? []
    const band = components.find((component) => component.id === ids.at(-1))?.band
    if (band) focusBandRef.current = band
    // Selecting content makes its page current: kept when the selection still
    // has content on the current page, otherwise the page of the last selected
    // content element. A header- or footer-only selection keeps the current
    // page, and nothing selected is page 1.
    if (ids.length === 0) { setCurrentPage(0); return }
    const content = ids.map((id) => components.find((component) => component.id === id)).filter((component) => component?.band === 'content') as CanvasProjection['components'][number][]
    if (content.length > 0 && !content.some((component) => componentPage(component) === currentPageRef.current)) setCurrentPage(componentPage(content.at(-1)!))
  }
  const canvasSelection = useCanvasSelection({ engine, canvas, revision: snapshot?.revision ?? 0, generation: documentGenerationValue, zoom, selection: selected, enabled: mode === 'design' && !placing && !fileBusy, snap: snapEnabled, documentDelta: canvasDisplay.documentDelta,
    capture: (id) => canvasRegionRef.current?.setPointerCapture?.(id),
    release: (id) => { const host = canvasRegionRef.current; if (host?.hasPointerCapture?.(id)) host.releasePointerCapture(id) },
    onSelection: installSelection, onCommit: commitComponent, onError: (error) => setCommitError(componentDiagnostic(error)),
  })
  const beginRectangle = (event: PointerEvent, band?: CanvasProjection['bands'][number], pageIndex = 0, gutter = false) => {
    if (!canvas || placing || event.button !== 0 || event.target !== event.currentTarget) return
    event.preventDefault(); event.stopPropagation()
    const point = placementPoint(event.nativeEvent, band ?? { name: 'content', x: 0, y: 0, width: canvas.width, height: canvas.height }, zoom)
    canvasSelection.beginRectangle(event, { x: point.x * 1000 - (gutter ? canvasDisplay.documentDelta(CANVAS_GUTTER, zoom) * 1000 : 0), y: point.y * 1000 + pageIndex * sheetPitch(canvas, zoom) }, event.shiftKey, band !== undefined || event.currentTarget.classList.contains('page-surface'))
  }
  // `grabStackY` is where a content component was pressed, down the whole
  // stack: story 3 uses it to find the page under the pointer while dragging.
  const beginSelectedGroup = (id: string, event: PointerEvent, grabStackY?: number) => {
    if (placing || event.button !== 0) return true
    if (event.shiftKey) { select(id, true, event.target); return true }
    event.preventDefault(); event.stopPropagation()
    const focusTarget = event.currentTarget instanceof HTMLElement && event.currentTarget.tabIndex >= 0 ? event.currentTarget : canvasRegionRef.current
    focusTarget?.focus({ preventScroll: true })
    const ids = selectedRef.current.includes(id) ? selectedRef.current : [id]
    if (!selectedRef.current.includes(id)) select(id, false, event.target)
    else if (ids.length === 1) select(id, false, event.target)
    setBindingError(undefined); setPropertyError(undefined); revokeTableEditor()
    if (ids.length > 1) setColumnSelection(undefined)
    canvasSelection.beginGroup(event, ids, id, grabStackY)
    return true
  }
  const openTableEditor = async (id: string) => {
    if (!engine || fileBusy || selectedRef.current.length !== 1 || selectedRef.current[0] !== id) return
    const generation = documentGeneration.current
    const revision = snapshotRef.current?.revision
		const session = ++tableEditorSession.current
		// ZEROED AT EVERY SITE THAT ADVANCES THE SESSION (there are exactly three:
		// here, revokeTableEditor, and setCurrentSnapshot's clearDocumentInteraction
		// branch). A count that outlived its session would let Cancel unwind edits
		// made before this dialog was ever opened.
		setTableEditorEdits(0); setTableEditorDiscarded(undefined); setTableEditorDiscarding(false)
		tableEditorInvoker.current = document.activeElement instanceof HTMLElement ? document.activeElement : undefined
    setTableEditorError(undefined); setTableEditorBusy(true)
    try {
      const result = await engine.request('table-columns', new TextEncoder().encode(JSON.stringify({ id })).buffer)
      if (tableEditorSession.current === session && documentGeneration.current === generation && selectedRef.current.length === 1 && selectedRef.current[0] === id && snapshotRef.current?.revision === revision && result.snapshot.revision === revision && result.tableColumns?.revision === revision && result.tableColumns.table.tableId === id) setTableEditor(result.tableColumns)
    } catch (error) { if (tableEditorSession.current === session && documentGeneration.current === generation) setTableEditorError(componentDiagnostic(error))
    } finally { if (tableEditorSession.current === session && documentGeneration.current === generation) setTableEditorBusy(false) }
  }
	const revokeTableEditor = () => {
		tableEditorSession.current++
		// THE DISCARD SENTENCE IS WITHDRAWN WHEREVER THE COUNT IS, and for the same
		// reason: it describes a session, and this is a site that ends one. A
		// sentence promising "Redo restores them" that outlives what it describes
		// is a claim the product can no longer keep.
		//
		// ⚠ SO `cancelTableEditor` SETS THAT SENTENCE **AFTER** `closeTableEditor`,
		// not before — this line would otherwise wipe it on the way out.
		setTableEditorEdits(0); setTableEditorDiscarded(undefined); setTableEditorDiscarding(false)
		setTableEditor(undefined); setTableEditorError(undefined); setTableEditorBusy(false)
	}
	const closeTableEditor = () => {
		revokeTableEditor()
		queueMicrotask(() => {
			const invoker = tableEditorInvoker.current
			if (invoker?.isConnected) invoker.focus()
			else canvasRegionRef.current?.focus()
		})
	}
  const commitTableColumn = async (payload: ArrayBuffer): Promise<boolean> => {
    const current = tableEditor
    if (!engine || fileBusy || !current || tableEditorBusy) return false
    const generation = documentGeneration.current
    const revision = snapshotRef.current?.revision
    const id = current.table.tableId
		const session = tableEditorSession.current
    let accepted = false
    setTableEditorError(undefined); setTableEditorBusy(true)
    try {
      const committed = await engine.request('command', payload)
			accepted = true
			// A committed canonical document is not scoped to transient selection or
			// editor visibility. Admit it whenever it follows the expected document
			// generation/revision; only the editor's re-projection remains scoped.
			if (documentGeneration.current === generation && snapshotRef.current?.revision === revision) {
				// STORY 14.7b — GUARDRAIL 1 SITS ON THIS EXISTING COMPARISON, and it is
				// the same question `invalidatePreview` already asks: did the engine's
				// revision actually move? A `clear` on an already-unset colour is a
				// LEGAL command that leaves canonical bytes unchanged, so Go returns
				// before pushUndo and the revision stands still — no history entry, so
				// no Cancel step. Counting it would make Cancel overshoot by one and
				// undo an edit from before the dialog opened.
				//
				// SCOPED TO THE SESSION CAPTURED AT ENTRY, so a commit whose dialog was
				// revoked mid-flight cannot add to whatever session came after it.
				if (committed.snapshot.revision !== revision) { invalidatePreview(); if (tableEditorSession.current === session) setTableEditorEdits(tableEditorEdits.current + 1) }
				setCurrentSnapshot(committed.snapshot)
			}
      const projected = await engine.request('table-columns', new TextEncoder().encode(JSON.stringify({ id })).buffer)
      if (tableEditorSession.current === session && documentGeneration.current === generation && selectedRef.current.length === 1 && selectedRef.current[0] === id && snapshotRef.current?.revision === committed.snapshot.revision && projected.snapshot.revision === committed.snapshot.revision && projected.tableColumns?.revision === committed.snapshot.revision && projected.tableColumns.table.tableId === id) { setTableEditor(projected.tableColumns); return true }
			else if (tableEditorSession.current === session) revokeTableEditor()
    } catch (error) { if (accepted) { if (tableEditorSession.current === session) revokeTableEditor() } else if (tableEditorSession.current === session && documentGeneration.current === generation && selectedRef.current.length === 1 && selectedRef.current[0] === id && snapshotRef.current?.revision === revision) setTableEditorError(componentDiagnostic(error))
    } finally { if (tableEditorSession.current === session && documentGeneration.current === generation) setTableEditorBusy(false) }
    return false
  }
  // STORY 14.7b — CANCEL: A COMPENSATING SEQUENCE, NOT A TRANSACTION.
  //
  // Nothing was ever buffered, so there is nothing to roll back. What this does
  // is ask the engine to replay ITS OWN byte snapshots — `pushUndo(e.bytes)` —
  // exactly as many times as this session committed a change the engine agreed
  // was a change. The application holds an integer; the document is the
  // engine's throughout.
  //
  // ⚠ IT DOES NOT REUSE `applyHistory`, and both reasons are structural rather
  // than stylistic. (1) `applyHistory` gates on `undoAvailable`, which is React
  // STATE: across an N-iteration loop inside one handler it is stale, and stale
  // in the permissive direction, so it would guard nothing. (2) It calls
  // `setCurrentSnapshot(…, true)` on EVERY iteration, and that third argument
  // runs `clearDocumentInteraction` — which closes this dialog. The dialog
  // would be gone after undo #1, and a closed dialog can state nothing, which
  // is precisely what a failed undo needs it to do.
  //
  // ⚠ THE LOOP RUNS WITH THE DIALOG STILL OPEN. Closing is the SUCCESS PATH
  // ONLY.
  //
  // ⚠ THE BOUND AND ITS PRECONDITION ARE ONE FACT. `N <= MAX_ENGINE_HISTORY_ENTRIES`
  // is sound only because no other path can commit while this modal is open —
  // see the modal-open guard on the window shortcut handler below. 100 is not a
  // large number that makes overshoot unlikely; it is the engine's ring-buffer
  // size, and at 101 the oldest entry has already been evicted, so the sequence
  // would land one edit short and the last undo would fail. That is why the
  // footer DISABLES Cancel above the bound rather than attempting it.
  const cancelTableEditor = async () => {
    const current = tableEditor
    if (!engine || fileBusy || !current || tableEditorBusy) return
    const count = tableEditorEdits.current
    // The same refusal the footer draws, restated where it is load-bearing: a
    // disabled button is a UI fact, and this loop must not depend on one.
    if (count > MAX_ENGINE_HISTORY_ENTRIES) return
    const id = current.table.tableId
    const session = tableEditorSession.current
    const generation = documentGeneration.current
    // Nothing changed the document, so there is nothing to compensate for and
    // nothing to announce. Cancel is then exactly Done.
    if (count === 0) { closeTableEditor(); return }
    // `tableEditorDiscarding` IS WHAT SHUTS THE TWO WAYS OUT WHILE THIS RUNS.
    // The dialog stays on screen — closing is the success path only — so `Done`
    // and Escape are live gestures over a sequence they would tear down; both
    // are gated on this flag, and on this flag alone. It is cleared in the
    // `finally` below, which runs on the teardown return inside the loop as
    // well, and again by `openTableEditor` / `revokeTableEditor`.
    setTableEditorError(undefined); setTableEditorDiscarded(undefined); setTableEditorDiscarding(true); setTableEditorBusy(true)
    let discarded = 0
    let reached: EngineSnapshot | undefined
    let stopped: string | undefined
    try {
      while (discarded < count) {
        try { const result = await engine.request('undo'); reached = result.snapshot; discarded++ }
        catch (error) {
          // `applyHistory`'s catch is the precedent: UNDO_UNAVAILABLE is mapped
          // to STATE, never thrown away and never rethrown as a failure. The
          // engine's failure envelope carries no snapshot, so the position we
          // reached is the last successful `reached`, not anything in here.
          const received = error as { code?: string }
          stopped = received.code === 'UNDO_UNAVAILABLE' ? 'the engine had nothing left to undo' : componentDiagnostic(error)
          break
        }
        // Something tore the session down mid-sequence. Whatever did it owns the
        // dialog now; installing over it would be this loop talking about a
        // document it no longer edits.
        //
        // ⚠ THIS IS NOW REACHABLE ONLY BY A NON-UI TEARDOWN — a document
        // replacement, or a commit whose re-projection revoked the session — and
        // that is the point of the `discarding` gate. The two gestures that used
        // to reach it, `Done` and Escape, could return here with the engine k
        // undos back and the canvas, `snapshotRef` and the preview still showing
        // the pre-Cancel document: the install below is skipped, and nothing on
        // screen said the two had parted company. The flag makes the UI unable to
        // get here; the guard stays because the other paths still can.
        if (tableEditorSession.current !== session || documentGeneration.current !== generation) { setTableEditorDiscarding(false); return }
      }
      // ONCE, at the end — not once per iteration.
      if (reached) { invalidatePreview(); setCurrentSnapshot(reached) }
      // NO `modeRef.current === 'preview'` ARM HERE, and its absence is measured
      // rather than assumed. It would have been copied from `applyHistory`, where
      // undo/redo really can be pressed in preview mode. This dialog cannot be:
      // `Configure columns` renders only in the design inspector, so the session
      // can only be opened in design mode; while it is open Alt+P is suppressed
      // by this story's own modal guard; and the PREVIEW control cannot be
      // clicked, because `.table-editor-backdrop` is `position: fixed; inset: 0;
      // z-index: 20` over the whole viewport and `trapDialog` wraps Tab at both
      // ends. So `modeRef.current` is 'design' at every reachable arrival here —
      // and the discard sentence below renders only inside the design `<main>`
      // anyway, so a preview-mode arrival would have announced nowhere.
      if (stopped === undefined) {
        setTableEditorEdits(0)
        // THE HONEST LIMIT IS PART OF THE SENTENCE. `Undo()` calls
        // `pushRedo(e.bytes)` before restoring, so the discarded edits are
        // redoable — but only until the next committed command, which sets
        // `e.redo = nil`. A discard stated as permanent would be a lie in one
        // direction and a discard stated as reversible forever a lie in the other.
        //
        // ⚠ STATED **AFTER** THE CLOSE, and the order is load-bearing:
        // `closeTableEditor` → `revokeTableEditor` withdraws this sentence along
        // with the count, so setting it first would have it wiped on the way out.
        // Both are state writes in one handler, so React commits the pair in a
        // single render and the sentence survives.
        closeTableEditor()
        setTableEditorDiscarded(`Discarded ${discarded} table editor ${discarded === 1 ? 'edit' : 'edits'}. Redo restores ${discarded === 1 ? 'it' : 'them'} until your next committed edit, which clears the engine's redo history.`)
        return
      }
      // AC6. The dialog STAYS OPEN, re-projects the document it actually
      // reached, and states the real position — both numbers, so a message
      // claiming a completed discard cannot pass for this one.
      setTableEditorEdits(count - discarded)
      const projected = await engine.request('table-columns', new TextEncoder().encode(JSON.stringify({ id })).buffer)
      if (tableEditorSession.current === session && documentGeneration.current === generation && snapshotRef.current?.revision === projected.snapshot.revision && projected.tableColumns?.revision === projected.snapshot.revision && projected.tableColumns.table.tableId === id) setTableEditor(projected.tableColumns)
      if (tableEditorSession.current === session) setTableEditorError(`Discarded ${discarded} of ${count} edits, then stopped: ${stopped}. The other ${count - discarded} still stand, and this dialog is showing the document as it is now.`)
    } catch (error) { if (tableEditorSession.current === session) setTableEditorError(componentDiagnostic(error))
    // THE FLAG IS CLEARED UNCONDITIONALLY AND THE BUSY FLAG IS NOT, and the
    // asymmetry is deliberate. `tableEditorBusy` is scoped to the session it was
    // raised for; `discarding` exists only to shut `Done` and Escape while THIS
    // sequence runs, so it must come down on every exit — the success path, the
    // failure path, the throw and the teardown return — or the dialog it is still
    // rendering would have no way out at all.
    } finally { setTableEditorDiscarding(false); if (tableEditorSession.current === session) setTableEditorBusy(false) }
  }
  const bindPickedPath = async (segments: ReadonlyArray<string>) => {
    const id = selectedRef.current.length === 1 ? selectedRef.current[0] : undefined
    if (!engine || fileBusy || !id || bindingInFlight.current) return
    const component = snapshotRef.current?.canvas?.components.find((candidate) => candidate.id === id)
    if (!component || selectedTableColumn || (component.type !== 'table' && !SCALAR_BINDING_COMPONENT_TYPES.includes(component.type))) return
    const requestGeneration = documentGeneration.current
    const priorRevision = snapshotRef.current?.revision
    const requestSample = sampleDataRef.current
    const requestSegments = [...segments]
    setBindingError(undefined)
    bindingInFlight.current = true
    setBindingBusy(true)
    try {
      const result = await engine.request('command', component.type === 'table' ? bindTableCollectionCommand(id, segments) : bindComponentScalarCommand(id, segments))
      // Selection is transient and cannot revoke an already committed engine
      // command. Only a document replacement or a newer authoritative view
      // can prevent this response from becoming the current projection.
      if (documentGeneration.current === requestGeneration && snapshotRef.current?.revision === priorRevision) {
        if (result.snapshot.revision !== priorRevision) invalidatePreview()
        setCurrentSnapshot(result.snapshot)
      }
    } catch (error) {
      if (requestSample && documentGeneration.current === requestGeneration && sampleDataRef.current === requestSample && selectedRef.current.length === 1 && selectedRef.current[0] === id && snapshotRef.current?.revision === priorRevision) setBindingError({ sample: requestSample, componentID: id, segments: requestSegments, message: componentDiagnostic(error) })
    } finally {
      bindingInFlight.current = false
      if (documentGeneration.current === requestGeneration) setBindingBusy(false)
    }
  }
  // STORY 14.10 — THE COLUMN BIND, IN `bindPickedPath`'S SHAPE AND NOT IN
  // `commitTableColumn`'S.
  //
  // It is a DATA-PANEL bind: the same `bindingInFlight` latch, the same
  // `bindingBusy` indicator and the same scoped `bindingError`, so a refusal is
  // presented where the pick was made rather than in a dialog that is not open.
  // `commitTableColumn` is the table editor's committer — it gates on
  // `tableEditor` being present and counts edits against a dialog session — and
  // reusing it here would have made a main-window pick a no-op.
  //
  // ⚠ ONE COMMAND, AND `updateTableColumnBindingCommand` IS UNCHANGED. It takes
  // the BARE row-relative field; Go resolves the alias itself (`row` unless the
  // table sets `as`) and writes `Bind = "{{" + alias + "." + field + "}}"`.
  // Nothing is sent before it — no `configureTableBinding` — because the column
  // is only offered fields of the collection the table is ALREADY bound to. That
  // is what makes AC6's one undo step true: `folio-go/internal/wasm/engine.go`'s `Apply` pushes
  // exactly one undo per accepted byte-changing command. It is asserted, not
  // built.
  const bindPickedColumn = async (field: string) => {
    const scope = selectedTableColumn
    if (!engine || fileBusy || !scope || bindingInFlight.current) return
    const requestGeneration = documentGeneration.current
    const priorRevision = snapshotRef.current?.revision
    const requestSample = sampleDataRef.current
    setBindingError(undefined)
    bindingInFlight.current = true
    setBindingBusy(true)
    try {
      const result = await engine.request('command', updateTableColumnBindingCommand(scope.tableId, scope.columnId, field))
      if (documentGeneration.current === requestGeneration && snapshotRef.current?.revision === priorRevision) {
        if (result.snapshot.revision !== priorRevision) invalidatePreview()
        setCurrentSnapshot(result.snapshot)
      }
    } catch (error) {
      if (requestSample && documentGeneration.current === requestGeneration && sampleDataRef.current === requestSample && snapshotRef.current?.revision === priorRevision) setBindingError({ sample: requestSample, componentID: scope.tableId, segments: [], message: componentDiagnostic(error), columnId: scope.columnId, field })
    } finally {
      bindingInFlight.current = false
      if (documentGeneration.current === requestGeneration) setBindingBusy(false)
    }
  }
  const place = (x: number, y: number) => {
    if (!placing || placing === 'sectionBreak') return
    const kind = placing
    // WHERE FOCUS WAS WHEN THE AUTHOR ASKED, read here and not when the engine
    // answers. The whole window a placement has to lose its claim in is the one
    // between the gesture and the response — reading it at the response would
    // capture wherever the author had already gone and then call that "no
    // change", which is the opposite of the guard.
    const from = document.activeElement
    clearInteraction()
    void commitComponent(dropComponentCommand(kind, x, y, snapEnabled)).then((placed) => selectPlaced(placed, from))
  }
  // THE SECOND PLACEMENT SPELLING, for the sheets that did not exist before
  // (Ruling H). `dropComponent` carries a PAGE point and Go hit-tests it, and
  // hitTestBand's rectangle is one page tall — so a point on sheet three
  // would resolve to whichever band of page ONE it happened to land in, and
  // be created there rather than refused. `createComponent` already exists on
  // the channel, already carries the band NAME and a band-relative
  // coordinate, and Go already handles it. Repeated header/footer image
  // placements use their template page point instead, so Go applies the same
  // image-drop containment on every occurrence.
  // SPEC-multi-pages story 3: `page` is the later page whose content band was
  // placed on; y is then in that page's own column. Page 1 omits it.
  const placeInBand = (band: CanvasProjection['bands'][number]['name'], x: number, y: number, page?: number) => {
    if (!placing) return
    // spec-section-break: the break lives in the content band only. A click in
    // a page header or footer places nothing and leaves the entry armed.
    if (placing === 'sectionBreak') {
      if (band !== 'content') return
      // SPEC-multi-pages story 5: the break lands on the page whose content
      // band was chosen; page 1 names none.
      const target = page ?? 0
      clearInteraction()
      // A break that appeared on that page while armed (undo, redo) is never
      // moved by a click.
      const projection = snapshotRef.current?.canvas
      if (!projection || sectionBreakOnPage(projection, target) !== undefined) return
      placeSectionBreak(y, target)
      return
    }
    const kind = placing
    const from = document.activeElement
    clearInteraction()
    void commitComponent(createComponentCommand(kind, band, x, y, snapEnabled, page)).then((placed) => selectPlaced(placed, from))
  }
  // STORY 14.10 — THE COLUMN IS READ OFF THE EVENT TARGET, NEVER OFF A
  // COORDINATE.
  //
  // `closest('[data-column-id]')` asks the DOM which painted span was hit.
  // Mapping a pointer coordinate to a column would mean computing where each
  // column's edge lands on screen — a browser-side model of the columns, which
  // is exactly what AD-15 / I-4 bars and exactly the premise Story 14.9 was
  // built on: the canvas paints tables from the ENGINE's projection, never a
  // browser-side model of the columns.
  //
  // A column is selected only by an unmodified click on a single table.
  // Shift toggles component membership; selected multi-table bodies belong
  // to the group gesture.
  //
  // ⚠ AND THE TARGET IS OPTIONAL BECAUSE THE KEYBOARD PATH HAS NONE. Enter and
  // Space on a canvas component pass nothing, so they select the component and
  // drop any column selection — 14.10 ships mouse-only by [D-14.10.3], and
  // DW-387 registers the unmet half of UX-DR25.
  const select = (id: string, extend: boolean, target?: EventTarget | null) => {
    canvasSelection.cancel()
    const current = selectedRef.current
    const ids = extend ? (current.includes(id) ? current.filter((value) => value !== id) : [...current, id]) : current.includes(id) ? current : [id]
    const wanted = new Set(ids)
    installSelection((snapshotRef.current?.canvas?.components ?? []).filter((component) => wanted.has(component.id)).map((component) => component.id))
    const host = !extend && ids.length === 1 && target instanceof Element ? target.closest('[data-column-id]') : null
    const columnId = host?.getAttribute('data-column-id') ?? undefined
    if (columnId) setColumnSelection({ tableId: id, columnId })
  }
  // STORY 14.3 — PLACING IS SELECTING, AND SELECTING IS STILL ONE FUNCTION.
  //
  // Placing a component used to leave nothing selected, so the inspector stayed
  // on PAGE SETUP and the author had to go and find the thing they had just
  // made — which for a 1pt Line is a two-pixel target. This routes through
  // `select` above rather than reaching for `setSelected`, so a placed
  // component is selected by EXACTLY the mechanism a clicked one is: same
  // table-editor revocation, same binding-error clear, one path to audit.
  //
  // THE FOCUS MOVE IS DEFERRED, in the shape `returnWithOptionalSelection`
  // already uses: the element being focused does not exist until React has
  // rendered the snapshot this id came out of.
  //
  // It is found by scanning `data-component-id` rather than by building a
  // selector string, so an id needs no escaping to be findable. Exactly ONE
  // occurrence of a component is interactive (`occurrence.home` — the others
  // are aria-hidden echoes and carry no id attribute), so the scan cannot be
  // ambiguous.
  //
  // ⚠ NOTHING HERE SENDS A COMMAND. Selection and focus are local, and the
  // absence of engine traffic is asserted rather than assumed.
  const selectPlaced = (id?: string, from: Element | null = document.activeElement) => {
    if (!id) return
    select(id, false)
    setPendingFocus({ id, from })
  }
  // STORY 14.3 — THE FOCUS LANDS WHEN THE ELEMENT APPEARS, NOT ON ONE GUESSED
  // TICK, and the difference was a real defect rather than a tidiness point.
  //
  // This was `setTimeout(() => …placed?.focus(), 0)`, and it failed on every
  // sheet AFTER THE FIRST: a later-sheet placement costs an extra render pass,
  // the timeout fired before the component was mounted, the lookup found
  // nothing, and `?.focus()` dropped the miss SILENTLY AND FOREVER — one
  // attempt, no retry, no diagnostic. It was invisible to the tests because
  // `waitFor` retries the ASSERTION, never the attempt, and every row placed on
  // sheet one where the single tick happened to be enough.
  //
  // The pending id is STATE and this effect is KEYED ON THE SNAPSHOT, so the
  // attempt is driven by the arrival of the projection that mounts the element
  // rather than by a guessed tick. `canvas` and the sheet stack are both derived
  // during render (`snapshot?.canvas`, `sheetStack(canvas)`) — no intermediate
  // effect-driven state stands between the snapshot and the mounted component —
  // so the render this effect follows is the render that mounts it, and a
  // snapshot that has not mounted it yet leaves the claim standing for the next.
  //
  // ⚠ IT REFUSES TO STEAL FOCUS THE AUTHOR HAS ALREADY MOVED. A deferred focus
  // that fires late can yank the caret out of a property field the author
  // clicked into while the command was in flight. `from` is where focus was
  // WHEN THE AUTHOR MADE THE GESTURE; if it has moved somewhere else since, and
  // that somewhere is still on the page, the placement gives up its claim. Focus
  // falling back to `body` — which is what happens when PAGE SETUP is replaced
  // by the component panel under it — is not the author moving it.
  //
  // ⚠ AND IT CANNOT OUTLIVE ITS COMPONENT OR ITS DOCUMENT. There is no timer
  // left to cancel on unmount, and a pending id that is no longer in the
  // snapshot — deleted, undone, or a document replaced underneath it — is
  // dropped rather than left waiting for an element that will never arrive.
  // Without that clear, a claim outliving its component could land on a LATER
  // component that reuses the id after an undo.
  //
  // ⚠ THAT LAST CLEAR IS DEFENCE IN DEPTH THAT NO TEST REACHES, AND IT IS SAID
  // HERE RATHER THAN LEFT TO LOOK COVERED. Deleting it leaves the suite green
  // (measured). It is only reachable when the claim survives a render without
  // landing — the design canvas not mounted at that moment — and then the
  // document moves on without the component; nothing in the unit suite can
  // sequence that. Read the two guards above as covered and this one as not.
  useEffect(() => {
    if (!pendingFocus) return
    const { id, from, preventScroll } = pendingFocus
    if (!snapshotRef.current?.canvas?.components.some((component) => component.id === id)) { setPendingFocus(undefined); return }
    if (document.activeElement !== from && document.activeElement !== document.body && from?.isConnected === true) { setPendingFocus(undefined); return }
    const placed = Array.from(canvasRegionRef.current?.querySelectorAll<HTMLElement>('[data-component-id]') ?? []).find((element) => element.dataset.componentId === id)
    // Not mounted yet. Keep the claim; the next render runs this again.
    if (!placed) return
    setPendingFocus(undefined)
    // An echo press focuses mid-gesture: a scroll would cancel the drag.
    placed.focus(preventScroll ? { preventScroll: true } : undefined)
  }, [pendingFocus, snapshot])
  useEffect(() => {
    if (pendingBreakFocus === undefined) return
    const handle = canvasRegionRef.current?.querySelector<HTMLElement>(`.section-break-handle[data-break-page="${pendingBreakFocus}"]`)
    if (!handle) {
      const projection = snapshotRef.current?.canvas
      if (!projection || sectionBreakOnPage(projection, pendingBreakFocus) === undefined) setPendingBreakFocus(undefined)
      return
    }
    setPendingBreakFocus(undefined)
    handle.focus()
  }, [pendingBreakFocus, snapshot])
  // Any selection size. One component keeps its single-id command; a group is
  // ONE deleteComponents command, so one undo restores all of it. A refusal
  // leaves the selection standing (commitComponent reports it).
  const deleteSelection = () => {
    const ids = selectedRef.current
    if (ids.length === 0 || mutationInFlight.current) return
    mutationInFlight.current = true
    void commitComponent(ids.length === 1 ? deleteComponentCommand(ids[0]!) : deleteComponentsCommand(ids), () => { revokeTableEditor(); selectedRef.current = []; setSelected([]); setCurrentPage(0); setColumnSelection(undefined) }).finally(() => { mutationInFlight.current = false })
  }
  // THE ONE OWNERSHIP CHECK for canvas keys, shared by the window arm and a
  // focused component's own Delete. Design mode, no modal, nothing else owning
  // the canvas, and the key aimed at the canvas (or at no control at all).
  const canvasKeyAllowed = (event: Pick<KeyboardEvent, 'target'>): boolean =>
    modeRef.current === 'design' && engine !== undefined && !fileBusy && !startupOpen && !unsavedWarningOpen && tableEditor === undefined && pageDeleteConfirm === undefined && placing === undefined && (drag === undefined || drag.released === true) && boundaryDrag === undefined && sectionBreakDrag === undefined && !canvasSelection.active()
    && (event.target === document.body || (event.target instanceof Node && canvasRegionRef.current?.contains(event.target) === true))
  const keyboardDelete = (event: Pick<KeyboardEvent, 'target' | 'repeat' | 'shiftKey' | 'metaKey' | 'ctrlKey' | 'altKey'>): boolean => {
    if (!canvasKeyAllowed(event) || event.repeat || event.shiftKey || event.metaKey || event.ctrlKey || event.altKey) return false
    if (breakPage !== undefined) { deleteSectionBreak(breakPage); return true }
    if (selectedRef.current.length === 0) return false
    deleteSelection()
    return true
  }
  const copySelection = () => { clipboardRef.current = { identity: documentIdentity.current, lists: [[...selectedRef.current]] } }
  // Paste duplicates the newest copied list with ids still in the document,
  // then selects the copies and pushes THEM onto the clipboard, so repeated
  // pastes stair-step and an undone paste falls back to what it copied. Go
  // decides where each copy lands.
  const pasteClipboard = (): boolean => {
    const clip = clipboardRef.current
    if (!clip || clip.identity !== documentIdentity.current || mutationInFlight.current) return false
    const present = new Set((snapshotRef.current?.canvas?.components ?? []).map((component) => component.id))
    for (let index = clip.lists.length - 1; index >= 0; index--) {
      const ids = clip.lists[index]!.filter((id) => present.has(id))
      if (ids.length === 0) continue
      const kept = clip.lists.slice(0, index + 1)
      mutationInFlight.current = true
      // The copies land on the page under the pointer; off every sheet, or in a
      // one-page document, the command is today's and each copy stays put.
      const pointerPage = pointerPageRef.current
      const page = pointerPage !== undefined && (snapshotRef.current?.canvas?.pageBreaks?.length ?? 1) > 1 ? pointerPage : undefined
      void commitComponent(duplicateComponentsCommand(ids, snapEnabled, page), (added) => {
        if (added.length === 0 || clipboardRef.current !== clip) return
        clipboardRef.current = { identity: clip.identity, lists: [...kept, added].slice(-32) }
        installSelection(added)
      }).finally(() => { mutationInFlight.current = false })
      return true
    }
    return false
  }
  const selectAllInFocusBand = () => {
    const band = focusBandRef.current
    installSelection((snapshotRef.current?.canvas?.components ?? []).filter((component) => component.band === band).map((component) => component.id))
  }
  const duplicateSelection = () => { if (selected.length === 1) void commitComponent(duplicateComponentCommand(selected[0]!, snapEnabled)) }
  const nudgeSelection = (dx: number, dy: number) => {
    const component = snapshotRef.current?.canvas?.components.find((candidate) => candidate.id === selectedRef.current[0])
    // Keyboard nudges are precise relative steps; grid snapping can erase a
    // one-point move or shift the coordinate on the other axis.
    if (component && selectedRef.current.length === 1) void commitComponent(moveComponentCommand(component.id, component.x + dx, component.y + dy, false))
  }
  // THE ONE PLACE A BOUNDARY GESTURE BECOMES A COMMAND, and it sends exactly
  // the command Story 12.1 shipped. `points()` spells the proposal in the same
  // decimal the projection was read out of, so a gesture that returned to its
  // start compares equal below and costs no round trip, no history entry and no
  // dirty mark — the same send-only-if-changed rule the panel's typed path
  // obeys, and what folio-go/internal/wasm/engine.go's byte-equality short circuit backs up.
  const sendBandHeight = (band: CappingBand, proposed: number, original: number) => {
    if (proposed === original) return
    void commitComponent(bandHeightCommand(band, points(proposed), snapEnabled))
  }
  // THE FOUR POINTER GUARDS ARE 17.5's, taken from beginProseResize rather than
  // from CanvasComponent's `begin`, which has none of them: primary button
  // only, one pointer owns the gesture, preventDefault before focus moves, and
  // the anchor recorded BEFORE setPointerCapture (which throws NotFoundError on
  // an inactive pointerId and would otherwise swallow the press).
  const beginBoundaryDrag = (band: CappingBand, event: PointerEvent<HTMLButtonElement>) => {
    // CSS makes the strip transparent to placement pointer events so the
    // underlying band receives the drop. Keep this guard for direct events
    // too: an armed palette must never begin a boundary resize.
    if (placing) return
    if (event.button !== 0) return
    if (boundaryDragRef.current !== undefined && boundaryDragRef.current.pointerId !== event.pointerId) return
    const projection = snapshotRef.current?.canvas
    const original = projection ? projectedBandHeight(projection, band) : undefined
    if (!projection || original === undefined) return
    event.preventDefault()
    const started: BoundaryDrag = { band, pointerId: event.pointerId, startClientY: event.clientY, original, limit: bandBoundaryCeiling(projection.bands, band), proposed: original, changed: false }
    boundaryDragRef.current = started
    setBoundaryDrag(started)
    event.currentTarget.setPointerCapture?.(event.pointerId)
  }
  // Both endings of a gesture that sends nothing: pointercancel, and a move
  // with no button held. Neither leaves drag state behind, and neither lets an
  // unrelated pointer end someone else's drag.
  const cancelBoundaryDrag = (event: PointerEvent<HTMLButtonElement>) => {
    if (boundaryDragRef.current?.pointerId !== event.pointerId) return
    abortBoundaryDrag()
  }
  // THE ONE PIXEL-TO-MILLIPOINT CONVERSION IN THIS GESTURE, so the move and the
  // release cannot disagree about what a given travel means. documentDelta is
  // the shared mapping every other canvas drag uses; proposedBandHeight rounds
  // the product back to a whole millipoint (see its own header for why).
  const proposedFor = (from: BoundaryDrag, travel: number) => proposedBandHeight(from.band, from.original, canvasDisplay.documentDelta(travel, zoom) * 1000, from.limit)
  // Pure clientY arithmetic against the press anchor, through
  // canvasDisplay.documentDelta — never a measured box, and never an increment
  // against the last event, which would accumulate rounding across a long drag.
  const moveBoundaryDrag = (event: PointerEvent<HTMLButtonElement>) => {
    const from = boundaryDragRef.current
    if (from === undefined || event.pointerId !== from.pointerId) return
    // A MOVE WITH NOTHING HELD DOWN IS A HOVER, NOT A DRAG, and it is checked
    // AFTER the id so a second pointer hovering cannot tear down a live
    // gesture. It ends the drag the way pointercancel does — the proposal is
    // discarded and nothing is sent.
    if (event.buttons === 0) { cancelBoundaryDrag(event); return }
    const rawDY = event.clientY - from.startClientY
    const next: BoundaryDrag = { ...from, changed: from.changed || Math.abs(rawDY) >= 2, proposed: proposedFor(from, rawDY) }
    boundaryDragRef.current = next
    setBoundaryDrag(next)
  }
  const finishBoundaryDrag = (event: PointerEvent<HTMLButtonElement>) => {
    const from = boundaryDragRef.current
    if (from === undefined || event.pointerId !== from.pointerId) return
    abortBoundaryDrag()
    // THE RELEASE'S OWN COORDINATE DECIDES, not the last pointermove's. A
    // pointerup can carry the pointer somewhere new without an intervening
    // move — a fast flick, a capture handed back, a coalesced sequence — and
    // committing the stale proposal would write a height the author never saw
    // the line at. It goes through exactly the same clamped path the move does,
    // so the released value is still the one on screen.
    const travel = event.clientY - from.startClientY
    if (!(from.changed || Math.abs(travel) >= 2)) return
    sendBandHeight(from.band, proposedFor(from, travel), from.original)
  }
  // THE GESTURE MUST NEVER BE THE ONLY WAY TO REACH THE VALUE. Arrow keys step
  // the focused boundary by a point, Shift by ten, one command per press.
  //
  // stopPropagation IS LOAD-BEARING, not tidiness. The window `shortcut`
  // handler's arrow-key arm is SELECTION-driven, not focus-driven, so with a
  // component selected AND this handle focused both would fire: the boundary
  // would move and so would the component (Matrix row 12). React's
  // stopPropagation calls the native one, and React's listener sits on the root
  // container, which is inside window — so the window handler never sees it.
  const nudgeBoundary = (band: CappingBand, event: ReactKeyboardEvent<HTMLButtonElement>) => {
    // A MODIFIED KEY BELONGS TO THE APPLICATION, NOT TO THE HANDLE. Save, undo,
    // redo, duplicate, snap and the preview toggle all live on the window
    // `shortcut` handler, and a control that swallowed them would take them
    // away from the author for as long as focus sat on a 7px strip. Tab is left
    // alone for the same reason: it is how focus leaves.
    if (event.metaKey || event.ctrlKey || event.altKey || event.key === 'Tab') return
    // EVERY OTHER KEY IS THE FOCUSED CONTROL'S, and stopping here is what makes
    // that true rather than aspirational. Two things reach past this handle
    // otherwise, and neither is the boundary's business: the window handler's
    // arrow arm is SELECTION-driven, so ArrowLeft/ArrowRight move a selected
    // component while focus is on the boundary; and the band <section>'s own
    // onKeyDown treats Enter/Space as a PLACEMENT and drops an armed palette
    // component at the band origin.
    event.stopPropagation()
    if (event.key === 'Escape') { abortBoundaryDrag(); return }
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') {
      // Swallow the default too for the keys that would otherwise DO something
      // here — Enter and Space activate the button, which the page surface
      // reads as a click and answers by clearing the selection.
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight' || event.key === 'Enter' || event.key === ' ') event.preventDefault()
      return
    }
    event.preventDefault()
    const projection = snapshotRef.current?.canvas
    const original = projection ? projectedBandHeight(projection, band) : undefined
    if (!projection || original === undefined) return
    const step = (event.shiftKey ? 10_000 : 1_000) * (event.key === 'ArrowDown' ? 1 : -1)
    sendBandHeight(band, proposedBandHeight(band, original, step, bandBoundaryCeiling(projection.bands, band)), original)
  }
  // ---------------------------------------------------------------------------
  // spec-section-break CAP-1: THE SECTION BREAK ON THE CANVAS.
  //
  // It behaves like an element and is modelled on the band boundary above:
  // placed from the palette, selected with its own state, dragged by clientY
  // travel with one proposal line, nudged 1pt (Shift 10pt), typed in Properties
  // and deleted. Each gesture is ONE engine command, so one undo entry, and the
  // engine snaps and refuses; nothing here measures the DOM.
  // SPEC-multi-pages story 5: selecting a break selects THAT page's break and
  // makes its page current.
  const selectSectionBreak = (page: number) => { installSelection([]); setSectionBreakSelected(page); setCurrentPage(page) }
  // SPEC-multi-pages story 2: PAGES ON THE CANVAS. Selecting a page is designer
  // state; adding, deleting and Page Break are each ONE engine command, so one
  // undo entry, and the engine refuses what cannot be done.
  const selectPage = (page: number) => { installSelection([]); setSelectedPage(page); setCurrentPage(page) }
  // SPEC-multi-pages story 4: a press on a header or footer ECHO acts like a
  // press on the component itself, makes the echo's page current, and sends
  // keyboard focus to the component's one interactive copy on that page (it
  // remounts there, so the focus waits for it). While placing, echoes are
  // pass-through and this never runs.
  const pressRepeatingEcho = (id: string, page: number, event: PointerEvent) => {
    if (placing || event.button !== 0) return
    event.stopPropagation()
    if (event.shiftKey) { event.preventDefault(); select(id, true, event.target) }
    else beginSelectedGroup(id, event)
    // A Shift-press that toggled it OUT leaves what installSelection set.
    if (!selectedRef.current.includes(id)) return
    setCurrentPage(page)
    setPendingFocus({ id, from: document.activeElement, preventScroll: true })
  }
  // After the selected page, or at the end; the new page arrives selected.
  const addPage = () => {
    const projection = snapshotRef.current?.canvas
    if (!projection || mutationInFlight.current) return
    const after = pageSelection
    const inserted = after === undefined ? pageCountOf(projection) : after + 1
    mutationInFlight.current = true
    void commitComponent(addPageCommand(after), () => selectPage(inserted)).finally(() => { mutationInFlight.current = false })
  }
  const requestDeletePage = () => { if ('page' in deletePageTarget) setPageDeleteConfirm(deletePageTarget.page) }
  const cancelDeletePage = () => { setPageDeleteConfirm(undefined); deletePageButtonRef.current?.focus() }
  const confirmDeletePage = () => {
    const page = pageDeleteConfirm
    setPageDeleteConfirm(undefined)
    // The dialog unmounts and Delete page is usually disabled afterwards, so
    // focus goes to the canvas region rather than falling to <body>.
    canvasRegionRef.current?.focus({ preventScroll: true })
    if (page === undefined || mutationInFlight.current) return
    mutationInFlight.current = true
    void commitComponent(deletePageCommand(page), () => installSelection([])).finally(() => { mutationInFlight.current = false })
  }
  // D-5.1: disabled while the current page already has its break.
  const armSectionBreak = () => { if (breakOnCurrentPage) return; setPlacing('sectionBreak'); setHoverBand(undefined) }
  // `y` is the click's content-column offset in points, in `page`'s column.
  // The engine snaps it. Page 1 names no page and sends today's bytes.
  const placeSectionBreak = (y: number, page: number) => {
    void commitComponent(setSectionBreakCommand(points(Math.round(y * 1000)), snapEnabled, page), () => { selectSectionBreak(page); setPendingBreakFocus(page) })
  }
  const deleteSectionBreak = (page: number | undefined) => {
    const projection = snapshotRef.current?.canvas
    if (page === undefined || mutationInFlight.current || !projection || sectionBreakOnPage(projection, page) === undefined) return
    mutationInFlight.current = true
    void commitComponent(removeSectionBreakCommand(page), () => setSectionBreakSelected(undefined)).finally(() => { mutationInFlight.current = false })
  }
  // Send-only-if-changed, as sendBandHeight: a gesture that returns to its
  // start costs no round trip and no history entry.
  const sendSectionBreak = (proposed: number, original: number, snap: boolean, page: number) => {
    if (proposed === original) return
    void commitComponent(setSectionBreakCommand(points(proposed), snap, page))
  }
  const nudgeSectionBreak = (page: number, large: boolean, down: boolean) => {
    const projection = snapshotRef.current?.canvas
    const original = projection ? sectionBreakOnPage(projection, page)?.offset : undefined
    if (!projection || original === undefined) return
    const step = (large ? 10_000 : 1_000) * (down ? 1 : -1)
    sendSectionBreak(proposedSectionBreak(original, step, contentBandHeight(projection)), original, false, page)
  }
  const beginSectionBreakDrag = (page: number, event: PointerEvent<HTMLButtonElement>) => {
    if (placing || event.button !== 0) return
    event.stopPropagation()
    if (sectionBreakDragRef.current !== undefined && sectionBreakDragRef.current.pointerId !== event.pointerId) return
    const projection = snapshotRef.current?.canvas
    const original = projection ? sectionBreakOnPage(projection, page)?.offset : undefined
    if (!projection || original === undefined) return
    event.preventDefault()
    selectSectionBreak(page)
    const started: SectionBreakDrag = { pointerId: event.pointerId, page, startClientY: event.clientY, original, limit: contentBandHeight(projection), proposed: original, changed: false }
    sectionBreakDragRef.current = started
    setSectionBreakDrag(started)
    event.currentTarget.focus()
    event.currentTarget.setPointerCapture?.(event.pointerId)
  }
  const cancelSectionBreakDrag = (event: PointerEvent<HTMLButtonElement>) => {
    if (sectionBreakDragRef.current?.pointerId !== event.pointerId) return
    abortSectionBreakDrag()
  }
  const proposedBreakFor = (from: SectionBreakDrag, travel: number) => proposedSectionBreak(from.original, canvasDisplay.documentDelta(travel, zoom) * 1000, from.limit)
  const moveSectionBreakDrag = (event: PointerEvent<HTMLButtonElement>) => {
    const from = sectionBreakDragRef.current
    if (from === undefined || event.pointerId !== from.pointerId) return
    if (event.buttons === 0) { cancelSectionBreakDrag(event); return }
    const rawDY = event.clientY - from.startClientY
    const next: SectionBreakDrag = { ...from, changed: from.changed || Math.abs(rawDY) >= 2, proposed: proposedBreakFor(from, rawDY) }
    sectionBreakDragRef.current = next
    setSectionBreakDrag(next)
  }
  const finishSectionBreakDrag = (event: PointerEvent<HTMLButtonElement>) => {
    const from = sectionBreakDragRef.current
    if (from === undefined || event.pointerId !== from.pointerId) return
    abortSectionBreakDrag()
    const travel = event.clientY - from.startClientY
    if (!(from.changed || Math.abs(travel) >= 2)) return
    sendSectionBreak(proposedBreakFor(from, travel), from.original, snapEnabled, from.page)
  }
  // The focused line owns its keys, and stops them reaching the window arm and
  // the band's Enter-to-place, exactly as nudgeBoundary does.
  const keySectionBreak = (page: number, event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (event.metaKey || event.ctrlKey || event.altKey || event.key === 'Tab') return
    if (event.key === 'Escape') {
      if (sectionBreakDragRef.current !== undefined) { event.stopPropagation(); event.preventDefault(); abortSectionBreakDrag() }
      return
    }
    event.stopPropagation()
    if (event.key === 'Delete' || event.key === 'Backspace') { event.preventDefault(); if (!event.repeat && !event.shiftKey) deleteSectionBreak(page); return }
    if (event.key === 'ArrowUp' || event.key === 'ArrowDown') { event.preventDefault(); selectSectionBreak(page); nudgeSectionBreak(page, event.shiftKey, event.key === 'ArrowDown'); return }
    if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectSectionBreak(page); return }
    if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') event.preventDefault()
  }
  // The line, its tab, its hit strip and a drag's proposal: DIRECT children of
  // the content band, outside `.band-window` (which clips), and <div> rather
  // than <span> so the `.page-band > span` band-tab rule cannot restyle them.
  // SPEC-multi-pages story 5: one marker per page with a break, drawn among
  // that page's own sheets. Its handle is "Section Break" in a one-page
  // document and "Section Break on page N" otherwise.
  const sectionBreakMarker = (y: number, page: number) => {
    const proposal = sectionBreakDrag?.changed && sectionBreakDrag.page === page ? sectionBreakDrag : undefined
    const at = (value: number) => ({ '--section-break-display-y': canvasDisplay.css(value, zoom) } as CSSProperties)
    const selectedHere = breakPage === page
    const anchored = canvas ? sectionBreakOnPage(canvas, page)?.anchored !== false : true
    return <>
      <div className={`section-break-line${selectedHere ? ' section-break-selected' : ''}`} aria-hidden="true" style={at(y)}><div className="section-break-tab">{anchored ? <SectionBreakAnchorIcon /> : undefined}Section Break</div></div>
      <button type="button" className="section-break-handle" data-break-page={page} aria-label={pageCount === 1 ? 'Section Break' : `Section Break on page ${page + 1}`} aria-pressed={selectedHere} style={at(y)} onPointerDown={(event) => beginSectionBreakDrag(page, event)} onPointerMove={moveSectionBreakDrag} onPointerUp={finishSectionBreakDrag} onPointerCancel={cancelSectionBreakDrag} onClick={(event) => { event.stopPropagation(); selectSectionBreak(page) }} onKeyDown={(event) => keySectionBreak(page, event)} />
      {proposal ? <><div className="section-break-proposal" aria-hidden="true" style={at(y + proposal.proposed - proposal.original)} /><div className="section-break-readout" aria-hidden="true" style={at(y + proposal.proposed - proposal.original)}>{points(proposal.proposed)}</div></> : undefined}
    </>
  }
  // STORY 12.1, WIDENED BY 12.2: APPLY IS A SEQUENCE, AND ITS HALVES REFUSE
  // DIFFERENTLY.
  //
  // THE ORDER IS: the two DOCUMENT-SETTINGS commands (locale, then utcOffset),
  // then the two BAND HEIGHTS, then the pageSetup command. The first four are
  // component commands and refuse LOCATED, through componentDiagnostic; the
  // last is a page-setup command and refuses through pageSetupDiagnostic's
  // fixed sentence. The sequence stops at the first refusal, so the common
  // failure leaves the document wholly unchanged.
  //
  // THE RESIDUE IS DISCLOSED RATHER THAN DESIGNED AWAY, and Story 12.2 makes it
  // WIDER rather than narrower, so the disclosure is widened with it: any
  // command of this sequence that is ACCEPTED before a later one is REFUSED
  // STANDS. Concretely — a `setDocumentLocale` that lands stays landed when the
  // offset, a band height, or the pageSetup that follows is refused; a band
  // height that lands stays landed when the pageSetup is refused. Each command
  // is individually atomic, which is what the engine guarantees; this gesture
  // is not, and nothing here claims it is.
  //
  // A ROW IS SENT ONLY WHEN IT DIFFERS FROM THE PROJECTED VALUE, and that is
  // not an optimisation. Re-sending a band height that has not changed is not
  // a no-op: on a hand-edited document that already strands a component, the
  // engine would refuse the height ALREADY IN FORCE, and an author who touched
  // only a margin could never apply anything again. The engine's own
  // unchanged-bytes short-circuit does not help — it runs after the door. This
  // comparison is two strings, both of them the engine's own spelling of its
  // own numbers; it is not a layout computation and it is not a bound.
  //
  // A REFUSAL FROM ANY OF THE FOUR COMPONENT COMMANDS GOES THROUGH
  // componentDiagnostic, never pageSetupDiagnostic: the latter discards the
  // engine's message for anything not carrying PAGE_SETUP_INVALID, and the
  // engine's own located sentence — the height it refused and the element it
  // would have stranded, or the field and the legal values for a locale or an
  // offset — is the entire point of refusing at the command door.
  //
  // AND BECAUSE IT IS NOW A SEQUENCE, IT IS GUARDED LIKE ONE. Before Story 12.1
  // this function awaited once and its `!engine || !canvas || fileBusy` test was
  // taken once, which was the whole of the check it needed. It now awaits up to
  // FIVE times — locale, utcOffset, the two band heights, pageSetup (Story 12.1
  // made it three; 12.2 added the first two) — with the Apply button live
  // throughout, so between any two of those awaits the author can open a file,
  // start a blank template, or undo — every
  // one of which REPLACES the document and advances documentGeneration — or
  // simply press Apply again. Neither must be able to land a later command of
  // this sequence on a document that is no longer the one the drafts were read
  // from. This is exactly the guard applyProperties, applyImageAsset and the
  // binding commit already take, in the shape they take it: a generation
  // captured before the first await and re-read after every one, plus an
  // in-flight flag so a second press cannot start a second sequence.
  const applyPageSetup = async () => {
    if (!engine || !canvas || fileBusy || pageSetupInFlight.current) return
    setCommitError(undefined)
    const requestGeneration = draftGeneration.current
    const requestDocument = documentGeneration.current
    pageSetupInFlight.current = true
    try {
      // THE TWO DOCUMENT-SETTINGS ROWS GO FIRST, before the band heights and
      // before the pageSetup command, for the same reason the band heights go
      // before pageSetup: the sequence stops at the first refusal, so the
      // cheapest and most independent writes are attempted first and a common
      // refusal leaves the document wholly unchanged.
      //
      // ONE COMMAND PER CHANGED ROW, and NOTHING for a row the author left
      // alone. Both comparisons are between two strings the ENGINE spelled —
      // the draft was seeded from the projection and nothing here rewrites it —
      // so an untouched row is byte-equal by construction and is worth no
      // command, no round trip and no history entry. Nothing here validates:
      // AD-12's closed set and ±HH:MM are the engine's rules, asked through one
      // exported predicate each, and a refusal arrives located on `locale` or
      // `utcOffset` and is rendered by componentDiagnostic — never by
      // pageSetupDiagnostic, which would discard the engine's sentence and
      // print one about size and margins that the author never touched.
      for (const [typed, projected, build] of [
        [draft.locale, canvas.locale, () => documentLocaleCommand(draft.locale as LocaleTag)],
        [draft.utcOffset, canvas.utcOffset, () => documentUTCOffsetCommand(draft.utcOffset)],
      ] as ReadonlyArray<readonly [string, string, () => ArrayBuffer]>) {
        if (typed === projected) continue
        const priorRevision = snapshotRef.current?.revision
        let result
        try {
          result = await engine.request('command', build())
        } catch (error) { if (documentGeneration.current === requestDocument) setCommitError(componentDiagnostic(error)); return }
        if (documentGeneration.current !== requestDocument) return
        if (result.snapshot.revision !== priorRevision) invalidatePreview()
        // Mid-gesture, so the drafts are KEPT: the margins and heights the
        // author typed have not been sent yet and must not be wiped out of
        // their boxes by a snapshot from a command that never carried them.
        setCurrentSnapshot(result.snapshot, true)
      }
      for (const band of CAPPING_BANDS) {
        const projected = projectedBandHeight(canvas, band)
        const typed = draft[band]
        // A band the projection does not carry has no row and no draft, and a
        // row the author left alone is worth no command, a round trip or a
        // history entry. Nothing here computes a bound: it compares two
        // strings, both of them the engine's own spelling of its own number.
        if (typed === undefined || projected === undefined || typed === points(projected)) continue
        const priorRevision = snapshotRef.current?.revision
        let result
        try {
          // `false`, ALWAYS, and it is what keeps this path byte-preserved
          // (12.5's R3): the box is typed, so an author who types 83 gets 83.
          // The Snap toggle governs GESTURES, and rounding a number somebody
          // spelled out would be the panel answering a question they answered.
          result = await engine.request('command', bandHeightCommand(band, typed, false))
        } catch (error) { if (documentGeneration.current === requestDocument) setCommitError(componentDiagnostic(error)); return }
        // THE DOCUMENT MAY HAVE BEEN REPLACED WHILE THAT WAS IN FLIGHT. The
        // command already landed — the engine holds whatever document it was
        // asked about — but nothing of this sequence may go on: neither the
        // snapshot, which would install a projection of a document the author
        // has left, nor the commands that would follow it.
        if (documentGeneration.current !== requestDocument) return
        if (result.snapshot.revision !== priorRevision) invalidatePreview()
        // The draft is deliberately KEPT here rather than reseeded: the
        // gesture is not finished, and reseeding would wipe the margins the
        // author typed out of the boxes before the page-setup command that
        // carries them has even been sent.
        setCurrentSnapshot(result.snapshot, true)
      }
      const priorRevision = snapshotRef.current?.revision
      let result
      try {
        result = await engine.request('command', pageSetupCommand(preset, orientation, preset === 'custom' ? draft.width : '0', preset === 'custom' ? draft.height : '0', draft))
      } catch (error) { if (documentGeneration.current === requestDocument) setCommitError(pageSetupDiagnostic(error)); return }
      if (documentGeneration.current !== requestDocument) return
      if (result.snapshot.revision !== priorRevision) invalidatePreview()
      setCurrentSnapshot(result.snapshot, draftGeneration.current !== requestGeneration)
    } finally {
      pageSetupInFlight.current = false
    }
  }
  const applyProperties = async (ids: ReadonlyArray<string>, intent: PropertyIntent | PropertyIntents, responseGeneration: number, selectionKey: string): Promise<CanvasProjection | undefined> => {
    if (!engine || fileBusy || documentGeneration.current !== responseGeneration || selectedRef.current.join(',') !== selectionKey) return undefined
    setCommitError(undefined)
    setPropertyError(undefined)
    // Clears propertyError's sibling too: a refusal is about the edit that
    // caused it, and leaving a stale chain/embed refusal standing in the
    // TYPOGRAPHY section while the author goes on committing font size or
    // bold shows them a sentence about an edit they have moved past.
    setFontChainError(undefined)
    try {
      const priorRevision = snapshotRef.current?.revision
      const result = await engine.request('command', updateComponentPropertiesCommand(ids, intent))
      if (documentGeneration.current !== responseGeneration) return undefined
      if (result.snapshot.revision !== priorRevision) invalidatePreview()
      setHistoryAvailability(result.snapshot)
      // A command result is authoritative only if it advances this snapshot;
      // field drafts independently ignore a response from a replaced scope.
      if ((snapshotRef.current?.revision ?? -1) < result.snapshot.revision) setCurrentSnapshot(result.snapshot)
      return documentGeneration.current === responseGeneration && selectedRef.current.join(',') === selectionKey ? result.snapshot.canvas : undefined
    } catch (error) {
      if (documentGeneration.current === responseGeneration && selectedRef.current.join(',') === selectionKey) {
        const diagnostic = componentDiagnosticDetail(error)
        // EVERY field the failing intent carried, not one. A refusal is
        // anchored by what the PANEL sent, never by what the engine returned —
        // and since Story 14.2 one intent may carry two fields, so the anchor
        // is a set and `errorFor` asks whether it contains the field it is
        // rendering beside. A single-field intent produces a one-member set and
        // behaves exactly as it did.
        setPropertyError({ fields: intentFields(intent), selectionKey, ...diagnostic })
      }
      return undefined
    }
  }
  // A FONT COMMAND TRAVELS THE SAME PATH A PROPERTY COMMIT TAKES — one opaque
  // command, one revision, one undo entry — and differs only in where a
  // refusal is anchored: by the CONTROL the panel dispatched from, never by
  // what the engine returned. That is what lets an unlocated ENGINE_REJECTED
  // (componentFields arity, a projection bound) be shown in exactly the same
  // place a located COMPONENT_INVALID is, with one rule. The message is the
  // engine's own string and is stored unprefixed, because the control already
  // says where it belongs.
  //
  // STORY 16.9 DELETED THIS FUNCTION'S OTHER TWO CALLERS, `applyFontChain` and
  // `pickCatalogueFamily`, WITH THE CONTROLS THEY SERVED — the chain editor
  // and the dropdown's install-tier pick. `engine-protocol.ts` still names the
  // six chain commands the removed editor issued, untouched, because this was
  // a UI removal and not a protocol change (see its own note). The one caller
  // left below is `dispatchEmbed`.
  //
  // STORY 16.3 GAVE IT A RETURN VALUE AND A SECOND ANNOUNCER, AND NEITHER
  // CHANGES AN EXISTING CALLER. The font browser dispatches one command per
  // staged family and has to name a refusal AGAINST THE FAMILY THAT EARNED IT —
  // three refusals cannot all be the panel's one `fontChainError`, and the two
  // that were overwritten would simply vanish. So the message is RETURNED, and
  // `announce` says whether this call also writes it to the panel.
  const sendFontChain = async (payload: ArrayBuffer, control: FontChainControl, responseGeneration: number, selectionKey: string, announce: 'panel' | 'caller' = 'panel'): Promise<string | undefined> => {
    if (!engine) return undefined
    setCommitError(undefined)
    if (announce === 'panel') setFontChainError(undefined)
    try {
      const priorRevision = snapshotRef.current?.revision
      const result = await engine.request('command', payload)
      if (result.snapshot.revision !== priorRevision) invalidatePreview()
      setHistoryAvailability(result.snapshot)
      if ((snapshotRef.current?.revision ?? -1) < result.snapshot.revision) setCurrentSnapshot(result.snapshot)
    } catch (error) {
      const message = componentDiagnosticDetail(error).message
      if (announce === 'panel' && documentGeneration.current === responseGeneration && selectedRef.current.join(',') === selectionKey) setFontChainError({ control, selectionKey, message })
      return message
    }
    return undefined
  }

  // STORY 8.6, WIDENED BY STORY 16.1: PICKING A FAMILY, FROM EITHER OF TWO TIERS.
  //
  // THE FUNCTION GAINED A SOURCE; IT DID NOT SWAP ONE (D-16.R.3). The local
  // tier is the 21 committed faces, read from the release bundle's own
  // content-addressed assets exactly as before — the same read `runtimeAssetUrls`
  // assets get, behind the service worker, so it works with the browser offline
  // and NO THIRD PARTY IS CONTACTED AT ALL. The web tier is a family from the
  // build-time index snapshot, whose metadata, licence text and bytes are
  // fetched at the moment of the pick.
  //
  // LOCAL WINS AND IS NEVER RE-FETCHED. `offeredFamilies` has already removed
  // the web duplicate of a family the local tier holds, so a `Roboto` or an
  // `Inter` pick never reaches `fetchWebFamily`, and the snapshot's `axes`
  // opinion about them is never consulted. The committed bytes carry a stronger
  // record than any fetch can produce; preferring a fetch would replace a
  // verified record with an unverified one.
  //
  // CLASSIFY, THEN EMBED. For the web tier every licence decision is made inside
  // `fetchWebFamily` BEFORE a byte is fetched, and a refusal comes back as a
  // sentence rather than as a face. No licence is knowable before a pick — the
  // index publishes no licence field — so this refusal is necessarily post-pick,
  // and it is surfaced at the control the author acted on.
  //
  // THE COMMAND IS UNTOUCHED, AT TWELVE FIELDS. `embedFontFamilyCommand` already
  // demands exactly what a `.folio` requires and Go refuses the pick without it,
  // so changing the SOURCE of those values while leaving that guard in place is
  // what keeps this story from reaching the format.
  //
  // THE PROPOSED TAIL IS COMPUTED HERE and is a PROPOSAL, not a rule: the
  // shipped faces for the scripts the picked face does not cover, in the order
  // scriptFallbackFaces names them. The document's chain data still carries it
  // exactly as ANY other chain; this designer ships no UI to hand-edit that
  // order (Story 16.9 removed the chain editor), so the proposal stands until
  // a future UI, or the engine's own commands, revise it (AC3).
  // THE BUSY FLAG IS HELD ACROSS THE WHOLE RESOLUTION, NOT ONLY THE COMMAND.
  // Before Story 16.1 the awaited work in front of this hold was ONE
  // same-origin read of a precached bundle asset, so the window in which a
  // second pick could pass this guard was negligible. It is now a chain of up to
  // six sequential cross-origin round-trips — up to four `METADATA.pb` probes,
  // then the licence file, then the face bytes — and a second pick during that
  // window would resolve concurrently and commit a second embed. So the flag is
  // taken HERE and released in the `finally`, and the command half is called
  // through `sendFontChain`, which does not take it again.
  //
  // STORY 16.3 BUILT THIS SEAM AND STORY 16.5 SWAPPED ITS BODY, WHICH IS EXACTLY
  // WHAT IT WAS BUILT FOR (D-16.R.46).
  //
  // IT USED TO EMBED. The bytes travelled into the document at the moment of a
  // pick, so a family the author merely tried landed in the `.folio` and stayed
  // there. IT NOW INSTALLS: fetch, classify, keep on this machine, AND SEND NO
  // ENGINE COMMAND AT ALL. The embed moved to first use — see
  // `embedInstalledFamily` below — so an installed-but-unused face is never in
  // the file to be pruned.
  //
  // BOTH CALLERS ARE UNCHANGED BY THAT SWAP, which is the property the seam was
  // built to have: the family control's pick and the font browser's confirm name
  // no mechanism, so neither had to learn a new one.
  //
  // THE NAME IS NOW WRONG AND IT IS KEPT ANYWAY, deliberately: it is the seam
  // D-16.R.46 Q2 named, two specs point at it by that name, and renaming it in
  // the same commit that inverts it would make the diff unreadable at exactly
  // the moment the record needs to be legible. It reads `installFamily` below,
  // which is what it does.
  //
  // NO REVISION, NO HISTORY ENTRY, NO UNDO, BY CONSTRUCTION rather than by
  // suppression. History is whole canonical `.folio` byte snapshots and `Apply`
  // short-circuits when the bytes do not move (`folio-go/internal/wasm/engine.go`), so an
  // action that sends no command cannot move any of the three.
  const addFamilyToDocument = async (source: FamilySource, responseGeneration: number, selectionKey: string, announce: 'panel' | 'caller' = 'panel'): Promise<string | undefined> => {
    if (!engine) return 'This designer has no engine to send the change to.'
    if (fileBusy || fontChainBusyRef.current) return `${source.family} was not installed: the designer was busy with another change. Try it again.`
    holdFontChain(true)
    try {
      return await installFamily(source, responseGeneration, selectionKey, announce)
    } finally {
      if (documentGeneration.current === responseGeneration) holdFontChain(false)
    }
  }

  /**
   * THE REFUSAL SURFACE, HOISTED SO BOTH HALVES OF THE SPLIT SAY IT THE SAME WAY.
   *
   * A refusal that resolves after the selection or the document moved on is NOT
   * shown in the panel — `sendFontChain`'s own rule, applied to the half of the
   * flow that happens before it is called. It is still RETURNED, because the
   * browser's caller is a modal that is still open and still owns a row for this
   * family: "the panel is looking at something else now" is a reason not to
   * paint an error on the panel, never a reason to swallow one.
   */
  const refuseFontChain = (message: string, responseGeneration: number, selectionKey: string, announce: 'panel' | 'caller'): string => {
    if (announce === 'panel' && documentGeneration.current === responseGeneration && selectedRef.current.join(',') === selectionKey) {
      setFontChainError({ control: { action: 'embed' }, selectionKey, message })
    }
    return message
  }

  /** A face resolved from any tier, carrying everything `embedFontFamily` refuses a document without. */
  type ResolvedFace = Readonly<{ family: string; style: string; licence: string; licenceText: string; copyright: string; source: string; mediaType: string; bytes: ArrayBuffer; scripts: ReadonlyArray<string> }>

  /**
   * THE ONE EMBED DISPATCH, so the three paths that can reach the command cannot
   * drift in what they send.
   *
   * THE PROPOSED TAIL IS COMPUTED HERE and is a PROPOSAL, not a rule: the shipped
   * faces for the scripts the picked face does not cover, in the order
   * `scriptFallbackFaces` names them. It lands in the document's chain data
   * like any other chain; this designer ships no UI to hand-edit that order.
   *
   * STORY 11.4 — THE TAIL NOW DECLARES THE CUTS ITS FACES HAVE. Every fallback
   * is a face this release ships, so `shipped-face-cuts.ts` knows what cuts it
   * has and the entry says so: `Noto Sans Thai` carries its bold, `Noto Sans
   * SC` carries none and stays a bare string (D-A). Before this, a document
   * whose Thai fallback was proposed by a pick could never bold its Thai even
   * though the engine shipped the face for it.
   *
   * ⚠ THE PICKED ENTRY ITSELF DECLARES NOTHING, and that is correct rather
   * than pending: a pick embeds ONE face, every catalogue face is a single
   * upright static Regular, and an entry may only declare a cut the document
   * actually carries.
   *
   * A FALLBACK OUTSIDE THE MIRROR FALLS BACK TO A BARE NAME, which is the same
   * entry today's pick writes — a family whose cuts are not declared has none
   * to declare, and that is an honest entry rather than a degraded one.
   */
  const dispatchEmbed = async (face: ResolvedFace, responseGeneration: number, selectionKey: string, announce: 'panel' | 'caller'): Promise<string | undefined> => {
    const tail = proposedFallbackTail(face.scripts)
    return sendFontChain(embedFontFamilyCommand({ chain: face.family, family: face.family, style: face.style, licence: face.licence, licenceText: face.licenceText, copyright: face.copyright, source: face.source, mediaType: face.mediaType, bytes: face.bytes, tail }), { action: 'embed' }, responseGeneration, selectionKey, announce)
  }

  /**
   * STORY 11.4 — PICKING A FAMILY THE RELEASE ALREADY SHIPS **NAMES** IT.
   *
   * IT EMBEDS NOTHING. D-16.5 moved the embed to "the moment a font starts
   * travelling", and a face that is already on every machine that can open the
   * file is not travelling: the engine hands its own FontSet to every render,
   * so a chain entry naming `Roboto` resolves everywhere `Roboto` is shipped.
   *
   * ⚠ MEASURED, AND IT IS WHY THIS PATH EXISTS AT ALL: picking `Roboto` used
   * to embed a byte-identical duplicate of the `Roboto` the engine already
   * ships — the same digest on both copies — as a Regular-only entry that
   * could never bold, while `Roboto Bold` sat unreachable in the same FontSet.
   * The pick now declares that family's cuts instead, and ~348 KB of duplicate
   * stops being written into documents.
   *
   * THE POPULATION IS ONE FAMILY, AND SAYING SO IS PART OF SHIPPING IT.
   * `Roboto` is the only shipped base family the catalogue carries at all;
   * `Noto Sans`, `Noto Sans Thai` and `Noto Sans SC` are uncatalogued
   * (D-11.1.5) and this control never offers them, and every catalogue and
   * fetched face is a single upright Regular by construction. So this is not
   * "picked families can now bold" — it is Roboto, and it is the family the
   * shipped starter already declares (D-11.4.2).
   *
   * TWO COMMANDS AND TWO UNDO ENTRIES, exactly as the embed path is: the chain
   * is declared, and only then may the property name it, because
   * `canvas.fontFamilies` is the closed set `style.fontFamily` may reference.
   * The engine forces that order; nothing here chooses it. The property half
   * is the family control's, for the same reason it is on the embed path.
   *
   * THE BUSY FLAG AND THE GENERATION GUARD ARE `embedInstalledFamily`'s, kept
   * rather than skipped because this is one `await` on a command that can move
   * the document under a slow machine just as the embed can.
   */
  const declareShippedFamily = async (source: FamilySource, responseGeneration: number, selectionKey: string): Promise<string | undefined> => {
    const family = source.family
    const refuse = (message: string) => refuseFontChain(message, responseGeneration, selectionKey, 'panel')
    const entry = shippedFamilyEntry(family)
    if (entry === undefined) {
      // UNREACHABLE BY THE ONE CALLER, AND NAMED RATHER THAN SKIPPED: the
      // family control asks the mirror the same question before routing here,
      // so arriving with a family the release does not ship is a routing
      // defect. Answering it by embedding would restore the duplicate.
      return refuse(`${family} is not a family this release ships, so it cannot be declared without carrying it. That is a routing defect, not something you did.`)
    }
    if (!engine) return refuse('This designer has no engine to send the change to.')
    if (fileBusy || fontChainBusyRef.current) return refuse(`${family} was not used: the designer was busy with another change. Try it again.`)
    holdFontChain(true)
    try {
      setFontChainError(undefined)
      // THE SAME PROPOSED TAIL THE EMBED PATH COMPUTES, from the same function.
      // A declare that wrote only the picked entry would hand back a chain with
      // LESS script coverage than the embed it replaced — measured: a `Roboto`
      // pick produced one entry where `starter.folio`'s own Roboto chain has
      // three, so every Thai and CJK run in the document lost its fallback and
      // nothing said so.
      const chain: ReadonlyArray<FontChainEntryAsk> = [entry, ...proposedFallbackTail(scriptsOfSource(source))]
      // ⚠ `action: 'embed'` ON A PATH THAT EMBEDS NOTHING, AND WHAT IT IS AND
      // IS NOT, MEASURED. `FontFamilyProperty` paints a pick refusal only when
      // `pickError.control.action === 'embed'`, so that string is what puts a
      // declare-path refusal on screen at the control the author acted on. It
      // is the ACTION OF THE CONTROL — "use this family here" — and not a claim
      // about bytes; both arms of the fork are that one gesture, which is why
      // it is not renamed.
      //
      // BUT THE COPY THAT PAINTS IS `refuseFontChain`'s, NOT THIS ONE. This
      // call passes `announce: 'caller'`, and `sendFontChain` writes the panel
      // error only when `announce === 'panel'` — so on this path the `control`
      // argument is never read, and the engine's own refusal reaches the author
      // through `refuse(...)` on the line below instead. Changing THIS string
      // is invisible; changing `refuseFontChain`'s hardcoded `'embed'` takes
      // every refusal this function makes off the screen with a green suite
      // unless something asserts it. `App.font-store.test.tsx`'s
      // "refuses a second declare of a shipped family…" is what reds, and it
      // was mutation-proved against that site rather than this one.
      const rejected = await sendFontChain(addFontChainCommand(family, chain), { action: 'embed' }, responseGeneration, selectionKey, 'caller')
      if (rejected !== undefined) return refuse(`${family} was not added to this document: ${rejected}`)
      if (documentGeneration.current !== responseGeneration || selectedRef.current.join(',') !== selectionKey) {
        return refuse(`${family} was declared, but no component was set in it: the document or the selection moved while the change was being written.`)
      }
      return undefined
    } finally {
      if (documentGeneration.current === responseGeneration) holdFontChain(false)
    }
  }

  /**
   * INSTALLING A FACE IS NOT EMBEDDING IT (Story 16.5, D-16.R.46).
   *
   * FETCH, CLASSIFY, STORE. NO ENGINE COMMAND. Three of the four steps the old
   * pick took are here unchanged, and the fourth — the embed — is gone from this
   * path entirely. The document's revision, history and asset map are untouched,
   * which is what makes "install what looks promising" cost the author's file
   * nothing.
   *
   * INSTALL RUNS EVERY ADMISSION CHECK THAT CAN RUN AT INSTALL. Moving the embed
   * to first use moves every refusal the command makes to a later moment than
   * the one the author acted in, and a refusal that arrives at first use is a
   * worse refusal. So `fetchWebFamily` does the whole admission it always did —
   * the closed licence-token table, the upstream licence text, nameID 0 from the
   * bytes — plus Story 16.5's refuse-only `fvar` filter over the fetched bytes.
   * Nothing here ADMITS anything: `embedFontFamily` still decides what enters a
   * document.
   *
   * ONE CHECK CANNOT MOVE AND IT IS NAMED RATHER THAN HIDDEN. Go's nameID-13
   * licence-signature tie (`internal/fontset/licencesignature.go`) compares a
   * face's declared licence against the licence written inside the face, and
   * porting it would need a second name-table reader and two regex tables in
   * this designer — a competing authority over what enters a document, which is
   * the one thing this story may not build. So a face CAN install successfully
   * and still be refused the first time it is used, and `embedInstalledFamily`
   * below says exactly that when it happens.
   *
   * TWO TIERS HAVE SOMETHING TO INSTALL SINCE spec-deferred-offline-cache STORY
   * 2, AND THEY INSTALL DIFFERENT THINGS. A `web` row is fetched, classified and
   * written to the machine store — the body below. A `local` row is a catalogue
   * face this release carries but this browser has not yet fetched, because the
   * catalogue is deferred now; installing it means PULLING ITS BYTES through the
   * service worker, which verifies them against the release manifest and keeps
   * them. Only a `stored` row still has nothing to install — it is in the
   * machine store by definition — and reaching this function with one is a
   * routing defect, NAMED rather than silently answered `undefined`, which would
   * report a no-op as a success.
   */
  const installFamily = async (source: FamilySource, responseGeneration: number, selectionKey: string, announce: 'panel' | 'caller' = 'panel'): Promise<string | undefined> => {
    const refuse = (message: string) => refuseFontChain(message, responseGeneration, selectionKey, announce)
    // A CATALOGUE FACE NOW HAS SOMETHING TO INSTALL, AND THIS IS IT
    // (spec-deferred-offline-cache, story 2). The 31 catalogue faces are
    // deferred: the release carries them, the worker does not precache them,
    // and until something asks, their bytes are not on this machine. So the
    // font browser is the door to an unfetched one — exactly as Story 16.9 made
    // it the only door to the web tier — and installing it means PULLING ITS
    // BYTES, nothing more.
    //
    // THE FETCH IS THE INSTALL, AND THE SERVICE WORKER IS THE MECHANISM. This
    // is a same-origin GET for a content-addressed URL in this release's own
    // manifest, so the worker answers it from the cache when held and otherwise
    // fetches, hash-verifies against that manifest entry and keeps it. Nothing
    // here re-implements any of that, and the response body is deliberately
    // dropped: what the author installed is the CACHE ENTRY, and the bytes
    // travel into a document only at first use, through `embedInstalledFamily`.
    //
    // NOTHING IS WRITTEN TO THE MACHINE STORE. That store is the fetched-web
    // tier's home and its records carry a licence provenance a catalogue face
    // already has committed beside its binary; a second copy there would be two
    // answers to one question.
    if (source.tier === 'local') {
      // THE WORKER HAS TO BE IN CHARGE, OR THIS INSTALLS NOTHING. An
      // uncontrolled page — the very first load, before `clients.claim()` has
      // taken effect — fetches straight past the worker, so the bytes arrive,
      // the response is fine, and NOTHING IS CACHED: the install would report
      // success while the family stayed absent from AVAILABLE LOCALLY, which is
      // the worst of the three possible outcomes. Refused by name instead, and
      // the remedy is one the author can act on.
      if (!('serviceWorker' in navigator) || !navigator.serviceWorker.controller) {
        return refuse(`${source.family} could not be put on this machine yet: this page's offline layer is not in charge of its own requests. Reload the page and try again.`)
      }
      try {
        const response = await fetch(source.face.url)
        if (!response.ok) throw new Error(`the bundled face responded ${response.status}`)
        await response.arrayBuffer()
      } catch (error) {
        return refuse(`${source.family} could not be fetched onto this machine: ${error instanceof Error ? error.message : String(error)}`)
      }
      refreshHeldLocalFamilies()
      return undefined
    }
    // Spelled as the discriminant rather than through `familyIsInstalled` so the
    // narrowing below is the compiler's and not a comment's.
    if (source.tier !== 'web') return refuse(`${source.family} is already on this machine, so there is nothing to install. Pick it again to use it in this document.`)
    const outcome = await fetchWebFamily(source.family)
    if (!outcome.ok) return refuse(outcome.reason)
    // LAYOUT DIVERGENCE IS AN OBSERVATION, AND AN OBSERVATION NEEDS A READER.
    // `fetchWebFamily` records when the directory a family was resolved in
    // disagrees with the licence its own metadata declares — never a refusal
    // (D-16.R.6: METADATA.pb wins, the directory is only where the files sit),
    // but worth seeing, because systematically it means the probe order is
    // costing round-trips. It is written to the browser's own log, which is
    // where a person already looks for what the designer did on a pick; it is
    // deliberately NOT a UI surface, because nothing here is wrong and an
    // author has no decision to make about it.
    if (outcome.face.layoutDivergence !== undefined) console.info(outcome.face.layoutDivergence)
    const face: ResolvedFace = { ...outcome.face, scripts: source.row.scripts }

    // A STORE THAT CANNOT BE OPENED DEGRADES TO THE PRE-16.5 MODEL RATHER THAN
    // REFUSING (orchestrator ruling, 2026-09-03). NEITHER OBVIOUS ANSWER WAS RIGHT.
    //
    // Refusing contradicts Story 16.2's locked contract — *"given storage that
    // cannot be opened or written, the designer still works and says what is
    // degraded"* — and would mean a private window could add no font at all.
    // But 16.2's degradation taken LITERALLY fails too: under the old model the
    // store was a convenience because the pick embedded immediately, while under
    // embed-on-use it is load-bearing, so "install anyway, keep nothing" stores
    // nothing and nothing can ever be used. That is the dead end D-16.R.46 Q4
    // forbids.
    //
    // SO THIS BROWSER GETS THE OLD MODEL: the pick puts the font straight into
    // the document, exactly as it did before this story. It is also the honest
    // description of the mode — with nowhere to keep a face there is nothing to
    // install, and embedding at once is the only way a web font can be used
    // here. The machine store stays a convenience and never becomes a
    // dependency. Story 16.6 deleted the note that used to say this on screen
    // (deliberate reversal, D-16.R.82) — nothing is said, here or anywhere.
    //
    // The property is NOT committed, because this is still the second arm of the
    // fork: Story 8.6's *"carry this typeface"* and *"draw this box with it"* stay
    // two decisions, and the degradation may not quietly fuse them.
    if (!(await fontStore.current)) {
      const rejected = await dispatchEmbed(face, responseGeneration, selectionKey, announce)
      if (rejected !== undefined) return rejected
      return undefined
    }

    // OTHERWISE THE STORE WRITE IS THE WHOLE ACT, AND THAT INVERTS 16.2's RULING.
    //
    // 16.2 put this write AFTER the embed and ruled a quota refusal a
    // DEGRADATION rather than a failed pick, on the ground that *"the face was
    // fetched, the terms were admitted and the document has it"* — there was
    // something left to degrade FROM. Here there is not. No command follows this
    // line, no document has the face, and if the write is refused the author has
    // nothing at all. So it is a REFUSAL, stated at the control they acted on.
    //
    // The ordering question 16.2 answered has dissolved rather than flipped:
    // with one act there is no second act to order it against.
    const kept = await keepOnThisMachine(face)
    if (kept !== undefined) return refuse(storeWriteRefusal(face.family, kept))
    return undefined
  }

  /**
   * FIRST USE — THE MOMENT A FONT STARTS TRAVELLING INSIDE THE TEMPLATE.
   *
   * Story 8.6's guarantee is untouched: send the file to a colleague and the
   * pages come out identical, because a font still travels inside the `.folio`
   * (CAP-2/AD-8). Only the moment it starts travelling moves — from the pick to
   * the first time something in the template is actually set in the family.
   *
   * TWO COMMANDS, NEVER ONE, AND THE ORDER IS FORCED BY THE ENGINE.
   * `canvas.fontFamilies` is the closed set `style.fontFamily` may name, so
   * `updateComponentProperties` is refused unless the chain is already declared.
   * This half sends the embed; the family control commits the property after it
   * returns, and only if it returns nothing. Two commands means two unambiguous
   * undo entries — "carry this typeface" and "draw this box with it" are two
   * decisions and fusing them would make one undo ambiguous. There is no
   * compound-command mechanism in this product and Story 8.6's refused fusion is
   * not reopened.
   *
   * THE BYTES COME FROM THIS MACHINE FIRST, AND A MISS FALLS THROUGH TO THE
   * FETCH RATHER THAN REFUSING (Story 16.2's contract, restored by orchestrator
   * ruling 2026-09-03 after this story briefly removed it).
   *
   * A `local` row is read from the release's own content-addressed assets behind
   * the service worker; a `stored` row is read out of the machine font store. A
   * stored read can MISS: the entry may have been dropped as unsound between the
   * listing and this read — the store self-heals by dropping, silently — or
   * removed in another tab. 16.2's matrix is explicit that this is SELF-HEALING,
   * *"entry treated as absent and dropped; refetch on next pick"*, and that is a
   * shipped contract this story may not amend. Refusing instead would convert a
   * path that repairs itself into a permanent local failure the author would
   * have no way to clear, since Story 16.6 removed the only control that ever
   * touched the store.
   *
   * THE FACE IS THEN WRITTEN BACK, which is the half that makes it self-healing
   * rather than merely survivable. That write follows an embed, so the document
   * already has the face; a write-back failure is silent (Story 16.6) rather
   * than stated, same as every other store degradation.
   */
  const embedInstalledFamily = async (source: FamilySource, responseGeneration: number, selectionKey: string): Promise<string | undefined> => {
    const refuse = (message: string) => refuseFontChain(message, responseGeneration, selectionKey, 'panel')
    // EVERY EXIT GOES THROUGH THE REFUSAL SURFACE, INCLUDING THE EARLY ONES.
    // These two returned a sentence that nothing painted and nothing read: the
    // family control discards the value, so clicking an installed family while
    // the designer was busy produced no command, no property commit and no
    // message anywhere. A guard the author cannot see is a guard that looks
    // like a broken control.
    if (!engine) return refuse('This designer has no engine to send the change to.')
    if (fileBusy || fontChainBusyRef.current) return refuse(`${source.family} was not used: the designer was busy with another change. Try it again.`)
    holdFontChain(true)
    try {
      let embedded: ResolvedFace
      // Set when the bytes had to come over the network because the stored entry
      // was gone. They are worth writing back; a face read successfully OUT of
      // the store is deliberately not rewritten to it, because the record is
      // already there byte for byte.
      let refetched = false
      if (source.tier === 'stored') {
        const read = await (await fontStore.current)?.get(source.record.key)
        if (read?.ok && read.value !== undefined) {
          const held = read.value
          embedded = { family: held.family, style: held.style, licence: held.licence, licenceText: held.licenceText, copyright: held.copyright, source: held.source, mediaType: held.mediaType, bytes: held.bytes, scripts: held.scripts }
        } else {
          const outcome = await fetchWebFamily(source.family)
          if (!outcome.ok) return refuse(outcome.reason)
          if (outcome.face.layoutDivergence !== undefined) console.info(outcome.face.layoutDivergence)
          embedded = { ...outcome.face, scripts: source.record.scripts }
          refetched = true
        }
      } else if (source.tier === 'local') {
        const face = source.face
        let bytes: ArrayBuffer
        try {
          const response = await fetch(face.url)
          if (!response.ok) throw new Error(`the bundled face responded ${response.status}`)
          bytes = await response.arrayBuffer()
        } catch (error) {
          return refuse(`${face.family} could not be read from the offline bundle: ${error instanceof Error ? error.message : String(error)}`)
        }
        embedded = { family: face.family, style: face.style, licence: face.licence, licenceText: face.licenceText, copyright: face.copyright, source: face.source, mediaType: 'font/ttf', bytes, scripts: face.scripts }
      } else {
        // UNREACHABLE BY EITHER CALLER, AND NAMED RATHER THAN SKIPPED. The family
        // control routes a `web` row to `installFamily`, so arriving here with one
        // is a routing defect; answering it by fetching would restore the fused
        // pick this story exists to split.
        return refuse(`${source.family} is not on this machine yet, so it cannot be put into this document. Install it first.`)
      }
      // THE ANNOUNCER IS `'caller'` SO THE PANEL IS PAINTED ONCE, WITH THE WHOLE
      // SENTENCE. `sendFontChain` would otherwise write the engine's bare
      // refusal and the disclosure below would have to overwrite it — two
      // messages for one event, in whichever order React settled.
      setFontChainError(undefined)
      const rejected = await dispatchEmbed(embedded, responseGeneration, selectionKey, 'caller')
      if (rejected !== undefined) return refuse(lateEmbedRefusal(embedded.family, rejected))
      // THE STORE HEALS AFTER THE EMBED. The document already has the face, so
      // a write-back failure here is silent (Story 16.6) rather than stated —
      // there is nothing left for the author to act on.
      if (refetched) await keepOnThisMachine(embedded)
      // THE PROPERTY COMMIT IS AUTHORISED HERE, NOT IN THE FAMILY CONTROL, AND
      // ONLY IF THE DOCUMENT AND THE SELECTION ARE STILL THE ONES THE AUTHOR
      // ACTED IN.
      //
      // The embed is a chain of awaits — a store read or a whole refetch — and
      // an Open, a Start blank or an undo can land in the middle of it. The
      // family control cannot see that: its `documentGeneration` and its `ids`
      // are the render's, frozen in the closure, so comparing them to
      // themselves would always agree. The live values are `documentGeneration`
      // and `selectedRef` HERE, which is why the check belongs here.
      //
      // AND IT MATTERS BECAUSE `applyProperties` SENDS FIRST AND GUARDS AFTER:
      // it dispatches the command unconditionally and only declines to install
      // the RESULT against a moved document. So a stale commit is not a dropped
      // response — it is `updateComponentProperties` reaching the engine with
      // the previous document's element ids.
      //
      // A NON-`undefined` RETURN SUPPRESSES THE COMMIT, and `refuse` declines to
      // paint anything whose generation has moved, so this is silent by
      // construction — which is right: the author is looking at something else.
      if (documentGeneration.current !== responseGeneration || selectedRef.current.join(',') !== selectionKey) {
        return refuse(`${embedded.family} was embedded, but no component was set in it: the document or the selection moved while the face was being written.`)
      }
      return undefined
    } finally {
      if (documentGeneration.current === responseGeneration) holdFontChain(false)
    }
  }

  /**
   * KEEPING THE FACE, AND FAILING TO KEEP IT IS NOW A REFUSAL RATHER THAN A
   * DEGRADATION (Story 16.5 — see `installFamily` for why the ruling inverted).
   *
   * IT RETURNS THE REASON AND RENDERS NOTHING, because its two callers owe the
   * author two different treatments. `installFamily` turns a non-`undefined`
   * reason into a refusal via `storeWriteRefusal`, because the write IS the
   * whole act there. `embedInstalledFamily`'s write-back after a refetch is the
   * one place under embed-on-use where the document already has the face, so it
   * discards the reason and stays silent (Story 16.6) rather than stating it.
   * `undefined` means the face is on this machine.
   *
   * Everything `embedFontFamily` requires travels into the store WITH the bytes
   * — licence identifier, licence text and copyright — because a face offered
   * from the store must be embeddable without a network, and the command
   * refuses without all three. A store that kept the bytes and dropped the
   * terms would put a document its own parser refuses one step away.
   *
   * The key is computed HERE, from the bytes, by `crypto.subtle`. It is the
   * store's own address and it agrees with the one Go derives for the same
   * bytes — `src/font-store.test.ts` asserts that agreement against a digest Go
   * itself produced, so the two addressings cannot drift.
   */
  const keepOnThisMachine = async (face: ResolvedFace): Promise<string | undefined> => {
    const store = await fontStore.current
    if (!store) return 'this browser is not letting the designer keep typefaces on this machine'
    let key: string
    try {
      key = await storedFaceKey(face.bytes)
    } catch (error) {
      return componentDiagnostic(error)
    }
    const written = await store.put({ ...face, key, byteLength: face.bytes.byteLength, fetchedAt: new Date().toISOString().slice(0, 10), bytes: face.bytes })
    if (!written.ok) return written.reason
    await refreshStoredFaces()
    return undefined
  }

  // Story 5.13: choosing a local image is a two-step boundary crossing — the
  // browser reads bytes (imageFileAccess), then sends ONE opaque committed
  // command carrying those bytes and the browser's own declared media type
  // (AC1). This function does not hash, sniff, or decide legality; Go alone
  // does, through the ordinary command/diagnostic path every other mutation
  // already uses.
  const applyImageAsset = async (id: string) => {
    if (!engine || !imageFileAccess || fileBusy || assetBusy) return
    // Finding 4 (review of 2026-08-29): this closure spans the two longest
    // awaits in the application — an OS file dialog, then an engine command
    // carrying up to megabytes — and element ids are reused across
    // documents. Capture the generation/revision BEFORE the picker await,
    // matching bindPickedPath's shape exactly, so a result that resolves
    // after an Open/Start-blank/undo/newer-command document replacement is
    // never installed over the authoritative snapshot (AC1's named red
    // proof: "a command result installed after document replacement").
    const requestGeneration = documentGeneration.current
    const priorRevision = snapshotRef.current?.revision
    setAssetError(undefined)
    setAssetBusy(true)
    try {
      const picked = await imageFileAccess.openImage()
      const result = await engine.request('command', setComponentAssetCommand(id, picked.mediaType, picked.bytes))
      if (documentGeneration.current === requestGeneration && snapshotRef.current?.revision === priorRevision) {
        if (result.snapshot.revision !== priorRevision) invalidatePreview()
        setCurrentSnapshot(result.snapshot)
      }
    } catch (error) {
      if (!isFileAccessCancelled(error) && documentGeneration.current === requestGeneration && snapshotRef.current?.revision === priorRevision) {
        setAssetError({ id, message: componentDiagnostic(error) })
      }
    } finally {
      if (documentGeneration.current === requestGeneration) setAssetBusy(false)
    }
  }

  const setHistoryAvailability = (next: EngineSnapshot | undefined) => { setUndoAvailable(next?.canUndo === true); setRedoAvailable(next?.canRedo === true) }
  const setCurrentSnapshot = (next: EngineSnapshot | undefined, keepNewerDraft = false, clearDocumentInteraction = false) => { const previousCanvas = snapshotRef.current?.canvas; snapshotRef.current = next; setSnapshot(next); setHistoryAvailability(next); if (clearDocumentInteraction) { // Story 5: a break selection names a page; once pages were added or removed that index may be another page's break.
    if (!previousCanvas || !next?.canvas || pageCountOf(previousCanvas) !== pageCountOf(next.canvas)) { setSectionBreakSelected(undefined); setPendingBreakFocus(undefined) }
    documentGeneration.current++; tableEditorSession.current++; setTableEditorEdits(0); setTableEditorDiscarded(undefined); setTableEditorDiscarding(false); setDocumentGenerationValue(documentGeneration.current); setSelected([]); setSelectedPage(undefined); setCurrentPage(0); setPageDeleteConfirm(undefined); setColumnSelection(undefined); setBindingError(undefined); setBindingBusy(false); setTableEditor(undefined); setTableEditorError(undefined); setFontBrowserOpen(false); setAssetError(undefined); setAssetBusy(false); setFontChainError(undefined); holdFontChain(false); clearInteraction() }; if (next?.canvas) { setPreset(next.canvas.preset); setOrientation(next.canvas.orientation); if (!keepNewerDraft) setDraft(draftFor(next.canvas)) } }
  const updateDraft = (key: keyof Draft, value: string) => { draftGeneration.current++; setDraft((current) => ({ ...current, [key]: value })) }
  const announceFailure = (message: string) => { setFileStatus(undefined); setFileError(message) }
  // Retire a settled status. Re-armed on every change to either input, so a new
  // outcome gets a full window rather than the tail of the previous one's, and
  // a status that arrives while busy is left alone until the bar is released.
  useEffect(() => {
    if (!fileStatus || fileBusy) return
    const handle = setTimeout(() => setFileStatus(undefined), SETTLED_FILE_STATUS_MS)
    return () => clearTimeout(handle)
  }, [fileStatus, fileBusy])

  // THE BOUNDARY'S OWN SENTENCE WINS OVER THIS FUNCTION'S FALLBACK.
  //
  // A `FileAccessFailure` already reads "<what was being done>: <what the
  // browser called the throw>" (`fileFailureFor` in file/file-access.ts), so
  // replacing it with the generic sentence here would re-erase, one layer up,
  // exactly the evidence that layer was changed to keep. Anything else — an
  // engine rejection, a serialize that produced no bytes, an abort deadline —
  // is not a file-boundary failure and gets the plain sentence it always got.
  const fileFailureSentence = (error: unknown, fallback: string) => error instanceof FileAccessFailure && error.message ? error.message : fallback
  const revokeSampleLoad = () => { sampleLoadGeneration.current++; setSampleBusy(false) }
  const clearSampleData = () => {
    revokeSampleLoad()
    sampleDataRef.current = undefined; setSampleData(undefined); setSampleError(undefined); setBindingError(undefined)
    clearPreviewParameters()
    // Clearing a previously accepted sample is itself a Preview input change.
    invalidatePreview(true)
    // STORY 13.4 — AND IT NOW LANDS ON A NO-DATA PREVIEW, not on an empty
    // 'idle' screen, WITH NOTHING ADDED HERE. Both callers (open, startBlank)
    // already re-render on their own tail — `if (modeRef.current ===
    // 'preview') { … renderPreview() }` — and what used to make that re-render
    // land on 'idle' was renderPreview's sample gate, which is gone. A
    // schedulePreview() call here would be a guard that cannot fail: measured,
    // deleting it reds nothing, because those two tails already cover it.
  }
  // THE ONE ACCEPT PATH FOR SAMPLE JSON, shared by Load sample JSON and by an
  // example opened from the startup dialog, so an example's sample is available
  // to binding exactly as if the author had opened that file. `stillCurrent` is
  // asked between parsing and installing, because a picker's answer can arrive
  // after a newer load has taken its place. Returns whether it installed.
  const acceptSample = (name: string, bytes: ArrayBuffer, stillCurrent: () => boolean = () => true): boolean => {
    const accepted = acceptSampleData(name, bytes)
    if (!stillCurrent()) return false
    installAcceptedSample(accepted)
    return true
  }
  // The install half, for a caller that has to parse BEFORE it replaces
  // anything: an example's sample is checked before its template is loaded.
  const installAcceptedSample = (accepted: SampleData) => {
    // Replacement is atomic: only a fully accepted raw file and its bounded
    // projection can replace the prior local sample.
    sampleDataRef.current = accepted; setSampleData(accepted); setBindingError(undefined)
    clearPreviewParameters()
    invalidatePreview()
    schedulePreview()
  }
  const loadSample = async () => {
    if (!sampleFileAccess || sampleBusy) return
    const authority = ++sampleLoadGeneration.current
    setSampleBusy(true); setSampleError(undefined)
    try {
      const selected = await sampleFileAccess.openSample()
      acceptSample(selected.name, selected.bytes, () => authority === sampleLoadGeneration.current)
    } catch (error) {
      if (authority === sampleLoadGeneration.current && !isFileAccessCancelled(error)) setSampleError(error instanceof Error ? error.message : 'Could not read local sample data')
    } finally { if (authority === sampleLoadGeneration.current) setSampleBusy(false) }
  }
  const open = async () => {
    if (!engine || !fileAccess || fileBusy) return
    revokeSampleLoad()
    invalidatePreview(true)
    clearPreviewParameters(true)
    setFileBusy(true); setFileError(undefined); setFileStatus('Opening local file…')
    try {
      await installPickedFile(await fileAccess.open())
    } catch (error) {
      if (isFileAccessCancelled(error)) setFileStatus(undefined)
      else announceFailure(fileFailureSentence(error, 'Could not open local file'))
    } finally { setFileBusy(false) }
  }
  // What Open does with a file the picker returned, shared by the document bar
  // and the startup dialog's Open existing file… (story 4). The mode is left as
  // it is; a Preview re-renders the new document.
  const installPickedFile = async (opened: LocalFile) => {
    const installed = await installOpenedDocument(opened.bytes, opened.name, opened.target)
    setSavedRevision(installed.inputWasCanonical ? installed.canonicalRevision : undefined)
    setFileStatus(installed.inputWasCanonical ? `Opened local file ${opened.name}` : `Opened local file ${opened.name}; canonical local changes need saving`)
    if (modeRef.current === 'preview') { void loadParameterReferences(); void renderPreview() }
  }
  // THE ONE DOCUMENT-REPLACEMENT PATH FOR TEMPLATE BYTES, shared by Open and by
  // an example opened from the startup dialog: load, canonical serialize, a new
  // document identity, the snapshot installed with interaction cleared, the
  // prior sample dropped, and the title and target taken from the caller. The
  // caller decides what the result means for `savedRevision` and the status.
  const installOpenedDocument = async (source: ArrayBuffer, name: string, fileTarget: FileTarget | undefined) => {
    const client = engine
    if (!client) throw new Error('The local engine is not ready')
    const loaded = await engineFileStep((signal) => client.request('load', source, signal))
    const canonical = await engineFileStep((signal) => client.request('serialize', undefined, signal))
    if (!canonical.bytes) throw new Error('Local file could not be serialized')
    const inputWasCanonical = equalBytes(source, canonical.bytes)
    installDocumentIdentity()
    setCurrentSnapshot(loaded.snapshot, false, true)
    setBaselineRevision(loaded.snapshot.revision)
    clearSampleData()
    setTitle(name)
    setTarget(fileTarget)
    return { inputWasCanonical, canonicalRevision: canonical.snapshot.revision }
  }

  // STORY 4. New… on a document with real edits warns before the dialog opens
  // (CAP-6, owner renegotiation); no choice inside the dialog asks again.
  const unsavedEdits = () => {
    const current = snapshotRef.current
    return current !== undefined && baselineRevision !== undefined && current.revision !== baselineRevision
  }
  const chooseStartup = (id: string) => {
    if (startupBusy !== undefined) return
    setStartupError(undefined)
    if (id === BLANK_CHOICE_ID) {
      if (startupOrigin === 'launch') { setStartupOpen(false); return }
      void startBlankFromStartup()
      return
    }
    void openExample(id)
  }
  // New…: on real edits, the warning first; otherwise straight to the dialog.
  const openNewDialog = () => {
    if (!engine || !blankBytes || fileBusy) return
    if (unsavedEdits()) { setUnsavedWarningOpen(true); return }
    openStartupFromNew()
  }
  const openStartupFromNew = () => { setStartupOrigin('new'); setStartupSelected(BLANK_CHOICE_ID); setStartupError(undefined); setStartupOpen(true) }
  // Keep editing (or Escape): nothing opens and nothing is requested.
  const keepEditing = () => setUnsavedWarningOpen(false)
  // Discard only agrees to replacement; the dialog's choice performs it, and
  // Cancel there still leaves the document as it is.
  const discardForNew = () => { setUnsavedWarningOpen(false); openStartupFromNew() }
  // Cancel and Escape: close, with nothing requested from the engine.
  const cancelStartup = () => {
    if (startupBusy !== undefined) return
    setStartupError(undefined); setStartupOpen(false)
  }
  // Blank from a reopened dialog is exactly the old document-bar Start blank.
  const startBlankFromStartup = async () => {
    setStartupBusy('Blank')
    try {
      if (await startBlank()) setStartupOpen(false)
      else setStartupError('Could not start a blank local template')
    } finally { setStartupBusy(undefined) }
  }
  // Open existing file… (CAP-8): the bar's picker and install path. The picker
  // is called before any await, inside the click's activation. A cancelled
  // picker changes nothing and says nothing; a failure stays in the footer.
  const requestStartupFile = () => {
    if (startupBusy !== undefined) return
    setStartupError(undefined)
    void openFileFromStartup()
  }
  const openFileFromStartup = async () => {
    if (!engine || !fileAccess || fileBusy) return
    let opened: LocalFile
    try { opened = await fileAccess.open() } catch (error) {
      if (!isFileAccessCancelled(error)) setStartupError(fileFailureSentence(error, 'Could not open local file'))
      return
    }
    setStartupBusy(opened.name)
    revokeSampleLoad()
    invalidatePreview(true)
    clearPreviewParameters(true)
    setFileBusy(true); setFileError(undefined); setFileStatus('Opening local file…')
    try {
      await installPickedFile(opened)
      setStartupOpen(false)
    } catch (error) {
      setFileStatus(undefined)
      setStartupError(fileFailureSentence(error, 'Could not open local file'))
    } finally { setFileBusy(false); setStartupBusy(undefined) }
  }
  // AN EXAMPLE FROM THE STARTUP DIALOG. At launch Blank (like Cancel and Escape)
  // is no request at all: the starter the engine already holds stays, at
  // revision 1. From New…, Blank loads the starter (story 4). An example
  // fetches BOTH files before anything is sent, so a failed fetch leaves the
  // document untouched and the dialog open with the failure in its footer.
  // Opened, it is an ordinary unsaved document with no file target, titled with
  // the example's name, and it enters Preview once — rendered from its sample.
  const openExample = async (id: string) => {
    if (startupBusy !== undefined) return
    const card = startupCards.find((entry) => entry.id === id)
    const asset = examples?.find((example) => example.id === id)
    if (!engine || !card || !asset || fileBusy) return
    setStartupBusy(card.name); setStartupError(undefined)
    try {
      const [template, sampleBytes] = await Promise.all([fetchExampleFile(asset.template), fetchExampleFile(asset.sample)])
      // Parsed before any engine request or state change, so a sample that is
      // refused fails with nothing replaced.
      const accepted = acceptSampleData(card.sample ?? `${card.id}.sample.json`, sampleBytes)
      revokeSampleLoad()
      invalidatePreview(true)
      clearPreviewParameters(true)
      setFileBusy(true); setFileError(undefined); setFileStatus(`Opening example ${card.name}…`)
      try {
        await installOpenedDocument(template, card.name, undefined)
        installAcceptedSample(accepted)
        setSavedRevision(undefined)
        setFileStatus(`Opened example ${card.name}`)
      } catch (error) {
        setFileStatus(undefined)
        throw error
      } finally { setFileBusy(false) }
      setStartupOpen(false)
      enterPreview()
    } catch (error) {
      setStartupError(`Could not open ${card.name}: ${error instanceof Error && error.message ? error.message : componentDiagnostic(error)}`)
    } finally { setStartupBusy(undefined) }
  }

  const save = async (saveAs: boolean) => {
    if (!engine || !fileAccess || saveInFlight.current || exportInFlight.current || sampleSaveInFlight.current) return
    saveInFlight.current = true; setFileBusy(true); setFileError(undefined); setFileStatus(saveAs ? 'Preparing Save As…' : 'Preparing local save…')
    try {
      // Must run inside the gesture before awaiting the worker: the native
      // picker is activation-gated. Cancellation leaves every session field as-is.
      const acquired = await fileAccess.acquireSaveTarget({ suggestedName: title, currentTarget: target, saveAs, format: folioFileFormat })
      setFileStatus('Saving local file…')
      const serialized = await engineFileStep((signal) => engine.request('serialize', undefined, signal))
      if (!serialized.bytes) throw new Error('Local file could not be serialized')
      const saved = await fileAccess.writeSave(acquired, { bytes: serialized.bytes })
      setTitle(saved.name)
      setTarget(saved.target)
      // Completion establishes only the written revision. It must never repaint
      // an older snapshot over a newer engine commit or call that newer state clean.
      const wroteCurrentRevision = snapshotRef.current?.revision === serialized.snapshot.revision
      if (wroteCurrentRevision) { setSavedRevision(serialized.snapshot.revision); setBaselineRevision(serialized.snapshot.revision) }
      setFileStatus(wroteCurrentRevision ? (saved.target ? `Saved locally as ${saved.name}` : `Downloaded local file ${saved.name}`) : `Saved revision ${serialized.snapshot.revision}; newer local changes need saving`)
    } catch (error) {
      if (isFileAccessCancelled(error)) setFileStatus(undefined)
      else announceFailure(fileFailureSentence(error, 'Could not save local file'))
    } finally { saveInFlight.current = false; setFileBusy(false) }
  }

  // Reached only through a reopened startup dialog's Blank (story 4). Resolves
  // whether the starter was installed.
  const startBlank = async (): Promise<boolean> => {
    if (!engine || !blankBytes || fileBusy) return false
    revokeSampleLoad()
    invalidatePreview(true)
    clearPreviewParameters(true)
    setFileBusy(true); setFileError(undefined); setFileStatus('Starting blank local template…')
    try {
      const loaded = await engineFileStep((signal) => engine.request('load', blankBytes, signal))
      installDocumentIdentity()
      setCurrentSnapshot(loaded.snapshot, false, true)
      setBaselineRevision(loaded.snapshot.revision)
      clearSampleData()
      setTitle('Untitled template'); setTarget(undefined); setSavedRevision(undefined)
      setFileStatus('Started an unnamed local template')
      if (modeRef.current === 'preview') { void loadParameterReferences(); void renderPreview() }
      return true
    } catch { announceFailure('Could not start a blank local template'); return false
    } finally { setFileBusy(false) }
  }

  const applyHistory = async (operation: 'undo' | 'redo') => {
    if (!engine || fileBusy || !(operation === 'undo' ? undoAvailable : redoAvailable)) return
    setCommitError(undefined)
    try {
      const result = await engine.request(operation)
      invalidatePreview(); setCurrentSnapshot(result.snapshot, false, true)
      // Undo/Redo installs a different canonical revision while Preview stays
      // open. Re-query the Go projection instead of keeping fields attributed
      // to the revision we just left.
      if (modeRef.current === 'preview') void loadParameterReferences()
    } catch (error) {
      const received = error as { code?: string }
      if (received.code === 'UNDO_UNAVAILABLE') setUndoAvailable(false)
      else if (received.code === 'REDO_UNAVAILABLE') setRedoAvailable(false)
      else setCommitError(componentDiagnostic(error))
    }
  }
  const returnWithOptionalSelection = (location?: DiagnosticLocation, announceUnavailable = false) => {
    const id = location?.elementId
    const current = snapshotRef.current?.canvas?.components
    clearInteraction()
    if (id && current?.some((component) => component.id === id)) { revokeTableEditor(); setSelected([id]); setSelectedPage(undefined); const located = current.find((component) => component.id === id); if (located?.band === 'content') setCurrentPage(componentPage(located)); setColumnSelection(undefined); setLocateStatus(`Selected ${id} in Design.`) }
    else if (id && announceUnavailable) setLocateStatus('Locate unavailable: the authoritative element is no longer present.')
    else setLocateStatus(undefined)
    returnToDesign()
    setTimeout(() => canvasRegionRef.current?.focus(), 0)
  }
  const admittedPreview = (candidate: PreviewRecord | undefined): candidate is PreviewRecord => Boolean(candidate && previewStatus === 'current' && modeRef.current === 'preview' && previewRef.current === candidate && candidate.token === previewToken.current && candidate.generation === previewGeneration.current && candidate.revision === snapshotRef.current?.revision && candidate.identity === previewRef.current.identity)
  const activeFailure = (candidate: PreviewFailureRecord | undefined): candidate is PreviewFailureRecord => Boolean(candidate && previewError === candidate && ['error', 'stale'].includes(previewStatus) && modeRef.current === 'preview' && candidate.token === previewToken.current && candidate.generation === previewGeneration.current && candidate.revision === snapshotRef.current?.revision)
  const locateDiagnostic = (candidate: PreviewRecord, location: DiagnosticLocation) => { if (admittedPreview(candidate)) returnWithOptionalSelection(location, true) }
  const retryFromFailure = (failure: PreviewFailureRecord) => {
    if (!activeFailure(failure) || retryingFailure.current === failure.token) return
    retryingFailure.current = failure.token
    // Bypass only the retained same-identity PDF shortcut. The scheduler, FIFO
    // worker, serializer, and PDF.js admission path remain unchanged.
    renderPreview(true)
  }
  const returnFromFailure = (failure: PreviewFailureRecord) => { if (activeFailure(failure)) returnWithOptionalSelection(failure.error, true) }

  // STORY 13.1 — THE PREVIEW KEEPS THE PDF.
  //
  // `preview.bytes` is the buffer the engine returned and the buffer the
  // displayed digest covers (`installPreview` stores `result.bytes.slice(0)`;
  // the viewer hands PDF.js its own `bytes.slice(0)` so the rasterizer never
  // neuters this one). It is written VERBATIM: nothing here re-renders,
  // re-serializes, or decodes it to text and back, and the engine receives no
  // request at all across the press.
  //
  // ⚠ `saveAs: true` AND NO `currentTarget`, AND THAT IS A SAFETY PROPERTY, NOT
  // A STYLE. `FileSystemAccess.acquireSaveTarget` reuses a retained handle with
  // no picker when `!saveAs && currentTarget?.kind === 'in-place'`, so a PDF
  // save that passed the template's target would overwrite the author's
  // `.folio` with PDF bytes, irreversibly. A PDF save is always a fresh target.
  //
  // ⚠ AND NOTHING FROM THE RESULT IS KEPT. `title`, `target` and
  // `savedRevision` describe the TEMPLATE the author is editing; a PDF is an
  // output taken off it, so writing any of them here would rename the document
  // after a file that cannot be reopened, or call an unsaved template clean.
  //
  // ONE NAME FOR THE CONTROL, used by the button and by its own reason. The
  // reason used to say "Save PDF is unavailable" under a button labelled `Save
  // stale PDF`, and to call the export "another local file action" while the
  // action in progress WAS the export.
  // STORY 13.4 — THE EXPORT NAMES THE STAND-IN, FOR A DIFFERENT REASON THAN
  // STALENESS. Saving is the one place these bytes leave the machine: the file
  // outlives the session, and nothing inside a PDF says its values were
  // fabricated. The two qualifiers are independent and both can apply at once,
  // so they compose rather than choosing between them.
  const pdfExportStale = Boolean(preview) && !admittedPreview(preview)
  const pdfExportQualifier = `${pdfExportStale ? ' stale' : ''}${preview?.standIn ? ' no-data' : ''}`
  const pdfExportLabel = `Save${pdfExportQualifier} PDF`
  const pdfExportUnavailable = !fileAccess ? `${pdfExportLabel} is unavailable: this browser exposes no local file access.`
    : !preview ? `${pdfExportLabel} is unavailable: no local PDF has been rendered yet.`
    : fileBusy ? `${pdfExportLabel} is unavailable while a local file action is in progress.`
    : undefined
  // The staleness the CONTROL states and the staleness the COMPLETION states are
  // one predicate, read once per render and once per press. `admittedPreview` is
  // the existing definition of "current, engine-authoritative, and admitted by
  // PDF.js"; minting a second one here could drift from the status line's.
  const exportPreviewPdf = async () => {
    // BOTH LATCHES, not just this one. `fileBusy` is the shared interlock that
    // keeps two local writes off the wire at once, and it stays — but until now
    // only the rendered `disabled` attributes held the template save and the PDF
    // save apart, which is an ordering property of React's flush rather than an
    // invariant of these two functions. Each refuses while the other is live.
    if (!fileAccess || !preview || exportInFlight.current || saveInFlight.current || sampleSaveInFlight.current) return
    const record = preview
    // Read off the record BEFORE the staleness question: `admittedPreview` is a
    // type predicate, so a `!`-negated alias narrows `record` to `never` in the
    // stale branch and the compiler loses the very fields the status needs.
    const pdfBytes = record.bytes
    const pdfRevision = record.revision
    const stale = !admittedPreview(record)
    // Read off the SAME record the bytes came from, for the reason the
    // staleness comment above gives: a second read of `preview` could describe
    // a different PDF than the one being written.
    const qualifier = `${stale ? ' stale' : ''}${record.standIn ? ' no-data' : ''}`
    exportInFlight.current = true; setFileBusy(true); setFileError(undefined); setFileStatus(`Preparing${qualifier} PDF save…`)
    try {
      // Inside the gesture, before any await that is not the picker itself: the
      // native picker is gated on the click's transient user activation.
      const acquired = await fileAccess.acquireSaveTarget({ suggestedName: title, saveAs: true, format: pdfFileFormat })
      const saved = await fileAccess.writeSave(acquired, { bytes: pdfBytes })
      const revision = `${qualifier.trim() ? `${qualifier.trim()} ` : ''}revision ${pdfRevision}`
      setFileStatus(saved.target ? `Saved PDF of ${revision} as ${saved.name}` : `Downloaded PDF of ${revision} as ${saved.name}`)
    } catch (error) {
      if (isFileAccessCancelled(error)) setFileStatus(undefined)
      else announceFailure(fileFailureSentence(error, 'Could not save the preview PDF'))
    } finally { exportInFlight.current = false; setFileBusy(false) }
  }

  // STORY 5 (startup templates), CAP-7 — SAVE SAMPLE DATA.
  //
  // THE BYTES ARE THE ACCEPTED BYTES, AND NOTHING HERE TOUCHES THEM.
  // `acceptSampleData` already kept `bytes.slice(0)` — the author's own file,
  // byte for byte — and the display tree beside it is a BOUNDED PROJECTION: a
  // deep or wide document is truncated for the panel. Re-serializing that
  // projection would hand the author a file that is not the one their Preview
  // rendered from, silently, and only for the large documents where they are
  // least likely to notice. So the buffer goes out untouched: no parse, no
  // re-encode, no reformat.
  //
  // IT IS AN OUTPUT SAVE, NOT A DOCUMENT SAVE. It reaches no engine operation,
  // moves no revision, and never touches `savedRevision`, the unsaved-changes
  // baseline, the title, or the retained `.folio` target — a sample is not part
  // of the template (format rule) and saving one must not make the document look
  // saved or unsaved. For the same reason NO TARGET IS REMEMBERED: `saveAs:
  // true` every time, so this can never overwrite a file the author last picked
  // for something else.
  //
  // WHERE THE SAMPLE CAME FROM IS NOT ASKED. An example's bundled sample and one
  // the author opened with Load sample JSON are the same `SampleData` by the
  // time they reach here, and the story is explicit that both save alike.
  const saveSampleData = async () => {
    // All three latches, for the reason 13.1 gives at the export: the rendered
    // `disabled` attribute is an ordering property of React's flush, not an
    // invariant of these functions.
    if (!fileAccess || !sampleData || sampleSaveInFlight.current || saveInFlight.current || exportInFlight.current) return
    // Read the record ONCE. A second read of `sampleData` could describe a
    // different sample than the one whose bytes are being written.
    const record = sampleData
    sampleSaveInFlight.current = true; setFileBusy(true); setFileError(undefined); setFileStatus('Preparing sample data save…')
    try {
      // Inside the gesture, before any await that is not the picker itself: the
      // native picker is gated on the click's transient user activation.
      const acquired = await fileAccess.acquireSaveTarget({ suggestedName: record.name, saveAs: true, format: jsonSampleFileFormat })
      const saved = await fileAccess.writeSave(acquired, { bytes: record.bytes })
      setFileStatus(saved.target ? `Saved sample data as ${saved.name}` : `Downloaded sample data ${saved.name}`)
    } catch (error) {
      if (isFileAccessCancelled(error)) setFileStatus(undefined)
      else announceFailure(fileFailureSentence(error, 'Could not save the sample data'))
    } finally { sampleSaveInFlight.current = false; setFileBusy(false) }
  }

  useEffect(() => {
    const shortcut = (event: KeyboardEvent) => {
      // THE STARTUP DIALOG OWNS THE KEYBOARD WHILE IT IS OPEN — every shortcut,
      // Cmd/Ctrl+S included. There is nothing behind it the author has chosen yet.
      // Save is still claimed from the browser, or it opens Save Page instead.
      const mac = isMacPlatform()
      const modifier = primaryModifier(event, mac)
      if (startupOpen || unsavedWarningOpen) { if (modifier && event.key.toLowerCase() === 's') event.preventDefault(); return }
      const editing = isEditableTarget(event.target) || event.isComposing
      if (modifier && event.key.toLowerCase() === 's' && engine && fileAccess && !fileBusy) {
        event.preventDefault()
        void save(false)
        return
      }
      // STORY 14.7b / DW-368 — THE GUARD LINE, AND IT NOW ASKS A SECOND
      // QUESTION. `isEditableTarget` answers "is the TARGET editable?"; that is
      // the wrong question when a modal is on screen, because the author can be
      // focused on a BUTTON inside it and every shortcut below then fires
      // against the document behind the dialog. Measured: the arrow keys sent
      // `moveComponent` and Cmd+D sent `duplicateComponent` against THE VERY
      // TABLE the dialog had open — both gate on a single selection, and
      // openTableEditor requires that single selection to BE the table — so the
      // dialog projected one table while the document held two, with nothing on
      // screen to show it. Cmd+Z was different and no better: it tore the
      // dialog down out from under the author.
      //
      // SO THE SECOND QUESTION IS "IS A MODAL OPEN?", KEYED ON THE OPEN-MODAL
      // STATE AND NEVER ON A LIST OF KEYS. A key-list guard is the shape that
      // produced this defect; it goes stale the moment a shortcut is added.
      //
      // ⚠ AND IT IS WHAT MAKES THE TABLE EDITOR'S EDIT COUNT TRUSTWORTHY. The
      // Cancel sequence's `N <= MAX_ENGINE_HISTORY_ENTRIES` bound assumes the
      // dialog's own commands are the ONLY source of undo entries while it is
      // open. Before this line that was false — a nudge or a duplicate pushed an
      // entry the dialog never counted, so N undos would consume it and leave
      // one of the dialog's own edits standing. The guard and the bound are the
      // same claim.
      //
      // `fontBrowserOpen` IS DELIBERATELY NOT HERE (Q1 = B'): the font browser
      // has the same leak and keeps it for now, as DW-371.
      //
      // Cmd+S sits ABOVE this line and is untouched: saving is not a mutation of
      // what the modal edits.
      if (editing || tableEditor !== undefined || pageDeleteConfirm !== undefined) return
      if (modifier && event.key.toLowerCase() === 'z' && !event.shiftKey && undoAvailable) { event.preventDefault(); void applyHistory('undo'); return }
      if ((modifier && event.shiftKey && event.key.toLowerCase() === 'z' || !mac && modifier && event.key.toLowerCase() === 'y') && redoAvailable) { event.preventDefault(); void applyHistory('redo'); return }
      if (modifier && event.key.toLowerCase() === 'd' && modeRef.current === 'design' && selectedRef.current.length === 1) { event.preventDefault(); duplicateSelection(); return }
      // CANVAS CLIPBOARD, DELETE AND SELECT ALL. Design mode only, and never
      // while a pointer gesture, a component drag, a placement or file work
      // owns the canvas. The editable-target and open-modal guard above
      // already applies.
      // A key aimed at a control outside the canvas is left to the browser.
      if (canvasKeyAllowed(event)) {
        const key = event.key.toLowerCase()
        if ((event.key === 'Delete' || event.key === 'Backspace') && keyboardDelete(event)) { event.preventDefault(); return }
        if (modifier && !event.altKey && !event.shiftKey && key === 'c' && selectedRef.current.length > 0) { event.preventDefault(); copySelection(); return }
        if (modifier && !event.altKey && !event.shiftKey && key === 'v' && !event.repeat) { if (pasteClipboard()) event.preventDefault(); return }
        if (modifier && !event.altKey && !event.shiftKey && key === 'a') { event.preventDefault(); selectAllInFocusBand(); return }
      }
      if (modeRef.current === 'design' && breakPage !== undefined && (event.key === 'ArrowUp' || event.key === 'ArrowDown') && canvasKeyAllowed(event)) { event.preventDefault(); nudgeSectionBreak(breakPage, event.shiftKey, event.key === 'ArrowDown'); return }
      if (modeRef.current === 'design' && selectedRef.current.length === 1 && ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) { event.preventDefault(); const step = event.shiftKey ? 10_000 : 1_000; if (event.key === 'ArrowLeft') nudgeSelection(-step, 0); if (event.key === 'ArrowRight') nudgeSelection(step, 0); if (event.key === 'ArrowUp') nudgeSelection(0, -step); if (event.key === 'ArrowDown') nudgeSelection(0, step); return }
      if (event.altKey && altLetter(event, 's') && modeRef.current === 'design') { event.preventDefault(); setSnapEnabled((value) => !value); return }
      if (event.altKey && altLetter(event, 'p') && engine && snapshotRef.current) {
        event.preventDefault()
        if (mode === 'preview') returnToDesign()
        else enterPreview()
      }
    }
    window.addEventListener('keydown', shortcut)
    return () => window.removeEventListener('keydown', shortcut)
  })

  useEffect(() => () => cancelPreviewWork(), [])

  // STORY 13.5 — THE TICKER, AND THE TWO CONDITIONS THAT FENCE IT.
  //
  // The document bar states how long ago the render on screen finished, and a
  // figure that is true for one instant and false forever after is not what
  // this story's title claims. So it advances on its own.
  //
  // IT MOUNTS ONLY IN PREVIEW, AND ONLY OVER AN INSTALLED RECORD. Both
  // conditions are folded into `previewInstalledAt` rather than tested
  // separately in the effect body, so there is ONE expression deciding whether
  // a live interval exists and Design mode cannot acquire one by a second
  // route. When it goes `undefined` — leaving Preview, or clearing the preview
  // — the effect re-runs and its cleanup clears the interval.
  //
  // THE PERIOD COMES OFF THE LADDER, NOT OFF A CONSTANT. `renderAgeTickMs` is
  // the same function `formatRenderAge` shares its tiers with, so a display
  // printing whole minutes cannot go on repainting ten times a second, and a
  // display printing milliseconds cannot go a second between repaints. Crossing
  // a tier re-arms at the new period, which is why this is a rescheduling
  // interval and not one fixed `setInterval`.
  const previewInstalledAt = mode === 'preview' ? preview?.installedAt : undefined
  useEffect(() => {
    if (previewInstalledAt === undefined) return
    const installedAt = previewInstalledAt
    let handle: ReturnType<typeof setInterval> | undefined
    let period = 0
    function arm(now: number): void {
      const next = renderAgeTickMs(installedAt, now)
      if (next === period) return
      if (handle !== undefined) clearInterval(handle)
      period = next
      handle = setInterval(tick, next)
    }
    function tick(): void {
      const now = Date.now()
      setRenderAgeNow(now)
      arm(now)
    }
    const started = Date.now()
    setRenderAgeNow(started)
    arm(started)
    return () => { if (handle !== undefined) clearInterval(handle) }
  }, [previewInstalledAt])

  if (loadState && !engine) {
    if (engineMayStart(loadState) && engineState !== 'failed') return <main className="engine-starting" aria-label="Engine preparation"><p role="status" aria-live="polite" aria-label="Engine preparation status">Starting local engine</p></main>
    return <LoadScreen lifecycle={loadState} payload={payload} engineState={engineState} onRetry={onRetry} />
  }

  const shortcuts = shortcutHintsFor()
  // STORY 13.4 — THE NO-DATA SCREEN, AND THE FABRICATED CONDITION IT NAMES.
  //
  // `noDataPreview` is "this screen is previewing without sample data": it is
  // what the heading and the status line withhold their production claim on.
  //
  // `standInNotice` IS A DIFFERENT QUESTION AND MUST STAY ONE. The notice
  // describes THE BYTES ON SCREEN, not the current inputs — the same subject
  // the viewer label and the digest line already have. Keying it on
  // `!sampleData` was wrong in both directions: loading a sample unmounted the
  // notice while the stand-in PDF was still displayed and still stale, and the
  // reverse transition would have printed "built from stand-ins" beside a
  // digest line correctly reading `Historical producer digest` — one PDF, two
  // contradictory claims. So: when a record is installed, follow the record;
  // when none is (before the first render, and after a clear), the screen
  // state is the only thing there is to follow.
  const noDataPreview = mode === 'preview' && !sampleData
  const standInNotice = mode === 'preview' && (preview !== undefined ? preview.standIn : !sampleData)
  // THE SPACE A TABLE'S COLUMNS HAVE, IN THE ENGINE'S OWN TERMS.
  // `folio-go/component_commands.go`'s `containComponent` refuses a component
  // whose `width > band.Width - x`, and `projectedSize` makes a table's width Σ
  // its column widths. So the budget's denominator is the band's width LESS the
  // table's own x — not the band's width, which would tell an author a table
  // indented into the band has more room than it has. `undefined` when the
  // canvas or either party is not projected: unknown, never a number.
  const tableEditorAvailableWidth = ((): number | undefined => {
    const component = canvas?.components.find((candidate) => candidate.id === tableEditor?.table.tableId)
    const bandBox = canvas?.bands.find((candidate) => candidate.name === component?.band)
    return component === undefined || bandBox === undefined ? undefined : bandBox.width - component.x
  })()
  const currentDiagnostics = previewStatus === 'current' && mode === 'preview' && preview?.revision === snapshot?.revision ? preview : undefined
  const currentFailure = previewError && ['error', 'stale'].includes(previewStatus) && mode === 'preview' && previewError.revision === snapshot?.revision ? previewError : undefined
  // STORY 13.5 — ONE CALL, TWO RENDERINGS. The document bar's token and the
  // preview heading's status line are the same fact said at two lengths, and
  // they used to be two independent expressions in this file. `freshnessChrome`
  // returns both from one switch, so they cannot disagree; no call site here
  // may build either string itself.
  const previewChrome = freshnessChrome({ status: previewStatus, staleReason, standIn: noDataPreview, hasRecord: preview !== undefined, issue: previewIssue, failureMessage: currentFailure?.error.message })
  // `no render yet` IS THE HONEST READING BEFORE THE FIRST RENDER: with no
  // record there is no instant to count from, so the bar states the absence
  // instead of an age of zero. The token is never `undefined` while a record
  // exists, and the guard says so rather than trusting that.
  const renderFreshness = preview && previewChrome.token ? `rendered ${formatRenderAge(preview.installedAt, renderAgeNow)} · ${previewChrome.token}` : 'no render yet'
  const engineLabel = initializationError ? 'ENGINE UNAVAILABLE' : snapshot ? `GO SNAPSHOT · REVISION ${snapshot.revision}` : 'ENGINE STARTING'
  const offlineLabel = import.meta.env.DEV && offlineState === 'dev-bypass' ? 'Offline layer bypassed (dev)' : offlineState === 'ready' ? 'Offline ready' : offlineState === 'checking' ? 'Offline cache checking' : offlineState === 'update-available' ? 'Update available; current release remains usable' : 'Offline cache unavailable'
  const dirty = !snapshot || savedRevision === undefined || snapshot.revision !== savedRevision
  const saveLabel = dirty ? 'Unsaved local changes' : 'Saved local file'
  // ONE SHEET PER PROJECTED WINDOW, from the projection alone. The model is
  // built in sheet-stack.ts, which is pure arithmetic over Go's numbers: the
  // window origins, the window height and the page geometry. Nothing here
  // measures the DOM and nothing multiplies a window height by an index.
  const displayCanvas = canvas && canvasSelection.group ? translatedCanvas(canvas, canvasSelection.group.ids, canvasSelection.group.dx, canvasSelection.group.dy, canvasSelection.group.page) : canvas
  const stack = displayCanvas ? sheetStack(displayCanvas) : undefined
  // SPEC-multi-pages story 2: ONE reason per disabled page button, fed to its
  // tooltip and its accessible description alike — never a bare grey-out.
  const fileActionReason = 'a file action is in progress'
  const addPageReason = !canvas ? 'no document is open' : fileBusy ? fileActionReason : undefined
  const deletePageReason = canvas && fileBusy ? fileActionReason : 'reason' in deletePageTarget ? deletePageTarget.reason : undefined
  const sheetSurface = (projection: CanvasProjection, model: SheetStack, sheet: Sheet) => {
    const sheets = model.sheets.length
    // A single-sheet document must render exactly the DOM and exactly the
    // accessible names it rendered before this story, so the page qualifier
    // appears only once there is more than one page to be ambiguous about —
    // and RTL and Playwright both fail outright on a duplicate exact label.
    const many = sheets > 1
    // Drawn ONCE per page, on the sheet of that page whose window holds the
    // offset, with no echoes.
    const breakAt = sectionBreakPlacement(projection, sheet.page)
    // D-4.1: with more than one DESIGNED page, names count designed pages, and a
    // sheet that is not its page's first adds its sheet number so every name
    // stays unique. A one-page document, overflow sheets included, keeps today's
    // sheet-counted names exactly.
    const designedPages = pageCountOf(projection)
    const multiPage = designedPages > 1
    const pageNumber = multiPage ? ` ${sheet.page + 1} of ${designedPages}${sheet.pageStart ? '' : `, sheet ${sheet.index + 1} of ${sheets}`}` : ` ${sheet.index + 1} of ${sheets}`
    const pageOf = many ? pageNumber : ''
    // Story 4: the shared header and footer draw their one interactive copy on
    // the current page's first sheet (sheet 0 if the cap truncated it away).
    const repeatingHome = model.sheets.find((candidate) => candidate.page === currentPage && candidate.pageStart)?.index ?? 0
    // SPEC-multi-pages story 2: clicking a sheet's empty space selects ITS
    // designed page, every sheet of which is outlined; the page's first sheet
    // carries the page label, which selects the page too.
    const pageIsSelected = pageSelection === sheet.page
    return <section key={sheet.index} data-page={sheet.page} className={`page-surface${gridVisible ? ' page-grid' : ''}${pageIsSelected ? ' page-surface-selected' : ''}`} aria-label={`Report page${pageOf} with Page Header, Content, and Page Footer`} style={pageStyle(projection, zoom)} onPointerDown={(event) => beginRectangle(event, undefined, sheet.index)} onClick={(event) => { if (!event.shiftKey) selectPage(sheet.page) }}>
      {sheet.pageStart ? <button type="button" className="page-label" aria-pressed={pageIsSelected} onPointerDown={(event) => event.stopPropagation()} onClick={(event) => { event.stopPropagation(); selectPage(sheet.page) }}>{`Page ${sheet.page + 1}`}</button> : undefined}
      <CanvasSelectionLayer>{projection.bands.map((band) => {
        const content = band.name === 'content'
        // The content band is one WINDOW of the column, so a component's
        // in-sheet position is its column position minus this window's
        // origin. A repeating band is the same band on every sheet, so its
        // origin is zero on all of them.
        const origin = content ? sheet.origin : 0
        // An empty repeating band has no page hit target; keep its named
        // creation path so the engine refuses it instead of hitting Content.
        const dropOnPage = placing === 'sectionBreak' ? false : placing === 'image' && !content ? band.height > 0 : sheet.index === 0
        // SPEC-multi-pages story 3: a palette element placed on a later page's
        // content band is created in THAT page's column, so the command names
        // the page. Page 1 names none and sends today's bytes. Only a section
        // break is still refused there: a later page's break is story 5's.
        const targetPage = content && sheet.page > 0 ? sheet.page : undefined
        // Story 5: a section break goes to this page too, but while armed a
        // content band on a page that already has its break is not a target
        // (D-5.1).
        const accepts = !(content && placing === 'sectionBreak' && sectionBreakOnPage(projection, sheet.page) !== undefined)
        // Where a content occurrence was pressed, down the whole stack: the
        // occurrence's top on this sheet plus the press's own offset into it
        // when the component itself is the target (a local event coordinate,
        // never a layout query).
        const grabAt = (occurrence: SheetOccurrence, event: PointerEvent) => {
          // Read through placementPoint, the one door for a local pointer offset.
          const pressed = event.target === event.currentTarget ? placementPoint(event.nativeEvent, { ...band, x: 0, y: 0 }, zoom).y * 1000 : 0
          const offset = Number.isFinite(pressed) ? pressed : 0
          return content ? sheet.index * sheetPitch(projection, zoom) + band.y + occurrence.y + offset : undefined
        }
        // The two repeating bands are drawn on every sheet because the engine
        // repeats them — but exactly ONE occurrence of each of their
        // components is interactive and accessibly named, the same rule a
        // content component spanning two windows obeys (Ruling G). Two
        // identical accessible names for one component would break selection,
        // getByLabelText and Playwright's strict mode alike.
        const occurrences = content ? sheet.content : projection.components.filter((component) => component.band === band.name).map((component) => ({ component, y: component.y, home: sheet.index === repeatingHome }))
        const target = `${sheet.index}:${band.name}`
        const paint = (occurrence: SheetOccurrence) => occurrence.home
          ? <CanvasComponent key={occurrence.component.id} component={occurrence.component} carriedFaces={paintableFaces} chromeOffset={{ x: band.x, y: band.y }} origin={occurrence.component.y - occurrence.y} note={content && !sheet.pageStart ? canvasColumnPositionNotice(sheet.index + 1, sheets, multiPage ? 'sheet' : 'page') : undefined} limit={{ band: band.name, width: band.width, height: band.height }} zoom={zoom} selected={selected.includes(occurrence.component.id)} selectedColumnId={selectedTableColumn?.tableId === occurrence.component.id ? selectedTableColumn.columnId : undefined} preview={drag?.id === occurrence.component.id ? drag : undefined} engine={engine} generation={documentGenerationValue} trackColumn={content && many ? (edge: number, delta: number) => columnEdgeAfterDrag(model, projection, zoom, edge, delta, componentPage(occurrence.component)) : undefined} onSelect={select} onBodyPress={(id, event) => beginSelectedGroup(id, event, grabAt(occurrence, event))} onDelete={keyboardDelete} onDragStart={setDrag} onDragEnd={(finished) => { if (!finished.changed) { setDrag(undefined); return } const command = finished.mode === 'move' ? moveComponentCommand(occurrence.component.id, finished.x, finished.y, snapEnabled) : setComponentBoundsCommand(occurrence.component.id, finished.x, finished.y, finished.width, finished.height, snapEnabled); setDrag({ ...finished, released: true }); void commitComponent(command, () => setDrag(undefined)).finally(() => setDrag(undefined)) }} />
          : <ComponentEcho key={`${occurrence.component.id}@${sheet.index}`} component={occurrence.component} carriedFaces={paintableFaces} selected={selected.includes(occurrence.component.id)} onSelect={select} onBodyPress={(event) => beginSelectedGroup(occurrence.component.id, event, grabAt(occurrence, event))} onPress={content ? undefined : (event) => pressRepeatingEcho(occurrence.component.id, sheet.page, event)} y={occurrence.y} zoom={zoom} engine={engine} generation={documentGenerationValue} />
        // Body previews stay inside their starting window, or draw on the
        // sheet under the pointer when moving to another page (story 3). Only
        // the existing resize path lifts the clip while its anchor tracks.
        const dragging = occurrences.some((occurrence) => drag?.id === occurrence.component.id)
        // ONE INTERACTIVE BOUNDARY PER DOCUMENT. The canvas draws 3N band
        // sections for an N-page stack and the document has ONE page-header
        // height, so the handle follows the `occurrence.home` idiom the
        // repeating components already follow: its ONE named, tabbable copy is
        // on the current page's first sheet (SPEC-multi-pages story 4). Every
        // other sheet draws an aria-hidden, untabbable copy that still takes a
        // drag, so the shared height can be resized from any page.
        //
        // The handle and the proposal are DIRECT CHILDREN of the band, outside
        // `.band-window`: that div clips to one window, and anything inside it
        // would be cut off — which is also why the boundary rule and the band
        // tab live outside it today.
        //
        // AND THE PROPOSAL AND READOUT ARE <div>, NOT <span>, WHICH IS LOAD
        // BEARING. App.css's band-tab rule is `.page-band > span` at
        // specificity (0,1,1); a class rule is (0,1,0). A <span> here would
        // therefore be PAINTED AS A BAND TAB — pinned at `top: 0` instead of
        // the proposed offset, translated a tab's width off the page, bordered,
        // tinted and uppercased — and jsdom, which applies no stylesheet, would
        // never see it. The band tab itself is untouched; App.test.tsx asserts
        // that no `.page-band > …` selector in the sheet can match these two.
        const boundary = boundaryAbove(band.name)
        // NOTHING IS PAINTED UNTIL THE GESTURE IS A DRAG. `changed` is the same
        // 2px travel gate the release consults, so a press — or a 1px
        // press-and-release — puts no line and no readout on the canvas that
        // the gesture has already decided to discard.
        // The band height is shared, so a live proposal shows on every sheet.
        const proposal = boundary && boundaryDrag?.band === boundary && boundaryDrag.changed ? boundaryDrag : undefined
        return <section key={band.name} className={`page-band page-band-${band.name}${hoverBand === target ? ' page-band-target' : ''}`} aria-label={many ? `${bandName(band.name)} on page${pageNumber}` : bandName(band.name)} aria-current={hoverBand === target ? 'true' : undefined} style={bandStyle(band, zoom, origin, projection.gridIncrement)} onPointerDownCapture={() => { focusBandRef.current = band.name }} onFocus={() => { focusBandRef.current = band.name }} onPointerDown={(event) => beginRectangle(event, band, sheet.index)} tabIndex={0} onPointerEnter={() => placing && accepts && setHoverBand(target)} onPointerLeave={() => setHoverBand((current) => current === target ? undefined : current)} onPointerUp={(event) => { if (placing && accepts && event.currentTarget === event.target) { const point = placementPoint(event.nativeEvent, band, zoom); if (dropOnPage) place(point.x, point.y); else placeInBand(band.name, point.x - band.x / 1000, origin / 1000 + point.y - band.y / 1000, targetPage) } }} onKeyDown={(event) => { if (placing && accepts && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); if (dropOnPage) place(band.x / 1000, band.y / 1000); else placeInBand(band.name, 0, origin / 1000 + (placing === 'sectionBreak' ? contentBandHeight(projection) / 2000 : 0), targetPage) } }}><span>{bandName(band.name)}</span>{boundary ? <button type="button" className="band-boundary-handle" {...(sheet.index === repeatingHome ? { 'aria-label': boundaryLabel(boundary) } : { 'aria-hidden': true, tabIndex: -1 })} onPointerDown={(event) => beginBoundaryDrag(boundary, event)} onPointerMove={moveBoundaryDrag} onPointerUp={finishBoundaryDrag} onPointerCancel={cancelBoundaryDrag} onKeyDown={(event) => nudgeBoundary(boundary, event)} /> : undefined}{proposal ? <><div className="band-boundary-proposal" aria-hidden="true" style={{ '--boundary-display-y': canvasDisplay.css(boundaryOffset(proposal.band, proposal.original, proposal.proposed), zoom) } as CSSProperties} /><div className="band-boundary-readout" aria-hidden="true" style={{ '--boundary-display-y': canvasDisplay.css(boundaryOffset(proposal.band, proposal.original, proposal.proposed), zoom) } as CSSProperties}>{points(proposal.proposed)}</div></> : undefined}{content && breakAt?.sheet === sheet.index ? sectionBreakMarker(breakAt.y, sheet.page) : undefined}{many || content ? <div className={`band-window${dragging ? ' band-window-open' : ''}`}>{occurrences.map(paint)}</div> : occurrences.map(paint)}{content && sheet.seam !== undefined ? <span className="page-seam" aria-hidden="true" style={{ '--seam-display-y': canvasDisplay.css(sheet.seam, zoom) } as CSSProperties} /> : undefined}</section>
      })}</CanvasSelectionLayer>
    </section>
  }
  // STORY 13.2 — THE VIEWER'S NAVIGATION, IN THE APPLICATION'S STATUS BAR.
  //
  // AC6 puts the page stepper, the page indicator and the zoom in the bottom
  // bar "so the page area carries the page and nothing else". That bar is a
  // sibling of `.workbench`, outside both `<main>`s, so App has to own the
  // controls and the viewer has to give them up. The view state was already
  // App's, which is what makes this a move rather than a lift.
  //
  // THE CONTROL SHAPE IS THE ORCHESTRATOR'S CALL AT THIS STORY'S PLAN GATE, NOT
  // A MOCKUP-DERIVED REQUIREMENT. `Preview.dc.html` is a static picture — no
  // button, input or select anywhere in it — and the strings "Fit width" and
  // "Fit page" appear nowhere in the design tree. What the mockup does settle is
  // the left-to-right order below and the bar's 32px preview height.
  const previewPageValue = previewPageDraft ?? String(previewViewState.page)
  const previewZoomValue = previewZoomDraft ?? String(Math.round(previewViewState.scale * 100))
  const goToPreviewPage = (page: number) => changePreviewViewState({ ...previewViewState, page })
  // A MANUAL ZOOM CLEARS THE FIT. Both answer the same question, so they cannot
  // both stand; and because the viewer writes the RESOLVED fit scale back into
  // the view state, stepping from here starts wherever the page actually is.
  const setPreviewScale = (scale: number) => changePreviewViewState({ ...previewViewState, scale, fit: undefined })
  const stepPreviewZoom = (steps: number) => { const next = steppedPreviewScale(previewViewState.scale, steps); if (next !== undefined) setPreviewScale(next) }
  // Accepted or refused, the field goes back to reading the view state, so a
  // rejected entry shows the page the viewer is really on rather than the typo.
  //
  // NOTHING TO COMMIT MEANS NOTHING TO WRITE. Both handlers also run on `blur`,
  // and with no draft the field is showing the DERIVED readout — so merely
  // moving focus through it would re-commit that readout. `setPreviewScale`
  // sets `fit: undefined`, so a Tab keypress would silently destroy an active
  // fit; and a resolved scale the readout rounds (800/612 = 1.30719…, shown as
  // `131`) would drift to 1.31. The frozen matrix clears a fit when the author
  // presses `+` OR TYPES A ZOOM, and a focus traversal is neither, so each
  // handler returns the moment it finds it has no draft of its own to commit.
  const commitPreviewPage = () => { if (previewPageDraft === undefined) return; const parsed = typedPreviewPage(previewPageDraft, previewPages); if (parsed !== undefined) goToPreviewPage(parsed); setPreviewPageDraft(undefined) }
  const commitPreviewZoom = () => { if (previewZoomDraft === undefined) return; const parsed = typedPreviewZoom(previewZoomDraft); if (parsed !== undefined) setPreviewScale(parsed); setPreviewZoomDraft(undefined) }
  const previewZoomChoice = previewViewState.fit === 'width' ? 'fit-width' : previewViewState.fit === 'page' ? 'fit-page' : PREVIEW_ZOOM_CHOICES.includes(previewViewState.scale) ? String(previewViewState.scale) : 'custom'
  const choosePreviewZoom = (value: string) => {
    if (value === 'fit-width' || value === 'fit-page') { changePreviewViewState({ ...previewViewState, fit: value === 'fit-width' ? 'width' : 'page' }); return }
    const scale = clampPreviewScale(Number(value))
    if (scale !== undefined) setPreviewScale(scale)
  }
  // Every name carries the `PDF` qualifier the viewer's controls already did,
  // so none of them collides with `.canvas-tools`' `Zoom in`/`Zoom out`/`Canvas
  // zoom` in Design mode. They are ordinary buttons, inputs and a select, so
  // keyboard reach, activation and the focus ring are the platform's own.
  const previewNavigation = <span className="canvas-tools preview-nav" role="group" aria-label="PDF navigation">
    <span className="preview-nav-group">
      <button type="button" onClick={() => goToPreviewPage(Math.max(1, previewViewState.page - 1))} disabled={previewViewState.page <= 1} aria-label="Previous PDF page">◀</button>
      <input type="text" inputMode="numeric" aria-label="PDF page number" value={previewPageValue} onChange={(event) => setPreviewPageDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') commitPreviewPage() }} onBlur={commitPreviewPage} />
      <output aria-live="polite" aria-label="PDF page status">{previewPages ? `Page ${previewViewState.page} of ${previewPages}` : 'Rendering PDF'}</output>
      <button type="button" onClick={() => goToPreviewPage(previewPages ? Math.min(previewPages, previewViewState.page + 1) : previewViewState.page + 1)} disabled={!previewPages || previewViewState.page >= previewPages} aria-label="Next PDF page">▶</button>
    </span>
    <span className="preview-nav-divider" aria-hidden="true" />
    <span className="preview-nav-group">
      <button type="button" onClick={() => stepPreviewZoom(-1)} aria-label="Zoom out PDF">−</button>
      <select aria-label="PDF zoom" value={previewZoomChoice} onChange={(event) => choosePreviewZoom(event.target.value)}>
        <option value="fit-width">Fit width</option>
        <option value="fit-page">Fit page</option>
        {PREVIEW_ZOOM_CHOICES.map((choice) => <option key={choice} value={String(choice)}>{`${Math.round(choice * 100)}%`}</option>)}
        {/* Offered only while the scale is one no listed choice names — a zoom
            stepped by hand, or a fit resolved to a ratio like 133%. Picking it
            changes nothing; it exists so the select never mislabels the zoom. */}
        {previewZoomChoice === 'custom' && <option value="custom">Custom</option>}
      </select>
      <input type="text" inputMode="numeric" aria-label="PDF zoom percentage" value={previewZoomValue} onChange={(event) => setPreviewZoomDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') commitPreviewZoom() }} onBlur={commitPreviewZoom} />
      <button type="button" onClick={() => stepPreviewZoom(1)} aria-label="Zoom in PDF">+</button>
    </span>
  </span>
  return <div className={`app-shell${mode === 'preview' ? ' app-shell-preview' : ''}`} aria-label="folio8 designer application shell" aria-busy={fileBusy}>
    <header className="document-bar" aria-label="Document bar">
      {/* STORY 14.5 — THE PRODUCT WEARS ITS OWN MARK.
          The mark is decorative (`aria-hidden`, no name of its own), so the
          lockup's accessible text is the wordmark ALONE — the product announces
          itself once, not twice. `.brand` stays byte-identical inside the
          lockup; only the wrapper is new. 18px here, 22px on the load screen,
          and nowhere else. */}
      <span className="brand-lockup"><BrandMark size={18} /><span className="brand">Folio8</span></span><span className="document-name">{title}</span><span className={`status-dot${dirty ? '' : ' status-clean'}`} aria-hidden="true" /><span className="status-copy" role="status">{saveLabel}</span>
      {/* SIX GLYPHS IN ONE NAMED GROUP, BY OWNER RULING.
          Story 14.1 spelled this family as six words; the owner has since ruled
          the document bar and the canvas toolbar are glyph controls with a hover
          guide. The family is still uniform (V3): all six are `.tool-button`
          glyphs, and `role="group"` keeps `Local file actions` in the
          accessibility tree. Every `aria-label` carries the name the control
          answered to as a word, byte-identical, so no query moved. The hover
          guide — name plus shortcut — is `data-tip`, painted by CSS; there is no
          `title`, so the browser does not draw a second, slower tooltip over it.
          `.file-button` stays the words' class (TABLE's `Configure columns`,
          the image picker): sharing it with glyphs would split one class across
          two treatments. */}
      {/* THE LOCAL-FILE MESSAGES NOW SIT WITH THE BUTTONS THEY ARE ABOUT.
          They were rendered at the tail of the design and preview mains, which
          are scrollable regions, so the sentence explaining a busy or failed
          file action was routinely below the fold while the four buttons it
          explains sat up here wearing `cursor: not-allowed`. A disabled control
          whose reason is off-screen reads as a BROKEN control: the reported
          symptom was "clicking Open does nothing", when Open was disabled by
          `fileBusy` and "Opening local file…" was on the page the whole time.

          ⚠ ABSOLUTELY POSITIONED, INSIDE THIS GROUP, AND THAT IS A LAYOUT
          RULING RATHER THAN A STYLE ONE. `.document-bar` is a single flex row
          with no wrap, and `e2e/document-bar-fit.spec.ts` measures the spare
          room between this group and `.later-control` at the shell's declared
          1024px minimum, requiring it to stay above zero. A message rendered as
          a SEVENTH FLEX ITEM would consume exactly that slack — "Opened local
          file statement.folio; canonical local changes need saving" is wider
          than the whole measured gap — so the bar would overfill on the very
          sentence this change exists to show. Out of the row, the instrument
          measures what it always did and the message cannot overfill anything.

          ONE NODE, NOT TWO. `fileError` WINS over `fileStatus` rather than
          rendering alongside it. `announceFailure` already clears the status
          when it sets the error, so the two are never both set today — the
          ternary is here so a future writer who sets one without clearing the
          other cannot produce two competing sentences in one slot. The alert is
          the half that must never be the one lost.

          NOT FOLDED INTO `.status-copy` next door, which is its own
          `role="status"`: that line says what the DOCUMENT is (saved, dirty)
          and this one says what the last file ACTION did. One line with two
          writers means either can erase the other's sentence. */}
      <div className="document-actions" role="group" aria-label="Local file actions"><button className="tool-button" type="button" onClick={() => void open()} disabled={!engine || !fileAccess || fileBusy} aria-label="Open local template" data-tip="Open"><ToolIcon glyph="open" /></button><button className="tool-button" type="button" onClick={() => void save(false)} disabled={!engine || !fileAccess || fileBusy} aria-label="Save local template" data-tip={toolTip('Save', shortcuts.save)}><ToolIcon glyph="save" /></button><button className="tool-button" type="button" onClick={() => void save(true)} disabled={!engine || !fileAccess || fileBusy} aria-label="Save As" data-tip="Save As"><ToolIcon glyph="save-as" /></button><button className="tool-button" type="button" onClick={openNewDialog} disabled={!engine || !blankBytes || fileBusy} aria-label="New…" data-tip="New…"><ToolIcon glyph="blank" /></button><button className="tool-button" type="button" onClick={() => void applyHistory('undo')} disabled={!undoAvailable || fileBusy} aria-label="Undo" data-tip={toolTip('Undo', shortcuts.undo)}><ToolIcon glyph="undo" /></button><button className="tool-button" type="button" onClick={() => void applyHistory('redo')} disabled={!redoAvailable || fileBusy} aria-label="Redo" data-tip={toolTip('Redo', shortcuts.redo)}><ToolIcon glyph="redo" /></button>{fileError ? <span role="alert" className="bar-message bar-message-alert" title={fileError}>{fileError}</span> : fileStatus ? <span role="status" aria-live="polite" className="bar-message" title={fileStatus}>{fileStatus}</span> : undefined}</div>
      {/* STORY 13.5 — THE SLOT SAYS SOMETHING ABOUT WHAT IS ON SCREEN.
          In Design that is the page setup; in Preview the page setup is a fact
          about a template nobody is looking at, and the render's own freshness
          is the fact the frame owes the author.

          NO `role="status"`, NO `aria-live`, and no `title` or
          `aria-describedby` either. Every neighbour in this bar is a live
          region and copying one here would announce the age on every tick —
          once a second, then once a minute, forever. The two ARIA associations
          are refused because no acceptance criterion asks for either and a
          cross-region association fails silently the moment either end moves. */}
      {mode === 'preview'
        ? <span className="later-control" aria-label="Render freshness">{renderFreshness}</span>
        : <span className="later-control" aria-label="Current page setup">{canvas ? `${canvas.preset} · ${canvas.orientation}` : 'Page setup unavailable'}</span>}
      <div className="mode-switch" role="group" aria-label="Designer mode"><button className={mode === 'design' ? 'mode-active' : ''} type="button" aria-pressed={mode === 'design'} onClick={returnToDesign}>DESIGN</button><button className={mode === 'preview' ? 'mode-active' : ''} type="button" aria-pressed={mode === 'preview'} onClick={enterPreview}>PREVIEW <kbd aria-hidden="true">{shortcuts.preview}</kbd></button></div>
      {/* THE DOCUMENTATION LINK IS ITS OWN GROUP, NOT A SEVENTH FILE ACTION.
          It is a real link to the bundled, precached rendering library guide,
          opened in a NEW TAB so the author's unsaved document, selection and
          undo history stay exactly where they are. It depends on nothing — no
          engine, template or render — so it is never disabled. Same glyph
          treatment as the file actions: `aria-label` names it, `data-tip` is
          the CSS-painted hover guide, and there is no `title`. */}
      <div className="documentation-actions" role="group" aria-label="Documentation"><a className="tool-button" href={documentationAssetUrls.guide} target="_blank" rel="noopener noreferrer" aria-label="Rendering library documentation" data-tip="Documentation"><ToolIcon glyph="docs" /></a></div>
    </header>
    <div className="workbench" id="future-features">
      {/* STORY 13.6 — THE PALETTE GIVES WAY TO THE PAGES RAIL.
          This column was an UNCONDITIONAL child until now: in Preview it was
          180px of placement controls that could not place anything, because
          the canvas they place onto is what Preview replaces. The mode
          ternary that already switches the middle column now switches this
          one too, off the SAME `mode` — never a second answer to "are we in
          preview".

          THE RAIL NEEDS A DOCUMENT TO ENUMERATE, so it renders only when
          there are bytes. A failed render has none, and the failure card in
          the main region is what the author reads instead; a rail of empty
          wells beside it would suggest pages that were never produced. */}
      {mode === 'design'
        ? <nav className="palette-rail" aria-label="Component palette"><p className="section-label">PALETTE</p>{paletteItems.map(([label, kind]) => <button className="palette-item" type="button" key={kind} onPointerDown={() => { setPlacing(kind); setHoverBand(undefined) }} onClick={() => { setPlacing(kind); setHoverBand(undefined) }} aria-pressed={placing === kind} aria-label={`Place ${label}`}><PaletteIcon kind={kind} />{label}<kbd>place</kbd></button>)}<button className="palette-item" type="button" onPointerDown={armSectionBreak} onClick={armSectionBreak} aria-pressed={placing === 'sectionBreak'} aria-label="Place Section Break" disabled={breakOnCurrentPage}><SectionBreakIcon />Section Break<kbd>place</kbd></button>{breakOnCurrentPage && <p className="honest-note">{pageCount === 1 ? 'This document already has its one Section Break.' : 'This page already has its Section Break.'}</p>}<p className="honest-note">Choose or drag a component, then choose a page band.</p><a className="palette-docs-link" href={documentationAssetUrls.guide} target="_blank" rel="noopener noreferrer"><ToolIcon glyph="docs" /><span>Documentation</span><span className="palette-docs-arrow" aria-hidden="true">↗</span></a></nav>
        : preview && <PageRail bytes={preview.bytes} pages={previewPages} currentPage={previewViewState.page} onGoToPage={goToPreviewPage} />}
      {/* NO onClick HERE, DELIBERATELY (Story 17.2). The backdrop — the grey
          space around the page — used to clear the selection when the click
          landed on the <main> itself. That guard (target === currentTarget)
          was written to spare the toolbar and the sheet stack, and so it fired
          on precisely the region where a click is most often a miss. Clicking
          the page surface (the `page-surface` onClick) and Escape (below, on
          this element, and NOT gated on the target test) remain the two ways
          to deselect. The revokeTableEditor call went with the clear: the
          editor is bound to the one selected component, and that selection now
          survives the click, so there is nothing left here for it to unbind.
          (Its own modal is `position: fixed; inset: 0; z-index: 20` at
          App.css:304, so in a real browser a click cannot reach this element
          while the editor is open — that is a reading of the stylesheet, which
          jsdom does not apply and no test measures.) */}
      {/* STORY 14.3 — `.canvas-region-placing` IS ALSO THE HIT PAD'S HOOK.
          The class below already carried the armed state for the `cursor: copy`
          rule (App.css:113); the pad's suppression rule keys on THE SAME class
          rather than a second attribute saying the same thing. One encoding, so
          the two cannot drift apart and no test is needed to pin them together.
          The suppression itself is `.canvas-component-echo`'s fix for the same
          defect class: an inert-until-needed pointer surface, so a click 4px
          from an existing rule is a PLACEMENT and not a selection of the
          neighbour. */}
      {mode === 'design' ? <main ref={canvasRegionRef} className={`canvas-region${placing ? ' canvas-region-placing' : ''}${selected.length > 1 ? ' canvas-region-multi' : ''}`} aria-label="Canvas region" tabIndex={0} onPointerMove={(event) => { canvasSelection.move(event); pointerPageRef.current = pageUnder(event.target); if (placing) setPlacingAt({ x: event.clientX, y: event.clientY }) }} onPointerUp={(event) => canvasSelection.finish(event)} onPointerCancel={() => canvasSelection.cancel()} onLostPointerCapture={() => canvasSelection.lostCapture()} onPointerDownCapture={(event) => { if (canvasSelection.blocksPointer()) { event.preventDefault(); event.stopPropagation() } else canvasSelection.freshPointer() }} onScroll={() => canvasSelection.cancel()} onClickCapture={(event) => { if (canvasSelection.consumeClick()) { event.preventDefault(); event.stopPropagation() } }} onPointerLeave={() => { setPlacingAt(undefined); pointerPageRef.current = undefined }} onKeyDown={(event) => { if (event.key === 'Escape') { if (canvasSelection.cancel()) { event.preventDefault(); event.stopPropagation(); return } clearInteraction(); installSelection([]) } }}>{/* THE CONTAINER IS THE LIVE REGION, AND IT IS ALWAYS MOUNTED. A `role="status"`
            put on each `<li>` would override its implicit `listitem` role, and a list
            that appears already populated is generally not announced at all — the
            region has to be on the page BEFORE the row arrives for the row to be read
            out. So the `<ul>` mounts empty and stays, and its children are plain list
            items. */}
          <ul className="canvas-face-misses" role="status" aria-live="polite" aria-label="Fonts the canvas could not paint">{shownFaceMisses.map((family) => <li key={family}><span>{family} could not be loaded, so this canvas is drawing a substitute for it. The layout, the page breaks and the PDF preview are unaffected — they come from the engine's own copy.</span><button type="button" className="diagnostic-dismiss" aria-label={`Dismiss the substitution warning for ${family}`} onClick={() => setDismissedFaceMisses((current) => new Set([...current, family]))}>Dismiss</button></li>)}</ul>
        <div className="canvas-tools" aria-label="Canvas controls"><button className="tool-button" type="button" onClick={() => setZoom((value) => Math.max(0.5, value - 0.1))} aria-label="Zoom out" data-tip="Zoom out"><ToolIcon glyph="zoom-out" /></button><output aria-label="Canvas zoom">{Math.round(zoom * 100)}%</output><button className="tool-button" type="button" onClick={() => setZoom((value) => Math.min(2, value + 0.1))} aria-label="Zoom in" data-tip="Zoom in"><ToolIcon glyph="zoom-in" /></button><button className="tool-button" type="button" onClick={() => setGridVisible((value) => !value)} aria-pressed={gridVisible} aria-label={`Grid ${gridVisible ? 'on' : 'off'}`} data-tip={`Grid ${gridVisible ? 'on' : 'off'}`}><ToolIcon glyph="grid" /></button><button className="tool-button" type="button" onClick={() => setSnapEnabled((value) => !value)} aria-pressed={snapEnabled} aria-label={`Snap ${snapEnabled ? 'on' : 'off'}`} data-tip={toolTip(`Snap ${snapEnabled ? 'on' : 'off'}`, shortcuts.snap)}><ToolIcon glyph="snap" /></button><button className="tool-button" type="button" onClick={duplicateSelection} disabled={selected.length !== 1} aria-label="Duplicate" data-tip={toolTip('Duplicate', shortcuts.duplicate)}><ToolIcon glyph="duplicate" /></button><button className="tool-button" type="button" onClick={breakSelected ? () => deleteSectionBreak(breakPage) : deleteSelection} disabled={selected.length === 0 && !breakSelected} aria-label="Delete" data-tip={toolTip('Delete', `${shortcuts.delete} key`)}><ToolIcon glyph="delete" /></button><button className="tool-button" type="button" onClick={addPage} disabled={addPageReason !== undefined} aria-label="Add page" data-tip={addPageReason ? `Add page: ${addPageReason}` : 'Add page'} aria-describedby={addPageReason ? 'add-page-reason' : undefined}><ToolIcon glyph="add-page" /></button>{addPageReason && <span id="add-page-reason" className="sr-only">{reasonSentence(addPageReason)}</span>}<button ref={deletePageButtonRef} className="tool-button" type="button" onClick={requestDeletePage} disabled={deletePageReason !== undefined} aria-label="Delete page" data-tip={deletePageReason ? `Delete page: ${deletePageReason}` : 'Delete page'} aria-describedby={deletePageReason ? 'delete-page-reason' : undefined}><ToolIcon glyph="delete-page" /></button>{deletePageReason && <span id="delete-page-reason" className="sr-only">{reasonSentence(deletePageReason)}</span>}<span className="tool-hint" role="img" aria-label={toolTip('Nudge', shortcuts.nudge)} data-tip={toolTip('Nudge', shortcuts.nudge)}><ToolIcon glyph="nudge" /></span></div>
        {displayCanvas && stack ? <div className="canvas-body" style={{ width: `calc(${canvasDisplay.css(displayCanvas.width, zoom)} + ${2 * CANVAS_GUTTER}px)`, paddingInline: `${CANVAS_GUTTER}px` }} onPointerDown={(event) => beginRectangle(event, undefined, 0, true)}><div className="sheet-stack" style={{ '--sheet-stack-gap': `${SHEET_STACK_GAP}px`, width: canvasDisplay.css(displayCanvas.width, zoom) } as CSSProperties} onPointerDown={(event) => beginRectangle(event)}>{stack.sheets.map((sheet) => sheetSurface(displayCanvas, stack, sheet))}{canvasSelection.rectangle && <div className="canvas-selection-rectangle" aria-label="Selection rectangle" style={{ left: canvasDisplay.css(canvasSelection.rectangle.left, zoom), top: canvasDisplay.css(canvasSelection.rectangle.top, zoom), width: canvasDisplay.css(canvasSelection.rectangle.right - canvasSelection.rectangle.left, zoom), height: canvasDisplay.css(canvasSelection.rectangle.bottom - canvasSelection.rectangle.top, zoom) }} />}</div></div> : <p className="canvas-awaiting" role="status">Waiting for Go page geometry.</p>}

        {pageDeleteConfirm !== undefined && <DeletePageDialog page={pageDeleteConfirm} onConfirm={confirmDeletePage} onCancel={cancelDeletePage} />}
        {placing && placingAt && <span className="placement-ghost" aria-hidden="true" style={{ '--ghost-x': `${placingAt.x}px`, '--ghost-y': `${placingAt.y}px` } as CSSProperties}>{placing === 'sectionBreak' ? <><SectionBreakIcon />Section Break</> : <><PaletteIcon kind={placing} />{paletteItems.find(([, kind]) => kind === placing)?.[0]}</>}</span>}
        {commitError && <p role="alert" className="file-message">{commitError}</p>}{locateStatus && <p role="status" aria-live="polite" className="file-message">{locateStatus}</p>}{/* STORY 14.7b — WHAT A COMPLETED CANCEL DISCARDED, in this region and in
            its OWN state. Not folded into `fileStatus`: that line is for local
            file outcomes, and two writers on one line means either can erase the
            other's sentence. */}{tableEditorDiscarded && <p role="status" aria-live="polite" className="file-message">{tableEditorDiscarded}</p>}
      </main> : <main className="preview-region" aria-label="Preview region"><p id="preview-freshness-status" className={previewStatus === 'current' ? 'sr-only' : 'preview-status'} role="status" aria-live="polite" aria-atomic="true">{previewChrome.statusLine}</p>{standInNotice && <div className="preview-standin-notice" role="note" aria-label="No-data preview notice" tabIndex={0}><span className="preview-standin-marker" aria-hidden="true">▲</span><div><p>No sample data is loaded. Every path this page reads from report data holds a stand-in value: text paths default to empty, numbers to zero (direct divisors to one), dates to 2024-01-15, and collections to empty. Formulas can produce other values. This page is not production output.</p><p>If a formula uses conditions, literal conditions retain their authored meaning; data-dependent conditions use stand-in values and may differ with sample data. Generated data does not guarantee a true condition.</p><p>Parameters are excluded from this: params.* still resolves from Preview inputs, and an absent parameter still fails the render.</p></div></div>}{previewNavigation}{currentFailure && <PreviewFailure error={currentFailure.error} onRetry={() => retryFromFailure(currentFailure)} onReturn={() => returnFromFailure(currentFailure)} />}{preview && <><PDFPreviewViewer bytes={preview.bytes} label={previewStatus !== 'current' ? `Stale historical PDF, revision ${preview.revision}` : preview.standIn ? `Current no-data layout PDF, revision ${preview.revision}` : `Current exact local production PDF, revision ${preview.revision}`} describedBy="preview-freshness-status" state={previewViewState} onStateChange={changePreviewViewState} onError={(error) => viewerError(preview.token, error)} onPageCount={(pages) => viewerPages(preview.token, pages)} />{currentDiagnostics && <PreviewDiagnostics diagnostics={currentDiagnostics.diagnostics} dismissed={dismissedDiagnostics} onDismiss={(key) => setDismissedDiagnostics((current) => new Set([...current, key]))} onLocate={(location) => locateDiagnostic(currentDiagnostics, location)} components={admittedPreview(currentDiagnostics) ? canvas?.components : undefined} />}</>}{/* STORY 13.3 — THE DIGEST LEFT THIS LINE AND BECAME THE RAIL'S OWN BLOCK.
        A grey footnote nobody can read a hash off is not evidence, and two
        copies of one digest on one screen is two things that can disagree.
        `Stand-in local digest` / `Historical producer digest` and the retained
        diagnostic count all survive, in `PreviewEvidenceRail`. */}{/* THE LOCAL-FILE MESSAGES USED TO BE RENDERED HERE, AND IN THE DESIGN MAIN,
        AND THEY ARE NOW IN THE DOCUMENT BAR INSTEAD (see the bar's own note).

        Story 13.1 put a copy in each main because the two are mutually
        exclusive, so a failure was still exactly one alert — which was true,
        and was not the problem. The problem was WHERE: both copies sat at the
        tail of a scrollable region, far from the four buttons whose state they
        explain, so a latched file bar showed `cursor: not-allowed` and its
        explanation below the fold. Moving the pair to the bar keeps Story
        13.1's rule intact (one alert, never inside a hideable container) and
        satisfies the rule it was missing: the sentence belongs beside the
        controls it is about.

        ⚠ NOT IN THE INSPECTOR PANEL — the original ruling, still binding. They
        were first put beside the control that produces them, inside
        `<div role="tabpanel" … hidden={inspectorTab !== 'properties'}>`, so
        switching to the DATA tab while a save was in flight took the
        `role="alert"` straight out of the accessibility tree. An alert that
        tests as present and behaves as absent is worse than no alert. The
        document bar is rendered in both modes and is hidden by nothing. */}</main>}
      <aside className={`inspector-panel${mode === 'preview' ? ' inspector-panel-preview' : ''}`} aria-label="Inspector">
        <div className="panel-tabs" role="tablist" aria-label="Inspector tabs">{inspectorTabs.map(([tab, designLabel, previewLabel]) => <button key={tab} type="button" role="tab" id={`inspector-tab-${tab}`} aria-controls={`inspector-panel-${tab}`} aria-selected={inspectorTab === tab} tabIndex={inspectorTab === tab ? 0 : -1} className={`panel-tab panel-tab-${tab}${inspectorTab === tab ? ' panel-tab-active' : ''}`} onClick={() => setInspectorTab(tab)} onKeyDown={(event) => { const next = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0; if (!next) return; event.preventDefault(); const order = inspectorTabs.map(([name]) => name); const target = order[(order.indexOf(tab) + next + order.length) % order.length]!; setInspectorTab(target); requestAnimationFrame(() => document.getElementById(`inspector-tab-${target}`)?.focus()) }}>{mode === 'preview' ? previewLabel : designLabel}</button>)}</div>
        <div className="panel-body" role="tabpanel" id="inspector-panel-properties" aria-label={mode === 'preview' ? 'Preview inputs' : 'Properties panel'} hidden={inspectorTab !== 'properties'}>{mode !== 'preview' && selectedTableColumn && <p className="column-identity" role="status">{/* STORY 14.10 / Q3(a) — AN IDENTITY STRIP, AND IT IS THE WHOLE OF WHAT AC1
            FORCES HERE. It ACKNOWLEDGES the column selection in the vocabulary
            the canvas and the table editor already use — the column's own label
            — and offers no control of any kind.
            ⚠ NO BINDING SECTION AND NO COLUMN CONTROLS. [D-14.4.Q2(a)] removed
            binding from the inspector once already; putting a column's binding
            here would re-commit that error inside the story written to finish
            undoing it, and would leave one value with two editing sites, which
            is the problem [D-14.10.1] exists to end. AC1's own gloss settles the
            scope: selecting a column is how binding one BEGINS — the binding
            itself belongs to the DATA panel.
            "Configure columns" stays live throughout: the TABLE is still the
            component selection, so `openTableEditor`'s
            `selectedRef.current[0] !== id` guard is untouched. */}<span className="column-identity-name">Column</span><span className="column-identity-meta">{selectedTableColumn.label === '' ? selectedTableColumn.columnId : selectedTableColumn.label}</span></p>}{mode === 'preview' ? <><p className="section-label">PREVIEW INPUTS</p><ParameterEditor referenceState={parameterReferenceState} accepted={previewParams} draft={previewParamsDraft} error={previewParamsError} onDraft={acceptPreviewParameters} onNamedValue={setNamedParameter} /><p className="honest-note">Parameters are local Preview input and are not part of the template.</p></> : selectedBreak !== undefined && breakPage !== undefined ? <SectionBreakProperties key={`${documentGenerationValue}:${breakPage}:${selectedBreak.offset}`} offset={selectedBreak.offset} anchor={selectedBreak.anchored} onCommit={(draft) => void commitComponent(setSectionBreakCommand(draft, false, breakPage))} onAnchor={(anchor) => void commitComponent(setSectionBreakAnchorCommand(anchor, breakPage))} /> : selected.length > 0 && canvas ? <ComponentProperties key={`${documentGenerationValue}:${selected.join(',')}`} components={canvas.components.filter((component) => selected.includes(component.id))} fontFamilies={canvas.fontFamilies} fontChains={canvas.fontChains} carriedFaces={paintableFaces} specimenBytes={familyControlSpecimenBytes} defaultFontSize={canvas.defaultFontSize} defaultLineSpacing={canvas.defaultLineSpacing} onCommit={applyProperties} onUseFamily={(source) => embedInstalledFamily(source, documentGeneration.current, selected.join(','))} onDeclareFamily={(source) => declareShippedFamily(source, documentGeneration.current, selected.join(','))} onOpenFontBrowser={() => { refreshHeldLocalFamilies(); setFontBrowserOpen(true) }} browserOpen={fontBrowserOpen} storedFaces={storedFaces} heldLocalFamilies={heldLocalFamilies} onFamilyListOpened={refreshHeldLocalFamilies} fontChainError={fontChainError} fontChainBusy={fontChainBusy || fileBusy} documentGeneration={documentGenerationValue} propertyError={propertyError} drag={drag} groupPreview={canvasSelection.group} onEditTable={(id) => void openTableEditor(id)} onPickImage={(id) => void applyImageAsset(id)} imageAvailable={imageFileAccess !== undefined} assetBusy={assetBusy} assetError={assetError} /> : <><div className="component-identity"><ToolIcon glyph="blank" /><span className="component-identity-name">Page</span><span className="component-identity-meta">{pageSelection !== undefined ? `page ${pageSelection + 1} of ${pageCount}` : `document · ${pageCount} ${pageCount === 1 ? 'page' : 'pages'}`}</span></div>{canvas && pageSelection !== undefined && <PageSection page={pageSelection} pageBreak={canvas.pageBreaks?.[pageSelection] ?? true} disabled={fileBusy} onPageBreak={(value) => void commitComponent(setPageBreakCommand(pageSelection, value))} />}<PageSetup preset={preset} orientation={orientation} draft={draft} onPreset={setPreset} onOrientation={setOrientation} onDraft={updateDraft} onApply={applyPageSetup} disabled={!canvas || fileBusy} /></>}</div>
        <div className="panel-body" role="tabpanel" id="inspector-panel-data" aria-labelledby="inspector-tab-data" hidden={inspectorTab !== 'data'}><DataPanel sample={sampleData} error={sampleError} busy={sampleBusy} available={Boolean(sampleFileAccess)} selectedComponentId={selected.length === 1 ? selected[0] : undefined} selectedComponentType={selectedComponent?.type} selectedBinding={selectedComponent?.type === 'table' ? selectedComponent.tableBind : selectedComponent?.binding} bindingError={bindingError} bindingBusy={bindingBusy} runtimeParameters={{ status: parameterReferenceState.status, names: parameterReferenceState.names, values: parameterValues(previewParams) }} columnScope={columnBindScope} saveDisabled={fileBusy || !fileAccess} onLoad={() => void loadSample()} onSave={() => void saveSampleData()} onConnect={(segments) => void bindPickedPath(segments)} onConnectColumn={(field) => void bindPickedColumn(field)} /></div>
        {/* STORY 13.3 — THE EVIDENCE RAIL, A SIBLING OF THE TABPANELS AND NEVER
            INSIDE ONE.
            ⚠ THIS IS THE WHOLE OF DW-281's DISCHARGE — an OWNER REQUEST, not a
            review finding: *"Later this button should be moved to the preview
            area."* Re-render and Save PDF used to sit inside
            `hidden={inspectorTab !== 'properties'}`, so selecting DATA took
            them out of the accessibility tree entirely. Rendering the rail here
            keeps every control, and the export's reason line, reachable
            whichever tab is selected. Putting it inside that tabpanel would
            inherit the defect and discharge nothing.
            The INPUTS tab keeps `ParameterEditor`; the DATA tab is untouched.

            `warningsOnScreen` IS THE SAME SET THE CARDS ARE, ASKED THE SAME WAY.
            `PreviewDiagnostics` hides a card whose key is dismissed; this counts
            what survives that filter through the one shared
            `diagnosticDismissalKey`, so the rail's header cannot claim warnings
            the author has nothing on screen to point at. */}
        {mode === 'preview' && <PreviewEvidenceRail
          render={preview ? { engineVersion: preview.version, target: RENDER_TARGET, pages: pdfExportStale ? undefined : previewPages, elapsedMs: preview.elapsedMs, sizeBytes: preview.bytes.byteLength } : undefined}
          hash={preview ? { digest: preview.digest, standIn: preview.standIn } : undefined}
          stale={pdfExportStale}
          warnings={preview ? preview.diagnostics.length : 0}
          warningsOnScreen={currentDiagnostics ? currentDiagnostics.diagnostics.filter((diagnostic, index) => !dismissedDiagnostics.has(diagnosticDismissalKey(diagnostic, index))).length : 0}
          errors={currentFailure ? 1 : 0}
          onRerender={() => void renderPreview(true)}
          exportLabel={pdfExportLabel}
          exportDisabled={Boolean(pdfExportUnavailable)}
          exportUnavailable={pdfExportUnavailable}
          onExport={() => void exportPreviewPdf()}
        />}
      </aside>
    </div>
    {/* STORY 14.7 — THE THREE READ-OUTS THE DIALOG GAINED ARE DERIVED HERE,
        FROM DATA THE BROWSER ALREADY HOLDS, and no projection field was added
        for any of them. The band is the table component's own `band`; the width
        the columns have is `band.width − table.x`, which is the engine's rule in
        `containComponent` read off the canvas projection rather than
        re-implemented; the item count is `SampleNode.count`. `undefined` on any
        of the three means UNKNOWN, and the dialog says so rather than drawing a
        zero. */}
    {tableEditor && <TableEditor projection={tableEditor} busy={tableEditorBusy} fileBusy={fileBusy} discarding={tableEditorDiscarding} error={tableEditorError} candidates={sampleCandidateScan.candidates} sampleAvailable={Boolean(sampleData)} band={canvas?.components.find((component) => component.id === tableEditor.table.tableId)?.band} availableWidth={tableEditorAvailableWidth} sampleItemCount={tableSampleItemCount(sampleData?.tree, tableEditor.table.collection)} onClose={closeTableEditor} onAdd={(index) => void commitTableColumn(addTableColumnCommand(tableEditor.table.tableId, index))} onRemove={(columnId) => void commitTableColumn(removeTableColumnCommand(tableEditor.table.tableId, columnId))} onMove={(columnId, index) => void commitTableColumn(moveTableColumnCommand(tableEditor.table.tableId, columnId, index))} onUpdate={(columnId, field, value) => commitTableColumn(updateTableColumnCommand(tableEditor.table.tableId, columnId, field, value))} onTotalWidth={(value) => commitTableColumn(tableWidthCommand(tableEditor.table.tableId, value))} onBinding={(columnId, binding) => commitTableColumn(updateTableColumnExpressionCommand(tableEditor.table.tableId, columnId, binding))} onConfigure={(collection, alias) => void commitTableColumn(configureTableBindingCommand(tableEditor.table.tableId, collection, alias))} onFooter={(columnId, footer, footerOf, footerFormat) => void commitTableColumn(updateTableColumnFooterCommand(tableEditor.table.tableId, columnId, footer, footerOf, footerFormat))} onHeaderHeight={(height) => void commitTableColumn(tableHeaderHeightCommand(tableEditor.table.tableId, height))} onAltRowBackground={(operation, value) => void commitTableColumn(tableAltRowBackgroundCommand(tableEditor.table.tableId, operation, value))} onHeaderStyle={(field, operation, value) => void commitTableColumn(tableHeaderStyleCommand(tableEditor.table.tableId, field, operation, value))} onMinHeight={(operation, value) => void commitTableColumn(tableMinHeightCommand(tableEditor.table.tableId, operation, value))} onRules={(field, operation, value) => void commitTableColumn(tableRulesCommand(tableEditor.table.tableId, field, operation, value))} onCellPadding={(field, operation, value) => void commitTableColumn(updateComponentPropertiesCommand([tableEditor.table.tableId], operation === 'clear' ? { field, operation } : { field, operation, value }))} editCount={tableEditorEditCount} onCancel={() => void cancelTableEditor()} />}
    {fontBrowserOpen && canvas && <FontBrowser sources={browsableFamilies} inTemplate={canvas.fontFamilies} heldLocalFamilies={heldLocalFamilies} previewBytes={browserSpecimenBytes} onAddFamily={(source) => addFamilyToDocument(source, documentGeneration.current, selected.join(','), 'caller')} storeKeepsFaces={storeKeepsFaces} onClose={() => setFontBrowserOpen(false)} />}
    {startupOpen && engine && <StartupDialog cards={startupCards} selected={startupSelected} busy={startupBusy} error={startupError} onSelect={(id) => { setStartupSelected(id); setStartupError(undefined) }} onConfirm={chooseStartup} onCancel={cancelStartup} onOpenFile={fileAccess ? requestStartupFile : undefined} />}
    {unsavedWarningOpen && <UnsavedChangesDialog document={title} onKeep={keepEditing} onDiscard={discardForNew} />}
    {offlineState === 'update-available' && (loadState?.mandatory === true || !updateDismissed) && <UpdateDialog version={loadState?.pendingVersion} mandatory={loadState?.mandatory === true} dirty={dirty} document={title} onLater={() => setUpdateDismissed(true)} onUpgrade={() => { void activatePendingRelease() }} onSave={(saveAs) => { void save(saveAs) }} />}
    {/* THE FONT COUNT, AND NOTHING ELSE NEW (Story 16.4). It is read off
        `canvas.fontFamilies`, which is `IN THIS TEMPLATE`'s own predicate, so
        the dropdown's first group and this line teach one model from one
        source and cannot drift apart.

        THE MOCKUP'S BINDING IS REFUSED, DELIBERATELY. `statusFontLine` counts
        `s.added.length` — the fonts added THIS SESSION — which is the
        session-scoped set this story forbids as a grouping key and would be no
        better here; and its else-branch is a hardcoded "3 fonts in template",
        which is placeholder data rather than a specification. No grid reading,
        no snap state and no selection content is added: those are three more
        claims about the canvas, and this story is not the place to make them. */}
    {/* STORY 13.5 — IN PREVIEW THE BAR STATES THE PRODUCT'S STANDING PROMISE,
        and it pays for the room by dropping exactly two Design-mode items.
        `LOCAL SHELL` goes because the assurance line says in words what that
        shorthand says in two; the template font count goes because it is a fact
        about the TEMPLATE and not about the render, and in Preview the frame
        should be describing the render. It is not relocated and nothing else
        carries it: the evidence rail states engine, target, pages, elapsed,
        size, the output hash and the diagnostics, and NO font fact at all
        (population searched: `preview/evidence-rail.tsx` and
        `preview/evidence-rail-facts.ts` entire, `grep -ain font` — zero hits).
        The count is simply not shown in Preview, and it returns the moment the
        author is back in Design, where it is about the thing on screen.
        NOTHING ELSE IS DROPPED: `engine-snapshot`, the offline live region and
        `PREVIEW MODE` all stay, and DESIGN MODE'S BAR IS UNTOUCHED — every
        fence here is on `mode`, so none of it can leak out of Preview.

        THE OFFLINE LIVE REGION IS VISUALLY HIDDEN IN PREVIEW, NOT REMOVED.
        `.sr-only` (`App.css:7`, and this is its first use anywhere) takes it
        out of the painted bar while `role="status"`, `aria-live="polite"`, its
        label, its testid and its full announcement text are every one of them
        untouched — so a screen reader still receives all five `offlineLabel`
        states, including 'Update available; current release remains usable' and
        'Offline cache unavailable', which can arrive while an author sits in
        Preview. Because `.sr-only` is `position: absolute` the span stops being
        a flex item, so it contributes neither width nor a `gap`, and the bar's
        spare room stops varying by the 24 characters that separate the longest
        offline label from the shortest. In Design the span carries no class at
        all and renders exactly as it always has.

        THE ASSURANCE IS NON-INTERACTIVE AND LAST. Non-interactive because the
        bar's `button, input, select` set is pinned exhaustively by name; last
        because that is where the design puts it, and because it makes the
        overflow assertion in `e2e/preview-navigation.spec.ts` mean something:
        if the bar ever stops fitting, this is the element pushed past the
        bar's right edge. */}
    <footer className="status-bar" aria-label="Status bar">{mode === 'design' && <span>LOCAL SHELL</span>}<code data-testid="engine-snapshot">{engineLabel}</code><span className="status-spacer" />{mode === 'design' && canvas && <span data-testid="template-font-count">{`${canvas.fontFamilies.length} font${canvas.fontFamilies.length === 1 ? '' : 's'} in template`}</span>}{/* STORY 14.6 / AC8 — HOW MUCH OF THE DOCUMENT IS BOUND, stated nowhere before
        this. "Bound" is `binding` OR `tableBind` (owner ruling, 2026-09-09): a
        table bound to a collection IS bound, and `tableBind` is a different
        field from `binding` in `page_setup.go`. The denominator is
        `canvas.components`, which `canvasComponents` builds from pageHeader +
        content + pageFooter, flat and once each regardless of page count.
        ⚠ THE EXCLUSIONS READ AS SURPRISING AND ARE CORRECT: `directCanvasBinding`
        populates `binding` only for a whole-value, single, non-reserved path
        placeholder, so "Total: {{amount}}", "{{amount}} THB",
        "{{upper(customer.name)}}" and "{{page}}" all count as UNBOUND.
        At zero elements the span is not rendered at all — "0 of 0 elements
        bound" is noise on an empty template, not information. */}{mode === 'design' && canvas && canvas.components.length > 0 && <span data-testid="bound-element-count">{`${canvas.components.filter((component) => component.binding !== undefined || component.tableBind !== undefined).length} of ${canvas.components.length} element${canvas.components.length === 1 ? '' : 's'} bound`}</span>}<span role="status" aria-live="polite" aria-label="Offline availability" data-testid="offline-status" className={mode === 'preview' ? 'sr-only' : undefined}>{offlineLabel}</span><code>{mode.toUpperCase()} MODE</code>{mode === 'preview' && <span data-testid="local-only-assurance">no network · nothing left this machine</span>}</footer>
  </div>
}

function ParameterEditor({ referenceState, accepted, draft, error, onDraft, onNamedValue }: { referenceState: ParameterReferenceState; accepted: string; draft: string; error?: string; onDraft: (value: string) => void; onNamedValue: (name: string, value: string) => void }) {
  const values = parameterValues(accepted)
  const references = referenceState.names
  return <><p className="panel-heading">Runtime parameters</p>{referenceState.status === 'pending' ? <p className="honest-note" role="status">Discovering parameter references from the local engine…</p> : referenceState.status === 'failed' ? <p className="file-message" role="alert">The local engine could not provide parameter references. The raw parameter document is still available.</p> : references.length > 0 ? <div aria-label="Engine-discovered parameter references">{references.map((name) => <ParameterValueInput key={name} name={name} acceptedValue={values[name]} onAccept={onNamedValue} />)}</div> : <p className="honest-note">The local engine found no parameter references in this template.</p>}<label>Raw parameter JSON<textarea aria-label="Raw parameter JSON" aria-invalid={Boolean(error)} aria-describedby={error ? 'parameter-input-error' : undefined} value={draft} onChange={(event) => onDraft(event.target.value)} /></label>{error && <p id="parameter-input-error" role="alert" className="file-message">{error}. The last accepted parameter document remains in Preview.</p>}</>
}

function ParameterValueInput({ name, acceptedValue, onAccept }: { name: string; acceptedValue?: string; onAccept: (name: string, value: string) => void }) {
	const accepted = acceptedValue ?? ''
	return <label>params.{name}<input aria-label={`Value for params.${name}`} value={accepted} onChange={(event) => onAccept(name, event.target.value)} /></label>
}

function parameterValues(raw: string): Record<string, string> {
  const values = Object.create(null) as Record<string, string>
  for (const property of topLevelJSONProperties(raw) ?? []) values[property.name] = raw.slice(property.valueStart, property.valueEnd)
  return values
}

type JSONPropertySpan = Readonly<{ name: string; valueStart: number; valueEnd: number }>

// This is intentionally a token locator, not a template parser or a second
// parameter schema. JSON.parse remains the acceptance authority; these spans
// merely let a named control replace one top-level raw JSON value without
// rewriting numeric lexemes, whitespace, unrelated keys, or special keys.
function topLevelJSONProperties(raw: string): ReadonlyArray<JSONPropertySpan> | undefined {
  let cursor = skipJSONWhitespace(raw, 0)
  if (raw[cursor++] !== '{') return undefined
  const properties: JSONPropertySpan[] = []
  cursor = skipJSONWhitespace(raw, cursor)
  if (raw[cursor] === '}') return properties
  while (cursor < raw.length) {
    const keyStart = cursor
    const keyEnd = scanJSONString(raw, cursor)
    if (keyEnd === undefined) return undefined
    let name: unknown
    try { name = JSON.parse(raw.slice(keyStart, keyEnd)) } catch { return undefined }
    if (typeof name !== 'string') return undefined
    cursor = skipJSONWhitespace(raw, keyEnd)
    if (raw[cursor++] !== ':') return undefined
    cursor = skipJSONWhitespace(raw, cursor)
    const valueStart = cursor
    const valueEnd = scanJSONValue(raw, cursor)
    if (valueEnd === undefined) return undefined
    properties.push({ name, valueStart, valueEnd })
    cursor = skipJSONWhitespace(raw, valueEnd)
    if (raw[cursor] === '}') return properties
    if (raw[cursor++] !== ',') return undefined
    cursor = skipJSONWhitespace(raw, cursor)
  }
  return undefined
}

function replaceTopLevelJSONValue(raw: string, name: string, value: string): string | undefined {
  const properties = topLevelJSONProperties(raw)
  if (!properties) return undefined
  // Duplicate JSON keys resolve last-wins in the production decoder; retain
  // that same value while preserving every other occurrence byte-for-byte.
  const existing = properties.filter((property) => property.name === name).at(-1)
  if (existing) return raw.slice(0, existing.valueStart) + value + raw.slice(existing.valueEnd)
  const close = skipJSONWhitespace(raw, properties.length === 0 ? 1 : properties.at(-1)!.valueEnd)
  const beforeClose = raw.indexOf('}', close)
  if (beforeClose < 0) return undefined
  return raw.slice(0, beforeClose) + `${properties.length > 0 ? ',' : ''}${JSON.stringify(name)}:${value}` + raw.slice(beforeClose)
}

function skipJSONWhitespace(raw: string, cursor: number): number {
  while (cursor < raw.length && /[\t\n\r ]/.test(raw[cursor]!)) cursor++
  return cursor
}

function scanJSONString(raw: string, cursor: number): number | undefined {
  if (raw[cursor] !== '"') return undefined
  for (cursor++; cursor < raw.length; cursor++) {
    if (raw[cursor] === '\\') { cursor++; continue }
    if (raw[cursor] === '"') return cursor + 1
  }
  return undefined
}

function scanJSONValue(raw: string, cursor: number): number | undefined {
  const first = raw[cursor]
  if (first === '"') return scanJSONString(raw, cursor)
  if (first === '{' || first === '[') {
    const close = first === '{' ? '}' : ']'
    cursor = skipJSONWhitespace(raw, cursor + 1)
    if (raw[cursor] === close) return cursor + 1
    while (cursor < raw.length) {
      if (first === '{') {
        const keyEnd = scanJSONString(raw, cursor)
        if (keyEnd === undefined) return undefined
        cursor = skipJSONWhitespace(raw, keyEnd)
        if (raw[cursor++] !== ':') return undefined
        cursor = skipJSONWhitespace(raw, cursor)
      }
      const valueEnd = scanJSONValue(raw, cursor)
      if (valueEnd === undefined) return undefined
      cursor = skipJSONWhitespace(raw, valueEnd)
      if (raw[cursor] === close) return cursor + 1
      if (raw[cursor++] !== ',') return undefined
      cursor = skipJSONWhitespace(raw, cursor)
    }
    return undefined
  }
  const token = raw.slice(cursor).match(/^(?:true|false|null|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?)/)?.[0]
  return token ? cursor + token.length : undefined
}

function PageSetup({ preset, orientation, draft, onPreset, onOrientation, onDraft, onApply, disabled }: { preset: string; orientation: string; draft: Draft; onPreset: (value: string) => void; onOrientation: (value: string) => void; onDraft: (key: keyof Draft, value: string) => void; onApply: () => void; disabled: boolean }) {
  return <section className="property-section property-section-page-setup"><p className="section-label">PAGE SETUP</p><p className="honest-note">Component properties require a selection.</p><div className="property-grid"><label>Preset<select aria-label="Page preset" value={preset} onChange={(event) => onPreset(event.target.value)}><option value="A4">A4</option><option value="Letter">Letter</option><option value="custom">Custom</option></select></label><label>Orientation<select aria-label="Page orientation" value={orientation} onChange={(event) => onOrientation(event.target.value)}><option value="portrait">Portrait</option><option value="landscape">Landscape</option></select></label><label>Locale<select aria-label="Document locale" value={draft.locale} onChange={(event) => onDraft('locale', event.target.value)}>{draft.locale === '' && <option value="" disabled>Not set</option>}{LOCALE_TAGS.map((tag) => <option key={tag} value={tag}>{tag}</option>)}</select></label><Field label="UTC offset (±HH:MM)" value={draft.utcOffset} inputMode="text" onChange={(value) => onDraft('utcOffset', value)}/></div>{preset === 'custom' && <><Field label="Width (pt)" value={draft.width} onChange={(value) => onDraft('width', value)}/><Field label="Height (pt)" value={draft.height} onChange={(value) => onDraft('height', value)}/></>}<div className="property-grid"><Field label="Top margin (pt)" value={draft.top} onChange={(value) => onDraft('top', value)}/><Field label="Right margin (pt)" value={draft.right} onChange={(value) => onDraft('right', value)}/><Field label="Bottom margin (pt)" value={draft.bottom} onChange={(value) => onDraft('bottom', value)}/><Field label="Left margin (pt)" value={draft.left} onChange={(value) => onDraft('left', value)}/></div>{(draft.pageHeader !== undefined || draft.pageFooter !== undefined) && <div className="property-grid">{draft.pageHeader !== undefined && <Field label="Page header height (pt)" value={draft.pageHeader} onChange={(value) => onDraft('pageHeader', value)}/>}{draft.pageFooter !== undefined && <Field label="Page footer height (pt)" value={draft.pageFooter} onChange={(value) => onDraft('pageFooter', value)}/>}</div>}<button type="button" className="file-button" onClick={onApply} disabled={disabled}>Apply page setup</button><p className="honest-note">Grid and snap are editor preferences; document undo is available in the document bar.</p></section>
}

type PanelComponent = CanvasProjection['components'][number]
// STORY 14.2. `fields` IS A SET BECAUSE AN INTENT MAY NOW CARRY TWO.
//
// It records what the PANEL sent, which is what anchors the refusal beside the
// control that sent it. It is deliberately not the engine's returned path:
// `propertyPath` (`component_commands.go`) answers with the FIRST key in
// canonical order rather than the key that actually failed, so on a
// `{width, height}` intent refused by `height` it says `component.width`.
// That is why `dataPath` is suppressed for a multi-field intent at the render
// site below, and why the anchor is this set instead.
type PropertyCommitError = Readonly<{ fields: ReadonlyArray<PropertyField>; selectionKey: string; elementId?: string; dataPath?: string; message: string }>
const intentFields = (intent: PropertyIntent | PropertyIntents): ReadonlyArray<PropertyField> => 'field' in intent ? [intent.field] : intent.map((one) => one.field)
type CommitProperties = (ids: ReadonlyArray<string>, intent: PropertyIntent | PropertyIntents, generation: number, key: string) => Promise<CanvasProjection | undefined>
// Panel sections mirror the UX design's inspector: an identity row, then
// POSITION / CONTENT / TYPOGRAPHY / BOX / BINDING. Each field keeps its exact
// engine field name and accessible label; only the presentation is grouped.
// `empty` is the engine's behaviour when the field carries no committed value —
// no border, no padding, no fill, always visible. It is shown as a placeholder,
// never as a value: the field stays empty and nothing is written to the
// document until the author types.
// `fx` marks a field the engine will read as an expression rather than as a
// literal, and says which of the two spellings it accepts — so the cue is on
// the exact fields that accept one, and never on a field where Go rejects a
// placeholder outright.
type FieldExpression = 'placeholder' | 'condition'
// `empty` is what the row says when it holds NOTHING — 'none', 'black',
// 'always', a grey word standing in for behaviour the document does not
// author. STORY 17.3 adds `shown`, which says that string is not a stand-in at
// all but the ENGINE'S OWN EFFECTIVE VALUE for this field, so the box carries
// it as real text the author can read, step and commit. Only `fontSize` and
// `lineSpacing` set it, and both take their string from the projection — never
// from a literal in this file.
type FieldSpec = Readonly<{ field: PropertyField; label: string; affix?: string; unit?: string; swatch?: true; prose?: true; lines?: number; empty?: string; shown?: true; fx?: FieldExpression }>
const fxHint: Readonly<Record<FieldExpression, string>> = { placeholder: 'Accepts literal text, or {{ }} expressions', condition: 'Accepts a boolean or null formula, e.g. loanAmount > 20000, written without {{ }}' }
// Where the fx cue sends a reader: the section of the expression reference
// that governs THIS kind of field, not the top of the page.
const fxAnchor: Readonly<Record<FieldExpression, string>> = { placeholder: 'paths', condition: 'formulas' }
// A condition field IS the expression, so any text in it is one; a text field
// holds an expression only where a placeholder is spelled.
function holdsExpression(fx: FieldExpression, text: string): boolean { return fx === 'placeholder' ? containsPlaceholder(text) : text !== '' }
const positionFields: ReadonlyArray<FieldSpec> = [{ field: 'x', label: 'X (pt)', affix: 'X', unit: 'pt' }, { field: 'y', label: 'Y (pt)', affix: 'Y', unit: 'pt' }]
const sizeFields: ReadonlyArray<FieldSpec> = [{ field: 'width', label: 'Width (pt)', affix: 'W', unit: 'pt' }, { field: 'height', label: 'Height (pt)', affix: 'H', unit: 'pt' }]
// STORY 14.2 — A LINE IS A THICKNESS AND A COLOUR, AND THE PANEL NOW SAYS SO.
//
// A Line is drawn as a very short, very wide filled box, which is how the
// engine models it — `internal/template/parse_bands.go` handles `ElementLine`
// and `ElementRect` with "no extra fields", the format spec says both are
// drawn from `style.border` and `style.background`, and
// `element_box_test.go` states outright that "a rule's declared height is its
// thickness". Height IS thickness, width IS length, background IS colour,
// already, in the engine. Until this story the panel spelled all three in the
// engine's implementation terms — `H`, `W`, `Background` — so drawing a
// hairline required knowing it is a filled box.
//
// The inspector vocabulary below is a relabel. `field` is untouched in every spec
// below, so every one of these controls writes exactly the key it wrote
// before, through the same `updateComponentProperties`. `affix` is the visible
// word; `label` is the accessible name and is never rendered. No new
// `PropertyField`, no new command kind, no new serialized key — and the wire
// bytes for the existing inspector edits are byte-identical, which
// `line-rect-vocabulary.test.tsx` asserts against literal JSON rather than
// claiming here.
type LineOrientation = 'horizontal' | 'vertical'
// DERIVED FROM THE COMMITTED BOX, READ-ONLY, AND NEVER LATCHED. There is no
// orientation in the document and this story may not add one: a stored
// orientation would be a new serialized key. So the panel reads the shape it
// was given. TIES READ HORIZONTAL — a square "line" is degenerate and one of
// the two answers has to be chosen; horizontal is the one a rule is drawn as
// by default (`lineDropHeight = 1000` against a much wider drop).
//
// ⚠ AND BECAUSE IT IS DERIVED, IT MOVES WHEN THE BOX MOVES. Setting Thickness
// above Length re-reads the shape as vertical and the two labels swap over the
// two values. That is accepted and stated rather than prevented — see the test
// that pins it — because preventing it needs stored state.
function lineOrientation(component: PanelComponent): LineOrientation { return component.height <= component.width ? 'horizontal' : 'vertical' }
// Length is the long axis, Thickness the short one. `draftFor` keys the
// rendered draft on `spec.field`, so a flip REORDERS these two rows and
// relabels them; it never remounts either, and neither loses its committed
// value.
function lineSizeFields(orientation: LineOrientation): ReadonlyArray<FieldSpec> {
  const along: PropertyField = orientation === 'horizontal' ? 'width' : 'height'
  const across: PropertyField = orientation === 'horizontal' ? 'height' : 'width'
  return [{ field: along, label: 'Length (pt)', affix: 'Length', unit: 'pt' }, { field: across, label: 'Thickness (pt)', affix: 'Thickness', unit: 'pt' }]
}
// One CONTENT field, not two. Go keeps two commands behind the same
// element.Value — `value` rejects a placeholder, `expression` requires one —
// and splitting the panel along that seam made the author pick the command
// before typing, with the same committed text showing in both rows. The field
// routes on what was actually typed instead; the engine's two guards, and the
// two spellings they accept, are unchanged.
// `prose` is what makes this a TEXTAREA rather than an <input type="text">,
// which cannot hold a line feed at all — the whole of Story 7.4's first AC.
// The flag was declared on FieldSpec long before anything set it; this is the
// field it was anticipated for, and it stays the only one that sets it.
const contentField: FieldSpec = { field: 'value', label: 'Text', affix: 'Text', prose: true, fx: 'placeholder' }
// spec-barcode-qr-elements CAP-5. A barcode's or QR code's content is shown
// wrapped over five rows and commits as the Text field does, after a pause.
// A new line (Enter) is a carriage return, the payment payload's field
// separator; `\n` and `\\` are escapes. Go projects and decodes both.
const barcodeContentField: FieldSpec = { field: 'value', label: 'Content', affix: 'Content', lines: 5, fx: 'placeholder' }
// A QR code's content is the same one-line field with the same escapes; its one
// option is the error-correction level, a closed set Go validates. Pressing the
// current level again clears the key, which is the default M.
const errorCorrectionSegments: ReadonlyArray<SegmentSpec> = [{ value: 'L', label: 'Error correction L (7%)', content: 'L' }, { value: 'M', label: 'Error correction M (15%)', content: 'M' }, { value: 'Q', label: 'Error correction Q (25%)', content: 'Q' }, { value: 'H', label: 'Error correction H (30%)', content: 'H' }]
// STORY 17.1: THE CANVAS FOLLOWS THE CONTENT FIELD, AFTER A PAUSE — NOT PER
// KEYSTROKE, AND THE DIFFERENCE IS NOT A DETAIL.
//
// AD-17 forbids the browser from measuring or breaking text, so a new line
// break exists only once the ENGINE has returned a new `textPaint`. There is
// therefore no local preview to paint: every update the author sees is a real
// round trip carrying a real `updateComponentProperties` command, with its own
// revision and its own undo entry. One per keystroke would be one command per
// keystroke. The owner took that trade at the gate and declined the three
// costlier options (a non-committing preview op among them), so what this
// number buys is stated plainly: the canvas LAGS typing by it, and nothing in
// this story may claim the canvas paints as you type.
//
// It is deliberately SHORTER than the PDF preview's own debounce: the canvas is
// the thing being typed into, the preview is not. That ordering is pinned by an
// assertion over the two constants rather than by a number written out here,
// which would go on claiming a relationship after either one moved.
export const PROSE_COMMIT_DEBOUNCE_MS = 200
// STORY 17.5. THE FOUR-ROW FLOOR, IN THE ONE PLACE THE DRAG CAN READ IT.
// `App.css`'s `textarea.property-value-prose { min-height: 72px }` is the
// RESTING floor and this is the DRAG's; they are the same number twice because
// no DOM measurement is available to derive one from the other
// (`canvas-authority-contract.test.ts` prohibits `getComputedStyle` and every
// layout box property across this repository). The pair is held together by a
// source-text assertion in `property-prose-height.test.ts` rather than by
// hoping the two copies are noticed together.
const PROSE_MIN_HEIGHT_PX = 72
function containsPlaceholder(text: string): boolean { return text.includes('{{') || text.includes('}}') }
function contentCommand(field: PropertyField, text: string): PropertyField { return field === 'value' && containsPlaceholder(text) ? 'expression' : field }
// TYPOGRAPHY is laid out as Main.dc.html draws it: the family row spans the
// panel, the size sits beside the B/I pair, and align/valign are the design's
// two segmented controls instead of free-text fields. Both are closed sets Go
// already validates, and the control offers exactly the values ITS OWN
// selection accepts and nothing else.
//
// That is no longer one list. Since Story 7.3 `style.align` admits FOUR
// values for text — left/center/right/justify — while a table's cells draw a
// justified value at the start edge, so it means nothing there. The align
// segments are therefore derived per selection in ComponentProperties, not a
// module constant; valign stays the one triple top/middle/bottom.
const fontSizeField: FieldSpec = { field: 'fontSize', label: 'Font size (pt)', unit: 'pt' }
// Story 7.4. A dimensionless ratio, shown in the author's own units: the
// engine carries thousandths and `points` already divides by 1000, so 1500
// reads back as "1.5".
//
// STORY 17.3 TOOK THE `'1'` OUT OF THIS LINE. The neutral ratio is the
// ENGINE'S number — `defaultLineSpacing` in render.go, which is
// template.LineSpacingUnit — and spelling it here made the designer a second
// authority on it: if the engine's default ever moved, this string would have
// gone on claiming the old one and nothing would have reddened. It is now
// projected (CanvasProjection.defaultLineSpacing) and supplied at the render
// site, exactly as `defaultFontSize` already was for the size beside it.
const lineSpacingField: FieldSpec = { field: 'lineSpacing', label: 'Line spacing', affix: 'Leading' }
// Story 10.1: the ink, in TYPOGRAPHY where the rest of the type lives —
// it colours the glyphs, not the box, so it belongs beside the family and
// the size rather than beside Background. Empty is the engine's own
// behaviour with no colour declared: the PDF's initial fill, black.
const colorField: FieldSpec = { field: 'color', label: 'Text colour', affix: 'Colour', swatch: true, empty: 'black' }
// BOX: a Border row, the edge set, then the Background and Visibility rows the
// design shows as label-and-value. No padding rows, per D-12.4.1 in the epic
// 11-14 decision log: style.padding stays an engine property a loaded document
// keeps and renders, and Go's command layer refuses it off a table. The owner's
// 2026-09-13 revision lets the TABLE EDITOR author a table's own left/right
// padding; the inspector still offers none.
const borderFields: ReadonlyArray<FieldSpec> = [{ field: 'borderWidth', label: 'Border width (pt)', affix: 'Border', unit: 'pt', empty: 'none' }, { field: 'borderColor', label: 'Border colour', affix: 'Border colour', swatch: true, empty: 'none' }]
const backgroundField: FieldSpec = { field: 'background', label: 'Background', affix: 'Background', swatch: true, empty: 'none' }
// STORY 14.2. THE SAME FIELD, SPELLED FOR THE KIND IN FRONT OF THE AUTHOR.
//
// `field` stays `background` in all three spellings, so the wire bytes are
// identical whichever word is on screen. Relabelling `label` renames up to
// four accessible names at once — the box, `Pick ${label}`, `Clear ${label}`
// and `Set ${label} null` — which is why the control-vocabulary census had to
// grow a Line and a Rectangle state before this landed.
//
// ⚠ GATED ON A SINGLE SELECTION. A mixed selection keeps `Background`: one
// command goes to every id in it, and no one word is true of two kinds.
// `empty` WITHOUT `shown`. `shown` puts the string in the BOX, where blur
// commits it (`fontSize` and `lineSpacing` do this deliberately, with the
// engine's own projected default). An invented default here would let
// selecting a Line and tabbing through it write the document.
function boxFillFieldFor(type: PanelComponent['type'] | undefined): FieldSpec {
  if (type === 'line') return { ...backgroundField, label: 'Colour', affix: 'Colour' }
  if (type === 'rect') return { ...backgroundField, label: 'Fill', affix: 'Fill' }
  return backgroundField
}
// D-14.2.Q1. WHAT THE WITHHELD BORDER CONTROLS LEAVE BEHIND, AND ITS LIMITS.
//
// A border on a Line PAINTS — `elementBoxDeclaration` is kind-agnostic and
// `borderPaints` returns true for a present, non-null, non-empty edge set. So
// hiding the controls hides a property that draws ink, and the honest answer
// is to disclose it rather than let it vanish. Hiding costs discoverability,
// not preservation: `PropertyDraft` writes nothing on mount, so a control this
// panel withholds never removes the value the document carries.
//
// ⚠ IT REPORTS THE PROJECTION, NOT THE PDF, and two known cases sit outside
// what it can see: a border that paints no ink projects nothing, and an
// all-edges border declared as `{}` (DW-145) prints while the canvas shows
// nothing. Both are out of this story's scope. The note therefore says what
// the panel knows and claims no more.
function borderProjected(component: PanelComponent): boolean {
  return component.borderWidth !== undefined || component.borderColor !== undefined || component.borderEdges !== undefined
}
const visibilityField: FieldSpec = { field: 'visibleIf', label: 'Visible if', affix: 'Visibility', empty: 'always', fx: 'condition' }
// STORY 14.7 — THE ALIGNMENT SEGMENTS, THE GLYPHS AND THE CONTROL ITSELF NOW
// LIVE IN `segmented-control.tsx`, so the table editor renders literally this
// control rather than a second spelling of it. The widening below is unchanged
// and still happens HERE, where the selection's types are known.
// STORY 14.1 / AC3 — VERTICAL ALIGN IS SPELLED THE WAY HORIZONTAL ALIGN IS.
// These two segmented controls sit side by side in ONE `.property-grid` at
// `1fr 1fr` (App.css:281), rendered by ONE `SegmentedProperty` through ONE
// `.property-segment` class, and until now one of them drew icons while the
// other drew the words TOP / MID / BOT. That is rule V3 in this story's spec —
// (a) two controls of one class disagreeing, and (b) a row mixing glyphs and
// words at the same size — and it is exactly the seam a glyph-versus-word rule
// exists to close.
//
// The glyphs are written in `AlignIcon`'s idiom on purpose: the same
// `.segment-icon` class, the same 16px `viewBox`, the same `strokeWidth`, the
// same `aria-hidden`, and INLINE — never a `.svg` on disk, because the offline
// release's cache manifest counts emitted files and `vite.config.ts` sets
// `assetsInlineLimit: 0` (D-14.0.1). The long stroke is the edge the content
// aligns to and the two short strokes are the content; `label` is untouched, so
// each segment's accessible name is byte-identical to the one it had (AC4).
type ValignVariant = 'top' | 'middle' | 'bottom'
const valignGlyphs: Readonly<Record<ValignVariant, string>> = { top: 'M2 3h12M4 7h8M4 10h8', middle: 'M4 4h8M2 8h12M4 12h8', bottom: 'M4 6h8M4 9h8M2 13h12' }
function ValignIcon({ variant }: { variant: ValignVariant }) {
  return <svg aria-hidden="true" className="segment-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2"><path d={valignGlyphs[variant]} /></svg>
}
const valignSegments: ReadonlyArray<SegmentSpec> = [{ value: 'top', label: 'Vertical align top', content: <ValignIcon variant="top" /> }, { value: 'middle', label: 'Vertical align middle', content: <ValignIcon variant="middle" /> }, { value: 'bottom', label: 'Vertical align bottom', content: <ValignIcon variant="bottom" /> }]
function PropertySection({ title, tone, children }: { title: string; tone?: 'bind'; children: ReactNode }) {
  return <section className={`property-section property-section-${title.toLowerCase()}${tone === 'bind' ? ' property-section-bind' : ''}`}><p className="section-label">{title}</p>{children}</section>
}
function ComponentProperties({ components, fontFamilies, fontChains, carriedFaces, specimenBytes, defaultFontSize, defaultLineSpacing, onCommit, onUseFamily, onDeclareFamily, onOpenFontBrowser, browserOpen, storedFaces, heldLocalFamilies, onFamilyListOpened, fontChainError, fontChainBusy, documentGeneration, propertyError, drag, groupPreview, onEditTable, onPickImage, imageAvailable, assetBusy, assetError }: { components: ReadonlyArray<PanelComponent>; fontFamilies: ReadonlyArray<string>; fontChains: CanvasProjection['fontChains']; carriedFaces: ReadonlySet<string>; specimenBytes: PreviewFaceBytes; defaultFontSize: number; defaultLineSpacing: number; onCommit: CommitProperties; onUseFamily: (source: FamilySource) => Promise<string | undefined>; onDeclareFamily: (source: FamilySource) => Promise<string | undefined>; onOpenFontBrowser: () => void; browserOpen: boolean; storedFaces: ReadonlyArray<StoredFace>; heldLocalFamilies: ReadonlySet<string>; onFamilyListOpened: () => void; fontChainError?: FontChainCommitError; fontChainBusy: boolean; documentGeneration: number; propertyError?: PropertyCommitError; drag?: DragState; groupPreview?: GroupPreview; onEditTable: (id: string) => void; onPickImage: (id: string) => void; imageAvailable: boolean; assetBusy: boolean; assetError?: Readonly<{ id: string; message: string }> }) {
  const ids = components.map((component) => component.id)
  const types = new Set(components.map((component) => component.type))
  const all = (predicate: (type: PanelComponent['type']) => boolean) => [...types].every(predicate)
  const single = components.length === 1 ? components[0]! : undefined
  const scopedError = propertyError?.selectionKey === ids.join(',') ? propertyError : undefined
  const scopedChainError = fontChainError?.selectionKey === ids.join(',') ? fontChainError : undefined
  const table = single?.type === 'table' ? single : undefined
  // STORY 14.2. The per-kind vocabulary is gated on a SINGLE selection, in the
  // idiom `table` and `image` already use, and for the same reason: a mixed
  // selection has no one kind to speak for, and `App.test.tsx`'s text+rect case
  // asserts it still reads `Width (pt)`.
  const line = single?.type === 'line' ? single : undefined
  const image = single?.type === 'image' ? single : undefined
  // STORY 14.4 / AC1 (D-14.4.Q1: HIDE). The BINDING section holds no control
  // and can never hold a value for a Line, a Rectangle, an Image or a Table:
  // Go writes `component.Binding` at exactly one site, inside
  // `if element.Type == template.ElementText`, so the bind-chip branch is
  // unreachable for those kinds and hiding the section suppresses NO value.
  // What it removes is an invitation to attempt something the engine refuses.
  // A MULTI-SELECTION still shows it — there is no one kind to speak for, and
  // its existing 'shown for one selected component' sentence is unchanged.
  // The kind test reads the mirrored constant rather than a fourth spelling
  // of `=== 'text'`.
  const scalarBindable = single !== undefined && SCALAR_BINDING_COMPONENT_TYPES.includes(single.type)
  const typographic = all((type) => type === 'text' || type === 'table')
  // FOUR segments for an all-text selection, THREE for anything carrying a
  // table. SegmentedProperty never sees component.type — the widening is a
  // derivation here, where the selection's types are already known.
  const alignChoices = all((type) => type === 'text') ? [...alignSegments, justifySegment] : alignSegments
  // A drag is the same transient local proposal the canvas is already
  // painting, shown in the same units. It is never committed from here; the
  // pointer release sends the one command and Go's accepted geometry then
  // replaces it through the committed value below.
  const dragging = single && drag?.id === single.id ? drag : undefined
  const live = (field: PropertyField): string | undefined => {
    if (field !== 'x' && field !== 'y' && field !== 'width' && field !== 'height') return undefined
    if (groupPreview) { const values = components.map((component) => component[field] + (field === 'x' ? groupPreview.dx : field === 'y' ? groupPreview.dy : 0)); return values.every((value) => value === values[0]) ? points(values[0]!) : '' }
    return dragging ? points(dragging[field]) : undefined
  }
  // The CONTENT field sends `value` or `expression` depending on the typed
  // text, so it owns the rejection of either command.
  // STORY 14.2 turned the equality into a MEMBERSHIP, and nothing else moved.
  // A single-field intent still records one field and still matches exactly the
  // control that sent it; an orientation intent records `width` and `height`,
  // so `errorFor('width')` and `errorFor('height')` both resolve for the one
  // refusal and it is anchored on both rows it moved.
  const errorFor = (field: PropertyField) => scopedError && (scopedError.fields.includes(field) || (field === 'value' && scopedError.fields.includes('expression'))) ? scopedError : undefined
  const draftFor = (spec: FieldSpec) => <PropertyDraft key={spec.field} spec={spec} components={components} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} live={live(spec.field)} error={errorFor(spec.field)} />
  // STORY 11.3 / F1 — THE CUT EACH TOGGLE WOULD REQUIRE, AND WHETHER THE CHAIN
  // DECLARES IT. Deduplicated, so the combined cut — which implicates BOTH
  // controls, because marking only one implies the other is fine — states its
  // reason ONCE for the pair. Two DIFFERENT missing cuts still state two
  // sentences: they are two different facts.
  const missingBoldCut = selectionMissingCut(components, 'bold', fontChains)
  const missingItalicCut = selectionMissingCut(components, 'italic', fontChains)
  const absentCuts = [...new Set([missingBoldCut, missingItalicCut].filter((cut): cut is StyleCut => cut !== undefined))]
  return <>
    <div className="component-identity">{single ? <PaletteIcon kind={single.type} /> : undefined}<span className="component-identity-name">{single ? single.type : `${components.length} selected`}</span><span className="component-identity-meta">{single ? `${single.id} · band: ${single.band}` : [...types].join(' · ')}</span></div>
    <PropertySection title="POSITION"><div className="property-grid">{positionFields.map(draftFor)}{all((type) => type !== 'table') && (line ? lineSizeFields(lineOrientation(line)) : sizeFields).map(draftFor)}</div>{line && <OrientationProperty component={line} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} error={errorFor('width') && errorFor('height') ? scopedError : undefined} />}</PropertySection>
    {single && types.has('text') && <PropertySection title="CONTENT">{draftFor(contentField)}<p className="honest-note">Literal text, or {'{{ }}'} placeholders for data.</p></PropertySection>}
    {single && types.has('qrcode') && <PropertySection title="CONTENT">{draftFor(barcodeContentField)}<p className="honest-note">Any text, UTF-8. Literal text or {'{{ }}'} placeholders; a new line (Enter) is a carriage return; type \n for a line feed, \\ for a backslash.</p><SegmentedProperty label="Error correction" field="errorCorrection" segments={errorCorrectionSegments} components={components} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} error={errorFor('errorCorrection')} /><p className="honest-note">Higher levels survive more damage and need more modules in the same box. None selected is M.</p></PropertySection>}
    {single && types.has('barcode') && <PropertySection title="CONTENT">{draftFor(barcodeContentField)}<p className="honest-note">Code 128, ASCII only. Literal text or {'{{ }}'} placeholders; a new line (Enter) is a carriage return; type \n for a line feed, \\ for a backslash.</p></PropertySection>}
    {typographic && <PropertySection title="TYPOGRAPHY"><FontFamilyProperty families={fontFamilies} fontChains={fontChains} carriedFaces={carriedFaces} specimenBytes={specimenBytes} components={components} ids={ids} onCommit={onCommit} onUseFamily={onUseFamily} onDeclareFamily={onDeclareFamily} onOpenFontBrowser={onOpenFontBrowser} browserOpen={browserOpen} storedFaces={storedFaces} heldLocalFamilies={heldLocalFamilies} onFamilyListOpened={onFamilyListOpened} pickBusy={fontChainBusy} pickError={scopedChainError?.control.action === 'embed' ? scopedChainError : undefined} documentGeneration={documentGeneration} error={errorFor('fontFamily')} /><div className="property-size-row">{draftFor({ ...fontSizeField, empty: points(defaultFontSize), shown: true })}<div className="property-toggles"><div className="property-toggle-row"><BooleanProperty label="Bold" field="bold" components={components} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} error={errorFor('bold')} absentCutId={missingBoldCut && cutAbsenceId(missingBoldCut)} /><BooleanProperty label="Italic" field="italic" components={components} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} error={errorFor('italic')} absentCutId={missingItalicCut && cutAbsenceId(missingItalicCut)} /></div>{absentCuts.map((cut) => <p key={cut} id={cutAbsenceId(cut)} className="property-unavailable">{cutAbsenceSentence(cut)}</p>)}</div></div>{draftFor({ ...lineSpacingField, empty: points(defaultLineSpacing), shown: true })}{draftFor(colorField)}<div className="property-grid"><SegmentedProperty label="Align" field="align" segments={alignChoices} components={components} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} error={errorFor('align')} /><SegmentedProperty label="Vertical align" field="valign" segments={valignSegments} components={components} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} error={errorFor('valign')} /></div></PropertySection>}
    {image && <ImageSection component={image} onPick={onPickImage} available={imageAvailable} busy={assetBusy} error={assetError?.id === image.id ? assetError.message : undefined} />}
    <PropertySection title="BOX">{!types.has('line') && !types.has('barcode') && !types.has('qrcode') && borderFields.map(draftFor)}{!types.has('line') && !types.has('barcode') && !types.has('qrcode') && <BorderEdgesProperty components={components} ids={ids} onCommit={onCommit} documentGeneration={documentGeneration} error={errorFor('borderEdges')} />}{line && borderProjected(line) && <p className="honest-note">This line carries a border in the document — the panel does not offer one, because a line is authored as a thickness and a colour. The stored border is unchanged and still paints. This note reports what the engine projects, not what the PDF draws.</p>}{!types.has('barcode') && !types.has('qrcode') && draftFor(boxFillFieldFor(single?.type))}{draftFor(visibilityField)}<p className="honest-note">Visibility takes a boolean or null formula — {'e.g. loanAmount > 20000'}. Use true, false, arithmetic, or nested conditions. Empty is always visible.</p></PropertySection>
    {table && <PropertySection title="TABLE"><button type="button" className="file-button" onClick={() => onEditTable(table.id)}>Configure columns</button></PropertySection>}
    {scalarBindable && <PropertySection title="BINDING" tone="bind">{single?.binding ? <p className="binding-chip"><span className="binding-dot" aria-hidden="true" />Bound to <code>{single.binding}</code></p> : <p className="honest-note">{single ? 'No engine binding on this component. Pick a root scalar in the Data tab.' : 'Binding is shown for one selected component.'}</p>}</PropertySection>}
    {/* STORY 14.4 / AC3 (D-14.4.Q2(a)). A Table's binding used to be stated
        THREE times in this panel — here, in the TABLE section above, and in the
        ungated BINDING section — and was editable in NONE of them. The sole
        editable site is the table editor's `Root collection` / `Row alias`
        pair, which Story 14.7 keeps there. So the three became ONE, and the one
        names where the value is actually changed. Option (b), a main-window
        editor, was explicitly REFUSED: it would build new capability and
        contradict 14.7's premise.
        THE MULTI-SELECTION SENTENCE IS UNTOUCHED — a selection carrying a table
        alongside other kinds has no single `tableBind` to state. */}
    <p className="honest-note">{table ? <>Table binding: {table.tableBind ?? 'Not set'} — the panel does not offer it here, because a table's collection and row alias are edited in the table editor, under Configure columns. Table size is not offered either; table geometry is derived from columns. The stored value is unchanged. This note reports what the engine projects, not what the PDF draws.</> : types.has('table') ? 'Table size and binding are not editable here; table geometry is derived from columns.' : 'Only committed engine values are shown. Arbitrary CSS is not editable here.'}</p>
  </>
}

// ImageSection is AC2's IMAGE section: it shows the CURRENTLY set asset's
// identity straight from the engine snapshot (never a local model of it),
// plus one named control that opens the local image picker. Every
// unavailable/failed state states its concrete reason in text — never
// colour alone — and the control is a plain, visibly-labelled button, so
// both the accessible name and keyboard/focus behaviour come from the same
// existing button pattern every other file/pick control in this panel uses.

function ImageSection({ component, onPick, available, busy, error }: { component: PanelComponent; onPick: (id: string) => void; available: boolean; busy: boolean; error?: string }) {
  const image = component.image
  return <PropertySection title="IMAGE">
    {image
      ? <p className="honest-note">{image.mediaType} · {image.width}×{image.height}px · asset {image.assetKey.slice(0, 12)}…</p>
      // Finding 9 (review of 2026-08-29): the paint field's absence used to
      // drive ONE fixed string here, which was FALSE for a dangling asset
      // reference — the media type is fine, the asset is simply gone.
      // Go's imageUnavailable discriminant (still one signal alongside the
      // absent paint, D-5.13.2) now says which of the two applies. With no
      // discriminant at all the box is simply unfilled: a placed image now
      // starts with a null asset and waits for a file.
      : <p className="honest-note" role="status">{component.imageUnavailable === 'missing' ? "This element's asset is not present in the document." : component.imageUnavailable === 'undecodable' ? "This version cannot render this asset's media type." : 'No image chosen yet. This box stays empty, and prints nothing, until you choose one.'}</p>}
    <button type="button" className="file-button" disabled={!available || busy} onClick={() => onPick(component.id)}>Choose image…</button>
    {!available && <p className="honest-note">No local file picker is available in this browser tier.</p>}
    {error && <p role="alert" className="property-error">{error}</p>}
  </PropertySection>
}

function propertyEvidence(component: PanelComponent | undefined, field: PropertyField) {
  if (!component) return undefined
  const key = field === 'expression' ? 'value' : field
  return component.authored && key in component.authored ? component.authored[key as keyof NonNullable<PanelComponent['authored']>] : component[key as keyof PanelComponent]
}
function propertyPresent(component: PanelComponent, field: PropertyField): boolean {
  const evidence = propertyEvidence(component, field)
  return evidence !== undefined && !(evidence && typeof evidence === 'object' && 'state' in evidence && evidence.state === 'absent')
}
function authoredValue(component: PanelComponent, field: PropertyField): unknown {
  const evidence = propertyEvidence(component, field)
  if (evidence && typeof evidence === 'object' && 'state' in evidence) return evidence.state === 'value' ? evidence.value : undefined
  return evidence
}
function sameProperty(components: ReadonlyArray<PanelComponent>, field: PropertyField): boolean {
  const first = JSON.stringify(propertyEvidence(components[0]!, field))
  return components.every((component) => JSON.stringify(propertyEvidence(component, field)) === first)
}
function committedValue(component: PanelComponent, field: PropertyField): string | undefined {
  const value = authoredValue(component, field)
  if (typeof value === 'number') return points(value)
  return typeof value === 'string' ? value : undefined
}
// STORY 17.4. THE EXACT INVERSE OF `points`, and the only reader on the arrow
// step's path: a plain decimal with AT MOST THREE PLACES, read into INTEGER
// thousandths.
//
// The trap this exists to close is arithmetic, not keys. Every value in the
// inspector is a decimal string Go parses exactly and refuses beyond three
// places (`internal/template/decimal.go`: `has more than three decimal
// places`), and it is passed through UNQUOTED. So the decimal itself is never
// added to: only its DIGIT GROUPS are read as integers, and every step, clamp
// and comparison downstream is integer arithmetic on thousandths.
//
// MEASURED, because the obvious illustration of the hazard is wrong: `1 + 0.1`
// is EXACTLY `1.1` in IEEE doubles, so a single step off a round number would
// survive float arithmetic and prove nothing. The damage begins on the SECOND
// step — `1.1 + 0.1` is `1.2000000000000002` — and compounds from there. That
// is why the guard is the arithmetic itself rather than a check on the result,
// and why the test that covers it steps repeatedly.
//
// A draft this refuses is NOT STEPPABLE and the arrow does nothing: an empty
// box, a mixed selection (which presents as an empty draft), `abc`, a fourth
// decimal place, `1e3`, or a magnitude past the exact-integer range. That
// refusal IS the guard — there is no path on which a float could be produced,
// and none on which an unreadable literal could be sent.
function draftThousandths(text: string): number | undefined {
  const parts = /^(-?)(\d+)(?:\.(\d{1,3}))?$/.exec(text)
  if (!parts) return undefined
  const magnitude = Number.parseInt(parts[2] as string, 10) * 1000 + Number.parseInt(((parts[3] ?? '') as string).padEnd(3, '0'), 10)
  if (!Number.isSafeInteger(magnitude)) return undefined
  return parts[1] === '-' ? -magnitude : magnitude
}
function PropertyDraft({ spec, components, ids, onCommit, documentGeneration, live, error }: { spec: FieldSpec; components: ReadonlyArray<PanelComponent>; ids: ReadonlyArray<string>; onCommit: CommitProperties; documentGeneration: number; live?: string; error?: PropertyCommitError }) {
  const { field, label, affix, unit, swatch, prose, lines, empty, shown, fx } = spec
  const values = components.map((component) => committedValue(component, field))
  const same = sameProperty(components, field)
  // `committed` IS THE DOCUMENT'S OWN VALUE AND MUST STAY SO — `''` when the
  // key is absent. Story 17.3 puts the engine's default in the BOX, never in
  // here: `commit()` below is `if (draft !== committed)`, so folding the
  // default into `committed` would make that comparison false and committing
  // the shown default would send NOTHING while every gate stayed green.
  const committed = same ? values[0] ?? '' : ''
  // STORY 17.3. What the box READS when the document says nothing. For every
  // field but two that is the empty string and this is the identity function;
  // for `fontSize` and `lineSpacing` it is the engine's own effective value,
  // arriving as `empty` from the projection with `shown` marking it real.
  //
  // NOT APPLIED TO A MIXED SELECTION. `same` is false there, the components
  // genuinely disagree, and filling in one number would both lie about them
  // and — through the arrow step, which reads the draft — put a flattening
  // edit one nudge key away. Mixed keeps its empty draft and its `Mixed`
  // placeholder.
  //
  // AND IT WRITES NOTHING BY ITSELF. This is display state: no command is sent
  // until the author commits the field, which is the safety property the whole
  // story rests on — opening a document may never mutate it.
  const inherited = (text: string): string => text === '' && same && shown === true && empty !== undefined ? empty : text
  const [draft, setDraft] = useState(inherited(committed))
  const touched = useRef(false)
  const [pending, setPending] = useState(false)
  const pendingRef = useRef(false)
  // STORY 17.1. THE DRAFT NOW HAS A READER THAT IS NOT A RENDER.
  //
  // A debounced commit fires from a TIMER, outside the render whose closure
  // scheduled it, and `draft` read from that closure is whatever was on screen
  // one keystroke ago — a silently truncated command. `draftRef` is the same
  // value read at the instant the timer runs, which is why EVERY write to the
  // draft goes through `writeDraft` and nothing calls `setDraft` alone. A
  // second writer would not fail loudly; it would send stale text.
  const draftRef = useRef(draft)
  const writeDraft = (value: string) => { draftRef.current = value; setDraft(value) }
  // The render-time write below (`setDraft` in the committed transition) does
  // NOT go through `writeDraft`, because touching a ref during render is both
  // impure and lint-flagged; this catches the ref up once the render commits,
  // which is long before any timer or continuation can read it.
  //
  // HONESTLY LABELLED: no test reddens when this line is deleted, and that is
  // not an oversight in the tests. The committed transition only writes the
  // draft when `unsentEdit` is false, and `unsentEdit` is false only after a
  // dispatch — at which point no debounce timer is armed, so nothing reads a
  // stale `draftRef`. This maintains the invariant "`draftRef` is `draft`"
  // rather than repairing a reachable defect, and it is what keeps that
  // invariant true if the transition's guard is ever relaxed.
  useEffect(() => { draftRef.current = draft })
  // STORY 17.1: WHO OWNS THE DRAFT — THE AUTHOR, OR THE ENGINE'S ECHO.
  //
  // TRUE from the moment the author types until the moment a command carrying
  // that text is dispatched. While it is true the engine may not write the
  // draft, because anything it has to say is about text the author has already
  // moved past. Blur alone could produce that collision before this story
  // (measured at c13864c: type `Invoice`, blur, type `Invoice 2026` while the
  // command is in flight, and the field comes back reading `Invoice`); a timer
  // makes it ordinary, which is what turns a rare race into the story's risk.
  //
  // IT IS HELD TWICE, AND THAT IS NOT AN OVERSIGHT. The committed transition
  // below reads it DURING RENDER, where a ref may not be read; `submit` reads
  // it in an async continuation, where a state value captured at dispatch is
  // stale by construction. One setter writes both, so the two cannot drift.
  //
  // ONLY THE PROSE FIELD EVER RAISES IT. The single-line rows have no debounce
  // and no way to type across their own commit — `shared` disables them for the
  // duration — so raising it there would change how a half-typed number
  // survives an unrelated canvas drag, on no evidence and outside this story's
  // remit. The flag is lowered by every dispatch regardless of which control
  // sent it, which costs nothing where it was never raised.
  const [unsentEdit, setUnsentEdit] = useState(false)
  const unsentEditRef = useRef(false)
  const holdDraft = (held: boolean) => { unsentEditRef.current = held; setUnsentEdit(held) }
  // Escape reverts and blurs, and `.blur()` dispatches blur SYNCHRONOUSLY into
  // `shared.onBlur`, which commits. This suppresses that one commit. See the
  // Escape arm in `keyDown` for the defect it repairs.
  const reverting = useRef(false)
  const proseTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  // A debounced commit that collides with a command already in flight is
  // QUEUED, never dropped. `submit`'s early return is right for a click — the
  // author cannot mean two — and wrong for a timer, where dropping means the
  // last thing typed never reaches the engine at all.
  const queuedProse = useRef(false)
  // WHAT THE ENGINE HAS LAST BEEN TOLD THIS FIELD SAYS — which is NOT always
  // what the document holds, and the difference is the point.
  //
  // It advances at DISPATCH, so the drain can tell a genuine new edit from a
  // repeat of the command that just went: the drain runs inside `submit`'s
  // continuation, BEFORE React has re-rendered with the accepted projection,
  // so `committed` is still the pre-commit value there and comparing against
  // it would resend text the engine already has.
  //
  // It re-syncs to `committed` whenever the DOCUMENT'S value moves, which
  // covers both our own accepted commit and a change this field did not make.
  // A REFUSED command is exactly the case where the two diverge and must: the
  // engine was told `{{cust`, the document still holds `Invoice`, and an
  // author who deletes back to `Invoice` has to be able to send it again —
  // that resend is what runs `applyProperties`' `setPropertyError(undefined)`
  // and clears the alert. Comparing against `committed` there would return
  // early, send nothing, and leave a refusal on screen describing text that is
  // no longer in the box.
  const toldEngine = useRef(committed)
  const mounted = useRef(true)
  // The timer must call the CURRENT send, not the one captured when it was
  // scheduled: `submit` closes over `committed`, `ids` and `onCommit`, all of
  // which move under the author while the timer runs.
  const proseSender = useRef<() => void>(() => {})
  const cancelProseCommit = () => { if (proseTimer.current !== undefined) { clearTimeout(proseTimer.current); proseTimer.current = undefined } }
  const scheduleProseCommit = () => {
    cancelProseCommit()
    proseTimer.current = setTimeout(() => { proseTimer.current = undefined; proseSender.current() }, PROSE_COMMIT_DEBOUNCE_MS)
  }
  // A canvas drag, resize or nudge commits geometry without this field ever
  // being touched. Follow the engine on a committed transition; a draft the
  // engine has not accepted (a rejected commit leaves the value unchanged)
  // still survives, because nothing transitioned.
  //
  // STORY 17.1 ADDED THE `unsentEdit` GUARD, AND THIS IS THE CLOBBER SITE THAT
  // ACTUALLY FIRES. When a commit is accepted the projection comes back,
  // `committed` becomes the text that was sent, this transition sees it change,
  // and it overwrites the draft with the engine's echo — including whatever
  // the author has typed since. It fires regardless of `reconcileDraft`, so
  // guarding only `submit`'s reconciliation below leaves the hazard open.
  // `lastCommitted` still advances either way: skipping THAT would leave the
  // transition armed to fire again later against an even older value.
  const [lastCommitted, setLastCommitted] = useState(committed)
  if (lastCommitted !== committed) { setLastCommitted(committed); if (!unsentEdit) setDraft(inherited(committed)) }
  const selectionKey = ids.join(',')
  const revert = () => { touched.current = false; holdDraft(false); writeDraft(inherited(committed)) }
  // `disable` is the ONE thing the arrow step varies, and it is not a second
  // commit path: the intent, the encoder, the reconciliation and the
  // single-flight `pendingRef` guard are all shared verbatim. `shared` carries
  // `disabled: pending`, which exists so a keystroke cannot race a commit
  // already in flight — but an arrow step IS that keystroke, and disabling a
  // FOCUSED input moves focus to the body and does not give it back when the
  // input is re-enabled (measured in Chromium 1217). Key repeat is delivered to
  // the focused element, so raising `pending` here would end an arrow HOLD after
  // exactly one step.
  //
  // A step that arrives mid-flight is still dropped by `pendingRef`, and what
  // happens to it then is a RACE, not an accumulation: the draft has already
  // advanced, so if the next repeat beats the engine's answer it carries the
  // accumulated value, but if the answer wins, the committed transition below
  // (`lastCommitted !== committed`) rewrites the draft to the engine's value
  // and that press is lost. A hold therefore steps at the round-trip rate, not
  // the repeat rate. Coalescing repeats into one command is the real fix and is
  // out of this story's scope — it also bears on undo, since each step is its
  // own revision and its own undo entry.
  const submit = async (intent: PropertyIntent, reconcileDraft: boolean, disable = true) => {
    if (!mounted.current || pendingRef.current || live !== undefined) return
    touched.current = false
    pendingRef.current = true
    if (disable) setPending(true)
    // The engine has now been told what the author is holding, so the author
    // no longer holds anything unsent — until the next keystroke, which is
    // exactly the window an echo may not write the draft in.
    holdDraft(false)
    // Only a TEXT value is recorded; the prose field is the only reader, and it
    // only ever sends a string.
    if (typeof intent.value === 'string') toldEngine.current = intent.value
    const accepted = await onCommit(ids, intent, documentGeneration, selectionKey)
    pendingRef.current = false
    if (disable) setPending(false)
    // Through `inherited` for the CLEAR case: the engine's answer for a
    // cleared key is `''`, and the box must come back to the shown default
    // rather than to an empty row. Without this wrapper the committed
    // transition above would set the default and this line would immediately
    // overwrite it with the empty string, whichever order they landed in.
    // `!unsentEditRef.current` is STORY 17.1's: the answer describes text the
    // author may already have typed past, and a canonical spelling is worth
    // nothing if the price is a lost character.
    if (accepted && reconcileDraft && !unsentEditRef.current) {
      const acceptedComponents = accepted.components.filter((component) => ids.includes(component.id))
      const value = canonicalValue(accepted, ids, field)
      const acceptedDefault = field === 'fontSize' ? points(accepted.defaultFontSize) : field === 'lineSpacing' ? points(accepted.defaultLineSpacing) : empty
      writeDraft(value === '' && shown && sameProperty(acceptedComponents, field) && acceptedDefault !== undefined ? acceptedDefault : value ?? draftRef.current)
    }
    // THE DRAIN. A debounced commit that arrived while this one was in flight
    // parked itself here rather than being dropped; it goes now, carrying
    // whatever the author is holding at this instant. Not after unmount: the
    // continuation of an in-flight command outlives the component, and a
    // command sent from a dead panel is one the author cannot see, undo from,
    // or be shown a refusal for. `sendProseDraft` is the ONE place that refuses
    // after unmount: guarding here as well would make each check unfalsifiable
    // through the other, which is how a dead guard survives a mutation run.
    if (queuedProse.current) { queuedProse.current = false; proseSender.current() }
  }
  // STORY 17.1. THE DEBOUNCED COMMIT ITSELF — the same `submit`, the same
  // encoder, the same single-flight guard, the same command bytes. Three
  // things differ from `commit`, and each is forced:
  //
  // 1. `disable = false`. `shared` carries `disabled: pending`, and disabling a
  //    FOCUSED input moves focus to the body and does not give it back when the
  //    input is re-enabled (measured in Chromium 1217, Story 17.4). With the
  //    default every debounce would blank the author's focus mid-sentence, and
  //    the acceptance criterion "the canvas shows the text WITHOUT focus
  //    leaving the field" would be false in a real browser. jsdom does not
  //    implement blur-on-disable, so a green suite is not evidence here — the
  //    browser run is.
  // 2. `reconcileDraft = false`. The author is still typing; there is nothing
  //    to normalise a live draft to.
  // 3. A collision QUEUES instead of returning.
  //
  // It reads `draftRef`, never `draft`: see `writeDraft` above.
  const sendProseDraft = () => {
    // Never while the field is read-only — a drag owns those fields and typing
    // does nothing — and never after unmount.
    if (!mounted.current || live !== undefined) return
    const text = draftRef.current
    // NOTHING TO SAY, AND THE AUTHOR IS HOLDING NOTHING UNSENT. The engine has
    // already been told this exact text, so there is no command to send — and
    // the ownership flag must come DOWN here rather than staying raised for the
    // life of the selection, which would stop the box ever following the engine
    // again. Typing a character and deleting it before the pause is the
    // ordinary way to reach this line.
    if (text === toldEngine.current) { holdDraft(false); return }
    if (pendingRef.current) { queuedProse.current = true; return }
    // A half-typed `{{` routes to `expression` and the engine REFUSES it
    // (measured over the whole ladder at c13864c: `{{`, `{{c`, `{{cust` and
    // `{{customer.name` are all refused with "component properties did not
    // pass format validation"; `{`, `{{customer.name}}` and `}}` are
    // accepted). It is sent anyway and the existing refusal path renders,
    // because the alternative — suppressing the send — means re-implementing
    // the engine's placeholder grammar in the browser, a mirrored invariant
    // this story carries no ruling for. `applyProperties` clears the error on
    // entry, so the next debounce that succeeds clears the alert itself.
    void submit({ field: contentCommand(field, text), operation: 'set', value: text }, false, false)
  }
  useEffect(() => { proseSender.current = sendProseDraft })
  useEffect(() => { toldEngine.current = committed }, [committed])
  // ONE PLACE CANCELS THE TIMER ON THE WAY OUT, and it is this cleanup.
  //
  // IN PRACTICE IT IS UNMOUNT-ONLY, and the deps do not earn their keep by
  // catching a live selection change: `ComponentProperties` is keyed
  // `documentGeneration:selection` (App.tsx:1478), so a change of selection or
  // document REMOUNTS every `PropertyDraft` beneath it and this cleanup runs as
  // an unmount. The deps are kept as a standing guard for the day that key
  // changes — they cost one comparison, and would be the only thing stopping a
  // timer firing against `ids` naming a component the author never typed into
  // — but nothing here should be read as evidence that the in-place transition
  // happens today. It does not.
  //
  // Cancelling in the unmount effect below as well would make each of the two
  // unfalsifiable through the other, which is how a dead guard survives a
  // mutation run.
  useEffect(() => cancelProseCommit, [selectionKey, documentGeneration])
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  // The `else` arm is Story 17.3's, and it is not a commit: emptying a box
  // whose key is ALREADY absent leaves `draft === committed === ''`, so there
  // is nothing to send — and nothing was sent before this story either. What
  // changes is what the author is left looking at. The row must come back to
  // the value it inherits, the same one it opened on, instead of sitting blank
  // beside a canvas that is still painting 12.
  const commit = async () => { if (!mounted.current || live !== undefined || (!touched.current && ids.length > 1)) return; if (draft !== committed) await submit({ field: contentCommand(field, draft), operation: draft === '' && field !== 'value' && field !== 'expression' ? 'clear' : 'set', value: draft }, true); else writeDraft(inherited(committed)) }
  // BLUR STILL COMMITS, AND IT COMMITS EXACTLY ONCE. It cancels the debounce
  // first, so a timer that was about to fire for this same text does not
  // become a second command.
  //
  // AND THAT CANCELLATION IS WHY THIS CANNOT SIMPLY FALL THROUGH TO `commit`.
  // A blur landing while a debounced command is still in flight used to lose
  // everything typed after that command went out, and the two halves of the
  // control conspired to do it silently: `cancelProseCommit` destroyed the
  // armed timer that would have QUEUED the text, and then `commit` reached
  // `submit`'s `if (pendingRef.current) return` and was dropped. Only
  // `sendProseDraft` ever queued, and its timer no longer existed. The text
  // reached neither the engine nor anything that would later reconcile it —
  // the spec's "TYPING MUST NEVER LOSE A CHARACTER" broken by the one gesture
  // an author makes to finish typing.
  //
  // So a blur whose commit the in-flight guard would swallow queues instead,
  // exactly as a debounced commit does, and the drain sends it. Prose only:
  // `queuedProse` is drained through `sendProseDraft`, which sends a `value`
  // or `expression` command, and a single-line row would have its own field's
  // edit sent under the wrong key. Those rows are `disabled` for the duration
  // of their own commit anyway, so nothing can be typed into them to lose.
  const blur = () => {
    cancelProseCommit()
    if (reverting.current) return
    if ((prose || lines !== undefined) && pendingRef.current) { queuedProse.current = true; return }
    // Leaving the field ends the author's hold on it: whatever is in the box is
    // being committed on the next line, and if it already IS the committed text
    // then nothing was ever unsent. Without this, a draft typed and then
    // deleted back before blurring would leave the flag raised for the life of
    // the selection.
    holdDraft(false)
    void commit()
  }
  // STORY 17.4: ARROWS STEP A NUMBER FIELD.
  //
  // THE NUMERIC SET IS THE ONE THE CONTROL ALREADY KNOWS. This predicate was
  // computed inline for `inputMode`; it is hoisted rather than restated, so a
  // field can never be typeable as a decimal and unsteppable, or the reverse.
  // It is exactly x, y, width, height, fontSize, borderWidth and lineSpacing.
  const numeric = unit === 'pt' || unit === undefined && (field === 'lineSpacing')
  // THE STEP IS DERIVED FROM THE FIELD, NOT A CONSTANT. A point field steps by
  // one POINT, the same increment the canvas nudge already uses; leading is a
  // dimensionless RATIO and steps by a tenth. Both are written in the
  // thousandths the arithmetic runs in. 0.001 is the floor of the
  // representation, never a step.
  const stepThousandths = field === 'lineSpacing' ? 100 : 1_000
  // The bounds are the ENGINE'S OWN, read from the places that declare them
  // rather than restated: `POSITIVE_LENGTH_FIELDS` is the four keys
  // `component_commands.go` refuses at or below zero, so their smallest legal
  // value is one thousandth; `ORIGIN_FLOOR_FIELDS` is `x` and `y`, which
  // `containComponent` refuses BELOW ZERO on this same command path; and
  // lineSpacing's pair is engine-protocol's mirror of `linespacing.go`.
  //
  // ⚠ THIS IS NOT THE WHOLE OF `containComponent`. It also bounds x, y, width
  // and height ABOVE against the band extents, and the arrow step does NOT
  // clamp to those — a step at the band edge still reaches the engine's own
  // located refusal. That is an OPEN question recorded in the story's Spec
  // Change Log, not a settled exclusion: the bound is per-component (two
  // components with equal widths at different x have different width ceilings),
  // so a selection-wide clamp needs a ruling this story does not carry.
  const lowest = field === 'lineSpacing' ? MIN_LINE_SPACING_THOUSANDTHS : POSITIVE_LENGTH_FIELDS.includes(field) ? 1 : ORIGIN_FLOOR_FIELDS.includes(field) ? 0 : undefined
  const highest = field === 'lineSpacing' ? MAX_LINE_SPACING_THOUSANDTHS : undefined
  // Returns whether the arrow was HANDLED, which is what suppresses the
  // browser's own caret jump — an unhandled arrow keeps it, on a non-numeric
  // field, during a drag, and on a draft with no value in it to step.
  const step = (direction: 1 | -1): boolean => {
    // A drag owns the geometry fields: they are `readOnly`, typing does
    // nothing, and an arrow does nothing either.
    if (!numeric || live !== undefined) return false
    // ONE PREDICATE, NOW CLOSING ONE ROW. Story 17.4 wrote this guard to close
    // TWO — an unset field and a mixed selection, both of which presented as an
    // empty draft the exact parser refuses. STORY 17.3 DISSOLVED THE FIRST OF
    // THEM, and the orchestrator retired that arm on 2026-09-04 rather than
    // carving it out: `fontSize` and `lineSpacing` now carry the engine's own
    // effective value as text, so an unset field HAS a value, it parses, and
    // ArrowUp steps from it. The precondition 17.4 rested on — "an unset field
    // has no value to step, and its placeholder is not one" — is simply no
    // longer true of these two fields, and keeping the guard would have made
    // typing `1.1` into a box reading `1` write 1.1 while ArrowUp on the same
    // visible `1` wrote nothing. That is the special case 17.4's own
    // `expect(stepped).toEqual(typed)` exists to forbid.
    //
    // 17.4's SECOND reason did not dissolve and this line still carries it. A
    // MIXED selection also presents as an empty draft, and stepping it would
    // flatten every component onto one value — a destructive edit fired by a
    // nudge key, on a field the author has not touched. Nothing in 17.3
    // reaches that: `inherited` above deliberately does not fill a mixed
    // draft. A mixed field the author has TYPED into is no longer empty and
    // steps like any other draft — they are stepping the value they entered.
    const current = draftThousandths(draft)
    if (current === undefined) return false
    const stepped = current + direction * stepThousandths
    // Floored, THEN capped, each against its own bound and neither against the
    // unclamped step: folding both into one expression let the absent ceiling
    // fall back to `stepped` and quietly undo the floor.
    const floored = lowest === undefined ? stepped : Math.max(stepped, lowest)
    const next = highest === undefined ? floored : Math.min(floored, highest)
    // Already at the bound: the arrow is still handled — the caret stays put —
    // but nothing changed, so no command is sent.
    if (next === current) return true
    const value = points(next)
    writeDraft(value)
    // `reconcileDraft` is false BY DESIGN. `points` already emits the
    // canonical spelling, so there is nothing for the engine to normalise, and
    // reconciling from a resolved step would overwrite a draft a later repeat
    // had already advanced. The committed transition above
    // (`lastCommitted !== committed`) is what lands the engine's answer.
    void submit({ field, operation: 'set', value }, false, false)
    return true
  }
  // The reset action appears with the value it resets, and with a mixed
  // selection, which also has committed values behind it. An unset row is
  // otherwise empty chrome with nothing to clear on it.
  //
  // STORY 17.3 gives `fontSize` and `lineSpacing` a value even when the
  // document is silent, so `×` now appears on those two rows unconditionally.
  // That is the matrix's own reading of the control — the box says 12, so the
  // row offers to reset it, and clearing an already-absent key lands back on
  // the same 12. `clear` itself is untouched: it still sends `op:"clear"`, and
  // Go still stores the zero Presence that omits the key from the file.
  const canClear = field !== 'x' && field !== 'y' && field !== 'width' && field !== 'height' && field !== 'value' && field !== 'expression' && (!same || (live ?? draft) !== '' || components.some((component) => propertyPresent(component, field)))
  const canNull = field === 'background'
  const errorId = error ? `property-error-${field}` : undefined
  // The fx cue states, in the row itself, that this field is read as an
  // expression, AND opens the expression reference at the section that governs
  // this kind of field. The same sentence still reaches a screen reader through
  // the input's description, so the cue is never colour- or sight-only; the
  // link carries its own name rather than relying on the two letters.
  const description = [same ? undefined : 'Mixed value', fx ? fxHint[fx] : undefined].filter((part) => part !== undefined).join('. ') || undefined
  // Enter COMMITS in a single-line field and INSERTS A LINE FEED in a prose
  // one — the one behaviour that differs between the two controls. Escape
  // still reverts and blurs, blur still commits, and the single-flight submit
  // and canonicalValue reconciliation are shared verbatim.
  const keyDown = (event: ReactKeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    // A `lines` field (barcode and QR code content) inserts a line feed as
    // prose does; Go stores each one as the carriage return a payment payload
    // separates fields with (barcode_element.go).
    if (event.key === 'Enter' && !prose && lines === undefined) { event.preventDefault(); void commit() }
    // STORY 17.1 REPAIRED THIS ARM, WHICH DID NOT DO WHAT IT SAYS.
    //
    // MEASURED AT c13864c: focus a prose field, type, press Escape — ONE
    // command leaves carrying the unwanted draft, and the field ends up still
    // showing it. `revert()` only SCHEDULES the new draft, while
    // `.blur()` dispatches blur SYNCHRONOUSLY into `shared.onBlur`, which
    // commits against the PRE-revert `draft` from the render in flight. The
    // pre-existing test for this row never focused the field, so `.blur()` was
    // a no-op, no blur event fired, and `expect(sent).toHaveLength(0)` passed
    // vacuously.
    //
    // `reverting` suppresses that one commit, and is lowered immediately
    // afterwards so an UNFOCUSED Escape — where `.blur()` dispatches nothing —
    // cannot leave the flag raised and swallow the next real blur.
    //
    // THE DEBOUNCE IS CANCELLED BY THE BLUR THIS DISPATCHES, not here. Escape
    // is defined as revert-and-blur, `blur` already cancels, and cancelling
    // twice would leave two arms neither of which any test could redden. On the
    // path where `.blur()` dispatches nothing the field was never focused, and
    // `revert()` has put the draft back to `committed`, which `sendProseDraft`
    // declines to send.
    if (event.key === 'Escape') { event.preventDefault(); reverting.current = true; revert(); event.currentTarget.blur(); reverting.current = false }
    // Story 17.4. Enter and Escape above are untouched; the arrows are the
    // only addition, and only where the step actually took the key.
    //
    // A MODIFIED arrow is left entirely alone. This is not modifier BEHAVIOUR,
    // which the story puts out of scope — it is the absence of it: inside a
    // text input Shift+Arrow extends the selection and (on macOS) Cmd+Arrow and
    // Alt+Arrow move the caret, so stepping on a modified arrow would take
    // three shipped editing gestures away from the author to no end. Adding a
    // coarse or fine step on a modifier is the thing that needs asking for.
    if ((event.key === 'ArrowUp' || event.key === 'ArrowDown') && !event.shiftKey && !event.ctrlKey && !event.altKey && !event.metaKey) { if (step(event.key === 'ArrowUp' ? 1 : -1)) event.preventDefault() }
  }
  const proseField = useRef<HTMLTextAreaElement>(null)
  const proseCaret = useRef<number | undefined>(undefined)
  useLayoutEffect(() => {
    const caret = proseCaret.current
    if (caret === undefined || proseField.current === null) return
    proseCaret.current = undefined
    proseField.current.setSelectionRange(caret, caret)
  })
  // ONLY the plain flavour is ever read. A clipboard from a word processor
  // also carries text/html and text/rtf, which is where its fonts, bold,
  // italics and indents live; discarding them "without error" is achieved by
  // never looking at them, and never by adding a sanitiser — a parser would
  // be a new runtime dependency, and design-contract.test.ts pins the
  // lockfile. Paragraph breaks survive because they are in the plain text,
  // and a CRLF pair is folded into ONE mandatory break by the engine itself.
  const pasteProse = (event: ReactClipboardEvent<HTMLTextAreaElement>) => {
    // preventDefault FIRST, and UNCONDITIONALLY. A clipboard carrying only
    // text/html or text/rtf has no plain flavour to insert, and returning
    // before this call handed that paste to the BROWSER, which inserts text it
    // derived from the HTML — the one outcome "only the plain flavour is ever
    // read" exists to forbid. Nothing to insert must mean nothing inserted.
    event.preventDefault()
    const plain = event.clipboardData.getData('text/plain')
    if (plain === '') return
    const field = event.currentTarget
    const head = field.selectionStart ?? field.value.length
    const tail = field.selectionEnd ?? field.value.length
    // A paste is a typed edit by another name: it changes the draft, so it
    // takes the author's ownership of it and it starts the same debounce.
    touched.current = true
    holdDraft(true)
    writeDraft(`${field.value.slice(0, head)}${plain}${field.value.slice(tail)}`)
    scheduleProseCommit()
    // The textarea is CONTROLLED, so React rewrites its value on the next
    // render and the caret goes to the end of the whole field. Pasting into
    // the middle of a long clause would then land the author's next keystroke
    // in the wrong paragraph, so the caret is put back at the end of what was
    // just inserted, in the layout effect that runs once the render commits.
    proseCaret.current = head + plain.length
  }
  // STORY 17.5: THE BOTTOM EDGE IS THE HANDLE, AND THE HEIGHT IS VIEW STATE.
  //
  // NOTHING BELOW SENDS A COMMAND, marks anything dirty or changes a saved
  // byte. It is the panel's own presentation, exactly as the user agent's grip
  // was — a `useState` and some `clientY` arithmetic, and no path from either
  // into `submit`, `writeDraft`, `holdDraft`, `queuedProse` or the debounce.
  //
  // AN AUTHORED HEIGHT DOES NOT SURVIVE A CHANGE OF SELECTION. That is not an
  // omission; it is what the grip did, and it is this `useState`'s ordinary
  // behaviour: `ComponentProperties` is keyed `documentGeneration:selection`
  // (App.tsx:1478), so selecting another component REMOUNTS every
  // `PropertyDraft` beneath it and this state starts again at the floor.
  // Persisting instead would mean lifting per-row view state above that keyed
  // boundary — a second copy of the drag bookkeeping — to apply one
  // component's authored height to a different component's prose. Note the
  // counter only bumps on a document REPLACEMENT (`setCurrentSnapshot`'s
  // `clearDocumentInteraction`, App.tsx:1227), never on a property commit, so
  // this is "resets when you select something else", not "evaporates as you
  // type"; both halves are asserted, because the first alone is not
  // falsifiable.
  //
  // `undefined` means "resting", so the box keeps the CSS floor and no inline
  // height is written until the author actually drags.
  const [proseHeight, setProseHeight] = useState<number>()
  // THE GESTURE IS IDENTIFIED, not merely "in progress". Without the id a
  // second pointer's press silently rebases the anchor under the first one's
  // drag, and a second pointer's move is accepted as if the first had made it.
  const proseResize = useRef<{ pointerId: number; pointerY: number; height: number } | undefined>(undefined)
  const beginProseResize = (event: PointerEvent<HTMLSpanElement>) => {
    // PRIMARY BUTTON ONLY. A right-button press fires `pointerdown` like any
    // other, so without this a context-menu click on the strip STARTED A DRAG
    // (measured in Chromium 1217: right-press then a 50px move set the box to
    // 122px, and the menu then blocked the page mid-gesture). A middle-click
    // paste-press is the same shape. `button === 0` is the primary button on
    // mouse, pen and touch alike.
    if (event.button !== 0) return
    // ONE GESTURE AT A TIME, AND THE FIRST POINTER OWNS IT. Without this a
    // second finger's press REBASES the anchor under the first one's drag —
    // the id recorded below is what a move is checked against, and it would
    // simply have become the second pointer's. A press from the SAME pointer
    // still re-anchors, which is the ordinary "press again" case and is what
    // keeps a gesture that somehow outlived its `pointerup` from wedging the
    // handle shut. (A gesture stranded with no `pointerup`, no `pointercancel`
    // AND no further move from its own pointer would block a new press; that
    // needs all three to go missing at once, and `pointercancel` is exactly
    // what a browser sends when a touch gesture is taken away.)
    if (proseResize.current !== undefined && proseResize.current.pointerId !== event.pointerId) return
    // THE ONE LINE THAT KEEPS A RESIZE FROM BECOMING A WRITE. Pressing the
    // handle would otherwise move focus out of the textarea, and `blur` on
    // this field is a commit path; it would also move the caret out of the
    // clause the author is mid-way through. `preventDefault` on pointerdown
    // suppresses both, so the drag disturbs neither the field nor the pending
    // debounce underneath it.
    event.preventDefault()
    // THE ANCHOR IS RECORDED BEFORE CAPTURE IS REQUESTED, and the order is the
    // point. `setPointerCapture` is specified to THROW NotFoundError for a
    // pointerId that is not active; with the call first, such a throw would
    // unwind before this assignment and swallow the press — the author would
    // hold the edge and nothing would move, with no error they could see.
    // Capture is an enhancement to a drag that already works without it.
    proseResize.current = { pointerId: event.pointerId, pointerY: event.clientY, height: proseHeight ?? PROSE_MIN_HEIGHT_PX }
    // Optional-called because jsdom leaves it undefined, exactly as the canvas
    // handles do (`begin`, App.tsx:2881). In a browser it is what lets the drag
    // survive the pointer leaving a 7px strip.
    event.currentTarget.setPointerCapture?.(event.pointerId)
  }
  // Pure `clientY` arithmetic against where the press started — never a
  // measured box, and never an increment against the last event, which would
  // accumulate rounding across a long drag.
  const moveProseResize = (event: PointerEvent<HTMLSpanElement>) => {
    const from = proseResize.current
    if (from === undefined || event.pointerId !== from.pointerId) return
    // A MOVE WITH NOTHING HELD DOWN IS A HOVER, NOT A DRAG. If a `pointerup` is
    // ever missed — capture lost to something outside this component, the
    // element taken out from under the gesture — the drag state would outlive
    // the press and the next bare hover across the strip would resize the box
    // with no button held. This ends it instead, on the first such move.
    //
    // IT IS CHECKED AFTER THE ID, not before: a pen hovering (buttons === 0)
    // while a touch drag is live must not tear down the touch's gesture, which
    // is the very cross-pointer interference the id guard above exists for.
    // The hazard this closes — a hover resizing the box — is closed either way,
    // because a hover from the DRAGGING pointer is exactly what reaches here.
    if (event.buttons === 0) { endProseResize(event); return }
    setProseHeight(Math.max(PROSE_MIN_HEIGHT_PX, from.height + (event.clientY - from.pointerY)))
  }
  // Both endings, and they are the same ending: pointerup, and the
  // pointercancel that fires when the gesture is taken away (the pointer
  // leaving the window, a touch interrupted). Neither leaves drag state behind
  // — and neither lets an unrelated pointer end someone else's drag.
  const endProseResize = (event: PointerEvent<HTMLSpanElement>) => { if (proseResize.current?.pointerId === event.pointerId) proseResize.current = undefined }
  const shared = { 'aria-label': label, 'aria-description': description, 'aria-invalid': error ? ('true' as const) : undefined, 'aria-errormessage': errorId, readOnly: live !== undefined, value: live ?? draft, placeholder: same ? empty : 'Mixed', disabled: pending, onBlur: blur, onKeyDown: keyDown }
  return <div className="property-editor"><div className={`property-field${prose ? ' property-field-prose' : lines ? ' property-field-lines' : ''}${live === undefined ? '' : ' property-field-live'}`}>{affix && <span className="property-affix">{affix}</span>}{prose
    ? <textarea ref={proseField} className="property-value property-value-prose" rows={4} style={proseHeight === undefined ? undefined : { height: `${proseHeight}px` }} {...shared} onChange={(event) => { touched.current = true; holdDraft(true); writeDraft(event.target.value); scheduleProseCommit() }} onPaste={pasteProse} />
    // A `lines` field commits as prose does — debounced while typing, queued on
    // blur — over that many rows; each new line is stored as `\r` (see keyDown).
    : lines ? <textarea ref={proseField} className="property-value property-value-lines" rows={lines} {...shared} onChange={(event) => { touched.current = true; holdDraft(true); writeDraft(event.target.value); scheduleProseCommit() }} />
    : <input className="property-value" {...shared} inputMode={numeric ? 'decimal' : undefined} onChange={(event) => { touched.current = true; writeDraft(event.target.value) }} />}{fx && <a className={`property-fx${holdsExpression(fx, live ?? draft) ? ' property-fx-active' : ''}`} href={`${documentationAssetUrls.expressions}#${fxAnchor[fx]}`} target="_blank" rel="noopener noreferrer" title={`${fxHint[fx]}. Opens the expression reference`} aria-label={`${label}: open the expression reference`} onMouseDown={(event) => event.preventDefault()}>fx</a>}{swatch && <input type="color" className={`property-swatch${isHexColour(live ?? draft) ? '' : ' property-swatch-unset'}`} aria-label={`Pick ${label}`} aria-description={same ? undefined : 'Mixed value; choose a colour'} value={swatchColor(live ?? draft)} disabled={pending || live !== undefined} onChange={(event) => { writeDraft(event.target.value); void submit({ field, operation: 'set', value: event.target.value }, true) }} />}{unit && <span className="property-unit">{unit}</span>}{canClear && <button type="button" className="property-inline-action" aria-label={`Clear ${label}`} title={`Clear ${label}`} disabled={pending} onMouseDown={(event) => event.preventDefault()} onClick={() => void submit({ field, operation: 'clear' }, true)}>×</button>}{canNull && <button type="button" className="property-inline-action" aria-label={`Set ${label} null`} title={`Set ${label} null`} disabled={pending} onMouseDown={(event) => event.preventDefault()} onClick={() => void submit({ field, operation: 'null' }, true)}>∅</button>}{prose && <span className="property-prose-resize" aria-hidden="true" onPointerDown={beginProseResize} onPointerMove={moveProseResize} onPointerUp={endProseResize} onPointerCancel={endProseResize} />}</div>{error && <p id={errorId} role="alert" className="property-error">{error.elementId ? `${error.elementId}: ` : ''}{printsDataPath(error) ? `${error.dataPath}: ` : ''}{error.message}</p>}</div>
}
// D-14.2.Q2b, AS AMENDED. THE RULE IS *NEVER PRINT A FIELD NAME THAT MAY BE
// WRONG* — NOT *NEVER PRINT ANYTHING*.
//
// Go's `propertyPath` returns THE FIRST KEY IN CANONICAL ORDER, not the key
// that failed — correct while `changes` always held one member, a mislabel the
// moment it holds two. On a `{width, height}` intent refused by `height` it
// answers `component.width`, and that reaches the screen.
//
// ⚠ AN EARLIER VERSION OF THIS FUNCTION SUPPRESSED EVERY PATH ON A MULTI-FIELD
// INTENT, on the stated premise that "the panel cannot tell the two apart".
// THAT PREMISE WAS FALSE, and it was throwing away the one diagnostic an author
// will actually hit: `containComponent` refuses a rotated rule with
// `component.geometry` (`component_commands.go`), which is ACCURATE for a
// width/height pair — and it is the refusal the orientation control produces.
// The panel CAN separate them exactly, because `propertyPath` returns one of
// `PropertyField`'s 23 members or the literal `changes`, and `geometry` is
// neither. So the test is a membership test against the union itself.
//
// Suppress only when BOTH hold: the intent carried more than one field, AND
// the path's last segment names a property field. Everything else prints —
// `component.geometry` because it is true, `component.changes` because it names
// no specific field and therefore cannot mislabel one (uninformative is not the
// same as wrong), and every single-field intent exactly as before this story.
//
// Fixing `propertyPath` in Go is the other, better repair and it is an engine
// change this story does not carry — registered as DW-333.
function printsDataPath(error: PropertyCommitError): boolean {
  if (error.dataPath === undefined) return false
  if (error.fields.length === 1) return true
  return !isPropertyField(error.dataPath.split('.').pop() ?? '')
}
function canonicalValue(canvas: CanvasProjection, ids: ReadonlyArray<string>, field: PropertyField): string | undefined { const values = canvas.components.filter((component) => ids.includes(component.id)).map((component) => committedValue(component, field)); return values.length === ids.length && values.every((value) => value === values[0]) ? values[0] ?? '' : undefined }
/**
 * THE THREE CUTS A CHAIN CAN DECLARE, and their names in a sentence. The wire
 * spells the combined one `boldItalic`; a person reads "bold italic".
 *
 * ⚠ THE SENTENCE IS DERIVED FROM THE CUT, NEVER FROM THE CONTROL, and that is
 * F1's second part rather than a formatting choice. A sentence per control says
 * "No bold face in this family" whenever B is unavailable — including on a
 * chain that DECLARES a bold and is missing only the combined cut, where the
 * statement is simply false. A panel that lies precisely is worse than one that
 * lies vaguely. One sentence per cut cannot go false that way.
 */
const CUT_NAMES = { bold: 'bold', italic: 'italic', boldItalic: 'bold italic' } as const
type StyleCut = keyof typeof CUT_NAMES

/**
 * DOES ANY ENTRY OF THIS COMPONENT'S CHAIN DECLARE THIS CUT?
 *
 * STORY 11.3 / AC3, and the only question the B / I controls' third state is
 * derived from. `chainDeclaresCut` is a READ-BACK of the projection Go now
 * carries (`CanvasFontChainEntry.bold/italic/boldItalic`, DW-239) — the
 * document's own declaration, copied verbatim by the engine. NOTHING HERE
 * RESOLVES ANYTHING: which face a painted fragment ends up in is
 * `fragment.face`, decided per rune by the engine against coverage, and a chain
 * entry never stands in for it.
 *
 * ⚠ EVERY ENTRY, NEVER `entries[0]` — and `declaredChainEntry` three functions
 * below IS THE WRONG FUNCTION TO REUSE HERE, named because it is the one an
 * implementer reaches for. It returns the FIRST entry, which is right for the
 * specimen row it serves and wrong for this: a chain
 * `["Noto Sans SC", "Roboto"]` would report "no bold face" while Latin bolds
 * perfectly well. Q2 ratified the all-entries rule at CHECKPOINT 1.
 *
 * ⚠ THE STARTER'S OWN CHAIN CANNOT DETECT THAT ERROR — its first entry is
 * Roboto, which declares a bold, so the two rules return the same answer on it
 * (D-11.2.8: an assertion whose two sides could be equal is not an assertion).
 * `App.test.tsx` drives this with a chain whose first entry has no bold and
 * whose later entry does.
 *
 * AN UNKNOWN FAMILY IS NOT AN ABSENCE. A component with no `fontFamily`, or one
 * naming a chain this projection does not carry, returns `true`: the panel
 * states an absence it has measured and never one it merely could not check.
 */
function chainDeclaresCut(family: string | undefined, cut: StyleCut, chains: CanvasProjection['fontChains']): boolean {
  if (family === undefined) return true
  const chain = chains.find((candidate) => candidate.name === family)
  if (chain === undefined) return true
  return chain.entries.some((entry) => entry[cut].length > 0)
}

/**
 * WHICH CUT WOULD THIS CONTROL BEING ON REQUIRE, AND IS IT MISSING?
 *
 * F1's first part. The cut is the one the element's RESULTING `(bold, italic)`
 * combination needs, not the control's own axis: B on an element that is
 * already italic asks the chain for `boldItalic`, not for `bold`.
 *
 * ⚠ THIS IS THE HOLE F1 WAS RAISED FOR. `boldItalic` was projected across the
 * whole seam and read by nothing, so an element with BOTH flags set, on a chain
 * declaring `bold` and `italic` but not `boldItalic`, resolved to the base face
 * and warned while both controls read plainly on — the exact state AC3 exists
 * to prevent, arriving through the one combination the I/O matrix never
 * enumerated.
 *
 * It is asked with the control ON rather than at the element's current
 * combination so the panel warns BEFORE the press as well as after it: an
 * unbolded element on a chain with no bold must still say so, which is AC3's
 * own sentence.
 */
function missingCutFor(component: PanelComponent, field: 'bold' | 'italic', chains: CanvasProjection['fontChains']): StyleCut | undefined {
  const bold = field === 'bold' || component.bold === true
  const italic = field === 'italic' || component.italic === true
  const cut: StyleCut = bold && italic ? 'boldItalic' : bold ? 'bold' : 'italic'
  return chainDeclaresCut(component.fontFamily, cut, chains) ? undefined : cut
}

/**
 * THE SELECTION'S ANSWER, and it is deliberately the CONSERVATIVE one: a cut is
 * reported missing only when EVERY selected component is missing it AND they
 * are missing the SAME one. A mixed selection in which one component can bold
 * keeps the plain control, because "this family has no bold face" would be
 * false of half of it — and a selection missing two DIFFERENT cuts has no one
 * true sentence to state, so it states none.
 */
function selectionMissingCut(components: ReadonlyArray<PanelComponent>, field: 'bold' | 'italic', chains: CanvasProjection['fontChains']): StyleCut | undefined {
  if (components.length === 0) return undefined
  const cuts = components.map((component) => missingCutFor(component, field, chains))
  const first = cuts[0]
  return first !== undefined && cuts.every((cut) => cut === first) ? first : undefined
}

/**
 * THE SENTENCE, BUILT IN ONE PLACE AND ANNOUNCED THROUGH ONE PATH (F1.3 / P9).
 *
 * It was folded into the button's `aria-label` AND rendered as a visible `<p>`,
 * phrased three ways — a double announcement that the combined case would have
 * made a quadruple. Now the visible paragraph is the only copy: the buttons
 * point at it with `aria-describedby`, so a screen reader hears the control's
 * plain name and then this sentence, once, in the wording that is on screen.
 *
 * The way out is part of the sentence, because a state with no stated exit is
 * the grey-out DESIGN.md forbids in a different costume. For the combined cut
 * the exit is the honest one: not "this family cannot do what you asked" but
 * "cannot do both at once" — turning off EITHER control reaches a combination
 * the chain does declare, which is what makes the state self-resolving.
 */
function cutAbsenceSentence(cut: StyleCut): string {
  return cut === 'boldItalic'
    ? 'No bold italic face in this family — it cannot do both at once. Turn off either one.'
    : `No ${CUT_NAMES[cut]} face in this family — the engine paints the regular face and warns.`
}

const cutAbsenceId = (cut: StyleCut) => `cut-absent-${cut}`

function BooleanProperty({ label, field, components, ids, onCommit, documentGeneration, error, absentCutId }: { label: string; field: 'bold' | 'italic'; components: ReadonlyArray<PanelComponent>; ids: ReadonlyArray<string>; onCommit: CommitProperties; documentGeneration: number; error?: PropertyCommitError; absentCutId?: string }) {
  const values = components.map((component) => authoredValue(component, field))
  const uniform = sameProperty(components, field)
  const active = uniform && values[0] === true
  const [pending, setPending] = useState(false); const pendingRef = useRef(false); const commit = async (intent: PropertyIntent) => { if (pendingRef.current) return; pendingRef.current = true; setPending(true); await onCommit(ids, intent, documentGeneration, ids.join(',')); pendingRef.current = false; setPending(false) }
  // A TOGGLE CARRIES ITS OWN CLEAR: pressing a pressed B or I unsets the
  // property, exactly as SegmentedProperty's "press again to clear" does, so
  // the inline × that used to sit beside each of these is gone. Turning bold
  // off therefore writes NO `bold: false` into the document -- unset and false
  // paint identically, and the round trip a toggle implies is off -> absent.
  //
  // THE THIRD STATE (Story 11.3 / AC3, ruled at CHECKPOINT 1 Q1(c)): the
  // chain declares no face at this weight or slope. It is a GENUINE third
  // state and not a disabled two-state control — the AC's own words are
  // "states that this family has no bold face RATHER THAN APPEARING TO BE ON",
  // and a disabled control still renders as on-or-off.
  //
  // IT STAYS OPERABLE, and that is the part that was refused outright. A
  // document can carry `bold: true` on a family with no bold cut — bold a
  // Roboto element, then switch it to a CJK-only chain — and a disabled
  // control would make that flag UNCLEARABLE: a control that has taken the
  // document hostage, against I-5's posture that the panel must never leave
  // the author unable to reach what the document carries.
  //
  // AND IT STATES ITS REASON BESIDE ITSELF, never a bare grey-out (DESIGN.md:
  // "State the reason next to anything disabled"). The sentence is also folded
  // into the accessible name, so the control does not read as a plain pressed
  // toggle to a screen reader while reading as something else on screen.
  //
  // ONE ANNOUNCEMENT PATH. The reason is rendered ONCE, per missing CUT, by the
  // caller — so two controls implicated by the same combined cut share one
  // sentence — and this control points at it with `aria-describedby`. The
  // accessible name stays the plain label: folding the sentence in as well
  // announced it twice, in a third wording, which is P9's bug and would have
  // been a quadruple in the combined case.
  const name = uniform ? label : `${label}, mixed`
  return <div className="property-editor"><div className="property-toggle-group"><button type="button" className={`property-toggle${absentCutId === undefined ? '' : ' property-toggle-unavailable'}`} disabled={pending} aria-pressed={uniform ? active : 'mixed'} aria-describedby={absentCutId} aria-label={name} title={active ? `${name}, press again to clear` : name} onClick={() => void commit(active ? { field, operation: 'clear' } : { field, operation: 'set', value: true })}>{label.slice(0, 1)}</button>{!uniform && <span className="property-toggle-mixed" aria-hidden="true">Mixed</span>}</div>{error && <p role="alert" className="property-error">{error.message}</p>}</div>
}
// The font family is a closed set too, but a per-DOCUMENT one: style.fontFamily
// must name a declared, non-empty font chain, and Go now projects exactly those
// names (CanvasProjection.fontFamilies). So this is a search-and-select over the
// engine's own list rather than a free-text field whose every typo is a round
// trip to a rejection. The typed text filters; it is never committed as a value.
//
// STORY 8.6 GAVE IT A SECOND GROUP; STORY 16.4 MADE IT THREE, ON THE AXIS THE
// CODE ALREADY FORKS ON — WHERE ARE THE BYTES, never when did they arrive.
// STORY 16.9 NARROWED THE DROPDOWN BACK TO TWO: the third relationship still
// exists in the code below — a family can still be not on this machine at
// all — but this control no longer OFFERS it. `Add fonts…` (the font
// browser) is the surface built for that relationship, and a menu listing
// ~1,273 rows nobody can use without a download, with no network and no
// wait, was never what the owner asked for.
//
//   1. IN THIS TEMPLATE — `families.includes(name)`, the document's own
//      declared chains. The bytes are IN THE FILE. Picking commits `fontFamily`,
//      exactly as it did before 8.6.
//   2. AVAILABLE LOCALLY — `familyIsInstalled(source)`, which is the `local` and
//      `stored` arms together. ON THIS MACHINE, NOT IN THIS FILE. Picking embeds
//      from the machine and then commits the property: two commands, two undos,
//      no network.
//
// A ROW'S GROUP IS A PURE FUNCTION OF (declared?, `familyIsInstalled`?) AND OF
// NOTHING ELSE — never of a set built up over this session. The `local`/`stored`
// split is deliberately invisible here: it is a provenance difference with no
// consequence at the moment of choosing. It used to surface at REMOVAL, in
// `lateEmbedRefusal`'s two branches — Story 16.6 deleted the removal control
// that distinction served, and the refusal below now says one sentence for
// both tiers.
//
// A FONT STILL CHANGES GROUP BECAUSE THE AUTHOR ACTED, even with the third
// group gone from this menu (Story 8.6's rule: nothing says "added", the
// entry simply moves). Installing through the font browser still moves a
// family into AVAILABLE LOCALLY the next time this dropdown opens; first use
// still moves it into IN THIS TEMPLATE. Only the affordance for the FIRST of
// those two moves left THIS control — the relationship the code forks on,
// and `offeredFamilies` itself, are unchanged.
//
// The engine stays the authority on what `fontFamily` may name. The designer
// never invents a family name — an offered name reaches the document only by
// going through a command, and the projection is what says it arrived.

/**
 * THE LATE REFUSAL, DISCLOSED RATHER THAN LEFT TO SURPRISE (Story 16.5).
 *
 * Install runs every admission check that can run at install. ONE CANNOT MOVE:
 * Go's nameID-13 licence-signature tie reads the licence written inside the face
 * and refuses a face whose own bytes contradict its declared terms
 * (`folio-go/internal/fontset/licencesignature.go`). Porting it would need a
 * second name-table reader and two regex tables in this designer — a competing
 * authority over what enters a document — so it stays in Go, and the residue is
 * that a face can install successfully and be refused the first time it is used.
 *
 * A DEAD END THE AUTHOR CAN SEE IS A STATED LIMIT; A DEAD END THAT SIMPLY FAILS
 * IS NOT. So the sentence says what the engine's own refusal cannot: that the
 * face IS on this machine and that nothing was written to the document.
 *
 * ONE SENTENCE FOR BOTH TIERS (Story 16.6). It used to point at a per-face
 * remove control for a stored face and say a bundled one had none — the
 * distinction the family control's `local`/`stored` split still tracks above,
 * for grouping only. That control is gone, so there is no remedy left to
 * offer and nothing left to tell the two tiers apart by.
 */
const lateEmbedRefusal = (family: string, engineMessage: string): string =>
  `${family} is installed on this machine and cannot be embedded in this document: ${engineMessage} Nothing was written to the document, and the face is still on this machine.`

// THE FAMILY CONTROL'S OWN SAMPLE TEXT (Story 16.7), SHORT ON PURPOSE AND
// DELIBERATELY NOT `font-browser-model.ts`'s `latinSample`/`thaiSample`.
// The design draws "a few letters set in that typeface" beside the name —
// `Aa Bb 123`, `กขค Aa` — never the browser's full sentence, which measured
// true in a real dropdown: at the panel's width it left three or four
// characters of the NAME before the specimen's ellipsis took over, on every
// row, which is the opposite of a control whose job is to show a name AND a
// face. Reusing the mechanism (the registry, the honesty rule, `lang="th"`)
// is Story 16.7's contract; reusing the browser's own sample sentence is not
// part of it, and the mockup never drew that sentence here to begin with.
const familyControlLatinSample = 'Aa Bb 123'
const familyControlThaiSample = 'กขค Aa'

// STORY 16.7 — RESOLVING A DECLARED CHAIN TO THE FACE IT PAINTS WITH, THE SAME
// WAY `TextPaint` RESOLVES A FRAGMENT (below, in this file): a carried entry
// counts only once its asset key is actually in `carriedFaces` (the paintable
// set built at the top of this component from `carriedFaceKeys`), and a
// shipped entry counts by its own engine name. ONLY THE CHAIN'S FIRST ENTRY IS
// CONSULTED. The engine alone decides, per glyph, which entry a real paragraph
// actually falls back to — that decision is AD-17's, and re-deriving it here
// against a fixed sample string is exactly the browser-side measurement AD-17
// forbids. The first entry is the chain's own declared primary, so a specimen
// set in it is an honest answer to "what face is this" even where a longer
// paragraph might fall through to the second.
//
// RETURNED AS `declaredEntry`, DELIBERATELY NOT `entry`. `canvas-font-stack.
// test.ts`'s authority census anchors its shipped-face pattern to the literal
// identifier `fragment` and poisons `entry.face` by name, because a document's
// declared chain entry is not the engine's attribution for a PAINTED fragment
// and must never stand in for one in the canvas's own paint path — the
// hazard argued at `TextPaint`, below. This value never reaches that path: it
// sets one `aria-hidden` specimen `<span>` with FIXED sample text, nothing is
// measured, and there is no sibling fragment for a mismatched entry to
// collide with. `declaredEntry` names that difference rather than hiding it.
function declaredChainEntry(name: string, chains: CanvasProjection['fontChains']): CanvasProjection['fontChains'][number]['entries'][number] | undefined {
  return chains.find((chain) => chain.name === name)?.entries[0]
}

/**
 * BEST-EFFORT SCRIPT COVERAGE FOR A DECLARED ROW'S SAMPLE TEXT. A chain
 * entry's own `family` is DISPLAY identity — the name a pick recorded, e.g.
 * `Inter` — so it is looked up the same two places every other row's coverage
 * comes from (`font-index.ts`'s local tier, then the snapshot), never
 * invented. A miss here — the two committed faces with no index row, or a
 * built-in engine name like `Noto Sans Thai` that names no offered family at
 * all — costs only which SAMPLE prints, never which face the row is set in.
 */
function scriptsForFamilyName(family: string): ReadonlyArray<string> {
  if (family === '') return []
  return catalogueFaces.find((face) => face.family === family)?.scripts ?? indexRowFor(family)?.scripts ?? []
}

function FontFamilyProperty({ families, fontChains, carriedFaces, specimenBytes, components, ids, onCommit, onUseFamily, onDeclareFamily, onOpenFontBrowser, browserOpen, storedFaces, heldLocalFamilies, onFamilyListOpened, pickBusy, pickError, documentGeneration, error }: { families: ReadonlyArray<string>; fontChains: CanvasProjection['fontChains']; carriedFaces: ReadonlySet<string>; specimenBytes: PreviewFaceBytes; components: ReadonlyArray<PanelComponent>; ids: ReadonlyArray<string>; onCommit: CommitProperties; onUseFamily: (source: FamilySource) => Promise<string | undefined>; onDeclareFamily: (source: FamilySource) => Promise<string | undefined>; onOpenFontBrowser: () => void; browserOpen: boolean; storedFaces: ReadonlyArray<StoredFace>; heldLocalFamilies: ReadonlySet<string>; onFamilyListOpened: () => void; pickBusy: boolean; pickError?: FontChainCommitError; documentGeneration: number; error?: PropertyCommitError }) {
  const values = components.map((component) => committedValue(component, 'fontFamily'))
  const uniform = sameProperty(components, 'fontFamily')
  const committed = uniform ? values[0] ?? '' : ''
  const [open, setOpen] = useState(false)
  // THE GROUP IS RE-READ EACH TIME THE LIST IS OPENED, which is what makes
  // "once its bytes are held it appears under AVAILABLE LOCALLY" true WITHOUT A
  // RELOAD (spec-deferred-offline-cache, story 2). An author installs a
  // catalogue family through `Add fonts…`, comes back to this control, and the
  // family is there. Nothing polls: opening the list is the only moment this
  // control's answer is looked at, so it is the only moment worth re-reading.
  useEffect(() => { if (open) onFamilyListOpened() }, [open, onFamilyListOpened])
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const [pending, setPending] = useState(false)
  const pendingRef = useRef(false)
  const listId = useId()
  // FOCUS RETURNS HERE WHEN THE BROWSER CLOSES (UX-DR25).
  //
  // THE CONTROL THAT OPENED IT CANNOT TAKE FOCUS BACK, WHICH IS WHY THIS IS THE
  // OWNER. The `Add fonts…` button lives inside the open dropdown and the
  // dropdown is closed on the way into the modal, so by the time the modal
  // unmounts its invoker is gone from the document — focus would land on
  // `<body>`, and a keyboard-only author would have to tab in from the top of
  // the page after every Escape. The family combobox is the control the door
  // belonged to and the one the author was working in, so it is where focus
  // goes: on Escape, on Cancel, on ×, and after a successful add.
  const field = useRef<HTMLInputElement>(null)
  const browserWasOpen = useRef(false)
  useEffect(() => {
    if (browserWasOpen.current && !browserOpen) field.current?.focus()
    browserWasOpen.current = browserOpen
  }, [browserOpen])
  // STORY 16.7 — THIS CONTROL'S OWN PREVIEW-FACE REGISTRY, ITS OWN INSTANCE.
  //
  // OPENED ON MOUNT AND CLOSED ON UNMOUNT, the same shape `FontBrowser.tsx`
  // opens its own in — this component just stays mounted across many opens and
  // closes of the DROPDOWN, where the modal only ever exists for one. "A face
  // is registered only while the dropdown is open, and released when it
  // closes" is enforced by the SHOW effect below rather than by tearing the
  // registry down every time: `show([])` releases every family it holds, so a
  // closed dropdown holds exactly zero preview faces on `document.fonts` —
  // observably identical to a torn-down registry — without re-losing every
  // in-flight fetch (and re-declining every failed one) on each reopen.
  //
  // THE RESOLVER IS READ THROUGH A REF for the same reason `FontBrowser` reads
  // its own through one: a caller re-creating the closure every render must
  // not tear the registry down and re-fetch everything on every keystroke.
  const specimenBytesRef = useRef(specimenBytes)
  useEffect(() => { specimenBytesRef.current = specimenBytes })
  const [specimenTick, setSpecimenTick] = useState(0)
  const [registry, setRegistry] = useState<PreviewFaceRegistry>()
  useEffect(() => {
    const opened = openPreviewFaceRegistry((family) => specimenBytesRef.current(family), () => setSpecimenTick((tick) => tick + 1))
    setRegistry(opened)
    return () => opened.close()
  }, [])
  const specimenStatus = (family: string): PreviewFaceStatus => {
    void specimenTick
    return registry?.statusOf(family) ?? 'preparing'
  }
  const needle = query.trim().toLowerCase()
  const hit = (name: string) => needle === '' || name.toLowerCase().includes(needle)
  const declared = families.filter(hit)
  // A catalogue family the document ALREADY declares a chain for is not
  // offered twice: the pick named the chain after the family, so the entry has
  // moved into the first group and showing it in both would make "declared"
  // and "not yet declared" stop meaning anything.
  // STORY 16.1: THE SECOND GROUP IS NO LONGER 21 ROWS. It is the local face
  // tier — the same 21 committed faces, which need no network — followed by the
  // families in the designer's build-time snapshot of the published library.
  // `offeredFamilies` owns the join and the filter; this control owns only the
  // one exclusion that is about THIS DOCUMENT: a family the document already
  // declares a chain for has moved into the first group and is not offered twice.
  // THE PARTITION. One predicate, two groups: `declared` above is
  // `families.includes(name)`, and `familyIsInstalled` is the one definition
  // of "this machine already holds it", shared with the browser's row state
  // so the two surfaces cannot disagree.
  //
  // STORY 16.9 REMOVED THE THIRD GROUP, AVAILABLE TO INSTALL. `offeredFamilies`
  // itself is untouched and still returns the web tier for the font browser,
  // which still draws it (`FontBrowser.tsx`) — this control simply never asks
  // for it, by filtering to `familyIsInstalled` before anything else. Nothing
  // downstream of this line ever sees a web-tier row, which is what makes
  // opening this dropdown a zero-fetch operation: a row that is never in
  // `onThisMachine` is never registered for a specimen, never rendered, and
  // never reachable by a pick.
  // AVAILABLE LOCALLY NOW MEANS GENUINELY HELD (spec-deferred-offline-cache,
  // story 2, owner decision 2026-09-19). `familyIsInstalled` reads the set of
  // catalogue families whose bytes this browser has actually fetched, so a
  // catalogue family nobody has used yet is simply NOT HERE — not greyed, not
  // relabelled, not a row that fetches when pressed. `Add fonts…` below is the
  // door to it, exactly as Story 16.9 made it the only door to the web tier.
  //
  // `IN THIS TEMPLATE` IS UNTOUCHED AND MUST STAY SO. `declared` above is the
  // document's own chains, and a font the open `.folio` carries is always
  // offered here whatever this browser's cache holds. Nothing in this line
  // reaches that group.
  const onThisMachine = offeredFamilies(query, storedFaces).filter((source) => !families.includes(source.family) && familyIsInstalled(source, heldLocalFamilies))
  // THE REGISTRY HOLDS EXACTLY THE FAMILIES THIS RENDER CAN SHOW A SPECIMEN
  // FOR — `onThisMachine`, and nothing else. NUL-JOINED for the reason
  // `FontBrowser.tsx`'s own key is: family names contain spaces. EMPTY
  // WHENEVER THE DROPDOWN IS CLOSED, so a stale key from the last time it was
  // open cannot re-open the registry's `show`.
  const installedFamilyKey = open ? onThisMachine.map((source) => source.family).join('\0') : ''
  useEffect(() => { registry?.show(installedFamilyKey === '' ? [] : installedFamilyKey.split('\0')) }, [registry, installedFamilyKey])
  // THE SCRIPT COVERAGE FOR EVERY `AVAILABLE LOCALLY` ROW, FROM THE SAME
  // DERIVATION THE FONT BROWSER'S OWN ROWS USE, so the two surfaces cannot
  // describe one family's coverage two different ways.
  const installedRowByFamily = new Map(browserRows(onThisMachine).map((row) => [row.family, row]))
  // GROUP 2 IS DELIBERATELY UNCAPPED, AND THE REVISIT TRIGGER IS NAMED RATHER
  // THAN LEFT TO BE NOTICED. Its population is the 31 committed faces plus
  // whatever this designer has downloaded, so it is tens of rows and not
  // thousands, and a heading saying a font is already on your machine may not
  // hide one. REVISIT WHEN THE MACHINE STORE CAN HOLD ON THE ORDER OF 200
  // ENTRIES: at that size the group needs its own bound, and the bound has to
  // arrive with something that still tells the truth about what it hid.
  //
  // THERE IS NO THIRD GROUP TO CAP (Story 16.9). The ~1,273 families this
  // machine does not hold are no longer offered here at all — `Add fonts…`
  // below is the only door to them — so the render-limit problem a cap once
  // solved does not exist in this control any more.
  //
  // ONE flat option list behind two visible groups, because the keyboard is
  // linear even when the list is not: `active` indexes this, arrow keys walk
  // it, and Enter dispatches whichever kind it lands on.
  const matches: ReadonlyArray<{ name: string; source?: FamilySource }> = [...declared.map((name) => ({ name })), ...onThisMachine.map((source) => ({ name: source.family, source }))]
  // THE TWO GROUPS, EACH OVER ITS OWN SLICE OF `matches`, so a row's option id
  // and its arrow-key position stay the flat index whatever the grouping does.
  // A heading is drawn only when its own group has rows after filtering.
  const groups: ReadonlyArray<{ key: string; label: string; from: number; rows: ReadonlyArray<{ name: string; source?: FamilySource }> }> = [
    { key: 'template', label: 'IN THIS TEMPLATE', from: 0, rows: matches.slice(0, declared.length) },
    { key: 'local', label: 'AVAILABLE LOCALLY', from: declared.length, rows: matches.slice(declared.length) },
  ]
  const close = () => { setOpen(false); setQuery(''); setActive(0) }
  const commit = async (intent: PropertyIntent) => {
    if (pendingRef.current) return
    pendingRef.current = true
    setPending(true)
    await onCommit(ids, intent, documentGeneration, ids.join(','))
    pendingRef.current = false
    setPending(false)
  }
  // FIRST USE — THE FORK'S OWN HALF FOR AN INSTALLED FAMILY. `onUseFamily` sends the embed and
  // resolves to a refusal sentence or to nothing; the property is committed ONLY
  // after it returns nothing, because `canvas.fontFamilies` is the closed set
  // `style.fontFamily` may name and the property command is refused until the
  // chain is declared. The engine forces the order; nothing here chooses it.
  //
  // TWO COMMANDS AND TWO UNDO ENTRIES, NOT ONE OF EITHER. There is no compound
  // command in this product and Story 8.6's refused fusion is not reopened.
  //
  // THE PENDING FLAG IS RELEASED BEFORE `commit`, because `commit` takes it
  // itself; holding it across both would make the second command drop silently.
  //
  // STORY 11.4 SPLIT THE FIRST HALF IN TWO AND LEFT THE SECOND ALONE. Which
  // command declares the chain depends on whether this release already ships
  // the family; that the chain is declared BEFORE the property, as two
  // commands and two undo entries, does not. So the shape below is one
  // function taking the dispatch, rather than two copies of the ordering that
  // could drift apart on the half that matters.
  const commitDeclaringChainFirst = async (family: string, declareChain: () => Promise<string | undefined>) => {
    if (pendingRef.current) return
    pendingRef.current = true
    setPending(true)
    let refusal: string | undefined
    try {
      refusal = await declareChain()
    } finally {
      pendingRef.current = false
      setPending(false)
    }
    if (refusal !== undefined) return
    await commit({ field: 'fontFamily', operation: 'set', value: family })
  }
  const commitFirstUse = (source: FamilySource) => commitDeclaringChainFirst(source.family, () => onUseFamily(source))
  // NAME AND DECLARE, THE OTHER HALF OF THE SAME FORK. The family is already
  // on every machine that can open the file, so nothing travels: the chain
  // names the shipped face and declares that family's cuts, and the document's
  // `assets` map is never touched.
  const commitDeclaredCuts = (source: FamilySource) => commitDeclaringChainFirst(source.family, () => onDeclareFamily(source))
  // THE FORK. A declared name is a property commit — today's behaviour, byte
  // for byte. A row carrying a `source` is always a family this machine
  // already holds: STORY 16.9 removed the dropdown's install-tier group, so
  // `match.source` can no longer name a family that is NOT on this machine
  // (`onThisMachine`, above, is the only source of a `source`-bearing row).
  //
  // TAKING IT IS THE FIRST USE — the moment the font starts travelling inside
  // the template — so `commitFirstUse` embeds from the machine and then
  // commits the property: the two decisions ("carry this typeface" and "draw
  // this box with it") are taken together here because the author took them
  // together. They are still TWO COMMANDS and two separately undoable
  // entries; only the trigger is one gesture. Fusing them into one command
  // would make that undo ambiguous, and there is no mechanism to fuse them
  // with (Story 8.6's refused fusion is not reopened).
  //
  // STORY 11.4 SPLIT THE `source` ARM ON ONE QUESTION: does this release
  // already ship the family? If it does, the pick NAMES it — a chain entry
  // carrying the shipped face and that family's declared cuts, and no asset at
  // all. If it does not, first use is still the moment the font starts
  // travelling and the embed is unchanged.
  //
  // ⚠ THE QUESTION IS MEMBERSHIP IN THE DECLARED MIRROR (`shipped-face-cuts.ts`)
  // and it is never `build-wasm.mjs`'s `shippedFamilies`, never a parse of
  // `fonts.go`, and never a comparison of bytes. `shippedFamilies` is the
  // BROWSER's CSS family registry and is measurably wrong in both directions
  // for this question — it omits plain `Roboto` and includes three IBM Plex
  // families the engine's FontSet has no key for at all.
  //
  // ⚠ AND IT IS ASKED OF THE FAMILY, NOT OF THE TIER, WHICH DECIDES ONE CASE
  // ON PURPOSE. `Noto Sans` and `Noto Sans Thai` are installable from the web
  // index, so a `stored` row for one of them can reach this fork carrying
  // bytes this designer fetched and kept. That row routes to the DECLARE path
  // and those bytes go unused — deliberately. They are a copy of a face the
  // release already ships to every machine that can open the file, so
  // embedding them would put ~348 KB into the document to reach a face already
  // reachable by name, and it would write a Regular-only entry that can never
  // bold while the shipped Bold sits in the same FontSet. The stored copy is
  // not wasted: it is what the designer paints the specimen with. Nothing is
  // deleted from the store either — this fork decides what a document carries,
  // not what this machine keeps.
  const choose = (match: { name: string; source?: FamilySource }) => {
    close()
    if (match.source) {
      if (isShippedFamily(match.source.family)) { void commitDeclaredCuts(match.source); return }
      void commitFirstUse(match.source)
      return
    }
    void commit({ field: 'fontFamily', operation: 'set', value: match.name })
  }
  const move = (step: number) => { if (matches.length > 0) setActive((current) => (current + step + matches.length) % matches.length) }
  // THE SPECIMEN, DRAWN FOR BOTH GROUPS THIS CONTROL RENDERS (Design Note 2 of
  // Story 16.7). STORY 16.9 REMOVED THE THIRD GROUP, `AVAILABLE TO INSTALL`,
  // which used to keep a per-row note instead of a specimen — that branch and
  // its row are both gone from this control now, not merely unreached.
  //
  // OMITTED ENTIRELY UNLESS THE FACE IS READY OR ALREADY ON THE PAGE. A row
  // whose face is still preparing, or that never resolves to one at all, shows
  // no specimen and never a sample set in a substitute face — the honesty rule
  // `FontBrowser.tsx:195-201` states in words, restated here as an omission
  // because a dropdown row has no room for the sentence.
  const specimenNode = (match: { name: string; source?: FamilySource }) => {
    if (match.source) {
      // EVERY `match.source` HERE IS A FACE THIS MACHINE ALREADY HOLDS.
      // `onThisMachine` is the only source of a `source`-bearing row now that
      // the third group is gone, so `installedRowByFamily` always has an
      // entry for one.
      const row = installedRowByFamily.get(match.source.family)
      if (row === undefined || specimenStatus(row.family) !== 'ready' || previewFaceFamily(row.family) === undefined) return undefined
      return <span className="property-option-specimen" aria-hidden="true" lang={row.scripts.includes('thai') ? 'th' : undefined} style={{ fontFamily: previewFaceFamily(row.family) } as CSSProperties}>{row.scripts.includes('thai') ? familyControlThaiSample : familyControlLatinSample}</span>
    }
    // A DECLARED CHAIN SHOWS THE FACE IT PAINTS WITH (Design Note 3), resolved
    // the way `TextPaint` resolves a fragment: a carried entry counts only
    // once its asset key is actually in `carriedFaces`, a shipped entry counts
    // by its own engine name — and NEITHER GOES THROUGH THIS CONTROL'S OWN
    // REGISTRY AT ALL, because the face is already on `document.fonts`,
    // registered for the open document or shipped with the build. Only the
    // chain's first entry is consulted; the engine alone decides per glyph
    // which entry a real paragraph actually uses (AD-17), and re-deriving
    // that here from a fixed sample string is the measurement AD-17 forbids.
    //
    // THE TWO BRANCHES ARE WRITTEN OUT RATHER THAN FUNNELLED THROUGH A SHARED
    // "resolve to a family" HELPER, so that each `fontFamily:` position in
    // this file names the exact approved derivation it uses in the clear —
    // `canvas-font-stack.test.ts`'s authority census reads designer source as
    // text, and a helper that returned a plain string would read as an
    // unapproved `css` at the one place that census actually looks.
    const declaredEntry = declaredChainEntry(match.name, fontChains)
    if (declaredEntry === undefined) return undefined
    const scripts = scriptsForFamilyName(declaredEntry.family)
    const lang = scripts.includes('thai') ? 'th' : undefined
    const sample = scripts.includes('thai') ? familyControlThaiSample : familyControlLatinSample
    if (isCarriedFaceAssetKey(declaredEntry.assetKey) && carriedFaces.has(declaredEntry.assetKey)) {
      return <span className="property-option-specimen" aria-hidden="true" lang={lang} style={{ fontFamily: embeddedFaceFamily(declaredEntry.assetKey) } as CSSProperties}>{sample}</span>
    }
    if (isShippedFaceName(declaredEntry.face)) {
      return <span className="property-option-specimen" aria-hidden="true" lang={lang} style={{ fontFamily: shippedFaceFamily(declaredEntry.face) } as CSSProperties}>{sample}</span>
    }
    return undefined
  }
  const errorId = error ? 'property-error-fontFamily' : undefined
  return <div className="property-editor property-combobox" onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) close() }}>
    <div className="property-field">
      <input ref={field} className="property-value property-value-prose" role="combobox" aria-label="Font family" aria-expanded={open} aria-controls={listId} aria-autocomplete="list" aria-activedescendant={open && matches.length > 0 ? `${listId}-${active}` : undefined} aria-description={uniform ? undefined : 'Mixed value'} aria-invalid={error ? 'true' : undefined} aria-errormessage={errorId} disabled={pending || pickBusy} value={open ? query : committed} placeholder={!uniform ? 'Mixed' : open ? 'Search fonts' : 'Choose a font'} onFocus={() => setOpen(true)} onChange={(event) => { setOpen(true); setQuery(event.target.value); setActive(0) }} onKeyDown={(event) => {
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); setOpen(true); move(event.key === 'ArrowDown' ? 1 : -1); return }
        if (event.key === 'Enter') { event.preventDefault(); const match = matches[active]; if (open && match) choose(match); return }
        if (event.key === 'Escape') { event.preventDefault(); close() }
      }} />
      {/* THE GLYPH IS THE MOCKUP'S OWN SVG PATH, NOT A TEXT CHARACTER.
          `Font Browser.dc.html:185` draws this exact chevron —
          `<svg width="8" height="8" viewBox="0 0 8 8" ...><path d="M1.5
          3l2.5 2.5L6.5 3">` — as ONE glyph, unconditionally: the mockup's
          `dropdownOpen` flag there gates the menu panel, not the chevron, so
          this button draws the same downward stroke whether open or closed.
          A first attempt swapped the text glyphs `⌃`/`⌄` (off-centre in
          OPPOSITE directions when measured by canvas ink-extent sampling —
          U+2303/U+2304 are accent-style glyphs anchored near cap-height, not
          shapes meant to fill their line) for `▲`/`▼`, which measured centred
          — but a filled triangle is the wrong SHAPE next to the design's thin
          stroke, the fix kept a two-state flip the design never had, and any
          text glyph centres by the FONT's metrics, not the box, so a UI
          typeface swap could silently reintroduce an off-centre glyph. An
          SVG box has no baseline: `.property-inline-action`'s own `display:
          grid; place-items: center` centres it structurally, the same way
          `align-items: center` centres the mockup's chevron in its row — a
          box property, not a font property, so it cannot drift when a face
          changes. `stroke="currentColor"` inherits this button's own
          ghost/hover/disabled color from `.property-inline-action` rather
          than hard-coding the mockup's static `#aab2bb`. */}
      <button type="button" className="property-inline-action property-disclosure" aria-label={open ? 'Hide fonts' : 'Show fonts'} title={open ? 'Hide fonts' : 'Show fonts'} disabled={pending} tabIndex={-1} onMouseDown={(event) => event.preventDefault()} onClick={() => (open ? close() : setOpen(true))}><svg aria-hidden="true" width="8" height="8" viewBox="0 0 8 8" fill="none" stroke="currentColor" strokeWidth="1.2"><path d="M1.5 3l2.5 2.5L6.5 3" /></svg></button>
      {/* STORY 16.9 REMOVED THE THIRD BUTTON THAT REVEALED THE CHAIN EDITOR,
          AND THE CLEAR BUTTON BESIDE IT. Text always has a typeface — there
          is no such thing as text with none — so a control offering to leave
          the field empty was offering something the product cannot do; see
          `FontChainEditor.tsx`'s own deletion and the code comment on
          `fontFamilies` for the chain data this UI removal leaves untouched. */}
    </div>
    {/* THE LISTBOX OWNS OPTIONS AND GROUPS OF OPTIONS, AND NOTHING ELSE (Story
        16.4, closing 8.6's deferral rather than multiplying it; narrowed to
        two groups by Story 16.9). It carried SIX `role="presentation"`
        children — two headings, an empty state, the disclosure, the cap note
        and the disk-font decline — which breaks a listbox's
        required-owned-elements rule. Both sanctioned repairs are used, each
        where it fits:

          · THE HEADINGS BECOME `role="group"` WITH AN `aria-label`. The visible
            heading is `aria-hidden` and the group carries the same words as its
            name, so the accessibility tree sees listbox → group → option with no
            stray text node in it, and a sighted reader still reads the heading.
          · THE NOTES MOVE OUT OF THE LIST ENTIRELY and are referenced with
            `aria-describedby`. They were never options: they describe the list.

        AND THE KEYBOARD WALK IS UNCHANGED, WHICH IS WHY THIS IS THE FIX AND NOT
        A SECOND DELIVERABLE. The one element in the walk that read POSITION
        semantically was the heading interleave this replaces (`index ===
        declared.length`); `move`, `active`, the option ids, `aria-activedescendant`
        and `choose` are all order-agnostic and are untouched. `active` still
        indexes the one flat `matches` array, and the groups are drawn over
        contiguous slices of it. */}
    {open && <div className="property-options">
      {/* NO CLASS ON THIS ELEMENT, DELIBERATELY. It carried
          `property-option-groups`, which `App.css` styled in zero places — a
          name that looks like a styling hook and is not one costs a reader the
          search. The shell above owns every box property, so the list needs no
          rule of its own; it is addressable by its role and its id. */}
      <div id={listId} role="listbox" aria-label="Fonts" aria-describedby={matches.length === 0 ? `${listId}-notes` : undefined}>
        {groups.filter((group) => group.rows.length > 0).map((group) => <div key={group.key} className={`property-option-group property-option-group-${group.key}`} role="group" aria-label={group.label}>
          <p className="property-option-heading" aria-hidden="true">{group.label}</p>
          {group.rows.map((match, offset) => {
            const index = group.from + offset
            // THE SPECIMEN REPLACES THE PER-ROW NOTE (Design Note 2 of Story
            // 16.7): a family this machine already holds, or a chain the
            // document already declares, draws a specimen instead of
            // restating what its own group heading already says. STORY 16.9
            // removed the one group whose rows fell back to a per-row note
            // instead — `AVAILABLE TO INSTALL` — so every row here now draws
            // a specimen or nothing, never a note.
            return <div key={`${group.key}:${match.name}`} id={`${listId}-${index}`} role="option" aria-selected={match.source === undefined && match.name === committed} className={`property-option${index === active ? ' property-option-active' : ''}${match.source ? ' property-option-catalogue' : ''}`} onMouseDown={(event) => event.preventDefault()} onMouseEnter={() => setActive(index)} onClick={() => choose(match)}><span className="property-option-name">{match.name}</span>{specimenNode(match)}</div>
          })}
        </div>)}
      </div>
      {matches.length === 0 && <div id={`${listId}-notes`} className="property-option-notes">
        {/* THE EMPTY STATE NAMES THE TWO PLACES IT LOOKED, because those are
            the two groups above it. STORY 16.9 dropped the third clause,
            "or in the list you can install" — this control no longer looks
            there at all, and `Add fonts…` below is where that search lives
            now. A control may not claim to have searched a place it never
            queried. */}
        {matches.length === 0 && <p className="property-option property-option-empty">{`Nothing in this template or on this machine matches "${query.trim()}".`}</p>}
      </div>}
      {/* STORY 16.3 — THE DOOR TO THE BROWSER, WHERE THE DESIGN PUTS IT: the last
          row of the open family dropdown, INSIDE the floating panel (Story 16.6 follow-on).
          It used to be a sibling of that panel, and `.property-combobox` is the
          positioning context, so `top: 100%` put the panel below the button and the
          design's last row rendered FIRST. Measured in Chromium before the move:
          button at y=433, list at y=481. The scroll now belongs to the listbox, so
          this row stays pinned at the foot of the panel instead of scrolling away. It is a real button OUTSIDE the
          `role="listbox"` rather than a fourth `role="presentation"` child inside
          it, because a listbox's children are options and this is not one — the
          keyboard walk over `matches` above must not land on it.

          NO KEYBOARD SHORTCUT AND NO HINT GLYPH, AND THE OMISSION IS RULED RATHER
          THAN FORGOTTEN (D-16.R.33 R2, owner-confirmed). The mockup prints `⌘G`
          beside this row; `⌘G` is the browser's own Find Next, and this
          application's convention puts conventional document actions on Command
          (⌘S, ⌘Z) and app-specific ones on Option (⌥P, ⌥S). `⌥F` is named as the
          eventual shape and is not bound in this epic — so no glyph is drawn,
          because a `⌘G` label beside a key that does nothing is a false UI
          string. `src/shortcuts.ts` is untouched. */}
      {/* THE ACCESSIBLE NAME CARRIES BOTH LINES. An `aria-label` REPLACES the
          element's contents for assistive technology, so naming this "Add fonts…"
          alone deleted the only sentence saying what the row does — the sub-label
          is visible to a sighted reader and was inaudible to everybody else. */}
      <button type="button" className="property-add-fonts" aria-label="Add fonts… Browse and embed web fonts" disabled={pending || pickBusy} onMouseDown={(event) => event.preventDefault()} onClick={() => { close(); onOpenFontBrowser() }}>
        <span className="property-add-fonts-label">Add fonts…</span>
        <span className="property-add-fonts-note">Browse and embed web fonts</span>
        </button>
    </div>}
    {pickError && <p role="alert" className="property-error">{pickError.message}</p>}
    {error && <p id={errorId} role="alert" className="property-error">{error.message}</p>}
  </div>
}
// The design's segmented control over one closed-set engine field. There is no
// separate clear button in the design and no room for one in a 1fr cell, so
// pressing the active segment again clears the property — the only way back to
// the value the document inherits, and the state the control already shows as
// "no segment pressed".
function SegmentedProperty({ label, field, segments, components, ids, onCommit, documentGeneration, error }: { label: string; field: 'align' | 'valign' | 'errorCorrection'; segments: ReadonlyArray<SegmentSpec>; components: ReadonlyArray<PanelComponent>; ids: ReadonlyArray<string>; onCommit: CommitProperties; documentGeneration: number; error?: PropertyCommitError }) {
  const values = components.map((component) => committedValue(component, field))
  const uniform = sameProperty(components, field)
  const current = uniform ? values[0] : undefined
  const [pending, setPending] = useState(false)
  const pendingRef = useRef(false)
  const commit = async (value: string) => {
    if (pendingRef.current) return
    pendingRef.current = true
    setPending(true)
    await onCommit(ids, current === value ? { field, operation: 'clear' } : { field, operation: 'set', value }, documentGeneration, ids.join(','))
    pendingRef.current = false
    setPending(false)
  }
  return <div className="property-editor"><SegmentedControl label={uniform ? label : `${label}, mixed`} segments={segments} current={current} disabled={pending} titleFor={(segment) => current === segment.value ? `${segment.label}, press again to clear` : segment.label} onPick={(value) => void commit(value)} trailing={!uniform && <span className="property-toggle-mixed" aria-hidden="true">Mixed</span>} />{error && <p role="alert" className="property-error">{error.message}</p>}</div>
}
// STORY 14.2 — THE ORIENTATION CONTROL, AND WHY IT IS A SIBLING OF
// `SegmentedProperty` RATHER THAN A WIDENING OF IT.
//
// `SegmentedProperty` writes ONE closed-set engine field and its `field` prop
// is typed `'align' | 'valign'`. This control writes no field of its own: it
// SWAPS two, and it must do so as ONE command. Widening the older control to
// carry a multi-intent would have put a second commit shape into a control that
// serves Align and Vertical align, for the benefit of one caller. (MEASURED, so
// the next reader does not re-derive it: `<SegmentedProperty` occurs twice in
// this file, both on the TYPOGRAPHY line. An earlier draft of this comment
// claimed eight sites and was wrong; the architectural argument never depended
// on the number.)
//
// IT IS ONE COMMAND, AND THAT IS THE POINT RATHER THAN A DETAIL. One
// `updateComponentProperties` carrying both `width` and `height` is one
// revision and ONE UNDO ENTRY, so a single undo returns both dimensions
// together. Two sequential single-field commands would be two of each — and
// would additionally pass through a transient shape (long AND thick, or short
// AND thin) that `containComponent` gets to refuse, because the engine
// contains each id once AFTER applying every change. There is no transient
// shape here to refuse.
//
// AND IT NEVER SNAPS. `updateComponentProperties` has no `snap` parameter to
// get wrong. `setComponentBounds` and `resizeComponent` both take one, and
// this would have been the codebase's only caller passing `snap: false` — a
// deviation whose failure mode is the next person tidying the inconsistency
// and silently destroying every thin rule in every document. Correct by
// construction beats correct by remembering.
//
// THE STATE IS DERIVED, NOT STORED. `lineOrientation` reads the committed box;
// pressing the segment that is already current sends nothing, because there is
// nothing to change and no property to clear — which is where this control
// deliberately differs from `SegmentedProperty`'s press-again-to-clear.
const orientationGlyphs: Readonly<Record<LineOrientation, string>> = { horizontal: 'M2 8h12', vertical: 'M8 2v12' }
function OrientationIcon({ variant }: { variant: LineOrientation }) {
  return <svg aria-hidden="true" className="segment-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2"><path d={orientationGlyphs[variant]} /></svg>
}
const orientationSegments: ReadonlyArray<Readonly<{ value: LineOrientation; label: string }>> = [{ value: 'horizontal', label: 'Horizontal orientation' }, { value: 'vertical', label: 'Vertical orientation' }]
// D-14.2.Q7. A SQUARE RULE HAS NO ORIENTATION TO CHANGE, AND THE PANEL SAYS SO
// INSTEAD OF OFFERING A CONTROL THAT DOES NOTHING.
//
// Swapping the dimensions of a square is the identity, so no implementation
// could make the press meaningful: the command would carry the numbers the
// document already holds, `folio-go/internal/wasm/engine.go` would find the produced bytes equal
// to the current bytes and not commit at all — no revision, no undo entry, no
// dirty flag — and the author would be left pressing a live control whose
// silence is its whole answer. In the epic whose subject is the panel telling
// the truth, the honest spelling is a disabled segment WITH ITS REASON BESIDE
// IT (DESIGN.md: "State the reason next to anything disabled").
//
// It is disabled FROM MOUNT rather than transiently, so the focused-control
// hazard the in-flight path avoids below does not arise here.
const squareRuleReason = 'A square rule has no orientation to change.'
function OrientationProperty({ component, ids, onCommit, documentGeneration, error }: { component: PanelComponent; ids: ReadonlyArray<string>; onCommit: CommitProperties; documentGeneration: number; error?: PropertyCommitError }) {
  const current = lineOrientation(component)
  const square = component.width === component.height
  const pendingRef = useRef(false)
  const swap = async (next: LineOrientation) => {
    if (next === current || square || pendingRef.current) return
    pendingRef.current = true
    try {
      // The two numbers exchanged, each spelled by `points` from the committed
      // box — never re-derived, never rounded, and never taken from a draft the
      // author may still be typing into.
      await onCommit(ids, [{ field: 'width', operation: 'set', value: points(component.height) }, { field: 'height', operation: 'set', value: points(component.width) }], documentGeneration, ids.join(','))
    } finally {
      // WITHOUT THE `finally` A THROWN COMMIT WEDGES THE CONTROL FOR THE LIFE OF
      // THE SELECTION: the flag would stay raised and every later press would
      // return at the guard above, silently. `applyProperties` catches its own
      // refusals today, so this is a guard against a future caller rather than a
      // reachable defect — which is exactly when a single-flight flag is easiest
      // to get wrong.
      pendingRef.current = false
    }
  }
  // ⚠ NO `disabled` WHILE A COMMAND IS IN FLIGHT, and that is the repository's
  // own measured rule rather than an omission. Disabling a FOCUSED control moves
  // focus to `<body>` and does not give it back when the control is re-enabled
  // (measured in Chromium 1217; see `PropertyDraft`'s arrow-step path, which
  // passes `disable = false` for the same reason). A toggle is pressed BY the
  // focused element, so raising a disabled flag here would throw the author's
  // focus away on every single orientation change. jsdom does not implement
  // blur-on-disable, so a green suite is not evidence here — the browser is.
  // The single-flight guard is `pendingRef` alone, which needs no re-render.
  const errorId = error ? 'property-error-orientation' : undefined
  const reasonId = square ? 'property-orientation-square' : undefined
  return <div className="property-editor"><div className="property-segmented" role="group" aria-label="Orientation">{orientationSegments.map((segment) => <button key={segment.value} type="button" className="property-segment" disabled={square && segment.value !== current} aria-pressed={current === segment.value} aria-label={segment.label} aria-describedby={segment.value === current ? undefined : reasonId} aria-invalid={error ? 'true' : undefined} aria-errormessage={errorId} title={segment.label} onClick={() => void swap(segment.value)}><OrientationIcon variant={segment.value} /></button>)}</div>{square && <p id={reasonId} className="property-unavailable">{squareRuleReason}</p>}{error && <p id={errorId} role="alert" className="property-error">{error.elementId ? `${error.elementId}: ` : ''}{printsDataPath(error) ? `${error.dataPath}: ` : ''}{error.message}</p>}</div>
}
function BorderEdgesProperty({ components, ids, onCommit, documentGeneration, error }: { components: ReadonlyArray<PanelComponent>; ids: ReadonlyArray<string>; onCommit: CommitProperties; documentGeneration: number; error?: PropertyCommitError }) {
  const same = sameProperty(components, 'borderEdges')
  const value = authoredValue(components[0]!, 'borderEdges')
  const edges: ReadonlyArray<string> = same && Array.isArray(value) ? value : []
  const [pending, setPending] = useState(false)
  const pendingRef = useRef(false)
  const update = async (next: ReadonlyArray<string>, clear = false) => {
    if (pendingRef.current) return
    pendingRef.current = true; setPending(true)
    await onCommit(ids, clear || next.length === 0 ? { field: 'borderEdges', operation: 'clear' } : { field: 'borderEdges', operation: 'set', value: next }, documentGeneration, ids.join(','))
    pendingRef.current = false; setPending(false)
  }
  return <div className="property-editor"><div className="property-edges" role="group" aria-label="Border edges"><span className="property-affix">Edges</span>{['top', 'right', 'bottom', 'left'].map((edge) => <label key={edge}><input type="checkbox" aria-label={`Border ${edge}`} aria-checked={same ? edges.includes(edge) : 'mixed'} ref={(node) => { if (node) node.indeterminate = !same }} disabled={pending} checked={edges.includes(edge)} onChange={() => void update(edges.includes(edge) ? edges.filter((value) => value !== edge) : [...edges, edge])} />{edge}</label>)}{!same && <span aria-label="Border edges mixed">Mixed</span>}{(edges.length > 0 || !same || components.some((component) => propertyPresent(component, 'borderEdges'))) && <button type="button" className="property-inline-action" aria-label="Clear Border edges" title="Clear Border edges" disabled={pending} onClick={() => void update([], true)}>×</button>}</div>{error && <p role="alert" className="property-error">{error.message}</p>}</div>
}


// STORY 12.1 adds `pageHeader` and `pageFooter`, and they are DELIBERATELY
// named for the bands themselves rather than for their position in the panel:
// applyPageSetup reads each row by the band name it sends, so there is no
// key-to-band map to rotate. `height` above is the PAGE's height and has
// nothing to do with either of them.
//
// THEY ARE OPTIONAL because a band the projection does not carry has no height
// to seed a row from, and `0` is a legal-looking height rather than an absence.
// Absent means the row is not shown and nothing is sent for it.
// `locale` and `utcOffset` are the DOCUMENT's two declared formatting
// authorities (Story 12.2), and they are REQUIRED rather than optional: unlike
// the two band keys above, every projection carries both — the loader refuses a
// document that declares neither — so an absent one is a channel fault, not a
// row that has nothing to show. They are strings like every other member here,
// including the locale, which the <select> constrains to LOCALE_TAGS at the
// point it is offered rather than by a type this record could rotate.
// THE EMPTY-LOCALE PLACEHOLDER IS NOT COSMETIC. `draftFor(undefined)` seeds
// `locale: ''`, and no tag option carries that value — so without the disabled
// `Not set` option the browser paints the FIRST option while React's value is
// `''`, and the panel asserts `en` for a document that has said nothing. The
// behaviour was already safe (Apply is disabled and applyPageSetup returns
// early), which is exactly what made it easy to leave: a control that shows a
// plausible wrong value in a state it can reach is Story 17.3's defect wearing
// a select instead of a number field. `locale === ''` iff there is no canvas —
// the select can only emit a LOCALE_TAGS member and the placeholder is
// disabled — but the label says `Not set` rather than `No document` so it stays
// honest if a later change makes the empty draft reachable some other way.
type Draft = { width: string; height: string; top: string; right: string; bottom: string; left: string; locale: string; utcOffset: string; pageHeader?: string; pageFooter?: string }
// The height the ENGINE says a band has, or `undefined` when the projection
// carries no such band. It is read off the projection and never measured, and
// it is the only number the difference test in applyPageSetup compares a draft
// against — so a fabricated `0` here would differ from any draft the author
// left alone and send a band-height command built on a number nobody projected.
function projectedBandHeight(canvas: CanvasProjection, band: CappingBand): number | undefined { return canvas.bands.find((candidate) => candidate.name === band)?.height }
function bandDraft(canvas: CanvasProjection, band: CappingBand): string | undefined { const height = projectedBandHeight(canvas, band); return height === undefined ? undefined : points(height) }
// `inputMode` DEFAULTS to 'decimal' and is a prop rather than a second
// component: every row this panel had before Story 12.2 is a number and must
// keep the numeric keypad, and the UTC offset is a `±HH:MM` string that must
// not get one. Duplicating Field to change one attribute would have put two
// nearly identical rows in the census that counts them. App.test.tsx asserts
// the split — every numeric row still carries inputMode="decimal" and the
// offset row does not — because flipping this default is otherwise a silent
// six-row change.
// spec-section-break: the break's Properties. One Y field, committed on blur
// or Enter exactly as PropertyDraft commits, sent UNSNAPPED — a typed value is
// written as typed. The panel is keyed on the projected offset, so an accepted
// value remounts it; a refused one leaves the offset unchanged and the field
// reverts to it while the canvas alert names what blocked it.
function SectionBreakProperties({ offset, anchor, onCommit, onAnchor }: { offset: number; anchor: boolean; onCommit: (draft: string) => void; onAnchor: (anchor: boolean) => void }) {
  const [draft, setDraft] = useState<string>()
  const committed = points(offset)
  const commit = () => {
    if (draft === undefined) return
    setDraft(undefined)
    if (draft !== committed) onCommit(draft)
  }
  return <><p className="section-label">SECTION BREAK</p><p className="honest-note">Content declared below this line moves together. If content above ends above the line, nothing moves.</p><label>Y (pt)<input aria-label="Y (pt)" inputMode="decimal" value={draft ?? committed} onChange={(event) => setDraft(event.target.value)} onBlur={commit} onKeyDown={(event) => { if (event.key === 'Enter') commit(); if (event.key === 'Escape') setDraft(undefined) }} /></label><label className="section-break-anchor"><input type="checkbox" aria-label="Anchor" checked={anchor} onChange={() => onAnchor(!anchor)} />Anchor</label><p className="honest-note">{anchor ? 'Anchored: when content above crosses the line, this section moves to a new page at its designed position.' : 'Not anchored: when content above crosses the line, this section moves down by that much on the same page, keeping its gap from the line, if it fits. Otherwise it starts a new page with the line at the top.'}</p></>
}

// SPEC-multi-pages story 2: THE SELECTED PAGE'S SECTION OF PAGE SETUP. Page
// setup itself is document-wide; the page's own setting is its Page Break,
// which does not apply to page 1 and is disabled there with the reason shown.
function PageSection({ page, pageBreak, disabled, onPageBreak }: { page: number; pageBreak: boolean; disabled: boolean; onPageBreak: (value: boolean) => void }) {
  const first = page === 0
  return <section className="property-section property-section-page"><p className="section-label">{`PAGE ${page + 1}`}</p><label className="page-break-setting"><input type="checkbox" aria-label="Page Break" checked={first || pageBreak} disabled={first || disabled} aria-describedby={first ? 'page-break-reason' : undefined} onChange={() => onPageBreak(!pageBreak)} />Page Break</label>{first ? <p id="page-break-reason" className="honest-note">Page 1 always starts the document, so Page Break does not apply to it.</p> : <p className="honest-note">{pageBreak ? 'On: this page starts on a new printed page after the previous page.' : 'Off: this page follows on from the previous page instead of always starting a new printed page.'}</p>}</section>
}

// SPEC-multi-pages story 2: THE DELETE PAGE CONFIRMATION. In-app, never
// window.confirm; it names the page, Cancel takes focus first, Tab stays inside
// it and Escape cancels. Keys never reach the canvas behind it.
function DeletePageDialog({ page, onConfirm, onCancel }: { page: number; onConfirm: () => void; onCancel: () => void }) {
  const confirm = useRef<HTMLButtonElement>(null)
  const cancel = useRef<HTMLButtonElement>(null)
  useEffect(() => { cancel.current?.focus() }, [])
  // A press on the backdrop would move focus to <body>, where Escape and Tab no
  // longer reach this dialog; keep it on Cancel instead.
  const holdFocus = (event: { target: EventTarget; preventDefault: () => void }) => {
    if (event.target instanceof Element && event.target.closest('.page-dialog') === null) { event.preventDefault(); cancel.current?.focus() }
  }
  return <section className="page-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="delete-page-title" aria-describedby="delete-page-description" onPointerDown={holdFocus} onMouseDown={holdFocus} onKeyDownCapture={(event) => {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); onCancel(); return }
    if (event.key !== 'Tab') { event.stopPropagation(); return }
    event.preventDefault(); event.stopPropagation()
    ;(document.activeElement === confirm.current ? cancel.current : confirm.current)?.focus()
  }}>
    <div className="page-dialog">
      <h2 id="delete-page-title">{`Delete page ${page + 1}?`}</h2>
      <p id="delete-page-description" className="honest-note">This removes the page and everything on it.</p>
      <div className="page-dialog-actions"><button ref={confirm} type="button" className="page-dialog-confirm" onClick={onConfirm}>Delete page</button><button ref={cancel} type="button" onClick={onCancel}>Cancel</button></div>
    </div>
  </section>
}

// THE UNSAVED-CHANGES WARNING BEFORE THE STARTUP DIALOG (spec-startup-templates
// story 4, owner renegotiation). DeletePageDialog's shape and `.page-dialog`
// styling: Keep editing is focused first, Tab stays between the two buttons,
// and Escape means Keep editing. No Save-first action.
function UnsavedChangesDialog({ document: name, onKeep, onDiscard }: { document: string; onKeep: () => void; onDiscard: () => void }) {
  const keep = useRef<HTMLButtonElement>(null)
  const discard = useRef<HTMLButtonElement>(null)
  useEffect(() => { keep.current?.focus() }, [])
  // A press on the backdrop would move focus to <body>, where Escape and Tab no
  // longer reach this dialog; keep it on Keep editing instead. A press on the
  // heading or text lands on the section itself (`tabIndex={-1}`, as in
  // StartupDialog), so its key handler still runs.
  const holdFocus = (event: { target: EventTarget; preventDefault: () => void }) => {
    if (event.target instanceof Element && event.target.closest('.page-dialog') === null) { event.preventDefault(); keep.current?.focus() }
  }
  return <section tabIndex={-1} className="page-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="unsaved-warning-title" aria-describedby="unsaved-warning-description" onPointerDown={holdFocus} onMouseDown={holdFocus} onKeyDownCapture={(event) => {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); onKeep(); return }
    if (event.key !== 'Tab') { event.stopPropagation(); return }
    event.preventDefault(); event.stopPropagation()
    ;(document.activeElement === keep.current ? discard.current : keep.current)?.focus()
  }}>
    <div className="page-dialog">
      <h2 id="unsaved-warning-title">Discard unsaved changes?</h2>
      <p id="unsaved-warning-description" className="honest-note unsaved-warning-description"><span className="unsaved-warning-dot" aria-hidden="true" /><span className="unsaved-warning-document">{name}</span>{' '}<span>has unsaved changes.</span></p>
      <div className="page-dialog-actions"><button ref={keep} type="button" onClick={onKeep}>Keep editing</button><button ref={discard} type="button" className="page-dialog-confirm" onClick={onDiscard}>Discard</button></div>
    </div>
  </section>
}

// THE UPDATE PROMPT, IN ITS TWO KINDS.
//
// OPTIONAL is the ordinary case and it is genuinely dismissible: the running
// release stays complete and usable, so an author mid-thought is entitled to say
// "not now" and never be asked again in this tab.
//
// MANDATORY is published by bumping the MAJOR version, and it blocks. What it
// must NOT do is take the document with it: activating the pending release
// reloads the tab, and anything unsaved dies in that reload. So a dirty document
// does not get an upgrade button AT ALL — the only way forward is through a
// save, and the button appears once the work is safe. The block and the data are
// not in tension here; the block simply waits.
function UpdateDialog({ version, mandatory, dirty, document: name, onLater, onUpgrade, onSave }: { version?: string; mandatory: boolean; dirty: boolean; document: string; onLater: () => void; onUpgrade: () => void; onSave: (saveAs: boolean) => void }) {
  const blocked = mandatory && dirty
  const named = version ? `Version ${version}` : 'A new version'
  return <section tabIndex={-1} className="page-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="update-dialog-title" aria-describedby="update-dialog-description" onKeyDownCapture={(event) => {
    // Escape is a dismissal, and a mandatory update has none. Swallowed rather
    // than ignored so it cannot fall through to whatever is behind the backdrop.
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); if (!mandatory) onLater(); return }
    event.stopPropagation()
  }}>
    <div className="page-dialog">
      <h2 id="update-dialog-title">{mandatory ? 'Update required' : 'Update available'}</h2>
      <p id="update-dialog-description" className="honest-note">
        {blocked
          ? <>{named} of folio8 is required, and <span className="unsaved-warning-document">{name}</span> has unsaved changes. Updating reloads this tab, so save your work to continue.</>
          : mandatory
            ? <>{named} of folio8 is required. Updating reloads this tab.</>
            : <>{named} of folio8 is ready. This version keeps working, so you can update whenever it suits you.</>}
      </p>
      <div className="page-dialog-actions">
        {!mandatory && <button type="button" onClick={onLater}>Later</button>}
        {blocked
          ? <><button type="button" onClick={() => onSave(true)}>Save As…</button><button type="button" className="page-dialog-confirm" onClick={() => onSave(false)}>Save</button></>
          : <button type="button" className="page-dialog-confirm" onClick={onUpgrade}>{mandatory ? 'Upgrade now' : 'Update now'}</button>}
      </div>
    </div>
  </section>
}

// A lower-case reason clause as a sentence, for an accessible description.
function reasonSentence(reason: string): string { return `${reason.charAt(0).toUpperCase()}${reason.slice(1)}.` }

function Field({ label, value, inputMode = 'decimal', onChange }: { label: string; value: string; inputMode?: 'decimal' | 'text'; onChange: (value: string) => void }) { return <label>{label}<input aria-label={label} inputMode={inputMode} value={value} onChange={(event) => onChange(event.target.value)} /></label> }
function bandName(name: CanvasProjection['bands'][number]['name']): string { return name === 'pageHeader' ? 'Page Header' : name === 'pageFooter' ? 'Page Footer' : 'Content' }
// THE BOUNDARY A BAND'S TOP EDGE IS. Two of the three bands have one: the top
// of `content` is the header/content boundary, and the top of `pageFooter` is
// the content/footer boundary. The top of `pageHeader` is the PAGE MARGIN — not
// a band boundary at all — so it gets no handle and no resize affordance
// (Matrix row 3).
function boundaryAbove(name: CanvasProjection['bands'][number]['name']): CappingBand | undefined { return name === 'content' ? 'pageHeader' : name === 'pageFooter' ? 'pageFooter' : undefined }
// The accessible name, and it deliberately does NOT reuse bandName. The regions
// are already named `Page Header` / `Content` / `Page Footer`, and
// application-shell.spec.ts and browser-native-roundtrip.spec.ts reach them by
// those exact names under Playwright strict mode — a handle sharing one would
// break two shipped specs on a duplicate match. It also names the ACT, which is
// what a control is for: the tab beside it is a label and stays one.
function boundaryLabel(band: CappingBand): string { return band === 'pageHeader' ? 'Resize the page header' : 'Resize the page footer' }
function points(value: number): string { const negative = value < 0; const magnitude = Math.abs(value); const whole = Math.floor(magnitude / 1000); const fraction = String(magnitude % 1000).padStart(3, '0').replace(/0+$/, ''); return `${negative ? '-' : ''}${whole}${fraction ? `.${fraction}` : ''}` }
function draftFor(canvas?: CanvasProjection): Draft { return canvas ? { width: points(canvas.commandWidth), height: points(canvas.commandHeight), top: points(canvas.marginTop), right: points(canvas.marginRight), bottom: points(canvas.marginBottom), left: points(canvas.marginLeft), locale: canvas.locale, utcOffset: canvas.utcOffset, pageHeader: bandDraft(canvas, 'pageHeader'), pageFooter: bandDraft(canvas, 'pageFooter') } : { width: '', height: '', top: '', right: '', bottom: '', left: '', locale: '', utcOffset: '' } }
// The canvas has one deliberately lossy display rounding rule. It maps only
// Go-owned millipoints plus local zoom; viewport, DPR, font metrics and DOM
// geometry are not inputs to painting or hit/drag proposals.
export const canvasDisplay = Object.freeze({
  css: (millipoints: number, zoom: number): string => `${Math.round(millipoints * zoom * 1000) / 1_000_000}px`,
  documentDelta: (pixels: number, zoom: number): number => Math.round((pixels / zoom) * 1000) / 1000,
})
// Pointer offsets are one local input event coordinate, not a layout query.
// They enter only this transient drop proposal and are converted through the
// same zoom mapping used by paint and drag; Go validates/snap-containes it.
export function placementPoint(event: Pick<MouseEvent, 'offsetX' | 'offsetY'>, band: CanvasProjection['bands'][number], zoom: number): Readonly<{ x: number; y: number }> {
  return { x: band.x / 1000 + canvasDisplay.documentDelta(event.offsetX, zoom), y: band.y / 1000 + canvasDisplay.documentDelta(event.offsetY, zoom) }
}
function pageStyle(canvas: CanvasProjection, zoom: number): CSSProperties { return { '--page-display-width': canvasDisplay.css(canvas.width, zoom), '--page-display-height': canvasDisplay.css(canvas.height, zoom), '--grid-display-pitch': canvasDisplay.css(canvas.gridIncrement, zoom), '--page-margin-left': canvasDisplay.css(canvas.marginLeft, zoom), '--page-margin-right': canvasDisplay.css(canvas.marginRight, zoom) } as CSSProperties }
function bandStyle(band: CanvasProjection['bands'][number], zoom: number, origin = 0, grid = 6000): CSSProperties { return { '--band-grid-offset': canvasDisplay.css(-(origin % grid), zoom), '--band-x': canvasDisplay.css(band.x, zoom), '--band-y': canvasDisplay.css(band.y, zoom), '--band-width': canvasDisplay.css(band.width, zoom), '--band-height': canvasDisplay.css(band.height, zoom) } as CSSProperties }
function pageSetupDiagnostic(error: unknown): string { const received = error as { code?: string; dataPath?: string; message?: string }; if (received.code === 'PAGE_SETUP_INVALID') return received.dataPath ? `${received.dataPath}: ${received.message ?? 'invalid value'}` : received.message ?? 'Page setup is invalid.'; return 'Page setup is invalid. Check the selected size and margins.' }
function componentDiagnosticDetail(error: unknown): Readonly<{ elementId?: string; dataPath?: string; message: string }> { const received = error as { elementId?: string; dataPath?: string; message?: string }; return { ...(received.elementId ? { elementId: received.elementId } : {}), ...(received.dataPath ? { dataPath: received.dataPath } : {}), message: received.message ?? 'Component change was rejected.' } }
function componentDiagnostic(error: unknown): string { const received = componentDiagnosticDetail(error); const prefix = received.elementId ?? received.dataPath; return prefix ? `${prefix}: ${received.message}` : received.message }

// STORY 12.5's transient gesture record. `original` and `limit` are read ONCE,
// at the press, off the projection that was in force then: re-reading them
// mid-drag would let a snapshot arriving from elsewhere move the anchor under
// the author's hand. `proposed` is the only field a move rewrites.
type BoundaryDrag = Readonly<{ band: CappingBand; pointerId: number; startClientY: number; original: number; limit: number; proposed: number; changed: boolean }>
type SectionBreakDrag = Readonly<{ pointerId: number; page: number; startClientY: number; original: number; limit: number; proposed: number; changed: boolean }>
type DragState = Readonly<{ id: string; mode: DragAnchor; startClientX: number; startClientY: number; x: number; y: number; width: number; height: number; originalX: number; originalY: number; originalWidth: number; originalHeight: number; changed: boolean; released?: boolean }>
// The layer belongs to the sheet, outside every band clip and stacking
// context. Only resize controls receive pointers; empty space still hits bands.
const CanvasSelectionLayerContext = createContext<HTMLDivElement | null>(null)
function CanvasSelectionLayer({ children }: { children: ReactNode }) {
  const [host, setHost] = useState<HTMLDivElement | null>(null)
  return <CanvasSelectionLayerContext value={host}>{children}<div className="canvas-selection-layer" ref={setHost} /></CanvasSelectionLayerContext>
}
function CanvasComponent({ component, carriedFaces, chromeOffset, origin, note, limit, zoom, selected, selectedColumnId, preview, engine, generation, trackColumn, onSelect, onBodyPress, onDelete, onDragStart, onDragEnd }: { component: CanvasProjection['components'][number]; carriedFaces: ReadonlySet<string>; chromeOffset: Readonly<{ x: number; y: number }>; origin: number; note?: string; limit: DragLimit; zoom: number; selected: boolean; selectedColumnId?: string; preview?: DragState; engine?: EngineClient; generation: number; trackColumn?: (edge: number, delta: number) => number; onSelect: (id: string, extend: boolean, target?: EventTarget | null) => void; onBodyPress: (id: string, event: PointerEvent) => boolean; onDelete: (event: KeyboardEvent) => boolean; onDragStart: (drag: DragState | undefined) => void; onDragEnd: (drag: DragState) => void }) {
  const chromeHost = useContext(CanvasSelectionLayerContext)
  const selectedByPointer = useRef(false)
  const proposal = preview ?? { x: component.x, y: component.y, width: component.width, height: component.height }
  // Component geometry is COLUMN geometry, in every band; this sheet shows one
  // window of it, and `origin` is where that window begins. It is 0 for the
  // repeating bands and 0 for the first window, which is why a single-sheet
  // document paints at exactly the coordinates it painted at before.
  const active = { ...proposal, y: proposal.y - origin }
  // A line is resized along its length; its cross-axis belongs to Thickness.
  // The engine snaps length and position while preserving the short axis.
  const lineAxis = component.type === 'line' ? lineOrientation(preview ? { ...component, width: preview.originalWidth, height: preview.originalHeight } : component) : undefined
  const anchors = lineAxis ? resizeAnchors.filter((anchor) => lineAxis === 'horizontal' ? anchor === 'w' || anchor === 'e' : anchor === 'n' || anchor === 's') : resizeAnchors
  // Long content keeps its authored box, but endpoints beyond the home
  // window must not become invisible controls over another page or footer.
  const chromeVisibleHeight = Math.min(active.height, Math.max(0, limit.height - active.y))
  const chromeOverflows = chromeVisibleHeight < active.height
  const visibleAnchors = anchors.filter((anchor) => {
    const offset = anchor === 'sw' || anchor === 's' || anchor === 'se' ? active.height : anchor === 'w' || anchor === 'e' ? active.height / 2 : 0
    return offset <= chromeVisibleHeight || preview?.mode === anchor
  })
  const begin = (event: PointerEvent, mode: DragAnchor) => { if (event.button !== 0) return; event.stopPropagation(); selectedByPointer.current = true; if (mode === 'move' && onBodyPress(component.id, event)) return; onSelect(component.id, event.shiftKey, event.target); if (event.shiftKey) return; event.currentTarget.setPointerCapture?.(event.pointerId); onDragStart({ id: component.id, mode, startClientX: event.clientX, startClientY: event.clientY, x: component.x, y: component.y, width: component.width, height: component.height, originalX: component.x, originalY: component.y, originalWidth: component.width, originalHeight: component.height, changed: false }) }
  // Ruling I. A linear pixel delta knows nothing about the page footer, the
  // gap and the page header standing between one window's foot and the next
  // window's head, so across a seam the component drifts from the hand by
  // exactly that much. `trackColumn` is the stack's own inverse: it converts
  // the distance the pointer travelled DOWN THE STACK into the distance it
  // travelled down the COLUMN, applied to the edge this anchor actually
  // moves. What Go receives is still one opaque command carrying a column
  // coordinate.
  const move = (event: PointerEvent) => { if (!preview) return; const rawDX = event.clientX - preview.startClientX; const rawDY = event.clientY - preview.startClientY; const changed = preview.changed || (preview.mode !== 'move' && lineAxis ? Math.abs(lineAxis === 'horizontal' ? rawDX : rawDY) >= 2 : Math.abs(rawDX) >= 2 || Math.abs(rawDY) >= 2); const dx = canvasDisplay.documentDelta(rawDX, zoom) * 1000; const travelled = canvasDisplay.documentDelta(rawDY, zoom) * 1000; const edge = preview.mode === 'sw' || preview.mode === 's' || preview.mode === 'se' ? preview.originalY + preview.originalHeight : preview.originalY; const dy = trackColumn ? trackColumn(edge, travelled) - edge : travelled; onDragStart({ ...preview, changed, ...proposedBounds(preview.mode, preview, dx, dy, limit, lineAxis === 'horizontal' ? Math.max(1000, preview.originalHeight) : lineAxis === 'vertical' ? Math.max(1000, preview.originalWidth + 1) : undefined) }) }
  const finish = (event: PointerEvent) => { if (!preview) return; event.stopPropagation(); onDragEnd(preview) }
  const paint = component.textPaint
  return <div className={`canvas-component canvas-component-${component.type}${paint?.overflow ? ' canvas-component-text-overflow' : ''}${selected ? ' canvas-component-selected canvas-component-external-chrome' : ''}`} aria-label={componentAccessibleName(component, note)} role="button" tabIndex={0} data-component-id={component.id} style={componentStyle(active, zoom)} onClick={(event) => { event.stopPropagation(); if (!selectedByPointer.current) onSelect(component.id, event.shiftKey, event.target); selectedByPointer.current = false }} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect(component.id, event.shiftKey) } if (selected && (event.key === 'Delete' || event.key === 'Backspace')) { event.stopPropagation(); if (onDelete(event.nativeEvent)) event.preventDefault() } }} onPointerDown={(event) => begin(event, 'move')} onPointerMove={move} onPointerUp={finish} onPointerCancel={() => onDragStart(undefined)}><ComponentBox component={component} zoom={zoom} />{paint?.truncated ? <span className="canvas-text-truncated">{canvasTruncationNotice}</span> : undefined}{paint ? <TextPaint component={component} carriedFaces={carriedFaces} zoom={zoom} /> : component.type === 'image' ? <ImagePaint component={component} zoom={zoom} engine={engine} generation={generation} /> : component.type === 'table' ? <TablePaint component={component} zoom={zoom} selectedColumnId={selectedColumnId} /> : component.type === 'barcode' ? <BarcodePaint component={component} zoom={zoom} /> : component.type === 'qrcode' ? <QRCodePaint component={component} zoom={zoom} /> : ''}{chromeHost && selected ? createPortal(<div className={`canvas-selection-chrome${chromeOverflows ? ' canvas-selection-chrome-overflow' : ''}`} style={{ ...componentStyle({ ...active, x: active.x + chromeOffset.x, y: active.y + chromeOffset.y }, zoom), '--chrome-visible-height': canvasDisplay.css(chromeVisibleHeight, zoom) } as CSSProperties}>{selected && <span className="canvas-dimension" aria-hidden="true">{points(active.width)} × {points(active.height)}</span>}{selected && component.resizable && visibleAnchors.map((anchor) => anchor === 'se'
      ? <button key={anchor} type="button" className="resize-handle" aria-label={`Resize ${component.id}`} onPointerDown={(event) => begin(event, anchor)} onPointerMove={move} onPointerUp={finish} onPointerCancel={() => onDragStart(undefined)} />
      : lineAxis ? <button key={anchor} type="button" className={`selection-handle selection-handle-${anchor}`} aria-label={`Resize ${component.id} ${anchor === 'w' || anchor === 'n' ? 'start' : 'end'}`} onPointerDown={(event) => begin(event, anchor)} onPointerMove={move} onPointerUp={finish} onPointerCancel={() => onDragStart(undefined)} />
      : <span key={anchor} className={`selection-handle selection-handle-${anchor}`} aria-hidden="true" onPointerDown={(event) => begin(event, anchor)} onPointerMove={move} onPointerUp={finish} onPointerCancel={() => onDragStart(undefined)} />)}</div>, chromeHost) : undefined}</div>
}
// ImagePaint is Story 5.13's canvas producer: it paints ONLY inside the
// fit-and-centre draw rectangle Go already computed (component.image), never
// a rectangle CSS or this component negotiates on its own (AD-17/guardrail
// 5) — object-fit is never used; the <img> is sized to the EXACT draw
// rectangle Go supplied, so the browser has no fitting decision left to
// make. Paintable bytes are fetched separately, per asset key, on a fresh
// effect run keyed by (assetKey, generation) — never cached across a
// document replacement — and the object URL this effect creates is
// revoked in its own cleanup, covering deletion (unmount), asset
// replacement (assetKey changes) and document replacement (generation
// changes) alike, with no accumulation across a session.
function ImagePaint({ component, zoom, engine, generation }: { component: CanvasProjection['components'][number]; zoom: number; engine?: EngineClient; generation: number }) {
  const image = component.image
  const [url, setUrl] = useState<string>()
  // Finding 13 (review of 2026-08-29): a failed per-key 'asset' fetch used
  // to leave `url` undefined forever, which rendered as "Loading image…"
  // permanently — the opposite of AC3's "honest placeholder" for a
  // component that cannot be painted. Track failure as its own state,
  // distinct from "still in flight", so the placeholder can say which one
  // it is. Causes include a stale/malformed key (Finding 12, now rejected
  // earlier by admission) and any transport failure.
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    setUrl(undefined)
    setFailed(false)
    if (!engine || !image) return
    let active = true
    let created: string | undefined
    void engine.request('asset', assetBytesRequest(image.assetKey)).then((result) => {
      if (!active) return
      if (!result.bytes) { setFailed(true); return }
      created = URL.createObjectURL(new Blob([result.bytes], { type: image.mediaType }))
      setUrl(created)
    }).catch(() => { if (active) setFailed(true) })
    return () => { active = false; if (created) URL.revokeObjectURL(created) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [engine, image?.assetKey, image?.mediaType, generation])
  if (!image) {
    // Finding 9: echo which of the two Go-side reasons applies, matching
    // ImageSection's text — one Go signal drives both surfaces. No reason at
    // all is the empty box a placed image starts as, which the design draws
    // as a dashed placeholder rather than a failure.
    if (!component.imageUnavailable) return <ImagePlaceholder>No image</ImagePlaceholder>
    return <ImagePlaceholder>{component.imageUnavailable === 'missing' ? 'Image unavailable — its asset is not present in the document' : 'Image unavailable — this version cannot render its media type'}</ImagePlaceholder>
  }
  const style: CSSProperties = { position: 'absolute', left: canvasDisplay.css(image.drawX - component.x, zoom), top: canvasDisplay.css(image.drawY - component.y, zoom), width: canvasDisplay.css(image.drawWidth, zoom), height: canvasDisplay.css(image.drawHeight, zoom) }
  if (url) return <img src={url} alt="" aria-hidden="true" draggable={false} className="canvas-image-paint" style={style} />
  return <ImagePlaceholder>{failed ? 'Image unavailable — could not load its bytes' : 'Loading image…'}</ImagePlaceholder>
}
// BarcodePaint draws the bars Go computed (component.barcode): each bar's X and
// width in millipoints relative to the component, full height. The browser
// encodes nothing and measures nothing (AD-17); when Go reports why a barcode
// cannot be painted, that bounded reason is echoed instead.
const barcodeUnavailableText: Readonly<Record<'unencodable' | 'doesNotFit', string>> = {
  unencodable: 'Barcode not drawn — its value has a character Code 128 cannot encode',
  doesNotFit: 'Barcode not drawn — it does not fit its box',
}
function BarcodePaint({ component, zoom }: { component: CanvasProjection['components'][number]; zoom: number }) {
  const paint = component.barcode
  if (!paint) return <span className="canvas-image-placeholder" aria-hidden="true"><svg className="canvas-placeholder-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="square">{paletteGlyphs.barcode}</svg><span>{component.barcodeUnavailable ? barcodeUnavailableText[component.barcodeUnavailable] : 'No barcode content'}</span></span>
  return <span className="canvas-barcode-paint" aria-hidden="true">{paint.bars.map((bar) => <span key={bar.x} className="canvas-barcode-bar" style={{ left: canvasDisplay.css(bar.x, zoom), width: canvasDisplay.css(bar.width, zoom) }} />)}</span>
}
// QRCodePaint draws the module runs Go computed (component.qrcode): each rect's
// position and size in millipoints relative to the component. The browser
// encodes nothing and measures nothing (AD-17); when Go reports why a QR code
// cannot be painted, that bounded reason is echoed instead.
const qrcodeUnavailableText: Readonly<Record<'tooLong' | 'doesNotFit', string>> = {
  tooLong: 'QR code not drawn — its value is too long for a QR Code at this level',
  doesNotFit: 'QR code not drawn — it does not fit its box',
}
function QRCodePaint({ component, zoom }: { component: CanvasProjection['components'][number]; zoom: number }) {
  const paint = component.qrcode
  if (!paint) return <span className="canvas-image-placeholder" aria-hidden="true"><svg className="canvas-placeholder-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="square">{paletteGlyphs.qrcode}</svg><span>{component.qrcodeUnavailable ? qrcodeUnavailableText[component.qrcodeUnavailable] : 'No QR code content'}</span></span>
  return <span className="canvas-qrcode-paint" aria-hidden="true">{paint.rects.map((rect) => <span key={`${rect.y}:${rect.x}`} className="canvas-qrcode-rect" style={{ left: canvasDisplay.css(rect.x, zoom), top: canvasDisplay.css(rect.y, zoom), width: canvasDisplay.css(rect.width, zoom), height: canvasDisplay.css(rect.height, zoom) }} />)}</span>
}
// The page's placeholder frame: the palette's own image glyph over the reason
// this box has nothing to paint. The reason text is unchanged — Go states
// which of its two cases applies and this still echoes it.
function ImagePlaceholder({ children }: { children: string }) {
  return <span className="canvas-image-placeholder" aria-hidden="true"><svg className="canvas-placeholder-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="square">{paletteGlyphs.image}</svg><span>{children}</span></span>
}
// STORY 14.9 — THE CANVAS DRAWS THE TABLE IT WILL PRINT, and this is the only
// place in the designer that paints a string which ALSO ENDS UP IN THE PDF.
//
// R1's THREE-CONDITION DISPLAY-PAINT TEST is what makes that legal, and the
// canonical statement of it — with condition 3's enforcement named and the
// precedent explicitly bounded — lives in `canvas-authority-contract.test.ts`'s
// own comment block, because that file is read years later and a spec is read
// once. In short: (1) the rectangle is the ENGINE's, (2) the browser makes no
// break decision, (3) nothing derived from the painted text flows back. Every
// element below that paints an engine-owned string carries
// `.canvas-display-paint` — named for the PROPERTY, display-only paint, so the
// next author with a candidate for this exception recognises their own case in
// it, and so grepping that class lands on the three-condition rule.
//
// CONDITION 1, MECHANICALLY. The surface is `inset: 0` on `.canvas-component`,
// whose box is the engine's `x/y/width/height`; the column TRACKS are the
// engine's `column.width` through `canvasDisplay.css`, the one millipoints→
// display mapping the whole canvas uses; and horizontal overflow is CLIPPED by
// CSS, exactly as `.canvas-text-paint` clips it. No browser measurement
// contributes to any of it — there is no arithmetic here beyond that zoom scale.
//
// CONDITION 2. `white-space: pre` and `overflow: hidden` with a CSS
// `text-overflow` on every text cell. There is no computed ellipsis anywhere:
// the browser decides where the glyphs stop, and NOTHING reads that decision.
// The permitted residue is exactly epics.md's Story 5.13 AC — a label may CLIP
// where the PDF wraps, a text-only inaccuracy and never a geometry error.
//
// CONDITION 3. Nothing here derives a width, a height, a line count, an
// overflow state or a scroll extent from anything painted, and
// canvas-authority-contract.test.ts scans this file for every API by which such
// a quantity could be OBTAINED at all: a value that cannot be obtained cannot
// flow back.
//
// AND IT ADDS NO INTERACTIVE ELEMENT. No `role`, no `tabIndex`, no second
// `data-component-id`, no `aria-label` on any inner node: `.canvas-component`
// stays the single `role="button"` the control vocabulary sweeps, and its
// accessible name is unchanged. Making a column selectable is Story 14.10's
// work and its selection-model change; 14.9 paints only.
//
// ⚠ AND THE PAINT IS DELIBERATELY NOT `aria-hidden`. `control-vocabulary-
// contract.test.tsx`'s `treatmentOf` reads a control's visible text with the
// `aria-hidden` subtrees stripped and falls through to 'glyph' when it finds an
// `<svg>`; hiding the whole table and leaving the chip's glyph inside it would
// flip this control from 'word' to 'glyph' and move a pinned census. Only the
// two decorative glyphs are hidden.
// trackWidth is a DISPLAY-ONLY FLOOR, and it refuses nothing: the projection
// still carries the engine's value verbatim, the guard still accepts it, and
// the inspector still reads it. It exists because a NEGATIVE `<length>` is not
// a valid grid track size, so one negative column would invalidate the whole
// `grid-template-columns` declaration — and the browser would then auto-size
// EVERY track from content, handing the engine's geometry back to the very
// layout engine AD-15 and R1's condition 1 exist to keep out of it. A negative
// width loads and paints today (the matrix requires it to), so the choice is
// between drawing a zero-width track for that one column and drawing every
// column at whatever width the browser feels like. It is a clamp on the paint,
// not arithmetic on the document: nothing derived from it reaches a command, a
// projection field or a geometry decision.
function trackWidth(width: number): number { return Math.max(0, width) }
const canvasTableUnsetBinding = 'Not set'
// The design draws neither of these two sentences, so they are MATCHED rather
// than invented: `Not set` is already the shipped word for an unset binding
// (the honest note renders `Table binding: {tableBind ?? 'Not set'}`), and
// `No columns yet.` is the opening of what `TableEditor.tsx` already says.
//
// ⚠ THE EDITOR'S SECOND SENTENCE IS DELIBERATELY NOT REPEATED HERE. It reads
// "Add a column to start the matrix.", and in the editor it sits beside an
// `Add column` BUTTON that carries it out. The canvas offers no such control —
// 14.9 paints only — so repeating the instruction would point at nothing. The
// vocabulary is the design's; the call to action belongs to the surface that
// can honour it.
const canvasTableNoColumnsNotice = 'No columns yet.'
// STORY 14.10 — `selectedColumnId` IS THE ONLY THING THIS PAINTER GAINED, and
// it is a marking input, not a model: the columns still come from the engine's
// projection, in the engine's order, at the engine's widths. The ECHO passes
// nothing, so a repeated body on a later sheet paints unmarked — it carries no
// role, no handlers and no name either, and a second cyan column would claim a
// selection the author cannot act on there.
function TablePaint({ component, zoom, selectedColumnId }: { component: CanvasProjection['components'][number]; zoom: number; selectedColumnId?: string }) {
  const columns: ReadonlyArray<CanvasTableColumn> | undefined = component.columns
  // ABSENCE, NOT AN EMPTY FRAME. Go omits the member entirely for a table that
  // declares no columns, and DESIGN.md's placeholder grammar — "Dashed grey on
  // page | A placeholder with no content yet" — is what says so: the glyph, the
  // reason in words, and App.css flipping this component's outline from dotted
  // to dashed, the same idiom ImagePlaceholder already ships.
  if (columns === undefined) return <span className="canvas-table-empty"><svg className="canvas-placeholder-icon" aria-hidden="true" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="square">{paletteGlyphs.table}</svg><span>{canvasTableNoColumnsNotice}</span></span>
  const collection = component.tableBind ?? ''
  // THE TABLE'S DECLARED CELL PADDING, left and right, mapped through the one
  // display rule. Absent keeps the stylesheet's drawing inset, so a table that
  // declares none paints exactly as before. A negative length (which the loader
  // admits) cannot be drawn by CSS and is floored at zero here.
  const inset: CSSProperties = { ...(component.paddingLeft === undefined ? {} : { paddingLeft: canvasDisplay.css(Math.max(0, component.paddingLeft), zoom) }), ...(component.paddingRight === undefined ? {} : { paddingRight: canvasDisplay.css(Math.max(0, component.paddingRight), zoom) }) }
  return <span className="canvas-table">
    <span className="canvas-table-chip">
      <svg className="canvas-table-chip-icon" aria-hidden="true" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="square">{paletteGlyphs.table}</svg>
      {/* The bind accent means DATA and only data (DESIGN.md), so a table that
          is bound to nothing says so in the muted page ink instead — painting
          `Not set` in amber would say the opposite of what the accent means. */}
      <span className={collection === '' ? 'canvas-table-unset canvas-display-paint' : 'canvas-table-collection canvas-display-paint'}>{collection === '' ? canvasTableUnsetBinding : collection}</span>
      <span className="canvas-table-count canvas-display-paint">{columns.length === 1 ? '1 column' : `${columns.length} columns`}</span>
    </span>
    {/* ONE GRID, TWO ROWS, AND THE TRACKS ARE THE ENGINE'S DECLARED WIDTHS.
        TableEditor.dc.html states the rule this honours: "Column widths are
        fixed and never negotiated against content." A zero or negative width is
        passed through as the engine gave it — both load and paint today, and
        the canvas refuses nothing it accepts. */}
    <span className="canvas-table-grid" style={{ gridTemplateColumns: columns.map((column) => canvasDisplay.css(trackWidth(column.width), zoom)).join(' ') }}>
      {/* THE HEADER ROW CONSUMES `headerAlign` AND THE ROW BELOW `cellAlign` —
          never one value used twice. The engine resolves them through two
          different cascades (resolveHeaderStyle takes headerStyle.align first;
          resolveBodyStyle never sees headerStyle at all), so a table whose
          headerStyle.align differs from its style.align aligns its two rows
          differently, and so does this drawing. Swapping these two keys is
          meant to turn a test red. */}
      {columns.map((column) => <span key={`${component.id}-heading-${column.id}`} data-column-id={column.id} className={`canvas-table-heading canvas-display-paint${column.id === selectedColumnId ? ' canvas-table-column-selected' : ''}`} style={{ textAlign: column.headerAlign, ...inset }}>{column.labelLines.map((line, index) => <span key={index} className="canvas-table-heading-line">{line}</span>)}</span>)}
      {/* THE LABEL IS PAINTED FROM `labelLines`, NEVER FROM `label`
          (SPEC-table-rules §4). A label may wrap in the PDF and may hold an
          authored line feed; both breaks are the ENGINE's, packed in Go by the
          same packer a data cell uses, and arrive here as one string per line.
          Each line is its own block so the browser makes no break decision —
          handing it `label` to wrap would be AD-17 condition 2 failing. */}
      {/* ONE REPRESENTATIVE ROW, SHOWING EACH COLUMN'S BINDING RATHER THAN A
          VALUE: a canvas shows the SHAPE of the document, not its contents, and
          the canvas has no data. A column nobody has pointed at data yet reads
          as unbound in the muted ink — not in the bind accent, because an
          unbound cell has no data to mark. */}
      {columns.map((column) => <span key={`${component.id}-cell-${column.id}`} data-column-id={column.id} className={`${column.bind === '' ? 'canvas-table-unset canvas-display-paint' : 'canvas-table-cell canvas-display-paint'}${column.id === selectedColumnId ? ' canvas-table-column-selected' : ''}`} style={{ textAlign: column.cellAlign, ...inset }}>{column.bind === '' ? canvasTableUnsetBinding : column.bind}</span>)}
    </span>
  </span>
}
// Display-only reading aid. Go painted this text and owns the expression
// grammar outright — scope, validation, evaluation, diagnostics. Tinting the
// delimiters it already painted decides nothing and parses nothing beyond the
// literal braces standing in the run.
const expressionRun = /(\{\{[^{}]*\}\})/g
function textRuns(text: string): ReadonlyArray<string> { return text.split(expressionRun).filter((part) => part !== '') }
function isExpressionRun(part: string): boolean { return part.startsWith('{{') && part.endsWith('}}') }
// STORY 8.4a — THE ONE FONT-FAMILY THE CANVAS EVER ASKS FOR BY NAME AT
// RUNTIME, and it is set PER FRAGMENT because a fragment is exactly one face
// by construction (the engine emits at most one run per face segment and never
// merges adjacent runs) while a component is not: a mixed-script element draws
// Latin through one chain entry and Thai through another.
//
// TWO POPULATIONS, TWO SEAMS, ONE EXPRESSION (Story 8.4e). A fragment carries
// exactly one of the engine's two identities for the face it was measured
// with, and each has its own derivation module and nothing else derives it:
//
//	assetKey -> embedded-face-family.ts  (the face the DOCUMENT carries)
//	face     -> shipped-face-family.ts   (the face the BUILD ships)
//
// THE CARRIED BRANCH is set only when the engine attributed this fragment to
// an asset the document carries *and* that asset's face has actually reached
// the page's font set. Both halves matter. Without the first, a face would be
// asked for under a family nothing declares; without the second, a fetch that
// failed would take the fragment OFF the stylesheet's declared stack — an
// inline declaration replaces the rule rather than extending it — and the
// canvas would paint with whatever the browser defaults to.
//
// THE SHIPPED BRANCH needs no such registration check: Story 8.4b declares an
// `@font-face` for each of the engine's own face names at build time, over the
// engine's own bytes, so the name the engine measured with is already
// resolvable. It carries the declared stack as its own tail, so a codepoint
// the attributed face does not cover still reaches the other shipped faces
// rather than the browser's default. Until this story it was set to NOTHING,
// and the fragment fell to one fixed stylesheet stack whatever order the
// document declared — which for a chain like ["Noto Sans Thai"] rasterized
// Latin with Noto Sans while the engine had measured it with Noto Sans Thai.
//
// WITH NEITHER identity the fragment falls to `.canvas-text-fragment`'s
// declared stack in App.css, which is the degrade path and the only path left
// for an unattributed fragment.
//
// Each family is derived FROM THE ENGINE'S OWN IDENTITY (D-8.4.1, D-8.4.14) by
// the one module that makes that decision. Nothing here reads a chain entry's
// `family`, `style` or a chain name, and the stylesheet still holds no
// document input at all.
//
// TextPaint is EXPORTED so the per-fragment face attribution can be asserted
// on a real DOM node rather than by scanning this file's text: what the canvas
// asks for is a rendered fact, and canvas-font-stack.test.ts reads it off the
// element.
export function TextPaint({ component, carriedFaces, zoom }: { component: CanvasProjection['components'][number]; carriedFaces: ReadonlySet<string>; zoom: number }) {
  const paint = component.textPaint!
  return <span className="canvas-text-paint" aria-hidden="true" style={{ '--text-font-size': canvasDisplay.css(component.fontSize ?? 12000, zoom), ...(component.color === undefined ? {} : { '--text-ink': component.color }) } as CSSProperties}>{paint.lines.map((line, lineIndex) => <span className="canvas-text-line" key={`${component.id}-${lineIndex}`} style={{ '--text-line-baseline': canvasDisplay.css(line.baseline - component.y, zoom), '--text-line-advance': canvasDisplay.css(line.advance, zoom) } as CSSProperties}>{line.fragments.map((fragment, fragmentIndex) => <span className="canvas-text-fragment" key={`${component.id}-${lineIndex}-${fragmentIndex}`} style={{ '--text-fragment-x': canvasDisplay.css(fragment.x - component.x, zoom), ...(fragment.assetKey !== undefined && carriedFaces.has(fragment.assetKey) ? { fontFamily: embeddedFaceFamily(fragment.assetKey) } : isShippedFaceName(fragment.face) ? { fontFamily: shippedFaceFamily(fragment.face) } : {}) } as CSSProperties}>{textRuns(fragment.text).map((part, partIndex) => isExpressionRun(part) ? <span className="canvas-text-expression" key={`${component.id}-${lineIndex}-${fragmentIndex}-${partIndex}`}>{part}</span> : part)}</span>)}</span>)}</span>
}
// The engine says this element's paint is a PREFIX. It is stated in words, at
// the component, in the same sentence a screen reader gets — not by colour,
// and not only by a class. (The older `overflow` flag sets
// `canvas-component-text-overflow` and NOTHING ELSE: there is no CSS rule for
// that class anywhere, so it is invisible to the author. Noted, deliberately
// not fixed here; repeating the shape is what this avoids.)
//
// It says what is true of the CANVAS, and what is still true of the document:
// the value is intact and prints whole. Nothing here is derived from how many
// lines were painted — the canvas must never turn a truncated paint into a
// number about the document.
const canvasTruncationNotice = 'Canvas preview cut short. The whole text is in the document and prints in full.'
// AC4's PER-COMPONENT half, folded into the accessible name exactly as the
// truncation notice above is — the shipped idiom, and the reason this is an
// obligation rather than a tooltip nobody reads. A component that sits on a
// later sheet is not pinned there: the sheet it lands on is a consequence of
// everything above it in the column, and the canvas has no data, so the page
// it prints on can differ from the page drawn here.
// D-4.1: a multi-page document names the SHEET here, since its page names count
// designed pages.
const canvasColumnPositionNotice = (page: number, pages: number, noun: 'page' | 'sheet' = 'page'): string => `on canvas ${noun} ${page} of ${pages}, which is a consequence of the content above it and can change when the data does — a column position, not a pin to ${noun} ${page}`
function componentAccessibleName(component: CanvasProjection['components'][number], note?: string): string {
  const page = note ? `; ${note}` : ''
  if (component.type !== 'text') return `${component.type} component ${component.id}${page}`
  const text = component.textPaint?.lines.map((line) => line.fragments.map((fragment) => fragment.text).join('').trim()).filter(Boolean).join(' ').slice(0, 160)
  const binding = component.binding ? `; bound to ${component.binding}` : ''
  const cut = component.textPaint?.truncated ? `; ${canvasTruncationNotice}` : ''
  return text ? `text component ${component.id}: ${text}${binding}${cut}${page}` : `text component ${component.id}${binding}${cut}${page}`
}
// A component whose extent crosses a window boundary is drawn on every window
// it intersects, because leaving the later sheets empty would make the
// drawing a lie in a second way. Only the HOME occurrence carries the role,
// the tab stop and the name; selected echoes accept body drags and Shift
// toggles but have no handles, so one component never presents two identical
// accessible names (Ruling G).
//
// SPEC-multi-pages story 4: a header or footer echo takes `onPress`, and then
// responds to a press whether selected or not (a repeating band's copies are
// one component, editable from any page). Content echoes never take it.
function ComponentEcho({ component, carriedFaces, selected, onSelect, onBodyPress, onPress, y, zoom, engine, generation }: { component: CanvasProjection['components'][number]; carriedFaces: ReadonlySet<string>; selected?: boolean; onSelect?: (id: string, extend: boolean) => void; onBodyPress?: (event: PointerEvent) => void; onPress?: (event: PointerEvent) => void; y: number; zoom: number; engine?: EngineClient; generation: number }) {
  const paint = component.textPaint
  const pressable = onPress !== undefined || selected
  return <span className={`canvas-component canvas-component-echo${onPress ? ' canvas-component-echo-repeating' : ''} canvas-component-${component.type}${selected ? ' canvas-component-selected' : ''}`} aria-hidden="true" onPointerDown={onPress ?? (selected ? (event) => { if (event.button !== 0) return; event.stopPropagation(); if (event.shiftKey) onSelect?.(component.id, true); else onBodyPress?.(event) } : undefined)} onClick={pressable ? (event) => event.stopPropagation() : undefined} style={{ ...componentStyle({ x: component.x, y, width: component.width, height: component.height }, zoom), '--echo-clip-top': canvasDisplay.css(Math.max(0, -y), zoom) } as CSSProperties}><ComponentBox component={component} zoom={zoom} />{paint ? <TextPaint component={component} carriedFaces={carriedFaces} zoom={zoom} /> : component.type === 'image' ? <ImagePaint component={component} zoom={zoom} engine={engine} generation={generation} /> : component.type === 'table' ? <TablePaint component={component} zoom={zoom} /> : component.type === 'barcode' ? <BarcodePaint component={component} zoom={zoom} /> : component.type === 'qrcode' ? <QRCodePaint component={component} zoom={zoom} /> : ''}</span>
}
// Story 9.2: the box the engine paints — style.background and
// style.border — drawn on the canvas from the ENGINE's own projection, so
// what the author sees is what will print. Painted as a child beneath the
// content rather than on the component itself, which leaves the selection
// tint and the dotted placement outline the canvas already draws alone.
//
// The engine's own defaults are mirrored where a border declares less than
// all of itself: 0.5pt, #000000, all four edges (buildCellRectWithBackground-
// Field). A border declared with NO fields at all (`"border": {}`, reachable
// only by hand — the designer's own clear drops an empty border) projects
// nothing at all, so the canvas cannot see it; the PDF still draws it.
const boxEdges = ['top', 'right', 'bottom', 'left'] as const
function ComponentBox({ component, zoom }: { component: CanvasProjection['components'][number]; zoom: number }) {
  const bordered = component.borderWidth !== undefined || component.borderColor !== undefined || component.borderEdges !== undefined
  if (component.background === undefined && !bordered) return undefined
  const edges = component.borderEdges ?? boxEdges
  // The declared width, through the one zoom mapping every other painted
  // length goes through — not floored at a device pixel. A 0.5pt hairline
  // draws as the sub-pixel line it is, because inflating it would show the
  // author a border the PDF will not print.
  const stroke = `${canvasDisplay.css(component.borderWidth ?? 500, zoom)} solid ${component.borderColor ?? '#000000'}`
  const style: Record<string, string> = {}
  if (component.background !== undefined) style.background = component.background
  if (bordered) for (const edge of boxEdges) style[`border${edge[0]!.toUpperCase()}${edge.slice(1)}`] = edges.includes(edge) ? stroke : '0'
  return <span className="canvas-box" aria-hidden="true" style={style as CSSProperties} />
}
// STORY 14.3 — THE COMFORTABLE HIT SIZE, AND WHY THE ARITHMETIC IS HERE.
//
// 12px on the thin axis: the same reachable box `.selection-handle` already
// gives a 6x6 painted mark, and the OWNER'S RULING over the file's other
// precedent. `.resize-handle` is 24px, but that is a CORNER grab of which there
// is one per component; an edge-to-edge pad of ~11px per side on every thin
// component would make overlap the common case rather than the edge case.
//
// A component already at or over 12px on an axis is padded by ZERO on it, so a
// normal text box's hit region is exactly its box, as today. A thinner one gets
// HALF the shortfall on each side, so the pointer-reachable extent reaches 12px
// while the drawn box keeps every pixel it had and not one more.
//
// Measure the shortfall against the outer interaction box, which keeps a 2px
// floor. Line paint uses the exact projected dimensions inside that box.
// Padding the projection instead would add 5.5px per side to a 2px wrapper
// for a 1pt line, making its hit region 13px instead of the intended 12px.
//
// ⚠ IT IS KEYED TO ELEMENT SIZE, NEVER TO COMPONENT KIND. A 1pt-tall image or
// rectangle is as unreachable as a 1pt line and is padded on the same terms.
//
// ⚠ IT IS COMPUTED HERE, FROM THE PROJECTION, AND NOT IN CSS OR FROM THE DOM.
// CSS cannot branch on whether a custom property is under a threshold, and the
// pad must be zero above it. Asking the element how big it is would be
// `getBoundingClientRect`/`offsetHeight` — measurements
// canvas-authority-contract.test.ts bans outright, and which this story was
// told to open no carve-out for. `component.width`/`height` are Go's own
// millipoints and `zoom` is local state, so this is ordinary arithmetic on
// document values through exactly the rounding `canvasDisplay.css` uses.
//
// ⚠ AND THE PAD IS CHROME. No command carries it, no FieldSpec authors it, and
// no document byte moves because of it. It is a hit region in the EDITOR, which
// is a different thing from `style.padding` (D-12.4.1) that happens to share a
// word.
const COMFORTABLE_HIT_PX = 12
// The floor `.canvas-component` declares, read rather than re-decided. If that
// rule's 2px ever moves, this constant is the one line that has to move with it.
const HIT_BOX_FLOOR_PX = 2
function hitPad(millipoints: number, zoom: number): string {
  const boxSize = Math.max(HIT_BOX_FLOOR_PX, Math.round(millipoints * zoom * 1000) / 1_000_000)
  return `${Math.max(0, Math.round((COMFORTABLE_HIT_PX - boxSize) * 500_000) / 1_000_000)}px`
}
function componentStyle(component: { x: number; y: number; width: number; height: number }, zoom: number): CSSProperties { return { '--component-x': canvasDisplay.css(component.x, zoom), '--component-y': canvasDisplay.css(component.y, zoom), '--component-width': canvasDisplay.css(component.width, zoom), '--component-height': canvasDisplay.css(component.height, zoom), '--hit-pad-x': hitPad(component.width, zoom), '--hit-pad-y': hitPad(component.height, zoom) } as CSSProperties }

function equalBytes(left: ArrayBuffer, right: ArrayBuffer): boolean {
  const a = new Uint8Array(left)
  const b = new Uint8Array(right)
  return a.length === b.length && a.every((value, index) => value === b[index])
}

function previewFailure(error: EngineError): EngineError {
  // Values reached here were bounded and validated at the worker boundary.
  // Preserve their presence and content; do not turn absent provenance into a
  // browser-owned replacement code, message, or location.
  return { code: error.code, message: error.message, ...(error.elementId !== undefined ? { elementId: error.elementId } : {}), ...(error.dataPath !== undefined ? { dataPath: error.dataPath } : {}) }
}

function localPreviewIssue(error: unknown): string {
  return error instanceof Error && error.message ? error.message.slice(0, 512) : 'The local Preview work did not complete'
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}
