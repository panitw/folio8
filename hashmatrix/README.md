# hashmatrix

This module holds Story 1.2's **retained floating-point contraction probe**
(`hashmatrix/probe/`) — the deliberately-introduced `float64` multiply-add that proves the
cross-target matrix can actually detect FMA contraction, rather than merely never having
seen one — **and nothing else** (D-1.2.3 (amended)).

The **harness driver** that builds, runs and compares across all four targets lives in
`folio-go/matrix_test.go` (`//go:build matrix`), *not here*. An earlier ruling (D-1.2.3)
placed the driver in this module too, for cohesion; that placement was withdrawn once it
was checked against what the driver actually needs: `assertWellFormedPDF`,
`firstDivergence` and `repoRootFromTest`, all unexported helpers in `folio-go`'s own
`package folio8` test scope. A driver living here would either duplicate those — a second
copy of the project's determinism diagnostic, free to drift silently — or force `folio-go`
to export test-only scaffolding as public API. Neither was worth buying cohesion that was
only ever wanted for tidiness, so the driver stays in `folio-go`, and only the probe —
the thing that actually needs to be out of the guards' reach — lives here.

This module is a **separate Go module** — `module github.com/panitw/folio8/hashmatrix` —
with **zero dependencies**, and it has **no dependency on `folio-go`**: no `require`, no
`replace`, no `go.work`. It does not need one. Render capture for the matrix goes through
`folio-go`'s own compiled *test binary*, driven by the `FOLIO8_SUBPROCESS_RENDER` and
`FOLIO8_SUBPROCESS_TOOLCHAIN` seams in `folio-go/render_test.go`, not through an import.

## Why this module exists, and why it is not inside `folio-go`

`folio-go/internal/arch_test.go` (`TestNoFloat64UnderInternal`) fails the build on the mere
*identifier* `float64` anywhere under `folio-go/internal/` — including in a bare
conversion, and regardless of build tags, because it parses every file with `go/parser`
rather than compiling it. Story 1.3's AD-1 import lint lands next with the same reach.
Story 1.2's negative test (AC8–AC10) needs a *retained*, deliberately-introduced float64
multiply-add of exactly the shape those guards exist to forbid — the guards and the
fixture are in direct tension by design.

Putting the probe under `fixtures/` was considered and rejected: that directory has a
defined contractual meaning under AD-21 and D-000.5 (the bytes every future SDK conforms
against, read by relative path at test runtime), and a deliberately-broken floating-point
module inside it would ship to every future vendoring SDK. It would also force Story 1.3's
lint to carve out an exception for `fixtures/`, which is exactly the exemption-list rot
this placement avoids.

`hashmatrix/` is a repo-root module (a D-000.6 spine amendment; see
`ARCHITECTURE-SPINE.md` §Source tree), deliberately outside `folio-go`, so the `float64`
AST guard and Story 1.3's AD-1 lint exclude it **by construction** — both bind
`folio-go/internal/` positively and never mention `hashmatrix/`, so there is nothing to
exempt and nothing to erode.

## `probe/`

`hashmatrix/probe/main.go` computes `pos := x*scale + origin` in `float64` — a multiply-add
of exactly the shape a compiler may fuse into one rounding step (FMA) on architectures that
support it. Two constraints are load-bearing, not stylistic:

- **Operands come from `os.Args` via `strconv.ParseFloat`.** With literal constants the
  compiler folds the whole expression at build time using exact arithmetic and every target
  agrees on the unfused value — the probe would be silently vacuous. Measured: a
  literal-operand variant emits the same 8 bytes on every target.
- **The output is `math.Float64bits(pos)` as 8 raw big-endian bytes**, never a formatted
  decimal. A decimal rendering can round two different `float64` values to the same text,
  masking the very low-bit difference this probe exists to expose.

There is deliberately **no `expected.json` for the probe**. No probe output is recorded in
any fixture; the retained test asserts that the four targets' outputs are **not all equal**,
and specifically that `darwin/arm64` and `linux/arm64` both differ from `js/wasm` — a
relation, not a value, because a probe value is a property of today's toolchain and a
legitimate Go bump could change every one of them without anything being wrong with the
product.

## Building

`go vet ./...` and `gofmt -l .` are clean from this directory. Plain `go build ./...` is
**not** — it exits 1 with `go: build output "probe" already exists and is a directory`,
because this module's only package is `main` at `./probe`, and `go build ./...` with no
`-o` tries to write a binary literally named `probe` into the module root, where the
`probe/` directory already sits. This is a naming collision, not a build defect: build a
specific output path instead, e.g. `go build -o /tmp/probe ./probe` (which is exactly what
`folio-go/matrix_test.go`'s `buildProbeBinary` does).

## Do not

- Do not add a `require` on `folio-go`, a `replace` directive, or a `go.work` file.
- Do not add `expected.json` or any recorded probe hash to this module or to `fixtures/`.
- Do not "fix" a red probe test by hard-coding its operands as literals (this is the
  vacuity guard 9 mistake — see `folio-go/matrix_test.go`'s `TestFMAProbeDiverges`).
