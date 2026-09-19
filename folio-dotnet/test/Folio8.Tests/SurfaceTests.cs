using System;
using System.Collections.Generic;
using System.IO;
using System.Text.RegularExpressions;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// The floor and the no-rendering-logic rule, checked by reading the
    /// shipped sources rather than by asserting them in prose. These are the
    /// acceptance criteria of the story, executed.
    /// </summary>
    public class SurfaceTests
    {
        private static IEnumerable<string> SourceFiles()
        {
            string src = Repo.Path_("folio-dotnet", "src", "Folio8");
            Assert.True(Directory.Exists(src), "folio-dotnet/src/Folio8 is missing at " + src);
            string[] files = Directory.GetFiles(src, "*.cs", SearchOption.AllDirectories);
            // VACUITY GUARD: a scan that found nothing is not a pass.
            Assert.NotEmpty(files);
            return files;
        }

        /// <summary>
        /// The .NET Framework 4.6 floor bans the modern toolkit. LangVersion
        /// 7.3 and the net46 compile project already make most of this a
        /// compiler error; this closes the cases that would compile under
        /// netstandard2.0 yet break on the floor.
        /// </summary>
        [Theory]
        [InlineData(@"\bSpan<")]
        [InlineData(@"\bReadOnlySpan<")]
        [InlineData(@"\bMemory<")]
        [InlineData(@"System\.Text\.Json")]
        [InlineData(@"\bNativeLibrary\b")]
        [InlineData(@"\bDllImportResolver\b")]
        public void TheShippedSourcesUseNothingTheFloorForbids(string pattern)
        {
            Regex forbidden = new Regex(pattern);
            foreach (string file in SourceFiles())
            {
                foreach (string line in File.ReadAllLines(file))
                {
                    // A doc comment may NAME what is banned; code may not use it.
                    if (line.TrimStart().StartsWith("//", StringComparison.Ordinal))
                    {
                        continue;
                    }
                    Assert.False(forbidden.IsMatch(line),
                        Path.GetFileName(file) + " uses " + pattern + ", which .NET Framework 4.6 does not have: " + line.Trim());
                }
            }
        }

        /// <summary>
        /// No rendering logic in C#. Layout, shaping, pagination and PDF
        /// emission stay in one engine, which is what makes byte identity
        /// possible at all — so the managed sources must not contain the
        /// vocabulary of a renderer.
        /// </summary>
        [Theory]
        [InlineData(@"\bMeasureText\b")]
        [InlineData(@"\bShape\w*Run\b")]
        [InlineData(@"\bLayout\w*\(")]
        [InlineData(@"%PDF")]
        [InlineData(@"\bxref\b")]
        public void TheShippedSourcesContainNoRenderingLogic(string pattern)
        {
            Regex forbidden = new Regex(pattern);
            foreach (string file in SourceFiles())
            {
                string text = File.ReadAllText(file);
                Assert.False(forbidden.IsMatch(text), Path.GetFileName(file) + " looks like it contains rendering logic (" + pattern + ")");
            }
        }

        /// <summary>
        /// The managed assembly takes no third-party runtime dependency. Not
        /// "few"; none — the project file carries no PackageReference at all.
        /// </summary>
        [Fact]
        public void TheManagedProjectHasNoPackageReference()
        {
            string project = Repo.Text("folio-dotnet", "src", "Folio8", "Folio8.csproj");
            Assert.DoesNotContain("<PackageReference", project, StringComparison.Ordinal);
            Assert.Contains("<TargetFramework>netstandard2.0</TargetFramework>", project, StringComparison.Ordinal);
        }

        /// <summary>
        /// The compile-only 4.6 project must compile the SAME sources the
        /// shipped assembly is built from. A stale glob, or a project that
        /// compiles a stub of its own, would prove nothing.
        /// </summary>
        [Fact]
        public void TheNet46CompileProjectCompilesTheShippedSources()
        {
            string project = Repo.Text("folio-dotnet", "test", "Folio8.Net46Compile", "Folio8.Net46Compile.csproj");
            Assert.Contains("net46", project, StringComparison.Ordinal);
            Assert.Contains("../../src/Folio8/**/*.cs", project, StringComparison.Ordinal);
        }

        /// <summary>
        /// The ABI's status codes are written down in exactly one place that
        /// both sides read. If the Go constants and this binding's ever
        /// disagree, every error path silently changes meaning.
        /// </summary>
        [Fact]
        public void TheAbiContractIsDocumented()
        {
            string readme = Repo.Text("folio-go", "cshared", "README.md");
            foreach (string export in new[]
            {
                "folio8_version", "folio8_parse", "folio8_render", "folio8_validate",
                "folio8_parameter_references", "folio8_free", "folio8_allocation_count",
            })
            {
                Assert.Contains(export, readme, StringComparison.Ordinal);
            }
            foreach (string status in new[]
            {
                "FOLIO8_OK", "FOLIO8_ERROR_DIAGNOSTIC", "FOLIO8_ERROR_MESSAGE",
                "FOLIO8_ERROR_ARGUMENT", "FOLIO8_ERROR_PANIC", "FOLIO8_ERROR_UNKNOWN_FREE",
            })
            {
                Assert.Contains(status, readme, StringComparison.Ordinal);
            }
        }
    }
}
