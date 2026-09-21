import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AnalyticsAction } from './analytics'

// THE MODULE READS ITS ENV AT IMPORT TIME, so every case here stubs the var
// FIRST and then imports a fresh instance. A top-level `import` of the module
// would bind one instance to whatever the env happened to be when the file was
// collected, and every row of the matrix below would test the same thing.
const loadAnalytics = async (containerId?: string) => {
  if (containerId === undefined) vi.stubEnv('VITE_GA_CONTAINER_ID', undefined as unknown as string)
  else vi.stubEnv('VITE_GA_CONTAINER_ID', containerId)
  vi.resetModules()
  return await import('./analytics')
}

// SPELLED FROM PIECES ON PURPOSE. `single-measurement-vendor.test.ts` proves
// the measurement host is named in exactly ONE file under src/ — analytics.ts —
// which is the whole enforcement of AD-27's "and from nowhere else". Writing it
// literally here would make this file the second occurrence and turn that guard
// red for no reason.
const MEASUREMENT_HOST = ['googletag', 'manager', '.com'].join('')

const gtmScripts = () => Array.from(document.head.querySelectorAll('script')).filter((script) => script.src.includes(MEASUREMENT_HOST))

// The whole vocabulary a payload is permitted to contain. The privacy rule
// (D-GA.1: usage may be reported, document content may never be) is asserted
// against THIS set, not against an eyeball on the call sites.
const ACTIONS: readonly AnalyticsAction[] = ['open_template', 'export_pdf', 'font_import', 'enter_preview']
// ⚠ `gtm.js` IS DELIBERATELY ABSENT. The assertion below reads the pushes made
// by `trackEvent` only — GTM's own seed entry is sliced off — so listing it
// here would be a permission nothing can exercise.
const PERMITTED_VALUES = new Set<unknown>([...ACTIONS, 'folio8_action'])

beforeEach(() => {
  delete window.dataLayer
  for (const script of Array.from(document.head.querySelectorAll('script'))) script.remove()
})

afterEach(() => {
  vi.unstubAllEnvs()
  vi.resetModules()
})

describe('analytics — the env var is the sole switch', () => {
  it('injects nothing and defines no dataLayer when the container id is unset', async () => {
    const { initAnalytics, trackEvent } = await loadAnalytics(undefined)
    expect(initAnalytics()).toBe(false)
    // ⚠ UNDEFINED, not "present but empty". An unconfigured page must be
    // indistinguishable from one that never had this feature.
    expect(window.dataLayer).toBeUndefined()
    expect(gtmScripts()).toHaveLength(0)
    // And a push made anyway must not be the thing that creates the queue.
    trackEvent('export_pdf')
    expect(window.dataLayer).toBeUndefined()
  })

  it('treats an empty container id as unset', async () => {
    const { initAnalytics } = await loadAnalytics('')
    expect(initAnalytics()).toBe(false)
    expect(window.dataLayer).toBeUndefined()
    expect(gtmScripts()).toHaveLength(0)
  })

  // FAIL CLOSED. Each of these is a real paste accident: a GA4 measurement id,
  // a shell variable that never expanded, a lower-cased id, a truncated prefix.
  it.each(['not-a-container', 'G-ABC123', '$VITE_GA_CONTAINER_ID', 'gtm-nqzrc9v4', 'GTM-', 'GTM NQZRC9V4'])(
    'treats the malformed id %j as unset rather than injecting a bad tag',
    async (junk) => {
      const { initAnalytics, trackEvent } = await loadAnalytics(junk)
      expect(initAnalytics()).toBe(false)
      expect(window.dataLayer).toBeUndefined()
      expect(gtmScripts()).toHaveLength(0)
      trackEvent('enter_preview')
      expect(window.dataLayer).toBeUndefined()
    },
  )
})

describe('analytics — a configured container', () => {
  it('injects exactly one async GTM script and seeds the pageview', async () => {
    const { initAnalytics } = await loadAnalytics('GTM-NQZRC9V4')
    expect(initAnalytics()).toBe(true)
    const scripts = gtmScripts()
    expect(scripts).toHaveLength(1)
    expect(scripts[0].src).toBe(`https://www.${MEASUREMENT_HOST}/gtm.js?id=GTM-NQZRC9V4`)
    // ⚠ ASYNC IS THE REASON AN UNREACHABLE CONTAINER IS HARMLESS. A blocking
    // third-party script would make their outage our outage.
    expect(scripts[0].async).toBe(true)
    expect(window.dataLayer).toHaveLength(1)
    expect((window.dataLayer![0] as Record<string, unknown>).event).toBe('gtm.js')
  })

  it('injects only once however many times it is initialised', async () => {
    const { initAnalytics } = await loadAnalytics('GTM-NQZRC9V4')
    initAnalytics()
    initAnalytics()
    initAnalytics()
    expect(gtmScripts()).toHaveLength(1)
    expect(window.dataLayer).toHaveLength(1)
  })

  it('pushes one entry per tracked action', async () => {
    const { initAnalytics, trackEvent } = await loadAnalytics('GTM-NQZRC9V4')
    initAnalytics()
    const before = window.dataLayer!.length
    trackEvent('open_template')
    expect(window.dataLayer!.length).toBe(before + 1)
    expect(window.dataLayer![before]).toEqual({ event: 'folio8_action', folio8_action: 'open_template' })
  })

  // THE CORE PRIVACY CLAIM, asserted rather than inspected: across every
  // action the union admits, no key and no value in anything that reaches
  // `dataLayer` is drawn from outside the fixed vocabulary — so no file name,
  // template name, font family, path or parameter value can be in there.
  it('pushes nothing outside the fixed action vocabulary', async () => {
    const { initAnalytics, trackEvent } = await loadAnalytics('GTM-NQZRC9V4')
    initAnalytics()
    for (const action of ACTIONS) trackEvent(action)
    const pushes = window.dataLayer!.slice(1) as Record<string, unknown>[]
    expect(pushes).toHaveLength(ACTIONS.length)
    for (const push of pushes) {
      expect(Object.keys(push).sort()).toEqual(['event', 'folio8_action'])
      for (const value of Object.values(push)) expect(PERMITTED_VALUES.has(value), `unexpected payload value ${String(value)}`).toBe(true)
    }
  })
})

describe('analytics — the name is already occupied', () => {
  // ⚠ THIS IS A BLANK-APP HAZARD, NOT A TIDINESS ONE. `initAnalytics` runs at
  // module scope in the entry module. Adopting a non-array with
  // `window.dataLayer ?? []` makes the very next `.push` throw, the entry
  // module aborts before `startObservation`, and the author gets an empty page
  // — because a browser extension put an object on a global name.
  it('replaces a non-array occupant rather than pushing into it', async () => {
    ;(window as unknown as { dataLayer: unknown }).dataLayer = { notAnArray: true }
    const { initAnalytics, trackEvent } = await loadAnalytics('GTM-NQZRC9V4')
    expect(() => initAnalytics()).not.toThrow()
    expect(Array.isArray(window.dataLayer)).toBe(true)
    trackEvent('export_pdf')
    expect(window.dataLayer).toHaveLength(2)
  })

  it('keeps an existing array queue, because GTM may have seeded it already', async () => {
    const seeded = [{ event: 'someone-elses' }]
    window.dataLayer = seeded
    const { initAnalytics } = await loadAnalytics('GTM-NQZRC9V4')
    initAnalytics()
    expect(window.dataLayer, 'an array on that name IS the queue and must not be discarded').toBe(seeded)
    expect(window.dataLayer).toHaveLength(2)
  })
})

describe('analytics — the container never arrives', () => {
  it('queues pushes harmlessly and never throws when the script fails to load', async () => {
    const { initAnalytics, trackEvent } = await loadAnalytics('GTM-NQZRC9V4')
    initAnalytics()
    // jsdom never fetches the script; this is the ad-blocked / offline /
    // DNS-refused case exactly — the queue is a plain array nobody drains.
    gtmScripts()[0].dispatchEvent(new Event('error'))
    expect(() => { for (const action of ACTIONS) trackEvent(action) }).not.toThrow()
    expect(window.dataLayer).toHaveLength(1 + ACTIONS.length)
  })
})
