// DW-396 spike. The question: on Linux, does a thread the BINDING creates get
// a different (larger) alternate signal stack than a CLR thread-pool thread?
// If they are the same, "create our own thread" fixes nothing on its own.
//
// Measured by reading sigaltstack(NULL, &old) on each thread kind, rather than
// by waiting for a nondeterministic overflow.

using System;
using System.Runtime.InteropServices;
using System.Threading;

internal static class Program
{
    // stack_t on 64-bit glibc: void *ss_sp; int ss_flags; (4 bytes padding); size_t ss_size;
    [StructLayout(LayoutKind.Sequential)]
    private struct StackT
    {
        public IntPtr ss_sp;
        public int ss_flags;
        private int _pad;
        public IntPtr ss_size;
    }

    private const int SS_DISABLE = 2;
    private const int SS_ONSTACK = 1;

    // glibc confname.h, added in 2.34.
    private const int _SC_MINSIGSTKSZ = 249;
    private const int _SC_SIGSTKSZ = 250;

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaltstack(IntPtr ss, ref StackT old);

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaltstack(ref StackT ss, IntPtr old);

    [DllImport("libc", SetLastError = true)]
    private static extern long sysconf(int name);

    [DllImport("libc")]
    private static extern IntPtr pthread_self();

    [DllImport("libc", SetLastError = true)]
    private static extern int pthread_create(out IntPtr thread, IntPtr attr, IntPtr start, IntPtr arg);

    [DllImport("libc", SetLastError = true)]
    private static extern int pthread_join(IntPtr thread, IntPtr retval);

    private static string Report(string label)
    {
        StackT cur = default;
        int rc = sigaltstack(IntPtr.Zero, ref cur);
        if (rc != 0)
        {
            return $"{label,-34} sigaltstack() failed, errno {Marshal.GetLastWin32Error()}";
        }
        long size = cur.ss_size.ToInt64();
        string flags = cur.ss_flags == 0 ? "0"
            : ((cur.ss_flags & SS_DISABLE) != 0 ? "SS_DISABLE" : "")
              + ((cur.ss_flags & SS_ONSTACK) != 0 ? "SS_ONSTACK" : "");
        string installed = (cur.ss_flags & SS_DISABLE) != 0 || cur.ss_sp == IntPtr.Zero
            ? "NONE INSTALLED"
            : $"sp=0x{cur.ss_sp.ToInt64():X} size={size} ({size / 1024.0:0.#} KiB)";
        return $"{label,-34} flags={flags,-10} {installed}";
    }

    [UnmanagedCallersOnly]
    private static IntPtr ForeignThread(IntPtr arg)
    {
        // A thread the CLR did NOT create. The first managed frame here is this
        // one, so this reading is "after the runtime has attached it".
        Console.WriteLine(Report("raw pthread (attached)"));
        return IntPtr.Zero;
    }

    private static unsafe void Main()
    {
        Console.WriteLine($"RuntimeInformation: {RuntimeInformation.OSDescription} / {RuntimeInformation.ProcessArchitecture}");
        Console.WriteLine($".NET             : {RuntimeInformation.FrameworkDescription}");
        long sc = sysconf(_SC_SIGSTKSZ);
        long mn = sysconf(_SC_MINSIGSTKSZ);
        Console.WriteLine($"sysconf(_SC_SIGSTKSZ)    = {sc}   ({(sc > 0 ? (sc / 1024.0).ToString("0.#") + " KiB" : "unavailable, pre-2.34 glibc")})");
        Console.WriteLine($"sysconf(_SC_MINSIGSTKSZ) = {mn}");
        Console.WriteLine($"Go's gsignal stack is 32768 (32 KiB) — the room its SA_ONSTACK handlers expect.");
        Console.WriteLine();

        Console.WriteLine(Report("main thread"));

        var done = new ManualResetEventSlim(false);
        ThreadPool.QueueUserWorkItem(_ =>
        {
            Console.WriteLine(Report(".NET TP Worker"));
            done.Set();
        });
        done.Wait();

        var t = new Thread(() => Console.WriteLine(Report("new Thread() default stack")));
        t.Start();
        t.Join();

        var tBig = new Thread(() => Console.WriteLine(Report("new Thread() 16 MiB stack")), 16 * 1024 * 1024);
        tBig.Start();
        tBig.Join();

        // A long-lived foreground thread, in case the runtime treats a
        // short-lived one differently.
        var tLive = new Thread(() =>
        {
            Thread.Sleep(50);
            Console.WriteLine(Report("new Thread() after 50ms"));
        });
        tLive.Start();
        tLive.Join();

        IntPtr tid;
        int rc = pthread_create(out tid, IntPtr.Zero, (IntPtr)(delegate* unmanaged<IntPtr, IntPtr>)&ForeignThread, IntPtr.Zero);
        if (rc == 0) { pthread_join(tid, IntPtr.Zero); }
        else { Console.WriteLine($"pthread_create failed: {rc}"); }

        Console.WriteLine();
        Console.WriteLine("--- can we enlarge it ourselves, and does it stick? ---");
        var tFix = new Thread(() =>
        {
            Console.WriteLine(Report("  before"));
            const int want = 1024 * 1024;
            IntPtr mem = Marshal.AllocHGlobal(want);
            StackT ss = new StackT { ss_sp = mem, ss_flags = 0, ss_size = (IntPtr)want };
            int r = sigaltstack(ref ss, IntPtr.Zero);
            Console.WriteLine($"  sigaltstack(set {want / 1024} KiB) -> {r}" + (r != 0 ? $" errno {Marshal.GetLastWin32Error()}" : ""));
            Console.WriteLine(Report("  after"));
            // Prove the runtime still works on this thread afterwards.
            try
            {
                object o = null;
                try { _ = o.ToString(); } catch (NullReferenceException) { Console.WriteLine("  NullReferenceException still caught normally after the swap."); }
                GC.Collect(2, GCCollectionMode.Forced, true);
                Console.WriteLine("  Full blocking GC completed on the swapped thread.");
            }
            catch (Exception e) { Console.WriteLine($"  runtime unhappy after swap: {e.GetType().Name}: {e.Message}"); }
        });
        tFix.Start();
        tFix.Join();
    }
}
