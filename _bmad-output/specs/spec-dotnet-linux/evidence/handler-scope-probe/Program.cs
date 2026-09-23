using System;
using System.Collections.Generic;
using System.IO;
using System.Runtime.InteropServices;
using System.Threading;

// Does Go's c-shared library install its SA_ONSTACK handlers PROCESS-WIDE?
// If it does, a signal on ANY thread -- including a .NET TP Worker that never
// crosses the ABI -- runs a Go handler on that thread's alternate signal
// stack, and enlarging only binding-owned threads cannot be sufficient.
// Read the state; do not wait for a crash.
internal static class Program
{
    [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
    private delegate int AllocCount();

    private const int SIGSEGV = 11, SIGBUS = 7, SIGURG = 23, SIGPROF = 27, SIGABRT = 6;
    private const int SA_ONSTACK = 0x08000000;
    private const int SA_SIGINFO = 0x00000004;

    // glibc x86-64: handler @0, sigset_t sa_mask @8 (128 bytes), int sa_flags @136.
    [DllImport("libc", SetLastError = true)]
    private static extern int sigaction(int signum, IntPtr act, byte[] oldact);

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaltstack(IntPtr ss, byte[] oldss);

    private static (IntPtr handler, int flags) ReadHandler(int sig)
    {
        var buf = new byte[256];
        if (sigaction(sig, IntPtr.Zero, buf) != 0) return (IntPtr.Zero, -1);
        return ((IntPtr)BitConverter.ToInt64(buf, 0), BitConverter.ToInt32(buf, 136));
    }

    private static long ReadAltStackSize()
    {
        var buf = new byte[64];
        if (sigaltstack(IntPtr.Zero, buf) != 0) return -1;
        // Linux stack_t: void* ss_sp; int ss_flags; <pad>; size_t ss_size;
        return BitConverter.ToInt64(buf, 16);
    }

    private static long ReadAltStackBase()
    {
        var buf = new byte[64];
        if (sigaltstack(IntPtr.Zero, buf) != 0) return -1;
        return BitConverter.ToInt64(buf, 0);
    }

    private sealed class Region { public ulong Lo, Hi; public string Path = ""; }

    private static List<Region> Maps()
    {
        var list = new List<Region>();
        foreach (var line in File.ReadAllLines("/proc/self/maps"))
        {
            int dash = line.IndexOf('-');
            if (dash <= 0) continue;
            int sp = line.IndexOf(' ');
            if (sp <= dash) continue;
            try
            {
                ulong lo = Convert.ToUInt64(line.Substring(0, dash), 16);
                ulong hi = Convert.ToUInt64(line.Substring(dash + 1, sp - dash - 1), 16);
                string path = "";
                int slash = line.IndexOf('/');
                if (slash > 0) path = line.Substring(slash).Trim();
                list.Add(new Region { Lo = lo, Hi = hi, Path = path });
            }
            catch { }
        }
        return list;
    }

    private static string Attribute(IntPtr addr)
    {
        if (addr == IntPtr.Zero) return "SIG_DFL (0)";
        if ((long)addr == 1) return "SIG_IGN (1)";
        ulong a = (ulong)(long)addr;
        foreach (var r in Maps())
            if (a >= r.Lo && a < r.Hi)
                return (r.Path.Length == 0 ? "[anonymous]" : r.Path) + "  (+0x" + (a - r.Lo).ToString("x") + ")";
        return "UNMAPPED";
    }

    private static void Dump(string when, int[] sigs, string[] names)
    {
        Console.WriteLine("--- SIGNAL HANDLERS " + when + " ---");
        for (int i = 0; i < sigs.Length; i++)
        {
            var (h, f) = ReadHandler(sigs[i]);
            if (f < 0) { Console.WriteLine(string.Format("  {0,-8} <sigaction failed>", names[i])); continue; }
            Console.WriteLine(string.Format("  {0,-8} handler=0x{1:X}  ONSTACK={2,-3} SIGINFO={3,-3}  {4}",
                names[i], (long)h, ((f & SA_ONSTACK) != 0) ? "yes" : "NO",
                ((f & SA_SIGINFO) != 0) ? "yes" : "NO", Attribute(h)));
        }
        Console.WriteLine();
    }

    public static int Main(string[] args)
    {
        string soPath = args.Length > 0 ? args[0] : "libfolio8_native.so";
        int[] sigs = { SIGSEGV, SIGBUS, SIGURG, SIGPROF, SIGABRT };
        string[] names = { "SIGSEGV", "SIGBUS", "SIGURG", "SIGPROF", "SIGABRT" };

        Console.WriteLine("pid " + Environment.ProcessId + ", ProcessorCount " + Environment.ProcessorCount);
        Console.WriteLine("main-thread altstack: base=0x" + ReadAltStackBase().ToString("X") + " size=" + ReadAltStackSize());
        Console.WriteLine();

        Dump("BEFORE loading the Go c-shared library", sigs, names);

        IntPtr lib = NativeLibrary.Load(soPath);
        Console.WriteLine("loaded " + soPath + " -> 0x" + ((long)lib).ToString("X") + Environment.NewLine);
        Dump("AFTER dlopen (Go runtime init)", sigs, names);

        // One real crossing, so any lazily-installed handler is in place too.
        IntPtr fn = NativeLibrary.GetExport(lib, "folio8_allocation_count");
        var count = (AllocCount)Marshal.GetDelegateForFunctionPointer(fn, typeof(AllocCount));
        Console.WriteLine("folio8_allocation_count() = " + count() + Environment.NewLine);
        Dump("AFTER one crossing", sigs, names);

        Console.WriteLine("--- ALTSTACK, PER THREAD KIND, AFTER THE ENGINE IS LIVE ---");
        Console.WriteLine(string.Format("  {0,-34} base=0x{1:X} size={2}", "main thread", ReadAltStackBase(), ReadAltStackSize()));

        var done = new ManualResetEventSlim(false);
        long tpBase = 0, tpSize = 0;
        ThreadPool.QueueUserWorkItem(_ => { tpBase = ReadAltStackBase(); tpSize = ReadAltStackSize(); done.Set(); });
        done.Wait();
        Console.WriteLine(string.Format("  {0,-34} base=0x{1:X} size={2}", ".NET TP Worker", tpBase, tpSize));

        long ntBase = 0, ntSize = 0;
        var t = new Thread(() => { ntBase = ReadAltStackBase(); ntSize = ReadAltStackSize(); });
        t.Start(); t.Join();
        Console.WriteLine(string.Format("  {0,-34} base=0x{1:X} size={2}", "new Thread()", ntBase, ntSize));
        Console.WriteLine();

        Console.WriteLine("Go sizes its own gsignal stack at 32768 bytes.");
        return 0;
    }
}
