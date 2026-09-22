# The folio8 C ABI

`cmd/folio8` is the folio8 engine built as a native library with
`-buildmode=c-shared`. It exists because **no wasm runtime targets .NET
Framework 4.6**, and `DllImport` over a C ABI is the only mechanism that spans
that floor and modern .NET from one codebase. folio-dotnet is its first
caller; this file is the contract they share.

Nothing here renders. Every export is a thin shell over the public
`folio8` API — the same discipline `folio-go/wasm/cmd/render` keeps, and
`imports_test.go` holds this package to it: no `internal/` package, no
`fonts` package, so **every call carries its own font set**.

## Shape of every call

- Inputs are **pointer plus length** byte buffers. Nothing is null-terminated;
  nothing is a struct.
- Every call returns a **status `int32`** and writes three out-parameters:
  an allocation **token**, a **result buffer** pointer, and that buffer's
  **length**.
- The engine owns what it allocates. The caller returns it with **exactly one**
  `folio8_free(token)`.
- **Nothing is stored between calls.** There is no handle, no session and no
  configuration. The only global state is the allocation table.
- All integers in buffers are **little-endian**, which every platform this
  library ships to is.

```c
int32_t folio8_abi_version(void);

int32_t folio8_version(uint64_t *token, void **result, int32_t *len);

int32_t folio8_parse(const void *tpl, int32_t tpl_len,
                     uint64_t *token, void **result, int32_t *len);

int32_t folio8_render(const void *tpl,    int32_t tpl_len,
                      const void *data,   int32_t data_len,
                      const void *params, int32_t params_len,
                      const void *fonts,  int32_t fonts_len,
                      int32_t fallback,
                      uint64_t *token, void **result, int32_t *len);

int32_t folio8_validate(/* identical parameters to folio8_render */);

int32_t folio8_parameter_references(const void *tpl, int32_t tpl_len,
                                    uint64_t *token, void **result, int32_t *len);

int32_t folio8_free(uint64_t token);
int32_t folio8_allocation_count(void);
```

`folio8_parse` returns the **canonical** template bytes as its payload
(`ParseTemplate` followed by `SerializeTemplate`). There is no template handle:
the caller keeps those bytes and hands them back to `folio8_render` and
`folio8_parameter_references`. Re-parsing canonical bytes is cheap and yields
identical output, and it means the boundary needs no deterministic disposal —
which .NET Framework's finalisers do not promise.

`folio8_validate` takes **raw template bytes**, not a parsed template,
matching Go's `Validate`.

`fallback` selects what a render does with a font-chain entry naming a face
this renderer was never given. **0 is strict** — refuse with
`TEXT_FACE_ABSENT`, which is what every call made before this parameter
existed did — and **1 is substitute**: paint the character in a face the
renderer does hold (the document's own embedded assets first, then the
supplied font set, each in face-name order) and report
`TEXT_FACE_SUBSTITUTED`. Any other value is `FOLIO8_ERROR_ARGUMENT`, never a
clamp. Keep the two values in step with `FaceFallback` in
`folio-dotnet/src/Folio8/FaceFallback.cs`.

`params` may be a null pointer or a zero-length buffer; **the two mean the
same thing**, because the engine maps any params of length zero onto the empty
params object (`decodeParams`, `render_entry.go`). There is nothing for a
caller to distinguish and nothing this ABI could carry across if there were.

## Versioning the ABI

`folio8_abi_version()` reports the shape of everything in this file: the
exports, their parameters, the status codes and the frame grammar. It
allocates nothing, issues no token and cannot fail, so a caller may call it
**before anything else** — and must:

> Check `folio8_abi_version()` once, before the first real call, and refuse to
> continue if it is not the version you were built against.

Without that check, a managed assembly paired with a native library from a
different build decodes a frame grammar that has moved under it, and reports
the result as data. Bump it for any change a caller would have to be
recompiled for. It is **2** today: version 2 added the `fallback` parameter
to `folio8_render` and `folio8_validate`, ahead of their out-parameters. A library so old that it does not export
this symbol at all fails at the first call with the platform's
missing-entry-point error, which is also a refusal to continue.

## Status codes

| Value | Name | Meaning |
| --- | --- | --- |
| `0` | `FOLIO8_OK` | The call succeeded. The result buffer is an *ok* frame. |
| `1` | `FOLIO8_ERROR_DIAGNOSTIC` | The engine returned a `*RenderError`. The frame carries its `Diagnostic`. |
| `2` | `FOLIO8_ERROR_MESSAGE` | The engine returned a plain error. The frame carries its message. |
| `3` | `FOLIO8_ERROR_ARGUMENT` | The **call itself** was malformed — a negative length, a null pointer with a positive length, a null out-parameter, a malformed font buffer. **No buffer is allocated and no token is issued**; do not call `folio8_free`. |
| `4` | `FOLIO8_ERROR_PANIC` | A panic was recovered. The frame carries its message. The library stays usable. |
| `5` | `FOLIO8_ERROR_UNKNOWN_FREE` | `folio8_free` only: the token was never issued, or has already been freed. A double free is **refused**, never undefined. |

A result larger than `INT32_MAX` cannot be described by the out-parameter at
all, so it is refused as status `2` with a named message rather than reported
with a wrapped-around length.

A token is issued for every status except `3` and `5`, including the error
statuses — so the caller frees on every path where a buffer came back.

## The result buffer

One frame shape serves every function. `u8` is one byte; `u32` is four
little-endian bytes; `blob` is a `u32` byte length followed by that many bytes;
a string is a `blob` holding UTF-8.

```
frame       := u8 kind , body
kind = 1    (ok)                body := diagnostics , references , payload
kind = 2    (error diagnostic)  body := diagnostic
kind = 3    (error message)     body := blob message

diagnostics := u32 count , diagnostic{count}
references  := u32 count , blob{count}
payload     := blob
diagnostic  := u8 severity , blob code , blob elementId , blob dataPath , blob message
severity    :  1 = warning , 2 = error       (there is no unset value, and a
                                             severity that is neither is
                                             refused, never sent as warning)
```

A section a call does not produce is still written, with a count or length of
zero — so one reader serves every function:

| Call | diagnostics | references | payload |
| --- | --- | --- | --- |
| `folio8_version` | empty | empty | the version string |
| `folio8_parse` | empty | empty | canonical template bytes |
| `folio8_render` | Go's warnings, in Go's order | empty | the PDF |
| `folio8_validate` | Go's diagnostic slice, verbatim | empty | empty |
| `folio8_parameter_references` | empty | Go's names, in Go's order | empty |

Diagnostic codes, severities and message text are **Go's, unchanged**. `code`
is the contract; `message` is prose and must never be parsed.

A **warning** reaches the `diagnostics` section of an *ok* frame. An **error**
aborts the call and arrives as kind `2` or `3` — it never appears in a
successful frame. That is the same split `Result.Diagnostics` and `*RenderError`
make in Go.

## The font buffer

A font set arrives as **one** buffer holding a length-prefixed sequence of
name/bytes pairs, running to the end of the buffer:

```
fonts := ( blob name , blob face )*
```

**Order carries no meaning.** The engine's font set is a map keyed by face
name, so the sequence the pairs arrive in does not reach the engine and is not
promised to. What order would otherwise decide is which of two entries sharing
a name wins — so a **duplicate face name is refused** as a malformed buffer
(`FOLIO8_ERROR_ARGUMENT`) rather than resolved silently in favour of the last
one seen.

An empty buffer is an empty font set, and **that is a legitimate call**.
There is no default font set and no ambient lookup — the engine never goes
looking for fonts on the machine it runs on — but a document that carries
every face it names has nothing for a font set to contribute, and nothing is
refused before the engine has tried to resolve the chains. An entry that
genuinely cannot be resolved still fails, with a located `TEXT_FACE_ABSENT`.
A buffer that does not decode cleanly is `FOLIO8_ERROR_ARGUMENT`.

## Ownership and lifetime

- The result buffer is valid until its token is freed, and not afterwards.
- `folio8_free` frees **one** token and returns `0`; a second call with the
  same token returns `5` and does nothing.
- `folio8_allocation_count()` reports how many tokens are outstanding. It
  exists so a caller can **prove** free discipline over a loop rather than
  assert it: take the count, run the loop, take it again.
- Calls are safe to make from several threads as far as *this library's own
  state* goes: the allocation table is mutex-guarded and no other state
  crosses calls. That is not the whole of the thread-safety contract — read
  the next section before calling from a runtime of your own.

## Threads, and the alternate signal stack

**Nothing here serialises calls, and nothing pins them to a thread.** Render,
validate and free may all be entered concurrently, from any thread, in any
order, and `folio8_free` need not run on the thread that produced the token.
The engine keeps no per-thread state.

**The caveat is not about this library's data; it is about signals.** A
`c-shared` Go library brings the Go runtime with it, and that runtime installs
`SA_ONSTACK` handlers — for stack growth, for preemption, for its own
profiling. Under cgo, Go **adopts the alternate signal stack it finds on the
calling thread** rather than installing its own (`minitSignalStack`, at
`needm` time), and it never checks that the one it found is large enough for
the frames it will push. So the handler runs in whatever room the calling
thread's `sigaltstack` already had.

**If your runtime installs its own alternate signal stack, size it before the
first call, on every thread that will ever cross this ABI.** A stack that is
too small does not fail cleanly: the kernel turns the overflow into `SIGSEGV`,
the host's own fault handler sees corruption it cannot explain, and the
process dies with no stack trace that names anything here. Two properties
matter and both are easy to get wrong:

- **Before the first crossing.** Go reads the stack once, when the thread
  first attaches, and the reading is fixed for the life of that attachment.
  Enlarging it afterwards changes nothing.
- **For the life of the thread.** A thread that can still take a signal must
  still own its alternate stack, so the memory must outlive the thread rather
  than be freed when the call returns.

A caller with no alternate signal stack of its own — an ordinary C program, or
a Go caller — needs none of this: Go installs its own 32 KiB stack when it
finds nothing to adopt. The trap is specifically a *managed* runtime that
installs a small fixed one, and the .NET CLR is the instance this repository
measured: folio-dotnet's Linux natives were withdrawn from its 1.1.0 release
over exactly this, and it now creates its own long-lived threads, calls
`sigaltstack()` on each with a megabyte before that thread's first crossing,
and never enters this ABI from a runtime-owned thread.

⚠ **Read your own size; do not copy ours.** The readings behind that work are
**arm64: 24576 bytes**, taken in containers on an Apple Silicon hypervisor and
not yet re-taken on a bare host, and **amd64: 16384 bytes**, taken under
**emulation** and therefore *not* admissible as evidence about real amd64
silicon. What they establish is the shape — a managed runtime's fixed altstack
sitting below the 32 KiB Go sizes for itself — not a constant to size against.
Take your own reading with `sigaltstack(NULL, &old)` on the thread that will
cross, and then size **well above** it: a few hundred KB per thread costs
nothing, and there is no single right queried value either, since `SIGSTKSZ`
is a compile-time constant before glibc 2.34 and a `sysconf()`-backed runtime
value after it. The measurements and their provenance are in `DW-396` in this
repository's deferred-work record.

**Darwin callers: `stack_t` is not laid out the way Linux lays it out.**
glibc is `void *ss_sp; int ss_flags; <4 bytes padding>; size_t ss_size`, and
the BSD/Darwin family is `void *ss_sp; size_t ss_size; int ss_flags`. The
wrong declaration compiles cleanly and does not obviously misbehave — it
passes a size of zero where the kernel expects one and comes back `ENOMEM`,
which is measured, on macOS, against the Linux layout. So pick the layout by
the kernel you are on rather than by assumption, **check the return value**,
and read the stack back to confirm the size you actually got.

**Windows callers are unaffected.** There are no POSIX signals there and no
alternate signal stack to size.

## Determinism

No clock, no locale, no environment, no network, no filesystem. `documentDate`
arrives through `params` or it is absent; `SOURCE_DATE_EPOCH` is a CLI
behaviour and is not honoured here.

## The library's file name

The built library is **`folio8_native`** — `folio8_native.dll` on Windows,
`libfolio8_native.dylib` / `libfolio8_native.so` elsewhere — and **not**
`folio8`. That is a requirement on every caller that ships it, not a
preference.

folio-dotnet's managed assembly is `Folio8.dll`. NTFS, like the default macOS
file system, is **case-insensitive**: `folio8.dll` and `Folio8.dll` are one
file. Staged into a single directory — which is exactly what a NuGet RID asset
does on modern .NET, and what any test project does — one silently overwrites
the other, and the loader then resolves a valid PE file containing none of the
exports above.

Measured, not theorised. The binding's first Windows CI run failed every
managed test with `EntryPointNotFoundException` on the first call, while
`objdump -p` showed both DLLs exporting all eight symbols, undecorated. The
same sources passed on macOS, where the native file is `libfolio8*.dylib` and
no collision was possible — so the platform that could catch it was the only
one that could not.

## Building

`folio-dotnet/build/build-native.sh` and `build-native.ps1` are the only
supported way to build it — they pin `CGO_ENABLED=1`, the toolchain, and the
output layout. Windows artifacts (`win-x86`, `win-x64`) need a mingw-w64
toolchain per architecture; the `linux-x64` and `linux-arm64` artifacts are
built inside a digest-pinned AlmaLinux 8 image, which is what holds their
glibc floor. The **host** library the scripts also build (a macOS `.dylib` or
a Linux `.so` compiled on whatever machine you are sitting at) is a
**development aid** so the ABI and the binding can be exercised locally; it is
never packaged and never shipped, and it is not the same artifact as the
`linux-*` targets above.
