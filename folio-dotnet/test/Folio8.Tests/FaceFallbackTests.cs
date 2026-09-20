using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// The face-fallback selector through the managed binding: the same
    /// behaviour Go and Node ship, reached through the C ABI.
    /// </summary>
    /// <remarks>
    /// The template is built here rather than read from <c>fixtures/</c>
    /// because no committed fixture names a face nobody supplies — which is
    /// the whole condition this capability exists for. It is the same document
    /// the Go tests use.
    /// </remarks>
    public class FaceFallbackTests
    {
        private const string BrandChainTemplate = @"{
  ""assets"": {},
  ""bands"": {
    ""content"": {
      ""elements"": [
        {""id"": ""e1"", ""type"": ""text"", ""x"": 0, ""y"": 0, ""width"": 500, ""height"": 20, ""value"": ""{{name}}"", ""style"": {""fontFamily"": ""body"", ""fontSize"": 14}}
      ]
    },
    ""pageFooter"": {""elements"": [], ""height"": 20},
    ""pageHeader"": {""elements"": [], ""height"": 20}
  },
  ""fonts"": {""body"": [""Brand Face""]},
  ""locale"": ""en"",
  ""nextId"": 2,
  ""page"": {""margin"": {""bottom"": 36, ""left"": 36, ""right"": 36, ""top"": 36}, ""orientation"": ""portrait"", ""size"": ""A4""},
  ""utcOffset"": ""+00:00"",
  ""version"": ""1.0""
}
";

        private static Template BrandChain()
        {
            return Template.Parse(Encoding.UTF8.GetBytes(BrandChainTemplate));
        }

        private static Data Name()
        {
            return new Data(@"{""name"":""Hi""}");
        }

        /// <summary>
        /// The default is strict, so nothing an existing caller wrote changes
        /// behaviour.
        /// </summary>
        [Fact]
        public void TheDefaultStillRefusesAnAbsentFace()
        {
            FolioRenderException thrown = Assert.Throws<FolioRenderException>(
                () => Folio8.Render(BrandChain(), Name(), null, Repo.ShippedFonts));
            Assert.Equal("TEXT_FACE_ABSENT", thrown.Diagnostic.Code);
        }

        [Fact]
        public void SubstitutePaintsAPoolFaceAndSaysSo()
        {
            RenderResult result = Folio8.Render(BrandChain(), Name(), null, Repo.ShippedFonts, FaceFallback.Substitute);
            Assert.NotEmpty(result.Bytes);

            List<Diagnostic> substituted = new List<Diagnostic>();
            foreach (Diagnostic d in result.Diagnostics)
            {
                if (d.Code == "TEXT_FACE_SUBSTITUTED")
                {
                    substituted.Add(d);
                }
            }
            Assert.NotEmpty(substituted);
            foreach (Diagnostic d in substituted)
            {
                Assert.Equal(Severity.Warning, d.Severity);
                Assert.Equal("e1", d.ElementId);
                Assert.Contains("Brand Face", d.Message);
                Assert.Contains("painted in ", d.Message);
            }
        }

        /// <summary>
        /// Dropping <c>fallback</c> from the RenderTo wrapper leaves every
        /// other test green while a lenient caller gets a refusal, so the
        /// forwarding is pinned directly.
        /// </summary>
        [Fact]
        public void RenderToCarriesTheSelector()
        {
            MemoryStream destination = new MemoryStream();
            IList<Diagnostic> diagnostics = Folio8.RenderTo(
                destination, BrandChain(), Name(), null, Repo.ShippedFonts, FaceFallback.Substitute);

            Assert.NotEmpty(destination.ToArray());
            bool saw = false;
            foreach (Diagnostic d in diagnostics)
            {
                if (d.Code == "TEXT_FACE_SUBSTITUTED")
                {
                    saw = true;
                }
            }
            Assert.True(saw, "RenderTo returned no substitution warning");

            Assert.Throws<FolioRenderException>(
                () => Folio8.RenderTo(new MemoryStream(), BrandChain(), Name(), null, Repo.ShippedFonts));
        }

        [Fact]
        public void ValidateCarriesTheSelector()
        {
            byte[] template = Encoding.UTF8.GetBytes(BrandChainTemplate);
            IList<Diagnostic> found = Folio8.Validate(template, Name(), null, Repo.ShippedFonts, FaceFallback.Substitute);
            bool saw = false;
            foreach (Diagnostic d in found)
            {
                if (d.Code == "TEXT_FACE_SUBSTITUTED")
                {
                    saw = true;
                }
            }
            Assert.True(saw, "Validate returned no substitution warning");

            Assert.Throws<FolioRenderException>(
                () => Folio8.Validate(template, Name(), null, Repo.ShippedFonts));
        }

        /// <summary>
        /// A renderer holding nothing that covers the character still refuses,
        /// under either selector: the selector asks for a SUBSTITUTE, not for
        /// the character to be dropped.
        /// </summary>
        [Fact]
        public void SubstituteWithNoCandidateStillRefuses()
        {
            FolioRenderException thrown = Assert.Throws<FolioRenderException>(
                () => Folio8.Render(BrandChain(), Name(), null, new FontSet(), FaceFallback.Substitute));
            Assert.Equal("TEXT_FACE_ABSENT", thrown.Diagnostic.Code);
        }
    }
}
