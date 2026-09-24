using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Runtime.ExceptionServices;
using System.Runtime.InteropServices;
using System.Text;

/// <summary>
/// The whole of the native boundary: the <c>DllImport</c> declarations for
/// the folio8 C ABI, the encoder for its one font buffer, and the reader
/// for its one result frame. Nothing above this file knows the ABI exists.
/// </summary>
/// <remarks>
/// The contract is written down in <c>folio-go/cshared/README.md</c> and
/// this file is held to it, not the other way round.
/// <para>
/// <see cref="CallingConvention.Cdecl"/> is load-bearing, not decoration.
/// cgo exports are cdecl on every platform, while the .NET default
/// (<see cref="CallingConvention.Winapi"/>) is stdcall on Windows — the
/// mismatch is invisible on x64, which has one convention, and corrupts
/// the stack on x86, which is the floor this library exists to reach.
/// </para>
/// <para>
/// The frame reader treats the native buffer as UNTRUSTED. It is produced by
/// the library this assembly loaded, which is not necessarily the one it was
/// built against — so every count and every length is checked against what
/// the buffer actually holds before a byte is allocated, and a frame that
/// does not add up is reported as malformed rather than as an overflow or an
/// end-of-stream from somewhere deeper.
/// </para>
/// </remarks>
internal static class Native
{
    /// <summary>
    /// The native library's base name. Probing resolves it to
    /// <c>folio8_native.dll</c> on Windows and
    /// <c>libfolio8_native.dylib</c>/<c>.so</c> elsewhere. RID layout and
    /// bitness-aware loading are story 7; this is deliberately the plain form.
    /// </summary>
    /// <remarks>
    /// <b><c>folio8_native</c>, not <c>folio8</c>, and the difference is
    /// load-bearing.</b> This assembly is <c>Folio8.dll</c>, and NTFS — like
    /// the default macOS file system — is case-insensitive, so a native
    /// library called <c>folio8.dll</c> IS <c>Folio8.dll</c> as far as the
    /// file system is concerned. Staged into one directory, which is exactly
    /// what a NuGet RID asset does on modern .NET and what the test project
    /// does, one silently overwrites the other — and <c>DllImport</c> then
    /// loads a perfectly valid PE file that happens to contain no
    /// <c>folio8_</c> exports at all.
    /// <para>
    /// Measured, not theorised: on Windows every managed test failed with
    /// <see cref="EntryPointNotFoundException"/> on the first call while
    /// <c>objdump -p</c> showed the DLL exporting all eight symbols,
    /// undecorated. The same sources passed on macOS, where the native file
    /// is <c>libfolio8.dylib</c> and no collision was possible.
    /// </para>
    /// </remarks>
    internal const string Library = "folio8_native";

    /// <summary>
    /// The ABI shape this assembly was built against. Checked once, before
    /// the first real call.
    /// </summary>
    internal const int ExpectedAbiVersion = 2;

    // Status codes. Keep in step with folio-go/cshared/README.md.
    internal const int StatusOk = 0;
    internal const int StatusErrorDiagnostic = 1;
    internal const int StatusErrorMessage = 2;
    internal const int StatusErrorArgument = 3;
    internal const int StatusErrorPanic = 4;
    internal const int StatusErrorUnknownFree = 5;

    // Result frame kinds.
    private const byte KindOk = 1;
    private const byte KindErrorDiagnostic = 2;
    private const byte KindErrorMessage = 3;

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_abi_version();

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_free(ulong token);

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_allocation_count();

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_version(out ulong token, out IntPtr result, out int length);

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_parse(byte[] template, int templateLength, out ulong token, out IntPtr result, out int length);

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_parameter_references(byte[] template, int templateLength, out ulong token, out IntPtr result, out int length);

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_render(
        byte[] template, int templateLength,
        byte[] data, int dataLength,
        byte[] parameters, int parametersLength,
        byte[] fonts, int fontsLength,
        int fallback,
        out ulong token, out IntPtr result, out int length);

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_validate(
        byte[] template, int templateLength,
        byte[] data, int dataLength,
        byte[] parameters, int parametersLength,
        byte[] fonts, int fontsLength,
        int fallback,
        out ulong token, out IntPtr result, out int length);

    private static readonly object AbiGate = new object();
    private static bool _abiChecked;

    /// <summary>
    /// Checks the loaded library's ABI version ONCE, before the first real
    /// call. A managed assembly paired with a native library from a different
    /// build would otherwise decode a frame grammar that has moved under it
    /// and report the result as data.
    /// </summary>
    internal static void EnsureAbi()
    {
        if (_abiChecked)
        {
            return;
        }
        // THIS CROSSES, SO IT CROSSES FROM AN ENGINE THREAD LIKE EVERY OTHER
        // CROSSING. Called from Invoke, which is already on one, this runs
        // inline and takes no second queue hop.
        //
        // The unsynchronised read of _abiChecked above is pre-existing and
        // deliberately left as it was: the write still happens under AbiGate,
        // a stale read costs one extra lock acquisition and nothing else, and
        // the hop below neither relies on the fast path nor makes it racier
        // than it already was.
        EngineThreads.Run(new Action(CheckAbi));
    }

    /// <summary>
    /// The ABI check itself, which runs on an engine thread. Separated from
    /// <see cref="EnsureAbi"/> only so the hop is expressed once.
    /// </summary>
    private static void CheckAbi()
    {
        lock (AbiGate)
        {
            if (_abiChecked)
            {
                return;
            }
            // BEFORE THE LOAD, ONCE: what every signal's disposition was
            // before the engine touched it. Go's initsig re-flags handlers
            // it does not own (DW-398); this is the reference the restore
            // below writes back. Taken once for the process rather than per
            // attempt, so a retry after a failed ABI check does not record
            // the post-load state as "before".
            if (_dispositionsBeforeLoad == null && !_dispositionsTaken)
            {
                _dispositionsTaken = true;
                _dispositionsBeforeLoad = SignalDispositions.Take();
            }

            // BEFORE THE FIRST P/INVOKE, AND THAT ORDER IS THE WHOLE POINT.
            // DllImport binds a module by name once; loading the
            // bitness-correct file by full path here means the seven imports
            // below bind to it rather than to whatever a bare name search
            // would turn up — or to nothing at all.
            NativeLibraryLoader.Ensure();

            int actual;
            try
            {
                actual = folio8_abi_version();
            }
            catch (EntryPointNotFoundException missing)
            {
                // The file loaded but has no folio8_ exports in it. That is
                // almost never "this one symbol is missing" — it is the wrong
                // file under the right name. Say so, and name what was
                // actually loaded, because the platform's own message names
                // only the symbol.
                throw new InvalidOperationException(
                    "folio8: a library named '" + Library + "' loaded, but it does not export folio8_abi_version — so it is not the folio8 engine. " +
                    LoadedModuleDescription() +
                    " Expected a c-shared build of folio-go/cshared/cmd/folio8, which exports folio8_abi_version, folio8_version, folio8_parse, folio8_render, folio8_validate, folio8_parameter_references, folio8_free and folio8_allocation_count.",
                    missing);
            }
            if (actual != ExpectedAbiVersion)
            {
                throw new InvalidOperationException(
                    "folio8: the native library reports ABI version " + Number(actual) +
                    ", but this assembly was built against version " + Number(ExpectedAbiVersion) +
                    ". The managed and native halves of folio8 are from different builds; replace the one that is out of date.");
            }

            // AFTER THE FIRST EXPORT HAS RETURNED — NOT AFTER THE LOAD. A
            // c-shared Go runtime initialises on its own thread and dlopen
            // returns before its initsig has run, so a restore placed right
            // after Ensure() can be undone moments later. folio8_abi_version
            // blocked until the runtime was up; Go's edits are all in place
            // now, and this puts the CLR's own dispositions back for the
            // handlers Go re-flagged without replacing (SIGRTMIN among them,
            // which is DW-398). See SignalDispositions for the whole account.
            _dispositionsRestored = SignalDispositions.Restore(_dispositionsBeforeLoad);
            _abiChecked = true;
        }
    }

    /// <summary>
    /// Every signal's disposition before the engine loaded, taken once per
    /// process; <c>null</c> off Linux.
    /// </summary>
    private static SignalDispositions.Snapshot _dispositionsBeforeLoad;
    private static bool _dispositionsTaken;

    /// <summary>
    /// The signals whose dispositions the first crossing put back, as a
    /// comma-separated list, for diagnostics. Empty until the first crossing,
    /// and empty off Linux.
    /// </summary>
    internal static string DispositionsRestored
    {
        get { return _dispositionsRestored ?? string.Empty; }
    }
    private static string _dispositionsRestored;

    /// <summary>
    /// Names the file actually loaded under <see cref="Library"/>, for the
    /// entry-point-not-found message. Best effort and never fatal: this runs
    /// only on a path that is already failing, and a host that forbids module
    /// enumeration must not turn a clear diagnostic into a second exception.
    /// </summary>
    private static string LoadedModuleDescription()
    {
        try
        {
            System.Diagnostics.ProcessModuleCollection modules = System.Diagnostics.Process.GetCurrentProcess().Modules;
            for (int i = 0; i < modules.Count; i++)
            {
                string name = modules[i].ModuleName;
                if (name != null && name.IndexOf(Library, StringComparison.OrdinalIgnoreCase) >= 0)
                {
                    return "The file loaded under that name is: " + modules[i].FileName + ".";
                }
            }
            return "No loaded module matched that name, so it may have been unloaded already.";
        }
        catch (Exception)
        {
            return "The loaded module could not be identified in this host.";
        }
    }

    /// <summary>A native call, in the one shape every export shares.</summary>
    internal delegate int Call(out ulong token, out IntPtr result, out int length);

    /// <summary>
    /// Runs <paramref name="call"/>, copies its result buffer into managed
    /// memory, returns the buffer to the engine with exactly one
    /// <c>folio8_free</c>, and then translates the outcome: a success
    /// becomes a <see cref="Frame"/>, an error becomes a throw carrying
    /// Go's own diagnostic or message.
    /// </summary>
    internal static Frame Invoke(Call call)
    {
        // THE WHOLE BODY MOVES, NOT JUST THE P/INVOKE. Moving only the inner
        // call would leave the copy and the folio8_free tail on the calling
        // thread, straddling the export and its free across a thread
        // boundary — and the free is itself an ABI crossing.
        return EngineThreads.Run(new Func<Frame>(delegate { return InvokeHere(call); }));
    }

    /// <summary>
    /// <see cref="Invoke"/>'s body, running on an engine thread.
    /// </summary>
    /// <remarks>
    /// NEVER INLINED, and that is an assertion's requirement rather than a
    /// performance one: this is the frame a native error is thrown from, and
    /// a test proves the hop preserved the original stack by finding this
    /// name in it. A Release JIT free to inline the method would delete the
    /// evidence and the test with it.
    /// </remarks>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.NoInlining)]
    private static Frame InvokeHere(Call call)
    {
        EnsureAbi();

        ulong token;
        IntPtr result;
        int length;
        int status = call(out token, out result, out length);

        if (status == StatusErrorArgument)
        {
            // The ABI issues no token and allocates nothing for this
            // status, so there is nothing to free. Reaching it means this
            // file built a malformed call, which is a defect here.
            throw new InvalidOperationException(
                "folio8: the native library rejected the call as malformed. This is a defect in the folio8 .NET binding, not in the caller's input.");
        }

        byte[] buffer = null;
        ExceptionDispatchInfo inFlight = null;
        try
        {
            if (length < 0)
            {
                throw Malformed("the result length is negative (" + Number(length) + ")");
            }
            buffer = new byte[length];
            if (length > 0)
            {
                if (result == IntPtr.Zero)
                {
                    throw Malformed("a result of " + Number(length) + " bytes was reported with a null pointer");
                }
                Marshal.Copy(result, buffer, 0, length);
            }
        }
        catch (Exception error)
        {
            inFlight = ExceptionDispatchInfo.Capture(error);
        }

        // Exactly one free, on every path, including the error statuses —
        // they allocate a buffer too. The free's own failure must NOT replace
        // an exception already on its way out: that would hide the real
        // problem behind a symptom of it.
        int freed = folio8_free(token);
        if (inFlight != null)
        {
            inFlight.Throw();
        }
        if (freed != StatusOk)
        {
            throw new InvalidOperationException(
                "folio8: the native library refused to free result token " + Number(token) +
                " (status " + Number(freed) + "). The allocation table and this binding disagree about ownership.");
        }

        Frame frame = Frame.Read(buffer);
        switch (status)
        {
            case StatusOk:
                RequireKind(frame, KindOk, status);
                return frame;
            case StatusErrorDiagnostic:
                RequireKind(frame, KindErrorDiagnostic, status);
                throw new FolioRenderException(frame.ErrorDiagnostic);
            case StatusErrorMessage:
            case StatusErrorPanic:
                // Go returns a plain error here, so this is a plain
                // exception — the same split folio-js makes between a
                // thrown FolioRenderError and a thrown Error.
                RequireKind(frame, KindErrorMessage, status);
                throw new InvalidOperationException(frame.ErrorMessage);
            default:
                throw new InvalidOperationException("folio8: the native library returned unknown status " + Number(status) + ".");
        }
    }

    /// <summary>
    /// A status and a frame kind that disagree mean the two halves are not
    /// speaking the same ABI. Saying so is far better than handing back a
    /// <see cref="FolioRenderException"/> whose <c>Diagnostic</c> is null.
    /// </summary>
    private static void RequireKind(Frame frame, byte expected, int status)
    {
        if (frame.Kind != expected)
        {
            throw Malformed("status " + Number(status) + " arrived with a kind-" + Number(frame.Kind) + " frame, but that status carries kind " + Number(expected));
        }
    }

    private static InvalidOperationException Malformed(string detail)
    {
        return new InvalidOperationException("folio8: the native library returned a malformed result frame — " + detail + ".");
    }

    private static string Number(long value)
    {
        return value.ToString(CultureInfo.InvariantCulture);
    }

    private static string Number(ulong value)
    {
        return value.ToString(CultureInfo.InvariantCulture);
    }

    /// <summary>
    /// Encodes a font set as the ABI's one font buffer: a length-prefixed
    /// sequence of name/bytes pairs. Order carries no meaning to the engine,
    /// which keys faces by name, and a duplicate name is refused there.
    /// </summary>
    internal static byte[] EncodeFonts(FontSet fonts)
    {
        using (MemoryStream stream = new MemoryStream())
        using (BinaryWriter writer = new BinaryWriter(stream))
        {
            foreach (KeyValuePair<string, byte[]> face in fonts)
            {
                byte[] name = Encoding.UTF8.GetBytes(face.Key);
                writer.Write(name.Length);
                writer.Write(name);
                writer.Write(face.Value.Length);
                writer.Write(face.Value);
            }
            writer.Flush();
            return stream.ToArray();
        }
    }

    /// <summary>One decoded result frame.</summary>
    internal sealed class Frame
    {
        internal byte Kind;
        internal Diagnostic[] Diagnostics;
        internal string[] References;
        internal byte[] Payload;
        internal Diagnostic ErrorDiagnostic;
        internal string ErrorMessage;

        private static readonly Diagnostic[] NoDiagnostics = new Diagnostic[0];
        private static readonly string[] NoReferences = new string[0];
        private static readonly byte[] NoPayload = new byte[0];

        internal static Frame Read(byte[] buffer)
        {
            if (buffer == null || buffer.Length == 0)
            {
                throw Malformed("it is empty");
            }

            Frame frame = new Frame
            {
                Diagnostics = NoDiagnostics,
                References = NoReferences,
                Payload = NoPayload,
            };

            Cursor cursor = new Cursor(buffer);
            frame.Kind = cursor.Byte();
            switch (frame.Kind)
            {
                case KindOk:
                    int diagnosticCount = cursor.Count("diagnostic count");
                    if (diagnosticCount > 0)
                    {
                        Diagnostic[] diagnostics = new Diagnostic[diagnosticCount];
                        for (int i = 0; i < diagnosticCount; i++)
                        {
                            diagnostics[i] = ReadDiagnostic(cursor);
                        }
                        frame.Diagnostics = diagnostics;
                    }
                    int referenceCount = cursor.Count("reference count");
                    if (referenceCount > 0)
                    {
                        string[] references = new string[referenceCount];
                        for (int i = 0; i < referenceCount; i++)
                        {
                            references[i] = cursor.String("reference");
                        }
                        frame.References = references;
                    }
                    frame.Payload = cursor.Blob("payload");
                    break;
                case KindErrorDiagnostic:
                    frame.ErrorDiagnostic = ReadDiagnostic(cursor);
                    break;
                case KindErrorMessage:
                    frame.ErrorMessage = cursor.String("message");
                    break;
                default:
                    throw Malformed("its kind byte is " + Number(frame.Kind));
            }
            // Nothing may trail a frame. A short payload followed by junk, or
            // a count that stopped early, both land here rather than passing
            // as a complete result.
            cursor.End();
            return frame;
        }

        private static Diagnostic ReadDiagnostic(Cursor cursor)
        {
            byte severity = cursor.Byte();
            string code = cursor.String("diagnostic code");
            string elementId = cursor.String("diagnostic element id");
            string dataPath = cursor.String("diagnostic data path");
            string message = cursor.String("diagnostic message");
            return new Diagnostic(SeverityOf(severity), code, elementId, dataPath, message);
        }

        private static Severity SeverityOf(byte value)
        {
            switch (value)
            {
                case 1: return Severity.Warning;
                case 2: return Severity.Error;
                default:
                    throw Malformed("it carries severity byte " + Number(value) + ", which is neither warning (1) nor error (2)");
            }
        }

        /// <summary>
        /// A bounds-checked reader over the native buffer. Every length is
        /// read as unsigned and validated against what remains before an
        /// allocation is made, so a corrupt or hostile frame produces the
        /// deliberate "malformed" message rather than an
        /// <see cref="OverflowException"/> or an end-of-stream.
        /// </summary>
        private sealed class Cursor
        {
            private readonly byte[] _buffer;
            private int _offset;

            internal Cursor(byte[] buffer)
            {
                _buffer = buffer;
            }

            internal byte Byte()
            {
                Need(1, "a kind or severity byte");
                return _buffer[_offset++];
            }

            internal int Count(string what)
            {
                Need(4, what);
                uint value = (uint)_buffer[_offset]
                    | ((uint)_buffer[_offset + 1] << 8)
                    | ((uint)_buffer[_offset + 2] << 16)
                    | ((uint)_buffer[_offset + 3] << 24);
                _offset += 4;
                if (value > int.MaxValue)
                {
                    throw Malformed(what + " is " + Number(value) + ", which exceeds what this runtime can address");
                }
                int count = (int)value;
                // An element is at least one byte, so a count larger than the
                // bytes remaining is refused BEFORE an array that size is
                // allocated.
                if (count > Remaining)
                {
                    throw Malformed(what + " is " + Number(count) + " but only " + Number(Remaining) + " bytes remain");
                }
                return count;
            }

            internal byte[] Blob(string what)
            {
                int length = Count(what + " length");
                if (length == 0)
                {
                    return NoPayload;
                }
                Need(length, what);
                byte[] bytes = new byte[length];
                Buffer.BlockCopy(_buffer, _offset, bytes, 0, length);
                _offset += length;
                return bytes;
            }

            internal string String(string what)
            {
                byte[] bytes = Blob(what);
                return bytes.Length == 0 ? string.Empty : Encoding.UTF8.GetString(bytes);
            }

            internal void End()
            {
                if (Remaining != 0)
                {
                    throw Malformed(Number(Remaining) + " bytes trail its declared contents");
                }
            }

            private int Remaining
            {
                get { return _buffer.Length - _offset; }
            }

            private void Need(int count, string what)
            {
                if (Remaining < count)
                {
                    throw Malformed("it is truncated before " + what + " (" + Number(count) + " bytes needed, " + Number(Remaining) + " left)");
                }
            }
        }
    }
}
