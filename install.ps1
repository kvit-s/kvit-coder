# Install kvit-coder from a published release (native Windows).
#
#   irm https://raw.githubusercontent.com/kvit-s/kvit-coder/main/install.ps1 | iex
#
# It works out which build fits this machine (amd64 or arm64), downloads the
# Windows zip from the GitHub release, checks it against the release's
# checksums file, copies kvit-coder.exe and kvit-coder-ui.exe to a bin
# directory on the user's PATH, and seeds %USERPROFILE%\.kvit-coder\config.yaml
# from the bundled example when there is no configuration yet.
#
# WSL stays supported via install.sh; this script is for native Windows, which
# needs Git for Windows (for sh.exe, grep, and git itself) — see
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
  $zipName = "kvit-coder_${version}_${platform}.zip"
  # goreleaser names archives <project>_<version>_<os>_<arch>.zip; fall back to
  # the version without v when the tag form 404s.
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

  Say "installed kvit-coder.exe and kvit-coder-ui.exe to $binDir"
  $path = [Environment]::GetEnvironmentVariable('PATH', 'User')
  if ($path -split ';' -notcontains $binDir) {
    Say "NOTE: $binDir is not on your user PATH. Add it (System Properties > Environment Variables) or move the .exes somewhere it is."
  }
  Say "Prerequisite: Git for Windows (https://git-scm.com/download/win) for sh.exe, grep, and git."
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
