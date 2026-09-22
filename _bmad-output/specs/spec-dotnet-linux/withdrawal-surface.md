# The withdrawal surface

Every site that currently asserts folio-dotnet is Windows-only, or actively
prevents a `linux` RID from shipping. Restoring Linux means **inverting** each
one — a guard that asserted absence now asserts presence — not deleting it.
Deleting a guard removes the thing that would catch a half-restored package.

Paths are repo-relative. Line numbers are as of commit `c778ed2` and are a
finding aid, not a contract.

## Guards that prohibit — invert to assert

| Site | What it does today | After |
| --- | --- | --- |
| [folio-dotnet/src/Folio8/Folio8.csproj](../../../folio-dotnet/src/Folio8/Folio8.csproj) | The two `FolioNative` items for `linux-x64` / `linux-arm64` are commented out with the DW-396 reasoning | The items are live; the comment becomes the record of why they were once withdrawn |
| [PackagingTests.cs](../../../folio-dotnet/test/Folio8.Tests/PackagingTests.cs) `BothNativesArePackedIntoTheRidLayout` | `Assert.DoesNotContain(PackedRids(), rid => rid.StartsWith("linux"))` | Asserts both Linux RIDs **are** packed, beside the two `win-` RIDs |
| [PackagingTests.cs](../../../folio-dotnet/test/Folio8.Tests/PackagingTests.cs) `TheReadmeTellsAnInstallerWhatTheyCannotGuess` | Requires the README to contain `"Windows only"` and `"Why there is no Linux build yet"` | Requires the README to state the Linux RIDs, the glibc floor, and the musl exclusion |
| [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) `folio-dotnet-linux` | ~~Builds and verifies both natives; the `dotnet test` step is **removed**, with a comment naming DW-396~~ — **done in story 3.** Now a two-leg matrix, one per shipped native, each running the corpus suite on real silicon, with the withdrawal comment rewritten rather than deleted | ✅ |

## Claims that must stop being true

| Site | Claim |
| --- | --- |
| [docs/folio-dotnet.md:35](../../../docs/folio-dotnet.md) + [docs/folio-dotnet.html:370](../../../docs/folio-dotnet.html) | "**Windows only, on x86 and on x64.** … There are no Linux, no macOS and no ARM64 native binaries here" — `.md` is source of truth, `.html` is the published page, and the two must agree |
| [folio-dotnet/README.md:118](../../../folio-dotnet/README.md) | "**Windows only**, on **x86** and **x64**", and the section *"Why there is no Linux build yet"* |
| [Folio8.csproj](../../../folio-dotnet/src/Folio8/Folio8.csproj) `<Description>` | "…from .NET Framework 4.6 through modern .NET, **on Windows x86 and x64**" — the NuGet listing text |
| [RELEASING.md:197](../../../RELEASING.md), and the package-contents section at ~line 311 | "folio-dotnet stays Windows-only"; "**NO LINUX RID SHIPS, AND IT IS WITHDRAWN RATHER THAN UNATTEMPTED**"; the prerequisites table marking Docker as "only for the Linux natives, which 1.1.0 does not ship" |
| [DW-396](../../implementation-artifacts/deferred-work.md) | Status OPEN. Closes with the mechanism, the fix, and the soak evidence recorded |

`_bmad-output/specs/spec-client-libraries/` states the Windows-only position
too — as a constraint, a non-goal, and in `packaging-matrix.md`. It is a
**historical record of that effort** and is not rewritten here; this spec is
what supersedes it.

## Tooling that already works and is not rebuilt

All of it survived the withdrawal deliberately. None of it is new work:

- [build-native.sh](../../../folio-dotnet/build/build-native.sh) — `linux-x64` and `linux-arm64` targets, built in the pinned AlmaLinux 8 image (pinned **by digest**), and the `host` target that is never packaged
- [verify-linux-natives.sh](../../../folio-dotnet/build/verify-linux-natives.sh) — existence, ELF machine per RID, and the **glibc 2.28 ceiling** that proves the native came out of the pinned image
- `FolioPackageCheck` in [Folio8.csproj](../../../folio-dotnet/src/Folio8/Folio8.csproj) — the ELF arm, including the `ET_DYN` assertion and the `linux-x64` = `0x003E` / `linux-arm64` = `0x00B7` machine table
- [folio-dotnet.targets](../../../folio-dotnet/build/folio-dotnet.targets) — the .NET Framework staging path, untouched: `net46` is Windows-only by definition, so it never stages a Linux native

## The load path, which is not the problem

[NativeLibraryLoader.cs](../../../folio-dotnet/src/Folio8/NativeLibraryLoader.cs)
does **nothing at all** off Windows, by design: its explicit-load dance exists
because .NET Framework 4.6 has no `DllImportResolver` and AnyCPU decides
bitness at load time. On Linux there is no `net46`, nothing is AnyCPU in that
sense, and the host resolves `runtimes/<rid>/native/` from the RID graph
before `DllImport` ever probes. Its class-level remark ("Off Windows this does
nothing at all… the macOS and Linux host libraries are a development aid")
needs its wording corrected — the Linux libraries are no longer only a
development aid — but its **behaviour** is already right.

⚠ `linux-musl-x64` does **not** inherit `linux-x64` assets in the RID graph.
An Alpine consumer therefore resolves no native from this package and fails at
install-time absence, which is the intended outcome — Go's `-buildmode=c-shared`
emits initial-exec TLS relocations musl's loader refuses under `dlopen`, so a
shipped musl RID would convert that clean absence into a first-render crash.
