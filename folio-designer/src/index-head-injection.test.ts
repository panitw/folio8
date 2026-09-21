import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// THE HEAD OF `index.html` IS SINGLE-OCCUPANCY, AND NOTHING ELSE PROVED IT.
//
// Two separate injections target the closing head tag of the production page:
// Vite puts the bundle and stylesheet there, and `generate-offline-release.mjs`
// (line 168) then puts the release-bootstrap trio there, with a plain
// `String.replace(...)` — which takes the FIRST occurrence in the file and
// nothing else.
//
// TWO WAYS TO BREAK THAT, AND BOTH HAVE HAPPENED HERE, ONE DAY APART:
//
//  1. A COMMENT THAT SPELLS THE CLOSING HEAD TAG. The first occurrence is then
//     inside the comment, both injections land between `<!--` and the
//     terminator, and the shipped page carries NO EXECUTABLE SCRIPT AT ALL —
//     an empty `<div id="root">` and a blank designer.
//  2. A COMMENT THAT SPELLS A TERMINATOR. The comment ends early, and every
//     line after it renders as visible prose above the application.
//
// ⚠ EVERY EXISTING GUARD STAYS GREEN THROUGH BOTH. `npm run build` succeeds;
// `build:offline` finds a tag to replace, so its own `'production index has no
// head for release bootstrap'` throw never fires; `verify:offline` matches the
// bootstrap by regex and never asks whether that match is commented out, or
// whether the head it sits in is still the head; the asset set is unchanged, so
// the exact-set check and the 30/30 core pin both pass. Failure one is visible
// only as a blank page and failure two only as garbage text — and nothing in
// the pipeline opens a browser.
//
// SO THIS TEST PARSES THE PAGE RATHER THAN COUNTING TOKENS. The first version
// of this file counted `<!--` against `-->` and passed on failure two, which is
// the whole reason the parse is here: a balanced count says nothing about where
// the parser actually ended the comment.
//
// THE BOUND IT CLAIMS, AND NOTHING WIDER (the house rule for scans): it proves
// the SOURCE page offers exactly one injection site and leaks no text. It does
// not prove the built page is correct — a Vite plugin that emitted a second
// head could still do this, and no source test can see that.
const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'index.html'), 'utf8')

// SPELLED FROM PIECES ON PURPOSE, both of them. Written literally, this file's
// own constants would be exactly what it forbids if anyone ever moved them into
// `index.html` — and a reader comparing the two files should see immediately
// that the absence there is deliberate rather than an oversight.
const closingHead = `</${'head'}>`
const terminator = `--${'>'}`

const parse = (html: string) => new DOMParser().parseFromString(html, 'text/html')

describe('index.html head injection site', () => {
  it('spells the closing head tag exactly once, so the injections cannot land inside a comment', () => {
    const occurrences = source.split(closingHead).length - 1
    expect(occurrences, `index.html must spell ${closingHead} exactly once — a second mention, even inside a comment, silently swallows the bundle and the release bootstrap and ships a blank page`).toBe(1)
  })

  it('leaks no comment prose into the body, so no comment terminates early', () => {
    const body = parse(source).body
    // The body's only legitimate text is whitespace between `<div id="root">`
    // and the module script. Any other text is a comment that ended early.
    const stray = Array.from(body.childNodes)
      .filter((node) => node.nodeType === 3)
      .map((node) => node.textContent ?? '')
      .join('')
      .trim()
    expect(stray, 'text leaked into <body>: a comment in the head terminated earlier than intended, and this prose renders above the application').toBe('')
  })

  it('keeps the title and both metas inside the parsed head', () => {
    const head = parse(source).head
    expect(head.querySelector('title')?.textContent).toBe('Folio8 Designer')
    expect(head.querySelector('meta[charset]'), 'the charset meta must parse inside <head>').not.toBeNull()
    expect(head.querySelector('meta[name="viewport"]'), 'the viewport meta must parse inside <head>').not.toBeNull()
  })

  // THE OBSERVER IS PROVEN ALIVE, ON BOTH FAILURE SHAPES. Every assertion above
  // passes trivially on a page with no comments at all, so this suite would
  // have been green on the day either bug shipped if it only ever read the real
  // file.
  //
  // ⚠ THE FIXTURES ARE BUILT FROM THE FILE'S STRUCTURE, NEVER FROM ITS PROSE.
  // An earlier version spliced on the literal string `'<!-- USAGE'`; rewording
  // the comment would have made the splice a silent no-op and turned the
  // control into a test that asserts nothing while still passing.
  const headAt = source.indexOf(closingHead)
  const splice = (insert: string) => `${source.slice(0, headAt)}${insert}\n    ${source.slice(headAt)}`

  it('fails on a comment that spells the closing head tag', () => {
    const broken = splice(`<!-- keep ${closingHead} literal ${terminator}`)
    expect(broken.split(closingHead).length - 1, 'the fixture must actually contain the extra mention').toBeGreaterThan(1)
  })

  it('fails on a comment that terminates early', () => {
    const broken = splice(`<!-- ends here ${terminator} and this prose escapes ${terminator}`)
    const body = parse(broken).body
    const stray = Array.from(body.childNodes)
      .filter((node) => node.nodeType === 3)
      .map((node) => node.textContent ?? '')
      .join('')
      .trim()
    expect(stray, 'the fixture must actually leak text, or the body assertion above proves nothing').not.toBe('')
  })
})
