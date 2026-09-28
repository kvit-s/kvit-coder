# copilot-doctor.ps1 — find out why `kvit-coder copilot models` says the
# GitHub Copilot credentials were not found.
#
#   powershell -ExecutionPolicy Bypass -File copilot-doctor.ps1
#
# or straight from GitHub (needs nothing else on the machine):
#
#   irm https://raw.githubusercontent.com/kvit-s/kvit-coder/main/scripts/copilot-doctor.ps1 | iex
#
# What it does: it walks the exact same credential sources kvit-coder walks,
# in the same order — COPILOT_GITHUB_TOKEN, GH_TOKEN, GITHUB_TOKEN,
# %USERPROFILE%\.copilot\config.json, the OS keychain entry, `gh auth token`
# — plus a few places kvit-coder does NOT look (the `copilot` binary itself,
# Windows Credential Manager, %APPDATA% leftovers) so the report can tell a
# "nothing signed in anywhere" apart from a "signed in where kvit-coder does
# not look".
#
# It never prints a whole secret: tokens appear as their first 4 characters
# plus their length (enough to tell two tokens apart, not enough to use one).
# It exits 0 when at least one usable credential is visible to kvit-coder,
# 1 when none is.
[CmdletBinding()]
param()

$ErrorActionPreference = 'Continue'
$script:usableFound = $false

function Say([string]$msg) { Write-Host $msg }

function Head([string]$title) {
  Write-Host ""
  Write-Host ("--- " + $title + " ---")
}

function Redact([string]$s) {
  if ([string]::IsNullOrEmpty($s)) { return "<empty>" }
  if ($s.Length -le 8) { return "*** (len " + $s.Length + ")" }
  return $s.Substring(0, 4) + "... (len " + $s.Length + ")"
}

# Mirrors internal/copilot usableToken/rejectedToken: gho_, ghu_ and
# github_pat_ are usable, classic ghp_ personal access tokens are refused.
function Token-Shape([string]$s) {
  if ([string]::IsNullOrWhiteSpace($s)) { return "empty" }
  $t = $s.Trim()
  if ($t.StartsWith("ghp_")) { return "classic PAT (ghp_ — refused by kvit-coder)" }
  if ($t.StartsWith("gho_")) { return "usable shape (gho_)" }
  if ($t.StartsWith("ghu_")) { return "usable shape (ghu_)" }
  if ($t.StartsWith("github_pat_")) { return "usable shape (github_pat_)" }
  return "unknown shape (not gho_/ghu_/github_pat_)"
}

function Is-Usable([string]$s) {
  if ([string]::IsNullOrWhiteSpace($s)) { return $false }
  $t = $s.Trim()
  return $t.StartsWith("gho_") -or $t.StartsWith("ghu_") -or $t.StartsWith("github_pat_")
}

Say "kvit-coder Copilot credential check"
Say ("machine: " + $env:COMPUTERNAME + "  user: " + $env:USERNAME)

# --- 1. environment ------------------------------------------------------
Head "1. environment variables (what kvit-coder checks first)"
foreach ($name in @("COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN")) {
  $proc = [Environment]::GetEnvironmentVariable($name, "Process")
  $user = [Environment]::GetEnvironmentVariable($name, "User")
  $mach = [Environment]::GetEnvironmentVariable($name, "Machine")
  if ([string]::IsNullOrWhiteSpace($proc)) {
    $hint = "not set in this terminal"
    if (-not [string]::IsNullOrWhiteSpace($user)) { $hint += " (but IS set for the User scope — open a NEW terminal so it arrives)" }
    elseif (-not [string]::IsNullOrWhiteSpace($mach)) { $hint += " (but IS set for the Machine scope — open a NEW terminal so it arrives)" }
    Say ("  $" + $name + " : " + $hint)
  } else {
    Say ("  $" + $name + " : set, " + (Redact $proc) + ", " + (Token-Shape $proc))
    if (Is-Usable $proc) { $script:usableFound = $true }
  }
}

# --- 2. the Copilot CLI config file --------------------------------------
Head "2. %USERPROFILE%\.copilot\config.json"
$homeDir = $env:USERPROFILE
if ([string]::IsNullOrWhiteSpace($homeDir)) { $homeDir = [Environment]::GetFolderPath("UserProfile") }
$cfgPath = Join-Path $homeDir ".copilot\config.json"
if (-not (Test-Path $cfgPath -PathType Leaf)) {
  Say ("  missing: " + $cfgPath)
  Say "  (this file is where 'copilot login' keeps its token on many machines)"
} else {
  $len = (Get-Item $cfgPath).Length
  Say ("  present: " + $cfgPath + " (" + $len + " bytes)")
  $tokenKeys = @("github_token", "oauth_token", "access_token", "token")
  $hostKeys = @("host", "hostname", "github_host")
  try {
    $json = Get-Content -Path $cfgPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    Say "  valid JSON: yes"
    $topKeys = @($json.PSObject.Properties | ForEach-Object { $_.Name })
    Say ("  top-level keys: " + ($topKeys -join ", "))
    $found = 0
    $checkMap = {
      param($map, $label)
      foreach ($k in $tokenKeys) {
        $v = $null
        try { $v = $map.$k } catch { $v = $null }
        if ($v -is [string] -and -not [string]::IsNullOrWhiteSpace($v)) {
          $h = ""
          foreach ($hk in $hostKeys) {
            try {
              $hv = $map.$hk
              if ($hv -is [string] -and -not [string]::IsNullOrWhiteSpace($hv)) { $h = $hv; break }
            } catch { }
          }
          Say ("  token under " + $label + "." + $k + ": " + (Redact $v) + ", " + (Token-Shape $v) + ", host=[" + $h + "]")
          $script:found = $script:found + 1
          if (Is-Usable $v) { $script:usableFound = $true }
        }
      }
    }
    $script:found = 0
    & $checkMap $json "top level"
    try {
      if ($null -ne $json.auth) { & $checkMap $json.auth "auth" }
    } catch { }
    try {
      $last = $json.last_logged_in_user
      if ($last -is [string] -and $last -ne "") { Say ("  last_logged_in_user: " + $last) }
    } catch { }
    try {
      if ($null -ne $json.users) {
        foreach ($u in @($json.users.PSObject.Properties)) {
          & $checkMap $u.Value ("users." + $u.Name)
        }
      }
    } catch { }
    if ($script:found -eq 0) {
      Say "  no github_token/oauth_token/access_token/token string found anywhere kvit-coder looks"
      Say "  (if the token lives under a differently-named key, that is the bug — paste the KEY NAMES, never values, in the issue)"
    }
    Remove-Variable -Name found -Scope Script -ErrorAction SilentlyContinue
  } catch {
    Say ("  valid JSON: NO — " + $_.Exception.Message)
  }
}

# --- 3. Windows Credential Manager ----------------------------------------
Head "3. Windows Credential Manager (NOT read by kvit-coder on Windows)"
Say "  kvit-coder queries the OS keychain on Linux and macOS only. If your"
Say "  only sign-in lives here, THAT is why 'copilot models' finds nothing."
$credTargets = @()
try {
  $lines = cmdkey /list 2>&1 | Out-String
  $credTargets = @($lines -split "`r?`n" | Where-Object { $_ -match "copilot|github" })
  if ($credTargets.Count -eq 0) {
    Say "  no credential targets mentioning copilot/github"
  } else {
    foreach ($t in $credTargets) { Say ("  " + $t.Trim()) }
    Say "  (target NAMES only — Windows never shows the secrets, and neither does this script)"
  }
} catch {
  Say ("  could not list credentials: " + $_.Exception.Message)
}

# --- 4. gh CLI -------------------------------------------------------------
Head "4. gh CLI (`gh auth token` is kvit-coder's last resort)"
$gh = Get-Command gh -ErrorAction SilentlyContinue
if (-not $gh) {
  Say "  gh: not on PATH"
} else {
  Say ("  gh: " + $gh.Source)
  try {
    $st = & gh auth status 2>&1 | Out-String
    Say "  gh auth status:"
    foreach ($line in ($st -split "`r?`n")) {
      if ($line.Trim() -ne "") { Say ("    " + $line.Trim()) }
    }
  } catch {
    Say ("  gh auth status failed: " + $_.Exception.Message)
  }
  try {
    $tok = & gh auth token 2>&1 | Out-String
    $tok = $tok.Trim()
    if ($tok -match "^(gho_|ghu_|github_pat_)") {
      Say ("  gh auth token: present, " + (Redact $tok) + ", " + (Token-Shape $tok))
      if (Is-Usable $tok) { $script:usableFound = $true }
    } elseif ($tok -ne "") {
      Say ("  gh auth token output is not a usable token shape: " + (Redact $tok))
      Say "  (probably an error message — is 'gh auth login' done?)"
    } else {
      Say "  gh auth token: empty (not logged in?)"
    }
  } catch {
    Say ("  gh auth token failed: " + $_.Exception.Message)
  }
}

# --- 5. the copilot binary itself ------------------------------------------
Head "5. copilot CLI binary (kvit-coder never calls it — sign-in reuse only)"
$cop = Get-Command copilot -ErrorAction SilentlyContinue
if (-not $cop) {
  Say "  copilot: not on PATH"
} else {
  Say ("  copilot: " + $cop.Source)
  try {
    $ver = & copilot --version 2>&1 | Out-String
    $first = ($ver -split "`r?`n" | Where-Object { $_.Trim() -ne "" } | Select-Object -First 1)
    if ($first) { Say ("  version: " + $first.Trim()) }
  } catch {
    Say ("  --version failed: " + $_.Exception.Message)
  }
  Say "  note: being on PATH proves nothing about sign-in — see sections 2 and 3."
}

# --- 6. other Copilot leftovers --------------------------------------------
Head "6. other Copilot data dirs (names only)"
foreach ($base in @($env:APPDATA, $env:LOCALAPPDATA)) {
  if ([string]::IsNullOrWhiteSpace($base) -or -not (Test-Path $base)) { continue }
  try {
    $dirs = Get-ChildItem -Path $base -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -match "copilot" }
    foreach ($d in $dirs) { Say ("  " + $d.FullName) }
  } catch { }
}
Say "  (end of list)"

# --- 7. reproduce the exact failure -----------------------------------------
Head "7. reproduce: kvit-coder copilot models"
$kvit = $null
if ($PSScriptRoot) {
  foreach ($n in @("kvit-coder.exe", "kvit-coder")) {
    $p = Join-Path $PSScriptRoot $n
    if (Test-Path $p -PathType Leaf) { $kvit = $p; break }
  }
}
if (-not $kvit) {
  $k = Get-Command kvit-coder -ErrorAction SilentlyContinue
  if ($k) { $kvit = $k.Source }
}
if (-not $kvit) {
  Say "  kvit-coder not found beside this script or on PATH — skipping repro."
} else {
  Say ("  running: " + $kvit + " copilot models")
  try {
    $out = & $kvit copilot models 2>&1 | Out-String
    $code = $LASTEXITCODE
    Say ("  exit code: " + $code)
    $lines = @($out -split "`r?`n")
    $tail = $lines | Select-Object -Last 15
    foreach ($line in $tail) { Say ("  | " + $line) }
  } catch {
    Say ("  failed to run: " + $_.Exception.Message)
  }
}

# --- verdict -----------------------------------------------------------------
Head "verdict"
if ($script:usableFound) {
  Say "A USABLE credential IS visible to kvit-coder (sections 1, 2 or 4)."
  Say "If 'copilot models' still fails, the problem is past credential lookup:"
  Say "paste the exact error from section 7 (network, host, or token exchange)."
  exit 0
}
if ($credTargets.Count -gt 0) {
  Say "LIKELY CAUSE: you are signed in, but only inside Windows Credential"
  Say "Manager — which kvit-coder does not read on Windows yet."
  Say ""
  Say "Fastest workaround (current terminal only):"
  Say '  $env:COPILOT_GITHUB_TOKEN = (gh auth token)'
  Say "then re-run 'kvit-coder copilot models' in the SAME terminal."
  Say "Needs 'gh auth login' done first (see section 4)."
  Say ""
  Say "Persistent workaround: System Properties > Environment Variables, add"
  Say "COPILOT_GITHUB_TOKEN for your user (needs a gho_/ghu_/github_pat_ token),"
  Say "then open a NEW terminal."
  exit 1
}
Say "No usable Copilot credential found anywhere the script looked."
Say "Sign in once, any one of these is enough:"
Say "  copilot login            (then re-run this script to confirm it landed)"
Say "  gh auth login            (kvit-coder falls back to 'gh auth token')"
Say '  $env:COPILOT_GITHUB_TOKEN = "<gho_/ghu_/github_pat_ token>"'
exit 1
