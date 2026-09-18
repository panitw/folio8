using System.Collections.Generic;
using System.Collections.ObjectModel;

/// <summary>
/// What a successful render produced: the PDF bytes, and whatever the engine
/// had to say about it.
/// </summary>
/// <remarks>
/// <see cref="Diagnostics"/> is always present and empty rather than null,
/// mirroring the engine's "empty is nil, one representation" rule. Every
/// entry in it is a <see cref="Severity.Warning"/>: an error aborts the render
/// and arrives as a thrown <see cref="FolioRenderException"/> instead, so a
/// successful result never carries one.
/// </remarks>
public sealed class RenderResult
{
    internal RenderResult(byte[] bytes, IList<Diagnostic> diagnostics)
    {
        Bytes = bytes;
        Diagnostics = diagnostics;
    }

    /// <summary>The PDF.</summary>
    public byte[] Bytes { get; private set; }

    /// <summary>The engine's warnings, in the engine's order. Empty, never null.</summary>
    public IList<Diagnostic> Diagnostics { get; private set; }

    internal static IList<Diagnostic> Readonly(Diagnostic[] diagnostics)
    {
        return new ReadOnlyCollection<Diagnostic>(diagnostics);
    }
}
