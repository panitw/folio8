// Story 5.13's asset-authoring command factory. It stays exactly as opaque
// as component-command.ts/component-property-command.ts: this module knows
// nothing about the .folio format, does not hash, does not sniff the file's
// real format, and does not decide legality — it only base64-encodes the
// bytes the browser already read and carries the browser/OS's OWN declared
// media type opaquely to Go, which is the sole authority (component_commands.go's
// setComponentAsset, D-5.13.1).
//
// STORY 15.2a: the hand-rolled escape table that used to live here has been
// deleted in favour of command-json.ts. It read `charCodeAt(0)` of a value
// iterated BY CODE POINT, so an astral character was escaped from its high
// surrogate alone and its low unit was never emitted — an asset key carrying
// one parsed as valid JSON and bound to a key the author never typed.
import { commandBytes, jsonString } from './command-json'

export function setComponentAssetCommand(id: string, mediaType: string, bytes: ArrayBuffer): ArrayBuffer {
  return commandBytes('setComponentAsset', [['id', jsonString(id)], ['mediaType', jsonString(mediaType)], ['data', jsonString(bytesToBase64(bytes))]])
}

function bytesToBase64(bytes: ArrayBuffer): string {
  let text = ''
  for (const byte of new Uint8Array(bytes)) text += String.fromCharCode(byte)
  return btoa(text)
}

// assetBytesRequest is the opaque per-key paintable-bytes request payload
// (D-5.13.2's "Producer" clause): the engine's 'asset' operation decodes
// this UTF-8 key directly, mirroring how 'command' payloads are opaque
// bytes Go alone parses. It is NOT command JSON — it is one bare key — so it
// does not route through the command-JSON authority and must not start to:
// wrapping it would change the bytes the engine reads.
export function assetBytesRequest(key: string): ArrayBuffer {
  return new TextEncoder().encode(key).buffer
}
