import { ENGINE_PROTOCOL_VERSION, parseRequest, requestCorrelationId, type EngineError, type EngineRequest } from './engine-protocol'

export type RequestAdmission = Readonly<
  | { kind: 'enqueue'; request: EngineRequest }
  | { kind: 'failure'; requestId?: string; error: EngineError; fatal: boolean }
>

// Tracks message identities before they can enter the worker queue. A duplicate
// receives a correlated failure but is never returned as an executable item.
export class EngineRequestAdmission {
  #seen = new Set<string>()

  admit(candidate: unknown): RequestAdmission {
    const requestId = requestCorrelationId(candidate)
    const request = parseRequest(candidate)
    if (!request) {
      if (!requestId) return { kind: 'failure', error: { code: 'PROTOCOL_INVALID', message: 'The engine received an uncorrelated invalid request' }, fatal: true }
      if (this.#seen.has(requestId)) return { kind: 'failure', requestId, error: { code: 'PROTOCOL_DUPLICATE_ID', message: 'The request id has already reached the engine' }, fatal: false }
      this.#seen.add(requestId)
      const code = typeof candidate === 'object' && candidate !== null && (candidate as { protocolVersion?: unknown }).protocolVersion !== ENGINE_PROTOCOL_VERSION ? 'PROTOCOL_VERSION_MISMATCH' : 'PROTOCOL_INVALID'
      return { kind: 'failure', requestId, error: { code, message: 'The engine request is invalid' }, fatal: false }
    }
    if (this.#seen.has(request.requestId)) return { kind: 'failure', requestId: request.requestId, error: { code: 'PROTOCOL_DUPLICATE_ID', message: 'The request id has already reached the engine' }, fatal: false }
    this.#seen.add(request.requestId)
    return { kind: 'enqueue', request }
  }
}
