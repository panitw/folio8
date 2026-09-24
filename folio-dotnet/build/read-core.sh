#!/usr/bin/env bash
#
# READ ONE CORE THE WAY DW-398 NEEDED IT READ.
#
#   ./read-core.sh <core> <executable> <out-dir>
#
# gdb with libcoreclr's symbols fetched (dotnet-symbol, placed beside the .so
# so the debuglink lookup finds it), every thread's backtrace, the fault
# itself (signal, si_addr, rip/rsp/fs_base, the instructions at rip), the
# futex errno if this is the futex abort, and every thread's alternate stack
# measured from the ucontext the kernel handed its handler. Writes
# <out-dir>/backtrace-<core>.txt and prints a summary. Best-effort at every
# step: a missing tool degrades the reading, it does not lose the core.
#
# Lifted from crash-trace.sh so load-repro can read a core too; crash-trace
# still carries its own copy. (Making it call this is a small change left
# for a quieter moment.)
set -euo pipefail
core="$1"; exe="$2"; out="$3"
mkdir -p "$out"
work="$(mktemp -d)"
export PATH="$HOME/.dotnet/tools:$PATH"

# Symbols beside libcoreclr.so, if dotnet-symbol can fetch them.
symbolised=no
fw_dir="$(dotnet --list-runtimes 2>/dev/null | awk '/Microsoft.NETCore.App/{print $3}' | tr -d '[]' | tail -1)"
fw_ver="$(dotnet --list-runtimes 2>/dev/null | awk '/Microsoft.NETCore.App/{print $2}' | tail -1)"
coreclr="$fw_dir/$fw_ver/libcoreclr.so"
if [ -f "$coreclr" ] && [ ! -f "$(dirname "$coreclr")/libcoreclr.so.dbg" ]; then
  command -v dotnet-symbol >/dev/null 2>&1 || dotnet tool install -g dotnet-symbol >/dev/null 2>&1 || true
  if command -v dotnet-symbol >/dev/null 2>&1; then
    mkdir -p "$work/syms"
    if dotnet-symbol --symbols -o "$work/syms" "$coreclr" >"$work/symbol.log" 2>&1 && [ -f "$work/syms/libcoreclr.so.dbg" ]; then
      sudo -n cp "$work/syms/libcoreclr.so.dbg" "$(dirname "$coreclr")/" 2>/dev/null && symbolised=yes
    fi
  fi
fi
[ -f "$(dirname "$coreclr")/libcoreclr.so.dbg" ] && symbolised=yes

cat >"$work/futex-errno.py" <<'PYEOF'
import gdb
print("===THE FUTEX ERRNO===")
try:
    gdb.execute("thread 1", to_string=True)
    f = gdb.newest_frame(); hit = None
    while f is not None:
        n = f.name() or ""
        if "futex_abstimed_wait_common" in n:
            hit = f; break
        f = f.older()
    if hit is None:
        print("no __futex_abstimed_wait_common frame on the faulting thread: this death is NOT the futex abort")
    else:
        hit.select()
        try: print("futex errno: err = %s" % hit.read_var("err"))
        except Exception as e: print("frame found but `err` not readable: %s" % e)
except Exception as e:
    print("errno probe failed: %s" % e)
PYEOF

cat >"$work/altstack.py" <<'PYEOF'
import gdb
print("===THE ALTERNATE STACK, FROM uc_stack, EVERY THREAD===")
HANDLERS = ("inject_activation_handler", "sigsegv_handler", "sigabrt_handler", "sigfpe_handler", "sigbus_handler", "sigill_handler", "sigtrap_handler")
try:
    inf = gdb.selected_inferior()
    rows = []
    for th in sorted(inf.threads(), key=lambda t: t.num):
        th.switch()
        rsp = int(gdb.parse_and_eval("$rsp"))
        f = gdb.newest_frame(); hit = None
        while f is not None:
            n = f.name() or ""
            if any(h in n for h in HANDLERS):
                hit = f; break
            f = f.older()
        if hit is None:
            continue
        hit.select()
        ctx = None
        for cand in ("context", "ucontext"):
            try: ctx = hit.read_var(cand); break
            except Exception: pass
        if ctx is None:
            rows.append((th.num, hit.name(), None)); continue
        try:
            uc = gdb.parse_and_eval("*(ucontext_t*)%s" % ctx)
            ss = uc["uc_stack"]
            rows.append((th.num, hit.name(), (int(ss["ss_sp"]), int(ss["ss_size"]), int(ss["ss_flags"]), rsp, int(ctx))))
        except Exception as e:
            rows.append((th.num, hit.name(), "unreadable: %s" % e))
    if not rows:
        print("no thread is inside a PAL signal handler; nothing to measure")
    for num, name, info in rows:
        if info is None:
            print("Thread %d  %s: no readable context argument" % (num, name)); continue
        if isinstance(info, str):
            print("Thread %d  %s: %s" % (num, name, info)); continue
        sp, size, flags, rsp, ucaddr = info
        top = sp + size
        verdict = "OVERFLOWED by %d bytes -- THIS THREAD RAN OFF ITS ALTSTACK" % (sp - rsp) if rsp < sp else "%d bytes remained" % (rsp - sp)
        print("Thread %d  %s" % (num, name))
        print("    altstack ss_sp=0x%x ss_size=%d ss_flags=%d%s" % (sp, size, flags, "  (SS_ONSTACK at delivery)" if flags & 1 else ""))
        print("    rsp now 0x%x: %d of %d bytes used, %s" % (rsp, top - rsp, size, verdict))
        print("    kernel frame: ucontext at 0x%x, %d bytes below the top" % (ucaddr, top - ucaddr))
    gdb.execute("thread 1", to_string=True)
except Exception as e:
    print("altstack probe failed: %s" % e)
PYEOF

bt="$out/backtrace-$(basename "$core").txt"
gdb -q -batch -ex "set pagination off" -ex "set debuginfod enabled on" \
    -ex "thread apply all bt" \
    -ex "echo \n===FAULTING THREAD, WITH LOCALS===\n" -ex "thread 1" -ex "bt full 12" \
    -ex "echo \n===THE FAULT ITSELF===\n" \
    -ex "p \$_siginfo.si_signo" -ex "p \$_siginfo.si_code" -ex "p/x \$_siginfo._sifields._sigfault.si_addr" \
    -ex "p/x \$rip" -ex "p/x \$rsp" -ex "p/x \$fs_base" -ex "x/10i \$pc-24" -ex "echo \n" \
    -x "$work/futex-errno.py" -x "$work/altstack.py" \
    -ex "echo \n===MAPPINGS===\n" -ex "info proc mappings" \
    -ex "echo \n===REGISTERS===\n" -ex "info registers" \
    "$exe" --core="$core" >"$bt" 2>"$out/gdb-$(basename "$core").err" || true

set +o pipefail
echo "=== core: $(basename "$core") ($(du -h "$core" | cut -f1)); libcoreclr symbols: $symbolised ==="
grep -m1 -E 'Program terminated with signal' "$bt" | sed 's/^/  /' || echo "  (no termination line)"
echo "--- faulting thread ---"
sed -n '/^Thread 1 (/,/^$/p' "$bt" | head -24
echo "--- the fault itself ---"
sed -n '/===THE FAULT ITSELF===/,/===THE FUTEX ERRNO===/p' "$bt" | grep -v '===THE FUTEX' | sed 's/^/  /' | head -22
echo "--- futex errno probe ---"
sed -n '/===THE FUTEX ERRNO===/,/===THE ALTERNATE STACK/p' "$bt" | grep -v '===THE ALTERNATE' | sed 's/^/  /' | head -6
echo "--- every thread's alternate stack ---"
sed -n '/===THE ALTERNATE STACK, FROM uc_stack, EVERY THREAD===/,/===MAPPINGS===/p' "$bt" | grep -v '===MAPPINGS===' | sed 's/^/  /' | head -40
echo "--- threads with a Go frame or a PAL handler (any thread) ---"
awk '/^Thread [0-9]+ \(/{t=$0; sub(/ \(Thread.*/,"",t)} /^#[0-9]+ /{ if ($0 ~ /libfolio8_native|inject_activation_handler|sigsegv_handler|sigabrt_handler|sigtramp|runtime\./) print t " :: " substr($0,1,120) }' "$bt" | head -24
echo "  full trace: $bt"
