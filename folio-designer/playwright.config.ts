import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  workers: 1,
  timeout: 90_000,
  expect: { timeout: 30_000 },
  // DW-297. Until 2026-09-08 this config named NO reporter and NO trace, so
  // `playwright-report/` was never created -- not in CI and not here. The CI
  // job's `if: failure()` step has been uploading a directory that does not
  // exist, with `if-no-files-found: ignore` turning that into silence: five red
  // runs, zero artefacts, while the step's own comment claimed a red run's
  // traces are "the difference between a reproducible defect and 'it went red
  // on CI once'". The reasoning was right; nothing implemented it.
  //
  // `open: 'never'` because CI must not try to launch a browser at the end of a
  // run, and neither should a scripted local run. `list` is kept alongside so
  // the console output a human reads is unchanged.
  reporter: [['html', { open: 'never' }], ['list']],
  use: {
    baseURL: 'http://127.0.0.1:4173',
    // retain-on-failure, not `on`: a green run's traces are storage cost for
    // evidence nobody reads, and a red run's are the whole point. There are no
    // retries configured, so `on-first-retry` would capture nothing at all --
    // the obvious-looking setting is the one that would reproduce the defect
    // being fixed here.
    trace: 'retain-on-failure',
    launchOptions: {
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    },
  },
  // This command is not "start a server" -- it is a full cold production
  // build first, and `npm run build` runs build:wasm, which compiles the Go
  // engine to wasm. That is 141s on a warm developer laptop. On a bare
  // ubuntu-24.04 runner with no Go module or build cache (no setup-go step in
  // this workflow sets cache-dependency-path, so none of them cache), it is
  // slower still -- and at 180_000 it timed out on every push, failing the
  // folio-designer-e2e job four runs running with
  // `Timed out waiting 180000ms from config.webServer`.
  //
  // 600_000 is a budget, not a wait: a server that comes up in 200s costs
  // 200s. It stays well inside the job's 45-minute cap, so a build that
  // genuinely hangs still fails the job rather than the workflow.
  webServer: {
    command: 'npm run build && npm run preview -- --host 127.0.0.1 --port 4173',
    url: 'http://127.0.0.1:4173',
    reuseExistingServer: false,
    timeout: 600_000,
  },
})
