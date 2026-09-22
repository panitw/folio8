// DW-396's mechanism, made re-runnable. The question the spike had to settle
// was whether a thread the BINDING creates gets a larger alternate signal
// stack than a CLR thread-pool thread. It does not: the CLR installs a
// FIXED-SIZE altstack on every thread it touches — 16 KiB on amd64, 24 KiB on
// arm64 — while Go sizes its own gsignal stack at 32 KiB and, under cgo,
// ADOPTS whatever it finds instead of installing that. Every Go SA_ONSTACK
// handler entered from .NET therefore runs in less room than Go expects, and
// the kernel turns the overflow into SIGSEGV.
//
// THIS PROBE ASSERTS NOTHING ABOUT WHAT THE NUMBERS SHOULD BE. A diagnostic
// that failed a build would turn "the CLR's altstack is 16 KiB here" into
// policy; what a reader needs is the number and its provenance. (It does exit
// non-zero when a reading it attempted could not be taken — that is a report
// about the run, not a verdict on the measurement.) Story 2 of
// SPEC-dotnet-linux owns the assertion, and declares its own sigaltstack
// P/Invoke inside the package's netstandard2.0 floor rather than sharing code
// with this tool.
//
// It reads sigaltstack(NULL, &old) on each thread kind rather than waiting
// for a nondeterministic overflow: a deterministic reading in seconds instead
// of a soak.
//
// EVERY ROW CARRIES ITS OWN PROVENANCE, AND THE PROBE DETERMINES IT RATHER
// THAN TRUSTING THE OPERATOR. DW-396 was burned twice by reading a tally off
// a host that was not what it looked like — once under qemu, once under
// Rosetta — so a translated row must never print identically to a
// real-silicon one. The probe reports the kernel's architecture against its
// own, plus the Rosetta and binfmt_misc markers, and tags each row with the
// verdict it derived.
//
// It also reports sysconf(_SC_SIGSTKSZ) beside the CLR's installed size,
// because the finding is precisely that the two disagree: glibc 2.34 turned
// SIGSTKSZ into a runtime value that tracks the CPU's register save area,
// and the CLR's altstack is a constant that tracks nothing.

using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

internal static class Program
{
    // stack_t on 64-bit glibc: void *ss_sp; int ss_flags; (4 bytes padding);
    // size_t ss_size. The padding is explicit rather than left to the
    // marshaller so the layout is readable against the C declaration.
    [StructLayout(LayoutKind.Sequential)]
    internal struct StackT
    {
        public IntPtr ss_sp;
        public int ss_flags;
        private int _pad;
        public IntPtr ss_size;
    }

    private const int SS_ONSTACK = 1;
    private const int SS_DISABLE = 2;

    // glibc confname.h. Both were ADDED IN 2.34; before that sysconf returns
    // -1 for them, which is a missing answer and not a size.
    private const int _SC_MINSIGSTKSZ = 249;
    private const int _SC_SIGSTKSZ = 250;

    // _UTSNAME_LENGTH. 65 on glibc and on musl alike; the buffer passed to
    // uname() is far larger than the six fields need, so a longer field on
    // some future libc overruns into slack rather than into memory we do not
    // own.
    private const int UtsFieldLength = 65;
    private const int UtsMachineField = 4;

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaltstack(IntPtr ss, ref StackT old);

    [DllImport("libc", SetLastError = true)]
    private static extern int sigaltstack(ref StackT ss, IntPtr old);

    [DllImport("libc", SetLastError = true)]
    private static extern long sysconf(int name);

    [DllImport("libc", SetLastError = true)]
    private static extern int uname(byte[] buf);

    // pthread_create IS NOT UNIFORMLY RESOLVABLE FROM libc. It lived in
    // libpthread until glibc's 2.34 merge, so on Ubuntu 20.04 (glibc 2.31)
    // this entry point does not exist and the call throws
    // EntryPointNotFoundException. That is a skipped row with a named reason,
    // not a failed run — and it is the same fact that makes Story 2's own
    // libc P/Invokes worth checking across the floor.
    [DllImport("libc", SetLastError = true)]
    private static extern int pthread_create(out IntPtr thread, IntPtr attr, IntPtr start, IntPtr arg);

    [DllImport("libc", SetLastError = true)]
    private static extern int pthread_join(IntPtr thread, IntPtr retval);

    // ---------------------------------------------------------------- rows

    // Reading is one sigaltstack() call's worth of answer: either an errno or
    // a stack_t. It exists so the row FORMATTING is a pure function of a
    // value, which is what --self-check drives with synthetic cases the
    // matrix's defensive branches cannot be provoked into on a real host.
    internal readonly struct Reading
    {
        public readonly bool Failed;
        public readonly int Errno;
        public readonly long Sp;
        public readonly int Flags;
        public readonly long Size;

        private Reading(bool failed, int errno, long sp, int flags, long size)
        {
            Failed = failed;
            Errno = errno;
            Sp = sp;
            Flags = flags;
            Size = size;
        }

        public static Reading Ok(long sp, int flags, long size)
        {
            return new Reading(false, 0, sp, flags, size);
        }

        public static Reading Failure(int errno)
        {
            return new Reading(true, errno, 0, 0, 0);
        }
    }

    internal static string FormatFlags(int flags)
    {
        if (flags == 0)
        {
            return "0";
        }

        var parts = new List<string>();
        if ((flags & SS_ONSTACK) != 0)
        {
            parts.Add("SS_ONSTACK");
        }

        if ((flags & SS_DISABLE) != 0)
        {
            parts.Add("SS_DISABLE");
        }

        int rest = flags & ~(SS_ONSTACK | SS_DISABLE);
        if (rest != 0)
        {
            parts.Add("0x" + rest.ToString("X", CultureInfo.InvariantCulture));
        }

        return string.Join("|", parts);
    }

    // FormatRow renders one measurement. NO ALTSTACK AT ALL IS A DISTINCT
    // ANSWER, not a size: SS_DISABLE, or a null ss_sp, means the kernel would
    // deliver an SA_ONSTACK handler on the ordinary stack, and printing the
    // stale ss_size beside that would be a measurement of nothing.
    internal static string FormatRow(string provenance, string label, Reading r)
    {
        string head = string.Format(
            CultureInfo.InvariantCulture, "[{0,-7}] {1,-34} ", provenance, label);

        if (r.Failed)
        {
            return head + string.Format(
                CultureInfo.InvariantCulture,
                "sigaltstack() failed, errno {0}", r.Errno);
        }

        string flags = string.Format(
            CultureInfo.InvariantCulture, "flags={0,-21} ", FormatFlags(r.Flags));

        if ((r.Flags & SS_DISABLE) != 0 || r.Sp == 0)
        {
            return head + flags + "NONE INSTALLED";
        }

        return head + flags + string.Format(
            CultureInfo.InvariantCulture,
            "sp=0x{0:X} size={1} ({2} KiB)",
            r.Sp, r.Size, (r.Size / 1024.0).ToString("0.#", CultureInfo.InvariantCulture));
    }

    // FormatSkip renders a row that could NOT be measured. It is a separate
    // rendering from a failed sigaltstack() — nothing was read at all — and it
    // must still carry the label, the provenance tag and a NAMED REASON, so a
    // reader can tell "this host cannot do that" from "this host said nothing".
    //
    // PURE, AND FOR THE SAME REASON FormatRow IS. The reason this row exists
    // is glibc < 2.34, where pthread_create lived in libpthread and the
    // P/Invoke throws EntryPointNotFoundException — and no SDK image this
    // probe runs on carries a glibc that old, so the only way to exercise the
    // degradation is to drive its rendering directly.
    internal static string FormatSkip(string provenance, string label, string reason)
    {
        return string.Format(
            CultureInfo.InvariantCulture,
            "[{0,-7}] {1,-34} SKIPPED — {2}", provenance, label, reason);
    }

    // The reason the pre-2.34 floor produces, written once so the row and its
    // self-check case cannot drift apart.
    internal const string PthreadRowLabel = "raw pthread (attached)";

    internal const string PthreadNotInLibc =
        "pthread_create is not in libc here (glibc < 2.34 keeps it in libpthread)";

    // ----------------------------------------------------------- provenance

    // Provenance is a VERDICT plus the one-line summary that justifies it.
    // Verdict derivation is the one piece of judgement in this tool, and the
    // one piece with a demonstrated defect history: its first cut called a
    // Rosetta container NATIVE. So the READING of each signal stays impure
    // (four small functions that touch uname, /proc/cpuinfo, /run and
    // binfmt_misc) and the DERIVATION over those signals is a pure function
    // of a value, driven by --self-check like every other rendering here.
    internal sealed class Signals
    {
        public string Machine;              // null: uname reported nothing
        public Architecture Self;
        public bool CpuinfoReadable;        // false: /proc/cpuinfo could not be read AT ALL
        public string Vendor;               // null: no vendor_id line (x86 shape absent)
        public string Implementer;          // null: no CPU implementer line (arm shape absent)
        public bool RosettaMarker;
        public List<string> BinfmtHits = new List<string>();
    }

    internal sealed class Provenance
    {
        public string Tag;                  // what every row is stamped with
        public string Summary;              // one line: the verdict and why
        public bool Translated;
        public List<string> Signals = new List<string>();

        // What the report prints once, under PROVENANCE.
        public string Verdict
        {
            get
            {
                return Translated
                    ? Summary
                      + Environment.NewLine
                      + "                       ⚠ NOT admissible as evidence about real silicon —"
                      + " sysconf here is the translator's answer, not the CPU's."
                    : Summary;
            }
        }
    }

    private static string KernelMachine()
    {
        try
        {
            var buf = new byte[UtsFieldLength * 8];
            if (uname(buf) != 0)
            {
                return null;
            }

            int start = UtsFieldLength * UtsMachineField;
            int end = start;
            while (end < buf.Length && buf[end] != 0)
            {
                end++;
            }

            string machine = Encoding.UTF8.GetString(buf, start, end - start).Trim();
            return machine.Length == 0 ? null : machine;
        }
        catch (Exception e) when (e is EntryPointNotFoundException || e is DllNotFoundException)
        {
            return null;
        }
    }

    // /proc/cpuinfo, read ONCE, with "could not read the file" kept distinct
    // from "the file has no such key". They are different facts and only the
    // second says anything about the CPU: a host with procfs masked must not
    // be told its CPU "is not x86 at all", which is a claim about hardware
    // derived from a permissions error.
    private static void ReadCpuinfo(out bool readable, out string vendor, out string implementer)
    {
        readable = false;
        vendor = null;
        implementer = null;
        try
        {
            foreach (string line in File.ReadLines("/proc/cpuinfo"))
            {
                readable = true;
                int colon = line.IndexOf(':');
                if (colon < 0)
                {
                    continue;
                }

                string key = line.Substring(0, colon).Trim();
                string value = line.Substring(colon + 1).Trim();
                if (value.Length == 0)
                {
                    continue;
                }

                if (vendor == null && string.Equals(key, "vendor_id", StringComparison.Ordinal))
                {
                    vendor = value;
                }
                else if (implementer == null && string.Equals(key, "CPU implementer", StringComparison.Ordinal))
                {
                    implementer = value;
                }
            }
        }
        catch (IOException)
        {
            readable = false;
        }
        catch (UnauthorizedAccessException)
        {
            readable = false;
        }
    }

    // The vendor_id strings real x86 silicon reports. CPUID pads some of them
    // to twelve characters ("  Shanghai  ", "VIA VIA VIA "), but the kernel
    // and this probe both hand on a TRIMMED value, so the table is trimmed
    // too -- padded entries here could never match, and Zhaoxin and VIA
    // silicon would have been stamped SUSPECT for it.
    private static readonly string[] KnownX86Vendors =
    {
        "GenuineIntel", "AuthenticAMD", "HygonGenuine", "CentaurHauls",
        "Shanghai", "VIA VIA VIA", "GenuineTMx86", "Geode by NSC",
    };

    // The machine strings a kernel reports for each architecture .NET can
    // report for itself. Anything unrecognised is left unmatched rather than
    // guessed: an unknown pairing must not read as agreement.
    internal static Architecture? MachineToArchitecture(string machine)
    {
        switch (machine)
        {
            case "x86_64":
            case "amd64":
                return Architecture.X64;
            case "aarch64":
            case "arm64":
                return Architecture.Arm64;
            case "i386":
            case "i486":
            case "i586":
            case "i686":
                return Architecture.X86;
            case "armv7l":
            case "armv6l":
                return Architecture.Arm;
            default:
                return null;
        }
    }

    // The binfmt_misc interpreter names that would be registered to run THIS
    // process's architecture on some other one. Registration is not proof of
    // use — a real amd64 box with an arm64 cross-build setup has qemu-aarch64
    // registered and is perfectly native — so only an entry matching our own
    // architecture is interesting, and even that is reported as SUSPECT
    // rather than as translation.
    private static string[] QemuNamesForSelf(Architecture self)
    {
        switch (self)
        {
            case Architecture.X64: return new[] { "qemu-x86_64" };
            case Architecture.Arm64: return new[] { "qemu-aarch64" };
            case Architecture.X86: return new[] { "qemu-i386" };
            case Architecture.Arm: return new[] { "qemu-arm" };
            default: return new string[0];
        }
    }

    private static string Join(string so_far, string next)
    {
        return so_far == null ? next : so_far + "; " + next;
    }

    // THE PURE HALF. Everything this decides, it decides from the value it is
    // given; nothing here touches the machine.
    internal static Provenance DeriveProvenance(Signals s)
    {
        var p = new Provenance();
        string how = null;
        string unsure = null;
        bool translated = false;
        bool suspect = false;

        Architecture? kernelArch = s.Machine == null ? null : MachineToArchitecture(s.Machine);
        if (kernelArch.HasValue && kernelArch.Value != s.Self)
        {
            translated = true;
            how = Join(how, "kernel is " + s.Machine + ", this process is " + s.Self);
        }

        bool x86Process = s.Self == Architecture.X64 || s.Self == Architecture.X86;
        bool armProcess = s.Self == Architecture.Arm64 || s.Self == Architecture.Arm;
        if (!s.CpuinfoReadable)
        {
            unsure = Join(unsure, "/proc/cpuinfo could not be read, so the CPU underneath was not checked");
        }
        else if (x86Process && string.Equals(s.Vendor, "VirtualApple", StringComparison.Ordinal))
        {
            translated = true;
            how = Join(how, "cpuinfo vendor_id is VirtualApple — Apple Silicon running x86 under Rosetta");
        }
        else if (x86Process && s.Vendor == null)
        {
            translated = true;
            how = Join(how, "cpuinfo carries no vendor_id, so the CPU underneath is not x86 at all");
        }
        else if (x86Process && Array.IndexOf(KnownX86Vendors, s.Vendor) < 0)
        {
            suspect = true;
            how = Join(how, "cpuinfo vendor_id '" + s.Vendor + "' is not a vendor real x86 silicon reports");
        }
        else if (armProcess && s.Vendor != null)
        {
            translated = true;
            how = Join(how, "cpuinfo is x86-shaped (vendor_id " + s.Vendor + ") while this process is " + s.Self);
        }

        if (s.RosettaMarker)
        {
            translated = true;
            how = Join(how, "a Rosetta marker is present");
        }

        if (s.BinfmtHits.Count > 0)
        {
            suspect = true;
            how = Join(how, "a binfmt_misc interpreter for this very architecture is registered");
        }

        if (s.Machine == null)
        {
            unsure = Join(unsure, "uname() gave no machine, so the kernel's architecture could not be compared with this process's");
        }
        else if (!kernelArch.HasValue)
        {
            // AN UNKNOWN PAIRING MUST NOT READ AS AGREEMENT, which is what
            // MachineToArchitecture's own comment promises. A machine string
            // this probe cannot map (s390x, ppc64le, riscv64) skips the
            // mismatch check above, so without this the verdict would fall
            // through to "NATIVE — kernel and process agree on s390x" having
            // compared nothing at all.
            unsure = Join(unsure, "uname reports the machine as '" + s.Machine
                                  + "', which this probe does not recognise, so it could not be compared with "
                                  + s.Self);
        }

        if (translated)
        {
            p.Tag = "XLATED";
            p.Translated = true;
            p.Summary = "TRANSLATED (" + how + ")";
        }
        else if (suspect)
        {
            p.Tag = "SUSPECT";
            p.Summary = "SUSPECT (" + how + "). Nothing proves this process is translated,"
                        + " and nothing proves it is not.";
        }
        else if (unsure != null)
        {
            p.Tag = "UNKNOWN";
            p.Summary = "UNKNOWN — " + unsure + ".";
        }
        else
        {
            p.Tag = "native";
            p.Summary = "NATIVE — kernel and process agree on " + s.Machine
                        + ", /proc/cpuinfo is consistent with " + s.Self
                        + ", no Rosetta marker, no matching binfmt_misc interpreter.";
        }

        return p;
    }

    // THE IMPURE HALF: four readers, and the raw signal lines the report
    // prints so a reader can second-guess the verdict above.
    private static Provenance DetermineProvenance()
    {
        var sig = new Signals { Self = RuntimeInformation.ProcessArchitecture };
        sig.Machine = KernelMachine();
        ReadCpuinfo(out bool cpuinfoReadable, out string vendor, out string implementer);
        sig.CpuinfoReadable = cpuinfoReadable;
        sig.Vendor = vendor;
        sig.Implementer = implementer;
        sig.RosettaMarker = Directory.Exists("/run/rosetta")
                            || File.Exists("/proc/sys/fs/binfmt_misc/rosetta");

        string binfmt = "/proc/sys/fs/binfmt_misc";
        bool binfmtMounted = Directory.Exists(binfmt);
        if (binfmtMounted)
        {
            foreach (string name in QemuNamesForSelf(sig.Self))
            {
                string path = Path.Combine(binfmt, name);
                if (!File.Exists(path))
                {
                    continue;
                }

                string state = "registered";
                try
                {
                    string[] lines = File.ReadAllLines(path);
                    if (lines.Length > 0 && lines[0].Trim().Length > 0)
                    {
                        state = lines[0].Trim();
                    }
                }
                catch (IOException)
                {
                    // procfs entries can refuse a read; the registration
                    // itself is the signal, so keep it.
                }
                catch (UnauthorizedAccessException)
                {
                }

                sig.BinfmtHits.Add(name + " (" + state + ")");
            }
        }

        Provenance p = DeriveProvenance(sig);
        p.Signals.Add(string.Format(
            CultureInfo.InvariantCulture,
            "uname machine        : {0}", sig.Machine ?? "unavailable"));
        p.Signals.Add(string.Format(
            CultureInfo.InvariantCulture,
            "process architecture : {0}", sig.Self));
        p.Signals.Add(string.Format(
            CultureInfo.InvariantCulture,
            "/proc/cpuinfo        : {0}",
            !sig.CpuinfoReadable ? "could not be read"
                : sig.Vendor != null ? "vendor_id " + sig.Vendor + " (x86-shaped)"
                : sig.Implementer != null ? "CPU implementer " + sig.Implementer + " (arm-shaped)"
                : "readable, but neither vendor_id nor CPU implementer"));
        p.Signals.Add(string.Format(
            CultureInfo.InvariantCulture,
            "/run/rosetta         : {0}", sig.RosettaMarker ? "present" : "absent"));
        p.Signals.Add(string.Format(
            CultureInfo.InvariantCulture,
            "binfmt_misc for {0,-5}: {1}",
            sig.Self,
            !binfmtMounted ? "not mounted"
                : sig.BinfmtHits.Count == 0 ? "no matching interpreter registered"
                : string.Join(", ", sig.BinfmtHits)));
        return p;
    }

    // ------------------------------------------------------------- readings

    private static int failures;
    private static string rowTag = "native";

    private static Reading Read()
    {
        StackT cur = default;
        int rc = sigaltstack(IntPtr.Zero, ref cur);
        if (rc != 0)
        {
            // ACCUMULATED, NOT BAILED ON. One thread kind failing to report
            // is not a reason to lose the other five; the exit status carries
            // the failure instead.
            failures++;
            return Reading.Failure(Marshal.GetLastPInvokeError());
        }

        return Reading.Ok(cur.ss_sp.ToInt64(), cur.ss_flags, cur.ss_size.ToInt64());
    }

    private static void Row(string label)
    {
        Console.WriteLine(FormatRow(rowTag, label, Read()));
    }

    [UnmanagedCallersOnly]
    private static IntPtr ForeignThread(IntPtr arg)
    {
        // A thread the CLR did NOT create. The first managed frame here is
        // this one, so the reading is "after the runtime has attached it".
        //
        // NOTHING MAY ESCAPE THIS FRAME. An exception crossing an
        // UnmanagedCallersOnly boundary aborts the process, which would throw
        // away every row already printed -- the whole report, lost to the one
        // row that could not be taken.
        try
        {
            Row(PthreadRowLabel);
        }
        catch (Exception e)
        {
            try
            {
                Console.WriteLine(FormatSkip(
                    rowTag, PthreadRowLabel, "the reading threw " + e.GetType().Name + ": " + e.Message));
                failures++;
            }
            catch (Exception)
            {
                // Even the reporting failed; returning is still better than
                // unwinding into C.
            }
        }

        return IntPtr.Zero;
    }

    private static unsafe void RawPthreadRow()
    {
        IntPtr tid;
        int rc;
        try
        {
            rc = pthread_create(
                out tid,
                IntPtr.Zero,
                (IntPtr)(delegate* unmanaged<IntPtr, IntPtr>)&ForeignThread,
                IntPtr.Zero);
        }
        catch (EntryPointNotFoundException)
        {
            Console.WriteLine(FormatSkip(rowTag, PthreadRowLabel, PthreadNotInLibc));
            return;
        }
        catch (DllNotFoundException e)
        {
            Console.WriteLine(FormatSkip(rowTag, PthreadRowLabel, "libc not loadable: " + e.Message));
            return;
        }

        if (rc != 0)
        {
            Console.WriteLine(FormatSkip(
                rowTag,
                PthreadRowLabel,
                string.Format(CultureInfo.InvariantCulture, "pthread_create failed, rc {0}", rc)));
            return;
        }

        // A FAILED JOIN IS NOT A SILENT ROW. The thread may not have run, so
        // the row above may never have printed; saying so is the whole point
        // of FormatSkip.
        int joined = pthread_join(tid, IntPtr.Zero);
        if (joined != 0)
        {
            Console.WriteLine(FormatSkip(
                rowTag,
                PthreadRowLabel,
                string.Format(
                    CultureInfo.InvariantCulture,
                    "pthread_join failed, rc {0} — any row above for this thread may be missing", joined)));
            failures++;
        }
    }

    // FormatSysconf is separate from the call for the same reason FormatRow
    // is: the pre-2.34 answer is a branch that only a glibc older than this
    // probe's own SDK images can reach, so --self-check drives it instead.
    // The reason a libc with no sysconf at all produces, written once so the
    // live path and its self-check case cannot drift -- the same discipline
    // PthreadNotInLibc exists for.
    internal const string SysconfNotInLibc = "sysconf is not in this libc";

    internal static string FormatSysconfUnavailable(string label, string why)
    {
        return string.Format(CultureInfo.InvariantCulture, "{0,-24} = unavailable ({1})", label, why);
    }

    internal static string FormatSysconf(string label, long v)
    {
        if (v <= 0)
        {
            // NOT A SIZE OF -1. Before glibc 2.34 these names do not exist,
            // and sysconf's answer is "I have no value for that".
            return FormatSysconfUnavailable(label, string.Format(
                CultureInfo.InvariantCulture,
                "sysconf returned {0}; the name was added in glibc 2.34", v));
        }

        return string.Format(
            CultureInfo.InvariantCulture,
            "{0,-24} = {1} ({2} KiB)",
            label, v, (v / 1024.0).ToString("0.#", CultureInfo.InvariantCulture));
    }

    private static string Sysconf(string label, int name)
    {
        try
        {
            return FormatSysconf(label, sysconf(name));
        }
        catch (EntryPointNotFoundException)
        {
            return FormatSysconfUnavailable(label, SysconfNotInLibc);
        }
    }

    // ----------------------------------------------------------- self-check

    // The SS_DISABLE row and the sigaltstack-fails row of this story's
    // I/O matrix are defensive branches no real host reaches on demand, and
    // an unexercised branch is an unverified one. --self-check drives the
    // formatter with synthetic values and prints the expected rendering
    // beside the actual one, so the reader checks the check.
    private static int SelfCheck()
    {
        var cases = new[]
        {
            new
            {
                Name = "installed, 16 KiB (the amd64 reading)",
                Actual = FormatRow("native", "main thread", Reading.Ok(0x7F0000001000, 0, 16384)),
                Expect = "[native ] main thread                        flags=0                     sp=0x7F0000001000 size=16384 (16 KiB)",
            },
            new
            {
                Name = "installed, 24 KiB, handler currently running on it",
                Actual = FormatRow("XLATED", ".NET TP Worker", Reading.Ok(0x10, SS_ONSTACK, 24576)),
                Expect = "[XLATED ] .NET TP Worker                     flags=SS_ONSTACK            sp=0x10 size=24576 (24 KiB)",
            },
            new
            {
                Name = "SS_DISABLE — no altstack, despite a stale ss_size",
                Actual = FormatRow("native", "new Thread() default stack", Reading.Ok(0x10, SS_DISABLE, 24576)),
                Expect = "[native ] new Thread() default stack         flags=SS_DISABLE            NONE INSTALLED",
            },
            new
            {
                Name = "null ss_sp — no altstack either",
                Actual = FormatRow("native", "raw pthread (attached)", Reading.Ok(0, 0, 24576)),
                Expect = "[native ] raw pthread (attached)             flags=0                     NONE INSTALLED",
            },
            new
            {
                Name = "pthread_create absent from libc — a named skip, not a silent gap",
                Actual = FormatSkip("native", PthreadRowLabel, PthreadNotInLibc),
                Expect = "[native ] raw pthread (attached)             SKIPPED — pthread_create is not in libc here (glibc < 2.34 keeps it in libpthread)",
            },
            new
            {
                Name = "sigaltstack() failed — the errno, not a size",
                Actual = FormatRow("native", "main thread", Reading.Failure(22)),
                Expect = "[native ] main thread                        sigaltstack() failed, errno 22",
            },
            new
            {
                Name = "an unknown flag bit is shown, not swallowed",
                Actual = FormatRow("native", "main thread", Reading.Ok(0x20, SS_ONSTACK | 0x40, 8192)),
                Expect = "[native ] main thread                        flags=SS_ONSTACK|0x40       sp=0x20 size=8192 (8 KiB)",
            },
            new
            {
                Name = "sysconf on a modern glibc",
                Actual = FormatSysconf("sysconf(_SC_SIGSTKSZ)", 20480),
                Expect = "sysconf(_SC_SIGSTKSZ)    = 20480 (20 KiB)",
            },
            new
            {
                Name = "sysconf before glibc 2.34 — unavailable, not a size of -1",
                Actual = FormatSysconf("sysconf(_SC_SIGSTKSZ)", -1),
                Expect = "sysconf(_SC_SIGSTKSZ)    = unavailable (sysconf returned -1; the name was added in glibc 2.34)",
            },
            new
            {
                Name = "sysconf missing from libc entirely — not reported as a returned value",
                Actual = FormatSysconfUnavailable("sysconf(_SC_SIGSTKSZ)", SysconfNotInLibc),
                Expect = "sysconf(_SC_SIGSTKSZ)    = unavailable (sysconf is not in this libc)",
            },

            // THE VERDICT DERIVATION, which is the one piece of judgement here
            // and the one piece with a defect history: its first cut called a
            // Docker Desktop Rosetta container NATIVE, because uname says
            // x86_64, /run/rosetta is absent and binfmt_misc is empty there.
            new
            {
                Name = "provenance: VirtualApple on an x86 process is Rosetta",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = "x86_64", Self = Architecture.X64,
                    CpuinfoReadable = true, Vendor = "VirtualApple",
                }).Summary,
                Expect = "TRANSLATED (cpuinfo vendor_id is VirtualApple — Apple Silicon running x86 under Rosetta)",
            },
            new
            {
                Name = "provenance: kernel and process disagree (the qemu shape)",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = "aarch64", Self = Architecture.X64,
                    CpuinfoReadable = true,
                }).Summary,
                Expect = "TRANSLATED (kernel is aarch64, this process is X64; cpuinfo carries no vendor_id, so the CPU underneath is not x86 at all)",
            },
            new
            {
                Name = "provenance: GenuineIntel, no markers — a real amd64 box",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = "x86_64", Self = Architecture.X64,
                    CpuinfoReadable = true, Vendor = "GenuineIntel",
                }).Summary,
                Expect = "NATIVE — kernel and process agree on x86_64, /proc/cpuinfo is consistent with X64, no Rosetta marker, no matching binfmt_misc interpreter.",
            },
            new
            {
                Name = "provenance: aarch64 with a CPU implementer — native arm64",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = "aarch64", Self = Architecture.Arm64,
                    CpuinfoReadable = true, Implementer = "0x61",
                }).Summary,
                Expect = "NATIVE — kernel and process agree on aarch64, /proc/cpuinfo is consistent with Arm64, no Rosetta marker, no matching binfmt_misc interpreter.",
            },
            new
            {
                Name = "provenance: uname gave nothing — UNKNOWN, not native",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = null, Self = Architecture.Arm64,
                    CpuinfoReadable = true, Implementer = "0x61",
                }).Summary,
                Expect = "UNKNOWN — uname() gave no machine, so the kernel's architecture could not be compared with this process's.",
            },
            new
            {
                Name = "provenance: an unrecognised machine is UNKNOWN, never agreement",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = "s390x", Self = Architecture.X64,
                    CpuinfoReadable = true, Vendor = "GenuineIntel",
                }).Summary,
                Expect = "UNKNOWN — uname reports the machine as 's390x', which this probe does not recognise, so it could not be compared with X64.",
            },
            new
            {
                Name = "provenance: cpuinfo unreadable asserts nothing about the CPU",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = "x86_64", Self = Architecture.X64,
                    CpuinfoReadable = false,
                }).Summary,
                Expect = "UNKNOWN — /proc/cpuinfo could not be read, so the CPU underneath was not checked.",
            },
            new
            {
                Name = "provenance: Zhaoxin's trimmed vendor_id is not SUSPECT",
                Actual = DeriveProvenance(new Signals
                {
                    Machine = "x86_64", Self = Architecture.X64,
                    CpuinfoReadable = true, Vendor = "Shanghai",
                }).Summary,
                Expect = "NATIVE — kernel and process agree on x86_64, /proc/cpuinfo is consistent with X64, no Rosetta marker, no matching binfmt_misc interpreter.",
            },
            new
            {
                Name = "a fractional KiB keeps one decimal",
                Actual = FormatRow("native", "main thread", Reading.Ok(0x20, 0, 20480 + 512)),
                Expect = "[native ] main thread                        flags=0                     sp=0x20 size=20992 (20.5 KiB)",
            },
        };

        int bad = 0;
        Console.WriteLine("signal-stack-probe --self-check: row formatting against synthetic stack_t values");
        Console.WriteLine();
        foreach (var c in cases)
        {
            bool ok = string.Equals(c.Actual, c.Expect, StringComparison.Ordinal);
            if (!ok)
            {
                bad++;
            }

            Console.WriteLine((ok ? "  ok   " : "  FAIL ") + c.Name);
            Console.WriteLine("    expected |" + c.Expect + "|");
            Console.WriteLine("    actual   |" + c.Actual + "|");
            Console.WriteLine();
        }

        if (bad != 0)
        {
            Console.Error.WriteLine(string.Format(
                CultureInfo.InvariantCulture,
                "signal-stack-probe: {0} of {1} self-check cases did not render as expected —"
                + " the row formatter changed without its expectations moving with it",
                bad, cases.Length));
            return 1;
        }

        Console.WriteLine(string.Format(
            CultureInfo.InvariantCulture,
            "  {0} of {0} cases rendered as expected.", cases.Length));
        return 0;
    }

    // ----------------------------------------------------------------- main

    private const string Usage =
        "signal-stack-probe — reads sigaltstack() on each kind of .NET thread (DW-396)\n"
        + "\n"
        + "  signal-stack-probe                the measurement; Linux only\n"
        + "  signal-stack-probe --self-check   verify the row formatting; any OS\n"
        + "  signal-stack-probe --help\n"
        + "\n"
        + "Normally run through ../probe-signal-stack.sh, which supplies the container.\n";

    private static unsafe int Main(string[] args)
    {
        // THE WHOLE ARGUMENT LIST IS VALIDATED BEFORE ANY WORK, as its
        // siblings in folio-dotnet/build do: a run that measures and then
        // complains about an argument looks like a whole run. VALIDATION IS
        // ITS OWN PASS, and --help is acted on only after it -- printing the
        // usage for `--help --bogus` and exiting 0 would be that same rule
        // broken by the one flag whose job is to explain the rules.
        bool selfCheck = false;
        bool help = false;
        foreach (string arg in args)
        {
            switch (arg)
            {
                case "--self-check":
                    selfCheck = true;
                    break;
                case "--help":
                case "-h":
                    help = true;
                    break;
                default:
                    Console.Error.WriteLine(
                        "signal-stack-probe: unknown argument '" + arg + "' (--self-check, --help)");
                    return 2;
            }
        }

        if (help)
        {
            Console.Write(Usage);
            return 0;
        }

        if (selfCheck)
        {
            return SelfCheck();
        }

        // THE MEASUREMENT IS LINUX-ONLY AND SAYS SO RATHER THAN GUESSING.
        // macOS has sigaltstack too, so this would happily print a number —
        // one about a platform the defect does not exist on, which is worse
        // than no number.
        if (!RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
        {
            Console.Error.WriteLine(
                "signal-stack-probe: this measurement is Linux-only ("
                + RuntimeInformation.OSDescription
                + "); run ../probe-signal-stack.sh amd64|arm64 to get a container, or --self-check here.");
            return 2;
        }

        // AND IT IS LP64-ONLY, FOR A REASON IN THE STRUCT ABOVE. StackT
        // hard-codes the four bytes of padding 64-bit glibc puts between
        // ss_flags and ss_size; on i686 or armv7 there is no such padding and
        // ss_size would be read from the wrong offset -- a plausible-looking
        // number that is not the signal stack's size. A refusal beats that.
        if (IntPtr.Size != 8)
        {
            Console.Error.WriteLine(string.Format(
                CultureInfo.InvariantCulture,
                "signal-stack-probe: this probe reads the 64-bit stack_t layout and this process is {0}-bit"
                + " ({1}); run it on a 64-bit runtime, where the measurement this diagnostic exists for lives.",
                IntPtr.Size * 8, RuntimeInformation.ProcessArchitecture));
            return 2;
        }

        Provenance provenance = DetermineProvenance();
        rowTag = provenance.Tag;

        Console.WriteLine("signal-stack-probe — DW-396: what alternate signal stack does a .NET thread get?");
        Console.WriteLine();
        Console.WriteLine("os                   : " + RuntimeInformation.OSDescription);
        Console.WriteLine("runtime              : " + RuntimeInformation.FrameworkDescription);
        foreach (string s in provenance.Signals)
        {
            Console.WriteLine(s);
        }

        Console.WriteLine("PROVENANCE           : " + provenance.Verdict);
        Console.WriteLine();
        Console.WriteLine(Sysconf("sysconf(_SC_SIGSTKSZ)", _SC_SIGSTKSZ));
        Console.WriteLine(Sysconf("sysconf(_SC_MINSIGSTKSZ)", _SC_MINSIGSTKSZ));
        Console.WriteLine("Go gsignal stack         = 32768 (32 KiB) — the room its SA_ONSTACK handlers expect");
        Console.WriteLine();

        Row("main thread");

        using (var done = new ManualResetEventSlim(false))
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                Row(".NET TP Worker");
                done.Set();
            });
            done.Wait();
        }

        var t = new Thread(() => Row("new Thread() default stack"));
        t.Start();
        t.Join();

        // The managed stack size sizes the ORDINARY stack, not the signal
        // stack. This row is here to show that, not because 16 MiB is a
        // plausible setting.
        var tBig = new Thread(() => Row("new Thread() 16 MiB stack"), 16 * 1024 * 1024);
        tBig.Start();
        tBig.Join();

        // A thread that has been alive a moment, in case the runtime attaches
        // a short-lived one differently.
        var tLive = new Thread(() =>
        {
            Thread.Sleep(50);
            Row("new Thread() after 50ms");
        });
        tLive.Start();
        tLive.Join();

        RawPthreadRow();

        Console.WriteLine();
        Console.WriteLine("--- can the binding enlarge it itself, and does the runtime survive? ---");
        var tFix = new Thread(() =>
        {
            Console.WriteLine(FormatRow(rowTag, "  before", Read()));
            const int want = 1024 * 1024;
            IntPtr mem = Marshal.AllocHGlobal(want);
            var ss = new StackT { ss_sp = mem, ss_flags = 0, ss_size = (IntPtr)want };
            int r = sigaltstack(ref ss, IntPtr.Zero);
            Console.WriteLine(string.Format(
                CultureInfo.InvariantCulture,
                "          sigaltstack(set {0} KiB) -> {1}{2}",
                want / 1024, r, r != 0 ? " errno " + Marshal.GetLastPInvokeError() : string.Empty));
            if (r != 0)
            {
                failures++;
            }

            Console.WriteLine(FormatRow(rowTag, "  after", Read()));

            // The CLR installed that altstack for its OWN SIGSEGV handling —
            // null checks, GC write barriers, stack-overflow detection. The
            // swap REPLACES the memory with more of it, so those paths must
            // still work; these two exercise exactly them.
            try
            {
                object o = null;
                try
                {
                    _ = o.ToString();
                }
                catch (NullReferenceException)
                {
                    Console.WriteLine("          NullReferenceException still caught normally after the swap.");
                }

                GC.Collect(2, GCCollectionMode.Forced, true);
                Console.WriteLine("          Full blocking GC completed on the swapped thread.");
            }
            catch (Exception e)
            {
                Console.WriteLine("          runtime unhappy after swap: " + e.GetType().Name + ": " + e.Message);
                failures++;
            }

            // NOT FREED. The altstack must outlive the thread's ability to
            // take a signal, and this process is about to exit anyway — a
            // leak of 1 MiB in a diagnostic, against a use-after-free in a
            // signal handler.
        });
        tFix.Start();
        tFix.Join();

        if (failures != 0)
        {
            Console.Error.WriteLine(string.Format(
                CultureInfo.InvariantCulture,
                "signal-stack-probe: {0} reading(s) failed — the rows above name the errno;"
                + " the rest of the report is still valid", failures));
            return 1;
        }

        return 0;
    }
}
