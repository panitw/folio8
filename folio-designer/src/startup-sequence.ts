import { AbsentFaceInstaller } from './absent-face-recovery'
import type { EngineClient } from './engine-client'
import type { EngineSnapshot } from './engine-protocol'

// The deadline one deferred face fetch is given. It is DELIBERATELY NOT the
// 20 s an engine step gets (`ENGINE_FILE_STEP_TIMEOUT_MS` in App.tsx): that
// budget bounds a computation this machine is already performing, while this
// one bounds a ~10 MiB transfer over a connection nobody here can see — 20 s
// aborts it below roughly 4 Mbps, on a fetch whose failure is a document that
// will not open. 120 s carries the largest deferred face over a slow mobile
// link and still bounds a request that never settles.
const FACE_FETCH_TIMEOUT_MS = 120_000

export async function loadStarterAfterEngineReady(clientPromise: Promise<EngineClient>, starterUrl: string, fetchStarter: (url: string) => Promise<Response>): Promise<Readonly<{ client: EngineClient; snapshot: EngineSnapshot; blankBytes: ArrayBuffer }>> {
  const client = await clientPromise
  // THE ABSENT-FACE RECOVERY IS INSTALLED BEFORE ANY DOCUMENT REACHES THE
  // ENGINE (spec-deferred-offline-cache, CAP-6). This is the one seam between
  // the ready handshake and the first `initialize`, so a recovery installed
  // here is in place for every request the session will ever make — including
  // the starter's own, which needs it for nothing and is the point: the starter
  // is Latin, so nothing is fetched and nothing is installed.
  //
  // ⚠ IT IS NOT A PREFETCH AND STARTS NO WORK. Constructing the installer
  // registers a callback; the callback runs only when the engine refuses for a
  // face it does not hold. A cold-cache Latin session therefore never touches
  // the deferred tier, which is CAP-1 and CAP-2 stated as control flow.
  client.onAbsentFace(new AbsentFaceInstaller(client, FACE_FETCH_TIMEOUT_MS).recover)
  const source = await fetchStarter(starterUrl)
  if (!source.ok) throw new Error('starter template unavailable')
  // Startup is the one explicit initialization of the fresh local session;
  // user-initiated Open and Start blank retain the distinct `load` operation.
  const loaded = await client.request('initialize', await source.arrayBuffer())
  const serialized = await client.request('serialize')
  if (!serialized.bytes || serialized.bytes.byteLength !== loaded.snapshot.byteLength) throw new Error('canonical serialization unavailable')
  return { client, snapshot: loaded.snapshot, blankBytes: serialized.bytes }
}
