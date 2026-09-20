import { describe, expect, it, vi } from 'vitest'
import type { EngineClient } from './engine-client'
import { loadStarterAfterEngineReady } from './startup-sequence'

describe('production startup sequence', () => {
  it('cannot fetch or issue starter document requests until the singleton engine promise is ready', async () => {
    let resolve!: (client: EngineClient) => void
    const ready = new Promise<EngineClient>((done) => { resolve = done })
    const fetchStarter = vi.fn(async () => new Response(new Uint8Array([1, 2, 3]), { status: 200 }))
    const request = vi.fn(async (operation: string) => operation === 'initialize' ? { snapshot: { documentState: 'loaded', revision: 1, byteLength: 3 } } : { snapshot: { documentState: 'loaded', revision: 1, byteLength: 3 }, bytes: new Uint8Array([1, 2, 3]).buffer })
    const onAbsentFace = vi.fn()
    const starting = loadStarterAfterEngineReady(ready, '/starter.folio', fetchStarter)
    expect(fetchStarter).not.toHaveBeenCalled()
    resolve({ request, onAbsentFace } as unknown as EngineClient)
    await starting
    expect(fetchStarter).toHaveBeenCalledWith('/starter.folio')
    expect(request).toHaveBeenCalledWith('initialize', expect.any(ArrayBuffer))
    expect(request).toHaveBeenLastCalledWith('serialize')
    // THE ABSENT-FACE RECOVERY IS IN PLACE BEFORE THE STARTER IS FETCHED
    // (spec-deferred-offline-cache, story 5). This seam is the only point
    // between the ready handshake and the first document, so a recovery
    // installed after the starter would leave the starter's own `initialize`
    // unable to recover — and, more to the point, would be a thing somebody
    // could quietly move later without any test noticing.
    expect(onAbsentFace, 'the engine must be given its face recovery exactly once, before any document reaches it').toHaveBeenCalledTimes(1)
    expect(onAbsentFace.mock.invocationCallOrder[0]).toBeLessThan(fetchStarter.mock.invocationCallOrder[0])
    // ⚠ AND INSTALLING IT FETCHES NOTHING. Registering a callback is not a
    // prefetch; a Latin session must reach an editable document having
    // transferred nothing outside the core tier (CAP-1, CAP-2).
    expect(request).not.toHaveBeenCalledWith('install-face', expect.anything())
  })
})
