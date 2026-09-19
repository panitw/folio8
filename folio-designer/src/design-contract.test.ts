import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { designTokenSets } from './design-tokens'

const sourceDir = path.dirname(fileURLToPath(import.meta.url))
const designPath = path.resolve(sourceDir, '../../_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md')
const cssPath = path.resolve(sourceDir, 'tokens.css')
const appCssPath = path.resolve(sourceDir, 'App.css')
const packagePath = path.resolve(sourceDir, '../package.json')
const lockPath = path.resolve(sourceDir, '../package-lock.json')

function namesFromDesign(group: string) {
  const source = fs.readFileSync(designPath, 'utf8')
  const block = source.match(new RegExp(`^${group}:\\n([\\s\\S]*?)(?=^[a-z]+:|^---$)`, 'm'))?.[1] ?? ''
  return [...block.matchAll(/^  ['"]?([\w-]+)['"]?:/gm)].map((match) => match[1]).sort()
}

describe('DESIGN.md token contract', () => {
  it('has exact token-name equality with the independent design source', () => {
    for (const [group, implemented] of Object.entries(designTokenSets)) expect([...implemented].sort()).toEqual(namesFromDesign(group))
  })

  it('routes every authoritative colour value into the consumed CSS token source', () => {
    const source = fs.readFileSync(designPath, 'utf8')
    const css = fs.readFileSync(cssPath, 'utf8')
    const colors = source.match(/^colors:\n([\s\S]*?)(?=^tints:)/m)?.[1] ?? ''
    for (const [, name, value] of colors.matchAll(/^  ([\w-]+): '([^']+)'/gm)) {
      expect(css).toContain(`--color-${name}: ${value}`)
    }
    expect(css).toContain('--type-page-eyebrow:')
    expect(css).toContain('--type-page-fine:')
    expect(css).toContain("@import './generated/runtime-fonts.css'")
  })

  // STORY 16.3 — THE DECLARED ELEVATIONS ARE ROUTED FROM `DESIGN.md`, NOT TYPED.
  //
  // `DESIGN.md` declares THREE elevations and says "no fourth may be added".
  // `tokens.css` carried exactly ONE of them (`--shadow-page`), so a floating
  // surface had no shadow of its own and every sheet borrowed the page's. This
  // story mints `--shadow-sheet` by TRANSCRIBING the declared
  // `components.sheet.shadow` value — a design system's third elevation is not
  // "added" by being implemented; it was added when it was declared, and the
  // count stays three.
  //
  // IT IS ASSERTED FROM `DESIGN.md`'s OWN TEXT so the token cannot drift from the
  // declaration, the same discipline the colour routing above uses. The third
  // declared elevation (the page in preview) is still unimplemented and is
  // deliberately NOT asserted here — registering an absence as a passing test is
  // how it stops being visible.
  it('routes each implemented elevation from the design source into the CSS token', () => {
    const source = fs.readFileSync(designPath, 'utf8')
    const css = fs.readFileSync(cssPath, 'utf8')
    const sheetShadow = source.match(/^  sheet:\n(?:.*\n)*?    shadow: '([^']+)'/m)?.[1]
    const pageShadow = source.match(/^  page-surface:\n(?:.*\n)*?    shadow: '([^']+)'/m)?.[1]
    expect(sheetShadow, 'DESIGN.md declares components.sheet.shadow').toBeTruthy()
    expect(pageShadow, 'DESIGN.md declares components.page-surface.shadow').toBeTruthy()
    expect(css).toContain(`--shadow-sheet: ${sheetShadow}`)
    expect(css).toContain(`--shadow-page: ${pageShadow}`)
    // AND THE MINTED TOKEN IS USED, NOT MERELY DEFINED. Asserting only the
    // definition leaves the story's actual fix unguarded: reverting
    // `.font-browser`'s `box-shadow` to `var(--shadow-page)` puts the modal back
    // on the page's elevation while every assertion above stays green, because
    // an unused token is still a defined one. A token nothing reaches for is
    // indistinguishable from its own absence.
    const shellCss = fs.readFileSync(appCssPath, 'utf8')
    expect(shellCss, 'the floating sheet surface must take the sheet elevation, not the page\'s').toContain('box-shadow: var(--shadow-sheet)')
  })

  it('pins package, lockfile, and strict compiler metadata independently', () => {
    const pkg = JSON.parse(fs.readFileSync(packagePath, 'utf8'))
    const lock = JSON.parse(fs.readFileSync(lockPath, 'utf8'))
    const compiler = fs.readFileSync(path.resolve(sourceDir, '../tsconfig.app.json'), 'utf8')
    expect(pkg.engines.node).toBe('24.16.0')
    expect(pkg.dependencies.react).toBe('19.2.0')
    expect(pkg.dependencies['react-dom']).toBe('19.2.0')
    expect(pkg.devDependencies.vite).toBe('7.3.6')
    expect(lock.packages[''].dependencies.react).toBe('19.2.0')
    expect(lock.packages['node_modules/react'].version).toBe('19.2.0')
    expect(compiler).toMatch(/"strict":\s*true/)
  })

  it('keeps colour literals and curved radii inside the token definition only', () => {
    const shellCss = fs.readFileSync(appCssPath, 'utf8')
    const tokensCss = fs.readFileSync(cssPath, 'utf8')
    expect(shellCss).not.toMatch(/#[0-9a-f]{3,8}|\b(?:rgb|hsl)\(/i)
    expect(shellCss).not.toMatch(/border-radius:(?!\s*var\(--radius)/)
    expect(tokensCss).toContain('--radius-default: 0')
    expect(tokensCss).toContain('--radius-dot: 50%')
  })

  it('retains the accent grammar and permitted hard-stop page grid', () => {
    const shellCss = fs.readFileSync(appCssPath, 'utf8')
    expect(shellCss).toContain('outline: 2px solid var(--color-select)')
    expect(shellCss).toContain('background: var(--color-bind)')
    expect(shellCss).not.toMatch(/linear-gradient|conic-gradient/i)
    expect(shellCss).toContain('.page-surface')
    expect(shellCss).toContain('radial-gradient(var(--color-page-dot)')
    expect(shellCss).not.toMatch(/\.canvas-region[^}]*--color-page-/)
		expect(shellCss).toContain('.tree-item:focus-visible { outline: 2px solid var(--color-select); outline-offset: -2px; }')
  })

  it('reserves the solid danger card and square marker for a failed local render', () => {
    const shellCss = fs.readFileSync(appCssPath, 'utf8')
    expect(shellCss).toContain('.preview-failure { display: grid; grid-template-columns: auto minmax(0, 1fr) auto;')
    expect(shellCss).toContain('border: 1px solid var(--color-danger); border-left: 3px solid var(--color-danger);')
    expect(shellCss).toContain('.preview-failure-marker { color: var(--color-danger);')
    expect(shellCss).toContain('.preview-failure button:focus-visible { outline: 2px solid var(--color-select);')
  })

  // STORY 16.10 — THE FONT BROWSER'S HEADER IS ONE ROW, PINNED WHERE A GATE RUNS.
  //
  // Nothing that actually RUNS could see this before. `npm test` is vitest over
  // jsdom, which applies no stylesheet at all, and the one real measurement — the
  // 46px `boundingBox` in `e2e/font-browser.spec.ts` — is in the Playwright suite,
  // which no gate in this epic executes (D-000.4). Reverting `.font-browser-header`
  // to its two-row grid left `npm test`, `typecheck`, `oxlint` and `build` all
  // green, so the story's central claim had no running guard. This is that guard.
  //
  // IT READS THE EXTRACTED DECLARATION BLOCK, NEVER THE RAW FILE. The comment
  // standing above that rule in `App.css` contains both "grid" and "46px" in
  // prose, so a whole-file `toContain`/`not.toContain` pair would be satisfied by
  // the commentary rather than by the rule — a false guard of exactly the kind
  // this epic keeps finding. The same reason strips comments before the
  // orphaned-selector scan below.
  it('draws the font browser header as the design\'s single 46px flex row', () => {
    const shellCss = fs.readFileSync(appCssPath, 'utf8')
    const block = shellCss.match(/^\.font-browser-header\s*\{([^}]*)\}/m)?.[1]
    expect(block, 'App.css must declare a `.font-browser-header` rule').toBeTruthy()
    expect(block, 'the design draws one flex row').toContain('display: flex')
    expect(block, 'Font Browser.dc.html:293 fixes the row at 46px').toContain('height: 46px')
    expect(block, 'a grid here is the two-row header the disclosure paragraph needed').not.toContain('display: grid')

    // AND THE SELECTORS THE PARAGRAPH OWNED ARE GONE FROM THE FILE, not merely
    // unreferenced by the TSX. Comments are stripped first for the reason above:
    // the prose that explains the removal names both classes.
    const withoutComments = shellCss.replaceAll(/\/\*[\s\S]*?\*\//g, '')
    expect(withoutComments, 'the disclosure paragraph and its rule went together').not.toContain('.font-browser-disclosure')
    expect(withoutComments, 'with one row left there is no inner row wrapper').not.toContain('.font-browser-header-row')
    // POSITIVE CONTROL for the strip-and-scan: a sibling rule that DOES survive.
    expect(withoutComments, 'the scan must still see the rules that remain').toContain('.font-browser-title')
  })

  it('limits the display and large numeric exception to S1', () => {
    const shellCss = fs.readFileSync(appCssPath, 'utf8')
    expect(shellCss.match(/var\(--type-display\)/g)).toHaveLength(1)
    expect(shellCss.match(/var\(--type-numeric-lg\)/g)).toHaveLength(1)
    expect(shellCss).toContain('.load-column h1')
    expect(shellCss).toContain('.load-numeric')
  })

  it('keeps actual shell foreground/background pairings above the usability floor', () => {
    const tokensCss = fs.readFileSync(cssPath, 'utf8')
    const channel = (name: string) => {
      const value = tokensCss.match(new RegExp(`--color-${name}: #(\\w{6})`))?.[1]
      if (!value) throw new Error(`missing colour token ${name}`)
      return value.match(/\w\w/g)!.map((part) => Number.parseInt(part, 16) / 255)
    }
    const luminance = (name: string) => channel(name).map((value) => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0)
    const contrast = (foreground: string, background: string) => (Math.max(luminance(foreground), luminance(background)) + 0.05) / (Math.min(luminance(foreground), luminance(background)) + 0.05)
    const shellCss = fs.readFileSync(appCssPath, 'utf8')
    expect(shellCss).toContain('background: var(--color-panel)')
    expect(shellCss).toContain('background-color: var(--color-page)')
    for (const [foreground, background] of [['ink', 'panel'], ['ink-high', 'raised'], ['page-ink', 'page'], ['page-ink-body', 'page'], ['page-ink-muted', 'page']]) {
      expect(contrast(foreground, background)).toBeGreaterThanOrEqual(4.5)
    }
  })
})
