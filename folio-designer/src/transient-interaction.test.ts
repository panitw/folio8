import { describe, expect, it } from 'vitest'
import { EngineClient, type WorkerPort } from './engine-client'
import { ENGINE_PROTOCOL_VERSION, type EngineRequest } from './engine-protocol'
import { TransientInteraction } from './transient-interaction'

class ControlledWorker implements WorkerPort {
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null
  onerror: ((event: ErrorEvent) => void) | null = null
  sent: EngineRequest[] = []
  postMessage(message: EngineRequest): void { this.sent.push(message) }
  terminate(): void {}
  ready(): void { this.onmessage?.({ data: { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'lifecycle', state: 'ready' } } as MessageEvent<unknown>) }
  succeed(): void {
    const request = this.sent[0]!
    this.onmessage?.({ data: { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: request.requestId, ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 12 } } } as MessageEvent<unknown>)
  }
}

describe('transient interaction boundary', () => {
  it('keeps drafts local until an explicit commit and remains responsive while pending', async () => {
    const worker = new ControlledWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const interaction = new TransientInteraction(client)
    interaction.update('draft value')
    expect(interaction.draft).toBe('draft value')
    expect(worker.sent).toHaveLength(0) // Red proof: draft does not leak into a document command.
    const pending = interaction.commit()
    expect(worker.sent).toHaveLength(1)
    expect(new TextDecoder().decode(worker.sent[0]!.payload as ArrayBuffer)).toBe('commit')
    interaction.update('later UI event')
    expect(interaction.draft).toBe('later UI event')
    worker.succeed()
    await expect(pending).resolves.toMatchObject({ snapshot: { revision: 1 } })
  })
})
