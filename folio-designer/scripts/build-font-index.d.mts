// Types for `build-font-index.mjs`, so `src/font-index.test.ts` can drive the
// REAL emit step rather than a copy of it — the same idiom
// `forbidden-font-hosts.d.mts` sets. The script itself is plain ESM because
// `npm run build` executes it with `node` before anything TypeScript-aware
// exists in the pipeline.
export interface FontIndexFamily { readonly family: string; readonly category: string; readonly subsets: ReadonlyArray<string>; readonly axes: ReadonlyArray<string>; readonly styles: ReadonlyArray<string>; readonly popularity: number }
export interface FontIndexSnapshot { readonly snapshotDate: string; readonly snapshotSource: string; readonly publishedFamilies: number; readonly excludedCjkFamilies: number; readonly families: ReadonlyArray<FontIndexFamily> }

export const snapshotPath: string
export function trimFamily(entry: Record<string, unknown>): FontIndexFamily
export function trimSnapshot(published: Record<string, unknown>, today: string): FontIndexSnapshot
export function readSnapshot(): FontIndexSnapshot
export function emitFontIndexModule(): { readonly families: number; readonly snapshotDate: string }
export function refreshSnapshot(today?: string): Promise<FontIndexSnapshot>
