import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { faceIsVariable, requireStaticTrueTypeTables, fontView } from './font-name-table'

// STORY 16.5 — THE INSTALL-TIME `fvar` FILTER, TIED TO GO'S REFUSAL OVER THE
// SAME BYTES.
//
// TWO FACTS MAKE THIS A TIE RATHER THAN A LOOKALIKE, and neither is visible from
// the assertions alone, so both are stated here.
//
//   (i) THE FIXTURES ARE THE VERY FILES GO `//go:embed`s. Not a copy, not a
//   regenerated equivalent: the same two paths, byte for byte.
//   `folio-go/testfont_embed_test.go:22,34` embeds
//   `testdata/fonts/Roboto-Regular.ttf` and
//   `testdata/fonts/notosansthai-variable-testonly/NotoSansThai-VF.ttf` into the
//   Go test binary, and the two constants below resolve those same paths. A
//   fixture of our own would let the two sides agree about different bytes,
//   which is the failure mode a duplicated predicate has.
//
//   (ii) IT MIRRORS AN EXISTING NAMED CONSTRUCTION RATHER THAN INVENTING ONE.
//   `folio-go/component_commands_test.go:2139`
//   `TestVariableFaceIsRefusedAtTheCommandAndAtIngestionOverTheSameBytes` feeds
//   ONE slice to both Go doors and asserts both refuse; `:2191`
//   `TestStaticFaceIsStillEmbeddedAtBothDoors` stands beside it as its NAMED
//   OVER-BROADNESS CONTROL, because a guard that refused every face would pass
//   every assertion in the first test. This file is that pair, with the
//   designer's install-time filter as the third door.
//
// AND THE FILTER MAY ONLY REFUSE. `fontset.RefuseVariableFace` remains the sole
// authority over what enters a document; nothing asserted here treats a `false`
// from `faceIsVariable` as permission to embed. See `variableface.go`'s own
// carve-out paragraph, and `faceIsVariable`'s doc, for why a refuse-only copy is
// permitted at install and forbidden at or behind the command.
//
// RED-PROOF (D-16.R.55: a red-proof must assert that its mutation applied).
// Recorded 2026-09-03, in a git worktree separate from the observing session,
// tree verified clean before and after, mutation asserted applied by diffing the
// file: deleting `faceIsVariable`'s body and returning `false` reds the variable
// arm below; the static arm and the divergence arm stay green, which is what
// says the two arms measure different things.

const here = path.dirname(fileURLToPath(import.meta.url))

// THE SAME PATHS GO EMBEDS. Resolved through `../../folio-go/...`, the idiom
// `src/engine-bounds-mirror.test.ts:44` established for a designer test reading
// the Go tree; thirteen designer test files already reference `folio-go`.
const goTestdata = path.resolve(here, '..', '..', 'folio-go', 'testdata', 'fonts')
const variableFixture = path.join(goTestdata, 'notosansthai-variable-testonly', 'NotoSansThai-VF.ttf')
const staticFixture = path.join(goTestdata, 'Roboto-Regular.ttf')

/**
 * A MISSING FIXTURE IS A FAILURE, NEVER A SKIP.
 *
 * A skipped tie is a tie that has stopped tying, reported in the one colour
 * nobody reads. If either file moves in the Go tree this must go red and name
 * the path it went looking for, because "the fixture moved" and "the predicate
 * broke" are both things this test exists to catch.
 */
const readFixture = (file: string): Buffer => {
  expect(fs.existsSync(file), `the shared fixture ${file} is not there; Go embeds it by this exact path (folio-go/testfont_embed_test.go), so a tie that cannot read it is broken, not skipped`).toBe(true)
  return fs.readFileSync(file)
}

describe('the install-time fvar filter, over the bytes Go embeds', () => {
  // ARM ONE: THE VARIABLE FACE IS REFUSED. This is the arm that carries the
  // story's acceptance criterion — fetched bytes carrying an `fvar` table are
  // refused at install, at the moment the author acted.
  it('refuses the variable fixture, which is the face Go refuses at both of its doors', () => {
    const bytes = readFixture(variableFixture)
    // NON-VACUITY FIRST: the file really is a static-TrueType-shaped container
    // that really does declare `fvar`. Without this a truncated or swapped
    // fixture would make the assertion below pass for the wrong reason.
    const tables = requireStaticTrueTypeTables(fontView(bytes))
    expect(Object.keys(tables), 'the variable fixture must actually declare an fvar table').toContain('fvar')
    expect(faceIsVariable(bytes)).toBe(true)
  })

  // ARM TWO: THE OVER-BROADNESS CONTROL, and it is the reason a refusal-only
  // test proves nothing. A filter that refused every face would pass the arm
  // above and every acceptance criterion phrased as "the variable one is
  // refused". Roboto is the same static face `TestStaticFaceIsStillEmbeddedAtBothDoors`
  // uses, and it must install.
  it('admits the static fixture, so a refuse-everything filter cannot pass this file', () => {
    const bytes = readFixture(staticFixture)
    const tables = requireStaticTrueTypeTables(fontView(bytes))
    expect(Object.keys(tables), 'the static fixture must not declare an fvar table').not.toContain('fvar')
    expect(faceIsVariable(bytes)).toBe(false)
  })

  // ARM THREE: THE DIVERGENCE, ASSERTED ON PURPOSE SO NOBODY "FIXES" IT.
  //
  // Over bytes that are not a font at all the two sides deliberately answer
  // differently. `fontset.RefuseVariableFace` returns `nil` — it answers exactly
  // one question and an unparsable face is not a variable one, and `fontset.New`
  // reports the parse failure in its own sentence. The designer THROWS EARLIER,
  // out of `requireStaticTrueTypeTables`, because its caller is holding a 200
  // from a third party and "this is not a font" is the refusal that response
  // needs.
  //
  // Both behaviours are correct for their own caller and NEITHER IS TO BE
  // WIDENED TO MATCH THE OTHER: widening Go would make `RefuseVariableFace` a
  // second, partial copy of `New`'s admission rules; widening the designer would
  // make it answer a question it is not the authority on.
  it('throws on bytes that are not a font, where Go returns nil, and that asymmetry is deliberate', () => {
    const notAFont = new TextEncoder().encode('<!doctype html><title>404</title>').buffer
    expect(() => faceIsVariable(notAFont)).toThrow(/not a static TrueType sfnt/)
    // Two bytes is the same answer in the same words, so the refusal is about
    // the container and not about a length that happens to reach a version field.
    expect(() => faceIsVariable(new Uint8Array([0, 1]).buffer)).toThrow(/too short to carry a table directory/)
  })
})
