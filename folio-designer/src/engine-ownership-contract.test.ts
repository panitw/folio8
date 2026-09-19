import fs from 'node:fs'
import path from 'node:path'
import ts from 'typescript'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const sourceDir = path.dirname(fileURLToPath(import.meta.url))
const productionFiles = fs.readdirSync(sourceDir, { recursive: true })
  .filter((entry): entry is string => typeof entry === 'string' && /\.(?:ts|tsx)$/.test(entry) && !/\.test\.(?:ts|tsx)$/.test(entry))
  .map((entry) => path.join(sourceDir, entry))
const documentFields = new Set(['version', 'page', 'bands', 'elements', 'assets'])

type OwnershipScan = Readonly<{ workers: string[]; wasmInstances: string[]; schemaMirrors: string[]; documentJson: string[] }>

function scanOwnership(files: ReadonlyArray<Readonly<{ name: string; source: string }>>): OwnershipScan {
  const workers: string[] = []
  const wasmInstances: string[] = []
  const schemaMirrors: string[] = []
  const documentJson: string[] = []
  for (const file of files) {
    const source = ts.createSourceFile(file.name, file.source, ts.ScriptTarget.Latest, true)
    const visit = (node: ts.Node): void => {
      if (ts.isNewExpression(node) && ts.isIdentifier(node.expression) && (node.expression.text === 'Worker' || node.expression.text === 'SharedWorker')) workers.push(file.name)
      if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && ts.isIdentifier(node.expression.expression) && node.expression.expression.text === 'WebAssembly' && (node.expression.name.text === 'instantiate' || node.expression.name.text === 'instantiateStreaming')) wasmInstances.push(file.name)
      const names = propertyNames(node)
      if (names.filter((name) => documentFields.has(name)).length >= 2) schemaMirrors.push(file.name)
      if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && ts.isIdentifier(node.expression.expression) && node.expression.expression.text === 'JSON' && (node.expression.name.text === 'parse' || node.expression.name.text === 'stringify')) documentJson.push(file.name)
      ts.forEachChild(node, visit)
    }
    visit(source)
  }
  return { workers, wasmInstances, schemaMirrors, documentJson }
}

function propertyNames(node: ts.Node): string[] {
  const members = ts.isInterfaceDeclaration(node) || ts.isTypeLiteralNode(node) || ts.isClassDeclaration(node) ? node.members : ts.isObjectLiteralExpression(node) ? node.properties : []
  return members.flatMap((member) => {
    if ('name' in member && member.name && ts.isIdentifier(member.name)) return [member.name.text]
    if ('name' in member && member.name && ts.isStringLiteral(member.name)) return [member.name.text]
    return []
  })
}

const sources = () => productionFiles.map((file) => ({ name: path.relative(sourceDir, file), source: fs.readFileSync(file, 'utf8') }))

describe('engine ownership structure', () => {
  // Vite parses worker options statically, so the dev server's module worker and
  // the emitted release's classic worker cannot share one call site. Both sites
  // stay inside the one owning module and the expectation stays exact: a Worker
  // constructed anywhere else, or a third one here, still fails this test.
  it('has a non-vacuous source witness and exactly one Worker-owning module across production source', () => {
    expect(productionFiles.length).toBeGreaterThan(5)
    expect(scanOwnership(sources()).workers).toEqual(['engine-client.ts', 'engine-client.ts'])
    expect(fs.readFileSync(path.join(sourceDir, 'engine-client.ts'), 'utf8')).toContain('import.meta.env.DEV')
  })

  it('keeps exactly one wasm instantiation, in the dedicated worker entry, across every production module', () => {
    const scan = scanOwnership(sources())
    expect(scan.wasmInstances).toEqual(['engine.worker.ts'])
    expect(fs.readFileSync(path.join(sourceDir, 'engine.worker.ts'), 'utf8')).toContain('importScripts(runtimeAssetUrls.wasmExec)')
  })

  it('does not mirror the .folio document schema in production TypeScript', () => {
    const scan = scanOwnership(sources())
    expect(scan.schemaMirrors).toEqual([])
    expect(scan.documentJson.filter((name) => !['engine.worker.ts', 'offline-lifecycle.ts', 'release-payload.ts', 'sample-data.ts', 'App.tsx', 'command-json.ts'].includes(name))).toEqual([]) // protocol/release envelopes, bounded local sample discovery, and the transient parameter-document editor may parse JSON. STORY 15.2a REPLACED THE THREE COMMAND FACTORIES ON THIS LIST WITH ONE ENTRY, and the count was the point: the comment used to say "the three command factories may JSON-quote SCALAR INTENT", while SIX modules built command JSON and three of them did it with hand-rolled escaping or none. command-json.ts is now the only module that quotes anything for the wire — it encodes SCALAR INTENT and the envelope around it, never a template and never a document — and every builder lists its own fields, in order, so the field arity Go counts is still visible at the call site. The soleness of that arrangement is asserted by command-json-soleness.test.ts, not by this filter, which is one-directional and would not notice a stale entry.
    expect(fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')).toContain('Parameter input must be a JSON object')
  })

  it('keeps main-thread engine messaging in the one client module', () => {
    const offenders = productionFiles.filter((file) => {
      const name = path.basename(file)
      return name !== 'engine-client.ts' && name !== 'engine.worker.ts' && name !== 'offline-lifecycle.ts' && /\.postMessage\s*\(/.test(fs.readFileSync(file, 'utf8'))
    })
    expect(offenders).toEqual([])
  })

  it('rejects the two realistic second-authority review mutations', () => {
    const schemaMutation = scanOwnership([{ name: 'designer-state.ts', source: 'interface DesignerFileState { version: string; page: {}; bands: []; elements: []; assets: [] }' }])
    expect(schemaMutation.schemaMirrors).toEqual(['designer-state.ts'])
    const wasmMutation = scanOwnership([{ name: 'secondary-wasm.ts', source: 'WebAssembly.instantiate(new ArrayBuffer(0), {})' }])
    expect(wasmMutation.wasmInstances).toEqual(['secondary-wasm.ts'])
    const jsonMutation = scanOwnership([{ name: 'save-template.ts', source: 'const template = JSON.parse(bytes); JSON.stringify(template)' }])
    expect(jsonMutation.documentJson).toEqual(['save-template.ts', 'save-template.ts'])
    const narrowSchemaMutation = scanOwnership([{ name: 'designer-state.ts', source: 'type DesignerFileState = { page: {}; elements: [] }' }])
    expect(narrowSchemaMutation.schemaMirrors).toEqual(['designer-state.ts'])
  })
})
