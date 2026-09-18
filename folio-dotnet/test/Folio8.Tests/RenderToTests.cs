using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// RenderTo's discipline: it renders FULLY, then writes once, and it never
    /// closes or disposes the stream the caller handed it. On failure nothing
    /// is written at all — the bytes exist in full before the first one
    /// reaches the stream, so a caller streaming an HTTP response never sends
    /// half a PDF.
    /// </summary>
    public class RenderToTests
    {
        [Fact]
        public void WritesTheSameBytesAsRenderAndLeavesTheStreamOpen()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "colour-strokes", "input.folio"));
            Data data = new Data(Repo.File_("fixtures", "colour-strokes", "data.json"));

            RenderResult direct = Folio8.Render(template, data, null, Repo.ShippedFonts);

            MemoryStream destination = new MemoryStream();
            IList<Diagnostic> diagnostics = Folio8.RenderTo(destination, template, data, null, Repo.ShippedFonts);

            Assert.Equal(direct.Bytes, destination.ToArray());
            Assert.Equal(direct.Diagnostics, diagnostics);

            // Still open: a disposed MemoryStream throws on CanWrite use.
            Assert.True(destination.CanWrite);
            destination.WriteByte(0);
        }

        [Fact]
        public void CarriesWarningsThroughUnchanged()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "wrapped-text", "input.folio"));
            Data data = new Data(Repo.File_("folio-js", "test", "data", "wrapped-text-named.json"));

            MemoryStream destination = new MemoryStream();
            IList<Diagnostic> diagnostics = Folio8.RenderTo(destination, template, data, null, Repo.ShippedFonts);

            Assert.NotEmpty(diagnostics);
            Assert.All(diagnostics, d => Assert.Equal(Severity.Warning, d.Severity));
            Assert.NotEmpty(destination.ToArray());
        }

        [Fact]
        public void WritesNothingWhenTheRenderFails()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "wrapped-text", "input.folio"));
            MemoryStream destination = new MemoryStream();

            Assert.Throws<FolioRenderException>(() =>
                Folio8.RenderTo(destination, template, new Data(Encoding.UTF8.GetBytes("{}")), null, Repo.ShippedFonts));

            Assert.Empty(destination.ToArray());
            Assert.True(destination.CanWrite);
        }

        [Fact]
        public void RefusesAStreamItCannotWriteTo()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "colour-strokes", "input.folio"));
            using (MemoryStream readOnly = new MemoryStream(new byte[4], false))
            {
                Assert.Throws<ArgumentException>(() =>
                    Folio8.RenderTo(readOnly, template, new Data(Repo.File_("fixtures", "colour-strokes", "data.json")), null, Repo.ShippedFonts));
            }
        }

        /// <summary>
        /// A render error arrives as a throw carrying Go's diagnostic — code,
        /// element id, data path — not as an entry in a result.
        /// </summary>
        [Fact]
        public void ARenderErrorCarriesGosDiagnostic()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "wrapped-text", "input.folio"));
            FolioRenderException thrown = Assert.Throws<FolioRenderException>(() =>
                Folio8.Render(template, new Data(Encoding.UTF8.GetBytes("{}")), null, Repo.ShippedFonts));

            Assert.Equal(Severity.Error, thrown.Diagnostic.Severity);
            Assert.Equal("BINDING_PATH_ABSENT", thrown.Diagnostic.Code);
            Assert.Equal("e4", thrown.Diagnostic.ElementId);
            Assert.Equal("customer.name", thrown.Diagnostic.DataPath);
            Assert.Equal(thrown.Diagnostic.Message, thrown.Message);
        }

        /// <summary>
        /// Params reach the engine: a template that reads documentDate names
        /// it in ParameterReferences, and a statement fixture renders to its
        /// committed hash only when its params arrive.
        /// </summary>
        [Fact]
        public void ParameterReferencesNamesWhatTheTemplateReads()
        {
            Template template = Template.Parse(Repo.File_("folio-js", "test", "data", "document-date.folio"));
            Assert.Equal(new[] { "documentDate" }, template.ParameterReferences());
        }

        [Fact]
        public void ParameterReferencesIsEmptyRatherThanNull()
        {
            Template template = Template.Parse(Repo.File_("fixtures", "colour-strokes", "input.folio"));
            IList<string> references = template.ParameterReferences();
            Assert.NotNull(references);
            Assert.Empty(references);
        }
    }
}
