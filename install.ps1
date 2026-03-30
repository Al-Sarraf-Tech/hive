param(
  [string]$Version = "",
  [switch]$Service,
  [string]$Token = "",
  [string]$InstallDir = "$env:ProgramFiles\Hive",
  [string]$DataDir = "$env:ProgramData\Hive\data"
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$Repo = "Al-Sarraf-Tech/hive"

function Info($m) { Write-Host "[hive] $m" -ForegroundColor Cyan }
function Ok($m) { Write-Host "[hive] $m" -ForegroundColor Green }
function Warn($m) { Write-Warning $m }
function Die($m) { throw $m }

function Require-Admin {
  $id = [Security.Principal.WindowsIdentity]::GetCurrent()
  $p = New-Object Security.Principal.WindowsPrincipal($id)
  if (-not $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Die "Run this installer from an elevated PowerShell session."
  }
}

function Get-Version {
  if ($Version) { return $Version.TrimStart("v") }
  $tag = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
  if (-not $tag) { Die "Could not determine latest version. Use -Version." }
  return $tag.TrimStart("v")
}

function Fetch($Url, $Dest, [switch]$Optional) {
  try { Invoke-WebRequest $Url -OutFile $Dest }
  catch {
    if ($Optional) {
      Warn "Skipped missing asset: $([IO.Path]::GetFileName($Url))"
      return $false
    }
    throw
  }
  return $true
}

function Add-ToPath($Dir) {
  $parts = @(([Environment]::GetEnvironmentVariable("Path", "Machine") -split ";") | Where-Object { $_ })
  if ($parts -notcontains $Dir) {
    [Environment]::SetEnvironmentVariable("Path", (($parts + $Dir) -join ";"), "Machine")
  }
  if (($env:Path -split ";") -notcontains $Dir) { $env:Path = "$Dir;$env:Path" }
}

function Setup-Service($ExePath) {
  New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
  $bin = "`"$ExePath`" --data-dir `"$DataDir`" --log-level info --http-port 7949"
  if ($Token) { $bin += " --http-token `"$Token`"" }
  if (Get-Service hived -ErrorAction SilentlyContinue) {
    & sc.exe config hived "binPath= $bin" "start= auto" | Out-Null
  } else {
    & sc.exe create hived "binPath= $bin" "start= auto" "DisplayName= Hive Daemon" | Out-Null
  }
  if ($LASTEXITCODE -ne 0) { Die "Failed to configure the hived Windows service." }
  Ok "Windows service configured: hived"
}

Require-Admin
if (-not [Environment]::Is64BitOperatingSystem) { Die "Unsupported architecture. Hive supports Windows amd64 only." }

$ResolvedVersion = Get-Version
$TmpDir = Join-Path $env:TEMP ("hive-install-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
  $BaseUrl = "https://github.com/$Repo/releases/download/v$ResolvedVersion"
  $Downloaded = @("hived.exe")

  Info "Installing Hive v$ResolvedVersion for windows-amd64"
  Fetch "$BaseUrl/hived-windows-amd64.exe" (Join-Path $TmpDir "hived.exe") | Out-Null
  if (Fetch "$BaseUrl/hive-windows-amd64.exe" (Join-Path $TmpDir "hive.exe") -Optional) { $Downloaded += "hive.exe" }
  if (Fetch "$BaseUrl/hivetop-windows-amd64.exe" (Join-Path $TmpDir "hivetop.exe") -Optional) { $Downloaded += "hivetop.exe" }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  foreach ($Name in $Downloaded) {
    Copy-Item (Join-Path $TmpDir $Name) (Join-Path $InstallDir $Name) -Force
  }

  Add-ToPath $InstallDir
  if ($Service) { Setup-Service (Join-Path $InstallDir "hived.exe") }

  Ok "Installed to $InstallDir"
  Write-Host "Installed: $($Downloaded -join ', ')"
  if ($Service) { Write-Host "Start service with: Start-Service hived" }
}
finally {
  Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
