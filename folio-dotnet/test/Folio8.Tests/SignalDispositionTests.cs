using System;
using System.Runtime.InteropServices;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// DW-398's fix, asserted as a mechanism: after the first crossing, the
    /// runtime's GC activation signal carries the disposition the runtime
    /// gave it, not the one the engine's load gave it.
    /// </summary>
    /// <remarks>
    /// A run of clean renders is not evidence — DW-396 was "cleared" that
    /// way once and the RIDs shipped on it. So the Linux test here reads
    /// <c>sigaction(SIGRTMIN)</c> directly, the same reading the reproducer
    /// took, and asserts the one bit that decides whether the CLR's
    /// activation handler runs on a megabyte of thread stack or on 16 KiB of
    /// alternate stack. The decision itself is pure and is tested on every
    /// OS against synthetic records, because the host that can produce a
    /// real one is not the host most of CI runs on.
    /// </remarks>
    [Collection("native")]
    public class SignalDispositionTests
    {
        private const int SigSegv = 11;
        private const int SaOnStack = SignalDispositions.SaOnStack;

        private static byte[] Record(long handler, int flags)
        {
            byte[] r = new byte[256];
            BitConverter.GetBytes(handler).CopyTo(r, 0);
            BitConverter.GetBytes(flags).CopyTo(r, 136);
            return r;
        }

        [Fact]
        public void ADispositionTheLoadDidNotChangeIsLeftAlone()
        {
            Assert.False(SignalDispositions.ShouldRestore(Record(0x1000, 0x14000004), Record(0x1000, 0x14000004)));
        }

        [Fact]
        public void AHandlerTheLoadOnlyReflaggedIsRestored()
        {
            // Exactly what Go's setsigstack does: same address, SA_ONSTACK added.
            Assert.True(SignalDispositions.ShouldRestore(Record(0x1000, 0x14000004), Record(0x1000, 0x14000004 | SaOnStack)));
        }

        [Fact]
        public void AHandlerTheLoadReplacedIsNeverTouched()
        {
            // Go's own SIGSEGV handler: different address. Go relies on it.
            Assert.False(SignalDispositions.ShouldRestore(Record(0x1000, 0x14000004 | SaOnStack), Record(0x2000, 0x14000004 | SaOnStack)));
            // Even when the flags happen to match.
            Assert.False(SignalDispositions.ShouldRestore(Record(0x1000, 0x14000004), Record(0x2000, 0x14000004)));
        }

        [Fact]
        public void AMissingRecordOnEitherSideRestoresNothing()
        {
            Assert.False(SignalDispositions.ShouldRestore(null, Record(0x1000, 0)));
            Assert.False(SignalDispositions.ShouldRestore(Record(0x1000, 0), null));
            Assert.False(SignalDispositions.ShouldRestore(null, null));
        }

        [Fact]
        public void OffLinuxTheSnapshotIsNullAndTheRestoreIsANoOp()
        {
            if (RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
            {
                return;
            }
            Assert.Null(SignalDispositions.Take());
            Assert.Equal(string.Empty, SignalDispositions.Restore(null));
        }

        /// <summary>
        /// The assertion that matters, on the host that can make it. Before
        /// the fix, this bit was set on SIGRTMIN in every Linux amd64 process
        /// that had loaded the engine, and that process died on its next GC
        /// more often than not.
        /// </summary>
        [Fact]
        public void OnLinuxTheActivationSignalRunsOnTheThreadStackAfterTheFirstCrossing()
        {
            if (!RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
            {
                return;
            }

            // The first crossing loads the engine, checks the ABI, and
            // restores. Any call will do; this is the smallest.
            string version = Folio8.Version;
            Assert.False(string.IsNullOrEmpty(version));

            int rtmin = SignalDispositions.FlagsOf(SignalDispositions.SigRtMin);
            Assert.NotEqual(-1, rtmin);
            Assert.NotEqual(0L, SignalDispositions.HandlerOf(SignalDispositions.SigRtMin));
            Assert.True((rtmin & SaOnStack) == 0,
                "SIGRTMIN carries SA_ONSTACK after the first crossing (sa_flags 0x" + rtmin.ToString("X") + "): the CLR's GC activation handler " +
                "would run on the 16 KiB alternate stack, which is DW-398. The restore did not run, ran before Go's runtime had initialised, " +
                "or the engine was loaded a second time from another path.");

            // And the handlers Go installed for itself were left alone: its
            // SIGSEGV handler is on the alternate stack by design and must be.
            int segv = SignalDispositions.FlagsOf(SigSegv);
            Assert.True((segv & SaOnStack) != 0, "SIGSEGV lost SA_ONSTACK: the restore touched a handler the load replaced, which it must never do.");
        }
    }
}
