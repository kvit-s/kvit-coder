# Install kvit-coder from a published release (native Windows).
#
#   irm https://raw.githubusercontent.com/kvit-s/kvit-coder/main/install.ps1 | iex
#
# It works out which build fits this machine (amd64 or arm64), downloads the
# Windows zip from the GitHub release, checks it against the release's
# checksums file, copies kvit-coder.exe and kvit-coder-ui.exe to a bin
# directory, makes kc.exe and kcu.exe beside them, adds that directory to the
# user's PATH, and seeds %USERPROFILE%\.kvit-coder\config.yaml from the
# bundled example when there is no configuration yet. The Windows installer
# (kvit-coder_<version>_windows_<arch>_setup.exe on the release page) does the
# same with an uninstaller.
#
# WSL stays supported via install.sh; this script is for native Windows, which
# needs Git for Windows (for sh.exe, grep, and git itself) -- see
# docs/configuration.md.
#
# Environment:
#   $env:KVIT_VERSION  a release tag instead of the latest (e.g. v0.1.0)
#   $env:KVIT_BIN_DIR  where to put the binaries (default %USERPROFILE%\bin)
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$Repo = 'kvit-s/kvit-coder'

function Say([string]$msg) { Write-Host $msg }
function Die([string]$msg) { Write-Error "install: $msg"; exit 1 }

# kc.exe and kcu.exe are the short names scripts/install.sh gives its symlinks
# on unix. Windows needs privilege for a symlink, so here they are hard links
# (copies where the disk cannot hold one), and kvit-coder-ui's :update replaces
# them along with the programs. A running kc.exe cannot be deleted but can be
# renamed, so an old one is moved aside to .old, which the next start of
# kvit-coder-ui removes.
function Set-ShortName([string]$dir, [string]$name, [string]$target) {
  $link = Join-Path $dir $name
  $existing = Join-Path $dir $target
  if (Test-Path $link) {
    try { Remove-Item -Force $link -ErrorAction Stop }
    catch {
      Remove-Item -Force "$link.old" -ErrorAction SilentlyContinue
      Rename-Item $link "$name.old"
    }
  }
  try { New-Item -ItemType HardLink -Path $link -Target $existing -ErrorAction Stop | Out-Null }
  catch { Copy-Item $existing $link }
}

function LatestVersion {
  try {
    $resp = Invoke-WebRequest -UseBasicParsing -Method Head `
      -Uri "https://github.com/$Repo/releases/latest" -MaximumRedirection 0 -ErrorAction SilentlyContinue
    # Follow the redirect manually: its Location names the tag.
    $url = $resp.Headers.Location
    if (-not $url) {
      # Newer PowerShell follows automatically; ask for the effective URL instead.
      $r = Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/$Repo/releases/latest"
      $url = $r.BaseResponse.ResponseUri.ToString()
    }
    $tag = $url.TrimEnd('/').Split('/')[-1]
    if ($tag -notlike 'v*') { Die "no published release found for $Repo" }
    return $tag
  } catch {
    Die "cannot reach GitHub to ask for the latest release: $_"
  }
}

# --- platform -------------------------------------------------------------
$arch = $env:PROCESSOR_ARCHITECTURE
switch -Wildcard ($arch) {
  'AMD64' { $goarch = 'amd64' }
  'ARM64' { $goarch = 'arm64' }
  default { Die "no build for architecture $arch (amd64 and arm64 only)" }
}
$platform = "windows_$goarch"

# --- which release ---------------------------------------------------------
$version = $env:KVIT_VERSION
if (-not $version) { $version = LatestVersion }
$number = $version.TrimStart('v')
Say "installing kvit-coder $version for $platform"

$binDir = $env:KVIT_BIN_DIR
if (-not $binDir) { $binDir = Join-Path $env:USERPROFILE 'bin' }
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$tmp = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ("kvit-" + [IO.Path]::GetRandomFileName()))
try {
  $zipName = "kvit-coder_${number}_${platform}.zip"
  # Archive names follow goreleaser's number without the v (kvit-coder_0.2.0_windows_amd64.zip).
  $base = "https://github.com/$Repo/releases/download/$version"
  $zipUrl = "$base/$zipName"
  $zipPath = Join-Path $tmp $zipName
  try {
    Invoke-WebRequest -UseBasicParsing -Uri $zipUrl -OutFile $zipPath
  } catch {
    Die "cannot download $zipUrl : $_"
  }

  # Verify against checksums.txt.
  $sumsUrl = "$base/checksums.txt"
  $sumsPath = Join-Path $tmp 'checksums.txt'
  Invoke-WebRequest -UseBasicParsing -Uri $sumsUrl -OutFile $sumsPath
  $expected = (Select-String -Path $sumsPath -Pattern ([regex]::Escape($zipName)) | ForEach-Object { $_.Line.Split()[0] })
  if (-not $expected) { Die "no checksum for $zipName in checksums.txt" }
  $actual = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()
  if ($actual -ne $expected.ToLower()) { Die "checksum mismatch for $zipName" }

  Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
  foreach ($exe in @('kvit-coder.exe', 'kvit-coder-ui.exe')) {
    $src = Get-ChildItem -Path $tmp -Recurse -Filter $exe | Select-Object -First 1
    if (-not $src) { Die "$exe not found in $zipName" }
    Copy-Item $src.FullName (Join-Path $binDir $exe) -Force
  }
  Set-ShortName $binDir 'kc.exe' 'kvit-coder.exe'
  Set-ShortName $binDir 'kcu.exe' 'kvit-coder-ui.exe'

  # Seed config from the bundled example.
  $configDir = Join-Path $env:USERPROFILE '.kvit-coder'
  $configPath = Join-Path $configDir 'config.yaml'
  if (-not (Test-Path $configPath)) {
    $example = Get-ChildItem -Path $tmp -Recurse -Filter 'config.example.yaml' | Select-Object -First 1
    if ($example) {
      New-Item -ItemType Directory -Force -Path $configDir | Out-Null
      Copy-Item $example.FullName $configPath
      Say "wrote $configPath (edit it to configure the model)"
    }
  }

  Say "installed kvit-coder.exe and kvit-coder-ui.exe (kc and kcu) to $binDir"
  $path = [Environment]::GetEnvironmentVariable('PATH', 'User')
  if ($null -eq $path) { $path = '' }
  if (($path -split ';') -notcontains $binDir) {
    [Environment]::SetEnvironmentVariable('PATH', (($path.TrimEnd(';') + ';' + $binDir).TrimStart(';')), 'User')
    Say "added $binDir to the user PATH; open a new terminal to use it"
  }
  Say "Prerequisite: Git for Windows (https://git-scm.com/download/win) for sh.exe, grep, and git."
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
