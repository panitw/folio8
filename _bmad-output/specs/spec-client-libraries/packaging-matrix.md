# Packaging and platform matrix

How one Go engine reaches two runtimes, what each package contains, and which
platforms are in scope. Rows marked **OPEN** are unresolved open questions in
SPEC.md, not decisions.

## Delivery

| | folio-js | folio-dotnet |
| --- | --- | --- |
| Engine form | Go → **wasm** | Go → **`c-shared`** native library |
| Binding | Node's WebAssembly + Go's `wasm_exec.js` shim, **`js/wasm` target** — the designer's own | `DllImport` over a C ABI |
| Artifact | one `.wasm`, portable across every Node platform | one `.dll` **per Windows architecture** |
| Fonts | embedded (~14 MB) | embedded (~14 MB) |
| Registry | npm | NuGet |
| Build toolchain on the user's machine | none | none |

folio-js targets **Node only** for now. A browser build is feasible — the
designer already runs this same wasm engine in a browser worker — but is out of
scope by decision, not by limitation.

The asymmetry is deliberate and is the whole reason for the hybrid: **no wasm
runtime targets .NET Framework 4.6.** Wasmtime and Wasmer's .NET bindings
require .NET Core. `DllImport`, by contrast, is present and unchanged from
.NET Framework 1.1 through modern .NET, which makes a C ABI the only mechanism
that spans the stated floor and ceiling from one codebase.

## .NET target frameworks

| Target | Consumes | Status |
| --- | --- | --- |
| .NET Framework 4.6 – 4.8 | `netstandard2.0` assembly | **Required floor.** Windows-only by definition. |
| .NET Core 2.0+ / .NET 5+ | `netstandard2.0` assembly | Required. |
| .NET Standard 2.0 | itself | The single build both columns consume. |

What the 4.6 floor rules out of the binding's own implementation: `Span<T>` and
`Memory<T>`, `System.Text.Json`, nullable reference types, `DllImportResolver`
(so native resolution is by RID-based package layout and probing, not by a
runtime hook), and any `System.Runtime.InteropServices.NativeLibrary` API.

## Native binary platforms — **DECIDED**

folio-dotnet is **Windows-only**, and ships **both** `win-x86` and `win-x64`.

| RID | Ships | Why |
| --- | --- | --- |
| `win-x86` | **yes** | The .NET Framework 4.6 floor, and genuinely 32-bit legacy applications — often the ones most in need of this. |
| `win-x64` | **yes** | Not optional in practice. .NET Framework projects default to **AnyCPU**, which runs as a *64-bit* process on 64-bit Windows; with only an x86 asset present, the normal first experience would be `BadImageFormatException`. |
| `linux-*`, `osx-*`, `win-arm64` | no | Out of scope for this spec. folio-js, being wasm, covers non-Windows hosts. |

Go supports cgo and `-buildmode=c-shared` on both `windows/386` and
`windows/amd64` (verified against this repo's toolchain), so both assets come
from one codebase. The 32-bit build needs a 32-bit mingw-w64 toolchain in CI.

### Resolution and failure

.NET Framework 4.6 has **no `DllImportResolver`**, so there is no runtime hook
to choose an asset. Resolution is therefore by **RID-based package layout plus
probing on process bitness** at load time:

```
runtimes/win-x86/native/folio8.dll
runtimes/win-x64/native/folio8.dll
```

When the load fails anyway — wrong bitness, missing asset, P/Invoke blocked by
a partial-trust or locked-down host — folio-dotnet throws a dedicated folio8
exception (CAP-11) naming the detected process bitness, the RID and filename it
looked for, and the likely cause. There is **no degraded mode**: render and
validate are the entire library, so there is nothing to fall back to.

## Fonts — full set embedded

Both libraries embed the whole `fonts.Shipped()` set — eleven faces, ~14 MB:
Roboto ×4 cuts, Noto Sans ×4, Noto Sans Thai ×2, Noto Sans SC ×1. The face list
is identical to the engine's, so a template that renders under `folio8-go`
renders under either binding with no packaging caveat.

That weight is dominated by one face: **Noto Sans SC is 10 MB of the 14 MB**,
and serves only documents rendering Chinese. Two routes to slimming it stay
open and neither is taken here:

1. **Engine-side retiering** — [SPEC-shipped-font-tiers](../spec-shipped-font-tiers/SPEC.md), deferred. After v1.0.0 this must be additive: a `fonts/cjk` sub-package and a `fonts.ShippedCore()` beside an unchanged `Shipped()`.
2. **Binding-side tiering** — available at any time with **no engine change at all**, since these libraries construct their own `FontSet` and need not call `Shipped()`. Declined for now in favour of matching the engine exactly.

Fetching faces from a third party is excluded on every route: it would introduce
the network dependency the determinism commitments rule out.

## Engine version

Both bindings build against the **`folio8-go/v1.0.0` tag**, never `main`.
Cutting that tag is a prerequisite of this spec, carries its own RELEASING.md
checklist, and irreversibly fixes the public API under D-1.1.c.

The tag is **`v1.0.0`**, not the `v0.1.0` that RELEASING.md and D-1.1.c name —
an owner decision, and a stronger commitment than those documents assume. Under
Go module semantics `v0.x` permits breaking changes freely; `v1.0.0` commits to
semver compatibility and pushes any later breaking change onto a
`github.com/panitw/folio8/folio8-go/v2` import path that every caller must edit.
Both documents need rewording.

`folio8-go v1.0.0` and `folio8-designer 1.0.0` are **independent version lines**
that happen to coincide. Nothing couples them.

## Conformance

CAP-5 requires the golden corpus to run through **each binding on each supported
runtime and platform**, comparing SHA-256 against the committed expected PDF —
the standard [matrix.yml](../../../.github/workflows/matrix.yml) already holds
the Go targets to. The matrix grows multiplicatively with the OPEN rows above,
which is the practical reason to settle them deliberately rather than by
accumulation.

| Binding | Axes |
| --- | --- |
| folio-js | Node LTS versions × host platforms (one portable `.wasm`) |
| folio-dotnet | target frameworks (`net46`, modern) × `win-x86` and `win-x64` × AnyCPU/32-bit/x86 process shapes |
