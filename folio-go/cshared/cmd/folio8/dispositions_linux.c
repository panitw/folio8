// The engine leaves the host's signal dispositions as it found them.
//
// A c-shared Go runtime's libpreinit — initsig(true) — runs INSIDE dlopen, on
// the loading thread, from this library's own .init_array entry
// (_rt0_<arch>_linux_lib). For the signals Go handles itself it installs its
// own handlers; for every OTHER signal that already has a handler it
// re-installs that handler with SA_ONSTACK added (runtime.setsigstack). On
// the .NET runtime that moves the GC activation handler (SIGRTMIN), which
// was sized for a thread's ordinary stack, onto the CLR's alternate stack:
// a 16 KiB mapping whose first page is a guard, so 12 KiB of stack, under a
// kernel signal frame that on x86 carries the XSAVE state. It overflows,
// and the process dies on a thread-pool worker during garbage collection.
// That is folio-dotnet's DW-396 and DW-398.
//
// A host can put its flags back after the load, and folio-dotnet does. But
// between Go's edit and that restore, every activation on every other
// thread runs on the small stack exactly as if nothing had been fixed, and
// that window killed one to four processes in a hundred under GC pressure
// (folio-dotnet/build/load-repro, runs 36002483670 and 36006849443). The
// window closes only where it opens: here, in the same .init_array pass.
//
// Two constructors:
//
//   snapshot, priority 101 — .init_array.00101 sorts before every
//     unprioritised entry, Go's included, so it sees the dispositions the
//     host had before Go touched them.
//
//   restore, unprioritised — the Go linker puts go.o before the host objects
//     (cmd/link/internal/ld/lib.go, hostlink: "argv = append(argv,
//     godotopath); argv = append(argv, hostObjCopyPaths...)"), so this entry
//     follows Go's in link order and runs once libpreinit has returned. It
//     writes the recorded struct back for every signal whose handler address
//     Go left alone and whose flags Go changed. A handler Go installed is
//     Go's — its SIGSEGV handler is how an engine fault becomes a status
//     code — and is never touched.
//
// folio-dotnet/build/verify-linux-natives.sh asserts that order on the
// shipped files. The restore also checks it at run time: if Go's SIGSEGV
// handler is not installed yet when it runs, it ran too early; it says so in
// the report and does nothing, and the Go side's init() makes the same call
// after initsig and before any export can return. Either way the host can
// read what happened through folio8_signal_dispositions.
//
// FOLIO8_SIGNAL_DISPOSITIONS=leave, in the environment at load, switches the
// restore off and leaves Go's edits in place. It exists for one reader: a
// harness that must first show it can SEE the defect on a host before a
// clean run on that host means anything (folio-dotnet/build/soak.sh's
// reproduce leg, load-repro's baseline arm). It is not a tuning knob, it is
// read once, and the report says `mode=leave` whenever it was honoured.
//
// Nothing here takes a lock: both constructors run under the dynamic
// loader's lock, on one thread, and the report reads only after they have
// run. The names buffer is written once, by whichever call restores.

#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define FOLIO8_NSIG 65

static struct sigaction folio8_snap[FOLIO8_NSIG];
static unsigned char folio8_snap_ok[FOLIO8_NSIG];
static int folio8_snapshot_taken;
static int folio8_restore_calls;
static int folio8_constructor_ran_before_go;
static const char *folio8_restored_by = "none";
static const char *folio8_mode = "restore";
static char folio8_restored_names[256];

/* 9 and 19 cannot be caught; 32 and 33 are glibc's and sigaction refuses them. */
static int folio8_catchable(int sig)
{
    return sig != SIGKILL && sig != SIGSTOP && sig != 32 && sig != 33;
}

static void folio8_append_name(int sig)
{
    char name[16];
    const char *known = NULL;
    switch (sig) {
    case SIGINT:  known = "SIGINT";  break;
    case SIGQUIT: known = "SIGQUIT"; break;
    case SIGILL:  known = "SIGILL";  break;
    case SIGTRAP: known = "SIGTRAP"; break;
    case SIGABRT: known = "SIGABRT"; break;
    case SIGTERM: known = "SIGTERM"; break;
    default: break;
    }
    if (known == NULL && sig == SIGRTMIN) {
        known = "SIGRTMIN";
    }
    if (known == NULL) {
        snprintf(name, sizeof name, "SIG%d", sig);
        known = name;
    }
    size_t used = strlen(folio8_restored_names);
    if (used + strlen(known) + 2 >= sizeof folio8_restored_names) {
        return;
    }
    if (used > 0) {
        strcat(folio8_restored_names, ",");
    }
    strcat(folio8_restored_names, known);
}

__attribute__((constructor(101)))
static void folio8_dispositions_snapshot(void)
{
    int sig;
    const char *env = getenv("FOLIO8_SIGNAL_DISPOSITIONS");
    if (env != NULL && strcmp(env, "leave") == 0) {
        folio8_mode = "leave";
    }
    for (sig = 1; sig < FOLIO8_NSIG; sig++) {
        folio8_snap_ok[sig] = folio8_catchable(sig) && sigaction(sig, NULL, &folio8_snap[sig]) == 0;
    }
    folio8_snapshot_taken = 1;
}

/*
 * Puts back every disposition Go re-flagged without replacing. Returns how
 * many. `who` names the caller for the report: "constructor" or "init".
 * Idempotent — a second call finds nothing left to do. Hidden: it is called
 * from this library's own constructor and from its Go side, and the
 * library's exports are the folio8_ symbols the README lists, counted by
 * CI — this and the report function are not among them.
 */
__attribute__((visibility("hidden")))
int folio8_dispositions_restore(const char *who)
{
    struct sigaction now;
    int sig, restored = 0;

    folio8_restore_calls++;
    if (!folio8_snapshot_taken || strcmp(folio8_mode, "leave") == 0) {
        return 0;
    }
    /* Go has run initsig exactly when SIGSEGV's handler is no longer what
       the snapshot holds: Go always installs its own. */
    if (!folio8_snap_ok[SIGSEGV] || sigaction(SIGSEGV, NULL, &now) != 0) {
        return 0;
    }
    if (now.sa_sigaction == folio8_snap[SIGSEGV].sa_sigaction) {
        if (strcmp(who, "constructor") == 0) {
            folio8_constructor_ran_before_go = 1;
        }
        return 0;
    }
    for (sig = 1; sig < FOLIO8_NSIG; sig++) {
        if (!folio8_snap_ok[sig] || sigaction(sig, NULL, &now) != 0) {
            continue;
        }
        if (now.sa_sigaction != folio8_snap[sig].sa_sigaction) {
            continue; /* replaced: Go's own, and Go relies on it */
        }
        if (now.sa_flags == folio8_snap[sig].sa_flags) {
            continue; /* untouched */
        }
        if (sigaction(sig, &folio8_snap[sig], NULL) != 0) {
            continue;
        }
        folio8_append_name(sig);
        restored++;
    }
    if (restored > 0 && strcmp(folio8_restored_by, "none") == 0) {
        folio8_restored_by = who;
    }
    return restored;
}

__attribute__((constructor))
static void folio8_dispositions_restore_ctor(void)
{
    folio8_dispositions_restore("constructor");
}

/*
 * One line of key=value pairs, for folio8_signal_dispositions. Writes at
 * most cap bytes into buf and returns the length written, or -1 if buf is
 * too small. The keys are the contract a test may read:
 *   mode=restore|leave               leave: FOLIO8_SIGNAL_DISPOSITIONS=leave was set
 *   snapshot=yes|no                  the pre-Go record exists
 *   constructor-ran-before-go=yes|no the link order held (no) or did not (yes)
 *   restored-by=constructor|init|none who actually put dispositions back
 *   restored=<names>|-               which, comma-separated
 *   restore-calls=<n>                how many times the restore was asked
 */
__attribute__((visibility("hidden")))
int folio8_dispositions_report(char *buf, int cap)
{
    int n = snprintf(buf, cap > 0 ? (size_t)cap : 0,
                     "linux mode=%s snapshot=%s constructor-ran-before-go=%s restored-by=%s restored=%s restore-calls=%d",
                     folio8_mode,
                     folio8_snapshot_taken ? "yes" : "no",
                     folio8_constructor_ran_before_go ? "yes" : "no",
                     folio8_restored_by,
                     folio8_restored_names[0] != '\0' ? folio8_restored_names : "-",
                     folio8_restore_calls);
    if (n < 0 || n >= cap) {
        return -1;
    }
    return n;
}
