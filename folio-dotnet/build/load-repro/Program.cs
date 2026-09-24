using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Threading;

// DW-398, the smallest process that can die of it -- and it can: run
// 35987979509, 177/300 SIGSEGV with the engine loaded, 0/300 without.
//
//   repro <libfolio8_native.so> [--load|--noload] [--pool N] [--hold MS]
//         [--gc|--nogc] [--fix none|restore-go|restore-relocated|restore-all|noonstack]
//
// THE WINDOW. Every core on record is a `.NET TP Worker` interrupted BY A
// SIGNAL while executing JIT'd managed code, with the CLR's own handler then
// dying under `<signal handler called>`. The CLR handler that interrupts
// managed code on pool threads is GC suspension: SuspendRuntime sends
// SIGRTMIN to every thread in managed code and the handler parks the thread
// until the GC is done. So: workers allocating in managed code, forced GCs
// through the hold, and the process leaving while that is going on. The
// first version parked idle threads and allocated nothing; it never GC'd and
// measured 0/300 on both arms. --nogc keeps that workload for comparison.
//
// --fix IS THE BISECTION. Loading the engine changes exactly two things in
// this process that any experiment has measured -- Go's own handlers on
// SEGV/BUS/FPE/PIPE/URG, and SA_ONSTACK added to the CLR's on
// ILL/TRAP/ABRT/TERM/RTMIN -- and every earlier attempt to undo either was
// made under vstest, where the rate swings 42-69/150 between runs and the
// hook's own output was swallowed, so nothing about those arms was ever
// verified. Here the baseline is 177/300 against a clean control, a fix that
// works reads as ZERO, and each fix prints what it touched to stderr, which
// the runner records per arm.
//
//   restore-go         put the CLR's original handler back where Go REPLACED one
//   restore-relocated  put the CLR's original struct back where Go only re-flagged one
//   restore-all        both
//   noonstack          clear SA_ONSTACK from every handled signal, leave handlers
//
// Exit status IS the result: 0 clean, 134 SIGABRT, 139 SIGSEGV.
internal static class Program
{
    private const int SA_ONSTACK = 0x08000000;
    private const int FLAGS_OFF  = 136;   // glibc x86_64 struct sigaction: handler 0, sa_mask 8, sa_flags 136

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaction(int signum, byte[] act, byte[] oldact);

    private static byte[] Read(int sig) { var b = new byte[256]; return sigaction(sig, null, b) == 0 ? b : null; }
    private static long HandlerOf(byte[] a) { return BitConverter.ToInt64(a, 0); }
    private static int  FlagsOf(byte[] a)   { return BitConverter.ToInt32(a, FLAGS_OFF); }
    private static bool Touchable(int sig)  { return sig != 9 && sig != 19 && sig != 32 && sig != 33 && sig >= 1 && sig <= 64; }

    private static string Name(int sig)
    {
        switch (sig)
        {
            case 4: return "ILL"; case 5: return "TRAP"; case 6: return "ABRT"; case 7: return "BUS";
            case 8: return "FPE"; case 11: return "SEGV"; case 13: return "PIPE"; case 15: return "TERM";
            case 23: return "URG"; case 34: return "RTMIN"; default: return sig.ToString();
        }
    }

    // Applies one fix after the load and returns the signals it changed, so
    // the runner can print per arm what was actually done -- the thing the
    // vstest experiments never had.
    private static string ApplyFix(string fix, byte[][] before)
    {
        var touched = new List<string>();
        for (int sig = 1; sig <= 64; sig++)
        {
            if (before[sig] == null) continue;
            byte[] now = Read(sig);
            if (now == null) continue;
            bool replaced  = HandlerOf(now) != HandlerOf(before[sig]);
            bool reflagged = !replaced && FlagsOf(now) != FlagsOf(before[sig]);
            bool ok = false;
            switch (fix)
            {
                case "restore-go":        if (replaced)               ok = sigaction(sig, before[sig], null) == 0; break;
                case "restore-relocated": if (reflagged)              ok = sigaction(sig, before[sig], null) == 0; break;
                case "restore-all":       if (replaced || reflagged)  ok = sigaction(sig, before[sig], null) == 0; break;
                case "noonstack":
                {
                    long h = HandlerOf(now); int f = FlagsOf(now);
                    if (h != 0 && h != 1 && (f & SA_ONSTACK) != 0)
                    {
                        BitConverter.GetBytes(f & ~SA_ONSTACK).CopyTo(now, FLAGS_OFF);
                        ok = sigaction(sig, now, null) == 0;
                    }
                    break;
                }
            }
            if (ok) touched.Add(Name(sig));
        }
        return touched.Count == 0 ? "-" : string.Join(",", touched);
    }

    private static volatile int s_sink;

    private static int Main(string[] argv)
    {
        string so = null, fix = "none";
        bool load = true, gc = true;
        int pool = 8, holdMs = 400;

        for (int i = 0; i < argv.Length; i++)
        {
            switch (argv[i])
            {
                case "--load":   load = true; break;
                case "--noload": load = false; break;
                case "--gc":     gc = true; break;
                case "--nogc":   gc = false; break;
                case "--pool":   pool = int.Parse(argv[++i]); break;
                case "--hold":   holdMs = int.Parse(argv[++i]); break;
                case "--fix":    fix = argv[++i]; break;
                default:         so = argv[i]; break;
            }
        }
        if (load && string.IsNullOrEmpty(so))
        {
            Console.Error.WriteLine("usage: repro <libfolio8_native.so> [--load|--noload] [--pool N] [--hold MS] [--gc|--nogc] [--fix MODE]");
            return 2;
        }
        switch (fix)
        {
            case "none": case "restore-go": case "restore-relocated": case "restore-all": case "noonstack": break;
            default: Console.Error.WriteLine("unknown --fix " + fix); return 2;
        }

        ThreadPool.SetMinThreads(Math.Max(pool, 1), 1);

        // Workers that stay IN managed code: allocate, compute, yield, repeat,
        // until told to stop -- which they never are.
        var stop = new ManualResetEventSlim(false);
        var started = new CountdownEvent(Math.Max(pool, 1));
        for (int i = 0; i < pool; i++)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                started.Signal();
                var rnd = new Random(Environment.CurrentManagedThreadId);
                while (!stop.IsSet)
                {
                    var a = new byte[rnd.Next(16, 4096)];
                    var o = new object[rnd.Next(1, 64)];
                    for (int k = 0; k < o.Length; k++) o[k] = new int[4];
                    s_sink += a.Length + o.Length;
                    if ((s_sink & 0xff) == 0) Thread.Yield();
                }
            });
        }
        if (pool == 0) started.Signal();
        started.Wait(2000);

        // Every handler's struct before the load, so a fix can put back the
        // CLR's own rather than a guess at it.
        var before = new byte[65][];
        if (RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
            for (int sig = 1; sig <= 64; sig++) if (Touchable(sig)) before[sig] = Read(sig);

        if (load)
        {
            IntPtr h = NativeLibrary.Load(so);
            if (h == IntPtr.Zero) { Console.Error.WriteLine("load returned null"); return 3; }
        }

        string touched = fix == "none" ? "-" : ApplyFix(fix, before);
        Console.Error.WriteLine("[repro] load=" + (load ? "yes" : "no") + " gc=" + (gc ? "yes" : "no")
                                + " fix=" + fix + " touched=" + touched);

        // The window: forced collections for the whole hold, each one
        // suspending every worker in managed code by signal.
        var sw = Stopwatch.StartNew();
        while (sw.ElapsedMilliseconds < holdMs)
        {
            if (gc) GC.Collect(2, GCCollectionMode.Forced, blocking: true);
            else Thread.Sleep(1);
        }

        // Leave with the workers still running: every death under vstest
        // printed `Passed!` first, so the process dies on the way out.
        Console.Out.Flush();
        return 0;
    }
}
