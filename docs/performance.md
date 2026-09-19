# Performance

This page records what the folio8 engine actually costs to run: how many documents a second it
produces, and how much memory a host needs to produce them. Every figure comes from the golden
fixture corpus in [`fixtures/`](../fixtures/), rendered through the same public `Render` entry point
the [rendering library guide](rendering-library.md) documents.

Three findings shape everything else. **Throughput is set by pages, not documents** — one core
sustains roughly 430 pages a second whatever the document size. **The engine retains nothing**, so
peak memory is bought per concurrent worker rather than per document and a long-running server does
not drift. **`Render` is safe to call concurrently** against one shared template and font set, which
is what makes a worker pool the natural shape for a publishing host.

These are measurements on one machine, not a guarantee. Read the ratios — pages per second per core,
megabytes per worker — rather than the absolute numbers, and re-measure on the hardware you intend
to run.

## Conditions

| | |
| --- | --- |
| Machine | Apple M4 — 4 performance + 6 efficiency cores, 24 GB |
| Operating system | macOS 26.6.2 |
| Toolchain | go1.26.0 darwin/arm64 |
| Engine | folio-go at `1559227`, `fonts.Shipped()` — 11 embedded faces |
| `GOGC` | 100 (Go's default) unless a figure says otherwise |

Measured on a shared desktop at load average 2–3.6, not an isolated host.

## What is measured

Every fixture that carries a `data.json` — twelve documents that exercise the real bind path, from a
four-kilobyte Thai bill-payment slip to a fifty-page account statement.

The template is parsed once, **outside** the measured loop, because that is how a publishing server
works. `ParseTemplate` costs 0.15 ms for the statement template against a 4.7 ms render, and a server
pays it at startup rather than per document. What is inside the loop is everything that varies per
document: the data JSON decode, binding, layout, pagination, shaping and PDF serialization.

Two harnesses produce the numbers. `folio-go/render_throughput_bench_test.go` gives per-document time
and allocation. Peak resident set comes from a separate probe process, because `ru_maxrss` is a
process-wide high-water mark and two workloads in one process cannot be told apart.

```sh
cd folio-go
go test -run '^$' -bench 'BenchmarkRenderThroughput$'       -benchmem -count 5 -cpu 1 .
go test -run '^$' -bench 'BenchmarkRenderThroughputParallel' -benchmem -count 5 .
go test -run '^$' -bench 'BenchmarkLoadTemplate'             -benchmem -count 3 -cpu 1 .
```

The benchmarks report `ns/op`, and documents a second is `1e9/ns_op`. They deliberately report no
`docs/s` metric of their own: `b.ReportMetric` takes a `float64`, and `float64` is banned under the
`folio-go` module root by AD-23.

Under `RunParallel`, Go's `ns/op` is wall time divided by total iterations — the reciprocal of
aggregate throughput, **not** per-document latency. The latency figures on this page come from the
serial runs.

## Throughput

The `statement-N` family is one template over 1, 5, 20 and 50 statements' worth of rows, so it
separates per-document cost from per-page cost. Documents per second falls steeply across it, 212
down to 8.7. Pages per second climbs and then flattens.

| Fixture | Pages | Pages/s, 1 core | Pages/s, 10 cores |
| --- | --- | --- | --- |
| `statement-1` | 1 | 212 | 881 |
| `statement-5` | 5 | 350 | 1,668 |
| `statement-20` | 20 | 415 | 1,985 |
| `statement-50` | 50 | 436 | 2,144 |

That flat line is the capacity number worth planning against. A one-page document is not ten times
cheaper than a ten-page one; it is about four times cheaper, because roughly half of a small
document's cost is fixed.

Per-document time is linear in page count — **2.25 ms per page plus 2.5 ms of fixed cost**. Nothing
about pagination degrades as documents grow, which is the property that matters for a corpus with a
long tail of large statements.

### Concurrency

Ten goroutines rendering against the same `*Template` and the same `FontSet` give a **4.8× speedup**
on the heavier fixtures and pass under the race detector. On a machine with four performance cores
that is close to the ceiling the hardware allows rather than evidence of contention — the six
efficiency cores contribute the remainder.

Latency degrades under that load. A one-page statement that takes 4.1 ms alone takes 10.6 ms when ten
workers compete. Throughput and latency are separate budgets: a synchronous request path wants fewer
workers than a batch run does.

### A note on very small documents

`barcode-thai-bill-payment` renders in 25 µs and does *not* speed up with more workers (0.95×). A CPU
profile of that run puts over 70% of samples in `runtime.usleep`, `madvise` and thread parking — the
Go scheduler and scavenger, not the engine — and repeated runs varied between 37,000 and 58,000
documents a second. Documents under about 100 µs are below this harness's resolution; do not read a
capacity number out of that row.

## Memory

The engine retains nothing between documents. After rendering four hundred documents, live heap sits
at 5–12 MB whichever fixture ran, the same order as an idle process. Everything a render allocates is
short-lived garbage, so peak resident set is **GC headroom under concurrency** and it scales with
worker count far more than with document size.

| Fixture | Peak RSS, 1 worker | Peak RSS, 10 workers |
| --- | --- | --- |
| `statement-1` | 24.0 MB | 113.0 MB |
| `statement-5` | 27.6 MB | 124.4 MB |
| `statement-20` | 28.8 MB | 158.6 MB |
| `statement-50` | 36.9 MB | 234.6 MB |

Baseline after warm-up is 18–34 MB, most of it the eleven embedded font faces. A sizing rule that
fits these numbers: **about 10 MB of resident set per concurrent worker per 10 pages of document**,
on top of a ~20 MB floor.

### Allocation is the cost centre

Per document the engine allocates roughly **2.2 MB and 20,000 objects per page**, plus about 4.5 MB
and 64,000 objects of fixed cost. A fifty-page statement churns 112 MB across 1.06 million
allocations to produce a 542 KB PDF, a ratio of about 200:1. None of it is retained, so it costs GC
time rather than footprint — but it is what caps the per-core page rate.

The contrast inside the corpus locates that cost precisely. `barcode-thai-bill-payment` lays out no
text and allocates 95 KB in 192 objects. `qrcode-payments`, also one page, allocates 1.8 MB in 320.
The moment a document shapes text, allocation counts rise by three orders of magnitude.

### Trading memory for throughput

Because the workload is nearly pure garbage, raising `GOGC` trades resident set for throughput
directly. The trade stops paying at 400.

| `GOGC` | Docs/s | Peak RSS |
| --- | --- | --- |
| 100 (default) | 340.6 | 114.5 MB |
| 400 | 438.1 (+29%) | 409.4 MB (3.6×) |
| 800 | 444.3 (+30%) | 723.3 MB (6.3×) |

`statement-5`, 400 documents, 10 workers. On one worker the same change is worth only +8% (79.9 →
86.6 docs/s) for 43 MB instead of 27 MB. `GOGC=400` is worth setting on a batch host with memory to
spare; on a memory-capped container it is not, and `GOMEMLIMIT` is the better lever.

## The full corpus

Median of five runs. Serial is `-cpu 1`; parallel is `GOMAXPROCS=10`. Allocation figures are per
document and are identical in both modes.

| Fixture | Pages | ms/doc | Docs/s, 1 core | Docs/s, 10 cores | Speed-up | Alloc/doc | Allocs/doc | PDF |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `statement-1` | 1 | 4.71 | 212.2 | 881.0 | 4.15× | 6.74 MB | 83,544 | 74 KB |
| `statement-5` | 5 | 14.27 | 70.1 | 333.5 | 4.76× | 15.50 MB | 165,494 | 124 KB |
| `statement-20` | 20 | 48.15 | 20.8 | 99.3 | 4.78× | 47.11 MB | 464,929 | 263 KB |
| `statement-50` | 50 | 114.69 | 8.7 | 42.9 | 4.92× | 112.30 MB | 1,064,269 | 542 KB |
| `multi-page-statement` | 4 | 19.42 | 51.5 | 234.3 | 4.55× | 19.81 MB | 222,335 | 127 KB |
| `multi-page-flow` | 4 | 14.01 | 71.4 | 322.6 | 4.52× | 14.78 MB | 170,639 | 107 KB |
| `section-break-statement` | 2 | 11.55 | 86.6 | 429.2 | 4.96× | 9.39 MB | 121,822 | 94 KB |
| `section-break-unanchored` | 1 | 10.57 | 94.6 | 461.3 | 4.88× | 8.81 MB | 116,206 | 91 KB |
| `alternating-rows` | 1 | 2.39 | 417.7 | 1,616 | 3.87× | 3.81 MB | 68,137 | 54 KB |
| `colour-strokes` | 1 | 2.92 | 342.9 | 1,354 | 3.95× | 3.97 MB | 79,755 | 40 KB |
| `qrcode-payments` | 1 | 1.25 | 799.7 | 2,195 | 2.74× | 1.82 MB | 320 | 90 KB |
| `barcode-thai-bill-payment` | 1 | 0.025 | 39,422 | 37,484 | 0.95× | 95 KB | 192 | 4 KB |

Template parsing, which a server pays once rather than per document, ranges from 0.023 ms
(`barcode-thai-bill-payment`, an 18 KB template) to 0.36 ms (`multi-page-flow`, 258 KB). It never
belongs in a per-document budget.

## Sizing a publishing host

- **Size in pages.** One core sustains about 430 pages a second on dense statement work; ten threads
  sustain about 2,100. Document counts alone will mislead you by an order of magnitude between a
  one-page slip and a fifty-page statement.
- **Parse once, share the template.** `Render` is race-clean against a shared `*Template` and
  `FontSet`. Re-parsing per request adds about 3% to a small document for nothing.
- **Budget about 20 MB per concurrent worker** for ordinary documents and about 25 MB for fifty-page
  ones, on top of a ~20 MB floor. Nothing accumulates across documents.
- **Separate the two budgets.** Full-concurrency throughput costs about 2.5× the per-document
  latency. Size a request path for latency and a batch path for throughput.
- **The optimisation target is allocation.** Twenty thousand objects per page is what sets the
  per-core ceiling. Reducing it would move throughput more than any other single change; `GOGC` is
  the cheap version of that lever today.

Source of truth: this file. The published page at `docs/performance.html` carries the same text.
