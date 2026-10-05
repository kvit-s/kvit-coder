# Tries a kvit-coder installer on this machine. It installs it silently into a
# scratch folder, checks the files, kc.exe and kcu.exe, the user PATH entry and
# the configuration, starts kc and kcu by name with the PATH a new terminal
# would get, uninstalls, and checks that everything the installer added is
# gone and the user PATH is back to what it was.
#
#   powershell -ExecutionPolicy Bypass -File scripts/test-installer.ps1 -Setup <setup.exe>
#
# The windows job in .github/workflows/test.yml runs it on a fresh runner. On
# a machine where kvit-coder is installed with the installer it refuses to
# run, because the test would upgrade that installation and then remove it.
# The scratch folder (-Work, default %TEMP%\kvit-coder-installer-test) is
# emptied first and kept afterwards with install.log and uninstall.log.
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)] [string]$Setup,
  [string]$Work = (Join-Path ([IO.Path]::GetTempPath()) 'kvit-coder-installer-test')
)

$ErrorActionPreference = 'Stop'

# The AppId in scripts/windows-installer.iss.
$AppId = '{2DEEC128-6BD2-4F6D-8C8E-3A4EFED181C0}'
$UninstallKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\${AppId}_is1"

$script:failures = 0
function Check([bool]$ok, [string]$what) {
  if ($ok) { Write-Host "  ok    $what" } else { Write-Host "  FAIL  $what"; $script:failures++ }
}

# The user PATH as stored, with %VARIABLES% unexpanded, which is what the
# installer reads and writes.
function Get-UserPath {
  $key = Get-Item 'HKCU:\Environment'
  return $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
}

function Test-PathEntry([string]$path, [string]$dir) {
  foreach ($entry in $path -split ';') {
    if ($entry.Trim().TrimEnd('\') -ieq $dir.TrimEnd('\')) { return $true }
  }
  return $false
}

function Get-Sha256([string]$file) { (Get-FileHash -Path $file -Algorithm SHA256).Hash }

# Runs an installer program and waits for it. Inno's uninstaller hands over to
# a copy of itself in %TEMP% and exits, so for that one the caller also waits
# for the install folder to empty.
function Invoke-Silent([string]$exe, [string[]]$arguments) {
  $p = Start-Process -FilePath $exe -ArgumentList $arguments -Wait -PassThru
  return $p.ExitCode
}

if (Test-Path $UninstallKey) {
  $where = (Get-ItemProperty $UninstallKey).InstallLocation
  throw "kvit-coder is installed on this machine ($where); this test would replace and then remove it"
}
$Setup = (Resolve-Path $Setup).Path

if (Test-Path $Work) { Remove-Item -Recurse -Force $Work }
New-Item -ItemType Directory -Path $Work | Out-Null
$app = Join-Path $Work 'app'

$pathBefore = Get-UserPath
$configDir = Join-Path $env:USERPROFILE '.kvit-coder'
$config = Join-Path $configDir 'config.yaml'
$configDirExisted = Test-Path $configDir
$configBefore = $null
if (Test-Path $config) { $configBefore = Get-Sha256 $config }

Write-Host "== installing $Setup into $app"
$code = Invoke-Silent $Setup @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/SP-',
  "/DIR=`"$app`"", "/LOG=`"$Work\install.log`"")
Check ($code -eq 0) "installer exit code $code"

foreach ($name in @('kvit-coder.exe', 'kvit-coder-ui.exe', 'kc.exe', 'kcu.exe',
                    'config.example.yaml', 'README.md', 'LICENSE', 'unins000.exe')) {
  Check (Test-Path (Join-Path $app $name)) "installed $name"
}
foreach ($pair in @(@('kc.exe', 'kvit-coder.exe'), @('kcu.exe', 'kvit-coder-ui.exe'))) {
  $short = Join-Path $app $pair[0]
  $long = Join-Path $app $pair[1]
  if ((Test-Path $short) -and (Test-Path $long)) {
    Check ((Get-Sha256 $short) -eq (Get-Sha256 $long)) "$($pair[0]) has the same bytes as $($pair[1])"
    Check ((Get-Item $short).LinkType -eq 'HardLink') "$($pair[0]) is a hard link"
  }
}

Check (Test-PathEntry (Get-UserPath) $app) "the user PATH has $app"
Check (Test-Path $UninstallKey) "the uninstall entry is registered"

if ($configBefore) {
  Check ((Get-Sha256 $config) -eq $configBefore) "the existing $config is unchanged"
} else {
  Check ((Test-Path $config) -and ((Get-Sha256 $config) -eq (Get-Sha256 (Join-Path $app 'config.example.yaml')))) `
    "$config was written from config.example.yaml"
}

# A new terminal builds its PATH from the machine and user values in the
# registry; this process started before the install and has neither change.
Write-Host "== starting kc and kcu by name"
$savedPath = $env:PATH
try {
  $env:PATH = [Environment]::GetEnvironmentVariable('PATH', 'Machine') + ';' +
              [Environment]::GetEnvironmentVariable('PATH', 'User')
  foreach ($pair in @(@('kc', 'kc.exe', 'kvit-coder '), @('kcu', 'kcu.exe', 'kvit-coder-ui '))) {
    $cmd = Get-Command $pair[0] -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    Check ($cmd -and $cmd.Source -ieq (Join-Path $app $pair[1])) "$($pair[0]) resolves to $(Join-Path $app $pair[1])"
    if ($cmd) {
      $out = (& $pair[0] --version | Out-String).Trim()
      Write-Host "        $($pair[0]) --version: $out"
      Check (($LASTEXITCODE -eq 0) -and $out.StartsWith($pair[2])) "$($pair[0]) --version runs"
    }
  }
} finally {
  $env:PATH = $savedPath
}

Write-Host "== uninstalling"
$code = Invoke-Silent (Join-Path $app 'unins000.exe') @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART',
  "/LOG=`"$Work\uninstall.log`"")
Check ($code -eq 0) "uninstaller exit code $code"
$deadline = (Get-Date).AddSeconds(60)
while ((Test-Path (Join-Path $app 'unins000.exe')) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 500 }

$left = @()
if (Test-Path $app) { $left = @(Get-ChildItem -Force -Recurse $app | ForEach-Object { $_.FullName }) }
Check ($left.Count -eq 0) "the install folder is empty or gone$(if ($left.Count) { ': ' + ($left -join ', ') })"
Check (-not (Test-Path $UninstallKey)) "the uninstall entry is gone"
$pathAfter = Get-UserPath
Check ($pathAfter -ceq $pathBefore) "the user PATH is as it was"
if ($pathAfter -cne $pathBefore) {
  Write-Host "        before: $pathBefore"
  Write-Host "        after:  $pathAfter"
}
if ($configBefore) {
  Check ((Get-Sha256 $config) -eq $configBefore) "the existing $config is still unchanged"
} else {
  Check (Test-Path $config) "uninstalling leaves $config"
  # Put the user's folder back as the test found it.
  Remove-Item -Force $config -ErrorAction SilentlyContinue
  if (-not $configDirExisted) { Remove-Item -Recurse -Force $configDir -ErrorAction SilentlyContinue }
}

if ($script:failures -gt 0) {
  Write-Host "$($script:failures) check(s) failed; logs in $Work"
  exit 1
}
Write-Host "all checks passed; logs in $Work"
