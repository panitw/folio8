// Types for `forbidden-font-hosts.mjs`, so `src/forbidden-font-hosts.test.ts`
// can drive the REAL scanner rather than a copy of it. The scanner itself is
// plain ESM because `npm run build` executes it with `node` before anything
// TypeScript-aware exists in the pipeline — the same reason every other file
// under `scripts/` is `.mjs`.
export interface ForbiddenFontHost { readonly host: string; readonly declaration: string }
export interface FontHostFinding { readonly file: string; readonly host: string; readonly half: string; readonly line: number; readonly text: string }
// `tracked` and `untracked` are Story 15.2b's split of the widened population,
// mirrored here BY HAND because nothing checks this sidecar against the `.mjs`:
// `allowJs` is off and no tsconfig includes `scripts/`, so this file IS the
// module to the compiler. A missing export is `TS2305` at the importer; a wrong
// signature is accepted silently and fails at runtime. `files === tracked + untracked`.
export interface FontHostScanResult { readonly files: number; readonly tracked: number; readonly untracked: number; readonly floor: number; readonly findings: ReadonlyArray<FontHostFinding> }

export const FORBIDDEN_FONT_HOSTS: ReadonlyArray<ForbiddenFontHost>
export const DECLARED_ONLY_FONT_HOSTS: ReadonlyArray<ForbiddenFontHost>
export const SCANNED_FONT_HOSTS: ReadonlyArray<ForbiddenFontHost & { readonly half: string }>
export const DECLARATION_MARKER: string
export const SCANNED_ROOTS: ReadonlyArray<string>
export const SCANNED_EXTENSIONS: ReadonlyArray<string>
export const POPULATION_FLOOR: number

export function blankComments(source: string, extension?: string): string
export function exemptLineNumbers(source: string, host: string, extension?: string): Set<number>
export function occurrencesIn(source: string, extension?: string): ReadonlyArray<Omit<FontHostFinding, 'file'>>
export function scannedPopulation(root: string): ReadonlyArray<string>
export function scanForbiddenFontHosts(root: string, options?: { floor?: number }): FontHostScanResult
export function assertNoForbiddenFontHosts(root: string, options?: { floor?: number }): FontHostScanResult
