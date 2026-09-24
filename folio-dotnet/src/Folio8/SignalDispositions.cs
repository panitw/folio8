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
/// <b>The order is the whole point and it is not "after the load".</b> A
/// c-shared Go runtime initialises on a thread of its own; <c>dlopen</c>
/// returns before <c>initsig</c> has run, and a restore applied then can
/// be undone moments later. Every export blocks until the runtime is up,
/// so the restore runs after the first export has <i>returned</i> — which
/// in this binding is the ABI check, the first crossing there is.
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

    private static bool IsLinux()
    {
        return RuntimeInformation.IsOSPlatform(OSPlatform.Linux);
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
