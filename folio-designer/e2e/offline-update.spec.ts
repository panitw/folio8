import { readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'
// @ts-expect-error The source module is executed by Vitest and Node; this
// browser-only spec is compile-covered through the Epic 5 boundary.
import { serviceWorkerSource } from '../scripts/offline-service-worker-template.mjs'

const usableOfflineState = /^(Offline ready|Update available; current release remains usable)$/

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const builtWorker = join(root, 'dist', 'sw.js')

async function currentRelease() {
  return JSON.parse(await readFile(join(root, 'dist', 'offline-release-manifest.json'), 'utf8')) as { id: string; pageId: string; workerRevision: string; assets: { url: string }[] }
}

function workerOnlyRevision(release: Awaited<ReturnType<typeof currentRelease>>) {
  return { ...release, id: 'f'.repeat(64), pageId: 'e'.repeat(64), workerRevision: 'd'.repeat(64) }
}

async function withBuiltWorker(source: string, exercise: () => Promise<void>) {
  const original = await readFile(builtWorker)
  await writeFile(builtWorker, source)
  try { await exercise() } finally { await writeFile(builtWorker, original) }
}

// These are intentionally executable Playwright lifecycle proofs. Execution
// is deferred by D-000.4 until Epic 5 closes; local Chromium absence is not a
// substitute for the unit-level lifecycle proofs in this story.
test('a complete worker-only update waits behind an old tab and preserves its cache', async ({ page }) => {
  await openWorkspace(page)
  await page.reload()
  await expect(page.getByTestId('offline-status')).toHaveText(usableOfflineState)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const oldRelease = await currentRelease()
  const oldCache = `folio8-release-${oldRelease.id}`
  await withBuiltWorker(serviceWorkerSource(workerOnlyRevision(oldRelease)), async () => {
    await page.evaluate(async () => { await (await navigator.serviceWorker.getRegistration())?.update() })
    await expect.poll(() => page.evaluate(async () => Boolean((await navigator.serviceWorker.getRegistration())?.waiting))).toBe(true)
    expect(await page.evaluate(async (name) => (await caches.keys()).includes(name), oldCache)).toBe(true)
    await expect(page.getByTestId('offline-status')).toHaveText(usableOfflineState)
  })
})

test('a failed replacement keeps the old complete release usable', async ({ page }) => {
  await openWorkspace(page)
  await page.reload()
  await expect(page.getByTestId('offline-status')).toHaveText(usableOfflineState)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const oldRelease = await currentRelease()
  const oldCache = `folio8-release-${oldRelease.id}`
  const doomed = oldRelease.assets.find((asset) => asset.url.endsWith('.wasm'))!
  const doomedPath = join(root, 'dist', doomed.url.slice(1))
  const originalAsset = await readFile(doomedPath)
  await writeFile(doomedPath, 'install failure')
  try {
    await withBuiltWorker(serviceWorkerSource(workerOnlyRevision(oldRelease)), async () => {
      await page.evaluate(async () => { await (await navigator.serviceWorker.getRegistration())?.update() })
      await expect.poll(() => page.evaluate(async (name) => (await caches.keys()).includes(name), oldCache)).toBe(true)
      await expect.poll(() => page.evaluate(async () => Boolean((await navigator.serviceWorker.getRegistration())?.waiting))).toBe(false)
      await expect(page.getByTestId('offline-status')).toHaveText(usableOfflineState)
    })
  } finally { await writeFile(doomedPath, originalAsset) }
})
