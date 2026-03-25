# Inari install script for Windows PowerShell
# Usage: irm https://raw.githubusercontent.com/KilimcininKorOglu/inari/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "KilimcininKorOglu/inari"
$Binary = "inari"

# Get latest release version
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name

if (-not $Version) {
    Write-Error "Could not determine latest release version"
    exit 1
}

# Determine architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else {
    Write-Error "Unsupported architecture: 32-bit Windows is not supported"
    exit 1
}

$Filename = "$Binary-$Version-windows-$Arch.exe"
$Url = "https://github.com/$Repo/releases/download/$Version/$Filename"

# Install directory
$InstallDir = Join-Path $env:LOCALAPPDATA "inari"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$InstallPath = Join-Path $InstallDir "$Binary.exe"

Write-Host "Installing Inari $Version for windows-$Arch..." -ForegroundColor Cyan
Write-Host "  Downloading from $Url"

# Download
Invoke-WebRequest -Uri $Url -OutFile $InstallPath -UseBasicParsing

Write-Host "  Installed to $InstallPath" -ForegroundColor Green
Write-Host ""

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$InstallDir;$UserPath", "User")
    $env:Path = "$InstallDir;$env:Path"
    Write-Host "  Added $InstallDir to user PATH" -ForegroundColor Yellow
    Write-Host "  Restart your terminal for PATH changes to take effect" -ForegroundColor Yellow
    Write-Host ""
}

Write-Host "Run 'inari --help' to get started."
