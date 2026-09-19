import { describe, expect, it } from 'vitest'
import { EngineRequestAdmission } from './engine-worker-admission'
import { ENGINE_PROTOCOL_VERSION } from './engine-protocol'

describe('engine worker request admission', () => {
  it('returns correlated typed failures for malformed and version-mismatched requests', () => {
    const admission = new EngineRequestAdmission()
    expect(admission.admit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'bad-operation', operation: 'erase-everything' })).toMatchObject({ kind: 'failure', requestId: 'bad-operation', error: { code: 'PROTOCOL_INVALID' }, fatal: false })
    expect(admission.admit({ protocolVersion: 999, kind: 'request', requestId: 'wrong-version', operation: 'snapshot' })).toMatchObject({ kind: 'failure', requestId: 'wrong-version', error: { code: 'PROTOCOL_VERSION_MISMATCH' }, fatal: false })
    expect(admission.admit({ kind: 'request' })).toMatchObject({ kind: 'failure', error: { code: 'PROTOCOL_INVALID' }, fatal: true })
  })

  it('admits a request exactly once and refuses its duplicate before queueing', () => {
    const admission = new EngineRequestAdmission()
    const request = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request' as const, requestId: 'one-command', operation: 'command' as const, payload: new TextEncoder().encode('commit').buffer }
    expect(admission.admit(request)).toMatchObject({ kind: 'enqueue', request: { requestId: 'one-command' } })
    expect(admission.admit(request)).toMatchObject({ kind: 'failure', requestId: 'one-command', error: { code: 'PROTOCOL_DUPLICATE_ID' }, fatal: false })
  })
})
