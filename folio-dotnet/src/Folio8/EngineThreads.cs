using System;
using System.Collections.Generic;
using System.Globalization;
using System.Runtime.ExceptionServices;
using System.Runtime.InteropServices;
using System.Threading;

/// <summary>
/// The threads this binding owns, and the only threads that ever enter the
/// native engine. Work submitted here runs on one of them and its result —
/// value or exception — comes back to the caller's thread.
/// </summary>
/// <remarks>
/// <b>Enlarging the alternate signal stack is the fix; owning the thread is
/// what makes the fix possible.</b> The CLR installs a fixed-size alternate
/// signal stack on every thread it touches — 16 KiB on amd64, 24 KiB on
/// arm64 — and Go, under cgo, <i>adopts</i> the stack it finds instead of
/// installing its own 32 KiB one. So a Go <c>SA_ONSTACK</c> handler entered
/// from an ordinary CLR thread runs in less room than Go sizes for itself,
/// the kernel turns the overflow into SIGSEGV, and the CLR reports
/// <c>Internal CLR error (0x80131506)</c>. That is DW-396, and it is why the
/// two Linux RIDs were withdrawn from 1.1.0.
/// <para>
/// Three properties carry the fix, and each is load-bearing:
/// </para>
/// <list type="bullet">
/// <item><description>
/// The enlargement happens <b>before the thread's first crossing</b>,
/// because Go's <c>minitSignalStack</c> adopts whatever it finds at
/// <c>needm</c> time — after that, the reading is fixed for the life of the
/// attachment.
/// </description></item>
/// <item><description>
/// The memory is <b>never returned</b>. A thread that can still take a
/// signal must still own its alternate stack, so these threads live for the
/// process's lifetime and the buffer is allocated once and kept.
/// </description></item>
/// <item><description>
/// A pool, not a single thread. cgo runs a <c>c-shared</c> export <b>on the
/// calling thread</b> rather than handing it to Go's scheduler, so one
/// marshalling thread would serialise every render in the process onto one
/// core. A pool of <i>ordinary</i> threads, though, reproduces the defect
/// exactly — the pool is the holder of the fix, not the fix.
/// </description></item>
/// </list>
/// <para>
/// <b>One path on every platform.</b> Windows crosses this boundary too,
/// even though it has no POSIX signals and does not need the enlargement.
/// Two call paths behind a conditional would leave the POSIX one unexercised
/// by every Windows leg of CI, which is how a Linux path rots between
/// releases. Only the <c>sigaltstack</c> step is POSIX-conditional; the call
/// path never is.
/// </para>
/// <para>
/// A caller with far more threads than cores previously had that many
/// concurrent renders in flight and now queues here instead. Rendering is
/// CPU-bound, so that is throughput-neutral-to-better, but it is an
/// observable change under high concurrency.
/// </para>
/// </remarks>
internal static class EngineThreads
{
    /// <summary>
    /// How much alternate signal stack each engine thread is given, in bytes.
    /// </summary>
    /// <remarks>
    /// Generous and fixed, deliberately. <c>SIGSTKSZ</c> is a compile-time
    /// constant before glibc 2.34 and a <c>sysconf()</c>-backed runtime value
    /// after it — because modern register save areas (AVX-512) are large — so
    /// no single queried value is right across the range this package
    /// supports. A megabyte per engine thread, paid once at creation, costs
    /// nothing against that ambiguity, and it is the size the DW-396 spike
    /// measured working.
    /// </remarks>
    internal const int SignalStackBytes = 1024 * 1024;

    /// <summary>The name every engine thread carries, plus its index.</summary>
    /// <remarks>
    /// A name, not a decoration: a test asserts that the thread which
    /// actually crosses the ABI is one of these and not a
    /// <c>.NET TP Worker</c>, and a crash report from a consumer names the
    /// thread too.
    /// </remarks>
    internal const string ThreadNamePrefix = "folio8-engine-";

    // stack_t, 64-bit, AND THE TWO POSIX FAMILIES DO NOT AGREE ON ITS FIELD
    // ORDER. Linux/glibc is `void *ss_sp; int ss_flags; <4 bytes padding>;
    // size_t ss_size`, and Darwin is `void *ss_sp; size_t ss_size; int
    // ss_flags`. The wrong one of these does not fail to compile and does not
    // obviously misbehave: it passes a size of zero where the kernel expects
    // one and comes back ENOMEM — measured, on macOS, against the Linux
    // layout. So the layout is CHOSEN by asking the kernel its name, never
    // assumed, and the result is read back and checked below.
    //
    // The padding is written out rather than left to the marshaller, so each
    // reads against its own C declaration.
    //
    // DELIBERATELY NOT SHARED WITH build/signal-stack-probe. That tool is a
    // Linux-only diagnostic outside the shipped floor; this declaration lives
    // inside netstandard2.0 and answers to it.
    [StructLayout(LayoutKind.Sequential)]
    private struct LinuxStackT
    {
        internal IntPtr ss_sp;
        internal int ss_flags;
        private int _pad;
        internal IntPtr ss_size;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct DarwinStackT
    {
        internal IntPtr ss_sp;
        internal IntPtr ss_size;
        internal int ss_flags;
        private int _pad;
    }

    [DllImport("libc", EntryPoint = "sigaltstack", SetLastError = true)]
    private static extern int sigaltstack_linux(ref LinuxStackT ss, IntPtr old);

    [DllImport("libc", EntryPoint = "sigaltstack", SetLastError = true)]
    private static extern int sigaltstack_linux_read(IntPtr ss, ref LinuxStackT old);

    [DllImport("libc", EntryPoint = "sigaltstack", SetLastError = true)]
    private static extern int sigaltstack_darwin(ref DarwinStackT ss, IntPtr old);

    [DllImport("libc", EntryPoint = "sigaltstack", SetLastError = true)]
    private static extern int sigaltstack_darwin_read(IntPtr ss, ref DarwinStackT old);

    // uname(2), to name the kernel. The buffer is far larger than any of the
    // six fields needs — _UTSNAME_LENGTH is 65 on glibc and 256 on Darwin —
    // so a longer field on some future libc overruns into slack rather than
    // into memory this process does not own. Only the FIRST field, sysname
    // at offset zero, is read.
    [DllImport("libc", EntryPoint = "uname", SetLastError = true)]
    private static extern int uname(byte[] buffer);

    private static readonly object StartGate = new object();
    private static readonly Queue<WorkItem> Pending = new Queue<WorkItem>();

    // The alternate signal stacks, held forever on purpose. Freeing one while
    // its thread can still take a signal is how a fix becomes a crash, and
    // these threads never stop. The list exists so the intent is written down
    // rather than inferred from an unreferenced pointer.
    private static readonly List<IntPtr> SignalStacks = new List<IntPtr>();

    // Its own gate, NOT StartGate. Start() holds StartGate while it waits for
    // every thread to report, so a thread reaching for StartGate on its way to
    // reporting would deadlock the pool at creation.
    private static readonly object SignalStackGate = new object();

    // VOLATILE, so the hot path can read it without taking StartGate. Every
    // crossing passes through Run, and a global monitor in front of the queue
    // would be contention the pool exists to avoid. The write still happens
    // under StartGate, and it is written LAST — after the threads exist and
    // their signal stacks are sized — so a reader that sees true sees a pool
    // that is ready.
    private static volatile bool _started;
    private static int _count;
    private static string _startFailure;

    // How many crossings the pool has served, and where the last one ran.
    // These exist so that "the call went through the pool" is OBSERVABLE
    // rather than inferred from a return value that would look identical if
    // the hop were deleted — which is exactly the mistake that let
    // OutstandingNativeAllocations bypass the funnel in the first place.
    private static long _served;
    private static int _lastServedOn;

    [ThreadStatic]
    private static bool _isEngineThread;

    /// <summary>
    /// Whether the calling thread is one this pool owns. Work submitted from
    /// such a thread runs inline.
    /// </summary>
    internal static bool IsEngineThread
    {
        get { return _isEngineThread; }
    }

    /// <summary>How many threads the pool started. Zero before first use.</summary>
    internal static int Count
    {
        get { lock (StartGate) { return _count; } }
    }

    /// <summary>
    /// How many crossings have been served, counting both the queued and the
    /// inline paths. A test reads it across a call to prove the call took the
    /// hop at all.
    /// </summary>
    internal static long Served
    {
        get { return Interlocked.Read(ref _served); }
    }

    /// <summary>
    /// The managed id of the thread the last crossing ran on — an engine
    /// thread's, never the submitting caller's unless the caller was itself
    /// an engine thread.
    /// </summary>
    internal static int LastServedOn
    {
        get { return Volatile.Read(ref _lastServedOn); }
    }

    /// <summary>
    /// Runs <paramref name="work"/> on an engine thread and returns its
    /// result on the calling thread.
    /// </summary>
    /// <remarks>
    /// <b>Re-entrancy runs inline.</b> Crossings nest — <c>Invoke</c> calls
    /// <c>EnsureAbi</c>, which crosses in its own right — so work submitted
    /// from a thread that is already an engine thread must not take a second
    /// queue hop: with every engine thread waiting on work only an engine
    /// thread could run, the pool would deadlock.
    /// </remarks>
    internal static T Run<T>(Func<T> work)
    {
        T value = default(T);
        Run(delegate { value = work(); });
        return value;
    }

    /// <summary>
    /// Runs <paramref name="work"/> on an engine thread, waits for it, and
    /// rethrows anything it threw on the calling thread.
    /// </summary>
    /// <remarks>
    /// <see cref="ExceptionDispatchInfo"/> is the rethrow mechanism: the
    /// caller sees the same exception instance, with the stack it was thrown
    /// with, rather than a wrapper naming a thread the caller never asked for.
    /// </remarks>
    internal static void Run(Action work)
    {
        if (_isEngineThread)
        {
            Interlocked.Increment(ref _served);
            Volatile.Write(ref _lastServedOn, Thread.CurrentThread.ManagedThreadId);
            work();
            return;
        }

        if (!_started)
        {
            Start();
        }

        WorkItem item = new WorkItem(work);
        lock (Pending)
        {
            Pending.Enqueue(item);
            Monitor.Pulse(Pending);
        }

        lock (item.Gate)
        {
            while (!item.Done)
            {
                Monitor.Wait(item.Gate);
            }
        }

        Interlocked.Increment(ref _served);
        Volatile.Write(ref _lastServedOn, item.ServedOn);

        if (item.Error != null)
        {
            item.Error.Throw();
        }
    }

    /// <summary>
    /// Starts the pool once. Every thread has its alternate signal stack
    /// enlarged and reports back before <see cref="Start"/> returns, so no
    /// call is ever handed to a thread that is still running on the CLR's
    /// small one.
    /// </summary>
    private static void Start()
    {
        lock (StartGate)
        {
            if (_startFailure != null)
            {
                // A pool that could not size its signal stacks is not a pool
                // that may quietly serve calls anyway. Refuse identically
                // every time rather than retrying into the same failure.
                throw new InvalidOperationException(_startFailure);
            }
            if (_started)
            {
                return;
            }

            // AT LEAST TWO, EVEN ON A ONE-CORE HOST. Environment.ProcessorCount
            // is 1 in a cpu-limited container, and a single-threaded pool has
            // no margin at all: any crossing that ends up submitted rather
            // than run inline would wait on the one thread that is already
            // inside a crossing, and the process would hang rather than fail.
            // Two threads cost one idle thread and one megabyte.
            int wanted = Environment.ProcessorCount;
            if (wanted < 2)
            {
                wanted = 2;
            }

            Startup startup = new Startup(wanted);
            for (int i = 0; i < wanted; i++)
            {
                Thread thread = new Thread(new ParameterizedThreadStart(Serve));
                thread.IsBackground = true;
                thread.Name = ThreadNamePrefix + i.ToString(CultureInfo.InvariantCulture);
                thread.Start(startup);
            }

            startup.WaitForAll();
            if (startup.Failure != null)
            {
                _startFailure = startup.Failure;
                throw new InvalidOperationException(_startFailure);
            }

            _count = wanted;
            _started = true;
        }
    }

    /// <summary>
    /// One engine thread: enlarge, report, then serve until the process ends.
    /// </summary>
    private static void Serve(object state)
    {
        Startup startup = (Startup)state;
        try
        {
            EnlargeSignalStack();
        }
        catch (Exception error)
        {
            startup.Failed(error.Message);
            return;
        }

        // ONLY NOW. Setting this before the enlargement would let a nested
        // submission run inline on a thread that is still on the CLR's
        // 16 KiB — which is the whole defect.
        _isEngineThread = true;
        startup.Ready();

        for (;;)
        {
            WorkItem item;
            lock (Pending)
            {
                while (Pending.Count == 0)
                {
                    Monitor.Wait(Pending);
                }
                item = Pending.Dequeue();
            }

            item.ServedOn = Thread.CurrentThread.ManagedThreadId;

            // A DEQUEUED ITEM IS ALWAYS COMPLETED, EVEN IF THIS THREAD IS
            // DYING. Run has no timeout — deliberately, since a render has no
            // deadline the binding could pick — so an item whose serving
            // thread left without pulsing its gate would park its caller for
            // the life of the process. The finally is the only thing standing
            // between an unexpected throw here and a hang over there.
            try
            {
                try
                {
                    item.Work();
                }
                catch (Exception error)
                {
                    item.Error = ExceptionDispatchInfo.Capture(error);
                }
            }
            finally
            {
                lock (item.Gate)
                {
                    item.Done = true;
                    Monitor.PulseAll(item.Gate);
                }
            }
        }
    }

    /// <summary>
    /// Replaces this thread's alternate signal stack with a larger one, on
    /// POSIX. A no-op where there are no POSIX signals.
    /// </summary>
    /// <remarks>
    /// This <b>replaces the memory with more of it</b> rather than removing
    /// it. The CLR installed an alternate stack for its own SIGSEGV handling
    /// — null checks, GC write barriers, stack-overflow detection — and those
    /// paths are not deprived by this; they are given more room too.
    /// </remarks>
    private static void EnlargeSignalStack()
    {
        if (_induceStartFailure)
        {
            // TEST SEAM. See InduceStartFailureForTesting.
            throw CouldNotEnlarge("sigaltstack() returned -1, errno 12, induced by a test seam");
        }

        if (!IsPosix())
        {
            // Windows has no POSIX signals, so there is nothing to enlarge —
            // and no libc P/Invoke is attempted. The CALL PATH is still the
            // same one Linux takes; only this step is conditional.
            return;
        }

        if (IntPtr.Size != 8)
        {
            // BOTH LAYOUTS BELOW ARE LP64, AND A 32-BIT POSIX PROCESS IS
            // REFUSED RATHER THAN SERVED. Each writes out the padding its
            // 64-bit ABI inserts; on a 32-bit POSIX target there is no such
            // padding and a field would be read from the wrong offset — a
            // plausible-looking wrong answer rather than an error. Declining
            // to guess is right; declining and then SERVING would leave a
            // thread on the runtime's small alternate stack, which is DW-396
            // unchanged. No 32-bit POSIX runtime identifier is shipped, so
            // nothing supported reaches this.
            throw CouldNotEnlarge(
                "this process is " + (IntPtr.Size * 8).ToString(CultureInfo.InvariantCulture) +
                "-bit and the alternate-stack layout this binding knows is 64-bit only");
        }

        // WHICH KERNEL, ASKED RATHER THAN ASSUMED — AND NO DEFAULT. stack_t's
        // field order is not portable, and the families this binding can
        // write down are Linux and Darwin. A kernel it cannot name is refused
        // rather than handed the Linux layout on the chance it fits: the BSDs
        // use Darwin's order, and guessing wrong is the ENOMEM this code
        // already met once on macOS.
        string kernel = KernelName();
        bool darwin;
        if (string.Equals(kernel, "Linux", StringComparison.Ordinal))
        {
            darwin = false;
        }
        else if (string.Equals(kernel, "Darwin", StringComparison.Ordinal))
        {
            darwin = true;
        }
        else
        {
            throw CouldNotEnlarge(
                "this kernel reports itself as " + (kernel == null ? "nothing uname() could return" : "'" + kernel + "'") +
                ", and the alternate-stack layouts this binding knows are Linux's and Darwin's");
        }

        // HEAP MEMORY, WITH NO GUARD PAGE, AND THAT IS A DELIBERATE TRADE.
        // mmap with PROT_NONE either side would turn an overflow of this
        // stack into a fault at the boundary instead of into corruption of
        // whatever the allocator put next to it — and silent corruption of a
        // neighbour is this story's own failure class. It is not done here
        // for two reasons: mmap/mprotect are a second and third libc surface
        // to carry across the floor for a case a megabyte makes remote (the
        // frames that overflowed 16 KiB are nowhere near 1 MiB), and the
        // memory is never released, so the allocator's own bookkeeping around
        // it never moves. If a future overflow is ever observed at this size,
        // a guarded mapping is the answer and this comment is the place it
        // was deferred.
        IntPtr memory = Marshal.AllocHGlobal(SignalStackBytes);
        int outcome;
        long installed;
        try
        {
            outcome = Install(darwin, memory);
            installed = outcome == 0 ? InstalledSize(darwin) : 0;
        }
        catch (Exception reason)
        {
            // libc entry points are not uniformly reachable across the range
            // this package supports — pthread_create lived in libpthread
            // until glibc's 2.34 merge, and a DllImport that cannot bind
            // throws rather than returning. Fail here rather than serve.
            //
            // MUSL ARRIVES HERE, AND THAT IS ACCEPTED. DllImport("libc")
            // resolves nothing on Alpine, so an Alpine consumer that somehow
            // reached this code would now fail at the first crossing rather
            // than at load. It cannot in practice: no linux-musl RID is
            // packed, deliberately, so musl's loader refuses the engine
            // itself long before a pool thread starts — the install-time
            // absence the spec asks for is unchanged. This message is the
            // backstop if that ever ceases to be true.
            Marshal.FreeHGlobal(memory);
            throw new InvalidOperationException(
                "folio8: this engine thread could not enlarge its alternate signal stack — sigaltstack() was not reachable through libc (" +
                reason.Message + "). " + WhyItMatters, reason);
        }

        if (outcome != 0)
        {
            int errno = Marshal.GetLastWin32Error();
            Marshal.FreeHGlobal(memory);
            throw CouldNotEnlarge(
                "sigaltstack() returned " + outcome.ToString(CultureInfo.InvariantCulture) +
                ", errno " + errno.ToString(CultureInfo.InvariantCulture));
        }

        // READ BACK, BECAUSE A SUCCESSFUL CALL IS NOT A LARGER STACK. The
        // field order above is chosen from the kernel's own name; checking
        // what the kernel now reports turns a wrong choice into this message
        // rather than into a thread that serves on the small default.
        if (installed != SignalStackBytes)
        {
            Marshal.FreeHGlobal(memory);
            throw CouldNotEnlarge(
                "it asked for " + SignalStackBytes.ToString(CultureInfo.InvariantCulture) +
                " bytes and the kernel reports " + installed.ToString(CultureInfo.InvariantCulture));
        }

        // Held for the life of the process, from here on. See SignalStacks.
        lock (SignalStackGate)
        {
            SignalStacks.Add(memory);
        }
    }

    private const string WhyItMatters =
        "The folio8 engine must not be entered on a thread whose signal stack is the runtime's small default: the Go runtime adopts it, and its handlers overflow it.";

    /// <summary>
    /// Every way this step can fail says the same thing, so they say it in
    /// one place: what went wrong, and why that is fatal rather than a
    /// degradation.
    /// </summary>
    private static InvalidOperationException CouldNotEnlarge(string detail)
    {
        return new InvalidOperationException(
            "folio8: this engine thread could not enlarge its alternate signal stack — " + detail + ". " + WhyItMatters);
    }

    /// <summary>Installs the enlarged stack, in this kernel's field order.</summary>
    private static int Install(bool darwin, IntPtr memory)
    {
        if (darwin)
        {
            DarwinStackT wanted = new DarwinStackT
            {
                ss_sp = memory,
                ss_size = (IntPtr)SignalStackBytes,
                ss_flags = 0,
            };
            return sigaltstack_darwin(ref wanted, IntPtr.Zero);
        }

        LinuxStackT linux = new LinuxStackT
        {
            ss_sp = memory,
            ss_flags = 0,
            ss_size = (IntPtr)SignalStackBytes,
        };
        return sigaltstack_linux(ref linux, IntPtr.Zero);
    }

    /// <summary>
    /// What the kernel now reports for this thread, through
    /// <c>sigaltstack(NULL, &amp;old)</c> — the same reading the DW-396 probe
    /// takes. Zero when there is no alternate stack at all, which
    /// <c>SS_DISABLE</c> or a null <c>ss_sp</c> both mean regardless of the
    /// size beside them.
    /// </summary>
    private static long InstalledSize(bool darwin)
    {
        const int SsDisable = 2;
        if (darwin)
        {
            DarwinStackT current = default(DarwinStackT);
            if (sigaltstack_darwin_read(IntPtr.Zero, ref current) != 0)
            {
                return 0;
            }
            if ((current.ss_flags & SsDisable) != 0 || current.ss_sp == IntPtr.Zero)
            {
                return 0;
            }
            return current.ss_size.ToInt64();
        }

        LinuxStackT linux = default(LinuxStackT);
        if (sigaltstack_linux_read(IntPtr.Zero, ref linux) != 0)
        {
            return 0;
        }
        if ((linux.ss_flags & SsDisable) != 0 || linux.ss_sp == IntPtr.Zero)
        {
            return 0;
        }
        return linux.ss_size.ToInt64();
    }

    /// <summary>
    /// This kernel's <c>sysname</c>, from <c>uname()</c>, or null when it
    /// could not be read. Null is a MISSING ANSWER and the caller refuses on
    /// it; it is never read as "not Darwin".
    /// </summary>
    /// <remarks>
    /// <see cref="Environment.OSVersion"/> cannot answer this: modern .NET
    /// reports <see cref="PlatformID.Unix"/> on macOS as well as on Linux,
    /// and <c>RuntimeInformation</c> — which could — arrived in .NET
    /// Framework 4.7.1, above this assembly's floor.
    /// </remarks>
    private static string KernelName()
    {
        // sysname is the first field, at offset zero, NUL-terminated in every
        // libc that has uname(). The rest of the buffer is never read.
        byte[] buffer = new byte[2048];
        if (uname(buffer) != 0)
        {
            return null;
        }
        int end = 0;
        while (end < buffer.Length && buffer[end] != 0)
        {
            end++;
        }
        if (end == 0)
        {
            return null;
        }
        return System.Text.Encoding.ASCII.GetString(buffer, 0, end);
    }

    /// <summary>Whether this process is running on a POSIX platform.</summary>
    /// <remarks>
    /// <see cref="RuntimeInformation"/> would be the modern spelling and is
    /// unavailable on the floor: it arrived in .NET Framework 4.7.1.
    /// <see cref="Environment.OSVersion"/> is present everywhere from 1.1,
    /// and this is the same reading <c>NativeLibraryLoader</c> takes.
    /// </remarks>
    private static bool IsPosix()
    {
        PlatformID platform = Environment.OSVersion.Platform;
        return platform == PlatformID.Unix || platform == PlatformID.MacOSX;
    }

    // ------------------------------------------------------------ test seam

    private static volatile bool _induceStartFailure;
    private static bool _savedStarted;
    private static int _savedCount;

    /// <summary>
    /// Makes the next pool start fail the way a refused <c>sigaltstack()</c>
    /// makes it fail. For tests only.
    /// </summary>
    /// <remarks>
    /// This path PERMANENTLY DISABLES the library, which is exactly why it
    /// must be exercised rather than reasoned about — and why there is no way
    /// to reach it from outside. The threads a failed start creates throw
    /// before they allocate anything or mark themselves as engine threads, so
    /// they simply exit; the pool that was already running is untouched and
    /// is restored by <see cref="EndInducedStartFailureForTesting"/>.
    /// </remarks>
    internal static void InduceStartFailureForTesting()
    {
        lock (StartGate)
        {
            _savedStarted = _started;
            _savedCount = _count;
            _induceStartFailure = true;
            _startFailure = null;
            _started = false;
            _count = 0;
        }
    }

    /// <summary>Restores the pool this process was running before. For tests only.</summary>
    internal static void EndInducedStartFailureForTesting()
    {
        lock (StartGate)
        {
            _induceStartFailure = false;
            _startFailure = null;
            _count = _savedCount;
            _started = _savedStarted;
        }
    }

    /// <summary>One submitted unit of work, and the caller waiting on it.</summary>
    private sealed class WorkItem
    {
        internal readonly object Gate = new object();
        internal readonly Action Work;
        internal ExceptionDispatchInfo Error;
        internal bool Done;
        internal int ServedOn;

        internal WorkItem(Action work)
        {
            Work = work;
        }
    }

    /// <summary>
    /// The rendezvous between <see cref="Start"/> and the threads it starts.
    /// It reports the FIRST failure and keeps waiting for the rest: a thread
    /// that never reports would hang the caller that started it.
    /// </summary>
    private sealed class Startup
    {
        private readonly object _gate = new object();
        private readonly int _expected;
        private int _reported;
        private string _failure;

        internal Startup(int expected)
        {
            _expected = expected;
        }

        internal string Failure
        {
            get { lock (_gate) { return _failure; } }
        }

        internal void Ready()
        {
            Report(null);
        }

        internal void Failed(string reason)
        {
            Report(reason ?? "folio8: an engine thread failed to start, without a reason.");
        }

        internal void WaitForAll()
        {
            lock (_gate)
            {
                while (_reported < _expected)
                {
                    Monitor.Wait(_gate);
                }
            }
        }

        private void Report(string failure)
        {
            lock (_gate)
            {
                if (failure != null && _failure == null)
                {
                    _failure = failure;
                }
                _reported++;
                Monitor.PulseAll(_gate);
            }
        }
    }
}
