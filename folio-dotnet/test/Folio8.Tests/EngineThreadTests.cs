using System;
using System.Collections.Generic;
using System.Globalization;
using System.Runtime.InteropServices;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// DW-396's fix, asserted as a mechanism rather than as the absence of a
    /// crash.
    /// </summary>
    /// <remarks>
    /// A run of clean renders is not evidence here — that is exactly the
    /// reading that cleared the Linux natives for 1.1.0 and was wrong. So
    /// these tests take the same <c>sigaltstack(NULL, &amp;old)</c> reading
    /// the DW-396 probe takes, on the thread that <b>actually crosses the
    /// ABI</b>, and assert two things about it: that the thread is one the
    /// binding owns, and that its alternate signal stack is the enlarged size
    /// rather than the runtime's 16 or 24 KiB.
    /// <para>
    /// The reading is taken from inside a <c>Native.Call</c> delegate, which
    /// is the closure the P/Invoke itself runs in — so it observes the
    /// crossing thread by construction and not by inference.
    /// </para>
    /// </remarks>
    [Collection("native")]
    public class EngineThreadTests
    {
        // stack_t, LP64 — AND THE FIELD ORDER DIFFERS BY KERNEL. Linux is
        // `ss_sp; ss_flags; <pad>; ss_size`; Darwin is `ss_sp; ss_size;
        // ss_flags`. Re-declared here rather than reached for across the
        // assembly boundary: the binding's copies are private to the floor it
        // ships on, and a test that shared them would be asserting the
        // binding's own declaration back at it.
        [StructLayout(LayoutKind.Sequential)]
        private struct LinuxStackT
        {
            public IntPtr ss_sp;
            public int ss_flags;
            private int _pad;
            public IntPtr ss_size;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct DarwinStackT
        {
            public IntPtr ss_sp;
            public IntPtr ss_size;
            public int ss_flags;
            private int _pad;
        }

        [DllImport("libc", EntryPoint = "sigaltstack", SetLastError = true)]
        private static extern int sigaltstack_linux(IntPtr ss, ref LinuxStackT old);

        [DllImport("libc", EntryPoint = "sigaltstack", SetLastError = true)]
        private static extern int sigaltstack_darwin(IntPtr ss, ref DarwinStackT old);

        [DllImport("libc", EntryPoint = "uname", SetLastError = true)]
        private static extern int uname(byte[] buffer);

        private static bool IsPosix64()
        {
            PlatformID platform = Environment.OSVersion.Platform;
            return IntPtr.Size == 8 && (platform == PlatformID.Unix || platform == PlatformID.MacOSX);
        }

        private static bool IsDarwin()
        {
            byte[] buffer = new byte[2048];
            if (uname(buffer) != 0)
            {
                return false;
            }
            int end = 0;
            while (end < buffer.Length && buffer[end] != 0)
            {
                end++;
            }
            return string.Equals(System.Text.Encoding.ASCII.GetString(buffer, 0, end), "Darwin", StringComparison.Ordinal);
        }

        /// <summary>
        /// The alternate signal stack of the calling thread in bytes, or −1
        /// where the reading cannot be taken. SS_DISABLE, or a null ss_sp, is
        /// an ANSWER — "no alternate stack" — and not a size; a stale ss_size
        /// beside either would be a measurement of nothing.
        /// </summary>
        private static long SignalStackSize()
        {
            const int SsDisable = 2;
            if (!IsPosix64())
            {
                return -1;
            }
            if (IsDarwin())
            {
                DarwinStackT current = default(DarwinStackT);
                if (sigaltstack_darwin(IntPtr.Zero, ref current) != 0)
                {
                    return -1;
                }
                if ((current.ss_flags & SsDisable) != 0 || current.ss_sp == IntPtr.Zero)
                {
                    return 0;
                }
                return current.ss_size.ToInt64();
            }

            LinuxStackT linux = default(LinuxStackT);
            if (sigaltstack_linux(IntPtr.Zero, ref linux) != 0)
            {
                return -1;
            }
            if ((linux.ss_flags & SsDisable) != 0 || linux.ss_sp == IntPtr.Zero)
            {
                return 0;
            }
            return linux.ss_size.ToInt64();
        }

        /// <summary>What one crossing saw about the thread it happened on.</summary>
        private sealed class Crossing
        {
            internal string ThreadName;
            internal bool IsEngineThread;
            internal bool IsThreadPoolThread;
            internal int ManagedThreadId;
            internal long SignalStack;
        }

        /// <summary>
        /// Makes a real ABI call and records the thread it crossed on. The
        /// recording happens inside the <c>Native.Call</c> closure, which is
        /// the frame the P/Invoke runs in.
        /// </summary>
        private static Crossing Cross()
        {
            Crossing seen = new Crossing();
            Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            {
                Thread here = Thread.CurrentThread;
                seen.ThreadName = here.Name;
                seen.IsEngineThread = EngineThreads.IsEngineThread;
                seen.IsThreadPoolThread = here.IsThreadPoolThread;
                seen.ManagedThreadId = here.ManagedThreadId;
                seen.SignalStack = SignalStackSize();
                return Native.folio8_version(out token, out result, out length);
            });
            return seen;
        }

        private static void AssertCrossedOnAnEngineThread(Crossing seen)
        {
            Assert.NotNull(seen.ThreadName);
            Assert.StartsWith(EngineThreads.ThreadNamePrefix, seen.ThreadName, StringComparison.Ordinal);
            Assert.True(seen.IsEngineThread, "the crossing thread is not one the binding owns: " + seen.ThreadName);
            // The defect's own signature. A `.NET TP Worker` must never be
            // the thread inside the engine, whatever its signal stack says.
            Assert.False(seen.IsThreadPoolThread, "a runtime thread-pool thread entered the engine: " + seen.ThreadName);

            if (!IsPosix64())
            {
                // Windows has no POSIX signals, so there is nothing to size —
                // but the call path above is the same one, which is the point
                // of running this test there at all.
                return;
            }
            Assert.Equal(EngineThreads.SignalStackBytes, seen.SignalStack);
        }

        /// <summary>
        /// The acceptance criterion, from the thread kind that crashed:
        /// a runtime thread-pool worker.
        /// </summary>
        [Fact]
        public void ACrossingFromAThreadPoolWorkerRunsOnAnEnlargedEngineThread()
        {
            Crossing seen = null;
            Exception failure = null;
            using (ManualResetEvent done = new ManualResetEvent(false))
            {
                ThreadPool.QueueUserWorkItem(delegate
                {
                    try
                    {
                        Assert.True(Thread.CurrentThread.IsThreadPoolThread);
                        seen = Cross();
                    }
                    catch (Exception error)
                    {
                        failure = error;
                    }
                    finally
                    {
                        done.Set();
                    }
                });
                Assert.True(done.WaitOne(TimeSpan.FromMinutes(2)), "the thread-pool crossing never completed");
            }
            if (failure != null)
            {
                throw new Xunit.Sdk.XunitException("the thread-pool crossing threw: " + failure);
            }
            AssertCrossedOnAnEngineThread(seen);
        }

        /// <summary>The same, from a continuation with no synchronization context.</summary>
        [Fact]
        public async Task ACrossingFromATaskContinuationRunsOnAnEnlargedEngineThread()
        {
            Crossing seen = await Task.Run(() => 1).ContinueWith(
                _ => Cross(),
                CancellationToken.None,
                TaskContinuationOptions.None,
                TaskScheduler.Default);
            AssertCrossedOnAnEngineThread(seen);
        }

        /// <summary>
        /// Every engine thread, not just whichever one happened to pick the
        /// work up. One crossing proves one thread; the pool is only as good
        /// as its least-prepared member.
        /// </summary>
        [Fact]
        public void EveryEngineThreadCarriesTheEnlargedSignalStack()
        {
            // Force the pool to exist before its size is read.
            Cross();
            int pool = EngineThreads.Count;
            Assert.True(pool >= 1);

            // Occupy every engine thread at once, so each of them must serve
            // one of these crossings.
            List<Crossing> seen = new List<Crossing>();
            using (Barrier gathered = new Barrier(pool))
            {
                Crossing[] results = new Crossing[pool];
                Thread[] callers = new Thread[pool];
                Exception[] failures = new Exception[pool];
                for (int i = 0; i < pool; i++)
                {
                    int slot = i;
                    callers[slot] = new Thread(delegate ()
                    {
                        try
                        {
                            results[slot] = CrossHoldingAll(gathered);
                        }
                        catch (Exception error)
                        {
                            failures[slot] = error;
                        }
                    });
                    callers[slot].IsBackground = true;
                    callers[slot].Start();
                }
                for (int i = 0; i < pool; i++)
                {
                    Assert.True(callers[i].Join(TimeSpan.FromMinutes(2)), "an engine thread never released caller " + i.ToString(CultureInfo.InvariantCulture));
                }
                for (int i = 0; i < pool; i++)
                {
                    if (failures[i] != null)
                    {
                        throw new Xunit.Sdk.XunitException("caller " + i.ToString(CultureInfo.InvariantCulture) + " threw: " + failures[i]);
                    }
                    seen.Add(results[i]);
                }
            }

            HashSet<int> distinct = new HashSet<int>();
            foreach (Crossing crossing in seen)
            {
                AssertCrossedOnAnEngineThread(crossing);
                distinct.Add(crossing.ManagedThreadId);
            }
            Assert.Equal(pool, distinct.Count);
        }

        /// <summary>
        /// One crossing that does not return until every other engine thread
        /// has also entered one, so the whole pool is observed.
        /// </summary>
        private static Crossing CrossHoldingAll(Barrier gathered)
        {
            Crossing seen = new Crossing();
            Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            {
                Thread here = Thread.CurrentThread;
                seen.ThreadName = here.Name;
                seen.IsEngineThread = EngineThreads.IsEngineThread;
                seen.IsThreadPoolThread = here.IsThreadPoolThread;
                seen.ManagedThreadId = here.ManagedThreadId;
                seen.SignalStack = SignalStackSize();
                // ASSERTED, NOT DISCARDED. A pool that cannot gather every
                // thread here is a deadlock, and it must say so rather than
                // resurface later as a puzzling count.
                Assert.True(gathered.SignalAndWait(TimeSpan.FromMinutes(2)), "the pool never gathered every engine thread");
                return Native.folio8_version(out token, out result, out length);
            });
            return seen;
        }

        /// <summary>
        /// A crossing submitted from a thread that is already an engine
        /// thread runs INLINE. With the pool's threads all waiting on work
        /// only an engine thread could run, a second queue hop would
        /// deadlock — and <c>Invoke</c> calls <c>EnsureAbi</c>, which crosses
        /// in its own right, so this nesting is the ordinary case and not an
        /// exotic one.
        /// </summary>
        [Fact]
        public void ACrossingSubmittedFromAnEngineThreadRunsInline()
        {
            int outerId = 0;
            int innerId = 0;
            bool innerWasEngineThread = false;

            Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            {
                outerId = Thread.CurrentThread.ManagedThreadId;
                Crossing nested = Cross();
                innerId = nested.ManagedThreadId;
                innerWasEngineThread = nested.IsEngineThread;
                return Native.folio8_version(out token, out result, out length);
            });

            Assert.True(innerWasEngineThread);
            Assert.Equal(outerId, innerId);
        }

        /// <summary>
        /// A render and the <c>folio8_free</c> that returns its buffer run on
        /// ONE engine thread, and neither runs on the caller's.
        /// </summary>
        /// <remarks>
        /// <c>Invoke</c> moves as a unit — the export, the copy into managed
        /// memory and the free are one method body on one engine thread — so
        /// that is the property worth asserting, and it is what lets a caller
        /// render on one of its own threads and read the result on another
        /// with no new restriction.
        /// <para>
        /// The caller-side split the matrix describes is not reachable from
        /// outside this assembly, and deliberately so: there is no public
        /// dispose, and the allocation token never escapes <c>InvokeHere</c>'s
        /// frame. A consumer therefore CANNOT strand a free on a thread of its
        /// own, which is the contract "dispose on the thread you rendered on"
        /// would have imposed. What is observable is that the engine's
        /// allocation table returns to its prior value, read from a third
        /// thread.
        /// </para>
        /// </remarks>
        [Fact]
        public void ARenderAndItsFreeRunOnOneEngineThread()
        {
            // Warm the lazily-loaded face set first, so its cost is not read
            // as a stranded buffer.
            RenderOnce();
            int before = Folio8.OutstandingNativeAllocations();

            int crossedOn = 0;
            int servedOn = 0;
            Exception failure = null;
            Thread renderer = new Thread(delegate ()
            {
                try
                {
                    crossedOn = RenderOnce();
                    servedOn = EngineThreads.LastServedOn;
                }
                catch (Exception error)
                {
                    failure = error;
                }
            });
            renderer.IsBackground = true;
            renderer.Start();
            Assert.True(renderer.Join(TimeSpan.FromMinutes(2)), "the render never completed");
            if (failure != null)
            {
                throw new Xunit.Sdk.XunitException("the render threw: " + failure);
            }

            // The export ran on the thread that served the whole unit, so the
            // free at the tail of that same body ran there too. Split Invoke
            // and this reddens.
            Assert.Equal(servedOn, crossedOn);
            Assert.NotEqual(renderer.ManagedThreadId, crossedOn);
            Assert.NotEqual(Thread.CurrentThread.ManagedThreadId, crossedOn);

            int checkedOn = 0;
            Thread reader = new Thread(delegate ()
            {
                checkedOn = Folio8.OutstandingNativeAllocations();
            });
            reader.IsBackground = true;
            reader.Start();
            Assert.True(reader.Join(TimeSpan.FromMinutes(2)), "the allocation count never came back");

            Assert.Equal(before, checkedOn);
            Assert.Equal(before, Folio8.OutstandingNativeAllocations());
        }

        /// <summary>
        /// The allocation count is a crossing like any other, and the hop is
        /// asserted rather than assumed.
        /// </summary>
        /// <remarks>
        /// This is the one entry point that used to bypass the funnel
        /// entirely, and the returned integer looks exactly the same whether
        /// it crossed on an engine thread or on the caller's — so the count of
        /// crossings the pool has served is what makes the routing
        /// observable. Delete the hop and this reddens; nothing else would.
        /// </remarks>
        [Fact]
        public void TheAllocationCountCrossesOnAnEngineThreadToo()
        {
            // Warm the pool, so what is measured below is the hop and not a
            // first-use start.
            RenderOnce();

            int callerId = 0;
            long servedBefore = 0;
            long servedAfter = 0;
            int servedOn = 0;
            Thread caller = new Thread(delegate ()
            {
                callerId = Thread.CurrentThread.ManagedThreadId;
                servedBefore = EngineThreads.Served;
                Folio8.OutstandingNativeAllocations();
                servedAfter = EngineThreads.Served;
                servedOn = EngineThreads.LastServedOn;
            });
            caller.IsBackground = true;
            caller.Start();
            Assert.True(caller.Join(TimeSpan.FromMinutes(2)), "the allocation count never came back");

            Assert.Equal(servedBefore + 1, servedAfter);
            Assert.NotEqual(callerId, servedOn);
        }

        /// <summary>
        /// An engine thread that cannot enlarge its alternate signal stack
        /// fails loudly and PERMANENTLY, and serves nothing in the meantime.
        /// </summary>
        /// <remarks>
        /// This is the matrix's "sigaltstack enlargement fails" row, and it
        /// is the path that disables the library, so it is exercised rather
        /// than reasoned about. It cannot be provoked on a healthy host — a
        /// megabyte is always available and the call always succeeds — so the
        /// binding carries an internal seam for it, which also restores the
        /// pool that was already running.
        /// </remarks>
        [Fact]
        public void AThreadThatCannotEnlargeItsSignalStackDisablesTheLibrary()
        {
            // A healthy pool first, so there is one to restore.
            AssertCrossedOnAnEngineThread(Cross());
            long servedBefore = EngineThreads.Served;

            EngineThreads.InduceStartFailureForTesting();
            try
            {
                InvalidOperationException first = Assert.Throws<InvalidOperationException>(() => Cross());
                Assert.Contains("could not enlarge its alternate signal stack", first.Message, StringComparison.Ordinal);
                Assert.Contains("must not be entered on a thread whose signal stack is the runtime's small default", first.Message, StringComparison.Ordinal);

                // STICKY. The second attempt refuses identically rather than
                // retrying into the same failure — and a different entry
                // point refuses too, not just the one that tripped it.
                InvalidOperationException second = Assert.Throws<InvalidOperationException>(
                    () => Folio8.OutstandingNativeAllocations());
                Assert.Equal(first.Message, second.Message);

                // AND NOTHING WAS SERVED. "Fail, do not degrade": no call may
                // slip through onto a thread that is on the small stack.
                Assert.Equal(servedBefore, EngineThreads.Served);
            }
            finally
            {
                EngineThreads.EndInducedStartFailureForTesting();
            }

            // The pool that was running before is serving again.
            AssertCrossedOnAnEngineThread(Cross());
        }

        /// <summary>
        /// More concurrent callers than the pool has threads: every call
        /// completes, nothing deadlocks, and the pool does not grow to meet
        /// them.
        /// </summary>
        [Fact]
        public void MoreCallersThanEngineThreadsAllComplete()
        {
            RenderOnce();
            int pool = EngineThreads.Count;
            int callers = (pool * 4) + 1;
            int before = Folio8.OutstandingNativeAllocations();

            Thread[] threads = new Thread[callers];
            Exception[] failures = new Exception[callers];
            int[] crossedOn = new int[callers];
            using (Barrier start = new Barrier(callers))
            {
                for (int i = 0; i < callers; i++)
                {
                    int slot = i;
                    threads[slot] = new Thread(delegate ()
                    {
                        try
                        {
                            // Everyone arrives at once, so the queue really is
                            // contended rather than drained as it fills.
                            Assert.True(start.SignalAndWait(TimeSpan.FromMinutes(2)), "the callers never all arrived");
                            crossedOn[slot] = RenderOnce();
                        }
                        catch (Exception error)
                        {
                            failures[slot] = error;
                        }
                    });
                    threads[slot].IsBackground = true;
                    threads[slot].Start();
                }
                for (int i = 0; i < callers; i++)
                {
                    Assert.True(threads[i].Join(TimeSpan.FromMinutes(5)),
                        "caller " + i.ToString(CultureInfo.InvariantCulture) + " of " + callers.ToString(CultureInfo.InvariantCulture) + " never completed — the pool deadlocked");
                }
            }

            for (int i = 0; i < callers; i++)
            {
                if (failures[i] != null)
                {
                    throw new Xunit.Sdk.XunitException("caller " + i.ToString(CultureInfo.InvariantCulture) + " threw: " + failures[i]);
                }
            }

            HashSet<int> engines = new HashSet<int>();
            for (int i = 0; i < callers; i++)
            {
                engines.Add(crossedOn[i]);
            }
            // Bounded, not merely finite: the pool does not spawn a thread
            // per caller under pressure.
            Assert.True(engines.Count <= pool, "the crossings used " + engines.Count.ToString(CultureInfo.InvariantCulture) + " threads for a pool of " + pool.ToString(CultureInfo.InvariantCulture));
            Assert.Equal(before, Folio8.OutstandingNativeAllocations());
        }

        /// <summary>
        /// A native error surfaces on the CALLER's thread as the same
        /// exception, with the stack it was thrown with — the hop is not
        /// allowed to become a wrapper or to lose the frames.
        /// </summary>
        [Fact]
        public void ANativeErrorArrivesOnTheCallersThreadWithItsStackIntact()
        {
            FolioRenderException thrown = Assert.Throws<FolioRenderException>(
                () => Template.Parse(System.Text.Encoding.UTF8.GetBytes("{")));
            Assert.NotNull(thrown.Diagnostic);
            Assert.NotNull(thrown.StackTrace);
            // The frames of the thread that actually threw are preserved
            // rather than replaced by the caller's rethrow site.
            // InvokeHere is the frame the engine thread threw from. It
            // survives the hop only because ExceptionDispatchInfo carried it;
            // a plain rethrow on the caller's thread would have replaced it.
            Assert.Contains("InvokeHere", thrown.StackTrace, StringComparison.Ordinal);
        }

        /// <summary>
        /// One real render, returning the managed id of the thread the
        /// crossing happened on.
        /// </summary>
        private static int RenderOnce()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "colour-strokes", "input.folio"));
            Data data = new Data(Repo.File_("fixtures", "colour-strokes", "data.json"));
            int crossedOn = 0;
            Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            {
                crossedOn = Thread.CurrentThread.ManagedThreadId;
                byte[] tpl = template.CanonicalBytes;
                byte[] payload = data.Bytes;
                byte[] fonts = Native.EncodeFonts(Repo.ShippedFonts);
                return Native.folio8_render(
                    tpl, tpl.Length,
                    payload, payload.Length,
                    null, 0,
                    fonts, fonts.Length,
                    0,
                    out token, out result, out length);
            });
            return crossedOn;
        }
    }
}
