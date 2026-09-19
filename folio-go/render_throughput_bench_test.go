package folio8_test

import (
	"os"
	"path/filepath"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// THE THROUGHPUT BENCHMARKS — WHAT THEY MEASURE AND WHAT THEY DO NOT.
//
// These benchmarks answer one production question: how many documents a
// second does this engine produce, and how much memory does producing
// them cost. They are shaped around the way a publishing server actually
// works — the template is parsed once at startup and the font set is a
// map of embedded byte slices, so neither is inside the measured loop.
// What IS inside the loop is everything that varies per document: the
// data JSON decode, binding, layout, pagination, shaping and PDF
// serialization. That is the per-document cost a caller pays.
//
// They measure nothing about correctness. Every fixture below is already
// pinned byte-for-byte by its own golden test; these benchmarks re-render
// the same inputs and only check that the render did not fail and did not
// produce zero bytes, so a benchmark that silently started returning an
// error could not be read as a speed-up.
//
// Run them with:
//
//	go test -run '^$' -bench 'Throughput' -benchmem ./...
//
// BenchmarkRenderThroughput is serial — one document at a time on one
// goroutine, the honest single-core number. BenchmarkRenderThroughputParallel
// runs GOMAXPROCS goroutines against the SAME *Template and FontSet,
// which is the realistic server shape and also the only place a shared
// mutable field in the template would show up; run it under -race to use
// it as that check.
//
// Both report a docs/sec custom metric alongside ns/op so the number a
// capacity plan needs does not have to be derived by hand.

// throughputFixtures is every fixture in the corpus that carries a
// data.json — that is, every fixture that exercises the bind path a real
// document goes through. The statement-N family is the scaling series
// (the same template over 1, 5, 20 and 50 statements' worth of rows);
// the rest cover the other per-document costs: barcode and QR raster
// generation, Thai shaping, section breaks and flowed multi-page text.
var throughputFixtures = []string{
	"statement-1",
	"statement-5",
	"statement-20",
	"statement-50",
	"multi-page-statement",
	"multi-page-flow",
	"section-break-statement",
	"section-break-unanchored",
	"alternating-rows",
	"colour-strokes",
	"qrcode-payments",
	"barcode-thai-bill-payment",
}

// loadThroughputInputs reads one fixture's template, data and (optional)
// params off disk. It is called outside the measured loop: a benchmark
// that included os.ReadFile would be measuring the filesystem.
func loadThroughputInputs(b *testing.B, name string) (*folio8.Template, folio8.Data, folio8.Params) {
	b.Helper()

	dir := filepath.Join(repoRootFromBench(b), "fixtures", name)

	tpl, err := folio8.LoadTemplate(filepath.Join(dir, "input.folio"))
	if err != nil {
		b.Fatalf("load template %s: %v", name, err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "data.json"))
	if err != nil {
		b.Fatalf("read data %s: %v", name, err)
	}

	var params folio8.Params
	raw, err := os.ReadFile(filepath.Join(dir, "params.json"))
	switch {
	case err == nil:
		params = folio8.Params(raw)
	case os.IsNotExist(err):
		// A fixture without params.json supplies none; that is legal.
	default:
		b.Fatalf("read params %s: %v", name, err)
	}

	return tpl, folio8.Data(data), params
}

// repoRootFromBench is repoRootFromTest for a *testing.B. The two cannot
// be shared through testing.TB because both call t.Helper() and Fatalf on
// their own concrete type, and duplicating six lines is cheaper than a
// wrapper that loses the failure location.
func repoRootFromBench(b *testing.B) string {
	b.Helper()

	dir, err := os.Getwd()
	if err != nil {
		b.Fatalf("getwd: %v", err)
	}
	for {
		if isBenchRepoRootDir(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			b.Fatalf("could not find repo root walking up from %s", dir)
		}
		dir = parent
	}
}

// isBenchRepoRootDir reports whether dir is the folio8 repository root —
// the one directory holding both folio-go/ and fixtures/. This external
// test package cannot see the in-package helper of the same shape, and a
// benchmark that guessed the root by relative path would break the moment
// it was run from anywhere but folio-go/.
func isBenchRepoRootDir(dir string) bool {
	folioGo, err1 := os.Stat(filepath.Join(dir, "folio-go"))
	fixtures, err2 := os.Stat(filepath.Join(dir, "fixtures"))
	return err1 == nil && folioGo.IsDir() && err2 == nil && fixtures.IsDir()
}

// reportDocsPerSecond turns the elapsed wall time of the whole benchmark
// into the throughput figure a capacity plan is written in. It must be
// called after the loop, while b.Elapsed() still covers only the measured
// iterations.
func reportDocsPerSecond(b *testing.B, docs int) {
	b.Helper()

	seconds := b.Elapsed().Seconds()
	if seconds <= 0 {
		return
	}
	b.ReportMetric(float64(docs)/seconds, "docs/s")
}

func BenchmarkRenderThroughput(b *testing.B) {
	faces := fonts.Shipped()

	for _, fixture := range throughputFixtures {
		b.Run(fixture, func(b *testing.B) {
			tpl, data, params := loadThroughputInputs(b, fixture)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				res, err := folio8.Render(tpl, data, params, faces)
				if err != nil {
					b.Fatalf("render %s: %v", fixture, err)
				}
				if len(res.Bytes) == 0 {
					b.Fatalf("render %s produced no bytes", fixture)
				}
			}

			b.StopTimer()
			reportDocsPerSecond(b, b.N)
		})
	}
}

func BenchmarkRenderThroughputParallel(b *testing.B) {
	faces := fonts.Shipped()

	for _, fixture := range throughputFixtures {
		b.Run(fixture, func(b *testing.B) {
			tpl, data, params := loadThroughputInputs(b, fixture)

			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					res, err := folio8.Render(tpl, data, params, faces)
					if err != nil {
						b.Errorf("render %s: %v", fixture, err)
						return
					}
					if len(res.Bytes) == 0 {
						b.Errorf("render %s produced no bytes", fixture)
						return
					}
				}
			})

			b.StopTimer()
			reportDocsPerSecond(b, b.N)
		})
	}
}

// BenchmarkLoadTemplate measures the startup cost the throughput
// benchmarks deliberately exclude: parsing one .folio file into a
// *Template. A server pays this once per template, not once per document,
// and the two figures should never be added together.
func BenchmarkLoadTemplate(b *testing.B) {
	for _, fixture := range throughputFixtures {
		b.Run(fixture, func(b *testing.B) {
			path := filepath.Join(repoRootFromBench(b), "fixtures", fixture, "input.folio")
			raw, err := os.ReadFile(path)
			if err != nil {
				b.Fatalf("read %s: %v", path, err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				tpl, err := folio8.ParseTemplate(raw)
				if err != nil {
					b.Fatalf("parse %s: %v", fixture, err)
				}
				if tpl == nil {
					b.Fatalf("parse %s returned nil template", fixture)
				}
			}
		})
	}
}
