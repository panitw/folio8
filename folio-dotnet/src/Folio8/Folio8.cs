using System;
using System.Collections.Generic;
using System.IO;

/// <summary>
/// folio8 for .NET: render a <c>.folio</c> template plus JSON data and params
/// into PDF bytes, or validate the same inputs without rendering.
/// </summary>
/// <remarks>
/// Synchronous, matching the Go engine it binds. Every call runs inside the
/// native folio8 library — there is no rendering logic in this assembly, and
/// there never will be: layout, shaping, pagination and PDF emission stay in
/// one engine so the bytes are the same everywhere.
/// <para>
/// Nothing here reads the clock, the environment, the network, or any file
/// beyond <see cref="Template.Load"/>. <c>documentDate</c> arrives through
/// <see cref="Params"/> or it is absent.
/// </para>
/// <para>
/// A <b>warning</b> reaches <see cref="RenderResult.Diagnostics"/>. An
/// <b>error</b> is thrown: a coded one as <see cref="FolioRenderException"/>,
/// and one the engine reports without a diagnostic as
/// <see cref="InvalidOperationException"/> carrying the engine's message.
/// </para>
/// <para>
/// The public types here deliberately sit in the global namespace so a call
/// site reads exactly as the API contract writes it —
/// <c>Folio8.Render(...)</c>, <c>Template.Parse(...)</c> — which a namespace
/// named <c>Folio8</c> would make unreachable: C# would resolve the namespace
/// and never find the class.
/// </para>
/// </remarks>
public static class Folio8
{
    /// <summary>
    /// The engine version this library binds — the same string
    /// <c>folio8.Version</c> reports in Go.
    /// </summary>
    public static string Version
    {
        get
        {
            Native.Frame frame = Native.Invoke((out ulong token, out IntPtr result, out int length) =>
                Native.folio8_version(out token, out result, out length));
            return System.Text.Encoding.UTF8.GetString(frame.Payload);
        }
    }

    /// <summary>Renders a PDF.</summary>
    /// <param name="template">The template. Required.</param>
    /// <param name="data">The JSON report data. Required.</param>
    /// <param name="parameters">The runtime params, or null for none.</param>
    /// <param name="fonts">The faces the template may use. Required, and never empty.</param>
    /// <returns>The PDF bytes and the engine's warnings.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="template"/>, <paramref name="data"/> or <paramref name="fonts"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="fonts"/> is empty.</exception>
    /// <exception cref="FolioRenderException">The engine aborted with a coded diagnostic.</exception>
    /// <exception cref="InvalidOperationException">The engine aborted without one.</exception>
    public static RenderResult Render(Template template, Data data, Params parameters, FontSet fonts)
    {
        byte[] fontBuffer = Check(template, data, fonts);
        byte[] tpl = template.CanonicalBytes;
        byte[] payload = data.Bytes;
        byte[] parameterBytes = parameters == null ? null : parameters.Bytes;
        Native.Frame frame = Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            Native.folio8_render(
                tpl, tpl.Length,
                payload, payload.Length,
                parameterBytes, parameterBytes == null ? 0 : parameterBytes.Length,
                fontBuffer, fontBuffer.Length,
                out token, out result, out length));
        return new RenderResult(frame.Payload, RenderResult.Readonly(frame.Diagnostics));
    }

    /// <summary>
    /// Renders completely, then writes the PDF to <paramref name="destination"/>
    /// in one write. For HTTP responses and large documents.
    /// </summary>
    /// <remarks>
    /// The stream is never closed, disposed or flushed to completion by this
    /// call, and <b>nothing is written when the render fails</b> — the bytes
    /// exist in full before the first one reaches the stream.
    /// </remarks>
    /// <param name="destination">Where to write. Never closed or disposed.</param>
    /// <param name="template">The template. Required.</param>
    /// <param name="data">The JSON report data. Required.</param>
    /// <param name="parameters">The runtime params, or null for none.</param>
    /// <param name="fonts">The faces the template may use. Required, and never empty.</param>
    /// <returns>The engine's warnings. Empty, never null.</returns>
    /// <exception cref="ArgumentNullException">A required argument is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="fonts"/> is empty, or the stream cannot be written to.</exception>
    /// <exception cref="FolioRenderException">The engine aborted with a coded diagnostic.</exception>
    /// <exception cref="InvalidOperationException">The engine aborted without one.</exception>
    public static IList<Diagnostic> RenderTo(Stream destination, Template template, Data data, Params parameters, FontSet fonts)
    {
        if (destination == null)
        {
            throw new ArgumentNullException("destination");
        }
        if (!destination.CanWrite)
        {
            throw new ArgumentException("folio8: destination is not writable", "destination");
        }
        RenderResult result = Render(template, data, parameters, fonts);
        destination.Write(result.Bytes, 0, result.Bytes.Length);
        return result.Diagnostics;
    }

    /// <summary>
    /// Predicts a render without performing one, returning the engine's
    /// diagnostics. Takes raw template bytes rather than a parsed
    /// <see cref="Template"/>, matching the engine.
    /// </summary>
    /// <remarks>
    /// It predicts a render <b>for the inputs given</b>. Passing empty or
    /// partial data yields absent-path errors, and those are correct
    /// predictions of a render with that same data — not defects in the
    /// template. Pass the data the render will actually receive.
    /// </remarks>
    /// <param name="template">The raw <c>.folio</c> bytes. Required.</param>
    /// <param name="data">The JSON report data. Required.</param>
    /// <param name="parameters">The runtime params, or null for none.</param>
    /// <param name="fonts">The faces the template may use. Required, and never empty.</param>
    /// <returns>The engine's diagnostics, in its order. Empty, never null.</returns>
    /// <exception cref="ArgumentNullException">A required argument is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="fonts"/> is empty.</exception>
    /// <exception cref="FolioRenderException">The engine aborted with a coded diagnostic — which is where it reports an absent binding, rather than in the returned list.</exception>
    /// <exception cref="InvalidOperationException">The engine aborted without one.</exception>
    public static IList<Diagnostic> Validate(byte[] template, Data data, Params parameters, FontSet fonts)
    {
        if (template == null)
        {
            throw new ArgumentNullException("template");
        }
        byte[] fontBuffer = CheckDataAndFonts(data, fonts);
        byte[] tpl = new byte[template.Length];
        Buffer.BlockCopy(template, 0, tpl, 0, template.Length);
        byte[] payload = data.Bytes;
        byte[] parameterBytes = parameters == null ? null : parameters.Bytes;
        Native.Frame frame = Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            Native.folio8_validate(
                tpl, tpl.Length,
                payload, payload.Length,
                parameterBytes, parameterBytes == null ? 0 : parameterBytes.Length,
                fontBuffer, fontBuffer.Length,
                out token, out result, out length));
        return RenderResult.Readonly(frame.Diagnostics);
    }

    /// <summary>
    /// How many native result buffers this process is still holding. It is
    /// zero between calls, and a test can prove that over a loop rather than
    /// assert it.
    /// </summary>
    internal static int OutstandingNativeAllocations()
    {
        return Native.folio8_allocation_count();
    }

    private static byte[] Check(Template template, Data data, FontSet fonts)
    {
        if (template == null)
        {
            throw new ArgumentNullException("template");
        }
        return CheckDataAndFonts(data, fonts);
    }

    private static byte[] CheckDataAndFonts(Data data, FontSet fonts)
    {
        if (data == null)
        {
            throw new ArgumentNullException("data");
        }
        if (fonts == null)
        {
            throw new ArgumentNullException("fonts");
        }
        if (fonts.Count == 0)
        {
            // There is no default font set and no ambient lookup. Omitting
            // fonts is a caller error, not a fallback.
            throw new ArgumentException("folio8: fonts must name at least one face; there is no default font set", "fonts");
        }
        return Native.EncodeFonts(fonts);
    }
}
