using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Text;

/// <summary>
/// Picks the folio8 native library that matches THIS PROCESS'S BITNESS and
/// loads it by full path, once, before the first <c>DllImport</c> binds.
/// </summary>
/// <remarks>
/// <b>Why an explicit load rather than a resolver.</b> .NET Framework 4.6 is
/// the floor this library exists to reach, and it has no
/// <c>DllImportResolver</c> and no <c>System.Runtime.InteropServices</c>
/// native-library API at all. What every .NET from 1.1 to today does share is
/// that <c>DllImport</c> resolves a module by NAME, once, and binds to a
/// module of that name that is ALREADY LOADED in preference to searching. So
/// loading the chosen file by full path first makes all seven imports in
/// <see cref="Native"/> bind to it, and makes the failure ONE explainable
/// point rather than seven <see cref="DllNotFoundException"/>s.
/// <para>
/// <b>Why process bitness and not the RID graph.</b> A .NET Framework project
/// is AnyCPU by default: the same build runs as a 64-bit process on 64-bit
/// Windows and as a 32-bit process when <c>Prefer32Bit</c> is set or the host
/// is 32-bit. Nothing at build time knows which, so nothing at build time can
/// choose the asset. <see cref="IntPtr.Size"/> at load time knows exactly.
/// </para>
/// <para>
/// <b>There is no fallback.</b> When the asset for this bitness is absent or
/// refuses to load, the other architecture is NOT tried — it could not work,
/// and trying it would replace one clear failure with a confusing second one.
/// A <see cref="FolioNativeLoadException"/> is thrown naming the bitness, the
/// RID, the filename and every path probed.
/// </para>
/// <para>
/// Off Windows this does nothing at all, and that is still correct now that
/// the package ships Linux natives. There is no <c>net46</c> off Windows and
/// nothing is AnyCPU in the sense above, so the host resolves
/// <c>runtimes/&lt;rid&gt;/native/</c> from the RID graph before
/// <c>DllImport</c> ever probes — and a macOS or Linux <b>host</b> library
/// built by <c>build-native.sh host</c> (a development aid, never packaged)
/// is found by that same probing beside the test assembly. This type has
/// nothing to add on either path.
/// </para>
/// </remarks>
internal static class NativeLibraryLoader
{
    /// <summary>The Windows file name of the native library, in both RIDs.</summary>
    internal const string WindowsFileName = "folio8_native.dll";

    /// <summary>
    /// The subdirectory <c>build/folio8.targets</c> stages both
    /// architectures into beside a .NET Framework consumer's output.
    /// <c>runtimes/</c> is a modern-.NET mechanism and a 4.6 project's build
    /// does nothing with it, so that delivery needs a path of its own.
    /// </summary>
    internal const string FrameworkNativeFolder = "folio8-native";

    // ERROR_BAD_EXE_FORMAT. What Windows returns when a 64-bit process is
    // handed a 32-bit PE file, or the reverse — the single most likely way
    // this fails, and the one worth naming in the message.
    private const int ErrorBadExeFormat = 193;
    private const int ErrorModNotFound = 126;
    private const int ErrorAccessDenied = 5;

    private const uint LoadWithAlteredSearchPath = 0x00000008;

    private static readonly object Gate = new object();
    private static bool _loaded;

    /// <summary>
    /// Loads the native library for this process, once. Safe to call from any
    /// number of threads and cheap after the first success.
    /// </summary>
    /// <exception cref="FolioNativeLoadException">Nothing loadable was found.</exception>
    internal static void Ensure()
    {
        if (_loaded)
        {
            return;
        }
        lock (Gate)
        {
            if (_loaded)
            {
                return;
            }
            if (IsWindows())
            {
                Resolve(ProbeDirectories(), IntPtr.Size, File.Exists, LoadByPath);
            }
            _loaded = true;
        }
    }

    /// <summary>Whether this process is running on Windows.</summary>
    /// <remarks>
    /// <see cref="RuntimeInformation"/> would be the modern spelling and is
    /// unavailable on the floor: it arrived in .NET Framework 4.7.1.
    /// <see cref="Environment.OSVersion"/> is present everywhere from 1.1.
    /// </remarks>
    private static bool IsWindows()
    {
        PlatformID platform = Environment.OSVersion.Platform;
        return platform == PlatformID.Win32NT || platform == PlatformID.Win32Windows || platform == PlatformID.Win32S;
    }

    /// <summary>A path load attempt, factored out so tests can drive it.</summary>
    /// <param name="path">The full path to load.</param>
    /// <param name="win32Error">The error code when the result is zero.</param>
    /// <returns>The module handle, or <see cref="IntPtr.Zero"/>.</returns>
    internal delegate IntPtr NativeLoad(string path, out int win32Error);

    /// <summary>
    /// The whole decision, with its inputs injected so every branch — including
    /// a host that refuses P/Invoke outright — is reachable from a test on any
    /// operating system.
    /// </summary>
    /// <param name="directories">Where to look, in order.</param>
    /// <param name="pointerSize">This process's <see cref="IntPtr.Size"/>.</param>
    /// <param name="exists">How to ask whether a candidate is there.</param>
    /// <param name="load">How to load one.</param>
    /// <returns>The loaded module handle.</returns>
    internal static IntPtr Resolve(IList<string> directories, int pointerSize, Func<string, bool> exists, NativeLoad load)
    {
        string rid = RidFor(pointerSize);
        List<string> probed = new List<string>();

        foreach (string directory in directories)
        {
            foreach (string candidate in CandidatesIn(directory, rid))
            {
                probed.Add(candidate);
                if (!exists(candidate))
                {
                    continue;
                }

                // THE FIRST CANDIDATE THAT EXISTS IS THE ONE. A file that is
                // present under the name and place we looked in is the
                // consumer's answer to "which native library"; if it will not
                // load, that is the failure, not a reason to keep hunting.
                int win32Error;
                IntPtr handle;
                try
                {
                    handle = load(candidate, out win32Error);
                }
                catch (Exception blocked)
                {
                    // The P/Invoke into the loader ITSELF failed. A
                    // partial-trust or otherwise locked-down host is what does
                    // this, and it is a different problem from a bad file.
                    throw Failure(
                        pointerSize, rid, probed, candidate,
                        "the host refused the P/Invoke that loads native libraries. folio8 renders inside a native library, so it cannot run in a process where P/Invoke is blocked — full trust, and permission to load unmanaged code, are required.",
                        blocked);
                }
                if (handle != IntPtr.Zero)
                {
                    return handle;
                }
                throw Failure(pointerSize, rid, probed, candidate, CauseOf(win32Error, pointerSize, rid), Win32(win32Error));
            }
        }

        throw Failure(
            pointerSize, rid, probed, null,
            "no " + WindowsFileName + " for " + rid + " was found. The folio8 package carries a native library for each supported architecture; if the one this process needs is absent, the package's assets did not reach this output directory — check that the PackageReference restored, and that a publish or a deployment step did not drop runtimes/ or " + FrameworkNativeFolder + "/.",
            null);
    }

    /// <summary>The RID a process of this pointer size needs.</summary>
    internal static string RidFor(int pointerSize)
    {
        return pointerSize == 8 ? "win-x64" : "win-x86";
    }

    /// <summary>
    /// Where the native library may be, under one directory, in the order the
    /// three delivery mechanisms are trusted.
    /// </summary>
    private static IEnumerable<string> CandidatesIn(string directory, string rid)
    {
        // 1. The NuGet RID layout, as modern .NET stages it.
        yield return Path.Combine(Path.Combine(Path.Combine(Path.Combine(directory, "runtimes"), rid), "native"), WindowsFileName);
        // 2. What the package's build/folio8.targets stages for a .NET Framework consumer.
        yield return Path.Combine(Path.Combine(Path.Combine(directory, FrameworkNativeFolder), rid), WindowsFileName);
        // 3. Flat beside the assembly: a self-contained publish, or a build
        //    that staged the library by hand. LAST, because it carries no RID
        //    and so asserts nothing about its own architecture.
        yield return Path.Combine(directory, WindowsFileName);
    }

    /// <summary>
    /// The directories worth looking in: where the application was started
    /// from, where this assembly actually sits, and an ASP.NET-style
    /// <c>bin</c> below the first.
    /// </summary>
    private static IList<string> ProbeDirectories()
    {
        List<string> directories = new List<string>();
        Add(directories, AppDomain.CurrentDomain.BaseDirectory);
        Add(directories, AssemblyDirectory());
        string baseDirectory = AppDomain.CurrentDomain.BaseDirectory;
        if (!string.IsNullOrEmpty(baseDirectory))
        {
            Add(directories, Path.Combine(baseDirectory, "bin"));
        }
        return directories;
    }

    private static string AssemblyDirectory()
    {
        try
        {
            string location = typeof(NativeLibraryLoader).Assembly.Location;
            return string.IsNullOrEmpty(location) ? null : Path.GetDirectoryName(location);
        }
        catch (Exception)
        {
            // A host that hides the location (single-file, or a byte-array
            // load) is not an error here; the other directories still apply.
            return null;
        }
    }

    private static void Add(IList<string> directories, string directory)
    {
        if (string.IsNullOrEmpty(directory))
        {
            return;
        }
        string full;
        try
        {
            full = Path.GetFullPath(directory);
        }
        catch (Exception)
        {
            return;
        }
        foreach (string already in directories)
        {
            if (string.Equals(already, full, StringComparison.OrdinalIgnoreCase))
            {
                return;
            }
        }
        directories.Add(full);
    }

    private static string CauseOf(int win32Error, int pointerSize, string rid)
    {
        switch (win32Error)
        {
            case ErrorBadExeFormat:
                return "a BITNESS MISMATCH: this is a " + Bits(pointerSize) +
                    " process, so it needs the " + rid + " build, and the file found is not one. A " +
                    (pointerSize == 8 ? "32-bit" : "64-bit") +
                    " library cannot be loaded here at any cost — replace the file with the " + rid +
                    " asset from the folio8 package, or run the process at the other bitness.";
            case ErrorModNotFound:
                return "the file was found but one of ITS OWN dependencies was not. A folio8 native library needs only the Windows system libraries, so this usually means the file is not the folio8 engine, or is truncated.";
            case ErrorAccessDenied:
                return "the operating system refused access to the file. Check that the deployment's file permissions allow the process account to read and execute it.";
            default:
                return "Windows refused to load it (error " + Number(win32Error) + ").";
        }
    }

    private static Exception Win32(int win32Error)
    {
        return win32Error == 0 ? null : new System.ComponentModel.Win32Exception(win32Error);
    }

    private static string Bits(int pointerSize)
    {
        return pointerSize == 8 ? "64-bit" : "32-bit";
    }

    /// <summary>
    /// Builds the one exception this type throws: bitness, RID, filename,
    /// every path probed, and the likely cause — CAP-11's whole content.
    /// </summary>
    private static FolioNativeLoadException Failure(
        int pointerSize, string rid, IList<string> probed, string chosen, string cause, Exception inner)
    {
        StringBuilder message = new StringBuilder();
        message.Append("folio8: the native library could not be loaded.");
        message.Append(" Process bitness: ").Append(Bits(pointerSize)).Append(" (IntPtr.Size = ").Append(Number(pointerSize)).Append(").");
        message.Append(" Runtime identifier sought: ").Append(rid).Append('.');
        message.Append(" File name sought: ").Append(WindowsFileName).Append('.');
        if (chosen != null)
        {
            message.Append(" The file it tried to load: ").Append(chosen).Append('.');
        }
        message.Append(" Likely cause: ").Append(cause);
        message.Append(" Paths probed, in order:");
        if (probed.Count == 0)
        {
            message.Append(" (none — no probe directory could be determined)");
        }
        else
        {
            for (int i = 0; i < probed.Count; i++)
            {
                message.Append(Environment.NewLine).Append("  ").Append(probed[i]);
            }
        }
        return new FolioNativeLoadException(message.ToString(), rid, WindowsFileName, pointerSize, ToArray(probed), inner);
    }

    private static string[] ToArray(IList<string> values)
    {
        string[] copy = new string[values.Count];
        for (int i = 0; i < values.Count; i++)
        {
            copy[i] = values[i];
        }
        return copy;
    }

    private static string Number(long value)
    {
        return value.ToString(CultureInfo.InvariantCulture);
    }

    private static IntPtr LoadByPath(string path, out int win32Error)
    {
        IntPtr handle = LoadLibraryEx(path, IntPtr.Zero, LoadWithAlteredSearchPath);
        win32Error = handle == IntPtr.Zero ? Marshal.GetLastWin32Error() : 0;
        return handle;
    }

    // LOAD_WITH_ALTERED_SEARCH_PATH makes the library's OWN directory the
    // first place Windows looks for its dependencies, which is what a library
    // staged under runtimes/ or folio8-native/ needs.
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern IntPtr LoadLibraryEx(string fileName, IntPtr reserved, uint flags);
}
