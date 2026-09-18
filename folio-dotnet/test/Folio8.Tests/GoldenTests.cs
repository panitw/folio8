using System.Collections.Generic;
using System.IO;
using System.Text.Json;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// Byte identity with the Go engine, which is the acceptance bar rather
    /// than a goal: the SHA-256 of a render through folio-dotnet must equal
    /// the hash the golden corpus already committed for the same inputs. The
    /// expected value is READ from <c>expected.json</c>, never restated here.
    /// <para>
    /// CAP-5, for folio-dotnet: the WHOLE renderable corpus, not a hand-picked
    /// subset. The fixture list comes from
    /// <c>folio-js/test/data/go-corpus.json</c>, which
    /// <c>folio8-go/wasm/cmd/render/corpus_test.go</c> derives from Go and
    /// holds equal to it — every <c>fixtures/</c> directory is either in that
    /// manifest or excluded there with a stated reason, so a new fixture
    /// cannot go uncovered. It is the same manifest folio-js drives, so the
    /// two bindings cannot diverge on WHAT they prove.
    /// </para>
    /// <para>
    /// This class runs in every folio-dotnet leg — net48 and modern .NET, 64-
    /// and 32-bit, plus the Linux host — so a byte that moves under one
    /// runtime alone reds exactly that leg.
    /// </para>
    /// </summary>
    public class GoldenTests
    {
        /// <summary>Every fixture the manifest records, one xUnit case each.</summary>
        public static IEnumerable<object[]> CorpusFixtures()
        {
            foreach (Repo.CorpusFixture fixture in Repo.Corpus)
            {
                yield return new object[] { fixture.Slug };
            }
        }

        /// <summary>The manifest must have been generated from the engine this binding actually loads.</summary>
        [Fact]
        public void WasGeneratedFromTheEngineVersionThisBindingReports()
        {
            Assert.Equal(Folio8.Version, Repo.CorpusVersion);
        }

        /// <summary>
        /// A leg that renders nothing must FAIL rather than pass empty, and a
        /// manifest fixture this suite skips is a failure rather than an
        /// omission. Comparing the manifest's list length to the manifest's own
        /// count would only prove the manifest agrees with itself, so this
        /// RENDERS every fixture and counts the renders that actually happened.
        /// </summary>
        [Fact]
        public void RendersEveryFixtureTheManifestRecords()
        {
            Assert.NotEmpty(Repo.Corpus);
            HashSet<string> slugs = new HashSet<string>();
            int rendered = 0;
            foreach (Repo.CorpusFixture fixture in Repo.Corpus)
            {
                Assert.True(slugs.Add(fixture.Slug), "duplicate fixture in the manifest: " + fixture.Slug);
                Render(fixture);
                rendered++;
            }
            Assert.Equal(Repo.CorpusCount, rendered);
        }

        /// <summary>An excluded fixture with no reason is an unrecorded gap, which is the thing the manifest exists to prevent.</summary>
        [Fact]
        public void EveryExcludedFixtureStatesWhy()
        {
            KeyValuePair<string, string>[] exclusions = Repo.CorpusExclusions();
            // Non-empty first: a vacuous pass over an empty list would report
            // the reason check as green while checking nothing.
            Assert.NotEmpty(exclusions);
            foreach (KeyValuePair<string, string> excluded in exclusions)
            {
                Assert.False(string.IsNullOrWhiteSpace(excluded.Value),
                    "fixtures/" + excluded.Key + " is excluded with no reason");
            }
        }

        /// <summary>
        /// Every fixture in the corpus renders clean today. Asserting that here
        /// means a regenerated manifest that BAKES IN a new warning reds,
        /// instead of both bindings quietly agreeing with it.
        /// </summary>
        [Fact]
        public void EveryRecordedDiagnosticSequenceIsEmpty()
        {
            foreach (Repo.CorpusFixture fixture in Repo.Corpus)
            {
                Assert.True(fixture.Diagnostics.Length == 0,
                    "fixtures/" + fixture.Slug + ": the manifest records a diagnostic; if that is intended, say so deliberately");
            }
        }

        [Theory]
        [MemberData(nameof(CorpusFixtures))]
        public void RendersByteIdenticallyToTheGoldenCorpus(string fixture)
        {
            Repo.CorpusFixture recorded = null;
            foreach (Repo.CorpusFixture candidate in Repo.Corpus)
            {
                if (candidate.Slug == fixture)
                {
                    recorded = candidate;
                    break;
                }
            }
            Assert.NotNull(recorded);

            RenderResult result = Render(recorded);

            // Assert.Equal, not Assert.True with a sentence: a mismatch has to
            // show the two hashes as a diff, which is the only readable form of
            // this failure.
            Assert.Equal(Repo.ExpectedSha256(fixture), Repo.Sha256(result.Bytes));
            Assert.Equal(recorded.Diagnostics, result.Diagnostics);
        }

        /// <summary>
        /// Renders one fixture exactly as the manifest says Go rendered it,
        /// first checking that the tree still agrees with the manifest about
        /// which optional files exist. A data.json added or removed without
        /// regenerating names the fixture here, rather than surfacing later as
        /// an unexplained hash.
        /// </summary>
        private static RenderResult Render(Repo.CorpusFixture fixture)
        {
            Assert.True(File.Exists(Repo.Path_("fixtures", fixture.Slug, "data.json")) == fixture.Data,
                "fixtures/" + fixture.Slug + ": data.json presence differs from the manifest — regenerate go-corpus.json");
            Assert.True(File.Exists(Repo.Path_("fixtures", fixture.Slug, "params.json")) == fixture.Params,
                "fixtures/" + fixture.Slug + ": params.json presence differs from the manifest — regenerate go-corpus.json");

            Template template = Template.Parse(Repo.File_("fixtures", fixture.Slug, "input.folio"));
            Data data = new Data(fixture.Data
                ? Repo.File_("fixtures", fixture.Slug, "data.json")
                : System.Text.Encoding.UTF8.GetBytes("{}"));
            Params parameters = fixture.Params ? new Params(Repo.File_("fixtures", fixture.Slug, "params.json")) : null;

            return Folio8.Render(template, data, parameters, Repo.ShippedFonts);
        }

        /// <summary>
        /// Template.Load is the only API in this library that touches disk on
        /// the caller's behalf, and it must produce the same template
        /// Template.Parse does from the same file.
        /// </summary>
        [Fact]
        public void LoadMatchesParse()
        {
            string expected;
            using (JsonDocument document = JsonDocument.Parse(Repo.Text("fixtures", "colour-strokes", "expected.json")))
            {
                expected = document.RootElement.GetProperty("sha256").GetString();
            }
            Template loaded = Template.Load(Repo.Path_("fixtures", "colour-strokes", "input.folio"));
            RenderResult result = Folio8.Render(loaded, new Data(Repo.File_("fixtures", "colour-strokes", "data.json")), null, Repo.ShippedFonts);
            Assert.Equal(expected, Repo.Sha256(result.Bytes));
        }

        /// <summary>
        /// A template producing no diagnostics reports an EMPTY list, never
        /// null — the engine's "empty is nil, one representation" rule,
        /// carried across the boundary.
        /// </summary>
        [Fact]
        public void EmptyDiagnosticsAreEmptyAndNotNull()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "colour-strokes", "input.folio"));
            RenderResult result = Folio8.Render(template, new Data(Repo.File_("fixtures", "colour-strokes", "data.json")), null, Repo.ShippedFonts);
            Assert.NotNull(result.Diagnostics);
            Assert.Empty(result.Diagnostics);

            System.Collections.Generic.IList<Diagnostic> validated = Folio8.Validate(
                Repo.File_("fixtures", "colour-strokes", "input.folio"),
                new Data(Repo.File_("fixtures", "colour-strokes", "data.json")),
                null,
                Repo.ShippedFonts);
            Assert.NotNull(validated);
            Assert.Empty(validated);
        }

        /// <summary>The engine version this binding reports is the one the fixtures were recorded under.</summary>
        [Fact]
        public void ReportsTheEngineVersion()
        {
            using (JsonDocument document = JsonDocument.Parse(Repo.Text("folio-js", "test", "data", "go-parity.json")))
            {
                Assert.Equal(document.RootElement.GetProperty("folio8Version").GetString(), Folio8.Version);
            }
        }
    }
}
