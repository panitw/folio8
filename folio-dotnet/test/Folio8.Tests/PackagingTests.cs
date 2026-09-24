using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// What the NuGet package promises, checked where it is declared rather
    /// than by unpacking a <c>.nupkg</c> this suite would have to build first.
    /// The pack step's own completeness checks — both natives present, each
    /// the right architecture, every face at the length the engine recorded —
    /// live in Folio8.csproj and run on every <c>dotnet pack</c>; these check
    /// the declarations those depend on, and the invariants a pack cannot see.
    /// </summary>
    public class PackagingTests
    {
        private static string Project
        {
            get { return Repo.Text("folio-dotnet", "src", "Folio8", "Folio8.csproj"); }
        }

        /// <summary>The package's identity and metadata, as the story fixes them.</summary>
        [Theory]
        [InlineData("<PackageId>folio8</PackageId>")]
        [InlineData("<PackageLicenseFile>LICENSE</PackageLicenseFile>")]
        [InlineData("<PackageReadmeFile>README.md</PackageReadmeFile>")]
        [InlineData("<PackageProjectUrl>https://folio8.report</PackageProjectUrl>")]
        [InlineData("<RepositoryUrl>https://github.com/panitw/folio8.git</RepositoryUrl>")]
        [InlineData("<IsPackable>true</IsPackable>")]
        public void ThePackageDeclaresItsMetadata(string declaration)
        {
            Assert.Contains(declaration, Project, StringComparison.Ordinal);
        }

        /// <summary>
        /// The version's SHAPE, not its value. Pinning the literal here would
        /// make a routine bump redden a test that has no opinion about which
        /// version is right — and the consumer projects take the version from
        /// the packed file name rather than restating it, so this is the only
        /// place it needs to be well-formed.
        /// </summary>
        [Fact]
        public void ThePackageVersionIsDeclaredOnceAndIsWellFormed()
        {
            Match version = Regex.Match(Project, @"<Version>([^<]+)</Version>");
            Assert.True(version.Success, "Folio8.csproj declares no <Version>");
            Assert.Matches(@"^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$", version.Groups[1].Value);
            Assert.Single(Regex.Matches(Project, @"<Version>"));
        }

        /// <summary>
        /// The RID layout packaging-matrix.md specifies, with the file name
        /// story 6 had to rename it to. <c>folio8.dll</c> would BE
        /// <c>Folio8.dll</c> on case-insensitive Windows.
        /// </summary>
        [Fact]
        public void AllFourNativesArePackedIntoTheRidLayout()
        {
            Assert.Contains("PackagePath=\"runtimes/%(Rid)/native/\"", Project, StringComparison.Ordinal);
            Assert.Contains("win-x64/folio8_native.dll", Project, StringComparison.Ordinal);
            Assert.Contains("win-x86/folio8_native.dll", Project, StringComparison.Ordinal);
            Assert.Contains("linux-x64/libfolio8_native.so", Project, StringComparison.Ordinal);
            Assert.Contains("linux-arm64/libfolio8_native.so", Project, StringComparison.Ordinal);
            // BOTH LINUX RIDs SHIP, AND THAT IS ENFORCED RATHER THAN ASSUMED.
            // This assertion used to run the other way: the two Linux natives
            // were built and verified for 1.1.0 and then withdrawn, because
            // entering the engine from a CLR thread-pool thread overflowed
            // that thread's sigaltstack and killed the process (DW-396).
            // The binding now crosses the ABI only on binding-owned engine
            // threads whose signal stacks it sizes itself, so the ban is
            // inverted rather than deleted: dropping a FolioNative item is a
            // HALF-RESTORED package, and this is what catches it.
            //
            // IT READS THE RID ELEMENTS, NOT THE FILE'S TEXT: the comment
            // beside those items names the Linux RIDs while recounting why
            // they were once out, and a raw substring search cannot tell the
            // history from the shipping declaration.
            Assert.Equal(
                new[] { "linux-arm64", "linux-x64", "win-x64", "win-x86" },
                Sorted(PackedRids()));
            // The COLLIDING name, in any of the places the csproj could spell
            // it. `native/folio8.dll` could never match: the natives are named
            // by their source path, `.../win-x64/folio8.dll`, so the guard has
            // to look for the file name after any separator.
            Assert.DoesNotContain("/folio8.dll", Project, StringComparison.Ordinal);
            Assert.DoesNotContain(@"\folio8.dll", Project, StringComparison.Ordinal);
        }

        /// <summary>
        /// NO musl RID, EVER. Go's <c>-buildmode=c-shared</c> emits
        /// initial-exec TLS relocations musl's loader refuses under
        /// <c>dlopen</c>, so shipping one would convert an Alpine consumer's
        /// clean install-time absence into a first-render crash.
        /// </summary>
        [Fact]
        public void NoMuslRidIsPacked()
        {
            Assert.DoesNotContain(PackedRids(), rid => rid.IndexOf("musl", StringComparison.Ordinal) >= 0);
        }

        private static List<string> Sorted(List<string> values)
        {
            values.Sort(StringComparer.Ordinal);
            return values;
        }

        /// <summary>
        /// EVERY PACKED RID IS KNOWN TO THE PACK CHECK. FolioPackageCheck
        /// validates each native's machine type against the RID it is filed
        /// under, and it can only do that for RIDs it has an arm for. Adding
        /// a FolioNative item without adding the arm would ship that native
        /// unverified — which is the one thing the check exists to prevent —
        /// so the two lists are held equal here rather than by eye.
        /// </summary>
        /// <summary>
        /// The RIDs the csproj actually files a native under — read from the
        /// <c>&lt;Rid&gt;</c> metadata rather than from the file's prose, so
        /// that commentary about a RID is never mistaken for shipping one.
        /// </summary>
        private static List<string> PackedRids()
        {
            var rids = new List<string>();
            foreach (Match m in Regex.Matches(Project, @"<Rid>([^<]+)</Rid>"))
            {
                rids.Add(m.Groups[1].Value);
            }
            Assert.NotEmpty(rids);
            return rids;
        }

        [Fact]
        public void EveryPackedRidHasAnArchitectureArmInThePackCheck()
        {
            foreach (string rid in PackedRids())
            {
                Assert.Contains("rid == \"" + rid + "\"", Project, StringComparison.Ordinal);
            }
        }

        /// <summary>
        /// THE FILES THAT MAY MENTION THE SOAK ASSERTION, by exact path, and
        /// every one of them asserted to exist — a renamed exemption must
        /// redden here rather than silently widen the sweep below.
        /// </summary>
        private static readonly string[] SoakAssertionExemptions =
        {
            // Declares the gate, and spells the property in its own refusal
            // text and in the command it documents. It does not SET it.
            "folio-dotnet/src/Folio8/Folio8.csproj",
            // This file.
            "folio-dotnet/test/Folio8.Tests/PackagingTests.cs",
            // The release procedure, typed by a human — exactly as
            // NothingInTheRepositoryPushesToNuGet already treats it.
            "RELEASING.md",
        };

        /// <summary>The planning record. Nothing under it runs.</summary>
        private const string SoakAssertionExemptPrefix = "_bmad-output/";

        /// <summary>
        /// THE PACK GATE EXISTS, AND IT IS SPELLED AS A CLAIM RATHER THAN A
        /// FLAG. Restoring the Linux RIDs made the repository ready; CAP-5's
        /// two hardware legs are still PENDING, so packing a Linux RID has to
        /// stay a deliberate act that names the evidence it asserts. A
        /// boolean, or a default value set anywhere in the tree, would make
        /// it a rubber stamp.
        /// </summary>
        [Fact]
        public void ThePackGateNamesWhatItIsWaitingFor()
        {
            Assert.Contains("<Target Name=\"FolioAssertLinuxSoak\" BeforeTargets=\"GenerateNuspec\">", Project, StringComparison.Ordinal);
            // It is armed by the Linux RIDs themselves, not by a hand-written list.
            Assert.Contains("StartsWith('linux')", Project, StringComparison.Ordinal);
            // ORDINAL, because MSBuild's own `!=` is case-insensitive and the
            // claim is one a human types word for word.
            Assert.Contains("CompareOrdinal", Project, StringComparison.Ordinal);
            // And it refuses an assertion that arrives from the environment,
            // where no sweep of tracked files could ever see it.
            Assert.Contains("GetEnvironmentVariable(`FolioLinuxSoakEvidence`)", Project, StringComparison.Ordinal);
            // It says what to run, where the record is, and which capability is outstanding.
            foreach (string named in new[] { "CAP-5", "soak.sh", "dotnet-linux-soak.md", "FolioLinuxSoakEvidence" })
            {
                Assert.Contains(named, Project, StringComparison.Ordinal);
            }
            // AND THE TWO PATHS IT NAMES ARE REALLY THERE. Pinning the
            // substrings alone would let either move and leave the refusal
            // instructing a packer to run something that does not exist.
            Assert.True(File.Exists(Repo.Path_("folio-dotnet", "build", "soak.sh")),
                "the gate tells a packer to run folio-dotnet/build/soak.sh, which is not there");
            Assert.True(File.Exists(Repo.Path_("_bmad-output", "implementation-artifacts", "dotnet-linux-soak.md")),
                "the gate names _bmad-output/implementation-artifacts/dotnet-linux-soak.md as the soak record, which is not there");

            // THE EXPECTED VALUE IS A LITERAL IN THE CONDITION, NOT A
            // PROPERTY. It was a property once: MSBuild global properties
            // override everything, so `-p:<that property>=` with no evidence
            // made both sides empty and opened the gate with nothing typed.
            Assert.True(ExpectedSoakAssertion().Length > 0);
        }

        /// <summary>
        /// NOTHING IN THE REPOSITORY SUPPLIES THE ASSERTION. A gate the tree
        /// satisfies on the packer's behalf is not a gate. The filter is
        /// INVERTED on purpose — every tracked text file, minus a short named
        /// list — because an allowlist of extensions missed this repository's
        /// tracked Makefile and Dockerfile, and a `pack:` rule in either would
        /// have opened the gate with this test still green.
        /// </summary>
        [Fact]
        public void NoTrackedFileSuppliesTheSoakAssertion()
        {
            foreach (string exemption in SoakAssertionExemptions)
            {
                Assert.True(File.Exists(Repo.Path_(exemption.Split('/'))),
                    "the sweep exempts " + exemption + ", which no longer exists — the exemption is now a hole");
            }
            Assert.True(Directory.Exists(Repo.Path_(SoakAssertionExemptPrefix.TrimEnd('/'))),
                "the sweep exempts " + SoakAssertionExemptPrefix + ", which no longer exists");

            // The ELEMENT form, including the attributed spelling
            // `<FolioLinuxSoakEvidence Condition="...">` a bare substring
            // search would walk straight past.
            Assert.False(Regex.IsMatch(Project, @"<FolioLinuxSoakEvidence[\s/>]"),
                "Folio8.csproj sets the soak assertion itself, which turns its own gate off");

            List<string> offenders = new List<string>();
            int read = 0;
            foreach (string path in TrackedFiles(null))
            {
                if (IsSoakAssertionExempt(path))
                {
                    continue;
                }
                string text;
                if (!TryReadTrackedText(path, out text))
                {
                    continue;
                }
                read++;
                if (text.IndexOf("FolioLinuxSoakEvidence", StringComparison.Ordinal) >= 0)
                {
                    offenders.Add(path);
                }
            }
            // VACUITY GUARD: a sweep that read nothing is not a pass.
            Assert.True(read > 50, "the sweep read only " + read + " tracked text files, so it asserts nothing");
            Assert.True(offenders.Count == 0, "these mention the soak assertion, and no tracked file outside the named exemptions may — a gate the repository satisfies for you is not a gate: " + string.Join(", ", offenders));
        }

        /// <summary>
        /// EVERY TRACKED `dotnet pack` OF THIS PROJECT IS COMPATIBLE WITH THE
        /// GATE. There are exactly two ways to be: assert the CAP-5 soak (the
        /// release pack), or pack the Windows RIDs alone with
        /// <c>-p:FolioPackPlatforms=windows</c>, which leaves no Linux RID for
        /// the gate to arm off. A third way — a pack that simply fails — is
        /// how the .NET Framework consumer harness broke when the gate landed,
        /// and it broke the one job that proves those consumers work at all.
        /// </summary>
        [Fact]
        public void EveryTrackedPackOfThisProjectSatisfiesTheGate()
        {
            List<string> offenders = new List<string>();
            int invocations = 0;
            foreach (string path in TrackedFiles(null))
            {
                if (IsSoakAssertionExempt(path) && !path.EndsWith("RELEASING.md", StringComparison.Ordinal))
                {
                    continue;
                }
                string text;
                if (!TryReadTrackedText(path, out text))
                {
                    continue;
                }
                // AN INVOCATION, NOT A MENTION. The consumer projects and
                // RELEASING.md both discuss `dotnet pack` in prose beside the
                // words "Folio8.csproj"; what is being enumerated here is a
                // command, so the project path has to be the ARGUMENT — either
                // written out, or PowerShell's Join-Path form.
                foreach (Match invocation in Regex.Matches(text, @"dotnet pack\s+(?:""?[^\s""]*Folio8\.csproj|\(Join-Path[^)]*Folio8\.csproj'\))"))
                {
                    invocations++;
                    // The command may continue onto the next lines — RELEASING.md's
                    // does, with a PowerShell backtick.
                    string[] window = text.Substring(invocation.Index, Math.Min(400, text.Length - invocation.Index)).Split('\n');
                    string command = string.Join("\n", window, 0, Math.Min(3, window.Length));
                    bool asserts = command.IndexOf("-p:FolioLinuxSoakEvidence=", StringComparison.Ordinal) >= 0;
                    bool windowsOnly = command.IndexOf("-p:FolioPackPlatforms=windows", StringComparison.Ordinal) >= 0;
                    if (!asserts && !windowsOnly)
                    {
                        int line = 1;
                        for (int i = 0; i < invocation.Index; i++)
                        {
                            if (text[i] == '\n') { line++; }
                        }
                        offenders.Add(path + ":" + line);
                    }
                }
            }
            Assert.True(invocations >= 2, "found " + invocations + " tracked `dotnet pack` of Folio8.csproj; the release procedure and the consumer harness are both supposed to be found, so this check is not looking where it thinks it is");
            Assert.True(offenders.Count == 0, "these pack Folio8.csproj in a way the soak gate refuses — each must either assert CAP-5's soak or pack -p:FolioPackPlatforms=windows: " + string.Join(", ", offenders));
        }

        /// <summary>
        /// AND IT ACTUALLY REFUSES A REAL PACK. The assertions above read the
        /// file and the one below runs the target by name — neither would
        /// notice an SDK change that reworked the pack graph out from under
        /// <c>BeforeTargets="GenerateNuspec"</c>, leaving both green while
        /// `dotnet pack` produced an unsoaked package. This runs the command
        /// the gate exists to stop.
        /// </summary>
        [Fact]
        public void ADotnetPackWithNoAssertionRefusesAndWritesNothing()
        {
            string output = Path.Combine(Path.GetTempPath(), "folio8-packgate-" + Guid.NewGuid().ToString("N"));
            try
            {
                string result;
                int exit = RunDotnet(
                    "pack \"" + Repo.Path_("folio-dotnet", "src", "Folio8", "Folio8.csproj") + "\" -c Release -o \"" + output + "\" -nologo",
                    null,
                    TimeSpan.FromMinutes(5),
                    out result);

                Assert.True(exit != 0, "`dotnet pack` with no soak assertion SUCCEEDED; the Linux RIDs are one command from shipping unsoaked:\n" + result);
                Assert.Contains("CAP-5", result, StringComparison.Ordinal);
                string[] written = Directory.Exists(output) ? Directory.GetFiles(output, "*.nupkg") : new string[0];
                Assert.True(written.Length == 0, "the refused pack still wrote " + string.Join(", ", written));
            }
            finally
            {
                if (Directory.Exists(output))
                {
                    Directory.Delete(output, true);
                }
            }
        }

        /// <summary>
        /// AND IT OPENS FOR THE CLAIM IT DOCUMENTS — otherwise the release it
        /// gates could not be made. The target is invoked by name here because
        /// this leg only asks about the condition; the leg above is the one
        /// that asks about `dotnet pack`.
        /// </summary>
        [Fact]
        public void ThePackGateOpensOnlyForTheAssertionItDocuments()
        {
            string csproj = "\"" + Repo.Path_("folio-dotnet", "src", "Folio8", "Folio8.csproj") + "\"";
            string invocation = "msbuild " + csproj + " -t:FolioAssertLinuxSoak -nologo -v:m";

            string refusal;
            Assert.True(RunDotnet(invocation, null, TimeSpan.FromMinutes(2), out refusal) != 0,
                "the gate opened with no assertion at all:\n" + refusal);
            Assert.Contains("CAP-5", refusal, StringComparison.Ordinal);
            Assert.Contains("soak.sh", refusal, StringComparison.Ordinal);

            // The WRONG CASE is a different claim. MSBuild's own `!=` would
            // have accepted it.
            string miscased;
            Assert.True(RunDotnet(invocation + " -p:FolioLinuxSoakEvidence=\"" + ExpectedSoakAssertion().ToUpperInvariant() + "\"", null, TimeSpan.FromMinutes(2), out miscased) != 0,
                "the gate accepted a differently-cased assertion, so it is not comparing ordinally:\n" + miscased);

            // AN ASSERTION THAT ARRIVED IN THE ENVIRONMENT IS NOT AN
            // ASSERTION. MSBuild reads environment variables as properties, so
            // without this leg an exported value would open the gate from a
            // shell profile or a CI env block — the one place the tracked-file
            // sweep cannot look.
            string exported;
            Assert.True(RunDotnet(invocation, ExpectedSoakAssertion(), TimeSpan.FromMinutes(2), out exported) != 0,
                "the gate opened for an assertion exported into the environment:\n" + exported);
            Assert.Contains("ENVIRONMENT", exported, StringComparison.Ordinal);

            // A WINDOWS-ONLY PACK disarms it, because there is no Linux RID left.
            string windowsOnly;
            Assert.True(RunDotnet(invocation + " -p:FolioPackPlatforms=windows", null, TimeSpan.FromMinutes(2), out windowsOnly) == 0,
                "the Windows-only pack the consumer harness uses is refused by the gate:\n" + windowsOnly);

            // And the claim itself opens it.
            string accepted;
            Assert.True(RunDotnet(invocation + " -p:FolioLinuxSoakEvidence=\"" + ExpectedSoakAssertion() + "\"", null, TimeSpan.FromMinutes(2), out accepted) == 0,
                "the gate refused even the assertion it documents, so it cannot be opened at release time:\n" + accepted);
        }

        /// <summary>
        /// The claim the csproj's own Error condition compares against — READ,
        /// never retyped, so re-wording it cannot leave these tests asserting
        /// the old one.
        /// </summary>
        private static string ExpectedSoakAssertion()
        {
            Match declared = Regex.Match(Project, @"CompareOrdinal\('\$\(FolioLinuxSoakEvidence\)',\s*'([^']+)'\)");
            Assert.True(declared.Success, "Folio8.csproj's soak gate no longer compares FolioLinuxSoakEvidence against a literal, so nothing states what a packer must assert");
            return declared.Groups[1].Value;
        }

        private static bool IsSoakAssertionExempt(string path)
        {
            if (path.StartsWith(SoakAssertionExemptPrefix, StringComparison.Ordinal))
            {
                return true;
            }
            foreach (string exemption in SoakAssertionExemptions)
            {
                if (string.Equals(path, exemption, StringComparison.Ordinal))
                {
                    return true;
                }
            }
            return false;
        }

        /// <summary>
        /// A tracked file's text, or false for one that is not text. Binaries
        /// are recognised by a NUL byte rather than by extension — the point
        /// of the sweep above is that it does not keep a list of what to look
        /// at — and anything over a megabyte is skipped because nothing that
        /// sets a build property is that large.
        /// </summary>
        private static bool TryReadTrackedText(string path, out string text)
        {
            text = null;
            string full = Repo.Path_(path.Split('/'));
            if (!File.Exists(full))
            {
                return false;
            }
            FileInfo info = new FileInfo(full);
            if (info.Length == 0 || info.Length > 1024 * 1024)
            {
                return false;
            }
            byte[] bytes = File.ReadAllBytes(full);
            int sniff = Math.Min(bytes.Length, 8000);
            for (int i = 0; i < sniff; i++)
            {
                if (bytes[i] == 0)
                {
                    return false;
                }
            }
            text = Encoding.UTF8.GetString(bytes);
            return true;
        }

        /// <summary>
        /// `dotnet` with the arguments as ONE STRING, because
        /// <c>ProcessStartInfo.ArgumentList</c> does not exist on .NET
        /// Framework and this suite also targets net48 on Windows. Both
        /// streams are drained asynchronously — reading one to the end first
        /// deadlocks the moment the child fills the other — and the wait is
        /// bounded, because a first-run SDK or NuGet stall would otherwise
        /// hang the suite with no diagnostic at all.
        /// </summary>
        /// <summary>
        /// A `dotnet` WITH AN SDK IN IT, FOUND RATHER THAN ASSUMED.
        ///
        /// These tests shell out to `dotnet pack` and `dotnet msbuild`, which
        /// need the SDK. Bare "dotnet" resolves through PATH, and on the x86
        /// corpus legs that is the wrong install: ci.yml puts a 32-bit
        /// RUNTIME-ONLY .NET in `%ProgramFiles(x86)%\dotnet`, which the
        /// runner image already has on PATH ahead of the x64 SDK. The child
        /// then answered "The command could not be loaded, possibly because
        /// this is not a valid .NET SDK command" and both gate tests failed
        /// on that leg alone while passing everywhere else.
        ///
        /// CLEARING DOTNET_ROOT WAS THE FIRST FIX AND IT WAS WRONG -- the
        /// variable was never the path being taken; PATH order was. This
        /// picks the first candidate that actually has an `sdk` directory,
        /// so the answer does not depend on what is in front on PATH.
        ///
        /// IT FAILS LOUDLY IF IT FINDS NOTHING. The pack gate is what stands
        /// between an unsoaked Linux RID and NuGet; a leg that cannot run it
        /// must say so, not pass quietly.
        /// </summary>
        private static string DotnetWithAnSdk()
        {
            List<string> candidates = new List<string>();
            string viaHost = Environment.GetEnvironmentVariable("DOTNET_HOST_PATH");
            if (!string.IsNullOrEmpty(viaHost))
            {
                candidates.Add(viaHost);
            }
            bool windows = Path.DirectorySeparatorChar == '\\';
            string exe = windows ? "dotnet.exe" : "dotnet";
            foreach (string variable in new string[] { "ProgramFiles", "ProgramW6432" })
            {
                string programs = Environment.GetEnvironmentVariable(variable);
                if (!string.IsNullOrEmpty(programs))
                {
                    candidates.Add(Path.Combine(programs, "dotnet", exe));
                }
            }
            candidates.Add("/usr/share/dotnet/" + exe);
            candidates.Add("/usr/local/share/dotnet/" + exe);
            candidates.Add(Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".dotnet", exe));

            foreach (string candidate in candidates)
            {
                if (string.IsNullOrEmpty(candidate) || !File.Exists(candidate))
                {
                    continue;
                }
                string sdks = Path.Combine(Path.GetDirectoryName(candidate), "sdk");
                if (Directory.Exists(sdks) && Directory.GetDirectories(sdks).Length > 0)
                {
                    return candidate;
                }
            }

            // Nothing verified. Fall back to PATH rather than refusing here:
            // on a developer machine `dotnet` is usually right, and a wrong
            // answer surfaces as these tests failing with the muxer's own
            // message rather than as a silent skip.
            return "dotnet";
        }

        private static int RunDotnet(string arguments, string soakEvidence, TimeSpan limit, out string output)
        {
            ProcessStartInfo start = new ProcessStartInfo(DotnetWithAnSdk(), arguments)
            {
                WorkingDirectory = Repo.Root,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            // A DEVELOPER WITH THE EVIDENCE EXPORTED MUST STILL SEE THE TRUTH.
            // Inheriting it would make the refusing legs above pass for the
            // wrong reason on their machine and fail confusingly here.
            start.EnvironmentVariables.Remove("FolioLinuxSoakEvidence");

            // AND THE 32-BIT LEG MUST NOT SEND `dotnet` TO A RUNTIME-ONLY
            // INSTALL. ci.yml installs an x86 runtime by hand for the x86
            // corpus legs -- "Runtime only, not the SDK", as it says -- and
            // exports DOTNET_ROOT(x86) pointing at it. Every child of that
            // test host inherits the variable, so the `dotnet` started here
            // resolved an install with no SDK in it and answered "The command
            // could not be loaded", which is not the refusal these tests are
            // reading for: the gate tests then failed on the x86 leg while
            // passing everywhere else. Clearing the roots lets the muxer use
            // its own location, which is the SDK that built the suite.
            //
            // THE ALTERNATIVE WAS TO SKIP THESE TWO ON x86, AND IT IS WORSE.
            // The pack gate is the one thing standing between an unsoaked
            // Linux RID and NuGet; a leg that quietly does not run it is how
            // that guard goes missing without anything reddening.
            start.EnvironmentVariables.Remove("DOTNET_ROOT");
            start.EnvironmentVariables.Remove("DOTNET_ROOT(x86)");
            start.EnvironmentVariables.Remove("DOTNET_ROOT_X86");
            start.EnvironmentVariables.Remove("DOTNET_ROOT_X64");
            if (soakEvidence != null)
            {
                start.EnvironmentVariables["FolioLinuxSoakEvidence"] = soakEvidence;
            }

            StringBuilder collected = new StringBuilder();
            using (Process child = new Process())
            {
                child.StartInfo = start;
                DataReceivedEventHandler collect = delegate(object sender, DataReceivedEventArgs e)
                {
                    if (e.Data != null)
                    {
                        lock (collected) { collected.AppendLine(e.Data); }
                    }
                };
                child.OutputDataReceived += collect;
                child.ErrorDataReceived += collect;
                child.Start();
                child.BeginOutputReadLine();
                child.BeginErrorReadLine();
                if (!child.WaitForExit((int)limit.TotalMilliseconds))
                {
                    try { child.Kill(); } catch (Exception) { }
                    lock (collected) { output = collected.ToString(); }
                    Assert.Fail("`dotnet " + arguments + "` did not finish within " + limit + "; killed. Output so far:\n" + output);
                }
                // The async readers can still be draining when WaitForExit(int)
                // returns; the parameterless overload waits for them too.
                child.WaitForExit();
                lock (collected) { output = collected.ToString(); }
                return child.ExitCode;
            }
        }

        /// <summary>
        /// The .NET Framework delivery is packed where NuGet imports it
        /// automatically — <c>build/&lt;package id&gt;.targets</c>. Named
        /// anything else it is inert and a 4.6 consumer gets no native
        /// library at all.
        /// </summary>
        [Fact]
        public void TheFrameworkTargetsArePackedWhereNuGetImportsThem()
        {
            Assert.Contains("PackagePath=\"build/folio8.targets\"", Project, StringComparison.Ordinal);
        }

        /// <summary>
        /// The targets file and the loader must agree about where the natives
        /// land, and there is no mechanism that makes them: one is MSBuild,
        /// the other is C#. This is that mechanism.
        /// </summary>
        [Fact]
        public void TheTargetsStageBothArchitecturesWhereTheLoaderProbes()
        {
            string targets = Repo.Text("folio-dotnet", "build", "folio-dotnet.targets");
            Assert.Contains("'$(TargetFrameworkIdentifier)' == '.NETFramework'", targets, StringComparison.Ordinal);
            Assert.Contains("runtimes/", targets, StringComparison.Ordinal);
            foreach (string rid in new[] { "win-x64", "win-x86" })
            {
                // Read from the RID layout the package already carries...
                Assert.Contains(rid + "/native/" + NativeLibraryLoader.WindowsFileName, targets, StringComparison.Ordinal);
                // ...and written where NativeLibraryLoader's second candidate looks.
                Assert.Contains(NativeLibraryLoader.FrameworkNativeFolder + @"\" + rid + @"\" + NativeLibraryLoader.WindowsFileName, targets, StringComparison.Ordinal);
            }
        }

        /// <summary>
        /// The engine version is RECORDED, and the three places that know it
        /// agree: the package's assembly metadata, the native library the
        /// process actually loaded, and the record the Go engine itself wrote.
        /// </summary>
        [Fact]
        public void TheRecordedEngineVersionMatchesTheEngine()
        {
            string declared = null;
            foreach (AssemblyMetadataAttribute metadata in typeof(Fonts).Assembly.GetCustomAttributes<AssemblyMetadataAttribute>())
            {
                if (metadata.Key == "folio8EngineVersion")
                {
                    declared = metadata.Value;
                }
            }
            Assert.False(string.IsNullOrEmpty(declared), "Folio8.dll records no folio8EngineVersion; Folio8.csproj's AssemblyMetadata is missing");

            using (JsonDocument document = JsonDocument.Parse(Repo.Text("folio-js", "test", "data", "go-parity.json")))
            {
                Assert.Equal(document.RootElement.GetProperty("folio8Version").GetString(), declared);
            }
            Assert.Equal(Folio8.Version, declared);
        }

        /// <summary>The package takes no runtime dependency, which is the whole of its dependency policy.</summary>
        [Fact]
        public void ThePackageDeclaresNoDependencies()
        {
            Assert.DoesNotContain("<PackageReference", Project, StringComparison.Ordinal);
        }

        /// <summary>
        /// The licence travels IN the package — an installed package carries
        /// no repository around it — and it is the repository's own MIT terms,
        /// byte for byte, not a retyped copy.
        /// </summary>
        [Fact]
        public void TheLicenceInThePackageIsTheRepositorysOwn()
        {
            Assert.Equal(
                Repo.Sha256(Repo.File_("LICENSE")),
                Repo.Sha256(Repo.File_("folio-dotnet", "LICENSE")));
        }

        /// <summary>AD-26: the census records the path this repository redistributes.</summary>
        [Fact]
        public void TheLicenceCensusRecordsThePackagesLicence()
        {
            string census = Repo.Text("lint", "internal", "licence", "licencecensus_test.go");
            Assert.Contains("{\"folio-dotnet/LICENSE\", FamilyPermissive, \"MIT\"}", census, StringComparison.Ordinal);
        }

        /// <summary>
        /// The README is what an installer reads first, so the two things they
        /// cannot discover by trying — which frameworks and which platforms —
        /// have to be stated, not implied. These pins ran the other way until
        /// the Linux RIDs were restored: they required "Windows only" and the
        /// section "Why there is no Linux build yet". They now pin what is
        /// true, so the README cannot quietly fall back to the old claim.
        /// </summary>
        [Theory]
        [InlineData("dotnet add package folio8")]
        [InlineData("Fonts.Shipped()")]
        [InlineData(".NET Framework 4.6")]
        [InlineData("win-x86")]
        [InlineData("win-x64")]
        // The four shipped RIDs, named where an installer looks for them.
        [InlineData("linux-x64")]
        [InlineData("linux-arm64")]
        // The glibc floor the pinned build image produces. A consumer cannot
        // guess it and it is the one thing that decides whether their base
        // image works.
        [InlineData("glibc 2.28")]
        // The musl exclusion, stated as a decision with its reason rather
        // than left for an Alpine user to discover by deploying.
        [InlineData("linux-musl-x64")]
        [InlineData("FolioNativeLoadException")]
        public void TheReadmeTellsAnInstallerWhatTheyCannotGuess(string text)
        {
            Assert.Contains(text, Repo.Text("folio-dotnet", "README.md"), StringComparison.Ordinal);
        }

        /// <summary>
        /// AND IT NO LONGER CARRIES THE WITHDRAWAL'S CLAIMS. Inverting the
        /// pins above is only half of it: a README that states the four RIDs
        /// AND still says "Windows only" somewhere further down is worse than
        /// either, and nothing above would catch it.
        /// </summary>
        [Theory]
        [InlineData("Windows only")]
        [InlineData("Why there is no Linux build yet")]
        public void TheReadmeNoLongerClaimsTheWithdrawal(string text)
        {
            Assert.DoesNotContain(text, Repo.Text("folio-dotnet", "README.md"), StringComparison.Ordinal);
        }

        /// <summary>
        /// NO BINARY IS TRACKED under folio-dotnet/. The faces come from
        /// folio-go/fonts/ at build time and the natives from
        /// build-native.{sh,ps1}; a committed copy of either is a second
        /// source of truth that can drift in silence.
        /// </summary>
        [Fact]
        public void NoFontOrBinaryIsTrackedUnderFolioDotnet()
        {
            List<string> offenders = new List<string>();
            foreach (string path in TrackedFiles("folio-dotnet"))
            {
                string lower = path.ToLowerInvariant();
                if (lower.EndsWith(".ttf", StringComparison.Ordinal) ||
                    lower.EndsWith(".dll", StringComparison.Ordinal) ||
                    lower.EndsWith(".nupkg", StringComparison.Ordinal))
                {
                    offenders.Add(path);
                }
            }
            Assert.True(offenders.Count == 0, "these are tracked by git and must not be: " + string.Join(", ", offenders));
        }

        /// <summary>
        /// PUBLISHING IS NEVER AUTOMATED. No script and no workflow in this
        /// repository runs `dotnet nuget push`; RELEASING.md's procedure is
        /// typed by the owner, on the owner's go-ahead.
        /// </summary>
        [Fact]
        public void NothingInTheRepositoryPushesToNuGet()
        {
            List<string> offenders = new List<string>();
            foreach (string path in TrackedFiles(null))
            {
                string lower = path.ToLowerInvariant();
                // RELEASING.md is the ONE place the command is written down —
                // it is the procedure. Everything a machine could run is in
                // scope, which is why this list covers the script languages
                // this repository actually uses rather than only MSBuild's.
                if (lower.EndsWith("releasing.md", StringComparison.Ordinal))
                {
                    continue;
                }
                bool executable = false;
                foreach (string extension in new[]
                {
                    ".yml", ".yaml", ".sh", ".bash", ".ps1", ".psm1", ".cmd", ".bat",
                    ".mjs", ".cjs", ".js", ".ts", ".py", ".csproj", ".targets", ".props", ".slnx",
                })
                {
                    executable = executable || lower.EndsWith(extension, StringComparison.Ordinal);
                }
                executable = executable ||
                    lower.EndsWith("makefile", StringComparison.Ordinal) ||
                    lower.EndsWith("dockerfile", StringComparison.Ordinal);
                if (!executable)
                {
                    continue;
                }
                string full = Repo.Path_(path.Split('/'));
                if (!File.Exists(full))
                {
                    continue;
                }
                if (File.ReadAllText(full).IndexOf("nuget push", StringComparison.OrdinalIgnoreCase) >= 0)
                {
                    offenders.Add(path);
                }
            }
            Assert.True(offenders.Count == 0, "these run `dotnet nuget push`, which no script or CI job in this repository may: " + string.Join(", ", offenders));
        }

        /// <summary>RELEASING.md carries the procedure, since nothing automates it.</summary>
        [Theory]
        [InlineData("Publishing `folio8` to NuGet")]
        [InlineData("folio-dotnet/v")]
        [InlineData("dotnet nuget push")]
        public void ReleasingDocumentsTheNuGetProcedure(string text)
        {
            Assert.Contains(text, Repo.Text("RELEASING.md"), StringComparison.Ordinal);
        }

        private static IEnumerable<string> TrackedFiles(string scope)
        {
            ProcessStartInfo start = new ProcessStartInfo("git", scope == null ? "ls-files" : "ls-files -- " + scope)
            {
                WorkingDirectory = Repo.Root,
                RedirectStandardOutput = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            using (Process git = Process.Start(start))
            {
                string output = git.StandardOutput.ReadToEnd();
                git.WaitForExit();
                Assert.True(git.ExitCode == 0, "git ls-files failed, so this check would be vacuous");
                string[] lines = output.Split(new[] { '\n', '\r' }, StringSplitOptions.RemoveEmptyEntries);
                // VACUITY GUARD: a listing that found nothing is not a pass.
                Assert.NotEmpty(lines);
                return lines;
            }
        }
    }
}
