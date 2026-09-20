---
title: 'Story 6: Apply several commands as one undoable unit'
type: 'feature'
created: '2026-09-20'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: []
baseline_commit: 'd8941b41e5f802adf2a285fc0859149e4bc53aaf'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A single author action sometimes needs more than one command — pressing **B** on a
family whose bold is held but not embedded must embed the face *and* set the property. `Apply`
decodes exactly one command object (`internal/wasm/engine.go:388-399`) and there is no compound,
batch or transaction kind anywhere in the repository, so that action would cost the author two
undo entries. The owner chose a general mechanism over a purpose-built fused command, knowing the
cost.

**Approach:** Mint one new command kind that carries an ordered list of ordinary commands and
applies them to a single candidate document. It commits as one unit or not at all, and it produces
exactly one revision, one undo entry and one history step. Story 2 is its first consumer; this
story ships the mechanism and its proof, and changes nothing the designer sends today.

## Boundaries & Constraints

**Always:**
- The unit is **all-or-nothing**. If any member refuses, the whole unit refuses and the document,
  its canonical bytes, its revision and both history branches are exactly as they were.
- A member is an **ordinary command**, unchanged and unaware it is in a group. Members are applied
  in the order given, each seeing the previous member's effect.
- Members keep their own `version: 1`, their own arity check, and their own located refusal. A
  member's refusal propagates verbatim — same `ElementID`, same `DataPath`, same `Message`.
- The unit re-establishes atomicity at the **public** seam (`applyComponentCommand`), the way
  `applyTableColumnCommand` (`component_commands.go:330-360`) already does, because 20+ test files
  call that seam directly and must be able to prove the property without the wasm `Engine`.
- The existing no-op rule still governs: a unit whose **net** canonical bytes are unchanged commits
  nothing and leaves revision and both history branches untouched (`engine.go:406-414`).
- Bounds and refusals are stated in the unit's own words and proved by a red-proof that edits the
  constant or the guard.

**Never:**
- **No change to what the designer sends.** No existing author action becomes fused here. No
  `folio-designer/src` behaviour change; no new designer builder without a consumer.
- **No new engine operation, no protocol version bump, no `.folio` format change.** The unit rides
  the existing `"command"` operation and its 8 MiB envelope.
- **Not a reversal of the two-commands separation.** Embedding and property-setting stay separate
  commands; only the number of undo entries a *unit* of them produces changes. See **D-6.5** below
  for why that separation is *not* D-16.5's ruling and what is written where.
- No inverse commands, no diffs, no serialized history. Undo stays whole-snapshot replay
  (`engine.go:432-435`).
- No silent widening of an existing guard to accommodate the unit.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Two members both succeed | unit of `addFontChainEntry` + `updateComponentProperties` | both effects present; `revision` advances by exactly 1; `canUndo` true; one undo restores the document to before *both* | N/A |
| Later member refuses | unit whose 2nd member names a missing element | unit refused; template bytes, revision, `canUndo`/`canRedo` unchanged at both the `Engine` and the public seam | member's own `*ComponentCommandError` verbatim |
| First member refuses | unit whose 1st member is malformed | as above; nothing applied | member's own refusal |
| Net no-op | unit whose members cancel (set a value and set it back) | no commit: revision, bytes and both history branches unchanged | N/A |
| Duplicate key inside a member | a member object declaring `kind` twice | refused before any member runs | `refuseDuplicateCommandKeys` walks arrays already (`component_commands.go:119-123`) |
| Unknown kind inside a unit | member `kind` not in the closed switch | unit refused, nothing applied | `folio8: unknown component command` |
| Over-length unit | members beyond the bound | refused before any member runs | located refusal naming the bound |
| Empty / under-length unit | `commands: []` | refused | located refusal |
| Wrong arity on the unit itself | a 4th key beside `kind`, `version`, `commands` | refused | `componentFields` plain error |
| Undo then redo | unit committed, undo, redo | one undo reverses the whole unit; one redo reapplies the whole unit; revision monotonic throughout | N/A |
| Unit inside a unit | a member whose kind is the unit's own | refused before any member runs | located refusal naming nesting |
| `pageSetup` member | a member of kind `pageSetup` | refused | existing `default:` — `folio8: unknown component command` |
| Stale `moveComponents` member | unit containing a `moveComponents` whose `expectedRevision` is not `e.revision` | refused by the same fence a bare move meets; bytes, revision and both history branches untouched | `folio8 wasm: group move refers to an outdated revision` |
| Current `moveComponents` member | same, `expectedRevision == e.revision` | admitted, applies as part of the unit | N/A |

## Decisions

Ruled at the Open Questions gate (2026-09-20). Recorded here verbatim because the spec is the
implementer's only source of truth.

- **D-6.1 — Component commands only.** The unit is one case in `applyComponentCommand`'s closed
  switch; `Engine.Apply`'s dispatch is untouched and a `pageSetup` member is refused by the existing
  `default:`. *Reason:* story 2 needs two component commands, nothing foreseeable fuses page setup
  with a component change, and the wider form stays available later without changing this wire
  shape. **The limit is deliberate, not accidental** — say so in the code, and name the wider form
  (the unit moving into `internal/wasm/engine.go` and replicating the two-way dispatch) as the
  extension path.

- **D-6.2 — OWNER: the staleness fence walks the unit's members.** A `moveComponents` inside a unit
  has its `expectedRevision` compared to `e.revision` exactly as a bare move does. Generality was
  chosen over the simpler stated limit, having been shown both.
  **Design obligation attached to this ruling:** *do not let a second package acquire its own
  knowledge of a unit's internal shape.* The package that defines the unit (`folio8`) owns the
  answer to *"what commands does this carry"* and exposes it through the existing
  `internal/designer` function-variable bridge; `checkMoveRevision` asks that question and keeps
  its own per-command question unchanged. **If a second parser for a unit's members appears
  anywhere, stop** — that is the defect this ruling was accepted in spite of, and duplicating the
  structure is what would make it real. A command that is not a unit carries itself, so one command
  and a unit of commands read the same way.

- **D-6.3 — Nesting is refused.** A flat list is exactly as expressive, and every bound and count
  question would otherwise be re-asked per level.

- **D-6.4 — Bounds: maximum 64, minimum 1.** A stated constant with its own refusal, matching
  `maxCanvasFontChainEntries` as the nearest neighbour — not an implicit limit inherited from the
  8 MiB envelope. An empty `commands` array is refused; a unit of one is admitted, so a caller
  assembling a list whose length it does not know in advance needs no special case.

- **D-6.5 — The D-16.5 premise was false and is corrected.** The dispatch stated that D-16.5 ruled
  a pick's embed and its property commit are two commands with two undo entries, never fused.
  **Measured:** D-16.5 (`_bmad-output/implementation-artifacts/epic-8-15-decision-log.md:3692`,
  summarised `epic-16-decision-log.md:20`) rules on variable-only families, deriving the ones worth
  having, and refusing browser-side instancing. It says nothing about command fusion or undo
  entries. The claim originates in `stories/2-embed-a-cut-on-first-use.md:95` and travelled from
  there into this story's dispatch note. The three citations of D-16.5 in shipped code
  (`folio-designer/src/App.tsx:2596`, `folio-designer/src/font-index.ts:205`,
  `folio-go/pick_declares_cuts_ext_test.go:156`) are all about embedding timing and browser
  instancing. The owner's actual decision is at
  `_bmad-output/specs/spec-install-all-face-cuts/.memlog.md:82`.
  **Ruled:** write the clarifying note in this story's own Go code beside the new kind, citing the
  memlog decision and **not** D-16.5. Correct `stories/2-embed-a-cut-on-first-use.md:95` so story 2
  does not inherit the false premise. **Do not annotate the three real D-16.5 citation sites** — a
  note there would make the record less true.

- **D-6.6 — Full spec kept.** The spec trips the 1,600-token gate at ~4,720 tokens. One mechanism,
  no second shippable deliverable, and the threshold has never been met on this project (recent
  peers run 16,000–26,000 tokens).

</frozen-after-approval>

## Code Map

- `folio-go/component_commands.go:212-327` — `applyComponentCommand`: the public command door.
  Duplicate-key refusal (`:216`), single-decode with trailing-data refusal (`:219-228`),
  `version: 1` check (`:229`), `kind` read (`:232-235`), then the **closed switch** (`:236-327`)
  ending in `default: "folio8: unknown component command"`. The new kind's case goes here.
- `folio-go/component_commands.go:330-360` — `applyTableColumnCommand`: **the pattern to copy.**
  Serialize `t`, reparse into `working`, apply, re-serialize, reparse, project, and only then
  `t.doc, t.derivedFooters = installed.doc, installed.derivedFooters`. This is how the public seam
  is made atomic. Note it projects with `canvas(installed)`; the unit must thread `fonts ...FontSet`
  through to members that take them (`moveComponents` does).
- `folio-go/component_commands.go:1790-1795` — `componentFields(raw, want)`: exact key count,
  including `kind` and `version`. A `{kind, version, commands}` unit is `want = 3`.
- `folio-go/component_commands.go:99-123`, `:67-78` — `refuseDuplicateCommandKeys` and
  `maxCommandKeyScanDepth = 10000`. **It already walks arrays and nested objects**, so a unit's
  members inherit duplicate-key protection for free. Assert this rather than re-implement it.
- `folio-go/component_commands.go:20-31` — `componentFailure` / `*ComponentCommandError`. Use it for
  the unit's own refusals. Host cuts: `Message` 512, `ElementID` 128, `DataPath` 256
  (`:3904-3905`). Do not mint a diagnostic registry code — the census test structurally forbids it
  for this error type.
- `folio-go/internal/wasm/engine.go:374-430` — `Engine.Apply`. **Already a transaction:** it parses
  a fresh candidate from `e.bytes` (`:384`), dispatches (`:395-399`), serializes, applies the no-op
  rule (`:411-413`), reparses, projects, and only then `pushUndo` + `install` (`:426-428`). Its
  dispatch needs no change under D-6.1. Do not touch `install` or `pushUndo`.
- `folio-go/internal/wasm/engine.go:141-153` — `checkMoveRevision`: today it unmarshals the **outer**
  command only and fires on `intent.Kind == "moveComponents"`. Under **D-6.2** it must instead ask
  `folio8` what the command carries and apply its existing per-command question to each. Note
  `group_movement.go:120` requires `expectedRevision` present and bounded but never checks it is
  current — the currency check lives only here.
- `folio-go/internal/designer/bridge.go:20-45` — the function-variable bridge, and **the seam D-6.2
  requires**. Its doc comment explains the pattern: implementations stay unexported in package
  `folio8` and assign these variables from `folio8`'s own `init`. Add one variable here for "what
  commands does this command carry"; `internal/wasm/engine.go` already imports `designer` and calls
  through it. Wire it in `folio-go/designer_bridge.go:19-38` beside the others.
  **`folio8` owns the one parser for a unit's members; nothing else may re-derive that structure.**
- `folio-go/internal/wasm/engine.go:29-36`, `:292-297`, `:469-486` — `Snapshot`, the
  `CanUndo: len(e.undo) > 0` / `CanRedo: len(e.redo) > 0` projection, `install`'s `e.revision++`,
  and `historyLimit = 100` with oldest-drops. **Settled, not an open question:** one `Apply` call is
  one `install`, so a unit is already exactly one revision and one undo entry, and the designer
  needs no change to read `canUndo`/`canRedo`.
- `folio-go/internal/wasm/engine_test.go` — the undo/history idiom to copy, in particular
  `TestEngineRefusedFontChainCommandLeavesByteRevisionAndHistoryUntouched` (`:899`),
  `TestEngineFontChainCommandsAdvanceExactlyOneRevisionAndOneUndoStep`, and
  `TestEngineNoOpDoesNotChangeHistoryRevisionOrRedo`.
- `folio-designer/src/command-json.ts:115-116` — `commandBytes(kind, fields)`, the sole envelope
  builder; `jsonArray` is already available. **Reference only — do not add a builder in this story.**
  TypeScript has no command-kind allowlist, so nothing on that side must learn the new kind.
- `folio-designer/src/engine-bounds-mirror.test.ts` — the Go↔TS bound census walks a hardcoded
  `pairs` list, so a Go-only bound does not join it. **Do not add a TS mirror without a TS consumer.**

**Do not change:** `folio-designer/src/**`, `docs/folio-format.md` (the command protocol is not
documented there and this story does not start documenting it), any existing command handler, the
`.folio` format, `Snapshot`, `install`, `pushUndo`/`appendBounded`.

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/component_commands.go` -- Add the new command kind as one case in the closed switch
      (D-6.1): a `componentFields(raw, 3)` door reading `commands`, bounded 1..64 (D-6.4), refusing
      nesting (D-6.3), applying each member in order by re-entering the command door -- so a member
      is literally an ordinary command and keeps its own arity, version and located refusal with no
      per-kind special-casing.
- [x] `folio-go/component_commands.go` -- Extract the member list through **one** function that is
      the sole place knowing a unit's internal shape, used both by the applier and by the bridge
      answer D-6.2 requires; a command that is not a unit carries itself -- so the fence and the
      applier can never disagree about what a unit contains.
- [x] `folio-go/component_commands.go` -- Wrap the unit in the `applyTableColumnCommand`
      serialize/reparse/copy-back pattern and thread `fonts ...FontSet` to members -- so the public
      seam is atomic for the 20+ test files that call it directly, not only `Engine.Apply`.
- [x] `folio-go/internal/designer/bridge.go` + `folio-go/designer_bridge.go` -- Declare and wire the
      one new function variable that answers "what commands does this command carry" -- D-6.2's
      design obligation: `internal/wasm` asks rather than parsing a unit itself.
- [x] `folio-go/internal/wasm/engine.go` -- Rewrite `checkMoveRevision` to walk the carried commands
      and apply its existing `moveComponents` currency rule to each, leaving that rule's wording and
      the malformed-command behaviour unchanged -- D-6.2, so a move inside a unit meets the same
      fence as a bare move.
- [x] `folio-go/component_commands.go` -- Write the comment beside the new kind recording what this
      does and does not change, per **D-6.5**: embedding and property-setting stay separate commands,
      only the undo-entry count for a unit of them changes. Cite the owner decision at
      `.memlog.md:82`, **not** D-16.5. Name the D-6.1 extension path in the same comment.
- [x] `folio-go/<new>_test.go` -- Cover every row of the I/O matrix at the public seam, including
      the refusal rows asserting the template's serialized bytes are byte-identical after a refused
      unit -- the all-or-nothing claim is only worth what its refusal rows prove.
- [x] `folio-go/internal/wasm/engine_test.go` -- Add the history tests: one unit advances the
      revision by exactly one and is reversed by exactly one undo and reapplied by exactly one
      redo; a refused unit leaves revision, bytes, `canUndo` and `canRedo` untouched; a net-no-op
      unit commits nothing. Copy the shape of the three named tests in the Code Map.
- [x] `folio-go/internal/wasm/engine_test.go` -- Cover D-6.2 directly: a unit carrying a **stale**
      `moveComponents` is refused and leaves bytes, revision and both history branches untouched,
      and a unit carrying a **current** one is admitted -- prove the fence rather than asserting it,
      and the pair keeps the test from passing on a fence that refuses everything.
- [x] `folio-go/<new>_test.go` -- Red-prove each new guard by **deletion or mutation of the guard
      itself**: the bound constant, the nesting refusal, the minimum, and the member walk in
      `checkMoveRevision`. Record which test caught which mutation; a guard no mutation reds is a
      guard with no test.
- [x] `_bmad-output/specs/spec-install-all-face-cuts/stories/2-embed-a-cut-on-first-use.md` --
      Correct the D-16.5 misattribution at line 95 if Open Question 5 is ruled **B** -- planning
      artifact only, no code change.

**Acceptance Criteria:**
- Given a loaded document and a unit of two commands that both succeed, when it is applied, then
  both effects are present, `revision` has advanced by exactly one, and `len(e.undo)` has grown by
  exactly one.
- Given that unit committed, when undo is called once, then the document is byte-identical to
  before the unit — not to a state between its members — and when redo is called once, both effects
  return.
- Given a unit whose second member refuses, when it is applied through `applyComponentCommand`,
  then `SerializeTemplate(t)` is byte-identical to its value before the call and the returned error
  is the member's own `*ComponentCommandError` with its original `ElementID`, `DataPath` and
  `Message`.
- Given the same unit applied through `Engine.Apply`, when it refuses, then `Snapshot().Revision`,
  `CanUndo` and `CanRedo` are unchanged and `e.bytes` is untouched.
- Given a member object that declares one key twice, when the unit is applied, then it is refused
  before any member runs, by the existing duplicate-key guard rather than by new code.
- Given a unit carrying a `moveComponents` whose `expectedRevision` is not the engine's current
  revision, when it is applied, then it meets the same fence and the same message a bare stale move
  meets, and nothing is committed; and given the same unit with a current `expectedRevision`, it is
  admitted.
- Given the source tree, when the story is complete, then exactly one function knows how to read a
  unit's member list, and `internal/wasm` reaches it through the `internal/designer` bridge rather
  than decoding the unit's own structure.
- Given `git diff --name-only`, when the story is complete, then no file under
  `folio-designer/src/` appears.

## Implementation Notes

**The kind is `applyCommands`.** Wire shape exactly as the Design Notes state it:
`{"kind":"applyCommands","version":1,"commands":[ ...ordinary command objects... ]}`. Nothing in
`folio-designer/src` learned it; there is no TS builder and no TS bound mirror, per the Code Map.

**Where it lives.** `folio-go/component_commands.go` — one `case unitCommandKind:` at the end of the
closed switch, then `unitCommandKind`, `unitCommandPath`, `minUnitCommands`/`maxUnitCommands`,
`carriedCommands` and `applyCommandUnit` immediately after `applyTableColumnCommand`, whose
serialize/reparse/copy-back shape `applyCommandUnit` wears. Members are applied by RE-ENTERING
`applyComponentCommand`, so each is literally an ordinary command with its own duplicate-key guard,
arity, version and located refusal, and nothing in the unit has to be kept in step with the
vocabulary. Nesting is refused before any member runs, so the re-entry is exactly one level deep.

**The one parser (D-6.2).** `carriedCommands(command []byte) (members [][]byte, unit bool, err error)`
is the only function in the module that reads a unit's member list —
`TestExactlyOneFunctionReadsAUnitsMemberList` holds that structurally on two axes: exactly one
non-test `.go` file under `folio-go/` may contain the `"commands"` literal, and that file may
contain it exactly once, so a second reader inside `component_commands.go` reds it too.
`internal/designer/bridge.go` gained `CarriedCommands`, assigned in `folio-go/designer_bridge.go`'s
`init`, and `internal/wasm/engine.go`'s `checkMoveRevision` loops over
`designer.CarriedCommands(command)` applying its unchanged per-command rule to each. A command that
is not a unit — and anything that does not decode as a command object — carries itself, so the
bare-move path and the malformed-bytes path are the same code with no branch.

**The fence must not speak for the door.** `CarriedCommands` returns `(members, unit)`, and the
`unit` flag is load-bearing. `checkMoveRevision` runs BEFORE the command door has judged anything,
so a member it cannot decode is skipped and left to the door — otherwise the Engine's unlocated
`folio8 wasm: command is malformed` reaches the browser in front of the member's own located
refusal, and "a member's refusal propagates verbatim" is false at the only seam that ships. Only a
command that carries ITSELF and does not decode is the fence's own malformed-input case.
`carriedCommands` also decides unit-ness exactly as the door does — `version` first, then `kind` —
so a `version: 2` object naming the kind is an unknown command, not a unit the fence may have
opinions about. Both of these shipped wrong in the first pass and are now pinned by
`TestEngineRefusedCommandUnitRefusesExactlyAsTheDoorDoes`, which measures the Engine's refusal
against what `applyComponentCommand` says about the same bytes rather than against a string copied
into the test.

**Two wordings, not one.** A unit with three keys whose third is not `commands` satisfies
`componentFields(raw, 3)` and then has no member list at all; it is told the field is absent, not
that its list is not an array — a list it never sent. Both wordings live in `carriedCommands`, for
the same reason the parsing does.

**Red-proof ledger.** Each mutation applied alone to the shipped source, the suite run, the source
restored:

| Mutation | Test that went red |
|---|---|
| `maxUnitCommands` 64 → 65 | `TestCommandUnitBoundsAreStatedAndProvedOnBothSides` |
| `minUnitCommands` 1 → 0 | `TestCommandUnitBoundsAreStatedAndProvedOnBothSides`, `TestCommandUnitRefusesAnUnreadableMemberList`, `TestEngineRefusedCommandUnitLeavesByteRevisionAndHistoryUntouched` |
| nesting refusal deleted | `TestCommandUnitRefusesAUnitInsideAUnit` |
| unreadable-member-list refusal deleted | `TestCommandUnitRefusesAnUnreadableMemberList` |
| `checkMoveRevision` reverted to decoding only the outer command | `TestEngineCommandUnitMeetsTheSameGroupMoveRevisionFence`, `TestExactlyOneFunctionReadsAUnitsMemberList` |
| members applied to the caller's template instead of the working copy | six public-seam tests incl. `TestCommandUnitMemberRefusalPropagatesVerbatimAndCommitsNothing`, plus two engine tests |
| missing-`commands` wording collapsed into the unreadable one | `TestCommandUnitRefusesAMissingMemberListInItsOwnWords` |
| fence stops skipping members it cannot decode | `TestEngineRefusedCommandUnitRefusesExactlyAsTheDoorDoes/a_member_that_is_not_a_command_object` |
| `carriedCommands` decides unit-ness from `kind` alone | `TestEngineRefusedCommandUnitRefusesExactlyAsTheDoorDoes/a_version_the_door_does_not_know...`, `TestDesignerBridgeCarriedCommandsAnswersForEveryShape` |

The bounds test deliberately writes `64` and `1` out as its own constants rather than sizing its
input from `maxUnitCommands`/`minUnitCommands`; a test that read the constants would have followed
them and the first mutation would not have red. The unreadable-member-list refusal initially red
NOTHING — an unreadable list decodes to no members and fell through to the minimum's refusal — so
the test was strengthened to assert the wording rather than merely the refusal.

**Census kept in step.** `folio-go/designer_bridge_test.go`'s hand-written `designerBridge()` map
gained `CarriedCommands`; `TestDesignerBridgeListsEveryVariable` fails without it. That file also
gained `TestDesignerBridgeCarriedCommandsAnswersForEveryShape`, which exercises the wrapper's
branches — ordinary command, undecodable bytes, a real unit, an unreadable list, an absent list, and
a version the door does not know — rather than only asserting the variable is non-nil.

**D-6.5 written where it was ruled.** The clarifying note is on `unitCommandKind` in
`component_commands.go`, citing the decision by its stable label — `.memlog.md`'s "Story 2 Q-undo
(OWNER)" — and quoting its words, never a line ordinal into an append-only file, and explicitly
disclaiming D-16.5. The same comment
names D-6.1's extension path (move the unit into `internal/wasm/engine.go` and replicate the two-way
dispatch there). `stories/2-embed-a-cut-on-first-use.md`'s Open Question 2 is rewritten as a
correction; the three real D-16.5 citation sites in shipped code were left alone.

**Verification deltas from the spec's stated expectations.** `folio-designer` lint reports **8**
pre-existing `only-export-components` warnings, not the 4 the Verification block predicted — none in
a file this story touched, and `git status` confirms no path under `folio-designer/src/` changed.
`npm test` is 89 files / 2056 tests, all green. `go test ./...` is green except the known
`TestCorpusMeetsP6ExerciseFloors/P6g` floor.

## Spec Change Log

## Review Triage Log

Pass 1 (2026-09-20). Three layers: blind-hunter (12 findings), edge-case-hunter (7), verification-gap
(2 gaps + 4 other). Every finding gets a row. Claims verified by probe against the working tree at
the shipping seam, not read off the diff.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | A unit member that is not a JSON object is refused by the fence's words, not the member's. Raised by all three layers. | **medium** | REPRODUCED. `Engine.Apply` on `{"kind":"applyCommands","version":1,"commands":[7]}` returns `folio8 wasm: command is malformed`; the identical bytes through `applyComponentCommand` return `folio8: component command is malformed`. `checkMoveRevision` decodes every carried member and treats a decode failure as the whole command being malformed, short-circuiting before the unit door runs. The located `COMPONENT_INVALID` the designer's error UI reads is lost, and the spec's "a member's refusal propagates verbatim" is false at the only seam that ships. |
| 2 | `carriedCommands` decides unit-ness from `kind` alone, so the fence judges members of a command the door will refuse. | **medium** | REPRODUCED. A `version: 2` unit carrying a stale move returns `folio8 wasm: group move refers to an outdated revision` from `Engine.Apply`, where `applyComponentCommand` returns `folio8: unknown component command`. Same root cause as #1: the fence acts on a structure whose validity the door has not yet established. |
| 3 | The `Engine.Apply` refusal test never inspects the error value, only that one exists. | **medium** | Confirmed by reading `TestEngineRefusedCommandUnitLeavesByteRevisionAndHistoryUntouched`: its rows assert non-nil, then bytes/revision/CanUndo/CanRedo. Nothing would red if member refusals started being wrapped — which is exactly what #1 shows already happens. |
| 4 | A missing `commands` key is reported as "must carry an array of commands". | **low** | Real: `componentFields(raw, 3)` admits any third key, so a unit with a wrongly-named third key is told its list is not an array when it sent no list at all. Direct correction, no new surface. |
| 5 | Shipped Go source cites `.memlog.md:82`, a line ordinal into an append-only planning file. | **medium** | Real and pointed: this story exists *because* a citation went wrong. A line number into `_bmad-output` will drift and the next reader follows it to the wrong line. Developer-facing; named harm is the same class of failure D-6.5 corrects. |
| 6 | `TestExactlyOneFunctionReadsAUnitsMemberList` scans files for the bare literal `"commands"`, so it measures files not functions. | **medium** | Real: it would stay green with five functions in `component_commands.go` reading `raw["commands"]` — the thing its own message forbids. Developer-facing: the D-6.2 single-reader discipline is the story's load-bearing invariant and its guard is weaker than it reads. |
| 7 | `TestCommandUnitBoundsAreStatedAndProvedOnBothSides`' docstring claims it proves both sides of the minimum; it asserts only 64-admitted, 65-refused, 0-refused. | **low** | Confirmed by reading the test. The minimum's admitting side lives in `TestCommandUnitOfOneIsAdmitted`. Direct correction to a comment. |
| 8 | `designer.CarriedCommands`' `!ok` branch has no test. | **medium** | Real: `designer_bridge_test.go` only asserts the variable is non-nil. A defensive branch nothing exercises is a branch nothing protects; it is also the branch #1's fix has to get right. |
| 9 | Two `moveComponents` in one unit both pass the fence against the same pre-unit revision. | **maybe-false** | Cannot settle from the diff. The author composed both members against revision N knowing the order, so compounding may be exactly what a unit means; the reviewer's "a document the author was never looking at" assumes the opposite. Nothing sends units at all today. Would be settled by an owner ruling on whether a unit may carry more than one group move, once a consumer exists. Claim, if true, is medium → deferred rather than rejected. |
| 10 | `GroupMovePreview` shares `checkMoveRevision`, so a unit passes the fence and then fails in `PreviewComponentMove`. | **low** | Not caused by this change: before it, a unit's outer kind was not `moveComponents`, so the fence passed it and preview refused with the same arity error. The change only made that path stricter (a stale member now refuses earlier). Nothing sends units to preview. Fix would add branches for an unreachable case. |
| 11 | A 64-member unit costs roughly 65 serialize/parse/canvas round trips; the bound counts members, not bytes. | **low** | Real as a scaling property — each member re-enters the door and `applyFontChainCommand`/`applyTableColumnCommand` each do their own round trip. Not reachable in everyday use: the only planned consumer sends two members. Recorded rather than fixed; the fix is a redesign, not a correction. |
| 12 | `applyCommandUnit` holds the decoded `raw` yet calls `carriedCommands(command)`, re-unmarshalling the whole command. | **low** | Real but cosmetic. The fix restructures the single-reader function the D-6.2 discipline rests on, which is more than a direct correction for a cost nobody can observe. |
| 13 | The TypeScript builders all return `ArrayBuffer`, so a unit cannot be assembled from them without decoding bytes back to text — `applyCommands` is unreachable from the designer. | **medium** | Real and verified in `command-json.ts` / `component-command.ts`. Not a defect in this story, which deliberately ships no TS builder, but it is a constraint story 2 will hit on its first day. Deferred and reported. |
| 14 | Story 2's Open Question 1 still carries its pre-transaction reasoning, though `.memlog.md:84` says the command-shape question is superseded and must be re-asked after story 6 lands. | **medium** | Real: the memlog says so in those words, and story 6 has now landed. Outside this story's fence — the ruling authorised correcting the D-16.5 premise at line 95 only, and re-opening story 2's command-shape question is story 2's planning, not story 6's code. Deferred and reported. |
| 15 | Story 2's edit claims the premise "is corrected in both places" but the second place is not in the diff. | **false** | The correction exists: D-6.5 is written in full in story 6's own spec file. That file is untracked and was excluded from the diff handed to the reviewers, so this is an artefact of what they could see, not a missing edit. |
| 16 | `folio-go/internal/wasm/zz_probe_test.go` is an untracked scratch test left in the tree. | **false** | Verified absent: `git status` and `git ls-files --others` are both clean of it, and the directory listing shows no such file. It was a *parallel reviewer's* own probe, live in the tree while another reviewer looked. Transient contamination, not an artefact of this change. |

**Grouping and routing.** #1, #2, #3 and #8 share one root cause — the fence acts on a unit structure
the door has not yet validated, and the bridge answer does not tell it what it is looking at — and
route together as **patch** (highest member: medium). #4, #5, #6, #7 are independent **patch**
entries. #9, #11, #13, #14 route to **defer**. #10 and #12 are rejected as `low` with a complex fix.
#15 and #16 are rejected on their refutations. No `intent_gap` and no `bad_spec`, so no loopback;
`review_loop_iteration` stays 0.

## Design Notes

**Why the mechanism is nearly free, and why that is the point.** `Engine.Apply` is already a
transaction: it never mutates live state until every step has succeeded, so "all-or-nothing" is not
something this story builds — it is something this story must avoid breaking. Likewise "one undo
entry" falls out of `pushUndo` and `install` being called once per `Apply` call. The genuine work is
therefore the *door*: a kind, its arity, its bounds, its refusals, and tests that prove the
properties rather than assume them.

Shape of the wire object (member commands are ordinary command objects, unchanged):

```json
{ "kind": "<name>", "version": 1, "commands": [
    { "kind": "addFontChainEntry", "version": 1, "...": "..." },
    { "kind": "updateComponentProperties", "version": 1, "...": "..." }
] }
```

**The trap to avoid.** A refusal test that only asserts an error was returned proves nothing about
atomicity — several handlers mutate their candidate before refusing. The refusal rows must compare
serialized bytes before and after.

## Verification

**Commands:**
- `cd folio-go && go test ./...` -- expected: all pass except the one known pre-existing failure
  `TestCorpusMeetsP6ExerciseFloors/P6g` ("floor not met: got 7, need >=20"). Any other red is this
  story's.
- `cd folio-go && go vet ./...` -- expected: clean.
- `cd folio-go && go vet -tags=matrix ./...` -- expected: clean.
- `cd lint && go test ./...` -- expected: all pass.
- `cd folio-designer && npm run typecheck && npm run lint && npm test` -- expected: unchanged from
  baseline (4 pre-existing `only-export-components` lint warnings), proving no designer regression.
- `git diff --name-only` -- expected: no path under `folio-designer/src/`.
