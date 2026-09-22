using System;
using System.IO;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// The native library's file name must not collide with the managed
    /// assembly's, on a case-INSENSITIVE file system.
    /// </summary>
    /// <remarks>
    /// This is not a hypothetical. The binding's first Windows CI run failed
    /// every managed test with <c>EntryPointNotFoundException</c> on the first
    /// P/Invoke, while <c>objdump -p</c> showed the DLL exporting all eight
    /// <c>folio8_</c> symbols, undecorated, from both architectures. The cause
    /// was the name: the managed assembly is <c>Folio8.dll</c> and the native
    /// library was <c>folio8.dll</c>, which on NTFS is THE SAME FILE. Both
    /// were staged into one output directory, one overwrote the other, and
    /// <c>DllImport</c> loaded a valid PE with no exports in it.
    /// <para>
    /// macOS could not catch it — the native file there is
    /// <c>libfolio8*.dylib</c>, a different name, so all 65 tests passed while
    /// Windows failed all of them. Hence a test that compares the NAMES rather
    /// than one that renders a document: it fails on every platform.
    /// </para>
    /// </remarks>
    public class NativeNamingTests
    {
        [Fact]
        public void TheNativeLibraryNameCannotCollideWithTheManagedAssembly()
        {
            string managed = Path.GetFileNameWithoutExtension(typeof(Template).Assembly.Location);
            Assert.False(
                string.Equals(managed, Native.Library, StringComparison.OrdinalIgnoreCase),
                "the native library is named '" + Native.Library + "' and the managed assembly is '" + managed +
                "'. On a case-insensitive file system those are one file, so staging both into an output directory " +
                "loses one of them and DllImport loads whichever survived.");
        }

        /// <summary>
        /// And the same check against what is actually on disk beside the test
        /// assembly, which is what the runtime resolves — a name that differs
        /// only by a prefix or an extension would still pass the comparison
        /// above while colliding here.
        /// </summary>
        [Fact]
        public void NoTwoStagedFilesDifferOnlyByCase()
        {
            // AppContext.BaseDirectory, not Assembly.CodeBase: the latter is
            // obsolete on modern .NET, and this is the directory the runtime
            // actually probes for a native library on both targets.
            string directory = AppContext.BaseDirectory;
            System.Collections.Generic.Dictionary<string, string> seen =
                new System.Collections.Generic.Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
            foreach (string path in Directory.GetFiles(directory))
            {
                string name = Path.GetFileName(path);
                string other;
                if (seen.TryGetValue(name, out other))
                {
                    Assert.Fail("'" + name + "' and '" + other + "' differ only by case; on Windows they are one file.");
                }
                seen.Add(name, name);
            }
            // VACUITY GUARD: a directory listing that found nothing is not a pass.
            Assert.NotEmpty(seen);
        }

        /// <summary>
        /// The engine answers. This is the call the ABI check makes, asserted
        /// on its own so a failure says "the native library is not reachable"
        /// rather than surfacing as sixty-four unrelated red tests.
        /// </summary>
        [Fact]
        public void TheEngineIsReachableAndAnswers()
        {
            // Through the engine threads: this crossing is no more exempt
            // than any other, and on Linux the xunit thread's alternate
            // signal stack is the one DW-396 overflows.
            Assert.Equal(Native.ExpectedAbiVersion, EngineThreads.Run(new Func<int>(Native.folio8_abi_version)));
            Assert.False(string.IsNullOrEmpty(Folio8.Version));
        }
    }
}
