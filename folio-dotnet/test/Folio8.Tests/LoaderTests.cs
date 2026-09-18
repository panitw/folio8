using System;
using System.Collections.Generic;
using System.IO;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// CAP-7 and CAP-11, checked on ANY host.
    /// </summary>
    /// <remarks>
    /// The loader's decision is driven through
    /// <see cref="NativeLibraryLoader.Resolve"/> with its inputs injected —
    /// the probe directories, the process's pointer size, how to test for a
    /// file and how to load one. That is deliberate: the three failure modes
    /// CAP-11 names include a bitness mismatch and a host that blocks
    /// P/Invoke, and neither can be produced on demand in the process running
    /// these tests. Injected, every branch runs on macOS, Linux and Windows
    /// alike, and the CONSUMER projects then force the same modes for real
    /// against a real installed package (test/consumers/).
    /// </remarks>
    public class LoaderTests
    {
        private const int ErrorBadExeFormat = 193;
        private static readonly IntPtr Loaded = new IntPtr(0x1234);

        /// <summary>The RID follows the PROCESS, not the machine and not the build.</summary>
        [Theory]
        [InlineData(8, "win-x64")]
        [InlineData(4, "win-x86")]
        public void TheRidFollowsProcessBitness(int pointerSize, string rid)
        {
            Assert.Equal(rid, NativeLibraryLoader.RidFor(pointerSize));
        }

        /// <summary>
        /// The RID layout modern .NET stages is preferred, then the
        /// folio8-native/ tree the package's targets stage for .NET
        /// Framework, then a file staged flat beside the assembly.
        /// </summary>
        [Fact]
        public void ProbeOrderIsRuntimesThenTheFrameworkFolderThenFlat()
        {
            List<string> probed = new List<string>();
            Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(
                    new[] { "/app" }, 8,
                    path => { probed.Add(path); return false; },
                    Never));

            Assert.Equal(3, probed.Count);
            Assert.EndsWith(Path.Combine("runtimes", "win-x64", "native", "folio8_native.dll"), probed[0], StringComparison.Ordinal);
            Assert.EndsWith(Path.Combine("folio8-native", "win-x64", "folio8_native.dll"), probed[1], StringComparison.Ordinal);
            Assert.EndsWith(Path.Combine("/app", "folio8_native.dll"), probed[2], StringComparison.Ordinal);
        }

        /// <summary>A 32-bit process never looks in a win-x64 directory. CAP-7.</summary>
        [Fact]
        public void A32BitProcessProbesOnlyWinX86()
        {
            List<string> probed = new List<string>();
            Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(
                    new[] { "/app" }, 4,
                    path => { probed.Add(path); return false; },
                    Never));

            foreach (string path in probed)
            {
                Assert.DoesNotContain("win-x64", path, StringComparison.Ordinal);
            }
            Assert.Contains(probed, path => path.Contains("win-x86"));
        }

        /// <summary>The first candidate that exists is the one that is loaded.</summary>
        [Fact]
        public void LoadsTheFirstCandidateThatExists()
        {
            string wanted = Path.Combine(Path.Combine(Path.Combine(Path.Combine("/app", "runtimes"), "win-x64"), "native"), "folio8_native.dll");
            string loaded = null;
            IntPtr handle = NativeLibraryLoader.Resolve(
                new[] { "/app" }, 8,
                path => true,
                (string path, out int error) => { loaded = path; error = 0; return Loaded; });

            Assert.Equal(Loaded, handle);
            Assert.EndsWith(wanted, loaded, StringComparison.Ordinal);
        }

        /// <summary>
        /// MISSING RID ASSET. The exception names the bitness, the RID, the
        /// file name and every path probed — CAP-11's whole content, asserted
        /// item by item rather than "an exception was thrown".
        /// </summary>
        [Fact]
        public void AMissingAssetNamesBitnessRidFilenameAndEveryProbedPath()
        {
            FolioNativeLoadException error = Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(new[] { "/app", "/app/bin" }, 8, path => false, Never));

            Assert.Equal("win-x64", error.RuntimeIdentifier);
            Assert.Equal("folio8_native.dll", error.FileName);
            Assert.Equal(8, error.PointerSize);
            Assert.Equal(6, error.ProbedPaths.Length);

            Assert.Contains("64-bit", error.Message, StringComparison.Ordinal);
            Assert.Contains("win-x64", error.Message, StringComparison.Ordinal);
            Assert.Contains("folio8_native.dll", error.Message, StringComparison.Ordinal);
            foreach (string path in error.ProbedPaths)
            {
                Assert.Contains(path, error.Message, StringComparison.Ordinal);
            }
            // No other architecture was tried. Not "should not"; did not.
            Assert.DoesNotContain("win-x86", error.Message, StringComparison.Ordinal);
        }

        /// <summary>
        /// WRONG BITNESS. Windows reports ERROR_BAD_EXE_FORMAT, and the
        /// message says so in words a developer can act on. The platform error
        /// is kept as the inner exception rather than discarded.
        /// </summary>
        [Fact]
        public void AWrongBitnessAssetNamesTheMismatchAndKeepsThePlatformError()
        {
            FolioNativeLoadException error = Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(
                    new[] { "/app" }, 8, path => true,
                    (string path, out int win32) => { win32 = ErrorBadExeFormat; return IntPtr.Zero; }));

            Assert.Contains("BITNESS MISMATCH", error.Message, StringComparison.Ordinal);
            Assert.Contains("64-bit process", error.Message, StringComparison.Ordinal);
            Assert.Contains("win-x64", error.Message, StringComparison.Ordinal);
            Assert.NotNull(error.InnerException);
        }

        /// <summary>The same, the other way round: a 32-bit process handed the x64 file.</summary>
        [Fact]
        public void A32BitProcessNamesTheMismatchToo()
        {
            FolioNativeLoadException error = Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(
                    new[] { "/app" }, 4, path => true,
                    (string path, out int win32) => { win32 = ErrorBadExeFormat; return IntPtr.Zero; }));

            Assert.Contains("BITNESS MISMATCH", error.Message, StringComparison.Ordinal);
            Assert.Contains("32-bit process", error.Message, StringComparison.Ordinal);
            Assert.Contains("win-x86", error.Message, StringComparison.Ordinal);
        }

        /// <summary>
        /// BLOCKED P/INVOKE. A partial-trust or locked-down host refuses the
        /// call into the platform loader itself; that is a different problem
        /// from a bad file and says so.
        /// </summary>
        [Fact]
        public void ABlockedPInvokeNamesTheHostAsTheCause()
        {
            FolioNativeLoadException error = Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(
                    new[] { "/app" }, 8, path => true,
                    (string path, out int win32) => { throw new System.Security.SecurityException("blocked"); }));

            Assert.Contains("P/Invoke", error.Message, StringComparison.Ordinal);
            Assert.Contains("blocked", error.Message, StringComparison.OrdinalIgnoreCase);
            Assert.IsType<System.Security.SecurityException>(error.InnerException);
        }

        /// <summary>
        /// NO SILENT FALLBACK. A candidate that exists and will not load ends
        /// the attempt: the later directories are never reached, because
        /// picking a different file after an explicit one failed is how a
        /// bitness problem turns into a mystery.
        /// </summary>
        [Fact]
        public void AFailedLoadIsNotFollowedByAnotherAttempt()
        {
            int attempts = 0;
            Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(
                    new[] { "/app", "/app/bin" }, 8, path => true,
                    (string path, out int win32) => { attempts++; win32 = ErrorBadExeFormat; return IntPtr.Zero; }));

            Assert.Equal(1, attempts);
        }

        /// <summary>
        /// The exception hands out a COPY of the probed paths: a caller that
        /// edits what it was given cannot change what the exception reports
        /// to the next reader, or to a log sink.
        /// </summary>
        [Fact]
        public void ProbedPathsCannotBeMutatedThroughTheException()
        {
            FolioNativeLoadException error = Assert.Throws<FolioNativeLoadException>(() =>
                NativeLibraryLoader.Resolve(new[] { "/app" }, 8, path => false, Never));

            string[] first = error.ProbedPaths;
            first[0] = "tampered";
            Assert.NotEqual("tampered", error.ProbedPaths[0]);
        }

        /// <summary>
        /// The real entry point is idempotent and, on this host, a no-op or a
        /// success — never a throw. Off Windows it does nothing at all; on
        /// Windows the package or the test project has staged the library.
        /// </summary>
        [Fact]
        public void EnsureIsIdempotent()
        {
            NativeLibraryLoader.Ensure();
            NativeLibraryLoader.Ensure();
        }

        private static IntPtr Never(string path, out int win32Error)
        {
            throw new InvalidOperationException("no candidate existed, so nothing should have been loaded: " + path);
        }
    }
}
