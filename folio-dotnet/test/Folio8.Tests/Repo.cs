using System;
using System.Collections.Generic;
using System.IO;
using System.Security.Cryptography;

namespace Folio8Tests
{
    /// <summary>
    /// Where the fixtures, the recorded Go expectations and the shipped faces
    /// live, and how to read them. Everything these tests compare against is
    /// committed evidence produced by the Go engine — never a value restated
    /// here.
    /// </summary>
    internal static class Repo
    {
        private static readonly Lazy<string> RootPath = new Lazy<string>(FindRoot);
        private static readonly Lazy<FontSet> Shipped = new Lazy<FontSet>(LoadShippedFaces);

        /// <summary>The repository root, found by walking up from the test assembly.</summary>
        internal static string Root
        {
            get { return RootPath.Value; }
        }

        private static string FindRoot()
        {
            DirectoryInfo dir = new DirectoryInfo(AppContext.BaseDirectory);
            while (dir != null)
            {
                if (Directory.Exists(Path.Combine(dir.FullName, "fixtures")) &&
                    Directory.Exists(Path.Combine(dir.FullName, "folio8-go")))
                {
                    return dir.FullName;
                }
                dir = dir.Parent;
            }
            throw new InvalidOperationException(
                "folio-dotnet tests: could not find the repository root above " + AppContext.BaseDirectory);
        }

        internal static string Path_(params string[] parts)
        {
            string path = Root;
            foreach (string part in parts)
            {
                path = System.IO.Path.Combine(path, part);
            }
            return path;
        }

        internal static byte[] File_(params string[] parts)
        {
            return File.ReadAllBytes(Path_(parts));
        }

        internal static string Text(params string[] parts)
        {
            return File.ReadAllText(Path_(parts));
        }

        internal static string Sha256(byte[] bytes)
        {
            using (SHA256 hash = SHA256.Create())
            {
                byte[] digest = hash.ComputeHash(bytes);
                char[] hex = new char[digest.Length * 2];
                const string alphabet = "0123456789abcdef";
                for (int i = 0; i < digest.Length; i++)
                {
                    hex[i * 2] = alphabet[digest[i] >> 4];
                    hex[i * 2 + 1] = alphabet[digest[i] & 0xF];
                }
                return new string(hex);
            }
        }

        /// <summary>
        /// The eleven faces <c>fonts.Shipped()</c> returns, as
        /// <c>folio8-go/fonts/fonts.go</c> names them — the same table
        /// folio-js keeps in <c>scripts/faces.mjs</c>, read straight from the
        /// Go tree rather than from a package that does not exist yet
        /// (embedding is story 7).
        /// <para>
        /// This table cannot drift silently: ParityTests checks every entry,
        /// by name and byte length, against the <c>shippedFaces</c> block the
        /// Go engine itself recorded into
        /// <c>folio-js/test/data/go-parity.json</c>.
        /// </para>
        /// </summary>
        internal static readonly KeyValuePair<string, string>[] ShippedFaceFiles =
        {
            new KeyValuePair<string, string>("Noto Sans", "notosans/NotoSans-Regular.ttf"),
            new KeyValuePair<string, string>("Noto Sans Bold", "notosans-bold/NotoSans-Bold.ttf"),
            new KeyValuePair<string, string>("Noto Sans Italic", "notosans-italic/NotoSans-Italic.ttf"),
            new KeyValuePair<string, string>("Noto Sans Bold Italic", "notosans-bolditalic/NotoSans-BoldItalic.ttf"),
            new KeyValuePair<string, string>("Noto Sans Thai", "notosansthai/NotoSansThai-Regular.ttf"),
            new KeyValuePair<string, string>("Noto Sans Thai Bold", "notosansthai-bold/NotoSansThai-Bold.ttf"),
            new KeyValuePair<string, string>("Noto Sans SC", "notosanssc/NotoSansSC-Regular.ttf"),
            new KeyValuePair<string, string>("Roboto", "roboto/Roboto-Regular.ttf"),
            new KeyValuePair<string, string>("Roboto Bold", "roboto-bold/Roboto-Bold.ttf"),
            new KeyValuePair<string, string>("Roboto Italic", "roboto-italic/Roboto-Italic.ttf"),
            new KeyValuePair<string, string>("Roboto Bold Italic", "roboto-bolditalic/Roboto-BoldItalic.ttf"),
        };

        /// <summary>The shipped font set, loaded once per test process.</summary>
        internal static FontSet ShippedFonts
        {
            get { return Shipped.Value; }
        }

        private static FontSet LoadShippedFaces()
        {
            FontSet set = new FontSet();
            foreach (KeyValuePair<string, string> face in ShippedFaceFiles)
            {
                string[] segments = face.Value.Split('/');
                set.Add(face.Key, File_("folio8-go", "fonts", segments[0], segments[1]));
            }
            return set;
        }
    }
}
