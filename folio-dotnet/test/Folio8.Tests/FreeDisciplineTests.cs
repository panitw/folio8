using System;
using System.Text;
using Xunit;

namespace Folio8Tests
{
    /// <summary>
    /// Ownership across the boundary, proved rather than asserted. The engine
    /// owns what it allocates and hands back a token; the binding returns it
    /// with exactly one free, on every path including the error paths. The ABI
    /// exports the size of its allocation table so this can be MEASURED over a
    /// loop.
    /// </summary>
    [Collection("native")]
    public class FreeDisciplineTests
    {
        [Fact]
        public void TheAllocationTableDoesNotGrowOverACorpusLoop()
        {
            // Warm the lazily-loaded font set first, so its cost is not
            // mistaken for a leak, and take the baseline after one full pass.
            Run();
            int before = Folio8.OutstandingNativeAllocations();

            for (int i = 0; i < 5; i++)
            {
                Run();
            }

            Assert.Equal(before, Folio8.OutstandingNativeAllocations());
        }

        [Fact]
        public void FailedCallsFreeTheirBuffersToo()
        {
            int before = Folio8.OutstandingNativeAllocations();
            for (int i = 0; i < 5; i++)
            {
                Assert.Throws<FolioRenderException>(() => Template.Parse(Encoding.UTF8.GetBytes("{")));
                Assert.Throws<FolioRenderException>(() => Folio8.Render(
                    Template.Parse(Repo.File_("fixtures", "wrapped-text", "input.folio")),
                    new Data(Encoding.UTF8.GetBytes("{}")),
                    null,
                    Repo.ShippedFonts));
            }
            Assert.Equal(before, Folio8.OutstandingNativeAllocations());
        }

        /// <summary>
        /// A second free of one token is REFUSED, not a crash. This is the one
        /// test that goes at the ABI directly, because the managed surface has
        /// no way to free a token twice — which is the point.
        /// </summary>
        [Fact]
        public void ASecondFreeIsRefused()
        {
            // THROUGH THE ENGINE THREADS, like every other crossing. This test
            // goes at the ABI directly because the managed surface has no way
            // to free a token twice — but "directly" means past the funnel,
            // not past the pool: on Linux a call made from the xunit thread
            // enters the Go engine carrying the runtime's small alternate
            // signal stack, which is DW-396 itself. A suite that is meant to
            // be the evidence must not contain the trigger.
            EngineThreads.Run(delegate
            {
                ulong token;
                IntPtr result;
                int length;
                Assert.Equal(Native.StatusOk, Native.folio8_version(out token, out result, out length));
                Assert.NotEqual(IntPtr.Zero, result);
                Assert.True(length > 0);

                Assert.Equal(Native.StatusOk, Native.folio8_free(token));
                Assert.Equal(Native.StatusErrorUnknownFree, Native.folio8_free(token));
            });
        }

        /// <summary>
        /// The loaded library speaks the ABI this assembly was built against.
        /// The binding checks this once before its first call; asserting it
        /// here means a native library rebuilt with a bumped abiVersion, and a
        /// managed side not updated to match, reddens on the spot instead of
        /// on whichever call first decodes a moved frame grammar.
        /// </summary>
        [Fact]
        public void TheLoadedLibrarySpeaksTheExpectedAbi()
        {
            Assert.Equal(Native.ExpectedAbiVersion, EngineThreads.Run(new Func<int>(Native.folio8_abi_version)));
        }

        /// <summary>An unknown token is refused the same way, not acted on.</summary>
        [Fact]
        public void AnUnknownTokenIsRefused()
        {
            EngineThreads.Run(delegate
            {
                Assert.Equal(Native.StatusErrorUnknownFree, Native.folio8_free(0));
                Assert.Equal(Native.StatusErrorUnknownFree, Native.folio8_free(ulong.MaxValue));
            });
        }

        private static void Run()
        {
            foreach (string fixture in new[] { "colour-strokes", "alternating-rows", "statement-5" })
            {
                Template template = Template.Parse(Repo.File_("fixtures", fixture, "input.folio"));
                Data data = new Data(Repo.File_("fixtures", fixture, "data.json"));
                Params parameters = System.IO.File.Exists(Repo.Path_("fixtures", fixture, "params.json"))
                    ? new Params(Repo.File_("fixtures", fixture, "params.json"))
                    : null;
                Folio8.Render(template, data, parameters, Repo.ShippedFonts);
                Folio8.Validate(Repo.File_("fixtures", fixture, "input.folio"), data, parameters, Repo.ShippedFonts);
                template.ParameterReferences();
            }
        }
    }
}
