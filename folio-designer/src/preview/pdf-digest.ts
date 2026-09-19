/**
 * STORY 13.3 — THE BROWSER COMPUTES THE HASH IT IS ABOUT TO ASK SOMEONE TO
 * COMPARE BY EYE (DW-270).
 *
 * Until this module existed, the digest on the preview screen was admitted by
 * SHAPE alone — `/^[a-f0-9]{64}$/` in `engine-protocol.ts` — and nothing in
 * `src/` ever hashed the bytes it claimed to cover. Between the engine and the
 * screen those bytes cross four value-preserving copies: the worker's
 * allocation, the structured-clone transfer, `copyBytes` in `EngineClient`, and
 * `slice(0)` at install time. Each is correct today; none was checked.
 *
 * The rail promotes that digest from a grey footnote to a bordered block a
 * person is told to compare against a producer's. Asking someone to verify a
 * claim we have not verified ourselves is worse than not showing it at all, so
 * the caller refuses to install a preview whose digest does not describe its own
 * bytes.
 *
 * NOT `storedFaceKey`, WHOSE NAME WOULD LIE. That function hashes a font face
 * for the face store's key space; reusing it would put "stored face" in the
 * stack of every PDF admission. The two are the same three lines of SHA-256 and
 * are deliberately spelt twice under their own names.
 *
 * `crypto.subtle` is present in every browser this designer supports and in the
 * test environment (jsdom provides it; `font-store.test.ts` already cross-checks
 * this idiom against Node's `createHash`).
 */
export async function pdfDigest(bytes: ArrayBuffer): Promise<string> {
  // A VIEW, NOT THE BUFFER ITSELF. The bytes reaching here have crossed a
  // worker boundary, and `font-store.ts` records that a buffer from another
  // realm can fail `instanceof ArrayBuffer` while being perfectly hashable.
  const digest = await crypto.subtle.digest('SHA-256', new Uint8Array(bytes))
  let hex = ''
  for (const byte of new Uint8Array(digest)) hex += byte.toString(16).padStart(2, '0')
  return hex
}
