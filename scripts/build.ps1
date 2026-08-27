# Production build for MASS (Windows). Output binaries land in dist/bin/.
# Usage: scripts\build.ps1
param()

$ErrorActionPreference = "Stop"
$ROOT = Split-Path -Parent $PSScriptRoot
New-Item -ItemType Directory -Force -Path (Join-Path $ROOT "dist\bin") | Out-Null

Push-Location (Join-Path $ROOT "backend")
try {
    Write-Host "==> building mass-server ..."
    $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags="-w -s" -o (Join-Path $ROOT "dist\bin\mass-server.exe") ./cmd/server
    Write-Host "==> building seed-demo-users ..."
    go build -trimpath -ldflags="-w -s" -o (Join-Path $ROOT "dist\bin\seed-demo-users.exe") ./cmd/seed-demo-users
} finally {
    Pop-Location
}

Write-Host "==> done:"
Get-ChildItem (Join-Path $ROOT "dist\bin") | Select-Object Name, Length