# Hive installer for Windows
# Usage: irm https://get.hive.dev/windows | iex
#    or: .\install.ps1 -Version v0.2.0

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:ProgramFiles\Hive"
)

$ErrorActionPreference = "Stop"
$Repo = "Al-Sarraf-Tech/hive"

function Main {
    Write-Host "  Hive installer" -ForegroundColor Cyan
    Write-Host "  OS: Windows / amd64"

    # Resolve version
    if ($Version -eq "latest") {
        $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $Version = $release.tag_name
    }
    Write-Host "  Version: $Version"

    # Download
    $baseUrl = "https://github.com/$Repo/releases/download/$Version"
    $tmpDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }

    try {
        Write-Host "  Downloading binaries..."
        Invoke-WebRequest "$baseUrl/hived-windows-amd64.exe" -OutFile "$tmpDir\hived.exe"
        Invoke-WebRequest "$baseUrl/hive-windows-amd64.exe" -OutFile "$tmpDir\hive.exe"
        try { Invoke-WebRequest "$baseUrl/hivetop-windows-amd64.exe" -OutFile "$tmpDir\hivetop.exe" } catch {}

        # Install
        Write-Host "  Installing to $InstallDir..."
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        Copy-Item "$tmpDir\hived.exe" "$InstallDir\hived.exe" -Force
        Copy-Item "$tmpDir\hive.exe" "$InstallDir\hive.exe" -Force
        if (Test-Path "$tmpDir\hivetop.exe") {
            Copy-Item "$tmpDir\hivetop.exe" "$InstallDir\hivetop.exe" -Force
        }

        # Add to PATH if not already there
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "Machine")
            Write-Host "  Added $InstallDir to system PATH"
        }

        Write-Host "  Installed: hived.exe, hive.exe" -ForegroundColor Green
        Write-Host ""
        Write-Host "  Get started (open a new terminal for PATH changes):"
        Write-Host "    hive setup              # interactive first-run wizard"
        Write-Host "    hive setup --join CODE  # join an existing cluster"
    }
    finally {
        Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Main
