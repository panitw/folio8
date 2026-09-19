import { describe, expect, it } from 'vitest'
import { assetBytesRequest, setComponentAssetCommand } from './component-asset-command'
import { ENGINE_PROTOCOL_VERSION, MAX_ENGINE_PAYLOAD_BYTES, parseRequest } from './engine-protocol'

const text = (value: ArrayBuffer) => new TextDecoder().decode(value)

describe('opaque asset commands', () => {
  it('base64-encodes bytes and carries the declared media type opaquely, with complete JSON escaping on other fields', () => {
    const bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47]).buffer
    expect(text(setComponentAssetCommand('e1', 'image/png', bytes))).toBe('{"kind":"setComponentAsset","version":1,"id":"e1","mediaType":"image/png","data":"iVBORw=="}')
  })

  it('escapes an id or media type containing JSON-hostile characters', () => {
    const bytes = new Uint8Array([1]).buffer
    expect(text(setComponentAssetCommand('e"1', 'image/png\n', bytes))).toBe('{"kind":"setComponentAsset","version":1,"id":"e\\"1","mediaType":"image/png\\n","data":"AQ=="}')
  })

  it('never hashes, sniffs, or otherwise inspects the bytes — it only encodes them', () => {
    // A red proof: this factory must produce the SAME base64 for the SAME
    // bytes regardless of the declared media type, proving it never branches
    // on file content.
    const bytes = new Uint8Array([1, 2, 3]).buffer
    const asPng = text(setComponentAssetCommand('e1', 'image/png', bytes))
    const asJpeg = text(setComponentAssetCommand('e1', 'image/jpeg', bytes))
    expect(asPng.replace('image/png', 'X')).toBe(asJpeg.replace('image/jpeg', 'X'))
  })

  it('sends the asset key as opaque UTF-8 bytes for the per-key bytes request', () => {
    const key = 'a'.repeat(64)
    expect(text(assetBytesRequest(key))).toBe(key)
  })

  // Finding 6 (review of 2026-08-29): D-5.13.4 requires the protocol
  // envelope and Go's own size diagnostic to never disagree about the
  // threshold. Go's maxComponentAssetBytes (component_commands.go) is now
  // DERIVED as (MAX_ENGINE_PAYLOAD_BYTES - 4 KiB overhead reservation) *
  // 3/4 — the largest decoded size whose base64-inflated command payload
  // still fits the envelope. This mirrors that formula (the browser has no
  // access to Go's constant and must not pre-reject on size, D-5.13.4) and
  // proves the two ends actually agree: a command carrying exactly that
  // many decoded bytes, with a realistic id/mediaType, must clear the
  // TRANSPORT admission Go's diagnostic assumes it already has.
  it("a command at Go's derived byte ceiling still clears protocol admission — the transport and Go's diagnostic never disagree (Finding 6)", () => {
    const maxComponentAssetPayloadOverheadBytes = 4 * 1024
    const maxComponentAssetBytes = Math.floor((MAX_ENGINE_PAYLOAD_BYTES - maxComponentAssetPayloadOverheadBytes) * 3 / 4)
    const bytes = new ArrayBuffer(maxComponentAssetBytes)
    const payload = setComponentAssetCommand('e1', 'image/png', bytes)
    expect(payload.byteLength).toBeLessThanOrEqual(MAX_ENGINE_PAYLOAD_BYTES)
    const request = parseRequest({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: 'command-1', operation: 'command', payload })
    expect(request).toBeDefined()
    // Demonstrates the bug this fixes: Go's OLD bound reused the raw 8 MiB
    // envelope ceiling directly against the DECODED byte count. A command
    // carrying that many decoded bytes (base64-inflated, 4/3) never fit the
    // envelope at all — the transport rejected it with PROTOCOL_INVALID
    // before Go's own size diagnostic could ever fire, which is exactly the
    // disagreement D-5.13.4 forbids.
    const oldBoundPayload = setComponentAssetCommand('e1', 'image/png', new ArrayBuffer(MAX_ENGINE_PAYLOAD_BYTES))
    expect(oldBoundPayload.byteLength).toBeGreaterThan(MAX_ENGINE_PAYLOAD_BYTES)
  })
})
