<#
.SYNOPSIS
  Packs the folio8 NuGet package, installs it into real consumer projects covering every
  supported process shape on both target families, renders a corpus fixture in
  each, and forces the CAP-11 failure modes a consumer can actually produce.

.DESCRIPTION
  This is the story's evidence, not its assertion. CAP-7 says a PackageReference
  and the documented snippet produce a correct PDF from an AnyCPU project on
  64-bit Windows, an AnyCPU project forced 32-bit and an explicitly x86
  project, on both net46-family and modern .NET, with no caller-authored load
  logic. Nothing in the unit suite can show that: it runs inside a test host
  that already staged a native library by hand. Only a project that installed
  the package can.

  EVERY LEG IS A `dotnet publish`, which is what a consumer ships, and each one
  gets its OWN intermediate directory as well as its own output directory.
  That second half is not tidiness: three legs build the SAME project with
  different PlatformTarget/Prefer32Bit settings, and with a shared obj/ MSBuild
  finds the compile up to date and skips it -- so the 32-bit legs would run the
  64-bit executable the first leg left behind and the bitness proof would be
  vacuous.

  Seven legs, three process shapes on each family:

    net48   AnyCPU                          -> 64-bit -> win-x64
    net48   AnyCPU + Prefer32Bit            -> 32-bit -> win-x86
    net48   PlatformTarget=x86              -> 32-bit -> win-x86
    modern  RID-agnostic                    -> 64-bit -> win-x64
    modern  self-contained win-x64          -> 64-bit -> win-x64
    modern  win-x86 + Prefer32Bit           -> 32-bit -> win-x86
    modern  win-x86 + PlatformTarget=x86    -> 32-bit -> win-x86

  The two 32-bit modern legs name a RID as well as the platform target because
  modern .NET picks its apphost's bitness from the RID, not from
  PlatformTarget, and setup-dotnet installs no 32-bit runtime on the runner --
  so they are self-contained and bring their own.

  Each leg renders the fixture; its SHA-256 must equal the committed
  expected.json and its ENGINE line must equal the version the Go engine
  recorded. Then two CAP-11 modes are FORCED, IN BOTH DIRECTIONS -- against a
  64-bit and a 32-bit leg on each family -- by deleting the native and by
  replacing it with the other architecture's file. The third mode, a host that
  blocks P/Invoke, cannot be produced in an ordinary console process and is
  covered in Folio8.Tests/LoaderTests.cs, which drives the same code path with
  the platform call injected.

  WINDOWS ONLY, deliberately: .NET Framework is Windows-only by definition and
  both natives are Windows PE files.

.PARAMETER Fixture
  The corpus fixture to render. Its expected.json supplies the hash.
#>
[CmdletBinding()]
param(
  [string] $Fixture = 'colour-strokes'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$repo = (Resolve-Path (Join-Path $here '../../..')).Path
$feed = Join-Path $repo 'artifacts/nupkg'
$work = Join-Path $repo 'artifacts/consumers'
$fixtureDir = Join-Path $repo "fixtures/$Fixture"

$expected = (Get-Content (Join-Path $fixtureDir 'expected.json') -Raw | ConvertFrom-Json).sha256
if (-not $expected) { throw "no sha256 in $fixtureDir/expected.json; this run would assert nothing" }
$engine = (Get-Content (Join-Path $repo 'folio-js/test/data/go-parity.json') -Raw | ConvertFrom-Json).folio8Version
if (-not $engine) { throw 'no folio8Version in folio-js/test/data/go-parity.json; this run would assert nothing' }
Write-Host "expected SHA-256 for $Fixture : $expected"
Write-Host "expected engine version       : $engine"

$failures = New-Object System.Collections.Generic.List[string]
function Fail([string] $what) { Write-Host "::error::$what"; $failures.Add($what) }

# ---------------------------------------------------------------- pack ----
# A FRESH PACKAGE EVERY RUN, and the global cache purged of the id first.
# NuGet caches by id+version, and this package's version does not move between
# commits -- so without the purge a second run on the same machine would
# install the FIRST run's package and report on code that is no longer there.
Remove-Item -Recurse -Force $feed -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
$packages = if ($env:NUGET_PACKAGES) { $env:NUGET_PACKAGES } else { Join-Path $HOME '.nuget/packages' }
Remove-Item -Recurse -Force (Join-Path $packages 'folio8') -ErrorAction SilentlyContinue

Write-Host '==> dotnet pack'
& dotnet pack (Join-Path $repo 'folio-dotnet/src/Folio8/Folio8.csproj') -c Release -o $feed
if ($LASTEXITCODE -ne 0) { throw 'dotnet pack failed' }

$nupkg = Get-ChildItem $feed -Filter '*.nupkg' | Select-Object -First 1
if (-not $nupkg) { throw "dotnet pack produced no .nupkg in $feed" }

# THE VERSION IS READ OFF THE PACKAGE, never typed here. Folio8.csproj is the
# one place it is declared; the consumer projects take it from the command
# line, so a bump is one edit and nothing downstream reddens.
if ($nupkg.Name -notmatch '^folio8\.(?<v>\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?)\.nupkg$') {
  throw "cannot read a version out of $($nupkg.Name)"
}
$version = $Matches['v']
Write-Host "packed $($nupkg.Name)  $([math]::Round($nupkg.Length / 1MB, 1)) MB  (version $version)"

# The package's own contents, asserted before a consumer ever sees it. A
# missing entry here is a far more legible failure than seven consumer legs
# all reporting a load failure -- and the counted entries are promises
# RELEASING.md and the README make that no consumer leg would ever notice
# were they dropped.
Add-Type -AssemblyName System.IO.Compression.FileSystem
$archive = [IO.Compression.ZipFile]::OpenRead($nupkg.FullName)
try {
  $entries = @($archive.Entries.FullName)
  $nuspec = $archive.GetEntry('folio8.nuspec')
  $nuspecText = if ($nuspec) { (New-Object IO.StreamReader($nuspec.Open())).ReadToEnd() } else { $null }
} finally {
  $archive.Dispose()
}

foreach ($required in @(
    'lib/netstandard2.0/Folio8.dll',
    'runtimes/win-x64/native/folio8_native.dll',
    'runtimes/win-x86/native/folio8_native.dll',
    'build/folio8.targets',
    'buildTransitive/folio8.targets',
    'README.md',
    'LICENSE')) {
  if ($entries -notcontains $required) { Fail "the package does not contain $required" }
}

# AD-26: a face's terms and notice travel with it. Eleven of each, counted --
# the npm package's test counts the same two files beside every face, and for
# the same reason: dropping the glob from the csproj is invisible to every
# other check in this suite.
foreach ($pair in @(@('LICENSE-OFL.txt', 11), @('NOTICE.md', 11))) {
  $found = @($entries | Where-Object { $_ -like "third-party-notices/fonts/*/$($pair[0])" }).Count
  if ($found -ne $pair[1]) { Fail "the package carries $found third-party-notices/fonts/*/$($pair[0]); it must carry $($pair[1]), one per shipped face" }
}

# NO RUNTIME DEPENDENCIES. SurfaceTests greps the csproj for PackageReference,
# which is the INPUT; this is the OUTPUT, and only it can see a dependency the
# SDK inferred.
if (-not $nuspecText) { Fail 'the package carries no .nuspec' }
elseif ($nuspecText -match '<dependency\s') { Fail 'the packed .nuspec declares a dependency; folio8 takes none' }

# ----------------------------------------------------------- the legs -----
# Rid is what each shape MUST resolve to, and it is what the forced failure
# modes below assert against -- the bitness the process reports and the RID the
# loader names are two different claims.
$legs = @(
  @{ Name = 'net48-anycpu-64';   Project = 'Consumer.Net48';  Bits = 64; Rid = 'win-x64'; Props = @() }
  @{ Name = 'net48-prefer32';    Project = 'Consumer.Net48';  Bits = 32; Rid = 'win-x86'; Props = @('-p:Prefer32Bit=true') }
  @{ Name = 'net48-x86';         Project = 'Consumer.Net48';  Bits = 32; Rid = 'win-x86'; Props = @('-p:PlatformTarget=x86') }
  @{ Name = 'modern-anycpu-64';  Project = 'Consumer.Modern'; Bits = 64; Rid = 'win-x64'; Props = @() }
  @{ Name = 'modern-x64';        Project = 'Consumer.Modern'; Bits = 64; Rid = 'win-x64'; Props = @('-r', 'win-x64', '--self-contained', 'true') }
  @{ Name = 'modern-prefer32';   Project = 'Consumer.Modern'; Bits = 32; Rid = 'win-x86'; Props = @('-r', 'win-x86', '--self-contained', 'true', '-p:Prefer32Bit=true') }
  @{ Name = 'modern-x86';        Project = 'Consumer.Modern'; Bits = 32; Rid = 'win-x86'; Props = @('-r', 'win-x86', '--self-contained', 'true', '-p:PlatformTarget=x86') }
)

function Invoke-Consumer([string] $out) {
  $exe = Join-Path $out 'Consumer.exe'
  if (-not (Test-Path $exe)) { return @{ Code = -1; Text = "no Consumer.exe in $out" } }
  $text = & $exe $fixtureDir 2>&1 | Out-String
  return @{ Code = $LASTEXITCODE; Text = $text }
}

$built = @{}

foreach ($leg in $legs) {
  $out = Join-Path $work $leg.Name
  $obj = Join-Path $work "obj/$($leg.Name)/"
  $bin = Join-Path $work "bin/$($leg.Name)/"
  $project = Join-Path $here "$($leg.Project)/$($leg.Project).csproj"
  Write-Host "==> publish $($leg.Name)"
  # The extra switches go through a VARIABLE, not an inline @(...): PowerShell
  # flattens an array variable into separate native-command arguments, which is
  # what `-r win-x86 --self-contained true` needs, and an empty one contributes
  # nothing.
  $props = @($leg.Props)
  & dotnet publish $project -c Release -o $out `
      "-p:BaseIntermediateOutputPath=$obj" "-p:BaseOutputPath=$bin" `
      "-p:FolioDotnetVersion=$version" $props
  if ($LASTEXITCODE -ne 0) { Fail "$($leg.Name): the consumer project did not publish"; continue }
  $built[$leg.Name] = $out

  Write-Host "==> run $($leg.Name)"
  $run = Invoke-Consumer $out
  Write-Host $run.Text
  if ($run.Code -ne 0) { Fail "$($leg.Name): the consumer exited $($run.Code); it should have rendered"; continue }
  if ($run.Text -notmatch "BITNESS $($leg.Bits)\b") { Fail "$($leg.Name): expected a $($leg.Bits)-bit process" }
  if ($run.Text -notmatch 'FACES 11\b') { Fail "$($leg.Name): Fonts.Shipped() did not return the eleven shipped faces" }
  if ($run.Text -notmatch "SHA256 $expected\b") { Fail "$($leg.Name): the rendered PDF does not match $Fixture/expected.json" }
  if ($run.Text -notmatch "ENGINE $([regex]::Escape($engine))\b") { Fail "$($leg.Name): the engine version is not $engine, which is what go-parity.json records" }
}

# -------------------------------------------------- CAP-11, forced --------
# Both modes are forced against a COPY of a built leg, so one failure mode
# cannot contaminate another or the legs above.
function Copy-Leg([string] $from, [string] $name) {
  $to = Join-Path $work $name
  Remove-Item -Recurse -Force $to -ErrorAction SilentlyContinue
  Copy-Item -Recurse $from $to
  return $to
}

# BOTH DIRECTIONS, ON BOTH FAMILIES. A 64-bit process handed the x86 file and a
# 32-bit process handed the x64 file are different code paths through Windows
# and different sentences out of the loader; forcing only the first would leave
# the I/O matrix's 32-bit row unproved.
foreach ($leg in $legs | Where-Object { $_.Name -in @('net48-anycpu-64', 'net48-x86', 'modern-anycpu-64', 'modern-x86') }) {
  $name = $leg.Name
  if (-not $built.ContainsKey($name)) { Fail "$name did not build, so its CAP-11 modes could not be forced"; continue }
  $otherRid = if ($leg.Rid -eq 'win-x64') { 'win-x86' } else { 'win-x64' }

  # (1) MISSING RID ASSET. Every copy of the native this process could load is
  #     removed -- including the OTHER architecture's, so that a pass cannot
  #     come from a silent fallback.
  $missing = Copy-Leg $built[$name] "$name-missing"
  Get-ChildItem -Recurse -Path $missing -Filter 'folio8_native.dll' | Remove-Item -Force
  $run = Invoke-Consumer $missing
  Write-Host "==> $name, native deleted:`n$($run.Text)"
  if ($run.Code -ne 2) { Fail "$name/missing: expected the folio8 load exception (exit 2), got $($run.Code)" }
  foreach ($required in @('FOLIO8-LOAD-FAILURE', "RID $($leg.Rid)", 'FILENAME folio8_native.dll', "Process bitness: $($leg.Bits)-bit", 'Paths probed')) {
    if ($run.Text -notmatch [regex]::Escape($required)) { Fail "$name/missing: the message does not state '$required'" }
  }
  if ($run.Text -match [regex]::Escape($otherRid)) { Fail "$name/missing: the message names $otherRid; nothing may probe the other architecture" }

  # (2) WRONG BITNESS. The other architecture's file is put where this process
  #     will find its own. Windows answers ERROR_BAD_EXE_FORMAT and the message
  #     has to say so in words.
  $wrong = Copy-Leg $built[$name] "$name-wrong-bitness"
  $swapIn = Join-Path $repo "folio-dotnet/build/native/$otherRid/folio8_native.dll"
  if (-not (Test-Path $swapIn)) {
    # GUARDED, NOT FATAL. A missing build artifact is this leg's problem; it
    # must not abort the run and hide every leg after it.
    Fail "$name/wrong-bitness: $swapIn is missing, so the mode could not be forced"
    continue
  }
  $targets = @(Get-ChildItem -Recurse -Path $wrong -Filter 'folio8_native.dll' |
               Where-Object { $_.FullName -match [regex]::Escape($leg.Rid) -or $_.Directory.FullName -eq $wrong })
  if ($targets.Count -eq 0) { Fail "$name/wrong-bitness: no $($leg.Rid) native was staged, so the mode could not be forced"; continue }
  foreach ($t in $targets) { Copy-Item -Force $swapIn $t.FullName }
  $run = Invoke-Consumer $wrong
  Write-Host "==> $name, $($leg.Rid) asset replaced by the $otherRid file:`n$($run.Text)"
  if ($run.Code -ne 2) { Fail "$name/wrong-bitness: expected the folio8 load exception (exit 2), got $($run.Code)" }
  foreach ($required in @('FOLIO8-LOAD-FAILURE', 'BITNESS MISMATCH', "Process bitness: $($leg.Bits)-bit", $leg.Rid)) {
    if ($run.Text -notmatch [regex]::Escape($required)) { Fail "$name/wrong-bitness: the message does not state '$required'" }
  }
}

if ($failures.Count -gt 0) {
  Write-Host ''
  Write-Host "CONSUMER SUITE FAILED -- $($failures.Count) problem(s):"
  $failures | ForEach-Object { Write-Host "  - $_" }
  exit 1
}

Write-Host ''
Write-Host "CONSUMER SUITE PASSED -- $($legs.Count) process shapes rendered $Fixture to $expected, and both forced failure modes reported the folio8 load exception in both directions on both families."

# EXIT 0 EXPLICITLY. The forced-failure legs above run a consumer that exits
# NON-ZERO ON PURPOSE, and $LASTEXITCODE still holds that value here. The
# shell that invokes this script exits with $LASTEXITCODE when the script
# does not exit itself, so without this line a fully passing suite fails the
# job -- which is exactly what happened on this file's first CI run.
exit 0
