using System;
using System.IO;
using System.Security.Cryptography;

/// <summary>
/// A CONSUMER, not a test. It knows nothing about this repository: it has a
/// PackageReference on folio-dotnet, it pastes the README's first-PDF snippet,
/// and it prints what it got. Everything it proves — that the right native
/// loaded for this process shape, that the shipped faces arrived, and that the
/// bytes are the corpus's — is decided by run-consumers.ps1 comparing this
/// output against the committed expected.json.
/// </summary>
/// <remarks>
/// ONE FILE, LINKED BY BOTH PROJECTS. The .NET Framework consumer and the
/// modern-.NET consumer compile the SAME source, because the claim under test
/// is that one managed assembly serves both families — a second, subtly
/// different program would let that claim pass while being false.
/// <para>
/// It prints rather than asserts, and never throws past Main. A consumer that
/// crashed with a stack trace and a consumer that reported a clean folio8
/// failure look identical in an exit code; the driver needs to tell them
/// apart, because CAP-11 is precisely the claim that the second is what
/// happens.
/// </para>
/// </remarks>
internal static class Program
{
    private static int Main(string[] args)
    {
        if (args.Length < 1)
        {
            Console.Error.WriteLine("usage: consumer <fixture directory>");
            return 64;
        }

        Console.WriteLine("BITNESS " + (IntPtr.Size * 8));

        try
        {
            string fixture = args[0];
            Template template = Template.Parse(File.ReadAllBytes(Path.Combine(fixture, "input.folio")));
            Data data = new Data(Optional(fixture, "data.json") ?? System.Text.Encoding.UTF8.GetBytes("{}"));
            byte[] rawParams = Optional(fixture, "params.json");
            Params parameters = rawParams == null ? null : new Params(rawParams);

            // INSIDE the try, deliberately, and not hoisted to a static field
            // the way the README tells an application to. A field initializer
            // throws TypeInitializationException, which would hide the
            // FolioNativeLoadException this program exists to report.
            FontSet fonts = Fonts.Shipped();
            Console.WriteLine("FACES " + fonts.Count);

            RenderResult result = Folio8.Render(template, data, parameters, fonts);

            Console.WriteLine("DIAGNOSTICS " + result.Diagnostics.Count);
            Console.WriteLine("SHA256 " + Hex(result.Bytes));
            Console.WriteLine("ENGINE " + Folio8.Version);
            return 0;
        }
        catch (FolioNativeLoadException load)
        {
            // CAP-11's whole point, printed intact — bitness, RID, file name,
            // every probed path and the likely cause — so the driver can
            // assert on the TEXT a developer would actually read.
            Console.WriteLine("FOLIO8-LOAD-FAILURE");
            Console.WriteLine("RID " + load.RuntimeIdentifier);
            Console.WriteLine("FILENAME " + load.FileName);
            Console.WriteLine("POINTERSIZE " + load.PointerSize);
            Console.WriteLine("PROBED " + load.ProbedPaths.Length);
            Console.WriteLine(load.Message);
            return 2;
        }
        catch (Exception other)
        {
            Console.WriteLine("UNEXPECTED " + other.GetType().FullName);
            Console.WriteLine(other);
            return 3;
        }
    }

    private static byte[] Optional(string fixture, string name)
    {
        string path = Path.Combine(fixture, name);
        return File.Exists(path) ? File.ReadAllBytes(path) : null;
    }

    private static string Hex(byte[] bytes)
    {
        using (SHA256 sha = SHA256.Create())
        {
            byte[] digest = sha.ComputeHash(bytes);
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
}
