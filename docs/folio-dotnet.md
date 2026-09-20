<!-- twin:begin -->
# folio-dotnet

folio-dotnet is the folio8 engine for .NET, published to NuGet as `folio8`. It turns a `.folio` template, JSON data and runtime parameters into PDF bytes in process, with the engine itself — built as a native library and called over a C ABI — doing the rendering. There is no service to run and no rendering logic written in C#.

The same template, data, params and font set produce the same bytes here as under the Go library, the `folio8` command-line tool and the designer's preview. That is a tested property rather than a claim: the package renders the fixture corpus and compares SHA-256 against the committed expected hashes on .NET Framework and on modern .NET, in 64-bit and in 32-bit processes.

**The API is synchronous, matching Go.** A render blocks the calling thread for its whole duration. folio-js, the same engine for Node, is promise-based instead. The two libraries cover the same surface in the same shape, each spelled the way its language spells things — `Folio8.Render` here is `render` there, `Template.Parse` is `parseTemplate`, `FolioRenderException` is `FolioRenderError` — with identical behaviour and identical diagnostics, so a caller can port between them and between both and Go.

Three companion references hold the rules this guide does not repeat: [the `.folio` format](folio-format.md) is every field of a template, [expressions](expression-reference.md) is the syntax inside the double braces, and [the rendering library guide](rendering-library.md) covers the same engine from Go, including the shared [diagnostic code registry](rendering-library.md#diagnostic-codes). [folio-js](folio-js.md) is this same library for Node.

## Install

```sh
dotnet add package folio8
```

Nothing else is needed. No Go, no C compiler, no build step and no configuration: both Windows native libraries and all eleven shipped font faces travel inside the package, the right native library is chosen for you at load time, and the managed assembly takes no package dependencies at all.

The public types sit in the **global namespace**, so there is no `using` directive to add for folio8 itself and a call site reads exactly as this reference writes it.

### Supported target frameworks

| Target | How it consumes the package |
| --- | --- |
| .NET Framework 4.6 through 4.8 | The `netstandard2.0` managed assembly. The package's own MSBuild targets stage both native architectures beside your output; you author nothing. |
| .NET Core 2.0 and newer, .NET 5 and newer | The same `netstandard2.0` managed assembly, with the natives resolved from the package's runtimes folder. |

**.NET Framework 4.6 is the floor, and it is reached deliberately rather than by implication.** Reporting workloads of the kind folio8 serves — statements, invoices, regulatory documents — run disproportionately on long-lived .NET Framework applications that cannot be moved to modern .NET, so the binding is built to meet them where they are.

That floor decides the implementation. It predates `Span`, `System.Text.Json` and `DllImportResolver`, so the binding is plain marshalling over a C ABI and the managed assembly targets `netstandard2.0` — the widest target framework moniker that .NET Framework 4.6 and modern .NET both consume from a single build. One managed assembly serves both families, and a compile-only project builds the shipped sources against 4.6 on every change so the floor cannot rot.

### Supported platforms

**Windows only, on x86 and on x64.** Both architectures ship in the package. There are no Linux, no macOS and no ARM64 native binaries here; for a non-Windows host use [folio-js](folio-js.md), which is the same engine compiled to WebAssembly and runs wherever Node does.

**On an unsupported platform the first call into the engine throws `FolioNativeLoadException`.** The same exception is thrown whenever the native library cannot be loaded for any other reason. There is no degraded mode and nothing to fall back to: rendering and validation are the entire library. The exception names the detected process bitness, the runtime identifier and file name it sought, every path it probed, and the likely cause — a bitness mismatch, a missing package asset, or a host that blocks calls into unmanaged code.

### Choosing the native library

**By process bitness, at load time.** A .NET Framework project is AnyCPU by default, which runs as a 64-bit process on 64-bit Windows and as a 32-bit process under Prefer32Bit or on a 32-bit host, so nothing at build time can know which architecture you will need. This library reads the size of a pointer before the first call into the engine, picks the 64-bit or the 32-bit runtime identifier, and loads that file by full path.

All three process shapes work on both target framework families with no load logic written by the caller:

| Project shape | Native library loaded |
| --- | --- |
| AnyCPU on 64-bit Windows | the x64 native library |
| AnyCPU with Prefer32Bit | the x86 native library |
| An explicit x86 platform target | the x86 native library |

Shipping only one architecture would have made a bad image format exception the normal first experience for an AnyCPU project, which is why both travel in the package.

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

**Plain C# 7.3 with an explicit `Main`, because .NET Framework 4.6 has no top-level statements.** The floor the package reaches is the floor its examples are written to, and this program compiles unchanged on every supported target.

`Template.Load` reads and parses the file; `Folio8.Render` returns the PDF bytes and every warning the engine raised on the way. The third argument is the params object, and `null` says the template reads none.

**The same calls are executed, not illustrated.** The package's consumer programs make exactly these calls — `Template` parse, `Fonts.Shipped`, `Folio8.Render` — against a corpus fixture from every supported process shape on both target framework families, and compare the PDF's SHA-256 against the committed expected hash. They differ from the program above only in taking the fixture from the command line and in holding the font set inside the `try` rather than in a static field, so that a load failure is reported rather than buried in a type initializer.

**Fonts are an explicit argument, always.** There is no default font set and no lookup on the machine the render runs on. `Fonts.Shipped` is a convenience that returns the eleven faces embedded in the managed assembly — the same faces, byte for byte, that Go's `fonts.Shipped` returns. Build your own `FontSet` instead whenever you want a different set.

**The set may be empty.** A template that carries every face it names — one saved with embedding on — has nothing for a font set to contribute, so `new FontSet()` is a legitimate call and not a caller error. Nothing is refused before the engine has tried to resolve the chains; an entry that genuinely cannot be resolved still fails with `TEXT_FACE_ABSENT`, located at the element. Passing `null` is still an `ArgumentNullException`.

**When a face is missing, choose what happens.** `Folio8.Render`, `Folio8.RenderTo` and `Folio8.Validate` take an optional trailing `FaceFallback`:

| Value | Behaviour |
|---|---|
| `FaceFallback.Strict` | The default, and what every call made before this argument existed does: a character covered by no present face of its chain, where some face that chain declares was never supplied, throws `FolioRenderException` with the code `TEXT_FACE_ABSENT`. |
| `FaceFallback.Substitute` | The character is painted in a face this renderer actually holds — the document's own embedded assets first, then your `FontSet`, each searched in face-name order, the first face that covers the character winning — and the warning `TEXT_FACE_SUBSTITUTED` names the element, the character, the face requested and the face painted. A renderer holding nothing that covers the character still throws `TEXT_FACE_ABSENT`. |

```csharp
RenderResult result = Folio8.Render(template, data, null, fonts, FaceFallback.Substitute);
foreach (Diagnostic d in result.Diagnostics)
{
    if (d.Code == "TEXT_FACE_SUBSTITUTED") Console.Error.WriteLine(d.Message);
}
```

The choice is deterministic: the same inputs produce the same bytes on any machine. A value outside the enum is an `ArgumentException` from the binding, never a silent default.

**A lenient render may space its lines differently.** Under substitution any face the renderer holds may end up painted into an element whose chain has an unsupplied member, so such an element is given a line box tall enough for the tallest of them — the alternative is a substituted character overflowing its box with nothing reported. The leading of those elements therefore differs from the same document rendered strictly. An element whose chain is fully supplied is unaffected, and a strict render is unchanged in every case.

**Call `Fonts.Shipped` once and hold what it gives you**, as the program above does with a static field. The faces are read once for the life of the process, but every call allocates its own `FontSet` of about 14 MB so that mutating what you are handed cannot change what the next caller gets. A font set is immutable as far as the engine is concerned and is safe to share across calls and across threads.

### Passing params

Params are the runtime values that are not report data — a run date, a branch code, a report title. They live in their own namespace, which a template reads as `params.branchCode`, and they can never be shadowed by report data. `documentDate` is the one reserved name: a template that prints a document date reads it from params, because nothing in the library reads the clock.

.NET Framework 4.6 has no `System.Text.Json`, so this library never serialises for you. Prepare the JSON however your application already does, wrap it, and ask the template which names it wants:

```csharp
Console.WriteLine(string.Join(", ", template.ParameterReferences()));

Params parameters = new Params(
    "{\"documentDate\":\"2026-08-15T00:00:00Z\",\"branchCode\":\"004\"}");

RenderResult result = Folio8.Render(template, data, parameters, Fonts_);
```

`documentDate` must be an RFC 3339 timestamp; anything else is a `DOCUMENT_DATE_INVALID` error.

### Calling from several threads

**`Folio8.Render` and `Folio8.Validate` may be called concurrently from as many threads as you like, against the one loaded native library.** Each call is self-contained: the engine carries no state from one call to the next, and the one structure that does span a call — the native allocation table that hands each result back across the boundary — is mutex-guarded. Nothing else crosses a call.

An ASP.NET application can therefore serve concurrent requests without a lock of its own. A `Template` is opaque and immutable, and a `FontSet` is never modified by the engine, so both are safe to share across threads; the only caution is the ordinary one, that you not mutate a `FontSet` while another thread is rendering with it. Each call still blocks its own thread for the duration of the render.

## Warnings and errors

folio8 reports every problem as a `Diagnostic`, field for field the object the Go library and folio-js produce for the same condition:

| Member | Meaning |
| --- | --- |
| `Severity` | `Severity.Warning` or `Severity.Error`. The enumeration is a closed set of two dispositions, and Go's unset zero value is not representable here. |
| `Code` | A stable string from a closed registry, such as TEXT_CLIPPED_WIDTH. This is the member to dispatch on. |
| `ElementId` | The template element the problem concerns, when there is one. |
| `DataPath` | The data path, or for a load failure the template field path, when there is one. |
| `Message` | A human-readable sentence. Print it; never parse it. |

Two diagnostics carrying the same five values are equal: `Diagnostic` implements value equality, so `Equals` compares the fields rather than the references.

**A warning reaches `Diagnostics`; an error is thrown.** A successful render never carries a diagnostic of severity `Severity.Error`, and a thrown error never appears in `Diagnostics` — the same split Go makes between `Result.Diagnostics` and a `RenderError`. The list is always present, and empty rather than null when nothing went wrong.

**`Code` is the contract and `Message` is not.** Codes are additive: once shipped, a code's string and its meaning never change. Message text is prose for a person to read, matches the Go library's word for word for the same condition, and carries no promise beyond that.

A failure the engine raised as a known document condition is a `FolioRenderException` carrying the `Diagnostic` that caused it. Everything else — a null argument, data that is not valid JSON, a stream that failed — throws an ordinary framework exception, and a native library that cannot be loaded throws `FolioNativeLoadException`:

```csharp
try
{
    RenderResult result = Folio8.Render(template, data, parameters, fonts);
    foreach (Diagnostic d in result.Diagnostics)
    {
        if (d.Code == "TEXT_MISSING_GLYPH")
        {
            Console.Error.WriteLine("element " + d.ElementId + " lost a character: " + d.Message);
        }
    }
    return result.Bytes;
}
catch (FolioRenderException error)
{
    if (error.Diagnostic.Code == "BINDING_PATH_ABSENT")
    {
        throw new InvalidOperationException("data is missing " + error.Diagnostic.DataPath);
    }
    throw new InvalidOperationException(error.Diagnostic.Code + ": " + error.Message);
}
catch (FolioNativeLoadException error)
{
    Console.Error.WriteLine(error.FileName + " for " + error.RuntimeIdentifier
        + " could not be loaded in a " + error.PointerSize + " byte pointer process");
    foreach (string probed in error.ProbedPaths)
    {
        Console.Error.WriteLine("  probed " + probed);
    }
    throw;
}
```

**A partial font set now refuses.** If you pass a `fonts` dictionary that is short of a face a document's chain declares, a character no supplied face covers throws a `FolioRenderException` carrying `TEXT_FACE_ABSENT` and produces no PDF, where earlier releases returned a `TEXT_MISSING_GLYPH` warning and a PDF with those characters silently missing. The message names every absent face. `TEXT_MISSING_GLYPH` still means what it always did: every declared face was supplied and none of them covers the character.

Every code, with its string value and its meaning, is listed under [diagnostic codes](rendering-library.md#diagnostic-codes) in the rendering library guide. The registry belongs to the engine, so it reads the same from all three languages and a caller can port between them.

`Folio8.Validate` answers the same question without producing a document. It takes raw template bytes, checks them against data, params and fonts, and returns the diagnostic list the engine would have raised — throwing on an error exactly as `Folio8.Render` does.

## API reference

Everything below is public, and everything public is below.

### Rendering and validation

#### Folio8.Render

```csharp
RenderResult Folio8.Render(Template template, Data data, Params parameters, FontSet fonts)
```

Renders the document and returns its bytes plus its warnings, which is Go's `Render`.

#### Folio8.RenderTo

```csharp
IList<Diagnostic> Folio8.RenderTo(Stream destination, Template template, Data data, Params parameters, FontSet fonts)
```

Renders, then writes the PDF to a `Stream` and returns the warnings — Go's `RenderTo` with a stream in place of an `io.Writer`, for HTTP responses and large documents. The stream is never closed, and nothing is written when the render fails.

#### Folio8.Validate

```csharp
IList<Diagnostic> Folio8.Validate(byte[] template, Data data, Params parameters, FontSet fonts)
```

Checks a template against data, params and fonts without producing a document, which is Go's `Validate`. It takes raw template bytes rather than a parsed `Template`, matching Go.

#### Folio8.Version

```csharp
string Folio8.Version
```

The folio8 engine version the packaged native library was built from, read from the engine itself rather than restated in managed code.

#### Template.Parse

```csharp
Template Template.Parse(byte[] bytes)
```

Parses `.folio` bytes into a `Template`, which is Go's `ParseTemplate`. This is the primary constructor: bytes in, template out. A malformed document throws `FolioRenderException`.

#### Template.Load

```csharp
Template Template.Load(string path)
```

Reads the file at the path and parses it, which is Go's `LoadTemplate`. It is the only call in the library that touches the filesystem on your behalf.

#### Template.ParameterReferences

```csharp
IList<string> Template.ParameterReferences()
```

The sorted, de-duplicated top-level parameter names the template reads, which is Go's `ParameterReferences`. Use it to build a params object without reading the template by hand.

### Fonts

#### Fonts.Shipped

```csharp
FontSet Fonts.Shipped()
```

The eleven faces Go's `fonts.Shipped` returns, read from resources embedded in the managed assembly: Roboto and Noto Sans in regular, bold, italic and bold italic, Noto Sans Thai in regular and bold, and Noto Sans SC. Keys and bytes are the engine's own, so a document rendered from this set here and from `fonts.Shipped` in Go produces identical PDF bytes.

These are the keys, and a template's font chain must name one of them verbatim — the engine never derives `Roboto Bold` from `Roboto`:

| Family | Face names (FontSet keys) |
| --- | --- |
| Noto Sans | `Noto Sans`, `Noto Sans Bold`, `Noto Sans Italic`, `Noto Sans Bold Italic` |
| Noto Sans Thai | `Noto Sans Thai`, `Noto Sans Thai Bold` |
| Noto Sans SC | `Noto Sans SC` |
| Roboto | `Roboto`, `Roboto Bold`, `Roboto Italic`, `Roboto Bold Italic` |

To use your own faces, build the set yourself, optionally starting from the shipped one:

```csharp
FontSet fonts = Fonts.Shipped();
fonts["Acme Display"] = File.ReadAllBytes("AcmeDisplay.ttf");
```

**The shipped faces are licensed under the SIL Open Font License 1.1**, and each face's licence text and upstream notice travel in the package under `third-party-notices/fonts/`. Faces you supply yourself, or embed in a template, are under their own licences: a template that embeds a font redistributes that font with the file. The Go guide's [fonts](rendering-library.md#fonts-1) section carries the same rules for the same face set.

### Types

#### Template

An opaque parsed template. It holds the engine's canonical bytes and offers no way to read or change the document — editing is the designer's surface and is deliberately not in this library. Obtain one from `Template.Parse` or `Template.Load`.

#### Data and Params

```csharp
Data(byte[] json)
Data(string json)
Params(byte[] json)
Params(string json)
```

Both wrap JSON, as bytes or as a string, and both convert implicitly from either so a call site can pass one without naming the type. .NET Framework 4.6 has no `System.Text.Json`, so the choice of serialiser is left to you: prepare the JSON and hand it over. `Data` is the report data; `Params` carries the runtime values that are not report data, such as `documentDate`, and is optional — pass `null` when the template reads none.

#### FontSet

```csharp
FontSet : IDictionary<string, byte[]>

FontSet()
FontSet(IEnumerable<KeyValuePair<string, byte[]>> faces)

byte[] this[string name]
ICollection<string> Keys
ICollection<byte[]> Values
int Count
bool IsReadOnly

void Add(string name, byte[] face)
void Add(KeyValuePair<string, byte[]> face)
void Clear()
bool Contains(KeyValuePair<string, byte[]> face)
bool ContainsKey(string name)
void CopyTo(KeyValuePair<string, byte[]>[] array, int index)
IEnumerator<KeyValuePair<string, byte[]>> GetEnumerator()
bool Remove(string name)
bool Remove(KeyValuePair<string, byte[]> face)
bool TryGetValue(string name, out byte[] face)
```

Face name to font file bytes, which is Go's `FontSet`. It is always an argument and never ambient. Construct an empty set, or one from a sequence of face name and bytes pairs; the rest is the dictionary surface a caller already knows, over an insertion-ordered store.

#### RenderResult

```csharp
byte[] RenderResult.Bytes
IList<Diagnostic> RenderResult.Diagnostics
```

What `Folio8.Render` returns: the PDF, and every warning in the engine's order.

#### Diagnostic and Severity

```csharp
Diagnostic(Severity severity, string code, string elementId, string dataPath, string message)

enum Severity { Warning, Error }
```

One engine diagnostic, field for field Go's `Diagnostic`, with the names in the casing .NET expects: `Severity`, `Code`, `ElementId`, `DataPath` and `Message`. `Severity` is a closed set of two dispositions, `Warning` and `Error`.

### Errors

#### FolioRenderException

```csharp
Diagnostic FolioRenderException.Diagnostic
```

Thrown when the engine aborts on a known document condition, which is Go's `RenderError`. Its `Message` is the engine's error text and its `Diagnostic` carries the code, the element and the data path. An error arrives as a throw and never as an entry in `Diagnostics`.

#### FolioNativeLoadException

```csharp
string FolioNativeLoadException.RuntimeIdentifier
string FolioNativeLoadException.FileName
int FolioNativeLoadException.PointerSize
string[] FolioNativeLoadException.ProbedPaths
```

Thrown when the native engine cannot be loaded. `RuntimeIdentifier` and `FileName` name what was sought, `PointerSize` reports the detected process bitness as the size of a pointer in bytes, and `ProbedPaths` lists every location that was tried. The message states the likely cause. This is the one failure that is about the installation rather than about the document.

## Determinism

Nothing here reads the clock, the locale, the environment or the network, and nothing touches the filesystem beyond the `Template.Load` call you make yourself. A document that needs a date takes it as an ordinary param named `documentDate`; the `SOURCE_DATE_EPOCH` convenience belongs to the command-line tool and is not honoured by this library.

Templates are immutable here. Neither this library nor folio-js exposes a way to change one.
<!-- twin:end -->

Source of truth: this file. The published page at `docs/folio-dotnet.html` carries the same text.
