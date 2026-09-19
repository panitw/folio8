import fs from 'node:fs'
import path from 'node:path'
import ts from 'typescript'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// THE SOLENESS GUARD FOR THE COMMAND-JSON AUTHORITY.
//
// It is an ALLOWLIST, not a denylist, and the distinction is the whole point.
// A denylist enumerating the splices we happen to know about — `${id}` inside
// quotes, `${value}` in a number position — cannot see the splice nobody
// thought to forbid, which is exactly how six encoders reached three different
// answers to "escape this for JSON". So the question this file asks is the
// other one: WHICH production modules build command JSON at all? The answer
// must be exactly one, and the moment it is two the test is red without
// anybody having to have predicted the second one's shape.
//
// TWO DETECTORS, because there are two ways to write JSON by hand and only one
// of them leaves a recognisable key literal behind:
//
//   1. HAND-WRITTEN STRUCTURE — a string or template literal whose own text
//      opens an object on a quoted key, `{"kind":` or `{"op":`. That is what
//      all six encoders looked like before this story.
//   2. HAND-WRITTEN SCAFFOLDING — a template literal whose static text is
//      nothing BUT JSON punctuation, with every name and value interpolated:
//      `{${key}:${value}}`. It leaves no key literal at all, so detector 1 is
//      blind to it, and it is the obvious way to evade detector 1.
//
// Detector 2 finds nothing in production today and is proved by mutation
// instead — a zero with no population behind it asserts nothing, so the
// mutation cases below are its non-vacuity, not decoration.
const sourceDir = path.dirname(fileURLToPath(import.meta.url))
const AUTHORITY = 'command-json.ts'

const productionFiles = fs.readdirSync(sourceDir, { recursive: true })
  .filter((entry): entry is string => typeof entry === 'string' && /\.(?:ts|tsx)$/.test(entry) && !/\.test\.(?:ts|tsx)$/.test(entry))

type Source = Readonly<{ name: string; source: string }>

const sources = (): ReadonlyArray<Source> => productionFiles.map((name) => ({ name, source: fs.readFileSync(path.join(sourceDir, name), 'utf8') }))

// An object literal opened on a quoted key. Deliberately NOT the looser
// `[{,[]\s*"` shape an earlier draft used: that one also matched the comma of
// ordinary English prose inside a generated licence blob, and a guard that
// reddens on prose gets edited away rather than answered.
const jsonStructure = /\{\s*"[A-Za-z_][A-Za-z0-9_]*"\s*:/
// Static text made only of JSON punctuation, OPENED by a brace, bracket or
// quote. Both halves earn their keep against real files in this tree: the
// punctuation-only rule is what keeps prose and a CSS selector out, and the
// must-OPEN-the-structure rule is what keeps out a label like `${name}[]`,
// whose static text is punctuation but which encloses nothing. What it lets
// through is the evasion detector 1 cannot see — `{${key}:${value}}`,
// `[${items}]`, and a value spliced raw INSIDE quotes as `"${preset}"`.
const jsonScaffolding = /^[\s{}[\]:,"]+$/
const opensJsonStructure = /^[{["]/

function commandJsonBuilders(files: ReadonlyArray<Source>): string[] {
  const builders: string[] = []
  for (const file of files) {
    const parsed = ts.createSourceFile(file.name, file.source, ts.ScriptTarget.Latest, true)
    const visit = (node: ts.Node): void => {
      if ((ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) && jsonStructure.test(node.text)) builders.push(file.name)
      if (ts.isTemplateExpression(node)) {
        const statik = [node.head.text, ...node.templateSpans.map((span) => span.literal.text)].join('')
        if (jsonStructure.test(statik)) builders.push(file.name)
        else if (jsonScaffolding.test(statik) && opensJsonStructure.test(node.head.text)) builders.push(file.name)
      }
      ts.forEachChild(node, visit)
    }
    visit(parsed)
    // A template literal is not the only way to interpolate: string
    // concatenation reaches the same bytes. A `+` whose left or right operand
    // is a JSON-structural literal is the same splice with different syntax.
    const concatenations = [...file.source.matchAll(/(['"])(\{\s*\\?"[A-Za-z_][A-Za-z0-9_]*\\?"\s*:[^\n]*?)\1\s*\+/g)]
    if (concatenations.length > 0) builders.push(file.name)
  }
  return [...new Set(builders)]
}

describe('command JSON has exactly one author', () => {
  it('scans a non-vacuous production corpus', () => {
    // Without this, every assertion below is a statement about an empty list.
    expect(productionFiles.length).toBeGreaterThan(40)
    expect(productionFiles).toContain(AUTHORITY)
  })

  it('finds command JSON built in the authority and nowhere else in production', () => {
    expect(commandJsonBuilders(sources())).toEqual([AUTHORITY])
  })

  it('names every command factory and asserts each one has stopped building JSON itself', () => {
    // BY NAME, and page-setup-command.ts by name in particular: it was the one
    // encoder with no escaping and no test file at all, so "the scan found
    // nothing" must not be able to mean "the scan never looked at it".
    // THIS LIST IS SPELLED TWICE IN THIS FILE — here and in the sibling test
    // below — and BOTH must move together: a factory added to only one of them
    // is scanned but never named, and the other test's check never looks at it.
    const factories = ['page-setup-command.ts', 'component-command.ts', 'component-asset-command.ts', 'component-property-command.ts', 'table-column-command.ts', 'font-chain-command.ts', 'band-height-command.ts', 'section-break-command.ts', 'document-settings-command.ts', 'table-style-command.ts']
    for (const factory of factories) {
      expect(productionFiles).toContain(factory)
      expect(fs.readFileSync(path.join(sourceDir, factory), 'utf8')).toContain(`from './${AUTHORITY.replace(/\.ts$/, '')}'`)
    }
    expect(commandJsonBuilders(sources().filter((file) => factories.includes(file.name)))).toEqual([])
  })

  it('lets no command factory spell a wire scalar for itself', () => {
    // THE OTHER WAY TO BYPASS THE AUTHORITY, and it leaves no JSON punctuation
    // behind for the detectors above to find: converting a value with a bare
    // `String(...)` and dropping the result into a field. It is invisible for
    // booleans, whose JS spelling happens to match their JSON spelling — which
    // is exactly why an exception for them survived the first consolidation
    // pass unnoticed.
    //
    // `\bString\(` does not match `jsonString(`: there is no word boundary
    // between `n` and `S`. The mutation cases below prove both directions.
    // THE SECOND SPELLING OF THE SAME LIST (see the test above). BOTH must
    // move together: a factory added only to the list above is never reached by
    // the `\bString(` check below, and this file goes on reporting no offenders.
    const factories = ['page-setup-command.ts', 'component-command.ts', 'component-asset-command.ts', 'component-property-command.ts', 'table-column-command.ts', 'font-chain-command.ts', 'band-height-command.ts', 'section-break-command.ts', 'document-settings-command.ts', 'table-style-command.ts']
    const offenders = factories.filter((factory) => /\bString\(/.test(withoutLineComments(fs.readFileSync(path.join(sourceDir, factory), 'utf8'))))
    expect(offenders).toEqual([])
    // Non-vacuity in both directions, because a regexp that matched nothing
    // would produce the same empty list.
    expect(/\bString\(/.test('if (typeof value === \'boolean\') return String(value)')).toBe(true)
    expect(/\bString\(/.test('return jsonString(value)')).toBe(false)
    // And `String.fromCharCode`, which two factories legitimately use for
    // base64, must not be caught by it.
    expect(/\bString\(/.test('binary += String.fromCharCode(...view)')).toBe(false)
  })

  it('reddens on a single reintroduced splice, in each shape a splice can take', () => {
    // ONE splice is enough, in each of the three shapes, because "reintroducing
    // a single splice anywhere reddens it" is the property — not "reintroducing
    // all six encoders reddens it".
    expect(commandJsonBuilders([{ name: 'page-setup-command.ts', source: 'const text = `{"kind":"pageSetup","version":1,"preset":"${preset}"}`' }])).toEqual(['page-setup-command.ts'])
    expect(commandJsonBuilders([{ name: 'component-command.ts', source: 'const text = `{"kind":"deleteComponent","version":1,"id":"${id}"}`' }])).toEqual(['component-command.ts'])
    expect(commandJsonBuilders([{ name: 'new-encoder.ts', source: 'const text = `{${quote(field)}:${change}}`' }])).toEqual(['new-encoder.ts'])
    expect(commandJsonBuilders([{ name: 'new-encoder.ts', source: 'const text = `[${ids.join(",")}]`' }])).toEqual(['new-encoder.ts'])
    expect(commandJsonBuilders([{ name: 'new-encoder.ts', source: 'const text = `"${preset}"`' }])).toEqual(['new-encoder.ts'])
    expect(commandJsonBuilders([{ name: 'new-encoder.ts', source: 'const text = \'{"kind":"addFontChain","version":1,"name":\' + quote(name) + \'}\'' }])).toEqual(['new-encoder.ts'])
    // A no-substitution literal counts too: a constant command body spliced
    // later by String.replace is the same defect with an extra step.
    expect(commandJsonBuilders([{ name: 'new-encoder.ts', source: 'const template = `{"kind":"deleteComponent","version":1,"id":"ID"}`' }])).toEqual(['new-encoder.ts'])
  })

  it('does not redden on the shapes that are not command JSON', () => {
    // The negative half. A guard that also fires on prose, on CSS selectors and
    // on a colon-joined key would be answered by editing this file rather than
    // the codebase, which is how a guard stops meaning anything.
    expect(commandJsonBuilders([{ name: 'font-licence.ts', source: 'const text = `this licence is published as "${name}", and it is not one this designer ships`' }])).toEqual([])
    expect(commandJsonBuilders([{ name: 'TableEditor.tsx', source: 'const selector = `[data-matrix-cell="${row}:${column}"]`' }])).toEqual([])
    expect(commandJsonBuilders([{ name: 'diagnostic-presenter.tsx', source: 'const key = `${index}:${severity}:${code}`' }])).toEqual([])
    expect(commandJsonBuilders([{ name: 'engine-client.ts', source: 'const label = `${operation} took ${elapsed}ms`' }])).toEqual([])
    // Punctuation that encloses nothing. Both of these are live in this tree —
    // App.tsx's binding-tree label and sample-data.ts's collection label — and
    // an earlier draft of the scaffolding rule reddened on both.
    expect(commandJsonBuilders([{ name: 'App.tsx', source: 'const label = `${node.segments.join(".")}[]`' }])).toEqual([])
  })
})

// withoutLineComments drops whole-line `//` comments, so a guard cannot redden
// on the prose that explains what not to write. It is deliberately not a
// general comment stripper: it only has to survive the headers these six files
// carry, and a half-correct scanner that mistook the `//` inside a URL for a
// comment would make the check vacuous exactly where it matters.
function withoutLineComments(source: string): string {
  return source.split('\n').filter((line) => !line.trimStart().startsWith('//')).join('\n')
}
