<#
.SYNOPSIS
  Builds the folio8 engine as a native library from folio8-go/cshared/cmd/folio8.

.DESCRIPTION
  The Windows half of build-native.sh, with the same output layout:

    build\native\win-x64\folio8.dll
    build\native\win-x86\folio8.dll
    build\native\host\folio8.dll     (the native architecture, a development aid)

  Windows targets need a mingw-w64 toolchain PER ARCHITECTURE. On a GitHub
  windows runner both live under MSYS2:

    C:\msys64\mingw64\bin\gcc.exe   (x86_64)
    C:\msys64\mingw32\bin\gcc.exe   (i686)

  and can be installed or repaired with
  `C:\msys64\usr\bin\pacman -S --noconfirm --needed mingw-w64-x86_64-gcc mingw-w64-i686-gcc`.

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

function Find-Cc {
    param([string[]] $Candidates)
    foreach ($candidate in $Candidates) {
        if (Test-Path -LiteralPath $candidate) { return (Resolve-Path -LiteralPath $candidate).Path }
        $found = Get-Command $candidate -ErrorAction SilentlyContinue
        if ($found) { return $found.Source }
    }
    return $null
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
    $env:GOOS = 'windows'
    $env:GOARCH = $Goarch
    $env:CC = $Cc
    Push-Location $module
    try {
        # -trimpath keeps the build reproducible across checkout locations.
        & go build -trimpath -buildmode=c-shared -o $artifact $pkg
        if ($LASTEXITCODE -ne 0) { throw "go build failed for $Target (exit $LASTEXITCODE)" }
    }
    finally { Pop-Location }
}

$x64Candidates = @('C:\msys64\mingw64\bin\gcc.exe', 'C:\mingw64\bin\gcc.exe', 'x86_64-w64-mingw32-gcc')
$x86Candidates = @('C:\msys64\mingw32\bin\gcc.exe', 'C:\mingw32\bin\gcc.exe', 'i686-w64-mingw32-gcc')

foreach ($target in $Targets) {
    switch ($target) {
        'host' {
            # Is64BitOperatingSystem is TRUE on Windows arm64 as well, so it
            # cannot tell amd64 from arm64 and would silently build an x64
            # library on an arm64 developer machine. Switch on the actual
            # process architecture instead.
            switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
                'X64'   { $arch = 'amd64'; $candidates = $x64Candidates }
                'X86'   { $arch = '386';   $candidates = $x86Candidates }
                'Arm64' { throw "host: windows/arm64 is out of scope for this spec; build win-x64 or win-x86 explicitly" }
                default { throw "host: unsupported OS architecture $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
            }
            $cc = Find-Cc $candidates
            if (-not $cc) { throw "host: no mingw-w64 gcc found. Tried: $($candidates -join ', ')" }
            Build-Target -Target 'host' -Goarch $arch -Cc $cc
        }
        'win-x64' {
            $cc = Find-Cc $x64Candidates
            if (-not $cc) { throw "win-x64: no 64-bit mingw-w64 gcc found. Tried: $($x64Candidates -join ', ')" }
            Build-Target -Target 'win-x64' -Goarch 'amd64' -Cc $cc
        }
        'win-x86' {
            $cc = Find-Cc $x86Candidates
            if (-not $cc) { throw "win-x86: no 32-bit mingw-w64 gcc found. Tried: $($x86Candidates -join ', ')" }
            Build-Target -Target 'win-x86' -Goarch '386' -Cc $cc
        }
    }
}
