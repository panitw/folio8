import { describe, expect, it, vi } from 'vitest'
import type { EngineClient } from './engine-client'
import { loadStarterAfterEngineReady } from './startup-sequence'

describe('production startup sequence', () => {
  it('cannot fetch or issue starter document requests until the singleton engine promise is ready', async () => {
    let resolve!: (client: EngineClient) => void
    const ready = new Promise<EngineClient>((done) => { resolve = done })
    const fetchStarter = vi.fn(async () => new Response(new Uint8Array([1, 2, 3]), { status: 200 }))
    const request = vi.fn(async (operation: string) => operation === 'initialize' ? { snapshot: { documentState: 'loaded', revision: 1, byteLength: 3 } } : { snapshot: { documentState: 'loaded', revision: 1, byteLength: 3 }, bytes: new Uint8Array([1, 2, 3]).buffer })
    const starting = loadStarterAfterEngineReady(ready, '/starter.folio', fetchStarter)
    expect(fetchStarter).not.toHaveBeenCalled()
    resolve({ request } as unknown as EngineClient)
    await starting
    expect(fetchStarter).toHaveBeenCalledWith('/starter.folio')
    expect(request).toHaveBeenCalledWith('initialize', expect.any(ArrayBuffer))
    expect(request).toHaveBeenLastCalledWith('serialize')
  })
})
