using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.IO;

/// <summary>
/// A parsed, canonicalised <c>.folio</c> document.
/// </summary>
/// <remarks>
/// Immutable, and it holds its canonical bytes rather than a native
/// handle. The reason is the same one folio-js gives: a handle would need
/// deterministic disposal across a boundary that .NET Framework's
/// finalisers do not promise. Re-parsing canonical bytes is cheap and
/// yields identical output, so the native side keeps nothing between
/// calls.
/// <para>
/// There is no way to change a template here. Editing is the designer's
/// surface and is out of scope for this library.
/// </para>
/// </remarks>
public sealed class Template
{
    private readonly byte[] _canonical;

    private Template(byte[] canonical)
    {
        _canonical = canonical;
    }

    /// <summary>Parses <c>.folio</c> bytes. The primary constructor.</summary>
    /// <param name="bytes">The template source.</param>
    /// <returns>The parsed template, holding its canonical bytes.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="bytes"/> is null.</exception>
    /// <exception cref="FolioRenderException">The template is malformed or fails a load-time check.</exception>
    public static Template Parse(byte[] bytes)
    {
        if (bytes == null)
        {
            throw new ArgumentNullException("bytes");
        }
        byte[] source = new byte[bytes.Length];
        Buffer.BlockCopy(bytes, 0, source, 0, bytes.Length);
        Native.Frame frame = Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            Native.folio8_parse(source, source.Length, out token, out result, out length));
        return new Template(frame.Payload);
    }

    /// <summary>
    /// Reads and parses the <c>.folio</c> file at <paramref name="path"/>.
    /// The only API in this library that touches disk on the caller's
    /// behalf.
    /// </summary>
    /// <param name="path">The file to read.</param>
    /// <returns>The parsed template.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="FolioRenderException">The template is malformed or fails a load-time check.</exception>
    public static Template Load(string path)
    {
        if (path == null)
        {
            throw new ArgumentNullException("path");
        }
        return Parse(File.ReadAllBytes(path));
    }

    /// <summary>
    /// The param names this template reads — what to build a
    /// <see cref="Params"/> object from, without reading the template by
    /// hand. Go's order, verbatim.
    /// </summary>
    /// <returns>The names. Empty, never null.</returns>
    public IList<string> ParameterReferences()
    {
        Native.Frame frame = Native.Invoke((out ulong token, out IntPtr result, out int length) =>
            Native.folio8_parameter_references(_canonical, _canonical.Length, out token, out result, out length));
        return new ReadOnlyCollection<string>(frame.References);
    }

    /// <summary>The canonical bytes, for the engine. Never handed out.</summary>
    internal byte[] CanonicalBytes
    {
        get { return _canonical; }
    }
}
