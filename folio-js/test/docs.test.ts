import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { repoRoot } from './helpers.js'

// CAP-8: docs/folio-js.md and docs/folio-js.html teach THIS library, and are
// held to it rather than trusted to keep up with it.
//
// Three claims, each executed:
//   1. the two twins agree in reader-visible text — punctuation included, so a
//      code sample cannot differ between them in brackets, generics or commas;
//   2. every public item of the package's entry points is named in both;
//   3. the guide's first-PDF snippet is the very text test/package.test.ts
//      already packs, installs offline and runs against a corpus fixture — so
//      the published code is executed rather than illustrated.
const docsDir = join(repoRoot, 'docs')
const packageRoot = join(repoRoot, 'folio-js')

const markdown = join(docsDir, 'folio-js.md')
const page = join(docsDir, 'folio-js.html')

const BEGIN = '<!-- twin:begin -->'
const END = '<!-- twin:end -->'

function read(path: string): string {
  const text = readFileSync(path, 'utf8')
  if (text.length === 0) throw new Error(`${path} is empty`)
  return text
}

/**
 * The region both twins publish, between the markers, and the two slices
 * outside it. Only the page's chrome may live outside — a test below holds
 * that, so prose cannot be authored where the comparison does not reach.
 */
function split(path: string): { prologue: string; region: string; epilogue: string } {
  const text = read(path)
  const begin = text.indexOf(BEGIN)
  const end = text.indexOf(END)
  if (begin < 0 || end < 0 || end < begin) throw new Error(`${path} has no twin:begin/twin:end region`)
  return { prologue: text.slice(0, begin), region: text.slice(begin, end).replace(/<!--[\s\S]*?-->/g, ' '), epilogue: text.slice(end + END.length) }
}

function decodeEntities(html: string): string {
  return html
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, '&')
}

/**
 * The Markdown region as a reader sees it: the format's own syntax — fences and
 * their info strings, table pipes and rules, heading hashes, list bullets,
 * emphasis and code ticks, and every link target — removed, and NOTHING else.
 * Lines inside a fence are kept verbatim, because a code sample's backticks,
 * pipes and dashes are content rather than markup.
 */
function markdownText(region: string): string {
  const out: string[] = []
  let fence = false
  for (const raw of region.split('\n')) {
    if (/^\s*```/.test(raw)) { fence = !fence; continue }
    if (fence) { out.push(raw); continue }
    let line = raw
    if (/^\s*\|[\s:|-]*\|\s*$/.test(line) && line.includes('-')) continue
    line = line.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    line = line.split('`').join('').split('**').join('')
    line = line.replace(/^\s{0,3}#{1,6}\s+/, '')
    line = line.replace(/^\s*[-*+]\s+/, ' ')
    if (line.trimStart().startsWith('|')) line = line.split('|').join(' ')
    out.push(line)
  }
  return out.join('\n')
}

function htmlText(region: string): string {
  return decodeEntities(region.replace(/<[^>]*>/g, ' '))
}

/**
 * The reader-visible token stream: words AND single punctuation marks, so that
 * `Promise<Template>` and `Promise<Template[]>` are different pages. Whitespace
 * alone is not a token, because the two formats wrap lines differently.
 */
function readerTokens(path: string): string[] {
  const { region } = split(path)
  const text = path.endsWith('.html') ? htmlText(region) : markdownText(region)
  return text.match(/[A-Za-z0-9]+|[^\sA-Za-z0-9]/g) ?? []
}

/** The twin region with markup stripped, for whole-word lookups. */
function readerText(path: string): string {
  const { region } = split(path)
  return path.endsWith('.html') ? decodeEntities(region.replace(/<[^>]*>/g, ' ')) : region
}

/**
 * The source file behind each published entry point, derived from
 * package.json's `exports` rather than listed here — an entry point added to
 * the package and not to this scan would otherwise go undocumented in silence.
 */
function entryPointSources(): string[] {
  const manifest = JSON.parse(read(join(packageRoot, 'package.json'))) as { exports: Record<string, unknown> }
  const sources: string[] = []
  for (const [specifier, target] of Object.entries(manifest.exports)) {
    if (specifier === './package.json') continue
    const resolved = typeof target === 'string' ? target : (target as { default?: string }).default
    const built = /^\.\/dist\/([\w-]+)\.js$/.exec(resolved ?? '')
    if (!built) throw new Error(`package.json exports "${specifier}" as ${JSON.stringify(resolved)}, which this scan cannot map back to a source file`)
    sources.push(join(packageRoot, 'src', `${built[1]!}.ts`))
  }
  if (sources.length === 0) throw new Error('package.json publishes no entry point')
  return sources
}

/**
 * Every name the package's entry points publish, read from their sources. Type-
 * only exports are part of the contract and are scanned too, because
 * `Diagnostic` and `Severity` are what a TypeScript caller writes even though
 * they leave no runtime trace. A re-export this scanner cannot name — `export *`
 * or `export default` — fails rather than being skipped.
 */
function publicNames(): string[] {
  const names = new Set<string>()
  for (const path of entryPointSources()) {
    const source = read(path)
    for (const [match] of source.matchAll(/^export\s+(?:type\s+)?\*.*$/gm)) {
      throw new Error(`${path} has a star re-export this scan cannot resolve, so the guide's surface check would silently miss its names: ${match.trim()}`)
    }
    for (const [match] of source.matchAll(/^export\s+default\b.*$/gm)) {
      throw new Error(`${path} has a default export this scan cannot name: ${match.trim()}`)
    }
    for (const [, list] of source.matchAll(/^export\s+(?:type\s+)?\{([^}]*)\}/gm)) {
      for (const item of list!.split(',')) {
        const name = item.trim().split(/\s+as\s+/).pop()?.trim()
        if (name) names.add(name)
      }
    }
    for (const [, name] of source.matchAll(/^export\s+(?:declare\s+)?(?:async\s+)?(?:function|const|let|class|interface|type|enum)\s+(\w+)/gm)) names.add(name!)
  }
  return [...names].sort()
}

/** The first ```js block of README.md — the snippet package.test.ts executes. */
function readmeSnippet(): string {
  const match = /```js\n([\s\S]*?)```/.exec(read(join(packageRoot, 'README.md')))
  if (!match) throw new Error('folio-js/README.md has no ```js block — the tested first-PDF snippet is gone')
  return match[1]!.trim()
}

/** The first code sample of the guide's "Your first PDF" section, as text. */
function firstPdfSample(): string {
  const section = /<section id="your-first-pdf">([\s\S]*?)<\/section>/.exec(read(page))
  if (!section) throw new Error('docs/folio-js.html has no your-first-pdf section')
  const sample = /<div class="sample"><pre[^>]*>([\s\S]*?)<\/pre><\/div>/.exec(section[1]!)
  if (!sample) throw new Error('docs/folio-js.html: the your-first-pdf section carries no code sample')
  return decodeEntities(sample[1]!.replace(/<[^>]*>/g, '')).trim()
}

describe('the folio-js guide', () => {
  it('publishes the same text in both twins, punctuation included', () => {
    const md = readerTokens(markdown)
    const html = readerTokens(page)
    // VACUITY GUARD: two empty regions are not two agreeing twins.
    expect(md.length, 'docs/folio-js.md carries almost no prose').toBeGreaterThan(1500)
    let i = 0
    while (i < md.length && i < html.length && md[i] === html[i]) i++
    const context = (tokens: string[]) => tokens.slice(Math.max(0, i - 20), i + 20).join(' ')
    expect(
      i,
      `docs/folio-js.md and docs/folio-js.html diverge at token ${i}\n  .md:   …${context(md)}…\n  .html: …${context(html)}…`,
    ).toBe(md.length)
    expect(html.length, 'docs/folio-js.html carries text the Markdown twin does not').toBe(md.length)
  })

  it('leaves nothing but page chrome outside the compared region', () => {
    // Without this, a paragraph written above twin:begin or below twin:end
    // would be published on one twin and invisible to the comparison above.
    const html = split(page)
    expect(html.prologue, 'docs/folio-js.html authors content above twin:begin').not.toMatch(/<(?:section|h1|h2|h3|h4|table|pre|ul|ol)\b/)
    expect(html.epilogue, 'docs/folio-js.html authors content below twin:end').not.toMatch(/<(?:section|h1|h2|h3|h4|table|pre|ul|ol)\b/)
    expect(html.epilogue.match(/<p\b/g) ?? [], 'docs/folio-js.html carries more than the source note below twin:end').toHaveLength(1)
    expect(html.epilogue).toContain('class="source-note"')

    const md = split(markdown)
    expect(md.prologue.trim(), 'docs/folio-js.md authors content above twin:begin').toBe('')
    expect(md.epilogue, 'docs/folio-js.md authors a heading or a code block below twin:end').not.toMatch(/^\s*(?:#|```)/m)
    expect(md.epilogue.trim().length, 'docs/folio-js.md authors prose below twin:end').toBeLessThan(200)
  })

  it('names every public item of the package in both twins', () => {
    const names = publicNames()
    // The scan must find a real surface: an empty or truncated one would let
    // every assertion below pass without reading a word of the guide.
    expect(names.length, `the entry-point scan found only ${names.length} exports: ${names.join(', ')}`).toBeGreaterThanOrEqual(16)
    for (const path of [markdown, page]) {
      const text = readerText(path)
      for (const name of names) {
        expect(new RegExp(`\\b${name}\\b`).test(text), `${path} does not document the public item ${name}`).toBe(true)
      }
    }
  })

  it('scans a surface that agrees with what the entry points really export', async () => {
    const names = new Set(publicNames())
    const index = (await import('../src/index.js')) as Record<string, unknown>
    const fonts = (await import('../src/fonts.js')) as Record<string, unknown>
    const runtime = [...Object.keys(index), ...Object.keys(fonts)]
    expect(runtime.length).toBeGreaterThan(0)
    for (const name of runtime) expect(names.has(name), `the source scan missed the runtime export ${name}`).toBe(true)
  })

  it('shows the first-PDF snippet the offline-install test executes', () => {
    const snippet = readmeSnippet()
    expect(snippet).toContain('await render(')
    expect(read(markdown), 'docs/folio-js.md does not carry README.md\'s tested first-PDF snippet verbatim').toContain(snippet)
    expect(firstPdfSample(), 'docs/folio-js.html\'s first-PDF sample is not README.md\'s tested snippet').toBe(snippet)
  })

  it('references no remote host, so it reads with the network off', () => {
    for (const path of [markdown, page]) {
      expect(read(path), `${path} references a remote host`).not.toMatch(/https?:\/\//)
    }
  })
})
