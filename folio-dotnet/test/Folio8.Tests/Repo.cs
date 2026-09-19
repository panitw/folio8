using System;
using System.Collections.Generic;
using System.IO;
using System.Security.Cryptography;
using System.Text.Json;

namespace Folio8Tests
{
    /// <summary>
    /// Where the fixtures, the recorded Go expectations and the shipped faces
    /// live, and how to read them. Everything these tests compare against is
    /// committed evidence produced by the Go engine — never a value restated
    /// here.
    /// </summary>
    internal static class Repo
    {
        private static readonly Lazy<string> RootPath = new Lazy<string>(FindRoot);
        private static readonly Lazy<FontSet> Shipped = new Lazy<FontSet>(LoadShippedFaces);

        /// <summary>The repository root, found by walking up from the test assembly.</summary>
        internal static string Root
        {
            get { return RootPath.Value; }
        }

        private static string FindRoot()
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
            throw new InvalidOperationException(
                "folio-dotnet tests: could not find the repository root above " + AppContext.BaseDirectory);
        }

        internal static string Path_(params string[] parts)
        {
            string path = Root;
            foreach (string part in parts)
            {
                path = System.IO.Path.Combine(path, part);
            }
            return path;
        }

        internal static byte[] File_(params string[] parts)
        {
            return File.ReadAllBytes(Path_(parts));
        }

        internal static string Text(params string[] parts)
        {
            return File.ReadAllText(Path_(parts));
        }

        /// <summary>
        /// One renderable fixture as <c>folio-go</c> recorded it into
        /// <c>folio-js/test/data/go-corpus.json</c>. It carries NO hash by
        /// design: <c>expected.json</c> stays the single source for every
        /// digest, and <see cref="ExpectedSha256"/> is what reads it.
        /// </summary>
        internal sealed class CorpusFixture
        {
            internal string Slug;
            internal bool Data;
            internal bool Params;
            internal Diagnostic[] Diagnostics;
        }

        /// <summary>The manifest, parsed once. Every accessor below reads this.</summary>
        private sealed class CorpusManifest
        {
            internal string Version;
            internal int Count;
            internal CorpusFixture[] Fixtures;
            internal KeyValuePair<string, string>[] Exclusions;
        }

        private static readonly Lazy<CorpusManifest> Manifest = new Lazy<CorpusManifest>(LoadCorpus);

        /// <summary>
        /// The corpus conformance manifest, derived from Go by
        /// <c>folio-go/wasm/cmd/render/corpus_test.go</c> and held equal to it
        /// there. Both bindings drive their byte-identity suites from this one
        /// file, so neither can quietly narrow its fixture list.
        /// </summary>
        internal static CorpusFixture[] Corpus
        {
            get { return Manifest.Value.Fixtures; }
        }

        /// <summary>The manifest's own record of how many fixtures it carries.</summary>
        internal static int CorpusCount
        {
            get { return Manifest.Value.Count; }
        }

        /// <summary>The engine version the manifest was generated from.</summary>
        internal static string CorpusVersion
        {
            get { return Manifest.Value.Version; }
        }

        /// <summary>Every excluded fixture, with the reason it is out.</summary>
        internal static KeyValuePair<string, string>[] CorpusExclusions()
        {
            return Manifest.Value.Exclusions;
        }

        /// <summary>
        /// Go writes exactly "warning" or "error". Anything else is a manifest
        /// this binding does not understand, and quietly reading it as a
        /// warning would turn an error-severity diagnostic into a passing test.
        /// </summary>
        private static Severity ParseSeverity(string value)
        {
            if (value == "warning")
            {
                return Severity.Warning;
            }
            if (value == "error")
            {
                return Severity.Error;
            }
            throw new InvalidOperationException(
                "go-corpus.json: unrecognised diagnostic severity \"" + value + "\"; expected \"warning\" or \"error\"");
        }

        private static CorpusManifest LoadCorpus()
        {
            List<CorpusFixture> fixtures = new List<CorpusFixture>();
            List<KeyValuePair<string, string>> excluded = new List<KeyValuePair<string, string>>();
            string version;
            int count;
            using (JsonDocument document = JsonDocument.Parse(Text("folio-js", "test", "data", "go-corpus.json")))
            {
                version = document.RootElement.GetProperty("folio8Version").GetString();
                count = document.RootElement.GetProperty("count").GetInt32();
                foreach (JsonElement entry in document.RootElement.GetProperty("fixtures").EnumerateArray())
                {
                    List<Diagnostic> diagnostics = new List<Diagnostic>();
                    foreach (JsonElement d in entry.GetProperty("diagnostics").EnumerateArray())
                    {
                        diagnostics.Add(new Diagnostic(
                            ParseSeverity(d.GetProperty("severity").GetString()),
                            d.GetProperty("code").GetString(),
                            d.GetProperty("elementId").GetString(),
                            d.GetProperty("dataPath").GetString(),
                            d.GetProperty("message").GetString()));
                    }
                    fixtures.Add(new CorpusFixture
                    {
                        Slug = entry.GetProperty("slug").GetString(),
                        Data = entry.GetProperty("data").GetBoolean(),
                        Params = entry.GetProperty("params").GetBoolean(),
                        Diagnostics = diagnostics.ToArray(),
                    });
                }
                foreach (JsonElement entry in document.RootElement.GetProperty("excluded").EnumerateArray())
                {
                    excluded.Add(new KeyValuePair<string, string>(
                        entry.GetProperty("slug").GetString(),
                        entry.GetProperty("reason").GetString()));
                }
            }
            return new CorpusManifest
            {
                Version = version,
                Count = count,
                Fixtures = fixtures.ToArray(),
                Exclusions = excluded.ToArray(),
            };
        }

        /// <summary>
        /// The committed hash for a fixture, READ from its own
        /// <c>expected.json</c> — never restated in this repository's .NET
        /// sources.
        /// </summary>
        internal static string ExpectedSha256(string slug)
        {
            using (JsonDocument document = JsonDocument.Parse(Text("fixtures", slug, "expected.json")))
            {
                return document.RootElement.GetProperty("sha256").GetString();
            }
        }

        internal static string Sha256(byte[] bytes)
        {
            using (SHA256 hash = SHA256.Create())
            {
                byte[] digest = hash.ComputeHash(bytes);
                char[] hex = new char[digest.Length * 2];
                const string alphabet = "0123456789abcdef";
                for (int i = 0; i < digest.Length; i++)
                {
                    hex[i * 2] = alphabet[digest[i] >> 4];
                    hex[i * 2 + 1] = alphabet[digest[i] & 0xF];
                }
                return new string(hex);
            }
        }

        /// <summary>
        /// The eleven faces <c>fonts.Shipped()</c> returns, as
        /// <c>folio-go/fonts/fonts.go</c> names them — the same table
        /// folio-js keeps in <c>scripts/faces.mjs</c>, read straight from the
        /// Go tree rather than from a package that does not exist yet
        /// (embedding is story 7).
        /// <para>
        /// This table cannot drift silently: ParityTests checks every entry,
        /// by name and byte length, against the <c>shippedFaces</c> block the
        /// Go engine itself recorded into
        /// <c>folio-js/test/data/go-parity.json</c>.
        /// </para>
        /// </summary>
        internal static readonly KeyValuePair<string, string>[] ShippedFaceFiles =
        {
            new KeyValuePair<string, string>("Noto Sans", "notosans/NotoSans-Regular.ttf"),
            new KeyValuePair<string, string>("Noto Sans Bold", "notosans-bold/NotoSans-Bold.ttf"),
            new KeyValuePair<string, string>("Noto Sans Italic", "notosans-italic/NotoSans-Italic.ttf"),
            new KeyValuePair<string, string>("Noto Sans Bold Italic", "notosans-bolditalic/NotoSans-BoldItalic.ttf"),
            new KeyValuePair<string, string>("Noto Sans Thai", "notosansthai/NotoSansThai-Regular.ttf"),
            new KeyValuePair<string, string>("Noto Sans Thai Bold", "notosansthai-bold/NotoSansThai-Bold.ttf"),
            new KeyValuePair<string, string>("Noto Sans SC", "notosanssc/NotoSansSC-Regular.ttf"),
            new KeyValuePair<string, string>("Roboto", "roboto/Roboto-Regular.ttf"),
            new KeyValuePair<string, string>("Roboto Bold", "roboto-bold/Roboto-Bold.ttf"),
            new KeyValuePair<string, string>("Roboto Italic", "roboto-italic/Roboto-Italic.ttf"),
            new KeyValuePair<string, string>("Roboto Bold Italic", "roboto-bolditalic/Roboto-BoldItalic.ttf"),
        };

        /// <summary>The shipped font set, loaded once per test process.</summary>
        internal static FontSet ShippedFonts
        {
            get { return Shipped.Value; }
        }

        private static FontSet LoadShippedFaces()
        {
            FontSet set = new FontSet();
            foreach (KeyValuePair<string, string> face in ShippedFaceFiles)
            {
                string[] segments = face.Value.Split('/');
                set.Add(face.Key, File_("folio-go", "fonts", segments[0], segments[1]));
            }
            return set;
        }
    }
}
