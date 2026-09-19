import type { EngineClient } from './engine-client'
import type { EngineSnapshot } from './engine-protocol'

export async function loadStarterAfterEngineReady(clientPromise: Promise<EngineClient>, starterUrl: string, fetchStarter: (url: string) => Promise<Response>): Promise<Readonly<{ client: EngineClient; snapshot: EngineSnapshot; blankBytes: ArrayBuffer }>> {
  const client = await clientPromise
  const source = await fetchStarter(starterUrl)
  if (!source.ok) throw new Error('starter template unavailable')
  // Startup is the one explicit initialization of the fresh local session;
  // user-initiated Open and Start blank retain the distinct `load` operation.
  const loaded = await client.request('initialize', await source.arrayBuffer())
  const serialized = await client.request('serialize')
  if (!serialized.bytes || serialized.bytes.byteLength !== loaded.snapshot.byteLength) throw new Error('canonical serialization unavailable')
  return { client, snapshot: loaded.snapshot, blankBytes: serialized.bytes }
}
