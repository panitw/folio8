// Every suite renders with the package's own shipped fonts. vitest runs this
// once per TEST FILE, in that file's own module registry, so the ~14 MB set is
// read once per file rather than once per run — which is what lets helpers.ts
// expose it synchronously to call sites that are not in async position.
import { loadShippedFonts } from './helpers.js'

await loadShippedFonts()
