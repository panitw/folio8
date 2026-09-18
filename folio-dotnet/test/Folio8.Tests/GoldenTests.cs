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
    /// </summary>
    public class GoldenTests
    {
        // The fixtures whose Go golden test renders with exactly
        // fonts.Shipped() — the same five folio-js's golden suite uses, so the
        // two bindings are held to one list. justified-thai and shaped-text
        // exercise Thai shaping, statement-5 passes params, colour-strokes
        // covers colour.
        [Theory]
        [InlineData("colour-strokes")]
        [InlineData("justified-thai")]
        [InlineData("shaped-text")]
        [InlineData("statement-5")]
        [InlineData("alternating-rows")]
        public void RendersByteIdenticallyToTheGoldenCorpus(string fixture)
        {
            string expected;
            using (JsonDocument document = JsonDocument.Parse(Repo.Text("fixtures", fixture, "expected.json")))
            {
                expected = document.RootElement.GetProperty("sha256").GetString();
            }

            Template template = Template.Parse(Repo.File_("fixtures", fixture, "input.folio"));
            Data data = new Data(Optional(fixture, "data.json") ?? System.Text.Encoding.UTF8.GetBytes("{}"));
            byte[] rawParams = Optional(fixture, "params.json");
            Params parameters = rawParams == null ? null : new Params(rawParams);

            RenderResult result = Folio8.Render(template, data, parameters, Repo.ShippedFonts);

            Assert.Equal(expected, Repo.Sha256(result.Bytes));
            Assert.Empty(result.Diagnostics);
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

        private static byte[] Optional(string fixture, string name)
        {
            string path = Repo.Path_("fixtures", fixture, name);
            return File.Exists(path) ? File.ReadAllBytes(path) : null;
        }
    }
}
