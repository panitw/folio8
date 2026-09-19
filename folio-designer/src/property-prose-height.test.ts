import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// The font-family combobox and the CONTENT field share `property-value-prose`
// because both want body type instead of the mono the other property values
// use. They do NOT share a height contract: CONTENT is a multi-line textarea
// with a four-row floor and a resize affordance, and the combobox is a
// single-line input. When the floor lived on the shared class the combobox
// rendered at roughly twice its intended height.
//
// STORY 17.5 CHANGED THIS FILE'S SUBJECT AND NOT ITS PURPOSE. The affordance
// on the textarea used to be `resize: vertical` — the USER AGENT's own 16px
// diagonal grip, the one control in the inspector this application did not
// draw. It is now `resize: none` plus a hand-drawn hit strip on the bottom
// edge of `.property-field-prose`, in the idiom of the canvas handles. The
// rule that must not drift onto the shared class is therefore a different
// rule; the class that must not carry it is the same class, and these tests
// still exist to catch exactly that edit.
//
// They read the two tracked sources rather than any built artefact, so they
// fail on the edit that would reintroduce the bug rather than on a build.
// jsdom applies no stylesheet, so source text is the only place these facts
// are observable in a unit test at all — the cursor and the absent grip are
// proved for real in the browser run.

const here = path.dirname(fileURLToPath(import.meta.url))
const css = fs.readFileSync(path.join(here, 'App.css'), 'utf8')
const tsx = fs.readFileSync(path.join(here, 'App.tsx'), 'utf8')

describe('property-value-prose height is scoped to the textarea', () => {
  it('does not put a height floor or a resize affordance on the shared class', () => {
    const shared = css.match(/^\.property-value-prose \{[^}]*\}/m)
    expect(shared, 'the shared .property-value-prose rule must exist').not.toBeNull()
    expect(shared![0]).not.toMatch(/min-height/)
    expect(shared![0]).not.toMatch(/resize/)
    expect(shared![0]).not.toMatch(/height/)
    expect(shared![0]).not.toMatch(/cursor/)
  })

  it('scopes the height floor to textarea and turns the native grip off', () => {
    const scoped = css.match(/^textarea\.property-value-prose \{[^}]*\}/m)
    expect(scoped, 'the textarea-scoped rule must exist').not.toBeNull()
    expect(scoped![0]).toMatch(/min-height:\s*72px/)
    expect(scoped![0]).toMatch(/resize:\s*none/)
    expect(scoped![0]).not.toMatch(/resize:\s*(?:vertical|both|horizontal)/)
  })

  // The `textarea` BASE rule (App.css) still says `resize: vertical`, and that
  // is deliberately out of this story's scope — the raw-parameter textarea
  // keeps the grip for now. It does NOT reach the CONTENT field: `textarea`
  // is specificity (0,0,1) and `textarea.property-value-prose` is (0,1,1), so
  // the scoped `none` wins. That ordering is read by hand — jsdom applies no
  // stylesheet — so what is pinned here is the shape the hand-reading depends
  // on: the base rule is a bare element selector, and the scoped rule that
  // outranks it is the one asserted above.
  it('is not overridden by the base textarea rule, which keeps its own grip', () => {
    const base = css.match(/^textarea \{[^}]*\}/m)
    expect(base, 'the base textarea rule must exist').not.toBeNull()
    expect(base![0]).toMatch(/resize:\s*vertical/)
    expect(css.indexOf('textarea.property-value-prose {')).toBeGreaterThan(-1)
    // THE REGRESSION THIS ROW IS FOR. Specificity is the whole argument, and
    // `!important` on the base rule would beat the scoped `none` outright and
    // put the grip back on the CONTENT field with every other assertion in
    // this file still green. Review found the row passing that mutation
    // unchanged; it does not now.
    expect(base![0]).not.toMatch(/!\s*important/)
  })

  it('draws the handle on the bordered box, full width, with a resize cursor', () => {
    const handle = css.match(/^\.property-prose-resize \{[^}]*\}/m)
    expect(handle, 'the handle rule must exist').not.toBeNull()
    expect(handle![0]).toMatch(/position:\s*absolute/)
    // Full width of the box the author sees, not a corner grip.
    expect(handle![0]).toMatch(/left:\s*0/)
    expect(handle![0]).toMatch(/right:\s*0/)
    // On the BOTTOM edge, straddling the border line rather than sitting
    // wholly inside or outside it.
    expect(handle![0]).toMatch(/bottom:\s*-?\d/)
    expect(handle![0]).not.toMatch(/\btop:/)
    const hit = /height:\s*(\d+)px/.exec(handle![0])
    expect(hit, 'the handle must declare a hit height').not.toBeNull()
    expect(Number(hit![1])).toBeGreaterThanOrEqual(6)
    expect(Number(hit![1])).toBeLessThanOrEqual(8)
    expect(handle![0]).toMatch(/cursor:\s*ns-resize/)
    // LOAD-BEARING, NOT DECORATION: without `touch-action: none` a touch press
    // on the strip is taken as a pan, the browser fires `pointercancel`, and
    // the resize ends before it starts. Nothing else in the suite could see
    // that, because jsdom has no touch scrolling to be stolen by.
    expect(handle![0]).toMatch(/touch-action:\s*none/)
  })

  // "IT PAINTS NOTHING" AS AN ALLOWLIST, WHICH IS THE ONLY WAY IT IS REAL.
  // This was a two-item denylist (`border`, `content:`), and review measured
  // what it let through: `border-top`, `border-bottom`, `box-shadow` and
  // `outline` all passed it, and a `.property-prose-resize::after` rule
  // elsewhere in the file was invisible to it entirely. Enumerating what the
  // rule MAY declare fails on anything new by construction, including the
  // property nobody has thought of yet.
  it('declares nothing that could paint, and grows no pseudo-element', () => {
    const handle = css.match(/^\.property-prose-resize \{([^}]*)\}/m)
    expect(handle, 'the handle rule must exist').not.toBeNull()
    const declared = handle![1].split(';').map((one) => one.trim()).filter((one) => one !== '').map((one) => one.slice(0, one.indexOf(':')).trim())
    const permitted = ['position', 'left', 'right', 'bottom', 'height', 'background', 'cursor', 'touch-action']
    expect([...declared].sort()).toEqual([...permitted].sort())
    // `background: transparent` is the one paint-adjacent property on the
    // list, and it is here to say the strip is invisible rather than to draw.
    expect(handle![0]).toMatch(/background:\s*transparent/)
    // A generated box would paint without appearing in the rule above at all.
    expect(css).not.toMatch(/\.property-prose-resize\s*::?(?:after|before)/)
  })

  // The containing block the absolute handle needs belongs to the PROSE row.
  // Putting `position` on `.property-field` would give every property row in
  // the panel a containing block it never had, which is the same shape of
  // over-broad rule this file was created to catch.
  it('gives the containing block to the prose row only', () => {
    const prose = css.match(/^\.property-field-prose \{[^}]*\}/m)
    expect(prose, 'the .property-field-prose rule must exist').not.toBeNull()
    expect(prose![0]).toMatch(/position:\s*relative/)
    const row = css.match(/^\.property-field \{[^}]*\}/m)
    expect(row, 'the shared .property-field rule must exist').not.toBeNull()
    expect(row![0]).not.toMatch(/position:/)
  })

  // Two copies of 72, in two languages, and no DOM measurement available to
  // derive either from the other. The CSS floor is where the box RESTS; the
  // TSX floor is where a drag CLAMPS. If they part company the box springs
  // back on the first drag, and nothing else in the suite would say so.
  it('spells the same 72px floor in the stylesheet and in the drag clamp', () => {
    const clamp = /const PROSE_MIN_HEIGHT_PX = (\d+)/.exec(tsx)
    expect(clamp, 'the drag clamp constant must exist').not.toBeNull()
    const floor = /^textarea\.property-value-prose \{[^}]*min-height:\s*(\d+)px/m.exec(css)
    expect(floor, 'the CSS floor must exist').not.toBeNull()
    expect(clamp![1]).toBe(floor![1])
    expect(clamp![1]).toBe('72')
    expect(tsx).toMatch(/Math\.max\(PROSE_MIN_HEIGHT_PX,/)
  })

  // Non-vacuity: the tests above would pass if nothing used the class at all.
  // These pin that the pairing they are about is still real.
  it('is still shared by exactly one textarea and one input', () => {
    const textareas = tsx.match(/<textarea[^>]*property-value-prose/g) ?? []
    const inputs = tsx.match(/<input[^>]*property-value-prose/g) ?? []
    expect(textareas).toHaveLength(1)
    expect(inputs).toHaveLength(1)
  })

  // And that the handle rule has exactly one wearer, rendered on the prose
  // branch, so "no handle on the combobox" is a structural fact rather than a
  // hope. `App.test.tsx` asserts the rendered consequence.
  it('renders the handle once, on the prose branch, as a non-focusable span', () => {
    const handles = tsx.match(/<span className="property-prose-resize"[^>]*>/g) ?? []
    expect(handles).toHaveLength(1)
    expect(tsx).toContain('{prose && <span className="property-prose-resize" aria-hidden="true"')
    expect(handles[0]).not.toMatch(/tabIndex/)
  })

  it('keeps body type on the shared class, which is why it is shared at all', () => {
    expect(css).toMatch(/\.property-value-prose \{[^}]*font:\s*var\(--type-body-em\)/)
  })
})
