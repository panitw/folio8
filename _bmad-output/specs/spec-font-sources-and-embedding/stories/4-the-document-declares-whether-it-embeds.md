---
title: 'The document declares whether it carries its faces'
type: 'feature'
created: '2026-09-21'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'cdd28052ea10649cb3cde962b45f2933650bf041'
context:
  - '{project-root}/_bmad-output/specs/spec-font-sources-and-embedding/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Embedding is the right default for a `.folio` that travels alone and the wrong one for
an organisation rendering a thousand statements a day on servers it controls, where the same face
rides inside every document for no reason. The document has no way to say which it is.

**Approach:** A document-level setting, edited from the main template's page setup beside locale and
margins, travelling as a document-settings command exactly as `locale` and `utcOffset` do.
**Nothing reads it in this story** — the setting exists, round-trips and undoes, and saves behave
precisely as they do today. Story 6 gives it teeth.

## Decisions

- **D1 — No version move, and this is derived rather than chosen.** An unknown top-level key is
  **passthrough, not an error**: `parse.go:216-222` collects leftovers into `doc.Extra`,
  `model.go:90-93` says they are carried opaquely, and `docs/folio-format.md:104-105` states the
  rule outright — *"Keys the library does not know are carried through verbatim and written back on
  save."* Apply D-7.3.1's test: would a pre-`4.2` reader **refuse** the file or **render it wrong**?
  Neither. It ignores a setting that governs only what a *save* writes, renders identically, and
  preserves the key on re-save. So the ladder is untouched and no rank is added. This is **not** the
  `5.0` the owner ruled on — that is story 5's acknowledgement field and `spec-loop-section`'s loop
  element, which do change how a document is read.
- **D2 — Absent means "embed", and the key is omitted when it is true.** Every existing document
  embeds, so an absent field must keep meaning that; and emitting nothing in the default case is
  what keeps every golden digest unmoved (`byte_neutrality_test.go` moves if the serialized bytes of
  an existing fixture change). Follow the `unbreakableValues` shape at `serialize.go:145-154`.
- **D3 — Follow the two existing document-settings commands exactly.** Same arity-3 command shape,
  same "only writer of its field outside the loader" property, same save-previous / write /
  `canvas(t)` / restore-on-error body, same `"version":1` JSON built browser-side.

## Boundaries & Constraints

**Always:**
- The setting is **inert**. Nothing branches on it: not the save path, not embedding, not
  resolution, not the renderer. A grep for the new field must find the loader, the serializer, the
  command, the projection and the control — and nothing else.
- Every existing golden digest and fixture is byte-unchanged. If any moves, D2 was not honoured.
- The projection carries the field with no `omitempty`, as `Locale` and `UTCOffset` do, and the
  designer's exact-key guard is updated with it.
- `docs/folio-format.md`'s field table gains the key in the same change — `drift_test.go` compares
  the serializer's AST key set against that table in both directions.

**Never:**
- No ladder rank, no `SupportedMajor` change, no `SupportedVersion` change (D1).
- Do not make the setting apply to anything. No embedding decision, no stripping, no warning.
- Do not convert `applyPageSetup` to `applyCommands` in this story. The seam sends its
  document-settings commands one at a time and discloses that an accepted one stands when a later
  one is refused; changing that is a separate concern with its own undo semantics.
- No per-chain or per-face control. The setting is the document's, whole.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Existing document | No such key | Reads as "embeds"; save emits no key; bytes unchanged | N/A |
| Turned off | Author sets it false | Key emitted, sorted into place; round-trips byte-identically | N/A |
| Turned back on | Author sets it true again | Key omitted again; bytes identical to the original | N/A |
| Undo | Setting changed, then undone | Projection and document return to the prior value | N/A |
| Bad payload | Command with a non-boolean value | Located refusal naming the field; document unchanged | Command error |
| Wrong arity | Command with an extra or missing member | Refused, as the other two document-settings commands are | Command error |
| Older reader | A `4.1` reader opens a document carrying the key | Renders identically and preserves the key on re-save | N/A |
| Render unaffected | Same document rendered with the setting true and false | Byte-identical PDFs | N/A |

</frozen-after-approval>

## Code Map

- `folio-go/internal/template/parse.go:216-222` -- `extraFields(top, consumed)` → `doc.Extra`; no
  refusal for leftovers. `decodehelpers.go:116-134`; `model.go:90-93` states the passthrough rule.
  **This is the evidence for D1 — read it before touching the version machinery.**
- `folio-go/internal/template/version.go:115-118` `SupportedMajor = 4`, `SupportedVersion = "4.1"`;
  `versionRequiredByContent` at `:329-415`, `versionForSave` at `:260`, ranks at `:429-439`,
  `versionForRank` at `:450-460`. **None of this changes.**
- `folio-go/component_commands.go:3312-3335` -- the header comment covering both existing document
  settings; `setDocumentLocale` at `:3344-3371` is the body to copy (`componentFields(raw, 3)` at
  `:2049`, `commandString`, validate, save-previous, write, `canvas(t)`, restore on error);
  `setDocumentUTCOffset` at `:3387-3410`; dispatch at `:313-316`.
- `folio-go/internal/designer/page_setup.go:299-315` -- `CanvasProjection`, with `Locale` and
  `UTCOffset` at `:314-315` and the comment forbidding `omitempty`. Built in exactly one place:
  `folio-go/page_setup.go:450`.
- `folio-go/internal/template/serialize.go:131-163` `writeDocument` -- the `[]kv{}` (locale `:134`,
  utcOffset `:135`), optional appends, `extraKVs`. `writeObject` at `:53-72` sorts by key, so a new
  key lands in byte order automatically. `unbreakableValues` at `:145-154` is D2's shape.
- `folio-designer/src/document-settings-command.ts:28,37` -- where the command JSON is built.
- `folio-designer/src/App.tsx:5059` -- the PAGE SETUP JSX (one line); `Draft` at `:7329` (all-string
  fields); `draftFor` at `:7570`; `applyPageSetup` at `:2404` with its rationale at `:2354-2403` and
  the document-settings row table at `:2426-2429`.
- `folio-designer/src/engine-protocol.ts:441` -- the projection type; exact-key guard at `:798-806`.

**Tests that move:** `internal/template/drift_test.go:232,:269,:402` (serializer AST vs the
`folio-format.md` field table, both directions, plus maximal-document emission);
`canvas_projection_wire_test.go:65,:192,:390,:864,:605-614` (recorded key list, both key-set checks,
the JSON fixture, and the committed-member-non-zero check);
`internal/template/omitempty_test.go:63,:101,:125`; `internal/template/roundtrip_test.go:84,:158,:182`;
`document_settings_command_test.go` (including the "do not rotate" ledger at `:646-680`, a
cross-field non-interference suite a third field is expected to join);
`folio-designer/src/App.test.tsx:3605-3608` (exact command bytes, and "no other command sent").
**Tests that must NOT move:** `byte_neutrality_test.go:777,:1160` and every golden fixture — if they
move, the key is being emitted when it should be absent.

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/internal/template/model.go`, `parse.go`, `serialize.go` -- the field, its decode, and
      its emission omitted-when-default (D2).
- [x] `folio-go/component_commands.go` -- the command, on the `setDocumentLocale` body, plus dispatch
      and the shared header comment now covering three settings.
- [x] `folio-go/internal/designer/page_setup.go`, `folio-go/page_setup.go` -- the projection field and
      the one site that builds it.
- [x] `folio-designer/src/engine-protocol.ts`, `document-settings-command.ts` -- the projection type,
      the exact-key guard and the command builder.
- [x] `folio-designer/src/App.tsx` -- the control in PAGE SETUP, the `Draft` field, `draftFor`, and
      the row in `applyPageSetup`'s document-settings table.
- [x] `docs/folio-format.md` and its `.html` twin -- the field table entry, and a sentence saying the
      setting governs saving rather than rendering, with **no ladder row**.
- [x] tests -- one per I/O Matrix row, including the round-trip pair and the "render unaffected"
      byte-identity assertion.

**Acceptance Criteria:**
- Given an existing document with no such key, when it is loaded and saved, then the bytes are
  identical to before this change.
- Given the setting turned off and saved, when the file is reopened, then the setting is still off
  and a further save is byte-identical.
- Given the setting turned off and then on again, then the document's bytes equal the original.
- Given a document rendered with the setting true and with it false, then the PDFs are byte-identical
  — nothing reads it yet.
- Given `go test -count=1 ./...` in `folio-go`, then every golden digest and byte-neutrality check
  is unchanged, and the drift, projection-wire and document-settings suites pass.
  (`internal/text`'s `TestCorpusMeetsP6ExerciseFloors/P6g` is pre-existing and red at baseline.)
- Given `npm test` in `folio-designer`, then it passes, including the exact-command-bytes assertion.

## Implementation Notes

**The key is `embedFonts`, a plain Go `bool`, and absence is seeded rather than derived.**
`ParseDocument` sets `doc.EmbedFonts = true` before it reads any key, so there is no order in
which a parsed document is observed at Go's zero value; `writeDocument` appends the kv only when
the field is false. An authored `"embedFonts": true` therefore loads and canonicalises back to the
absent key, which is what makes the Matrix's "turned back on" row return the *original* bytes
rather than equivalent ones — the same normalisation `{"face": "X"}` -> `"X"` already performs.
The one residual hazard is disclosed in the field's own doc comment: a `template.Document` built as
a literal rather than parsed starts at `false` and would emit the key. Every production Document
comes from `ParseDocument`.

**`null` is refused at both doors, and it needed a line of its own at each.** `encoding/json`
admits the literal `null` into a `bool` destination *without an error*, leaving the zero value — so
`decodeBoolRaw` and `commandBool` would both have turned embedding OFF for an author who declared
nothing. The loader refuses it with `parse_bands.go`'s existing sentence shape ("must be true or
false — remove the key to …"); the command arm refuses it by naming the field, as the two string
arms refuse the empty string.

**No version machinery moved** (D1), and the argument is recorded where a reader of the format will
meet it: `docs/folio-format.md` gains the field-table row plus a paragraph under *Versions* saying
the key has no ladder row and why, and `_bmad-output/specs/spec-folio/folio-format.md` gains "A
FOURTH NON-EVENT" beside the three already there. Both copies of the document were edited because
`drift_test.go`, `goldenfixture_test.go` and `numeric_classification_test.go` all read the
`_bmad-output` copy while `docs/` is the published one; `docs/folio-format.html` carries the same
two additions.

**The designer draft holds the value as a string.** `Draft` is an all-string record and
`applyPageSetup`'s document-settings table is three `[typed, projected, build]` string rows; keeping
`embedFonts` as `'true'`/`'false'` (compared against `String(canvas.embedFonts)`) let the third row
join the set without turning that comparison into a type switch. `''` is the no-canvas seed, and the
checkbox is then unchecked *and disabled* — the same honesty the `Not set` locale placeholder
carries, rather than asserting "embeds" for a document that does not exist.

**Inertness is measured, not asserted.**
`TestTheEmbedSettingIsUnreadableFromTheRenderedBytes` renders one document twice, once with the
setting true and once false, and requires byte-identical PDFs. A later story that gives the setting
teeth must change that test deliberately. A grep for the field finds exactly nine non-test files:
the loader, the serializer, the model, the command, the projection struct, the one site that builds
it, the protocol type/guard, the command builder and the control.

**Fixture that moved on purpose:** `maximalFixture` now carries `"embedFonts": false`, because
`TestDriftASTMatchesRuntimeEmission` requires every key the serializer's AST can emit to be emitted
by that fixture at runtime. Every golden digest and both `byte_neutrality_test.go` anchors are
unchanged, which is D2 holding.

## Spec Change Log

## Review Triage Log

Pass 1 — three layers.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | `draft.embedFonts` can hold the no-canvas seed `''` while a canvas exists; `applyPageSetup`'s row comparison then fires and silently sends `embedFonts:false`, turning embedding off unasked. | **high** | Confirmed against `draftFor`'s empty-string branch and the row table. The one failure mode where an unrelated page-setup edit changes a font decision. |
| 2 | The render-inertness witness renders a document whose chain is **shipped faces only**, so the coupling most likely to leak — a read of the field on the carried-face path — would not redden it. | **medium** | Pre-verified: adding `if !t.doc.EmbedFonts` to the carried-face path leaves the test green, because neither branch has a carried face to drop. `embeddedFontTemplateJSON()` already exists in the same package. |
| 3 | No designer test renders the panel with an `embedFonts: false` projection; every one uses the `true` fixture. Seeding `draftFor` instead of reading it — a plausible slip, since `true` is the default — stays green, shows a ticked box for a document whose author turned embedding off, and then re-enables it on any Apply. | **medium** | Pre-verified by mutation. The only `embedFonts: false` in the designer is a guard test that never renders `App`. |
| 4 | D1's whole argument rests on unknown-key passthrough, and **no test exercises it**. The "Older reader" matrix row is claimed by a test whose reader *does* know the key. | **medium** | Confirmed. The no-version-move decision is the most consequential in this story and its premise is untested. |
| 5 | `applyPageSetup`'s block comment still says "the two DOCUMENT-SETTINGS commands", "awaits up to FIVE times", "THE TWO ROWS GO FIRST"; the diff adds a third row to the loop that comment governs. `engine-protocol.ts:795` says "THE TWO DOCUMENT-SETTINGS CLAUSES" directly above the new third clause. | **medium** | Confirmed. These are the comments a reader trusts to know the order of writes. |
| 6 | A parallel three-arm rotation ledger was added and the two-arm one left in place, so a fourth setting has two ledgers to choose between and the older asserts non-interference over a set that is no longer the set. | **medium** | Confirmed. The Code Map said the third field "is expected to join" the existing ledger. |
| 7 | The format doc says "only `false` ever appears in a file" but never says an authored `true` is **accepted** and canonicalised away — so `true` reads as illegal to another implementation. It is also silent that the value must be boolean and that `null` is a load error. | **medium** | Confirmed in both doc copies. The trap the implementation spends two code paths defending is documented only in Go comments. |
| 8 | The two doors' refusal sentences diverge — the loader offers a remedy, the command does not — and neither wording is pinned, so the deliberate `null` sentence can be deleted without a test going red. | **low** | Confirmed. |
| 9 | `maximalFixture` quietly became a non-embedding document, with no comment saying why. Once story 6 makes the setting behavioural, every test using it inherits "do not embed" without anyone choosing it. | **medium** | Confirmed. A latent trap aimed squarely at story 6. |
| 10 | The UI's inertness note ("Nothing acts on it yet…") has no assertion, so when story 6 gives the setting teeth the note can survive as a lie — while the engine's inertness *is* measured. | **low** | Confirmed. |
| 11 | The new checkbox is emitted after `property-grid` closes, wedging a font note between the document settings and the page dimensions, though `applyPageSetup` treats the three as siblings. | **low** | Confirmed. |
| 12 | `App.css` duplicates `label.page-break-setting`'s two rules verbatim, below the comment that explains them, so the new rules read as uncommented. | **low** | Confirmed. |
| 13 | A malformed-value probe labelled "the key absent" actually sends a **misspelled** key at arity 3. | **low** | Confirmed. The name misleads about which case is covered where. |
| 14 | The Code Map's "tests that move" list named four suites the diff never touches. | **low — not patched** | True. My planning overstated the blast radius; `drift_test.go`, `omitempty_test.go` and `roundtrip_test.go` did not need changing because the field is omitted when default. Recorded in `DECISIONS.md` A-25 rather than sent to the implementer, since the fix is to my spec. |

**Routing.** No `bad_spec` and no `intent_gap`. #1–#13 route to **patch**; #14 is mine and is
recorded, not patched.

## Design Notes

D1 is the decision a reviewer should push on hardest, so here is the whole argument in one place.
The owner ruled that this spec's format additions ride the `5.0` that `spec-loop-section` opens.
That ruling is about constructs that change how a document is **read** — a `loop` element, a `$.`
path, an acknowledgement the engine must honour at both licence doors. This key changes only what a
**save writes**. An older reader meets it, does not recognise it, carries it through verbatim, and
renders the same page set it always would. Under the format's own ladder rule — *"a file declares
the lowest version its own content requires"* — it requires nothing. Adding a rank would make the
document declare a version it does not need, which is the mirror of the error D-7.3.1 guards
against.

## Verification

**Commands:**
- `cd folio-go && go vet ./... && go test -count=1 ./...` -- expected: pass except the pre-existing
  `internal/text` `P6g` corpus floor.
- `cd folio-go && go test -count=1 -run 'TestTargetRenderHash' -tags=matrix .` -- expected: pass.
- `cd folio-designer && npm test` -- expected: pass.
- `cd folio-designer && ./node_modules/.bin/oxlint && ./node_modules/.bin/tsc -p tsconfig.app.json
  --noEmit` -- expected: both exit 0. Run the binaries directly; `npm run lint` reports a spurious
  failure in this environment, and `tsconfig.test.json` is `folio-js`'s, not the designer's.
