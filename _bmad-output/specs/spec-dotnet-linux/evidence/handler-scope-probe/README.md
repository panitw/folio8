# The handler-scope probe

Answers one question, by reading rather than by crashing: **when a Go
`c-shared` library is loaded into a .NET process, whose signal handlers are
installed, and on which threads?**

```
dotnet run -c Release -- /path/to/libfolio8_native.so
```

It reads `sigaction(sig, NULL, &old)` for `SIGSEGV`, `SIGBUS`, `SIGURG`,
`SIGPROF` and `SIGABRT` three times — before `dlopen`, after `dlopen`, and
after one crossing — attributing every handler address against
`/proc/self/maps`, and then reads `sigaltstack(NULL, &old)` on the main
thread, a `.NET TP Worker` and a `new Thread()`.

**What it found on 2026-09-23** (.NET 10.0.11, Debian-family WSL2, real
amd64): `dlopen` **alone** replaces `SIGSEGV`, `SIGBUS` and `SIGURG`
process-wide with handlers inside `libfolio8_native.so`, all `SA_ONSTACK`,
before any crossing; Go additionally *adds* `SA_ONSTACK` to the CLR's own
`SIGABRT` handler. Every .NET thread still carries a 16384-byte alternate
signal stack. Go sizes its own for 32768.

That is why routing crossings through binding-owned threads does not fix
DW-396: the exposed threads are the ones that never cross. See the
2026-09-23 subsection of DW-396.

It is a diagnostic, not a test, and is wired into no CI job.
