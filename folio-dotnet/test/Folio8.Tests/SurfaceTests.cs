using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
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
        /// EVERY CROSSING GOES THROUGH THE ENGINE THREADS. A direct
        /// <c>folio8_*</c> call from anywhere else enters the Go engine on
        /// whatever thread the caller happened to be on — which, on Linux, is
        /// a thread carrying the runtime's small alternate signal stack, and
        /// that is DW-396 exactly. It is invisible to every other test: the
        /// call returns the right answer and the process crashes later, or on
        /// someone else's machine.
        /// </summary>
        /// <remarks>
        /// The rule is checked by reading the sources because it is a
        /// standing one: a future call site added in good faith, with the
        /// whole suite green, is precisely how the two Linux RIDs would be
        /// withdrawn a second time.
        /// </remarks>
        [Fact]
        public void EveryNativeEntryPointIsReachedOnlyThroughTheEngineThreads()
        {
            Regex names = new Regex(@"\bfolio8_[a-z_]+\b");
            int checkedSites = 0;
            foreach (string file in SourceFiles())
            {
                string where = Path.GetFileName(file);
                foreach (string statement in Statements(File.ReadAllText(file)))
                {
                    if (!names.IsMatch(statement))
                    {
                        continue;
                    }
                    // The DllImport declarations themselves are the contract,
                    // not a call.
                    if (statement.Contains("extern"))
                    {
                        continue;
                    }
                    checkedSites++;
                    if (statement.Contains("Native.Invoke(") || statement.Contains("EngineThreads.Run("))
                    {
                        continue;
                    }
                    // The two sites INSIDE the pooled path itself. Native.cs
                    // reaches the ABI bare in exactly these two places, and
                    // both are only ever entered from an engine thread.
                    if (where == "Native.cs" &&
                        (statement == "actual = folio8_abi_version()" || statement == "int freed = folio8_free(token)"))
                    {
                        continue;
                    }
                    Assert.Fail(
                        where + " reaches the native ABI outside the engine threads, which is how DW-396 crashes a host: " + statement);
                }
            }
            // VACUITY GUARD: a scan that matched nothing is not a pass.
            Assert.True(checkedSites >= 8, "the scan found only " + checkedSites + " native call sites, so it is not reading the sources it thinks it is");
        }

        /// <summary>
        /// The source with its comments and string literals removed, split
        /// into statements. Stripping matters: a doc comment may NAME an
        /// entry point, and an error message may list all eight of them.
        /// </summary>
        private static IEnumerable<string> Statements(string source)
        {
            StringBuilder code = new StringBuilder(source.Length);
            for (int i = 0; i < source.Length; i++)
            {
                char c = source[i];
                if (c == '/' && i + 1 < source.Length && source[i + 1] == '/')
                {
                    while (i < source.Length && source[i] != '\n') { i++; }
                    code.Append(' ');
                }
                else if (c == '/' && i + 1 < source.Length && source[i + 1] == '*')
                {
                    i += 2;
                    while (i + 1 < source.Length && !(source[i] == '*' && source[i + 1] == '/')) { i++; }
                    i++;
                    code.Append(' ');
                }
                else if (c == '@' && i + 1 < source.Length && source[i + 1] == '"')
                {
                    i += 2;
                    while (i < source.Length)
                    {
                        if (source[i] == '"')
                        {
                            if (i + 1 < source.Length && source[i + 1] == '"') { i++; }
                            else { break; }
                        }
                        i++;
                    }
                    code.Append('"');
                }
                else if (c == '"')
                {
                    i++;
                    while (i < source.Length && source[i] != '"')
                    {
                        if (source[i] == '\\') { i++; }
                        i++;
                    }
                    code.Append('"');
                }
                else if (c == '\'')
                {
                    i++;
                    while (i < source.Length && source[i] != '\'')
                    {
                        if (source[i] == '\\') { i++; }
                        i++;
                    }
                    code.Append('\'');
                }
                else
                {
                    code.Append(c);
                }
            }

            // Split on braces as well as semicolons, so a statement is
            // bounded by its own block: `try { actual = folio8_abi_version()`
            // would otherwise read as one chunk with the `try` glued on.
            Regex whitespace = new Regex(@"\s+");
            foreach (string chunk in code.ToString().Split(';', '{', '}'))
            {
                yield return whitespace.Replace(chunk, " ").Trim();
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
