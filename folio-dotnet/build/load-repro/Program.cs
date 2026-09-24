using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Threading;

// DW-398, the smallest process that can die of it -- if it can.
//
// Everything measured so far has died inside `dotnet test`: a vstest host with
// its own threads, its own AppDomain setup and its own way of reporting a dead
// child. That makes vstest a confounder nobody has removed, and it makes every
// iteration cost seconds. This is the same exposure with nothing else in it.
//
//   repro <libfolio8_native.so> [--load|--noload] [--pool N] [--hold MS]
//
// --noload is the control: identical process, identical thread-pool work, no
// engine. load-exposure.sh established that merely loading the library is
// enough under vstest (134/500 against 0/500); if that holds here the
// instrument gets ~100x cheaper, and if it does NOT hold, vstest is part of
// the trigger and that is a finding in its own right.
//
// WHY THE THREAD POOL. The one real backtrace (run 35945107391) died on a
// `.NET TP Worker` in pthread_cond_wait. Pool workers park on a condvar, which
// is the state the crash implicates, so the pool has to be awake and parking
// for the window to exist at all. --pool 0 turns that off to test whether it
// is required.
//
// Exit status IS the result: 0 clean, 134 SIGABRT (128+6), 139 SIGSEGV
// (128+11). The runner reads those rather than scraping a message, because a
// self-abort and a segfault are different defects and this entry has already
// conflated two once.
internal static class Program
{
    private static int Main(string[] argv)
    {
        string so = null;
        bool load = true;
        int pool = 8, holdMs = 300;

        for (int i = 0; i < argv.Length; i++)
        {
            switch (argv[i])
            {
                case "--load":   load = true; break;
                case "--noload": load = false; break;
                case "--pool":   pool = int.Parse(argv[++i]); break;
                case "--hold":   holdMs = int.Parse(argv[++i]); break;
                default:         so = argv[i]; break;
            }
        }
        if (load && string.IsNullOrEmpty(so))
        {
            Console.Error.WriteLine("usage: repro <libfolio8_native.so> [--load|--noload] [--pool N] [--hold MS]");
            return 2;
        }

        // Get the pool warm and parking BEFORE the load, so the load lands on
        // threads already sitting in the condvar rather than on a cold pool.
        var done = new CountdownEvent(Math.Max(pool, 1));
        for (int i = 0; i < pool; i++)
            ThreadPool.QueueUserWorkItem(_ => { Thread.Sleep(5); done.Signal(); });
        if (pool == 0) done.Signal();
        done.Wait(2000);

        if (load)
        {
            IntPtr h = NativeLibrary.Load(so);
            if (h == IntPtr.Zero) { Console.Error.WriteLine("load returned null"); return 3; }
        }

        // Keep the pool cycling across the window. The crash is not at the
        // instant of dlopen -- load-exposure's arm B returned from the load
        // and died later -- so the process has to stay alive and scheduling.
        var sw = Stopwatch.StartNew();
        while (sw.ElapsedMilliseconds < holdMs)
        {
            var round = new CountdownEvent(Math.Max(pool, 1));
            for (int i = 0; i < pool; i++)
                ThreadPool.QueueUserWorkItem(_ => { Thread.Yield(); round.Signal(); });
            if (pool == 0) round.Signal();
            round.Wait(1000);
        }

        Console.Out.Flush();
        return 0;
    }
}
