# Autonomous decision log — spec-font-sources-and-embedding

Standing instruction, 2026-09-20: *"run to the end of the spec without stop and ask me the
question. log all decision in a file under the spec folder."*

Every decision below was made **without asking**, under that instruction. Each entry records the
choice, the options rejected, and the reasoning — so a decision that turns out wrong can be found
and reversed without re-deriving why it was made. Decisions the owner made in conversation live in
`.memlog.md` and in each story's frozen block; this file holds only what was decided autonomously.

Story `done_checkpoint` flags in `stories.yaml` are overridden by this instruction and are not
honoured during this run. Story 3's flag is the only one affected.

---

## A-0 — Where this log lives, and what goes in it

**Decision:** `DECISIONS.md`, a sibling of `SPEC.md`, not a companion and not listed in
`companions:`.

**Rejected:** appending to `.memlog.md` (it is the owner's decision-of-record and append-only via a
script — mixing autonomous calls into it would blur who decided what); a per-story log (the whole
point is one place to scan).

**Reasoning:** the owner asked for "a file under the spec folder". It is deliberately *not* a
companion, because companions are the what-to-build contract every downstream consumer reads, and
this is a record of how one run behaved.

---

## Story 1 — verification findings, and what was done about them

### A-1 — The one red test is pre-existing, and was proved so rather than assumed

**Decision:** accepted `folio-go/internal/text` `TestCorpusMeetsP6ExerciseFloors/P6g` as red and
unrelated; did not fix it, did not skip it, did not fold it into this story.

**Evidence, not the subagent's word:** checked out the baseline commit `85cdcc0` into a throwaway
worktree and ran that test alone. Identical numbers — `got 7, need >=20`, `P6g:7`. The diff touches
no file under `internal/text`. The worktree was removed afterwards.

**Reasoning:** a red test a story did not cause is not that story's to fix, and fixing it here would
put an unrelated corpus change inside a font-resolution commit.

### A-2 — `npm run lint` reports a false failure in this environment

**Decision:** treated the `folio-js` lint failure as an artifact of this environment's command
proxy, not a code defect, and rewrote the spec's Verification block to invoke the binaries directly.

**Evidence:** `npm run lint` exits 2 printing `ESLint output (JSON parse failed: EOF while parsing
a value at line 1 column 0)` — which is a wrapper trying to parse the script's output as ESLint
JSON. Running the same two tools directly, `./node_modules/.bin/oxlint` and
`./node_modules/.bin/tsc -p tsconfig.test.json --noEmit`, both exit 0.

**Also corrected:** the spec named `npm run typecheck` in `folio-js`, which does not exist — that
script lives in another package and I lifted it from the wrong CI job when writing the spec. My
error, fixed in the spec rather than worked around.

### A-3 — A defensive inconsistency left for review rather than silently patched

**Finding:** `substitutionPool` (`folio-go/render.go:1955`) guards `cache == nil` and returns the
`FontSet` keys, but its only caller `substituteFace` (`:1978`) then calls `cache.get(name, fs)`
unguarded — so the nil guard promises a safety its caller does not honour.

**Decision:** not fixed here. Every production call site of `shapeSegments` passes a real cache
(`render.go:851`, `table_render.go:907/1250/1506`, `page_number.go:463`,
`page_setup.go:1054/1668`), so it is unreachable rather than a live bug.

**Reasoning:** it is a review-shaped question — delete the misleading guard, or honour it — and
step-04's reviewers should see the code as the implementer left it, not as I pre-emptively
adjusted it. Recorded here so it cannot be lost if review misses it.

**CLOSED by the A-6 re-derivation.** All three review layers raised it in pass 1; the revert then
replaced the code entirely, and the re-derived `substitutionPool`/`substituteFace` carry no nil
guard at all, so the inconsistency no longer exists. The line numbers above refer to code that was
discarded and should not be chased.

### A-4 — Accepted the implementer's weaker .NET byte-identity assertion

**Decision:** accepted that the .NET test asserts "a font set the document never consults does not
change the bytes" over the existing `fixtures/embedded-font`, rather than the full "identical bytes
under any font set".

**Reasoning:** that fixture's chain names `"Noto Sans"` alongside its asset, so it is not an
all-embedded document and the stronger claim is not true of it. The stronger claim — the one the
issue reporter measured — is proved in Go against a purpose-built all-asset document
(`TestAnAllEmbeddedDocumentNeedsNoFontSetAtAll`). Proving it once, where it is true, beats proving
a weaker thing twice.

### A-5 — ABI version moved 1 → 2, which is a real break

**Decision:** let it stand.

**Reasoning:** the cshared entry points gained a parameter, so the ABI genuinely changed and the
version check exists precisely to catch a mismatched pair. The consequence — the managed package
and the native library must ship together — is acceptable inside the coordinated 2.0.0 release this
spec is already scoped to, and it is exactly the kind of break that release exists to absorb.

### A-6 — Took the bad_spec loopback rather than patching, and reverted 41 files to do it

**Decision:** routed review findings #1/#2 to `bad_spec`, reverted every code change to the baseline
commit, amended the spec, and re-derived — instead of patching the one broken arm.

**The finding:** a chain whose *every* member is absent refuses under `FaceFallbackSubstitute`
before shaping is reached, because `verticalModel` (`folio-go/wrap.go:653`) raises on
`len(metrics) == 0` without consulting the selector. Two review layers found it independently. I
proved it with a throwaway probe rather than inferring it: `{"body": ["Brand Face"]}` rendered
leniently over the shipped set was refused. That is the spec's own Success-signal case.

**Why bad_spec and not patch:** the root cause is a sentence *I* wrote in the Code Map — *"This is
the 'no candidate' state; keep it an error."* True under strict, false under lenient, and the
implementer followed it exactly. The workflow says to prefer `bad_spec` when in doubt because a
spec-level fix produces more coherent code, and that applies here: the real fix has to decide what
line metrics a substituted face contributes, which is finding #2 and is a design question, not a
one-line guard.

**Why the revert was worth its cost:** ten further findings were outstanding. Patching them into
code that was about to be replaced would have been wasted work, so all ten are folded into the
amended spec as explicit tasks and get fixed in one re-derivation. The KEEP block preserves the
sixteen things the first attempt got right, so the revert costs the implementation, not the design.

**What would have made this unnecessary:** a test for the single-entry absent chain. Matrix row 4
passed only because every lenient test used a two-entry chain with a present member — the row was
covered in name and not in substance. That is now an explicit task.


### A-7 — Accepted a layout trade-off the owner should know about

**Decision:** accepted that a **lenient** render widens the line box whenever a chain member is
absent — *even when nothing actually substitutes* — so its bytes differ from a strict render of the
same document, with no diagnostic saying so.

**Why it happens:** the table body's vertical model is computed **once per table, before any cell is
shaped**, so the faces that will actually be painted are not knowable at the point the line height
is decided. The implementer chose a data-independent envelope over every pool face, gated on
`absent && lenient`. Sizing from the painted faces instead would make line height depend on row
content, which is a restructuring of the table vertical model, not a fix.

**Why accepted rather than escalated:** strict is byte-for-byte unchanged, so no existing integrator
is affected; lenient is new and opt-in, so there is no prior lenient output to preserve; and the
envelope is never too *small*, so nothing clips — the failure mode is slightly generous leading, not
broken layout. Deciding it was within the standing instruction not to stop and ask.

**What the owner may want to revisit:** if byte-identity between strict and lenient matters for a
document whose chain has an absent member that never actually substitutes, this is the thing to
change, and it costs a table-vertical-model restructure. Flagged because it is the one place this
story's behaviour is *surprising* rather than merely new.

**What was fixed rather than accepted:** the same behaviour was documented in the Go guide only.
That half violates this story's "the three libraries ship the same behaviour" constraint outright
and was sent back as a patch — a .NET or Node caller must be told their leading moves too.

### A-8 — One finding deferred rather than chased

**Decision:** deferred review finding #33 — whether a chain whose every member is a carried asset
with unparseable bytes reaches the substitution pool under lenient, or refuses before consulting it.

**Reasoning:** graded `maybe-false`. The pool arm is gated on `absent`, and it is genuinely unclear
from the diff whether an unparseable *carried* asset sets that flag. It would be settled by one
fixture: a single-entry chain naming a carried asset with corrupt bytes, rendered leniently with a
covering face in the supplied font set. I asked the implementer to check it directly rather than
leaving it purely deferred.

**OUTCOME: refuted, so nothing is deferred.** The refusal is never reached. With text,
`shapeSegments` aborts first on the carried-face parse error (`required table "head" is absent`),
byte-identically under both selectors — because `absent` is `!cache.declares(...)`, not `!present`,
so a declared-but-unparseable asset is a document fault with nothing to substitute *for*. With no
text, `verticalModel`'s empty-metrics arm is not reached at all. Verified across
`{"", "Hi"} x {Strict, Substitute}`. The `deferred-work.md` entry was rewritten to say so rather
than left standing as open work that does not exist.

## Story 2 — decisions taken while planning

### A-9 — A new leaf package, added to the census deliberately

**Decision:** the disk loader is a new importable package under `folio-go/`, and
`TestOnlyRootAndFontsAreImportableLibraryPackages`'s allowlist is extended to admit it.

**Rejected:** putting it in `folio-go/fonts` (forces a consumer who wants only a disk loader to
pull in ~14.8 MB of embedded faces — the exact cost `fonts.go`'s header calls load-bearing); and
putting it in the root `folio8` package (would put filesystem-font code inside the engine's own
package, which is the distinction this whole spec draws).

**Reasoning:** the census test's own failure text says *"Move it under internal/, or add it to the
census deliberately."* `internal/` is not available — the whole point is that integrators import
it. So this is the deliberate branch, taken with the allowlist and `publicSurfacePins` extended in
the same commit rather than worked around.

### A-10 — Keyed by the binary's name table, not by filename

**Decision:** a disk face is keyed by sfnt name ID 1, plus name ID 2 when the subfamily is not
`Regular` — `Sarabun`, `Sarabun Bold` — matching the shape `fonts.Shipped()` already uses.

**Rejected:** keying by filename stem. It is simpler, needs no new code, and would have let the
integrator control keys directly by renaming files.

**Reasoning:** `fonts/fonts.go:170-175` is emphatic that nothing may derive a family from a key and
that *"the machine-readable family is the face's own sfnt name ID 1"* — it names
`strings.TrimSuffix(key, " Bold")` as the specific anti-pattern, because it reinstates a
naming-convention weight carrier the project foreclosed. A loader that trusted filenames makes the
same mistake one layer out: a file a human renamed becomes a different face, or silently shadows
another. It also has to match what the designer writes in story 3, which reads the same name table.

**Cost accepted:** name ID 1 is not reachable from any exported symbol today, so this story adds
family/subfamily accessors to `internal/fontset.Font` beside the existing `PostScriptName`. That
is new surface inside the seam, which is why it is recorded here rather than assumed.

### A-11 — No merge helper, and no CLI fallback flag

**Decision:** the loader returns the directory's set and nothing else; callers combine sets with
`maps.Copy`. And `FaceFallback` gets no CLI flag in this story.

**Reasoning:** "second wins" is already the repo's merge precedent in two places (`fonts.go:200`,
`internal/wasm/engine.go`), so inventing a precedence rule here would be a third answer to a
settled question. The CLI fallback flag is tempting because it would make the disk path
demonstrable end to end, but story 2's success criterion needs no substitution — the face is
present — and adding it would smuggle story 1 surface into story 2.

### A-12 — `Skipped.Reason` left as prose, against my own judgement

**Decision:** rejected review finding #16 — a caller who wants to fail their build on "a real font
was rejected" but tolerate "a variable build" must string-match English sentences the validators
own. `Skipped` keeps `Reason string` and gains no programmatic cause.

**Why I think the finding is right:** the guide tells integrators they can act on skips, and acting
on prose owned by another package is not something a careful caller should have to do. A wrapped
`Err error` field would make it `errors.Is`-able, and it is much cheaper to add before the v1
public surface freezes than after.

**Why I rejected it anyway:** the workflow's routing rules bar it from `patch` — the fix adds
public surface the spec does not settle — and the only other route is `intent_gap`, which means
stopping to ask the owner. The standing instruction for this run is not to stop. Rejecting it
visibly, with the cost written down, is more honest than smuggling a public-surface decision
through as a patch.

**What the owner may want to do:** add the field before `folio-go` 2.0.0 ships. After that it is a
surface change rather than a surface choice.

### A-13 — Two findings rejected on the workflow's own rules, not on the merits

**Decision:** rejected review finding #17 (AC4 claims `go test ./...` passes, but `internal/text`'s
`P6g` corpus floor is red) and #18 (Tasks say fixtures live under `testdata/`; they are built into
`t.TempDir()`).

**Reasoning:** #17 is *true* — the acceptance criterion as written is false — but the workflow
says to reject any finding whose fix is to edit this build's spec, and the failure is pre-existing,
unrelated to the diff, confirmed red at the baseline commit, and already disclosed in this same
spec's Verification section. #18 is not a defect at all: building fixtures from bytes already in
the repo is precisely what makes the filename-independence assertions mean anything, so the
implementer's deviation from my planning guess was the better call.

### A-14 — Two judgement calls delegated to the implementer, and what they chose

**Context:** two review findings had no single right answer, so the patch message asked the
implementer to decide and say which way it went, rather than my guessing from outside the code.

**Symlinks — kept the strict rule.** A font-named symlink is never followed, including one
resolving inside the directory. Rejected: accepting in-directory links, which deploy scripts do use
to stage fonts. The reasoning given, and I agree with it: "inside" has no cheap correct answer once
the directory is itself a link. The compensating change is that a font-named symlink is now
*reported as skipped* rather than vanishing silently, so an operator staging by symlink is told,
instead of seeing an empty set. Documented in the package doc, both doc twins and the README.

**`-strict` — a skipped font now fails a strict run.** So does an empty font directory. Without
`-strict` both remain stderr notices and the run succeeds. This is the more opinionated of the two
available answers: it means a CI job that adds `-fonts` and `-strict` starts failing the day a
brand face goes missing, which is the point of `-strict`. Stated in the usage text, the flag help,
the README and both doc twins, and pinned by a test across both subcommands and both notice kinds.

**Also worth recording — a promise dropped rather than kept.** The guide had said a face passing
the loader's checks would fail at *load* rather than at render. Rather than add a `cmap`/outline
check to make that true, the implementer narrowed the claim: these are the renderer's load-time
checks, and a face that passes is one this build can *read*, not one guaranteed to *draw*. Dropping
an over-promise is the right call over widening a validator to match marketing copy.

## Story 3 — decisions taken while planning

### A-15 — The designer must key a face exactly as `fontdir` does (cross-story constraint)

**Decision:** the designer keys an imported face by sfnt name ID 1, plus name ID 2 when the
subfamily is not `Regular` — byte-for-byte the same rule story 2's `fontdir.Set` uses.

**Reasoning, and this is the one decision in story 3 that reaches outside it:** in story 6 a
document may *name* its faces rather than carry them, and a host resolves those names from a
directory `fontdir.Set` built. If the designer keyed a face any other way — filename, PostScript
name, family record alone — the name the document carries would not be the name the host's
directory produces, and a name-only document would fail to resolve on exactly the deployment this
whole spec exists to serve. Two implementations, one rule; recorded because the coupling is
invisible from inside either story.

### A-16 — Follow the image-file seam rather than inventing a font one

**Decision:** font file access copies `src/image-file.ts` exactly — a `showOpenFilePicker` tier and
an `<input type="file">` tier, selected in `src/file/capability.ts`, injected as an `App` prop.

**Reasoning:** the designer has already answered "let the author pick a file" twice, for images and
for sample JSON, and both went through the same two-tier seam with capability selection. A third
answer would be a third thing to keep working across browsers. The seam also keeps the new module
clear of `src/file/file-access-contract.test.ts`'s bans (`showDirectoryPicker`, IndexedDB outside
`font-store.ts`).

### A-17 — The acknowledgement is one gesture per import, and it is not remembered

**Decision:** one acknowledgement per import gesture, covering every file picked together, and
never persisted across imports.

**Rejected:** per file (an author importing four cuts would answer four times — the way to make
people click through without reading); and remembered forever (a checkbox nobody sees again is not
an acknowledgement, it is a setting).

**Reasoning:** this dialog is the product's entire statement of position on a question it has
deliberately stopped answering, so it has to stay a conscious act. Per-gesture is the only framing
where that is true and the flow is still usable.

### A-18 — An absent copyright record stops being fatal, for author-supplied faces only

**Decision:** `faceCopyright` throws when name ID 0 is absent; that refusal does not apply to an
author-supplied face, which stores an empty string.

**Reasoning:** the throw exists because a face embedded into a `.folio` must state whose it is, and
the engine refuses a document that does not. Nothing is embedded in story 3 — the face only enters
the local store — and CAP-5 says an absent name record produces an empty value and a loadable
document. Refusing an import over a record the store does not need would decline a face the owner
has explicitly said the author is entitled to use. The constraint reappears in story 5, where the
record actually travels.

### A-19 — A cross-story gap I missed, carried to story 5 rather than patched here

**Finding:** an author can import a face whose binary declares no licence records — story 3's I/O
matrix explicitly admits it — and then never use it on a chain. `folio-go/component_commands.go:4914`
refuses a font record whose `licence`, `licenceText` or `copyright` is blank, and `parse.go`'s
`requireEmbeddedFaceLicence` refuses whitespace-only terms. Verified by reading both doors.

**Decision:** change nothing in story 3, and add the requirement to **story 5**.

**Reasoning:** nothing is embedded in story 3, so nothing here is actually broken — the face reaches
the store exactly as specified. The promise only becomes false at the moment something embeds it,
which is story 5, and story 5 already opens the very doors this refusal lives behind. Patching
story 3 instead would mean either refusing an import the frozen matrix admits, or showing the
author a warning that story 5 immediately makes untrue.

**And it fits CAP-6 rather than straining it.** The acknowledgement is precisely what stands in
place of a licence assertion for an author-supplied face. A document carrying an acknowledged
record should therefore satisfy `requireEmbeddedFaceLicence` with blank terms — the same override,
at the same doors, that CAP-6 already requires for the copyleft case. Story 5 now carries this.

**What I got wrong when planning:** SPEC.md CAP-5 says an absent name record produces "an empty
value and a loadable document", and I wrote story 3's matrix from it without checking that the
engine agreed. It does not, today. The spec's claim is one story premature, not wrong.

### A-20 — An imported family carries no script badge, and that is deliberate

**Decision:** accepted that an imported face stores `scripts: []`, so such a family shows no script
badge and is excluded by the writing-system filters.

**Reasoning:** populating it would mean classifying the face's coverage — inference about a binary
nobody vetted, which is exactly what this story's Boundaries forbid. The honest cost is that an
author who filters by writing system loses sight of their own imported family. Recorded rather than
patched because the alternative is worse than the symptom.

### A-21 — A family-name collision is refused at import, not made to work

**Decision:** accepted the implementer's deviation. An imported face whose binary declares a family
name the designer already offers — from the catalogue snapshot or the shipped set — is **refused
per file, by name**, rather than stored. The sentence says why: two different faces cannot share one
family name, and a face is named by its own binary, so this one cannot be renamed.

**What the review asked for instead:** make it reachable. The implementer investigated and reported
that both routes restructure `FamilySource` — either two browser rows share a family name (rows are
keyed by family, the family control stages by family, React keys collide), or the stored row
displaces the catalogue row, which takes the release's own cuts of that family out of reach and is
the same class of regression as the census bug this same review pass found.

**Why I accepted it:** the behaviour before this change was the worst of the three — the face was
stored, reported as kept, and then never appeared anywhere. Refusing it is honest, and it is a
strictly smaller lie than a success message for a face the author can never use. Restructuring
`FamilySource` is a real piece of work and does not belong inside a story about importing files.

**The cost, stated plainly for the owner:** an author who licenses their own cut of a family the
catalogue also publishes — a bought Kanit, a corporate build of Inter — cannot import it. That is a
genuine product limitation, not a technicality, and it is the kind of thing a brand-typeface user
may well hit. If it matters, it is its own story: let a stored family coexist with a catalogue
family of the same name, which means `FamilySource` carries the tier as part of its identity rather
than the family name alone.

### A-22 — Two smaller implementer calls, accepted

**Media type stays an extension check.** The review offered deriving it from the sfnt version
instead. Kept the extension, improved the sentence (*"rename it to the extension it really is and
pick it again"*). Reasoning given and accepted: deriving it would put the designer and `fontdir` on
two different rules for one fact, and story 3's whole D2 point is that those two must agree.

**The acknowledgement day is stamped at the answer, not at the pick.** `ImportedFace` no longer
carries `source` at all; `acknowledgedFace(face, today)` is the single writer, and `importFontFiles`
has no date parameter — so recording the pick's day is not merely avoided, it is unreachable. A
better fix than the one asked for.
