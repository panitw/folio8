import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { cutsBesideTheRegular, familyDirectorySlug, fetchTimeoutMs, fetchWebFamily, fontHostDeclarations, parseFamilyMetadata, probeDirectories, publishedCuts, timedFetcher } from './font-source'
import { admittedByTheTokenTable } from './font-licence'
import { sfntWithNames } from './test/sfnt-fixture'
import { assertProvenanceShape } from './test/provenance-shape'
import { blankComments } from '../scripts/forbidden-font-hosts.mjs'

// STORY 16.1 — THE FETCH PATH AND ITS ADMISSION DECISION.
//
// EVERY TEST HERE DRIVES A STUB FETCHER, NOT THE NETWORK. What is being asserted
// is this module's own decisions — which directory it derives, what it accepts as
// confirmation, what it refuses and in which words, and above all THE ORDER:
// classify, then embed. A test that reached upstream would assert Google's
// current state rather than this module's rules, and would go red for reasons
// that are nobody's defect.

const here = path.dirname(fileURLToPath(import.meta.url))
// THE HOST IS SPELLED HERE WITH THE SCANNER'S MARKER, IN CODE, on this one
// line — the same shape `FORBIDDEN_FONT_HOSTS` uses. A marker in a comment
// would declare nothing: the scan reads RAW source while the exemption reads
// COMMENT-BLANKED source, deliberately.
const base = { url: 'https://raw.githubusercontent.com/google/fonts/main', declaration: 'folio8:font-host-declaration' }.url

const kanitMetadata = `name: "Kanit"
designer: "Cadson Demak"
license: "OFL"
category: "SANS_SERIF"
date_added: "2015-11-04"
fonts {
  name: "Kanit"
  style: "normal"
  weight: 100
  filename: "Kanit-Thin.ttf"
  post_script_name: "Kanit-Thin"
}
fonts {
  name: "Kanit"
  style: "italic"
  weight: 400
  filename: "Kanit-Italic.ttf"
}
fonts {
  name: "Kanit"
  style: "normal"
  weight: 400
  filename: "Kanit-Regular.ttf"
  post_script_name: "Kanit-Regular"
}
subsets: "latin"
source {
  repository_url: "https://github.com/cadsondemak/Kanit"
  files {
    source_file: "Kanit-Regular.ttf"
    dest_file: "Kanit-Regular.ttf"
  }
}
`

const face = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors' }])
// A DIFFERENT BYTE SEQUENCE FOR THE ITALIC, so a cut resolving to the wrong
// face is visible rather than hidden behind two identical fixtures.
const italicFace = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors (italic)' }])

type StubFile = Readonly<{ status?: number; body?: string | ArrayBuffer }>

/** A fetcher over a fixed map of paths, recording every URL it was asked for. */
function stub(files: Readonly<Record<string, StubFile>>) {
  const asked: string[] = []
  const fetcher = async (url: string) => {
    asked.push(url)
    const file = Object.hasOwn(files, url) ? files[url] : undefined
    if (file === undefined) return { ok: false, status: 404, text: async () => '', arrayBuffer: async () => new ArrayBuffer(0) } as unknown as Response
    const status = file.status ?? 200
    return {
      ok: status >= 200 && status < 300,
      status,
      text: async () => (typeof file.body === 'string' ? file.body : ''),
      arrayBuffer: async () => (typeof file.body === 'string' ? new ArrayBuffer(0) : file.body ?? new ArrayBuffer(0)),
    } as unknown as Response
  }
  return { fetcher, asked }
}

// THE FIXTURE SERVES EVERY CUT ITS OWN METADATA PUBLISHES, which since
// spec-install-all-face-cuts story 1 is two: the upright 400 and the italic
// 400. A fixture that published an italic and refused to serve it would make
// every success-path request count a measurement of a 404 rather than of the
// chain.
const kanitUpstream = (overrides: Readonly<Record<string, StubFile>> = {}) => ({
  [`${base}/ofl/kanit/METADATA.pb`]: { body: kanitMetadata },
  [`${base}/ofl/kanit/OFL.txt`]: { body: 'Copyright 2020 The Kanit Project Authors\n\nThis Font Software is licensed under the SIL Open Font License, Version 1.1.\n' },
  [`${base}/ofl/kanit/Kanit-Regular.ttf`]: { body: face },
  [`${base}/ofl/kanit/Kanit-Italic.ttf`]: { body: italicFace },
  ...overrides,
})

describe('the family directory slug', () => {
  // D-16.R.6, verified 8 of 8 on deliberately awkward families. These are the
  // measured pairs, not invented ones: digits survive, spaces and punctuation
  // do not, and nothing is inserted.
  it('lowercases the family name and deletes every character outside [a-z0-9]', () => {
    for (const [family, slug] of [
      ['Press Start 2P', 'pressstart2p'],
      ['Baloo Bhai 2', 'baloobhai2'],
      ['Alegreya SC', 'alegreyasc'],
      ['Source Serif 4', 'sourceserif4'],
      ['DM Sans', 'dmsans'],
      ['Ma Shan Zheng', 'mashanzheng'],
      ['Playpen Sans Thai', 'playpensansthai'],
      ['Noto Sans Thai Looped', 'notosansthailooped'],
    ] as const) expect(familyDirectorySlug(family), family).toBe(slug)
  })

  it('deletes rather than transliterates, so a name with nothing to keep produces nothing', () => {
    expect(familyDirectorySlug('Röboto')).toBe('rboto')
    expect(familyDirectorySlug('— —')).toBe('')
  })

  it('probes the four upstream directories, in the order the ruling names', () => {
    expect([...probeDirectories]).toEqual(['ofl', 'apache', 'ufl', 'cc-by-sa'])
  })
})

describe('reading METADATA.pb', () => {
  // DEPTH IS THE WHOLE DIFFICULTY. `name:` appears at the top level (the
  // family) AND inside every `fonts { … }` block (the face) AND inside nested
  // `source { files { … } }` blocks. A flat scan would confirm the directory
  // against the wrong string, which is the exact failure the confirmation
  // exists to prevent.
  it('reads the FAMILY name from the top level and never a face name or a nested one', () => {
    const metadata = parseFamilyMetadata(kanitMetadata)
    expect(metadata?.name).toBe('Kanit')
    expect(metadata?.licence).toBe('OFL')
    expect(metadata?.faces).toHaveLength(3)
  })

  it('reads each cut\'s filename from its own style/weight entry rather than constructing one', () => {
    const metadata = parseFamilyMetadata(kanitMetadata)
    // THE CUT SET, IN RIBBI ORDER AND READ OFF THE FILE. Kanit's fixture
    // publishes an upright 400 and an italic 400 and no 700 of either, so the
    // set is two entries — absence is a first-class answer and nothing is
    // invented for the two it does not publish.
    expect(publishedCuts(metadata!)).toEqual([
      { cut: 'Regular', filename: 'Kanit-Regular.ttf' },
      { cut: 'Italic', filename: 'Kanit-Italic.ttf' },
    ])
    // AND THE REGULAR IS NOT THE FIRST ENTRY, NOR THE ONE THE FAMILY NAME WOULD
    // SUGGEST IF THE FILES WERE NAMED DIFFERENTLY: the Thin comes first in the
    // file and the italic 400 sits between them.
    expect(metadata!.faces[0].filename).toBe('Kanit-Thin.ttf')
    // NO WEIGHT OUTSIDE THE FOUR CUTS, EITHER. The Thin at weight 100 is a real
    // face upstream and there is nowhere in this format to put it, so it is
    // read and dropped rather than mapped onto a cut it is not.
    expect(publishedCuts(metadata!).some((cut) => cut.filename === 'Kanit-Thin.ttf')).toBe(false)
  })

  it('reads all four cuts when upstream publishes all four, and no fifth', () => {
    const full = parseFamilyMetadata(`name: "Full"
license: "OFL"
fonts {
  style: "normal"
  weight: 400
  filename: "Full-Regular.ttf"
}
fonts {
  style: "normal"
  weight: 700
  filename: "Full-Bold.ttf"
}
fonts {
  style: "italic"
  weight: 400
  filename: "Full-Italic.ttf"
}
fonts {
  style: "italic"
  weight: 700
  filename: "Full-BoldItalic.ttf"
}
fonts {
  style: "normal"
  weight: 500
  filename: "Full-Medium.ttf"
}
`)
    expect(publishedCuts(full!)).toEqual([
      { cut: 'Regular', filename: 'Full-Regular.ttf' },
      { cut: 'Bold', filename: 'Full-Bold.ttf' },
      { cut: 'Italic', filename: 'Full-Italic.ttf' },
      { cut: 'Bold Italic', filename: 'Full-BoldItalic.ttf' },
    ])
  })

  it('has no Regular to offer when upstream declares no upright 400', () => {
    const variableOnly = parseFamilyMetadata('name: "Anuphan"\nlicense: "OFL"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: ""\n}\n')
    expect(publishedCuts(variableOnly!).some((cut) => cut.cut === 'Regular')).toBe(false)
  })

  it('returns nothing at all for a file that declares neither a name nor a licence', () => {
    expect(parseFamilyMetadata('# a comment\n')).toBeUndefined()
  })

  // A MALFORMED FILE MAY FAIL TO RESOLVE; IT MAY NEVER RESOLVE TO THE WRONG
  // STRING. One stray `}` used to drive the depth counter negative, and the next
  // `{` returned it to zero WITHOUT opening a block — so the `name:` inside a
  // `fonts { … }` entry, which upstream blocks really do carry, was read as the
  // FAMILY name and then confirmed against. That is exactly the confusion the
  // name-equality confirmation exists to prevent, arriving through the parser.
  it('never reads a nested face name as the family name, however unbalanced the braces are', () => {
    const unbalanced = `license: "OFL"
source {
  files {
    dest_file: "Kanit-Thin.ttf"
  }
}
}
fonts {
  name: "Kanit Thin"
  style: "normal"
  weight: 400
  filename: "Kanit-Thin.ttf"
}
`
    expect(parseFamilyMetadata(unbalanced), 'an unbalanced file may fail to resolve, never resolve to a face name').toBeUndefined()
  })
})

describe('fetching a family from the web tier', () => {
  it('resolves a static family end to end and hands the command exactly what it requires', async () => {
    const { fetcher, asked } = stub(kanitUpstream())
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(true)
    if (!outcome.ok) return
    // THE WHOLE CUT SET, IN RIBBI ORDER. The fixture publishes an upright 400
    // and an italic 400, so both arrive and each carries its OWN record.
    expect(outcome.published).toEqual(['Regular', 'Italic'])
    expect(outcome.refused).toEqual([])
    expect(outcome.faces.map((cut) => cut.style)).toEqual(['Regular', 'Italic'])
    const regular = outcome.faces[0]!
    expect(regular.family).toBe('Kanit')
    expect(regular.style).toBe('Regular')
    // THE SPDX ID, NEVER THE UPSTREAM TOKEN. This value is literally Go's input
    // at component_commands.go's RefuseContradictedLicence call.
    expect(regular.licence).toBe('OFL-1.1')
    // THE UPSTREAM FILE, NEVER A HAND-COPY — a hand-copy would be a second
    // authority on the terms.
    expect(regular.licenceText).toContain('SIL Open Font License')
    // nameID 0 FROM THE FACE'S OWN BYTES, never from METADATA.pb.
    expect(regular.copyright).toBe('Copyright 2020 The Kanit Project Authors')
    expect(regular.mediaType).toBe('font/ttf')
    expect(regular.source).toContain('ofl/kanit/Kanit-Regular.ttf')
    expect(regular.layoutDivergence).toBeUndefined()
    // AND THE ITALIC IS DESCRIBED FROM ITS OWN BYTES AND ITS OWN PATH, never
    // copied off the Regular: a cut set whose records all named one file would
    // put four rows in the store pointing at one face.
    const italic = outcome.faces[1]!
    expect(italic.style).toBe('Italic')
    expect(italic.source).toContain('ofl/kanit/Kanit-Italic.ttf')
    expect(italic.copyright).toBe('Copyright 2020 The Kanit Project Authors (italic)')
    // The licence record travels with EVERY cut, because the store refuses a
    // face without it and each cut is its own record.
    expect(italic.licence).toBe('OFL-1.1')
    expect(italic.licenceText).toContain('SIL Open Font License')
    expect(italic.mediaType).toBe('font/ttf')
    // PROBING IS ONCE PER PICK: one metadata read, one licence file, then one
    // request per cut the family publishes.
    expect(asked).toHaveLength(4)
    expect(asked[0]).toContain('/ofl/kanit/METADATA.pb')
    expect(asked.filter((url) => url.endsWith('METADATA.pb'))).toHaveLength(1)
    expect(asked.filter((url) => url.endsWith('OFL.txt'))).toHaveLength(1)
  })

  // D-3 — A RE-PICK OF A FAMILY THIS MACHINE HOLDS A SHORT SET FOR FETCHES ONLY
  // WHAT IT LACKS.
  //
  // The Regular is the expensive one (median 107 KB, p99 1.7 MB) and it is
  // already here; refetching it to get the italic would make a repair cost more
  // than the original install. A cut named in `held` is absent from `faces` AND
  // from `refused`, because it was never attempted — which is the third state
  // the census has to be able to express.
  it('fetches only the cuts this machine lacks, and never refetches one it holds', async () => {
    const { fetcher, asked } = stub(kanitUpstream())
    const outcome = await fetchWebFamily('Kanit', fetcher, '2026-09-20', ['Regular'])
    expect(outcome.ok).toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces.map((cut) => cut.style), 'only the missing cut is fetched').toEqual(['Italic'])
    expect(outcome.published, 'what upstream publishes does not change because this machine holds some of it').toEqual(['Regular', 'Italic'])
    expect(outcome.refused, 'a cut that was never attempted is not a refused one').toEqual([])
    expect(asked.some((url) => url.endsWith('Kanit-Regular.ttf')), 'the held Regular must not be refetched').toBe(false)
    // The metadata and the licence file are still read: the metadata is what
    // says which cuts are missing, and the licence text travels with every new
    // record.
    expect(asked).toHaveLength(3)
  })

  // AND A FAMILY THIS MACHINE ALREADY HOLDS IN FULL FETCHES NO BYTES AND NO
  // TERMS — the migration path, and the common case rather than a corner. 947
  // of the 1,270 offered families publish a Regular and nothing else, so every
  // one of them installed before this story is exactly this shape: everything
  // it publishes is held, and the only thing missing is the census.
  it('reads the metadata and stops when this machine already holds every cut', async () => {
    const { fetcher, asked } = stub(kanitUpstream())
    const outcome = await fetchWebFamily('Kanit', fetcher, '2026-09-20', ['Regular', 'Italic'])
    expect(outcome.ok).toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces).toEqual([])
    expect(outcome.refused).toEqual([])
    // WHAT UPSTREAM PUBLISHES IS STILL ANSWERED, because that is the whole
    // reason the call was made.
    expect(outcome.published).toEqual(['Regular', 'Italic'])
    // ONE REQUEST: the metadata. The licence text travels with a NEW RECORD and
    // no record will be written, so reading it would buy nothing.
    expect(asked).toHaveLength(1)
    expect(asked[0]).toContain('/ofl/kanit/METADATA.pb')
  })

  // `cutsBesideTheRegular` IS THE BROWSE PATH'S BOUND, AND IT IS A REAL ONE.
  // The font browser sets a specimen in the family's REGULAR, and the re-embed
  // refetch replaces one dropped record in a document that carries the Regular
  // alone. Both pass this, and both would otherwise cost up to four body reads
  // per family on a page of twelve rows.
  it('fetches the Regular alone when the caller asks to skip every other cut', async () => {
    const { fetcher, asked } = stub(kanitUpstream())
    const outcome = await fetchWebFamily('Kanit', fetcher, '2026-09-20', cutsBesideTheRegular)
    expect(outcome.ok).toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces.map((cut) => cut.style)).toEqual(['Regular'])
    expect(asked.some((url) => url.endsWith('Kanit-Italic.ttf')), 'a specimen needs no italic').toBe(false)
    expect(asked).toHaveLength(3)
    // AND A SKIPPED CUT IS NOT A REFUSED ONE: nothing was attempted, so nothing
    // is recorded against it.
    expect(outcome.refused).toEqual([])
    expect(outcome.published, 'what upstream publishes is still answered in full').toEqual(['Regular', 'Italic'])
  })

  // THE PER-CUT REFUSALS, WHICH ARE THE WHOLE POINT OF DOING THIS PER CUT. One
  // bad cut is refused on its own evidence and the family installs around it.
  //
  // AND EACH REFUSAL SAYS WHETHER IT IS SETTLED (D-2, amended). `permanence` is
  // derived HERE, from the failure itself — the status, the abort, the `fvar`
  // table — because only this module sees any of those. The classification is
  // asserted per row rather than in one lump, because the whole point of the
  // amendment is that these cases are NOT alike.
  const badCut: ReadonlyArray<readonly [string, Readonly<Record<string, StubFile>>, RegExp, 'permanent' | 'transient']> = [
    ['a variable cut', { [`${base}/ofl/kanit/Kanit-Italic.ttf`]: { body: sfntWithNames([{ platform: 3, nameID: 0, value: 'c' }], { withFvar: true }) } }, /VARIABLE font/, 'permanent'],
    // 404 IS UPSTREAM STATING WHAT IT PUBLISHES; 500 IS THE HOST HAVING A BAD
    // MINUTE. Recording them alike is what stranded a Bold upstream really has.
    ['a cut upstream does not serve at all', { [`${base}/ofl/kanit/Kanit-Italic.ttf`]: { status: 404 } }, /responded 404/, 'permanent'],
    ['a cut that responds 500', { [`${base}/ofl/kanit/Kanit-Italic.ttf`]: { status: 500 } }, /responded 500/, 'transient'],
    // A 403 FROM A PROXY IS A CAPTIVE PORTAL, NOT A WITHDRAWN FILE — the
    // default rule, and the reason it points at `transient`.
    ['a cut a proxy refuses', { [`${base}/ofl/kanit/Kanit-Italic.ttf`]: { status: 403 } }, /responded 403/, 'transient'],
    // AND A BODY THAT WILL NOT PARSE, which is what a captive portal's 200 HTML
    // login page looks like from here.
    ['a cut whose body is not a font', { [`${base}/ofl/kanit/Kanit-Italic.ttf`]: { body: new TextEncoder().encode('<!doctype html><title>Sign in</title>').buffer as ArrayBuffer } }, /not a static TrueType sfnt/, 'transient'],
  ]
  it.each(badCut)('installs the family without %s, and records why and whether it is settled', async (_case, overrides, reason, permanence) => {
    const { fetcher } = stub(kanitUpstream(overrides))
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok, 'one bad cut may not poison the family').toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces.map((cut) => cut.style)).toEqual(['Regular'])
    expect(outcome.published, 'what upstream publishes is unchanged by this machine failing to get it').toEqual(['Regular', 'Italic'])
    expect(outcome.refused.map((entry) => entry.style)).toEqual(['Italic'])
    expect(outcome.refused[0]!.reason).toMatch(reason)
    expect(outcome.refused[0]!.permanence, 'the classification is derived from the failure, not from the sentence').toBe(permanence)
  })

  // AN OFFLINE CONNECTION IS THE OTHER TRANSIENT SHAPE, and it reaches the same
  // catch by a different door: the fetcher REJECTS rather than aborting.
  it('records a cut lost to a dead connection as transient, not as something upstream lacks', async () => {
    const inner = stub(kanitUpstream())
    const fetcher = async (url: string) => {
      if (url.endsWith('Kanit-Italic.ttf')) throw new TypeError('Failed to fetch')
      return inner.fetcher(url)
    }
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces.map((cut) => cut.style)).toEqual(['Regular'])
    expect(outcome.refused[0]!.permanence, 'a dead connection says nothing about what upstream publishes').toBe('transient')
  })

  // AND THE MEDIA-TYPE ROUTE, WHICH IS SHUT PER CUT AS WELL AS ON THE BASE. A
  // `.woff2` italic costs no request at all — the filename alone refuses it —
  // so this also asserts that the refusal happens before the fetch.
  it('installs the family without a cut published in a format the engine cannot read', async () => {
    const woff2 = kanitMetadata.replace('filename: "Kanit-Italic.ttf"', 'filename: "Kanit-Italic.woff2"')
    const { fetcher, asked } = stub(kanitUpstream({ [`${base}/ofl/kanit/METADATA.pb`]: { body: woff2 } }))
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces.map((cut) => cut.style)).toEqual(['Regular'])
    expect(outcome.refused.map((entry) => entry.style)).toEqual(['Italic'])
    expect(outcome.refused[0]!.reason).toMatch(/not a font file this engine reads/)
    expect(outcome.refused[0]!.permanence, 'the filename upstream publishes will not become a .ttf by asking again').toBe('permanent')
    expect(asked.some((url) => url.includes('woff2')), 'a filename this engine cannot read costs no round-trip').toBe(false)
  })

  // A STALLED CUT IS THE SAME SHAPE, AND IT DOES NOT TERMINATE THE CHAIN. The
  // base chain's abort contract stops at the Regular; past it, a stall is one
  // cut's own failure.
  it('installs the family without a cut whose body stalls, and does not abort the family', async () => {
    const asked: string[] = []
    const inner = stub(kanitUpstream())
    const fetcher = async (url: string) => {
      asked.push(url)
      if (url.endsWith('Kanit-Italic.ttf')) throw new DOMException('The operation was aborted due to timeout', 'TimeoutError')
      return inner.fetcher(url)
    }
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok, 'a stalled non-base cut may not refuse the family').toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces.map((cut) => cut.style)).toEqual(['Regular'])
    expect(outcome.refused.map((entry) => entry.style)).toEqual(['Italic'])
    expect(outcome.refused[0]!.reason).toMatch(/stopped responding/)
    // ⚠ AND IT IS TRANSIENT, WHICH IS WHAT MAKES THE SENTENCE IT CARRIES TRUE.
    // That reason ends "Try the pick again if you like"; recorded as settled,
    // the family would read complete and never be offered again, so the pick
    // the sentence invites could not be made. This is the assertion that reds
    // against the first cut of this story.
    expect(outcome.refused[0]!.reason).toMatch(/Try the pick again if you like/)
    expect(outcome.refused[0]!.permanence).toBe('transient')
    // THE CHAIN RAN TO THE END OF THE CUT SET RATHER THAN STOPPING, and the
    // stalled request was not retried.
    expect(asked).toHaveLength(4)
    expect(new Set(asked).size).toBe(asked.length)
  })

  // THE PROVENANCE SHAPE, ON THE REAL WRITE PATH AND WITH NO DATE SUPPLIED
  // (D-16.R.13, DW-160).
  //
  // `font-provenance.test.ts` asserts the same predicate over `webFaceSource`
  // called DIRECTLY, with an explicit date — which cannot observe the default
  // `today` expression at `fetchWebFamily`'s signature, the only place a fetched
  // pick's date actually comes from. MEASURED, and this case exists because of
  // it: deleting the `.slice(0, 10)` from that default makes every fetched pick
  // publish `…, fetched 2026-09-03T…Z`, and the whole suite stayed green.
  //
  // So this one calls `fetchWebFamily` with NO `today` argument, takes the
  // source off the OUTCOME rather than off the helper, and applies the shared
  // predicate — which requires the field to END in `, fetched YYYY-MM-DD`. A
  // timestamp fails that, and so would a scheme, a host, a moving ref or a
  // duplicated digest reaching the field by any future route.
  it('writes a scheme-free, host-free provenance string on the path a real pick takes', async () => {
    const { fetcher } = stub(kanitUpstream())
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok, 'the fixture must resolve, or the assertions below run on nothing').toBe(true)
    if (!outcome.ok) return
    // EVERY CUT'S PROVENANCE, not only the Regular's: the store writes one
    // record per cut and each carries its own `source`, so a second cut whose
    // field was built any other way would be invisible to a one-face check.
    for (const cut of outcome.faces) assertProvenanceShape(expect, 'fetched tier', `the ${cut.style} fetched with no \`today\` argument`, cut.source)
    const regular = outcome.faces[0]!
    // AND IT IS TODAY'S DATE, not a stale constant: the default is evaluated at
    // the call, so the recorded date must be the one this machine is running on.
    expect(regular.source).toContain(`, fetched ${new Date().toISOString().slice(0, 10)}`)
    expect(regular.source).toContain('ofl/kanit/Kanit-Regular.ttf')
  })

  it('walks the probe order and stops at the directory that answers', async () => {
    // THE FILENAMES ARE THE SHAPE UPSTREAM ACTUALLY PUBLISHES: `google/fonts`
    // names its files after the family with the spaces removed, so Roboto Slab's
    // Regular is `RobotoSlab-Regular.ttf`. The rename below is done BEFORE the
    // family rename, so no fixture URL carries an unencoded space — a shape no
    // real fetch produces and one this module has never been asked to handle.
    const apache = kanitMetadata.replace('license: "OFL"', 'license: "APACHE2"').replace(/Kanit-/g, 'RobotoSlab-').replace(/Kanit/g, 'Roboto Slab')
    const { fetcher, asked } = stub({
      [`${base}/apache/robotoslab/METADATA.pb`]: { body: apache },
      [`${base}/apache/robotoslab/LICENSE.txt`]: { body: 'Apache License, Version 2.0' },
      [`${base}/apache/robotoslab/RobotoSlab-Regular.ttf`]: { body: face },
    })
    const outcome = await fetchWebFamily('Roboto Slab', fetcher)
    expect(outcome.ok).toBe(true)
    // AND NOTHING THIS MODULE BUILT CARRIES A SPACE: the directory is the slug
    // and the filename is READ, so a family whose display name has a space still
    // produces a URL a fetch can be made with.
    for (const url of asked) expect(url, `${url} must be a URL a fetch can be made with`).not.toContain(' ')
    expect(asked[0]).toContain('/ofl/robotoslab/METADATA.pb')
    expect(asked[1]).toContain('/apache/robotoslab/METADATA.pb')
    // AND IT DID NOT KEEP PROBING once a directory answered.
    expect(asked.filter((url) => url.endsWith('METADATA.pb'))).toHaveLength(2)
  })

  // D-16.R.6: THE DIRECTORY IS DERIVED THEN CONFIRMED. A mismatch is a refusal,
  // never a fallback to the next directory — continuing past a disagreement
  // would turn "derived then confirmed" back into the guess it replaces.
  it('refuses when METADATA.pb names a different family, and does not try the next directory', async () => {
    const { fetcher, asked } = stub({ [`${base}/ofl/geist/METADATA.pb`]: { body: 'name: "Geist Mono"\nlicense: "OFL"\n' } })
    const outcome = await fetchWebFamily('Geist', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toContain('Geist Mono')
    expect(outcome.reason).toMatch(/refused rather than guessed past/)
    expect(asked).toHaveLength(1)
  })

  // METADATA.pb ALWAYS WINS ON LICENCE; THE PROBE RESULT IS NEVER EVIDENCE OF
  // TERMS. Upstream moves families between directories — `apache/roboto` now
  // 404s and Roboto lives in `ofl/` — so reading layout as a licence assertion
  // would let a family that moved silently change the terms a document
  // publishes.
  it('admits a family whose directory disagrees with its token, and RECORDS the divergence', async () => {
    const { fetcher } = stub({
      [`${base}/ofl/movedfamily/METADATA.pb`]: { body: 'name: "Moved Family"\nlicense: "APACHE2"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: "MovedFamily-Regular.ttf"\n}\n' },
      [`${base}/ofl/movedfamily/LICENSE.txt`]: { body: 'Apache License, Version 2.0' },
      [`${base}/ofl/movedfamily/MovedFamily-Regular.ttf`]: { body: face },
    })
    const outcome = await fetchWebFamily('Moved Family', fetcher)
    expect(outcome.ok, 'a layout disagreement is an observation, not a refusal').toBe(true)
    if (!outcome.ok) return
    expect(outcome.faces[0]!.licence).toBe('Apache-2.0')
    expect(outcome.faces[0]!.layoutDivergence).toContain('ofl/')
    expect(outcome.faces[0]!.layoutDivergence).toContain('APACHE2')
  })

  it('refuses a mapped-but-unacceptable licence by name, with its reason, before any byte is fetched', async () => {
    const { fetcher, asked } = stub({
      [`${base}/ofl/sharealike/METADATA.pb`]: { body: 'name: "ShareAlike"\nlicense: "CC-BY-SA"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: "ShareAlike-Regular.ttf"\n}\n' },
      [`${base}/ofl/sharealike/ShareAlike-Regular.ttf`]: { body: face },
    })
    const outcome = await fetchWebFamily('ShareAlike', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.classification?.state).toBe('refused')
    expect(outcome.reason).toContain('CC-BY-SA')
    // CLASSIFY, THEN EMBED. The face was never fetched.
    expect(asked.some((url) => url.endsWith('.ttf'))).toBe(false)
  })

  it('refuses an unrecognised token as NOT RECOGNISED rather than as forbidden, before any byte is fetched', async () => {
    const { fetcher, asked } = stub({
      [`${base}/ofl/fifthdirectory/METADATA.pb`]: { body: 'name: "Fifth Directory"\nlicense: "SOMETHINGNEW"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: "FifthDirectory-Regular.ttf"\n}\n' },
      [`${base}/ofl/fifthdirectory/FifthDirectory-Regular.ttf`]: { body: face },
    })
    const outcome = await fetchWebFamily('Fifth Directory', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.classification?.state).toBe('unrecognised')
    expect(outcome.reason).toMatch(/does not recognise/)
    expect(outcome.reason).toMatch(/not the same as being forbidden/)
    expect(asked.some((url) => url.endsWith('.ttf'))).toBe(false)
  })

  it('refuses a family that publishes no upright static Regular', async () => {
    const { fetcher } = stub({ [`${base}/ofl/anuphan/METADATA.pb`]: { body: 'name: "Anuphan"\nlicense: "OFL"\nfonts {\n  style: "normal"\n  weight: 200\n  filename: "Anuphan-ExtraLight.ttf"\n}\n' } })
    const outcome = await fetchWebFamily('Anuphan', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/no upright Regular/)
  })

  // THE `woff2` ROUTE, SHUT AT THE FILE LEVEL AS WELL AS AT THE HOST LEVEL. The
  // engine's accepted media types are exactly font/ttf and font/otf.
  it('refuses a Regular published in a format the engine does not read', async () => {
    const { fetcher } = stub({ [`${base}/ofl/subsetted/METADATA.pb`]: { body: 'name: "Subsetted"\nlicense: "OFL"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: "Subsetted-Regular.woff2"\n}\n' } })
    const outcome = await fetchWebFamily('Subsetted', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/not a font file this engine reads/)
  })

  // TWO CASES, TWO TESTS, REPORTED INDEPENDENTLY. Written as one loop the
  // narrowing `if (outcome.ok) return` exits the whole TEST rather than the
  // iteration, so a regression in the first case silently skips the second and
  // one red hides the other. `it.each` is what makes each case its own result.
  const missingLicenceFile: ReadonlyArray<readonly [string, Readonly<Record<string, StubFile>>]> = [
    ['absent upstream', { [`${base}/ofl/kanit/OFL.txt`]: { status: 404 } }],
    ['present but blank', { [`${base}/ofl/kanit/OFL.txt`]: { body: '   \n' } }],
  ]
  it.each(missingLicenceFile)('refuses a family whose licence file is %s, stating why', async (_case, overrides) => {
    const { fetcher, asked } = stub(kanitUpstream(overrides))
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/publishes no OFL\.txt/)
    expect(outcome.reason).toMatch(/may not carry a face without the text of its licence/)
    // AND THE BYTES WERE NEVER FETCHED: a face may not reach the document
    // before its terms are in hand.
    expect(asked.some((url) => url.endsWith('.ttf'))).toBe(false)
  })

  // THE LICENCE FILE IS NAMED BY THE DECLARED TERMS, NOT BY THE DIRECTORY, so a
  // family sitting in `ofl/` under Apache terms is asked for LICENSE.txt.
  it('asks for the licence file the declared terms name, not the one the directory suggests', async () => {
    const { fetcher, asked } = stub({
      [`${base}/ofl/movedfamily/METADATA.pb`]: { body: 'name: "Moved Family"\nlicense: "APACHE2"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: "MovedFamily-Regular.ttf"\n}\n' },
    })
    await fetchWebFamily('Moved Family', fetcher)
    expect(asked.some((url) => url.endsWith('/ofl/movedfamily/LICENSE.txt'))).toBe(true)
    expect(asked.some((url) => url.endsWith('/ofl/movedfamily/OFL.txt'))).toBe(false)
  })

  // THE CONTAINER IS CHECKED BEFORE THE WALK, AND THIS IS THE UNTRUSTED CALLER
  // THAT MAKES THAT MATTER. These bytes arrived from a third party seconds
  // earlier: an `OTTO`/CFF or WOFF wrapper has a table directory at the same
  // offsets meaning something else, and a 200 carrying an error page has none at
  // all. Both are refused in the version's own words rather than walked into a
  // plausible-looking copyright.
  it('refuses a fetched body that is not a static TrueType container, rather than walking it', async () => {
    const notAFont = new TextEncoder().encode('<!doctype html><title>404: Not Found</title>').buffer as ArrayBuffer
    for (const body of [sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright someone else' }], { sfntVersion: 0x4f54544f }), notAFont]) {
      const { fetcher } = stub(kanitUpstream({ [`${base}/ofl/kanit/Kanit-Regular.ttf`]: { body } }))
      const outcome = await fetchWebFamily('Kanit', fetcher)
      expect(outcome.ok).toBe(false)
      if (outcome.ok) continue
      expect(outcome.reason).toMatch(/not a static TrueType sfnt/)
    }
  })

  // THE LICENCE-FILE MAP HOLDS EXACTLY THE IDS THE TOKEN TABLE CAN EMIT. Not
  // D-8.5.3's four: `font-licence.ts` deliberately has no MIT row — absence, not
  // narrowing — so a MIT row here would be dead code, and the mapping a future
  // MIT token would silently inherit without anybody reviewing it. This walks
  // every admitted id and asserts a real filename was asked for; a `.../undefined`
  // is what an unguarded lookup would produce.
  it('asks for a real licence file for every id the token table can emit, and holds no row it cannot', async () => {
    expect(admittedByTheTokenTable.length, 'a new admitted id must bring its licence file name with it').toBe(3)
    for (const [token, file] of [['OFL', 'OFL.txt'], ['APACHE2', 'LICENSE.txt'], ['UFL', 'UFL.txt']] as const) {
      const { fetcher, asked } = stub({
        [`${base}/ofl/afamily/METADATA.pb`]: { body: `name: "A Family"\nlicense: "${token}"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: "AFamily-Regular.ttf"\n}\n` },
      })
      await fetchWebFamily('A Family', fetcher)
      expect(asked.some((url) => url.endsWith(`/ofl/afamily/${file}`)), `${token} must ask for ${file}`).toBe(true)
      for (const url of asked) expect(url, `${token} produced a malformed URL`).not.toContain('undefined')
    }
  })

  it('refuses a face that carries no copyright in its own name table', async () => {
    const { fetcher } = stub(kanitUpstream({ [`${base}/ofl/kanit/Kanit-Regular.ttf`]: { body: sfntWithNames([], { omitNameTable: true }) } }))
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/declares no copyright in its own `name` table \(nameID 0\)/)
  })

  // OFFLINE DEGRADES, NEVER BREAKS. No network means no NEW family; it never
  // means a document that will not render.
  // MECHANICAL (Story 16.5, finding 10): the verb follows the action. This
  // resolver runs on the INSTALL path only, so "cannot be added" described a
  // step that no longer happens here.
  it('states that a family cannot be installed right now when the network is down', async () => {
    const outcome = await fetchWebFamily('Kanit', async () => { throw new TypeError('Failed to fetch') })
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/You cannot install a family without a network connection/)
    expect(outcome.reason, 'the machine tier is still offered, and using one needs no network').toMatch(/needs no network at all/)
    expect(outcome.reason).toMatch(/faces this machine already holds are still offered/)
  })

  // A STALE SNAPSHOT IS A NAMED, ACTIONABLE REFUSAL. The list ships with the
  // designer and ages between releases, so it can name a family upstream has
  // since renamed or withdrawn — and the message says exactly that rather than
  // reporting a bare 404.
  it('says the snapshot is stale when every probe 404s', async () => {
    const { fetcher, asked } = stub({})
    const outcome = await fetchWebFamily('Withdrawn Family', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/no longer published upstream/)
    expect(outcome.reason).toMatch(/ages between releases/)
    expect(asked).toHaveLength(probeDirectories.length)
  })
})

describe('the declared hosts', () => {
  const source = fs.readFileSync(path.join(here, 'font-source.ts'), 'utf8')

  // THE MARKER MUST BE REAL CODE ON THE LINE NAMING THE HOST. The scan computes
  // its exemption over COMMENT-BLANKED source, so a marker written in a comment
  // declares nothing — see forbidden-font-hosts.mjs's blankComments.
  it('declares both hosts in code, on the line that names them', () => {
    expect(fontHostDeclarations.map((entry) => entry.host), 'folio8:font-host-declaration').toEqual(['raw.githubusercontent.com', 'fonts.google.com'])
    for (const entry of fontHostDeclarations) {
      expect(entry.declaration).toBe('folio8:font-host-declaration')
      const line = source.split('\n').find((text) => text.includes(`'${entry.host}'`) && !text.trimStart().startsWith('//'))
      expect(line, `${entry.host} must be spelled on a line that also carries the marker, in code`).toContain('folio8:font-host-declaration')
    }
  })

  // THE `woff2` ROUTE ASSERTED ABSENT FROM SOURCE. The stylesheet endpoint
  // returns woff2 under a modern browser UA — which the engine refuses by design
  // — split by `unicode-range` into per-script subsets, which would embed partial
  // coverage into a document naming the whole family. The full TTF that endpoint
  // serves to a legacy UA is unreachable, because a browser cannot set
  // `User-Agent`.
  it('reaches for no stylesheet endpoint, no woff2 and no unicode-range subset', () => {
    // OVER COMMENT-BLANKED SOURCE, using the scanner's own blanker. This module
    // has to NAME the refused route to explain why it is refused, and a check
    // that could not tell an explanation from a call site would either fire on
    // the prose or be silently satisfied by deleting it.
    const code = blankComments(source, '.ts')
    expect(code, 'the blanker must not have eaten the file').toContain('fetchWebFamily')
    for (const forbidden of ['css2', 'woff2', 'unicode-range', 'googleapis', 'gstatic']) {
      expect(code, `font-source.ts must not reach for ${forbidden}`).not.toMatch(new RegExp(forbidden, 'i'))
    }
  })
})

// ─────────────────────────────────────────────────────────────────────────────
// STORY 16.2 — THE FETCH TIMEOUT (D-16.R.14, D-16.R.42; discharges DW-165).
//
// A fetch that REJECTS degrades with a stated message and releases the pick's
// hold. A fetch that STALLS never settles, so the hold is never released and
// the font control is dead for the rest of the session with no message at all.
// That is the worst member of the failure class this module's matrix covers and
// the one it did not cover.
describe('a fetch that stalls rather than rejecting', () => {
  const timeoutError = () => new DOMException('The operation was aborted due to timeout', 'TimeoutError')

  /** A fetcher that serves `files` until call index `abortAt`, where it aborts instead. */
  function stubAbortingAt(files: Readonly<Record<string, StubFile>>, abortAt: number) {
    const inner = stub(files)
    const asked: string[] = []
    const fetcher = async (url: string) => {
      asked.push(url)
      if (asked.length - 1 === abortAt) throw timeoutError()
      return inner.fetcher(url)
    }
    return { fetcher, asked }
  }

  // THE NUMBER, AND THE ARITHMETIC BEHIND IT. A timeout is a number and a
  // number needs a basis; a constant whose stated reason its own arithmetic
  // contradicts is the defect D-16.R.25 names. Both halves are pinned: the
  // value, and the derivation written beside it.
  it('is 30 seconds, and the constant carries the arithmetic rather than prose', () => {
    expect(fetchTimeoutMs).toBe(30_000)
    const source = fs.readFileSync(path.join(here, 'font-source.ts'), 'utf8')
    // The measured maximum, the factor, the product, and the bound it clears.
    expect(source, 'the constant must carry the measured maximum it was sized against').toContain('2,097 ms')
    expect(source, 'the constant must carry its arithmetic, not a claim about it').toContain('2,097 x 10 = 20,970 <= 30,000')
    // The largest offerable face, which is what makes 646 KB the wrong
    // denominator: the budget serves the FETCHABLE population, not the
    // committed one.
    expect(source).toContain('24,271,604')
    // AND THE SAMPLE'S OWN LIMIT, which is the honest reason the factor is x10
    // and not x2.
    expect(source.replace(/\n\s*\*\s*/g, ' ')).toMatch(/One connection, one day, five repetitions/)
  })

  // THE SIGNAL GOES INTO `fetch()` ITSELF, so the abort reaches the BODY
  // stream. The bytes are read by `response.arrayBuffer()` AFTER the fetcher
  // returns, so a timeout armed around the fetcher alone would leave the worst
  // real stall — a 24 MB body that stops arriving — completely uncovered.
  it('passes an abort signal into fetch, rather than arming a timer around it', async () => {
    const seen: Array<RequestInit | undefined> = []
    const restore = globalThis.fetch
    globalThis.fetch = (async (_url: string, init?: RequestInit) => { seen.push(init); return { ok: false, status: 404, text: async () => '' } as unknown as Response }) as never
    try {
      await fetchWebFamily('Kanit')
      expect(seen.length, 'the default fetcher must have been used').toBeGreaterThan(0)
      for (const init of seen) {
        expect(init?.signal, 'every request must carry the abort signal, so the abort reaches the body stream').toBeInstanceOf(AbortSignal)
        expect(init?.signal?.aborted, 'the signal must still be live at the moment of the request').toBe(false)
      }
    } finally {
      globalThis.fetch = restore
    }
  })

  // AND THE TIMEOUT REALLY FIRES — the whole mechanism, end to end, at a real
  // short budget. `AbortSignal.timeout` runs on the platform's own timer, which
  // a fake clock does not reach, so this is the only honest shape for this
  // claim: a never-settling `fetch` that resolves only when the signal aborts.
  //
  // THERE IS NO DISARM PATH, which is why `AbortSignal.timeout` is chosen over
  // a hand-armed setTimeout/clearTimeout: nothing can clear it when the headers
  // arrive, so it still covers the body.
  it('fires on its own, with no disarm, and the abort reaches the caller as a refusal', async () => {
    const restore = globalThis.fetch
    globalThis.fetch = ((_url: string, init?: RequestInit) => new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener('abort', () => reject(init.signal?.reason ?? timeoutError()))
    })) as never
    try {
      const outcome = await fetchWebFamily('Kanit', timedFetcher(5))
      expect(outcome.ok).toBe(false)
      if (outcome.ok) return
      expect(outcome.reason).toMatch(/stopped responding/)
    } finally {
      globalThis.fetch = restore
    }
  })

  // THE STALL'S SENTENCE IS ITS OWN, AND IT DELIBERATELY DOES NOT BORROW THE
  // OFFLINE ONE. "You cannot install a family without a network connection" is
  // FALSE when the network is up and the host is hanging, and it sends the
  // author to check their wifi over a problem that is not theirs.
  //
  // AND IT MUST NOT FAIL IN THE OTHER DIRECTION EITHER, which is the half that
  // is easy to miss. An earlier wording said "your network is reachable — the
  // font host is not answering". A timeout knows NEITHER of those things: the
  // same abort fires when the network is down and packets are blackholed rather
  // than refused (the captive portal this story cites as its own trigger), when
  // DNS hangs, and on a link too slow to move a 24 MB face in 30 s. Both
  // assertions below are therefore negative, and they point in opposite
  // directions on purpose.
  it('states a stall as a stall — neither as the offline refusal nor as a claim the network is fine', async () => {
    const { fetcher } = stubAbortingAt(kanitUpstream(), 0)
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toContain('Kanit')
    expect(outcome.reason).toMatch(/waited 30 seconds/)
    expect(outcome.reason, 'a stall must not be reported as being offline').not.toMatch(/without a network connection/)
    // THE OTHER DIRECTION. A timeout cannot diagnose the network, and must not
    // pretend to: saying the link is fine is as false as saying it is down.
    expect(outcome.reason, 'a timeout cannot know the network is reachable, so it must not say so').not.toMatch(/network is reachable|connection is fine|you are online/i)
    // WHAT IT DOES KNOW, AND ALL IT KNOWS: a request was made and did not
    // finish inside the budget.
    expect(outcome.reason).toMatch(/did not complete in time/)
    expect(outcome.reason, 'the three things a timeout cannot distinguish are named rather than collapsed into one of them').toMatch(/cannot tell/)
    // AND IT SAYS NOTHING WAS RETRIED. A retry over a deterministic stall hides
    // it, so this designer does not retry — and says so, because "it gave up
    // after 30 seconds" reads as not trying hard enough unless the decision is
    // visible.
    expect(outcome.reason).toMatch(/nothing was retried automatically/)
    // AND WHAT STILL WORKS. A degradation the author cannot act on is an error
    // message wearing a friendlier coat.
    expect(outcome.reason).toMatch(/faces this machine already holds are still offered/)
  })

  // THE STALL DURING `arrayBuffer()`, WHICH IS THE ONE THAT MATTERS MOST. This
  // is the case a header-only timeout would miss entirely: the request answers,
  // and then the 24 MB body stops arriving.
  it('states the same degradation when the stall is in the body rather than the request', async () => {
    const asked: string[] = []
    const fetcher = async (url: string) => {
      asked.push(url)
      if (url.endsWith('/ofl/kanit/METADATA.pb')) return { ok: true, status: 200, text: async () => kanitMetadata } as unknown as Response
      if (url.endsWith('/ofl/kanit/OFL.txt')) return { ok: true, status: 200, text: async () => 'SIL Open Font License' } as unknown as Response
      // THE HEADERS ARRIVE. The body never does.
      return { ok: true, status: 200, arrayBuffer: async () => { throw timeoutError() } } as unknown as Response
    }
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/stopped responding while sending the Kanit-Regular\.ttf face itself/)
    expect(outcome.reason).not.toMatch(/without a network connection/)
    expect(asked).toHaveLength(3)
  })

  // A STALL READING THE LICENCE IS A STALL, NOT A MISSING LICENCE FILE. Before
  // the timeout existed that catch could only mean "not there"; reporting a
  // stall as "publishes no OFL.txt" would send the author upstream to look for
  // a file that is sitting there.
  it('does not report a stalled licence read as a family that publishes no licence file', async () => {
    const { fetcher } = stubAbortingAt(kanitUpstream(), 1)
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(false)
    if (outcome.ok) return
    expect(outcome.reason).toMatch(/stopped responding while sending the text of its licence/)
    expect(outcome.reason).not.toMatch(/publishes no OFL\.txt/)
  })

  // THE CHAIN TERMINATES ON THE FIRST ABORT, ASSERTED RATHER THAN ASSUMED.
  //
  // A deferral with a trigger would be the wrong instrument for a property that
  // can simply be asserted: a note ages, an assertion reds. The worst-case hold
  // is therefore T plus the requests that already completed — NOT the six-times-T
  // an unterminated chain would produce.
  //
  // RED-PROOF: change any one of the three catches to `continue` (or to fall
  // through rather than return) and the matching row reds on its call count.
  it('ends the whole chain at the first abort, wherever in it the abort lands', async () => {
    // THE EXPECTED COUNTS ARE LITERALS, never computed from the code path that
    // drives the fetches — a count derived from the implementation would agree
    // with any implementation.
    const rows: ReadonlyArray<readonly [string, string, Readonly<Record<string, StubFile>>, number, number]> = [
      ['the first probe', 'Kanit', kanitUpstream(), 0, 1],
      ['the licence read', 'Kanit', kanitUpstream(), 1, 2],
      ['the byte read', 'Kanit', kanitUpstream(), 2, 3],
    ]
    for (const [where, family, files, abortAt, expectedCalls] of rows) {
      const { fetcher, asked } = stubAbortingAt(files, abortAt)
      const outcome = await fetchWebFamily(family, fetcher)
      expect(outcome.ok, where).toBe(false)
      if (!outcome.ok) expect(outcome.reason, where).toMatch(/stopped responding/)
      // NOT ONE REQUEST MORE. A chain that continued past the abort would show
      // up here as a larger number, and a silent retry would show up as the
      // aborted URL appearing twice.
      expect(asked, `${where}: the chain must stop at the abort and make no further request`).toHaveLength(expectedCalls)
      expect(new Set(asked).size, `${where}: no request may be retried`).toBe(asked.length)
    }
  })

  // AND A 404 IS THE ONE THING THE PROBE LOOP CONTINUES PAST — which an abort
  // never produces. This row is what makes the claim above about the LOOP and
  // not merely about its first iteration.
  it('continues past a 404 and still ends at the first abort inside the loop', async () => {
    const apacheMetadata = kanitMetadata.replace('license: "OFL"', 'license: "APACHE2"')
    const files = {
      [`${base}/apache/kanit/METADATA.pb`]: { body: apacheMetadata },
      [`${base}/apache/kanit/LICENSE.txt`]: { body: 'Apache License, Version 2.0' },
      [`${base}/apache/kanit/Kanit-Regular.ttf`]: { body: face },
    }
    // Call 0 is the `ofl` probe, which 404s and is continued past. Call 1 is
    // the `apache` probe, which aborts.
    const { fetcher, asked } = stubAbortingAt(files, 1)
    const outcome = await fetchWebFamily('Kanit', fetcher)
    expect(outcome.ok).toBe(false)
    if (!outcome.ok) expect(outcome.reason).toMatch(/stopped responding while looking for its upstream metadata/)
    expect(asked).toHaveLength(2)
    expect(asked[0]).toContain('/ofl/kanit/METADATA.pb')
    expect(asked[1]).toContain('/apache/kanit/METADATA.pb')
  })

  // NO SILENT RETRY, WITH THE COUNT AS A LITERAL. A retry over a deterministic
  // stall hides it, which is Story 16.0's `Never:` clause and the same reason.
  it('makes exactly one request per step and never repeats the one that stalled', async () => {
    const { fetcher, asked } = stubAbortingAt(kanitUpstream(), 2)
    await fetchWebFamily('Kanit', fetcher)
    expect(asked).toHaveLength(3)
    expect(asked.filter((url) => url.endsWith('Kanit-Regular.ttf'))).toHaveLength(1)
    // AND THE SUCCESSFUL CHAIN IS THE SAME THREE REQUESTS PLUS ONE PER
    // ADDITIONAL CUT — four here, because the Kanit fixture publishes an
    // italic-400 beside its Regular. The abort above still stops at THREE, so
    // the numbers differ on purpose: the base chain terminates at the Regular
    // and the extra cut is only ever reached past it.
    const clean = stub(kanitUpstream())
    const outcome = await fetchWebFamily('Kanit', clean.fetcher)
    expect(outcome.ok).toBe(true)
    expect(clean.asked).toHaveLength(4)
  })
})
