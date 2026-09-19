import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { FolioRenderError, parameterReferences, parseTemplate, render, validate, version, type Diagnostic } from '../src/index.js'
import { repoFile, repoRoot, sha256, shippedFonts } from './helpers.js'

// Replays folio-js/test/data/go-parity.json, which
// folio-go/wasm/cmd/render/parity_test.go records from the Go engine and
// holds equal to it. Every expectation here is Go's.
interface Input {
  file?: string
  text?: string
}
interface ParityCase {
  name: string
  op: 'parse' | 'render' | 'validate' | 'parameterReferences'
  template: Input
  data?: Input
  params: Input | null
  expect: {
    sha256?: string
    diagnostics?: Diagnostic[]
    references?: string[]
    error?: { diagnostic?: Diagnostic; message?: string }
  }
}

const parity = JSON.parse(readFileSync(join(repoRoot, 'folio-js', 'test', 'data', 'go-parity.json'), 'utf8')) as {
  folio8Version: string
  shippedFaces: { name: string; byteLength: number }[]
  cases: ParityCase[]
}

const bytesOf = (input: Input): Uint8Array => (input.file !== undefined ? repoFile(input.file) : new TextEncoder().encode(input.text ?? ''))

async function run(c: ParityCase): Promise<{ sha256?: string; diagnostics?: Diagnostic[]; references?: string[] }> {
  const data = c.data ? bytesOf(c.data) : undefined
  const params = c.params ? bytesOf(c.params) : undefined
  switch (c.op) {
    case 'parse':
      await parseTemplate(bytesOf(c.template))
      return {}
    case 'render': {
      const result = await render(await parseTemplate(bytesOf(c.template)), data!, params, shippedFonts())
      return { sha256: sha256(result.bytes), diagnostics: result.diagnostics }
    }
    case 'validate':
      return { diagnostics: await validate(bytesOf(c.template), data!, params, shippedFonts()) }
    case 'parameterReferences':
      return { references: await parameterReferences(await parseTemplate(bytesOf(c.template))) }
  }
}

describe('diagnostic parity with Go', () => {
  it('covers the cases the I/O matrix names', () => {
    const names = parity.cases.map((c) => c.name)
    for (const name of ['render with a clip warning', 'render of an absent data path', 'render of data that is not JSON', 'render with params', 'parse of a malformed template', 'validate of a clean template', 'validate with a clip warning', 'validate of an absent data path', 'validate of a malformed template', 'parameter references']) {
      expect(names).toContain(name)
    }
  })

  it('was recorded from the engine version this package reports', () => {
    expect(parity.folio8Version).toBe(version)
  })

  it('the test font map is exactly fonts.Shipped()', () => {
    const helper = [...shippedFonts()].map(([name, bytes]) => ({ name, byteLength: bytes.byteLength })).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
    expect(helper).toEqual(parity.shippedFaces)
  })

  for (const c of parity.cases) {
    it(c.name, async () => {
      const { error, ...success } = c.expect
      if (!error) {
        expect(await run(c)).toEqual(success)
        return
      }
      const thrown = await run(c).then(
        (value) => ({ resolved: value }),
        (reason: unknown) => reason,
      )
      expect(thrown).toBeInstanceOf(Error)
      if (error.diagnostic) {
        expect(thrown).toBeInstanceOf(FolioRenderError)
        const renderError = thrown as FolioRenderError
        expect(renderError.diagnostic).toEqual(error.diagnostic)
        expect(renderError.message).toBe(error.diagnostic.message)
        expect(renderError.name).toBe('FolioRenderError')
      } else {
        expect(thrown).not.toBeInstanceOf(FolioRenderError)
        expect((thrown as Error).message).toBe(error.message)
      }
    })
  }

  it('a render result never carries an error diagnostic', async () => {
    const warning = parity.cases.find((c) => c.name === 'render with a clip warning')!
    const result = await run(warning)
    expect(result.diagnostics!.length).toBeGreaterThan(0)
    expect(result.diagnostics!.every((d) => d.severity === 'warning')).toBe(true)
  })
})
