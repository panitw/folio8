using System;

/// <summary>
/// Which of the engine's two dispositions a <see cref="Diagnostic"/>
/// carries. A closed set, and the unset zero value is deliberately not
/// representable — there is no <c>Severity</c> a diagnostic can hold that
/// means "nobody said".
/// </summary>
public enum Severity
{
    /// <summary>
    /// Content that could not be honoured exactly as declared, reported
    /// alongside complete, valid bytes. A warning reaches
    /// <see cref="RenderResult.Diagnostics"/>; it never aborts a render.
    /// </summary>
    Warning = 1,

    /// <summary>
    /// A failure that aborts the call. An error is never returned
    /// alongside bytes — it arrives as a thrown
    /// <see cref="FolioRenderException"/>.
    /// </summary>
    Error = 2,
}

/// <summary>
/// One thing the engine has to say about a render or a validation.
/// </summary>
/// <remarks>
/// <see cref="Code"/> is the contract and the thing callers dispatch on:
/// a stable registry string, identical in Go, folio-js and here.
/// <see cref="Message"/> is prose for humans and must never be parsed; it
/// carries no stability promise beyond matching Go's text for the same
/// condition.
/// </remarks>
public sealed class Diagnostic : IEquatable<Diagnostic>
{
    /// <summary>Creates a diagnostic. Callers read these; the engine makes them.</summary>
    /// <param name="severity">Warning or error.</param>
    /// <param name="code">The stable registry code.</param>
    /// <param name="elementId">The element this concerns, or the empty string.</param>
    /// <param name="dataPath">The data path this concerns, or the empty string.</param>
    /// <param name="message">Go's message text, verbatim.</param>
    public Diagnostic(Severity severity, string code, string elementId, string dataPath, string message)
    {
        Severity = severity;
        Code = code ?? string.Empty;
        ElementId = elementId ?? string.Empty;
        DataPath = dataPath ?? string.Empty;
        Message = message ?? string.Empty;
    }

    /// <summary>Warning or error.</summary>
    public Severity Severity { get; private set; }

    /// <summary>The stable registry code — the thing to dispatch on.</summary>
    public string Code { get; private set; }

    /// <summary>The element this concerns, or the empty string.</summary>
    public string ElementId { get; private set; }

    /// <summary>The data path this concerns, or the empty string.</summary>
    public string DataPath { get; private set; }

    /// <summary>Go's message text, verbatim. Never parse it.</summary>
    public string Message { get; private set; }

    /// <summary>Value equality across all five fields.</summary>
    /// <param name="other">The diagnostic to compare with.</param>
    /// <returns><c>true</c> when every field matches.</returns>
    public bool Equals(Diagnostic other)
    {
        if (ReferenceEquals(other, null))
        {
            return false;
        }
        return Severity == other.Severity
            && Code == other.Code
            && ElementId == other.ElementId
            && DataPath == other.DataPath
            && Message == other.Message;
    }

    /// <inheritdoc />
    public override bool Equals(object obj)
    {
        return Equals(obj as Diagnostic);
    }

    /// <inheritdoc />
    public override int GetHashCode()
    {
        unchecked
        {
            int hash = (int)Severity;
            hash = (hash * 397) ^ Code.GetHashCode();
            hash = (hash * 397) ^ ElementId.GetHashCode();
            hash = (hash * 397) ^ DataPath.GetHashCode();
            hash = (hash * 397) ^ Message.GetHashCode();
            return hash;
        }
    }

    /// <summary>A human-readable rendering. Not a parsing format.</summary>
    /// <returns>The severity, the code and the message.</returns>
    public override string ToString()
    {
        return Severity + " " + Code + ": " + Message;
    }
}
