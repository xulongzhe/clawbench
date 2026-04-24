# ClawBench 构建脚本 (Windows PowerShell)
# 用法: .\build.ps1

$ErrorActionPreference = "Stop"

$NAME = "clawbench"
$DIST = "dist"
$ASSETS = "assets"

Write-Host "=== Building $NAME ==="

# 1. Build Go backend
Write-Host "[1/2] Building Go backend..."
if (Get-Command go -ErrorAction SilentlyContinue) {
    go build -o "$NAME.exe" ./cmd/server
    Write-Host "  Go binary: .\$NAME.exe"
} else {
    Write-Host "  Go not found, skipping backend build"
}

# 2. Build Vue frontend
Write-Host "[2/2] Building Vue frontend..."
if ((Test-Path "package.json") -and (Get-Command npm -ErrorAction SilentlyContinue)) {
    if (-not (Test-Path "node_modules")) {
        Write-Host "  Installing dependencies..."
        npm install
    }
    npm run build
    Write-Host "  Frontend: public/"
} else {
    Write-Host "  npm not found or no package.json, skipping frontend build"
}

Write-Host ""
Write-Host "=== Build complete ==="
Write-Host "  .\$NAME.exe         # Go binary"
Write-Host "  public/             # Frontend (if built)"
Write-Host ""
Write-Host "Run with: .\$NAME.exe"
