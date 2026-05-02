param(
    [string]$InstallDir = "$env:USERPROFILE\.stac-man"
)

$ErrorActionPreference = "Stop"

$REPO = "bluegardenproject/stac-man"
$BINARY_NAME = "sm.exe"
$ASSET_NAME = "sm-windows-amd64.exe"

Write-Host "stac-man Installer" -ForegroundColor Blue -BackgroundColor Black
Write-Host "Installing to: $InstallDir" -ForegroundColor Yellow
Write-Host

Write-Host "Creating installation directory..." -ForegroundColor Blue
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Write-Host "Fetching latest release..." -ForegroundColor Blue
$releaseUrl = "https://api.github.com/repos/$REPO/releases/latest"
$release = Invoke-RestMethod -Uri $releaseUrl
$downloadUrl = ($release.assets | Where-Object { $_.name -eq $ASSET_NAME }).browser_download_url

if (-not $downloadUrl) {
    Write-Host "Error: Could not find Windows binary ($ASSET_NAME)" -ForegroundColor Red
    Write-Host "Available releases: https://github.com/$REPO/releases" -ForegroundColor Yellow
    exit 1
}

Write-Host "Download URL: $downloadUrl" -ForegroundColor Green

Write-Host "Downloading stac-man..." -ForegroundColor Blue
$binaryPath = "$InstallDir\$BINARY_NAME"
Invoke-WebRequest -Uri $downloadUrl -OutFile $binaryPath

Write-Host "Adding to PATH..." -ForegroundColor Blue
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$InstallDir*") {
    $newPath = "$InstallDir;$currentPath"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$InstallDir;$env:Path"
    Write-Host "Added $InstallDir to PATH" -ForegroundColor Green
} else {
    Write-Host "$InstallDir already in PATH" -ForegroundColor Yellow
}

Write-Host "Verifying installation..." -ForegroundColor Blue
try {
    & $binaryPath version | Out-Null
    Write-Host "Installation successful!" -ForegroundColor Green
} catch {
    Write-Host "Installation completed, but verification failed" -ForegroundColor Yellow
    Write-Host "  You may need to restart your terminal" -ForegroundColor Yellow
}

Write-Host
Write-Host "Installation Complete!" -ForegroundColor Green -BackgroundColor Black
Write-Host
Write-Host "Usage:" -ForegroundColor White
Write-Host "  sm log        - Show the stack tree" -ForegroundColor Green
Write-Host "  sm create     - Create a new stacked branch" -ForegroundColor Green
Write-Host "  sm submit     - Push branches and open/update PRs" -ForegroundColor Green
Write-Host "  sm --help     - Show all commands" -ForegroundColor Green
Write-Host
Write-Host "Note: You may need to restart your terminal" -ForegroundColor Yellow
