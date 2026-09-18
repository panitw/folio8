using System;
using System.Runtime.Serialization;

/// <summary>
/// Thrown when the engine aborts a call with a coded diagnostic — the
/// managed form of Go's <c>*RenderError</c>.
/// </summary>
/// <remarks>
/// An <see cref="Severity.Error"/> never appears in
/// <see cref="RenderResult.Diagnostics"/>; it arrives here instead. Match
/// on the <see cref="Diagnostic"/>'s <c>Code</c>,
/// never on <see cref="Exception.Message"/>, which is Go's prose.
/// <para>
/// Not every engine failure is one of these: where Go returns a plain
/// error with no diagnostic — malformed JSON report data, for instance —
/// this library throws <see cref="InvalidOperationException"/> carrying
/// Go's message, exactly as folio-js throws a plain <c>Error</c> rather
/// than a <c>FolioRenderError</c>.
/// </para>
/// </remarks>
[Serializable]
public sealed class FolioRenderException : Exception
{
    [NonSerialized]
    private readonly Diagnostic _diagnostic;

    /// <summary>Creates an exception carrying the engine's diagnostic.</summary>
    /// <param name="diagnostic">The diagnostic that aborted the call.</param>
    public FolioRenderException(Diagnostic diagnostic)
        : base(diagnostic == null ? "folio8: the render failed" : diagnostic.Message)
    {
        _diagnostic = diagnostic;
    }

    /// <summary>Deserialisation constructor. The diagnostic does not survive the round trip.</summary>
    /// <param name="info">The serialisation store.</param>
    /// <param name="context">The streaming context.</param>
    private FolioRenderException(SerializationInfo info, StreamingContext context)
        : base(info, context)
    {
    }

    /// <summary>
    /// The engine's diagnostic: its code, severity, element id, data path
    /// and message, identical to what Go reports for the same inputs.
    /// </summary>
    public Diagnostic Diagnostic
    {
        get { return _diagnostic; }
    }
}
