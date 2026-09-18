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
/// The contract is written down in <c>folio8-go/cshared/README.md</c> and
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
    /// <c>folio8.dll</c> on Windows and <c>libfolio8.dylib</c>/<c>.so</c>
    /// on a development host. RID layout and bitness-aware loading are
    /// story 7; this is deliberately the plain form.
    /// </summary>
    internal const string Library = "folio8";

    /// <summary>
    /// The ABI shape this assembly was built against. Checked once, before
    /// the first real call.
    /// </summary>
    internal const int ExpectedAbiVersion = 1;

    // Status codes. Keep in step with folio8-go/cshared/README.md.
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
        out ulong token, out IntPtr result, out int length);

    [DllImport(Library, CallingConvention = CallingConvention.Cdecl)]
    internal static extern int folio8_validate(
        byte[] template, int templateLength,
        byte[] data, int dataLength,
        byte[] parameters, int parametersLength,
        byte[] fonts, int fontsLength,
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
        lock (AbiGate)
        {
            if (_abiChecked)
            {
                return;
            }
            int actual = folio8_abi_version();
            if (actual != ExpectedAbiVersion)
            {
                throw new InvalidOperationException(
                    "folio8: the native library reports ABI version " + Number(actual) +
                    ", but this assembly was built against version " + Number(ExpectedAbiVersion) +
                    ". The managed and native halves of folio8 are from different builds; replace the one that is out of date.");
            }
            _abiChecked = true;
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
