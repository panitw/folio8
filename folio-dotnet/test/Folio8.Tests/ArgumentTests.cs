using System;
using System.Collections.Generic;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// The bad-argument row of the I/O matrix: every one of these is refused
    /// in managed code, BEFORE the native call, so a caller mistake never
    /// reaches the ABI and never depends on the engine to catch it.
    /// </summary>
    public class ArgumentTests
    {
        private static Template Fixture()
        {
            return Template.Parse(Repo.File_("fixtures", "colour-strokes", "input.folio"));
        }

        private static Data FixtureData()
        {
            return new Data(Repo.File_("fixtures", "colour-strokes", "data.json"));
        }

        [Fact]
        public void RenderRefusesANullTemplate()
        {
            Assert.Throws<ArgumentNullException>(() => Folio8.Render(null, FixtureData(), null, Repo.ShippedFonts));
        }

        [Fact]
        public void RenderRefusesNullData()
        {
            Assert.Throws<ArgumentNullException>(() => Folio8.Render(Fixture(), null, null, Repo.ShippedFonts));
        }

        [Fact]
        public void RenderRefusesNullFonts()
        {
            Assert.Throws<ArgumentNullException>(() => Folio8.Render(Fixture(), FixtureData(), null, null));
        }

        /// <summary>
        /// There is no default font set and no ambient lookup, so an empty set
        /// is a caller error rather than a fallback.
        /// </summary>
        [Fact]
        public void RenderRefusesAnEmptyFontSet()
        {
            Assert.Throws<ArgumentException>(() => Folio8.Render(Fixture(), FixtureData(), null, new FontSet()));
        }

        [Fact]
        public void ValidateRefusesNullTemplateBytes()
        {
            Assert.Throws<ArgumentNullException>(() => Folio8.Validate(null, FixtureData(), null, Repo.ShippedFonts));
        }

        [Fact]
        public void ValidateRefusesAnEmptyFontSet()
        {
            Assert.Throws<ArgumentException>(() => Folio8.Validate(Repo.File_("fixtures", "colour-strokes", "input.folio"), FixtureData(), null, new FontSet()));
        }

        [Fact]
        public void ParseRefusesNullBytes()
        {
            Assert.Throws<ArgumentNullException>(() => Template.Parse(null));
        }

        [Fact]
        public void LoadRefusesANullPath()
        {
            Assert.Throws<ArgumentNullException>(() => Template.Load(null));
        }

        [Fact]
        public void RenderToRefusesANullStream()
        {
            Assert.Throws<ArgumentNullException>(() => Folio8.RenderTo(null, Fixture(), FixtureData(), null, Repo.ShippedFonts));
        }

        /// <summary>
        /// Invalid UTF-8 is refused where the caller can see it, not decoded
        /// lossily and handed to the engine as different bytes.
        /// </summary>
        [Fact]
        public void DataRefusesInvalidUtf8()
        {
            // 0x80 is a continuation byte with nothing to continue.
            ArgumentException thrown = Assert.Throws<ArgumentException>(() => new Data(new byte[] { 0x7B, 0x80, 0x7D }));
            Assert.Contains("UTF-8", thrown.Message);
        }

        [Fact]
        public void ParamsRefusesInvalidUtf8()
        {
            Assert.Throws<ArgumentException>(() => new Params(new byte[] { 0xC3, 0x28 }));
        }

        [Fact]
        public void DataAndParamsRefuseNull()
        {
            Assert.Throws<ArgumentNullException>(() => new Data((byte[])null));
            Assert.Throws<ArgumentNullException>(() => new Data((string)null));
            Assert.Throws<ArgumentNullException>(() => new Params((byte[])null));
            Assert.Throws<ArgumentNullException>(() => new Params((string)null));
        }

        /// <summary>
        /// Data holds a COPY: mutating the caller's array after construction
        /// cannot change what the engine sees.
        /// </summary>
        [Fact]
        public void DataCopiesItsInput()
        {
            byte[] json = Repo.File_("fixtures", "colour-strokes", "data.json");
            Data data = new Data(json);
            Array.Clear(json, 0, json.Length);

            RenderResult result = Folio8.Render(Fixture(), data, null, Repo.ShippedFonts);

            string expected;
            using (System.Text.Json.JsonDocument document = System.Text.Json.JsonDocument.Parse(Repo.Text("fixtures", "colour-strokes", "expected.json")))
            {
                expected = document.RootElement.GetProperty("sha256").GetString();
            }
            Assert.Equal(expected, Repo.Sha256(result.Bytes));
        }

        /// <summary>
        /// A string that cannot be encoded as UTF-8 is refused, exactly as
        /// bytes that cannot be decoded are. The default UTF8 encoder would
        /// substitute U+FFFD and say nothing — and the implicit conversion
        /// from string makes that the DEFAULT way into this library, so the
        /// lossy path would have been the common one.
        /// </summary>
        [Fact]
        public void DataAndParamsRefuseAStringThatCannotBeEncoded()
        {
            // A high surrogate with no low surrogate after it.
            string unpaired = "{\"a\":\"" + (char)0xD800 + "\"}";
            ArgumentException fromData = Assert.Throws<ArgumentException>(() => new Data(unpaired));
            Assert.Contains("UTF-8", fromData.Message);
            Assert.Throws<ArgumentException>(() => new Params(unpaired));
            // And through the implicit conversion, which is the common path.
            Assert.Throws<ArgumentException>(() => { Data implicitly = unpaired; Assert.NotNull(implicitly); });
        }

        /// <summary>
        /// A font set COPIES a face on the way in, like Data, Params and
        /// Template. Without that, a caller mutating the array after Add would
        /// change what the engine renders.
        /// </summary>
        [Fact]
        public void FontSetCopiesEachFace()
        {
            byte[] face = Repo.File_("folio-go", "fonts", "roboto", "Roboto-Regular.ttf");
            FontSet fonts = new FontSet();
            fonts.Add("Roboto", face);
            byte[] stored = fonts["Roboto"];

            Assert.NotSame(face, stored);
            Array.Clear(face, 0, face.Length);
            Assert.NotEqual(face, stored);

            // The indexer setter copies too.
            byte[] replacement = new byte[] { 1, 2, 3 };
            fonts["Roboto"] = replacement;
            replacement[0] = 9;
            Assert.Equal(new byte[] { 1, 2, 3 }, fonts["Roboto"]);
        }

        /// <summary>
        /// CopyTo throws the ICollection contract's exceptions BEFORE writing
        /// anything, rather than an IndexOutOfRangeException part-way through.
        /// </summary>
        [Fact]
        public void FontSetCopyToRefusesADestinationItDoesNotFit()
        {
            FontSet fonts = new FontSet();
            fonts.Add("Alpha", new byte[] { 1 });
            fonts.Add("Bravo", new byte[] { 2 });

            Assert.Throws<ArgumentNullException>(() => fonts.CopyTo(null, 0));
            Assert.Throws<ArgumentOutOfRangeException>(() => fonts.CopyTo(new KeyValuePair<string, byte[]>[2], -1));

            KeyValuePair<string, byte[]>[] tooSmall = new KeyValuePair<string, byte[]>[1];
            Assert.Throws<ArgumentException>(() => fonts.CopyTo(tooSmall, 0));
            // Nothing was written before the refusal.
            Assert.Null(tooSmall[0].Key);
        }

        /// <summary>A face name may not be empty, and face bytes may not be null.</summary>
        [Fact]
        public void FontSetRefusesDegenerateEntries()
        {
            FontSet fonts = new FontSet();
            Assert.Throws<ArgumentNullException>(() => fonts.Add(null, new byte[1]));
            Assert.Throws<ArgumentException>(() => fonts.Add(string.Empty, new byte[1]));
            Assert.Throws<ArgumentNullException>(() => fonts.Add("Roboto", null));
        }

        /// <summary>A font set enumerates in insertion order, not hash order.</summary>
        [Fact]
        public void FontSetKeepsCallerOrder()
        {
            FontSet fonts = new FontSet();
            fonts.Add("Zulu", new byte[] { 1 });
            fonts.Add("Alpha", new byte[] { 2 });
            fonts.Add("Mike", new byte[] { 3 });
            Assert.Equal(new[] { "Zulu", "Alpha", "Mike" }, fonts.Keys);
        }
    }
}
