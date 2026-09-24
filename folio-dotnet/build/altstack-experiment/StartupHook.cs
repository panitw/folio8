using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;

// Loaded via DOTNET_STARTUP_HOOKS so the experiment touches no shipped code.
//
// FOLIO_EXP selects the arm:
//   none             the shipped binding, untouched
//   restore-foreign  every signal Go only RELOCATED -- same handler address,
//                    different flags or mask -- put back exactly as the CLR
//                    had it, byte for byte
//   restore-rtmin    the same, for SIGRTMIN alone
//   noonstack-all    SA_ONSTACK cleared from every signal that has a handler
//   noonstack        SA_ONSTACK cleared from Go's three (SEGV/BUS/URG)
//   restore          the CLR's original SEGV/BUS/URG handlers put back
//   report           print every signal's handler, flags and mask; change nothing
//
// WHY `restore-foreign` EXISTS, AND WHY EVERY EARLIER ARM WAS LOOKING AT THE
// WRONG FIELD. Run 35979184685 settled the stack-size question in the
// negative: clearing SA_ONSTACK from EVERY handled signal changed nothing
// (65 deaths/150 against 69/150). No handler is dying for want of room.
//
// But the same run's report block confirmed the premise underneath it. At
// dlopen, Go leaves five of the CLR's handlers at their original addresses
// and rewrites their flags anyway -- SIG4, SIG5, SIGABRT, SIG15 and SIGRTMIN,
// the last being CoreCLR's thread-suspension activation signal. Go does that
// with sigaction(), which replaces the WHOLE struct: sa_flags and sa_mask
// together. SA_ONSTACK is merely the bit the earlier report happened to print.
//
// sa_mask is the signals blocked while the handler runs. A handler installed
// by one runtime and re-flagged by another can end up re-entrant, or
// interruptible where its author assumed it was not -- and the crash we have
// is glibc's futex_fatal_error out of pthread_cond_wait UNDER a
// `<signal handler called>` frame, which is what condvar state looks like
// when a handler re-enters it. Clearing one flag bit would not touch that;
// putting the CLR's own struct back does.
//
// This arm is also the only one so far whose shape could ship: it restores
// handlers Go did not install and does not want, so nothing Go relies on is
// disturbed. The three signals Go genuinely REPLACED are deliberately left
// alone -- Go needs SEGV, BUS and URG, and `restore` (which took them back)
// is kept only as the record of an arm already run.
internal sealed class StartupHook
{
    private const int SA_ONSTACK   = 0x08000000;
    private const int SA_RESTART   = 0x10000000;
    private const int SA_NODEFER   = 0x40000000;
    private const int SA_RESETHAND = unchecked((int)0x80000000);
    private const int SA_SIGINFO   = 0x00000004;
    private const int SA_RESTORER  = 0x04000000;

    // glibc x86_64 `struct sigaction`: handler at 0, sa_mask at 8 (128 bytes),
    // sa_flags at 136, sa_restorer at 144. 256 is room to spare; glibc reads
    // only what it needs.
    //
    // ONLY THE FIRST 8 BYTES OF sa_mask ARE REAL. Linux has 64 signals; the
    // kernel fills 8 bytes and glibc's sigaction() copies the whole 128-byte
    // field out of a kernel struct whose tail it never initialised. Run
    // 35983414123 compared all 128 and reported every signal's mask as
    // CHANGED, including ones nothing had touched, because the tail was
    // stack garbage that differed between two calls. Comparing and rendering
    // the same 8 bytes is what makes the two agree.
    private const int MASK_OFF  = 8;
    private const int MASK_LEN  = 8;
    private const int FLAGS_OFF = 136;

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaction(int signum, byte[] act, byte[] oldact);

    private static byte[] Read(int sig)
    {
        var b = new byte[256];
        return sigaction(sig, null, b) == 0 ? b : null;
    }

    private static long HandlerOf(byte[] a) { return BitConverter.ToInt64(a, 0); }
    private static int FlagsOf(byte[] a) { return BitConverter.ToInt32(a, FLAGS_OFF); }

    private static bool MaskEquals(byte[] x, byte[] y)
    {
        for (int i = MASK_OFF; i < MASK_OFF + MASK_LEN; i++) if (x[i] != y[i]) return false;
        return true;
    }

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

    private static string Flags(int f)
    {
        var s = new List<string>();
        if ((f & SA_ONSTACK) != 0) s.Add("ONSTACK");
        if ((f & SA_RESTART) != 0) s.Add("RESTART");
        if ((f & SA_NODEFER) != 0) s.Add("NODEFER");
        if ((f & SA_RESETHAND) != 0) s.Add("RESETHAND");
        if ((f & SA_SIGINFO) != 0) s.Add("SIGINFO");
        if ((f & SA_RESTORER) != 0) s.Add("RESTORER");
        int known = SA_ONSTACK | SA_RESTART | SA_NODEFER | SA_RESETHAND | SA_SIGINFO | SA_RESTORER;
        int rest = f & ~known;
        if (rest != 0) s.Add("0x" + rest.ToString("X"));
        return s.Count == 0 ? "-" : string.Join("|", s);
    }

    // sa_mask as the signal numbers it blocks, run-length collapsed, because
    // "1-31,34-64" is readable and 128 bytes of hex is not.
    private static string Mask(byte[] a)
    {
        var on = new List<int>();
        for (int sig = 1; sig <= 64; sig++)
            if ((a[MASK_OFF + ((sig - 1) / 8)] & (1 << ((sig - 1) % 8))) != 0) on.Add(sig);
        if (on.Count == 0) return "none";
        var parts = new List<string>();
        int i = 0;
        while (i < on.Count)
        {
            int j = i;
            while (j + 1 < on.Count && on[j + 1] == on[j] + 1) j++;
            parts.Add(i == j ? on[i].ToString() : on[i] + "-" + on[j]);
            i = j + 1;
        }
        return string.Join(",", parts);
    }

    public static void Initialize()
    {
        string mode = Environment.GetEnvironmentVariable("FOLIO_EXP") ?? "none";
        string so = Environment.GetEnvironmentVariable("FOLIO_SO");
        if (string.IsNullOrEmpty(so)) { Console.Error.WriteLine("[hook] FOLIO_SO unset"); return; }

        // The whole struct for every signal, not just the two fields an
        // earlier arm thought were the interesting ones.
        var before = new byte[65][];
        for (int sig = 1; sig <= 64; sig++)
            if (Touchable(sig)) before[sig] = Read(sig);

        int[] three = { 11, 7, 23 };

        NativeLibrary.Load(so);   // Go installs and REWRITES handlers here

        if (mode == "report")
        {
            Console.Error.WriteLine("[hook] signals whose handler, flags or mask CHANGED at dlopen:");
            for (int sig = 1; sig <= 64; sig++)
            {
                if (before[sig] == null) continue;
                byte[] a = Read(sig);
                if (a == null) continue;
                long hb = HandlerOf(before[sig]), h = HandlerOf(a);
                int fb = FlagsOf(before[sig]), f = FlagsOf(a);
                bool maskSame = MaskEquals(before[sig], a);
                if (h == hb && f == fb && maskSame) continue;
                Console.Error.WriteLine(string.Format("[hook]   {0,-12} {1}",
                    Name(sig), h == hb ? "handler UNCHANGED 0x" + h.ToString("X")
                                       : "handler 0x" + hb.ToString("X") + " -> 0x" + h.ToString("X")));
                Console.Error.WriteLine(string.Format("[hook]       flags {0}  ->  {1}", Flags(fb), Flags(f)));
                Console.Error.WriteLine(string.Format("[hook]       mask  {0}  ->  {1}{2}",
                    Mask(before[sig]), Mask(a), maskSame ? "   (unchanged)" : "   *** CHANGED ***"));
            }
            return;
        }

        if (mode == "none") { return; }

        // The two arms that matter now. A signal Go only RELOCATED is one
        // whose handler address survived the load -- Go did not install it,
        // it edited someone else's. Putting the original struct back restores
        // flags and mask together, which is the pair sigaction() overwrote.
        if (mode == "restore-foreign" || mode == "restore-rtmin")
        {
            int restored = 0;
            var touched = new List<string>();
            for (int sig = 1; sig <= 64; sig++)
            {
                if (before[sig] == null) continue;
                if (mode == "restore-rtmin" && sig != 34) continue;
                byte[] a = Read(sig);
                if (a == null) continue;
                if (HandlerOf(a) != HandlerOf(before[sig])) continue;   // Go REPLACED it; leave it
                if (FlagsOf(a) == FlagsOf(before[sig]) && MaskEquals(before[sig], a)) continue;
                if (sigaction(sig, before[sig], null) == 0) { restored++; touched.Add(Name(sig)); }
            }
            Console.Error.WriteLine("[hook] restored " + restored + " relocated handler(s): "
                                    + (touched.Count == 0 ? "-" : string.Join(",", touched)));
            return;
        }

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
                BitConverter.GetBytes(f & ~SA_ONSTACK).CopyTo(a, FLAGS_OFF);
                if (sigaction(sig, a, null) == 0) cleared++;
            }
            Console.Error.WriteLine("[hook] cleared SA_ONSTACK from " + cleared + " signal(s)");
            return;
        }

        for (int i = 0; i < three.Length; i++)
        {
            if (mode == "restore")
            {
                if (before[three[i]] != null) sigaction(three[i], before[three[i]], null);
            }
            else if (mode == "noonstack")
            {
                byte[] cur = Read(three[i]);
                if (cur == null) continue;
                BitConverter.GetBytes(FlagsOf(cur) & ~SA_ONSTACK).CopyTo(cur, FLAGS_OFF);
                sigaction(three[i], cur, null);
            }
        }
        Console.Error.WriteLine("[hook] applied " + mode);
    }
}
