import type { EngineClient, EngineResult } from './engine-client'

// Future property editors can hold drafts here without receiving document
// access. Only commit enters the one engine command channel.
export class TransientInteraction {
	#draft = ''
	private readonly client: EngineClient
	constructor(client: EngineClient) { this.client = client }

  get draft(): string { return this.#draft }
  update(value: string): void { this.#draft = value }
  commit(): Promise<EngineResult> {
    // The only currently-defined command is owned by Go. Draft text remains
    // UI-local until later stories define an engine command vocabulary.
    return this.client.request('command', new TextEncoder().encode('commit').buffer)
  }
}
