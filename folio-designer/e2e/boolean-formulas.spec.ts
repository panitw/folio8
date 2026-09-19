import { expect, test, type Page } from '@playwright/test'
import { openWorkspace } from './app.js'
import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'

const root = path.resolve(import.meta.dirname, '../..')
const goRoot = path.join(root, 'folio-go')
const formula = 'vip ? true : (blocked ? false : loanAmount + fee > 20000)'
type Capture = { renders: number[][]; errors: Array<{ code?: string; message?: string; elementId?: string; dataPath?: string }>; commands: string[] }

async function observeWorker(page: Page) {
  await page.addInitScript(() => {
    Object.assign(window, { showOpenFilePicker: undefined, showSaveFilePicker: undefined })
    const NativeWorker = window.Worker
    const operations = new Map<string, string>()
    const capture: Capture = { renders: [], errors: [], commands: [] }
    class ObservedWorker extends NativeWorker {
      constructor(...args: ConstructorParameters<typeof Worker>) {
        super(...args)
        this.addEventListener('message', (event: MessageEvent) => {
          const response = event.data as { kind?: string; requestId?: string; ok?: boolean; bytes?: ArrayBuffer; error?: Capture['errors'][number] }
          if (response.kind !== 'response') return
          if (!response.ok && response.error) capture.errors.push(response.error)
          if (response.ok && response.bytes && operations.get(response.requestId ?? '') === 'render') capture.renders.push(Array.from(new Uint8Array(response.bytes)))
        })
      }
      postMessage(message: unknown, transfer: Transferable[]): void
      postMessage(message: unknown, options?: StructuredSerializeOptions): void
      postMessage(message: unknown, transferOrOptions?: Transferable[] | StructuredSerializeOptions): void {
        const request = message as { kind?: string; operation?: string; requestId?: string; payload?: ArrayBuffer }
        if (request.kind === 'request' && request.operation && request.requestId) {
          operations.set(request.requestId, request.operation)
          if (request.operation === 'command' && request.payload instanceof ArrayBuffer) capture.commands.push(new TextDecoder().decode(request.payload))
        }
        if (Array.isArray(transferOrOptions)) super.postMessage(message, transferOrOptions)
        else super.postMessage(message, transferOrOptions)
      }
    }
    Object.assign(window, { Worker: ObservedWorker, __booleanFormulaCapture: capture })
  })
}
const capture = (page: Page): Promise<Capture> => page.evaluate(() => (window as typeof window & { __booleanFormulaCapture: Capture }).__booleanFormulaCapture)
async function revision(page: Page): Promise<string> { return await page.getByTestId('engine-snapshot').textContent() ?? '' }
async function openFile(page: Page, buffer: Buffer) {
  const chooser = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Open local template' }).click()
  await (await chooser).setFiles({ name: 'boolean-formulas.folio', mimeType: 'application/json', buffer })
  await expect(page.getByRole('button', { name: /text component e1/ })).toBeVisible()
}
async function save(page: Page): Promise<Buffer> {
  const downloaded = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Save As', exact: true }).click()
  const stream = await (await downloaded).createReadStream()
  if (!stream) throw new Error('missing saved template stream')
  const chunks: Buffer[] = []
  for await (const chunk of stream) chunks.push(Buffer.from(chunk))
  return Buffer.concat(chunks)
}

test('authored boolean formulas survive real worker history, persistence and native render parity', async ({ page }, testInfo) => {
  test.setTimeout(300_000)
  await observeWorker(page)
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/REVISION 1/)
  const starter = JSON.parse(readFileSync(path.join(root, 'folio-designer/public/templates/starter.folio'), 'utf8'))
  starter.nextId = 3
  starter.bands.content.elements = [
    { id: 'e1', type: 'text', x: 0, y: 0, width: 200, height: 24, value: 'Conditional line', style: { fontFamily: 'Roboto', fontSize: 12 } },
    { id: 'e2', type: 'text', x: 0, y: 50, width: 200, height: 24, value: 'Always visible', style: { fontFamily: 'Roboto', fontSize: 12 } },
  ]
  await openFile(page, Buffer.from(JSON.stringify(starter)))
  await page.getByRole('button', { name: /text component e1/ }).click()
  await page.getByRole('tab', { name: 'PROPERTIES', exact: true }).click()
  const literalPDFs: Buffer[] = []
  const visibleIf = page.getByRole('textbox', { name: 'Visible if', exact: true })
  await expect(visibleIf).toHaveAttribute('aria-description', /loanAmount > 20000/)
  for (const source of ['loanAmount > 20000', 'loanAmount + fee > 20000', 'true', 'false', 'null', formula]) {
    const before = await revision(page)
    await visibleIf.fill(source)
    await visibleIf.press('Enter')
    await expect.poll(() => revision(page)).not.toBe(before)
    await expect(visibleIf).toHaveValue(source)
    if (['true', 'false', 'null'].includes(source)) {
      const beforeRender = (await capture(page)).renders.length
      await page.getByRole('button', { name: 'PREVIEW', exact: true }).click()
      await expect(page.getByRole('img', { name: /Current no-data layout PDF/ })).toBeVisible({ timeout: 60_000 })
      await expect.poll(async () => (await capture(page)).renders.length, { timeout: 60_000 }).toBeGreaterThan(beforeRender)
      const notice = page.getByRole('note', { name: 'No-data preview notice' })
      await expect(notice).toContainText('literal conditions retain their authored meaning')
      await expect(notice).not.toContainText('fabricates at least one condition')
      literalPDFs.push(Buffer.from((await capture(page)).renders.at(-1)!))
      await page.getByRole('button', { name: 'DESIGN', exact: true }).click()
      await page.getByRole('button', { name: /text component e1/ }).click()
      await page.getByRole('tab', { name: 'PROPERTIES', exact: true }).click()
    }
  }
  expect(literalPDFs[0]).not.toEqual(literalPDFs[1])
  expect(literalPDFs[1]).toEqual(literalPDFs[2])
  const beforeUndo = await revision(page)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => revision(page)).not.toBe(beforeUndo)
  await page.getByRole('button', { name: /text component e1/ }).click()
  await expect(visibleIf).toHaveValue('null')
  const beforeRedo = await revision(page)
  await page.getByRole('button', { name: 'Redo', exact: true }).click()
  await expect.poll(() => revision(page)).not.toBe(beforeRedo)
  await page.getByRole('button', { name: /text component e1/ }).click()
  await expect(visibleIf).toHaveValue(formula)
  const beforeInvalid = await revision(page)
  await visibleIf.fill(' '.repeat(600) + 'loanAmount >')
  await visibleIf.press('Enter')
  await expect(page.getByRole('alert').filter({ hasText: /visibleIf/ })).toBeVisible()
  expect(await revision(page)).toBe(beforeInvalid)
  const failure = (await capture(page)).errors.at(-1)
  expect(failure?.code).toBe('EXPRESSION_INVALID')
  expect(failure?.elementId).toBe('e1')
  expect(failure?.dataPath).toBe('visibleIf')
  expect(failure?.message).toMatch(/position/)
  await visibleIf.press('Escape')
  const saved = await save(page)
  expect(JSON.parse(saved.toString('utf8')).bands.content.elements[0].visibleIf).toBe(formula)
  expect(JSON.parse(saved.toString('utf8')).version).toBe('2.0')
  await openFile(page, saved)
  await page.getByRole('button', { name: /text component e1/ }).click()
  await page.getByRole('tab', { name: 'PROPERTIES', exact: true }).click()
  await expect(visibleIf).toHaveValue(formula)
  expect(await save(page)).toEqual(saved)

  const output = testInfo.outputPath('boolean-formula-parity')
  mkdirSync(output, { recursive: true })
  execFileSync('go', ['build', '-o', path.join(output, 'folio8'), './cmd/folio8'], { cwd: goRoot, stdio: 'pipe' })
  const templatePath = path.join(output, 'input.folio')
  const paramsPath = path.join(output, 'params.json')
  writeFileSync(templatePath, saved)
  writeFileSync(paramsPath, '{}')
  const pdfs: Buffer[] = []
  for (const amount of [25000, 20000]) {
    await page.getByRole('tab', { name: 'DATA', exact: true }).click()
    const chooser = page.waitForEvent('filechooser')
    await page.getByRole('button', { name: /^(Load|Replace) sample JSON$/ }).click()
    const data = Buffer.from(JSON.stringify({ loanAmount: amount, fee: 0, vip: false, blocked: false }))
    await (await chooser).setFiles({ name: `amount-${amount}.json`, mimeType: 'application/json', buffer: data })
    const before = (await capture(page)).renders.length
    await page.getByRole('button', { name: 'PREVIEW', exact: true }).click()
    await expect(page.getByRole('img', { name: /Current exact local production PDF/ })).toBeVisible({ timeout: 60_000 })
    await expect.poll(async () => (await capture(page)).renders.length, { timeout: 60_000 }).toBeGreaterThan(before)
    const workerPDF = Buffer.from((await capture(page)).renders.at(-1)!)
    const dataPath = path.join(output, `amount-${amount}.json`)
    const nativePath = path.join(output, `amount-${amount}.pdf`)
    writeFileSync(dataPath, data)
    execFileSync(path.join(output, 'folio8'), ['render', '-data', dataPath, '-params', paramsPath, '-o', nativePath, templatePath], { cwd: goRoot, stdio: 'pipe' })
    expect(readFileSync(nativePath)).toEqual(workerPDF)
    pdfs.push(workerPDF)
    await page.getByRole('button', { name: 'DESIGN', exact: true }).click()
  }
  expect(pdfs[0]).not.toEqual(pdfs[1])
  expect((await capture(page)).commands.filter((command) => command.includes('visibleIf')).length).toBeGreaterThanOrEqual(7)
})
