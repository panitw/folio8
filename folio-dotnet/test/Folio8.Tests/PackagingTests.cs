using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
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
        public void BothNativesArePackedIntoTheRidLayout()
        {
            Assert.Contains("PackagePath=\"runtimes/%(Rid)/native/\"", Project, StringComparison.Ordinal);
            Assert.Contains("win-x64/folio8_native.dll", Project, StringComparison.Ordinal);
            Assert.Contains("win-x86/folio8_native.dll", Project, StringComparison.Ordinal);
            // The COLLIDING name, in any of the places the csproj could spell
            // it. `native/folio8.dll` could never match: the natives are named
            // by their source path, `.../win-x64/folio8.dll`, so the guard has
            // to look for the file name after any separator.
            Assert.DoesNotContain("/folio8.dll", Project, StringComparison.Ordinal);
            Assert.DoesNotContain(@"\folio8.dll", Project, StringComparison.Ordinal);
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
        /// have to be stated, not implied.
        /// </summary>
        [Theory]
        [InlineData("dotnet add package folio8")]
        [InlineData("Fonts.Shipped()")]
        [InlineData(".NET Framework 4.6")]
        [InlineData("Windows only")]
        [InlineData("win-x86")]
        [InlineData("win-x64")]
        [InlineData("FolioNativeLoadException")]
        public void TheReadmeTellsAnInstallerWhatTheyCannotGuess(string text)
        {
            Assert.Contains(text, Repo.Text("folio-dotnet", "README.md"), StringComparison.Ordinal);
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
