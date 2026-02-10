# Install script for codes - https://github.com/ourines/codes
# Usage: irm https://raw.githubusercontent.com/ourines/codes/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = "ourines/codes"
$Binary = "codes.exe"

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "32-bit systems are not supported."
    return
}

Write-Host "  > Detected platform: windows/$Arch" -ForegroundColor Cyan

$Url = "https://github.com/$Repo/releases/latest/download/codes-windows-$Arch.exe"
$TempFile = Join-Path $env:TEMP "codes-install.exe"

Write-Host "  > Downloading codes from $Url..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $Url -OutFile $TempFile -UseBasicParsing
} catch {
    Write-Error "Download failed. Check that a release exists for windows/$Arch."
    return
}
Write-Host "  ✓ Downloaded successfully" -ForegroundColor Green

# Install to user's local bin directory
$InstallDir = Join-Path $env:LOCALAPPDATA "codes"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$InstallPath = Join-Path $InstallDir $Binary
Move-Item -Path $TempFile -Destination $InstallPath -Force
Write-Host "  ✓ Installed to $InstallPath" -ForegroundColor Green

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "  ✓ Added $InstallDir to PATH" -ForegroundColor Green
}

# Run init
Write-Host "  > Running codes init..." -ForegroundColor Cyan
& $InstallPath init

Write-Host ""
Write-Host "  ✓ Installation complete! Run 'codes --help' to get started." -ForegroundColor Green
