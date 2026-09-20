---
title: 'Build a FontSet from a directory on disk'
type: 'feature'
created: '2026-09-21'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'de178a6c5e2d5b21b5f33f99c2fe0c681384c4b5'
context:
  - '{project-root}/_bmad-output/specs/spec-font-sources-and-embedding/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A document may now name a face it does not carry (story 1), but the only way to supply
one is to build a `FontSet` in code. An integrator who drops a brand typeface onto a server has no
route from that file to a render.

**Approach:** A host-side helper turns a directory of font files into a `folio8.FontSet`, keyed by
what each binary says its own name is, and the CLI gains a flag for it. The engine is untouched:
it still takes fonts as an explicit value and still never reads the filesystem.

## Decisions

- **D1 — The helper is a new leaf package, not `fonts` and not the root.** Putting it in
  `folio-go/fonts` would force a consumer who wants only a disk loader to pull in ~14.8 MB of
  embedded faces; putting it in `folio8` would put filesystem-font code in the engine's own
  package. `TestOnlyRootAndFontsAreImportableLibraryPackages` allows exactly `{".", "fonts"}` and
  says in its own failure text to *"add it to the census deliberately"* — this is that deliberate
  addition, allowlist and `publicSurfacePins` extended in the same commit.
- **D2 — A face is keyed by its own name table, never by its filename.** `fonts.go` is explicit
  that nothing may derive a family from a key and that *"the machine-readable family is the face's
  own sfnt name ID 1"*. Deriving identity from a filename is that same anti-pattern one layer out.
  The key is name ID 1, plus name ID 2 when the subfamily is not `Regular`, joined by a space —
  producing `Sarabun` and `Sarabun Bold`, the shape `fonts.Shipped()`'s own keys already have and
  the shape the designer will write in story 3.
- **D3 — Merging is the caller's business.** The helper returns the directory's set and nothing
  else. Callers combine sets with `maps.Copy`, whose "second wins" is the repo's existing
  precedent in `fonts.go:200` and `internal/wasm/engine.go`. No merge helper, no precedence rule
  invented here.

## Boundaries & Constraints

**Always:**
- The engine is untouched. No file under `folio-go/*.go`, `internal/` or the bindings changes
  behaviour; `folio8.FontSet` stays `map[string][]byte` and the sole font input.
- A file that will not parse as a font this version can read is **skipped, not fatal** — a
  directory with one corrupt `.ttf` is the normal case on a real server. Skips are **reported to
  the caller**, never silent.
- Reuse the existing validation rather than restating it: `internal/template.DecodeFontForRender`
  and `internal/fontset.RefuseVariableFace`, in that order, as `component_commands.go:4576-4598`
  already does for an embedded face.
- The CLI flag lands on **both** `validate` and `render`. `TestSubcommandNamesMatchRunSwitch` and
  the parity table exist because these two must not drift.

**Never:**
- No recursion into subdirectories, no following symlinks out of the directory, no reading the
  operating system's font book, no network.
- No new module dependency — `wantModuleGraph` is exactly two entries.
- No `float64`/`float32` anywhere, including tests (`TestNoFloat64UnderModule` scans the whole
  module).
- Do not expose `FaceFallback` on the CLI in this story, and do not change `fonts.Shipped()`.
- Do not make package `folio8` import the new package, or `fonts` import it.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Ordinary directory | Three readable faces | A `FontSet` of three, keyed from their name tables | N/A |
| Bold cut | `Sarabun-Bold.ttf` whose name table says family `Sarabun`, subfamily `Bold` | Key is `Sarabun Bold`, not the filename stem | N/A |
| Regular cut | Subfamily is `Regular` | Key is the bare family, `Sarabun` | N/A |
| Corrupt file | One unreadable file beside two good ones | The two good faces load; the bad one is reported as skipped | Skip, reported |
| Variable font | An `fvar`-carrying face | Skipped and reported, like any other refusal | Skip, reported |
| Non-font files | `.txt`, `LICENSE`, a subdirectory | Ignored without being reported as skips | N/A |
| Unreadable name table | A parseable face whose name ID 1 is absent | Skipped and reported — it cannot be keyed | Skip, reported |
| Two files, one key | Two faces whose name tables produce the same key | Deterministic winner, and the loser reported | Reported |
| Missing directory | Path does not exist | An error naming the path | Error |
| Empty directory | Exists, no font files | An empty `FontSet` and no error — an empty set is legal since story 1 | N/A |

</frozen-after-approval>

## Code Map

- **New package**, name it for what it holds (e.g. `folio-go/fontdir`). It may import `internal/…`
  because it lives inside `folio-go/`.
- `folio-go/public_surface_census_test.go:218` `TestOnlyRootAndFontsAreImportableLibraryPackages`
  -- `allowed := {".": true, "fonts": true}` at `:220`; the failure text at `:253` is the
  instruction. `:265` closes it downward, so the entry must be added, not worked around.
- `folio-go/public_surface_census_test.go:271` `TestPublicSurfaceMatchesTheFrozenV1Census` --
  scans the pair list at `:275`; add the new package there and add every exported identifier to
  `publicSurfacePins` (`:26`) in the same commit (`:290`).
- `folio-go/fonts/fonts.go:180-201` `Shipped()` -- the key shape to match (`Noto Sans`,
  `Noto Sans Bold`, …) and, at `:170-175`, the rule D2 follows: never parse a key; name ID 1 is
  the machine-readable family.
- `folio-go/internal/fontset/fontset.go:85` `New(name, data)` -- parses and validates
  unitsPerEm and readable tables. `internal/fontset/variableface.go:93` `RefuseVariableFace`.
  `internal/template/fontasset.go:197` `DecodeFontForRender(mediaType, data, site)` -- needs a
  media type (`font/ttf`, `font/otf`); `checkSfnt` at `:249` is unexported and reached through it.
- **Name ID 1 is not reachable today.** `fontset.Font` exposes `PostScriptName()` (`:465`, name ID
  6 via `:502`) but no family. `internal/fontset/licencesignature.go:240` `ReadLicenceStatement`
  shows the reading pattern — `ot.ParseFont` → `HasTable(ot.TagName)` → `TableData` →
  `ot.ParseName` → `names.Get(n)`. Add family/subfamily accessors to `fontset.Font` beside
  `PostScriptName`, keeping `*ot.Font` behind the fontset seam (D-1.5.10/AC17a forbids it
  crossing).
- `folio-go/cmd/folio8/main.go` -- `subcommandNames` literal at `:74`; `run`'s switch at `:95-101`;
  usage at `:106`; per-subcommand flag sets at `:203-207` (validate) and `:248-253` (render);
  shared `resolveInputs` at `:137`. `fonts.Shipped()` is passed at exactly `:235` and `:286`.
- `folio-go/cmd/folio8/subcommand_parity_test.go:33,:124,:244` -- parity is AST-checked and
  behaviour-checked across both subcommands. `main_test.go:62` pins the usage text;
  `:103` pins stream discipline (stdout carries only PDF bytes).

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/internal/fontset` -- add family and subfamily accessors reading name ID 1 and 2,
      beside `PostScriptName`, keeping `*ot.Font` inside the seam.
- [x] new leaf package -- the loader: read the directory non-recursively, validate each candidate
      through `DecodeFontForRender` then `RefuseVariableFace`, key it per D2, and return the set
      plus the skips.
- [x] `folio-go/public_surface_census_test.go` -- add the package to the allowlist and the scan
      pair list, and every exported identifier to `publicSurfacePins`.
- [x] `folio-go/cmd/folio8/main.go` -- add the directory flag to **both** subcommands and merge
      its set over `fonts.Shipped()` at both `:235` and `:286`; extend the usage text.
- [x] new package tests -- one per I/O Matrix row, with fixtures under `testdata/`.
- [x] `folio-go/cmd/folio8` tests -- the flag works on both subcommands and a document naming a
      face only the directory supplies renders.
- [x] `docs/rendering-library.{md,html}` -- document the helper and the CLI flag, twins in
      lockstep; `docs_examples_test.go` requires every exported identifier to appear verbatim.

**Acceptance Criteria:**
- Given a directory holding a face, when a document naming that face by its name-table key is
  rendered with the directory's set merged over the shipped set, then it renders correctly and
  emits no diagnostic — identical to the same document with that face embedded.
- Given the same directory with one corrupt file added, when the set is built, then every good
  face still loads and the corrupt one is reported as skipped.
- Given a face file renamed on disk, when the set is built, then its key is unchanged — the key
  comes from the binary, not the filename.
- Given `go test -count=1 ./...` in `folio-go`, then it passes, including the public-surface
  census, the module graph, the no-float64 scan and the CLI parity tests.

## Implementation Notes

- **The new package is `folio-go/fontdir`**, one file, one exported function and one exported type:
  `Set(dir) (folio8.FontSet, []Skipped, error)` and `Skipped{File, Reason}` with a `String()` log
  line. Candidates are `.ttf`/`.otf` regular files directly in the directory; a subdirectory or a
  symlink fails `entry.Type().IsRegular()` and is ignored by category rather than by name.
- **Name ID 1 reached the outside through `fontset.Font`**, as the Code Map directed:
  `readFamilyNames` (records 1 and 2, via `ot.ParseName`, beside `readPostScriptName`) feeds two
  new accessors, `Family()` and `Subfamily()`. Both return plain strings; no `*ot.Font` crosses the
  seam. An absent name table stays a non-error there, as it already was for record 6 — `fontdir`
  turns the empty family into a reported skip of its own.
- **Validation order is `DecodeFontForRender` → `RefuseVariableFace` → `fontset.New`.** The third
  is the renderer's own ingestion, run at load so that a face which would fail at render fails
  where there is a caller holding a report — and it is also what reads the name table the key comes
  from.
- **The CLI flag is `-fonts <dir>`, assembled inside `resolveInputs`**, not beside it: both
  subcommands get it from the one input-assembly step, which is the same structural reason
  SOURCE_DATE_EPOCH lives there. Skips print to stderr as `SKIPPED FONT <file>: <reason>`; a
  missing directory is an ordinary input failure (exit 1), identical on both subcommands, like a
  missing `-data` file. Precedence is `maps.Copy(fonts.Shipped(), supplied)` — second wins.
- **No font binary was committed.** Every fixture is written into `t.TempDir()` from bytes already
  in the repo (the eleven `fonts.Shipped()` faces and the committed variable build), under
  filenames the tests choose — which is also what makes the "the key comes from the binary, not the
  filename" assertions mean anything. The nameless-face row is built by dropping the `name` table's
  directory record; the CLI's unshipped brand face is built by an equal-length rename of
  "Noto Sans" to "Foto Sans" inside the binary, so no table offset moves.
- **Symlinks: the STRICT rule was taken.** No symlink is followed, including one resolving inside
  the directory. Distinguishing "inside" from "outside" means resolving a target and ruling on it,
  which has no cheap correct answer once the directory is itself a link or is relocated between
  deploys. The compensating property is that a link whose name looks like a face is REPORTED as a
  skip, so an operator who staged fonts by linking sees why they are missing. Stated in the package
  doc, the guide twins and the README.
- **`-strict` counts a skipped font and an empty font directory.** `-strict` already means "a
  Warning fails the build", and a brand face that did not load is exactly the deployment gap a
  strict build exists to catch. Without `-strict` both stay notices on stderr and the run succeeds.
  A directory that yields no faces at all prints `NO FONTS FOUND in <dir>`, so a typo'd path is not
  indistinguishable from a correct one.
- **Near misses are reported, unrelated files are not.** A `.woff`, `.woff2`, `.ttc` or `.otc` is
  somebody's brand face that will simply not appear, so it comes back as a skip; a `LICENSE`, a
  `.txt` and a subdirectory stay silently ignored, as the frozen I/O matrix requires.
- **Name records are read ONCE.** `readNames` returns records 1, 2 and 6 from a single
  `ot.ParseName`, so the engine does not pay for a value only `fontdir` consumes. Records 1 and 2
  prefer the Windows/Unicode English record (platform 3, encoding 1, language `0x0409`), falling
  back to the vendor's answer — `ot.Name` keys by nameID alone and keeps the LAST record, so a face
  with localized names would otherwise be keyed in whichever script came last in the table.
- **Strongest single assertion:** `TestKeysComeFromTheBinaryAndNotTheFilename` writes all eleven
  shipped faces to disk as `fa.ttf` … `fk.ttf` and asserts the keys `Set` derives are byte-equal to
  `fonts.Shipped()`'s own key set.

## Spec Change Log

## Review Triage Log

Pass 1 — three layers. One row per finding.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | `-fonts` precedence (a disk face replacing a same-named shipped face) is asserted nowhere; every fixture uses `Foto Sans`, a key the shipped set does not carry, so inverting `maps.Copy` leaves the whole suite green. | **medium** | Pre-verified by the verification-gap layer, which demonstrated the inversion. Usage text, README and both doc twins all promise the behaviour. |
| 2 | `.otf` is an advertised candidate extension that no test ever exercises; the module has no `.otf` fixture at all. | **medium** | Pre-verified: deleting the `.otf` entry, or mistyping its media type, leaves every test green. OTF is the common delivery format for retail CFF faces. |
| 3 | The `name` table is now parsed twice for every face the ENGINE loads — `readPostScriptName` and the new `readFamilyNames` each call `ot.ParseName` — for a value only `fontdir` consumes. | **medium** | Confirmed in the diff. A render-path regression paid by every consumer to serve one package. |
| 4 | `ot.Name.Get` keeps the LAST matching record, so a face carrying localized name records can be keyed in a non-English script and never match its chain entry. | **medium** | Confirmed against the accessor. Real for any face shipping a localized family name. |
| 5 | Font-shaped entries that are not candidates — `.woff`, `.woff2`, `.ttc`, a symlinked `.ttf`, a subdirectory — are dropped before the candidate loop and reported as nothing at all. | **medium** | Confirmed at the extension filter. An operator whose brand face is a `.woff` sees an empty set and zero skips. |
| 6 | `-fonts` pointed at an existing but wrong or empty directory renders with fallback faces and says nothing. | **medium** | Confirmed: a typo'd path is indistinguishable from a correct one at the CLI. |
| 7 | All symlinks are dropped, where the frozen boundary forbids only following them OUT of the directory — and staging fonts by symlink is ordinary deploy practice. | **low** | Confirmed. Stricter than the intent rather than in breach of it; the gap is that nothing says so. |
| 8 | A face passing the load checks but lacking `cmap` or outlines is keyed clean and fails later at render. | **low** | `requiredTables` covers `head`/`maxp`/`hhea`/`hmtx`/`OS/2` only. Real but narrow. |
| 9 | Family/subfamily records containing control characters become `FontSet` keys; only leading/trailing space is trimmed. | **low** | Confirmed. Corrupts skip reports and diagnostics rather than breaking a render. |
| 10 | The duplicate-key skip reason names a bare filename while `Skipped.File` carries the full path — one log line, two spellings of the same directory. | **low** | Confirmed: `keyedFrom` stores `entry.Name()`, `File` stores `filepath.Join(dir, name)`. |
| 11 | Four tests do not bite: `TestTwoFilesOneKeyResolveDeterministically` uses byte-identical fixtures so the winner is unobservable; `TestSetIsMergedByTheCaller`'s comment describes a fixture it does not build and hand-rolls the merge instead of using `maps.Copy`; the symlink `t.Skipf` aborts the whole non-font-files test; a comment says `f0.ttf … f10.ttf` where the code writes `fa.ttf … fk.ttf`. | **low** | All four confirmed by reading the test file. |
| 12 | `TestOnlyRootAndFontsAreImportableLibraryPackages` now allows three packages but still says "RootAndFonts" in its name. | **low** | Confirmed. A failure headed by that name sends the next reader after a rule that no longer exists. |
| 13 | The interaction between a skipped font and `-strict` is unspecified and untested: a missing brand face prints to stderr and still exits 0 under `-strict`. | **low** | Confirmed. Whichever way it is decided, nothing states it. |
| 14 | The two new error wrappings from `fontset.New` are prefix-identical, so a reader cannot tell which name record failed. | **low** | Confirmed. |
| 15 | README omits that a directory face replaces a shipped one, and that a missing directory fails the run. | **low** | Confirmed. |
| 16 | `Skipped.Reason` is prose only, so a caller who wants to fail their build on "a real font was rejected" but tolerate "a variable build" must string-match English owned by the validators. | **low — rejected** | Real, and cheaper now than after the v1 surface freezes. Rejected under the workflow's own rule: the fix adds public surface the spec does not settle, which bars it from `patch`, and routing it to `intent_gap` would stop an explicitly autonomous run. Recorded in `DECISIONS.md` A-12 so the cost is visible rather than lost. |
| 17 | AC4 says `go test ./...` passes, but `internal/text`'s `TestCorpusMeetsP6ExerciseFloors/P6g` is red. | **rejected** | True but rejected on the workflow's "reject any finding whose fix is to edit this build's spec". The failure is pre-existing, unrelated to this diff, confirmed red at baseline, and already disclosed in this spec's own Verification section. |
| 18 | The Tasks section says fixtures live under `testdata/`; they are built into `t.TempDir()` instead. | **false** | Not a defect: building fixtures from bytes already in the repo is what makes the filename-independence assertions meaningful, and the test file says so deliberately. The task's wording was a planning guess, not a contract. |

**Routing.** No `intent_gap` and no `bad_spec`. #1–#15 route to **patch**; #16 and #17 are rejected
with their reasons recorded; #18 is rejected on its refutation.

## Design Notes

D2 is the decision most likely to be second-guessed, so the reasoning is worth keeping: keying by
filename would work and is simpler, and it is exactly what `fonts/fonts.go:170-175` forbids one
layer up — *"a `strings.TrimSuffix(key, " Bold")` anywhere would reinstate the naming-convention
weight carrier D-B explicitly foreclosed"*. A directory loader that trusted filenames would make
the same mistake with the same consequence: two files a human renamed become two different faces,
or one face silently shadows another.

## Verification

**Commands:**
- `cd folio-go && go vet ./...` -- expected: clean.
- `cd folio-go && go test -count=1 ./...` -- expected: all pass. `public_surface_census_test.go`,
  `gomod_test.go`, `internal/arch_test.go` and `cmd/folio8/subcommand_parity_test.go` are the ones
  a new package and a new flag break first. One failure is **pre-existing and not this story's**:
  `internal/text`'s `TestCorpusMeetsP6ExerciseFloors/P6g` (`got 7, need >=20`), confirmed red at
  baseline.
- `cd folio-go && go test -count=1 -run 'TestTargetRenderHash' -tags=matrix .` -- expected: pass;
  this story must not move any rendered byte.
