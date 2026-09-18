import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it, vi } from 'vitest'
import { shipped } from '../src/fonts.js'
import { repoRoot } from './helpers.js'

// folio8-go/wasm/cmd/render/parity_test.go records shippedFaces from the
// engine's own fonts.Shipped(). It is the drift check for the packaged copy.
const shippedFaces = (JSON.parse(readFileSync(join(repoRoot, 'folio-js', 'test', 'data', 'go-parity.json'), 'utf8')) as { shippedFaces: { name: string; byteLength: number }[] }).shippedFaces

const digest = (bytes: Uint8Array) => createHash('sha256').update(bytes).digest('hex')

const byName = (a: { name: string }, b: { name: string }) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0)

describe('shipped()', () => {
  it('is the whole fonts.Shipped() set, name for name and byte for byte', async () => {
    const fonts = await shipped()
    expect(fonts.size).toBe(11)
    expect([...fonts].map(([name, bytes]) => ({ name, byteLength: bytes.byteLength })).sort(byName)).toEqual([...shippedFaces].sort(byName))
  })

  it('carries the bytes folio8-go/fonts/ holds', async () => {
    const fonts = await shipped()
    const manifest = JSON.parse(readFileSync(join(repoRoot, 'folio-js', 'fonts', 'manifest.json'), 'utf8')) as { faces: { name: string; file: string }[] }
    // Compared by digest: a 10 MB element-by-element deep-equal costs seconds
    // per face and says nothing more.
    for (const face of manifest.faces) {
      expect(digest(fonts.get(face.name)!), face.name).toBe(digest(readFileSync(join(repoRoot, 'folio8-go', 'fonts', face.file))))
    }
  })

  it('caches the bytes but hands each caller its own Map', async () => {
    const first = await shipped()
    const second = await shipped()
    // Compared as a boolean, not `expect(second).not.toBe(first)`: vitest
    // serialises both sides of a Map matcher, and these hold 14 MB each.
    expect(second === first, 'shipped() handed out the same Map twice').toBe(false)
    expect([...second.keys()].sort()).toEqual([...first.keys()].sort())
    // Cached, not re-read: the face bytes are the very same buffers.
    for (const [name, bytes] of first) expect(second.get(name) === bytes, name).toBe(true)

    // A caller mangling its own Map cannot reach the next caller's.
    first.delete('Noto Sans SC')
    first.set('Roboto', new Uint8Array())
    const third = await shipped()
    expect(third.size).toBe(11)
    expect(third.get('Roboto')!.byteLength).toBeGreaterThan(0)
  })

  it('does not cache a failed read: a later call still resolves', async () => {
    vi.resetModules()
    const actual = await vi.importActual<typeof import('node:fs/promises')>('node:fs/promises')
    let failNext = true
    vi.doMock('node:fs/promises', () => ({
      ...actual,
      readFile: (...args: Parameters<typeof actual.readFile>) => {
        if (!failNext) return actual.readFile(...args)
        failNext = false
        return Promise.reject(Object.assign(new Error('ENOENT: no such file or directory'), { code: 'ENOENT' }))
      },
    }))

    // A fresh module, so its cache is empty and the first read is the one
    // that fails.
    const { shipped: freshShipped } = await import('../src/fonts.js')
    await expect(freshShipped()).rejects.toThrow('npm run build:fonts')

    // The rejection was not cached, so the process is not poisoned.
    const recovered = await freshShipped()
    expect(recovered.size).toBe(11)
    expect([...recovered.keys()].sort()).toEqual([...shippedFaces].map((face) => face.name).sort())

    vi.doUnmock('node:fs/promises')
    vi.resetModules()
  })

  it('hands back a set render accepts as a FontSet', async () => {
    const fonts = await shipped()
    for (const [name, bytes] of fonts) {
      expect(typeof name).toBe('string')
      expect(bytes).toBeInstanceOf(Uint8Array)
      expect(bytes.byteLength).toBeGreaterThan(0)
    }
  })
})
