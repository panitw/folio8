using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;

/// <summary>
/// Puts the runtime's own signal dispositions back after the engine has
/// loaded — the fix for DW-398, which is also DW-396's cause.
/// </summary>
/// <remarks>
/// <para>
/// Loading a Go c-shared library runs Go's <c>initsig</c>. For the signals
/// Go handles itself it installs its own handlers; for every OTHER signal
/// that already has a handler it re-installs that handler <b>with
/// <c>SA_ONSTACK</c> added</b> (<c>runtime.setsigstack</c>), so that a
/// foreign handler landing on one of Go's small-stacked threads runs on
/// the alternate stack. On the CLR that flag lands on handlers the CLR
/// installed deliberately without it — including its GC activation signal,
/// <c>SIGRTMIN</c>, whose handler carries a multi-kilobyte frame and was
/// sized for the thread's ordinary stack. Once relocated onto the CLR's
/// fixed 16 KiB alternate stack, under a kernel signal frame that on x86
/// carries the full XSAVE state, it overflows: a segfault when what lies
/// below the alternate stack is unmapped, a trampled mutex and
/// <c>futex_fatal_error</c> when it is another allocation, and the
/// kernel's <c>overflowed sigaltstack</c> when a second signal arrives
/// while it is already over the edge. Measured on Linux amd64 at 55–60%
/// of processes that load the engine and then collect garbage; never on
/// arm64, whose signal frames are small. A thread pool worker suspended
/// for GC is the thread that dies, and it is not a thread this binding
/// owns, so no amount of work on engine threads reaches it.
/// </para>
/// <para>
/// The fix is to undo exactly that edit and nothing else: for every
/// signal whose <b>handler address the load did not change</b> but whose
/// flags it did, write the pre-load <c>struct sigaction</c> back
/// verbatim. Handlers Go installed are left alone — Go relies on them —
/// and handlers Go merely re-flagged are not Go's to keep. Under cgo Go's
/// own threads are ordinary pthreads with ordinary stacks, so a CLR
/// handler that reaches one of them without <c>SA_ONSTACK</c> has room.
/// </para>
/// <para>
/// <b>This class is the fallback; the engine closes the window itself.</b>
/// Go's <c>initsig</c> runs <i>inside</i> <c>dlopen</c>, from the
/// library's own constructor, on the loading thread — not later on the
/// runtime's thread, as an earlier version of this remark claimed. So
/// between that edit and any restore the host makes afterwards, every GC
/// activation on every other thread runs on the small stack exactly as
/// before the fix, and that window killed one to four processes in a
/// hundred under GC pressure (DW-398, run 36006849443). The engine's
/// <c>dispositions_linux.c</c> now puts the flags back in a constructor
/// of its own that the linker places right after Go's, so on a current
/// native <see cref="RestoreOnce"/> finds nothing to do. It runs anyway,
/// after the first export has returned — by then every edit is in place —
/// for an older native, and so the assertion has two witnesses:
/// <c>sigaction</c> read back, and the engine's own report through
/// <c>folio8_signal_dispositions</c>.
/// </para>
/// <para>
/// Linux only, by the kernel's name. The struct layout below is glibc's
/// on both x86_64 and aarch64 — handler at 0, a 128-byte mask at 8, flags
/// at 136 — and the flag itself is Linux's. Darwin's Go runtime does the
/// same thing to a differently laid-out struct on a platform where the
/// defect has not been measured; it is deliberately not touched here.
/// </para>
/// </remarks>
internal static class SignalDispositions
{
    /// <summary>glibc's <c>SIGRTMIN</c>: 32 and 33 are NPTL's, so the first real-time signal is 34.</summary>
    internal const int SigRtMin = 34;

    /// <summary><c>SA_ONSTACK</c> on Linux.</summary>
    internal const int SaOnStack = 0x08000000;

    // glibc `struct sigaction`, LP64 Linux: `union handler` (8) at 0,
    // `sigset_t sa_mask` (128) at 8, `int sa_flags` at 136, `sa_restorer`
    // at 144. 152 bytes; the buffer is larger so a future field cannot
    // run off the end of it.
    private const int HandlerOffset = 0;
    private const int FlagsOffset = 136;
    private const int RecordBytes = 256;

    [DllImport("libc", EntryPoint = "sigaction", SetLastError = true)]
    private static extern int sigaction_linux(int signum, byte[] act, byte[] oldact);

    /// <summary>Every catchable signal's disposition at one moment, indexed by signal number.</summary>
    internal sealed class Snapshot
    {
        internal readonly byte[][] Records = new byte[65][];
    }

    // THE SNAPSHOT IS TAKEN WHERE THE LOAD HAPPENS, NOT WHERE THE FIRST CALL
    // HAPPENS. The first version snapshotted inside the ABI check, which is
    // the first CROSSING -- but not necessarily the first LOAD: anything that
    // reaches NativeLibraryLoader.Ensure() earlier (a test does) had already
    // brought Go up, the snapshot then recorded Go's edits as the baseline,
    // and there was nothing to put back. CI run 35995186412 caught it: the
    // same assertion passed in one Linux job and failed in another, on test
    // order alone. So Ensure() calls TakeOnce() immediately before its
    // dlopen, whichever caller got there first, and the ABI check calls
    // RestoreOnce() after the first export has returned. Both latch.
    private static readonly object Gate = new object();
    private static bool _taken;
    private static Snapshot _before;
    private static string _restored;

    /// <summary>
    /// Records every disposition, once per process, and only the first time —
    /// a second call after the engine has loaded must not overwrite the
    /// pre-load record with a post-load one. Call it immediately before the
    /// load. Cheap and harmless off Linux.
    /// </summary>
    internal static void TakeOnce()
    {
        lock (Gate)
        {
            if (_taken)
            {
                return;
            }
            _taken = true;
            _before = Take();
        }
    }

    /// <summary>
    /// Puts the recorded dispositions back where the load re-flagged them,
    /// once, and remembers what it restored. Safe to call again; later calls
    /// do nothing. Call it after the first export has returned, never merely
    /// after the load.
    /// </summary>
    internal static string RestoreOnce()
    {
        lock (Gate)
        {
            if (_restored != null)
            {
                return _restored;
            }
            _restored = Restore(_before);
            return _restored;
        }
    }

    /// <summary>
    /// What <see cref="RestoreOnce"/> put back, comma-separated; empty until
    /// it has run, and empty off Linux. Whether the snapshot was ever taken
    /// is reported separately so a test can tell "nothing needed restoring"
    /// from "nothing was ever recorded".
    /// </summary>
    internal static string Restored
    {
        get { lock (Gate) { return _restored ?? string.Empty; } }
    }

    internal static bool SnapshotTaken
    {
        get { lock (Gate) { return _taken; } }
    }

    /// <summary>
    /// Reads every catchable signal's disposition. Returns <c>null</c> off
    /// Linux, and on a Linux where <c>sigaction</c> is not reachable —
    /// there is then nothing to restore and nothing to refuse over.
    /// </summary>
    internal static Snapshot Take()
    {
        if (!IsLinux())
        {
            return null;
        }
        Snapshot snapshot = new Snapshot();
        for (int sig = 1; sig <= 64; sig++)
        {
            if (!Catchable(sig))
            {
                continue;
            }
            byte[] record = new byte[RecordBytes];
            try
            {
                if (sigaction_linux(sig, null, record) != 0)
                {
                    continue;
                }
            }
            catch (DllNotFoundException)
            {
                return null;
            }
            catch (EntryPointNotFoundException)
            {
                return null;
            }
            snapshot.Records[sig] = record;
        }
        return snapshot;
    }

    /// <summary>
    /// Writes back every disposition the load re-flagged without
    /// replacing, and reports which. Returns an empty string when there
    /// was nothing to do — including off Linux and on a null snapshot.
    /// </summary>
    internal static string Restore(Snapshot before)
    {
        if (before == null || !IsLinux())
        {
            return string.Empty;
        }
        List<string> restored = new List<string>();
        for (int sig = 1; sig <= 64; sig++)
        {
            byte[] then = before.Records[sig];
            if (then == null)
            {
                continue;
            }
            byte[] now = new byte[RecordBytes];
            if (sigaction_linux(sig, null, now) != 0)
            {
                continue;
            }
            if (!ShouldRestore(then, now))
            {
                continue;
            }
            if (sigaction_linux(sig, then, null) == 0)
            {
                restored.Add(Name(sig));
            }
        }
        return string.Join(",", restored.ToArray());
    }

    /// <summary>
    /// The one decision, kept pure so it can be tested off Linux: restore
    /// a signal when the load left its handler where it was and changed
    /// its flags. A handler the load REPLACED is the loader's own and is
    /// never touched; a disposition the load did not change needs nothing.
    /// </summary>
    internal static bool ShouldRestore(byte[] before, byte[] after)
    {
        if (before == null || after == null)
        {
            return false;
        }
        long handlerThen = BitConverter.ToInt64(before, HandlerOffset);
        long handlerNow = BitConverter.ToInt64(after, HandlerOffset);
        if (handlerThen != handlerNow)
        {
            return false;
        }
        return BitConverter.ToInt32(before, FlagsOffset) != BitConverter.ToInt32(after, FlagsOffset);
    }

    /// <summary>The flags a signal currently carries, or <c>-1</c> if they cannot be read. Linux only.</summary>
    internal static int FlagsOf(int sig)
    {
        if (!IsLinux())
        {
            return -1;
        }
        byte[] record = new byte[RecordBytes];
        return sigaction_linux(sig, null, record) == 0 ? BitConverter.ToInt32(record, FlagsOffset) : -1;
    }

    /// <summary>The handler address a signal currently carries, or <c>0</c>. Linux only.</summary>
    internal static long HandlerOf(int sig)
    {
        if (!IsLinux())
        {
            return 0;
        }
        byte[] record = new byte[RecordBytes];
        return sigaction_linux(sig, null, record) == 0 ? BitConverter.ToInt64(record, HandlerOffset) : 0;
    }

    // 9 and 19 cannot be caught; 32 and 33 are glibc's and sigaction on
    // them is refused.
    private static bool Catchable(int sig)
    {
        return sig != 9 && sig != 19 && sig != 32 && sig != 33;
    }

    // NOT RuntimeInformation: that arrived in .NET Framework 4.7.1 and this
    // assembly's floor is 4.6 — the Net46Compile project reddened on it (CI
    // run 35992568594). The same reading EngineThreads takes: a POSIX
    // PlatformID, then the kernel's own name from uname(2). A null name is a
    // missing answer and reads as "not Linux", which here means "do nothing".
    private static bool IsLinux()
    {
        return EngineThreads.IsPosix() && string.Equals(EngineThreads.KernelName(), "Linux", StringComparison.Ordinal);
    }

    private static string Name(int sig)
    {
        switch (sig)
        {
            case 4: return "SIGILL";
            case 5: return "SIGTRAP";
            case 6: return "SIGABRT";
            case 15: return "SIGTERM";
            case SigRtMin: return "SIGRTMIN";
            default: return "SIG" + sig.ToString(System.Globalization.CultureInfo.InvariantCulture);
        }
    }
}
