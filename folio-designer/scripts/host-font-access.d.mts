// Types for `host-font-access.mjs`, so `src/host-font-access.test.ts` can drive
// the REAL scanner rather than a copy of it. The scanner itself is plain ESM
// for the same reason every other file under `scripts/` is: it must be runnable
// by `node` with nothing TypeScript-aware in the pipeline.
export interface HostFontAccessApi { readonly name: string; readonly pattern: RegExp; readonly declaration: string }
export interface HostFontFinding { readonly file: string; readonly api: string; readonly line: number; readonly text: string }
// `tracked` and `untracked` are Story 15.2b's split of the widened population,
// mirrored here BY HAND — nothing checks this sidecar against the `.mjs`.
// `files === tracked + untracked`.
export interface HostFontScanResult { readonly files: number; readonly tracked: number; readonly untracked: number; readonly floor: number; readonly findings: ReadonlyArray<HostFontFinding> }

export const HOST_FONT_ACCESS_APIS: ReadonlyArray<HostFontAccessApi>
export const HOST_FONT_DECLARATION_MARKER: string
export const SCANNED_ROOTS: ReadonlyArray<string>
export const SCANNED_EXTENSIONS: ReadonlyArray<string>
export const POPULATION_FLOOR: number

export function blankComments(source: string, extension?: string): string
export function exemptLineNumbers(source: string, pattern: RegExp, extension?: string): Set<number>
export function occurrencesIn(source: string, extension?: string): ReadonlyArray<Omit<HostFontFinding, 'file'>>
export function scannedPopulation(root: string): ReadonlyArray<string>
export function scanHostFontAccess(root: string, options?: { floor?: number }): HostFontScanResult
export function assertNoHostFontAccess(root: string, options?: { floor?: number }): HostFontScanResult
