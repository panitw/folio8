using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Threading;

namespace RenderThroughput
{
    /// <summary>
    /// CAP-10's measurement: how many renders a second does this build of the
    /// binding sustain, at one caller and at many?
    /// </summary>
    /// <remarks>
    /// <para>
    /// <b>It measures a build; it does not choose one.</b> Which binding is
    /// linked is decided by <c>FolioBindingProject</c> at build time, so the
    /// before/after comparison is two builds of these same sources rather than
    /// a switch inside the shipped binding. That is deliberate: a bypass flag
    /// in <c>src/Folio8</c> would be surface that exists only for a benchmark.
    /// </para>
    /// <para>
    /// <b>It reports; it does not judge.</b> There is no threshold here and no
    /// exit code that means "too slow". The one thing it does refuse is to
    /// report numbers for a render it cannot verify: the fixture is rendered
    /// once and its SHA-256 checked against the committed
    /// <c>expected.json</c> before any timing starts, because a fast wrong
    /// render is not a measurement of anything.
    /// </para>
    /// <para>
    /// <b>A developer machine is indicative, not authoritative</b>, and the
    /// provenance block says so in the output rather than leaving the reader
    /// to remember it. The same discipline story 1 established for the
    /// signal-stack readings: record the host, its architecture, and whether
    /// it was translated, so a number can never be quoted without them.
    /// </para>
    /// </remarks>
    internal static class Program
    {
        /// <summary>
        /// The fixture rendered when none is named. It is named, and not
        /// "some representative document", because a benchmark whose input is
        /// unnamed is not reproducible.
        /// </summary>
        /// <remarks>
        /// <c>multi-page-statement</c> is the corpus's realistic reporting
        /// workload: a two-page design over 130 synthetic transactions,
        /// flowing to four output pages with a page header and footer on each.
        /// It is big enough that per-render work dominates process noise and
        /// small enough that four concurrency levels finish in a coffee break
        /// — and it is the shape the .NET consumers this epic is about
        /// actually render.
        /// </remarks>
        private const string DefaultFixture = "multi-page-statement";

        private const double DefaultSeconds = 5.0;
        private const double DefaultWarmupSeconds = 1.0;

        /// <summary>The eleven faces the engine ships, as folio-go/fonts names them.</summary>
        /// <remarks>
        /// Read from <c>folio-go/fonts/</c> at run time rather than from the
        /// assembly's embedded set, for one reason: the baseline leg links a
        /// DIFFERENT build of Folio8.dll, and loading fonts from disk means
        /// both legs are handed byte-identical face bytes from one place
        /// instead of each from its own worktree's embedding.
        /// </remarks>
        private static readonly string[][] ShippedFaceFiles =
        {
            new[] { "Noto Sans", "notosans", "NotoSans-Regular.ttf" },
            new[] { "Noto Sans Bold", "notosans-bold", "NotoSans-Bold.ttf" },
            new[] { "Noto Sans Italic", "notosans-italic", "NotoSans-Italic.ttf" },
            new[] { "Noto Sans Bold Italic", "notosans-bolditalic", "NotoSans-BoldItalic.ttf" },
            new[] { "Noto Sans Thai", "notosansthai", "NotoSansThai-Regular.ttf" },
            new[] { "Noto Sans Thai Bold", "notosansthai-bold", "NotoSansThai-Bold.ttf" },
            new[] { "Noto Sans SC", "notosanssc", "NotoSansSC-Regular.ttf" },
            new[] { "Roboto", "roboto", "Roboto-Regular.ttf" },
            new[] { "Roboto Bold", "roboto-bold", "Roboto-Bold.ttf" },
            new[] { "Roboto Italic", "roboto-italic", "Roboto-Italic.ttf" },
            new[] { "Roboto Bold Italic", "roboto-bolditalic", "Roboto-BoldItalic.ttf" },
        };

        private const string Usage =
@"render-throughput — CAP-10's render throughput harness (see Program.cs for why)

  render-throughput [options]

  --repo <dir>          repository root (default: found by walking up from this binary)
  --fixture <slug>      fixtures/<slug> to render (default: " + DefaultFixture + @")
  --seconds <n>         timed seconds per concurrency level (default: 5)
  --warmup <n>          discarded seconds per concurrency level (default: 1)
  --levels <a,b,c>      concurrency levels (default: 1,2,4,ProcessorCount)
  --label <text>        names this run in the output, e.g. 'before' or 'after'
  --pool <n|none>       engine threads the linked binding holds, for the table's
                        oversubscription annotation only (default:
                        max(ProcessorCount,2); pass 'none' for a binding with
                        no pool, such as the pre-engine-thread baseline)
  --host-context <text> provenance the program cannot see from inside itself,
                        e.g. 'linux/amd64 container on darwin/arm64 — TRANSLATED'
  --help

It prints a table and one RESULT<TAB>-prefixed line per level for a wrapper to
join. It has no pass/fail: a human reads the numbers.";

        private static int Main(string[] args)
        {
            Options options;
            string failure;
            // VALIDATE THE WHOLE ARGUMENT LIST BEFORE ANY WORK. Nothing is
            // loaded, no font is read and no render is attempted until every
            // argument is known good — a harness that renders for six seconds
            // and then complains about --secondss has wasted the run it was
            // asked for.
            if (!Options.TryParse(args, out options, out failure))
            {
                if (failure == null)
                {
                    Console.WriteLine(Usage);
                    return 0;
                }
                Console.Error.WriteLine("render-throughput: " + failure);
                return 2;
            }

            try
            {
                return Run(options);
            }
            catch (Exception error)
            {
                Console.Error.WriteLine("render-throughput: " + error.GetType().Name + ": " + error.Message);
                return 1;
            }
        }

        private static int Run(Options options)
        {
            string fixtureDir = Path.Combine(Path.Combine(options.Repo, "fixtures"), options.Fixture);
            if (!Directory.Exists(fixtureDir))
            {
                Console.Error.WriteLine("render-throughput: no fixture at " + fixtureDir +
                    "; pass --fixture <slug> naming a directory under fixtures/");
                return 2;
            }

            string templatePath = Path.Combine(fixtureDir, "input.folio");
            if (!File.Exists(templatePath))
            {
                Console.Error.WriteLine("render-throughput: " + templatePath + " does not exist, so fixtures/" +
                    options.Fixture + " is not a renderable fixture");
                return 2;
            }

            byte[] templateBytes = File.ReadAllBytes(templatePath);
            string dataPath = Path.Combine(fixtureDir, "data.json");
            byte[] dataBytes = File.Exists(dataPath)
                ? File.ReadAllBytes(dataPath)
                : Encoding.UTF8.GetBytes("{}");
            string paramsPath = Path.Combine(fixtureDir, "params.json");
            byte[] paramsBytes = File.Exists(paramsPath) ? File.ReadAllBytes(paramsPath) : null;

            FontSet fonts = LoadShippedFaces(options.Repo);
            Template template = Template.Parse(templateBytes);
            Data data = new Data(dataBytes);
            Params parameters = paramsBytes == null ? null : new Params(paramsBytes);

            // THE CORRECTNESS CHECK COMES FIRST, AND IT IS NOT DECORATION. A
            // build of the binding that renders the wrong bytes quickly would
            // otherwise be reported as a throughput win. The expected digest is
            // READ from the fixture's own expected.json, never restated here.
            RenderResult first = Folio8.Render(template, data, parameters, fonts);
            string expected = ExpectedSha256(fixtureDir);
            string actual = Sha256(first.Bytes);
            if (expected != null && expected != actual)
            {
                Console.Error.WriteLine("render-throughput: fixtures/" + options.Fixture +
                    " rendered " + actual + " but expected.json records " + expected +
                    ". Refusing to report throughput for a render that is not the committed one.");
                return 1;
            }

            WriteProvenance(options, fixtureDir, templateBytes, dataBytes, first, expected == null);

            List<Level> results = new List<Level>();
            int verifiedRenders = 0;
            foreach (int concurrency in options.Levels)
            {
                // THE WARMUP IS ALSO THE CONCURRENT CORRECTNESS CHECK, and the
                // pairing is the point. The single-threaded render above proves
                // the binding renders the committed bytes; it says nothing about
                // what happens when N callers cross the ABI at once, which is
                // precisely the failure mode moving the crossing onto a shared
                // pool could introduce. So every worker at every level hashes
                // its FIRST render and compares — and it happens in the warmup,
                // outside the timed window, so the timed figure stays a pure
                // throughput number with no digest arithmetic in it.
                verifiedRenders += Measure(
                    template, data, parameters, fonts, concurrency, options.WarmupSeconds, expected).Verified;
                results.Add(Measure(template, data, parameters, fonts, concurrency, options.Seconds, null));
            }

            WriteTable(options, results, verifiedRenders);
            return 0;
        }

        /// <summary>
        /// One concurrency level: <paramref name="concurrency"/> dedicated
        /// threads rendering in a loop until the deadline.
        /// </summary>
        /// <remarks>
        /// <b>Dedicated threads, not the thread pool.</b> The level has to BE
        /// the number asked for; the CLR's pool grows on its own schedule and
        /// would make "four concurrent callers" mean whatever it decided. It
        /// also keeps the harness honest about what it is measuring: ordinary
        /// caller threads entering the binding, which is what a consumer does.
        /// <para>
        /// Every worker is started and parked on a gate before any of them
        /// renders, so thread creation is outside the timed window and all
        /// workers are running for the whole of it.
        /// </para>
        /// </remarks>
        private static Level Measure(
            Template template, Data data, Params parameters, FontSet fonts,
            int concurrency, double seconds, string verifySha256)
        {
            ManualResetEventSlim go = new ManualResetEventSlim(false);
            CountdownEvent ready = new CountdownEvent(concurrency);
            long[] counts = new long[concurrency];
            double[] latencySums = new double[concurrency];
            List<double>[] latencies = new List<double>[concurrency];
            Exception failure = null;
            Stopwatch clock = new Stopwatch();
            long deadlineTicks = (long)(seconds * Stopwatch.Frequency);
            int verified = 0;

            Thread[] workers = new Thread[concurrency];
            for (int i = 0; i < concurrency; i++)
            {
                int index = i;
                latencies[index] = new List<double>();
                Thread worker = new Thread(delegate ()
                {
                    ready.Signal();
                    go.Wait();
                    try
                    {
                        // NOT A PLAIN while. When a digest is being verified
                        // this worker renders AT LEAST once, even at
                        // --warmup 0, because "every caller hashed one render"
                        // must be true of every run rather than of runs that
                        // happened to have a warmup window.
                        bool pending = verifySha256 != null;
                        while (pending || clock.ElapsedTicks < deadlineTicks)
                        {
                            long started = clock.ElapsedTicks;
                            RenderResult result = Folio8.Render(template, data, parameters, fonts);
                            long finished = clock.ElapsedTicks;
                            // Touch the bytes so nothing about the render can
                            // be elided as unobserved work.
                            if (result.Bytes.Length == 0)
                            {
                                throw new InvalidOperationException("the engine returned an empty PDF");
                            }
                            if (pending)
                            {
                                string digest = Sha256(result.Bytes);
                                if (digest != verifySha256)
                                {
                                    throw new InvalidOperationException(
                                        "this caller's render is " + digest + ", but expected.json records " +
                                        verifySha256 + " — the binding renders correctly alone and NOT under " +
                                        concurrency.ToString(CultureInfo.InvariantCulture) + " concurrent callers");
                                }
                                Interlocked.Increment(ref verified);
                                pending = false;
                            }
                            double ms = (finished - started) * 1000.0 / Stopwatch.Frequency;
                            counts[index]++;
                            latencySums[index] += ms;
                            latencies[index].Add(ms);
                        }
                    }
                    catch (Exception error)
                    {
                        Interlocked.CompareExchange(ref failure, error, null);
                    }
                });
                worker.IsBackground = false;
                worker.Name = "throughput-caller-" + index.ToString(CultureInfo.InvariantCulture);
                workers[index] = worker;
                worker.Start();
            }

            ready.Wait();
            clock.Start();
            go.Set();
            foreach (Thread worker in workers)
            {
                worker.Join();
            }
            clock.Stop();

            if (failure != null)
            {
                throw new InvalidOperationException(
                    "a render failed at concurrency " + concurrency.ToString(CultureInfo.InvariantCulture) +
                    ": " + failure.Message, failure);
            }

            long renders = 0;
            double latencyTotal = 0;
            List<double> all = new List<double>();
            for (int i = 0; i < concurrency; i++)
            {
                renders += counts[i];
                latencyTotal += latencySums[i];
                all.AddRange(latencies[i]);
            }
            all.Sort();

            Level level = new Level();
            level.Verified = verified;
            level.Concurrency = concurrency;
            level.Renders = renders;
            level.Seconds = clock.Elapsed.TotalSeconds;
            level.MeanLatencyMs = renders == 0 ? 0 : latencyTotal / renders;
            level.MedianLatencyMs = all.Count == 0 ? 0 : all[all.Count / 2];
            level.P95LatencyMs = all.Count == 0 ? 0 : all[(int)(all.Count * 0.95) >= all.Count ? all.Count - 1 : (int)(all.Count * 0.95)];
            return level;
        }

        private sealed class Level
        {
            internal int Concurrency;
            internal int Verified;
            internal long Renders;
            internal double Seconds;
            internal double MeanLatencyMs;
            internal double MedianLatencyMs;
            internal double P95LatencyMs;

            internal double RendersPerSecond
            {
                get { return Seconds <= 0 ? 0 : Renders / Seconds; }
            }
        }

        private static void WriteProvenance(
            Options options, string fixtureDir, byte[] templateBytes, byte[] dataBytes,
            RenderResult first, bool unverified)
        {
            string label = string.IsNullOrEmpty(options.Label) ? "(unlabelled)" : options.Label;
            Console.WriteLine("=== render-throughput: " + label + " ===");
            Console.WriteLine("  taken            " + DateTime.UtcNow.ToString("yyyy-MM-dd HH:mm:ss'Z'", CultureInfo.InvariantCulture));
            Console.WriteLine("  os               " + RuntimeInformation.OSDescription.Replace("\n", " ").Trim());
            Console.WriteLine("  os arch          " + RuntimeInformation.OSArchitecture);
            Console.WriteLine("  process arch     " + RuntimeInformation.ProcessArchitecture);
            Console.WriteLine("  processors       " + Environment.ProcessorCount);
            Console.WriteLine("  runtime          " + RuntimeInformation.FrameworkDescription);
            Console.WriteLine("  server gc        " + System.Runtime.GCSettings.IsServerGC);
            Console.WriteLine("  cpu              " + DescribeCpu());
            Console.WriteLine("  translated       " + DescribeTranslation());
            if (!string.IsNullOrEmpty(options.HostContext))
            {
                Console.WriteLine("  host context     " + options.HostContext);
            }
            Console.WriteLine("  engine           " + Folio8.Version);
            Console.WriteLine("  fixture          fixtures/" + options.Fixture);
            Console.WriteLine("    input.folio    " + templateBytes.Length + " bytes, sha256 " + Sha256(templateBytes).Substring(0, 16));
            Console.WriteLine("    data.json      " + dataBytes.Length + " bytes, sha256 " + Sha256(dataBytes).Substring(0, 16));
            Console.WriteLine("    render         " + first.Bytes.Length + " bytes, sha256 " + Sha256(first.Bytes).Substring(0, 16) +
                (unverified ? "  (NO expected.json — UNVERIFIED)" : "  (matches expected.json)"));
            Console.WriteLine("  window           " + options.Seconds.ToString("0.#", CultureInfo.InvariantCulture) +
                "s timed, " + options.WarmupSeconds.ToString("0.#", CultureInfo.InvariantCulture) + "s warmup, per level");
            Console.WriteLine();
            // A MEASUREMENT WITHOUT ITS WORTH STATED GETS QUOTED WITHOUT IT.
            Console.WriteLine("  A developer machine is INDICATIVE. It is not a controlled environment, and a");
            Console.WriteLine("  translated host measures the translator as much as the binding. Quote these");
            Console.WriteLine("  numbers only with the four lines above them.");
            Console.WriteLine();
        }

        private static void WriteTable(Options options, List<Level> results, int verifiedRenders)
        {
            // ⚠ A RESTATEMENT OF EngineThreads.Start()'s SIZING RULE, AND IT
            // CAN DRIFT FROM IT. EngineThreads is internal to the binding and
            // this harness is not allowed a seam into it, so the rule
            // — Environment.ProcessorCount, floored at 2 — is copied here
            // rather than read. If that rule ever changes in src/Folio8, this
            // annotation becomes quietly wrong and nothing will say so; it is
            // an annotation on a table a human reads, not an input to any
            // number, which is the only reason a restatement is tolerable at
            // all.
            //
            // IT IS ALSO A PROPERTY OF ONE LEG ONLY. The pre-engine-thread
            // binding has no pool — every caller crossed on its own thread —
            // so --pool none suppresses the annotation entirely rather than
            // describing the baseline leg in terms of a mechanism it does not
            // have.
            int pool = options.PoolSize;

            Console.WriteLine("  concurrency   renders   renders/s   mean ms   median ms   p95 ms   vs c=1");
            Console.WriteLine("  -----------   -------   ---------   -------   ---------   ------   ------");
            double baseline = results.Count == 0 ? 0 : results[0].RendersPerSecond;
            foreach (Level level in results)
            {
                string scale = baseline <= 0
                    ? "  n/a"
                    : (level.RendersPerSecond / baseline).ToString("0.00", CultureInfo.InvariantCulture) + "x";
                Console.WriteLine("  " +
                    Pad(level.Concurrency.ToString(CultureInfo.InvariantCulture) +
                        (pool > 0 && level.Concurrency > pool ? "*" : ""), 11) + "   " +
                    Pad(level.Renders.ToString(CultureInfo.InvariantCulture), 7) + "   " +
                    Pad(level.RendersPerSecond.ToString("0.00", CultureInfo.InvariantCulture), 9) + "   " +
                    Pad(level.MeanLatencyMs.ToString("0.00", CultureInfo.InvariantCulture), 7) + "   " +
                    Pad(level.MedianLatencyMs.ToString("0.00", CultureInfo.InvariantCulture), 9) + "   " +
                    Pad(level.P95LatencyMs.ToString("0.00", CultureInfo.InvariantCulture), 6) + "   " +
                    scale);
            }
            Console.WriteLine();
            if (pool > 0)
            {
                Console.WriteLine("  * more callers than this binding's engine pool holds (max(ProcessorCount,2) = " +
                    pool.ToString(CultureInfo.InvariantCulture) + ", restated from EngineThreads — see the comment there).");
            }
            else
            {
                Console.WriteLine("  This binding has no engine pool (--pool none), so no level is annotated as");
                Console.WriteLine("  oversubscribing one: every caller crosses the ABI on its own thread.");
            }
            Console.WriteLine("  Correctness: " + verifiedRenders.ToString(CultureInfo.InvariantCulture) +
                " renders — one per caller per level, in each level's warmup — hashed and matched" +
                Environment.NewLine +
                "  against expected.json, so a binding that corrupted only under concurrent crossings" +
                Environment.NewLine +
                "  could not be counted here as throughput.");
            // SAY IT WHEN THE LEVEL SET COLLAPSED. The default is 1, 2, 4 and
            // ProcessorCount, deduplicated — so a 1-, 2- or 4-core host gets
            // FEWER than the four levels the acceptance criterion names, and a
            // reader comparing two tables from two machines would otherwise
            // have to work out for themselves why one has three rows.
            if (options.LevelsAreDefault && results.Count < 4)
            {
                Console.WriteLine("  Note: the default level set is 1, 2, 4 and ProcessorCount (" +
                    Environment.ProcessorCount.ToString(CultureInfo.InvariantCulture) + " here), which");
                Console.WriteLine("  DEDUPLICATES to " + results.Count.ToString(CultureInfo.InvariantCulture) +
                    " levels on this host rather than four. Pass --levels to widen it.");
            }
            Console.WriteLine();

            // The machine-readable arm, for measure-throughput.sh to join the
            // two legs on. Tab-separated and prefixed, so it survives being
            // grepped out of a build log.
            foreach (Level level in results)
            {
                Console.WriteLine(string.Join("\t", new[]
                {
                    "RESULT",
                    string.IsNullOrEmpty(options.Label) ? "-" : options.Label,
                    level.Concurrency.ToString(CultureInfo.InvariantCulture),
                    level.Renders.ToString(CultureInfo.InvariantCulture),
                    level.Seconds.ToString("0.000", CultureInfo.InvariantCulture),
                    level.RendersPerSecond.ToString("0.000", CultureInfo.InvariantCulture),
                    level.MeanLatencyMs.ToString("0.000", CultureInfo.InvariantCulture),
                    level.MedianLatencyMs.ToString("0.000", CultureInfo.InvariantCulture),
                    level.P95LatencyMs.ToString("0.000", CultureInfo.InvariantCulture),
                }));
            }
        }

        private static string Pad(string value, int width)
        {
            return value.Length >= width ? value : value.PadLeft(width);
        }

        /// <summary>
        /// Whether this process is being translated, where that is knowable
        /// from inside it.
        /// </summary>
        /// <remarks>
        /// <b>It says "unknown" rather than "no" when it cannot tell</b>, which
        /// is the whole point. macOS answers honestly through
        /// <c>sysctl.proc_translated</c>. A Linux process cannot see a Rosetta
        /// or qemu layer beneath its container from inside — <c>uname</c>
        /// reports the emulated architecture by design — so it says so and
        /// leaves the claim to the <c>--host-context</c> the wrapper stamps,
        /// which knows what platform it asked docker for and what the host is.
        /// This is the qemu trap DW-396 fell into once already; guessing "no"
        /// here would be how it happens a third time.
        /// </remarks>
        private static string DescribeTranslation()
        {
            if (RuntimeInformation.IsOSPlatform(OSPlatform.OSX))
            {
                try
                {
                    int translated = 0;
                    IntPtr size = new IntPtr(sizeof(int));
                    if (sysctlbyname("sysctl.proc_translated", out translated, ref size, IntPtr.Zero, IntPtr.Zero) == 0)
                    {
                        return translated != 0
                            ? "YES — Rosetta (sysctl.proc_translated=1); NOT real-silicon evidence"
                            : "no — sysctl.proc_translated=0";
                    }
                }
                catch (Exception)
                {
                    // Fall through to unknown: a failed sysctl is not a "no".
                }
                return "unknown — sysctl.proc_translated could not be read";
            }
            if (RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
            {
                // THE SIGNAL THAT SURVIVES TRANSLATION IS vendor_id, and that
                // is story 1's measured finding rather than a guess here:
                // inside a Docker Desktop linux/amd64 container on Apple
                // Silicon, uname says x86_64, /run/rosetta is absent and
                // binfmt_misc is empty — but vendor_id reads VirtualApple
                // against GenuineIntel / AuthenticAMD on real silicon.
                //
                // Only the one conclusive reading is claimed here. The full
                // four-signal derivation belongs to probe-signal-stack.sh,
                // which owns that measurement; a second, weaker copy of it
                // inside a benchmark could only disagree with the first.
                string vendor = CpuInfoField("vendor_id");
                if (vendor != null && vendor.IndexOf("VirtualApple", StringComparison.OrdinalIgnoreCase) >= 0)
                {
                    return "YES — /proc/cpuinfo vendor_id is '" + vendor +
                           "'; NOT admissible as evidence about real silicon";
                }
                return "not established from inside" +
                       (vendor == null ? "" : " — /proc/cpuinfo vendor_id is '" + vendor + "'") +
                       "; uname reports the EMULATED arch, so see 'host context', " +
                       "and run probe-signal-stack.sh for the full derivation";
            }
            return "n/a";
        }

        [DllImport("libc", EntryPoint = "sysctlbyname", SetLastError = true)]
        private static extern int sysctlbyname(string name, out int value, ref IntPtr size, IntPtr newp, IntPtr newlen);

        /// <summary>The CPU as the host names it, raw and unjudged.</summary>
        private static string DescribeCpu()
        {
            string[] wanted = { "model name", "Model name", "Hardware", "cpu model", "Processor" };
            foreach (string key in wanted)
            {
                string value = CpuInfoField(key);
                if (value != null)
                {
                    return value;
                }
            }
            return File.Exists("/proc/cpuinfo")
                ? "(no model line in /proc/cpuinfo)"
                : "(not reported by this OS)";
        }

        /// <summary>One <c>key : value</c> field of <c>/proc/cpuinfo</c>, or null.</summary>
        private static string CpuInfoField(string key)
        {
            try
            {
                if (!File.Exists("/proc/cpuinfo"))
                {
                    return null;
                }
                foreach (string line in File.ReadAllLines("/proc/cpuinfo"))
                {
                    int colon = line.IndexOf(':');
                    if (colon > 0 && line.Substring(0, colon).Trim() == key)
                    {
                        return line.Substring(colon + 1).Trim();
                    }
                }
            }
            catch (Exception)
            {
                // Provenance, not a measurement. Not knowing it must not stop
                // the run — but it must not be reported as a "no", either,
                // which is why this returns null rather than a verdict.
            }
            return null;
        }

        private static FontSet LoadShippedFaces(string repo)
        {
            FontSet fonts = new FontSet();
            foreach (string[] face in ShippedFaceFiles)
            {
                string path = Path.Combine(Path.Combine(Path.Combine(Path.Combine(repo, "folio-go"), "fonts"), face[1]), face[2]);
                if (!File.Exists(path))
                {
                    throw new FileNotFoundException(
                        "the shipped face '" + face[0] + "' is not at " + path +
                        "; pass --repo <dir> naming the folio8 repository root", path);
                }
                fonts.Add(face[0], File.ReadAllBytes(path));
            }
            return fonts;
        }

        /// <summary>
        /// The committed digest for this fixture, read from its own
        /// <c>expected.json</c>. Null when the fixture records none — a shape
        /// the harness reports rather than assumes away.
        /// </summary>
        /// <remarks>
        /// Parsed by hand rather than with System.Text.Json for no reason
        /// other than that it is one field of one flat object, and the file is
        /// generated by Go rather than hand-edited.
        /// </remarks>
        private static string ExpectedSha256(string fixtureDir)
        {
            string path = Path.Combine(fixtureDir, "expected.json");
            if (!File.Exists(path))
            {
                return null;
            }
            string text = File.ReadAllText(path);
            int at = text.IndexOf("\"sha256\"", StringComparison.Ordinal);
            if (at < 0)
            {
                return null;
            }
            int open = text.IndexOf('"', text.IndexOf(':', at) + 1);
            int close = open < 0 ? -1 : text.IndexOf('"', open + 1);
            return close < 0 ? null : text.Substring(open + 1, close - open - 1);
        }

        private static string Sha256(byte[] bytes)
        {
            using (SHA256 hash = SHA256.Create())
            {
                byte[] digest = hash.ComputeHash(bytes);
                StringBuilder hex = new StringBuilder(digest.Length * 2);
                foreach (byte b in digest)
                {
                    hex.Append(b.ToString("x2", CultureInfo.InvariantCulture));
                }
                return hex.ToString();
            }
        }

        private sealed class Options
        {
            internal string Repo;
            internal string Fixture = DefaultFixture;
            internal double Seconds = DefaultSeconds;
            internal double WarmupSeconds = DefaultWarmupSeconds;
            internal int[] Levels;
            internal bool LevelsAreDefault;
            internal string Label;
            internal string HostContext;

            /// <summary>
            /// How many engine threads the linked binding holds, or 0 for a
            /// binding that has no pool. Annotation only — it feeds no number.
            /// </summary>
            internal int PoolSize = Environment.ProcessorCount < 2 ? 2 : Environment.ProcessorCount;

            /// <summary>
            /// Parses the whole argument list, or explains the first thing
            /// wrong with it. <c>--help</c> returns false with a null failure.
            /// </summary>
            internal static bool TryParse(string[] args, out Options options, out string failure)
            {
                options = new Options();
                failure = null;
                List<int> levels = null;

                for (int i = 0; i < args.Length; i++)
                {
                    string argument = args[i];
                    if (argument == "--help" || argument == "-h")
                    {
                        options = null;
                        return false;
                    }

                    string value = null;
                    if (i + 1 < args.Length)
                    {
                        value = args[i + 1];
                    }

                    switch (argument)
                    {
                        case "--repo":
                        case "--fixture":
                        case "--seconds":
                        case "--warmup":
                        case "--levels":
                        case "--label":
                        case "--pool":
                        case "--host-context":
                            if (value == null)
                            {
                                failure = argument + " needs a value";
                                options = null;
                                return false;
                            }
                            i++;
                            break;
                        default:
                            failure = "unknown argument '" + argument + "'; run with --help";
                            options = null;
                            return false;
                    }

                    switch (argument)
                    {
                        case "--repo":
                            options.Repo = value;
                            break;
                        case "--fixture":
                            options.Fixture = value;
                            break;
                        case "--seconds":
                            if (!TryPositive(value, out options.Seconds))
                            {
                                failure = "--seconds needs a positive number, not '" + value + "'";
                                options = null;
                                return false;
                            }
                            break;
                        case "--warmup":
                            if (!double.TryParse(value, NumberStyles.Float, CultureInfo.InvariantCulture, out options.WarmupSeconds) ||
                                options.WarmupSeconds < 0)
                            {
                                failure = "--warmup needs a non-negative number, not '" + value + "'";
                                options = null;
                                return false;
                            }
                            break;
                        case "--levels":
                            levels = new List<int>();
                            foreach (string part in value.Split(','))
                            {
                                int level;
                                if (!int.TryParse(part.Trim(), NumberStyles.Integer, CultureInfo.InvariantCulture, out level) || level < 1)
                                {
                                    failure = "--levels takes comma-separated positive integers; '" + part.Trim() + "' is not one";
                                    options = null;
                                    return false;
                                }
                                levels.Add(level);
                            }
                            if (levels.Count == 0)
                            {
                                failure = "--levels was given no levels";
                                options = null;
                                return false;
                            }
                            break;
                        case "--label":
                            options.Label = value;
                            break;
                        case "--pool":
                            if (value == "none")
                            {
                                options.PoolSize = 0;
                            }
                            else if (!int.TryParse(value, NumberStyles.Integer, CultureInfo.InvariantCulture, out options.PoolSize) ||
                                     options.PoolSize < 1)
                            {
                                failure = "--pool takes 'none' or a positive integer, not '" + value + "'";
                                options = null;
                                return false;
                            }
                            break;
                        case "--host-context":
                            options.HostContext = value;
                            break;
                    }
                }

                // THE FOUR LEVELS THE STORY ASKS FOR, deduplicated and sorted.
                // 1 isolates the queue-hop cost; ProcessorCount answers the
                // question the owner actually asked when choosing a pool over a
                // single thread. Reporting only an aggregate would hide
                // whichever of the two went wrong.
                if (levels == null)
                {
                    levels = new List<int> { 1, 2, 4, Environment.ProcessorCount };
                    options.LevelsAreDefault = true;
                }
                SortedSet<int> distinct = new SortedSet<int>(levels);
                options.Levels = new int[distinct.Count];
                distinct.CopyTo(options.Levels);

                if (options.Repo == null)
                {
                    options.Repo = FindRepoRoot();
                    if (options.Repo == null)
                    {
                        failure = "could not find the repository root above " + AppContext.BaseDirectory +
                                  "; pass --repo <dir>";
                        options = null;
                        return false;
                    }
                }
                else if (!Directory.Exists(options.Repo))
                {
                    failure = "--repo names " + options.Repo + ", which is not a directory";
                    options = null;
                    return false;
                }

                return true;
            }

            private static bool TryPositive(string value, out double result)
            {
                return double.TryParse(value, NumberStyles.Float, CultureInfo.InvariantCulture, out result) && result > 0;
            }

            /// <summary>
            /// The repository root, found the way the test project finds it —
            /// by walking up looking for both <c>fixtures/</c> and
            /// <c>folio-go/</c>, so a directory that has only one of them
            /// cannot be mistaken for it.
            /// </summary>
            private static string FindRepoRoot()
            {
                DirectoryInfo dir = new DirectoryInfo(AppContext.BaseDirectory);
                while (dir != null)
                {
                    if (Directory.Exists(Path.Combine(dir.FullName, "fixtures")) &&
                        Directory.Exists(Path.Combine(dir.FullName, "folio-go")))
                    {
                        return dir.FullName;
                    }
                    dir = dir.Parent;
                }
                return null;
            }
        }
    }
}
