using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Threading;

// DW-398, the smallest process that can die of it -- if it can.
//
//   repro <libfolio8_native.so> [--load|--noload] [--pool N] [--hold MS] [--gc|--nogc]
//
// WHAT THE TWO CORES SAY THE WINDOW IS. Both cores on record (runs
// 35945107391 and 35984125246) are the same shape: a `.NET TP Worker`
// interrupted BY A SIGNAL while executing JIT'd managed code, with the CLR's
// own handler then dying under `<signal handler called>` -- once by aborting
// in pthread_cond_wait, once by segfaulting. The CLR handler that interrupts
// managed code on a pool thread and then waits on a condvar is the GC
// suspension path: SuspendRuntime sends SIGRTMIN to every thread in managed
// code and the handler parks the thread until the GC is done.
//
// So the window needs three things this program's first version had none of:
// pool threads IN MANAGED CODE (not parked -- parked threads are not signalled),
// GCs actually happening, and the process leaving while that is going on.
// The first version parked eight threads for 300ms and allocated nothing, so
// no GC ever ran, no activation was ever sent, and its 0/300 measured an
// empty room. --nogc reproduces that arm so the comparison is on record.
//
// --noload is the control: identical process, identical work, no engine.
//
// Exit status IS the result: 0 clean, 134 SIGABRT, 139 SIGSEGV. Both have
// now been seen for this one defect, and the runner counts them apart.
internal static class Program
{
    private static volatile int s_sink;

    private static int Main(string[] argv)
    {
        string so = null;
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
                default:         so = argv[i]; break;
            }
        }
        if (load && string.IsNullOrEmpty(so))
        {
            Console.Error.WriteLine("usage: repro <libfolio8_native.so> [--load|--noload] [--pool N] [--hold MS] [--gc|--nogc]");
            return 2;
        }

        ThreadPool.SetMinThreads(Math.Max(pool, 1), 1);

        // Workers that stay IN managed code: allocate, compute, yield, repeat,
        // until told to stop. A thread that is signalled mid-allocation is
        // exactly the thread the activation handler has to redirect.
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

        if (load)
        {
            IntPtr h = NativeLibrary.Load(so);
            if (h == IntPtr.Zero) { Console.Error.WriteLine("load returned null"); return 3; }
        }

        // The window. With --gc the main thread forces collections for the
        // whole hold, each one suspending every worker that is in managed
        // code by signal. Without it the workers just run.
        var sw = Stopwatch.StartNew();
        while (sw.ElapsedMilliseconds < holdMs)
        {
            if (gc) GC.Collect(2, GCCollectionMode.Forced, blocking: true);
            else Thread.Sleep(1);
        }

        // Leave with the workers still running. Every death under vstest
        // printed `Passed!` first: the process dies on the way OUT, and a
        // reproducer that tidies up before exiting removes that from the
        // window. `stop` is deliberately never set.
        Console.Out.Flush();
        return 0;
    }
}
