using System;
using System.Collections.Generic;
using System.Text;
using System.Text.Json;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// Diagnostic parity with Go, replayed from
    /// <c>folio-js/test/data/go-parity.json</c> — which
    /// <c>folio-go/wasm/cmd/render/parity_test.go</c> RECORDS from the Go
    /// engine and holds equal to it. Every expectation here is Go's, and
    /// folio-js replays the same file, so the two bindings are proved equal to
    /// the engine against one set of recorded outcomes rather than two.
    /// </summary>
    public class ParityTests
    {
        private static readonly JsonDocument Parity = JsonDocument.Parse(Repo.Text("folio-js", "test", "data", "go-parity.json"));

        public static IEnumerable<object[]> Cases()
        {
            foreach (JsonElement c in Parity.RootElement.GetProperty("cases").EnumerateArray())
            {
                yield return new object[] { c.GetProperty("name").GetString() };
            }
        }

        /// <summary>
        /// The recorded file must still cover every row the story's I/O matrix
        /// names. A case quietly dropped upstream would otherwise turn this
        /// whole suite vacuous without failing anything.
        /// </summary>
        [Fact]
        public void CoversTheCasesTheIoMatrixNames()
        {
            List<string> names = new List<string>();
            foreach (JsonElement c in Parity.RootElement.GetProperty("cases").EnumerateArray())
            {
                names.Add(c.GetProperty("name").GetString());
            }
            foreach (string required in new[]
            {
                "render with a clip warning",
                "render of an absent data path",
                "render of data that is not JSON",
                "render with params",
                "parse of a malformed template",
                "validate of a clean template",
                "validate with a clip warning",
                "validate of an absent data path",
                "validate of a malformed template",
                "parameter references",
            })
            {
                Assert.Contains(required, names);
            }
        }

        /// <summary>
        /// The font table in Repo.cs is exactly <c>fonts.Shipped()</c>, by name
        /// and by byte length, as the Go engine recorded it. Without this the
        /// golden hashes would be measuring a font set of this test's own
        /// invention.
        /// </summary>
        [Fact]
        public void TheTestFontSetIsExactlyShipped()
        {
            List<string> expected = new List<string>();
            foreach (JsonElement face in Parity.RootElement.GetProperty("shippedFaces").EnumerateArray())
            {
                expected.Add(face.GetProperty("name").GetString() + ":" + face.GetProperty("byteLength").GetInt32());
            }
            expected.Sort(StringComparer.Ordinal);

            List<string> actual = new List<string>();
            foreach (KeyValuePair<string, byte[]> face in Repo.ShippedFonts)
            {
                actual.Add(face.Key + ":" + face.Value.Length);
            }
            actual.Sort(StringComparer.Ordinal);

            Assert.Equal(expected, actual);
        }

        [Theory]
        [MemberData(nameof(Cases))]
        public void MatchesGo(string name)
        {
            JsonElement c = Find(name);
            JsonElement expect = c.GetProperty("expect");

            if (expect.TryGetProperty("error", out JsonElement error))
            {
                Exception thrown = Record.Exception(() => Run(c));
                Assert.NotNull(thrown);
                if (error.TryGetProperty("diagnostic", out JsonElement expectedDiagnostic))
                {
                    FolioRenderException folio = Assert.IsType<FolioRenderException>(thrown);
                    Assert.Equal(Diagnostic(expectedDiagnostic), folio.Diagnostic);
                    // The message is Go's own, unchanged — the same identity
                    // Go's RenderError.Error() keeps with the error it wraps.
                    Assert.Equal(expectedDiagnostic.GetProperty("message").GetString(), folio.Message);
                }
                else
                {
                    // Go returns a plain error here, so this must NOT be a
                    // FolioRenderException — the same split folio-js makes.
                    Assert.IsNotType<FolioRenderException>(thrown);
                    Assert.Equal(error.GetProperty("message").GetString(), thrown.Message);
                }
                return;
            }

            Outcome outcome = Run(c);

            if (expect.TryGetProperty("sha256", out JsonElement sha))
            {
                Assert.Equal(sha.GetString(), Repo.Sha256(outcome.Bytes));
            }
            if (expect.TryGetProperty("diagnostics", out JsonElement diagnostics))
            {
                Assert.Equal(Diagnostics(diagnostics), outcome.Diagnostics);
            }
            if (expect.TryGetProperty("references", out JsonElement references))
            {
                List<string> expectedReferences = new List<string>();
                foreach (JsonElement reference in references.EnumerateArray())
                {
                    expectedReferences.Add(reference.GetString());
                }
                Assert.Equal(expectedReferences, outcome.References);
            }
        }

        /// <summary>
        /// A successful render never carries an error-severity diagnostic:
        /// an error aborts and is thrown instead.
        /// </summary>
        [Fact]
        public void ASuccessfulRenderNeverCarriesAnErrorDiagnostic()
        {
            Outcome outcome = Run(Find("render with a clip warning"));
            Assert.NotEmpty(outcome.Diagnostics);
            Assert.All(outcome.Diagnostics, d => Assert.Equal(Severity.Warning, d.Severity));
        }

        private static JsonElement Find(string name)
        {
            foreach (JsonElement c in Parity.RootElement.GetProperty("cases").EnumerateArray())
            {
                if (c.GetProperty("name").GetString() == name)
                {
                    return c;
                }
            }
            throw new InvalidOperationException("no recorded parity case named " + name);
        }

        private sealed class Outcome
        {
            internal byte[] Bytes = new byte[0];
            internal IList<Diagnostic> Diagnostics = new Diagnostic[0];
            internal IList<string> References = new string[0];
        }

        private static Outcome Run(JsonElement c)
        {
            byte[] template = Bytes(c.GetProperty("template"));
            byte[] data = c.TryGetProperty("data", out JsonElement d) && d.ValueKind != JsonValueKind.Null ? Bytes(d) : null;
            Params parameters = c.TryGetProperty("params", out JsonElement p) && p.ValueKind != JsonValueKind.Null ? new Params(Bytes(p)) : null;

            // A recorded case may carry no data at all. Pass that THROUGH as a
            // null Data so what the test observes is the engine's behaviour;
            // wrapping it would turn the case into an ArgumentNullException
            // from this binding's own argument check and prove nothing about
            // Go.
            Data payload = data == null ? null : new Data(data);

            switch (c.GetProperty("op").GetString())
            {
                case "parse":
                    Template.Parse(template);
                    return new Outcome();
                case "render":
                {
                    RenderResult result = Folio8.Render(Template.Parse(template), payload, parameters, Repo.ShippedFonts);
                    return new Outcome { Bytes = result.Bytes, Diagnostics = result.Diagnostics };
                }
                case "validate":
                    return new Outcome { Diagnostics = Folio8.Validate(template, payload, parameters, Repo.ShippedFonts) };
                case "parameterReferences":
                    return new Outcome { References = Template.Parse(template).ParameterReferences() };
                default:
                    throw new InvalidOperationException("unknown op " + c.GetProperty("op").GetString());
            }
        }

        private static byte[] Bytes(JsonElement input)
        {
            if (input.TryGetProperty("file", out JsonElement file))
            {
                return Repo.File_(file.GetString().Split('/'));
            }
            return Encoding.UTF8.GetBytes(input.TryGetProperty("text", out JsonElement text) ? text.GetString() : string.Empty);
        }

        private static IList<Diagnostic> Diagnostics(JsonElement values)
        {
            List<Diagnostic> list = new List<Diagnostic>();
            foreach (JsonElement value in values.EnumerateArray())
            {
                list.Add(Diagnostic(value));
            }
            return list;
        }

        private static Diagnostic Diagnostic(JsonElement value)
        {
            string severity = value.GetProperty("severity").GetString();
            return new Diagnostic(
                severity == "error" ? Severity.Error : Severity.Warning,
                value.GetProperty("code").GetString(),
                value.GetProperty("elementId").GetString(),
                value.GetProperty("dataPath").GetString(),
                value.GetProperty("message").GetString());
        }
    }
}
