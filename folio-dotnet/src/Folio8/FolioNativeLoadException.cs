using System;
using System.Runtime.Serialization;

/// <summary>
/// Thrown when the folio8 native library cannot be loaded for this process.
/// </summary>
/// <remarks>
/// folio8 renders inside a native library, so this is not a degraded mode: no
/// render and no validate can run after it. It exists so the failure arrives
/// as a sentence a developer can act on — the process bitness detected, the
/// runtime identifier and file name sought, every path probed and the likely
/// cause — rather than as a platform <see cref="DllNotFoundException"/> or a
/// <see cref="BadImageFormatException"/> naming only a symbol.
/// <para>
/// The three causes worth telling apart, all of which land here: a BITNESS
/// MISMATCH (an AnyCPU project running 64-bit against the x86 asset, or the
/// reverse), a MISSING RID ASSET (the package's <c>runtimes/</c> or
/// <c>folio8-native/</c> files did not reach the output directory), and a
/// P/INVOKE BLOCKED BY THE HOST (a partial-trust or locked-down process).
/// <c>Message</c> says which.
/// </para>
/// </remarks>
[Serializable]
public sealed class FolioNativeLoadException : Exception
{
    /// <summary>The runtime identifier this process needed: <c>win-x86</c> or <c>win-x64</c>.</summary>
    public string RuntimeIdentifier { get; private set; }

    /// <summary>The native library file name that was looked for.</summary>
    public string FileName { get; private set; }

    /// <summary>The process's pointer size in bytes — 8 for a 64-bit process, 4 for a 32-bit one.</summary>
    public int PointerSize { get; private set; }

    /// <summary>Every path probed, in the order they were tried.</summary>
    /// <remarks>A copy: the array handed back is never the one the exception holds.</remarks>
    public string[] ProbedPaths
    {
        get
        {
            string[] copy = new string[_probedPaths.Length];
            Array.Copy(_probedPaths, copy, _probedPaths.Length);
            return copy;
        }
    }

    private readonly string[] _probedPaths;

    /// <summary>Creates the exception with a message alone.</summary>
    /// <param name="message">The diagnostic sentence.</param>
    /// <remarks>
    /// The conventional overload. folio-dotnet itself always uses the full
    /// constructor below, because a load failure with no RID and no probed
    /// paths is exactly the unactionable message CAP-11 exists to replace.
    /// </remarks>
    public FolioNativeLoadException(string message)
        : this(message, null, null, IntPtr.Size, null, null)
    {
    }

    /// <summary>Creates the exception with a message and an inner exception.</summary>
    /// <param name="message">The diagnostic sentence.</param>
    /// <param name="inner">The error underneath.</param>
    public FolioNativeLoadException(string message, Exception inner)
        : this(message, null, null, IntPtr.Size, null, inner)
    {
    }

    /// <summary>Creates the exception.</summary>
    /// <param name="message">The full diagnostic sentence.</param>
    /// <param name="runtimeIdentifier">The RID sought.</param>
    /// <param name="fileName">The file name sought.</param>
    /// <param name="pointerSize">The detected pointer size.</param>
    /// <param name="probedPaths">The paths probed, in order.</param>
    /// <param name="inner">The platform error underneath, when there was one.</param>
    public FolioNativeLoadException(string message, string runtimeIdentifier, string fileName, int pointerSize, string[] probedPaths, Exception inner)
        : base(message, inner)
    {
        RuntimeIdentifier = runtimeIdentifier;
        FileName = fileName;
        PointerSize = pointerSize;
        _probedPaths = probedPaths ?? new string[0];
    }

    /// <summary>Deserialization constructor.</summary>
    /// <param name="info">The serialized data.</param>
    /// <param name="context">The streaming context.</param>
    private FolioNativeLoadException(SerializationInfo info, StreamingContext context)
        : base(info, context)
    {
        RuntimeIdentifier = info.GetString("RuntimeIdentifier");
        FileName = info.GetString("FileName");
        PointerSize = info.GetInt32("PointerSize");
        _probedPaths = (string[])info.GetValue("ProbedPaths", typeof(string[])) ?? new string[0];
    }

    /// <summary>Serializes the exception.</summary>
    /// <param name="info">The destination.</param>
    /// <param name="context">The streaming context.</param>
    /// <exception cref="ArgumentNullException"><paramref name="info"/> is null.</exception>
    public override void GetObjectData(SerializationInfo info, StreamingContext context)
    {
        if (info == null)
        {
            throw new ArgumentNullException("info");
        }
        base.GetObjectData(info, context);
        info.AddValue("RuntimeIdentifier", RuntimeIdentifier);
        info.AddValue("FileName", FileName);
        info.AddValue("PointerSize", PointerSize);
        info.AddValue("ProbedPaths", _probedPaths, typeof(string[]));
    }
}
