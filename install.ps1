# Install script for codes - https://github.com/ourines/codes
# Usage: irm https://raw.githubusercontent.com/ourines/codes/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = "ourines/codes"
$Binary = "codes.exe"

# Detect architecture
$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    "arm64"
} elseif ([Environment]::Is64BitOperatingSystem) {
    "amd64"
} else {
    "386"
}

Write-Host "  > Detected platform: windows/$Arch" -ForegroundColor Cyan

# Fetch latest version from GitHub API
Write-Host "  > Fetching latest version..." -ForegroundColor Cyan
try {
    $Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    $Version = $Release.tag_name
} catch {
    Write-Error "Failed to determine latest version."
    return
}
Write-Host "  > Latest version: $Version" -ForegroundColor Cyan

$Archive = "codes-$Version-windows-$Arch.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Archive"
$TempDir = Join-Path $env:TEMP "codes-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
$TempZip = Join-Path $TempDir $Archive

Write-Host "  > Downloading $Archive..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $Url -OutFile $TempZip -UseBasicParsing
} catch {
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
    Write-Error "Download failed. Check that a release exists for windows/$Arch."
    return
}
Write-Host "  ✓ Downloaded successfully" -ForegroundColor Green

# Extract binary from archive
Expand-Archive -Path $TempZip -DestinationPath $TempDir -Force

# Install to user's local bin directory
$InstallDir = Join-Path $env:LOCALAPPDATA "codes"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$InstallPath = Join-Path $InstallDir $Binary
Move-Item -Path (Join-Path $TempDir $Binary) -Destination $InstallPath -Force
Write-Host "  ✓ Installed to $InstallPath" -ForegroundColor Green

# Clean up temp directory
Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "  ✓ Added $InstallDir to PATH" -ForegroundColor Green
}

# Run init
Write-Host "  > Running codes init..." -ForegroundColor Cyan
& $InstallPath init --yes

Write-Host ""
Write-Host "  ✓ Installation complete! Run 'codes --help' to get started." -ForegroundColor Green
