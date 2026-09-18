using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Reflection;

/// <summary>
/// The shipped font set, packaged inside this assembly: the same eleven faces
/// Go's <c>fonts.Shipped()</c> returns, byte for byte.
/// </summary>
/// <remarks>
/// Fonts stay an EXPLICIT ARGUMENT. Nothing here is ambient and nothing here
/// is a default — <see cref="Folio8.Render(Template, Data, Params, FontSet)"/>
/// still demands a <see cref="FontSet"/>, and building your own instead of
/// calling this is a first-class way to use the library.
/// <para>
/// <b>The faces are embedded in this assembly</b>, not shipped beside it.
/// It is the one asset that has to reach .NET Framework and modern .NET
/// alike, and those two disagree about almost every file-delivery mechanism
/// NuGet has — <c>runtimes/</c>, content files, <c>None</c> items with
/// <c>CopyToOutputDirectory</c>. An embedded resource is simply there, in any
/// host, after any publish, with no MSBuild help at all. The cost is about
/// 14 MB of assembly, which is the deliberate "embed the full set" decision
/// rather than an accident — <c>NotoSansSC-Regular.ttf</c> alone is 10 MB
/// of it.
/// </para>
/// </remarks>
public static class Fonts
{
    /// <summary>
    /// The eleven faces, as <c>folio8-go/fonts/fonts.go</c> names them and in
    /// the order it lists them: the face name a template's fallback chain
    /// uses, then the resource this assembly embeds it under.
    /// </summary>
    /// <remarks>
    /// This table cannot drift silently. The pack step checks each face's
    /// byte length against the <c>shippedFaces</c> record the Go engine itself
    /// wrote into <c>folio-js/test/data/go-parity.json</c>, and refuses to
    /// produce a package if any face is missing, has drifted, or if the set is
    /// not exactly eleven; the test suite checks the same thing on every run.
    /// </remarks>
    private static readonly string[][] Faces =
    {
        new[] { "Noto Sans", "NotoSans-Regular.ttf" },
        new[] { "Noto Sans Bold", "NotoSans-Bold.ttf" },
        new[] { "Noto Sans Italic", "NotoSans-Italic.ttf" },
        new[] { "Noto Sans Bold Italic", "NotoSans-BoldItalic.ttf" },
        new[] { "Noto Sans Thai", "NotoSansThai-Regular.ttf" },
        new[] { "Noto Sans Thai Bold", "NotoSansThai-Bold.ttf" },
        new[] { "Noto Sans SC", "NotoSansSC-Regular.ttf" },
        new[] { "Roboto", "Roboto-Regular.ttf" },
        new[] { "Roboto Bold", "Roboto-Bold.ttf" },
        new[] { "Roboto Italic", "Roboto-Italic.ttf" },
        new[] { "Roboto Bold Italic", "Roboto-BoldItalic.ttf" },
    };

    /// <summary>The prefix Folio8.csproj embeds each face under.</summary>
    internal const string ResourcePrefix = "folio8.fonts.";

    private static readonly object Gate = new object();
    private static KeyValuePair<string, byte[]>[] _cached;

    /// <summary>
    /// The eleven faces <c>fonts.Shipped()</c> ships: Roboto and Noto Sans in
    /// regular, bold, italic and bold italic; Noto Sans Thai in regular and
    /// bold; and Noto Sans SC.
    /// </summary>
    /// <returns>A font set equal to the engine's, keys and bytes alike.</returns>
    /// <remarks>
    /// The resources are read ONCE for the life of the process. Every call
    /// then builds a fresh <see cref="FontSet"/> over its own copy of those
    /// bytes, so a caller that clears, replaces or edits the set it was handed
    /// cannot change what the next caller gets.
    /// <para>
    /// <b>CALL IT ONCE AND HOLD THE RESULT.</b> That isolation is not free:
    /// <see cref="FontSet"/> copies every face on the way in, so each call
    /// allocates about 14 MB. Assign it to a static field, or to whatever your
    /// application uses for long-lived state, and pass that same set to every
    /// render — calling it per render would copy 14 MB per document for no
    /// benefit. A font set is immutable as far as the engine is concerned and
    /// is safe to share across threads and across calls.
    /// </para>
    /// </remarks>
    /// <exception cref="InvalidOperationException">
    /// A face is missing from this assembly, or does not read back at its
    /// recorded length — which means the assembly was built without its font
    /// resources and cannot render with the shipped set.
    /// </exception>
    public static FontSet Shipped()
    {
        return new FontSet(Read());
    }

    private static KeyValuePair<string, byte[]>[] Read()
    {
        if (_cached != null)
        {
            return _cached;
        }
        lock (Gate)
        {
            if (_cached != null)
            {
                return _cached;
            }
            Assembly assembly = typeof(Fonts).Assembly;
            KeyValuePair<string, byte[]>[] faces = new KeyValuePair<string, byte[]>[Faces.Length];
            for (int i = 0; i < Faces.Length; i++)
            {
                string name = Faces[i][0];
                string resource = ResourcePrefix + Faces[i][1];
                faces[i] = new KeyValuePair<string, byte[]>(name, ReadResource(assembly, name, resource));
            }
            // Assigned only once the WHOLE set read, so a failure part-way
            // through is retried rather than cached as a half set.
            _cached = faces;
            return _cached;
        }
    }

    private static byte[] ReadResource(Assembly assembly, string face, string resource)
    {
        using (Stream stream = assembly.GetManifestResourceStream(resource))
        {
            if (stream == null)
            {
                throw new InvalidOperationException(
                    "folio8: the shipped face '" + face + "' is not embedded in this assembly (resource '" + resource +
                    "'). A folio-dotnet package always carries all eleven faces; an assembly built without them cannot serve Fonts.Shipped(). Pass your own FontSet instead, or reinstall the package.");
            }
            long length = stream.Length;
            if (length < 0 || length > int.MaxValue)
            {
                throw new InvalidOperationException(
                    "folio8: the embedded face '" + face + "' reports a length of " + length.ToString(CultureInfo.InvariantCulture) + " bytes, which cannot be read into memory.");
            }
            byte[] bytes = new byte[(int)length];
            int offset = 0;
            while (offset < bytes.Length)
            {
                int read = stream.Read(bytes, offset, bytes.Length - offset);
                if (read <= 0)
                {
                    throw new InvalidOperationException(
                        "folio8: the embedded face '" + face + "' is truncated — " + offset.ToString(CultureInfo.InvariantCulture) +
                        " of " + bytes.Length.ToString(CultureInfo.InvariantCulture) + " bytes could be read.");
                }
                offset += read;
            }
            return bytes;
        }
    }
}
