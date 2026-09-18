using System;
using System.Text;

/// <summary>
/// The JSON report data a render binds against.
/// </summary>
/// <remarks>
/// It wraps bytes or a string and nothing else. .NET Framework 4.6 has no
/// <c>System.Text.Json</c>, and this library takes no third-party
/// dependency, so the choice of serialiser stays with the caller — hand it
/// the UTF-8 bytes or the string your serialiser produced.
/// <para>
/// Both an implicit conversion from <see cref="byte"/>[] and one from
/// <see cref="string"/> exist, so a call site reads
/// <c>Folio8.Render(template, json, null, fonts)</c>.
/// </para>
/// </remarks>
public sealed class Data
{
    private readonly byte[] _json;

    /// <summary>Wraps UTF-8 JSON bytes.</summary>
    /// <param name="json">The bytes. Copied, so later edits cannot reach the engine.</param>
    /// <exception cref="ArgumentNullException"><paramref name="json"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="json"/> is not valid UTF-8.</exception>
    public Data(byte[] json)
    {
        _json = Utf8.Checked(json, "json");
    }

    /// <summary>Wraps a JSON string, encoded as UTF-8.</summary>
    /// <param name="json">The JSON text.</param>
    /// <exception cref="ArgumentNullException"><paramref name="json"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="json"/> cannot be encoded as UTF-8 — an unpaired surrogate, for instance.</exception>
    public Data(string json)
    {
        _json = Utf8.Encoded(json, "json");
    }

    /// <summary>Wraps UTF-8 JSON bytes.</summary>
    /// <param name="json">The bytes.</param>
    /// <returns>The wrapper, or null when <paramref name="json"/> is null.</returns>
    public static implicit operator Data(byte[] json)
    {
        return json == null ? null : new Data(json);
    }

    /// <summary>Wraps a JSON string.</summary>
    /// <param name="json">The JSON text.</param>
    /// <returns>The wrapper, or null when <paramref name="json"/> is null.</returns>
    public static implicit operator Data(string json)
    {
        return json == null ? null : new Data(json);
    }

    internal byte[] Bytes
    {
        get { return _json; }
    }
}

/// <summary>
/// The runtime values that are not report data — <c>documentDate</c> among
/// them. Same shape as <see cref="Data"/>.
/// </summary>
/// <remarks>
/// Passing null means the engine receives no params. That is the SAME thing
/// as receiving empty ones: the engine maps any params of length zero onto
/// the empty params object, so absent and empty are not distinguishable and
/// nothing in this library pretends otherwise.
/// </remarks>
public sealed class Params
{
    private readonly byte[] _json;

    /// <summary>Wraps UTF-8 JSON bytes.</summary>
    /// <param name="json">The bytes. Copied, so later edits cannot reach the engine.</param>
    /// <exception cref="ArgumentNullException"><paramref name="json"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="json"/> is not valid UTF-8.</exception>
    public Params(byte[] json)
    {
        _json = Utf8.Checked(json, "json");
    }

    /// <summary>Wraps a JSON string, encoded as UTF-8.</summary>
    /// <param name="json">The JSON text.</param>
    /// <exception cref="ArgumentNullException"><paramref name="json"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="json"/> cannot be encoded as UTF-8 — an unpaired surrogate, for instance.</exception>
    public Params(string json)
    {
        _json = Utf8.Encoded(json, "json");
    }

    /// <summary>Wraps UTF-8 JSON bytes.</summary>
    /// <param name="json">The bytes.</param>
    /// <returns>The wrapper, or null when <paramref name="json"/> is null.</returns>
    public static implicit operator Params(byte[] json)
    {
        return json == null ? null : new Params(json);
    }

    /// <summary>Wraps a JSON string.</summary>
    /// <param name="json">The JSON text.</param>
    /// <returns>The wrapper, or null when <paramref name="json"/> is null.</returns>
    public static implicit operator Params(string json)
    {
        return json == null ? null : new Params(json);
    }

    internal byte[] Bytes
    {
        get { return _json; }
    }
}

/// <summary>
/// UTF-8 at the managed boundary, STRICT IN BOTH DIRECTIONS, so bad input is
/// an <see cref="ArgumentException"/> here rather than a decode failure
/// inside the engine — or, worse, a silent substitution.
/// </summary>
/// <remarks>
/// The default <see cref="Encoding.UTF8"/> replaces what it cannot encode
/// with U+FFFD and says nothing. Since the implicit conversion from
/// <see cref="string"/> makes that the DEFAULT path into this library, a
/// string holding an unpaired surrogate would otherwise reach the engine as
/// different bytes than the caller wrote — while the byte[] path threw for
/// the mirror-image problem. Both are strict.
/// </remarks>
internal static class Utf8
{
    private static readonly UTF8Encoding Strict = new UTF8Encoding(false, true);

    internal static byte[] Encoded(string json, string name)
    {
        if (json == null)
        {
            throw new ArgumentNullException(name);
        }
        try
        {
            return Strict.GetBytes(json);
        }
        catch (EncoderFallbackException error)
        {
            throw new ArgumentException("folio8: " + name + " cannot be encoded as UTF-8 (it holds an unpaired surrogate or another unencodable character)", name, error);
        }
    }

    internal static byte[] Checked(byte[] json, string name)
    {
        if (json == null)
        {
            throw new ArgumentNullException(name);
        }
        try
        {
            Strict.GetCharCount(json, 0, json.Length);
        }
        catch (DecoderFallbackException error)
        {
            throw new ArgumentException("folio8: " + name + " is not valid UTF-8", name, error);
        }
        byte[] copy = new byte[json.Length];
        Buffer.BlockCopy(json, 0, copy, 0, json.Length);
        return copy;
    }
}
