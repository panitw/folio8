import { expect, test, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'

const root = path.resolve(import.meta.dirname, '../..')
const goRoot = path.join(root, 'folio-go')
// STORY 14.10 / Q4b — CAP-13 NOW AUTHORS FROM A LOADED DOCUMENT.
//
// This test used to TYPE its statement into the designer: place a text, set a
// font, fill Y, fill the value, place a table, fill a collection, add five
// columns, fill five row fields, fill five headers, set a footer aggregate. Its
// byte-identity claim — the most trusted number in the project — was therefore
// permanently coupled to first-run designer UI, and 14.10 removes one of the
// controls that path drove (the Binding input, [D-14.10.1]). Rewriting those
// keystrokes would have coupled it to the NEXT UI instead.
//
// The claim itself is unchanged and nothing is lost: what the browser SAVES is
// still asserted byte-identical to what its own engine serialized, to what it
// sent to render, and to what the native CLI renders. What moved out is the
// authoring leg — *"a document authored through the browser's own commands
// round-trips"* — which is now `e2e/table-column-binding.spec.ts`'s subject, so
// a red HERE means reproducibility broke and a red THERE means the UI moved.
// That is the only arm that fixes failure ATTRIBUTION, which is what a misread
// of exactly this signal cost in DW-383.
//
// ⚠ THE FIXTURE IS READ-ONLY AND ITS PRECONDITION IS ASSERTED, NOT ASSUMED.
// `fixtures/statement-1/input.folio` carries the logo asset, the account
// framing, the generated-date/page footer AND a five-column table with every
// column bound — which is exactly what `assertCustomerStatementFacts` in
// `folio-go/browser_roundtrip_witness_test.go` reads out of the Go-owned model.
// `openPreparedStatement` re-asserts the five bound columns through the shipped
// dialog on every run: a fixture that stopped matching the precondition is a
// guard that cannot see the defect ([D-14.8.4]).
const fixtures = path.join(root, 'fixtures/statement-1')
const template = readFileSync(path.join(fixtures, 'input.folio'))
const sample = readFileSync(path.join(fixtures, 'data.json'))
const params = readFileSync(path.join(fixtures, 'params.json'))

type CapturedRequest = Readonly<{ sessionId: string; operation: string; requestId: string; command?: number[]; render?: Readonly<{ template: number[]; data: number[]; params: number[] }> }>
type Captured = Readonly<{ requests: ReadonlyArray<CapturedRequest>; responses: ReadonlyArray<string>; serializations: ReadonlyArray<number[]>; renders: ReadonlyArray<number[]>; failures: ReadonlyArray<string> }>
type BrowserWitness = Readonly<{ template: Buffer; data: Buffer; params: Buffer; pdf: Buffer; repeatedPDF: Buffer; requests: ReadonlyArray<CapturedRequest>; serialized: number }>

// The observer wraps the existing Worker constructor before the app starts.
// It copies messages for evidence only; it neither constructs another worker
// nor changes an opaque request/response byte array. PDF.js admission is still
// proved by the visible "EXACT LOCAL PRODUCTION PDF" state below.
async function observeOneWorker(page: Page, sessionId: string): Promise<void> {
  await page.addInitScript((session) => {
    // The proof deliberately exercises the fallback file picker/download
    // route so Playwright can retain the exact downloaded bytes.
    Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined })
    const NativeWorker = window.Worker
    const requestOperations = new Map<string, string>()
    const capture = { requests: [] as Array<CapturedRequest>, responses: [] as string[], serializations: [] as number[][], renders: [] as number[][], failures: [] as string[] }
    class ObservedWorker extends NativeWorker {
      constructor(...args: ConstructorParameters<typeof Worker>) {
        super(...args)
        this.addEventListener('message', (event: MessageEvent<unknown>) => {
          const message = event.data as { kind?: string; ok?: boolean; requestId?: string; bytes?: ArrayBuffer; error?: { code?: string; message?: string } }
          const operation = message.requestId ? requestOperations.get(message.requestId) : undefined
          if (message.kind === 'response') capture.responses.push(`${operation ?? 'unknown'}:${message.ok ? 'ok' : 'error'}`)
          if (message.kind === 'response' && message.ok && message.bytes && operation === 'serialize') capture.serializations.push(Array.from(new Uint8Array(message.bytes)))
          if (message.kind === 'response' && message.ok && message.bytes && operation === 'render') capture.renders.push(Array.from(new Uint8Array(message.bytes)))
          if (message.kind === 'response' && !message.ok) capture.failures.push(`${operation ?? 'unknown'}:${message.error?.code ?? 'unknown'}:${message.error?.message ?? 'missing message'}`)
        })
        this.addEventListener('error', (event) => capture.failures.push(`worker-runtime:${event.message || 'missing message'}`))
      }
      postMessage(message: unknown, transfer: Transferable[]): void
      postMessage(message: unknown, options?: StructuredSerializeOptions): void
      postMessage(message: unknown, transferOrOptions?: Transferable[] | StructuredSerializeOptions): void {
        const request = message as { kind?: string; operation?: string; requestId?: string; payload?: ArrayBuffer | { template: ArrayBuffer; data: ArrayBuffer; params: ArrayBuffer } }
        if (request.kind === 'request' && request.operation && request.requestId) {
          requestOperations.set(request.requestId, request.operation)
          const bytes = (value: ArrayBuffer) => Array.from(new Uint8Array(value.slice(0)))
          const record: { sessionId: string; operation: string; requestId: string; command?: number[]; render?: { template: number[]; data: number[]; params: number[] } } = { sessionId: session, operation: request.operation, requestId: request.requestId }
          if (request.operation === 'command' && request.payload instanceof ArrayBuffer) record.command = bytes(request.payload)
          if (request.operation === 'render' && request.payload && !(request.payload instanceof ArrayBuffer)) record.render = { template: bytes(request.payload.template), data: bytes(request.payload.data), params: bytes(request.payload.params) }
          capture.requests.push(record)
        }
        if (Array.isArray(transferOrOptions)) super.postMessage(message, transferOrOptions)
        else super.postMessage(message, transferOrOptions)
      }
    }
    Object.assign(window, { Worker: ObservedWorker, __folio8RoundTripCapture: capture })
  }, sessionId)
}

async function captured(page: Page): Promise<Captured> {
  return page.evaluate(() => (window as typeof window & { __folio8RoundTripCapture: Captured }).__folio8RoundTripCapture)
}

// The inspector is one tabbed panel: PROPERTIES (INPUTS in Preview) and DATA.
// Each authoring helper opens the tab holding the control it drives.
async function openTab(page: Page, name: 'PROPERTIES' | 'DATA' | 'INPUTS'): Promise<void> {
  await page.getByRole('tab', { name }).click()
}

// THE ONE CHAIN A FRESH SESSION DECLARES. Startup initializes the engine from
// the shipped starter (`folio-designer/public/templates/starter.folio`, via
// `loadStarterAfterEngineReady`), so `CanvasProjection.fontFamilies` is exactly
// that file's font-map keys and the family control's IN THIS TEMPLATE group has
// exactly this one row. It was `body` until commit 4d2b27e ("Ship Roboto in the
// engine, and open new documents in a typeface with a name") renamed the key,
// which left this helper searching for a name no fresh document declares any
// more. Go pins the same name from the same file: `starterChainName` in
// `folio-go/starter_template_test.go`, guarded by that file's parse of the real
// bytes — so a future rename fails there loudly rather than only here by timeout.
const starterFontFamily = 'Roboto'

// The family is chosen from the engine's own declared chains — Go projects
// them (CanvasProjection.fontFamilies) and the inspector searches that list —
// so this picks the option rather than typing a value at the field.
async function setFontFamily(page: Page): Promise<void> {
  await openTab(page, 'PROPERTIES')
  const font = page.getByRole('combobox', { name: 'Font family' })
  await font.click()
  await font.fill(starterFontFamily)
  // Scoped to the declared group rather than the whole listbox: typing the
  // family name also matches AVAILABLE LOCALLY rows that merely start with it
  // (Roboto Condensed, Roboto Mono, …), and picking one of those would embed a
  // second family instead of naming the chain this document already declares.
  await page.getByRole('group', { name: 'IN THIS TEMPLATE' }).getByRole('option', { name: starterFontFamily, exact: true }).click()
  await expect(font).toHaveValue(starterFontFamily)
}

async function loadSample(page: Page): Promise<void> {
  await openTab(page, 'DATA')
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Load sample JSON' }).click()
  await (await chooser).setFiles({ name: 'authored-session-data.json', mimeType: 'application/json', buffer: sample })
  await expect(page.getByRole('tree', { name: 'Sample data paths' })).toBeVisible()
}

async function revision(page: Page): Promise<number> {
  const text = await page.getByTestId('engine-snapshot').textContent()
  const found = text?.match(/REVISION (\d+)/)
  if (!found) throw new Error(`could not read engine revision from ${text}`)
  return Number(found[1])
}

async function waitForRevisionAdvance(page: Page, before: number): Promise<void> {
  await expect.poll(() => revision(page), { timeout: 12_000 }).toBeGreaterThan(before)
}

async function bindTextToCustomer(page: Page, content: ReturnType<Page['getByRole']>): Promise<void> {
  const texts = content.getByRole('button', { name: /text component/ })
  const beforeCount = await texts.count()
  await page.getByRole('button', { name: 'Place Text' }).click()
  await content.press('Enter')
  await expect(texts).toHaveCount(beforeCount + 1, { timeout: 12_000 })
  await texts.last().click()
  await setFontFamily(page)
  await openTab(page, 'DATA')
  const tree = page.getByRole('tree', { name: 'Sample data paths' })
  const customer = tree.getByRole('treeitem').filter({ hasText: /^customer/ })
  await customer.click()
  const name = tree.getByRole('treeitem').filter({ hasText: /^name/ })
  await expect(name).toBeVisible()
  await name.click()
  // STORY 14.6 — THE PICK IS THE BIND. The leaf click above dispatches
  // `bindComponentScalar` directly; the intermediate control is gone.
  await openTab(page, 'PROPERTIES')
  await expect(page.getByText('Bound to').locator('..')).toContainText('customer.name')
}

async function placeStatementText(page: Page, band: ReturnType<Page['getByRole']>, value: string, y: number): Promise<void> {
  const texts = band.getByRole('button', { name: /text component/ })
  const beforeCount = await texts.count()
  await page.getByRole('button', { name: 'Place Text' }).click()
  await band.press('Enter')
  await expect(texts).toHaveCount(beforeCount + 1, { timeout: 12_000 })
  const text = texts.last()
  await text.click()
  // A palette text starts deliberately style-free. Use the visible authoring
  // control so each statement field is independently renderable.
  await setFontFamily(page)
  if (y !== 0) {
    const yField = page.getByRole('textbox', { name: 'Y (pt)' })
    const beforeY = await revision(page)
    await yField.fill(String(y))
    await yField.press('Enter')
    await waitForRevisionAdvance(page, beforeY)
  }
  const field = page.getByRole('textbox', { name: 'Text', exact: true })
  const beforeValue = await revision(page)
  await field.fill(value)
  // Story 7.4 made the CONTENT control a textarea: Enter inserts a paragraph
  // break there instead of committing, so this witness commits it the way the
  // field actually commits — by blurring, exactly as App.test.tsx does. Every
  // other field here is single-line and still commits on Enter.
  await field.blur()
  await waitForRevisionAdvance(page, beforeValue)
}

// STORY 14.10 / Q4b — THE PREPARED DOCUMENT, OPENED THROUGH THE SHIPPED FILE
// TIER RATHER THAN TYPED.
//
// This is the whole of what replaced ~90 lines of keystrokes. It uses the same
// `readFileSync` + `setFiles` idiom eleven other specs already use, and it
// re-proves the fixture's precondition on every run: five columns, EVERY ONE
// BOUND. That second half is not decoration — CAP-13 exists to cover the
// statement the escalation was about, and a fixture that quietly lost its table
// bindings would leave this test green while covering nothing ([D-14.8.4]).
async function openPreparedStatement(page: Page): Promise<void> {
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name: 'statement-1.folio', mimeType: 'application/json', buffer: template })
  await expect(page.locator('.document-name')).toHaveText('statement-1.folio', { timeout: 12_000 })
  // The Go-owned facts `assertCustomerStatementFacts` will read, seen first
  // through the app's own canvas.
  await expect(page.getByRole('button', { name: /image component e1/ })).toBeVisible({ timeout: 12_000 })
  await expect(page.getByRole('button', { name: /table component e8/ })).toBeVisible()
  // THE PRECONDITION, ASSERTED THROUGH THE SHIPPED DIALOG. `Binding for column
  // N` is the complete binding in its single editable input.
  await page.getByRole('button', { name: /table component e8/ }).click()
  await openTab(page, 'PROPERTIES')
  await page.getByRole('button', { name: 'Configure columns' }).click()
  const dialog = page.getByRole('dialog', { name: 'Table Editor' })
  await expect(dialog.getByRole('grid', { name: 'Table columns' })).toBeVisible({ timeout: 12_000 })
  await expect(dialog.getByRole('grid', { name: 'Table columns' })).toHaveAttribute('aria-rowcount', '6')
  for (const index of [1, 2, 3, 4, 5]) {
    await expect(dialog.getByLabel(`Binding for column ${index}`), `fixture column ${index} must still be bound for CAP-13 to cover what it claims`).not.toHaveValue('')
  }
  // Merely opening the editor keeps this native-PDF witness read-only.
  await expect(dialog.getByRole('combobox', { name: 'Binding for column 1' })).toBeVisible()
  await expect(dialog.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{txn.date}}')
  await dialog.getByRole('button', { name: 'Done' }).click()
  await expect(dialog).toHaveCount(0)
}

async function authorAlternateReport(page: Page): Promise<void> {
  const header = page.getByRole('region', { name: 'Page Header', exact: true })
  const content = page.getByRole('region', { name: 'Content', exact: true })
  const footer = page.getByRole('region', { name: 'Page Footer', exact: true })
  await placeStatementText(page, header, 'ACCOUNT NOTICE', 30)
  await placeStatementText(page, footer, 'Archive copy — {{params.generatedDate}}', 0)
  await page.getByRole('button', { name: 'Place Rectangle' }).click()
  await content.press('Enter')
  await expect(content.getByRole('button', { name: /rect component/ })).toHaveCount(1)
}

function assertNativePreflight(output: string, name: string, template: Buffer, data: Buffer, parameterBytes: Buffer): void {
  const templatePath = path.join(output, `${name}.folio`)
  const dataPath = path.join(output, `${name}.data.json`)
  const paramsPath = path.join(output, `${name}.params.json`)
  const pdfPath = path.join(output, `${name}.preflight.pdf`)
  writeFileSync(templatePath, template)
  writeFileSync(dataPath, data)
  writeFileSync(paramsPath, parameterBytes)
  try {
    execFileSync(path.join(output, 'folio8'), ['validate', '-data', dataPath, '-params', paramsPath, templatePath], { cwd: goRoot, stdio: 'pipe' })
    execFileSync(path.join(output, 'folio8'), ['render', '-data', dataPath, '-params', paramsPath, '-o', pdfPath, templatePath], { cwd: goRoot, stdio: 'pipe' })
  } catch (error) {
    const processError = error as { stdout?: Buffer; stderr?: Buffer; message: string }
    throw new Error(`native preflight rejected the exact browser-saved ${name} input: ${processError.stderr?.toString('utf8') || processError.stdout?.toString('utf8') || processError.message}`, { cause: error })
  }
}

async function savePreviewAndCapture(page: Page, fileName: string, output: string, name: string): Promise<BrowserWitness> {
  const download = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Save As' }).click()
  const saved = await download
  const savedBytes = Buffer.from(await saved.createReadStream().then(async (stream) => {
    if (!stream) throw new Error('browser returned no saved stream')
    const chunks: Buffer[] = []
    for await (const chunk of stream) chunks.push(Buffer.from(chunk))
    return Buffer.concat(chunks)
  }))
  expect(saved.suggestedFilename()).toBe(fileName)

  await page.getByRole('button', { name: 'PREVIEW' }).click()
  await openTab(page, 'INPUTS')
  const rawParameters = page.getByRole('textbox', { name: 'Raw parameter JSON' })
  await rawParameters.fill(params.toString('utf8'))
  // Fail with the public CLI's concrete diagnostic before asking the WASM
  // adapter to map it to its intentionally bounded response schema. The
  // input bytes here are exactly those downloaded from this browser session.
  assertNativePreflight(output, name, savedBytes, sample, params)
  await page.getByRole('button', { name: 'Re-render' }).click()
  try {
    // Identity deliberately hashes the complete shipped font set before the
    // one Go render and PDF.js admission. This is runtime work, not a locator
    // ambiguity (all selector-facing steps above retain short timeouts).
    await expect(page.getByRole('img', { name: /Current exact local production PDF, revision/ })).toBeVisible({ timeout: 60_000 })
  } catch (error) {
    const proof = await captured(page)
    throw new Error(`Preview was not admitted; worker evidence: ${JSON.stringify({ requests: proof.requests.map(({ operation }) => operation), responses: proof.responses, failures: proof.failures })}`, { cause: error })
  }
  const proof = await captured(page)
  const serialized = proof.serializations.at(-1)
  const pdf = proof.renders.at(-1)
  const render = proof.requests.filter((request) => request.operation === 'render').at(-1)?.render
  if (!serialized || !pdf || !render) throw new Error('the one worker did not produce canonical, render-request, and PDF bytes')
  expect(Buffer.from(serialized)).toEqual(savedBytes)
  expect(Buffer.from(render.template)).toEqual(savedBytes)
  expect(Buffer.from(render.data)).toEqual(sample)
  expect(Buffer.from(render.params)).toEqual(params)
  await page.getByRole('button', { name: 'Re-render' }).click()
  await expect.poll(async () => (await captured(page)).renders.length).toBeGreaterThan(proof.renders.length)
  const repeated = (await captured(page)).renders.at(-1)
  if (!repeated) throw new Error('repeated browser render did not return PDF bytes')
  expect(Buffer.from(repeated)).toEqual(Buffer.from(pdf))
  return { template: Buffer.from(render.template), data: Buffer.from(render.data), params: Buffer.from(render.params), pdf: Buffer.from(pdf), repeatedPDF: Buffer.from(repeated), requests: (await captured(page)).requests, serialized: proof.serializations.length }
}

function fingerprint(value: Buffer): Readonly<{ length: number; sha256: string }> {
  return { length: value.length, sha256: createHash('sha256').update(value).digest('hex') }
}

function runNativeCLI(output: string, name: string): Buffer {
  const binary = path.join(output, 'folio8')
  const template = path.join(output, `${name}.folio`)
  const data = path.join(output, `${name}.data.json`)
  const parameterFile = path.join(output, `${name}.params.json`)
  const pdf = path.join(output, `${name}.native-cli.pdf`)
  execFileSync(binary, ['validate', '-data', data, '-params', parameterFile, template], { cwd: goRoot, stdio: 'pipe' })
  execFileSync(binary, ['render', '-data', data, '-params', parameterFile, '-o', pdf, template], { cwd: goRoot, stdio: 'pipe' })
  return readFileSync(pdf)
}

test('fresh authored sessions close exactly through admitted Preview and native folio8', async ({ browser }, testInfo) => {
  test.setTimeout(300_000)
  const output = testInfo.outputPath('browser-native-roundtrip')
  mkdirSync(output, { recursive: true })
  execFileSync('go', ['build', '-o', path.join(output, 'folio8'), './cmd/folio8'], { cwd: goRoot, stdio: 'pipe' })

  const goldenContext = await browser.newContext()
  const goldenPage = await goldenContext.newPage()
  await observeOneWorker(goldenPage, 'golden-fresh-session')
  await openWorkspace(goldenPage)
  await expect(goldenPage.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const goldenStartup = await captured(goldenPage)
  expect(goldenStartup.requests.map(({ operation }) => operation)).toEqual(['initialize', 'serialize'])
  expect(await goldenPage.getByRole('button', { name: /component/ }).count()).toBe(0)
  // ⚠ THE TEMPLATE FIRST, THE SAMPLE SECOND, AND THE ORDER IS LOAD-BEARING.
  // `open()` calls `clearSampleData()` — a sample belongs to the document that
  // was on screen when it was chosen — so loading the fixture after the sample
  // silently drops it and Preview falls back to STAND-IN data, which is not an
  // exact production PDF and never reaches `EXACT LOCAL PRODUCTION PDF`.
  await openPreparedStatement(goldenPage)
  await loadSample(goldenPage)
  const golden = await savePreviewAndCapture(goldenPage, 'statement-1.folio', output, 'golden')
  const goldenSession = golden.requests
  // STORY 14.10 / Q4b — THE GOLDEN DOCUMENT ARRIVES WHOLE, so this session
  // issues NO document command at all. It was `>= 8` while the statement was
  // typed in; asserting ZERO is the stronger claim and is what makes the
  // byte-identity result attributable to reproducibility rather than to the
  // designer's controls.
  expect(goldenSession.filter(({ operation, command }) => operation === 'command' && command).map(({ command }) => new TextDecoder().decode(new Uint8Array(command!)))).toEqual([])
  // ONE session, ONE engine, and the document reached it by `load` — the leg
  // this rewrite added, asserted rather than implied.
  expect(goldenSession.filter(({ operation }) => operation === 'load' || operation === 'initialize').map(({ operation }) => operation)).toEqual(['initialize', 'load'])
  await goldenContext.close()

  const alternateContext = await browser.newContext()
  const alternatePage = await alternateContext.newPage()
  await observeOneWorker(alternatePage, 'alternate-fresh-session')
  await openWorkspace(alternatePage)
  await expect(alternatePage.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const alternateStartup = await captured(alternatePage)
  expect(alternateStartup.requests.map(({ operation }) => operation)).toEqual(['initialize', 'serialize'])
  await loadSample(alternatePage)
  const alternateContent = alternatePage.getByRole('region', { name: 'Content', exact: true })
  await bindTextToCustomer(alternatePage, alternateContent)
  await authorAlternateReport(alternatePage)
  const alternate = await savePreviewAndCapture(alternatePage, 'Untitled template.folio', output, 'alternate')
  const alternateSession = alternate.requests
  expect(alternateSession.filter(({ operation, command }) => operation === 'command' && command).length).toBeGreaterThanOrEqual(3)
  expect(alternateSession.filter(({ operation }) => operation === 'load' || operation === 'initialize').map(({ operation }) => operation)).toEqual(['initialize'])
	const historyKeys = (requests: ReadonlyArray<CapturedRequest>) => new Set(requests.map(({ sessionId, requestId }) => `${sessionId}:${requestId}`))
	const goldenHistory = historyKeys(goldenSession)
	for (const key of historyKeys(alternateSession)) expect(goldenHistory.has(key)).toBe(false)
  await alternateContext.close()

  for (const [name, witness] of Object.entries({ golden, alternate })) {
    writeFileSync(path.join(output, `${name}.folio`), witness.template)
    writeFileSync(path.join(output, `${name}.data.json`), witness.data)
    writeFileSync(path.join(output, `${name}.params.json`), witness.params)
    writeFileSync(path.join(output, `${name}.browser.pdf`), witness.pdf)
  }
  const goldenNative = runNativeCLI(output, 'golden')
  const alternateNative = runNativeCLI(output, 'alternate')
  expect(goldenNative).toEqual(golden.pdf)
  expect(alternateNative).toEqual(alternate.pdf)
  execFileSync('go', ['test', '.', '-run', '^TestBrowserAuthoredRoundTripWitness$', '-count=1'], { cwd: goRoot, env: { ...process.env, FOLIO8_ROUNDTRIP_DIR: output }, stdio: 'pipe' })

  const evidence = {
    retention: 'Committed manifest contains only hashes, small raw render inputs, and command provenance. Playwright output/PDFs/native binary are disposable and ignored.',
    runtime: { platform: process.platform, arch: process.arch, node: process.version, chromium: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ? 'explicit Chromium executable' : 'Playwright default' },
    rawRenderInputs: { dataUTF8: golden.data.toString('utf8'), paramsUTF8: golden.params.toString('utf8') },
    golden: { template: fingerprint(golden.template), data: fingerprint(golden.data), params: fingerprint(golden.params), browserPDF: fingerprint(golden.pdf), repeatedBrowserPDF: fingerprint(golden.repeatedPDF), nativePDF: fingerprint(goldenNative), serializations: golden.serialized, commands: golden.requests.filter(({ operation, command }) => operation === 'command' && command).map(({ requestId, command }) => ({ requestId, utf8: new TextDecoder().decode(new Uint8Array(command!)) })), operations: golden.requests.map(({ operation, requestId }) => ({ operation, requestId })) },
    alternate: { template: fingerprint(alternate.template), data: fingerprint(alternate.data), params: fingerprint(alternate.params), browserPDF: fingerprint(alternate.pdf), repeatedBrowserPDF: fingerprint(alternate.repeatedPDF), nativePDF: fingerprint(alternateNative), serializations: alternate.serialized, commands: alternate.requests.filter(({ operation, command }) => operation === 'command' && command).map(({ requestId, command }) => ({ requestId, utf8: new TextDecoder().decode(new Uint8Array(command!)) })), operations: alternate.requests.map(({ operation, requestId }) => ({ operation, requestId })) },
    handEditedNativeTemplate: 'folio-go/testdata/template/golden/worked-example.json (native test only; never uploaded by this browser test)',
  }
  writeFileSync(path.join(output, 'evidence.json'), `${JSON.stringify(evidence, null, 2)}\n`)
	// This is deliberately the only durable artifact. It contains no PDF,
	// browser-result path, or platform executable; future runs replace it with
	// the exact current provenance rather than retaining unbounded test output.
	const durableEvidence = path.join(root, '_bmad-output/implementation-artifacts/evidence/story-6.7-roundtrip-manifest.json')
	mkdirSync(path.dirname(durableEvidence), { recursive: true })
	writeFileSync(durableEvidence, `${JSON.stringify(evidence, null, 2)}\n`)
  await testInfo.attach('browser-native-roundtrip-evidence', { path: path.join(output, 'evidence.json'), contentType: 'application/json' })
})
