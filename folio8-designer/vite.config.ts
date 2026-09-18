import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'
import { createHash } from 'node:crypto'
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

const generatedRuntime = join(import.meta.dirname, 'src', 'generated', 'runtime')
const directoryFingerprint = (directory: string) => existsSync(directory) ? createHash('sha256').update(readdirSync(directory).sort().map((file) => `${file}:${createHash('sha256').update(readFileSync(join(directory, file))).digest('hex')}`).join('\n')).digest('hex').slice(0, 20) : 'missing-runtime-input'
const cMapDirectory = `pdfjs-cmaps-${directoryFingerprint(join(generatedRuntime, 'pdfjs-cmaps'))}`
const standardFontDirectory = `pdfjs-standard-fonts-${directoryFingerprint(join(generatedRuntime, 'pdfjs-standard_fonts'))}`

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    assetsInlineLimit: 0,
    rollupOptions: {
      output: {
        // PDF.js appends known filenames to its configured local bases. These
        // two directories are content-addressed as collections, so names stay
        // functional without giving up immutable release URLs.
        assetFileNames: (asset) => {
          if (asset.name?.endsWith('.bcmap')) return `assets/${cMapDirectory}/[name][extname]`
          if (asset.name?.startsWith('LiberationSans-') && asset.name.endsWith('.ttf')) return `assets/${standardFontDirectory}/[name][extname]`
          // The bundled documentation pages are already content-addressed by
          // build-wasm.mjs, and they link to each other by those exact names,
          // so a second hash here would break every cross-page link.
          if (/^(?:rendering-library|folio-js|folio-dotnet|folio-format|expression-reference)-[a-f0-9]{20}\.html$/.test(asset.name ?? '')) return 'assets/[name][extname]'
          return 'assets/[name]-[hash][extname]'
        },
      },
    },
  },
  // Runtime inputs are copied through Vite's asset graph by build-wasm.mjs.
  // Never publish the mutable source files from public/ as application URLs.
  publicDir: false,
  test: { environment: 'jsdom', setupFiles: ['./src/test/setup.ts'], include: ['src/**/*.test.{ts,tsx}', 'scripts/**/*.test.mjs'] },
})
