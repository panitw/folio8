using System;
using System.Collections.Generic;
using System.Text.Json;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// <c>Fonts.Shipped()</c> is the package's promise that an installer can
    /// render without finding fonts of their own. These check it against the
    /// engine itself — the bytes in <c>folio8-go/fonts/</c> and the
    /// <c>shippedFaces</c> record the Go engine wrote into
    /// <c>folio-js/test/data/go-parity.json</c> — never against a value
    /// restated here.
    /// </summary>
    public class FontsTests
    {
        /// <summary>Eleven faces, and exactly the eleven the engine ships.</summary>
        [Fact]
        public void ShipsTheSameElevenFacesTheEngineDoes()
        {
            FontSet shipped = Fonts.Shipped();
            Dictionary<string, long> recorded = RecordedFaces();

            Assert.Equal(11, recorded.Count);
            Assert.Equal(recorded.Count, shipped.Count);
            foreach (KeyValuePair<string, long> face in recorded)
            {
                Assert.True(shipped.ContainsKey(face.Key), "Fonts.Shipped() is missing the face '" + face.Key + "'");
                Assert.Equal(face.Value, shipped[face.Key].Length);
            }
        }

        /// <summary>
        /// Not merely the right LENGTHS: the right BYTES. Compared against
        /// folio8-go/fonts/ directly, which is where the pack step reads them
        /// from and where the engine embeds them from.
        /// </summary>
        [Fact]
        public void EveryFaceIsByteIdenticalToTheEnginesCopy()
        {
            FontSet shipped = Fonts.Shipped();
            foreach (KeyValuePair<string, string> face in Repo.ShippedFaceFiles)
            {
                string[] segments = face.Value.Split('/');
                byte[] fromGo = Repo.File_("folio8-go", "fonts", segments[0], segments[1]);
                Assert.True(shipped.ContainsKey(face.Key), "Fonts.Shipped() is missing the face '" + face.Key + "'");
                Assert.Equal(Repo.Sha256(fromGo), Repo.Sha256(shipped[face.Key]));
            }
        }

        /// <summary>
        /// Calling it twice reads once — and hands back two INDEPENDENT sets,
        /// so one caller emptying or replacing what it was given cannot change
        /// what the next caller renders with.
        /// </summary>
        [Fact]
        public void RepeatCallsAreIsolatedFromEachOther()
        {
            FontSet first = Fonts.Shipped();
            FontSet second = Fonts.Shipped();
            Assert.NotSame(first, second);

            first.Remove("Roboto");
            first["Noto Sans"] = new byte[] { 1, 2, 3 };

            FontSet third = Fonts.Shipped();
            Assert.True(second.ContainsKey("Roboto"));
            Assert.True(third.ContainsKey("Roboto"));
            Assert.Equal(11, third.Count);
            Assert.NotEqual(3, third["Noto Sans"].Length);
        }

        /// <summary>
        /// THE POINT OF EMBEDDING THEM: a caller who installs the package and
        /// nothing else renders the golden corpus byte-identically. Same
        /// fixture and same committed hash the corpus tests use, with the
        /// package's own faces instead of files read out of the repository.
        /// </summary>
        [Theory]
        [InlineData("colour-strokes")]
        [InlineData("justified-thai")]
        [InlineData("shaped-text")]
        public void RendersTheGoldenCorpusWithTheShippedSetAlone(string fixture)
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

            RenderResult result = Folio8.Render(template, data, parameters, Fonts.Shipped());

            Assert.Equal(expected, Repo.Sha256(result.Bytes));
        }

        private static byte[] Optional(string fixture, string name)
        {
            string path = Repo.Path_("fixtures", fixture, name);
            return System.IO.File.Exists(path) ? System.IO.File.ReadAllBytes(path) : null;
        }

        private static Dictionary<string, long> RecordedFaces()
        {
            Dictionary<string, long> faces = new Dictionary<string, long>(StringComparer.Ordinal);
            using (JsonDocument document = JsonDocument.Parse(Repo.Text("folio-js", "test", "data", "go-parity.json")))
            {
                foreach (JsonElement face in document.RootElement.GetProperty("shippedFaces").EnumerateArray())
                {
                    faces[face.GetProperty("name").GetString()] = face.GetProperty("byteLength").GetInt64();
                }
            }
            return faces;
        }
    }
}
