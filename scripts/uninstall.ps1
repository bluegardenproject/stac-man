param(
    [switch]$KeepConfig,
    [switch]$Purge
)

$ErrorActionPreference = "Stop"

$InstallDir = "$env:USERPROFILE\.stac-man"
# Matches internal/config.Path(): XDG_CONFIG_HOME wins, else
# $HOME/.config/stac-man (on Windows that's $USERPROFILE\.config\stac-man).
if ($env:XDG_CONFIG_HOME) {
    $ConfigDir = Join-Path $env:XDG_CONFIG_HOME "stac-man"
} else {
    $ConfigDir = "$env:USERPROFILE\.config\stac-man"
}

Write-Host "stac-man Uninstaller" -ForegroundColor Blue
Write-Host

if (Test-Path $InstallDir) {
    Write-Host "Removing binary directory: $InstallDir" -ForegroundColor Blue
    Remove-Item -Recurse -Force $InstallDir
    Write-Host "  removed" -ForegroundColor Green
} else {
    Write-Host "No binary directory at $InstallDir (already gone?)" -ForegroundColor Yellow
}

# Strip $InstallDir from the user's PATH. We rewrite the User-scoped
# environment variable, leaving system PATH untouched.
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -and $currentPath -like "*$InstallDir*") {
    $newPath = ($currentPath -split ';' | Where-Object { $_ -ne $InstallDir -and $_ -ne "" }) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "  removed $InstallDir from user PATH" -ForegroundColor Green
} else {
    Write-Host "  $InstallDir not in user PATH" -ForegroundColor Yellow
}

if (Test-Path $ConfigDir) {
    if ($KeepConfig) {
        Write-Host "Keeping config directory: $ConfigDir" -ForegroundColor Yellow
    } elseif ($Purge) {
        Remove-Item -Recurse -Force $ConfigDir
        Write-Host "Removed config directory: $ConfigDir" -ForegroundColor Green
    } else {
        $reply = Read-Host "Also remove user config at $ConfigDir? [y/N]"
        if ($reply -match '^(y|Y|yes|YES)$') {
            Remove-Item -Recurse -Force $ConfigDir
            Write-Host "  removed" -ForegroundColor Green
        } else {
            Write-Host "  kept" -ForegroundColor Yellow
        }
    }
}

Write-Host
Write-Host "Uninstall complete." -ForegroundColor Green
Write-Host
Write-Host "Per-repo stack metadata in each repo's .git/config was left in place;" -ForegroundColor Yellow
Write-Host "it has no effect once sm is gone, and is reused if you reinstall." -ForegroundColor Yellow
Write-Host "Open a new shell to drop $InstallDir from PATH." -ForegroundColor Yellow
