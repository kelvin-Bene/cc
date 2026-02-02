# qk Installer
# Builds and installs the 'qk' command

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "  qk Installer" -ForegroundColor Cyan
Write-Host "  ============" -ForegroundColor Cyan
Write-Host ""

# Build the binary
Write-Host "  Building qk.exe..." -ForegroundColor White
go build -o qk.exe .
if ($LASTEXITCODE -ne 0) {
    Write-Host "  Build failed!" -ForegroundColor Red
    exit 1
}
Write-Host "  Built qk.exe" -ForegroundColor Green

# Determine install location
$installDir = "$env:USERPROFILE\.qk\bin"

# Create install directory
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Write-Host "  Created: $installDir" -ForegroundColor Green
}

# Copy the binary
Copy-Item "qk.exe" -Destination "$installDir\qk.exe" -Force
Write-Host "  Installed: $installDir\qk.exe" -ForegroundColor Green

# Clean up local build artifact
Remove-Item "qk.exe" -ErrorAction SilentlyContinue

# Add to PATH if not already there
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "  Added to PATH: $installDir" -ForegroundColor Green
    Write-Host ""
    Write-Host "  NOTE: Restart your terminal for PATH changes to take effect" -ForegroundColor Yellow
} else {
    Write-Host "  Already in PATH: $installDir" -ForegroundColor Gray
}

Write-Host ""
Write-Host "  Installation complete!" -ForegroundColor Cyan
Write-Host "  Run 'qk set' to get started." -ForegroundColor White
Write-Host ""
