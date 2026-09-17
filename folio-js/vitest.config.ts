import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    include: ['test/**/*.test.ts'],
    // Golden renders run the whole engine in wasm; the multi-page and CJK
    // fixtures take seconds, not milliseconds.
    testTimeout: 120_000,
  },
})
