# cc Installer

$ErrorActionPreference = "Stop"

# ANSI color codes (pre-built to avoid PowerShell array-index parsing)
$R  = [char]27 + '[0m'
$BC = [char]27 + '[96m'
$C  = [char]27 + '[36m'
$DG = [char]27 + '[90m'
$BW = [char]27 + '[97m'
$BG = [char]27 + '[92m'
$BY = [char]27 + '[93m'
$BR = [char]27 + '[91m'
$BB = [char]27 + '[1;97m'

$line = $DG + ('─' * 38) + $R

# Logo
Write-Host ""
Write-Host "  ${BC} ██████╗ ██╗  ██╗${R}"
Write-Host "  ${BC}██╔═══██╗██║ ██╔╝${R}   ${BB}installer${R}"
Write-Host "  ${C}██║   ██║█████╔╝${R}"
Write-Host "  ${C}██║▄▄ ██║██╔═██╗${R}"
Write-Host "  ${DG}╚██████╔╝██║  ██╗${R}"
Write-Host "  ${DG} ╚══▀▀═╝ ╚═╝  ╚═╝${R}"
Write-Host ""
Write-Host "  $line"

# Build
Write-Host ""
Write-Host "  ${BC}◆${R} ${BW}Building${R}"

go build -o cc.exe .
if ($LASTEXITCODE -ne 0) {
    Write-Host "   ${BR}✗ Build failed${R}"
    exit 1
}
Write-Host "   ${DG}▪${R} cc.exe                     ${BG}✓${R}"

# Install
$installDir = "$env:USERPROFILE\.cc\bin"

Write-Host ""
Write-Host "  ${BC}◆${R} ${BW}Installing${R}"

if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Write-Host "   ${DG}▪${R} Created ~/.cc/bin/          ${BG}✓${R}"
}

Copy-Item "cc.exe" -Destination "$installDir\cc.exe" -Force
Write-Host "   ${DG}▪${R} Copied to ~/.cc/bin/        ${BG}✓${R}"

Remove-Item "cc.exe" -ErrorAction SilentlyContinue

# PATH
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "   ${DG}▪${R} Added to PATH               ${BG}✓${R}"
    Write-Host ""
    Write-Host "   ${BY}▪ Restart terminal for PATH changes${R}"
} else {
    Write-Host "   ${DG}▪${R} Already in PATH             ${BG}✓${R}"
}

# Done
Write-Host ""
Write-Host "  $line"
Write-Host "  ${BG}◆${R} ${BW}Ready${R} ${DG}· run${R} ${BC}cc set${R} ${DG}to configure${R}"
Write-Host ""


