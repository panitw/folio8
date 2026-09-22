# folio8 for .NET

The folio8 engine for .NET. Turn a `.folio` template plus JSON data into PDF
bytes in-process, with the folio8 engine itself — built as a native library —
doing the rendering.

The same template, data, params and font set produce the same PDF bytes here
as under the Go engine, the `folio8` CLI and the designer's preview. That is a
tested property: this package's suite renders corpus fixtures and compares
SHA-256 against the committed expected hashes, on .NET Framework and modern
.NET, in 64-bit and 32-bit processes.

```sh
dotnet add package folio8
```

Nothing else is needed. No Go, no C compiler, no build step, no configuration:
all four native libraries — two Windows, two Linux — and all eleven shipped
font faces are in the package, and the right native is chosen for you at load
time.

## Where templates come from

A `.folio` template is plain text. It diffs, it reviews, and it lives in your
repository next to the code that renders it.

**Design one in the browser at <https://folio8.report>.** The folio8 designer
runs entirely client-side — nothing is uploaded — previews with this same
engine, and saves a `.folio` file you drop into your project. The format
reference and the expression reference are linked from inside the designer, and
their sources are in the repository under
[`docs/`](https://github.com/panitw/folio8/tree/main/docs): start with
[the folio format reference](https://github.com/panitw/folio8/blob/main/docs/folio-format.md).

Because the format is text and documented, a person or an agent can also write
and edit a template by hand, with no designer involved.

## Your first PDF

Put a template at `invoice.folio` and its data at `invoice.json`, then:

```csharp
using System;
using System.IO;

internal static class Program
{
    // ONCE, not per render. Fonts.Shipped() hands every caller its own copy of
    // about 14 MB, so hold the set and pass it to every render.
    private static readonly FontSet Fonts_ = Fonts.Shipped();

    private static void Main()
    {
        Template template = Template.Load("invoice.folio");
        Data data = new Data(File.ReadAllBytes("invoice.json"));

        RenderResult result = Folio8.Render(template, data, null, Fonts_);

        File.WriteAllBytes("invoice.pdf", result.Bytes);
        foreach (Diagnostic d in result.Diagnostics)
        {
            Console.Error.WriteLine(d.Severity + " " + d.Code + ": " + d.Message);
        }
    }
}
```

Plain C# 7.3 with an explicit `Main`, because **.NET Framework 4.6 has no
top-level statements** — the floor this package reaches is the floor its
examples are written to.

The public types sit in the **global namespace**, so there is no `using` to
add for folio8 itself — a call site reads exactly as the contract writes it.

`Render` throws when the engine cannot produce a document; everything it can
render around arrives in `Diagnostics` as a warning instead. This package's
consumer tests make these same calls against a corpus fixture, from every
supported process shape on both target families, and compare the PDF's
SHA-256 with the committed expected hash.

## The API

| Member | What it does |
| --- | --- |
| `Template.Parse(byte[])` | Parses `.folio` bytes into a `Template`. Go's `ParseTemplate`. |
| `Template.Load(string)` | Reads and parses a `.folio` file. Go's `LoadTemplate`. |
| `Folio8.Render(tpl, data, params, fonts)` | Returns a `RenderResult`. Go's `Render`. |
| `Folio8.RenderTo(stream, tpl, data, params, fonts)` | Renders, then writes the PDF to a `Stream`. Go's `RenderTo`. |
| `Folio8.Validate(bytes, data, params, fonts)` | Diagnostics without rendering. Go's `Validate`. |
| `Folio8.ParameterReferences(tpl)` | The param names a template reads. |
| `Folio8.Version` | The folio8 engine version this package's native library was built from. |
| `Fonts.Shipped()` | The shipped font set, as a `FontSet`. |
| `FolioRenderException` | A render the engine aborted with a coded diagnostic. |
| `FolioNativeLoadException` | The native library could not be loaded. See below. |

`parameters` is optional: pass `null` when the template reads none.

Diagnostic codes, severities and message text are the engine's, identical
across this package, the npm package and Go, so a caller can port between
them.

A render runs synchronously inside the native library and blocks the calling
thread while it runs.

## Supported target frameworks

| Target | How it consumes this package |
| --- | --- |
| **.NET Framework 4.6 – 4.8** | `lib/netstandard2.0/Folio8.dll`. The package's MSBuild targets stage both native architectures beside your output; you author nothing. |
| **.NET Core 2.0+ / .NET 5 and newer** | The same `lib/netstandard2.0/Folio8.dll`, with the natives resolved from `runtimes/`. |

One managed assembly serves both. It targets `netstandard2.0` and uses
nothing newer than .NET Framework 4.6 has — no `Span<T>`, no
`System.Text.Json`, no `DllImportResolver` — and it takes **no package
dependencies at all**.

## Supported platforms

| Platform | RID | Native |
| --- | --- | --- |
| **Windows x64** | `win-x64` | `folio8_native.dll` |
| **Windows x86** | `win-x86` | `folio8_native.dll` |
| **Linux x64 (glibc)** | `linux-x64` | `libfolio8_native.so` |
| **Linux ARM64 (glibc)** | `linux-arm64` | `libfolio8_native.so` |

Four natives, one managed assembly, no caller-authored load logic on any of
them.

**The Linux natives need glibc 2.28 or newer** — they are built in a pinned
AlmaLinux 8 image, and that image is what holds the floor there. In practice
that is RHEL/CentOS/Alma/Rocky 8 and newer, Debian 10 and newer, Ubuntu 18.10
and newer, and the glibc-based `mcr.microsoft.com/dotnet` images — the
`-alpine` variants are musl, and are covered by the next paragraph rather than
this one.

**Alpine and other musl hosts are not supported, deliberately.** No
`linux-musl-x64` RID ships and `linux-musl-x64` does not inherit `linux-x64`
assets from the RID graph, so an Alpine consumer resolves no native at all and
meets a clean install-time absence. This is not a gap awaiting effort: Go's
`-buildmode=c-shared` emits initial-exec TLS relocations that musl's loader
refuses under `dlopen`, which is exactly how P/Invoke loads this library, and
building the native *with* musl does not change that. Shipping the RID anyway
would turn that clear absence into a crash on the first render. On musl, use
[folio8 for Node](https://www.npmjs.com/package/folio8), the same engine
compiled to WebAssembly.

**macOS is not shipped.** `osx-x64` and `osx-arm64` are a build leg, a CI leg
and a minimum-version discipline this package has not taken on; the engine
itself builds there, so this is a scope decision rather than a technical one.
On macOS, use [folio8 for Node](https://www.npmjs.com/package/folio8).

**Windows ARM64 is not shipped either.** `win-arm64` does **not** inherit
`win-x64` assets from the RID graph, so an ARM64 Windows host resolves no
native from this package — note that a .NET Framework consumer is unaffected,
since it runs the `win-x64` build under emulation as a 64-bit process.

On an unsupported platform — or where the native library cannot be loaded for
any other reason — the first call throws `FolioNativeLoadException`. There is
no degraded mode: render and validate are the entire library, so there is
nothing to fall back to. Wherever a native is missing,
[folio8 for Node](https://www.npmjs.com/package/folio8) runs the same engine
compiled to WebAssembly, wherever Node runs.

### How the right native is chosen

**On Linux and modern .NET generally, the host does it** — `dotnet` resolves
`runtimes/<rid>/native/` from the RID graph before `DllImport` ever probes,
so nothing in this package chooses and nothing in your project configures it.

On **.NET Framework**, which is Windows-only by definition, it is chosen by
**process bitness, at load time.** A .NET Framework project is
AnyCPU by default, which runs as a 64-bit process on 64-bit Windows and as a
32-bit one under `Prefer32Bit` or on a 32-bit host — so nothing at build time
can know which native you will need. This package reads `IntPtr.Size` before
the first call into the engine, picks `win-x64` or `win-x86`, and loads that
file by full path.

All three process shapes work with no caller-authored load logic, on both
target families:

- AnyCPU on 64-bit Windows → `win-x64`
- AnyCPU with `Prefer32Bit` → `win-x86`
- `PlatformTarget=x86` → `win-x86`

When it cannot load, `FolioNativeLoadException` names the detected process
bitness, the runtime identifier and file name it sought, every path it
probed, and the likely cause — a bitness mismatch, a missing package asset,
or a host that blocks P/Invoke.

## Fonts

Fonts are always an explicit argument. `Fonts.Shipped()` is a convenience, not
a default: it returns a `FontSet` equal to the one the engine's
`fonts.Shipped()` gives a Go caller, keys and bytes alike. Build your own
`FontSet` instead whenever you want a different set — nothing in this library
looks for fonts on the machine it runs on.

The faces are **embedded in `Folio8.dll`**. They are read once for the life of
the process; every call then returns its **own** `FontSet`, so mutating what
you are handed cannot change what the next caller gets.

**Call `Fonts.Shipped()` once and hold the result.** That isolation costs a
copy: each call allocates about 14 MB. Put it in a static field — as the
snippet above does — and pass the same set to every render. A font set is
immutable as far as the engine is concerned, and is safe to share across
threads and across calls.

Eleven faces ship, about 14 MB, and they are the reason the package is large.
`NotoSansSC-Regular.ttf` alone is 10 MB of it.

| Face name | File | Licence |
| --- | --- | --- |
| `Noto Sans` | `NotoSans-Regular.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Bold` | `NotoSans-Bold.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Italic` | `NotoSans-Italic.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Bold Italic` | `NotoSans-BoldItalic.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Thai` | `NotoSansThai-Regular.ttf` | SIL Open Font License 1.1 |
| `Noto Sans Thai Bold` | `NotoSansThai-Bold.ttf` | SIL Open Font License 1.1 |
| `Noto Sans SC` | `NotoSansSC-Regular.ttf` | SIL Open Font License 1.1 |
| `Roboto` | `Roboto-Regular.ttf` | SIL Open Font License 1.1 |
| `Roboto Bold` | `Roboto-Bold.ttf` | SIL Open Font License 1.1 |
| `Roboto Italic` | `Roboto-Italic.ttf` | SIL Open Font License 1.1 |
| `Roboto Bold Italic` | `Roboto-BoldItalic.ttf` | SIL Open Font License 1.1 |

Each face's licence text and upstream notice travel in the package under
`third-party-notices/fonts/`.

## Determinism

No clock, no locale, no network, no ambient environment input, and no file
access beyond `Template.Load`. Pass `documentDate` as an ordinary param if a
document needs one.

## Licence

MIT — see `LICENSE` in this package. The font faces are licensed separately,
under the SIL Open Font License 1.1; their terms ship with them.
