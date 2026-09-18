using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text;
using System.Text.RegularExpressions;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// CAP-9: docs/folio-dotnet.md and docs/folio-dotnet.html teach THIS
    /// library, and are held to it rather than trusted to keep up with it.
    /// <para>
    /// Three claims, each executed: the two twins agree in reader-visible text,
    /// punctuation included; every public item of the shipped assembly is named
    /// in a code span or a signature block of both twins; and the guide's
    /// first-PDF program is the very text README.md carries.
    /// </para>
    /// </summary>
    public class DocsTests
    {
        private const string Begin = "<!-- twin:begin -->";
        private const string End = "<!-- twin:end -->";

        private static readonly string Markdown = Repo.Path_("docs", "folio-dotnet.md");
        private static readonly string Page = Repo.Path_("docs", "folio-dotnet.html");

        private static string Read(string path)
        {
            string text = File.ReadAllText(path);
            Assert.False(text.Length == 0, path + " is empty");
            return text;
        }

        private static bool IsHtml(string path)
        {
            return path.EndsWith(".html", StringComparison.Ordinal);
        }

        /// <summary>
        /// The region both twins publish, between the markers. Only the page's
        /// chrome may live outside it — a test below holds that, so prose cannot
        /// be authored where the comparison does not reach.
        /// </summary>
        private static string TwinRegion(string path)
        {
            string text = Read(path);
            int begin = text.IndexOf(Begin, StringComparison.Ordinal);
            int end = text.IndexOf(End, StringComparison.Ordinal);
            Assert.True(begin >= 0 && end > begin, path + " has no twin:begin/twin:end region");
            return Regex.Replace(text.Substring(begin, end - begin), "<!--.*?-->", " ", RegexOptions.Singleline);
        }

        private static string Prologue(string path)
        {
            string text = Read(path);
            return text.Substring(0, text.IndexOf(Begin, StringComparison.Ordinal));
        }

        private static string Epilogue(string path)
        {
            string text = Read(path);
            int end = text.IndexOf(End, StringComparison.Ordinal);
            return text.Substring(end + End.Length);
        }

        private static string Decoded(string html)
        {
            return html
                .Replace("&lt;", "<")
                .Replace("&gt;", ">")
                .Replace("&quot;", "\"")
                .Replace("&#39;", "'")
                .Replace("&amp;", "&");
        }

        /// <summary>
        /// The Markdown region as a reader sees it: the format's own syntax —
        /// fences and their info strings, table pipes and rules, heading hashes,
        /// list bullets, emphasis and code ticks, and every link target —
        /// removed, and NOTHING else. Lines inside a fence are kept verbatim,
        /// because a code sample's backticks, pipes and dashes are content.
        /// </summary>
        private static string MarkdownText(string region)
        {
            StringBuilder text = new StringBuilder();
            bool fence = false;
            foreach (string raw in region.Split('\n'))
            {
                if (Regex.IsMatch(raw, "^\\s*```"))
                {
                    fence = !fence;
                    continue;
                }
                if (fence)
                {
                    text.Append(raw).Append('\n');
                    continue;
                }
                string line = raw;
                if (Regex.IsMatch(line, "^\\s*\\|[\\s:|-]*\\|\\s*$") && line.IndexOf('-') >= 0)
                {
                    continue;
                }
                line = Regex.Replace(line, @"\[([^\]]*)\]\([^)]*\)", "$1");
                line = line.Replace("`", string.Empty).Replace("**", string.Empty);
                line = Regex.Replace(line, "^\\s{0,3}#{1,6}\\s+", string.Empty);
                line = Regex.Replace(line, "^\\s*[-*+]\\s+", " ");
                if (line.TrimStart().StartsWith("|", StringComparison.Ordinal))
                {
                    line = line.Replace("|", " ");
                }
                text.Append(line).Append('\n');
            }
            return text.ToString();
        }

        /// <summary>
        /// The reader-visible token stream: words AND single punctuation marks,
        /// so that <c>IList&lt;Diagnostic&gt;</c> and <c>IList&lt;string&gt;</c>
        /// are different pages. Whitespace alone is not a token, because the two
        /// formats wrap lines differently.
        /// </summary>
        private static string[] ReaderTokens(string path)
        {
            string region = TwinRegion(path);
            string text = IsHtml(path) ? Decoded(Regex.Replace(region, "<[^>]*>", " ")) : MarkdownText(region);
            return Regex.Matches(text, "[A-Za-z0-9]+|[^\\sA-Za-z0-9]").Cast<Match>().Select(m => m.Value).ToArray();
        }

        /// <summary>
        /// Only what the page sets in code: every inline code span and every
        /// code sample of the twin region, concatenated. The surface check reads
        /// THIS rather than the prose, because names like <c>Count</c>,
        /// <c>Values</c>, <c>Add</c>, <c>Message</c> and <c>Error</c> occur in
        /// ordinary English and would be satisfied by a sentence that documents
        /// nothing.
        /// </summary>
        private static string CodeText(string path)
        {
            string region = TwinRegion(path);
            StringBuilder code = new StringBuilder();
            if (IsHtml(path))
            {
                foreach (Match m in Regex.Matches(region, "<code>(.*?)</code>|<pre[^>]*>(.*?)</pre>", RegexOptions.Singleline))
                {
                    string inner = m.Groups[1].Success ? m.Groups[1].Value : m.Groups[2].Value;
                    code.Append(Decoded(Regex.Replace(inner, "<[^>]*>", string.Empty))).Append('\n');
                }
            }
            else
            {
                bool fence = false;
                foreach (string line in region.Split('\n'))
                {
                    if (Regex.IsMatch(line, "^\\s*```"))
                    {
                        fence = !fence;
                        continue;
                    }
                    if (fence)
                    {
                        code.Append(line).Append('\n');
                        continue;
                    }
                    foreach (Match m in Regex.Matches(line, "`([^`]*)`"))
                    {
                        code.Append(m.Groups[1].Value).Append('\n');
                    }
                }
            }
            return code.ToString();
        }

        /// <summary>
        /// Every public name of the shipped assembly, read by reflection rather
        /// than restated here: the exported types, and the members each type
        /// introduces. Constructors share their type's name; property accessors
        /// and conversion operators are compiler spellings of a member already
        /// counted; an indexer has no name a guide can print; and an override of
        /// a member declared OUTSIDE this assembly (ToString, Equals(object),
        /// GetHashCode, GetObjectData) is the framework's contract rather than
        /// this library's. An override of a member this library itself declares
        /// is this library's surface and is kept.
        /// </summary>
        private static string[] PublicNames()
        {
            SortedSet<string> names = new SortedSet<string>(StringComparer.Ordinal);
            Assembly assembly = typeof(Template).Assembly;
            foreach (Type type in assembly.GetExportedTypes())
            {
                names.Add(type.Name);
                foreach (MemberInfo member in type.GetMembers(
                             BindingFlags.Public | BindingFlags.Instance | BindingFlags.Static | BindingFlags.DeclaredOnly))
                {
                    if (member is ConstructorInfo || member is Type)
                    {
                        continue;
                    }
                    MethodInfo method = member as MethodInfo;
                    if (method != null)
                    {
                        if (method.IsSpecialName)
                        {
                            continue;
                        }
                        MethodInfo baseDefinition = method.GetBaseDefinition();
                        if (baseDefinition.DeclaringType != method.DeclaringType &&
                            baseDefinition.DeclaringType.Assembly != assembly)
                        {
                            continue;
                        }
                    }
                    PropertyInfo property = member as PropertyInfo;
                    if (property != null && property.GetIndexParameters().Length > 0)
                    {
                        continue;
                    }
                    FieldInfo field = member as FieldInfo;
                    if (field != null && field.Name == "value__")
                    {
                        continue;
                    }
                    names.Add(member.Name);
                }
            }
            return names.ToArray();
        }

        /// <summary>The first ```csharp block of README.md — the documented first-PDF program.</summary>
        private static string ReadmeSnippet()
        {
            Match match = Regex.Match(Repo.Text("folio-dotnet", "README.md"), "```csharp\n(.*?)```", RegexOptions.Singleline);
            Assert.True(match.Success, "folio-dotnet/README.md has no ```csharp block — the documented first-PDF program is gone");
            return match.Groups[1].Value.Trim();
        }

        /// <summary>The first code sample of the guide's "Your first PDF" section, as text.</summary>
        private static string FirstPdfSample()
        {
            Match section = Regex.Match(Read(Page), "<section id=\"your-first-pdf\">(.*?)</section>", RegexOptions.Singleline);
            Assert.True(section.Success, "docs/folio-dotnet.html has no your-first-pdf section");
            Match sample = Regex.Match(section.Groups[1].Value, "<div class=\"sample\"><pre[^>]*>(.*?)</pre></div>", RegexOptions.Singleline);
            Assert.True(sample.Success, "docs/folio-dotnet.html: the your-first-pdf section carries no code sample");
            return Decoded(Regex.Replace(sample.Groups[1].Value, "<[^>]*>", string.Empty)).Trim();
        }

        [Fact]
        public void TheTwinsPublishTheSameTextPunctuationIncluded()
        {
            string[] md = ReaderTokens(Markdown);
            string[] html = ReaderTokens(Page);
            // VACUITY GUARD: two empty regions are not two agreeing twins.
            Assert.True(md.Length > 1500, "docs/folio-dotnet.md carries almost no prose (" + md.Length + " tokens)");
            int i = 0;
            while (i < md.Length && i < html.Length && md[i] == html[i])
            {
                i++;
            }
            if (i != md.Length || i != html.Length)
            {
                Assert.Fail("docs/folio-dotnet.md and docs/folio-dotnet.html diverge at token " + i
                    + "\n  .md:   " + Context(md, i) + "\n  .html: " + Context(html, i));
            }
        }

        private static string Context(string[] tokens, int at)
        {
            int from = Math.Max(0, at - 20);
            int to = Math.Min(tokens.Length, at + 20);
            return string.Join(" ", tokens.Skip(from).Take(to - from));
        }

        [Fact]
        public void NothingButPageChromeLivesOutsideTheComparedRegion()
        {
            // Without this, a paragraph written above twin:begin or below
            // twin:end would be published on one twin and invisible to the
            // comparison above.
            const string Structural = "<(?:section|h1|h2|h3|h4|table|pre|ul|ol)\\b";
            Assert.False(Regex.IsMatch(Prologue(Page), Structural), "docs/folio-dotnet.html authors content above twin:begin");
            string epilogue = Epilogue(Page);
            Assert.False(Regex.IsMatch(epilogue, Structural), "docs/folio-dotnet.html authors content below twin:end");
            Assert.Single(Regex.Matches(epilogue, "<p\\b"));
            Assert.Contains("class=\"source-note\"", epilogue, StringComparison.Ordinal);

            Assert.Equal(string.Empty, Prologue(Markdown).Trim());
            Assert.False(Regex.IsMatch(Epilogue(Markdown), "^\\s*(?:#|```)", RegexOptions.Multiline),
                "docs/folio-dotnet.md authors a heading or a code block below twin:end");
            Assert.True(Epilogue(Markdown).Trim().Length < 200, "docs/folio-dotnet.md authors prose below twin:end");
        }

        [Fact]
        public void TheGuideNamesEveryPublicItemInCodeInBothTwins()
        {
            string[] names = PublicNames();
            // A reflection scan that found almost nothing would let every
            // assertion below pass without reading a word of the guide.
            Assert.True(names.Length >= 40,
                "the public-surface scan found only " + names.Length + " names: " + string.Join(", ", names));
            foreach (string path in new[] { Markdown, Page })
            {
                string code = CodeText(path);
                Assert.True(code.Length > 2000, Path.GetFileName(path) + " yielded almost no code text (" + code.Length + " characters)");
                foreach (string name in names)
                {
                    Assert.True(Regex.IsMatch(code, @"\b" + Regex.Escape(name) + @"\b"),
                        Path.GetFileName(path) + " does not name the public item " + name + " in a code span or signature block");
                }
            }
        }

        [Fact]
        public void TheGuideShowsTheReadmeFirstPdfProgram()
        {
            string snippet = ReadmeSnippet();
            Assert.Contains("Folio8.Render(", snippet, StringComparison.Ordinal);
            Assert.Contains(snippet, Read(Markdown), StringComparison.Ordinal);
            Assert.Equal(snippet, FirstPdfSample());
        }

        [Fact]
        public void TheGuideReferencesNoRemoteHost()
        {
            foreach (string path in new[] { Markdown, Page })
            {
                Assert.False(Regex.IsMatch(Read(path), "https?://"), Path.GetFileName(path) + " references a remote host");
            }
        }
    }
}
