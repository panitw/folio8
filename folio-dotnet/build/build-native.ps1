<#
.SYNOPSIS
  Builds the folio8 engine as a native library from folio8-go/cshared/cmd/folio8.

.DESCRIPTION
  The Windows half of build-native.sh, with the same output layout:

    build\native\win-x64\folio8.dll
    build\native\win-x86\folio8.dll
    build\native\host\folio8.dll     (the native architecture, a development aid)

  Windows targets need a mingw-w64 toolchain PER ARCHITECTURE. A GitHub
  windows-2022 runner carries two independent ones:

    C:\mingw64\bin\gcc.exe          (x86_64, standalone, already on PATH)
    C:\msys64\mingw64\bin\gcc.exe   (x86_64, MSYS2)
    C:\msys64\mingw32\bin\gcc.exe   (i686,   MSYS2 — the only 32-bit one)

  and the MSYS2 ones can be installed or repaired with
  `C:\msys64\usr\bin\pacman -S --noconfirm --needed mingw-w64-x86_64-gcc mingw-w64-i686-gcc`.

  THE COMPILER'S OWN bin DIRECTORY IS PREPENDED TO PATH before the build.
  This is not tidiness. A mingw gcc.exe invoked by full path still loads its
  runtime DLLs (libiconv-2.dll, libintl-8.dll, zlib1.dll and friends) from
  beside itself, by the ordinary Windows search order — and when that search
  fails, the process dies at load time with NO stderr at all. cgo then reports
  only `cgo.exe: exit status 2` with no compiler diagnostic, which is exactly
  what the first Windows CI run produced.

  A candidate is also PROVED before it is used: each one compiles a trivial
  translation unit, and a compiler that cannot do that is rejected by name
  with its reason, and the next candidate is tried. That covers the other way
  this breaks — a `pacman -S` against a stale database leaving a partially
  upgraded toolchain whose gcc and its DLLs disagree.

  CGO_ENABLED and GOTOOLCHAIN are pinned here rather than inherited, for the
  reason build-native.sh gives.

.PARAMETER Targets
  Any of host, win-x64, win-x86, or all. Defaults to win-x64 and win-x86.
#>
[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]] $Targets = @('win-x64', 'win-x86')
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
# NESTED, not three-argument: Join-Path only takes more than two paths on
# PowerShell 6+, and this script has to run under Windows PowerShell 5.1 too,
# which is what a plain `powershell.exe` is on every Windows box.
$repo = Resolve-Path (Join-Path (Join-Path $here '..') '..')
$module = Join-Path $repo 'folio8-go'
$out = Join-Path $here 'native'
$pkg = './cshared/cmd/folio8'

$env:CGO_ENABLED = '1'
$env:GOTOOLCHAIN = 'go1.26.0'

# Expand `all` wherever it appears and VALIDATE THE WHOLE LIST BEFORE
# BUILDING ANYTHING, so `build-native.ps1 win-x64 bogus` fails before it
# produces a half-done run that looks like a whole one.
$expanded = @()
foreach ($target in $Targets) {
    switch ($target) {
        'all'     { $expanded += @('host', 'win-x64', 'win-x86') }
        'host'    { $expanded += 'host' }
        'win-x64' { $expanded += 'win-x64' }
        'win-x86' { $expanded += 'win-x86' }
        default   { throw "unknown target '$target' (host, win-x64, win-x86, all)" }
    }
}
$Targets = $expanded

# Runs a native command with its stderr surfaced into the transcript.
#
# PowerShell turns a native command's stderr into ErrorRecords, and under
# `$ErrorActionPreference = 'Stop'` the first of them can terminate the
# pipeline before the rest is ever written — which is one way a compiler
# diagnostic disappears from a CI log. Nothing is hidden here: the stream is
# merged, written verbatim, and the exit code is returned for the caller to
# judge.
function Invoke-Native {
    param([string] $Command, [string[]] $Arguments)
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & $Command @Arguments 2>&1 | ForEach-Object { Write-Host $_ }
        return $LASTEXITCODE
    }
    finally { $ErrorActionPreference = $previous }
}

# Captures a native command's combined output as text, for probing.
function Get-NativeOutput {
    param([string] $Command, [string[]] $Arguments)
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $text = (& $Command @Arguments 2>&1 | Out-String)
        return [pscustomobject]@{ ExitCode = $LASTEXITCODE; Text = $text.Trim() }
    }
    finally { $ErrorActionPreference = $previous }
}

# Compiles a trivial translation unit with $Cc, with $Cc's own directory
# prepended to PATH. Returns $null on success or the reason it failed.
function Test-Cc {
    param([string] $Cc)
    $dir = Split-Path -Parent $Cc
    $saved = $env:PATH
    $env:PATH = "$dir;$env:PATH"
    $work = Join-Path ([System.IO.Path]::GetTempPath()) ("folio8-cc-" + [Guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Force -Path $work | Out-Null
        $source = Join-Path $work 'probe.c'
        Set-Content -LiteralPath $source -Value 'int folio8_probe(void) { return 0; }' -Encoding ASCII

        $version = Get-NativeOutput $Cc @('--version')
        if ($version.ExitCode -ne 0) {
            return "it could not report its version (exit $($version.ExitCode)): $($version.Text)"
        }

        $compile = Get-NativeOutput $Cc @('-c', $source, '-o', (Join-Path $work 'probe.o'))
        if ($compile.ExitCode -ne 0) {
            return "it could not compile a trivial file (exit $($compile.ExitCode)): $($compile.Text)"
        }
        return $null
    }
    finally {
        $env:PATH = $saved
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $work
    }
}

# Returns the first candidate that EXISTS and WORKS, or throws naming every
# one it tried and why each was rejected.
function Resolve-Cc {
    param([string] $Target, [string[]] $Candidates)
    Write-Host "--- $Target compiler probe ---"
    $rejected = @()
    foreach ($candidate in $Candidates) {
        $path = $null
        if (Test-Path -LiteralPath $candidate) {
            $path = (Resolve-Path -LiteralPath $candidate).Path
        }
        else {
            $found = Get-Command $candidate -ErrorAction SilentlyContinue
            if ($found) { $path = $found.Source }
        }
        if (-not $path) {
            $rejected += "  $candidate — not found"
            continue
        }
        $why = Test-Cc $path
        if ($why) {
            $rejected += "  $path — $why"
            continue
        }
        $probe = Get-NativeOutput $path @('--version')
        $firstLine = $probe.Text.Split("`n")[0].Trim()
        Write-Host "    compiler: $path"
        Write-Host "    version:  $firstLine"
        return $path
    }
    $tried = $rejected -join [Environment]::NewLine
    throw ("${Target}: no working mingw-w64 gcc." + [Environment]::NewLine + "Tried:" + [Environment]::NewLine + $tried)
}

function Build-Target {
    param([string] $Target, [string] $Goarch, [string] $Cc)
    $dir = Join-Path $out $Target
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $artifact = Join-Path $dir 'folio8.dll'
    # A FAILED BUILD MUST NOT LEAVE THE PREVIOUS ARTIFACT IN PLACE: go build
    # writes nothing when it fails, and a stale DLL tested as if fresh is the
    # most expensive kind of green. The generated header goes with it.
    Remove-Item -Force -ErrorAction SilentlyContinue $artifact, (Join-Path $dir 'folio8.h')
    Write-Host "==> $Target`: $artifact"

    $ccDir = Split-Path -Parent $Cc
    $savedPath = $env:PATH
    # THE FIX FOR `cgo.exe: exit status 2` WITH NO DIAGNOSTIC: gcc loads its
    # own runtime DLLs from beside itself, and cannot start without them.
    $env:PATH = "$ccDir;$env:PATH"
    $env:GOOS = 'windows'
    $env:GOARCH = $Goarch
    $env:CC = $Cc
    Write-Host "    PATH head: $ccDir"

    Push-Location $module
    try {
        # -trimpath keeps the build reproducible across checkout locations.
        $arguments = @('build', '-v', '-trimpath', '-buildmode=c-shared', '-o', $artifact, $pkg)
        $code = Invoke-Native 'go' $arguments
        if ($code -ne 0) {
            # DIAGNOSE BEFORE FAILING. A bare "exit status 2" from cgo says
            # nothing a reader can act on, so the environment cgo actually saw
            # is dumped, and the build is repeated with -x so the exact
            # compiler invocation appears in the log.
            Write-Host "--- go build failed for $Target (exit $code); environment cgo saw ---"
            # -json so each value arrives NAMED; the bare form prints a
            # column of values a reader has to count against the argument list.
            Invoke-Native 'go' @('env', '-json', 'GOOS', 'GOARCH', 'CC', 'CGO_ENABLED', 'CGO_CFLAGS', 'CGO_LDFLAGS', 'GOTOOLCHAIN', 'GOROOT') | Out-Null
            Write-Host "--- mingw/msys/go entries on PATH ---"
            foreach ($entry in ($env:PATH -split ';')) {
                if ($entry -match 'mingw|msys|\\go\\|Go\\bin') { Write-Host "  $entry" }
            }
            Write-Host "--- retrying with -x to show the cgo and compiler invocations ---"
            Invoke-Native 'go' (@('build', '-x') + $arguments[2..($arguments.Length - 1)]) | Out-Null
            throw "go build failed for $Target (exit $code) using CC=$Cc. The retry above shows the commands cgo ran; a compiler that starts but cannot compile prints a diagnostic there, and one that cannot start at all prints none."
        }
        if (-not (Test-Path -LiteralPath $artifact)) {
            throw "go build reported success for $Target but produced no $artifact"
        }
    }
    finally {
        Pop-Location
        $env:PATH = $savedPath
    }
}

# ORDER MATTERS. C:\mingw64 is the runner image's standalone mingw-w64: it is
# already on PATH, self-contained, and involves MSYS2 not at all, so it cannot
# be caught by a half-finished pacman transaction. MSYS2's own x86_64 gcc is
# the fallback. There is no standalone 32-bit mingw on the image, so win-x86
# has MSYS2 and nothing else — which is why Resolve-Cc proves it works rather
# than assuming it does.
$x64Candidates = @('C:\mingw64\bin\gcc.exe', 'C:\msys64\mingw64\bin\gcc.exe', 'x86_64-w64-mingw32-gcc', 'gcc')
$x86Candidates = @('C:\msys64\mingw32\bin\gcc.exe', 'C:\mingw32\bin\gcc.exe', 'i686-w64-mingw32-gcc')

foreach ($target in $Targets) {
    switch ($target) {
        'host' {
            # Is64BitOperatingSystem is TRUE on Windows arm64 as well, so it
            # cannot tell amd64 from arm64 and would silently build an x64
            # library on an arm64 developer machine. Switch on the actual
            # OS architecture instead.
            switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
                'X64'   { $arch = 'amd64'; $candidates = $x64Candidates }
                'X86'   { $arch = '386';   $candidates = $x86Candidates }
                'Arm64' { throw "host: windows/arm64 is out of scope for this spec; build win-x64 or win-x86 explicitly" }
                default { throw "host: unsupported OS architecture $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
            }
            Build-Target -Target 'host' -Goarch $arch -Cc (Resolve-Cc 'host' $candidates)
        }
        'win-x64' {
            Build-Target -Target 'win-x64' -Goarch 'amd64' -Cc (Resolve-Cc 'win-x64' $x64Candidates)
        }
        'win-x86' {
            Build-Target -Target 'win-x86' -Goarch '386' -Cc (Resolve-Cc 'win-x86' $x86Candidates)
        }
    }
}
