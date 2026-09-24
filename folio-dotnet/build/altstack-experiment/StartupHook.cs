using System;
using System.Runtime.InteropServices;

// Loaded via DOTNET_STARTUP_HOOKS so the experiment touches no shipped code.
//
// FOLIO_EXP selects the arm:
//   none          the shipped binding, untouched
//   noonstack     SA_ONSTACK cleared from Go's THREE handlers (SEGV/BUS/URG)
//   restore       the CLR's original SEGV/BUS/URG handlers put back
//   noonstack-all SA_ONSTACK cleared from EVERY signal that has a handler
//   report        print every signal's handler and flags, change nothing
//
// WHY `noonstack-all` EXISTS, AND WHY THE FIRST THREE ARMS WERE NOT ENOUGH.
// Go does not only REPLACE handlers; it ADDS SA_ONSTACK to handlers other
// runtimes installed (`runtime.setsigstack`), which the handler-scope probe
// measured directly: the CLR's own SIGABRT handler went from no SA_ONSTACK to
// SA_ONSTACK at dlopen, with the handler address unchanged. CoreCLR suspends
// threads for GC with an ACTIVATION SIGNAL -- SIGRTMIN on Linux -- and that
// handler does real work, condvar waits included. If Go moved it onto the
// thread's 16 KiB alternate stack, the CLR is running a handler it sized for
// an ordinary stack in a quarter of the room, and a futex that comes back
// corrupted is what that looks like.
//
// The first three arms cleared SEGV, BUS and URG only, so every one of them
// left SIGRTMIN exactly as Go had rewritten it. All three crashed at the same
// rate, which is consistent with none of them having touched the signal that
// matters.
internal sealed class StartupHook
{
    private const int SA_ONSTACK = 0x08000000;

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaction(int signum, byte[] act, byte[] oldact);

    private static byte[] Read(int sig)
    {
        var b = new byte[256];
        return sigaction(sig, null, b) == 0 ? b : null;
    }

    private static long HandlerOf(byte[] a) { return BitConverter.ToInt64(a, 0); }
    private static int FlagsOf(byte[] a) { return BitConverter.ToInt32(a, 136); }

    // 9 and 19 cannot be caught; 32 and 33 are glibc's own NPTL signals and
    // sigaction on them is refused. Everything else is fair game.
    private static bool Touchable(int sig)
    {
        return sig != 9 && sig != 19 && sig != 32 && sig != 33 && sig >= 1 && sig <= 64;
    }

    private static string Name(int sig)
    {
        switch (sig)
        {
            case 6: return "SIGABRT"; case 7: return "SIGBUS"; case 11: return "SIGSEGV";
            case 8: return "SIGFPE"; case 13: return "SIGPIPE"; case 23: return "SIGURG";
            case 27: return "SIGPROF"; case 34: return "SIGRTMIN"; case 35: return "SIGRTMIN+1";
            case 36: return "SIGRTMIN+2"; default: return "SIG" + sig;
        }
    }

    public static void Initialize()
    {
        string mode = Environment.GetEnvironmentVariable("FOLIO_EXP") ?? "none";
        string so = Environment.GetEnvironmentVariable("FOLIO_SO");
        if (string.IsNullOrEmpty(so)) { Console.Error.WriteLine("[hook] FOLIO_SO unset"); return; }

        int[] three = { 11, 7, 23 };
        var before = new byte[three.Length][];
        for (int i = 0; i < three.Length; i++) before[i] = Read(three[i]);

        // Snapshot every signal's flags BEFORE the load, so `report` can show
        // exactly what dlopen changed rather than what it ended up as.
        var flagsBefore = new int[65];
        var handlerBefore = new long[65];
        for (int sig = 1; sig <= 64; sig++)
        {
            if (!Touchable(sig)) continue;
            byte[] a = Read(sig);
            if (a == null) continue;
            flagsBefore[sig] = FlagsOf(a); handlerBefore[sig] = HandlerOf(a);
        }

        NativeLibrary.Load(so);   // Go installs and REWRITES handlers here

        if (mode == "report")
        {
            Console.Error.WriteLine("[hook] signals whose flags or handler CHANGED at dlopen:");
            for (int sig = 1; sig <= 64; sig++)
            {
                if (!Touchable(sig)) continue;
                byte[] a = Read(sig);
                if (a == null) continue;
                int f = FlagsOf(a); long h = HandlerOf(a);
                if (f == flagsBefore[sig] && h == handlerBefore[sig]) continue;
                Console.Error.WriteLine(string.Format(
                    "[hook]   {0,-12} handler 0x{1:X}->0x{2:X}  ONSTACK {3}->{4}",
                    Name(sig), handlerBefore[sig], h,
                    (flagsBefore[sig] & SA_ONSTACK) != 0 ? "yes" : "no",
                    (f & SA_ONSTACK) != 0 ? "yes" : "no"));
            }
            return;
        }

        if (mode == "none") { return; }

        if (mode == "noonstack-all")
        {
            int cleared = 0;
            for (int sig = 1; sig <= 64; sig++)
            {
                if (!Touchable(sig)) continue;
                byte[] a = Read(sig);
                if (a == null) continue;
                long h = HandlerOf(a);
                if (h == 0 || h == 1) continue;            // SIG_DFL / SIG_IGN
                int f = FlagsOf(a);
                if ((f & SA_ONSTACK) == 0) continue;
                BitConverter.GetBytes(f & ~SA_ONSTACK).CopyTo(a, 136);
                if (sigaction(sig, a, null) == 0) cleared++;
            }
            Console.Error.WriteLine("[hook] cleared SA_ONSTACK from " + cleared + " signal(s)");
            return;
        }

        for (int i = 0; i < three.Length; i++)
        {
            if (mode == "restore")
            {
                if (before[i] != null) sigaction(three[i], before[i], null);
            }
            else if (mode == "noonstack")
            {
                byte[] cur = Read(three[i]);
                if (cur == null) continue;
                BitConverter.GetBytes(FlagsOf(cur) & ~SA_ONSTACK).CopyTo(cur, 136);
                sigaction(three[i], cur, null);
            }
        }
        Console.Error.WriteLine("[hook] applied " + mode);
    }
}
