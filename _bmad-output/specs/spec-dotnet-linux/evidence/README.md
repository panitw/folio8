# The spike probe, preserved

**This is the throwaway probe from the 2026-09-22 spike, kept as the record of
how [sigaltstack-findings.md](../sigaltstack-findings.md) was measured. Do not
run it and do not develop it.**

The supported tool is:

```
folio-dotnet/build/probe-signal-stack.sh
folio-dotnet/build/signal-stack-probe/
```

It is a superset of this one — same readings, plus per-row provenance the probe
derives itself (this version would report a Rosetta container as native), the
edge-case handling this version lacks, and a `--self-check` mode. Every
measurement quoted anywhere in this spec should come from **that** tool.

These two files stay because a finding is worth what its method is worth, and
the method that produced the original numbers should remain readable at the
version that produced them. `SPEC-dotnet-linux` story 1 landed the supported
tool; nothing here is a second source of truth for anything.
