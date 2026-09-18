using Xunit;

// Serial, deliberately. FreeDisciplineTests MEASURES the engine's allocation
// table across a loop, and a render running in parallel in another collection
// would hold a buffer of its own while the measurement is taken — turning a
// real ownership guarantee into a flaky one. Renders are CPU-bound anyway, so
// parallelism buys little here.
[assembly: CollectionBehavior(DisableTestParallelization = true)]
