import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { openWorkspace } from './app.js'

// STORY 16.0 — THE PROBE HARNESS, AND IT IS EXECUTED.
//
// Every other spec in this directory carries the D-000.4 banner: compiled by
// `tsc --noEmit`, never run in a browser. THIS ONE IS DIFFERENT, and the
// difference is the whole point. The defect it exists for — `embedFontFamily`
// throwing at the WASM/worker boundary and reporting only "The engine returned
// an invalid response" — reached the owner PRECISELY BECAUSE the e2e suite is
// compile-checked and never executed. A verification pass that proved this
// story without a real browser would reproduce the conditions that produced
// the bug. Run it with:
//
//   PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH="$HOME/Library/Caches/ms-playwright/\
//   chromium-1217/chrome-mac-arm64/Google Chrome for Testing.app/Contents/\
//   MacOS/Google Chrome for Testing" \
//     npx playwright test e2e/font-embed-boundary.spec.ts --reporter=list
//
// THE EXECUTABLE PATH IS LOAD-BEARING, NOT OPTIONAL, AND THIS PARAGRAPH IS A
// MEASUREMENT RATHER THAN A MEMORY. `playwright.config.ts:11` passes
// `process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` straight into
// `launchOptions.executablePath` with NO fallback and no validation, so leaving
// it unset silently defers to whatever revision the installed @playwright/test
// pins — and `71627a5` unpinned that to `^1.63.0`.
//
// Measured 2026-09-05 with `du -sh` over `~/Library/Caches/ms-playwright/`:
// chromium-1217 336M, chromium-1223 341M, chromium-1228 344M — all complete and
// all usable. There is NO chromium-1208 on this machine at all; the 428 KB
// truncated stub earlier revisions of this comment warned about has been
// removed outright, so a warning naming it now describes a vanished artifact.
// `chromium_headless_shell-1243` IS present with **no headed chromium-1243**
// beside it, which is why the headed path above is spelled out in full rather
// than left for Playwright to resolve.
//
// A DIRECTORY EXISTING IS NOT A BROWSER: check `du -sh` before concluding a
// revision is usable. That ten-second check is what the stub defeated.
//
// THE CATALOGUE IS NEVER HARDCODED HERE, in either of the two places it could
// have been. The family list is read from `font-catalogue.json` — the build's
// own input, not a copy of it — and it is ALSO enumerated out of the running
// application's own listbox and compared against that file. A twenty-second
// family added to the catalogue therefore cannot escape this harness: it
// appears in the file, it appears in the listbox, and it gets picked. The
// generated `src/generated/font-catalogue.ts` cannot be imported here because
// it imports `.ttf?url` modules that only Vite resolves.
type CatalogueRow = Readonly<{ id: string; family: string }>
const cataloguePath = fileURLToPath(new URL('../font-catalogue.json', import.meta.url))
const catalogue = JSON.parse(readFileSync(cataloguePath, 'utf8')) as ReadonlyArray<CatalogueRow>
const families = catalogue.map((row) => row.family)

// AND THE STARTER TEMPLATE IS READ THE SAME WAY, FOR THE SAME REASON.
//
// Two assertions in this file closed over a premise rather than a source: "the
// starter declares no catalogue family". Story 16.8 renamed `starter.folio`'s
// only chain from `body` to `Roboto`, which made a catalogue family
// document-declared, and both assertions went red over behaviour that is
// CORRECT. `App.tsx:2580` puts every declared chain under `IN THIS TEMPLATE`;
// `:2604` filters those same names out of `AVAILABLE LOCALLY`, so a family the
// template declares is offered exactly once, in the other group.
//
// So the split is DERIVED from the template the browser actually loads —
// `public/templates/starter.folio`, the file `scripts/build-wasm.mjs` serves as
// `/starter.folio` — rather than from a premise about what it contains. Rename
// a chain, or declare a second one, and EVERY expectation below re-derives from
// the file with no line here changing: which group a row must appear under,
// from the declared set; and which pick is a no-op, from `defaultFontFamily`'s
// own rule over that same set (`bornFamily`, below). Node reads the template
// directly (precedent: `folio-go/wasm/cmd/engine/main_test.go:229`), and a
// template that parses to no chains at all THROWS: an empty declared set would
// silently collapse this file back to the one-armed harness it was.
//
// ⚠ AN ENTRY IS `unknown`, NOT `string` (Story 11.3). `starter.folio` declares
// style variants now, so two of its three entries are OBJECTS
// (`{"face": "Roboto", "bold": "Roboto Bold", …}`) and only the third is a bare
// string. This harness reads `Object.keys(fonts)` and nothing else, so the
// widening is a TRUTHFULNESS fix rather than a behavioural one — but a type
// that lies about the file it parses is exactly the premise-instead-of-source
// mistake the comment above records.
type StarterTemplate = Readonly<{ fonts?: Readonly<Record<string, ReadonlyArray<unknown>>> }>
const starterPath = fileURLToPath(new URL('../public/templates/starter.folio', import.meta.url))
const starter = JSON.parse(readFileSync(starterPath, 'utf8')) as StarterTemplate
const declaredChains = Object.keys(starter.fonts ?? {})
if (declaredChains.length === 0) throw new Error(`${starterPath} declares no \`fonts\` chains this harness can read; re-derive the partition rather than deleting it — every text element in a new document names one of these chains`)

// THE ONE PREDICATE BOTH TESTS BELOW PARTITION ON. `canvas.fontFamilies` — and
// therefore the `IN THIS TEMPLATE` group — is the document's declared chain
// NAMES (App.tsx:2524 -> 1769 -> 1741 -> 1478; Go side `page_setup.go:662,:778`),
// so "the catalogue minus the template's chains" is exactly what the second
// group offers and "the template's chains" is exactly what the first one does.
const declaredByTemplate = (family: string) => declaredChains.includes(family)
const templateFamilies = families.filter(declaredByTemplate)
const localFamilies = families.filter((family) => !declaredByTemplate(family))

// AND A SECOND, DIFFERENT PREDICATE — BECAUSE DECLAREDNESS IS NOT WHAT DECIDES
// WHETHER A PICK MOVES THE REVISION. VALUE-IDENTITY IS.
//
// A newly placed text element adopts `defaultFontFamily`
// (`folio-go/component_commands.go:1722-1734`): the first declared non-empty
// chain in SORTED KEY ORDER. So the element is born carrying one particular
// declared chain, and picking THAT one changes nothing — which is what makes
// the engine return its stable snapshot. Picking a DIFFERENT declared chain is
// a real value change: still a plain commit with nothing to embed, but the
// revision moves, correctly.
//
// Today the starter declares exactly one chain, so the two predicates coincide
// and it would be easy to write "declared" where "already carried" is meant.
// Declare a second catalogue chain sorting earlier and they come apart at once
// — the earlier one becomes the born family and the other becomes a declared
// pick that DOES advance. The revision expectation below is therefore derived
// from what the element actually carries before the pick, read off the field,
// with this constant as the independent claim about which chain that will be.
const bornFamily = [...declaredChains].sort()[0]!
const carriedFamilies = families.filter((family) => family === bornFamily)
const changedFamilies = families.filter((family) => family !== bornFamily)

// AND THE BOUNDARY'S SENTENCES ARE NOT HARDCODED EITHER — for the same reason,
// and it is the sharper of the two.
//
// The first version of this harness rejected exactly one string, "The engine
// returned an invalid response". That was already stale when it was written:
// this story's own change RENAMES the production fault, so a recurrence of the
// wasm out-of-memory now reaches the panel as "The engine's response could not
// be parsed: SyntaxError …". The harness would have recorded that as an
// ordinary located refusal and passed green over the exact defect it exists
// for. Four of the five stages evaded it.
//
// So the set is READ OUT OF engine.worker.ts's own `boundarySentences` table.
// A sixth stage added there enters this rejection set in the same commit that
// adds it, with nobody having to remember. If that table is renamed or
// restructured this throws rather than silently matching nothing — an empty
// rejection set is the failure mode being designed against, not an outcome.
const workerSource = readFileSync(fileURLToPath(new URL('../src/engine.worker.ts', import.meta.url)), 'utf8')
const boundarySentences = readBoundarySentences(workerSource)

function readBoundarySentences(source: string): ReadonlyArray<string> {
  const table = /const boundarySentences[^=]*=\s*\{([\s\S]*?)\n\}/.exec(source)
  if (!table) throw new Error('engine.worker.ts no longer declares a `boundarySentences` table this harness can read; re-derive the rejection set rather than deleting the check')
  const found = [...table[1].matchAll(/:\s*(['"])((?:\\.|(?!\1).)*)\1/g)].map((match) => match[2].replace(/\\(['"])/g, '$1'))
  if (found.length < 5) throw new Error(`read ${found.length} boundary sentences from engine.worker.ts, expected at least the five stages; the extraction has gone stale`)
  return found
}

// One row per family, in the shape the story asks to be reported: embedded,
// picked without embedding, or refused with its located text, or a boundary
// sentence — which is the defect. `outcome` is deliberately the engine's own
// words wherever the engine spoke, and not a classification of them: a
// diagnosis cannot be read out of a label this harness invented.
type Row = { family: string; carried: string; outcome: string }

// THE TWO LABELS, DEFINED WHERE THEY ARE DECLARED — because the previous
// version of this harness recorded the bare literal `'EMBEDDED'` for every
// family, including the one that could not possibly have embedded. A label
// reading EMBEDDED beside a pick that embedded nothing is the defect this
// partition exists to remove, so each one now states exactly what was measured
// AND NO MORE.
//
// IN PARTICULAR, NEITHER LABEL CLAIMS A CAUSE. This harness can see the
// revision and the error paragraph; it cannot see which branch of
// `App.tsx:2688-2692`'s fork ran, so it must not say. `ADVANCED` covers both a
// first-use embed-then-commit and a plain commit of a changed declared chain —
// the two-command order is read off the wire, with a payload in hand, in
// `src/App.test.tsx`. `UNCHANGED` says only that the revision did not move,
// which is the consequence a browser CAN observe of
// `folio-go/internal/wasm/engine.go:334-340` returning its stable snapshot.
const ADVANCED = 'ADVANCED (the pick changed the element\'s family and the engine committed it: the revision moved)'
const UNCHANGED = 'UNCHANGED (the element already carried this family: the pick landed, the engine kept its stable snapshot, and the revision held for the whole watch window)'

// THE ABSENCE, MEASURED RATHER THAN SLEPT THROUGH. A fixed `waitForTimeout`
// would pass over a revision bump that landed one poll after it, and would put
// a bare numeral in a file whose whole thesis is deriving rather than
// restating. This holds the claim open for the whole window and returns on the
// FIRST disturbance it sees, so a late move is a finding rather than a nap.
const NO_CHANGE_WINDOW_MS = 3_000
const NO_CHANGE_POLL_MS = 250

async function firstDisturbance(page: import('@playwright/test').Page, revision: number): Promise<string | undefined> {
  const deadline = Date.now() + NO_CHANGE_WINDOW_MS
  for (;;) {
    const alerts = await page.locator('p.property-error').allTextContents()
    if (alerts.length > 0) return `REFUSED: ${alerts[0]}`
    const now = await currentRevision(page)
    if (now !== revision) return `REVISION MOVED ${revision} -> ${now} after a pick that changed no value`
    if (Date.now() >= deadline) return undefined
    await page.waitForTimeout(NO_CHANGE_POLL_MS)
  }
}

const boundaryAnswers = (rows: ReadonlyArray<Row>): ReadonlyArray<Row> =>
  rows.filter((row) => boundarySentences.some((sentence) => row.outcome.includes(sentence)))

async function placeAndSelectText(page: import('@playwright/test').Page) {
  await openWorkspace(page)
  await expect(page.getByTestId('engine-snapshot')).toHaveText(/GO SNAPSHOT · REVISION 1/)
  const content = page.getByRole('region', { name: 'Content', exact: true })
  await page.getByRole('button', { name: 'Place Text' }).click()
  await content.press('Enter')
  await content.getByRole('button', { name: /text component/ }).last().click()
  await expect(page.getByRole('combobox', { name: 'Font family' })).toBeVisible()
}

async function currentRevision(page: import('@playwright/test').Page): Promise<number> {
  const label = await page.getByTestId('engine-snapshot').textContent()
  const match = /REVISION (\d+)/.exec(label ?? '')
  return match ? Number(match[1]) : -1
}

// STORY 16.1 SPLIT THE SECOND GROUP IN TWO, AND THIS TEST NOW ASSERTS THE
// TIER RATHER THAN THE WHOLE LIST.
//
// The control used to offer exactly the committed families and nothing else.
// It now offers those PLUS families from the build-time index snapshot, whose
// bytes are fetched at the moment of a pick — and the painted list is capped
// while the count is not. So "exactly the families the catalogue declares" is
// no longer a true statement about the listbox, and the property that replaced
// it is the one the ruling actually cares about: THE LOCAL FACE TIER IS OFFERED
// IN FULL, IT SAYS THAT IT NEEDS NO DOWNLOAD, AND NOTHING IS MISSING FROM IT.
// A twenty-second committed family still cannot escape this harness.
//
// STORY 16.5 CHANGED WHAT THE NOTES SAY, BECAUSE IT CHANGED WHAT A PICK DOES. A
// family this machine already holds is USED when it is picked — the face is
// embedded and the property committed, two commands — while one it does not hold
// is INSTALLED and reaches no document at all. The local tier is the first case,
// which is why this file's every-family-embedded measurement below survives the
// split intact.
//
// STORY 16.7 REMOVED `LOCAL_NOTE` FROM THE ROW (Design Note 2): a specimen
// replaced it, so a local-tier row's own text is now just its family name.
// The LOCAL half of this measurement finds its rows by GROUP MEMBERSHIP and
// reads each one's name off `.property-option-name`, never off a note this
// group no longer carries.
//
// STORY 16.9 REMOVED THE SNAPSHOT TIER FROM THIS DROPDOWN ENTIRELY. The
// control no longer offers "those PLUS families from the build-time index
// snapshot" — that PLUS is gone, `Add fonts…` is its door now, and `WEB_NOTE`
// below is asserted ABSENT rather than present.
const WEB_NOTE = ' — install on this machine'

test('the dropdown splits the catalogue into the template\'s own chains and the whole rest of the local face tier, with no install row', async ({ page }) => {
  await placeAndSelectText(page)
  await page.getByRole('combobox', { name: 'Font family' }).click()
  const local = (await page.getByRole('group', { name: 'AVAILABLE LOCALLY' }).getByRole('option').locator('.property-option-name').allTextContents()).map((text) => text.trim())
  // STORY 16.8 MADE A CATALOGUE FAMILY DOCUMENT-DECLARED, AND THIS LINE HAD
  // CLOSED OVER ITS ABSENCE. The expectation is now the catalogue MINUS the
  // starter's declared chains, both read from their own files — the same
  // subtraction `App.tsx:2604` performs (`!families.includes(source.family)`),
  // derived from the template rather than restated as a numeral or a name.
  expect(localFamilies.length, 'the starter may not declare every catalogue family; AVAILABLE LOCALLY would then be empty and this assertion vacuous').toBeGreaterThan(0)
  expect([...local].sort()).toEqual([...localFamilies].sort())
  // AND THE POSITIVE HALF OF THE SAME SUBTRACTION, WITHOUT WHICH THIS IS A
  // BLIND SPOT RATHER THAN A REPAIR. Removing a family from one group's
  // expectation while asserting nothing about the other converts a false red
  // into a family that is simply never offered anywhere and nobody notices.
  expect(templateFamilies.length, 'the starter must declare at least one CATALOGUE family for the IN THIS TEMPLATE assertion below to have a subject').toBeGreaterThan(0)
  const template = (await page.getByRole('group', { name: 'IN THIS TEMPLATE' }).getByRole('option').locator('.property-option-name').allTextContents()).map((text) => text.trim())
  expect([...template].sort(), 'every chain the starter declares must be offered under IN THIS TEMPLATE, including the ones subtracted from AVAILABLE LOCALLY above').toEqual([...declaredChains].sort())
  // AND THE THIRD TIER IS GONE (Story 16.9): the dropdown no longer offers a
  // row for any family not on this machine at all — `Add fonts…` is the
  // only door to one now — so no option may carry the install note.
  const options = await page.getByRole('listbox', { name: 'Fonts' }).getByRole('option').allTextContents()
  const web = options.filter((text) => text.includes(WEB_NOTE))
  expect(web.length, 'Story 16.9 removed the install-tier group; no row may carry its note').toBe(0)
  await expect(page.getByRole('group', { name: 'AVAILABLE TO INSTALL' })).toHaveCount(0)
  // THE ADDABLE-COUNT ASSERTION IS DELETED, AND ITS SUBJECT NO LONGER EXISTS.
  // Four lines here required the control to state how many families can be
  // added, and how stale that build-time snapshot was, by quoting
  // `familyIndexDisclosure()`'s sentence. That paragraph was cut
  // from this dropdown before Story 16.7 opened, and STORY 16.10 THEN DELETED
  // `familyIndexDisclosure()` ITSELF — measured at zero consumers — because the
  // design draws no paragraph in the browser's header either. The assertion was
  // therefore not stale but permanently unsatisfiable: there is no sentence
  // anywhere for it to find. Left in place it would be a red a human has to
  // read past, which is how the next real red goes unnoticed.
  //
  // THE COUNT SURVIVES WHERE IT ALWAYS BELONGED. `resultLine`
  // (`font-browser-model.ts:318-321`) still prints `N of M families` in the
  // browser's results toolbar, pinned by `font-browser-model.test.ts:285-288`,
  // and `addableFamilyCount`'s own two facts are pinned in
  // `font-index.test.ts:253-256`. Two different subjects, two files, neither of
  // them this one — see `App.test.tsx:1932-1941`, which records the same
  // deletion on the unit side.
})

// STORY 16.5: THIS MEASUREMENT IS UNCHANGED, AND THAT IS THE POINT OF STATING IT.
//
// Install/embed separation moves WHEN a face enters a document for a family this
// machine does not hold. The committed catalogue families are the LOCAL FACE TIER
// — they ship inside the release, so this machine already holds them — and picking
// one is FIRST USE, which embeds. So the after-state is still every one of them
// EMBEDDED and nothing less, over whatever `families` currently holds — the
// assertion below reads the list rather than a numeral, which is why it cannot
// rot into a floor the way the counts in the comments below had.
//
// WHAT THE REVISION POLL NOW WATCHES IS TWO COMMANDS RATHER THAN ONE: the embed
// and then the `fontFamily` commit. `> before` is satisfied by either, and it is
// deliberately not tightened to `+2` here — this file's subject is the WASM
// boundary answering at all, and the two-command order is asserted on the wire in
// `src/App.test.tsx` and `src/App.font-store.test.tsx` where a payload can be read.
//
// STORY 16.8 BROKE THE SENTENCE THIS TEST WAS NAMED AFTER, AND THE HONEST REPAIR
// IS TWO PARTITIONS, NOT A NARROWER LOOP.
//
// "Every catalogue family embeds" stopped being true the moment `starter.folio`
// declared `Roboto`. The repair needs TWO different splits, and collapsing them
// into one is the mistake this comment exists to prevent:
//
//   · WHICH GROUP THE ROW SITS IN is decided by DECLAREDNESS. A declared name is
//     an `IN THIS TEMPLATE` row and `App.tsx:2688-2692` routes it to a plain
//     `commit`; only an `AVAILABLE LOCALLY` row carries a `source` and can reach
//     `commitFirstUse`'s embed at all.
//   · WHETHER THE REVISION MOVES is decided by VALUE-IDENTITY. The element is
//     born carrying `bornFamily`, so picking that one changes nothing and the
//     engine returns its stable snapshot; picking anything else — declared or
//     not — is a real change and the revision advances.
//
// With one declared chain the two coincide. Writing the second as though it were
// the first would make this loop fail over correct behaviour the moment a second
// declared chain appeared, which is the same closing-over-a-moving-set defect
// this story exists to remove, freshly minted.
//
// THE TEMPTING REPAIR IS THE DISHONEST ONE. Setting the element to some other
// family first and then picking would make the old single-armed loop pass, and
// record EMBEDDED for a family that was never embedded. A test that goes green by
// making its own label false is worse than the red it replaced.
test('every family is offered in the group its declaredness puts it in, the pick that changes a value advances the revision, and the pick that changes none leaves it exactly where it was', async ({ page }) => {
  // ONE FAMILY PER BLANK DOCUMENT: a shared document would let one pick's
  // document state (every earlier family's asset and chain) confound the next
  // pick's result, and the report would no longer be one measurement per family.
  //
  // STALE NUMERAL CORRECTED (Story 16.5, D-16.R.35's rule). This said "21
  // families" in three places; Story 16.1a grew the local face tier and
  // `font-catalogue.json` has carried more than 21 entries since. The count is
  // deliberately not restated as a new numeral — `families` is the list and
  // every assertion below is written against it.
  test.setTimeout(900_000)
  // EVERY CLASS MUST HAVE A SUBJECT, ON BOTH PARTITIONS. A split that puts every
  // family on one side asserts nothing about the other and would pass green over
  // exactly the asymmetries this test was rewritten to measure.
  expect(templateFamilies.length, 'the IN THIS TEMPLATE group has no catalogue subject: the starter template declares no catalogue family').toBeGreaterThan(0)
  expect(localFamilies.length, 'the AVAILABLE LOCALLY group has no subject: the starter template declares every catalogue family').toBeGreaterThan(0)
  expect(carriedFamilies.length, `the UNCHANGED class has no subject: a new element is born carrying ${bornFamily}, which is not a catalogue family, so no pick in this loop can be a no-op`).toBeGreaterThan(0)
  expect(changedFamilies.length, 'the ADVANCED class has no subject: every catalogue family is the one a new element is born carrying').toBeGreaterThan(0)
  const rows: Row[] = []
  for (const family of families) {
    await placeAndSelectText(page)
    const before = await currentRevision(page)
    const combobox = page.getByRole('combobox', { name: 'Font family' })
    // READ BEFORE OPENING. `App.tsx:2746` renders `value={open ? query : committed}`,
    // so the field reads the element's committed family only while the dropdown
    // is shut — once `fill` has run it reads the search text instead. This is the
    // value the revision expectation is derived from.
    const carried = await combobox.inputValue()
    await combobox.click()
    await combobox.fill(family)
    // THE GROUP THE ROW MUST SIT IN, FROM THE TEMPLATE. Reading the group the row
    // is actually offered under is what turns a wrong partition into a red here
    // rather than into a wrong expectation further down — and it is also the leg
    // that proves the document really does declare a chain for this family.
    const groupLabel = declaredByTemplate(family) ? 'IN THIS TEMPLATE' : 'AVAILABLE LOCALLY'
    const option = page.getByRole('group', { name: groupLabel }).getByRole('option', { name: family, exact: true })
    let outcome = 'TIMED OUT with no engine answer'
    try {
      // MEASURED, NOT ASSERTED, so one misplaced family is a row in the table
      // rather than a throw that abandons the other thirty — which is the rule
      // the catch below already states and this check used to sit outside of.
      const offered = await option.count()
      if (offered !== 1) {
        outcome = `NOT OFFERED under ${groupLabel} (${offered} matching rows): starter.folio's declared chains are what put a family in that group`
      } else if (carried === family) {
        // THE NO-OP ARM. `choose` sends the command unconditionally — there is no
        // comparison against `committed` in `App.tsx` — and the ENGINE is what
        // decides the pick was not a mutation: `folio-go/internal/wasm/engine.go:334-340`
        // returns the stable snapshot for canonical bytes that did not move, so
        // the revision, the dirty flag and both history branches stay exactly as
        // they were. That is the AD-15 property, and the browser can only see its
        // consequence. "NO COMMAND WAS SENT" IS NOT OBSERVABLE FROM HERE and is
        // deliberately not asserted: it is measured on the wire, with a payload in
        // hand, in `src/App.test.tsx`.
        await option.click()
        // NON-VACUITY: THE PICK HAS TO HAVE LANDED. Everything this arm expects is
        // an absence, so a click that hit nothing would satisfy all of it. The
        // dropdown being gone is `choose` having run through to `close()`, and it
        // is the ONE leg here that can fail — checking the field's value would
        // not, because `fill` has already put this family into `query` and the
        // field reads whichever of the two applies.
        await expect(page.getByRole('listbox', { name: 'Fonts' }), 'the pick did not land: the dropdown never closed, so `choose` never ran').toHaveCount(0)
        outcome = (await firstDisturbance(page, before)) ?? UNCHANGED
      } else {
        await option.click()
        await expect
          .poll(async () => {
            const alerts = await page.locator('p.property-error').allTextContents()
            if (alerts.length > 0) return `REFUSED: ${alerts[0]}`
            return (await currentRevision(page)) > before ? ADVANCED : 'pending'
          }, { timeout: 120_000, intervals: [250] })
          .not.toBe('pending')
        const alerts = await page.locator('p.property-error').allTextContents()
        outcome = alerts.length > 0 ? `REFUSED: ${alerts[0]}` : ADVANCED
      }
    } catch (error) {
      // A FAMILY THAT NEVER ANSWERS IS A FINDING, NOT A REASON TO ABANDON THE
      // OTHER THIRTY — but the arms fail for different reasons and the table must
      // not conflate them. The advancing arm's only throw is its poll timing out,
      // which the sentence above already says. The no-op arm's throw is its
      // non-vacuity leg, and "the pick never landed" is a different finding from
      // "the engine never answered".
      if (carried === family) outcome = `PICK DID NOT LAND: ${error instanceof Error ? error.message.split('\n')[0] : String(error)}`
    }
    rows.push({ family, carried, outcome })
    console.log(`${family.padEnd(24)} ${outcome}`)
  }
  console.log('\nPER-FAMILY TABLE')
  for (const row of rows) console.log(`| ${row.family} | carried ${row.carried} | ${row.outcome} |`)

  // THE ACCEPTANCE, IN THREE PARTS, AND ALL THREE ARE LOAD-BEARING.
  //
  // First: NO pick may be answered by ANY boundary sentence. A boundary
  // sentence is never an engine refusal — the engine's refusals carry a
  // diagnosticCode and a located message through a different arm entirely —
  // so one appearing here is always this story's defect wearing a new stage
  // name, and must never be counted as a located refusal.
  expect(boundaryAnswers(rows).map((row) => `${row.family}: ${row.outcome}`)).toEqual([])
  const silent = rows.filter((row) => row.outcome.startsWith('TIMED OUT'))
  expect(silent.map((row) => row.family)).toEqual([])

  // Second: EVERY ELEMENT WAS BORN CARRYING THE CHAIN THE ENGINE'S OWN RULE SAYS
  // IT WOULD — the first declared chain in sorted key order
  // (`component_commands.go:1722-1734`). This is what licenses deriving the
  // revision expectation from `carried` below, and it is asserted rather than
  // assumed: if the rule changed, every row's expectation would silently follow
  // the product instead of failing.
  expect(rows.filter((row) => row.carried !== bornFamily).map((row) => `${row.family}: born carrying ${row.carried}`)).toEqual([])

  // Third: THE OVER-BROADNESS CONTROL, ONE MODEL OVER BOTH CLASSES. Everything
  // above is satisfied by a regression that refuses every family with a tidy
  // located reason, while the Go half has had an answer to that since
  // TestStaticFaceIsStillEmbeddedAtBothDoors. Every catalogue face is static by
  // construction (each public/fonts/*/NOTICE.md asserts no `fvar`, and
  // scripts/build-wasm.mjs validates it at build time), so the measured
  // after-state is: every pick that changed the element's family ADVANCED, every
  // pick that changed nothing left the revision UNCHANGED, and no pick in either
  // class produced a `p.property-error`.
  const expected = (row: Row) => (row.carried === row.family ? UNCHANGED : ADVANCED)
  expect(rows.filter((row) => row.outcome !== expected(row)).map((row) => `${row.family}: ${row.outcome}`)).toEqual([])
  // AND THE COUNTS COME FROM THE PARTITION, NOT FROM A NUMERAL: a class that
  // silently emptied would otherwise satisfy the line above vacuously.
  expect(rows.filter((row) => row.outcome === ADVANCED)).toHaveLength(changedFamilies.length)
  expect(rows.filter((row) => row.outcome === UNCHANGED)).toHaveLength(carriedFamilies.length)
  expect(rows).toHaveLength(families.length)
})

// THE DISCLOSURE CHEVRON IS THE MOCKUP'S OWN SVG, MEASURED IN A REAL BROWSER,
// AND THE COMPARISON IS READ OUT OF THE MOCKUP RATHER THAN COPIED FROM IT.
//
// A first version of this button drew the chevron as a text character
// (`▲`/`▼`), justified by a canvas ink-extent measurement showing it centred
// in THIS font. That measurement was true and the conclusion was wrong: text
// glyphs centre by font metrics, a UI typeface swap could silently reopen the
// gap, and a filled triangle is the wrong shape next to the design's thin
// stroked chevron in `Font Browser.dc.html:185`. The corrected button draws
// that exact `<svg>` — one glyph, unconditionally, since the mockup's own
// `dropdownOpen` flag there gates the menu panel, not the chevron — and
// leans on `.property-inline-action`'s `display: grid; place-items: center`
// for a BOX-level centering that cannot drift with the font, the same way
// the mockup's `align-items: center` centres it in its row.
//
// The `viewBox`/`path d` this test compares against are read out of the
// mockup file, not retyped here, for the same reason `families` above is read
// out of `font-catalogue.json`: a hand-copied second source of truth goes
// stale silently, while a read that finds nothing throws.
const mockupPath = path.resolve(fileURLToPath(new URL('.', import.meta.url)), '..', '..', '_bmad-output', 'planning-artifacts', 'ux-designs', 'ux-folio-2026-08-23', 'mockups', 'Font Browser.dc.html')
const mockupSource = readFileSync(mockupPath, 'utf8')

function readMockupChevron(source: string): Readonly<{ viewBox: string; path: string }> {
  const match = /<svg width="8" height="8" viewBox="([^"]+)"[^>]*><path d="([^"]+)"><\/path><\/svg>/.exec(source)
  if (!match) throw new Error('the mockup no longer draws the 8x8 chevron this test compares the field against at Font Browser.dc.html:185; re-derive the comparison rather than deleting the check')
  return { viewBox: match[1], path: match[2] }
}
const mockupChevron = readMockupChevron(mockupSource)

test('the disclosure chevron is the mockup\'s own SVG, centred structurally in both states', async ({ page }) => {
  await placeAndSelectText(page)
  const button = page.locator('button.property-disclosure')
  const svg = button.locator('svg')

  const measure = async () => {
    const buttonBox = await button.boundingBox()
    const svgBox = await svg.boundingBox()
    if (!buttonBox || !svgBox) throw new Error('the disclosure button or its svg has no box; it is not laid out')
    const deltaX = (svgBox.x + svgBox.width / 2) - (buttonBox.x + buttonBox.width / 2)
    const deltaY = (svgBox.y + svgBox.height / 2) - (buttonBox.y + buttonBox.height / 2)
    return { deltaX, deltaY, viewBox: await svg.getAttribute('viewBox'), path: await svg.locator('path').getAttribute('d') }
  }

  // CLOSED STATE.
  await expect(button).toHaveAttribute('aria-label', 'Show fonts')
  const closed = await measure()
  expect(closed.viewBox, 'the rendered chevron must be the mockup\'s own viewBox, not a stand-in').toBe(mockupChevron.viewBox)
  expect(closed.path, 'the rendered chevron must be the mockup\'s own path, not a stand-in').toBe(mockupChevron.path)
  expect(Math.abs(closed.deltaX), `the chevron must be centred in the field; measured deltaX=${closed.deltaX}`).toBeLessThan(0.5)
  expect(Math.abs(closed.deltaY), `the chevron must be centred in the field; measured deltaY=${closed.deltaY}`).toBeLessThan(0.5)

  // OPEN STATE — SAME GLYPH, NO FLIP. The mockup draws one chevron regardless
  // of `dropdownOpen`; a two-state flip here would be inventing state the
  // design does not have. The accessible name still flips: that is
  // screen-reader state the design does not govern.
  await button.click()
  await expect(button).toHaveAttribute('aria-label', 'Hide fonts')
  const open = await measure()
  expect(open.path, 'the design draws one chevron, not two; this button must not flip shape when open').toBe(closed.path)
  expect(open.viewBox).toBe(closed.viewBox)
  expect(Math.abs(open.deltaX), `the chevron must stay centred when open; measured deltaX=${open.deltaX}`).toBeLessThan(0.5)
  expect(Math.abs(open.deltaY), `the chevron must stay centred when open; measured deltaY=${open.deltaY}`).toBeLessThan(0.5)
})

