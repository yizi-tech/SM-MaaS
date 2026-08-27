# Package the production-grade MASS source release (Windows). Produces:
#   dist\mass-platform-<version>-src.tar.gz
#   dist\mass-platform-<version>-src.zip
#   dist\SHA256SUMS.txt
# Usage: scripts\package.ps1 [-Version 1.0.0]
param([string]$Version = "1.0.0")

$ErrorActionPreference = "Stop"
$ROOT = Split-Path -Parent $PSScriptRoot
$DIST = Join-Path $ROOT "dist"
New-Item -ItemType Directory -Force -Path $DIST | Out-Null

$NAME = "mass-platform-$Version-src"
$TGZ = Join-Path $DIST "$NAME.tar.gz"
$ZIP = Join-Path $DIST "$NAME.zip"

# Exclusions are read by bsdtar through -X. The file is written in the system
# ANSI codepage (ACP): Windows tar.exe performs its own file-name matching in
# ANSI, so passing CJK patterns inline breaks (mojibake). NOTE: do NOT write
# the Chinese name as a source literal here — PowerShell 5 reads BOM-less
# UTF-8 scripts as ANSI and would corrupt the literal; it is built from code
# points instead.
$EXCLUDEFILE = Join-Path $DIST "_exclude.txt"
$cjkScratch = ([string]::Concat([char]0x843D, [char]0x5730, [char]0x9875, [char]0x793A, [char]0x4F8B)) + ".html" # 落地页示例.html
$excludeLines = @(
    "*.exe",
    "*.log",
    "./uploads",
    "./backend/uploads",
    "./dist",
    "home_redesign.html",
    $cjkScratch,
    "./debug-auth-login-refused.md",
    "./docs/debug-auth-login-refused.md"
)
[System.IO.File]::WriteAllLines($EXCLUDEFILE, $excludeLines, [System.Text.Encoding]::Default)

Push-Location $ROOT
try {
    Write-Host "==> creating $TGZ"
    & tar.exe -a -cf $TGZ -X $EXCLUDEFILE -C $ROOT .
    if ($LASTEXITCODE -ne 0) { throw "tar (gz) failed" }

    Write-Host "==> creating $ZIP"
    & tar.exe -a -cf $ZIP -X $EXCLUDEFILE -C $ROOT .
    if ($LASTEXITCODE -ne 0) { throw "tar (zip) failed" }
} finally {
    Pop-Location
}

Write-Host "==> writing checksums"
Get-FileHash $TGZ -Algorithm SHA256 | ForEach-Object { $_.Hash.ToLower() + "  " + (Split-Path $_.Path -Leaf) } | Set-Content (Join-Path $DIST "SHA256SUMS.txt")
Get-FileHash $ZIP -Algorithm SHA256 | ForEach-Object { $_.Hash.ToLower() + "  " + (Split-Path $_.Path -Leaf) } | Add-Content (Join-Path $DIST "SHA256SUMS.txt")

Remove-Item $EXCLUDEFILE -Force -ErrorAction SilentlyContinue

Write-Host "==> done:"
Get-ChildItem $DIST | Where-Object { $_.Name -like "$NAME*" -or $_.Name -eq "SHA256SUMS.txt" } | Select-Object Name, Length