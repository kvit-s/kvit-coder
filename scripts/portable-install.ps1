# Portable install for kvit-coder (runs from the extracted folder).
#
#   powershell -ExecutionPolicy Bypass -File install.ps1
#
# This file ships INSIDE the portable zip as install.ps1: the folder it runs
# from stays the install location, so extract the zip where you want to keep
# it (e.g. C:\apps\kvit-coder) and run this there. It adds that folder to the
# user's PATH so kvit-coder.exe and kvit-coder-ui.exe work in any terminal,
# and seeds %USERPROFILE%\.kvit-coder\config.yaml from the bundled
# config.example.yaml when there is no configuration yet.
#
# This is not the downloader: the install.ps1 at the repository root downloads
# a release from GitHub, while this one installs the files beside it.
#
#   install.ps1             add this folder to the user PATH, seed config
#   install.ps1 -Uninstall  remove this folder from the user PATH again
[CmdletBinding()]
param([switch]$Uninstall)

$ErrorActionPreference = 'Stop'

function Say([string]$msg) { Write-Host $msg }
function Die([string]$msg) { Write-Error "install: $msg"; exit 1 }

# The folder this script runs from is the install location. $PSScriptRoot is
# empty when the script arrives via irm|iex, which never happens for the
# bundled copy — but say so rather than installing the wrong directory.
$InstallDir = $PSScriptRoot
if (-not $InstallDir) { $InstallDir = Split-Path -Parent $MyInvocation.MyCommand.Path }
if (-not $InstallDir) { Die "cannot tell which folder this script runs from; run the install.ps1 inside the extracted portable zip" }

if ($Uninstall) {
  $path = [Environment]::GetEnvironmentVariable('PATH', 'User')
  if ($null -eq $path) { $path = '' }
  $kept = @($path -split ';' | Where-Object { $_ -ne '' -and $_ -ne $InstallDir })
  [Environment]::SetEnvironmentVariable('PATH', ($kept -join ';'), 'User')
  $env:PATH = ($env:PATH -split ';' | Where-Object { $_ -ne '' -and $_ -ne $InstallDir }) -join ';'
  Say "removed $InstallDir from the user PATH (config in %USERPROFILE%\.kvit-coder left alone)"
  return
}

foreach ($exe in @('kvit-coder.exe', 'kvit-coder-ui.exe')) {
  if (-not (Test-Path (Join-Path $InstallDir $exe))) {
    Die "$exe not found in $InstallDir; run this from the extracted portable zip"
  }
}

# The agent's shell is Git for Windows' sh.exe, never cmd.exe. Warn here; the
# Shell tool itself fails fast with the same message when none is found.
#
# A default Git install puts only Git\cmd (git.exe) on PATH, not Git\usr\bin
# (sh.exe) — so "git --version works but sh.exe is not on PATH" is the common
# case, not a broken install. Resolve sh.exe the same way the agent does:
# sh.exe on PATH, then derived from git.exe's location, then the registry's
# install path, then the standard and per-user install locations.
function Find-GitSh {
  $found = Get-Command sh.exe -ErrorAction SilentlyContinue
  if ($found) { return $found.Source }
  $sh = Get-Command sh -ErrorAction SilentlyContinue
  if ($sh) { return $sh.Source }

  $candidates = @()
  $git = Get-Command git.exe -ErrorAction SilentlyContinue
  if ($git) {
    $dir = Split-Path -Parent $git.Source
    for ($i = 0; $i -lt 3 -and $dir; $i++) {
      $candidates += (Join-Path $dir 'usr\bin\sh.exe')
      $candidates += (Join-Path $dir 'bin\sh.exe')
      $candidates += (Join-Path $dir 'sh.exe')
      $dir = Split-Path -Parent $dir
    }
  }

  foreach ($key in @('HKCU:\SOFTWARE\GitForWindows', 'HKLM:\SOFTWARE\GitForWindows')) {
    try {
      $root = (Get-ItemProperty -Path $key -Name InstallPath -ErrorAction Stop).InstallPath
      if ($root) {
        $candidates += (Join-Path $root 'usr\bin\sh.exe')
        $candidates += (Join-Path $root 'bin\sh.exe')
      }
    } catch { }
  }

  $bases = @($env:ProgramFiles, ${env:ProgramFiles(x86)}, $env:ProgramW6432)
  if ($env:LocalAppData) { $bases += (Join-Path $env:LocalAppData 'Programs') }
  elseif ($env:USERPROFILE) { $bases += (Join-Path $env:USERPROFILE 'AppData\Local\Programs') }
  $bases += 'C:\Program Files', 'C:\Program Files (x86)'
  foreach ($base in $bases) {
    if (-not $base) { continue }
    $candidates += (Join-Path $base 'Git\usr\bin\sh.exe')
    $candidates += (Join-Path $base 'Git\bin\sh.exe')
  }

  foreach ($c in $candidates) {
    if ($c -and (Test-Path $c -PathType Leaf)) { return $c }
  }
  return $null
}

$gitSh = Find-GitSh
if ($gitSh) {
  Say "found Git shell: $gitSh"
} else {
  Say "NOTE: Git for Windows not found (https://git-scm.com/download/win). The agent needs its sh.exe: install Git and keep the default 'Git from the command line' PATH option."
}

$path = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($null -eq $path) { $path = '' }
if (($path -split ';') -notcontains $InstallDir) {
  [Environment]::SetEnvironmentVariable('PATH', (($path.TrimEnd(';') + ';' + $InstallDir).TrimStart(';')), 'User')
  Say "added $InstallDir to the user PATH"
} else {
  Say "$InstallDir is already on the user PATH"
}
if (($env:PATH -split ';') -notcontains $InstallDir) { $env:PATH = "$env:PATH;$InstallDir" }

# A first install has nothing to run against, so leave a configuration to
# edit rather than an error message about not finding one. Never overwrite.
$configDir = Join-Path $env:USERPROFILE '.kvit-coder'
$configPath = Join-Path $configDir 'config.yaml'
if (-not (Test-Path $configPath)) {
  $example = Join-Path $InstallDir 'config.example.yaml'
  if (Test-Path $example) {
    New-Item -ItemType Directory -Force -Path $configDir | Out-Null
    Copy-Item $example $configPath
    Say "wrote $configPath (edit it to configure the model)"
  }
}

try {
  $v = & (Join-Path $InstallDir 'kvit-coder-ui.exe') --version 2>$null
  if ($v) { Say $v }
} catch { }

Say "installed: kvit-coder.exe and kvit-coder-ui.exe run from $InstallDir"
Say "open a NEW terminal (PATH reloads on launch), then run kvit-coder-ui"
