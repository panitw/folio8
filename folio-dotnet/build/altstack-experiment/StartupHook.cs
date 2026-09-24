using System;
using System.Runtime.InteropServices;

// Loaded via DOTNET_STARTUP_HOOKS, so the experiment touches no repo code.
// FOLIO_EXP selects the workaround:
//   none      -- baseline
//   noonstack -- clear SA_ONSTACK on Go's synchronous handlers after load
//   restore   -- put the CLR's own handlers back after load
internal sealed class StartupHook
{
    private const int SIGSEGV = 11, SIGBUS = 7, SIGURG = 23;
    private const int SA_ONSTACK = 0x08000000;

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaction(int signum, byte[] act, byte[] oldact);

    private static byte[] Read(int sig)
    {
        var b = new byte[256];
        return sigaction(sig, null, b) == 0 ? b : null;
    }

    private static void Write(int sig, byte[] act)
    {
        int rc = sigaction(sig, act, null);
        Console.Error.WriteLine("[hook] sigaction(" + sig + ") -> " + rc);
    }

    public static void Initialize()
    {
        string mode = Environment.GetEnvironmentVariable("FOLIO_EXP") ?? "none";
        string so = Environment.GetEnvironmentVariable("FOLIO_SO");
        if (string.IsNullOrEmpty(so)) { Console.Error.WriteLine("[hook] FOLIO_SO unset"); return; }

        var before = new byte[3][];
        int[] sigs = { SIGSEGV, SIGBUS, SIGURG };
        for (int i = 0; i < sigs.Length; i++) before[i] = Read(sigs[i]);

        NativeLibrary.Load(so);   // Go installs its handlers here

        if (mode == "none") { Console.Error.WriteLine("[hook] baseline, no change"); return; }

        for (int i = 0; i < sigs.Length; i++)
        {
            if (mode == "restore")
            {
                if (before[i] != null) Write(sigs[i], before[i]);
            }
            else if (mode == "noonstack")
            {
                var cur = Read(sigs[i]);
                if (cur == null) continue;
                int flags = BitConverter.ToInt32(cur, 136);
                BitConverter.GetBytes(flags & ~SA_ONSTACK).CopyTo(cur, 136);
                Write(sigs[i], cur);
            }
        }
        Console.Error.WriteLine("[hook] applied " + mode);
    }
}
