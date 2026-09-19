import { createHash } from 'node:crypto'

/**
 * STORY 13.3 — ONE PLACE THAT KNOWS WHAT A RENDER FIXTURE'S DIGEST ACTUALLY IS.
 *
 * Twenty fixture sites across four suites used to answer `'a'.repeat(64)` for
 * bytes whose real SHA-256 is nothing of the kind, which was harmless only
 * because nothing ever checked. App.tsx now recomputes the digest before it
 * installs a preview (DW-270), so a fixture that lies about its own bytes is a
 * fixture that renders no preview at all — and every one of those suites would
 * have been "fixed" by pasting a different constant, which is the same defect
 * one revision later.
 *
 * ⚠ THE DIGEST IS DERIVED FROM THE BYTES, HERE, BY NODE — never copied from
 * what the browser code produced. The application hashes with
 * `crypto.subtle`; this hashes with `node:crypto`. The two sides of every
 * digest assertion in this repository therefore reach the same value by
 * different implementations, which is the only arrangement in which agreeing
 * means anything (D-11.2.2, D-11.2.8).
 */
export function fixtureDigest(bytes: ArrayBuffer | Uint8Array): string {
  return createHash('sha256').update(bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes)).digest('hex')
}

/** The one-byte stand-in PDF the preview suites have always handed the viewer. */
export const PDF_FIXTURE_BYTES = new Uint8Array([9])
export const PDF_FIXTURE_DIGEST = fixtureDigest(PDF_FIXTURE_BYTES)

/**
 * The two render facts Story 13.3 added to the reply. The elapsed value is
 * deliberately NOT zero here: `0` is a legitimate answer that has its own
 * dedicated coverage, so a shared fixture that used it would make the
 * all-or-nothing admission arm indistinguishable from a dropped field for every
 * other test in the file.
 */
export const RENDER_ELAPSED_MS = 7
export const RENDER_ENGINE_VERSION = '0.0.0-dev'
